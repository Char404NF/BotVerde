package almacen

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"botchat/internal/cifrado"
)

// ---------- módulos (v0.5.0, H0) ----------

// ModuloFila es lo guardado de un módulo: si está activo, su configuración (ya descifrada) y sus topes propios (0 = los del módulo).
type ModuloFila struct {
	ID         string `json:"id"`
	Activo     bool   `json:"activo"`
	Config     string `json:"config"`
	TopeHora   int    `json:"topeHora"`
	TopeDia    int    `json:"topeDia"`
	ActivadoTS int64  `json:"activadoTs"`
}

// ObtenerModulo devuelve la fila del módulo (nil si nunca se guardó nada).
func (a *Almacen) ObtenerModulo(botID int64, id string) (*ModuloFila, error) {
	var f ModuloFila
	var act int
	var cfg []byte
	err := a.db.QueryRow(`SELECT id,activo,config,tope_hora,tope_dia,activado_ts FROM modulos WHERE bot_id=? AND id=?`, botID, id).
		Scan(&f.ID, &act, &cfg, &f.TopeHora, &f.TopeDia, &f.ActivadoTS)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	f.Activo = act == 1
	if len(cfg) > 0 {
		if t, err := cifrado.AbrirTexto(a.clave, "modulo_cfg", cfg); err == nil {
			f.Config = string(t)
		}
	}
	return &f, nil
}

