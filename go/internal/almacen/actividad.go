package almacen

import (
	"time"
)

// Eventos y motivos del registro de actividad del motor. Nunca llevan textos de mensajes ni nombres: solo qué pasó y por qué.
const (
	ActLlego      = "llego"     // llegó un mensaje al motor
	ActCoincidio  = "coincidio" // coincidió con una automatización
	ActRespondio  = "respondio" // se respondió
	ActOmitido    = "omitido"   // no se respondió (ver motivo)
	ActManual     = "manual"    // envío escrito a mano desde la app (no es el bot)
	ActFreno      = "freno"     // el motor se pausó por errores de envío
	ActSinCoincid = "sin_regla" // no coincidió con nada
	MotDuplicado  = "duplicado"
	MotAntiguo    = "antiguo"
	MotLimite     = "limite"
	MotPausado    = "pausado"
	MotHumano     = "humano"
	MotApagado    = "motor apagado"
	MotGrupo      = "grupo sin mención"
	MotChatPausa  = "chat en pausa"
	MotRepetido   = "repetido"
	MotYaContesto = "ya contestaste tú"
	MotIntervino  = "intervención manual"
	MotCola       = "cola llena"
	MotFreno      = "freno"
	MotNoTexto    = "no es texto"
)

// RegistrarActividad anota un renglón en el registro de actividad. chat se guarda como un identificador corto, nunca el número.
func (a *Almacen) RegistrarActividad(botID int64, evento, motivo, chat, detalle string) {
	h := ""
	if chat != "" {
		h = HashChat(botID, chat)[:6]
	}
	_, _ = a.db.Exec(`INSERT INTO actividad(bot_id, ts, evento, motivo, chat, detalle) VALUES(?,?,?,?,?,?)`, botID, time.Now().Unix(), evento, motivo, h, detalle)
	a.avisa("actividad")
}

// Actividad es un renglón del registro.
type Actividad struct {
	TS      int64  `json:"ts"`
	Evento  string `json:"evento"`
	Motivo  string `json:"motivo"`
	Chat    string `json:"chat"`
	Detalle string `json:"detalle"`
}

// UltimaActividad devuelve los últimos renglones del registro, del más nuevo al más viejo.
func (a *Almacen) UltimaActividad(botID int64, limite int) ([]Actividad, error) {
	if limite <= 0 || limite > 1000 {
		limite = 200
	}
	filas, err := a.db.Query(`SELECT ts, evento, motivo, chat, detalle FROM actividad WHERE bot_id=? ORDER BY id DESC LIMIT ?`, botID, limite)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []Actividad{}
	for filas.Next() {
		var x Actividad
		if err := filas.Scan(&x.TS, &x.Evento, &x.Motivo, &x.Chat, &x.Detalle); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, filas.Err()
}

// LimpiarActividad borra el registro con más de `dias` días y recorta lo que sobre de 5000 renglones.
func (a *Almacen) LimpiarActividad(botID int64, dias int) {
	_, _ = a.db.Exec(`DELETE FROM actividad WHERE bot_id=? AND ts<?`, botID, time.Now().AddDate(0, 0, -dias).Unix())
	_, _ = a.db.Exec(`DELETE FROM actividad WHERE bot_id=? AND id NOT IN (SELECT id FROM actividad WHERE bot_id=? ORDER BY id DESC LIMIT 5000)`, botID, botID)
}

// PrimeraVez anota un mensaje como visto y dice si es la primera vez que se ve (para no contestarlo dos veces, aunque la app se reinicie).
func (a *Almacen) PrimeraVez(botID int64, chat, msgID string) bool {
	r, err := a.db.Exec(`INSERT OR IGNORE INTO dedupe(bot_id, chat_jid, msg_id, ts) VALUES(?,?,?,?)`, botID, chat, msgID, time.Now().Unix())
	if err != nil {
		return true // ante la duda, no se pierde el mensaje
	}
	n, _ := r.RowsAffected()
	return n > 0
}

// LimpiarVistos borra los mensajes vistos hace más de `dias` días.
func (a *Almacen) LimpiarVistos(botID int64, dias int) int64 {
	r, err := a.db.Exec(`DELETE FROM dedupe WHERE bot_id=? AND ts<?`, botID, time.Now().AddDate(0, 0, -dias).Unix())
	if err != nil {
		return 0
	}
	n, _ := r.RowsAffected()
	return n
}
