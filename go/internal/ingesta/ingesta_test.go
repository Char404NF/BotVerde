package ingesta

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"botchat/internal/almacen"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/proto/waWeb"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

type contactosFalso map[types.JID]types.ContactInfo

func (c contactosFalso) GetContact(_ context.Context, j types.JID) (types.ContactInfo, error) {
	return c[j], nil
}

type mapaFalso map[types.JID]types.JID

func (m mapaFalso) GetPNForLID(_ context.Context, l types.JID) (types.JID, error) {
	return m[l], nil
}

type serviciosFalso struct {
	mu                    sync.Mutex
	fotos, asuntos, descs []string
}

func (s *serviciosFalso) PedirFoto(c string) {
	s.mu.Lock()
	s.fotos = append(s.fotos, c)
	s.mu.Unlock()
}
func (s *serviciosFalso) PedirAsunto(c string) {
	s.mu.Lock()
	s.asuntos = append(s.asuntos, c)
	s.mu.Unlock()
}
func (s *serviciosFalso) EncolarDescarga(c, id string) {
	s.mu.Lock()
	s.descs = append(s.descs, c+"|"+id)
	s.mu.Unlock()
}

type entorno struct {
	g    *Ingesta
	alm  *almacen.Almacen
	srv  *serviciosFalso
	cont contactosFalso
	mapa mapaFalso
	red  Red
	logs []string
}

func nuevoEntorno(t *testing.T) *entorno {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 9)
	}
	alm, err := almacen.Abrir(t.TempDir(), k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = alm.Cerrar() })
	e := &entorno{alm: alm, srv: &serviciosFalso{}, cont: contactosFalso{}, mapa: mapaFalso{}, red: RedWifi}
	e.g = Nueva(Config{BotID: 1, Alm: alm, Contactos: e.cont, Mapa: e.mapa, Servicios: e.srv, Red: func() Red { return e.red },
		Libre: func() int64 { return 100 << 30 }, Log: func(s string) { e.logs = append(e.logs, s) }})
	return e
}

func jid(s string) types.JID { j, _ := types.ParseJID(s); return j }

func mensajeEvt(chat, sender types.JID, id string, mio bool, m *waE2E.Message) *events.Message {
	e := &events.Message{Info: types.MessageInfo{
		MessageSource: types.MessageSource{Chat: chat, Sender: sender, IsFromMe: mio, IsGroup: chat.Server == types.GroupServer},
		ID:            id, Timestamp: time.Unix(1_800_000_000, 0), PushName: "Pushy",
	}, RawMessage: m}
	e.UnwrapRaw()
	return e
}

func texto(s string) *waE2E.Message { return &waE2E.Message{Conversation: proto.String(s)} }

func TestChatPrivadoConNombresDeAgenda(t *testing.T) {
	e := nuevoEntorno(t)
	pn := jid("5215512345678@s.whatsapp.net")
	e.cont[pn] = types.ContactInfo{Found: true, FullName: "Mamá", PushName: "Rosa"}
	e.g.Mensaje(context.Background(), mensajeEvt(pn, pn, "M1", false, texto("hola")), false)
	c, _ := e.alm.ObtenerChat(1, pn.String())
	if c == nil || c.Mostrado != "Mamá" || c.TelMostrado != "+52 55 1234 5678" || c.PaisISO != "MX" || c.TelE164 != "+5215512345678" {
		t.Fatalf("%+v", c)
	}
	l, _ := e.alm.ListarChats(1, false, "", 10, 0)
	if len(l) != 1 || l[0].UltimoTexto != "hola" {
		t.Fatalf("%+v", l)
	}
	if len(e.srv.fotos) != 1 {
		t.Fatalf("debía pedir la foto: %v", e.srv.fotos)
	}
	// Sin agenda: se usa el nombre público que viene en el mensaje.
	otro := jid("34612345678@s.whatsapp.net")
	e.g.Mensaje(context.Background(), mensajeEvt(otro, otro, "M2", false, texto("hi")), false)
	if c, _ := e.alm.ObtenerChat(1, otro.String()); c.Mostrado != "Pushy" || c.PaisISO != "ES" {
		t.Fatalf("%+v", c)
	}
}

