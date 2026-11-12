package almacen

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"botchat/internal/cifrado"
)

// Silenciar en grupos (v0.6.0, E5): el bot BORRA lo que escribe una persona en uno, varios o todos los grupos que administra.
// No confundir con persona_silencio (que el bot deje de CONTESTARLE; desde la 0.6.0 se llama "ignorar").

// Alcances de un silencio de grupo.
const (
	SilUno    = "uno"
	SilVarios = "varios"
	SilTodos  = "todos"
)

// Cómo terminó un silencio.
const (
	FinVencio      = "vencio"
	FinQuitado     = "quitado"
	FinReemplazado = "reemplazado"
	FinOlvidado    = "olvidado"
)

// SilencioG es un silencio de una persona en grupos.
type SilencioG struct {
	ID        int64    `json:"id"`
	Persona   string   `json:"persona"`
	Alcance   string   `json:"alcance"`
	Grupos    []string `json:"grupos"`
	HastaTS   int64    `json:"hastaTs"` // 0 = para siempre
	CreadoTS  int64    `json:"creadoTs"`
	Por       string   `json:"por"`
	Origen    string   `json:"origen"` // manual | escalera | tope
	AvisoTS   int64    `json:"avisoTs"`
	Avisar    bool     `json:"avisar"`
	Texto     string   `json:"texto"` // texto propio del aviso ("" = el de siempre)
	Activo    bool     `json:"activo"`
	FinTS     int64    `json:"finTs"`
	FinMotivo string   `json:"finMotivo"`
}

// Cubre dice si el silencio vale en ese grupo.
func (s *SilencioG) Cubre(grupo string) bool {
	if s.Alcance == SilTodos {
		return true
	}
	for _, g := range s.Grupos {
		if g == grupo {
			return true
		}
	}
	return false
}

const colsSil = "id,persona,alcance,hasta_ts,creado_ts,por,origen,aviso_ts,avisar,texto_cifrado,activo,fin_ts,fin_motivo"

func (a *Almacen) leerSilencios(q string, args ...any) ([]SilencioG, error) {
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	var out []SilencioG
	for filas.Next() {
		var x SilencioG
		var t []byte
		var av, ac int
		if filas.Scan(&x.ID, &x.Persona, &x.Alcance, &x.HastaTS, &x.CreadoTS, &x.Por, &x.Origen, &x.AvisoTS, &av, &t, &ac, &x.FinTS, &x.FinMotivo) != nil {
			continue
		}
		x.Avisar, x.Activo, x.Grupos = av == 1, ac == 1, []string{}
		if len(t) > 0 {
			if b, e := cifrado.AbrirTexto(a.clave, "texto_silencio", t); e == nil {
				x.Texto = string(b)
			}
		}
		out = append(out, x)
	}
	filas.Close()
	for i := range out {
		gs, err := a.db.Query(`SELECT grupo FROM silencio_grupo_chats WHERE silencio_id=? ORDER BY grupo`, out[i].ID)
		if err != nil {
			return out, err
		}
		for gs.Next() {
			var g string
			if gs.Scan(&g) == nil {
				out[i].Grupos = append(out[i].Grupos, g)
			}
		}
		gs.Close()
	}
	return out, nil
}

