package lab

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"botchat/internal/almacen"
)

// Qué pasó con un voto o un toque (Entrada.Accion).
const (
	AccionVoto       = "voto"        // eligió una opción
	AccionCambio     = "cambio"      // cambió su voto por otra opción
	AccionRetiro     = "retiro"      // quitó su voto
	AccionDuplicado  = "duplicado"   // el mismo voto otra vez: no se hace nada
	AccionVarias     = "varias"      // marcó más de una opción en una encuesta de una sola
	AccionToque      = "toque"       // tocó un botón o una fila
	AccionVieja      = "vieja"       // el menú ya tiene otro más nuevo: se anota y no navega
	AccionExpirada   = "expirada"    // pasó el tiempo de ese menú
	AccionAjeno      = "ajeno"       // en un grupo, votó otra persona
	AccionOtroChat   = "otro_chat"   // el voto no es de ese chat
	AccionNoDescifra = "no_descifra" // no se pudo abrir el voto
	AccionDesconoce  = "desconocida" // una encuesta que no es nuestra
	AccionSinOpcion  = "sin_opcion"  // el voto no coincide con ninguna opción
)

// Entrada es lo que resultó de un voto o un toque. Si Navega es true, quien llama debe pasar Num al menú como si la persona
// hubiera escrito ese número.
type Entrada struct {
	Sesion  *almacen.SesionLab
	Accion  string
	Num     int
	Texto   string // texto de la opción elegida
	Votante string
	Navega  bool
	Prueba  bool // era una prueba de la matriz: ya se anotó "respondió"
}

// Resolver lleva el origen de un mensaje a su chat y su persona canónicos (número aunque llegue con LID). ok=false si no se conoce.
type Resolver func(ctx context.Context, ms types.MessageSource) (chat, persona string, ok bool)

// EsRespuesta dice si un mensaje es un voto de encuesta o la respuesta a un botón, una lista o un mensaje interactivo.
func EsRespuesta(m *waE2E.Message) bool {
	return m.GetPollUpdateMessage() != nil || m.GetButtonsResponseMessage() != nil || m.GetListResponseMessage() != nil || m.GetInteractiveResponseMessage() != nil
}

// Recibir procesa un voto o un toque. Devuelve nil si el mensaje no tiene que ver con el Laboratorio (por ejemplo, la respuesta de un
// botón de un menú que no es nuestro). Se llama desde el trabajador, nunca desde el manejador de eventos.
func (s *Servicio) Recibir(ctx context.Context, evt *events.Message, res Resolver) *Entrada {
	if evt == nil || evt.Message == nil || evt.Info.IsFromMe || !EsRespuesta(evt.Message) {
		return nil
	}
	if evt.Message.GetPollUpdateMessage() != nil {
		return s.recibirVoto(ctx, evt, res)
	}
	return s.recibirToque(ctx, evt, res)
}

// origenDe es el remitente original que usa la librería para abrir un voto (misma regla que en whatsmeow).
func origenDe(evt *events.Message, key *waCommon.MessageKey) (types.JID, bool) {
	if key.GetFromMe() {
		return evt.Info.Sender, true
	}
	if evt.Info.Chat.Server == types.DefaultUserServer || evt.Info.Chat.Server == types.HiddenUserServer {
		j, err := types.ParseJID(key.GetRemoteJID())
		return j, err == nil
	}
	j, err := types.ParseJID(key.GetParticipant())
	return j, err == nil
}

// descifra abre el voto. Si la librería no encuentra el secreto (por ejemplo, el voto llega por el chat con LID y el mensaje salió
// al chat con número), se vuelve a guardar NUESTRA copia del secreto bajo la clave que busca la librería y se reintenta una vez.
func (s *Servicio) descifra(ctx context.Context, evt *events.Message, ses *almacen.SesionLab) (*waE2E.PollVoteMessage, error) {
	v, err := s.Cli.DecryptPollVote(ctx, evt)
	if err == nil || len(ses.Secreto) == 0 {
		return v, err
	}
	key := evt.Message.GetPollUpdateMessage().GetPollCreationMessageKey()
	orig, ok := origenDe(evt, key)
	if !ok {
		return nil, err
	}
	if e := s.Cli.PutMessageSecret(ctx, evt.Info.Chat, orig, types.MessageID(ses.ID), ses.Secreto); e != nil {
		return nil, err
	}
	return s.Cli.DecryptPollVote(ctx, evt)
}

