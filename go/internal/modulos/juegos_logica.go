package modulos

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Lógica pura de los juegos de H9 (sin envío ni base): dados, piedra-papel-tijera, ruleta, tres en raya, adivina y trivia.

// ---------- tres en raya ----------

var numerosCaja = []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣"}

func tableroTexto(t [9]int8) string {
	var b strings.Builder
	for i, c := range t {
		switch c {
		case 1:
			b.WriteString("❌")
		case 2:
			b.WriteString("⭕")
		default:
			b.WriteString(numerosCaja[i])
		}
		if i%3 == 2 && i < 8 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

var lineasTTT = [8][3]int{{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, {0, 3, 6}, {1, 4, 7}, {2, 5, 8}, {0, 4, 8}, {2, 4, 6}}

func ganadorTTT(t [9]int8) int8 {
	for _, l := range lineasTTT {
		if t[l[0]] != 0 && t[l[0]] == t[l[1]] && t[l[1]] == t[l[2]] {
			return t[l[0]]
		}
	}
	return 0
}

func llenoTTT(t [9]int8) bool {
	for _, c := range t {
		if c == 0 {
			return false
		}
	}
	return true
}

// jugadaBotTTT elige la jugada del bot (⭕): gana si puede, bloquea si debe, si no centro, esquina o lado. azar(n) da 0..n-1.
func jugadaBotTTT(t [9]int8, azar func(n int) int) int {
	for _, quien := range []int8{2, 1} {
		for _, l := range lineasTTT {
			n, vacio := 0, -1
			for _, i := range l {
				if t[i] == quien {
					n++
				} else if t[i] == 0 {
					vacio = i
				}
			}
			if n == 2 && vacio >= 0 {
				return vacio
			}
		}
	}
	if t[4] == 0 {
		return 4
	}
	for _, grupo := range [][]int{{0, 2, 6, 8}, {1, 3, 5, 7}} {
		var libres []int
		for _, i := range grupo {
			if t[i] == 0 {
				libres = append(libres, i)
			}
		}
		if len(libres) > 0 {
			return libres[azar(len(libres))]
		}
	}
	return -1
}

// ---------- trivia ----------

// PreguntaTrivia es una pregunta de un banco.
type PreguntaTrivia struct {
	P string   `json:"p"`
	C string   `json:"c"`
	I []string `json:"i"`
}

const bancoGeneral = "general"

var preguntasGeneral = []PreguntaTrivia{
	{"¿Cuál es la capital de Francia?", "París", []string{"Roma", "Madrid", "Berlín"}},
	{"¿Cuántos lados tiene un hexágono?", "6", []string{"5", "7", "8"}},
	{"¿Qué planeta es conocido como el planeta rojo?", "Marte", []string{"Venus", "Júpiter", "Saturno"}},
	{"¿Cuál es el océano más grande del mundo?", "Pacífico", []string{"Atlántico", "Índico", "Ártico"}},
	{"¿Quién pintó la Mona Lisa?", "Leonardo da Vinci", []string{"Pablo Picasso", "Vincent van Gogh", "Miguel Ángel"}},
	{"¿Cuál es el animal terrestre más grande?", "Elefante africano", []string{"Jirafa", "Rinoceronte", "Hipopótamo"}},
	{"¿Cuántos días tiene un año bisiesto?", "366", []string{"365", "364", "367"}},
	{"¿Cuál es el metal cuyo símbolo químico es Au?", "Oro", []string{"Plata", "Aluminio", "Cobre"}},
	{"¿En qué continente está Egipto?", "África", []string{"Asia", "Europa", "Oceanía"}},
	{"¿Cuál es el resultado de 7 × 8?", "56", []string{"54", "58", "49"}},
	{"¿Qué gas respiramos para vivir?", "Oxígeno", []string{"Nitrógeno", "Helio", "Dióxido de carbono"}},
	{"¿Cuál es el idioma más hablado en Brasil?", "Portugués", []string{"Español", "Inglés", "Francés"}},
}

func (p PreguntaTrivia) valida() error {
	if len([]rune(p.P)) < 5 || len([]rune(p.P)) > 300 {
		return fmt.Errorf("la pregunta debe tener entre 5 y 300 letras")
	}
	if strings.TrimSpace(p.C) == "" || len([]rune(p.C)) > 100 {
		return fmt.Errorf("la respuesta correcta debe tener entre 1 y 100 letras")
	}
	if len(p.I) < 1 || len(p.I) > 3 {
		return fmt.Errorf("pon entre 1 y 3 respuestas incorrectas (una por línea)")
	}
	for _, x := range p.I {
		if strings.TrimSpace(x) == "" || len([]rune(x)) > 100 {
			return fmt.Errorf("cada respuesta incorrecta debe tener entre 1 y 100 letras")
		}
		if llave(x) == llave(p.C) {
			return fmt.Errorf("una respuesta incorrecta es igual a la correcta")
		}
	}
	return nil
}

func (p PreguntaTrivia) json() string { b, _ := json.Marshal(p); return string(b) }

// opcionesTrivia revuelve las opciones y dice cuál es la correcta.
func opcionesTrivia(p PreguntaTrivia, azar func(n int) int) (ops []string, correcta int) {
	ops = append([]string{p.C}, p.I...)
	for i := len(ops) - 1; i > 0; i-- {
		j := azar(i + 1)
		ops[i], ops[j] = ops[j], ops[i]
	}
	for i, o := range ops {
		if o == p.C {
			correcta = i
		}
	}
	return ops, correcta
}

func preguntaTexto(p PreguntaTrivia, ops []string) string {
	var b strings.Builder
	b.WriteString("🧠 " + p.P + "\n")
	for i, o := range ops {
		b.WriteString(fmt.Sprintf("%c) %s\n", 'A'+i, o))
	}
	b.WriteString("Responde con la letra (A, B…) o el número.")
	return b.String()
}

// respuestaTrivia dice qué opción eligió el texto (-1 si no es una respuesta válida).
func respuestaTrivia(texto string, ops []string) int {
	t := llave(texto)
	if len(t) == 1 {
		if c := t[0]; c >= 'a' && c < 'a'+byte(len(ops)) {
			return int(c - 'a')
		} else if c >= '1' && c < '1'+byte(len(ops)) {
			return int(c - '1')
		}
	}
	for i, o := range ops {
		if t != "" && t == llave(o) {
			return i
		}
	}
	return -1
}

// ---------- dados, piedra-papel-tijera, ruleta ----------

// tiraDados lee «6», «20», «2d6» o «3d10» (hasta 10 dados de 2 a 1000 caras) y tira.
func tiraDados(arg string, azar func(n int) int) (string, error) {
	n, caras := 1, 6
	arg = strings.ToLower(strings.TrimSpace(arg))
	if arg != "" {
		a, b, esD := strings.Cut(arg, "d")
		var err1, err2 error
		if esD {
			if a != "" {
				n, err1 = strconv.Atoi(a)
			}
			caras, err2 = strconv.Atoi(b)
		} else {
			caras, err2 = strconv.Atoi(a)
		}
		if err1 != nil || err2 != nil || n < 1 || n > 10 || caras < 2 || caras > 1000 {
			return "", fmt.Errorf("usa por ejemplo: dado, dado 20 o dado 2d6 (hasta 10 dados de 2 a 1000 caras)")
		}
	}
	var partes []string
	total := 0
	for i := 0; i < n; i++ {
		v := azar(caras) + 1
		total += v
		partes = append(partes, strconv.Itoa(v))
	}
	if n == 1 {
		return fmt.Sprintf("🎲 Sacaste %d (dado de %d caras)", total, caras), nil
	}
	return fmt.Sprintf("🎲 Sacaste %s · total %d (%dd%d)", strings.Join(partes, " + "), total, n, caras), nil
}

var opcionesPPT = []string{"piedra", "papel", "tijera"}
var emojiPPT = map[string]string{"piedra": "🪨", "papel": "📄", "tijera": "✂️"}

// jugadaPPT devuelve el texto y 1 si ganó la persona, 0 si empate, -1 si perdió.
func jugadaPPT(arg string, azar func(n int) int) (string, int, error) {
	yo := llave(arg)
	if yo != "piedra" && yo != "papel" && yo != "tijera" {
		return "", 0, fmt.Errorf("elige: ppt piedra, ppt papel o ppt tijera")
	}
	bot := opcionesPPT[azar(3)]
	res := 0
	switch {
	case yo == bot:
	case (yo == "piedra" && bot == "tijera") || (yo == "papel" && bot == "piedra") || (yo == "tijera" && bot == "papel"):
		res = 1
	default:
		res = -1
	}
	fin := map[int]string{1: "¡Ganaste! 🎉", 0: "Empate 🤝", -1: "Gané yo 😎"}[res]
	return fmt.Sprintf("Tú: %s %s · Yo: %s %s\n%s", emojiPPT[yo], yo, emojiPPT[bot], bot, fin), res, nil
}

var rojosRuleta = map[int]bool{1: true, 3: true, 5: true, 7: true, 9: true, 12: true, 14: true, 16: true, 18: true, 19: true, 21: true, 23: true, 25: true, 27: true, 30: true, 32: true, 34: true, 36: true}

func colorRuleta(n int) string {
	switch {
	case n == 0:
		return "verde"
	case rojosRuleta[n]:
		return "rojo"
	}
	return "negro"
}

// ruleta gira (0 a 36). apuesta: "", rojo, negro, verde, par, impar o un número. Devuelve texto y si ganó (false si no apostó).
func ruleta(apuesta string, azar func(n int) int) (string, bool, error) {
	ap := llave(apuesta)
	num := -1
	switch ap {
	case "", "rojo", "negro", "verde", "par", "impar":
	default:
		v, err := strconv.Atoi(ap)
		if err != nil || v < 0 || v > 36 {
			return "", false, fmt.Errorf("apuesta: rojo, negro, verde, par, impar o un número del 0 al 36")
		}
		num = v
	}
	n := azar(37)
	col := colorRuleta(n)
	emo := map[string]string{"rojo": "🔴", "negro": "⚫", "verde": "🟢"}[col]
	txt := fmt.Sprintf("🎡 La bola cayó en %s %d (%s)", emo, n, col)
	if ap == "" {
		return txt, false, nil
	}
	gana := false
	switch ap {
	case "rojo", "negro", "verde":
		gana = col == ap
	case "par":
		gana = n != 0 && n%2 == 0
	case "impar":
		gana = n%2 == 1
	default:
		gana = n == num
	}
	if gana {
		return txt + "\n¡Ganaste tu apuesta! 🎉", true, nil
	}
	return txt + "\nEsta vez no. 😅", false, nil
}
