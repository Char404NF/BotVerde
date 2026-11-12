package almacen

import (
	"fmt"
	"sort"
	"strings"

	"botchat/internal/identidad"
)

// Categorías de archivos para filtrar y sumar.
const (
	CatFoto      = "foto"
	CatVideo     = "video"
	CatAudio     = "audio"
	CatDocumento = "documento"
)

// Categorias son todas, en el orden en que se muestran.
var Categorias = []string{CatFoto, CatVideo, CatAudio, CatDocumento}

// CategoriaDe dice a qué categoría pertenece un tipo de mensaje ("" si no es multimedia).
func CategoriaDe(tipo string) string {
	switch tipo {
	case TipoImagen, TipoSticker:
		return CatFoto
	case TipoVideo:
		return CatVideo
	case TipoAudio:
		return CatAudio
	case TipoDocumento:
		return CatDocumento
	}
	return ""
}

func tiposDe(cat string) []string {
	switch cat {
	case CatFoto:
		return []string{TipoImagen, TipoSticker}
	case CatVideo:
		return []string{TipoVideo}
	case CatAudio:
		return []string{TipoAudio}
	case CatDocumento:
		return []string{TipoDocumento}
	}
	return nil
}

// ChatLista es una fila de la lista de chats.
type ChatLista struct {
	JID           string  `json:"jid"` // clave interna: la app nunca la muestra
	Nombre        string  `json:"nombre"`
	Telefono      string  `json:"telefono"`
	Pais          string  `json:"pais"`
	EsGrupo       bool    `json:"esGrupo"`
	Foto          string  `json:"foto"`
	FotoTS        int64   `json:"fotoTs"`
	UltimoTS      int64   `json:"ultimoTs"`
	UltimoTexto   string  `json:"ultimoTexto"`
	UltimoTipo    string  `json:"ultimoTipo"`
	UltimoBorrado bool    `json:"ultimoBorrado"`
	UltimoMio     bool    `json:"ultimoMio"`
	EstadoTrabajo string  `json:"estadoTrabajo"` // "", "pendiente" o "resuelto"
	Etiquetas     []int64 `json:"etiquetas"`     // ids de las etiquetas del chat
	Fijado        bool    `json:"fijado"`        // fijado arriba (local)
}

// ListarChats devuelve los chats de personas o de grupos, del más reciente al más viejo.
// consulta busca por nombre (sin acentos ni mayúsculas) y por número (solo dígitos).
func (a *Almacen) ListarChats(botID int64, grupos bool, consulta string, limite, desde int) ([]ChatLista, error) {
	if limite <= 0 || limite > 500 {
		limite = 100
	}
	extra, args := "", []any(nil)
	if t := strings.TrimSpace(consulta); t != "" {
		extra = ` AND c.busqueda LIKE ? ESCAPE '\'`
		args = append(args, "%"+escapaLike(identidad.Normalizar(t))+"%")
	}
	return a.listaChats(extra, args, botID, b2i(grupos), limite, desde, false)
}

