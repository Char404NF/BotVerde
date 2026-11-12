package silencio

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/grupos"
	"botchat/internal/permisos"
)

const (
	pnAna  = "5215511111111@s.whatsapp.net"
	pnLuis = "5215522222222@s.whatsapp.net"
	lidAna = "99887766554433@lid"
	g1     = "grupo1@g.us"
	g2     = "grupo2@g.us"
	g3     = "grupo3@g.us"
	propio = "5215500000001@s.whatsapp.net"
)

type env struct {
	alm   *almacen.Almacen
	perm  *permisos.Servicio
	s     *Servicio
	g     *grupos.Falso
	hora  time.Time
	mu    sync.Mutex
	salen []string // "chat|texto"
	nmsg  int
}

func nuevo(t *testing.T) *env {
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
	p, err := permisos.Nuevo(alm, 1)
	if err != nil {
		t.Fatal(err)
	}
	_ = alm.FijarZona("UTC")
	e := &env{alm: alm, perm: p, g: grupos.NuevoFalso()}
	// 12:00 UTC de un día fijo; todo el reloj sale de aquí
	e.hora = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	p.Ahora = func() time.Time { return e.hora }
	p.EsPropio = func(x string) bool { return x == propio }
	alm.Reloj = func() time.Time { return e.hora }
	e.s = Nuevo(alm, 1, p)
	e.s.Ahora = func() time.Time { return e.hora }
	e.s.Grupo = e.g
	e.s.Enviar = func(_ context.Context, chat, texto string) error {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.salen = append(e.salen, chat+"|"+texto)
		return nil
	}
	return e
}

func (e *env) enviados() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.salen...)
}

func (e *env) ajuste(k, v string) {
	if err := e.alm.FijarAjusteBot(1, k, v); err != nil {
		panic(err)
	}
}

// msg simula que `persona` escribió en `grupo` justo ahora.
func (e *env) msg(persona, grupo string) Mensaje {
	e.nmsg++
	return Mensaje{Chat: grupo, ID: fmt.Sprintf("M%d", e.nmsg), Persona: persona, TS: e.hora, EsGrupo: true}
}

func (e *env) silencia(persona string, o Opciones) *almacen.SilencioG {
	sil, err := e.s.Silenciar(almacen.PersonaDueno, persona, o)
	if err != nil {
		panic(err)
	}
	return sil
}

func TestAlcancesYVencimiento(t *testing.T) {
	e := nuevo(t)
	e.silencia(pnAna, Opciones{Alcance: almacen.SilUno, Grupos: []string{g1}, Duracion: time.Hour})
	if e.s.Vigente(pnAna, g1, e.hora) == nil {
		t.Fatal("en el grupo 1 sí")
	}
	if e.s.Vigente(pnAna, g2, e.hora) != nil {
		t.Fatal("en el grupo 2 no")
	}
	if e.s.Vigente(pnLuis, g1, e.hora) != nil {
		t.Fatal("Luis no")
	}
	// varios
	e.silencia(pnAna, Opciones{Alcance: almacen.SilVarios, Grupos: []string{g1, g2}, Duracion: 0})
	if e.s.Vigente(pnAna, g2, e.hora) == nil || e.s.Vigente(pnAna, g3, e.hora) != nil {
		t.Fatal("varios: g1 y g2, no g3")
	}
	// uno nuevo reemplaza al anterior: solo queda uno activo
	xs, _ := e.alm.SilenciosActivos(1, pnAna, e.hora)
	if len(xs) != 1 {
		t.Fatalf("debe quedar uno activo: %d", len(xs))
	}
	// todos
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos, Duracion: 30 * time.Minute})
	if e.s.Vigente(pnAna, g3, e.hora) == nil {
		t.Fatal("todos incluye g3")
	}
	e.hora = e.hora.Add(31 * time.Minute)
	if e.s.Vigente(pnAna, g3, e.hora) != nil {
		t.Fatal("a los 31 min venció")
	}
	// errores de alcance
	if _, err := e.s.Silenciar(almacen.PersonaDueno, pnAna, Opciones{Alcance: almacen.SilUno}); err == nil {
		t.Fatal("uno sin grupo")
	}
	if _, err := e.s.Silenciar(almacen.PersonaDueno, pnAna, Opciones{Grupos: []string{pnLuis}}); err == nil {
		t.Fatal("eso no es un grupo")
	}
	// al Dueño no
	if _, err := e.s.Silenciar(almacen.PersonaDueno, propio, Opciones{Alcance: almacen.SilTodos}); !errors.Is(err, permisos.ErrJerarquia) {
		t.Fatalf("al Dueño: %v", err)
	}
}

