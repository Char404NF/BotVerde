package flujo

import (
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
)

func TestValidadoresDeCampos(t *testing.T) {
	hoy := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	fechas := map[string]string{
		"25/12/2026": "25/12/2026", "5-3-26": "05/03/2026", "2026-12-25": "25/12/2026", "1.2.1990": "01/02/1990",
		"25 de diciembre de 2026": "25/12/2026", "5 marzo 2027": "05/03/2027", "7 dic": "07/12/2026", "hoy": "10/11/2026",
		"mañana": "11/11/2026", "pasado mañana": "12/11/2026", "29/02/2028": "29/02/2028", "15/8": "15/08/2026",
	}
	for in, want := range fechas {
		if got, ok := NormalizarFecha(in, hoy); !ok || got != want {
			t.Errorf("fecha %q → %q (%v), quería %q", in, got, ok, want)
		}
	}
	for _, mala := range []string{"", "31/02/2026", "32/01/2026", "00/01/2026", "13/13/2026", "abc", "5 de foo", "29/02/2027", "10/10/1800", "2026-13-01"} {
		if got, ok := NormalizarFecha(mala, hoy); ok {
			t.Errorf("fecha %q no debía valer y dio %q", mala, got)
		}
	}
	nums := map[string]string{"25": "25", "1,5": "1.5", "1.5": "1.5", "1,234.50": "1234.5", "1.234,50": "1234.5", "$100": "100", "1,234": "1234", "-3": "-3", " 42 ": "42", "1.234.567": "1234567", "0,5": "0.5"}
	for in, want := range nums {
		if _, got, ok := NormalizarNumero(in); !ok || got != want {
			t.Errorf("número %q → %q (%v), quería %q", in, got, ok, want)
		}
	}
	for _, malo := range []string{"", "abc", "12a", "1,2,3x", "--4", "1.2.3,4,5"} {
		if _, got, ok := NormalizarNumero(malo); ok {
			t.Errorf("número %q no debía valer y dio %q", malo, got)
		}
	}
	casos := []struct {
		c     Campo
		in    string
		valor string
		ok    bool
	}{
		{Campo{Tipo: CampoCorreo}, " Ana@Correo.MX ", "ana@correo.mx", true},
		{Campo{Tipo: CampoCorreo}, "ana@correo", "", false},
		{Campo{Tipo: CampoCorreo}, "ana correo@x.com", "", false},
		{Campo{Tipo: CampoTelefono}, "55 1234 5678", "+525512345678", true},
		{Campo{Tipo: CampoTelefono}, "+1 (415) 555-2671", "+14155552671", true},
		{Campo{Tipo: CampoTelefono}, "+44 20 7946 0958", "+442079460958", true},
		{Campo{Tipo: CampoTelefono}, "12345", "", false},
		{Campo{Tipo: CampoTelefono}, "hola", "", false},
		{Campo{Tipo: CampoNumero, Min: f64(18), Max: f64(99)}, "17", "", false},
		{Campo{Tipo: CampoNumero, Min: f64(18), Max: f64(99)}, "100", "", false},
		{Campo{Tipo: CampoNumero, Min: f64(18), Max: f64(99)}, "30", "30", true},
		{Campo{Tipo: CampoFecha}, "mañana", "11/11/2026", true},
		{Campo{Tipo: CampoOpcion, Opciones: []string{"Rojo", "Verde", "Azul"}}, "2", "Verde", true},
		{Campo{Tipo: CampoOpcion, Opciones: []string{"Rojo", "Verde", "Azul"}}, "azu", "Azul", true},
		{Campo{Tipo: CampoOpcion, Opciones: []string{"Rojo", "Verde", "Azul"}}, "morado", "", false},
		{Campo{Tipo: CampoOpcion, Opciones: []string{"Rojo", "Verde", "Azul"}}, "4", "", false},
		{Campo{Tipo: CampoTexto}, "  hola  ", "hola", true},
		{Campo{Tipo: CampoTexto}, "   ", "", false},
		{Campo{Tipo: CampoTexto}, strings.Repeat("a", 501), "", false},
	}
	for _, c := range casos {
		v, msg, ok := ValidarCampo(c.c, c.in, "MX", hoy)
		if ok != c.ok || v != c.valor || (!ok && msg == "") {
			t.Errorf("%s %q → %q ok=%v msg=%q; quería %q ok=%v", c.c.Tipo, c.in, v, ok, msg, c.valor, c.ok)
		}
	}
	// el mensaje de error del paso se usa cuando lo trae
	if _, msg, _ := ValidarCampo(Campo{Tipo: CampoNumero, Error: "Solo números, porfa."}, "x", "MX", hoy); msg != "Solo números, porfa." {
		t.Fatalf("%q", msg)
	}
}

