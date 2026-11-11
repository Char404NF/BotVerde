package wabot

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/envio"
	"botchat/internal/flujo"
	"botchat/internal/motor"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func TestApiAjustesDeMenusValidados(t *testing.T) {
	abreSolo(t)
	aj := decodifica[map[string]string](t, func() (string, error) { return AjustesDeMenus(1) })
	if aj["menu_err_max"] != "3" || aj["grupo_max_sesiones"] != "5" || aj["grupo_expira_min"] != "10" || aj["menu_volver"] != "reiniciar" || aj["grupo_cualquiera"] != "0" || len(aj) != len(clavesAjustesMenus) {
		t.Fatalf("%v", aj)
	}
	for _, ok := range [][2]string{{"menu_err_max", "5"}, {"menu_volver", "retomar"}, {"grupo_cualquiera", "1"}, {"menu_region", "us"}, {"menu_txt_n1", "Perdón, no entendí."},
		{"menu_horario", `{"dias":[1,2,3],"desde":"09:00","hasta":"18:00"}`}, {"menu_horario", ""}, {"noentendio_dias", "0"}, {"menu_saludos", "hola, ey"}} {
		if err := FijarAjusteDeMenus(1, ok[0], ok[1]); err != nil {
			t.Fatalf("%v debía valer: %v", ok, err)
		}
	}
	aj = decodifica[map[string]string](t, func() (string, error) { return AjustesDeMenus(1) })
	if aj["menu_err_max"] != "5" || aj["menu_region"] != "US" || aj["menu_volver"] != "retomar" {
		t.Fatalf("%v", aj)
	}
	for _, mal := range [][2]string{{"menu_err_max", "0"}, {"menu_err_max", "11"}, {"menu_err_max", "x"}, {"menu_volver", "nunca"}, {"grupo_cualquiera", "2"}, {"menu_region", "Mexico"},
		{"menu_horario", "no es json"}, {"menu_horario", `{"dias":[9]}`}, {"menu_horario", `{"desde":"25:99"}`}, {"grupo_max_sesiones", "0"}, {"grupo_expira_min", "99999"},
		{"menu_txt_n1", strings.Repeat("a", 700)}, {"prefijo", "!"}, {"inexistente", "1"}} {
		if err := FijarAjusteDeMenus(1, mal[0], mal[1]); err == nil {
			t.Fatalf("%v no debía valer", mal)
		}
	}
}

