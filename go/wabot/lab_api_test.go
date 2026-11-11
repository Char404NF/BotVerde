package wabot

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	"botchat/internal/almacen"
	"botchat/internal/envio"
	"botchat/internal/lab"
	"botchat/internal/motor"
)

func TestApiLabAjustesValidanYExigenElAviso(t *testing.T) {
	_, a := abreSolo(t)
	st := decodifica[estadoLab](t, func() (string, error) { return EstadoLab(1) })
	if st.Activo || st.AvisoAceptado || st.Modo != "texto" || st.ModoEfectivo != "texto" || st.Grupos || st.EnMarcha || len(st.Plan) < 10 || st.EncMax != 12 {
		t.Fatalf("por defecto todo apagado: %+v", st)
	}
	// sin aceptar el aviso no se enciende ni se elige otro modo
	if err := FijarAjusteLab(1, "lab_activo", "1"); err == nil {
		t.Fatal("sin aviso no enciende")
	}
	if err := FijarAjusteLab(1, "lab_modo_menu", "encuesta"); err == nil {
		t.Fatal("sin aviso no hay encuesta")
	}
	if err := AceptarAvisoLab(1); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"lab_activo": "1", "lab_modo_menu": "encuesta", "lab_grupos": "1", "lab_enc_max_opc": "10", "lab_botones_tipo": "lista",
		"lab_expira_min": "30", "lab_respaldo_enc": "1", "lab_prueba_hora": "3", "lab_prueba_dia": "9", "lab_version_wa": "2.26.1"} {
		if err := FijarAjusteLab(1, k, v); err != nil {
			t.Fatalf("%s=%s: %v", k, v, err)
		}
	}
	for k, v := range map[string]string{"lab_activo": "2", "lab_modo_menu": "magia", "lab_enc_max_opc": "99", "lab_botones_tipo": "x",
		"lab_expira_min": "0", "lab_prueba_hora": "0", "lab_prueba_chat": "no-es-jid@@", "lab_otro": "1", "prefijo": "!"} {
		if err := FijarAjusteLab(1, k, v); err == nil {
			t.Fatalf("%s=%s debía rechazarse", k, v)
		}
	}
	// el chat de prueba debe existir
	if err := FijarAjusteLab(1, "lab_prueba_chat", "5215599999999@s.whatsapp.net"); err == nil {
		t.Fatal("el chat no existe")
	}
	_, _ = a.ActualizarChat(1, "5215599999999@s.whatsapp.net", false, func(c *almacen.Chat) { c.Publico = "Mi otro número" })
	if err := FijarAjusteLab(1, "lab_prueba_chat", "5215599999999@s.whatsapp.net"); err != nil {
		t.Fatal(err)
	}
	st = decodifica[estadoLab](t, func() (string, error) { return EstadoLab(1) })
	if !st.Activo || !st.AvisoAceptado || st.ModoEfectivo != "encuesta" || !st.Grupos || st.EncMax != 10 || st.PruebaChat == "" || st.VersionWA != "2.26.1" {
		t.Fatalf("%+v", st)
	}
	// apagar el interruptor deja el menú en texto aunque el modo siga elegido
	_ = FijarAjusteLab(1, "lab_activo", "0")
	st = decodifica[estadoLab](t, func() (string, error) { return EstadoLab(1) })
	if st.Modo != "encuesta" || st.ModoEfectivo != "texto" {
		t.Fatalf("%+v", st)
	}
	// el camino general de ajustes no deja saltarse el aviso
	if err := FijarAjusteDePermisos(1, "lab_activo", "1"); err == nil {
		t.Fatal("lab_* solo se cambia por FijarAjusteLab")
	}
	// sin el bot en marcha no se pueden mandar pruebas
	if _, err := EnviarPruebaLab(1, "encuesta", 4); err == nil {
		t.Fatal("hace falta el bot en marcha")
	}
}

