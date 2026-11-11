package almacen

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"botchat/internal/cifrado"
)

// Menús ampliados (v0.5.0, bloque C): pila de "atrás", fases de un formulario, mensajes del bot en grupos, fila de espera de un
// grupo lleno, avisos de "conversación vencida", lo que el menú no entendió y los formularios completos.

// ---------- estado fino de una conversación ----------

// FijarFase anota en qué fase de un formulario va una conversación (campo e intentos fallidos incluidos).
func (a *Almacen) FijarFase(sesionID int64, fase string, campo, reint int) error {
	_, err := a.db.Exec(`UPDATE flow_sessions SET fase=?, campo=?, reint=? WHERE id=?`, fase, campo, reint, sesionID)
	return err
}

// FijarPila guarda los pasos de menú por los que pasó una conversación (para "atrás").
func (a *Almacen) FijarPila(sesionID int64, pila string) error {
	_, err := a.db.Exec(`UPDATE flow_sessions SET pila=? WHERE id=?`, pila, sesionID)
	return err
}

// FijarCierre anota por qué terminó una conversación (sin texto libre).
func (a *Almacen) FijarCierre(sesionID int64, motivo string) {
	_, _ = a.db.Exec(`UPDATE flow_sessions SET cierre=? WHERE id=?`, motivo, sesionID)
}

// BorrarVariable quita un dato guardado de una conversación (por ejemplo al corregir o cancelar un formulario).
func (a *Almacen) BorrarVariable(sesionID int64, nombre string) {
	_, _ = a.db.Exec(`DELETE FROM flow_vars WHERE session_id=? AND nombre=?`, sesionID, nombre)
}

// BorrarVariables quita todos los datos guardados de una conversación.
func (a *Almacen) BorrarVariables(sesionID int64) {
	_, _ = a.db.Exec(`DELETE FROM flow_vars WHERE session_id=?`, sesionID)
}

