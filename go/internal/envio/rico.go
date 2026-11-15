package envio

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"

	"botchat/internal/almacen"
	"botchat/internal/identidad"

	"go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// Respuestas del bot con ubicación o tarjeta de contacto, y con el autor exacto del mensaje que se cita (menús en grupos).
//
// El Participant de una cita debe ser el JID del autor TAL COMO LO MANDÓ WhatsApp (LID en los grupos que direccionan por LID):
// el motor lo trae de `RemitenteWA` y aquí se usa sin cambiarlo. Si no se da, se busca en el grupo como siempre (autorEnWhatsApp).

// recordarAutor guarda el autor exacto de la cita para el mensaje `id` hasta que se transmita.
func (s *Servicio) recordarAutor(id, autor string) {
	if autor == "" {
		return
	}
	s.mu.Lock()
	s.autores[id] = autor
	s.mu.Unlock()
}

// autorDeCita es el autor del mensaje citado: el que se dio tal cual (si se dio) o el que resulta de buscarlo en el grupo.
func (s *Servicio) autorDeCita(ctx context.Context, j types.JID, m *almacen.Mensaje) types.JID {
	if m.CitaID == "" {
		return types.EmptyJID
	}
	s.mu.Lock()
	ov := s.autores[m.ID]
	delete(s.autores, m.ID)
	s.mu.Unlock()
	if ov != "" {
		if pj, err := types.ParseJID(ov); err == nil && !pj.IsEmpty() {
			return pj.ToNonAD()
		}
	}
	if o, _ := s.Alm.ObtenerMensaje(s.BotID, m.ChatJID, m.CitaID); o != nil {
		return s.autorEnWhatsApp(ctx, j, o)
	}
	return types.EmptyJID
}

// EnviarAutoCita es EnviarAuto citando a `citaAutorWA` (el JID de quien escribió el mensaje citado, tal como lo mandó WhatsApp).
func (s *Servicio) EnviarAutoCita(ctx context.Context, chat, texto, citaID, citaAutorWA string, menciones []string) (Resultado, error) {
	return s.enviarTexto(ctx, chat, texto, citaID, citaAutorWA, menciones, false)
}

// EnviarMedioAutoCita es EnviarMedioAuto citando con el autor exacto.
func (s *Servicio) EnviarMedioAutoCita(ctx context.Context, chat string, a Adjunto, citaID, citaAutorWA string, menciones []string) (Resultado, error) {
	return s.enviarMedio(ctx, chat, a, citaID, citaAutorWA, menciones, false)
}

// vCard arma la tarjeta de contacto (versión 3.0) con el teléfono para que WhatsApp lo reconozca (waid).
func vCard(nombre, telefono string) string {
	nombre = strings.NewReplacer("\n", " ", "\r", " ", ";", ",").Replace(strings.TrimSpace(nombre))
	dig := identidad.Digitos(telefono)
	return "BEGIN:VCARD\nVERSION:3.0\nFN:" + nombre + "\nTEL;type=CELL;waid=" + dig + ":+" + dig + "\nEND:VCARD"
}

// EnviarUbicacionAuto manda un punto del mapa como respuesta del bot.
func (s *Servicio) EnviarUbicacionAuto(ctx context.Context, chat string, lat, lon float64, nombre, direccion, citaID, citaAutorWA string) (Resultado, error) {
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return Resultado{}, fmt.Errorf("La ubicación no es válida.")
	}
	vista, _ := json.Marshal(almacen.UbicacionVista{V: 1, Lat: lat, Lon: lon, Nombre: strings.TrimSpace(nombre), Dir: strings.TrimSpace(direccion)})
	msg := &waE2E.LocationMessage{DegreesLatitude: proto.Float64(lat), DegreesLongitude: proto.Float64(lon)}
	if n := strings.TrimSpace(nombre); n != "" {
		msg.Name = proto.String(n)
	}
	if d := strings.TrimSpace(direccion); d != "" {
		msg.Address = proto.String(d)
	}
	return s.enviarEspecial(ctx, chat, almacen.TipoUbicacion, string(vista), citaID, citaAutorWA, func(ci *waE2E.ContextInfo) *waE2E.Message {
		msg.ContextInfo = ci
		return &waE2E.Message{LocationMessage: msg}
	})
}

