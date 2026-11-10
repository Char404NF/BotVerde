package wabot

import (
	"context"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/types"
)

// servicios conecta la ingesta con los trabajos de segundo plano.
type servicios struct{ s *estado }

func (v servicios) PedirFoto(chat string)           { v.s.fotos.Pedir(chat) }
func (v servicios) PedirAsunto(chat string)         { v.s.grupos.pedir(chat) }
func (v servicios) EncolarDescarga(chat, id string) { v.s.descargas.Encolar(chat, id) }

// colaGrupos pide el asunto de los grupos nuevos, de uno en uno y con pausa.
type colaGrupos struct {
	s     *estado
	mu    sync.Mutex
	en    map[string]bool
	canal chan string
}

func nuevaColaGrupos(s *estado) *colaGrupos {
	return &colaGrupos{s: s, en: map[string]bool{}, canal: make(chan string, 512)}
}

func (c *colaGrupos) pedir(chat string) {
	c.mu.Lock()
	if c.en[chat] {
		c.mu.Unlock()
		return
	}
	c.en[chat] = true
	c.mu.Unlock()
	select {
	case c.canal <- chat:
	default:
		c.mu.Lock()
		delete(c.en, chat)
		c.mu.Unlock()
	}
}

func (c *colaGrupos) iniciar(ctx context.Context) {
	c.s.wg.Add(1)
	go func() {
		defer c.s.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case chat := <-c.canal:
				c.una(ctx, chat)
				select {
				case <-ctx.Done():
					return
				case <-time.After(2 * time.Second):
				}
			}
		}
	}()
}

func (c *colaGrupos) una(ctx context.Context, chat string) {
	defer func() {
		c.mu.Lock()
		delete(c.en, chat)
		c.mu.Unlock()
	}()
	if !c.s.cli.IsConnected() || !c.s.cli.IsLoggedIn() {
		return
	}
	j, err := types.ParseJID(chat)
	if err != nil {
		return
	}
	cx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	info, err := c.s.cli.GetGroupInfo(cx, j)
	if err != nil || info == nil {
		c.s.log("no se pudo leer el asunto de un grupo")
		return
	}
	c.s.ing.Asunto(chat, info.Name)
}
