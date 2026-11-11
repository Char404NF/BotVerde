package modulos

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Tipos de campo de la configuración genérica. La app dibuja la ventana de un módulo a partir de estos campos cuando el módulo
// no trae una pantalla propia.
const (
	CampoTexto       = "texto"
	CampoNumero      = "numero"
	CampoInterruptor = "interruptor"
	CampoLista       = "lista"
	CampoHora        = "hora" // "HH:MM"
	// CampoSecreto es una llave o contraseña: la app la guarda en el Android Keystore y se la pasa a Go solo en memoria. NUNCA entra a la
	// configuración ni a la base: guardar una configuración que la traiga es un error.
	CampoSecreto = "secreto"
)

// Opcion es una entrada de un campo de tipo lista.
type Opcion struct {
	Valor  string `json:"valor"`
	Titulo string `json:"titulo"`
}

// Campo describe un ajuste simple del módulo.
type Campo struct {
	Clave       string   `json:"clave"`
	Titulo      string   `json:"titulo"`
	Ayuda       string   `json:"ayuda,omitempty"`
	Tipo        string   `json:"tipo"`
	Defecto     any      `json:"defecto"`
	Min         float64  `json:"min,omitempty"`    // número: mínimo
	Max         float64  `json:"max,omitempty"`    // número: máximo (0 = sin tope)
	Largo       int      `json:"largo,omitempty"`  // texto: máximo de letras (0 = 500)
	Varias      bool     `json:"varias,omitempty"` // texto: varias líneas
	Opciones    []Opcion `json:"opciones,omitempty"`
	Obligatorio bool     `json:"obligatorio,omitempty"` // texto: no puede ir vacío
}

var reHora = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// valorPorDefecto da el valor inicial de un campo según su tipo.
func (c Campo) valorPorDefecto() any {
	if c.Defecto != nil {
		return c.Defecto
	}
	switch c.Tipo {
	case CampoNumero:
		return c.Min
	case CampoInterruptor:
		return false
	case CampoLista:
		if len(c.Opciones) > 0 {
			return c.Opciones[0].Valor
		}
		return ""
	case CampoHora:
		return "00:00"
	}
	return ""
}

func (c Campo) secreto() bool { return c.Tipo == CampoSecreto }

// validar revisa un valor y lo deja normalizado (número como float64, texto recortado…).
func (c Campo) validar(v any) (any, error) {
	nombre := c.Titulo
	if nombre == "" {
		nombre = c.Clave
	}
	switch c.Tipo {
	case CampoSecreto:
		return nil, fmt.Errorf("«%s» es una llave: se guarda aparte, en el Keystore del teléfono, no en la configuración", nombre)
	case CampoTexto:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("«%s» debe ser un texto", nombre)
		}
		s = strings.TrimSpace(s)
		max := c.Largo
		if max <= 0 {
			max = 500
		}
		if len([]rune(s)) > max {
			return nil, fmt.Errorf("«%s» es muy largo (máximo %d letras)", nombre, max)
		}
		if c.Obligatorio && s == "" {
			return nil, fmt.Errorf("«%s» no puede ir vacío", nombre)
		}
		return s, nil
	case CampoNumero:
		f, ok := v.(float64)
		if !ok {
			return nil, fmt.Errorf("«%s» debe ser un número", nombre)
		}
		if f != float64(int64(f)) && c.Min == float64(int64(c.Min)) && c.Max == float64(int64(c.Max)) {
			// los topes enteros piden enteros
			return nil, fmt.Errorf("«%s» debe ser un número entero", nombre)
		}
		if f < c.Min || (c.Max != 0 && f > c.Max) {
			if c.Max != 0 {
				return nil, fmt.Errorf("«%s» debe estar entre %v y %v", nombre, c.Min, c.Max)
			}
			return nil, fmt.Errorf("«%s» debe ser al menos %v", nombre, c.Min)
		}
		return f, nil
	case CampoInterruptor:
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("«%s» debe ser sí o no", nombre)
		}
		return b, nil
	case CampoLista:
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("«%s» debe ser una de las opciones", nombre)
		}
		for _, o := range c.Opciones {
			if o.Valor == s {
				return s, nil
			}
		}
		return nil, fmt.Errorf("«%s»: esa opción no existe", nombre)
	case CampoHora:
		s, ok := v.(string)
		if !ok || !reHora.MatchString(s) {
			return nil, fmt.Errorf("«%s» debe ser una hora como 08:30", nombre)
		}
		return s, nil
	}
	return nil, fmt.Errorf("el campo «%s» tiene un tipo desconocido", c.Clave)
}

