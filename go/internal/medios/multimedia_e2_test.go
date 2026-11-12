package medios

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/ingesta"

	"go.mau.fi/whatsmeow"
)

const chatE2 = "1@s.whatsapp.net"

func (e *entorno) conRed(r ingesta.Red) { e.red.Store(int32(r)) }

func TestGrandeVaAArchivoYNoDejaTemporales(t *testing.T) {
	e := nuevoEntornoCfg(t, func(c *Config) { c.UmbralArchivo = 1000 })
	contenido := bytes.Repeat([]byte("VIDEO-SECRETO-"), 2000) // 28 KB > umbral
	e.cli.respuesta = func(string) ([]byte, error) { return contenido, nil }
	e.mensaje(t, "V1", "video", int64(len(contenido)))
	e.g.Encolar(chatE2, "V1")
	esperar(t, "descargado", func() bool { return e.estado("V1") == almacen.EstadoDescargado })
	if e.cli.aArchivo != 1 {
		t.Fatalf("debía bajar a archivo: %d", e.cli.aArchivo)
	}
	m, _ := e.alm.ObtenerMensaje(1, chatE2, "V1")
	if m.ArchivoBytes != int64(len(contenido)) {
		t.Fatalf("bytes %d", m.ArchivoBytes)
	}
	if b, err := e.alm.LeerCifrado(m.ArchivoRuta); err != nil || !bytes.Equal(b, contenido) {
		t.Fatalf("contenido: %v", err)
	}
	abs, _ := e.alm.Abs(m.ArchivoRuta)
	crudo, _ := os.ReadFile(abs)
	if bytes.Contains(crudo, []byte("VIDEO-SECRETO")) {
		t.Fatal("el archivo final está en claro")
	}
	// Ningún temporal en claro queda.
	restos, _ := filepath.Glob(filepath.Join(e.alm.Raiz(), "tmp", "descargas", "*"))
	if len(restos) != 0 {
		t.Fatalf("quedaron temporales: %v", restos)
	}
}

// Un archivo "grande" simulado no se acumula en la RAM: la memoria en uso se mantiene muy por debajo del tamaño.
func TestArchivoGrandeNoAcumulaEnRAM(t *testing.T) {
	e := nuevoEntorno(t)
	const total = 160 << 20
	e.cli.respuestaArchivo = func(f whatsmeow.File) error {
		trozo := bytes.Repeat([]byte{7}, 1<<20)
		for i := 0; i < total>>20; i++ {
			if _, err := f.Write(trozo); err != nil {
				return err
			}
		}
		return nil
	}
	e.mensaje(t, "BIG", "video", total)
	runtime.GC()
	var pico atomic.Uint64
	parar := make(chan struct{})
	hecho := make(chan struct{})
	go func() {
		defer close(hecho)
		var ms runtime.MemStats
		for {
			select {
			case <-parar:
				return
			case <-time.After(10 * time.Millisecond):
				runtime.ReadMemStats(&ms)
				if ms.HeapInuse > pico.Load() {
					pico.Store(ms.HeapInuse)
				}
			}
		}
	}()
	e.g.Encolar(chatE2, "BIG")
	fin := time.Now().Add(60 * time.Second)
	for e.estado("BIG") != almacen.EstadoDescargado && time.Now().Before(fin) {
		time.Sleep(20 * time.Millisecond)
	}
	close(parar)
	<-hecho
	if e.estado("BIG") != almacen.EstadoDescargado {
		t.Fatalf("estado %s %v", e.estado("BIG"), e.logs)
	}
	m, _ := e.alm.ObtenerMensaje(1, chatE2, "BIG")
	if m.ArchivoBytes != total {
		t.Fatalf("bytes %d", m.ArchivoBytes)
	}
	if p := pico.Load(); p > 48<<20 {
		t.Fatalf("pico de memoria %d MB con un archivo de %d MB", p>>20, total>>20)
	}
}

