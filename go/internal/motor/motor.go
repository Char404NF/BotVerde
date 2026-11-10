// Package motor es el bot automático: recibe los mensajes ya guardados, decide si responder y manda las respuestas con
// cuidado para no parecer un robot ni saturar a WhatsApp. SOLO RESPONDE: nunca escribe primero ni hace nada programado.
//
// Reglas de cuidado: no contesta lo viejo ni lo de sincronización, no contesta dos veces el mismo mensaje (aunque la app se
// reinicie), un chat a la vez en orden y chats distintos en paralelo, junta las ráfagas de un chat privado en una sola
// respuesta, en grupos solo contesta si lo mencionan, una sola cola de salida con esperas al azar y límites por minuto, y
// se frena solo si fallan varios envíos seguidos.
package motor

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"botchat/internal/almacen"
)

// Mensaje es lo que el motor necesita saber de un mensaje recibido (nunca se guarda su texto en el registro).
type Mensaje struct {
	Chat        string // clave interna del chat
	ID          string // id del mensaje
	Remitente   string // quién lo escribió (clave guardada)
	RemitenteWA string // JID tal como lo manda WhatsApp (para mencionar)
	Texto       string
	EsGrupo     bool
	EsMio       bool
	TS          time.Time
	Antiguo     bool // llegó en la sincronización offline o del historial
	Mencionado  bool // en un grupo: me mencionaron
	CitaEsMia   bool // en un grupo: responden citando un mensaje mío
	EsTexto     bool
	EsEdicion   bool
}

// Salida es una respuesta lista para mandar.
type Salida struct{ Texto string }

// Decision es lo que resuelve el decisor para un mensaje (o un bloque de mensajes).
type Decision struct {
	Respuestas   []Salida
	Coincidio    string // nombre corto de lo que coincidió (para el registro; sin textos del usuario)
	PasarAHumano bool   // calla en ese chat hasta que se reactive o pasen 12 h
	Aviso        string // texto neutro para una notificación local ("" = ninguna)
}

// Decisor decide qué responder. `lote` trae todos los mensajes juntados (en privado) y `ultimo` es el último.
type Decisor interface {
	Decidir(ctx context.Context, ultimo Mensaje, lote []Mensaje) (Decision, error)
}

// Enviador manda lo que decide el motor.
type Enviador interface {
	// Enviar manda un texto citando el mensaje `citaID`; en grupos `mencion` es el JID a mencionar (con su @usuario ya en el texto).
	Enviar(ctx context.Context, chat, texto, citaID, mencion string) error
	Escribiendo(ctx context.Context, chat string, si bool)
}

// Config son las piezas y los ajustes del motor. Los tiempos por defecto son los de producción; las pruebas los acortan.
type Config struct {
	BotID    int64
	Alm      *almacen.Almacen
	Decisor  Decisor
	Enviador Enviador
	Log      func(string)

	Activo         func() bool          // interruptor general (Avanzado → Funciones)
	VentanaAntigua func() time.Duration // más viejo que esto no se responde (10 min)
	Ahora          func() time.Time
	Dormir         func(ctx context.Context, d time.Duration) bool // false si se canceló

	Debounce     time.Duration // espera tras el último mensaje de un privado (1,5 s)
	DebounceMax  time.Duration // tope de espera para juntar una ráfaga (6 s)
	EsperaMin    time.Duration // espera antes de enviar (0,8 s)
	EsperaMax    time.Duration // (2,5 s)
	PorCaracter  time.Duration // espera extra por cada letra de la respuesta, como alguien que teclea (25 ms; tope 2,5 s)
	LimiteGlobal int           // respuestas por minuto en total (20)
	LimiteChat   int           // respuestas por minuto en un chat (6)
	MaxCola      int           // mensajes en espera por chat (10)
	Repetido     time.Duration // el mismo texto de la misma persona se ignora si vuelve antes de esto (5 s)
	FrenoFallos  int           // envíos fallidos seguidos para frenar (5)
	FrenoDura    time.Duration // cuánto dura el freno (5 min)
	AlFrenar     func()        // se avisa al usuario
	AlHumano     func(chat, aviso string)
	AlCoincidir  func(chat string, d Decision)
	Aleatorio    func(n int64) int64
}

