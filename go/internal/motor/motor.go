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
	"botchat/internal/comandos"
	"botchat/internal/permisos"
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

	// Rangos y comandos (v0.5.0, bloque A).
	Nombre       string   // nombre público con el que escribe (solo para la bandeja de solicitudes)
	EsChatPropio bool     // el chat es "Mensajes guardados" (el chat conmigo mismo: mi propio número o LID)
	Menciones    []string // personas mencionadas (clave guardada)
	CitaAutor    string   // autor del mensaje citado (clave guardada), si cita a alguien
	CitaID       string   // id del mensaje citado, si cita alguno
}

// Salida es una respuesta lista para mandar.
type Salida struct{ Texto string }

// Decision es lo que resuelve el decisor para un mensaje (o un bloque de mensajes).
type Decision struct {
	Respuestas   []Salida
	Coincidio    string // nombre corto de lo que coincidió (para el registro; sin textos del usuario)
	PasarAHumano bool   // calla en ese chat (privado) hasta que se reactive o pasen 12 h; en grupos el motor de menús calla solo con esa persona
	Omitir       string // no responder y anotar este motivo en el registro (por ejemplo "esperando a una persona")
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
	Firma          func() string        // texto opcional al final de las respuestas automáticas ("" = ninguno; apagado por defecto)
	VentanaAntigua func() time.Duration // más viejo que esto no se responde (10 min)
	Ahora          func() time.Time
	Dormir         func(ctx context.Context, d time.Duration) bool // false si se canceló

	Debounce     time.Duration // espera tras el último mensaje de un privado (1,5 s)
	DebounceMax  time.Duration // tope de espera para juntar una ráfaga (6 s)
	EsperaMin    time.Duration // espera antes de enviar (0,8 s)
	EsperaMax    time.Duration // (2,5 s)
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

	// Perm decide a quién le contesta el bot (bloqueados, desconocidos, política de cada grupo). nil = a todos, como antes.
	Perm *permisos.Servicio
	// Cmd es el despachador de comandos con prefijo. nil = sin comandos.
	Cmd *comandos.Despachador
	// Baja revisa un texto recibido en privado: si pide BAJA / STOP / NO MÁS, deja a la persona en la lista No molestar y devuelve
	// una función que da el texto de confirmación (solo la primera vez; después, vacío). nil = sin lista No molestar.
	Baja func(chat, texto string) (bool, func() string)
	// EsAdminGrupo dice si una persona administra el grupo (solo se pregunta si la política del grupo es "solo administradores").
	EsAdminGrupo func(chat, persona string) bool
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
	manual     map[string]time.Time // chat -> hora de mi último mensaje escrito a mano ahí
	mDueno     sync.Mutex           // los comandos del Dueño salen de uno en uno

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
	ts                           time.Time // hora del mensaje al que se responde
	res                          chan error
}

