package motor

import (
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/permisos"
)

const (
	pnAna     = "5215511111111@s.whatsapp.net"
	pnLuis    = "5215522222222@s.whatsapp.net"
	propio    = "5215500000001@s.whatsapp.net"
	grupoJID  = "grupo@g.us"
	otroGrupo = "otro@g.us"
)

// conRangos arma el motor con la compuerta de permisos y los comandos.
func conRangos(t *testing.T, admin func(chat, persona string) bool) (*entorno, *permisos.Servicio, *comandos.Despachador) {
	t.Helper()
	var perm *permisos.Servicio
	var cmd *comandos.Despachador
	e := nuevoEntorno(t, func(c *Config) {
		p, err := permisos.Nuevo(c.Alm, c.BotID)
		if err != nil {
			t.Fatal(err)
		}
		perm = p
		cmd = comandos.Nuevo(c.Alm, c.BotID, p)
		c.Perm, c.Cmd, c.EsAdminGrupo = p, cmd, admin
	})
	return e, perm, cmd
}

func miMensaje(id, texto string) Mensaje {
	return Mensaje{Chat: propio, ID: id, Texto: texto, EsTexto: true, EsMio: true, EsChatPropio: true, TS: time.Now()}
}

func TestDuenoSoloDesdeMensajesGuardados(t *testing.T) {
	e, _, _ := conRangos(t, nil)
	// El comando del dueño en "Mensajes guardados" se contesta en ese mismo chat, citando mi mensaje.
	e.m.Entrar(miMensaje("D1", "!rango"))
	e.espera(t, func() bool { return len(e.env.lista()) == 1 })
	r := e.env.lista()[0]
	if r.chat != propio || r.cita != "D1" || !strings.Contains(r.texto, "Dueño") {
		t.Fatalf("%+v", r)
	}
	// !ayuda del dueño lista todo y no empieza con el prefijo (no puede volver como eco).
	e.m.Entrar(miMensaje("D2", "!ayuda"))
	e.espera(t, func() bool { return len(e.env.lista()) == 2 })
	if a := e.env.lista()[1].texto; strings.HasPrefix(a, "!") || !strings.Contains(a, "!asignar") {
		t.Fatalf("%q", a)
	}
	// Lo que escribo en OTRO chat (un cliente) con prefijo NO es un comando de dueño.
	cli := Mensaje{Chat: pnAna, ID: "X1", Texto: "!rango", EsTexto: true, EsMio: true, EsChatPropio: false, TS: time.Now()}
	e.m.Entrar(cli)
	// Tampoco sin prefijo en Mensajes guardados, ni una edición, ni un mensaje viejo.
	e.m.Entrar(miMensaje("D3", "hola, nota para mí"))
	ed := miMensaje("D4", "!rango")
	ed.EsEdicion = true
	e.m.Entrar(ed)
	viejo := miMensaje("D5", "!rango")
	viejo.TS = time.Now().Add(-time.Hour)
	e.m.Entrar(viejo)
	// Un duplicado del mismo id no se repite.
	e.m.Entrar(miMensaje("D1", "!rango"))
	e.quieto()
	if n := len(e.env.lista()); n != 2 {
		t.Fatalf("se mandaron %d respuestas: %+v", n, e.env.lista())
	}
	// El eco de la propia respuesta del bot en Mensajes guardados no dispara nada.
	e.m.Entrar(miMensaje("E1", "Eres el Dueño: puedes todo."))
	e.quieto()
	if len(e.env.lista()) != 2 {
		t.Fatal("el eco no debe contestarse")
	}
	// Un comando de dueño no cuenta como "contesté yo": no cancela lo pendiente de otros chats.
	e.m.Entrar(priv(pnLuis, "L1", "hola"))
	e.m.Entrar(miMensaje("D6", "!rango"))
	e.espera(t, func() bool { return len(e.env.lista()) >= 4 })
	hubo := false
	for _, x := range e.env.lista() {
		if x.chat == pnLuis && x.texto == "eco hola" {
			hubo = true
		}
	}
	if !hubo {
		t.Fatalf("el cliente debía recibir su respuesta: %+v", e.env.lista())
	}
	// Con el interruptor general apagado tampoco contesta.
}

