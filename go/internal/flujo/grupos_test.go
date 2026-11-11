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

const (
	grupo = "120363000000000001@g.us"
	pnBen = "5215533333333@s.whatsapp.net"
	lidBn = "9988776655@lid"
	pnCla = "5215544444444@s.whatsapp.net"
	pnDan = "5215566666666@s.whatsapp.net"
	pnEva = "5215577777777@s.whatsapp.net"
)

func menuGrupo() *Def {
	d := tienda()
	d.Alcance = Alcance{Privados: true, TodosGrupos: true}
	d.Disparadores = Disparadores{Mencion: true, Palabras: []string{"tienda"}, Comando: ".menu"}
	return d
}

type msgG struct {
	quien      string // JID tal como lo manda WhatsApp (puede ser LID)
	texto      string
	mencion    bool
	citaID     string
	citaEsMia  bool
	noTexto    bool
}

// grupoEntorno prepara un grupo con alias número↔LID para Ben.
func grupoEntorno(t *testing.T) *entorno {
	e := nuevo(t)
	e.crea(t, menuGrupo(), true)
	e.alm.AprenderAlias(1, pnBen, lidBn)
	e.m.c.Persona = func(p string) string { return e.alm.ResolverPersona(1, p) }
	return e
}

var seqG int

func (e *entorno) en(t *testing.T, m msgG) motor.Decision {
	t.Helper()
	e.rel.t = e.rel.t.Add(3 * time.Second)
	seqG++
	u := motor.Mensaje{Chat: grupo, ID: fmt.Sprintf("G%d", seqG), Remitente: m.quien, RemitenteWA: m.quien, Texto: m.texto, EsTexto: !m.noTexto, TS: e.rel.t, EsGrupo: true,
		Mencionado: m.mencion, CitaID: m.citaID, CitaEsMia: m.citaEsMia || m.citaID != ""}
	// quien escribe se guarda con su número si se conoce (como la ingesta)
	u.Remitente = e.alm.ResolverPersona(1, m.quien)
	d, err := e.m.Decidir(context.Background(), u, []motor.Mensaje{u})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// registra liga lo que contestó el bot con su conversación (lo hace el motor al enviar) y devuelve el id del "mensaje del bot".
func (e *entorno) registra(t *testing.T, d motor.Decision) string {
	t.Helper()
	if d.Sesion == nil {
		t.Fatalf("la decisión debe traer la conversación: %+v", d)
	}
	seqG++
	id := fmt.Sprintf("BOT%d", seqG)
	if err := e.alm.GuardarMsgBot(1, grupo, id, d.Sesion.ID, d.Sesion.Persona, d.Sesion.MenuID, e.rel.t); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestSesionPorGrupoYPersonaCanonicaNumeroYLID(t *testing.T) {
	e := grupoEntorno(t)
	d := e.en(t, msgG{quien: lidBn, texto: "tienda", mencion: true})
	if !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("%q", textos(d))
	}
	// la misma persona escribiendo con su número continúa la misma conversación
	if d := e.en(t, msgG{quien: pnBen, texto: "contacto"}); !strings.Contains(textos(d), "Contacto\n1. Teléfono") {
		t.Fatalf("misma conversación por número y LID: %q", textos(d))
	}
	ss, _ := e.alm.SesionesDeChat(1, grupo, e.rel.t, 2*time.Hour)
	if len(ss) != 1 || ss[0].Sender != pnBen {
		t.Fatalf("una sola conversación, con la clave canónica: %+v", ss)
	}
	// otra persona tiene la suya
	e.en(t, msgG{quien: pnCla, texto: "tienda", mencion: true})
	e.en(t, msgG{quien: pnCla, texto: "3"})
	ss, _ = e.alm.SesionesDeChat(1, grupo, e.rel.t, 2*time.Hour)
	if len(ss) != 1 || ss[0].Sender != pnBen {
		t.Fatalf("Clara terminó la suya; la de Ben sigue: %+v", ss)
	}
	// una persona no puede tener dos conversaciones en el mismo grupo
	e.en(t, msgG{quien: pnBen, texto: "!menu"})
	if n := e.alm.ContarSesionesGrupo(1, grupo, e.rel.t, 10*time.Minute); n != 1 {
		t.Fatalf("una sesión por persona y grupo: %d", n)
	}
	// y en otro grupo, otra conversación distinta
	e.m.c.Alm.SesionVigente(1, grupo, pnBen, false, e.rel.t, time.Hour)
}

func TestArrancaConMencionPrefijoOCitaYNoConCharla(t *testing.T) {
	e := grupoEntorno(t)
	// charla sin mención ni cita ni prefijo: nada, aunque diga la palabra
	d := e.en(t, msgG{quien: pnBen, texto: "la tienda está cerca"})
	if d.Omitir != almacen.MotGrupo || len(d.Respuestas) != 0 {
		t.Fatalf("charla: %+v", d)
	}
	// prefijo de comando
	d = e.en(t, msgG{quien: pnBen, texto: "!menu"})
	if !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("prefijo: %q", textos(d))
	}
	e.en(t, msgG{quien: pnBen, texto: "salir"})
	// mención
	if d := e.en(t, msgG{quien: pnBen, texto: "tienda", mencion: true}); !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("mención: %q", textos(d))
	}
	e.en(t, msgG{quien: pnBen, texto: "salir"})
	// cita a un mensaje del bot de un menú que terminó: se puede abrir otro con el comando; citar solo no inventa uno
	d = e.en(t, msgG{quien: pnBen, texto: "!menu"})
	idBot := e.registra(t, d)
	e.en(t, msgG{quien: pnBen, texto: "3"}) // horario: termina
	d = e.en(t, msgG{quien: pnBen, texto: "otra vez porfa", citaID: idBot})
	if !strings.HasPrefix(textos(d), "Esa conversación terminó. Escribe !menu para empezar otra.") {
		t.Fatalf("cita a un menú vencido: %+v", d)
	}
}

