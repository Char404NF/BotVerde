package almacen

import (
	"errors"
	"strings"
	"time"

	"botchat/internal/cifrado"
)

// Etiquetas de personas y marcas de mensajes (v0.6.0, E3). Las etiquetas de chats y grupos siguen en chatpro.go (chat_etiqueta).
// La nota privada de una marca va cifrada con el AAD propio "nota_marca" y nunca sale a registros.

const maxNotaMarca = 2000

func (a *Almacen) existeEtiqueta(botID, id int64) bool {
	var x int
	return a.db.QueryRow(`SELECT 1 FROM etiquetas WHERE bot_id=? AND id=?`, botID, id).Scan(&x) == nil
}

// ---------- personas ----------

// PonerEtiquetaPersona pone (si) o quita la etiqueta de una persona (se guarda con su clave canónica).
func (a *Almacen) PonerEtiquetaPersona(botID int64, persona string, id int64, si bool) error {
	canon := a.ResolverPersona(botID, persona)
	if canon == "" || canon == PersonaDueno {
		return ErrRolDueno
	}
	if si {
		if !a.existeEtiqueta(botID, id) {
			return errors.New("esa etiqueta no existe")
		}
		if _, err := a.db.Exec(`INSERT OR IGNORE INTO persona_etiqueta(bot_id,persona,etiqueta_id) VALUES(?,?,?)`, botID, canon, id); err != nil {
			return err
		}
	} else {
		fs := a.formas(botID, canon)
		if _, err := a.db.Exec(`DELETE FROM persona_etiqueta WHERE bot_id=? AND etiqueta_id=? AND persona IN `+enLista(len(fs)), append([]any{botID, id}, argsSolo(fs)...)...); err != nil {
			return err
		}
	}
	a.avisa("etiquetas")
	a.avisa("personas")
	a.avisa("persona:" + canon)
	return nil
}

