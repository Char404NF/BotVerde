package wabot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/personas"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

const (
	pnAna  = "5215511111111@s.whatsapp.net"
	pnLuis = "5215522222222@s.whatsapp.net"
)

func conChats(t *testing.T) *almacen.Almacen {
	t.Helper()
	_, a := abreSolo(t)
	ts := time.Now().Unix()
	for jid, nom := range map[string]string{pnAna: "Ana Pérez", pnLuis: "Luis"} {
		_, _ = a.ActualizarChat(1, jid, false, func(c *almacen.Chat) { c.TelE164, c.Publico = "+"+jid[:13], nom })
	}
	_, _ = a.GuardarMensaje(&almacen.Mensaje{ID: "A1", ChatJID: pnAna, BotID: 1, RemitenteJID: pnAna, Tipo: almacen.TipoTexto, Texto: "hola, soy Ana", TS: ts})
	_, _ = a.GuardarMensaje(&almacen.Mensaje{ID: "L1", ChatJID: pnLuis, BotID: 1, RemitenteJID: pnLuis, Tipo: almacen.TipoTexto, Texto: "hola, soy Luis", TS: ts})
	return a
}

func TestApiPersonasDeExtremoAExtremo(t *testing.T) {
	a := conChats(t)
	// Silenciar y bloquear (con la ficha viéndolo).
	if err := SilenciarPersona(1, pnAna, 30, "ruido"); err != nil {
		t.Fatal(err)
	}
	f := decodifica[personas.FichaCompleta](t, func() (string, error) { return FichaPersona(1, pnAna) })
	if f.Nombre != "Ana Pérez" || f.Silencio == nil || f.Silencio.Motivo != "ruido" || f.Recibidos != 1 {
		t.Fatalf("%+v", f)
	}
	if ls := decodifica[[]almacen.Silencio](t, func() (string, error) { return ListarSilencios(1) }); len(ls) != 1 {
		t.Fatalf("%+v", ls)
	}
	if ok, _ := QuitarSilencioPersona(1, pnAna); !ok {
		t.Fatal("quitar silencio")
	}
	if err := SilenciarPersona(1, pnAna, -3, ""); err == nil {
		t.Fatal("minutos negativos")
	}
	_ = BloquearPersona(1, pnLuis)
	if !a.EstaBloqueada(1, pnLuis) {
		t.Fatal("bloqueada")
	}
	if ok, _ := DesbloquearPersona(1, pnLuis); !ok {
		t.Fatal("desbloquear")
	}
	// Advertencias con tope.
	if err := FijarAjusteDePermisos(1, "adv_tope", "2"); err != nil {
		t.Fatal(err)
	}
	r1 := decodifica[personas.ResultadoAdvertir](t, func() (string, error) { return AdvertirPersona(1, pnAna, "", "primera") })
	r2 := decodifica[personas.ResultadoAdvertir](t, func() (string, error) { return AdvertirPersona(1, pnAna, "", "segunda") })
	if r1.Total != 1 || r1.LlegoAlTope || r2.Total != 2 || !r2.LlegoAlTope || r2.Hecho != "avisado" {
		t.Fatalf("%+v %+v", r1, r2)
	}
	adv := decodifica[[]almacen.Advertencia](t, func() (string, error) { return ListarAdvertencias(1, pnAna) })
	if len(adv) != 2 || adv[0].Motivo != "segunda" {
		t.Fatalf("%+v", adv)
	}
	pa := decodifica[[]struct {
		Persona string `json:"persona"`
		Total   int    `json:"total"`
		Nombre  string `json:"nombre"`
		Tope    int    `json:"tope"`
	}](t, func() (string, error) { return ListarPersonasAdvertidas(1) })
	if len(pa) != 1 || pa[0].Total != 2 || pa[0].Nombre != "Ana Pérez" || pa[0].Tope != 2 {
		t.Fatalf("%+v", pa)
	}
	if n, _ := QuitarAdvertenciaPersona(1, pnAna, 0); n != 1 {
		t.Fatalf("quedan %d", n)
	}
	if n, _ := BorrarAdvertenciasPersona(1, pnAna); n != 1 {
		t.Fatalf("borradas %d", n)
	}
	// Notas.
	_ = AgregarNotaPersona(1, pnAna, "prefiere WhatsApp")
	ns := decodifica[[]almacen.Nota](t, func() (string, error) { return ListarNotasPersona(1, pnAna) })
	if len(ns) != 1 || ns[0].Texto != "prefiere WhatsApp" {
		t.Fatalf("%+v", ns)
	}
	// Estadísticas y exportar (archivo BCF1 cifrado dentro de la carpeta de datos).
	est := decodifica[almacen.EstadisticasPersona](t, func() (string, error) { return EstadisticasPersona(1, pnAna, 30) })
	if est.Total != 1 {
		t.Fatalf("%+v", est)
	}
	rel, err := ExportarPersona(1, pnAna)
	if err != nil || !strings.HasPrefix(rel, "exportes/") {
		t.Fatalf("%q %v", rel, err)
	}
	abs, _ := a.Abs(rel)
	crudo, _ := os.ReadFile(abs)
	if len(crudo) < 4 || string(crudo[:4]) != "BCF1" || strings.Contains(string(crudo), "prefiere") {
		t.Fatal("el export va cifrado")
	}
	if filepath.Ext(abs) != ".bcf" {
		t.Fatal(abs)
	}
	// Olvidar.
	ro := decodifica[almacen.ResultadoOlvidar](t, func() (string, error) { return OlvidarPersona(1, pnAna) })
	if ro.Mensajes != 1 || ro.Notas != 1 || a.TieneMensajes(1, pnAna) {
		t.Fatalf("%+v", ro)
	}
	// Todo lo del dueño quedó en el registro de acciones (sin textos).
	acs, _ := a.ListarAcciones(1, 100, 0)
	for _, x := range acs {
		if strings.Contains(x.Detalle, "prefiere") || strings.Contains(x.Detalle, "primera") || strings.Contains(x.Detalle, "ruido") {
			t.Fatalf("texto en el registro: %+v", x)
		}
	}
}