// ContarSesionesGrupo cuenta las conversaciones activas (no vencidas) de un grupo. `defecto` es el tiempo de caducidad de las que
// no traen el suyo.
func (a *Almacen) ContarSesionesGrupo(botID int64, chat string, ahora time.Time, defecto time.Duration) int {
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM flow_sessions WHERE bot_id=? AND chat=? AND prueba=0 AND estado=? AND ultima_ts + CASE WHEN expira_seg>0 THEN expira_seg ELSE ? END > ?`,
		botID, chat, SesActiva, int64(defecto.Seconds()), ahora.Unix()).Scan(&n)
	return n
}

// ExpirarSesiones cierra en silencio (sin mandar nada) las conversaciones activas que ya pasaron su tiempo sin actividad. Devuelve
// cuántas cerró. Así un grupo lleno recupera sus lugares sin esperar a que la persona vuelva a escribir.
func (a *Almacen) ExpirarSesiones(botID int64, ahora time.Time, defecto time.Duration) int {
	filas, err := a.db.Query(`SELECT `+colsSesion+` FROM flow_sessions WHERE bot_id=? AND prueba=0 AND estado=? AND ultima_ts + CASE WHEN expira_seg>0 THEN expira_seg ELSE ? END <= ? LIMIT 500`,
		botID, SesActiva, int64(defecto.Seconds()), ahora.Unix())
	if err != nil {
		return 0
	}
	var ss []*Sesion
	for filas.Next() {
		if x, err := escanearSesion(filas); err == nil {
			ss = append(ss, x)
		}
	}
	filas.Close()
	n := 0
	for _, x := range ss {
		tx, err := a.db.Begin()
		if err != nil {
			continue
		}
		if _, err := tx.Exec(`UPDATE flow_sessions SET estado=?, cierre='silencio' WHERE id=? AND estado=?`, SesExpirada, x.ID, SesActiva); err != nil {
			tx.Rollback()
			continue
		}
		_ = a.sumaStat(tx, botID, x.MenuID, "", "expirado", false, ahora, 1)
		if x.Paso != "" {
			_ = a.sumaStat(tx, botID, x.MenuID, x.Paso, "abandono", false, ahora, 1)
		}
		if tx.Commit() == nil {
			n++
		}
	}
	if n > 0 {
		a.avisa("sesiones")
	}
	return n
}

// UltimaExpirada devuelve la última conversación de esa persona en ese menú que caducó por inactividad desde `desde` (nil si no hay).
func (a *Almacen) UltimaExpirada(botID int64, chat, sender string, menuID int64, desde time.Time) *Sesion {
	x, err := escanearSesion(a.db.QueryRow(`SELECT `+colsSesion+` FROM flow_sessions WHERE bot_id=? AND chat=? AND sender=? AND menu_id=? AND prueba=0 AND estado=? AND ultima_ts>=?
		AND NOT EXISTS (SELECT 1 FROM flow_sessions o WHERE o.bot_id=flow_sessions.bot_id AND o.chat=flow_sessions.chat AND o.sender=flow_sessions.sender AND o.prueba=0 AND o.id>flow_sessions.id)
		ORDER BY id DESC LIMIT 1`, botID, chat, sender, menuID, SesExpirada, desde.Unix()))
	if err != nil {
		return nil
	}
	return x
}

// ReabrirSesion vuelve a dejar activa una conversación que caducó (retomar donde iba).
func (a *Almacen) ReabrirSesion(s *Sesion, ahora time.Time) error {
	if _, err := a.db.Exec(`UPDATE flow_sessions SET estado=?, ultima_ts=?, cierre='', reint=0, invalidas=0, cortado=0 WHERE id=?`, SesActiva, ahora.Unix(), s.ID); err != nil {
		return err
	}
	s.Estado, s.UltimaTS, s.Invalidas, s.Cortado, s.Reint = SesActiva, ahora.Unix(), 0, false, 0
	a.avisa("sesiones")
	return nil
}

// ---------- mensajes del bot en grupos ----------

// MsgBot liga un mensaje que mandó el bot en un grupo con su conversación y su dueña.
type MsgBot struct {
	Chat     string `json:"chat"`
	MsgID    string `json:"msgId"`
	SesionID int64  `json:"sesionId"`
	Persona  string `json:"persona"`
	MenuID   int64  `json:"menuId"`
	TS       int64  `json:"ts"`
}

// GuardarMsgBot anota que el mensaje `msgID` del bot pertenece a una conversación de menú.
func (a *Almacen) GuardarMsgBot(botID int64, chat, msgID string, sesionID int64, persona string, menuID int64, ts time.Time) error {
	if msgID == "" {
		return nil
	}
	_, err := a.db.Exec(`INSERT INTO msg_bot_sesion(bot_id, chat, msg_id, sesion_id, persona, menu_id, ts) VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(bot_id, chat, msg_id) DO UPDATE SET sesion_id=excluded.sesion_id, persona=excluded.persona, menu_id=excluded.menu_id, ts=excluded.ts`,
		botID, chat, msgID, sesionID, persona, menuID, ts.Unix())
	return err
}

// MsgBotPorID busca a qué conversación pertenece un mensaje del bot (nil si no es de un menú).
func (a *Almacen) MsgBotPorID(botID int64, chat, msgID string) *MsgBot {
	if msgID == "" {
		return nil
	}
	var x MsgBot
	err := a.db.QueryRow(`SELECT chat, msg_id, sesion_id, persona, menu_id, ts FROM msg_bot_sesion WHERE bot_id=? AND chat=? AND msg_id=?`, botID, chat, msgID).
		Scan(&x.Chat, &x.MsgID, &x.SesionID, &x.Persona, &x.MenuID, &x.TS)
	if err != nil {
		return nil
	}
	return &x
}

// LimpiarMsgBot borra las ligas de más de `dias` días.
func (a *Almacen) LimpiarMsgBot(botID int64, dias int, ahora time.Time) {
	_, _ = a.db.Exec(`DELETE FROM msg_bot_sesion WHERE bot_id=? AND ts<?`, botID, ahora.AddDate(0, 0, -dias).Unix())
}

// ---------- conversación vencida ----------

// MarcarVencido anota que ya se le dijo a esa persona "esa conversación terminó" en ese menú. Devuelve true la primera vez (hay que
// avisarle) y false si ya se le había dicho.
func (a *Almacen) MarcarVencido(botID int64, chat, persona string, menuID int64, ahora time.Time) bool {
	r, err := a.db.Exec(`INSERT OR IGNORE INTO flow_vencido(bot_id, chat, persona, menu_id, ts) VALUES(?,?,?,?,?)`, botID, chat, persona, menuID, ahora.Unix())
	if err != nil {
		return false
	}
	n, _ := r.RowsAffected()
	return n > 0
}

// LimpiarVencido olvida el aviso (cuando esa persona empieza una conversación nueva de ese menú).
func (a *Almacen) LimpiarVencido(botID int64, chat, persona string, menuID int64) {
	_, _ = a.db.Exec(`DELETE FROM flow_vencido WHERE bot_id=? AND chat=? AND persona=? AND menu_id=?`, botID, chat, persona, menuID)
}

// ---------- fila de espera de un grupo ----------

// EnCola es alguien que espera lugar para usar un menú en un grupo lleno.
type EnCola struct {
	ID        int64  `json:"id"`
	Chat      string `json:"chat"`
	Persona   string `json:"persona"`
	MencionWA string `json:"mencionWa"`
	MsgID     string `json:"msgId"`
	MenuID    int64  `json:"menuId"`
	TS        int64  `json:"ts"`
}

// Encolar pone a la persona en la fila del grupo. Devuelve su turno (1 = la siguiente) y si es nueva en la fila.
func (a *Almacen) Encolar(botID int64, chat, persona, mencionWA, msgID string, menuID int64, ahora time.Time) (turno int, nueva bool) {
	if t := a.TurnoEnCola(botID, chat, persona); t > 0 {
		return t, false
	}
	if _, err := a.db.Exec(`INSERT INTO flow_cola(bot_id, chat, persona, mencion_wa, msg_id, menu_id, ts) VALUES(?,?,?,?,?,?,?)`,
		botID, chat, persona, mencionWA, msgID, menuID, ahora.Unix()); err != nil {
		return 0, false
	}
	return a.TurnoEnCola(botID, chat, persona), true
}

// TurnoEnCola dice en qué lugar de la fila está la persona (0 = no está).
func (a *Almacen) TurnoEnCola(botID int64, chat, persona string) int {
	var id int64
	if a.db.QueryRow(`SELECT id FROM flow_cola WHERE bot_id=? AND chat=? AND persona=?`, botID, chat, persona).Scan(&id) != nil {
		return 0
	}
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM flow_cola WHERE bot_id=? AND chat=? AND id<=?`, botID, chat, id).Scan(&n)
	return n
}

