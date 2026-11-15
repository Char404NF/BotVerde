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
	// mediodía, recién encendido: el horario nunca cerró el grupo, así que no abre ni avisa; solo lo anota
	e.hora = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	n, avisos := len(e.cli.Llamadas), len(e.env)
	if e.s.AplicarHorarios(ctx(), e.hora) != 0 || len(e.cli.Llamadas) != n || len(e.env) != avisos {
		t.Fatal("no debía hacer nada de día")
	}
	if e.alm.HorarioDe(1, grupoJID).Ultimo != "abierto" {
		t.Fatal("debía anotar que está abierto")
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

func TestLimpiarInactivosSacaSoloALosNoAdminsConConfirmacion(t *testing.T) {
	e := nuevo(t, true)
	e.rol("Mod", "grupos.expulsar")
	e.da(pnAna, "Mod")
	r := e.en(pnAna, "!limpiarinactivos 30")
	if !strings.Contains(r.Texto, "confirmación") || len(e.cod) != 1 {
		t.Fatalf("%q", r.Texto)
	}
	for _, l := range e.cli.Llamadas {
		if strings.HasPrefix(l, "UpdateGroupParticipants") {
			t.Fatalf("no debía sacar a nadie antes de confirmar: %v", e.cli.Llamadas)
		}
	}
	txt, err := e.d.Confirmar(e.cod[0].Codigo, pnAna)
	if err != nil || !strings.Contains(txt, "saqué a") {
		t.Fatalf("%q %v", txt, err)
	}
	// Luis y Rosa no han escrito: salen. Ana (admin), Carla (creó el grupo) y el bot se quedan.
	for _, p := range []string{pnLuis, lidRosa} {
		if !e.cli.Hubo(fmt.Sprintf("UpdateGroupParticipants|%s|remove|[%s]", jid(grupoJID), jid(p))) {
			t.Fatalf("debía sacar a %s: %v", p, e.cli.Llamadas)
		}
	}
	for _, p := range []string{pnAna, pnCarla, pnYo} {
		if e.cli.Hubo(fmt.Sprintf("UpdateGroupParticipants|%s|remove|[%s]", jid(grupoJID), jid(p))) {
			t.Fatalf("no debía sacar a %s", p)
		}
	}
}

func TestRaidCierraElGrupoYAvisaUnaSolaVez(t *testing.T) {
	e := nuevo(t, true)
	if err := e.alm.GuardarEntrada(1, almacen.ProteccionEntrada{Grupo: grupoJID, RaidActivo: true, RaidMax: 3, RaidMinutos: 2}); err != nil {
		t.Fatal(err)
	}
	if e.s.AlEntrarRevisa(ctx(), grupoJID, "5219900000001@s.whatsapp.net") != "" || e.s.AlEntrarRevisa(ctx(), grupoJID, "5219900000002@s.whatsapp.net") != "" {
		t.Fatal("con 2 entradas aún no hay alarma")
	}
	if e.s.AlEntrarRevisa(ctx(), grupoJID, "5219900000003@s.whatsapp.net") != "raid" {
		t.Fatal("la tercera debía disparar la alarma")
	}
	if !e.cli.Hubo(fmt.Sprintf("SetGroupAnnounce|%s|true", jid(grupoJID))) {
		t.Fatalf("debía cerrar el grupo: %v", e.cli.Llamadas)
	}
	if len(e.env) == 0 || !strings.Contains(e.env[len(e.env)-1].texto, "Por seguridad cerré el grupo") {
		t.Fatalf("debía avisar en el grupo: %+v", e.env)
	}
	// la siguiente ráfaga dentro de 10 minutos no repite el cierre ni el aviso
	n := len(e.env)
	for i := 4; i < 8; i++ {
		e.s.AlEntrarRevisa(ctx(), grupoJID, fmt.Sprintf("521990000000%d@s.whatsapp.net", i))
	}
	if len(e.env) != n {
		t.Fatal("no debía avisar otra vez")
	}
	av, _ := e.alm.AvisosRecientes(1, 10)
	hay := false
	for _, a := range av {
		if strings.Contains(a.Titulo, "ataque") {
			hay = true
		}
	}
	if !hay {
		t.Fatalf("debía dejar un aviso local: %+v", av)
	}
}

func TestRaidFueraDeLaVentanaYApagadoNoActuan(t *testing.T) {
	e := nuevo(t, true)
	_ = e.alm.GuardarEntrada(1, almacen.ProteccionEntrada{Grupo: grupoJID, RaidActivo: true, RaidMax: 3, RaidMinutos: 2})
	e.s.AlEntrarRevisa(ctx(), grupoJID, "5219900000001@s.whatsapp.net")
	e.s.AlEntrarRevisa(ctx(), grupoJID, "5219900000002@s.whatsapp.net")
	e.hora = e.hora.Add(5 * time.Minute) // ya pasó la ventana
	if e.s.AlEntrarRevisa(ctx(), grupoJID, "5219900000003@s.whatsapp.net") != "" {
		t.Fatal("fuera de la ventana no es raid")
	}
	_ = e.alm.GuardarEntrada(1, almacen.ProteccionEntrada{Grupo: grupoJID, RaidActivo: false, RaidMax: 2, RaidMinutos: 2})
	for i := 0; i < 5; i++ {
		if e.s.AlEntrarRevisa(ctx(), grupoJID, fmt.Sprintf("521880000000%d@s.whatsapp.net", i)) != "" {
			t.Fatal("apagado no actúa")
		}
	}
}

func TestPrefijoDePaisBloqueadoSacaAlEntrar(t *testing.T) {
	e := nuevo(t, true)
	_ = e.alm.GuardarEntrada(1, almacen.ProteccionEntrada{Grupo: grupoJID, Prefijos: []string{"+212", "91", "abc", "123456"}})
	if got := e.alm.EntradaDe(1, grupoJID).Prefijos; len(got) != 2 || got[0] != "212" || got[1] != "91" {
		t.Fatalf("prefijos limpios: %v", got)
	}
	if e.s.AlEntrarRevisa(ctx(), grupoJID, pnLuis) != "" {
		t.Fatal("un número de México no está bloqueado")
	}
	if e.s.AlEntrarRevisa(ctx(), grupoJID, "212600000000@s.whatsapp.net") != "prefijo" {
		t.Fatal("debía sacar a un +212")
	}
	if e.s.AlEntrarRevisa(ctx(), grupoJID, "99999999@lid") != "" {
		t.Fatal("un LID no tiene país conocido: no se actúa")
	}
}

// El horario usa la hora del teléfono (su zona), no la de UTC: a las 23:00 de México ya son las 05:00 UTC del día siguiente.
func TestHorarioUsaLaZonaDelTelefono(t *testing.T) {
	e := nuevo(t, true)
	if err := e.alm.FijarZona("America/Mexico_City"); err != nil {
		t.Skip("sin base de zonas horarias")
	}
	_ = e.alm.GuardarHorario(1, almacen.HorarioGrupo{Grupo: grupoJID, Activo: true, Cierra: "22:00", Abre: "07:00", Dias: 127})
	e.alm.MarcarHorario(1, grupoJID, "abierto")
	// 04:30 UTC = 22:30 en México (UTC-6 en noviembre): ya es hora de cerrar aunque en UTC sea de madrugada
	if e.s.AplicarHorarios(ctx(), time.Date(2026, 11, 11, 4, 30, 0, 0, time.UTC)) != 1 || !e.cli.Hubo(fmt.Sprintf("SetGroupAnnounce|%s|true", jid(grupoJID))) {
		t.Fatalf("debía cerrar a las 22:30 de México: %v", e.cli.Llamadas)
	}
}
