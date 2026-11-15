package almacen

import (
	"errors"
	"sort"
	"strings"
	"time"
)

// Copia de seguridad de la configuración: lo que el dueño dejó a su gusto (ajustes, respuestas propias, filtros, reglas, horarios,
// protección de entrada, baneados y la configuración base de los grupos). NO incluye mensajes, fotos, llaves ni contraseñas.

const versionRespaldoConfig = 1

// GrupoRespaldo es la configuración de un grupo dentro de la copia.
type GrupoRespaldo struct {
	Chat     string             `json:"chat"`
	Filtros  []FiltroGrupo      `json:"filtros,omitempty"`
	Reglas   string             `json:"reglas,omitempty"`
	Horario  *HorarioGrupo      `json:"horario,omitempty"`
	Entrada  *ProteccionEntrada `json:"entrada,omitempty"`
	Baneados []Baneado          `json:"baneados,omitempty"`
}

// RespaldoConfig es la copia de la configuración.
type RespaldoConfig struct {
	Version int               `json:"version"`
	Fecha   int64             `json:"fecha"`
	Ajustes map[string]string `json:"ajustes"`
	Grupos  []GrupoRespaldo   `json:"grupos"`
	Base    []byte            `json:"base,omitempty"` // la configuración base, tal cual (JSON)
}

// ResumenRestauracion cuenta lo que se restauró.
type ResumenRestauracion struct {
	Ajustes  int  `json:"ajustes"`
	Grupos   int  `json:"grupos"`
	Filtros  int  `json:"filtros"`
	Baneados int  `json:"baneados"`
	Base     bool `json:"base"`
}

// ajusteDeEstado son ajustes que guardan «lo último que pasó», no una decisión del dueño: no se copian.
func ajusteDeEstado(k string) bool {
	switch k {
	case "usuario_creado", "resumen_ultimo", "resumen_sem_ultimo", "adv_tipos_sembrados":
		return true
	}
	return false
}

// ajusteSecreto dice si el nombre parece guardar una llave o contraseña (nunca se copia).
func ajusteSecreto(k string) bool {
	k = strings.ToLower(k)
	for _, p := range []string{"clave", "key", "token", "secret", "pass", "contrasena", "contraseña"} {
		if strings.Contains(k, p) {
			return true
		}
	}
	return false
}

// ajusteCopiable dice si ese ajuste entra en la copia y se puede restaurar.
func ajusteCopiable(k string) bool {
	if ajusteDeEstado(k) || ajusteSecreto(k) {
		return false
	}
	if strings.HasPrefix(k, prefijoRespuesta) {
		return true
	}
	_, ok := ajustesBotDefecto[k]
	return ok
}

