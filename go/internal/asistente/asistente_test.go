package asistente

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"botchat/internal/almacen"
	"botchat/internal/flujo"
)

type modFalso struct {
	activo  bool
	cfg     string
	activos int
	apagos  int
}

func (m *modFalso) Activo(string) bool                { return m.activo }
func (m *modFalso) Activar(string) error              { m.activo = true; m.activos++; return nil }
func (m *modFalso) Desactivar(string) error           { m.activo = false; m.apagos++; return nil }
func (m *modFalso) GuardarConfig(_, t string) error   { m.cfg = t; return nil }
func (m *modFalso) ConfigJSON(string) (string, error) { return m.cfg, nil }

func abre(t *testing.T) *almacen.Almacen {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 7)
	}
	a, err := almacen.Abrir(t.TempDir(), k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Cerrar() })
	return a
}

const g1 = "120363000000000001@g.us"

func TestBorradorSeGuardaCifradoYSeReanuda(t *testing.T) {
	a := abre(t)
	b := Leer(a, 1)
	if b.Paso != PasoDonde || len(b.Menu.Opciones) != 3 || !b.Privados || b.Reglas.AlEntrar {
		t.Fatalf("arranque: %+v", b)
	}
	b.Paso = PasoMenu
	b.Bienvenida.Texto = "Texto secreto de prueba 123"
	if err := Guardar(a, 1, b); err != nil {
		t.Fatal(err)
	}
	// En disco no aparece en claro.
	v := a.BorradorCrudoAsistente(1)
	if v == "" {
		t.Fatal("no se guardó")
	}
	if strings.Contains(v, "secreto") {
		t.Fatal("el borrador debe ir cifrado")
	}
	otro := Leer(a, 1)
	if otro.Paso != PasoMenu || otro.Bienvenida.Texto != "Texto secreto de prueba 123" {
		t.Fatalf("reanudar: %+v", otro)
	}
	// No sale en la lista general de ajustes.
	if _, ok := a.AjustesBot(1)["asistente_borrador"]; ok {
		t.Fatal("el borrador no debe viajar con los ajustes")
	}
	_ = Descartar(a, 1)
	if Leer(a, 1).Paso != PasoDonde {
		t.Fatal("descartado")
	}
}

func TestAplicarCreaTodoYDeshacerLoQuita(t *testing.T) {
	a := abre(t)
	m := &modFalso{}
	b := Nuevo()
	b.Grupos, b.GruposElegidos = true, []string{g1}
	b.Reglas.AlEntrar, b.Reglas.AvisoOk = true, true
	b.Util.Silenciar, b.Util.Notas, b.Util.Recordatorios, b.Util.Advertencias = false, false, false, false
	if p := b.Problemas(); len(p) != 0 {
		t.Fatal(p)
	}
	c, err := Aplicar(a, 1, b, m)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Autos) != 1 || c.Menu == 0 || !c.Modulo || !m.activo || len(c.Etiquetas) != 3 {
		t.Fatalf("creado: %+v", c)
	}
	autos, _ := a.ListarAutos(1, almacen.AutoBienvenida)
	if len(autos) != 1 || !autos[0].Activo {
		t.Fatalf("bienvenida: %+v", autos)
	}
	menus, _ := a.ListarMenus(1)
	if len(menus) != 1 || !menus[0].Activo {
		t.Fatalf("menú: %+v", menus)
	}
	d, err := flujo.Parse(menus[0].Def)
	if err != nil || len(d.Pasos) != 4 || !d.Alcance.Privados || len(d.Alcance.Grupos) != 1 {
		t.Fatalf("def: %v %+v", err, d)
	}
	if !a.EstaChatMod(1, IDBienvenidaModulo, g1) {
		t.Fatal("grupo del módulo")
	}
	if a.AjusteBot(1, "sil_activo") != "0" || a.AjusteBot(1, "util_notas") != "0" || a.AjusteBot(1, "adv_tope") != "0" {
		t.Fatal("ajustes")
	}
	// Quedó guardado qué se creó.
	if Leer(a, 1).Creado == nil {
		t.Fatal("falta Creado")
	}
	// Deshacer.
	bb := Leer(a, 1)
	if err := Deshacer(a, 1, bb, m); err != nil {
		t.Fatal(err)
	}
	if autos, _ := a.ListarAutos(1, almacen.AutoBienvenida); len(autos) != 0 {
		t.Fatal("la bienvenida debía borrarse")
	}
	if menus, _ := a.ListarMenus(1); len(menus) != 0 {
		t.Fatal("el menú debía borrarse")
	}
	if et, _ := a.ListarEtiquetas(1); len(et) != 0 {
		t.Fatal("etiquetas")
	}
	if m.activo || a.EstaChatMod(1, IDBienvenidaModulo, g1) {
		t.Fatal("módulo")
	}
	if a.AjusteBot(1, "sil_activo") != "1" || a.AjusteBot(1, "util_notas") != "1" || a.AjusteBot(1, "adv_tope") != "3" {
		t.Fatalf("ajustes no volvieron: %v", a.AjustesBot(1))
	}
	if Leer(a, 1).Creado != nil {
		t.Fatal("Creado debía borrarse")
	}
}