// ChatsConCola lista los grupos que tienen a alguien esperando.
func (a *Almacen) ChatsConCola(botID int64) []string {
	filas, err := a.db.Query(`SELECT DISTINCT chat FROM flow_cola WHERE bot_id=?`, botID)
	if err != nil {
		return nil
	}
	defer filas.Close()
	var out []string
	for filas.Next() {
		var c string
		if filas.Scan(&c) == nil {
			out = append(out, c)
		}
	}
	return out
}

// SiguienteEnCola devuelve a quien lleva más tiempo esperando en ese grupo (nil si nadie).
func (a *Almacen) SiguienteEnCola(botID int64, chat string) *EnCola {
	var x EnCola
	err := a.db.QueryRow(`SELECT id, chat, persona, mencion_wa, msg_id, menu_id, ts FROM flow_cola WHERE bot_id=? AND chat=? ORDER BY id LIMIT 1`, botID, chat).
		Scan(&x.ID, &x.Chat, &x.Persona, &x.MencionWA, &x.MsgID, &x.MenuID, &x.TS)
	if err != nil {
		return nil
	}
	return &x
}

// QuitarDeCola saca a alguien de la fila.
func (a *Almacen) QuitarDeCola(botID, id int64) {
	_, _ = a.db.Exec(`DELETE FROM flow_cola WHERE bot_id=? AND id=?`, botID, id)
}

