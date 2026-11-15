package almacen

import (
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"time"
	_ "time/tzdata" // zonas horarias dentro del programa: no dependen del teléfono
)

// Estadísticas: solo cuentan datos sin cifrar (quién, cuándo, tipo y tamaño). Nunca leen el texto de los mensajes.
// Se actualizan en la misma transacción que guarda cada mensaje, así que nunca quedan a medias.

// QuienYo es el "remitente" de los mensajes enviados por el bot.
const QuienYo = "yo"

const upsertStats = `INSERT INTO daily_stats(bot_id,chat,sender,day,hora,tipo,count,bytes) VALUES(?,?,?,?,?,?,?,?)
	ON CONFLICT(bot_id,chat,sender,day,hora,tipo) DO UPDATE SET count=count+excluded.count, bytes=bytes+excluded.bytes`

// FijarZona establece la zona horaria (por ejemplo "America/Mexico_City") con la que se agrupan los mensajes por día.
// Si el nombre no existe se queda la anterior.
func (a *Almacen) FijarZona(id string) error {
	l, err := time.LoadLocation(id)
	if err != nil {
		return err
	}
	a.zona.Store(l)
	return nil
}

func (a *Almacen) loc() *time.Location {
	if l := a.zona.Load(); l != nil {
		return l
	}
	return time.Local
}

func (a *Almacen) diaHora(ts int64) (string, int) {
	t := time.Unix(ts, 0).In(a.loc())
	return t.Format("2006-01-02"), t.Hour()
}

func quienDe(m *Mensaje) string {
	switch {
	case m.EsMio:
		return QuienYo
	case m.RemitenteJID != "":
		return m.RemitenteJID
	}
	return m.ChatJID
}

func (a *Almacen) sumaStats(st *sql.Stmt, m *Mensaje) error {
	dia, hora := a.diaHora(m.TS)
	_, err := st.Exec(m.BotID, m.ChatJID, quienDe(m), dia, hora, m.Tipo, 1, m.ArchivoBytes)
	return err
}

// fusionaStats pasa las estadísticas de un chat a otro (cuando un LID y un teléfono resultan ser la misma persona).
func fusionaStats(tx *sql.Tx, botID int64, desde, hasta string) error {
	if _, err := tx.Exec(`INSERT INTO daily_stats(bot_id,chat,sender,day,hora,tipo,count,bytes)
		SELECT bot_id,?,sender,day,hora,tipo,count,bytes FROM daily_stats WHERE bot_id=? AND chat=? AND true
		ON CONFLICT(bot_id,chat,sender,day,hora,tipo) DO UPDATE SET count=count+excluded.count, bytes=bytes+excluded.bytes`,
		hasta, botID, desde); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM daily_stats WHERE bot_id=? AND chat=?`, botID, desde)
	return err
}

// RellenarEstadisticas cuenta los mensajes que ya existían antes de que hubiera estadísticas. Se hace una sola vez
// (al fijar la zona horaria) y solo con los mensajes anteriores a la migración; los nuevos ya se cuentan al guardarlos.
func (a *Almacen) RellenarEstadisticas() error {
	var hecho, tope int64
	_ = a.db.QueryRow(`SELECT valor FROM stats_info WHERE campo='relleno'`).Scan(&hecho)
	if hecho == 1 {
		return nil
	}
	_ = a.db.QueryRow(`SELECT valor FROM stats_info WHERE campo='rowid_max'`).Scan(&tope)
	type clave struct {
		bot                    int64
		chat, quien, dia, tipo string
		hora                   int
	}
	type suma struct{ n, bytes int64 }
	acum := map[clave]*suma{}
	for desde := int64(0); desde < tope; desde += 5000 {
		filas, err := a.db.Query(`SELECT bot_id,chat_jid,remitente_jid,es_mio,tipo,ts,archivo_bytes FROM messages WHERE rowid>? AND rowid<=?`, desde, min(desde+5000, tope))
		if err != nil {
			return err
		}
		for filas.Next() {
			var m Mensaje
			var mio int
			if err := filas.Scan(&m.BotID, &m.ChatJID, &m.RemitenteJID, &mio, &m.Tipo, &m.TS, &m.ArchivoBytes); err != nil {
				filas.Close()
				return err
			}
			m.EsMio = mio == 1
			dia, hora := a.diaHora(m.TS)
			k := clave{m.BotID, m.ChatJID, quienDe(&m), dia, m.Tipo, hora}
			s := acum[k]
			if s == nil {
				s = &suma{}
				acum[k] = s
			}
			s.n++
			s.bytes += m.ArchivoBytes
		}
		if err := filas.Err(); err != nil {
			filas.Close()
			return err
		}
		filas.Close()
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	st, err := tx.Prepare(upsertStats)
	if err != nil {
		return err
	}
	defer st.Close()
	for k, s := range acum {
		if _, err := st.Exec(k.bot, k.chat, k.quien, k.dia, k.hora, k.tipo, s.n, s.bytes); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE stats_info SET valor=1 WHERE campo='relleno'`); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	a.avisa("estadisticas")
	return nil
}

