package wabot

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/modulos"
	"botchat/internal/permisos"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func TestModulosHRegistradosApagadosYConEsquema(t *testing.T) {
	abreSolo(t)
	lista := decodifica[[]modulos.Info](t, func() (string, error) { return ListarModulos(1) })
	riesgos := map[string]string{"bienvenida": "medio", "difusion": "alto", "programados": "medio", "invitar": "medio", "descargas": "medio",
		"traductor": "bajo", "ia": "medio", "llamadas": "bajo", "juegos": "bajo"}
	vistos := map[string]modulos.Info{}
	for _, i := range lista {
		vistos[i.ID] = i
	}
	for id, r := range riesgos {
		i, ok := vistos[id]
		if !ok {
			t.Fatalf("falta el módulo %s", id)
		}
		if i.Activo || i.Codigo != "apagado" || i.Riesgo != r || !i.ConEsquema {
			t.Fatalf("%s: %+v", id, i)
		}
	}
	if !vistos["difusion"].Iniciativa || vistos["descargas"].Iniciativa || vistos["traductor"].Iniciativa || !vistos["ia"].Iniciativa || !vistos["llamadas"].Iniciativa || !vistos["juegos"].Iniciativa {
		t.Fatal("difusión, IA, llamadas y juegos escriben por la cola; descargas y traductor solo responden")
	}
	// los paneles viajan en la información de cada módulo
	info := decodifica[modulos.Info](t, func() (string, error) { return InfoModulo(1, "difusion") })
	ids := []string{}
	for _, p := range info.Paneles {
		ids = append(ids, p.ID)
	}
	if strings.Join(ids, ",") != "listas,difusiones" {
		t.Fatalf("paneles: %v", ids)
	}
	// las casillas de los módulos están en el catálogo para asignarlas a rangos antes de encenderlos
	for _, c := range []string{"bienvenida.exento", "difusion.ver", "programados.ver", "invitar.usar", "descargas.bajar", "descargas.conectar", "traductor.usar", "ia.usar", "juegos.jugar"} {
		x, ok := permisos.Buscar(c)
		if !ok || x.Modulo == "" {
			t.Fatalf("la casilla %s no está en el catálogo", c)
		}
	}
}

