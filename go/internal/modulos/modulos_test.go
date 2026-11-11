package modulos

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/envio"
	"botchat/internal/permisos"
)

const (
	pnAna  = "5215511111111@s.whatsapp.net"
	lidAna = "99887766554433@lid"
	pnLuis = "5215522222222@s.whatsapp.net"
	grupo  = "120363000000000001@g.us"
)

type falsoEnv struct {
	mu        sync.Mutex
	enviados  []string // "chat|texto"
	citas     []string // id citado de cada texto enviado
	menc      [][]string
	conectado bool
	falla     error
}

func (f *falsoEnv) Texto(_ context.Context, chat, texto, cita string, menc []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.falla != nil {
		return f.falla
	}
	f.citas = append(f.citas, cita)
	f.menc = append(f.menc, menc)
	f.enviados = append(f.enviados, chat+"|"+texto)
	return nil
}
func (f *falsoEnv) Medio(_ context.Context, chat string, a envio.Adjunto, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.falla != nil {
		return f.falla
	}
	f.enviados = append(f.enviados, chat+"|[medio:"+a.Tipo+"]")
	return nil
}
func (f *falsoEnv) Conectado() bool { return f.conectado }
func (f *falsoEnv) cuantos() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.enviados)
}

type ent struct {
	t      *testing.T
	alm    *almacen.Almacen
	g      *Gestor
	cmd    *comandos.Despachador
	perm   *permisos.Servicio
	env    *falsoEnv
	hora   time.Time
	avisos []string
	camb   []string
}

// nuevoEnt arma un bot de prueba con el módulo Eco registrado, reloj falso, sin esperas al azar y zona UTC.
func nuevoEnt(t *testing.T) *ent {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 9)
	}
	alm, err := almacen.Abrir(t.TempDir(), k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = alm.Cerrar() })
	if err := RegistrarEco(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Quitar(IDEco) })
	p, err := permisos.Nuevo(alm, 1)
	if err != nil {
		t.Fatal(err)
	}
	e := &ent{t: t, alm: alm, perm: p, env: &falsoEnv{conectado: true}, hora: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)}
	e.cmd = comandos.Nuevo(alm, 1, p)
	e.g = NuevoGestor(alm, 1, e.cmd)
	e.g.Ahora = func() time.Time { return e.hora }
	e.g.Aleatorio = func(n int64) int64 { return 0 }
	e.g.Zona = func() *time.Location { return time.UTC }
	e.g.Enviador = e.env
	e.g.Aviso = func(titulo, det string) { e.avisos = append(e.avisos, titulo) }
	e.g.Cambio = func(ev string) { e.camb = append(e.camb, ev) }
	e.alm.Cambio = func(ev string) { e.camb = append(e.camb, ev) }
	e.retraso(0, 0)
	return e
}

// retraso fija el retraso al azar (segundos) sin chocar con el orden mínimo/máximo.
func (e *ent) retraso(mn, mx int) {
	e.fija("retraso_min", "0")
	e.fija("retraso_max", strconv.Itoa(mx))
	e.fija("retraso_min", strconv.Itoa(mn))
}

func (e *ent) fija(clave, valor string) {
	e.t.Helper()
	if err := e.g.FijarAjuste(clave, valor); err != nil {
		e.t.Fatal(err)
	}
}

func (e *ent) enciende() {
	e.t.Helper()
	if err := e.g.Activar(IDEco); err != nil {
		e.t.Fatal(err)
	}
	e.fija("maestro", "1")
}

// corre saca de la cola todo lo que ya puede salir con la hora actual.
func (e *ent) corre() {
	for i := 0; i < 200; i++ {
		if hecho, _ := e.g.Cola.Paso(context.Background()); !hecho {
			return
		}
	}
}

// avanza mueve el reloj y corre.
func (e *ent) avanza(d time.Duration) {
	e.hora = e.hora.Add(d)
	e.corre()
}

func (e *ent) encola(chat, texto string) string {
	e.t.Helper()
	r, err := e.g.Enviar(Iniciativa{Modulo: IDEco, Que: "aviso", Chat: chat, Texto: texto})
	if err != nil {
		e.t.Fatal(err)
	}
	return r
}

func (e *ent) registro() []almacen.RegistroModulo {
	r, _ := e.alm.ListarRegistroModulos(1, "", 100, 0)
	return r
}

func (e *ent) resultados() []string {
	var out []string
	for _, r := range e.registro() {
		out = append(out, r.Resultado)
	}
	return out
}

func tiene(l []string, x string) bool {
	for _, y := range l {
		if y == x {
			return true
		}
	}
	return false
}

// ---------- registro y casillas ----------

func TestRegistroCentral(t *testing.T) {
	e := nuevoEnt(t)
	_ = e
	m, ok := Obtener(IDEco)
	if !ok || m.Nombre() != "Eco de prueba" || m.Riesgo() != RiesgoMedio || !m.Iniciativa() {
		t.Fatalf("módulo: %v %v", m, ok)
	}
	if len(Lista()) < 1 || Lista()[0].ID() != IDEco {
		t.Fatal("la lista no trae el eco")
	}
	if err := Registrar(Nuevo(Def{ID: "Mal Id", Nombre: "x", Descripcion: "x"})); err != ErrIDInvalido {
		t.Fatalf("id inválido: %v", err)
	}
	if err := Registrar(nil); err == nil {
		t.Fatal("nil debe fallar")
	}
	// Un módulo nuevo es una declaración corta.
	if err := Registrar(Nuevo(Def{ID: "minimo", Nombre: "Mínimo", Descripcion: "Nada", Icono: "x"})); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Quitar("minimo") })
	m2, _ := Obtener("minimo")
	if m2.Riesgo() != RiesgoBajo || m2.Iniciativa() || m2.Estado(e.g) != "" {
		t.Fatalf("valores por defecto: %v", m2.Riesgo())
	}
}

