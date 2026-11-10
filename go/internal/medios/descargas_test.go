package medios

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/ingesta"

	"go.mau.fi/whatsmeow"
)

type clienteFalso struct {
	mu        sync.Mutex
	llamadas  int
	enVuelo   int32
	maxVuelo  int32
	respuesta func(path string) ([]byte, error)
	bloqueo   chan struct{} // si no es nil, cada descarga espera aquí
}

func (c *clienteFalso) DownloadMediaWithPath(ctx context.Context, directPath string, enc, sha, clave []byte, mt whatsmeow.MediaType, mms string, sinHash bool) ([]byte, error) {
	c.mu.Lock()
	c.llamadas++
	c.mu.Unlock()
	n := atomic.AddInt32(&c.enVuelo, 1)
	for {
		m := atomic.LoadInt32(&c.maxVuelo)
		if n <= m || atomic.CompareAndSwapInt32(&c.maxVuelo, m, n) {
			break
		}
	}
	defer atomic.AddInt32(&c.enVuelo, -1)
	if c.bloqueo != nil {
		select {
		case <-c.bloqueo:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return c.respuesta(directPath)
}

type entorno struct {
	alm       *almacen.Almacen
	g         *Gestor
	cli       *clienteFalso
	libre     int64
	pol       ingesta.Politica
	logs      []string
	conectado atomic.Bool
}

func nuevoEntorno(t *testing.T) *entorno {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(3 * i)
	}
	alm, err := almacen.Abrir(t.TempDir(), k)
	if err != nil {
		t.Fatal(err)
	}
	e := &entorno{alm: alm, libre: 100 << 30, pol: ingesta.PoliticaPorDefecto(), cli: &clienteFalso{respuesta: func(string) ([]byte, error) { return []byte("contenido de prueba"), nil }}}
	e.conectado.Store(true)
	e.g = Nuevo(Config{BotID: 1, Alm: alm,
		Cliente:  func() (Cliente, bool) { return e.cli, e.conectado.Load() },
		Politica: func() ingesta.Politica { return e.pol }, Libre: func() int64 { return e.libre },
		Esperas: []time.Duration{20 * time.Millisecond, 20 * time.Millisecond}, Log: func(s string) { e.logs = append(e.logs, s) }})
	e.g.Iniciar(context.Background())
	t.Cleanup(func() { e.g.Detener(); _ = alm.Cerrar() })
	return e
}

func (e *entorno) mensaje(t *testing.T, id, tipo string, bytes int64) {
	t.Helper()
	chat := "1@s.whatsapp.net"
	if _, err := e.alm.ActualizarChat(1, chat, false, func(c *almacen.Chat) { c.TelE164 = "+521" }); err != nil {
		t.Fatal(err)
	}
	info, _ := json.Marshal(ingesta.InfoDescarga{Tipo: tipo, DirectPath: "/v/" + id, MediaKey: "Y2xhdmU=", SHA256: "c2hh", EncSHA256: "ZXNoYQ=="})
	_, err := e.alm.GuardarMensaje(&almacen.Mensaje{ID: id, ChatJID: chat, BotID: 1, Tipo: tipo, TS: 10, ArchivoInfo: info, ArchivoBytes: bytes, ArchivoEstado: almacen.EstadoPendiente})
	if err != nil {
		t.Fatal(err)
	}
}

func (e *entorno) estado(id string) string {
	m, _ := e.alm.ObtenerMensaje(1, "1@s.whatsapp.net", id)
	if m == nil {
		return "?"
	}
	return m.ArchivoEstado
}

func esperar(t *testing.T, descripcion string, cond func() bool) {
	t.Helper()
	fin := time.Now().Add(5 * time.Second)
	for time.Now().Before(fin) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("tiempo agotado esperando: %s", descripcion)
}

func TestDescargaCifraAlDisco(t *testing.T) {
	e := nuevoEntorno(t)
	secreto := bytes.Repeat([]byte("FOTO-SECRETA-"), 30000)
	e.cli.respuesta = func(string) ([]byte, error) { return secreto, nil }
	e.mensaje(t, "F1", "imagen", 1000)
	e.g.Encolar("1@s.whatsapp.net", "F1")
	esperar(t, "descargado", func() bool { return e.estado("F1") == almacen.EstadoDescargado })
	m, _ := e.alm.ObtenerMensaje(1, "1@s.whatsapp.net", "F1")
	if m.ArchivoBytes != int64(len(secreto)) || m.ArchivoRuta != almacen.RutaMedio(1, "1@s.whatsapp.net", "F1") {
		t.Fatalf("%+v", m)
	}
	abs, _ := e.alm.Abs(m.ArchivoRuta)
	crudo, _ := os.ReadFile(abs)
	if bytes.Contains(crudo, []byte("FOTO-SECRETA")) {
		t.Fatal("el archivo quedó en claro")
	}
	if _, err := os.Stat(abs + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("quedó un .tmp")
	}
	if got, err := e.alm.LeerCifrado(m.ArchivoRuta); err != nil || !bytes.Equal(got, secreto) {
		t.Fatalf("no se lee igual: %v", err)
	}
	// Una segunda orden para el mismo archivo no vuelve a bajarlo.
	antes := e.cli.llamadas
	e.g.Encolar("1@s.whatsapp.net", "F1")
	time.Sleep(50 * time.Millisecond)
	if e.cli.llamadas != antes {
		t.Fatal("no debe repetir la descarga")
	}
}

func TestMaximoDosDescargasAlMismoTiempo(t *testing.T) {
	e := nuevoEntorno(t)
	e.cli.bloqueo = make(chan struct{})
	for _, id := range []string{"A", "B", "C", "D", "E"} {
		e.mensaje(t, id, "documento", 1000)
		e.g.Encolar("1@s.whatsapp.net", id)
	}
	esperar(t, "dos en vuelo", func() bool { return atomic.LoadInt32(&e.cli.enVuelo) == 2 })
	time.Sleep(60 * time.Millisecond)
	if atomic.LoadInt32(&e.cli.enVuelo) != 2 {
		t.Fatalf("en vuelo: %d", e.cli.enVuelo)
	}
	close(e.cli.bloqueo)
	esperar(t, "las cinco descargadas", func() bool {
		for _, id := range []string{"A", "B", "C", "D", "E"} {
			if e.estado(id) != almacen.EstadoDescargado {
				return false
			}
		}
		return true
	})
	if atomic.LoadInt32(&e.cli.maxVuelo) > 2 {
		t.Fatalf("hubo %d a la vez", e.cli.maxVuelo)
	}
}

func TestArchivosGrandesDeUnoEnUno(t *testing.T) {
	e := nuevoEntorno(t)
	e.cli.bloqueo = make(chan struct{})
	for _, id := range []string{"G1", "G2"} {
		e.mensaje(t, id, "video", 30<<20)
		e.g.Encolar("1@s.whatsapp.net", id)
	}
	esperar(t, "uno en vuelo", func() bool { return atomic.LoadInt32(&e.cli.enVuelo) == 1 })
	time.Sleep(60 * time.Millisecond)
	if atomic.LoadInt32(&e.cli.enVuelo) != 1 {
		t.Fatalf("los grandes bajan de uno en uno, hay %d", e.cli.enVuelo)
	}
	close(e.cli.bloqueo)
	esperar(t, "ambos", func() bool {
		return e.estado("G1") == almacen.EstadoDescargado && e.estado("G2") == almacen.EstadoDescargado
	})
}

func TestReintentosConEsperaYFallo(t *testing.T) {
	e := nuevoEntorno(t)
	var intentos int32
	e.cli.respuesta = func(string) ([]byte, error) {
		if atomic.AddInt32(&intentos, 1) < 3 {
			return nil, errors.New("sin red")
		}
		return []byte("ok"), nil
	}
	e.mensaje(t, "R1", "audio", 10)
	e.g.Encolar("1@s.whatsapp.net", "R1")
	esperar(t, "descargado tras 2 fallos", func() bool { return e.estado("R1") == almacen.EstadoDescargado })
	if intentos != 3 {
		t.Fatalf("intentos: %d", intentos)
	}
	// Siempre falla: agota los intentos y queda fallido.
	e.cli.respuesta = func(string) ([]byte, error) { return nil, errors.New("sin red") }
	e.mensaje(t, "R2", "audio", 10)
	e.g.Encolar("1@s.whatsapp.net", "R2")
	esperar(t, "fallido", func() bool { return e.estado("R2") == almacen.EstadoFallido })
	// Un archivo expirado (410) falla de inmediato sin reintentos.
	var n410 int32
	e.cli.respuesta = func(string) ([]byte, error) {
		atomic.AddInt32(&n410, 1)
		return nil, whatsmeow.ErrMediaDownloadFailedWith410
	}
	e.mensaje(t, "R3", "audio", 10)
	e.g.Encolar("1@s.whatsapp.net", "R3")
	esperar(t, "fallido por expirado", func() bool { return e.estado("R3") == almacen.EstadoFallido })
	time.Sleep(80 * time.Millisecond)
	if n410 != 1 {
		t.Fatalf("un 410 no se reintenta: %d", n410)
	}
	// El botón "Descargar" vuelve a intentarlo.
	e.cli.respuesta = func(string) ([]byte, error) { return []byte("ya sí"), nil }
	if err := e.g.Manual("1@s.whatsapp.net", "R3"); err != nil {
		t.Fatal(err)
	}
	esperar(t, "descargado con el botón", func() bool { return e.estado("R3") == almacen.EstadoDescargado })
}

func TestPocoEspacioPausaLasAutomaticasPeroNoElBoton(t *testing.T) {
	e := nuevoEntorno(t)
	e.libre = 100 << 20 // menos del piso de 500 MB
	e.mensaje(t, "P1", "imagen", 100)
	e.g.Encolar("1@s.whatsapp.net", "P1")
	esperar(t, "queda con botón", func() bool { return e.estado("P1") == almacen.EstadoNinguno })
	if e.cli.llamadas != 0 {
		t.Fatal("no debe descargar con poco espacio")
	}
	if err := e.g.Manual("1@s.whatsapp.net", "P1"); err != nil {
		t.Fatal(err)
	}
	esperar(t, "el botón sí descarga", func() bool { return e.estado("P1") == almacen.EstadoDescargado })
}

func TestSinConexionNoGastaIntentos(t *testing.T) {
	e := nuevoEntorno(t)
	e.conectado.Store(false)
	e.mensaje(t, "C1", "imagen", 100)
	e.g.Encolar("1@s.whatsapp.net", "C1")
	time.Sleep(100 * time.Millisecond)
	if e.estado("C1") != almacen.EstadoPendiente || e.cli.llamadas != 0 {
		t.Fatalf("sin conexión espera: %s", e.estado("C1"))
	}
}

func TestManualNoAceptaLoQueNoSePuedeBajar(t *testing.T) {
	e := nuevoEntorno(t)
	chat := "1@s.whatsapp.net"
	_, _ = e.alm.ActualizarChat(1, chat, false, func(c *almacen.Chat) { c.TelE164 = "+521" })
	_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: "VO", ChatJID: chat, BotID: 1, Tipo: almacen.TipoVerUnaVez, TS: 1, ArchivoEstado: almacen.EstadoOmitido})
	if err := e.g.Manual(chat, "VO"); err == nil {
		t.Fatal("un ver-una-vez no se puede descargar")
	}
	_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: "TX", ChatJID: chat, BotID: 1, Tipo: almacen.TipoTexto, Texto: "hola", TS: 2})
	if err := e.g.Manual(chat, "TX"); err == nil {
		t.Fatal("un texto no tiene archivo")
	}
}

