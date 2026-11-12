package wabot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/grupos"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// La API de silencio de grupos y de advertencias con sanciones responde JSON con la forma que espera la app (sin el bot encendido).
func TestApiSilencioYAdvertenciasConTipo(t *testing.T) {
	reinicia(t)
	if err := AbrirAlmacen(t.TempDir(), clavePrueba(), &obsFalso{}); err != nil {
		t.Fatal(err)
	}
	a, _ := almacenAbierto()
	pn := "34600000009@s.whatsapp.net"
	g1, g2 := "g1@g.us", "g2@g.us"
	_, _ = a.ActualizarChat(1, pn, false, func(c *almacen.Chat) { c.Guardado = "Ana" })
	_, _ = a.ActualizarChat(1, g1, true, func(c *almacen.Chat) { c.Guardado = "Grupo Uno" })
	_, _ = a.ActualizarMiembros(1, g1, []almacen.MiembroGrupo{{Persona: pn, JID: pn}}, time.Now())
	_, _ = a.ActualizarMiembros(1, g2, []almacen.MiembroGrupo{{Persona: pn, JID: pn}}, time.Now())

	// ajustes y textos
	j, err := AjustesSilencio(1)
	if err != nil || !strings.Contains(j, `"sil_tope":"10"`) || !strings.Contains(j, `"sil_avisos_persona_dia":"4"`) || !strings.Contains(j, "Estás silenciado; tus mensajes no se leerán") {
		t.Fatalf("%s %v", j, err)
	}
	if err := FijarAjusteSilencio(1, "sil_tope", "5"); err != nil {
		t.Fatal(err)
	}
	if err := FijarAjusteSilencio(1, "sil_tope", "mucho"); err == nil {
		t.Fatal("valor malo")
	}
	if err := FijarAjusteSilencio(1, "adv_tope", "5"); err == nil {
		t.Fatal("ese no es del silencio")
	}
	if err := FijarTextoSilencio(1, "noadmin", "Estás callado"); err != nil {
		t.Fatal(err)
	}
	if j, _ = AjustesSilencio(1); !strings.Contains(j, "Estás callado") {
		t.Fatal(j)
	}

	// silenciar / listar / contador / quitar
	j, err = SilenciarEnGrupos(1, pn, 3600, "uno", `["`+g1+`"]`, false, "")
	if err != nil || !strings.Contains(j, `"alcance":"uno"`) {
		t.Fatalf("%s %v", j, err)
	}
	if _, err := SilenciarEnGrupos(1, pn, 60, "uno", `no-json`, false, ""); err == nil {
		t.Fatal("lista de grupos mala")
	}
	if j, _ = ListarSilenciosGrupos(1); !strings.Contains(j, `"nombre"`) || !strings.Contains(j, `"contador"`) {
		t.Fatal(j)
	}
	if j, _ = ContadorSilencio(1, pn, ""); !strings.Contains(j, `"tope":5`) || !strings.Contains(j, `"color":"verde"`) {
		t.Fatal(j)
	}
	if err := PonerContadorEnCero(1, pn, ""); err != nil {
		t.Fatal(err)
	}
	ms, _ := a.MiembrosDe(1, g1, false, time.Now())
	if len(ms) != 1 || !ms[0].Silenciado {
		t.Fatalf("la lista de miembros ve el silencio: %+v", ms)
	}
	if j, _ = HistorialSilencios(1, pn); !strings.Contains(j, `"activo":true`) {
		t.Fatal(j)
	}
	// selector de grupos (sin el bot encendido no se sabe si soy admin)
	if j, _ = GruposQueAdministro(1); !strings.Contains(j, g1) || !strings.Contains(j, `"sabido":false`) || !strings.Contains(j, "Grupo Uno") {
		t.Fatal(j)
	}
	if ok, err := QuitarSilencioGrupos(1, pn); !ok || err != nil {
		t.Fatalf("%v %v", ok, err)
	}
	if ok, _ := QuitarSilencioGrupos(1, pn); ok {
		t.Fatal("ya no tenía")
	}

	// tipos y escalera
	j, err = ListarTiposAdvertencia(1)
	var tipos []almacen.TipoAdv
	if err != nil || json.Unmarshal([]byte(j), &tipos) != nil || len(tipos) != 4 || tipos[0].Nombre != "Spam" {
		t.Fatalf("%s %v", j, err)
	}
	id, err := GuardarTipoAdvertencia(1, `{"nombre":"Ruido","emoji":"🔊","caducidadDias":-1,"activo":true,"escalera":[{"nivel":1,"accion":"avisar","alcance":"grupo"}]}`)
	if err != nil || id == 0 {
		t.Fatalf("%d %v", id, err)
	}
	if _, err := GuardarTipoAdvertencia(1, `{"nombre":"Mal","activo":true,"escalera":[{"nivel":1,"accion":"matar"}]}`); err == nil {
		t.Fatal("acción mala")
	}
	if err := GuardarEscalera(1, id, `[{"nivel":1,"accion":"avisar"},{"nivel":2,"accion":"expulsar"}]`); err != nil {
		t.Fatal(err)
	}
	if j, _ = ListarEscalera(1, id); !strings.Contains(j, `"expulsar"`) {
		t.Fatal(j)
	}

	// reglas como lista
	rid, err := GuardarReglaItem(1, g1, 0, "Respeto", "Sin insultos", 0)
	if err != nil || rid == 0 {
		t.Fatalf("%d %v", rid, err)
	}
	if j, _ = ListarReglasGrupo(1, g1); !strings.Contains(j, "Respeto") {
		t.Fatal(j)
	}
	if got := a.ReglasGrupo(1, g1); got != "1. Respeto: Sin insultos" {
		t.Fatalf("!reglas sigue funcionando: %q", got)
	}

	// advertir con tipo (nivel 1 = avisar; sin el bot no hay aviso privado pero la advertencia queda)
	j, err = AdvertirConTipo(1, pn, g1, "", "ruido", id, rid)
	if err != nil || !strings.Contains(j, `"nivel":1`) || !strings.Contains(j, `"regla":"Regla 1: Respeto"`) {
		t.Fatalf("%s %v", j, err)
	}
	// nivel 2 = expulsar: queda pendiente de confirmar
	j, err = AdvertirConTipo(1, pn, g1, "", "otra vez", id, 0)
	if err != nil || !strings.Contains(j, `"hecho":"pendiente"`) {
		t.Fatalf("%s %v", j, err)
	}
	if j, _ = SancionesPendientes(1); !strings.Contains(j, `"origen":"advertencia"`) || !strings.Contains(j, "Grupo Uno") {
		t.Fatal(j)
	}
	var ps []almacen.SancionPend
	_ = json.Unmarshal([]byte(j), &ps)
	if _, err := ConfirmarSancion(1, ps[0].ID, false); err != nil {
		t.Fatal(err)
	}
	if j, _ = SancionesPendientes(1); strings.Contains(j, `"origen"`) {
		t.Fatal("ya no está pendiente")
	}
	if err := BorrarTipoAdvertencia(1, id); err != nil {
		t.Fatal(err)
	}
	if err := BorrarReglaItem(1, rid); err != nil {
		t.Fatal(err)
	}
}

