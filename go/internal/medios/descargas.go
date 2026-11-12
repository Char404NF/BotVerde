// Package medios descarga y guarda cifrado el multimedia de los mensajes: máximo 2 descargas a la vez,
// una sola si el archivo es grande, con reintentos de espera creciente, fuera del bucle de eventos.
package medios

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/ingesta"

	"go.mau.fi/whatsmeow"
)

// Cliente es lo que se necesita de whatsmeow para bajar un archivo (la implementa *whatsmeow.Client).
type Cliente interface {
	DownloadMediaWithPath(ctx context.Context, directPath string, encFileHash, fileHash, mediaKey []byte, mediaType whatsmeow.MediaType, mmsType string, allowNoHash bool) ([]byte, error)
	// DownloadMediaWithPathToFile baja a un archivo (para lo grande: no pasa por la RAM).
	DownloadMediaWithPathToFile(ctx context.Context, directPath string, encFileHash, fileHash, mediaKey []byte, mediaType whatsmeow.MediaType, mmsType string, allowNoHash bool, file whatsmeow.File) error
}

// Config reúne lo que necesita el gestor.
type Config struct {
	BotID int64
	Alm   *almacen.Almacen
	// Cliente devuelve el cliente conectado, o false si no hay conexión.
	Cliente  func() (Cliente, bool)
	Politica func() ingesta.Politica
	Libre    func() int64
	// Red, Bateria y TopeTotal son opcionales (nil = red desconocida, batería desconocida, sin tope total).
	Red       func() ingesta.Red
	Bateria   func() ingesta.Bateria
	TopeTotal func() int64
	// UmbralArchivo: por encima de esto la descarga va a un archivo temporal en vez de la RAM (por defecto 64 MB).
	UmbralArchivo int64
	// EsperaPasajera es lo que espera una descarga detenida por red o batería antes de volver a mirar (por defecto 2 min).
	EsperaPasajera time.Duration
	// TopeCada es el mínimo entre dos corridas del tope total (por defecto 1 min) y TopeInicio la espera tras arrancar (90 s).
	TopeCada, TopeInicio time.Duration
	// Esperas son los tiempos entre reintentos (por defecto 10 s, 1 min y 5 min); después de agotarlos el archivo queda "fallido".
	Esperas []time.Duration
	// Trabajadores es el máximo de descargas a la vez (por defecto 2).
	Trabajadores int
	// UmbralGrande: los archivos de más de esto bajan de uno en uno (por defecto 8 MB).
	UmbralGrande int64
	Log          func(string)
}

type item struct {
	chat, id string
	manual   bool
}

func (i item) clave() string { return i.chat + "\x00" + i.id }

// Gestor es la cola de descargas.
type Gestor struct {
	cfg      Config
	cola     chan item
	mu       sync.Mutex
	activos  map[string]bool // en cola o descargando
	intentos map[string]int
	grande   chan struct{}
	wg       sync.WaitGroup
	cancel   context.CancelFunc
	ctx      context.Context
	timers   map[*time.Timer]struct{}
	// tope total
	topeMu    sync.Mutex
	topeUlt   time.Time
	topeCorre bool
	topeCita  bool
	topeWg    sync.WaitGroup
}

// Nuevo crea el gestor. Hay que llamar a Iniciar.
func Nuevo(cfg Config) *Gestor {
	if len(cfg.Esperas) == 0 {
		cfg.Esperas = []time.Duration{10 * time.Second, time.Minute, 5 * time.Minute}
	}
	if cfg.Trabajadores <= 0 {
		cfg.Trabajadores = 2
	}
	if cfg.UmbralGrande <= 0 {
		cfg.UmbralGrande = 8 << 20
	}
	if cfg.UmbralArchivo <= 0 {
		cfg.UmbralArchivo = 64 << 20
	}
	if cfg.EsperaPasajera <= 0 {
		cfg.EsperaPasajera = 2 * time.Minute
	}
	if cfg.TopeCada <= 0 {
		cfg.TopeCada = time.Minute
	}
	if cfg.TopeInicio <= 0 {
		cfg.TopeInicio = 90 * time.Second
	}
	if cfg.Log == nil {
		cfg.Log = func(string) {}
	}
	return &Gestor{cfg: cfg, cola: make(chan item, 2048), activos: map[string]bool{}, intentos: map[string]int{},
		grande: make(chan struct{}, 1), timers: map[*time.Timer]struct{}{}}
}

