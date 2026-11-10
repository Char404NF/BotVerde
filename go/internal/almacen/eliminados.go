package almacen

import (
	"strings"
	"time"

	"botchat/internal/cifrado"
)

// ConservarEliminados dice si se conserva el contenido de lo que otros eliminan (por defecto sí).
func (a *Almacen) ConservarEliminados() bool { return a.conservar.Load() != 2 }

// FijarConservarEliminados enciende o apaga la conservación de mensajes eliminados por otros.
func (a *Almacen) FijarConservarEliminados(si bool) {
	if si {
		a.conservar.Store(1)
	} else {
		a.conservar.Store(2)
	}
}

// MarcarEliminado anota que llegó la eliminación "para todos" de un mensaje. Con la conservación encendida NO toca el contenido
// ni el archivo: solo guarda cuándo llegó y quién lo eliminó (si no fue su autor). Apagada, sobrescribe el contenido y borra el archivo.
// `por` es el JID de quien envió la eliminación. Devuelve false si el mensaje no existe.
func (a *Almacen) MarcarEliminado(botID int64, chat, id, por string, ts int64) (bool, error) {
	m, err := a.ObtenerMensaje(botID, chat, id)
	if err != nil || m == nil {
		return false, err
	}
	if ts <= 0 {
		ts = time.Now().Unix()
	}
	if por == m.RemitenteJID || (m.EsMio && por == "") {
		por = ""
	}
	if m.Borrado && m.EliminadoEn > 0 {
		return true, nil // ya estaba marcado: no se pisa la hora original
	}
	if _, err := a.db.Exec("UPDATE messages SET borrado=1, eliminado_en=?, eliminado_por=? WHERE bot_id=? AND chat_jid=? AND id=?", ts, por, botID, chat, id); err != nil {
		return false, err
	}
	if m.Tipo == TipoVerUnaVez || (!a.ConservarEliminados() && !m.EsMio) { // un "ver una vez" que el remitente borra nunca se conserva
		if err := a.olvidar(m); err != nil {
			return false, err
		}
	}
	a.avisaMsg(chat, id)
	a.avisa("chats")
	a.avisa("estadisticas")
	return true, nil
}

// OlvidarMensaje borra para siempre el contenido y el archivo de un mensaje (queda como "Eliminado · contenido no disponible").
func (a *Almacen) OlvidarMensaje(botID int64, chat, id string) (bool, error) {
	m, err := a.ObtenerMensaje(botID, chat, id)
	if err != nil || m == nil {
		return false, err
	}
	if _, err := a.db.Exec("UPDATE messages SET borrado=1, eliminado_en=CASE WHEN eliminado_en=0 THEN ? ELSE eliminado_en END WHERE bot_id=? AND chat_jid=? AND id=?",
		time.Now().Unix(), botID, chat, id); err != nil {
		return false, err
	}
	if err := a.olvidar(m); err != nil {
		return false, err
	}
	a.avisaMsg(chat, id)
	a.avisa("chats")
	a.avisa("espacio")
	a.avisa("estadisticas")
	return true, nil
}

// olvidar sobrescribe el texto, el nombre y los datos de descarga, y borra archivo y miniatura.
func (a *Almacen) olvidar(m *Mensaje) error {
	vacio, err := cifrado.SellarTexto(a.clave, "texto", nil)
	if err != nil {
		return err
	}
	nombre, err := cifrado.SellarTexto(a.clave, "archivo_nombre", nil)
	if err != nil {
		return err
	}
	info, err := cifrado.SellarTexto(a.clave, "archivo_info", nil)
	if err != nil {
		return err
	}
	for _, rel := range []string{m.ArchivoRuta, m.MiniaturaRuta} {
		if rel != "" {
			a.quita(rel)
		}
	}
	_, err = a.db.Exec(`UPDATE messages SET texto_cifrado=?, archivo_nombre_cifrado=?, archivo_info_cifrado=?, archivo_ruta='', miniatura_ruta='',
		archivo_estado=CASE WHEN tipo IN ('imagen','video','audio','documento','sticker') THEN 'borrado' ELSE archivo_estado END, ancho=0, alto=0
		WHERE bot_id=? AND chat_jid=? AND id=?`, vacio, nombre, info, m.BotID, m.ChatJID, m.ID)
	return err
}

// Eliminados cuenta los mensajes eliminados de un chat (de todos si chat="").
func (a *Almacen) Eliminados(botID int64, chat string) int64 {
	var n int64
	if chat == "" {
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE bot_id=? AND borrado=1`, botID).Scan(&n)
	} else {
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE bot_id=? AND chat_jid=? AND borrado=1`, botID, chat).Scan(&n)
	}
	return n
}

// ResumenCorto es el texto breve de un mensaje (para citas y vistas previas).
func ResumenCorto(m *Mensaje) string {
	corto := func(s string) string {
		s = strings.Join(strings.Fields(s), " ")
		if r := []rune(s); len(r) > 140 {
			return string(r[:140]) + "…"
		}
		return s
	}
	con := func(etiqueta, pie string) string {
		if strings.TrimSpace(pie) == "" {
			return etiqueta
		}
		return etiqueta + " · " + corto(pie)
	}
	switch m.Tipo {
	case TipoImagen:
		return con("Foto", m.Texto)
	case TipoVideo:
		return con("Video", m.Texto)
	case TipoAudio:
		if m.NotaVoz {
			return "Nota de voz"
		}
		return "Audio"
	case TipoDocumento:
		return con("Documento", m.ArchivoNombre)
	case TipoSticker:
		return "Sticker"
	case TipoContacto, TipoUbicacion:
		return VistaPrevia(m.Tipo, m.Texto)
	case TipoVerUnaVez:
		return "Ver una vez"
	case TipoOtro:
		return "Mensaje especial"
	}
	return corto(m.Texto)
}

// BorrarParaMi quita mensajes de este teléfono (con sus archivos, acuses y reacciones). No toca WhatsApp ni las estadísticas ya contadas.
func (a *Almacen) BorrarParaMi(botID int64, chat string, ids []string) (int, error) {
	n := 0
	for _, id := range ids {
		m, err := a.ObtenerMensaje(botID, chat, id)
		if err != nil {
			return n, err
		}
		if m == nil {
			continue
		}
		a.quita(m.ArchivoRuta)
		a.quita(m.MiniaturaRuta)
		for _, q := range []string{"DELETE FROM messages", "DELETE FROM message_receipts", "DELETE FROM reacciones"} {
			col := "id"
			if q != "DELETE FROM messages" {
				col = "msg_id"
			}
			if _, err := a.db.Exec(q+" WHERE bot_id=? AND chat_jid=? AND "+col+"=?", botID, chat, id); err != nil {
				return n, err
			}
		}
		n++
	}
	if n > 0 {
		a.nombres.limpiar()
		for _, id := range ids {
			a.avisa("msg:" + chat + ":" + id)
		}
		a.avisa("chat:" + chat)
		a.avisa("chats")
		a.avisa("espacio")
	}
	return n, nil
}
