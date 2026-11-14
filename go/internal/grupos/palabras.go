package grupos

import (
	"strings"
	"unicode"
)

// Coincidencia de palabras prohibidas que entiende los trucos de siempre: letras cambiadas por números o símbolos (p3nd3j0, put@),
// letras separadas (p u t o, p.u.t.o), letras repetidas (puuuuto) y acentos. Compara palabras completas: "computadora" no es "puta".

// sustituciones: lo que se escribe en lugar de una letra.
var sustituciones = map[rune]rune{
	'0': 'o', '1': 'i', '3': 'e', '4': 'a', '5': 's', '7': 't', '8': 'b', '9': 'g',
	'@': 'a', '$': 's', '!': 'i', '|': 'i', '¡': 'i', '€': 'e', '+': 't',
}

func sinAcento(r rune) rune {
	switch r {
	case 'á', 'à', 'ä', 'â':
		return 'a'
	case 'é', 'è', 'ë', 'ê':
		return 'e'
	case 'í', 'ì', 'ï', 'î':
		return 'i'
	case 'ó', 'ò', 'ö', 'ô':
		return 'o'
	case 'ú', 'ù', 'ü', 'û':
		return 'u'
	case 'ñ':
		return 'n'
	}
	return r
}

// colapsa deja solo letras (a-z) sin acentos ni letras repetidas seguidas ("puuuto" → "puto"), cambiando los símbolos por su letra.
func colapsa(t string, sustituye bool) string {
	var b strings.Builder
	var ult rune
	for _, r := range strings.ToLower(t) {
		r = sinAcento(r)
		if sustituye {
			if x, ok := sustituciones[r]; ok {
				r = x
			}
		}
		if r < 'a' || r > 'z' {
			continue
		}
		if r == ult {
			continue
		}
		b.WriteRune(r)
		ult = r
	}
	return b.String()
}

// palabrasDe parte un texto en palabras ("a1 b2" → ["a1","b2"]) conservando letras, números y los símbolos que se usan como letras.
func palabrasDe(t string) []string {
	return strings.FieldsFunc(strings.ToLower(t), func(r rune) bool {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
		_, esSimbolo := sustituciones[r]
		return !esSimbolo
	})
}

// ContienePalabra dice si el texto trae la palabra o frase prohibida. Reglas:
//   - una palabra suelta coincide con una palabra del texto (aunque tenga números o símbolos en lugar de letras, o letras repetidas);
//   - también coincide si la escriben con las letras separadas ("p u t o", "p.u.t.o"): se juntan las letras sueltas y se revisa;
//   - una frase (con espacios) coincide si aparece completa, sin importar acentos, mayúsculas ni signos.
func ContienePalabra(texto, prohibida string) bool {
	p := strings.TrimSpace(prohibida)
	if p == "" {
		return false
	}
	clave := colapsa(p, false)
	if clave == "" {
		return false
	}
	partes := strings.Fields(p)
	if len(partes) > 1 {
		return strings.Contains(" "+strings.Join(palabrasLimpias(texto), " ")+" ", " "+strings.Join(palabrasLimpias(p), " ")+" ")
	}
	ws := palabrasDe(texto)
	for _, w := range ws {
		// los signos pegados al final o al principio ("puta!") no son letras cambiadas
		t := strings.TrimFunc(w, func(r rune) bool { _, ok := sustituciones[r]; return ok && !unicode.IsDigit(r) && r != '@' && r != '$' })
		if colapsa(w, true) == clave || colapsa(t, true) == clave || colapsa(w, false) == clave {
			return true
		}
	}
	// letras sueltas: una racha de palabras de una sola letra ("p u t o") se junta en una.
	var racha strings.Builder
	cierra := func() bool {
		if racha.Len() >= 3 && colapsa(racha.String(), true) == clave {
			return true
		}
		racha.Reset()
		return false
	}
	for _, w := range ws {
		if len([]rune(w)) == 1 {
			racha.WriteString(w)
			continue
		}
		if cierra() {
			return true
		}
	}
	return cierra()
}

// palabrasLimpias son las palabras de un texto sin acentos ni símbolos, para comparar frases.
func palabrasLimpias(t string) []string {
	return strings.Fields(strings.TrimSpace(quitaAcentos(t)))
}

// PalabrasDeFabrica es la lista inicial de groserías (español de México). La persona la edita en la app: quita o agrega lo que quiera.
// Son palabras completas: se comparan con letras cambiadas, separadas o repetidas.
var PalabrasDeFabrica = []string{
	"puto", "puta", "putos", "putas", "putada", "pendejo", "pendeja", "pendejos", "pendejas", "pendejada",
	"cabron", "cabrona", "cabrones", "chingar", "chingado", "chingada", "chingadera", "chingatumadre",
	"verga", "vergas", "mamada", "mamadas", "mamon", "mamona", "culero", "culera", "culeros", "joto",
	"marica", "maricon", "mierda", "hijodeputa", "hijueputa", "hdp", "ctm", "ptm", "malparido", "malparida",
	"zorra", "cojudo", "gilipollas", "imbecil", "estupido", "estupida", "vete a la verga", "chinga tu madre",
}

// PalabrasAdultasDeFabrica son términos sexuales explícitos que se suman al filtro de contenido adulto.
var PalabrasAdultasDeFabrica = []string{
	"porno", "pornografia", "xxx", "desnudos", "nudes", "onlyfans", "hentai", "cogiendo", "tetas", "orgia",
	"masturbacion", "escort", "videos xxx", "pack de fotos", "contenido para adultos",
}
