package lab

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
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
	nodos    [][]waBinary.Node
}

func (e *emisorFalso) EnviarInteractivoNodos(c context.Context, chat, resumen, citaID, citaAutorWA string, armar func(string, *waE2E.ContextInfo) *waE2E.Message, antes func(string, *waE2E.Message) error, nodos []waBinary.Node) (envio.Resultado, error) {
	e.mu.Lock()
	e.nodos = append(e.nodos, nodos)
	e.mu.Unlock()
	return e.EnviarInteractivo(c, chat, resumen, citaID, citaAutorWA, armar, antes)
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

func TestEnTextoNoMandaNadaYNoNecesitaLaboratorio(t *testing.T) {
	e := nuevoEntorno(t)
	for _, caso := range []func(){
		func() {}, // todo por defecto: texto
		func() { e.fija("lab_modo_menu", ModoTexto) },
	} {
		caso()
		_, ok, err := e.s.EnviarMenu(context.Background(), jidVotante, jidVotante, menu(3), "texto", "", "")
		if ok || err != nil || len(e.em.enviados) != 0 {
			t.Fatalf("debía salir como texto: ok=%v err=%v", ok, err)
		}
	}
	// ya no hace falta «encender el Laboratorio» ni aceptar un aviso: elegir el formato basta
	e.fija("lab_modo_menu", "encuesta")
	if _, ok, err := e.s.EnviarMenu(context.Background(), jidVotante, jidVotante, menu(3), "texto", "", ""); !ok || err != nil {
		t.Fatalf("la encuesta debía salir: ok=%v err=%v", ok, err)
	}
	e.fija("lab_modo_menu", ModoTexto)
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
	if !ok || err != nil || r.Tipo != almacen.LabBotones || r.Respaldo {
		t.Fatalf("botones: %+v %v %v", r, ok, err)
	}
	b := e.em.enviados[0].GetInteractiveMessage().GetNativeFlowMessage()
	if b == nil || len(b.GetButtons()) != 3 || b.GetButtons()[1].GetName() != "quick_reply" ||
		!strings.Contains(b.GetButtons()[1].GetButtonParamsJSON(), `"display_text":"Precios"`) || !strings.Contains(b.GetButtons()[1].GetButtonParamsJSON(), `"id":"bc|`+r.ID+`|2"`) {
		t.Fatalf("botones armados: %+v", b)
	}
	// van con el nodo <biz> (sin él WhatsApp los acepta pero no los muestra)
	if len(e.em.nodos) == 0 || len(e.em.nodos[len(e.em.nodos)-1]) != 1 || e.em.nodos[len(e.em.nodos)-1][0].Tag != "biz" {
		t.Fatalf("faltó el nodo biz: %+v", e.em.nodos)
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
	l := e.em.enviados[len(e.em.enviados)-1].GetInteractiveMessage().GetNativeFlowMessage()
	if l == nil || len(l.GetButtons()) != 1 || l.GetButtons()[0].GetName() != "single_select" || strings.Count(l.GetButtons()[0].GetButtonParamsJSON(), `"bc|`+r.ID+`|`) != 8 {
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

func TestNodoBizTieneLaEstructuraDelPR1235(t *testing.T) {
	ahora := time.Unix(1700000123, 0)
	n := NodoBiz(ahora, bytes.NewReader(bytes.Repeat([]byte{0xab}, 20)))
	if n.Tag != "biz" || n.Attrs["actual_actors"] != "2" || n.Attrs["host_storage"] != "2" || n.Attrs["privacy_mode_ts"] != "1700000123" {
		t.Fatalf("biz: %+v", n)
	}
	h := n.GetChildren()
	if len(h) != 2 || h[0].Tag != "interactive" || h[1].Tag != "quality_control" {
		t.Fatalf("hijos: %+v", h)
	}
	if h[0].Attrs["type"] != "native_flow" || h[0].Attrs["v"] != "1" {
		t.Fatalf("interactive: %+v", h[0].Attrs)
	}
	nf := h[0].GetChildren()
	if len(nf) != 1 || nf[0].Tag != "native_flow" || nf[0].Attrs["v"] != "9" || nf[0].Attrs["name"] != "mixed" {
		t.Fatalf("native_flow: %+v", nf)
	}
	if h[1].Attrs["source_type"] != "third_party" || h[1].Attrs["decision_id"] != strings.Repeat("ab", 20) {
		t.Fatalf("quality_control: %+v", h[1].Attrs)
	}
	ds := h[1].GetChildren()
	if len(ds) != 1 || ds[0].Tag != "decision_source" || ds[0].Attrs["value"] != "df" {
		t.Fatalf("decision_source: %+v", ds)
	}
	// el decision_id cambia en cada mensaje
	a, b := NodoBiz(ahora, nil), NodoBiz(ahora, nil)
	if a.GetChildren()[1].Attrs["decision_id"] == b.GetChildren()[1].Attrs["decision_id"] {
		t.Fatal("decision_id debe ser al azar")
	}
}

func TestPruebaBizSoloConInterruptorCasillaYTope(t *testing.T) {
	e := nuevoEntorno(t)
	c := context.Background()
	e.enciende("texto")
	e.fija("lab_prueba_chat", jidVotante)
	if len(PlanBiz()) != 2 {
		t.Fatal("plan biz: 1 y 3 botones")
	}
	for _, p := range Plan() {
		if p.Tipo == almacen.LabInteractivoBiz {
			t.Fatal("el plan normal no ofrece biz")
		}
	}
	if _, err := e.s.EnviarPrueba(c, almacen.LabInteractivoBiz, 1); err != ErrBizApagado {
		t.Fatalf("sin interruptor: %v", err)
	}
	e.fija("lab_biz_activo", "1")
	if _, err := e.s.EnviarPrueba(c, almacen.LabInteractivoBiz, 1); err != ErrBizSinAck {
		t.Fatalf("sin casilla: %v", err)
	}
	e.fija("lab_biz_activo", "0")
	e.fija("lab_biz_ack", "1")
	if _, err := e.s.EnviarPrueba(c, almacen.LabInteractivoBiz, 1); err != ErrBizApagado {
		t.Fatalf("casilla sin interruptor: %v", err)
	}
	if len(e.em.enviados) != 0 || len(e.em.nodos) != 0 {
		t.Fatal("no debe enviarse nada")
	}
	e.fija("lab_biz_activo", "1")
	if _, err := e.s.EnviarPrueba(c, almacen.LabInteractivoBiz, 2); err != ErrTipo {
		t.Fatalf("variante: %v", err)
	}
	e.fija("lab_prueba_hora", "1")
	r, err := e.s.EnviarPrueba(c, almacen.LabInteractivoBiz, 3)
	if err != nil || r.Enviado != 1 || r.Tipo != "interactivo_biz" {
		t.Fatalf("envío: %+v %v", r, err)
	}
	if len(e.em.nodos) != 1 || len(e.em.nodos[0]) != 1 || e.em.nodos[0][0].Tag != "biz" {
		t.Fatalf("nodos: %+v", e.em.nodos)
	}
	if im := e.em.enviados[0].GetInteractiveMessage(); im == nil || len(im.GetNativeFlowMessage().GetButtons()) != 3 {
		t.Fatal("3 botones quick_reply")
	}
	if _, err := e.s.EnviarPrueba(c, almacen.LabInteractivoBiz, 1); err != ErrTope {
		t.Fatalf("tope: %v", err)
	}
	// el menú real nunca usa biz, aunque el modo sea botones/interactivo
	e.fija("lab_botones_tipo", "interactivo")
	e.enciende("botones")
	e.s.Modo()
	n := len(e.em.nodos)
	_, _, _ = e.s.EnviarMenu(c, jidVotante, jidVotante, menu(3), "", "", "")
	if len(e.em.nodos) != n {
		t.Fatal("el menú real no añade nodos")
	}
}

// ---------- respuesta interactiva (InteractiveResponseMessage) con y sin ParamsJSON válido ----------

func respInteractiva(chat types.JID, citado, params, cuerpo string) *events.Message {
	ir := &waE2E.InteractiveResponseMessage{
		InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{
			Name: proto.String("quick_reply"), ParamsJSON: proto.String(params)}},
		ContextInfo: &waE2E.ContextInfo{StanzaID: proto.String(citado)},
	}
	if cuerpo != "" {
		ir.Body = &waE2E.InteractiveResponseMessage_Body{Text: proto.String(cuerpo)}
	}
	return &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chat, Sender: chat}, ID: "IR"},
		Message: &waE2E.Message{InteractiveResponseMessage: ir}}
}

