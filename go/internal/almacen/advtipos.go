package almacen

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"botchat/internal/cifrado"
)

// Tipos de advertencia, su escalera de sanciones y las reglas del grupo como lista (v0.6.0, E5).

// Acciones de un paso de la escalera.
const (
	EscAvisar        = "avisar"         // un aviso a la persona (y a mí)
	EscSilenciar     = "silenciar"      // silencio de grupo por `minutos`
	EscSilencioLargo = "silencio_largo" // silencio de grupo largo (sil_largo_min)
	EscExpulsar      = "expulsar"       // expulsión CON confirmación mía (nunca sola)
)

// EsAccionEscalera dice si el valor es una acción válida de la escalera.
func EsAccionEscalera(v string) bool {
	switch v {
	case EscAvisar, EscSilenciar, EscSilencioLargo, EscExpulsar:
		return true
	}
	return false
}

// Alcances de un paso de silencio de la escalera.
const (
	EscAlcanceGrupo = "grupo" // el grupo donde pasó
	EscAlcanceTodos = "todos" // todos los grupos
)

// TipoAdv es un tipo de advertencia (Spam, Insultos…).
type TipoAdv struct {
	ID            int64     `json:"id"`
	Nombre        string    `json:"nombre"`
	Emoji         string    `json:"emoji"`
	Color         string    `json:"color"`
	Posicion      int       `json:"posicion"`
	CaducidadDias int       `json:"caducidadDias"` // -1 = la general (adv_caducidad_dias); 0 = no caduca
	ReglaID       int64     `json:"reglaId"`
	Activo        bool      `json:"activo"`
	Escalera      []PasoEsc `json:"escalera"`
}

// PasoEsc es un paso de la escalera: con la advertencia número `Nivel` (vigente, de ese tipo) se hace `Accion`.
type PasoEsc struct {
	Nivel   int    `json:"nivel"`
	Accion  string `json:"accion"`
	Minutos int    `json:"minutos"`
	Alcance string `json:"alcance"`
	Aviso   string `json:"aviso"` // texto del aviso a la persona ("" = el de fábrica)
}

// tiposDeFabrica son los tipos que se siembran la primera vez, con su escalera.
func tiposDeFabrica() []TipoAdv {
	esc := func() []PasoEsc {
		return []PasoEsc{
			{Nivel: 1, Accion: EscAvisar, Alcance: EscAlcanceGrupo},
			{Nivel: 2, Accion: EscSilenciar, Minutos: 60, Alcance: EscAlcanceGrupo},
			{Nivel: 3, Accion: EscSilencioLargo, Alcance: EscAlcanceGrupo},
			{Nivel: 4, Accion: EscExpulsar, Alcance: EscAlcanceGrupo},
		}
	}
	return []TipoAdv{
		{Nombre: "Spam", Emoji: "📢", Color: "#E5484D", CaducidadDias: -1, Activo: true, Escalera: esc()},
		{Nombre: "Insultos", Emoji: "🤬", Color: "#F59E28", CaducidadDias: -1, Activo: true, Escalera: esc()},
		{Nombre: "Enlaces", Emoji: "🔗", Color: "#2EC4CB", CaducidadDias: -1, Activo: true, Escalera: esc()},
		{Nombre: "Otro", Emoji: "⚠️", Color: "#8E8E93", CaducidadDias: -1, Activo: true, Escalera: esc()},
	}
}