func TestApiLabMatrizMarcarBorrarYUsarLimite(t *testing.T) {
	_, a := abreSolo(t)
	now := time.Now()
	for _, n := range []string{"8", "12", "13"} {
		id, _ := a.NuevoResultadoLab(1, "encuesta", n, "x@s.whatsapp.net", "", "", now)
		a.ResultadoLabEnviado(1, id, "M"+n, true, "")
	}
	rs := decodifica[[]almacen.ResultadoLab](t, func() (string, error) { return ListarResultadosLab(1) })
	if len(rs) != 3 || rs[0].Variante != "13" {
		t.Fatalf("%+v", rs)
	}
	if err := MarcarResultadoLab(1, rs[0].ID, "llego", 1); err == nil {
		t.Fatal("«llegó» lo anota el bot, no la persona")
	}
	if _, err := UsarLimiteMedido(1); err == nil {
		t.Fatal("sin pruebas marcadas no hay límite")
	}
	_ = MarcarResultadoLab(1, rs[2].ID, "visible", 1) // 8
	_ = MarcarResultadoLab(1, rs[1].ID, "visible", 1) // 12
	_ = MarcarResultadoLab(1, rs[0].ID, "visible", 0) // 13 no se ve
	_ = MarcarResultadoLab(1, rs[0].ID, "tocable", 0)
	n, err := UsarLimiteMedido(1)
	if err != nil || n != 12 || a.AjusteBotInt(1, "lab_enc_max_opc") != 12 {
		t.Fatal(n, err)
	}
	if st := decodifica[estadoLab](t, func() (string, error) { return EstadoLab(1) }); st.LimiteMedido != 12 || st.PruebasDia != 3 {
		t.Fatalf("%+v", st)
	}
	if c, err := BorrarResultadosLab(1); err != nil || c != 3 {
		t.Fatal(c, err)
	}
}

// ---------- el envío de un menú por el enviador del motor ----------

type clienteQueFalla struct {
	*clienteFalso
	mu2   sync.Mutex
	falla bool
}

func (c *clienteQueFalla) SendMessage(ctx context.Context, to types.JID, m *waE2E.Message, extra ...whatsmeow.SendRequestExtra) (whatsmeow.SendResponse, error) {
	c.mu2.Lock()
	f := c.falla && m.GetPollCreationMessage() != nil
	c.mu2.Unlock()
	if f {
		return whatsmeow.SendResponse{}, errors.New("rechazado")
	}
	return c.clienteFalso.SendMessage(ctx, to, m, extra...)
}

func TestEnviadorMandaElMenuComoEncuestaYSiFallaSaleEnTexto(t *testing.T) {
	_, a := abreSolo(t)
	const ana = "5215511111111@s.whatsapp.net"
	_, _ = a.ActualizarChat(1, ana, false, func(c *almacen.Chat) { c.Publico = "Ana" })
	cli := &clienteQueFalla{clienteFalso: &clienteFalso{}}
	s := &estado{botID: 1, l: &falso{}}
	s.envio = envio.Nuevo(1, a, cli, nil)
	pn := types.NewJID("5215500000000", types.DefaultUserServer)
	s.lab = lab.Nuevo(1, a, lab.Real{Client: whatsmeow.NewClient(&store.Device{ID: &pn}, waLog.Noop)}, s.envio, nil)
	e := enviadorMotor{s}
	ctx := context.Background()
	sal := motor.Salida{Texto: "¿Qué necesitas?\n1. Horario\n2. Precios", Menu: &motor.MenuLab{Persona: ana, Titulo: "¿Qué necesitas?",
		Opciones: []motor.OpcionMenu{{Num: 1, Texto: "Horario"}, {Num: 2, Texto: "Precios"}}}}
	// Laboratorio apagado: texto numerado de siempre
	if id, err := e.EnviarSalida(ctx, ana, sal, "", "", ""); err != nil || id == "" {
		t.Fatal(id, err)
	}
	if cli.enviados[0].GetPollCreationMessage() != nil || cli.enviados[0].GetConversation() != sal.Texto {
		t.Fatalf("apagado debe ser texto: %+v", cli.enviados[0])
	}
	// encendido en modo encuesta
	_ = a.FijarAjusteBot(1, "lab_activo", "1")
	_ = a.FijarAjusteBot(1, "lab_aviso_ts", "1700000000")
	_ = a.FijarAjusteBot(1, "lab_modo_menu", "encuesta")
	id, err := e.EnviarSalida(ctx, ana, sal, "", "", "")
	if err != nil || id == "" || cli.enviados[1].GetPollCreationMessage().GetName() != "¿Qué necesitas?" || len(cli.enviados) != 2 {
		t.Fatalf("encuesta: %v %v", err, cli.enviados)
	}
	if ses := a.ObtenerSesionLab(1, id); ses == nil || ses.Persona != ana || len(ses.Secreto) != 32 {
		t.Fatalf("sesión: %+v", ses)
	}
	// con el respaldo en texto, sale también el menú numerado
	_ = a.FijarAjusteBot(1, "lab_respaldo_enc", "1")
	if _, err := e.EnviarSalida(ctx, ana, sal, "", "", ""); err != nil || len(cli.enviados) != 4 || cli.enviados[3].GetConversation() != sal.Texto {
		t.Fatalf("respaldo: %v %d", err, len(cli.enviados))
	}
	// si WhatsApp rechaza la encuesta, la persona recibe el texto numerado
	cli.falla = true
	if id, err := e.EnviarSalida(ctx, ana, sal, "", "", ""); err != nil || id == "" {
		t.Fatal(id, err)
	}
	if cli.enviados[len(cli.enviados)-1].GetConversation() != sal.Texto {
		t.Fatalf("debía caer a texto: %+v", cli.enviados[len(cli.enviados)-1])
	}
}