func TestMensajeSinCitarSoloSeTomaConSesionYOpcionOComandoValido(t *testing.T) {
	e := grupoEntorno(t)
	msg := func(quien, texto string) motor.Mensaje {
		return motor.Mensaje{Chat: grupo, Remitente: e.alm.ResolverPersona(1, quien), RemitenteWA: quien, Texto: texto, EsTexto: true, EsGrupo: true, TS: e.rel.t}
	}
	// sin conversación: nada
	if e.m.AdmiteEnGrupo(msg(pnBen, "3")) || e.m.AdmiteEnGrupo(msg(pnBen, "menú")) {
		t.Fatal("sin conversación abierta no se toma nada")
	}
	e.en(t, msgG{quien: lidBn, texto: "tienda", mencion: true})
	for _, ok := range []string{"3", "horario", "hor", "contacto", "menú", "salir", "repetir", "persona", "cancelar"} {
		if !e.m.AdmiteEnGrupo(msg(lidBn, ok)) || !e.m.AdmiteEnGrupo(msg(pnBen, ok)) {
			t.Fatalf("%q es una opción o comando válido de su conversación", ok)
		}
	}
	for _, no := range []string{"jaja qué bueno", "¿alguien sabe dónde queda?", "hola", "9", "ok", "pr"} {
		if e.m.AdmiteEnGrupo(msg(pnBen, no)) {
			t.Fatalf("%q no es opción ni comando", no)
		}
	}
	// otra persona sin conversación no cuenta aunque escriba un número válido
	if e.m.AdmiteEnGrupo(msg(pnCla, "3")) {
		t.Fatal("Clara no tiene conversación")
	}
	// un mensaje que no es texto tampoco se admite sin mención ni cita
	foto := msg(pnBen, "")
	foto.EsTexto = false
	if e.m.AdmiteEnGrupo(foto) || e.m.EsperaTexto(foto) {
		t.Fatal("una foto suelta en el grupo no es para el bot")
	}
	foto.Mencionado = true
	if !e.m.EsperaTexto(foto) {
		t.Fatal("una foto que menciona al bot, con menú abierto, sí tiene respuesta")
	}
	// en un formulario o pregunta libre hace falta mencionar o citar (si no, la charla se tomaría como respuesta)
	e.en(t, msgG{quien: pnBen, texto: "5"})
	if e.m.AdmiteEnGrupo(msg(pnBen, "Ana López")) {
		t.Fatal("en un formulario, el texto libre sin mención ni cita no se toma")
	}
	if !e.m.AdmiteEnGrupo(msg(pnBen, "cancelar")) {
		t.Fatal("cancelar siempre vale")
	}
	// y dirigido al bot, un mensaje que cita o menciona puede recibir "no entendí"
	e.en(t, msgG{quien: pnBen, texto: "cancelar"})
	e.en(t, msgG{quien: pnBen, texto: "!menu"})
	if d := e.en(t, msgG{quien: pnBen, texto: "blablabla", mencion: true}); !strings.HasPrefix(textos(d), "No entendí") {
		t.Fatalf("dirigido: %q", textos(d))
	}
	// sin mención ni cita llegando a Decidir (por una carrera), se ignora
	if d := e.en(t, msgG{quien: pnBen, texto: "blablabla otra vez"}); d.Omitir != almacen.MotGrupo || len(d.Respuestas) != 0 {
		t.Fatalf("no dirigido: %+v", d)
	}
}

