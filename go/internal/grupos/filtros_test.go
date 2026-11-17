package grupos

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
)

func (e *entorno) filtro(nombre string, activo bool, accion string, f func(*ConfigFiltro)) {
	var x Filtro
	for _, y := range e.s.FiltrosDe(grupoJID) {
		if y.Filtro == nombre {
			x = y
		}
	}
	x.Activo, x.Accion = activo, accion
	if f != nil {
		f(&x.Config)
	}
	if err := e.s.GuardarFiltro("yo", x); err != nil {
		panic(err)
	}
}

// msg guarda el mensaje (la ingesta lo guarda antes de que se evalúe) y arma la entrada del filtro.
func (e *entorno) msg(quien, texto string) Entrada {
	_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: "MSG-" + texto, ChatJID: grupoJID, BotID: 1, RemitenteJID: quien, Tipo: almacen.TipoTexto, Texto: texto, TS: e.hora.Unix()})
	return Entrada{Chat: grupoJID, MensajeID: "MSG-" + texto, Persona: quien, PersonaWA: quien, Nombre: "Luis", Texto: texto}
}

func (e *entorno) avisos() []almacen.Aviso {
	l, _ := e.alm.AvisosRecientes(1, 50)
	var out []almacen.Aviso
	for _, a := range l {
		if a.Tipo == "filtros" {
			out = append(out, a)
		}
	}
	return out
}

func TestFiltroDeEnlacesConListaPermitida(t *testing.T) {
	e := nuevo(t, true)
	e.filtro(FiltroEnlaces, true, AccAvisarme, func(c *ConfigFiltro) { c.Permitidos = []string{"https://www.YouTube.com/", "miescuela.edu.mx"} })
	cfg := e.s.FiltrosDe(grupoJID)[0].Config
	if len(cfg.Permitidos) != 2 || cfg.Permitidos[0] != "youtube.com" {
		t.Fatalf("debe normalizar los dominios: %v", cfg.Permitidos)
	}
	casos := []struct {
		texto string
		malo  bool
	}{
		{"mira https://m.youtube.com/watch?v=1", false},
		{"youtube.com/algo", false},
		{"entra a www.miescuela.edu.mx/tareas", false},
		{"https://youtube.com.estafa.xyz/x", true},
		{"https://notyoutube.com", true},
		{"únete chat.whatsapp.com/ABCDEF", true},
		{"www.tienda.mx", true},
		{"visita ejemplo.com ya", true},
		{"mi correo es ana@gmail.com", false},
		{"cuesta 3.5 pesos, nos vemos a las 4.30", false},
		{"hola a todos", false},
		{"Fin.Hola", false},
	}
	for i, c := range casos {
		got := e.s.Evaluar(e.msg(pnLuis, c.texto)) != nil
		if got != c.malo {
			t.Fatalf("%d %q: esperaba malo=%v", i, c.texto, c.malo)
		}
	}
	// Sin lista permitida, todos los enlaces cuentan.
	e.filtro(FiltroEnlaces, true, AccAvisarme, func(c *ConfigFiltro) { c.Permitidos = nil })
	e.hora = e.hora.Add(time.Minute) // vence la caché de filtros
	if e.s.Evaluar(e.msg(pnLuis, "youtube.com/algo")) == nil {
		t.Fatal("sin permitidos debe contar")
	}
}

func TestFiltroDePalabrasProhibidas(t *testing.T) {
	e := nuevo(t, true)
	e.filtro(FiltroPalabras, true, AccAvisarme, func(c *ConfigFiltro) { c.Palabras = []string{"Estafa", "no sirve", "camión"} })
	casos := []struct {
		texto string
		malo  bool
	}{
		{"Es una ESTAFA!!!", true},
		{"es una estafá", true},
		{"ese estafador", false}, // palabra completa
		{"esto NO   sirve nada", true},
		{"no, esto sirve", false},
		{"llegó el camion", true},
		{"nada raro", false},
	}
	for _, c := range casos {
		if got := e.s.Evaluar(e.msg(pnLuis, c.texto)) != nil; got != c.malo {
			t.Fatalf("%q: malo=%v", c.texto, got)
		}
	}
	if i := e.s.Evaluar(e.msg(pnLuis, "una estafa")); i == nil || i.Motivo != "palabra" {
		t.Fatal("motivo")
	}
}