func TestNoTocaLoDelUsuarioNiPisaUnModuloYaEncendido(t *testing.T) {
	a := abre(t)
	// Algo del usuario: una etiqueta propia y un menú propio.
	_, _ = a.CrearEtiqueta(1, "Mía", "#2255CC")
	propio := &flujo.Def{V: 1, Nombre: "Mío", Inicio: "r", Pasos: []flujo.Paso{{ID: "r", Tipo: flujo.PasoRespuesta, Texto: "x"}}, Disparadores: flujo.Disparadores{Palabras: []string{"x"}}, Alcance: flujo.Alcance{Privados: true}}
	propio.Normalizar()
	idPropio, _ := a.CrearMenu(1, propio.JSON(), true, false)
	m := &modFalso{activo: true, cfg: `{"mensaje":"del usuario"}`}
	b := Nuevo()
	b.Grupos, b.GruposElegidos = true, []string{g1}
	b.Reglas.AlEntrar, b.Reglas.AvisoOk = true, true
	c, err := Aplicar(a, 1, b, m)
	if err != nil {
		t.Fatal(err)
	}
	if c.Modulo || m.activos != 0 || m.cfg != `{"mensaje":"del usuario"}` || len(c.Avisos) != 1 {
		t.Fatalf("no debía tocar el módulo del usuario: %+v %+v", c, m)
	}
	if len(c.Etiquetas) != 0 {
		t.Fatal("con etiquetas propias no se siembran las de ejemplo")
	}
	if err := Deshacer(a, 1, Leer(a, 1), m); err != nil {
		t.Fatal(err)
	}
	if mn, _ := a.ObtenerMenu(1, idPropio); mn == nil {
		t.Fatal("el menú del usuario debe seguir")
	}
	if et, _ := a.ListarEtiquetas(1); len(et) != 1 || et[0].Nombre != "Mía" {
		t.Fatalf("etiquetas del usuario: %+v", et)
	}
	if !m.activo {
		t.Fatal("el módulo del usuario debe seguir encendido")
	}
}

func TestProblemasYAplicarNoDejaMediasCosas(t *testing.T) {
	a := abre(t)
	b := Nuevo()
	b.Privados = false
	if len(b.Problemas()) == 0 {
		t.Fatal("sin dónde responder debe avisar")
	}
	if _, err := Aplicar(a, 1, b, nil); err == nil {
		t.Fatal("debía fallar")
	}
	b = Nuevo()
	b.Reglas.AlEntrar = true // sin aviso aceptado y sin grupo
	if p := b.Problemas(); len(p) != 2 {
		t.Fatalf("%v", p)
	}
	// Un fallo a la mitad deshace lo creado: el módulo no existe.
	b = Nuevo()
	b.Grupos, b.GruposElegidos = true, []string{g1}
	b.Reglas.AlEntrar, b.Reglas.AvisoOk = true, true
	if _, err := Aplicar(a, 1, b, &falla{}); err == nil {
		t.Fatal("debía fallar")
	}
	if autos, _ := a.ListarAutos(1, ""); len(autos) != 0 {
		t.Fatal("quedó una bienvenida a medias")
	}
	if menus, _ := a.ListarMenus(1); len(menus) != 0 {
		t.Fatal("quedó un menú a medias")
	}
	if a.AjusteBot(1, "sil_activo") != "1" {
		t.Fatal("ajustes")
	}
}

