package flujo

import (
	"context"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/motor"
)

// Etapa C: menús ampliados (número o texto parcial, comandos, escalera de errores, expiración, hablar con persona, formularios,
// visibilidad por opción, plantillas, estadísticas).

func tienda() *Def {
	return &Def{V: 1, Nombre: "Tienda", Inicio: "m",
		Pasos: []Paso{
			{ID: "m", Tipo: PasoMenu, Titulo: "Hola {nombre}. ¿Qué necesitas?", Opciones: []Opcion{
				{Texto: "Precios de refacciones", Destino: "pr"}, {Texto: "Precios de servicio", Destino: "ps"}, {Texto: "Horario", Destino: "h"},
				{Texto: "Contacto", Destino: "c"}, {Texto: "Cotizar", Destino: "f"}}},
			{ID: "pr", Tipo: PasoRespuesta, Texto: "Refacciones desde $50."},
			{ID: "ps", Tipo: PasoRespuesta, Texto: "Servicio desde $200."},
			{ID: "h", Tipo: PasoRespuesta, Texto: "Abrimos de 9 a 6. Hoy es {dia} {fecha}."},
			{ID: "c", Tipo: PasoMenu, Titulo: "Contacto", Opciones: []Opcion{{Texto: "Teléfono", Destino: "tel"}, {Texto: "Correo", Destino: "cor"}}},
			{ID: "tel", Tipo: PasoRespuesta, Texto: "Llámanos al 55 1234 5678."},
			{ID: "cor", Tipo: PasoRespuesta, Texto: "Escríbenos a hola@tienda.mx."},
			{ID: "f", Tipo: PasoForm, Titulo: "Cotización", Texto: "Gracias {nombre_c}, recibimos tu solicitud.", Campos: []Campo{
				{ID: "nombre_c", Etiqueta: "Nombre", Pregunta: "¿Cómo te llamas?", Tipo: CampoTexto},
				{ID: "edad", Etiqueta: "Edad", Pregunta: "¿Cuántos años tienes?", Tipo: CampoNumero, Min: f64(18), Max: f64(99)},
				{ID: "correo", Etiqueta: "Correo", Pregunta: "¿Tu correo?", Tipo: CampoCorreo},
				{ID: "tel", Etiqueta: "Teléfono", Pregunta: "¿Tu teléfono?", Tipo: CampoTelefono},
				{ID: "nac", Etiqueta: "Nacimiento", Pregunta: "¿Cuándo naciste?", Tipo: CampoFecha},
				{ID: "color", Etiqueta: "Color", Pregunta: "¿Qué color prefieres?", Tipo: CampoOpcion, Opciones: []string{"Rojo", "Verde", "Azul"}},
				{ID: "nota", Etiqueta: "Nota", Pregunta: "¿Algo más?", Tipo: CampoTexto, Opcional: true},
			}},
		},
		Disparadores: Disparadores{Palabras: []string{"tienda"}, Comando: ".menu"}, Alcance: Alcance{Privados: true}}
}

func (e *entorno) conf(t *testing.T, clave, valor string) {
	t.Helper()
	if err := e.alm.FijarAjusteBot(1, clave, valor); err != nil {
		t.Fatal(err)
	}
}

