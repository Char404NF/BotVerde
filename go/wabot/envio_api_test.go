package wabot

import (
	"testing"
	"time"
)

func TestPresenciaExpiraElEscribiendo(t *testing.T) {
	presMu.Lock()
	p := presenciaPara("x@s.whatsapp.net")
	p.Escribiendo, p.desde = true, time.Now()
	p.EnLinea, p.UltimaVez = true, 1700000000
	presMu.Unlock()
	if got := PresenciaDe("x@s.whatsapp.net"); got != `{"escribiendo":true,"enLinea":true,"ultimaVez":1700000000}` {
		t.Fatal(got)
	}
	presMu.Lock()
	pres["x@s.whatsapp.net"].desde = time.Now().Add(-time.Minute)
	presMu.Unlock()
	if got := PresenciaDe("x@s.whatsapp.net"); got != `{"escribiendo":false,"enLinea":true,"ultimaVez":1700000000}` {
		t.Fatal("el 'escribiendo' debe apagarse solo:", got)
	}
	if got := PresenciaDe("nadie@s.whatsapp.net"); got != `{"escribiendo":false,"enLinea":false,"ultimaVez":0}` {
		t.Fatal("sin datos no se muestra nada:", got)
	}
}

func TestConfigurarEnvioPausa(t *testing.T) {
	ConfigurarEnvio(45)
	if pausaTrasEscribir() != 45*time.Minute {
		t.Fatal(pausaTrasEscribir())
	}
	ConfigurarEnvio(0)
	if pausaTrasEscribir() != 0 {
		t.Fatal("0 = nunca se pausa")
	}
	ConfigurarEnvio(0) // el valor por defecto de la 0.5.0
}

func TestLaPausaTrasEscribirEmpiezaEnCero(t *testing.T) {
	// El init del paquete la deja en 0: el bot no calla por mí.
	ConfigurarEnvio(0)
	if pausaTrasEscribirMin.Load() != 0 {
		t.Fatal("por defecto no hay pausa")
	}
}
