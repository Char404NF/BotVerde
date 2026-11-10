package ingesta

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"

	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func evt(m *waE2E.Message) *events.Message {
	e := &events.Message{Info: types.MessageInfo{ID: "ID1", Timestamp: time.Unix(100, 0)}, RawMessage: m}
	e.UnwrapRaw()
	return e
}

func imagen(visto bool) *waE2E.ImageMessage {
	return &waE2E.ImageMessage{
		Caption: proto.String("pie de foto"), Mimetype: proto.String("image/jpeg"), FileLength: proto.Uint64(1234), Width: proto.Uint32(800), Height: proto.Uint32(600),
		JPEGThumbnail: []byte("MINIATURA"), MediaKey: []byte("CLAVE-DE-MEDIO-32-BYTES-AAAAAAAA"), DirectPath: proto.String("/v/t62/abc"),
		FileSHA256: []byte("sha"), FileEncSHA256: []byte("esha"), URL: proto.String("https://mmg.whatsapp.net/secreto"), ViewOnce: proto.Bool(visto),
	}
}

func TestTextoYMultimedia(t *testing.T) {
	c := Clasificar(evt(&waE2E.Message{Conversation: proto.String("hola")}))
	if c.Tipo != almacen.TipoTexto || c.Texto != "hola" {
		t.Fatalf("%+v", c)
	}
	c = Clasificar(evt(&waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("con enlace")}}))
	if c.Texto != "con enlace" {
		t.Fatalf("%+v", c)
	}
	c = Clasificar(evt(&waE2E.Message{ImageMessage: imagen(false)}))
	if c.Tipo != almacen.TipoImagen || c.Texto != "pie de foto" || c.Bytes != 1234 || c.Ancho != 800 || string(c.Miniatura) != "MINIATURA" || len(c.Info) == 0 {
		t.Fatalf("%+v", c)
	}
	info, err := ParseInfo(c.Info)
	if err != nil || info.DirectPath != "/v/t62/abc" || info.Tipo != "imagen" {
		t.Fatalf("%+v %v", info, err)
	}
	if strings.Contains(string(c.Info), "mmg.whatsapp.net") {
		t.Fatal("la URL no se guarda")
	}
	k, sha, esha, err := info.Bytes()
	if err != nil || string(k) != "CLAVE-DE-MEDIO-32-BYTES-AAAAAAAA" || string(sha) != "sha" || string(esha) != "esha" {
		t.Fatalf("%q %v", k, err)
	}
	c = Clasificar(evt(&waE2E.Message{AudioMessage: &waE2E.AudioMessage{PTT: proto.Bool(true), Seconds: proto.Uint32(12), MediaKey: []byte("k"), DirectPath: proto.String("/a")}}))
	if c.Tipo != almacen.TipoAudio || !c.NotaVoz || c.DurS != 12 {
		t.Fatalf("%+v", c)
	}
	c = Clasificar(evt(&waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{FileName: proto.String("cuenta.pdf"), Mimetype: proto.String("application/pdf"), MediaKey: []byte("k"), DirectPath: proto.String("/d")}}))
	if c.Tipo != almacen.TipoDocumento || c.Nombre != "cuenta.pdf" {
		t.Fatalf("%+v", c)
	}
	c = Clasificar(evt(&waE2E.Message{VideoMessage: &waE2E.VideoMessage{Seconds: proto.Uint32(30), FileLength: proto.Uint64(9 << 20), MediaKey: []byte("k"), DirectPath: proto.String("/v")}}))
	if c.Tipo != almacen.TipoVideo || c.DurS != 30 || c.Bytes != 9<<20 {
		t.Fatalf("%+v", c)
	}
	// sin clave o sin ruta no se puede bajar
	c = Clasificar(evt(&waE2E.Message{ImageMessage: &waE2E.ImageMessage{Mimetype: proto.String("image/jpeg")}}))
	if c.Tipo != almacen.TipoImagen || len(c.Info) != 0 {
		t.Fatalf("%+v", c)
	}
}

