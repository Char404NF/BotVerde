package almacen

import "time"

// EstadoPersonaLista es lo que pinta un puntito en una fila de la lista de Personas.
type EstadoPersonaLista struct {
	Silenciado bool `json:"silenciado"` // silencio de grupos vigente
	Ignorada   bool `json:"ignorada"`   // el bot no le contesta (persona_silencio vigente)
	Advertido  bool `json:"advertido"`  // tiene advertencias vigentes
}

// EstadosDePersonas devuelve, para una lista de personas (chats privados o JIDs), quién está silenciada, ignorada o advertida.
// Solo trae a las que tienen algo (las demás no salen). Son tres consultas, sin importar cuántas personas se pidan.
func (a *Almacen) EstadosDePersonas(botID int64, personas []string, ahora time.Time) map[string]EstadoPersonaLista {
	out := map[string]EstadoPersonaLista{}
	if len(personas) == 0 {
		return out
	}
	silenciadas := map[string]bool{}
	if xs, err := a.SilenciosActivos(botID, "", ahora); err == nil {
		for _, s := range xs {
			silenciadas[a.ResolverPersona(botID, s.Persona)] = true
		}
	}
	ignoradas := map[string]bool{}
	if filas, err := a.db.Query(`SELECT persona FROM persona_silencio WHERE bot_id=? AND (hasta_ts=0 OR hasta_ts>?)`, botID, ahora.Unix()); err == nil {
		for filas.Next() {
			var p string
			if filas.Scan(&p) == nil {
				ignoradas[a.ResolverPersona(botID, p)] = true
			}
		}
		filas.Close()
	}
	advertidas := map[string]bool{}
	if filas, err := a.db.Query(`SELECT DISTINCT persona FROM advertencias WHERE bot_id=? AND (caduca_ts=0 OR caduca_ts>?)`, botID, ahora.Unix()); err == nil {
		for filas.Next() {
			var p string
			if filas.Scan(&p) == nil {
				advertidas[a.ResolverPersona(botID, p)] = true
			}
		}
		filas.Close()
	}
	if len(silenciadas)+len(ignoradas)+len(advertidas) == 0 {
		return out
	}
	for _, p := range personas {
		c := a.ResolverPersona(botID, p)
		e := EstadoPersonaLista{Silenciado: silenciadas[c], Ignorada: ignoradas[c], Advertido: advertidas[c]}
		if e.Silenciado || e.Ignorada || e.Advertido {
			out[p] = e
		}
	}
	return out
}
