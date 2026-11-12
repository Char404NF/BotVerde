package almacen

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"botchat/internal/cifrado"
	"botchat/internal/identidad"
)

// Auditoría (v0.6.0, etapa E1).
//
// El interruptor `audit_texto` decide si se guarda el TEXTO de lo que llega (texto, cita, nombre de archivo y menciones). Apagado, la
// fila se guarda igual con tipo, hora, remitente y contadores (las estadísticas no cambian) y queda marcada como "olvidada".
// La retención (30, 90 o 0 = siempre) "olvida" el contenido viejo conservando las filas. Nada de esto toca registros ni logs.

// Retención permitida (días; 0 = siempre).
var retencionesValidas = map[int]bool{0: true, 30: true, 90: true}

// Valores de messages.contenido_olvidado.
const (
	contenidoNormal   = 0
	contenidoOlvidado = 1 // se quitó el texto (interruptor apagado o retención)
	contenidoRevisado = 2 // la retención lo revisó y no había nada que olvidar
)

// AuditoriaGlobal dice si el texto se guarda por defecto (de fábrica sí).
func (a *Almacen) AuditoriaGlobal(botID int64) bool {
	return a.AjusteBot(botID, "audit_defecto") != "0"
}

// AuditoriaActiva dice si se guarda el texto de ese chat: el interruptor del chat si lo tiene, o el global.
func (a *Almacen) AuditoriaActiva(botID int64, chat string) bool {
	var v int
	err := a.db.QueryRow(`SELECT audit_texto FROM chat_estado WHERE bot_id=? AND chat_jid=?`, botID, chat).Scan(&v)
	if err == nil && v >= 0 {
		return v == 1
	}
	return a.AuditoriaGlobal(botID)
}

// AplicarAuditoria deja el mensaje como debe guardarse: con la auditoría apagada en su chat quita texto, cita, nombre de archivo y
// menciones (conserva tipo, hora, remitente, tamaños y lo necesario para descargar) y lo marca como olvidado si había algo que quitar.
// Es el ÚNICO punto por el que pasan los mensajes que llegan (en vivo y del historial). El "ver una vez" conserva su texto fijo.
func (a *Almacen) AplicarAuditoria(m *Mensaje) {
	if m == nil || a.AuditoriaActiva(m.BotID, m.ChatJID) {
		return
	}
	hubo := m.CitaTexto != "" || len(m.Menciones) > 0 || m.ArchivoNombre != ""
	if m.Tipo != TipoVerUnaVez {
		hubo = hubo || m.Texto != ""
		m.Texto = ""
	}
	m.CitaTexto, m.Menciones, m.ArchivoNombre, m.Enlaces = "", nil, "", false
	if hubo {
		m.Olvidado = true
	}
}

// AjustesAuditoria es el estado global que ve la pantalla.
type AjustesAuditoria struct {
	Defecto      bool `json:"defecto"`      // guardar el texto por defecto
	RetDias      int  `json:"retDias"`      // 30, 90 o 0 (siempre)
	RetPendiente bool `json:"retPendiente"` // viene de una versión anterior y aún no elige: no se purga nada
	AvisoVisto   bool `json:"avisoVisto"`
}

// Auditoria devuelve los ajustes globales de auditoría.
func (a *Almacen) Auditoria(botID int64) AjustesAuditoria {
	return AjustesAuditoria{
		Defecto:      a.AuditoriaGlobal(botID),
		RetDias:      a.AjusteBotInt(botID, "audit_ret_dias"),
		RetPendiente: a.AjusteBot(botID, "audit_ret_pendiente") == "1",
		AvisoVisto:   a.AjusteBot(botID, "audit_aviso_visto") == "1",
	}
}

// FijarAuditoria cambia el interruptor global (guardar) y la retención global. retDias < 0 deja la retención como está; si se elige
// una (30, 90 o 0) termina el estado "pendiente de elegir".
func (a *Almacen) FijarAuditoria(botID int64, guardar bool, retDias int) error {
	if retDias >= 0 && !retencionesValidas[retDias] {
		return errors.New("la retención debe ser 30, 90 o 0 (siempre)")
	}
	if err := a.FijarAjusteBot(botID, "audit_defecto", fmt.Sprint(b2i(guardar))); err != nil {
		return err
	}
	if retDias >= 0 {
		if err := a.FijarAjusteBot(botID, "audit_ret_dias", fmt.Sprint(retDias)); err != nil {
			return err
		}
		return a.FijarAjusteBot(botID, "audit_ret_pendiente", "0")
	}
	return nil
}

