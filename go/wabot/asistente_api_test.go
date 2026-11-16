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

func TestApiBotParaTodosSeAplicaConAvatarMenuYRangoDeTodos(t *testing.T) {
	_, a := abreSolo(t)
	e := decodifica[estadoAsistente](t, func() (string, error) { return AsistenteBase(1) })
	if e.Borrador == nil || !e.Borrador.Menu.Fabrica || len(e.Borrador.Bienvenida.Adjuntos) != 1 || e.Borrador.Bienvenida.Adjuntos[0].Tipo != "imagen" {
		t.Fatalf("el borrador trae el menú de fábrica y la cara del avatar: %+v", e.Borrador)
	}
	// el teléfono guarda el borrador sin el campo «fabrica»: se conserva si no cambió las opciones, y se pierde si las cambió
	sinCampo := *e.Borrador
	sinCampo.Menu.Fabrica = false
	raw, _ := json.Marshal(sinCampo)
	if err := AsistenteGuardar(1, string(raw)); err != nil {
		t.Fatal(err)
	}
	if e = decodifica[estadoAsistente](t, func() (string, error) { return AsistenteEstado(1) }); !e.Borrador.Menu.Fabrica {
		t.Fatal("sin cambios en las opciones sigue siendo el menú de fábrica")
	}
	if _, err := AsistenteAplicar(1); err != nil {
		t.Fatal(err)
	}
	ms, _ := a.ListarMenus(1)
	if len(ms) != 1 || !strings.Contains(string(ms[0].Def), "Bot Chat") {
		t.Fatalf("menús: %+v", ms)
	}
	r, _ := a.RolPorID(1, int64(a.AjusteBotInt(1, "usuario_rol")))
	if r == nil || len(r.Casillas) < 7 {
		t.Fatalf("el rango de todos queda completo: %+v", r)
	}
	if a.AjusteBot(1, "lab_modo_menu") != "botones" {
		t.Fatal("los botones van encendidos por defecto (solo en privado)")
	}
}

func TestApiCambiarLasOpcionesDelMenuQuitaElMenuDeFabrica(t *testing.T) {
	abreSolo(t)
	e := decodifica[estadoAsistente](t, func() (string, error) { return AsistenteBase(1) })
	e.Borrador.Menu.Opciones[0].Texto = "Mi opción"
	e.Borrador.Menu.Fabrica = false
	raw, _ := json.Marshal(e.Borrador)
	if err := AsistenteGuardar(1, string(raw)); err != nil {
		t.Fatal(err)
	}
	if e = decodifica[estadoAsistente](t, func() (string, error) { return AsistenteEstado(1) }); e.Borrador.Menu.Fabrica {
		t.Fatal("al editar las opciones se usan las del usuario")
	}
}