// Iniciar arranca los trabajadores y reanuda las descargas que quedaron pendientes.
func (g *Gestor) Iniciar(ctx context.Context) {
	g.ctx, g.cancel = context.WithCancel(ctx)
	g.limpiarTemporales()
	for i := 0; i < g.cfg.Trabajadores; i++ {
		g.wg.Add(1)
		go g.trabajador()
	}
	if pend, err := g.cfg.Alm.Pendientes(g.cfg.BotID); err == nil {
		for _, m := range pend {
			g.poner(item{chat: m.ChatJID, id: m.ID})
		}
	}
	g.pedirTope(g.cfg.TopeInicio)
}

// Detener cancela lo que está en curso y espera a que terminen los trabajadores.
func (g *Gestor) Detener() {
	if g.cancel != nil {
		g.cancel()
	}
	g.mu.Lock()
	for t := range g.timers {
		t.Stop()
	}
	g.timers = map[*time.Timer]struct{}{}
	g.mu.Unlock()
	g.wg.Wait()
	g.topeWg.Wait()
}

// Encolar pone en la cola una descarga automática (el mensaje ya está en estado "pendiente").
func (g *Gestor) Encolar(chat, id string) { g.poner(item{chat: chat, id: id}) }

// Manual atiende el botón "Descargar": ignora la política automática y reintenta aunque haya fallado.
func (g *Gestor) Manual(chat, id string) error {
	m, err := g.cfg.Alm.ObtenerMensaje(g.cfg.BotID, chat, id)
	if err != nil {
		return err
	}
	if m == nil || almacen.CategoriaDe(m.Tipo) == "" || len(m.ArchivoInfo) == 0 {
		return errors.New("este mensaje no tiene un archivo que descargar")
	}
	switch m.ArchivoEstado {
	case almacen.EstadoDescargado, almacen.EstadoPendiente:
		return nil
	}
	g.mu.Lock()
	delete(g.intentos, item{chat: chat, id: id}.clave())
	g.mu.Unlock()
	if err := g.cfg.Alm.FijarEstado(g.cfg.BotID, chat, id, almacen.EstadoPendiente); err != nil {
		return err
	}
	g.poner(item{chat: chat, id: id, manual: true})
	return nil
}

func (g *Gestor) poner(it item) {
	g.mu.Lock()
	if g.activos[it.clave()] {
		g.mu.Unlock()
		return
	}
	g.activos[it.clave()] = true
	g.mu.Unlock()
	select {
	case g.cola <- it:
	default: // cola llena: queda con el botón
		g.liberar(it)
		_ = g.cfg.Alm.FijarEstado(g.cfg.BotID, it.chat, it.id, almacen.EstadoNinguno)
	}
}

func (g *Gestor) liberar(it item) {
	g.mu.Lock()
	delete(g.activos, it.clave())
	g.mu.Unlock()
}

func (g *Gestor) trabajador() {
	defer g.wg.Done()
	for {
		select {
		case <-g.ctx.Done():
			return
		case it := <-g.cola:
			g.procesar(it)
		}
	}
}

var tipoMedio = map[string]whatsmeow.MediaType{
	"imagen": whatsmeow.MediaImage, "sticker": whatsmeow.MediaImage, "video": whatsmeow.MediaVideo,
	"audio": whatsmeow.MediaAudio, "documento": whatsmeow.MediaDocument,
}

func (g *Gestor) entorno() ingesta.Entorno {
	e := ingesta.Entorno{Libre: g.cfg.Libre(), Bateria: ingesta.Bateria{Nivel: -1}}
	if g.cfg.Red != nil {
		e.Red = g.cfg.Red()
	}
	if g.cfg.Bateria != nil {
		e.Bateria = g.cfg.Bateria()
	}
	if g.cfg.TopeTotal != nil {
		e.TopeTotal = g.cfg.TopeTotal()
	}
	e.UsoTipo = func(k string) int64 { return g.cfg.Alm.UsoDeTipo(g.cfg.BotID, k) }
	return e
}

