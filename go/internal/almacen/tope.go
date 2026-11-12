package almacen

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"
)

// Tope total del almacenamiento (v0.6.0, etapa E2): cuando lo descargado pasa del tope se borra lo más viejo hasta bajar al 90 %.
// Nunca se borra: lo marcado o etiquetado, un "ver una vez", un video que todavía no tiene portada ni lo que llegó hace menos de una hora.

// TopeDefecto es el tope total de fábrica (5 GB).
const TopeDefecto int64 = 5 << 30

// TopeTotal devuelve el tope total en bytes (0 = sin límite).
func (a *Almacen) TopeTotal(botID int64) int64 {
	v := strings.TrimSpace(a.AjusteBot(botID, "tope_total_bytes"))
	var n int64
	if _, err := fmt.Sscan(v, &n); err != nil || n < 0 {
		return TopeDefecto
	}
	return n
}

// FijarTopeTotal cambia el tope total (0 = sin límite).
func (a *Almacen) FijarTopeTotal(botID int64, bytes int64) error {
	if bytes < 0 {
		bytes = 0
	}
	if err := a.FijarAjusteBot(botID, "tope_total_bytes", fmt.Sprint(bytes)); err != nil {
		return err
	}
	a.avisa("espacio")
	return nil
}

// ConservarPortada dice si al borrar un video se conserva su portada (de fábrica sí).
func (a *Almacen) ConservarPortada(botID int64) bool {
	return a.AjusteBot(botID, "conservar_portada") != "0"
}

// FijarConservarPortada enciende o apaga el interruptor.
func (a *Almacen) FijarConservarPortada(botID int64, si bool) error {
	v := "0"
	if si {
		v = "1"
	}
	return a.FijarAjusteBot(botID, "conservar_portada", v)
}

// tiposPolitica traduce la clave de tipo de la política a los tipos de mensaje.
func tiposPolitica(clave string) []string {
	switch clave {
	case "foto":
		return []string{TipoImagen}
	case "sticker":
		return []string{TipoSticker}
	case "video":
		return []string{TipoVideo}
	case "audio":
		return []string{TipoAudio}
	case "documento":
		return []string{TipoDocumento}
	}
	return nil
}

// claveUnica agrupa las filas que comparten archivo (misma huella): un duplicado enlazado no se cuenta dos veces.
const claveUnica = "CASE WHEN sha256 IS NULL THEN 'r'||rowid ELSE hex(sha256) END"

// sumaUnica suma los bytes descargados de esos tipos contando una sola vez los archivos con la misma huella.
// La cantidad es la de mensajes (cada duplicado sigue siendo un mensaje con su archivo).
func (a *Almacen) sumaUnica(botID int64, tipos []string) (bytes int64, n int, err error) {
	q := "SELECT COALESCE(SUM(b),0) FROM (SELECT MAX(archivo_bytes) b FROM messages WHERE bot_id=? AND archivo_estado='descargado'"
	args := []any{botID}
	c := "SELECT COUNT(*) FROM messages WHERE bot_id=? AND archivo_estado='descargado'"
	if tipos != nil {
		in := " AND tipo IN (" + marcas(len(tipos)) + ")"
		q += in
		c += in
		for _, t := range tipos {
			args = append(args, t)
		}
	}
	q += " GROUP BY " + claveUnica + ")"
	if err = a.db.QueryRow(q, args...).Scan(&bytes); err != nil {
		return
	}
	err = a.db.QueryRow(c, args...).Scan(&n)
	return
}

// UsoDeTipo devuelve los bytes descargados de un tipo de la política (foto, video, audio, documento, sticker), sin contar duplicados.
func (a *Almacen) UsoDeTipo(botID int64, clave string) int64 {
	t := tiposPolitica(clave)
	if t == nil {
		return 0
	}
	b, _, _ := a.sumaUnica(botID, t)
	return b
}

// AhorradoPorDuplicados dice cuántos bytes y archivos se evitaron guardando enlaces en vez de copias.
func (a *Almacen) AhorradoPorDuplicados(botID int64) (bytes int64, archivos int) {
	var total, unico int64
	var filas, unicos int
	_ = a.db.QueryRow("SELECT COALESCE(SUM(archivo_bytes),0), COUNT(*) FROM messages WHERE bot_id=? AND archivo_estado='descargado' AND sha256 IS NOT NULL", botID).Scan(&total, &filas)
	_ = a.db.QueryRow("SELECT COALESCE(SUM(b),0), COUNT(*) FROM (SELECT MAX(archivo_bytes) b FROM messages WHERE bot_id=? AND archivo_estado='descargado' AND sha256 IS NOT NULL GROUP BY sha256)", botID).Scan(&unico, &unicos)
	return total - unico, filas - unicos
}

