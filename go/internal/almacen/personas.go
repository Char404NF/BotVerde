package almacen

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"botchat/internal/cifrado"
)

// Personas (v0.5.0, bloque B): silencio del bot, advertencias, notas, ficha, estadísticas, olvidar y exportar. Todo va por persona
// canónica (número si se conoce) y con sus alias (número y LID). Bloquear en el bot NO tiene tabla propia: es el estado "bloqueada"
// de solicitudes. Los motivos y las notas van cifrados y nunca se copian al registro de acciones.

// formas devuelve todas las claves (canónica y alias) de una persona.
func (a *Almacen) formas(botID int64, persona string) []string {
	return a.AliasDe(botID, persona)
}

func enLista(n int) string { return "(" + marcas(n) + ")" }

func args2(botID int64, xs []string) []any {
	out := make([]any, 0, len(xs)+1)
	out = append(out, botID)
	for _, x := range xs {
		out = append(out, x)
	}
	return out
}

// ---------- silencio ----------

// Silencio es el aviso "el bot no le contesta a esta persona" (HastaTS 0 = para siempre).
type Silencio struct {
	Persona  string `json:"persona"`
	HastaTS  int64  `json:"hastaTs"`
	Motivo   string `json:"motivo"`
	CreadoTS int64  `json:"creadoTs"`
	Por      string `json:"por"`
}

// FijarSilencio silencia al bot para esa persona hasta `hasta` (segundos; 0 = siempre). Reemplaza el silencio anterior.
func (a *Almacen) FijarSilencio(botID int64, persona string, hasta int64, motivo, por string, ahora time.Time) error {
	persona = a.ResolverPersona(botID, persona)
	if persona == "" || persona == PersonaDueno {
		return ErrRolDueno
	}
	_, err := a.db.Exec(`INSERT INTO persona_silencio(bot_id,persona,hasta_ts,motivo,creado_ts,por) VALUES(?,?,?,?,?,?)
		ON CONFLICT(bot_id,persona) DO UPDATE SET hasta_ts=excluded.hasta_ts, motivo=excluded.motivo, creado_ts=excluded.creado_ts, por=excluded.por`,
		botID, persona, hasta, motivo, ahora.Unix(), por)
	if err == nil {
		a.avisa("personas")
		a.avisa("persona:" + persona)
	}
	return err
}

// QuitarSilencio deja que el bot vuelva a contestarle. Devuelve si había silencio.
func (a *Almacen) QuitarSilencio(botID int64, persona string) (bool, error) {
	persona = a.ResolverPersona(botID, persona)
	r, err := a.db.Exec(`DELETE FROM persona_silencio WHERE bot_id=? AND persona=?`, botID, persona)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	if n > 0 {
		a.avisa("personas")
		a.avisa("persona:" + persona)
	}
	return n > 0, nil
}

// SilencioDe devuelve el silencio vigente de la persona (nil si no hay o ya venció; el vencido se limpia).
func (a *Almacen) SilencioDe(botID int64, persona string, ahora time.Time) *Silencio {
	if persona == "" {
		return nil
	}
	persona = a.ResolverPersona(botID, persona)
	var s Silencio
	err := a.db.QueryRow(`SELECT persona,hasta_ts,motivo,creado_ts,por FROM persona_silencio WHERE bot_id=? AND persona=?`, botID, persona).
		Scan(&s.Persona, &s.HastaTS, &s.Motivo, &s.CreadoTS, &s.Por)
	if err != nil {
		return nil
	}
	if s.HastaTS > 0 && s.HastaTS <= ahora.Unix() {
		_, _ = a.db.Exec(`DELETE FROM persona_silencio WHERE bot_id=? AND persona=?`, botID, persona)
		a.avisa("personas")
		return nil
	}
	return &s
}