func TestRespuestaInteractivaToleranteYDeMenuReal(t *testing.T) {
	e := nuevoEntorno(t)
	e.enciende(ModoBotones)
	e.fija("lab_botones_tipo", "interactivo")
	c := context.Background()
	res := resolverFijo(jidVotante, jidVotante)
	chat, _ := types.ParseJID(jidVotante)
	r, _, _ := e.s.EnviarMenu(c, jidVotante, jidVotante, menu(3), "", "", "")

	// con ParamsJSON válido (id nuestro)
	if x := e.s.Recibir(c, respInteractiva(chat, r.ID, `{"id":"bc|`+r.ID+`|3","display_text":"x"}`, ""), res); x == nil || x.Num != 3 || !x.Navega {
		t.Fatalf("id válido: %+v", x)
	}
	// JSON inválido pero con el cuerpo igual al título de la opción 2
	if x := e.s.Recibir(c, respInteractiva(chat, r.ID, `{no es json`, "Precios"), res); x == nil || x.Num != 2 || !x.Navega {
		t.Fatalf("json inválido + cuerpo: %+v", x)
	}
	// id numérico simple (como número JSON) y display_text
	if x := e.s.Recibir(c, respInteractiva(chat, r.ID, `{"id":1}`, ""), res); x == nil || x.Num != 1 || !x.Navega {
		t.Fatalf("id numérico: %+v", x)
	}
	if x := e.s.Recibir(c, respInteractiva(chat, r.ID, `{"display_text":"Precios"}`, ""), res); x == nil || x.Num != 2 {
		t.Fatalf("display_text: %+v", x)
	}
	// sin nada identificable: se anota, no navega
	if x := e.s.Recibir(c, respInteractiva(chat, r.ID, ``, ""), res); x == nil || x.Accion != AccionSinOpcion || x.Navega {
		t.Fatalf("sin datos: %+v", x)
	}
	// sesión ajena (no existe) o sin cita: no es nuestra
	if x := e.s.Recibir(c, respInteractiva(chat, "AJENA", `{"id":"1"}`, "Precios"), res); x != nil {
		t.Fatalf("ajena: %+v", x)
	}
	if x := e.s.Recibir(c, respInteractiva(chat, "", `{}`, "Precios"), res); x != nil {
		t.Fatalf("sin cita: %+v", x)
	}
	// caducada: se anota y no navega
	e.alm.MarcarSesionLab(1, r.ID, almacen.LabExpirada)
	if x := e.s.Recibir(c, respInteractiva(chat, r.ID, `{"id":"bc|`+r.ID+`|1"}`, ""), res); x == nil || x.Accion != AccionExpirada || x.Navega {
		t.Fatalf("caducada: %+v", x)
	}
}

