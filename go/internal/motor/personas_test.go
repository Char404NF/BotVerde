package motor

import (
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/personas"
)

func conPersonas(t *testing.T) (*entorno, *personas.Servicio) {
	t.Helper()
	var ps *personas.Servicio
	e, perm, cmd := conRangos(t, nil)
	perm.Ahora = e.rel.ahora
	ps = personas.Nuevo(e.alm, 1, perm)
	ps.Ahora = e.rel.ahora
	ps.RegistrarComandos(cmd)
	_, _ = e.alm.ActualizarChat(1, pnAna, false, func(c *almacen.Chat) { c.TelE164, c.Publico = "5215511111111", "Ana" })
	_, _ = e.alm.ActualizarChat(1, grupoJID, true, func(c *almacen.Chat) { c.Guardado = "Vecinos" })
	return e, ps
}

func TestSilencioConCaducidadYBloqueoSeRespetanEnEntrar(t *testing.T) {
	e, ps := conPersonas(t)
	// Silenciada 10 minutos: el bot no le contesta (ni comandos).
	if err := ps.Silenciar(almacen.PersonaDueno, pnAna, 10*time.Minute, ""); err != nil {
		t.Fatal(err)
	}
	e.m.Entrar(priv(pnAna, "S1", "hola"))
	e.m.Entrar(priv(pnAna, "S2", "!rango"))
	e.quieto()
	if len(e.env.lista()) != 0 || e.motivos()["persona silenciada"] != 2 {
		t.Fatalf("enviados %+v motivos %v", e.env.lista(), e.motivos())
	}
	// Otra persona sí.
	e.m.Entrar(priv(pnLuis, "S3", "hola"))
	e.espera(t, func() bool { return len(e.env.lista()) == 1 })
	// Pasan los 10 minutos: vuelve a contestarle.
	e.rel.avanza(11 * time.Minute)
	s4 := priv(pnAna, "S4", "ya volví")
	s4.TS = e.rel.ahora()
	e.m.Entrar(s4)
	e.espera(t, func() bool { return len(e.env.lista()) == 2 })
	// Bloqueada: lo mismo, por el estado de solicitudes (y hasta en grupos).
	if err := ps.Bloquear(almacen.PersonaDueno, pnAna); err != nil {
		t.Fatal(err)
	}
	b1, b2 := priv(pnAna, "B1", "hola otra vez"), grupoM(grupoJID, "B2", pnAna, "@bot hola")
	b1.TS, b2.TS = e.rel.ahora(), e.rel.ahora()
	e.m.Entrar(b1)
	e.m.Entrar(b2)
	e.quieto()
	if len(e.env.lista()) != 2 || e.motivos()["persona bloqueada"] != 2 {
		t.Fatalf("enviados %+v motivos %v", e.env.lista(), e.motivos())
	}
	if _, err := ps.Desbloquear(almacen.PersonaDueno, pnAna); err != nil {
		t.Fatal(err)
	}
	b3 := priv(pnAna, "B3", "estoy de vuelta")
	b3.TS = e.rel.ahora()
	e.m.Entrar(b3)
	e.espera(t, func() bool { return len(e.env.lista()) == 3 })
}

func TestChatPausadoSoloAtiendeReanudar(t *testing.T) {
	e, ps := conPersonas(t)
	idRol, _ := ps.Perm.GuardarRol(almacen.Rol{Nombre: "Ayuda", Casillas: map[string]almacen.Alcance{"chat.pausar": {}}, LimiteMin: 100})
	_ = e.alm.AsignarRol(1, pnLuis, idRol, 0, "yo")
	// El chat del grupo está en "tomar el control": el bot calla todo...
	if err := ps.PausarChat(almacen.PersonaDueno, grupoJID, personas.DuracionControl, true); err != nil {
		t.Fatal(err)
	}
	e.m.Entrar(grupoM(grupoJID, "P1", pnAna, "!rango"))
	e.m.Entrar(grupoM(grupoJID, "P2", pnLuis, "!pausar"))
	e.m.Entrar(grupoM(grupoJID, "P3", pnLuis, "@bot hola"))
	e.quieto()
	if len(e.env.lista()) != 0 {
		t.Fatalf("pausado: %+v", e.env.lista())
	}
	// ...salvo !reanudar de quien tiene permiso (una persona sin la casilla no puede).
	e.m.Entrar(grupoM(grupoJID, "P4", pnAna, "!reanudar"))
	e.espera(t, func() bool { return len(e.env.lista()) == 1 })
	if !strings.Contains(e.env.lista()[0].texto, "No tienes permiso") || !e.alm.BotPausado(1, grupoJID, e.rel.ahora()) {
		t.Fatalf("%+v", e.env.lista())
	}
	e.m.Entrar(grupoM(grupoJID, "P5", pnLuis, "!reanudar"))
	e.espera(t, func() bool { return len(e.env.lista()) == 2 })
	if !strings.Contains(e.env.lista()[1].texto, "vuelve a contestar") || e.alm.BotPausado(1, grupoJID, e.rel.ahora()) {
		t.Fatalf("%+v", e.env.lista())
	}
	// Ya reanudado, el bot vuelve a contestar.
	e.m.Entrar(grupoM(grupoJID, "P6", pnLuis, "!rango"))
	e.espera(t, func() bool { return len(e.env.lista()) == 3 })
	// Y con el interruptor del grupo apagado (Pausado) pasa lo mismo.
	_ = e.alm.FijarBotActivo(1, otroGrupo, false)
	e.m.Entrar(grupoM(otroGrupo, "Q1", pnLuis, "!rango"))
	e.quieto()
	if len(e.env.lista()) != 3 {
		t.Fatal("grupo apagado calla")
	}
	e.m.Entrar(grupoM(otroGrupo, "Q2", pnLuis, "!reanudar"))
	e.espera(t, func() bool { return len(e.env.lista()) == 4 })
	if !e.alm.EstadoDeChat(1, otroGrupo).BotActivo {
		t.Fatal("el grupo vuelve a estar activo")
	}
}

func TestReportarPorElMotorNoContestaEnElChat(t *testing.T) {
	e, _ := conPersonas(t)
	// Alguien sin rango responde a un mensaje de Luis con !reportar: llega solo un aviso local.
	m := grupoM(grupoJID, "R1", pnAna, "!reportar insulta")
	m.CitaAutor, m.CitaID = pnLuis, "MALO"
	e.m.Entrar(m)
	e.espera(t, func() bool {
		av, _ := e.alm.AvisosRecientes(1, 5)
		return len(av) == 1
	})
	e.quieto()
	// por defecto le confirma a quien reportó (con su texto de fábrica); el aviso al dueño sigue siendo solo local
	if l := e.env.lista(); len(l) != 1 || !strings.Contains(l[0].texto, "Gracias por tu reporte, lo analizaremos. Gracias.") || l[0].chat != grupoJID {
		t.Fatalf("debía confirmar a quien reportó: %+v", l)
	}
	av, _ := e.alm.AvisosRecientes(1, 5)
	if av[0].Tipo != "reporte" || av[0].Chat != grupoJID || !strings.Contains(av[0].Detalle, "insulta") {
		t.Fatalf("%+v", av[0])
	}
	acs, _ := e.alm.ListarAcciones(1, 20, 0)
	for _, a := range acs {
		if strings.Contains(a.Detalle, "insulta") {
			t.Fatalf("el registro no lleva textos: %+v", a)
		}
	}
	if len(e.dec.lotes) != 0 {
		t.Fatal("un comando conocido no pasa por el motor de menús")
	}
}
