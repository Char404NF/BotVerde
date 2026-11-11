package almacen

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"botchat/internal/cifrado"
	"botchat/internal/identidad"
)

// Avisos para mí (v0.5.0, bloque B): recordatorios, alertas por palabra clave y los números del resumen diario. Todo se muestra
// como notificación local de Android; nada de esto escribe en WhatsApp.

// Repeticiones de un recordatorio.
const (
	RepiteNunca   = ""
	RepiteDiaria  = "diaria"
	RepiteSemanal = "semanal"
)

// Estados de un recordatorio.
const (
	RecPendiente = "pendiente"
	RecHecho     = "hecho"
)

// Recordatorio es un aviso programado para mí.
type Recordatorio struct {
	ID        int64  `json:"id"`
	Texto     string `json:"texto"`
	ProximaTS int64  `json:"proximaTs"`
	Repetir   string `json:"repetir"`
	Estado    string `json:"estado"`
	CreadoTS  int64  `json:"creadoTs"`
	AvisadoTS int64  `json:"avisadoTs"`
}

func validaRepeticion(r string) (string, error) {
	switch r {
	case RepiteNunca, RepiteDiaria, RepiteSemanal:
		return r, nil
	}
	return "", errors.New("la repetición debe ser diaria o semanal")
}

// CrearRecordatorio programa un aviso para `proxima` (segundos). El texto va cifrado.
func (a *Almacen) CrearRecordatorio(botID int64, texto string, proxima int64, repetir string, ahora time.Time) (int64, error) {
	texto = strings.TrimSpace(texto)
	if texto == "" {
		return 0, errors.New("escribe de qué te aviso")
	}
	if len([]rune(texto)) > 500 {
		return 0, errors.New("el texto es muy largo (máximo 500 letras)")
	}
	if proxima <= 0 {
		return 0, errors.New("falta la fecha y la hora")
	}
	repetir, err := validaRepeticion(repetir)
	if err != nil {
		return 0, err
	}
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM recordatorios WHERE bot_id=? AND estado=?`, botID, RecPendiente).Scan(&n)
	if n >= 500 {
		return 0, errors.New("ya tienes 500 recordatorios pendientes")
	}
	sel, err := cifrado.SellarTexto(a.clave, "recordatorio", []byte(texto))
	if err != nil {
		return 0, err
	}
	r, err := a.db.Exec(`INSERT INTO recordatorios(bot_id,texto_cifrado,proxima_ts,repetir,estado,creado_ts) VALUES(?,?,?,?,?,?)`,
		botID, sel, proxima, repetir, RecPendiente, ahora.Unix())
	if err != nil {
		return 0, err
	}
	id, _ := r.LastInsertId()
	a.avisa("recordatorios")
	return id, nil
}

// EditarRecordatorio cambia el texto, la hora y la repetición (y lo deja pendiente).
func (a *Almacen) EditarRecordatorio(botID, id int64, texto string, proxima int64, repetir string) error {
	texto = strings.TrimSpace(texto)
	if texto == "" || proxima <= 0 {
		return errors.New("falta el texto o la fecha")
	}
	if len([]rune(texto)) > 500 {
		return errors.New("el texto es muy largo (máximo 500 letras)")
	}
	repetir, err := validaRepeticion(repetir)
	if err != nil {
		return err
	}
	sel, err := cifrado.SellarTexto(a.clave, "recordatorio", []byte(texto))
	if err != nil {
		return err
	}
	r, err := a.db.Exec(`UPDATE recordatorios SET texto_cifrado=?, proxima_ts=?, repetir=?, estado=? WHERE bot_id=? AND id=?`, sel, proxima, repetir, RecPendiente, botID, id)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return errors.New("ese recordatorio no existe")
	}
	a.avisa("recordatorios")
	return nil
}

// BorrarRecordatorio quita un recordatorio.
func (a *Almacen) BorrarRecordatorio(botID, id int64) error {
	_, err := a.db.Exec(`DELETE FROM recordatorios WHERE bot_id=? AND id=?`, botID, id)
	if err == nil {
		a.avisa("recordatorios")
	}
	return err
}

func (a *Almacen) leerRecordatorios(q string, args ...any) ([]Recordatorio, error) {
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []Recordatorio{}
	for filas.Next() {
		var x Recordatorio
		var t []byte
		if filas.Scan(&x.ID, &t, &x.ProximaTS, &x.Repetir, &x.Estado, &x.CreadoTS, &x.AvisadoTS) != nil {
			continue
		}
		if b, e := cifrado.AbrirTexto(a.clave, "recordatorio", t); e == nil {
			x.Texto = string(b)
		}
		out = append(out, x)
	}
	return out, filas.Err()
}

const colsRec = "id,texto_cifrado,proxima_ts,repetir,estado,creado_ts,avisado_ts"

// ListarRecordatorios devuelve los pendientes por fecha; con `hechos` también los que ya sonaron (los últimos 50).
func (a *Almacen) ListarRecordatorios(botID int64, hechos bool) ([]Recordatorio, error) {
	pend, err := a.leerRecordatorios(`SELECT `+colsRec+` FROM recordatorios WHERE bot_id=? AND estado=? ORDER BY proxima_ts, id`, botID, RecPendiente)
	if err != nil || !hechos {
		return pend, err
	}
	hec, err := a.leerRecordatorios(`SELECT `+colsRec+` FROM recordatorios WHERE bot_id=? AND estado=? ORDER BY avisado_ts DESC, id DESC LIMIT 50`, botID, RecHecho)
	return append(pend, hec...), err
}

// RecordatoriosVencidos devuelve los pendientes que ya tocan.
func (a *Almacen) RecordatoriosVencidos(botID int64, ahora time.Time) ([]Recordatorio, error) {
	return a.leerRecordatorios(`SELECT `+colsRec+` FROM recordatorios WHERE bot_id=? AND estado=? AND proxima_ts<=? ORDER BY proxima_ts, id`, botID, RecPendiente, ahora.Unix())
}

// SiguienteRecordatorio da la hora del próximo pendiente (0 si no hay).
func (a *Almacen) SiguienteRecordatorio(botID int64) int64 {
	var t *int64
	_ = a.db.QueryRow(`SELECT MIN(proxima_ts) FROM recordatorios WHERE bot_id=? AND estado=?`, botID, RecPendiente).Scan(&t)
	if t == nil {
		return 0
	}
	return *t
}

// ProximaRepeticion calcula la siguiente hora (mismo reloj de pared en la zona del teléfono) que esté en el futuro.
func (a *Almacen) ProximaRepeticion(desde int64, repetir string, ahora time.Time) int64 {
	paso := 1
	if repetir == RepiteSemanal {
		paso = 7
	} else if repetir != RepiteDiaria {
		return 0
	}
	t := time.Unix(desde, 0).In(a.loc())
	for i := 0; i < 4000 && !t.After(ahora); i++ {
		t = t.AddDate(0, 0, paso)
	}
	return t.Unix()
}

// MarcarRecordatorioAvisado anota que sonó: si se repite pasa a la siguiente vez; si no, queda hecho.
func (a *Almacen) MarcarRecordatorioAvisado(botID int64, r Recordatorio, ahora time.Time) error {
	var err error
	if r.Repetir == RepiteNunca {
		_, err = a.db.Exec(`UPDATE recordatorios SET estado=?, avisado_ts=? WHERE bot_id=? AND id=?`, RecHecho, ahora.Unix(), botID, r.ID)
	} else {
		_, err = a.db.Exec(`UPDATE recordatorios SET proxima_ts=?, avisado_ts=? WHERE bot_id=? AND id=?`, a.ProximaRepeticion(r.ProximaTS, r.Repetir, ahora), ahora.Unix(), botID, r.ID)
	}
	if err == nil {
		a.avisa("recordatorios")
	}
	return err
}

// LimpiarRecordatoriosHechos borra los hechos de hace más de `dias` días.
func (a *Almacen) LimpiarRecordatoriosHechos(botID int64, dias int, ahora time.Time) {
	_, _ = a.db.Exec(`DELETE FROM recordatorios WHERE bot_id=? AND estado=? AND avisado_ts<?`, botID, RecHecho, ahora.AddDate(0, 0, -dias).Unix())
}

// ---------- alertas por palabra clave ----------

// AlertaPalabra avisa cuando llega un mensaje con la palabra o frase, en todos los chats o solo en los elegidos.
type AlertaPalabra struct {
	ID      int64    `json:"id"`
	Palabra string   `json:"palabra"`
	Chats   []string `json:"chats"` // vacío = todos
	Activa  bool     `json:"activa"`
}

// CrearAlertaPalabra guarda una palabra (cifrada) con los chats donde vale (vacío = todos).
func (a *Almacen) CrearAlertaPalabra(botID int64, palabra string, chats []string) (int64, error) {
	palabra = strings.TrimSpace(palabra)
	if identidad.Normalizar(palabra) == "" {
		return 0, errors.New("escribe la palabra o frase")
	}
	if len([]rune(palabra)) > 60 {
		return 0, errors.New("la palabra es muy larga (máximo 60 letras)")
	}
	todas, _ := a.ListarAlertasPalabra(botID)
	if len(todas) >= 100 {
		return 0, errors.New("ya tienes 100 palabras")
	}
	for _, x := range todas {
		if identidad.Normalizar(x.Palabra) == identidad.Normalizar(palabra) {
			return 0, errors.New("esa palabra ya está en la lista")
		}
	}
	sel, err := cifrado.SellarTexto(a.clave, "alerta_palabra", []byte(palabra))
	if err != nil {
		return 0, err
	}
	cj, _ := json.Marshal(nuncaNil(chats))
	r, err := a.db.Exec(`INSERT INTO alertas_palabra(bot_id,palabra_cifrada,chats,activa,creado_ts) VALUES(?,?,?,1,?)`, botID, sel, string(cj), time.Now().Unix())
	if err != nil {
		return 0, err
	}
	id, _ := r.LastInsertId()
	a.avisa("alertas")
	return id, nil
}

func nuncaNil(x []string) []string {
	if x == nil {
		return []string{}
	}
	return x
}

// EditarAlertaPalabra cambia los chats y si está activa.
func (a *Almacen) EditarAlertaPalabra(botID, id int64, chats []string, activa bool) error {
	cj, _ := json.Marshal(nuncaNil(chats))
	r, err := a.db.Exec(`UPDATE alertas_palabra SET chats=?, activa=? WHERE bot_id=? AND id=?`, string(cj), b2i(activa), botID, id)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return errors.New("esa palabra no existe")
	}
	a.avisa("alertas")
	return nil
}

// BorrarAlertaPalabra quita una palabra.
func (a *Almacen) BorrarAlertaPalabra(botID, id int64) error {
	_, err := a.db.Exec(`DELETE FROM alertas_palabra WHERE bot_id=? AND id=?`, botID, id)
	if err == nil {
		a.avisa("alertas")
	}
	return err
}

// ListarAlertasPalabra devuelve la lista.
func (a *Almacen) ListarAlertasPalabra(botID int64) ([]AlertaPalabra, error) {
	filas, err := a.db.Query(`SELECT id,palabra_cifrada,chats,activa FROM alertas_palabra WHERE bot_id=? ORDER BY id`, botID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []AlertaPalabra{}
	for filas.Next() {
		var x AlertaPalabra
		var b []byte
		var cj string
		var act int
		if filas.Scan(&x.ID, &b, &cj, &act) != nil {
			continue
		}
		if t, e := cifrado.AbrirTexto(a.clave, "alerta_palabra", b); e == nil {
			x.Palabra = string(t)
		}
		_ = json.Unmarshal([]byte(cj), &x.Chats)
		x.Chats, x.Activa = nuncaNil(x.Chats), act == 1
		out = append(out, x)
	}
	return out, filas.Err()
}

// ---------- resumen diario ----------

// DatosResumen son los números del resumen del día (contados de la base, sin servidor).
type DatosResumen struct {
	Dia          string `json:"dia"`
	Recibidos    int    `json:"recibidos"`
	Chats        int    `json:"chats"` // chats distintos que escribieron hoy
	Pendientes   int    `json:"pendientes"`
	Solicitudes  int    `json:"solicitudes"`
	Advertencias int    `json:"advertencias"`
}

// ResumenDelDia cuenta lo del día de `ahora` (en la zona del teléfono).
func (a *Almacen) ResumenDelDia(botID int64, ahora time.Time) DatosResumen {
	t := ahora.In(a.loc())
	dia := t.Format("2006-01-02")
	ini := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, a.loc())
	d := DatosResumen{Dia: dia}
	_ = a.db.QueryRow(`SELECT COALESCE(SUM(count),0) FROM daily_stats WHERE bot_id=? AND day=? AND sender<>?`, botID, dia, QuienYo).Scan(&d.Recibidos)
	_ = a.db.QueryRow(`SELECT COUNT(DISTINCT chat) FROM daily_stats WHERE bot_id=? AND day=? AND sender<>?`, botID, dia, QuienYo).Scan(&d.Chats)
	d.Pendientes = a.ContarPendientes(botID)
	d.Solicitudes = a.ContarSolicitudes(botID, SolPendiente)
	d.Advertencias = a.AdvertenciasDesde(botID, ini.Unix())
	return d
}
