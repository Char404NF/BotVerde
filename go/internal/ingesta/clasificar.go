// Package ingesta convierte los eventos de WhatsApp en registros del almacén: clasifica cada mensaje,
// resuelve identidades (LID → teléfono, nombres) y decide qué multimedia se descarga sola.
package ingesta

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"

	"botchat/internal/almacen"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Edicion es el cambio de texto de un mensaje ya guardado.
type Edicion struct {
	ID    string
	Texto string
}

// Clasificado es lo que se sabe de un mensaje recibido, listo para guardar.
type Clasificado struct {
	Omitir  bool     // no se guarda (reacciones, mensajes internos, etc.)
	Revocar string   // id del mensaje original que su autor borró
	Editar  *Edicion // edición de un mensaje anterior

	Tipo      string
	Texto     string
	Mime      string
	Nombre    string // nombre del archivo (documentos)
	Bytes     int64
	Ancho     int
	Alto      int
	DurS      int
	NotaVoz   bool
	Info      []byte // datos para descargar (JSON); vacío si no hay nada que descargar
	Miniatura []byte // miniatura JPEG que ya viene en el mensaje

	CitaID    string    // id del mensaje al que responde
	CitaAutor string    // JID de quien escribió el mensaje citado
	CitaTexto string    // vista previa del mensaje citado
	Menciones []string  // JIDs mencionados con @
	Reenviado bool      // viene con la marca "Reenviado"
	Reaccion  *Reaccion // es una reacción a otro mensaje (Emoji vacío = la quitó)

	VerTipo string // en un "ver una vez" con contenido: imagen, video o audio
}

// Reaccion es una reacción con emoji a un mensaje.
type Reaccion struct {
	ID    string
	Emoji string
}

// InfoDescarga son los datos mínimos para descargar y descifrar un archivo de WhatsApp.
// Se guardan cifrados en la base (columna archivo_info_cifrado).
type InfoDescarga struct {
	Tipo       string `json:"t"`
	DirectPath string `json:"dp"`
	MediaKey   string `json:"mk"`
	SHA256     string `json:"sha"`
	EncSHA256  string `json:"esha"`
	Mime       string `json:"mime,omitempty"`
	Largo      int64  `json:"len,omitempty"`
}

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// Bytes devuelve los campos binarios decodificados.
func (i InfoDescarga) Bytes() (clave, sha, encSha []byte, err error) {
	if clave, err = base64.StdEncoding.DecodeString(i.MediaKey); err != nil {
		return
	}
	if sha, err = base64.StdEncoding.DecodeString(i.SHA256); err != nil {
		return
	}
	encSha, err = base64.StdEncoding.DecodeString(i.EncSHA256)
	return
}

// ParseInfo lee la información de descarga guardada.
func ParseInfo(b []byte) (InfoDescarga, error) {
	var i InfoDescarga
	err := json.Unmarshal(b, &i)
	return i, err
}

func infoDe(tipo, directPath string, clave, sha, enc []byte, mime string, largo uint64) []byte {
	if directPath == "" || len(clave) == 0 {
		return nil // sin esto no se puede bajar
	}
	b, _ := json.Marshal(InfoDescarga{Tipo: tipo, DirectPath: directPath, MediaKey: b64(clave), SHA256: b64(sha), EncSHA256: b64(enc), Mime: mime, Largo: int64(largo)})
	return b
}

func textoDe(m *waE2E.Message) (string, bool) {
	if t := m.GetConversation(); t != "" {
		return t, true
	}
	if e := m.GetExtendedTextMessage(); e != nil {
		return e.GetText(), true
	}
	return "", false
}

// EsVerUnaVez dice si el mensaje es de "ver una vez" en cualquiera de sus formas
// (envolturas ViewOnceMessage, V2 y V2Extension, o la marca dentro de foto, video o audio).
func EsVerUnaVez(evt *events.Message) bool {
	if evt == nil {
		return false
	}
	if evt.IsViewOnce || evt.IsViewOnceV2 || evt.IsViewOnceV2Extension {
		return true
	}
	for _, m := range []*waE2E.Message{evt.Message, evt.RawMessage} {
		if m == nil {
			continue
		}
		if m.GetViewOnceMessage() != nil || m.GetViewOnceMessageV2() != nil || m.GetViewOnceMessageV2Extension() != nil {
			return true
		}
		if m.GetImageMessage().GetViewOnce() || m.GetVideoMessage().GetViewOnce() || m.GetAudioMessage().GetViewOnce() {
			return true
		}
	}
	return false
}