func (a *Almacen) rellenoListo() bool {
	var v int64
	_ = a.db.QueryRow(`SELECT valor FROM stats_info WHERE campo='relleno'`).Scan(&v)
	return v == 1
}

// primerTS es la hora del primer mensaje guardado (de un chat, o de todos si chat=""), o 0 si no hay.
func (a *Almacen) primerTS(botID int64, chat string) int64 {
	var ts sql.NullInt64
	if chat == "" {
		_ = a.db.QueryRow(`SELECT MIN(ts) FROM messages WHERE bot_id=?`, botID).Scan(&ts)
	} else {
		_ = a.db.QueryRow(`SELECT MIN(ts) FROM messages WHERE bot_id=? AND chat_jid=?`, botID, chat).Scan(&ts)
	}
	return ts.Int64
}

// TipoCuenta es lo enviado y recibido de un tipo de mensaje.
type TipoCuenta struct {
	Tipo      string `json:"tipo"`
	Enviados  int64  `json:"enviados"`
	Recibidos int64  `json:"recibidos"`
	Bytes     int64  `json:"bytes"`
}

// Quien es una persona de un grupo y cuántos mensajes escribió.
type Quien struct {
	Nombre   string `json:"nombre"`
	Cantidad int64  `json:"cantidad"`
}

// ResumenChat es lo que muestra la pestaña "Resumen" de un chat.
type ResumenChat struct {
	Desde      int64        `json:"desde"` // hora del primer mensaje guardado
	Listo      bool         `json:"listo"` // false mientras se cuentan los mensajes anteriores
	Enviados   int64        `json:"enviados"`
	Recibidos  int64        `json:"recibidos"`
	PorTipo    []TipoCuenta `json:"porTipo"`
	PorHora    []int64      `json:"porHora"`      // 24 valores
	PorSemana  []int64      `json:"porDiaSemana"` // 7 valores: lunes a domingo
	Quien      []Quien      `json:"quien"`        // solo grupos, los que más escriben
	Eliminados int64        `json:"eliminados"`   // mensajes eliminados por su autor o un admin (el contenido puede seguir guardado)
}

func semanaDe(dia string) int {
	t, err := time.Parse("2006-01-02", dia)
	if err != nil {
		return 0
	}
	return (int(t.Weekday()) + 6) % 7 // lunes = 0
}

// ResumenDeChat arma el resumen de un chat solo con los contadores.
func (a *Almacen) ResumenDeChat(botID int64, chat string) (*ResumenChat, error) {
	r := &ResumenChat{Eliminados: a.Eliminados(botID, chat), Desde: a.primerTS(botID, chat), Listo: a.rellenoListo(), PorHora: make([]int64, 24), PorSemana: make([]int64, 7), PorTipo: []TipoCuenta{}, Quien: []Quien{}}
	filas, err := a.db.Query(`SELECT sender,day,hora,tipo,count,bytes FROM daily_stats WHERE bot_id=? AND chat=?`, botID, chat)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	tipos := map[string]*TipoCuenta{}
	quien := map[string]int64{}
	for filas.Next() {
		var s, dia, tipo string
		var hora int
		var n, b int64
		if err := filas.Scan(&s, &dia, &hora, &tipo, &n, &b); err != nil {
			return nil, err
		}
		t := tipos[tipo]
		if t == nil {
			t = &TipoCuenta{Tipo: tipo}
			tipos[tipo] = t
		}
		t.Bytes += b
		if s == QuienYo {
			r.Enviados += n
			t.Enviados += n
		} else {
			r.Recibidos += n
			t.Recibidos += n
			quien[s] += n
		}
		if hora >= 0 && hora < 24 {
			r.PorHora[hora] += n
		}
		r.PorSemana[semanaDe(dia)] += n
	}
	if err := filas.Err(); err != nil {
		return nil, err
	}
	for _, t := range tipos {
		r.PorTipo = append(r.PorTipo, *t)
	}
	sort.Slice(r.PorTipo, func(i, j int) bool {
		x, y := r.PorTipo[i], r.PorTipo[j]
		if x.Enviados+x.Recibidos != y.Enviados+y.Recibidos {
			return x.Enviados+x.Recibidos > y.Enviados+y.Recibidos
		}
		return x.Tipo < y.Tipo
	})
	c, _ := a.ObtenerChat(botID, chat)
	if c != nil && c.EsGrupo {
		for jid, n := range quien {
			r.Quien = append(r.Quien, Quien{Nombre: a.nombreDe(botID, chat, jid), Cantidad: n})
		}
		sort.Slice(r.Quien, func(i, j int) bool {
			if r.Quien[i].Cantidad != r.Quien[j].Cantidad {
				return r.Quien[i].Cantidad > r.Quien[j].Cantidad
			}
			return r.Quien[i].Nombre < r.Quien[j].Nombre
		})
		if len(r.Quien) > 10 {
			r.Quien = r.Quien[:10]
		}
	}
	return r, nil
}

