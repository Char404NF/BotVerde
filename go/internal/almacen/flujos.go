package almacen

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"botchat/internal/cifrado"
)

// Estados de una conversación dentro de un menú.
const (
	SesActiva     = "activo"
	SesCompletada = "completado"
	SesAbandonada = "abandonado"
	SesHumano     = "humano"
	SesPausada    = "pausado"
	SesExpirada   = "expirado"
)

// Menu es una automatización guardada. Def es su definición en JSON (cifrada en la base).
type Menu struct {
	ID          int64  `json:"id"`
	Activo      bool   `json:"activo"`
	Sistema     bool   `json:"sistema"` // venido de fábrica (el Saludo)
	Version     int    `json:"version"`
	Orden       int    `json:"orden"`
	Actualizado int64  `json:"actualizado"`
	Def         []byte `json:"-"`
}

func (a *Almacen) sellaDef(def []byte) ([]byte, error) {
	return cifrado.SellarTexto(a.clave, "menu_def", def)
}

func (a *Almacen) abreDef(b []byte) []byte {
	t, err := cifrado.AbrirTexto(a.clave, "menu_def", b)
	if err != nil {
		return nil
	}
	return t
}

// CrearMenu guarda una automatización nueva (versión 1) al final de la lista.
func (a *Almacen) CrearMenu(botID int64, def []byte, activo, sistema bool) (int64, error) {
	sel, err := a.sellaDef(def)
	if err != nil {
		return 0, err
	}
	ahora := time.Now().Unix()
	var orden int
	_ = a.db.QueryRow(`SELECT COALESCE(MAX(orden),0)+1 FROM menus WHERE bot_id=?`, botID).Scan(&orden)
	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	r, err := tx.Exec(`INSERT INTO menus(bot_id, activo, sistema, version, orden, creado, actualizado, def_cifrado) VALUES(?,?,?,?,?,?,?,?)`, botID, b2i(activo), b2i(sistema), 1, orden, ahora, ahora, sel)
	if err != nil {
		return 0, err
	}
	id, _ := r.LastInsertId()
	if _, err := tx.Exec(`INSERT INTO menu_versiones(menu_id, version, ts, def_cifrado) VALUES(?,?,?,?)`, id, 1, ahora, sel); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	a.avisa("automatizaciones")
	return id, nil
}

// GuardarMenu reemplaza la definición de una automatización: se crea una versión nueva. Las conversaciones en curso siguen con la suya.
func (a *Almacen) GuardarMenu(botID, id int64, def []byte) (int, error) {
	sel, err := a.sellaDef(def)
	if err != nil {
		return 0, err
	}
	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var v int
	if err := tx.QueryRow(`SELECT version FROM menus WHERE bot_id=? AND id=?`, botID, id).Scan(&v); err != nil {
		return 0, err
	}
	v++
	ahora := time.Now().Unix()
	if _, err := tx.Exec(`UPDATE menus SET version=?, actualizado=?, def_cifrado=? WHERE bot_id=? AND id=?`, v, ahora, sel, botID, id); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`INSERT INTO menu_versiones(menu_id, version, ts, def_cifrado) VALUES(?,?,?,?)`, id, v, ahora, sel); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	a.avisa("automatizaciones")
	// un archivo que la versión nueva ya no usa (y que ninguna conversación abierta necesita) se borra del teléfono
	a.LimpiarAdjuntosHuerfanos(botID, time.Hour, time.Now())
	return v, nil
}

// FijarMenuActivo enciende o apaga una automatización.
func (a *Almacen) FijarMenuActivo(botID, id int64, si bool) error {
	_, err := a.db.Exec(`UPDATE menus SET activo=? WHERE bot_id=? AND id=?`, b2i(si), botID, id)
	if err == nil {
		a.avisa("automatizaciones")
	}
	return err
}

