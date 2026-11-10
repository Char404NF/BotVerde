package wabot

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/envio"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// pausaTrasEscribirMin: minutos que el bot calla en un chat después de que escribo yo (ajustable en Avanzado; 0 = no se pausa).
var pausaTrasEscribirMin atomic.Int32

func init() { pausaTrasEscribirMin.Store(30) }

// ConfigurarEnvio fija cuántos minutos calla el bot en un chat después de que escribo yo a mano (0 = nunca se pausa).
func ConfigurarEnvio(pausaBotMin int) { pausaTrasEscribirMin.Store(int32(pausaBotMin)) }

func pausaTrasEscribir() time.Duration {
	return time.Duration(pausaTrasEscribirMin.Load()) * time.Minute
}

func servicioEnvio(botID int64) (*envio.Servicio, error) {
	mu.Lock()
	s := st
	mu.Unlock()
	if s == nil || s.botID != botID || s.envio == nil {
		return nil, errors.New("Enciende el bot para enviar mensajes.")
	}
	return s.envio, nil
}

func ctxEnvio() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 90*time.Second)
}

// EnviarTexto escribe un mensaje a mano en un chat (no es el bot). citaID es el mensaje al que se responde ("" = ninguno);
// mencionesJSON es un arreglo JSON de JIDs mencionados (grupos). Devuelve (JSON) {id, estado, ts}; si falla, el mensaje queda
// guardado como "fallido" para poder reintentarlo.
func EnviarTexto(botID int64, chat, texto, citaID, mencionesJSON string) (string, error) {
	sv, err := servicioEnvio(botID)
	if err != nil {
		return "", err
	}
	var men []string
	if mencionesJSON != "" {
		_ = json.Unmarshal([]byte(mencionesJSON), &men)
	}
	ctx, cancel := ctxEnvio()
	defer cancel()
	r, err := sv.EnviarTexto(ctx, chat, texto, citaID, men)
	return js(r, err)
}

// ReintentarEnvio vuelve a mandar un mensaje que quedó como "fallido".
func ReintentarEnvio(botID int64, chat, id string) (string, error) {
	sv, err := servicioEnvio(botID)
	if err != nil {
		return "", err
	}
	ctx, cancel := ctxEnvio()
	defer cancel()
	r, err := sv.Reintentar(ctx, chat, id)
	return js(r, err)
}

// Reaccionar pone mi reacción (emoji; vacío = quitarla) a un mensaje.
func Reaccionar(botID int64, chat, id, emoji string) error {
	sv, err := servicioEnvio(botID)
	if err != nil {
		return err
	}
	ctx, cancel := ctxEnvio()
	defer cancel()
	return sv.Reaccionar(ctx, chat, id, emoji)
}

// EditarMensaje cambia el texto de un mensaje mío. Si WhatsApp lo rechaza, devuelve su error.
func EditarMensaje(botID int64, chat, id, texto string) error {
	sv, err := servicioEnvio(botID)
	if err != nil {
		return err
	}
	ctx, cancel := ctxEnvio()
	defer cancel()
	return sv.Editar(ctx, chat, id, texto)
}

// PlazoEdicionSeg es el plazo (en segundos) que la librería da para editar un mensaje.
func PlazoEdicionSeg() int { return int(envio.PlazoEdicion().Seconds()) }

// BorrarParaTodos pide a WhatsApp borrar un mensaje para todos. Si lo rechaza, devuelve su error.
func BorrarParaTodos(botID int64, chat, id string) error {
	sv, err := servicioEnvio(botID)
	if err != nil {
		return err
	}
	ctx, cancel := ctxEnvio()
	defer cancel()
	return sv.BorrarParaTodos(ctx, chat, id)
}

// BorrarParaMi quita mensajes de este teléfono (idsJSON: arreglo de IDs). No necesita conexión.
func BorrarParaMi(botID int64, chat, idsJSON string) (int, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	var ids []string
	if err := json.Unmarshal([]byte(idsJSON), &ids); err != nil {
		return 0, err
	}
	return a.BorrarParaMi(botID, chat, ids)
}

// ReenviarTexto manda mensajes de texto a hasta 5 chats con la marca "Reenviado". Devuelve (JSON) {enviados, omitidos, fallidos}.
func ReenviarTexto(botID int64, origen, idsJSON, destinosJSON string) (string, error) {
	sv, err := servicioEnvio(botID)
	if err != nil {
		return "", err
	}
	var ids, dest []string
	if err := json.Unmarshal([]byte(idsJSON), &ids); err != nil {
		return "", err
	}
	if err := json.Unmarshal([]byte(destinosJSON), &dest); err != nil {
		return "", err
	}
	ctx, cancel := ctxEnvio()
	defer cancel()
	r, err := sv.ReenviarTexto(ctx, origen, ids, dest)
	return js(r, err)
}

// MarcarLeido avisa "leído" de lo recibido en un chat (respeta la configuración de confirmaciones de lectura de la cuenta).
func MarcarLeido(botID int64, chat string) error {
	sv, err := servicioEnvio(botID)
	if err != nil {
		return err
	}
	ctx, cancel := ctxEnvio()
	defer cancel()
	return sv.MarcarLeido(ctx, chat)
}

// Escribiendo avisa (o quita) "escribiendo…" mientras tecleo, con tope.
func Escribiendo(botID int64, chat string, si bool) error {
	sv, err := servicioEnvio(botID)
	if err != nil {
		return nil // sin conexión no hay nada que avisar
	}
	ctx, cancel := ctxEnvio()
	defer cancel()
	return sv.Escribiendo(ctx, chat, si)
}

