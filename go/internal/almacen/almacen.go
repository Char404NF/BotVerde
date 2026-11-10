// Package almacen guarda chats y mensajes en messages.db (separada de la sesión de whatsmeow).
// Los textos y los datos para descargar multimedia van cifrados por columna; chat, remitente, hora, tipo y tamaño no,
// para poder listar y ordenar. Los archivos van cifrados en disco con el formato de internal/cifrado.
package almacen

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"botchat/internal/cifrado"
	"botchat/internal/identidad"

	_ "modernc.org/sqlite" // SQLite en Go puro (sin cgo)
)

// Tipos de mensaje.
const (
	TipoTexto     = "texto"
	TipoImagen    = "imagen"
	TipoVideo     = "video"
	TipoAudio     = "audio"
	TipoDocumento = "documento"
	TipoSticker   = "sticker"
	TipoContacto  = "contacto"
	TipoUbicacion = "ubicacion"
	TipoEncuesta  = "encuesta"
	TipoVerUnaVez = "ver_una_vez"
	TipoOtro      = "otro"
)

// Estados del archivo de un mensaje.
const (
	EstadoNinguno    = "ninguno"
	EstadoPendiente  = "pendiente"
	EstadoDescargado = "descargado"
	EstadoBorrado    = "borrado"
	EstadoFallido    = "fallido"
	EstadoOmitido    = "omitido"
)

// TextoVerUnaVez es lo único que se guarda de un mensaje de "ver una vez".
const TextoVerUnaVez = "[Ver una vez: ábrelo en tu teléfono]"

// Almacen es la base de mensajes. Es seguro para uso concurrente.
type Almacen struct {
	db        *sql.DB
	raiz      string
	clave     []byte
	conservar atomic.Int32                  // 0 = por defecto (conservar), 1 = conservar, 2 = no conservar eliminados
	zona      atomic.Pointer[time.Location] // zona horaria del teléfono para agrupar por día (la fija la app)
	mu        sync.Mutex                    // serializa lectura-modificación-escritura de chats
	// Cambio, si no es nil, se llama cuando algo cambia (sin bloquear la operación). Los eventos son:
	// "chats", "chat:<jid>", "archivo:<jid>:<id>" y "espacio".
	Cambio func(evento string)
}

// Abrir abre (o crea) raiz/messages.db y aplica las migraciones pendientes.
func Abrir(raiz string, clave []byte) (*Almacen, error) {
	if len(clave) != cifrado.TamClave {
		return nil, cifrado.ErrClave
	}
	if err := os.MkdirAll(raiz, 0o700); err != nil {
		return nil, err
	}
	dsn := "file:" + filepath.Join(raiz, "messages.db") +
		"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(30000)&_pragma=foreign_keys(1)&_pragma=temp_store(2)&_pragma=synchronous(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	a := &Almacen{db: db, raiz: raiz, clave: append([]byte(nil), clave...)}
	if err := a.migrar(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migraciones: %w", err)
	}
	return a, nil
}

// Cerrar cierra la base.
func (a *Almacen) Cerrar() error {
	for i := range a.clave {
		a.clave[i] = 0
	}
	return a.db.Close()
}

// Raiz es la carpeta donde viven la base y los archivos.
func (a *Almacen) Raiz() string { return a.raiz }

func (a *Almacen) avisa(evento string) {
	if f := a.Cambio; f != nil {
		f(evento)
	}
}

// ---------- rutas de archivos (relativas a la raíz) ----------

var idSeguro = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)

func nombreSeguro(id string) string {
	if idSeguro.MatchString(id) {
		return id
	}
	h := sha256.Sum256([]byte(id))
	return "x" + hex.EncodeToString(h[:10])
}

// HashChat identifica un chat en los nombres de archivo sin escribir su JID.
func HashChat(botID int64, jid string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", botID, jid)))
	return hex.EncodeToString(h[:8])
}

// RutaMedio es media/<bot>/<hash del chat>/<id>.enc
func RutaMedio(botID int64, chat, id string) string {
	return filepath.ToSlash(filepath.Join("media", fmt.Sprint(botID), HashChat(botID, chat), nombreSeguro(id)+".enc"))
}

// RutaMiniatura es media/<bot>/<hash del chat>/<id>.thumb.enc
func RutaMiniatura(botID int64, chat, id string) string {
	return filepath.ToSlash(filepath.Join("media", fmt.Sprint(botID), HashChat(botID, chat), nombreSeguro(id)+".thumb.enc"))
}

