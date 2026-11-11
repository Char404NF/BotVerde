package flujo

import (
	"fmt"
	"regexp"
	"strings"
)

// Tipos de la etapa C (menús ampliados): ajustes por menú, adjuntos, ubicación, contacto y campos de formulario.

// AjustesMenu son los ajustes de UN menú. Lo que está vacío o en cero usa el valor general (Ajustes → Avanzado).
type AjustesMenu struct {
	// Escalera de errores: cuántos errores seguidos detienen el menú y los textos de cada nivel.
	ErrMax  int    `json:"errMax,omitempty"`
	TxtN1   string `json:"txtN1,omitempty"`
	TxtN2   string `json:"txtN2,omitempty"`
	TxtN3   string `json:"txtN3,omitempty"`
	TxtNoTx string `json:"txtNoTexto,omitempty"` // imagen, audio, sticker o archivo cuando se espera texto
	// Caducidad en silencio (minutos) en privado y en grupos, y qué hacer cuando la persona vuelve a escribir.
	ExpiraMin      int    `json:"expiraMin,omitempty"`
	ExpiraGrupoMin int    `json:"expiraGrupoMin,omitempty"`
	Volver         string `json:"volver,omitempty"` // "retomar" | "reiniciar"
	// Hablar con una persona: textos según esté en horario o no.
	HumanoDentro string `json:"humanoDentro,omitempty"`
	HumanoFuera  string `json:"humanoFuera,omitempty"`
	// Grupos
	MaxSesionesGrupo int    `json:"maxSesionesGrupo,omitempty"`
	Cualquiera       string `json:"cualquiera,omitempty"` // "si" | "no" | "" (general)
}

// AdjuntoMenu es un archivo que se manda con una respuesta. El archivo vive cifrado en el teléfono (almacen.GuardarAdjuntoMenu).
type AdjuntoMenu struct {
	ID     string `json:"id"`
	Tipo   string `json:"tipo"` // imagen, video, audio, documento
	Mime   string `json:"mime,omitempty"`
	Nombre string `json:"nombre,omitempty"`
}

// Ubicacion es un punto del mapa que se manda con una respuesta.
type Ubicacion struct {
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Nombre    string  `json:"nombre,omitempty"`
	Direccion string  `json:"direccion,omitempty"`
}

// Contacto es una tarjeta de contacto que se manda con una respuesta.
type Contacto struct {
	Nombre   string `json:"nombre"`
	Telefono string `json:"telefono"`
}

// Tipos de dato de un campo de formulario.
const (
	CampoTexto    = "texto"
	CampoNumero   = "numero"
	CampoCorreo   = "correo"
	CampoTelefono = "telefono"
	CampoFecha    = "fecha"
	CampoOpcion   = "opcion"
)

// Campo es una pregunta de un formulario.
type Campo struct {
	ID       string   `json:"id"`                 // nombre del dato: se puede usar como {id} en los textos
	Etiqueta string   `json:"etiqueta,omitempty"` // cómo se llama en el resumen ("Nombre", "Correo")
	Pregunta string   `json:"pregunta"`
	Tipo     string   `json:"tipo"`
	Opciones []string `json:"opciones,omitempty"` // tipo opcion
	Opcional bool     `json:"opcional,omitempty"` // se puede saltar escribiendo "omitir"
	Error    string   `json:"error,omitempty"`    // mensaje cuando la respuesta no sirve (vacío = uno por tipo)
	Min      *float64 `json:"min,omitempty"`      // numero: mínimo
	Max      *float64 `json:"max,omitempty"`      // numero: máximo
}

// TiposDeCampo son los que se pueden elegir.
var TiposDeCampo = []string{CampoTexto, CampoNumero, CampoCorreo, CampoTelefono, CampoFecha, CampoOpcion}

func tipoCampoValido(t string) bool {
	for _, x := range TiposDeCampo {
		if x == t {
			return true
		}
	}
	return false
}

func (c Campo) etiqueta() string {
	if strings.TrimSpace(c.Etiqueta) != "" {
		return strings.TrimSpace(c.Etiqueta)
	}
	return c.ID
}

var reIDVar = regexp.MustCompile(`[^a-z0-9_]+`)

// idVar deja un nombre de dato en minúsculas, sin acentos ni símbolos (sirve para {variable}).
func idVar(s string) string {
	s = strings.ReplaceAll(norm(s), " ", "_")
	return reIDVar.ReplaceAllString(s, "")
}

func (a *AjustesMenu) validar() []string {
	var out []string
	if a == nil {
		return out
	}
	if a.ErrMax < 0 || a.ErrMax > 10 {
		out = append(out, "Los errores seguidos permitidos van de 1 a 10.")
	}
	if a.ExpiraMin < 0 || a.ExpiraMin > 7*24*60 || a.ExpiraGrupoMin < 0 || a.ExpiraGrupoMin > 7*24*60 {
		out = append(out, "La caducidad debe ser de 1 minuto a 7 días.")
	}
	switch a.Volver {
	case "", "retomar", "reiniciar":
	default:
		out = append(out, "Al volver a escribir solo se puede retomar o reiniciar.")
	}
	switch a.Cualquiera {
	case "", "si", "no":
	default:
		out = append(out, "El ajuste \"cualquiera puede contestar\" no es válido.")
	}
	if a.MaxSesionesGrupo < 0 || a.MaxSesionesGrupo > 100 {
		out = append(out, "Las personas a la vez en un grupo van de 1 a 100.")
	}
	return out
}