func TestBorraCuentaYAvisaCadaTres(t *testing.T) {
	e := nuevo(t)
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos, Avisar: false})
	for i := 1; i <= 9; i++ {
		r := e.s.Procesar(context.Background(), e.msg(pnAna, g1))
		if r.Hecho != "borrado" || r.N != i {
			t.Fatalf("mensaje %d: %+v", i, r)
		}
	}
	if e.g.NumBorrados() != 9 {
		t.Fatalf("borrados %d", e.g.NumBorrados())
	}
	if e.g.Borrados[0].Persona != pnAna || e.g.Borrados[0].Chat != g1 {
		t.Fatalf("%+v", e.g.Borrados[0])
	}
	av := e.enviados()
	if len(av) != 3 {
		t.Fatalf("3 avisos (3, 6 y 9): %v", av)
	}
	if !strings.Contains(av[0], "Hoy van 3 de 10") || !strings.HasPrefix(av[0], pnAna+"|") {
		t.Fatalf("%q", av[0])
	}
	// el décimo llega al tope: sanción por defecto = avisarme y confirmar (nada se expulsa solo)
	r := e.s.Procesar(context.Background(), e.msg(pnAna, g1))
	if r.N != 10 {
		t.Fatalf("%+v", r)
	}
	if e.g.NumExpulsados() != 0 {
		t.Fatal("la expulsión NUNCA es automática por defecto")
	}
	ps, _ := e.alm.ListarSancionesPendientes(1)
	if len(ps) != 1 || ps[0].Persona != pnAna || ps[0].Grupo != g1 || ps[0].Accion != "expulsar" {
		t.Fatalf("%+v", ps)
	}
	// el 11 no repite la sanción ni manda más avisos ese día
	e.s.Procesar(context.Background(), e.msg(pnAna, g1))
	if ps, _ = e.alm.ListarSancionesPendientes(1); len(ps) != 1 {
		t.Fatalf("sin repetir: %+v", ps)
	}
	if len(e.enviados()) != 3 {
		t.Fatalf("sin más avisos: %v", e.enviados())
	}
	c := e.s.ContadorDe(pnAna, "", e.hora)
	if c.Usado != 11 || c.Tope != 10 || c.Color != "rojo" || !c.SancionAplicada {
		t.Fatalf("%+v", c)
	}
}

func TestSinAdminNoBorraNiCuentaYAvisaUnaVezPorDiaYGrupo(t *testing.T) {
	e := nuevo(t)
	e.g.PonerAdmin(g1, false)
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos})
	for i := 0; i < 4; i++ {
		r := e.s.Procesar(context.Background(), e.msg(pnAna, g1))
		if r.Hecho != "no_admin" {
			t.Fatalf("%+v", r)
		}
	}
	if e.g.NumBorrados() != 0 {
		t.Fatal("sin admin NO borra")
	}
	if c := e.s.ContadorDe(pnAna, "", e.hora); c.Usado != 0 {
		t.Fatalf("no cuenta: %+v", c)
	}
	// un aviso para mí (No soy admin aquí), uno solo
	n := 0
	var det string
	for id := int64(1); id < 20; id++ {
		if a, _ := e.alm.ObtenerAviso(1, id); a != nil && a.Tipo == "silencio_noadmin" {
			n++
			det = a.Titulo + "|" + a.Detalle
		}
	}
	if n != 1 || !strings.Contains(det, "No soy admin aquí") {
		t.Fatalf("avisos para mí: %d %q", n, det)
	}
	// a la persona: una vez, con el texto sin amenazas
	av := e.enviados()
	if len(av) != 1 || !strings.HasSuffix(av[0], "|Estás silenciado; tus mensajes no se leerán") || strings.Contains(strings.ToLower(av[0]), "expuls") {
		t.Fatalf("%v", av)
	}
	// otro grupo sin admin: otro aviso para mí
	e.g.PonerAdmin(g2, false)
	e.s.Procesar(context.Background(), e.msg(pnAna, g2))
	n = 0
	for id := int64(1); id < 30; id++ {
		if a, _ := e.alm.ObtenerAviso(1, id); a != nil && a.Tipo == "silencio_noadmin" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("un aviso por grupo: %d", n)
	}
	// al día siguiente (día lógico nuevo) vuelve a avisar
	e.hora = e.hora.Add(24 * time.Hour)
	e.s.Procesar(context.Background(), e.msg(pnAna, g1))
	n = 0
	for id := int64(1); id < 40; id++ {
		if a, _ := e.alm.ObtenerAviso(1, id); a != nil && a.Tipo == "silencio_noadmin" {
			n++
		}
	}
	if n != 3 {
		t.Fatalf("otro día, otro aviso: %d", n)
	}
	// si se vuelve admin, ya borra
	e.g.PonerAdmin(g1, true)
	if r := e.s.Procesar(context.Background(), e.msg(pnAna, g1)); r.Hecho != "borrado" {
		t.Fatalf("%+v", r)
	}
}

