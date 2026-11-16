package permisos

import (
	"testing"
	"time"

	"botchat/internal/almacen"
)

const (
	grpU  = "120363000000000077@g.us"
	pnAdm = "5215599999991@s.whatsapp.net" // administra el grupo
	pnMie = "5215599999992@s.whatsapp.net" // miembro
	pnMi2 = "5215599999993@s.whatsapp.net" // miembro con rango dado a mano
)

func (e *entorno) grupoConMiembros(t *testing.T) {
	t.Helper()
	_, _ = e.alm.ActualizarMiembros(1, grpU, []almacen.MiembroGrupo{
		{Persona: pnAdm, JID: pnAdm, Rol: almacen.RolGrupoAdmin}, {Persona: pnMie, JID: pnMie, Rol: almacen.RolGrupoMiembro}, {Persona: pnMi2, JID: pnMi2, Rol: almacen.RolGrupoMiembro},
	}, e.hora)
}

func TestRolUsuarioSeCreaUnaVezYPuedePedirDescargas(t *testing.T) {
	e := nuevoEntorno(t)
	id := e.s.RolUsuarioID()
	if id <= 0 || e.s.RolUsuarioID() != id {
		t.Fatalf("debía crearse una sola vez: %d", id)
	}
	r, _ := e.alm.RolPorID(1, id)
	if r == nil || r.Nombre != "Usuario" || r.Plantilla != "usuario" || len(r.Casillas) != len(IDsCasillasUsuario) {
		t.Fatalf("%+v", r)
	}
	for _, c := range []string{"video.pedir", "util.sticker", "ia.usar", "juegos.jugar"} {
		if _, ok := r.Casillas[c]; !ok {
			t.Fatalf("el rango de todos debía traer %s: %v", c, r.Casillas)
		}
	}
	if _, ok := r.Casillas["grupos.banear"]; ok || e.s.CompletarUsuario() != 0 {
		t.Fatalf("no debe traer permisos de administración ni agregar de más: %v", r.Casillas)
	}
	// un bot que ya existía con el rango viejo (solo descargas) queda completo, sin quitar lo que el dueño ya tenía
	r.Casillas = map[string]almacen.Alcance{"video.pedir": {}, "util.notas": {}}
	if err := e.alm.GuardarRol(1, *r); err != nil {
		t.Fatal(err)
	}
	if n := e.s.CompletarUsuario(); n != len(IDsCasillasUsuario)-2 {
		t.Fatalf("debía agregar las que faltan: %d", n)
	}
	// si el dueño lo borra, no vuelve a aparecer solo
	if _, err := e.alm.BorrarRol(1, id); err != nil {
		t.Fatal(err)
	}
	if e.s.RolUsuarioID() != 0 {
		t.Fatal("un rango borrado no se recrea solo")
	}
}

func TestQuienEscribeRecibeUsuarioMenosAdminsYConRango(t *testing.T) {
	e := nuevoEntorno(t)
	e.grupoConMiembros(t)
	_ = e.alm.AsignarRol(1, pnMi2, e.ids["Ayudante"], 0, "yo")

	e.s.AsegurarUsuario(pnMie)
	e.s.AsegurarUsuario(pnAdm)
	e.s.AsegurarUsuario(pnMi2)
	e.s.AsegurarUsuario(pnLuis) // alguien que escribe en privado
	e.s.AsegurarUsuario(almacen.PersonaDueno)

	if r := e.s.RolDe(pnMie); r == nil || r.Nombre != "Usuario" {
		t.Fatalf("el miembro debía ser Usuario: %+v", r)
	}
	if r := e.s.RolDe(pnLuis); r == nil || r.Nombre != "Usuario" {
		t.Fatalf("quien escribe en privado también: %+v", r)
	}
	if r := e.s.RolDe(pnAdm); r != nil {
		t.Fatalf("un admin de grupo no es Usuario: %+v", r)
	}
	if r := e.s.RolDe(pnMi2); r == nil || r.Nombre != "Ayudante" {
		t.Fatalf("el rango dado a mano no se toca: %+v", r)
	}
	if r := e.s.RolDe(almacen.PersonaDueno); r == nil || r.Nombre != "Dueño" {
		t.Fatalf("el dueño sigue siendo Dueño: %+v", r)
	}
	// puede pedir descargas, pero nada más
	if !e.s.Puede(pnMie, "video.pedir", true, grpU) || e.s.Puede(pnMie, "grupos.expulsar", true, grpU) {
		t.Fatal("Usuario solo pide descargas")
	}
	// el bloqueado no recibe nada
	_, _ = e.alm.RegistrarSolicitud(1, pnRosa, "Rosa", 50, 7, e.hora)
	_ = e.alm.DecidirSolicitud(1, pnRosa, almacen.SolBloqueada, e.hora)
	e.s.AsegurarUsuario(pnRosa)
	if e.s.RolDe(pnRosa) != nil {
		t.Fatal("una persona bloqueada no recibe rango")
	}
}

