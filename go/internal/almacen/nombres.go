package almacen

import (
	"strings"
	"sync"
	"sync/atomic"
)

// Nombres de personas para mostrar en una página de mensajes (autor de una cita, mencionados, quien eliminó).
// Se resuelven por lote (una consulta por chat y no una por mensaje) y se guardan en una caché pequeña que se invalida
// cuando cambia el contacto (ActualizarChat: nombre guardado, push name, negocio), cuando llega un mensaje con nombre
// de esa persona y cuando se borran o fusionan chats.

type claveNombre struct {
	bot  int64
	chat string
	jid  string
	vis  bool // true = nombre para mostrar (contacto o, si no hay, el del chat); false = el último nombre visto en mensajes
}

type cacheNombres struct {
	mu sync.Mutex
	m  map[claveNombre]string
}

const maxNombres = 4000

func (c *cacheNombres) get(k claveNombre) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[k]
	return v, ok
}

func (c *cacheNombres) put(k claveNombre, v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil || len(c.m) >= maxNombres {
		c.m = map[claveNombre]string{}
	}
	c.m[k] = v
}

func (c *cacheNombres) limpiar() {
	c.mu.Lock()
	c.m = nil
	c.mu.Unlock()
}

// invalidaContacto olvida los nombres "para mostrar" de esa persona en cualquier chat.
func (c *cacheNombres) invalidaContacto(bot int64, jid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.m {
		if k.bot == bot && k.jid == jid {
			delete(c.m, k)
		}
	}
}

// invalidaRemitente olvida los dos nombres de una persona en un chat (llegó un mensaje con nombre nuevo).
func (c *cacheNombres) invalidaRemitente(bot int64, chat, jid string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, claveNombre{bot, chat, jid, false})
	delete(c.m, claveNombre{bot, chat, jid, true})
}

// cargaAntigua reproduce la forma de cargar de la 0.4.0 (una consulta por nombre y por mensaje, sin caché), solo para comparar.
var cargaAntigua atomic.Bool

// CargaAntigua activa o apaga la ruta antigua de carga (medición "antes y después").
func CargaAntigua(si bool) { cargaAntigua.Store(si) }

// nombresPagina guarda los nombres ya resueltos de una página.
type nombresPagina struct {
	vis, msg map[claveNombre]string
}

func (n *nombresPagina) mostrado(bot int64, chat, jid string) string {
	if jid == QuienYo || jid == "" {
		return "Tú"
	}
	return n.vis[claveNombre{bot, chat, jid, true}]
}

func (n *nombresPagina) deMensajes(bot int64, chat, jid string) string {
	return n.msg[claveNombre{bot, chat, jid, false}]
}

// resolverNombres arma los nombres que necesitan los mensajes de la página, con una consulta por chat para los contactos
// y otra para los nombres vistos en mensajes.
func (a *Almacen) resolverNombres(ms []*Mensaje) *nombresPagina {
	res := &nombresPagina{vis: map[claveNombre]string{}, msg: map[claveNombre]string{}}
	type grupo struct {
		bot  int64
		chat string
	}
	porGrupo := map[grupo]struct{ vis, msg map[string]bool }{}
	pide := func(g grupo) struct{ vis, msg map[string]bool } {
		x, ok := porGrupo[g]
		if !ok {
			x = struct{ vis, msg map[string]bool }{map[string]bool{}, map[string]bool{}}
			porGrupo[g] = x
		}
		return x
	}
	for _, m := range ms {
		g := grupo{m.BotID, m.ChatJID}
		if m.EliminadoPor != "" && m.EliminadoPor != m.RemitenteJID {
			pide(g).msg[m.EliminadoPor] = true
		}
		if m.CitaID != "" && m.CitaAutor != QuienYo && m.CitaAutor != "" {
			pide(g).vis[m.CitaAutor] = true
		}
		for _, j := range m.Menciones {
			if j != QuienYo && j != "" {
				pide(g).vis[j] = true
			}
		}
	}
	for g, x := range porGrupo {
		// Del caché lo que haya.
		faltaVis, faltaMsg := []string{}, []string{}
		for j := range x.vis {
			k := claveNombre{g.bot, g.chat, j, true}
			if v, ok := a.nombres.get(k); ok {
				res.vis[k] = v
			} else {
				faltaVis = append(faltaVis, j)
			}
		}
		for j := range x.msg {
			k := claveNombre{g.bot, g.chat, j, false}
			if v, ok := a.nombres.get(k); ok {
				res.msg[k] = v
			} else {
				faltaMsg = append(faltaMsg, j)
			}
		}
		if len(faltaVis) == 0 && len(faltaMsg) == 0 {
			continue
		}
		// Contactos (una consulta), y lo que no tenga nombre de contacto se busca entre los mensajes.
		contactos := a.nombresDeContactos(g.bot, faltaVis)
		paraMensajes := append([]string{}, faltaMsg...)
		for _, j := range faltaVis {
			if contactos[j] == "" {
				paraMensajes = append(paraMensajes, j)
			}
		}
		vistos := a.nombresDeMensajes(g.bot, g.chat, paraMensajes)
		nombreMsg := func(j string) string {
			if n := vistos[j]; n != "" {
				return n
			}
			if n := a.nombrePorAlias(g.bot, g.chat, j); n != "" {
				return n
			}
			return "Participante"
		}
		for _, j := range faltaMsg {
			k := claveNombre{g.bot, g.chat, j, false}
			v := nombreMsg(j)
			res.msg[k] = v
			a.nombres.put(k, v)
		}
		for _, j := range faltaVis {
			k := claveNombre{g.bot, g.chat, j, true}
			v := contactos[j]
			if v == "" {
				v = nombreMsg(j)
			}
			res.vis[k] = v
			a.nombres.put(k, v)
		}
	}
	return res
}