func TestSiElBorradoFallaNoCuenta(t *testing.T) {
	e := nuevo(t)
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos})
	e.g.ErrBorrar = errors.New("el servidor dijo que no")
	for i := 0; i < 5; i++ {
		if r := e.s.Procesar(context.Background(), e.msg(pnAna, g1)); r.Hecho != "error" {
			t.Fatalf("%+v", r)
		}
	}
	if c := e.s.ContadorDe(pnAna, "", e.hora); c.Usado != 0 {
		t.Fatalf("un intento fallido no cuenta: %+v", c)
	}
	// error al saber si soy admin: tampoco cuenta
	e.g.ErrBorrar = nil
	e.g.ErrAdmin = errors.New("sin conexión")
	if r := e.s.Procesar(context.Background(), e.msg(pnAna, g1)); r.Hecho != "error" {
		t.Fatalf("%+v", r)
	}
	if c := e.s.ContadorDe(pnAna, "", e.hora); c.Usado != 0 {
		t.Fatal("no cuenta")
	}
}

func TestIgnoraSincronizacionViejosPrivadosYMios(t *testing.T) {
	e := nuevo(t)
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos})
	m := e.msg(pnAna, g1)
	m.Sinc = true
	if e.s.AlMensaje(m) {
		t.Fatal("sincronización offline: no se borra")
	}
	m = e.msg(pnAna, g1)
	m.TS = e.hora.Add(-11 * time.Minute)
	if e.s.AlMensaje(m) {
		t.Fatal("más de 10 minutos: no se borra")
	}
	m = e.msg(pnAna, g1)
	m.EsGrupo = false
	if e.s.AlMensaje(m) {
		t.Fatal("privado: no")
	}
	m = e.msg(pnAna, g1)
	m.EsMio = true
	if e.s.AlMensaje(m) {
		t.Fatal("mío: no")
	}
	if e.s.AlMensaje(e.msg(pnLuis, g1)) {
		t.Fatal("quien no está silenciado: no")
	}
	if e.g.NumBorrados() != 0 {
		t.Fatal("nada borrado")
	}
	// uno bueno (sin trabajador iniciado procesa en el momento)
	if !e.s.AlMensaje(e.msg(pnAna, g1)) || e.g.NumBorrados() != 1 {
		t.Fatal("este sí")
	}
	// el silencio apagado no borra
	e.ajuste("sil_activo", "0")
	if r := e.s.Procesar(context.Background(), e.msg(pnAna, g1)); r.Hecho != "omitido" {
		t.Fatalf("%+v", r)
	}
}

func TestAliasLIDyNumeroSonLaMismaPersona(t *testing.T) {
	e := nuevo(t)
	e.alm.AprenderAlias(1, pnAna, lidAna)
	e.silencia(lidAna, Opciones{Alcance: almacen.SilTodos})
	if !e.s.AlMensaje(e.msg(pnAna, g1)) {
		t.Fatal("silenciada por LID, escribe con número")
	}
	if !e.s.AlMensaje(e.msg(lidAna, g1)) {
		t.Fatal("y con LID")
	}
	if c := e.s.ContadorDe(lidAna, "", e.hora); c.Usado != 2 {
		t.Fatalf("%+v", c)
	}
}

func TestTopesDeAvisosPropios(t *testing.T) {
	e := nuevo(t)
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos})
	e.silencia(pnLuis, Opciones{Alcance: almacen.SilTodos})
	// por persona y día: 4
	for i := 0; i < 6; i++ {
		e.s.AvisoPrivado(pnAna, "hola")
	}
	if n := len(e.enviados()); n != 4 {
		t.Fatalf("4 por persona y día: %d", n)
	}
	if ok, why := e.s.AvisoPrivado(pnAna, "otro"); ok || why != "tope por persona" {
		t.Fatalf("%v %q", ok, why)
	}
	// otro día: otros 4
	e.hora = e.hora.Add(24 * time.Hour)
	if ok, _ := e.s.AvisoPrivado(pnAna, "otro día"); !ok {
		t.Fatal("otro día vuelve")
	}
	// por hora: 12 (Ana ya lleva 1 hoy; Luis y otras personas)
	e2 := nuevo(t)
	e2.ajuste("sil_avisos_persona_dia", "100")
	enviados := 0
	for i := 0; i < 20; i++ {
		if ok, _ := e2.s.AvisoPrivado(fmt.Sprintf("52155000000%02d@s.whatsapp.net", i), "x"); ok {
			enviados++
		}
	}
	if enviados != 12 {
		t.Fatalf("12 por hora: %d", enviados)
	}
	e2.hora = e2.hora.Add(61 * time.Minute)
	if ok, _ := e2.s.AvisoPrivado("5215500009999@s.whatsapp.net", "x"); !ok {
		t.Fatal("pasada la hora vuelve")
	}
	// por día: 60 (subiendo el de la hora)
	e3 := nuevo(t)
	e3.ajuste("sil_avisos_persona_dia", "100")
	e3.ajuste("sil_avisos_hora", "1000")
	n := 0
	for i := 0; i < 80; i++ {
		if ok, _ := e3.s.AvisoPrivado(fmt.Sprintf("5215500%05d@s.whatsapp.net", i), "x"); ok {
			n++
		}
	}
	if n != 60 {
		t.Fatalf("60 por día: %d", n)
	}
}

