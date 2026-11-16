package modulos

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
)

const grupoVot = "120363000000000020@g.us"

func votacionLista(t *testing.T) (*ent, *accFalsa) {
	e, acc := entH(t)
	e.conChat(grupoVot, true, "Club")
	e.activa(IDVotaciones)
	e.fija("tope_hora", "200")
	e.fija("tope_dia", "200")
	_, _ = e.alm.ActualizarMiembros(1, grupoVot, []almacen.MiembroGrupo{
		{Persona: pnAna, JID: pnAna, Rol: almacen.RolGrupoMiembro}, {Persona: pnLuis, JID: pnLuis, Rol: almacen.RolGrupoMiembro},
	}, e.hora)
	return e, acc
}

func crearVotacion(t *testing.T, e *ent, extra Args) {
	t.Helper()
	a := Args{"nombre": "Cena", "grupo": grupoVot, "pregunta": "¿Dónde cenamos?", "opciones": "Tacos\nPizza\nSushi", "horas": 2.0}
	for k, v := range extra {
		a[k] = v
	}
	if _, err := e.g.AccionModulo(IDVotaciones, "votaciones.agregar", a); err != nil {
		t.Fatal(err)
	}
	e.corre()
}

func votDe(t *testing.T, e *ent) (almacen.AgendaFila, votacionDatos) {
	t.Helper()
	fs, _ := e.alm.ListarAgenda(1, IDVotaciones)
	if len(fs) == 0 {
		t.Fatal("no hay votación")
	}
	var d votacionDatos
	_ = json.Unmarshal([]byte(fs[0].Contenido), &d)
	return fs[0], d
}

func TestVotacionSeEnviaComoEncuestaYSeAbre(t *testing.T) {
	e, acc := votacionLista(t)
	crearVotacion(t, e, nil)
	if len(acc.encuestas) != 1 || acc.encuestas[0] != grupoVot+"|¿Dónde cenamos?|Tacos,Pizza,Sushi" {
		t.Fatalf("encuestas: %v", acc.encuestas)
	}
	f, d := votDe(t, e)
	if d.Fase != votAbierta || d.PollID != "POLL1" || string(d.Secreto) != "secreto-POLL1" || f.Estado != pgActivo {
		t.Fatalf("estado: %s %+v", f.Estado, d)
	}
	if string(e.g.SecretoVotacion("POLL1")) != "secreto-POLL1" || e.g.SecretoVotacion("OTRA") != nil {
		t.Fatal("el secreto debía poder consultarse")
	}
}

func TestVotacionCuentaVotosCambiosYRetiros(t *testing.T) {
	e, _ := votacionLista(t)
	crearVotacion(t, e, nil)
	voto := func(p string, ops ...string) bool {
		var hs [][]byte
		for _, o := range ops {
			hs = append(hs, HashOpcion(o))
		}
		return e.g.VotoEncuesta(grupoVot, "POLL1", p, hs)
	}
	if voto(pnAna, "Tacos") && !voto(pnLuis, "Pizza") {
		t.Fatal("debía aceptar los votos")
	}
	if e.g.VotoEncuesta(grupoVot, "AJENA", pnAna, nil) || e.g.VotoEncuesta("otro@g.us", "POLL1", pnAna, nil) {
		t.Fatal("no debía aceptar votos de otra encuesta u otro chat")
	}
	voto(pnLuis, "Tacos") // cambia de opinión
	_, d := votDe(t, e)
	por, n := d.conteo(e.g)
	if n != 2 || por[0] != 2 || por[1] != 0 {
		t.Fatalf("conteo: %v %d", por, n)
	}
	voto(pnAna) // retira su voto
	_, d = votDe(t, e)
	if _, n := d.conteo(e.g); n != 1 {
		t.Fatalf("tras retirar: %d", n)
	}
}

