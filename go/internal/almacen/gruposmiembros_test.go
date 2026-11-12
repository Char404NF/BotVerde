package almacen

import (
	"testing"
	"time"
)

func TestMiembrosYCambiosDeGrupo(t *testing.T) {
	a := nuevo(t)
	g := "g1@g.us"
	chatBasico(t, a, g, "", "Grupo", true)
	ana, beto, caro := "5215500000001@s.whatsapp.net", "5215500000002@s.whatsapp.net", "5215500000003@s.whatsapp.net"
	t0 := time.Now().Add(-48 * time.Hour)
	n, err := a.ActualizarMiembros(1, g, []MiembroGrupo{{ana, ana, RolGrupoCreador}, {beto, beto, RolGrupoMiembro}, {caro, caro, RolGrupoMiembro}}, t0)
	if err != nil || n != 3 {
		t.Fatal(n, err)
	}
	// mensajes de Ana en el grupo (contador)
	for i := 0; i < 3; i++ {
		m := msg(g, string(rune('a'+i)), time.Now().Unix()-int64(i), "x")
		m.RemitenteJID = ana
		_, _ = a.GuardarMensaje(m)
	}
	// Beto sale solo; Caro es expulsada por Ana; Dani entra por enlace; Eva la agregó Ana; Beto promovido antes de salir no importa
	dani, eva := "5215500000004@s.whatsapp.net", "5215500000005@s.whatsapp.net"
	ahora := time.Now()
	_ = a.RegistrarCambio(1, g, CambioSale, beto, beto, "", "", ahora.Add(-4*time.Hour))
	_ = a.RegistrarCambio(1, g, CambioExpulsado, caro, caro, ana, "", ahora.Add(-3*time.Hour))
	_ = a.RegistrarCambio(1, g, CambioEnlace, dani, dani, "", "invite", ahora.Add(-2*time.Hour))
	_ = a.RegistrarCambio(1, g, CambioAgregado, eva, eva, ana, "", ahora.Add(-time.Hour))
	_ = a.RegistrarCambio(1, g, CambioPromovido, eva, eva, ana, "", ahora)
	if err := a.RegistrarCambio(1, g, "raro", eva, eva, "", "", ahora); err == nil {
		t.Fatal("tipo no válido")
	}
	vig, _ := a.MiembrosDe(1, g, false, ahora)
	nombres := map[string]MiembroVista{}
	for _, m := range vig {
		nombres[m.Persona] = m
	}
	if len(vig) != 3 || nombres[beto].Persona != "" || nombres[caro].Persona != "" {
		t.Fatalf("vigentes: %+v", vig)
	}
	if nombres[ana].Rol != RolGrupoCreador || nombres[ana].Mensajes != 3 || nombres[eva].Rol != RolGrupoAdmin {
		t.Fatalf("%+v", nombres)
	}
	todos, _ := a.MiembrosDe(1, g, true, ahora)
	if len(todos) != 5 {
		t.Fatalf("todos: %d", len(todos))
	}
	cs, _ := a.ListarCambios(1, g, 0, 10)
	if len(cs) != 5 || cs[0].Tipo != CambioPromovido || cs[3].Tipo != CambioExpulsado || cs[3].Por != ana || cs[4].Por != "" || cs[4].Tipo != CambioSale {
		t.Fatalf("%+v", cs)
	}
	if p, _ := a.ListarCambios(1, g, cs[2].TS, 10); len(p) != 2 {
		t.Fatalf("paginar: %d", len(p))
	}
	// la foto completa reabre a quien vuelve y cierra a quien ya no está, sin inventar cambios
	_, _ = a.ActualizarMiembros(1, g, []MiembroGrupo{{ana, ana, RolGrupoAdmin}, {caro, caro, RolGrupoMiembro}}, ahora)
	vig, _ = a.MiembrosDe(1, g, false, ahora)
	if len(vig) != 2 {
		t.Fatalf("%+v", vig)
	}
	if cs2, _ := a.ListarCambios(1, g, 0, 10); len(cs2) != 5 {
		t.Fatal("la foto completa no debe crear cambios")
	}
	// grupos en común y grupo ausente
	if gs, _ := a.GruposEnComun(1, caro); len(gs) != 1 {
		t.Fatalf("%v", gs)
	}
	a.CerrarGruposAusentes(1, nil, ahora)
	if gs, _ := a.GruposEnComun(1, caro); len(gs) != 0 {
		t.Fatal("el bot ya no está: nadie vigente")
	}
	if cs3, _ := a.ListarCambios(1, g, 0, 10); len(cs3) != 5 {
		t.Fatal("el historial de cambios se conserva")
	}
	// marca de la última foto completa
	if a.GrupoSnapshotTS(1) != 0 {
		t.Fatal("ts")
	}
	a.FijarGrupoSnapshotTS(1, 123)
	if a.GrupoSnapshotTS(1) != 123 {
		t.Fatal("ts")
	}
}
