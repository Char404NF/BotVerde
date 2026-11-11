package almacen

import (
	"encoding/json"
	"errors"
	"sort"
	"time"

	"botchat/internal/cifrado"
)

// Laboratorio (v0.5.0, bloque G): menús interactivos enviados (encuestas, botones, listas), sus votos y la matriz de pruebas.
// El MessageSecret, las opciones y lo que votó cada persona van CIFRADOS con el cifrado de columna del almacén.

// Tipos de menú interactivo.
const (
	LabEncuesta    = "encuesta"
	LabBotones     = "botones"
	LabLista       = "lista"
	LabInteractivo = "interactivo"
	// LabInteractivoBiz: prueba experimental (solo Laboratorio, solo chat de prueba) que añade el nodo <biz> del PR #1235 de whatsmeow.
	LabInteractivoBiz = "interactivo_biz"
)

// Estados de una sesión.
const (
	LabActiva      = "activa"
	LabReemplazada = "reemplazada" // salió un menú más nuevo para esa persona
	LabExpirada    = "expirada"
)

// Orígenes de una sesión.
const (
	LabOrigenMenu   = "menu"
	LabOrigenPrueba = "prueba"
)

// OpcionLab es una opción de un menú interactivo: el número que entiende el menú de texto y el nombre con el que salió.
type OpcionLab struct {
	Num   int    `json:"n"`
	Texto string `json:"t"`
	ID    string `json:"i,omitempty"` // id de botón o fila (botones y listas)
}

// SesionLab es un menú interactivo que salió.
type SesionLab struct {
	ID       string      `json:"id"`
	Chat     string      `json:"chat"`
	Persona  string      `json:"persona"`
	Tipo     string      `json:"tipo"`
	Origen   string      `json:"origen"`
	PruebaID int64       `json:"pruebaId,omitempty"`
	TS       int64       `json:"ts"`
	ExpiraTS int64       `json:"expiraTs"`
	Estado   string      `json:"estado"`
	Secreto  []byte      `json:"-"`
	Opciones []OpcionLab `json:"opciones"`
}

// GuardarSesionLab guarda un menú interactivo recién enviado. Si es de un menú real, los anteriores de esa persona en ese chat
// quedan "reemplazados" (un voto tardío en uno viejo ya no navega el menú). También limpia lo de más de 7 días.
func (a *Almacen) GuardarSesionLab(botID int64, s SesionLab) error {
	if s.ID == "" || s.Chat == "" {
		return errors.New("falta el id o el chat")
	}
	if s.Tipo == "" {
		s.Tipo = LabEncuesta
	}
	if s.Origen == "" {
		s.Origen = LabOrigenMenu
	}
	if s.Estado == "" {
		s.Estado = LabActiva
	}
	var sec []byte
	var err error
	if len(s.Secreto) > 0 {
		if sec, err = cifrado.SellarTexto(a.clave, "poll_secreto", s.Secreto); err != nil {
			return err
		}
	}
	js, _ := json.Marshal(s.Opciones)
	ops, err := cifrado.SellarTexto(a.clave, "poll_opciones", js)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if s.Origen == LabOrigenMenu {
		if _, err := a.db.Exec(`UPDATE poll_sessions SET estado=? WHERE bot_id=? AND chat=? AND persona=? AND origen=? AND estado=? AND id<>?`,
			LabReemplazada, botID, s.Chat, s.Persona, LabOrigenMenu, LabActiva, s.ID); err != nil {
			return err
		}
	}
	_, err = a.db.Exec(`INSERT OR REPLACE INTO poll_sessions(bot_id,id,chat,persona,tipo,origen,prueba_id,ts,expira_ts,estado,secreto_cifrado,opciones_cifrado)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, botID, s.ID, s.Chat, s.Persona, s.Tipo, s.Origen, s.PruebaID, s.TS, s.ExpiraTS, s.Estado, sec, ops)
	if err != nil {
		return err
	}
	viejo := time.Now().Add(-7 * 24 * time.Hour).Unix()
	_, _ = a.db.Exec(`DELETE FROM poll_votos WHERE bot_id=? AND poll_id IN (SELECT id FROM poll_sessions WHERE bot_id=? AND ts<?)`, botID, botID, viejo)
	_, _ = a.db.Exec(`DELETE FROM poll_sessions WHERE bot_id=? AND ts<?`, botID, viejo)
	return nil
}

// ObtenerSesionLab devuelve una sesión por el id del mensaje que salió (nil si no existe).
func (a *Almacen) ObtenerSesionLab(botID int64, id string) *SesionLab {
	var s SesionLab
	var sec, ops []byte
	err := a.db.QueryRow(`SELECT id,chat,persona,tipo,origen,prueba_id,ts,expira_ts,estado,secreto_cifrado,opciones_cifrado FROM poll_sessions WHERE bot_id=? AND id=?`, botID, id).
		Scan(&s.ID, &s.Chat, &s.Persona, &s.Tipo, &s.Origen, &s.PruebaID, &s.TS, &s.ExpiraTS, &s.Estado, &sec, &ops)
	if err != nil {
		return nil
	}
	if p, e := cifrado.AbrirTexto(a.clave, "poll_secreto", sec); e == nil {
		s.Secreto = p
	}
	if p, e := cifrado.AbrirTexto(a.clave, "poll_opciones", ops); e == nil {
		_ = json.Unmarshal(p, &s.Opciones)
	}
	return &s
}

// MarcarSesionLab cambia el estado de una sesión.
func (a *Almacen) MarcarSesionLab(botID int64, id, estado string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, _ = a.db.Exec(`UPDATE poll_sessions SET estado=? WHERE bot_id=? AND id=?`, estado, botID, id)
}

// BorrarSesionLab quita una sesión (por ejemplo, si el envío falló) y sus votos.
func (a *Almacen) BorrarSesionLab(botID int64, id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, _ = a.db.Exec(`DELETE FROM poll_votos WHERE bot_id=? AND poll_id=?`, botID, id)
	_, _ = a.db.Exec(`DELETE FROM poll_sessions WHERE bot_id=? AND id=?`, botID, id)
}

// ContarSesionesLab cuenta las sesiones guardadas (para las pruebas y la pantalla).
func (a *Almacen) ContarSesionesLab(botID int64) int {
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM poll_sessions WHERE bot_id=?`, botID).Scan(&n)
	return n
}

