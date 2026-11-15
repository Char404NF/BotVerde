package almacen

// ResumenGrupoSemana son los números de un grupo en una ventana de tiempo (para el resumen semanal).
type ResumenGrupoSemana struct {
	Chat       string `json:"chat"`
	Nombre     string `json:"nombre"`
	Mensajes   int    `json:"mensajes"`
	Personas   int    `json:"personas"`
	Entraron   int    `json:"entraron"`
	Salieron   int    `json:"salieron"`
	Expulsados int    `json:"expulsados"`
}

// ResumenSemana da, de los grupos con actividad desde `desde` (unix), los más activos primero (máximo `limite`).
func (a *Almacen) ResumenSemana(botID, desde int64, limite int) ([]ResumenGrupoSemana, error) {
	if limite <= 0 {
		limite = 5
	}
	filas, err := a.db.Query(`SELECT m.chat_jid, COUNT(*), COUNT(DISTINCT m.remitente_jid) FROM messages m
		JOIN chats c ON c.jid=m.chat_jid AND c.bot_id=m.bot_id
		WHERE m.bot_id=? AND c.es_grupo=1 AND m.ts>=? AND m.es_mio=0 AND m.borrado=0
		GROUP BY m.chat_jid ORDER BY 2 DESC LIMIT ?`, botID, desde, limite)
	if err != nil {
		return nil, err
	}
	out := []ResumenGrupoSemana{}
	for filas.Next() {
		var x ResumenGrupoSemana
		if err := filas.Scan(&x.Chat, &x.Mensajes, &x.Personas); err != nil {
			filas.Close()
			return nil, err
		}
		out = append(out, x)
	}
	filas.Close()
	for i := range out {
		if c, _ := a.ObtenerChat(botID, out[i].Chat); c != nil {
			out[i].Nombre = c.Mostrado
		}
		if out[i].Nombre == "" {
			out[i].Nombre = "Un grupo"
		}
		f2, err := a.db.Query(`SELECT tipo, COUNT(*) FROM grupo_cambios WHERE bot_id=? AND grupo=? AND ts>=? GROUP BY tipo`, botID, out[i].Chat, desde)
		if err != nil {
			continue
		}
		for f2.Next() {
			var t string
			var n int
			if f2.Scan(&t, &n) != nil {
				continue
			}
			switch t {
			case CambioEntra, CambioEnlace, CambioAgregado:
				out[i].Entraron += n
			case CambioSale:
				out[i].Salieron += n
			case CambioExpulsado:
				out[i].Expulsados += n
			}
		}
		f2.Close()
	}
	return out, nil
}

// ContarAcciones cuenta las acciones del registro desde `desde` cuyo nombre empieza con `prefijo` y terminaron bien.
func (a *Almacen) ContarAcciones(botID, desde int64, prefijo string) int {
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM registro_acciones WHERE bot_id=? AND ts>=? AND accion LIKE ? AND resultado='ok'`, botID, desde, prefijo+"%").Scan(&n)
	return n
}