func TestApiFormulariosNoEntendioYExportacion(t *testing.T) {
	_, a := abreSolo(t)
	const ana = "5215511111111@s.whatsapp.net"
	d := &flujo.Def{V: 1, Nombre: "Citas", Pasos: []flujo.Paso{{ID: "a", Tipo: flujo.PasoRespuesta, Texto: "x"}}, Disparadores: flujo.Disparadores{Cualquiera: true}, Alcance: flujo.Alcance{Privados: true}}
	d.Normalizar()
	mid, _ := a.CrearMenu(1, d.JSON(), false, false)
	for i, v := range []string{"Ana López", "=HYPERLINK(\"http://x\")"} {
		a.GuardarFormulario(1, almacen.Formulario{MenuID: mid, Paso: "f", Titulo: "Citas", Chat: ana, Persona: ana, TS: time.Now().Unix() + int64(i), Nombre: "Ana",
			Campos: []almacen.CampoValor{{ID: "n", Etiqueta: "Nombre", Valor: v}, {ID: "c", Etiqueta: "Correo", Valor: "a@b.co"}}})
	}
	l := decodifica[[]formularioVista](t, func() (string, error) { return ListarFormularios(1, 0, 10) })
	if len(l) != 2 || l[0].Menu != "Citas" || l[0].Cliente != "Ana" || len(l[0].Campos) != 2 {
		t.Fatalf("%+v", l)
	}
	if n, _ := ContarFormularios(1, mid); n != 2 {
		t.Fatal(n)
	}
	f := decodifica[formularioVista](t, func() (string, error) { return ObtenerFormulario(1, l[1].ID) })
	if f.Campos[0].Valor != "Ana López" {
		t.Fatalf("%+v", f)
	}
	if _, err := ObtenerFormulario(1, 999); err == nil {
		t.Fatal("inexistente")
	}
	rel, err := ExportarFormularios(1, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := a.LeerCifrado(rel)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatal("el CSV lleva marca para que Excel lea los acentos")
	}
	filas, err := csv.NewReader(bytes.NewReader(b[3:])).ReadAll()
	if err != nil || len(filas) != 5 || filas[0][0] != "Fecha" || filas[0][4] != "Respuesta" {
		t.Fatalf("%v %v", err, filas)
	}
	for _, fila := range filas[1:] {
		if strings.HasPrefix(fila[4], "=") {
			t.Fatalf("una celda que empieza con = se tomaría por fórmula: %q", fila[4])
		}
	}
	if abs, _ := a.Abs(rel); !strings.HasSuffix(abs, ".bcf") {
		t.Fatal(abs)
	}
	if err := BorrarFormulario(1, l[0].ID); err != nil {
		t.Fatal(err)
	}
	if n, _ := BorrarFormularios(1, 0); n != 1 {
		t.Fatalf("%d", n)
	}
	if _, err := ExportarFormularios(1, 0); err == nil {
		t.Fatal("sin formularios no hay nada que exportar")
	}
	// lo que no entendió
	a.GuardarNoEntendio(1, mid, "a", "quiero una pizza", time.Now())
	ne := decodifica[[]struct {
		Texto string `json:"texto"`
		Menu  string `json:"menu"`
	}](t, func() (string, error) { return ListarNoEntendio(1, 0, 10) })
	if len(ne) != 1 || ne[0].Texto != "quiero una pizza" || ne[0].Menu != "Citas" {
		t.Fatalf("%+v", ne)
	}
	if n, _ := BorrarNoEntendio(1, 0); n != 1 {
		t.Fatal(n)
	}
	// el registro de acciones no lleva textos
	acc, _ := a.ListarAcciones(1, 20, 0)
	for _, x := range acc {
		if strings.Contains(x.Detalle+x.Objetivo, "pizza") || strings.Contains(x.Detalle, "López") {
			t.Fatalf("el registro lleva texto: %+v", x)
		}
	}
}

func TestApiAdjuntosYExportarImportarMenus(t *testing.T) {
	_, a := abreSolo(t)
	tmp := filepath.Join(t.TempDir(), "foto.jpg")
	os.WriteFile(tmp, []byte("FOTO-DE-PRUEBA"), 0o600)
	adj := decodifica[map[string]any](t, func() (string, error) { return SubirAdjuntoMenu(1, tmp, "imagen", "image/jpeg", "foto.jpg") })
	id, _ := adj["id"].(string)
	if id == "" || adj["bytes"] != 14.0 {
		t.Fatalf("%v", adj)
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatal("la copia en claro se borra")
	}
	if b, err := a.LeerAdjuntoMenu(1, id); err != nil || string(b) != "FOTO-DE-PRUEBA" {
		t.Fatalf("%q %v", b, err)
	}
	for _, tipo := range []string{"sticker", "exe", ""} {
		os.WriteFile(tmp, []byte("x"), 0o600)
		if _, err := SubirAdjuntoMenu(1, tmp, tipo, "", ""); err == nil {
			t.Fatalf("tipo %q", tipo)
		}
	}
	grande := filepath.Join(t.TempDir(), "g.bin")
	fh, _ := os.Create(grande)
	fh.Truncate(9 << 20)
	fh.Close()
	if _, err := SubirAdjuntoMenu(1, grande, "documento", "", "g.bin"); err == nil || !strings.Contains(err.Error(), "8 MB") {
		t.Fatalf("%v", err)
	}
	if err := BorrarAdjuntoMenu(1, "../x"); err == nil {
		t.Fatal("ruta no válida")
	}
	// un menú con ese adjunto, exportado con contraseña e importado de vuelta (en el mismo teléfono, como copia)
	d := &flujo.Def{V: 1, Nombre: "Catálogo", Inicio: "m", Pasos: []flujo.Paso{{ID: "m", Tipo: flujo.PasoRespuesta, Texto: "Mira",
		Adjuntos: []flujo.AdjuntoMenu{{ID: id, Tipo: "imagen", Mime: "image/jpeg", Nombre: "foto.jpg"}},
		Ubicacion: &flujo.Ubicacion{Lat: 19.4, Lon: -99.1, Nombre: "Tienda"}, Contacto: &flujo.Contacto{Nombre: "Ventas", Telefono: "+525512345678"}}},
		Disparadores: flujo.Disparadores{Comando: ".catalogo"}, Alcance: flujo.Alcance{Privados: true}}
	d.Normalizar()
	for _, av := range d.Validar() {
		if av.Grave {
			t.Fatalf("%+v", av)
		}
	}
	mid, _ := a.CrearMenu(1, d.JSON(), true, false)
	rel, err := ExportarMenus(1, `[`+itoa(mid)+`]`, "clave-larga")
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := a.Abs(rel)
	datos, err := os.ReadFile(abs)
	if err != nil || bytes.Contains(datos, []byte("Catálogo")) {
		t.Fatal("el archivo exportado va cifrado")
	}
	copia := filepath.Join(t.TempDir(), "entrada.bcm")
	os.WriteFile(copia, datos, 0o600)
	if _, err := ImportarMenus(1, copia, "mala-clave"); err == nil || !strings.Contains(err.Error(), "contraseña") {
		t.Fatalf("%v", err)
	}
	if _, err := os.Stat(copia); !os.IsNotExist(err) {
		t.Fatal("la copia de entrada se borra siempre")
	}
	os.WriteFile(copia, datos, 0o600)
	res := decodifica[flujo.ResultadoImport](t, func() (string, error) { return ImportarMenus(1, copia, "clave-larga") })
	if len(res.IDs) != 1 || res.Nombres[0] != "Catálogo" {
		t.Fatalf("%+v", res)
	}
	m, _ := a.ObtenerMenu(1, res.IDs[0])
	if m.Activo {
		t.Fatal("importado = apagado")
	}
	di, _ := flujo.Parse(m.Def)
	if di.Paso("m").Ubicacion == nil || di.Paso("m").Contacto == nil || di.Paso("m").Adjuntos[0].ID == id {
		t.Fatalf("%+v", di.Paso("m"))
	}
	if _, err := ExportarMenus(1, `no`, ""); err == nil {
		t.Fatal("ids mal")
	}
	garbage := filepath.Join(t.TempDir(), "x.bcm")
	os.WriteFile(garbage, []byte("hola"), 0o600)
	if _, err := ImportarMenus(1, garbage, ""); err == nil {
		t.Fatal("archivo ajeno")
	}
}