func TestApiChatEtiquetasPendienteYFiltros(t *testing.T) {
	_ = conChats(t)
	if err := MarcarEstadoChat(1, pnAna, "pendiente"); err != nil {
		t.Fatal(err)
	}
	if err := MarcarEstadoChat(1, pnAna, "raro"); err == nil {
		t.Fatal("estado inválido")
	}
	if n, _ := ContarPendientes(1); n != 1 {
		t.Fatal(n)
	}
	vip, err := CrearEtiqueta(1, "VIP", "#FF8800")
	if err != nil {
		t.Fatal(err)
	}
	_ = PonerEtiquetaChat(1, pnLuis, vip, true)
	pend := decodifica[[]almacen.ChatLista](t, func() (string, error) { return ListarChatsFiltrados(1, false, "", "pendiente", 0, 50, 0) })
	if len(pend) != 1 || pend[0].JID != pnAna || pend[0].EstadoTrabajo != "pendiente" {
		t.Fatalf("%+v", pend)
	}
	con := decodifica[[]almacen.ChatLista](t, func() (string, error) { return ListarChatsFiltrados(1, false, "", "", vip, 50, 0) })
	if len(con) != 1 || con[0].JID != pnLuis || len(con[0].Etiquetas) != 1 {
		t.Fatalf("%+v", con)
	}
	ets := decodifica[[]almacen.Etiqueta](t, func() (string, error) { return ListarEtiquetas(1) })
	if len(ets) != 1 || ets[0].Chats != 1 {
		t.Fatalf("%+v", ets)
	}
	if err := EditarEtiqueta(1, vip, "Muy VIP", "#00AA00"); err != nil {
		t.Fatal(err)
	}
	_ = BorrarEtiqueta(1, vip)
	if ets = decodifica[[]almacen.Etiqueta](t, func() (string, error) { return ListarEtiquetas(1) }); len(ets) != 0 {
		t.Fatal("borrar")
	}
}

