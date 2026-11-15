package envio

import (
	"context"
	"testing"

	"botchat/internal/almacen"
	"go.mau.fi/whatsmeow/proto/waE2E"
)

func TestEnviarEncuestaYFijarMensaje(t *testing.T) {
	s, f, a := nuevoServicio(t)
	ctx := context.Background()
	if _, err := s.EnviarEncuesta(ctx, "120@g.us", "¿Cena?", []string{"solo una"}, false); err != ErrEncuesta {
		t.Fatalf("una opción: %v", err)
	}
	r, err := s.EnviarEncuesta(ctx, "120@g.us", "¿Cena?", []string{"Tacos", "Pizza", "Sushi"}, true)
	if err != nil || r.Estado != "enviado" || len(f.enviados) != 1 {
		t.Fatalf("%v %+v", err, r)
	}
	p := f.enviados[0].msg.GetPollCreationMessage()
	if p.GetName() != "¿Cena?" || len(p.GetOptions()) != 3 || p.GetSelectableOptionsCount() != 3 || len(f.enviados[0].msg.GetMessageContextInfo().GetMessageSecret()) != 32 {
		t.Fatalf("encuesta mal armada: %+v", f.enviados[0].msg)
	}
	m, _ := a.ObtenerMensaje(1, "120@g.us", r.ID)
	if m == nil || !m.EsMio {
		t.Fatal("la encuesta debía quedar en el chat")
	}

	recibido(t, a, "120@g.us", "AJENO1", "5215511111111@s.whatsapp.net", "Reunión el sábado", 1700000000)
	if err := s.FijarMensaje(ctx, "120@g.us", "AJENO1", 604800, true); err != nil {
		t.Fatal(err)
	}
	pin := f.enviados[len(f.enviados)-1].msg
	if pin.GetPinInChatMessage().GetType() != waE2E.PinInChatMessage_PIN_FOR_ALL || pin.GetPinInChatMessage().GetKey().GetID() != "AJENO1" ||
		pin.GetMessageContextInfo().GetMessageAddOnDurationInSecs() != 604800 {
		t.Fatalf("fijar mal armado: %+v", pin)
	}
	if err := s.FijarMensaje(ctx, "120@g.us", "AJENO1", 0, false); err != nil {
		t.Fatal(err)
	}
	if f.enviados[len(f.enviados)-1].msg.GetPinInChatMessage().GetType() != waE2E.PinInChatMessage_UNPIN_FOR_ALL {
		t.Fatal("debía soltar")
	}
	if err := s.FijarMensaje(ctx, "120@g.us", "NOEXISTE", 604800, true); err != ErrNoExiste {
		t.Fatalf("mensaje desconocido: %v", err)
	}
	_ = almacen.TipoTexto
}
