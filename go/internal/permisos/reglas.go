package permisos

import (
	"fmt"
	"sort"
	"strings"

	"botchat/internal/almacen"
)

// Modos de alcance de una casilla.
const (
	AlcanceTodo    = ""
	AlcancePrivado = "privado"
	AlcanceGrupos  = "grupos"
	AlcanceChats   = "chats"
)

// Permite dice si el rango tiene la casilla en ese lugar. Un rango nil es "sin rango": solo tiene las casillas básicas.
// El Dueño lo puede todo. El alcance de la casilla se junta con lo que la propia función admite (privado y/o grupo).
func Permite(r *almacen.Rol, casilla string, esGrupo bool, chat string) bool {
	def, conocida := Buscar(casilla)
	if r != nil && r.EsDueno {
		return true
	}
	if conocida {
		if esGrupo && !def.Grupos {
			return false
		}
		if !esGrupo && !def.Privado {
			return false
		}
		if def.Base {
			return true
		}
	}
	if r == nil {
		return false
	}
	al, ok := r.Casillas[casilla]
	if !ok {
		return false
	}
	switch al.Modo {
	case AlcancePrivado:
		return !esGrupo
	case AlcanceGrupos:
		return esGrupo
	case AlcanceChats:
		for _, c := range al.Chats {
			if c == chat {
				return true
			}
		}
		return false
	}
	return true
}

// PosicionDe es el lugar de un rango en la jerarquía (menor = más arriba). Sin rango es el último.
func PosicionDe(r *almacen.Rol) int {
	if r == nil {
		return 1 << 30
	}
	if r.EsDueno {
		return 0
	}
	return r.Posicion
}

// PuedeActuar dice si alguien con el rango `actor` puede actuar sobre alguien con el rango `objetivo`: solo sobre rangos más abajo.
// El Dueño actúa sobre todos y nadie toca al Dueño.
func PuedeActuar(actor, objetivo *almacen.Rol) bool {
	if objetivo != nil && objetivo.EsDueno {
		return false
	}
	if actor == nil {
		return false
	}
	if actor.EsDueno {
		return true
	}
	return PosicionDe(actor) < PosicionDe(objetivo)
}

// PuedeDar dice si `actor` puede dar el rango `destino` a alguien (solo rangos más abajo del suyo; el Dueño nunca se da).
func PuedeDar(actor, destino *almacen.Rol) bool {
	if destino == nil || destino.EsDueno || actor == nil {
		return false
	}
	return actor.EsDueno || PosicionDe(actor) < PosicionDe(destino)
}

// MenuVisible dice si el rango ve ese menú (o esa opción: "menu:opcion"). Sin rango se ve todo lo que no esté restringido.
func MenuVisible(r *almacen.Rol, menuID string, opcion string) bool {
	if r == nil || r.EsDueno {
		return true
	}
	switch r.MenusModo {
	case "ninguno":
		return false
	case "lista":
		for _, m := range r.Menus {
			if m == menuID || (opcion != "" && m == menuID+":"+opcion) {
				return true
			}
			// sin pedir una opción: el menú se ve si el rango ve al menos una de sus opciones sueltas ("menú:opción")
			if opcion == "" && strings.HasPrefix(m, menuID+":") {
				return true
			}
		}
		return false
	}
	return true
}

// OpcionVisible dice si el rango ve esa opción de un menú: si ve el menú entero (el id solo) las ve todas; si solo tiene opciones
// sueltas ("idMenu:idOpción"), solo esas.
func OpcionVisible(r *almacen.Rol, menuID, opcionID string) bool {
	return MenuVisible(r, menuID, opcionID)
}