// BorrarMenu quita una automatización. Sus conversaciones en curso se cierran; el historial de estadísticas se conserva sin nombre.
func (a *Almacen) BorrarMenu(botID, id int64) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ahora := time.Now().Unix()
	if _, err := tx.Exec(`UPDATE flow_sessions SET estado=?, ultima_ts=? WHERE bot_id=? AND menu_id=? AND estado IN (?,?,?)`, SesAbandonada, ahora, botID, id, SesActiva, SesHumano, SesPausada); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM menu_versiones WHERE menu_id=?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM menus WHERE bot_id=? AND id=?`, botID, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	a.avisa("automatizaciones")
	a.LimpiarAdjuntosHuerfanos(botID, time.Hour, time.Now())
	return nil
}

// ListarMenus devuelve las automatizaciones en su orden (la de menor número se evalúa primero).
func (a *Almacen) ListarMenus(botID int64) ([]Menu, error) {
	filas, err := a.db.Query(`SELECT id, activo, sistema, version, orden, actualizado, def_cifrado FROM menus WHERE bot_id=? ORDER BY sistema, orden, id`, botID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var out []Menu
	for filas.Next() {
		var m Menu
		var act, sis int
		var d []byte
		if err := filas.Scan(&m.ID, &act, &sis, &m.Version, &m.Orden, &m.Actualizado, &d); err != nil {
			return nil, err
		}
		m.Activo, m.Sistema, m.Def = act == 1, sis == 1, a.abreDef(d)
		out = append(out, m)
	}
	return out, filas.Err()
}

// ObtenerMenu devuelve una automatización o nil.
func (a *Almacen) ObtenerMenu(botID, id int64) (*Menu, error) {
	var m Menu
	var act, sis int
	var d []byte
	err := a.db.QueryRow(`SELECT id, activo, sistema, version, orden, actualizado, def_cifrado FROM menus WHERE bot_id=? AND id=?`, botID, id).Scan(&m.ID, &act, &sis, &m.Version, &m.Orden, &m.Actualizado, &d)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.Activo, m.Sistema, m.Def = act == 1, sis == 1, a.abreDef(d)
	return &m, nil
}

// VersionDeMenu devuelve la definición de una versión concreta (nil si ya no existe).
func (a *Almacen) VersionDeMenu(menuID int64, version int) []byte {
	var d []byte
	if err := a.db.QueryRow(`SELECT def_cifrado FROM menu_versiones WHERE menu_id=? AND version=?`, menuID, version).Scan(&d); err != nil {
		return nil
	}
	return a.abreDef(d)
}

// ReordenarMenus fija el orden de evaluación según la lista de ids dada.
func (a *Almacen) ReordenarMenus(botID int64, ids []int64) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE menus SET orden=? WHERE bot_id=? AND id=?`, i+1, botID, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	a.avisa("automatizaciones")
	return nil
}