func TestCasillasEnElCatalogo(t *testing.T) {
	e := nuevoEnt(t)
	c, ok := permisos.Buscar("eco.usar")
	if !ok || c.Modulo != IDEco || c.Riesgo != RiesgoMedio || c.Grupo != "Eco de prueba" {
		t.Fatalf("casilla al registrar: %+v %v", c, ok)
	}
	// Si el catálogo la pierde, activar el módulo (o iniciar el bot) la vuelve a dejar.
	permisos.Quitar("eco.usar")
	if _, ok := permisos.Buscar("eco.usar"); ok {
		t.Fatal("debió quitarse")
	}
	if err := e.g.Activar(IDEco); err != nil {
		t.Fatal(err)
	}
	if _, ok := permisos.Buscar("eco.usar"); !ok {
		t.Fatal("al activar, la casilla debe estar en el catálogo")
	}
	// Se quita con el módulo.
	Quitar(IDEco)
	if _, ok := permisos.Buscar("eco.usar"); ok {
		t.Fatal("al quitar el módulo se va su casilla")
	}
	if _, ok := Obtener(IDEco); ok {
		t.Fatal("el módulo sigue en el registro")
	}
}

// ---------- activar, comandos, persistencia ----------

func TestComandosSoloMientrasActivo(t *testing.T) {
	e := nuevoEnt(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.g.Iniciar(ctx)
	defer e.g.Detener()
	dueno := func(txt string) comandos.Resultado {
		return e.cmd.Ejecutar(comandos.Entrada{Persona: almacen.PersonaDueno, Chat: "yo@s.whatsapp.net", Texto: txt, Dueno: true, MensajeID: "D"})
	}
	if e.cmd.Existe("!eco hola") {
		t.Fatal("el comando existe con el módulo apagado")
	}
	if r := dueno("!eco hola"); !strings.Contains(r.Texto, "No conozco") {
		t.Fatalf("apagado debe ser desconocido: %+v", r)
	}
	if err := e.g.Activar(IDEco); err != nil {
		t.Fatal(err)
	}
	if !e.cmd.Existe("!eco hola") {
		t.Fatal("al activar debe existir el comando")
	}
	if r := dueno("!eco hola mundo"); !r.Manejado || r.Texto != "hola mundo" {
		t.Fatalf("eco: %+v", r)
	}
	// Aparece en la ayuda solo de quien tiene la casilla; un rango sin ella no lo usa.
	rol, _ := e.perm.GuardarRol(almacen.Rol{Nombre: "Sin eco", Casillas: map[string]almacen.Alcance{"cmd.ayuda": {}}, LimiteMin: 50})
	rolEco, _ := e.perm.GuardarRol(almacen.Rol{Nombre: "Con eco", Casillas: map[string]almacen.Alcance{"eco.usar": {}}, LimiteMin: 50})
	if err := e.perm.AsignarComo(almacen.PersonaDueno, pnAna, rol, 0); err != nil {
		t.Fatal(err)
	}
	if err := e.perm.AsignarComo(almacen.PersonaDueno, pnLuis, rolEco, 0); err != nil {
		t.Fatal(err)
	}
	ayuda := func(p string) string {
		return e.cmd.Ejecutar(comandos.Entrada{Persona: p, Chat: p, Texto: "!ayuda", MensajeID: "A"}).Texto
	}
	if strings.Contains(ayuda(pnAna), "!eco") || !strings.Contains(ayuda(pnLuis), "!eco") {
		t.Fatalf("ayuda por casilla: %q / %q", ayuda(pnAna), ayuda(pnLuis))
	}
	if err := e.g.Desactivar(IDEco); err != nil {
		t.Fatal(err)
	}
	if e.cmd.Existe("!eco hola") {
		t.Fatal("al desactivar, el comando debe irse")
	}
	if r := dueno("!eco hola"); !strings.Contains(r.Texto, "No conozco") {
		t.Fatalf("desactivado debe ser desconocido: %+v", r)
	}
	// Activar otra vez lo trae de vuelta.
	_ = e.g.Activar(IDEco)
	if !e.cmd.Existe("!eco x") {
		t.Fatal("no volvió")
	}
}

func TestEncendidoPersisteYAlArrancarSeRestauran(t *testing.T) {
	e := nuevoEnt(t)
	if e.g.Activo(IDEco) {
		t.Fatal("nace apagado")
	}
	if i, _ := e.g.InfoDe(IDEco); i.Codigo != "apagado" || i.Activo {
		t.Fatalf("info: %+v", i)
	}
	// Sin el bot en marcha se puede activar: queda guardado pero no hay comandos todavía.
	if err := e.g.Activar(IDEco); err != nil {
		t.Fatal(err)
	}
	if e.cmd.Existe("!eco x") {
		t.Fatal("sin bot en marcha no se registran comandos")
	}
	g2 := NuevoGestor(e.alm, 1, e.cmd)
	g2.Ahora = e.g.Ahora
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g2.Iniciar(ctx)
	defer g2.Detener()
	if !g2.Activo(IDEco) || !e.cmd.Existe("!eco x") {
		t.Fatal("al arrancar el bot el módulo encendido debe volver con sus comandos")
	}
	f, _ := e.alm.ObtenerModulo(1, IDEco)
	if f == nil || !f.Activo || f.ActivadoTS == 0 || f.Config == "" {
		t.Fatalf("fila: %+v", f)
	}
	if err := g2.Activar("no_existe"); err != ErrNoExiste {
		t.Fatalf("módulo inexistente: %v", err)
	}
}

func TestDesactivarVaciaLaColaYConservaConfig(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	e.retraso(5, 5)
	if err := e.g.GuardarConfig(IDEco, `{"frase":"hola"}`); err != nil {
		t.Fatal(err)
	}
	if r := e.encola(pnAna, "x"); r != EnCola {
		t.Fatal(r)
	}
	if e.g.Cola.Pendientes() != 1 {
		t.Fatal("debía quedar esperando")
	}
	_ = e.g.Desactivar(IDEco)
	if e.g.Cola.Pendientes() != 0 || !tiene(e.resultados(), "descartado:modulo") {
		t.Fatalf("la cola debió vaciarse: %v", e.resultados())
	}
	if e.g.Config(IDEco).Texto("frase") != "hola" {
		t.Fatal("la configuración se conserva")
	}
	e.avanza(time.Minute)
	if e.env.cuantos() != 0 {
		t.Fatal("nada debió salir")
	}
}

// ---------- configuración ----------

func TestConfiguracionYEsquema(t *testing.T) {
	e := nuevoEnt(t)
	c := e.g.Config(IDEco)
	if c.Texto("frase") != "eco" || c.Int("veces") != 1 || c.Bool("gritar") || c.Texto("estilo") != "normal" || c.Texto("hora_aviso") != "09:00" {
		t.Fatalf("por defecto: %v", c)
	}
	if err := e.g.GuardarConfig(IDEco, `{"frase":"  hola  ","veces":3,"gritar":true,"estilo":"corto","hora_aviso":"08:30"}`); err != nil {
		t.Fatal(err)
	}
	c = e.g.Config(IDEco)
	if c.Texto("frase") != "hola" || c.Int("veces") != 3 || !c.Bool("gritar") || c.Texto("estilo") != "corto" || c.Texto("hora_aviso") != "08:30" {
		t.Fatalf("guardada: %v", c)
	}
	malos := map[string]string{
		`no es json`:   "objeto",
		`[1,2]`:        "objeto",
		`{"frase":5}`:  "texto",
		`{"frase":""}`: "vacío",
		`{"frase":"` + strings.Repeat("a", 41) + `"}`: "largo",
		`{"veces":9}`:            "entre",
		`{"veces":"2"}`:          "número",
		`{"veces":1.5}`:          "entero",
		`{"gritar":"si"}`:        "sí o no",
		`{"estilo":"enorme"}`:    "opción",
		`{"hora_aviso":"25:00"}`: "hora",
		`{"hora_aviso":"8:30"}`:  "hora",
		`{"inventado":1}`:        "desconocido",
	}
	for js, parte := range malos {
		err := e.g.GuardarConfig(IDEco, js)
		if err == nil || !strings.Contains(err.Error(), parte) {
			t.Fatalf("%s: esperaba error con %q, fue %v", js, parte, err)
		}
	}
	if e.g.Config(IDEco).Texto("frase") != "hola" {
		t.Fatal("una configuración mala no debe cambiar nada")
	}
	if err := e.g.GuardarConfig("nadie", `{}`); err != ErrNoExiste {
		t.Fatal(err)
	}
	// Con esquema, la ventana genérica trae los cinco tipos.
	i, _ := e.g.InfoDe(IDEco)
	tipos := map[string]bool{}
	for _, f := range i.Esquema {
		tipos[f.Tipo] = true
	}
	for _, t1 := range []string{CampoTexto, CampoNumero, CampoInterruptor, CampoLista, CampoHora} {
		if !tipos[t1] {
			t.Fatalf("falta el tipo %s en el esquema", t1)
		}
	}
	if err := ValidarEsquema([]Campo{{Clave: "a", Tipo: "raro"}}); err == nil {
		t.Fatal("tipo raro")
	}
	if err := ValidarEsquema([]Campo{{Clave: "a", Tipo: CampoLista}}); err == nil {
		t.Fatal("lista sin opciones")
	}
	if err := ValidarEsquema([]Campo{{Clave: "a", Tipo: CampoTexto}, {Clave: "a", Tipo: CampoTexto}}); err == nil {
		t.Fatal("clave repetida")
	}
	// Un validador propio también se respeta.
	_ = Registrar(Nuevo(Def{ID: "con_validador", Nombre: "V", Descripcion: "v", Validar: func(c map[string]any) error {
		if c["x"] == nil {
			return errors.New("falta x")
		}
		return nil
	}}))
	t.Cleanup(func() { Quitar("con_validador") })
	if err := e.g.GuardarConfig("con_validador", `{"y":1}`); err == nil || !strings.Contains(err.Error(), "falta x") {
		t.Fatalf("validador: %v", err)
	}
	if err := e.g.GuardarConfig("con_validador", `{"x":1}`); err != nil {
		t.Fatal(err)
	}
}

// ---------- interruptor maestro ----------

func TestMaestroApagadoBloqueaAunqueElModuloEsteActivo(t *testing.T) {
	e := nuevoEnt(t)
	if e.g.Maestro() {
		t.Fatal("el maestro nace apagado")
	}
	if err := e.g.Activar(IDEco); err != nil {
		t.Fatal(err)
	}
	if i, _ := e.g.InfoDe(IDEco); i.Codigo != "esperando_maestro" {
		t.Fatalf("estado: %+v", i)
	}
	if r := e.encola(pnAna, "hola"); r != "descartado:maestro" {
		t.Fatalf("con el maestro apagado: %s", r)
	}
	e.avanza(time.Hour)
	if e.env.cuantos() != 0 || !tiene(e.resultados(), "descartado:maestro") {
		t.Fatalf("no debió enviar: %v %v", e.env.enviados, e.resultados())
	}
	// Encendido, sale; apagado con cosas esperando, se descartan de golpe.
	e.fija("maestro", "1")
	e.retraso(30, 30)
	e.encola(pnAna, "uno")
	e.encola(pnLuis, "dos")
	if e.g.Cola.Pendientes() != 2 {
		t.Fatal("deben esperar su retraso")
	}
	if err := e.g.FijarAjuste("maestro", "0"); err != nil {
		t.Fatal(err)
	}
	if e.g.Cola.Pendientes() != 0 {
		t.Fatal("al apagar el maestro se vacía la cola")
	}
	e.avanza(time.Hour)
	if e.env.cuantos() != 0 {
		t.Fatal("nada sale con el maestro apagado")
	}
	// Y si se apaga justo entre sacar de la cola y enviar (se revisa otra vez al enviar).
	e.fija("maestro", "1")
	e.encola(pnAna, "tres")
	e.alm.FijarAjusteIni(1, "ini_maestro", "0") // apagado por fuera de FijarMaestro (no vacía la cola)
	e.avanza(time.Hour)
	if e.env.cuantos() != 0 {
		t.Fatal("se debe volver a revisar el maestro al enviar")
	}
}

func TestEnvioNormalYNoEsIniciativa(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	if r := e.encola(pnAna, "hola"); r != EnCola {
		t.Fatal(r)
	}
	e.corre()
	if e.env.cuantos() != 1 || e.env.enviados[0] != pnAna+"|hola" {
		t.Fatalf("enviados: %v", e.env.enviados)
	}
	// Un módulo que no declara iniciativa no puede usar la cola.
	_ = Registrar(Nuevo(Def{ID: "pasivo", Nombre: "P", Descripcion: "p"}))
	t.Cleanup(func() { Quitar("pasivo") })
	_ = e.g.Activar("pasivo")
	r, _ := e.g.Enviar(Iniciativa{Modulo: "pasivo", Chat: pnAna, Texto: "x"})
	if r != "descartado:"+MotNoIniciativa {
		t.Fatalf("pasivo: %s", r)
	}
	if _, err := e.g.Enviar(Iniciativa{Modulo: IDEco, Chat: pnAna}); err == nil {
		t.Fatal("sin texto ni medio debe fallar")
	}
	if _, err := e.g.Enviar(Iniciativa{Chat: pnAna, Texto: "x"}); err == nil {
		t.Fatal("sin módulo debe fallar")
	}
	// Con medio.
	if r, _ := e.g.Enviar(Iniciativa{Modulo: IDEco, Chat: pnAna, Medio: &envio.Adjunto{Tipo: "imagen"}}); r != EnCola {
		t.Fatal(r)
	}
	e.avanza(time.Minute)
	if e.env.cuantos() != 2 || !strings.Contains(e.env.enviados[1], "[medio:imagen]") {
		t.Fatalf("medio: %v", e.env.enviados)
	}
}

// ---------- topes ----------

func TestTopeGlobalPorHora(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	e.fija("tope_hora", "2")
	e.fija("espera_max", "30")
	for i := 0; i < 3; i++ {
		e.encola(pnAna, "m")
	}
	e.corre()
	if e.env.cuantos() != 2 {
		t.Fatalf("el tope por hora es 2: salieron %d", e.env.cuantos())
	}
	// La tercera tendría que esperar casi una hora: es más que la espera máxima (30 min), se descarta.
	if !tiene(e.resultados(), "descartado:tope") {
		t.Fatalf("resultados: %v", e.resultados())
	}
	// Las fichas vuelven pasada la hora.
	e.avanza(61 * time.Minute)
	e.encola(pnAna, "otra")
	e.corre()
	if e.env.cuantos() != 3 {
		t.Fatalf("pasada la hora vuelve a salir: %d", e.env.cuantos())
	}
}

func TestTopeEsperaSiCabeEnLaEsperaMaxima(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	e.fija("tope_hora", "1")
	e.fija("espera_max", "90")
	e.encola(pnAna, "a")
	e.encola(pnLuis, "b")
	e.corre()
	if e.env.cuantos() != 1 || e.g.Cola.Pendientes() != 1 {
		t.Fatalf("la segunda espera su ficha: %d enviados, %d pendientes", e.env.cuantos(), e.g.Cola.Pendientes())
	}
	e.avanza(30 * time.Minute)
	if e.env.cuantos() != 1 {
		t.Fatal("todavía no hay ficha")
	}
	e.avanza(31 * time.Minute)
	if e.env.cuantos() != 2 {
		t.Fatalf("ya hay ficha: %d", e.env.cuantos())
	}
}

func TestTopeGlobalPorDia(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	e.fija("tope_dia", "3")
	e.fija("tope_hora", "100")
	e.fija("espera_max", "60")
	e.alm.FijarTopesModulo(1, IDEco, 0, 0)
	e.g.olvidar(IDEco)
	// El eco trae 5 por hora y 20 por día; el global de 3 por día manda.
	for i := 0; i < 5; i++ {
		e.encola(pnAna, "m")
		e.avanza(11 * time.Minute) // a las 12:00..12:55: dentro del mismo día
	}
	if e.env.cuantos() != 3 {
		t.Fatalf("tope diario 3: %d", e.env.cuantos())
	}
	e.avanza(24 * time.Hour)
	e.encola(pnAna, "nuevo día")
	e.corre()
	if e.env.cuantos() != 4 {
		t.Fatalf("al día siguiente vuelve: %d", e.env.cuantos())
	}
}

func TestTopeDelModuloConfigurable(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	e.fija("tope_hora", "100")
	e.fija("tope_dia", "1000")
	e.fija("espera_max", "5")
	if h, d := e.g.TopesModulo(IDEco); h != 5 || d != 20 {
		t.Fatalf("topes por defecto del módulo: %d/%d", h, d)
	}
	if err := e.g.FijarTopesModulo(IDEco, 2, 0); err != nil {
		t.Fatal(err)
	}
	if h, d := e.g.TopesModulo(IDEco); h != 2 || d != 20 {
		t.Fatalf("topes propios: %d/%d", h, d)
	}
	for i := 0; i < 4; i++ {
		e.encola(pnAna, "m")
	}
	e.corre()
	if e.env.cuantos() != 2 {
		t.Fatalf("tope del módulo 2 por hora: %d", e.env.cuantos())
	}
	if err := e.g.FijarTopesModulo(IDEco, -1, 0); err == nil {
		t.Fatal("topes negativos")
	}
}

func TestFichasSobrevivenAUnReinicio(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	e.fija("tope_hora", "2")
	e.fija("espera_max", "5")
	e.encola(pnAna, "a")
	e.encola(pnAna, "b")
	e.corre()
	if e.env.cuantos() != 2 {
		t.Fatal("deben salir dos")
	}
	// Otro gestor (como tras reiniciar la app) lee lo gastado del registro y no regala fichas.
	g2 := NuevoGestor(e.alm, 1, nil)
	g2.Ahora, g2.Aleatorio, g2.Zona, g2.Enviador = e.g.Ahora, e.g.Aleatorio, e.g.Zona, e.env
	if r, _ := g2.Enviar(Iniciativa{Modulo: IDEco, Que: "aviso", Chat: pnAna, Texto: "c"}); r != EnCola {
		t.Fatal(r)
	}
	g2.Cola.Paso(context.Background())
	if e.env.cuantos() != 2 {
		t.Fatalf("el reinicio no debe regalar fichas: %d", e.env.cuantos())
	}
}

func TestRetrasoAlAzarYSeparacionEntreEnvios(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	e.retraso(5, 9)
	e.g.Aleatorio = func(n int64) int64 { return n - 1 } // el máximo (menos un nanosegundo)
	e.encola(pnAna, "a")
	e.encola(pnLuis, "b")
	e.corre()
	if e.env.cuantos() != 0 {
		t.Fatal("antes del retraso no sale nada")
	}
	e.avanza(10 * time.Second)
	if e.env.cuantos() != 1 {
		t.Fatalf("pasado el retraso sale una: %d", e.env.cuantos())
	}
	e.avanza(3 * time.Second)
	if e.env.cuantos() != 1 {
		t.Fatal("entre una y otra hay espera")
	}
	e.avanza(10 * time.Second)
	if e.env.cuantos() != 2 {
		t.Fatalf("la segunda sale después: %d", e.env.cuantos())
	}
	e.fija("retraso_min", "20") // el máximo sube con el mínimo
	if e.g.Alm.AjusteIniInt(1, "ini_retraso_max") != 20 {
		t.Fatal("el máximo debe subir con el mínimo")
	}
	e.fija("retraso_max", "2")
	if e.g.Alm.AjusteIniInt(1, "ini_retraso_min") != 2 {
		t.Fatal("el mínimo debe bajar con el máximo")
	}
}

// ---------- pausa nocturna ----------

func TestPausaNocturnaDifiere(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	e.hora = time.Date(2026, 11, 10, 23, 30, 0, 0, time.UTC)
	e.fija("espera_max", "5")
	e.encola(pnAna, "de noche")
	e.corre()
	if e.env.cuantos() != 0 || !tiene(e.resultados(), "diferido") {
		t.Fatalf("de noche no sale; se difiere: %v %v", e.env.enviados, e.resultados())
	}
	e.avanza(6 * time.Hour) // 05:30
	if e.env.cuantos() != 0 {
		t.Fatal("sigue la noche")
	}
	e.avanza(90 * time.Minute) // 07:00
	if e.env.cuantos() != 1 {
		t.Fatalf("al terminar la pausa sale: %d", e.env.cuantos())
	}
	// Un solo "diferido" en el registro por iniciativa.
	n := 0
	for _, r := range e.resultados() {
		if r == "diferido" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("diferidos: %d", n)
	}
}

func TestPausaNocturnaDescartaSiElModuloLoPide(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	e.hora = time.Date(2026, 11, 10, 3, 0, 0, 0, time.UTC)
	r, _ := e.g.Enviar(Iniciativa{Modulo: IDEco, Que: "aviso", Chat: pnAna, Texto: "x", Noche: NocheDescartar})
	if r != EnCola {
		t.Fatal(r)
	}
	e.corre()
	if e.env.cuantos() != 0 || !tiene(e.resultados(), "descartado:noche") {
		t.Fatalf("debió descartarse: %v", e.resultados())
	}
	e.avanza(5 * time.Hour)
	if e.env.cuantos() != 0 {
		t.Fatal("no vuelve")
	}
}

func TestPausaNocturnaConfigurableYApagable(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	e.hora = time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC)
	e.fija("noche_desde", "13:00")
	e.fija("noche_hasta", "15:00") // ventana dentro del mismo día
	e.encola(pnAna, "a")
	e.corre()
	if e.env.cuantos() != 0 {
		t.Fatal("dentro de la ventana configurada no sale")
	}
	e.avanza(61 * time.Minute)
	if e.env.cuantos() != 1 {
		t.Fatalf("termina a las 15:00: %d", e.env.cuantos())
	}
	e.fija("noche_on", "0")
	e.hora = time.Date(2026, 11, 11, 14, 0, 0, 0, time.UTC)
	e.encola(pnAna, "b")
	e.corre()
	if e.env.cuantos() != 2 {
		t.Fatal("con la pausa apagada sale a cualquier hora")
	}
	for _, mal := range []string{"25:00", "7:00", "mañana", ""} {
		if err := e.g.FijarAjuste("noche_desde", mal); err == nil {
			t.Fatalf("hora mala %q aceptada", mal)
		}
	}
	// Zona horaria: 23:30 UTC es 17:30 en UTC-6, fuera de la pausa.
	e.fija("noche_on", "1")
	e.fija("noche_desde", "23:00")
	e.fija("noche_hasta", "07:00")
	e.g.Zona = func() *time.Location { return time.FixedZone("mx", -6*3600) }
	e.hora = time.Date(2026, 11, 12, 23, 30, 0, 0, time.UTC)
	e.encola(pnAna, "c")
	e.corre()
	if e.env.cuantos() != 3 {
		t.Fatal("la pausa se mide en la hora local")
	}
}