// RutaFoto es avatars/<bot>/<hash del chat>.enc (o _full.enc para la foto grande).
func RutaFoto(botID int64, chat string, grande bool) string {
	n := HashChat(botID, chat)
	if grande {
		n += "_full"
	}
	return filepath.ToSlash(filepath.Join("avatars", fmt.Sprint(botID), n+".enc"))
}

// Abs convierte una ruta relativa de la base en ruta absoluta, rechazando cualquier salida de la raíz.
func (a *Almacen) Abs(rel string) (string, error) {
	if rel == "" {
		return "", errors.New("ruta vacía")
	}
	p := filepath.Join(a.raiz, filepath.FromSlash(rel))
	r, err := filepath.Rel(a.raiz, p)
	if err != nil || strings.HasPrefix(r, "..") {
		return "", errors.New("ruta fuera de la carpeta de datos")
	}
	return p, nil
}

// EscribirCifrado escribe contenido ya en memoria como archivo cifrado, de forma atómica (.tmp y renombrar).
func (a *Almacen) EscribirCifrado(rel string, contenido []byte) error {
	abs, err := a.Abs(rel)
	if err != nil {
		return err
	}
	cif, err := cifrado.CifrarBytes(a.clave, contenido)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return err
	}
	tmp := abs + ".tmp"
	if err := os.WriteFile(tmp, cif, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, abs)
}

// EscribirCifradoDesde cifra en flujo lo que entrega r y lo deja en rel, de forma atómica. Nunca escribe el contenido en claro.
// Devuelve cuántos bytes de contenido (sin cifrar) se guardaron.
func (a *Almacen) EscribirCifradoDesde(rel string, r io.Reader) (int64, error) {
	abs, err := a.Abs(rel)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return 0, err
	}
	tmp := abs + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, err
	}
	w, err := cifrado.NuevoEscritor(f, a.clave)
	if err != nil {
		f.Close()
		os.Remove(tmp)
		return 0, err
	}
	n, err := io.Copy(w, r)
	if err == nil {
		err = w.Close()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return 0, err
	}
	if err := os.Rename(tmp, abs); err != nil {
		os.Remove(tmp)
		return 0, err
	}
	return n, nil
}

// LeerCifrado lee y descifra un archivo completo (para fotos pequeñas y miniaturas).
func (a *Almacen) LeerCifrado(rel string) ([]byte, error) {
	abs, err := a.Abs(rel)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	return cifrado.DescifrarBytes(a.clave, b)
}

// TamCifrado calcula cuánto mide en disco un contenido de n bytes.
func TamCifrado(n int64) int64 {
	bloques := (n + cifrado.TamBloque - 1) / cifrado.TamBloque
	if bloques == 0 {
		bloques = 1
	}
	return cifrado.TamCabecera + n + bloques*cifrado.TamEtiqueta
}

// ---------- chats ----------

// Chat es una conversación (persona o grupo).
type Chat struct {
	JID         string
	BotID       int64
	EsGrupo     bool
	Guardado    string // nombre de la agenda, o asunto del grupo
	Publico     string
	Negocio     string
	Mostrado    string
	TelE164     string
	TelMostrado string
	PaisISO     string
	FotoRuta    string
	FotoID      string
	FotoTS      int64
	UltimoTS    int64
}

// Recalcular rellena Mostrado, TelMostrado y PaisISO a partir de los demás datos.
func (c *Chat) Recalcular() {
	tel := identidad.ParseTelefono(c.TelE164)
	if c.TelE164 != "" {
		c.PaisISO = tel.ISO
	}
	c.TelMostrado = identidad.TelefonoMostrado(c.EsGrupo, tel)
	c.Mostrado = identidad.NombreMostrado(identidad.Datos{
		EsGrupo: c.EsGrupo, Asunto: c.Guardado, Guardado: c.Guardado, Publico: c.Publico, Negocio: c.Negocio, Telefono: tel.Mostrado,
	})
}

func (c *Chat) busqueda() string {
	return identidad.TextoBusqueda(c.Mostrado, identidad.Telefono{E164: c.TelE164})
}

const colsChat = "jid,bot_id,es_grupo,nombre_guardado,nombre_publico,nombre_negocio,nombre_mostrado,telefono_e164,telefono_mostrado,pais_iso,foto_ruta,foto_id,foto_ts,ultimo_ts"

