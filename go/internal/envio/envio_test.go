package envio

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"botchat/internal/almacen"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

type envioHecho struct {
	a   types.JID
	msg *waE2E.Message
	id  string
}

type falso struct {
	mu        sync.Mutex
	conectado bool
	n         int
	enviados  []envioHecho
	falla     error
	leidos    [][]types.MessageID
	leidoDe   []types.JID
	presencia []string
	grupo     *types.GroupInfo
	yo, lid   types.JID
	subidas   [][]byte
	lecturaOn bool
	played    int
}

func nuevoFalso() *falso {
	return &falso{conectado: true, lecturaOn: true, yo: types.NewJID("5215500000000", types.DefaultUserServer), lid: types.NewJID("999", types.HiddenUserServer)}
}

func (f *falso) GenerateMessageID() types.MessageID {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	return types.MessageID("ID" + string(rune('A'+f.n)))
}
func (f *falso) SendMessage(_ context.Context, to types.JID, m *waE2E.Message, extra ...whatsmeow.SendRequestExtra) (whatsmeow.SendResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.falla != nil {
		return whatsmeow.SendResponse{}, f.falla
	}
	id := ""
	if len(extra) > 0 {
		id = string(extra[0].ID)
	}
	f.enviados = append(f.enviados, envioHecho{a: to, msg: m, id: id})
	return whatsmeow.SendResponse{ID: types.MessageID(id), Timestamp: time.Now()}, nil
}
func (f *falso) BuildReaction(chat, sender types.JID, id types.MessageID, r string) *waE2E.Message {
	k := &waCommon.MessageKey{ID: proto.String(string(id)), FromMe: proto.Bool(sender.IsEmpty())}
	if !sender.IsEmpty() && chat.Server == types.GroupServer {
		k.Participant = proto.String(sender.String())
	}
	return &waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{Key: k, Text: proto.String(r)}}
}
func (f *falso) BuildEdit(chat types.JID, id types.MessageID, c *waE2E.Message) *waE2E.Message {
	return &waE2E.Message{EditedMessage: &waE2E.FutureProofMessage{Message: &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{
		Type: waE2E.ProtocolMessage_MESSAGE_EDIT.Enum(), Key: &waCommon.MessageKey{ID: proto.String(string(id))}, EditedMessage: c}}}}
}
func (f *falso) BuildRevoke(chat, sender types.JID, id types.MessageID) *waE2E.Message {
	k := &waCommon.MessageKey{ID: proto.String(string(id)), FromMe: proto.Bool(sender.IsEmpty())}
	if !sender.IsEmpty() && chat.Server == types.GroupServer {
		k.Participant = proto.String(sender.String())
	}
	return &waE2E.Message{ProtocolMessage: &waE2E.ProtocolMessage{Type: waE2E.ProtocolMessage_REVOKE.Enum(), Key: k}}
}
func (f *falso) MarkRead(_ context.Context, ids []types.MessageID, _ time.Time, _, sender types.JID, extra ...types.ReceiptType) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.leidos = append(f.leidos, ids)
	for _, t := range extra {
		if t == types.ReceiptTypePlayed {
			f.played++
		}
	}
	f.leidoDe = append(f.leidoDe, sender)
	return nil
}
func (f *falso) SendChatPresence(_ context.Context, _ types.JID, st types.ChatPresence, _ types.ChatPresenceMedia) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.presencia = append(f.presencia, string(st))
	return nil
}
func (f *falso) SendPresence(_ context.Context, st types.Presence) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.presencia = append(f.presencia, "p:"+string(st))
	return nil
}
func (f *falso) SubscribePresence(context.Context, types.JID) error { return nil }
func (f *falso) GetGroupInfo(context.Context, types.JID) (*types.GroupInfo, error) {
	if f.grupo == nil {
		return nil, errors.New("sin grupo")
	}
	return f.grupo, nil
}
func (f *falso) UploadReader(_ context.Context, r io.Reader, _ io.ReadWriteSeeker, _ whatsmeow.MediaType) (whatsmeow.UploadResponse, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return whatsmeow.UploadResponse{}, err
	}
	f.mu.Lock()
	f.subidas = append(f.subidas, b)
	f.mu.Unlock()
	if f.falla != nil {
		return whatsmeow.UploadResponse{}, f.falla
	}
	return whatsmeow.UploadResponse{URL: "https://mmg/x", DirectPath: "/v/x", MediaKey: []byte("clavefalsa"), FileEncSHA256: []byte("e"), FileSHA256: []byte("s"), FileLength: uint64(len(b))}, nil
}
func (f *falso) ConfirmacionesLectura(context.Context) bool { return f.lecturaOn }
func (f *falso) Conectado() bool                            { return f.conectado }
func (f *falso) Propio() (types.JID, types.JID)             { return f.yo, f.lid }