// listaChats arma la consulta de chats con su último mensaje. Con ignoraTipo se devuelven personas y grupos por igual.
func (a *Almacen) listaChats(extra string, extraArgs []any, botID int64, esGrupo, limite, desde int, ignoraTipo bool) ([]ChatLista, error) {
	q := `SELECT c.jid,c.nombre_mostrado,c.telefono_mostrado,c.pais_iso,c.es_grupo,c.foto_ruta,c.foto_ts,c.ultimo_ts,
		m.tipo,m.texto_cifrado,m.borrado,m.es_mio,COALESCE(e.estado_trabajo,''),COALESCE(e.fijado,0)
		FROM chats c LEFT JOIN messages m ON m.rowid = (
			SELECT rowid FROM messages WHERE bot_id=c.bot_id AND chat_jid=c.jid ORDER BY ts DESC, id DESC LIMIT 1)
		LEFT JOIN chat_estado e ON e.bot_id=c.bot_id AND e.chat_jid=c.jid
		WHERE c.bot_id=?`
	args := []any{botID}
	if !ignoraTipo {
		q += ` AND c.es_grupo=?`
		args = append(args, esGrupo)
	}
	q += extra
	args = append(args, extraArgs...)
	// Los chats sin mensajes (solo creados por nombres de contacto) no se listan.
	q += ` AND m.id IS NOT NULL ORDER BY COALESCE(e.fijado,0) DESC, c.ultimo_ts DESC, c.jid LIMIT ? OFFSET ?`
	args = append(args, limite, desde)
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []ChatLista{}
	for filas.Next() {
		var c ChatLista
		var g, borrado, mio, fij int
		var tipo *string
		var texto []byte
		if err := filas.Scan(&c.JID, &c.Nombre, &c.Telefono, &c.Pais, &g, &c.Foto, &c.FotoTS, &c.UltimoTS, &tipo, &texto, &borrado, &mio, &c.EstadoTrabajo, &fij); err != nil {
			return nil, err
		}
		c.EsGrupo, c.UltimoBorrado, c.UltimoMio, c.Fijado = g == 1, borrado == 1, mio == 1, fij == 1
		if tipo != nil {
			c.UltimoTipo = *tipo
		}
		if c.UltimoTipo != TipoVerUnaVez && !c.UltimoBorrado {
			c.UltimoTexto = VistaPrevia(c.UltimoTipo, a.abreTexto("texto", texto))
		}
		if c.UltimoTipo == TipoVerUnaVez {
			c.UltimoTexto = TextoVerUnaVez
		}
		out = append(out, c)
	}
	if err := filas.Err(); err != nil {
		return nil, err
	}
	filas.Close()
	if len(out) > 0 {
		jids := make([]string, len(out))
		for i := range out {
			jids[i] = out[i].JID
		}
		et := a.etiquetasPorChat(botID, jids)
		for i := range out {
			out[i].Etiquetas = et[out[i].JID]
			if out[i].Etiquetas == nil {
				out[i].Etiquetas = []int64{}
			}
		}
	}
	return out, nil
}

// ChatPorJID devuelve un chat con su último mensaje (o nil si no existe), con el mismo formato que la lista.
func (a *Almacen) ChatPorJID(botID int64, jid string) (*ChatLista, error) {
	filas, err := a.listaChats(` AND c.jid=?`, []any{jid}, botID, b2i(false), 1, 0, true)
	if err != nil || len(filas) == 0 {
		return nil, err
	}
	return &filas[0], nil
}

func escapaLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// MensajeVista es un mensaje listo para mostrar.
type MensajeVista struct {
	ID            string `json:"id"`
	Chat          string `json:"chat"`
	RemitenteID   string `json:"remitenteId"` // clave interna para agrupar y dar color; la app no la muestra
	Remitente     string `json:"remitente"`
	EsMio         bool   `json:"esMio"`
	Tipo          string `json:"tipo"`
	Texto         string `json:"texto"`
	TS            int64  `json:"ts"`
	Borrado       bool   `json:"borrado"`
	Editado       bool   `json:"editado"`
	Estado        string `json:"estado"` // estado del archivo
	Bytes         int64  `json:"bytes"`
	Mime          string `json:"mime"`
	Nombre        string `json:"nombre"` // nombre del archivo (documentos)
	Ruta          string `json:"ruta"`
	Miniatura     string `json:"miniatura"`
	Portada       string `json:"portada"`       // portada del video (se conserva aunque se borre el video)
	BorradoTS     int64  `json:"borradoTs"`     // cuándo se borró el archivo (0 = no)
	BorradoMotivo string `json:"borradoMotivo"` // limpieza | tope | manual | retención
	Ancho         int    `json:"ancho"`
	Alto          int    `json:"alto"`
	DuracionS     int    `json:"dur"`
	NotaVoz       bool   `json:"notaVoz"`
	PuedeBajarse  bool   `json:"puedeBajarse"`
	EliminadoEn   int64  `json:"eliminadoEn"`
	EliminadoPor  string `json:"eliminadoPor"` // nombre de quien lo eliminó, solo si no fue su autor

	EstadoEnvio  string          `json:"estadoEnvio"` // solo mensajes propios: enviando, enviado, entregado, leido o fallido
	Entregados   int             `json:"entregados"`
	Leidos       int             `json:"leidos"`
	CitaID       string          `json:"citaId"`
	CitaAutor    string          `json:"citaAutor"` // nombre de quien escribió lo citado ("yo" si fui yo)
	CitaTexto    string          `json:"citaTexto"`
	Menciones    []MencionVista  `json:"menciones"`
	Reenviado    bool            `json:"reenviado"`
	Reacciones   []ReaccionVista `json:"reacciones"`
	VerEstado    string          `json:"verEstado"`
	VerTipo      string          `json:"verTipo"`
	VerExpira    int64           `json:"verExpira"`
	Automatico   string          `json:"automatico"`   // mensaje mío que mandó una automatización ("" = lo escribí yo)
	Olvidado     bool            `json:"olvidado"`     // se guardó sin texto (auditoría apagada o retención): la pantalla dice "contenido no guardado"
	DelHistorial bool            `json:"delHistorial"` // llegó en la sincronización de historial

	bot int64
}