func escanearChat(s interface{ Scan(...any) error }) (*Chat, error) {
	var c Chat
	var g int
	if err := s.Scan(&c.JID, &c.BotID, &g, &c.Guardado, &c.Publico, &c.Negocio, &c.Mostrado, &c.TelE164, &c.TelMostrado, &c.PaisISO,
		&c.FotoRuta, &c.FotoID, &c.FotoTS, &c.UltimoTS); err != nil {
		return nil, err
	}
	c.EsGrupo = g == 1
	return &c, nil
}

// ObtenerChat devuelve el chat o nil si no existe.
func (a *Almacen) ObtenerChat(botID int64, jid string) (*Chat, error) {
	c, err := escanearChat(a.db.QueryRow("SELECT "+colsChat+" FROM chats WHERE bot_id=? AND jid=?", botID, jid))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (a *Almacen) escribirChat(ex execer, c *Chat) error {
	_, err := ex.Exec(`INSERT INTO chats(`+colsChat+`,busqueda,creado_ts) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(jid,bot_id) DO UPDATE SET es_grupo=excluded.es_grupo, nombre_guardado=excluded.nombre_guardado,
		nombre_publico=excluded.nombre_publico, nombre_negocio=excluded.nombre_negocio, nombre_mostrado=excluded.nombre_mostrado,
		telefono_e164=excluded.telefono_e164, telefono_mostrado=excluded.telefono_mostrado, pais_iso=excluded.pais_iso,
		foto_ruta=excluded.foto_ruta, foto_id=excluded.foto_id, foto_ts=excluded.foto_ts,
		ultimo_ts=MAX(chats.ultimo_ts, excluded.ultimo_ts), busqueda=excluded.busqueda`,
		c.JID, c.BotID, b2i(c.EsGrupo), c.Guardado, c.Publico, c.Negocio, c.Mostrado, c.TelE164, c.TelMostrado, c.PaisISO,
		c.FotoRuta, c.FotoID, c.FotoTS, c.UltimoTS, c.busqueda(), time.Now().Unix())
	return err
}

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// ActualizarChat lee el chat (o crea uno vacío), aplica cambios y lo guarda con los nombres recalculados.
// Devuelve true si el nombre que se muestra cambió.
func (a *Almacen) ActualizarChat(botID int64, jid string, esGrupo bool, cambia func(c *Chat)) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	c, err := a.ObtenerChat(botID, jid)
	if err != nil {
		return false, err
	}
	if c == nil {
		c = &Chat{JID: jid, BotID: botID, EsGrupo: esGrupo}
	}
	antes := c.Mostrado + "|" + c.TelMostrado + "|" + c.PaisISO
	cambia(c)
	c.Recalcular()
	if err := a.escribirChat(a.db, c); err != nil {
		return false, err
	}
	a.avisa("chats")
	return antes != c.Mostrado+"|"+c.TelMostrado+"|"+c.PaisISO, nil
}