// QuitarPersonaDeCola saca a una persona de la fila de un grupo.
func (a *Almacen) QuitarPersonaDeCola(botID int64, chat, persona string) {
	_, _ = a.db.Exec(`DELETE FROM flow_cola WHERE bot_id=? AND chat=? AND persona=?`, botID, chat, persona)
}

// LimpiarCola descarta, sin avisar, a quien esperó más de `vida`.
func (a *Almacen) LimpiarCola(botID int64, vida time.Duration, ahora time.Time) int {
	r, err := a.db.Exec(`DELETE FROM flow_cola WHERE bot_id=? AND ts<?`, botID, ahora.Add(-vida).Unix())
	if err != nil {
		return 0
	}
	n, _ := r.RowsAffected()
	return int(n)
}

// ---------- lo que el menú no entendió ----------

// NoEntendio es un mensaje que el menú no supo interpretar.
type NoEntendio struct {
	ID     int64  `json:"id"`
	MenuID int64  `json:"menuId"`
	Paso   string `json:"paso"`
	TS     int64  `json:"ts"`
	Texto  string `json:"texto"`
}

// GuardarNoEntendio guarda (cifrado) un texto que el menú no entendió.
func (a *Almacen) GuardarNoEntendio(botID, menuID int64, paso, texto string, ahora time.Time) error {
	if len([]rune(texto)) > 300 {
		texto = string([]rune(texto)[:300])
	}
	sel, err := cifrado.SellarTexto(a.clave, "noentendio", []byte(texto))
	if err != nil {
		return err
	}
	_, err = a.db.Exec(`INSERT INTO flow_noentendio(bot_id, menu_id, paso, ts, texto_cifrado) VALUES(?,?,?,?,?)`, botID, menuID, paso, ahora.Unix(), sel)
	if err == nil {
		a.avisa("noentendio")
	}
	return err
}

// ListarNoEntendio devuelve los mensajes que un menú no entendió (el más nuevo primero). menuID 0 = todos los menús.
func (a *Almacen) ListarNoEntendio(botID, menuID int64, limite int) ([]NoEntendio, error) {
	if limite <= 0 || limite > 500 {
		limite = 200
	}
	q := `SELECT id, menu_id, paso, ts, texto_cifrado FROM flow_noentendio WHERE bot_id=?`
	args := []any{botID}
	if menuID > 0 {
		q += ` AND menu_id=?`
		args = append(args, menuID)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limite)
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []NoEntendio{}
	for filas.Next() {
		var x NoEntendio
		var b []byte
		if err := filas.Scan(&x.ID, &x.MenuID, &x.Paso, &x.TS, &b); err != nil {
			return nil, err
		}
		if t, err := cifrado.AbrirTexto(a.clave, "noentendio", b); err == nil {
			x.Texto = string(t)
		}
		out = append(out, x)
	}
	return out, filas.Err()
}

// ContarNoEntendio cuenta lo que no entendió un menú (0 = todos).
func (a *Almacen) ContarNoEntendio(botID, menuID int64) int {
	var n int
	if menuID > 0 {
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM flow_noentendio WHERE bot_id=? AND menu_id=?`, botID, menuID).Scan(&n)
	} else {
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM flow_noentendio WHERE bot_id=?`, botID).Scan(&n)
	}
	return n
}

// BorrarNoEntendio borra la lista de un menú (0 = de todos). Devuelve cuántos borró.
func (a *Almacen) BorrarNoEntendio(botID, menuID int64) int {
	var r sql.Result
	var err error
	if menuID > 0 {
		r, err = a.db.Exec(`DELETE FROM flow_noentendio WHERE bot_id=? AND menu_id=?`, botID, menuID)
	} else {
		r, err = a.db.Exec(`DELETE FROM flow_noentendio WHERE bot_id=?`, botID)
	}
	if err != nil {
		return 0
	}
	n, _ := r.RowsAffected()
	a.avisa("noentendio")
	return int(n)
}