// ---------- el envío de lo que sale de un menú ----------

type clienteFalso struct {
	mu       sync.Mutex
	n        int
	enviados []*waE2E.Message
	ids      []string
	subidas  [][]byte
}

func (f *clienteFalso) GenerateMessageID() types.MessageID {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	return types.MessageID("X" + string(rune('A'+f.n)))
}
func (f *clienteFalso) SendMessage(_ context.Context, _ types.JID, m *waE2E.Message, extra ...whatsmeow.SendRequestExtra) (whatsmeow.SendResponse, error) {
	f.mu.Lock()
	f.enviados = append(f.enviados, m)
	f.ids = append(f.ids, string(extra[0].ID))
	f.mu.Unlock()
	return whatsmeow.SendResponse{ID: extra[0].ID, Timestamp: time.Now()}, nil
}
func (f *clienteFalso) BuildReaction(chat, sender types.JID, id types.MessageID, r string) *waE2E.Message {
	return nil
}
func (f *clienteFalso) BuildEdit(chat types.JID, id types.MessageID, c *waE2E.Message) *waE2E.Message {
	return nil
}
func (f *clienteFalso) BuildRevoke(chat, sender types.JID, id types.MessageID) *waE2E.Message { return nil }
func (f *clienteFalso) MarkRead(context.Context, []types.MessageID, time.Time, types.JID, types.JID, ...types.ReceiptType) error {
	return nil
}
func (f *clienteFalso) SendChatPresence(context.Context, types.JID, types.ChatPresence, types.ChatPresenceMedia) error {
	return nil
}
func (f *clienteFalso) SendPresence(context.Context, types.Presence) error       { return nil }
func (f *clienteFalso) SubscribePresence(context.Context, types.JID) error        { return nil }
func (f *clienteFalso) GetGroupInfo(context.Context, types.JID) (*types.GroupInfo, error) {
	return nil, errors.New("sin grupo")
}
func (f *clienteFalso) UploadReader(_ context.Context, r io.Reader, _ io.ReadWriteSeeker, _ whatsmeow.MediaType) (whatsmeow.UploadResponse, error) {
	b, _ := io.ReadAll(r)
	f.mu.Lock()
	f.subidas = append(f.subidas, b)
	f.mu.Unlock()
	return whatsmeow.UploadResponse{URL: "https://x", DirectPath: "/x", MediaKey: []byte("k"), FileEncSHA256: []byte("e"), FileSHA256: []byte("s"), FileLength: uint64(len(b))}, nil
}
func (f *clienteFalso) ConfirmacionesLectura(context.Context) bool { return false }
func (f *clienteFalso) Conectado() bool                            { return true }
func (f *clienteFalso) Propio() (types.JID, types.JID) {
	return types.NewJID("5215500000000", types.DefaultUserServer), types.EmptyJID
}

