package wabot

import (
	"context"
	"sync"
	"testing"
	"time"

	"botchat/internal/motor"

	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

type capturaDecisor struct {
	mu     sync.Mutex
	vistos []motor.Mensaje
}

func (c *capturaDecisor) Decidir(_ context.Context, u motor.Mensaje, _ []motor.Mensaje) (motor.Decision, error) {
	c.mu.Lock()
	c.vistos = append(c.vistos, u)
	c.mu.Unlock()
	return motor.Decision{}, nil
}

func (c *capturaDecisor) espera(t *testing.T, n int) []motor.Mensaje {
	t.Helper()
	for i := 0; i < 300; i++ {
		c.mu.Lock()
		if len(c.vistos) >= n {
			v := append([]motor.Mensaje(nil), c.vistos...)
			c.mu.Unlock()
			return v
		}
		c.mu.Unlock()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("el motor no recibió %d mensajes", n)
	return nil
}

func TestEventosDeMensajeLleganAlMotorConMencionCitaYSincronizacion(t *testing.T) {
	reinicia(t)
	cap := &capturaDecisor{}
	decisorPrueba = cap
	defer func() { decisorPrueba = nil }()
	ConfigurarMotor(true, 10)
	f := &falso{}
	raiz := t.TempDir()
	if err := AbrirAlmacen(raiz, clavePrueba(), nil); err != nil {
		t.Fatal(err)
	}
	if err := Start(raiz, 1, clavePrueba(), f); err != nil {
		t.Fatal(err)
	}
	defer Stop()
	mu.Lock()
	s := st
	mu.Unlock()
	yo := types.NewJID("5215500000000", types.DefaultUserServer)
	s.cli.Store.ID = &yo
	grupo, _ := types.ParseJID("120363000000000001@g.us")
	ana, _ := types.ParseJID("5215511111111@s.whatsapp.net")
	msg := func(id string, m *waE2E.Message) *events.Message {
		e := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: grupo, Sender: ana, IsGroup: true}, ID: id, Timestamp: time.Now(), PushName: "Ana"}, RawMessage: m}
		e.UnwrapRaw()
		return e
	}
	s.evento(msg("M1", &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("hola @5215500000000"),
		ContextInfo: &waE2E.ContextInfo{MentionedJID: []string{yo.String()}}}}))
	s.evento(msg("M2", &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("gracias"),
		ContextInfo: &waE2E.ContextInfo{StanzaID: proto.String("MIO"), Participant: proto.String(yo.String())}}}))
	s.evento(&events.OfflineSyncPreview{Total: 5})
	s.evento(msg("M3", &waE2E.Message{Conversation: proto.String(".menu")}))
	s.evento(&events.OfflineSyncCompleted{Count: 5})
	vistos := cap.espera(t, 2)
	por := map[string]motor.Mensaje{}
	for _, v := range vistos {
		por[v.ID] = v
	}
	if m := por["M1"]; !m.Mencionado || !m.EsGrupo || !m.EsTexto || m.RemitenteWA != "5215511111111@s.whatsapp.net" || m.Texto != "hola @5215500000000" {
		t.Fatalf("M1: %+v", m)
	}
	if m := por["M2"]; !m.CitaEsMia || m.Mencionado {
		t.Fatalf("M2: %+v", m)
	}
	// Lo que llega durante la sincronización nunca llega al decisor: se anota "omitido: antiguo".
	var act string
	for i := 0; i < 200; i++ {
		act, _ = ActividadMotor(1, 50)
		if contiene(act, `"motivo":"antiguo"`) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !contiene(act, `"motivo":"antiguo"`) {
		t.Fatalf("falta el omitido por antiguo: %s", act)
	}
	if _, ok := por["M3"]; ok {
		t.Fatal("M3 no debía llegar al decisor")
	}
}

func TestConfigurarMotorYPausas(t *testing.T) {
	reinicia(t)
	if err := AbrirAlmacen(t.TempDir(), clavePrueba(), nil); err != nil {
		t.Fatal(err)
	}
	ConfigurarMotor(false, 3)
	if motorActivo.Load() || ventanaAntiguaMin.Load() != 3 {
		t.Fatal("ajustes")
	}
	ConfigurarMotor(true, 0)
	if !motorActivo.Load() || ventanaAntiguaMin.Load() != 10 {
		t.Fatal("0 = valor por defecto (10)")
	}
	if err := PausarBotEnChat(1, "5215511111111@s.whatsapp.net", 15); err != nil {
		t.Fatal(err)
	}
	est, _ := EstadoDelChat(1, "5215511111111@s.whatsapp.net")
	if !contiene(est, `"botPausaHasta":`) || contiene(est, `"botPausaHasta":0`) {
		t.Fatal(est)
	}
	_ = PausarBotEnChat(1, "5215511111111@s.whatsapp.net", 0)
	est, _ = EstadoDelChat(1, "5215511111111@s.whatsapp.net")
	if !contiene(est, `"botPausaHasta":0`) {
		t.Fatal(est)
	}
	_ = BotEnChat(1, "120@g.us", false)
	est, _ = EstadoDelChat(1, "120@g.us")
	if !contiene(est, `"botActivo":false`) {
		t.Fatal(est)
	}
}

