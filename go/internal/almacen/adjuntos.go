package almacen

import (
	"botchat/internal/cifrado"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Servicio común de adjuntos (bloque D): mensajes automáticos, opciones de menú y comandos propios guardan sus archivos AQUÍ, cifrados
// (menu_adj/<bot>/<id>.bcf). Se suben en cada envío (las URLs de media de WhatsApp caducan); en el teléfono el archivo siempre está cifrado.

// MaxAdjuntos es cuántos archivos puede llevar un mensaje.
const MaxAdjuntos = 3

// Tipos de adjunto permitidos.
const (
	AdjImagen    = "imagen"
	AdjVideo     = "video"
	AdjAudio     = "audio" // solo notas de voz: .ogg, códec Opus, mono
	AdjDocumento = "documento"
)

// LimiteAdjunto es el peso máximo de un archivo de ese tipo (0 = tipo no permitido).
func LimiteAdjunto(tipo string) int64 {
	switch tipo {
	case AdjImagen:
		return 8 << 20
	case AdjAudio:
		return 8 << 20
	case AdjVideo, AdjDocumento:
		return 16 << 20
	}
	return 0
}

// ErrNotaVoz son los motivos por los que un audio no sirve como nota de voz.
var (
	ErrNoOgg       = errors.New("El audio tiene que ser un archivo .ogg. Grábalo desde la app o usa un .ogg con códec Opus.")
	ErrNoOpus      = errors.New("El audio no está en formato Opus, así que WhatsApp no lo reproduciría como nota de voz. Usa un .ogg con códec Opus.")
	ErrNoMono      = errors.New("La nota de voz debe ser mono (un solo canal) y este audio tiene más.")
	ErrOggCorrupto = errors.New("No se pudo leer el audio: el archivo está dañado o incompleto.")
)

// ValidarNotaDeVoz revisa la cabecera de un archivo Ogg: tiene que ser Ogg, con un flujo Opus (paquete OpusHead) de un solo canal.
// Recibe el principio del archivo (con 512 bytes basta).
func ValidarNotaDeVoz(cab []byte) error {
	if len(cab) < 4 || string(cab[:4]) != "OggS" {
		return ErrNoOgg
	}
	if len(cab) < 27 {
		return ErrOggCorrupto
	}
	nseg := int(cab[26])
	ini := 27 + nseg
	if len(cab) < ini+8 {
		return ErrOggCorrupto
	}
	paq := cab[ini:]
	if !bytes.HasPrefix(paq, []byte("OpusHead")) {
		return ErrNoOpus
	}
	if len(paq) < 19 {
		return ErrOggCorrupto
	}
	if paq[8]>>4 != 0 { // versión mayor distinta de 0: no se entiende
		return ErrNoOpus
	}
	if paq[9] != 1 {
		return ErrNoMono
	}
	return nil
}

// AdjuntoOpus lee el número de canales y la duración aproximada (en segundos) de un Ogg/Opus. Para mostrar en la pantalla.
func DuracionOpus(b []byte) int {
	// el granule position de la última página, a 48 kHz
	i := bytes.LastIndex(b, []byte("OggS"))
	if i < 0 || len(b) < i+14 {
		return 0
	}
	g := binary.LittleEndian.Uint64(b[i+6 : i+14])
	if g == 0 || g > 1<<40 {
		return 0
	}
	return int(g / 48000)
}

func limpiaNombre(n string) string {
	n = strings.TrimSpace(n)
	n = strings.NewReplacer("/", "_", "\\", "_", "\x00", "").Replace(n)
	if r := []rune(n); len(r) > 80 {
		n = string(r[:80])
	}
	return n
}

// GuardarAdjuntoDeArchivo valida y guarda (cifrado) el archivo de `ruta`. La copia en claro de `ruta` NO se borra aquí.
// Reglas: tipo permitido, peso máximo por tipo, los documentos llevan nombre, y el audio tiene que ser una nota de voz válida
// (.ogg, Opus, mono).
func (a *Almacen) GuardarAdjuntoDeArchivo(botID int64, ruta, tipo, mime, nombre string) (AdjuntoRef, error) {
	lim := LimiteAdjunto(tipo)
	if lim == 0 {
		return AdjuntoRef{}, errors.New("Ese tipo de archivo no se puede mandar. Elige foto, video, documento o nota de voz.")
	}
	st, err := os.Stat(ruta)
	if err != nil || st.IsDir() || st.Size() == 0 {
		return AdjuntoRef{}, errors.New("No se pudo leer el archivo.")
	}
	if st.Size() > lim {
		return AdjuntoRef{}, fmt.Errorf("El archivo pesa %d MB; para %s el máximo es %d MB.", (st.Size()+(1<<20)-1)>>20, nombreTipo(tipo), lim>>20)
	}
	f, err := os.Open(ruta)
	if err != nil {
		return AdjuntoRef{}, errors.New("No se pudo leer el archivo.")
	}
	defer f.Close()
	nombre = limpiaNombre(nombre)
	switch tipo {
	case AdjAudio:
		cab := make([]byte, 512)
		n, _ := io.ReadFull(f, cab)
		if err := ValidarNotaDeVoz(cab[:n]); err != nil {
			return AdjuntoRef{}, err
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return AdjuntoRef{}, err
		}
		mime = "audio/ogg; codecs=opus"
		if nombre == "" {
			nombre = "nota de voz.ogg"
		}
	case AdjDocumento:
		if nombre == "" {
			nombre = filepath.Base(ruta)
		}
		if mime == "" {
			mime = "application/octet-stream"
		}
	case AdjImagen:
		if !strings.HasPrefix(mime, "image/") {
			mime = "image/jpeg"
		}
	case AdjVideo:
		if !strings.HasPrefix(mime, "video/") {
			mime = "video/mp4"
		}
	}
	id, n, err := a.GuardarAdjuntoMenu(botID, f)
	if err != nil {
		return AdjuntoRef{}, err
	}
	return AdjuntoRef{ID: id, Tipo: tipo, Mime: mime, Nombre: nombre, Bytes: n}, nil
}

func nombreTipo(t string) string {
	switch t {
	case AdjImagen:
		return "una foto"
	case AdjVideo:
		return "un video"
	case AdjAudio:
		return "una nota de voz"
	}
	return "un documento"
}

// IDAdjuntoValido dice si es un id de adjunto bien formado (nunca una ruta).
func IDAdjuntoValido(id string) bool {
	if len(id) != 24 {
		return false
	}
	for _, c := range id {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// ReferenciasAdjuntos devuelve los ids de adjuntos que algo usa todavía: los mensajes automáticos, las automatizaciones (menús y
// comandos propios) y las versiones viejas de un menú que aún tienen una conversación abierta.
func (a *Almacen) ReferenciasAdjuntos(botID int64) map[string]bool {
	textos := []string{}
	if autos, err := a.ListarAutos(botID, ""); err == nil {
		for _, m := range autos {
			for _, x := range m.Adjuntos {
				textos = append(textos, x.ID)
			}
		}
	}
	if menus, err := a.ListarMenus(botID); err == nil {
		for _, m := range menus {
			textos = append(textos, string(m.Def))
		}
	}
	if filas, err := a.db.Query(`SELECT DISTINCT menu_id, menu_version FROM flow_sessions WHERE bot_id=? AND estado IN (?,?,?)`, botID, SesActiva, SesHumano, SesPausada); err == nil {
		type mv struct {
			m int64
			v int
		}
		var vs []mv
		for filas.Next() {
			var x mv
			if filas.Scan(&x.m, &x.v) == nil {
				vs = append(vs, x)
			}
		}
		filas.Close()
		for _, x := range vs {
			textos = append(textos, string(a.VersionDeMenu(x.m, x.v)))
		}
	}
	// mensajes programados y difusiones (módulos H2/H3): su contenido cifrado guarda los ids de sus archivos
	if filas, err := a.db.Query(`SELECT contenido FROM mod_agenda WHERE bot_id=? AND estado NOT IN ('cancelado','terminada')`, botID); err == nil {
		var cs [][]byte
		for filas.Next() {
			var c []byte
			if filas.Scan(&c) == nil {
				cs = append(cs, c)
			}
		}
		filas.Close()
		for _, c := range cs {
			if t, err := cifrado.AbrirTexto(a.clave, "mod_agenda", c); err == nil {
				textos = append(textos, string(t))
			}
		}
	}
	return refsEn(textos)
}

func refsEn(textos []string) map[string]bool {
	out := map[string]bool{}
	for _, t := range textos {
		// un id de adjunto son 24 caracteres hexadecimales
		for i := 0; i+24 <= len(t); i++ {
			if t[i] == '"' && i+25 < len(t) && IDAdjuntoValido(t[i+1:i+25]) && t[i+25] == '"' {
				out[t[i+1:i+25]] = true
			}
		}
		if IDAdjuntoValido(t) {
			out[t] = true
		}
	}
	return out
}

// quitarAdjuntosSueltos borra del teléfono los archivos de `viejos` que `nuevos` ya no incluye, si nada más los usa.
func (a *Almacen) quitarAdjuntosSueltos(botID int64, viejos, nuevos []AdjuntoRef) {
	if len(viejos) == 0 {
		return
	}
	siguen := map[string]bool{}
	for _, n := range nuevos {
		siguen[n.ID] = true
	}
	var quitar []string
	for _, v := range viejos {
		if !siguen[v.ID] && IDAdjuntoValido(v.ID) {
			quitar = append(quitar, v.ID)
		}
	}
	if len(quitar) == 0 {
		return
	}
	refs := a.ReferenciasAdjuntos(botID)
	for _, id := range quitar {
		if !refs[id] {
			a.BorrarAdjuntoMenu(botID, id)
		}
	}
}

// LimpiarAdjuntosHuerfanos borra los archivos guardados que ya nada usa y que tienen más de `gracia` (un archivo recién subido, que
// todavía no se guardó en ningún mensaje, no se toca). Devuelve cuántos borró.
func (a *Almacen) LimpiarAdjuntosHuerfanos(botID int64, gracia time.Duration, ahora time.Time) int {
	dir, err := a.Abs(filepath.Join("menu_adj", fmt.Sprint(botID)))
	if err != nil {
		return 0
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	refs := a.ReferenciasAdjuntos(botID)
	n := 0
	for _, e := range ents {
		nom := e.Name()
		if e.IsDir() || !strings.HasSuffix(nom, ".bcf") {
			continue
		}
		id := strings.TrimSuffix(nom, ".bcf")
		if !IDAdjuntoValido(id) || refs[id] {
			continue
		}
		info, err := e.Info()
		if err != nil || ahora.Sub(info.ModTime()) < gracia {
			continue
		}
		if os.Remove(filepath.Join(dir, nom)) == nil {
			n++
		}
	}
	return n
}

// ExisteAdjunto dice si el archivo guardado existe.
func (a *Almacen) ExisteAdjunto(botID int64, id string) bool {
	abs, err := a.Abs(RutaAdjuntoMenu(botID, id))
	if err != nil {
		return false
	}
	_, err = os.Stat(abs)
	return err == nil
}