// SembrarTiposAdv crea los tipos de fábrica la primera vez (se anota en adv_tipos_sembrados; si luego se borran, no vuelven).
func (a *Almacen) SembrarTiposAdv(botID int64) error {
	if a.AjusteBot(botID, "adv_tipos_sembrados") == "1" {
		return nil
	}
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM adv_tipos WHERE bot_id=?`, botID).Scan(&n)
	if n == 0 {
		for i, t := range tiposDeFabrica() {
			t.Posicion = i
			if _, err := a.GuardarTipoAdv(botID, t); err != nil {
				return err
			}
		}
	}
	return a.FijarAjusteBot(botID, "adv_tipos_sembrados", "1")
}

func validaEscalera(esc []PasoEsc) ([]PasoEsc, error) {
	vistos := map[int]bool{}
	out := make([]PasoEsc, 0, len(esc))
	for _, p := range esc {
		if p.Nivel < 1 || p.Nivel > 50 {
			return nil, errors.New("el número de advertencia de cada paso debe estar entre 1 y 50")
		}
		if vistos[p.Nivel] {
			return nil, fmt.Errorf("el paso %d está repetido", p.Nivel)
		}
		vistos[p.Nivel] = true
		if !EsAccionEscalera(p.Accion) {
			return nil, errors.New("la acción debe ser avisar, silenciar, silencio_largo o expulsar")
		}
		if p.Accion == EscSilenciar && (p.Minutos < 1 || p.Minutos > 525600) {
			return nil, errors.New("los minutos de silencio deben estar entre 1 y 525600")
		}
		if p.Alcance == "" {
			p.Alcance = EscAlcanceGrupo
		}
		if p.Alcance != EscAlcanceGrupo && p.Alcance != EscAlcanceTodos {
			return nil, errors.New("el alcance debe ser grupo o todos")
		}
		p.Aviso = strings.TrimSpace(p.Aviso)
		if len([]rune(p.Aviso)) > 500 {
			return nil, errors.New("el aviso es muy largo (máximo 500 letras)")
		}
		out = append(out, p)
	}
	// ordenadas por nivel
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Nivel < out[j-1].Nivel; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, nil
}

// GuardarTipoAdv crea (ID = 0) o edita un tipo y reemplaza su escalera. Devuelve su id.
func (a *Almacen) GuardarTipoAdv(botID int64, t TipoAdv) (int64, error) {
	t.Nombre = strings.TrimSpace(t.Nombre)
	if t.Nombre == "" {
		return 0, errors.New("el tipo necesita un nombre")
	}
	if len([]rune(t.Nombre)) > 30 {
		return 0, errors.New("el nombre es muy largo (máximo 30 letras)")
	}
	if len([]rune(t.Emoji)) > 8 {
		return 0, errors.New("el emoji es muy largo")
	}
	if t.Color != "" && !colorHex(t.Color) {
		return 0, errors.New("el color debe ser como #E5484D")
	}
	if t.CaducidadDias < -1 || t.CaducidadDias > 3650 {
		return 0, errors.New("la caducidad debe ser -1 (la general), 0 (nunca) o hasta 3650 días")
	}
	esc, err := validaEscalera(t.Escalera)
	if err != nil {
		return 0, err
	}
	var otro int64
	if e := a.db.QueryRow(`SELECT id FROM adv_tipos WHERE bot_id=? AND LOWER(nombre)=LOWER(?) AND id<>?`, botID, t.Nombre, t.ID).Scan(&otro); e == nil {
		return 0, errors.New("ya hay un tipo con ese nombre")
	}
	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if t.ID == 0 {
		pos := t.Posicion
		if pos == 0 {
			_ = tx.QueryRow(`SELECT COALESCE(MAX(posicion)+1,0) FROM adv_tipos WHERE bot_id=?`, botID).Scan(&pos)
		}
		r, err := tx.Exec(`INSERT INTO adv_tipos(bot_id,nombre,emoji,color,posicion,caducidad_dias,regla_id,activo) VALUES(?,?,?,?,?,?,?,?)`,
			botID, t.Nombre, t.Emoji, t.Color, pos, t.CaducidadDias, t.ReglaID, b2i(t.Activo))
		if err != nil {
			return 0, err
		}
		t.ID, _ = r.LastInsertId()
	} else {
		r, err := tx.Exec(`UPDATE adv_tipos SET nombre=?, emoji=?, color=?, posicion=?, caducidad_dias=?, regla_id=?, activo=? WHERE bot_id=? AND id=?`,
			t.Nombre, t.Emoji, t.Color, t.Posicion, t.CaducidadDias, t.ReglaID, b2i(t.Activo), botID, t.ID)
		if err != nil {
			return 0, err
		}
		if n, _ := r.RowsAffected(); n == 0 {
			return 0, errors.New("ese tipo no existe")
		}
	}
	if err := a.escribirEscalera(tx, t.ID, esc); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	a.avisa("adv_tipos")
	return t.ID, nil
}

func colorHex(c string) bool {
	if len(c) != 7 || c[0] != '#' {
		return false
	}
	for _, x := range c[1:] {
		if !(x >= '0' && x <= '9' || x >= 'a' && x <= 'f' || x >= 'A' && x <= 'F') {
			return false
		}
	}
	return true
}

func (a *Almacen) escribirEscalera(tx *sql.Tx, tipoID int64, esc []PasoEsc) error {
	if _, err := tx.Exec(`DELETE FROM adv_escalera WHERE tipo_id=?`, tipoID); err != nil {
		return err
	}
	for _, p := range esc {
		var sel []byte
		if p.Aviso != "" {
			var err error
			if sel, err = cifrado.SellarTexto(a.clave, "aviso_esc", []byte(p.Aviso)); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`INSERT INTO adv_escalera(tipo_id,nivel,accion,minutos,alcance,aviso_cifrado) VALUES(?,?,?,?,?,?)`,
			tipoID, p.Nivel, p.Accion, p.Minutos, p.Alcance, sel); err != nil {
			return err
		}
	}
	return nil
}

// GuardarEscalera reemplaza la escalera de un tipo.
func (a *Almacen) GuardarEscalera(botID, tipoID int64, esc []PasoEsc) error {
	esc, err := validaEscalera(esc)
	if err != nil {
		return err
	}
	var n int
	if a.db.QueryRow(`SELECT COUNT(*) FROM adv_tipos WHERE bot_id=? AND id=?`, botID, tipoID).Scan(&n); n == 0 {
		return errors.New("ese tipo no existe")
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := a.escribirEscalera(tx, tipoID, esc); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	a.avisa("adv_tipos")
	return nil
}

// Escalera devuelve los pasos de un tipo, por nivel.
func (a *Almacen) Escalera(tipoID int64) []PasoEsc {
	filas, err := a.db.Query(`SELECT nivel,accion,minutos,alcance,aviso_cifrado FROM adv_escalera WHERE tipo_id=? ORDER BY nivel`, tipoID)
	out := []PasoEsc{}
	if err != nil {
		return out
	}
	defer filas.Close()
	for filas.Next() {
		var p PasoEsc
		var b []byte
		if filas.Scan(&p.Nivel, &p.Accion, &p.Minutos, &p.Alcance, &b) != nil {
			continue
		}
		if len(b) > 0 {
			if t, e := cifrado.AbrirTexto(a.clave, "aviso_esc", b); e == nil {
				p.Aviso = string(t)
			}
		}
		out = append(out, p)
	}
	return out
}

// ListarTiposAdv devuelve los tipos (con su escalera) por posición. Siembra los de fábrica la primera vez.
func (a *Almacen) ListarTiposAdv(botID int64, soloActivos bool) ([]TipoAdv, error) {
	if err := a.SembrarTiposAdv(botID); err != nil {
		return nil, err
	}
	q := `SELECT id,nombre,emoji,color,posicion,caducidad_dias,regla_id,activo FROM adv_tipos WHERE bot_id=?`
	if soloActivos {
		q += ` AND activo=1`
	}
	filas, err := a.db.Query(q+` ORDER BY posicion, id`, botID)
	if err != nil {
		return nil, err
	}
	var out []TipoAdv
	for filas.Next() {
		var t TipoAdv
		var ac int
		if filas.Scan(&t.ID, &t.Nombre, &t.Emoji, &t.Color, &t.Posicion, &t.CaducidadDias, &t.ReglaID, &ac) == nil {
			t.Activo = ac == 1
			out = append(out, t)
		}
	}
	filas.Close()
	for i := range out {
		out[i].Escalera = a.Escalera(out[i].ID)
	}
	if out == nil {
		out = []TipoAdv{}
	}
	return out, nil
}

// TipoAdvPorID devuelve un tipo con su escalera, o nil.
func (a *Almacen) TipoAdvPorID(botID, id int64) *TipoAdv {
	var t TipoAdv
	var ac int
	if err := a.db.QueryRow(`SELECT id,nombre,emoji,color,posicion,caducidad_dias,regla_id,activo FROM adv_tipos WHERE bot_id=? AND id=?`, botID, id).
		Scan(&t.ID, &t.Nombre, &t.Emoji, &t.Color, &t.Posicion, &t.CaducidadDias, &t.ReglaID, &ac); err != nil {
		return nil
	}
	t.Activo = ac == 1
	t.Escalera = a.Escalera(t.ID)
	return &t
}

// BorrarTipoAdv borra un tipo y su escalera; las advertencias que tenían ese tipo pasan a «sin tipo» (conservan su caducidad).
func (a *Almacen) BorrarTipoAdv(botID, id int64) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM adv_escalera WHERE tipo_id IN (SELECT id FROM adv_tipos WHERE bot_id=? AND id=?)`, botID, id); err != nil {
		return err
	}
	r, err := tx.Exec(`DELETE FROM adv_tipos WHERE bot_id=? AND id=?`, botID, id)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return errors.New("ese tipo no existe")
	}
	if _, err := tx.Exec(`UPDATE advertencias SET tipo_id=0 WHERE bot_id=? AND tipo_id=?`, botID, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	a.avisa("adv_tipos")
	return nil
}