func TestNoMolestarYBloqueadaNoRecibenAvisos(t *testing.T) {
	e := nuevo(t)
	if _, _, err := e.alm.AgregarNoMolestar(1, pnAna, "pidió no recibir", e.hora); err != nil {
		t.Fatal(err)
	}
	if ok, why := e.s.AvisoPrivado(pnAna, "x"); ok || why != "no molestar" {
		t.Fatalf("%v %q", ok, why)
	}
	if err := e.alm.BloquearPersona(1, pnLuis, e.hora); err != nil {
		t.Fatal(err)
	}
	if ok, why := e.s.AvisoPrivado(pnLuis, "x"); ok || why != "bloqueada" {
		t.Fatalf("%v %q", ok, why)
	}
	if len(e.enviados()) != 0 {
		t.Fatal("nada salió")
	}
	// y los topes del silencio no gastan los de iniciativa (viven en otra cuenta: ini_tope_* no se tocan)
	if e.alm.AjusteBotInt(1, "sil_avisos_hora") != 12 || e.alm.AjusteBotInt(1, "sil_avisos_dia") != 60 || e.alm.AjusteBotInt(1, "sil_avisos_persona_dia") != 4 || e.alm.AjusteBotInt(1, "sil_borrados_min") != 20 {
		t.Fatal("topes de fábrica distintos de lo acordado")
	}
}

func TestTopeDeBorradosPorMinuto(t *testing.T) {
	e := nuevo(t)
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos})
	e.ajuste("sil_tope", "0")
	e.ajuste("sil_aviso_cada", "0")
	hechos, topados := 0, 0
	for i := 0; i < 25; i++ {
		switch e.s.Procesar(context.Background(), e.msg(pnAna, g1)).Hecho {
		case "borrado":
			hechos++
		case "tope_borrados":
			topados++
		}
	}
	if hechos != 20 || topados != 5 {
		t.Fatalf("20 por minuto: %d %d", hechos, topados)
	}
	if c := e.s.ContadorDe(pnAna, "", e.hora); c.Usado != 20 {
		t.Fatalf("lo no borrado no cuenta: %+v", c)
	}
	e.hora = e.hora.Add(61 * time.Second)
	if r := e.s.Procesar(context.Background(), e.msg(pnAna, g1)); r.Hecho != "borrado" {
		t.Fatalf("%+v", r)
	}
}

func TestDiaLogicoConHoraDeReinicio(t *testing.T) {
	e := nuevo(t)
	e.ajuste("sil_reinicio_hora", "04:00")
	e.hora = time.Date(2026, 11, 10, 3, 59, 0, 0, time.UTC)
	if d := e.s.DiaLogico(e.hora); d != "2026-11-09" {
		t.Fatalf("a las 03:59 todavía es el día anterior: %s", d)
	}
	e.hora = time.Date(2026, 11, 10, 4, 0, 0, 0, time.UTC)
	if d := e.s.DiaLogico(e.hora); d != "2026-11-10" {
		t.Fatalf("%s", d)
	}
	if r := e.s.ProximoReinicio(e.hora); !r.Equal(time.Date(2026, 11, 11, 4, 0, 0, 0, time.UTC)) {
		t.Fatalf("%v", r)
	}
	// el contador se reinicia al cambiar el día lógico
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos})
	e.ajuste("sil_aviso_cada", "0")
	for i := 0; i < 3; i++ {
		e.s.Procesar(context.Background(), e.msg(pnAna, g1))
	}
	e.hora = time.Date(2026, 11, 11, 3, 0, 0, 0, time.UTC)
	e.s.Procesar(context.Background(), e.msg(pnAna, g1))
	if c := e.s.ContadorDe(pnAna, "", e.hora); c.Usado != 4 {
		t.Fatalf("a las 03:00 del día siguiente sigue el mismo día lógico: %+v", c)
	}
	e.hora = time.Date(2026, 11, 11, 4, 5, 0, 0, time.UTC)
	if c := e.s.ContadorDe(pnAna, "", e.hora); c.Usado != 0 || c.Color != "verde" {
		t.Fatalf("pasadas las 04:00 vuelve a cero: %+v", c)
	}
	// hora inválida = medianoche
	e.ajuste("sil_reinicio_hora", "basura")
	if h, m := e.s.HoraReinicio(); h != 0 || m != 0 {
		t.Fatal("basura")
	}
}

func TestPonerEnCero(t *testing.T) {
	e := nuevo(t)
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos})
	e.ajuste("sil_aviso_cada", "0")
	for i := 0; i < 5; i++ {
		e.s.Procesar(context.Background(), e.msg(pnAna, g1))
	}
	if err := e.s.PonerEnCero(almacen.PersonaDueno, pnAna, ""); err != nil {
		t.Fatal(err)
	}
	if c := e.s.ContadorDe(pnAna, "", e.hora); c.Usado != 0 {
		t.Fatalf("%+v", c)
	}
	// después de llegar al tope y ponerlo en cero, la sanción puede volver a ocurrir
	for i := 0; i < 10; i++ {
		e.s.Procesar(context.Background(), e.msg(pnAna, g1))
	}
	ps, _ := e.alm.ListarSancionesPendientes(1)
	if len(ps) != 1 {
		t.Fatalf("%+v", ps)
	}
	_ = e.s.PonerEnCero(almacen.PersonaDueno, pnAna, "")
	if c := e.s.ContadorDe(pnAna, "", e.hora); c.SancionAplicada {
		t.Fatal("la sanción se reinicia con el contador")
	}
}

