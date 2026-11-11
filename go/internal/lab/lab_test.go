package lab

import (
	"context"
	"errors"
	"strings"
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
	"botchat/internal/motor"
)

// ---------- piezas de prueba ----------

type memSecretos struct {
	mu sync.Mutex
	m  map[string][]byte
	s  map[string]types.JID
}

func nuevaMem() *memSecretos { return &memSecretos{m: map[string][]byte{}, s: map[string]types.JID{}} }

func clave(chat, sender types.JID, id types.MessageID) string {
	return chat.ToNonAD().String() + "|" + sender.ToNonAD().String() + "|" + string(id)
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
	m.m[clave(chat, sender, id)] = secret
	m.s[clave(chat, sender, id)] = sender
	return nil
}
func (m *memSecretos) GetMessageSecret(_ context.Context, chat, sender types.JID, id types.MessageID) ([]byte, types.JID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := clave(chat, sender, id)
	if v, ok := m.m[k]; ok {
		return v, m.s[k], nil
	}
	return nil, types.EmptyJID, nil
}

func cliente(pn, lid types.JID) (*whatsmeow.Client, *memSecretos) {
	mem := nuevaMem()
	return whatsmeow.NewClient(&store.Device{ID: &pn, LID: lid, MsgSecrets: mem}, waLog.Noop), mem
}

// clienteFalso usa la librería real para armar y abrir encuestas.
type clienteFalso struct {
	*whatsmeow.Client
	mem *memSecretos
}

func (c clienteFalso) PutMessageSecret(ctx context.Context, chat, sender types.JID, id types.MessageID, secret []byte) error {
	return c.mem.PutMessageSecret(ctx, chat, sender, id, secret)
}
func (c clienteFalso) VersionCliente() string { return "2.3000.test" }

// emisorFalso hace lo mismo que envio.Servicio sin red: genera el id, arma, llama a antes y guarda el secreto como SendMessage.
type emisorFalso struct {
	mu       sync.Mutex
	n        int
	cuenta   int
	propio   types.JID
	mem      *memSecretos
	enviados []*waE2E.Message
	chats    []string
	resumen  []string
	falla    error
}

func (e *emisorFalso) EnviarInteractivo(_ context.Context, chat, resumen, citaID, citaAutorWA string, armar func(string, *waE2E.ContextInfo) *waE2E.Message, antes func(string, *waE2E.Message) error) (envio.Resultado, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.n++
	id := "ID" + string(rune('A'+e.n))
	m := armar(id, nil)
	if antes != nil {
		if err := antes(id, m); err != nil {
			return envio.Resultado{}, err
		}
	}
	if e.falla != nil {
		return envio.Resultado{ID: id, Estado: "fallido"}, e.falla
	}
	if sec := m.GetMessageContextInfo().GetMessageSecret(); sec != nil {
		to, _ := types.ParseJID(chat)
		_ = e.mem.PutMessageSecret(context.Background(), to, e.propio, types.MessageID(id), sec)
	}
	e.enviados = append(e.enviados, m)
	e.chats = append(e.chats, chat)
	e.resumen = append(e.resumen, resumen)
	return envio.Resultado{ID: id, Estado: "enviado"}, nil
}

const (
	jidBot      = "5215500000000@s.whatsapp.net"
	jidVotante  = "5215511111111@s.whatsapp.net"
	jidVotanteL = "99887766@lid"
	jidBotL     = "11223344@lid"
)

type entorno struct {
	t     *testing.T
	s     *Servicio
	alm   *almacen.Almacen
	bot   *whatsmeow.Client
	votan *whatsmeow.Client
	vmem  *memSecretos
	em    *emisorFalso
	ahora time.Time
}

func nuevoEntorno(t *testing.T) *entorno {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 3)
	}
	alm, err := almacen.Abrir(t.TempDir(), k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = alm.Cerrar() })
	pn, _ := types.ParseJID(jidBot)
	lid, _ := types.ParseJID(jidBotL)
	bot, bmem := cliente(pn, lid)
	vpn, _ := types.ParseJID(jidVotante)
	vlid, _ := types.ParseJID(jidVotanteL)
	votan, vmem := cliente(vpn, vlid)
	em := &emisorFalso{propio: pn, mem: bmem}
	s := Nuevo(1, alm, clienteFalso{bot, bmem}, em, func(string) {})
	e := &entorno{t: t, s: s, alm: alm, bot: bot, votan: votan, vmem: vmem, em: em, ahora: time.Now()}
	s.Ahora = func() time.Time { return e.ahora }
	return e
}

