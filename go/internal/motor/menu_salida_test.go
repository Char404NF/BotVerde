package motor

import (
	"testing"
	"time"
)

// Un menú sin archivos ni mensaje automático ligado (por ejemplo, al seguir una conversación) también llega a quien envía con sus
// partes, para que pueda mandarlo con botones.
func TestUnMenuSoloLlegaAlEnviadorConSusPartes(t *testing.T) {
	env := &enviadorConSalidas{enviadorFalso: &enviadorFalso{}}
	e := nuevoEntorno(t, func(c *Config) { c.Enviador = env })
	menu := &MenuLab{Titulo: "¿Qué buscas?", Opciones: []OpcionMenu{{Num: 1, Texto: "Precios"}, {Num: 2, Texto: "Contacto"}}}
	if err := e.m.enviarSalida(pnAna, Salida{Texto: "¿Qué buscas?\n1. Precios\n2. Contacto", Menu: menu}, "M1", "", "", time.Now(), nil); err != nil {
		t.Fatal(err)
	}
	env.mu2.Lock()
	defer env.mu2.Unlock()
	if len(env.salida) != 1 || env.salida[0].Menu == nil || len(env.salida[0].Menu.Opciones) != 2 || env.salida[0].Menu.Opciones[0].Texto != "Precios" {
		t.Fatalf("el menú se perdió por el camino: %+v", env.salida)
	}
}