// CrearSilencioGrupo guarda un silencio nuevo de la persona (canónica). Si ya tenía uno activo, el anterior queda como
// «reemplazado» (sin aviso de vencimiento). hasta = 0 es para siempre.
func (a *Almacen) CrearSilencioGrupo(botID int64, persona, alcance string, grupos []string, hasta int64, por, origen, texto string, avisar bool, ahora time.Time) (int64, error) {
	persona = a.ResolverPersona(botID, persona)
	if persona == "" || persona == PersonaDueno {
		return 0, ErrRolDueno
	}
	switch alcance {
	case SilUno, SilVarios, SilTodos:
	default:
		return 0, errors.New("el alcance debe ser uno, varios o todos")
	}
	var gs []string
	vistos := map[string]bool{}
	for _, g := range grupos {
		if g = strings.TrimSpace(g); g != "" && !vistos[g] {
			vistos[g] = true
			gs = append(gs, g)
		}
	}
	if alcance == SilUno && len(gs) != 1 {
		return 0, errors.New("elige un grupo")
	}
	if alcance == SilVarios && len(gs) == 0 {
		return 0, errors.New("elige al menos un grupo")
	}
	if alcance == SilTodos {
		gs = nil
	}
	if hasta < 0 || (hasta > 0 && hasta <= ahora.Unix()) {
		return 0, errors.New("ese momento ya pasó")
	}
	texto = strings.TrimSpace(texto)
	if len([]rune(texto)) > 500 {
		return 0, errors.New("el texto es muy largo (máximo 500 letras)")
	}
	var sel []byte
	if texto != "" {
		var err error
		if sel, err = cifrado.SellarTexto(a.clave, "texto_silencio", []byte(texto)); err != nil {
			return 0, err
		}
	}
	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE silencio_grupo SET activo=0, fin_ts=?, fin_motivo=? WHERE bot_id=? AND persona=? AND activo=1`, ahora.Unix(), FinReemplazado, botID, persona); err != nil {
		return 0, err
	}
	r, err := tx.Exec(`INSERT INTO silencio_grupo(bot_id,persona,alcance,hasta_ts,creado_ts,por,origen,avisar,texto_cifrado,activo) VALUES(?,?,?,?,?,?,?,?,?,1)`,
		botID, persona, alcance, hasta, ahora.Unix(), por, origen, b2i(avisar), sel)
	if err != nil {
		return 0, err
	}
	id, _ := r.LastInsertId()
	for _, g := range gs {
		if _, err := tx.Exec(`INSERT INTO silencio_grupo_chats(silencio_id,grupo) VALUES(?,?)`, id, g); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	a.RefrescarBanderas(botID, persona, ahora)
	a.avisa("silencios")
	a.avisa("persona:" + persona)
	return id, nil
}

// SilenciosActivos devuelve los silencios vigentes (de una persona, o de todas si persona = ""), el más nuevo primero.
func (a *Almacen) SilenciosActivos(botID int64, persona string, ahora time.Time) ([]SilencioG, error) {
	q := `SELECT ` + colsSil + ` FROM silencio_grupo WHERE bot_id=? AND activo=1 AND (hasta_ts=0 OR hasta_ts>?)`
	args := []any{botID, ahora.Unix()}
	if persona != "" {
		q += ` AND persona=?`
		args = append(args, persona)
	}
	return a.leerSilencios(q+` ORDER BY id DESC`, args...)
}

// SilencioPorID devuelve un silencio (activo o no), o nil.
func (a *Almacen) SilencioPorID(botID, id int64) (*SilencioG, error) {
	xs, err := a.leerSilencios(`SELECT `+colsSil+` FROM silencio_grupo WHERE bot_id=? AND id=?`, botID, id)
	if err != nil || len(xs) == 0 {
		return nil, err
	}
	return &xs[0], nil
}

// HistorialSilencios devuelve los últimos silencios de la persona (también los terminados), el más nuevo primero.
func (a *Almacen) HistorialSilencios(botID int64, persona string, limite int) ([]SilencioG, error) {
	if limite <= 0 {
		limite = 50
	}
	return a.leerSilencios(`SELECT `+colsSil+` FROM silencio_grupo WHERE bot_id=? AND persona=? ORDER BY id DESC LIMIT ?`, botID, persona, limite)
}

// NumSilenciosActivos cuenta los silencios activos (con o sin vencer): sirve para saltarse todo el trabajo cuando no hay ninguno.
func (a *Almacen) NumSilenciosActivos(botID int64) int {
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM silencio_grupo WHERE bot_id=? AND activo=1`, botID).Scan(&n)
	return n
}

