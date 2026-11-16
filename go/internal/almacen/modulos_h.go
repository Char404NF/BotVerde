package almacen

import (
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	"botchat/internal/cifrado"
)

// ---------- módulos opcionales H1–H5 (v0.5.0, migración 18) ----------
//
// Tablas: mod_listas (listas de destinatarios: suscritos, "quieren entrar"…), mod_agenda (mensajes programados y difusiones; el
// contenido va CIFRADO), mod_dest (destinatarios de una difusión y cómo les fue), mod_pendientes (verificaciones de bienvenida),
// mod_chats (a qué grupos aplica un módulo) y mod_conectores (conectores HTTP; su configuración, con llaves, va CIFRADA).
// Nada de esto guarda textos de mensajes recibidos; lo único que se escribe es lo que la persona dueña redactó.

// ---------- listas de destinatarios ----------

// ListaConteo es una lista y cuántas personas tiene.
type ListaConteo struct {
	Lista string `json:"lista"`
	N     int    `json:"n"`
}

// AgregarALista mete a una persona (clave canónica) en una lista. Devuelve si era nueva.
func (a *Almacen) AgregarALista(botID int64, lista, persona, origen string, ahora time.Time) (bool, error) {
	lista, persona = strings.TrimSpace(lista), a.ResolverPersona(botID, persona)
	if lista == "" || persona == "" {
		return false, errors.New("falta la lista o la persona")
	}
	r, err := a.db.Exec(`INSERT OR IGNORE INTO mod_listas(bot_id,lista,persona,desde_ts,origen) VALUES(?,?,?,?,?)`, botID, lista, persona, ahora.Unix(), origen)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n > 0, nil
}

// QuitarDeLista saca a una persona de una lista.
func (a *Almacen) QuitarDeLista(botID int64, lista, persona string) error {
	_, err := a.db.Exec(`DELETE FROM mod_listas WHERE bot_id=? AND lista=? AND persona=?`, botID, lista, a.ResolverPersona(botID, persona))
	return err
}

// QuitarDeTodasLasListas saca a una persona de todas las listas (cuando pide la baja). Devuelve de cuántas la sacó.
func (a *Almacen) QuitarDeTodasLasListas(botID int64, persona string) int {
	r, err := a.db.Exec(`DELETE FROM mod_listas WHERE bot_id=? AND persona=?`, botID, a.ResolverPersona(botID, persona))
	if err != nil {
		return 0
	}
	n, _ := r.RowsAffected()
	return int(n)
}

// EstaEnLista dice si la persona está en la lista.
func (a *Almacen) EstaEnLista(botID int64, lista, persona string) bool {
	var x int
	return a.db.QueryRow(`SELECT 1 FROM mod_listas WHERE bot_id=? AND lista=? AND persona=?`, botID, lista, a.ResolverPersona(botID, persona)).Scan(&x) == nil
}

// MiembrosLista devuelve las personas de una lista (la más nueva primero). limite <= 0 = todas.
func (a *Almacen) MiembrosLista(botID int64, lista string, limite int) ([]string, error) {
	q := `SELECT persona FROM mod_listas WHERE bot_id=? AND lista=? ORDER BY desde_ts DESC, persona`
	args := []any{botID, lista}
	if limite > 0 {
		q += ` LIMIT ?`
		args = append(args, limite)
	}
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var out []string
	for filas.Next() {
		var p string
		if filas.Scan(&p) == nil {
			out = append(out, p)
		}
	}
	return out, filas.Err()
}

