package envio

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"botchat/internal/almacen"
	"botchat/internal/identidad"

	"go.mau.fi/whatsmeow"
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
	if _, err := s.Alm.GuardarMensaje(m); err != nil {
		return Resultado{}, err
	}
	s.RegistrarEnvio(m.ID)
	s.recordarAutor(m.ID, citaAutorWA)
	res := Resultado{ID: m.ID, Estado: "fallido", TS: m.TS}
	if !s.Cli.Conectado() {
		_ = s.Alm.EstadoDeEnvio(s.BotID, chat, m.ID, "fallido")
		return res, ErrSinConexion
	}
	var ci *waE2E.ContextInfo
	if m.CitaID != "" {
		ci = contexto(m.CitaID, s.autorDeCita(ctx, j, m), m.CitaTexto, nil, false)
	}
	if _, err := s.Cli.SendMessage(ctx, j, armar(ci), whatsmeow.SendRequestExtra{ID: types.MessageID(m.ID)}); err != nil {
		_ = s.Alm.EstadoDeEnvio(s.BotID, chat, m.ID, "fallido")
		s.Log("No se pudo enviar un mensaje: " + err.Error())
		return res, err
	}
	_ = s.Alm.EstadoDeEnvio(s.BotID, chat, m.ID, "enviado")
	s.Log("Mensaje enviado (automático) al chat …" + sufijo(chat) + ".")
	res.Estado = "enviado"
	return res, nil
}
