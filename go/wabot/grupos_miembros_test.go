package wabot

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// Los eventos de grupo solo se copian y encolan; luego se distingue "salió" de "lo sacaron" por quién lo hizo.
func TestCambiosDeMiembrosDesdeEventosDeGrupo(t *testing.T) {
	reinicia(t)
	raiz := t.TempDir()
	if err := AbrirAlmacen(raiz, clavePrueba(), &obsFalso{}); err != nil {
		t.Fatal(err)
	}
	if err := Start(raiz, 1, clavePrueba(), &falso{}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	s := st
	mu.Unlock()
	defer Stop()
	grp := mustJID(grpJID)
	ana, beto, caro, dani := mustJID("34600000001@s.whatsapp.net"), mustJID("34600000002@s.whatsapp.net"), mustJID("34600000003@s.whatsapp.net"), mustJID("34600000004@s.whatsapp.net")
	ts := time.Now()
	s.evento(&events.GroupInfo{JID: grp, Timestamp: ts, Join: []types.JID{ana, beto, caro}})
	s.evento(&events.GroupInfo{JID: grp, Timestamp: ts.Add(time.Second), Sender: &ana, Leave: []types.JID{beto}})      // la sacó Ana
	s.evento(&events.GroupInfo{JID: grp, Timestamp: ts.Add(2 * time.Second), Sender: &caro, Leave: []types.JID{caro}}) // salió sola
	s.evento(&events.GroupInfo{JID: grp, Timestamp: ts.Add(3 * time.Second), Leave: []types.JID{dani}})                // sin quién: salió
	s.evento(&events.GroupInfo{JID: grp, Timestamp: ts.Add(4 * time.Second), Sender: &ana, Promote: []types.JID{caro}})
	esperaHasta(t, "cambios guardados", func() bool {
		j, _ := CambiosMiembrosGrupo(1, grpJID, 0, 50)
		return strings.Count(j, `"tipo"`) == 7
	})
	var cs []struct{ Tipo, Persona, Por string }
	j, _ := CambiosMiembrosGrupo(1, grpJID, 0, 50)
	if err := json.Unmarshal([]byte(j), &cs); err != nil {
		t.Fatal(err)
	}
	cuenta := map[string]int{}
	for _, c := range cs {
		cuenta[c.Tipo]++
		if c.Tipo == "expulsado" && !strings.HasPrefix(c.Persona, "34600000002") {
			t.Fatalf("expulsado: %+v", c)
		}
		if c.Tipo == "expulsado" && !strings.HasPrefix(c.Por, "34600000001") {
			t.Fatalf("lo sacó Ana: %+v", c)
		}
	}
	if cuenta["entra"] != 3 || cuenta["expulsado"] != 1 || cuenta["sale"] != 2 || cuenta["promovido"] != 1 {
		t.Fatalf("%v", cuenta)
	}
	// vigentes: solo Ana y Caro (Caro volvió a ser miembro? no: salió) -> Ana
	m, _ := MiembrosGrupoDetallado(1, grpJID, false)
	if strings.Count(m, `"persona"`) != 1 || !strings.Contains(m, "34600000001") {
		t.Fatalf("%s", m)
	}
}

func TestApiEtiquetasMarcadosPorLotes(t *testing.T) {
	reinicia(t)
	if err := AbrirAlmacen(t.TempDir(), clavePrueba(), &obsFalso{}); err != nil {
		t.Fatal(err)
	}
	id, err := CrearEtiquetaEmoji(1, "Cliente", "#00AA00", "🤝")
	if err != nil {
		t.Fatal(err)
	}
	p := "34600000001@s.whatsapp.net"
	if err := PonerEtiquetaPersona(1, p, id, true); err != nil {
		t.Fatal(err)
	}
	j, _ := EtiquetasDePersonas(1, `["`+p+`"]`)
	if !strings.Contains(j, p) {
		t.Fatal(j)
	}
	if j, _ := ListarMarcados(1, "persona", id, 10, 0); !strings.Contains(j, p) {
		t.Fatal(j)
	}
	if j, _ := ContarMarcados(1, id); !strings.Contains(j, `"persona":1`) {
		t.Fatal(j)
	}
	if j, _ := ListarEtiquetas(1); !strings.Contains(j, `"emoji":"🤝"`) || !strings.Contains(j, `"personas":1`) {
		t.Fatal(j)
	}
	if err := MarcarMensaje(1, p, "NO", id, ""); err == nil {
		t.Fatal("mensaje inexistente")
	}
}
