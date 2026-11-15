package almacen

import (
	"errors"
	"time"

	"botchat/internal/cifrado"
)

// DescargaVideo es una fila del historial de descargas de video. Nunca guarda el archivo: el video se borra al enviarse y desde aquí se
// puede volver a pedir con el mismo enlace.
type DescargaVideo struct {
	ID       int64  `json:"id"`
	TS       int64  `json:"ts"`
	Persona  string `json:"persona"`
	Nombre   string `json:"nombre"`
	Chat     string `json:"chat"`
	EsGrupo  bool   `json:"esGrupo"`
	URL      string `json:"url"`
	Titulo   string `json:"titulo"`
	Subidor  string `json:"subidor"`
	Mini     []byte `json:"-"`
	Sitio    string `json:"sitio"`
	Duracion int    `json:"duracion"`
	Peso     int64  `json:"peso"`
	Formato  string `json:"formato"` // mp4, mp3 o m4a
	Calidad  string `json:"calidad"` // "720p", "128 kbps"…
	Partes   int    `json:"partes"`
	Entrega  string `json:"entrega"` // directo, zip o partes
	Estado   string `json:"estado"`  // en_curso, enviado, fallido
	Error    string `json:"error"`
}

const colDescargaVideo = "descarga_video"

func (a *Almacen) sellaDV(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	s, err := cifrado.SellarTexto(a.clave, colDescargaVideo, b)
	if err != nil {
		return nil
	}
	return s
}

func (a *Almacen) abreDV(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	t, err := cifrado.AbrirTexto(a.clave, colDescargaVideo, b)
	if err != nil {
		return nil
	}
	return t
}

