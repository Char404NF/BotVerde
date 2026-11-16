package wabot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"botchat/internal/lab"
	"botchat/internal/motor"
)

// Laboratorio (v0.5.0, bloque G): menú como encuesta o botones con respaldo a texto, y la matriz de pruebas. Apagado por defecto.

// armarLab crea el Laboratorio del bot en marcha.
func (s *estado) armarLab() {
	s.lab = lab.Nuevo(s.botID, alm, lab.Real{Client: s.cli}, s.envio, s.log)
	s.lab.AlVolverATexto = func(motivo string) {
		s.avisaCambio("lab")
		accionDueno(s.botID, "lab.volver_texto", "", motivo)
	}
}

// resolverLab lleva un voto o un toque a su chat y su persona canónicos (número aunque llegue con LID).
func (s *estado) resolverLab(ctx context.Context, ms types.MessageSource) (string, string, bool) {
	chat, ok := s.ing.ChatDe(ctx, ms)
	if !ok {
		return "", "", false
	}
	persona := chat
	if ms.IsGroup || ms.Chat.Server == types.GroupServer {
		persona = s.ing.Persona(ctx, ms.Sender)
	}
	return chat, persona, true
}

// alLab atiende los votos de encuesta y las respuestas de botones, listas y mensajes interactivos. Corre en el trabajador (el
// manejador de eventos solo encola). Devuelve true si el mensaje era de un menú nuestro y ya se atendió; si no, sigue el camino de
// siempre (por ejemplo, la respuesta de un botón de otro menú se trata como el texto que muestra).
func (s *estado) alLab(v *events.Message, sincronizando bool) bool {
	if s.lab == nil || !lab.EsRespuesta(v.Message) {
		return false
	}
	// Un voto de encuesta nunca pasa por el camino de texto (la clasificación lo descarta), pero se atiende igual.
	e := s.lab.Recibir(s.ctx, v, s.resolverLab)
	if e == nil {
		return false
	}
	s.avisaCambio("lab")
	s.log("Menú interactivo: " + e.Accion + ".")
	if !e.Navega {
		if e.Avisar && !sincronizando {
			s.avisaMenuViejo(e.Sesion.Chat)
		}
		return true
	}
	if sincronizando {
		return true // un voto que llegó mientras el teléfono estaba sin conexión no navega el menú
	}
	chat := e.Sesion.Chat
	if e.Comando == "" && s.mods != nil && s.mods.VideoResponder(chat, e.Votante, e.Num) {
		return true // era la respuesta a las opciones de una descarga de video
	}
	texto := strconv.Itoa(e.Num)
	if e.Comando != "" {
		texto = e.Comando // un botón de un módulo («Reclamar premio»): el bot lo recibe como si ella lo hubiera escrito
	}
	m := motor.Mensaje{
		Chat: chat, ID: string(v.Info.ID), Texto: texto, EsTexto: true, EsGrupo: esGrupoChat(chat),
		TS: v.Info.Timestamp, RemitenteWA: v.Info.Sender.ToNonAD().String(), Remitente: e.Votante, Nombre: v.Info.PushName,
		// Votar o tocar es contestar al menú que mandé yo: en un grupo cuenta como respuesta a un mensaje mío.
		CitaEsMia: true, CitaID: e.Sesion.ID,
	}
	s.aprendeAlias(v)
	s.motor.Entrar(m)
	return true
}

func esGrupoChat(chat string) bool { return strings.HasSuffix(chat, "@g.us") }

// ---------- API para la app ----------

func laboratorio(botID int64) (*lab.Servicio, error) {
	mu.Lock()
	s := st
	mu.Unlock()
	if s == nil || s.botID != botID || s.lab == nil {
		return nil, errors.New("Enciende el bot para hacer pruebas.")
	}
	return s.lab, nil
}

type estadoLab struct {
	Activo         bool   `json:"activo"`
	AvisoAceptado  bool   `json:"avisoAceptado"`
	AvisoTS        int64  `json:"avisoTs"`
	Modo           string `json:"modo"`     // lo elegido (texto, encuesta, botones)
	ModoEfectivo   string `json:"modoReal"` // lo que vale ahora (texto si el Laboratorio está apagado)
	Grupos         bool   `json:"grupos"`
	EncMax         int    `json:"encMax"`
	ExpiraMin      int    `json:"expiraMin"`
	RespaldoEnc    bool   `json:"respaldoEnc"`
	RespaldoBot    bool   `json:"respaldoBot"`
	BotonesTipo    string `json:"botonesTipo"`
	VersionWA      string `json:"versionWa"`
	VersionCliente string `json:"versionCliente"`
	EnMarcha       bool   `json:"enMarcha"`
	BotonesHora    int    `json:"botonesHora"` // tope de menús con botones o listas por hora
}

