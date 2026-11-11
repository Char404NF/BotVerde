package grupos

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/permisos"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

const (
	grupoJID = "120363000000000001@g.us"
	pnYo     = "5215500000001@s.whatsapp.net"
	pnAna    = "5215511111111@s.whatsapp.net" // administradora
	pnLuis   = "5215522222222@s.whatsapp.net"
	pnRosa   = "5215533333333@s.whatsapp.net"
	lidRosa  = "77665544332211@lid"
	pnCarla  = "5215544444444@s.whatsapp.net" // creó el grupo
	pnJefe   = "5215566666666@s.whatsapp.net" // rango alto que escribe comandos
)

func jid(s string) types.JID { j, _ := types.ParseJID(s); return j }

func part(pn string, admin, super bool) types.GroupParticipant {
	return types.GroupParticipant{JID: jid(pn), PhoneNumber: jid(pn), IsAdmin: admin || super, IsSuperAdmin: super}
}

type entorno struct {
	alm  *almacen.Almacen
	perm *permisos.Servicio
	d    *comandos.Despachador
	s    *Servicio
	cli  *ClienteFalso
	hora time.Time
	cod  []almacen.Confirmacion
	ids  map[string]int64
	borr []string
	env  []envio
}

type envio struct {
	chat, texto, cita string
	menciones         []string
}

func nuevo(t *testing.T, soyAdmin bool) *entorno {
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
	e := &entorno{alm: alm, perm: p, hora: time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC), ids: map[string]int64{}}
	_ = alm.FijarZona("UTC")
	p.Ahora = func() time.Time { return e.hora }
	p.EsPropio = func(x string) bool { return x == pnYo }
	e.cli = &ClienteFalso{Conexion: true, PN: jid(pnYo), Errores: map[string]error{}, Enlace: "https://chat.whatsapp.com/ABCDEF"}
	e.cli.Info = &types.GroupInfo{JID: jid(grupoJID), ParticipantCount: 5,
		GroupName:  types.GroupName{Name: "Familia"},
		GroupTopic: types.GroupTopic{Topic: "Para la familia", TopicID: "TOP1"},
		Participants: []types.GroupParticipant{part(pnYo, soyAdmin, false), part(pnAna, true, false), part(pnLuis, false, false),
			{JID: jid(lidRosa), PhoneNumber: jid(pnRosa), LID: jid(lidRosa)}, part(pnCarla, false, true)},
		GroupCreated: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)}
	e.s = Nuevo(alm, 1, e.cli, p)
	e.s.Ahora = func() time.Time { return e.hora }
	e.s.Pausa = func(context.Context, time.Duration) {}
	e.s.Borrar = func(_ context.Context, chat, id string) error { e.borr = append(e.borr, chat+"|"+id); return nil }
	e.s.Enviar = func(_ context.Context, chat, texto, cita string, men []string) error {
		e.env = append(e.env, envio{chat, texto, cita, men})
		return nil
	}
	e.d = comandos.Nuevo(alm, 1, p)
	e.d.Ahora = p.Ahora
	e.d.AlCodigo = func(c almacen.Confirmacion, _ string) { e.cod = append(e.cod, c) }
	e.s.RegistrarComandos(e.d)
	alm.AprenderAlias(1, pnRosa, lidRosa)
	_, _ = alm.ActualizarChat(1, grupoJID, true, func(c *almacen.Chat) { c.Guardado = "Familia" })
	roles, _ := alm.ListarRoles(1)
	for _, r := range roles {
		e.ids[r.Nombre] = r.ID
	}
	return e
}

func (e *entorno) rol(nombre string, cas ...string) int64 {
	m := map[string]almacen.Alcance{}
	for _, c := range cas {
		m[c] = almacen.Alcance{}
	}
	id, err := e.perm.GuardarRol(almacen.Rol{Nombre: nombre, Casillas: m, LimiteMin: 100})
	if err != nil {
		panic(err)
	}
	e.ids[nombre] = id
	return id
}

func (e *entorno) da(persona, rol string) {
	if err := e.alm.AsignarRol(1, persona, e.ids[rol], 0, "yo"); err != nil {
		panic(err)
	}
}

func (e *entorno) en(quien, texto string, extra ...func(*comandos.Entrada)) comandos.Resultado {
	en := comandos.Entrada{Persona: quien, Chat: grupoJID, EsGrupo: true, Texto: texto, MensajeID: "CMD1"}
	for _, f := range extra {
		f(&en)
	}
	return e.d.Ejecutar(en)
}

func (e *entorno) dueno(texto string) comandos.Resultado {
	return e.d.Ejecutar(comandos.Entrada{Persona: almacen.PersonaDueno, Chat: pnYo, Texto: texto, Dueno: true, MensajeID: "D1"})
}

func citando(autor, id string) func(*comandos.Entrada) {
	return func(en *comandos.Entrada) { en.CitaAutor, en.CitaID = autor, id }
}

func ctx() context.Context { return context.Background() }

