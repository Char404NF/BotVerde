package almacen

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"botchat/internal/cifrado"
)

// EstadoChat es lo que la app guarda de cada chat aparte de sus mensajes.
type EstadoChat struct {
	LeidoEnviadoTS int64  `json:"leidoEnviadoTs"` // hasta qué hora se avisó "leído" a WhatsApp
	Silenciado     bool   `json:"silenciado"`     // sin notificaciones de este chat
	BotPausaHasta  int64  `json:"botPausaHasta"`  // el bot calla en este chat hasta esta hora (0 = no)
	BotActivo      bool   `json:"botActivo"`      // interruptor del bot en este chat (para grupos)
	PausaHumano    bool   `json:"pausaHumano"`    // la pausa es porque alguien pidió hablar con una persona (no porque escribí yo)
	EstadoTrabajo  string `json:"estadoTrabajo"`  // "" (sin marca), "pendiente" o "resuelto"
}

func (a *Almacen) asegurarEstado(botID int64, chat string) {
	_, _ = a.db.Exec(`INSERT OR IGNORE INTO chat_estado(bot_id, chat_jid) VALUES(?,?)`, botID, chat)
}

// EstadoDeChat devuelve el estado del chat (con valores por defecto si no hay registro).
func (a *Almacen) EstadoDeChat(botID int64, chat string) EstadoChat {
	var e EstadoChat
	var sil, act, hum int
	err := a.db.QueryRow(`SELECT leido_enviado_ts, silenciado, bot_pausa_hasta, bot_activo, pausa_humano, estado_trabajo FROM chat_estado WHERE bot_id=? AND chat_jid=?`, botID, chat).
		Scan(&e.LeidoEnviadoTS, &sil, &e.BotPausaHasta, &act, &hum, &e.EstadoTrabajo)
	if errors.Is(err, sql.ErrNoRows) {
		return EstadoChat{BotActivo: true}
	}
	e.Silenciado, e.BotActivo, e.PausaHumano = sil == 1, act == 1, hum == 1
	return e
}

// Silenciar enciende o apaga las notificaciones de un chat.
func (a *Almacen) Silenciar(botID int64, chat string, si bool) error {
	a.asegurarEstado(botID, chat)
	_, err := a.db.Exec(`UPDATE chat_estado SET silenciado=? WHERE bot_id=? AND chat_jid=?`, b2i(si), botID, chat)
	if err == nil {
		a.avisa("chats")
	}
	return err
}

// PausarBot hace que el bot calle en un chat hasta `hasta` (segundos desde 1970). Con 0 se reactiva.
func (a *Almacen) PausarBot(botID int64, chat string, hasta int64) error {
	return a.pausarBot(botID, chat, hasta, false)
}

// PausarBotHumano hace que el bot calle en un chat porque alguien pidió hablar con una persona, hasta `hasta` (0 = reactivar).
func (a *Almacen) PausarBotHumano(botID int64, chat string, hasta int64) error {
	return a.pausarBot(botID, chat, hasta, true)
}

func (a *Almacen) pausarBot(botID int64, chat string, hasta int64, humano bool) error {
	a.asegurarEstado(botID, chat)
	_, err := a.db.Exec(`UPDATE chat_estado SET bot_pausa_hasta=?, pausa_humano=? WHERE bot_id=? AND chat_jid=?`, hasta, b2i(humano && hasta > 0), botID, chat)
	if err == nil {
		a.avisa("chat:" + chat)
	}
	return err
}

// BotPausado dice si el bot debe callar ahora en ese chat.
func (a *Almacen) BotPausado(botID int64, chat string, ahora time.Time) bool {
	return a.EstadoDeChat(botID, chat).BotPausaHasta > ahora.Unix()
}

// FijarBotActivo enciende o apaga el bot en un chat (interruptor Activo / Pausado de los grupos).
func (a *Almacen) FijarBotActivo(botID int64, chat string, si bool) error {
	a.asegurarEstado(botID, chat)
	_, err := a.db.Exec(`UPDATE chat_estado SET bot_activo=? WHERE bot_id=? AND chat_jid=?`, b2i(si), botID, chat)
	if err == nil {
		a.avisa("chat:" + chat)
	}
	return err
}

// GuardarBorrador guarda (cifrado) lo que se estaba escribiendo en un chat; texto vacío lo borra.
func (a *Almacen) GuardarBorrador(botID int64, chat, texto string) error {
	a.asegurarEstado(botID, chat)
	var v any
	if strings.TrimSpace(texto) != "" {
		b, err := cifrado.SellarTexto(a.clave, "borrador", []byte(texto))
		if err != nil {
			return err
		}
		v = b
	}
	_, err := a.db.Exec(`UPDATE chat_estado SET borrador_cifrado=? WHERE bot_id=? AND chat_jid=?`, v, botID, chat)
	return err
}

// Borrador devuelve el borrador guardado ("" si no hay).
func (a *Almacen) Borrador(botID int64, chat string) string {
	var b []byte
	_ = a.db.QueryRow(`SELECT borrador_cifrado FROM chat_estado WHERE bot_id=? AND chat_jid=?`, botID, chat).Scan(&b)
	if len(b) == 0 {
		return ""
	}
	t, err := cifrado.AbrirTexto(a.clave, "borrador", b)
	if err != nil {
		return ""
	}
	return string(t)
}

// MarcarLeidoHasta anota hasta qué hora ya se avisó "leído".
func (a *Almacen) MarcarLeidoHasta(botID int64, chat string, ts int64) error {
	a.asegurarEstado(botID, chat)
	_, err := a.db.Exec(`UPDATE chat_estado SET leido_enviado_ts=MAX(leido_enviado_ts,?) WHERE bot_id=? AND chat_jid=?`, ts, botID, chat)
	return err
}

// PorMarcarLeido son los mensajes recibidos que todavía no se avisaron como leídos, por remitente.
type PorMarcarLeido struct {
	Remitente string
	IDs       []string
	UltimoTS  int64
}

// PendientesDeLeido lista los mensajes recibidos después del último aviso de lectura (hasta 200).
func (a *Almacen) PendientesDeLeido(botID int64, chat string) ([]PorMarcarLeido, error) {
	desde := a.EstadoDeChat(botID, chat).LeidoEnviadoTS
	filas, err := a.db.Query(`SELECT id, remitente_jid, ts FROM messages WHERE bot_id=? AND chat_jid=? AND es_mio=0 AND ts>? AND borrado=0 ORDER BY ts LIMIT 200`, botID, chat, desde)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	var out []PorMarcarLeido
	idx := map[string]int{}
	for filas.Next() {
		var id, rem string
		var ts int64
		if err := filas.Scan(&id, &rem, &ts); err != nil {
			return nil, err
		}
		i, ok := idx[rem]
		if !ok {
			i = len(out)
			idx[rem] = i
			out = append(out, PorMarcarLeido{Remitente: rem})
		}
		out[i].IDs = append(out[i].IDs, id)
		if ts > out[i].UltimoTS {
			out[i].UltimoTS = ts
		}
	}
	return out, filas.Err()
}