// ListarSilencios devuelve los silencios vigentes (los vencidos se borran).
func (a *Almacen) ListarSilencios(botID int64, ahora time.Time) ([]Silencio, error) {
	_, _ = a.db.Exec(`DELETE FROM persona_silencio WHERE bot_id=? AND hasta_ts>0 AND hasta_ts<=?`, botID, ahora.Unix())
	filas, err := a.db.Query(`SELECT persona,hasta_ts,motivo,creado_ts,por FROM persona_silencio WHERE bot_id=? ORDER BY creado_ts DESC`, botID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []Silencio{}
	for filas.Next() {
		var s Silencio
		if filas.Scan(&s.Persona, &s.HastaTS, &s.Motivo, &s.CreadoTS, &s.Por) == nil {
			out = append(out, s)
		}
	}
	return out, filas.Err()
}

// ---------- bloqueo en el bot (estado "bloqueada" de solicitudes) ----------

// BloquearPersona hace que el bot ignore por completo a la persona. No es el bloqueo de WhatsApp.
func (a *Almacen) BloquearPersona(botID int64, persona string, ahora time.Time) error {
	return a.DecidirSolicitud(botID, persona, SolBloqueada, ahora)
}

// DesbloquearPersona quita el bloqueo (la persona vuelve a ser una persona normal). Devuelve si estaba bloqueada.
func (a *Almacen) DesbloquearPersona(botID int64, persona string) (bool, error) {
	s, err := a.ObtenerSolicitud(botID, persona)
	if err != nil || s == nil || s.Estado != SolBloqueada {
		return false, err
	}
	return true, a.BorrarSolicitud(botID, persona)
}

// EstaBloqueada dice si el bot ignora a la persona.
func (a *Almacen) EstaBloqueada(botID int64, persona string) bool {
	s, _ := a.ObtenerSolicitud(botID, persona)
	return s != nil && s.Estado == SolBloqueada
}

// ---------- advertencias ----------

// Advertencia es un aviso de conducta a una persona. Desde la 0.6.0 puede tener tipo, regla del grupo y fecha de caducidad
// (CaducaTS = 0: no caduca, así quedan las que ya existían).
type Advertencia struct {
	ID       int64  `json:"id"`
	Persona  string `json:"persona"`
	Chat     string `json:"chat"`
	Motivo   string `json:"motivo"`
	TS       int64  `json:"ts"`
	Por      string `json:"por"`
	TipoID   int64  `json:"tipoId"`
	ReglaID  int64  `json:"reglaId"`
	CaducaTS int64  `json:"caducaTs"`
	Vigente  bool   `json:"vigente"`
}

const condVigente = ` AND (caduca_ts=0 OR caduca_ts>?)`

// AgregarAdvertencia anota una advertencia sin tipo (motivo opcional, cifrado) y devuelve su id y cuántas vigentes lleva la persona.
func (a *Almacen) AgregarAdvertencia(botID int64, persona, chat, motivo, por string, ahora time.Time) (id int64, total int, err error) {
	return a.AgregarAdvertenciaTipo(botID, persona, chat, motivo, por, 0, 0, ahora)
}

// AgregarAdvertenciaTipo anota una advertencia con tipo y regla (0 = sin tipo / sin regla). La caducidad sale del tipo, o del ajuste
// adv_caducidad_dias si el tipo no tiene la suya (0 = no caduca).
func (a *Almacen) AgregarAdvertenciaTipo(botID int64, persona, chat, motivo, por string, tipoID, reglaID int64, ahora time.Time) (id int64, total int, err error) {
	persona = a.ResolverPersona(botID, persona)
	if persona == "" || persona == PersonaDueno {
		return 0, 0, ErrRolDueno
	}
	var sel []byte
	if strings.TrimSpace(motivo) != "" {
		if sel, err = cifrado.SellarTexto(a.clave, "advertencia", []byte(strings.TrimSpace(motivo))); err != nil {
			return 0, 0, err
		}
	}
	var caduca int64
	if dias := a.CaducidadDias(botID, tipoID); dias > 0 {
		caduca = ahora.Add(time.Duration(dias) * 24 * time.Hour).Unix()
	}
	r, err := a.db.Exec(`INSERT INTO advertencias(bot_id,persona,chat,motivo_cifrado,ts,por,tipo_id,caduca_ts,regla_id) VALUES(?,?,?,?,?,?,?,?,?)`,
		botID, persona, chat, sel, ahora.Unix(), por, tipoID, caduca, reglaID)
	if err != nil {
		return 0, 0, err
	}
	id, _ = r.LastInsertId()
	a.RefrescarBanderas(botID, persona, ahora)
	a.avisa("personas")
	a.avisa("persona:" + persona)
	return id, a.ContarAdvertenciasEn(botID, persona, ahora), nil
}

// ContarAdvertencias cuenta las advertencias VIGENTES de la persona (todas sus formas), a la hora de ahora.
func (a *Almacen) ContarAdvertencias(botID int64, persona string) int {
	return a.ContarAdvertenciasEn(botID, persona, a.reloj())
}

// ContarAdvertenciasEn cuenta las vigentes a esa hora (las que no caducaron).
func (a *Almacen) ContarAdvertenciasEn(botID int64, persona string, ahora time.Time) int {
	fs := a.formas(botID, persona)
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM advertencias WHERE bot_id=? AND persona IN `+enLista(len(fs))+condVigente, append(args2(botID, fs), ahora.Unix())...).Scan(&n)
	return n
}

// ContarAdvertenciasTipo cuenta las vigentes de un tipo (0 = sin tipo).
func (a *Almacen) ContarAdvertenciasTipo(botID int64, persona string, tipoID int64, ahora time.Time) int {
	fs := a.formas(botID, persona)
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM advertencias WHERE bot_id=? AND persona IN `+enLista(len(fs))+` AND tipo_id=?`+condVigente,
		append(append(args2(botID, fs), tipoID), ahora.Unix())...).Scan(&n)
	return n
}

// ListarAdvertencias devuelve las advertencias de la persona (vigentes y vencidas, con la bandera Vigente), la más reciente primero.
func (a *Almacen) ListarAdvertencias(botID int64, persona string, limite int) ([]Advertencia, error) {
	return a.ListarAdvertenciasEn(botID, persona, limite, a.reloj())
}

