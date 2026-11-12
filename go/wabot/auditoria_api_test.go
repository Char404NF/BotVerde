package wabot

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"botchat/internal/almacen"
)

func TestApiAuditoriaDeExtremoAExtremo(t *testing.T) {
	a := conChats(t)
	au := decodifica[almacen.AjustesAuditoria](t, func() (string, error) { return AuditoriaAjustes(1) })
	if !au.Defecto || au.RetDias != 90 || au.RetPendiente || au.AvisoVisto {
		t.Fatalf("%+v", au)
	}
	if p, err := RetencionPendiente(1); err != nil || p {
		t.Fatal(p, err)
	}
	if err := FijarAvisoAuditoriaVisto(1, true); err != nil {
		t.Fatal(err)
	}
	if err := FijarAuditoria(1, false, 30); err != nil {
		t.Fatal(err)
	}
	au = decodifica[almacen.AjustesAuditoria](t, func() (string, error) { return AuditoriaAjustes(1) })
	if au.Defecto || au.RetDias != 30 || !au.AvisoVisto {
		t.Fatalf("%+v", au)
	}
	if FijarAuditoria(1, true, 15) == nil {
		t.Fatal("retención inválida")
	}
	if err := FijarAuditoriaChat(1, pnAna, 1, 90); err != nil {
		t.Fatal(err)
	}
	c := decodifica[almacen.AuditoriaDeChat](t, func() (string, error) { return AuditoriaChat(1, pnAna) })
	if c.Audit != 1 || !c.Efectivo || c.RetEfectiva != 90 {
		t.Fatalf("%+v", c)
	}
	l := decodifica[[]almacen.AuditoriaDeChat](t, func() (string, error) { return ListarAuditoriaChats(1, "", 10, 0) })
	if len(l) != 2 {
		t.Fatalf("%+v", l)
	}
	// ediciones
	_, _ = a.EditarTexto(1, pnAna, "A1", "cambiado")
	v := decodifica[[]almacen.VersionMensaje](t, func() (string, error) { return VersionesDeMensaje(1, pnAna, "A1") })
	if len(v) != 1 || v[0].Texto != "hola, soy Ana" {
		t.Fatalf("%+v", v)
	}
	// retención pendiente: no toca nada
	_ = a.FijarAjusteBot(1, "audit_ret_pendiente", "1")
	var r almacen.ResultadoRetencion
	s, err := LimpiarPorRetencion(1)
	if err != nil || json.Unmarshal([]byte(s), &r) != nil || !r.Pendiente {
		t.Fatalf("%s %v", s, err)
	}
	if p, _ := RetencionPendiente(1); !p {
		t.Fatal("debía estar pendiente")
	}
	// borrar todo de una persona
	if _, err := BorrarTodoDePersona(1, pnLuis); err != nil {
		t.Fatal(err)
	}
	if m, _ := a.ObtenerMensaje(1, pnLuis, "L1"); m != nil {
		t.Fatal("se debía borrar todo")
	}
}

func TestPasadaDeAuditoriaOlvidaYRellena(t *testing.T) {
	a := conChats(t)
	old := time.Now().AddDate(0, 0, -200).Unix()
	_, _ = a.GuardarMensaje(&almacen.Mensaje{ID: "V1", ChatJID: pnAna, BotID: 1, RemitenteJID: pnAna, Tipo: almacen.TipoTexto, Texto: "viejo", TS: old})
	s := &estado{botID: 1, l: &falso{}}
	s.pasadaAuditoria(context.Background())
	if m, _ := a.ObtenerMensaje(1, pnAna, "V1"); m.Texto != "" || !m.Olvidado {
		t.Fatalf("%+v", m)
	}
	if a.AjusteBot(1, "sha_relleno_fin") != "1" {
		t.Fatal("el relleno de huellas debía terminar")
	}
}
