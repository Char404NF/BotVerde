package almacen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Perfil de persona, actividad por chat, lista de lo que envió, búsqueda de texto, puntitos de etiqueta y exportar chat (v0.6.0, E4).
// Todo por persona canónica con sus alias (PN y LID). Los contadores salen de daily_stats (sin descifrar); solo la búsqueda por
// palabra descifra, por lotes, y solo las filas que ya pasaron los filtros baratos.

// ---------- perfil ----------

// GrupoComun es un grupo donde está la persona.
type GrupoComun struct {
	JID    string `json:"jid"`
	Nombre string `json:"nombre"`
	Foto   string `json:"foto"`
}

// PerfilPersona es todo lo que muestra la cabecera del perfil.
type PerfilPersona struct {
	Persona       string       `json:"persona"`
	Nombre        string       `json:"nombre"`
	Telefono      string       `json:"telefono"`
	Rango         string       `json:"rango"`
	RangoColor    string       `json:"rangoColor"`
	Foto          string       `json:"foto"`
	Etiquetas     []Etiqueta   `json:"etiquetas"`
	Mensajes      int          `json:"mensajes"`      // todo lo que escribió (privado y grupos)
	Archivos      int          `json:"archivos"`      // fotos, videos, audios y documentos que envió
	ArchivosBytes int64        `json:"archivosBytes"` // lo que pesan
	GruposComun   []GrupoComun `json:"gruposComun"`
	Ficha         *Ficha       `json:"ficha"`
}

var tiposArchivo = []string{TipoImagen, TipoVideo, TipoAudio, TipoDocumento}

