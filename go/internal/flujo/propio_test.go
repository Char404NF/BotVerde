package flujo

import (
	"strings"
	"testing"
	"time"
)

func (e *entorno) propio(t *testing.T, dp DatosPropio, activo bool) int64 {
	t.Helper()
	d := &Def{V: 1, Propio: &dp}
	if err := d.ConstruirPropio(); err != nil {
		t.Fatal(err)
	}
	return e.crea(t, d, activo)
}

func TestComandoPropioUsaElPrefijoActual(t *testing.T) {
	e := nuevo(t)
	prefijo := "!"
	e.m.c.Prefijo = func() string { return prefijo }
	e.propio(t, DatosPropio{Nombre: "horario", Texto: "Abrimos a las 9, {nombre}. *Lunes a viernes*.", Resumen: "Nuestros horarios", Privados: true}, true)
	if d := e.dice(t, ana, ana, "horario"); len(d.Respuestas) != 0 {
		t.Fatalf("sin prefijo no es un comando: %q", textos(d))
	}
	d := e.dice(t, ana, ana, "!horario")
	if textos(d) != "Abrimos a las 9, Ana. *Lunes a viernes*." || d.Coincidio != "Comando horario" {
		t.Fatalf("%q %q", textos(d), d.Coincidio)
	}
	// Mayúsculas, acentos y argumentos de más.
	if d = e.dice(t, ana, ana, "!HORARIO por favor"); textos(d) == "" {
		t.Fatal("tolera mayúsculas y texto después")
	}
	// Cambia el prefijo: ahora vale "#horario" y "!horario" ya no.
	prefijo = "#"
	if d = e.dice(t, ana, ana, "!horario"); len(d.Respuestas) != 0 {
		t.Fatalf("el prefijo viejo ya no vale: %q", textos(d))
	}
	if d = e.dice(t, ana, ana, "#horario"); textos(d) == "" {
		t.Fatal("el prefijo nuevo vale")
	}
	// Un comando con su propio símbolo (el de siempre, ".menu") se compara tal cual.
	e.crea(t, ventas(), true)
	if d = e.dice(t, ana, ana, ".menu"); !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("comando con símbolo propio: %q", textos(d))
	}
}

func TestComandoPropioRangoMinimoYGrupos(t *testing.T) {
	e := nuevo(t)
	e.m.c.Prefijo = func() string { return "!" }
	sinRango := map[string]bool{}
	e.m.c.PuedeUsar = func(sender string, rolMin int64) bool { return !sinRango[sender] && rolMin == 7 }
	e.propio(t, DatosPropio{Nombre: "precios", Texto: "Lista de precios", Privados: true, Grupos: true, RangoMin: 7}, true)
	if d := e.dice(t, ana, ana, "!precios"); textos(d) != "Lista de precios" {
		t.Fatalf("con rango: %q", textos(d))
	}
	sinRango[ana] = true
	if d := e.dice(t, ana, ana, "!precios"); len(d.Respuestas) != 0 {
		t.Fatalf("sin el rango mínimo no responde: %q", textos(d))
	}
	// En cualquier grupo (donde ya responde el bot), para quien sí tiene el rango.
	const g = "grupo@g.us"
	const luis = "5215522222222@s.whatsapp.net"
	if d := e.dice(t, g, luis, "!precios"); textos(d) != "Lista de precios" {
		t.Fatalf("en un grupo: %q", textos(d))
	}
	// Solo privados: en un grupo no.
	e.propio(t, DatosPropio{Nombre: "secreto", Texto: "solo aquí", Privados: true}, true)
	if d := e.dice(t, g, luis, "!secreto"); len(d.Respuestas) != 0 {
		t.Fatalf("solo privado: %q", textos(d))
	}
	// Apagado: no responde.
	id := e.propio(t, DatosPropio{Nombre: "apagado", Texto: "x", Privados: true}, false)
	if d := e.dice(t, ana, ana, "!apagado"); len(d.Respuestas) != 0 {
		t.Fatal("apagado no responde")
	}
	_ = e.alm.FijarMenuActivo(1, id, true)
	if d := e.dice(t, ana, ana, "!apagado"); textos(d) != "x" {
		t.Fatal("encendido responde")
	}
}