// ListasConConteo devuelve las listas que existen y cuántas personas tiene cada una. Las que pidan `siempre` salen aunque estén vacías.
func (a *Almacen) ListasConConteo(botID int64, siempre ...string) []ListaConteo {
	cuenta := map[string]int{}
	for _, s := range siempre {
		cuenta[s] = 0
	}
	filas, err := a.db.Query(`SELECT lista, COUNT(*) FROM mod_listas WHERE bot_id=? GROUP BY lista`, botID)
	if err == nil {
		for filas.Next() {
			var l string
			var n int
			if filas.Scan(&l, &n) == nil {
				cuenta[l] = n
			}
		}
		filas.Close()
	}
	out := make([]ListaConteo, 0, len(cuenta))
	for l, n := range cuenta {
		out = append(out, ListaConteo{l, n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Lista < out[j].Lista })
	return out
}

// BorrarListaMod vacía y borra una lista.
func (a *Almacen) BorrarListaMod(botID int64, lista string) error {
	_, err := a.db.Exec(`DELETE FROM mod_listas WHERE bot_id=? AND lista=?`, botID, lista)
	return err
}

// PersonasQueEscribieron devuelve los chats privados donde la otra persona escribió alguna vez (a ellas sí se les puede escribir).
func (a *Almacen) PersonasQueEscribieron(botID int64) []string {
	filas, err := a.db.Query(`SELECT DISTINCT m.chat_jid FROM messages m JOIN chats c ON c.bot_id=m.bot_id AND c.jid=m.chat_jid
		WHERE m.bot_id=? AND m.es_mio=0 AND c.es_grupo=0 AND m.chat_jid LIKE '%@%' ORDER BY m.chat_jid`, botID)
	if err != nil {
		return nil
	}
	defer filas.Close()
	var out []string
	for filas.Next() {
		var j string
		if filas.Scan(&j) == nil && !strings.HasSuffix(j, "@broadcast") && !strings.HasSuffix(j, "@newsletter") {
			out = append(out, j)
		}
	}
	return out
}

// ---------- agenda (mensajes programados y difusiones) ----------

// AgendaFila es un renglón de la agenda. Contenido es el JSON descifrado (texto y adjuntos) que arma cada módulo.
type AgendaFila struct {
	ID         int64  `json:"id"`
	Modulo     string `json:"modulo"`
	Nombre     string `json:"nombre"`
	Destino    string `json:"destino"` // un chat, o "lista:<nombre>"
	Contenido  string `json:"-"`
	ProximoTS  int64  `json:"proximoTs"` // 0 = sin fecha (por ejemplo una difusión sin programar)
	Repeticion string `json:"repeticion"`
	Estado     string `json:"estado"`
	CreadoTS   int64  `json:"creadoTs"`
	UltimoTS   int64  `json:"ultimoTs"`
	UltimoRes  string `json:"ultimoRes"`
	Veces      int    `json:"veces"`
}

// CrearAgenda guarda una fila nueva y devuelve su id.
func (a *Almacen) CrearAgenda(botID int64, f AgendaFila, ahora time.Time) (int64, error) {
	sel, err := cifrado.SellarTexto(a.clave, "mod_agenda", []byte(f.Contenido))
	if err != nil {
		return 0, err
	}
	if f.Estado == "" {
		f.Estado = "activo"
	}
	r, err := a.db.Exec(`INSERT INTO mod_agenda(bot_id,modulo,nombre,destino,contenido,proximo_ts,repeticion,estado,creado_ts) VALUES(?,?,?,?,?,?,?,?,?)`,
		botID, f.Modulo, f.Nombre, f.Destino, sel, f.ProximoTS, f.Repeticion, f.Estado, ahora.Unix())
	if err != nil {
		return 0, err
	}
	id, _ := r.LastInsertId()
	a.avisa("modulos")
	return id, nil
}

const colsAgenda = `id,modulo,nombre,destino,contenido,proximo_ts,repeticion,estado,creado_ts,ultimo_ts,ultimo_res,veces`

func (a *Almacen) leerAgenda(fila interface{ Scan(...any) error }) (AgendaFila, error) {
	var f AgendaFila
	var c []byte
	if err := fila.Scan(&f.ID, &f.Modulo, &f.Nombre, &f.Destino, &c, &f.ProximoTS, &f.Repeticion, &f.Estado, &f.CreadoTS, &f.UltimoTS, &f.UltimoRes, &f.Veces); err != nil {
		return f, err
	}
	if len(c) > 0 {
		if t, err := cifrado.AbrirTexto(a.clave, "mod_agenda", c); err == nil {
			f.Contenido = string(t)
		}
	}
	return f, nil
}

// ObtenerAgenda devuelve una fila (nil si no existe).
func (a *Almacen) ObtenerAgenda(botID, id int64) (*AgendaFila, error) {
	f, err := a.leerAgenda(a.db.QueryRow(`SELECT `+colsAgenda+` FROM mod_agenda WHERE bot_id=? AND id=?`, botID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func (a *Almacen) listaAgenda(q string, args ...any) ([]AgendaFila, error) {
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var out []AgendaFila
	for filas.Next() {
		if f, err := a.leerAgenda(filas); err == nil {
			out = append(out, f)
		}
	}
	return out, filas.Err()
}

// ListarAgenda devuelve las filas de un módulo (las que tienen fecha, por fecha; luego el resto), de las que no se han cancelado.
func (a *Almacen) ListarAgenda(botID int64, modulo string) ([]AgendaFila, error) {
	return a.listaAgenda(`SELECT `+colsAgenda+` FROM mod_agenda WHERE bot_id=? AND modulo=? ORDER BY CASE WHEN estado IN ('activo','enviando','pausada') THEN 0 ELSE 1 END, CASE WHEN proximo_ts=0 THEN 9223372036854775807 ELSE proximo_ts END, id DESC`, botID, modulo)
}

// AgendaVencida devuelve las filas activas cuya fecha ya llegó (proximo_ts entre 1 y `hasta`).
func (a *Almacen) AgendaVencida(botID int64, modulo string, hasta time.Time) ([]AgendaFila, error) {
	return a.listaAgenda(`SELECT `+colsAgenda+` FROM mod_agenda WHERE bot_id=? AND modulo=? AND estado='activo' AND proximo_ts>0 AND proximo_ts<=? ORDER BY proximo_ts, id`, botID, modulo, hasta.Unix())
}

// AgendaConEstado devuelve las filas de un módulo en ese estado.
func (a *Almacen) AgendaConEstado(botID int64, modulo, estado string) ([]AgendaFila, error) {
	return a.listaAgenda(`SELECT `+colsAgenda+` FROM mod_agenda WHERE bot_id=? AND modulo=? AND estado=? ORDER BY id`, botID, modulo, estado)
}

// ProximaAgenda dice cuándo toca la siguiente fila activa de esos módulos (0 = ninguna).
func (a *Almacen) ProximaAgenda(botID int64, modulos ...string) int64 {
	if len(modulos) == 0 {
		return 0
	}
	marcas := strings.TrimSuffix(strings.Repeat("?,", len(modulos)), ",")
	args := []any{botID}
	for _, m := range modulos {
		args = append(args, m)
	}
	var t sql.NullInt64
	_ = a.db.QueryRow(`SELECT MIN(proximo_ts) FROM mod_agenda WHERE bot_id=? AND estado='activo' AND proximo_ts>0 AND modulo IN (`+marcas+`)`, args...).Scan(&t)
	return t.Int64
}

// ActualizarAgenda cambia el estado, la próxima fecha y el último resultado de una fila. veces se suma 1 si `contar`.
func (a *Almacen) ActualizarAgenda(botID, id int64, estado string, proximo int64, ultimo time.Time, res string, contar bool) error {
	q := `UPDATE mod_agenda SET estado=?, proximo_ts=?, ultimo_res=?`
	args := []any{estado, proximo, res}
	if !ultimo.IsZero() {
		q += `, ultimo_ts=?`
		args = append(args, ultimo.Unix())
	}
	if contar {
		q += `, veces=veces+1`
	}
	q += ` WHERE bot_id=? AND id=?`
	args = append(args, botID, id)
	_, err := a.db.Exec(q, args...)
	if err == nil {
		a.avisa("modulos")
	}
	return err
}

// ActualizarContenidoAgenda reemplaza el contenido (cifrado) de una fila de la agenda.
func (a *Almacen) ActualizarContenidoAgenda(botID, id int64, contenido string) error {
	sel, err := cifrado.SellarTexto(a.clave, "mod_agenda", []byte(contenido))
	if err != nil {
		return err
	}
	_, err = a.db.Exec(`UPDATE mod_agenda SET contenido=? WHERE bot_id=? AND id=?`, sel, botID, id)
	if err == nil {
		a.avisa("modulos")
	}
	return err
}

// BorrarAgenda borra una fila y sus destinatarios.
func (a *Almacen) BorrarAgenda(botID, id int64) error {
	if _, err := a.db.Exec(`DELETE FROM mod_dest WHERE bot_id=? AND agenda_id=?`, botID, id); err != nil {
		return err
	}
	_, err := a.db.Exec(`DELETE FROM mod_agenda WHERE bot_id=? AND id=?`, botID, id)
	if err == nil {
		a.avisa("modulos")
	}
	return err
}

// ---------- destinatarios de una difusión ----------

// AgregarDest anota a quiénes se les va a mandar una difusión (estado "pendiente").
func (a *Almacen) AgregarDest(botID, agenda int64, personas []string) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	for _, p := range personas {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO mod_dest(bot_id,agenda_id,persona) VALUES(?,?,?)`, botID, agenda, p); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// DestConEstado devuelve hasta `limite` destinatarios en ese estado (limite <= 0 = todos).
func (a *Almacen) DestConEstado(botID, agenda int64, estado string, limite int) []string {
	q := `SELECT persona FROM mod_dest WHERE bot_id=? AND agenda_id=? AND estado=? ORDER BY persona`
	args := []any{botID, agenda, estado}
	if limite > 0 {
		q += ` LIMIT ?`
		args = append(args, limite)
	}
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil
	}
	defer filas.Close()
	var out []string
	for filas.Next() {
		var p string
		if filas.Scan(&p) == nil {
			out = append(out, p)
		}
	}
	return out
}

// MarcarDest fija cómo le fue a un destinatario (pendiente, encolado, enviado, fallido, omitido).
func (a *Almacen) MarcarDest(botID, agenda int64, persona, estado string, ahora time.Time) {
	_, _ = a.db.Exec(`UPDATE mod_dest SET estado=?, ts=? WHERE bot_id=? AND agenda_id=? AND persona=?`, estado, ahora.Unix(), botID, agenda, persona)
}

// ContarDest cuenta los destinatarios por estado.
func (a *Almacen) ContarDest(botID, agenda int64) map[string]int {
	out := map[string]int{}
	filas, err := a.db.Query(`SELECT estado, COUNT(*) FROM mod_dest WHERE bot_id=? AND agenda_id=? GROUP BY estado`, botID, agenda)
	if err != nil {
		return out
	}
	defer filas.Close()
	for filas.Next() {
		var e string
		var n int
		if filas.Scan(&e, &n) == nil {
			out[e] = n
		}
	}
	return out
}

// DestEncoladosAPendientes devuelve a "pendiente" lo que quedó "encolado": la cola de envío vive en memoria y se pierde al cerrar
// la app, así que al arrancar se vuelve a encolar. Devuelve cuántos.
func (a *Almacen) DestEncoladosAPendientes(botID int64) int {
	r, err := a.db.Exec(`UPDATE mod_dest SET estado='pendiente' WHERE bot_id=? AND estado='encolado'`, botID)
	if err != nil {
		return 0
	}
	n, _ := r.RowsAffected()
	return int(n)
}

// ---------- verificaciones de bienvenida ----------

// Pendiente es alguien que entró a un grupo y todavía no contesta la verificación.
type Pendiente struct {
	Chat, Persona string
	DesdeTS       int64
	VenceTS       int64
	Esperada      string
	Intentos      int
}

// PonerPendiente anota (o reinicia) una verificación.
func (a *Almacen) PonerPendiente(botID int64, modulo string, p Pendiente) error {
	_, err := a.db.Exec(`INSERT OR REPLACE INTO mod_pendientes(bot_id,modulo,chat,persona,desde_ts,vence_ts,esperada,intentos) VALUES(?,?,?,?,?,?,?,?)`,
		botID, modulo, p.Chat, p.Persona, p.DesdeTS, p.VenceTS, p.Esperada, p.Intentos)
	return err
}

// PendientesDe devuelve las verificaciones de un módulo en un grupo (chat "" = todas).
func (a *Almacen) PendientesDe(botID int64, modulo, chat string) []Pendiente {
	q := `SELECT chat,persona,desde_ts,vence_ts,esperada,intentos FROM mod_pendientes WHERE bot_id=? AND modulo=?`
	args := []any{botID, modulo}
	if chat != "" {
		q += ` AND chat=?`
		args = append(args, chat)
	}
	q += ` ORDER BY vence_ts`
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil
	}
	defer filas.Close()
	var out []Pendiente
	for filas.Next() {
		var p Pendiente
		if filas.Scan(&p.Chat, &p.Persona, &p.DesdeTS, &p.VenceTS, &p.Esperada, &p.Intentos) == nil {
			out = append(out, p)
		}
	}
	return out
}

// QuitarPendiente borra una verificación.
func (a *Almacen) QuitarPendiente(botID int64, modulo, chat, persona string) {
	_, _ = a.db.Exec(`DELETE FROM mod_pendientes WHERE bot_id=? AND modulo=? AND chat=? AND persona=?`, botID, modulo, chat, persona)
}

// SumarIntentoPendiente cuenta un intento fallido y devuelve el total.
func (a *Almacen) SumarIntentoPendiente(botID int64, modulo, chat, persona string) int {
	_, _ = a.db.Exec(`UPDATE mod_pendientes SET intentos=intentos+1 WHERE bot_id=? AND modulo=? AND chat=? AND persona=?`, botID, modulo, chat, persona)
	var n int
	_ = a.db.QueryRow(`SELECT intentos FROM mod_pendientes WHERE bot_id=? AND modulo=? AND chat=? AND persona=?`, botID, modulo, chat, persona).Scan(&n)
	return n
}

// ---------- a qué grupos aplica un módulo ----------

// AgregarChatMod anota un chat en la lista de un módulo.
func (a *Almacen) AgregarChatMod(botID int64, modulo, chat string) error {
	_, err := a.db.Exec(`INSERT OR IGNORE INTO mod_chats(bot_id,modulo,chat) VALUES(?,?,?)`, botID, modulo, chat)
	return err
}

// QuitarChatMod lo quita.
func (a *Almacen) QuitarChatMod(botID int64, modulo, chat string) error {
	_, err := a.db.Exec(`DELETE FROM mod_chats WHERE bot_id=? AND modulo=? AND chat=?`, botID, modulo, chat)
	return err
}

// ChatsMod devuelve los chats anotados para el módulo.
func (a *Almacen) ChatsMod(botID int64, modulo string) []string {
	filas, err := a.db.Query(`SELECT chat FROM mod_chats WHERE bot_id=? AND modulo=? ORDER BY chat`, botID, modulo)
	if err != nil {
		return nil
	}
	defer filas.Close()
	var out []string
	for filas.Next() {
		var c string
		if filas.Scan(&c) == nil {
			out = append(out, c)
		}
	}
	return out
}

// EstaChatMod dice si el chat está en la lista del módulo.
func (a *Almacen) EstaChatMod(botID int64, modulo, chat string) bool {
	var x int
	return a.db.QueryRow(`SELECT 1 FROM mod_chats WHERE bot_id=? AND modulo=? AND chat=?`, botID, modulo, chat).Scan(&x) == nil
}

// ---------- conectores HTTP ----------

// ConectorFila es un conector guardado. Config es el JSON descifrado (con las cabeceras y llaves).
type ConectorFila struct {
	ID     int64  `json:"id"`
	Nombre string `json:"nombre"`
	Config string `json:"-"`
}

// GuardarConector crea o reemplaza (por nombre) un conector. La configuración se cifra con el cifrado de columna.
func (a *Almacen) GuardarConector(botID int64, nombre, config string) (int64, error) {
	nombre = strings.TrimSpace(nombre)
	if nombre == "" {
		return 0, errors.New("falta el nombre")
	}
	sel, err := cifrado.SellarTexto(a.clave, "mod_conector", []byte(config))
	if err != nil {
		return 0, err
	}
	if _, err := a.db.Exec(`INSERT INTO mod_conectores(bot_id,nombre,config) VALUES(?,?,?) ON CONFLICT(bot_id,nombre) DO UPDATE SET config=excluded.config`, botID, nombre, sel); err != nil {
		return 0, err
	}
	var id int64
	_ = a.db.QueryRow(`SELECT id FROM mod_conectores WHERE bot_id=? AND nombre=?`, botID, nombre).Scan(&id)
	a.avisa("modulos")
	return id, nil
}

// ObtenerConector busca un conector por nombre (sin importar mayúsculas).
func (a *Almacen) ObtenerConector(botID int64, nombre string) (*ConectorFila, error) {
	var f ConectorFila
	var c []byte
	err := a.db.QueryRow(`SELECT id,nombre,config FROM mod_conectores WHERE bot_id=? AND nombre=? COLLATE NOCASE`, botID, strings.TrimSpace(nombre)).Scan(&f.ID, &f.Nombre, &c)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if t, err := cifrado.AbrirTexto(a.clave, "mod_conector", c); err == nil {
		f.Config = string(t)
	}
	return &f, nil
}

// ListarConectores devuelve todos los conectores por nombre.
func (a *Almacen) ListarConectores(botID int64) ([]ConectorFila, error) {
	filas, err := a.db.Query(`SELECT id,nombre,config FROM mod_conectores WHERE bot_id=? ORDER BY nombre COLLATE NOCASE`, botID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var out []ConectorFila
	for filas.Next() {
		var f ConectorFila
		var c []byte
		if filas.Scan(&f.ID, &f.Nombre, &c) != nil {
			continue
		}
		if t, err := cifrado.AbrirTexto(a.clave, "mod_conector", c); err == nil {
			f.Config = string(t)
		}
		out = append(out, f)
	}
	return out, filas.Err()
}

// BorrarConector borra un conector por nombre.
func (a *Almacen) BorrarConector(botID int64, nombre string) error {
	_, err := a.db.Exec(`DELETE FROM mod_conectores WHERE bot_id=? AND nombre=? COLLATE NOCASE`, botID, strings.TrimSpace(nombre))
	if err == nil {
		a.avisa("modulos")
	}
	return err
}

// olvidarModulosPersona borra de las listas, difusiones y verificaciones a las personas dadas (olvidar persona).
func (a *Almacen) olvidarModulosPersona(botID int64, lista string, fs []string) {
	for _, t := range []string{"mod_listas", "mod_dest", "mod_pendientes"} {
		_, _ = a.db.Exec(`DELETE FROM `+t+` WHERE bot_id=? AND persona IN `+lista, args2(botID, fs)...)
	}
	// marcador de los juegos: la clave es «chat|persona»
	_, _ = a.db.Exec(`DELETE FROM mod_datos WHERE bot_id=? AND modulo='juegos' AND coleccion='puntos' AND substr(clave, instr(clave,'|')+1) IN `+lista, args2(botID, fs)...)
}

// ContarRegistroModulo cuenta renglones del registro de un módulo con ese «qué» y ese resultado desde `desde` (que "" = cualquiera).
func (a *Almacen) ContarRegistroModulo(botID int64, modulo, que, resultado string, desde time.Time) int {
	q := `SELECT COUNT(*) FROM modulos_registro WHERE bot_id=? AND modulo=? AND resultado=? AND ts>=?`
	args := []any{botID, modulo, resultado, desde.Unix()}
	if que != "" {
		q += ` AND que=?`
		args = append(args, que)
	}
	var n int
	_ = a.db.QueryRow(q, args...).Scan(&n)
	return n
}

// ---------- datos propios de módulos (migración 19) ----------

// DatoMod es una fila de mod_datos: Valor viene descifrado.
type DatoMod struct {
	Clave string
	Num   int64
	TS    int64
	Valor string
}

// GuardarDatoMod crea o reemplaza un dato de un módulo (el valor se cifra con el cifrado de columna).
func (a *Almacen) GuardarDatoMod(botID int64, modulo, coleccion, clave string, num int64, valor string, ts time.Time) error {
	if modulo == "" || coleccion == "" || clave == "" {
		return errors.New("falta el módulo, la colección o la clave")
	}
	sel, err := cifrado.SellarTexto(a.clave, "mod_dato", []byte(valor))
	if err != nil {
		return err
	}
	_, err = a.db.Exec(`INSERT INTO mod_datos(bot_id,modulo,coleccion,clave,num,ts,valor) VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(bot_id,modulo,coleccion,clave) DO UPDATE SET num=excluded.num, ts=excluded.ts, valor=excluded.valor`,
		botID, modulo, coleccion, clave, num, ts.Unix(), sel)
	if err == nil {
		a.avisa("modulos")
	}
	return err
}

// ObtenerDatoMod devuelve un dato o nil.
func (a *Almacen) ObtenerDatoMod(botID int64, modulo, coleccion, clave string) *DatoMod {
	var d DatoMod
	var v []byte
	err := a.db.QueryRow(`SELECT clave,num,ts,valor FROM mod_datos WHERE bot_id=? AND modulo=? AND coleccion=? AND clave=?`, botID, modulo, coleccion, clave).Scan(&d.Clave, &d.Num, &d.TS, &v)
	if err != nil {
		return nil
	}
	if t, err := cifrado.AbrirTexto(a.clave, "mod_dato", v); err == nil {
		d.Valor = string(t)
	}
	return &d
}

// ListarDatosMod devuelve los datos de una colección (orden: por num de mayor a menor si porNum, si no por clave).
func (a *Almacen) ListarDatosMod(botID int64, modulo, coleccion string, porNum bool, limite int) []DatoMod {
	if limite <= 0 || limite > 5000 {
		limite = 1000
	}
	orden := "clave"
	if porNum {
		orden = "num DESC, clave"
	}
	filas, err := a.db.Query(`SELECT clave,num,ts,valor FROM mod_datos WHERE bot_id=? AND modulo=? AND coleccion=? ORDER BY `+orden+` LIMIT ?`, botID, modulo, coleccion, limite)
	if err != nil {
		return nil
	}
	defer filas.Close()
	var out []DatoMod
	for filas.Next() {
		var d DatoMod
		var v []byte
		if filas.Scan(&d.Clave, &d.Num, &d.TS, &v) != nil {
			continue
		}
		if t, err := cifrado.AbrirTexto(a.clave, "mod_dato", v); err == nil {
			d.Valor = string(t)
		}
		out = append(out, d)
	}
	return out
}

// BorrarDatoMod borra un dato.
func (a *Almacen) BorrarDatoMod(botID int64, modulo, coleccion, clave string) error {
	_, err := a.db.Exec(`DELETE FROM mod_datos WHERE bot_id=? AND modulo=? AND coleccion=? AND clave=?`, botID, modulo, coleccion, clave)
	if err == nil {
		a.avisa("modulos")
	}
	return err
}

// SumarDatoMod suma delta al contador de un dato (lo crea si no existe) y devuelve el total.
func (a *Almacen) SumarDatoMod(botID int64, modulo, coleccion, clave string, delta int64, ts time.Time) (int64, error) {
	if _, err := a.db.Exec(`INSERT INTO mod_datos(bot_id,modulo,coleccion,clave,num,ts) VALUES(?,?,?,?,?,?)
		ON CONFLICT(bot_id,modulo,coleccion,clave) DO UPDATE SET num=num+excluded.num, ts=excluded.ts`, botID, modulo, coleccion, clave, delta, ts.Unix()); err != nil {
		return 0, err
	}
	var n int64
	err := a.db.QueryRow(`SELECT num FROM mod_datos WHERE bot_id=? AND modulo=? AND coleccion=? AND clave=?`, botID, modulo, coleccion, clave).Scan(&n)
	return n, err
}

// BorrarColeccionMod borra todos los datos de una colección.
func (a *Almacen) BorrarColeccionMod(botID int64, modulo, coleccion string) error {
	_, err := a.db.Exec(`DELETE FROM mod_datos WHERE bot_id=? AND modulo=? AND coleccion=?`, botID, modulo, coleccion)
	if err == nil {
		a.avisa("modulos")
	}
	return err
}

// UltimoRegistroModulo devuelve la hora (segundos) del renglón más reciente del registro con ese módulo, «qué» y chat (0 si no hay).
func (a *Almacen) UltimoRegistroModulo(botID int64, modulo, que, chat string) int64 {
	var ts sql.NullInt64
	_ = a.db.QueryRow(`SELECT MAX(ts) FROM modulos_registro WHERE bot_id=? AND modulo=? AND que=? AND chat=?`, botID, modulo, que, chat).Scan(&ts)
	return ts.Int64
}
