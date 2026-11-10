// Package envio manda lo que se escribe a mano desde la app (texto, citas, menciones, reacciones, ediciones, borrados, reenvíos,
// acuses de lectura y "escribiendo…"). No es el bot: el bot automático usa su propia cola. Aquí no hay retardos artificiales,
// solo un tope alto contra ráfagas accidentales.
package envio

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"botchat/internal/almacen"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// Cliente es lo que se usa de whatsmeow (para poder probar sin red).
type Cliente interface {
	GenerateMessageID() types.MessageID
	SendMessage(ctx context.Context, to types.JID, message *waE2E.Message, extra ...whatsmeow.SendRequestExtra) (whatsmeow.SendResponse, error)
	BuildReaction(chat, sender types.JID, id types.MessageID, reaction string) *waE2E.Message
	BuildEdit(chat types.JID, id types.MessageID, newContent *waE2E.Message) *waE2E.Message
	BuildRevoke(chat, sender types.JID, id types.MessageID) *waE2E.Message
	MarkRead(ctx context.Context, ids []types.MessageID, timestamp time.Time, chat, sender types.JID, receiptTypeExtra ...types.ReceiptType) error
	SendChatPresence(ctx context.Context, jid types.JID, state types.ChatPresence, media types.ChatPresenceMedia) error
	SendPresence(ctx context.Context, state types.Presence) error
	SubscribePresence(ctx context.Context, jid types.JID) error
	GetGroupInfo(ctx context.Context, jid types.JID) (*types.GroupInfo, error)
	UploadReader(ctx context.Context, plaintext io.Reader, tempFile io.ReadWriteSeeker, appInfo whatsmeow.MediaType) (whatsmeow.UploadResponse, error)
	ConfirmacionesLectura(ctx context.Context) bool
	Conectado() bool
	Propio() (pn types.JID, lid types.JID)
}

// Real adapta un cliente de whatsmeow.
type Real struct{ *whatsmeow.Client }

// Conectado dice si hay conexión y sesión.
func (r Real) Conectado() bool { return r.IsConnected() && r.IsLoggedIn() }

// ConfirmacionesLectura dice si la cuenta tiene activadas las confirmaciones de lectura.
func (r Real) ConfirmacionesLectura(ctx context.Context) bool {
	return r.GetPrivacySettings(ctx).ReadReceipts != types.PrivacySettingNone
}

// Propio devuelve el teléfono y el LID de la cuenta.
func (r Real) Propio() (types.JID, types.JID) {
	if r.Store == nil {
		return types.EmptyJID, types.EmptyJID
	}
	pn := types.EmptyJID
	if r.Store.ID != nil {
		pn = r.Store.ID.ToNonAD()
	}
	return pn, r.Store.LID.ToNonAD()
}

// Errores que la app muestra tal cual.
var (
	ErrSinConexion   = errors.New("No hay conexión con WhatsApp. Revisa que el bot esté encendido.")
	ErrDemasiados    = errors.New("Demasiados mensajes seguidos. Espera un momento e inténtalo de nuevo.")
	ErrMensajeVacio  = errors.New("Escribe algo antes de enviar.")
	ErrNoExiste      = errors.New("Ese mensaje ya no está en este teléfono.")
	ErrSoloPropios   = errors.New("Solo se pueden editar los mensajes que escribiste tú.")
	ErrSoloTexto     = errors.New("Solo se pueden editar mensajes de texto.")
	ErrMuchosChats   = errors.New("Puedes reenviar a 5 chats como máximo cada vez.")
	ErrChatDesconoci = errors.New("Ese chat no existe.")
)

const (
	maxTexto       = 20000
	maxReenvios    = 5
	topeRafaga     = 30               // mensajes manuales seguidos permitidos
	recargaPorSeg  = 1.0              // y luego uno por segundo
	maxEscribiendo = 60 * time.Second // "escribiendo…" continuo máximo
	minEscribiendo = 8 * time.Second  // mínimo entre dos avisos de "escribiendo…"
	vidaGrupo      = 10 * time.Minute
)