// FijarRetencion es un atajo: el usuario eligió retención (30, 90 o 0) sin tocar el interruptor.
func (a *Almacen) FijarRetencion(botID int64, retDias int) error {
	if !retencionesValidas[retDias] {
		return errors.New("la retención debe ser 30, 90 o 0 (siempre)")
	}
	if err := a.FijarAjusteBot(botID, "audit_ret_dias", fmt.Sprint(retDias)); err != nil {
		return err
	}
	return a.FijarAjusteBot(botID, "audit_ret_pendiente", "0")
}

// AuditoriaDeChat es lo que tiene un chat: sus valores propios (-1 = usa el global) y lo que vale en la práctica.
type AuditoriaDeChat struct {
	Chat        string `json:"chat"`
	Nombre      string `json:"nombre,omitempty"`
	EsGrupo     bool   `json:"esGrupo,omitempty"`
	UltimoTS    int64  `json:"ultimoTs,omitempty"`
	Audit       int    `json:"audit"`       // -1 usa el global, 0 no guarda el texto, 1 sí
	Efectivo    bool   `json:"efectivo"`    // lo que vale ahora
	RetDias     int    `json:"retDias"`     // -1 usa la global, 0 siempre, 30 o 90
	RetEfectiva int    `json:"retEfectiva"` // lo que vale ahora (0 = siempre)
}

func (a *Almacen) efectivosDe(botID int64, audit, ret int) (bool, int) {
	ef := a.AuditoriaGlobal(botID)
	if audit >= 0 {
		ef = audit == 1
	}
	r := a.AjusteBotInt(botID, "audit_ret_dias")
	if ret >= 0 {
		r = ret
	}
	return ef, r
}

// AuditoriaChat devuelve la configuración de auditoría de un chat.
func (a *Almacen) AuditoriaChat(botID int64, chat string) AuditoriaDeChat {
	d := AuditoriaDeChat{Chat: chat, Audit: -1, RetDias: -1}
	_ = a.db.QueryRow(`SELECT audit_texto, audit_ret_dias FROM chat_estado WHERE bot_id=? AND chat_jid=?`, botID, chat).Scan(&d.Audit, &d.RetDias)
	d.Efectivo, d.RetEfectiva = a.efectivosDe(botID, d.Audit, d.RetDias)
	return d
}

// FijarAuditoriaChat cambia el interruptor (-1 = usar el global, 0, 1) y la retención (-1 = usar la global, 0, 30, 90) de un chat.
func (a *Almacen) FijarAuditoriaChat(botID int64, chat string, audit, retDias int) error {
	if audit < -1 || audit > 1 {
		return errors.New("valor de auditoría no válido")
	}
	if retDias != -1 && !retencionesValidas[retDias] {
		return errors.New("la retención debe ser 30, 90 o 0 (siempre)")
	}
	a.asegurarEstado(botID, chat)
	if _, err := a.db.Exec(`UPDATE chat_estado SET audit_texto=?, audit_ret_dias=? WHERE bot_id=? AND chat_jid=?`, audit, retDias, botID, chat); err != nil {
		return err
	}
	a.avisa("chat:" + chat)
	return nil
}

// ListarAuditoriaChats lista los chats con su configuración de auditoría (los más recientes primero). consulta filtra por nombre/número.
func (a *Almacen) ListarAuditoriaChats(botID int64, consulta string, limite, desde int) ([]AuditoriaDeChat, error) {
	if limite <= 0 || limite > 500 {
		limite = 200
	}
	q := `SELECT c.jid, c.nombre_mostrado, c.es_grupo, c.ultimo_ts, COALESCE(e.audit_texto,-1), COALESCE(e.audit_ret_dias,-1)
		FROM chats c LEFT JOIN chat_estado e ON e.bot_id=c.bot_id AND e.chat_jid=c.jid
		WHERE c.bot_id=? AND c.ultimo_ts>0`
	args := []any{botID}
	if t := strings.TrimSpace(consulta); t != "" {
		q += ` AND c.busqueda LIKE ? ESCAPE '\'`
		args = append(args, "%"+escapaLike(identidad.Normalizar(t))+"%")
	}
	q += ` ORDER BY c.ultimo_ts DESC, c.jid LIMIT ? OFFSET ?`
	args = append(args, limite, desde)
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []AuditoriaDeChat{}
	for filas.Next() {
		var d AuditoriaDeChat
		var g int
		if err := filas.Scan(&d.Chat, &d.Nombre, &g, &d.UltimoTS, &d.Audit, &d.RetDias); err != nil {
			return nil, err
		}
		d.EsGrupo = g == 1
		d.Efectivo, d.RetEfectiva = a.efectivosDe(botID, d.Audit, d.RetDias)
		out = append(out, d)
	}
	return out, filas.Err()
}

// ---------- historial de ediciones ----------

// VersionMensaje es un texto anterior de un mensaje editado.
type VersionMensaje struct {
	N     int    `json:"n"`
	TS    int64  `json:"ts"` // cuándo fue reemplazado por la edición siguiente
	Texto string `json:"texto"`
}