func TestReanudaPendientesAlIniciar(t *testing.T) {
	e := nuevoEntorno(t)
	e.g.Detener()
	e.mensaje(t, "Z1", "imagen", 10) // queda "pendiente" en la base, como tras cerrar la app
	g2 := Nuevo(e.g.cfg)
	g2.Iniciar(context.Background())
	defer g2.Detener()
	esperar(t, "reanudado", func() bool { return e.estado("Z1") == almacen.EstadoDescargado })
}

func TestTopeDeTamanoRealEnAutomaticas(t *testing.T) {
	e := nuevoEntorno(t)
	e.pol.MaxBytes = 10
	e.cli.respuesta = func(string) ([]byte, error) { return bytes.Repeat([]byte{1}, 100), nil }
	e.mensaje(t, "T1", "documento", 0) // tamaño desconocido en el mensaje
	e.g.Encolar("1@s.whatsapp.net", "T1")
	esperar(t, "se descarta por grande", func() bool { return e.estado("T1") == almacen.EstadoNinguno })
	if m, _ := e.alm.ObtenerMensaje(1, "1@s.whatsapp.net", "T1"); m.ArchivoRuta != "" {
		t.Fatal("no debe quedar archivo")
	}
}

func TestLosLogsNoLlevanDatosPersonales(t *testing.T) {
	e := nuevoEntorno(t)
	e.cli.respuesta = func(string) ([]byte, error) { return nil, errors.New("fallo con 5215512345678 y nombre Pepe") }
	e.mensaje(t, "L1", "audio", 10)
	e.g.Encolar("1@s.whatsapp.net", "L1")
	esperar(t, "fallido", func() bool { return e.estado("L1") == almacen.EstadoFallido })
	for _, l := range e.logs {
		if bytes.Contains([]byte(l), []byte("5215512345678")) || bytes.Contains([]byte(l), []byte("Pepe")) || bytes.Contains([]byte(l), []byte("@s.whatsapp.net")) {
			t.Fatalf("el registro lleva datos personales: %q", l)
		}
	}
}