// PerfilDe arma el perfil de una persona.
func (a *Almacen) PerfilDe(botID int64, persona string, ahora time.Time) (*PerfilPersona, error) {
	f, err := a.FichaDe(botID, persona, ahora)
	if err != nil {
		return nil, err
	}
	p := &PerfilPersona{Persona: f.Persona, Nombre: f.Nombre, Telefono: f.Telefono, Etiquetas: f.Etiquetas, Mensajes: f.Recibidos, Ficha: f, GruposComun: []GrupoComun{}}
	if p.Nombre == "" {
		p.Nombre = a.NombreDePersona(botID, f.Persona)
	}
	if p.Telefono == "" {
		if i := strings.Index(f.Persona, "@s.whatsapp.net"); i > 0 {
			p.Telefono = "+" + f.Persona[:i]
		}
	}
	if r, _, err := a.RolDePersona(botID, f.Persona, ahora); err == nil && r != nil {
		p.Rango, p.RangoColor = r.Nombre, r.Color
	}
	p.Foto = a.FotoDePersona(botID, f.Persona)
	fs := f.Alias
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(count),0),COALESCE(SUM(bytes),0) FROM daily_stats WHERE bot_id=? AND sender IN `+enLista(len(fs))+` AND tipo IN `+enLista(len(tiposArchivo)),
		append(args2(botID, fs), argsSolo(tiposArchivo)...)...).Scan(&p.Archivos, &p.ArchivosBytes)
	gs, _ := a.GruposEnComun(botID, f.Persona)
	for _, g := range gs {
		x := GrupoComun{JID: g}
		if c, _ := a.ObtenerChat(botID, g); c != nil {
			x.Nombre, x.Foto = c.Mostrado, c.FotoRuta
		}
		p.GruposComun = append(p.GruposComun, x)
	}
	sort.Slice(p.GruposComun, func(i, j int) bool { return plegar(p.GruposComun[i].Nombre) < plegar(p.GruposComun[j].Nombre) })
	return p, nil
}

// ---------- actividad por chat ----------

// TarjetaActividad es un renglón de "Actividad": el chat privado o un grupo, con lo que la persona envió ahí.
type TarjetaActividad struct {
	Chat       string `json:"chat"`
	Nombre     string `json:"nombre"`
	Foto       string `json:"foto"`
	Privado    bool   `json:"privado"`
	Miembro    bool   `json:"miembro"`  // sigue en el grupo (según la última lista de miembros)
	Mensajes   int    `json:"mensajes"` // todo lo que envió (cualquier tipo)
	Fotos      int    `json:"fotos"`
	Videos     int    `json:"videos"`
	Audios     int    `json:"audios"`
	Archivos   int    `json:"archivos"` // documentos
	Enlaces    int    `json:"enlaces"`
	MisEnvios  int    `json:"misEnvios"` // solo en el chat privado: lo que yo le escribí
	UltimoTS   int64  `json:"ultimoTs"`
	TieneDatos bool   `json:"tieneDatos"`
}

// ActividadPersona devuelve la tarjeta "Chat privado" (siempre primera) y una por cada grupo donde escribió o donde está.
func (a *Almacen) ActividadPersona(botID int64, persona string) ([]TarjetaActividad, error) {
	canon := a.ResolverPersona(botID, persona)
	if canon == "" {
		return nil, fmt.Errorf("falta la persona")
	}
	fs := a.formas(botID, canon)
	priv := a.chatsPrivados(botID, fs)
	tarjetas := map[string]*TarjetaActividad{}
	privada := &TarjetaActividad{Chat: "", Nombre: "Chat privado", Privado: true}
	if len(priv) > 0 {
		privada.Chat = priv[0]
		for _, x := range priv {
			if c, _ := a.ObtenerChat(botID, x); c != nil {
				privada.Chat, privada.Foto = x, c.FotoRuta
				break
			}
		}
	}
	get := func(chat string) *TarjetaActividad {
		if !strings.HasSuffix(chat, "@g.us") {
			return privada
		}
		t := tarjetas[chat]
		if t == nil {
			t = &TarjetaActividad{Chat: chat}
			if c, _ := a.ObtenerChat(botID, chat); c != nil {
				t.Nombre, t.Foto = c.Mostrado, c.FotoRuta
			}
			if t.Nombre == "" {
				t.Nombre = "Grupo"
			}
			tarjetas[chat] = t
		}
		return t
	}
	filas, err := a.db.Query(`SELECT chat,tipo,SUM(count) FROM daily_stats WHERE bot_id=? AND sender IN `+enLista(len(fs))+` GROUP BY chat,tipo`, args2(botID, fs)...)
	if err != nil {
		return nil, err
	}
	for filas.Next() {
		var chat, tipo string
		var n int
		if filas.Scan(&chat, &tipo, &n) != nil {
			continue
		}
		if !strings.HasSuffix(chat, "@g.us") && !enListaStr(priv, chat) {
			continue
		}
		t := get(chat)
		t.TieneDatos = true
		t.Mensajes += n
		switch tipo {
		case TipoImagen:
			t.Fotos += n
		case TipoVideo:
			t.Videos += n
		case TipoAudio:
			t.Audios += n
		case TipoDocumento:
			t.Archivos += n
		}
	}
	filas.Close()
	// enlaces y última hora: del índice por autor (una consulta para todos los chats de la persona)
	if f2, err := a.db.Query(`SELECT chat_jid,COALESCE(SUM(enlaces),0),MAX(ts) FROM messages WHERE bot_id=? AND remitente_jid IN `+enLista(len(fs))+` GROUP BY chat_jid`, args2(botID, fs)...); err == nil {
		for f2.Next() {
			var chat string
			var en int
			var ts int64
			if f2.Scan(&chat, &en, &ts) != nil {
				continue
			}
			if !strings.HasSuffix(chat, "@g.us") && !enListaStr(priv, chat) {
				continue
			}
			t := get(chat)
			t.Enlaces += en
			if ts > t.UltimoTS {
				t.UltimoTS = ts
			}
		}
		f2.Close()
	}
	if len(priv) > 0 {
		_ = a.db.QueryRow(`SELECT COALESCE(SUM(count),0) FROM daily_stats WHERE bot_id=? AND sender=? AND chat IN `+enLista(len(priv)),
			append([]any{botID, QuienYo}, argsSolo(priv)...)...).Scan(&privada.MisEnvios)
	}
	gs, _ := a.GruposEnComun(botID, canon)
	for _, g := range gs {
		get(g).Miembro = true
	}
	out := []TarjetaActividad{*privada}
	grupos := make([]TarjetaActividad, 0, len(tarjetas))
	for _, t := range tarjetas {
		grupos = append(grupos, *t)
	}
	sort.Slice(grupos, func(i, j int) bool {
		if grupos[i].UltimoTS != grupos[j].UltimoTS {
			return grupos[i].UltimoTS > grupos[j].UltimoTS
		}
		if grupos[i].Mensajes != grupos[j].Mensajes {
			return grupos[i].Mensajes > grupos[j].Mensajes
		}
		return plegar(grupos[i].Nombre) < plegar(grupos[j].Nombre)
	})
	return append(out, grupos...), nil
}

func enListaStr(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// ---------- lista de lo que envió ----------

// Pestañas de la lista.
const (
	PestanaTodo      = ""
	PestanaMensajes  = "mensajes"
	PestanaFotos     = "fotos"
	PestanaAudios    = "audios"
	PestanaArchivos  = "archivos"
	topeEscaneoLlama = 200000 // filas que una sola llamada de búsqueda por palabra puede descifrar
	loteEscaneo      = 500
)

// FiltroPersona describe qué quiere ver: lo que `Persona` envió en `Chat` (cualquiera de los dos puede ir vacío; con ambos vacíos no
// hay filtro, así que la búsqueda de texto de un chat es este mismo filtro con solo `Chat` y `Palabra`).
type FiltroPersona struct {
	Chat         string `json:"chat"`
	Persona      string `json:"persona"`
	Pestana      string `json:"pestana"`      // "", mensajes, fotos, audios, archivos
	Desde        int64  `json:"desde"`        // hora desde (incluida); 0 = sin límite
	Hasta        int64  `json:"hasta"`        // hora hasta (excluida); 0 = sin límite
	Palabra      string `json:"palabra"`      // sin importar mayúsculas ni acentos; varias palabras = todas
	SoloMarcados bool   `json:"soloMarcados"` // con etiqueta o nota mía
	SoloBorrados bool   `json:"soloBorrados"` // borrados (los que el bot alcanzó a ver)
	SoloEditados bool   `json:"soloEditados"` // editados (los que el bot alcanzó a ver)
}

// ParseFiltroPersona lee el filtro de su JSON.
func ParseFiltroPersona(s string) (FiltroPersona, error) {
	var f FiltroPersona
	if strings.TrimSpace(s) == "" {
		return f, nil
	}
	err := json.Unmarshal([]byte(s), &f)
	return f, err
}

// MensajePersona es un mensaje de la lista con sus marcas.
type filaP struct {
	id, chat string
	ts       int64
}

type MensajePersona struct {
	MensajeVista
	MarcaRoja  string  `json:"marcaRoja"`  // "borrado", "editado" o "" (solo lo que el bot vio pasar)
	BorradoPor string  `json:"borradoPor"` // "autor" si lo borró quien lo envió (o WhatsApp no dijo quién); si no, el nombre de quien lo borró
	Versiones  int     `json:"versiones"`  // textos anteriores guardados de un mensaje editado
	Marcado    bool    `json:"marcado"`    // tiene etiqueta o nota mía
	Etiquetas  []int64 `json:"etiquetas"`  // etiquetas del mensaje (sin las notas solas)
}

// PaginaPersona es una página de la lista. Con palabra, `Total` es -1 mientras no se haya recorrido todo.
type PaginaPersona struct {
	Total      int64            `json:"total"`
	Items      []MensajePersona `json:"items"`
	Fin        bool             `json:"fin"`   // no hay más páginas
	SigTS      int64            `json:"sigTs"` // para pedir la siguiente: hora y id del último recorrido
	SigID      string           `json:"sigId"`
	Escaneados int              `json:"escaneados"` // mensajes descifrados al buscar por palabra
}

func (a *Almacen) condicionPersona(botID int64, f FiltroPersona) (string, []any, error) {
	q := ` WHERE bot_id=?`
	args := []any{botID}
	if f.Chat != "" {
		q += ` AND chat_jid=?`
		args = append(args, f.Chat)
	}
	if f.Persona != "" {
		fs := a.formas(botID, f.Persona)
		q += ` AND remitente_jid IN ` + enLista(len(fs))
		args = append(args, argsSolo(fs)...)
	}
	switch f.Pestana {
	case PestanaMensajes:
		q += ` AND tipo NOT IN ('imagen','video','audio','documento')`
	case PestanaFotos:
		q += ` AND tipo IN ('imagen','video')`
	case PestanaAudios:
		q += ` AND tipo='audio'`
	case PestanaArchivos:
		q += ` AND (tipo='documento' OR enlaces=1)`
	case PestanaTodo:
	default:
		return "", nil, fmt.Errorf("pestaña desconocida: %s", f.Pestana)
	}
	if f.Desde > 0 {
		q += ` AND ts>=?`
		args = append(args, f.Desde)
	}
	if f.Hasta > 0 {
		q += ` AND ts<?`
		args = append(args, f.Hasta)
	}
	if f.SoloBorrados {
		q += ` AND borrado=1`
	}
	if f.SoloEditados {
		q += ` AND editado=1`
	}
	if f.SoloMarcados {
		q += ` AND EXISTS (SELECT 1 FROM msg_marca k WHERE k.bot_id=messages.bot_id AND k.chat_jid=messages.chat_jid AND k.msg_id=messages.id)`
	}
	return q, args, nil
}

// MensajesDePersona devuelve, del más nuevo al más viejo, lo que la persona envió que cumple el filtro. Para la página siguiente se
// pasan (antesTS, antesID) = (SigTS, SigID) de la anterior; con antesTS=0 es la primera.
func (a *Almacen) MensajesDePersona(botID int64, f FiltroPersona, antesTS int64, antesID string, limite int) (*PaginaPersona, error) {
	if limite <= 0 || limite > 500 {
		limite = 50
	}
	cond, args, err := a.condicionPersona(botID, f)
	if err != nil {
		return nil, err
	}
	pag := &PaginaPersona{Items: []MensajePersona{}}
	var elegidos []filaP
	palabras := strings.Fields(plegar(f.Palabra))
	curTS, curID := antesTS, antesID
	if len(palabras) == 0 {
		if antesTS == 0 {
			if err := a.db.QueryRow(`SELECT COUNT(*) FROM messages`+cond, args...).Scan(&pag.Total); err != nil {
				return nil, err
			}
		}
		q := `SELECT id,ts,chat_jid FROM messages` + cond
		qa := append([]any{}, args...)
		if antesTS > 0 {
			q += ` AND (ts<? OR (ts=? AND id<?))`
			qa = append(qa, antesTS, antesTS, antesID)
		}
		q += ` ORDER BY ts DESC, id DESC LIMIT ?`
		qa = append(qa, limite+1)
		filas, err := a.db.Query(q, qa...)
		if err != nil {
			return nil, err
		}
		for filas.Next() {
			var r filaP
			if filas.Scan(&r.id, &r.ts, &r.chat) == nil {
				elegidos = append(elegidos, r)
			}
		}
		filas.Close()
		pag.Fin = len(elegidos) <= limite
		if len(elegidos) > limite {
			elegidos = elegidos[:limite]
		}
	} else {
		pag.Total = -1
		// escaneo por lotes: pasa los filtros baratos por SQL, descifra el lote y se queda con los que traen todas las palabras
	escaneo:
		for pag.Escaneados < topeEscaneoLlama {
			q := `SELECT id,ts,chat_jid,texto_cifrado,archivo_nombre_cifrado FROM messages` + cond
			qa := append([]any{}, args...)
			if curTS > 0 {
				q += ` AND (ts<? OR (ts=? AND id<?))`
				qa = append(qa, curTS, curTS, curID)
			}
			q += ` ORDER BY ts DESC, id DESC LIMIT ?`
			qa = append(qa, loteEscaneo)
			filas, err := a.db.Query(q, qa...)
			if err != nil {
				return nil, err
			}
			n := 0
			for filas.Next() {
				var r filaP
				var t, nom []byte
				if filas.Scan(&r.id, &r.ts, &r.chat, &t, &nom) != nil {
					continue
				}
				n++
				pag.Escaneados++
				curTS, curID = r.ts, r.id
				if len(t) == 0 && len(nom) == 0 {
					continue
				}
				hay := plegar(a.abreTexto("texto", t) + " " + a.abreTexto("archivo_nombre", nom))
				ok := true
				for _, p := range palabras {
					if !strings.Contains(hay, p) {
						ok = false
						break
					}
				}
				if !ok {
					continue
				}
				elegidos = append(elegidos, r)
				if len(elegidos) >= limite {
					filas.Close()
					break escaneo
				}
			}
			filas.Close()
			if n < loteEscaneo {
				pag.Fin = true
				break
			}
		}
		if pag.Fin && antesTS == 0 {
			pag.Total = int64(len(elegidos))
		}
	}
	if len(palabras) > 0 {
		pag.SigTS, pag.SigID = curTS, curID
	} else if len(elegidos) > 0 {
		pag.SigTS, pag.SigID = elegidos[len(elegidos)-1].ts, elegidos[len(elegidos)-1].id
	}
	// armar las vistas: cada mensaje completo (la página es chica)
	ms := make([]*Mensaje, 0, len(elegidos))
	for _, r := range elegidos {
		if m, err := a.ObtenerMensaje(botID, r.chat, r.id); err == nil && m != nil {
			ms = append(ms, m)
		}
	}
	vs := a.vistasDeMensajes(ms)
	pag.Items = a.conMarcas(botID, vs)
	return pag, nil
}

// conMarcas agrega a cada vista la marca roja, las versiones guardadas y las etiquetas puestas (consultas por lote y por chat).
func (a *Almacen) conMarcas(botID int64, vs []MensajeVista) []MensajePersona {
	out := make([]MensajePersona, len(vs))
	porChat := map[string][]string{}
	for i, v := range vs {
		out[i] = MensajePersona{MensajeVista: v, Etiquetas: []int64{}}
		porChat[v.Chat] = append(porChat[v.Chat], v.ID)
	}
	marcas := map[string]map[string][]int64{} // chat -> id -> etiquetas (0 = solo nota)
	vers := map[string]map[string]int{}
	for chat, ids := range porChat {
		marcas[chat], vers[chat] = map[string][]int64{}, map[string]int{}
		for i := 0; i < len(ids); i += 400 {
			trozo := ids[i:min(i+400, len(ids))]
			args := append([]any{botID, chat}, argsSolo(trozo)...)
			if fl, err := a.db.Query(`SELECT msg_id,etiqueta_id FROM msg_marca WHERE bot_id=? AND chat_jid=? AND msg_id IN `+enLista(len(trozo))+` ORDER BY ts`, args...); err == nil {
				for fl.Next() {
					var id string
					var e int64
					if fl.Scan(&id, &e) == nil {
						marcas[chat][id] = append(marcas[chat][id], e)
					}
				}
				fl.Close()
			}
			if fl, err := a.db.Query(`SELECT msg_id,COUNT(*) FROM mensaje_ediciones WHERE bot_id=? AND chat_jid=? AND msg_id IN `+enLista(len(trozo))+` GROUP BY msg_id`, args...); err == nil {
				for fl.Next() {
					var id string
					var n int
					if fl.Scan(&id, &n) == nil {
						vers[chat][id] = n
					}
				}
				fl.Close()
			}
		}
	}
	for i := range out {
		m := &out[i]
		if es, ok := marcas[m.Chat][m.ID]; ok {
			m.Marcado = true
			for _, e := range es {
				if e > 0 {
					m.Etiquetas = append(m.Etiquetas, e)
				}
			}
		}
		m.Versiones = vers[m.Chat][m.ID]
		switch {
		case m.Borrado:
			m.MarcaRoja, m.BorradoPor = "borrado", "autor"
			if m.EliminadoPor != "" {
				m.BorradoPor = m.EliminadoPor
			}
		case m.Editado:
			m.MarcaRoja = "editado"
		}
	}
	return out
}

// BuscarEnChat busca texto (sin importar mayúsculas ni acentos) en un chat; es el escaneo por lotes de MensajesDePersona.
func (a *Almacen) BuscarEnChat(botID int64, chat, palabra string, desde, hasta int64, antesTS int64, antesID string, limite int) (*PaginaPersona, error) {
	if strings.TrimSpace(palabra) == "" {
		return &PaginaPersona{Items: []MensajePersona{}, Fin: true}, nil
	}
	return a.MensajesDePersona(botID, FiltroPersona{Chat: chat, Palabra: palabra, Desde: desde, Hasta: hasta}, antesTS, antesID, limite)
}

// ---------- puntitos de etiqueta ----------

// PuntitosDeMensajes devuelve {idMensaje: [idEtiqueta…]} con las etiquetas de QUIEN ENVIÓ cada mensaje (solo los que tienen alguna).
// Una consulta para los remitentes y otra para sus etiquetas.
func (a *Almacen) PuntitosDeMensajes(botID int64, chat string, ids []string) map[string][]int64 {
	out := map[string][]int64{}
	remitente := map[string]string{}
	for i := 0; i < len(ids); i += 400 {
		trozo := ids[i:min(i+400, len(ids))]
		args := append([]any{botID, chat}, argsSolo(trozo)...)
		fl, err := a.db.Query(`SELECT id,remitente_jid FROM messages WHERE bot_id=? AND chat_jid=? AND es_mio=0 AND id IN `+enLista(len(trozo)), args...)
		if err != nil {
			continue
		}
		for fl.Next() {
			var id, r string
			if fl.Scan(&id, &r) == nil && r != "" {
				remitente[id] = r
			}
		}
		fl.Close()
	}
	unicos, vistos := []string{}, map[string]bool{}
	for _, r := range remitente {
		if !vistos[r] {
			vistos[r] = true
			unicos = append(unicos, r)
		}
	}
	et := a.EtiquetasDePersonas(botID, unicos)
	for id, r := range remitente {
		if e := et[r]; len(e) > 0 {
			out[id] = e
		}
	}
	return out
}

// ---------- exportar chat ----------

// MensajeChatExportado es una línea de la exportación de un chat.
type MensajeChatExportado struct {
	ID        string `json:"id"`
	TS        int64  `json:"ts"`
	De        string `json:"de"`
	EsMio     bool   `json:"esMio"`
	Tipo      string `json:"tipo"`
	Texto     string `json:"texto,omitempty"`
	Archivo   string `json:"archivo,omitempty"`
	Eliminado bool   `json:"eliminado,omitempty"`
	Editado   bool   `json:"editado,omitempty"`
	Olvidado  bool   `json:"olvidado,omitempty"`
}

// ExporteChat es el contenido del archivo de exportación de un chat.
type ExporteChat struct {
	Version    int                    `json:"version"`
	GeneradoTS int64                  `json:"generadoTs"`
	Chat       string                 `json:"chat"`
	Nombre     string                 `json:"nombre"`
	Grupo      bool                   `json:"grupo"`
	Mensajes   []MensajeChatExportado `json:"mensajes"`
	Recortado  bool                   `json:"recortado,omitempty"` // más de maxExportados mensajes: se exportaron los más recientes
}

// DatosDeChat reúne los mensajes del chat entre desde y hasta (0 = sin límite), del más viejo al más nuevo. El texto va descifrado: el
// archivo se cifra al guardarlo. Los archivos no van dentro, solo su tipo y tamaño.
func (a *Almacen) DatosDeChat(botID int64, chat string, desde, hasta int64, ahora time.Time) (*ExporteChat, error) {
	c, err := a.ObtenerChat(botID, chat)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, fmt.Errorf("ese chat no existe")
	}
	ex := &ExporteChat{Version: 1, GeneradoTS: ahora.Unix(), Chat: chat, Nombre: c.Mostrado, Grupo: c.EsGrupo, Mensajes: []MensajeChatExportado{}}
	cond, args, _ := a.condicionPersona(botID, FiltroPersona{Chat: chat, Desde: desde, Hasta: hasta})
	var curTS int64
	var curID string
	for {
		q := `SELECT id,ts FROM messages` + cond
		qa := append([]any{}, args...)
		if curTS > 0 {
			q += ` AND (ts<? OR (ts=? AND id<?))`
			qa = append(qa, curTS, curTS, curID)
		}
		q += ` ORDER BY ts DESC, id DESC LIMIT 500`
		fl, err := a.db.Query(q, qa...)
		if err != nil {
			return nil, err
		}
		var ids []filaLigera
		for fl.Next() {
			var r filaLigera
			if fl.Scan(&r.id, &r.ts) == nil {
				ids = append(ids, r)
			}
		}
		fl.Close()
		if len(ids) == 0 {
			break
		}
		curTS, curID = ids[len(ids)-1].ts, ids[len(ids)-1].id
		lista := make([]string, len(ids))
		for i, r := range ids {
			lista[i] = r.id
		}
		vs, err := a.MensajesPorID(botID, chat, lista)
		if err != nil {
			return nil, err
		}
		for _, v := range vs {
			if len(ex.Mensajes) >= maxExportados {
				ex.Recortado = true
				break
			}
			m := MensajeChatExportado{ID: v.ID, TS: v.TS, EsMio: v.EsMio, Tipo: v.Tipo, Texto: v.Texto, Eliminado: v.Borrado, Editado: v.Editado, Olvidado: v.Olvidado}
			m.De = v.Remitente
			if v.EsMio {
				m.De = "Yo"
			} else if m.De == "" || m.De == "Participante" {
				m.De = a.NombreDePersona(botID, v.RemitenteID)
				if m.De == "" {
					m.De = usuarioDe(v.RemitenteID)
				}
			}
			if v.Bytes > 0 {
				m.Archivo = fmt.Sprintf("%s, %d bytes", v.Tipo, v.Bytes)
				if v.Nombre != "" {
					m.Archivo = v.Nombre + " (" + m.Archivo + ")"
				}
			}
			ex.Mensajes = append(ex.Mensajes, m)
		}
		if ex.Recortado || len(ids) < 500 {
			break
		}
	}
	// se reunió de lo nuevo a lo viejo: se invierte para que quede cronológico
	for i, j := 0, len(ex.Mensajes)-1; i < j; i, j = i+1, j-1 {
		ex.Mensajes[i], ex.Mensajes[j] = ex.Mensajes[j], ex.Mensajes[i]
	}
	return ex, nil
}

// TextoDeExporte da el chat como texto simple, una línea por mensaje: "[2026-11-12 10:03] Nombre: texto".
func (a *Almacen) TextoDeExporte(ex *ExporteChat) string {
	var b strings.Builder
	loc := a.loc()
	fmt.Fprintf(&b, "Chat: %s\n\n", ex.Nombre)
	for _, m := range ex.Mensajes {
		t := m.Texto
		switch {
		case m.Olvidado:
			t = "[contenido no guardado]"
		case t == "" && m.Archivo != "":
			t = "[" + m.Archivo + "]"
		case t == "":
			t = "[" + m.Tipo + "]"
		}
		if m.Eliminado {
			t += " [eliminado]"
		}
		if m.Editado {
			t += " [editado]"
		}
		fmt.Fprintf(&b, "[%s] %s: %s\n", time.Unix(m.TS, 0).In(loc).Format("2006-01-02 15:04"), m.De, t)
	}
	return b.String()
}

// ---------- texto sin mayúsculas ni acentos ----------

// plegar pasa a minúsculas y quita acentos. Es el camino caliente de la búsqueda por palabra (se llama por cada mensaje), así que
// no usa tablas de Unicode: cubre las letras latinas acentuadas (español, portugués, francés…) y quita las marcas combinadas.
func plegar(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return strings.ToLower(s)
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 0x300 && r <= 0x36f: // marca combinada (acento suelto)
			continue
		case r < 0xc0 || r > 0x17f:
			b.WriteRune(r)
			continue
		}
		switch r {
		case 'à', 'á', 'â', 'ã', 'ä', 'å', 'ā', 'ă', 'ą':
			r = 'a'
		case 'ç', 'ć', 'č':
			r = 'c'
		case 'è', 'é', 'ê', 'ë', 'ē', 'ę', 'ě':
			r = 'e'
		case 'ì', 'í', 'î', 'ï', 'ī':
			r = 'i'
		case 'ñ', 'ń', 'ň':
			r = 'n'
		case 'ò', 'ó', 'ô', 'õ', 'ö', 'ø', 'ō':
			r = 'o'
		case 'ù', 'ú', 'û', 'ü', 'ū', 'ů':
			r = 'u'
		case 'ý', 'ÿ':
			r = 'y'
		case 'š', 'ś':
			r = 's'
		case 'ž', 'ź', 'ż':
			r = 'z'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// EscribirExporteChat guarda la exportación del chat (formato "json" o "txt") cifrada con BCF1 en exportes/ y devuelve la ruta relativa
// y cuántos mensajes llevó. Las exportaciones anteriores del mismo chat se reemplazan.
func (a *Almacen) EscribirExporteChat(botID int64, chat, formato string, desde, hasta int64, ahora time.Time) (string, int, error) {
	ex, err := a.DatosDeChat(botID, chat, desde, hasta, ahora)
	if err != nil {
		return "", 0, err
	}
	var cuerpo []byte
	ext := "json"
	if formato == "txt" {
		cuerpo, ext = []byte(a.TextoDeExporte(ex)), "txt"
	} else if cuerpo, err = json.Marshal(ex); err != nil {
		return "", 0, err
	}
	pre := "chat_" + HashChat(botID, chat) + "_"
	if abs, err := a.Abs(filepath.Join("exportes", fmt.Sprint(botID))); err == nil {
		_ = os.MkdirAll(abs, 0o700)
		if es, err := os.ReadDir(abs); err == nil {
			for _, e := range es {
				if strings.HasPrefix(e.Name(), pre) {
					_ = os.Remove(filepath.Join(abs, e.Name()))
				}
			}
		}
	}
	rel := filepath.Join("exportes", fmt.Sprint(botID), fmt.Sprintf("%s%d.%s.bcf", pre, ahora.Unix(), ext))
	if err := a.EscribirCifrado(rel, cuerpo); err != nil {
		return "", 0, err
	}
	a.LimpiarExportes(botID, 24, ahora)
	return rel, len(ex.Mensajes), nil
}