// normalizarExtras asigna ids a las opciones y a los campos, y limpia los nombres de dato. Lo llama Def.Normalizar.
func (d *Def) normalizarExtras() {
	usados := map[string]bool{}
	for _, p := range d.Pasos {
		for _, o := range p.Opciones {
			if o.ID != "" {
				usados[o.ID] = true
			}
		}
	}
	n := 0
	for i := range d.Pasos {
		p := &d.Pasos[i]
		for j := range p.Opciones {
			if p.Opciones[j].ID == "" {
				for {
					n++
					c := fmt.Sprintf("o%d", n)
					if !usados[c] {
						p.Opciones[j].ID = c
						usados[c] = true
						break
					}
				}
			}
		}
		vistos := map[string]bool{}
		for j := range p.Campos {
			c := &p.Campos[j]
			c.ID = idVar(c.ID)
			if c.ID == "" {
				c.ID = idVar(c.Etiqueta)
			}
			if c.ID == "" {
				c.ID = fmt.Sprintf("campo%d", j+1)
			}
			base := c.ID
			for k := 2; vistos[c.ID]; k++ {
				c.ID = fmt.Sprintf("%s%d", base, k)
			}
			vistos[c.ID] = true
			if c.Tipo == "" {
				c.Tipo = CampoTexto
			}
			c.Etiqueta = strings.TrimSpace(c.Etiqueta)
		}
	}
}

// validarExtras revisa los formularios, los adjuntos, la ubicación, el contacto y los ajustes. Lo llama Def.Validar.
func (d *Def) validarExtras(add func(paso, texto string, grave bool)) {
	for _, t := range d.Ajustes.validar() {
		add("", t, true)
	}
	for _, p := range d.Pasos {
		for i, o := range p.Opciones {
			switch o.Ambito {
			case "", "todos", "privado", "grupo":
			default:
				add(p.ID, fmt.Sprintf("La opción %d tiene un ámbito que no existe.", i+1), true)
			}
		}
		if len(p.Adjuntos) > 3 {
			add(p.ID, "Una respuesta puede llevar hasta 3 archivos.", true)
		}
		for i, a := range p.Adjuntos {
			switch a.Tipo {
			case "imagen", "video", "audio", "documento":
			default:
				add(p.ID, fmt.Sprintf("El archivo %d no es de un tipo que se pueda mandar.", i+1), true)
			}
			if a.ID == "" {
				add(p.ID, fmt.Sprintf("Al archivo %d le falta el archivo guardado.", i+1), true)
			}
		}
		if u := p.Ubicacion; u != nil && (u.Lat < -90 || u.Lat > 90 || u.Lon < -180 || u.Lon > 180 || (u.Lat == 0 && u.Lon == 0)) {
			add(p.ID, "La ubicación no es válida.", true)
		}
		if c := p.Contacto; c != nil {
			if strings.TrimSpace(c.Nombre) == "" || len(digitos(c.Telefono)) < 7 {
				add(p.ID, "El contacto necesita un nombre y un teléfono.", true)
			}
		}
		if p.Tipo != PasoForm {
			continue
		}
		if len(p.Campos) == 0 {
			add(p.ID, "El formulario no tiene preguntas.", true)
		}
		if len(p.Campos) > 15 {
			add(p.ID, "Un formulario puede tener hasta 15 preguntas.", true)
		}
		if p.MaxReintentos < 0 || p.MaxReintentos > 10 {
			add(p.ID, "Los intentos por pregunta van de 1 a 10.", true)
		}
		for i, c := range p.Campos {
			if strings.TrimSpace(c.Pregunta) == "" {
				add(p.ID, fmt.Sprintf("La pregunta %d del formulario está vacía.", i+1), true)
			}
			if !tipoCampoValido(c.Tipo) {
				add(p.ID, fmt.Sprintf("La pregunta %d tiene un tipo de dato que no existe.", i+1), true)
			}
			if c.Tipo == CampoOpcion {
				n := 0
				for _, o := range c.Opciones {
					if strings.TrimSpace(o) != "" {
						n++
					}
				}
				if n < 2 {
					add(p.ID, fmt.Sprintf("La pregunta %d necesita al menos 2 opciones.", i+1), true)
				}
			}
			if c.Min != nil && c.Max != nil && *c.Min > *c.Max {
				add(p.ID, fmt.Sprintf("En la pregunta %d el mínimo es mayor que el máximo.", i+1), true)
			}
		}
	}
}