func (e *entorno) sesion(t *testing.T, chat, sender string) *almacen.Sesion {
	t.Helper()
	s, err := e.alm.SesionVigente(1, chat, sender, false, e.rel.t, 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func empieza(t *testing.T, e *entorno, d *Def) {
	t.Helper()
	e.crea(t, d, true)
	if r := e.dice(t, ana, ana, ".menu"); len(r.Respuestas) == 0 {
		t.Fatalf("el menú debía arrancar: %+v", r)
	}
}

func TestNumeroTextoParcialYAmbiguo(t *testing.T) {
	e := nuevo(t)
	e.crea(t, tienda(), true)
	casos := []struct{ dicho, espera string }{
		{"3", "Abrimos de 9 a 6. Hoy es martes 10/11/2026."},
		{"horario", "Abrimos de 9 a 6."},
		{"HOR", "Abrimos de 9 a 6."},         // principio del texto, sin mayúsculas
		{"refacc", "Refacciones desde $50."}, // palabra de una sola opción
		{"precios de serv", "Servicio desde $200."},
		{"2 por favor", "Servicio desde $200."}, // número con más texto
		{"cotiz", "Cotización"},
	}
	for _, c := range casos {
		e.dice(t, ana, ana, ".menu")
		d := e.dice(t, ana, ana, c.dicho)
		if !strings.Contains(textos(d), c.espera) {
			t.Fatalf("%q debía dar %q y dio %q", c.dicho, c.espera, textos(d))
		}
		e.dice(t, ana, ana, "!salir")
	}
	// ambiguo: varias opciones encajan → pide aclarar, lista las coincidencias, no cuenta como error
	e.dice(t, ana, ana, ".menu")
	d := e.dice(t, ana, ana, "precios")
	if !strings.Contains(textos(d), "varias opciones parecidas") || !strings.Contains(textos(d), "1. Precios de refacciones") || !strings.Contains(textos(d), "2. Precios de servicio") || strings.Contains(textos(d), "Horario") {
		t.Fatalf("ambiguo: %q", textos(d))
	}
	if s := e.sesion(t, ana, ana); s == nil || s.Invalidas != 0 || s.Paso != "m" {
		t.Fatalf("aclarar no es un error: %+v", s)
	}
	if d := e.dice(t, ana, ana, "2"); textos(d) != "Servicio desde $200." {
		t.Fatalf("después de aclarar se elige por número: %q", textos(d))
	}
	// una palabra muy corta o que no existe sí es error
	e.dice(t, ana, ana, ".menu")
	if d := e.dice(t, ana, ana, "pr"); !strings.HasPrefix(textos(d), "No entendí") {
		t.Fatalf("muy corto: %q", textos(d))
	}
	if d := e.dice(t, ana, ana, "ubicacion"); !strings.Contains(textos(d), "*!persona*") {
		t.Fatalf("no existe, 2.º error: %q", textos(d))
	}
	// con acentos: se entiende sin ellos
	e.dice(t, ana, ana, ".menu") // el comando reinicia
	e.dice(t, ana, ana, "contacto")
	if d := e.dice(t, ana, ana, "TELEFONO"); textos(d) != "Llámanos al 55 1234 5678." {
		t.Fatalf("%q", textos(d))
	}
}

func TestComandosSiempreActivosDentroDeUnMenu(t *testing.T) {
	e := nuevo(t)
	e.crea(t, tienda(), true)
	e.dice(t, ana, ana, ".menu")
	e.dice(t, ana, ana, "4") // submenú Contacto
	if d := e.dice(t, ana, ana, "!repetir"); !strings.HasPrefix(textos(d), "Contacto\n1. Teléfono") {
		t.Fatalf("repetir: %q", textos(d))
	}
	if d := e.dice(t, ana, ana, "!atrás"); !strings.HasPrefix(textos(d), "Hola Ana") || strings.Contains(textos(d), "0. ") {
		t.Fatalf("atrás vuelve al menú de antes: %q", textos(d))
	}
	e.dice(t, ana, ana, "contacto")
	if d := e.dice(t, ana, ana, "!menú"); !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("menú: %q", textos(d))
	}
	e.dice(t, ana, ana, "4")
	if d := e.dice(t, ana, ana, "!menu"); !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("con prefijo: %q", textos(d))
	}
	e.dice(t, ana, ana, "4")
	if d := e.dice(t, ana, ana, "!inicio"); !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("inicio: %q", textos(d))
	}
	// atrás en el menú principal no se sale ni se rompe: muestra el principal
	if d := e.dice(t, ana, ana, "!atras"); !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("atrás en el principal: %q", textos(d))
	}
	// salir cierra la conversación con el mensaje editable
	d := e.dice(t, ana, ana, "!salir")
	if textos(d) != "Listo. Cuando necesites algo, escribe *!menu*." || e.sesion(t, ana, ana) != nil {
		t.Fatalf("salir: %q", textos(d))
	}
	// la palabra «menú» suelta NO abre nada (no se activa por error); con el símbolo sí, aunque ninguna regla lo pida
	if d := e.dice(t, ana, ana, "menú"); strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("«menú» sin símbolo no debe abrir el menú: %q", textos(d))
	}
	if d := e.dice(t, ana, ana, "!menú"); !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("!menú abre el menú: %q", textos(d))
	}
	// el prefijo es configurable
	e.m.c.Prefijo = func() string { return "#" }
	e.dice(t, ana, ana, "4")
	if d := e.dice(t, ana, ana, "#atras"); !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("prefijo #: %q", textos(d))
	}
	// una frase que contiene la palabra no es un comando
	if d := e.dice(t, ana, ana, "quiero salir de dudas"); !strings.HasPrefix(textos(d), "No entendí") {
		t.Fatalf("frase larga: %q", textos(d))
	}
	// persona = hablar con una persona
	d = e.dice(t, ana, ana, "!persona")
	if !d.PasarAHumano || len(e.avisos) == 0 || e.avisos[len(e.avisos)-1].tipo != "humano" {
		t.Fatalf("persona: %+v %+v", d, e.avisos)
	}
}

