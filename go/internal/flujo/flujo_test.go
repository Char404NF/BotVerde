package flujo

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/motor"
)

type reloj struct{ t time.Time }

func (r *reloj) ahora() time.Time { return r.t }

type aviso struct{ chat, tipo, titulo, detalle string }

type entorno struct {
	alm    *almacen.Almacen
	m      *Motor
	rel    *reloj
	avisos []aviso
}

func nuevo(t *testing.T) *entorno {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 5)
	}
	alm, err := almacen.Abrir(t.TempDir(), k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = alm.Cerrar() })
	_ = alm.FijarZona("America/Mexico_City")
	e := &entorno{alm: alm, rel: &reloj{t: time.Date(2026, 11, 10, 11, 30, 0, 0, time.UTC)}}
	e.m = Nuevo(Config{
		BotID: 1, Alm: alm, Ahora: e.rel.ahora, Zona: func() *time.Location { l, _ := time.LoadLocation("America/Mexico_City"); return l },
		Nombre: func(chat, sender string) string { return "Ana" },
		AlAviso: func(chat, tipo, titulo, detalle string) {
			e.avisos = append(e.avisos, aviso{chat, tipo, titulo, detalle})
		},
	})
	return e
}

const ana = "5215511111111@s.whatsapp.net"

func (e *entorno) crea(t *testing.T, d *Def, activo bool) int64 {
	t.Helper()
	d.Normalizar()
	id, err := e.alm.CrearMenu(1, d.JSON(), activo, false)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (e *entorno) dice(t *testing.T, chat, sender, texto string) motor.Decision {
	t.Helper()
	e.rel.t = e.rel.t.Add(3 * time.Second)
	u := motor.Mensaje{Chat: chat, ID: fmt.Sprint(e.rel.t.UnixNano()), Remitente: sender, Texto: texto, EsTexto: true, TS: e.rel.t, EsGrupo: strings.HasSuffix(chat, "@g.us"), Mencionado: strings.HasSuffix(chat, "@g.us")}
	d, err := e.m.Decidir(context.Background(), u, []motor.Mensaje{u})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func textos(d motor.Decision) string {
	var b []string
	for _, r := range d.Respuestas {
		b = append(b, r.Texto)
	}
	return strings.Join(b, " || ")
}

func ventas() *Def {
	return &Def{V: 1, Nombre: "Ventas", Inicio: "m",
		Pasos: []Paso{
			{ID: "m", Tipo: PasoMenu, Titulo: "Hola {nombre}, son las {hora}. ¿Qué buscas?", Opciones: []Opcion{{Texto: "Precios", Destino: "p"}, {Texto: "Contacto", Destino: "h"}, {Texto: "Dejar un dato", Destino: "d"}}},
			{ID: "p", Tipo: PasoMenu, Titulo: "Precios", Opciones: []Opcion{{Texto: "Plan básico", Destino: "b"}}},
			{ID: "b", Tipo: PasoRespuesta, Texto: "El plan básico cuesta $100."},
			{ID: "h", Tipo: PasoHumano, Texto: "Te paso con una persona."},
			{ID: "d", Tipo: PasoPedir, Pregunta: "¿Cuál es tu correo?", Variable: "dato1", Siguiente: "g"},
			{ID: "g", Tipo: PasoRespuesta, Texto: "Anotado: {dato1}. Gracias, {nombre}.", Notificar: true, Aviso: "Contacto: {dato1}"},
		},
		Disparadores: Disparadores{Palabras: []string{"ventas"}, Comando: ".menu"}, Alcance: Alcance{Privados: true}}
}

func TestDelDisparadorAlMenuYEnCadena(t *testing.T) {
	e := nuevo(t)
	e.crea(t, ventas(), true)
	if d := e.dice(t, ana, ana, "hola buenas"); len(d.Respuestas) != 0 || d.Coincidio != "" {
		t.Fatalf("sin disparador no responde: %+v", d)
	}
	d := e.dice(t, ana, ana, "quiero ventas por favor")
	if d.Coincidio != "Ventas" || textos(d) != "Hola Ana, son las 05:30. ¿Qué buscas?\n1. Precios\n2. Contacto\n3. Dejar un dato" {
		t.Fatalf("%q", textos(d))
	}
	d = e.dice(t, ana, ana, "1")
	if textos(d) != "Precios\n1. Plan básico\n0. Volver al menú principal" {
		t.Fatalf("%q", textos(d))
	}
	d = e.dice(t, ana, ana, "Plan básico") // también por texto
	if textos(d) != "El plan básico cuesta $100." {
		t.Fatalf("%q", textos(d))
	}
	// terminó: un mensaje cualquiera ya no responde (no hay disparador)
	if d := e.dice(t, ana, ana, "gracias"); len(d.Respuestas) != 0 {
		t.Fatalf("%+v", d)
	}
	est, _ := e.alm.EstadisticasMenu(1, 1, "2026-01-01", "2026-12-31", false)
	cuenta := map[string]int64{}
	for _, f := range est {
		cuenta[f.Paso+"/"+f.Opcion] = f.Cuenta
	}
	if cuenta["/iniciado"] != 1 || cuenta["/completado"] != 1 || cuenta["m/1"] != 1 || cuenta["p/1"] != 1 || cuenta["m/visita"] != 1 {
		t.Fatalf("%v", cuenta)
	}
}

func TestCeroVuelveAlMenuPrincipal(t *testing.T) {
	e := nuevo(t)
	e.crea(t, ventas(), true)
	e.dice(t, ana, ana, ".menu")
	e.dice(t, ana, ana, "1")
	d := e.dice(t, ana, ana, "0")
	if !strings.HasPrefix(textos(d), "Hola Ana") || strings.Contains(textos(d), "\n0. ") {
		t.Fatalf("%q", textos(d))
	}
}

func TestEscaleraDeErroresPrimeraReconoceSegundaOfreceHumanoTerceraParaYReinicia(t *testing.T) {
	e := nuevo(t)
	e.crea(t, ventas(), true)
	e.dice(t, ana, ana, ".menu")
	d := e.dice(t, ana, ana, "blablabla")
	if !strings.HasPrefix(textos(d), "No entendí esa opción. Estas son las opciones:\nHola Ana") || !strings.Contains(textos(d), "1. Precios") {
		t.Fatalf("1.ª: reconoce y ofrece las opciones: %q", textos(d))
	}
	d = e.dice(t, ana, ana, "xyz")
	if !strings.Contains(textos(d), "*persona*") || !strings.Contains(textos(d), "1. Precios") {
		t.Fatalf("2.ª: ofrece hablar con una persona: %q", textos(d))
	}
	d = e.dice(t, ana, ana, "otra cosa")
	if !strings.Contains(textos(d), "empezar de nuevo") || strings.Contains(textos(d), "1. Precios") || d.Omitir != "" {
		t.Fatalf("3.ª: para y avisa: %+v", d)
	}
	if s, _ := e.alm.SesionVigente(1, ana, ana, false, e.rel.t, 2*time.Hour); s != nil {
		t.Fatalf("la sesión quedó cerrada, sin ciclos: %+v", s)
	}
	// después de cortado, el texto sin sentido ya no responde nada; un disparador vuelve a empezar desde cero
	if d := e.dice(t, ana, ana, "sigo sin entender"); len(d.Respuestas) != 0 {
		t.Fatalf("sin sesión y sin disparador no hay respuesta: %+v", d)
	}
	if d := e.dice(t, ana, ana, ".menu"); !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("%q", textos(d))
	}
	// una opción válida pone la cuenta en cero
	e.dice(t, ana, ana, "zzz")
	e.dice(t, ana, ana, "1")
	if d := e.dice(t, ana, ana, "zzz"); !strings.HasPrefix(textos(d), "No entendí") {
		t.Fatalf("tras una opción válida vuelve a empezar la cuenta: %q", textos(d))
	}
	est, _ := e.alm.EstadisticasMenu(1, 1, "2026-01-01", "2026-12-31", false)
	var inv int64
	for _, f := range est {
		if f.Opcion == "invalida" {
			inv += f.Cuenta
		}
	}
	if inv != 5 {
		t.Fatalf("inválidas contadas: %d", inv)
	}
}

func TestPedirDatoGuardaLaVariableCifradaYNotifica(t *testing.T) {
	e := nuevo(t)
	e.crea(t, ventas(), true)
	e.dice(t, ana, ana, ".menu")
	d := e.dice(t, ana, ana, "3")
	if textos(d) != "¿Cuál es tu correo?" {
		t.Fatalf("%q", textos(d))
	}
	d = e.dice(t, ana, ana, "ana@correo.mx")
	if textos(d) != "Anotado: ana@correo.mx. Gracias, Ana." {
		t.Fatalf("%q", textos(d))
	}
	if len(e.avisos) != 1 || e.avisos[0].tipo != "solicitud" || e.avisos[0].titulo != "Nueva solicitud" || e.avisos[0].detalle != "Contacto: ana@correo.mx" {
		t.Fatalf("%+v", e.avisos)
	}
	s, _ := e.alm.SesionesEnCurso(1, e.rel.t, time.Hour)
	if len(s) != 0 {
		t.Fatal("terminó")
	}
	ev, _ := e.alm.Recorrido(1)
	for _, x := range ev {
		if strings.Contains(x.Paso+x.Opcion+x.Resultado, "correo") {
			t.Fatal("el texto libre no va en los eventos")
		}
	}
}

func TestPasarAHumanoCierraYCallaEsaPersona(t *testing.T) {
	e := nuevo(t)
	e.crea(t, ventas(), true)
	e.dice(t, ana, ana, ".menu")
	d := e.dice(t, ana, ana, "2")
	if !d.PasarAHumano || textos(d) != "Te paso con una persona." {
		t.Fatalf("%+v", d)
	}
	if len(e.avisos) != 1 || e.avisos[0].tipo != "humano" || e.avisos[0].titulo != "Alguien pidió hablar con una persona" || !strings.Contains(e.avisos[0].detalle, "Ana") {
		t.Fatalf("%+v", e.avisos)
	}
	if d := e.dice(t, ana, ana, "¿hola?"); d.Omitir != almacen.MotHumano || len(d.Respuestas) != 0 {
		t.Fatalf("callado mientras espera a una persona: %+v", d)
	}
	e.rel.t = e.rel.t.Add(13 * time.Hour)
	if d := e.dice(t, ana, ana, ".menu"); !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("pasadas 12 h vuelve a responder: %q", textos(d))
	}
}