func TestVotacionSeCierraSolaYPublicaElResultado(t *testing.T) {
	e, _ := votacionLista(t)
	crearVotacion(t, e, nil)
	e.g.VotoEncuesta(grupoVot, "POLL1", pnAna, [][]byte{HashOpcion("Pizza")})
	e.g.VotoEncuesta(grupoVot, "POLL1", pnLuis, [][]byte{HashOpcion("Pizza")})
	e.simula(100)
	if e.hay("Faltan") && !e.hay("Ya votaron 2") {
		t.Fatalf("aviso antes de cerrar: %v", e.enviados())
	}
	e.simula(25)
	f, d := votDe(t, e)
	if d.Fase != votCerrada || f.Estado != pgTerminado {
		t.Fatalf("debía cerrar: %s %+v", f.Estado, d.Fase)
	}
	if !e.hay("📊 Resultado de la votación") || !e.hay("🏆 Pizza — 2 (100%)") || !e.hay("Votaron 2 personas") {
		t.Fatalf("resultado: %v", e.enviados())
	}
	if !strings.Contains(strings.Join(e.avisos, "|"), "Votación cerrada") {
		t.Fatalf("avisos: %v", e.avisos)
	}
}

func TestVotacionLimitadaARangosIgnoraLosDemas(t *testing.T) {
	e, _ := votacionLista(t)
	socio, _ := e.alm.CrearRol(1, almacen.Rol{Nombre: "Socio", Posicion: 5, Casillas: map[string]almacen.Alcance{}})
	_ = e.alm.AsignarRol(1, pnAna, socio, 0, "yo")
	crearVotacion(t, e, Args{"rango1": itoa64(socio)})
	e.g.VotoEncuesta(grupoVot, "POLL1", pnAna, [][]byte{HashOpcion("Tacos")})
	e.g.VotoEncuesta(grupoVot, "POLL1", pnLuis, [][]byte{HashOpcion("Sushi")})
	_, d := votDe(t, e)
	por, n := d.conteo(e.g)
	if n != 1 || por[0] != 1 || por[2] != 0 {
		t.Fatalf("solo cuenta Ana: %v %d", por, n)
	}
}

func TestVotacionSeRepiteYSoloVaEnGrupos(t *testing.T) {
	e, _ := votacionLista(t)
	_, err := e.g.AccionModulo(IDVotaciones, "votaciones.agregar", Args{"nombre": "X", "grupo": pnAna, "pregunta": "¿?", "opciones": "a\nb"})
	esperaErr(t, err, "solo funcionan en grupos")
	_, err = e.g.AccionModulo(IDVotaciones, "votaciones.agregar", Args{"nombre": "X", "grupo": grupoVot, "pregunta": "¿?", "opciones": "una sola"})
	esperaErr(t, err, "2 a 12 opciones")
	crearVotacion(t, e, Args{"horas": 1.0, "repeticion": "diaria", "fecha": e.hora.Add(5 * time.Minute).Format("02/01/2006 15:04")})
	if _, d := votDe(t, e); d.Fase != votProgramada {
		t.Fatalf("aún programada: %s", d.Fase)
	}
	e.simula(8)
	if _, d := votDe(t, e); d.Fase != votAbierta {
		t.Fatalf("debía abrirse: %s", d.Fase)
	}
	e.simula(70)
	f, d := votDe(t, e)
	if d.Fase != votProgramada || f.Estado != pgActivo || f.ProximoTS <= e.hora.Unix() {
		t.Fatalf("debía quedar lista la siguiente: %s %s %d", f.Estado, d.Fase, f.ProximoTS)
	}
}

func TestVotacionCerrarAMano(t *testing.T) {
	e, _ := votacionLista(t)
	crearVotacion(t, e, Args{"horas": 0.0})
	e.g.VotoEncuesta(grupoVot, "POLL1", pnAna, [][]byte{HashOpcion("Sushi")})
	f, d := votDe(t, e)
	if d.Cierre != 0 || f.ProximoTS != 0 {
		t.Fatalf("sin cierre automático: %+v %d", d, f.ProximoTS)
	}
	msg, err := e.g.AccionModulo(IDVotaciones, "votaciones.cerrar", Args{"id": float64(f.ID)})
	if err != nil || !strings.Contains(msg, "Sushi") {
		t.Fatalf("%q %v", msg, err)
	}
	e.corre()
	if !e.hay("🏆 Sushi — 1 (100%)") {
		t.Fatalf("resultado: %v", e.enviados())
	}
	cmd := comandoDe(t, e, IDVotaciones, "votaciones")
	if r, _ := cmd.Ejecutar(&comandos.Contexto{Chat: grupoVot, EsGrupo: true}); !strings.Contains(r, "No hay votaciones abiertas") {
		t.Fatalf("%q", r)
	}
}