func TestComandoPropioConOpciones(t *testing.T) {
	e := nuevo(t)
	e.m.c.Prefijo = func() string { return "!" }
	e.propio(t, DatosPropio{Nombre: "info", Texto: "¿Qué quieres saber?", Privados: true,
		Opciones: []OpcionPropia{{Texto: "Horario", Respuesta: "9 a 6"}, {Texto: "Dirección", Respuesta: "Calle 1"}}}, true)
	if d := e.dice(t, ana, ana, "!info"); textos(d) != "¿Qué quieres saber?\n1. Horario\n2. Dirección" {
		t.Fatalf("%q", textos(d))
	}
	if d := e.dice(t, ana, ana, "2"); textos(d) != "Calle 1" {
		t.Fatalf("%q", textos(d))
	}
	// Con opciones solo funciona en privado.
	d := &Def{V: 1, Propio: &DatosPropio{Nombre: "x", Texto: "t", Grupos: true, Opciones: []OpcionPropia{{Texto: "a", Respuesta: "b"}}}}
	if err := d.ConstruirPropio(); err == nil || !strings.Contains(err.Error(), "chats privados") {
		t.Fatalf("%v", err)
	}
}

func TestAyudaDeComandosPropiosSegunRango(t *testing.T) {
	e := nuevo(t)
	e.m.c.Prefijo = func() string { return "!" }
	e.m.c.PuedeUsar = func(sender string, rolMin int64) bool { return sender == ana }
	e.propio(t, DatosPropio{Nombre: "horario", Texto: "t", Resumen: "Ver el horario", Privados: true, Grupos: true}, true)
	e.propio(t, DatosPropio{Nombre: "vip", Texto: "t", Resumen: "Solo VIP", Privados: true, RangoMin: 3}, true)
	e.propio(t, DatosPropio{Nombre: "oculto", Texto: "t", Resumen: "No sale", Privados: true, OcultoAyuda: true}, true)
	e.propio(t, DatosPropio{Nombre: "apagado", Texto: "t", Privados: true}, false)
	const luis = "5215522222222@s.whatsapp.net"
	a := strings.Join(e.m.AyudaComandos(false, ana, ana, false), "\n")
	if !strings.Contains(a, "• !horario — Ver el horario") || !strings.Contains(a, "!vip") || strings.Contains(a, "oculto") || strings.Contains(a, "apagado") {
		t.Fatalf("con rango:\n%s", a)
	}
	l := strings.Join(e.m.AyudaComandos(false, luis, luis, false), "\n")
	if !strings.Contains(l, "!horario") || strings.Contains(l, "!vip") {
		t.Fatalf("sin el rango mínimo no ve !vip:\n%s", l)
	}
	if g := strings.Join(e.m.AyudaComandos(true, "g@g.us", ana, false), "\n"); !strings.Contains(g, "!horario") || strings.Contains(g, "!vip") {
		t.Fatalf("en un grupo solo los de grupo:\n%s", g)
	}
	if d := strings.Join(e.m.AyudaComandos(false, "x", "x", true), "\n"); !strings.Contains(d, "!vip") {
		t.Fatalf("el dueño ve todos los activos:\n%s", d)
	}
	// El prefijo cambia en la ayuda.
	e.m.c.Prefijo = func() string { return "#" }
	if a = strings.Join(e.m.AyudaComandos(false, ana, ana, false), "\n"); !strings.Contains(a, "• #horario") {
		t.Fatalf("%s", a)
	}
}

func TestValidacionDeComandoPropio(t *testing.T) {
	malos := []DatosPropio{
		{Nombre: "", Texto: "t", Privados: true},
		{Nombre: "dos palabras", Texto: "t", Privados: true},
		{Nombre: "con-guion", Texto: "t", Privados: true},
		{Nombre: strings.Repeat("a", 21), Texto: "t", Privados: true},
		{Nombre: "ok", Texto: "  ", Privados: true},
		{Nombre: "ok", Texto: "t"},
		{Nombre: "ok", Texto: "t", Privados: true, Resumen: strings.Repeat("r", 81)},
		{Nombre: "ok", Texto: "t", Privados: true, Opciones: []OpcionPropia{{Texto: "a"}}},
	}
	for i, m := range malos {
		if err := m.Validar(); err == nil {
			t.Fatalf("caso %d debía fallar: %+v", i, m)
		}
	}
	// Se limpia el prefijo que escriba la persona y las mayúsculas.
	dp := DatosPropio{Nombre: "  !Horario ", Texto: "t", Privados: true}
	if err := dp.Validar(); err != nil || dp.Nombre != "horario" {
		t.Fatalf("%v %q", err, dp.Nombre)
	}
	d := &Def{V: 1, Propio: &dp}
	_ = d.ConstruirPropio()
	d.Normalizar()
	for _, a := range d.Validar() {
		if a.Grave {
			t.Fatalf("un comando propio válido no tiene avisos graves: %+v", a)
		}
	}
	if d.Nombre != "Comando horario" || d.Disparadores.Comando != "horario" {
		t.Fatalf("%+v", d)
	}
	if got := TextoDeEjemplo("Hola {nombre}, {hora} {dia} {otro}", "Ana", time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC), time.UTC); got != "Hola Ana, 15:00 martes …" {
		t.Fatalf("%q", got)
	}
}