// LimpiarNoEntendio aplica la retención: borra lo de más de `dias` días (0 o menos = no borra nada).
func (a *Almacen) LimpiarNoEntendio(botID int64, dias int, ahora time.Time) int {
	if dias <= 0 {
		return 0
	}
	r, err := a.db.Exec(`DELETE FROM flow_noentendio WHERE bot_id=? AND ts<?`, botID, ahora.AddDate(0, 0, -dias).Unix())
	if err != nil {
		return 0
	}
	n, _ := r.RowsAffected()
	return int(n)
}

// ---------- formularios recibidos ----------

// CampoValor es una respuesta de un formulario.
type CampoValor struct {
	ID       string `json:"id"`
	Etiqueta string `json:"etiqueta"`
	Valor    string `json:"valor"`
}

// Formulario es un formulario completo, confirmado por la persona.
type Formulario struct {
	ID       int64        `json:"id"`
	MenuID   int64        `json:"menuId"`
	Paso     string       `json:"paso"`
	Titulo   string       `json:"titulo"`
	SesionID int64        `json:"sesionId"`
	Chat     string       `json:"chat"`
	Persona  string       `json:"persona"`
	TS       int64        `json:"ts"`
	Nombre   string       `json:"nombre"`
	Campos   []CampoValor `json:"campos"`
}