func (e *entorno) fija(k, v string) {
	e.t.Helper()
	if err := e.alm.FijarAjusteBot(1, k, v); err != nil {
		e.t.Fatal(err)
	}
}

// enciende deja el Laboratorio habilitado en el modo dado.
func (e *entorno) enciende(modo string) {
	e.fija("lab_activo", "1")
	e.fija("lab_aviso_ts", "1700000000")
	e.fija("lab_modo_menu", modo)
}

func menu(n int) *motor.MenuLab {
	m := &motor.MenuLab{Persona: jidVotante, Titulo: "¿Qué necesitas?"}
	nombres := []string{"Horario", "Precios", "Ubicación", "Hablar con alguien", "Pedidos", "Facturas", "Garantía", "Soporte", "Envíos", "Cambios", "Promos", "Otros", "Extra1", "Extra2"}
	for i := 0; i < n; i++ {
		m.Opciones = append(m.Opciones, motor.OpcionMenu{Num: i + 1, Texto: nombres[i]})
	}
	return m
}

func resolverFijo(chat, persona string) Resolver {
	return func(_ context.Context, _ types.MessageSource) (string, string, bool) { return chat, persona, true }
}

// voto arma el evento de un voto de la persona (con la librería real) sobre la encuesta `pollID` que salió a `chatBot`.
func (e *entorno) voto(pollID string, opciones []string, chatVoto, remitente types.JID) *events.Message {
	e.t.Helper()
	bot, _ := types.ParseJID(jidBot)
	// el votante recibió la encuesta: guarda su secreto igual que lo haría al recibirla
	var sec []byte
	e.s.Cli.(clienteFalso).mem.mu.Lock()
	for k, v := range e.s.Cli.(clienteFalso).mem.m {
		if strings.HasSuffix(k, "|"+pollID) {
			sec = v
		}
	}
	e.s.Cli.(clienteFalso).mem.mu.Unlock()
	_ = e.vmem.PutMessageSecret(context.Background(), bot, bot, types.MessageID(pollID), sec)
	info := &types.MessageInfo{MessageSource: types.MessageSource{Chat: bot, Sender: bot}, ID: pollID}
	m, err := e.votan.BuildPollVote(context.Background(), info, opciones)
	if err != nil {
		e.t.Fatal(err)
	}
	return &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chatVoto, Sender: remitente}, ID: "V" + strings.Join(opciones, "")}, Message: m}
}

// ---------- envío ----------

func TestApagadoOEnTextoNoMandaNada(t *testing.T) {
	e := nuevoEntorno(t)
	for _, caso := range []func(){
		func() {}, // todo por defecto: apagado
		func() { e.fija("lab_activo", "1"); e.fija("lab_modo_menu", "encuesta") },           // sin aviso aceptado
		func() { e.fija("lab_aviso_ts", "1700000000"); e.fija("lab_modo_menu", ModoTexto) }, // en texto
		func() { e.fija("lab_modo_menu", "encuesta"); e.fija("lab_activo", "0") },           // apagado otra vez
	} {
		caso()
		_, ok, err := e.s.EnviarMenu(context.Background(), jidVotante, jidVotante, menu(3), "texto", "", "")
		if ok || err != nil || len(e.em.enviados) != 0 {
			t.Fatalf("debía salir como texto: ok=%v err=%v", ok, err)
		}
	}
	if e.s.Modo() != ModoTexto {
		t.Fatal("modo")
	}
}