// ---------- No molestar ----------

func TestEsBaja(t *testing.T) {
	for _, s := range []string{"BAJA", "baja", " Baja ", "STOP", "stop!", "No más", "NO MAS", "no  más.", "¡BAJA!", "nomás", "Stop."} {
		if !EsBaja(s) {
			t.Errorf("%q debe ser baja", s)
		}
	}
	for _, s := range []string{"", "hola", "quiero la baja de mi cuenta", "bajar", "stop por favor", "no", "más", "estoy en baja"} {
		if EsBaja(s) {
			t.Errorf("%q no es baja", s)
		}
	}
}

func TestNoMolestarConfirmaUnaSolaVezYBloquea(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	es, conf := e.g.Baja(pnAna, "BAJA")
	if !es || conf == nil {
		t.Fatal("debe detectarse")
	}
	if !e.alm.EstaNoMolestar(1, pnAna) {
		t.Fatal("debe quedar en la lista de inmediato")
	}
	if txt := conf(); txt == "" || strings.Contains(txt, "BAJA") {
		t.Fatalf("primera confirmación: %q", txt)
	}
	// Una segunda baja: sigue consumida pero ya no se le vuelve a escribir.
	es2, conf2 := e.g.Baja(pnAna, "stop")
	if !es2 || conf2() != "" {
		t.Fatal("la confirmación es una sola vez")
	}
	// No se detecta en grupos ni en textos normales.
	if es, _ := e.g.Baja(grupo, "baja"); es {
		t.Fatal("en grupos no")
	}
	if es, _ := e.g.Baja(pnLuis, "hola"); es {
		t.Fatal("texto normal")
	}
	// Ninguna iniciativa le llega.
	if r := e.encola(pnAna, "promo"); r != "descartado:no_molestar" {
		t.Fatalf("a quien está en la lista: %s", r)
	}
	e.avanza(time.Minute)
	if e.env.cuantos() != 0 {
		t.Fatal("no debió enviarse")
	}
	// Sí a los demás; y en grupos, si se menciona a alguien de la lista (Personas), tampoco.
	if r := e.encola(pnLuis, "promo"); r != EnCola {
		t.Fatal(r)
	}
	r, _ := e.g.Enviar(Iniciativa{Modulo: IDEco, Chat: grupo, Personas: []string{pnAna}, Texto: "bienvenida"})
	if r != "descartado:no_molestar" {
		t.Fatalf("grupo con persona en la lista: %s", r)
	}
	if r, _ := e.g.Enviar(Iniciativa{Modulo: IDEco, Chat: grupo, Texto: "al grupo"}); r != EnCola {
		t.Fatalf("al grupo sin personas: %s", r)
	}
	e.corre()
	if e.env.cuantos() != 2 {
		t.Fatalf("enviados: %v", e.env.enviados)
	}
	// Quitarla de la lista la deja de nuevo.
	if err := e.g.QuitarNoMolestar(pnAna); err != nil {
		t.Fatal(err)
	}
	if r := e.encola(pnAna, "ya sí"); r != EnCola {
		t.Fatal(r)
	}
}

