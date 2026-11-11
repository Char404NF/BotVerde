package almacen

// ContarNoLeidos cuenta los mensajes de otras personas en un chat posteriores a `desdeTS` (segundos). Es un COUNT sobre el
// índice parcial idx_mensajes_noleidos: no recorre el chat aunque tenga cientos de miles de mensajes.
func (a *Almacen) ContarNoLeidos(botID int64, chat string, desdeTS int64) (int, error) {
	var n int
	err := a.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE bot_id=? AND chat_jid=? AND es_mio=0 AND ts>?`, botID, chat, desdeTS).Scan(&n)
	return n, err
}

// MarcarIntervencion anota que escribí a mano en este chat: las conversaciones de menú abiertas ahí quedan "intervenidas" y el
// bot no contesta "opción no válida" al texto libre hasta que la persona elija una opción válida o use un comando.
func (a *Almacen) MarcarIntervencion(botID int64, chat string) {
	r, err := a.db.Exec(`UPDATE flow_sessions SET intervencion=1 WHERE bot_id=? AND chat=? AND prueba=0 AND estado=? AND intervencion=0`, botID, chat, SesActiva)
	if err == nil {
		if n, _ := r.RowsAffected(); n > 0 {
			a.avisa("sesiones")
		}
	}
}

// SesionIntervenida dice si escribí a mano en esa conversación después de lo último que dijo el bot.
func (a *Almacen) SesionIntervenida(sesionID int64) bool {
	var v int
	_ = a.db.QueryRow(`SELECT intervencion FROM flow_sessions WHERE id=?`, sesionID).Scan(&v)
	return v == 1
}

// LimpiarIntervencion quita la marca: el bot vuelve a hablar en esa conversación (la persona eligió algo válido).
func (a *Almacen) LimpiarIntervencion(sesionID int64) {
	_, _ = a.db.Exec(`UPDATE flow_sessions SET intervencion=0 WHERE id=? AND intervencion=1`, sesionID)
}