// FinalizarSilencio da por terminado un silencio. Devuelve si estaba activo.
func (a *Almacen) FinalizarSilencio(botID, id int64, ahora time.Time, motivo string) (bool, error) {
	var persona string
	if err := a.db.QueryRow(`SELECT persona FROM silencio_grupo WHERE bot_id=? AND id=? AND activo=1`, botID, id).Scan(&persona); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if _, err := a.db.Exec(`UPDATE silencio_grupo SET activo=0, fin_ts=?, fin_motivo=? WHERE bot_id=? AND id=?`, ahora.Unix(), motivo, botID, id); err != nil {
		return false, err
	}
	a.RefrescarBanderas(botID, persona, ahora)
	a.avisa("silencios")
	a.avisa("persona:" + persona)
	return true, nil
}

// QuitarSilenciosGrupo termina todos los silencios activos de la persona. Devuelve cuántos había.
func (a *Almacen) QuitarSilenciosGrupo(botID int64, persona string, ahora time.Time, motivo string) (int, error) {
	r, err := a.db.Exec(`UPDATE silencio_grupo SET activo=0, fin_ts=?, fin_motivo=? WHERE bot_id=? AND persona=? AND activo=1`, ahora.Unix(), motivo, botID, persona)
	if err != nil {
		return 0, err
	}
	n, _ := r.RowsAffected()
	if n > 0 {
		a.RefrescarBanderas(botID, persona, ahora)
		a.avisa("silencios")
		a.avisa("persona:" + persona)
	}
	return int(n), nil
}

// SilenciosVencidos devuelve los silencios activos con hora que ya llegó.
func (a *Almacen) SilenciosVencidos(botID int64, ahora time.Time) ([]SilencioG, error) {
	return a.leerSilencios(`SELECT `+colsSil+` FROM silencio_grupo WHERE bot_id=? AND activo=1 AND hasta_ts>0 AND hasta_ts<=? ORDER BY hasta_ts`, botID, ahora.Unix())
}

// ProximoVencimientoSilencio da la hora del siguiente silencio que vence (0 = ninguno).
func (a *Almacen) ProximoVencimientoSilencio(botID int64) int64 {
	var t sql.NullInt64
	_ = a.db.QueryRow(`SELECT MIN(hasta_ts) FROM silencio_grupo WHERE bot_id=? AND activo=1 AND hasta_ts>0`, botID).Scan(&t)
	return t.Int64
}

// AlargarSilencio cambia la hora en que termina un silencio activo (0 = para siempre).
func (a *Almacen) AlargarSilencio(botID, id, hasta int64, ahora time.Time) error {
	var persona string
	if err := a.db.QueryRow(`SELECT persona FROM silencio_grupo WHERE bot_id=? AND id=? AND activo=1`, botID, id).Scan(&persona); err != nil {
		return err
	}
	if _, err := a.db.Exec(`UPDATE silencio_grupo SET hasta_ts=? WHERE bot_id=? AND id=?`, hasta, botID, id); err != nil {
		return err
	}
	a.avisa("silencios")
	a.avisa("persona:" + persona)
	return nil
}

// MarcarAvisoSilencio anota que ya se le avisó a la persona al empezar.
func (a *Almacen) MarcarAvisoSilencio(botID, id int64, ts int64) {
	_, _ = a.db.Exec(`UPDATE silencio_grupo SET aviso_ts=? WHERE bot_id=? AND id=?`, ts, botID, id)
}

// ---------- contador de intentos ----------

// ConteoSil es el contador de intentos (mensajes borrados) de una persona en un día lógico.
type ConteoSil struct {
	Persona   string `json:"persona"`
	Grupo     string `json:"grupo"` // "" = total
	Dia       string `json:"dia"`
	N         int    `json:"n"`
	Avisos    int    `json:"avisos"`
	UltimoAv  int64  `json:"ultimoAvisoTs"`
	SancionTS int64  `json:"sancionTs"`
}