const privado = "5215511111111@s.whatsapp.net"

func nuevoServicio(t *testing.T) (*Servicio, *falso, *almacen.Almacen) {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 3)
	}
	a, err := almacen.Abrir(t.TempDir(), k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Cerrar() })
	for _, c := range []struct {
		jid, nombre string
		g           bool
	}{{privado, "Ana", false}, {"120@g.us", "Familia", true}} {
		_, _ = a.ActualizarChat(1, c.jid, c.g, func(x *almacen.Chat) {
			x.Publico = c.nombre
			if c.g {
				x.Guardado = c.nombre
			} else {
				x.TelE164 = "5215511111111"
			}
		})
	}
	f := nuevoFalso()
	return Nuevo(1, a, f, nil), f, a
}

func recibido(t *testing.T, a *almacen.Almacen, chat, id, quien, texto string, ts int64) {
	t.Helper()
	if _, err := a.GuardarMensaje(&almacen.Mensaje{ID: id, ChatJID: chat, BotID: 1, RemitenteJID: quien, RemitenteNombre: "Ana", Tipo: almacen.TipoTexto, Texto: texto, TS: ts}); err != nil {
		t.Fatal(err)
	}
}

func TestEnviarTextoGuardaPrimeroYMarcaEnviado(t *testing.T) {
	s, f, a := nuevoServicio(t)
	r, err := s.EnviarTexto(context.Background(), privado, "  Hola Ana  ", "", nil)
	if err != nil || r.Estado != "enviado" {
		t.Fatal(r, err)
	}
	if len(f.enviados) != 1 || f.enviados[0].msg.GetConversation() != "Hola Ana" || f.enviados[0].id != r.ID {
		t.Fatalf("%+v", f.enviados)
	}
	m, _ := a.ObtenerMensaje(1, privado, r.ID)
	if m == nil || !m.EsMio || m.EstadoEnvio != "enviado" || m.Texto != "Hola Ana" || m.RemitenteJID != almacen.QuienYo {
		t.Fatalf("%+v", m)
	}
	if !s.EsEnvioPropio(privado, r.ID) {
		t.Fatal("debe reconocerse como envío de la app")
	}
	// cuenta como enviado en las estadísticas
	rs, _ := a.ResumenDeChat(1, privado)
	if rs.Enviados != 1 {
		t.Fatalf("estadísticas: %d", rs.Enviados)
	}
}

func TestEnviarSinConexionQuedaFallidoYSeReintenta(t *testing.T) {
	s, f, a := nuevoServicio(t)
	f.conectado = false
	r, err := s.EnviarTexto(context.Background(), privado, "pendiente", "", nil)
	if !errors.Is(err, ErrSinConexion) || r.Estado != "fallido" {
		t.Fatal(r, err)
	}
	m, _ := a.ObtenerMensaje(1, privado, r.ID)
	if m.EstadoEnvio != "fallido" {
		t.Fatal(m.EstadoEnvio)
	}
	f.conectado = true
	r2, err := s.Reintentar(context.Background(), privado, r.ID)
	if err != nil || r2.Estado != "enviado" || r2.ID != r.ID {
		t.Fatal(r2, err)
	}
	if m, _ := a.ObtenerMensaje(1, privado, r.ID); m.EstadoEnvio != "enviado" {
		t.Fatal("debía quedar enviado")
	}
	if _, err := s.Reintentar(context.Background(), privado, r.ID); !errors.Is(err, ErrNoExiste) {
		t.Fatal("solo se reintentan los fallidos")
	}
}

func TestErrorDeWhatsAppSeMuestraYQuedaFallido(t *testing.T) {
	s, f, a := nuevoServicio(t)
	f.falla = errors.New("rechazado por el servidor")
	r, err := s.EnviarTexto(context.Background(), privado, "x", "", nil)
	if err == nil || err.Error() != "rechazado por el servidor" {
		t.Fatal(err)
	}
	if m, _ := a.ObtenerMensaje(1, privado, r.ID); m.EstadoEnvio != "fallido" {
		t.Fatal("fallido")
	}
}

