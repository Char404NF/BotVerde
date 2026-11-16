package flujo

import (
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Cómo se entiende lo que escribe la persona dentro de un menú: número, texto exacto, texto parcial y comandos.

// visible es una opción que esa persona puede ver y elegir ahora. Num es el número con que se le muestra (1, 2, 3…) e Idx su lugar
// en la definición (las estadísticas cuentan por Idx+1, así no cambian si a otra persona le ocultan opciones).
type visible struct {
	Idx int
	Num int
	O   Opcion
}

const (
	minParcial = 3  // letras mínimas para entender un texto parcial
	maxParcial = 40 // un mensaje más largo que esto no se toma como "una opción escrita a medias"
)

// elige resuelve la opción que eligió la persona: por número, por el texto exacto o por una parte del texto (prefijo único, o
// palabras que coinciden). Si varias opciones encajan, devuelve la lista en `ambiguas` y ok=false.
func elige(vis []visible, texto string) (v visible, ambiguas []visible, ok bool) {
	if v, ok := eligeExacta(vis, texto); ok {
		return v, nil, true
	}
	return eligeParcial(vis, texto)
}

// eligeExacta: número dentro de la lista o el texto completo de la opción (sin acentos ni mayúsculas).
func eligeExacta(vis []visible, texto string) (visible, bool) {
	if n, ok := numeroDeOpcion(texto); ok && n >= 1 {
		for _, v := range vis {
			if v.Num == n {
				return v, true
			}
		}
	}
	t := norm(texto)
	if t == "" {
		return visible{}, false
	}
	for _, v := range vis {
		if norm(v.O.Texto) == t {
			return v, true
		}
	}
	return visible{}, false
}

func palabras(s string) []string {
	return strings.FieldsFunc(norm(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// eligeParcial: el texto es el principio de UNA opción, o todas sus palabras empiezan igual que palabras de UNA opción.
func eligeParcial(vis []visible, texto string) (visible, []visible, bool) {
	t := strings.Join(palabras(texto), " ")
	if len([]rune(t)) < minParcial || len([]rune(texto)) > maxParcial {
		return visible{}, nil, false
	}
	var pref, pal []visible
	qs := strings.Fields(t)
	for _, v := range vis {
		ot := strings.Join(palabras(v.O.Texto), " ")
		if strings.HasPrefix(ot, t) {
			pref = append(pref, v)
			continue
		}
		ow := strings.Fields(ot)
		if todasEmpiezan(qs, ow) {
			pal = append(pal, v)
		}
	}
	for _, c := range [][]visible{pref, pal} {
		switch len(c) {
		case 0:
		case 1:
			return c[0], nil, true
		default:
			sort.Slice(c, func(i, j int) bool { return c[i].Num < c[j].Num })
			return visible{}, c, false
		}
	}
	return visible{}, nil, false
}

// todasEmpiezan: cada palabra escrita es el principio de una palabra distinta de la opción (las palabras sueltas muy cortas no cuentan).
func todasEmpiezan(qs, ow []string) bool {
	usadas := make([]bool, len(ow))
	for _, q := range qs {
		if len([]rune(q)) < 2 {
			return false
		}
		hallada := false
		for i, w := range ow {
			if !usadas[i] && strings.HasPrefix(w, q) {
				usadas[i], hallada = true, true
				break
			}
		}
		if !hallada {
			return false
		}
	}
	return len(qs) > 0
}

// ---------- comandos que siempre funcionan dentro de un menú ----------

const (
	CmdMenu     = "menu"
	CmdAtras    = "atras"
	CmdInicio   = "inicio"
	CmdSalir    = "salir"
	CmdPersona  = "persona"
	CmdRepetir  = "repetir"
	CmdCancelar = "cancelar"
)

var palabrasComando = map[string]string{
	"menu": CmdMenu, "menú": CmdMenu,
	"atras": CmdAtras, "atrás": CmdAtras, "volver": CmdAtras, "regresar": CmdAtras,
	"inicio": CmdInicio, "empezar": CmdInicio, "reiniciar": CmdInicio,
	"salir": CmdSalir, "adios": CmdSalir, "adiós": CmdSalir, "terminar": CmdSalir,
	"persona": CmdPersona, "humano": CmdPersona, "agente": CmdPersona, "asesor": CmdPersona,
	"repetir": CmdRepetir, "repite": CmdRepetir,
	"cancelar": CmdCancelar, "cancela": CmdCancelar,
}

// comandoSuelto devuelve el comando que dice el mensaje ("" si no es uno). Solo vale CON el símbolo de los comandos («!menu»,
// «!atrás»): la palabra suelta («menú», «salir», «persona») no activa nada, para que nadie lo dispare por error al conversar. Solo cuenta
// si el mensaje es ESA palabra: una frase larga que la contiene no es un comando.
func (m *Motor) comandoSuelto(texto string) string {
	t := strings.TrimSpace(texto)
	if t == "" || len([]rune(t)) > 20 || !m.conSimbolo(t) {
		return ""
	}
	pref := m.prefijo()
	t = strings.TrimPrefix(t, pref)
	if pref != "!" {
		t = strings.TrimPrefix(t, "!")
	}
	t = strings.TrimSpace(strings.TrimRight(t, ".!¡"))
	if c, ok := palabrasComando[strings.ToLower(t)]; ok {
		return c
	}
	if c, ok := palabrasComando[norm(t)]; ok {
		return c
	}
	return ""
}

func (m *Motor) prefijo() string {
	if m.c.Prefijo != nil {
		if p := m.c.Prefijo(); p != "" {
			return p
		}
	}
	return "!"
}

// esNeutro: un saludo, un "gracias" o solo emojis y signos. No cuenta como error dentro de un menú.
func (m *Motor) esNeutro(texto string) bool {
	t := strings.TrimSpace(texto)
	if t == "" {
		return true
	}
	letras := false
	for _, r := range t {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			letras = true
			break
		}
	}
	if !letras {
		return true // emojis, signos o símbolos sueltos
	}
	lista := map[string]bool{}
	for _, f := range strings.FieldsFunc(m.ajuste("menu_saludos"), func(r rune) bool { return r == ',' || r == '\n' || r == ';' }) {
		if p := strings.Join(palabras(f), " "); p != "" {
			lista[p] = true
		}
	}
	// todas las palabras del mensaje deben ser palabras de la lista (así "hola buenas tardes" y "muchas gracias" valen)
	ws := palabras(t)
	if len(ws) == 0 || len(ws) > 6 {
		return false
	}
	conocidas := map[string]bool{}
	for k := range lista {
		for _, w := range strings.Fields(k) {
			conocidas[w] = true
		}
	}
	for _, w := range ws {
		if !conocidas[w] {
			return false
		}
	}
	return true
}

func numeroDeOpcion(t string) (int, bool) {
	t = strings.TrimSpace(t)
	if len([]rune(t)) > umbralOpcion {
		return 0, false
	}
	mm := reNumero.FindStringSubmatch(t)
	if mm == nil {
		return 0, false
	}
	n, err := strconv.Atoi(mm[1])
	return n, err == nil
}