func TestDuenoNoRespondeConElBotApagado(t *testing.T) {
	apagado := false
	e, _, _ := conRangos(t, nil)
	_ = apagado
	e.m.c.Activo = func() bool { return false }
	e.m.Entrar(miMensaje("D1", "!rango"))
	e.quieto()
	if len(e.env.lista()) != 0 {
		t.Fatal("apagado")
	}
}

func TestComandosDeOtrosRangosSonRespuestas(t *testing.T) {
	e, perm, cmd := conRangos(t, nil)
	idJefe, _ := perm.GuardarRol(almacen.Rol{Nombre: "Jefe", Casillas: map[string]almacen.Alcance{"rangos.asignar": {}}, LimiteMin: 30})
	_ = e.alm.AsignarRol(1, pnAna, idJefe, 0, "yo")
	_, _ = perm.GuardarRol(almacen.Rol{Nombre: "Raso", LimiteMin: 30})

	// !ayuda de alguien sin rango: contesta la ayuda (no el decisor).
	e.m.Entrar(priv(pnLuis, "A1", "!ayuda"))
	e.espera(t, func() bool { return len(e.env.lista()) == 1 })
	if r := e.env.lista()[0]; r.chat != pnLuis || r.cita != "A1" || !strings.Contains(r.texto, "!rango") || strings.Contains(r.texto, "!asignar") {
		t.Fatalf("%+v", r)
	}
	if len(e.dec.lotes) != 0 {
		t.Fatal("un comando conocido no pasa por el motor de menús")
	}
	// Un comando que no existe sigue al motor de menús (por ejemplo un comando propio !horario).
	e.m.Entrar(priv(pnLuis, "A2", "!horario"))
	e.espera(t, func() bool { return len(e.env.lista()) == 2 })
	if e.env.lista()[1].texto != "eco !horario" {
		t.Fatalf("%+v", e.env.lista()[1])
	}
	// !asignar con jerarquía: el jefe da "Raso" a Luis.
	e.m.Entrar(priv(pnAna, "A3", "!asignar 5215522222222 Raso 24h"))
	e.espera(t, func() bool { return len(e.env.lista()) == 3 })
	if !strings.Contains(e.env.lista()[2].texto, "Listo") {
		t.Fatalf("%+v", e.env.lista()[2])
	}
	if r, _, _ := e.alm.RolDePersona(1, pnLuis, time.Now()); r == nil || r.Nombre != "Raso" {
		t.Fatal("no se asignó")
	}
	// Luis (Raso) no puede asignar.
	e.m.Entrar(priv(pnLuis, "A4", "!asignar 5215511111111 Raso"))
	e.espera(t, func() bool { return len(e.env.lista()) == 4 })
	if !strings.Contains(e.env.lista()[3].texto, "No tienes permiso") {
		t.Fatalf("%+v", e.env.lista()[3])
	}
	// En un grupo: el comando sin mencionar al bot pasa la puerta y se contesta mencionando a quien lo pidió.
	g := Mensaje{Chat: grupoJID, ID: "G1", Remitente: pnAna, RemitenteWA: pnAna, Texto: "!rango", EsGrupo: true, EsTexto: true, TS: time.Now()}
	e.m.Entrar(g)
	e.espera(t, func() bool { return len(e.env.lista()) == 5 })
	r := e.env.lista()[4]
	if r.chat != grupoJID || r.mencion != pnAna || !strings.Contains(r.texto, "Jefe") || !strings.HasPrefix(r.texto, "@5215511111111 ") {
		t.Fatalf("%+v", r)
	}
	// En un grupo, un mensaje común sin mención ni comando se ignora.
	sin := g
	sin.ID, sin.Texto = "G2", "buenas"
	e.m.Entrar(sin)
	e.quieto()
	if len(e.env.lista()) != 5 {
		t.Fatal("grupo sin mención")
	}
	// Prefijo configurable también en el motor.
	_ = e.alm.FijarAjusteBot(1, "prefijo", "#")
	e.m.Entrar(priv(pnLuis, "A5", "#rango"))
	e.espera(t, func() bool { return len(e.env.lista()) == 6 })
	if !strings.Contains(e.env.lista()[5].texto, "Raso") {
		t.Fatalf("%+v", e.env.lista()[5])
	}
	_ = cmd
	// Pasado el límite por minuto de un rango, solo se avisa una vez.
	for i := 0; i < 40; i++ {
		e.m.Entrar(priv(pnLuis, "R"+string(rune('a'+i%26))+string(rune('A'+i/26)), "#rango"))
		time.Sleep(2 * time.Millisecond)
	}
	e.quieto()
	e.quieto()
	var avisos int
	for _, x := range e.env.lista() {
		if strings.Contains(x.texto, "muy rápido") {
			avisos++
		}
	}
	if avisos > 1 {
		t.Fatalf("el aviso de límite se repitió %d veces", avisos)
	}
}

