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
}
