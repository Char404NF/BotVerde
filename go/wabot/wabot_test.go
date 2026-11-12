package wabot

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/ingesta"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

type falso struct {
	mu      sync.Mutex
	estados []string
	lineas  []string
}

func (f *falso) OnStatus(t string)      { f.mu.Lock(); f.estados = append(f.estados, t); f.mu.Unlock() }
func (f *falso) OnPairingCode(c string) {}
func (f *falso) OnLog(l string)         { f.mu.Lock(); f.lineas = append(f.lineas, l); f.mu.Unlock() }

type obsFalso struct {
	mu    sync.Mutex
	lotes []string
}

func (o *obsFalso) OnCambio(j string) { o.mu.Lock(); o.lotes = append(o.lotes, j); o.mu.Unlock() }

func clavePrueba() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 40)
	}
	return k
}

// reinicia deja el estado global limpio entre pruebas.
func reinicia(t *testing.T) {
	t.Helper()
	CerrarAlmacen()
	mu.Lock()
	agrup = nil
	mu.Unlock()
	t.Cleanup(func() { CerrarAlmacen() })
}

// Sin conexión real: abre la base, valida números y cierra.
func TestStartPairStop(t *testing.T) {
	reinicia(t)
	f := &falso{}
	raiz := t.TempDir()
	if err := Start(raiz, 1, clavePrueba(), f); err != nil {
		t.Fatal(err)
	}
	if IsConnected() {
		t.Fatal("no debe estar conectado")
	}
	if err := Start(raiz, 1, clavePrueba(), f); err == nil {
		t.Fatal("un segundo Start debía fallar")
	}
	if err := Pair("12345"); err == nil {
		t.Fatal("número corto debía fallar")
	}
	if err := Pair("0123456789"); err == nil {
		t.Fatal("número sin código de país debía fallar")
	}
	// Con el bot iniciado, la lista de chats ya se puede consultar (vacía).
	if j, err := ListarChats(1, false, "", 50, 0); err != nil || j != "[]" {
		t.Fatalf("%q %v", j, err)
	}
	Stop()
	// Después de detener el bot, lo guardado sigue disponible.
	if j, err := ListarChats(1, false, "", 50, 0); err != nil || j != "[]" {
		t.Fatalf("tras Stop: %q %v", j, err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.estados) < 2 || f.estados[0] != "sin_vincular" || f.estados[len(f.estados)-1] != "detenido" {
		t.Fatalf("estados inesperados: %v", f.estados)
	}
	if err := Pair("525512345678"); err == nil {
		t.Fatal("después de Stop, Pair debía fallar")
	}
}

func TestDsnUsaMemoriaParaTemporales(t *testing.T) {
	if d := dsnSesion("/datos/bot"); !strings.Contains(d, "temp_store(2)") || !strings.HasPrefix(d, "file:/datos/bot/sesion.db?") {
		t.Fatalf("dsn inesperado: %s", d)
	}
}

func TestLlaveInvalida(t *testing.T) {
	reinicia(t)
	if err := Start(t.TempDir(), 1, []byte("corta"), &falso{}); err == nil {
		t.Fatal("una llave corta debía fallar")
	}
	if err := AbrirAlmacen(t.TempDir(), nil, nil); err == nil {
		t.Fatal("sin llave debía fallar")
	}
	if _, err := ListarChats(1, false, "", 10, 0); err == nil {
		t.Fatal("sin almacén abierto debe dar error claro")
	}
}

func TestAlmacenSinBotYApiJSON(t *testing.T) {
	reinicia(t)
	obs := &obsFalso{}
	raiz := t.TempDir()
	if err := AbrirAlmacen(raiz, clavePrueba(), obs); err != nil {
		t.Fatal(err)
	}
	if err := AbrirAlmacen(raiz, clavePrueba(), obs); err != nil {
		t.Fatalf("abrir dos veces la misma carpeta es válido: %v", err)
	}
	if err := AbrirAlmacen(t.TempDir(), clavePrueba(), obs); err == nil {
		t.Fatal("otra carpeta distinta no se acepta")
	}
	a, _ := almacenAbierto()
	chat := "5215512345678@s.whatsapp.net"
	_, _ = a.ActualizarChat(1, chat, false, func(c *almacen.Chat) { c.TelE164 = "+5215512345678"; c.Publico = "Ana" })
	_, _ = a.GuardarMensaje(&almacen.Mensaje{ID: "M1", ChatJID: chat, BotID: 1, Tipo: almacen.TipoTexto, Texto: "hola", TS: 100})
	rel := almacen.RutaMedio(1, chat, "F1")
	_ = a.EscribirCifrado(rel, make([]byte, 5000))
	_, _ = a.GuardarMensaje(&almacen.Mensaje{ID: "F1", ChatJID: chat, BotID: 1, Tipo: almacen.TipoImagen, TS: 101, ArchivoRuta: rel, ArchivoBytes: 5000, ArchivoEstado: almacen.EstadoDescargado})

	var chats []map[string]any
	j, err := ListarChats(1, false, "ana", 10, 0)
	if err != nil || json.Unmarshal([]byte(j), &chats) != nil || len(chats) != 1 || chats[0]["nombre"] != "Ana" || chats[0]["telefono"] != "+52 55 1234 5678" || chats[0]["pais"] != "MX" {
		t.Fatalf("%s %v", j, err)
	}
	j, _ = ListarMensajes(1, chat, 0, "", 10)
	var msgs []map[string]any
	if json.Unmarshal([]byte(j), &msgs) != nil || len(msgs) != 2 {
		t.Fatalf("%s", j)
	}
	j, _ = ListarArchivos(1, "", "foto", true, 10, 0)
	var arch []map[string]any
	if json.Unmarshal([]byte(j), &arch) != nil || len(arch) != 1 || arch[0]["id"] != "F1" {
		t.Fatalf("%s", j)
	}
	j, _ = EspacioPorTipo(1)
	if !strings.Contains(j, `"categoria":"foto","bytes":5000`) {
		t.Fatalf("%s", j)
	}
	j, _ = EspacioPorChat(1)
	if !strings.Contains(j, `"bytes":5000`) {
		t.Fatalf("%s", j)
	}
	if j, _ = EspacioTotal(1); !strings.Contains(j, `"libre"`) {
		t.Fatalf("%s", j)
	}
	if j, _ = EstimarBorrado(1, `{"categoria":"foto"}`); j != `{"bytes":5000,"cantidad":1}` {
		t.Fatalf("%s", j)
	}
	if j, _ = EstimarLista(1, `[{"chat":"`+chat+`","id":"F1"}]`); j != `{"bytes":5000,"cantidad":1}` {
		t.Fatalf("%s", j)
	}
	if j, err = BorrarArchivos(1, `{"categoria":"foto"}`); err != nil || j != `{"bytes":5000,"cantidad":1}` {
		t.Fatalf("%s %v", j, err)
	}
	if _, err = EstimarBorrado(1, `no es json`); err == nil {
		t.Fatal("un filtro inválido debe dar error")
	}
	if j, _ = Reparar(1); !strings.Contains(j, `"estadosCorregidos":0`) {
		t.Fatalf("%s", j)
	}
	// Los cambios llegaron al observador, agrupados.
	time.Sleep(500 * time.Millisecond)
	obs.mu.Lock()
	defer obs.mu.Unlock()
	if len(obs.lotes) == 0 {
		t.Fatal("el observador no recibió cambios")
	}
	todo := strings.Join(obs.lotes, "")
	if !strings.Contains(todo, `"chats"`) || !strings.Contains(todo, `"espacio"`) {
		t.Fatalf("faltan eventos: %s", todo)
	}
	if _, err := BorrarChat(1, chat); err != nil {
		t.Fatal(err)
	}
	if err := EliminarBot(1); err != nil {
		t.Fatal(err)
	}
	if err := Descargar(1, chat, "F1"); err == nil || !strings.Contains(err.Error(), "inicia el bot") {
		t.Fatalf("sin bot no se puede descargar: %v", err)
	}
	if _, err := FotoGrande(1, chat); err == nil {
		t.Fatal("sin bot no hay foto grande")
	}
}

func TestConfigurarDescargas(t *testing.T) {
	defer func() { polMu.Lock(); pol = ingesta.PoliticaPorDefecto(); polMu.Unlock() }()
	if err := ConfigurarDescargas(`{"video":{"wifi":true,"datos":true},"videoMaxBytes":1000,"pisoLibre":42}`); err != nil {
		t.Fatal(err)
	}
	p := politicaActual()
	if !p.Video.Datos || p.VideoMaxBytes != 1000 || p.PisoLibre != 42 || !p.Foto.Wifi || p.MaxBytes != 0 || !p.SoloWifiGrandes || p.GrandePct != 25 {
		t.Fatalf("%+v", p)
	}
	if !strings.Contains(DescargasConfig(), `"pisoLibre":42`) {
		t.Fatal(DescargasConfig())
	}
	if ConfigurarDescargas(`{mal`) == nil {
		t.Fatal("JSON inválido")
	}
	EstablecerRed(RedWifi)
	if redActual() != 1 {
		t.Fatal("red")
	}
	EstablecerRed(RedDesconocida)
}

// ---------- el bot de punta a punta, sin red ----------

func TestEventoDeMensajeSeGuarda(t *testing.T) {
	reinicia(t)
	f := &falso{}
	obs := &obsFalso{}
	raiz := t.TempDir()
	if err := AbrirAlmacen(raiz, clavePrueba(), obs); err != nil {
		t.Fatal(err)
	}
	if err := Start(raiz, 1, clavePrueba(), f); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	s := st
	mu.Unlock()
	grupo, _ := types.ParseJID("120363000000000001@g.us")
	ana, _ := types.ParseJID("34600000001@s.whatsapp.net")
	e := &events.Message{Info: types.MessageInfo{
		MessageSource: types.MessageSource{Chat: grupo, Sender: ana, IsGroup: true}, ID: "G1", Timestamp: time.Now(), PushName: "Ana"},
		RawMessage: &waE2E.Message{Conversation: proto.String("hola grupo")}}
	e.UnwrapRaw()
	s.evento(e)
	// ver una vez redactado (sin datos para descargar): solo el aviso
	vo := &events.Message{Info: e.Info, RawMessage: &waE2E.Message{ImageMessage: &waE2E.ImageMessage{ViewOnce: proto.Bool(true), JPEGThumbnail: []byte("MINI")}}}
	vo.Info.ID = "VO"
	vo.UnwrapRaw()
	s.evento(vo)
	// el evento se procesa en segundo plano
	var j string
	for i := 0; i < 100; i++ {
		j, _ = ListarMensajes(1, grupo.String(), 0, "", 10)
		if strings.Count(j, `"id"`) == 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if strings.Count(j, `"id"`) != 2 || !strings.Contains(j, "hola grupo") || !strings.Contains(j, almacen.TextoVerUnaVez) || strings.Contains(j, "MINI") {
		t.Fatalf("%s", j)
	}
	if lista, _ := ListarChats(1, true, "", 10, 0); !strings.Contains(lista, `"esGrupo":true`) {
		t.Fatalf("grupos: %s", lista)
	}
	Stop()
	// Ninguna línea de registro lleva texto, nombres ni números largos.
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, l := range f.lineas {
		for _, malo := range []string{"hola grupo", "Ana", "34600000001", "120363000000000001"} {
			if strings.Contains(l, malo) {
				t.Fatalf("el registro lleva datos personales: %q", l)
			}
		}
	}
}

func TestEnmascaraTexto(t *testing.T) {
	casos := map[string]string{
		"Successfully paired 5215500000002:16@s.whatsapp.net": "Successfully paired …2121:16@s.whatsapp.net",
		"sin números largos 123":                              "sin números largos 123",
		"dos 521241132212 y 34600000001":                      "dos …2212 y …0001",
	}
	for in, want := range casos {
		if got := enmascaraTexto(in); got != want {
			t.Errorf("%q -> %q (se esperaba %q)", in, got, want)
		}
	}
}

func TestRegistroDescartaRuido(t *testing.T) {
	var lineas []string
	r := (&registro{salida: func(s string) { lineas = append(lineas, s) }}).Sub("WA")
	r.Errorf("Error reading from websocket: failed to get reader: failed to read frame header: EOF")
	r.Warnf("%d duplicate contacts found in PutAllContactNames", 637)
	r.Warnf("Error sending close to websocket: failed to close WebSocket: failed to read frame header: EOF")
	r.Debugf("depuración")
	if len(lineas) != 0 {
		t.Fatalf("el ruido debía quedar en debug: %v", lineas)
	}
	r.Errorf("Failed to sync app state: algo real")
	if len(lineas) != 1 || !strings.Contains(lineas[0], "algo real") {
		t.Fatalf("%v", lineas)
	}
}

func TestVigilanteReconectaTras3Minutos(t *testing.T) {
	var mu sync.Mutex
	ahora := time.Unix(1000, 0)
	conectado := false
	llamadas := 0
	v := &vigilante{vinculado: func() bool { return true }, conectado: func() bool { mu.Lock(); defer mu.Unlock(); return conectado },
		conectar: func() { mu.Lock(); llamadas++; mu.Unlock() }, log: func(string) {}, limite: 3 * time.Minute, paso: 5 * time.Millisecond,
		ahora: func() time.Time { mu.Lock(); defer mu.Unlock(); return ahora }}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { v.correr(ctx); close(done) }()
	avanza := func(d time.Duration) { mu.Lock(); ahora = ahora.Add(d); mu.Unlock(); time.Sleep(40 * time.Millisecond) }
	time.Sleep(30 * time.Millisecond) // toma la hora de inicio
	avanza(2 * time.Minute)
	mu.Lock()
	if llamadas != 0 {
		t.Fatal("antes de 3 minutos no reconecta")
	}
	mu.Unlock()
	avanza(2 * time.Minute)
	mu.Lock()
	if llamadas != 1 {
		t.Fatalf("a los 4 minutos debe reconectar una vez: %d", llamadas)
	}
	conectado = true
	mu.Unlock()
	avanza(10 * time.Minute)
	mu.Lock()
	if llamadas != 1 {
		t.Fatal("conectado no reconecta")
	}
	mu.Unlock()
	cancel() // Stop detiene el vigilante
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("el vigilante no se detuvo")
	}
}

func TestAgrupadorJuntaEventos(t *testing.T) {
	var mu sync.Mutex
	var lotes []string
	a := nuevoAgrupador(30*time.Millisecond, func(s string) { mu.Lock(); lotes = append(lotes, s); mu.Unlock() })
	for i := 0; i < 50; i++ {
		a.Agregar("chats")
		a.Agregar("chat:uno")
	}
	time.Sleep(120 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(lotes) != 1 || lotes[0] != `["chats","chat:uno"]` {
		t.Fatalf("%v", lotes)
	}
}
