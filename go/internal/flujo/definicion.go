// Package flujo es el constructor de menús y respuestas: define las automatizaciones, las valida y las ejecuta paso a paso.
// SOLO RESPONDE: un menú nunca escribe primero; arranca cuando alguien escribe.
package flujo

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Tipos de paso.
const (
	PasoMenu      = "menu"      // título y opciones numeradas
	PasoRespuesta = "respuesta" // solo texto
	PasoPedir     = "pedir"     // hace una pregunta y guarda la respuesta libre en una variable cifrada
	PasoHumano    = "humano"    // pasa la conversación a una persona
	PasoFin       = "fin"
	PasoForm      = "formulario" // formulario paso a paso con validación, resumen y corrección
)

// Opcion de un menú: lleva a otro paso.
type Opcion struct {
	// ID es la clave estable de la opción (o1, o2…). El rango "ve" opciones sueltas con "idMenu:ID" (etapa A).
	ID      string `json:"id,omitempty"`
	Texto   string `json:"texto"`
	Destino string `json:"destino"`
	// Ambito: "" o "todos" (en privado y en grupos), "privado" (en un grupo avisa "escríbeme al privado") o "grupo" (solo en grupos).
	Ambito string `json:"ambito,omitempty"`
	// Horario: la opción solo se ofrece dentro de este horario (vacío = siempre).
	Horario *Horario `json:"horario,omitempty"`
	// Boton: si el menú sale con botones, esta opción es un botón especial (enlace, copiar o llamar). Con menú de texto o encuesta se
	// ignora y la opción funciona como siempre.
	Boton *BotonOpcion `json:"boton,omitempty"`
}

// BotonOpcion es un botón especial de una opción de menú.
type BotonOpcion struct {
	Tipo  string `json:"tipo"`  // enlace | copiar | llamar
	Valor string `json:"valor"` // la dirección, el texto a copiar o el número
}

// Paso es una pieza de la conversación.
type Paso struct {
	ID        string   `json:"id"`
	Tipo      string   `json:"tipo"`
	Titulo    string   `json:"titulo,omitempty"`    // menú
	Opciones  []Opcion `json:"opciones,omitempty"`  // menú
	Texto     string   `json:"texto,omitempty"`     // respuesta, humano, fin
	Pregunta  string   `json:"pregunta,omitempty"`  // pedir
	Variable  string   `json:"variable,omitempty"`  // pedir: dato1, dato2…
	Siguiente string   `json:"siguiente,omitempty"` // respuesta y pedir: a dónde sigue ("" = termina)
	Notificar bool     `json:"notificar,omitempty"` // respuesta: avisa con una notificación local
	Aviso     string   `json:"aviso,omitempty"`     // texto del aviso (admite variables)
	// Formato: cómo sale ESTE menú: "" (el general), "texto", "encuesta" o "botones" (solo en chats privados).
	Formato string `json:"formato,omitempty"`
	// Avatar: momento del avatar («menu»…) si este paso puede salir con la imagen del avatar del bot (según sus ajustes en la app).
	Avatar string `json:"avatar,omitempty"`

	// Respuesta: archivos, ubicación y tarjeta de contacto que se mandan junto con el texto (bloque C).
	Adjuntos  []AdjuntoMenu `json:"adjuntos,omitempty"`
	Ubicacion *Ubicacion    `json:"ubicacion,omitempty"`
	Contacto  *Contacto     `json:"contacto,omitempty"`
	// Humano: texto cuando es fuera de horario (vacío = el general).
	TextoFuera string `json:"textoFuera,omitempty"`
	// Formulario: los campos que se piden uno por uno. Titulo es la introducción, Texto el mensaje de agradecimiento al confirmar.
	Campos        []Campo `json:"campos,omitempty"`
	MaxReintentos int     `json:"maxReintentos,omitempty"` // intentos por campo antes de cancelar (3 por defecto)
	SinResumen    bool    `json:"sinResumen,omitempty"`    // no pedir "¿Es correcto?" al final
}