// ListarAdvertenciasEn es ListarAdvertencias con la hora de ahora dada.
func (a *Almacen) ListarAdvertenciasEn(botID int64, persona string, limite int, ahora time.Time) ([]Advertencia, error) {
	if limite <= 0 {
		limite = 200
	}
	fs := a.formas(botID, persona)
	filas, err := a.db.Query(`SELECT id,persona,chat,motivo_cifrado,ts,por,tipo_id,regla_id,caduca_ts FROM advertencias WHERE bot_id=? AND persona IN `+enLista(len(fs))+` ORDER BY ts DESC, id DESC LIMIT ?`,
		append(args2(botID, fs), limite)...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []Advertencia{}
	for filas.Next() {
		var x Advertencia
		var m []byte
		if filas.Scan(&x.ID, &x.Persona, &x.Chat, &m, &x.TS, &x.Por, &x.TipoID, &x.ReglaID, &x.CaducaTS) != nil {
			continue
		}
		x.Vigente = x.CaducaTS == 0 || x.CaducaTS > ahora.Unix()
		if len(m) > 0 {
			if t, e := cifrado.AbrirTexto(a.clave, "advertencia", m); e == nil {
				x.Motivo = string(t)
			}
		}
		out = append(out, x)
	}
	return out, filas.Err()
}

// ListarPersonasAdvertidas devuelve quién tiene advertencias vigentes y cuántas: [{persona, total, ultimaTs}].
type ResumenAdvertencias struct {
	Persona  string `json:"persona"`
	Total    int    `json:"total"`
	UltimaTS int64  `json:"ultimaTs"`
}

func (a *Almacen) ListarPersonasAdvertidas(botID int64) ([]ResumenAdvertencias, error) {
	filas, err := a.db.Query(`SELECT persona,COUNT(*),MAX(ts) FROM advertencias WHERE bot_id=?`+condVigente+` GROUP BY persona ORDER BY MAX(ts) DESC`, botID, a.reloj().Unix())
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []ResumenAdvertencias{}
	for filas.Next() {
		var x ResumenAdvertencias
		if filas.Scan(&x.Persona, &x.Total, &x.UltimaTS) == nil {
			out = append(out, x)
		}
	}
	return out, filas.Err()
}

// QuitarAdvertencia borra una advertencia concreta (id>0) o la más reciente VIGENTE de la persona (id 0). Devuelve cuántas quedan.
func (a *Almacen) QuitarAdvertencia(botID int64, persona string, id int64) (quedan int, quitada bool, err error) {
	return a.QuitarAdvertenciaEn(botID, persona, id, a.reloj())
}

// QuitarAdvertenciaEn es QuitarAdvertencia con la hora de ahora dada.
func (a *Almacen) QuitarAdvertenciaEn(botID int64, persona string, id int64, ahora time.Time) (quedan int, quitada bool, err error) {
	fs := a.formas(botID, persona)
	lista := enLista(len(fs))
	if id == 0 {
		err = a.db.QueryRow(`SELECT id FROM advertencias WHERE bot_id=? AND persona IN `+lista+condVigente+` ORDER BY ts DESC, id DESC LIMIT 1`, append(args2(botID, fs), ahora.Unix())...).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		if err != nil {
			return 0, false, err
		}
	}
	r, err := a.db.Exec(`DELETE FROM advertencias WHERE bot_id=? AND id=? AND persona IN `+lista, append([]any{botID, id}, args2(botID, fs)[1:]...)...)
	if err != nil {
		return 0, false, err
	}
	n, _ := r.RowsAffected()
	canon := a.ResolverPersona(botID, persona)
	a.RefrescarBanderas(botID, canon, ahora)
	a.avisa("personas")
	a.avisa("persona:" + canon)
	return a.ContarAdvertenciasEn(botID, persona, ahora), n > 0, nil
}

// BorrarAdvertencias borra todo el historial de advertencias de la persona (también las vencidas). Devuelve cuántas había.
func (a *Almacen) BorrarAdvertencias(botID int64, persona string) (int, error) {
	fs := a.formas(botID, persona)
	r, err := a.db.Exec(`DELETE FROM advertencias WHERE bot_id=? AND persona IN `+enLista(len(fs)), args2(botID, fs)...)
	if err != nil {
		return 0, err
	}
	n, _ := r.RowsAffected()
	canon := a.ResolverPersona(botID, persona)
	a.RefrescarBanderas(botID, canon, a.reloj())
	a.avisa("personas")
	a.avisa("persona:" + canon)
	return int(n), nil
}

// AdvertenciasDelDia cuenta las advertencias puestas desde `desde` (para el resumen diario).
func (a *Almacen) AdvertenciasDesde(botID int64, desde int64) int {
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM advertencias WHERE bot_id=? AND ts>=?`, botID, desde).Scan(&n)
	return n
}

// ---------- notas ----------

// Nota es una nota privada sobre una persona (cifrada).
type Nota struct {
	ID      int64  `json:"id"`
	Persona string `json:"persona"`
	Texto   string `json:"texto"`
	TS      int64  `json:"ts"`
	Por     string `json:"por"`
}

// AgregarNota guarda una nota sobre la persona.
func (a *Almacen) AgregarNota(botID int64, persona, texto, por string, ahora time.Time) (int64, error) {
	persona = a.ResolverPersona(botID, persona)
	texto = strings.TrimSpace(texto)
	if persona == "" || persona == PersonaDueno {
		return 0, ErrRolDueno
	}
	if texto == "" {
		return 0, errors.New("la nota está vacía")
	}
	if len([]rune(texto)) > 2000 {
		return 0, errors.New("la nota es muy larga (máximo 2000 letras)")
	}
	sel, err := cifrado.SellarTexto(a.clave, "nota_persona", []byte(texto))
	if err != nil {
		return 0, err
	}
	r, err := a.db.Exec(`INSERT INTO notas_persona(bot_id,persona,texto_cifrado,ts,por) VALUES(?,?,?,?,?)`, botID, persona, sel, ahora.Unix(), por)
	if err != nil {
		return 0, err
	}
	id, _ := r.LastInsertId()
	a.avisa("personas")
	a.avisa("persona:" + persona)
	return id, nil
}

// ListarNotas devuelve las notas de la persona, la más reciente primero.
func (a *Almacen) ListarNotas(botID int64, persona string) ([]Nota, error) {
	fs := a.formas(botID, persona)
	filas, err := a.db.Query(`SELECT id,persona,texto_cifrado,ts,por FROM notas_persona WHERE bot_id=? AND persona IN `+enLista(len(fs))+` ORDER BY ts DESC, id DESC`, args2(botID, fs)...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []Nota{}
	for filas.Next() {
		var x Nota
		var b []byte
		if filas.Scan(&x.ID, &x.Persona, &b, &x.TS, &x.Por) != nil {
			continue
		}
		if t, e := cifrado.AbrirTexto(a.clave, "nota_persona", b); e == nil {
			x.Texto = string(t)
		}
		out = append(out, x)
	}
	return out, filas.Err()
}

// ContarNotas cuenta las notas de la persona.
func (a *Almacen) ContarNotas(botID int64, persona string) int {
	fs := a.formas(botID, persona)
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM notas_persona WHERE bot_id=? AND persona IN `+enLista(len(fs)), args2(botID, fs)...).Scan(&n)
	return n
}