func TestApiAvisosAlertasRecordatoriosYAjustes(t *testing.T) {
	_ = conChats(t)
	id, err := CrearAlertaPalabra(1, "urgente", "[]")
	if err != nil {
		t.Fatal(err)
	}
	_ = EditarAlertaPalabra(1, id, `["`+pnAna+`"]`, true)
	al := decodifica[[]almacen.AlertaPalabra](t, func() (string, error) { return ListarAlertasPalabra(1) })
	if len(al) != 1 || len(al[0].Chats) != 1 {
		t.Fatalf("%+v", al)
	}
	if _, err := CrearAlertaPalabra(1, "x", "no es json"); err == nil {
		t.Fatal("chats mal escritos")
	}
	// Recordatorio: no se puede en el pasado; el siguiente aviso es el más próximo; al vencer se dispara una vez.
	if _, err := CrearRecordatorio(1, "ayer", time.Now().Add(-time.Hour).Unix(), ""); err == nil {
		t.Fatal("en el pasado")
	}
	cuando := time.Now().Add(1500 * time.Millisecond).Unix()
	rid, err := CrearRecordatorio(1, "pagar la luz", cuando+1, "")
	if err != nil {
		t.Fatal(err)
	}
	if sig, _ := ProximoAviso(1); sig != cuando+1 {
		t.Fatalf("próximo %d (esperaba %d)", sig, cuando+1)
	}
	rs := decodifica[[]almacen.Recordatorio](t, func() (string, error) { return ListarRecordatorios(1, false) })
	if len(rs) != 1 || rs[0].Texto != "pagar la luz" {
		t.Fatalf("%+v", rs)
	}
	disp := decodifica[struct {
		Avisos    []int64 `json:"avisos"`
		Siguiente int64   `json:"siguiente"`
	}](t, func() (string, error) { return DispararAvisos(1) })
	if len(disp.Avisos) != 0 || disp.Siguiente != cuando+1 {
		t.Fatalf("aún no toca: %+v", disp)
	}
	time.Sleep(2600 * time.Millisecond)
	disp = decodifica[struct {
		Avisos    []int64 `json:"avisos"`
		Siguiente int64   `json:"siguiente"`
	}](t, func() (string, error) { return DispararAvisos(1) })
	if len(disp.Avisos) != 1 || disp.Siguiente != 0 {
		t.Fatalf("debía sonar: %+v", disp)
	}
	a, _ := almacenAbierto()
	av, _ := a.ObtenerAviso(1, disp.Avisos[0])
	if av.Tipo != "recordatorio" || av.Detalle != "pagar la luz" {
		t.Fatalf("%+v", av)
	}
	_ = BorrarRecordatorio(1, rid)
	// Resumen: se puede previsualizar.
	if v, _ := VistaPreviaResumen(1); !strings.Contains(v, "Chats pendientes") {
		t.Fatal(v)
	}
	// Ajustes validados.
	for clave, mal := range map[string]string{"adv_tope": "99", "adv_accion": "explotar", "resumen_hora": "25:00", "resumen_activo": "2", "reportar_texto": strings.Repeat("x", 201), "resumen_ultimo": "2026-01-01"} {
		if err := FijarAjusteDePermisos(1, clave, mal); err == nil {
			t.Fatalf("%s=%q debía fallar", clave, mal)
		}
	}
	for clave, bien := range map[string]string{"adv_tope": "5", "adv_accion": "expulsar", "adv_silencio_min": "60", "resumen_hora": "07:30", "resumen_activo": "1", "alerta_fragmento": "1", "reportar_gracias": "1", "avisos_exactos": "0"} {
		if err := FijarAjusteDePermisos(1, clave, bien); err != nil {
			t.Fatalf("%s=%q: %v", clave, bien, err)
		}
	}
	m := decodifica[map[string]string](t, func() (string, error) { return AjustesDePermisos(1) })
	if m["adv_accion"] != "expulsar" || m["resumen_hora"] != "07:30" {
		t.Fatalf("%v", m)
	}
}

