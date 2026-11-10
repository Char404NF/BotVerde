package almacen

import "sort"

// ReaccionVista agrupa las reacciones con un mismo emoji.
type ReaccionVista struct {
	Emoji    string `json:"emoji"`
	Cantidad int    `json:"cantidad"`
	Mia      bool   `json:"mia"`
}

// GuardarReaccion anota la reacción de `sender` a un mensaje; un emoji vacío la quita.
func (a *Almacen) GuardarReaccion(botID int64, chat, msgID, sender, emoji string, ts int64) error {
	var err error
	if emoji == "" {
		_, err = a.db.Exec(`DELETE FROM reacciones WHERE bot_id=? AND chat_jid=? AND msg_id=? AND sender=?`, botID, chat, msgID, sender)
	} else {
		_, err = a.db.Exec(`INSERT INTO reacciones(bot_id,chat_jid,msg_id,sender,emoji,ts) VALUES(?,?,?,?,?,?)
			ON CONFLICT(bot_id,chat_jid,msg_id,sender) DO UPDATE SET emoji=excluded.emoji, ts=excluded.ts`, botID, chat, msgID, sender, emoji, ts)
	}
	if err == nil {
		a.avisaMsg(chat, msgID)
	}
	return err
}

func (a *Almacen) reaccionesDe(botID int64, chat string, ids []string) (map[string][]ReaccionVista, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := []any{botID, chat}
	for _, id := range ids {
		args = append(args, id)
	}
	filas, err := a.db.Query(`SELECT msg_id, emoji, sender FROM reacciones WHERE bot_id=? AND chat_jid=? AND msg_id IN (`+marcas(len(ids))+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	cuenta := map[string]map[string]*ReaccionVista{}
	for filas.Next() {
		var id, emoji, sender string
		if err := filas.Scan(&id, &emoji, &sender); err != nil {
			return nil, err
		}
		if cuenta[id] == nil {
			cuenta[id] = map[string]*ReaccionVista{}
		}
		r := cuenta[id][emoji]
		if r == nil {
			r = &ReaccionVista{Emoji: emoji}
			cuenta[id][emoji] = r
		}
		r.Cantidad++
		if sender == QuienYo {
			r.Mia = true
		}
	}
	out := map[string][]ReaccionVista{}
	for id, m := range cuenta {
		for _, r := range m {
			out[id] = append(out[id], *r)
		}
		l := out[id]
		sort.Slice(l, func(i, j int) bool {
			if l[i].Cantidad != l[j].Cantidad {
				return l[i].Cantidad > l[j].Cantidad
			}
			return l[i].Emoji < l[j].Emoji
		})
	}
	return out, filas.Err()
}

// MiReaccion devuelve el emoji con el que reaccioné a un mensaje ("" si no).
func (a *Almacen) MiReaccion(botID int64, chat, msgID string) string {
	var e string
	_ = a.db.QueryRow(`SELECT emoji FROM reacciones WHERE bot_id=? AND chat_jid=? AND msg_id=? AND sender=?`, botID, chat, msgID, QuienYo).Scan(&e)
	return e
}
