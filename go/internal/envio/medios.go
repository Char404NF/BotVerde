package envio

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"botchat/internal/almacen"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// Adjunto es un archivo que se va a mandar. Ruta apunta a una copia en claro y temporal (que se borra al terminar).
type Adjunto struct {
	Tipo      string // imagen, video, audio o documento
	Ruta      string
	Mime      string
	Nombre    string
	Pie       string
	Ancho     int
	Alto      int
	DurS      int
	NotaVoz   bool
	Miniatura []byte // JPEG pequeño
	Onda      []byte // forma de onda de una nota de voz (hasta 64 valores de 0 a 100)
	// TopeBytes (opcional) sube el máximo de peso de este archivo (0 = el de siempre, 100 MB). Lo usan las descargas de video.
	TopeBytes int64
	// SinCopia: al terminar de enviarlo se borra también la copia cifrada que se guarda para el chat (el mensaje queda sin archivo).
	SinCopia bool
}

const (
	maxImagen = 20 << 20
	maxOtros  = 100 << 20
)

// ErrNotaVozFormato avisa que la nota de voz no está en un formato que WhatsApp reproduzca como nota de voz.
const mimeNotaDeVoz = "audio/ogg; codecs=opus"

func tipoSubida(tipo string) (whatsmeow.MediaType, error) {
	switch tipo {
	case almacen.TipoImagen, almacen.TipoSticker:
		return whatsmeow.MediaImage, nil
	case almacen.TipoVideo:
		return whatsmeow.MediaVideo, nil
	case almacen.TipoAudio:
		return whatsmeow.MediaAudio, nil
	case almacen.TipoDocumento:
		return whatsmeow.MediaDocument, nil
	}
	return "", fmt.Errorf("No se puede enviar este tipo de archivo.")
}

func (s *Servicio) carpetaTmp() string {
	d := filepath.Join(s.Alm.Raiz(), "tmp")
	_ = os.MkdirAll(d, 0o700)
	return d
}

// EnviarMedio manda una foto, un video, un audio, una nota de voz o un documento. Se guarda primero (cifrado) para verlo de inmediato.
func (s *Servicio) EnviarMedio(ctx context.Context, chat string, a Adjunto, citaID string) (Resultado, error) {
	return s.enviarMedio(ctx, chat, a, citaID, "", nil, true)
}

// EnviarMedioAuto manda un archivo por iniciativa de un módulo: igual que EnviarMedio, pero sin el tope de envíos manuales, sin
// pausar al bot en ese chat y sin contarlo como escrito a mano. Solo lo usa la cola de iniciativas.
func (s *Servicio) EnviarMedioAuto(ctx context.Context, chat string, a Adjunto, citaID string) (Resultado, error) {
	return s.enviarMedio(ctx, chat, a, citaID, "", nil, false)
}

