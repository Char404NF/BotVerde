package modulos

import (
	"errors"
	"strings"
	"testing"
	"time"

	"botchat/internal/comandos"
)

func invitarListo(t *testing.T) (*ent, *accFalsa) {
	e, acc := entH(t)
	e.conChat(grupo, true, "Vecinos")
	e.conChat(pnAna, false, "Ana")
	e.activa(IDInvitar)
	e.fija("tope_hora", "100")
	e.fija("tope_dia", "100")
	return e, acc
}

func TestInvitarPedirEntrarYEnlace(t *testing.T) {
	e, _ := invitarListo(t)
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: grupo, EsGrupo: true, Persona: pnAna, Texto: "quiero entrar"})
	if len(mustMiembros(e.g, ListaQuierenEntrar)) != 0 {
		t.Fatal("en grupos no cuenta")
	}
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: pnAna, Persona: pnAna, Texto: "Quiero entrar!"})
	if m := mustMiembros(e.g, ListaQuierenEntrar); len(m) != 1 || m[0] != pnAna {
		t.Fatalf("lista: %v", m)
	}
	if !tiene(e.avisos, "Alguien quiere entrar a un grupo") {
		t.Fatalf("avisos: %v", e.avisos)
	}
	e.simula(1)
	if !e.hay("Anotado") {
		t.Fatalf("confirmación: %v", e.enviados())
	}
	m, err := e.g.AccionModulo(IDInvitar, "invitar.agregar", Args{"grupo": grupo, "persona": pnAna})
	if err != nil || !strings.Contains(m, "1 invitaciones") {
		t.Fatalf("invitar: %q %v", m, err)
	}
	e.simula(3)
	if !e.hay(pnAna + "|Hola Ana, aquí está la invitación al grupo *Vecinos*: https://chat.whatsapp.com/ABCDEF") {
		t.Fatalf("enviados: %v", e.enviados())
	}
	if len(mustMiembros(e.g, ListaQuierenEntrar)) != 0 {
		t.Fatal("tras invitar sale de la lista")
	}
}

func TestInvitarNoEscribeANadieSinChatNiNoMolestar(t *testing.T) {
	e, _ := invitarListo(t)
	_, err := e.g.AccionModulo(IDInvitar, "anotar.agregar", Args{"persona": pnLuis})
	esperaErr(t, err, "ya te escribió")
	e.conChat(pnLuis, false, "Luis")
	_ = e.g.AgregarNoMolestar(pnLuis, "baja")
	_, err = e.g.AccionModulo(IDInvitar, "invitar.agregar", Args{"grupo": grupo, "persona": pnLuis})
	esperaErr(t, err, "no recibir")
	// sin ser admin el enlace falla y se explica
	e2, acc := invitarListo(t)
	acc.errEnlace = errors.New("solo los administradores pueden ver o reiniciar el enlace del grupo")
	_, _ = e2.alm.AgregarALista(1, ListaQuierenEntrar, pnAna, "a mano", e2.hora)
	_, err = e2.g.AccionModulo(IDInvitar, "invitar.agregar", Args{"grupo": grupo, "persona": pnAna})
	esperaErr(t, err, "administradores")
	if e2.cuantos() != 0 {
		t.Fatal("nada salió")
	}
}