// MencionVista es alguien mencionado con @ en el texto: Usuario es lo que aparece tras la @ en el texto.
type MencionVista struct {
	Usuario string `json:"usuario"`
	Nombre  string `json:"nombre"`
}

// vista arma lo que ve la pantalla. Con `nom` los nombres ya vienen resueltos por lote; con nil se piden uno por uno (ruta antigua).
func (a *Almacen) vista(m *Mensaje, nom *nombresPagina) MensajeVista {
	v := MensajeVista{
		ID: m.ID, Chat: m.ChatJID, RemitenteID: m.RemitenteJID, Remitente: m.RemitenteNombre, EsMio: m.EsMio, Tipo: m.Tipo, Texto: m.Texto,
		TS: m.TS, Borrado: m.Borrado, Editado: m.Editado, Estado: m.ArchivoEstado, Bytes: m.ArchivoBytes, Mime: m.ArchivoMime,
		Nombre: m.ArchivoNombre, Ruta: m.ArchivoRuta, Miniatura: m.MiniaturaRuta, Portada: m.PortadaRuta, BorradoTS: m.ArchivoBorradoTS, BorradoMotivo: m.ArchivoBorradoMotivo, Ancho: m.Ancho, Alto: m.Alto, DuracionS: m.DuracionS, NotaVoz: m.NotaVoz,
	}
	v.EliminadoEn = m.EliminadoEn
	if m.EliminadoPor != "" && m.EliminadoPor != m.RemitenteJID {
		if nom != nil {
			v.EliminadoPor = nom.deMensajes(m.BotID, m.ChatJID, m.EliminadoPor)
		} else {
			v.EliminadoPor = a.nombreDe(m.BotID, m.ChatJID, m.EliminadoPor)
		}
	}
	v.bot = m.BotID
	v.CitaID, v.CitaTexto, v.Reenviado = m.CitaID, m.CitaTexto, m.Reenviado
	if m.CitaID != "" {
		if nom != nil {
			v.CitaAutor = nom.mostrado(m.BotID, m.ChatJID, m.CitaAutor)
		} else {
			v.CitaAutor = a.nombreParaMostrar(m.BotID, m.ChatJID, m.CitaAutor)
		}
		if m.CitaAutor == QuienYo {
			v.CitaAutor = QuienYo
		}
	}
	v.Menciones = []MencionVista{}
	for _, j := range m.Menciones {
		var nombre string
		if nom != nil {
			nombre = nom.mostrado(m.BotID, m.ChatJID, j)
		} else {
			nombre = a.nombreParaMostrar(m.BotID, m.ChatJID, j)
		}
		v.Menciones = append(v.Menciones, MencionVista{Usuario: usuarioDe(j), Nombre: nombre})
	}
	v.VerEstado, v.VerTipo, v.VerExpira = m.VerEstado, m.VerTipo, m.VerExpira
	v.Automatico = m.Automatico
	v.Olvidado, v.DelHistorial = m.Olvidado, m.DelHistorial
	v.EstadoEnvio = estadoDe(m.EstadoEnvio, m.EsMio, 0, 0)
	v.Reacciones = []ReaccionVista{}
	v.PuedeBajarse = CategoriaDe(m.Tipo) != "" && len(m.ArchivoInfo) > 0 &&
		(m.ArchivoEstado == EstadoNinguno || m.ArchivoEstado == EstadoOmitido || m.ArchivoEstado == EstadoFallido)
	return v
}