func TestExpiraADas2HorasYMuestraElMenuOtraVez(t *testing.T) {
	e := nuevo(t)
	e.crea(t, ventas(), true)
	e.dice(t, ana, ana, ".menu")
	e.dice(t, ana, ana, "1")
	e.rel.t = e.rel.t.Add(2*time.Hour + time.Minute)
	if d := e.dice(t, ana, ana, "1"); len(d.Respuestas) != 0 { // "1" no es un disparador
		t.Fatalf("caducada y sin disparador: %+v", d)
	}
	est, _ := e.alm.EstadisticasMenu(1, 1, "2026-01-01", "2026-12-31", false)
	for _, f := range est {
		if f.Opcion == "expirado" && f.Cuenta == 1 {
			if d := e.dice(t, ana, ana, ".menu"); !strings.HasPrefix(textos(d), "Hola Ana") {
				t.Fatalf("%q", textos(d))
			}
			return
		}
	}
	t.Fatalf("debió contar un expirado: %v", est)
}

func TestVersionesLaConversacionTerminaConLaQueEmpezoYLosNuevosUsanLaNueva(t *testing.T) {
	e := nuevo(t)
	id := e.crea(t, ventas(), true)
	e.dice(t, ana, ana, ".menu")
	otra := "5215522222222@s.whatsapp.net"
	nueva := ventas()
	nueva.Pasos[2].Texto = "El plan básico ahora cuesta $120."
	nueva.Normalizar()
	v, err := e.alm.GuardarMenu(1, id, nueva.JSON())
	if err != nil || v != 2 {
		t.Fatal(v, err)
	}
	e.dice(t, ana, ana, "1")
	if d := e.dice(t, ana, ana, "1"); textos(d) != "El plan básico cuesta $100." {
		t.Fatalf("la que ya empezó sigue con su versión: %q", textos(d))
	}
	e.dice(t, otra, otra, ".menu")
	e.dice(t, otra, otra, "1")
	if d := e.dice(t, otra, otra, "1"); textos(d) != "El plan básico ahora cuesta $120." {
		t.Fatalf("las nuevas usan la actualizada: %q", textos(d))
	}
}