// SumarIntento cuenta un mensaje borrado: en el total de la persona y en el de ese grupo. Devuelve los dos números nuevos.
func (a *Almacen) SumarIntento(botID int64, persona, grupo, dia string) (total, delGrupo int, err error) {
	tx, err := a.db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()
	for _, g := range []string{"", grupo} {
		if _, err := tx.Exec(`INSERT INTO silencio_conteo(bot_id,persona,grupo,dia_logico,n) VALUES(?,?,?,?,1)
			ON CONFLICT(bot_id,persona,grupo,dia_logico) DO UPDATE SET n=n+1`, botID, persona, g, dia); err != nil {
			return 0, 0, err
		}
		if g == grupo && g == "" {
			break
		}
	}
	_ = tx.QueryRow(`SELECT n FROM silencio_conteo WHERE bot_id=? AND persona=? AND grupo='' AND dia_logico=?`, botID, persona, dia).Scan(&total)
	_ = tx.QueryRow(`SELECT n FROM silencio_conteo WHERE bot_id=? AND persona=? AND grupo=? AND dia_logico=?`, botID, persona, grupo, dia).Scan(&delGrupo)
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	a.avisa("persona:" + persona)
	return total, delGrupo, nil
}

// ConteoDe lee un contador (en cero si no hay fila).
func (a *Almacen) ConteoDe(botID int64, persona, grupo, dia string) ConteoSil {
	c := ConteoSil{Persona: persona, Grupo: grupo, Dia: dia}
	_ = a.db.QueryRow(`SELECT n,avisos,ultimo_aviso_ts,sancion_ts FROM silencio_conteo WHERE bot_id=? AND persona=? AND grupo=? AND dia_logico=?`,
		botID, persona, grupo, dia).Scan(&c.N, &c.Avisos, &c.UltimoAv, &c.SancionTS)
	return c
}

// ConteosDePersona devuelve los contadores de la persona de ese día (el total primero).
func (a *Almacen) ConteosDePersona(botID int64, persona, dia string) []ConteoSil {
	filas, err := a.db.Query(`SELECT grupo,n,avisos,ultimo_aviso_ts,sancion_ts FROM silencio_conteo WHERE bot_id=? AND persona=? AND dia_logico=? AND grupo NOT LIKE 'na:%' ORDER BY grupo`, botID, persona, dia)
	if err != nil {
		return nil
	}
	defer filas.Close()
	var out []ConteoSil
	for filas.Next() {
		c := ConteoSil{Persona: persona, Dia: dia}
		if filas.Scan(&c.Grupo, &c.N, &c.Avisos, &c.UltimoAv, &c.SancionTS) == nil {
			out = append(out, c)
		}
	}
	return out
}

// SumarAvisoConteo cuenta un aviso mandado en la fila indicada (la crea si no existe) y devuelve cuántos lleva.
func (a *Almacen) SumarAvisoConteo(botID int64, persona, grupo, dia string, ts int64) int {
	_, _ = a.db.Exec(`INSERT INTO silencio_conteo(bot_id,persona,grupo,dia_logico,avisos,ultimo_aviso_ts) VALUES(?,?,?,?,1,?)
		ON CONFLICT(bot_id,persona,grupo,dia_logico) DO UPDATE SET avisos=avisos+1, ultimo_aviso_ts=excluded.ultimo_aviso_ts`, botID, persona, grupo, dia, ts)
	var n int
	_ = a.db.QueryRow(`SELECT avisos FROM silencio_conteo WHERE bot_id=? AND persona=? AND grupo=? AND dia_logico=?`, botID, persona, grupo, dia).Scan(&n)
	return n
}

// MarcarSancionConteo anota que ya se aplicó la sanción de ese contador (no se repite el mismo día).
func (a *Almacen) MarcarSancionConteo(botID int64, persona, grupo, dia string, ts int64) {
	_, _ = a.db.Exec(`INSERT INTO silencio_conteo(bot_id,persona,grupo,dia_logico,sancion_ts) VALUES(?,?,?,?,?)
		ON CONFLICT(bot_id,persona,grupo,dia_logico) DO UPDATE SET sancion_ts=excluded.sancion_ts`, botID, persona, grupo, dia, ts)
}

