// Package fotos baja las fotos de perfil de los chats con una cola lenta (una cada 3 segundos), cifradas en disco.
// Solo se piden para chats que ya tienen mensajes, nunca para números desconocidos, y se refrescan cada 7 días
// o cuando WhatsApp avisa que cambiaron.
package fotos

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"botchat/internal/almacen"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// Cliente es lo que se necesita de whatsmeow (la implementa *whatsmeow.Client).
type Cliente interface {
	GetProfilePictureInfo(ctx context.Context, jid types.JID, params *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error)
}

// Descargador baja la imagen desde su dirección.
type Descargador func(ctx context.Context, url string, maxBytes int64) ([]byte, error)

// Config reúne lo que necesita el gestor.
type Config struct {
	BotID   int64
	Alm     *almacen.Almacen
	Cliente func() (Cliente, bool)
	// Pausa entre una petición y la siguiente (3 s por defecto).
	Pausa time.Duration
	// Vigencia de una foto antes de volver a pedirla (7 días por defecto).
	Vigencia time.Duration
	Bajar    Descargador
	Ahora    func() time.Time
	Log      func(string)
}

// Gestor es la cola de fotos de perfil.
type Gestor struct {
	cfg     Config
	mu      sync.Mutex
	cola    []string
	en      map[string]bool
	visto   map[string]time.Time // última vez que se pidió, para no repetir cada mensaje
	aviso   chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	grandes sync.Mutex
}

// Nuevo crea el gestor. Hay que llamar a Iniciar.
func Nuevo(cfg Config) *Gestor {
	if cfg.Pausa <= 0 {
		cfg.Pausa = 3 * time.Second
	}
	if cfg.Vigencia <= 0 {
		cfg.Vigencia = 7 * 24 * time.Hour
	}
	if cfg.Bajar == nil {
		cfg.Bajar = descargaHTTP
	}
	if cfg.Ahora == nil {
		cfg.Ahora = time.Now
	}
	if cfg.Log == nil {
		cfg.Log = func(string) {}
	}
	return &Gestor{cfg: cfg, en: map[string]bool{}, visto: map[string]time.Time{}, aviso: make(chan struct{}, 1)}
}

// Iniciar arranca el trabajador.
func (g *Gestor) Iniciar(ctx context.Context) {
	g.ctx, g.cancel = context.WithCancel(ctx)
	g.wg.Add(1)
	go g.trabajador()
}

// Detener espera a que termine el trabajador.
func (g *Gestor) Detener() {
	if g.cancel != nil {
		g.cancel()
	}
	g.wg.Wait()
}

// Pedir pide la foto de un chat si nunca se pidió o ya venció. Es barato: se puede llamar con cada mensaje.
func (g *Gestor) Pedir(chat string) { g.poner(chat, false) }

// Forzar pide la foto aunque no haya vencido (WhatsApp avisó que cambió).
func (g *Gestor) Forzar(chat string) { g.poner(chat, true) }

// RevisarVencidas pone en cola los chats con mensajes cuya foto venció o nunca se pidió.
func (g *Gestor) RevisarVencidas() {
	limite := g.cfg.Ahora().Add(-g.cfg.Vigencia).Unix()
	chats, err := g.cfg.Alm.ChatsParaFoto(g.cfg.BotID, limite, 300)
	if err != nil {
		return
	}
	for _, c := range chats {
		g.poner(c, false)
	}
}

func (g *Gestor) poner(chat string, forzar bool) {
	alm := g.cfg.Alm
	if !alm.TieneMensajes(g.cfg.BotID, chat) { // solo chats con mensajes: nunca números desconocidos
		return
	}
	ahora := g.cfg.Ahora()
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.en[chat] {
		return
	}
	if !forzar {
		if t, ok := g.visto[chat]; ok && ahora.Sub(t) < time.Hour {
			return
		}
		c, err := alm.ObtenerChat(g.cfg.BotID, chat)
		if err != nil || c == nil {
			return
		}
		if c.FotoTS > 0 && ahora.Sub(time.Unix(c.FotoTS, 0)) < g.cfg.Vigencia {
			return
		}
	}
	g.visto[chat] = ahora
	g.en[chat] = true
	g.cola = append(g.cola, chat)
	select {
	case g.aviso <- struct{}{}:
	default:
	}
}

func (g *Gestor) siguiente() (string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.cola) == 0 {
		return "", false
	}
	c := g.cola[0]
	g.cola = g.cola[1:]
	return c, true
}

