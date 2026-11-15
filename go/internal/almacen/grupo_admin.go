package almacen

import (
	"sort"
	"strings"
	"time"
)

// Administrar grupos: contadores por miembro, baneados y horario de cierre. Los contadores salen de `messages` (tipo, nota de voz,
// borrados y editados); nunca se lee el texto.

// ConteoMiembro es lo que una persona ha enviado a un grupo.
type ConteoMiembro struct {
	Mensajes   int   `json:"mensajes"`
	Texto      int   `json:"texto"`
	Imagenes   int   `json:"imagenes"`
	Videos     int   `json:"videos"`
	Audios     int   `json:"audios"`
	NotasVoz   int   `json:"notasVoz"`
	Stickers   int   `json:"stickers"`
	Documentos int   `json:"documentos"`
	Otros      int   `json:"otros"`
	Borrados   int   `json:"borrados"`
	Editados   int   `json:"editados"`
	PrimerTS   int64 `json:"primerTs"`
	UltimoTS   int64 `json:"ultimoTs"`
}

func (c *ConteoMiembro) suma(tipo string, notaVoz bool, n, borrados, editados int, primero, ultimo int64) {
	c.Mensajes += n
	c.Borrados += borrados
	c.Editados += editados
	switch {
	case tipo == TipoTexto:
		c.Texto += n
	case tipo == TipoImagen:
		c.Imagenes += n
	case tipo == TipoVideo:
		c.Videos += n
	case tipo == TipoAudio && notaVoz:
		c.NotasVoz += n
	case tipo == TipoAudio:
		c.Audios += n
	case tipo == TipoSticker:
		c.Stickers += n
	case tipo == TipoDocumento:
		c.Documentos += n
	default:
		c.Otros += n
	}
	if c.PrimerTS == 0 || (primero > 0 && primero < c.PrimerTS) {
		c.PrimerTS = primero
	}
	if ultimo > c.UltimoTS {
		c.UltimoTS = ultimo
	}
}

// ConteosMiembros cuenta, por persona canónica, lo que cada uno envió a un grupo (los mensajes del propio bot no cuentan).
func (a *Almacen) ConteosMiembros(botID int64, grupo string) map[string]*ConteoMiembro {
	out := map[string]*ConteoMiembro{}
	filas, err := a.db.Query(`SELECT remitente_jid, tipo, nota_voz, COUNT(*), SUM(borrado), SUM(editado), MIN(ts), MAX(ts)
		FROM messages WHERE bot_id=? AND chat_jid=? AND es_mio=0 AND remitente_jid<>'' GROUP BY remitente_jid, tipo, nota_voz`, botID, grupo)
	if err != nil {
		return out
	}
	defer filas.Close()
	for filas.Next() {
		var jid, tipo string
		var nota, n, bor, edi int
		var p, u int64
		if filas.Scan(&jid, &tipo, &nota, &n, &bor, &edi, &p, &u) != nil {
			continue
		}
		c := a.ResolverPersona(botID, jid)
		x := out[c]
		if x == nil {
			x = &ConteoMiembro{}
			out[c] = x
		}
		x.suma(tipo, nota == 1, n, bor, edi, p, u)
	}
	return out
}

// ConteoDe da lo mismo para una sola persona (todas sus formas: número y LID).
func (a *Almacen) ConteoPersona(botID int64, grupo, persona string) ConteoMiembro {
	var c ConteoMiembro
	fs := a.formas(botID, persona)
	if len(fs) == 0 {
		return c
	}
	args := []any{botID, grupo}
	for _, f := range fs {
		args = append(args, f)
	}
	filas, err := a.db.Query(`SELECT tipo, nota_voz, COUNT(*), SUM(borrado), SUM(editado), MIN(ts), MAX(ts) FROM messages
		WHERE bot_id=? AND chat_jid=? AND es_mio=0 AND remitente_jid IN `+enLista(len(fs))+` GROUP BY tipo, nota_voz`, args...)
	if err != nil {
		return c
	}
	defer filas.Close()
	for filas.Next() {
		var tipo string
		var nota, n, bor, edi int
		var p, u int64
		if filas.Scan(&tipo, &nota, &n, &bor, &edi, &p, &u) == nil {
			c.suma(tipo, nota == 1, n, bor, edi, p, u)
		}
	}
	return c
}