type falla struct{ modFalso }

func (f *falla) Activar(string) error { return errors.New("no se pudo") }

func TestBienvenidaUnaVezAlDiaYNoEncendidaDeFabrica(t *testing.T) {
	a := abre(t)
	// De fábrica: nada encendido por el solo hecho de abrir el asistente.
	if autos, _ := a.ListarAutos(1, ""); len(autos) != 0 {
		t.Fatal("el asistente no crea nada solo")
	}
	b := Nuevo()
	b.Bienvenida.Cuando = "diaria"
	b.Menu.Activo = false
	b.Reglas.AlEntrar = false
	if _, err := Aplicar(a, 1, b, nil); err != nil {
		t.Fatal(err)
	}
	autos, _ := a.ListarAutos(1, almacen.AutoRecibido)
	if len(autos) != 1 {
		t.Fatalf("una vez al día debe ser un «recibido» diario: %+v", autos)
	}
	if !strings.Contains(autos[0].Config, `"dias"`) {
		t.Fatalf("config: %s", autos[0].Config)
	}
	// Aplicar otra vez no duplica: deshace lo anterior primero.
	if _, err := Aplicar(a, 1, Leer(a, 1), nil); err != nil {
		t.Fatal(err)
	}
	if autos, _ := a.ListarAutos(1, ""); len(autos) != 1 {
		t.Fatalf("duplicó: %d", len(autos))
	}
}

func TestTextoDePruebaYNormalizar(t *testing.T) {
	b := Nuevo()
	if got := TextoDePrueba(b, "Ana"); !strings.HasPrefix(got, "¡Hola Ana!") {
		t.Fatal(got)
	}
	raw, _ := json.Marshal(map[string]any{"v": 1, "paso": 99, "gruposElegidos": []string{"x", g1, g1}, "menu": map[string]any{"opciones": []Opcion{{" ", "a"}, {"Uno", " u "}}}})
	var c Borrador
	_ = json.Unmarshal(raw, &c)
	c.Normalizar()
	if c.Paso != PasoFinal || len(c.GruposElegidos) != 1 || len(c.Menu.Opciones) != 1 || c.Menu.Opciones[0].Respuesta != "u" {
		t.Fatalf("%+v", c)
	}
}

func TestSaltarUnaSeccionNoCreaNiCambiaNada(t *testing.T) {
	a := abre(t)
	_ = a.FijarAjusteBot(1, "sil_activo", "0") // el usuario ya lo había apagado
	b := Nuevo()
	b.Saltados = []int{PasoBienvenida, PasoMenu, PasoReglas, PasoUtilidades}
	b.Reglas.AlEntrar = true // da igual: está saltado
	b.DesdeAjustes(a, 1)
	if b.Util.Silenciar {
		t.Fatal("DesdeAjustes debe leer lo de hoy")
	}
	b.Util.Silenciar = true
	if p := b.Problemas(); len(p) != 0 {
		t.Fatal(p)
	}
	c, err := Aplicar(a, 1, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Autos) != 0 || c.Menu != 0 || c.Modulo || len(c.Ajustes) != 0 || a.AjusteBot(1, "sil_activo") != "0" {
		t.Fatalf("saltado no debe tocar nada: %+v", c)
	}
}

func TestNuevoBaseTieneTresOpcionesYSePuedeAplicar(t *testing.T) {
	b := NuevoBase()
	if len(b.Menu.Opciones) != 3 || b.Menu.Opciones[0].Texto != "GitHub" || b.Menu.Opciones[1].Texto != "Menú" || b.Menu.Opciones[2].Texto != "Ayuda" {
		t.Fatalf("%+v", b.Menu.Opciones)
	}
	if !strings.Contains(b.Menu.Opciones[0].Respuesta, RepoBotBase) {
		t.Fatal("el botón GitHub no lleva al repositorio")
	}
	b.Normalizar()
	if p := b.Problemas(); len(p) > 0 {
		t.Fatalf("el bot base debe estar listo para usar: %v", p)
	}
}