// nombreDe busca el nombre con el que se guardó el último mensaje de esa persona en el grupo.
func (a *Almacen) nombreDe(botID int64, chat, jid string) string {
	var n string
	indice := "" // en la ruta antigua se usa el índice de la 0.4.0 (recorre el chat), para poder comparar
	if cargaAntigua.Load() {
		indice = " INDEXED BY idx_mensajes_chat_ts"
	}
	_ = a.db.QueryRow(`SELECT remitente_nombre FROM messages`+indice+` WHERE bot_id=? AND chat_jid=? AND remitente_jid=? AND remitente_nombre<>'' AND remitente_nombre<>'Número no disponible' ORDER BY ts DESC LIMIT 1`,
		botID, chat, jid).Scan(&n)
	if n == "" {
		return "Participante"
	}
	return n
}

// DiaCuenta es la cantidad de mensajes de un día.
type DiaCuenta struct {
	Dia      string `json:"dia"` // AAAA-MM-DD
	Cantidad int64  `json:"cantidad"`
}

// Calendario son los días de un mes (AAAA-MM) con su cantidad de mensajes; los días sin mensajes no aparecen.
type Calendario struct {
	Desde int64       `json:"desde"`
	Listo bool        `json:"listo"`
	Dias  []DiaCuenta `json:"dias"`
}

// CalendarioMes cuenta los mensajes de cada día de un mes en un chat.
func (a *Almacen) CalendarioMes(botID int64, chat, mes string) (*Calendario, error) {
	out := &Calendario{Desde: a.primerTS(botID, chat), Listo: a.rellenoListo(), Dias: []DiaCuenta{}}
	if len(mes) != 7 {
		return out, nil
	}
	filas, err := a.db.Query(`SELECT day, SUM(count) FROM daily_stats WHERE bot_id=? AND chat=? AND day>=? AND day<? GROUP BY day ORDER BY day`,
		botID, chat, mes+"-01", mes+"-32")
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	for filas.Next() {
		var d DiaCuenta
		if err := filas.Scan(&d.Dia, &d.Cantidad); err != nil {
			return nil, err
		}
		out.Dias = append(out.Dias, d)
	}
	return out, filas.Err()
}

// MensajesDelDia devuelve los mensajes de un día (AAAA-MM-DD, en la zona del teléfono), del más viejo al más nuevo.
func (a *Almacen) MensajesDelDia(botID int64, chat, dia string) ([]MensajeVista, error) {
	t, err := time.ParseInLocation("2006-01-02", dia, a.loc())
	if err != nil {
		return nil, err
	}
	ini := t.Unix()
	fin := t.AddDate(0, 0, 1).Unix()
	return a.vistasDe("SELECT "+colsMensaje+" FROM messages WHERE bot_id=? AND chat_jid=? AND ts>=? AND ts<? ORDER BY ts, id LIMIT 1000",
		[]any{botID, chat, ini, fin})
}

// ChatActivo es un chat y cuántos mensajes tiene.
type ChatActivo struct {
	JID      string `json:"jid"`
	Nombre   string `json:"nombre"`
	Foto     string `json:"foto"`
	FotoTS   int64  `json:"fotoTs"`
	EsGrupo  bool   `json:"esGrupo"`
	Pais     string `json:"pais"`
	Cantidad int64  `json:"cantidad"`
}

// ResumenGlobal es lo que muestra la pantalla Estadísticas.
type ResumenGlobal struct {
	Desde     int64        `json:"desde"`
	Listo     bool         `json:"listo"`
	Enviados  int64        `json:"enviados"`
	Recibidos int64        `json:"recibidos"`
	Chats     int64        `json:"chats"`
	PorTipo   []TipoCuenta `json:"porTipo"`
	Activos   []ChatActivo `json:"activos"`
	PorDia    []DiaCuenta  `json:"porDia"` // los últimos `dias` días, incluidos los vacíos
}