// PuntoDia es un día con su número de mensajes.
type PuntoDia struct {
	Dia string `json:"dia"`
	N   int    `json:"n"`
}

// SerieMiembro devuelve los mensajes por día (últimos `dias`, con ceros) y por hora del día de una persona en un grupo.
func (a *Almacen) SerieMiembro(botID int64, grupo, persona string, dias int, ahora time.Time) ([]PuntoDia, [24]int) {
	var horas [24]int
	if dias <= 0 || dias > 365 {
		dias = 30
	}
	fs := a.formas(botID, persona)
	porDia := map[string]int{}
	if len(fs) > 0 {
		args := []any{botID, grupo}
		for _, f := range fs {
			args = append(args, f)
		}
		desde := ahora.In(a.loc()).AddDate(0, 0, -(dias - 1)).Format("2006-01-02")
		args = append(args, desde)
		filas, err := a.db.Query(`SELECT day, hora, SUM(count) FROM daily_stats WHERE bot_id=? AND chat=? AND sender IN `+enLista(len(fs))+` AND day>=? GROUP BY day, hora`, args...)
		if err == nil {
			for filas.Next() {
				var d string
				var h, n int
				if filas.Scan(&d, &h, &n) == nil {
					porDia[d] += n
					if h >= 0 && h < 24 {
						horas[h] += n
					}
				}
			}
			filas.Close()
		}
	}
	serie := make([]PuntoDia, 0, dias)
	for i := dias - 1; i >= 0; i-- {
		d := ahora.In(a.loc()).AddDate(0, 0, -i).Format("2006-01-02")
		serie = append(serie, PuntoDia{Dia: d, N: porDia[d]})
	}
	return serie, horas
}

// ---------- baneados ----------

// Baneado es una persona que no puede volver al grupo.
type Baneado struct {
	Grupo     string `json:"grupo"`
	Persona   string `json:"persona"`
	Nombre    string `json:"nombre"`
	Telefono  string `json:"telefono"`
	TS        int64  `json:"ts"`
	Por       string `json:"por"`
	PorNombre string `json:"porNombre"`
	Motivo    string `json:"motivo"`
}

// Banear anota a la persona (canónica) como baneada del grupo. Si ya estaba, actualiza el motivo.
func (a *Almacen) Banear(botID int64, grupo, persona, por, motivo string, ahora time.Time) error {
	c := a.ResolverPersona(botID, persona)
	motivo = strings.TrimSpace(motivo)
	if len([]rune(motivo)) > 200 {
		motivo = string([]rune(motivo)[:200])
	}
	_, err := a.db.Exec(`INSERT INTO grupo_baneados(bot_id,grupo,persona,ts,por,motivo) VALUES(?,?,?,?,?,?)
		ON CONFLICT(bot_id,grupo,persona) DO UPDATE SET ts=excluded.ts, por=excluded.por, motivo=excluded.motivo`, botID, grupo, c, ahora.Unix(), por, motivo)
	if err == nil {
		a.avisa("grupo:" + grupo)
	}
	return err
}

// Desbanear quita el baneo; devuelve si había alguno.
func (a *Almacen) Desbanear(botID int64, grupo, persona string) (bool, error) {
	fs := a.formas(botID, persona)
	if len(fs) == 0 {
		fs = []string{persona}
	}
	args := []any{botID, grupo}
	for _, f := range fs {
		args = append(args, f)
	}
	r, err := a.db.Exec(`DELETE FROM grupo_baneados WHERE bot_id=? AND grupo=? AND persona IN `+enLista(len(fs)), args...)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	if n > 0 {
		a.avisa("grupo:" + grupo)
	}
	return n > 0, nil
}

