package flujo

import (
	"strings"
	"testing"
)

// El menú trae sus partes (título y opciones con su número) además del texto numerado, que sigue siendo el respaldo.
func TestMenuTrazaSusPartesParaElLaboratorio(t *testing.T) {
	e := nuevo(t)
	e.crea(t, ventas(), true)
	d := e.dice(t, ana, ana, "quiero ventas")
	m := d.Respuestas[0].Menu
	if m == nil || m.Persona != ana || m.Titulo != "Hola Ana, son las 05:30. ¿Qué buscas?" || len(m.Opciones) != 3 ||
		m.Opciones[0].Num != 1 || m.Opciones[0].Texto != "Precios" || m.Opciones[2].Texto != "Dejar un dato" {
		t.Fatalf("partes del menú: %+v", m)
	}
	if d.Respuestas[0].Texto != "Hola Ana, son las 05:30. ¿Qué buscas?\n1. Precios\n2. Contacto\n3. Dejar un dato" {
		t.Fatalf("el texto numerado no cambia: %q", d.Respuestas[0].Texto)
	}
	// un submenú trae el "0. Volver"
	d = e.dice(t, ana, ana, "1")
	m = d.Respuestas[0].Menu
	if m == nil || len(m.Opciones) != 2 || m.Opciones[1].Num != 0 || m.Opciones[1].Texto != "Volver al menú principal" {
		t.Fatalf("submenú: %+v", m)
	}
	// una respuesta final no es un menú
	d = e.dice(t, ana, ana, "1")
	if d.Respuestas[0].Menu != nil {
		t.Fatalf("una respuesta no lleva partes de menú: %+v", d.Respuestas[0].Menu)
	}
}

// El formato de un menú y el botón especial de una opción llegan al Laboratorio; sin ellos todo sigue igual.
func TestMenuLlevaSuFormatoYSusBotonesEspeciales(t *testing.T) {
	e := nuevo(t)
	d := ventas()
	d.Pasos[0].Formato = "botones"
	d.Pasos[0].Opciones[0].Boton = &BotonOpcion{Tipo: "enlace", Valor: "https://ejemplo.com/precios"}
	d.Pasos[0].Opciones[1].Boton = &BotonOpcion{Tipo: "llamar", Valor: "+52 55 1234 5678"}
	e.crea(t, d, true)
	r := e.dice(t, ana, ana, "quiero ventas")
	m := r.Respuestas[0].Menu
	if m == nil || m.Formato != "botones" || m.Opciones[0].BotonTipo != "enlace" || m.Opciones[0].BotonValor != "https://ejemplo.com/precios" ||
		m.Opciones[1].BotonTipo != "llamar" || m.Opciones[2].BotonTipo != "" {
		t.Fatalf("partes del menú: %+v", m)
	}
	// el texto numerado no cambia: sigue siendo el respaldo
	if r.Respuestas[0].Texto != "Hola Ana, son las 05:30. ¿Qué buscas?\n1. Precios\n2. Contacto\n3. Dejar un dato" {
		t.Fatalf("%q", r.Respuestas[0].Texto)
	}
}

func TestValidarFormatoYBotonesEspeciales(t *testing.T) {
	grave := func(d *Def) string {
		for _, a := range d.Validar() {
			if a.Grave {
				return a.Texto
			}
		}
		return ""
	}
	malos := map[string]func(*Def){
		"formato del menú no es válido": func(d *Def) { d.Pasos[0].Formato = "lista" },
		"https://":                      func(d *Def) { d.Pasos[0].Opciones[0].Boton = &BotonOpcion{Tipo: "enlace", Valor: "ejemplo.com"} },
		"copiar":                        func(d *Def) { d.Pasos[0].Opciones[0].Boton = &BotonOpcion{Tipo: "copiar", Valor: ""} },
		"7 a 15 dígitos":                func(d *Def) { d.Pasos[0].Opciones[0].Boton = &BotonOpcion{Tipo: "llamar", Valor: "123"} },
		"solo lleva dígitos":            func(d *Def) { d.Pasos[0].Opciones[0].Boton = &BotonOpcion{Tipo: "llamar", Valor: "55abc12345"} },
		"enlace, copiar o llamar":       func(d *Def) { d.Pasos[0].Opciones[0].Boton = &BotonOpcion{Tipo: "volar", Valor: "x"} },
	}
	for esperado, f := range malos {
		d := ventas()
		d.Normalizar()
		f(d)
		if g := grave(d); !strings.Contains(g, esperado) {
			t.Errorf("esperaba un error con %q, llegó %q", esperado, g)
		}
	}
	d := ventas()
	d.Normalizar()
	d.Pasos[0].Formato = "encuesta"
	d.Pasos[0].Opciones[0].Boton = &BotonOpcion{Tipo: "copiar", Valor: "CLABE 0123"}
	d.Pasos[0].Opciones[1].Boton = &BotonOpcion{Tipo: "llamar", Valor: "+52 55 1234 5678"}
	if g := grave(d); g != "" {
		t.Fatalf("lo válido no debe dar error: %q", g)
	}
}

func contiene(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indice(s, sub) >= 0)
}
func indice(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
