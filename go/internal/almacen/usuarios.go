package almacen

import (
	"strings"
	"time"
)

// Rango «Usuario» automático: todas las personas que le escriben al bot o están en sus grupos, menos quienes administran un grupo,
// quienes ya tienen un rango y quienes están bloqueadas. Lo que se asigna solo queda marcado con asignado_por = "auto" para poder
// distinguirlo de un rango que el dueño dio a mano (el automático no cuenta como «conocido» para la política de desconocidos).

// AsignadoAuto marca los rangos que se dieron solos.
const AsignadoAuto = "auto"

// adminsDeGrupos devuelve las personas (canónicas) que administran algún grupo en el que están ahora.
func (a *Almacen) adminsDeGrupos(botID int64) map[string]bool {
	out := map[string]bool{}
	filas, err := a.db.Query(`SELECT DISTINCT persona FROM grupo_miembros WHERE bot_id=? AND hasta_ts=0 AND rol IN ('admin','creador')`, botID)
	if err != nil {
		return out
	}
	var ps []string
	for filas.Next() {
		var p string
		if filas.Scan(&p) == nil {
			ps = append(ps, p)
		}
	}
	filas.Close()
	for _, p := range ps {
		out[a.ResolverPersona(botID, p)] = true
		out[p] = true
	}
	return out
}

// EsAdminDeAlgunGrupo dice si la persona administra (o creó) algún grupo donde está el bot.
func (a *Almacen) EsAdminDeAlgunGrupo(botID int64, persona string) bool {
	return a.adminsDeGrupos(botID)[a.ResolverPersona(botID, persona)]
}

// AsignarAuto da el rango (rolID) a las personas que no tienen ninguno, no son administradoras de grupo ni están bloqueadas.
// Devuelve a cuántas se lo dio. Lo asignado queda marcado como «auto».
func (a *Almacen) AsignarAuto(botID, rolID int64, personas []string, ahora time.Time) (int, error) {
	if rolID <= 0 || len(personas) == 0 {
		return 0, nil
	}
	admins := a.adminsDeGrupos(botID)
	bloq := map[string]bool{}
	if filas, err := a.db.Query(`SELECT persona FROM solicitudes WHERE bot_id=? AND estado=?`, botID, SolBloqueada); err == nil {
		for filas.Next() {
			var p string
			if filas.Scan(&p) == nil {
				bloq[p] = true
			}
		}
		filas.Close()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	n := 0
	visto := map[string]bool{}
	for _, p := range personas {
		p = a.ResolverPersona(botID, strings.TrimSpace(p))
		if p == "" || p == PersonaDueno || p == QuienYo || visto[p] || admins[p] || bloq[p] || strings.HasSuffix(p, "@g.us") || strings.Contains(p, "@broadcast") || strings.HasPrefix(p, "status@") {
			continue
		}
		visto[p] = true
		r, err := tx.Exec(`INSERT OR IGNORE INTO persona_rol(bot_id,persona,rol_id,caduca_en,asignado_por,asignado_ts) VALUES(?,?,?,0,?,?)`, botID, p, rolID, AsignadoAuto, ahora.Unix())
		if err != nil {
			return 0, err
		}
		if c, _ := r.RowsAffected(); c > 0 {
			n++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	if n > 0 {
		a.avisa("rangos")
	}
	return n, nil
}

// QuitarAuto quita el rango a quien lo tiene por asignación automática (uno dado a mano no se toca). Dice si quitó algo.
func (a *Almacen) QuitarAuto(botID int64, persona string) bool {
	fs := a.formas(botID, persona)
	if len(fs) == 0 {
		fs = []string{persona}
	}
	args := []any{botID, AsignadoAuto}
	for _, f := range fs {
		args = append(args, f)
	}
	r, err := a.db.Exec(`DELETE FROM persona_rol WHERE bot_id=? AND asignado_por=? AND persona IN `+enLista(len(fs)), args...)
	if err != nil {
		return false
	}
	n, _ := r.RowsAffected()
	if n > 0 {
		a.avisa("rangos")
	}
	return n > 0
}

// QuitarTodosAuto quita todos los rangos dados solos (de un rango en particular si rolID > 0). Devuelve cuántos.
func (a *Almacen) QuitarTodosAuto(botID, rolID int64) int {
	q, args := `DELETE FROM persona_rol WHERE bot_id=? AND asignado_por=?`, []any{botID, AsignadoAuto}
	if rolID > 0 {
		q += ` AND rol_id=?`
		args = append(args, rolID)
	}
	r, err := a.db.Exec(q, args...)
	if err != nil {
		return 0
	}
	n, _ := r.RowsAffected()
	if n > 0 {
		a.avisa("rangos")
	}
	return int(n)
}

// QuitarAutoDeAdmins quita el rango automático a quienes ahora administran un grupo. Devuelve cuántos.
func (a *Almacen) QuitarAutoDeAdmins(botID int64) int {
	n := 0
	for p := range a.adminsDeGrupos(botID) {
		if a.QuitarAuto(botID, p) {
			n++
		}
	}
	return n
}

// PersonasParaUsuario devuelve las personas que cuentan para el rango Usuario: miembros vigentes de grupos y quienes tienen un chat privado
// con el bot. Las administradoras de grupo las descarta AsignarAuto.
func (a *Almacen) PersonasParaUsuario(botID int64, grupo string) []string {
	var out []string
	q, args := `SELECT DISTINCT persona FROM grupo_miembros WHERE bot_id=? AND hasta_ts=0`, []any{botID}
	if grupo != "" {
		q += ` AND grupo=?`
		args = append(args, grupo)
	}
	if filas, err := a.db.Query(q, args...); err == nil {
		for filas.Next() {
			var p string
			if filas.Scan(&p) == nil {
				out = append(out, p)
			}
		}
		filas.Close()
	}
	if grupo == "" {
		if filas, err := a.db.Query(`SELECT jid FROM chats WHERE bot_id=? AND es_grupo=0 AND ultimo_ts>0 AND jid LIKE '%@%'`, botID); err == nil {
			for filas.Next() {
				var p string
				if filas.Scan(&p) == nil {
					out = append(out, p)
				}
			}
			filas.Close()
		}
	}
	return out
}

// TieneRango dice si la persona ya tiene algún rango (de cualquier tipo, vigente o no vencido).
func (a *Almacen) TieneRango(botID int64, persona string) bool {
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM persona_rol WHERE bot_id=? AND persona=? AND (caduca_en=0 OR caduca_en>?)`, botID, a.ResolverPersona(botID, persona), time.Now().Unix()).Scan(&n)
	return n > 0
}

// RazonNoUsuario explica por qué a una persona no se le da el rango Usuario ("" = sí le toca).
func (a *Almacen) RazonNoUsuario(botID int64, persona string) string {
	p := a.ResolverPersona(botID, persona)
	switch {
	case p == "" || p == PersonaDueno || p == QuienYo:
		return "es el dueño"
	case a.TieneRango(botID, p):
		return "ya tiene un rango"
	case a.adminsDeGrupos(botID)[p]:
		return "administra un grupo"
	case a.EstaBloqueada(botID, p):
		return "está bloqueada"
	}
	return ""
}