// VersionesDeMensaje devuelve los textos anteriores de un mensaje editado, de la más vieja a la más reciente (sin incluir el actual).
func (a *Almacen) VersionesDeMensaje(botID int64, chat, id string) ([]VersionMensaje, error) {
	filas, err := a.db.Query(`SELECT n, ts, texto_cifrado FROM mensaje_ediciones WHERE bot_id=? AND chat_jid=? AND msg_id=? ORDER BY n`, botID, chat, id)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []VersionMensaje{}
	for filas.Next() {
		var v VersionMensaje
		var b []byte
		if err := filas.Scan(&v.N, &v.TS, &b); err != nil {
			return nil, err
		}
		if t, err := cifrado.AbrirTexto(a.clave, "texto_ed", b); err == nil {
			v.Texto = string(t)
		}
		out = append(out, v)
	}
	return out, filas.Err()
}

// ---------- retención ----------

// ResultadoRetencion cuenta lo que hizo una pasada de retención.
type ResultadoRetencion struct {
	Pendiente bool `json:"pendiente"` // todavía no se eligió retención: no se tocó nada
	Mensajes  int  `json:"mensajes"`  // mensajes cuyo texto se olvidó
	Ediciones int  `json:"ediciones"` // versiones anteriores borradas
}

// Lote de la retención (mensajes por transacción).
const loteRetencion = 500

// LimpiarPorRetencion olvida el contenido (texto, cita, nombre de archivo y menciones) de los mensajes más viejos que la retención de su
// chat, conservando las filas, la hora, el remitente y las estadísticas, y borra sus versiones de edición. Va por lotes de 500.
// Mientras la retención esté "pendiente de elegir" NO hace nada. Se puede cancelar con ctx.
func (a *Almacen) LimpiarPorRetencion(ctx context.Context, botID int64, ahora time.Time) (ResultadoRetencion, error) {
	res := ResultadoRetencion{}
	if a.AjusteBot(botID, "audit_ret_pendiente") == "1" {
		res.Pendiente = true
		return res, nil
	}
	if global := a.AjusteBotInt(botID, "audit_ret_dias"); global > 0 {
		if err := a.olvidarAntes(ctx, botID, ahora.AddDate(0, 0, -global).Unix(), "", &res); err != nil {
			return res, err
		}
	}
	filas, err := a.db.Query(`SELECT chat_jid, audit_ret_dias FROM chat_estado WHERE bot_id=? AND audit_ret_dias>0`, botID)
	if err != nil {
		return res, err
	}
	type par struct {
		chat string
		dias int
	}
	var pares []par
	for filas.Next() {
		var p par
		if filas.Scan(&p.chat, &p.dias) == nil {
			pares = append(pares, p)
		}
	}
	filas.Close()
	for _, p := range pares {
		if err := a.olvidarAntes(ctx, botID, ahora.AddDate(0, 0, -p.dias).Unix(), p.chat, &res); err != nil {
			return res, err
		}
	}
	if res.Mensajes > 0 {
		a.avisa("chats")
		a.avisa("espacio")
	}
	return res, nil
}

// olvidarAntes olvida el contenido de los mensajes con hora < corte. Con chat vacío, los de todos los chats que NO tienen retención propia.
func (a *Almacen) olvidarAntes(ctx context.Context, botID, corte int64, chat string, res *ResultadoRetencion) error {
	const hay = `((texto_cifrado IS NOT NULL AND tipo<>'ver_una_vez') OR cita_texto_cifrado IS NOT NULL OR menciones_cifrado IS NOT NULL OR archivo_nombre_cifrado IS NOT NULL)`
	q := `SELECT id, chat_jid, ` + hay + ` FROM messages WHERE bot_id=? AND contenido_olvidado=0 AND ts<?`
	args := []any{botID, corte}
	if chat == "" {
		q += ` AND chat_jid NOT IN (SELECT chat_jid FROM chat_estado WHERE bot_id=? AND audit_ret_dias>=0)`
		args = append(args, botID)
	} else {
		q += ` AND chat_jid=?`
		args = append(args, chat)
	}
	q += ` ORDER BY ts LIMIT ` + fmt.Sprint(loteRetencion)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		type fila struct {
			id, chat string
			hay      bool
		}
		filas, err := a.db.Query(q, args...)
		if err != nil {
			return err
		}
		var lote []fila
		for filas.Next() {
			var f fila
			var h int
			if filas.Scan(&f.id, &f.chat, &h) == nil {
				f.hay = h == 1
				lote = append(lote, f)
			}
		}
		filas.Close()
		if len(lote) == 0 {
			return nil
		}
		tx, err := a.db.Begin()
		if err != nil {
			return err
		}
		for _, f := range lote {
			marca := contenidoRevisado
			if f.hay {
				marca = contenidoOlvidado
			}
			if _, err := tx.Exec(`UPDATE messages SET texto_cifrado=CASE WHEN tipo='ver_una_vez' THEN texto_cifrado ELSE NULL END,
				cita_texto_cifrado=NULL, menciones_cifrado=NULL, archivo_nombre_cifrado=NULL, contenido_olvidado=?
				WHERE bot_id=? AND chat_jid=? AND id=?`, marca, botID, f.chat, f.id); err != nil {
				_ = tx.Rollback()
				return err
			}
			r, err := tx.Exec(`DELETE FROM mensaje_ediciones WHERE bot_id=? AND chat_jid=? AND msg_id=?`, botID, f.chat, f.id)
			if err != nil {
				_ = tx.Rollback()
				return err
			}
			n, _ := r.RowsAffected()
			res.Ediciones += int(n)
			if f.hay {
				res.Mensajes++
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		if len(lote) < loteRetencion {
			return nil
		}
		select { // un respiro entre lotes para no acaparar la base
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pausaRetencion):
		}
	}
}

