package motor

import (
	"context"
	"sync"
	"testing"
	"time"

	"botchat/internal/almacen"
)

// compuerta hace que Dormir de 1 h o más (la espera humana de salida) se quede esperando hasta abrirla; lo demás pasa de largo.
type compuerta struct {
	abre chan struct{}
	mu   sync.Mutex
	dur  []time.Duration
}

func nuevaCompuerta() *compuerta { return &compuerta{abre: make(chan struct{})} }

func (g *compuerta) dormir(ctx context.Context, d time.Duration) bool {
	g.mu.Lock()
	g.dur = append(g.dur, d)
	g.mu.Unlock()
	if d < time.Hour {
		return true
	}
	select {
	case <-g.abre:
		return true
	case <-ctx.Done():
		return false
	}
}

func TestSiContesteYoSeCancelaLaRespuestaPendienteEnLaFila(t *testing.T) {
	g := nuevaCompuerta()
	bloqueaDebounce := make(chan struct{})
	e := nuevoEntorno(t, func(c *Config) {
		c.Dormir = func(ctx context.Context, d time.Duration) bool {
			if d < time.Hour { // el debounce del privado espera aquí hasta que yo escriba
				select {
				case <-bloqueaDebounce:
					return true
				case <-ctx.Done():
					return false
				}
			}
			return g.dormir(ctx, d)
		}
	})
	m := priv("5215500000001@s.whatsapp.net", "a1", "hola")
	e.m.Entrar(m)
	e.m.Intervino(m.Chat, m.TS.Add(2*time.Second)) // contesté yo, después de su mensaje
	close(bloqueaDebounce)
	e.espera(t, func() bool { return e.motivos()[almacen.MotYaContesto] >= 1 })
	e.quieto()
	if n := len(e.env.lista()); n != 0 {
		t.Fatalf("no debía responder: %d envíos", n)
	}
	e.dec.mu.Lock()
	lotes := len(e.dec.lotes)
	e.dec.mu.Unlock()
	if lotes != 0 {
		t.Fatalf("ni siquiera debía decidir: %d", lotes)
	}
}

func TestSiContesteYoSeCancelaLaRespuestaYaLista(t *testing.T) {
	g := nuevaCompuerta()
	e := nuevoEntorno(t, func(c *Config) {
		c.EsperaMin, c.EsperaMax = time.Hour, 2*time.Hour
		c.Dormir = g.dormir
	})
	m := priv("5215500000001@s.whatsapp.net", "a1", "hola")
	e.m.Entrar(m)
	// La respuesta ya está decidida y esperando su turno de salida ("escribiendo…").
	e.espera(t, func() bool { e.env.mu.Lock(); defer e.env.mu.Unlock(); return e.env.escr > 0 })
	e.m.Intervino(m.Chat, m.TS.Add(time.Second))
	close(g.abre)
	e.espera(t, func() bool { return e.motivos()[almacen.MotYaContesto] >= 1 })
	e.quieto()
	if n := len(e.env.lista()); n != 0 {
		t.Fatalf("no debía enviarse: %d", n)
	}
}

func TestUnMensajePosteriorDeLaPersonaSiSeContesta(t *testing.T) {
	e := nuevoEntorno(t, nil)
	chat := "5215500000001@s.whatsapp.net"
	ahora := time.Now()
	e.m.Intervino(chat, ahora.Add(-time.Minute)) // escribí hace un minuto
	m := priv(chat, "b1", "otra cosa")
	m.TS = ahora
	e.m.Entrar(m)
	e.espera(t, func() bool { return len(e.env.lista()) == 1 })
}

func TestSoloSeCancelaElChatDondeEscribi(t *testing.T) {
	e := nuevoEntorno(t, nil)
	a, b := "5215500000001@s.whatsapp.net", "5215500000002@s.whatsapp.net"
	e.m.Intervino(a, time.Now().Add(time.Hour))
	m := priv(b, "c1", "hola")
	e.m.Entrar(m)
	e.espera(t, func() bool { return len(e.env.lista()) == 1 })
}

func TestRetrasoHumanoEntre08y25SinEsperasExtra(t *testing.T) {
	var mu sync.Mutex
	var durs []time.Duration
	var respuesta string
	for i := 0; i < 2000; i++ {
		respuesta += "palabra "
	}
	e := nuevoEntorno(t, func(c *Config) {
		c.EsperaMin, c.EsperaMax = 800*time.Millisecond, 2500*time.Millisecond
		c.Aleatorio = func(n int64) int64 { return n - 1 } // el peor caso
		c.Dormir = func(_ context.Context, d time.Duration) bool {
			mu.Lock()
			durs = append(durs, d)
			mu.Unlock()
			return true
		}
		c.Decisor = decisorFijo{respuesta}
	})
	e.m.Entrar(priv("5215500000001@s.whatsapp.net", "d1", "hola"))
	e.espera(t, func() bool { return len(e.env.lista()) == 1 })
	mu.Lock()
	defer mu.Unlock()
	var mayor time.Duration
	for _, d := range durs {
		if d > mayor {
			mayor = d
		}
	}
	// El debounce del privado son 1,5 s como mucho 6 s en total; la espera de salida nunca pasa de 2,5 s aunque el texto sea larguísimo.
	var salida []time.Duration
	for _, d := range durs {
		if d >= 800*time.Millisecond && d <= 2500*time.Millisecond {
			salida = append(salida, d)
		}
	}
	if len(salida) == 0 {
		t.Fatalf("no hubo espera de salida: %v", durs)
	}
	for _, d := range salida {
		if d > 2500*time.Millisecond {
			t.Fatalf("espera de más: %v", d)
		}
	}
	if mayor > 2500*time.Millisecond && mayor != 0 {
		// la única espera grande permitida sería el debounce, que es de 1,5 s por defecto; nada de sumas por carácter
		t.Fatalf("hay una espera mayor a la humana: %v (%v)", mayor, durs)
	}
}

type decisorFijo struct{ texto string }

func (d decisorFijo) Decidir(context.Context, Mensaje, []Mensaje) (Decision, error) {
	return Decision{Coincidio: "fijo", Respuestas: []Salida{{Texto: d.texto}}}, nil
}

func TestFirmaOpcionalSoloEnRespuestasDelBot(t *testing.T) {
	firma := ""
	e := nuevoEntorno(t, func(c *Config) { c.Firma = func() string { return firma } })
	e.m.Entrar(priv("5215500000001@s.whatsapp.net", "f1", "uno"))
	e.espera(t, func() bool { return len(e.env.lista()) == 1 })
	if got := e.env.lista()[0].texto; got != "eco uno" {
		t.Fatalf("sin firma el texto no cambia: %q", got)
	}
	firma = "  — Asistente automático  "
	e.m.Entrar(priv("5215500000003@s.whatsapp.net", "f2", "dos"))
	e.espera(t, func() bool { return len(e.env.lista()) == 2 })
	if got := e.env.lista()[1].texto; got != "eco dos\n\n— Asistente automático" {
		t.Fatalf("firma al final: %q", got)
	}
}
