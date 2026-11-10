// Package ingesta convierte los eventos de WhatsApp en registros del almacén: clasifica cada mensaje,
// resuelve identidades (LID → teléfono, nombres) y decide qué multimedia se descarga sola.
package ingesta

import (
	"encoding/base64"
	"encoding/json"
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
	if evt == nil || evt.Message == nil && evt.RawMessage == nil {
		return Clasificado{Omitir: true}
	}
	// Los "ver una vez" se resuelven primero y no conservan NADA del contenido: ni miniatura, ni claves, ni rutas.
	if EsVerUnaVez(evt) {
		return Clasificado{Tipo: almacen.TipoVerUnaVez, Texto: almacen.TextoVerUnaVez}
	}
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
	if m.GetReactionMessage() != nil || m.GetPollUpdateMessage() != nil || m.GetKeepInChatMessage() != nil || m.GetPinInChatMessage() != nil {
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
		return Clasificado{Tipo: almacen.TipoContacto, Texto: "Contacto: " + m.GetContactMessage().GetDisplayName()}
	case m.GetContactsArrayMessage() != nil:
		return Clasificado{Tipo: almacen.TipoContacto, Texto: "Contactos: " + m.GetContactsArrayMessage().GetDisplayName()}
	case m.GetLocationMessage() != nil:
		l := m.GetLocationMessage()
		return Clasificado{Tipo: almacen.TipoUbicacion, Texto: strings.TrimSpace("Ubicación: " + l.GetName() + " " + l.GetAddress())}
	case m.GetLiveLocationMessage() != nil:
		return Clasificado{Tipo: almacen.TipoUbicacion, Texto: "Ubicación en tiempo real"}
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
	return Clasificado{Tipo: almacen.TipoOtro, Texto: "[Mensaje no compatible con esta versión]"}
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
