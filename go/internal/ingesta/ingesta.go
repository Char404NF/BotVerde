package ingesta

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/identidad"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/proto/waWeb"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// Contactos es el almacén de contactos de whatsmeow (client.Store.Contacts).
type Contactos interface {
	GetContact(ctx context.Context, jid types.JID) (types.ContactInfo, error)
}

// Mapa es el mapa entre identificadores ocultos (LID) y números (client.Store.LIDs).
type Mapa interface {
	GetPNForLID(ctx context.Context, lid types.JID) (types.JID, error)
}

// Servicios son los trabajos en segundo plano que dispara la ingesta.
type Servicios interface {
	PedirFoto(chat string)
	PedirAsunto(chat string)
	EncolarDescarga(chat, id string)
}

// Config reúne lo que necesita la ingesta.
type Config struct {
	BotID     int64
	Alm       *almacen.Almacen
	Contactos Contactos
	Mapa      Mapa
	Servicios Servicios
	Politica  func() Politica
	Red       func() Red
	Libre     func() int64
	// EsPropio dice si un JID es el del propio bot (teléfono o LID). Puede ser nil.
	EsPropio func(types.JID) bool
	// VerUnaVezDias son los días que un "ver una vez" espera a que lo abran antes de borrarse (0 = 14).
	VerUnaVezDias func() int
	// Log recibe avisos sin datos personales (nunca texto de mensajes, nombres ni nombres de archivo).
	Log func(string)
}

// Ingesta guarda lo que llega de WhatsApp.
type Ingesta struct {
	Config
	mu sync.Mutex
}

// Nueva crea la ingesta.
func Nueva(c Config) *Ingesta {
	if c.Log == nil {
		c.Log = func(string) {}
	}
	if c.Politica == nil {
		c.Politica = PoliticaPorDefecto
	}
	if c.Red == nil {
		c.Red = func() Red { return RedDesconocida }
	}
	if c.Libre == nil {
		c.Libre = func() int64 { l, _ := c.Alm.Libre(); return l }
	}
	return &Ingesta{Config: c}
}

func jidPN(j types.JID) bool  { return j.Server == types.DefaultUserServer }
func jidLID(j types.JID) bool { return j.Server == types.HiddenUserServer }

// resolver pasa un LID a su número si se conoce; si no, devuelve el mismo JID (solo para uso interno).
func (g *Ingesta) resolver(ctx context.Context, j types.JID) types.JID {
	j = j.ToNonAD()
	if jidLID(j) && g.Mapa != nil {
		if pn, err := g.Mapa.GetPNForLID(ctx, j); err == nil && !pn.IsEmpty() {
			return pn.ToNonAD()
		}
	}
	return j
}

// chatDe decide la clave interna del chat. Devuelve también el LID original si se terminó usando el teléfono
// (para unir chats que antes se guardaron por separado).
func (g *Ingesta) chatDe(ctx context.Context, info types.MessageInfo) (chat types.JID, lidOriginal types.JID, esGrupo, ok bool) {
	c := info.Chat.ToNonAD()
	switch c.Server {
	case types.GroupServer:
		return c, types.EmptyJID, true, true
	case types.DefaultUserServer, types.HiddenUserServer:
	default:
		return c, types.EmptyJID, false, false // estados, canales, bots, listas de difusión
	}
	r := g.resolver(ctx, c)
	if jidLID(r) {
		alt := info.SenderAlt
		if info.IsFromMe {
			alt = info.RecipientAlt
		}
		if jidPN(alt) {
			return alt.ToNonAD(), r, false, true
		}
		return r, types.EmptyJID, false, true
	}
	if jidLID(c) {
		return r, c, false, true
	}
	return r, types.EmptyJID, false, true
}

type nombres struct{ guardado, publico, negocio string }