// ---------- administrador y no administrador ----------

func TestCadaAccionLlamaALaAPICorrectaSiendoAdmin(t *testing.T) {
	e := nuevo(t, true)
	g := jid(grupoJID)
	casos := []struct {
		nombre string
		f      func() error
		quiere string
	}{
		{"cerrar", func() error { return e.s.Cerrar(ctx(), "yo", grupoJID, true) }, fmt.Sprintf("SetGroupAnnounce|%s|true", g)},
		{"abrir", func() error { return e.s.Cerrar(ctx(), "yo", grupoJID, false) }, fmt.Sprintf("SetGroupAnnounce|%s|false", g)},
		{"bloquear", func() error { return e.s.BloquearInfo(ctx(), "yo", grupoJID, true) }, fmt.Sprintf("SetGroupLocked|%s|true", g)},
		{"desbloquear", func() error { return e.s.BloquearInfo(ctx(), "yo", grupoJID, false) }, fmt.Sprintf("SetGroupLocked|%s|false", g)},
		{"renombrar", func() error { return e.s.Renombrar(ctx(), "yo", grupoJID, "  Primos  ") }, fmt.Sprintf("SetGroupName|%s|Primos", g)},
		{"descripcion", func() error { return e.s.Descripcion(ctx(), "yo", grupoJID, "Nueva desc") }, fmt.Sprintf("SetGroupTopic|%s|prev=|new=|Nueva desc", g)},
		{"ascender", func() error { return e.s.Ascender(ctx(), "yo", grupoJID, pnLuis) }, fmt.Sprintf("UpdateGroupParticipants|%s|promote|[%s]", g, jid(pnLuis))},
		{"bajar", func() error { return e.s.Bajar(ctx(), "yo", grupoJID, pnAna) }, fmt.Sprintf("UpdateGroupParticipants|%s|demote|[%s]", g, jid(pnAna))},
		{"expulsar", func() error { return e.s.ExpulsarComo(ctx(), "yo", grupoJID, pnLuis) }, fmt.Sprintf("UpdateGroupParticipants|%s|remove|[%s]", g, jid(pnLuis))},
		{"enlace", func() error { _, err := e.s.Enlace(ctx(), "yo", grupoJID, false); return err }, fmt.Sprintf("GetGroupInviteLink|%s|false", g)},
		{"enlace nuevo", func() error { _, err := e.s.Enlace(ctx(), "yo", grupoJID, true); return err }, fmt.Sprintf("GetGroupInviteLink|%s|true", g)},
	}
	for _, c := range casos {
		if err := c.f(); err != nil {
			t.Fatalf("%s: %v", c.nombre, err)
		}
		if !e.cli.Hubo(c.quiere) {
			t.Fatalf("%s: falta la llamada %q en %v", c.nombre, c.quiere, e.cli.Llamadas)
		}
	}
	// El registro de acciones guardó cada una, sin textos.
	acs, _ := e.alm.ListarAcciones(1, 100, 0)
	for _, a := range acs {
		if strings.Contains(a.Detalle, "Primos") || strings.Contains(a.Detalle, "Nueva desc") {
			t.Fatalf("texto en el registro: %+v", a)
		}
	}
	if len(acs) < len(casos) {
		t.Fatalf("registro incompleto: %d", len(acs))
	}
}

