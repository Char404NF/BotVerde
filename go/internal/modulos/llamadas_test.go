package modulos

import (
	"errors"
	"strings"
	"testing"
)

func llamadasLista(t *testing.T, modo string) (*ent, *accFalsa) {
	e, acc := entH(t)
	e.conChat(pnAna, false, "Ana")
	e.activa(IDLlamadas)
	e.config(IDLlamadas, map[string]any{"modo": modo})
	e.fija("tope_hora", "100")
	e.fija("tope_dia", "100")
	return e, acc
}

func llama(e *ent, p string, id string, video bool) {
	e.g.Evento(Evento{Tipo: EvLlamada, Chat: p, Persona: p, LlamadaID: id, LlamadaDe: p, LlamadaVideo: video})
}

func TestLlamadasNaceApagadoBajoRiesgoYDiceQueNoHaceLlamadas(t *testing.T) {
	e, _ := entH(t)
	i, _ := e.g.InfoDe(IDLlamadas)
	if i.Activo || i.Riesgo != RiesgoBajo || !strings.Contains(i.Descripcion, "NO hace ni contesta llamadas") {
		t.Fatalf("info: %+v", i)
	}
	if e.g.Config(IDLlamadas).Texto("modo") != "avisar" {
		t.Fatal("por defecto solo avisa")
	}
}

func TestLlamadasIgnorarYAvisarSoloAnotan(t *testing.T) {
	for _, modo := range []string{"ignorar", "avisar"} {
		e, acc := llamadasLista(t, modo)
		e.config(IDLlamadas, map[string]any{"avisar": modo == "avisar"})
		llama(e, pnAna, "C1", false)
		e.simula(1)
		if len(acc.rechazadas) != 0 || e.cuantos() != 0 {
			t.Fatalf("%s: no debe rechazar ni escribir", modo)
		}
		if (len(e.avisos) == 1) != (modo == "avisar") {
			t.Fatalf("%s: avisos %v", modo, e.avisos)
		}
		fs, _ := e.g.FilasPanel(IDLlamadas, "perdidas")
		if len(fs) != 1 || !strings.Contains(fs[0].Titulo, "Ana") {
			t.Fatalf("registro de perdidas: %v", fs)
		}
	}
}

func TestLlamadasRechazarPasaPorLaColaYRespetaMaestroYSeguro(t *testing.T) {
	e, acc := llamadasLista(t, "rechazar")
	llama(e, pnAna, "C1", true)
	e.simula(1)
	if len(acc.rechazadas) != 1 || acc.rechazadas[0] != pnAna+"|C1" {
		t.Fatalf("rechazadas: %v", acc.rechazadas)
	}
	if e.cuantos() != 0 {
		t.Fatal("rechazar solo no escribe")
	}
	fs, _ := e.g.FilasPanel(IDLlamadas, "perdidas")
	if len(fs) != 1 || !strings.Contains(fs[0].Titulo, "Videollamada") {
		t.Fatalf("filas: %v", fs)
	}
	// maestro apagado: no rechaza
	e.fija("maestro", "0")
	llama(e, pnAna, "C2", false)
	e.simula(1)
	if len(acc.rechazadas) != 1 {
		t.Fatal("con la Iniciativa propia apagada no se rechaza")
	}
	// modo seguro: el módulo no recibe eventos
	e.fija("maestro", "1")
	e.g.ModoSeguro("errores")
	llama(e, pnAna, "C3", false)
	e.simula(1)
	if len(acc.rechazadas) != 1 {
		t.Fatal("en modo seguro no se rechaza")
	}
}

func TestLlamadasRechazarNoEsBloqueadoPorNoMolestarNiPorLaNoche(t *testing.T) {
	e, acc := llamadasLista(t, "rechazar_responder")
	_ = e.g.AgregarNoMolestar(pnAna, "baja")
	e.fija("noche_on", "1")
	e.fija("noche_desde", "11:00")
	e.fija("noche_hasta", "13:00")
	llama(e, pnAna, "C1", false)
	e.simula(1)
	if len(acc.rechazadas) != 1 {
		t.Fatalf("debe rechazar aunque esté en No molestar o de noche: %v", acc.rechazadas)
	}
	if e.cuantos() != 0 {
		t.Fatal("pero no le escribe (No molestar y pausa nocturna)")
	}
}

func TestLlamadasRechazarYResponderUnaVezPorPeriodo(t *testing.T) {
	e, acc := llamadasLista(t, "rechazar_responder")
	llama(e, pnAna, "C1", false)
	e.simula(1)
	if len(acc.rechazadas) != 1 || !e.hay(pnAna+"|Hola, no puedo contestar llamadas") {
		t.Fatalf("rechazadas %v enviados %v", acc.rechazadas, e.enviados())
	}
	llama(e, pnAna, "C2", false)
	e.simula(1)
	if len(acc.rechazadas) != 2 || e.cuantos() != 1 {
		t.Fatalf("segunda llamada: se rechaza pero no se vuelve a responder (%d, %d)", len(acc.rechazadas), e.cuantos())
	}
	e.hora = e.hora.Add(25 * 3600 * 1e9)
	llama(e, pnAna, "C3", false)
	e.simula(1)
	if e.cuantos() != 2 {
		t.Fatalf("pasado el periodo vuelve a responder: %v", e.enviados())
	}
}

func TestLlamadasDesconocidosNoRecibenMensajePorDefecto(t *testing.T) {
	e, acc := llamadasLista(t, "rechazar_responder")
	llama(e, pnLuis, "X1", false)
	e.simula(1)
	if len(acc.rechazadas) != 1 || e.cuantos() != 0 {
		t.Fatalf("rechaza pero no escribe a quien nunca habló: %v %v", acc.rechazadas, e.enviados())
	}
	e.config(IDLlamadas, map[string]any{"desconocidos": true})
	e.hora = e.hora.Add(1 * 3600 * 1e9)
	llama(e, pnLuis, "X2", false)
	e.simula(1)
	if e.cuantos() != 1 {
		t.Fatalf("con la opción, sí responde: %v", e.enviados())
	}
}

func TestLlamadasErrorAlRechazarCuentaComoFallo(t *testing.T) {
	e, acc := llamadasLista(t, "rechazar")
	acc.errRechazar = errors.New("sin conexión")
	llama(e, pnAna, "C1", false)
	e.simula(1)
	if got := e.resultados(); !strings.Contains(strings.Join(got, ","), "fallido") {
		t.Fatalf("resultados: %v", got)
	}
	e.g.Acc = nil
	llama(e, pnAna, "C2", false)
	e.simula(1)
}