// Clasificar decide qué es un mensaje y extrae lo que se guarda. No toca la red ni el disco.
func Clasificar(evt *events.Message) Clasificado {
	c := clasificar(evt)
	if !c.Omitir && evt != nil && evt.Message != nil && c.Reaccion == nil && c.Revocar == "" && c.Editar == nil {
		if ci := contextoDe(evt.Message); ci != nil {
			c.CitaID = ci.GetStanzaID()
			if c.CitaID != "" {
				c.CitaAutor = ci.GetParticipant()
				c.CitaTexto = PreviewCita(ci.GetQuotedMessage())
			}
			c.Menciones = ci.GetMentionedJID()
			c.Reenviado = ci.GetIsForwarded() || ci.GetForwardingScore() > 0
		}
	}
	return c
}

// contextoDe encuentra el ContextInfo (cita, menciones, reenviado) de cualquier tipo de mensaje.
func contextoDe(m *waE2E.Message) *waE2E.ContextInfo {
	switch {
	case m.GetExtendedTextMessage() != nil:
		return m.GetExtendedTextMessage().GetContextInfo()
	case m.GetImageMessage() != nil:
		return m.GetImageMessage().GetContextInfo()
	case m.GetVideoMessage() != nil:
		return m.GetVideoMessage().GetContextInfo()
	case m.GetAudioMessage() != nil:
		return m.GetAudioMessage().GetContextInfo()
	case m.GetDocumentMessage() != nil:
		return m.GetDocumentMessage().GetContextInfo()
	case m.GetStickerMessage() != nil:
		return m.GetStickerMessage().GetContextInfo()
	case m.GetContactMessage() != nil:
		return m.GetContactMessage().GetContextInfo()
	case m.GetLocationMessage() != nil:
		return m.GetLocationMessage().GetContextInfo()
	}
	return nil
}

// PreviewCita es el texto corto de un mensaje citado.
func PreviewCita(q *waE2E.Message) string {
	if q == nil {
		return ""
	}
	corto := func(s string) string {
		s = strings.Join(strings.Fields(s), " ")
		if r := []rune(s); len(r) > 140 {
			return string(r[:140]) + "…"
		}
		return s
	}
	con := func(etiqueta, pie string) string {
		if strings.TrimSpace(pie) == "" {
			return etiqueta
		}
		return etiqueta + " · " + corto(pie)
	}
	switch {
	case q.GetConversation() != "":
		return corto(q.GetConversation())
	case q.GetExtendedTextMessage() != nil:
		return corto(q.GetExtendedTextMessage().GetText())
	case q.GetImageMessage() != nil:
		return con("Foto", q.GetImageMessage().GetCaption())
	case q.GetVideoMessage() != nil:
		return con("Video", q.GetVideoMessage().GetCaption())
	case q.GetAudioMessage() != nil:
		if q.GetAudioMessage().GetPTT() {
			return "Nota de voz"
		}
		return "Audio"
	case q.GetDocumentMessage() != nil:
		n := q.GetDocumentMessage().GetFileName()
		if n == "" {
			n = q.GetDocumentMessage().GetTitle()
		}
		return con("Documento", n)
	case q.GetStickerMessage() != nil:
		return "Sticker"
	case q.GetContactMessage() != nil:
		return "Contacto: " + corto(q.GetContactMessage().GetDisplayName())
	case q.GetLocationMessage() != nil:
		return "Ubicación"
	}
	return "Mensaje"
}

func clasificar(evt *events.Message) Clasificado {
	if evt == nil || evt.Message == nil && evt.RawMessage == nil {
		return Clasificado{Omitir: true}
	}
	// Los "ver una vez" se resuelven primero. Si el contenido llega completo (con sus datos de descarga) se guarda para abrirlo
	// una sola vez, sin miniatura ni pie de foto; si llega redactado, solo queda el aviso y no se intenta sortear nada.
	if EsVerUnaVez(evt) {
		if evt.Message != nil {
			inner := clasificarContenido(evt)
			if (inner.Tipo == almacen.TipoImagen || inner.Tipo == almacen.TipoVideo || inner.Tipo == almacen.TipoAudio) && len(inner.Info) > 0 {
				return Clasificado{Tipo: almacen.TipoVerUnaVez, VerTipo: inner.Tipo, Mime: inner.Mime, Bytes: inner.Bytes, Ancho: inner.Ancho, Alto: inner.Alto, DurS: inner.DurS, Info: inner.Info}
			}
		}
		return Clasificado{Tipo: almacen.TipoVerUnaVez, Texto: almacen.TextoVerUnaVez}
	}
	return clasificarContenido(evt)
}