func TestEncuestaSeArmaYGuardaSesionCifrada(t *testing.T) {
	e := nuevoEntorno(t)
	e.enciende(ModoEncuesta)
	r, ok, err := e.s.EnviarMenu(context.Background(), jidVotante, jidVotante, menu(3), "¿Qué necesitas?\n1. Horario\n2. Precios\n3. Ubicación", "", "")
	if !ok || err != nil || r.Tipo != almacen.LabEncuesta || r.Respaldo {
		t.Fatalf("envío: %+v %v %v", r, ok, err)
	}
	p := e.em.enviados[0].GetPollCreationMessage()
	if p == nil || p.GetName() != "¿Qué necesitas?" || len(p.GetOptions()) != 3 || p.GetOptions()[1].GetOptionName() != "Precios" || p.GetSelectableOptionsCount() != 1 {
		t.Fatalf("encuesta: %+v", p)
	}
	if !strings.Contains(e.em.resumen[0], "1. Horario") {
		t.Fatalf("el chat guarda el menú numerado: %q", e.em.resumen[0])
	}
	ses := e.alm.ObtenerSesionLab(1, r.ID)
	if ses == nil || ses.Tipo != almacen.LabEncuesta || len(ses.Secreto) != 32 || len(ses.Opciones) != 3 || ses.Persona != jidVotante || ses.Estado != almacen.LabActiva {
		t.Fatalf("sesión: %+v", ses)
	}
	if ses.ExpiraTS != e.ahora.Add(120*time.Minute).Unix() {
		t.Fatal("la caducidad sale del ajuste")
	}
	// el respaldo en texto, si se pide, se avisa a quien envía
	e.fija("lab_respaldo_enc", "1")
	r2, _, _ := e.s.EnviarMenu(context.Background(), jidVotante, jidVotante, menu(3), "", "", "")
	if !r2.Respaldo {
		t.Fatal("el respaldo en texto debía pedirse")
	}
}

func TestEncuestaLimitesGrupoYRepetidas(t *testing.T) {
	e := nuevoEntorno(t)
	e.enciende(ModoEncuesta)
	c := context.Background()
	// una sola opción no es encuesta; más que el tope tampoco
	if _, ok, _ := e.s.EnviarMenu(c, jidVotante, jidVotante, menu(1), "", "", ""); ok {
		t.Fatal("una opción sale en texto")
	}
	if _, ok, _ := e.s.EnviarMenu(c, jidVotante, jidVotante, menu(13), "", "", ""); ok {
		t.Fatal("13 > 12: sale en texto")
	}
	e.fija("lab_enc_max_opc", "14")
	if _, ok, _ := e.s.EnviarMenu(c, jidVotante, jidVotante, menu(13), "", "", ""); !ok {
		t.Fatal("con el tope en 14 sí sale como encuesta")
	}
	// en grupos solo si se permite
	if _, ok, _ := e.s.EnviarMenu(c, "g@g.us", jidVotante, menu(3), "", "", ""); ok {
		t.Fatal("grupo apagado: texto")
	}
	e.fija("lab_grupos", "1")
	if _, ok, _ := e.s.EnviarMenu(c, "g@g.us", jidVotante, menu(3), "", "", ""); !ok {
		t.Fatal("grupo permitido")
	}
	// nombres repetidos y largos
	m := &motor.MenuLab{Titulo: strings.Repeat("T", 300), Opciones: []motor.OpcionMenu{{Num: 1, Texto: "Igual"}, {Num: 2, Texto: "igual"}, {Num: 3, Texto: strings.Repeat("x", 150)}}}
	n := len(e.em.enviados)
	e.s.EnviarMenu(c, jidVotante, jidVotante, m, "", "", "")
	p := e.em.enviados[n].GetPollCreationMessage()
	if len([]rune(p.GetName())) > 255 || p.GetOptions()[0].GetOptionName() == p.GetOptions()[1].GetOptionName() || len([]rune(p.GetOptions()[2].GetOptionName())) > 100 {
		t.Fatalf("nombres: %+v", p)
	}
}

