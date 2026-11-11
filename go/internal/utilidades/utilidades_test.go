package utilidades

import (
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/permisos"
)

const (
	pnAna  = "5215511111111@s.whatsapp.net"
	pnLuis = "5215522222222@s.whatsapp.net"
	grupo  = "grupo@g.us"
	propio = "5215500000001@s.whatsapp.net"
)

type entorno struct {
	alm  *almacen.Almacen
	perm *permisos.Servicio
	d    *comandos.Despachador
	s    *Servicio
	hora time.Time
	ids  map[string]int64
}

func nuevoEntorno(t *testing.T) *entorno {
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
	p, err := permisos.Nuevo(alm, 1)
	if err != nil {
		t.Fatal(err)
	}
	e := &entorno{alm: alm, perm: p, hora: time.Now(), ids: map[string]int64{}}
	p.Ahora = func() time.Time { return e.hora }
	e.d = comandos.Nuevo(alm, 1, p)
	e.d.Ahora = p.Ahora
	e.s = Nuevo(alm, 1)
	e.s.Ahora = p.Ahora
	e.s.RegistrarComandos(e.d)
	return e
}

func (e *entorno) rol(nombre string, cas ...string) {
	m := map[string]almacen.Alcance{}
	for _, c := range cas {
		m[c] = almacen.Alcance{}
	}
	id, err := e.perm.GuardarRol(almacen.Rol{Nombre: nombre, Casillas: m, LimiteMin: 100})
	if err != nil {
		panic(err)
	}
	e.ids[nombre] = id
}

func (e *entorno) da(persona, rol string) {
	if err := e.alm.AsignarRol(1, persona, e.ids[rol], 0, "yo"); err != nil {
		panic(err)
	}
}

func (e *entorno) priv(quien, texto string) comandos.Resultado {
	return e.d.Ejecutar(comandos.Entrada{Persona: quien, Chat: quien, Texto: texto, MensajeID: "M"})
}

func (e *entorno) dueno(texto string) string {
	return e.d.Ejecutar(comandos.Entrada{Persona: almacen.PersonaDueno, Chat: propio, Texto: texto, Dueno: true, MensajeID: "D"}).Texto
}

func quiere(t *testing.T, got, parte string) {
	t.Helper()
	if !strings.Contains(got, parte) {
		t.Fatalf("esperaba %q en %q", parte, got)
	}
}

func TestNotasDelDuenoDesdeMensajesGuardados(t *testing.T) {
	e := nuevoEntorno(t)
	quiere(t, e.dueno("!notas"), "No tienes notas")
	quiere(t, e.dueno("!anotar comprar café"), "número 1")
	quiere(t, e.dueno("!apuntar llamar a mamá"), "número 2")
	l := e.dueno("!notas")
	quiere(t, l, "1. comprar café")
	quiere(t, l, "2. llamar a mamá")
	quiere(t, e.dueno("!borrarnota 1"), "Nota borrada")
	l = e.dueno("!notas")
	if strings.Contains(l, "café") || !strings.Contains(l, "1. llamar a mamá") {
		t.Fatalf("%q", l)
	}
	quiere(t, e.dueno("!borrarnota 9"), "no está en la lista")
	quiere(t, e.dueno("!borrarnota"), "Dime el número")
	quiere(t, e.dueno("!anotar"), "Escribe el texto")
}

func TestPendientesHechoDeshacerYLimpiar(t *testing.T) {
	e := nuevoEntorno(t)
	e.dueno("!tarea pagar luz")
	e.dueno("!tarea enviar factura")
	e.dueno("!tarea pedir cotización")
	quiere(t, e.dueno("!hecho 2"), "hecho")
	l := e.dueno("!tareas")
	quiere(t, l, "1. [ ] pagar luz")
	quiere(t, l, "2. [x] enviar factura")
	quiere(t, l, "2 por hacer, 1 hechos")
	quiere(t, e.dueno("!deshacer 2"), "por hacer")
	e.dueno("!listo 1")
	e.dueno("!hecho 3")
	quiere(t, e.dueno("!limpiartareas"), "borré 2")
	l = e.dueno("!pendientes")
	if strings.Contains(l, "luz") || !strings.Contains(l, "1. [ ] enviar factura") {
		t.Fatalf("%q", l)
	}
	quiere(t, e.dueno("!limpiartareas"), "No había")
	quiere(t, e.dueno("!borrartarea 1"), "Pendiente borrado")
	quiere(t, e.dueno("!tareas"), "No tienes pendientes")
	// las notas y los pendientes son listas distintas
	e.dueno("!anotar algo")
	quiere(t, e.dueno("!tareas"), "No tienes pendientes")
}

func TestOtrosRangosSoloSuPrivadoYSusPermisos(t *testing.T) {
	e := nuevoEntorno(t)
	e.rol("Auxiliar", "util.notas")
	e.da(pnAna, "Auxiliar")
	e.da(pnLuis, "Auxiliar")
	quiere(t, e.priv(pnAna, "!anotar secreto de Ana").Texto, "número 1")
	// Luis no ve lo de Ana ni al revés; el dueño tampoco lo mezcla
	quiere(t, e.priv(pnLuis, "!notas").Texto, "No tienes notas")
	quiere(t, e.priv(pnAna, "!notas").Texto, "secreto de Ana")
	quiere(t, e.dueno("!notas"), "No tienes notas")
	// no tiene la casilla de pendientes
	quiere(t, e.priv(pnAna, "!tarea algo").Texto, "No tienes permiso")
	// en un grupo no funcionan (la respuesta la vería todo el grupo)
	r := e.d.Ejecutar(comandos.Entrada{Persona: pnAna, Chat: grupo, EsGrupo: true, Texto: "!notas", MensajeID: "G"})
	quiere(t, r.Texto, "solo funciona en un chat privado")
	// una persona sin rango no tiene nada de esto
	quiere(t, e.priv("5215599999999@s.whatsapp.net", "!notas").Texto, "No tienes permiso")
}

func TestTopeYLargoMensajesClaros(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.alm.FijarAjusteBot(1, "util_tope", "2")
	e.dueno("!tarea a")
	e.dueno("!tarea b")
	quiere(t, e.dueno("!tarea c"), "La lista está llena")
	quiere(t, e.dueno("!anotar "+strings.Repeat("x", 600)), "muy largo")
}

func TestComandosEnLaAyudaYCasillasDelCatalogo(t *testing.T) {
	e := nuevoEntorno(t)
	for _, c := range []string{"util.notas", "util.pendientes"} {
		ok := false
		for _, x := range permisos.Catalogo() {
			if x.ID == c {
				ok = true
			}
		}
		if !ok {
			t.Fatalf("falta la casilla %s", c)
		}
	}
	a := e.dueno("!ayuda")
	for _, n := range []string{"anotar", "notas", "tarea", "tareas", "hecho"} {
		quiere(t, a, n)
	}
}