// dirTmp es la carpeta de los temporales de descarga (en claro, solo mientras baja el archivo grande).
func (g *Gestor) dirTmp() string { return filepath.Join(g.cfg.Alm.Raiz(), "tmp", "descargas") }

// limpiarTemporales borra lo que haya quedado de una descarga cortada.
func (g *Gestor) limpiarTemporales() { _ = os.RemoveAll(g.dirTmp()) }

func (g *Gestor) procesar(it item) {
	reintentar := false
	defer func() {
		if r := recover(); r != nil {
			g.cfg.Log(fmt.Sprintf("error interno al descargar: %v", r))
			_ = g.cfg.Alm.FijarEstado(g.cfg.BotID, it.chat, it.id, almacen.EstadoFallido)
		}
		if !reintentar {
			g.liberar(it)
		}
	}()
	alm, bot := g.cfg.Alm, g.cfg.BotID
	m, err := alm.ObtenerMensaje(bot, it.chat, it.id)
	if err != nil || m == nil || m.ArchivoEstado != almacen.EstadoPendiente {
		return
	}
	if m.Tipo == almacen.TipoVerUnaVez {
		it.manual = true // un "ver una vez" siempre se baja al llegar, sin importar las reglas de descarga automática
	}
	info, err := ingesta.ParseInfo(m.ArchivoInfo)
	mt, ok := tipoMedio[info.Tipo]
	if err != nil || !ok {
		_ = alm.FijarEstado(bot, it.chat, it.id, almacen.EstadoFallido)
		return
	}
	clave, sha, enc, err := info.Bytes()
	if err != nil {
		_ = alm.FijarEstado(bot, it.chat, it.id, almacen.EstadoFallido)
		return
	}
	tipo := ingesta.ClaveTipo(m.Tipo)
	rel := almacen.RutaMedio(bot, it.chat, it.id)

	// Duplicado: si ya se bajó el mismo archivo, se enlaza (cero bytes y cero red). Si el sistema no deja, se baja normal.
	if m.Tipo != almacen.TipoVerUnaVez && len(sha) > 0 {
		if orig, bytes, ok := alm.Duplicado(bot, sha, it.chat, it.id); ok && alm.EnlazarDuplicado(orig, rel) {
			if err := alm.ActualizarArchivo(bot, it.chat, it.id, almacen.EstadoDescargado, rel, bytes); err == nil {
				g.mu.Lock()
				delete(g.intentos, it.clave())
				g.mu.Unlock()
				return
			}
		}
	}
	if !it.manual {
		ok, motivo := g.cfg.Politica().Evaluar(tipo, m.ArchivoBytes, g.entorno())
		if !ok {
			if ingesta.Pasajero(motivo) {
				reintentar = true // red o batería: espera y vuelve a mirar sin gastar un intento
				g.despues(it, g.cfg.EsperaPasajera)
				return
			}
			_ = alm.FijarEstado(bot, it.chat, it.id, almacen.EstadoNinguno) // queda con botón
			return
		}
	}
	cli, conectado := g.cfg.Cliente()
	if !conectado {
		// Sin conexión no se gasta un intento: se vuelve a ver pronto.
		reintentar = true
		g.despues(it, 15*time.Second)
		return
	}
	grande := m.ArchivoBytes > g.cfg.UmbralGrande
	if grande {
		select {
		case g.grande <- struct{}{}:
			defer func() { <-g.grande }()
		case <-g.ctx.Done():
			return
		}
	}
	aArchivo := m.ArchivoBytes > g.cfg.UmbralArchivo ||
		(m.ArchivoBytes <= 0 && (m.Tipo == almacen.TipoVideo || m.Tipo == almacen.TipoDocumento))
	if aArchivo {
		// El temporal en claro más el cifrado final: hace falta lugar para los dos.
		if libre := g.cfg.Libre(); libre > 0 && m.ArchivoBytes > 0 && libre < 2*m.ArchivoBytes+(64<<20) {
			g.cfg.Log("no hay espacio para bajar un archivo grande")
			_ = alm.FijarEstado(bot, it.chat, it.id, almacen.EstadoNinguno)
			return
		}
	}
	espera := 10 * time.Minute
	if m.ArchivoBytes > 0 {
		espera += time.Duration(m.ArchivoBytes/(100<<10)) * time.Second // al menos ~100 KB/s
		if espera > 3*time.Hour {
			espera = 3 * time.Hour
		}
	}
	ctx, cancel := context.WithTimeout(g.ctx, espera)
	defer cancel()

	var n int64
	if aArchivo {
		n, err = g.bajarAArchivo(ctx, cli, info, enc, sha, clave, mt, rel, it)
	} else {
		var datos []byte
		datos, err = cli.DownloadMediaWithPath(ctx, info.DirectPath, enc, sha, clave, mt, "", false)
		if err == nil {
			if g.sobrepasa(it, tipo, int64(len(datos))) {
				_ = alm.FijarEstado(bot, it.chat, it.id, almacen.EstadoNinguno)
				return
			}
			n, err = alm.EscribirCifradoDesde(rel, bytes.NewReader(datos))
			datos = nil
			if grande {
				debug.FreeOSMemory()
			}
		}
	}
	if errors.Is(err, errSobrepasa) {
		_ = alm.FijarEstado(bot, it.chat, it.id, almacen.EstadoNinguno)
		return
	}
	if err != nil {
		if g.ctx.Err() != nil {
			return // se está cerrando: queda pendiente y se reanuda al iniciar
		}
		g.falla(it, err, &reintentar)
		return
	}
	g.mu.Lock()
	delete(g.intentos, it.clave())
	g.mu.Unlock()
	if err := alm.ActualizarArchivo(bot, it.chat, it.id, almacen.EstadoDescargado, rel, n); err != nil {
		g.cfg.Log("no se pudo anotar un archivo: " + err.Error())
		return
	}
	if m.Tipo != almacen.TipoVerUnaVez {
		_ = alm.FijarSHASiFalta(bot, it.chat, it.id, sha)
	}
	g.pedirTope(0)
}