// ResumenDeTodo arma las estadísticas de toda la cuenta; porDia cubre los últimos `dias` días (hasta hoy).
func (a *Almacen) ResumenDeTodo(botID int64, dias int) (*ResumenGlobal, error) {
	if dias <= 0 || dias > 366 {
		dias = 30
	}
	r := &ResumenGlobal{Desde: a.primerTS(botID, ""), Listo: a.rellenoListo(), PorTipo: []TipoCuenta{}, Activos: []ChatActivo{}, PorDia: []DiaCuenta{}}
	filas, err := a.db.Query(`SELECT tipo, SUM(CASE WHEN sender=? THEN count ELSE 0 END), SUM(CASE WHEN sender<>? THEN count ELSE 0 END), SUM(bytes)
		FROM daily_stats WHERE bot_id=? GROUP BY tipo`, QuienYo, QuienYo, botID)
	if err != nil {
		return nil, err
	}
	for filas.Next() {
		var t TipoCuenta
		if err := filas.Scan(&t.Tipo, &t.Enviados, &t.Recibidos, &t.Bytes); err != nil {
			filas.Close()
			return nil, err
		}
		r.Enviados += t.Enviados
		r.Recibidos += t.Recibidos
		r.PorTipo = append(r.PorTipo, t)
	}
	filas.Close()
	sort.Slice(r.PorTipo, func(i, j int) bool {
		x, y := r.PorTipo[i], r.PorTipo[j]
		if x.Enviados+x.Recibidos != y.Enviados+y.Recibidos {
			return x.Enviados+x.Recibidos > y.Enviados+y.Recibidos
		}
		return x.Tipo < y.Tipo
	})
	_ = a.db.QueryRow(`SELECT COUNT(DISTINCT chat) FROM daily_stats WHERE bot_id=?`, botID).Scan(&r.Chats)

	f2, err := a.db.Query(`SELECT c.jid,c.nombre_mostrado,c.foto_ruta,c.foto_ts,c.es_grupo,c.pais_iso,SUM(s.count) AS n
		FROM daily_stats s JOIN chats c ON c.bot_id=s.bot_id AND c.jid=s.chat WHERE s.bot_id=? GROUP BY c.jid ORDER BY n DESC, c.jid LIMIT 10`, botID)
	if err != nil {
		return nil, err
	}
	for f2.Next() {
		var c ChatActivo
		var g int
		if err := f2.Scan(&c.JID, &c.Nombre, &c.Foto, &c.FotoTS, &g, &c.Pais, &c.Cantidad); err != nil {
			f2.Close()
			return nil, err
		}
		c.EsGrupo = g == 1
		r.Activos = append(r.Activos, c)
	}
	f2.Close()

	hoy := time.Now().In(a.loc())
	ini := hoy.AddDate(0, 0, -(dias - 1)).Format("2006-01-02")
	f3, err := a.db.Query(`SELECT day, SUM(count) FROM daily_stats WHERE bot_id=? AND day>=? GROUP BY day`, botID, ini)
	if err != nil {
		return nil, err
	}
	defer f3.Close()
	por := map[string]int64{}
	for f3.Next() {
		var d string
		var n int64
		if err := f3.Scan(&d, &n); err != nil {
			return nil, err
		}
		por[d] = n
	}
	for i := dias - 1; i >= 0; i-- {
		d := hoy.AddDate(0, 0, -i).Format("2006-01-02")
		r.PorDia = append(r.PorDia, DiaCuenta{Dia: d, Cantidad: por[d]})
	}
	return r, f3.Err()
}

// ---------- contactos y ubicaciones ----------

// ContactoVista es el contenido de un mensaje de contacto, guardado como JSON en el texto cifrado.
type ContactoVista struct {
	V        int          `json:"v"`
	Contacts []ContactoUn `json:"contactos"`
}

// ContactoUn es una persona compartida con sus teléfonos.
type ContactoUn struct {
	Nombre string   `json:"nombre"`
	Tels   []string `json:"tels"`
}

// UbicacionVista es el contenido de un mensaje de ubicación.
type UbicacionVista struct {
	V      int     `json:"v"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Nombre string  `json:"nombre,omitempty"`
	Dir    string  `json:"dir,omitempty"`
	Vivo   bool    `json:"vivo,omitempty"`
}

// VistaPrevia es el texto corto de un mensaje para la lista de chats (los contactos y ubicaciones guardan JSON).
func VistaPrevia(tipo, texto string) string {
	switch tipo {
	case TipoContacto:
		var c ContactoVista
		if json.Unmarshal([]byte(texto), &c) == nil && c.V > 0 {
			nombres := make([]string, 0, len(c.Contacts))
			for _, x := range c.Contacts {
				nombres = append(nombres, x.Nombre)
			}
			return "Contacto: " + strings.Join(nombres, ", ")
		}
	case TipoUbicacion:
		var u UbicacionVista
		if json.Unmarshal([]byte(texto), &u) == nil && u.V > 0 {
			switch {
			case u.Vivo:
				return "Ubicación en tiempo real"
			case u.Nombre != "":
				return "Ubicación: " + u.Nombre
			}
			return "Ubicación"
		}
	}
	return texto
}