// Disparadores dicen cuándo se activa. Basta que se cumpla uno.
type Disparadores struct {
	PrimerMensaje   bool     `json:"primerMensaje,omitempty"`
	DiasSinEscribir int      `json:"diasSinEscribir,omitempty"`
	Palabras        []string `json:"palabras,omitempty"`
	Comando         string   `json:"comando,omitempty"`
	FueraDeHorario  bool     `json:"fueraDeHorario,omitempty"`
	Mencion         bool     `json:"mencion,omitempty"`
	Cualquiera      bool     `json:"cualquiera,omitempty"`
	EnfriamientoSeg int      `json:"enfriamientoSeg,omitempty"`
}

// Alcance dice dónde funciona.
type Alcance struct {
	Privados bool     `json:"privados"`
	Chats    []string `json:"chats,omitempty"`  // chats privados concretos (si Privados es falso)
	Grupos   []string `json:"grupos,omitempty"` // solo en los grupos elegidos
	// TodosGrupos: vale en cualquier grupo (donde el bot ya responde por mención, cita o comando). Lo usan los comandos propios.
	TodosGrupos bool `json:"todosGrupos,omitempty"`
}

// Horario son los días y las horas en que atiende. Días: 1 = lunes … 7 = domingo. Vacío = siempre.
type Horario struct {
	Dias  []int  `json:"dias,omitempty"`
	Desde string `json:"desde,omitempty"` // HH:MM
	Hasta string `json:"hasta,omitempty"`
}

// Def es una automatización completa.
type Def struct {
	V            int          `json:"v"`
	Nombre       string       `json:"nombre"`
	Inicio       string       `json:"inicio"`
	Pasos        []Paso       `json:"pasos"`
	Disparadores Disparadores `json:"disparadores"`
	Alcance      Alcance      `json:"alcance"`
	Horario      Horario      `json:"horario"`
	// Ajustes de este menú que reemplazan a los generales (escalera de errores, caducidad, grupos…).
	Ajustes *AjustesMenu `json:"ajustes,omitempty"`
	// Propio marca un comando propio creado en Ajustes → Comandos propios (!horario, !precios…). Sus pasos y disparadores se generan
	// a partir de estos datos (ConstruirPropio) y no salen en la lista normal de automatizaciones.
	Propio *DatosPropio `json:"propio,omitempty"`
}

// Parse lee una definición de su JSON.
func Parse(b []byte) (*Def, error) {
	var d Def
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("el archivo no es una automatización válida")
	}
	if d.V == 0 {
		d.V = 1
	}
	return &d, nil
}

// JSON devuelve la definición lista para guardar.
func (d *Def) JSON() []byte {
	b, _ := json.Marshal(d)
	return b
}

// Paso busca un paso por su id.
func (d *Def) Paso(id string) *Paso {
	for i := range d.Pasos {
		if d.Pasos[i].ID == id {
			return &d.Pasos[i]
		}
	}
	return nil
}

// Normalizar da id a los pasos que no lo tengan y nombre a las variables de "pedir" (dato1, dato2…), sin repetir.
func (d *Def) Normalizar() {
	usados := map[string]bool{}
	for _, p := range d.Pasos {
		if p.ID != "" {
			usados[p.ID] = true
		}
	}
	n := 0
	for i := range d.Pasos {
		if d.Pasos[i].ID == "" {
			for {
				n++
				c := fmt.Sprintf("p%d", n)
				if !usados[c] {
					d.Pasos[i].ID = c
					usados[c] = true
					break
				}
			}
		}
	}
	vars := map[string]bool{}
	for _, p := range d.Pasos {
		if p.Tipo == PasoPedir && p.Variable != "" {
			vars[p.Variable] = true
		}
	}
	k := 0
	for i := range d.Pasos {
		if d.Pasos[i].Tipo == PasoPedir && d.Pasos[i].Variable == "" {
			for {
				k++
				c := fmt.Sprintf("dato%d", k)
				if !vars[c] {
					d.Pasos[i].Variable = c
					vars[c] = true
					break
				}
			}
		}
	}
	if d.Inicio == "" && len(d.Pasos) > 0 {
		d.Inicio = d.Pasos[0].ID
	}
	d.normalizarExtras()
	d.Nombre = strings.TrimSpace(d.Nombre)
	if d.Nombre == "" {
		d.Nombre = "Automatización sin nombre"
	}
}