func (s *Servicio) enviarMedio(ctx context.Context, chat string, a Adjunto, citaID, autorWA string, menciones []string, manual bool) (Resultado, error) {
	defer func() { _ = os.Remove(a.Ruta) }() // la copia en claro nunca se queda
	if _, err := tipoSubida(a.Tipo); err != nil {
		return Resultado{}, err
	}
	if a.Tipo == almacen.TipoSticker && manual {
		return Resultado{}, fmt.Errorf("No se puede enviar este tipo de archivo.") // los stickers solo los manda el bot (comando !sticker)
	}
	j, err := parsea(chat)
	if err != nil {
		return Resultado{}, err
	}
	if c, _ := s.Alm.ObtenerChat(s.BotID, chat); c == nil {
		return Resultado{}, ErrChatDesconoci
	}
	st, err := os.Stat(a.Ruta)
	if err != nil || st.Size() == 0 {
		return Resultado{}, fmt.Errorf("No se pudo leer el archivo.")
	}
	tope := int64(maxOtros)
	if a.Tipo == almacen.TipoImagen || a.Tipo == almacen.TipoSticker {
		tope = maxImagen
	} else if a.TopeBytes > tope {
		tope = a.TopeBytes
	}
	if st.Size() > tope {
		return Resultado{}, fmt.Errorf("El archivo pesa %d MB; el máximo es %d MB.", st.Size()>>20, tope>>20)
	}
	if manual && !s.tomarFicha() {
		return Resultado{}, ErrDemasiados
	}
	m := &almacen.Mensaje{
		ID: string(s.Cli.GenerateMessageID()), ChatJID: chat, BotID: s.BotID, RemitenteJID: almacen.QuienYo, EsMio: true, Tipo: a.Tipo,
		Texto: strings.TrimSpace(a.Pie), TS: s.ahora().Unix(), EstadoEnvio: "enviando", ArchivoMime: a.Mime, ArchivoNombre: a.Nombre, ArchivoBytes: st.Size(),
		Ancho: a.Ancho, Alto: a.Alto, DuracionS: a.DurS, NotaVoz: a.NotaVoz, Menciones: menciones,
	}
	if m.ArchivoMime == "" {
		m.ArchivoMime = "application/octet-stream"
	}
	if a.NotaVoz {
		m.ArchivoMime = mimeNotaDeVoz
	}
	if err := s.guardarArchivoLocal(m, a.Ruta, a.Miniatura); err != nil {
		return Resultado{}, err
	}
	if citaID != "" {
		s.ponerCita(m, citaID)
	}
	if !manual {
		m.Automatico = etiquetaAuto(ctx)
	}
	if _, err := s.Alm.GuardarMensaje(m); err != nil {
		return Resultado{}, err
	}
	s.RegistrarEnvio(m.ID)
	s.recordarAutor(m.ID, autorWA)
	if manual {
		s.pausarBot(chat)
		s.Alm.RegistrarActividad(s.BotID, almacen.ActManual, "", chat, a.Tipo)
	}
	r, err := s.transmitirMedio(ctx, j, m, a.Onda)
	if a.SinCopia {
		// el archivo ya salió (o falló): la copia cifrada del chat no se queda ocupando espacio
		_, _, _ = s.Alm.BorrarListaMotivo(s.BotID, []almacen.Item{{Chat: chat, ID: m.ID}}, "descarga")
	}
	return r, err
}

func (s *Servicio) ponerCita(m *almacen.Mensaje, citaID string) {
	if o, _ := s.Alm.ObtenerMensaje(s.BotID, m.ChatJID, citaID); o != nil {
		m.CitaID = citaID
		if o.EsMio {
			m.CitaAutor = almacen.QuienYo
		} else {
			m.CitaAutor = o.RemitenteJID
		}
		m.CitaTexto = almacen.ResumenCorto(o)
	}
}

// guardarArchivoLocal copia el archivo (y su miniatura) a la carpeta de datos, cifrados.
func (s *Servicio) guardarArchivoLocal(m *almacen.Mensaje, ruta string, mini []byte) error {
	f, err := os.Open(ruta)
	if err != nil {
		return fmt.Errorf("No se pudo leer el archivo.")
	}
	defer f.Close()
	rel := almacen.RutaMedio(s.BotID, m.ChatJID, m.ID)
	n, err := s.Alm.EscribirCifradoDesde(rel, f)
	if err != nil {
		return err
	}
	m.ArchivoRuta, m.ArchivoBytes, m.ArchivoEstado = rel, n, almacen.EstadoDescargado
	if len(mini) > 0 {
		mr := almacen.RutaMiniatura(s.BotID, m.ChatJID, m.ID)
		if err := s.Alm.EscribirCifrado(mr, mini); err == nil {
			m.MiniaturaRuta = mr
		}
	}
	return nil
}

