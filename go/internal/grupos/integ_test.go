package grupos_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/grupos"
	"botchat/internal/permisos"
	"botchat/internal/personas"

	"go.mau.fi/whatsmeow/types"
)

const (
	gr   = "120363000000000009@g.us"
	yo   = "5215500000001@s.whatsapp.net"
	luis = "5215522222222@s.whatsapp.net"
)

func j(s string) types.JID { x, _ := types.ParseJID(s); return x }

// Las advertencias de la etapa B, al llegar al tope, usan el AccionesGrupo REAL (grupos.Servicio) contra un cliente falso.
func TestAdvertenciasAlTopeUsanLasAccionesRealesDeGrupo(t *testing.T) {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 5)
	}
	for _, soyAdmin := range []bool{true, false} {
		alm, err := almacen.Abrir(t.TempDir(), k)
		if err != nil {
			t.Fatal(err)
		}
		perm, _ := permisos.Nuevo(alm, 1)
		perm.EsPropio = func(x string) bool { return x == yo }
		cli := &grupos.ClienteFalso{Conexion: true, PN: j(yo), Errores: map[string]error{}}
		cli.Info = &types.GroupInfo{JID: j(gr), GroupName: types.GroupName{Name: "G"}, Participants: []types.GroupParticipant{
			{JID: j(yo), PhoneNumber: j(yo), IsAdmin: soyAdmin},
			{JID: j(luis), PhoneNumber: j(luis)},
		}}
		gs := grupos.Nuevo(alm, 1, cli, perm)
		var borrados []string
		gs.Borrar = func(_ context.Context, chat, id string) error { borrados = append(borrados, chat+"|"+id); return nil }
		_, _ = alm.ActualizarChat(1, gr, true, func(c *almacen.Chat) { c.Guardado = "G" })
		if _, err := alm.GuardarMensaje(&almacen.Mensaje{ID: "FEO", ChatJID: gr, BotID: 1, RemitenteJID: luis, Tipo: almacen.TipoTexto, Texto: "x", TS: time.Now().Unix()}); err != nil {
			t.Fatal(err)
		}
		ps := personas.Nuevo(alm, 1, perm)
		ps.Grupo = gs
		_ = alm.FijarAjusteBot(1, "adv_tope", "2")

		// Tope con acción borrar.
		_ = alm.FijarAjusteBot(1, "adv_accion", "borrar")
		if _, err := ps.Advertir(context.Background(), "yo", luis, gr, "FEO", "uno"); err != nil {
			t.Fatal(err)
		}
		r, err := ps.Advertir(context.Background(), "yo", luis, gr, "FEO", "dos")
		if err != nil || !r.LlegoAlTope {
			t.Fatalf("%+v %v", r, err)
		}
		if soyAdmin {
			if r.Hecho != "borrado" || len(borrados) != 1 || borrados[0] != gr+"|FEO" {
				t.Fatalf("admin: %+v %v", r, borrados)
			}
		} else if r.Hecho != "solo_aviso" || !strings.Contains(r.Motivo, "no soy administrador") || len(borrados) != 0 {
			t.Fatalf("no admin: %+v %v", r, borrados)
		}
		// Tope con acción expulsar.
		_ = alm.FijarAjusteBot(1, "adv_accion", "expulsar")
		r, err = ps.Advertir(context.Background(), "yo", luis, gr, "FEO", "tres")
		if err != nil {
			t.Fatal(err)
		}
		quiere := "UpdateGroupParticipants|" + j(gr).String() + "|remove|[" + j(luis).String() + "]"
		if soyAdmin {
			if r.Hecho != "expulsado" || !cli.Hubo(quiere) {
				t.Fatalf("admin: %+v %v", r, cli.Llamadas)
			}
		} else if r.Hecho != "solo_aviso" || !cli.SinEscrituras() {
			t.Fatalf("no admin: %+v %v", r, cli.Llamadas)
		}
		// El dueño siempre recibe la notificación local.
		av, _ := alm.AvisosRecientes(1, 20)
		n := 0
		for _, a := range av {
			if a.Tipo == "advertencias" {
				n++
			}
		}
		if n < 2 {
			t.Fatalf("avisos: %+v", av)
		}
		// Un error de WhatsApp al expulsar no rompe: se alerta.
		if soyAdmin {
			cli.Errores["UpdateGroupParticipants"] = grupos.ErrNoAdmin
			r, _ = ps.Advertir(context.Background(), "yo", luis, gr, "FEO", "cuatro")
			if r.Hecho != "solo_aviso" {
				t.Fatalf("%+v", r)
			}
		}
		_ = alm.Cerrar()
	}
}
