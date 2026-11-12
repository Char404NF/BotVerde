package almacen

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"botchat/internal/identidad"
)

// Chat (v0.5.0, bloque B): Pendiente / Resuelto y etiquetas locales. Nada de esto sale del teléfono ni se manda a WhatsApp.

// Estados de trabajo de un chat.
const (
	TrabajoNinguno   = ""
	TrabajoPendiente = "pendiente"
	TrabajoResuelto  = "resuelto"
)

// FijarEstadoTrabajo marca el chat como pendiente, resuelto o sin marca ("").
func (a *Almacen) FijarEstadoTrabajo(botID int64, chat, estado string) error {
	switch estado {
	case TrabajoNinguno, TrabajoPendiente, TrabajoResuelto:
	default:
		return errors.New("estado no válido")
	}
	a.asegurarEstado(botID, chat)
	_, err := a.db.Exec(`UPDATE chat_estado SET estado_trabajo=? WHERE bot_id=? AND chat_jid=?`, estado, botID, chat)
	if err == nil {
		a.avisa("chat:" + chat)
		a.avisa("chats")
	}
	return err
}

// ContarPendientes cuenta los chats marcados como pendientes.
func (a *Almacen) ContarPendientes(botID int64) int {
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM chat_estado WHERE bot_id=? AND estado_trabajo=?`, botID, TrabajoPendiente).Scan(&n)
	return n
}

// Etiqueta es una etiqueta local para ordenar los chats.
type Etiqueta struct {
	ID       int64  `json:"id"`
	Nombre   string `json:"nombre"`
	Color    string `json:"color"`
	Posicion int    `json:"posicion"`
	Chats    int    `json:"chats"`    // cuántos chats la tienen (solo al listar)
	Emoji    string `json:"emoji"`    // opcional (v0.6.0)
	Personas int    `json:"personas"` // cuántas personas la tienen (solo al listar)
	Mensajes int    `json:"mensajes"` // cuántos mensajes marcados con ella (solo al listar)
}

func emojiEtiqueta(e string) (string, error) {
	e = strings.TrimSpace(e)
	if len([]rune(e)) > 16 {
		return "", errors.New("el emoji es muy largo")
	}
	return e, nil
}

func colorEtiqueta(c string) (string, error) {
	c = strings.TrimSpace(c)
	if c == "" {
		return "#2EC4CB", nil
	}
	if len(c) != 7 || c[0] != '#' {
		return "", errors.New("color no válido")
	}
	for _, x := range c[1:] {
		if !(x >= '0' && x <= '9' || x >= 'a' && x <= 'f' || x >= 'A' && x <= 'F') {
			return "", errors.New("color no válido")
		}
	}
	return c, nil
}

func nombreEtiqueta(n string) (string, error) {
	n = strings.TrimSpace(n)
	if n == "" {
		return "", errors.New("falta el nombre de la etiqueta")
	}
	if len([]rune(n)) > 24 {
		return "", errors.New("el nombre es muy largo (máximo 24 letras)")
	}
	return n, nil
}

// ListarEtiquetas devuelve las etiquetas en su orden, con cuántos chats tiene cada una.
func (a *Almacen) ListarEtiquetas(botID int64) ([]Etiqueta, error) {
	filas, err := a.db.Query(`SELECT e.id,e.nombre,e.color,e.posicion,
		(SELECT COUNT(*) FROM chat_etiqueta x WHERE x.bot_id=e.bot_id AND x.etiqueta_id=e.id), e.emoji,
		(SELECT COUNT(*) FROM persona_etiqueta p WHERE p.bot_id=e.bot_id AND p.etiqueta_id=e.id),
		(SELECT COUNT(*) FROM msg_marca mm WHERE mm.bot_id=e.bot_id AND mm.etiqueta_id=e.id)
		FROM etiquetas e WHERE e.bot_id=? ORDER BY e.posicion, e.id`, botID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []Etiqueta{}
	for filas.Next() {
		var x Etiqueta
		if filas.Scan(&x.ID, &x.Nombre, &x.Color, &x.Posicion, &x.Chats, &x.Emoji, &x.Personas, &x.Mensajes) == nil {
			out = append(out, x)
		}
	}
	return out, filas.Err()
}

// EtiquetaPorNombre busca una etiqueta sin importar mayúsculas ni acentos.
func (a *Almacen) EtiquetaPorNombre(botID int64, nombre string) (*Etiqueta, error) {
	todas, err := a.ListarEtiquetas(botID)
	if err != nil {
		return nil, err
	}
	n := identidad.Normalizar(nombre)
	for i := range todas {
		if identidad.Normalizar(todas[i].Nombre) == n {
			return &todas[i], nil
		}
	}
	return nil, nil
}

// CrearEtiqueta crea una etiqueta sin emoji (el nombre no se puede repetir).
func (a *Almacen) CrearEtiqueta(botID int64, nombre, color string) (int64, error) {
	return a.CrearEtiquetaEmoji(botID, nombre, color, "")
}

// CrearEtiquetaEmoji crea una etiqueta con nombre, color y emoji (opcional). No hay límite de etiquetas.
func (a *Almacen) CrearEtiquetaEmoji(botID int64, nombre, color, emoji string) (int64, error) {
	nombre, err := nombreEtiqueta(nombre)
	if err != nil {
		return 0, err
	}
	if emoji, err = emojiEtiqueta(emoji); err != nil {
		return 0, err
	}
	if color, err = colorEtiqueta(color); err != nil {
		return 0, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if e, _ := a.EtiquetaPorNombre(botID, nombre); e != nil {
		return 0, errors.New("ya existe una etiqueta con ese nombre")
	}
	var pos int
	_ = a.db.QueryRow(`SELECT COALESCE(MAX(posicion),0)+1 FROM etiquetas WHERE bot_id=?`, botID).Scan(&pos)
	r, err := a.db.Exec(`INSERT INTO etiquetas(bot_id,nombre,color,emoji,posicion,creado_ts) VALUES(?,?,?,?,?,?)`, botID, nombre, color, emoji, pos, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	id, _ := r.LastInsertId()
	a.avisa("etiquetas")
	return id, nil
}

// EditarEtiqueta cambia el nombre y el color (el emoji se queda como estaba).
func (a *Almacen) EditarEtiqueta(botID, id int64, nombre, color string) error {
	var emoji string
	_ = a.db.QueryRow(`SELECT emoji FROM etiquetas WHERE bot_id=? AND id=?`, botID, id).Scan(&emoji)
	return a.EditarEtiquetaEmoji(botID, id, nombre, color, emoji)
}

// EditarEtiquetaEmoji cambia el nombre, el color y el emoji.
func (a *Almacen) EditarEtiquetaEmoji(botID, id int64, nombre, color, emoji string) error {
	nombre, err := nombreEtiqueta(nombre)
	if err != nil {
		return err
	}
	if emoji, err = emojiEtiqueta(emoji); err != nil {
		return err
	}
	if color, err = colorEtiqueta(color); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if e, _ := a.EtiquetaPorNombre(botID, nombre); e != nil && e.ID != id {
		return errors.New("ya existe una etiqueta con ese nombre")
	}
	r, err := a.db.Exec(`UPDATE etiquetas SET nombre=?, color=?, emoji=? WHERE bot_id=? AND id=?`, nombre, color, emoji, botID, id)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return errors.New("esa etiqueta no existe")
	}
	a.avisa("etiquetas")
	return nil
}

// BorrarEtiqueta borra la etiqueta y la quita de todos los chats, personas y mensajes marcados (una marca que solo tenía esa etiqueta
// y ninguna nota desaparece; la nota sola se conserva como marca sin etiqueta).
func (a *Almacen) BorrarEtiqueta(botID, id int64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, _ = a.db.Exec(`DELETE FROM chat_etiqueta WHERE bot_id=? AND etiqueta_id=?`, botID, id)
	_, _ = a.db.Exec(`DELETE FROM persona_etiqueta WHERE bot_id=? AND etiqueta_id=?`, botID, id)
	a.limpiarMarcasDeEtiqueta(botID, id)
	_, err := a.db.Exec(`DELETE FROM etiquetas WHERE bot_id=? AND id=?`, botID, id)
	if err == nil {
		a.avisa("etiquetas")
		a.avisa("chats")
		a.avisa("personas")
	}
	return err
}

// PonerEtiqueta asigna (si=true) o quita (si=false) la etiqueta de un chat.
func (a *Almacen) PonerEtiqueta(botID int64, chat string, id int64, si bool) error {
	if si {
		var x int
		if err := a.db.QueryRow(`SELECT 1 FROM etiquetas WHERE bot_id=? AND id=?`, botID, id).Scan(&x); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errors.New("esa etiqueta no existe")
			}
			return err
		}
		if _, err := a.db.Exec(`INSERT OR IGNORE INTO chat_etiqueta(bot_id,chat_jid,etiqueta_id) VALUES(?,?,?)`, botID, chat, id); err != nil {
			return err
		}
	} else if _, err := a.db.Exec(`DELETE FROM chat_etiqueta WHERE bot_id=? AND chat_jid=? AND etiqueta_id=?`, botID, chat, id); err != nil {
		return err
	}
	a.avisa("chat:" + chat)
	a.avisa("chats")
	a.avisa("etiquetas")
	return nil
}

// EtiquetasDeChat devuelve las etiquetas de un chat.
func (a *Almacen) EtiquetasDeChat(botID int64, chat string) ([]Etiqueta, error) {
	filas, err := a.db.Query(`SELECT e.id,e.nombre,e.color,e.posicion,e.emoji FROM etiquetas e JOIN chat_etiqueta x ON x.etiqueta_id=e.id
		WHERE x.bot_id=? AND x.chat_jid=? ORDER BY e.posicion, e.id`, botID, chat)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []Etiqueta{}
	for filas.Next() {
		var x Etiqueta
		if filas.Scan(&x.ID, &x.Nombre, &x.Color, &x.Posicion, &x.Emoji) == nil {
			out = append(out, x)
		}
	}
	return out, filas.Err()
}

// etiquetasPorChat trae de una vez las etiquetas (ids) de varios chats.
func (a *Almacen) etiquetasPorChat(botID int64, chats []string) map[string][]int64 {
	out := map[string][]int64{}
	if len(chats) == 0 {
		return out
	}
	args := []any{botID}
	for _, c := range chats {
		args = append(args, c)
	}
	filas, err := a.db.Query(`SELECT x.chat_jid,x.etiqueta_id FROM chat_etiqueta x JOIN etiquetas e ON e.id=x.etiqueta_id
		WHERE x.bot_id=? AND x.chat_jid IN (`+marcas(len(chats))+`) ORDER BY e.posicion, e.id`, args...)
	if err != nil {
		return out
	}
	defer filas.Close()
	for filas.Next() {
		var c string
		var id int64
		if filas.Scan(&c, &id) == nil {
			out[c] = append(out[c], id)
		}
	}
	return out
}

// FiltroChats limita la lista de chats por estado de trabajo y/o etiqueta.
type FiltroChats struct {
	Estado   string // "", "pendiente" o "resuelto"
	Etiqueta int64  // 0 = todas
}

// ListarChatsFiltrados es ListarChats con el filtro de Pendiente/Resuelto y de etiqueta.
func (a *Almacen) ListarChatsFiltrados(botID int64, grupos bool, consulta string, f FiltroChats, limite, desde int) ([]ChatLista, error) {
	if limite <= 0 || limite > 500 {
		limite = 100
	}
	extra, args := "", []any(nil)
	if t := strings.TrimSpace(consulta); t != "" {
		extra += ` AND c.busqueda LIKE ? ESCAPE '\'`
		args = append(args, "%"+escapaLike(identidad.Normalizar(t))+"%")
	}
	if f.Estado == TrabajoPendiente || f.Estado == TrabajoResuelto {
		extra += ` AND e.estado_trabajo=?`
		args = append(args, f.Estado)
	}
	if f.Etiqueta > 0 {
		extra += ` AND EXISTS(SELECT 1 FROM chat_etiqueta x WHERE x.bot_id=c.bot_id AND x.chat_jid=c.jid AND x.etiqueta_id=?)`
		args = append(args, f.Etiqueta)
	}
	return a.listaChats(extra, args, botID, b2i(grupos), limite, desde, false)
}