func TestFallosSeguidosVuelvenATexto(t *testing.T) {
	e := nuevoEntorno(t)
	e.enciende(ModoEncuesta)
	e.em.falla = errors.New("boom")
	var motivo string
	e.s.AlVolverATexto = func(m string) { motivo = m }
	for i := 0; i < 3; i++ {
		_, ok, err := e.s.EnviarMenu(context.Background(), jidVotante, jidVotante, menu(3), "", "", "")
		if ok || err == nil {
			t.Fatal("debía fallar")
		}
	}
	if e.alm.AjusteBot(1, "lab_modo_menu") != ModoTexto || motivo != "fallos_seguidos" {
		t.Fatalf("modo=%s motivo=%s", e.alm.AjusteBot(1, "lab_modo_menu"), motivo)
	}
	if e.alm.ContarSesionesLab(1) != 0 {
		t.Fatal("una sesión de un envío fallido no debe quedar")
	}
	// sin conexión no cuenta como fallo del modo
	e.enciende(ModoEncuesta)
	e.em.falla = envio.ErrSinConexion
	for i := 0; i < 5; i++ {
		e.s.EnviarMenu(context.Background(), jidVotante, jidVotante, menu(3), "", "", "")
	}
	if e.alm.AjusteBot(1, "lab_modo_menu") != ModoEncuesta {
		t.Fatal("estar sin conexión no cambia el modo")
	}
}

// ---------- votos ----------

func (e *entorno) encuesta() string {
	e.t.Helper()
	r, ok, err := e.s.EnviarMenu(context.Background(), jidVotante, jidVotante, menu(3), "", "", "")
	if !ok || err != nil {
		e.t.Fatalf("encuesta: %v %v", ok, err)
	}
	return r.ID
}

func TestVotoCambioRetiroYDuplicados(t *testing.T) {
	e := nuevoEntorno(t)
	e.enciende(ModoEncuesta)
	id := e.encuesta()
	c := context.Background()
	chat, _ := types.ParseJID(jidVotante)
	res := resolverFijo(jidVotante, jidVotante)

	r := e.s.Recibir(c, e.voto(id, []string{"Precios"}, chat, chat), res)
	if r == nil || r.Accion != AccionVoto || r.Num != 2 || r.Texto != "Precios" || !r.Navega || r.Votante != jidVotante {
		t.Fatalf("voto: %+v", r)
	}
	// el mismo voto llegando otra vez (otro id de mensaje) no hace nada
	d := e.voto(id, []string{"Precios"}, chat, chat)
	d.Info.ID = "OTRO"
	if r = e.s.Recibir(c, d, res); r.Accion != AccionDuplicado || r.Navega {
		t.Fatalf("duplicado: %+v", r)
	}
	// cambia de opinión
	if r = e.s.Recibir(c, e.voto(id, []string{"Horario"}, chat, chat), res); r.Accion != AccionCambio || r.Num != 1 || !r.Navega {
		t.Fatalf("cambio: %+v", r)
	}
	// retira el voto
	if r = e.s.Recibir(c, e.voto(id, nil, chat, chat), res); r.Accion != AccionRetiro || r.Navega {
		t.Fatalf("retiro: %+v", r)
	}
	// retirarlo otra vez es duplicado
	rr := e.voto(id, nil, chat, chat)
	rr.Info.ID = "R2"
	if r = e.s.Recibir(c, rr, res); r.Accion != AccionDuplicado {
		t.Fatalf("retiro repetido: %+v", r)
	}
	// y votar de nuevo después de retirar cuenta como voto nuevo
	if r = e.s.Recibir(c, e.voto(id, []string{"Ubicación"}, chat, chat), res); r.Accion != AccionVoto || r.Num != 3 {
		t.Fatalf("voto tras retiro: %+v", r)
	}
	// varias opciones en una encuesta de una sola
	if r = e.s.Recibir(c, e.voto(id, []string{"Horario", "Precios"}, chat, chat), res); r.Accion != AccionVarias || r.Navega {
		t.Fatalf("varias: %+v", r)
	}
	// una opción que no es nuestra
	if r = e.s.Recibir(c, e.voto(id, []string{"Inventada"}, chat, chat), res); r.Accion != AccionSinOpcion || r.Navega {
		t.Fatalf("sin opción: %+v", r)
	}
}

