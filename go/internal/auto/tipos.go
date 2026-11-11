// Package auto son los mensajes automáticos: bienvenida, ausencia, recibido, cierre y respuestas rápidas (atajos). Todos son
// RESPUESTAS a un mensaje que alguien me escribió (o, el atajo, algo que escribo yo): nunca escriben primero.
//
// Prioridad: menú en curso > regla (automatización con disparadores o comando) > bienvenida y ausencia > recibido. El cierre se manda
// al terminar un menú. Si contesté yo, la respuesta pendiente se cancela (lo hace el motor).
package auto

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"botchat/internal/almacen"
)

// Rango es un tramo del día, "HH:MM" a "HH:MM". Si Desde es mayor que Hasta cruza la medianoche.
type Rango struct {
	Desde string `json:"desde"`
	Hasta string `json:"hasta"`
}

// DiaHorario son los tramos de un día de la semana (1 = lunes … 7 = domingo). Hasta DOS rangos por día (por ejemplo mañana y tarde).
type DiaHorario struct {
	Dia    int     `json:"dia"`
	Rangos []Rango `json:"rangos"`
}

// Vacacion es un rango de fechas (incluye los dos días), "AAAA-MM-DD".
type Vacacion struct {
	Nombre string `json:"nombre,omitempty"`
	Desde  string `json:"desde"`
	Hasta  string `json:"hasta"`
}

// FechaAnual es un día especial que se repite cada año (día y mes), por ejemplo 25/12.
type FechaAnual struct {
	Nombre string `json:"nombre,omitempty"`
	Dia    int    `json:"dia"`
	Mes    int    `json:"mes"`
}

// Cuando dice en qué momentos vale un mensaje. Basta que se cumpla UNA de las condiciones.
type Cuando struct {
	Siempre bool `json:"siempre,omitempty"`
	// Horario semanal con hasta dos rangos por día; HorarioModo dice si el mensaje vale "dentro" o "fuera" de ese horario.
	Horario     []DiaHorario `json:"horario,omitempty"`
	HorarioModo string       `json:"horarioModo,omitempty"` // dentro | fuera
	Vacaciones  []Vacacion   `json:"vacaciones,omitempty"`
	Fechas      []FechaAnual `json:"fechas,omitempty"`
	// Zona es la zona horaria (nombre IANA, "America/Mexico_City"). Vacía = la del bot.
	Zona string `json:"zona,omitempty"`
}

// Quien dice a quién se le manda.
type Quien struct {
	Personas string `json:"personas"` // todos | desconocidos | contactos
	Privados bool   `json:"privados"`
	Grupos   bool   `json:"grupos"`
	// GruposElegidos: si hay alguno, solo en esos grupos (vacío = en cualquier grupo donde el bot responde).
	GruposElegidos []string `json:"gruposElegidos,omitempty"`
	Excepto        []string `json:"excepto,omitempty"`   // a estas personas nunca
	SoloLista      []string `json:"soloLista,omitempty"` // si hay alguna, solo a estas personas
}

// Repeticion limita cada cuánto se le manda lo mismo a la misma persona.
type Repeticion struct {
	Cada   int    `json:"cada"`
	Unidad string `json:"unidad"` // minutos | horas | dias
}

// Duracion es el tiempo mínimo entre dos envíos a la misma persona (0 = sin límite).
func (r Repeticion) Duracion() time.Duration {
	switch r.Unidad {
	case "minutos":
		return time.Duration(r.Cada) * time.Minute
	case "dias":
		return time.Duration(r.Cada) * 24 * time.Hour
	}
	return time.Duration(r.Cada) * time.Hour
}

// Config es la configuración de un mensaje automático (va cifrada en auto_mensajes.config_cifrado).
type Config struct {
	Cuando     Cuando     `json:"cuando"`
	Quien      Quien      `json:"quien"`
	Repeticion Repeticion `json:"repeticion"`
	// SilencioDias (bienvenida): además de la primera vez, volver a dar la bienvenida si la persona lleva tantos días sin escribir
	// (7, 14 o 30; 0 = solo la primera vez).
	SilencioDias int `json:"silencioDias,omitempty"`
}

// ConfigPorDefecto es la configuración con la que nace un mensaje de ese tipo.
func ConfigPorDefecto(tipo string) Config {
	c := Config{Cuando: Cuando{Siempre: true}, Quien: Quien{Personas: "todos", Privados: true}, Repeticion: Repeticion{Cada: 6, Unidad: "horas"}}
	switch tipo {
	case almacen.AutoAusencia:
		c.Cuando = Cuando{HorarioModo: "fuera", Horario: SemanaLaboral()}
		c.Repeticion = Repeticion{Cada: 12, Unidad: "horas"}
	case almacen.AutoCierre:
		c.Repeticion = Repeticion{Cada: 1, Unidad: "horas"}
	case almacen.AutoBienvenida:
		c.Repeticion = Repeticion{Cada: 1, Unidad: "dias"}
	case almacen.AutoAtajo:
		c = Config{}
	}
	return c
}