// PonerConteoCero deja el contador de intentos del día en cero (todos los de la persona si grupo = "*", el total y el de un grupo si no).
// Conserva el número de avisos mandados (los topes de avisos no se reinician).
func (a *Almacen) PonerConteoCero(botID int64, persona, grupo, dia string) error {
	q := `UPDATE silencio_conteo SET n=0, sancion_ts=0 WHERE bot_id=? AND persona=? AND dia_logico=? AND grupo NOT LIKE 'na:%'`
	args := []any{botID, persona, dia}
	if grupo != "*" {
		q += ` AND (grupo='' OR grupo=?)`
		args = append(args, grupo)
	}
	_, err := a.db.Exec(q, args...)
	if err == nil {
		a.avisa("persona:" + persona)
	}
	return err
}

// LimpiarConteosSilencio borra los contadores de días anteriores al indicado (AAAA-MM-DD).
func (a *Almacen) LimpiarConteosSilencio(botID int64, antesDe string) {
	_, _ = a.db.Exec(`DELETE FROM silencio_conteo WHERE bot_id=? AND dia_logico<?`, botID, antesDe)
}

// ---------- textos editables ----------

// TextoSilencio devuelve un texto editable guardado ("" y false si nunca se editó).
func (a *Almacen) TextoSilencio(botID int64, id string) (string, bool) {
	var b []byte
	if err := a.db.QueryRow(`SELECT texto_cifrado FROM silencio_textos WHERE bot_id=? AND texto_id=?`, botID, id).Scan(&b); err != nil {
		return "", false
	}
	t, err := cifrado.AbrirTexto(a.clave, "texto_silencio", b)
	if err != nil {
		return "", false
	}
	return string(t), true
}

// FijarTextoSilencio guarda un texto editable ("" vuelve al de fábrica).
func (a *Almacen) FijarTextoSilencio(botID int64, id, texto string) error {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		_, err := a.db.Exec(`DELETE FROM silencio_textos WHERE bot_id=? AND texto_id=?`, botID, id)
		return err
	}
	if len([]rune(texto)) > 500 {
		return errors.New("el texto es muy largo (máximo 500 letras)")
	}
	b, err := cifrado.SellarTexto(a.clave, "texto_silencio", []byte(texto))
	if err != nil {
		return err
	}
	_, err = a.db.Exec(`INSERT INTO silencio_textos(bot_id,texto_id,texto_cifrado) VALUES(?,?,?)
		ON CONFLICT(bot_id,texto_id) DO UPDATE SET texto_cifrado=excluded.texto_cifrado`, botID, id, b)
	return err
}

// ---------- sanciones que esperan confirmación ----------

// Estados de una sanción pendiente.
const (
	SancionPendiente = "pendiente"
	SancionHecha     = "hecha"
	SancionRechazada = "rechazada"
	SancionFallida   = "fallida"
)

// SancionPend es una sanción (por ahora, expulsar) que espera mi confirmación con un toque.
type SancionPend struct {
	ID        int64  `json:"id"`
	Origen    string `json:"origen"` // silencio | advertencia
	Persona   string `json:"persona"`
	Grupo     string `json:"grupo"`
	Accion    string `json:"accion"`
	AvisoID   int64  `json:"avisoId"`
	CreadoTS  int64  `json:"creadoTs"`
	Estado    string `json:"estado"`
	ResueltoT int64  `json:"resueltoTs"`
	Resultado string `json:"resultado"`
}

// CrearSancionPendiente anota una sanción por confirmar. Si ya hay una pendiente igual (misma persona, grupo y acción) devuelve esa.
func (a *Almacen) CrearSancionPendiente(botID int64, origen, persona, grupo, accion string, avisoID int64, ahora time.Time) (int64, bool, error) {
	var id int64
	if err := a.db.QueryRow(`SELECT id FROM sancion_pendiente WHERE bot_id=? AND persona=? AND grupo=? AND accion=? AND estado=?`,
		botID, persona, grupo, accion, SancionPendiente).Scan(&id); err == nil {
		return id, false, nil
	}
	r, err := a.db.Exec(`INSERT INTO sancion_pendiente(bot_id,origen,persona,grupo,accion,aviso_id,creado_ts) VALUES(?,?,?,?,?,?,?)`,
		botID, origen, persona, grupo, accion, avisoID, ahora.Unix())
	if err != nil {
		return 0, false, err
	}
	id, _ = r.LastInsertId()
	a.avisa("sanciones")
	return id, true, nil
}

