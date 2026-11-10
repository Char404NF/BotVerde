package wabot

import (
	"encoding/json"
	"sync"
	"time"
)

// Observador recibe los cambios en vivo de los datos (para que la pantalla se actualice sola).
// OnCambio recibe un arreglo JSON de eventos: "chats", "chat:<jid>", "archivo:<jid>:<id>" y "espacio".
type Observador interface {
	OnCambio(eventosJSON string)
}

// agrupador junta los eventos que llegan seguidos y los entrega de una vez.
type agrupador struct {
	mu    sync.Mutex
	pend  map[string]struct{}
	orden []string
	t     *time.Timer
	hasta time.Duration
	envia func(string)
}

func nuevoAgrupador(hasta time.Duration, envia func(string)) *agrupador {
	return &agrupador{pend: map[string]struct{}{}, hasta: hasta, envia: envia}
}

func (a *agrupador) Agregar(ev string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.pend[ev]; !ok {
		a.pend[ev] = struct{}{}
		a.orden = append(a.orden, ev)
	}
	if a.t == nil {
		a.t = time.AfterFunc(a.hasta, a.vacia)
	}
}

func (a *agrupador) vacia() {
	a.mu.Lock()
	evs := a.orden
	a.orden = nil
	a.pend = map[string]struct{}{}
	a.t = nil
	a.mu.Unlock()
	if len(evs) == 0 {
		return
	}
	b, _ := json.Marshal(evs)
	a.envia(string(b))
}
