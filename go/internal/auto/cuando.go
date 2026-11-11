package auto

import (
	"fmt"
	"strings"
	"time"

	"botchat/internal/flujo"
)

// Zona devuelve la zona horaria del mensaje (la suya si trae una válida, si no la `defecto`).
func (c Cuando) ZonaDe(defecto *time.Location) *time.Location {
	if c.Zona != "" {
		if l, err := time.LoadLocation(c.Zona); err == nil {
			return l
		}
	}
	if defecto == nil {
		return time.Local
	}
	return defecto
}

// Activo dice si en ese momento (en la zona dada) se cumple alguna de las condiciones.
func (c Cuando) Activo(t time.Time, defecto *time.Location) bool {
	t = t.In(c.ZonaDe(defecto))
	if c.Siempre {
		return true
	}
	if len(c.Horario) > 0 {
		dentro := SemanaContiene(c.Horario, t)
		if (c.HorarioModo == "fuera") == !dentro {
			return true
		}
	}
	dia := t.Format("2006-01-02")
	for _, v := range c.Vacaciones {
		if v.Desde <= dia && dia <= v.Hasta {
			return true
		}
	}
	for _, f := range c.Fechas {
		if fechaCoincide(f, t) {
			return true
		}
	}
	return false
}

// fechaCoincide: es ese día y mes (el 29 de febrero se celebra el 28 en los años que no son bisiestos).
func fechaCoincide(f FechaAnual, t time.Time) bool {
	if int(t.Month()) != f.Mes {
		return false
	}
	if t.Day() == f.Dia {
		return true
	}
	if f.Mes == 2 && f.Dia == 29 && t.Day() == 28 {
		y := t.Year()
		bisiesto := y%4 == 0 && (y%100 != 0 || y%400 == 0)
		return !bisiesto
	}
	return false
}

func diaSemana(t time.Time) int {
	d := int(t.Weekday())
	if d == 0 {
		return 7
	}
	return d
}

// SemanaContiene dice si el momento cae dentro de alguno de los rangos del horario semanal. Un rango que cruza la medianoche
// (22:00 a 02:00) cuenta también en las primeras horas del día siguiente.
func SemanaContiene(h []DiaHorario, t time.Time) bool {
	min := t.Hour()*60 + t.Minute()
	hoy, ayer := diaSemana(t), diaSemana(t.AddDate(0, 0, -1))
	for _, d := range h {
		for _, r := range d.Rangos {
			a, ok1 := hora(r.Desde)
			b, ok2 := hora(r.Hasta)
			if !ok1 || !ok2 || a == b {
				continue
			}
			if d.Dia == hoy {
				if a < b && min >= a && min < b {
					return true
				}
				if a > b && min >= a {
					return true
				}
			}
			if d.Dia == ayer && a > b && min < b {
				return true
			}
		}
	}
	return false
}

// DescribirSemana dice un horario semanal con palabras: "de lunes a viernes de 09:00 a 13:00 y de 15:00 a 18:00; sábado de 09:00 a 13:00".
func DescribirSemana(h []DiaHorario) string {
	type grupo struct {
		firma string
		dias  []int
		rang  []Rango
	}
	var gs []*grupo
	for d := 1; d <= 7; d++ {
		for _, x := range h {
			if x.Dia != d || len(x.Rangos) == 0 {
				continue
			}
			var partes []string
			for _, r := range x.Rangos {
				partes = append(partes, r.Desde+"-"+r.Hasta)
			}
			f := strings.Join(partes, ",")
			var g *grupo
			for _, y := range gs {
				if y.firma == f {
					g = y
				}
			}
			if g == nil {
				g = &grupo{firma: f, rang: x.Rangos}
				gs = append(gs, g)
			}
			g.dias = append(g.dias, d)
		}
	}
	if len(gs) == 0 {
		return "todos los días"
	}
	var out []string
	for _, g := range gs {
		var rs []string
		for _, r := range g.rang {
			rs = append(rs, fmt.Sprintf("de %s a %s", r.Desde, r.Hasta))
		}
		out = append(out, strings.TrimSpace(flujo.DescribirHorario(flujo.Horario{Dias: g.dias})+" "+strings.Join(rs, " y ")))
	}
	return strings.Join(out, "; ")
}
