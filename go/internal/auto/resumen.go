package auto

import (
	"fmt"
	"strings"

	"botchat/internal/almacen"
)

// ResumenCuando dice con pocas palabras cuándo se manda.
func ResumenCuando(c Config) string {
	cu := c.Cuando
	var p []string
	if cu.Siempre {
		p = append(p, "siempre")
	}
	if len(cu.Horario) > 0 {
		if cu.HorarioModo == "fuera" {
			p = append(p, "fuera de horario ("+DescribirSemana(cu.Horario)+")")
		} else {
			p = append(p, "en horario ("+DescribirSemana(cu.Horario)+")")
		}
	}
	for _, v := range cu.Vacaciones {
		p = append(p, "vacaciones "+fechaCorta(v.Desde)+" a "+fechaCorta(v.Hasta))
	}
	if n := len(cu.Fechas); n > 0 {
		var f []string
		for _, x := range cu.Fechas {
			f = append(f, fmt.Sprintf("%d/%d", x.Dia, x.Mes))
		}
		p = append(p, "fechas especiales ("+strings.Join(f, ", ")+")")
	}
	if len(p) == 0 {
		return "siempre"
	}
	return strings.Join(p, " · ")
}

func fechaCorta(s string) string {
	if len(s) == 10 {
		return s[8:10] + "/" + s[5:7]
	}
	return s
}

// ResumenQuien dice con pocas palabras a quién se le manda.
func ResumenQuien(c Config) string {
	q := c.Quien
	var p []string
	switch q.Personas {
	case "desconocidos":
		p = append(p, "solo desconocidos")
	case "contactos":
		p = append(p, "solo contactos")
	default:
		p = append(p, "todos")
	}
	switch {
	case q.Privados && q.Grupos:
		p = append(p, "privados y grupos")
	case q.Grupos:
		p = append(p, "solo grupos")
	default:
		p = append(p, "solo privados")
	}
	if n := len(q.SoloLista); n > 0 {
		p = append(p, fmt.Sprintf("solo %d de la lista", n))
	}
	if n := len(q.Excepto); n > 0 {
		p = append(p, fmt.Sprintf("excepto %d", n))
	}
	return strings.Join(p, " · ")
}

// ResumenRepeticion dice cada cuánto se repite a la misma persona.
func ResumenRepeticion(tipo string, c Config) string {
	if tipo == almacen.AutoBienvenida {
		if c.SilencioDias > 0 {
			return fmt.Sprintf("primera vez y tras %d días sin escribir", c.SilencioDias)
		}
		return "solo la primera vez"
	}
	r := c.Repeticion
	if r.Cada <= 0 {
		return "cada vez"
	}
	u := map[string]string{"minutos": "minutos", "horas": "horas", "dias": "días"}[r.Unidad]
	if r.Cada == 1 {
		u = map[string]string{"minutos": "minuto", "horas": "hora", "dias": "día"}[r.Unidad]
		return "una vez por " + u
	}
	return fmt.Sprintf("una vez cada %d %s", r.Cada, u)
}