// ListarMensajes devuelve hasta `limite` mensajes de un chat, del más nuevo al más viejo.
// Para la página siguiente se pasa la hora y el id del último mensaje recibido (antesTS, antesID); con antesTS=0 empieza por los últimos.
func (a *Almacen) ListarMensajes(botID int64, chat string, antesTS int64, antesID string, limite int) ([]MensajeVista, error) {
	if limite <= 0 || limite > 500 {
		limite = 50
	}
	q := "SELECT " + colsMensaje + " FROM messages WHERE bot_id=? AND chat_jid=?"
	args := []any{botID, chat}
	if antesTS > 0 {
		q += " AND (ts < ? OR (ts = ? AND id < ?))"
		args = append(args, antesTS, antesTS, antesID)
	}
	q += " ORDER BY ts DESC, id DESC LIMIT ?"
	args = append(args, limite)
	return a.vistasDe(q, args)
}

// usuarioDe devuelve la parte de un JID antes de la @ (y antes de los dos puntos del dispositivo).
func usuarioDe(j string) string {
	if i := strings.IndexByte(j, '@'); i >= 0 {
		j = j[:i]
	}
	if i := strings.IndexByte(j, ':'); i >= 0 {
		j = j[:i]
	}
	return j
}

// enriquecer agrega a una página de mensajes sus acuses de recibo y sus reacciones (una consulta por chat).
func (a *Almacen) enriquecer(vs []MensajeVista) {
	porChat := map[string][]int{}
	for i := range vs {
		k := fmt.Sprintf("%d\x00%s", vs[i].bot, vs[i].Chat)
		porChat[k] = append(porChat[k], i)
	}
	for _, idx := range porChat {
		bot, chat := vs[idx[0]].bot, vs[idx[0]].Chat
		ids := make([]string, 0, len(idx))
		propios := make([]string, 0, len(idx))
		pos := map[string]int{}
		for _, i := range idx {
			ids = append(ids, vs[i].ID)
			pos[vs[i].ID] = i
			if vs[i].EsMio {
				propios = append(propios, vs[i].ID)
			}
		}
		if rs, err := a.reaccionesDe(bot, chat, ids); err == nil {
			for id, r := range rs {
				vs[pos[id]].Reacciones = r
			}
		}
		if len(propios) == 0 {
			continue
		}
		args := []any{bot, chat}
		for _, id := range propios {
			args = append(args, id)
		}
		filas, err := a.db.Query(`SELECT msg_id, tipo, COUNT(*) FROM message_receipts WHERE bot_id=? AND chat_jid=? AND msg_id IN (`+marcas(len(propios))+`) GROUP BY msg_id, tipo`, args...)
		if err != nil {
			continue
		}
		for filas.Next() {
			var id, tipo string
			var n int
			if filas.Scan(&id, &tipo, &n) != nil {
				continue
			}
			v := &vs[pos[id]]
			switch tipo {
			case AcuseEntregado:
				v.Entregados = n
			case AcuseLeido:
				v.Leidos = n
			}
		}
		filas.Close()
		for _, id := range propios {
			v := &vs[pos[id]]
			v.EstadoEnvio = estadoDe(v.EstadoEnvio, true, v.Entregados, v.Leidos)
		}
	}
}