func contiene(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestSaludoDeFabricaCoincideConUnMensajePrivado(t *testing.T) {
	reinicia(t)
	decisorPrueba = nil
	ConfigurarMotor(true, 10)
	f := &falso{}
	raiz := t.TempDir()
	if err := AbrirAlmacen(raiz, clavePrueba(), nil); err != nil {
		t.Fatal(err)
	}
	if err := Start(raiz, 1, clavePrueba(), f); err != nil {
		t.Fatal(err)
	}
	defer Stop()
	l, _ := ListarAutomatizaciones(1)
	if !contiene(l, `"nombre":"Saludo"`) || !contiene(l, `"sistema":true`) || !contiene(l, `"activo":true`) {
		t.Fatalf("el Saludo de fábrica debe estar creado y activo: %s", l)
	}
	mu.Lock()
	s := st
	mu.Unlock()
	ana, _ := types.ParseJID("34600000001@s.whatsapp.net")
	e := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: ana, Sender: ana}, ID: "P1", Timestamp: time.Now(), PushName: "Ana"},
		RawMessage: &waE2E.Message{Conversation: proto.String("buenas")}}
	e.UnwrapRaw()
	s.evento(e)
	var act string
	for i := 0; i < 300; i++ {
		act, _ = ActividadMotor(1, 50)
		if contiene(act, `"evento":"coincidio"`) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !contiene(act, `"evento":"coincidio"`) || !contiene(act, `"detalle":"Saludo"`) {
		t.Fatalf("debió coincidir con el Saludo: %s", act)
	}
}

func TestReactivarElChatCierraLasConversacionesConUnaPersona(t *testing.T) {
	reinicia(t)
	if err := AbrirAlmacen(t.TempDir(), clavePrueba(), nil); err != nil {
		t.Fatal(err)
	}
	a, _ := almacenAbierto()
	chat := "34600000001@s.whatsapp.net"
	now := time.Now()
	_ = a.SilenciarSesion(1, chat, chat, 1, 1, "humano", now.Add(12*time.Hour), now)
	_ = a.PausarBotHumano(1, chat, now.Add(12*time.Hour).Unix())
	if s, _ := a.SesionVigente(1, chat, chat, false, now, time.Hour); s == nil {
		t.Fatal("debía haber una conversación con una persona")
	}
	if err := PausarBotEnChat(1, chat, 0); err != nil {
		t.Fatal(err)
	}
	if s, _ := a.SesionVigente(1, chat, chat, false, now, time.Hour); s != nil {
		t.Fatal("al reactivar se cierra la conversación con una persona")
	}
	if a.BotPausado(1, chat, now) {
		t.Fatal("y el chat deja de estar en pausa")
	}
}

func TestElBotNoReaccionaALasEliminacionesNiALasReacciones(t *testing.T) {
	reinicia(t)
	decisorPrueba = nil
	ConfigurarMotor(true, 10)
	f := &falso{}
	raiz := t.TempDir()
	if err := AbrirAlmacen(raiz, clavePrueba(), nil); err != nil {
		t.Fatal(err)
	}
	if err := Start(raiz, 1, clavePrueba(), f); err != nil {
		t.Fatal(err)
	}
	defer Stop()
	mu.Lock()
	s := st
	mu.Unlock()
	ana, _ := types.ParseJID("34600000002@s.whatsapp.net")
	base := types.MessageInfo{MessageSource: types.MessageSource{Chat: ana, Sender: ana}, Timestamp: time.Now(), PushName: "Ana"}
	enviar := func(id string, m *waE2E.Message) {
		e := &events.Message{Info: base, RawMessage: m}
		e.Info.ID = id
		e.UnwrapRaw()
		s.evento(e)
	}
	enviar("T1", &waE2E.Message{Conversation: proto.String("hola")}) // este sí llega al motor
	for i := 0; i < 200; i++ {
		if a, _ := ActividadMotor(1, 50); contiene(a, `"evento":"llego"`) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	antes, _ := ActividadMotor(1, 500)
	llegadas := func(a string) int {
		n := 0
		for i := 0; i+len(`"evento":"llego"`) <= len(a); i++ {
			if a[i:i+len(`"evento":"llego"`)] == `"evento":"llego"` {
				n++
			}
		}
		return n
	}
	enviar("R1", &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{Type: waE2E.ProtocolMessage_REVOKE.Enum(), Key: &waCommon.MessageKey{ID: proto.String("T1")}}})
	enviar("X1", &waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{Key: &waCommon.MessageKey{ID: proto.String("T1")}, Text: proto.String("👍")}})
	enviar("E1", &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{Type: waE2E.ProtocolMessage_MESSAGE_EDIT.Enum(), Key: &waCommon.MessageKey{ID: proto.String("T1")}, EditedMessage: &waE2E.Message{Conversation: proto.String("otra cosa")}}})
	time.Sleep(500 * time.Millisecond)
	despues, _ := ActividadMotor(1, 500)
	if llegadas(antes) != 1 || llegadas(despues) != 1 {
		t.Fatalf("eliminaciones, reacciones y ediciones no deben llegar al motor: antes=%d después=%d\n%s", llegadas(antes), llegadas(despues), despues)
	}
}