// PlazoEdicion es el plazo que la librería de WhatsApp da para editar un mensaje.
func PlazoEdicion() time.Duration { return whatsmeow.EditWindow }

// Resultado de un envío.
type Resultado struct {
	ID     string `json:"id"`
	Estado string `json:"estado"`
	TS     int64  `json:"ts"`
}

// Servicio manda mensajes manuales.
type Servicio struct {
	BotID int64
	Alm   *almacen.Almacen
	Cli   Cliente
	Log   func(string)
	Ahora func() time.Time
	// PausaBot es cuánto calla el bot en un chat después de que escribo yo (0 = no se pausa).
	PausaBot func() time.Duration

	mu        sync.Mutex
	fichas    float64
	ultimaRec time.Time
	propios   map[string]time.Time
	grupos    map[string]grupoCache
	escr      map[string]escribiendo
}

type grupoCache struct {
	info *types.GroupInfo
	en   time.Time
}

type escribiendo struct{ desde, ultimo time.Time }

// Nuevo crea el servicio.
func Nuevo(botID int64, alm *almacen.Almacen, cli Cliente, log func(string)) *Servicio {
	if log == nil {
		log = func(string) {}
	}
	return &Servicio{BotID: botID, Alm: alm, Cli: cli, Log: log, Ahora: time.Now, fichas: topeRafaga,
		propios: map[string]time.Time{}, grupos: map[string]grupoCache{}, escr: map[string]escribiendo{}}
}

func (s *Servicio) ahora() time.Time {
	if s.Ahora != nil {
		return s.Ahora()
	}
	return time.Now()
}

// tomarFicha aplica el tope contra ráfagas accidentales.
func (s *Servicio) tomarFicha() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.ahora()
	if !s.ultimaRec.IsZero() {
		s.fichas += t.Sub(s.ultimaRec).Seconds() * recargaPorSeg
		if s.fichas > topeRafaga {
			s.fichas = topeRafaga
		}
	}
	s.ultimaRec = t
	if s.fichas < 1 {
		return false
	}
	s.fichas--
	return true
}

// RegistrarEnvio recuerda un ID enviado por la app o el motor, para no confundirlo con algo escrito en el teléfono.
func (s *Servicio) RegistrarEnvio(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := s.ahora()
	s.propios[id] = t
	if len(s.propios) > 2000 {
		for k, v := range s.propios {
			if t.Sub(v) > 24*time.Hour {
				delete(s.propios, k)
			}
		}
	}
}

// EsEnvioPropio dice si un mensaje lo mandó esta app (o el motor) y no el teléfono.
func (s *Servicio) EsEnvioPropio(chat, id string) bool {
	s.mu.Lock()
	_, ok := s.propios[id]
	s.mu.Unlock()
	if ok {
		return true
	}
	m, err := s.Alm.ObtenerMensaje(s.BotID, chat, id)
	return err == nil && m != nil && (m.EstadoEnvio == "enviando" || m.EstadoEnvio == "enviado" || m.EstadoEnvio == "fallido")
}

func (s *Servicio) pausarBot(chat string) {
	if s.PausaBot == nil {
		return
	}
	if d := s.PausaBot(); d > 0 {
		_ = s.Alm.PausarBot(s.BotID, chat, s.ahora().Add(d).Unix())
	}
}

func parsea(chat string) (types.JID, error) {
	j, err := types.ParseJID(chat)
	if err != nil || j.IsEmpty() {
		return types.EmptyJID, ErrChatDesconoci
	}
	return j, nil
}

func (s *Servicio) esGrupo(j types.JID) bool { return j.Server == types.GroupServer }

// ---------- grupos ----------

func (s *Servicio) infoGrupo(ctx context.Context, j types.JID) (*types.GroupInfo, error) {
	s.mu.Lock()
	c, ok := s.grupos[j.String()]
	s.mu.Unlock()
	if ok && s.ahora().Sub(c.en) < vidaGrupo {
		return c.info, nil
	}
	if !s.Cli.Conectado() {
		if ok {
			return c.info, nil
		}
		return nil, ErrSinConexion
	}
	info, err := s.Cli.GetGroupInfo(ctx, j)
	if err != nil {
		if ok {
			return c.info, nil
		}
		return nil, err
	}
	s.mu.Lock()
	s.grupos[j.String()] = grupoCache{info: info, en: s.ahora()}
	s.mu.Unlock()
	return info, nil
}