// Motor es el bot automático.
type Motor struct {
	c Config

	mu         sync.Mutex
	carriles   map[string]*carril
	repetido   map[string]time.Time
	salida     chan *pedido
	avisos     chan struct{}
	marcasG    []time.Time
	marcasC    map[string][]time.Time
	fallos     int
	frenoHasta time.Time

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type carril struct {
	cola    []Mensaje
	activo  bool
	primero time.Time
	ultimo  time.Time
}

type pedido struct {
	chat, texto, citaID, mencion string
	res                          chan error
}

// Errores internos del envío.
var (
	ErrLimite = errors.New("límite de envíos")
	ErrFreno  = errors.New("freno")
)

// Nuevo crea el motor con los valores por defecto donde falte algo.
func Nuevo(c Config) *Motor {
	if c.Log == nil {
		c.Log = func(string) {}
	}
	if c.Activo == nil {
		c.Activo = func() bool { return true }
	}
	if c.VentanaAntigua == nil {
		c.VentanaAntigua = func() time.Duration { return 10 * time.Minute }
	}
	if c.Ahora == nil {
		c.Ahora = time.Now
	}
	if c.Dormir == nil {
		c.Dormir = func(ctx context.Context, d time.Duration) bool {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-t.C:
				return true
			case <-ctx.Done():
				return false
			}
		}
	}
	def := func(p *time.Duration, v time.Duration) {
		if *p == 0 {
			*p = v
		}
	}
	def(&c.Debounce, 1500*time.Millisecond)
	def(&c.DebounceMax, 6*time.Second)
	def(&c.EsperaMin, 800*time.Millisecond)
	def(&c.EsperaMax, 2500*time.Millisecond)
	def(&c.PorCaracter, 25*time.Millisecond)
	def(&c.Repetido, 5*time.Second)
	def(&c.FrenoDura, 5*time.Minute)
	for p, v := range map[*int]int{&c.LimiteGlobal: 20, &c.LimiteChat: 6, &c.MaxCola: 10, &c.FrenoFallos: 5} {
		if *p == 0 {
			*p = v
		}
	}
	if c.Aleatorio == nil {
		c.Aleatorio = rand.Int63n
	}
	return &Motor{c: c, carriles: map[string]*carril{}, repetido: map[string]time.Time{}, salida: make(chan *pedido, 256), marcasC: map[string][]time.Time{}}
}

// Iniciar arranca la cola de salida. Hay que llamar a Detener al terminar.
func (m *Motor) Iniciar(ctx context.Context) {
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.wg.Add(1)
	go m.colaDeSalida()
	m.c.Alm.LimpiarVistos(m.c.BotID, 7)
	m.c.Alm.LimpiarActividad(m.c.BotID, 7)
}

// Detener para el motor y espera a que terminen sus tareas.
func (m *Motor) Detener() {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
}

func (m *Motor) omite(msg Mensaje, motivo string) {
	m.c.Alm.RegistrarActividad(m.c.BotID, almacen.ActOmitido, motivo, msg.Chat, "")
	m.c.Log("Mensaje de …" + corto(msg.Chat) + " omitido: " + motivo + ".")
}