func TestSinSerAdminNoSeLlamaANadaYSeExplica(t *testing.T) {
	e := nuevo(t, false)
	pruebas := []func() error{
		func() error { return e.s.Cerrar(ctx(), "yo", grupoJID, true) },
		func() error { return e.s.BloquearInfo(ctx(), "yo", grupoJID, true) },
		func() error { return e.s.Renombrar(ctx(), "yo", grupoJID, "X") },
		func() error { return e.s.Descripcion(ctx(), "yo", grupoJID, "X") },
		func() error { return e.s.Ascender(ctx(), "yo", grupoJID, pnLuis) },
		func() error { return e.s.ExpulsarComo(ctx(), "yo", grupoJID, pnLuis) },
		func() error { _, err := e.s.Enlace(ctx(), "yo", grupoJID, false); return err },
		func() error { _, err := e.s.Solicitudes(ctx(), "yo", grupoJID); return err },
		func() error { return e.s.BorrarMensajeAjeno(ctx(), grupoJID, "X", pnLuis) },
	}
	for i, f := range pruebas {
		if err := f(); !errors.Is(err, ErrNoAdmin) {
			t.Fatalf("%d: %v", i, err)
		}
	}
	if !e.cli.SinEscrituras() || e.cli.Hubo(fmt.Sprintf("GetGroupInviteLink|%s|false", jid(grupoJID))) || len(e.borr) != 0 {
		t.Fatalf("no debía llamar a nada: %v %v", e.cli.Llamadas, e.borr)
	}
	// Lo que no necesita admin sí funciona.
	if _, err := e.s.Info(ctx(), grupoJID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.Admins(ctx(), grupoJID); err != nil {
		t.Fatal(err)
	}
	st := e.s.Estado(ctx(), grupoJID, false)
	if st.SoyAdmin || !st.EnGrupo || !st.Conectado || !strings.Contains(st.Motivo, "No soy administrador") {
		t.Fatalf("%+v", st)
	}
	if ok, err := e.s.EsAdmin(ctx(), grupoJID); ok || err != nil {
		t.Fatalf("%v %v", ok, err)
	}
}

func TestEstadoSinConexionYFueraDelGrupo(t *testing.T) {
	e := nuevo(t, true)
	e.cli.Conexion = false
	st := e.s.Estado(ctx(), grupoJID, true)
	if st.Conectado || st.SoyAdmin || !strings.Contains(st.Motivo, "no está conectado") {
		t.Fatalf("%+v", st)
	}
	e.cli.Conexion = true
	e.cli.Errores["GetGroupInfo"] = whatsmeow.ErrNotInGroup
	st = e.s.Estado(ctx(), grupoJID, true)
	if st.EnGrupo || st.SoyAdmin || st.Motivo != "Ya no estás en este grupo." {
		t.Fatalf("%+v", st)
	}
	if st := e.s.Estado(ctx(), "5215511111111@s.whatsapp.net", true); st.SoyAdmin || !strings.Contains(st.Motivo, "grupos") {
		t.Fatalf("no es un grupo: %+v", st)
	}
	// Con la app sin cliente (bot apagado) todo se explica sin romperse.
	solo := Nuevo(e.alm, 1, nil, e.perm)
	if _, err := solo.Info(ctx(), grupoJID); !errors.Is(err, ErrSinConexion) {
		t.Fatalf("%v", err)
	}
	if st := solo.Estado(ctx(), grupoJID, false); st.Conectado || st.Motivo == "" {
		t.Fatalf("%+v", st)
	}
}

func TestLaInfoDelGrupoSeCacheaYLosEventosLaRenuevan(t *testing.T) {
	e := nuevo(t, true)
	for i := 0; i < 3; i++ {
		e.s.Estado(ctx(), grupoJID, false)
	}
	if e.cli.InfoPedida != 1 {
		t.Fatalf("la caché debía evitar pedir otra vez: %d", e.cli.InfoPedida)
	}
	// Dejo de ser administrador: hasta que no llega el evento sigue la información vieja; luego se vuelve a preguntar.
	copia := *e.cli.Info
	copia.Participants = append([]types.GroupParticipant(nil), e.cli.Info.Participants...)
	copia.Participants[0].IsAdmin = false
	e.cli.Info = &copia
	if !e.s.Estado(ctx(), grupoJID, false).SoyAdmin {
		t.Fatal("aún vale la caché")
	}
	e.s.Invalidar(grupoJID)
	if e.s.Estado(ctx(), grupoJID, false).SoyAdmin {
		t.Fatal("tras el evento ya no soy admin")
	}
	// La caché también vence sola.
	e.hora = e.hora.Add(10 * time.Minute)
	e.s.Estado(ctx(), grupoJID, false)
	if e.cli.InfoPedida != 3 {
		t.Fatalf("%d", e.cli.InfoPedida)
	}
}

func TestErroresTipadosSeTraducenAMensajesClaros(t *testing.T) {
	casos := []struct {
		err    error
		quiere string
	}{
		{whatsmeow.ErrGroupInviteLinkUnauthorized, "solo los administradores pueden ver o reiniciar el enlace"},
		{fmt.Errorf("x: %w", whatsmeow.ErrNotInGroup), "ya no estás en ese grupo"},
		{whatsmeow.ErrGroupNotFound, "ese grupo ya no existe"},
		{whatsmeow.ErrIQForbidden, "ya no soy administrador"},
		{whatsmeow.ErrIQNotAuthorized, "ya no soy administrador"},
		{whatsmeow.ErrIQNotFound, "ya no existe"},
		{whatsmeow.ErrIQRateOverLimit, "más despacio"},
		{whatsmeow.ErrIQTimedOut, "tardó demasiado"},
		{context.DeadlineExceeded, "tardó demasiado"},
		{whatsmeow.ErrNotConnected, "no hay conexión"},
		{whatsmeow.ErrIQDisconnected, "no hay conexión"},
		{whatsmeow.ErrIQServiceUnavailable, "no está disponible"},
		{whatsmeow.ErrIQResourceLimit, "límite"},
		{errors.New("cosa rara en inglés"), "WhatsApp no pudo hacerlo"},
	}
	for _, c := range casos {
		got := Explicar(c.err)
		if got == nil || !strings.Contains(got.Error(), c.quiere) {
			t.Fatalf("%v -> %v (quería %q)", c.err, got, c.quiere)
		}
		if strings.Contains(got.Error(), "info query") || strings.Contains(got.Error(), "that group") || strings.Contains(got.Error(), "inglés") {
			t.Fatalf("se coló el texto en inglés: %v", got)
		}
		if !errors.Is(got, c.err) {
			t.Fatalf("debe conservar la causa: %v", got)
		}
	}
	if Explicar(nil) != nil {
		t.Fatal("nil")
	}
	if Explicar(ErrNoAdmin) != ErrNoAdmin {
		t.Fatal("los errores propios pasan tal cual")
	}
	// En una acción real: el servidor dice que ya no soy admin -> mensaje claro y la caché se renueva.
	e := nuevo(t, true)
	e.cli.Errores["GetGroupInviteLink"] = whatsmeow.ErrGroupInviteLinkUnauthorized
	_, err := e.s.Enlace(ctx(), "yo", grupoJID, false)
	if err == nil || !strings.Contains(err.Error(), "solo los administradores") {
		t.Fatalf("%v", err)
	}
	e.cli.Errores["SetGroupAnnounce"] = whatsmeow.ErrIQForbidden
	e.cli.InfoPedida = 0
	if err := e.s.Cerrar(ctx(), "yo", grupoJID, true); err == nil || !strings.Contains(err.Error(), "ya no soy administrador") {
		t.Fatalf("%v", err)
	}
	e.s.Estado(ctx(), grupoJID, false)
	if e.cli.InfoPedida < 2 {
		t.Fatal("tras un error de permiso se vuelve a preguntar si sigo siendo admin")
	}
}

func TestExpulsarProtegeAlCreadorAYoMismoYLaJerarquia(t *testing.T) {
	e := nuevo(t, true)
	if err := e.s.ExpulsarComo(ctx(), "yo", grupoJID, pnCarla); !errors.Is(err, ErrCreador) {
		t.Fatalf("%v", err)
	}
	if err := e.s.ExpulsarComo(ctx(), "yo", grupoJID, pnYo); !errors.Is(err, ErrYoMismo) {
		t.Fatalf("%v", err)
	}
	if err := e.s.ExpulsarComo(ctx(), "yo", grupoJID, "5219999999999@s.whatsapp.net"); !errors.Is(err, ErrNoMiembro) {
		t.Fatalf("%v", err)
	}
	if err := e.s.Ascender(ctx(), "yo", grupoJID, pnAna); !errors.Is(err, ErrYaAdmin) {
		t.Fatalf("%v", err)
	}
	if err := e.s.Bajar(ctx(), "yo", grupoJID, pnLuis); !errors.Is(err, ErrNoEsAdmin) {
		t.Fatalf("%v", err)
	}
	// Rosa viene por LID en el grupo: se encuentra por su número.
	if err := e.s.ExpulsarComo(ctx(), "yo", grupoJID, pnRosa); err != nil {
		t.Fatal(err)
	}
	if !e.cli.Hubo(fmt.Sprintf("UpdateGroupParticipants|%s|remove|[%s]", jid(grupoJID), jid(lidRosa))) {
		t.Fatalf("debía usar el JID del grupo (LID): %v", e.cli.Llamadas)
	}
	// Un código de error por participante también es un error claro.
	e.cli.Resultado = []types.GroupParticipant{{JID: jid(pnLuis), Error: 403}}
	if err := e.s.ExpulsarComo(ctx(), "yo", grupoJID, pnLuis); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("%v", err)
	}
	// Jerarquía: un rango bajo no expulsa a uno alto.
	e.cli.Resultado = nil
	e.rol("Alto", "grupos.expulsar")
	e.rol("Bajo", "grupos.expulsar")
	e.da(pnAna, "Alto")
	e.da(pnLuis, "Bajo")
	if err := e.s.ExpulsarComo(ctx(), pnLuis, grupoJID, pnAna); !errors.Is(err, permisos.ErrJerarquia) {
		t.Fatalf("%v", err)
	}
	if err := e.s.ExpulsarComo(ctx(), pnAna, grupoJID, pnLuis); err != nil {
		t.Fatal(err)
	}
}