// clasificarContenido clasifica el contenido ya sin la envoltura de "ver una vez".
func clasificarContenido(evt *events.Message) Clasificado {
	m := evt.Message
	if m == nil {
		return Clasificado{Omitir: true}
	}
	if p := m.GetProtocolMessage(); p != nil {
		switch p.GetType() {
		case waE2E.ProtocolMessage_REVOKE:
			return Clasificado{Revocar: p.GetKey().GetID()}
		case waE2E.ProtocolMessage_MESSAGE_EDIT:
			ed := p.GetEditedMessage()
			if t, ok := textoDe(ed); ok {
				return Clasificado{Editar: &Edicion{ID: p.GetKey().GetID(), Texto: t}}
			}
			if ed.GetImageMessage() != nil || ed.GetVideoMessage() != nil || ed.GetDocumentMessage() != nil {
				c := Clasificar(&events.Message{Info: evt.Info, Message: ed})
				if c.Texto != "" || c.Tipo == "" {
					return Clasificado{Editar: &Edicion{ID: p.GetKey().GetID(), Texto: c.Texto}}
				}
			}
		}
		return Clasificado{Omitir: true}
	}
	if r := m.GetReactionMessage(); r != nil {
		return Clasificado{Reaccion: &Reaccion{ID: r.GetKey().GetID(), Emoji: r.GetText()}}
	}
	if m.GetPollUpdateMessage() != nil || m.GetKeepInChatMessage() != nil || m.GetPinInChatMessage() != nil {
		return Clasificado{Omitir: true}
	}
	if t, ok := textoDe(m); ok {
		if t == "" {
			return Clasificado{Omitir: true}
		}
		return Clasificado{Tipo: almacen.TipoTexto, Texto: t}
	}
	switch {
	case m.GetImageMessage() != nil:
		i := m.GetImageMessage()
		return Clasificado{Tipo: almacen.TipoImagen, Texto: i.GetCaption(), Mime: i.GetMimetype(), Bytes: int64(i.GetFileLength()),
			Ancho: int(i.GetWidth()), Alto: int(i.GetHeight()), Miniatura: i.GetJPEGThumbnail(),
			Info: infoDe("imagen", i.GetDirectPath(), i.GetMediaKey(), i.GetFileSHA256(), i.GetFileEncSHA256(), i.GetMimetype(), i.GetFileLength())}
	case m.GetVideoMessage() != nil:
		v := m.GetVideoMessage()
		return Clasificado{Tipo: almacen.TipoVideo, Texto: v.GetCaption(), Mime: v.GetMimetype(), Bytes: int64(v.GetFileLength()),
			Ancho: int(v.GetWidth()), Alto: int(v.GetHeight()), DurS: int(v.GetSeconds()), Miniatura: v.GetJPEGThumbnail(),
			Info: infoDe("video", v.GetDirectPath(), v.GetMediaKey(), v.GetFileSHA256(), v.GetFileEncSHA256(), v.GetMimetype(), v.GetFileLength())}
	case m.GetAudioMessage() != nil:
		a := m.GetAudioMessage()
		return Clasificado{Tipo: almacen.TipoAudio, Mime: a.GetMimetype(), Bytes: int64(a.GetFileLength()), DurS: int(a.GetSeconds()), NotaVoz: a.GetPTT(),
			Info: infoDe("audio", a.GetDirectPath(), a.GetMediaKey(), a.GetFileSHA256(), a.GetFileEncSHA256(), a.GetMimetype(), a.GetFileLength())}
	case m.GetDocumentMessage() != nil:
		d := m.GetDocumentMessage()
		nombre := d.GetFileName()
		if nombre == "" {
			nombre = d.GetTitle()
		}
		return Clasificado{Tipo: almacen.TipoDocumento, Texto: d.GetCaption(), Mime: d.GetMimetype(), Nombre: nombre, Bytes: int64(d.GetFileLength()),
			Miniatura: d.GetJPEGThumbnail(),
			Info:      infoDe("documento", d.GetDirectPath(), d.GetMediaKey(), d.GetFileSHA256(), d.GetFileEncSHA256(), d.GetMimetype(), d.GetFileLength())}
	case m.GetStickerMessage() != nil:
		s := m.GetStickerMessage()
		return Clasificado{Tipo: almacen.TipoSticker, Mime: s.GetMimetype(), Bytes: int64(s.GetFileLength()), Ancho: int(s.GetWidth()), Alto: int(s.GetHeight()),
			Info: infoDe("sticker", s.GetDirectPath(), s.GetMediaKey(), s.GetFileSHA256(), s.GetFileEncSHA256(), s.GetMimetype(), s.GetFileLength())}
	case m.GetContactMessage() != nil:
		return Clasificado{Tipo: almacen.TipoContacto, Texto: contactosJSON([]*waE2E.ContactMessage{m.GetContactMessage()})}
	case m.GetContactsArrayMessage() != nil:
		return Clasificado{Tipo: almacen.TipoContacto, Texto: contactosJSON(m.GetContactsArrayMessage().GetContacts())}
	case m.GetLocationMessage() != nil:
		l := m.GetLocationMessage()
		return Clasificado{Tipo: almacen.TipoUbicacion, Texto: ubicacionJSON(l.GetDegreesLatitude(), l.GetDegreesLongitude(), l.GetName(), l.GetAddress(), false)}
	case m.GetLiveLocationMessage() != nil:
		l := m.GetLiveLocationMessage()
		return Clasificado{Tipo: almacen.TipoUbicacion, Texto: ubicacionJSON(l.GetDegreesLatitude(), l.GetDegreesLongitude(), "", l.GetCaption(), true)}
	case m.GetPollCreationMessage() != nil:
		return Clasificado{Tipo: almacen.TipoEncuesta, Texto: "Encuesta: " + m.GetPollCreationMessage().GetName()}
	case m.GetPollCreationMessageV2() != nil:
		return Clasificado{Tipo: almacen.TipoEncuesta, Texto: "Encuesta: " + m.GetPollCreationMessageV2().GetName()}
	case m.GetPollCreationMessageV3() != nil:
		return Clasificado{Tipo: almacen.TipoEncuesta, Texto: "Encuesta: " + m.GetPollCreationMessageV3().GetName()}
	case m.GetButtonsResponseMessage() != nil:
		return Clasificado{Tipo: almacen.TipoTexto, Texto: m.GetButtonsResponseMessage().GetSelectedDisplayText()}
	case m.GetListResponseMessage() != nil:
		return Clasificado{Tipo: almacen.TipoTexto, Texto: m.GetListResponseMessage().GetTitle()}
	case m.GetSenderKeyDistributionMessage() != nil && onlySenderKey(m):
		return Clasificado{Omitir: true}
	}
	if sinContenido(m) {
		return Clasificado{Omitir: true}
	}
	return Clasificado{Tipo: almacen.TipoOtro, Texto: TextoEspecial}
}