// transmitirMedio sube el archivo guardado y manda el mensaje.
func (s *Servicio) transmitirMedio(ctx context.Context, j types.JID, m *almacen.Mensaje, onda []byte) (Resultado, error) {
	res := Resultado{ID: m.ID, Estado: "fallido", TS: m.TS}
	falla := func(err error) (Resultado, error) {
		_ = s.Alm.EstadoDeEnvio(s.BotID, m.ChatJID, m.ID, "fallido")
		s.Log("No se pudo enviar un archivo: " + err.Error())
		return res, err
	}
	if !s.Cli.Conectado() {
		_ = s.Alm.EstadoDeEnvio(s.BotID, m.ChatJID, m.ID, "fallido")
		return res, ErrSinConexion
	}
	mt, err := tipoSubida(m.Tipo)
	if err != nil {
		return falla(err)
	}
	lector, err := s.Alm.AbrirDescifrado(m.ArchivoRuta)
	if err != nil {
		return falla(fmt.Errorf("No se pudo leer el archivo guardado."))
	}
	defer lector.Close()
	tmp, err := os.CreateTemp(s.carpetaTmp(), "subida-*")
	if err != nil {
		return falla(err)
	}
	defer func() { tmp.Close(); _ = os.Remove(tmp.Name()) }()
	up, err := s.Cli.UploadReader(ctx, lector, tmp, mt)
	if err != nil {
		return falla(err)
	}
	var mini []byte
	if m.MiniaturaRuta != "" {
		mini, _ = s.Alm.LeerCifrado(m.MiniaturaRuta)
	}
	var ci *waE2E.ContextInfo
	if m.CitaID != "" || m.Reenviado || len(m.Menciones) > 0 {
		autor := s.autorDeCita(ctx, j, m)
		ci = contexto(m.CitaID, autor, m.CitaTexto, m.Menciones, m.Reenviado)
	}
	pie := proto.String(m.Texto)
	if m.Texto == "" {
		pie = nil
	}
	var msg *waE2E.Message
	switch m.Tipo {
	case almacen.TipoImagen:
		msg = &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
			URL: &up.URL, DirectPath: &up.DirectPath, MediaKey: up.MediaKey, Mimetype: proto.String(m.ArchivoMime), FileEncSHA256: up.FileEncSHA256,
			FileSHA256: up.FileSHA256, FileLength: &up.FileLength, Caption: pie, JPEGThumbnail: mini, Width: proto.Uint32(uint32(m.Ancho)), Height: proto.Uint32(uint32(m.Alto)), ContextInfo: ci}}
	case almacen.TipoSticker:
		msg = &waE2E.Message{StickerMessage: &waE2E.StickerMessage{
			URL: &up.URL, DirectPath: &up.DirectPath, MediaKey: up.MediaKey, Mimetype: proto.String("image/webp"), FileEncSHA256: up.FileEncSHA256,
			FileSHA256: up.FileSHA256, FileLength: &up.FileLength, Width: proto.Uint32(uint32(m.Ancho)), Height: proto.Uint32(uint32(m.Alto)),
			IsAnimated: proto.Bool(false), ContextInfo: ci}}
	case almacen.TipoVideo:
		msg = &waE2E.Message{VideoMessage: &waE2E.VideoMessage{
			URL: &up.URL, DirectPath: &up.DirectPath, MediaKey: up.MediaKey, Mimetype: proto.String(m.ArchivoMime), FileEncSHA256: up.FileEncSHA256,
			FileSHA256: up.FileSHA256, FileLength: &up.FileLength, Caption: pie, JPEGThumbnail: mini, Seconds: proto.Uint32(uint32(m.DuracionS)),
			Width: proto.Uint32(uint32(m.Ancho)), Height: proto.Uint32(uint32(m.Alto)), ContextInfo: ci}}
	case almacen.TipoAudio:
		msg = &waE2E.Message{AudioMessage: &waE2E.AudioMessage{
			URL: &up.URL, DirectPath: &up.DirectPath, MediaKey: up.MediaKey, Mimetype: proto.String(m.ArchivoMime), FileEncSHA256: up.FileEncSHA256,
			FileSHA256: up.FileSHA256, FileLength: &up.FileLength, Seconds: proto.Uint32(uint32(m.DuracionS)), PTT: proto.Bool(m.NotaVoz), Waveform: onda, ContextInfo: ci}}
	case almacen.TipoDocumento:
		nombre := m.ArchivoNombre
		if nombre == "" {
			nombre = "archivo"
		}
		msg = &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{
			URL: &up.URL, DirectPath: &up.DirectPath, MediaKey: up.MediaKey, Mimetype: proto.String(m.ArchivoMime), FileEncSHA256: up.FileEncSHA256,
			FileSHA256: up.FileSHA256, FileLength: &up.FileLength, FileName: proto.String(nombre), Title: proto.String(nombre), Caption: pie, JPEGThumbnail: mini, ContextInfo: ci}}
	}
	if _, err := s.Cli.SendMessage(ctx, j, msg, whatsmeow.SendRequestExtra{ID: types.MessageID(m.ID)}); err != nil {
		return falla(err)
	}
	_ = s.Alm.EstadoDeEnvio(s.BotID, m.ChatJID, m.ID, "enviado")
	s.Log("Archivo enviado desde la app (manual) al chat …" + sufijo(m.ChatJID) + ".")
	res.Estado = "enviado"
	return res, nil
}