// BorrarNota quita una nota.
func (a *Almacen) BorrarNota(botID int64, persona string, id int64) (bool, error) {
	fs := a.formas(botID, persona)
	r, err := a.db.Exec(`DELETE FROM notas_persona WHERE bot_id=? AND id=? AND persona IN `+enLista(len(fs)), append([]any{botID, id}, args2(botID, fs)[1:]...)...)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	a.avisa("personas")
	a.avisa("persona:" + a.ResolverPersona(botID, persona))
	return n > 0, nil
}

// ---------- ficha y estadísticas ----------

// Ficha es todo lo que se sabe de una persona, en una sola vista.
type Ficha struct {
	Persona      string     `json:"persona"`
	Alias        []string   `json:"alias"`
	Nombre       string     `json:"nombre"`
	NombrePropio string     `json:"nombrePropio"` // el que le puso el dueño en la app ("" = ninguno)
	Telefono     string     `json:"telefono"`
	Silencio     *Silencio  `json:"silencio"`
	Bloqueada    bool       `json:"bloqueada"`
	NoMolestar   bool       `json:"noMolestar"`
	Advertencias int        `json:"advertencias"`
	Notas        int        `json:"notas"`
	Etiquetas    []Etiqueta `json:"etiquetas"`
	PrimeraTS    int64      `json:"primeraTs"`
	UltimaTS     int64      `json:"ultimaTs"`
	Recibidos    int        `json:"recibidos"` // mensajes que escribió esa persona (en privado y en grupos)
	Enviados     int        `json:"enviados"`  // lo que le escribí yo en su chat privado
	Grupos       int        `json:"grupos"`    // en cuántos grupos escribió
}

func (a *Almacen) chatsPrivados(botID int64, fs []string) []string {
	var out []string
	for _, f := range fs {
		if !strings.HasSuffix(f, "@g.us") {
			out = append(out, f)
		}
	}
	return out
}

