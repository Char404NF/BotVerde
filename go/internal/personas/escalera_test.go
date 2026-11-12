package personas

import (
	"context"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/grupos"
	"botchat/internal/silencio"
)

type conEscalera struct {
	*entorno
	gf     *grupos.Falso
	sil    *silencio.Servicio
	avisos []string
}

func nuevaEscalera(t *testing.T) *conEscalera {
	e := &conEscalera{entorno: nuevoEntorno(t), gf: grupos.NuevoFalso()}
	e.alm.Reloj = func() time.Time { return e.hora }
	e.sil = silencio.Nuevo(e.alm, 1, e.perm)
	e.sil.Ahora = e.perm.Ahora
	e.sil.Grupo = e.gf
	e.s.Sil = e.sil
	e.s.Grupo = e.gf
	e.s.Avisar = func(p, txt string) (bool, string) { e.avisos = append(e.avisos, p+"|"+txt); return true, "" }
	return e
}

func (e *conEscalera) spam() int64 {
	ts, _ := e.alm.ListarTiposAdv(1, true)
	for _, t := range ts {
		if t.Nombre == "Spam" {
			return t.ID
		}
	}
	panic("sin Spam")
}

func TestEscaleraDeSpamPasoAPaso(t *testing.T) {
	e := nuevaEscalera(t)
	ctx := context.Background()
	id := e.spam()
	// 1: avisar
	r, err := e.s.AdvertirConTipo(ctx, almacen.PersonaDueno, pnAna, grupo, "", "", id, 0)
	if err != nil || r.Nivel != 1 || r.Paso != "avisar" || r.Hecho != "avisado" {
		t.Fatalf("%+v %v", r, err)
	}
	if len(e.avisos) != 1 || !strings.HasPrefix(e.avisos[0], pnAna+"|") || !strings.Contains(e.avisos[0], "Spam") {
		t.Fatalf("%v", e.avisos)
	}
	// 2: silencio de 60 min en ese grupo
	r, _ = e.s.AdvertirConTipo(ctx, almacen.PersonaDueno, pnAna, grupo, "", "", id, 0)
	if r.Nivel != 2 || r.Hecho != "silenciado" {
		t.Fatalf("%+v", r)
	}
	sil := e.sil.Vigente(pnAna, grupo, e.hora)
	if sil == nil || sil.Origen != "escalera" || sil.HastaTS != e.hora.Add(time.Hour).Unix() || e.sil.Vigente(pnAna, "otro@g.us", e.hora) != nil {
		t.Fatalf("%+v", sil)
	}
	// 3: silencio largo
	r, _ = e.s.AdvertirConTipo(ctx, almacen.PersonaDueno, pnAna, grupo, "", "", id, 0)
	if r.Nivel != 3 || r.Hecho != "silenciado" {
		t.Fatalf("%+v", r)
	}
	if sil = e.sil.Vigente(pnAna, grupo, e.hora); sil.HastaTS != e.hora.Add(10080*time.Minute).Unix() {
		t.Fatalf("una semana: %+v", sil)
	}
	// 4: expulsar SOLO con confirmación
	r, _ = e.s.AdvertirConTipo(ctx, almacen.PersonaDueno, pnAna, grupo, "", "", id, 0)
	if r.Nivel != 4 || r.Hecho != "pendiente" || r.SancionID == 0 {
		t.Fatalf("%+v", r)
	}
	if e.gf.NumExpulsados() != 0 {
		t.Fatal("no se expulsa sola")
	}
	if txt, err := e.sil.ConfirmarSancion(ctx, r.SancionID, true); err != nil || !strings.Contains(txt, "expulsé") || e.gf.NumExpulsados() != 1 {
		t.Fatalf("%q %v", txt, err)
	}
	// 5: pasado el último paso se repite el último (otra confirmación)
	r, _ = e.s.AdvertirConTipo(ctx, almacen.PersonaDueno, pnAna, grupo, "", "", id, 0)
	if r.Nivel != 5 || r.Hecho != "pendiente" {
		t.Fatalf("%+v", r)
	}
	// todo en el registro sin textos
	acs, _ := e.alm.ListarAcciones(1, 100, 0)
	for _, a := range acs {
		if strings.Contains(a.Detalle, "Spam") {
			t.Fatalf("texto en el registro: %+v", a)
		}
	}
}