// NuevaDescargaVideo agrega una fila (estado "en_curso") y devuelve su id.
func (a *Almacen) NuevaDescargaVideo(botID int64, d DescargaVideo) (int64, error) {
	if d.TS == 0 {
		d.TS = time.Now().Unix()
	}
	if d.Estado == "" {
		d.Estado = "en_curso"
	}
	r, err := a.db.Exec(`INSERT INTO descargas_video(bot_id,ts,persona,chat,es_grupo,nombre_cifrado,url_cifrado,titulo_cifrado,subidor_cifrado,mini_cifrada,
		extractor,duracion,peso,formato,calidad,partes,entrega,estado,error) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		botID, d.TS, d.Persona, d.Chat, b2i(d.EsGrupo), a.sellaDV([]byte(d.Nombre)), a.sellaDV([]byte(d.URL)), a.sellaDV([]byte(d.Titulo)),
		a.sellaDV([]byte(d.Subidor)), a.sellaDV(d.Mini), d.Sitio, d.Duracion, d.Peso, d.Formato, d.Calidad, maxInt(d.Partes, 1), d.Entrega, d.Estado, d.Error)
	if err != nil {
		return 0, err
	}
	id, _ := r.LastInsertId()
	a.avisa("descargas_video")
	return id, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// CerrarDescargaVideo anota cómo terminó una descarga (estado "enviado" o "fallido") y lo que se supo al final.
func (a *Almacen) CerrarDescargaVideo(botID, id int64, estado, errTxt string, peso int64, partes int, entrega string) error {
	_, err := a.db.Exec(`UPDATE descargas_video SET estado=?, error=?, peso=CASE WHEN ?>0 THEN ? ELSE peso END, partes=CASE WHEN ?>0 THEN ? ELSE partes END,
		entrega=CASE WHEN ?<>'' THEN ? ELSE entrega END WHERE bot_id=? AND id=?`, estado, errTxt, peso, peso, partes, partes, entrega, entrega, botID, id)
	if err == nil {
		a.avisa("descargas_video")
	}
	return err
}

const selDV = `SELECT id,ts,persona,chat,es_grupo,nombre_cifrado,url_cifrado,titulo_cifrado,subidor_cifrado,extractor,duracion,peso,formato,calidad,partes,entrega,estado,error FROM descargas_video`

func (a *Almacen) leeDV(r escaneable) (DescargaVideo, error) {
	var d DescargaVideo
	var g int
	var nom, url, tit, sub []byte
	if err := r.Scan(&d.ID, &d.TS, &d.Persona, &d.Chat, &g, &nom, &url, &tit, &sub, &d.Sitio, &d.Duracion, &d.Peso, &d.Formato, &d.Calidad, &d.Partes, &d.Entrega, &d.Estado, &d.Error); err != nil {
		return d, err
	}
	d.EsGrupo = g == 1
	d.Nombre, d.URL, d.Titulo, d.Subidor = string(a.abreDV(nom)), string(a.abreDV(url)), string(a.abreDV(tit)), string(a.abreDV(sub))
	return d, nil
}

// ListarDescargasVideo devuelve el historial, lo más nuevo primero. antesID > 0 pide la página siguiente.
func (a *Almacen) ListarDescargasVideo(botID int64, limite int, antesID int64) ([]DescargaVideo, error) {
	if limite <= 0 || limite > 200 {
		limite = 50
	}
	q, args := selDV+` WHERE bot_id=?`, []any{botID}
	if antesID > 0 {
		q += ` AND id<?`
		args = append(args, antesID)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limite)
	rows, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DescargaVideo
	for rows.Next() {
		d, err := a.leeDV(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DescargaVideoPorID devuelve una fila.
func (a *Almacen) DescargaVideoPorID(botID, id int64) (*DescargaVideo, error) {
	d, err := a.leeDV(a.db.QueryRow(selDV+` WHERE bot_id=? AND id=?`, botID, id))
	if err != nil {
		return nil, errors.New("esa descarga ya no está en el historial")
	}
	return &d, nil
}

// MiniaturaDescargaVideo devuelve la miniatura (JPEG) de una fila, o nil.
func (a *Almacen) MiniaturaDescargaVideo(botID, id int64) []byte {
	var b []byte
	if err := a.db.QueryRow(`SELECT mini_cifrada FROM descargas_video WHERE bot_id=? AND id=?`, botID, id).Scan(&b); err != nil {
		return nil
	}
	return a.abreDV(b)
}

// BorrarDescargaVideo quita una fila del historial (id 0 = todas las terminadas).
func (a *Almacen) BorrarDescargaVideo(botID, id int64) error {
	var err error
	if id == 0 {
		_, err = a.db.Exec(`DELETE FROM descargas_video WHERE bot_id=? AND estado<>'en_curso'`, botID)
	} else {
		_, err = a.db.Exec(`DELETE FROM descargas_video WHERE bot_id=? AND id=?`, botID, id)
	}
	if err == nil {
		a.avisa("descargas_video")
	}
	return err
}

// ContarDescargasVideo cuenta las descargas pedidas por una persona desde un momento (para los topes por persona). persona "" = de todos.
func (a *Almacen) ContarDescargasVideo(botID int64, persona string, desdeTS int64) int {
	var n int
	if persona == "" {
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM descargas_video WHERE bot_id=? AND ts>=?`, botID, desdeTS).Scan(&n)
	} else {
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM descargas_video WHERE bot_id=? AND persona=? AND ts>=?`, botID, persona, desdeTS).Scan(&n)
	}
	return n
}

// MarcarDescargasVideoHuerfanas pasa a "fallido" lo que quedó "en_curso" (por ejemplo si la app se cerró a media descarga).
func (a *Almacen) MarcarDescargasVideoHuerfanas(botID int64) {
	if _, err := a.db.Exec(`UPDATE descargas_video SET estado='fallido', error='Se interrumpió porque la app se cerró' WHERE bot_id=? AND estado='en_curso'`, botID); err == nil {
		a.avisa("descargas_video")
	}
}

// FormatoDescargaVideo corrige el formato anotado (por ejemplo si el mp3 no se pudo hacer y salió m4a).
func (a *Almacen) FormatoDescargaVideo(botID, id int64, formato string) {
	if _, err := a.db.Exec(`UPDATE descargas_video SET formato=? WHERE bot_id=? AND id=?`, formato, botID, id); err == nil {
		a.avisa("descargas_video")
	}
}