func TestConteoPorGrupo(t *testing.T) {
	e := nuevo(t)
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos})
	e.ajuste("sil_aviso_cada", "0")
	e.ajuste("sil_tope", "4")
	for i := 0; i < 3; i++ {
		e.s.Procesar(context.Background(), e.msg(pnAna, g1))
		e.s.Procesar(context.Background(), e.msg(pnAna, g2))
	}
	// sumando los grupos (6) ya pasó el tope de 4
	if ps, _ := e.alm.ListarSancionesPendientes(1); len(ps) != 1 {
		t.Fatalf("suma de grupos: %+v", ps)
	}
	e2 := nuevo(t)
	e2.silencia(pnAna, Opciones{Alcance: almacen.SilTodos})
	e2.ajuste("sil_aviso_cada", "0")
	e2.ajuste("sil_tope", "4")
	e2.ajuste("sil_conteo_por_grupo", "1")
	for i := 0; i < 3; i++ {
		e2.s.Procesar(context.Background(), e2.msg(pnAna, g1))
		e2.s.Procesar(context.Background(), e2.msg(pnAna, g2))
	}
	if ps, _ := e2.alm.ListarSancionesPendientes(1); len(ps) != 0 {
		t.Fatalf("por grupo cada uno lleva 3: %+v", ps)
	}
	e2.s.Procesar(context.Background(), e2.msg(pnAna, g1))
	ps, _ := e2.alm.ListarSancionesPendientes(1)
	if len(ps) != 1 || ps[0].Grupo != g1 {
		t.Fatalf("%+v", ps)
	}
	if c := e2.s.ContadorDe(pnAna, g2, e2.hora); c.Usado != 3 || c.Total != 7 {
		t.Fatalf("%+v", c)
	}
}

func TestSancionesPorModo(t *testing.T) {
	// silencio_largo
	e := nuevo(t)
	e.ajuste("sil_aviso_cada", "0")
	e.ajuste("sil_tope", "3")
	e.ajuste("sil_sancion", SancSilencioLargo)
	sil := e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos, Duracion: time.Hour})
	for i := 0; i < 3; i++ {
		e.s.Procesar(context.Background(), e.msg(pnAna, g1))
	}
	x, _ := e.alm.SilencioPorID(1, sil.ID)
	if x.HastaTS != e.hora.Add(10080*time.Minute).Unix() {
		t.Fatalf("silencio alargado a una semana: %v", time.Unix(x.HastaTS, 0))
	}
	if av := e.enviados(); len(av) != 1 || !strings.Contains(av[0], "se alargó") {
		t.Fatalf("%v", av)
	}
	if e.g.NumExpulsados() != 0 {
		t.Fatal("no expulsa")
	}
	// avisar: solo una notificación
	e = nuevo(t)
	e.ajuste("sil_aviso_cada", "0")
	e.ajuste("sil_tope", "2")
	e.ajuste("sil_sancion", SancAvisar)
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos})
	for i := 0; i < 2; i++ {
		e.s.Procesar(context.Background(), e.msg(pnAna, g1))
	}
	if ps, _ := e.alm.ListarSancionesPendientes(1); len(ps) != 0 {
		t.Fatal("avisar no deja pendiente")
	}
	a, _ := e.alm.ObtenerAviso(1, 1)
	if a == nil || a.Tipo != "silencio_tope" {
		t.Fatalf("%+v", a)
	}
	// expulsar sin permiso de expulsión automática = pide confirmar
	e = nuevo(t)
	e.ajuste("sil_aviso_cada", "0")
	e.ajuste("sil_tope", "2")
	e.ajuste("sil_sancion", SancExpulsar)
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos})
	for i := 0; i < 2; i++ {
		e.s.Procesar(context.Background(), e.msg(pnAna, g1))
	}
	if e.g.NumExpulsados() != 0 {
		t.Fatal("expulsar necesita sil_expulsion_auto=1")
	}
	if ps, _ := e.alm.ListarSancionesPendientes(1); len(ps) != 1 {
		t.Fatal("queda por confirmar")
	}
	// con la expulsión automática encendida a mano, expulsa
	e = nuevo(t)
	e.ajuste("sil_aviso_cada", "0")
	e.ajuste("sil_tope", "2")
	e.ajuste("sil_sancion", SancExpulsar)
	e.ajuste("sil_expulsion_auto", "1")
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos})
	for i := 0; i < 2; i++ {
		e.s.Procesar(context.Background(), e.msg(pnAna, g1))
	}
	if e.g.NumExpulsados() != 1 || e.g.Expulsados[0].Persona != pnAna || e.g.Expulsados[0].Chat != g1 {
		t.Fatalf("%+v", e.g.Expulsados)
	}
	// sin tope (0) nunca sanciona
	e = nuevo(t)
	e.ajuste("sil_aviso_cada", "0")
	e.ajuste("sil_tope", "0")
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos})
	for i := 0; i < 15; i++ {
		e.s.Procesar(context.Background(), e.msg(pnAna, g1))
	}
	if ps, _ := e.alm.ListarSancionesPendientes(1); len(ps) != 0 {
		t.Fatal("sin tope")
	}
}