// VotoLab es lo último que votó una persona en una encuesta: los números de opción (vacío = retiró su voto).
type VotoLab struct {
	Nums []int `json:"nums"`
	TS   int64 `json:"ts"`
}

// ObtenerVotoLab devuelve el último voto guardado de ese votante (ok=false si nunca votó).
func (a *Almacen) ObtenerVotoLab(botID int64, pollID, votante string) (VotoLab, bool) {
	var b []byte
	var v VotoLab
	if a.db.QueryRow(`SELECT seleccion_cifrado, ts FROM poll_votos WHERE bot_id=? AND poll_id=? AND votante=?`, botID, pollID, votante).Scan(&b, &v.TS) != nil {
		return v, false
	}
	if p, e := cifrado.AbrirTexto(a.clave, "poll_voto", b); e == nil {
		_ = json.Unmarshal(p, &v.Nums)
	}
	sort.Ints(v.Nums)
	return v, true
}

// GuardarVotoLab guarda lo que votó (nums vacío = retiró el voto).
func (a *Almacen) GuardarVotoLab(botID int64, pollID, votante string, nums []int, ts time.Time) error {
	sort.Ints(nums)
	if nums == nil {
		nums = []int{}
	}
	js, _ := json.Marshal(nums)
	sel, err := cifrado.SellarTexto(a.clave, "poll_voto", js)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	_, err = a.db.Exec(`INSERT INTO poll_votos(bot_id,poll_id,votante,seleccion_cifrado,ts) VALUES(?,?,?,?,?)
		ON CONFLICT(bot_id,poll_id,votante) DO UPDATE SET seleccion_cifrado=excluded.seleccion_cifrado, ts=excluded.ts`, botID, pollID, votante, sel, ts.Unix())
	return err
}

// ---------- matriz de pruebas ----------

// ResultadoLab es un renglón de la matriz: una prueba enviada. Los campos -1 son "sin saber", 0 no y 1 sí.
type ResultadoLab struct {
	ID             int64  `json:"id"`
	TS             int64  `json:"ts"`
	Tipo           string `json:"tipo"`
	Variante       string `json:"variante"`
	Chat           string `json:"chat"`
	MsgID          string `json:"msgId"`
	Enviado        int    `json:"enviado"`   // WhatsApp aceptó el envío
	Llego          int    `json:"llego"`     // acuse de entrega recibido (se calcula de los acuses)
	Visible        int    `json:"visible"`   // lo marco yo
	Tocable        int    `json:"tocable"`   // lo marco yo
	Respondio      int    `json:"respondio"` // la respuesta (voto, botón o fila) llegó al bot
	VersionWA      string `json:"versionWa"`
	VersionCliente string `json:"versionCliente"`
	Error          string `json:"error,omitempty"`
}

// NuevoResultadoLab anota una prueba que se va a enviar y devuelve su id.
func (a *Almacen) NuevoResultadoLab(botID int64, tipo, variante, chat, versionWA, versionCliente string, ahora time.Time) (int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	r, err := a.db.Exec(`INSERT INTO lab_resultados(bot_id,ts,tipo,variante,chat,version_wa,version_cliente) VALUES(?,?,?,?,?,?,?)`,
		botID, ahora.Unix(), tipo, variante, chat, versionWA, versionCliente)
	if err != nil {
		return 0, err
	}
	id, _ := r.LastInsertId()
	return id, nil
}

