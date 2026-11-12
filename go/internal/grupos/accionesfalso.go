package grupos

import (
	"context"
	"sync"
)

// Falso es un AccionesGrupo para las pruebas (de este paquete y de los que lo usan, como silencio y personas): dice si soy admin
// de cada grupo, anota lo que se borró y a quién se expulsó, y puede fallar a pedido. No se usa en la app.
type Falso struct {
	mu sync.Mutex
	// Admin dice, por grupo, si el bot es administrador. Si el grupo no está en el mapa se usa AdminPorDefecto.
	Admin           map[string]bool
	AdminPorDefecto bool
	ErrAdmin        error // lo que devuelve EsAdmin (con el valor false)
	ErrBorrar       error // si no es nil, BorrarMensajeAjeno falla con esto
	ErrExpulsar     error // si no es nil, Expulsar falla con esto

	Borrados   []BorradoFalso
	Expulsados []BorradoFalso
	Consultas  int // veces que se preguntó si soy admin
}

// BorradoFalso es un mensaje borrado o una persona expulsada.
type BorradoFalso struct{ Chat, ID, Persona string }

// NuevoFalso crea un Falso donde el bot es administrador de todo.
func NuevoFalso() *Falso { return &Falso{Admin: map[string]bool{}, AdminPorDefecto: true} }

var _ AccionesGrupo = (*Falso)(nil)

// PonerAdmin dice si el bot es admin de ese grupo.
func (f *Falso) PonerAdmin(chat string, si bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Admin == nil {
		f.Admin = map[string]bool{}
	}
	f.Admin[chat] = si
}

// EsAdmin dice si el bot administra el grupo.
func (f *Falso) EsAdmin(_ context.Context, chat string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Consultas++
	if f.ErrAdmin != nil {
		return false, f.ErrAdmin
	}
	if v, ok := f.Admin[chat]; ok {
		return v, nil
	}
	return f.AdminPorDefecto, nil
}

// BorrarMensajeAjeno anota el borrado (o falla si se pidió).
func (f *Falso) BorrarMensajeAjeno(_ context.Context, chat, msgID, autor string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ErrBorrar != nil {
		return f.ErrBorrar
	}
	f.Borrados = append(f.Borrados, BorradoFalso{chat, msgID, autor})
	return nil
}

// Expulsar anota la expulsión (o falla si se pidió).
func (f *Falso) Expulsar(_ context.Context, chat, persona string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ErrExpulsar != nil {
		return f.ErrExpulsar
	}
	f.Expulsados = append(f.Expulsados, BorradoFalso{Chat: chat, Persona: persona})
	return nil
}

// NumBorrados cuenta los mensajes borrados hasta ahora.
func (f *Falso) NumBorrados() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.Borrados)
}

// NumExpulsados cuenta las expulsiones hechas hasta ahora.
func (f *Falso) NumExpulsados() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.Expulsados)
}