// SembrarSaludo crea la automatización "Saludo" de fábrica la primera vez (y solo esa vez, aunque la borren).
func (a *Almacen) SembrarSaludo(botID int64, def []byte) {
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM stats_info WHERE campo=?`, fmt.Sprintf("saludo_%d", botID)).Scan(&n)
	if n > 0 {
		return
	}
	if _, err := a.CrearMenu(botID, def, true, true); err == nil {
		_, _ = a.db.Exec(`INSERT INTO stats_info(campo, valor) VALUES(?, 1)`, fmt.Sprintf("saludo_%d", botID))
	}
}

// ---------- conversaciones ----------

// Sesion es una persona dentro de un menú.
type Sesion struct {
	ID          int64  `json:"id"`
	Chat        string `json:"chat"`
	Sender      string `json:"sender"`
	MenuID      int64  `json:"menuId"`
	MenuVersion int    `json:"menuVersion"`
	Paso        string `json:"paso"`
	EsperaDato  bool   `json:"esperaDato"`
	Invalidas   int    `json:"invalidas"`
	Cortado     bool   `json:"cortado"`
	InicioTS    int64  `json:"inicio"`
	UltimaTS    int64  `json:"ultima"`
	HastaTS     int64  `json:"hasta"`
	Estado      string `json:"estado"`
	Prueba      bool   `json:"prueba"`
	// Menús ampliados (v0.5.0, bloque C).
	ExpiraSeg int64  `json:"expiraSeg"` // sin actividad por este tiempo la conversación caduca en silencio (0 = el valor general)
	Pila      string `json:"pila"`      // pasos de menú por los que pasó (separados por coma) para "atrás"
	Fase      string `json:"fase"`      // en un formulario: preguntando, resumen, eligiendo, corrigiendo
	Campo     int    `json:"campo"`     // en un formulario: el campo en que va
	Reint     int    `json:"reint"`     // en un formulario: intentos fallidos del campo actual
	Cierre    string `json:"cierre"`    // por qué terminó (errores, salio, persona, cancelo…)
}

const colsSesion = "id, chat, sender, menu_id, menu_version, paso, espera_dato, invalidas, cortado, inicio_ts, ultima_ts, hasta_ts, estado, prueba, expira_seg, pila, fase, campo, reint, cierre"

func escanearSesion(s interface{ Scan(...any) error }) (*Sesion, error) {
	var x Sesion
	var esp, cor, pru int
	if err := s.Scan(&x.ID, &x.Chat, &x.Sender, &x.MenuID, &x.MenuVersion, &x.Paso, &esp, &x.Invalidas, &cor, &x.InicioTS, &x.UltimaTS, &x.HastaTS, &x.Estado, &pru, &x.ExpiraSeg, &x.Pila, &x.Fase, &x.Campo, &x.Reint, &x.Cierre); err != nil {
		return nil, err
	}
	x.EsperaDato, x.Cortado, x.Prueba = esp == 1, cor == 1, pru == 1
	return &x, nil
}

// SesionVigente devuelve la conversación abierta de esa persona en ese chat. Dentro de una transacción comprueba si ya caducó
// (sin actividad por `expira`, o su pausa terminó): en ese caso la cierra como expirada y devuelve nil.
func (a *Almacen) SesionVigente(botID int64, chat, sender string, prueba bool, ahora time.Time, expira time.Duration) (*Sesion, error) {
	tx, err := a.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	x, err := escanearSesion(tx.QueryRow(`SELECT `+colsSesion+` FROM flow_sessions WHERE bot_id=? AND chat=? AND sender=? AND prueba=? AND estado IN (?,?,?) ORDER BY id DESC LIMIT 1`,
		botID, chat, sender, b2i(prueba), SesActiva, SesHumano, SesPausada))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	vencida := false
	switch x.Estado {
	case SesActiva:
		if x.ExpiraSeg > 0 {
			expira = time.Duration(x.ExpiraSeg) * time.Second
		}
		vencida = ahora.Sub(time.Unix(x.UltimaTS, 0)) > expira
	default: // humano o pausada: duran hasta su hora
		vencida = x.HastaTS > 0 && ahora.Unix() > x.HastaTS
	}
	if vencida {
		// ultima_ts se conserva: es la última vez que la persona escribió (sirve para decidir si todavía se puede retomar)
		if _, err := tx.Exec(`UPDATE flow_sessions SET estado=? WHERE id=?`, SesExpirada, x.ID); err != nil {
			return nil, err
		}
		if err := a.sumaStat(tx, botID, x.MenuID, "", "expirado", prueba, ahora, 1); err != nil {
			return nil, err
		}
		if x.Estado == SesActiva && x.Paso != "" {
			if err := a.sumaStat(tx, botID, x.MenuID, x.Paso, "abandono", prueba, ahora, 1); err != nil {
				return nil, err
			}
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	return x, tx.Commit()
}

// NuevaSesion abre una conversación (cerrando cualquier otra abierta de la misma persona) y cuenta el inicio.
func (a *Almacen) NuevaSesion(botID int64, chat, sender string, menuID int64, version int, paso string, prueba bool, ahora time.Time) (int64, error) {
	return a.NuevaSesionConExpira(botID, chat, sender, menuID, version, paso, prueba, ahora, 0)
}

// NuevaSesionConExpira es NuevaSesion con cuánto tiempo sin actividad hace caducar la conversación (0 = el valor general).
func (a *Almacen) NuevaSesionConExpira(botID int64, chat, sender string, menuID int64, version int, paso string, prueba bool, ahora time.Time, expira time.Duration) (int64, error) {
	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE flow_sessions SET estado=?, ultima_ts=? WHERE bot_id=? AND chat=? AND sender=? AND prueba=? AND estado IN (?,?,?)`,
		SesAbandonada, ahora.Unix(), botID, chat, sender, b2i(prueba), SesActiva, SesHumano, SesPausada); err != nil {
		return 0, err
	}
	r, err := tx.Exec(`INSERT INTO flow_sessions(bot_id, chat, sender, menu_id, menu_version, paso, inicio_ts, ultima_ts, prueba, expira_seg) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		botID, chat, sender, menuID, version, paso, ahora.Unix(), ahora.Unix(), b2i(prueba), int64(expira.Seconds()))
	if err != nil {
		return 0, err
	}
	id, _ := r.LastInsertId()
	if err := a.sumaStat(tx, botID, menuID, "", "iniciado", prueba, ahora, 1); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	a.avisa("sesiones")
	return id, nil
}

// MoverSesion cambia el paso de una conversación y anota el evento (sin texto libre) y la estadística.
func (a *Almacen) MoverSesion(botID int64, s *Sesion, paso string, esperaDato bool, invalidas int, cortado bool, evento struct{ Paso, Opcion, Resultado string }, ahora time.Time) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE flow_sessions SET paso=?, espera_dato=?, invalidas=?, cortado=?, ultima_ts=? WHERE id=?`, paso, b2i(esperaDato), invalidas, b2i(cortado), ahora.Unix(), s.ID); err != nil {
		return err
	}
	if evento.Resultado != "" {
		if _, err := tx.Exec(`INSERT INTO flow_events(session_id, ts, paso, opcion, resultado) VALUES(?,?,?,?,?)`, s.ID, ahora.Unix(), evento.Paso, evento.Opcion, evento.Resultado); err != nil {
			return err
		}
		op := evento.Opcion
		if evento.Resultado == "invalida" {
			op = "invalida"
		}
		if evento.Resultado == "valida" || evento.Resultado == "invalida" {
			if err := a.sumaStat(tx, botID, s.MenuID, evento.Paso, op, s.Prueba, ahora, 1); err != nil {
				return err
			}
		}
	}
	s.Paso, s.EsperaDato, s.Invalidas, s.Cortado, s.UltimaTS = paso, esperaDato, invalidas, cortado, ahora.Unix()
	if err := tx.Commit(); err != nil {
		return err
	}
	a.avisa("sesiones")
	return nil
}