// ---------- comandos con confirmación ----------

func TestExpulsarPorComandoPideCodigoYLuegoLlamaALaAPI(t *testing.T) {
	e := nuevo(t, true)
	e.rol("Mod", "grupos.expulsar", "grupos.ascender")
	e.da(pnAna, "Mod")
	n := len(e.cli.Llamadas)
	r := e.en(pnAna, "!expulsar 5215522222222")
	if !strings.Contains(r.Texto, "confirmación") || len(e.cod) != 1 {
		t.Fatalf("%q %d", r.Texto, len(e.cod))
	}
	for _, l := range e.cli.Llamadas[n:] {
		if strings.HasPrefix(l, "Update") {
			t.Fatalf("no debía expulsar antes de confirmar: %v", e.cli.Llamadas)
		}
	}
	txt, err := e.d.Confirmar(e.cod[0].Codigo, pnAna)
	if err != nil || !strings.Contains(txt, "salió del grupo") {
		t.Fatalf("%q %v", txt, err)
	}
	if !e.cli.Hubo(fmt.Sprintf("UpdateGroupParticipants|%s|remove|[%s]", jid(grupoJID), jid(pnLuis))) {
		t.Fatalf("%v", e.cli.Llamadas)
	}
	// Sin indicar a quién, ni siquiera pide código.
	e.cod = nil
	if r := e.en(pnAna, "!expulsar"); !strings.Contains(r.Texto, "Dime a quién") || len(e.cod) != 0 {
		t.Fatalf("%q", r.Texto)
	}
	// Quien no tiene la casilla no puede.
	if r := e.en(pnRosa, "!expulsar 5215522222222"); !strings.Contains(r.Texto, "No tienes permiso") {
		t.Fatalf("%q", r.Texto)
	}
	// Ascender no pide código.
	if r := e.en(pnAna, "!ascender 5215522222222"); !strings.Contains(r.Texto, "administradora") {
		t.Fatalf("%q", r.Texto)
	}
	if !e.cli.Hubo(fmt.Sprintf("UpdateGroupParticipants|%s|promote|[%s]", jid(grupoJID), jid(pnLuis))) {
		t.Fatalf("%v", e.cli.Llamadas)
	}
}

