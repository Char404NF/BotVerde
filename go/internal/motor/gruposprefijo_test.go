package motor

import (
	"strings"
	"testing"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
)

// En un grupo el bot solo responde si lo mencionan, citan uno de sus mensajes o usan el prefijo de comandos (el configurable).
func TestEnGruposSoloRespondeConMencionCitaOPrefijo(t *testing.T) {
	e, perm, cmd := conRangos(t, nil)
	_ = cmd.Registrar(comandos.Comando{Nombre: "grupo", Ejecutar: func(*comandos.Contexto) (string, error) { return "info del grupo", nil }})
	idTodo, _ := perm.GuardarRol(almacen.Rol{Nombre: "Todo", LimiteMin: 100})
	_ = e.alm.AsignarRol(1, pnAna, idTodo, 0, "yo")
	mk := func(id, texto string, menc bool) Mensaje {
		m := grupoM(grupoJID, id, pnAna, texto)
		m.Mencionado = menc
		return m
	}
	e.m.Entrar(mk("g1", "buenos días", false)) // ni mención ni prefijo: calla
	e.m.Entrar(mk("g2", "grupo", false))       // el nombre del comando sin prefijo: calla
	e.m.Entrar(mk("g3", "!grupo", false))      // con el prefijo: contesta el comando
	e.m.Entrar(mk("g4", "hola bot", true))     // con mención: contesta el motor
	e.espera(t, func() bool { return len(e.env.lista()) == 2 })
	e.quieto()
	got := e.env.lista()
	if len(got) != 2 || e.motivos()[almacen.MotGrupo] != 2 {
		t.Fatalf("%+v %v", got, e.motivos())
	}
	ok := map[string]bool{}
	for _, g := range got {
		ok[strings.TrimSpace(g.texto[strings.Index(g.texto, " ")+1:])] = true // quita la @mención del principio
		if g.chat != grupoJID {
			t.Fatalf("%+v", g)
		}
	}
	if !ok["info del grupo"] || !ok["eco hola bot"] {
		t.Fatalf("%+v", got)
	}
	// Con otro prefijo ("#") el "!" deja de ser un comando y el "#" empieza a serlo.
	_ = e.alm.FijarAjusteBot(1, "prefijo", "#")
	e.m.Entrar(mk("g5", "!grupo", false))
	e.m.Entrar(mk("g6", "#grupo", false))
	e.espera(t, func() bool { return len(e.env.lista()) == 3 })
	e.quieto()
	if l := e.env.lista(); len(l) != 3 || !strings.Contains(l[2].texto, "info del grupo") {
		t.Fatalf("%+v", l)
	}
}