// PonerAvisoSancion liga la sanción con el aviso (notificación) que se le creó al dueño.
func (a *Almacen) PonerAvisoSancion(botID, id, avisoID int64) {
	_, _ = a.db.Exec(`UPDATE sancion_pendiente SET aviso_id=? WHERE bot_id=? AND id=?`, avisoID, botID, id)
}

func (a *Almacen) leerSanciones(q string, args ...any) ([]SancionPend, error) {
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []SancionPend{}
	for filas.Next() {
		var x SancionPend
		if filas.Scan(&x.ID, &x.Origen, &x.Persona, &x.Grupo, &x.Accion, &x.AvisoID, &x.CreadoTS, &x.Estado, &x.ResueltoT, &x.Resultado) == nil {
			out = append(out, x)
		}
	}
	return out, filas.Err()
}

const colsSanc = "id,origen,persona,grupo,accion,aviso_id,creado_ts,estado,resuelto_ts,resultado"

// ObtenerSancion devuelve una sanción, o nil.
func (a *Almacen) ObtenerSancion(botID, id int64) (*SancionPend, error) {
	xs, err := a.leerSanciones(`SELECT `+colsSanc+` FROM sancion_pendiente WHERE bot_id=? AND id=?`, botID, id)
	if err != nil || len(xs) == 0 {
		return nil, err
	}
	return &xs[0], nil
}

// ListarSancionesPendientes devuelve las que esperan confirmación, la más vieja primero.
func (a *Almacen) ListarSancionesPendientes(botID int64) ([]SancionPend, error) {
	return a.leerSanciones(`SELECT `+colsSanc+` FROM sancion_pendiente WHERE bot_id=? AND estado=? ORDER BY id`, botID, SancionPendiente)
}

// ResolverSancion cierra una sanción pendiente con el estado y el resultado dados. Devuelve false si ya no estaba pendiente.
func (a *Almacen) ResolverSancion(botID, id int64, estado, resultado string, ahora time.Time) (bool, error) {
	r, err := a.db.Exec(`UPDATE sancion_pendiente SET estado=?, resuelto_ts=?, resultado=? WHERE bot_id=? AND id=? AND estado=?`,
		estado, ahora.Unix(), resultado, botID, id, SancionPendiente)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	if n > 0 {
		a.avisa("sanciones")
	}
	return n > 0, nil
}

// ---------- banderas de grupo_miembros ----------

// RefrescarBanderas pone al día grupo_miembros.silenciado y .advertido de una persona (o de todas si persona = "").
func (a *Almacen) RefrescarBanderas(botID int64, persona string, ahora time.Time) {
	filtro, extra := "", []any{}
	if persona != "" {
		filtro, extra = " AND persona=?", []any{persona}
	}
	t := ahora.Unix()
	args := append([]any{t, botID}, extra...)
	_, _ = a.db.Exec(`UPDATE grupo_miembros SET silenciado = CASE WHEN EXISTS (
			SELECT 1 FROM silencio_grupo s WHERE s.bot_id=grupo_miembros.bot_id AND s.persona=grupo_miembros.persona AND s.activo=1
			AND (s.hasta_ts=0 OR s.hasta_ts>?)
			AND (s.alcance='todos' OR EXISTS (SELECT 1 FROM silencio_grupo_chats c WHERE c.silencio_id=s.id AND c.grupo=grupo_miembros.grupo))
		) THEN 1 ELSE 0 END WHERE bot_id=?`+filtro, args...)
	_, _ = a.db.Exec(`UPDATE grupo_miembros SET advertido = CASE WHEN EXISTS (
			SELECT 1 FROM advertencias v WHERE v.bot_id=grupo_miembros.bot_id AND v.persona=grupo_miembros.persona AND (v.caduca_ts=0 OR v.caduca_ts>?)
		) THEN 1 ELSE 0 END WHERE bot_id=?`+filtro, args...)
}