func TestEscaleraConfigurablePorMenuYGlobal(t *testing.T) {
	e := nuevo(t)
	d := tienda()
	d.Ajustes = &AjustesMenu{ErrMax: 2, TxtN1: "Uy, esa no es una opción.", TxtN3: "Me rindo. Escribe *menú* cuando quieras."}
	e.crea(t, d, true)
	e.dice(t, ana, ana, ".menu")
	if r := e.dice(t, ana, ana, "zzzz"); !strings.HasPrefix(textos(r), "Uy, esa no es una opción.\nHola Ana") {
		t.Fatalf("texto del menú: %q", textos(r))
	}
	r := e.dice(t, ana, ana, "yyyy")
	if textos(r) != "Me rindo. Escribe *menú* cuando quieras." || e.sesion(t, ana, ana) != nil {
		t.Fatalf("con tope 2 la segunda para: %q", textos(r))
	}
	// ajustes generales para un menú que no trae los suyos
	e2 := nuevo(t)
	e2.crea(t, tienda(), true)
	e2.conf(t, "menu_err_max", "4")
	e2.conf(t, "menu_txt_n2", "Prueba con *persona* o un número.")
	e2.conf(t, "menu_txt_n3", "")
	e2.dice(t, ana, ana, ".menu")
	e2.dice(t, ana, ana, "aaaa")
	if r := e2.dice(t, ana, ana, "bbbb"); !strings.HasPrefix(textos(r), "Prueba con *persona* o un número.") {
		t.Fatalf("%q", textos(r))
	}
	if r := e2.dice(t, ana, ana, "cccc"); !strings.HasPrefix(textos(r), "Prueba con *persona*") {
		t.Fatalf("la 3.ª todavía no para con tope 4: %q", textos(r))
	}
	r = e2.dice(t, ana, ana, "dddd")
	if len(r.Respuestas) != 0 || e2.sesion(t, ana, ana) != nil {
		t.Fatalf("la 4.ª para; con el texto vacío no dice nada: %+v", r)
	}
}