// Errores internos del envío.
var (
	ErrLimite = errors.New("límite de envíos")
	ErrFreno  = errors.New("freno")
	// ErrYaContesto: yo contesté a mano ese mensaje antes de que saliera la respuesta automática.
	ErrYaContesto = errors.New("ya contestaste tú")
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
	return &Motor{c: c, carriles: map[string]*carril{}, repetido: map[string]time.Time{}, salida: make(chan *pedido, 256), marcasC: map[string][]time.Time{}, manual: map[string]time.Time{}}
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
		// Lo que escribo yo se ignora, salvo un comando con prefijo en "Mensajes guardados": ese es un comando de Dueño.
		if msg.EsChatPropio && m.c.Cmd != nil && msg.EsTexto && !msg.EsEdicion && !msg.EsGrupo && m.c.Cmd.EsComando(msg.Texto) {
			m.entrarDueno(msg)
		}
		return
	}
	m.c.Alm.RegistrarActividad(m.c.BotID, almacen.ActLlego, "", msg.Chat, tipoChat(msg))
	if !m.c.Alm.PrimeraVez(m.c.BotID, msg.Chat, msg.ID) {
		m.omite(msg, almacen.MotDuplicado)
		return
	}
	now := m.c.Ahora()
	// BAJA / STOP / NO MÁS en privado: la persona entra a la lista No molestar aunque el bot esté apagado o ella esté en espera.
	var bajaConfirma func() string
	esBaja := false
	if m.c.Baja != nil && !msg.EsGrupo && msg.EsTexto && !msg.EsEdicion && !msg.Antiguo && (msg.TS.IsZero() || now.Sub(msg.TS) <= m.c.VentanaAntigua()) {
		esBaja, bajaConfirma = m.c.Baja(msg.Chat, msg.Texto)
	}
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
	// Un chat pausado solo atiende los comandos marcados "siempre" (por ejemplo !reanudar), para poder volver a encenderlo.
	siempre := m.c.Cmd != nil && m.c.Cmd.SiemprePermitido(msg.Texto)
	if est.BotPausaHasta > now.Unix() && !siempre {
		if est.PausaHumano {
			m.omite(msg, almacen.MotHumano)
		} else {
			m.omite(msg, almacen.MotPausado)
		}
		return
	}
	if msg.EsGrupo {
		if !est.BotActivo && !siempre {
			m.omite(msg, almacen.MotChatPausa)
			return
		}
		if !(msg.Mencionado || msg.CitaEsMia || esComando(msg.Texto) || (m.c.Cmd != nil && m.c.Cmd.EsComando(msg.Texto))) {
			m.omite(msg, almacen.MotGrupo)
			return
		}
	}
	// Bloqueados, desconocidos (privado) y política de cada grupo.
	if m.c.Perm != nil {
		quien := msg.Remitente
		if !msg.EsGrupo {
			quien = msg.Chat
		}
		sal := m.c.Perm.Compuerta(permisos.Entrada{Persona: quien, Chat: msg.Chat, Nombre: msg.Nombre, EsGrupo: msg.EsGrupo,
			EsAdmin: func() bool { return m.c.EsAdminGrupo != nil && m.c.EsAdminGrupo(msg.Chat, quien) }})
		if !sal.Pasa {
			m.omite(msg, sal.Motivo)
			if sal.Responder != "" {
				m.wg.Add(1)
				go func() {
					defer m.wg.Done()
					if err := m.enviar(msg.Chat, sal.Responder, msg.ID, "", msg.TS); err == nil {
						m.c.Alm.RegistrarActividad(m.c.BotID, almacen.ActRespondio, "", msg.Chat, "desconocido")
					}
				}()
			}
			return
		}
	}
	// La baja se atiende aquí (ya pasó la compuerta): se le confirma UNA sola vez y el texto no sigue a los menús.
	if esBaja {
		m.omite(msg, "no molestar")
		if t := bajaConfirma(); t != "" {
			m.wg.Add(1)
			go func() {
				defer m.wg.Done()
				if err := m.enviar(msg.Chat, t, msg.ID, "", msg.TS); err == nil {
					m.c.Alm.RegistrarActividad(m.c.BotID, almacen.ActRespondio, "", msg.Chat, "no_molestar")
				}
			}()
		}
		return
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

// yaContesto: escribí yo en ese chat a la hora del mensaje o después (la respuesta automática ya no hace falta).
func (m *Motor) yaContesto(chat string, ts time.Time) bool {
	if ts.IsZero() {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.manual[chat]
	return ok && !t.Before(ts)
}

// Intervino avisa que escribí yo (a mano, desde la app o desde el teléfono) en un chat a la hora `ts`. Los mensajes que seguían
// esperando en la fila de ese chat, y los que ya tenían respuesta lista pero sin enviar, se cancelan si son de esa hora o anteriores.
// No debe llamarse con lo que mandó la app o el motor (eso no es una intervención mía).
func (m *Motor) Intervino(chat string, ts time.Time) {
	if ts.IsZero() {
		ts = m.c.Ahora()
	}
	m.mu.Lock()
	if t, ok := m.manual[chat]; !ok || ts.After(t) {
		m.manual[chat] = ts
	}
	if len(m.manual) > 2000 { // solo importan los recientes
		for k, t := range m.manual {
			if ts.Sub(t) > time.Hour {
				delete(m.manual, k)
			}
		}
	}
	var quitados []Mensaje
	if cr := m.carriles[chat]; cr != nil {
		resto := cr.cola[:0:0]
		for _, x := range cr.cola {
			if !x.TS.IsZero() && !ts.Before(x.TS) {
				quitados = append(quitados, x)
			} else {
				resto = append(resto, x)
			}
		}
		cr.cola = resto
	}
	m.mu.Unlock()
	for _, x := range quitados {
		m.omite(x, almacen.MotYaContesto)
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
		if len(cr.cola) == 0 { // contesté yo mientras esperaba: no queda nada que atender
			m.mu.Unlock()
			continue
		}
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
	// Pudieron pedir silencio mientras esperaba en la fila. En un chat pausado solo se atienden los comandos "siempre" (!reanudar).
	soloSiempre := false
	if est := m.c.Alm.EstadoDeChat(m.c.BotID, chat); est.BotPausaHasta > m.c.Ahora().Unix() || (ultimo.EsGrupo && !est.BotActivo) {
		hay := false
		if m.c.Cmd != nil {
			for _, x := range lote {
				if m.c.Cmd.SiemprePermitido(x.Texto) {
					hay = true
				}
			}
		}
		if !hay {
			switch {
			case est.BotPausaHasta <= m.c.Ahora().Unix():
				m.omite(ultimo, almacen.MotChatPausa)
			case est.PausaHumano:
				m.omite(ultimo, almacen.MotHumano)
			default:
				m.omite(ultimo, almacen.MotPausado)
			}
			return
		}
		soloSiempre = true
	}
	if m.yaContesto(chat, ultimo.TS) {
		m.omite(ultimo, almacen.MotYaContesto)
		return
	}
	for _, x := range lote[:len(lote)-1] {
		m.c.Alm.RegistrarActividad(m.c.BotID, almacen.ActOmitido, "agrupado con el siguiente", chat, "")
		_ = x
	}
	var deComandos []Salida
	ultimoEsComando := false
	if m.c.Cmd != nil {
		for _, x := range lote {
			if !m.c.Cmd.Existe(x.Texto) || (soloSiempre && !m.c.Cmd.SiemprePermitido(x.Texto)) {
				continue
			}
			r := m.c.Cmd.Ejecutar(comandos.Entrada{Persona: personaDe(x), Nombre: x.Nombre, Chat: chat, EsGrupo: x.EsGrupo, MensajeID: x.ID, Texto: x.Texto,
				Menciones: x.Menciones, CitaAutor: x.CitaAutor, CitaID: x.CitaID})
			if !r.Manejado {
				continue
			}
			if r.Texto != "" {
				deComandos = append(deComandos, Salida{Texto: m.sinPrefijo(r.Texto)})
			}
			if x.ID == ultimo.ID {
				ultimoEsComando = true
			}
		}
	}
	var d Decision
	if soloSiempre {
		d = Decision{Coincidio: "comando", Respuestas: deComandos}
	} else if ultimoEsComando {
		d = Decision{Coincidio: "comando", Respuestas: deComandos}
	} else {
		var err error
		d, err = m.c.Decisor.Decidir(m.ctx, ultimo, lote)
		if err != nil {
			m.c.Log("Error al decidir la respuesta: " + err.Error())
			return
		}
		if len(deComandos) > 0 {
			d.Respuestas = append(deComandos, d.Respuestas...)
			if d.Coincidio == "" {
				d.Coincidio = "comando"
			}
		}
	}
	if d.Omitir != "" {
		m.omite(ultimo, d.Omitir)
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
		if m.c.Firma != nil {
			if f := strings.TrimSpace(m.c.Firma()); f != "" {
				texto += "\n\n" + f
			}
		}
		mencion := ""
		if ultimo.EsGrupo && ultimo.RemitenteWA != "" {
			mencion = ultimo.RemitenteWA
			texto = "@" + usuarioDe(mencion) + " " + texto
		}
		if m.yaContesto(chat, ultimo.TS) {
			m.omite(ultimo, almacen.MotYaContesto)
			return
		}
		err := m.enviar(chat, texto, ultimo.ID, mencion, ultimo.TS)
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
		case errors.Is(err, ErrYaContesto):
			m.omite(ultimo, almacen.MotYaContesto)
			return
		default:
			m.c.Log("No se pudo responder a …" + corto(chat) + ": " + err.Error())
			return
		}
	}
	if d.PasarAHumano && !ultimo.EsGrupo { // en grupos no se calla todo el grupo: el motor de menús calla solo con esa persona
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
func (m *Motor) enviar(chat, texto, citaID, mencion string, ts time.Time) error {
	p := &pedido{chat: chat, texto: texto, citaID: citaID, mencion: mencion, ts: ts, res: make(chan error, 1)}
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
	// Una sola espera humana (0,8 a 2,5 s) y nada más: sin esperas extra por la longitud del texto.
	m.c.Enviador.Escribiendo(m.ctx, p.chat, true)
	ok := m.c.Dormir(m.ctx, espera)
	if !ok {
		m.c.Enviador.Escribiendo(m.ctx, p.chat, false)
		return m.ctx.Err()
	}
	// Si mientras esperaba contesté yo a ese mensaje, no se manda.
	if m.yaContesto(p.chat, p.ts) {
		m.c.Enviador.Escribiendo(m.ctx, p.chat, false)
		return ErrYaContesto
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

// personaDe es quién escribió el mensaje: en privado, el chat; en grupos, el remitente.
func personaDe(x Mensaje) string {
	if x.EsGrupo {
		return x.Remitente
	}
	return x.Chat
}

// sinPrefijo evita que una respuesta empiece con el prefijo de comandos (así ni un eco ni un bucle la toman por una orden).
func (m *Motor) sinPrefijo(t string) string {
	if m.c.Cmd != nil && strings.HasPrefix(strings.TrimSpace(t), m.c.Cmd.Prefijo()) {
		return "· " + t
	}
	return t
}

// entrarDueno atiende un comando que escribí yo en "Mensajes guardados". La respuesta sale a ese mismo chat, nunca a terceros, y
// no cuenta como intervención mía (no cancela respuestas pendientes de otros).
func (m *Motor) entrarDueno(msg Mensaje) {
	now := m.c.Ahora()
	if msg.Antiguo || (!msg.TS.IsZero() && now.Sub(msg.TS) > m.c.VentanaAntigua()) || !m.c.Activo() {
		return
	}
	if !m.c.Alm.PrimeraVez(m.c.BotID, msg.Chat, msg.ID) {
		return
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.mDueno.Lock()
		defer m.mDueno.Unlock()
		if m.ctx == nil || m.ctx.Err() != nil {
			return
		}
		r := m.c.Cmd.Ejecutar(comandos.Entrada{Persona: almacen.PersonaDueno, Chat: msg.Chat, MensajeID: msg.ID, Texto: msg.Texto, Dueno: true})
		if r.Texto == "" {
			return
		}
		if err := m.enviar(msg.Chat, m.sinPrefijo(r.Texto), msg.ID, "", time.Time{}); err != nil {
			m.c.Log("No se pudo responder un comando del dueño: " + err.Error())
		}
	}()
}

// ResponderA manda un texto a un chat citando un mensaje, como respuesta a un comando (por ejemplo cuando el dueño confirma
// una acción desde la app). No lleva espera de la fila del chat.
func (m *Motor) ResponderA(chat, citaID, texto string) error {
	if m.ctx == nil {
		return errors.New("el motor no está iniciado")
	}
	return m.enviar(chat, m.sinPrefijo(texto), citaID, "", time.Time{})
}
