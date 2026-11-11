// Package grupos reúne lo que el bot puede hacer en un grupo siendo administrador. La etapa B (advertencias) solo necesita saber si
// soy administrador y, cuando llega al tope, borrar un mensaje ajeno o expulsar: lo declara aquí con una implementación nula que
// responde "no disponible". La etapa E (grupos) rellena Borrar y Expulsar con la API real; hasta entonces solo se alerta al dueño.
package grupos

import (
	"context"
	"errors"
)

// ErrNoDisponible: esa acción todavía no existe (la implementación nula, o WhatsApp no la ofrece ahora).
var ErrNoDisponible = errors.New("no disponible")

// AccionesGrupo son las acciones de administrador sobre un grupo. `chat` es la clave del grupo; `autor` y `persona` son claves de persona.
type AccionesGrupo interface {
	// EsAdmin dice si el bot (esta cuenta) administra el grupo.
	EsAdmin(ctx context.Context, chat string) (bool, error)
	// BorrarMensajeAjeno borra para todos un mensaje de otra persona (solo siendo administrador).
	BorrarMensajeAjeno(ctx context.Context, chat, msgID, autor string) error
	// Expulsar saca a una persona del grupo (solo siendo administrador).
	Expulsar(ctx context.Context, chat, persona string) error
}

// Nula no puede nada: EsAdmin dice que no y las acciones devuelven ErrNoDisponible.
type Nula struct{}

func (Nula) EsAdmin(context.Context, string) (bool, error)                    { return false, ErrNoDisponible }
func (Nula) BorrarMensajeAjeno(context.Context, string, string, string) error { return ErrNoDisponible }
func (Nula) Expulsar(context.Context, string, string) error                   { return ErrNoDisponible }