func (g *Gestor) trabajador() {
	defer g.wg.Done()
	for {
		chat, ok := g.siguiente()
		if !ok {
			select {
			case <-g.ctx.Done():
				return
			case <-g.aviso:
			}
			continue
		}
		g.procesar(chat)
		g.mu.Lock()
		delete(g.en, chat)
		g.mu.Unlock()
		select { // una petición cada Pausa
		case <-g.ctx.Done():
			return
		case <-time.After(g.cfg.Pausa):
		}
	}
}

// PedirPersona pide la foto de una persona que quizá no tiene chat privado (por ejemplo, solo es miembro de un grupo). jid es la
// forma con la que se pide; grupoComun (opcional) es un grupo que compartimos: mejora el éxito con quien no es contacto. Usa la misma
// cola lenta (una petición cada 3 s). forzar salta la vigencia (al abrir el perfil de la persona).
func (g *Gestor) PedirPersona(persona, jid, grupoComun string, forzar bool) {
	if persona == "" || jid == "" {
		return
	}
	alm := g.cfg.Alm
	ahora := g.cfg.Ahora()
	clave := "p|" + persona + "|" + jid + "|" + grupoComun
	if !forzar {
		if _, _, ts := alm.FotoPersonaInfo(g.cfg.BotID, persona); ts > 0 && ahora.Sub(time.Unix(ts, 0)) < g.cfg.Vigencia {
			return
		}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.en[clave] {
		return
	}
	if !forzar {
		if t, ok := g.visto[clave]; ok && ahora.Sub(t) < time.Hour {
			return
		}
	}
	g.visto[clave] = ahora
	g.en[clave] = true
	g.cola = append(g.cola, clave)
	select {
	case g.aviso <- struct{}{}:
	default:
	}
}

func (g *Gestor) procesarPersona(clave string) {
	partes := strings.SplitN(clave, "|", 4)
	if len(partes) != 4 {
		return
	}
	persona, jidS, grupoS := partes[1], partes[2], partes[3]
	olvida := func() {
		g.mu.Lock()
		delete(g.visto, clave)
		g.mu.Unlock()
	}
	cli, ok := g.cfg.Cliente()
	if !ok {
		olvida()
		return
	}
	jid, err := types.ParseJID(jidS)
	if err != nil {
		return
	}
	params := &whatsmeow.GetProfilePictureParams{Preview: true}
	if grupoS != "" {
		if gj, e := types.ParseJID(grupoS); e == nil {
			params.CommonGID = gj
		}
	}
	bot, alm := g.cfg.BotID, g.cfg.Alm
	rutaAnt, idAnt, _ := alm.FotoPersonaInfo(bot, persona)
	params.ExistingID = idAnt
	ctx, cancel := context.WithTimeout(g.ctx, 30*time.Second)
	defer cancel()
	info, err := cli.GetProfilePictureInfo(ctx, jid, params)
	ahora := g.cfg.Ahora().Unix()
	switch {
	case errors.Is(err, whatsmeow.ErrProfilePictureNotSet), errors.Is(err, whatsmeow.ErrProfilePictureUnauthorized):
		_ = alm.FijarFotoPersona(bot, persona, "", "", ahora)
		return
	case err != nil:
		g.cfg.Log("no se pudo pedir la foto de una persona")
		olvida()
		return
	case info == nil: // no cambió
		_ = alm.FijarFotoPersona(bot, persona, rutaAnt, idAnt, ahora)
		return
	}
	datos, err := g.bajar()(ctx, info.URL, 3<<20)
	if err != nil || len(datos) == 0 {
		g.cfg.Log("no se pudo bajar la foto de una persona")
		olvida()
		return
	}
	rel := almacen.RutaFotoPersona(bot, persona)
	if err := alm.EscribirCifrado(rel, datos); err != nil {
		g.cfg.Log("no se pudo guardar la foto de una persona")
		return
	}
	_ = alm.FijarFotoPersona(bot, persona, rel, info.ID, ahora)
}

func (g *Gestor) procesar(chat string) {
	if strings.HasPrefix(chat, "p|") {
		defer func() {
			if r := recover(); r != nil {
				g.cfg.Log(fmt.Sprintf("error interno con una foto de persona: %v", r))
			}
		}()
		g.procesarPersona(chat)
		return
	}
	defer func() {
		if r := recover(); r != nil {
			g.cfg.Log(fmt.Sprintf("error interno con una foto de perfil: %v", r))
		}
	}()
	cli, ok := g.cfg.Cliente()
	if !ok {
		g.mu.Lock()
		delete(g.visto, chat) // sin conexión: se volverá a pedir
		g.mu.Unlock()
		return
	}
	jid, err := types.ParseJID(chat)
	if err != nil {
		return
	}
	bot, alm := g.cfg.BotID, g.cfg.Alm
	c, _ := alm.ObtenerChat(bot, chat)
	existente := ""
	if c != nil {
		existente = c.FotoID
	}
	ctx, cancel := context.WithTimeout(g.ctx, 30*time.Second)
	defer cancel()
	info, err := cli.GetProfilePictureInfo(ctx, jid, &whatsmeow.GetProfilePictureParams{Preview: true, ExistingID: existente})
	ahora := g.cfg.Ahora().Unix()
	switch {
	case errors.Is(err, whatsmeow.ErrProfilePictureNotSet), errors.Is(err, whatsmeow.ErrProfilePictureUnauthorized):
		// Sin foto o no autorizado: es normal. Se anota que ya se preguntó y se vuelve a ver en 7 días.
		_ = alm.FijarFoto(bot, chat, "", "", ahora)
		return
	case err != nil:
		g.cfg.Log("no se pudo pedir una foto de perfil")
		g.mu.Lock()
		delete(g.visto, chat)
		g.mu.Unlock()
		return
	case info == nil: // no cambió
		if c != nil {
			_ = alm.FijarFoto(bot, chat, c.FotoRuta, c.FotoID, ahora)
		}
		return
	}
	datos, err := g.bajar()(ctx, info.URL, 3<<20)
	if err != nil || len(datos) == 0 {
		g.cfg.Log("no se pudo bajar una foto de perfil")
		g.mu.Lock()
		delete(g.visto, chat)
		g.mu.Unlock()
		return
	}
	rel := almacen.RutaFoto(bot, chat, false)
	if err := alm.EscribirCifrado(rel, datos); err != nil {
		g.cfg.Log("no se pudo guardar una foto de perfil")
		return
	}
	_ = alm.FijarFoto(bot, chat, rel, info.ID, ahora)
	// La foto grande anterior ya no corresponde.
	if abs, err := alm.Abs(almacen.RutaFoto(bot, chat, true)); err == nil {
		_ = os.Remove(abs)
	}
}

// Grande trae la foto completa de un chat (al tocar el encabezado). Devuelve la ruta relativa del archivo cifrado.
func (g *Gestor) Grande(ctx context.Context, chat string) (string, error) {
	g.grandes.Lock()
	defer g.grandes.Unlock()
	bot, alm := g.cfg.BotID, g.cfg.Alm
	rel := almacen.RutaFoto(bot, chat, true)
	if abs, err := alm.Abs(rel); err == nil {
		if i, err := os.Stat(abs); err == nil && g.cfg.Ahora().Sub(i.ModTime()) < g.cfg.Vigencia {
			return rel, nil
		}
	}
	cli, ok := g.cfg.Cliente()
	if !ok {
		return "", errors.New("sin conexión")
	}
	jid, err := types.ParseJID(chat)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	info, err := cli.GetProfilePictureInfo(ctx, jid, &whatsmeow.GetProfilePictureParams{Preview: false})
	switch {
	case errors.Is(err, whatsmeow.ErrProfilePictureNotSet):
		return "", errors.New("este chat no tiene foto")
	case errors.Is(err, whatsmeow.ErrProfilePictureUnauthorized):
		return "", errors.New("la foto no está disponible para ti")
	case err != nil:
		return "", errors.New("no se pudo pedir la foto")
	case info == nil || info.URL == "":
		return "", errors.New("este chat no tiene foto")
	}
	datos, err := g.bajar()(ctx, info.URL, 8<<20)
	if err != nil || len(datos) == 0 {
		return "", errors.New("no se pudo bajar la foto")
	}
	if err := alm.EscribirCifrado(rel, datos); err != nil {
		return "", err
	}
	return rel, nil
}

func descargaHTTP(ctx context.Context, url string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("respuesta %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBytes))
}

// fijaBajar cambia el descargador (para pruebas).
func (g *Gestor) fijaBajar(f Descargador) {
	g.mu.Lock()
	g.cfg.Bajar = f
	g.mu.Unlock()
}

func (g *Gestor) bajar() Descargador {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.cfg.Bajar
}