// Aviso es una observación de la validación. Grave = no se debería activar así.
type Aviso struct {
	Paso  string `json:"paso"`
	Texto string `json:"texto"`
	Grave bool   `json:"grave"`
}

func (p *Paso) esFinal() bool {
	switch p.Tipo {
	case PasoFin, PasoHumano:
		return true
	case PasoRespuesta, PasoPedir, PasoForm:
		return p.Siguiente == ""
	}
	return false
}

func (p *Paso) destinos() []string {
	var out []string
	switch p.Tipo {
	case PasoMenu:
		for _, o := range p.Opciones {
			if o.Destino != "" {
				out = append(out, o.Destino)
			}
		}
	case PasoRespuesta, PasoPedir, PasoForm:
		if p.Siguiente != "" {
			out = append(out, p.Siguiente)
		}
	}
	return out
}

// Validar revisa la automatización: opciones sin destino, pasos huérfanos, ciclos sin salida y cosas que faltan.
func (d *Def) Validar() []Aviso {
	var av []Aviso
	add := func(paso, texto string, grave bool) { av = append(av, Aviso{Paso: paso, Texto: texto, Grave: grave}) }
	if len(d.Pasos) == 0 {
		add("", "No tiene ningún paso.", true)
		return av
	}
	ids := map[string]bool{}
	for _, p := range d.Pasos {
		if ids[p.ID] {
			add(p.ID, "Hay dos pasos con el mismo identificador.", true)
		}
		ids[p.ID] = true
	}
	if !ids[d.Inicio] {
		add("", "El paso inicial no existe.", true)
		return av
	}
	for _, p := range d.Pasos {
		switch p.Tipo {
		case PasoMenu:
			if strings.TrimSpace(p.Titulo) == "" {
				add(p.ID, "El menú no tiene título.", false)
			}
			if len(p.Opciones) == 0 {
				add(p.ID, "El menú no tiene opciones.", true)
			}
			switch p.Formato {
			case "", "texto", "encuesta", "botones":
			default:
				add(p.ID, "El formato del menú no es válido: usa texto, encuesta o botones.", true)
			}
			for i, o := range p.Opciones {
				if msg := o.Boton.problema(); msg != "" {
					add(p.ID, fmt.Sprintf("La opción %d (%s): %s", i+1, o.Texto, msg), true)
				}
				switch {
				case strings.TrimSpace(o.Texto) == "":
					add(p.ID, fmt.Sprintf("La opción %d no tiene texto.", i+1), true)
				case o.Destino == "":
					add(p.ID, fmt.Sprintf("La opción %d (%s) no lleva a ningún paso.", i+1, o.Texto), true)
				case !ids[o.Destino]:
					add(p.ID, fmt.Sprintf("La opción %d (%s) lleva a un paso que ya no existe.", i+1, o.Texto), true)
				}
			}
		case PasoRespuesta:
			if strings.TrimSpace(p.Texto) == "" && len(p.Adjuntos) == 0 && p.Ubicacion == nil && p.Contacto == nil {
				add(p.ID, "La respuesta está vacía.", true)
			}
		case PasoPedir:
			if strings.TrimSpace(p.Pregunta) == "" {
				add(p.ID, "El paso no tiene pregunta.", true)
			}
		case PasoHumano, PasoFin, PasoForm:
		default:
			add(p.ID, "Tipo de paso desconocido.", true)
		}
		if (p.Tipo == PasoRespuesta || p.Tipo == PasoPedir || p.Tipo == PasoForm) && p.Siguiente != "" && !ids[p.Siguiente] {
			add(p.ID, "Sigue a un paso que ya no existe.", true)
		}
	}
	d.validarExtras(add)
	// pasos que nadie alcanza desde el inicio
	visto := map[string]bool{}
	cola := []string{d.Inicio}
	for len(cola) > 0 {
		id := cola[0]
		cola = cola[1:]
		if visto[id] {
			continue
		}
		visto[id] = true
		if p := d.Paso(id); p != nil {
			cola = append(cola, p.destinos()...)
		}
	}
	for _, p := range d.Pasos {
		if !visto[p.ID] {
			add(p.ID, "Ningún paso lleva a este (queda suelto).", false)
		}
	}
	// ciclos sin salida: pasos alcanzables desde los que no se llega a ningún final
	conSalida := map[string]bool{}
	cambio := true
	for _, p := range d.Pasos {
		if p.esFinal() {
			conSalida[p.ID] = true
		}
	}
	for cambio {
		cambio = false
		for _, p := range d.Pasos {
			if conSalida[p.ID] {
				continue
			}
			for _, t := range p.destinos() {
				if conSalida[t] {
					conSalida[p.ID], cambio = true, true
					break
				}
			}
		}
	}
	var sinSalida []string
	for id := range visto {
		if !conSalida[id] && ids[id] {
			sinSalida = append(sinSalida, id)
		}
	}
	sort.Strings(sinSalida)
	for _, id := range sinSalida {
		add(id, "Desde aquí no se puede terminar: es un ciclo sin salida.", true)
	}
	t := d.Disparadores
	if !t.PrimerMensaje && t.DiasSinEscribir == 0 && len(t.Palabras) == 0 && t.Comando == "" && !t.FueraDeHorario && !t.Mencion && !t.Cualquiera {
		add("", "No tiene cuándo activarse: elige al menos un disparador.", false)
	}
	if !d.Alcance.Privados && len(d.Alcance.Chats) == 0 && len(d.Alcance.Grupos) == 0 && !d.Alcance.TodosGrupos {
		add("", "No está asignada a ningún chat.", false)
	}
	if (len(d.Alcance.Grupos) > 0 || d.Alcance.TodosGrupos) && !t.Mencion && t.Comando == "" && len(t.Palabras) == 0 && !t.Cualquiera {
		add("", "En grupos solo responde si lo mencionan o usan un comando.", false)
	}
	return av
}

// problema dice qué está mal en un botón especial ("" = bien o no hay botón).
func (b *BotonOpcion) problema() string {
	if b == nil || (b.Tipo == "" && strings.TrimSpace(b.Valor) == "") {
		return ""
	}
	v := strings.TrimSpace(b.Valor)
	switch b.Tipo {
	case "enlace":
		if !(strings.HasPrefix(strings.ToLower(v), "https://") || strings.HasPrefix(strings.ToLower(v), "http://")) || len(v) > 500 || strings.ContainsAny(v, " \n\t") {
			return "el enlace debe empezar con https:// y no llevar espacios."
		}
	case "copiar":
		if v == "" || len([]rune(v)) > 200 {
			return "el texto a copiar va de 1 a 200 letras."
		}
	case "llamar":
		n := 0
		for _, r := range v {
			switch {
			case r >= '0' && r <= '9':
				n++
			case r == '+' || r == ' ' || r == '-' || r == '(' || r == ')':
			default:
				return "el número solo lleva dígitos, +, espacios y guiones."
			}
		}
		if n < 7 || n > 15 {
			return "el número debe tener de 7 a 15 dígitos."
		}
	default:
		return "el botón especial debe ser enlace, copiar o llamar."
	}
	return ""
}