var errSobrepasa = errors.New("sobrepasa un tope")

// sobrepasa dice si el tamaño real (ya conocido) rompe un tope; solo cuenta en descargas automáticas.
func (g *Gestor) sobrepasa(it item, tipo string, n int64) bool {
	if it.manual {
		return false
	}
	ok, motivo := g.cfg.Politica().Evaluar(tipo, n, g.entorno())
	return !ok && (motivo == ingesta.MotivoGrande || motivo == ingesta.MotivoTopeTotal || motivo == ingesta.MotivoTopeTipo)
}

// bajarAArchivo baja un archivo grande a un temporal (sin pasar por la RAM), lo cifra en flujo y borra el temporal.
func (g *Gestor) bajarAArchivo(ctx context.Context, cli Cliente, info ingesta.InfoDescarga, enc, sha, clave []byte, mt whatsmeow.MediaType, rel string, it item) (int64, error) {
	if err := os.MkdirAll(g.dirTmp(), 0o700); err != nil {
		return 0, err
	}
	f, err := os.CreateTemp(g.dirTmp(), "dl-*.part")
	if err != nil {
		return 0, err
	}
	defer func() { f.Close(); os.Remove(f.Name()) }()
	if err := cli.DownloadMediaWithPathToFile(ctx, info.DirectPath, enc, sha, clave, mt, "", false, f); err != nil {
		return 0, err
	}
	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	if g.sobrepasa(it, ingesta.ClaveTipo(tipoDe(info.Tipo)), st.Size()) {
		return 0, errSobrepasa
	}
	if _, err := f.Seek(0, 0); err != nil {
		return 0, err
	}
	n, err := g.cfg.Alm.EscribirCifradoDesde(rel, f)
	if err != nil {
		g.cfg.Log("no se pudo guardar un archivo: " + err.Error())
	}
	return n, err
}

func tipoDe(t string) string { return t } // los tipos de InfoDescarga ("imagen", "video"...) son los mismos del mensaje

// ---------- tope total ----------

// PedirTope pide revisar el tope total pronto (por ejemplo, tras cambiarlo).
func (g *Gestor) PedirTope() { g.pedirTope(0) }

