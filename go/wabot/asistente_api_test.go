package wabot

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestApiAsistenteGuardarAplicarDeshacer(t *testing.T) {
	abreSolo(t)
	e := decodifica[estadoAsistente](t, func() (string, error) { return AsistenteEstado(1) })
	if e.Borrador == nil || e.Aplicado || e.Visto || len(e.Borrador.Menu.Opciones) != 3 || e.NumeroPrb != "" {
		t.Fatalf("estado inicial: %+v", e)
	}
	e.Borrador.Paso = 3
	e.Borrador.Bienvenida.Texto = "Hola {nombre}, bienvenido"
	e.Borrador.Util.Notas = false
	raw, _ := json.Marshal(e.Borrador)
	if err := AsistenteGuardar(1, string(raw)); err != nil {
		t.Fatal(err)
	}
	e = decodifica[estadoAsistente](t, func() (string, error) { return AsistenteEstado(1) })
	if e.Borrador.Paso != 3 || e.Borrador.Bienvenida.Texto != "Hola {nombre}, bienvenido" {
		t.Fatalf("reanudar: %+v", e.Borrador)
	}
	if _, err := AsistenteAplicar(1); err != nil {
		t.Fatal(err)
	}
	e = decodifica[estadoAsistente](t, func() (string, error) { return AsistenteEstado(1) })
	if !e.Aplicado || !e.Visto {
		t.Fatalf("aplicado: %+v", e)
	}
	// Kotlin manda el borrador sin «creado»: el servidor conserva lo que ya aplicó.
	e.Borrador.Creado = nil
	raw, _ = json.Marshal(e.Borrador)
	if err := AsistenteGuardar(1, string(raw)); err != nil {
		t.Fatal(err)
	}
	if e = decodifica[estadoAsistente](t, func() (string, error) { return AsistenteEstado(1) }); !e.Aplicado {
		t.Fatal("se perdió lo aplicado")
	}
	if err := AsistenteDeshacer(1); err != nil {
		t.Fatal(err)
	}
	if e = decodifica[estadoAsistente](t, func() (string, error) { return AsistenteEstado(1) }); e.Aplicado {
		t.Fatal("debía deshacerse")
	}
	if err := AsistenteGuardar(1, "no es json"); err == nil {
		t.Fatal("debía rechazar")
	}
}

func TestApiAsistenteProbarExplicaSiNoHayOtroNumero(t *testing.T) {
	abreSolo(t)
	_, err := AsistenteProbar(1, "")
	if err == nil || !strings.Contains(err.Error(), "segundo número") {
		t.Fatalf("debía explicar que falta otro número: %v", err)
	}
	if _, err := AsistenteProbar(1, "123"); err == nil || !strings.Contains(err.Error(), "código de país") {
		t.Fatalf("número corto: %v", err)
	}
	if _, err := AsistenteProbar(1, "120363000000000001@g.us"); err == nil || !strings.Contains(err.Error(), "no a un grupo") {
		t.Fatalf("grupo: %v", err)
	}
	// Con un número válido pero sin el bot encendido, pide encenderlo (no manda nada).
	if _, err := AsistenteProbar(1, "5215512345678"); err == nil || !strings.Contains(err.Error(), "Enciende el bot") {
		t.Fatalf("sin bot: %v", err)
	}
}

func TestApiTopePendienteSeConfirmaPorLaApi(t *testing.T) {
	abreSolo(t)
	a, _ := almacenAbierto()
	_ = a.FijarAjusteBot(1, "tope_confirmado", "0")
	if p, _ := TopePendiente(1); !p {
		t.Fatal("pendiente")
	}
	e := decodifica[map[string]any](t, func() (string, error) { return EstadoAlmacenamiento(1) })
	if e["topePendiente"] != true {
		t.Fatalf("el estado debe decirlo: %v", e["topePendiente"])
	}
	r, _ := AplicarTopeAhora(1)
	if !strings.Contains(r, `"borrados":0`) {
		t.Fatal(r)
	}
	if err := ConfirmarTope(1); err != nil {
		t.Fatal(err)
	}
	if p, _ := TopePendiente(1); p {
		t.Fatal("confirmado")
	}
}