func TestSoloLIDSinNumero(t *testing.T) {
	e := nuevoEntorno(t)
	lid := jid("1234567890@lid")
	e.g.Mensaje(context.Background(), mensajeEvt(lid, lid, "L1", false, texto("hola")), false)
	c, _ := e.alm.ObtenerChat(1, lid.String())
	if c == nil || c.Mostrado != "Pushy" || c.TelMostrado != "Número no disponible" || c.TelE164 != "" {
		t.Fatalf("%+v", c)
	}
	// Sin nombre público tampoco se muestra el LID.
	otro := jid("999@lid")
	ev := mensajeEvt(otro, otro, "L2", false, texto("x"))
	ev.Info.PushName = ""
	e.g.Mensaje(context.Background(), ev, false)
	c, _ = e.alm.ObtenerChat(1, otro.String())
	if c.Mostrado != "Número no disponible" {
		t.Fatalf("%+v", c)
	}
}

func TestLIDSeResuelveYSeFusiona(t *testing.T) {
	e := nuevoEntorno(t)
	lid := jid("1234567890@lid")
	pn := jid("5215512345678@s.whatsapp.net")
	// Primero llega por LID sin saber el número.
	e.g.Mensaje(context.Background(), mensajeEvt(lid, lid, "A1", false, texto("uno")), false)
	if !e.alm.ExisteChat(1, lid.String()) {
		t.Fatal("el chat LID debía existir")
	}
	// Después se aprende el mapa: el siguiente mensaje usa el teléfono y los chats se unen.
	e.mapa[lid] = pn
	e.g.Mensaje(context.Background(), mensajeEvt(lid, lid, "A2", false, texto("dos")), false)
	if e.alm.ExisteChat(1, lid.String()) {
		t.Fatal("el chat LID debía unirse al del teléfono")
	}
	v, _ := e.alm.ListarMensajes(1, pn.String(), 0, "", 10)
	if len(v) != 2 {
		t.Fatalf("mensajes: %d", len(v))
	}
	c, _ := e.alm.ObtenerChat(1, pn.String())
	if c.TelMostrado != "+52 55 1234 5678" {
		t.Fatalf("%+v", c)
	}
	// La pista que trae el propio mensaje (SenderAlt) también sirve.
	lid2 := jid("55555@lid")
	pn2 := jid("34612345678@s.whatsapp.net")
	ev := mensajeEvt(lid2, lid2, "B1", false, texto("x"))
	ev.Info.SenderAlt = pn2
	e.g.Mensaje(context.Background(), ev, false)
	if !e.alm.ExisteChat(1, pn2.String()) || e.alm.ExisteChat(1, lid2.String()) {
		t.Fatal("debía usar el número de SenderAlt")
	}
}

func TestGrupoConRemitentesYAsunto(t *testing.T) {
	e := nuevoEntorno(t)
	grupo := jid("120363000000000001@g.us")
	ana := jid("34600000001@s.whatsapp.net")
	e.cont[ana] = types.ContactInfo{Found: true, FullName: "Ana Agenda"}
	luis := jid("777@lid")
	e.g.Mensaje(context.Background(), mensajeEvt(grupo, ana, "G1", false, texto("hola grupo")), false)
	ev := mensajeEvt(grupo, luis, "G2", false, texto("soy luis"))
	ev.Info.PushName = "Luis"
	e.g.Mensaje(context.Background(), ev, false)
	e.g.Mensaje(context.Background(), mensajeEvt(grupo, types.EmptyJID, "G3", true, texto("yo desde el teléfono")), false)
	if len(e.srv.asuntos) != 1 {
		t.Fatalf("el asunto se pide una sola vez: %v", e.srv.asuntos)
	}
	e.g.Asunto(grupo.String(), "Familia")
	c, _ := e.alm.ObtenerChat(1, grupo.String())
	if !c.EsGrupo || c.Mostrado != "Familia" || c.TelMostrado != "" {
		t.Fatalf("%+v", c)
	}
	v, _ := e.alm.ListarMensajes(1, grupo.String(), 0, "", 10)
	if len(v) != 3 {
		t.Fatalf("%d", len(v))
	}
	porID := map[string]almacen.MensajeVista{}
	for _, m := range v {
		porID[m.ID] = m
	}
	if porID["G1"].Remitente != "Ana Agenda" || porID["G2"].Remitente != "Luis" || !porID["G3"].EsMio {
		t.Fatalf("%+v", porID)
	}
	if grupos, _ := e.alm.ListarChats(1, true, "", 10, 0); len(grupos) != 1 {
		t.Fatal("el grupo va en la pestaña de grupos")
	}
}