// ValidarEsquema revisa que el esquema de un módulo esté bien armado (claves únicas, tipos conocidos, listas con opciones).
func ValidarEsquema(campos []Campo) error {
	vistos := map[string]bool{}
	for _, c := range campos {
		if c.Clave == "" || vistos[c.Clave] {
			return fmt.Errorf("campo sin clave o repetido: %q", c.Clave)
		}
		vistos[c.Clave] = true
		switch c.Tipo {
		case CampoTexto, CampoNumero, CampoInterruptor, CampoHora, CampoSecreto:
		case CampoLista:
			if len(c.Opciones) == 0 {
				return fmt.Errorf("la lista %q no tiene opciones", c.Clave)
			}
		default:
			return fmt.Errorf("tipo desconocido %q en %q", c.Tipo, c.Clave)
		}
	}
	return nil
}

// configPorDefecto junta el JSON por defecto del módulo con los valores por defecto del esquema.
func configPorDefecto(m Modulo) map[string]any {
	out := map[string]any{}
	if t := strings.TrimSpace(m.ConfigDefecto()); t != "" {
		_ = json.Unmarshal([]byte(t), &out)
	}
	for _, c := range m.Esquema() {
		if c.secreto() {
			continue
		}
		if _, ok := out[c.Clave]; !ok {
			out[c.Clave] = c.valorPorDefecto()
		}
	}
	return out
}

// mezclarConfig completa lo guardado con los valores por defecto (campos nuevos de una versión posterior).
func mezclarConfig(m Modulo, guardado string) map[string]any {
	out := configPorDefecto(m)
	if strings.TrimSpace(guardado) == "" {
		return out
	}
	var g map[string]any
	if json.Unmarshal([]byte(guardado), &g) == nil {
		for k, v := range g {
			out[k] = v
		}
	}
	return out
}

// validarConfig revisa una configuración nueva (JSON de un objeto) y devuelve la completa. Con esquema, cada campo se valida por
// tipo y rango y no se aceptan claves que no estén en él.
func validarConfig(m Modulo, texto string) (map[string]any, error) {
	var nuevo map[string]any
	if err := json.Unmarshal([]byte(texto), &nuevo); err != nil || nuevo == nil {
		return nil, errors.New("la configuración debe ser un objeto JSON")
	}
	if len(texto) > 64<<10 {
		return nil, errors.New("la configuración es demasiado grande")
	}
	completa := configPorDefecto(m)
	esq := m.Esquema()
	if len(esq) > 0 {
		if err := ValidarEsquema(esq); err != nil {
			return nil, err
		}
		campos := map[string]Campo{}
		for _, c := range esq {
			campos[c.Clave] = c
		}
		for k := range nuevo {
			if _, ok := campos[k]; !ok {
				return nil, fmt.Errorf("ajuste desconocido: %q", k)
			}
		}
		for k, v := range nuevo {
			n, err := campos[k].validar(v)
			if err != nil {
				return nil, err
			}
			completa[k] = n
		}
	} else {
		for k, v := range nuevo {
			completa[k] = v
		}
	}
	if v, ok := m.(Validador); ok {
		if err := v.ValidarConfig(completa); err != nil {
			return nil, err
		}
	}
	return completa, nil
}

// Config es la configuración de un módulo ya leída, con ayudas para leerla sin errores de tipo.
type Config map[string]any

// Texto devuelve un campo de texto o lista.
func (c Config) Texto(k string) string { s, _ := c[k].(string); return s }

// Num devuelve un campo numérico (0 si no es número).
func (c Config) Num(k string) float64 { f, _ := c[k].(float64); return f }

// Int devuelve un campo numérico como entero.
func (c Config) Int(k string) int { return int(c.Num(k)) }

// Bool devuelve un interruptor.
func (c Config) Bool(k string) bool { b, _ := c[k].(bool); return b }