func TestVotoPorChatConLIDSeRecuperaConNuestraCopiaDelSecreto(t *testing.T) {
	e := nuevoEntorno(t)
	e.enciende(ModoEncuesta)
	id := e.encuesta() // salió al chat con número
	c := context.Background()
	lid, _ := types.ParseJID(jidVotanteL)
	// el voto llega por el chat con LID: la librería no encuentra el secreto (se guardó bajo el número)
	botL, _ := types.ParseJID(jidBotL)
	_ = botL
	ev := e.votoLID(id, []string{"Precios"}, lid)
	if _, err := e.s.Cli.DecryptPollVote(c, ev); err == nil {
		t.Fatal("la librería sola no debía poder abrirlo (si puede, el caso LID ya no hace falta)")
	}
	// la normalización de identidades lleva el LID al chat canónico (el número)
	r := e.s.Recibir(c, ev, resolverFijo(jidVotante, jidVotante))
	if r == nil || r.Accion != AccionVoto || r.Num != 2 || !r.Navega {
		t.Fatalf("voto por LID: %+v", r)
	}
}

// votoLID arma un voto que llega por el chat LID del votante; el votante conoce al bot por su LID.
func (e *entorno) votoLID(pollID string, opciones []string, chatVoto types.JID) *events.Message {
	e.t.Helper()
	botL, _ := types.ParseJID(jidBotL)
	var sec []byte
	cf := e.s.Cli.(clienteFalso)
	cf.mem.mu.Lock()
	for k, v := range cf.mem.m {
		if strings.HasSuffix(k, "|"+pollID) {
			sec = v
		}
	}
	cf.mem.mu.Unlock()
	_ = e.vmem.PutMessageSecret(context.Background(), botL, botL, types.MessageID(pollID), sec)
	info := &types.MessageInfo{MessageSource: types.MessageSource{Chat: botL, Sender: botL}, ID: pollID}
	m, err := e.votan.BuildPollVote(context.Background(), info, opciones)
	if err != nil {
		e.t.Fatal(err)
	}
	return &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chatVoto, Sender: chatVoto}, ID: "VL"}, Message: m}
}

func TestVotosViejosExpiradosAjenosYDeOtrosChats(t *testing.T) {
	e := nuevoEntorno(t)
	e.enciende(ModoEncuesta)
	c := context.Background()
	chat, _ := types.ParseJID(jidVotante)
	res := resolverFijo(jidVotante, jidVotante)
	viejo := e.encuesta()
	nuevo := e.encuesta() // la nueva reemplaza a la vieja
	if e.alm.ObtenerSesionLab(1, viejo).Estado != almacen.LabReemplazada {
		t.Fatal("la vieja debía quedar reemplazada")
	}
	if r := e.s.Recibir(c, e.voto(viejo, []string{"Horario"}, chat, chat), res); r.Accion != AccionVieja || r.Navega {
		t.Fatalf("vieja: %+v", r)
	}
	if r := e.s.Recibir(c, e.voto(nuevo, []string{"Horario"}, chat, chat), res); r.Accion != AccionVoto || !r.Navega {
		t.Fatalf("nueva: %+v", r)
	}
	// un voto en otro chat (no es de esa conversación)
	if r := e.s.Recibir(c, e.voto(nuevo, []string{"Precios"}, chat, chat), resolverFijo("otro@s.whatsapp.net", "otro@s.whatsapp.net")); r.Accion != AccionOtroChat || r.Navega {
		t.Fatalf("otro chat: %+v", r)
	}
	// caduca
	e.ahora = e.ahora.Add(3 * time.Hour)
	if r := e.s.Recibir(c, e.voto(nuevo, []string{"Ubicación"}, chat, chat), res); r.Accion != AccionExpirada || r.Navega {
		t.Fatalf("expirada: %+v", r)
	}
	if e.alm.ObtenerSesionLab(1, nuevo).Estado != almacen.LabExpirada {
		t.Fatal("quedó marcada expirada")
	}
	// una encuesta que no es nuestra, y mis propios votos desde otro teléfono
	otro := e.voto(nuevo, []string{"Horario"}, chat, chat)
	otro.Message.PollUpdateMessage.PollCreationMessageKey.ID = proto.String("NOESNUESTRA")
	if r := e.s.Recibir(c, otro, res); r == nil || r.Accion != AccionDesconoce || r.Navega {
		t.Fatalf("desconocida: %+v", r)
	}
	mio := e.voto(nuevo, []string{"Horario"}, chat, chat)
	mio.Info.IsFromMe = true
	if e.s.Recibir(c, mio, res) != nil {
		t.Fatal("lo que yo voto no cuenta")
	}
}