func TestEnviadosYTemporales(t *testing.T) {
	e := nuevoEntorno(t)
	pn := jid("525512345678@s.whatsapp.net")
	// enviado desde el teléfono
	e.g.Mensaje(context.Background(), mensajeEvt(pn, jid("999999@s.whatsapp.net"), "E1", true, texto("te escribo")), false)
	// mensaje temporal (envoltura de mensajes que desaparecen)
	temp := &waE2E.Message{EphemeralMessage: &waE2E.FutureProofMessage{Message: texto("se borra solo")}}
	e.g.Mensaje(context.Background(), mensajeEvt(pn, pn, "T1", false, temp), false)
	v, _ := e.alm.ListarMensajes(1, pn.String(), 0, "", 10)
	if len(v) != 2 {
		t.Fatalf("%d", len(v))
	}
	for _, m := range v {
		if m.ID == "E1" && !m.EsMio {
			t.Error("E1 es mío")
		}
		if m.ID == "T1" && m.Texto != "se borra solo" {
			t.Errorf("T1: %+v", m)
		}
	}
}

func TestEdicionYRevocacion(t *testing.T) {
	e := nuevoEntorno(t)
	pn := jid("525512345678@s.whatsapp.net")
	ctx := context.Background()
	e.g.Mensaje(ctx, mensajeEvt(pn, pn, "O1", false, texto("original")), false)
	ed := &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{Type: waE2E.ProtocolMessage_MESSAGE_EDIT.Enum(),
		Key: &waCommon.MessageKey{ID: proto.String("O1")}, EditedMessage: texto("corregido")}}
	e.g.Mensaje(ctx, mensajeEvt(pn, pn, "EDIT1", false, ed), false)
	m, _ := e.alm.ObtenerMensaje(1, pn.String(), "O1")
	if m.Texto != "corregido" || !m.Editado {
		t.Fatalf("%+v", m)
	}
	rev := &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{Type: waE2E.ProtocolMessage_REVOKE.Enum(), Key: &waCommon.MessageKey{ID: proto.String("O1")}}}
	e.g.Mensaje(ctx, mensajeEvt(pn, pn, "REV1", false, rev), false)
	m, _ = e.alm.ObtenerMensaje(1, pn.String(), "O1")
	if !m.Borrado || m.Texto != "corregido" {
		t.Fatalf("revocado se conserva: %+v", m)
	}
	if v, _ := e.alm.ListarMensajes(1, pn.String(), 0, "", 10); len(v) != 1 {
		t.Fatalf("la edición y la revocación no son mensajes aparte: %d", len(v))
	}
}

func TestChatsAjenosSeIgnoran(t *testing.T) {
	e := nuevoEntorno(t)
	for _, c := range []string{"status@broadcast", "120363@newsletter", "1234@bot"} {
		j := jid(c)
		e.g.Mensaje(context.Background(), mensajeEvt(j, j, "X", false, texto("x")), false)
	}
	if l, _ := e.alm.ListarChats(1, false, "", 10, 0); len(l) != 0 {
		t.Fatal("no debe guardarse nada de estados, canales ni bots")
	}
}

func imagenConMedia(vista bool) *waE2E.Message {
	return &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
		Mimetype: proto.String("image/jpeg"), FileLength: proto.Uint64(2 << 20), JPEGThumbnail: []byte("MINI"), MediaKey: []byte("clave"),
		DirectPath: proto.String("/v/x"), FileSHA256: []byte("s"), FileEncSHA256: []byte("e"), ViewOnce: proto.Bool(vista)}}
}