// FusionarChat mueve los mensajes de `desde` a `hasta` (cuando se descubre que un LID y un teléfono son la misma persona).
func (a *Almacen) FusionarChat(botID int64, desde, hasta string) error {
	if desde == hasta {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	origen, err := a.ObtenerChat(botID, desde)
	if err != nil || origen == nil {
		return err
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	dest, err := escanearChat(tx.QueryRow("SELECT "+colsChat+" FROM chats WHERE bot_id=? AND jid=?", botID, hasta))
	if errors.Is(err, sql.ErrNoRows) {
		dest = &Chat{JID: hasta, BotID: botID, EsGrupo: origen.EsGrupo}
	} else if err != nil {
		return err
	}
	if dest.Guardado == "" {
		dest.Guardado = origen.Guardado
	}
	if dest.Publico == "" {
		dest.Publico = origen.Publico
	}
	if dest.Negocio == "" {
		dest.Negocio = origen.Negocio
	}
	if dest.FotoRuta == "" {
		dest.FotoRuta, dest.FotoID, dest.FotoTS = origen.FotoRuta, origen.FotoID, origen.FotoTS
	}
	if origen.UltimoTS > dest.UltimoTS {
		dest.UltimoTS = origen.UltimoTS
	}
	dest.Recalcular()
	if err := a.escribirChat(tx, dest); err != nil {
		return err
	}
	// Los mensajes con el mismo id que ya existan en el destino se descartan; el resto se mueve.
	if _, err := tx.Exec(`DELETE FROM messages WHERE bot_id=? AND chat_jid=? AND id IN (SELECT id FROM messages WHERE bot_id=? AND chat_jid=?)`,
		botID, desde, botID, hasta); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE messages SET chat_jid=? WHERE bot_id=? AND chat_jid=?`, hasta, botID, desde); err != nil {
		return err
	}
	if err := fusionaStats(tx, botID, desde, hasta); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM chats WHERE bot_id=? AND jid=?`, botID, desde); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	a.avisa("chats")
	return nil
}

// ---------- mensajes ----------

// Mensaje es un mensaje guardado, con sus textos ya descifrados.
type Mensaje struct {
	ID              string
	ChatJID         string
	BotID           int64
	RemitenteJID    string
	RemitenteNombre string
	EsMio           bool
	Tipo            string
	Texto           string
	TS              int64
	Borrado         bool
	Editado         bool
	ArchivoRuta     string
	ArchivoBytes    int64
	ArchivoMime     string
	ArchivoEstado   string
	MiniaturaRuta   string
	ArchivoNombre   string
	ArchivoInfo     []byte // datos para descargar de nuevo (JSON); nunca sale de la base en claro
	Ancho, Alto     int
	DuracionS       int
	NotaVoz         bool
	EliminadoEn     int64  // hora en que llegó la eliminación (0 = no se sabe)
	EliminadoPor    string // JID de quien lo eliminó si no fue su autor (un admin); vacío si no se sabe o fue el autor
}

const colsInsertMensaje = "id,chat_jid,bot_id,remitente_jid,remitente_nombre,es_mio,tipo,texto_cifrado,ts,borrado,editado,archivo_ruta,archivo_bytes,archivo_mime,archivo_estado,miniatura_ruta,archivo_nombre_cifrado,archivo_info_cifrado,ancho,alto,duracion_s,nota_voz"
const colsMensaje = colsInsertMensaje + ",eliminado_en,eliminado_por"

func (a *Almacen) escanearMensaje(s interface{ Scan(...any) error }) (*Mensaje, error) {
	var m Mensaje
	var mio, borrado, editado, nota int
	var texto, nombre, info []byte
	if err := s.Scan(&m.ID, &m.ChatJID, &m.BotID, &m.RemitenteJID, &m.RemitenteNombre, &mio, &m.Tipo, &texto, &m.TS, &borrado, &editado,
		&m.ArchivoRuta, &m.ArchivoBytes, &m.ArchivoMime, &m.ArchivoEstado, &m.MiniaturaRuta, &nombre, &info, &m.Ancho, &m.Alto, &m.DuracionS, &nota, &m.EliminadoEn, &m.EliminadoPor); err != nil {
		return nil, err
	}
	m.EsMio, m.Borrado, m.Editado, m.NotaVoz = mio == 1, borrado == 1, editado == 1, nota == 1
	if t, err := cifrado.AbrirTexto(a.clave, "texto", texto); err == nil {
		m.Texto = string(t)
	}
	if t, err := cifrado.AbrirTexto(a.clave, "archivo_nombre", nombre); err == nil {
		m.ArchivoNombre = string(t)
	}
	if t, err := cifrado.AbrirTexto(a.clave, "archivo_info", info); err == nil {
		m.ArchivoInfo = t
	}
	return &m, nil
}

func (a *Almacen) valoresMensaje(m *Mensaje) ([]any, error) {
	texto, err := cifrado.SellarTexto(a.clave, "texto", []byte(m.Texto))
	if err != nil {
		return nil, err
	}
	nombre, err := cifrado.SellarTexto(a.clave, "archivo_nombre", []byte(m.ArchivoNombre))
	if err != nil {
		return nil, err
	}
	info, err := cifrado.SellarTexto(a.clave, "archivo_info", m.ArchivoInfo)
	if err != nil {
		return nil, err
	}
	estado := m.ArchivoEstado
	if estado == "" {
		estado = EstadoNinguno
	}
	return []any{m.ID, m.ChatJID, m.BotID, m.RemitenteJID, m.RemitenteNombre, b2i(m.EsMio), m.Tipo, texto, m.TS, b2i(m.Borrado), b2i(m.Editado),
		m.ArchivoRuta, m.ArchivoBytes, m.ArchivoMime, estado, m.MiniaturaRuta, nombre, info, m.Ancho, m.Alto, m.DuracionS, b2i(m.NotaVoz)}, nil
}

const insertMensaje = "INSERT INTO messages(" + colsInsertMensaje + ") VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id,chat_jid,bot_id) DO NOTHING"

// GuardarMensajes guarda varios mensajes en una sola transacción. Los chats deben existir.
// Devuelve cuáles eran nuevos (los repetidos se ignoran).
func (a *Almacen) GuardarMensajes(msgs []*Mensaje) (nuevos []*Mensaje, err error) {
	if len(msgs) == 0 {
		return nil, nil
	}
	tx, err := a.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	st, err := tx.Prepare(insertMensaje)
	if err != nil {
		return nil, err
	}
	defer st.Close()
	est, err := tx.Prepare(upsertStats)
	if err != nil {
		return nil, err
	}
	defer est.Close()
	tocados := map[string]int64{}
	for _, m := range msgs {
		v, err := a.valoresMensaje(m)
		if err != nil {
			return nil, err
		}
		r, err := st.Exec(v...)
		if err != nil {
			return nil, err
		}
		if n, _ := r.RowsAffected(); n > 0 {
			if err := a.sumaStats(est, m); err != nil {
				return nil, err
			}
			nuevos = append(nuevos, m)
			k := fmt.Sprintf("%d\x00%s", m.BotID, m.ChatJID)
			if m.TS > tocados[k] {
				tocados[k] = m.TS
			}
		}
	}
	for k, ts := range tocados {
		var bot int64
		var jid string
		parts := strings.SplitN(k, "\x00", 2)
		fmt.Sscan(parts[0], &bot)
		jid = parts[1]
		if _, err := tx.Exec("UPDATE chats SET ultimo_ts=MAX(ultimo_ts,?) WHERE bot_id=? AND jid=?", ts, bot, jid); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if len(nuevos) > 0 {
		a.avisa("chats")
		seen := map[string]bool{}
		for _, m := range nuevos {
			if !seen[m.ChatJID] {
				seen[m.ChatJID] = true
				a.avisa("chat:" + m.ChatJID)
			}
		}
	}
	return nuevos, nil
}

// GuardarMensaje guarda uno solo; devuelve true si era nuevo.
func (a *Almacen) GuardarMensaje(m *Mensaje) (bool, error) {
	n, err := a.GuardarMensajes([]*Mensaje{m})
	return len(n) > 0, err
}

// ObtenerMensaje devuelve el mensaje o nil.
func (a *Almacen) ObtenerMensaje(botID int64, chat, id string) (*Mensaje, error) {
	m, err := a.escanearMensaje(a.db.QueryRow("SELECT "+colsMensaje+" FROM messages WHERE bot_id=? AND chat_jid=? AND id=?", botID, chat, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return m, err
}

// EditarTexto cambia el texto de un mensaje y lo marca como editado. Devuelve false si no existe.
func (a *Almacen) EditarTexto(botID int64, chat, id, texto string) (bool, error) {
	sellado, err := cifrado.SellarTexto(a.clave, "texto", []byte(texto))
	if err != nil {
		return false, err
	}
	r, err := a.db.Exec("UPDATE messages SET texto_cifrado=?, editado=1 WHERE bot_id=? AND chat_jid=? AND id=?", sellado, botID, chat, id)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	if n > 0 {
		a.avisa("chat:" + chat)
		a.avisa("chats")
	}
	return n > 0, nil
}

// ActualizarArchivo fija el estado, la ruta y el tamaño del archivo de un mensaje.
func (a *Almacen) ActualizarArchivo(botID int64, chat, id, estado, ruta string, bytes int64) error {
	_, err := a.db.Exec(`UPDATE messages SET archivo_estado=?, archivo_ruta=?, archivo_bytes=CASE WHEN ?>0 THEN ? ELSE archivo_bytes END
		WHERE bot_id=? AND chat_jid=? AND id=?`, estado, ruta, bytes, bytes, botID, chat, id)
	if err == nil {
		a.avisa(fmt.Sprintf("archivo:%s:%s", chat, id))
		a.avisa("espacio")
	}
	return err
}

// FijarEstado cambia solo el estado del archivo.
func (a *Almacen) FijarEstado(botID int64, chat, id, estado string) error {
	_, err := a.db.Exec("UPDATE messages SET archivo_estado=? WHERE bot_id=? AND chat_jid=? AND id=?", estado, botID, chat, id)
	if err == nil {
		a.avisa(fmt.Sprintf("archivo:%s:%s", chat, id))
	}
	return err
}

// FijarMiniatura guarda la ruta de la miniatura.
func (a *Almacen) FijarMiniatura(botID int64, chat, id, ruta string) error {
	_, err := a.db.Exec("UPDATE messages SET miniatura_ruta=? WHERE bot_id=? AND chat_jid=? AND id=?", ruta, botID, chat, id)
	return err
}

func abrirTextoCol(clave []byte, col string, b []byte) (string, error) {
	t, err := cifrado.AbrirTexto(clave, col, b)
	return string(t), err
}