// EstaBaneado dice si la persona (con cualquiera de sus formas) está baneada de ese grupo.
func (a *Almacen) EstaBaneado(botID int64, grupo, persona string) bool {
	fs := a.formas(botID, persona)
	if len(fs) == 0 {
		fs = []string{persona}
	}
	args := []any{botID, grupo}
	for _, f := range fs {
		args = append(args, f)
	}
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM grupo_baneados WHERE bot_id=? AND grupo=? AND persona IN `+enLista(len(fs)), args...).Scan(&n)
	return n > 0
}

// ListarBaneados devuelve los baneados de un grupo (el más reciente primero) con su nombre.
func (a *Almacen) ListarBaneados(botID int64, grupo string) ([]Baneado, error) {
	filas, err := a.db.Query(`SELECT grupo,persona,ts,por,motivo FROM grupo_baneados WHERE bot_id=? AND grupo=? ORDER BY ts DESC`, botID, grupo)
	if err != nil {
		return nil, err
	}
	out := []Baneado{}
	for filas.Next() {
		var b Baneado
		if filas.Scan(&b.Grupo, &b.Persona, &b.TS, &b.Por, &b.Motivo) == nil {
			out = append(out, b)
		}
	}
	filas.Close()
	for i := range out {
		out[i].Nombre = a.nombreCorto(botID, out[i].Persona)
		out[i].PorNombre = a.nombreCorto(botID, out[i].Por)
		if j := strings.Index(out[i].Persona, "@s.whatsapp.net"); j > 0 {
			out[i].Telefono = "+" + out[i].Persona[:j]
		}
	}
	return out, nil
}

// ---------- horario de cierre ----------

// HorarioGrupo cierra el grupo a `Cierra` y lo abre a `Abre` (HH:MM, hora del teléfono) los días marcados (bit 0 = lunes … bit 6 = domingo).
type HorarioGrupo struct {
	Grupo  string `json:"grupo"`
	Activo bool   `json:"activo"`
	Cierra string `json:"cierra"`
	Abre   string `json:"abre"`
	Dias   int    `json:"dias"`
	Ultimo string `json:"ultimo"` // "cerrado" | "abierto" | "" (lo último que hizo el horario)
}

// ValidaHora comprueba "HH:MM".
func ValidaHora(h string) bool {
	t, err := time.Parse("15:04", h)
	return err == nil && t.Format("15:04") == h
}

// GuardarHorario crea o cambia el horario de un grupo.
func (a *Almacen) GuardarHorario(botID int64, h HorarioGrupo) error {
	if !ValidaHora(h.Cierra) || !ValidaHora(h.Abre) {
		return errHoraNoValida
	}
	if h.Dias <= 0 || h.Dias > 127 {
		h.Dias = 127
	}
	act := 0
	if h.Activo {
		act = 1
	}
	_, err := a.db.Exec(`INSERT INTO grupo_horario(bot_id,grupo,activo,cierra,abre,dias,ultimo) VALUES(?,?,?,?,?,?,'')
		ON CONFLICT(bot_id,grupo) DO UPDATE SET activo=excluded.activo, cierra=excluded.cierra, abre=excluded.abre, dias=excluded.dias`,
		botID, h.Grupo, act, h.Cierra, h.Abre, h.Dias)
	if err == nil {
		a.avisa("grupo:" + h.Grupo)
	}
	return err
}

// HorarioDe devuelve el horario del grupo (uno apagado por defecto si no hay).
func (a *Almacen) HorarioDe(botID int64, grupo string) HorarioGrupo {
	h := HorarioGrupo{Grupo: grupo, Cierra: "22:00", Abre: "07:00", Dias: 127}
	var act int
	if a.db.QueryRow(`SELECT activo,cierra,abre,dias,ultimo FROM grupo_horario WHERE bot_id=? AND grupo=?`, botID, grupo).Scan(&act, &h.Cierra, &h.Abre, &h.Dias, &h.Ultimo) == nil {
		h.Activo = act == 1
	}
	return h
}

// HorariosActivos devuelve los horarios encendidos del bot.
func (a *Almacen) HorariosActivos(botID int64) []HorarioGrupo {
	filas, err := a.db.Query(`SELECT grupo,cierra,abre,dias,ultimo FROM grupo_horario WHERE bot_id=? AND activo=1`, botID)
	if err != nil {
		return nil
	}
	defer filas.Close()
	var out []HorarioGrupo
	for filas.Next() {
		h := HorarioGrupo{Activo: true}
		if filas.Scan(&h.Grupo, &h.Cierra, &h.Abre, &h.Dias, &h.Ultimo) == nil {
			out = append(out, h)
		}
	}
	return out
}

// MarcarHorario anota lo último que hizo el horario ("cerrado" o "abierto").
func (a *Almacen) MarcarHorario(botID int64, grupo, ultimo string) {
	_, _ = a.db.Exec(`UPDATE grupo_horario SET ultimo=? WHERE bot_id=? AND grupo=?`, ultimo, botID, grupo)
}

// EstadoDeseado dice cómo debería estar el grupo a esa hora según el horario: "cerrado", "abierto" o "" si hoy no aplica.
// El tramo cerrado puede cruzar la medianoche (22:00 → 07:00); los días marcados son los del momento en que cierra.
func (h HorarioGrupo) EstadoDeseado(t time.Time) string {
	if !h.Activo {
		return ""
	}
	min := func(s string) int {
		x, _ := time.Parse("15:04", s)
		return x.Hour()*60 + x.Minute()
	}
	c, ab, ahora := min(h.Cierra), min(h.Abre), t.Hour()*60+t.Minute()
	dia := func(d time.Time) bool { return h.Dias&(1<<uint((int(d.Weekday())+6)%7)) != 0 }
	if c == ab {
		return ""
	}
	if c > ab { // cruza la medianoche
		if ahora >= c {
			if dia(t) {
				return "cerrado"
			}
			return "abierto"
		}
		if ahora < ab {
			if dia(t.AddDate(0, 0, -1)) {
				return "cerrado"
			}
			return "abierto"
		}
		return "abierto"
	}
	if ahora >= c && ahora < ab && dia(t) {
		return "cerrado"
	}
	return "abierto"
}

var errHoraNoValida = errorString("la hora debe ser HH:MM (por ejemplo 22:30)")

type errorString string

func (e errorString) Error() string { return string(e) }

// OrdenarConteos devuelve las claves por mensajes (útil para rankings en pruebas).
func OrdenarConteos(m map[string]*ConteoMiembro) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return m[ks[i]].Mensajes > m[ks[j]].Mensajes })
	return ks
}

// ---------- historial de moderación ----------

// EventoGrupo es una línea del historial de un grupo: un cambio de miembros o algo que hizo el dueño, un comando o el bot.
type EventoGrupo struct {
	TS            int64  `json:"ts"`
	Clase         string `json:"clase"` // cambio | accion
	Tipo          string `json:"tipo"`  // entra, sale, expulsado… o grupo.expulsar, grupo.banear…
	Texto         string `json:"texto"` // frase lista para mostrar, sin el nombre de la persona
	Actor         string `json:"actor"`
	ActorNombre   string `json:"actorNombre"`
	Persona       string `json:"persona"`
	PersonaNombre string `json:"personaNombre"`
	Resultado     string `json:"resultado"`
}

var textoAccionGrupo = map[string]string{
	"grupo.expulsar": "expulsó a", "grupo.ascender": "hizo administradora a", "grupo.bajar": "quitó el cargo de administradora a",
	"grupo.banear": "baneó a", "grupo.desbanear": "quitó el baneo a", "grupo.baneado_volvio": "sacó de nuevo (baneada) a",
	"grupo.cerrar": "cerró el grupo", "grupo.abrir": "abrió el grupo", "grupo.renombrar": "cambió el nombre del grupo",
	"grupo.descripcion": "cambió la descripción del grupo", "grupo.enlace": "pidió el enlace del grupo", "grupo.enlace_reiniciar": "cambió el enlace del grupo", "grupo.agregar": "agregó a", "grupo.reglas": "cambió las reglas del grupo", "grupo.filtro": "cambió un filtro del grupo", "grupo.todos": "mencionó a todos",
	"grupo.borrar_mensaje": "borró un mensaje de", "grupo.bloquear_info": "bloqueó la información del grupo", "grupo.desbloquear_info": "liberó la información del grupo",
	"grupo.aprobar": "aprobó la entrada de", "grupo.rechazar": "rechazó la entrada de", "grupo.mencionar_todos": "mencionó a todos",
}

var textoCambioGrupo = map[string]string{
	CambioEntra: "entró al grupo", CambioEnlace: "entró por el enlace", CambioAgregado: "fue agregada", CambioSale: "salió del grupo",
	CambioExpulsado: "fue expulsada", CambioPromovido: "fue hecha administradora", CambioDegradado: "dejó de ser administradora",
}

// HistorialGrupo mezcla los cambios de miembros y las acciones de moderación de un grupo, lo más reciente primero.
// persona != "" deja solo lo que tiene que ver con esa persona. antes > 0 pagina.
func (a *Almacen) HistorialGrupo(botID int64, grupo, persona string, antes int64, limite int) ([]EventoGrupo, error) {
	if limite <= 0 || limite > 300 {
		limite = 100
	}
	var fs []string
	if persona != "" {
		fs = a.formas(botID, persona)
		if len(fs) == 0 {
			fs = []string{persona}
		}
	}
	filtroP := func(col string) (string, []any) {
		if persona == "" {
			return "", nil
		}
		args := []any{}
		for _, f := range fs {
			args = append(args, f)
		}
		return " AND " + col + " IN " + enLista(len(fs)), args
	}
	var out []EventoGrupo

	q := `SELECT ts,tipo,persona,por FROM grupo_cambios WHERE bot_id=? AND grupo=?`
	args := []any{botID, grupo}
	if antes > 0 {
		q += ` AND ts<?`
		args = append(args, antes)
	}
	if f, ar := filtroP("persona"); f != "" {
		q += f
		args = append(args, ar...)
	}
	q += ` ORDER BY ts DESC LIMIT ?`
	args = append(args, limite)
	if filas, err := a.db.Query(q, args...); err == nil {
		for filas.Next() {
			var e EventoGrupo
			if filas.Scan(&e.TS, &e.Tipo, &e.Persona, &e.Actor) == nil {
				e.Clase = "cambio"
				e.Texto = textoCambioGrupo[e.Tipo]
				out = append(out, e)
			}
		}
		filas.Close()
	}

	q = `SELECT ts,quien,accion,objetivo,resultado FROM registro_acciones WHERE bot_id=? AND chat=? AND accion LIKE 'grupo.%'`
	args = []any{botID, grupo}
	if antes > 0 {
		q += ` AND ts<?`
		args = append(args, antes)
	}
	if f, ar := filtroP("objetivo"); f != "" {
		q += f
		args = append(args, ar...)
	}
	q += ` ORDER BY ts DESC LIMIT ?`
	args = append(args, limite)
	if filas, err := a.db.Query(q, args...); err == nil {
		for filas.Next() {
			var e EventoGrupo
			if filas.Scan(&e.TS, &e.Actor, &e.Tipo, &e.Persona, &e.Resultado) == nil {
				e.Clase = "accion"
				e.Texto = textoAccionGrupo[e.Tipo]
				if e.Texto == "" {
					e.Texto = strings.TrimPrefix(e.Tipo, "grupo.")
				}
				out = append(out, e)
			}
		}
		filas.Close()
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].TS > out[j].TS })
	if len(out) > limite {
		out = out[:limite]
	}
	for i := range out {
		out[i].ActorNombre = a.nombreCorto(botID, out[i].Actor)
		out[i].PersonaNombre = a.nombreCorto(botID, out[i].Persona)
	}
	if out == nil {
		out = []EventoGrupo{}
	}
	return out, nil
}

// SerieGrupo devuelve los mensajes por día de todo el grupo (últimos `dias`, con ceros).
func (a *Almacen) SerieGrupo(botID int64, grupo string, dias int, ahora time.Time) []PuntoDia {
	if dias <= 0 || dias > 365 {
		dias = 14
	}
	porDia := map[string]int{}
	desde := ahora.In(a.loc()).AddDate(0, 0, -(dias - 1)).Format("2006-01-02")
	if filas, err := a.db.Query(`SELECT day, SUM(count) FROM daily_stats WHERE bot_id=? AND chat=? AND day>=? GROUP BY day`, botID, grupo, desde); err == nil {
		for filas.Next() {
			var d string
			var n int
			if filas.Scan(&d, &n) == nil {
				porDia[d] = n
			}
		}
		filas.Close()
	}
	out := make([]PuntoDia, 0, dias)
	for i := dias - 1; i >= 0; i-- {
		d := ahora.In(a.loc()).AddDate(0, 0, -i).Format("2006-01-02")
		out = append(out, PuntoDia{Dia: d, N: porDia[d]})
	}
	return out
}

// MensajesDesde cuenta los mensajes de otras personas en un grupo desde esa hora.
func (a *Almacen) MensajesDesde(botID int64, grupo string, desdeTS int64) int {
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE bot_id=? AND chat_jid=? AND es_mio=0 AND ts>=?`, botID, grupo, desdeTS).Scan(&n)
	return n
}
