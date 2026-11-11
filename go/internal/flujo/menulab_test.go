package flujo

import "testing"

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