// EnviarContactoAuto manda una tarjeta de contacto como respuesta del bot.
func (s *Servicio) EnviarContactoAuto(ctx context.Context, chat, nombre, telefono, citaID, citaAutorWA string) (Resultado, error) {
	nombre = strings.TrimSpace(nombre)
	if nombre == "" || len(identidad.Digitos(telefono)) < 7 {
		return Resultado{}, fmt.Errorf("El contacto necesita un nombre y un teléfono.")
	}
	vista, _ := json.Marshal(almacen.ContactoVista{V: 1, Contacts: []almacen.ContactoUn{{Nombre: nombre, Tels: []string{"+" + identidad.Digitos(telefono)}}}})
	card := vCard(nombre, telefono)
	return s.enviarEspecial(ctx, chat, almacen.TipoContacto, string(vista), citaID, citaAutorWA, func(ci *waE2E.ContextInfo) *waE2E.Message {
		return &waE2E.Message{ContactMessage: &waE2E.ContactMessage{DisplayName: proto.String(nombre), Vcard: proto.String(card), ContextInfo: ci}}
	})
}

// enviarEspecial guarda primero el mensaje (se ve en el chat) y lo manda; `armar` construye el mensaje de WhatsApp con la cita.
func (s *Servicio) enviarEspecial(ctx context.Context, chat, tipo, texto, citaID, citaAutorWA string, armar func(ci *waE2E.ContextInfo) *waE2E.Message) (Resultado, error) {
	return s.enviarEspecialID(ctx, chat, tipo, texto, citaID, citaAutorWA, func(_ string, ci *waE2E.ContextInfo) *waE2E.Message { return armar(ci) }, nil, nil)
}

// EnviarInteractivo manda un mensaje interactivo del Laboratorio (encuesta, botones, lista) como automático. En el chat se guarda
// como texto (`resumen`: el menú numerado). `armar` recibe el id del mensaje y la cita; `antes` (opcional) se llama con el id y el
// mensaje ya armado justo antes de enviar: si devuelve error no se envía. Así el Laboratorio guarda su sesión antes de que pueda
// llegar la primera respuesta.
func (s *Servicio) EnviarInteractivo(ctx context.Context, chat, resumen, citaID, citaAutorWA string, armar func(id string, ci *waE2E.ContextInfo) *waE2E.Message, antes func(id string, m *waE2E.Message) error) (Resultado, error) {
	return s.enviarEspecialID(ctx, chat, almacen.TipoTexto, resumen, citaID, citaAutorWA, armar, antes, nil)
}

// EnviarInteractivoNodos es EnviarInteractivo con nodos adicionales en el envío (SendRequestExtra.AdditionalNodes). Solo lo usa la
// prueba experimental del Laboratorio "Interactivo + biz" (pedida por el dueño); el menú real nunca pasa por aquí.
func (s *Servicio) EnviarInteractivoNodos(ctx context.Context, chat, resumen, citaID, citaAutorWA string, armar func(id string, ci *waE2E.ContextInfo) *waE2E.Message, antes func(id string, m *waE2E.Message) error, nodos []waBinary.Node) (Resultado, error) {
	return s.enviarEspecialID(ctx, chat, almacen.TipoTexto, resumen, citaID, citaAutorWA, armar, antes, nodos)
}

func (s *Servicio) enviarEspecialID(ctx context.Context, chat, tipo, texto, citaID, citaAutorWA string, armar func(id string, ci *waE2E.ContextInfo) *waE2E.Message, antes func(id string, m *waE2E.Message) error, nodos []waBinary.Node) (Resultado, error) {
	j, err := parsea(chat)
	if err != nil {
		return Resultado{}, err
	}
	if c, _ := s.Alm.ObtenerChat(s.BotID, chat); c == nil {
		return Resultado{}, ErrChatDesconoci
	}
	m := &almacen.Mensaje{
		ID: string(s.Cli.GenerateMessageID()), ChatJID: chat, BotID: s.BotID, RemitenteJID: almacen.QuienYo, EsMio: true, Tipo: tipo,
		Texto: texto, TS: s.ahora().Unix(), ArchivoEstado: almacen.EstadoNinguno, EstadoEnvio: "enviando",
	}
	if citaID != "" {
		s.ponerCita(m, citaID)
	}
	m.Automatico = etiquetaAuto(ctx)
	res := Resultado{ID: m.ID, Estado: "fallido", TS: m.TS}
	if !s.Cli.Conectado() {
		if _, err := s.Alm.GuardarMensaje(m); err == nil {
			_ = s.Alm.EstadoDeEnvio(s.BotID, chat, m.ID, "fallido")
		}
		return res, ErrSinConexion
	}
	var ci *waE2E.ContextInfo
	if m.CitaID != "" {
		ci = contexto(m.CitaID, s.autorDeCita(ctx, j, m), m.CitaTexto, nil, false)
	}
	wm := armar(m.ID, ci)
	if antes != nil {
		if err := antes(m.ID, wm); err != nil {
			return res, err
		}
	}
	if _, err := s.Alm.GuardarMensaje(m); err != nil {
		return Resultado{}, err
	}
	s.RegistrarEnvio(m.ID)
	s.recordarAutor(m.ID, citaAutorWA)
	extra := whatsmeow.SendRequestExtra{ID: types.MessageID(m.ID)}
	if len(nodos) > 0 {
		extra.AdditionalNodes = &nodos
	}
	if _, err := s.Cli.SendMessage(ctx, j, wm, extra); err != nil {
		_ = s.Alm.EstadoDeEnvio(s.BotID, chat, m.ID, "fallido")
		s.Log("No se pudo enviar un mensaje: " + err.Error())
		return res, err
	}
	_ = s.Alm.EstadoDeEnvio(s.BotID, chat, m.ID, "enviado")
	s.Log("Mensaje enviado (automático) al chat …" + sufijo(chat) + ".")
	res.Estado = "enviado"
	return res, nil
}

