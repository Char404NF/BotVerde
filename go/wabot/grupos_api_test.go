package wabot

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/estadoapp"
	"botchat/internal/grupos"

	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

const grpJID = "120363000000000042@g.us"

func TestApiGruposSinElBotEncendido(t *testing.T) {
	a := conChats(t)
	_, _ = a.ActualizarChat(1, grpJID, true, func(c *almacen.Chat) { c.Guardado = "Familia" })
	ts := time.Now().Unix()
	for i := 0; i < 3; i++ {
		_, _ = a.GuardarMensaje(&almacen.Mensaje{ID: "R" + string(rune('a'+i)), ChatJID: grpJID, BotID: 1, RemitenteJID: pnAna, Tipo: almacen.TipoTexto, Texto: "x", TS: ts + int64(i)})
	}
	// Filtros: valores de fábrica, guardar y leer.
	fs := decodifica[[]grupos.Filtro](t, func() (string, error) { return GrupoFiltros(1, grpJID) })
	if len(fs) != 4 || fs[0].Activo || fs[0].Accion != "avisarme" {
		t.Fatalf("%+v", fs)
	}
	fs[0].Activo, fs[0].Accion = true, "borrar"
	fs[0].Config.Permitidos = []string{"https://youtube.com"}
	b, _ := json.Marshal(fs[0])
	if err := GrupoGuardarFiltro(1, grpJID, string(b)); err != nil {
		t.Fatal(err)
	}
	fs = decodifica[[]grupos.Filtro](t, func() (string, error) { return GrupoFiltros(1, grpJID) })
	if !fs[0].Activo || fs[0].Accion != "borrar" || fs[0].Config.Permitidos[0] != "youtube.com" || fs[0].Chat != grpJID {
		t.Fatalf("%+v", fs[0])
	}
	if err := GrupoGuardarFiltro(1, grpJID, "no es json"); err == nil {
		t.Fatal("json malo")
	}
	fs[1].Accion = "volar"
	b, _ = json.Marshal(fs[1])
	if err := GrupoGuardarFiltro(1, grpJID, string(b)); err == nil || !strings.Contains(err.Error(), "no existe") {
		t.Fatalf("%v", err)
	}
	if err := GrupoRestaurarFiltro(1, grpJID, "enlaces"); err != nil {
		t.Fatal(err)
	}
	// Reglas.
	if err := GrupoGuardarReglas(1, grpJID, "Sé amable"); err != nil {
		t.Fatal(err)
	}
	if r, _ := GrupoReglas(1, grpJID); r != "Sé amable" {
		t.Fatalf("%q", r)
	}
	// Ranking funciona sin conexión.
	rk := decodifica[grupos.Ranking](t, func() (string, error) { return GrupoRanking(1, grpJID, 7, 5) })
	if rk.Total != 3 || rk.SoloMiembros || len(rk.Filas) != 1 || rk.Filas[0].Mensajes != 3 {
		t.Fatalf("%+v", rk)
	}
	// Lo que necesita WhatsApp lo dice claro.
	st := decodifica[grupos.EstadoGrupo](t, func() (string, error) { return GrupoEstado(1, grpJID, true) })
	if st.Conectado || st.SoyAdmin || !strings.Contains(st.Motivo, "no está conectado") {
		t.Fatalf("%+v", st)
	}
	for _, err := range []error{
		GrupoCerrar(1, grpJID, true), GrupoExpulsar(1, grpJID, pnAna), GrupoRenombrar(1, grpJID, "X"), GrupoBorrarMensaje(1, grpJID, "Ra"),
		func() error { _, e := GrupoEnlace(1, grpJID, false); return e }(),
		func() error { _, e := GrupoInfo(1, grpJID); return e }(),
		func() error { _, e := GrupoInactivos(1, grpJID, 30); return e }(),
	} {
		if err == nil || !strings.Contains(err.Error(), "no hay conexión") {
			t.Fatalf("%v", err)
		}
	}
	if _, err := GrupoResolverSolicitudes(1, grpJID, "no json", true); err == nil {
		t.Fatal("lista mala")
	}
	// Ajustes nuevos con validación.
	if err := FijarAjusteDePermisos(1, "todos_tope", "0"); err == nil {
		t.Fatal("el tope no puede ser 0")
	}
	if err := FijarAjusteDePermisos(1, "todos_tope", "80"); err != nil {
		t.Fatal(err)
	}
	if err := FijarAjusteDePermisos(1, "grupo_dias", "90"); err != nil {
		t.Fatal(err)
	}
	// El estado de la app de WhatsApp sin el bot encendido: nada disponible y sin errores confusos.
	if AppStateDisponible(1) {
		t.Fatal("sin bot no hay llaves")
	}
	e := decodifica[estadoapp.Estado](t, func() (string, error) { return AppStateEstado(1) })
	if e.Disponible || !strings.Contains(e.Motivo, "no está encendido") {
		t.Fatalf("%+v", e)
	}
	if err := AppStateFijar(1, pnAna, true); err == nil || !strings.Contains(err.Error(), "no está encendido") {
		t.Fatalf("%v", err)
	}
}