// ficha escribe las respuestas de la cotización de `tienda()` una por una.
func llenaFicha(t *testing.T, e *entorno) {
	t.Helper()
	e.dice(t, ana, ana, ".menu")
	d := e.dice(t, ana, ana, "cotizar")
	if !strings.HasPrefix(textos(d), "Cotización\n\n(1 de 7) ¿Cómo te llamas?") || !strings.Contains(textos(d), "*cancelar*") {
		t.Fatalf("introducción y primera pregunta: %q", textos(d))
	}
}

func TestFormularioPasoAPasoConValidacionResumenYCorreccion(t *testing.T) {
	e := nuevo(t)
	e.crea(t, tienda(), true)
	llenaFicha(t, e)
	if d := e.dice(t, ana, ana, "Ana López"); !strings.HasPrefix(textos(d), "(2 de 7) ¿Cuántos años tienes?") {
		t.Fatalf("%q", textos(d))
	}
	// validación con mensaje y se repite la pregunta
	if d := e.dice(t, ana, ana, "diecisiete"); !strings.HasPrefix(textos(d), "Eso no parece un número") || !strings.Contains(textos(d), "(2 de 7)") {
		t.Fatalf("%q", textos(d))
	}
	if d := e.dice(t, ana, ana, "12"); !strings.HasPrefix(textos(d), "Escribe un número entre 18 y 99.") {
		t.Fatalf("rango: %q", textos(d))
	}
	e.dice(t, ana, ana, "31")
	if d := e.dice(t, ana, ana, "ana@@correo"); !strings.HasPrefix(textos(d), "Ese correo no parece correcto") {
		t.Fatalf("%q", textos(d))
	}
	e.dice(t, ana, ana, "ANA@Correo.mx")
	if d := e.dice(t, ana, ana, "123"); !strings.HasPrefix(textos(d), "Ese teléfono no parece correcto") {
		t.Fatalf("%q", textos(d))
	}
	e.dice(t, ana, ana, "55 1234 5678")
	if d := e.dice(t, ana, ana, "ayer por la tarde"); !strings.HasPrefix(textos(d), "No entendí la fecha") {
		t.Fatalf("%q", textos(d))
	}
	e.dice(t, ana, ana, "5 de marzo de 1990")
	if d := e.dice(t, ana, ana, "morado"); !strings.HasPrefix(textos(d), "Elige una de las opciones") || !strings.Contains(textos(d), "2. Verde") {
		t.Fatalf("%q", textos(d))
	}
	// "atrás" regresa a la pregunta anterior y se puede contestar otra vez
	if d := e.dice(t, ana, ana, "atrás"); !strings.HasPrefix(textos(d), "(5 de 7) ¿Cuándo naciste?") {
		t.Fatalf("%q", textos(d))
	}
	e.dice(t, ana, ana, "6/3/1990")
	e.dice(t, ana, ana, "verde")
	// campo opcional: se salta con "omitir"
	d := e.dice(t, ana, ana, "omitir")
	for _, esperado := range []string{"Revisa tus datos:", "• Nombre: Ana López", "• Edad: 31", "• Correo: ana@correo.mx", "• Teléfono: +525512345678", "• Nacimiento: 06/03/1990", "• Color: Verde", "• Nota: (sin contestar)", "¿Es correcto?\n1. Sí\n2. Corregir"} {
		if !strings.Contains(textos(d), esperado) {
			t.Fatalf("falta %q en el resumen: %q", esperado, textos(d))
		}
	}
	// Corregir permite elegir qué campo
	d = e.dice(t, ana, ana, "2")
	if !strings.HasPrefix(textos(d), "¿Cuál quieres corregir?") || !strings.Contains(textos(d), "3. Correo: ana@correo.mx") {
		t.Fatalf("%q", textos(d))
	}
	if d := e.dice(t, ana, ana, "correo"); !strings.HasPrefix(textos(d), "(3 de 7) ¿Tu correo?") {
		t.Fatalf("%q", textos(d))
	}
	if d := e.dice(t, ana, ana, "malo"); !strings.HasPrefix(textos(d), "Ese correo no parece correcto") {
		t.Fatalf("al corregir también se valida: %q", textos(d))
	}
	d = e.dice(t, ana, ana, "nuevo@correo.mx")
	if !strings.Contains(textos(d), "• Correo: nuevo@correo.mx") || !strings.Contains(textos(d), "¿Es correcto?") {
		t.Fatalf("vuelve al resumen: %q", textos(d))
	}
	// mientras no confirma no hay formulario guardado
	if e.alm.ContarFormularios(1, 0) != 0 {
		t.Fatal("no se guarda antes de confirmar")
	}
	d = e.dice(t, ana, ana, "1")
	if textos(d) != "Gracias Ana López, recibimos tu solicitud." || d.Omitir != "" {
		t.Fatalf("agradecimiento con variables: %q", textos(d))
	}
	if e.sesion(t, ana, ana) != nil {
		t.Fatal("el formulario terminó la conversación")
	}
	fs, _ := e.alm.ListarFormularios(1, 0, 10)
	if len(fs) != 1 || fs[0].Titulo != "Tienda" || fs[0].Nombre != "Ana" || fs[0].Chat != ana || len(fs[0].Campos) != 7 {
		t.Fatalf("%+v", fs)
	}
	val := map[string]string{}
	for _, c := range fs[0].Campos {
		val[c.ID] = c.Valor
	}
	if val["edad"] != "31" || val["correo"] != "nuevo@correo.mx" || val["tel"] != "+525512345678" || val["nac"] != "06/03/1990" || val["color"] != "Verde" || val["nota"] != "" || val["nombre_c"] != "Ana López" {
		t.Fatalf("valores normalizados: %v", val)
	}
	if n := len(e.avisos); n == 0 || e.avisos[n-1].tipo != "formulario" || e.avisos[n-1].titulo != "Nuevo formulario recibido" || !strings.Contains(e.avisos[n-1].detalle, "Correo: nuevo@correo.mx") {
		t.Fatalf("aviso con título neutro: %+v", e.avisos)
	}
	// estadísticas del formulario
	est, _ := e.alm.EstadisticasMenu(1, 1, "2026-01-01", "2026-12-31", false)
	c := map[string]int64{}
	for _, f := range est {
		c[f.Paso+"/"+f.Opcion] = f.Cuenta
	}
	if c["f/enviado"] != 1 || c["f/visita"] != 1 || c["f/invalida"] < 5 {
		t.Fatalf("%v", c)
	}
}