func TestGrupoSoloVotaLaPersonaDelMenu(t *testing.T) {
	e := nuevoEntorno(t)
	e.enciende(ModoEncuesta)
	e.fija("lab_grupos", "1")
	c := context.Background()
	grupo := "120363000000000000@g.us"
	r, ok, err := e.s.EnviarMenu(c, grupo, jidVotante, menu(3), "", "", "")
	if !ok || err != nil {
		t.Fatal(err)
	}
	// en el grupo el voto llega con Chat=grupo; los secretos se guardaron bajo (grupo, bot)
	gj, _ := types.ParseJID(grupo)
	bot, _ := types.ParseJID(jidBot)
	vot, _ := types.ParseJID(jidVotante)
	sec := e.alm.ObtenerSesionLab(1, r.ID).Secreto
	_ = e.vmem.PutMessageSecret(c, gj, bot, types.MessageID(r.ID), sec)
	info := &types.MessageInfo{MessageSource: types.MessageSource{Chat: gj, Sender: bot, IsGroup: true}, ID: r.ID}
	m, err := e.votan.BuildPollVote(c, info, []string{"Precios"})
	if err != nil {
		t.Fatal(err)
	}
	ev := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: gj, Sender: vot, IsGroup: true}, ID: "VG"}, Message: m}
	// otra persona del grupo vota: no cuenta
	if x := e.s.Recibir(c, ev, resolverFijo(grupo, "otra@s.whatsapp.net")); x.Accion != AccionAjeno || x.Navega {
		t.Fatalf("ajeno: %+v", x)
	}
	// la persona del menú vota: cuenta, con el grupo como chat
	if x := e.s.Recibir(c, ev, resolverFijo(grupo, jidVotante)); x.Accion != AccionVoto || !x.Navega || x.Num != 2 || x.Votante != jidVotante {
		t.Fatalf("voto en grupo: %+v", x)
	}
}

// ---------- botones, listas, interactivo ----------