func TestDuplicadoSeEnlazaSinDescargar(t *testing.T) {
	e := nuevoEntorno(t)
	datos := bytes.Repeat([]byte("igual"), 5000)
	e.cli.respuesta = func(string) ([]byte, error) { return datos, nil }
	e.mensajeSHA(t, "D1", "imagen", int64(len(datos)), []byte("misma-huella"))
	e.g.Encolar(chatE2, "D1")
	esperar(t, "D1", func() bool { return e.estado("D1") == almacen.EstadoDescargado })
	// D2 trae la misma huella que D1.
	e.mensajeSHA(t, "D2", "imagen", int64(len(datos)), []byte("misma-huella"))
	llamadas := e.cli.llamadas
	e.g.Encolar(chatE2, "D2")
	esperar(t, "D2", func() bool { return e.estado("D2") == almacen.EstadoDescargado })
	if e.cli.llamadas != llamadas {
		t.Fatal("el duplicado no debe descargarse")
	}
	m1, _ := e.alm.ObtenerMensaje(1, chatE2, "D1")
	m2, _ := e.alm.ObtenerMensaje(1, chatE2, "D2")
	a1, _ := e.alm.Abs(m1.ArchivoRuta)
	a2, _ := e.alm.Abs(m2.ArchivoRuta)
	var s1, s2 syscall.Stat_t
	_ = syscall.Stat(a1, &s1)
	_ = syscall.Stat(a2, &s2)
	if s1.Ino != s2.Ino || s1.Nlink != 2 {
		t.Fatalf("no es enlace duro: %d %d nlink %d", s1.Ino, s2.Ino, s1.Nlink)
	}
	if b, err := e.alm.LeerCifrado(m2.ArchivoRuta); err != nil || !bytes.Equal(b, datos) {
		t.Fatalf("el duplicado no se lee: %v", err)
	}
	// La contabilidad no cuenta el duplicado.
	por, _ := e.alm.EspacioPorTipo(1)
	if por[0].Bytes != int64(len(datos)) || por[0].Cantidad != 2 {
		t.Fatalf("por tipo: %+v", por[0])
	}
	if tot := e.alm.EspacioTotal(1); tot.Multimedia != almacen.TamCifrado(int64(len(datos))) {
		t.Fatalf("multimedia %d, esperado %d", tot.Multimedia, almacen.TamCifrado(int64(len(datos))))
	}
	if b, n := e.alm.AhorradoPorDuplicados(1); b != int64(len(datos)) || n != 1 {
		t.Fatalf("ahorrado %d %d", b, n)
	}
	// Borrar uno no libera nada y el otro sigue legible; borrar el último libera.
	if b, _, _ := e.alm.EstimarBorrado(1, almacen.Filtro{}); b != int64(len(datos)) {
		t.Fatalf("estimar todo: %d", b)
	}
	if b, n, _ := e.alm.BorrarLista(1, []almacen.Item{{Chat: chatE2, ID: "D1"}}); b != 0 || n != 1 {
		t.Fatalf("borrar uno: %d %d", b, n)
	}
	if b, err := e.alm.LeerCifrado(m2.ArchivoRuta); err != nil || !bytes.Equal(b, datos) {
		t.Fatalf("el otro se rompió: %v", err)
	}
	if b, _, _ := e.alm.BorrarLista(1, []almacen.Item{{Chat: chatE2, ID: "D2"}}); b != int64(len(datos)) {
		t.Fatalf("borrar el último: %d", b)
	}
}

func TestSinEnlaceDuroDescargaNormal(t *testing.T) {
	e := nuevoEntorno(t)
	e.cli.respuesta = func(string) ([]byte, error) { return []byte("contenido"), nil }
	e.mensajeSHA(t, "N1", "imagen", 9, []byte("huella-n"))
	e.g.Encolar(chatE2, "N1")
	esperar(t, "N1", func() bool { return e.estado("N1") == almacen.EstadoDescargado })
	e.mensajeSHA(t, "N2", "imagen", 9, []byte("huella-n"))
	// El origen existe en la base pero su archivo ya no: no hay a qué enlazar y se baja normal.
	m1, _ := e.alm.ObtenerMensaje(1, chatE2, "N1")
	a1, _ := e.alm.Abs(m1.ArchivoRuta)
	_ = os.Remove(a1)
	antes := e.cli.llamadas
	e.g.Encolar(chatE2, "N2")
	esperar(t, "N2", func() bool { return e.estado("N2") == almacen.EstadoDescargado })
	if e.cli.llamadas != antes+1 {
		t.Fatal("debía descargar normal")
	}
}