// Participante es un miembro de un grupo, para el selector de @menciones.
type Participante struct {
	JID      string `json:"jid"` // el que se usa para mencionar (el del grupo: teléfono o LID)
	Usuario  string `json:"usuario"`
	Nombre   string `json:"nombre"`
	Telefono string `json:"telefono"`
	EsAdmin  bool   `json:"esAdmin"`
}

// Participantes lista a los miembros del grupo (sin contarte a ti).
func (s *Servicio) Participantes(ctx context.Context, chat string) ([]Participante, error) {
	j, err := parsea(chat)
	if err != nil {
		return nil, err
	}
	if !s.esGrupo(j) {
		return []Participante{}, nil
	}
	info, err := s.infoGrupo(ctx, j)
	if err != nil {
		return nil, err
	}
	pn, lid := s.Cli.Propio()
	out := make([]Participante, 0, len(info.Participants))
	for _, p := range info.Participants {
		if (!pn.IsEmpty() && (p.JID.User == pn.User || p.PhoneNumber.User == pn.User)) || (!lid.IsEmpty() && (p.JID.User == lid.User || p.LID.User == lid.User)) {
			continue
		}
		x := Participante{JID: p.JID.ToNonAD().String(), Usuario: p.JID.User, EsAdmin: p.IsAdmin || p.IsSuperAdmin}
		for _, cand := range []string{p.PhoneNumber.ToNonAD().String(), p.JID.ToNonAD().String(), p.LID.ToNonAD().String()} {
			if n := s.Alm.NombreConocido(s.BotID, chat, cand); n != "" {
				x.Nombre = n
				break
			}
		}
		if p.PhoneNumber.User != "" {
			x.Telefono = "+" + p.PhoneNumber.User
		}
		if x.Nombre == "" {
			x.Nombre = strings.TrimSpace(p.DisplayName)
		}
		if x.Nombre == "" {
			x.Nombre = x.Telefono
		}
		if x.Nombre == "" {
			x.Nombre = "Participante"
		}
		out = append(out, x)
	}
	return out, nil
}

// propioEnChat es mi JID tal como se usa en ese chat (LID si el grupo direcciona por LID).
func (s *Servicio) propioEnChat(ctx context.Context, j types.JID) types.JID {
	pn, lid := s.Cli.Propio()
	if s.esGrupo(j) && !lid.IsEmpty() {
		if info, err := s.infoGrupo(ctx, j); err == nil && info.AddressingMode == types.AddressingModeLID {
			return lid
		}
	}
	if pn.IsEmpty() {
		return lid
	}
	return pn
}

// autorEnWhatsApp lleva el autor guardado de un mensaje a la forma que WhatsApp espera en una cita o en una reacción.
func (s *Servicio) autorEnWhatsApp(ctx context.Context, chat types.JID, m *almacen.Mensaje) types.JID {
	if m.EsMio {
		return s.propioEnChat(ctx, chat)
	}
	if !s.esGrupo(chat) {
		return chat
	}
	guardado, err := types.ParseJID(m.RemitenteJID)
	if err != nil {
		return types.EmptyJID
	}
	if info, err := s.infoGrupo(ctx, chat); err == nil {
		for _, p := range info.Participants {
			if p.JID.User == guardado.User || p.PhoneNumber.User == guardado.User || p.LID.User == guardado.User {
				return p.JID.ToNonAD()
			}
		}
	}
	return guardado.ToNonAD()
}

// ---------- texto ----------