// ---------- de un voto o un toque al menú ----------

type memSecretos struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (m *memSecretos) PutMessageSecrets(ctx context.Context, ins []store.MessageSecretInsert) error {
	for _, i := range ins {
		_ = m.PutMessageSecret(ctx, i.Chat, i.Sender, i.ID, i.Secret)
	}
	return nil
}
func (m *memSecretos) PutMessageSecret(_ context.Context, chat, sender types.JID, id types.MessageID, secret []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.m == nil {
		m.m = map[string][]byte{}
	}
	m.m[chat.ToNonAD().String()+"|"+sender.ToNonAD().String()+"|"+string(id)] = secret
	return nil
}
func (m *memSecretos) GetMessageSecret(_ context.Context, chat, sender types.JID, id types.MessageID) ([]byte, types.JID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v, ok := m.m[chat.ToNonAD().String()+"|"+sender.ToNonAD().String()+"|"+string(id)]; ok {
		return v, sender, nil
	}
	return nil, types.EmptyJID, nil
}

func TestVotoYToqueLlegaAlMotorComoElNumeroDeLaOpcion(t *testing.T) {
	reinicia(t)
	cap := &capturaDecisor{}
	decisorPrueba = cap
	defer func() { decisorPrueba = nil }()
	ConfigurarMotor(true, 10)
	raiz := t.TempDir()
	if err := AbrirAlmacen(raiz, clavePrueba(), nil); err != nil {
		t.Fatal(err)
	}
	if err := Start(raiz, 1, clavePrueba(), &falso{}); err != nil {
		t.Fatal(err)
	}
	defer Stop()
	mu.Lock()
	s := st
	mu.Unlock()
	yo := types.NewJID("5215500000000", types.DefaultUserServer)
	s.cli.Store.ID = &yo
	anaJ, _ := types.ParseJID("5215511111111@s.whatsapp.net")
	ana := anaJ.String()
	a, _ := almacenAbierto()
	_ = a.FijarAjusteBot(1, "lab_activo", "1")
	_ = a.FijarAjusteBot(1, "lab_aviso_ts", "1700000000")
	// Ana ya es un contacto conocido
	_, _ = a.ActualizarChat(1, ana, false, func(c *almacen.Chat) { c.Publico = "Ana"; c.TelE164 = "5215511111111" })

	// una encuesta que salió a Ana (el secreto lo guardó la librería al enviar, aquí lo hacemos igual)
	secreto := make([]byte, 32)
	for i := range secreto {
		secreto[i] = byte(i + 1)
	}
	s.cli.Store.MsgSecrets = &memSecretos{}
	if err := s.cli.Store.MsgSecrets.PutMessageSecret(context.Background(), anaJ, yo, "POLL1", secreto); err != nil {
		t.Fatal(err)
	}
	ses := almacen.SesionLab{ID: "POLL1", Chat: ana, Persona: ana, Tipo: almacen.LabEncuesta, TS: time.Now().Unix(), ExpiraTS: time.Now().Add(time.Hour).Unix(),
		Secreto: secreto, Opciones: []almacen.OpcionLab{{Num: 1, Texto: "Horario"}, {Num: 2, Texto: "Precios"}, {Num: 0, Texto: "Menú principal"}}}
	if err := a.GuardarSesionLab(1, ses); err != nil {
		t.Fatal(err)
	}
	// Ana vota «Precios» con la librería real
	vlid := types.NewJID("99887766", types.HiddenUserServer)
	votan := whatsmeow.NewClient(&store.Device{ID: &anaJ, LID: vlid, MsgSecrets: &memSecretos{}}, waLog.Noop)
	_ = votan.Store.MsgSecrets.PutMessageSecret(context.Background(), yo, yo, "POLL1", secreto)
	vm, err := votan.BuildPollVote(context.Background(), &types.MessageInfo{MessageSource: types.MessageSource{Chat: yo, Sender: yo}, ID: "POLL1"}, []string{"Precios"})
	if err != nil {
		t.Fatal(err)
	}
	ev := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: anaJ, Sender: anaJ}, ID: "VOTO1", Timestamp: time.Now(), PushName: "Ana"}, Message: vm, RawMessage: vm}
	s.evento(ev)
	v := cap.espera(t, 1)
	if v[0].Texto != "2" || !v[0].EsTexto || v[0].Chat != ana || v[0].ID != "VOTO1" || v[0].EsGrupo {
		t.Fatalf("el voto debía llegar como «2»: %+v", v[0])
	}
	// el mismo voto repetido (otro id) no vuelve a llegar; cambiar a «Horario» sí
	dup := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: anaJ, Sender: anaJ}, ID: "VOTO2", Timestamp: time.Now()}, Message: vm, RawMessage: vm}
	s.evento(dup)
	vm2, _ := votan.BuildPollVote(context.Background(), &types.MessageInfo{MessageSource: types.MessageSource{Chat: yo, Sender: yo}, ID: "POLL1"}, []string{"Horario"})
	s.evento(&events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: anaJ, Sender: anaJ}, ID: "VOTO3", Timestamp: time.Now()}, Message: vm2, RawMessage: vm2})
	v = cap.espera(t, 2)
	time.Sleep(200 * time.Millisecond)
	cap.mu.Lock()
	n := len(cap.vistos)
	ultimo := cap.vistos[len(cap.vistos)-1]
	cap.mu.Unlock()
	if n != 2 || ultimo.Texto != "1" || ultimo.ID != "VOTO3" {
		t.Fatalf("duplicado ignorado y cambio aceptado: n=%d %+v", n, ultimo)
	}

	// un toque en un botón
	ses2 := almacen.SesionLab{ID: "BTN1", Chat: ana, Persona: ana, Tipo: almacen.LabBotones, TS: time.Now().Unix(), ExpiraTS: time.Now().Add(time.Hour).Unix(),
		Opciones: []almacen.OpcionLab{{Num: 1, Texto: "Horario", ID: "bc|BTN1|1"}, {Num: 5, Texto: "Pedidos", ID: "bc|BTN1|5"}}}
	_ = a.GuardarSesionLab(1, ses2)
	tq := &waE2E.Message{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{SelectedButtonID: proto.String("bc|BTN1|5"), Response: &waE2E.ButtonsResponseMessage_SelectedDisplayText{SelectedDisplayText: "Pedidos"},
		ContextInfo: &waE2E.ContextInfo{StanzaID: proto.String("BTN1")}}}
	s.evento(&events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: anaJ, Sender: anaJ}, ID: "TOQUE1", Timestamp: time.Now(), PushName: "Ana"}, Message: tq, RawMessage: tq})
	v = cap.espera(t, 3)
	if v[2].Texto != "5" || v[2].ID != "TOQUE1" {
		t.Fatalf("el toque debía llegar como «5»: %+v", v[2])
	}
	// un botón de otro menú sigue el camino de siempre: llega como el texto que muestra
	aj := &waE2E.Message{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{SelectedButtonID: proto.String("otro"), Response: &waE2E.ButtonsResponseMessage_SelectedDisplayText{SelectedDisplayText: "Sí"}}}
	s.evento(&events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: anaJ, Sender: anaJ}, ID: "TOQUE2", Timestamp: time.Now()}, Message: aj, RawMessage: aj})
	v = cap.espera(t, 4)
	if v[3].Texto != "Sí" {
		t.Fatalf("botón ajeno: %+v", v[3])
	}
}