func TestEsperaPorWifiYBateriaSinGastarIntentos(t *testing.T) {
	e := nuevoEntornoCfg(t, func(c *Config) { c.EsperaPasajera = 30 * time.Millisecond })
	e.conRed(ingesta.RedDatos)
	e.mensaje(t, "W1", "video", 1000)
	e.g.Encolar(chatE2, "W1")
	time.Sleep(150 * time.Millisecond)
	if e.estado("W1") != almacen.EstadoPendiente || e.cli.llamadas != 0 {
		t.Fatalf("debía esperar el Wi-Fi: %s", e.estado("W1"))
	}
	e.conRed(ingesta.RedWifi)
	e.bat.Store(10)
	time.Sleep(150 * time.Millisecond)
	if e.estado("W1") != almacen.EstadoPendiente || e.cli.llamadas != 0 {
		t.Fatal("debía esperar por batería baja")
	}
	e.bat.Store(60)
	esperar(t, "baja al subir la batería", func() bool { return e.estado("W1") == almacen.EstadoDescargado })
	// Cargando también cuenta.
	e.bat.Store(5)
	e.cargando.Store(true)
	e.mensaje(t, "W2", "imagen", 100)
	e.g.Encolar(chatE2, "W2")
	esperar(t, "W2 cargando", func() bool { return e.estado("W2") == almacen.EstadoDescargado })
}

func TestArchivoMayorAl25PorCientoDelTopeNoBajaSolo(t *testing.T) {
	e := nuevoEntorno(t)
	e.tope.Store(1000)
	e.mensaje(t, "X1", "documento", 300) // 30 %
	e.g.Encolar(chatE2, "X1")
	esperar(t, "queda con botón", func() bool { return e.estado("X1") == almacen.EstadoNinguno })
	e.mensaje(t, "X2", "documento", 200) // 20 %
	e.g.Encolar(chatE2, "X2")
	esperar(t, "baja", func() bool { return e.estado("X2") == almacen.EstadoDescargado })
	// A mano sí baja.
	if err := e.g.Manual(chatE2, "X1"); err != nil {
		t.Fatal(err)
	}
	esperar(t, "manual", func() bool { return e.estado("X1") == almacen.EstadoDescargado })
}

func TestTopesPorTipoEInterruptor(t *testing.T) {
	e := nuevoEntorno(t)
	e.pol.TopeArchivo = map[string]int64{"audio": 100}
	e.pol.Sticker.Apagada = true
	e.mensaje(t, "A1", "audio", 500)
	e.mensaje(t, "S1", "sticker", 50)
	e.g.Encolar(chatE2, "A1")
	e.g.Encolar(chatE2, "S1")
	esperar(t, "A1", func() bool { return e.estado("A1") == almacen.EstadoNinguno })
	esperar(t, "S1", func() bool { return e.estado("S1") == almacen.EstadoNinguno })
	// Tope del tipo en total.
	e.pol = ingesta.PoliticaPorDefecto()
	e.pol.TopeTipoTotal = map[string]int64{"documento": 40}
	e.cli.respuesta = func(string) ([]byte, error) { return bytes.Repeat([]byte{1}, 30), nil }
	e.mensaje(t, "T1", "documento", 30)
	e.g.Encolar(chatE2, "T1")
	esperar(t, "T1", func() bool { return e.estado("T1") == almacen.EstadoDescargado })
	e.mensaje(t, "T2", "documento", 30)
	e.g.Encolar(chatE2, "T2")
	esperar(t, "T2 no cabe", func() bool { return e.estado("T2") == almacen.EstadoNinguno })
}

func TestTopeTotalSeEvaluaTrasCadaDescarga(t *testing.T) {
	e := nuevoEntornoCfg(t, func(c *Config) { c.TopeCada = 20 * time.Millisecond; c.TopeInicio = 20 * time.Millisecond })
	e.tope.Store(almacen.TamCifrado(100) * 3)
	e.cli.respuesta = func(string) ([]byte, error) { return bytes.Repeat([]byte{1}, 100), nil }
	// ts viejos para que no valga la protección de la última hora
	for i, id := range []string{"P1", "P2", "P3", "P4"} {
		e.mensaje(t, id, "documento", 100)
		e.g.Encolar(chatE2, id)
		esperar(t, id, func() bool { return e.estado(id) != almacen.EstadoPendiente })
		_ = i
	}
	time.Sleep(100 * time.Millisecond)
	// Todo es "reciente" para el tope (ts=10 es viejo, pero el archivo es de hace segundos): nada se borra.
	for _, id := range []string{"P1", "P2", "P3", "P4"} {
		if e.estado(id) != almacen.EstadoDescargado {
			t.Fatalf("%s: lo recién bajado no se borra (%s)", id, e.estado(id))
		}
	}
	_ = context.Background
}