// ValidarRol revisa un rango antes de guardarlo: casillas que existen, alcances coherentes y colores bien escritos.
func ValidarRol(r *almacen.Rol) error {
	r.Nombre = strings.TrimSpace(r.Nombre)
	if r.Nombre == "" {
		return fmt.Errorf("falta el nombre del rango")
	}
	if len([]rune(r.Nombre)) > 30 {
		return fmt.Errorf("el nombre es muy largo (máximo 30 letras)")
	}
	if strings.EqualFold(strings.TrimSpace(r.Nombre), "dueño") || strings.EqualFold(strings.TrimSpace(r.Nombre), "dueno") {
		return fmt.Errorf("ese nombre está reservado para el Dueño")
	}
	if r.Color != "" && !colorValido(r.Color) {
		return fmt.Errorf("color no válido")
	}
	if r.LimiteMin < 0 || r.LimiteMin > 600 {
		return fmt.Errorf("el límite por minuto debe estar entre 0 y 600")
	}
	switch r.MenusModo {
	case "", "todos", "lista", "ninguno":
	default:
		return fmt.Errorf("modo de menús no válido")
	}
	var malas []string
	for id, al := range r.Casillas {
		def, ok := Buscar(id)
		if !ok {
			malas = append(malas, id)
			continue
		}
		switch al.Modo {
		case AlcanceTodo:
		case AlcancePrivado:
			if !def.Privado {
				return fmt.Errorf("«%s» no funciona en chats privados", def.Nombre)
			}
		case AlcanceGrupos:
			if !def.Grupos {
				return fmt.Errorf("«%s» no funciona en grupos", def.Nombre)
			}
		case AlcanceChats:
			if len(al.Chats) == 0 {
				return fmt.Errorf("«%s»: elige al menos un chat", def.Nombre)
			}
		default:
			return fmt.Errorf("alcance no válido en «%s»", def.Nombre)
		}
	}
	if len(malas) > 0 {
		sort.Strings(malas)
		return fmt.Errorf("casillas que no existen: %s", strings.Join(malas, ", "))
	}
	return nil
}

func colorValido(c string) bool {
	if len(c) != 7 || c[0] != '#' {
		return false
	}
	for _, x := range c[1:] {
		if !(x >= '0' && x <= '9' || x >= 'a' && x <= 'f' || x >= 'A' && x <= 'F') {
			return false
		}
	}
	return true
}

// RolAdmin es el rango «Admin» de fábrica: modera los grupos (silenciar, advertir, expulsar, borrar mensajes, cerrar el grupo, filtros) y
// atiende chats. No puede actuar sobre el Dueño ni sobre otros Admin (la jerarquía solo deja actuar hacia abajo). No da ni quita rangos.
func RolAdmin() almacen.Rol {
	m := map[string]almacen.Alcance{}
	for _, id := range []string{
		"personas.silenciar", "personas.advertir", "personas.notas", "personas.ficha", "chat.pausar", "chat.tomar_control", "chat.estado", "chat.etiquetas",
		"grupos.info", "grupos.admins", "grupos.mencionar_todos", "grupos.ranking", "grupos.inactivos", "grupos.cerrar", "grupos.expulsar", "grupos.banear", "grupos.horario",
		"grupos.borrar_mensaje", "grupos.solicitudes", "silencio.silenciar", "silencio.ver", "grupos.filtros", "util.notas", "util.pendientes", "rangos.ver",
	} {
		m[id] = almacen.Alcance{}
	}
	return almacen.Rol{Nombre: "Admin", Color: "#E0A100", Plantilla: "admin", LimiteMin: 30, MenusModo: "todos", Casillas: m}
}

// Plantillas son los rangos de ejemplo que se crean la primera vez (se pueden editar o borrar).
func Plantillas() []almacen.Rol {
	solo := func(ids ...string) map[string]almacen.Alcance {
		m := map[string]almacen.Alcance{}
		for _, id := range ids {
			m[id] = almacen.Alcance{}
		}
		return m
	}
	return []almacen.Rol{
		RolAdmin(),
		{
			Nombre: "Ayudante", Color: "#2EC4CB", Plantilla: "ayudante", LimiteMin: 20, MenusModo: "todos",
			Casillas: solo("personas.silenciar", "personas.advertir", "personas.notas", "personas.ficha", "chat.pausar", "chat.tomar_control", "chat.estado",
				"chat.etiquetas", "grupos.info", "grupos.admins", "grupos.ranking", "grupos.inactivos", "util.notas", "util.pendientes", "menus.formularios", "rangos.ver"),
		},
		{
			Nombre: "Cliente VIP", Color: "#F59E28", Plantilla: "cliente_vip", LimiteMin: 10, MenusModo: "todos",
			Casillas: map[string]almacen.Alcance{
				"menus.formularios": {Modo: AlcancePrivado},
				"auto.prioridad":    {Modo: AlcancePrivado},
				"util.pendientes":   {Modo: AlcancePrivado},
			},
		},
	}
}
