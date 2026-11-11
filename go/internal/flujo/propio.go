package flujo

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Comandos propios (bloque B): "!horario", "!precios"… con un texto (variables {nombre} {hora} {dia} y estilos de WhatsApp como
// *negrita*), o un menú de opciones numeradas, con un rango mínimo para usarlo y visibilidad en !ayuda. Se guardan como una
// automatización normal (mismo motor de menús, mismo prefijo configurable); sus datos viven en Def.Propio.

// OpcionPropia es una opción de un comando propio con menú.
type OpcionPropia struct {
	Texto     string `json:"texto"`
	Respuesta string `json:"respuesta"`
}

// DatosPropio son los datos que llena la pantalla "Comandos propios".
type DatosPropio struct {
	Nombre      string         `json:"nombre"`   // sin prefijo ("horario"); el prefijo es el configurado
	Texto       string         `json:"texto"`    // respuesta (o título del menú si hay opciones)
	Resumen     string         `json:"resumen"`  // línea corta para !ayuda
	RangoMin    int64          `json:"rangoMin"` // id del rango mínimo (0 = cualquiera)
	Privados    bool           `json:"privados"`
	Grupos      bool           `json:"grupos"` // en cualquier grupo donde el bot responde
	OcultoAyuda bool           `json:"ocultoAyuda,omitempty"`
	Opciones    []OpcionPropia `json:"opciones,omitempty"`
	// Adjuntos queda reservado: los adjuntos de mensajes automáticos llegan con el Bloque D y se podrán reutilizar aquí.
	Adjuntos []string `json:"adjuntos,omitempty"`
}

// NombreComandoValido revisa que el nombre sea una sola palabra de letras y números.
func NombreComandoValido(n string) error {
	n = strings.TrimSpace(n)
	if n == "" {
		return fmt.Errorf("escribe el nombre del comando, por ejemplo horario")
	}
	if len([]rune(n)) > 20 {
		return fmt.Errorf("el nombre es muy largo (máximo 20 letras)")
	}
	for _, r := range n {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return fmt.Errorf("el nombre solo puede tener letras y números, sin espacios ni símbolos")
		}
	}
	return nil
}

// Validar revisa los datos del comando propio.
func (p *DatosPropio) Validar() error {
	p.Nombre = strings.ToLower(strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(p.Nombre), "!/#.")))
	if err := NombreComandoValido(p.Nombre); err != nil {
		return err
	}
	p.Texto = strings.TrimSpace(p.Texto)
	if p.Texto == "" {
		return fmt.Errorf("escribe qué responde el comando")
	}
	if len([]rune(p.Texto)) > 2000 {
		return fmt.Errorf("el texto es muy largo (máximo 2000 letras)")
	}
	p.Resumen = strings.TrimSpace(p.Resumen)
	if len([]rune(p.Resumen)) > 80 {
		return fmt.Errorf("la descripción para la ayuda es muy larga (máximo 80 letras)")
	}
	if !p.Privados && !p.Grupos {
		return fmt.Errorf("elige dónde funciona: en privados, en grupos o en ambos")
	}
	if len(p.Opciones) > 0 && p.Grupos {
		return fmt.Errorf("los comandos con opciones solo funcionan en chats privados: en un grupo nadie puede contestar con un número")
	}
	if len(p.Opciones) > 9 {
		return fmt.Errorf("un menú puede tener hasta 9 opciones")
	}
	for i := range p.Opciones {
		p.Opciones[i].Texto = strings.TrimSpace(p.Opciones[i].Texto)
		p.Opciones[i].Respuesta = strings.TrimSpace(p.Opciones[i].Respuesta)
		if p.Opciones[i].Texto == "" || p.Opciones[i].Respuesta == "" {
			return fmt.Errorf("la opción %d necesita un texto y una respuesta", i+1)
		}
	}
	return nil
}

// ConstruirPropio genera los pasos, el disparador y el alcance a partir de Def.Propio.
func (d *Def) ConstruirPropio() error {
	p := d.Propio
	if p == nil {
		return nil
	}
	if err := p.Validar(); err != nil {
		return err
	}
	d.Nombre = "Comando " + p.Nombre
	d.Disparadores = Disparadores{Comando: p.Nombre}
	d.Alcance = Alcance{Privados: p.Privados, TodosGrupos: p.Grupos}
	d.Horario = Horario{}
	if len(p.Opciones) == 0 {
		d.Pasos = []Paso{{ID: "r1", Tipo: PasoRespuesta, Texto: p.Texto}}
		d.Inicio = "r1"
		return nil
	}
	m := Paso{ID: "m1", Tipo: PasoMenu, Titulo: p.Texto}
	d.Pasos = []Paso{m}
	for i, o := range p.Opciones {
		id := fmt.Sprintf("o%d", i+1)
		d.Pasos[0].Opciones = append(d.Pasos[0].Opciones, Opcion{Texto: o.Texto, Destino: id})
		d.Pasos = append(d.Pasos, Paso{ID: id, Tipo: PasoRespuesta, Texto: o.Respuesta})
	}
	d.Inicio = "m1"
	return nil
}

// AyudaComandos son las líneas de !ayuda para los comandos propios que esa persona puede usar aquí.
func (m *Motor) AyudaComandos(esGrupo bool, chat, sender string, dueno bool) []string {
	menus, err := m.c.Alm.ListarMenus(m.c.BotID)
	if err != nil {
		return nil
	}
	pref := "!"
	if m.c.Prefijo != nil && m.c.Prefijo() != "" {
		pref = m.c.Prefijo()
	}
	var out []string
	for _, mn := range menus {
		if !mn.Activo {
			continue
		}
		d, err := Parse(mn.Def)
		if err != nil || d.Propio == nil || d.Propio.OcultoAyuda {
			continue
		}
		p := d.Propio
		if esGrupo && !p.Grupos || !esGrupo && !p.Privados {
			continue
		}
		if !dueno && p.RangoMin > 0 && (m.c.PuedeUsar == nil || !m.c.PuedeUsar(sender, p.RangoMin)) {
			continue
		}
		l := "• " + pref + p.Nombre
		if p.Resumen != "" {
			l += " — " + p.Resumen
		}
		out = append(out, l)
	}
	return out
}

// TextoDeEjemplo pone datos de ejemplo en {nombre}, {hora} y {dia} (para la vista previa del comando).
func TextoDeEjemplo(t, nombre string, ahora time.Time, zona *time.Location) string {
	return reVar.ReplaceAllStringFunc(t, func(x string) string {
		switch strings.ToLower(x[1 : len(x)-1]) {
		case "nombre":
			return nombre
		case "hora":
			return ahora.In(zona).Format("15:04")
		case "dia", "día":
			return dias[int(ahora.In(zona).Weekday())]
		}
		return "…"
	})
}