// CaducidadDias da los días que vale una advertencia nueva de ese tipo (0 = no caduca). Tipo 0 o sin la suya = el ajuste general.
func (a *Almacen) CaducidadDias(botID, tipoID int64) int {
	if tipoID > 0 {
		var d int
		if err := a.db.QueryRow(`SELECT caducidad_dias FROM adv_tipos WHERE bot_id=? AND id=?`, botID, tipoID).Scan(&d); err == nil && d >= 0 {
			return d
		}
	}
	d := a.AjusteBotInt(botID, "adv_caducidad_dias")
	if d < 0 {
		d = 0
	}
	return d
}

// BorrarAdvertenciasVencidas borra las advertencias que caducaron hace más de `gracia` (para que el historial no crezca sin fin).
// Devuelve cuántas. Con gracia < 0 no hace nada.
func (a *Almacen) BorrarAdvertenciasVencidas(botID int64, ahora time.Time, gracia time.Duration) int {
	if gracia < 0 {
		return 0
	}
	r, err := a.db.Exec(`DELETE FROM advertencias WHERE bot_id=? AND caduca_ts>0 AND caduca_ts<?`, botID, ahora.Add(-gracia).Unix())
	if err != nil {
		return 0
	}
	n, _ := r.RowsAffected()
	return int(n)
}