// TextoEspecial es lo que se guarda de un mensaje que esta versión no sabe mostrar (la pantalla lo dibuja como tarjeta).
const TextoEspecial = "Mensaje especial"

var reTel = regexp.MustCompile(`(?im)^(?:item\d+\.)?TEL[^:\r\n]*:([^\r\n]+)`)
var reNombre = regexp.MustCompile(`(?im)^FN[^:\r\n]*:([^\r\n]+)`)

// contactosJSON guarda cada contacto compartido con su nombre y teléfonos (sacados de la tarjeta vCard).
func contactosJSON(cs []*waE2E.ContactMessage) string {
	v := almacen.ContactoVista{V: 1, Contacts: []almacen.ContactoUn{}}
	for _, c := range cs {
		nombre := strings.TrimSpace(c.GetDisplayName())
		card := c.GetVcard()
		if nombre == "" {
			if m := reNombre.FindStringSubmatch(card); m != nil {
				nombre = strings.TrimSpace(m[1])
			}
		}
		u := almacen.ContactoUn{Nombre: nombre, Tels: []string{}}
		for _, m := range reTel.FindAllStringSubmatch(card, -1) {
			if t := strings.TrimSpace(m[1]); t != "" {
				u.Tels = append(u.Tels, t)
			}
		}
		v.Contacts = append(v.Contacts, u)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func ubicacionJSON(lat, lon float64, nombre, dir string, vivo bool) string {
	b, _ := json.Marshal(almacen.UbicacionVista{V: 1, Lat: lat, Lon: lon, Nombre: strings.TrimSpace(nombre), Dir: strings.TrimSpace(dir), Vivo: vivo})
	return string(b)
}

// sinContenido: el mensaje no trae nada que mostrar (solo datos internos o un texto vacío).
func sinContenido(m *waE2E.Message) bool {
	vacio := true
	m.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch fd.Name() {
		case "messageContextInfo", "senderKeyDistributionMessage", "messageSecret":
			return true
		case "conversation":
			if v.String() == "" {
				return true
			}
		}
		vacio = false
		return false
	})
	return vacio
}

// onlySenderKey: el mensaje solo trae la distribución de claves de un grupo (no es contenido).
func onlySenderKey(m *waE2E.Message) bool {
	c := m.GetSenderKeyDistributionMessage()
	return c != nil && m.GetConversation() == "" && m.GetExtendedTextMessage() == nil && m.GetImageMessage() == nil
}