func TestEnviadorMandaTextoArchivoUbicacionYContactoDeUnMenu(t *testing.T) {
	_, a := abreSolo(t)
	const grupo = "120363000000000001@g.us"
	_, _ = a.ActualizarChat(1, grupo, true, func(c *almacen.Chat) { c.Guardado = "Grupo" })
	cli := &clienteFalso{}
	s := &estado{botID: 1, l: &falso{}}
	s.envio = envio.Nuevo(1, a, cli, nil)
	e := enviadorMotor{s}
	ctx := context.Background()
	id, _, err := a.GuardarAdjuntoMenu(1, strings.NewReader("PDF-DEL-CATALOGO"))
	if err != nil {
		t.Fatal(err)
	}
	const ben = "9988776655@lid"
	a.GuardarMensaje(&almacen.Mensaje{ID: "IN1", ChatJID: grupo, BotID: 1, RemitenteJID: "5215533333333@s.whatsapp.net", Tipo: almacen.TipoTexto, Texto: "!menu", TS: time.Now().Unix()})
	// texto con mención y cita
	r1, err := e.EnviarSalida(ctx, grupo, motor.Salida{Texto: "@9988776655 hola"}, "IN1", ben, ben)
	if err != nil || r1 == "" {
		t.Fatal(r1, err)
	}
	ci := cli.enviados[0].GetExtendedTextMessage().GetContextInfo()
	if ci.GetParticipant() != ben || ci.GetMentionedJID()[0] != ben || ci.GetStanzaID() != "IN1" {
		t.Fatalf("%+v", ci)
	}
	// documento con pie
	r2, err := e.EnviarSalida(ctx, grupo, motor.Salida{Texto: "@9988776655 el catálogo", Medio: &motor.Medio{ID: id, Tipo: "documento", Mime: "application/pdf", Nombre: "catalogo.pdf"}}, "IN1", ben, ben)
	if err != nil || r2 == "" {
		t.Fatal(r2, err)
	}
	doc := cli.enviados[1].GetDocumentMessage()
	if doc == nil || doc.GetFileName() != "catalogo.pdf" || doc.GetCaption() != "@9988776655 el catálogo" || string(cli.subidas[0]) != "PDF-DEL-CATALOGO" || doc.GetContextInfo().GetParticipant() != ben {
		t.Fatalf("%+v", doc)
	}
	// no quedan copias en claro
	if es, _ := os.ReadDir(filepath.Join(a.Raiz(), "tmp")); len(es) != 0 {
		t.Fatalf("copias en claro sin borrar: %d", len(es))
	}
	// ubicación y contacto
	if _, err := e.EnviarSalida(ctx, grupo, motor.Salida{Ubicacion: &motor.Ubicacion{Lat: 19.4, Lon: -99.1, Nombre: "Tienda"}}, "IN1", ben, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := e.EnviarSalida(ctx, grupo, motor.Salida{Contacto: &motor.Contacto{Nombre: "Ventas", Telefono: "55 1234 5678"}}, "IN1", ben, ""); err != nil {
		t.Fatal(err)
	}
	if cli.enviados[2].GetLocationMessage().GetName() != "Tienda" || cli.enviados[3].GetContactMessage().GetDisplayName() != "Ventas" {
		t.Fatal("ubicación y contacto")
	}
	// si el archivo ya no está, se manda solo el texto (no se pierde la respuesta)
	r5, err := e.EnviarSalida(ctx, grupo, motor.Salida{Texto: "mira", Medio: &motor.Medio{ID: "noexiste", Tipo: "imagen"}}, "IN1", ben, "")
	if err != nil || r5 == "" || cli.enviados[4].GetExtendedTextMessage().GetText() != "mira" {
		t.Fatalf("%v %v", err, cli.enviados[4])
	}
	if _, err := e.EnviarSalida(ctx, grupo, motor.Salida{Medio: &motor.Medio{ID: "noexiste", Tipo: "imagen"}}, "IN1", ben, ""); err == nil {
		t.Fatal("sin texto y sin archivo es un error")
	}
}

// De punta a punta: un mensaje de WhatsApp en un grupo (con LID) pasa por la ingesta, el motor y el menú, y la respuesta sale
// mencionando y citando con el LID tal como lo mandó WhatsApp; el mensaje del bot queda ligado a la conversación.
func TestGrupoConLIDDePuntaAPuntaConMenu(t *testing.T) {
	s := arrancaBot(t)
	cli := &clienteFalso{}
	s.envio = envio.Nuevo(1, alm, cli, nil)
	d := &flujo.Def{V: 1, Nombre: "Tienda", Inicio: "m", Pasos: []flujo.Paso{
		{ID: "m", Tipo: flujo.PasoMenu, Titulo: "Hola {nombre}", Opciones: []flujo.Opcion{{Texto: "Horario", Destino: "h"}, {Texto: "Contacto", Destino: "c"}}},
		{ID: "h", Tipo: flujo.PasoRespuesta, Texto: "Abrimos de 9 a 6."}, {ID: "c", Tipo: flujo.PasoRespuesta, Texto: "Llámanos."}},
		Disparadores: flujo.Disparadores{Mencion: true}, Alcance: flujo.Alcance{TodosGrupos: true}}
	d.Normalizar()
	if _, err := alm.CrearMenu(1, d.JSON(), true, false); err != nil {
		t.Fatal(err)
	}
	grupo, _ := types.ParseJID("120363000000000001@g.us")
	lid := types.NewJID("9988776655", types.HiddenUserServer)
	pn := types.NewJID("5215533333333", types.DefaultUserServer)
	yo := *s.cli.Store.ID
	msg := func(id string, m *waE2E.Message) *events.Message {
		e := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: grupo, Sender: lid, SenderAlt: pn, IsGroup: true}, ID: id, Timestamp: time.Now(), PushName: "Ben"}, RawMessage: m}
		e.UnwrapRaw()
		return e
	}
	s.evento(msg("W1", &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("@5215500000000 menú"),
		ContextInfo: &waE2E.ContextInfo{MentionedJID: []string{yo.String()}}}}))
	esperaHasta(t, "no salió el menú", func() bool { cli.mu.Lock(); defer cli.mu.Unlock(); return len(cli.enviados) >= 1 })
	cli.mu.Lock()
	m1 := cli.enviados[0].GetExtendedTextMessage()
	cli.mu.Unlock()
	ci := m1.GetContextInfo()
	if !strings.HasPrefix(m1.GetText(), "@9988776655 Hola") || ci.GetStanzaID() != "W1" || ci.GetParticipant() != lid.String() || len(ci.GetMentionedJID()) != 1 || ci.GetMentionedJID()[0] != lid.String() {
		t.Fatalf("cita y mención con el LID tal cual: %q %+v", m1.GetText(), ci)
	}
	// el mensaje del bot quedó ligado a la conversación de Ben (su número canónico)
	var ref *almacen.MsgBot
	esperaHasta(t, "no se ligó el mensaje del bot", func() bool {
		cli.mu.Lock()
		defer cli.mu.Unlock()
		ref = alm.MsgBotPorID(1, grupo.String(), cli.ids[0])
		return ref != nil
	})
	if ref.Persona != pn.String() {
		t.Fatalf("dueña canónica: %+v", ref)
	}
	// Ben escribe su opción, esta vez con su número y sin mencionar: se le responde
	e2 := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: grupo, Sender: pn, IsGroup: true}, ID: "W2", Timestamp: time.Now(), PushName: "Ben"},
		RawMessage: &waE2E.Message{Conversation: proto.String("1")}}
	e2.UnwrapRaw()
	s.evento(e2)
	esperaHasta(t, "no salió la respuesta", func() bool { cli.mu.Lock(); defer cli.mu.Unlock(); return len(cli.enviados) >= 2 })
	cli.mu.Lock()
	m2 := cli.enviados[1].GetExtendedTextMessage()
	cli.mu.Unlock()
	if !strings.Contains(m2.GetText(), "Abrimos de 9 a 6.") || m2.GetContextInfo().GetStanzaID() != "W2" || m2.GetContextInfo().GetParticipant() != pn.String() {
		t.Fatalf("%q %+v", m2.GetText(), m2.GetContextInfo())
	}
	// charla suelta de otra persona: el bot no dice nada
	e3 := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: grupo, Sender: types.NewJID("5215544444444", types.DefaultUserServer), IsGroup: true}, ID: "W3", Timestamp: time.Now(), PushName: "Clara"},
		RawMessage: &waE2E.Message{Conversation: proto.String("1")}}
	e3.UnwrapRaw()
	s.evento(e3)
	time.Sleep(500 * time.Millisecond)
	cli.mu.Lock()
	n := len(cli.enviados)
	cli.mu.Unlock()
	if n != 2 {
		t.Fatalf("sin mención ni conversación, el bot calla: %d", n)
	}
}

