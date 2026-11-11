package almacen

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"botchat/internal/cifrado"
)

// ---------- espacio ----------

// EspacioTipo es lo que ocupa una categoría.
type EspacioTipo struct {
	Categoria string `json:"categoria"`
	Bytes     int64  `json:"bytes"`
	Cantidad  int    `json:"cantidad"`
}

// EspacioChat es lo que ocupa un chat, por categoría.
type EspacioChat struct {
	JID      string  `json:"jid"`
	Nombre   string  `json:"nombre"`
	Foto     string  `json:"foto"`
	FotoTS   int64   `json:"fotoTs"`
	EsGrupo  bool    `json:"esGrupo"`
	Pais     string  `json:"pais"`
	Bytes    int64   `json:"bytes"`
	Cantidad int     `json:"cantidad"`
	PorTipo  []int64 `json:"porTipo"` // en el orden de Categorias
}

// EspacioTotal resume lo que ocupa BotChat y lo que queda libre en el teléfono.
type EspacioTotal struct {
	Multimedia int64 `json:"multimedia"` // archivos descargados
	Miniaturas int64 `json:"miniaturas"`
	Fotos      int64 `json:"fotos"` // fotos de perfil
	Base       int64 `json:"base"`  // la base de mensajes
	Libre      int64 `json:"libre"` // espacio libre del teléfono
	Total      int64 `json:"total"` // espacio total del teléfono
}

// Libre devuelve los bytes libres y totales del volumen donde vive la raíz.
func (a *Almacen) Libre() (libre, total int64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(a.raiz, &st); err != nil {
		return 0, 0
	}
	return int64(st.Bavail) * int64(st.Bsize), int64(st.Blocks) * int64(st.Bsize)
}

func tamDir(dir string, filtro func(nombre string) bool) int64 {
	var n int64
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if filtro == nil || filtro(d.Name()) {
			if i, err := d.Info(); err == nil {
				n += i.Size()
			}
		}
		return nil
	})
	return n
}

// EspacioTotal calcula el uso de disco de BotChat para un bot.
func (a *Almacen) EspacioTotal(botID int64) EspacioTotal {
	var e EspacioTotal
	media := filepath.Join(a.raiz, "media", fmt.Sprint(botID))
	e.Multimedia = tamDir(media, func(n string) bool { return !strings.HasSuffix(n, ".thumb.enc") })
	e.Miniaturas = tamDir(media, func(n string) bool { return strings.HasSuffix(n, ".thumb.enc") })
	e.Fotos = tamDir(filepath.Join(a.raiz, "avatars", fmt.Sprint(botID)), nil)
	for _, suf := range []string{"", "-wal", "-shm"} {
		if i, err := os.Stat(filepath.Join(a.raiz, "messages.db"+suf)); err == nil {
			e.Base += i.Size()
		}
	}
	e.Libre, e.Total = a.Libre()
	return e
}