// ExportarConfig reúne la configuración del bot. `base` es la configuración base ya leída (puede ser nil).
func (a *Almacen) ExportarConfig(botID int64, base []byte, ahora time.Time) (*RespaldoConfig, error) {
	out := &RespaldoConfig{Version: versionRespaldoConfig, Fecha: ahora.Unix(), Ajustes: map[string]string{}, Grupos: []GrupoRespaldo{}, Base: base}
	filas, err := a.db.Query(`SELECT clave, valor FROM bot_ajustes WHERE bot_id=?`, botID)
	if err != nil {
		return nil, err
	}
	for filas.Next() {
		var k, v string
		if filas.Scan(&k, &v) == nil && ajusteCopiable(k) {
			out.Ajustes[k] = v
		}
	}
	filas.Close()
	chats := map[string]bool{}
	add := func(q string) {
		f, err := a.db.Query(q, botID)
		if err != nil {
			return
		}
		defer f.Close()
		for f.Next() {
			var c string
			if f.Scan(&c) == nil && strings.HasSuffix(c, "@g.us") {
				chats[c] = true
			}
		}
	}
	add(`SELECT DISTINCT chat_jid FROM grupo_filtros WHERE bot_id=?`)
	add(`SELECT DISTINCT grupo FROM grupo_horario WHERE bot_id=?`)
	add(`SELECT DISTINCT grupo FROM grupo_entrada WHERE bot_id=?`)
	add(`SELECT DISTINCT grupo FROM grupo_baneados WHERE bot_id=?`)
	add(`SELECT DISTINCT chat_jid FROM grupo_reglas WHERE bot_id=?`)
	lista := make([]string, 0, len(chats))
	for c := range chats {
		lista = append(lista, c)
	}
	sort.Strings(lista)
	for _, c := range lista {
		g := GrupoRespaldo{Chat: c, Reglas: a.ReglasGrupo(botID, c)}
		g.Filtros, _ = a.FiltrosDeGrupo(botID, c)
		for i := range g.Filtros {
			g.Filtros[i].Chat = c
		}
		if h := a.HorarioDe(botID, c); h.Activo || h.Cierra != "" {
			h.Ultimo = ""
			g.Horario = &h
		}
		if e := a.EntradaDe(botID, c); e.RaidActivo || len(e.Prefijos) > 0 {
			g.Entrada = &e
		}
		g.Baneados, _ = a.ListarBaneados(botID, c)
		out.Grupos = append(out.Grupos, g)
	}
	return out, nil
}

// RestaurarConfig pone la configuración de la copia. Lo que ya hay se reemplaza; lo que la copia no trae se deja como está.
// `guardaBase` recibe la configuración base si la copia la trae.
func (a *Almacen) RestaurarConfig(botID int64, r *RespaldoConfig, guardaBase func([]byte) error, ahora time.Time) (ResumenRestauracion, error) {
	var res ResumenRestauracion
	if r == nil || r.Version < 1 {
		return res, errors.New("la copia no es válida")
	}
	if r.Version > versionRespaldoConfig {
		return res, errors.New("la copia es de una versión más nueva de la app: actualiza la app y vuelve a intentarlo")
	}
	for k, v := range r.Ajustes {
		if !ajusteCopiable(k) {
			continue
		}
		if _, err := a.db.Exec(`INSERT INTO bot_ajustes(bot_id,clave,valor) VALUES(?,?,?) ON CONFLICT(bot_id,clave) DO UPDATE SET valor=excluded.valor`, botID, k, v); err != nil {
			return res, err
		}
		res.Ajustes++
	}
	for _, g := range r.Grupos {
		if !strings.HasSuffix(g.Chat, "@g.us") {
			continue
		}
		for _, f := range g.Filtros {
			f.Chat = g.Chat
			if err := a.GuardarFiltroGrupo(botID, f, ahora); err != nil {
				return res, err
			}
			res.Filtros++
		}
		if err := a.FijarReglasGrupo(botID, g.Chat, g.Reglas, ahora); err != nil {
			return res, err
		}
		if g.Horario != nil && ValidaHora(g.Horario.Cierra) && ValidaHora(g.Horario.Abre) {
			h := *g.Horario
			h.Grupo, h.Ultimo = g.Chat, ""
			if err := a.GuardarHorario(botID, h); err != nil {
				return res, err
			}
		}
		if g.Entrada != nil {
			e := *g.Entrada
			e.Grupo = g.Chat
			if err := a.GuardarEntrada(botID, e); err != nil {
				return res, err
			}
		}
		for _, b := range g.Baneados {
			if b.Persona == "" || a.EstaBaneado(botID, g.Chat, b.Persona) {
				continue
			}
			if err := a.Banear(botID, g.Chat, b.Persona, b.Por, b.Motivo, time.Unix(b.TS, 0)); err == nil {
				res.Baneados++
			}
		}
		res.Grupos++
	}
	if len(r.Base) > 0 && guardaBase != nil {
		if err := guardaBase(r.Base); err != nil {
			return res, err
		}
		res.Base = true
	}
	a.avisa("permisos_ajustes")
	a.avisa("respuestas")
	return res, nil
}
