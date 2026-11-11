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
		return true
	}
	if sincronizando {
		return true // un voto que llegó mientras el teléfono estaba sin conexión no navega el menú
	}
	chat := e.Sesion.Chat
	m := motor.Mensaje{
		Chat: chat, ID: string(v.Info.ID), Texto: strconv.Itoa(e.Num), EsTexto: true, EsGrupo: esGrupoChat(chat),
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
	Activo         bool         `json:"activo"`
	AvisoAceptado  bool         `json:"avisoAceptado"`
	AvisoTS        int64        `json:"avisoTs"`
	Modo           string       `json:"modo"`     // lo elegido (texto, encuesta, botones)
	ModoEfectivo   string       `json:"modoReal"` // lo que vale ahora (texto si el Laboratorio está apagado)
	Grupos         bool         `json:"grupos"`
	EncMax         int          `json:"encMax"`
	ExpiraMin      int          `json:"expiraMin"`
	RespaldoEnc    bool         `json:"respaldoEnc"`
	RespaldoBot    bool         `json:"respaldoBot"`
	BotonesTipo    string       `json:"botonesTipo"`
	PruebaChat     string       `json:"pruebaChat"`
	PruebaHora     int          `json:"pruebaHora"`
	PruebaDia      int          `json:"pruebaDia"`
	VersionWA      string       `json:"versionWa"`
	VersionCliente string       `json:"versionCliente"`
	EnMarcha       bool         `json:"enMarcha"`
	LimiteMedido   int          `json:"limiteMedido"`
	PruebasHora    int          `json:"pruebasHora"`
	PruebasDia     int          `json:"pruebasDia"`
	Plan           []lab.Prueba `json:"plan"`
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
		BotonesTipo: aj("lab_botones_tipo"), PruebaChat: aj("lab_prueba_chat"), PruebaHora: n("lab_prueba_hora"), PruebaDia: n("lab_prueba_dia"),
		VersionWA: aj("lab_version_wa"), Plan: lab.Plan(),
	}
	e.ModoEfectivo = lab.ModoTexto
	if e.Activo && e.AvisoAceptado {
		e.ModoEfectivo = e.Modo
	}
	ahora := time.Now()
	e.PruebasHora = a.ContarEnviosLab(botID, ahora.Add(-time.Hour))
	e.PruebasDia = a.ContarEnviosLab(botID, ahora.Add(-24*time.Hour))
	e.LimiteMedido = lab.LimiteMedido(a.ListarResultadosLab(botID, 500))
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
		if valor != lab.ModoTexto && !aviso {
			return lab.ErrSinAviso
		}
	case "lab_grupos", "lab_respaldo_enc", "lab_respaldo_bot":
		if err := num(0, 1); err != nil {
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
	case "lab_prueba_hora", "lab_prueba_dia":
		if err := num(1, 100); err != nil {
			return err
		}
	case "lab_prueba_chat":
		if valor != "" {
			if _, e := types.ParseJID(valor); e != nil {
				return errors.New("ese chat no es válido")
			}
			if c, _ := a.ObtenerChat(botID, valor); c == nil {
				return errors.New("ese chat todavía no existe: escríbele algo desde el teléfono o ábrelo primero")
			}
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

// EnviarPruebaLab manda UNA prueba al chat de prueba y la anota en la matriz. Devuelve (JSON) el renglón de la matriz.
func EnviarPruebaLab(botID int64, tipo string, variante int) (string, error) {
	l, err := laboratorio(botID)
	if err != nil {
		return "", err
	}
	ctx, cancel := ctxEnvio()
	defer cancel()
	r, err := l.EnviarPrueba(ctx, tipo, variante)
	accionDueno(botID, "lab.prueba", tipo, "variante="+strconv.Itoa(variante))
	if err != nil && r.ID == 0 {
		return "", err
	}
	// Un envío fallido ya quedó anotado con su error: se devuelve el renglón para que la matriz lo muestre.
	return js(r, nil)
}

// ListarResultadosLab devuelve (JSON) la matriz, de la prueba más nueva a la más vieja:
// [{id, ts, tipo, variante, chat, msgId, enviado, llego, visible, tocable, respondio, versionWa, versionCliente, error}].
// Los números son -1 (sin saber), 0 (no) y 1 (sí).
func ListarResultadosLab(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ListarResultadosLab(botID, 200), nil)
}

// MarcarResultadoLab anota lo que ves en el teléfono: campo "visible" o "tocable"; valor -1 (sin saber), 0 (no) o 1 (sí).
func MarcarResultadoLab(botID, id int64, campo string, valor int) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	if campo != "visible" && campo != "tocable" {
		return errors.New("solo puedes marcar «visible» y «tocable»: lo demás lo anota el bot")
	}
	return a.MarcarResultadoLab(botID, id, campo, valor)
}

// BorrarResultadosLab vacía la matriz. Devuelve cuántas pruebas había.
func BorrarResultadosLab(botID int64) (int, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	accionDueno(botID, "lab.borrar", "", "")
	return a.BorrarResultadosLab(botID), nil
}

// UsarLimiteMedido pone como tope de opciones de las encuestas el límite que midieron tus pruebas. Devuelve el nuevo tope.
func UsarLimiteMedido(botID int64) (int, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	n := lab.LimiteMedido(a.ListarResultadosLab(botID, 500))
	if n < 2 {
		return 0, errors.New("todavía no marcaste ninguna encuesta como «visible»")
	}
	if err := a.FijarAjusteBot(botID, "lab_enc_max_opc", strconv.Itoa(n)); err != nil {
		return 0, err
	}
	accionDueno(botID, "lab.limite", "", "n="+strconv.Itoa(n))
	return n, nil
}
