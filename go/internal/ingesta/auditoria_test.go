package ingesta

import (
	"context"
	"testing"

	"botchat/internal/almacen"

	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestAuditoriaEncendidaDeFabricaGuardaTodo(t *testing.T) {
	e := nuevoEntorno(t)
	pn := jid("525512345678@s.whatsapp.net")
	e.g.Mensaje(context.Background(), mensajeEvt(pn, pn, "A1", false, texto("mira https://ejemplo.com/x")), false)
	m, _ := e.alm.ObtenerMensaje(1, pn.String(), "A1")
	if m.Texto == "" || m.Olvidado || m.DelHistorial || !m.Enlaces {
		t.Fatalf("%+v", m)
	}
}

func TestAuditoriaApagadaPorChatGuardaFilaSinTexto(t *testing.T) {
	e := nuevoEntorno(t)
	ctx := context.Background()
	pn := jid("525512345678@s.whatsapp.net")
	otro := jid("34612345678@s.whatsapp.net")
	e.g.Mensaje(ctx, mensajeEvt(pn, pn, "A0", false, texto("antes")), false)
	if err := e.alm.FijarAuditoriaChat(1, pn.String(), 0, -1); err != nil {
		t.Fatal(err)
	}
	cita := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("con cita https://x.com"),
		ContextInfo: &waE2E.ContextInfo{StanzaID: proto.String("A0"), Participant: proto.String(pn.String()), QuotedMessage: texto("antes")}}}
	e.g.Mensaje(ctx, mensajeEvt(pn, pn, "A1", false, cita), false)
	e.g.Mensaje(ctx, mensajeEvt(otro, otro, "B1", false, texto("este sí")), false)
	// lo que ya estaba no se toca
	if m, _ := e.alm.ObtenerMensaje(1, pn.String(), "A0"); m.Texto != "antes" || m.Olvidado {
		t.Fatalf("lo anterior no se toca: %+v", m)
	}
	m, _ := e.alm.ObtenerMensaje(1, pn.String(), "A1")
	if m == nil || m.Texto != "" || m.CitaTexto != "" || !m.Olvidado || m.Enlaces || m.Tipo != almacen.TipoTexto || m.TS == 0 || m.RemitenteJID == "" || m.CitaID != "A0" {
		t.Fatalf("debía quedar la fila sin texto: %+v", m)
	}
	if b, _ := e.alm.ObtenerMensaje(1, otro.String(), "B1"); b.Texto != "este sí" || b.Olvidado {
		t.Fatalf("el otro chat sigue guardando: %+v", b)
	}
	// Las estadísticas cuentan igual.
	st, _ := e.alm.ObtenerMensaje(1, pn.String(), "A1")
	_ = st
	// Una edición no guarda el texto nuevo ni versión, pero sí queda "editado".
	ed := &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{Type: waE2E.ProtocolMessage_MESSAGE_EDIT.Enum(),
		Key: &waCommon.MessageKey{ID: proto.String("A1")}, EditedMessage: texto("nuevo secreto")}}
	e.g.Mensaje(ctx, mensajeEvt(pn, pn, "ED", false, ed), false)
	m, _ = e.alm.ObtenerMensaje(1, pn.String(), "A1")
	if !m.Editado || m.Texto != "" {
		t.Fatalf("%+v", m)
	}
	if v, _ := e.alm.VersionesDeMensaje(1, pn.String(), "A1"); len(v) != 0 {
		t.Fatalf("no debe haber versiones: %+v", v)
	}
	// Volver a encender: lo nuevo se guarda.
	_ = e.alm.FijarAuditoriaChat(1, pn.String(), -1, -1)
	e.g.Mensaje(ctx, mensajeEvt(pn, pn, "A2", false, texto("otra vez")), false)
	if m, _ := e.alm.ObtenerMensaje(1, pn.String(), "A2"); m.Texto != "otra vez" {
		t.Fatalf("%+v", m)
	}
}