// ---------- duplicados (enlace duro) ----------

// Duplicado busca un mensaje que ya tenga bajado el mismo archivo (misma huella y tamaño) y devuelve su ruta relativa.
func (a *Almacen) Duplicado(botID int64, sha []byte, chat, id string) (ruta string, bytes int64, ok bool) {
	if len(sha) == 0 {
		return "", 0, false
	}
	filas, err := a.db.Query(`SELECT archivo_ruta, archivo_bytes FROM messages WHERE bot_id=? AND sha256=? AND archivo_estado='descargado'
		AND archivo_ruta<>'' AND NOT (chat_jid=? AND id=?) AND tipo<>? LIMIT 5`, botID, sha, chat, id, TipoVerUnaVez)
	if err != nil {
		return "", 0, false
	}
	defer filas.Close()
	for filas.Next() {
		var r string
		var b int64
		if filas.Scan(&r, &b) != nil {
			continue
		}
		if abs, err := a.Abs(r); err == nil {
			if st, err := os.Stat(abs); err == nil && st.Mode().IsRegular() {
				return r, b, true
			}
		}
	}
	return "", 0, false
}

// EnlazarDuplicado deja en `destino` (ruta relativa) un enlace duro al archivo `origen`. Falso si el sistema no lo permite
// (entonces se descarga normal). Los archivos cifrados no dependen del nombre, así que el enlace se lee igual.
func (a *Almacen) EnlazarDuplicado(origen, destino string) bool {
	o, err := a.Abs(origen)
	if err != nil {
		return false
	}
	d, err := a.Abs(destino)
	if err != nil {
		return false
	}
	if err := os.MkdirAll(dirDe(d), 0o700); err != nil {
		return false
	}
	_ = os.Remove(d) // por si quedó un archivo viejo del mismo mensaje
	return os.Link(o, d) == nil
}

func dirDe(p string) string {
	if i := strings.LastIndexByte(p, os.PathSeparator); i > 0 {
		return p[:i]
	}
	return "."
}

// ---------- uso real y tope ----------

// archivoInfo es lo que importa de un archivo para saber cuánto libera borrarlo.
type archivoInfo struct {
	dev, ino uint64
	nlink    uint64
	tam      int64
	existe   bool
}

func statArchivo(abs string) archivoInfo {
	var st syscall.Stat_t
	if err := syscall.Stat(abs, &st); err != nil {
		return archivoInfo{}
	}
	return archivoInfo{dev: uint64(st.Dev), ino: uint64(st.Ino), nlink: uint64(st.Nlink), tam: st.Size, existe: true}
}

// liberables dice cuántos bytes (de contenido) se liberan de verdad al quitar esas filas: un archivo con otros enlaces fuera de la
// lista no libera nada hasta que se quite el último. Cuenta cada archivo físico una sola vez.
func (a *Almacen) liberables(filas []filaArchivo) (bytes int64, n int) {
	type clave struct{ dev, ino uint64 }
	cuenta := map[clave]uint64{}
	infos := make([]archivoInfo, len(filas))
	for i, f := range filas {
		abs, err := a.Abs(f.ruta)
		if err != nil {
			continue
		}
		infos[i] = statArchivo(abs)
		if infos[i].existe {
			cuenta[clave{infos[i].dev, infos[i].ino}]++
		}
	}
	hecho := map[clave]bool{}
	for i, f := range filas {
		in := infos[i]
		if !in.existe {
			bytes += f.bytes
			n++
			continue
		}
		k := clave{in.dev, in.ino}
		if hecho[k] {
			continue
		}
		if cuenta[k] >= in.nlink {
			hecho[k] = true
			bytes += f.bytes
			n++
		}
	}
	return
}

// UsoMultimedia suma lo que ocupan en disco los archivos descargados (sin miniaturas ni portadas), contando una vez los enlaces duros.
func (a *Almacen) UsoMultimedia(botID int64) int64 {
	return tamDir(a.dirMedia(botID), func(n string) bool { return !esMini(n) })
}

func esMini(n string) bool {
	return strings.HasSuffix(n, ".thumb.enc") || strings.HasSuffix(n, ".cover.enc")
}

// ResultadoTope cuenta lo que hizo AplicarTope.
type ResultadoTope struct {
	Borrados int   `json:"borrados"`
	Bytes    int64 `json:"bytes"`
	Antes    int64 `json:"antes"`
	Despues  int64 `json:"despues"`
	Tope     int64 `json:"tope"`
}