func TestCitaEnPrivadoYEnGrupoUsaElParticipantCorrecto(t *testing.T) {
	s, f, a := nuevoServicio(t)
	recibido(t, a, privado, "ORIG", privado, "¿vienes?", 1000)
	if _, err := s.EnviarTexto(context.Background(), privado, "sí", "ORIG", nil); err != nil {
		t.Fatal(err)
	}
	ci := f.enviados[0].msg.GetExtendedTextMessage().GetContextInfo()
	if ci.GetStanzaID() != "ORIG" || ci.GetParticipant() != privado || ci.GetQuotedMessage().GetConversation() != "¿vienes?" {
		t.Fatalf("cita en privado: %+v", ci)
	}
	// Grupo con direccionamiento por LID: el participante citado sale con el JID del grupo, no con el teléfono guardado.
	f.grupo = &types.GroupInfo{AddressingMode: types.AddressingModeLID, Participants: []types.GroupParticipant{{
		JID: types.NewJID("777", types.HiddenUserServer), LID: types.NewJID("777", types.HiddenUserServer), PhoneNumber: types.NewJID("5215522222222", types.DefaultUserServer),
	}}}
	recibido(t, a, "120@g.us", "G1", "5215522222222@s.whatsapp.net", "hola grupo", 1001)
	if _, err := s.EnviarTexto(context.Background(), "120@g.us", "respondo", "G1", nil); err != nil {
		t.Fatal(err)
	}
	ci = f.enviados[1].msg.GetExtendedTextMessage().GetContextInfo()
	if ci.GetParticipant() != "777@lid" {
		t.Fatalf("participant en grupo: %q", ci.GetParticipant())
	}
	// Citar un mensaje mío: el participante soy yo, con mi LID si el grupo es por LID.
	r, _ := s.EnviarTexto(context.Background(), "120@g.us", "uno mío", "", nil)
	if _, err := s.EnviarTexto(context.Background(), "120@g.us", "cito el mío", r.ID, nil); err != nil {
		t.Fatal(err)
	}
	ci = f.enviados[3].msg.GetExtendedTextMessage().GetContextInfo()
	if ci.GetParticipant() != "999@lid" {
		t.Fatalf("participant de mi mensaje: %q", ci.GetParticipant())
	}
	m, _ := a.ObtenerMensaje(1, "120@g.us", f.enviados[3].id)
	if m.CitaID != r.ID || m.CitaAutor != almacen.QuienYo || m.CitaTexto != "uno mío" {
		t.Fatalf("cita guardada: %+v", m)
	}
}

func TestMencionesSeEnvianYSeGuardan(t *testing.T) {
	s, f, a := nuevoServicio(t)
	f.grupo = &types.GroupInfo{Participants: []types.GroupParticipant{{JID: types.NewJID("5215522222222", types.DefaultUserServer), PhoneNumber: types.NewJID("5215522222222", types.DefaultUserServer)}}}
	r, err := s.EnviarTexto(context.Background(), "120@g.us", "hola @5215522222222", "", []string{"5215522222222@s.whatsapp.net"})
	if err != nil {
		t.Fatal(err)
	}
	ci := f.enviados[0].msg.GetExtendedTextMessage().GetContextInfo()
	if len(ci.GetMentionedJID()) != 1 || ci.GetMentionedJID()[0] != "5215522222222@s.whatsapp.net" {
		t.Fatalf("%+v", ci)
	}
	m, _ := a.ObtenerMensaje(1, "120@g.us", r.ID)
	if len(m.Menciones) != 1 {
		t.Fatalf("%+v", m.Menciones)
	}
	ps, err := s.Participantes(context.Background(), "120@g.us")
	if err != nil || len(ps) != 1 || ps[0].JID != "5215522222222@s.whatsapp.net" || ps[0].Telefono != "+5215522222222" {
		t.Fatal(ps, err)
	}
}

func TestPausaDelBotYTopeContraRafagas(t *testing.T) {
	s, _, a := nuevoServicio(t)
	s.PausaBot = func() time.Duration { return 30 * time.Minute }
	if _, err := s.EnviarTexto(context.Background(), privado, "hola", "", nil); err != nil {
		t.Fatal(err)
	}
	if !a.BotPausado(1, privado, time.Now().Add(29*time.Minute)) || a.BotPausado(1, privado, time.Now().Add(31*time.Minute)) {
		t.Fatal("el bot debe callar 30 min en ese chat")
	}
	if a.BotPausado(1, "120@g.us", time.Now()) {
		t.Fatal("otro chat no se pausa")
	}
	var err error
	for i := 0; i < 40 && err == nil; i++ {
		_, err = s.EnviarTexto(context.Background(), privado, "ráfaga", "", nil)
	}
	if !errors.Is(err, ErrDemasiados) {
		t.Fatal("el tope contra ráfagas debe frenar", err)
	}
}