func TestFiltroRepetidosYFlood(t *testing.T) {
	e := nuevo(t, true)
	e.filtro(FiltroRepetidos, true, AccAvisarme, func(c *ConfigFiltro) { c.Max, c.Segundos = 3, 60 })
	e.filtro(FiltroFlood, true, AccAvisarme, func(c *ConfigFiltro) { c.Max, c.Segundos = 5, 10 })
	// Tres veces lo mismo (aunque cambie la puntuación o las mayúsculas) dentro de la ventana.
	if e.s.Evaluar(e.msg(pnLuis, "COMPRA YA!")) != nil || e.s.Evaluar(e.msg(pnLuis, "compra ya")) != nil {
		t.Fatal("todavía no")
	}
	if i := e.s.Evaluar(e.msg(pnLuis, "compra, ya")); i == nil || i.Motivo != "repetido" {
		t.Fatalf("%+v", i)
	}
	// Después de saltar empieza de cero (no salta en cada mensaje siguiente).
	if e.s.Evaluar(e.msg(pnLuis, "compra ya")) != nil {
		t.Fatal("una sola vez por tanda")
	}
	// Otra persona cuenta aparte.
	if e.s.Evaluar(e.msg(pnRosa, "compra ya")) != nil || e.s.Evaluar(e.msg(pnRosa, "compra ya")) != nil {
		t.Fatal("otra persona")
	}
	// Fuera de la ventana no cuenta.
	e.hora = e.hora.Add(2 * time.Minute)
	if e.s.Evaluar(e.msg(pnLuis, "compra ya")) != nil || e.s.Evaluar(e.msg(pnLuis, "compra ya")) != nil {
		t.Fatal("lo de hace 2 minutos ya no cuenta")
	}
	// Flood: 5 mensajes distintos en 10 segundos saltan; si van espaciados no.
	e.hora = e.hora.Add(5 * time.Minute)
	var i *Infraccion
	for k := 0; k < 5; k++ {
		i = e.s.Evaluar(e.msg(pnAna, "m"+string(rune('a'+k))))
		e.hora = e.hora.Add(time.Second)
	}
	if i == nil || i.Motivo != "flood" {
		t.Fatalf("%+v", i)
	}
	e.hora = e.hora.Add(time.Minute)
	for k := 0; k < 8; k++ {
		if e.s.Evaluar(e.msg(pnRosa, "x"+string(rune('a'+k)))) != nil {
			t.Fatal("espaciados no es flood")
		}
		e.hora = e.hora.Add(4 * time.Second)
	}
}

func TestLosFiltrosNoMiranLoQueNoDeben(t *testing.T) {
	e := nuevo(t, true)
	e.filtro(FiltroEnlaces, true, AccAvisarme, nil)
	base := e.msg(pnLuis, "https://malo.com")
	if e.s.Evaluar(base) == nil {
		t.Fatal("base")
	}
	m := base
	m.EsMio = true
	if e.s.Evaluar(m) != nil {
		t.Fatal("lo que escribo yo")
	}
	m = base
	m.Sincronizando = true
	if e.s.Evaluar(m) != nil {
		t.Fatal("historial")
	}
	m = base
	m.Chat = pnLuis
	if e.s.Evaluar(m) != nil {
		t.Fatal("privado")
	}
	m = base
	m.Chat = "otro@g.us"
	if e.s.Evaluar(m) != nil {
		t.Fatal("otro grupo sin filtros")
	}
	// Filtros apagados no evalúan.
	e.filtro(FiltroEnlaces, false, AccAvisarme, nil)
	e.hora = e.hora.Add(time.Minute)
	if e.s.Evaluar(base) != nil {
		t.Fatal("apagado")
	}
}