func TestBotonesListasEInteractivoSeArmanYRespondenPorId(t *testing.T) {
	e := nuevoEntorno(t)
	e.enciende(ModoBotones)
	c := context.Background()
	res := resolverFijo(jidVotante, jidVotante)
	chat, _ := types.ParseJID(jidVotante)

	// auto: 3 o menos = botones; hasta 10 = lista; más = texto
	r, ok, err := e.s.EnviarMenu(c, jidVotante, jidVotante, menu(3), "", "", "")
	if !ok || err != nil || r.Tipo != almacen.LabBotones || !r.Respaldo {
		t.Fatalf("botones: %+v %v %v", r, ok, err)
	}
	b := e.em.enviados[0].GetButtonsMessage()
	if b == nil || len(b.GetButtons()) != 3 || b.GetButtons()[1].GetButtonText().GetDisplayText() != "Precios" || b.GetButtons()[1].GetButtonID() != "bc|"+r.ID+"|2" {
		t.Fatalf("botones armados: %+v", b)
	}
	// la respuesta de un botón
	resp := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chat, Sender: chat}, ID: "T1"},
		Message: &waE2E.Message{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{SelectedButtonID: proto.String("bc|" + r.ID + "|2"),
			ContextInfo: &waE2E.ContextInfo{StanzaID: proto.String(r.ID)}}}}
	x := e.s.Recibir(c, resp, res)
	if x == nil || x.Accion != AccionToque || x.Num != 2 || x.Texto != "Precios" || !x.Navega {
		t.Fatalf("toque: %+v", x)
	}

	r, _, _ = e.s.EnviarMenu(c, jidVotante, jidVotante, menu(8), "", "", "")
	if r.Tipo != almacen.LabLista {
		t.Fatalf("lista: %+v", r)
	}
	l := e.em.enviados[len(e.em.enviados)-1].GetListMessage()
	if l == nil || len(l.GetSections()[0].GetRows()) != 8 || l.GetListType() != waE2E.ListMessage_SINGLE_SELECT {
		t.Fatalf("lista armada: %+v", l)
	}
	resp = &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chat, Sender: chat}, ID: "T2"},
		Message: &waE2E.Message{ListResponseMessage: &waE2E.ListResponseMessage{SingleSelectReply: &waE2E.ListResponseMessage_SingleSelectReply{SelectedRowID: proto.String("bc|" + r.ID + "|5")},
			ContextInfo: &waE2E.ContextInfo{StanzaID: proto.String(r.ID)}}}}
	if x = e.s.Recibir(c, resp, res); x == nil || x.Num != 5 || x.Texto != "Pedidos" || !x.Navega {
		t.Fatalf("fila: %+v", x)
	}
	// más de 10 filas: texto
	if _, ok, _ := e.s.EnviarMenu(c, jidVotante, jidVotante, menu(12), "", "", ""); ok {
		t.Fatal("12 opciones salen en texto")
	}

	// interactivo con quick_reply
	e.fija("lab_botones_tipo", "interactivo")
	r, _, _ = e.s.EnviarMenu(c, jidVotante, jidVotante, menu(2), "", "", "")
	im := e.em.enviados[len(e.em.enviados)-1].GetInteractiveMessage()
	if r.Tipo != almacen.LabInteractivo || im == nil || im.GetNativeFlowMessage().GetButtons()[0].GetName() != "quick_reply" ||
		!strings.Contains(im.GetNativeFlowMessage().GetButtons()[0].GetButtonParamsJSON(), `"id":"bc|`+r.ID+`|1"`) {
		t.Fatalf("interactivo: %+v", im)
	}
	resp = &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chat, Sender: chat}, ID: "T3"},
		Message: &waE2E.Message{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{
			InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{
				Name: proto.String("quick_reply"), ParamsJSON: proto.String(`{"id":"bc|` + r.ID + `|2","display_text":"x"}`)}}}}}
	if x = e.s.Recibir(c, resp, res); x == nil || x.Num != 2 || !x.Navega {
		t.Fatalf("quick_reply: %+v", x)
	}
	// un botón que no es nuestro no se toca
	ajeno := &events.Message{Info: resp.Info, Message: &waE2E.Message{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{SelectedButtonID: proto.String("otro")}}}
	if e.s.Recibir(c, ajeno, res) != nil {
		t.Fatal("un botón ajeno se deja pasar")
	}
}

func TestBotonDeMenuReemplazadoNoNavega(t *testing.T) {
	e := nuevoEntorno(t)
	e.enciende(ModoBotones)
	c := context.Background()
	chat, _ := types.ParseJID(jidVotante)
	r1, _, _ := e.s.EnviarMenu(c, jidVotante, jidVotante, menu(3), "", "", "")
	e.s.EnviarMenu(c, jidVotante, jidVotante, menu(3), "", "", "")
	resp := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chat, Sender: chat}, ID: "T1"},
		Message: &waE2E.Message{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{SelectedButtonID: proto.String("bc|" + r1.ID + "|1")}}}
	if x := e.s.Recibir(c, resp, resolverFijo(jidVotante, jidVotante)); x == nil || x.Accion != AccionVieja || x.Navega {
		t.Fatalf("%+v", x)
	}
}

// ---------- pruebas de la matriz ----------

