package identidad

import "testing"

func TestTelefonoMexico(t *testing.T) {
	casos := []struct{ entrada, e164, mostrado, iso string }{
		{"5215512345678", "+5215512345678", "+52 55 1234 5678", "MX"}, // con el 1 móvil: se conserva en E164, se oculta al mostrar
		{"525512345678", "+525512345678", "+52 55 1234 5678", "MX"},
		{"+52 (55) 1234-5678", "+525512345678", "+52 55 1234 5678", "MX"},
		{"5215500000002", "+5215500000002", "+52 55 0000 0002", "MX"},
	}
	for _, c := range casos {
		got := ParseTelefono(c.entrada)
		if got.E164 != c.e164 || got.Mostrado != c.mostrado || got.ISO != c.iso {
			t.Errorf("%s: got %+v", c.entrada, got)
		}
	}
}

func TestTelefonoOtrosPaises(t *testing.T) {
	if got := ParseTelefono("12025550123"); got.ISO != "US" || got.Mostrado != "+1 202-555-0123" {
		t.Errorf("US: %+v", got)
	}
	if got := ParseTelefono("34612345678"); got.ISO != "ES" || got.Mostrado != "+34 612 34 56 78" {
		t.Errorf("ES: %+v", got)
	}
	if got := ParseTelefono("442079460958"); got.ISO != "GB" {
		t.Errorf("GB: %+v", got)
	}
}

func TestTelefonoInvalidoOVacio(t *testing.T) {
	if got := ParseTelefono(""); got != (Telefono{}) {
		t.Errorf("vacío: %+v", got)
	}
	// Un número no válido se muestra tal cual, con "+", y sin país.
	got := ParseTelefono("999")
	if got.Mostrado != "+999" || got.ISO != "" || got.E164 != "+999" {
		t.Errorf("inválido: %+v", got)
	}
}

func TestNombreMostrado(t *testing.T) {
	tel := ParseTelefono("5215512345678")
	casos := []struct {
		d    Datos
		want string
	}{
		{Datos{Guardado: "Mamá", Publico: "Ana", Negocio: "Tienda", Telefono: tel.Mostrado}, "Mamá"},
		{Datos{Publico: "Ana", Negocio: "Tienda", Telefono: tel.Mostrado}, "Ana"},
		{Datos{Negocio: "Tienda López", Telefono: tel.Mostrado}, "Tienda López"},
		{Datos{Telefono: tel.Mostrado}, "+52 55 1234 5678"},
		{Datos{Publico: "Luis"}, "Luis"}, // solo LID, con nombre público
		{Datos{}, NumeroNoDisponible},    // solo LID, sin nombre
		{Datos{EsGrupo: true, Asunto: "Familia", Guardado: "x"}, "Familia"},
		{Datos{EsGrupo: true}, "Grupo"},
		{Datos{Guardado: "  Pedro\nPérez  "}, "Pedro Pérez"},
		{Datos{Guardado: "   ", Publico: "Ana"}, "Ana"},
	}
	for i, c := range casos {
		if got := NombreMostrado(c.d); got != c.want {
			t.Errorf("caso %d: got %q want %q", i, got, c.want)
		}
	}
}

func TestTelefonoMostradoParaLID(t *testing.T) {
	if got := TelefonoMostrado(false, Telefono{}); got != NumeroNoDisponible {
		t.Errorf("got %q", got)
	}
	if got := TelefonoMostrado(true, Telefono{}); got != "" {
		t.Errorf("grupo: %q", got)
	}
	if got := TelefonoMostrado(false, ParseTelefono("525512345678")); got != "+52 55 1234 5678" {
		t.Errorf("got %q", got)
	}
}

func TestBusquedaSinAcentos(t *testing.T) {
	if got := Normalizar("  JOSÉ Peña  "); got != "jose pena" {
		t.Errorf("got %q", got)
	}
	if got := TextoBusqueda("María", ParseTelefono("5215512345678")); got != "maria 5215512345678" {
		t.Errorf("got %q", got)
	}
}