func TestReaccionEdicionYBorradoParaTodos(t *testing.T) {
	s, f, a := nuevoServicio(t)
	recibido(t, a, privado, "R1", privado, "mira", 1000)
	if err := s.Reaccionar(context.Background(), privado, "R1", "👍"); err != nil {
		t.Fatal(err)
	}
	if r := f.enviados[0].msg.GetReactionMessage(); r.GetText() != "👍" || r.GetKey().GetFromMe() {
		t.Fatalf("%+v", r)
	}
	if a.MiReaccion(1, privado, "R1") != "👍" {
		t.Fatal("mi reacción se guarda")
	}
	if err := s.Reaccionar(context.Background(), privado, "R1", ""); err != nil || a.MiReaccion(1, privado, "R1") != "" {
		t.Fatal("quitar la reacción", err)
	}
	r, _ := s.EnviarTexto(context.Background(), privado, "original", "", nil)
	if err := s.Editar(context.Background(), privado, r.ID, "corregido"); err != nil {
		t.Fatal(err)
	}
	m, _ := a.ObtenerMensaje(1, privado, r.ID)
	if m.Texto != "corregido" || !m.Editado {
		t.Fatalf("%+v", m)
	}
	if err := s.Editar(context.Background(), privado, "R1", "x"); !errors.Is(err, ErrSoloPropios) {
		t.Fatal("solo se editan los propios", err)
	}
	f.falla = errors.New("fuera de plazo")
	if err := s.Editar(context.Background(), privado, r.ID, "tarde"); err == nil || err.Error() != "fuera de plazo" {
		t.Fatal("el error de WhatsApp se muestra", err)
	}
	f.falla = nil
	if err := s.BorrarParaTodos(context.Background(), privado, r.ID); err != nil {
		t.Fatal(err)
	}
	if rv := f.enviados[len(f.enviados)-1].msg.GetProtocolMessage(); rv.GetType() != waE2E.ProtocolMessage_REVOKE || rv.GetKey().GetID() != r.ID {
		t.Fatalf("%+v", rv)
	}
	if m, _ := a.ObtenerMensaje(1, privado, r.ID); !m.Borrado || m.Texto != "corregido" {
		t.Fatal("mi mensaje borrado para todos queda marcado y conserva el contenido")
	}
	f.falla = errors.New("no permitido")
	if err := s.BorrarParaTodos(context.Background(), privado, "R1"); err == nil || err.Error() != "no permitido" {
		t.Fatal("se intenta siempre y se muestra el error", err)
	}
}

func TestReenviarTextoConMarcaYTope(t *testing.T) {
	s, f, a := nuevoServicio(t)
	recibido(t, a, privado, "O1", privado, "para todos", 1000)
	_, _ = a.GuardarMensaje(&almacen.Mensaje{ID: "O2", ChatJID: privado, BotID: 1, RemitenteJID: privado, Tipo: almacen.TipoImagen, TS: 1001})
	r, err := s.ReenviarTexto(context.Background(), privado, []string{"O1", "O2"}, []string{"120@g.us"})
	if err != nil || r.Enviados != 1 || r.Omitidos != 1 {
		t.Fatal(r, err)
	}
	ci := f.enviados[0].msg.GetExtendedTextMessage().GetContextInfo()
	if !ci.GetIsForwarded() || ci.GetForwardingScore() != 1 {
		t.Fatalf("%+v", ci)
	}
	m, _ := a.ObtenerMensaje(1, "120@g.us", f.enviados[0].id)
	if m == nil || !m.Reenviado || m.Texto != "para todos" {
		t.Fatalf("%+v", m)
	}
	if _, err := s.ReenviarTexto(context.Background(), privado, []string{"O1"}, []string{"a@s.whatsapp.net", "b@s.whatsapp.net", "c@s.whatsapp.net", "d@s.whatsapp.net", "e@s.whatsapp.net", "f@s.whatsapp.net"}); !errors.Is(err, ErrMuchosChats) {
		t.Fatal("máximo 5 chats", err)
	}
}