func TestExcepcionesPorRangoYAdmins(t *testing.T) {
	e := nuevo(t, true)
	e.rol("Vip")
	e.rol("Raso")
	e.da(pnLuis, "Vip")
	e.da(pnRosa, "Raso")
	e.filtro(FiltroEnlaces, true, AccBorrar, func(c *ConfigFiltro) { c.ExentoAdmins = true; c.ExentoRangos = []int64{e.ids["Vip"]} })
	aplica := func(quien string) {
		i := e.s.Evaluar(e.msg(quien, "https://malo.com/"+quien))
		if i == nil {
			t.Fatal("debía romper el filtro")
		}
		e.s.Aplicar(ctx(), i)
	}
	aplica(pnAna)  // administradora del grupo: exenta
	aplica(pnLuis) // rango exento
	aplica(almacen.PersonaDueno)
	if len(e.borr) != 0 || len(e.avisos()) != 0 {
		t.Fatalf("los exentos no se tocan: %v %v", e.borr, e.avisos())
	}
	aplica(pnRosa) // rango no exento
	if len(e.borr) != 1 {
		t.Fatalf("Rosa no está exenta: %v", e.borr)
	}
	// Si los administradores dejan de ser exentos, la administradora también cuenta.
	e.filtro(FiltroEnlaces, true, AccBorrar, func(c *ConfigFiltro) { c.ExentoAdmins = false; c.ExentoRangos = nil })
	e.hora = e.hora.Add(time.Minute)
	aplica(pnAna)
	if len(e.borr) != 2 {
		t.Fatalf("%v", e.borr)
	}
}

func TestAccionesDelFiltroSiendoAdmin(t *testing.T) {
	e := nuevo(t, true)
	var advertidos []string
	e.s.Advertir = func(_ context.Context, obj, chat, id, motivo string) error {
		advertidos = append(advertidos, obj+"|"+chat+"|"+id+"|"+motivo)
		return nil
	}
	hecho := func(accion string, f func(*ConfigFiltro)) {
		e.filtro(FiltroEnlaces, true, accion, f)
		e.hora = e.hora.Add(time.Minute)
		e.s.Aplicar(ctx(), e.s.Evaluar(e.msg(pnLuis, "https://malo.com")))
	}
	hecho(AccBorrar, nil)
	if len(e.borr) != 1 || e.borr[0] != grupoJID+"|MSG-https://malo.com" {
		t.Fatalf("%v", e.borr)
	}
	hecho(AccExpulsar, nil)
	if !e.cli.Hubo("UpdateGroupParticipants|" + jid(grupoJID).String() + "|remove|[" + jid(pnLuis).String() + "]") {
		t.Fatalf("%v", e.cli.Llamadas)
	}
	if av := e.avisos(); len(av) != 1 || !strings.Contains(av[0].Detalle, "expulsó") {
		t.Fatalf("al expulsar también aviso: %+v", av)
	}
	hecho(AccAvisarGrupo, func(c *ConfigFiltro) { c.Aviso = "Nada de enlaces, por favor." })
	if len(e.env) != 1 || !strings.HasSuffix(e.env[0].texto, "Nada de enlaces, por favor.") || !strings.HasPrefix(e.env[0].texto, "@5215522222222 ") ||
		e.env[0].cita != "MSG-https://malo.com" || len(e.env[0].menciones) != 1 {
		t.Fatalf("%+v", e.env)
	}
	hecho(AccAdvertir, nil)
	if len(advertidos) != 1 || !strings.HasPrefix(advertidos[0], pnLuis+"|"+grupoJID+"|MSG-https://malo.com|filtro enlaces") {
		t.Fatalf("%v", advertidos)
	}
	hecho(AccSilenciar, func(c *ConfigFiltro) { c.SilencioMin = 30 })
	if s := e.alm.SilencioDe(1, pnLuis, e.hora); s == nil || s.HastaTS != e.hora.Add(30*time.Minute).Unix() {
		t.Fatalf("%+v", s)
	}
	hecho(AccAvisarme, nil)
	acs, _ := e.alm.ListarAcciones(1, 100, 0)
	vistos := map[string]bool{}
	for _, a := range acs {
		if strings.HasPrefix(a.Accion, "filtro.") {
			vistos[a.Accion+"|"+a.Resultado] = true
			if strings.Contains(a.Detalle, "malo.com") {
				t.Fatalf("texto en el registro: %+v", a)
			}
		}
	}
	if !vistos["filtro.enlaces|ok"] {
		t.Fatalf("%v", vistos)
	}
}

