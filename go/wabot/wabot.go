// Paquete wabot: conexión con WhatsApp (whatsmeow) para usar desde Android con gomobile.
// Solo usa tipos simples (string, bool, error, interfaz Listener).
package wabot

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite" // SQLite en Go puro (sin cgo)
)

// Listener recibe lo que pasa en el bot. Lo implementa la app Kotlin.
//
// Textos de OnStatus: "detenido", "sin_vincular", "esperando_codigo", "vinculado",
// "conectando", "conectado", "desconectado", "sesion_cerrada" y "error: <detalle>".
type Listener interface {
	OnStatus(texto string)
	OnPairingCode(codigo string)
	OnLog(linea string)
}

type estado struct {
	l    Listener
	cont *sqlstore.Container
	cli  *whatsmeow.Client
	lim  *limitador
	qr   chan struct{}
	cerr bool // true cuando el cierre lo pidió la app (Stop/Logout)
}

var (
	mu sync.Mutex
	st *estado
)

func (s *estado) status(t string) { s.l.OnStatus(t) }

// dsnSesion arma la dirección de la base de la sesión. En Android no existe /tmp: sin temp_store=MEMORY,
// SQLite falla con "disk I/O error (6410)" al necesitar un archivo temporal (se vio al sincronizar el estado de la app).
func dsnSesion(dataDir string) string {
	return "file:" + filepath.Join(dataDir, "sesion.db") +
		"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=temp_store(2)"
}

// Start abre (o crea) la sesión guardada en dataDir. Si ya está vinculada, se conecta sola.
func Start(dataDir string, l Listener) error {
	if l == nil {
		return errors.New("falta el listener")
	}
	mu.Lock()
	defer mu.Unlock()
	if st != nil {
		return errors.New("el bot ya está iniciado")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("no se pudo crear la carpeta: %w", err)
	}
	lg := &registro{l: l}
	ctx := context.Background()
	dsn := dsnSesion(dataDir)
	cont, err := sqlstore.New(ctx, "sqlite", dsn, lg.Sub("DB"))
	if err != nil {
		return fmt.Errorf("no se pudo abrir la base: %w", err)
	}
	dev, err := cont.GetFirstDevice(ctx)
	if err != nil {
		_ = cont.Close()
		return fmt.Errorf("no se pudo leer la sesión: %w", err)
	}
	cli := whatsmeow.NewClient(dev, lg.Sub("WA"))
	cli.EnableAutoReconnect = true
	s := &estado{l: l, cont: cont, cli: cli, lim: nuevoLimitador(), qr: make(chan struct{}, 1)}
	cli.AddEventHandler(s.evento)
	st = s
	if dev.ID == nil {
		l.OnLog("Sesión nueva: falta vincular.")
		s.status("sin_vincular")
		return nil
	}
	l.OnLog("Sesión encontrada. Conectando…")
	s.status("conectando")
	go func() {
		if err := cli.Connect(); err != nil {
			l.OnLog("No se pudo conectar: " + err.Error())
			s.status("error: " + err.Error())
		}
	}()
	return nil
}

// Pair pide a WhatsApp un código de 8 caracteres para vincular el número dado
// (con código de país, solo dígitos, sin "+", por ejemplo 525512345678).
// El código llega por Listener.OnPairingCode; los avisos y errores por OnStatus.
func Pair(telefono string) error {
	mu.Lock()
	s := st
	mu.Unlock()
	if s == nil {
		return errors.New("el bot no está iniciado")
	}
	if s.cli.Store.ID != nil {
		return errors.New("este bot ya está vinculado")
	}
	tel := soloDigitos(telefono)
	if len(tel) <= 6 {
		return errors.New("el número es muy corto")
	}
	if tel[0] == '0' {
		return errors.New("el número debe incluir el código de país")
	}
	go s.vincular(tel)
	return nil
}

