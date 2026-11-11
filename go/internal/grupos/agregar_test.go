package grupos

import (
	"errors"
	"fmt"
	"testing"

	"go.mau.fi/whatsmeow/types"
)

const pnNueva = "5215577777777@s.whatsapp.net"

func TestAgregarMiembroAccionRealYResultados(t *testing.T) {
	e := nuevo(t, true)
	g := jid(grupoJID)
	r, err := e.s.AgregarMiembro(ctx(), grupoJID, pnNueva)
	if err != nil || r != AgregadoOK {
		t.Fatalf("agregar: %q %v", r, err)
	}
	if !e.cli.Hubo(fmt.Sprintf("UpdateGroupParticipants|%s|add|[%s]", g, jid(pnNueva))) {
		t.Fatalf("debió usar la acción real de agregar: %v", e.cli.Llamadas)
	}
	// ya estaba: no se llama a WhatsApp
	antes := len(e.cli.Llamadas)
	r, err = e.s.AgregarMiembro(ctx(), grupoJID, pnLuis)
	if err != nil || r != AgregadoYaEsta {
		t.Fatalf("ya estaba: %q %v", r, err)
	}
	for _, l := range e.cli.Llamadas[antes:] {
		if len(l) > 22 && l[:22] == "UpdateGroupParticipant" {
			t.Fatal("no se agrega a quien ya está")
		}
	}
	// privacidad: WhatsApp responde 403 sin avisar
	e.cli.Resultado = []types.GroupParticipant{{JID: jid(pnNueva), Error: 403, AddRequest: &types.GroupParticipantAddRequest{Code: "X"}}}
	r, err = e.s.AgregarMiembro(ctx(), grupoJID, pnNueva)
	if err != nil || r != AgregadoPrivacidad {
		t.Fatalf("privacidad: %q %v", r, err)
	}
	e.cli.Resultado = []types.GroupParticipant{{JID: jid(pnNueva), Error: 408}}
	if r, _ = e.s.AgregarMiembro(ctx(), grupoJID, pnNueva); r != AgregadoPrivacidad {
		t.Fatalf("salió hace poco: %q", r)
	}
	e.cli.Resultado = []types.GroupParticipant{{JID: jid(pnNueva), Error: 500}}
	if _, err = e.s.AgregarMiembro(ctx(), grupoJID, pnNueva); err == nil {
		t.Fatal("un código raro es un error")
	}
	// sin token de privacidad la librería falla y se trata como privacidad
	e.cli.Resultado = nil
	e.cli.Errores["UpdateGroupParticipants"] = errors.New("failed to get privacy token for participant x")
	if r, err = e.s.AgregarMiembro(ctx(), grupoJID, pnNueva); err != nil || r != AgregadoPrivacidad {
		t.Fatalf("sin token: %q %v", r, err)
	}
	e.cli.Errores["UpdateGroupParticipants"] = errors.New("boom")
	if _, err = e.s.AgregarMiembro(ctx(), grupoJID, pnNueva); err == nil {
		t.Fatal("otro error falla")
	}
}

func TestAgregarMiembroExigeSerAdmin(t *testing.T) {
	e := nuevo(t, false)
	if _, err := e.s.AgregarMiembro(ctx(), grupoJID, pnNueva); !errors.Is(err, ErrNoAdmin) {
		t.Fatalf("sin ser admin: %v", err)
	}
	if !e.cli.SinEscrituras() {
		t.Fatal("no debe tocar el grupo")
	}
	if _, err := e.s.AgregarMiembro(ctx(), grupoJID, "no-es-jid"); err == nil {
		t.Fatal("persona inválida")
	}
}