func TestFormularioCancelarEnCualquierMomentoYReintentosAgotados(t *testing.T) {
	e := nuevo(t)
	e.crea(t, tienda(), true)
	llenaFicha(t, e)
	e.dice(t, ana, ana, "Ana")
	e.dice(t, ana, ana, "40")
	d := e.dice(t, ana, ana, "cancelar")
	if !strings.HasPrefix(textos(d), "Formulario cancelado.") || !strings.Contains(textos(d), "Hola Ana") {
		t.Fatalf("cancelar vuelve al menú: %q", textos(d))
	}
	s := e.sesion(t, ana, ana)
	if s == nil || s.Paso != "m" || len(e.alm.Variables(s.ID)) != 0 || s.Fase != "" {
		t.Fatalf("se descartó lo contestado: %+v %v", s, e.alm.Variables(s.ID))
	}
	if e.alm.ContarFormularios(1, 0) != 0 {
		t.Fatal("un formulario cancelado no se guarda")
	}
	// cancelar desde el resumen
	e.dice(t, ana, ana, "cotizar")
	for _, r := range []string{"Ana", "40", "a@b.co", "5512345678", "1/1/2000", "rojo", "no"} {
		e.dice(t, ana, ana, r)
	}
	if d := e.dice(t, ana, ana, "cancelar"); !strings.HasPrefix(textos(d), "Formulario cancelado.") {
		t.Fatalf("%q", textos(d))
	}
	// tres intentos fallidos seguidos en un campo cancelan el formulario (sin ciclos)
	e.dice(t, ana, ana, "cotizar")
	e.dice(t, ana, ana, "Ana")
	e.dice(t, ana, ana, "x")
	e.dice(t, ana, ana, "y")
	d = e.dice(t, ana, ana, "z")
	if !strings.HasPrefix(textos(d), "No pude entender ese dato después de varios intentos, así que cancelé el formulario.") || !strings.Contains(textos(d), "Hola Ana") {
		t.Fatalf("%q", textos(d))
	}
	// un formulario que es lo único del menú: cancelar cierra la conversación
	e2 := nuevo(t)
	solo := &Def{V: 1, Nombre: "Solo", Inicio: "f", Pasos: []Paso{{ID: "f", Tipo: PasoForm, Titulo: "Datos", Campos: []Campo{{ID: "n", Pregunta: "¿Nombre?", Tipo: CampoTexto}}}},
		Disparadores: Disparadores{Comando: ".datos"}, Alcance: Alcance{Privados: true}}
	e2.crea(t, solo, true)
	e2.dice(t, ana, ana, ".datos")
	d = e2.dice(t, ana, ana, "cancelar")
	if textos(d) != "Cancelado. Cuando quieras, escribe *menú*." || e2.sesion(t, ana, ana) != nil {
		t.Fatalf("%q", textos(d))
	}
	// con un solo campo, "Corregir" va directo a esa pregunta; y sin resumen se guarda de una vez
	e2.dice(t, ana, ana, ".datos")
	e2.dice(t, ana, ana, "Luis")
	if d := e2.dice(t, ana, ana, "2"); !strings.HasPrefix(textos(d), "¿Nombre?") {
		t.Fatalf("%q", textos(d))
	}
	e2.dice(t, ana, ana, "Luisa")
	if d := e2.dice(t, ana, ana, "sí"); textos(d) != "¡Listo! Recibimos tus datos." {
		t.Fatalf("%q", textos(d))
	}
	fs, _ := e2.alm.ListarFormularios(1, 0, 5)
	if len(fs) != 1 || fs[0].Campos[0].Valor != "Luisa" {
		t.Fatalf("%+v", fs)
	}
}