func TestSinSerAdminSoloAlertaYNoIntentaBorrarNiExpulsar(t *testing.T) {
	e := nuevo(t, false)
	for _, acc := range []string{AccBorrar, AccExpulsar} {
		e.filtro(FiltroEnlaces, true, acc, nil)
		e.hora = e.hora.Add(11 * time.Minute) // fuera del anti-repetición de alertas
		e.s.Aplicar(ctx(), e.s.Evaluar(e.msg(pnLuis, "https://malo.com")))
	}
	if len(e.borr) != 0 || !e.cli.SinEscrituras() {
		t.Fatalf("no debía borrar ni expulsar: %v %v", e.borr, e.cli.Llamadas)
	}
	av := e.avisos()
	if len(av) != 2 {
		t.Fatalf("debe alertar cada vez: %+v", av)
	}
	for _, a := range av {
		if !strings.Contains(a.Detalle, "no soy administrador") || !strings.Contains(a.Titulo, "enlaces") || a.Persona != pnLuis || a.Chat != grupoJID {
			t.Fatalf("%+v", a)
		}
		if strings.Contains(a.Detalle, "malo.com") {
			t.Fatalf("la alerta no lleva el texto del mensaje: %+v", a)
		}
	}
	acs, _ := e.alm.ListarAcciones(1, 50, 0)
	n := 0
	for _, a := range acs {
		if a.Accion == "filtro.enlaces" && a.Resultado == "no_hecho" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("registro: %d", n)
	}
	// Avisar al grupo, advertir y silenciar no necesitan ser admin.
	e.filtro(FiltroEnlaces, true, AccAvisarGrupo, nil)
	e.hora = e.hora.Add(time.Minute)
	e.s.Aplicar(ctx(), e.s.Evaluar(e.msg(pnLuis, "https://malo.com")))
	if len(e.env) != 1 || !strings.Contains(e.env[0].texto, "no se permiten enlaces") {
		t.Fatalf("%+v", e.env)
	}
	// Sin conexión tampoco se rompe: solo alerta.
	e.cli.Conexion = false
	e.s.Invalidar(grupoJID)
	e.filtro(FiltroEnlaces, true, AccBorrar, nil)
	e.hora = e.hora.Add(11 * time.Minute)
	e.s.Aplicar(ctx(), e.s.Evaluar(e.msg(pnLuis, "https://malo.com")))
	if av := e.avisos(); len(av) != 3 || !strings.Contains(av[0].Detalle, "no pude saber") {
		t.Fatalf("%+v", av)
	}
}

func TestLasAlertasNoSeRepitenPorCadaMensaje(t *testing.T) {
	e := nuevo(t, true)
	e.filtro(FiltroEnlaces, true, AccAvisarme, nil)
	for k := 0; k < 5; k++ {
		e.s.Aplicar(ctx(), e.s.Evaluar(e.msg(pnLuis, "https://malo.com/"+string(rune('a'+k)))))
		e.hora = e.hora.Add(30 * time.Second)
	}
	if n := len(e.avisos()); n != 1 {
		t.Fatalf("una alerta por persona y filtro cada 10 minutos: %d", n)
	}
	e.hora = e.hora.Add(11 * time.Minute)
	e.s.Aplicar(ctx(), e.s.Evaluar(e.msg(pnLuis, "https://malo.com/z")))
	if n := len(e.avisos()); n != 2 {
		t.Fatalf("%d", n)
	}
}

