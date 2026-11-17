package wabot

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/modulos"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func conEco(t *testing.T) {
	t.Helper()
	if err := modulos.RegistrarEco(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { modulos.Quitar(modulos.IDEco) })
}

// listaEco devuelve las tarjetas de los módulos pero solo la del Eco de prueba (los módulos H1–H5 de producción también aparecen en la
// lista real; aquí se prueban aparte).
func listaEco(t *testing.T) []modulos.Info {
	t.Helper()
	var out []modulos.Info
	for _, i := range decodifica[[]modulos.Info](t, func() (string, error) { return ListarModulos(1) }) {
		if i.ID == "eco_prueba" {
			out = append(out, i)
		}
	}
	return out
}

func TestApiModulosDeExtremoAExtremo(t *testing.T) {
	obs, a := abreSolo(t)
	conEco(t)
	const ana = "5215511111111@s.whatsapp.net"
	lista := listaEco(t)
	if len(lista) != 1 || lista[0].ID != "eco_prueba" || lista[0].Activo || lista[0].Codigo != "apagado" || lista[0].Riesgo != "medio" || !lista[0].ConEsquema {
		t.Fatalf("lista: %+v", lista)
	}
	info := decodifica[modulos.Info](t, func() (string, error) { return InfoModulo(1, "eco_prueba") })
	if len(info.Esquema) != 6 || info.DefHora != 5 || info.DefDia != 20 {
		t.Fatalf("info: %+v", info)
	}
	if _, err := InfoModulo(1, "nadie"); err == nil {
		t.Fatal("módulo inexistente")
	}
	// Activar, ver el estado, configurar con validación.
	if err := ActivarModulo(1, "eco_prueba"); err != nil {
		t.Fatal(err)
	}
	if err := ActivarModulo(1, "nadie"); err == nil {
		t.Fatal("activar inexistente")
	}
	lista = listaEco(t)
	if !lista[0].Activo || lista[0].Codigo != "esperando_maestro" || lista[0].ActivadoTS == 0 {
		t.Fatalf("activo con el maestro apagado: %+v", lista[0])
	}
	cfg := decodifica[map[string]any](t, func() (string, error) { return ConfigModulo(1, "eco_prueba") })
	if cfg["frase"] != "eco" || cfg["veces"] != 1.0 {
		t.Fatalf("config: %v", cfg)
	}
	if err := GuardarConfigModulo(1, "eco_prueba", `{"frase":"hola","veces":2}`); err != nil {
		t.Fatal(err)
	}
	if err := GuardarConfigModulo(1, "eco_prueba", `{"veces":99}`); err == nil || !strings.Contains(err.Error(), "entre") {
		t.Fatalf("validación: %v", err)
	}
	cfg = decodifica[map[string]any](t, func() (string, error) { return ConfigModulo(1, "eco_prueba") })
	if cfg["frase"] != "hola" || cfg["veces"] != 2.0 {
		t.Fatalf("config guardada: %v", cfg)
	}
	if err := FijarTopesModulo(1, "eco_prueba", 3, 7); err != nil {
		t.Fatal(err)
	}
	if err := FijarTopesModulo(1, "eco_prueba", -1, 7); err == nil {
		t.Fatal("topes negativos")
	}
	info = decodifica[modulos.Info](t, func() (string, error) { return InfoModulo(1, "eco_prueba") })
	if info.TopeHora != 3 || info.TopeDia != 7 {
		t.Fatalf("topes: %+v", info)
	}
	// Ajustes de iniciativa: el maestro nace apagado.
	aj := decodifica[map[string]string](t, func() (string, error) { return AjustesIniciativa(1) })
	if aj["sin_limites"] != "1" {
		t.Fatalf("de fábrica el bot no tiene límites: %v", aj)
	}
	if aj["maestro"] != "0" || aj["tope_hora"] != "10" || aj["noche_on"] != "1" || aj["noche_desde"] != "23:00" || aj["noche_hasta"] != "07:00" {
		t.Fatalf("ajustes: %v", aj)
	}
	if err := FijarAjusteIniciativa(1, "tope_hora", "4"); err != nil {
		t.Fatal(err)
	}
	if err := FijarAjusteIniciativa(1, "noche_desde", "25:00"); err == nil {
		t.Fatal("hora inválida")
	}
	if err := FijarAjusteIniciativa(1, "inventado", "1"); err == nil {
		t.Fatal("ajuste desconocido")
	}
	if err := FijarIniciativaMaestro(1, true); err != nil {
		t.Fatal(err)
	}
	aj = decodifica[map[string]string](t, func() (string, error) { return AjustesIniciativa(1) })
	if aj["maestro"] != "1" || aj["tope_hora"] != "4" {
		t.Fatalf("ajustes: %v", aj)
	}
	lista = listaEco(t)
	if lista[0].Codigo != "activo" {
		t.Fatalf("con el maestro encendido: %+v", lista[0])
	}
	// No molestar.
	if err := AgregarNoMolestar(1, ana, "a mano"); err != nil {
		t.Fatal(err)
	}
	nm := decodifica[[]almacen.NoMolestarItem](t, func() (string, error) { return ListarNoMolestar(1) })
	if len(nm) != 1 || nm[0].Persona != ana || nm[0].Motivo != "a mano" {
		t.Fatalf("no molestar: %+v", nm)
	}
	if err := QuitarNoMolestar(1, ana); err != nil {
		t.Fatal(err)
	}
	if nm = decodifica[[]almacen.NoMolestarItem](t, func() (string, error) { return ListarNoMolestar(1) }); len(nm) != 0 {
		t.Fatal("quitada")
	}
	// Registro de iniciativas: sin textos y con el chat enmascarado.
	a.RegistrarModulo(1, "eco_prueba", "aviso", "enviado", ana, time.Now())
	a.RegistrarModulo(1, "eco_prueba", "aviso", "descartado:maestro", ana, time.Now())
	reg := decodifica[[]registroIniciativa](t, func() (string, error) { return ListarIniciativas(1, "", 50, 0) })
	if len(reg) != 2 || reg[0].Resultado != "descartado:maestro" || reg[0].Chat != "…1111" || strings.Contains(reg[0].Chat, "5215") {
		t.Fatalf("registro: %+v", reg)
	}
	if rr := decodifica[[]registroIniciativa](t, func() (string, error) { return ListarIniciativas(1, "", 50, reg[0].ID) }); len(rr) != 1 {
		t.Fatalf("página siguiente: %+v", rr)
	}
	// Modo seguro (a mano, como si hubiera ocurrido) y reanudar.
	g, _ := gestorDe(1)
	g.ModoSeguro("errores")
	seg := decodifica[map[string]any](t, func() (string, error) { return EstadoModoSeguro(1) })
	if seg["activo"] != true || seg["motivo"] != "errores" {
		t.Fatalf("seguro: %v", seg)
	}
	lista = listaEco(t)
	if lista[0].Codigo != "pausado_seguro" || !lista[0].Activo {
		t.Fatalf("pausado: %+v", lista[0])
	}
	if err := ReanudarModoSeguro(1); err != nil {
		t.Fatal(err)
	}
	seg = decodifica[map[string]any](t, func() (string, error) { return EstadoModoSeguro(1) })
	if seg["activo"] != false {
		t.Fatalf("seguro: %v", seg)
	}
	// Desactivar es inmediato.
	if err := DesactivarModulo(1, "eco_prueba"); err != nil {
		t.Fatal(err)
	}
	lista = listaEco(t)
	if lista[0].Activo || lista[0].Codigo != "apagado" {
		t.Fatalf("apagado: %+v", lista[0])
	}
	// Las acciones del dueño quedan en su registro (sin textos).
	acc, _ := a.ListarAcciones(1, 100, 0)
	vistas := map[string]bool{}
	for _, x := range acc {
		vistas[x.Accion] = true
	}
	for _, q := range []string{"modulo.activar", "modulo.config", "modulo.topes", "iniciativa.maestro", "iniciativa.no_molestar_agregar", "iniciativa.reanudar", "modulo.desactivar"} {
		if !vistas[q] {
			t.Fatalf("falta la acción %s: %v", q, vistas)
		}
	}
	// Eventos agrupados para la pantalla.
	time.Sleep(500 * time.Millisecond)
	obs.mu.Lock()
	todo := strings.Join(obs.lotes, ",")
	obs.mu.Unlock()
	for _, ev := range []string{`"modulos"`, `"iniciativas"`} {
		if !strings.Contains(todo, ev) {
			t.Fatalf("falta el evento %s en %s", ev, todo)
		}
	}
}