func contexto(citaID string, citaAutor types.JID, citaTexto string, menciones []string, reenviado bool) *waE2E.ContextInfo {
	if citaID == "" && len(menciones) == 0 && !reenviado {
		return nil
	}
	ci := &waE2E.ContextInfo{}
	if citaID != "" {
		ci.StanzaID = proto.String(citaID)
		if !citaAutor.IsEmpty() {
			ci.Participant = proto.String(citaAutor.ToNonAD().String())
		}
		ci.QuotedMessage = &waE2E.Message{Conversation: proto.String(citaTexto)}
	}
	if len(menciones) > 0 {
		ci.MentionedJID = menciones
	}
	if reenviado {
		ci.IsForwarded = proto.Bool(true)
		ci.ForwardingScore = proto.Uint32(1)
	}
	return ci
}

func mensajeTexto(texto string, ci *waE2E.ContextInfo) *waE2E.Message {
	if ci == nil {
		return &waE2E.Message{Conversation: proto.String(texto)}
	}
	return &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String(texto), ContextInfo: ci}}
}

// EnviarTexto escribe un mensaje en un chat. Si citaID no está vacío, responde a ese mensaje; menciones son JIDs (grupos).
// Siempre se guarda primero en el teléfono (se ve de inmediato); si el envío falla queda como "fallido" y se puede reintentar.
func (s *Servicio) EnviarTexto(ctx context.Context, chat, texto, citaID string, menciones []string) (Resultado, error) {
	return s.enviarTexto(ctx, chat, texto, citaID, menciones, true)
}

// EnviarAuto manda una respuesta del bot automático: igual que EnviarTexto, pero no usa el tope de envíos manuales,
// no pausa al bot en ese chat y se registra como automática (no como manual).
func (s *Servicio) EnviarAuto(ctx context.Context, chat, texto, citaID string, menciones []string) (Resultado, error) {
	return s.enviarTexto(ctx, chat, texto, citaID, menciones, false)
}

func (s *Servicio) enviarTexto(ctx context.Context, chat, texto, citaID string, menciones []string, manual bool) (Resultado, error) {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return Resultado{}, ErrMensajeVacio
	}
	if len([]rune(texto)) > maxTexto {
		texto = string([]rune(texto)[:maxTexto])
	}
	j, err := parsea(chat)
	if err != nil {
		return Resultado{}, err
	}
	if c, _ := s.Alm.ObtenerChat(s.BotID, chat); c == nil {
		return Resultado{}, ErrChatDesconoci
	}
	if manual && !s.tomarFicha() {
		return Resultado{}, ErrDemasiados
	}
	m := &almacen.Mensaje{
		ID: string(s.Cli.GenerateMessageID()), ChatJID: chat, BotID: s.BotID, RemitenteJID: almacen.QuienYo, EsMio: true, Tipo: almacen.TipoTexto,
		Texto: texto, TS: s.ahora().Unix(), ArchivoEstado: almacen.EstadoNinguno, EstadoEnvio: "enviando", Menciones: menciones,
	}
	if citaID != "" {
		if o, _ := s.Alm.ObtenerMensaje(s.BotID, chat, citaID); o != nil {
			m.CitaID = citaID
			if o.EsMio {
				m.CitaAutor = almacen.QuienYo
			} else {
				m.CitaAutor = o.RemitenteJID
			}
			m.CitaTexto = almacen.ResumenCorto(o)
		}
	}
	if _, err := s.Alm.GuardarMensaje(m); err != nil {
		return Resultado{}, err
	}
	s.RegistrarEnvio(m.ID)
	if manual {
		s.pausarBot(chat)
		s.Alm.RegistrarActividad(s.BotID, almacen.ActManual, "", chat, "texto")
		return s.transmitir(ctx, j, m, "manual")
	}
	return s.transmitir(ctx, j, m, "automático")
}