func TestGuardarFiltroValidaYSePersiste(t *testing.T) {
	e := nuevo(t, true)
	fs := e.s.FiltrosDe(grupoJID)
	if len(fs) != 7 || fs[0].Filtro != FiltroEnlaces || fs[0].Activo || !fs[0].Config.ExentoAdmins || fs[3].Config.Max != 6 || fs[2].Config.Max != 3 {
		t.Fatalf("valores de fábrica: %+v", fs)
	}
	malos := []Filtro{
		{Chat: grupoJID, Filtro: "nada", Accion: AccBorrar},
		{Chat: grupoJID, Filtro: FiltroEnlaces, Accion: "volar"},
		{Chat: pnLuis, Filtro: FiltroEnlaces, Accion: AccBorrar},
		{Chat: grupoJID, Filtro: FiltroFlood, Accion: AccBorrar, Config: ConfigFiltro{Max: 1, Segundos: 10, SilencioMin: 5}},
		{Chat: grupoJID, Filtro: FiltroFlood, Accion: AccBorrar, Config: ConfigFiltro{Max: 5, Segundos: 0, SilencioMin: 5}},
		{Chat: grupoJID, Filtro: FiltroEnlaces, Accion: AccBorrar, Config: ConfigFiltro{SilencioMin: 0}},
	}
	for i, m := range malos {
		if err := e.s.GuardarFiltro("yo", m); err == nil {
			t.Fatalf("%d debía fallar", i)
		}
	}
	e.filtro(FiltroPalabras, true, AccAdvertir, func(c *ConfigFiltro) { c.Palabras = []string{"  uno ", "UNO", "", "dos"}; c.ExentoRangos = []int64{3} })
	// Otro servicio (por ejemplo, tras reiniciar la app) lee lo mismo.
	otro := Nuevo(e.alm, 1, nil, e.perm)
	var p Filtro
	for _, f := range otro.FiltrosDe(grupoJID) {
		if f.Filtro == FiltroPalabras {
			p = f
		}
	}
	if !p.Activo || p.Accion != AccAdvertir || len(p.Config.Palabras) != 2 || p.Config.ExentoRangos[0] != 3 || !p.Config.ExentoAdmins {
		t.Fatalf("%+v", p)
	}
	if err := otro.RestaurarFiltro("yo", grupoJID, FiltroPalabras); err != nil {
		t.Fatal(err)
	}
	for _, f := range otro.FiltrosDe(grupoJID) {
		if f.Filtro == FiltroPalabras && (f.Activo || len(f.Config.Palabras) != len(PalabrasDeFabrica)) {
			t.Fatalf("%+v", f)
		}
	}
	if err := otro.RestaurarFiltro("yo", grupoJID, "x"); !errors.Is(err, ErrFiltroNoHay) {
		t.Fatalf("%v", err)
	}
	if l, _ := e.alm.GruposConFiltros(1); len(l) != 0 {
		t.Fatalf("%v", l)
	}
}

func TestComandoFiltros(t *testing.T) {
	e := nuevo(t, true)
	e.rol("Mod", "grupos.filtros")
	e.rol("Raso")
	e.da(pnAna, "Mod")
	e.da(pnLuis, "Raso")
	if r := e.en(pnLuis, "!filtros"); !strings.Contains(r.Texto, "No tienes permiso") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.en(pnAna, "!filtros"); !strings.Contains(r.Texto, "Enlaces: apagado") || !strings.Contains(r.Texto, "Flood: apagado (6 mensajes en 10 s)") {
		t.Fatalf("%q", r.Texto)
	}
	pruebas := []struct{ texto, contiene string }{
		{"!filtros enlaces on", "quedó encendido"},
		{"!filtros enlaces permitir https://youtube.com/", "queda permitido"},
		{"!filtros enlaces accion borrar", "borrar el mensaje"},
		{"!filtros palabras agregar mala palabra", "agregué"},
		{"!filtros flood limite 8 20", "8 en 20"},
		{"!filtros flood on", "encendido"},
	}
	for _, p := range pruebas {
		if r := e.en(pnAna, p.texto); !strings.Contains(r.Texto, p.contiene) {
			t.Fatalf("%s: %q", p.texto, r.Texto)
		}
	}
	fs := map[string]Filtro{}
	for _, f := range e.s.FiltrosDe(grupoJID) {
		fs[f.Filtro] = f
	}
	if !fs["enlaces"].Activo || fs["enlaces"].Accion != AccBorrar || fs["enlaces"].Config.Permitidos[0] != "youtube.com" ||
		fs["palabras"].Config.Palabras[len(fs["palabras"].Config.Palabras)-1] != "mala palabra" || fs["flood"].Config.Max != 8 || fs["flood"].Config.Segundos != 20 {
		t.Fatalf("%+v", fs)
	}
	if r := e.en(pnAna, "!filtros enlaces quitar youtube.com"); !strings.Contains(r.Texto, "quité") || len(e.s.FiltrosDe(grupoJID)[0].Config.Permitidos) != 0 {
		t.Fatalf("%q", r.Texto)
	}
	for texto, quiere := range map[string]string{"!filtros cosa on": "no existe", "!filtros enlaces accion volar": "no existe", "!filtros enlaces limite 3 4": "solo repetidos y flood", "!filtros palabras permitir x": "solo el filtro de enlaces"} {
		if r := e.en(pnAna, texto); !strings.Contains(strings.ToLower(r.Texto), strings.ToLower(quiere)) {
			t.Fatalf("%s: %q", texto, r.Texto)
		}
	}
	// Siendo no admin, avisa que solo habrá notificación.
	e2 := nuevo(t, false)
	e2.rol("Mod", "grupos.filtros")
	e2.da(pnAna, "Mod")
	e2.en(pnAna, "!filtros enlaces on")
	if r := e2.en(pnAna, "!filtros enlaces accion borrar"); !strings.Contains(r.Texto, "no soy administrador") {
		t.Fatalf("%q", r.Texto)
	}
}