func TestNoMolestarSeRevisaAlEnviarYUneNumeroYLid(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	e.retraso(10, 10)
	e.alm.AprenderAlias(1, pnAna, lidAna)
	e.encola(lidAna+"", "para el lid") // llega por LID
	if err := e.g.AgregarNoMolestar(pnAna, "a mano"); err != nil {
		t.Fatal(err)
	}
	e.avanza(time.Minute)
	if e.env.cuantos() != 0 || !tiene(e.resultados(), "descartado:no_molestar") {
		t.Fatalf("la misma persona por LID: %v %v", e.env.enviados, e.resultados())
	}
	// Agregada a mano: no se le escribe ni para confirmar.
	if _, conf := e.g.Baja(pnAna, "baja"); conf() != "" {
		t.Fatal("agregada a mano no se confirma")
	}
	lista, _ := e.alm.ListarNoMolestar(1)
	if len(lista) != 1 || lista[0].Motivo != "a mano" {
		t.Fatalf("lista: %+v", lista)
	}
	if err := e.g.AgregarNoMolestar(" ", ""); err == nil {
		t.Fatal("persona vacía")
	}
}

// ---------- modo seguro ----------

func TestModoSeguroPorErroresSeguidos(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	e.fija("errores", "3")
	e.fija("tope_hora", "100")
	e.env.falla = errors.New("sin red")
	for i := 0; i < 3; i++ {
		e.encola(pnAna, "m")
	}
	e.corre()
	if !e.g.EnModoSeguro() {
		t.Fatalf("tres fallos seguidos activan el modo seguro: errores=%d", e.g.Cola.ErroresSeguidos())
	}
	if len(e.avisos) != 1 || e.avisos[0] != "Modo seguro activado" {
		t.Fatalf("avisos: %v", e.avisos)
	}
	// No se apaga: sigue activo pero pausado.
	if !e.g.Activo(IDEco) {
		t.Fatal("el módulo no se apaga")
	}
	i, _ := e.g.InfoDe(IDEco)
	if i.Codigo != "pausado_seguro" || !i.Pausado || !i.Activo || !strings.Contains(i.Estado, "modo seguro") {
		t.Fatalf("info: %+v", i)
	}
	// Mientras dure, nada sale y los eventos no le llegan.
	e.env.falla = nil
	if r := e.encola(pnAna, "x"); r != "descartado:seguro" {
		t.Fatal(r)
	}
	vistos := len(e.registro())
	e.g.Evento(Evento{Tipo: EvMiembroEntra, Chat: grupo, Personas: []string{pnLuis}})
	if e.g.Cola.Pendientes() != 0 || len(e.registro()) != vistos {
		t.Fatal("un módulo pausado no recibe eventos")
	}
	var s struct {
		Activo bool   `json:"activo"`
		Motivo string `json:"motivo"`
		Texto  string `json:"texto"`
	}
	_ = json.Unmarshal([]byte(e.g.SeguroJSON()), &s)
	if !s.Activo || s.Motivo != "errores" || !strings.Contains(s.Texto, "fallaron varios envíos") {
		t.Fatalf("estado: %+v", s)
	}
	// Reanudar a mano.
	if err := e.g.Reanudar(); err != nil {
		t.Fatal(err)
	}
	if e.g.EnModoSeguro() || e.g.Cola.ErroresSeguidos() != 0 {
		t.Fatal("reanudado")
	}
	if r := e.encola(pnAna, "ya"); r != EnCola {
		t.Fatal(r)
	}
	e.corre()
	if e.env.cuantos() != 1 {
		t.Fatal("tras reanudar vuelve a enviar")
	}
}

