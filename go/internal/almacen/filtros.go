package almacen

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// FiltroLista describe qué mensajes de un chat se quieren ver. Solo usa datos sin cifrar (tipo, remitente, hora, eliminado),
// así que filtrar y contar no descifra nada; el texto se descifra únicamente de la página que se muestra.
type FiltroLista struct {
	Tipos      []string `json:"tipos"`      // vacío = todos; si no: imagen, sticker, video, audio, documento, texto, contacto, ubicacion, encuesta, ver_una_vez, otro
	Direccion  string   `json:"direccion"`  // "", "enviados" o "recibidos"
	Persona    string   `json:"persona"`    // JID del remitente (grupos)
	Eliminados bool     `json:"eliminados"` // solo los eliminados
	Hora       *int     `json:"hora"`       // 0-23 en la zona del teléfono
	Semana     *int     `json:"semana"`     // 0 = lunes … 6 = domingo
	Dia        string   `json:"dia"`        // AAAA-MM-DD
}

// ParseFiltro lee el filtro de su JSON; un texto vacío es "sin filtro".
func ParseFiltro(s string) (FiltroLista, error) {
	var f FiltroLista
	if strings.TrimSpace(s) == "" {
		return f, nil
	}
	err := json.Unmarshal([]byte(s), &f)
	return f, err
}

// Pagina es una página de resultados, con el total solo en la primera página.
type Pagina struct {
	Total int64          `json:"total"`
	Items []MensajeVista `json:"items"`
}

type filaLigera struct {
	id string
	ts int64
}

// ListarFiltrados devuelve hasta `limite` mensajes del chat que cumplen el filtro, del más nuevo al más viejo.
// Con antesTS=0 es la primera página y además trae el total; después se pasan hora e id del último recibido.
func (a *Almacen) ListarFiltrados(botID int64, chat string, f FiltroLista, antesTS int64, antesID string, limite int) (*Pagina, error) {
	if limite <= 0 || limite > 500 {
		limite = 60
	}
	q := `SELECT id, ts FROM messages WHERE bot_id=? AND chat_jid=?`
	args := []any{botID, chat}
	if len(f.Tipos) > 0 {
		q += ` AND tipo IN (` + marcas(len(f.Tipos)) + `)`
		for _, t := range f.Tipos {
			args = append(args, t)
		}
	}
	switch f.Direccion {
	case "enviados":
		q += ` AND es_mio=1`
	case "recibidos":
		q += ` AND es_mio=0`
	}
	if f.Persona != "" {
		q += ` AND es_mio=0 AND remitente_jid=?`
		args = append(args, f.Persona)
	}
	if f.Eliminados {
		q += ` AND borrado=1`
	}
	var diaIni, diaFin int64 = 0, 0
	if f.Dia != "" {
		t, err := time.ParseInLocation("2006-01-02", f.Dia, a.loc())
		if err != nil {
			return nil, err
		}
		diaIni, diaFin = t.Unix(), t.AddDate(0, 0, 1).Unix()
		q += ` AND ts>=? AND ts<?`
		args = append(args, diaIni, diaFin)
	}
	q += ` ORDER BY ts DESC, id DESC`
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	loc := a.loc()
	pag := &Pagina{Items: []MensajeVista{}}
	var elegidos []filaLigera
	for filas.Next() {
		var r filaLigera
		if err := filas.Scan(&r.id, &r.ts); err != nil {
			return nil, err
		}
		if f.Hora != nil || f.Semana != nil {
			t := time.Unix(r.ts, 0).In(loc)
			if f.Hora != nil && t.Hour() != *f.Hora {
				continue
			}
			if f.Semana != nil && (int(t.Weekday())+6)%7 != *f.Semana {
				continue
			}
		}
		pag.Total++
		if antesTS > 0 && !(r.ts < antesTS || (r.ts == antesTS && r.id < antesID)) {
			continue
		}
		if len(elegidos) < limite {
			elegidos = append(elegidos, r)
		}
	}
	if err := filas.Err(); err != nil {
		return nil, err
	}
	if antesTS > 0 {
		pag.Total = 0 // el total solo se calcula en la primera página
	}
	ms := []*Mensaje{}
	for _, r := range elegidos {
		m, err := a.ObtenerMensaje(botID, chat, r.id)
		if err != nil {
			return nil, err
		}
		if m != nil {
			ms = append(ms, m)
		}
	}
	pag.Items = a.vistasDeMensajes(ms)
	return pag, nil
}

// Persona es alguien que escribió en un chat.
type PersonaChat struct {
	JID      string `json:"jid"`
	Nombre   string `json:"nombre"`
	Cantidad int64  `json:"cantidad"`
}

// PersonasDeChat lista quién escribió en el chat (sin contar al bot), de quien más a quien menos.
func (a *Almacen) PersonasDeChat(botID int64, chat string) ([]PersonaChat, error) {
	filas, err := a.db.Query(`SELECT sender, SUM(count) FROM daily_stats WHERE bot_id=? AND chat=? AND sender<>? GROUP BY sender`, botID, chat, QuienYo)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []PersonaChat{}
	for filas.Next() {
		var p PersonaChat
		if err := filas.Scan(&p.JID, &p.Cantidad); err != nil {
			return nil, err
		}
		p.Nombre = a.nombreDe(botID, chat, p.JID)
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Cantidad != out[j].Cantidad {
			return out[i].Cantidad > out[j].Cantidad
		}
		return out[i].Nombre < out[j].Nombre
	})
	return out, filas.Err()
}