func arrancaBot(t *testing.T) *estado {
	t.Helper()
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
	t.Cleanup(Stop)
	mu.Lock()
	s := st
	mu.Unlock()
	yo := types.NewJID("5215500000000", types.DefaultUserServer)
	s.cli.Store.ID = &yo
	s.cli.Store.LID = types.NewJID("777000111", types.HiddenUserServer)
	return s
}

func textoDeChat(id string, chat types.JID, t string) *events.Message {
	e := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: chat, Sender: chat}, ID: id, Timestamp: time.Now(), PushName: "Pedro"},
		RawMessage: &waE2E.Message{Conversation: proto.String(t)}}
	e.UnwrapRaw()
	return e
}

func esperaHasta(t *testing.T, msg string, f func() bool) {
	t.Helper()
	for i := 0; i < 300; i++ {
		if f() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal(msg)
}

// Con el bot iniciado: BAJA llega por el motor a la lista No molestar y no sigue a los menús.
func TestBotBajaVaALaListaNoMolestar(t *testing.T) {
	s := arrancaBot(t)
	conEco(t)
	pn := types.NewJID("5215511111111", types.DefaultUserServer)
	s.evento(textoDeChat("B1", pn, "BAJA"))
	esperaHasta(t, "BAJA no llegó a la lista", func() bool { return alm.EstaNoMolestar(1, pn.String()) })
	// Un texto cualquiera no.
	otro := types.NewJID("5215522222222", types.DefaultUserServer)
	s.evento(textoDeChat("B2", otro, "hola, quiero precios"))
	time.Sleep(300 * time.Millisecond)
	if alm.EstaNoMolestar(1, otro.String()) {
		t.Fatal("un texto normal no es baja")
	}
	// Con el módulo y el maestro encendidos, ninguna iniciativa le llega.
	if err := ActivarModulo(1, "eco_prueba"); err != nil {
		t.Fatal(err)
	}
	_ = FijarIniciativaMaestro(1, true)
	r, _ := s.mods.Enviar(modulos.Iniciativa{Modulo: "eco_prueba", Chat: pn.String(), Texto: "promo"})
	if r != "descartado:no_molestar" {
		t.Fatalf("a quien pidió la baja: %s", r)
	}
	reg, _ := alm.ListarRegistroModulos(1, "", 20, 0)
	vistos := map[string]bool{}
	for _, x := range reg {
		vistos[x.Que+"/"+x.Resultado] = true
		if strings.Contains(x.Que+x.Resultado+x.Chat, "promo") {
			t.Fatal("el registro lleva texto")
		}
	}
	if !vistos["no_molestar/agregado"] {
		t.Fatalf("registro: %v", vistos)
	}
}

// Una desconexión forzada pausa los módulos de iniciativa y avisa con una notificación local; también llegan los eventos a los módulos.
func TestBotDesconexionForzadaYEventosAModulos(t *testing.T) {
	s := arrancaBot(t)
	conEco(t)
	if err := ActivarModulo(1, "eco_prueba"); err != nil {
		t.Fatal(err)
	}
	_ = FijarIniciativaMaestro(1, true)
	estado := func() string {
		l := listaEco(t)
		return l[0].Estado
	}
	pn := types.NewJID("5215511111111", types.DefaultUserServer)
	s.evento(textoDeChat("E1", pn, "hola"))
	s.evento(&events.GroupInfo{JID: types.NewJID("120363000000000001", types.GroupServer), Join: []types.JID{pn}})
	s.evento(&events.CallOffer{BasicCallMeta: types.BasicCallMeta{From: pn, CallID: "C1"}})
	esperaHasta(t, "los módulos no recibieron los 3 eventos: "+estado(), func() bool { return strings.Contains(estado(), "3 eventos") })
	if s.mods.EnModoSeguro() {
		t.Fatal("nace sin modo seguro")
	}
	for i, ev := range []any{&events.LoggedOut{}, &events.StreamReplaced{}, &events.TemporaryBan{}, &events.ClientOutdated{}} {
		if err := ReanudarModoSeguro(1); err != nil {
			t.Fatal(err)
		}
		s.evento(ev)
		esperaHasta(t, "no entró al modo seguro con "+[]string{"LoggedOut", "StreamReplaced", "TemporaryBan", "ClientOutdated"}[i], s.mods.EnModoSeguro)
		_, _, mot := s.mods.EstadoSeguro()
		if !strings.HasPrefix(mot, "desconexion:") {
			t.Fatalf("motivo: %q", mot)
		}
	}
	// Aviso local creado para la notificación.
	n := 0
	for id := int64(1); id <= 20; id++ {
		if x, _ := alm.ObtenerAviso(1, id); x != nil && x.Tipo == "modo_seguro" && x.Titulo == "Modo seguro activado" {
			n++
		}
	}
	if n != 4 {
		t.Fatalf("avisos de modo seguro: %d", n)
	}
	// El módulo sigue encendido, solo pausado, y deja de recibir eventos.
	l := listaEco(t)
	if !l[0].Activo || l[0].Codigo != "pausado_seguro" {
		t.Fatalf("pausado: %+v", l[0])
	}
	s.evento(textoDeChat("E2", pn, "otra"))
	time.Sleep(200 * time.Millisecond)
	_ = ReanudarModoSeguro(1)
	if strings.Contains(estado(), "4 eventos") {
		t.Fatal("pausado no recibe eventos")
	}
	// Al apagar el bot, los módulos se quedan encendidos en la base y al volver a arrancar se restauran.
	Stop()
	l = listaEco(t)
	if !l[0].Activo {
		t.Fatal("detener el bot no apaga los módulos")
	}
}

// Los envíos fallidos seguidos que frenan al motor activan el modo seguro de los módulos de iniciativa.
func TestBotFrenoDelMotorActivaModoSeguro(t *testing.T) {
	s := arrancaBot(t)
	conEco(t)
	if err := ActivarModulo(1, "eco_prueba"); err != nil {
		t.Fatal(err)
	}
	// Sin conexión, cada respuesta del bot falla: a la 5.ª seguida el motor se frena.
	for i := 0; i < 5; i++ {
		chat := types.NewJID("52155"+string(rune('1'+i))+"0000000", types.DefaultUserServer)
		s.evento(textoDeChat("F"+string(rune('1'+i)), chat, "hola"))
	}
	for i := 0; i < 1500 && !s.mods.EnModoSeguro(); i++ {
		time.Sleep(20 * time.Millisecond)
	}
	if !s.mods.EnModoSeguro() {
		act, _ := alm.UltimaActividad(1, 40)
		t.Fatalf("el freno no activó el modo seguro; actividad: %+v", act)
	}
	if _, _, mot := s.mods.EstadoSeguro(); mot != "errores" {
		t.Fatalf("motivo: %q", mot)
	}
	var seg struct{ Activo bool }
	_ = json.Unmarshal([]byte(s.mods.SeguroJSON()), &seg)
	if !seg.Activo {
		t.Fatal("estado")
	}
}