func TestUnExitoReiniciaElContadorDeErrores(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	e.fija("errores", "3")
	e.fija("tope_hora", "100")
	e.env.falla = errors.New("x")
	e.encola(pnAna, "1")
	e.encola(pnAna, "2")
	e.corre()
	e.env.falla = nil
	e.encola(pnAna, "3")
	e.corre()
	e.env.falla = errors.New("x")
	e.encola(pnAna, "4")
	e.encola(pnAna, "5")
	e.corre()
	if e.g.EnModoSeguro() {
		t.Fatal("no eran 3 seguidos")
	}
}

func TestModoSeguroPorDesconexionForzada(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	e.retraso(30, 30)
	e.encola(pnAna, "esperando")
	e.g.ModoSeguro("desconexion:sesion_cerrada")
	e.g.ModoSeguro("desconexion:otra") // una segunda no repite el aviso
	if !e.g.EnModoSeguro() || len(e.avisos) != 1 {
		t.Fatalf("avisos: %v", e.avisos)
	}
	if e.g.Cola.Pendientes() != 0 || !tiene(e.resultados(), "descartado:seguro") {
		t.Fatalf("lo que esperaba se descarta: %v", e.resultados())
	}
	act, ts, mot := e.g.EstadoSeguro()
	if !act || ts == 0 || mot != "desconexion:sesion_cerrada" {
		t.Fatalf("estado: %v %v %v", act, ts, mot)
	}
	// Persiste: otro gestor sobre la misma base lo ve.
	if !NuevoGestor(e.alm, 1, nil).EnModoSeguro() {
		t.Fatal("el modo seguro debe sobrevivir a un reinicio")
	}
	// Los módulos sin iniciativa no se pausan.
	_ = Registrar(Nuevo(Def{ID: "pasivo", Nombre: "P", Descripcion: "p"}))
	t.Cleanup(func() { Quitar("pasivo") })
	_ = e.g.Activar("pasivo")
	if i, _ := e.g.InfoDe("pasivo"); i.Pausado || i.Codigo != "activo" {
		t.Fatalf("pasivo: %+v", i)
	}
}