// nombresDe consulta el almacén de contactos (por teléfono y por LID).
func (g *Ingesta) nombresDe(ctx context.Context, js ...types.JID) nombres {
	var n nombres
	if g.Contactos == nil {
		return n
	}
	for _, j := range js {
		if j.IsEmpty() {
			continue
		}
		ci, err := g.Contactos.GetContact(ctx, j.ToNonAD())
		if err != nil || !ci.Found {
			continue
		}
		if n.guardado == "" {
			n.guardado = firstNonEmpty(ci.FullName, ci.FirstName)
		}
		if n.publico == "" {
			n.publico = ci.PushName
		}
		if n.negocio == "" {
			n.negocio = ci.BusinessName
		}
	}
	return n
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// asegurarChat crea o actualiza la fila del chat con los nombres que se conocen. Devuelve true si el chat es nuevo.
func (g *Ingesta) asegurarChat(ctx context.Context, chat types.JID, lid types.JID, esGrupo bool, pushName string) (nuevo bool, err error) {
	jid := chat.String()
	existia := g.Alm.ExisteChat(g.BotID, jid)
	if !jidLID(lid) {
		lid = types.EmptyJID
	}
	if lid.Server == types.HiddenUserServer && g.Alm.ExisteChat(g.BotID, lid.String()) {
		if err := g.Alm.FusionarChat(g.BotID, lid.String(), jid); err != nil {
			g.Log("no se pudo unir un chat: " + err.Error())
		}
	}
	var n nombres
	if !esGrupo {
		n = g.nombresDe(ctx, chat, lid)
	}
	_, err = g.Alm.ActualizarChat(g.BotID, jid, esGrupo, func(c *almacen.Chat) {
		if esGrupo {
			return
		}
		if jidPN(chat) {
			c.TelE164 = "+" + chat.User
		}
		if n.guardado != "" {
			c.Guardado = n.guardado
		}
		if n.negocio != "" {
			c.Negocio = n.negocio
		}
		if p := firstNonEmpty(n.publico, pushName); p != "" {
			c.Publico = p
		}
	})
	return !existia, err
}

// remitente devuelve la clave interna y el nombre para mostrar de quien envió un mensaje.
func (g *Ingesta) remitente(ctx context.Context, info types.MessageInfo, chat types.JID, esGrupo bool) (clave, nombre string) {
	if info.IsFromMe {
		return "yo", ""
	}
	if !esGrupo {
		return chat.String(), ""
	}
	s := g.resolver(ctx, info.Sender)
	alt := info.SenderAlt
	if jidLID(s) && jidPN(alt) {
		s = alt.ToNonAD()
	}
	n := g.nombresDe(ctx, s, info.Sender)
	tel := identidad.Telefono{}
	if jidPN(s) {
		tel = identidad.ParseTelefono(s.User)
	}
	nombre = identidad.NombreMostrado(identidad.Datos{Guardado: n.guardado, Publico: firstNonEmpty(n.publico, info.PushName), Negocio: n.negocio, Telefono: tel.Mostrado})
	return s.String(), nombre
}

// Mensaje procesa un mensaje en vivo (historial=false) o de historial (historial=true).
func (g *Ingesta) Mensaje(ctx context.Context, evt *events.Message, historial bool) {
	defer func() {
		if r := recover(); r != nil {
			g.Log(fmt.Sprintf("error interno al guardar un mensaje: %v", r))
		}
	}()
	l, err := g.preparar(ctx, evt, historial)
	if err != nil {
		g.Log("no se pudo guardar un mensaje: " + err.Error())
		return
	}
	if l == nil {
		return
	}
	g.guardarLote(ctx, []*listo{l})
}

// listo es un mensaje preparado para guardar.
type listo struct {
	msg       *almacen.Mensaje
	miniatura []byte
	auto      bool
}

func (g *Ingesta) preparar(ctx context.Context, evt *events.Message, historial bool) (*listo, error) {
	c := Clasificar(evt)
	if c.Omitir {
		return nil, nil
	}
	chat, lid, esGrupo, ok := g.chatDe(ctx, evt.Info)
	if !ok {
		return nil, nil
	}
	nuevoChat, err := g.asegurarChat(ctx, chat, lid, esGrupo, pushDe(evt))
	if err != nil {
		return nil, err
	}
	jid := chat.String()
	if nuevoChat && esGrupo && g.Servicios != nil {
		g.Servicios.PedirAsunto(jid)
	}
	if c.Revocar != "" {
		quien, _ := g.remitente(ctx, evt.Info, chat, esGrupo)
		if evt.Info.IsFromMe {
			quien = ""
		}
		_, err := g.Alm.MarcarEliminado(g.BotID, jid, c.Revocar, quien, evt.Info.Timestamp.Unix())
		return nil, err
	}
	if c.Editar != nil {
		if !g.Alm.AuditoriaActiva(g.BotID, jid) {
			// Auditoría apagada en este chat: se anota que se editó, pero el texto nuevo no se guarda.
			_, err := g.Alm.MarcarEditado(g.BotID, jid, c.Editar.ID)
			return nil, err
		}
		_, err := g.Alm.EditarTexto(g.BotID, jid, c.Editar.ID, c.Editar.Texto)
		return nil, err
	}
	rem, remNombre := g.remitente(ctx, evt.Info, chat, esGrupo)
	if c.Reaccion != nil {
		quien := rem
		if evt.Info.IsFromMe {
			quien = almacen.QuienYo
		}
		return nil, g.Alm.GuardarReaccion(g.BotID, jid, c.Reaccion.ID, quien, c.Reaccion.Emoji, evt.Info.Timestamp.Unix())
	}
	m := &almacen.Mensaje{
		ID: evt.Info.ID, ChatJID: jid, BotID: g.BotID, RemitenteJID: rem, RemitenteNombre: remNombre, EsMio: evt.Info.IsFromMe,
		Tipo: c.Tipo, Texto: c.Texto, TS: evt.Info.Timestamp.Unix(), ArchivoMime: c.Mime, ArchivoBytes: c.Bytes, ArchivoNombre: c.Nombre,
		Ancho: c.Ancho, Alto: c.Alto, DuracionS: c.DurS, NotaVoz: c.NotaVoz, ArchivoInfo: c.Info, ArchivoEstado: almacen.EstadoNinguno,
		CitaID: c.CitaID, CitaTexto: c.CitaTexto, Menciones: c.Menciones, Reenviado: c.Reenviado,
		DelHistorial: historial, SHA256: shaDe(c.Info), Enlaces: tieneEnlace(c.Texto),
	}
	if c.CitaID != "" {
		m.CitaAutor = g.autorCitado(ctx, c.CitaAutor, evt.Info, chat, esGrupo)
	}
	if evt.Info.IsFromMe {
		m.EstadoEnvio = "telefono" // lo escribí yo desde el teléfono (no desde esta app)
	}
	if m.TS <= 0 {
		m.TS = time.Now().Unix()
	}
	cat := almacen.CategoriaDe(c.Tipo)
	l := &listo{msg: m, miniatura: c.Miniatura}
	switch {
	case c.Tipo == almacen.TipoVerUnaVez && len(c.Info) > 0 && !historial:
		// Llegó completo: se baja al llegar (sin importar las reglas de descarga) y queda cifrado hasta que se abra.
		dias := 14
		if g.VerUnaVezDias != nil && g.VerUnaVezDias() > 0 {
			dias = g.VerUnaVezDias()
		}
		m.VerEstado, m.VerTipo, m.VerExpira = almacen.VerPendiente, c.VerTipo, m.TS+int64(dias)*86400
		m.ArchivoEstado = almacen.EstadoPendiente
		l.auto = true
		if ok, _ := g.Alm.PromoverVerUnaVez(m); ok {
			// ya había un aviso sin contenido de este mensaje: se completó
			if g.Servicios != nil {
				g.Servicios.EncolarDescarga(jid, m.ID)
			}
			return nil, nil
		}
	case c.Tipo == almacen.TipoVerUnaVez:
		// Sin contenido (aviso) o venido del historial: no se guarda nada del archivo.
		m.ArchivoEstado = almacen.EstadoOmitido
		m.Texto, m.ArchivoInfo, m.ArchivoMime, m.ArchivoBytes, m.Ancho, m.Alto, m.DuracionS = almacen.TextoVerUnaVez, nil, "", 0, 0, 0, 0
		l.miniatura = nil
	case cat != "" && len(c.Info) == 0:
		m.ArchivoEstado = almacen.EstadoOmitido
	case cat != "" && historial:
		m.ArchivoEstado = almacen.EstadoOmitido // el multimedia del historial solo baja con el botón "Descargar"
	case cat != "":
		auto, _ := g.Politica().Decidir(cat, c.Bytes, g.Red(), g.Libre())
		if auto {
			m.ArchivoEstado = almacen.EstadoPendiente
			l.auto = true
		}
	}
	// Único punto de la auditoría para todo lo que llega (en vivo y del historial): con el interruptor del chat apagado se guarda sin texto.
	if m.Tipo == almacen.TipoVerUnaVez {
		m.SHA256 = nil // un "ver una vez" no deja ninguna huella
	}
	g.Alm.AplicarAuditoria(m)
	return l, nil
}

var reEnlace = regexp.MustCompile(`(?i)(?:https?://|www\.)\S+`)

// tieneEnlace dice si el texto trae un enlace (solo la bandera; el texto no se guarda en otro lado).
func tieneEnlace(t string) bool { return t != "" && reEnlace.MatchString(t) }

// shaDe saca la huella del archivo de los datos de descarga (nil si no hay).
func shaDe(info []byte) []byte {
	if len(info) == 0 {
		return nil
	}
	i, err := ParseInfo(info)
	if err != nil || i.SHA256 == "" {
		return nil
	}
	b, err := base64.StdEncoding.DecodeString(i.SHA256)
	if err != nil || len(b) == 0 {
		return nil
	}
	return b
}

// VerUnaVezSinContenido guarda el aviso de un "ver una vez" que WhatsApp no entregó a este dispositivo.
func (g *Ingesta) VerUnaVezSinContenido(ctx context.Context, info types.MessageInfo) {
	defer func() {
		if r := recover(); r != nil {
			g.Log(fmt.Sprintf("error interno al guardar un aviso: %v", r))
		}
	}()
	chat, lid, esGrupo, ok := g.chatDe(ctx, info)
	if !ok {
		return
	}
	nuevo, err := g.asegurarChat(ctx, chat, lid, esGrupo, "")
	if err != nil {
		return
	}
	if nuevo && esGrupo && g.Servicios != nil {
		g.Servicios.PedirAsunto(chat.String())
	}
	rem, nombre := g.remitente(ctx, info, chat, esGrupo)
	m := &almacen.Mensaje{
		ID: info.ID, ChatJID: chat.String(), BotID: g.BotID, RemitenteJID: rem, RemitenteNombre: nombre, EsMio: info.IsFromMe,
		Tipo: almacen.TipoVerUnaVez, Texto: almacen.TextoVerUnaVez, TS: info.Timestamp.Unix(), ArchivoEstado: almacen.EstadoOmitido,
	}
	if m.TS <= 0 {
		m.TS = time.Now().Unix()
	}
	g.guardarLote(ctx, []*listo{{msg: m}})
}

// ChatDe da la clave del chat a la que pertenece un evento (acuses, presencia) con las mismas reglas que los mensajes.
func (g *Ingesta) ChatDe(ctx context.Context, src types.MessageSource) (string, bool) {
	chat, _, _, ok := g.chatDe(ctx, types.MessageInfo{MessageSource: src})
	if !ok {
		return "", false
	}
	return chat.String(), true
}

// Persona lleva el JID de quien dio un acuse a la forma guardada (teléfono si se conoce).
func (g *Ingesta) Persona(ctx context.Context, j types.JID) string {
	return g.resolver(ctx, j).String()
}

// autorCitado lleva a la forma guardada (teléfono si se conoce, o "yo") el JID de quien escribió un mensaje citado.
func (g *Ingesta) autorCitado(ctx context.Context, participante string, info types.MessageInfo, chat types.JID, esGrupo bool) string {
	if participante == "" {
		if !esGrupo { // en un chat privado, sin participante, la cita es de la otra persona
			return chat.String()
		}
		return ""
	}
	j, err := types.ParseJID(participante)
	if err != nil {
		return participante
	}
	if g.EsPropio != nil && g.EsPropio(j) {
		return almacen.QuienYo
	}
	r := g.resolver(ctx, j.ToNonAD())
	return r.String()
}

func pushDe(evt *events.Message) string {
	if evt.Info.IsFromMe {
		return ""
	}
	return evt.Info.PushName
}

// guardarLote guarda varios mensajes (de uno o de muchos chats) y lanza el trabajo posterior.
func (g *Ingesta) guardarLote(ctx context.Context, ls []*listo) {
	msgs := make([]*almacen.Mensaje, 0, len(ls))
	porID := map[string]*listo{}
	for _, l := range ls {
		msgs = append(msgs, l.msg)
		porID[l.msg.ChatJID+"\x00"+l.msg.ID] = l
	}
	nuevos, err := g.Alm.GuardarMensajes(msgs)
	if err != nil {
		g.Log("no se pudo guardar un lote de mensajes: " + err.Error())
		return
	}
	fotos := map[string]bool{}
	for _, m := range nuevos {
		l := porID[m.ChatJID+"\x00"+m.ID]
		if l != nil && len(l.miniatura) > 0 && m.Tipo != almacen.TipoVerUnaVez {
			ruta := almacen.RutaMiniatura(g.BotID, m.ChatJID, m.ID)
			if err := g.Alm.EscribirCifrado(ruta, l.miniatura); err == nil {
				_ = g.Alm.FijarMiniatura(g.BotID, m.ChatJID, m.ID, ruta)
			}
		}
		if l != nil && l.auto && g.Servicios != nil {
			g.Servicios.EncolarDescarga(m.ChatJID, m.ID)
		}
		fotos[m.ChatJID] = true
	}
	if g.Servicios != nil {
		for chat := range fotos {
			g.Servicios.PedirFoto(chat)
		}
	}
}

// ---------- historial ----------

// ParseFunc convierte un mensaje del historial al mismo formato que los mensajes en vivo (client.ParseWebMessage).
type ParseFunc func(chat types.JID, w *waWeb.WebMessageInfo) (*events.Message, error)

// Historial procesa un bloque de historial por lotes, directo a la base, sin volcarlo a disco.
func (g *Ingesta) Historial(ctx context.Context, hs *waHistorySync.HistorySync, parse ParseFunc) (mensajes int) {
	defer func() {
		if r := recover(); r != nil {
			g.Log(fmt.Sprintf("error interno al procesar historial: %v", r))
		}
	}()
	if hs == nil {
		return 0
	}
	switch hs.GetSyncType() {
	case waHistorySync.HistorySync_PUSH_NAME:
		g.nombresPublicos(ctx, hs)
		return 0
	case waHistorySync.HistorySync_INITIAL_BOOTSTRAP, waHistorySync.HistorySync_RECENT, waHistorySync.HistorySync_ON_DEMAND:
	default:
		return 0
	}
	for _, conv := range hs.GetConversations() {
		raw := conv.GetID()
		if raw == "" {
			continue
		}
		cj, err := types.ParseJID(raw)
		if err != nil {
			continue
		}
		var lote []*listo
		for _, hm := range conv.GetMessages() {
			w := hm.GetMessage()
			if w == nil || w.GetKey().GetID() == "" {
				continue
			}
			evt, err := parse(cj, w)
			if err != nil || evt == nil {
				continue // sin remitente conocido u otro defecto: se salta
			}
			l, err := g.preparar(ctx, evt, true)
			if err != nil {
				g.Log("historial: " + err.Error())
				continue
			}
			if l != nil {
				lote = append(lote, l)
			}
		}
		if cj.Server == types.GroupServer && conv.GetDisplayName() != "" {
			jid := cj.ToNonAD().String()
			_, _ = g.Alm.ActualizarChat(g.BotID, jid, true, func(c *almacen.Chat) { c.Guardado = conv.GetDisplayName() })
		}
		g.guardarLote(ctx, lote)
		mensajes += len(lote)
	}
	// Los nombres públicos de la lista mandan sobre el de cada mensaje, y solo se aplican a chats que ya existen.
	g.nombresPublicos(ctx, hs)
	return mensajes
}

func (g *Ingesta) nombresPublicos(ctx context.Context, hs *waHistorySync.HistorySync) {
	for _, pn := range hs.GetPushnames() {
		j, err := types.ParseJID(pn.GetID())
		if err != nil || pn.GetPushname() == "" {
			continue
		}
		g.NombrePublico(ctx, j, pn.GetPushname())
	}
}

// NombrePublico guarda el nombre público (push name) de una persona si ya tiene chat.
func (g *Ingesta) NombrePublico(ctx context.Context, j types.JID, nombre string) {
	if nombre == "" {
		return
	}
	chat := g.resolver(ctx, j)
	if chat.Server != types.DefaultUserServer && chat.Server != types.HiddenUserServer {
		return
	}
	jid := chat.String()
	if !g.Alm.ExisteChat(g.BotID, jid) {
		return
	}
	_, _ = g.Alm.ActualizarChat(g.BotID, jid, false, func(c *almacen.Chat) { c.Publico = nombre })
}

// RefrescarNombres vuelve a leer la agenda para todos los chats (después de sincronizar contactos).
func (g *Ingesta) RefrescarNombres(ctx context.Context) {
	chats, err := g.Alm.TodosLosChats(g.BotID)
	if err != nil {
		return
	}
	for _, s := range chats {
		j, err := types.ParseJID(s)
		if err != nil || j.Server == types.GroupServer {
			continue
		}
		n := g.nombresDe(ctx, j)
		if n == (nombres{}) {
			continue
		}
		_, _ = g.Alm.ActualizarChat(g.BotID, s, false, func(c *almacen.Chat) {
			if n.guardado != "" {
				c.Guardado = n.guardado
			}
			if n.publico != "" {
				c.Publico = n.publico
			}
			if n.negocio != "" {
				c.Negocio = n.negocio
			}
		})
	}
}

// Asunto guarda el asunto de un grupo.
func (g *Ingesta) Asunto(chat, asunto string) {
	if strings.TrimSpace(asunto) == "" {
		return
	}
	_, _ = g.Alm.ActualizarChat(g.BotID, chat, true, func(c *almacen.Chat) { c.Guardado = asunto })
}

// Enviado guarda un mensaje de texto que envió el propio bot (por ejemplo la respuesta "hola").
func (g *Ingesta) Enviado(ctx context.Context, chat types.JID, id, texto string, ts time.Time, propio types.JID) {
	e := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: chat, Sender: propio, IsFromMe: true, IsGroup: chat.Server == types.GroupServer},
			ID:            id, Timestamp: ts,
		},
		RawMessage: &waE2E.Message{Conversation: &texto},
	}
	e.UnwrapRaw()
	g.Mensaje(ctx, e, false)
}
