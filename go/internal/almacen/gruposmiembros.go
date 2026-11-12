package almacen

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Miembros de grupo y sus cambios (v0.6.0, E3). Solo JIDs y números: nada de texto de usuario (mismo criterio que daily_stats).
// Los nombres, el rango, las etiquetas y las advertencias se resuelven al leer.

// Roles dentro de un grupo.
const (
	RolGrupoMiembro = "miembro"
	RolGrupoAdmin   = "admin"
	RolGrupoCreador = "creador"
)

// Tipos de cambio de membresía.
const (
	CambioEntra     = "entra"     // entró por su cuenta (sin invitación de nadie visible)
	CambioEnlace    = "enlace"    // entró por el enlace de invitación
	CambioAgregado  = "agregado"  // lo agregó otra persona
	CambioSale      = "sale"      // salió por su cuenta
	CambioExpulsado = "expulsado" // lo sacó otra persona
	CambioPromovido = "promovido"
	CambioDegradado = "degradado"
)

// MiembroGrupo es una persona en un grupo, tal como llega de WhatsApp.
type MiembroGrupo struct {
	Persona string
	JID     string
	Rol     string
}

// MiembroVista es la fila que ve la pantalla de miembros.
type MiembroVista struct {
	Persona    string  `json:"persona"`
	JID        string  `json:"jid"`
	Nombre     string  `json:"nombre"`
	Telefono   string  `json:"telefono"`
	Rol        string  `json:"rol"`
	Rango      string  `json:"rango"`
	RangoColor string  `json:"rangoColor"`
	DesdeTS    int64   `json:"desdeTs"`
	HastaTS    int64   `json:"hastaTs"`
	Mensajes   int     `json:"mensajes"`
	UltimoTS   int64   `json:"ultimoTs"`
	Silenciado bool    `json:"silenciado"`
	Advertido  bool    `json:"advertido"`
	Etiquetas  []int64 `json:"etiquetas"`
	Foto       string  `json:"foto"`
}

// CambioGrupo es una entrada, salida, expulsión o ascenso.
type CambioGrupo struct {
	ID        int64  `json:"id"`
	Grupo     string `json:"grupo"`
	TS        int64  `json:"ts"`
	Tipo      string `json:"tipo"`
	Persona   string `json:"persona"`
	Nombre    string `json:"nombre"`
	Por       string `json:"por"`
	PorNombre string `json:"porNombre"`
	Via       string `json:"via"`
}

