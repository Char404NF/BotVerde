package wabot

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/modulos"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Un voto real (cifrado con la librería) de una encuesta que mandó el módulo Votaciones se cuenta; uno de otra encuesta no.
func TestVotoDeUnaVotacionSeCuentaConElVotoRealDeWhatsApp(t *testing.T) {
	reinicia(t)
	raiz := t.TempDir()
	if err := AbrirAlmacen(raiz, clavePrueba(), nil); err != nil {
		t.Fatal(err)
	}
	if err := Start(raiz, 1, clavePrueba(), &falso{}); err != nil {
		t.Fatal(err)
	}
	defer Stop()
	mu.Lock()
	s := st
	mu.Unlock()
	yo := types.NewJID("5215500000000", types.DefaultUserServer)
	s.cli.Store.ID = &yo
	grupoJ := types.NewJID("120363000000000040", types.GroupServer)
	anaJ, _ := types.ParseJID("5215511111111@s.whatsapp.net")
	a, _ := almacenAbierto()
	_, _ = a.ActualizarChat(1, grupoJ.String(), true, func(c *almacen.Chat) { c.Guardado = "Club" })
	_, _ = a.ActualizarChat(1, anaJ.String(), false, func(c *almacen.Chat) { c.Publico = "Ana"; c.TelE164 = "5215511111111" })
	if err := s.mods.Activar(modulos.IDVotaciones); err != nil {
		t.Fatal(err)
	}

	secreto := make([]byte, 32)
	for i := range secreto {
		secreto[i] = byte(i + 7)
	}
	s.cli.Store.MsgSecrets = &memSecretos{}
	_ = s.cli.Store.MsgSecrets.PutMessageSecret(context.Background(), grupoJ, yo, "POLLV", secreto)
	d := map[string]any{"pregunta": "¿Dónde?", "opciones": []string{"Tacos", "Pizza"}, "grupo": grupoJ.String(), "fase": "abierta", "pollId": "POLLV", "secreto": secreto, "votos": map[string][]int{}}
	b, _ := json.Marshal(d)
	if _, err := a.CrearAgenda(1, almacen.AgendaFila{Modulo: modulos.IDVotaciones, Nombre: "Cena", Destino: grupoJ.String(), Contenido: string(b), Estado: "activo"}, time.Now()); err != nil {
		t.Fatal(err)
	}

	votan := whatsmeow.NewClient(&store.Device{ID: &anaJ, LID: types.NewJID("99887766", types.HiddenUserServer), MsgSecrets: &memSecretos{}}, waLog.Noop)
	_ = votan.Store.MsgSecrets.PutMessageSecret(context.Background(), grupoJ, yo, "POLLV", secreto)
	vm, err := votan.BuildPollVote(context.Background(), &types.MessageInfo{MessageSource: types.MessageSource{Chat: grupoJ, Sender: yo, IsGroup: true}, ID: "POLLV"}, []string{"Pizza"})
	if err != nil {
		t.Fatal(err)
	}
	ev := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: grupoJ, Sender: anaJ, IsGroup: true}, ID: "VOTOV1", Timestamp: time.Now(), PushName: "Ana"}, Message: vm, RawMessage: vm}
	s.evento(ev)
	deadline := time.Now().Add(5 * time.Second)
	cuenta := func() (int, int) {
		fs, _ := a.ListarAgenda(1, modulos.IDVotaciones)
		var x struct {
			Votos map[string][]int `json:"votos"`
		}
		_ = json.Unmarshal([]byte(fs[0].Contenido), &x)
		por := 0
		for _, v := range x.Votos {
			for _, o := range v {
				if o == 1 {
					por++
				}
			}
		}
		return len(x.Votos), por
	}
	for time.Now().Before(deadline) {
		if n, _ := cuenta(); n == 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if n, pizza := cuenta(); n != 1 || pizza != 1 {
		t.Fatalf("el voto por Pizza debía contarse: votantes=%d pizza=%d", n, pizza)
	}
	// un voto de otra encuesta se ignora
	vm2, _ := votan.BuildPollVote(context.Background(), &types.MessageInfo{MessageSource: types.MessageSource{Chat: grupoJ, Sender: yo, IsGroup: true}, ID: "POLLV"}, []string{"Tacos"})
	_ = vm2
	if s.mods.SecretoVotacion("OTRA") != nil {
		t.Fatal("no debía conocer otra encuesta")
	}
}