func TestDesconocidosEnEntrar(t *testing.T) {
	e, perm, _ := conRangos(t, nil)
	// Política por defecto (menú): contesta como siempre.
	e.m.Entrar(priv(pnAna, "1", "hola"))
	e.espera(t, func() bool { return len(e.env.lista()) == 1 })

	// Esperar aprobación: no contesta, queda una solicitud.
	if err := perm.FijarPoliticaDesconocidos(permisos.DescAprobar); err != nil {
		t.Fatal(err)
	}
	e.m.Entrar(priv(pnLuis, "2", "hola"))
	e.quieto()
	if len(e.env.lista()) != 1 {
		t.Fatal("no debía contestar")
	}
	if e.motivos()["desconocido: esperando aprobación"] != 1 {
		t.Fatalf("motivos: %v", e.motivos())
	}
	l, _ := e.alm.ListarSolicitudes(1, "", 10)
	if len(l) != 1 || l[0].Persona != pnLuis {
		t.Fatalf("solicitudes: %+v", l)
	}
	// Aprobar → desde ese momento sí contesta.
	_ = e.alm.DecidirSolicitud(1, pnLuis, almacen.SolAprobada, time.Now())
	e.m.Entrar(priv(pnLuis, "3", "otra vez"))
	e.espera(t, func() bool { return len(e.env.lista()) == 2 })
	// Bloquear → silencio total.
	_ = e.alm.DecidirSolicitud(1, pnLuis, almacen.SolBloqueada, time.Now())
	e.m.Entrar(priv(pnLuis, "4", "hola?"))
	e.quieto()
	if len(e.env.lista()) != 2 {
		t.Fatal("bloqueado")
	}
	// "Te contestamos pronto": una sola vez.
	_ = perm.FijarPoliticaDesconocidos(permisos.DescPronto)
	nuevo := "5215509090909@s.whatsapp.net"
	e.m.Entrar(priv(nuevo, "5", "buenas"))
	e.espera(t, func() bool { return len(e.env.lista()) == 3 })
	if r := e.env.lista()[2]; r.chat != nuevo || !strings.Contains(r.texto, "contestamos pronto") || r.cita != "5" {
		t.Fatalf("%+v", r)
	}
	e.m.Entrar(priv(nuevo, "6", "¿hay alguien?"))
	e.quieto()
	if len(e.env.lista()) != 3 {
		t.Fatal("el aviso solo se manda una vez")
	}
	// Ignorar: nada, y no queda guardado.
	_ = perm.FijarPoliticaDesconocidos(permisos.DescIgnorar)
	pend := e.alm.ContarSolicitudes(1, almacen.SolPendiente)
	e.m.Entrar(priv("5215508080808@s.whatsapp.net", "7", "hola"))
	e.quieto()
	if len(e.env.lista()) != 3 || e.alm.ContarSolicitudes(1, almacen.SolPendiente) != pend {
		t.Fatal("ignorar")
	}
	// Una persona con rango nunca es desconocida.
	id, _ := perm.GuardarRol(almacen.Rol{Nombre: "VIP"})
	vip := "5215507070707@s.whatsapp.net"
	_ = e.alm.AsignarRol(1, vip, id, 0, "yo")
	e.m.Entrar(priv(vip, "8", "hola"))
	e.espera(t, func() bool { return len(e.env.lista()) == 4 })
}