// VisitaPaso cuenta que una conversación llegó a un paso (para el embudo).
func (a *Almacen) VisitaPaso(botID int64, s *Sesion, paso string, ahora time.Time) {
	tx, err := a.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	if a.sumaStat(tx, botID, s.MenuID, paso, "visita", s.Prueba, ahora, 1) == nil {
		_ = tx.Commit()
	}
}

// CerrarSesion termina una conversación con el estado dado (completado, abandonado, humano…) y la cuenta.
func (a *Almacen) CerrarSesion(botID int64, s *Sesion, estado string, hasta int64, ahora time.Time) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE flow_sessions SET estado=?, hasta_ts=?, ultima_ts=? WHERE id=?`, estado, hasta, ahora.Unix(), s.ID); err != nil {
		return err
	}
	ev := map[string]string{SesCompletada: "completado", SesAbandonada: "abandonado", SesHumano: "humano", SesExpirada: "expirado"}[estado]
	if ev != "" {
		if err := a.sumaStat(tx, botID, s.MenuID, "", ev, s.Prueba, ahora, 1); err != nil {
			return err
		}
	}
	// dónde se quedó quien abandona (para "dónde abandonan"); el paso solo se cuenta si estaba en uno
	if (estado == SesAbandonada || estado == SesExpirada) && s.Estado == SesActiva && s.Paso != "" {
		if err := a.sumaStat(tx, botID, s.MenuID, s.Paso, "abandono", s.Prueba, ahora, 1); err != nil {
			return err
		}
	}
	s.Estado = estado
	if err := tx.Commit(); err != nil {
		return err
	}
	a.avisa("sesiones")
	return nil
}

// SilenciarSesion hace que el bot calle con esa persona (humano o pausado) hasta `hasta`; si no tenía conversación abierta, la crea.
func (a *Almacen) SilenciarSesion(botID int64, chat, sender string, menuID int64, version int, estado string, hasta time.Time, ahora time.Time) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	r, err := tx.Exec(`UPDATE flow_sessions SET estado=?, hasta_ts=?, ultima_ts=? WHERE bot_id=? AND chat=? AND sender=? AND prueba=0 AND estado IN (?,?,?)`,
		estado, hasta.Unix(), ahora.Unix(), botID, chat, sender, SesActiva, SesHumano, SesPausada)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		if _, err := tx.Exec(`INSERT INTO flow_sessions(bot_id, chat, sender, menu_id, menu_version, paso, inicio_ts, ultima_ts, hasta_ts, estado) VALUES(?,?,?,?,?,?,?,?,?,?)`,
			botID, chat, sender, menuID, version, "", ahora.Unix(), ahora.Unix(), hasta.Unix(), estado); err != nil {
			return err
		}
	}
	if estado == SesHumano {
		if err := a.sumaStat(tx, botID, menuID, "", "humano", false, ahora, 1); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	a.avisa("sesiones")
	return nil
}

// GuardarVariable guarda (cifrada) la respuesta libre de una persona.
func (a *Almacen) GuardarVariable(sesionID int64, nombre, valor string) error {
	sel, err := cifrado.SellarTexto(a.clave, "flow_var", []byte(valor))
	if err != nil {
		return err
	}
	_, err = a.db.Exec(`INSERT INTO flow_vars(session_id, nombre, valor_cifrado) VALUES(?,?,?) ON CONFLICT(session_id, nombre) DO UPDATE SET valor_cifrado=excluded.valor_cifrado`, sesionID, nombre, sel)
	return err
}

// Variables devuelve lo que la persona contestó en esta conversación.
func (a *Almacen) Variables(sesionID int64) map[string]string {
	out := map[string]string{}
	filas, err := a.db.Query(`SELECT nombre, valor_cifrado FROM flow_vars WHERE session_id=?`, sesionID)
	if err != nil {
		return out
	}
	defer filas.Close()
	for filas.Next() {
		var n string
		var b []byte
		if filas.Scan(&n, &b) == nil {
			if t, err := cifrado.AbrirTexto(a.clave, "flow_var", b); err == nil {
				out[n] = string(t)
			}
		}
	}
	return out
}

// EventoVista es un paso del recorrido de una persona.
type EventoVista struct {
	TS        int64  `json:"ts"`
	Paso      string `json:"paso"`
	Opcion    string `json:"opcion"`
	Resultado string `json:"resultado"`
}

// Recorrido lista lo que una persona hizo dentro del menú (sin texto libre).
func (a *Almacen) Recorrido(sesionID int64) ([]EventoVista, error) {
	filas, err := a.db.Query(`SELECT ts, paso, opcion, resultado FROM flow_events WHERE session_id=? ORDER BY id`, sesionID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []EventoVista{}
	for filas.Next() {
		var e EventoVista
		if err := filas.Scan(&e.TS, &e.Paso, &e.Opcion, &e.Resultado); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, filas.Err()
}

// SesionEnCurso es una conversación abierta lista para mostrar.
type SesionEnCurso struct {
	Sesion
	Nombre string `json:"nombre"`
}

// SesionesEnCurso lista las conversaciones abiertas (no de prueba), de la más reciente a la más vieja, caducando las vencidas.
func (a *Almacen) SesionesEnCurso(botID int64, ahora time.Time, expira time.Duration) ([]SesionEnCurso, error) {
	filas, err := a.db.Query(`SELECT `+colsSesion+` FROM flow_sessions WHERE bot_id=? AND prueba=0 AND estado IN (?,?,?) ORDER BY ultima_ts DESC LIMIT 200`, botID, SesActiva, SesHumano, SesPausada)
	if err != nil {
		return nil, err
	}
	var ss []*Sesion
	for filas.Next() {
		x, err := escanearSesion(filas)
		if err != nil {
			filas.Close()
			return nil, err
		}
		ss = append(ss, x)
	}
	filas.Close()
	out := []SesionEnCurso{}
	for _, x := range ss {
		v, err := a.SesionVigente(botID, x.Chat, x.Sender, false, ahora, expira) // caduca las vencidas
		if err != nil || v == nil || v.ID != x.ID {
			continue
		}
		out = append(out, SesionEnCurso{Sesion: *v, Nombre: a.nombreParaMostrar(botID, v.Chat, v.Sender)})
	}
	return out, nil
}

// SesionPorID devuelve una conversación.
func (a *Almacen) SesionPorID(botID, id int64) (*Sesion, error) {
	x, err := escanearSesion(a.db.QueryRow(`SELECT `+colsSesion+` FROM flow_sessions WHERE bot_id=? AND id=?`, botID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return x, err
}

// SesionesDeChat devuelve las conversaciones abiertas de un chat (varias en grupos: una por persona).
func (a *Almacen) SesionesDeChat(botID int64, chat string, ahora time.Time, expira time.Duration) ([]SesionEnCurso, error) {
	todas, err := a.SesionesEnCurso(botID, ahora, expira)
	if err != nil {
		return nil, err
	}
	out := []SesionEnCurso{}
	for _, s := range todas {
		if s.Chat == chat {
			out = append(out, s)
		}
	}
	return out, nil
}

// ---------- estadísticas ----------

func (a *Almacen) sumaStat(tx *sql.Tx, botID, menuID int64, paso, opcion string, prueba bool, ahora time.Time, n int) error {
	dia, _ := a.diaHora(ahora.Unix())
	_, err := tx.Exec(`INSERT INTO menu_stats(bot_id, day, menu_id, paso, opcion, prueba, cuenta) VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(bot_id, day, menu_id, paso, opcion, prueba) DO UPDATE SET cuenta=cuenta+excluded.cuenta`, botID, dia, menuID, paso, opcion, b2i(prueba), n)
	return err
}

// FilaStat es un contador de las estadísticas de un menú.
type FilaStat struct {
	Paso   string `json:"paso"`
	Opcion string `json:"opcion"`
	Cuenta int64  `json:"cuenta"`
}

// EstadisticasMenu suma los contadores de un menú entre dos días (AAAA-MM-DD, incluidos). `prueba` incluye las pruebas del simulador.
func (a *Almacen) EstadisticasMenu(botID, menuID int64, desde, hasta string, prueba bool) ([]FilaStat, error) {
	filas, err := a.db.Query(`SELECT paso, opcion, SUM(cuenta) FROM menu_stats WHERE bot_id=? AND menu_id=? AND day>=? AND day<=? AND prueba=? GROUP BY paso, opcion ORDER BY paso, opcion`,
		botID, menuID, desde, hasta, b2i(prueba))
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []FilaStat{}
	for filas.Next() {
		var f FilaStat
		if err := filas.Scan(&f.Paso, &f.Opcion, &f.Cuenta); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, filas.Err()
}

// UltimoMensajeAnterior devuelve la hora del último mensaje recibido de un chat anterior a `antes` (0 si no hay).
func (a *Almacen) UltimoMensajeAnterior(botID int64, chat, remitente string, antes int64) int64 {
	var ts sql.NullInt64
	q := `SELECT MAX(ts) FROM messages WHERE bot_id=? AND chat_jid=? AND es_mio=0 AND ts<?`
	args := []any{botID, chat, antes}
	if remitente != "" {
		q += ` AND remitente_jid=?`
		args = append(args, remitente)
	}
	_ = a.db.QueryRow(q, args...).Scan(&ts)
	return ts.Int64
}

// ---------- avisos locales ----------

// Aviso es una notificación local pendiente de mostrar.
type Aviso struct {
	ID      int64  `json:"id"`
	TS      int64  `json:"ts"`
	Tipo    string `json:"tipo"`
	Chat    string `json:"chat"`
	Titulo  string `json:"titulo"`
	Detalle string `json:"detalle"`
	Persona string `json:"persona"` // persona a la que se refiere (para abrir su ficha), si la hay
}

// CrearAviso guarda un aviso (el detalle va cifrado) y devuelve su id.
func (a *Almacen) CrearAviso(botID int64, tipo, chat, titulo, detalle string) (int64, error) {
	return a.CrearAvisoDe(botID, tipo, chat, "", titulo, detalle)
}

// CrearAvisoDe es CrearAviso con la persona a la que se refiere el aviso.
func (a *Almacen) CrearAvisoDe(botID int64, tipo, chat, persona, titulo, detalle string) (int64, error) {
	sel, err := cifrado.SellarTexto(a.clave, "aviso", []byte(detalle))
	if err != nil {
		return 0, err
	}
	r, err := a.db.Exec(`INSERT INTO avisos(bot_id, ts, tipo, chat, titulo, detalle_cifrado, persona) VALUES(?,?,?,?,?,?,?)`, botID, time.Now().Unix(), tipo, chat, titulo, sel, persona)
	if err != nil {
		return 0, err
	}
	id, _ := r.LastInsertId()
	a.avisa(fmt.Sprintf("aviso:%d", id))
	if f := a.AlAviso; f != nil {
		go f(botID, tipo, chat, titulo, detalle)
	}
	return id, nil
}

// ObtenerAviso devuelve un aviso con su detalle descifrado.
func (a *Almacen) ObtenerAviso(botID, id int64) (*Aviso, error) {
	var x Aviso
	var d []byte
	err := a.db.QueryRow(`SELECT id, ts, tipo, chat, titulo, detalle_cifrado, persona FROM avisos WHERE bot_id=? AND id=?`, botID, id).Scan(&x.ID, &x.TS, &x.Tipo, &x.Chat, &x.Titulo, &d, &x.Persona)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if t, e := cifrado.AbrirTexto(a.clave, "aviso", d); e == nil {
		x.Detalle = string(t)
	}
	return &x, nil
}

// LimpiarAvisos borra los avisos viejos.
func (a *Almacen) LimpiarAvisos(botID int64, dias int) {
	_, _ = a.db.Exec(`DELETE FROM avisos WHERE bot_id=? AND ts<?`, botID, time.Now().AddDate(0, 0, -dias).Unix())
}

// UltimaActivacion devuelve la hora en que esa persona empezó por última vez esa automatización (0 si nunca).
func (a *Almacen) UltimaActivacion(botID, menuID int64, chat, sender string, ts *int64) error {
	var x sql.NullInt64
	err := a.db.QueryRow(`SELECT MAX(inicio_ts) FROM flow_sessions WHERE bot_id=? AND menu_id=? AND chat=? AND sender=? AND prueba=0`, botID, menuID, chat, sender).Scan(&x)
	*ts = x.Int64
	return err
}

// Zona es la zona horaria del teléfono que se usa para agrupar y para los horarios.
func (a *Almacen) Zona() *time.Location { return a.loc() }

// ReactivarSesiones cierra las conversaciones "con una persona" o pausadas de un chat (para que el bot vuelva a responder ahí).
func (a *Almacen) ReactivarSesiones(botID int64, chat string) {
	_, _ = a.db.Exec(`UPDATE flow_sessions SET estado=?, ultima_ts=? WHERE bot_id=? AND chat=? AND estado IN (?,?)`, SesAbandonada, time.Now().Unix(), botID, chat, SesHumano, SesPausada)
	a.avisa("sesiones")
}

// AvisosRecientes devuelve los últimos avisos locales (el más nuevo primero), con su detalle descifrado.
func (a *Almacen) AvisosRecientes(botID int64, limite int) ([]Aviso, error) {
	if limite <= 0 {
		limite = 50
	}
	filas, err := a.db.Query(`SELECT id FROM avisos WHERE bot_id=? ORDER BY id DESC LIMIT ?`, botID, limite)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for filas.Next() {
		var id int64
		if filas.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	filas.Close()
	out := []Aviso{}
	for _, id := range ids {
		if x, err := a.ObtenerAviso(botID, id); err == nil && x != nil {
			out = append(out, *x)
		}
	}
	return out, nil
}