// SemanaLaboral es de lunes a viernes, de 9 a 13 y de 15 a 18.
func SemanaLaboral() []DiaHorario {
	var h []DiaHorario
	for d := 1; d <= 5; d++ {
		h = append(h, DiaHorario{Dia: d, Rangos: []Rango{{"09:00", "13:00"}, {"15:00", "18:00"}}})
	}
	return h
}

// LeerConfig lee la configuración guardada; si falta o está dañada devuelve la de fábrica del tipo.
func LeerConfig(tipo, js string) Config {
	c := ConfigPorDefecto(tipo)
	if strings.TrimSpace(js) == "" {
		return c
	}
	var x Config
	if json.Unmarshal([]byte(js), &x) != nil {
		return c
	}
	if x.Quien.Personas == "" {
		x.Quien.Personas = "todos"
	}
	return x
}

// JSON devuelve la configuración lista para guardar.
func (c Config) JSON() string {
	b, _ := json.Marshal(c)
	return string(b)
}

// Límites de un mensaje.
const (
	MaxTexto   = 1500
	MaxNombre  = 40
	MaxAtajo   = 20
	MaxListas  = 200
	MaxFechas  = 60
	MaxVacacio = 30
)

func hora(s string) (int, bool) {
	var h, m int
	if n, err := fmt.Sscanf(strings.TrimSpace(s), "%d:%d", &h, &m); err != nil || n != 2 || h < 0 || h > 24 || m < 0 || m > 59 || (h == 24 && m != 0) {
		return 0, false
	}
	return h*60 + m, true
}

// Validar revisa un mensaje automático completo y deja la configuración en orden. Los mensajes de error se muestran tal cual.
func Validar(m *almacen.AutoMensaje, c *Config) error {
	if !almacen.EsTipoAuto(m.Tipo) {
		return fmt.Errorf("Ese tipo de mensaje no existe.")
	}
	m.Texto = strings.TrimSpace(m.Texto)
	m.Nombre = strings.TrimSpace(m.Nombre)
	if utf8.RuneCountInString(m.Texto) > MaxTexto {
		return fmt.Errorf("El mensaje es muy largo (máximo %d letras).", MaxTexto)
	}
	if utf8.RuneCountInString(m.Nombre) > MaxNombre {
		return fmt.Errorf("El nombre es muy largo (máximo %d letras).", MaxNombre)
	}
	if len(m.Adjuntos) > almacen.MaxAdjuntos {
		return fmt.Errorf("Un mensaje puede llevar hasta %d archivos.", almacen.MaxAdjuntos)
	}
	for i, a := range m.Adjuntos {
		if !almacen.IDAdjuntoValido(a.ID) {
			return fmt.Errorf("Al archivo %d le falta el archivo guardado.", i+1)
		}
		if almacen.LimiteAdjunto(a.Tipo) == 0 {
			return fmt.Errorf("El archivo %d no es de un tipo que se pueda mandar.", i+1)
		}
		if a.Tipo == almacen.AdjDocumento && strings.TrimSpace(a.Nombre) == "" {
			return fmt.Errorf("El documento %d necesita un nombre.", i+1)
		}
	}
	if m.Texto == "" && len(m.Adjuntos) == 0 {
		return fmt.Errorf("Escribe el mensaje (o agrega un archivo).")
	}
	if m.Tipo == almacen.AutoAtajo {
		n, err := NombreAtajo(m.Nombre)
		if err != nil {
			return err
		}
		m.Nombre = n
		if m.Texto == "" {
			return fmt.Errorf("Un atajo necesita un texto: es lo que reemplaza a lo que escribes.")
		}
		*c = Config{}
		return nil
	}
	if m.Nombre == "" {
		m.Nombre = NombrePorDefecto(m.Tipo)
	}
	if err := c.validar(m.Tipo); err != nil {
		return err
	}
	return nil
}

// NombrePorDefecto es el nombre que lleva un mensaje al que no se le puso uno.
func NombrePorDefecto(tipo string) string {
	switch tipo {
	case almacen.AutoBienvenida:
		return "Bienvenida"
	case almacen.AutoAusencia:
		return "Ausencia"
	case almacen.AutoRecibido:
		return "Recibido"
	case almacen.AutoCierre:
		return "Cierre"
	}
	return "Mensaje"
}

// NombreAtajo revisa el nombre de un atajo (lo que va tras la "/"): una sola palabra de letras, números, guion o guion bajo.
func NombreAtajo(n string) (string, error) {
	n = strings.ToLower(strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(n), "/")))
	if n == "" {
		return "", fmt.Errorf("Escribe el nombre del atajo, por ejemplo gracias.")
	}
	if utf8.RuneCountInString(n) > MaxAtajo {
		return "", fmt.Errorf("El nombre del atajo es muy largo (máximo %d letras).", MaxAtajo)
	}
	for _, r := range n {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r > 127 && r != 0xA0) {
			return "", fmt.Errorf("El atajo solo puede tener letras, números, guion y guion bajo, sin espacios.")
		}
	}
	return n, nil
}