func TestDescargaAutomaticaSegunPolitica(t *testing.T) {
	e := nuevoEntorno(t)
	pn := jid("525512345678@s.whatsapp.net")
	ctx := context.Background()
	e.g.Mensaje(ctx, mensajeEvt(pn, pn, "FOTO", false, imagenConMedia(false)), false)
	m, _ := e.alm.ObtenerMensaje(1, pn.String(), "FOTO")
	if m.ArchivoEstado != almacen.EstadoPendiente || len(e.srv.descs) != 1 || m.MiniaturaRuta == "" {
		t.Fatalf("%+v %v", m, e.srv.descs)
	}
	if b, err := e.alm.LeerCifrado(m.MiniaturaRuta); err != nil || string(b) != "MINI" {
		t.Fatalf("miniatura: %q %v", b, err)
	}
	// El video con datos móviles no baja solo; con Wi-Fi sí.
	vid := &waE2E.Message{VideoMessage: &waE2E.VideoMessage{FileLength: proto.Uint64(10 << 20), MediaKey: []byte("k"), DirectPath: proto.String("/v")}}
	e.red = RedDatos
	e.g.Mensaje(ctx, mensajeEvt(pn, pn, "VID1", false, vid), false)
	if m, _ := e.alm.ObtenerMensaje(1, pn.String(), "VID1"); m.ArchivoEstado != almacen.EstadoNinguno {
		t.Fatalf("VID1: %s", m.ArchivoEstado)
	}
	e.red = RedWifi
	e.g.Mensaje(ctx, mensajeEvt(pn, pn, "VID2", false, vid), false)
	if m, _ := e.alm.ObtenerMensaje(1, pn.String(), "VID2"); m.ArchivoEstado != almacen.EstadoPendiente {
		t.Fatalf("VID2: %s", m.ArchivoEstado)
	}
	// Archivo grande: queda con botón.
	grande := &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{FileLength: proto.Uint64(80 << 20), MediaKey: []byte("k"), DirectPath: proto.String("/d"), FileName: proto.String("x.zip")}}
	e.g.Mensaje(ctx, mensajeEvt(pn, pn, "DOC", false, grande), false)
	if m, _ := e.alm.ObtenerMensaje(1, pn.String(), "DOC"); m.ArchivoEstado != almacen.EstadoNinguno {
		t.Fatalf("DOC: %s", m.ArchivoEstado)
	}
	// Poco espacio libre: se pausan las automáticas.
	e.g.Libre = func() int64 { return 100 << 20 }
	e.g.Mensaje(ctx, mensajeEvt(pn, pn, "FOTO2", false, imagenConMedia(false)), false)
	if m, _ := e.alm.ObtenerMensaje(1, pn.String(), "FOTO2"); m.ArchivoEstado != almacen.EstadoNinguno {
		t.Fatalf("FOTO2: %s", m.ArchivoEstado)
	}
}

func TestVerUnaVezNoDejaNadaEnDisco(t *testing.T) {
	e := nuevoEntorno(t)
	pn := jid("525512345678@s.whatsapp.net")
	ctx := context.Background()
	casos := map[string]*waE2E.Message{
		"VO1": imagenConMedia(true),
		"VO2": {ViewOnceMessageV2: &waE2E.FutureProofMessage{Message: imagenConMedia(false)}},
		"VO3": {ViewOnceMessage: &waE2E.FutureProofMessage{Message: &waE2E.Message{VideoMessage: &waE2E.VideoMessage{JPEGThumbnail: []byte("MINIVIDEO"), MediaKey: []byte("k"), DirectPath: proto.String("/v")}}}},
		"VO4": {AudioMessage: &waE2E.AudioMessage{ViewOnce: proto.Bool(true), MediaKey: []byte("k"), DirectPath: proto.String("/a")}},
	}
	for id, m := range casos {
		e.g.Mensaje(ctx, mensajeEvt(pn, pn, id, false, m), false)
		m2, _ := e.alm.ObtenerMensaje(1, pn.String(), id)
		if m2 == nil || m2.Tipo != almacen.TipoVerUnaVez || m2.Texto != almacen.TextoVerUnaVez || m2.ArchivoEstado != almacen.EstadoOmitido {
			t.Fatalf("%s: %+v", id, m2)
		}
		if len(m2.ArchivoInfo) != 0 || m2.ArchivoRuta != "" || m2.MiniaturaRuta != "" || m2.ArchivoMime != "" || m2.ArchivoBytes != 0 || m2.Ancho != 0 {
			t.Fatalf("%s conserva datos del archivo: %+v", id, m2)
		}
	}
	if len(e.srv.descs) != 0 {
		t.Fatal("un ver-una-vez no se descarga")
	}
	// Nada de multimedia en disco: ni archivos, ni miniaturas, ni claves.
	hallados := 0
	_ = filepath.WalkDir(e.alm.Raiz(), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Base(filepath.Dir(p)) != filepath.Base(e.alm.Raiz()) {
			hallados++
		}
		return nil
	})
	if hallados != 0 {
		t.Fatalf("quedaron %d archivos en subcarpetas", hallados)
	}
	if _, err := os.Stat(filepath.Join(e.alm.Raiz(), "media")); !os.IsNotExist(err) {
		t.Fatal("no debe existir carpeta de multimedia")
	}
	// Ni en la base: ninguna columna de clave, ruta o miniatura con contenido.
	crudo, _ := os.ReadFile(filepath.Join(e.alm.Raiz(), "messages.db"))
	wal, _ := os.ReadFile(filepath.Join(e.alm.Raiz(), "messages.db-wal"))
	for _, secreto := range []string{"MINIVIDEO", "MINI", "/v/x", "clave"} {
		if contiene(crudo, secreto) || contiene(wal, secreto) {
			t.Fatalf("%q aparece en la base", secreto)
		}
	}
}

