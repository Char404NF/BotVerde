package flujo

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"botchat/internal/identidad"
)

// Validación de las respuestas de un formulario. Cada función devuelve el valor ya normalizado y si sirve.

var reCorreo = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9\-]+(\.[a-z0-9\-]+)*\.[a-z]{2,}$`)

func digitos(s string) string { return identidad.Digitos(s) }

var meses = map[string]int{
	"enero": 1, "ene": 1, "febrero": 2, "feb": 2, "marzo": 3, "mar": 3, "abril": 4, "abr": 4, "mayo": 5, "may": 5, "junio": 6, "jun": 6,
	"julio": 7, "jul": 7, "agosto": 8, "ago": 8, "septiembre": 9, "setiembre": 9, "sep": 9, "sept": 9, "octubre": 10, "oct": 10,
	"noviembre": 11, "nov": 11, "diciembre": 12, "dic": 12,
}

var (
	reFechaNum = regexp.MustCompile(`^(\d{1,2})[/\-.](\d{1,2})(?:[/\-.](\d{2}|\d{4}))?$`)
	reFechaISO = regexp.MustCompile(`^(\d{4})[/\-.](\d{1,2})[/\-.](\d{1,2})$`)
	reFechaTxt = regexp.MustCompile(`^(\d{1,2})\s*(?:de\s+)?([a-z]+)\.?(?:\s*(?:de\s+|del\s+)?(\d{2}|\d{4}))?$`)
)

// NormalizarFecha entiende varios formatos (25/12/2026, 25-12-26, 2026-12-25, 25 de diciembre de 2026, 25 dic, hoy, mañana…) y la
// devuelve como dd/mm/aaaa. Sin año se toma el año de `hoy`. La fecha tiene que existir.
func NormalizarFecha(texto string, hoy time.Time) (string, bool) {
	t := norm(texto)
	t = strings.Join(strings.Fields(t), " ")
	switch t {
	case "hoy":
		return hoy.Format("02/01/2006"), true
	case "manana":
		return hoy.AddDate(0, 0, 1).Format("02/01/2006"), true
	case "pasado manana":
		return hoy.AddDate(0, 0, 2).Format("02/01/2006"), true
	}
	var d, m, y int
	if mm := reFechaISO.FindStringSubmatch(t); mm != nil {
		y, _ = strconv.Atoi(mm[1])
		m, _ = strconv.Atoi(mm[2])
		d, _ = strconv.Atoi(mm[3])
	} else if mm := reFechaNum.FindStringSubmatch(t); mm != nil {
		d, _ = strconv.Atoi(mm[1])
		m, _ = strconv.Atoi(mm[2])
		y = anio(mm[3], hoy)
	} else if mm := reFechaTxt.FindStringSubmatch(t); mm != nil {
		mes, ok := meses[mm[2]]
		if !ok {
			return "", false
		}
		d, m = 0, mes
		d, _ = strconv.Atoi(mm[1])
		y = anio(mm[3], hoy)
	} else {
		return "", false
	}
	if y < 1900 || y > 2100 || m < 1 || m > 12 || d < 1 || d > 31 {
		return "", false
	}
	f := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	if f.Day() != d || int(f.Month()) != m { // 31/02 no existe
		return "", false
	}
	return f.Format("02/01/2006"), true
}

func anio(s string, hoy time.Time) int {
	if s == "" {
		return hoy.Year()
	}
	n, _ := strconv.Atoi(s)
	if len(s) == 2 {
		n += 2000
	}
	return n
}

// NormalizarNumero entiende 25, 1,5, 1.5, 1,234.50, 1.234,50 y $100 y lo devuelve con punto decimal y sin separadores de miles.
func NormalizarNumero(texto string) (float64, string, bool) {
	t := strings.TrimSpace(texto)
	t = strings.NewReplacer("$", "", "€", "", " ", "", " ", "", "'", "").Replace(t)
	if t == "" || len(t) > 24 {
		return 0, "", false
	}
	signo := ""
	if t[0] == '-' || t[0] == '+' {
		if t[0] == '-' {
			signo = "-"
		}
		t = t[1:]
	}
	for _, r := range t {
		if !(r >= '0' && r <= '9' || r == '.' || r == ',') {
			return 0, "", false
		}
	}
	if t == "" || t[0] == '.' && strings.Count(t, ".") > 1 {
		return 0, "", false
	}
	pc, pp := strings.LastIndex(t, ","), strings.LastIndex(t, ".")
	var ent, dec string
	switch {
	case pc >= 0 && pp >= 0: // el último símbolo es el decimal
		sep := pc
		if pp > pc {
			sep = pp
		}
		ent, dec = t[:sep], t[sep+1:]
	case pc >= 0 || pp >= 0:
		sep := pc
		simbolo := ","
		if pp >= 0 {
			sep, simbolo = pp, "."
		}
		if strings.Count(t, simbolo) > 1 { // 1.234.567: miles
			ent, dec = t, ""
		} else if len(t)-sep-1 == 3 && sep > 0 && sep <= 3 && t[:sep] != "0" { // 1,234: miles
			ent, dec = t, ""
		} else {
			ent, dec = t[:sep], t[sep+1:]
		}
	default:
		ent = t
	}
	if strings.ContainsAny(ent, ",.") {
		grupos := strings.FieldsFunc(ent, func(r rune) bool { return r == ',' || r == '.' })
		for i, g := range grupos {
			if (i == 0 && (len(g) < 1 || len(g) > 3)) || (i > 0 && len(g) != 3) {
				return 0, "", false
			}
		}
	}
	ent = strings.NewReplacer(",", "", ".", "").Replace(ent)
	if ent == "" {
		ent = "0"
	}
	if strings.ContainsAny(dec, ",.") {
		return 0, "", false
	}
	s := signo + ent
	if dec != "" {
		s += "." + dec
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, "", false
	}
	return v, strconv.FormatFloat(v, 'f', -1, 64), true
}

// ValidarCampo revisa lo que escribió la persona para ese campo. Devuelve el valor normalizado, y si no sirve, un mensaje que dice qué falta.
func ValidarCampo(c Campo, texto, region string, hoy time.Time) (valor, mensaje string, ok bool) {
	t := strings.TrimSpace(texto)
	fallo := func(def string) (string, string, bool) {
		if strings.TrimSpace(c.Error) != "" {
			return "", strings.TrimSpace(c.Error), false
		}
		return "", def, false
	}
	if t == "" {
		return fallo("Escribe una respuesta, por favor.")
	}
	switch c.Tipo {
	case CampoNumero:
		v, s, ok := NormalizarNumero(t)
		if !ok {
			return fallo("Eso no parece un número. Escribe solo el número, por ejemplo 25.")
		}
		if (c.Min != nil && v < *c.Min) || (c.Max != nil && v > *c.Max) {
			switch {
			case c.Min != nil && c.Max != nil:
				return fallo(fmt.Sprintf("Escribe un número entre %s y %s.", fmtNum(*c.Min), fmtNum(*c.Max)))
			case c.Min != nil:
				return fallo(fmt.Sprintf("Escribe un número que sea %s o más.", fmtNum(*c.Min)))
			default:
				return fallo(fmt.Sprintf("Escribe un número que sea %s o menos.", fmtNum(*c.Max)))
			}
		}
		return s, "", true
	case CampoCorreo:
		e := strings.ToLower(t)
		if len(e) > 254 || strings.ContainsAny(e, " \t\n") || !reCorreo.MatchString(e) {
			return fallo("Ese correo no parece correcto. Escríbelo así: nombre@correo.com")
		}
		return e, "", true
	case CampoTelefono:
		tel, ok := identidad.ValidarTelefono(t, region)
		if !ok {
			return fallo("Ese teléfono no parece correcto. Escríbelo con lada, por ejemplo 55 1234 5678.")
		}
		return tel, "", true
	case CampoFecha:
		f, ok := NormalizarFecha(t, hoy)
		if !ok {
			return fallo("No entendí la fecha. Escríbela así: 25/12/2026.")
		}
		return f, "", true
	case CampoOpcion:
		vis := make([]visible, 0, len(c.Opciones))
		for i, o := range c.Opciones {
			if strings.TrimSpace(o) != "" {
				vis = append(vis, visible{Idx: i, Num: len(vis) + 1, O: Opcion{Texto: strings.TrimSpace(o)}})
			}
		}
		v, amb, ok := elige(vis, t)
		if !ok {
			if len(amb) > 1 {
				return fallo("Hay varias parecidas. Escribe el número de la opción.")
			}
			return fallo("Elige una de las opciones escribiendo su número.")
		}
		return v.O.Texto, "", true
	}
	// texto
	if len([]rune(t)) > 500 {
		return fallo("Es muy largo (máximo 500 letras). Escríbelo más corto, por favor.")
	}
	return t, "", true
}

func fmtNum(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