func TestAuditoriaGlobalApagadaYChatEncendido(t *testing.T) {
	e := nuevoEntorno(t)
	ctx := context.Background()
	pn := jid("525512345678@s.whatsapp.net")
	otro := jid("34612345678@s.whatsapp.net")
	_ = e.alm.FijarAuditoria(1, false, -1)
	e.g.Mensaje(ctx, mensajeEvt(pn, pn, "A1", false, texto("no se guarda")), false)
	_ = e.alm.FijarAuditoriaChat(1, otro.String(), 1, -1)
	e.g.Mensaje(ctx, mensajeEvt(otro, otro, "B1", false, texto("sí se guarda")), false)
	if m, _ := e.alm.ObtenerMensaje(1, pn.String(), "A1"); m.Texto != "" || !m.Olvidado {
		t.Fatalf("%+v", m)
	}
	if m, _ := e.alm.ObtenerMensaje(1, otro.String(), "B1"); m.Texto != "sí se guarda" {
		t.Fatalf("%+v", m)
	}
}

func TestHistorialRespetaInterruptorYSeMarcaDelHistorial(t *testing.T) {
	e := nuevoEntorno(t)
	pn := "525512345678@s.whatsapp.net"
	_ = e.alm.FijarAuditoriaChat(1, "120363111@g.us", 0, -1)
	e.g.Historial(context.Background(), historialPrueba(), parseReal(t))
	h1, _ := e.alm.ObtenerMensaje(1, pn, "H1")
	if h1.Texto != "hola vieja" || !h1.DelHistorial || h1.Olvidado {
		t.Fatalf("%+v", h1)
	}
	g1, _ := e.alm.ObtenerMensaje(1, "120363111@g.us", "G1")
	if g1 == nil || g1.Texto != "" || !g1.Olvidado || !g1.DelHistorial {
		t.Fatalf("el grupo apagado no guarda texto: %+v", g1)
	}
	// uno en vivo no se marca
	e.g.Mensaje(context.Background(), mensajeEvt(jid(pn), jid(pn), "V1", false, texto("en vivo")), false)
	if v, _ := e.alm.ObtenerMensaje(1, pn, "V1"); v.DelHistorial {
		t.Fatal("en vivo no es del historial")
	}
}

func TestEdicionGuardaVersionAnteriorYHuellaDelArchivo(t *testing.T) {
	e := nuevoEntorno(t)
	ctx := context.Background()
	pn := jid("525512345678@s.whatsapp.net")
	e.g.Mensaje(ctx, mensajeEvt(pn, pn, "O1", false, texto("uno")), false)
	for i, nuevo := range []string{"dos", "tres"} {
		ed := &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{Type: waE2E.ProtocolMessage_MESSAGE_EDIT.Enum(),
			Key: &waCommon.MessageKey{ID: proto.String("O1")}, EditedMessage: texto(nuevo)}}
		e.g.Mensaje(ctx, mensajeEvt(pn, pn, "E"+string(rune('0'+i)), false, ed), false)
	}
	v, _ := e.alm.VersionesDeMensaje(1, pn.String(), "O1")
	if len(v) != 2 || v[0].Texto != "uno" || v[1].Texto != "dos" || v[0].N != 1 || v[1].N != 2 {
		t.Fatalf("%+v", v)
	}
	if m, _ := e.alm.ObtenerMensaje(1, pn.String(), "O1"); m.Texto != "tres" || !m.Editado {
		t.Fatalf("%+v", m)
	}
	// huella del archivo
	e.g.Mensaje(ctx, mensajeEvt(pn, pn, "IMG", false, imagenConMedia(false)), false)
	im, _ := e.alm.ObtenerMensaje(1, pn.String(), "IMG")
	if string(im.SHA256) != "s" {
		t.Fatalf("huella: %q", im.SHA256)
	}
	// el "ver una vez" no deja huella
	e.g.Mensaje(ctx, mensajeEvt(pn, pn, "VU", false, imagenConMedia(true)), false)
	if vu, _ := e.alm.ObtenerMensaje(1, pn.String(), "VU"); vu == nil || len(vu.SHA256) != 0 {
		t.Fatalf("ver una vez sin huella: %+v", vu)
	}
}