func TestSaludosGraciasYEmojisNoCuentanComoError(t *testing.T) {
	e := nuevo(t)
	e.crea(t, tienda(), true)
	e.dice(t, ana, ana, ".menu")
	for _, neutro := range []string{"hola", "Hola buenas tardes", "gracias", "muchas gracias", "ok", "👍", "😀😀", "!!!", "..."} {
		d := e.dice(t, ana, ana, neutro)
		if !strings.HasPrefix(textos(d), "Hola Ana") {
			t.Fatalf("%q debía volver a mostrar el menú: %q", neutro, textos(d))
		}
		if s := e.sesion(t, ana, ana); s == nil || s.Invalidas != 0 {
			t.Fatalf("%q no cuenta como error: %+v", neutro, s)
		}
	}
	est, _ := e.alm.EstadisticasMenu(1, 1, "2026-01-01", "2026-12-31", false)
	for _, f := range est {
		if f.Opcion == "invalida" {
			t.Fatalf("no debía contar inválidas: %+v", est)
		}
	}
	// "ey" no está en la lista: error. Si se agrega a la lista editable, ya no.
	if d := e.dice(t, ana, ana, "ey"); !strings.HasPrefix(textos(d), "No entendí") {
		t.Fatalf("%q", textos(d))
	}
	e.conf(t, "menu_saludos", "ey, quiubole")
	if d := e.dice(t, ana, ana, "quiubole ey"); !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("lista editable: %q", textos(d))
	}
	if s := e.sesion(t, ana, ana); s.Invalidas != 1 {
		t.Fatalf("el error anterior sigue contando solo una vez: %+v", s)
	}
}

func TestImagenOAudioCuandoSeEsperaTextoTieneMensajePropio(t *testing.T) {
	e := nuevo(t)
	d := tienda()
	d.Ajustes = &AjustesMenu{TxtNoTx: "Mándame el número, no la foto."}
	e.crea(t, d, true)
	foto := motor.Mensaje{Chat: ana, ID: "F1", Remitente: ana, Texto: "", EsTexto: false, TS: e.rel.t}
	dec, _ := e.m.Decidir(context.Background(), foto, []motor.Mensaje{foto})
	if dec.Omitir != almacen.MotNoTexto || len(dec.Respuestas) != 0 {
		t.Fatalf("sin menú abierto una foto se omite: %+v", dec)
	}
	if e.m.EsperaTexto(foto) {
		t.Fatal("sin conversación no se espera texto")
	}
	e.dice(t, ana, ana, ".menu")
	if !e.m.EsperaTexto(foto) {
		t.Fatal("con el menú abierto sí se espera")
	}
	foto.TS = e.rel.t
	dec, _ = e.m.Decidir(context.Background(), foto, []motor.Mensaje{foto})
	if textos(dec) != "Mándame el número, no la foto." {
		t.Fatalf("%q", textos(dec))
	}
	if s := e.sesion(t, ana, ana); s.Invalidas != 0 || s.Paso != "m" {
		t.Fatalf("no cuenta como error ni mueve la conversación: %+v", s)
	}
	// el mensaje general es editable
	e2 := nuevo(t)
	e2.crea(t, tienda(), true)
	e2.dice(t, ana, ana, ".menu")
	dec, _ = e2.m.Decidir(context.Background(), foto, []motor.Mensaje{foto})
	if !strings.Contains(textos(dec), "solo entiendo mensajes escritos") {
		t.Fatalf("%q", textos(dec))
	}
	e2.conf(t, "menu_txt_notexto", "Solo texto, por favor.")
	dec, _ = e2.m.Decidir(context.Background(), foto, []motor.Mensaje{foto})
	if textos(dec) != "Solo texto, por favor." {
		t.Fatalf("%q", textos(dec))
	}
	// una ráfaga con texto y foto: se toma el texto
	txt := motor.Mensaje{Chat: ana, ID: "T1", Remitente: ana, Texto: "3", EsTexto: true, TS: e2.rel.t}
	dec, _ = e2.m.Decidir(context.Background(), txt, []motor.Mensaje{foto, txt})
	if !strings.Contains(textos(dec), "Abrimos") {
		t.Fatalf("%q", textos(dec))
	}
}

