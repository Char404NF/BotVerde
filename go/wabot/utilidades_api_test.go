package wabot

import (
	"strings"
	"testing"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/permisos"
)

func TestApiUtilidadesNotasYPendientesDelDueno(t *testing.T) {
	abreSolo(t)
	id1, err := AgregarUtilidad(1, "nota", "  llamar al proveedor ")
	if err != nil || id1 == 0 {
		t.Fatal(id1, err)
	}
	id2, _ := AgregarUtilidad(1, "pendiente", "pagar la renta")
	_, _ = AgregarUtilidad(1, "pendiente", "enviar factura")
	if _, err := AgregarUtilidad(1, "otra", "x"); err == nil {
		t.Fatal("tipo malo")
	}
	if _, err := AgregarUtilidad(1, "nota", "   "); err == nil {
		t.Fatal("vacía")
	}
	if _, err := ListarUtilidades(1, "x"); err == nil {
		t.Fatal("tipo malo")
	}
	notas := decodifica[[]almacen.ItemUtil](t, func() (string, error) { return ListarUtilidades(1, "nota") })
	if len(notas) != 1 || notas[0].Texto != "llamar al proveedor" || notas[0].ID != id1 {
		t.Fatalf("%+v", notas)
	}
	if err := MarcarUtilidad(1, id2, true); err != nil {
		t.Fatal(err)
	}
	if err := MarcarUtilidad(1, 99999, true); err == nil {
		t.Fatal("no existe")
	}
	pend := decodifica[[]almacen.ItemUtil](t, func() (string, error) { return ListarUtilidades(1, "pendiente") })
	if len(pend) != 2 || !pend[0].Hecho || pend[1].Hecho {
		t.Fatalf("%+v", pend)
	}
	if n, err := LimpiarUtilidadesHechas(1); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if err := BorrarUtilidad(1, id1); err != nil {
		t.Fatal(err)
	}
	if l := decodifica[[]almacen.ItemUtil](t, func() (string, error) { return ListarUtilidades(1, "nota") }); len(l) != 0 {
		t.Fatalf("%+v", l)
	}
	// los ajustes de utilidades se validan
	if err := FijarAjusteDePermisos(1, "util_tope", "0"); err == nil {
		t.Fatal("el tope mínimo es 1")
	}
	if err := FijarAjusteDePermisos(1, "util_tope", "20"); err != nil {
		t.Fatal(err)
	}
	if err := FijarAjusteDePermisos(1, "sticker_tope_hora", "0"); err != nil {
		t.Fatal(err)
	}
	if err := FijarAjusteDePermisos(1, "sticker_tope_hora", "5000"); err == nil {
		t.Fatal("demasiado")
	}
}

func TestArmarUtilidadesRegistraSusComandos(t *testing.T) {
	_, a := abreSolo(t)
	p, err := permisos.Nuevo(a, 1)
	if err != nil {
		t.Fatal(err)
	}
	s := &estado{botID: 1, cmd: comandos.Nuevo(a, 1, p)}
	s.armarUtilidades()
	for _, c := range []string{"anotar", "notas", "borrarnota", "tarea", "tareas", "hecho", "deshacer", "borrartarea", "limpiartareas", "sticker"} {
		if !s.cmd.Existe("!" + c) {
			t.Errorf("falta !%s", c)
		}
	}
	// el dueño usa las notas igual que desde la app: lo escrito por comando aparece en la lista de la API
	r := s.cmd.Ejecutar(comandos.Entrada{Persona: almacen.PersonaDueno, Chat: "yo@s.whatsapp.net", Texto: "!tarea revisar bodega", Dueno: true, MensajeID: "M"})
	if !strings.Contains(r.Texto, "número 1") {
		t.Fatalf("%q", r.Texto)
	}
	l := decodifica[[]almacen.ItemUtil](t, func() (string, error) { return ListarUtilidades(1, "pendiente") })
	if len(l) != 1 || l[0].Texto != "revisar bodega" {
		t.Fatalf("%+v", l)
	}
}