func TestPruebasRespetanAvisoDestinoYTopes(t *testing.T) {
	e := nuevoEntorno(t)
	c := context.Background()
	if _, err := e.s.EnviarPrueba(c, "encuesta", 4); err != ErrApagado {
		t.Fatalf("apagado: %v", err)
	}
	e.fija("lab_activo", "1")
	if _, err := e.s.EnviarPrueba(c, "encuesta", 4); err != ErrSinAviso {
		t.Fatalf("sin aviso: %v", err)
	}
	e.fija("lab_aviso_ts", "1700000000")
	if _, err := e.s.EnviarPrueba(c, "encuesta", 4); err != ErrSinDestino {
		t.Fatalf("sin destino: %v", err)
	}
	e.fija("lab_prueba_chat", jidVotante)
	if _, err := e.s.EnviarPrueba(c, "botones", 9999); err != ErrTipo {
		t.Fatalf("tipo: %v", err)
	}
	if _, err := e.s.EnviarPrueba(c, "encuesta", 1); err != ErrTipo {
		t.Fatalf("una opción: %v", err)
	}
	e.fija("lab_prueba_hora", "2")
	e.fija("lab_version_wa", "2.26.5.1")
	r, err := e.s.EnviarPrueba(c, "encuesta", 4)
	if err != nil || r.Enviado != 1 || r.Variante != "4" || r.VersionWA != "2.26.5.1" || r.VersionCliente != "2.3000.test" || r.MsgID == "" {
		t.Fatalf("prueba: %+v %v", r, err)
	}
	if len(e.em.enviados[0].GetPollCreationMessage().GetOptions()) != 4 {
		t.Fatal("4 opciones")
	}
	if _, err := e.s.EnviarPrueba(c, "botones", 3); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.EnviarPrueba(c, "lista", 10); err != ErrTope {
		t.Fatalf("tope por hora: %v", err)
	}
	e.ahora = e.ahora.Add(61 * time.Minute)
	e.fija("lab_prueba_dia", "2")
	if _, err := e.s.EnviarPrueba(c, "lista", 10); err != ErrTope {
		t.Fatalf("tope por día: %v", err)
	}
	// un envío que falla se anota con su error
	e.fija("lab_prueba_dia", "50")
	e.em.falla = errors.New("no soportado")
	r, err = e.s.EnviarPrueba(c, "interactivo", 3)
	if err == nil || r.Enviado != 0 || r.Error != "no soportado" {
		t.Fatalf("fallida: %+v %v", r, err)
	}
	if e.alm.ContarSesionesLab(1) != 2 {
		t.Fatalf("la sesión del envío fallido no debe quedar: %d", e.alm.ContarSesionesLab(1))
	}
}

func TestVotoEnPruebaMarcaRespondioYNoNavega(t *testing.T) {
	e := nuevoEntorno(t)
	e.enciende(ModoTexto)
	e.fija("lab_prueba_chat", jidVotante)
	c := context.Background()
	r, err := e.s.EnviarPrueba(c, "encuesta", 3)
	if err != nil {
		t.Fatal(err)
	}
	chat, _ := types.ParseJID(jidVotante)
	x := e.s.Recibir(c, e.voto(r.MsgID, []string{"Opción 2"}, chat, chat), resolverFijo(jidVotante, jidVotante))
	if x == nil || x.Accion != AccionVoto || x.Navega || !x.Prueba {
		t.Fatalf("voto de prueba: %+v", x)
	}
	if l := e.alm.ListarResultadosLab(1, 5); l[0].Respondio != 1 {
		t.Fatalf("respondió: %+v", l[0])
	}
	// un toque en una prueba de botones
	rb, _ := e.s.EnviarPrueba(c, "botones", 2)
	resp := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chat, Sender: chat}, ID: "T"},
		Message: &waE2E.Message{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{SelectedButtonID: proto.String("bc|" + rb.MsgID + "|1")}}}
	if x = e.s.Recibir(c, resp, resolverFijo(jidVotante, jidVotante)); x == nil || x.Navega || !x.Prueba {
		t.Fatalf("toque de prueba: %+v", x)
	}
	if l := e.alm.ListarResultadosLab(1, 5); l[0].Respondio != 1 {
		t.Fatalf("respondió botón: %+v", l[0])
	}
}

func TestPlanYLimiteMedido(t *testing.T) {
	if len(Plan()) < 10 {
		t.Fatal("el plan trae las pruebas")
	}
	rs := []almacen.ResultadoLab{
		{Tipo: "encuesta", Variante: "8", Visible: 1, Tocable: 1},
		{Tipo: "encuesta", Variante: "12", Visible: 1, Tocable: -1},
		{Tipo: "encuesta", Variante: "13", Visible: 0, Tocable: -1},
		{Tipo: "encuesta", Variante: "16", Visible: 1, Tocable: 0}, // se ve pero no se toca: no cuenta
		{Tipo: "botones", Variante: "3", Visible: 1, Tocable: 1},
	}
	if LimiteMedido(rs) != 12 {
		t.Fatalf("límite %d", LimiteMedido(rs))
	}
	if LimiteMedido(nil) != 0 {
		t.Fatal("sin datos")
	}
}