func TestPausaYReanudaLlegaALosModulos(t *testing.T) {
	e := nuevoEnt(t)
	var pausas, reanudas int
	_ = Registrar(Nuevo(Def{ID: "avisado", Nombre: "A", Descripcion: "a", Iniciativa: true,
		AlPausar: func(*Gestor) { pausas++ }, AlReanudar: func(*Gestor) { reanudas++ }}))
	t.Cleanup(func() { Quitar("avisado") })
	_ = e.g.Activar("avisado")
	e.g.ModoSeguro("errores")
	_ = e.g.Reanudar()
	if pausas != 1 || reanudas != 1 {
		t.Fatalf("pausas %d reanudas %d", pausas, reanudas)
	}
}

// ---------- registro sin textos y eventos ----------

func TestRegistroSinTextos(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	secreto := "TEXTO-SECRETO-123 número 5215599999999"
	e.encola(pnAna, secreto)
	e.env.falla = nil
	e.corre()
	if e.env.cuantos() != 1 {
		t.Fatal("debió salir")
	}
	e.fija("maestro", "0")
	e.encola(pnAna, secreto)
	reg := e.registro()
	if len(reg) < 2 {
		t.Fatalf("registro: %+v", reg)
	}
	b, _ := json.Marshal(reg)
	if strings.Contains(string(b), "SECRETO") || strings.Contains(string(b), "número") {
		t.Fatalf("el registro lleva texto: %s", b)
	}
	got := map[string]bool{}
	for _, r := range reg {
		got[r.Resultado] = true
		if r.Modulo != IDEco || r.Que != "aviso" || r.TS == 0 || r.Chat != pnAna {
			t.Fatalf("renglón: %+v", r)
		}
	}
	if !got["enviado"] || !got["descartado:maestro"] {
		t.Fatalf("resultados: %v", got)
	}
	// Se avisa a la app.
	if !tiene(e.camb, "iniciativas") {
		t.Fatalf("cambios: %v", e.camb)
	}
}

