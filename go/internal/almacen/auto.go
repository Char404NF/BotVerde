package almacen

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"botchat/internal/cifrado"
)

// Mensajes automáticos (bloque D). El almacén solo guarda y consulta: qué significa cada configuración lo decide el paquete `auto`.

// Tipos de mensaje automático.
const (
	AutoBienvenida = "bienvenida"
	AutoAusencia   = "ausencia"
	AutoRecibido   = "recibido"
	AutoCierre     = "cierre"
	AutoAtajo      = "atajo"
)

// EsTipoAuto dice si es un tipo de mensaje automático conocido.
func EsTipoAuto(t string) bool {
	switch t {
	case AutoBienvenida, AutoAusencia, AutoRecibido, AutoCierre, AutoAtajo:
		return true
	}
	return false
}

// AdjuntoRef es un archivo guardado (cifrado) que se manda con un mensaje. El archivo vive en menu_adj/<bot>/<id>.bcf.
type AdjuntoRef struct {
	ID     string `json:"id"`
	Tipo   string `json:"tipo"` // imagen, video, audio (nota de voz) o documento
	Mime   string `json:"mime,omitempty"`
	Nombre string `json:"nombre,omitempty"`
	Bytes  int64  `json:"bytes,omitempty"`
}

// AutoMensaje es un mensaje automático guardado. Config es un JSON que interpreta el paquete `auto`.
type AutoMensaje struct {
	ID       int64        `json:"id"`
	BotID    int64        `json:"-"`
	Tipo     string       `json:"tipo"`
	Nombre   string       `json:"nombre"` // nombre del mensaje; en un atajo es lo que se escribe tras la "/"
	Texto    string       `json:"texto"`
	Config   string       `json:"config"`
	Adjuntos []AdjuntoRef `json:"adjuntos"`
	Activo   bool         `json:"activo"`
	Orden    int          `json:"orden"`
	Creado   int64        `json:"creado"`
	Editado  int64        `json:"editado"`
}

func (a *Almacen) sellaAuto(m *AutoMensaje) (texto, cfg, adj []byte, err error) {
	if texto, err = cifrado.SellarTexto(a.clave, "auto_texto", []byte(m.Texto)); err != nil {
		return
	}
	if cfg, err = cifrado.SellarTexto(a.clave, "auto_config", []byte(m.Config)); err != nil {
		return
	}
	var aj []byte
	if len(m.Adjuntos) > 0 {
		aj, _ = json.Marshal(m.Adjuntos)
	}
	adj, err = cifrado.SellarTexto(a.clave, "auto_adjuntos", aj)
	return
}

const colsAuto = "id,bot_id,tipo,nombre,texto_cifrado,config_cifrado,adjuntos_cifrado,activo,orden,creado_ts,editado_ts"

func (a *Almacen) escanearAuto(s interface{ Scan(...any) error }) (*AutoMensaje, error) {
	var m AutoMensaje
	var t, c, aj []byte
	var act int
	if err := s.Scan(&m.ID, &m.BotID, &m.Tipo, &m.Nombre, &t, &c, &aj, &act, &m.Orden, &m.Creado, &m.Editado); err != nil {
		return nil, err
	}
	m.Activo = act == 1
	if x, err := cifrado.AbrirTexto(a.clave, "auto_texto", t); err == nil {
		m.Texto = string(x)
	}
	if x, err := cifrado.AbrirTexto(a.clave, "auto_config", c); err == nil {
		m.Config = string(x)
	}
	m.Adjuntos = []AdjuntoRef{}
	if x, err := cifrado.AbrirTexto(a.clave, "auto_adjuntos", aj); err == nil && len(x) > 0 {
		_ = json.Unmarshal(x, &m.Adjuntos)
	}
	return &m, nil
}