func corto(s string) string {
	if i := strings.IndexByte(s, '@'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 4 {
		return s[len(s)-4:]
	}
	return s
}

// Entrar recibe un mensaje ya guardado. Hace solo comprobaciones rápidas y lo pone en la fila de su chat.
func (m *Motor) Entrar(msg Mensaje) {
	if msg.EsMio {
		return
	}
	m.c.Alm.RegistrarActividad(m.c.BotID, almacen.ActLlego, "", msg.Chat, tipoChat(msg))
	if !m.c.Alm.PrimeraVez(m.c.BotID, msg.Chat, msg.ID) {
		m.omite(msg, almacen.MotDuplicado)
		return
	}
	now := m.c.Ahora()
	switch {
	case msg.EsEdicion:
		m.omite(msg, "mensaje editado")
		return
	case msg.Antiguo || (!msg.TS.IsZero() && now.Sub(msg.TS) > m.c.VentanaAntigua()):
		m.omite(msg, almacen.MotAntiguo)
		return
	case !m.c.Activo():
		m.omite(msg, almacen.MotApagado)
		return
	case !msg.EsTexto || strings.TrimSpace(msg.Texto) == "":
		m.omite(msg, almacen.MotNoTexto)
		return
	}
	est := m.c.Alm.EstadoDeChat(m.c.BotID, msg.Chat)
	if est.BotPausaHasta > now.Unix() {
		if est.PausaHumano {
			m.omite(msg, almacen.MotHumano)
		} else {
			m.omite(msg, almacen.MotPausado)
		}
		return
	}
	if msg.EsGrupo {
		if !est.BotActivo {
			m.omite(msg, almacen.MotChatPausa)
			return
		}
		if !(msg.Mencionado || msg.CitaEsMia || esComando(msg.Texto)) {
			m.omite(msg, almacen.MotGrupo)
			return
		}
	}
	// El mismo texto de la misma persona, repetido enseguida, se ignora.
	clave := msg.Chat + "\x00" + msg.Remitente + "\x00" + strings.ToLower(strings.TrimSpace(msg.Texto))
	m.mu.Lock()
	if t, ok := m.repetido[clave]; ok && now.Sub(t) < m.c.Repetido {
		m.mu.Unlock()
		m.omite(msg, almacen.MotRepetido)
		return
	}
	m.repetido[clave] = now
	if len(m.repetido) > 5000 {
		for k, t := range m.repetido {
			if now.Sub(t) > time.Minute {
				delete(m.repetido, k)
			}
		}
	}
	cr := m.carriles[msg.Chat]
	if cr == nil {
		cr = &carril{}
		m.carriles[msg.Chat] = cr
	}
	if len(cr.cola) >= m.c.MaxCola {
		cr.cola = cr.cola[1:] // se descarta lo más viejo
		m.mu.Unlock()
		m.omite(Mensaje{Chat: msg.Chat}, almacen.MotCola)
		m.mu.Lock()
	}
	cr.cola = append(cr.cola, msg)
	cr.ultimo = now
	if len(cr.cola) == 1 {
		cr.primero = now
	}
	arranca := !cr.activo
	cr.activo = true
	m.mu.Unlock()
	if arranca {
		m.wg.Add(1)
		go m.trabajaCarril(msg.Chat)
	}
}

func tipoChat(msg Mensaje) string {
	if msg.EsGrupo {
		return "grupo"
	}
	return "privado"
}

// esComando: el texto empieza con "." seguido de una letra (por ejemplo ".menu").
func esComando(t string) bool {
	t = strings.TrimSpace(t)
	if len(t) < 2 || t[0] != '.' {
		return false
	}
	r := []rune(t)[1]
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r > 127
}

// trabajaCarril atiende, en orden, los mensajes de un chat. En privado junta las ráfagas; en grupos va de uno en uno.
func (m *Motor) trabajaCarril(chat string) {
	defer m.wg.Done()
	for {
		m.mu.Lock()
		cr := m.carriles[chat]
		if cr == nil || len(cr.cola) == 0 {
			if cr != nil {
				cr.activo = false
			}
			m.mu.Unlock()
			return
		}
		grupo := cr.cola[0].EsGrupo
		m.mu.Unlock()
		if !grupo {
			// Espera a que el chat se calle un momento (con tope) para contestar una sola vez a todo el bloque.
			for {
				m.mu.Lock()
				ahora := m.c.Ahora()
				quieto := ahora.Sub(cr.ultimo)
				total := ahora.Sub(cr.primero)
				m.mu.Unlock()
				if quieto >= m.c.Debounce || total >= m.c.DebounceMax {
					break
				}
				espera := m.c.Debounce - quieto
				if resto := m.c.DebounceMax - total; resto < espera {
					espera = resto
				}
				if espera < time.Millisecond {
					espera = time.Millisecond
				}
				if !m.c.Dormir(m.ctx, espera) {
					return
				}
			}
		}
		m.mu.Lock()
		var lote []Mensaje
		if grupo {
			lote = []Mensaje{cr.cola[0]}
			cr.cola = cr.cola[1:]
		} else {
			lote = cr.cola
			cr.cola = nil
		}
		if len(cr.cola) > 0 {
			cr.primero = m.c.Ahora()
		}
		m.mu.Unlock()
		if m.ctx.Err() != nil {
			return
		}
		m.atiende(chat, lote)
	}
}

// atiende decide y responde a un mensaje o a un bloque de mensajes del mismo chat.
func (m *Motor) atiende(chat string, lote []Mensaje) {
	ultimo := lote[len(lote)-1]
	// Pudieron pedir silencio mientras esperaba en la fila.
	if est := m.c.Alm.EstadoDeChat(m.c.BotID, chat); est.BotPausaHasta > m.c.Ahora().Unix() {
		if est.PausaHumano {
			m.omite(ultimo, almacen.MotHumano)
		} else {
			m.omite(ultimo, almacen.MotPausado)
		}
		return
	}
	for _, x := range lote[:len(lote)-1] {
		m.c.Alm.RegistrarActividad(m.c.BotID, almacen.ActOmitido, "agrupado con el siguiente", chat, "")
		_ = x
	}
	d, err := m.c.Decisor.Decidir(m.ctx, ultimo, lote)
	if err != nil {
		m.c.Log("Error al decidir la respuesta: " + err.Error())
		return
	}
	if d.Coincidio == "" && len(d.Respuestas) == 0 && !d.PasarAHumano {
		m.c.Alm.RegistrarActividad(m.c.BotID, almacen.ActSinCoincid, "", chat, "")
		return
	}
	m.c.Alm.RegistrarActividad(m.c.BotID, almacen.ActCoincidio, "", chat, d.Coincidio)
	if m.c.AlCoincidir != nil {
		m.c.AlCoincidir(chat, d)
	}
	for _, r := range d.Respuestas {
		texto := r.Texto
		mencion := ""
		if ultimo.EsGrupo && ultimo.RemitenteWA != "" {
			mencion = ultimo.RemitenteWA
			texto = "@" + usuarioDe(mencion) + " " + texto
		}
		err := m.enviar(chat, texto, ultimo.ID, mencion)
		switch {
		case err == nil:
			m.c.Alm.RegistrarActividad(m.c.BotID, almacen.ActRespondio, "", chat, "respondido a "+ultimo.ID)
			m.c.Log("Respondido a …" + corto(chat) + " (mensaje " + ultimo.ID + ").")
		case errors.Is(err, ErrLimite):
			m.omite(ultimo, almacen.MotLimite)
			return
		case errors.Is(err, ErrFreno):
			m.omite(ultimo, almacen.MotFreno)
			return
		default:
			m.c.Log("No se pudo responder a …" + corto(chat) + ": " + err.Error())
			return
		}
	}
	if d.PasarAHumano {
		_ = m.c.Alm.PausarBotHumano(m.c.BotID, chat, m.c.Ahora().Add(12*time.Hour).Unix())
		m.c.Alm.RegistrarActividad(m.c.BotID, almacen.ActOmitido, almacen.MotHumano, chat, "pasó a una persona")
		if m.c.AlHumano != nil {
			m.c.AlHumano(chat, d.Aviso)
		}
	}
}

func usuarioDe(j string) string {
	if i := strings.IndexByte(j, '@'); i >= 0 {
		j = j[:i]
	}
	if i := strings.IndexByte(j, ':'); i >= 0 {
		j = j[:i]
	}
	return j
}

// enviar pone una respuesta en la cola de salida y espera su resultado (así las respuestas de un chat salen en orden).
func (m *Motor) enviar(chat, texto, citaID, mencion string) error {
	p := &pedido{chat: chat, texto: texto, citaID: citaID, mencion: mencion, res: make(chan error, 1)}
	select {
	case m.salida <- p:
	case <-m.ctx.Done():
		return m.ctx.Err()
	}
	select {
	case err := <-p.res:
		return err
	case <-m.ctx.Done():
		return m.ctx.Err()
	}
}

// Frenado dice si el motor está en pausa por errores de envío.
func (m *Motor) Frenado() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.c.Ahora().Before(m.frenoHasta)
}