func TestVerUnaVezNoConservaNada(t *testing.T) {
	casos := map[string]*waE2E.Message{
		"marca en la imagen": {ImageMessage: imagen(true)},
		"marca en el video":  {VideoMessage: &waE2E.VideoMessage{ViewOnce: proto.Bool(true), JPEGThumbnail: []byte("MINI"), MediaKey: []byte("k"), DirectPath: proto.String("/v")}},
		"marca en el audio":  {AudioMessage: &waE2E.AudioMessage{ViewOnce: proto.Bool(true), MediaKey: []byte("k"), DirectPath: proto.String("/a")}},
		"envoltura v1":       {ViewOnceMessage: &waE2E.FutureProofMessage{Message: &waE2E.Message{ImageMessage: imagen(false)}}},
		"envoltura v2":       {ViewOnceMessageV2: &waE2E.FutureProofMessage{Message: &waE2E.Message{ImageMessage: imagen(false)}}},
		"envoltura v2 ext":   {ViewOnceMessageV2Extension: &waE2E.FutureProofMessage{Message: &waE2E.Message{VideoMessage: &waE2E.VideoMessage{MediaKey: []byte("k"), DirectPath: proto.String("/v")}}}},
	}
	for nombre, m := range casos {
		e := evt(m)
		if !EsVerUnaVez(e) {
			t.Errorf("%s: no se detectó", nombre)
		}
		c := Clasificar(e)
		if c.Tipo != almacen.TipoVerUnaVez || c.Texto != almacen.TextoVerUnaVez {
			t.Errorf("%s: %+v", nombre, c)
		}
		if len(c.Info) != 0 || len(c.Miniatura) != 0 || c.Mime != "" || c.Bytes != 0 || c.Nombre != "" || c.Ancho != 0 {
			t.Errorf("%s: conserva datos del archivo: %+v", nombre, c)
		}
	}
}

func TestProtocoloEdicionRevocacionYOmitidos(t *testing.T) {
	rev := &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{Type: waE2E.ProtocolMessage_REVOKE.Enum(), Key: &waCommon.MessageKey{ID: proto.String("ORIG")}}}
	if c := Clasificar(evt(rev)); c.Revocar != "ORIG" {
		t.Fatalf("%+v", c)
	}
	ed := &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{Type: waE2E.ProtocolMessage_MESSAGE_EDIT.Enum(), Key: &waCommon.MessageKey{ID: proto.String("ORIG")},
		EditedMessage: &waE2E.Message{Conversation: proto.String("texto corregido")}}}
	if c := Clasificar(evt(ed)); c.Editar == nil || c.Editar.ID != "ORIG" || c.Editar.Texto != "texto corregido" {
		t.Fatalf("%+v", c)
	}
	otros := []*waE2E.Message{
		{ProtocolMessage: &waE2E.ProtocolMessage{Type: waE2E.ProtocolMessage_HISTORY_SYNC_NOTIFICATION.Enum()}},
		{SenderKeyDistributionMessage: &waE2E.SenderKeyDistributionMessage{GroupID: proto.String("g")}},
		{Conversation: proto.String("")},
	}
	for i, m := range otros {
		if c := Clasificar(evt(m)); !c.Omitir {
			t.Errorf("caso %d: debía omitirse: %+v", i, c)
		}
	}
	if c := Clasificar(nil); !c.Omitir {
		t.Error("nil se omite")
	}
	if c := Clasificar(evt(&waE2E.Message{ContactMessage: &waE2E.ContactMessage{DisplayName: proto.String("Pepe")}})); c.Tipo != almacen.TipoContacto || !strings.Contains(c.Texto, "Pepe") {
		t.Errorf("%+v", c)
	}
	if c := Clasificar(evt(&waE2E.Message{LocationMessage: &waE2E.LocationMessage{Name: proto.String("Casa")}})); c.Tipo != almacen.TipoUbicacion {
		t.Errorf("%+v", c)
	}
}

