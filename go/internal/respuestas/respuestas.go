// Package respuestas reúne los textos que el bot contesta por su cuenta cuando alguien usa un comando («Listo: el grupo quedó cerrado»,
// «No tienes permiso…»). Cada uno tiene un texto de fábrica y el dueño puede cambiarlo desde la app (o volver al de fábrica).
// Un texto puede traer {variables} que se llenan al contestar; la pantalla las muestra para que no se borren por accidente.
package respuestas

import (
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"botchat/internal/almacen"
)

// Def declara una respuesta editable.
type Def struct {
	ID          string   `json:"id"`
	Grupo       string   `json:"grupo"`       // sección de la pantalla
	Descripcion string   `json:"descripcion"` // cuándo sale, en palabras simples
	Defecto     string   `json:"defecto"`     // texto de fábrica (con {variables})
	Vars        []string `json:"vars"`        // variables que se pueden usar: {nombre}…
}

// Fila es una respuesta tal como la ve la app: la de fábrica y la propia (vacía = la de fábrica).
type Fila struct {
	Def
	Propio string `json:"propio"`
}

var (
	mu     sync.RWMutex
	orden  []string
	porID  = map[string]Def{}
	grupos []string
)

// Registrar agrega (o reemplaza) respuestas al catálogo.
func Registrar(ds ...Def) {
	mu.Lock()
	defer mu.Unlock()
	for _, d := range ds {
		if d.ID == "" {
			continue
		}
		if _, ok := porID[d.ID]; !ok {
			orden = append(orden, d.ID)
		}
		porID[d.ID] = d
		visto := false
		for _, g := range grupos {
			visto = visto || g == d.Grupo
		}
		if !visto {
			grupos = append(grupos, d.Grupo)
		}
	}
}

// Catalogo devuelve todas las respuestas en el orden en que se registraron.
func Catalogo() []Def {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Def, 0, len(orden))
	for _, id := range orden {
		out = append(out, porID[id])
	}
	return out
}

// Buscar devuelve la definición de una respuesta.
func Buscar(id string) (Def, bool) {
	mu.RLock()
	defer mu.RUnlock()
	d, ok := porID[id]
	return d, ok
}

// Texto da lo que se debe contestar: el texto propio del dueño si lo cambió, o el de fábrica; con las {variables} llenas.
// vars va en pares: "nombre", "Ana", "n", "3".
func Texto(alm *almacen.Almacen, botID int64, id string, vars ...string) string {
	d, ok := Buscar(id)
	t := d.Defecto
	if !ok {
		return ""
	}
	if alm != nil {
		if p := strings.TrimSpace(alm.Respuesta(botID, id)); p != "" {
			t = p
		}
	}
	return Llenar(t, vars...)
}

// Llenar cambia {nombre} por su valor. Una variable que no existe se deja tal cual.
func Llenar(t string, vars ...string) string {
	if len(vars) < 2 {
		return t
	}
	r := make([]string, 0, len(vars))
	for i := 0; i+1 < len(vars); i += 2 {
		r = append(r, "{"+vars[i]+"}", vars[i+1])
	}
	return strings.NewReplacer(r...).Replace(t)
}

// Reemplazo es «cambiar esta frase por esta otra» en cualquier respuesta (incluso las que no están en el catálogo).
type Reemplazo struct {
	De string `json:"de"`
	A  string `json:"a"`
}

const claveReemplazos = "_reemplazos"

// Reemplazos devuelve los cambios de frases que hizo el dueño.
func Reemplazos(alm *almacen.Almacen, botID int64) []Reemplazo {
	var out []Reemplazo
	if alm == nil {
		return out
	}
	if raw := alm.Respuesta(botID, claveReemplazos); raw != "" {
		_ = json.Unmarshal([]byte(raw), &out)
	}
	return out
}

// FijarReemplazos guarda los cambios de frases (los vacíos o repetidos se descartan; como mucho 40).
func FijarReemplazos(alm *almacen.Almacen, botID int64, l []Reemplazo) error {
	var limpio []Reemplazo
	vistos := map[string]bool{}
	for _, r := range l {
		r.De = strings.TrimSpace(r.De)
		if r.De == "" || vistos[r.De] {
			continue
		}
		if len([]rune(r.De)) > 200 || len([]rune(r.A)) > 600 {
			return errLargo
		}
		vistos[r.De] = true
		limpio = append(limpio, r)
		if len(limpio) == 40 {
			break
		}
	}
	if len(limpio) == 0 {
		return alm.FijarRespuesta(botID, claveReemplazos, "")
	}
	b, _ := json.Marshal(limpio)
	return alm.FijarRespuesta(botID, claveReemplazos, string(b))
}

// Aplicar cambia en un texto las frases que el dueño reemplazó. Rápido cuando no hay ninguna.
func Aplicar(alm *almacen.Almacen, botID int64, texto string) string {
	for _, r := range Reemplazos(alm, botID) {
		texto = strings.ReplaceAll(texto, r.De, r.A)
	}
	return texto
}

// Lista devuelve el catálogo con lo que cada respuesta tiene guardado.
func Lista(alm *almacen.Almacen, botID int64) []Fila {
	ds := Catalogo()
	out := make([]Fila, 0, len(ds))
	for _, d := range ds {
		f := Fila{Def: d}
		if alm != nil {
			f.Propio = alm.Respuesta(botID, d.ID)
		}
		if f.Vars == nil {
			f.Vars = []string{}
		}
		out = append(out, f)
	}
	return out
}

// Validar dice si un texto propio sirve: no muy largo y sin variables inventadas (una {variable} que la respuesta no conoce saldría tal cual).
func Validar(id, texto string) error {
	d, ok := Buscar(id)
	if !ok {
		return errNoExiste
	}
	t := strings.TrimSpace(texto)
	if len([]rune(t)) > 600 {
		return errLargo
	}
	conocidas := map[string]bool{}
	for _, v := range d.Vars {
		conocidas[v] = true
	}
	for _, v := range variablesDe(t) {
		if !conocidas[v] {
			return &errVar{v, d.Vars}
		}
	}
	return nil
}

func variablesDe(t string) []string {
	var out []string
	for {
		i := strings.IndexByte(t, '{')
		if i < 0 {
			return out
		}
		j := strings.IndexByte(t[i:], '}')
		if j < 0 {
			return out
		}
		if v := t[i+1 : i+j]; v != "" && !strings.ContainsAny(v, " \n{") {
			out = append(out, v)
		}
		t = t[i+j+1:]
	}
}

// Grupos devuelve las secciones en orden.
func Grupos() []string {
	mu.RLock()
	defer mu.RUnlock()
	return append([]string(nil), grupos...)
}

var _ = sort.Strings