// ---------- reglas del grupo como lista ----------

// ReglaItem es una regla del grupo (grupo "" = regla global).
type ReglaItem struct {
	ID     int64  `json:"id"`
	Grupo  string `json:"grupo"`
	Pos    int    `json:"pos"`
	Titulo string `json:"titulo"`
	Texto  string `json:"texto"`
}

// ListarReglaItems devuelve las reglas de un grupo (o las globales con grupo = ""), en orden.
func (a *Almacen) ListarReglaItems(botID int64, grupo string) ([]ReglaItem, error) {
	filas, err := a.db.Query(`SELECT id,grupo,pos,titulo_cifrado,texto_cifrado FROM grupo_regla_items WHERE bot_id=? AND grupo=? ORDER BY pos,id`, botID, grupo)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []ReglaItem{}
	for filas.Next() {
		var r ReglaItem
		var t, x []byte
		if filas.Scan(&r.ID, &r.Grupo, &r.Pos, &t, &x) != nil {
			continue
		}
		if len(t) > 0 {
			if b, e := cifrado.AbrirTexto(a.clave, "titulo_regla", t); e == nil {
				r.Titulo = string(b)
			}
		}
		if len(x) > 0 {
			if b, e := cifrado.AbrirTexto(a.clave, "texto_regla", x); e == nil {
				r.Texto = string(b)
			}
		}
		out = append(out, r)
	}
	return out, filas.Err()
}