// transmitir manda un mensaje de texto ya guardado y actualiza su estado.
func (s *Servicio) transmitir(ctx context.Context, j types.JID, m *almacen.Mensaje, origen string) (Resultado, error) {
	res := Resultado{ID: m.ID, Estado: "fallido", TS: m.TS}
	if !s.Cli.Conectado() {
		_ = s.Alm.EstadoDeEnvio(s.BotID, m.ChatJID, m.ID, "fallido")
		return res, ErrSinConexion
	}
	var ci *waE2E.ContextInfo
	if m.CitaID != "" || len(m.Menciones) > 0 || m.Reenviado {
		var autor types.JID
		if m.CitaID != "" {
			if o, _ := s.Alm.ObtenerMensaje(s.BotID, m.ChatJID, m.CitaID); o != nil {
				autor = s.autorEnWhatsApp(ctx, j, o)
			}
		}
		ci = contexto(m.CitaID, autor, m.CitaTexto, m.Menciones, m.Reenviado)
	}
	_, err := s.Cli.SendMessage(ctx, j, mensajeTexto(m.Texto, ci), whatsmeow.SendRequestExtra{ID: types.MessageID(m.ID)})
	if err != nil {
		_ = s.Alm.EstadoDeEnvio(s.BotID, m.ChatJID, m.ID, "fallido")
		s.Log("No se pudo enviar un mensaje: " + err.Error())
		return res, err
	}
	_ = s.Alm.EstadoDeEnvio(s.BotID, m.ChatJID, m.ID, "enviado")
	s.Log("Mensaje enviado (" + origen + ") al chat …" + sufijo(m.ChatJID) + ".")
	res.Estado = "enviado"
	return res, nil
}

func sufijo(jid string) string {
	if i := strings.IndexByte(jid, '@'); i >= 0 {
		jid = jid[:i]
	}
	if len(jid) > 4 {
		return jid[len(jid)-4:]
	}
	return jid
}

// Reintentar vuelve a mandar un mensaje que quedó como "fallido" (con su mismo ID).
func (s *Servicio) Reintentar(ctx context.Context, chat, id string) (Resultado, error) {
	m, err := s.Alm.ObtenerMensaje(s.BotID, chat, id)
	if err != nil {
		return Resultado{}, err
	}
	if m == nil || !m.EsMio || m.EstadoEnvio != "fallido" {
		return Resultado{}, ErrNoExiste
	}
	j, err := parsea(chat)
	if err != nil {
		return Resultado{}, err
	}
	if !s.tomarFicha() {
		return Resultado{}, ErrDemasiados
	}
	_ = s.Alm.EstadoDeEnvio(s.BotID, chat, id, "enviando")
	m.EstadoEnvio = "enviando"
	s.RegistrarEnvio(id)
	if m.Tipo != almacen.TipoTexto {
		return s.transmitirMedio(ctx, j, m, nil)
	}
	return s.transmitir(ctx, j, m, "manual")
}

// Reaccionar pone (o quita, con emoji vacío) mi reacción a un mensaje.
func (s *Servicio) Reaccionar(ctx context.Context, chat, id, emoji string) error {
	j, err := parsea(chat)
	if err != nil {
		return err
	}
	m, err := s.Alm.ObtenerMensaje(s.BotID, chat, id)
	if err != nil {
		return err
	}
	if m == nil {
		return ErrNoExiste
	}
	if !s.Cli.Conectado() {
		return ErrSinConexion
	}
	if !s.tomarFicha() {
		return ErrDemasiados
	}
	autor := types.EmptyJID
	if !m.EsMio {
		autor = s.autorEnWhatsApp(ctx, j, m)
	}
	resp, err := s.Cli.SendMessage(ctx, j, s.Cli.BuildReaction(j, autor, types.MessageID(id), emoji))
	if err != nil {
		return err
	}
	s.RegistrarEnvio(resp.ID)
	s.pausarBot(chat)
	return s.Alm.GuardarReaccion(s.BotID, chat, id, almacen.QuienYo, emoji, s.ahora().Unix())
}