func TestBorrarMensajeCitadoPideCodigoYBorra(t *testing.T) {
	e := nuevo(t, true)
	_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: "MALO", ChatJID: grupoJID, BotID: 1, RemitenteJID: pnLuis, Tipo: almacen.TipoTexto, Texto: "feo", TS: e.hora.Unix()})
	e.rol("Mod", "grupos.borrar_mensaje")
	e.da(pnAna, "Mod")
	if r := e.en(pnAna, "!borrar"); !strings.Contains(r.Texto, "Responde al mensaje") || len(e.cod) != 0 {
		t.Fatalf("%q", r.Texto)
	}
	r := e.en(pnAna, "!borrar", citando(pnLuis, "MALO"))
	if !strings.Contains(r.Texto, "confirmación") || len(e.cod) != 1 || len(e.borr) != 0 {
		t.Fatalf("%q", r.Texto)
	}
	if txt, err := e.d.Confirmar(e.cod[0].Codigo, pnAna); err != nil || !strings.Contains(txt, "borré ese mensaje") {
		t.Fatalf("%q %v", txt, err)
	}
	if len(e.borr) != 1 || e.borr[0] != grupoJID+"|MALO" {
		t.Fatalf("%v", e.borr)
	}
	// Si no soy admin, el comando lo dice claro tras confirmar y no borra.
	e2 := nuevo(t, false)
	_, _ = e2.alm.GuardarMensaje(&almacen.Mensaje{ID: "MALO", ChatJID: grupoJID, BotID: 1, RemitenteJID: pnLuis, Tipo: almacen.TipoTexto, Texto: "feo", TS: e2.hora.Unix()})
	e2.rol("Mod", "grupos.borrar_mensaje")
	e2.da(pnAna, "Mod")
	e2.en(pnAna, "!borrar", citando(pnLuis, "MALO"))
	txt, _ := e2.d.Confirmar(e2.cod[0].Codigo, pnAna)
	if !strings.Contains(txt, "No soy administrador") || len(e2.borr) != 0 {
		t.Fatalf("%q %v", txt, e2.borr)
	}
}