func (a *Almacen) existeTabla(nombre string) bool {
	var x int
	return a.db.QueryRow("SELECT 1 FROM sqlite_master WHERE type='table' AND name=?", nombre).Scan(&x) == nil
}

// condicionProtegido es la parte del WHERE que deja fuera lo que el tope nunca borra (alias m para messages).
func (a *Almacen) condicionProtegido() string {
	c := ` AND m.tipo<>'` + TipoVerUnaVez + `' AND NOT (m.tipo='` + TipoVideo + `' AND m.portada_ruta='')
		AND NOT EXISTS (SELECT 1 FROM chat_etiqueta ce WHERE ce.bot_id=m.bot_id AND ce.chat_jid=m.chat_jid)`
	if a.existeTabla("msg_marca") {
		c += ` AND NOT EXISTS (SELECT 1 FROM msg_marca mm WHERE mm.bot_id=m.bot_id AND mm.chat_jid=m.chat_jid AND mm.msg_id=m.id)`
	}
	return c
}

// AplicarTope borra los archivos descargados más viejos si lo descargado pasa del tope, hasta bajar al 90 %.
// tope <= 0 no hace nada. Deja un aviso local con lo que se borró. Es seguro llamarlo seguido (si no pasa del tope sale enseguida).
func (a *Almacen) AplicarTope(botID int64, tope int64, ahora time.Time) (ResultadoTope, error) {
	res := ResultadoTope{Tope: tope}
	if tope <= 0 {
		return res, nil
	}
	res.Antes = a.UsoMultimedia(botID)
	res.Despues = res.Antes
	if res.Antes <= tope {
		return res, nil
	}
	objetivo := tope / 10 * 9
	corte := ahora.Add(-time.Hour)
	var ultTS int64 = -1 << 62
	ultID := ""
	prot := a.condicionProtegido()
	for res.Despues > objetivo {
		filas, err := a.db.Query(`SELECT m.chat_jid,m.id,m.archivo_ruta,m.miniatura_ruta,m.archivo_bytes,m.portada_ruta,m.tipo,m.ts FROM messages m
			WHERE m.bot_id=? AND m.archivo_estado='descargado' AND m.ts<? AND (m.ts>? OR (m.ts=? AND m.id>?))`+prot+` ORDER BY m.ts, m.id LIMIT 200`,
			botID, corte.Unix(), ultTS, ultTS, ultID)
		if err != nil {
			return res, err
		}
		var lote []filaArchivo
		for filas.Next() {
			var f filaArchivo
			var ts int64
			if filas.Scan(&f.chat, &f.id, &f.ruta, &f.mini, &f.bytes, &f.portada, &f.tipo, &ts) == nil {
				lote = append(lote, f)
				ultTS, ultID = ts, f.id
			}
		}
		filas.Close()
		if len(lote) == 0 {
			break
		}
		// Se elige archivo por archivo hasta llegar al objetivo; lo que llegó hace menos de una hora (por la hora del archivo) se salta.
		var elegidos []filaArchivo
		type clave struct{ dev, ino uint64 }
		quedan := map[clave]uint64{}
		for _, f := range lote {
			if res.Despues <= objetivo {
				break
			}
			abs, err := a.Abs(f.ruta)
			if err != nil {
				continue
			}
			in := statArchivo(abs)
			if st, e := os.Stat(abs); e == nil && st.ModTime().After(corte) {
				continue
			}
			elegidos = append(elegidos, f)
			if !in.existe {
				continue
			}
			k := clave{in.dev, in.ino}
			if _, ok := quedan[k]; !ok {
				quedan[k] = in.nlink
			}
			quedan[k]--
			if quedan[k] == 0 { // se fue el último enlace: ahora sí se libera el disco
				res.Despues -= in.tam
			}
		}
		if len(elegidos) > 0 {
			b, n, err := a.borrarFilas(botID, elegidos, "tope")
			res.Bytes += b
			res.Borrados += n
			if err != nil {
				return res, err
			}
		}
		if len(lote) < 200 {
			break
		}
	}
	if res.Borrados > 0 {
		titulo := fmt.Sprintf("Se borraron %d archivos viejos", res.Borrados)
		if res.Borrados == 1 {
			titulo = "Se borró 1 archivo viejo"
		}
		_, _ = a.CrearAviso(botID, "almacenamiento", "", titulo, fmt.Sprintf("Se liberaron %s para no pasar del tope de %s. Los chats y su texto siguen igual.", humanoBytes(res.Antes-res.Despues), humanoBytes(tope)))
	}
	return res, nil
}

