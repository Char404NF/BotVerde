package wabot

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/permisos"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func abreSolo(t *testing.T) (*obsFalso, *almacen.Almacen) {
	t.Helper()
	reinicia(t)
	obs := &obsFalso{}
	if err := AbrirAlmacen(t.TempDir(), clavePrueba(), obs); err != nil {
		t.Fatal(err)
	}
	a, _ := almacenAbierto()
	return obs, a
}

func decodifica[T any](t *testing.T, f func() (string, error)) T {
	t.Helper()
	j, err := f()
	if err != nil {
		t.Fatal(err)
	}
	var v T
	if e := json.Unmarshal([]byte(j), &v); e != nil {
		t.Fatalf("%v: %s", e, j)
	}
	return v
}

func TestApiRangosDeExtremoAExtremo(t *testing.T) {
	obs, _ := abreSolo(t)
	const ana = "5215511111111@s.whatsapp.net"
	cat := decodifica[[]permisos.Casilla](t, func() (string, error) { return CatalogoPermisos(), nil })
	if len(cat) < 35 {
		t.Fatal("catálogo")
	}
	roles := decodifica[[]almacen.Rol](t, func() (string, error) { return ListarRangos(1) })
	if len(roles) != 3 || !roles[0].EsDueno || roles[1].Nombre != "Ayudante" || roles[2].Nombre != "Cliente VIP" {
		t.Fatalf("%+v", roles)
	}
	// Crear con alcance por casilla, menús visibles y límite.
	id, err := GuardarRango(1, `{"id":0,"nombre":"Gerente","color":"#336699","limiteMin":15,"menusModo":"lista","menus":["2"],
		"casillas":{"personas.silenciar":{"modo":"privado"},"chat.pausar":{"modo":"chats","chats":["x@g.us"]},"util.notas":{}}}`)
	if err != nil || id == 0 {
		t.Fatalf("%d %v", id, err)
	}
	g := decodifica[*almacen.Rol](t, func() (string, error) { return ObtenerRango(1, id) })
	if g.Casillas["chat.pausar"].Chats[0] != "x@g.us" || g.LimiteMin != 15 || g.MenusModo != "lista" {
		t.Fatalf("%+v", g)
	}
	// Errores claros.
	if _, err := GuardarRango(1, `{"nombre":"Malo","casillas":{"inventada":{}}}`); err == nil || !strings.Contains(err.Error(), "no existen") {
		t.Fatalf("%v", err)
	}
	if _, err := GuardarRango(1, `{"nombre":"gerente"}`); err == nil {
		t.Fatal("nombre repetido")
	}
	if _, err := GuardarRango(1, `{"id":`+string(rune('0'+roles[0].ID))+`,"nombre":"X"}`); err == nil {
		t.Fatal("editar el Dueño")
	}
	// Reordenar.
	if err := ReordenarRangos(1, `[`+itoa(id)+`,`+itoa(roles[1].ID)+`,`+itoa(roles[2].ID)+`]`); err != nil {
		t.Fatal(err)
	}
	roles = decodifica[[]almacen.Rol](t, func() (string, error) { return ListarRangos(1) })
	if roles[1].ID != id {
		t.Fatalf("orden %+v", roles)
	}
	// Asignar con caducidad, ver, revocar.
	if err := AsignarRango(1, ana, id, "7d"); err != nil {
		t.Fatal(err)
	}
	if err := AsignarRango(1, ana, id, "mañana"); err == nil {
		t.Fatal("duración inválida")
	}
	if err := AsignarRango(1, "yo", id, "perm"); err == nil {
		t.Fatal("al dueño no")
	}
	rp := decodifica[rangoDePersona](t, func() (string, error) { return RangoDePersona(1, ana) })
	if rp.Rol == nil || rp.Rol.Nombre != "Gerente" || rp.Asignacion.CaducaEn < time.Now().Unix()+6*86400 {
		t.Fatalf("%+v", rp)
	}
	asig := decodifica[[]almacen.Asignacion](t, func() (string, error) { return ListarAsignaciones(1) })
	if len(asig) != 1 {
		t.Fatal("asignaciones")
	}
	if err := RevocarRango(1, ana); err != nil {
		t.Fatal(err)
	}
	rp = decodifica[rangoDePersona](t, func() (string, error) { return RangoDePersona(1, ana) })
	if rp.Rol != nil {
		t.Fatal("revocado")
	}
	// Borrar el rango.
	_ = AsignarRango(1, ana, id, "perm")
	if n, err := BorrarRango(1, id); err != nil || n != 1 {
		t.Fatalf("%d %v", n, err)
	}
	if _, err := BorrarRango(1, roles[0].ID); err == nil {
		t.Fatal("el Dueño no se borra")
	}
	// Eventos para refrescar la pantalla.
	time.Sleep(400 * time.Millisecond)
	obs.mu.Lock()
	todo := strings.Join(obs.lotes, ",")
	obs.mu.Unlock()
	for _, ev := range []string{`"rangos"`, `"rango:` + ana + `"`} {
		if !strings.Contains(todo, ev) {
			t.Fatalf("falta el evento %s en %s", ev, todo)
		}
	}
	// Registro de acciones (sin textos).
	acc := decodifica[[]almacen.Accion](t, func() (string, error) { return ListarAcciones(1, 50, 0) })
	if len(acc) < 5 {
		t.Fatalf("registro: %+v", acc)
	}
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestApiSolicitudesPoliticasYAjustes(t *testing.T) {
	_, a := abreSolo(t)
	const ana = "5215511111111@s.whatsapp.net"
	if err := FijarAjusteDePermisos(1, "desconocidos", "aprobar"); err != nil {
		t.Fatal(err)
	}
	for _, malo := range [][2]string{{"desconocidos", "x"}, {"prefijo", "ab"}, {"prefijo", " "}, {"prefijo", "a"}, {"prefijo", "!!!!"}, {"tope_pendientes", "0"},
		{"tope_pendientes", "x"}, {"confirm_seg", "1"}, {"grupo_modo_defecto", "nadie"}, {"nada", "1"}} {
		if FijarAjusteDePermisos(1, malo[0], malo[1]) == nil {
			t.Fatalf("debía rechazar %v", malo)
		}
	}
	if err := FijarAjusteDePermisos(1, "prefijo", "#"); err != nil {
		t.Fatal(err)
	}
	aj := decodifica[map[string]string](t, func() (string, error) { return AjustesDePermisos(1) })
	if aj["prefijo"] != "#" || aj["desconocidos"] != "aprobar" || aj["tope_pendientes"] != "50" {
		t.Fatalf("%v", aj)
	}
	if _, ok := aj["desconocidos_desde"]; ok {
		t.Fatal("no se expone el ajuste interno")
	}
	// Solicitudes.
	_, _ = a.RegistrarSolicitud(1, ana, "Ana", 10, 7, time.Now())
	_, _ = a.RegistrarSolicitud(1, "5215522222222@s.whatsapp.net", "Luis", 10, 7, time.Now())
	if n, _ := ContarSolicitudes(1); n != 2 {
		t.Fatalf("pendientes %d", n)
	}
	l := decodifica[[]almacen.Solicitud](t, func() (string, error) { return ListarSolicitudes(1, "") })
	if len(l) != 2 || l[0].Nombre == "" {
		t.Fatalf("%+v", l)
	}
	if err := AprobarSolicitud(1, ana); err != nil {
		t.Fatal(err)
	}
	if err := BloquearSolicitud(1, "5215522222222@s.whatsapp.net"); err != nil {
		t.Fatal(err)
	}
	if n, _ := ContarSolicitudes(1); n != 0 {
		t.Fatal("ya no hay pendientes")
	}
	rp := decodifica[rangoDePersona](t, func() (string, error) { return RangoDePersona(1, "5215522222222@s.whatsapp.net") })
	if !rp.Bloqueada {
		t.Fatal("bloqueada")
	}
	if err := IgnorarSolicitud(1, ana); err != nil {
		t.Fatal(err)
	}
	if ig := decodifica[[]almacen.Solicitud](t, func() (string, error) { return ListarSolicitudes(1, "ignorada") }); len(ig) != 1 {
		t.Fatal("ignoradas")
	}
	if err := QuitarSolicitud(1, ana); err != nil {
		t.Fatal(err)
	}
	// Política de grupos.
	if err := FijarPoliticaDeGrupo(1, `{"chat":"g@g.us","modo":"lista","lista":["`+ana+`"]}`); err != nil {
		t.Fatal(err)
	}
	if FijarPoliticaDeGrupo(1, `{"chat":"g@g.us","modo":"raro"}`) == nil || FijarPoliticaDeGrupo(1, `{"modo":"todos"}`) == nil {
		t.Fatal("validación")
	}
	p := decodifica[almacen.PoliticaGrupo](t, func() (string, error) { return PoliticaDeGrupo(1, "g@g.us") })
	if p.Modo != "lista" || len(p.Lista) != 1 {
		t.Fatalf("%+v", p)
	}
	if ls := decodifica[[]almacen.PoliticaGrupo](t, func() (string, error) { return ListarPoliticasDeGrupo(1) }); len(ls) != 1 {
		t.Fatal("lista de políticas")
	}
}

func TestApiComandosYConfirmaciones(t *testing.T) {
	_, a := abreSolo(t)
	r := decodifica[resultadoComando](t, func() (string, error) { return EjecutarComando(1, "!ayuda") })
	if !r.Manejado || !strings.Contains(r.Texto, "!asignar") {
		t.Fatalf("%+v", r)
	}
	r = decodifica[resultadoComando](t, func() (string, error) { return EjecutarComando(1, "hola") })
	if r.Manejado {
		t.Fatal("no es comando")
	}
	// Confirmaciones: pendiente → listar → confirmar desde la app (un solo uso) / cancelar.
	c, _ := a.CrearConfirmacion(1, "yo", "asignar", []string{"5215500000000", "Ayudante"}, "", "", time.Minute, time.Now())
	l := decodifica[[]almacen.Confirmacion](t, func() (string, error) { return ConfirmacionesPendientes(1) })
	if len(l) != 1 || l[0].Codigo != c.Codigo {
		t.Fatalf("%+v", l)
	}
	txt, err := ConfirmarCodigo(1, c.Codigo)
	if err != nil || !strings.Contains(txt, "Listo") {
		t.Fatalf("%q %v", txt, err)
	}
	if _, err := ConfirmarCodigo(1, c.Codigo); err == nil {
		t.Fatal("un solo uso")
	}
	c2, _ := a.CrearConfirmacion(1, "yo", "asignar", nil, "", "", time.Minute, time.Now())
	if err := CancelarConfirmacion(1, c2.Codigo); err != nil {
		t.Fatal(err)
	}
	if _, err := ConfirmarCodigo(1, c2.Codigo); err == nil {
		t.Fatal("cancelado")
	}
	if l := decodifica[[]almacen.Confirmacion](t, func() (string, error) { return ConfirmacionesPendientes(1) }); len(l) != 0 {
		t.Fatal("no quedan pendientes")
	}
}

// Con el bot iniciado: los eventos de WhatsApp pasan por la compuerta, aprenden alias y llegan los comandos del dueño.
func TestBotRangosDesconocidosAliasYMensajesGuardados(t *testing.T) {
	reinicia(t)
	ConfigurarMotor(true, 10)
	f := &falso{}
	raiz := t.TempDir()
	if err := AbrirAlmacen(raiz, clavePrueba(), nil); err != nil {
		t.Fatal(err)
	}
	if err := Start(raiz, 1, clavePrueba(), f); err != nil {
		t.Fatal(err)
	}
	defer Stop()
	mu.Lock()
	s := st
	mu.Unlock()
	yo := types.NewJID("5215500000000", types.DefaultUserServer)
	yoLID := types.NewJID("777000111", types.HiddenUserServer)
	s.cli.Store.ID = &yo
	s.cli.Store.LID = yoLID
	if err := FijarAjusteDePermisos(1, "desconocidos", "aprobar"); err != nil {
		t.Fatal(err)
	}
	lid := types.NewJID("99887766554433", types.HiddenUserServer)
	pn := types.NewJID("5215511111111", types.DefaultUserServer)
	texto := func(id string, chat, sender, alt types.JID, fromMe bool, t string) *events.Message {
		e := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chat, Sender: sender, SenderAlt: alt, IsFromMe: fromMe}, ID: id, Timestamp: time.Now(), PushName: "Pedro"},
			RawMessage: &waE2E.Message{Conversation: proto.String(t)}}
		e.UnwrapRaw()
		return e
	}
	// Un desconocido escribe desde su LID, pero el mensaje trae su número: se aprende el alias y queda UNA solicitud.
	s.evento(texto("A1", lid, lid, pn, false, "hola"))
	var sols []almacen.Solicitud
	for i := 0; i < 200; i++ {
		sols, _ = alm.ListarSolicitudes(1, "", 10)
		if len(sols) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(sols) != 1 || sols[0].Nombre != "Pedro" {
		t.Fatalf("solicitudes: %+v", sols)
	}
	if alm.ResolverPersona(1, lid.String()) != pn.String() {
		t.Fatalf("alias no aprendido: %v", alm.AliasDe(1, lid.String()))
	}
	// Ahora llega por número: es la misma persona, no se duplica la solicitud.
	s.evento(texto("A2", pn, pn, types.EmptyJID, false, "sigo aquí"))
	time.Sleep(300 * time.Millisecond)
	if sols, _ = alm.ListarSolicitudes(1, "", 10); len(sols) != 1 || sols[0].Mensajes < 2 {
		t.Fatalf("misma persona por número y LID: %+v", sols)
	}
	// Un comando mío en "Mensajes guardados" (por mi LID) llega al despachador como comando de Dueño.
	s.evento(texto("D1", yoLID, yoLID, types.EmptyJID, true, "!rango"))
	var ok bool
	for i := 0; i < 200 && !ok; i++ {
		acc, _ := alm.ListarAcciones(1, 50, 0)
		for _, a := range acc {
			if a.Accion == "cmd.rango" && a.Quien == almacen.PersonaDueno && a.Resultado == "ok" {
				ok = true
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ok {
		t.Fatal("el comando del dueño no llegó al despachador")
	}
	// Lo mismo escrito en el chat de un cliente NO es un comando de dueño.
	antes, _ := alm.ListarAcciones(1, 100, 0)
	s.evento(texto("D2", pn, yo, types.EmptyJID, true, "!rango"))
	time.Sleep(400 * time.Millisecond)
	despues, _ := alm.ListarAcciones(1, 100, 0)
	if len(despues) != len(antes) {
		t.Fatal("un comando mío en el chat de un cliente no se ejecuta")
	}
	// Y no pausa ni cancela nada: escribir en Mensajes guardados no es "intervenir" en un cliente.
	if est := alm.EstadoDeChat(1, yoLID.String()); est.BotPausaHasta != 0 {
		t.Fatal("no debía pausar")
	}
}