// ActualizarMiembros guarda la foto completa de los miembros de un grupo: los que ya no están quedan con hasta_ts, los que vuelven
// se reabren. No genera cambios (no se sabe cuándo pasaron). Devuelve cuántos miembros vigentes quedaron.
func (a *Almacen) ActualizarMiembros(botID int64, grupo string, ms []MiembroGrupo, ahora time.Time) (int, error) {
	if grupo == "" {
		return 0, errors.New("falta el grupo")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	ts := ahora.Unix()
	vistos := map[string]bool{}
	for _, m := range ms {
		if m.Persona == "" || vistos[m.Persona] {
			continue
		}
		vistos[m.Persona] = true
		rol := m.Rol
		if rol == "" {
			rol = RolGrupoMiembro
		}
		if _, err := tx.Exec(`INSERT INTO grupo_miembros(bot_id,grupo,persona,jid,rol,desde_ts,hasta_ts) VALUES(?,?,?,?,?,?,0)
			ON CONFLICT(bot_id,grupo,persona) DO UPDATE SET jid=excluded.jid, rol=excluded.rol,
			desde_ts=CASE WHEN grupo_miembros.hasta_ts<>0 THEN excluded.desde_ts ELSE grupo_miembros.desde_ts END, hasta_ts=0`,
			botID, grupo, m.Persona, m.JID, rol, ts); err != nil {
			return 0, err
		}
	}
	filas, err := tx.Query(`SELECT persona FROM grupo_miembros WHERE bot_id=? AND grupo=? AND hasta_ts=0`, botID, grupo)
	if err != nil {
		return 0, err
	}
	var salieron []string
	for filas.Next() {
		var p string
		if filas.Scan(&p) == nil && !vistos[p] {
			salieron = append(salieron, p)
		}
	}
	filas.Close()
	for _, p := range salieron {
		if _, err := tx.Exec(`UPDATE grupo_miembros SET hasta_ts=? WHERE bot_id=? AND grupo=? AND persona=?`, ts, botID, grupo, p); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	a.avisa("grupo:" + grupo)
	return len(vistos), nil
}

// CerrarGruposAusentes marca como salidos todos los miembros de los grupos que ya no están en la lista de grupos de la cuenta
// (el bot salió o lo sacaron). No borra el historial de cambios.
func (a *Almacen) CerrarGruposAusentes(botID int64, vigentes []string, ahora time.Time) {
	set := map[string]bool{}
	for _, g := range vigentes {
		set[g] = true
	}
	filas, err := a.db.Query(`SELECT DISTINCT grupo FROM grupo_miembros WHERE bot_id=? AND hasta_ts=0`, botID)
	if err != nil {
		return
	}
	var cerrar []string
	for filas.Next() {
		var g string
		if filas.Scan(&g) == nil && !set[g] {
			cerrar = append(cerrar, g)
		}
	}
	filas.Close()
	for _, g := range cerrar {
		_, _ = a.db.Exec(`UPDATE grupo_miembros SET hasta_ts=? WHERE bot_id=? AND grupo=? AND hasta_ts=0`, ahora.Unix(), botID, g)
	}
}

// CerrarGrupo marca como salidos a todos los miembros de un grupo (el bot ya no está).
func (a *Almacen) CerrarGrupo(botID int64, grupo string, ahora time.Time) {
	_, _ = a.db.Exec(`UPDATE grupo_miembros SET hasta_ts=? WHERE bot_id=? AND grupo=? AND hasta_ts=0`, ahora.Unix(), botID, grupo)
	a.avisa("grupo:" + grupo)
}

// RegistrarCambio anota un cambio de membresía y mantiene grupo_miembros al día. persona y por son claves canónicas ("" si no se sabe).
func (a *Almacen) RegistrarCambio(botID int64, grupo, tipo, persona, jid, por, via string, ts time.Time) error {
	switch tipo {
	case CambioEntra, CambioEnlace, CambioAgregado, CambioSale, CambioExpulsado, CambioPromovido, CambioDegradado:
	default:
		return errors.New("tipo de cambio no válido")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	t := ts.Unix()
	if _, err := tx.Exec(`INSERT INTO grupo_cambios(bot_id,grupo,ts,tipo,persona,por,via) VALUES(?,?,?,?,?,?,?)`, botID, grupo, t, tipo, persona, por, via); err != nil {
		return err
	}
	if persona != "" {
		switch tipo {
		case CambioEntra, CambioEnlace, CambioAgregado:
			_, err = tx.Exec(`INSERT INTO grupo_miembros(bot_id,grupo,persona,jid,rol,desde_ts,hasta_ts) VALUES(?,?,?,?,?,?,0)
				ON CONFLICT(bot_id,grupo,persona) DO UPDATE SET jid=CASE WHEN excluded.jid<>'' THEN excluded.jid ELSE grupo_miembros.jid END,
				rol='miembro', desde_ts=excluded.desde_ts, hasta_ts=0`, botID, grupo, persona, jid, RolGrupoMiembro, t)
		case CambioSale, CambioExpulsado:
			_, err = tx.Exec(`UPDATE grupo_miembros SET hasta_ts=? WHERE bot_id=? AND grupo=? AND persona=?`, t, botID, grupo, persona)
		case CambioPromovido:
			_, err = tx.Exec(`UPDATE grupo_miembros SET rol=CASE WHEN rol='creador' THEN rol ELSE 'admin' END WHERE bot_id=? AND grupo=? AND persona=?`, botID, grupo, persona)
		case CambioDegradado:
			_, err = tx.Exec(`UPDATE grupo_miembros SET rol='miembro' WHERE bot_id=? AND grupo=? AND persona=? AND rol<>'creador'`, botID, grupo, persona)
		}
		if err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	a.avisa("grupo:" + grupo)
	return nil
}

// ListarCambios devuelve los cambios de un grupo, el más reciente primero. antes > 0 pagina (ts menor a antes).
func (a *Almacen) ListarCambios(botID int64, grupo string, antes int64, limite int) ([]CambioGrupo, error) {
	if limite <= 0 || limite > 500 {
		limite = 100
	}
	q := `SELECT id,grupo,ts,tipo,persona,por,via FROM grupo_cambios WHERE bot_id=? AND grupo=?`
	args := []any{botID, grupo}
	if antes > 0 {
		q += ` AND ts<?`
		args = append(args, antes)
	}
	q += ` ORDER BY ts DESC, id DESC LIMIT ?`
	args = append(args, limite)
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	var out []CambioGrupo
	for filas.Next() {
		var c CambioGrupo
		if filas.Scan(&c.ID, &c.Grupo, &c.TS, &c.Tipo, &c.Persona, &c.Por, &c.Via) == nil {
			out = append(out, c)
		}
	}
	filas.Close()
	for i := range out {
		out[i].Nombre = a.nombreCorto(botID, out[i].Persona)
		out[i].PorNombre = a.nombreCorto(botID, out[i].Por)
	}
	if out == nil {
		out = []CambioGrupo{}
	}
	return out, nil
}

func (a *Almacen) nombreCorto(botID int64, persona string) string {
	if persona == "" {
		return ""
	}
	if persona == PersonaDueno || persona == QuienYo {
		return "Yo"
	}
	return a.NombreDePersona(botID, persona)
}

// MiembrosDe devuelve los miembros vigentes de un grupo (o todos los que alguna vez estuvieron si incluirSalidos) con rango del bot,
// etiquetas, silencio/advertencias y el contador de mensajes en ese grupo (por persona canónica, desde daily_stats).
func (a *Almacen) MiembrosDe(botID int64, grupo string, incluirSalidos bool, ahora time.Time) ([]MiembroVista, error) {
	q := `SELECT persona,jid,rol,desde_ts,hasta_ts,silenciado,advertido FROM grupo_miembros WHERE bot_id=? AND grupo=?`
	if !incluirSalidos {
		q += ` AND hasta_ts=0`
	}
	filas, err := a.db.Query(q+` ORDER BY persona`, botID, grupo)
	if err != nil {
		return nil, err
	}
	var out []MiembroVista
	for filas.Next() {
		var m MiembroVista
		var s, ad int
		if filas.Scan(&m.Persona, &m.JID, &m.Rol, &m.DesdeTS, &m.HastaTS, &s, &ad) == nil {
			m.Silenciado, m.Advertido, m.Etiquetas = s == 1, ad == 1, []int64{}
			out = append(out, m)
		}
	}
	filas.Close()
	// contador y último mensaje por persona canónica (una sola consulta por grupo)
	cont, ult := map[string]int{}, map[string]int64{}
	if fs, err := a.db.Query(`SELECT sender,SUM(count),MAX(day) FROM daily_stats WHERE bot_id=? AND chat=? GROUP BY sender`, botID, grupo); err == nil {
		for fs.Next() {
			var sd, dia string
			var n int
			if fs.Scan(&sd, &n, &dia) == nil {
				c := a.ResolverPersona(botID, sd)
				cont[c] += n
				if t, e := time.ParseInLocation("2006-01-02", dia, a.loc()); e == nil && t.Unix() > ult[c] {
					ult[c] = t.Unix()
				}
			}
		}
		fs.Close()
	}
	ps := make([]string, len(out))
	for i := range out {
		ps[i] = out[i].Persona
	}
	ets := a.EtiquetasDePersonas(botID, ps)
	for i := range out {
		m := &out[i]
		m.Mensajes, m.UltimoTS = cont[m.Persona], ult[m.Persona]
		if e := ets[m.Persona]; e != nil {
			m.Etiquetas = e
		}
		m.Nombre = a.nombreDe(botID, grupo, m.JID)
		if m.Nombre == "Participante" || m.Nombre == "" {
			if n := a.NombreDePersona(botID, m.Persona); n != "" {
				m.Nombre = n
			}
		}
		if i := strings.Index(m.Persona, "@s.whatsapp.net"); i > 0 {
			m.Telefono = "+" + m.Persona[:i]
		}
		if r, _, err := a.RolDePersona(botID, m.Persona, ahora); err == nil && r != nil {
			m.Rango, m.RangoColor = r.Nombre, r.Color
		}
		if m.Foto == "" {
			m.Foto = a.FotoDePersona(botID, m.Persona)
		}
		if a.ContarAdvertenciasEn(botID, m.Persona, ahora) > 0 {
			m.Advertido = true
		}
	}
	return out, nil
}

// GruposEnComun devuelve los grupos (vigentes) donde está la persona.
func (a *Almacen) GruposEnComun(botID int64, persona string) ([]string, error) {
	fs := a.formas(botID, persona)
	filas, err := a.db.Query(`SELECT DISTINCT grupo FROM grupo_miembros WHERE bot_id=? AND hasta_ts=0 AND persona IN `+enLista(len(fs)), args2(botID, fs)...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []string{}
	for filas.Next() {
		var g string
		if filas.Scan(&g) == nil {
			out = append(out, g)
		}
	}
	return out, filas.Err()
}

// GrupoSnapshotTS dice cuándo se hizo la última foto completa de miembros (0 = nunca).
func (a *Almacen) GrupoSnapshotTS(botID int64) int64 {
	var n int64
	_ = a.db.QueryRow(`SELECT COALESCE(CAST(valor AS INTEGER),0) FROM bot_ajustes WHERE bot_id=? AND clave='grupo_snapshot_ts'`, botID).Scan(&n)
	return n
}

// FijarGrupoSnapshotTS anota la hora de la última foto completa.
func (a *Almacen) FijarGrupoSnapshotTS(botID int64, ts int64) {
	_, _ = a.db.Exec(`INSERT INTO bot_ajustes(bot_id,clave,valor) VALUES(?,?,?) ON CONFLICT(bot_id,clave) DO UPDATE SET valor=excluded.valor`, botID, "grupo_snapshot_ts", strconv.FormatInt(ts, 10))
}

// ---------- foto de personas sin chat privado ----------

// RutaFotoPersona es el archivo cifrado de la foto de una persona (no depende de tener chat).
func RutaFotoPersona(botID int64, persona string) string {
	return filepath.ToSlash(filepath.Join("avatars", strconv.FormatInt(botID, 10), "p_"+HashChat(botID, "persona:"+persona)+".enc"))
}

// FotoDePersona devuelve la ruta de la foto de la persona: la de su chat privado o, si no, la guardada para personas de grupo.
func (a *Almacen) FotoDePersona(botID int64, persona string) string {
	for _, x := range a.formas(botID, persona) {
		if c, _ := a.ObtenerChat(botID, x); c != nil && !c.EsGrupo && c.FotoRuta != "" {
			return c.FotoRuta
		}
	}
	var r string
	_ = a.db.QueryRow(`SELECT ruta FROM persona_foto WHERE bot_id=? AND persona=?`, botID, a.ResolverPersona(botID, persona)).Scan(&r)
	return r
}

// FotoPersonaInfo devuelve (ruta, id, ts) de la foto guardada para la persona (ts 0 = nunca se pidió).
func (a *Almacen) FotoPersonaInfo(botID int64, persona string) (string, string, int64) {
	var r, id string
	var ts int64
	_ = a.db.QueryRow(`SELECT ruta,foto_id,ts FROM persona_foto WHERE bot_id=? AND persona=?`, botID, a.ResolverPersona(botID, persona)).Scan(&r, &id, &ts)
	return r, id, ts
}

// FijarFotoPersona anota el resultado de pedir la foto (ruta "" = sin foto o no disponible).
func (a *Almacen) FijarFotoPersona(botID int64, persona, ruta, id string, ts int64) error {
	_, err := a.db.Exec(`INSERT INTO persona_foto(bot_id,persona,ruta,foto_id,ts) VALUES(?,?,?,?,?)
		ON CONFLICT(bot_id,persona) DO UPDATE SET ruta=excluded.ruta, foto_id=excluded.foto_id, ts=excluded.ts`, botID, a.ResolverPersona(botID, persona), ruta, id, ts)
	if err == nil {
		a.avisa("personas")
	}
	return err
}

func (a *Almacen) borrarFotoPersona(botID int64, formas []string) {
	for _, f := range formas {
		var r string
		if a.db.QueryRow(`SELECT ruta FROM persona_foto WHERE bot_id=? AND persona=?`, botID, f).Scan(&r) == nil {
			if r != "" {
				a.quita(r)
			}
			_, _ = a.db.Exec(`DELETE FROM persona_foto WHERE bot_id=? AND persona=?`, botID, f)
		}
		a.quita(RutaFotoPersona(botID, f))
	}
}