func TestRespuestaInteractivaDePruebaMarcaRespondio(t *testing.T) {
	e := nuevoEntorno(t)
	e.enciende(ModoTexto)
	e.fija("lab_prueba_chat", jidVotante)
	e.fija("lab_biz_activo", "1")
	e.fija("lab_biz_ack", "1")
	c := context.Background()
	chat, _ := types.ParseJID(jidVotante)
	rb, err := e.s.EnviarPrueba(c, "interactivo_biz", 3)
	if err != nil {
		t.Fatal(err)
	}
	x := e.s.Recibir(c, respInteractiva(chat, rb.MsgID, `{"display_text":"Opción 2"}`, ""), resolverFijo(jidVotante, jidVotante))
	if x == nil || x.Navega || !x.Prueba {
		t.Fatalf("toque de prueba: %+v", x)
	}
	if l := e.alm.ListarResultadosLab(1, 5); l[0].Respondio != 1 {
		t.Fatalf("respondió: %+v", l[0])
	}
}

func TestTemplateButtonReplyYEnvolturasSeAtienden(t *testing.T) {
	e := nuevoEntorno(t)
	e.enciende(ModoBotones)
	e.fija("lab_botones_tipo", "interactivo")
	c := context.Background()
	res := resolverFijo(jidVotante, jidVotante)
	chat, _ := types.ParseJID(jidVotante)
	r, _, _ := e.s.EnviarMenu(c, jidVotante, jidVotante, menu(3), "", "", "")
	tpl := func(id, texto string) *waE2E.Message {
		return &waE2E.Message{TemplateButtonReplyMessage: &waE2E.TemplateButtonReplyMessage{SelectedID: proto.String(id), SelectedDisplayText: proto.String(texto),
			ContextInfo: &waE2E.ContextInfo{StanzaID: proto.String(r.ID)}}}
	}
	ev := func(m *waE2E.Message) *events.Message {
		return &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chat, Sender: chat}, ID: "TT"}, Message: m}
	}
	if !EsRespuesta(tpl("x", "y")) {
		t.Fatal("TemplateButtonReply es una respuesta")
	}
	if x := e.s.Recibir(c, ev(tpl("bc|"+r.ID+"|3", "z")), res); x == nil || x.Num != 3 || !x.Navega {
		t.Fatalf("id nuestro: %+v", x)
	}
	// sin id nuestro: por número simple o por el título
	if x := e.s.Recibir(c, ev(tpl("1", "")), res); x == nil || x.Num != 1 || !x.Navega {
		t.Fatalf("número: %+v", x)
	}
	if x := e.s.Recibir(c, ev(tpl("", "Precios")), res); x == nil || x.Num != 2 {
		t.Fatalf("título: %+v", x)
	}
	// dentro de envolturas anidadas (efímero dentro de enviado desde otro dispositivo)
	env := &waE2E.Message{EphemeralMessage: &waE2E.FutureProofMessage{Message: tpl("bc|"+r.ID+"|2", "")}}
	env = &waE2E.Message{DeviceSentMessage: &waE2E.DeviceSentMessage{Message: env}}
	if x := e.s.Recibir(c, ev(env), res); x == nil || x.Num != 2 || !x.Navega {
		t.Fatalf("envuelto: %+v", x)
	}
}