func TestSiBorranElPasoVuelveAlMenuPrincipalYSiBorranLaAutomatizacionSeCierra(t *testing.T) {
	e := nuevo(t)
	id := e.crea(t, ventas(), true)
	e.dice(t, ana, ana, ".menu")
	e.dice(t, ana, ana, "1") // está en el paso "p"
	// Una versión sin el paso "p"; la conversación sigue con su versión, pero si su paso ya no existe en ella vuelve al principio.
	s, _ := e.alm.SesionVigente(1, ana, ana, false, e.rel.t, time.Hour)
	if s == nil || s.Paso != "p" {
		t.Fatalf("%+v", s)
	}
	_, _ = e.alm.MoverSesion, 0
	if err := e.alm.MoverSesion(1, s, "paso-que-ya-no-existe", false, 0, false, struct{ Paso, Opcion, Resultado string }{}, e.rel.t); err != nil {
		t.Fatal(err)
	}
	if d := e.dice(t, ana, ana, "1"); !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("vuelve al menú principal: %q", textos(d))
	}
	if err := e.alm.BorrarMenu(1, id); err != nil {
		t.Fatal(err)
	}
	if d := e.dice(t, ana, ana, "1"); len(d.Respuestas) != 0 {
		t.Fatalf("sin automatización no hay respuesta: %+v", d)
	}
}

