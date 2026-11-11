package envio

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"botchat/internal/almacen"

	"go.mau.fi/whatsmeow/types"
)

func grupoLID(f *falso) {
	f.grupo = &types.GroupInfo{AddressingMode: types.AddressingModeLID, Participants: []types.GroupParticipant{{
		JID: types.NewJID("777", types.HiddenUserServer), LID: types.NewJID("777", types.HiddenUserServer), PhoneNumber: types.NewJID("5215522222222", types.DefaultUserServer),
	}}}
}

func TestEnviarUbicacionYContactoComoRespuestaDelBot(t *testing.T) {
	s, f, a := nuevoServicio(t)
	ctx := context.Background()
	recibido(t, a, privado, "Q1", privado, "¿dónde están?", 1000)
	r, err := s.EnviarUbicacionAuto(ctx, privado, 19.4326, -99.1332, "Sucursal Centro", "Calle 1 #2", "Q1", "")
	if err != nil || r.Estado != "enviado" {
		t.Fatal(r, err)
	}
	loc := f.enviados[0].msg.GetLocationMessage()
	if loc == nil || loc.GetDegreesLatitude() != 19.4326 || loc.GetDegreesLongitude() != -99.1332 || loc.GetName() != "Sucursal Centro" || loc.GetAddress() != "Calle 1 #2" || loc.GetContextInfo().GetStanzaID() != "Q1" {
		t.Fatalf("%+v", loc)
	}
	m, _ := a.ObtenerMensaje(1, privado, r.ID)
	var v almacen.UbicacionVista
	if m == nil || m.Tipo != almacen.TipoUbicacion || !m.EsMio || json.Unmarshal([]byte(m.Texto), &v) != nil || v.Lat != 19.4326 || v.Nombre != "Sucursal Centro" {
		t.Fatalf("se guarda como ubicación para verla en el chat: %+v", m)
	}
	if _, err := s.EnviarUbicacionAuto(ctx, privado, 200, 0, "", "", "", ""); err == nil {
		t.Fatal("una ubicación fuera de rango se rechaza")
	}
	r2, err := s.EnviarContactoAuto(ctx, privado, "Soporte Técnico", "+52 55 1234 5678", "", "")
	if err != nil || r2.Estado != "enviado" {
		t.Fatal(r2, err)
	}
	cm := f.enviados[1].msg.GetContactMessage()
	if cm == nil || cm.GetDisplayName() != "Soporte Técnico" || !strings.Contains(cm.GetVcard(), "FN:Soporte Técnico") || !strings.Contains(cm.GetVcard(), "waid=525512345678") || !strings.HasPrefix(cm.GetVcard(), "BEGIN:VCARD") {
		t.Fatalf("%+v", cm)
	}
	m2, _ := a.ObtenerMensaje(1, privado, r2.ID)
	var cv almacen.ContactoVista
	if m2 == nil || m2.Tipo != almacen.TipoContacto || json.Unmarshal([]byte(m2.Texto), &cv) != nil || len(cv.Contacts) != 1 || cv.Contacts[0].Tels[0] != "+525512345678" {
		t.Fatalf("%+v", m2)
	}
	if _, err := s.EnviarContactoAuto(ctx, privado, "", "123", "", ""); err == nil {
		t.Fatal("el contacto necesita nombre y teléfono")
	}
	if a.BotPausado(1, privado, s.ahora()) {
		t.Fatal("lo que manda el bot no pausa al bot")
	}
}

func TestCitaConElAutorExactoYMencionEnGrupoPorLID(t *testing.T) {
	s, f, a := nuevoServicio(t)
	ctx := context.Background()
	grupoLID(f)
	// El mensaje se guardó con el número de la persona; WhatsApp lo mandó con su LID: la cita lleva ese LID, tal cual.
	recibido(t, a, "120@g.us", "G1", "5215522222222@s.whatsapp.net", "!menu", 1001)
	if _, err := s.EnviarAutoCita(ctx, "120@g.us", "@777 hola", "G1", "777@lid", []string{"777@lid"}); err != nil {
		t.Fatal(err)
	}
	ci := f.enviados[0].msg.GetExtendedTextMessage().GetContextInfo()
	if ci.GetParticipant() != "777@lid" || ci.GetStanzaID() != "G1" || len(ci.GetMentionedJID()) != 1 || ci.GetMentionedJID()[0] != "777@lid" {
		t.Fatalf("cita y mención con LID: %+v", ci)
	}
	// Con un número (grupo que direcciona por teléfono) la cita y la mención llevan el número.
	f.grupo = &types.GroupInfo{Participants: []types.GroupParticipant{{JID: types.NewJID("5215533333333", types.DefaultUserServer)}}}
	recibido(t, a, "120@g.us", "G2", "5215533333333@s.whatsapp.net", "!menu", 1002)
	if _, err := s.EnviarAutoCita(ctx, "120@g.us", "@5215533333333 hola", "G2", "5215533333333@s.whatsapp.net", []string{"5215533333333@s.whatsapp.net"}); err != nil {
		t.Fatal(err)
	}
	ci = f.enviados[1].msg.GetExtendedTextMessage().GetContextInfo()
	if ci.GetParticipant() != "5215533333333@s.whatsapp.net" || ci.GetMentionedJID()[0] != "5215533333333@s.whatsapp.net" {
		t.Fatalf("cita y mención con número: %+v", ci)
	}
	// El autor dado no se queda pegado: el siguiente envío sin él busca en el grupo como siempre.
	grupoLID(f)
	if _, err := s.EnviarAuto(ctx, "120@g.us", "sin autor", "G1", nil); err != nil {
		t.Fatal(err)
	}
	if p := f.enviados[2].msg.GetExtendedTextMessage().GetContextInfo().GetParticipant(); p != "777@lid" {
		t.Fatalf("sin autor dado se busca en el grupo: %q", p)
	}
}

func TestUbicacionYArchivoCitanConAutorExacto(t *testing.T) {
	s, f, a := nuevoServicio(t)
	ctx := context.Background()
	grupoLID(f)
	recibido(t, a, "120@g.us", "G1", "5215522222222@s.whatsapp.net", "hola", 1001)
	if _, err := s.EnviarUbicacionAuto(ctx, "120@g.us", 1, 2, "x", "", "G1", "777@lid"); err != nil {
		t.Fatal(err)
	}
	if p := f.enviados[0].msg.GetLocationMessage().GetContextInfo().GetParticipant(); p != "777@lid" {
		t.Fatalf("%q", p)
	}
	_, err := s.EnviarMedioAutoCita(ctx, "120@g.us", Adjunto{Tipo: almacen.TipoImagen, Ruta: archivoTemporal(t, "f.jpg", []byte("FOTO")), Mime: "image/jpeg", Pie: "@777 mira"}, "G1", "777@lid", []string{"777@lid"})
	if err != nil {
		t.Fatal(err)
	}
	im := f.enviados[1].msg.GetImageMessage()
	ci := im.GetContextInfo()
	if im.GetCaption() != "@777 mira" || ci.GetParticipant() != "777@lid" || ci.GetStanzaID() != "G1" || len(ci.GetMentionedJID()) != 1 {
		t.Fatalf("pie, cita y mención en una foto: %+v", im)
	}
	if a.BotPausado(1, "120@g.us", s.ahora()) {
		t.Fatal("no pausa al bot")
	}
}