// ReglaItemPorID devuelve una regla, o nil.
func (a *Almacen) ReglaItemPorID(botID, id int64) *ReglaItem {
	var r ReglaItem
	var t, x []byte
	if err := a.db.QueryRow(`SELECT id,grupo,pos,titulo_cifrado,texto_cifrado FROM grupo_regla_items WHERE bot_id=? AND id=?`, botID, id).Scan(&r.ID, &r.Grupo, &r.Pos, &t, &x); err != nil {
		return nil
	}
	if b, e := cifrado.AbrirTexto(a.clave, "titulo_regla", t); e == nil {
		r.Titulo = string(b)
	}
	if b, e := cifrado.AbrirTexto(a.clave, "texto_regla", x); e == nil {
		r.Texto = string(b)
	}
	return &r
}

// NumeroDeRegla da la posición (1, 2, 3…) de una regla dentro de su lista, o 0 si no existe.
func (a *Almacen) NumeroDeRegla(botID, id int64) int {
	r := a.ReglaItemPorID(botID, id)
	if r == nil {
		return 0
	}
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM grupo_regla_items WHERE bot_id=? AND grupo=? AND (pos<? OR (pos=? AND id<=?))`, botID, r.Grupo, r.Pos, r.Pos, r.ID).Scan(&n)
	return n
}

// GuardarReglaItem crea (ID = 0) o edita una regla. Pos 0 en una nueva = al final. Después regenera el texto de !reglas del grupo.
func (a *Almacen) GuardarReglaItem(botID int64, r ReglaItem, ahora time.Time) (int64, error) {
	r.Titulo, r.Texto = strings.TrimSpace(r.Titulo), strings.TrimSpace(r.Texto)
	if r.Titulo == "" && r.Texto == "" {
		return 0, errors.New("la regla está vacía")
	}
	if len([]rune(r.Titulo)) > 80 {
		return 0, errors.New("el título es muy largo (máximo 80 letras)")
	}
	if len([]rune(r.Texto)) > 1000 {
		return 0, errors.New("la regla es muy larga (máximo 1000 letras)")
	}
	if r.Grupo != "" && !strings.HasSuffix(r.Grupo, "@g.us") {
		return 0, errors.New("el grupo no es válido")
	}
	t, err := cifrado.SellarTexto(a.clave, "titulo_regla", []byte(r.Titulo))
	if err != nil {
		return 0, err
	}
	x, err := cifrado.SellarTexto(a.clave, "texto_regla", []byte(r.Texto))
	if err != nil {
		return 0, err
	}
	if r.ID == 0 {
		if r.Pos == 0 {
			_ = a.db.QueryRow(`SELECT COALESCE(MAX(pos),0)+1 FROM grupo_regla_items WHERE bot_id=? AND grupo=?`, botID, r.Grupo).Scan(&r.Pos)
		}
		res, err := a.db.Exec(`INSERT INTO grupo_regla_items(bot_id,grupo,pos,titulo_cifrado,texto_cifrado) VALUES(?,?,?,?,?)`, botID, r.Grupo, r.Pos, t, x)
		if err != nil {
			return 0, err
		}
		r.ID, _ = res.LastInsertId()
	} else {
		old := a.ReglaItemPorID(botID, r.ID)
		if old == nil {
			return 0, errors.New("esa regla no existe")
		}
		if r.Pos == 0 {
			r.Pos = old.Pos
		}
		if _, err := a.db.Exec(`UPDATE grupo_regla_items SET pos=?, titulo_cifrado=?, texto_cifrado=? WHERE bot_id=? AND id=?`, r.Pos, t, x, botID, r.ID); err != nil {
			return 0, err
		}
		r.Grupo = old.Grupo
	}
	a.regenerarTextoReglas(botID, r.Grupo, ahora)
	a.avisa("reglas")
	return r.ID, nil
}

// BorrarReglaItem quita una regla (las advertencias y tipos ligados a ella quedan sin regla).
func (a *Almacen) BorrarReglaItem(botID, id int64, ahora time.Time) error {
	old := a.ReglaItemPorID(botID, id)
	if old == nil {
		return errors.New("esa regla no existe")
	}
	if _, err := a.db.Exec(`DELETE FROM grupo_regla_items WHERE bot_id=? AND id=?`, botID, id); err != nil {
		return err
	}
	_, _ = a.db.Exec(`UPDATE advertencias SET regla_id=0 WHERE bot_id=? AND regla_id=?`, botID, id)
	_, _ = a.db.Exec(`UPDATE adv_tipos SET regla_id=0 WHERE bot_id=? AND regla_id=?`, botID, id)
	a.regenerarTextoReglas(botID, old.Grupo, ahora)
	a.avisa("reglas")
	return nil
}

// MoverReglaItem sube (-1) o baja (+1) una regla dentro de su lista.
func (a *Almacen) MoverReglaItem(botID, id int64, delta int, ahora time.Time) error {
	old := a.ReglaItemPorID(botID, id)
	if old == nil {
		return errors.New("esa regla no existe")
	}
	lista, err := a.ListarReglaItems(botID, old.Grupo)
	if err != nil {
		return err
	}
	i := -1
	for k, x := range lista {
		if x.ID == id {
			i = k
		}
	}
	j := i + delta
	if i < 0 || j < 0 || j >= len(lista) {
		return nil
	}
	lista[i], lista[j] = lista[j], lista[i]
	for k, x := range lista {
		if _, err := a.db.Exec(`UPDATE grupo_regla_items SET pos=? WHERE bot_id=? AND id=?`, k+1, botID, x.ID); err != nil {
			return err
		}
	}
	a.regenerarTextoReglas(botID, old.Grupo, ahora)
	a.avisa("reglas")
	return nil
}

// TextoReglasItems arma el texto de las reglas del grupo a partir de la lista: «1. Título: texto».
func (a *Almacen) TextoReglasItems(botID int64, grupo string) string {
	lista, _ := a.ListarReglaItems(botID, grupo)
	var b strings.Builder
	for i, r := range lista {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%d. ", i+1)
		switch {
		case r.Titulo != "" && r.Texto != "":
			b.WriteString(r.Titulo + ": " + r.Texto)
		case r.Titulo != "":
			b.WriteString(r.Titulo)
		default:
			b.WriteString(r.Texto)
		}
	}
	return b.String()
}

// regenerarTextoReglas deja en grupo_reglas el texto que usa !reglas (compatibilidad). Solo se llama cuando cambia la lista, así
// que si la lista queda vacía el texto también (era el generado). Las reglas globales (grupo "") no tienen texto de !reglas.
func (a *Almacen) regenerarTextoReglas(botID int64, grupo string, ahora time.Time) {
	if grupo == "" {
		return
	}
	_ = a.FijarReglasGrupo(botID, grupo, a.TextoReglasItems(botID, grupo), ahora)
}