func contiene(b []byte, s string) bool { return len(b) > 0 && indexOf(b, []byte(s)) >= 0 }

func indexOf(b, sub []byte) int {
	for i := 0; i+len(sub) <= len(b); i++ {
		ok := true
		for j := range sub {
			if b[i+j] != sub[j] {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}

// ---------- historial con datos sintéticos ----------

func webMsg(id string, fromMe bool, ts uint64, participante string, m *waE2E.Message) *waWeb.WebMessageInfo {
	return &waWeb.WebMessageInfo{
		Key:              &waCommon.MessageKey{ID: proto.String(id), FromMe: proto.Bool(fromMe)},
		MessageTimestamp: proto.Uint64(ts), Message: m, PushName: proto.String("Historico"),
		Participant: proto.String(participante),
	}
}

func historialPrueba() *waHistorySync.HistorySync {
	hm := func(w *waWeb.WebMessageInfo) *waHistorySync.HistorySyncMsg {
		return &waHistorySync.HistorySyncMsg{Message: w}
	}
	return &waHistorySync.HistorySync{
		SyncType: waHistorySync.HistorySync_INITIAL_BOOTSTRAP.Enum(),
		Conversations: []*waHistorySync.Conversation{
			{ID: proto.String("525512345678@s.whatsapp.net"), Messages: []*waHistorySync.HistorySyncMsg{
				hm(webMsg("H1", false, 1000, "", texto("hola vieja"))),
				hm(webMsg("H2", true, 1001, "", texto("mi respuesta"))),
				hm(webMsg("H3", false, 1002, "", imagenConMedia(false))),
				hm(webMsg("H4", false, 1003, "", imagenConMedia(true))),
				hm(webMsg("", false, 1004, "", texto("sin id"))),
			}},
			{ID: proto.String("120363111@g.us"), DisplayName: proto.String("Equipo"), Messages: []*waHistorySync.HistorySyncMsg{
				hm(webMsg("G1", false, 2000, "34600000002@s.whatsapp.net", texto("en el grupo"))),
				hm(webMsg("G2", false, 2001, "", texto("sin remitente"))), // se salta
			}},
			{ID: proto.String("status@broadcast"), Messages: []*waHistorySync.HistorySyncMsg{hm(webMsg("S1", false, 3000, "x@s.whatsapp.net", texto("estado")))}},
		},
		Pushnames: []*waHistorySync.Pushname{{ID: proto.String("525512345678@s.whatsapp.net"), Pushname: proto.String("Nombre Publico")}},
	}
}

func parseReal(t *testing.T) ParseFunc {
	me := jid("5215599999999:7@s.whatsapp.net")
	cli := whatsmeow.NewClient(&store.Device{ID: &me}, waLog.Noop)
	return cli.ParseWebMessage
}

func TestHistorialPorLotes(t *testing.T) {
	e := nuevoEntorno(t)
	n := e.g.Historial(context.Background(), historialPrueba(), parseReal(t))
	if n != 5 { // H1..H4 del chat privado y G1 del grupo; se saltan el que no tiene id y el que no tiene remitente
		t.Fatalf("mensajes guardados: %d", n)
	}
	pn := "525512345678@s.whatsapp.net"
	v, _ := e.alm.ListarMensajes(1, pn, 0, "", 10)
	if len(v) != 4 {
		t.Fatalf("chat privado: %d", len(v))
	}
	porID := map[string]almacen.MensajeVista{}
	for _, m := range v {
		porID[m.ID] = m
	}
	if porID["H2"].EsMio != true || porID["H1"].Texto != "hola vieja" {
		t.Fatalf("%+v", porID)
	}
	// El multimedia del historial no se descarga solo, pero queda el botón.
	if porID["H3"].Estado != almacen.EstadoOmitido || !porID["H3"].PuedeBajarse {
		t.Fatalf("H3: %+v", porID["H3"])
	}
	if len(e.srv.descs) != 0 {
		t.Fatal("el historial no encola descargas")
	}
	// ver una vez en el historial: solo el aviso
	if porID["H4"].Tipo != almacen.TipoVerUnaVez || porID["H4"].PuedeBajarse || porID["H4"].Miniatura != "" {
		t.Fatalf("H4: %+v", porID["H4"])
	}
	// nombres públicos del historial y asunto del grupo
	c, _ := e.alm.ObtenerChat(1, pn)
	if c.Mostrado != "Nombre Publico" {
		t.Fatalf("%+v", c)
	}
	g, _ := e.alm.ObtenerChat(1, "120363111@g.us")
	if g == nil || g.Mostrado != "Equipo" {
		t.Fatalf("%+v", g)
	}
	if gv, _ := e.alm.ListarMensajes(1, "120363111@g.us", 0, "", 10); len(gv) != 1 || gv[0].ID != "G1" {
		t.Fatalf("grupo: %+v", gv)
	}
	if e.alm.ExisteChat(1, "status@broadcast") {
		t.Fatal("los estados no se guardan")
	}
	// Procesar el mismo historial otra vez no duplica.
	e.g.Historial(context.Background(), historialPrueba(), parseReal(t))
	if v2, _ := e.alm.ListarMensajes(1, pn, 0, "", 10); len(v2) != 4 {
		t.Fatalf("duplicó: %d", len(v2))
	}
	// PUSH_NAME aparte
	e.g.Historial(context.Background(), &waHistorySync.HistorySync{SyncType: waHistorySync.HistorySync_PUSH_NAME.Enum(),
		Pushnames: []*waHistorySync.Pushname{{ID: proto.String(pn), Pushname: proto.String("Otro Nombre")}}}, parseReal(t))
	if c, _ := e.alm.ObtenerChat(1, pn); c.Mostrado != "Otro Nombre" {
		t.Fatalf("%+v", c)
	}
	// Tipos que no son de mensajes se ignoran.
	if n := e.g.Historial(context.Background(), &waHistorySync.HistorySync{SyncType: waHistorySync.HistorySync_NON_BLOCKING_DATA.Enum()}, parseReal(t)); n != 0 {
		t.Fatal("otro tipo de historial")
	}
}

func TestNingunLogLlevaDatosPersonales(t *testing.T) {
	e := nuevoEntorno(t)
	pn := jid("525512345678@s.whatsapp.net")
	e.cont[pn] = types.ContactInfo{Found: true, FullName: "Nombre Muy Secreto"}
	e.g.Mensaje(context.Background(), mensajeEvt(pn, pn, "L1", false, texto("mensaje privado 123")), false)
	e.g.Historial(context.Background(), historialPrueba(), parseReal(t))
	for _, l := range e.logs {
		for _, malo := range []string{"Secreto", "privado", "5512345678", "hola vieja"} {
			if contiene([]byte(l), malo) {
				t.Fatalf("el registro lleva datos personales: %q", l)
			}
		}
	}
}