func humanoBytes(n int64) string {
	const mb, gb = 1 << 20, 1 << 30
	switch {
	case n >= gb:
		return fmt.Sprintf("%.1f GB", float64(n)/gb)
	case n >= mb:
		return fmt.Sprintf("%d MB", n/mb)
	}
	return fmt.Sprintf("%d KB", n/1024)
}

// ---------- portadas de video ----------

// SinPortada es un video descargado que todavía no tiene portada.
type SinPortada struct {
	Chat  string `json:"chat"`
	ID    string `json:"id"`
	Ruta  string `json:"ruta"`
	Bytes int64  `json:"bytes"`
}

// VideosSinPortada devuelve hasta `limite` videos descargados sin portada, de los más nuevos a los más viejos.
func (a *Almacen) VideosSinPortada(botID int64, limite int) ([]SinPortada, error) {
	if limite <= 0 || limite > 200 {
		limite = 20
	}
	filas, err := a.db.Query(`SELECT chat_jid,id,archivo_ruta,archivo_bytes FROM messages WHERE bot_id=? AND tipo=? AND archivo_estado='descargado'
		AND portada_ruta='' AND archivo_ruta<>'' ORDER BY ts DESC LIMIT ?`, botID, TipoVideo, limite)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []SinPortada{}
	for filas.Next() {
		var s SinPortada
		if filas.Scan(&s.Chat, &s.ID, &s.Ruta, &s.Bytes) == nil {
			out = append(out, s)
		}
	}
	return out, filas.Err()
}

// RutaPortada es media/<bot>/<hash del chat>/<id>.cover.enc
func RutaPortada(botID int64, chat, id string) string {
	r := RutaMedio(botID, chat, id)
	return strings.TrimSuffix(r, ".enc") + ".cover.enc"
}

// GuardarPortada guarda cifrada la portada (JPEG) de un video descargado y la anota en su fila.
func (a *Almacen) GuardarPortada(botID int64, chat, id string, jpeg []byte) error {
	if len(jpeg) == 0 {
		return fmt.Errorf("portada vacía")
	}
	var tipo string
	if err := a.db.QueryRow("SELECT tipo FROM messages WHERE bot_id=? AND chat_jid=? AND id=?", botID, chat, id).Scan(&tipo); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("el mensaje ya no existe")
		}
		return err
	}
	if tipo != TipoVideo {
		return fmt.Errorf("el mensaje no es un video")
	}
	rel := RutaPortada(botID, chat, id)
	if err := a.EscribirCifrado(rel, jpeg); err != nil {
		return err
	}
	if _, err := a.db.Exec("UPDATE messages SET portada_ruta=? WHERE bot_id=? AND chat_jid=? AND id=?", rel, botID, chat, id); err != nil {
		return err
	}
	a.avisa("portada:" + chat + ":" + id)
	a.avisa("espacio")
	return nil
}

// PortadaRuta devuelve la ruta de la portada de un mensaje ("" si no tiene).
func (a *Almacen) PortadaRuta(botID int64, chat, id string) string {
	var r string
	_ = a.db.QueryRow("SELECT portada_ruta FROM messages WHERE bot_id=? AND chat_jid=? AND id=?", botID, chat, id).Scan(&r)
	return r
}

// FijarSHASiFalta guarda la huella del archivo si el mensaje todavía no la tenía.
func (a *Almacen) FijarSHASiFalta(botID int64, chat, id string, sha []byte) error {
	if len(sha) == 0 {
		return nil
	}
	_, err := a.db.Exec("UPDATE messages SET sha256=? WHERE bot_id=? AND chat_jid=? AND id=? AND sha256 IS NULL", sha, botID, chat, id)
	return err
}

// UsoClave es lo descargado de un tipo de la política (el sticker aparte de la foto).
type UsoClave struct {
	Tipo     string `json:"tipo"`
	Bytes    int64  `json:"bytes"`
	Cantidad int    `json:"cantidad"`
}

// UsoPorClave suma lo descargado por tipo de la política: foto, video, audio, documento y sticker (sin contar duplicados).
func (a *Almacen) UsoPorClave(botID int64) ([]UsoClave, error) {
	var out []UsoClave
	for _, k := range []string{"foto", "video", "audio", "documento", "sticker"} {
		b, n, err := a.sumaUnica(botID, tiposPolitica(k))
		if err != nil {
			return nil, err
		}
		out = append(out, UsoClave{Tipo: k, Bytes: b, Cantidad: n})
	}
	return out, nil
}

// ContarSinPortada cuenta los videos descargados que todavía no tienen portada.
func (a *Almacen) ContarSinPortada(botID int64) int {
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE bot_id=? AND tipo=? AND archivo_estado='descargado' AND portada_ruta=''`, botID, TipoVideo).Scan(&n)
	return n
}