func TestApiComandosPropios(t *testing.T) {
	_ = conChats(t)
	id, err := GuardarComandoPropio(1, 0, `{"nombre":"Horario","texto":"Abrimos de 9 a 6, {nombre}","resumen":"Nuestro horario","privados":true,"grupos":true}`)
	if err != nil || id == 0 {
		t.Fatalf("%d %v", id, err)
	}
	// Repetido, o igual que un comando del bot, o mal escrito: errores claros.
	if _, err := GuardarComandoPropio(1, 0, `{"nombre":"horario","texto":"x","privados":true}`); err == nil || !strings.Contains(err.Error(), "ya tienes") {
		t.Fatalf("%v", err)
	}
	if _, err := GuardarComandoPropio(1, 0, `{"nombre":"ayuda","texto":"x","privados":true}`); err == nil || !strings.Contains(err.Error(), "ya es un comando del bot") {
		t.Fatalf("%v", err)
	}
	if _, err := GuardarComandoPropio(1, 0, `{"nombre":"dos palabras","texto":"x","privados":true}`); err == nil {
		t.Fatal("nombre inválido")
	}
	l := decodifica[[]comandoPropioVista](t, func() (string, error) { return ListarComandosPropios(1) })
	if len(l) != 1 || l[0].Datos.Nombre != "horario" || !l[0].Activo {
		t.Fatalf("%+v", l)
	}
	// Editar conserva el id y no choca consigo mismo.
	if id2, err := GuardarComandoPropio(1, id, `{"nombre":"horario","texto":"Nuevo texto","privados":true}`); err != nil || id2 != id {
		t.Fatalf("%d %v", id2, err)
	}
	if _, err := GuardarComandoPropio(1, 9999, `{"nombre":"otro","texto":"x","privados":true}`); err == nil {
		t.Fatal("editar uno que no existe")
	}
	// No aparece entre las automatizaciones normales.
	auto := decodifica[[]resumenAuto](t, func() (string, error) { return ListarAutomatizaciones(1) })
	for _, x := range auto {
		if strings.Contains(x.Nombre, "horario") {
			t.Fatalf("los comandos propios tienen su pantalla: %+v", x)
		}
	}
	// Vista previa con datos de ejemplo.
	v, err := VistaPreviaComandoPropio(1, `{"nombre":"precios","texto":"Hola {nombre}: elige","privados":true,"opciones":[{"texto":"Básico","respuesta":"$100"}]}`)
	if err != nil || !strings.Contains(v, "Escriben: !precios") || !strings.Contains(v, "Hola Ana: elige") || !strings.Contains(v, "1. Básico") || !strings.Contains(v, "$100") {
		t.Fatalf("%q %v", v, err)
	}
	// !ayuda del dueño (sin bot) no lista los propios aún porque no hay motor; con el prefijo cambiado, la vista previa lo usa.
	_ = FijarAjusteDePermisos(1, "prefijo", "#")
	if v, _ = VistaPreviaComandoPropio(1, `{"nombre":"x","texto":"t","privados":true}`); !strings.Contains(v, "Escriben: #x") {
		t.Fatal(v)
	}
	if err := ActivarAutomatizacion(1, id, false); err != nil {
		t.Fatal(err)
	}
	if err := BorrarAutomatizacion(1, id); err != nil {
		t.Fatal(err)
	}
}