func TestEventosSoloALosActivos(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	e.fija("espera_max", "5")
	e.g.Evento(Evento{Tipo: EvMiembroEntra, Chat: grupo, Personas: []string{pnLuis}})
	e.corre()
	if e.env.cuantos() != 1 || e.env.enviados[0] != grupo+"|eco" {
		t.Fatalf("el eco saluda al grupo: %v", e.env.enviados)
	}
	if i, _ := e.g.InfoDe(IDEco); !strings.Contains(i.Estado, "1 evento visto") {
		t.Fatalf("estado: %q", i.Estado)
	}
	// La alarma de cada minuto manda al chat configurado.
	_ = e.g.GuardarConfig(IDEco, `{"frase":"aviso","gritar":true,"chat_aviso":"`+pnLuis+`"}`)
	e.g.Evento(Evento{Tipo: EvTick})
	e.corre()
	if e.env.cuantos() != 2 || e.env.enviados[1] != pnLuis+"|AVISO" {
		t.Fatalf("tick: %v", e.env.enviados)
	}
	// Apagado, no recibe nada. Un módulo que se rompe no tumba a los demás.
	_ = e.g.Desactivar(IDEco)
	e.g.Evento(Evento{Tipo: EvTick})
	e.corre()
	if e.env.cuantos() != 2 {
		t.Fatal("apagado no recibe eventos")
	}
	_ = Registrar(Nuevo(Def{ID: "roto", Nombre: "R", Descripcion: "r", AlEvento: func(*Gestor, Evento) { panic("boom") }}))
	t.Cleanup(func() { Quitar("roto") })
	_ = e.g.Activar("roto")
	_ = e.g.Activar(IDEco)
	e.g.Evento(Evento{Tipo: EvTick}) // no debe entrar en pánico
	e.corre()
	if e.env.cuantos() != 3 {
		t.Fatalf("el eco sigue trabajando: %d", e.env.cuantos())
	}
}

