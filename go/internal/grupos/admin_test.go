package grupos

import (
	"botchat/internal/almacen"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestBanearSacaYNoDejaVolver(t *testing.T) {
	e := nuevo(t, true)
	if err := e.s.Banear(ctx(), "yo", grupoJID, pnLuis, "spam"); err != nil {
		t.Fatal(err)
	}
	if !e.cli.Hubo(fmt.Sprintf("UpdateGroupParticipants|%s|remove|[%s]", jid(grupoJID), jid(pnLuis))) {
		t.Fatalf("debía expulsar: %v", e.cli.Llamadas)
	}
	if !e.alm.EstaBaneado(1, grupoJID, pnLuis) {
		t.Fatal("debía quedar baneado")
	}
	if err := e.s.Banear(ctx(), "yo", grupoJID, pnLuis, ""); err != ErrYaBaneado {
		t.Fatalf("ya baneado: %v", err)
	}
	// si vuelve a entrar, se saca otra vez
	n := len(e.cli.Llamadas)
	if !e.s.AlEntrar(ctx(), grupoJID, pnLuis) {
		t.Fatal("debía actuar")
	}
	if len(e.cli.Llamadas) == n {
		t.Fatal("no llamó a WhatsApp")
	}
	if e.s.AlEntrar(ctx(), grupoJID, pnAna) {
		t.Fatal("Ana no está baneada")
	}
	if err := e.s.Desbanear("yo", grupoJID, pnLuis); err != nil {
		t.Fatal(err)
	}
	if err := e.s.Desbanear("yo", grupoJID, pnLuis); err != ErrNoBaneado {
		t.Fatalf("%v", err)
	}
	if e.s.AlEntrar(ctx(), grupoJID, pnLuis) {
		t.Fatal("ya no está baneado")
	}
}

func TestBanearAunqueYaNoEsteEnElGrupo(t *testing.T) {
	e := nuevo(t, true)
	const fuera = "5215599999999@s.whatsapp.net"
	if err := e.s.Banear(ctx(), "yo", grupoJID, fuera, "se fue antes"); err != nil {
		t.Fatalf("debía banear aunque ya no esté: %v", err)
	}
	if !e.alm.EstaBaneado(1, grupoJID, fuera) {
		t.Fatal("no quedó baneado")
	}
}

func TestBanPorComandoPideCodigoYBaneadosLista(t *testing.T) {
	e := nuevo(t, true)
	e.rol("Mod", "grupos.banear", "grupos.horario", "grupos.ranking")
	e.da(pnAna, "Mod")
	r := e.en(pnAna, "!ban 5215522222222 vendía cosas")
	if !strings.Contains(r.Texto, "confirmación") || len(e.cod) != 1 {
		t.Fatalf("%q %d", r.Texto, len(e.cod))
	}
	if e.alm.EstaBaneado(1, grupoJID, pnLuis) {
		t.Fatal("no debía banear antes de confirmar")
	}
	txt, err := e.d.Confirmar(e.cod[0].Codigo, pnAna)
	if err != nil || !strings.Contains(txt, "no podrá volver") {
		t.Fatalf("%q %v", txt, err)
	}
	if !e.alm.EstaBaneado(1, grupoJID, pnLuis) {
		t.Fatal("debía quedar baneado")
	}
	if r := e.en(pnAna, "!baneados"); !strings.Contains(r.Texto, "Baneados (1)") || !strings.Contains(r.Texto, "vendía cosas") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.en(pnAna, "!unban 5215522222222"); !strings.Contains(r.Texto, "ya puede volver") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.en(pnAna, "!baneados"); !strings.Contains(r.Texto, "nadie baneado") {
		t.Fatalf("%q", r.Texto)
	}
	// sin la casilla no puede
	if r := e.en(pnRosa, "!baneados"); !strings.Contains(r.Texto, "No tienes permiso") {
		t.Fatalf("%q", r.Texto)
	}
	// kick es otro nombre de expulsar
	e.rol("Exp", "grupos.expulsar")
	e.da(pnLuis, "Exp")
	if r := e.en(pnLuis, "!kick 5215533333333"); !strings.Contains(r.Texto, "confirmación") {
		t.Fatalf("kick: %q", r.Texto)
	}
}

func TestHorarioPorComandoYAplicacion(t *testing.T) {
	e := nuevo(t, true)
	e.rol("Mod", "grupos.horario", "grupos.ranking")
	e.da(pnAna, "Mod")
	if r := e.en(pnAna, "!horariogrupo"); !strings.Contains(r.Texto, "no tiene horario") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.en(pnAna, "!horariogrupo 25:00 07:00"); strings.Contains(r.Texto, "Listo") {
		t.Fatalf("hora inválida: %q", r.Texto)
	}
	if r := e.en(pnAna, "!horariogrupo 22:00 07:00"); !strings.Contains(r.Texto, "se cerrará a las 22:00") {
		t.Fatalf("%q", r.Texto)
	}
	// mediodía: abierto y sin cambios (ya estaba)
	e.hora = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	e.alm.MarcarHorario(1, grupoJID, "abierto")
	n := len(e.cli.Llamadas)
	if e.s.AplicarHorarios(ctx(), e.hora) != 0 || len(e.cli.Llamadas) != n {
		t.Fatal("no debía hacer nada de día")
	}
	// 23:00: cierra una sola vez
	noche := time.Date(2026, 11, 10, 23, 0, 0, 0, time.UTC)
	if e.s.AplicarHorarios(ctx(), noche) != 1 || !e.cli.Hubo(fmt.Sprintf("SetGroupAnnounce|%s|true", jid(grupoJID))) {
		t.Fatalf("debía cerrar: %v", e.cli.Llamadas)
	}
	if len(e.env) == 0 || !strings.Contains(e.env[len(e.env)-1].texto, "se cierra") {
		t.Fatalf("debía avisar al grupo: %+v", e.env)
	}
	if e.s.AplicarHorarios(ctx(), noche.Add(time.Minute)) != 0 {
		t.Fatal("no debía cerrar dos veces")
	}
	// 07:00: abre
	if e.s.AplicarHorarios(ctx(), time.Date(2026, 11, 11, 7, 0, 0, 0, time.UTC)) != 1 || !e.cli.Hubo(fmt.Sprintf("SetGroupAnnounce|%s|false", jid(grupoJID))) {
		t.Fatalf("debía abrir: %v", e.cli.Llamadas)
	}
	if r := e.en(pnAna, "!horariogrupo off"); !strings.Contains(r.Texto, "ya no se cierra") {
		t.Fatalf("%q", r.Texto)
	}
	if e.s.AplicarHorarios(ctx(), noche.Add(24*time.Hour)) != 0 {
		t.Fatal("apagado no actúa")
	}
}

func TestHorarioSinPermisoOSinAdminNoMarcaHecho(t *testing.T) {
	e := nuevo(t, false) // el bot no es administrador
	_ = e.alm.GuardarHorario(1, almacenHorario(grupoJID))
	noche := time.Date(2026, 11, 10, 23, 0, 0, 0, time.UTC)
	if e.s.AplicarHorarios(ctx(), noche) != 0 {
		t.Fatal("sin admin no puede cerrar")
	}
	if e.alm.HorarioDe(1, grupoJID).Ultimo != "" {
		t.Fatal("no debía marcarse como hecho (se reintenta)")
	}
}

func TestPerfilPorComando(t *testing.T) {
	e := nuevo(t, true)
	e.rol("Mod", "grupos.ranking")
	e.da(pnAna, "Mod")
	if r := e.en(pnAna, "!perfil 5215522222222"); !strings.Contains(r.Texto, "Sin mensajes") {
		t.Fatalf("%q", r.Texto)
	}
}

func almacenHorario(grupo string) almacen.HorarioGrupo {
	return almacen.HorarioGrupo{Grupo: grupo, Activo: true, Cierra: "22:00", Abre: "07:00", Dias: 127}
}
