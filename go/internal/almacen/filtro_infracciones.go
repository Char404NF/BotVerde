package almacen

import (
	"strings"
	"time"
)

// ---------- contador de infracciones de los filtros de grupo ----------
//
// Cada mensaje que rompe un filtro deja una fila (grupo, persona, filtro, hora). Nunca se guarda el texto. Con ellas se cuenta
// cuántas lleva cada persona por filtro (en total o en los últimos N días) y se puede escalar la acción.

// retencionInfracciones es cuánto se guardan las infracciones (un año).
const retencionInfracciones = 365 * 24 * time.Hour

// RegistrarInfraccionFiltro anota una infracción y devuelve cuántas lleva la persona en ese filtro dentro de la ventana
// (dias = 0: todas las que hay).
func (a *Almacen) RegistrarInfraccionFiltro(botID int64, grupo, persona, filtro string, ahora time.Time, dias int) (int, error) {
	if _, err := a.db.Exec(`INSERT INTO filtro_infracciones(bot_id,grupo,persona,filtro,ts) VALUES(?,?,?,?,?)`, botID, grupo, persona, filtro, ahora.Unix()); err != nil {
		return 0, err
	}
	_, _ = a.db.Exec(`DELETE FROM filtro_infracciones WHERE bot_id=? AND ts<?`, botID, ahora.Add(-retencionInfracciones).Unix())
	a.avisa("grupo:" + grupo)
	return a.ContarInfraccionesFiltro(botID, grupo, persona, filtro, ahora, dias), nil
}

// ContarInfraccionesFiltro cuenta las infracciones de la persona (con cualquiera de sus formas) en un filtro. dias = 0: todas.
func (a *Almacen) ContarInfraccionesFiltro(botID int64, grupo, persona, filtro string, ahora time.Time, dias int) int {
	fs := a.formas(botID, persona)
	if len(fs) == 0 {
		fs = []string{persona}
	}
	desde := int64(0)
	if dias > 0 {
		desde = ahora.Add(-time.Duration(dias) * 24 * time.Hour).Unix()
	}
	args := []any{botID, grupo, filtro, desde}
	for _, f := range fs {
		args = append(args, f)
	}
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM filtro_infracciones WHERE bot_id=? AND grupo=? AND filtro=? AND ts>=? AND persona IN `+enLista(len(fs)), args...).Scan(&n)
	return n
}

// InfraccionesPersona es lo que lleva una persona en un filtro de un grupo.
type InfraccionesPersona struct {
	Filtro string `json:"filtro"`
	Total  int    `json:"total"`
	Ultima int64  `json:"ultima"`
}

// InfraccionesDePersona devuelve, por filtro, cuántas lleva la persona en ese grupo (los filtros sin infracciones no salen).
func (a *Almacen) InfraccionesDePersona(botID int64, grupo, persona string) []InfraccionesPersona {
	fs := a.formas(botID, persona)
	if len(fs) == 0 {
		fs = []string{persona}
	}
	args := []any{botID, grupo}
	for _, f := range fs {
		args = append(args, f)
	}
	out := []InfraccionesPersona{}
	filas, err := a.db.Query(`SELECT filtro, COUNT(*), MAX(ts) FROM filtro_infracciones WHERE bot_id=? AND grupo=? AND persona IN `+enLista(len(fs))+` GROUP BY filtro ORDER BY COUNT(*) DESC`, args...)
	if err != nil {
		return out
	}
	defer filas.Close()
	for filas.Next() {
		var x InfraccionesPersona
		if filas.Scan(&x.Filtro, &x.Total, &x.Ultima) == nil {
			out = append(out, x)
		}
	}
	return out
}

// InfractorFiltro es una persona con lo que lleva en un filtro.
type InfractorFiltro struct {
	Persona  string `json:"persona"`
	Nombre   string `json:"nombre"`
	Telefono string `json:"telefono,omitempty"`
	Total    int    `json:"total"`
	Ultima   int64  `json:"ultima"`
}

// InfractoresDeFiltro devuelve quién lleva más infracciones en un filtro de un grupo (filtro vacío = todos los filtros juntos).
func (a *Almacen) InfractoresDeFiltro(botID int64, grupo, filtro string, limite int) []InfractorFiltro {
	if limite < 1 || limite > 200 {
		limite = 50
	}
	q := `SELECT persona, COUNT(*), MAX(ts) FROM filtro_infracciones WHERE bot_id=? AND grupo=?`
	args := []any{botID, grupo}
	if filtro != "" {
		q += ` AND filtro=?`
		args = append(args, filtro)
	}
	q += ` GROUP BY persona ORDER BY COUNT(*) DESC, MAX(ts) DESC LIMIT ?`
	args = append(args, limite)
	out := []InfractorFiltro{}
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return out
	}
	for filas.Next() {
		var x InfractorFiltro
		if filas.Scan(&x.Persona, &x.Total, &x.Ultima) == nil {
			out = append(out, x)
		}
	}
	filas.Close()
	for i := range out {
		out[i].Nombre = a.nombreCorto(botID, out[i].Persona)
		if j := strings.Index(out[i].Persona, "@s.whatsapp.net"); j > 0 {
			out[i].Telefono = "+" + out[i].Persona[:j]
		}
	}
	return out
}

// BorrarInfraccionesFiltro pone en cero el contador. persona vacía = todas las personas del grupo; filtro vacío = todos los filtros.
func (a *Almacen) BorrarInfraccionesFiltro(botID int64, grupo, persona, filtro string) (int, error) {
	q := `DELETE FROM filtro_infracciones WHERE bot_id=? AND grupo=?`
	args := []any{botID, grupo}
	if persona != "" {
		fs := a.formas(botID, persona)
		if len(fs) == 0 {
			fs = []string{persona}
		}
		q += ` AND persona IN ` + enLista(len(fs))
		for _, f := range fs {
			args = append(args, f)
		}
	}
	if filtro != "" {
		q += ` AND filtro=?`
		args = append(args, filtro)
	}
	r, err := a.db.Exec(q, args...)
	if err != nil {
		return 0, err
	}
	n, _ := r.RowsAffected()
	a.avisa("grupo:" + grupo)
	return int(n), nil
}