// ---------- formato por menú, botones especiales, solo privado, tope y botones de módulos ----------

func TestFormatoPorMenuGanaAlGeneralYLosGruposNuncaLlevanBotones(t *testing.T) {
	e := nuevoEntorno(t)
	c := context.Background()
	e.fija("lab_modo_menu", ModoTexto)
	m := menu(3)
	m.Formato = "botones"
	if r, ok, err := e.s.EnviarMenu(c, jidVotante, jidVotante, m, "", "", ""); !ok || err != nil || r.Tipo != almacen.LabBotones {
		t.Fatalf("el menú pide botones aunque el general sea texto: %+v %v %v", r, ok, err)
	}
	m2 := menu(3)
	m2.Formato = "texto"
	e.fija("lab_modo_menu", ModoBotones)
	if _, ok, _ := e.s.EnviarMenu(c, jidVotante, jidVotante, m2, "", "", ""); ok {
		t.Fatal("el menú pide texto aunque el general sea botones")
	}
	// grupo: botones no, ni con lab_grupos
	e.fija("lab_grupos", "1")
	n := len(e.em.enviados)
	if _, ok, _ := e.s.EnviarMenu(c, "120363000000000001@g.us", jidVotante, menu(3), "", "", ""); ok || len(e.em.enviados) != n {
		t.Fatal("en un grupo los botones nunca salen")
	}
	// la encuesta sí, si se permitió en grupos
	e.fija("lab_modo_menu", ModoEncuesta)
	if r, ok, _ := e.s.EnviarMenu(c, "120363000000000001@g.us", jidVotante, menu(3), "", "", ""); !ok || r.Tipo != almacen.LabEncuesta {
		t.Fatal("la encuesta en grupo debía salir con lab_grupos")
	}
}