func (s *Servicio) recibirVoto(ctx context.Context, evt *events.Message, res Resolver) *Entrada {
	pu := evt.Message.GetPollUpdateMessage()
	pollID := pu.GetPollCreationMessageKey().GetID()
	ses := s.Alm.ObtenerSesionLab(s.BotID, pollID)
	if ses == nil || ses.Tipo != almacen.LabEncuesta {
		return &Entrada{Accion: AccionDesconoce}
	}
	chat, persona, ok := res(ctx, evt.Info.MessageSource)
	if !ok || chat != ses.Chat {
		return &Entrada{Sesion: ses, Accion: AccionOtroChat}
	}
	votante := persona
	if !esGrupo(chat) {
		votante = chat
	}
	e := &Entrada{Sesion: ses, Votante: votante, Prueba: ses.Origen == almacen.LabOrigenPrueba}
	if esGrupo(chat) && persona != ses.Persona && s.aj("grupo_cualquiera") != "1" {
		e.Accion = AccionAjeno
		return e
	}
	vote, err := s.descifra(ctx, evt, ses)
	if err != nil {
		s.Log("No se pudo abrir un voto: " + err.Error())
		e.Accion = AccionNoDescifra
		return e
	}
	// Los votos son hashes SHA-256 de los nombres de las opciones: se comparan con los de las opciones que mandamos.
	var nums []int
	desconocidas := 0
	for _, h := range vote.GetSelectedOptions() {
		hallada := false
		for _, o := range ses.Opciones {
			x := hashOpcion(o.Texto)
			if bytes.Equal(h, x[:]) {
				nums = append(nums, o.Num)
				hallada = true
				break
			}
		}
		if !hallada {
			desconocidas++
		}
	}
	sort.Ints(nums)
	nums = sinRepetir(nums)
	if len(nums) == 0 && desconocidas > 0 {
		e.Accion = AccionSinOpcion
		return e
	}
	ant, hubo := s.Alm.ObtenerVotoLab(s.BotID, ses.ID, votante)
	if hubo && igualesInt(ant.Nums, nums) {
		e.Accion = AccionDuplicado // la misma selección otra vez (la repetición de un mismo voto)
		return e
	}
	_ = s.Alm.GuardarVotoLab(s.BotID, ses.ID, votante, nums, s.ahora())
	switch {
	case len(nums) == 0:
		e.Accion = AccionRetiro
		return e
	case len(nums) > 1:
		e.Accion = AccionVarias
		return e
	}
	e.Num = nums[0]
	e.Texto = textoDe(ses, e.Num)
	e.Accion = AccionVoto
	if hubo && len(ant.Nums) > 0 {
		e.Accion = AccionCambio
	}
	s.cierra(ses, e)
	return e
}

// cierra aplica las reglas comunes a un voto o un toque válido: si la sesión ya caducó o la reemplazó otra, se anota y no navega.
func (s *Servicio) cierra(ses *almacen.SesionLab, e *Entrada) {
	switch {
	case ses.Estado == almacen.LabReemplazada:
		e.Accion = AccionVieja
	case ses.Estado == almacen.LabExpirada || (ses.ExpiraTS > 0 && s.ahora().Unix() > ses.ExpiraTS):
		if ses.Estado != almacen.LabExpirada {
			s.Alm.MarcarSesionLab(s.BotID, ses.ID, almacen.LabExpirada)
		}
		e.Accion = AccionExpirada
	}
	if e.Prueba {
		// Una prueba de la matriz: la respuesta llegó al bot. No navega ningún menú.
		if ses.PruebaID > 0 && (e.Accion == AccionVoto || e.Accion == AccionCambio || e.Accion == AccionToque) {
			_ = s.Alm.MarcarResultadoLab(s.BotID, ses.PruebaID, "respondio", 1)
		}
		return
	}
	e.Navega = e.Accion == AccionVoto || e.Accion == AccionCambio || e.Accion == AccionToque
}

func textoDe(ses *almacen.SesionLab, num int) string {
	for _, o := range ses.Opciones {
		if o.Num == num {
			return o.Texto
		}
	}
	return ""
}

func sinRepetir(xs []int) []int {
	var out []int
	for i, x := range xs {
		if i == 0 || x != xs[i-1] {
			out = append(out, x)
		}
	}
	return out
}

func igualesInt(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// idDeToque saca el id del botón, la fila o el quick_reply que tocó la persona y el id del mensaje al que responde.
func idDeToque(m *waE2E.Message) (id, mensaje string) {
	switch {
	case m.GetButtonsResponseMessage() != nil:
		r := m.GetButtonsResponseMessage()
		return r.GetSelectedButtonID(), r.GetContextInfo().GetStanzaID()
	case m.GetListResponseMessage() != nil:
		r := m.GetListResponseMessage()
		return r.GetSingleSelectReply().GetSelectedRowID(), r.GetContextInfo().GetStanzaID()
	case m.GetInteractiveResponseMessage() != nil:
		r := m.GetInteractiveResponseMessage()
		var p struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal([]byte(r.GetNativeFlowResponseMessage().GetParamsJSON()), &p)
		return p.ID, r.GetContextInfo().GetStanzaID()
	}
	return "", ""
}

// partesID separa "bc|<mensaje>|<num>".
func partesID(id string) (mensaje string, num int, ok bool) {
	p := strings.Split(id, "|")
	if len(p) != 3 || p[0] != "bc" {
		return "", 0, false
	}
	n, err := strconv.Atoi(p[2])
	if err != nil {
		return "", 0, false
	}
	return p[1], n, true
}

func (s *Servicio) recibirToque(ctx context.Context, evt *events.Message, res Resolver) *Entrada {
	id, citado := idDeToque(evt.Message)
	msgID, num, ok := partesID(id)
	if !ok {
		return nil // no es un botón nuestro
	}
	if citado != "" && citado != msgID {
		msgID = citado
	}
	ses := s.Alm.ObtenerSesionLab(s.BotID, msgID)
	if ses == nil || ses.Tipo == almacen.LabEncuesta {
		return nil
	}
	chat, persona, ok := res(ctx, evt.Info.MessageSource)
	if !ok || chat != ses.Chat {
		return &Entrada{Sesion: ses, Accion: AccionOtroChat}
	}
	votante := persona
	if !esGrupo(chat) {
		votante = chat
	}
	e := &Entrada{Sesion: ses, Votante: votante, Accion: AccionToque, Num: num, Texto: textoDe(ses, num), Prueba: ses.Origen == almacen.LabOrigenPrueba}
	if esGrupo(chat) && persona != ses.Persona && s.aj("grupo_cualquiera") != "1" {
		e.Accion = AccionAjeno
		return e
	}
	valida := false
	for _, o := range ses.Opciones {
		if o.Num == num {
			valida = true
		}
	}
	if !valida {
		e.Accion = AccionSinOpcion
		return e
	}
	s.cierra(ses, e)
	return e
}
