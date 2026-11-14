// Package permisos tiene el catálogo de funciones (casillas) que se le pueden dar a un rango, las reglas de jerarquía y alcance,
// y la política de desconocidos y de grupos. El catálogo vive en código: la app solo trae las FUNCIONES; los rangos los crea
// el dueño. Cada módulo agrega sus propias casillas con Registrar.
package permisos

import (
	"encoding/json"
	"sort"
	"sync"
)

// Riesgos de una casilla.
const (
	RiesgoBajo  = "bajo"
	RiesgoMedio = "medio"
	RiesgoAlto  = "alto"
)

// Casilla es una función que un rango puede o no tener.
type Casilla struct {
	ID          string `json:"id"`          // estable, por ejemplo "personas.silenciar"
	Nombre      string `json:"nombre"`      // en español, corto
	Descripcion string `json:"descripcion"` // en palabras simples
	Grupo       string `json:"grupo"`       // sección de la pantalla ("Personas", "Chat"…)
	Riesgo      string `json:"riesgo"`      // bajo | medio | alto
	Privado     bool   `json:"privado"`     // tiene sentido en un chat privado
	Grupos      bool   `json:"grupos"`      // tiene sentido en un grupo
	Confirma    bool   `json:"confirma"`    // borra o expulsa: pide confirmación con código
	Base        bool   `json:"base"`        // la tiene todo el mundo, con o sin rango
	Modulo      string `json:"modulo,omitempty"`
}

var (
	mu     sync.RWMutex
	orden  []string
	porID  = map[string]Casilla{}
	grupos []string // orden de aparición de los grupos de la pantalla
)

// Registrar agrega (o reemplaza) casillas en el catálogo. Los módulos lo llaman al iniciarse. Es seguro desde varias goroutines.
func Registrar(cs ...Casilla) {
	mu.Lock()
	defer mu.Unlock()
	for _, c := range cs {
		if c.ID == "" {
			continue
		}
		if c.Riesgo == "" {
			c.Riesgo = RiesgoBajo
		}
		if !c.Privado && !c.Grupos {
			c.Privado, c.Grupos = true, true
		}
		if _, ok := porID[c.ID]; !ok {
			orden = append(orden, c.ID)
		}
		porID[c.ID] = c
		visto := false
		for _, g := range grupos {
			if g == c.Grupo {
				visto = true
			}
		}
		if !visto {
			grupos = append(grupos, c.Grupo)
		}
	}
}

// Quitar saca casillas del catálogo (por ejemplo un módulo que se desinstala; también lo usan las pruebas).
func Quitar(ids ...string) {
	mu.Lock()
	defer mu.Unlock()
	for _, id := range ids {
		delete(porID, id)
	}
	nuevo := orden[:0]
	for _, id := range orden {
		if _, ok := porID[id]; ok {
			nuevo = append(nuevo, id)
		}
	}
	orden = nuevo
}

// Buscar devuelve una casilla.
func Buscar(id string) (Casilla, bool) {
	mu.RLock()
	defer mu.RUnlock()
	c, ok := porID[id]
	return c, ok
}

// Catalogo devuelve todas las casillas, agrupadas en el orden en que se registraron los grupos.
func Catalogo() []Casilla {
	mu.RLock()
	defer mu.RUnlock()
	pos := map[string]int{}
	for i, g := range grupos {
		pos[g] = i
	}
	out := make([]Casilla, 0, len(orden))
	for _, id := range orden {
		out = append(out, porID[id])
	}
	sort.SliceStable(out, func(i, j int) bool { return pos[out[i].Grupo] < pos[out[j].Grupo] })
	return out
}

// CatalogoJSON es el catálogo para la pantalla.
func CatalogoJSON() string {
	b, _ := json.Marshal(Catalogo())
	return string(b)
}

// Nombres de los grupos del catálogo (en este orden salen en la pantalla).
const (
	GBasico      = "Lo básico"
	GRangos      = "Rangos"
	GPersonas    = "Personas"
	GChat        = "Chat"
	GGruposLibre = "Grupos (sin ser administrador)"
	GGruposAdmin = "Grupos (siendo administrador)"
	GUtilidades  = "Utilidades"
	GMenus       = "Menús"
	GAutomaticos = "Mensajes automáticos"
)

