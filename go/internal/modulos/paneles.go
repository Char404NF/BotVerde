package modulos

import (
	"errors"
	"fmt"
	"strings"
)

// Paneles: listas y formularios simples que la ventana genérica dibuja debajo de los ajustes de un módulo (destinatarios, mensajes
// programados, conectores…). Así un módulo con listas NO necesita una pantalla de Kotlin propia: Go dice qué campos, qué filas y qué
// botones hay, y la app los dibuja. Todo lo que un botón hace pasa por Accion, que valida y responde en español.

// CampoAdjuntos es un campo de panel (no de ajustes) que la app dibuja con el selector de archivos; su valor es un arreglo JSON de
// adjuntos subidos ([{id,tipo,mime,nombre,bytes}], hasta 3).
const CampoAdjuntos = "adjuntos"

// Boton es un botón de un panel. Accion es el nombre que llega a Accion; Confirma, si no está vacío, se muestra como pregunta antes.
type Boton struct {
	Accion   string `json:"accion"`
	Titulo   string `json:"titulo"`
	Confirma string `json:"confirma,omitempty"`
}

// Panel describe una lista con su formulario.
type Panel struct {
	ID     string  `json:"id"`
	Titulo string  `json:"titulo"`
	Ayuda  string  `json:"ayuda,omitempty"`
	Campos []Campo `json:"campos,omitempty"` // formulario de alta: el botón principal llama a la acción "<id>.agregar"
	Alta   string  `json:"alta,omitempty"`   // texto del botón principal ("Agregar")
	// Extras son botones del formulario que usan sus mismos valores (por ejemplo "Vista previa" o "Probar").
	Extras []Boton `json:"extras,omitempty"`
	// PorFila son los botones de cada fila (reciben {"id": <id de la fila>}).
	PorFila []Boton `json:"porFila,omitempty"`
	// Generales son botones del panel que no usan el formulario (por ejemplo "Importar a quienes ya me escribieron").
	Generales []Boton `json:"generales,omitempty"`
}

// Fila es un renglón de un panel.
type Fila struct {
	ID      string `json:"id"`
	Titulo  string `json:"titulo"`
	Detalle string `json:"detalle,omitempty"`
}

// ConPaneles lo implementan los módulos con listas.
type ConPaneles interface {
	Paneles(g *Gestor) []Panel
	Filas(g *Gestor, panel string) []Fila
	// Accion corre un botón. args son los valores del formulario (o {"id": ...} de una fila). Devuelve un mensaje corto en español.
	Accion(g *Gestor, accion string, args Args) (string, error)
}

// Args son los valores que manda la app con un botón.
type Args map[string]any

// Texto devuelve un valor de texto recortado.
func (a Args) Texto(k string) string {
	switch v := a[k].(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return fmt.Sprint(int64(v))
	}
	return ""
}

// Num devuelve un número (0 si no lo es).
func (a Args) Num(k string) float64 {
	switch v := a[k].(type) {
	case float64:
		return v
	case string:
		var f float64
		if _, err := fmt.Sscanf(strings.TrimSpace(v), "%f", &f); err == nil {
			return f
		}
	}
	return 0
}

// Bool devuelve un interruptor.
func (a Args) Bool(k string) bool {
	switch v := a[k].(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	}
	return false
}

// Lista devuelve el valor como arreglo de objetos (los adjuntos).
func (a Args) Lista(k string) []map[string]any {
	var out []map[string]any
	switch v := a[k].(type) {
	case []any:
		for _, x := range v {
			if m, ok := x.(map[string]any); ok {
				out = append(out, m)
			}
		}
	}
	return out
}

// PanelesDe devuelve los paneles de un módulo (vacío si no tiene).
func (g *Gestor) PanelesDe(id string) []Panel {
	m, ok := Obtener(id)
	if !ok {
		return nil
	}
	if p, ok := m.(ConPaneles); ok {
		var out []Panel
		func() { defer func() { _ = recover() }(); out = p.Paneles(g) }()
		return out
	}
	return nil
}

// FilasPanel devuelve los renglones de un panel.
func (g *Gestor) FilasPanel(id, panel string) ([]Fila, error) {
	m, ok := Obtener(id)
	if !ok {
		return nil, ErrNoExiste
	}
	p, ok := m.(ConPaneles)
	if !ok {
		return nil, errors.New("este módulo no tiene listas")
	}
	var out []Fila
	func() { defer func() { _ = recover() }(); out = p.Filas(g, panel) }()
	if out == nil {
		out = []Fila{}
	}
	return out, nil
}

// AccionModulo corre un botón de un panel. Quedan en el registro de acciones del dueño (sin textos).
func (g *Gestor) AccionModulo(id, accion string, args Args) (msg string, err error) {
	m, ok := Obtener(id)
	if !ok {
		return "", ErrNoExiste
	}
	p, ok := m.(ConPaneles)
	if !ok {
		return "", errors.New("este módulo no tiene botones")
	}
	defer func() {
		if r := recover(); r != nil {
			msg, err = "", fmt.Errorf("no se pudo hacer: %v", r)
		}
	}()
	if args == nil {
		args = Args{}
	}
	msg, err = p.Accion(g, accion, args)
	g.cambio("modulos")
	return msg, err
}