func TestEnGruposCadaPersonaTieneSuPropioMenu(t *testing.T) {
	e := nuevo(t)
	d := ventas()
	d.Alcance = Alcance{Grupos: []string{"120363000000000001@g.us"}}
	d.Disparadores = Disparadores{Comando: ".menu", Mencion: true}
	e.crea(t, d, true)
	g := "120363000000000001@g.us"
	ben, cla := "5215533333333@s.whatsapp.net", "5215544444444@s.whatsapp.net"
	e.dice(t, g, ben, ".menu")
	e.dice(t, g, cla, ".menu")
	e.dice(t, g, ben, "1") // Ben va por Precios
	if r := e.dice(t, g, cla, "2"); !r.PasarAHumano {
		t.Fatalf("Clara pidió una persona: %+v", r)
	}
	// Ben sigue en su menú aunque Clara esté con una persona
	if r := e.dice(t, g, ben, "1"); textos(r) != "El plan básico cuesta $100." {
		t.Fatalf("%q", textos(r))
	}
	if r := e.dice(t, g, cla, "hola"); r.Omitir != almacen.MotHumano {
		t.Fatalf("%+v", r)
	}
	// un grupo que no se eligió no activa nada
	if r := e.dice(t, "120363000000000099@g.us", ben, ".menu"); len(r.Respuestas) != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestSaludoPorDefectoTienePrioridadMasBajaYEnfriamiento(t *testing.T) {
	e := nuevo(t)
	e.alm.SembrarSaludo(1, SaludoJSON())
	e.alm.SembrarSaludo(1, SaludoJSON()) // solo una vez
	l, _ := e.alm.ListarMenus(1)
	if len(l) != 1 || !l[0].Sistema || !l[0].Activo {
		t.Fatalf("%+v", l)
	}
	if d := e.dice(t, ana, ana, "buenas"); textos(d) != "hola" || d.Coincidio != "Saludo" {
		t.Fatalf("%+v", d)
	}
	if d := e.dice(t, ana, ana, "otra vez"); len(d.Respuestas) != 0 {
		t.Fatalf("una vez por minuto: %+v", d)
	}
	e.rel.t = e.rel.t.Add(2 * time.Minute)
	if d := e.dice(t, ana, ana, "ya"); textos(d) != "hola" {
		t.Fatal("pasado el minuto vuelve a saludar")
	}
	// Si coincide otra automatización, el Saludo no responde.
	e.crea(t, ventas(), true)
	e.rel.t = e.rel.t.Add(5 * time.Minute)
	d := e.dice(t, ana, ana, "ventas")
	if d.Coincidio != "Ventas" || strings.Contains(textos(d), "hola") && !strings.Contains(textos(d), "Hola Ana") {
		t.Fatalf("%+v", d)
	}
	// Se puede apagar.
	_ = e.alm.FijarMenuActivo(1, l[0].ID, false)
	e.rel.t = e.rel.t.Add(10 * time.Minute)
	if d := e.dice(t, "5215566666666@s.whatsapp.net", "5215566666666@s.whatsapp.net", "hola"); len(d.Respuestas) != 0 {
		t.Fatalf("apagado no responde: %+v", d)
	}
}

func TestDisparadoresPrimerMensajeDiasPalabrasHorarioYFuera(t *testing.T) {
	e := nuevo(t)
	d := &Def{V: 1, Nombre: "Bienvenida", Inicio: "r", Pasos: []Paso{{ID: "r", Tipo: PasoRespuesta, Texto: "bienvenido"}},
		Disparadores: Disparadores{PrimerMensaje: true, DiasSinEscribir: 7}, Alcance: Alcance{Privados: true}}
	e.crea(t, d, true)
	nuevoChat := "5215577777777@s.whatsapp.net"
	if r := e.dice(t, nuevoChat, nuevoChat, "hola"); textos(r) != "bienvenido" {
		t.Fatalf("primer mensaje: %+v", r)
	}
	// ya hay historial reciente: no es primer mensaje
	for _, j := range []string{ana, "5215588888888@s.whatsapp.net"} {
		_, _ = e.alm.ActualizarChat(1, j, false, func(c *almacen.Chat) { c.Publico = "Alguien" })
	}
	for i, ts := range []time.Time{e.rel.t.Add(-48 * time.Hour)} {
		_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: fmt.Sprint("h", i), ChatJID: ana, BotID: 1, RemitenteJID: ana, Tipo: "texto", Texto: "viejo", TS: ts.Unix()})
	}
	if r := e.dice(t, ana, ana, "hola"); len(r.Respuestas) != 0 {
		t.Fatalf("hace 2 días no cuenta como 7 sin escribir: %+v", r)
	}
	_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: "h9", ChatJID: "5215588888888@s.whatsapp.net", BotID: 1, RemitenteJID: "5215588888888@s.whatsapp.net", Tipo: "texto", Texto: "viejo", TS: e.rel.t.Add(-10 * 24 * time.Hour).Unix()})
	o := "5215588888888@s.whatsapp.net"
	if r := e.dice(t, o, o, "hola"); textos(r) != "bienvenido" {
		t.Fatalf("hace 10 días sí: %+v", r)
	}
}