// Editar cambia el texto de un mensaje mío. WhatsApp decide si todavía está dentro del plazo; si lo rechaza se devuelve su error.
func (s *Servicio) Editar(ctx context.Context, chat, id, texto string) error {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return ErrMensajeVacio
	}
	j, err := parsea(chat)
	if err != nil {
		return err
	}
	m, err := s.Alm.ObtenerMensaje(s.BotID, chat, id)
	if err != nil {
		return err
	}
	switch {
	case m == nil:
		return ErrNoExiste
	case !m.EsMio:
		return ErrSoloPropios
	case m.Tipo != almacen.TipoTexto:
		return ErrSoloTexto
	}
	if !s.Cli.Conectado() {
		return ErrSinConexion
	}
	if !s.tomarFicha() {
		return ErrDemasiados
	}
	var ci *waE2E.ContextInfo
	if m.CitaID != "" || len(m.Menciones) > 0 {
		var autor types.JID
		if m.CitaID != "" {
			if o, _ := s.Alm.ObtenerMensaje(s.BotID, chat, m.CitaID); o != nil {
				autor = s.autorEnWhatsApp(ctx, j, o)
			}
		}
		ci = contexto(m.CitaID, autor, m.CitaTexto, m.Menciones, false)
	}
	resp, err := s.Cli.SendMessage(ctx, j, s.Cli.BuildEdit(j, types.MessageID(id), mensajeTexto(texto, ci)))
	if err != nil {
		return err
	}
	s.RegistrarEnvio(resp.ID)
	_, err = s.Alm.EditarTexto(s.BotID, chat, id, texto)
	return err
}

// BorrarParaTodos pide a WhatsApp borrar un mensaje para todos. Siempre se intenta; si WhatsApp lo rechaza se devuelve su error.
func (s *Servicio) BorrarParaTodos(ctx context.Context, chat, id string) error {
	j, err := parsea(chat)
	if err != nil {
		return err
	}
	m, err := s.Alm.ObtenerMensaje(s.BotID, chat, id)
	if err != nil {
		return err
	}
	if m == nil {
		return ErrNoExiste
	}
	if !s.Cli.Conectado() {
		return ErrSinConexion
	}
	if !s.tomarFicha() {
		return ErrDemasiados
	}
	autor := types.EmptyJID
	if !m.EsMio {
		autor = s.autorEnWhatsApp(ctx, j, m)
	}
	resp, err := s.Cli.SendMessage(ctx, j, s.Cli.BuildRevoke(j, autor, types.MessageID(id)))
	if err != nil {
		return err
	}
	s.RegistrarEnvio(resp.ID)
	por := ""
	if !m.EsMio {
		por = almacen.QuienYo
	}
	_, err = s.Alm.MarcarEliminado(s.BotID, chat, id, por, s.ahora().Unix())
	return err
}

// Reenviado dice cómo salió un reenvío.
type Reenviado struct {
	Enviados int `json:"enviados"`
	Omitidos int `json:"omitidos"` // tipos que todavía no se pueden reenviar
	Fallidos int `json:"fallidos"`
}

// ReenviarTexto manda los mensajes elegidos (texto, y fotos, videos, audios o documentos ya descargados) a hasta 5 chats con la marca "Reenviado".
func (s *Servicio) ReenviarTexto(ctx context.Context, origen string, ids, destinos []string) (Reenviado, error) {
	var r Reenviado
	if len(destinos) > maxReenvios {
		return r, ErrMuchosChats
	}
	var textos []*almacen.Mensaje
	for _, id := range ids {
		m, err := s.Alm.ObtenerMensaje(s.BotID, origen, id)
		if err != nil {
			return r, err
		}
		if !puedeReenviarse(m) {
			r.Omitidos++
			continue
		}
		textos = append(textos, m)
	}
	for _, d := range destinos {
		j, err := parsea(d)
		if err != nil {
			r.Fallidos += len(textos)
			continue
		}
		for _, o := range textos {
			if o.Tipo != almacen.TipoTexto {
				if err := s.reenviarMedio(ctx, o, d); err != nil {
					r.Fallidos++
				} else {
					r.Enviados++
				}
				continue
			}
			if !s.tomarFicha() {
				r.Fallidos++
				continue
			}
			m := &almacen.Mensaje{
				ID: string(s.Cli.GenerateMessageID()), ChatJID: d, BotID: s.BotID, RemitenteJID: almacen.QuienYo, EsMio: true, Tipo: almacen.TipoTexto,
				Texto: o.Texto, TS: s.ahora().Unix(), ArchivoEstado: almacen.EstadoNinguno, EstadoEnvio: "enviando", Reenviado: true,
			}
			if c, _ := s.Alm.ObtenerChat(s.BotID, d); c == nil {
				r.Fallidos++
				continue
			}
			if _, err := s.Alm.GuardarMensaje(m); err != nil {
				r.Fallidos++
				continue
			}
			s.RegistrarEnvio(m.ID)
			s.pausarBot(d)
			if res, err := s.transmitir(ctx, j, m, "manual"); err != nil || res.Estado != "enviado" {
				r.Fallidos++
			} else {
				r.Enviados++
			}
		}
	}
	return r, nil
}