func TestConfirmarSancion(t *testing.T) {
	llegar := func() *env {
		e := nuevo(t)
		e.ajuste("sil_aviso_cada", "0")
		e.ajuste("sil_tope", "2")
		e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos})
		for i := 0; i < 2; i++ {
			e.s.Procesar(context.Background(), e.msg(pnAna, g1))
		}
		return e
	}
	ctx := context.Background()
	// sí
	e := llegar()
	ps, _ := e.s.SancionesPendientes()
	if ps[0].AvisoID == 0 {
		t.Fatal("ligada al aviso")
	}
	txt, err := e.s.ConfirmarSancion(ctx, ps[0].ID, true)
	if err != nil || !strings.Contains(txt, "expulsé") || e.g.NumExpulsados() != 1 {
		t.Fatalf("%q %v", txt, err)
	}
	if _, err := e.s.ConfirmarSancion(ctx, ps[0].ID, true); !errors.Is(err, ErrNoSancion) {
		t.Fatalf("ya resuelta: %v", err)
	}
	// no
	e = llegar()
	ps, _ = e.s.SancionesPendientes()
	if _, err := e.s.ConfirmarSancion(ctx, ps[0].ID, false); err != nil || e.g.NumExpulsados() != 0 {
		t.Fatal("no expulsa")
	}
	if p, _ := e.alm.ObtenerSancion(1, ps[0].ID); p.Estado != almacen.SancionRechazada {
		t.Fatalf("%+v", p)
	}
	// sin ser admin ya
	e = llegar()
	ps, _ = e.s.SancionesPendientes()
	e.g.PonerAdmin(g1, false)
	if _, err := e.s.ConfirmarSancion(ctx, ps[0].ID, true); err == nil || !strings.Contains(err.Error(), "no soy administrador") {
		t.Fatalf("%v", err)
	}
	if p, _ := e.alm.ObtenerSancion(1, ps[0].ID); p.Estado != almacen.SancionFallida {
		t.Fatalf("%+v", p)
	}
	// WhatsApp lo rechaza
	e = llegar()
	ps, _ = e.s.SancionesPendientes()
	e.g.ErrExpulsar = errors.New("403")
	if _, err := e.s.ConfirmarSancion(ctx, ps[0].ID, true); err == nil || !strings.Contains(err.Error(), "WhatsApp no lo permitió") {
		t.Fatalf("%v", err)
	}
}

func TestVencimientoAvisaALaPersonaYAMi(t *testing.T) {
	e := nuevo(t)
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos, Duracion: time.Hour, Avisar: true})
	e.silencia(pnLuis, Opciones{Alcance: almacen.SilTodos, Duracion: 0, Avisar: true})
	antes := len(e.enviados()) // los avisos de inicio
	if antes != 2 {
		t.Fatalf("avisos de inicio: %v", e.enviados())
	}
	if sig := e.s.Siguiente(); sig != e.hora.Add(time.Hour).Unix() {
		t.Fatalf("siguiente %d", sig)
	}
	if ids := e.s.Vencer(); len(ids) != 0 {
		t.Fatal("todavía no")
	}
	e.hora = e.hora.Add(61 * time.Minute)
	ids := e.s.Vencer()
	if len(ids) != 1 {
		t.Fatalf("un aviso para mí: %v", ids)
	}
	a, _ := e.alm.ObtenerAviso(1, ids[0])
	if a.Tipo != "silencio_fin" || !strings.Contains(a.Detalle, "terminó") {
		t.Fatalf("%+v", a)
	}
	av := e.enviados()
	if len(av) != antes+1 || !strings.HasSuffix(av[len(av)-1], "Tu silencio terminó. Ya puedes escribir de nuevo.") || !strings.HasPrefix(av[len(av)-1], pnAna+"|") {
		t.Fatalf("aviso a la persona: %v", av)
	}
	// seguro llamarlo de más
	if ids := e.s.Vencer(); len(ids) != 0 {
		t.Fatal("no se repite")
	}
	if e.s.Vigente(pnLuis, g1, e.hora) == nil {
		t.Fatal("el de Luis es para siempre")
	}
	// quitar a mano NO avisa de vencimiento
	if ok, err := e.s.QuitarSilencio(almacen.PersonaDueno, pnLuis); !ok || err != nil {
		t.Fatalf("%v %v", ok, err)
	}
	n := len(e.enviados())
	e.hora = e.hora.Add(100 * time.Hour)
	e.s.Vencer()
	if len(e.enviados()) != n {
		t.Fatal("quitar no manda aviso de vencimiento")
	}
	if ok, _ := e.s.QuitarSilencio(almacen.PersonaDueno, pnLuis); ok {
		t.Fatal("ya no tenía")
	}
	// sin avisar: no se le escribe
	e2 := nuevo(t)
	e2.silencia(pnAna, Opciones{Alcance: almacen.SilTodos, Duracion: time.Minute, Avisar: false})
	e2.hora = e2.hora.Add(2 * time.Minute)
	e2.s.Vencer()
	if len(e2.enviados()) != 0 {
		t.Fatalf("avisar=false: %v", e2.enviados())
	}
}