func TestOtraPersonaCitandoUnMenuAjenoSeIgnoraPorDefecto(t *testing.T) {
	e := grupoEntorno(t)
	d := e.en(t, msgG{quien: pnBen, texto: "tienda", mencion: true})
	bot := e.registra(t, d)
	// Clara contesta citando el menú de Ben: se ignora
	d = e.en(t, msgG{quien: pnCla, texto: "3", citaID: bot})
	if len(d.Respuestas) != 0 || d.Omitir != "menú de otra persona" {
		t.Fatalf("%+v", d)
	}
	if s := e.sesion(t, grupo, pnBen); s == nil || s.Paso != "m" {
		t.Fatalf("la conversación de Ben no se movió: %+v", s)
	}
	// con el ajuste "cualquiera puede contestar" se toma como si la dueña lo hubiera dicho
	e.conf(t, "grupo_cualquiera", "1")
	d = e.en(t, msgG{quien: pnCla, texto: "4", citaID: bot})
	if !strings.Contains(textos(d), "Contacto\n1. Teléfono") {
		t.Fatalf("cualquiera puede: %q", textos(d))
	}
	if s := e.sesion(t, grupo, pnBen); s == nil || s.Paso != "c" {
		t.Fatalf("avanzó la conversación de Ben: %+v", s)
	}
	if e.sesion(t, grupo, pnCla) != nil {
		t.Fatal("Clara no abrió una conversación propia")
	}
	// el ajuste del menú manda sobre el general
	e2 := grupoEntorno(t)
	e2.conf(t, "grupo_cualquiera", "1")
	m := menuGrupo()
	m.Ajustes = &AjustesMenu{Cualquiera: "no"}
	e2.alm.GuardarMenu(1, 1, m.JSON())
	d = e2.en(t, msgG{quien: pnBen, texto: "tienda", mencion: true})
	bot = e2.registra(t, d)
	if d := e2.en(t, msgG{quien: pnCla, texto: "3", citaID: bot}); d.Omitir != "menú de otra persona" {
		t.Fatalf("por menú manda: %+v", d)
	}
}

func TestTopeDeSesionesConFilaDeEsperaYTurnoAvisadoUnaVez(t *testing.T) {
	e := grupoEntorno(t)
	e.conf(t, "grupo_max_sesiones", "2")
	e.en(t, msgG{quien: pnBen, texto: "tienda", mencion: true})
	e.en(t, msgG{quien: pnCla, texto: "tienda", mencion: true})
	// la tercera persona espera: se le dice una sola vez en qué turno está
	d := e.en(t, msgG{quien: pnDan, texto: "tienda", mencion: true})
	if textos(d) != "Hay varias personas usando el menú. Estás en el turno 1; te atiendo en cuanto se libere un lugar." {
		t.Fatalf("%q", textos(d))
	}
	if d := e.en(t, msgG{quien: pnDan, texto: "¿ya?", mencion: true}); len(d.Respuestas) != 0 {
		t.Fatalf("el turno se avisa una vez: %+v", d)
	}
	d = e.en(t, msgG{quien: pnEva, texto: "tienda", mencion: true})
	if !strings.Contains(textos(d), "turno 2") {
		t.Fatalf("%q", textos(d))
	}
	if df := e.m.Diferidas(e.rel.t); len(df) != 0 {
		t.Fatalf("sigue lleno: %+v", df)
	}
	// Ben termina → se atiende a Dani, citando su mensaje y mencionándola; Eva sigue esperando
	e.dice(t, grupo, pnBen, "salir")
	df := e.m.Diferidas(e.rel.t)
	if len(df) != 1 || df[0].Chat != grupo || df[0].MencionWA != pnDan || df[0].CitaAutorWA != pnDan || !strings.HasPrefix(textos(df[0].D), "Hola Ana") || df[0].CitaID == "" {
		t.Fatalf("%+v", df)
	}
	if s := e.sesion(t, grupo, pnDan); s == nil || s.Paso != "m" {
		t.Fatalf("Dani ya tiene su conversación: %+v", s)
	}
	if df[0].D.Sesion == nil || df[0].D.Sesion.Persona != pnDan {
		t.Fatalf("la respuesta diferida también liga su conversación: %+v", df[0].D.Sesion)
	}
	if e.alm.TurnoEnCola(1, grupo, pnEva) != 1 {
		t.Fatal("Eva pasa al turno 1")
	}
	// una conversación que caduca en silencio libera su lugar sola
	e.rel.t = e.rel.t.Add(11 * time.Minute)
	df = e.m.Diferidas(e.rel.t)
	if len(df) != 0 {
		// Eva espera desde hace 11 min: la fila dura 10 por defecto, así que se descarta sin avisar
		t.Fatalf("%+v", df)
	}
	if e.alm.TurnoEnCola(1, grupo, pnEva) != 0 {
		t.Fatal("quien espera demasiado sale de la fila sin que le escriban")
	}
	e.conf(t, "grupo_cola_min", "60")
	e.en(t, msgG{quien: pnEva, texto: "tienda", mencion: true}) // Clara y Dani caducaron: hay lugar
	if e.sesion(t, grupo, pnEva) == nil {
		t.Fatal("con lugar libre entra directo")
	}
	// el tope también se puede poner por menú
	e3 := grupoEntorno(t)
	m := menuGrupo()
	m.Ajustes = &AjustesMenu{MaxSesionesGrupo: 1}
	e3.alm.GuardarMenu(1, 1, m.JSON())
	e3.en(t, msgG{quien: pnBen, texto: "tienda", mencion: true})
	if d := e3.en(t, msgG{quien: pnCla, texto: "tienda", mencion: true}); !strings.Contains(textos(d), "turno 1") {
		t.Fatalf("%q", textos(d))
	}
}

