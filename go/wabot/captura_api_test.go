package wabot

import (
	"encoding/json"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"botchat/internal/captura"
)

func TestApiCapturaApagadaPorDefectoYSoloEstructura(t *testing.T) {
	// sin bot en marcha: vacío y apagado, y activar da un error claro
	mu.Lock()
	previo := st
	st = nil
	mu.Unlock()
	defer func() { mu.Lock(); st = previo; mu.Unlock() }()
	e := decodifica[captura.Estado](t, func() (string, error) { return EstadoCaptura(1) })
	if e.Activo || len(e.Items) != 0 || e.Max != 30 {
		t.Fatalf("sin bot: %+v", e)
	}
	if err := ActivarCaptura(1, true); err == nil {
		t.Fatal("sin bot no se puede activar")
	}

	s := &estado{botID: 1, captura: captura.Nuevo()}
	mu.Lock()
	st = s
	mu.Unlock()
	chat, _ := types.ParseJID("5215512345678@s.whatsapp.net")
	ev := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chat, Sender: chat}},
		Message: &waE2E.Message{TemplateButtonReplyMessage: &waE2E.TemplateButtonReplyMessage{SelectedID: proto.String("bc|X|1"), SelectedDisplayText: proto.String("Sí")}}}
	s.capturar(ev) // apagada: nada
	if e = decodifica[captura.Estado](t, func() (string, error) { return EstadoCaptura(1) }); len(e.Items) != 0 {
		t.Fatalf("apagada capturó: %+v", e)
	}
	if err := ActivarCaptura(1, true); err != nil {
		t.Fatal(err)
	}
	s.capturar(ev)
	j, _ := EstadoCaptura(1)
	var x captura.Estado
	if err := json.Unmarshal([]byte(j), &x); err != nil || !x.Activo || len(x.Items) != 1 {
		t.Fatalf("encendida: %v %s", err, j)
	}
	it := x.Items[0]
	if it.Chat != "privado" || it.Clasif != "Tipo=texto" || !strings.Contains(it.Arbol, "templateButtonReplyMessage") || strings.Contains(j, "5215512345678") {
		t.Fatalf("item: %+v", it)
	}
	if err := BorrarCaptura(1); err != nil {
		t.Fatal(err)
	}
	if e = decodifica[captura.Estado](t, func() (string, error) { return EstadoCaptura(1) }); len(e.Items) != 0 {
		t.Fatal("borrar")
	}
	_ = ActivarCaptura(1, false)
	if e = decodifica[captura.Estado](t, func() (string, error) { return EstadoCaptura(1) }); e.Activo {
		t.Fatal("apagar")
	}
}
