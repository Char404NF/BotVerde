package motor

import (
	"strings"
	"testing"

	"botchat/internal/modulos"
)

const ana = "5215511111111@s.whatsapp.net"

// BAJA / STOP / NO MÁS en privado: entra a la lista No molestar, se le confirma UNA sola vez y el texto no llega a los menús.
func TestBajaConfirmaUnaVezYNoSigueAlMenu(t *testing.T) {
	var g *modulos.Gestor
	e := nuevoEntorno(t, func(c *Config) {
		g = modulos.NuevoGestor(c.Alm, 1, nil)
		c.Baja = g.Baja
	})
	e.m.Entrar(priv(ana, "1", "BAJA"))
	e.espera(t, func() bool { return len(e.env.lista()) == 1 })
	if !strings.Contains(e.env.lista()[0].texto, "ya no te escribiré") || e.env.lista()[0].cita != "1" {
		t.Fatalf("confirmación: %+v", e.env.lista())
	}
	if !e.alm.EstaNoMolestar(1, ana) {
		t.Fatal("debe estar en la lista")
	}
	// Segunda vez (con acento y signos): se consume sin volver a escribirle.
	e.m.Entrar(priv(ana, "2", "¡Stop!"))
	e.m.Entrar(priv(ana, "3", "no más"))
	e.quieto()
	if len(e.env.lista()) != 1 {
		t.Fatalf("la confirmación es una sola vez: %+v", e.env.lista())
	}
	if len(e.dec.lotes) != 0 {
		t.Fatal("la baja no debe llegar al motor de menús")
	}
	// Un texto normal sí sigue su camino.
	e.m.Entrar(priv(ana, "4", "hola"))
	e.espera(t, func() bool { return len(e.env.lista()) == 2 })
	if e.env.lista()[1].texto != "eco hola" {
		t.Fatalf("texto normal: %+v", e.env.lista())
	}
	// En un grupo, "baja" no es una baja.
	e.m.Entrar(grupoM("g@g.us", "5", "5215522222222@s.whatsapp.net", "baja"))
	e.quieto()
	if e.alm.EstaNoMolestar(1, "5215522222222@s.whatsapp.net") {
		t.Fatal("en grupos no")
	}
}

// Con el motor apagado la baja igual se anota (y no se contesta nada).
func TestBajaConElMotorApagadoSeAnotaSinResponder(t *testing.T) {
	e := nuevoEntorno(t, func(c *Config) {
		g := modulos.NuevoGestor(c.Alm, 1, nil)
		c.Baja = g.Baja
		c.Activo = func() bool { return false }
	})
	e.m.Entrar(priv(ana, "1", "STOP"))
	e.quieto()
	if !e.alm.EstaNoMolestar(1, ana) {
		t.Fatal("debe quedar en la lista aunque el motor esté apagado")
	}
	if len(e.env.lista()) != 0 {
		t.Fatal("apagado no responde")
	}
}