// EspacioPorTipo suma los archivos descargados por categoría.
func (a *Almacen) EspacioPorTipo(botID int64) ([]EspacioTipo, error) {
	out := make([]EspacioTipo, len(Categorias))
	for i, c := range Categorias {
		out[i].Categoria = c
		tipos := tiposDe(c)
		q := "SELECT COALESCE(SUM(archivo_bytes),0), COUNT(*) FROM messages WHERE bot_id=? AND archivo_estado='descargado' AND tipo IN (" + marcas(len(tipos)) + ")"
		args := []any{botID}
		for _, t := range tipos {
			args = append(args, t)
		}
		if err := a.db.QueryRow(q, args...).Scan(&out[i].Bytes, &out[i].Cantidad); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// EspacioPorChat lista los chats que tienen archivos descargados, del que más ocupa al que menos.
func (a *Almacen) EspacioPorChat(botID int64) ([]EspacioChat, error) {
	filas, err := a.db.Query(`SELECT c.jid,c.nombre_mostrado,c.foto_ruta,c.foto_ts,c.es_grupo,c.pais_iso,m.tipo,SUM(m.archivo_bytes),COUNT(*)
		FROM messages m JOIN chats c ON c.jid=m.chat_jid AND c.bot_id=m.bot_id
		WHERE m.bot_id=? AND m.archivo_estado='descargado' GROUP BY c.jid, m.tipo`, botID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	mapa := map[string]*EspacioChat{}
	orden := []string{}
	for filas.Next() {
		var jid, nombre, foto, pais, tipo string
		var fts int64
		var g int
		var bytes int64
		var n int
		if err := filas.Scan(&jid, &nombre, &foto, &fts, &g, &pais, &tipo, &bytes, &n); err != nil {
			return nil, err
		}
		e := mapa[jid]
		if e == nil {
			e = &EspacioChat{JID: jid, Nombre: nombre, Foto: foto, FotoTS: fts, EsGrupo: g == 1, Pais: pais, PorTipo: make([]int64, len(Categorias))}
			mapa[jid] = e
			orden = append(orden, jid)
		}
		for i, c := range Categorias {
			if CategoriaDe(tipo) == c {
				e.PorTipo[i] += bytes
			}
		}
		e.Bytes += bytes
		e.Cantidad += n
	}
	if err := filas.Err(); err != nil {
		return nil, err
	}
	out := make([]EspacioChat, 0, len(orden))
	for _, j := range orden {
		out = append(out, *mapa[j])
	}
	// De mayor a menor
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Bytes > out[j-1].Bytes; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, nil
}

// ---------- borrado ----------

// Filtro elige qué archivos descargados borrar. Todo vacío significa todo.
type Filtro struct {
	Chat      string `json:"chat"`
	Categoria string `json:"categoria"`
	DiasMin   int    `json:"diasMin"` // solo los de más de N días
}

// Item identifica un archivo concreto.
type Item struct {
	Chat string `json:"chat"`
	ID   string `json:"id"`
}

type filaArchivo struct {
	chat, id, ruta, mini string
	bytes                int64
}

func (f Filtro) donde(botID int64, ahora time.Time) (string, []any) {
	q := " WHERE bot_id=? AND archivo_estado='descargado'"
	args := []any{botID}
	if f.Chat != "" {
		q += " AND chat_jid=?"
		args = append(args, f.Chat)
	}
	if tipos := tiposDe(f.Categoria); tipos != nil {
		q += " AND tipo IN (" + marcas(len(tipos)) + ")"
		for _, t := range tipos {
			args = append(args, t)
		}
	}
	if f.DiasMin > 0 {
		q += " AND ts < ?"
		args = append(args, ahora.Add(-time.Duration(f.DiasMin)*24*time.Hour).Unix())
	}
	return q, args
}

func (a *Almacen) filas(q string, args []any) ([]filaArchivo, error) {
	r, err := a.db.Query("SELECT chat_jid,id,archivo_ruta,miniatura_ruta,archivo_bytes FROM messages"+q, args...)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var out []filaArchivo
	for r.Next() {
		var f filaArchivo
		if err := r.Scan(&f.chat, &f.id, &f.ruta, &f.mini, &f.bytes); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, r.Err()
}

// EstimarBorrado dice cuántos archivos y bytes se liberarían con el filtro, sin borrar nada.
func (a *Almacen) EstimarBorrado(botID int64, f Filtro) (bytes int64, n int, err error) {
	q, args := f.donde(botID, time.Now())
	err = a.db.QueryRow("SELECT COALESCE(SUM(archivo_bytes),0), COUNT(*) FROM messages"+q, args...).Scan(&bytes, &n)
	return
}

// BorrarArchivos borra los archivos (y sus miniaturas) que cumplen el filtro. Conserva el texto de los mensajes.
func (a *Almacen) BorrarArchivos(botID int64, f Filtro) (bytes int64, n int, err error) {
	q, args := f.donde(botID, time.Now())
	filas, err := a.filas(q, args)
	if err != nil {
		return 0, 0, err
	}
	return a.borrarFilas(botID, filas)
}

// EstimarLista calcula lo que se liberaría al borrar archivos concretos.
func (a *Almacen) EstimarLista(botID int64, items []Item) (bytes int64, n int, err error) {
	filas, err := a.filasDeItems(botID, items)
	for _, f := range filas {
		bytes += f.bytes
	}
	return bytes, len(filas), err
}

// BorrarLista borra archivos concretos (selección múltiple).
func (a *Almacen) BorrarLista(botID int64, items []Item) (bytes int64, n int, err error) {
	filas, err := a.filasDeItems(botID, items)
	if err != nil {
		return 0, 0, err
	}
	return a.borrarFilas(botID, filas)
}

func (a *Almacen) filasDeItems(botID int64, items []Item) ([]filaArchivo, error) {
	var out []filaArchivo
	for _, it := range items {
		fs, err := a.filas(" WHERE bot_id=? AND chat_jid=? AND id=? AND archivo_estado='descargado'", []any{botID, it.Chat, it.ID})
		if err != nil {
			return nil, err
		}
		out = append(out, fs...)
	}
	return out, nil
}

func (a *Almacen) quita(rel string) {
	if rel == "" {
		return
	}
	if abs, err := a.Abs(rel); err == nil {
		_ = os.Remove(abs)
		_ = os.Remove(abs + ".tmp")
	}
}

func (a *Almacen) borrarFilas(botID int64, filas []filaArchivo) (bytes int64, n int, err error) {
	for _, f := range filas {
		a.quita(f.ruta)
		a.quita(f.mini)
		if _, e := a.db.Exec(`UPDATE messages SET archivo_estado='borrado', archivo_ruta='', miniatura_ruta='' WHERE bot_id=? AND chat_jid=? AND id=?`,
			botID, f.chat, f.id); e != nil {
			err = errors.Join(err, e)
			continue
		}
		bytes += f.bytes
		n++
		a.avisa(fmt.Sprintf("archivo:%s:%s", f.chat, f.id))
	}
	if n > 0 {
		a.avisa("espacio")
	}
	return bytes, n, err
}

// BorrarChat borra un chat completo: mensajes, archivos, miniaturas y foto.
func (a *Almacen) BorrarChat(botID int64, chat string) (bytes int64, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	bytes, _, _ = a.EstimarBorrado(botID, Filtro{Chat: chat})
	c, _ := a.ObtenerChat(botID, chat)
	// Toda la carpeta del chat (archivos y miniaturas, también los .tmp que hayan quedado).
	_ = os.RemoveAll(filepath.Join(a.raiz, "media", fmt.Sprint(botID), HashChat(botID, chat)))
	a.quita(RutaFoto(botID, chat, false))
	a.quita(RutaFoto(botID, chat, true))
	if c != nil && c.FotoRuta != "" {
		a.quita(c.FotoRuta)
	}
	if _, err = a.db.Exec("DELETE FROM chats WHERE bot_id=? AND jid=?", botID, chat); err != nil {
		return 0, err
	}
	if _, err = a.db.Exec("DELETE FROM daily_stats WHERE bot_id=? AND chat=?", botID, chat); err != nil {
		return 0, err
	}
	for _, t := range []string{"message_receipts", "reacciones", "chat_estado", "dedupe", "chat_etiqueta"} {
		if _, err = a.db.Exec("DELETE FROM "+t+" WHERE bot_id=? AND chat_jid=?", botID, chat); err != nil {
			return 0, err
		}
	}
	a.nombres.limpiar()
	a.avisa("chats")
	a.avisa("espacio")
	return bytes, nil
}

// ---------- reparar ----------

// Reporte cuenta lo que "Revisar y reparar" encontró y corrigió.
type Reporte struct {
	EstadosCorregidos   int   `json:"estadosCorregidos"`
	TamanosCorregidos   int   `json:"tamanosCorregidos"`
	ArchivosDanados     int   `json:"archivosDanados"`
	MiniaturasLimpiadas int   `json:"miniaturasLimpiadas"`
	FotosLimpiadas      int   `json:"fotosLimpiadas"`
	Huerfanos           int   `json:"huerfanos"`
	BytesHuerfanos      int64 `json:"bytesHuerfanos"`
	ChatsRecalculados   int   `json:"chatsRecalculados"`
}

// Reparar compara la base con los archivos que existen de verdad y corrige estados y tamaños.
// No toca los archivos que están en descarga (pendiente).
func (a *Almacen) Reparar(botID int64) (Reporte, error) {
	var rep Reporte
	filas, err := a.db.Query(`SELECT chat_jid,id,archivo_ruta,miniatura_ruta,archivo_bytes,archivo_estado FROM messages
		WHERE bot_id=? AND (archivo_estado='descargado' OR miniatura_ruta<>'')`, botID)
	if err != nil {
		return rep, err
	}
	type fila struct {
		chat, id, ruta, mini, estado string
		bytes                        int64
	}
	var todas []fila
	for filas.Next() {
		var f fila
		if err := filas.Scan(&f.chat, &f.id, &f.ruta, &f.mini, &f.bytes, &f.estado); err != nil {
			filas.Close()
			return rep, err
		}
		todas = append(todas, f)
	}
	filas.Close()

	usados := map[string]bool{}
	for _, f := range todas {
		if f.mini != "" {
			if abs, err := a.Abs(f.mini); err == nil {
				if _, err := os.Stat(abs); err != nil {
					_, _ = a.db.Exec("UPDATE messages SET miniatura_ruta='' WHERE bot_id=? AND chat_jid=? AND id=?", botID, f.chat, f.id)
					rep.MiniaturasLimpiadas++
				} else {
					usados[filepath.Clean(abs)] = true
				}
			}
		}
		if f.estado != EstadoDescargado {
			continue
		}
		abs, err := a.Abs(f.ruta)
		if err != nil {
			_, _ = a.db.Exec("UPDATE messages SET archivo_estado='fallido', archivo_ruta='' WHERE bot_id=? AND chat_jid=? AND id=?", botID, f.chat, f.id)
			rep.EstadosCorregidos++
			continue
		}
		info, err := os.Stat(abs)
		if err != nil {
			_, _ = a.db.Exec("UPDATE messages SET archivo_estado='fallido', archivo_ruta='' WHERE bot_id=? AND chat_jid=? AND id=?", botID, f.chat, f.id)
			rep.EstadosCorregidos++
			continue
		}
		usados[filepath.Clean(abs)] = true
		real, ok := a.tamReal(abs, info.Size())
		if !ok {
			_ = os.Remove(abs)
			delete(usados, filepath.Clean(abs))
			_, _ = a.db.Exec("UPDATE messages SET archivo_estado='fallido', archivo_ruta='' WHERE bot_id=? AND chat_jid=? AND id=?", botID, f.chat, f.id)
			rep.ArchivosDanados++
			rep.EstadosCorregidos++
			continue
		}
		if real != f.bytes {
			_, _ = a.db.Exec("UPDATE messages SET archivo_bytes=? WHERE bot_id=? AND chat_jid=? AND id=?", real, botID, f.chat, f.id)
			rep.TamanosCorregidos++
		}
	}

	// Fotos de perfil: se limpia la referencia si el archivo no existe (se volverá a bajar).
	cf, err := a.db.Query("SELECT jid, foto_ruta FROM chats WHERE bot_id=? AND foto_ruta<>''", botID)
	if err == nil {
		var perdidas []string
		for cf.Next() {
			var jid, ruta string
			if cf.Scan(&jid, &ruta) == nil {
				if abs, e := a.Abs(ruta); e != nil {
					perdidas = append(perdidas, jid)
				} else if _, e := os.Stat(abs); e != nil {
					perdidas = append(perdidas, jid)
				} else {
					usados[filepath.Clean(abs)] = true
					if g, e := a.Abs(RutaFoto(botID, jid, true)); e == nil {
						usados[filepath.Clean(g)] = true
					}
				}
			}
		}
		cf.Close()
		for _, jid := range perdidas {
			_, _ = a.db.Exec("UPDATE chats SET foto_ruta='', foto_id='', foto_ts=0 WHERE bot_id=? AND jid=?", botID, jid)
			rep.FotosLimpiadas++
		}
	}

	// Archivos sueltos que ninguna fila usa.
	for _, base := range []string{filepath.Join("media", fmt.Sprint(botID)), filepath.Join("avatars", fmt.Sprint(botID))} {
		raiz := filepath.Join(a.raiz, base)
		_ = filepath.WalkDir(raiz, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if usados[filepath.Clean(p)] {
				return nil
			}
			// Los que están en descarga (.tmp recientes) y los que pertenecen a mensajes en estado "pendiente" se respetan.
			if i, e := d.Info(); e == nil {
				if strings.HasSuffix(d.Name(), ".tmp") && time.Since(i.ModTime()) < 10*time.Minute {
					return nil
				}
				rep.BytesHuerfanos += i.Size()
			}
			if os.Remove(p) == nil {
				rep.Huerfanos++
			}
			return nil
		})
	}

	// La hora del último mensaje de cada chat.
	r, err := a.db.Exec(`UPDATE chats SET ultimo_ts = COALESCE((SELECT MAX(ts) FROM messages m WHERE m.bot_id=chats.bot_id AND m.chat_jid=chats.jid), ultimo_ts)
		WHERE bot_id=? AND ultimo_ts <> COALESCE((SELECT MAX(ts) FROM messages m WHERE m.bot_id=chats.bot_id AND m.chat_jid=chats.jid), ultimo_ts)`, botID)
	if err == nil {
		n, _ := r.RowsAffected()
		rep.ChatsRecalculados = int(n)
	}
	a.avisa("chats")
	a.avisa("espacio")
	return rep, nil
}

// tamReal abre el archivo cifrado y devuelve el tamaño del contenido; false si el formato está dañado o incompleto.
func (a *Almacen) tamReal(abs string, tamDisco int64) (int64, bool) {
	f, err := os.Open(abs)
	if err != nil {
		return 0, false
	}
	defer f.Close()
	l, err := cifrado.Abrir(f, tamDisco, a.clave)
	if err != nil {
		return 0, false
	}
	// Se comprueba el último bloque: un archivo cortado falla aquí.
	if l.Tam() > 0 {
		b := make([]byte, 1)
		if _, err := l.ReadAt(b, l.Tam()-1); err != nil {
			return 0, false
		}
	}
	return l.Tam(), true
}

// Pendientes devuelve los mensajes que quedaron en estado "pendiente" (para reanudar descargas al iniciar).
func (a *Almacen) Pendientes(botID int64) ([]*Mensaje, error) {
	filas, err := a.db.Query("SELECT "+colsMensaje+" FROM messages WHERE bot_id=? AND archivo_estado='pendiente' ORDER BY ts", botID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var out []*Mensaje
	for filas.Next() {
		m, err := a.escanearMensaje(filas)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, filas.Err()
}

// ChatsParaFoto devuelve los chats con mensajes cuya foto está vencida (foto_ts anterior a `antes`) o nunca se pidió.
func (a *Almacen) ChatsParaFoto(botID int64, antes int64, limite int) ([]string, error) {
	filas, err := a.db.Query(`SELECT c.jid FROM chats c WHERE c.bot_id=? AND c.foto_ts < ? AND c.es_grupo IN (0,1)
		AND EXISTS (SELECT 1 FROM messages m WHERE m.bot_id=c.bot_id AND m.chat_jid=c.jid) ORDER BY c.ultimo_ts DESC LIMIT ?`, botID, antes, limite)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var out []string
	for filas.Next() {
		var j string
		if err := filas.Scan(&j); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, filas.Err()
}

// FijarFoto guarda la ruta, el id y la hora de la foto de perfil de un chat.
func (a *Almacen) FijarFoto(botID int64, chat, ruta, id string, ts int64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, err := a.db.Exec("UPDATE chats SET foto_ruta=?, foto_id=?, foto_ts=? WHERE bot_id=? AND jid=?", ruta, id, ts, botID, chat)
	if err == nil {
		a.avisa("chats")
	}
	return err
}

// ExisteChat dice si hay un chat con ese JID.
func (a *Almacen) ExisteChat(botID int64, chat string) bool {
	var x int
	err := a.db.QueryRow("SELECT 1 FROM chats WHERE bot_id=? AND jid=?", botID, chat).Scan(&x)
	return !errors.Is(err, sql.ErrNoRows) && err == nil
}

// TodosLosChats devuelve los JID de todos los chats de un bot (para refrescar nombres).
func (a *Almacen) TodosLosChats(botID int64) ([]string, error) {
	filas, err := a.db.Query("SELECT jid FROM chats WHERE bot_id=?", botID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var out []string
	for filas.Next() {
		var j string
		if err := filas.Scan(&j); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, filas.Err()
}

// EliminarBot borra todo lo de un bot (chats, mensajes, archivos y fotos).
func (a *Almacen) EliminarBot(botID int64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, err := a.db.Exec("DELETE FROM chats WHERE bot_id=?", botID); err != nil {
		return err
	}
	if _, err := a.db.Exec("DELETE FROM daily_stats WHERE bot_id=?", botID); err != nil {
		return err
	}
	for _, t := range []string{"message_receipts", "reacciones", "chat_estado", "dedupe", "actividad",
		"persona_rol", "roles", "persona_alias", "registro_acciones", "solicitudes", "codigos_confirmacion", "grupos_politica", "bot_ajustes", "modulos", "modulos_registro", "no_molestar",
		"persona_silencio", "advertencias", "notas_persona", "chat_etiqueta", "etiquetas", "recordatorios", "alertas_palabra", "avisos"} {
		if _, err := a.db.Exec("DELETE FROM "+t+" WHERE bot_id=?", botID); err != nil {
			return err
		}
	}
	_ = os.RemoveAll(filepath.Join(a.raiz, "media", fmt.Sprint(botID)))
	_ = os.RemoveAll(filepath.Join(a.raiz, "avatars", fmt.Sprint(botID)))
	_ = os.RemoveAll(filepath.Join(a.raiz, "exportes", fmt.Sprint(botID)))
	a.nombres.limpiar()
	a.avisa("chats")
	return nil
}

// TieneMensajes dice si el chat ya tiene al menos un mensaje guardado.
func (a *Almacen) TieneMensajes(botID int64, chat string) bool {
	var x int
	return a.db.QueryRow("SELECT 1 FROM messages WHERE bot_id=? AND chat_jid=? LIMIT 1", botID, chat).Scan(&x) == nil
}