// FichaDe arma la ficha de una persona.
func (a *Almacen) FichaDe(botID int64, persona string, ahora time.Time) (*Ficha, error) {
	canon := a.ResolverPersona(botID, persona)
	if canon == "" {
		return nil, errors.New("falta la persona")
	}
	fs := a.formas(botID, canon)
	f := &Ficha{Persona: canon, Alias: fs, Silencio: a.SilencioDe(botID, canon, ahora), Bloqueada: a.EstaBloqueada(botID, canon),
		NoMolestar: a.EstaNoMolestar(botID, canon), Advertencias: a.ContarAdvertenciasEn(botID, canon, ahora), Notas: a.ContarNotas(botID, canon), Etiquetas: []Etiqueta{}}
	for _, x := range fs {
		if c, _ := a.ObtenerChat(botID, x); c != nil && !c.EsGrupo {
			if f.Nombre == "" {
				f.Nombre = c.Mostrado
			}
			if f.NombrePropio == "" {
				f.NombrePropio = c.Propio
			}
			if f.Telefono == "" {
				f.Telefono = c.TelMostrado
			}
			if et, err := a.EtiquetasDeChat(botID, x); err == nil {
				f.Etiquetas = append(f.Etiquetas, et...)
			}
		}
	}
	if et, err := a.EtiquetasDePersona(botID, canon); err == nil {
		for _, e := range et {
			dup := false
			for _, y := range f.Etiquetas {
				dup = dup || y.ID == e.ID
			}
			if !dup {
				f.Etiquetas = append(f.Etiquetas, e)
			}
		}
	}
	lista := enLista(len(fs))
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(count),0) FROM daily_stats WHERE bot_id=? AND sender IN `+lista, args2(botID, fs)...).Scan(&f.Recibidos)
	_ = a.db.QueryRow(`SELECT COUNT(DISTINCT chat) FROM daily_stats WHERE bot_id=? AND chat LIKE '%@g.us' AND sender IN `+lista, args2(botID, fs)...).Scan(&f.Grupos)
	if priv := a.chatsPrivados(botID, fs); len(priv) > 0 {
		pa := append([]any{botID, QuienYo}, func() []any {
			o := make([]any, len(priv))
			for i, x := range priv {
				o[i] = x
			}
			return o
		}()...)
		_ = a.db.QueryRow(`SELECT COALESCE(SUM(count),0) FROM daily_stats WHERE bot_id=? AND sender=? AND chat IN `+enLista(len(priv)), pa...).Scan(&f.Enviados)
		var mn, mx sql.NullInt64
		for _, c := range priv {
			var a1, a2 sql.NullInt64
			_ = a.db.QueryRow(`SELECT MIN(ts),MAX(ts) FROM messages WHERE bot_id=? AND chat_jid=?`, botID, c).Scan(&a1, &a2)
			if a1.Valid && (!mn.Valid || a1.Int64 < mn.Int64) {
				mn = a1
			}
			if a2.Valid && (!mx.Valid || a2.Int64 > mx.Int64) {
				mx = a2
			}
		}
		f.PrimeraTS, f.UltimaTS = mn.Int64, mx.Int64
	}
	// En grupos solo hay días: sirven de límite si no hay chat privado.
	if f.PrimeraTS == 0 {
		var d1, d2 sql.NullString
		_ = a.db.QueryRow(`SELECT MIN(day),MAX(day) FROM daily_stats WHERE bot_id=? AND sender IN `+lista, args2(botID, fs)...).Scan(&d1, &d2)
		if d1.Valid {
			if t, err := time.ParseInLocation("2006-01-02", d1.String, a.loc()); err == nil {
				f.PrimeraTS = t.Unix()
			}
			if t, err := time.ParseInLocation("2006-01-02", d2.String, a.loc()); err == nil {
				f.UltimaTS = t.Add(23 * time.Hour).Unix()
			}
		}
	}
	return f, nil
}

// EstadisticasPersona son los números de actividad de una persona.
type EstadisticasPersona struct {
	Persona   string             `json:"persona"`
	Total     int                `json:"total"`
	PorTipo   map[string]int     `json:"porTipo"`
	PorHora   [24]int            `json:"porHora"`
	PorDia    []DiaConteoPersona `json:"porDia"` // últimos días con actividad, de viejo a nuevo
	Chats     []ChatCuenta       `json:"chats"`
	Bytes     int64              `json:"bytes"`
	PrimerDia string             `json:"primerDia"`
	UltimoDia string             `json:"ultimoDia"`
}

// DiaConteoPersona son los mensajes de un día.
type DiaConteoPersona struct {
	Dia    string `json:"dia"`
	Cuenta int    `json:"cuenta"`
}

// ChatCuenta son los mensajes de la persona en un chat.
type ChatCuenta struct {
	Chat   string `json:"chat"`
	Nombre string `json:"nombre"`
	Cuenta int    `json:"cuenta"`
	Grupo  bool   `json:"grupo"`
}

// EstadisticasDePersona cuenta lo que escribió la persona (de daily_stats; nunca lee textos). `dias` limita la lista por día.
func (a *Almacen) EstadisticasDePersona(botID int64, persona string, dias int) (*EstadisticasPersona, error) {
	if dias <= 0 {
		dias = 30
	}
	fs := a.formas(botID, persona)
	lista := enLista(len(fs))
	e := &EstadisticasPersona{Persona: a.ResolverPersona(botID, persona), PorTipo: map[string]int{}, PorDia: []DiaConteoPersona{}, Chats: []ChatCuenta{}}
	filas, err := a.db.Query(`SELECT chat,day,hora,tipo,count,bytes FROM daily_stats WHERE bot_id=? AND sender IN `+lista, args2(botID, fs)...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	dia := map[string]int{}
	chats := map[string]int{}
	for filas.Next() {
		var chat, d, tipo string
		var h, n int
		var by int64
		if filas.Scan(&chat, &d, &h, &tipo, &n, &by) != nil {
			continue
		}
		e.Total += n
		e.PorTipo[tipo] += n
		if h >= 0 && h < 24 {
			e.PorHora[h] += n
		}
		e.Bytes += by
		dia[d] += n
		chats[chat] += n
		if e.PrimerDia == "" || d < e.PrimerDia {
			e.PrimerDia = d
		}
		if d > e.UltimoDia {
			e.UltimoDia = d
		}
	}
	if err := filas.Err(); err != nil {
		return nil, err
	}
	ds := make([]string, 0, len(dia))
	for d := range dia {
		ds = append(ds, d)
	}
	sort.Strings(ds)
	if len(ds) > dias {
		ds = ds[len(ds)-dias:]
	}
	for _, d := range ds {
		e.PorDia = append(e.PorDia, DiaConteoPersona{Dia: d, Cuenta: dia[d]})
	}
	for c, n := range chats {
		x := ChatCuenta{Chat: c, Cuenta: n, Grupo: strings.HasSuffix(c, "@g.us")}
		if ch, _ := a.ObtenerChat(botID, c); ch != nil {
			x.Nombre = ch.Mostrado
		}
		e.Chats = append(e.Chats, x)
	}
	sort.Slice(e.Chats, func(i, j int) bool { return e.Chats[i].Cuenta > e.Chats[j].Cuenta })
	if len(e.Chats) > 10 {
		e.Chats = e.Chats[:10]
	}
	return e, nil
}

// ---------- olvidar ----------

// ResultadoOlvidar cuenta lo que se borró.
type ResultadoOlvidar struct {
	Mensajes     int   `json:"mensajes"`
	Bytes        int64 `json:"bytes"`
	Notas        int   `json:"notas"`
	Advertencias int   `json:"advertencias"`
	Sesiones     int   `json:"sesiones"`
	Alias        int   `json:"alias"`
}

// OlvidarPersona borra todo lo de una persona: su chat privado completo (mensajes, archivos, fotos), lo que escribió en grupos, sus
// notas, advertencias, silencio, rango, sesiones de menú y alias. NO quita la lista No molestar ni el bloqueo del bot (son para
// respetar su pedido y guardan solo el número). El registro de acciones conserva que se olvidó (sin textos).
func (a *Almacen) OlvidarPersona(botID int64, persona string) (*ResultadoOlvidar, error) {
	canon := a.ResolverPersona(botID, persona)
	if canon == "" || canon == PersonaDueno {
		return nil, ErrRolDueno
	}
	fs := a.formas(botID, canon)
	res := &ResultadoOlvidar{}
	// 1. chats privados completos
	for _, c := range a.chatsPrivados(botID, fs) {
		var n int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE bot_id=? AND chat_jid=?`, botID, c).Scan(&n)
		res.Mensajes += n
		by, err := a.BorrarChat(botID, c)
		if err != nil {
			return res, err
		}
		res.Bytes += by
		_, _ = a.db.Exec(`DELETE FROM chat_etiqueta WHERE bot_id=? AND chat_jid=?`, botID, c)
	}
	// 2. lo que escribió en grupos
	lista := enLista(len(fs))
	filas, err := a.db.Query(`SELECT chat_jid,id,archivo_ruta,miniatura_ruta,archivo_bytes FROM messages WHERE bot_id=? AND remitente_jid IN `+lista, args2(botID, fs)...)
	if err != nil {
		return res, err
	}
	type fila struct {
		chat, id, ruta, mini string
		bytes                int64
	}
	var ms []fila
	for filas.Next() {
		var f fila
		if filas.Scan(&f.chat, &f.id, &f.ruta, &f.mini, &f.bytes) == nil {
			ms = append(ms, f)
		}
	}
	filas.Close()
	a.mu.Lock()
	for _, m := range ms {
		a.quita(m.ruta)
		a.quita(m.mini)
		res.Bytes += m.bytes
		for _, q := range []string{"DELETE FROM messages WHERE bot_id=? AND chat_jid=? AND id=?", "DELETE FROM message_receipts WHERE bot_id=? AND chat_jid=? AND msg_id=?", "DELETE FROM reacciones WHERE bot_id=? AND chat_jid=? AND msg_id=?"} {
			if _, err := a.db.Exec(q, botID, m.chat, m.id); err != nil {
				a.mu.Unlock()
				return res, err
			}
		}
		a.borrarMarcasDeMensaje(botID, m.chat, m.id)
		res.Mensajes++
	}
	a.mu.Unlock()
	// sus reacciones y acuses en mensajes de otros, y sus estadísticas
	_, _ = a.db.Exec(`DELETE FROM reacciones WHERE bot_id=? AND sender IN `+lista, args2(botID, fs)...)
	_, _ = a.db.Exec(`DELETE FROM message_receipts WHERE bot_id=? AND participante IN `+lista, args2(botID, fs)...)
	_, _ = a.db.Exec(`DELETE FROM daily_stats WHERE bot_id=? AND sender IN `+lista, args2(botID, fs)...)
	// 3. datos de la persona
	del := func(q string, extra ...any) int {
		r, e := a.db.Exec(q, append(args2(botID, fs), extra...)...)
		if e != nil {
			return 0
		}
		n, _ := r.RowsAffected()
		return int(n)
	}
	del(`DELETE FROM persona_etiqueta WHERE bot_id=? AND persona IN ` + lista)
	del(`DELETE FROM grupo_miembros WHERE bot_id=? AND persona IN ` + lista)
	del(`DELETE FROM grupo_cambios WHERE bot_id=? AND (persona IN `+lista+` OR por IN `+lista+`)`, argsSolo(fs)...)
	a.borrarFotoPersona(botID, fs)
	res.Notas = del(`DELETE FROM notas_persona WHERE bot_id=? AND persona IN ` + lista)
	del(`DELETE FROM util_items WHERE bot_id=? AND persona IN ` + lista)
	a.borrarLabDe(botID, fs)
	a.olvidarModulosPersona(botID, lista, fs)
	res.Advertencias = del(`DELETE FROM advertencias WHERE bot_id=? AND persona IN ` + lista)
	del(`DELETE FROM persona_silencio WHERE bot_id=? AND persona IN ` + lista)
	del(`DELETE FROM filtro_infracciones WHERE bot_id=? AND persona IN ` + lista)
	a.limpiaSilencioDe(botID, lista, fs)
	a.OlvidarAutoPersona(botID, fs)
	del(`DELETE FROM persona_rol WHERE bot_id=? AND persona IN ` + lista)
	del(`DELETE FROM codigos_confirmacion WHERE bot_id=? AND persona IN ` + lista)
	del(`DELETE FROM avisos WHERE bot_id=? AND (persona IN `+lista+` OR chat IN `+lista+`)`, argsSolo(fs)...)
	del(`DELETE FROM actividad WHERE bot_id=? AND chat IN ` + lista)
	// el bloqueo y No molestar se conservan; una solicitud pendiente/aprobada/ignorada se borra
	del(`DELETE FROM solicitudes WHERE bot_id=? AND estado<>'bloqueada' AND persona IN ` + lista)
	// 4. sesiones de menú
	ids := []int64{}
	if fl, e := a.db.Query(`SELECT id FROM flow_sessions WHERE bot_id=? AND (sender IN `+lista+` OR chat IN `+lista+`)`, append(args2(botID, fs), argsSolo(fs)...)...); e == nil {
		for fl.Next() {
			var id int64
			if fl.Scan(&id) == nil {
				ids = append(ids, id)
			}
		}
		fl.Close()
	}
	for _, id := range ids {
		_, _ = a.db.Exec(`DELETE FROM flow_events WHERE session_id=?`, id)
		_, _ = a.db.Exec(`DELETE FROM flow_vars WHERE session_id=?`, id)
		_, _ = a.db.Exec(`DELETE FROM flow_sessions WHERE id=?`, id)
	}
	res.Sesiones = len(ids)
	// formularios que mandó, fila de espera y mensajes del bot ligados a sus conversaciones (etapa C)
	del(`DELETE FROM formularios WHERE bot_id=? AND (persona IN `+lista+` OR chat IN `+lista+`)`, argsSolo(fs)...)
	del(`DELETE FROM flow_cola WHERE bot_id=? AND persona IN ` + lista)
	del(`DELETE FROM flow_vencido WHERE bot_id=? AND persona IN ` + lista)
	del(`DELETE FROM msg_bot_sesion WHERE bot_id=? AND persona IN ` + lista)
	// 5. registro de acciones: queda que pasó, pero sin a quién
	_, _ = a.db.Exec(`UPDATE registro_acciones SET objetivo='', chat='' WHERE bot_id=? AND (objetivo IN `+lista+` OR chat IN `+lista+`)`, append(args2(botID, fs), argsSolo(fs)...)...)
	_, _ = a.db.Exec(`UPDATE registro_acciones SET quien='persona_olvidada' WHERE bot_id=? AND quien IN `+lista, args2(botID, fs)...)
	// 6. alias (al final: arriba hacía falta conocer todas las formas)
	res.Alias = del(`DELETE FROM persona_alias WHERE bot_id=? AND (alias IN `+lista+` OR canonica IN `+lista+`)`, argsSolo(fs)...)
	a.nombres.limpiar()
	a.avisa("chats")
	a.avisa("espacio")
	a.avisa("personas")
	a.avisa("rangos")
	a.avisa("solicitudes")
	return res, nil
}