func (c *Config) validar(tipo string) error {
	cu := &c.Cuando
	if cu.Zona != "" {
		if _, err := time.LoadLocation(cu.Zona); err != nil {
			return fmt.Errorf("La zona horaria «%s» no existe.", cu.Zona)
		}
	}
	if len(cu.Horario) > 0 {
		vistos := map[int]bool{}
		for i := range cu.Horario {
			d := &cu.Horario[i]
			if d.Dia < 1 || d.Dia > 7 {
				return fmt.Errorf("Un día del horario no es válido.")
			}
			if vistos[d.Dia] {
				return fmt.Errorf("Un día aparece dos veces en el horario.")
			}
			vistos[d.Dia] = true
			if len(d.Rangos) > 2 {
				return fmt.Errorf("Cada día puede tener hasta dos rangos de horas.")
			}
			for _, r := range d.Rangos {
				a, ok1 := hora(r.Desde)
				b, ok2 := hora(r.Hasta)
				if !ok1 || !ok2 {
					return fmt.Errorf("Una hora del horario no es válida (usa el formato 09:00).")
				}
				if a == b {
					return fmt.Errorf("Un rango del horario empieza y termina a la misma hora.")
				}
			}
		}
		switch cu.HorarioModo {
		case "dentro", "fuera":
		case "":
			cu.HorarioModo = "dentro"
		default:
			return fmt.Errorf("El horario solo puede valer dentro o fuera de sus horas.")
		}
	} else {
		cu.HorarioModo = ""
	}
	if len(cu.Vacaciones) > MaxVacacio || len(cu.Fechas) > MaxFechas {
		return fmt.Errorf("Hay demasiadas fechas.")
	}
	for _, v := range cu.Vacaciones {
		d, e1 := time.Parse("2006-01-02", v.Desde)
		h, e2 := time.Parse("2006-01-02", v.Hasta)
		if e1 != nil || e2 != nil {
			return fmt.Errorf("Una fecha de las vacaciones no es válida.")
		}
		if h.Before(d) {
			return fmt.Errorf("En las vacaciones, la fecha final es antes de la inicial.")
		}
	}
	for _, f := range cu.Fechas {
		if f.Mes < 1 || f.Mes > 12 || f.Dia < 1 || f.Dia > diasDelMes(f.Mes) {
			return fmt.Errorf("Una fecha especial no es válida (día %d, mes %d).", f.Dia, f.Mes)
		}
	}
	if !cu.Siempre && len(cu.Horario) == 0 && len(cu.Vacaciones) == 0 && len(cu.Fechas) == 0 {
		if tipo == almacen.AutoAusencia {
			return fmt.Errorf("Elige cuándo se manda: fuera de horario, vacaciones o una fecha especial.")
		}
		cu.Siempre = true
	}
	q := &c.Quien
	switch q.Personas {
	case "todos", "desconocidos", "contactos":
	case "":
		q.Personas = "todos"
	default:
		return fmt.Errorf("Elige a quién se le manda: todos, solo desconocidos o solo contactos.")
	}
	if !q.Privados && !q.Grupos {
		return fmt.Errorf("Elige dónde se manda: en chats privados, en grupos o en los dos.")
	}
	if len(q.Excepto) > MaxListas || len(q.SoloLista) > MaxListas || len(q.GruposElegidos) > MaxListas {
		return fmt.Errorf("Las listas son demasiado largas.")
	}
	if len(q.SoloLista) > 0 && !q.Privados && !q.Grupos {
		return fmt.Errorf("Elige dónde se manda.")
	}
	r := &c.Repeticion
	switch r.Unidad {
	case "minutos", "horas", "dias":
	case "":
		r.Unidad = "horas"
	default:
		return fmt.Errorf("La repetición se cuenta en minutos, horas o días.")
	}
	if r.Cada < 0 || r.Cada > 365*24*60 {
		return fmt.Errorf("La repetición no es válida.")
	}
	switch tipo {
	case almacen.AutoBienvenida:
		if c.SilencioDias < 0 || c.SilencioDias > 365 {
			return fmt.Errorf("Los días de silencio no son válidos.")
		}
	case almacen.AutoAusencia, almacen.AutoRecibido:
		// Sin límite de repetición mandaría el mismo aviso con cada mensaje: se exige al menos una hora.
		if r.Duracion() < time.Hour {
			return fmt.Errorf("Para no repetirse con cada mensaje, ponle al menos una hora entre un envío y el siguiente a la misma persona.")
		}
	}
	return nil
}

func diasDelMes(m int) int {
	switch m {
	case 2:
		return 29
	case 4, 6, 9, 11:
		return 30
	}
	return 31
}
