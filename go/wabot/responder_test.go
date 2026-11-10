package wabot

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

func base() entrada {
	return entrada{Servidor: "s.whatsapp.net", Texto: "hola bot", Clave: "5255@s.whatsapp.net", Antiguedad: time.Second}
}

func TestResponderTextoPrivado(t *testing.T) {
	l := nuevoLimitador()
	if ok, m := l.decidir(base(), t0); !ok {
		t.Fatalf("debía responder: %s", m)
	}
}

func TestNoResponde(t *testing.T) {
	casos := map[string]func(*entrada){
		"propio":   func(e *entrada) { e.EsMio = true },
		"grupo":    func(e *entrada) { e.EsGrupo = true },
		"servidor": func(e *entrada) { e.Servidor = "g.us" },
		"estado":   func(e *entrada) { e.Servidor = "broadcast" },
		"canal":    func(e *entrada) { e.Servidor = "newsletter" },
		"edicion":  func(e *entrada) { e.Edicion = true },
		"viejo":    func(e *entrada) { e.Antiguedad = 3 * time.Minute },
		"vacio":    func(e *entrada) { e.Texto = "   " },
		"sinclave": func(e *entrada) { e.Clave = "" },
	}
	for nombre, mod := range casos {
		e := base()
		mod(&e)
		if ok, _ := nuevoLimitador().decidir(e, t0); ok {
			t.Errorf("%s: no debía responder", nombre)
		}
	}
}

func TestLID(t *testing.T) {
	e := base()
	e.Servidor = "lid"
	if ok, m := nuevoLimitador().decidir(e, t0); !ok {
		t.Fatalf("un chat privado con LID sí se responde: %s", m)
	}
}

func TestPausa60Segundos(t *testing.T) {
	l := nuevoLimitador()
	if ok, _ := l.decidir(base(), t0); !ok {
		t.Fatal("la primera debía responderse")
	}
	if ok, _ := l.decidir(base(), t0.Add(59*time.Second)); ok {
		t.Fatal("a los 59 s no debía responder")
	}
	if ok, _ := l.decidir(base(), t0.Add(60*time.Second)); !ok {
		t.Fatal("a los 60 s ya debía responder")
	}
	otro := base()
	otro.Clave = "5266@s.whatsapp.net"
	if ok, _ := l.decidir(otro, t0.Add(61*time.Second)); !ok {
		t.Fatal("otro contacto no comparte la pausa")
	}
}

func TestIgnoradoNoGastaPausa(t *testing.T) {
	l := nuevoLimitador()
	g := base()
	g.EsGrupo = true
	l.decidir(g, t0)
	if ok, _ := l.decidir(base(), t0); !ok {
		t.Fatal("un mensaje ignorado no debe consumir la pausa")
	}
}

func TestFormatoCodigo(t *testing.T) {
	if got := formatoCodigo("abcd1234"); got != "ABCD-1234" {
		t.Fatalf("got %s", got)
	}
}

func TestSoloDigitosYMascara(t *testing.T) {
	if got := soloDigitos("+52 (55) 1234-5678"); got != "525512345678" {
		t.Fatalf("got %s", got)
	}
	if got := enmascara("5215512345678:12@s.whatsapp.net"); got != "…5678" {
		t.Fatalf("got %s", got)
	}
}