func TestFormularioSeGuardaCifradoYElSimuladorNoGuardaNiAvisa(t *testing.T) {
	e := nuevo(t)
	id := e.crea(t, tienda(), true)
	if _, err := e.m.Probar(id, ".menu", true, ""); err != nil {
		t.Fatal(err)
	}
	e.m.Probar(id, "5", false, "")
	for _, r := range []string{"Ana", "40", "a@b.co", "5512345678", "1/1/2000", "rojo", "omitir"} {
		e.m.Probar(id, r, false, "")
	}
	p, _ := e.m.Probar(id, "1", false, "")
	if !strings.Contains(strings.Join(p.Salidas, " "), "Gracias Ana") || p.Estado != almacen.SesCompletada {
		t.Fatalf("%+v", p)
	}
	if e.alm.ContarFormularios(1, 0) != 0 || len(e.avisos) != 0 {
		t.Fatalf("el simulador no guarda ni avisa: %d %v", e.alm.ContarFormularios(1, 0), e.avisos)
	}
}

func TestFormularioEnLaPracticaConOpcionYOtrosComandos(t *testing.T) {
	e := nuevo(t)
	e.crea(t, tienda(), true)
	llenaFicha(t, e)
	// "repetir" vuelve a hacer la pregunta
	if d := e.dice(t, ana, ana, "repetir"); !strings.HasPrefix(textos(d), "(1 de 7) ¿Cómo te llamas?") {
		t.Fatalf("%q", textos(d))
	}
	// en la primera pregunta, "atrás" regresa al menú de donde venía
	if d := e.dice(t, ana, ana, "atrás"); !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("%q", textos(d))
	}
	e.dice(t, ana, ana, "cotizar")
	e.dice(t, ana, ana, "Ana")
	// "menú" a medias descarta lo contestado
	if d := e.dice(t, ana, ana, "menú"); !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("%q", textos(d))
	}
	if v := e.alm.Variables(e.sesion(t, ana, ana).ID); len(v) != 0 {
		t.Fatalf("%v", v)
	}
	// el resumen insiste con 1 o 2 y, si no se entiende tras varios intentos, cancela
	e.dice(t, ana, ana, "cotizar")
	for _, r := range []string{"Ana", "40", "a@b.co", "5512345678", "1/1/2000", "rojo", "omitir"} {
		e.dice(t, ana, ana, r)
	}
	if d := e.dice(t, ana, ana, "quizá"); !strings.HasPrefix(textos(d), "Responde 1 si está bien o 2") {
		t.Fatalf("%q", textos(d))
	}
	e.dice(t, ana, ana, "mmm")
	if d := e.dice(t, ana, ana, "nose"); !strings.HasPrefix(textos(d), "No me quedó claro si los datos están bien") {
		t.Fatalf("%q", textos(d))
	}
	// salir también descarta el formulario
	e.dice(t, ana, ana, "cotizar")
	e.dice(t, ana, ana, "Ana")
	if d := e.dice(t, ana, ana, "salir"); !strings.HasPrefix(textos(d), "Listo.") || e.sesion(t, ana, ana) != nil {
		t.Fatalf("%q", textos(d))
	}
}