func TestHorarioYFueraDeHorario(t *testing.T) {
	e := nuevo(t) // martes 10 nov 2026, 05:30 hora de México
	abierto := &Def{V: 1, Nombre: "Abierto", Inicio: "r", Pasos: []Paso{{ID: "r", Tipo: PasoRespuesta, Texto: "estamos abiertos"}},
		Disparadores: Disparadores{Palabras: []string{"hola"}}, Alcance: Alcance{Privados: true}, Horario: Horario{Dias: []int{1, 2, 3, 4, 5}, Desde: "09:00", Hasta: "18:00"}}
	cerrado := &Def{V: 1, Nombre: "Cerrado", Inicio: "r", Pasos: []Paso{{ID: "r", Tipo: PasoRespuesta, Texto: "ahora estamos cerrados"}},
		Disparadores: Disparadores{FueraDeHorario: true}, Alcance: Alcance{Privados: true}, Horario: Horario{Dias: []int{1, 2, 3, 4, 5}, Desde: "09:00", Hasta: "18:00"}}
	e.crea(t, abierto, true)
	e.crea(t, cerrado, true)
	if r := e.dice(t, ana, ana, "hola"); textos(r) != "ahora estamos cerrados" {
		t.Fatalf("a las 05:30 está fuera de horario: %+v", r)
	}
	e.rel.t = time.Date(2026, 11, 10, 20, 0, 0, 0, time.UTC) // 14:00 en México
	o := "5215599999999@s.whatsapp.net"
	if r := e.dice(t, o, o, "hola"); textos(r) != "estamos abiertos" {
		t.Fatalf("a las 14:00 en horario: %+v", r)
	}
	e.rel.t = time.Date(2026, 11, 14, 20, 0, 0, 0, time.UTC) // sábado
	p := "5215500000099@s.whatsapp.net"
	if r := e.dice(t, p, p, "hola"); textos(r) != "ahora estamos cerrados" {
		t.Fatalf("sábado fuera de horario: %+v", r)
	}
}