// EstadoLab devuelve (JSON) los ajustes del Laboratorio y lo que se puede probar.
func EstadoLab(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	aj := func(k string) string { return a.AjusteBot(botID, k) }
	n := func(k string) int { return a.AjusteBotInt(botID, k) }
	ts := int64(n("lab_aviso_ts"))
	e := estadoLab{
		Activo: aj("lab_activo") == "1", AvisoAceptado: ts > 0, AvisoTS: ts, Modo: aj("lab_modo_menu"), Grupos: aj("lab_grupos") == "1",
		EncMax: n("lab_enc_max_opc"), ExpiraMin: n("lab_expira_min"), RespaldoEnc: aj("lab_respaldo_enc") == "1", RespaldoBot: aj("lab_respaldo_bot") == "1",
		BotonesTipo: aj("lab_botones_tipo"),
		VersionWA:   aj("lab_version_wa"),
	}
	e.ModoEfectivo = lab.ModoTexto
	switch e.Modo {
	case lab.ModoEncuesta, lab.ModoBotones:
		e.ModoEfectivo = e.Modo
	}
	e.BotonesHora = n("menu_botones_hora")
	mu.Lock()
	s := st
	mu.Unlock()
	if s != nil && s.botID == botID && s.lab != nil {
		e.EnMarcha = true
		e.VersionCliente = s.lab.Cli.VersionCliente()
	}
	return js(e, nil)
}

// AceptarAvisoLab anota que aceptaste el aviso de riesgo del Laboratorio (hace falta para encenderlo).
func AceptarAvisoLab(botID int64) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	if err := a.FijarAjusteBot(botID, "lab_aviso_ts", strconv.FormatInt(time.Now().Unix(), 10)); err != nil {
		return err
	}
	accionDueno(botID, "lab.aviso", "", "")
	return nil
}

// FijarAjusteLab cambia un ajuste del Laboratorio (con validación). Solo acepta las claves lab_*.
func FijarAjusteLab(botID int64, clave, valor string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	valor = strings.TrimSpace(valor)
	num := func(min, max int) error {
		n, e := strconv.Atoi(valor)
		if e != nil || n < min || n > max || strconv.Itoa(n) != valor {
			return fmt.Errorf("el valor debe estar entre %d y %d", min, max)
		}
		return nil
	}
	aviso := a.AjusteBotInt(botID, "lab_aviso_ts") > 0
	switch clave {
	case "lab_activo":
		if err := num(0, 1); err != nil {
			return err
		}
		if valor == "1" && !aviso {
			return lab.ErrSinAviso
		}
	case "lab_modo_menu":
		if valor != lab.ModoTexto && valor != lab.ModoEncuesta && valor != lab.ModoBotones {
			return errors.New("el modo debe ser texto, encuesta o botones")
		}
	case "lab_grupos", "lab_respaldo_enc", "lab_respaldo_bot":
		if err := num(0, 1); err != nil {
			return err
		}
	case "menu_botones_hora":
		if err := num(0, 500); err != nil {
			return err
		}
	case "lab_enc_max_opc":
		if err := num(2, 32); err != nil {
			return err
		}
	case "lab_expira_min":
		if err := num(1, 1440); err != nil {
			return err
		}
	case "lab_botones_tipo":
		switch valor {
		case "auto", "botones", "lista", "interactivo":
		default:
			return errors.New("el tipo debe ser auto, botones, lista o interactivo")
		}
	case "lab_version_wa":
		if len([]rune(valor)) > 30 {
			return errors.New("la versión es muy larga")
		}
	default:
		return errors.New("ajuste desconocido")
	}
	if err := a.FijarAjusteBot(botID, clave, valor); err != nil {
		return err
	}
	accionDueno(botID, "lab.ajuste", clave, "valor="+valor)
	return nil
}

// avisaMenuViejo le dice a quien tocó un botón de un menú que ya no sirve que escriba !menu. A lo más una vez cada 2 minutos por chat, y
// nunca a un grupo: así una ráfaga de toques viejos no llena el chat de avisos.
func (s *estado) avisaMenuViejo(chat string) {
	if esGrupoChat(chat) || s.envio == nil {
		return
	}
	s.avisosViejosMu.Lock()
	if s.avisosViejos == nil || len(s.avisosViejos) > 500 {
		s.avisosViejos = map[string]time.Time{}
	}
	if t, ok := s.avisosViejos[chat]; ok && time.Since(t) < 2*time.Minute {
		s.avisosViejosMu.Unlock()
		return
	}
	s.avisosViejos[chat] = time.Now()
	s.avisosViejosMu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
		defer cancel()
		_, _ = s.envio.EnviarAuto(ctx, chat, "Ese menú ya no está activo. Escribe *!menu* para ver las opciones.", "", nil)
	}()
}
