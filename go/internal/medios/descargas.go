// Package medios descarga y guarda cifrado el multimedia de los mensajes: máximo 2 descargas a la vez,
// una sola si el archivo es grande, con reintentos de espera creciente, fuera del bucle de eventos.
package medios

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
}

// Config reúne lo que necesita el gestor.
type Config struct {
	BotID int64
	Alm   *almacen.Almacen
	// Cliente devuelve el cliente conectado, o false si no hay conexión.
	Cliente  func() (Cliente, bool)
	Politica func() ingesta.Politica
	Libre    func() int64
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
	if cfg.Log == nil {
		cfg.Log = func(string) {}
	}
	return &Gestor{cfg: cfg, cola: make(chan item, 2048), activos: map[string]bool{}, intentos: map[string]int{},
		grande: make(chan struct{}, 1), timers: map[*time.Timer]struct{}{}}
}

// Iniciar arranca los trabajadores y reanuda las descargas que quedaron pendientes.
func (g *Gestor) Iniciar(ctx context.Context) {
	g.ctx, g.cancel = context.WithCancel(ctx)
	for i := 0; i < g.cfg.Trabajadores; i++ {
		g.wg.Add(1)
		go g.trabajador()
	}
	if pend, err := g.cfg.Alm.Pendientes(g.cfg.BotID); err == nil {
		for _, m := range pend {
			g.poner(item{chat: m.ChatJID, id: m.ID})
		}
	}
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
	if !it.manual {
		if p := g.cfg.Politica(); p.PisoLibre > 0 && g.cfg.Libre() > 0 && g.cfg.Libre() < p.PisoLibre {
			_ = alm.FijarEstado(bot, it.chat, it.id, almacen.EstadoNinguno) // poco espacio: queda con botón
			return
		}
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
	ctx, cancel := context.WithTimeout(g.ctx, 10*time.Minute)
	defer cancel()
	datos, err := cli.DownloadMediaWithPath(ctx, info.DirectPath, enc, sha, clave, mt, "", false)
	if err != nil {
		if g.ctx.Err() != nil {
			return // se está cerrando: queda pendiente y se reanuda al iniciar
		}
		g.falla(it, err, &reintentar)
		return
	}
	if !it.manual {
		p := g.cfg.Politica()
		tope := p.MaxBytes
		if almacen.CategoriaDe(m.Tipo) == almacen.CatVideo {
			tope = p.VideoMaxBytes
		}
		if tope > 0 && int64(len(datos)) > tope {
			_ = alm.FijarEstado(bot, it.chat, it.id, almacen.EstadoNinguno)
			return
		}
	}
	rel := almacen.RutaMedio(bot, it.chat, it.id)
	n, err := alm.EscribirCifradoDesde(rel, bytes.NewReader(datos))
	datos = nil
	if grande {
		debug.FreeOSMemory()
	}
	if err != nil {
		g.cfg.Log("no se pudo guardar un archivo: " + err.Error())
		g.falla(it, err, &reintentar)
		return
	}
	g.mu.Lock()
	delete(g.intentos, it.clave())
	g.mu.Unlock()
	if err := alm.ActualizarArchivo(bot, it.chat, it.id, almacen.EstadoDescargado, rel, n); err != nil {
		g.cfg.Log("no se pudo anotar un archivo: " + err.Error())
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