func init() {
	// Los grupos se registran primero para fijar el orden de la pantalla.
	Registrar(
		// ---- Lo básico (todo el mundo) ----
		Casilla{ID: "cmd.ayuda", Nombre: "Ver la ayuda", Descripcion: "Ve la lista de comandos que puede usar.", Grupo: GBasico, Base: true},
		Casilla{ID: "cmd.rango", Nombre: "Ver su rango", Descripcion: "Puede preguntar cuál es su rango.", Grupo: GBasico, Base: true},
		Casilla{ID: "cmd.confirmar", Nombre: "Confirmar con código", Descripcion: "Puede escribir el código para confirmar lo que pidió.", Grupo: GBasico, Base: true},

		// ---- Rangos ----
		Casilla{ID: "rangos.asignar", Nombre: "Dar y quitar rangos", Descripcion: "Puede dar o quitar rangos a personas con un rango más bajo que el suyo.", Grupo: GRangos, Riesgo: RiesgoMedio},
		Casilla{ID: "rangos.ver", Nombre: "Ver quién tiene cada rango", Descripcion: "Ve la lista de rangos y de personas.", Grupo: GRangos},

		// ---- Personas (Bloque B) ----
		Casilla{ID: "personas.silenciar", Nombre: "Ignorar a una persona", Descripcion: "El bot deja de contestarle por un rato o para siempre (comandos !ignorar y !designorar). Su id sigue siendo personas.silenciar para no perder los permisos ya dados.", Grupo: GPersonas},
		Casilla{ID: "personas.bloquear", Nombre: "Bloquear y desbloquear", Descripcion: "El bot ignora por completo a esa persona.", Grupo: GPersonas, Riesgo: RiesgoMedio},
		Casilla{ID: "personas.advertir", Nombre: "Advertir", Descripcion: "Pone o quita advertencias y ve el historial.", Grupo: GPersonas, Riesgo: RiesgoMedio},
		Casilla{ID: "personas.notas", Nombre: "Notas de una persona", Descripcion: "Escribe y lee notas privadas sobre una persona.", Grupo: GPersonas},
		Casilla{ID: "personas.ficha", Nombre: "Ver la ficha", Descripcion: "Ve la ficha y las estadísticas de una persona.", Grupo: GPersonas},
		Casilla{ID: "personas.olvidar", Nombre: "Olvidar o exportar datos", Descripcion: "Borra o exporta lo que se guardó de una persona.", Grupo: GPersonas, Riesgo: RiesgoAlto, Confirma: true},

		// ---- Chat (Bloque B) ----
		Casilla{ID: "chat.pausar", Nombre: "Pausar y reanudar el bot", Descripcion: "Apaga o enciende el bot en este chat.", Grupo: GChat},
		Casilla{ID: "chat.tomar_control", Nombre: "Tomar el control", Descripcion: "Una persona atiende y el bot calla en ese chat.", Grupo: GChat},
		Casilla{ID: "chat.estado", Nombre: "Pendiente y Resuelto", Descripcion: "Marca el chat como pendiente o resuelto.", Grupo: GChat},
		Casilla{ID: "chat.etiquetas", Nombre: "Etiquetas", Descripcion: "Pone o quita etiquetas al chat.", Grupo: GChat},
		Casilla{ID: "chat.reportar", Nombre: "Reportar un mensaje", Descripcion: "Avisa al dueño de un mensaje (solo le llega una notificación).", Grupo: GChat, Base: true},

		// ---- Grupos sin ser admin (Bloque E) ----
		Casilla{ID: "grupos.info", Nombre: "Información del grupo", Descripcion: "Ve datos del grupo.", Grupo: GGruposLibre, Privado: false, Grupos: true, Base: true},
		Casilla{ID: "grupos.admins", Nombre: "Lista de administradores", Descripcion: "Ve quiénes administran el grupo.", Grupo: GGruposLibre, Grupos: true, Base: true},
		Casilla{ID: "grupos.mencionar_todos", Nombre: "Mencionar a todos", Descripcion: "Menciona a todo el grupo de una vez.", Grupo: GGruposLibre, Riesgo: RiesgoAlto, Grupos: true},
		Casilla{ID: "grupos.ranking", Nombre: "Ranking de actividad", Descripcion: "Ve quién escribe más en el grupo.", Grupo: GGruposLibre, Grupos: true},
		Casilla{ID: "grupos.inactivos", Nombre: "Ver inactivos", Descripcion: "Ve quiénes no escriben hace tiempo.", Grupo: GGruposLibre, Grupos: true},

		// ---- Grupos siendo admin (Bloque E) ----
		Casilla{ID: "grupos.cerrar", Nombre: "Cerrar y abrir el grupo", Descripcion: "Solo administradores pueden escribir, o todos.", Grupo: GGruposAdmin, Riesgo: RiesgoMedio, Grupos: true},
		Casilla{ID: "grupos.bloquear_info", Nombre: "Bloquear la información", Descripcion: "Solo administradores cambian nombre, foto y descripción.", Grupo: GGruposAdmin, Riesgo: RiesgoMedio, Grupos: true},
		Casilla{ID: "grupos.expulsar", Nombre: "Expulsar", Descripcion: "Saca a una persona del grupo.", Grupo: GGruposAdmin, Riesgo: RiesgoAlto, Confirma: true, Grupos: true},
		Casilla{ID: "grupos.ascender", Nombre: "Dar y quitar administrador", Descripcion: "Convierte a alguien en administrador o se lo quita.", Grupo: GGruposAdmin, Riesgo: RiesgoAlto, Grupos: true},
		Casilla{ID: "grupos.solicitudes", Nombre: "Aprobar solicitudes de ingreso", Descripcion: "Acepta o rechaza a quien pide entrar.", Grupo: GGruposAdmin, Riesgo: RiesgoMedio, Grupos: true},
		Casilla{ID: "grupos.renombrar", Nombre: "Cambiar nombre y descripción", Descripcion: "Edita el nombre y la descripción del grupo.", Grupo: GGruposAdmin, Riesgo: RiesgoMedio, Grupos: true},
		Casilla{ID: "grupos.enlace", Nombre: "Enlace de invitación", Descripcion: "Pide o reinicia el enlace del grupo.", Grupo: GGruposAdmin, Riesgo: RiesgoMedio, Grupos: true},
		Casilla{ID: "grupos.borrar_mensaje", Nombre: "Borrar mensajes de otros", Descripcion: "Borra un mensaje ajeno en el grupo.", Grupo: GGruposAdmin, Riesgo: RiesgoAlto, Confirma: true, Grupos: true},
		Casilla{ID: "silencio.silenciar", Nombre: "Silenciar en grupos", Descripcion: "El bot borra lo que escribe una persona en los grupos que administra (comandos !silenciar, !desilenciar y !contador).", Grupo: GGruposAdmin, Riesgo: RiesgoAlto, Grupos: true},
		Casilla{ID: "silencio.ver", Nombre: "Ver quién está silenciado en grupos", Descripcion: "Ve la lista de silenciados y sus contadores (!silenciados).", Grupo: GGruposAdmin, Grupos: true},
		Casilla{ID: "grupos.filtros", Nombre: "Filtros del grupo", Descripcion: "Enlaces, palabras prohibidas, repetidos y flood.", Grupo: GGruposAdmin, Riesgo: RiesgoMedio, Grupos: true},

		// ---- Utilidades (Bloque F) ----
		Casilla{ID: "util.notas", Nombre: "Notas", Descripcion: "Guarda y lee notas por comando.", Grupo: GUtilidades},
		Casilla{ID: "util.pendientes", Nombre: "Pendientes", Descripcion: "Lleva una lista de pendientes por comando.", Grupo: GUtilidades},
		Casilla{ID: "util.sticker", Nombre: "Convertir imagen en sticker", Descripcion: "Convierte una imagen en sticker.", Grupo: GUtilidades},
		Casilla{ID: "comandos.propios", Nombre: "Comandos propios", Descripcion: "Usa los comandos que creó el dueño (!horario, !precios…).", Grupo: GUtilidades, Base: true},

		// ---- Menús (Bloque C) ----
		Casilla{ID: "menus.usar", Nombre: "Usar los menús", Descripcion: "El bot le muestra y le atiende los menús.", Grupo: GMenus, Base: true},
		Casilla{ID: "menus.persona", Nombre: "Hablar con una persona", Descripcion: "Puede pedir que lo atienda una persona.", Grupo: GMenus, Base: true},
		Casilla{ID: "menus.formularios", Nombre: "Llenar formularios", Descripcion: "Puede llenar los formularios de los menús.", Grupo: GMenus},

		// ---- Mensajes automáticos (Bloque D) ----
		Casilla{ID: "auto.recibir", Nombre: "Recibir mensajes automáticos", Descripcion: "Le llegan bienvenida, ausencia y cierre.", Grupo: GAutomaticos, Base: true},
		Casilla{ID: "auto.prioridad", Nombre: "Atención preferente", Descripcion: "Sus mensajes no se mezclan con la bienvenida automática.", Grupo: GAutomaticos},
	)
}
