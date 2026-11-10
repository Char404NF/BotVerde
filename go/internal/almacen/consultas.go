package almacen

import (
	"strings"

	"botchat/internal/identidad"
)

// Categorías de archivos para filtrar y sumar.
const (
	CatFoto      = "foto"
	CatVideo     = "video"
	CatAudio     = "audio"
	CatDocumento = "documento"
)

// Categorias son todas, en el orden en que se muestran.
var Categorias = []string{CatFoto, CatVideo, CatAudio, CatDocumento}

// CategoriaDe dice a qué categoría pertenece un tipo de mensaje ("" si no es multimedia).
func CategoriaDe(tipo string) string {
	switch tipo {
	case TipoImagen, TipoSticker:
		return CatFoto
	case TipoVideo:
		return CatVideo
	case TipoAudio:
		return CatAudio
	case TipoDocumento:
		return CatDocumento
	}
	return ""
}

func tiposDe(cat string) []string {
	switch cat {
	case CatFoto:
		return []string{TipoImagen, TipoSticker}
	case CatVideo:
		return []string{TipoVideo}
	case CatAudio:
		return []string{TipoAudio}
	case CatDocumento:
		return []string{TipoDocumento}
	}
	return nil
}

// ChatLista es una fila de la lista de chats.
type ChatLista struct {
	JID           string `json:"jid"` // clave interna: la app nunca la muestra
	Nombre        string `json:"nombre"`
	Telefono      string `json:"telefono"`
	Pais          string `json:"pais"`
	EsGrupo       bool   `json:"esGrupo"`
	Foto          string `json:"foto"`
	FotoTS        int64  `json:"fotoTs"`
	UltimoTS      int64  `json:"ultimoTs"`
	UltimoTexto   string `json:"ultimoTexto"`
	UltimoTipo    string `json:"ultimoTipo"`
	UltimoBorrado bool   `json:"ultimoBorrado"`
	UltimoMio     bool   `json:"ultimoMio"`
}

// ListarChats devuelve los chats de personas o de grupos, del más reciente al más viejo.
// consulta busca por nombre (sin acentos ni mayúsculas) y por número (solo dígitos).
func (a *Almacen) ListarChats(botID int64, grupos bool, consulta string, limite, desde int) ([]ChatLista, error) {
	if limite <= 0 || limite > 500 {
		limite = 100
	}
	q := `SELECT c.jid,c.nombre_mostrado,c.telefono_mostrado,c.pais_iso,c.es_grupo,c.foto_ruta,c.foto_ts,c.ultimo_ts,
		m.tipo,m.texto_cifrado,m.borrado,m.es_mio
		FROM chats c LEFT JOIN messages m ON m.rowid = (
			SELECT rowid FROM messages WHERE bot_id=c.bot_id AND chat_jid=c.jid ORDER BY ts DESC, id DESC LIMIT 1)
		WHERE c.bot_id=? AND c.es_grupo=?`
	args := []any{botID, b2i(grupos)}
	if t := strings.TrimSpace(consulta); t != "" {
		q += ` AND c.busqueda LIKE ? ESCAPE '\'`
		args = append(args, "%"+escapaLike(identidad.Normalizar(t))+"%")
	}
	// Los chats sin mensajes (solo creados por nombres de contacto) no se listan.
	q += ` AND m.id IS NOT NULL ORDER BY c.ultimo_ts DESC, c.jid LIMIT ? OFFSET ?`
	args = append(args, limite, desde)
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []ChatLista{}
	for filas.Next() {
		var c ChatLista
		var g, borrado, mio int
		var tipo *string
		var texto []byte
		if err := filas.Scan(&c.JID, &c.Nombre, &c.Telefono, &c.Pais, &g, &c.Foto, &c.FotoTS, &c.UltimoTS, &tipo, &texto, &borrado, &mio); err != nil {
			return nil, err
		}
		c.EsGrupo, c.UltimoBorrado, c.UltimoMio = g == 1, borrado == 1, mio == 1
		if tipo != nil {
			c.UltimoTipo = *tipo
		}
		if c.UltimoTipo != TipoVerUnaVez && !c.UltimoBorrado {
			c.UltimoTexto = a.abreTexto("texto", texto)
		}
		if c.UltimoTipo == TipoVerUnaVez {
			c.UltimoTexto = TextoVerUnaVez
		}
		out = append(out, c)
	}
	return out, filas.Err()
}

func escapaLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// MensajeVista es un mensaje listo para mostrar.
type MensajeVista struct {
	ID           string `json:"id"`
	Chat         string `json:"chat"`
	RemitenteID  string `json:"remitenteId"` // clave interna para agrupar y dar color; la app no la muestra
	Remitente    string `json:"remitente"`
	EsMio        bool   `json:"esMio"`
	Tipo         string `json:"tipo"`
	Texto        string `json:"texto"`
	TS           int64  `json:"ts"`
	Borrado      bool   `json:"borrado"`
	Editado      bool   `json:"editado"`
	Estado       string `json:"estado"` // estado del archivo
	Bytes        int64  `json:"bytes"`
	Mime         string `json:"mime"`
	Nombre       string `json:"nombre"` // nombre del archivo (documentos)
	Ruta         string `json:"ruta"`
	Miniatura    string `json:"miniatura"`
	Ancho        int    `json:"ancho"`
	Alto         int    `json:"alto"`
	DuracionS    int    `json:"dur"`
	NotaVoz      bool   `json:"notaVoz"`
	PuedeBajarse bool   `json:"puedeBajarse"`
}

func (a *Almacen) vista(m *Mensaje) MensajeVista {
	v := MensajeVista{
		ID: m.ID, Chat: m.ChatJID, RemitenteID: m.RemitenteJID, Remitente: m.RemitenteNombre, EsMio: m.EsMio, Tipo: m.Tipo, Texto: m.Texto,
		TS: m.TS, Borrado: m.Borrado, Editado: m.Editado, Estado: m.ArchivoEstado, Bytes: m.ArchivoBytes, Mime: m.ArchivoMime,
		Nombre: m.ArchivoNombre, Ruta: m.ArchivoRuta, Miniatura: m.MiniaturaRuta, Ancho: m.Ancho, Alto: m.Alto, DuracionS: m.DuracionS, NotaVoz: m.NotaVoz,
	}
	v.PuedeBajarse = CategoriaDe(m.Tipo) != "" && len(m.ArchivoInfo) > 0 &&
		(m.ArchivoEstado == EstadoNinguno || m.ArchivoEstado == EstadoOmitido || m.ArchivoEstado == EstadoFallido)
	return v
}

// ListarMensajes devuelve hasta `limite` mensajes de un chat, del más nuevo al más viejo.
// Para la página siguiente se pasa la hora y el id del último mensaje recibido (antesTS, antesID); con antesTS=0 empieza por los últimos.
func (a *Almacen) ListarMensajes(botID int64, chat string, antesTS int64, antesID string, limite int) ([]MensajeVista, error) {
	if limite <= 0 || limite > 500 {
		limite = 50
	}
	q := "SELECT " + colsMensaje + " FROM messages WHERE bot_id=? AND chat_jid=?"
	args := []any{botID, chat}
	if antesTS > 0 {
		q += " AND (ts < ? OR (ts = ? AND id < ?))"
		args = append(args, antesTS, antesTS, antesID)
	}
	q += " ORDER BY ts DESC, id DESC LIMIT ?"
	args = append(args, limite)
	return a.vistasDe(q, args)
}

func (a *Almacen) vistasDe(q string, args []any) ([]MensajeVista, error) {
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []MensajeVista{}
	for filas.Next() {
		m, err := a.escanearMensaje(filas)
		if err != nil {
			return nil, err
		}
		out = append(out, a.vista(m))
	}
	return out, filas.Err()
}

// ListarArchivos devuelve los mensajes con multimedia de un chat (o de todos si chat=""), del más nuevo al más viejo.
// categoria: foto, video, audio, documento o "" para todas. soloDescargados limita a los que ya están en el teléfono;
// si es falso se incluyen también los que esperan un botón "Descargar" (pero no los "ver una vez").
func (a *Almacen) ListarArchivos(botID int64, chat, categoria string, soloDescargados bool, limite, desde int) ([]MensajeVista, error) {
	if limite <= 0 || limite > 1000 {
		limite = 100
	}
	tipos := tiposMedia(categoria)
	q := "SELECT " + colsMensaje + " FROM messages WHERE bot_id=? AND tipo IN (" + marcas(len(tipos)) + ")"
	args := []any{botID}
	for _, t := range tipos {
		args = append(args, t)
	}
	if chat != "" {
		q += " AND chat_jid=?"
		args = append(args, chat)
	}
	if soloDescargados {
		q += " AND archivo_estado='descargado'"
	}
	q += " ORDER BY ts DESC, id DESC LIMIT ? OFFSET ?"
	args = append(args, limite, desde)
	return a.vistasDe(q, args)
}

func tiposMedia(cat string) []string {
	if t := tiposDe(cat); t != nil {
		return t
	}
	return []string{TipoImagen, TipoSticker, TipoVideo, TipoAudio, TipoDocumento}
}

func marcas(n int) string {
	if n <= 0 {
		return "NULL"
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func (a *Almacen) abreTexto(col string, b []byte) string {
	if len(b) == 0 {
		return ""
	}
	t, err := a.abre(col, b)
	if err != nil {
		return ""
	}
	return t
}

func (a *Almacen) abre(col string, b []byte) (string, error) {
	return abrirTextoCol(a.clave, col, b)
}
