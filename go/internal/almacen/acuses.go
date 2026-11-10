package almacen

import (
	"sort"
	"strings"
)

// Tipos de acuse.
const (
	AcuseEntregado   = "entregado"
	AcuseLeido       = "leido"
	AcuseReproducido = "reproducido"
)

// GuardarAcuse anota que `participante` recibió, leyó o reprodujo un mensaje propio. Se ignora si el mensaje no existe o no es mío.
func (a *Almacen) GuardarAcuse(botID int64, chat, msgID, participante, tipo string, ts int64) (bool, error) {
	var mio int
	if err := a.db.QueryRow(`SELECT es_mio FROM messages WHERE bot_id=? AND chat_jid=? AND id=?`, botID, chat, msgID).Scan(&mio); err != nil || mio != 1 {
		return false, nil
	}
	// Un acuse de lectura implica que también lo recibió.
	tipos := []string{tipo}
	if tipo == AcuseLeido || tipo == AcuseReproducido {
		tipos = append(tipos, AcuseEntregado)
	}
	for _, t := range tipos {
		if _, err := a.db.Exec(`INSERT OR IGNORE INTO message_receipts(bot_id,chat_jid,msg_id,participante,tipo,ts) VALUES(?,?,?,?,?,?)`, botID, chat, msgID, participante, t, ts); err != nil {
			return false, err
		}
	}
	a.avisaMsg(chat, msgID)
	return true, nil
}

// AcusesDe son las horas de entrega, lectura y reproducción de una persona para un mensaje.
type AcusesDe struct {
	JID         string `json:"jid"`
	Nombre      string `json:"nombre"`
	Entregado   int64  `json:"entregado"`
	Leido       int64  `json:"leido"`
	Reproducido int64  `json:"reproducido"`
}

// InfoMensaje es lo que muestra "Info del mensaje".
type InfoMensajeVista struct {
	Enviado   int64      `json:"enviado"`
	Estado    string     `json:"estado"`
	EsGrupo   bool       `json:"esGrupo"`
	Personas  []AcusesDe `json:"personas"`
	Editado   bool       `json:"editado"`
	Reenviado bool       `json:"reenviado"`
}

// InfoDeMensaje devuelve las horas de entrega y lectura de un mensaje propio, y en grupos quién lo recibió y quién lo leyó.
func (a *Almacen) InfoDeMensaje(botID int64, chat, id string) (*InfoMensajeVista, error) {
	m, err := a.ObtenerMensaje(botID, chat, id)
	if err != nil || m == nil {
		return nil, err
	}
	c, _ := a.ObtenerChat(botID, chat)
	out := &InfoMensajeVista{Enviado: m.TS, Estado: m.EstadoEnvio, EsGrupo: c != nil && c.EsGrupo, Personas: []AcusesDe{}, Editado: m.Editado, Reenviado: m.Reenviado}
	filas, err := a.db.Query(`SELECT participante, tipo, ts FROM message_receipts WHERE bot_id=? AND chat_jid=? AND msg_id=?`, botID, chat, id)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	por := map[string]*AcusesDe{}
	for filas.Next() {
		var p, t string
		var ts int64
		if err := filas.Scan(&p, &t, &ts); err != nil {
			return nil, err
		}
		x := por[p]
		if x == nil {
			x = &AcusesDe{JID: p, Nombre: a.nombreParaMostrar(botID, chat, p)}
			por[p] = x
		}
		switch t {
		case AcuseEntregado:
			x.Entregado = ts
		case AcuseLeido:
			x.Leido = ts
		case AcuseReproducido:
			x.Reproducido = ts
		}
	}
	for _, x := range por {
		out.Personas = append(out.Personas, *x)
	}
	sort.Slice(out.Personas, func(i, j int) bool { return out.Personas[i].Nombre < out.Personas[j].Nombre })
	if out.Estado == "" || out.Estado == "enviado" {
		out.Estado = estadoDe(m.EstadoEnvio, m.EsMio, cuentaTipo(out.Personas, AcuseEntregado), cuentaTipo(out.Personas, AcuseLeido))
	}
	return out, filas.Err()
}

func cuentaTipo(ps []AcusesDe, tipo string) int {
	n := 0
	for _, p := range ps {
		if (tipo == AcuseEntregado && p.Entregado > 0) || (tipo == AcuseLeido && p.Leido > 0) {
			n++
		}
	}
	return n
}

// estadoDe resume el estado de un mensaje propio: enviando, enviado, entregado, leido o fallido.
func estadoDe(envio string, mio bool, entregados, leidos int) string {
	if !mio {
		return ""
	}
	switch envio {
	case "enviando", "fallido":
		return envio
	}
	switch {
	case leidos > 0:
		return "leido"
	case entregados > 0:
		return "entregado"
	}
	return "enviado"
}

// nombreParaMostrar da el nombre de una persona dentro de un chat: el del chat si es el privado, o el que ya se vio en el grupo.
func (a *Almacen) nombreParaMostrar(botID int64, chat, jid string) string {
	if jid == QuienYo || jid == "" {
		return "Tú"
	}
	var n string
	if err := a.db.QueryRow(`SELECT nombre_mostrado FROM chats WHERE bot_id=? AND jid=?`, botID, jid).Scan(&n); err == nil && strings.TrimSpace(n) != "" {
		return n
	}
	return a.nombreDe(botID, chat, jid)
}

// EstadoDeEnvio cambia el estado de envío de un mensaje propio (enviando, enviado, fallido).
func (a *Almacen) EstadoDeEnvio(botID int64, chat, id, estado string) error {
	_, err := a.db.Exec(`UPDATE messages SET estado_envio=? WHERE bot_id=? AND chat_jid=? AND id=? AND es_mio=1`, estado, botID, chat, id)
	if err == nil {
		a.avisaMsg(chat, id)
	}
	return err
}

// NombreConocido devuelve el nombre ya conocido de una persona en un chat, o "" si no se sabe.
func (a *Almacen) NombreConocido(botID int64, chat, jid string) string {
	var n string
	if err := a.db.QueryRow(`SELECT nombre_mostrado FROM chats WHERE bot_id=? AND jid=?`, botID, jid).Scan(&n); err == nil && strings.TrimSpace(n) != "" {
		return n
	}
	_ = a.db.QueryRow(`SELECT remitente_nombre FROM messages WHERE bot_id=? AND chat_jid=? AND remitente_jid=? AND remitente_nombre<>'' ORDER BY ts DESC LIMIT 1`, botID, chat, jid).Scan(&n)
	return n
}