// Con el bot iniciado: el enganche del trabajador borra lo de quien está silenciado, ignora la sincronización offline y no toca
// a los demás ni los privados.
func TestBotSilencioEnElTrabajador(t *testing.T) {
	reinicia(t)
	ConfigurarMotor(true, 10)
	f := &falso{}
	raiz := t.TempDir()
	if err := AbrirAlmacen(raiz, clavePrueba(), nil); err != nil {
		t.Fatal(err)
	}
	if err := Start(raiz, 1, clavePrueba(), f); err != nil {
		t.Fatal(err)
	}
	defer Stop()
	mu.Lock()
	s := st
	mu.Unlock()
	yo := types.NewJID("5215500000000", types.DefaultUserServer)
	s.cli.Store.ID = &yo
	gf := grupos.NuevoFalso()
	s.sil.Grupo = gf
	var salen []string
	var smu sync.Mutex
	s.sil.Enviar = func(_ context.Context, chat, texto string) error {
		smu.Lock()
		defer smu.Unlock()
		salen = append(salen, chat+"|"+texto)
		return nil
	}
	ana, _ := types.ParseJID(pnAna)
	luis, _ := types.ParseJID(pnLuis)
	grp := types.NewJID("120363000000000001", types.GroupServer)
	n := 0
	msg := func(de types.JID, chat types.JID, sinc bool) {
		n++
		e := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chat, Sender: de, IsGroup: chat.Server == types.GroupServer},
			ID: fmt.Sprintf("S%d", n), Timestamp: time.Now()}, RawMessage: &waE2E.Message{Conversation: proto.String("hola grupo")}}
		e.UnwrapRaw()
		s.sincronizando.Store(sinc)
		s.evento(e)
		s.sincronizando.Store(false)
	}
	espera := func(cond func() bool, msg string) {
		t.Helper()
		for i := 0; i < 250; i++ {
			if cond() {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal(msg)
	}
	if _, err := SilenciarEnGrupos(1, pnAna, 0, "todos", "", false, ""); err != nil {
		t.Fatal(err)
	}
	msg(luis, grp, false)          // otra persona: no
	msg(ana, grp, true)            // sincronización offline: no
	msg(ana, ana.ToNonAD(), false) // privado: no
	msg(ana, grp, false)
	msg(ana, grp, false)
	msg(ana, grp, false)
	espera(func() bool { return gf.NumBorrados() == 3 }, "debía borrar los 3 mensajes del grupo")
	time.Sleep(150 * time.Millisecond)
	if gf.NumBorrados() != 3 {
		t.Fatalf("solo los 3: %d", gf.NumBorrados())
	}
	for _, b := range gf.Borrados {
		if b.Chat != grp.String() {
			t.Fatalf("%+v", b)
		}
	}
	// al tercero (aviso cada 3) sale el aviso privado como respuesta
	espera(func() bool { smu.Lock(); defer smu.Unlock(); return len(salen) == 1 }, "debía avisarle a Ana al tercero")
	if c, _ := ContadorSilencio(1, pnAna, ""); !strings.Contains(c, `"usado":3`) {
		t.Fatal(c)
	}
}