func (e *entorno) rompe(quien, texto string) *Infraccion {
	e.hora = e.hora.Add(time.Minute)
	inf := e.s.Evaluar(e.msg(quien, texto))
	e.s.Aplicar(ctx(), inf)
	return inf
}

func TestElContadorDeInfraccionesEsPorPersonaYPorFiltro(t *testing.T) {
	e := nuevo(t, true)
	e.filtro(FiltroEnlaces, true, AccAvisarme, nil)
	e.filtro(FiltroPalabras, true, AccAvisarme, func(c *ConfigFiltro) { c.Palabras = []string{"spamazo"} })
	e.rompe(pnLuis, "https://a.com")
	e.rompe(pnLuis, "https://b.com")
	e.rompe(pnLuis, "https://c.com")
	e.rompe(pnLuis, "spamazo total")
	e.rompe(pnRosa, "https://d.com")
	cuenta := func(p, f string) int { return e.alm.ContarInfraccionesFiltro(1, grupoJID, p, f, e.hora, 0) }
	if cuenta(pnLuis, FiltroEnlaces) != 3 || cuenta(pnLuis, FiltroPalabras) != 1 || cuenta(pnRosa, FiltroEnlaces) != 1 || cuenta(pnRosa, FiltroPalabras) != 0 {
		t.Fatalf("luis enlaces=%d palabras=%d, ana enlaces=%d", cuenta(pnLuis, FiltroEnlaces), cuenta(pnLuis, FiltroPalabras), cuenta(pnRosa, FiltroEnlaces))
	}
	top := e.alm.InfractoresDeFiltro(1, grupoJID, FiltroEnlaces, 10)
	if len(top) != 2 || top[0].Persona != pnLuis || top[0].Total != 3 || top[1].Total != 1 {
		t.Fatalf("top: %+v", top)
	}
	if per := e.alm.InfraccionesDePersona(1, grupoJID, pnLuis); len(per) != 2 || per[0].Filtro != FiltroEnlaces || per[0].Total != 3 {
		t.Fatalf("persona: %+v", per)
	}
	// reiniciar solo uno de los contadores
	if n, err := e.alm.BorrarInfraccionesFiltro(1, grupoJID, pnLuis, FiltroEnlaces); err != nil || n != 3 || cuenta(pnLuis, FiltroPalabras) != 1 {
		t.Fatalf("reiniciar: %d %v", n, err)
	}
	// el texto del mensaje nunca queda guardado
	acs, _ := e.alm.ListarAcciones(1, 100, 0)
	for _, a := range acs {
		if strings.Contains(a.Detalle, "a.com") || strings.Contains(a.Detalle, "spamazo") {
			t.Fatalf("texto en el registro: %+v", a)
		}
	}
}