func TestApiPanelesAccionesYAlarma(t *testing.T) {
	_, a := abreSolo(t)
	const ana = "5215511111111@s.whatsapp.net"
	_, _ = a.ActualizarChat(1, ana, false, func(c *almacen.Chat) { c.Guardado = "Ana" })
	if p, err := ProximoModulos(1); err != nil || p != 0 {
		t.Fatalf("sin nada agendado: %d %v", p, err)
	}
	// agregar a una lista con el JSON que manda la app
	m, err := AccionModulo(1, "difusion", "listas.agregar", `{"lista":"suscritos","persona":"`+ana+`"}`)
	if err != nil || !strings.Contains(m, "suscritos") {
		t.Fatalf("agregar: %q %v", m, err)
	}
	if _, err := AccionModulo(1, "difusion", "listas.agregar", `{no es json`); err == nil {
		t.Fatal("JSON roto")
	}
	if _, err := AccionModulo(1, "nadie", "x", `{}`); err == nil {
		t.Fatal("módulo inexistente")
	}
	filas := decodifica[[]modulos.Fila](t, func() (string, error) { return FilasPanelModulo(1, "difusion", "listas") })
	if len(filas) != 1 || filas[0].ID != "suscritos" || !strings.Contains(filas[0].Detalle, "1") {
		t.Fatalf("filas: %+v", filas)
	}
	// programar algo: la alarma de Android se calcula con ProximoModulos (solo con el módulo encendido)
	cuando := time.Now().Add(3 * time.Hour).Truncate(time.Minute)
	_, err = AccionModulo(1, "programados", "programados.agregar", mustJSON(map[string]any{"nombre": "Aviso", "destino": ana, "texto": "Hola", "fecha": cuando.In(a.Zona()).Format("02/01/2006 15:04")}))
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := ProximoModulos(1); p != 0 {
		t.Fatalf("apagado no hay alarma: %d", p)
	}
	if err := ActivarModulo(1, "programados"); err != nil {
		t.Fatal(err)
	}
	if p, _ := ProximoModulos(1); p != cuando.Unix() {
		t.Fatalf("alarma: %d vs %d", p, cuando.Unix())
	}
	if err := TickModulos(1); err != nil {
		t.Fatal("sin bot en marcha no hace nada, sin error")
	}
	// lo hecho por el dueño queda en el registro de acciones, sin textos
	acc, _ := a.ListarAcciones(1, 50, 0)
	n := 0
	for _, x := range acc {
		if x.Accion == "modulo.accion" {
			n++
			if strings.Contains(x.Objetivo+x.Detalle, "Hola") {
				t.Fatal("el registro de acciones lleva texto")
			}
		}
	}
	if n < 2 {
		t.Fatalf("acciones registradas: %d", n)
	}
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

// Con el bot en marcha: alguien entra a un grupo, el módulo de bienvenida encola el saludo (con mención) y la cola lo guarda para
// salir con su espera; apagado el maestro no encola nada.
func TestBotBienvenidaPorEventoDeGrupo(t *testing.T) {
	s := arrancaBot(t)
	grupo := types.NewJID("120363000000000009", types.GroupServer)
	_, _ = alm.ActualizarChat(1, grupo.String(), true, func(c *almacen.Chat) { c.Guardado = "Vecinos" })
	if _, err := AccionModulo(1, "bienvenida", "grupos.agregar", `{"chat":"`+grupo.String()+`"}`); err != nil {
		t.Fatal(err)
	}
	if err := ActivarModulo(1, "bienvenida"); err != nil {
		t.Fatal(err)
	}
	nuevo := types.NewJID("5215588888888", types.DefaultUserServer)
	s.evento(&events.GroupInfo{JID: grupo, Join: []types.JID{nuevo}})
	time.Sleep(300 * time.Millisecond)
	if s.mods.Cola.Pendientes() != 0 {
		t.Fatal("con «Iniciativa propia» apagada no se encola nada")
	}
	_ = FijarIniciativaMaestro(1, true)
	s.evento(&events.GroupInfo{JID: grupo, Join: []types.JID{nuevo}})
	esperaHasta(t, "el saludo no llegó a la cola", func() bool { return s.mods.Cola.Pendientes() == 1 })
	reg, _ := alm.ListarRegistroModulos(1, "bienvenida", 10, 0)
	for _, r := range reg {
		if strings.Contains(r.Que+r.Resultado, "Hola") {
			t.Fatal("el registro lleva texto")
		}
	}
	// apagar el módulo vacía su cola en el acto
	_ = DesactivarModulo(1, "bienvenida")
	if s.mods.Cola.Pendientes() != 0 {
		t.Fatal("al apagar se descarta lo que esperaba")
	}
}

func TestApiLlavesDeModulosSoloEnMemoria(t *testing.T) {
	_, a := abreSolo(t)
	registrarModulosH()
	if err := FijarLlaveModulo(1, "ia", "llave", "sk-PRUEBA-123"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = FijarLlaveModulo(1, "ia", "llave", "") })
	if !TieneLlaveModulo(1, "ia", "llave") || TieneLlaveModulo(1, "traductor", "llave") {
		t.Fatal("la llave debe quedar solo para ese módulo")
	}
	if err := FijarLlaveModulo(1, "ia", "url", "x"); err == nil {
		t.Fatal("solo los campos secretos aceptan llave")
	}
	if err := FijarLlaveModulo(1, "nadie", "llave", "x"); err == nil {
		t.Fatal("módulo inexistente")
	}
	cfg, err := ConfigModulo(1, "ia")
	if err != nil || strings.Contains(cfg, "PRUEBA") || strings.Contains(cfg, `"llave"`) {
		t.Fatalf("la configuración no lleva la llave: %q %v", cfg, err)
	}
	if err := GuardarConfigModulo(1, "ia", `{"llave":"sk-otra"}`); err == nil {
		t.Fatal("no se guarda una llave en la configuración")
	}
	regs, _ := a.ListarAcciones(1, 50, 0)
	for _, r := range regs {
		if strings.Contains(r.Objetivo+r.Detalle+r.Accion, "PRUEBA") {
			t.Fatal("la llave llegó al registro de acciones")
		}
	}
	info, _ := InfoModulo(1, "ia")
	if !strings.Contains(info, `"tipo":"secreto"`) || strings.Contains(info, "PRUEBA") {
		t.Fatal("el esquema trae el campo secreto y nunca su valor")
	}
	_ = FijarLlaveModulo(1, "ia", "llave", "")
	if TieneLlaveModulo(1, "ia", "llave") {
		t.Fatal("vacío la olvida")
	}
}

// Con el bot en marcha: una llamada entrante llega al módulo Llamadas por el evento de whatsmeow y queda en el registro de perdidas.
func TestBotLlamadaEntranteQuedaEnElRegistro(t *testing.T) {
	s := arrancaBot(t)
	ana := types.NewJID("5215511111111", types.DefaultUserServer)
	_, _ = alm.ActualizarChat(1, ana.String(), false, func(c *almacen.Chat) { c.Guardado = "Ana" })
	if err := ActivarModulo(1, "llamadas"); err != nil {
		t.Fatal(err)
	}
	s.evento(&events.CallOffer{BasicCallMeta: types.BasicCallMeta{From: ana, CallCreator: ana, CallID: "LLAM1"}})
	esperaHasta(t, "la llamada no quedó en el registro", func() bool {
		fs, _ := FilasPanelModulo(1, "llamadas", "perdidas")
		return strings.Contains(fs, "Ana")
	})
}
