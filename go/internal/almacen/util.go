package almacen

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"botchat/internal/cifrado"
)

// Utilidades (v0.5.0, bloque F): notas y pendientes PROPIOS de cada persona, por comando. Son de quien los escribe (el Dueño usa la
// persona "yo"); no son las notas del personal sobre alguien (notas_persona). El texto va cifrado.

// Tipos de elemento.
const (
	UtilNota      = "nota"
	UtilPendiente = "pendiente"
)

// MaxTextoUtil es el largo máximo de una nota o un pendiente.
const MaxTextoUtil = 500

// ItemUtil es una nota o un pendiente.
type ItemUtil struct {
	ID      int64  `json:"id"`
	Persona string `json:"persona"`
	Tipo    string `json:"tipo"`
	Texto   string `json:"texto"`
	Hecho   bool   `json:"hecho"`
	TS      int64  `json:"ts"`
	HechoTS int64  `json:"hechoTs,omitempty"`
}

// ErrTopeUtil avisa que la lista ya está llena.
var ErrTopeUtil = errors.New("la lista está llena: borra algo antes de agregar más")

func validaTipoUtil(t string) error {
	if t != UtilNota && t != UtilPendiente {
		return errors.New("tipo desconocido")
	}
	return nil
}

func (a *Almacen) personaUtil(botID int64, persona string) string {
	if persona == PersonaDueno {
		return persona
	}
	return a.ResolverPersona(botID, persona)
}

func (a *Almacen) formasUtil(botID int64, persona string) []string {
	if persona == PersonaDueno {
		return []string{persona}
	}
	return a.formas(botID, persona)
}

// AgregarItemUtil guarda una nota o un pendiente de la persona (hasta `util_tope` por lista).
func (a *Almacen) AgregarItemUtil(botID int64, persona, tipo, texto string, ahora time.Time) (ItemUtil, error) {
	if err := validaTipoUtil(tipo); err != nil {
		return ItemUtil{}, err
	}
	persona = a.personaUtil(botID, persona)
	texto = strings.TrimSpace(texto)
	if persona == "" {
		return ItemUtil{}, errors.New("falta la persona")
	}
	if texto == "" {
		return ItemUtil{}, errors.New("el texto está vacío")
	}
	if len([]rune(texto)) > MaxTextoUtil {
		return ItemUtil{}, fmt.Errorf("es muy largo (máximo %d letras)", MaxTextoUtil)
	}
	if tope := a.AjusteBotInt(botID, "util_tope"); tope > 0 {
		var n int
		fs := a.formasUtil(botID, persona)
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM util_items WHERE bot_id=? AND tipo=? AND persona IN `+enLista(len(fs)), append([]any{botID, tipo}, args2(botID, fs)[1:]...)...).Scan(&n)
		if n >= tope {
			return ItemUtil{}, ErrTopeUtil
		}
	}
	sel, err := cifrado.SellarTexto(a.clave, "util_item", []byte(texto))
	if err != nil {
		return ItemUtil{}, err
	}
	r, err := a.db.Exec(`INSERT INTO util_items(bot_id,persona,tipo,texto_cifrado,ts) VALUES(?,?,?,?,?)`, botID, persona, tipo, sel, ahora.Unix())
	if err != nil {
		return ItemUtil{}, err
	}
	id, _ := r.LastInsertId()
	a.avisa("util:" + persona)
	return ItemUtil{ID: id, Persona: persona, Tipo: tipo, Texto: texto, TS: ahora.Unix()}, nil
}

// ListarItemsUtil lista de más viejo a más nuevo (así los números que ve la persona no cambian al agregar). tipo "" = ambos.
// Con conHechos=false se omiten los pendientes ya hechos.
func (a *Almacen) ListarItemsUtil(botID int64, persona, tipo string, conHechos bool) []ItemUtil {
	fs := a.formasUtil(botID, a.personaUtil(botID, persona))
	q := `SELECT id,persona,tipo,texto_cifrado,hecho,ts,hecho_ts FROM util_items WHERE bot_id=? AND persona IN ` + enLista(len(fs))
	args := args2(botID, fs)
	if tipo != "" {
		q += ` AND tipo=?`
		args = append(args, tipo)
	}
	if !conHechos {
		q += ` AND hecho=0`
	}
	q += ` ORDER BY id`
	filas, err := a.db.Query(q, args...)
	out := []ItemUtil{}
	if err != nil {
		return out
	}
	defer filas.Close()
	for filas.Next() {
		var x ItemUtil
		var b []byte
		var h int
		if filas.Scan(&x.ID, &x.Persona, &x.Tipo, &b, &h, &x.TS, &x.HechoTS) != nil {
			continue
		}
		x.Hecho = h != 0
		if t, e := cifrado.AbrirTexto(a.clave, "util_item", b); e == nil {
			x.Texto = string(t)
		}
		out = append(out, x)
	}
	return out
}

// MarcarItemUtil marca (o desmarca) un pendiente como hecho. Devuelve false si no existe o no es de esa persona.
func (a *Almacen) MarcarItemUtil(botID int64, persona string, id int64, hecho bool, ahora time.Time) (bool, error) {
	fs := a.formasUtil(botID, a.personaUtil(botID, persona))
	ts := int64(0)
	if hecho {
		ts = ahora.Unix()
	}
	r, err := a.db.Exec(`UPDATE util_items SET hecho=?, hecho_ts=? WHERE bot_id=? AND id=? AND tipo=? AND persona IN `+enLista(len(fs)),
		append([]any{b2i(hecho), ts, botID, id, UtilPendiente}, args2(botID, fs)[1:]...)...)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	a.avisa("util:" + a.personaUtil(botID, persona))
	return n > 0, nil
}

// BorrarItemUtil quita una nota o un pendiente de esa persona.
func (a *Almacen) BorrarItemUtil(botID int64, persona string, id int64) (bool, error) {
	fs := a.formasUtil(botID, a.personaUtil(botID, persona))
	r, err := a.db.Exec(`DELETE FROM util_items WHERE bot_id=? AND id=? AND persona IN `+enLista(len(fs)), append([]any{botID, id}, args2(botID, fs)[1:]...)...)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	a.avisa("util:" + a.personaUtil(botID, persona))
	return n > 0, nil
}

// BorrarHechosUtil quita los pendientes ya hechos. Devuelve cuántos.
func (a *Almacen) BorrarHechosUtil(botID int64, persona string) (int, error) {
	fs := a.formasUtil(botID, a.personaUtil(botID, persona))
	r, err := a.db.Exec(`DELETE FROM util_items WHERE bot_id=? AND tipo=? AND hecho=1 AND persona IN `+enLista(len(fs)), append([]any{botID, UtilPendiente}, args2(botID, fs)[1:]...)...)
	if err != nil {
		return 0, err
	}
	n, _ := r.RowsAffected()
	a.avisa("util:" + a.personaUtil(botID, persona))
	return int(n), nil
}