func TestEscaladoPorContadorCambiaLaAccionDesdeLaInfraccionN(t *testing.T) {
	e := nuevo(t, true)
	e.filtro(FiltroEnlaces, true, AccAvisarme, func(c *ConfigFiltro) { c.EscalaDesde, c.EscalaAccion, c.EscalaDias = 3, AccBorrar, 1 })
	e.rompe(pnLuis, "https://a.com")
	e.rompe(pnLuis, "https://b.com")
	if len(e.borr) != 0 {
		t.Fatalf("antes de la tercera solo avisa: %v", e.borr)
	}
	if inf := e.rompe(pnLuis, "https://c.com"); inf == nil || inf.Numero != 3 || !inf.Escalada || len(e.borr) != 1 {
		t.Fatalf("la tercera borra: %+v %v", inf, e.borr)
	}
	// otra persona empieza de cero
	if inf := e.rompe(pnRosa, "https://d.com"); inf.Numero != 1 || inf.Escalada || len(e.borr) != 1 {
		t.Fatalf("Ana: %+v %v", inf, e.borr)
	}
	// la ventana de 1 día: dos días después Luis vuelve a empezar
	e.hora = e.hora.Add(48 * time.Hour)
	if inf := e.rompe(pnLuis, "https://e.com"); inf.Numero != 1 || inf.Escalada {
		t.Fatalf("fuera de la ventana: %+v", inf)
	}
}

func TestExentoPorPersonaNoCuentaNiActua(t *testing.T) {
	e := nuevo(t, true)
	e.filtro(FiltroEnlaces, true, AccBorrar, func(c *ConfigFiltro) { c.ExentoPersonas = []string{pnLuis} })
	e.rompe(pnLuis, "https://a.com")
	if len(e.borr) != 0 || e.alm.ContarInfraccionesFiltro(1, grupoJID, pnLuis, FiltroEnlaces, e.hora, 0) != 0 {
		t.Fatalf("la persona exenta no cuenta: %v", e.borr)
	}
	e.rompe(pnRosa, "https://b.com")
	if len(e.borr) != 1 {
		t.Fatalf("las demás sí: %v", e.borr)
	}
}

func TestGuardarFiltroValidaElEscalado(t *testing.T) {
	e := nuevo(t, true)
	for _, c := range []ConfigFiltro{{EscalaDesde: 1}, {EscalaDesde: 99}, {EscalaDesde: 3, EscalaAccion: "inventada"}, {EscalaDias: 400}} {
		x := filtroDefecto(grupoJID, FiltroEnlaces)
		cc := x.Config
		cc.EscalaDesde, cc.EscalaDias, cc.EscalaAccion = c.EscalaDesde, c.EscalaDias, c.EscalaAccion
		x.Config = cc
		if err := e.s.GuardarFiltro("yo", x); err == nil {
			t.Errorf("debía rechazar %+v", c)
		}
	}
}

func TestComandoInfracciones(t *testing.T) {
	e := nuevo(t, true)
	e.rol("Mod", "grupos.filtros")
	e.rol("Raso")
	e.da(pnAna, "Mod")
	e.da(pnLuis, "Raso")
	if r := e.en(pnLuis, "!infracciones"); !strings.Contains(r.Texto, "No tienes permiso") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.en(pnAna, "!infracciones"); !strings.Contains(r.Texto, "Nadie lleva infracciones") {
		t.Fatalf("%q", r.Texto)
	}
	e.filtro(FiltroEnlaces, true, AccAvisarme, nil)
	e.filtro(FiltroPalabras, true, AccAvisarme, func(c *ConfigFiltro) { c.Palabras = []string{"spamazo"} })
	e.rompe(pnLuis, "https://a.com")
	e.rompe(pnLuis, "https://b.com")
	e.rompe(pnLuis, "spamazo total")
	e.rompe(pnRosa, "https://c.com")
	r := e.en(pnAna, "!infracciones")
	if !strings.Contains(r.Texto, "Quién lleva más infracciones") || !strings.Contains(r.Texto, "1. ") || !strings.Contains(r.Texto, "*3*") {
		t.Fatalf("ranking: %q", r.Texto)
	}
}
