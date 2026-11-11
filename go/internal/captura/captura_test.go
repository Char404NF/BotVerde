package captura

import (
	"strings"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func evento(m *waE2E.Message, chat string) *events.Message {
	j, _ := types.ParseJID(chat)
	return &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: j, Sender: j}}, Message: m, RawMessage: m}
}

func TestVolcadoSoloEstructura(t *testing.T) {
	secreto := "hola 5215512345678 clave"
	m := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String(secreto), JPEGThumbnail: []byte("BYTESSECRETOS"),
			ContextInfo: &waE2E.ContextInfo{StanzaID: proto.String("ABC123"), Participant: proto.String("5215512345678@s.whatsapp.net")}},
		MessageContextInfo: &waE2E.MessageContextInfo{MessageSecret: []byte("LLAVE")},
	}
	a := Volcar(evento(m, "5215512345678@s.whatsapp.net"), false)
	for _, malo := range []string{"hola", "5215512345678", "BYTESSECRETOS", "LLAVE", "ABC123"} {
		if strings.Contains(a, malo) {
			t.Fatalf("el volcado contiene %q:\n%s", malo, a)
		}
	}
	for _, bien := range []string{"extendedTextMessage", "text = texto(", "JPEGThumbnail = bytes(13)", "messageSecret = bytes(5)", "stanzaID"} {
		if !strings.Contains(a, bien) {
			t.Fatalf("falta %q:\n%s", bien, a)
		}
	}
	// con textos: se ve, pero el teléfono va enmascarado; los bytes nunca
	b := Volcar(evento(m, "5215512345678@s.whatsapp.net"), true)
	if !strings.Contains(b, "hola ***5678 clave") || strings.Contains(b, "5215512345678") || strings.Contains(b, "BYTESSECRETOS") {
		t.Fatalf("con textos:\n%s", b)
	}
	// textos largos (más de 60) solo por su largo
	largo := strings.Repeat("a", 61)
	c := Volcar(evento(&waE2E.Message{Conversation: proto.String(largo)}, "1@s.whatsapp.net"), true)
	if strings.Contains(c, largo) || !strings.Contains(c, "texto(61)") {
		t.Fatalf("largo:\n%s", c)
	}
}

func TestRespuestaDeBotonMuestraSusCampos(t *testing.T) {
	m := &waE2E.Message{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{
		Body: &waE2E.InteractiveResponseMessage_Body{Text: proto.String("Precios")},
		InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{
			NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{Name: proto.String("quick_reply"), ParamsJSON: proto.String(`{"id":"bc|X|2"}`)}}}}
	a := Volcar(evento(m, "1@s.whatsapp.net"), false)
	if !strings.Contains(a, `name = "quick_reply"`) || !strings.Contains(a, `paramsJSON = "{\"id\":\"bc|X|2\"}"`) {
		t.Fatalf("faltan los campos de respuesta:\n%s", a)
	}
	if strings.Contains(a, "Precios") {
		t.Fatalf("el cuerpo no es un campo conocido:\n%s", a)
	}
	tp := &waE2E.Message{TemplateButtonReplyMessage: &waE2E.TemplateButtonReplyMessage{SelectedID: proto.String("id1"), SelectedDisplayText: proto.String("Ver")}}
	if a := Volcar(evento(tp, "1@s.whatsapp.net"), false); !strings.Contains(a, `selectedID = "id1"`) || !strings.Contains(a, `selectedDisplayText = "Ver"`) {
		t.Fatalf("template:\n%s", a)
	}
}

func TestRawDistintoYBanderas(t *testing.T) {
	interior := &waE2E.Message{Conversation: proto.String("x")}
	raw := &waE2E.Message{EphemeralMessage: &waE2E.FutureProofMessage{Message: interior}}
	e := evento(interior, "1-2@g.us")
	e.RawMessage = raw
	e.IsEphemeral = true
	a := Volcar(e, false)
	if !strings.Contains(a, "RawMessage") || !strings.Contains(a, "ephemeralMessage") {
		t.Fatalf("raw:\n%s", a)
	}
	if !strings.Contains(banderas(e), "efímero") || !strings.Contains(banderas(e), "RawMessage distinto") || TipoChat(e.Info.Chat) != "grupo" {
		t.Fatalf("banderas: %s %s", banderas(e), TipoChat(e.Info.Chat))
	}
}

func TestInterruptorLimitesYBuffer(t *testing.T) {
	s := Nuevo()
	t0 := time.Unix(1000, 0)
	ahora := t0
	s.FijarReloj(func() time.Time { return ahora })
	ev := evento(&waE2E.Message{Conversation: proto.String("x")}, "1@s.whatsapp.net")
	s.Registrar(ev, "a")
	if s.Activa() || len(s.Estado().Items) != 0 {
		t.Fatal("apagada por defecto: no captura")
	}
	s.Activar(true)
	fromMe := evento(&waE2E.Message{Conversation: proto.String("x")}, "1@s.whatsapp.net")
	fromMe.Info.IsFromMe = true
	s.Registrar(fromMe, "propio")
	if len(s.Estado().Items) != 0 {
		t.Fatal("los propios no se capturan")
	}
	for i := 0; i < 29; i++ {
		s.Registrar(ev, "a")
	}
	if !s.Activa() {
		t.Fatal("sigue activa a los 29")
	}
	s.Registrar(ev, "a")
	if s.Activa() || len(s.Estado().Items) != 30 {
		t.Fatalf("se apaga a los 30: %+v", s.Estado())
	}
	s.Registrar(ev, "a")
	if len(s.Estado().Items) != 30 {
		t.Fatal("apagada no captura")
	}
	// de nuevo: el buffer no pasa de 30 (se van los más viejos)
	s.Activar(true)
	s.Registrar(ev, "nueva")
	it := s.Estado().Items
	if len(it) != 30 || it[29].Clasif != "nueva" || it[0].N != 2 {
		t.Fatalf("buffer: %d %+v", len(it), it[0])
	}
	// caduca a los 10 minutos
	ahora = ahora.Add(9 * time.Minute)
	if !s.Activa() || s.Estado().RestanSeg != 60 {
		t.Fatalf("a los 9 min: %+v", s.Estado())
	}
	ahora = ahora.Add(61 * time.Second)
	if s.Activa() {
		t.Fatal("a los 10 min se apaga")
	}
	s.Borrar()
	if len(s.Estado().Items) != 0 {
		t.Fatal("borrar")
	}
}