func TestSimuladorUsaElMismoMotorYMarcaPrueba(t *testing.T) {
	e := nuevo(t)
	id := e.crea(t, ventas(), false) // ni siquiera hace falta que esté activa
	p, err := e.m.Probar(id, "hola", true, "")
	if err != nil || !strings.HasPrefix(strings.Join(p.Salidas, ""), "Hola Ana") || p.Estado != almacen.SesActiva {
		t.Fatal(p, err)
	}
	p, _ = e.m.Probar(id, "2", false, "")
	if p.Estado != almacen.SesHumano || len(e.avisos) != 0 {
		t.Fatalf("en pruebas no hay notificaciones: %+v %+v", p, e.avisos)
	}
	real, _ := e.alm.EstadisticasMenu(1, id, "2026-01-01", "2026-12-31", false)
	prueba, _ := e.alm.EstadisticasMenu(1, id, "2026-01-01", "2026-12-31", true)
	if len(real) != 0 || len(prueba) == 0 {
		t.Fatalf("las pruebas van aparte: real=%v prueba=%v", real, prueba)
	}
	if s, _ := e.alm.SesionesEnCurso(1, e.rel.t, time.Hour); len(s) != 0 {
		t.Fatal("las pruebas no aparecen en 'En curso ahora'")
	}
	// reiniciar empieza de cero
	p, _ = e.m.Probar(id, "x", true, "")
	if !strings.HasPrefix(strings.Join(p.Salidas, ""), "Hola Ana") {
		t.Fatal(p)
	}
}

func TestValidacion(t *testing.T) {
	d := &Def{V: 1, Nombre: "Rota", Inicio: "m", Pasos: []Paso{
		{ID: "m", Tipo: PasoMenu, Titulo: "Menú", Opciones: []Opcion{{Texto: "Uno", Destino: "a"}, {Texto: "Dos"}, {Texto: "Tres", Destino: "no-existe"}}},
		{ID: "a", Tipo: PasoMenu, Titulo: "A", Opciones: []Opcion{{Texto: "Ir a B", Destino: "b"}}},
		{ID: "b", Tipo: PasoMenu, Titulo: "B", Opciones: []Opcion{{Texto: "Ir a A", Destino: "a"}}},
		{ID: "suelto", Tipo: PasoRespuesta, Texto: "nadie llega aquí"},
	}}
	av := d.Validar()
	tiene := func(paso, frag string) bool {
		for _, a := range av {
			if a.Paso == paso && strings.Contains(a.Texto, frag) {
				return true
			}
		}
		return false
	}
	if !tiene("m", "no lleva a ningún paso") || !tiene("m", "ya no existe") || !tiene("suelto", "queda suelto") || !tiene("a", "ciclo sin salida") || !tiene("b", "ciclo sin salida") {
		t.Fatalf("%+v", av)
	}
	if !tiene("", "No tiene cuándo activarse") || !tiene("", "No está asignada") {
		t.Fatalf("%+v", av)
	}
	// una definición sana no da avisos graves
	ok := ventas()
	ok.Normalizar()
	for _, a := range ok.Validar() {
		if a.Grave {
			t.Fatalf("no debía haber avisos graves: %+v", a)
		}
	}
	for _, p := range Plantillas() {
		for _, a := range p.Def.Validar() {
			if a.Grave {
				t.Fatalf("plantilla %q: %+v", p.Nombre, a)
			}
		}
	}
	if len(Plantillas()) != 8 { // las 3 de siempre + refacciones, soporte, citas, preguntas frecuentes y grupo (etapa C)
		t.Fatal("ocho plantillas de arranque")
	}
}