func TestBotonesEspecialesEnlaceCopiarLlamar(t *testing.T) {
	e := nuevoEntorno(t)
	c := context.Background()
	e.fija("lab_modo_menu", ModoBotones)
	m := menu(3)
	m.Opciones[0].BotonTipo, m.Opciones[0].BotonValor = "enlace", "https://ejemplo.com/catalogo"
	m.Opciones[1].BotonTipo, m.Opciones[1].BotonValor = "copiar", "CLABE 0123"
	m.Opciones[2].BotonTipo, m.Opciones[2].BotonValor = "llamar", "+52 55 1234 5678"
	r, ok, err := e.s.EnviarMenu(c, jidVotante, jidVotante, m, "", "", "")
	if !ok || err != nil || r.Tipo != almacen.LabBotones {
		t.Fatalf("%+v %v %v", r, ok, err)
	}
	bs := e.em.enviados[len(e.em.enviados)-1].GetInteractiveMessage().GetNativeFlowMessage().GetButtons()
	if len(bs) != 3 || bs[0].GetName() != "cta_url" || !strings.Contains(bs[0].GetButtonParamsJSON(), "https://ejemplo.com/catalogo") ||
		bs[1].GetName() != "cta_copy" || !strings.Contains(bs[1].GetButtonParamsJSON(), `"copy_code":"CLABE 0123"`) ||
		bs[2].GetName() != "cta_call" || !strings.Contains(bs[2].GetButtonParamsJSON(), `"phone_number":"+52 55 1234 5678"`) {
		t.Fatalf("botones especiales: %+v", bs)
	}
	// con botones especiales y más de 3 opciones no hay lista (no puede llevar enlaces): sale en texto
	m4 := menu(5)
	m4.Opciones[0].BotonTipo, m4.Opciones[0].BotonValor = "enlace", "https://ejemplo.com"
	if _, ok, _ := e.s.EnviarMenu(c, jidVotante, jidVotante, m4, "", "", ""); ok {
		t.Fatal("5 opciones con un enlace salen en texto")
	}
}

func TestTopePorHoraDeBotonesYBotonesDeModulos(t *testing.T) {
	e := nuevoEntorno(t)
	c := context.Background()
	e.fija("lab_modo_menu", ModoBotones)
	e.fija("menu_botones_hora", "2")
	for i := 0; i < 2; i++ {
		if _, ok, _ := e.s.EnviarMenu(c, jidVotante, jidVotante, menu(3), "", "", ""); !ok {
			t.Fatalf("el menú %d debía salir con botones", i+1)
		}
	}
	if _, ok, _ := e.s.EnviarMenu(c, jidVotante, jidVotante, menu(3), "", "", ""); ok {
		t.Fatal("pasado el tope por hora el menú sale en texto")
	}
	e.ahora = e.ahora.Add(61 * time.Minute)
	if _, ok, _ := e.s.EnviarMenu(c, jidVotante, jidVotante, menu(3), "", "", ""); !ok {
		t.Fatal("pasada la hora vuelven los botones")
	}
	// botones de un módulo: «Reclamar premio» → !reclamar
	e.fija("menu_botones_hora", "0")
	ok, err := e.s.EnviarBotones(c, jidVotante, jidVotante, "🎉 Ganaste", []BotonCmd{{Texto: "Reclamar premio", Comando: "!reclamar"}})
	if !ok || err != nil {
		t.Fatalf("%v %v", ok, err)
	}
	ses := e.alm.ObtenerSesionLab(1, "ID"+string(rune('A'+e.em.n)))
	if ses == nil || len(ses.Opciones) != 1 || ses.Opciones[0].Cmd != "!reclamar" {
		t.Fatalf("sesión: %+v", ses)
	}
	chat, _ := types.ParseJID(jidVotante)
	resp := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chat, Sender: chat}, ID: "TC"},
		Message: &waE2E.Message{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{
			InteractiveResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage_{NativeFlowResponseMessage: &waE2E.InteractiveResponseMessage_NativeFlowResponseMessage{
				Name: proto.String("quick_reply"), ParamsJSON: proto.String(`{"id":"` + ses.Opciones[0].ID + `","display_text":"Reclamar premio"}`)}},
			ContextInfo: &waE2E.ContextInfo{StanzaID: proto.String(ses.ID)}}}}
	x := e.s.Recibir(c, resp, resolverFijo(jidVotante, jidVotante))
	if x == nil || x.Comando != "!reclamar" || !x.Navega {
		t.Fatalf("toque de módulo: %+v", x)
	}
	// con el formato general en texto no hay botones de módulos
	e.fija("lab_modo_menu", ModoTexto)
	if ok, _ := e.s.EnviarBotones(c, jidVotante, jidVotante, "x", []BotonCmd{{"A", "!a"}}); ok {
		t.Fatal("sin el formato Botones no salen botones de módulos")
	}
}