// CrearAuto guarda un mensaje automático nuevo al final de la lista de su tipo. Devuelve su id.
func (a *Almacen) CrearAuto(botID int64, m *AutoMensaje) (int64, error) {
	if !EsTipoAuto(m.Tipo) {
		return 0, errors.New("tipo de mensaje automático no válido")
	}
	t, c, aj, err := a.sellaAuto(m)
	if err != nil {
		return 0, err
	}
	var orden int
	_ = a.db.QueryRow(`SELECT COALESCE(MAX(orden),0)+1 FROM auto_mensajes WHERE bot_id=? AND tipo=?`, botID, m.Tipo).Scan(&orden)
	ahora := time.Now().Unix()
	r, err := a.db.Exec(`INSERT INTO auto_mensajes(bot_id,tipo,nombre,texto_cifrado,config_cifrado,adjuntos_cifrado,activo,orden,creado_ts,editado_ts) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		botID, m.Tipo, m.Nombre, t, c, aj, b2i(m.Activo), orden, ahora, ahora)
	if err != nil {
		return 0, err
	}
	id, _ := r.LastInsertId()
	a.avisa("auto")
	return id, nil
}

// GuardarAuto reemplaza el contenido de un mensaje automático. Los archivos que ya no usa se borran del teléfono.
func (a *Almacen) GuardarAuto(botID int64, m *AutoMensaje) error {
	viejo, err := a.ObtenerAuto(botID, m.ID)
	if err != nil {
		return err
	}
	if viejo == nil {
		return errors.New("Ese mensaje ya no existe.")
	}
	t, c, aj, err := a.sellaAuto(m)
	if err != nil {
		return err
	}
	if _, err := a.db.Exec(`UPDATE auto_mensajes SET nombre=?, texto_cifrado=?, config_cifrado=?, adjuntos_cifrado=?, activo=?, editado_ts=? WHERE bot_id=? AND id=?`,
		m.Nombre, t, c, aj, b2i(m.Activo), time.Now().Unix(), botID, m.ID); err != nil {
		return err
	}
	a.quitarAdjuntosSueltos(botID, viejo.Adjuntos, m.Adjuntos)
	a.avisa("auto")
	return nil
}

// ObtenerAuto devuelve un mensaje automático (nil si no existe).
func (a *Almacen) ObtenerAuto(botID, id int64) (*AutoMensaje, error) {
	m, err := a.escanearAuto(a.db.QueryRow(`SELECT `+colsAuto+` FROM auto_mensajes WHERE bot_id=? AND id=?`, botID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return m, err
}

// ListarAutos devuelve los mensajes de un tipo ("" = todos) en su orden.
func (a *Almacen) ListarAutos(botID int64, tipo string) ([]AutoMensaje, error) {
	q := `SELECT ` + colsAuto + ` FROM auto_mensajes WHERE bot_id=?`
	args := []any{botID}
	if tipo != "" {
		q += ` AND tipo=?`
		args = append(args, tipo)
	}
	q += ` ORDER BY tipo, orden, id`
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []AutoMensaje{}
	for filas.Next() {
		m, err := a.escanearAuto(filas)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, filas.Err()
}

// AutoPorNombre busca el atajo activo con ese nombre (sin la "/"). nil si no hay.
func (a *Almacen) AutoPorNombre(botID int64, nombre string) *AutoMensaje {
	m, err := a.escanearAuto(a.db.QueryRow(`SELECT `+colsAuto+` FROM auto_mensajes WHERE bot_id=? AND tipo=? AND activo=1 AND nombre=? COLLATE NOCASE ORDER BY orden, id LIMIT 1`, botID, AutoAtajo, nombre))
	if err != nil {
		return nil
	}
	return m
}

// ExisteAtajo dice, sin descifrar nada, si hay un atajo activo con ese nombre.
func (a *Almacen) ExisteAtajo(botID int64, nombre string) bool {
	var x int
	return a.db.QueryRow(`SELECT 1 FROM auto_mensajes WHERE bot_id=? AND tipo=? AND activo=1 AND nombre=? COLLATE NOCASE LIMIT 1`, botID, AutoAtajo, nombre).Scan(&x) == nil
}

// FijarAutoActivo enciende o apaga un mensaje automático.
func (a *Almacen) FijarAutoActivo(botID, id int64, si bool) error {
	r, err := a.db.Exec(`UPDATE auto_mensajes SET activo=?, editado_ts=? WHERE bot_id=? AND id=?`, b2i(si), time.Now().Unix(), botID, id)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return errors.New("Ese mensaje ya no existe.")
	}
	a.avisa("auto")
	return nil
}

// BorrarAuto quita un mensaje automático, lo recordado de él y sus archivos guardados.
func (a *Almacen) BorrarAuto(botID, id int64) error {
	m, err := a.ObtenerAuto(botID, id)
	if err != nil || m == nil {
		return err
	}
	for _, q := range []string{`DELETE FROM auto_repeticion WHERE bot_id=? AND auto_id=?`, `DELETE FROM auto_registro WHERE bot_id=? AND auto_id=?`} {
		if _, err := a.db.Exec(q, botID, id); err != nil {
			return err
		}
	}
	if _, err := a.db.Exec(`DELETE FROM auto_mensajes WHERE bot_id=? AND id=?`, botID, id); err != nil {
		return err
	}
	a.quitarAdjuntosSueltos(botID, m.Adjuntos, nil)
	a.avisa("auto")
	return nil
}

// ReordenarAutos fija el orden de los mensajes de un tipo según la lista de ids.
func (a *Almacen) ReordenarAutos(botID int64, ids []int64) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE auto_mensajes SET orden=? WHERE bot_id=? AND id=?`, i+1, botID, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	a.avisa("auto")
	return nil
}

// ---------- repetición por persona ----------

// UltimaRepeticion da cuándo se le mandó por última vez ese mensaje a esa persona (0 = nunca).
func (a *Almacen) UltimaRepeticion(botID, autoID int64, persona string) int64 {
	var ts int64
	_ = a.db.QueryRow(`SELECT ultimo_ts FROM auto_repeticion WHERE bot_id=? AND auto_id=? AND persona=?`, botID, autoID, persona).Scan(&ts)
	return ts
}

// MarcarRepeticion anota que se le mandó ese mensaje a esa persona a la hora ts.
func (a *Almacen) MarcarRepeticion(botID, autoID int64, persona string, ts time.Time) {
	_, _ = a.db.Exec(`INSERT INTO auto_repeticion(bot_id,auto_id,persona,ultimo_ts) VALUES(?,?,?,?) ON CONFLICT(bot_id,auto_id,persona) DO UPDATE SET ultimo_ts=excluded.ultimo_ts`, botID, autoID, persona, ts.Unix())
}

// LimpiarRepeticion borra lo recordado con más de `dias` días (0 = no borra).
func (a *Almacen) LimpiarRepeticion(botID int64, dias int, ahora time.Time) {
	if dias <= 0 {
		return
	}
	_, _ = a.db.Exec(`DELETE FROM auto_repeticion WHERE bot_id=? AND ultimo_ts<?`, botID, ahora.AddDate(0, 0, -dias).Unix())
}

// ---------- registro de envíos automáticos (sin textos) ----------

// Resultados de un envío automático.
const (
	AutoOK        = "ok"         // salió el texto y todos los archivos
	AutoParcial   = "parcial"    // salió, pero algún archivo falló y se mandó solo el texto o lo demás
	AutoSoloTexto = "solo_texto" // se pasó el tope por hora: salió solo el texto, sin archivos
	AutoTopeDuro  = "tope"       // se pasó muchísimo del tope: no se mandó
)

// AutoEnvio es un renglón del registro de envíos automáticos. No lleva textos ni nombres.
type AutoEnvio struct {
	ID        int64  `json:"id"`
	TS        int64  `json:"ts"`
	AutoID    int64  `json:"autoId"`
	Tipo      string `json:"tipo"`
	Chat      string `json:"chat"` // identificador corto, nunca el número
	Resultado string `json:"resultado"`
	AdjOK     int    `json:"adjOk"`
	AdjFallo  int    `json:"adjFallo"`
	Motivo    string `json:"motivo"`
}

// RegistrarAutoEnvio anota un envío automático.
func (a *Almacen) RegistrarAutoEnvio(botID int64, e AutoEnvio) {
	h := ""
	if e.Chat != "" {
		h = HashChat(botID, e.Chat)[:6]
	}
	if e.TS == 0 {
		e.TS = time.Now().Unix()
	}
	_, _ = a.db.Exec(`INSERT INTO auto_registro(bot_id,ts,auto_id,tipo,chat,resultado,adj_ok,adj_fallo,motivo) VALUES(?,?,?,?,?,?,?,?,?)`,
		botID, e.TS, e.AutoID, e.Tipo, h, e.Resultado, e.AdjOK, e.AdjFallo, e.Motivo)
	a.avisa("auto_registro")
}

// ContarAutoEnvios cuenta los envíos automáticos desde `desde` (para el tope por hora). Los atajos no cuentan (los pido yo).
func (a *Almacen) ContarAutoEnvios(botID int64, desde time.Time) int {
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM auto_registro WHERE bot_id=? AND ts>=? AND tipo<>? AND resultado<>?`, botID, desde.Unix(), AutoAtajo, AutoTopeDuro).Scan(&n)
	return n
}

// ListarAutoRegistro devuelve los últimos renglones del registro (autoID 0 = de todos), el más nuevo primero.
func (a *Almacen) ListarAutoRegistro(botID, autoID int64, limite int) ([]AutoEnvio, error) {
	if limite <= 0 || limite > 500 {
		limite = 100
	}
	q := `SELECT id,ts,auto_id,tipo,chat,resultado,adj_ok,adj_fallo,motivo FROM auto_registro WHERE bot_id=?`
	args := []any{botID}
	if autoID > 0 {
		q += ` AND auto_id=?`
		args = append(args, autoID)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limite)
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []AutoEnvio{}
	for filas.Next() {
		var x AutoEnvio
		if err := filas.Scan(&x.ID, &x.TS, &x.AutoID, &x.Tipo, &x.Chat, &x.Resultado, &x.AdjOK, &x.AdjFallo, &x.Motivo); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, filas.Err()
}

// ContarAutoRegistro cuenta los envíos de un mensaje (0 = todos) desde `desde`.
func (a *Almacen) ContarAutoRegistro(botID, autoID int64, desde time.Time) int {
	var n int
	q := `SELECT COUNT(*) FROM auto_registro WHERE bot_id=? AND ts>=?`
	args := []any{botID, desde.Unix()}
	if autoID > 0 {
		q += ` AND auto_id=?`
		args = append(args, autoID)
	}
	_ = a.db.QueryRow(q, args...).Scan(&n)
	return n
}

// LimpiarAutoRegistro borra el registro con más de `dias` días.
func (a *Almacen) LimpiarAutoRegistro(botID int64, dias int, ahora time.Time) {
	if dias <= 0 {
		return
	}
	_, _ = a.db.Exec(`DELETE FROM auto_registro WHERE bot_id=? AND ts<?`, botID, ahora.AddDate(0, 0, -dias).Unix())
}

// ---------- etiqueta "automático" ----------

type claveEtiqueta struct{}

// ConEtiqueta marca que lo que se envíe con este contexto lo manda una automatización de ese tipo ("menu", "bienvenida"…).
func ConEtiqueta(ctx context.Context, etiqueta string) context.Context {
	return context.WithValue(ctx, claveEtiqueta{}, etiqueta)
}

// EtiquetaDe devuelve la etiqueta de automatización del contexto ("" = no hay).
func EtiquetaDe(ctx context.Context) string {
	s, _ := ctx.Value(claveEtiqueta{}).(string)
	return s
}

// EtiquetaPorDefecto es la que lleva un mensaje no escrito a mano cuando nadie dijo de qué automatización viene.
const EtiquetaPorDefecto = "bot"

// ListarMensajesAuto devuelve hasta `limite` mensajes automáticos de un chat (chat "" = de todos), del más nuevo al más viejo.
// Con antesTS > 0 pagina hacia atrás.
func (a *Almacen) ListarMensajesAuto(botID int64, chat string, antesTS int64, limite int) ([]MensajeVista, error) {
	if limite <= 0 || limite > 500 {
		limite = 50
	}
	q := "SELECT " + colsMensaje + " FROM messages WHERE bot_id=? AND automatico<>''"
	args := []any{botID}
	if chat != "" {
		q += " AND chat_jid=?"
		args = append(args, chat)
	}
	if antesTS > 0 {
		q += " AND ts<?"
		args = append(args, antesTS)
	}
	q += " ORDER BY ts DESC, id DESC LIMIT ?"
	args = append(args, limite)
	return a.vistasDe(q, args)
}

// ContarMensajesAuto cuenta los mensajes automáticos de un chat (chat "" = de todos) desde `desde`.
func (a *Almacen) ContarMensajesAuto(botID int64, chat string, desde time.Time) int {
	var n int
	q := `SELECT COUNT(*) FROM messages WHERE bot_id=? AND automatico<>'' AND ts>=?`
	args := []any{botID, desde.Unix()}
	if chat != "" {
		q += ` AND chat_jid=?`
		args = append(args, chat)
	}
	_ = a.db.QueryRow(q, args...).Scan(&n)
	return n
}

// ChatsConAutomaticos devuelve los chats que tienen mensajes automáticos desde `desde` (para el filtro de la lista de chats).
func (a *Almacen) ChatsConAutomaticos(botID int64, desde time.Time) []string {
	filas, err := a.db.Query(`SELECT DISTINCT chat_jid FROM messages WHERE bot_id=? AND automatico<>'' AND ts>=?`, botID, desde.Unix())
	if err != nil {
		return nil
	}
	defer filas.Close()
	var out []string
	for filas.Next() {
		var j string
		if filas.Scan(&j) == nil {
			out = append(out, j)
		}
	}
	return out
}

// OlvidarAutoPersona borra lo que las automatizaciones recuerdan de una persona (cuándo se le mandó cada mensaje).
func (a *Almacen) OlvidarAutoPersona(botID int64, formas []string) {
	for _, f := range formas {
		_, _ = a.db.Exec(`DELETE FROM auto_repeticion WHERE bot_id=? AND persona=?`, botID, f)
		_, _ = a.db.Exec(`DELETE FROM auto_registro WHERE bot_id=? AND chat=?`, botID, HashChat(botID, f)[:6])
	}
}

// DescribeAutoEnvio es un texto corto del resultado, para la pantalla.
func DescribeAutoEnvio(e AutoEnvio) string {
	switch e.Resultado {
	case AutoOK:
		return "Enviado"
	case AutoParcial:
		return fmt.Sprintf("Enviado; %d archivo(s) no salieron", e.AdjFallo)
	case AutoSoloTexto:
		return "Solo texto (tope por hora)"
	case AutoTopeDuro:
		return "No se mandó (tope por hora)"
	}
	return strings.TrimSpace(e.Resultado)
}

// MarcarAutomatico etiqueta un mensaje mío ya guardado como mandado por una automatización (por ejemplo, un "/atajo" que escribí en el
// teléfono y que el bot cambió por su texto).
func (a *Almacen) MarcarAutomatico(botID int64, chat, id, etiqueta string) {
	if _, err := a.db.Exec(`UPDATE messages SET automatico=? WHERE bot_id=? AND chat_jid=? AND id=? AND es_mio=1`, etiqueta, botID, chat, id); err == nil {
		a.avisaMsg(chat, id)
	}
}
