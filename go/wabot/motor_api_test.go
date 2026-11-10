package wabot

import (
	"context"
	"sync"
	"testing"
	"time"

	"botchat/internal/motor"

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
	anterior := decisorGlobal
	decisorGlobal = cap
	defer func() { decisorGlobal = anterior }()
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