// reenviarMedio copia un archivo ya descargado a otro chat y lo manda con la marca "Reenviado".
func (s *Servicio) reenviarMedio(ctx context.Context, o *almacen.Mensaje, destino string) error {
	j, err := parsea(destino)
	if err != nil {
		return err
	}
	if c, _ := s.Alm.ObtenerChat(s.BotID, destino); c == nil {
		return ErrChatDesconoci
	}
	if !s.tomarFicha() {
		return ErrDemasiados
	}
	m := &almacen.Mensaje{
		ID: string(s.Cli.GenerateMessageID()), ChatJID: destino, BotID: s.BotID, RemitenteJID: almacen.QuienYo, EsMio: true, Tipo: o.Tipo,
		Texto: o.Texto, TS: s.ahora().Unix(), EstadoEnvio: "enviando", ArchivoMime: o.ArchivoMime, ArchivoNombre: o.ArchivoNombre,
		Ancho: o.Ancho, Alto: o.Alto, DuracionS: o.DuracionS, NotaVoz: o.NotaVoz, Reenviado: true,
	}
	lector, err := s.Alm.AbrirDescifrado(o.ArchivoRuta)
	if err != nil {
		return fmt.Errorf("No se pudo leer el archivo guardado.")
	}
	rel := almacen.RutaMedio(s.BotID, destino, m.ID)
	n, err := s.Alm.EscribirCifradoDesde(rel, lector)
	lector.Close()
	if err != nil {
		return err
	}
	m.ArchivoRuta, m.ArchivoBytes, m.ArchivoEstado = rel, n, almacen.EstadoDescargado
	if o.MiniaturaRuta != "" {
		if mini, err := s.Alm.LeerCifrado(o.MiniaturaRuta); err == nil {
			mr := almacen.RutaMiniatura(s.BotID, destino, m.ID)
			if s.Alm.EscribirCifrado(mr, mini) == nil {
				m.MiniaturaRuta = mr
			}
		}
	}
	if _, err := s.Alm.GuardarMensaje(m); err != nil {
		return err
	}
	s.RegistrarEnvio(m.ID)
	s.pausarBot(destino)
	res, err := s.transmitirMedio(ctx, j, m, nil)
	if err == nil && res.Estado != "enviado" {
		err = fmt.Errorf("no se pudo enviar")
	}
	return err
}

// puedeReenviarse dice si ese mensaje se puede reenviar (texto, o multimedia ya descargada).
func puedeReenviarse(m *almacen.Mensaje) bool {
	if m == nil || m.Borrado && m.Texto == "" && m.ArchivoRuta == "" {
		return false
	}
	switch m.Tipo {
	case almacen.TipoTexto:
		return strings.TrimSpace(m.Texto) != ""
	case almacen.TipoImagen, almacen.TipoVideo, almacen.TipoAudio, almacen.TipoDocumento:
		return m.ArchivoEstado == almacen.EstadoDescargado && m.ArchivoRuta != ""
	}
	return false
}

// CerrarVerUnaVez termina de ver un "ver una vez": se borra para siempre y, si la cuenta tiene activadas las confirmaciones de lectura,
// se avisa "reproducido" como lo hace WhatsApp.
func (s *Servicio) CerrarVerUnaVez(ctx context.Context, chat, id string) error {
	m, _ := s.Alm.ObtenerMensaje(s.BotID, chat, id)
	ok, err := s.Alm.CerrarVerUnaVez(s.BotID, chat, id)
	if err != nil || !ok || m == nil || m.EsMio {
		return err
	}
	if !s.Cli.Conectado() || !s.Cli.ConfirmacionesLectura(ctx) {
		return nil
	}
	j, err := parsea(chat)
	if err != nil {
		return nil
	}
	sender := types.EmptyJID
	if s.esGrupo(j) {
		if sj, e := types.ParseJID(m.RemitenteJID); e == nil {
			sender = sj
		}
	}
	return s.Cli.MarkRead(ctx, []types.MessageID{types.MessageID(id)}, time.Now(), j, sender, types.ReceiptTypePlayed)
}