func TestInvitarAgregarDirectoPideAceptarYSoloListaYTope(t *testing.T) {
	e, acc := invitarListo(t)
	_, _ = e.alm.AgregarALista(1, ListaQuierenEntrar, pnAna, "palabra", e.hora)
	_, err := e.g.AccionModulo(IDInvitar, "invitar.directo", Args{"grupo": grupo, "persona": pnAna})
	esperaErr(t, err, "apagado")
	b, _ := jsonDe(map[string]any{"directo": true})
	esperaErr(t, e.g.GuardarConfig(IDInvitar, b), "Entiendo el riesgo")
	e.config(IDInvitar, map[string]any{"directo": true, "directo_acepto": true, "directo_tope_dia": 2.0})
	// alguien que no la pidió
	e.conChat(pnLuis, false, "Luis")
	_, err = e.g.AccionModulo(IDInvitar, "invitar.directo", Args{"grupo": grupo, "persona": pnLuis})
	esperaErr(t, err, "Quieren entrar")
	// ahora sí
	if _, err := e.g.AccionModulo(IDInvitar, "invitar.directo", Args{"grupo": grupo, "persona": pnAna}); err != nil {
		t.Fatal(err)
	}
	e.avanza(10 * time.Second)
	if len(acc.agregados) != 0 {
		t.Fatal("antes de la espera de 30 s no se agrega")
	}
	e.simula(2)
	if len(acc.agregados) != 1 || acc.agregados[0] != grupo+"|"+pnAna {
		t.Fatalf("agregados: %v", acc.agregados)
	}
	if len(mustMiembros(e.g, ListaQuierenEntrar)) != 0 {
		t.Fatal("al quedar dentro sale de la lista")
	}
	if !tiene(e.resultados(), "enviado") {
		t.Fatal("queda en el registro")
	}
	// tope por día = 2
	for _, p := range []string{"5215500000011@s.whatsapp.net", "5215500000012@s.whatsapp.net", "5215500000013@s.whatsapp.net"} {
		e.conChat(p, false, p)
		_, _ = e.alm.AgregarALista(1, ListaQuierenEntrar, p, "palabra", e.hora)
	}
	hecho := 0
	var ultimo error
	for _, p := range []string{"5215500000011@s.whatsapp.net", "5215500000012@s.whatsapp.net", "5215500000013@s.whatsapp.net"} {
		if err := e.g.agregarDirecto(grupo, p); err == nil {
			hecho++
		} else {
			ultimo = err
		}
	}
	if hecho != 1 {
		t.Fatalf("ya había 1 hoy, caben 2: aceptó %d", hecho)
	}
	esperaErr(t, ultimo, "tope de 2")
}

func TestInvitarAgregarDirectoCaeAInvitacionPorPrivacidad(t *testing.T) {
	e, acc := invitarListo(t)
	e.config(IDInvitar, map[string]any{"directo": true, "directo_acepto": true})
	acc.resAgregar = AgregPrivacidad
	_, _ = e.alm.AgregarALista(1, ListaQuierenEntrar, pnAna, "palabra", e.hora)
	if err := e.g.agregarDirecto(grupo, pnAna); err != nil {
		t.Fatal(err)
	}
	e.simula(5)
	if !e.hay("https://chat.whatsapp.com/ABCDEF") {
		t.Fatalf("debía mandarse la invitación: %v", e.enviados())
	}
	if !tiene(e.resultados(), "respaldo_invitacion") {
		t.Fatalf("registro: %v", e.resultados())
	}
}

func TestInvitarErrorAlAgregarNoCuentaSiYaNoEsta(t *testing.T) {
	e, acc := invitarListo(t)
	e.config(IDInvitar, map[string]any{"directo": true, "directo_acepto": true})
	acc.errAgregar = errors.New("falló")
	_, _ = e.alm.AgregarALista(1, ListaQuierenEntrar, pnAna, "palabra", e.hora)
	_ = e.g.agregarDirecto(grupo, pnAna)
	e.simula(5)
	if !tiene(e.resultados(), "fallido") {
		t.Fatalf("registro: %v", e.resultados())
	}
	if !e.alm.EstaEnLista(1, ListaQuierenEntrar, pnAna) {
		t.Fatal("si falla, sigue en la lista")
	}
}

func TestInvitarComandoEnGrupoYValidaciones(t *testing.T) {
	e, _ := invitarListo(t)
	m, _ := Obtener(IDInvitar)
	var cmd *comandos.Comando
	for _, c := range m.(ComandosDeGestor).ComandosG(e.g) {
		c := c
		cmd = &c
	}
	if cmd == nil || cmd.Casilla != "invitar.usar" || cmd.Ambito != comandos.Grupo {
		t.Fatalf("comando: %+v", cmd)
	}
	if _, err := cmd.Ejecutar(&comandos.Contexto{Chat: grupo, EsGrupo: true}); err == nil {
		t.Fatal("sin a quién")
	}
	if _, err := cmd.Ejecutar(&comandos.Contexto{Chat: grupo, EsGrupo: true, Menciones: []string{pnAna}}); err != nil {
		t.Fatal(err)
	}
	e.simula(3)
	if !e.hay(pnAna + "|Hola Ana") {
		t.Fatalf("enviados: %v", e.enviados())
	}
	b, _ := jsonDe(map[string]any{"texto_invitacion": "sin enlace"})
	esperaErr(t, e.g.GuardarConfig(IDInvitar, b), "{enlace}")
	b, _ = jsonDe(map[string]any{"palabra": "stop"})
	esperaErr(t, e.g.GuardarConfig(IDInvitar, b), "no sirve")
	_ = time.Second
}