// ListarModulosFilas devuelve las filas guardadas, por id.
func (a *Almacen) ListarModulosFilas(botID int64) (map[string]ModuloFila, error) {
	filas, err := a.db.Query(`SELECT id FROM modulos WHERE bot_id=?`, botID)
	if err != nil {
		return nil, err
	}
	var ids []string
	for filas.Next() {
		var id string
		if filas.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	filas.Close()
	out := map[string]ModuloFila{}
	for _, id := range ids {
		if f, err := a.ObtenerModulo(botID, id); err == nil && f != nil {
			out[id] = *f
		}
	}
	return out, nil
}

func (a *Almacen) asegurarModulo(botID int64, id string) error {
	_, err := a.db.Exec(`INSERT OR IGNORE INTO modulos(bot_id,id) VALUES(?,?)`, botID, id)
	return err
}

// FijarModuloActivo enciende o apaga un módulo (la configuración se conserva).
func (a *Almacen) FijarModuloActivo(botID int64, id string, activo bool, ahora time.Time) error {
	if err := a.asegurarModulo(botID, id); err != nil {
		return err
	}
	v, ts := 0, int64(0)
	if activo {
		v, ts = 1, ahora.Unix()
	}
	q := `UPDATE modulos SET activo=? WHERE bot_id=? AND id=?`
	args := []any{v, botID, id}
	if activo {
		q = `UPDATE modulos SET activo=?, activado_ts=? WHERE bot_id=? AND id=?`
		args = []any{v, ts, botID, id}
	}
	if _, err := a.db.Exec(q, args...); err != nil {
		return err
	}
	a.avisa("modulos")
	return nil
}

// GuardarModuloConfig guarda la configuración (JSON) cifrada con el cifrado de columna.
func (a *Almacen) GuardarModuloConfig(botID int64, id, cfg string) error {
	sel, err := cifrado.SellarTexto(a.clave, "modulo_cfg", []byte(cfg))
	if err != nil {
		return err
	}
	if err := a.asegurarModulo(botID, id); err != nil {
		return err
	}
	if _, err := a.db.Exec(`UPDATE modulos SET config=? WHERE bot_id=? AND id=?`, sel, botID, id); err != nil {
		return err
	}
	a.avisa("modulos")
	return nil
}

// FijarTopesModulo guarda los topes propios del módulo (0 = usar los que trae el módulo).
func (a *Almacen) FijarTopesModulo(botID int64, id string, hora, dia int) error {
	if err := a.asegurarModulo(botID, id); err != nil {
		return err
	}
	if _, err := a.db.Exec(`UPDATE modulos SET tope_hora=?, tope_dia=? WHERE bot_id=? AND id=?`, hora, dia, botID, id); err != nil {
		return err
	}
	a.avisa("modulos")
	return nil
}

// ---------- registro de iniciativas (sin textos) ----------

// RegistroModulo es un renglón del registro: qué módulo, qué hizo, cuándo y cómo salió. Nunca lleva textos de mensajes.
type RegistroModulo struct {
	ID        int64  `json:"id"`
	Modulo    string `json:"modulo"`
	Que       string `json:"que"`
	TS        int64  `json:"ts"`
	Resultado string `json:"resultado"`
	Chat      string `json:"chat"`
}

// RegistrarModulo anota una iniciativa (enviada, descartada, diferida…). `que` y `resultado` son códigos cortos.
func (a *Almacen) RegistrarModulo(botID int64, modulo, que, resultado, chat string, ts time.Time) {
	_, err := a.db.Exec(`INSERT INTO modulos_registro(bot_id,modulo,que,ts,resultado,chat) VALUES(?,?,?,?,?,?)`, botID, modulo, que, ts.Unix(), resultado, chat)
	if err == nil {
		a.avisa("iniciativas")
	}
}

// ListarRegistroModulos devuelve el registro, lo más nuevo primero. antesID > 0 pide la página siguiente; modulo "" = todos.
func (a *Almacen) ListarRegistroModulos(botID int64, modulo string, limite int, antesID int64) ([]RegistroModulo, error) {
	if limite <= 0 || limite > 500 {
		limite = 100
	}
	q := `SELECT id,modulo,que,ts,resultado,chat FROM modulos_registro WHERE bot_id=?`
	args := []any{botID}
	if modulo != "" {
		q += ` AND modulo=?`
		args = append(args, modulo)
	}
	if antesID > 0 {
		q += ` AND id<?`
		args = append(args, antesID)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limite)
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []RegistroModulo{}
	for filas.Next() {
		var x RegistroModulo
		if filas.Scan(&x.ID, &x.Modulo, &x.Que, &x.TS, &x.Resultado, &x.Chat) == nil {
			out = append(out, x)
		}
	}
	return out, filas.Err()
}

// ContarRegistroModulos cuenta renglones con ese resultado desde `desde` (modulo "" = todos). Sirve para recargar las fichas al arrancar.
func (a *Almacen) ContarRegistroModulos(botID int64, modulo, resultado string, desde time.Time) int {
	q := `SELECT COUNT(*) FROM modulos_registro WHERE bot_id=? AND resultado=? AND ts>=?`
	args := []any{botID, resultado, desde.Unix()}
	if modulo != "" {
		q += ` AND modulo=?`
		args = append(args, modulo)
	}
	var n int
	_ = a.db.QueryRow(q, args...).Scan(&n)
	return n
}

// EnvioReciente es una salida (enviada o fallida) de un módulo, para recargar las fichas al arrancar.
type EnvioReciente struct {
	Modulo string
	TS     int64
}

// EnviosRecientesModulos devuelve las iniciativas que gastaron ficha (enviadas o que fallaron) desde `desde`, de la más vieja a la más nueva.
func (a *Almacen) EnviosRecientesModulos(botID int64, desde time.Time) []EnvioReciente {
	filas, err := a.db.Query(`SELECT modulo, ts FROM modulos_registro WHERE bot_id=? AND resultado IN ('enviado','fallido') AND ts>=? ORDER BY ts`, botID, desde.Unix())
	if err != nil {
		return nil
	}
	defer filas.Close()
	var out []EnvioReciente
	for filas.Next() {
		var x EnvioReciente
		if filas.Scan(&x.Modulo, &x.TS) == nil {
			out = append(out, x)
		}
	}
	return out
}

// LimpiarRegistroModulos borra los renglones con más de `dias` días.
func (a *Almacen) LimpiarRegistroModulos(botID int64, dias int, ahora time.Time) {
	if dias < 1 {
		dias = 90
	}
	_, _ = a.db.Exec(`DELETE FROM modulos_registro WHERE bot_id=? AND ts<?`, botID, ahora.Add(-time.Duration(dias)*24*time.Hour).Unix())
}

// ---------- No molestar ----------

// NoMolestarItem es alguien a quien ninguna iniciativa le escribe.
type NoMolestarItem struct {
	Persona    string `json:"persona"`
	DesdeTS    int64  `json:"desdeTs"`
	Motivo     string `json:"motivo"`
	Confirmado bool   `json:"confirmado"`
}

// AgregarNoMolestar mete a una persona en la lista. Devuelve nuevo=true si no estaba y yaConfirmado=true si ya se le avisó antes.
func (a *Almacen) AgregarNoMolestar(botID int64, persona, motivo string, ahora time.Time) (nuevo, yaConfirmado bool, err error) {
	persona = a.ResolverPersona(botID, persona)
	if persona == "" || persona == PersonaDueno {
		return false, false, errors.New("persona no válida")
	}
	var conf int
	e := a.db.QueryRow(`SELECT confirmado FROM no_molestar WHERE bot_id=? AND persona=?`, botID, persona).Scan(&conf)
	switch {
	case e == nil:
		return false, conf == 1, nil
	case !errors.Is(e, sql.ErrNoRows):
		return false, false, e
	}
	if _, err = a.db.Exec(`INSERT INTO no_molestar(bot_id,persona,desde_ts,motivo,confirmado) VALUES(?,?,?,?,0)`, botID, persona, ahora.Unix(), motivo); err != nil {
		return false, false, err
	}
	a.avisa("iniciativas")
	return true, false, nil
}

// MarcarNoMolestarConfirmado anota que ya se le avisó a esa persona (se hace una sola vez).
func (a *Almacen) MarcarNoMolestarConfirmado(botID int64, persona string) {
	persona = a.ResolverPersona(botID, persona)
	_, _ = a.db.Exec(`UPDATE no_molestar SET confirmado=1 WHERE bot_id=? AND persona=?`, botID, persona)
}

// EstaNoMolestar dice si la persona (número o LID) está en la lista.
func (a *Almacen) EstaNoMolestar(botID int64, persona string) bool {
	if persona == "" {
		return false
	}
	persona = a.ResolverPersona(botID, persona)
	var x int
	return a.db.QueryRow(`SELECT 1 FROM no_molestar WHERE bot_id=? AND persona=?`, botID, persona).Scan(&x) == nil
}

// QuitarNoMolestar saca a una persona de la lista.
func (a *Almacen) QuitarNoMolestar(botID int64, persona string) error {
	persona = a.ResolverPersona(botID, persona)
	if _, err := a.db.Exec(`DELETE FROM no_molestar WHERE bot_id=? AND persona=?`, botID, persona); err != nil {
		return err
	}
	a.avisa("iniciativas")
	return nil
}

// ListarNoMolestar devuelve la lista, la más reciente primero.
func (a *Almacen) ListarNoMolestar(botID int64) ([]NoMolestarItem, error) {
	filas, err := a.db.Query(`SELECT persona,desde_ts,motivo,confirmado FROM no_molestar WHERE bot_id=? ORDER BY desde_ts DESC, persona`, botID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []NoMolestarItem{}
	for filas.Next() {
		var x NoMolestarItem
		var c int
		if filas.Scan(&x.Persona, &x.DesdeTS, &x.Motivo, &c) == nil {
			x.Confirmado = c == 1
			out = append(out, x)
		}
	}
	return out, filas.Err()
}

// ---------- ajustes de iniciativa (bot_ajustes, prefijo ini_) ----------

// Claves de iniciativa y sus valores por defecto. El maestro nace APAGADO.
var ajustesIniDefecto = map[string]string{
	"ini_maestro":     "0",     // interruptor general "Iniciativa propia"
	"ini_sin_limites": "1",     // 1 = sin topes, sin pausa nocturna y sin esperas: el bot manda cuando quiera; 0 = valen los ajustes de abajo
	"ini_tope_hora":   "10",    // iniciativas por hora, en total
	"ini_tope_dia":    "60",    // iniciativas por día, en total
	"ini_noche_on":    "1",     // pausa nocturna
	"ini_noche_desde": "23:00", // hora local
	"ini_noche_hasta": "07:00",
	"ini_retraso_min": "3", // segundos de espera al azar antes de cada iniciativa
	"ini_retraso_max": "12",
	"ini_errores":     "5",  // errores de envío seguidos para activar el modo seguro
	"ini_espera_max":  "30", // minutos máximos que una iniciativa espera una ficha antes de descartarse
	"ini_seguro":      "",   // "" = normal; "<ts>|<motivo>" = modo seguro activo
	"ini_baja_texto":  "Listo, ya no te escribiré por iniciativa propia. Si cambias de idea, escríbeme tú primero.",
}

// SinLimites dice si el bot está en «sin límites» (de fábrica sí): no se aplican topes por hora ni por día, pausa nocturna, esperas
// ni límites por minuto. Lo consultan el motor, los comandos, las respuestas automáticas, los stickers y la cola de iniciativa.
func (a *Almacen) SinLimites(botID int64) bool { return a.AjusteIni(botID, "ini_sin_limites") != "0" }

// ClaveIniValida dice si es un ajuste de iniciativa conocido.
func ClaveIniValida(clave string) bool { _, ok := ajustesIniDefecto[clave]; return ok }

// AjusteIni devuelve un ajuste de iniciativa (su valor por defecto si nunca se fijó).
func (a *Almacen) AjusteIni(botID int64, clave string) string {
	var v string
	if err := a.db.QueryRow(`SELECT valor FROM bot_ajustes WHERE bot_id=? AND clave=?`, botID, clave).Scan(&v); err == nil {
		return v
	}
	return ajustesIniDefecto[clave]
}

// AjusteIniInt es AjusteIni como número.
func (a *Almacen) AjusteIniInt(botID int64, clave string) int {
	n, err := strconv.Atoi(strings.TrimSpace(a.AjusteIni(botID, clave)))
	if err != nil {
		n, _ = strconv.Atoi(ajustesIniDefecto[clave])
	}
	return n
}

// FijarAjusteIni guarda un ajuste de iniciativa. Solo acepta las claves conocidas (la validación del valor la hace quien llama).
func (a *Almacen) FijarAjusteIni(botID int64, clave, valor string) error {
	if !ClaveIniValida(clave) {
		return fmt.Errorf("ajuste desconocido")
	}
	_, err := a.db.Exec(`INSERT INTO bot_ajustes(bot_id, clave, valor) VALUES(?,?,?) ON CONFLICT(bot_id,clave) DO UPDATE SET valor=excluded.valor`, botID, clave, valor)
	if err == nil {
		a.avisa("iniciativas")
	}
	return err
}

// AjustesIni devuelve todos los ajustes de iniciativa (con sus valores por defecto).
func (a *Almacen) AjustesIni(botID int64) map[string]string {
	out := map[string]string{}
	for k := range ajustesIniDefecto {
		out[k] = a.AjusteIni(botID, k)
	}
	return out
}
