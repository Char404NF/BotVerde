package modulos

import (
	"sync"
	"time"
)

// estadoH es lo que los módulos H recuerdan en memoria entre un evento y otro (se pierde al reiniciar la app a propósito: todo lo que
// importa está en la base).
type estadoH struct {
	mu          sync.Mutex
	privCola    int           // mensajes privados de bienvenida en la cola
	agregarCola int           // «agregar directo» en la cola
	difEncolado int           // mensajes de difusión en la cola
	difFallos   map[int64]int // errores seguidos por difusión
	difPrimero  time.Time     // cuándo salió el primer mensaje de difusión (calentamiento)
	usos        []time.Time   // usos de !bajar y !api en la última hora
	lim         map[string][]time.Time
	wg          sync.WaitGroup // consultas a la IA en curso
}

// limite dice si todavía cabe un uso más de la clave dentro de la ventana (y lo cuenta). tope <= 0 = sin límite.
func (g *Gestor) limite(clave string, tope int, ventana time.Duration) bool {
	if tope <= 0 {
		return true
	}
	g.h.mu.Lock()
	defer g.h.mu.Unlock()
	if g.h.lim == nil {
		g.h.lim = map[string][]time.Time{}
	}
	ahora := g.ahora()
	var vivos []time.Time
	for _, t := range g.h.lim[clave] {
		if ahora.Sub(t) < ventana {
			vivos = append(vivos, t)
		}
	}
	if len(vivos) >= tope {
		g.h.lim[clave] = vivos
		return false
	}
	g.h.lim[clave] = append(vivos, ahora)
	return true
}

// EsperarH espera a que terminen las consultas en segundo plano de los módulos (las pruebas lo usan).
func (g *Gestor) EsperarH() { g.h.wg.Wait() }