func TestPoliticaDeDescarga(t *testing.T) {
	p := PoliticaPorDefecto()
	const gb = 1024 * mb
	casos := []struct {
		cat    string
		bytes  int64
		red    Red
		libre  int64
		auto   bool
		porque string
	}{
		{almacen.CatFoto, 5 * mb, RedDatos, 10 * gb, true, ""},
		{almacen.CatAudio, 1 * mb, RedWifi, 10 * gb, true, ""},
		{almacen.CatDocumento, 50 * mb, RedDatos, 10 * gb, true, ""},
		{almacen.CatDocumento, 51 * mb, RedWifi, 10 * gb, false, "archivo grande"},
		{almacen.CatVideo, 10 * mb, RedWifi, 10 * gb, true, ""},
		{almacen.CatVideo, 10 * mb, RedDatos, 10 * gb, false, "solo con Wi-Fi"},
		{almacen.CatVideo, 10 * mb, RedDesconocida, 10 * gb, false, "solo con Wi-Fi"},
		{almacen.CatVideo, 60 * mb, RedWifi, 10 * gb, false, "archivo grande"},
		{almacen.CatFoto, 1 * mb, RedWifi, 400 * mb, false, "poco espacio libre"},
		{"", 1, RedWifi, 10 * gb, false, "no es multimedia"},
	}
	for i, c := range casos {
		auto, porque := p.Decidir(c.cat, c.bytes, c.red, c.libre)
		if auto != c.auto || porque != c.porque {
			t.Errorf("caso %d: %v %q", i, auto, porque)
		}
	}
	// Con la casilla de videos con datos móviles activa:
	p.Video.Datos = true
	if auto, _ := p.Decidir(almacen.CatVideo, 10*mb, RedDatos, 10*gb); !auto {
		t.Error("con la casilla activa, el video baja con datos")
	}
	// Sin tamaño conocido (0) se permite; el tope se aplica con el tamaño real al descargar.
	if auto, _ := p.Decidir(almacen.CatFoto, 0, RedWifi, 10*gb); !auto {
		t.Error("tamaño desconocido")
	}
}

func TestContactoUbicacionYEspecial(t *testing.T) {
	vc := "BEGIN:VCARD\nVERSION:3.0\nFN:Juan Pérez\nTEL;type=CELL;waid=5215512345678:+52 1 55 1234 5678\nitem1.TEL:+52 55 0000 1111\nEND:VCARD"
	c := Clasificar(&events.Message{Message: &waE2E.Message{ContactMessage: &waE2E.ContactMessage{DisplayName: proto.String("Juan"), Vcard: proto.String(vc)}}})
	var cv almacen.ContactoVista
	if c.Tipo != almacen.TipoContacto || json.Unmarshal([]byte(c.Texto), &cv) != nil || len(cv.Contacts) != 1 || len(cv.Contacts[0].Tels) != 2 || cv.Contacts[0].Nombre != "Juan" {
		t.Fatalf("contacto: %+v %s", c, c.Texto)
	}
	u := Clasificar(&events.Message{Message: &waE2E.Message{LocationMessage: &waE2E.LocationMessage{DegreesLatitude: proto.Float64(19.43), DegreesLongitude: proto.Float64(-99.13), Name: proto.String("Zócalo")}}})
	var uv almacen.UbicacionVista
	if u.Tipo != almacen.TipoUbicacion || json.Unmarshal([]byte(u.Texto), &uv) != nil || uv.Nombre != "Zócalo" || uv.Lat != 19.43 {
		t.Fatalf("ubicación: %+v", u)
	}
	o := Clasificar(&events.Message{Message: &waE2E.Message{OrderMessage: &waE2E.OrderMessage{}}})
	if o.Tipo != almacen.TipoOtro || o.Texto != TextoEspecial {
		t.Fatalf("especial: %+v", o)
	}
}

func TestReaccionCitaMencionesYReenviado(t *testing.T) {
	r := Clasificar(evt(&waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{Key: &waCommon.MessageKey{ID: proto.String("ORIG")}, Text: proto.String("👍")}}))
	if r.Omitir || r.Reaccion == nil || r.Reaccion.ID != "ORIG" || r.Reaccion.Emoji != "👍" {
		t.Fatalf("reacción: %+v", r)
	}
	q := Clasificar(evt(&waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
		Text: proto.String("sí, ahí estaré @5215511111111"),
		ContextInfo: &waE2E.ContextInfo{
			StanzaID: proto.String("ORIG"), Participant: proto.String("5215522222222@s.whatsapp.net"),
			QuotedMessage: &waE2E.Message{ImageMessage: &waE2E.ImageMessage{Caption: proto.String("mi casa")}},
			MentionedJID:  []string{"5215511111111@s.whatsapp.net"}, IsForwarded: proto.Bool(true),
		}}}))
	if q.Tipo != "texto" || q.CitaID != "ORIG" || q.CitaAutor != "5215522222222@s.whatsapp.net" || q.CitaTexto != "Foto · mi casa" ||
		len(q.Menciones) != 1 || !q.Reenviado {
		t.Fatalf("cita: %+v", q)
	}
	if p := PreviewCita(&waE2E.Message{AudioMessage: &waE2E.AudioMessage{PTT: proto.Bool(true)}}); p != "Nota de voz" {
		t.Fatal(p)
	}
	if p := PreviewCita(nil); p != "" {
		t.Fatal("sin cita no hay vista previa")
	}
}
