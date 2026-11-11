package modulos

import (
	"errors"
	"regexp"
	"sync"

	"botchat/internal/permisos"
)

// Grupo del catálogo de permisos donde caen las casillas de módulos que no traen el suyo.
const GrupoCasillas = "Módulos"

var (
	regMu  sync.RWMutex
	regOrd []string
	regMap = map[string]Modulo{}
	reID   = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
)

// Errores del registro.
var (
	ErrIDInvalido = errors.New("el id del módulo debe ir en minúsculas, sin espacios (por ejemplo \"bienvenida\")")
	ErrNoExiste   = errors.New("ese módulo no existe")
)

// Registrar agrega un módulo al registro central y sus casillas al catálogo de permisos. Si ya había uno con ese id, lo reemplaza.
func Registrar(m Modulo) error {
	if m == nil || !reID.MatchString(m.ID()) {
		return ErrIDInvalido
	}
	regMu.Lock()
	if _, ok := regMap[m.ID()]; !ok {
		regOrd = append(regOrd, m.ID())
	}
	regMap[m.ID()] = m
	regMu.Unlock()
	asegurarCasillas(m)
	return nil
}

// asegurarCasillas deja en el catálogo las casillas del módulo (se puede repetir sin efecto).
func asegurarCasillas(m Modulo) {
	cs := m.Casillas()
	if len(cs) == 0 {
		return
	}
	out := make([]permisos.Casilla, 0, len(cs))
	for _, c := range cs {
		c.Modulo = m.ID()
		if c.Grupo == "" {
			c.Grupo = GrupoCasillas
		}
		if c.Riesgo == "" {
			c.Riesgo = m.Riesgo()
		}
		out = append(out, c)
	}
	permisos.Registrar(out...)
}

// Quitar saca un módulo del registro y sus casillas del catálogo (lo usan las pruebas).
func Quitar(id string) {
	regMu.Lock()
	m := regMap[id]
	delete(regMap, id)
	n := regOrd[:0]
	for _, o := range regOrd {
		if o != id {
			n = append(n, o)
		}
	}
	regOrd = n
	regMu.Unlock()
	if m != nil {
		for _, c := range m.Casillas() {
			permisos.Quitar(c.ID)
		}
	}
}

// Obtener devuelve un módulo por id.
func Obtener(id string) (Modulo, bool) {
	regMu.RLock()
	defer regMu.RUnlock()
	m, ok := regMap[id]
	return m, ok
}

// Lista devuelve los módulos en el orden en que se registraron.
func Lista() []Modulo {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]Modulo, 0, len(regOrd))
	for _, id := range regOrd {
		out = append(out, regMap[id])
	}
	return out
}
