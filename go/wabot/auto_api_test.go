package wabot

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/auto"
	"botchat/internal/envio"
	"botchat/internal/flujo"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func oggMono(canales byte) []byte {
	var b bytes.Buffer
	b.WriteString("OggS")
	b.Write([]byte{0, 2})
	b.Write(make([]byte, 8+4+4+4))
	b.Write([]byte{1, 19})
	b.WriteString("OpusHead")
	b.Write([]byte{1, canales, 0x38, 0x01, 0x80, 0xBB, 0, 0, 0, 0, 0})
	b.WriteString("OggS")
	b.Write([]byte{0, 4})
	var g [8]byte
	binary.LittleEndian.PutUint64(g[:], 48000)
	b.Write(g[:])
	b.Write(make([]byte, 16))
	return b.Bytes()
}

func subeAdj(t *testing.T, tipo, mime, nombre string, datos []byte) almacen.AdjuntoRef {
	t.Helper()
	p := filepath.Join(t.TempDir(), "subida.bin")
	_ = os.WriteFile(p, datos, 0o600)
	r := decodifica[almacen.AdjuntoRef](t, func() (string, error) { return SubirAdjunto(1, p, tipo, mime, nombre) })
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("la copia en claro se borra siempre")
	}
	return r
}

func guarda(t *testing.T, e autoEntrada) int64 {
	t.Helper()
	b, _ := json.Marshal(e)
	id, err := GuardarAuto(1, string(b))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestApiMensajesAutomaticosDeExtremoAExtremo(t *testing.T) {
	_, a := abreSolo(t)
	// plantillas y configuración de fábrica
	ps := decodifica[[]auto.Plantilla](t, func() (string, error) { return PlantillasAuto() })
	if len(ps) < 8 {
		t.Fatal(len(ps))
	}
	cfg := decodifica[auto.Config](t, func() (string, error) { return ConfigPorDefectoAuto("ausencia") })
	if cfg.Cuando.HorarioModo != "fuera" || len(cfg.Cuando.Horario) != 5 {
		t.Fatalf("%+v", cfg)
	}
	foto := subeAdj(t, "imagen", "image/jpeg", "f.jpg", []byte("JPG"))
	id := guarda(t, autoEntrada{Tipo: "ausencia", Nombre: "Fuera de horario", Texto: "Hola {nombre}, atendemos {horario}", Config: cfg, Adjuntos: []almacen.AdjuntoRef{foto}, Activo: true})
	lista := decodifica[autoLista](t, func() (string, error) { return ListarAutos(1, "ausencia") })
	if len(lista.Mensajes) != 1 || lista.Mensajes[0].ID != id || !strings.Contains(lista.Mensajes[0].Cuando, "fuera de horario") || lista.Mensajes[0].Repite != "una vez cada 12 horas" ||
		lista.Mensajes[0].Estado != "activo" || !lista.Activo || lista.TopeHora != 30 {
		t.Fatalf("%+v", lista)
	}
	// el interruptor
	if err := FijarAutoActivo(1, id, false); err != nil {
		t.Fatal(err)
	}
	v := decodifica[autoVista](t, func() (string, error) { return ObtenerAuto(1, id) })
	if v.Activo || v.Estado != "apagado" {
		t.Fatalf("%+v", v)
	}
	// validación con mensajes claros
	for _, mal := range []autoEntrada{
		{Tipo: "recibido", Texto: "", Config: auto.ConfigPorDefecto("recibido")},
		{Tipo: "ausencia", Texto: "x", Config: auto.Config{Quien: auto.Quien{Personas: "todos", Privados: true}}},
		{Tipo: "atajo", Nombre: "con espacio", Texto: "x"},
		{Tipo: "recibido", Texto: "x", Config: auto.ConfigPorDefecto("recibido"), Adjuntos: []almacen.AdjuntoRef{{ID: strings.Repeat("9", 24), Tipo: "imagen"}}}, // archivo que no existe
	} {
		b, _ := json.Marshal(mal)
		if _, err := GuardarAuto(1, string(b)); err == nil {
			t.Fatalf("no debía valer: %+v", mal)
		}
	}
	// atajos: nombre único
	guarda(t, autoEntrada{Tipo: "atajo", Nombre: "/Gracias", Texto: "Gracias {nombre}", Activo: true})
	b, _ := json.Marshal(autoEntrada{Tipo: "atajo", Nombre: "gracias", Texto: "otro", Activo: true})
	if _, err := GuardarAuto(1, string(b)); err == nil || !strings.Contains(err.Error(), "Ya tienes un atajo") {
		t.Fatalf("%v", err)
	}
	// el selector de la app: texto ya con variables para el chat
	at := decodifica[[]atajoVista](t, func() (string, error) { return ListarAtajos(1, "5215511111111@s.whatsapp.net") })
	if len(at) != 1 || at[0].Nombre != "gracias" || strings.Contains(at[0].Texto, "{") {
		t.Fatalf("%+v", at)
	}
	// quitar el archivo al editar borra el archivo guardado
	v = decodifica[autoVista](t, func() (string, error) { return ObtenerAuto(1, id) })
	guarda(t, autoEntrada{ID: id, Tipo: "ausencia", Nombre: v.Nombre, Texto: v.Texto, Config: v.Config, Adjuntos: nil, Activo: true})
	if a.ExisteAdjunto(1, foto.ID) {
		t.Fatal("el archivo quitado del editor se borra")
	}
	// ProbarAuto: sin guardar y sin mandar
	ent := autoEntrada{Tipo: "ausencia", Texto: "Hola {nombre}, atendemos {horario}", Config: cfg}
	eb, _ := json.Marshal(ent)
	pr := decodifica[auto.Prueba](t, func() (string, error) {
		return ProbarAuto(1, string(eb), `{"nombre":"Marta","cuando":"2026-11-14T11:00"}`)
	})
	if !pr.Aplica || !strings.HasPrefix(pr.Texto, "Hola Marta, atendemos de lunes a viernes") {
		t.Fatalf("%+v", pr)
	}
	pr = decodifica[auto.Prueba](t, func() (string, error) {
		return ProbarAuto(1, string(eb), `{"nombre":"Marta","cuando":"2026-11-10T11:00"}`)
	})
	if pr.Aplica {
		t.Fatalf("en horario no se manda: %+v", pr)
	}
	if _, err := ProbarAuto(1, `{"tipo":"recibido","texto":""}`, ""); err == nil {
		t.Fatal("probar valida igual que guardar")
	}
	// borrar
	if err := BorrarAuto(1, id); err != nil {
		t.Fatal(err)
	}
	if _, err := ObtenerAuto(1, id); err == nil {
		t.Fatal("borrado")
	}
	// registro de acciones sin textos
	acc, _ := a.ListarAcciones(1, 50, 0)
	for _, x := range acc {
		if strings.Contains(x.Detalle, "Hola") || strings.Contains(x.Detalle, "Gracias") {
			t.Fatalf("el registro no lleva textos: %+v", x)
		}
	}
}

func TestApiAjustesDeMensajesAutomaticos(t *testing.T) {
	abreSolo(t)
	for _, ok := range [][2]string{{"auto_activo", "0"}, {"auto_tope_hora", "50"}, {"auto_zona", "America/Bogota"}, {"auto_zona", ""}, {"atajos_activo", "1"}, {"atajo_modo", "enviar"}} {
		if err := FijarAjusteDePermisos(1, ok[0], ok[1]); err != nil {
			t.Fatalf("%v: %v", ok, err)
		}
	}
	for _, mal := range [][2]string{{"auto_activo", "2"}, {"auto_tope_hora", "-1"}, {"auto_tope_hora", "x"}, {"auto_zona", "Marte/Olimpo"}, {"atajo_modo", "borrar"}} {
		if FijarAjusteDePermisos(1, mal[0], mal[1]) == nil {
			t.Fatalf("%v no debía valer", mal)
		}
	}
}

func TestApiAdjuntosNotaDeVozYComandosPropios(t *testing.T) {
	_, a := abreSolo(t)
	// la nota de voz se valida leyendo la cabecera
	n := subeAdj(t, "audio", "audio/ogg", "", oggMono(1))
	if n.Mime != "audio/ogg; codecs=opus" || !a.ExisteAdjunto(1, n.ID) {
		t.Fatalf("%+v", n)
	}
	p := filepath.Join(t.TempDir(), "e.ogg")
	_ = os.WriteFile(p, oggMono(2), 0o600)
	if _, err := SubirAdjunto(1, p, "audio", "", ""); err == nil || !strings.Contains(err.Error(), "mono") {
		t.Fatalf("estéreo: %v", err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("aunque falle, la copia en claro se borra")
	}
	// BorrarAdjunto no borra lo que algo usa
	guarda(t, autoEntrada{Tipo: "recibido", Texto: "escucha", Config: auto.ConfigPorDefecto("recibido"), Adjuntos: []almacen.AdjuntoRef{n}, Activo: true})
	if err := BorrarAdjunto(1, n.ID); err != nil || !a.ExisteAdjunto(1, n.ID) {
		t.Fatal("lo que usa un mensaje no se borra")
	}
	suelto := subeAdj(t, "documento", "application/pdf", "lista.pdf", []byte("%PDF"))
	if err := BorrarAdjunto(1, suelto.ID); err != nil || a.ExisteAdjunto(1, suelto.ID) {
		t.Fatal("el suelto sí")
	}
	if BorrarAdjunto(1, "../x") == nil {
		t.Fatal("id no válido")
	}
	// un comando propio lleva archivos con el mismo servicio (campo reservado de la etapa B, ya completo)
	doc := subeAdj(t, "documento", "application/pdf", "precios.pdf", []byte("%PDF-precios"))
	j := `{"nombre":"precios","texto":"Aquí van","privados":true,"adjuntos":[{"id":"` + doc.ID + `","tipo":"documento","mime":"application/pdf","nombre":"precios.pdf"}]}`
	cid, err := GuardarComandoPropio(1, 0, j)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := a.ObtenerMenu(1, cid)
	if !strings.Contains(string(m.Def), doc.ID) {
		t.Fatalf("el comando guarda su archivo: %s", m.Def)
	}
	if k, _ := LimpiarAdjuntos(1); k != 0 || !a.ExisteAdjunto(1, doc.ID) {
		t.Fatal("el archivo de un comando propio no es huérfano")
	}
	j4 := `{"nombre":"precios","texto":"Aquí van","privados":true,"adjuntos":[{"id":"a","tipo":"documento"},{"id":"b","tipo":"documento"},{"id":"c","tipo":"documento"},{"id":"d","tipo":"documento"}]}`
	if _, err := GuardarComandoPropio(1, 0, j4); err == nil {
		t.Fatal("máximo 3 archivos")
	}
}

// De punta a punta con el bot arrancado: llega un mensaje por WhatsApp y sale la bienvenida con su archivo (y la etiqueta automático);
// un segundo archivo que no se puede leer se omite y queda anotado sin nombres.
func TestBienvenidaDePuntaAPuntaConArchivoQueFalla(t *testing.T) {
	s := arrancaBot(t)
	cli := &clienteFalso{}
	s.envio = envio.Nuevo(1, alm, cli, nil)
	apagaMenusDeSistema(t)
	foto := subeAdj(t, "imagen", "image/jpeg", "bienvenida.jpg", []byte("JPG-BIENVENIDA"))
	roto := subeAdj(t, "documento", "application/pdf", "roto.pdf", []byte("PDF"))
	alm.BorrarAdjuntoMenu(1, roto.ID) // el archivo se perdió después de guardar el mensaje
	cfg := auto.ConfigPorDefecto("bienvenida")
	b, _ := json.Marshal(autoEntrada{Tipo: "bienvenida", Texto: "¡Hola {nombre}!", Config: cfg, Adjuntos: []almacen.AdjuntoRef{foto, roto}, Activo: true})
	// se guarda con los dos archivos existentes y luego se pierde uno
	roto2 := subeAdj(t, "documento", "application/pdf", "roto.pdf", []byte("PDF"))
	b, _ = json.Marshal(autoEntrada{Tipo: "bienvenida", Texto: "¡Hola {nombre}!", Config: cfg, Adjuntos: []almacen.AdjuntoRef{foto, roto2}, Activo: true})
	id, err := GuardarAuto(1, string(b))
	if err != nil {
		t.Fatal(err)
	}
	alm.BorrarAdjuntoMenu(1, roto2.ID)
	ana := types.NewJID("5215511111111", types.DefaultUserServer)
	s.evento(textoDeChat("W1", ana, "hola, buenas tardes"))
	esperaHasta(t, "no salió la bienvenida", func() bool { cli.mu.Lock(); defer cli.mu.Unlock(); return len(cli.enviados) >= 1 })
	time.Sleep(1500 * time.Millisecond) // por si saliera algo más
	cli.mu.Lock()
	n := len(cli.enviados)
	img := cli.enviados[0].GetImageMessage()
	cli.mu.Unlock()
	if n != 1 || img == nil || !strings.HasPrefix(img.GetCaption(), "¡Hola ") {
		t.Fatalf("sale la foto con el texto de pie y el archivo roto se omite: %d %+v", n, img)
	}
	// la etiqueta "automático" quedó en el mensaje que salió
	au, _ := alm.ListarMensajesAuto(1, ana.String(), 0, 10)
	if len(au) != 1 || au[0].Automatico != "bienvenida" || !au[0].EsMio {
		t.Fatalf("%+v", au)
	}
	esperaHasta(t, "no quedó el registro", func() bool { r, _ := alm.ListarAutoRegistro(1, id, 5); return len(r) == 1 })
	r, _ := alm.ListarAutoRegistro(1, id, 5)
	if r[0].Resultado != almacen.AutoParcial || r[0].AdjOK != 1 || r[0].AdjFallo != 1 || r[0].Motivo != "archivo_ilegible" {
		t.Fatalf("%+v", r[0])
	}
	if alm.UltimaRepeticion(1, id, ana.String()) == 0 {
		t.Fatal("repetición por persona")
	}
	// mismo mensaje, otra vez: la bienvenida es solo la primera vez
	s.evento(textoDeChat("W2", ana, "otra cosa"))
	time.Sleep(700 * time.Millisecond)
	cli.mu.Lock()
	n = len(cli.enviados)
	cli.mu.Unlock()
	if n != 1 {
		t.Fatalf("no se repite: %d", n)
	}
}

func apagaMenusDeSistema(t *testing.T) {
	t.Helper()
	ms, _ := alm.ListarMenus(1)
	for _, m := range ms {
		if m.Sistema {
			_ = alm.FijarMenuActivo(1, m.ID, false)
		}
	}
}

// El Saludo de fábrica (regla "cualquier mensaje") se lleva la prioridad: los automáticos no se meten y la pantalla avisa.
func TestUnaReglaQueContestaATodoTienePrioridadYSeAvisa(t *testing.T) {
	s := arrancaBot(t)
	cli := &clienteFalso{}
	s.envio = envio.Nuevo(1, alm, cli, nil)
	guarda(t, autoEntrada{Tipo: "recibido", Texto: "Recibido", Config: auto.ConfigPorDefecto("recibido"), Activo: true})
	// v0.6.0 E9: el Saludo de fábrica ya no es un conflicto ni tapa los mensajes automáticos.
	l := decodifica[autoLista](t, func() (string, error) { return ListarAutos(1, "") })
	if len(l.Conflictos) != 0 {
		t.Fatalf("el Saludo de fábrica no debe avisar como conflicto: %+v", l.Conflictos)
	}
	ana := types.NewJID("5215511111111", types.DefaultUserServer)
	s.evento(textoDeChat("W1", ana, "hola"))
	esperaHasta(t, "no salió el recibido", func() bool { cli.mu.Lock(); defer cli.mu.Unlock(); return len(cli.enviados) >= 1 })
	cli.mu.Lock()
	txt := cli.enviados[0].GetExtendedTextMessage().GetText()
	cli.mu.Unlock()
	if txt != "Recibido" {
		t.Fatalf("el automático manda sobre el Saludo: %q", txt)
	}
	au, _ := alm.ListarMensajesAuto(1, ana.String(), 0, 10)
	if len(au) != 1 || au[0].Automatico != "recibido" {
		t.Fatalf("se etiqueta como automático: %+v", au)
	}
	// Una regla PROPIA que contesta a todo sí tapa a los automáticos y se avisa.
	apagaMenusDeSistema(t)
	todo := &flujo.Def{V: 1, Nombre: "Todo", Inicio: "r", Pasos: []flujo.Paso{{ID: "r", Tipo: flujo.PasoRespuesta, Texto: "Regla propia"}},
		Disparadores: flujo.Disparadores{Cualquiera: true}, Alcance: flujo.Alcance{Privados: true}}
	todo.Normalizar()
	if _, err := alm.CrearMenu(1, todo.JSON(), true, false); err != nil {
		t.Fatal(err)
	}
	l = decodifica[autoLista](t, func() (string, error) { return ListarAutos(1, "") })
	if len(l.Conflictos) != 1 || l.Conflictos[0] != "Todo" {
		t.Fatalf("avisa de la regla propia: %+v", l.Conflictos)
	}
	beto := types.NewJID("5215522222222", types.DefaultUserServer)
	s.evento(textoDeChat("W2", beto, "hola"))
	esperaHasta(t, "no salió la regla", func() bool { cli.mu.Lock(); defer cli.mu.Unlock(); return len(cli.enviados) >= 2 })
	cli.mu.Lock()
	txt = cli.enviados[1].GetExtendedTextMessage().GetText()
	cli.mu.Unlock()
	if txt != "Regla propia" {
		t.Fatalf("la regla propia manda: %q", txt)
	}
}

// El atajo escrito en el teléfono: el mensaje se edita y queda con el texto; nunca para un atajo que no existe ni para un cliente.
func TestAtajoDesdeElTelefonoDePuntaAPunta(t *testing.T) {
	s := arrancaBot(t)
	cli := &clienteFalso{}
	s.envio = envio.Nuevo(1, alm, cli, nil)
	apagaMenusDeSistema(t)
	guarda(t, autoEntrada{Tipo: "atajo", Nombre: "gracias", Texto: "¡Gracias por escribir!", Activo: true})
	ana := types.NewJID("5215511111111", types.DefaultUserServer)
	yo := *s.cli.Store.ID
	mio := func(id, texto string) *events.Message {
		e := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: ana, Sender: yo, IsFromMe: true}, ID: id, Timestamp: time.Now()},
			RawMessage: &waE2E.Message{Conversation: proto.String(texto)}}
		e.UnwrapRaw()
		return e
	}
	s.evento(mio("MIO1", "/noexiste"))
	s.evento(mio("MIO2", "/gracias por todo"))
	s.evento(textoDeChat("W9", ana, "/gracias")) // lo escribe el cliente, no yo
	time.Sleep(700 * time.Millisecond)
	cli.mu.Lock()
	n := len(cli.enviados)
	cli.mu.Unlock()
	for _, id := range []string{"MIO1", "MIO2"} {
		if m, _ := alm.ObtenerMensaje(1, ana.String(), id); m != nil && m.Automatico != "" {
			t.Fatalf("%s no es un atajo", id)
		}
	}
	if n != 0 {
		t.Fatalf("nada de eso activa un atajo: %d envíos", n)
	}
	s.evento(mio("MIO3", "/gracias"))
	esperaHasta(t, "no se editó el mensaje", func() bool {
		m, _ := alm.ObtenerMensaje(1, ana.String(), "MIO3")
		return m != nil && m.Texto == "¡Gracias por escribir!"
	})
	m, _ := alm.ObtenerMensaje(1, ana.String(), "MIO3")
	if !m.Editado || m.Automatico != "atajo" {
		t.Fatalf("%+v", m)
	}
}
