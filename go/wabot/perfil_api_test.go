package wabot

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
)

// La API de perfil, actividad, lista, búsqueda, puntitos y exportar responde JSON con la forma que espera la app.
func TestApiPerfilActividadBusquedaYExportar(t *testing.T) {
	reinicia(t)
	raiz := t.TempDir()
	if err := AbrirAlmacen(raiz, clavePrueba(), &obsFalso{}); err != nil {
		t.Fatal(err)
	}
	a, _ := almacenAbierto()
	pn := "34600000009@s.whatsapp.net"
	_, _ = a.ActualizarChat(1, grpJID, true, func(c *almacen.Chat) { c.Guardado = "Grupo P" })
	_, _ = a.ActualizarChat(1, pn, false, func(c *almacen.Chat) { c.Guardado = "Ana" })
	now := time.Now().Unix() - 100
	for i, x := range []string{"hola mundo", "Adiós Camión", "otra cosa"} {
		if _, err := a.GuardarMensaje(&almacen.Mensaje{ID: "M" + string(rune('1'+i)), ChatJID: grpJID, BotID: 1, RemitenteJID: pn, RemitenteNombre: "Ana", Tipo: "texto", Texto: x, TS: now + int64(i)}); err != nil {
			t.Fatal(err)
		}
	}
	et, _ := a.CrearEtiquetaEmoji(1, "VIP", "#00FF00", "⭐")
	_ = a.PonerEtiquetaPersona(1, pn, et, true)

	j, err := PerfilPersona(1, pn)
	var perfil struct {
		Persona  string `json:"persona"`
		Mensajes int    `json:"mensajes"`
		Nombre   string `json:"nombre"`
	}
	if err != nil || json.Unmarshal([]byte(j), &perfil) != nil || perfil.Persona != pn || perfil.Mensajes != 3 || perfil.Nombre != "Ana" {
		t.Fatalf("%s %v", j, err)
	}
	j, err = ActividadPersona(1, pn)
	if err != nil || !strings.Contains(j, `"privado":true`) || !strings.Contains(j, grpJID) {
		t.Fatalf("%s %v", j, err)
	}
	j, err = MensajesDePersona(1, `{"chat":"`+grpJID+`","persona":"`+pn+`","pestana":"mensajes","palabra":"camion"}`, 0, "", 20)
	if err != nil || !strings.Contains(j, `"id":"M2"`) || strings.Contains(j, `"id":"M1"`) || !strings.Contains(j, `"marcaRoja":""`) {
		t.Fatalf("%s %v", j, err)
	}
	if _, err := MensajesDePersona(1, `{no es json`, 0, "", 20); err == nil {
		t.Fatal("JSON malo")
	}
	j, err = BuscarEnChat(1, grpJID, "COSA", 0, 0, 0, "", 20)
	if err != nil || !strings.Contains(j, `"M3"`) {
		t.Fatalf("%s %v", j, err)
	}
	j, err = PuntitosDeMensajes(1, grpJID, `["M1","M2"]`)
	if err != nil || !strings.Contains(j, `"M1":[`) {
		t.Fatalf("%s %v", j, err)
	}
	j, err = ExportarChat(1, grpJID, "txt", 0, 0)
	if err != nil || !strings.Contains(j, `"mensajes":3`) || !strings.Contains(j, `.txt.bcf`) {
		t.Fatalf("%s %v", j, err)
	}
}