// nombresDeContactos devuelve el nombre mostrado de varios contactos con una sola consulta.
func (a *Almacen) nombresDeContactos(bot int64, jids []string) map[string]string {
	out := map[string]string{}
	for i := 0; i < len(jids); i += 400 {
		fin := min(i+400, len(jids))
		trozo := jids[i:fin]
		args := []any{bot}
		for _, j := range trozo {
			args = append(args, j)
		}
		filas, err := a.db.Query(`SELECT jid, nombre_mostrado FROM chats WHERE bot_id=? AND jid IN (`+marcas(len(trozo))+`)`, args...)
		if err != nil {
			continue
		}
		for filas.Next() {
			var j, n string
			if filas.Scan(&j, &n) == nil && strings.TrimSpace(n) != "" {
				out[j] = n
			}
		}
		filas.Close()
	}
	return out
}

// nombresDeMensajes devuelve el último nombre visto de cada persona en un chat, con una sola sentencia (un salto al
// índice por persona) y no una consulta por persona.
func (a *Almacen) nombresDeMensajes(bot int64, chat string, jids []string) map[string]string {
	out := map[string]string{}
	for i := 0; i < len(jids); i += 100 {
		fin := min(i+100, len(jids))
		trozo := jids[i:fin]
		partes := make([]string, 0, len(trozo))
		args := []any{}
		for _, j := range trozo {
			partes = append(partes, `SELECT * FROM (SELECT remitente_jid, remitente_nombre FROM messages WHERE bot_id=? AND chat_jid=? AND remitente_jid=? AND remitente_nombre<>'' ORDER BY ts DESC LIMIT 1)`)
			args = append(args, bot, chat, j)
		}
		filas, err := a.db.Query(strings.Join(partes, " UNION ALL "), args...)
		if err != nil {
			continue
		}
		for filas.Next() {
			var j, n string
			if filas.Scan(&j, &n) == nil {
				out[j] = n
			}
		}
		filas.Close()
	}
	return out
}

// nombrePorAlias busca el nombre de una persona por las otras formas de su identificador (número y LID): a veces la mención
// viene con una forma y sus mensajes con la otra. Si la persona es la dueña del bot se llama "Tú".
func (a *Almacen) nombrePorAlias(bot int64, chat, j string) string {
	if a.ResolverPersona(bot, j) == PersonaDueno {
		return "Tú"
	}
	var otros []string
	for _, x := range a.AliasDe(bot, j) {
		if x != j {
			otros = append(otros, x)
		}
	}
	if len(otros) == 0 {
		return ""
	}
	contactos := a.nombresDeContactos(bot, otros)
	for _, x := range otros {
		if v := contactos[x]; v != "" {
			return v
		}
	}
	vistos := a.nombresDeMensajes(bot, chat, otros)
	for _, x := range otros {
		if v := vistos[x]; v != "" {
			return v
		}
	}
	return ""
}