// EtiquetasDePersona devuelve las etiquetas de una persona (en cualquiera de sus formas).
func (a *Almacen) EtiquetasDePersona(botID int64, persona string) ([]Etiqueta, error) {
	fs := a.formas(botID, persona)
	filas, err := a.db.Query(`SELECT DISTINCT e.id,e.nombre,e.color,e.posicion,e.emoji FROM etiquetas e JOIN persona_etiqueta x ON x.etiqueta_id=e.id
		WHERE x.bot_id=? AND x.persona IN `+enLista(len(fs))+` ORDER BY e.posicion, e.id`, args2(botID, fs)...)
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

// EtiquetasDePersonas trae de una vez los ids de etiqueta de varias personas (la clave del mapa es la que se pidió).
func (a *Almacen) EtiquetasDePersonas(botID int64, personas []string) map[string][]int64 {
	out := map[string][]int64{}
	if len(personas) == 0 {
		return out
	}
	todas := map[string]bool{}
	for _, p := range personas {
		todas[p] = true
	}
	canon := map[string]string{}
	for i := 0; i < len(personas); i += 400 {
		trozo := personas[i:min(i+400, len(personas))]
		if filas, err := a.db.Query(`SELECT alias,canonica FROM persona_alias WHERE bot_id=? AND alias IN `+enLista(len(trozo)), args2(botID, trozo)...); err == nil {
			for filas.Next() {
				var al, c string
				if filas.Scan(&al, &c) == nil {
					canon[al] = c
					todas[c] = true
				}
			}
			filas.Close()
		}
	}
	claves := make([]string, 0, len(todas))
	for k := range todas {
		claves = append(claves, k)
	}
	porPersona := map[string][]int64{}
	for i := 0; i < len(claves); i += 400 {
		trozo := claves[i:min(i+400, len(claves))]
		filas, err := a.db.Query(`SELECT x.persona,x.etiqueta_id FROM persona_etiqueta x JOIN etiquetas e ON e.id=x.etiqueta_id
			WHERE x.bot_id=? AND x.persona IN `+enLista(len(trozo))+` ORDER BY e.posicion, e.id`, args2(botID, trozo)...)
		if err != nil {
			continue
		}
		for filas.Next() {
			var p string
			var id int64
			if filas.Scan(&p, &id) == nil {
				porPersona[p] = append(porPersona[p], id)
			}
		}
		filas.Close()
	}
	for _, p := range personas {
		ids := porPersona[p]
		if c, ok := canon[p]; ok && c != p {
			for _, id := range porPersona[c] {
				dup := false
				for _, y := range ids {
					if y == id {
						dup = true
					}
				}
				if !dup {
					ids = append(ids, id)
				}
			}
		}
		if len(ids) > 0 {
			out[p] = ids
		}
	}
	return out
}

// EtiquetasDeChats es la versión pública de la consulta por lotes de chats (un solo query para pintar puntitos).
func (a *Almacen) EtiquetasDeChats(botID int64, chats []string) map[string][]int64 {
	return a.etiquetasPorChat(botID, chats)
}

// ---------- mensajes ----------

// Marca es una etiqueta (o solo una nota) puesta a un mensaje.
type Marca struct {
	EtiquetaID int64  `json:"etiquetaId"` // 0 = solo nota
	Nota       string `json:"nota"`
	TS         int64  `json:"ts"`
}

// MarcarMensaje marca un mensaje con una etiqueta (etiquetaID > 0) y/o una nota privada. etiquetaID 0 guarda solo la nota.
// Si ya había marca con esa etiqueta, se cambia su nota (vacía = sin nota).
func (a *Almacen) MarcarMensaje(botID int64, chat, msgID string, etiquetaID int64, nota string, ahora time.Time) error {
	nota = strings.TrimSpace(nota)
	if etiquetaID < 0 {
		return errors.New("etiqueta no válida")
	}
	if etiquetaID == 0 && nota == "" {
		return errors.New("falta la etiqueta o la nota")
	}
	if len([]rune(nota)) > maxNotaMarca {
		return errors.New("la nota es muy larga (máximo 2000 letras)")
	}
	m, err := a.ObtenerMensaje(botID, chat, msgID)
	if err != nil {
		return err
	}
	if m == nil {
		return errors.New("ese mensaje no está guardado")
	}
	if etiquetaID > 0 && !a.existeEtiqueta(botID, etiquetaID) {
		return errors.New("esa etiqueta no existe")
	}
	var sel []byte
	if nota != "" {
		if sel, err = cifrado.SellarTexto(a.clave, "nota_marca", []byte(nota)); err != nil {
			return err
		}
	}
	if _, err := a.db.Exec(`INSERT INTO msg_marca(bot_id,chat_jid,msg_id,etiqueta_id,nota_cifrado,ts) VALUES(?,?,?,?,?,?)
		ON CONFLICT(bot_id,chat_jid,msg_id,etiqueta_id) DO UPDATE SET nota_cifrado=excluded.nota_cifrado, ts=excluded.ts`,
		botID, chat, msgID, etiquetaID, sel, ahora.Unix()); err != nil {
		return err
	}
	a.avisaMsg(chat, msgID)
	a.avisa("etiquetas")
	return nil
}

// QuitarMarca quita una marca del mensaje. etiquetaID < 0 quita todas (etiquetas y nota).
func (a *Almacen) QuitarMarca(botID int64, chat, msgID string, etiquetaID int64) error {
	var err error
	if etiquetaID < 0 {
		_, err = a.db.Exec(`DELETE FROM msg_marca WHERE bot_id=? AND chat_jid=? AND msg_id=?`, botID, chat, msgID)
	} else {
		_, err = a.db.Exec(`DELETE FROM msg_marca WHERE bot_id=? AND chat_jid=? AND msg_id=? AND etiqueta_id=?`, botID, chat, msgID, etiquetaID)
	}
	if err == nil {
		a.avisaMsg(chat, msgID)
		a.avisa("etiquetas")
	}
	return err
}

// MarcasDeMensaje devuelve las marcas (con su nota descifrada) de un mensaje.
func (a *Almacen) MarcasDeMensaje(botID int64, chat, msgID string) ([]Marca, error) {
	filas, err := a.db.Query(`SELECT etiqueta_id,nota_cifrado,ts FROM msg_marca WHERE bot_id=? AND chat_jid=? AND msg_id=? ORDER BY ts, etiqueta_id`, botID, chat, msgID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []Marca{}
	for filas.Next() {
		var m Marca
		var b []byte
		if filas.Scan(&m.EtiquetaID, &b, &m.TS) != nil {
			continue
		}
		if len(b) > 0 {
			if t, e := cifrado.AbrirTexto(a.clave, "nota_marca", b); e == nil {
				m.Nota = string(t)
			}
		}
		out = append(out, m)
	}
	return out, filas.Err()
}

// EtiquetasDeMensajes trae de una vez los ids de etiqueta (sin la "solo nota") de varios mensajes de un chat.
func (a *Almacen) EtiquetasDeMensajes(botID int64, chat string, ids []string) map[string][]int64 {
	out := map[string][]int64{}
	for i := 0; i < len(ids); i += 400 {
		trozo := ids[i:min(i+400, len(ids))]
		args := []any{botID, chat}
		for _, id := range trozo {
			args = append(args, id)
		}
		filas, err := a.db.Query(`SELECT msg_id,etiqueta_id FROM msg_marca WHERE bot_id=? AND chat_jid=? AND msg_id IN `+enLista(len(trozo))+` ORDER BY ts`, args...)
		if err != nil {
			continue
		}
		for filas.Next() {
			var id string
			var e int64
			if filas.Scan(&id, &e) == nil {
				out[id] = append(out[id], e)
			}
		}
		filas.Close()
	}
	return out
}

// limpiarMarcasDeEtiqueta: al borrar una etiqueta, las marcas que tenían nota quedan como "solo nota"; las demás desaparecen.
func (a *Almacen) limpiarMarcasDeEtiqueta(botID, id int64) {
	_, _ = a.db.Exec(`UPDATE OR IGNORE msg_marca SET etiqueta_id=0 WHERE bot_id=? AND etiqueta_id=? AND nota_cifrado IS NOT NULL AND length(nota_cifrado)>0`, botID, id)
	_, _ = a.db.Exec(`DELETE FROM msg_marca WHERE bot_id=? AND etiqueta_id=?`, botID, id)
}

// ---------- listado "Marcados" ----------

// Tipos de lo marcado.
const (
	MarcadoMensaje = "mensaje"
	MarcadoPersona = "persona"
	MarcadoChat    = "chat"
	MarcadoGrupo   = "grupo"
)

// Marcado es una fila de la vista "Marcados".
type Marcado struct {
	Tipo       string  `json:"tipo"`
	Chat       string  `json:"chat"`
	ChatNombre string  `json:"chatNombre"`
	MsgID      string  `json:"msgId"`
	MsgTipo    string  `json:"msgTipo"`
	Texto      string  `json:"texto"`
	Olvidado   bool    `json:"olvidado"`
	EsMio      bool    `json:"esMio"`
	Remitente  string  `json:"remitente"`
	Persona    string  `json:"persona"`
	Nombre     string  `json:"nombre"`
	Foto       string  `json:"foto"`
	TS         int64   `json:"ts"`
	Etiquetas  []int64 `json:"etiquetas"`
	Nota       string  `json:"nota"`
}

// FiltroMarcados: Tipo (mensaje, persona, chat o grupo; vacío = mensaje) y Etiqueta (0 = cualquiera).
type FiltroMarcados struct {
	Tipo     string
	Etiqueta int64
}

// NombreDePersona da el nombre con el que se ve a la persona (el de su chat privado o, si no, el último que usó en un grupo).
func (a *Almacen) NombreDePersona(botID int64, persona string) string {
	for _, x := range a.formas(botID, persona) {
		if c, _ := a.ObtenerChat(botID, x); c != nil && !c.EsGrupo && strings.TrimSpace(c.Mostrado) != "" {
			return c.Mostrado
		}
	}
	for _, x := range a.formas(botID, persona) {
		var n string
		_ = a.db.QueryRow(`SELECT remitente_nombre FROM messages WHERE bot_id=? AND remitente_jid=? AND remitente_nombre<>'' ORDER BY ts DESC LIMIT 1`, botID, x).Scan(&n)
		if n != "" {
			return n
		}
	}
	return ""
}

// ListarMarcados devuelve lo marcado por tipo, lo más reciente primero, con paginación por desplazamiento.
func (a *Almacen) ListarMarcados(botID int64, f FiltroMarcados, limite, desde int) ([]Marcado, error) {
	if limite <= 0 || limite > 500 {
		limite = 100
	}
	if desde < 0 {
		desde = 0
	}
	switch f.Tipo {
	case "", MarcadoMensaje:
		return a.marcadosMensajes(botID, f.Etiqueta, limite, desde)
	case MarcadoPersona:
		return a.marcadosPersonas(botID, f.Etiqueta, limite, desde)
	case MarcadoChat, MarcadoGrupo:
		return a.marcadosChats(botID, f.Etiqueta, f.Tipo == MarcadoGrupo, limite, desde)
	}
	return nil, errors.New("tipo no válido")
}

func (a *Almacen) marcadosMensajes(botID, et int64, limite, desde int) ([]Marcado, error) {
	q := `SELECT chat_jid,msg_id,MAX(ts) t FROM msg_marca WHERE bot_id=?`
	args := []any{botID}
	if et > 0 {
		q += ` AND msg_id IN (SELECT msg_id FROM msg_marca x WHERE x.bot_id=msg_marca.bot_id AND x.chat_jid=msg_marca.chat_jid AND x.etiqueta_id=?)`
		args = append(args, et)
	}
	q += ` GROUP BY chat_jid,msg_id ORDER BY t DESC, msg_id LIMIT ? OFFSET ?`
	args = append(args, limite, desde)
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	type clave struct{ chat, id string }
	var ks []clave
	var tss []int64
	for filas.Next() {
		var k clave
		var t int64
		if filas.Scan(&k.chat, &k.id, &t) == nil {
			ks = append(ks, k)
			tss = append(tss, t)
		}
	}
	filas.Close()
	out := []Marcado{}
	for i, k := range ks {
		m, err := a.ObtenerMensaje(botID, k.chat, k.id)
		if err != nil || m == nil {
			continue
		}
		x := Marcado{Tipo: MarcadoMensaje, Chat: k.chat, MsgID: k.id, MsgTipo: m.Tipo, EsMio: m.EsMio, TS: m.TS, Etiquetas: []int64{}}
		_ = tss[i]
		if m.Tipo != TipoVerUnaVez && !m.Borrado {
			x.Texto = VistaPrevia(m.Tipo, m.Texto)
		}
		x.Olvidado = m.Olvidado
		if m.EsMio {
			x.Remitente = "Yo"
		} else {
			x.Remitente = m.RemitenteNombre
			x.Persona = a.ResolverPersona(botID, m.RemitenteJID)
		}
		if c, _ := a.ObtenerChat(botID, k.chat); c != nil {
			x.ChatNombre, x.Foto = c.Mostrado, c.FotoRuta
		}
		var notas []string
		if ms, _ := a.MarcasDeMensaje(botID, k.chat, k.id); ms != nil {
			for _, mm := range ms {
				if mm.EtiquetaID > 0 {
					x.Etiquetas = append(x.Etiquetas, mm.EtiquetaID)
				}
				if mm.Nota != "" {
					notas = append(notas, mm.Nota)
				}
			}
		}
		x.Nota = strings.Join(notas, "\n")
		out = append(out, x)
	}
	return out, nil
}

func (a *Almacen) marcadosPersonas(botID, et int64, limite, desde int) ([]Marcado, error) {
	q := `SELECT x.persona FROM persona_etiqueta x WHERE x.bot_id=?`
	args := []any{botID}
	if et > 0 {
		q += ` AND x.etiqueta_id=?`
		args = append(args, et)
	}
	q += ` GROUP BY x.persona ORDER BY x.persona LIMIT ? OFFSET ?`
	args = append(args, limite, desde)
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	var ps []string
	for filas.Next() {
		var p string
		if filas.Scan(&p) == nil {
			ps = append(ps, p)
		}
	}
	filas.Close()
	ets := a.EtiquetasDePersonas(botID, ps)
	out := []Marcado{}
	for _, p := range ps {
		x := Marcado{Tipo: MarcadoPersona, Persona: p, Nombre: a.NombreDePersona(botID, p), Etiquetas: ets[p]}
		if x.Etiquetas == nil {
			x.Etiquetas = []int64{}
		}
		for _, f := range a.formas(botID, p) {
			if c, _ := a.ObtenerChat(botID, f); c != nil && !c.EsGrupo {
				x.Foto, x.Chat, x.TS = c.FotoRuta, f, c.UltimoTS
				break
			}
		}
		out = append(out, x)
	}
	return out, nil
}

func (a *Almacen) marcadosChats(botID, et int64, grupos bool, limite, desde int) ([]Marcado, error) {
	q := `SELECT c.jid,c.nombre_mostrado,c.foto_ruta,c.ultimo_ts FROM chats c WHERE c.bot_id=? AND c.es_grupo=?
		AND EXISTS (SELECT 1 FROM chat_etiqueta x WHERE x.bot_id=c.bot_id AND x.chat_jid=c.jid`
	args := []any{botID, b2i(grupos)}
	if et > 0 {
		q += ` AND x.etiqueta_id=?`
		args = append(args, et)
	}
	q += `) ORDER BY c.ultimo_ts DESC, c.jid LIMIT ? OFFSET ?`
	args = append(args, limite, desde)
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	var out []Marcado
	var jids []string
	tipo := MarcadoChat
	if grupos {
		tipo = MarcadoGrupo
	}
	for filas.Next() {
		x := Marcado{Tipo: tipo, Etiquetas: []int64{}}
		if filas.Scan(&x.Chat, &x.Nombre, &x.Foto, &x.TS) == nil {
			x.ChatNombre = x.Nombre
			out = append(out, x)
			jids = append(jids, x.Chat)
		}
	}
	filas.Close()
	ets := a.etiquetasPorChat(botID, jids)
	for i := range out {
		if e := ets[out[i].Chat]; e != nil {
			out[i].Etiquetas = e
		}
	}
	if out == nil {
		out = []Marcado{}
	}
	return out, nil
}

// ContarMarcados cuenta lo marcado por tipo (etiqueta 0 = cualquiera): para los números de las pestañas.
func (a *Almacen) ContarMarcados(botID, et int64) map[string]int {
	out := map[string]int{MarcadoMensaje: 0, MarcadoPersona: 0, MarcadoChat: 0, MarcadoGrupo: 0}
	cuenta := func(q string, args ...any) int {
		var n int
		_ = a.db.QueryRow(q, args...).Scan(&n)
		return n
	}
	ex, ea := "", []any{}
	if et > 0 {
		ex, ea = " AND etiqueta_id=?", []any{et}
	}
	out[MarcadoMensaje] = cuenta(`SELECT COUNT(*) FROM (SELECT 1 FROM msg_marca WHERE bot_id=?`+ex+` GROUP BY chat_jid,msg_id)`, append([]any{botID}, ea...)...)
	out[MarcadoPersona] = cuenta(`SELECT COUNT(DISTINCT persona) FROM persona_etiqueta WHERE bot_id=?`+ex, append([]any{botID}, ea...)...)
	for _, g := range []int{0, 1} {
		n := cuenta(`SELECT COUNT(*) FROM chats c WHERE c.bot_id=? AND c.es_grupo=? AND EXISTS (SELECT 1 FROM chat_etiqueta x WHERE x.bot_id=c.bot_id AND x.chat_jid=c.jid`+
			strings.ReplaceAll(ex, "etiqueta_id", "x.etiqueta_id")+`)`, append([]any{botID, g}, ea...)...)
		if g == 1 {
			out[MarcadoGrupo] = n
		} else {
			out[MarcadoChat] = n
		}
	}
	return out
}

// borrarMarcasDeMensaje quita las marcas de un mensaje que se borra.
func (a *Almacen) borrarMarcasDeMensaje(botID int64, chat, id string) {
	_, _ = a.db.Exec(`DELETE FROM msg_marca WHERE bot_id=? AND chat_jid=? AND msg_id=?`, botID, chat, id)
}