func (a *Almacen) vistasDe(q string, args []any) ([]MensajeVista, error) {
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	ms := []*Mensaje{}
	for filas.Next() {
		m, err := a.escanearMensaje(filas)
		if err != nil {
			return nil, err
		}
		ms = append(ms, m)
	}
	if err := filas.Err(); err != nil {
		return nil, err
	}
	return a.vistasDeMensajes(ms), nil
}

// vistasDeMensajes convierte una página de mensajes en vistas: nombres por lote, y reacciones y acuses por lote.
func (a *Almacen) vistasDeMensajes(ms []*Mensaje) []MensajeVista {
	var nom *nombresPagina
	if !cargaAntigua.Load() {
		nom = a.resolverNombres(ms)
	}
	out := make([]MensajeVista, 0, len(ms))
	for _, m := range ms {
		out = append(out, a.vista(m, nom))
	}
	a.enriquecer(out)
	return out
}

// MensajesPorID devuelve los mensajes pedidos (en el orden de la lista: del más nuevo al más viejo) con una consulta por
// trozo de hasta 300 ids; los que ya no existen simplemente no aparecen.
func (a *Almacen) MensajesPorID(botID int64, chat string, ids []string) ([]MensajeVista, error) {
	ms := []*Mensaje{}
	for i := 0; i < len(ids); i += 300 {
		trozo := ids[i:min(i+300, len(ids))]
		args := []any{botID, chat}
		for _, id := range trozo {
			args = append(args, id)
		}
		filas, err := a.db.Query("SELECT "+colsMensaje+" FROM messages WHERE bot_id=? AND chat_jid=? AND id IN ("+marcas(len(trozo))+")", args...)
		if err != nil {
			return nil, err
		}
		for filas.Next() {
			m, err := a.escanearMensaje(filas)
			if err != nil {
				filas.Close()
				return nil, err
			}
			ms = append(ms, m)
		}
		filas.Close()
	}
	sort.Slice(ms, func(i, j int) bool {
		if ms[i].TS != ms[j].TS {
			return ms[i].TS > ms[j].TS
		}
		return ms[i].ID > ms[j].ID
	})
	return a.vistasDeMensajes(ms), nil
}

// ListarArchivos devuelve los mensajes con multimedia de un chat (o de todos si chat=""), del más nuevo al más viejo.
// categoria: foto, video, audio, documento o "" para todas. soloDescargados limita a los que ya están en el teléfono;
// si es falso se incluyen también los que esperan un botón "Descargar" (pero no los "ver una vez").
func (a *Almacen) ListarArchivos(botID int64, chat, categoria string, soloDescargados bool, limite, desde int) ([]MensajeVista, error) {
	if limite <= 0 || limite > 1000 {
		limite = 100
	}
	tipos := tiposMedia(categoria)
	q := "SELECT " + colsMensaje + " FROM messages WHERE bot_id=? AND tipo IN (" + marcas(len(tipos)) + ")"
	args := []any{botID}
	for _, t := range tipos {
		args = append(args, t)
	}
	if chat != "" {
		q += " AND chat_jid=?"
		args = append(args, chat)
	}
	if soloDescargados {
		q += " AND archivo_estado='descargado'"
	}
	q += " ORDER BY ts DESC, id DESC LIMIT ? OFFSET ?"
	args = append(args, limite, desde)
	return a.vistasDe(q, args)
}

func tiposMedia(cat string) []string {
	if t := tiposDe(cat); t != nil {
		return t
	}
	return []string{TipoImagen, TipoSticker, TipoVideo, TipoAudio, TipoDocumento}
}

func marcas(n int) string {
	if n <= 0 {
		return "NULL"
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func (a *Almacen) abreTexto(col string, b []byte) string {
	if len(b) == 0 {
		return ""
	}
	t, err := a.abre(col, b)
	if err != nil {
		return ""
	}
	return t
}

func (a *Almacen) abre(col string, b []byte) (string, error) {
	return abrirTextoCol(a.clave, col, b)
}