// ---------- encuestas y mensajes fijados ----------

// EnviarEncuesta manda una encuesta al chat como mensaje automático. `multiple` deja elegir varias opciones. En el chat se guarda como
// texto (la pregunta y sus opciones numeradas).
func (s *Servicio) EnviarEncuesta(ctx context.Context, chat, titulo string, opciones []string, multiple bool) (Resultado, error) {
	titulo = strings.TrimSpace(titulo)
	if titulo == "" || len(opciones) < 2 || len(opciones) > 12 {
		return Resultado{}, ErrEncuesta
	}
	var b strings.Builder
	b.WriteString("📊 " + titulo)
	for i, o := range opciones {
		fmt.Fprintf(&b, "\n%d. %s", i+1, o)
	}
	n := 1
	if multiple {
		n = len(opciones)
	}
	return s.EnviarInteractivo(ctx, chat, b.String(), "", "", func(_ string, _ *waE2E.ContextInfo) *waE2E.Message {
		return construirEncuesta(titulo, opciones, n)
	}, nil)
}

// construirEncuesta arma el mensaje de encuesta (lo mismo que hace whatsmeow, sin necesitar el cliente).
func construirEncuesta(titulo string, opciones []string, seleccionables int) *waE2E.Message {
	ops := make([]*waE2E.PollCreationMessage_Option, len(opciones))
	for i, o := range opciones {
		ops[i] = &waE2E.PollCreationMessage_Option{OptionName: proto.String(o)}
	}
	secreto := make([]byte, 32)
	_, _ = rand.Read(secreto)
	return &waE2E.Message{
		PollCreationMessage: &waE2E.PollCreationMessage{Name: proto.String(titulo), Options: ops, SelectableOptionsCount: proto.Uint32(uint32(seleccionables))},
		MessageContextInfo:  &waE2E.MessageContextInfo{MessageSecret: secreto},
	}
}

// FijarMensaje fija (o suelta) un mensaje del chat para todos durante `segundos` (1 día, 7 días o 30 días). Solo si el bot administra
// el grupo o el grupo deja fijar a todos: si no, WhatsApp lo rechaza.
func (s *Servicio) FijarMensaje(ctx context.Context, chat, id string, segundos int, fijar bool) error {
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
	autor := s.autorEnWhatsApp(ctx, j, m)
	clave := &waCommon.MessageKey{RemoteJID: proto.String(j.String()), FromMe: proto.Bool(m.EsMio), ID: proto.String(id)}
	if !m.EsMio && autor.User != "" && s.esGrupo(j) {
		clave.Participant = proto.String(autor.String())
	}
	tipo := waE2E.PinInChatMessage_PIN_FOR_ALL
	msg := &waE2E.Message{PinInChatMessage: &waE2E.PinInChatMessage{Key: clave, Type: tipo.Enum(), SenderTimestampMS: proto.Int64(s.ahora().UnixMilli())}}
	if fijar {
		if segundos != 86400 && segundos != 604800 && segundos != 2592000 {
			segundos = 604800
		}
		msg.MessageContextInfo = &waE2E.MessageContextInfo{MessageAddOnDurationInSecs: proto.Uint32(uint32(segundos))}
	} else {
		msg.PinInChatMessage.Type = waE2E.PinInChatMessage_UNPIN_FOR_ALL.Enum()
	}
	resp, err := s.Cli.SendMessage(ctx, j, msg)
	if err != nil {
		return err
	}
	s.RegistrarEnvio(resp.ID)
	s.pausarBot(chat)
	return nil
}
