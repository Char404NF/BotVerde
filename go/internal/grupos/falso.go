package grupos

import (
	"context"
	"fmt"
	"sync"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// ClienteFalso es un Cliente para las pruebas (de este paquete y de los que lo usan): anota cada llamada con sus argumentos y
// responde con lo que se le configure. No se usa en la app.
type ClienteFalso struct {
	mu         sync.Mutex
	Conexion   bool
	PN, LID    types.JID
	Info       *types.GroupInfo
	Errores    map[string]error // por nombre de método: el error que devuelve
	Pendientes []types.GroupParticipantRequest
	Enlace     string
	Resultado  []types.GroupParticipant // lo que devuelven UpdateGroupParticipants y UpdateGroupRequestParticipants
	Llamadas   []string
	InfoPedida int
}

func (c *ClienteFalso) anota(f string, a ...any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Llamadas = append(c.Llamadas, f+"|"+fmt.Sprint(a...))
	return c.Errores[f]
}

// Hubo dice si se llamó exactamente a esa llamada ("Método|argumentos").
func (c *ClienteFalso) Hubo(llamada string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, l := range c.Llamadas {
		if l == llamada {
			return true
		}
	}
	return false
}

// SinEscrituras dice si no se llamó a nada que cambie el grupo.
func (c *ClienteFalso) SinEscrituras() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, l := range c.Llamadas {
		if len(l) >= 3 && (l[:3] == "Set" || l[:3] == "Upd") {
			return false
		}
	}
	return true
}

func (c *ClienteFalso) Conectado() bool { return c.Conexion }

func (c *ClienteFalso) Propio() (types.JID, types.JID) { return c.PN, c.LID }

func (c *ClienteFalso) GetGroupInfo(_ context.Context, j types.JID) (*types.GroupInfo, error) {
	c.mu.Lock()
	c.InfoPedida++
	err := c.Errores["GetGroupInfo"]
	info := c.Info
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (c *ClienteFalso) SetGroupAnnounce(_ context.Context, j types.JID, v bool) error {
	return c.anota("SetGroupAnnounce", j, "|", v)
}

func (c *ClienteFalso) SetGroupLocked(_ context.Context, j types.JID, v bool) error {
	return c.anota("SetGroupLocked", j, "|", v)
}

func (c *ClienteFalso) SetGroupName(_ context.Context, j types.JID, n string) error {
	return c.anota("SetGroupName", j, "|", n)
}

func (c *ClienteFalso) SetGroupTopic(_ context.Context, j types.JID, prev, nuevo, t string) error {
	return c.anota("SetGroupTopic", j, "|prev=", prev, "|new=", nuevo, "|", t)
}

func (c *ClienteFalso) UpdateGroupParticipants(_ context.Context, j types.JID, ps []types.JID, a whatsmeow.ParticipantChange) ([]types.GroupParticipant, error) {
	if err := c.anota("UpdateGroupParticipants", j, "|", string(a), "|", ps); err != nil {
		return nil, err
	}
	return c.Resultado, nil
}

func (c *ClienteFalso) GetGroupRequestParticipants(_ context.Context, j types.JID) ([]types.GroupParticipantRequest, error) {
	if err := c.anota("GetGroupRequestParticipants", j); err != nil {
		return nil, err
	}
	return c.Pendientes, nil
}

func (c *ClienteFalso) UpdateGroupRequestParticipants(_ context.Context, j types.JID, ps []types.JID, a whatsmeow.ParticipantRequestChange) ([]types.GroupParticipant, error) {
	if err := c.anota("UpdateGroupRequestParticipants", j, "|", string(a), "|", ps); err != nil {
		return nil, err
	}
	return c.Resultado, nil
}

func (c *ClienteFalso) GetGroupInviteLink(_ context.Context, j types.JID, reset bool) (string, error) {
	if err := c.anota("GetGroupInviteLink", j, "|", reset); err != nil {
		return "", err
	}
	return c.Enlace, nil
}

var _ Cliente = (*ClienteFalso)(nil)