func TestExpiraEnSilencioYSeRetomaOSeReinicia(t *testing.T) {
	// reiniciar (lo normal): la conversación caduca sin mandar nada y al volver empieza de nuevo
	e := nuevo(t)
	d := tienda()
	d.Ajustes = &AjustesMenu{ExpiraMin: 5}
	d.Disparadores.Palabras = []string{"tienda", "hola"}
	e.crea(t, d, true)
	e.dice(t, ana, ana, ".menu")
	e.dice(t, ana, ana, "contacto")
	e.rel.t = e.rel.t.Add(6 * time.Minute)
	if df := e.m.Diferidas(e.rel.t); len(df) != 0 {
		t.Fatalf("expirar no manda nada: %+v", df)
	}
	if e.sesion(t, ana, ana) != nil {
		t.Fatal("ya caducó")
	}
	est, _ := e.alm.EstadisticasMenu(1, 1, "2026-01-01", "2026-12-31", false)
	cuenta := map[string]int64{}
	for _, f := range est {
		cuenta[f.Paso+"/"+f.Opcion] = f.Cuenta
	}
	if cuenta["/expirado"] != 1 || cuenta["c/abandono"] != 1 {
		t.Fatalf("dónde abandonó: %v", cuenta)
	}
	if d := e.dice(t, ana, ana, "hola"); !strings.HasPrefix(textos(d), "Hola Ana") || e.sesion(t, ana, ana).Paso != "m" {
		t.Fatalf("reinicia: %q", textos(d))
	}
	// retomar: vuelve a donde iba, con los datos
	e2 := nuevo(t)
	d2 := tienda()
	d2.Ajustes = &AjustesMenu{ExpiraMin: 5, Volver: "retomar"}
	d2.Disparadores.Palabras = []string{"tienda", "hola"}
	e2.crea(t, d2, true)
	e2.dice(t, ana, ana, ".menu")
	e2.dice(t, ana, ana, "contacto")
	antes := e2.sesion(t, ana, ana).ID
	e2.rel.t = e2.rel.t.Add(30 * time.Minute)
	d3 := e2.dice(t, ana, ana, "hola")
	if !strings.HasPrefix(textos(d3), "Seguimos donde ibas.") || !strings.Contains(textos(d3), "Contacto\n1. Teléfono") {
		t.Fatalf("retoma: %q", textos(d3))
	}
	if s := e2.sesion(t, ana, ana); s == nil || s.ID != antes || s.Paso != "c" {
		t.Fatalf("misma conversación: %+v", s)
	}
	if d4 := e2.dice(t, ana, ana, "teléfono"); textos(d4) != "Llámanos al 55 1234 5678." {
		t.Fatalf("%q", textos(d4))
	}
	// pasadas las horas de retomar, empieza de nuevo; y el comando explícito siempre empieza de cero
	e2.dice(t, ana, ana, ".menu")
	e2.dice(t, ana, ana, "contacto")
	e2.rel.t = e2.rel.t.Add(30 * time.Minute)
	if d5 := e2.dice(t, ana, ana, ".menu"); !strings.HasPrefix(textos(d5), "Hola Ana") {
		t.Fatalf("el comando reinicia: %q", textos(d5))
	}
	e2.rel.t = e2.rel.t.Add(30 * time.Minute)
	e2.rel.t = e2.rel.t.Add(30 * time.Hour)
	if d6 := e2.dice(t, ana, ana, "hola"); !strings.HasPrefix(textos(d6), "Hola Ana") {
		t.Fatalf("pasaron más de 24 h: %q", textos(d6))
	}
}

func conHorario(d *Def) *Def {
	d.Horario = Horario{Dias: []int{1, 2, 3, 4, 5}, Desde: "09:00", Hasta: "18:00"}
	// el horario del menú limita cuándo se activa; para probar "fuera de horario" el menú se activa con cualquier mensaje
	d.Disparadores = Disparadores{Cualquiera: true, FueraDeHorario: true, Comando: ".menu"}
	return d
}