func argsSolo(xs []string) []any {
	out := make([]any, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}

// ---------- exportar ----------

// MensajeExportado es un mensaje en el archivo de exportación.
type MensajeExportado struct {
	Chat      string `json:"chat"`
	Grupo     bool   `json:"grupo"`
	ID        string `json:"id"`
	TS        int64  `json:"ts"`
	Tipo      string `json:"tipo"`
	EsMio     bool   `json:"esMio"`
	Texto     string `json:"texto,omitempty"`
	Archivo   string `json:"archivo,omitempty"` // solo el tipo y el tamaño; los archivos no van dentro
	Eliminado bool   `json:"eliminado,omitempty"`
}

// ExportePersona es lo que contiene el archivo de exportación de una persona.
type ExportePersona struct {
	Version      int                  `json:"version"`
	GeneradoTS   int64                `json:"generadoTs"`
	Ficha        *Ficha               `json:"ficha"`
	Advertencias []Advertencia        `json:"advertencias"`
	Notas        []Nota               `json:"notas"`
	Estadisticas *EstadisticasPersona `json:"estadisticas"`
	Mensajes     []MensajeExportado   `json:"mensajes"`
	Formularios  []Formulario         `json:"formularios,omitempty"` // formularios que completó en un menú (etapa C)
	Utilidades   []ItemUtil           `json:"utilidades,omitempty"`  // sus notas y pendientes propios (etapa F)
	Etiquetas    []string             `json:"etiquetas,omitempty"`   // nombres de las etiquetas de la persona (v0.6.0)
	Recortado    bool                 `json:"recortado,omitempty"`
}

const maxExportados = 200000

// DatosDePersona reúne todo lo guardado de la persona para exportarlo: ficha, advertencias, notas, estadísticas y los mensajes de
// su chat privado y los que escribió en grupos (el texto va descifrado: el archivo se cifra al guardarlo).
func (a *Almacen) DatosDePersona(botID int64, persona string, ahora time.Time) (*ExportePersona, error) {
	f, err := a.FichaDe(botID, persona, ahora)
	if err != nil {
		return nil, err
	}
	ex := &ExportePersona{Version: 1, GeneradoTS: ahora.Unix(), Ficha: f, Mensajes: []MensajeExportado{}}
	if et, e := a.EtiquetasDePersona(botID, persona); e == nil {
		for _, x := range et {
			ex.Etiquetas = append(ex.Etiquetas, x.Emoji+" "+x.Nombre)
		}
	}
	if ex.Advertencias, err = a.ListarAdvertencias(botID, persona, 10000); err != nil {
		return nil, err
	}
	if ex.Notas, err = a.ListarNotas(botID, persona); err != nil {
		return nil, err
	}
	if ex.Estadisticas, err = a.EstadisticasDePersona(botID, persona, 3650); err != nil {
		return nil, err
	}
	ex.Utilidades = a.ListarItemsUtil(botID, persona, "", true)
	fs := a.formas(botID, persona)
	lista := enLista(len(fs))
	if ff, e := a.db.Query(`SELECT `+colsForm+` FROM formularios WHERE bot_id=? AND (persona IN `+lista+` OR chat IN `+lista+`) ORDER BY id`, append(args2(botID, fs), argsSolo(fs)...)...); e == nil {
		for ff.Next() {
			if f, e := a.escanearFormulario(ff); e == nil {
				ex.Formularios = append(ex.Formularios, *f)
			}
		}
		ff.Close()
	}
	q := `SELECT chat_jid,id,ts,tipo,es_mio,texto_cifrado,archivo_bytes,borrado FROM messages WHERE bot_id=? AND (chat_jid IN ` + lista + ` OR remitente_jid IN ` + lista + `) ORDER BY ts, id LIMIT ?`
	filas, err := a.db.Query(q, append(append(args2(botID, fs), argsSolo(fs)...), maxExportados+1)...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	for filas.Next() {
		var m MensajeExportado
		var mio, bor int
		var t []byte
		var bytes int64
		if filas.Scan(&m.Chat, &m.ID, &m.TS, &m.Tipo, &mio, &t, &bytes, &bor) != nil {
			continue
		}
		if len(ex.Mensajes) >= maxExportados {
			ex.Recortado = true
			break
		}
		m.EsMio, m.Eliminado, m.Grupo = mio == 1, bor == 1, strings.HasSuffix(m.Chat, "@g.us")
		m.Texto = a.abreTexto("texto", t)
		if bytes > 0 {
			m.Archivo = fmt.Sprintf("%s, %d bytes", m.Tipo, bytes)
		}
		ex.Mensajes = append(ex.Mensajes, m)
	}
	return ex, filas.Err()
}

// EscribirExporte guarda el contenido (ya en JSON) cifrado con el formato BCF1 en exportes/ y devuelve la ruta relativa. Los
// exportes anteriores de la misma persona se reemplazan.
func (a *Almacen) EscribirExporte(botID int64, persona string, json []byte, ahora time.Time) (string, error) {
	rel := filepath.Join("exportes", fmt.Sprint(botID), fmt.Sprintf("persona_%s_%d.bcf", HashChat(botID, a.ResolverPersona(botID, persona)), ahora.Unix()))
	if abs, err := a.Abs(filepath.Join("exportes", fmt.Sprint(botID))); err == nil {
		_ = os.MkdirAll(abs, 0o700)
		if es, err := os.ReadDir(abs); err == nil {
			pre := "persona_" + HashChat(botID, a.ResolverPersona(botID, persona)) + "_"
			for _, e := range es {
				if strings.HasPrefix(e.Name(), pre) {
					_ = os.Remove(filepath.Join(abs, e.Name()))
				}
			}
		}
	}
	if err := a.EscribirCifrado(rel, json); err != nil {
		return "", err
	}
	return rel, nil
}

// LimpiarExportes borra los archivos de exportación de más de `horas` horas (se hacen para compartirse una vez).
func (a *Almacen) LimpiarExportes(botID int64, horas int, ahora time.Time) {
	abs, err := a.Abs(filepath.Join("exportes", fmt.Sprint(botID)))
	if err != nil {
		return
	}
	es, err := os.ReadDir(abs)
	if err != nil {
		return
	}
	for _, e := range es {
		if i, err := e.Info(); err == nil && ahora.Sub(i.ModTime()) > time.Duration(horas)*time.Hour {
			_ = os.Remove(filepath.Join(abs, e.Name()))
		}
	}
}

// FijarNombrePropio le pone a la persona el nombre que se verá en toda la app y en los mensajes del bot ({nombre}). Con "" vuelve al nombre
// de la agenda o al público. Vale para todas las formas de la persona (teléfono y LID).
func (a *Almacen) FijarNombrePropio(botID int64, persona, nombre string) error {
	nombre = strings.Join(strings.Fields(nombre), " ")
	if len([]rune(nombre)) > 40 {
		return errors.New("el nombre es muy largo (máximo 40 letras)")
	}
	canon := a.ResolverPersona(botID, persona)
	if canon == "" || canon == PersonaDueno {
		return errors.New("falta la persona")
	}
	hecho := false
	for _, x := range a.formas(botID, canon) {
		c, _ := a.ObtenerChat(botID, x)
		if c == nil || c.EsGrupo {
			continue
		}
		hecho = true
		if _, err := a.ActualizarChat(botID, x, false, func(c *Chat) { c.Propio = nombre }); err != nil {
			return err
		}
		if nombre == "" { // la escritura normal no borra un nombre propio ya guardado: se quita aparte
			if _, err := a.db.Exec(`UPDATE chats SET nombre_propio='' WHERE bot_id=? AND jid=?`, botID, x); err != nil {
				return err
			}
		}
	}
	if !hecho {
		return errors.New("todavía no hay un chat con esa persona")
	}
	a.avisa("personas")
	a.avisa("persona:" + canon)
	return nil
}