func TestEscaleraCaducaYEvitaDobleSancion(t *testing.T) {
	e := nuevaEscalera(t)
	ctx := context.Background()
	id := e.spam()
	// el tope global antiguo (1 advertencia → expulsar) NO se aplica a un tipo con escalera
	_ = e.alm.FijarAjusteBot(1, "adv_tope", "1")
	_ = e.alm.FijarAjusteBot(1, "adv_accion", "expulsar")
	e.gf.PonerAdmin(grupo, true)
	r, _ := e.s.AdvertirConTipo(ctx, almacen.PersonaDueno, pnAna, grupo, "M1", "", id, 0)
	if r.LlegoAlTope || r.Tope != 0 || len(e.gf.Expulsados) != 0 {
		t.Fatalf("doble sanción: %+v", r)
	}
	// sin tipo (General) sigue valiendo el tope de siempre
	r, _ = e.s.AdvertirConTipo(ctx, almacen.PersonaDueno, pnLuis, grupo, "M2", "", 0, 0)
	if !r.LlegoAlTope || r.Hecho != "expulsado" || e.gf.NumExpulsados() != 1 {
		t.Fatalf("%+v", r)
	}
	// caducidad: a los 31 días la de Ana ya no cuenta y vuelve a empezar en el paso 1
	e.hora = e.hora.Add(31 * 24 * time.Hour)
	r, _ = e.s.AdvertirConTipo(ctx, almacen.PersonaDueno, pnAna, grupo, "", "", id, 0)
	if r.Nivel != 1 || r.Paso != "avisar" {
		t.Fatalf("%+v", r)
	}
	// «Otro»/tipos sin escalera usan el tope general
	otro, _ := e.alm.GuardarTipoAdv(1, almacen.TipoAdv{Nombre: "Sin escalera", Activo: true, CaducidadDias: -1})
	r, _ = e.s.AdvertirConTipo(ctx, almacen.PersonaDueno, pnRosa, grupo, "M3", "", otro, 0)
	if !r.LlegoAlTope {
		t.Fatalf("%+v", r)
	}
	if _, err := e.s.AdvertirConTipo(ctx, almacen.PersonaDueno, pnRosa, grupo, "", "", 9999, 0); err == nil {
		t.Fatal("tipo inexistente")
	}
}

func TestEscaleraLigadaALaReglaYEnPrivado(t *testing.T) {
	e := nuevaEscalera(t)
	ctx := context.Background()
	id := e.spam()
	rid, _ := e.alm.GuardarReglaItem(1, almacen.ReglaItem{Grupo: grupo + "x", Titulo: "x"}, e.hora)
	_ = rid
	g2 := "g2@g.us"
	rid, err := e.alm.GuardarReglaItem(1, almacen.ReglaItem{Grupo: g2, Titulo: "No spam", Texto: "Nada de publicidad"}, e.hora)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := e.s.AdvertirConTipo(ctx, almacen.PersonaDueno, pnAna, g2, "", "publicidad", id, rid)
	if r.Regla != "Regla 1: No spam" || r.ReglaID != rid {
		t.Fatalf("%+v", r)
	}
	if len(e.avisos) != 1 || !strings.Contains(e.avisos[0], "Regla 1: No spam") {
		t.Fatalf("el aviso cita la regla: %v", e.avisos)
	}
	xs, _ := e.alm.ListarAdvertenciasEn(1, pnAna, 5, e.hora)
	if xs[0].ReglaID != rid || xs[0].TipoID != id || xs[0].Motivo != "publicidad" {
		t.Fatalf("%+v", xs[0])
	}
	// el paso 2 (silenciar) en un chat privado no se puede: solo avisa al dueño
	r, _ = e.s.AdvertirConTipo(ctx, almacen.PersonaDueno, pnAna, pnAna, "", "", id, 0)
	if r.Nivel != 2 || r.Hecho != "solo_aviso" || !strings.Contains(r.Motivo, "grupos") {
		t.Fatalf("%+v", r)
	}
	// el tipo puede traer su regla por defecto
	tp := e.alm.TipoAdvPorID(1, id)
	tp.ReglaID = rid
	_, _ = e.alm.GuardarTipoAdv(1, *tp)
	r, _ = e.s.AdvertirConTipo(ctx, almacen.PersonaDueno, pnLuis, g2, "", "", id, 0)
	if r.ReglaID != rid {
		t.Fatalf("regla del tipo: %+v", r)
	}
}

func TestAdvertirPorComandoConTipo(t *testing.T) {
	e := nuevaEscalera(t)
	e.s.RegistrarComandos(e.d)
	r := e.dueno("!advertir 5215511111111 spam vendiendo cosas")
	if !strings.Contains(r.Texto, "Advertencia puesta (lleva 1") || !strings.Contains(r.Texto, "Le avisé") {
		t.Fatalf("%q", r.Texto)
	}
	xs, _ := e.alm.ListarAdvertenciasEn(1, pnAna, 5, e.hora)
	if len(xs) != 1 || xs[0].TipoID != e.spam() || xs[0].Motivo != "vendiendo cosas" {
		t.Fatalf("%+v", xs)
	}
	// sin tipo: como antes
	r = e.dueno("!advertir 5215511111111 solo un motivo")
	xs, _ = e.alm.ListarAdvertenciasEn(1, pnAna, 5, e.hora)
	if xs[0].TipoID != 0 || xs[0].Motivo != "solo un motivo" {
		t.Fatalf("%+v", xs[0])
	}
	// !advertencias solo muestra las vigentes
	e.hora = e.hora.Add(31 * 24 * time.Hour)
	if r = e.dueno("!advertencias 5215511111111"); !strings.Contains(r.Texto, "no tiene advertencias") {
		t.Fatalf("%q", r.Texto)
	}
}