// limpiaSilencioDe borra todo lo del silencio de grupos de esas formas de persona (al olvidarla).
func (a *Almacen) limpiaSilencioDe(botID int64, lista string, fs []string) {
	_, _ = a.db.Exec(`DELETE FROM silencio_grupo WHERE bot_id=? AND persona IN `+lista, args2(botID, fs)...)
	_, _ = a.db.Exec(`DELETE FROM silencio_conteo WHERE bot_id=? AND persona IN `+lista, args2(botID, fs)...)
	_, _ = a.db.Exec(`DELETE FROM sancion_pendiente WHERE bot_id=? AND persona IN `+lista, args2(botID, fs)...)
}

// fusionaSilencioDe pasa lo del silencio de grupos de una forma de persona a la canónica (dentro de la transacción de la fusión).
func fusionaSilencioDe(tx *sql.Tx, botID int64, desde, hacia string) error {
	for _, q := range []string{
		`UPDATE silencio_grupo SET persona=? WHERE bot_id=? AND persona=?`,
		`UPDATE sancion_pendiente SET persona=? WHERE bot_id=? AND persona=?`,
	} {
		if _, err := tx.Exec(q, hacia, botID, desde); err != nil {
			return err
		}
	}
	// los contadores: se suman los del mismo día y grupo
	if _, err := tx.Exec(`INSERT INTO silencio_conteo(bot_id,persona,grupo,dia_logico,n,avisos,ultimo_aviso_ts,sancion_ts)
		SELECT bot_id,?,grupo,dia_logico,n,avisos,ultimo_aviso_ts,sancion_ts FROM silencio_conteo WHERE bot_id=? AND persona=?
		ON CONFLICT(bot_id,persona,grupo,dia_logico) DO UPDATE SET n=silencio_conteo.n+excluded.n, avisos=silencio_conteo.avisos+excluded.avisos`, hacia, botID, desde); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM silencio_conteo WHERE bot_id=? AND persona=?`, botID, desde)
	return err
}

var _ = fmt.Sprint

// GrupoConMiembros es un grupo vigente de la lista local, con cuántos miembros tiene y si la cuenta propia es administradora.
type GrupoConMiembros struct {
	Grupo    string
	Miembros int
	SoyAdmin bool
}

// GruposConMiembros devuelve los grupos donde la cuenta sigue (por la lista local de miembros). `propios` son los usuarios (la parte
// antes de la @) de mi número y de mi LID; con ellos se sabe si soy administradora. Sin ellos, SoyAdmin sale siempre falso.
func (a *Almacen) GruposConMiembros(botID int64, propios []string) ([]GrupoConMiembros, error) {
	filas, err := a.db.Query(`SELECT grupo, COUNT(*) FROM grupo_miembros WHERE bot_id=? AND hasta_ts=0 GROUP BY grupo`, botID)
	if err != nil {
		return nil, err
	}
	var out []GrupoConMiembros
	for filas.Next() {
		var g GrupoConMiembros
		if filas.Scan(&g.Grupo, &g.Miembros) == nil {
			out = append(out, g)
		}
	}
	filas.Close()
	if len(propios) == 0 {
		return out, nil
	}
	adm := map[string]bool{}
	for _, u := range propios {
		fs, err := a.db.Query(`SELECT grupo FROM grupo_miembros WHERE bot_id=? AND hasta_ts=0 AND rol IN ('admin','creador') AND (jid LIKE ? OR persona LIKE ?)`, botID, u+"@%", u+"@%")
		if err != nil {
			return out, err
		}
		for fs.Next() {
			var g string
			if fs.Scan(&g) == nil {
				adm[g] = true
			}
		}
		fs.Close()
	}
	for i := range out {
		out[i].SoyAdmin = adm[out[i].Grupo]
	}
	return out, nil
}