func TestNormalizarAsignaIdsYVariables(t *testing.T) {
	d := &Def{Pasos: []Paso{{Tipo: PasoPedir, Pregunta: "¿a?"}, {Tipo: PasoPedir, Pregunta: "¿b?", Variable: "dato1"}, {Tipo: PasoPedir, Pregunta: "¿c?"}, {ID: "p1", Tipo: PasoFin}}}
	d.Normalizar()
	ids := map[string]bool{}
	vars := map[string]bool{}
	for _, p := range d.Pasos {
		if p.ID == "" || ids[p.ID] {
			t.Fatalf("ids: %+v", d.Pasos)
		}
		ids[p.ID] = true
		if p.Tipo == PasoPedir {
			if p.Variable == "" || vars[p.Variable] {
				t.Fatalf("variables: %+v", d.Pasos)
			}
			vars[p.Variable] = true
		}
	}
	if d.Inicio != d.Pasos[0].ID || d.Nombre == "" {
		t.Fatalf("%+v", d)
	}
}

func TestLasConversacionesEnCursoYSuRecorrido(t *testing.T) {
	e := nuevo(t)
	e.crea(t, ventas(), true)
	e.dice(t, ana, ana, ".menu")
	e.dice(t, ana, ana, "1")
	en, err := e.alm.SesionesEnCurso(1, e.rel.t, 2*time.Hour)
	if err != nil || len(en) != 1 || en[0].Paso != "p" || en[0].Nombre == "" {
		t.Fatal(en, err)
	}
	rec, _ := e.alm.Recorrido(en[0].ID)
	var pasos []string
	for _, r := range rec {
		pasos = append(pasos, r.Paso+":"+r.Resultado)
	}
	if strings.Join(pasos, ",") != "m:mostrado,m:valida,p:mostrado" {
		t.Fatalf("%v", pasos)
	}
	// silenciar a una persona (botón "Pausar bot con esta persona")
	if err := e.alm.SilenciarSesion(1, ana, ana, en[0].MenuID, en[0].MenuVersion, almacen.SesPausada, e.rel.t.Add(time.Hour), e.rel.t); err != nil {
		t.Fatal(err)
	}
	if d := e.dice(t, ana, ana, "1"); d.Omitir == "" {
		t.Fatalf("pausada: %+v", d)
	}
}

func TestConMenuActivoEIntervencionManualSoloRespondeAOpcionesOComandos(t *testing.T) {
	e := nuevo(t)
	e.crea(t, ventas(), true)
	e.dice(t, ana, ana, ".menu")
	// Intervine a mano: el texto libre ya no recibe "opción no válida" ni cuenta como error.
	e.alm.MarcarIntervencion(1, ana)
	for _, libre := range []string{"hola, ya te atiendo", "xyz", "ok", "gracias"} {
		d := e.dice(t, ana, ana, libre)
		if len(d.Respuestas) != 0 || d.Omitir != almacen.MotIntervino {
			t.Fatalf("%q: el bot debía callar por intervención: %+v", libre, d)
		}
	}
	s, _ := e.alm.SesionVigente(1, ana, ana, false, e.rel.t, 2*time.Hour)
	if s == nil || s.Invalidas != 0 || s.Cortado {
		t.Fatalf("el texto libre no debe contar como error: %+v", s)
	}
	// Una opción válida sí se atiende y devuelve la palabra al bot.
	if d := e.dice(t, ana, ana, "1"); !strings.Contains(textos(d), "Precios") {
		t.Fatalf("la opción válida se responde: %q", textos(d))
	}
	if e.alm.SesionIntervenida(s.ID) {
		t.Fatal("tras una opción válida la marca se quita")
	}
	if d := e.dice(t, ana, ana, "zzz"); !strings.HasPrefix(textos(d), "No entendí") {
		t.Fatalf("sin intervención vuelve el comportamiento normal: %q", textos(d))
	}
	// El comando vuelve a abrir el menú aun con intervención.
	e.alm.MarcarIntervencion(1, ana)
	if d := e.dice(t, ana, ana, ".menu"); !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("el comando responde: %q", textos(d))
	}
	// El "0" también cuenta como opción válida.
	e.dice(t, ana, ana, "1")
	e.alm.MarcarIntervencion(1, ana)
	if d := e.dice(t, ana, ana, "0"); !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("0 vuelve al menú: %q", textos(d))
	}
}
