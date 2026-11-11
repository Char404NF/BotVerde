// Package modulos es el marco de los módulos opcionales (v0.5.0, H0). Por defecto el bot SOLO RESPONDE; todo lo que escribe sin que
// le hablen existe únicamente como módulo: apagado por defecto, con su interruptor, su ventana de configuración, su aviso de riesgo,
// sus límites y su registro. Un módulo nuevo es, en lo normal, UN archivo con un `init()` que llama a `Registrar(Nuevo(Def{...}))`.
//
// Piezas:
//   - Modulo / Def / Nuevo: qué es un módulo y cómo se declara en pocas líneas.
//   - Registrar / Lista / Obtener: el registro central (también agrega las casillas de permisos al catálogo).
//   - Gestor: lo que vive por cada bot (activar, desactivar, configuración cifrada, comandos, eventos, modo seguro).
//   - Cola: la ÚNICA vía por la que un módulo escribe por iniciativa propia (maestro, topes, pausa nocturna, No molestar, registro).
package modulos

import (
	"time"

	"botchat/internal/comandos"
	"botchat/internal/permisos"
)

// Riesgos de un módulo (los mismos que las casillas).
const (
	RiesgoBajo  = permisos.RiesgoBajo
	RiesgoMedio = permisos.RiesgoMedio
	RiesgoAlto  = permisos.RiesgoAlto
)

// Tipos de evento que reciben los módulos activos.
const (
	EvMensaje      = "mensaje"       // llegó un mensaje (ya guardado)
	EvMiembroEntra = "miembro_entra" // alguien entró a un grupo
	EvLlamada      = "llamada"       // llamada entrante
	EvTick         = "tick"          // cada minuto (alarma)
)

// Evento es lo que se le avisa a un módulo. Vive solo en memoria: los módulos no guardan textos de mensajes por su cuenta.
type Evento struct {
	Tipo      string
	Hora      time.Time
	Chat      string   // chat donde ocurrió
	EsGrupo   bool     // el chat es un grupo
	Persona   string   // quien escribió, entró o llamó (clave guardada)
	Nombre    string   // nombre público, si lo trae
	Texto     string   // texto del mensaje (solo EvMensaje)
	MensajeID string   // id del mensaje (solo EvMensaje)
	EsMio     bool     // lo escribí yo
	Personas  []string // quienes entraron (solo EvMiembroEntra)
}

// Modulo es lo que un módulo ofrece al marco.
type Modulo interface {
	ID() string          // estable, en minúsculas y sin espacios ("bienvenida")
	Nombre() string      // corto, en español
	Descripcion() string // en palabras simples
	Icono() string       // nombre de icono que la app sabe dibujar ("saludo", "difusion", "reloj"…); si no lo conoce usa uno genérico
	Riesgo() string      // bajo | medio | alto
	// Iniciativa dice si el módulo escribe sin que le hablen. Esos módulos dependen del interruptor general "Iniciativa propia",
	// se pausan con el modo seguro y SOLO pueden enviar por la Cola.
	Iniciativa() bool
	Casillas() []permisos.Casilla // casillas de permisos que aporta (se registran en el catálogo al registrar el módulo)
	ConfigDefecto() string        // configuración por defecto (JSON de un objeto)
	Esquema() []Campo             // campos simples para dibujar la configuración sin pantalla propia (nil = pantalla propia)
	Limites() (hora, dia int)     // topes propios por defecto de lo que el módulo escribe (0 = sin tope propio)
	Comandos() []comandos.Comando // comandos que existen SOLO mientras el módulo está activo
	Activar(g *Gestor) error      // se llama al encenderlo y cada vez que arranca el bot con el módulo encendido
	Desactivar(g *Gestor) error
	Evento(g *Gestor, e Evento) // manejo de eventos; debe ser corto: solo anota y encola
	Estado(g *Gestor) string    // una línea para la tarjeta ("3 mensajes enviados hoy"); "" = nada que decir
}

// ConPausa lo implementan los módulos que quieren enterarse del modo seguro (opcional).
type ConPausa interface {
	Pausar(g *Gestor)
	Reanudar(g *Gestor)
}

// Validador valida la configuración completa antes de guardarla (opcional; el esquema ya se valida solo).
type Validador interface {
	ValidarConfig(cfg map[string]any) error
}

// ConfigCambiada avisa al módulo que su configuración cambió (opcional).
type ConfigCambiada interface {
	ConfigCambiada(g *Gestor)
}

// Def declara un módulo sin escribir métodos. Lo mínimo: ID, Nombre, Descripcion y Riesgo.
type Def struct {
	ID, Nombre, Descripcion, Icono, Riesgo string
	Iniciativa                             bool
	Casillas                               []permisos.Casilla
	Config                                 string  // JSON por defecto; lo que falte se completa con los valores por defecto del esquema
	Esquema                                []Campo // configuración genérica (texto, número, interruptor, lista, hora)
	TopeHora, TopeDia                      int
	Comandos                               []comandos.Comando
	AlActivar, AlDesactivar                func(g *Gestor) error
	AlEvento                               func(g *Gestor, e Evento)
	AlEstado                               func(g *Gestor) string
	AlPausar, AlReanudar                   func(g *Gestor)
	Validar                                func(cfg map[string]any) error
	AlCambiarConfig                        func(g *Gestor)
}

type simple struct{ d Def }

// Nuevo arma un Modulo a partir de su declaración.
func Nuevo(d Def) Modulo {
	if d.Riesgo == "" {
		d.Riesgo = RiesgoBajo
	}
	return &simple{d}
}

func (s *simple) ID() string                   { return s.d.ID }
func (s *simple) Nombre() string               { return s.d.Nombre }
func (s *simple) Descripcion() string          { return s.d.Descripcion }
func (s *simple) Icono() string                { return s.d.Icono }
func (s *simple) Riesgo() string               { return s.d.Riesgo }
func (s *simple) Iniciativa() bool             { return s.d.Iniciativa }
func (s *simple) Casillas() []permisos.Casilla { return s.d.Casillas }
func (s *simple) Esquema() []Campo             { return s.d.Esquema }
func (s *simple) Limites() (int, int)          { return s.d.TopeHora, s.d.TopeDia }
func (s *simple) Comandos() []comandos.Comando { return s.d.Comandos }
func (s *simple) ConfigDefecto() string        { return s.d.Config }
func (s *simple) Activar(g *Gestor) error {
	if s.d.AlActivar != nil {
		return s.d.AlActivar(g)
	}
	return nil
}
func (s *simple) Desactivar(g *Gestor) error {
	if s.d.AlDesactivar != nil {
		return s.d.AlDesactivar(g)
	}
	return nil
}
func (s *simple) Evento(g *Gestor, e Evento) {
	if s.d.AlEvento != nil {
		s.d.AlEvento(g, e)
	}
}
func (s *simple) Estado(g *Gestor) string {
	if s.d.AlEstado != nil {
		return s.d.AlEstado(g)
	}
	return ""
}
func (s *simple) Pausar(g *Gestor) {
	if s.d.AlPausar != nil {
		s.d.AlPausar(g)
	}
}
func (s *simple) Reanudar(g *Gestor) {
	if s.d.AlReanudar != nil {
		s.d.AlReanudar(g)
	}
}
func (s *simple) ValidarConfig(cfg map[string]any) error {
	if s.d.Validar != nil {
		return s.d.Validar(cfg)
	}
	return nil
}
func (s *simple) ConfigCambiada(g *Gestor) {
	if s.d.AlCambiarConfig != nil {
		s.d.AlCambiarConfig(g)
	}
}
