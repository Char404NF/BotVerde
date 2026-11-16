package modulos

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
)

const grupoTab = "120363000000000030@g.us"

func tablonListo(t *testing.T) (*ent, *accFalsa) {
	e, acc := entH(t)
	e.conChat(grupoTab, true, "Club")
	e.activa(IDTablon)
	e.fija("tope_hora", "200")
	e.fija("tope_dia", "200")
	_ = e.alm.FijarReglasGrupo(1, grupoTab, "Respeto ante todo", e.hora)
	return e, acc
}

func tablonDe(t *testing.T, e *ent) tablonDatos {
	t.Helper()
	fs, _ := e.alm.ListarAgenda(1, IDTablon)
	if len(fs) == 0 {
		t.Fatal("no hay tablón")
	}
	var d tablonDatos
	_ = json.Unmarshal([]byte(fs[0].Contenido), &d)
	return d
}

func TestTablonSePublicaSeFijaYLlevaLasReglas(t *testing.T) {
	e, acc := tablonListo(t)
	_, err := e.g.AccionModulo(IDTablon, "tablon.agregar", Args{"grupo": grupoTab, "titulo": "Club", "texto": "Bienvenidos", "evento": "Reunión",
		"fecha_evento": e.hora.Add(48 * time.Hour).Format("02/01/2006 15:04"), "reglas": true, "sorteo": true, "votacion": true, "dias": "7", "auto": true})
	if err != nil {
		t.Fatal(err)
	}
	e.corre()
	if len(acc.textos) != 1 {
		t.Fatalf("textos: %v", acc.textos)
	}
	for _, sub := range []string{"📌 *Club*", "Bienvenidos", "📅 *Próximo evento:* Reunión", "📜 *Reglas:*", "Respeto ante todo"} {
		if !strings.Contains(acc.textos[0], sub) {
			t.Fatalf("falta %q en %q", sub, acc.textos[0])
		}
	}
	if len(acc.fijados) != 1 || acc.fijados[0] != grupoTab+"|TXT1|7|true" {
		t.Fatalf("fijados: %v", acc.fijados)
	}
	if d := tablonDe(t, e); d.MsgID != "TXT1" || d.Hash == "" || d.Ocupado {
		t.Fatalf("estado: %+v", d)
	}
}

func TestTablonSeActualizaSoloSiCambiaYBorraElViejo(t *testing.T) {
	e, acc := tablonListo(t)
	if _, err := e.g.AccionModulo(IDTablon, "tablon.agregar", Args{"grupo": grupoTab, "titulo": "Club", "texto": "Hola", "reglas": true, "sorteo": true, "votacion": true, "auto": true}); err != nil {
		t.Fatal(err)
	}
	e.corre()
	e.simula(120)
	if len(acc.textos) != 1 {
		t.Fatalf("sin cambios no se vuelve a publicar: %v", acc.textos)
	}
	_ = e.alm.FijarReglasGrupo(1, grupoTab, "Reglas nuevas", e.hora)
	e.simula(3)
	if len(acc.textos) != 2 || !strings.Contains(acc.textos[1], "Reglas nuevas") {
		t.Fatalf("debía publicar el cambio: %v", acc.textos)
	}
	if len(acc.borrados) != 1 || acc.borrados[0] != grupoTab+"|TXT1" {
		t.Fatalf("debía borrar el anterior: %v", acc.borrados)
	}
	_ = e.alm.FijarReglasGrupo(1, grupoTab, "Otra vez nuevas", e.hora)
	e.simula(10)
	if len(acc.textos) != 2 {
		t.Fatalf("pasó menos del mínimo entre publicaciones: %v", acc.textos)
	}
	e.simula(60)
	if len(acc.textos) != 3 || !strings.Contains(acc.textos[2], "Otra vez nuevas") {
		t.Fatalf("pasado el mínimo debía publicar: %v", acc.textos)
	}
}

func TestTablonMuestraSorteoYVotacionEnCurso(t *testing.T) {
	e, _ := tablonListo(t)
	e.activa(IDVotaciones)
	_, _ = e.alm.CrearAgenda(1, almacen.AgendaFila{Modulo: IDVotaciones, Nombre: "v", Destino: grupoTab, Estado: pgActivo,
		Contenido: mustJSON(&votacionDatos{Pregunta: "¿Playa o monte?", Grupo: grupoTab, Fase: votAbierta})}, e.hora)
	_, _ = e.alm.CrearAgenda(1, almacen.AgendaFila{Modulo: IDSorteos, Nombre: "Navidad", Destino: grupoTab, Estado: pgActivo,
		Contenido: mustJSON(&sorteoDatos{Premio: "Una pizza", Grupo: grupoTab, Fase: faseReclamo})}, e.hora)
	msg, err := e.g.AccionModulo(IDTablon, "tablon.vista", Args{"grupo": grupoTab, "titulo": "Club", "sorteo": true, "votacion": true})
	if err != nil || !strings.Contains(msg, "🗳️ *Votación abierta:* ¿Playa o monte?") || !strings.Contains(msg, "🎁 *Sorteo:* Navidad — premio: Una pizza") {
		t.Fatalf("%q %v", msg, err)
	}
}

func TestTablonValidacionesYQuitar(t *testing.T) {
	e, acc := tablonListo(t)
	_, err := e.g.AccionModulo(IDTablon, "tablon.agregar", Args{"grupo": pnAna})
	esperaErr(t, err, "elige el grupo")
	_, err = e.g.AccionModulo(IDTablon, "tablon.agregar", Args{"grupo": grupoTab, "reglas": false, "sorteo": false, "votacion": false})
	esperaErr(t, err, "está vacío")
	if _, err := e.g.AccionModulo(IDTablon, "tablon.agregar", Args{"grupo": grupoTab, "texto": "Hola", "auto": false}); err != nil {
		t.Fatal(err)
	}
	e.corre()
	_, err = e.g.AccionModulo(IDTablon, "tablon.agregar", Args{"grupo": grupoTab, "texto": "Otro"})
	esperaErr(t, err, "ya tiene un tablón")
	cmd := comandoDe(t, e, IDTablon, "tablon")
	if r, _ := cmd.Ejecutar(&comandos.Contexto{Chat: grupoTab, EsGrupo: true}); !strings.Contains(r, "Hola") {
		t.Fatalf("%q", r)
	}
	fs, _ := e.alm.ListarAgenda(1, IDTablon)
	if _, err := e.g.AccionModulo(IDTablon, "tablon.quitar", Args{"id": float64(fs[0].ID)}); err != nil {
		t.Fatal(err)
	}
	e.corre()
	if len(acc.borrados) != 1 || len(acc.fijados) < 2 || !strings.HasSuffix(acc.fijados[len(acc.fijados)-1], "|false") {
		t.Fatalf("debía soltar y borrar: %v %v", acc.fijados, acc.borrados)
	}
	if r, _ := cmd.Ejecutar(&comandos.Contexto{Chat: grupoTab, EsGrupo: true}); !strings.Contains(r, "no tiene tablón") {
		t.Fatalf("%q", r)
	}
}