// pausaRetencion es la espera entre lotes (las pruebas la ponen en 0).
var pausaRetencion = 20 * time.Millisecond

// ---------- huella (sha256) de archivos viejos ----------

// RellenarSHA calcula, por lotes y en segundo plano, la huella sha256 de los mensajes anteriores a la migración 20 a partir de los datos
// de descarga (descifra solo ese JSON pequeño; no toca los archivos). Recuerda por dónde va. Devuelve cuántos llenó y si ya terminó.
func (a *Almacen) RellenarSHA(botID int64, lote int) (hechos int, fin bool, err error) {
	if a.AjusteBot(botID, "sha_relleno_fin") == "1" {
		return 0, true, nil
	}
	if lote <= 0 {
		lote = 200
	}
	tope := int64(a.AjusteBotInt(botID, "sha_relleno_tope"))
	if tope == 0 {
		if err := a.db.QueryRow(`SELECT COALESCE(MAX(rowid),0) FROM messages WHERE bot_id=?`, botID).Scan(&tope); err != nil {
			return 0, false, err
		}
		if tope == 0 {
			tope = -1 // no había nada: terminado
		}
		if err := a.guardaAjusteSilencioso(botID, "sha_relleno_tope", fmt.Sprint(tope)); err != nil {
			return 0, false, err
		}
	}
	cursor := int64(a.AjusteBotInt(botID, "sha_relleno_cursor"))
	filas, err := a.db.Query(`SELECT rowid, archivo_info_cifrado FROM messages
		WHERE bot_id=? AND rowid>? AND rowid<=? AND sha256 IS NULL AND archivo_info_cifrado IS NOT NULL ORDER BY rowid LIMIT ?`, botID, cursor, tope, lote)
	if err != nil {
		return 0, false, err
	}
	type fila struct {
		rowid int64
		sha   []byte
	}
	var hechas []fila
	var ultimo int64
	n := 0
	for filas.Next() {
		var rid int64
		var b []byte
		if err := filas.Scan(&rid, &b); err != nil {
			filas.Close()
			return 0, false, err
		}
		n++
		ultimo = rid
		if claro, err := cifrado.AbrirTexto(a.clave, "archivo_info", b); err == nil {
			var j struct {
				SHA string `json:"sha"`
			}
			if json.Unmarshal(claro, &j) == nil && j.SHA != "" {
				if sha, err := base64.StdEncoding.DecodeString(j.SHA); err == nil && len(sha) > 0 {
					hechas = append(hechas, fila{rid, sha})
				}
			}
		}
	}
	filas.Close()
	if n == 0 {
		return 0, true, a.guardaAjusteSilencioso(botID, "sha_relleno_fin", "1")
	}
	tx, err := a.db.Begin()
	if err != nil {
		return 0, false, err
	}
	for _, h := range hechas {
		if _, err := tx.Exec(`UPDATE messages SET sha256=? WHERE rowid=? AND sha256 IS NULL`, h.sha, h.rowid); err != nil {
			_ = tx.Rollback()
			return 0, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, false, err
	}
	if err := a.guardaAjusteSilencioso(botID, "sha_relleno_cursor", fmt.Sprint(ultimo)); err != nil {
		return len(hechas), false, err
	}
	return len(hechas), false, nil
}

// guardaAjusteSilencioso guarda un ajuste interno sin avisar a la pantalla.
func (a *Almacen) guardaAjusteSilencioso(botID int64, clave, valor string) error {
	_, err := a.db.Exec(`INSERT INTO bot_ajustes(bot_id, clave, valor) VALUES(?,?,?) ON CONFLICT(bot_id,clave) DO UPDATE SET valor=excluded.valor`, botID, clave, valor)
	return err
}