// ResultadoLabEnviado anota cómo terminó el envío de una prueba.
func (a *Almacen) ResultadoLabEnviado(botID, id int64, msgID string, ok bool, errTxt string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, _ = a.db.Exec(`UPDATE lab_resultados SET msg_id=?, enviado=?, error=? WHERE bot_id=? AND id=?`, msgID, b2i(ok), errTxt, botID, id)
	a.avisa("lab")
}

// campos que se pueden marcar a mano o por el bot.
var camposLab = map[string]string{"visible": "visible", "tocable": "tocable", "respondio": "respondio"}

// MarcarResultadoLab fija visible, tocable o respondio (-1 sin saber, 0 no, 1 sí).
func (a *Almacen) MarcarResultadoLab(botID, id int64, campo string, valor int) error {
	col, ok := camposLab[campo]
	if !ok {
		return errors.New("campo desconocido")
	}
	if valor < -1 || valor > 1 {
		return errors.New("el valor debe ser -1, 0 o 1")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	r, err := a.db.Exec(`UPDATE lab_resultados SET `+col+`=? WHERE bot_id=? AND id=?`, valor, botID, id)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return errors.New("esa prueba no existe")
	}
	a.avisa("lab")
	return nil
}

// ListarResultadosLab devuelve las pruebas, de la más nueva a la más vieja. "Llegó" sale de los acuses de entrega o lectura.
func (a *Almacen) ListarResultadosLab(botID int64, limite int) []ResultadoLab {
	if limite <= 0 || limite > 500 {
		limite = 200
	}
	filas, err := a.db.Query(`SELECT r.id,r.ts,r.tipo,r.variante,r.chat,r.msg_id,r.enviado,
		CASE WHEN EXISTS(SELECT 1 FROM message_receipts m WHERE m.bot_id=r.bot_id AND m.chat_jid=r.chat AND m.msg_id=r.msg_id AND m.tipo IN ('entregado','leido')) THEN 1
		     WHEN r.enviado=1 THEN -1 ELSE 0 END,
		r.visible,r.tocable,r.respondio,r.version_wa,r.version_cliente,r.error
		FROM lab_resultados r WHERE r.bot_id=? ORDER BY r.id DESC LIMIT ?`, botID, limite)
	out := []ResultadoLab{}
	if err != nil {
		return out
	}
	defer filas.Close()
	for filas.Next() {
		var x ResultadoLab
		if filas.Scan(&x.ID, &x.TS, &x.Tipo, &x.Variante, &x.Chat, &x.MsgID, &x.Enviado, &x.Llego, &x.Visible, &x.Tocable, &x.Respondio, &x.VersionWA, &x.VersionCliente, &x.Error) == nil {
			out = append(out, x)
		}
	}
	return out
}

// ContarEnviosLab cuenta las pruebas enviadas desde `desde` (para los topes por hora y por día).
func (a *Almacen) ContarEnviosLab(botID int64, desde time.Time) int {
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM lab_resultados WHERE bot_id=? AND ts>=?`, botID, desde.Unix()).Scan(&n)
	return n
}

// BorrarResultadosLab vacía la matriz y las sesiones de prueba. Devuelve cuántas pruebas había.
func (a *Almacen) BorrarResultadosLab(botID int64) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	r, _ := a.db.Exec(`DELETE FROM lab_resultados WHERE bot_id=?`, botID)
	_, _ = a.db.Exec(`DELETE FROM poll_votos WHERE bot_id=? AND poll_id IN (SELECT id FROM poll_sessions WHERE bot_id=? AND origen=?)`, botID, botID, LabOrigenPrueba)
	_, _ = a.db.Exec(`DELETE FROM poll_sessions WHERE bot_id=? AND origen=?`, botID, LabOrigenPrueba)
	a.avisa("lab")
	if r == nil {
		return 0
	}
	n, _ := r.RowsAffected()
	return int(n)
}

// borrarLabDe borra las sesiones y los votos de esa persona (al olvidarla): las de su chat privado, las que salieron para ella en
// grupos y lo que votó ella.
func (a *Almacen) borrarLabDe(botID int64, formas []string) {
	if len(formas) == 0 {
		return
	}
	l := enLista(len(formas))
	ids := `SELECT id FROM poll_sessions WHERE bot_id=? AND (chat IN ` + l + ` OR persona IN ` + l + `)`
	arg := append(args2(botID, formas), anys(formas)...)
	_, _ = a.db.Exec(`DELETE FROM poll_votos WHERE bot_id=? AND poll_id IN (`+ids+`)`, append([]any{botID}, arg...)...)
	_, _ = a.db.Exec(`DELETE FROM poll_votos WHERE bot_id=? AND votante IN `+l, args2(botID, formas)...)
	_, _ = a.db.Exec(`DELETE FROM poll_sessions WHERE bot_id=? AND (chat IN `+l+` OR persona IN `+l+`)`, arg...)
}

func anys(xs []string) []any {
	out := make([]any, len(xs))
	for i, x := range xs {
		out[i] = x
	}
	return out
}