func TestHablarConPersonaDentroYFueraDeHorarioConservaLosDatos(t *testing.T) {
	e := nuevo(t)
	e.crea(t, conHorario(tienda()), true)
	// 05:30 hora de México de un martes: fuera del horario 09:00–18:00
	e.dice(t, ana, ana, "hola")
	e.dice(t, ana, ana, "5") // formulario
	e.dice(t, ana, ana, "Ana López")
	e.dice(t, ana, ana, "31")
	d := e.dice(t, ana, ana, "!persona")
	if !d.PasarAHumano || !strings.Contains(textos(d), "fuera de horario (de lunes a viernes de 09:00 a 18:00)") {
		t.Fatalf("fuera de horario: %+v %q", d, textos(d))
	}
	if st := e.alm.EstadoDeChat(1, ana).EstadoTrabajo; st != almacen.TrabajoPendiente {
		t.Fatalf("el chat queda Pendiente: %q", st)
	}
	av := e.avisos[len(e.avisos)-1]
	if av.tipo != "humano" || av.titulo != "Alguien pidió hablar con una persona" || !strings.Contains(av.detalle, "Nombre: Ana López") || !strings.Contains(av.detalle, "Edad: 31") {
		t.Fatalf("el aviso lleva lo capturado: %+v", av)
	}
	ses, _ := e.alm.SesionesEnCurso(1, e.rel.t, 2*time.Hour)
	if len(ses) != 1 || ses[0].Estado != almacen.SesHumano {
		t.Fatalf("%+v", ses)
	}
	vars := e.alm.Variables(ses[0].ID)
	if vars["nombre_c"] != "Ana López" || vars["edad"] != "31" {
		t.Fatalf("conserva los datos capturados: %v", vars)
	}
	if d := e.dice(t, ana, ana, "¿hay alguien?"); d.Omitir != almacen.MotHumano {
		t.Fatalf("el bot calla en ese chat: %+v", d)
	}
	// dentro de horario (10:00 hora de México)
	e2 := nuevo(t)
	e2.crea(t, conHorario(tienda()), true)
	e2.rel.t = time.Date(2026, 11, 10, 16, 0, 0, 0, time.UTC)
	e2.dice(t, ana, ana, ".menu")
	d = e2.dice(t, ana, ana, "!persona")
	if textos(d) != "Gracias. Una persona te atenderá en cuanto pueda." || !d.PasarAHumano {
		t.Fatalf("dentro de horario: %q", textos(d))
	}
	// los textos son editables
	e3 := nuevo(t)
	e3.crea(t, conHorario(tienda()), true)
	e3.conf(t, "menu_humano_fuera", "Cerrado. Abrimos {horario}.")
	e3.dice(t, ana, ana, ".menu")
	if d := e3.dice(t, ana, ana, "!persona"); textos(d) != "Cerrado. Abrimos de lunes a viernes de 09:00 a 18:00." {
		t.Fatalf("%q", textos(d))
	}
	// el horario general de atención también vale cuando el menú no trae el suyo
	e4 := nuevo(t)
	e4.crea(t, tienda(), true)
	e4.conf(t, "menu_horario", `{"dias":[1,2,3,4,5],"desde":"09:00","hasta":"18:00"}`)
	e4.dice(t, ana, ana, ".menu")
	if d := e4.dice(t, ana, ana, "!persona"); !strings.Contains(textos(d), "fuera de horario (de lunes a viernes de 09:00 a 18:00)") {
		t.Fatalf("%q", textos(d))
	}
}

func TestPasoHumanoUsaTextoFueraDeHorario(t *testing.T) {
	e := nuevo(t)
	d := &Def{V: 1, Nombre: "H", Inicio: "m", Horario: Horario{Dias: []int{1}, Desde: "09:00", Hasta: "10:00"},
		Pasos: []Paso{{ID: "m", Tipo: PasoMenu, Titulo: "Hola", Opciones: []Opcion{{Texto: "Persona", Destino: "h"}}},
			{ID: "h", Tipo: PasoHumano, Texto: "Ya voy", TextoFuera: "Hoy no hay nadie."}},
		Disparadores: Disparadores{Cualquiera: true, FueraDeHorario: true}, Alcance: Alcance{Privados: true}}
	e.crea(t, d, true)
	e.dice(t, ana, ana, "x")
	if r := e.dice(t, ana, ana, "1"); textos(r) != "Hoy no hay nadie." || !r.PasarAHumano {
		t.Fatalf("%q", textos(r))
	}
}