func TestRepartirUsuariosYAdminsNuevos(t *testing.T) {
	e := nuevoEntorno(t)
	e.grupoConMiembros(t)
	if n := e.s.RepartirUsuarios(grpU); n != 2 {
		t.Fatalf("debían recibirlo los 2 miembros que no administran: %d", n)
	}
	if e.s.RolDe(pnAdm) != nil || e.s.RolDe(pnMie) == nil {
		t.Fatal("reparto incorrecto")
	}
	// un miembro se vuelve admin: pierde el rango automático
	_ = e.alm.RegistrarCambio(1, grpU, almacen.CambioPromovido, pnMie, pnMie, pnAdm, "", e.hora)
	e.s.QuitarUsuarioDeAdmin(pnMie)
	if e.s.RolDe(pnMie) != nil {
		t.Fatal("un admin ya no es Usuario")
	}
	// deja de ser admin: vuelve
	_ = e.alm.RegistrarCambio(1, grpU, almacen.CambioDegradado, pnMie, pnMie, pnAdm, "", e.hora)
	e.s.AsegurarUsuario(pnMie)
	if r := e.s.RolDe(pnMie); r == nil || r.Nombre != "Usuario" {
		t.Fatalf("debía volver a ser Usuario: %+v", r)
	}
}

func TestApagarElRangoAutomaticoQuitaSoloLosDadosSolos(t *testing.T) {
	e := nuevoEntorno(t)
	e.grupoConMiembros(t)
	_ = e.alm.AsignarRol(1, pnMi2, e.ids["Ayudante"], 0, "yo")
	e.s.RepartirUsuarios(grpU)
	if n, err := e.s.FijarUsuarioAuto(false); err != nil || n != 1 {
		t.Fatalf("debía quitar 1 (el miembro): %d %v", n, err)
	}
	if e.s.RolDe(pnMie) != nil || e.s.RolDe(pnMi2) == nil {
		t.Fatal("solo se quitan los automáticos")
	}
	e.s.AsegurarUsuario(pnLuis)
	if e.s.RolDe(pnLuis) != nil {
		t.Fatal("apagado no reparte")
	}
	if n, _ := e.s.FijarUsuarioAuto(true); n < 1 {
		t.Fatalf("al encender reparte de nuevo: %d", n)
	}
	if e.s.RolDe(pnMie) == nil {
		t.Fatal("debía volver a dárselo")
	}
}

func TestElRangoAutomaticoNoCuentaComoConocidoParaLaPoliticaDeDesconocidos(t *testing.T) {
	e := nuevoEntorno(t)
	e.grupoConMiembros(t)
	e.s.RepartirUsuarios(grpU)
	if e.s.RolDe(pnMie) == nil {
		t.Fatal("tenía que tener Usuario")
	}
	if e.s.EsConocido(pnMie, pnMie) {
		t.Fatal("estar en un grupo no lo vuelve conocido en privado")
	}
	if err := e.s.FijarPoliticaDesconocidos(DescAprobar); err != nil {
		t.Fatal(err)
	}
	sal := e.s.Compuerta(Entrada{Persona: pnMie, Chat: pnMie, Nombre: "Mie"})
	if sal.Pasa {
		t.Fatalf("con la política «aprobar», un Usuario automático sigue siendo desconocido: %+v", sal)
	}
	// con un rango dado a mano sí es conocido
	_ = e.alm.AsignarRol(1, pnMie, e.ids["Ayudante"], 0, "yo")
	if !e.s.EsConocido(pnMie, pnMie) || !e.s.Compuerta(Entrada{Persona: pnMie, Chat: pnMie}).Pasa {
		t.Fatal("con rango propio es conocido")
	}
	_ = time.Second
}

func TestUnRangoQuitadoAMano_SeVuelveAAutoAsignarEnElSiguienteMensaje(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.alm.AsignarRol(1, pnLuis, e.ids["Ayudante"], 0, "yo")
	e.s.AsegurarUsuario(pnLuis) // ya tiene rango: no se toca
	if r := e.s.RolDe(pnLuis); r == nil || r.Nombre != "Ayudante" {
		t.Fatalf("%+v", r)
	}
	_, _ = e.alm.QuitarRol(1, pnLuis)
	e.s.AsegurarUsuario(pnLuis)
	if r := e.s.RolDe(pnLuis); r == nil || r.Nombre != "Usuario" {
		t.Fatalf("sin rango propio debía quedar como Usuario enseguida: %+v", r)
	}
}
