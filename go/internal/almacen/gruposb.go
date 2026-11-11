package almacen

import (
	"database/sql"
	"errors"
	"sort"
	"time"

	"botchat/internal/cifrado"
)

// Grupos (bloque E): filtros por grupo, reglas del grupo y actividad (ranking e inactivos) desde daily_stats. El almacén solo guarda y
// consulta; qué significa cada filtro lo decide el paquete `grupos`.

// FiltroGrupo es la configuración guardada de un filtro de un grupo. Config es un JSON que interpreta el paquete `grupos`.
type FiltroGrupo struct {
	Chat    string `json:"chat"`
	Filtro  string `json:"filtro"`
	Activo  bool   `json:"activo"`
	Accion  string `json:"accion"`
	Config  string `json:"config"`
	Editado int64  `json:"editado"`
}

// GuardarFiltroGrupo crea o reemplaza un filtro de un grupo.
func (a *Almacen) GuardarFiltroGrupo(botID int64, f FiltroGrupo, ahora time.Time) error {
	if f.Chat == "" || f.Filtro == "" {
		return errors.New("falta el grupo o el filtro")
	}
	cfg, err := cifrado.SellarTexto(a.clave, "grupo_filtro", []byte(f.Config))
	if err != nil {
		return err
	}
	act := 0
	if f.Activo {
		act = 1
	}
	_, err = a.db.Exec(`INSERT INTO grupo_filtros(bot_id,chat_jid,filtro,activo,accion,config_cifrado,editado_ts) VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(bot_id,chat_jid,filtro) DO UPDATE SET activo=excluded.activo, accion=excluded.accion, config_cifrado=excluded.config_cifrado, editado_ts=excluded.editado_ts`,
		botID, f.Chat, f.Filtro, act, f.Accion, cfg, ahora.Unix())
	if err == nil {
		a.avisa("grupo_filtros")
	}
	return err
}

func (a *Almacen) escanearFiltroGrupo(s interface{ Scan(...any) error }) (*FiltroGrupo, error) {
	var f FiltroGrupo
	var cfg []byte
	var act int
	if err := s.Scan(&f.Chat, &f.Filtro, &act, &f.Accion, &cfg, &f.Editado); err != nil {
		return nil, err
	}
	f.Activo = act != 0
	if len(cfg) > 0 {
		if t, err := cifrado.AbrirTexto(a.clave, "grupo_filtro", cfg); err == nil {
			f.Config = string(t)
		}
	}
	return &f, nil
}

// FiltrosDeGrupo devuelve los filtros guardados de un grupo (los que nunca se tocaron no aparecen).
func (a *Almacen) FiltrosDeGrupo(botID int64, chat string) ([]FiltroGrupo, error) {
	filas, err := a.db.Query(`SELECT chat_jid,filtro,activo,accion,config_cifrado,editado_ts FROM grupo_filtros WHERE bot_id=? AND chat_jid=? ORDER BY filtro`, botID, chat)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []FiltroGrupo{}
	for filas.Next() {
		if f, err := a.escanearFiltroGrupo(filas); err == nil {
			out = append(out, *f)
		}
	}
	return out, filas.Err()
}

// BorrarFiltroGrupo quita un filtro (vuelve a sus valores de fábrica).
func (a *Almacen) BorrarFiltroGrupo(botID int64, chat, filtro string) error {
	_, err := a.db.Exec(`DELETE FROM grupo_filtros WHERE bot_id=? AND chat_jid=? AND filtro=?`, botID, chat, filtro)
	if err == nil {
		a.avisa("grupo_filtros")
	}
	return err
}

// GruposConFiltros dice qué grupos tienen al menos un filtro encendido.
func (a *Almacen) GruposConFiltros(botID int64) ([]string, error) {
	filas, err := a.db.Query(`SELECT DISTINCT chat_jid FROM grupo_filtros WHERE bot_id=? AND activo=1 ORDER BY chat_jid`, botID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []string{}
	for filas.Next() {
		var c string
		if filas.Scan(&c) == nil {
			out = append(out, c)
		}
	}
	return out, filas.Err()
}

// FijarReglasGrupo guarda las reglas del grupo ("" las quita).
func (a *Almacen) FijarReglasGrupo(botID int64, chat, texto string, ahora time.Time) error {
	if texto == "" {
		_, err := a.db.Exec(`DELETE FROM grupo_reglas WHERE bot_id=? AND chat_jid=?`, botID, chat)
		return err
	}
	b, err := cifrado.SellarTexto(a.clave, "grupo_reglas", []byte(texto))
	if err != nil {
		return err
	}
	_, err = a.db.Exec(`INSERT INTO grupo_reglas(bot_id,chat_jid,texto_cifrado,editado_ts) VALUES(?,?,?,?)
		ON CONFLICT(bot_id,chat_jid) DO UPDATE SET texto_cifrado=excluded.texto_cifrado, editado_ts=excluded.editado_ts`, botID, chat, b, ahora.Unix())
	return err
}

// ReglasGrupo devuelve las reglas del grupo ("" si no hay).
func (a *Almacen) ReglasGrupo(botID int64, chat string) string {
	var b []byte
	if err := a.db.QueryRow(`SELECT texto_cifrado FROM grupo_reglas WHERE bot_id=? AND chat_jid=?`, botID, chat).Scan(&b); err != nil {
		return ""
	}
	t, err := cifrado.AbrirTexto(a.clave, "grupo_reglas", b)
	if err != nil {
		return ""
	}
	return string(t)
}

// DiaHace da el día (en la zona de las estadísticas) de hace `dias` días; dias = 0 es hoy.
func (a *Almacen) DiaHace(ahora time.Time, dias int) string {
	return ahora.In(a.loc()).AddDate(0, 0, -dias).Format("2006-01-02")
}

// ActividadMiembro es lo que un remitente escribió en un grupo.
type ActividadMiembro struct {
	Sender    string `json:"sender"`
	Mensajes  int    `json:"mensajes"`  // desde el día pedido
	UltimoDia string `json:"ultimoDia"` // último día con mensajes (en todo el historial guardado)
}

// ActividadGrupo suma, por remitente, los mensajes desde `desdeDia` (inclusive) y su último día con mensajes. Los remitentes sin
// mensajes en ese periodo salen con Mensajes = 0 y su último día anterior. Solo lee daily_stats (nunca el texto).
func (a *Almacen) ActividadGrupo(botID int64, chat, desdeDia string) ([]ActividadMiembro, error) {
	filas, err := a.db.Query(`SELECT sender, COALESCE(SUM(CASE WHEN day>=? THEN count ELSE 0 END),0), MAX(day) FROM daily_stats WHERE bot_id=? AND chat=? GROUP BY sender`,
		desdeDia, botID, chat)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []ActividadMiembro{}
	for filas.Next() {
		var x ActividadMiembro
		var ult sql.NullString
		if err := filas.Scan(&x.Sender, &x.Mensajes, &ult); err != nil {
			return nil, err
		}
		x.UltimoDia = ult.String
		out = append(out, x)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Mensajes != out[j].Mensajes {
			return out[i].Mensajes > out[j].Mensajes
		}
		return out[i].Sender < out[j].Sender
	})
	return out, filas.Err()
}

// PrimerDiaGrupo es el primer día con estadísticas de un grupo ("" si no hay): sirve para avisar que el ranking solo cuenta desde ahí.
func (a *Almacen) PrimerDiaGrupo(botID int64, chat string) string {
	var d sql.NullString
	_ = a.db.QueryRow(`SELECT MIN(day) FROM daily_stats WHERE bot_id=? AND chat=?`, botID, chat).Scan(&d)
	return d.String
}