func TestSinConexionEsperaYDescarta(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	e.fija("espera_max", "2")
	e.env.conectado = false
	e.encola(pnAna, "x")
	e.corre()
	e.avanza(time.Minute)
	if e.env.cuantos() != 0 || e.g.Cola.Pendientes() != 1 {
		t.Fatal("espera la conexión")
	}
	e.env.conectado = true
	e.avanza(31 * time.Second)
	if e.env.cuantos() != 1 {
		t.Fatalf("con conexión sale: %d", e.env.cuantos())
	}
	e.env.conectado = false
	e.encola(pnAna, "y")
	e.corre()
	e.avanza(3 * time.Minute)
	if !tiene(e.resultados(), "descartado:sin_conexion") {
		t.Fatalf("resultados: %v", e.resultados())
	}
}

func TestAjustesValidados(t *testing.T) {
	e := nuevoEnt(t)
	malos := [][2]string{{"tope_hora", "0"}, {"tope_hora", "abc"}, {"tope_dia", "99999"}, {"noche_on", "2"}, {"errores", "0"}, {"espera_max", "0"}, {"baja_texto", ""}, {"nada", "1"}}
	for _, m := range malos {
		if err := e.g.FijarAjuste(m[0], m[1]); err == nil {
			t.Errorf("%v aceptado", m)
		}
	}
	e.fija("baja_texto", "Hecho.")
	var a map[string]string
	_ = json.Unmarshal([]byte(e.g.AjustesJSON()), &a)
	if a["maestro"] != "0" || a["tope_hora"] != "10" || a["baja_texto"] != "Hecho." || a["noche_desde"] != "23:00" {
		t.Fatalf("ajustes: %v", a)
	}
	if _, ok := a["seguro"]; ok {
		t.Fatal("el estado interno del modo seguro no va en los ajustes")
	}
}

func TestPasoDevuelveEspera(t *testing.T) {
	e := nuevoEnt(t)
	e.enciende()
	if hecho, esp := e.g.Cola.Paso(context.Background()); hecho || esp != -1 {
		t.Fatalf("cola vacía: %v %v", hecho, esp)
	}
	e.retraso(7, 7)
	e.encola(pnAna, "x")
	if hecho, esp := e.g.Cola.Paso(context.Background()); hecho || esp != 7*time.Second {
		t.Fatalf("debe esperar 7 s: %v %v", hecho, esp)
	}
}

func TestTrabajadorRealEnvia(t *testing.T) {
	e := nuevoEnt(t)
	e.g.Ahora = time.Now
	e.g.Zona = func() *time.Location { return time.UTC }
	e.fija("noche_on", "0")
	e.enciende()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.g.Iniciar(ctx)
	defer e.g.Detener()
	e.encola(pnAna, "real")
	for i := 0; i < 100 && e.env.cuantos() == 0; i++ {
		time.Sleep(50 * time.Millisecond)
	}
	if e.env.cuantos() != 1 {
		t.Fatal("el trabajador no envió")
	}
}
