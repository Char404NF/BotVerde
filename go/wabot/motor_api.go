package wabot

import (
	"context"
	"strings"
	"sync/atomic"
	"time"

	"botchat/internal/ingesta"
	"botchat/internal/motor"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

var (
	motorActivo       atomic.Bool
	ventanaAntiguaMin atomic.Int32
)

func init() {
	motorActivo.Store(true)
	ventanaAntiguaMin.Store(10)
}

// ConfigurarMotor fija el interruptor general del bot automático y los minutos de antigüedad máxima de lo que se responde
// (más viejo que eso no se contesta; por defecto 10).
func ConfigurarMotor(activo bool, ventanaMin int) {
	motorActivo.Store(activo)
	if ventanaMin < 1 {
		ventanaMin = 10
	}
	ventanaAntiguaMin.Store(int32(ventanaMin))
}

var firmaBot atomic.Value // string: texto opcional al final de las respuestas automáticas

// ConfigurarFirma fija el texto que se agrega al final de lo que responde el bot (no de lo que escribo yo). "" = sin firma.
func ConfigurarFirma(texto string) { firmaBot.Store(strings.TrimSpace(texto)) }

func firmaActual() string {
	f, _ := firmaBot.Load().(string)
	return f
}

// ActividadMotor devuelve (JSON) los últimos renglones del registro de actividad: qué llegó, qué coincidió, qué se respondió
// o por qué se omitió. Sin textos de mensajes ni nombres.
func ActividadMotor(botID int64, limite int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.UltimaActividad(botID, limite))
}

// BotEnChat enciende o apaga el bot en un chat (el interruptor Activo / Pausado de los grupos).
func BotEnChat(botID int64, chat string, activo bool) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.FijarBotActivo(botID, chat, activo)
}

// PausarBotEnChat hace que el bot calle en un chat durante `minutos` (0 = reactivarlo ya).
func PausarBotEnChat(botID int64, chat string, minutos int) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	if minutos <= 0 {
		a.ReactivarSesiones(botID, chat)
		return a.PausarBot(botID, chat, 0)
	}
	return a.PausarBot(botID, chat, time.Now().Add(time.Duration(minutos)*time.Minute).Unix())
}

// MotorFrenado dice si el motor está en pausa por errores de envío seguidos.
func MotorFrenado() bool {
	mu.Lock()
	s := st
	mu.Unlock()
	return s != nil && s.motor != nil && s.motor.Frenado()
}

// enviadorMotor adapta el servicio de envío a lo que pide el motor.
type enviadorMotor struct{ s *estado }

func (e enviadorMotor) Enviar(ctx context.Context, chat, texto, citaID, mencion string) error {
	var men []string
	if mencion != "" {
		men = []string{mencion}
	}
	r, err := e.s.envio.EnviarAuto(ctx, chat, texto, citaID, men)
	if err != nil {
		return err
	}
	if r.Estado != "enviado" {
		return errNoEnviado
	}
	return nil
}

func (e enviadorMotor) Escribiendo(ctx context.Context, chat string, si bool) {
	_ = e.s.envio.Escribiendo(ctx, chat, si)
}

var errNoEnviado = errNoSalio{}

type errNoSalio struct{}

func (errNoSalio) Error() string { return "el mensaje no salió" }

// alMotor pasa un mensaje recién guardado al bot automático.
func (s *estado) alMotor(v *events.Message, sincronizando bool) {
	c := ingesta.Clasificar(v)
	if c.Omitir || c.Revocar != "" || c.Editar != nil || c.Reaccion != nil {
		return
	}
	chat, ok := s.ing.ChatDe(s.ctx, v.Info.MessageSource)
	if !ok {
		return
	}
	s.aprendeAlias(v)
	m := motor.Mensaje{
		Chat: chat, ID: string(v.Info.ID), Texto: c.Texto, EsGrupo: v.Info.IsGroup || v.Info.Chat.Server == types.GroupServer, EsMio: v.Info.IsFromMe,
		TS: v.Info.Timestamp, Antiguo: sincronizando, EsTexto: c.Tipo == "texto", EsEdicion: v.IsEdit,
		RemitenteWA: v.Info.Sender.ToNonAD().String(), Remitente: s.ing.Persona(s.ctx, v.Info.Sender),
	}
	if !m.EsGrupo {
		m.Remitente = chat
	}
	for _, j := range c.Menciones {
		if pj, err := types.ParseJID(j); err == nil && s.esPropio(pj) {
			m.Mencionado = true
		}
	}
	if c.CitaAutor != "" {
		if pj, err := types.ParseJID(c.CitaAutor); err == nil && s.esPropio(pj) {
			m.CitaEsMia = true
		}
	}
	if m.EsMio && s.envio.EsEnvioPropio(chat, m.ID) {
		return // lo que mandó esta app o el motor (por ejemplo la respuesta a un comando) nunca vuelve como comando
	}
	m.Nombre = v.Info.PushName
	m.EsChatPropio = !m.EsGrupo && s.esPropio(v.Info.Chat)
	for _, j := range c.Menciones {
		if pj, err := types.ParseJID(j); err == nil && !s.esPropio(pj) {
			m.Menciones = append(m.Menciones, s.ing.Persona(s.ctx, pj))
		}
	}
	if c.CitaAutor != "" {
		if pj, err := types.ParseJID(c.CitaAutor); err == nil && !s.esPropio(pj) {
			m.CitaAutor = s.ing.Persona(s.ctx, pj)
		}
	}
	s.motor.Entrar(m)
}