func (s *estado) vincular(tel string) {
	select { // vacía avisos viejos de QR
	case <-s.qr:
	default:
	}
	if !s.cli.IsConnected() {
		s.l.OnLog("Conectando para pedir el código…")
		if err := s.cli.Connect(); err != nil {
			s.l.OnLog("No se pudo conectar: " + err.Error())
			s.status("error: " + err.Error())
			return
		}
		select { // WhatsApp avisa con un evento QR cuando ya se puede pedir el código
		case <-s.qr:
		case <-time.After(20 * time.Second):
			s.l.OnLog("WhatsApp tardó en responder; se intenta igual.")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	codigo, err := s.cli.PairPhone(ctx, tel, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		s.l.OnLog("WhatsApp rechazó la solicitud: " + err.Error())
		s.status("error: " + err.Error())
		return
	}
	s.l.OnLog("Código de vinculación generado para el número " + enmascara(tel) + ".")
	s.status("esperando_codigo")
	s.l.OnPairingCode(formatoCodigo(codigo))
}

// Stop desconecta y cierra la sesión local (la vinculación con WhatsApp se conserva).
func Stop() {
	mu.Lock()
	s := st
	st = nil
	mu.Unlock()
	if s == nil {
		return
	}
	s.cerr = true
	s.cli.Disconnect()
	_ = s.cont.Close()
	s.status("detenido")
}

// IsConnected dice si el bot está conectado y vinculado.
func IsConnected() bool {
	mu.Lock()
	s := st
	mu.Unlock()
	return s != nil && s.cli.IsConnected() && s.cli.IsLoggedIn()
}

// Logout quita el dispositivo vinculado en WhatsApp y borra la sesión local. Deja el bot detenido.
func Logout() error {
	mu.Lock()
	s := st
	mu.Unlock()
	if s == nil {
		return errors.New("el bot no está iniciado")
	}
	var aviso error
	if s.cli.Store.ID != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		err := s.cli.Logout(ctx)
		cancel()
		if err != nil {
			s.l.OnLog("No se pudo avisar a WhatsApp: " + err.Error())
			aviso = fmt.Errorf("no se pudo cerrar la sesión en WhatsApp (quita el dispositivo a mano en Dispositivos vinculados): %w", err)
			if derr := s.cli.Store.Delete(context.Background()); derr != nil {
				s.l.OnLog("Tampoco se pudo borrar la sesión local: " + derr.Error())
			}
		} else {
			s.l.OnLog("Sesión cerrada en WhatsApp.")
		}
	}
	Stop()
	return aviso
}

func (s *estado) evento(e any) {
	switch v := e.(type) {
	case *events.QR:
		select {
		case s.qr <- struct{}{}:
		default:
		}
	case *events.PairSuccess:
		s.l.OnLog("Vinculado con el número " + enmascara(v.ID.User) + ".")
		s.status("vinculado")
	case *events.PairError:
		s.l.OnLog("Falló la vinculación: " + v.Error.Error())
		s.status("error: falló la vinculación")
	case *events.Connected:
		s.l.OnLog("Conectado.")
		s.status("conectado")
	case *events.Disconnected:
		if !s.cerr {
			s.l.OnLog("Desconectado; se intentará reconectar.")
			s.status("desconectado")
		}
	case *events.LoggedOut:
		s.l.OnLog("WhatsApp cerró la sesión de este dispositivo.")
		s.status("sesion_cerrada")
	case *events.StreamReplaced:
		s.l.OnLog("La sesión se abrió en otro lugar.")
		s.status("error: la sesión se abrió en otro lugar")
	case *events.TemporaryBan:
		s.l.OnLog("Bloqueo temporal: " + v.String())
		s.status("error: bloqueo temporal de WhatsApp")
	case *events.ClientOutdated:
		s.l.OnLog("WhatsApp pide una versión más nueva de la conexión.")
		s.status("error: conexión desactualizada")
	case *events.ConnectFailure:
		s.l.OnLog(fmt.Sprintf("No se pudo conectar (código %d).", int(v.Reason)))
		s.status(fmt.Sprintf("error: no se pudo conectar (%d)", int(v.Reason)))
	case *events.Message:
		s.mensaje(v)
	}
}

func textoDe(m *waE2E.Message) string {
	if t := m.GetConversation(); t != "" {
		return t
	}
	return m.GetExtendedTextMessage().GetText()
}

func (s *estado) mensaje(v *events.Message) {
	chat := v.Info.Chat
	ent := entrada{
		EsMio:      v.Info.IsFromMe,
		EsGrupo:    v.Info.IsGroup || chat.Server == types.GroupServer,
		Edicion:    v.IsEdit || v.IsBotInvoke,
		Servidor:   chat.Server,
		Antiguedad: time.Since(v.Info.Timestamp),
		Texto:      textoDe(v.Message),
		Clave:      chat.String(),
	}
	ok, motivo := s.lim.decidir(ent, time.Now())
	if !ok {
		if !ent.EsMio {
			s.l.OnLog("Mensaje de " + enmascara(chat.User) + " ignorado: " + motivo + ".")
		}
		return
	}
	go func() {
		time.Sleep(time.Duration(1000+rand.Intn(2000)) * time.Millisecond) // pausa corta, como una persona
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := s.cli.SendMessage(ctx, chat, &waE2E.Message{Conversation: proto.String(textoRespuesta)})
		if err != nil {
			s.l.OnLog("No se pudo responder a " + enmascara(chat.User) + ": " + err.Error())
			return
		}
		s.l.OnLog("Respondido a " + enmascara(chat.User) + ".")
	}()
}

// registro lleva los mensajes internos de whatsmeow al Listener (sin datos sensibles: solo avisos y errores).
type registro struct {
	l   Listener
	mod string
}

func (r *registro) emite(nivel, msg string, a ...any) {
	if r.l == nil {
		return
	}
	r.l.OnLog(fmt.Sprintf("[%s %s] ", r.mod, nivel) + fmt.Sprintf(msg, a...))
}
func (r *registro) Warnf(m string, a ...any)  { r.emite("aviso", m, a...) }
func (r *registro) Errorf(m string, a ...any) { r.emite("error", m, a...) }
func (r *registro) Infof(m string, a ...any)  { r.emite("info", m, a...) }
func (r *registro) Debugf(string, ...any)     {}
func (r *registro) Sub(mod string) waLog.Logger {
	return &registro{l: r.l, mod: mod}
}