// ---------- lectura y presencia ----------

// MarcarLeido avisa "leído" de lo recibido en un chat. Si en la cuenta las confirmaciones de lectura están desactivadas,
// whatsmeow manda "read-self" (nadie se entera): esto nunca las envía a escondidas.
func (s *Servicio) MarcarLeido(ctx context.Context, chat string) error {
	j, err := parsea(chat)
	if err != nil {
		return err
	}
	if !s.Cli.Conectado() {
		return ErrSinConexion
	}
	grupos, err := s.Alm.PendientesDeLeido(s.BotID, chat)
	if err != nil {
		return err
	}
	var ultimo int64
	var fallo error
	for _, g := range grupos {
		ids := make([]types.MessageID, len(g.IDs))
		for i, id := range g.IDs {
			ids[i] = types.MessageID(id)
		}
		sender := types.EmptyJID
		if s.esGrupo(j) {
			if sj, err := types.ParseJID(g.Remitente); err == nil {
				sender = sj
			}
		}
		if err := s.Cli.MarkRead(ctx, ids, time.Unix(g.UltimoTS, 0), j, sender); err != nil {
			fallo = err
			continue
		}
		if g.UltimoTS > ultimo {
			ultimo = g.UltimoTS
		}
	}
	if ultimo > 0 && fallo == nil {
		_ = s.Alm.MarcarLeidoHasta(s.BotID, chat, ultimo)
	}
	return fallo
}

// Escribiendo manda (o quita) el aviso "escribiendo…" mientras se teclea, con tope: un aviso cada 8 s y como mucho 60 s seguidos.
func (s *Servicio) Escribiendo(ctx context.Context, chat string, si bool) error {
	j, err := parsea(chat)
	if err != nil {
		return err
	}
	if !s.Cli.Conectado() {
		return nil
	}
	t := s.ahora()
	s.mu.Lock()
	e, activo := s.escr[chat]
	if !si {
		delete(s.escr, chat)
		s.mu.Unlock()
		if activo {
			return s.Cli.SendChatPresence(ctx, j, types.ChatPresencePaused, types.ChatPresenceMediaText)
		}
		return nil
	}
	if activo {
		if t.Sub(e.desde) > maxEscribiendo || t.Sub(e.ultimo) < minEscribiendo {
			s.mu.Unlock()
			return nil
		}
		e.ultimo = t
	} else {
		e = escribiendo{desde: t, ultimo: t}
	}
	s.escr[chat] = e
	s.mu.Unlock()
	return s.Cli.SendChatPresence(ctx, j, types.ChatPresenceComposing, types.ChatPresenceMediaText)
}

// EnLinea marca la cuenta como disponible o no (solo se usa con un chat abierto y si el usuario lo permitió).
func (s *Servicio) EnLinea(ctx context.Context, si bool) error {
	if !s.Cli.Conectado() {
		return nil
	}
	if si {
		return s.Cli.SendPresence(ctx, types.PresenceAvailable)
	}
	return s.Cli.SendPresence(ctx, types.PresenceUnavailable)
}

// Suscribir pide a WhatsApp la presencia ("en línea", "últ. vez") de un contacto; solo llega si esa persona lo permite.
func (s *Servicio) Suscribir(ctx context.Context, chat string) error {
	j, err := parsea(chat)
	if err != nil || s.esGrupo(j) || !s.Cli.Conectado() {
		return err
	}
	return s.Cli.SubscribePresence(ctx, j)
}