// cliente de app state de mentira
type appFalso struct {
	mu      sync.Mutex
	llaves  bool
	parches []appstate.PatchInfo
}

func (f *appFalso) Conectado() bool { return true }
func (f *appFalso) SendAppState(_ context.Context, p appstate.PatchInfo) error {
	f.mu.Lock()
	f.parches = append(f.parches, p)
	f.mu.Unlock()
	return nil
}
func (f *appFalso) TieneLlaves(context.Context) (bool, error)               { return f.llaves, nil }
func (f *appFalso) Sincronizar(context.Context, appstate.WAPatchName) error { return nil }

func TestElBotAplicaLosFiltrosYAvisaDeCambiosDeGrupo(t *testing.T) {
	reinicia(t)
	f := &falso{}
	raiz := t.TempDir()
	if err := AbrirAlmacen(raiz, clavePrueba(), &obsFalso{}); err != nil {
		t.Fatal(err)
	}
	if err := Start(raiz, 1, clavePrueba(), f); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	s := st
	mu.Unlock()
	defer Stop()
	// Los comandos de grupos quedaron registrados (ninguno chocó con otro).
	nombres := strings.Join(s.cmd.Nombres(), " ")
	for _, c := range []string{"grupo", "admins", "todos", "ranking", "inactivos", "cerrar", "abrir", "bloquearinfo", "desbloquearinfo", "expulsar", "ascender", "bajar",
		"solicitudes", "aprobar", "rechazar", "nombre", "descripcion", "enlace", "enlacenuevo", "borrar", "filtros", "reglas"} {
		if !strings.Contains(" "+nombres+" ", " "+c+" ") {
			t.Fatalf("falta el comando %s en %s", c, nombres)
		}
	}
	// Cliente de grupos de mentira: yo soy administradora.
	grp, _ := types.ParseJID(grpJID)
	ana, _ := types.ParseJID("34600000001@s.whatsapp.net")
	yo := s.cli.Store.ID
	_ = yo
	fake := &grupos.ClienteFalso{Conexion: true, PN: mustJID("34699999999@s.whatsapp.net"), Errores: map[string]error{}}
	fake.Info = &types.GroupInfo{JID: grp, GroupName: types.GroupName{Name: "Familia"}, Participants: []types.GroupParticipant{
		{JID: fake.PN, PhoneNumber: fake.PN, IsAdmin: true}, {JID: ana, PhoneNumber: ana}}}
	s.gru.Cli = fake
	var mb sync.Mutex
	var borrados []string
	s.gru.Borrar = func(_ context.Context, chat, id string) error {
		mb.Lock()
		borrados = append(borrados, chat+"|"+id)
		mb.Unlock()
		return nil
	}
	// Enlaces prohibidos -> borrar.
	fs := decodifica[[]grupos.Filtro](t, func() (string, error) { return GrupoFiltros(1, grpJID) })
	fs[0].Activo, fs[0].Accion = true, "borrar"
	b, _ := json.Marshal(fs[0])
	if err := GrupoGuardarFiltro(1, grpJID, string(b)); err != nil {
		t.Fatal(err)
	}
	mandar := func(id, texto string) {
		e := &events.Message{Info: types.MessageInfo{MessageSource: types.MessageSource{Chat: grp, Sender: ana, IsGroup: true}, ID: id, Timestamp: time.Now(), PushName: "Ana"},
			RawMessage: &waE2E.Message{Conversation: proto.String(texto)}}
		e.UnwrapRaw()
		s.evento(e) // el manejador solo guarda y encola
	}
	mandar("OK1", "hola a todos")
	mandar("MAL1", "compren en https://estafa.example.com ya")
	esperaHasta(t, "borrado del mensaje con enlace", func() bool { mb.Lock(); defer mb.Unlock(); return len(borrados) == 1 })
	mb.Lock()
	if borrados[0] != grpJID+"|MAL1" {
		t.Fatalf("%v", borrados)
	}
	mb.Unlock()
	// Los dos mensajes se guardaron igual (historial intacto).
	if j, _ := ListarMensajes(1, grpJID, 0, "", 10); strings.Count(j, `"id"`) != 2 {
		t.Fatalf("%s", j)
	}
	// Si dejo de ser administradora (el evento de grupo renueva lo recordado) solo alerta.
	copia := *fake.Info
	copia.Participants = []types.GroupParticipant{{JID: fake.PN, PhoneNumber: fake.PN}, {JID: ana, PhoneNumber: ana}}
	fake.Info = &copia
	before := fake.InfoPedida
	s.evento(&events.GroupInfo{JID: grp, Demote: []types.JID{fake.PN}})
	if !strings.Contains(jsonDe(t, func() (string, error) { return GrupoEstado(1, grpJID, false) }), `"soyAdmin":false`) || fake.InfoPedida == before {
		t.Fatalf("el evento debía renovar la información (pedidas %d -> %d)", before, fake.InfoPedida)
	}
	s.gru.Ahora = func() time.Time { return time.Now().Add(time.Hour) } // fuera del anti-repetición de alertas y de la caché de filtros
	mandar("MAL2", "otro https://estafa2.example.com")
	esperaHasta(t, "alerta local", func() bool {
		av, _ := s.gru.Alm.AvisosRecientes(1, 10)
		for _, x := range av {
			if x.Tipo == "filtros" && strings.Contains(x.Detalle, "no soy administrador") {
				return true
			}
		}
		return false
	})
	mb.Lock()
	if len(borrados) != 1 {
		t.Fatalf("sin ser admin no debe borrar: %v", borrados)
	}
	mb.Unlock()
	// App state con y sin llaves por la API.
	app := &appFalso{}
	s.eapp.Cli = app
	if AppStateDisponible(1) {
		t.Fatal("sin llaves")
	}
	if e := decodifica[estadoapp.Estado](t, func() (string, error) { return AppStateEstado(1) }); e.Disponible || e.Motivo != estadoapp.TextoSinLlaves {
		t.Fatalf("%+v", e)
	}
	if err := AppStateSilenciar(1, pnAna, 3600); err == nil || !strings.Contains(err.Error(), "teléfono principal sincronice") || len(app.parches) != 0 {
		t.Fatalf("%v", err)
	}
	app.llaves = true
	if !AppStateDisponible(1) {
		t.Fatal("con llaves")
	}
	if err := AppStateSilenciar(1, pnAna, 8*3600); err != nil {
		t.Fatal(err)
	}
	if err := AppStateSilenciar(1, pnAna, 0); err != nil {
		t.Fatal(err)
	}
	if err := AppStateSilenciar(1, pnAna, -1); err == nil {
		t.Fatal("negativo")
	}
	if err := AppStateDesilenciar(1, pnAna); err != nil {
		t.Fatal(err)
	}
	if err := AppStateFijar(1, pnAna, true); err != nil {
		t.Fatal(err)
	}
	if err := AppStateArchivar(1, pnAna, true); err != nil {
		t.Fatal(err)
	}
	if err := AppStateSincronizar(1); err != nil {
		t.Fatal(err)
	}
	if len(app.parches) != 5 {
		t.Fatalf("%d", len(app.parches))
	}
}

func mustJID(s string) types.JID { j, _ := types.ParseJID(s); return j }

func jsonDe(t *testing.T, f func() (string, error)) string {
	t.Helper()
	j, err := f()
	if err != nil {
		t.Fatal(err)
	}
	return j
}