func TestSimuladorYEstadisticasConLosPasosNuevos(t *testing.T) {
	_, a := abreSolo(t)
	d := &flujo.Def{V: 1, Nombre: "Nuevo", Inicio: "m", Pasos: []flujo.Paso{
		{ID: "m", Tipo: flujo.PasoMenu, Titulo: "Hola", Opciones: []flujo.Opcion{{Texto: "Dónde estamos", Destino: "u"}, {Texto: "Registrarme", Destino: "f"}}},
		{ID: "u", Tipo: flujo.PasoRespuesta, Texto: "Aquí estamos.", Ubicacion: &flujo.Ubicacion{Lat: 19.4, Lon: -99.1, Nombre: "Tienda"}, Contacto: &flujo.Contacto{Nombre: "Ventas", Telefono: "+525512345678"}},
		{ID: "f", Tipo: flujo.PasoForm, Titulo: "Registro", Campos: []flujo.Campo{{ID: "edad", Pregunta: "¿Edad?", Tipo: flujo.CampoNumero}}}},
		Disparadores: flujo.Disparadores{Comando: ".menu"}, Alcance: flujo.Alcance{Privados: true}}
	d.Normalizar()
	guardado := decodifica[struct {
		ID int64 `json:"id"`
	}](t, func() (string, error) { return GuardarAutomatizacion(1, 0, string(d.JSON())) })
	sim := func(texto string, reiniciar bool) flujo.Prueba {
		return decodifica[flujo.Prueba](t, func() (string, error) { return SimularAutomatizacion(1, guardado.ID, texto, reiniciar) })
	}
	sim("", true)
	if p := sim(".menu", false); !strings.HasPrefix(strings.Join(p.Salidas, ""), "Hola") {
		t.Fatalf("%+v", p)
	}
	p := sim("estamos", false) // texto parcial
	if len(p.Salidas) != 3 || p.Salidas[0] != "Aquí estamos." || p.Salidas[1] != "[Ubicación: Tienda]" || p.Salidas[2] != "[Contacto: Ventas · +525512345678]" {
		t.Fatalf("el simulador describe lo que mandaría: %+v", p.Salidas)
	}
	sim(".menu", true)
	sim(".menu", false)
	sim("registrarme", false)
	if p := sim("abc", false); !strings.HasPrefix(p.Salidas[0], "Eso no parece un número") {
		t.Fatalf("%+v", p.Salidas)
	}
	sim("33", false)
	p = sim("1", false)
	if p.Estado != almacen.SesCompletada {
		t.Fatalf("%+v", p)
	}
	if a.ContarFormularios(1, 0) != 0 {
		t.Fatal("el simulador no guarda formularios")
	}
	e := decodifica[map[string]any](t, func() (string, error) { return EstadisticasAutomatizacion(1, guardado.ID, "2000-01-01", "2100-01-01", true) })
	if e["formulariosEnviados"] != 1.0 || e["noEntendio"] != 0.0 || e["formularios"] != 0.0 {
		t.Fatalf("%v", e)
	}
	if _, ok := e["abandonos"]; !ok {
		t.Fatalf("%v", e)
	}
}