func TestEnlaceYReinicioConConfirmacion(t *testing.T) {
	e := nuevo(t, true)
	e.rol("Mod", "grupos.enlace")
	e.da(pnAna, "Mod")
	if r := e.en(pnAna, "!enlace"); r.Texto != "https://chat.whatsapp.com/ABCDEF" {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.en(pnAna, "!enlacenuevo"); !strings.Contains(r.Texto, "confirmación") || e.cli.Hubo(fmt.Sprintf("GetGroupInviteLink|%s|true", jid(grupoJID))) {
		t.Fatalf("%q", r.Texto)
	}
	txt, err := e.d.Confirmar(e.cod[0].Codigo, pnAna)
	if err != nil || !strings.Contains(txt, "Enlace nuevo") || !e.cli.Hubo(fmt.Sprintf("GetGroupInviteLink|%s|true", jid(grupoJID))) {
		t.Fatalf("%q %v", txt, err)
	}
	// No soy admin del grupo: mensaje claro del tipo de error de la librería.
	e.cli.Errores["GetGroupInviteLink"] = whatsmeow.ErrGroupInviteLinkUnauthorized
	if r := e.en(pnAna, "!enlace"); !strings.Contains(r.Texto, "Solo los administradores") {
		t.Fatalf("%q", r.Texto)
	}
}

func TestComandosDeAjustesSolicitudesYDueñoDesdeMensajesGuardados(t *testing.T) {
	e := nuevo(t, true)
	e.rol("Mod", "grupos.cerrar", "grupos.bloquear_info", "grupos.renombrar", "grupos.solicitudes", "grupos.info", "grupos.admins")
	e.da(pnAna, "Mod")
	g := jid(grupoJID)
	pruebas := []struct{ texto, contiene, llamada string }{
		{"!cerrar", "quedó cerrado", fmt.Sprintf("SetGroupAnnounce|%s|true", g)},
		{"!abrir", "quedó abierto", fmt.Sprintf("SetGroupAnnounce|%s|false", g)},
		{"!bloquearinfo", "solo los administradores", fmt.Sprintf("SetGroupLocked|%s|true", g)},
		{"!desbloquearinfo", "todos pueden", fmt.Sprintf("SetGroupLocked|%s|false", g)},
		{"!nombre Los Primos", "cambié el nombre", fmt.Sprintf("SetGroupName|%s|Los Primos", g)},
		{"!descripcion Hola a todos", "cambié la descripción", fmt.Sprintf("SetGroupTopic|%s|prev=|new=|Hola a todos", g)},
		{"!descripcion", "borré la descripción", fmt.Sprintf("SetGroupTopic|%s|prev=|new=|", g)},
	}
	for _, p := range pruebas {
		if r := e.en(pnAna, p.texto); !strings.Contains(r.Texto, p.contiene) || !e.cli.Hubo(p.llamada) {
			t.Fatalf("%s: %q %v", p.texto, r.Texto, e.cli.Llamadas)
		}
	}
	// Solicitudes: listar, aprobar a una, rechazar a todas.
	e.cli.Pendientes = []types.GroupParticipantRequest{{JID: jid(pnLuis), RequestedAt: e.hora}, {JID: jid("5215577777777@s.whatsapp.net"), RequestedAt: e.hora}}
	if r := e.en(pnAna, "!solicitudes"); !strings.Contains(r.Texto, "2 solicitudes") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.en(pnAna, "!aprobar 5215522222222"); !strings.Contains(r.Texto, "aprobé 1") ||
		!e.cli.Hubo(fmt.Sprintf("UpdateGroupRequestParticipants|%s|approve|[%s]", g, jid(pnLuis))) {
		t.Fatalf("%q %v", r.Texto, e.cli.Llamadas)
	}
	if r := e.en(pnAna, "!rechazar"); !strings.Contains(r.Texto, "rechacé 2") ||
		!e.cli.Hubo(fmt.Sprintf("UpdateGroupRequestParticipants|%s|reject|[%s %s]", g, jid(pnLuis), jid("5215577777777@s.whatsapp.net"))) {
		t.Fatalf("%q %v", r.Texto, e.cli.Llamadas)
	}
	e.cli.Pendientes = nil
	if r := e.en(pnAna, "!aprobar"); !strings.Contains(r.Texto, "No hay solicitudes") {
		t.Fatalf("%q", r.Texto)
	}
	// Info y admins sin ser admin: siguen contestando.
	if r := e.en(pnAna, "!grupo"); !strings.Contains(r.Texto, "Familia") || !strings.Contains(r.Texto, "Miembros: 5") || !strings.Contains(r.Texto, "02/01/2025") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.en(pnAna, "!admins"); !strings.Contains(r.Texto, "Administradores (3)") || !strings.Contains(r.Texto, "creó el grupo") {
		t.Fatalf("%q", r.Texto)
	}
	// El dueño desde Mensajes guardados indica el grupo por nombre.
	if r := e.dueno("!cerrar"); !strings.Contains(r.Texto, "de qué grupo") {
		t.Fatalf("%q", r.Texto)
	}
	e.dijo(pnAna, 1, 1) // un grupo sin mensajes no sale en las búsquedas
	if r := e.dueno("!cerrar #trabajo"); !strings.Contains(r.Texto, "No encontré un grupo") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.dueno("!cerrar #familia"); !strings.Contains(r.Texto, "quedó cerrado") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.dueno("!cerrar " + grupoJID); !strings.Contains(r.Texto, "quedó cerrado") {
		t.Fatalf("%q", r.Texto)
	}
}

func TestLaAyudaSoloMuestraLosComandosDeGruposDelRango(t *testing.T) {
	e := nuevo(t, true)
	e.rol("Mod", "grupos.cerrar")
	e.rol("Nada")
	e.da(pnAna, "Mod")
	e.da(pnLuis, "Nada")
	if r := e.en(pnAna, "!ayuda"); !strings.Contains(r.Texto, "cerrar") || strings.Contains(r.Texto, "expulsar") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.en(pnLuis, "!ayuda"); strings.Contains(r.Texto, "cerrar") {
		t.Fatalf("%q", r.Texto)
	}
	// En privado, un comando de grupo avisa que solo vale en grupos.
	if r := e.d.Ejecutar(comandos.Entrada{Persona: pnAna, Chat: pnAna, Texto: "!cerrar"}); !strings.Contains(r.Texto, "dentro de un grupo") {
		t.Fatalf("%q", r.Texto)
	}
}

// ---------- mencionar a todos ----------

func TestMencionarATodosConLotesTopeYEspera(t *testing.T) {
	e := nuevo(t, false) // no hace falta ser admin
	e.rol("Alto", "grupos.mencionar_todos")
	e.rol("Raso")
	e.da(pnAna, "Alto")
	e.da(pnLuis, "Raso")
	// Un grupo de 70 personas más yo.
	ps := []types.GroupParticipant{part(pnYo, false, false)}
	for i := 0; i < 70; i++ {
		ps = append(ps, part(fmt.Sprintf("52166%08d@s.whatsapp.net", i), false, false))
	}
	e.cli.Info.Participants = ps
	_ = e.alm.FijarAjusteBot(1, "todos_tope", "50")
	_ = e.alm.FijarAjusteBot(1, "todos_lote", "20")
	if r := e.en(pnLuis, "!todos hola"); !strings.Contains(r.Texto, "No tienes permiso") || len(e.env) != 0 {
		t.Fatalf("%q", r.Texto)
	}
	r := e.en(pnAna, "!todos Reunión a las 5")
	if !strings.Contains(r.Texto, "Mencioné a 50 de 70") || !strings.Contains(r.Texto, "tope es 50") {
		t.Fatalf("debe avisar el tope: %q", r.Texto)
	}
	if len(e.env) != 3 || len(e.env[0].menciones) != 20 || len(e.env[1].menciones) != 20 || len(e.env[2].menciones) != 10 {
		t.Fatalf("lotes: %d", len(e.env))
	}
	if !strings.HasPrefix(e.env[0].texto, "Reunión a las 5\n@") || e.env[0].cita != "CMD1" || e.env[1].cita != "" {
		t.Fatalf("%+v", e.env[0])
	}
	// Cada @usuario del texto es una mención real (MentionedJID) y nunca me menciona a mí.
	for _, m := range e.env {
		for _, j := range m.menciones {
			if !strings.Contains(m.texto, "@"+usuarioDe(j)) || strings.HasPrefix(j, "5215500000001") {
				t.Fatalf("mención rota: %s en %q", j, m.texto)
			}
		}
	}
	// Un segundo uso enseguida espera; el dueño no espera.
	n := len(e.env)
	if r := e.en(pnAna, "!todos otra vez"); !strings.Contains(r.Texto, "espera") || len(e.env) != n {
		t.Fatalf("%q", r.Texto)
	}
	e.hora = e.hora.Add(6 * time.Minute)
	if r := e.en(pnAna, "!todos ya"); strings.Contains(r.Texto, "espera") || len(e.env) == n {
		t.Fatalf("%q", r.Texto)
	}
	if _, err := e.s.MencionarTodos(ctx(), "yo", grupoJID, "", "x"); err != nil {
		t.Fatal(err)
	}
}

func TestMencionarATodosSiFallaAMediasAvisaCuantosYSinEnviador(t *testing.T) {
	e := nuevo(t, false)
	_ = e.alm.FijarAjusteBot(1, "todos_lote", "1")
	llamadas := 0
	e.s.Enviar = func(_ context.Context, chat, texto, cita string, men []string) error {
		llamadas++
		if llamadas == 3 {
			return whatsmeow.ErrIQRateOverLimit
		}
		return nil
	}
	r, err := e.s.MencionarTodos(ctx(), "yo", grupoJID, "", "")
	if err != nil || r.Mencionadas != 2 || !r.Truncado || !strings.Contains(r.Aviso, "Solo alcancé a mencionar a 2 de 4") {
		t.Fatalf("%+v %v", r, err)
	}
	e.s.Enviar = nil
	if _, err := e.s.MencionarTodos(ctx(), "yo", grupoJID, "", ""); !errors.Is(err, ErrNoDisponible) {
		t.Fatalf("%v", err)
	}
}

// ---------- ranking e inactivos ----------

func (e *entorno) dijo(quien string, diasAtras int, n int) {
	for i := 0; i < n; i++ {
		ts := e.hora.AddDate(0, 0, -diasAtras).Add(time.Duration(i) * time.Second).Unix()
		_, err := e.alm.GuardarMensaje(&almacen.Mensaje{ID: fmt.Sprintf("%s-%d-%d", quien, diasAtras, i), ChatJID: grupoJID, BotID: 1, RemitenteJID: quien, Tipo: almacen.TipoTexto, Texto: "hola", TS: ts})
		if err != nil {
			panic(err)
		}
	}
}

func TestRankingEInactivosDesdeDailyStats(t *testing.T) {
	e := nuevo(t, false)
	e.dijo(pnAna, 1, 5)
	e.dijo(pnAna, 20, 3)
	e.dijo(pnLuis, 2, 2)
	e.dijo(pnLuis, 40, 9)                         // fuera de 30 días, dentro de 90
	e.dijo(lidRosa, 3, 1)                         // llega por LID: cuenta como Rosa (alias)
	e.dijo("5219999999999@s.whatsapp.net", 1, 20) // ya no está en el grupo
	e.dijo(pnCarla, 60, 4)
	r, err := e.s.Ranking(ctx(), grupoJID, 30, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !r.SoloMiembros || r.Total != 11 || len(r.Filas) != 3 || r.Filas[0].Puesto != 1 || r.Filas[0].Mensajes != 8 || r.Filas[1].Mensajes != 2 || r.Filas[2].Mensajes != 1 {
		t.Fatalf("%+v", r)
	}
	if r.Filas[2].Nombre == "" || r.Dias != 30 {
		t.Fatalf("%+v", r)
	}
	// Periodos de la app: 7 días deja solo lo reciente; 90 suma lo viejo.
	r7, _ := e.s.Ranking(ctx(), grupoJID, 7, 10)
	r90, _ := e.s.Ranking(ctx(), grupoJID, 90, 10)
	if r7.Total != 8 || r90.Total != 11+9+4 {
		t.Fatalf("7:%d 90:%d", r7.Total, r90.Total)
	}
	// Por el comando.
	if r := e.dueno("!ranking 7 " + grupoJID); !strings.Contains(r.Texto, "últimos 7 días") || !strings.Contains(r.Texto, "1.") {
		t.Fatalf("%q", r.Texto)
	}
	// Inactivos entre miembros actuales: nadie fuera del grupo, ni yo.
	in30, aviso, err := e.s.Inactivos(ctx(), grupoJID, 30)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Inactivo{}
	for _, x := range in30 {
		got[x.Persona] = x
	}
	if len(in30) != 1 || got[pnCarla].UltimoDia != "2026-09-11" {
		t.Fatalf("%+v", in30)
	}
	_ = aviso
	in7, _, _ := e.s.Inactivos(ctx(), grupoJID, 7)
	if len(in7) != 1+0 && len(in7) != 2 {
		t.Fatalf("%+v", in7)
	}
	names := map[string]bool{}
	for _, x := range in7 {
		names[x.Persona] = true
	}
	if names[pnAna] || names[pnYo] || names["5219999999999@s.whatsapp.net"] {
		t.Fatalf("%+v", in7)
	}
	// Quien nunca escribió sale con último día vacío y primero.
	e2 := nuevo(t, false)
	e2.dijo(pnAna, 1, 1)
	l, _, _ := e2.s.Inactivos(ctx(), grupoJID, 30)
	if len(l) != 3 || l[0].UltimoDia != "" || l[0].DiasSin != -1 {
		t.Fatalf("%+v", l)
	}
	if txt := TextoInactivos(l, "", 30); !strings.Contains(txt, "3 personas sin mensajes en 30 días") || !strings.Contains(txt, "sin mensajes guardados") {
		t.Fatalf("%q", txt)
	}
	// Sin conexión el ranking sigue funcionando (cuenta a todos los que escribieron) pero inactivos no puede.
	e.cli.Conexion = false
	e.s.Invalidar(grupoJID)
	rs, err := e.s.Ranking(ctx(), grupoJID, 30, 10)
	if err != nil || rs.SoloMiembros || rs.Total != 11+20 {
		t.Fatalf("%+v %v", rs, err)
	}
	if _, _, err := e.s.Inactivos(ctx(), grupoJID, 30); !errors.Is(err, ErrSinConexion) {
		t.Fatalf("%v", err)
	}
}

func TestPeriodoPorDefectoYLimites(t *testing.T) {
	e := nuevo(t, false)
	if e.s.NormalizaDias(0) != 30 || e.s.NormalizaDias(9999) != 365 || e.s.NormalizaDias(7) != 7 {
		t.Fatal("días")
	}
	_ = e.alm.FijarAjusteBot(1, "grupo_dias", "90")
	if e.s.NormalizaDias(0) != 90 {
		t.Fatal("ajuste")
	}
}

// ---------- reglas ----------

func TestReglasDelGrupoSeGuardanYSalenEnLaInfo(t *testing.T) {
	e := nuevo(t, false)
	if err := e.s.FijarReglas("yo", grupoJID, "1. Respeto\n2. Sin spam"); err != nil {
		t.Fatal(err)
	}
	i, err := e.s.Info(ctx(), grupoJID)
	if err != nil || !strings.Contains(i.Reglas, "Sin spam") || !strings.Contains(TextoInfo(i), "Reglas:") {
		t.Fatalf("%+v %v", i, err)
	}
	e.rol("Mod", "grupos.info")
	e.da(pnAna, "Mod")
	if r := e.en(pnAna, "!reglas"); !strings.Contains(r.Texto, "Respeto") {
		t.Fatalf("%q", r.Texto)
	}
	_ = e.s.FijarReglas("yo", grupoJID, "")
	if r := e.en(pnAna, "!reglas"); !strings.Contains(r.Texto, "todavía no tiene reglas") {
		t.Fatalf("%q", r.Texto)
	}
	if err := e.s.FijarReglas("yo", "5215511111111@s.whatsapp.net", "x"); !errors.Is(err, ErrNoEsGrupo) {
		t.Fatalf("%v", err)
	}
}