// pedirTope pide revisar el tope total (con `espera` antes); entre dos corridas pasa al menos TopeCada.
func (g *Gestor) pedirTope(espera time.Duration) {
	if g.cfg.TopeTotal == nil || g.ctx == nil || g.ctx.Err() != nil {
		return
	}
	g.topeMu.Lock()
	if g.topeCita {
		g.topeMu.Unlock()
		return
	}
	d := espera
	if desde := time.Since(g.topeUlt); desde < g.cfg.TopeCada && g.cfg.TopeCada-desde > d {
		d = g.cfg.TopeCada - desde
	}
	g.topeCita = true
	g.topeMu.Unlock()
	g.mu.Lock()
	var t *time.Timer
	t = time.AfterFunc(d, func() {
		g.mu.Lock() // espera a que `t` esté asignado
		delete(g.timers, t)
		if g.ctx.Err() != nil {
			g.mu.Unlock()
			return
		}
		g.topeWg.Add(1)
		g.mu.Unlock()
		defer g.topeWg.Done()
		g.correrTope()
	})
	g.timers[t] = struct{}{}
	g.mu.Unlock()
}

func (g *Gestor) correrTope() {
	g.topeMu.Lock()
	g.topeCita = false
	if g.topeCorre || g.ctx.Err() != nil {
		g.topeMu.Unlock()
		return
	}
	g.topeCorre = true
	g.topeUlt = time.Now()
	g.topeMu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			g.cfg.Log(fmt.Sprintf("error interno en el tope: %v", r))
		}
		g.topeMu.Lock()
		g.topeCorre = false
		g.topeMu.Unlock()
	}()
	r, err := g.cfg.Alm.AplicarTope(g.cfg.BotID, g.cfg.TopeTotal(), time.Now())
	if err != nil {
		g.cfg.Log("tope total: " + err.Error())
	} else if r.Borrados > 0 {
		g.cfg.Log(fmt.Sprintf("tope total: se borraron %d archivos viejos", r.Borrados))
	}
}

// falla decide entre reintentar con espera creciente o dejar el archivo como fallido.
func (g *Gestor) falla(it item, err error, reintentar *bool) {
	if esPermanente(err) {
		g.cfg.Log("archivo no disponible: " + resumen(err))
		_ = g.cfg.Alm.FijarEstado(g.cfg.BotID, it.chat, it.id, almacen.EstadoFallido)
		return
	}
	g.mu.Lock()
	n := g.intentos[it.clave()]
	g.intentos[it.clave()] = n + 1
	g.mu.Unlock()
	if n >= len(g.cfg.Esperas) {
		g.cfg.Log("descarga fallida tras varios intentos: " + resumen(err))
		_ = g.cfg.Alm.FijarEstado(g.cfg.BotID, it.chat, it.id, almacen.EstadoFallido)
		g.mu.Lock()
		delete(g.intentos, it.clave())
		g.mu.Unlock()
		return
	}
	g.cfg.Log(fmt.Sprintf("descarga fallida (intento %d), se reintenta: %s", n+1, resumen(err)))
	*reintentar = true
	g.despues(it, g.cfg.Esperas[n])
}

func (g *Gestor) despues(it item, d time.Duration) {
	var t *time.Timer
	t = time.AfterFunc(d, func() {
		g.mu.Lock()
		delete(g.timers, t)
		g.mu.Unlock()
		if g.ctx.Err() != nil {
			return
		}
		select {
		case g.cola <- it:
		default:
			g.liberar(it)
		}
	})
	g.mu.Lock()
	g.timers[t] = struct{}{}
	g.mu.Unlock()
}

// esPermanente: el archivo ya no existe o no es legible; reintentar no sirve.
func esPermanente(err error) bool {
	for _, e := range []error{whatsmeow.ErrMediaDownloadFailedWith403, whatsmeow.ErrMediaDownloadFailedWith404, whatsmeow.ErrMediaDownloadFailedWith410,
		whatsmeow.ErrNoURLPresent, whatsmeow.ErrInvalidMediaSHA256, whatsmeow.ErrInvalidMediaEncSHA256, whatsmeow.ErrInvalidMediaHMAC, whatsmeow.ErrFileLengthMismatch} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

// resumen deja el error sin datos personales (solo el tipo de problema).
func resumen(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "tiempo agotado"
	case errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith403):
		return "403"
	case errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith404):
		return "404"
	case errors.Is(err, whatsmeow.ErrMediaDownloadFailedWith410):
		return "410 (expiró)"
	}
	return fmt.Sprintf("%T", err)
}