func TestTextosEditables(t *testing.T) {
	e := nuevo(t)
	if err := e.s.FijarTexto(TxtNoAdmin, "Silenciado en {grupo}"); err != nil {
		t.Fatal(err)
	}
	e.g.PonerAdmin(g1, false)
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos})
	e.s.Procesar(context.Background(), e.msg(pnAna, g1))
	if av := e.enviados(); len(av) != 1 || !strings.HasSuffix(av[0], "|Silenciado en el grupo") {
		t.Fatalf("%v", av)
	}
	// texto propio del silencio para el aviso del contador
	e2 := nuevo(t)
	e2.silencia(pnAna, Opciones{Alcance: almacen.SilTodos, Texto: "Basta, llevas {n}"})
	for i := 0; i < 3; i++ {
		e2.s.Procesar(context.Background(), e2.msg(pnAna, g1))
	}
	if av := e2.enviados(); len(av) != 1 || !strings.HasSuffix(av[0], "|Basta, llevas 3") {
		t.Fatalf("%v", av)
	}
	// volver al de fábrica
	if err := e.s.FijarTexto(TxtNoAdmin, ""); err != nil || e.s.Texto(TxtNoAdmin) != TextosDeFabrica[TxtNoAdmin] {
		t.Fatal("de fábrica")
	}
	if err := e.s.FijarTexto("inventado", "x"); err == nil {
		t.Fatal("texto desconocido")
	}
	// el texto no queda en claro en el disco: se guarda cifrado
	if t1, ok := e.alm.TextoSilencio(1, TxtNoAdmin); ok {
		t.Fatalf("debía borrarse: %q", t1)
	}
}

func TestBanderasDeMiembros(t *testing.T) {
	e := nuevo(t)
	if _, err := e.alm.ActualizarMiembros(1, g1, []almacen.MiembroGrupo{{Persona: pnAna, JID: pnAna}, {Persona: pnLuis, JID: pnLuis}}, e.hora); err != nil {
		t.Fatal(err)
	}
	if _, err := e.alm.ActualizarMiembros(1, g2, []almacen.MiembroGrupo{{Persona: pnAna, JID: pnAna}}, e.hora); err != nil {
		t.Fatal(err)
	}
	flag := func(g, p string) (bool, bool) {
		ms, _ := e.alm.MiembrosDe(1, g, false, e.hora)
		for _, m := range ms {
			if m.Persona == p {
				return m.Silenciado, m.Advertido
			}
		}
		t.Fatalf("no está %s en %s", p, g)
		return false, false
	}
	e.silencia(pnAna, Opciones{Alcance: almacen.SilUno, Grupos: []string{g1}, Duracion: time.Hour})
	if s, _ := flag(g1, pnAna); !s {
		t.Fatal("silenciada en g1")
	}
	if s, _ := flag(g2, pnAna); s {
		t.Fatal("no en g2")
	}
	if s, _ := flag(g1, pnLuis); s {
		t.Fatal("Luis no")
	}
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos, Duracion: time.Hour})
	if s, _ := flag(g2, pnAna); !s {
		t.Fatal("ahora en todos")
	}
	e.hora = e.hora.Add(2 * time.Hour)
	e.s.Vencer()
	if s, _ := flag(g1, pnAna); s {
		t.Fatal("al vencer se quita la marca")
	}
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos})
	_, _ = e.s.QuitarSilencio(almacen.PersonaDueno, pnAna)
	if s, _ := flag(g1, pnAna); s {
		t.Fatal("al quitar se quita la marca")
	}
	// advertido sigue la vigencia de las advertencias
	if _, _, err := e.alm.AgregarAdvertencia(1, pnLuis, g1, "x", "yo", e.hora); err != nil {
		t.Fatal(err)
	}
	if _, a := flag(g1, pnLuis); !a {
		t.Fatal("advertido")
	}
	e.hora = e.hora.Add(31 * 24 * time.Hour)
	e.s.Vencer()
	if _, a := flag(g1, pnLuis); a {
		t.Fatal("la advertencia caducó a los 30 días: ya no está advertido")
	}
}

