package almacen

import (
	"testing"
	"time"
)

func TestConteosMiembrosPorTipo(t *testing.T) {
	a := nuevo(t)
	g := "g1@g.us"
	chatBasico(t, a, g, "", "Grupo", true)
	ana, beto := "5215500000001@s.whatsapp.net", "5215500000002@s.whatsapp.net"
	ahora := time.Now().Unix()
	poner := func(id, de, tipo string, nota, borrado, editado bool, ts int64) {
		m := msg(g, id, ts, "x")
		m.RemitenteJID, m.Tipo, m.NotaVoz, m.Borrado, m.Editado = de, tipo, nota, borrado, editado
		if _, err := a.GuardarMensaje(m); err != nil {
			t.Fatal(err)
		}
	}
	poner("1", ana, TipoTexto, false, false, false, ahora-500)
	poner("2", ana, TipoTexto, false, false, true, ahora-400)
	poner("3", ana, TipoImagen, false, false, false, ahora-300)
	poner("4", ana, TipoAudio, true, false, false, ahora-200)
	poner("5", ana, TipoAudio, false, true, false, ahora-100)
	poner("6", ana, TipoVideo, false, false, false, ahora-50)
	poner("7", ana, TipoSticker, false, false, false, ahora-40)
	poner("8", ana, TipoDocumento, false, false, false, ahora-30)
	poner("9", beto, TipoTexto, false, false, false, ahora-10)
	mio := msg(g, "10", ahora, "del bot")
	mio.EsMio, mio.RemitenteJID = true, ana
	_, _ = a.GuardarMensaje(mio)

	c := a.ConteosMiembros(1, g)
	x := c[a.ResolverPersona(1, ana)]
	if x == nil || x.Mensajes != 8 || x.Texto != 2 || x.Imagenes != 1 || x.NotasVoz != 1 || x.Audios != 1 || x.Videos != 1 || x.Stickers != 1 || x.Documentos != 1 {
		t.Fatalf("ana: %+v", x)
	}
	if x.Borrados != 1 || x.Editados != 1 || x.UltimoTS != ahora-30 || x.PrimerTS != ahora-500 {
		t.Fatalf("ana tiempos: %+v", x)
	}
	if b := c[a.ResolverPersona(1, beto)]; b == nil || b.Mensajes != 1 {
		t.Fatalf("beto: %+v", b)
	}
	if y := a.ConteoPersona(1, g, ana); y.Mensajes != 8 || y.NotasVoz != 1 {
		t.Fatalf("persona: %+v", y)
	}
	if ks := OrdenarConteos(c); len(ks) != 2 || ks[0] != a.ResolverPersona(1, ana) {
		t.Fatalf("orden: %v", ks)
	}
}

func TestBaneadosYHorario(t *testing.T) {
	a := nuevo(t)
	g := "g1@g.us"
	ana, beto := "5215500000001@s.whatsapp.net", "5215500000002@s.whatsapp.net"
	ahora := time.Now()
	if a.EstaBaneado(1, g, ana) {
		t.Fatal("no debía estar baneada")
	}
	if err := a.Banear(1, g, ana, PersonaDueno, "spam de ventas", ahora); err != nil {
		t.Fatal(err)
	}
	if !a.EstaBaneado(1, g, ana) || a.EstaBaneado(1, g, beto) || a.EstaBaneado(1, "otro@g.us", ana) {
		t.Fatal("baneo por grupo y persona")
	}
	l, _ := a.ListarBaneados(1, g)
	if len(l) != 1 || l[0].Motivo != "spam de ventas" || l[0].Telefono != "+5215500000001" {
		t.Fatalf("%+v", l)
	}
	if ok, _ := a.Desbanear(1, g, ana); !ok || a.EstaBaneado(1, g, ana) {
		t.Fatal("desbanear")
	}
	if ok, _ := a.Desbanear(1, g, ana); ok {
		t.Fatal("ya no había baneo")
	}

	if err := a.GuardarHorario(1, HorarioGrupo{Grupo: g, Activo: true, Cierra: "25:00", Abre: "07:00"}); err == nil {
		t.Fatal("hora inválida")
	}
	if err := a.GuardarHorario(1, HorarioGrupo{Grupo: g, Activo: true, Cierra: "22:00", Abre: "07:00", Dias: 127}); err != nil {
		t.Fatal(err)
	}
	if h := a.HorarioDe(1, g); !h.Activo || h.Cierra != "22:00" || len(a.HorariosActivos(1)) != 1 {
		t.Fatalf("%+v", h)
	}
	if h := a.HorarioDe(1, "nuevo@g.us"); h.Activo || h.Dias != 127 {
		t.Fatalf("por defecto: %+v", h)
	}
}

func TestEstadoDeseadoDelHorario(t *testing.T) {
	h := HorarioGrupo{Activo: true, Cierra: "22:00", Abre: "07:00", Dias: 127}
	en := func(s string) time.Time { x, _ := time.Parse("2006-01-02 15:04", s); return x }
	// 2026-11-16 es lunes
	casos := map[string]string{"2026-11-16 23:00": "cerrado", "2026-11-17 03:00": "cerrado", "2026-11-17 07:00": "abierto", "2026-11-17 12:00": "abierto", "2026-11-16 21:59": "abierto"}
	for cuando, quiero := range casos {
		if got := h.EstadoDeseado(en(cuando)); got != quiero {
			t.Errorf("%s: %q, quería %q", cuando, got, quiero)
		}
	}
	// solo lunes (bit 0): el martes de madrugada sigue cerrado (cerró el lunes); el martes por la noche no cierra
	l := HorarioGrupo{Activo: true, Cierra: "22:00", Abre: "07:00", Dias: 1}
	if l.EstadoDeseado(en("2026-11-17 03:00")) != "cerrado" || l.EstadoDeseado(en("2026-11-17 23:00")) != "abierto" {
		t.Error("días marcados")
	}
	// tramo dentro del mismo día
	d := HorarioGrupo{Activo: true, Cierra: "13:00", Abre: "15:00", Dias: 127}
	if d.EstadoDeseado(en("2026-11-16 14:00")) != "cerrado" || d.EstadoDeseado(en("2026-11-16 16:00")) != "abierto" {
		t.Error("tramo diurno")
	}
	if (HorarioGrupo{Cierra: "22:00", Abre: "07:00"}).EstadoDeseado(en("2026-11-16 23:00")) != "" {
		t.Error("apagado no decide")
	}
	// serie por día
	a := nuevo(t)
	g := "g1@g.us"
	chatBasico(t, a, g, "", "Grupo", true)
	m := msg(g, "1", time.Now().Unix(), "x")
	m.RemitenteJID = "5215500000001@s.whatsapp.net"
	_, _ = a.GuardarMensaje(m)
	s, horas := a.SerieMiembro(1, g, m.RemitenteJID, 7, time.Now())
	tot, th := 0, 0
	for _, p := range s {
		tot += p.N
	}
	for _, n := range horas {
		th += n
	}
	if len(s) != 7 || tot != 1 || th != 1 {
		t.Fatalf("serie %d total %d horas %d", len(s), tot, th)
	}
}
