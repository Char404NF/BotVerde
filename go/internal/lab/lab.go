// Package lab es el Laboratorio (v0.5.0, bloque G): menús como encuesta o botones, con respaldo SIEMPRE a texto numerado, y las pruebas
// que miden qué se ve y qué responde en cada versión de WhatsApp.
//
// Reglas del paquete:
//   - Solo usa lo que la librería hace por defecto (BuildPollCreation, ButtonsMessage, ListMessage, InteractiveMessage con quick_reply).
//     No se simula ser una cuenta Business, no se cambia el tipo de dispositivo y no se inventan nodos <biz>.
//   - Nada corre en el manejador de eventos: wabot solo encola y el trabajador llama a Recibir.
//   - Apagado por defecto. Hace falta el interruptor Y haber aceptado el aviso de riesgo.
package lab

import (
	"context"
	"crypto/sha256"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"botchat/internal/almacen"
	"botchat/internal/envio"
	"botchat/internal/motor"
)

// Modos del menú.
const (
	ModoTexto    = "texto"
	ModoEncuesta = "encuesta"
	ModoBotones  = "botones"
)

// Límites de texto que se usan al armar los mensajes (prudentes; el límite de opciones de la encuesta se MIDE en el Laboratorio).
const (
	maxNombreEncuesta = 255
	maxOpcionEncuesta = 100
	maxBoton          = 20
	maxFila           = 24
	maxBotones        = 3
	maxFilas          = 10
	// fallosParaVolver: envíos interactivos fallidos seguidos tras los que el menú vuelve solo a texto.
	fallosParaVolver = 3
)

// Errores.
var (
	ErrApagado    = errors.New("El Laboratorio está apagado.")
	ErrSinAviso   = errors.New("Falta aceptar el aviso de riesgo del Laboratorio.")
	ErrSinDestino = errors.New("Falta elegir el chat de prueba (tu otro número o un grupo de prueba).")
	ErrTope       = errors.New("Llegaste al tope de pruebas. Espera un rato o súbelo en los ajustes del Laboratorio.")
	ErrTipo       = errors.New("Esa prueba no existe.")
)

// Cliente es lo que se usa de whatsmeow (para poder probar sin red).
type Cliente interface {
	BuildPollCreation(name string, optionNames []string, selectableOptionCount int) *waE2E.Message
	DecryptPollVote(ctx context.Context, vote *events.Message) (*waE2E.PollVoteMessage, error)
	PutMessageSecret(ctx context.Context, chat, sender types.JID, id types.MessageID, secret []byte) error
	VersionCliente() string
}

// Real adapta un cliente de whatsmeow.
type Real struct{ *whatsmeow.Client }

// PutMessageSecret guarda el MessageSecret de un mensaje en el almacén de la librería.
func (r Real) PutMessageSecret(ctx context.Context, chat, sender types.JID, id types.MessageID, secret []byte) error {
	return r.Store.MsgSecrets.PutMessageSecret(ctx, chat, sender, id, secret)
}

// VersionCliente es la versión del cliente web de WhatsApp que usa la librería para conectarse.
func (r Real) VersionCliente() string { return store.GetWAVersion().String() }

// Emisor manda un mensaje interactivo guardándolo en el chat (lo implementa envio.Servicio).
type Emisor interface {
	EnviarInteractivo(ctx context.Context, chat, resumen, citaID, citaAutorWA string, armar func(id string, ci *waE2E.ContextInfo) *waE2E.Message,
		antes func(id string, m *waE2E.Message) error) (envio.Resultado, error)
}

// Servicio es el Laboratorio de un bot.
type Servicio struct {
	BotID int64
	Alm   *almacen.Almacen
	Cli   Cliente
	Env   Emisor
	Log   func(string)
	Ahora func() time.Time
	// AlVolverATexto se llama cuando el menú vuelve solo a texto por fallos seguidos.
	AlVolverATexto func(motivo string)

	mu     sync.Mutex
	fallos int
}

// Nuevo crea el Laboratorio.
func Nuevo(botID int64, alm *almacen.Almacen, cli Cliente, env Emisor, log func(string)) *Servicio {
	if log == nil {
		log = func(string) {}
	}
	return &Servicio{BotID: botID, Alm: alm, Cli: cli, Env: env, Log: log, Ahora: time.Now}
}

func (s *Servicio) ahora() time.Time {
	if s.Ahora != nil {
		return s.Ahora()
	}
	return time.Now()
}

func (s *Servicio) aj(k string) string { return s.Alm.AjusteBot(s.BotID, k) }
func (s *Servicio) ajInt(k string) int { return s.Alm.AjusteBotInt(s.BotID, k) }

// Habilitado dice si el Laboratorio está encendido y con el aviso de riesgo aceptado.
func (s *Servicio) Habilitado() error {
	if s.aj("lab_activo") != "1" {
		return ErrApagado
	}
	if s.ajInt("lab_aviso_ts") <= 0 {
		return ErrSinAviso
	}
	return nil
}

// Modo es el modo del menú que vale ahora: texto salvo que el Laboratorio esté habilitado y se haya elegido otro.
func (s *Servicio) Modo() string {
	if s.Habilitado() != nil {
		return ModoTexto
	}
	switch m := strings.TrimSpace(s.aj("lab_modo_menu")); m {
	case ModoEncuesta, ModoBotones:
		return m
	}
	return ModoTexto
}

// ---------- utilidades ----------

func truncar(t string, n int) string {
	t = strings.TrimSpace(t)
	r := []rune(t)
	if len(r) <= n {
		return t
	}
	if n <= 1 {
		return string(r[:n])
	}
	return strings.TrimSpace(string(r[:n-1])) + "…"
}

// unicos devuelve los nombres sin repetir (WhatsApp no admite dos opciones iguales): el repetido lleva " (2)", " (3)"…
func unicos(nombres []string) []string {
	visto := map[string]int{}
	out := make([]string, len(nombres))
	for i, n := range nombres {
		k := strings.ToLower(n)
		visto[k]++
		if c := visto[k]; c > 1 {
			n = n + " (" + strconv.Itoa(c) + ")"
		}
		out[i] = n
	}
	return out
}

func hashOpcion(nombre string) [32]byte { return sha256.Sum256([]byte(nombre)) }

func esGrupo(chat string) bool { return strings.HasSuffix(chat, "@g.us") }

func textoVolver(o motor.OpcionMenu, limite int) string {
	if o.Num == 0 {
		return truncar("Menú principal", limite)
	}
	return truncar(o.Texto, limite)
}

// ponerContexto pone la cita en el mensaje interactivo, según su tipo.
func ponerContexto(m *waE2E.Message, ci *waE2E.ContextInfo) {
	if ci == nil {
		return
	}
	switch {
	case m.PollCreationMessage != nil:
		m.PollCreationMessage.ContextInfo = ci
	case m.ButtonsMessage != nil:
		m.ButtonsMessage.ContextInfo = ci
	case m.ListMessage != nil:
		m.ListMessage.ContextInfo = ci
	case m.InteractiveMessage != nil:
		m.InteractiveMessage.ContextInfo = ci
	}
}