func TestMarcarLeidoSoloLoNuevoYPorRemitente(t *testing.T) {
	s, f, a := nuevoServicio(t)
	recibido(t, a, "120@g.us", "G1", "5215522222222@s.whatsapp.net", "uno", 1000)
	recibido(t, a, "120@g.us", "G2", "5215533333333@s.whatsapp.net", "dos", 1001)
	recibido(t, a, "120@g.us", "G3", "5215522222222@s.whatsapp.net", "tres", 1002)
	if err := s.MarcarLeido(context.Background(), "120@g.us"); err != nil {
		t.Fatal(err)
	}
	if len(f.leidos) != 2 || len(f.leidos[0]) != 2 || f.leidoDe[0].User != "5215522222222" {
		t.Fatalf("%+v %+v", f.leidos, f.leidoDe)
	}
	if err := s.MarcarLeido(context.Background(), "120@g.us"); err != nil || len(f.leidos) != 2 {
		t.Fatal("no se vuelve a avisar lo ya leído", err)
	}
	recibido(t, a, "120@g.us", "G4", "5215522222222@s.whatsapp.net", "cuatro", 1003)
	_ = s.MarcarLeido(context.Background(), "120@g.us")
	if len(f.leidos) != 3 || len(f.leidos[2]) != 1 {
		t.Fatal("solo lo nuevo", f.leidos)
	}
}

func TestEscribiendoTieneTope(t *testing.T) {
	s, f, _ := nuevoServicio(t)
	t0 := time.Now()
	s.Ahora = func() time.Time { return t0 }
	ctx := context.Background()
	_ = s.Escribiendo(ctx, privado, true)
	_ = s.Escribiendo(ctx, privado, true) // muy pronto: no se repite
	t0 = t0.Add(9 * time.Second)
	_ = s.Escribiendo(ctx, privado, true)
	t0 = t0.Add(70 * time.Second)
	_ = s.Escribiendo(ctx, privado, true) // pasó el máximo de 60 s seguidos: ya no manda más
	_ = s.Escribiendo(ctx, privado, false)
	want := []string{"composing", "composing", "paused"}
	if len(f.presencia) != len(want) {
		t.Fatalf("%v", f.presencia)
	}
	for i := range want {
		if f.presencia[i] != want[i] {
			t.Fatalf("%v", f.presencia)
		}
	}
	_ = s.EnLinea(ctx, true)
	_ = s.EnLinea(ctx, false)
	if f.presencia[3] != "p:available" || f.presencia[4] != "p:unavailable" {
		t.Fatalf("%v", f.presencia)
	}
}

func TestBorrarParaMiYBorrador(t *testing.T) {
	_, _, a := nuevoServicio(t)
	recibido(t, nil2(a), privado, "B1", privado, "se va", 1000)
	_ = a.GuardarReaccion(1, privado, "B1", "yo", "👍", 1)
	if n, err := a.BorrarParaMi(1, privado, []string{"B1", "NO"}); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if m, _ := a.ObtenerMensaje(1, privado, "B1"); m != nil {
		t.Fatal("debía desaparecer")
	}
	if a.MiReaccion(1, privado, "B1") != "" {
		t.Fatal("sus reacciones también")
	}
	_ = a.GuardarBorrador(1, privado, "a medias")
	if a.Borrador(1, privado) != "a medias" {
		t.Fatal("borrador")
	}
	_ = a.GuardarBorrador(1, privado, "  ")
	if a.Borrador(1, privado) != "" {
		t.Fatal("borrar borrador")
	}
}

func nil2(a *almacen.Almacen) *almacen.Almacen { return a }

func TestAcusesYEstadoDeMensajes(t *testing.T) {
	s, _, a := nuevoServicio(t)
	r, _ := s.EnviarTexto(context.Background(), privado, "palomitas", "", nil)
	estado := func() string {
		l, _ := a.ListarMensajes(1, privado, 0, "", 10)
		for _, v := range l {
			if v.ID == r.ID {
				return v.EstadoEnvio
			}
		}
		return "?"
	}
	if estado() != "enviado" {
		t.Fatal(estado())
	}
	_, _ = a.GuardarAcuse(1, privado, r.ID, privado, almacen.AcuseEntregado, 2000)
	if estado() != "entregado" {
		t.Fatal(estado())
	}
	_, _ = a.GuardarAcuse(1, privado, r.ID, privado, almacen.AcuseLeido, 2005)
	if estado() != "leido" {
		t.Fatal(estado())
	}
	info, _ := a.InfoDeMensaje(1, privado, r.ID)
	if info.Estado != "leido" || len(info.Personas) != 1 || info.Personas[0].Entregado != 2000 || info.Personas[0].Leido != 2005 {
		t.Fatalf("%+v", info)
	}
	if ok, _ := a.GuardarAcuse(1, privado, "NOEXISTE", privado, almacen.AcuseLeido, 1); ok {
		t.Fatal("sin mensaje no hay acuse")
	}
}

