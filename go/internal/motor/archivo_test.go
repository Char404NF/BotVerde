package motor

import (
	"context"
	"sync"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/permisos"
)

// enviadorConSalidas es el enviador falso, que además recibe archivos con EnviarSalida.
type enviadorConSalidas struct {
	*enviadorFalso
	mu2    sync.Mutex
	salida []Salida
	chats  []string
	citas  []string
}

func (e *enviadorConSalidas) EnviarSalida(ctx context.Context, chat string, s Salida, cita, autor, mencion string) (string, error) {
	e.mu2.Lock()
	e.salida = append(e.salida, s)
	e.chats = append(e.chats, chat)
	e.citas = append(e.citas, cita)
	e.mu2.Unlock()
	return "ID", nil
}

func (e *enviadorConSalidas) cuantas() int {
	e.mu2.Lock()
	defer e.mu2.Unlock()
	return len(e.salida)
}

func TestUnComandoPuedeResponderConUnArchivo(t *testing.T) {
	env := &enviadorConSalidas{enviadorFalso: &enviadorFalso{}}
	var perm *permisos.Servicio
	e := nuevoEntorno(t, func(c *Config) {
		p, err := permisos.Nuevo(c.Alm, c.BotID)
		if err != nil {
			t.Fatal(err)
		}
		perm = p
		d := comandos.Nuevo(c.Alm, c.BotID, p)
		_ = d.Registrar(comandos.Comando{Nombre: "foto", Casilla: "util.sticker", Ejecutar: func(cx *comandos.Contexto) (string, error) {
			cx.Archivo = &comandos.Archivo{ID: "0123456789abcdef01234567", Tipo: "sticker", Mime: "image/webp", Nombre: "sticker.webp"}
			return "", nil
		}})
		c.Perm, c.Cmd, c.Enviador = p, d, env
	})
	// el dueño, desde Mensajes guardados
	e.m.Entrar(miMensaje("D1", "!foto"))
	e.espera(t, func() bool { return env.cuantas() == 1 })
	env.mu2.Lock()
	s := env.salida[0]
	if s.Medio == nil || s.Medio.Tipo != "sticker" || s.Medio.Mime != "image/webp" || s.Medio.ID != "0123456789abcdef01234567" || env.chats[0] != propio || env.citas[0] != "D1" {
		t.Fatalf("%+v %v %v", s, env.chats, env.citas)
	}
	env.mu2.Unlock()
	// una imagen de Mensajes guardados con el comando de pie de foto también es comando del dueño (no es de tipo texto)
	img := Mensaje{Chat: propio, ID: "D2", Texto: "!foto", EsTexto: false, EsMio: true, EsChatPropio: true, TS: time.Now()}
	e.m.Entrar(img)
	e.espera(t, func() bool { return env.cuantas() == 2 })
	// otro rango con la casilla: la respuesta sale a su chat, etiquetada como comando
	id, err := perm.GuardarRol(almacen.Rol{Nombre: "Cliente", Casillas: map[string]almacen.Alcance{"util.sticker": {}}, LimiteMin: 30})
	if err != nil {
		t.Fatal(err)
	}
	_ = e.alm.AsignarRol(1, pnAna, id, 0, "yo")
	e.m.Entrar(priv(pnAna, "A1", "!foto"))
	e.espera(t, func() bool { return env.cuantas() == 3 })
	env.mu2.Lock()
	defer env.mu2.Unlock()
	if env.chats[2] != pnAna || env.citas[2] != "A1" || env.salida[2].Etiqueta != "comando" || env.salida[2].Medio == nil {
		t.Fatalf("%+v %v", env.salida[2], env.chats)
	}
	// y sin la casilla, nada sale
}

func TestUnComandoSinPermisoNoMandaArchivo(t *testing.T) {
	env := &enviadorConSalidas{enviadorFalso: &enviadorFalso{}}
	e := nuevoEntorno(t, func(c *Config) {
		p, _ := permisos.Nuevo(c.Alm, c.BotID)
		d := comandos.Nuevo(c.Alm, c.BotID, p)
		_ = d.Registrar(comandos.Comando{Nombre: "foto", Casilla: "util.sticker", Ejecutar: func(cx *comandos.Contexto) (string, error) {
			cx.Archivo = &comandos.Archivo{ID: "0123456789abcdef01234567", Tipo: "sticker"}
			return "", nil
		}})
		c.Perm, c.Cmd, c.Enviador = p, d, env
	})
	e.m.Entrar(priv(pnLuis, "L1", "!foto"))
	e.espera(t, func() bool { return env.cuantas() == 1 })
	env.mu2.Lock()
	defer env.mu2.Unlock()
	if env.salida[0].Medio != nil || env.salida[0].Texto == "" {
		t.Fatalf("debía ser solo el texto de permiso: %+v", env.salida[0])
	}
}