// Con el bot iniciado: palabra clave, silencio, bloqueo y comandos propios con los eventos reales de WhatsApp.
func TestBotAvisosSilencioYComandosPropiosDeExtremoAExtremo(t *testing.T) {
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
	ana, _ := types.ParseJID(pnAna)
	luis, _ := types.ParseJID(pnLuis)
	n := 0
	texto := func(de types.JID, t string, cita *waE2E.ContextInfo) *events.Message {
		n++
		var m *waE2E.Message
		if cita != nil {
			m = &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String(t), ContextInfo: cita}}
		} else {
			m = &waE2E.Message{Conversation: proto.String(t)}
		}
		e := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: de, Sender: de}, ID: "E" + string(rune('A'+n)), Timestamp: time.Now(), PushName: "Pedro"}, RawMessage: m}
		e.UnwrapRaw()
		return e
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
	avisosDe := func(tipo string) int {
		av, _ := alm.AvisosRecientes(1, 50)
		c := 0
		for _, x := range av {
			if x.Tipo == tipo {
				c++
			}
		}
		return c
	}
	actividad := func(motivo string) int {
		l, _ := alm.UltimaActividad(1, 500)
		c := 0
		for _, a := range l {
			if a.Motivo == motivo || a.Detalle == motivo {
				c++
			}
		}
		return c
	}
	// 1. Palabra clave: llega un aviso local sin copiar el mensaje.
	_, _ = CrearAlertaPalabra(1, "cotización", "[]")
	s.evento(texto(ana, "Hola, necesito una COTIZACION para mi boda en la playa", nil))
	espera(func() bool { return avisosDe("palabra") == 1 }, "debía llegar la alerta por palabra clave")
	av, _ := alm.AvisosRecientes(1, 5)
	if strings.Contains(av[0].Detalle, "boda") || strings.Contains(av[0].Detalle, "playa") {
		t.Fatalf("la alerta no copia el mensaje: %q", av[0].Detalle)
	}
	// 2. Silenciar: el bot ignora a Ana (pero la alerta por palabra sigue: es para mí, no una respuesta).
	_ = SilenciarPersona(1, pnAna, 0, "")
	s.evento(texto(ana, "hola de nuevo", nil))
	espera(func() bool { return actividad("persona silenciada") == 1 }, "el motor debía ignorar a la silenciada")
	// 3. Bloquear a Luis.
	_ = BloquearPersona(1, pnLuis)
	s.evento(texto(luis, "hola", nil))
	espera(func() bool { return actividad("persona bloqueada") == 1 }, "el motor debía ignorar a la bloqueada")
	// 4. Comando propio !horario para quien tenga rango; desconocidos sin rango no. Prefijo "!".
	_, _ = QuitarSilencioPersona(1, pnAna)
	_, _ = DesbloquearPersona(1, pnLuis)
	if _, err := GuardarComandoPropio(1, 0, `{"nombre":"horario","texto":"Abrimos a las 9","resumen":"Nuestro horario","privados":true}`); err != nil {
		t.Fatal(err)
	}
	s.evento(texto(ana, "!horario", nil))
	espera(func() bool { return actividad("Comando horario") >= 1 }, "el comando propio debía coincidir")
	// !ayuda lo lista.
	r := decodifica[resultadoComando](t, func() (string, error) { return EjecutarComando(1, "!ayuda") })
	if !strings.Contains(r.Texto, "!horario — Nuestro horario") || !strings.Contains(r.Texto, "!silenciar") || !strings.Contains(r.Texto, "!recordar") {
		t.Fatalf("%q", r.Texto)
	}
	// 5. !reportar citando un mensaje: solo un aviso local para mí.
	cita := &waE2E.ContextInfo{StanzaID: proto.String("MALO"), Participant: proto.String(pnLuis)}
	s.evento(texto(ana, "!reportar me parece grosero", cita))
	espera(func() bool { return avisosDe("reporte") == 1 }, "el reporte debía llegar como aviso")
	acs, _ := alm.ListarAcciones(1, 200, 0)
	for _, a := range acs {
		if strings.Contains(a.Detalle, "grosero") {
			t.Fatalf("texto en el registro: %+v", a)
		}
	}
}