// EnLinea marca la cuenta como disponible o no disponible.
func EnLinea(botID int64, si bool) error {
	sv, err := servicioEnvio(botID)
	if err != nil {
		return nil
	}
	ctx, cancel := ctxEnvio()
	defer cancel()
	return sv.EnLinea(ctx, si)
}

// SuscribirPresencia pide la presencia de un contacto (solo llega si esa persona lo permite).
func SuscribirPresencia(botID int64, chat string) error {
	sv, err := servicioEnvio(botID)
	if err != nil {
		return nil
	}
	ctx, cancel := ctxEnvio()
	defer cancel()
	return sv.Suscribir(ctx, chat)
}

// ParticipantesGrupo lista (JSON) a los miembros de un grupo para elegir a quién mencionar.
func ParticipantesGrupo(botID int64, chat string) (string, error) {
	sv, err := servicioEnvio(botID)
	if err != nil {
		return "", err
	}
	ctx, cancel := ctxEnvio()
	defer cancel()
	return js(sv.Participantes(ctx, chat))
}

// InfoMensaje devuelve (JSON) las horas de entrega y lectura de un mensaje mío (en grupos, por persona).
func InfoMensaje(botID int64, chat, id string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.InfoDeMensaje(botID, chat, id))
}

// GuardarBorrador guarda (cifrado) lo que se estaba escribiendo en un chat; vacío lo borra.
func GuardarBorrador(botID int64, chat, texto string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.GuardarBorrador(botID, chat, texto)
}

// LeerBorrador devuelve el borrador de un chat ("" si no hay).
func LeerBorrador(botID int64, chat string) string {
	a, err := almacenAbierto()
	if err != nil {
		return ""
	}
	return a.Borrador(botID, chat)
}

// SilenciarChat enciende o apaga las notificaciones de un chat.
func SilenciarChat(botID int64, chat string, si bool) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.Silenciar(botID, chat, si)
}

// EstadoDelChat devuelve (JSON) lo guardado de un chat: silenciado, pausa del bot, etc.
func EstadoDelChat(botID int64, chat string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.EstadoDeChat(botID, chat), nil)
}

// ---------- presencia en vivo (escribiendo, en línea, última vez) ----------

type presenciaChat struct {
	Escribiendo bool  `json:"escribiendo"`
	EnLinea     bool  `json:"enLinea"`
	UltimaVez   int64 `json:"ultimaVez"` // 0 = no se sabe (la persona no lo permite)
	desde       time.Time
	visto       bool
}

var (
	presMu sync.Mutex
	pres   = map[string]*presenciaChat{}
)

const vidaEscribiendo = 12 * time.Second

func presenciaPara(chat string) *presenciaChat {
	p := pres[chat]
	if p == nil {
		p = &presenciaChat{}
		pres[chat] = p
	}
	return p
}

// PresenciaDe devuelve (JSON) lo que se sabe en vivo de un chat: {escribiendo, enLinea, ultimaVez}. Solo trae datos cuando
// la otra persona lo permite y se pidió con SuscribirPresencia; si no llega nada, todo viene apagado.
func PresenciaDe(chat string) string {
	presMu.Lock()
	defer presMu.Unlock()
	p := pres[chat]
	if p == nil {
		return `{"escribiendo":false,"enLinea":false,"ultimaVez":0}`
	}
	q := *p
	if q.Escribiendo && time.Since(q.desde) > vidaEscribiendo {
		q.Escribiendo = false
	}
	b, _ := json.Marshal(q)
	return string(b)
}

func (s *estado) alRecibirPresencia(v *events.Presence) {
	s.encola(func() {
		chat := s.ing.Persona(s.ctx, v.From)
		presMu.Lock()
		p := presenciaPara(chat)
		p.EnLinea = !v.Unavailable
		if !v.LastSeen.IsZero() {
			p.UltimaVez = v.LastSeen.Unix()
		}
		presMu.Unlock()
		s.avisaCambio("presencia:" + chat)
	})
}

func (s *estado) alRecibirChatPresencia(v *events.ChatPresence) {
	s.encola(func() {
		chat, ok := s.ing.ChatDe(s.ctx, v.MessageSource)
		if !ok || v.IsFromMe {
			return
		}
		presMu.Lock()
		p := presenciaPara(chat)
		p.Escribiendo = v.State == types.ChatPresenceComposing
		p.desde = time.Now()
		presMu.Unlock()
		s.avisaCambio("presencia:" + chat)
		if v.State == types.ChatPresenceComposing {
			time.AfterFunc(vidaEscribiendo+time.Second, func() { s.avisaCambio("presencia:" + chat) })
		}
	})
}

func (s *estado) avisaCambio(ev string) {
	if g := agrup; g != nil {
		g.Agregar(ev)
	}
}

// alRecibirAcuse guarda los acuses de entrega, lectura y reproducción de mis mensajes.
func (s *estado) alRecibirAcuse(v *events.Receipt) {
	var tipo string
	switch v.Type {
	case types.ReceiptTypeDelivered:
		tipo = almacen.AcuseEntregado
	case types.ReceiptTypeRead:
		tipo = almacen.AcuseLeido
	case types.ReceiptTypePlayed:
		tipo = almacen.AcuseReproducido
	default:
		return
	}
	if v.IsFromMe {
		return // acuses míos desde otro dispositivo
	}
	s.encola(func() {
		chat, ok := s.ing.ChatDe(s.ctx, v.MessageSource)
		if !ok {
			return
		}
		quien := chat
		if v.IsGroup {
			quien = s.ing.Persona(s.ctx, v.Sender)
		}
		ts := v.Timestamp.Unix()
		for _, id := range v.MessageIDs {
			_, _ = alm.GuardarAcuse(s.botID, chat, string(id), quien, tipo, ts)
		}
	})
}