// GuardarFormulario guarda un formulario completo; los datos y el nombre van cifrados. Devuelve su id.
func (a *Almacen) GuardarFormulario(botID int64, f Formulario) (int64, error) {
	datos, err := json.Marshal(f.Campos)
	if err != nil {
		return 0, err
	}
	sd, err := cifrado.SellarTexto(a.clave, "formulario_datos", datos)
	if err != nil {
		return 0, err
	}
	sn, err := cifrado.SellarTexto(a.clave, "formulario_nombre", []byte(f.Nombre))
	if err != nil {
		return 0, err
	}
	if f.TS == 0 {
		f.TS = time.Now().Unix()
	}
	r, err := a.db.Exec(`INSERT INTO formularios(bot_id, menu_id, paso, titulo, sesion_id, chat, persona, ts, nombre_cifrado, datos_cifrado) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		botID, f.MenuID, f.Paso, f.Titulo, f.SesionID, f.Chat, f.Persona, f.TS, sn, sd)
	if err != nil {
		return 0, err
	}
	id, _ := r.LastInsertId()
	a.avisa("formularios")
	return id, nil
}

func (a *Almacen) escanearFormulario(s interface{ Scan(...any) error }) (*Formulario, error) {
	var f Formulario
	var nb, db []byte
	if err := s.Scan(&f.ID, &f.MenuID, &f.Paso, &f.Titulo, &f.SesionID, &f.Chat, &f.Persona, &f.TS, &nb, &db); err != nil {
		return nil, err
	}
	if t, err := cifrado.AbrirTexto(a.clave, "formulario_nombre", nb); err == nil {
		f.Nombre = string(t)
	}
	f.Campos = []CampoValor{}
	if t, err := cifrado.AbrirTexto(a.clave, "formulario_datos", db); err == nil {
		_ = json.Unmarshal(t, &f.Campos)
	}
	return &f, nil
}

const colsForm = "id, menu_id, paso, titulo, sesion_id, chat, persona, ts, nombre_cifrado, datos_cifrado"

// ListarFormularios devuelve los formularios recibidos, el más nuevo primero. menuID 0 = de todos los menús.
func (a *Almacen) ListarFormularios(botID, menuID int64, limite int) ([]Formulario, error) {
	if limite <= 0 || limite > 1000 {
		limite = 200
	}
	q := `SELECT ` + colsForm + ` FROM formularios WHERE bot_id=?`
	args := []any{botID}
	if menuID > 0 {
		q += ` AND menu_id=?`
		args = append(args, menuID)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limite)
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []Formulario{}
	for filas.Next() {
		f, err := a.escanearFormulario(filas)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, filas.Err()
}

// ObtenerFormulario devuelve un formulario o nil.
func (a *Almacen) ObtenerFormulario(botID, id int64) (*Formulario, error) {
	f, err := a.escanearFormulario(a.db.QueryRow(`SELECT `+colsForm+` FROM formularios WHERE bot_id=? AND id=?`, botID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return f, err
}

// BorrarFormulario borra un formulario recibido.
func (a *Almacen) BorrarFormulario(botID, id int64) error {
	_, err := a.db.Exec(`DELETE FROM formularios WHERE bot_id=? AND id=?`, botID, id)
	if err == nil {
		a.avisa("formularios")
	}
	return err
}

// BorrarFormularios borra todos los de un menú (0 = todos los menús). Devuelve cuántos.
func (a *Almacen) BorrarFormularios(botID, menuID int64) int {
	var r sql.Result
	var err error
	if menuID > 0 {
		r, err = a.db.Exec(`DELETE FROM formularios WHERE bot_id=? AND menu_id=?`, botID, menuID)
	} else {
		r, err = a.db.Exec(`DELETE FROM formularios WHERE bot_id=?`, botID)
	}
	if err != nil {
		return 0
	}
	n, _ := r.RowsAffected()
	a.avisa("formularios")
	return int(n)
}

// ContarFormularios cuenta los formularios recibidos (0 = todos los menús).
func (a *Almacen) ContarFormularios(botID, menuID int64) int {
	var n int
	if menuID > 0 {
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM formularios WHERE bot_id=? AND menu_id=?`, botID, menuID).Scan(&n)
	} else {
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM formularios WHERE bot_id=?`, botID).Scan(&n)
	}
	return n
}

// ---------- adjuntos de los menús ----------

// RutaAdjuntoMenu es dónde vive (cifrado) un adjunto de menú.
func RutaAdjuntoMenu(botID int64, id string) string {
	return filepath.Join("menu_adj", fmt.Sprint(botID), id+".bcf")
}

// GuardarAdjuntoMenu cifra y guarda el archivo que se va a mandar desde un menú. Devuelve su id y su tamaño. La infraestructura de
// adjuntos de mensajes automáticos (bloque D) puede reemplazar esto: el id queda en la definición del menú.
func (a *Almacen) GuardarAdjuntoMenu(botID int64, r io.Reader) (string, int64, error) {
	id := nuevoCodigoAdj()
	n, err := a.EscribirCifradoDesde(RutaAdjuntoMenu(botID, id), r)
	if err != nil {
		return "", 0, err
	}
	return id, n, nil
}

// BorrarAdjuntoMenu quita el archivo de un adjunto de menú.
func (a *Almacen) BorrarAdjuntoMenu(botID int64, id string) {
	if abs, err := a.Abs(RutaAdjuntoMenu(botID, id)); err == nil {
		_ = os.Remove(abs)
	}
}

// AbrirAdjuntoMenu abre (descifrando) un adjunto de menú.
func (a *Almacen) AbrirAdjuntoMenu(botID int64, id string) (*Descifrado, error) {
	return a.AbrirDescifrado(RutaAdjuntoMenu(botID, id))
}

func nuevoCodigoAdj() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// SellarArchivo cifra bytes con la llave de datos de este teléfono (formato BCF1).
func (a *Almacen) SellarArchivo(plano []byte) ([]byte, error) {
	return cifrado.CifrarBytes(a.clave, plano)
}

// AbrirArchivoSellado descifra bytes cifrados con la llave de datos de este teléfono.
func (a *Almacen) AbrirArchivoSellado(b []byte) ([]byte, error) {
	return cifrado.DescifrarBytes(a.clave, b)
}

// LeerAdjuntoMenu devuelve el contenido (en claro) de un adjunto de menú.
func (a *Almacen) LeerAdjuntoMenu(botID int64, id string) ([]byte, error) {
	return a.LeerCifrado(RutaAdjuntoMenu(botID, id))
}