func TestPoliticaDeGruposEnEntrar(t *testing.T) {
	admins := map[string]bool{}
	e, perm, _ := conRangos(t, func(chat, p string) bool { return admins[p] })
	g := func(id, quien string) Mensaje {
		return Mensaje{Chat: grupoJID, ID: id, Remitente: quien, RemitenteWA: quien, Texto: "hola " + id, EsGrupo: true, EsTexto: true, Mencionado: true, TS: time.Now()}
	}
	// Por defecto: a todos.
	e.m.Entrar(g("1", pnAna))
	e.espera(t, func() bool { return len(e.env.lista()) == 1 })
	// Solo administradores.
	_ = e.alm.FijarPoliticaGrupo(1, almacen.PoliticaGrupo{Chat: grupoJID, Modo: "admins"})
	e.m.Entrar(g("2", pnLuis))
	e.quieto()
	if len(e.env.lista()) != 1 {
		t.Fatal("no es admin")
	}
	admins[pnAna] = true
	e.m.Entrar(g("3", pnAna))
	e.espera(t, func() bool { return len(e.env.lista()) == 2 })
	// Solo una lista.
	_ = e.alm.FijarPoliticaGrupo(1, almacen.PoliticaGrupo{Chat: grupoJID, Modo: "lista", Lista: []string{pnLuis}})
	e.m.Entrar(g("4", pnAna))
	e.m.Entrar(g("5", pnLuis))
	e.espera(t, func() bool { return len(e.env.lista()) == 3 })
	e.quieto()
	if len(e.env.lista()) != 3 {
		t.Fatalf("solo Luis: %+v", e.env.lista())
	}
	// Aprobar en privado NO autoriza el grupo: Ana aprobada sigue fuera de la lista.
	_ = perm.FijarPoliticaDesconocidos(permisos.DescAprobar)
	_ = e.alm.DecidirSolicitud(1, pnAna, almacen.SolAprobada, time.Now())
	e.m.Entrar(g("6", pnAna))
	e.quieto()
	if len(e.env.lista()) != 3 {
		t.Fatal("aprobar en privado no autoriza el grupo")
	}
	// Otro grupo: política general.
	o := g("7", pnAna)
	o.Chat = otroGrupo
	e.m.Entrar(o)
	e.espera(t, func() bool { return len(e.env.lista()) == 4 })
	// Los comandos en un grupo restringido también pasan por la compuerta.
	c := g("8", pnAna)
	c.Texto, c.Mencionado = "!rango", false
	e.m.Entrar(c)
	e.quieto()
	if len(e.env.lista()) != 4 {
		t.Fatal("comando de alguien fuera de la lista")
	}
}

// Quien escribe (y pasa la compuerta) recibe el rango Usuario; un administrador de grupo no.
func TestQuienEscribeRecibeElRangoUsuarioMenosLosAdmins(t *testing.T) {
	e, perm, _ := conRangos(t, func(chat, persona string) bool { return persona == pnLuis })
	_, _ = e.alm.ActualizarMiembros(1, grupoJID, []almacen.MiembroGrupo{{Persona: pnLuis, JID: pnLuis, Rol: almacen.RolGrupoAdmin}, {Persona: pnAna, JID: pnAna, Rol: almacen.RolGrupoMiembro}}, time.Now())
	e.m.Entrar(priv(pnAna, "u1", "hola"))
	e.m.Entrar(priv(pnLuis, "u2", "hola"))
	e.espera(t, func() bool { return len(e.env.lista()) >= 1 })
	e.quieto()
	if r := perm.RolDe(pnAna); r == nil || r.Nombre != "Usuario" {
		t.Fatalf("quien escribe debía quedar como Usuario: %+v", r)
	}
	if r := perm.RolDe(pnLuis); r != nil {
		t.Fatalf("un administrador de grupo no es Usuario: %+v", r)
	}
}

// En un grupo, quien escribe con su LID (alias de su número) y no tiene rango también recibe Usuario.
func TestUsuarioEnGrupoConLID(t *testing.T) {
	e, perm, _ := conRangos(t, func(chat, persona string) bool { return false })
	const lid = "150831433277469@lid"
	e.alm.AprenderAlias(1, pnAna, lid)
	_, _ = e.alm.ActualizarMiembros(1, grupoJID, []almacen.MiembroGrupo{{Persona: pnAna, JID: lid, Rol: almacen.RolGrupoMiembro}}, time.Now())
	e.m.Entrar(grupoM(grupoJID, "g1", lid, "!rango"))
	e.espera(t, func() bool { return len(e.env.lista()) >= 1 })
	if r := perm.RolDe(pnAna); r == nil || r.Nombre != "Usuario" {
		t.Fatalf("debía quedar como Usuario: %+v · respuestas: %+v", r, e.env.lista())
	}
	if !strings.Contains(e.env.lista()[0].texto, "Usuario") {
		t.Logf("respuesta: %q", e.env.lista()[0].texto)
	}
}