// permiso aplica los límites por minuto (global y por chat) y el freno.
func (m *Motor) permiso(chat string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.c.Ahora()
	if now.Before(m.frenoHasta) {
		return ErrFreno
	}
	corta := func(l []time.Time) []time.Time {
		i := 0
		for i < len(l) && now.Sub(l[i]) >= time.Minute {
			i++
		}
		return l[i:]
	}
	m.marcasG = corta(m.marcasG)
	m.marcasC[chat] = corta(m.marcasC[chat])
	if len(m.marcasG) >= m.c.LimiteGlobal || len(m.marcasC[chat]) >= m.c.LimiteChat {
		return ErrLimite
	}
	m.marcasG = append(m.marcasG, now)
	m.marcasC[chat] = append(m.marcasC[chat], now)
	return nil
}

func (m *Motor) colaDeSalida() {
	defer m.wg.Done()
	for {
		select {
		case <-m.ctx.Done():
			return
		case p := <-m.salida:
			p.res <- m.salirUno(p)
		}
	}
}

// salirUno espera un rato al azar mostrando "escribiendo…" y manda la respuesta.
func (m *Motor) salirUno(p *pedido) error {
	if err := m.permiso(p.chat); err != nil {
		return err
	}
	espera := m.c.EsperaMin
	if span := int64(m.c.EsperaMax - m.c.EsperaMin); span > 0 {
		espera += time.Duration(m.c.Aleatorio(span))
	}
	// Un poco más de espera si el texto es largo, como alguien que teclea.
	extra := time.Duration(len([]rune(p.texto))) * m.c.PorCaracter
	if extra > 2500*time.Millisecond {
		extra = 2500 * time.Millisecond
	}
	m.c.Enviador.Escribiendo(m.ctx, p.chat, true)
	ok := m.c.Dormir(m.ctx, espera+extra)
	if !ok {
		m.c.Enviador.Escribiendo(m.ctx, p.chat, false)
		return m.ctx.Err()
	}
	err := m.c.Enviador.Enviar(m.ctx, p.chat, p.texto, p.citaID, p.mencion)
	m.c.Enviador.Escribiendo(m.ctx, p.chat, false)
	m.mu.Lock()
	if err == nil {
		m.fallos = 0
		m.mu.Unlock()
		return nil
	}
	m.fallos++
	frenar := m.fallos >= m.c.FrenoFallos
	if frenar {
		m.frenoHasta = m.c.Ahora().Add(m.c.FrenoDura)
		m.fallos = 0
	}
	m.mu.Unlock()
	if frenar {
		m.c.Alm.RegistrarActividad(m.c.BotID, almacen.ActFreno, almacen.MotFreno, "", fmt.Sprintf("%d envíos seguidos fallaron", m.c.FrenoFallos))
		m.c.Log("El motor se pausó " + m.c.FrenoDura.String() + " porque fallaron varios envíos seguidos.")
		if m.c.AlFrenar != nil {
			m.c.AlFrenar()
		}
	}
	return err
}
