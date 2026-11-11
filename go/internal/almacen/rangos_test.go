package almacen

import (
	"strings"
	"testing"
	"time"
)

const (
	pnAna  = "5215511111111@s.whatsapp.net"
	lidAna = "99887766554433@lid"
	pnLuis = "5215522222222@s.whatsapp.net"
)

func conRoles(t *testing.T) *Almacen {
	t.Helper()
	a := nuevo(t)
	if err := a.SembrarRoles(1, []Rol{{Nombre: "Ayudante", Plantilla: "ayudante"}, {Nombre: "Cliente VIP", Plantilla: "cliente_vip"}}); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestMigracion10Existe(t *testing.T) {
	a := nuevo(t)
	if a.Version() < 10 {
		t.Fatalf("falta la migración 10: versión %d", a.Version())
	}
	for _, tabla := range []string{"roles", "persona_rol", "persona_alias", "registro_acciones", "solicitudes", "codigos_confirmacion", "grupos_politica", "bot_ajustes"} {
		var n int
		if err := a.db.QueryRow(`SELECT COUNT(*) FROM ` + tabla).Scan(&n); err != nil {
			t.Fatalf("tabla %s: %v", tabla, err)
		}
	}
}

func TestDuenoFijoYPlantillas(t *testing.T) {
	a := conRoles(t)
	roles, _ := a.ListarRoles(1)
	if len(roles) != 3 || !roles[0].EsDueno || roles[0].Nombre != "Dueño" || roles[0].Posicion != 0 {
		t.Fatalf("roles: %+v", roles)
	}
	// El Dueño no se edita ni se borra ni se da.
	if err := a.GuardarRol(1, Rol{ID: roles[0].ID, Nombre: "X"}); err != ErrRolDueno {
		t.Fatalf("editar dueño: %v", err)
	}
	if _, err := a.BorrarRol(1, roles[0].ID); err != ErrRolDueno {
		t.Fatalf("borrar dueño: %v", err)
	}
	if err := a.AsignarRol(1, pnAna, roles[0].ID, 0, "yo"); err != ErrRolDueno {
		t.Fatalf("asignar dueño: %v", err)
	}
	// Sembrar de nuevo no repite; borrar una plantilla no la hace volver.
	if _, err := a.BorrarRol(1, roles[1].ID); err != nil {
		t.Fatal(err)
	}
	_ = a.SembrarRoles(1, []Rol{{Nombre: "Ayudante"}})
	roles, _ = a.ListarRoles(1)
	if len(roles) != 2 {
		t.Fatalf("la plantilla borrada volvió: %+v", roles)
	}
	if roles[1].Posicion != 1 {
		t.Fatalf("posiciones sin renumerar: %+v", roles)
	}
	// Nombre repetido.
	if _, err := a.CrearRol(1, Rol{Nombre: "Gerente de turno"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CrearRol(1, Rol{Nombre: "GERENTE DE TURNO"}); err != ErrNombreRepetido {
		t.Fatalf("repetido: %v", err)
	}
}

func TestReordenarRoles(t *testing.T) {
	a := conRoles(t)
	roles, _ := a.ListarRoles(1)
	ay, vip := roles[1].ID, roles[2].ID
	if err := a.ReordenarRoles(1, []int64{vip, ay}); err != nil {
		t.Fatal(err)
	}
	roles, _ = a.ListarRoles(1)
	if roles[0].EsDueno != true || roles[1].ID != vip || roles[2].ID != ay {
		t.Fatalf("orden: %+v", roles)
	}
	if err := a.ReordenarRoles(1, []int64{vip}); err == nil {
		t.Fatal("debe traer todos")
	}
	if err := a.ReordenarRoles(1, []int64{vip, vip}); err == nil {
		t.Fatal("repetido")
	}
}

func TestCaducidadYRevocacion(t *testing.T) {
	a := conRoles(t)
	roles, _ := a.ListarRoles(1)
	ay := roles[1].ID
	ahora := time.Now()
	if err := a.AsignarRol(1, pnAna, ay, ahora.Add(24*time.Hour).Unix(), "yo"); err != nil {
		t.Fatal(err)
	}
	if r, _, _ := a.RolDePersona(1, pnAna, ahora); r == nil || r.ID != ay {
		t.Fatalf("debe tener rango: %+v", r)
	}
	// 25 horas después ya caducó y se limpia sola.
	if r, _, _ := a.RolDePersona(1, pnAna, ahora.Add(25*time.Hour)); r != nil {
		t.Fatalf("debió caducar: %+v", r)
	}
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM persona_rol`).Scan(&n)
	if n != 0 {
		t.Fatal("no se limpió la fila caducada")
	}
	// Limpieza masiva.
	_ = a.AsignarRol(1, pnAna, ay, ahora.Add(time.Hour).Unix(), "yo")
	_ = a.AsignarRol(1, pnLuis, ay, 0, "yo")
	if c := a.LimpiarCaducados(1, ahora.Add(2*time.Hour)); c != 1 {
		t.Fatalf("limpiados %d", c)
	}
	if r, _, _ := a.RolDePersona(1, pnLuis, ahora.Add(1000*time.Hour)); r == nil {
		t.Fatal("el permanente no caduca")
	}
	// Revocar.
	if hubo, _ := a.QuitarRol(1, pnLuis); !hubo {
		t.Fatal("debía revocar")
	}
	if r, _, _ := a.RolDePersona(1, pnLuis, ahora); r != nil {
		t.Fatal("seguía con rango")
	}
	// Borrar un rango deja sin rango a sus personas.
	_ = a.AsignarRol(1, pnLuis, ay, 0, "yo")
	if n, _ := a.BorrarRol(1, ay); n != 1 {
		t.Fatalf("afectadas %d", n)
	}
	if r, _, _ := a.RolDePersona(1, pnLuis, ahora); r != nil {
		t.Fatal("seguía con rango tras borrar el rol")
	}
	// El dueño siempre es Dueño.
	if r, _, _ := a.RolDePersona(1, PersonaDueno, ahora); r == nil || !r.EsDueno {
		t.Fatal("yo debe ser Dueño")
	}
}

func TestAliasNumeroYLID(t *testing.T) {
	a := conRoles(t)
	roles, _ := a.ListarRoles(1)
	ay, vip := roles[1].ID, roles[2].ID
	// Primero la conocemos solo por LID y le damos un rango.
	if err := a.AsignarRol(1, lidAna, ay, 0, "yo"); err != nil {
		t.Fatal(err)
	}
	if r, _, _ := a.RolDePersona(1, lidAna, time.Now()); r == nil {
		t.Fatal("por LID")
	}
	if r, _, _ := a.RolDePersona(1, pnAna, time.Now()); r != nil {
		t.Fatal("todavía no se sabe que son la misma")
	}
	// Después el LID aprende su número: el rango se une al del número.
	a.AprenderAlias(1, pnAna, lidAna)
	for _, p := range []string{pnAna, lidAna} {
		r, as, _ := a.RolDePersona(1, p, time.Now())
		if r == nil || r.ID != ay {
			t.Fatalf("%s sin rango tras unir", p)
		}
		if as.Persona != pnAna {
			t.Fatalf("la clave canónica debe ser el número: %s", as.Persona)
		}
	}
	if a.ResolverPersona(1, lidAna) != pnAna || a.ResolverPersona(1, pnAna) != pnAna {
		t.Fatal("resolver")
	}
	// El orden de los argumentos no importa y repetir no rompe nada.
	a.AprenderAlias(1, lidAna, pnAna)
	if got := a.AliasDe(1, lidAna); len(got) != 2 {
		t.Fatalf("alias: %v", got)
	}
	// Si ambos tenían rango distinto gana el más alto.
	a2 := conRoles(t)
	r2, _ := a2.ListarRoles(1)
	_ = a2.AsignarRol(1, pnAna, r2[2].ID, 0, "yo")  // VIP (más bajo)
	_ = a2.AsignarRol(1, lidAna, r2[1].ID, 0, "yo") // Ayudante (más alto)
	a2.AprenderAlias(1, pnAna, lidAna)
	if r, _, _ := a2.RolDePersona(1, pnAna, time.Now()); r == nil || r.ID != r2[1].ID {
		t.Fatalf("debía ganar el más alto: %+v", r)
	}
	_ = vip
	// Asignar por LID cuando ya se conoce el número guarda en el canónico.
	_ = a2.AsignarRol(1, lidAna, r2[2].ID, 0, "yo")
	var n int
	_ = a2.db.QueryRow(`SELECT COUNT(*) FROM persona_rol`).Scan(&n)
	if n != 1 {
		t.Fatalf("filas %d", n)
	}
	// Un par que no es número+LID se ignora.
	a2.AprenderAlias(1, pnAna, pnLuis)
	if a2.ResolverPersona(1, pnLuis) != pnLuis {
		t.Fatal("no debía unir dos números")
	}
}

func TestSolicitudesTopeIgnoradaYDecisiones(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	r, err := a.RegistrarSolicitud(1, pnAna, "Ana", 2, 7, ahora)
	if err != nil || !r.Nueva || r.Solicitud.Nombre != "Ana" {
		t.Fatalf("%+v %v", r, err)
	}
	// otro mensaje de la misma: no es nueva, cuenta
	r, _ = a.RegistrarSolicitud(1, pnAna, "Ana", 2, 7, ahora.Add(time.Second))
	if r.Nueva || r.Solicitud.Mensajes != 2 {
		t.Fatalf("%+v", r)
	}
	if r, _ = a.RegistrarSolicitud(1, pnLuis, "Luis", 2, 7, ahora); !r.Nueva {
		t.Fatal("segunda")
	}
	// tope: la tercera no se guarda
	r, _ = a.RegistrarSolicitud(1, "5215533333333@s.whatsapp.net", "", 2, 7, ahora)
	if !r.Descartada || a.ContarSolicitudes(1, SolPendiente) != 2 {
		t.Fatalf("tope: %+v", r)
	}
	// ignorar no es bloqueo: dentro de 7 días sigue ignorada, después vuelve a pendiente
	if err := a.DecidirSolicitud(1, pnAna, SolIgnorada, ahora); err != nil {
		t.Fatal(err)
	}
	r, _ = a.RegistrarSolicitud(1, pnAna, "Ana", 2, 7, ahora.Add(24*time.Hour))
	if r.Solicitud.Estado != SolIgnorada || r.Nueva {
		t.Fatalf("debía seguir ignorada: %+v", r)
	}
	r, _ = a.RegistrarSolicitud(1, pnAna, "Ana", 2, 7, ahora.Add(8*24*time.Hour))
	if r.Solicitud.Estado != SolPendiente || !r.Nueva {
		t.Fatalf("debía reabrirse: %+v", r)
	}
	// aprobar / bloquear
	_ = a.DecidirSolicitud(1, pnLuis, SolBloqueada, ahora)
	if s, _ := a.ObtenerSolicitud(1, pnLuis); s.Estado != SolBloqueada {
		t.Fatal("bloqueada")
	}
	if err := a.DecidirSolicitud(1, PersonaDueno, SolBloqueada, ahora); err == nil {
		t.Fatal("no se bloquea al dueño")
	}
	// el nombre se guarda cifrado
	var b []byte
	_ = a.db.QueryRow(`SELECT nombre_cifrado FROM solicitudes WHERE persona=?`, pnAna).Scan(&b)
	if len(b) == 0 || strings.Contains(string(b), "Ana") {
		t.Fatal("el nombre debe ir cifrado")
	}
	// listar por estado
	l, _ := a.ListarSolicitudes(1, SolBloqueada, 10)
	if len(l) != 1 || l[0].Persona != pnLuis {
		t.Fatalf("lista: %+v", l)
	}
}

func TestConfirmacionesUnSoloUso(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	c, err := a.CrearConfirmacion(1, pnAna, "expulsar", []string{"5215500000000"}, "grupo@g.us", "M1", 2*time.Minute, ahora)
	if err != nil || len(c.Codigo) != 6 {
		t.Fatalf("%+v %v", c, err)
	}
	// otra persona no puede gastarlo (y no lo quema)
	if _, err := a.ConsumirConfirmacion(1, c.Codigo, pnLuis, ahora); err == nil {
		t.Fatal("otra persona no puede")
	}
	// minúsculas y espacios valen
	got, err := a.ConsumirConfirmacion(1, " "+strings.ToLower(c.Codigo)+" ", pnAna, ahora.Add(time.Second))
	if err != nil || got.Comando != "expulsar" || len(got.Args) != 1 || got.Chat != "grupo@g.us" {
		t.Fatalf("%+v %v", got, err)
	}
	// un solo uso
	if _, err := a.ConsumirConfirmacion(1, c.Codigo, pnAna, ahora.Add(2*time.Second)); err == nil {
		t.Fatal("el código se usó dos veces")
	}
	// caduca
	c2, _ := a.CrearConfirmacion(1, pnAna, "expulsar", nil, "", "", time.Minute, ahora)
	if _, err := a.ConsumirConfirmacion(1, c2.Codigo, "", ahora.Add(61*time.Second)); err == nil {
		t.Fatal("el código vencido sirvió")
	}
	// uno nuevo del mismo comando reemplaza al anterior
	c3, _ := a.CrearConfirmacion(1, pnAna, "borrar", nil, "", "", time.Minute, ahora)
	c4, _ := a.CrearConfirmacion(1, pnAna, "borrar", nil, "", "", time.Minute, ahora)
	if _, err := a.ConsumirConfirmacion(1, c3.Codigo, "", ahora); err == nil {
		t.Fatal("el reemplazado sirvió")
	}
	for _, p := range func() []Confirmacion { l, _ := a.ConfirmacionesPendientes(1, ahora); return l }() {
		if p.Codigo == c3.Codigo {
			t.Fatalf("el reemplazado sigue pendiente: %+v", p)
		}
	}
	// cancelar
	_ = a.CancelarConfirmacion(1, c4.Codigo)
	if _, err := a.ConsumirConfirmacion(1, c4.Codigo, "", ahora); err == nil {
		t.Fatal("cancelado sirvió")
	}
	// los argumentos van cifrados
	var b []byte
	_ = a.db.QueryRow(`SELECT args_cifrados FROM codigos_confirmacion WHERE codigo=?`, c.Codigo).Scan(&b)
	if strings.Contains(string(b), "5215500000000") {
		t.Fatal("los argumentos van cifrados")
	}
}

func TestPoliticaGrupoYAjustes(t *testing.T) {
	a := nuevo(t)
	if p := a.PoliticaDeGrupo(1, "g@g.us"); p.Modo != "todos" {
		t.Fatalf("por defecto: %+v", p)
	}
	_ = a.FijarPoliticaGrupo(1, PoliticaGrupo{Chat: "g@g.us", Modo: "lista", Lista: []string{lidAna}})
	a.AprenderAlias(1, pnAna, lidAna)
	p := a.PoliticaDeGrupo(1, "g@g.us")
	if p.Modo != "lista" || len(p.Lista) != 1 {
		t.Fatalf("%+v", p)
	}
	if err := a.FijarPoliticaGrupo(1, PoliticaGrupo{Chat: "g@g.us", Modo: "otro"}); err == nil {
		t.Fatal("modo inválido")
	}
	if a.AjusteBot(1, "prefijo") != "!" || a.AjusteBotInt(1, "tope_pendientes") != 50 {
		t.Fatal("valores por defecto")
	}
	_ = a.FijarAjusteBot(1, "prefijo", "#")
	if a.AjusteBot(1, "prefijo") != "#" {
		t.Fatal("prefijo")
	}
	if a.FijarAjusteBot(1, "no_existe", "x") == nil {
		t.Fatal("clave desconocida")
	}
}

func TestRegistroDeAccionesSinTextos(t *testing.T) {
	a := nuevo(t)
	a.RegistrarAccion(1, pnAna, "cmd.asignar", pnLuis, "chat", "ok", "rol=2")
	a.RegistrarAccion(1, pnAna, "cmd.asignar", pnLuis, "chat", "denegado", "jerarquia")
	l, _ := a.ListarAcciones(1, 10, 0)
	if len(l) != 2 || l[0].Resultado != "denegado" || l[1].Detalle != "rol=2" {
		t.Fatalf("%+v", l)
	}
	if l2, _ := a.ListarAcciones(1, 10, l[0].ID); len(l2) != 1 {
		t.Fatal("paginación")
	}
	a.LimpiarAcciones(1, 90, time.Now().Add(100*24*time.Hour))
	if l3, _ := a.ListarAcciones(1, 10, 0); len(l3) != 0 {
		t.Fatal("limpieza")
	}
}

func TestEliminarBotLimpiaRangos(t *testing.T) {
	a := conRoles(t)
	roles, _ := a.ListarRoles(1)
	_ = a.AsignarRol(1, pnAna, roles[1].ID, 0, "yo")
	a.RegistrarAccion(1, "yo", "x", "", "", "ok", "")
	if err := a.EliminarBot(1); err != nil {
		t.Fatal(err)
	}
	var n int
	for _, tabla := range []string{"roles", "persona_rol", "registro_acciones"} {
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM ` + tabla).Scan(&n)
		if n != 0 {
			t.Fatalf("%s quedó con %d", tabla, n)
		}
	}
}