func archivoTemporal(t *testing.T, nombre string, contenido []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), nombre)
	if err := os.WriteFile(p, contenido, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestEnviarFotoLaGuardaCifradaYBorraLaCopiaEnClaro(t *testing.T) {
	s, f, a := nuevoServicio(t)
	ruta := archivoTemporal(t, "foto.jpg", []byte("JPEG-FALSO-CONTENIDO"))
	r, err := s.EnviarMedio(context.Background(), privado, Adjunto{Tipo: almacen.TipoImagen, Ruta: ruta, Mime: "image/jpeg", Pie: "mi casa", Ancho: 800, Alto: 600, Miniatura: []byte("MINI")}, "")
	if err != nil || r.Estado != "enviado" {
		t.Fatal(r, err)
	}
	if string(f.subidas[0]) != "JPEG-FALSO-CONTENIDO" {
		t.Fatal("se sube el contenido original")
	}
	im := f.enviados[0].msg.GetImageMessage()
	if im == nil || im.GetCaption() != "mi casa" || im.GetWidth() != 800 || string(im.GetJPEGThumbnail()) != "MINI" || im.GetMimetype() != "image/jpeg" || im.GetDirectPath() != "/v/x" {
		t.Fatalf("%+v", im)
	}
	m, _ := a.ObtenerMensaje(1, privado, r.ID)
	if m == nil || m.Tipo != almacen.TipoImagen || m.ArchivoEstado != almacen.EstadoDescargado || m.EstadoEnvio != "enviado" || m.Texto != "mi casa" || m.ArchivoBytes != int64(len("JPEG-FALSO-CONTENIDO")) {
		t.Fatalf("%+v", m)
	}
	b, err := a.LeerCifrado(m.ArchivoRuta)
	if err != nil || string(b) != "JPEG-FALSO-CONTENIDO" {
		t.Fatal("la copia local debe poder leerse", err)
	}
	crudo, _ := os.ReadFile(filepath.Join(a.Raiz(), filepath.FromSlash(m.ArchivoRuta)))
	if string(crudo) == "JPEG-FALSO-CONTENIDO" || len(crudo) == 0 {
		t.Fatal("la copia local debe estar cifrada")
	}
	if _, err := os.Stat(ruta); !os.IsNotExist(err) {
		t.Fatal("la copia en claro debe borrarse")
	}
	if rs, _ := a.ResumenDeChat(1, privado); rs.Enviados != 1 {
		t.Fatal("cuenta como enviado")
	}
}

func TestNotaDeVozYDocumentoYVideo(t *testing.T) {
	s, f, _ := nuevoServicio(t)
	ctx := context.Background()
	_, err := s.EnviarMedio(ctx, privado, Adjunto{Tipo: almacen.TipoAudio, Ruta: archivoTemporal(t, "n.ogg", []byte("OGG")), Mime: "audio/ogg", DurS: 7, NotaVoz: true, Onda: []byte{1, 2, 3}}, "")
	if err != nil {
		t.Fatal(err)
	}
	au := f.enviados[0].msg.GetAudioMessage()
	if !au.GetPTT() || au.GetSeconds() != 7 || au.GetMimetype() != "audio/ogg; codecs=opus" || len(au.GetWaveform()) != 3 {
		t.Fatalf("%+v", au)
	}
	_, err = s.EnviarMedio(ctx, privado, Adjunto{Tipo: almacen.TipoDocumento, Ruta: archivoTemporal(t, "x.xlsx", []byte("XLSX")), Mime: "application/vnd.ms-excel", Nombre: "Cuentas.xlsx"}, "")
	if err != nil {
		t.Fatal(err)
	}
	d := f.enviados[1].msg.GetDocumentMessage()
	if d.GetFileName() != "Cuentas.xlsx" || d.GetMimetype() != "application/vnd.ms-excel" {
		t.Fatalf("%+v", d)
	}
	_, err = s.EnviarMedio(ctx, privado, Adjunto{Tipo: almacen.TipoVideo, Ruta: archivoTemporal(t, "v.mp4", []byte("MP4")), Mime: "video/mp4", DurS: 12, Ancho: 640, Alto: 360}, "")
	if err != nil {
		t.Fatal(err)
	}
	v := f.enviados[2].msg.GetVideoMessage()
	if v.GetSeconds() != 12 || v.GetHeight() != 360 {
		t.Fatalf("%+v", v)
	}
}

func TestMediosConCitaFalloYReintento(t *testing.T) {
	s, f, a := nuevoServicio(t)
	ctx := context.Background()
	recibido(t, a, privado, "Q1", privado, "¿me mandas la foto?", 1000)
	f.falla = errors.New("se cayó la subida")
	r, err := s.EnviarMedio(ctx, privado, Adjunto{Tipo: almacen.TipoImagen, Ruta: archivoTemporal(t, "f.jpg", []byte("FOTO")), Mime: "image/jpeg"}, "Q1")
	if err == nil || r.Estado != "fallido" {
		t.Fatal(r, err)
	}
	m, _ := a.ObtenerMensaje(1, privado, r.ID)
	if m.EstadoEnvio != "fallido" || m.CitaID != "Q1" {
		t.Fatalf("%+v", m)
	}
	f.falla = nil
	r2, err := s.Reintentar(ctx, privado, r.ID)
	if err != nil || r2.Estado != "enviado" {
		t.Fatal(r2, err)
	}
	im := f.enviados[len(f.enviados)-1].msg.GetImageMessage()
	if im == nil || im.GetContextInfo().GetStanzaID() != "Q1" {
		t.Fatalf("la cita se conserva al reintentar: %+v", im)
	}
}

func TestLimitesDeTamano(t *testing.T) {
	s, _, _ := nuevoServicio(t)
	grande := filepath.Join(t.TempDir(), "g.jpg")
	fh, _ := os.Create(grande)
	_ = fh.Truncate(maxImagen + 1)
	fh.Close()
	if _, err := s.EnviarMedio(context.Background(), privado, Adjunto{Tipo: almacen.TipoImagen, Ruta: grande}, ""); err == nil {
		t.Fatal("una foto de más de 20 MB se rechaza")
	}
	if _, err := s.EnviarMedio(context.Background(), privado, Adjunto{Tipo: "sticker", Ruta: archivoTemporal(t, "s", []byte("x"))}, ""); err == nil {
		t.Fatal("tipo no soportado")
	}
	if _, err := s.EnviarMedio(context.Background(), privado, Adjunto{Tipo: almacen.TipoImagen, Ruta: filepath.Join(t.TempDir(), "noexiste.jpg")}, ""); err == nil {
		t.Fatal("archivo inexistente")
	}
}

func TestReenviarMultimediaDescargada(t *testing.T) {
	s, f, a := nuevoServicio(t)
	ctx := context.Background()
	r, _ := s.EnviarMedio(ctx, privado, Adjunto{Tipo: almacen.TipoImagen, Ruta: archivoTemporal(t, "f.jpg", []byte("LA-FOTO")), Mime: "image/jpeg", Pie: "pie"}, "")
	_, _ = a.GuardarMensaje(&almacen.Mensaje{ID: "SIN", ChatJID: privado, BotID: 1, RemitenteJID: privado, Tipo: almacen.TipoImagen, ArchivoEstado: almacen.EstadoNinguno, TS: 5})
	res, err := s.ReenviarTexto(ctx, privado, []string{r.ID, "SIN"}, []string{"120@g.us"})
	if err != nil || res.Enviados != 1 || res.Omitidos != 1 {
		t.Fatal(res, err)
	}
	im := f.enviados[len(f.enviados)-1].msg.GetImageMessage()
	if im == nil || !im.GetContextInfo().GetIsForwarded() || string(f.subidas[len(f.subidas)-1]) != "LA-FOTO" {
		t.Fatalf("%+v", im)
	}
	m, _ := a.ObtenerMensaje(1, "120@g.us", f.enviados[len(f.enviados)-1].id)
	if m == nil || !m.Reenviado || m.Texto != "pie" || m.ArchivoRuta == "" {
		t.Fatalf("%+v", m)
	}
}

func TestVerUnaVezSeAbreUnaSolaVezYSeBorra(t *testing.T) {
	s, f, a := nuevoServicio(t)
	ctx := context.Background()
	rel := almacen.RutaMedio(1, privado, "VO1")
	if err := a.EscribirCifrado(rel, []byte("SECRETO")); err != nil {
		t.Fatal(err)
	}
	m := &almacen.Mensaje{ID: "VO1", ChatJID: privado, BotID: 1, RemitenteJID: privado, Tipo: almacen.TipoVerUnaVez, VerEstado: almacen.VerPendiente, VerTipo: "imagen",
		ArchivoEstado: almacen.EstadoDescargado, ArchivoRuta: rel, ArchivoMime: "image/jpeg", TS: 1000, VerExpira: time.Now().Add(time.Hour).Unix()}
	if _, err := a.GuardarMensaje(m); err != nil {
		t.Fatal(err)
	}
	ruta, tipo, _, err := a.AbrirVerUnaVez(1, privado, "VO1", time.Now())
	if err != nil || ruta != rel || tipo != "imagen" {
		t.Fatal(ruta, tipo, err)
	}
	if _, _, _, err := a.AbrirVerUnaVez(1, privado, "VO1", time.Now()); err == nil {
		t.Fatal("no se puede abrir dos veces")
	}
	if err := s.CerrarVerUnaVez(ctx, privado, "VO1"); err != nil {
		t.Fatal(err)
	}
	g, _ := a.ObtenerMensaje(1, privado, "VO1")
	if g.VerEstado != almacen.VerVisto || g.ArchivoRuta != "" || g.Texto != "" {
		t.Fatalf("%+v", g)
	}
	if abs, _ := a.Abs(rel); fileExists(abs) {
		t.Fatal("el archivo debía borrarse")
	}
	if f.played != 1 {
		t.Fatal("se avisa 'reproducido' una vez")
	}
	// una segunda vez no repite el aviso
	_ = s.CerrarVerUnaVez(ctx, privado, "VO1")
	if f.played != 1 {
		t.Fatal("no repetir")
	}
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

func TestVerUnaVezNoAvisaSiLasConfirmacionesEstanApagadas(t *testing.T) {
	s, f, a := nuevoServicio(t)
	f.lecturaOn = false
	rel := almacen.RutaMedio(1, privado, "VO2")
	_ = a.EscribirCifrado(rel, []byte("X"))
	_, _ = a.GuardarMensaje(&almacen.Mensaje{ID: "VO2", ChatJID: privado, BotID: 1, RemitenteJID: privado, Tipo: almacen.TipoVerUnaVez, VerEstado: almacen.VerPendiente, VerTipo: "audio", ArchivoEstado: almacen.EstadoDescargado, ArchivoRuta: rel, TS: 1})
	if _, _, _, err := a.AbrirVerUnaVez(1, privado, "VO2", time.Now()); err != nil {
		t.Fatal(err)
	}
	_ = s.CerrarVerUnaVez(context.Background(), privado, "VO2")
	if f.played != 0 {
		t.Fatal("con las confirmaciones apagadas nunca se manda 'reproducido'")
	}
}

func TestVerUnaVezBarrido(t *testing.T) {
	_, _, a := nuevoServicio(t)
	mk := func(id, est string, exp int64) {
		rel := almacen.RutaMedio(1, privado, id)
		_ = a.EscribirCifrado(rel, []byte("X"))
		_, _ = a.GuardarMensaje(&almacen.Mensaje{ID: id, ChatJID: privado, BotID: 1, RemitenteJID: privado, Tipo: almacen.TipoVerUnaVez, VerEstado: est, VerTipo: "imagen", ArchivoEstado: almacen.EstadoDescargado, ArchivoRuta: rel, TS: 1, VerExpira: exp})
	}
	ahora := time.Now()
	mk("ABIERTO", almacen.VerAbierto, ahora.Add(time.Hour).Unix()) // la app se cerró mientras se veía
	mk("VIEJO", almacen.VerPendiente, ahora.Add(-time.Hour).Unix())
	mk("VIVO", almacen.VerPendiente, ahora.Add(time.Hour).Unix())
	c, x, err := a.BarrerVerUnaVez(1, ahora)
	if err != nil || c != 1 || x != 1 {
		t.Fatal(c, x, err)
	}
	for id, esperado := range map[string]string{"ABIERTO": almacen.VerVisto, "VIEJO": almacen.VerExpirado, "VIVO": almacen.VerPendiente} {
		if m, _ := a.ObtenerMensaje(1, privado, id); m.VerEstado != esperado {
			t.Fatalf("%s: %s", id, m.VerEstado)
		}
	}
	if _, _, _, err := a.AbrirVerUnaVez(1, privado, "VIVO", ahora.Add(2*time.Hour)); err == nil {
		t.Fatal("uno caducado no se abre")
	}
}

func TestVerUnaVezBorradoPorElRemitenteNuncaSeConserva(t *testing.T) {
	_, _, a := nuevoServicio(t)
	rel := almacen.RutaMedio(1, privado, "VO3")
	_ = a.EscribirCifrado(rel, []byte("X"))
	_, _ = a.GuardarMensaje(&almacen.Mensaje{ID: "VO3", ChatJID: privado, BotID: 1, RemitenteJID: privado, Tipo: almacen.TipoVerUnaVez, VerEstado: almacen.VerPendiente, VerTipo: "imagen", ArchivoEstado: almacen.EstadoDescargado, ArchivoRuta: rel, TS: 1})
	_, _ = a.MarcarEliminado(1, privado, "VO3", "", 5)
	m, _ := a.ObtenerMensaje(1, privado, "VO3")
	if m.ArchivoRuta != "" || !m.Borrado {
		t.Fatalf("%+v", m)
	}
	if _, _, _, err := a.AbrirVerUnaVez(1, privado, "VO3", time.Now()); err == nil {
		t.Fatal("ya no se puede abrir")
	}
}