func TestConversacionVencidaSeAvisaUnaSolaVezPorPersonaYMenu(t *testing.T) {
	e := grupoEntorno(t)
	d := e.en(t, msgG{quien: pnBen, texto: "tienda", mencion: true})
	bot := e.registra(t, d)
	d = e.en(t, msgG{quien: pnBen, texto: "3"}) // horario: termina
	botFin := e.registra(t, d)
	d = e.en(t, msgG{quien: pnBen, texto: "gracias", citaID: bot})
	if textos(d) != "Esa conversación terminó. Escribe !menu para empezar otra." {
		t.Fatalf("%q", textos(d))
	}
	if d := e.en(t, msgG{quien: pnBen, texto: "oye", citaID: botFin}); len(d.Respuestas) != 0 {
		t.Fatalf("una sola vez: %+v", d)
	}
	// otra persona citando ese menú terminado es un menú ajeno: se ignora
	if d := e.en(t, msgG{quien: pnCla, texto: "oye", citaID: bot}); d.Omitir != "menú de otra persona" {
		t.Fatalf("%+v", d)
	}
	// el texto del aviso es editable y respeta el prefijo
	e.conf(t, "grupo_txt_vencido", "Se acabó. Usa {prefijo}menu.")
	e.m.c.Prefijo = func() string { return "#" }
	d = e.en(t, msgG{quien: pnCla, texto: "tienda", mencion: true})
	b2 := e.registra(t, d)
	e.en(t, msgG{quien: pnCla, texto: "3"})
	if d := e.en(t, msgG{quien: pnCla, texto: "y ahora", citaID: b2}); textos(d) != "Se acabó. Usa #menu." {
		t.Fatalf("%q", textos(d))
	}
	// si cita y a la vez abre el menú con el comando, se abre (no se le dice que terminó)
	if d := e.en(t, msgG{quien: pnBen, texto: "#menu", citaID: bot}); !strings.HasPrefix(textos(d), "Hola Ana") {
		t.Fatalf("%q", textos(d))
	}
}

func TestExpiracionEnSilencioMasCortaEnGruposYAjustable(t *testing.T) {
	e := grupoEntorno(t)
	e.en(t, msgG{quien: pnBen, texto: "tienda", mencion: true})
	e.rel.t = e.rel.t.Add(9 * time.Minute)
	if e.sesion(t, grupo, pnBen) == nil {
		t.Fatal("a los 9 minutos sigue (el valor de grupo es 10)")
	}
	e.rel.t = e.rel.t.Add(2 * time.Minute)
	if e.sesion(t, grupo, pnBen) != nil {
		t.Fatal("a los 11 minutos caducó")
	}
	// en privado el valor general es de 2 horas
	e.dice(t, ana, ana, ".menu")
	e.rel.t = e.rel.t.Add(30 * time.Minute)
	if e.sesion(t, ana, ana) == nil {
		t.Fatal("el privado dura más")
	}
	// ajustable: 3 minutos en grupos
	e.conf(t, "grupo_expira_min", "3")
	e.en(t, msgG{quien: pnCla, texto: "tienda", mencion: true})
	e.rel.t = e.rel.t.Add(4 * time.Minute)
	if e.sesion(t, grupo, pnCla) != nil {
		t.Fatal("ajuste de 3 minutos")
	}
}