func TestColaDeFondoYDetener(t *testing.T) {
	e := nuevo(t)
	e.silencia(pnAna, Opciones{Alcance: almacen.SilTodos})
	e.ajuste("sil_aviso_cada", "1")
	e.s.Iniciar(context.Background())
	defer e.s.Detener()
	for i := 0; i < 5; i++ {
		if !e.s.AlMensaje(e.msg(pnAna, g1)) {
			t.Fatal("en cola")
		}
	}
	dentro := func(f func() bool) {
		for i := 0; i < 200; i++ {
			if f() {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("no se cumplió a tiempo")
	}
	dentro(func() bool { return e.g.NumBorrados() == 5 })
	dentro(func() bool { return len(e.enviados()) == 4 }) // 5 avisos pero el tope por persona y día es 4
}

func TestComandos(t *testing.T) {
	e := nuevo(t)
	d := comandos.Nuevo(e.alm, 1, e.perm)
	d.Ahora = func() time.Time { return e.hora }
	e.s.RegistrarComandos(d)
	// el rango «Jefe» tiene las casillas nuevas; Raso no
	id, err := e.perm.GuardarRol(almacen.Rol{Nombre: "Jefe", Casillas: map[string]almacen.Alcance{"silencio.silenciar": {}, "silencio.ver": {}}, LimiteMin: 100})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.alm.AsignarRol(1, pnLuis, id, 0, "yo"); err != nil {
		t.Fatal(err)
	}
	enGrupo := func(quien, texto string) string {
		return d.Ejecutar(comandos.Entrada{Persona: quien, Chat: g1, EsGrupo: true, Texto: texto, MensajeID: "M"}).Texto
	}
	if r := enGrupo(pnAna, "!silenciar 5215511111111"); !strings.Contains(r, "No tienes permiso") {
		t.Fatalf("sin casilla: %q", r)
	}
	if r := enGrupo(pnLuis, "!silenciar 5215511111111 2h"); !strings.Contains(r, "silenciada aquí por 2 horas") {
		t.Fatalf("%q", r)
	}
	if e.s.Vigente(pnAna, g1, e.hora) == nil || e.s.Vigente(pnAna, g2, e.hora) != nil {
		t.Fatal("en grupo: solo ese grupo")
	}
	if r := enGrupo(pnLuis, "!silenciar 5215511111111 siempre todos"); !strings.Contains(r, "todos los grupos") {
		t.Fatalf("%q", r)
	}
	if e.s.Vigente(pnAna, g2, e.hora) == nil {
		t.Fatal("todos")
	}
	if r := enGrupo(pnLuis, "!silenciados"); !strings.Contains(r, "+5215511111111") && !strings.Contains(r, "Silenciados en grupos") {
		t.Fatalf("%q", r)
	}
	if r := enGrupo(pnLuis, "!contador 5215511111111"); !strings.Contains(r, "0 de 10") {
		t.Fatalf("%q", r)
	}
	if r := enGrupo(pnLuis, "!contador 5215511111111 cero"); !strings.Contains(r, "en cero") {
		t.Fatalf("%q", r)
	}
	if r := enGrupo(pnLuis, "!silenciar 5215511111111 mañana"); !strings.Contains(r, "30m, 2h, 1d o siempre") {
		t.Fatalf("%q", r)
	}
	if r := enGrupo(pnLuis, "!desilenciar 5215511111111"); !strings.Contains(r, "ya puede escribir") {
		t.Fatalf("%q", r)
	}
	if r := enGrupo(pnLuis, "!desilenciar 5215511111111"); !strings.Contains(r, "no estaba silenciada en grupos") || !strings.Contains(r, "designorar") {
		t.Fatalf("%q", r)
	}
	// al Dueño no
	if r := enGrupo(pnLuis, "!silenciar "+strings.TrimSuffix(propio, "@s.whatsapp.net")); r == "" {
		t.Fatal("algo debe contestar")
	}
	// registro sin textos
	acs, _ := e.alm.ListarAcciones(1, 100, 0)
	vistos := map[string]bool{}
	for _, a := range acs {
		vistos[a.Accion+"|"+a.Resultado] = true
	}
	for _, k := range []string{"silencio.silenciar|ok", "silencio.quitar|ok", "silencio.contador_cero|ok"} {
		if !vistos[k] {
			t.Fatalf("falta %s en %v", k, vistos)
		}
	}
}

func TestValidarAjustes(t *testing.T) {
	casos := []struct {
		k, v string
		ok   bool
	}{
		{"sil_tope", "10", true}, {"sil_tope", "-1", false}, {"sil_tope", "abc", false},
		{"sil_sancion", "confirmar", true}, {"sil_sancion", "matar", false},
		{"sil_reinicio_hora", "04:30", true}, {"sil_reinicio_hora", "4:30", false}, {"sil_reinicio_hora", "25:00", false},
		{"sil_expulsion_auto", "2", false}, {"sil_avisos_persona_dia", "4", true},
		{"adv_caducidad_dias", "30", true}, {"adv_caducidad_dias", "-5", false},
	}
	for _, c := range casos {
		conocido, err := ValidarAjuste(c.k, c.v)
		if !conocido || (err == nil) != c.ok {
			t.Errorf("%s=%s: conocido=%v err=%v", c.k, c.v, conocido, err)
		}
	}
	if c, _ := ValidarAjuste("otra_cosa", "1"); c {
		t.Fatal("clave ajena")
	}
}
