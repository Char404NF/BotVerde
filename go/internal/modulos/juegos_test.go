package modulos

import (
	"strings"
	"testing"
	"time"

	"botchat/internal/comandos"
)

func juegosLista(t *testing.T) *ent {
	e, _ := entH(t)
	e.conChat(pnAna, false, "Ana")
	e.conChat(pnLuis, false, "Luis")
	e.conChat(grupo, true, "Vecinos")
	e.activa(IDJuegos)
	e.fija("tope_hora", "500")
	e.fija("tope_dia", "500")
	e.config(IDJuegos, map[string]any{"espera_s": 0.0})
	return e
}

func cmdJ(t *testing.T, e *ent, nombre string) comandos.Comando {
	t.Helper()
	m, _ := Obtener(IDJuegos)
	for _, c := range m.(ComandosDeGestor).ComandosG(e.g) {
		if c.Nombre == nombre {
			return c
		}
	}
	t.Fatalf("no hay %s", nombre)
	return comandos.Comando{}
}

func juega(t *testing.T, e *ent, nombre, chat, persona, resto string) (string, error) {
	t.Helper()
	return cmdJ(t, e, nombre).Ejecutar(&comandos.Contexto{Chat: chat, Persona: persona, Resto: resto, Args: strings.Fields(resto), D: e.cmd})
}

func dice(e *ent, chat, persona, texto string, extra func(*Evento)) {
	ev := Evento{Tipo: EvMensaje, Chat: chat, EsGrupo: strings.HasSuffix(chat, "@g.us"), Persona: persona, JID: persona, Texto: texto, MensajeID: "m" + texto}
	if extra != nil {
		extra(&ev)
	}
	e.g.Evento(ev)
	e.simula(1)
}

func TestJuegosNaceApagadoYSinPantallaPropia(t *testing.T) {
	e, _ := entH(t)
	i, _ := e.g.InfoDe(IDJuegos)
	if i.Activo || !i.Iniciativa || i.Riesgo != RiesgoBajo || len(i.Paneles) == 0 {
		t.Fatalf("info: %+v", i)
	}
}

func TestJuegosRapidosDadoPptRuletaYMarcador(t *testing.T) {
	e := juegosLista(t)
	r, err := juega(t, e, "dado", pnAna, pnAna, "2d6")
	if err != nil || !strings.Contains(r, "total 2") { // el azar de prueba da siempre 0 → cada dado saca 1
		t.Fatalf("dado: %q %v", r, err)
	}
	_, err = juega(t, e, "dado", pnAna, pnAna, "0d6")
	esperaErr(t, err, "usa por ejemplo")
	r, err = juega(t, e, "ppt", pnAna, pnAna, "tijera") // el bot saca piedra (índice 0) y gana
	if err != nil || !strings.Contains(r, "Gané yo") {
		t.Fatalf("ppt: %q %v", r, err)
	}
	r, _ = juega(t, e, "ppt", pnAna, pnAna, "papel") // papel gana a piedra
	if !strings.Contains(r, "¡Ganaste!") || !strings.Contains(r, "+1 puntos") {
		t.Fatalf("ppt gana: %q", r)
	}
	_, err = juega(t, e, "ppt", pnAna, pnAna, "lagarto")
	esperaErr(t, err, "elige")
	r, _ = juega(t, e, "ruleta", pnAna, pnAna, "verde") // el azar da 0 → verde
	if !strings.Contains(r, "verde") || !strings.Contains(r, "Ganaste") {
		t.Fatalf("ruleta: %q", r)
	}
	_, err = juega(t, e, "ruleta", pnAna, pnAna, "99")
	esperaErr(t, err, "apuesta")
	m, _ := juega(t, e, "marcador", pnAna, pnAna, "")
	if !strings.Contains(m, "Ana — 2") {
		t.Fatalf("marcador: %q", m)
	}
	e.config(IDJuegos, map[string]any{"ppt": false})
	_, err = juega(t, e, "ppt", pnAna, pnAna, "papel")
	esperaErr(t, err, "apagado")
}

func TestJuegosEnfriamientoPorPersona(t *testing.T) {
	e := juegosLista(t)
	e.config(IDJuegos, map[string]any{"espera_s": 5.0})
	if _, err := juega(t, e, "dado", pnAna, pnAna, ""); err != nil {
		t.Fatal(err)
	}
	_, err := juega(t, e, "dado", pnAna, pnAna, "")
	esperaErr(t, err, "espera unos segundos")
	if _, err := juega(t, e, "dado", pnAna, pnLuis, ""); err != nil {
		t.Fatal("otra persona no espera")
	}
}

func TestJuegosPorTurnosNecesitanIniciativaPropia(t *testing.T) {
	e := juegosLista(t)
	e.fija("maestro", "0")
	_, err := juega(t, e, "adivina", pnAna, pnAna, "")
	esperaErr(t, err, "Iniciativa propia")
	e.fija("maestro", "1")
	e.g.ModoSeguro("errores")
	_, err = juega(t, e, "trivia", pnAna, pnAna, "")
	esperaErr(t, err, "Iniciativa propia")
}

func TestJuegosAdivinaEnGrupoCitaMencionaEIgnoraLoDemas(t *testing.T) {
	e := juegosLista(t)
	r, err := juega(t, e, "adivina", grupo, pnAna, "")
	if err != nil || !strings.Contains(r, "entre 1 y 100") {
		t.Fatalf("inicio: %q %v", r, err)
	}
	_, err = juega(t, e, "adivina", grupo, pnAna, "")
	esperaErr(t, err, "ya tienes una partida")
	// otra persona sin partida: se ignora; la misma persona escribiendo charla: se ignora
	dice(e, grupo, pnLuis, "50", nil)
	dice(e, grupo, pnAna, "hola a todos", nil)
	if e.cuantos() != 0 {
		t.Fatalf("no debe contestar: %v", e.enviados())
	}
	// charla citando al bot: "no entendí"
	dice(e, grupo, pnAna, "hola", func(ev *Evento) { ev.CitaEsMia = true })
	if !e.hay("No entendí esa jugada") {
		t.Fatalf("enviados: %v", e.enviados())
	}
	// jugada válida: el objetivo de prueba es 1, así que 50 es «más bajo»
	dice(e, grupo, pnAna, "50", nil)
	n := e.cuantos()
	if !e.hay("@5215511111111 ⬇️ Más bajo") {
		t.Fatalf("enviados: %v", e.enviados())
	}
	if e.env.citas[n-1] != "m50" || len(e.env.menc[n-1]) != 1 || e.env.menc[n-1][0] != pnAna {
		t.Fatalf("debe citar su mensaje y mencionarla: %v %v", e.env.citas, e.env.menc)
	}
	dice(e, grupo, pnAna, "1", nil)
	if !e.hay("¡Correcto!") || !e.hay("+5 puntos") {
		t.Fatalf("enviados: %v", e.enviados())
	}
	// terminada la partida, ya no responde
	c := e.cuantos()
	dice(e, grupo, pnAna, "1", nil)
	if e.cuantos() != c {
		t.Fatal("sin partida no contesta")
	}
	if m := e.g.marcadorTexto(grupo); !strings.Contains(m, "Ana — 5") {
		t.Fatalf("marcador: %q", m)
	}
}

func TestJuegosAdivinaSeAcabanLosIntentos(t *testing.T) {
	e := juegosLista(t)
	e.config(IDJuegos, map[string]any{"intentos_adivina": 2.0})
	_, _ = juega(t, e, "adivina", pnAna, pnAna, "")
	dice(e, pnAna, pnAna, "10", nil)
	dice(e, pnAna, pnAna, "11", nil)
	if !e.hay("Se acabaron los intentos") || !e.hay("Era el 1") {
		t.Fatalf("enviados: %v", e.enviados())
	}
}

func TestJuegosPartidaExpiraEnSilencio(t *testing.T) {
	e := juegosLista(t)
	_, _ = juega(t, e, "adivina", pnAna, pnAna, "")
	e.simula(11)
	dice(e, pnAna, pnAna, "5", nil)
	if e.cuantos() != 0 {
		t.Fatalf("expiró en silencio: %v", e.enviados())
	}
	if _, err := juega(t, e, "adivina", pnAna, pnAna, ""); err != nil {
		t.Fatal("puede empezar otra")
	}
}

func TestJuegosFilaDeEsperaConTopeDeSesiones(t *testing.T) {
	e := juegosLista(t)
	e.config(IDJuegos, map[string]any{"max_sesiones": 1.0})
	if _, err := juega(t, e, "adivina", pnAna, pnAna, ""); err != nil {
		t.Fatal(err)
	}
	r, err := juega(t, e, "trivia", pnLuis, pnLuis, "")
	if err != nil || !strings.Contains(r, "fila (lugar 1)") {
		t.Fatalf("fila: %q %v", r, err)
	}
	_, err = juega(t, e, "trivia", pnLuis, pnLuis, "")
	esperaErr(t, err, "ya estás en la fila")
	// Ana termina y a Luis le toca
	if _, err := juega(t, e, "rendirse", pnAna, pnAna, ""); err != nil {
		t.Fatal(err)
	}
	e.simula(1)
	if !e.hay(pnLuis + "|Es tu turno. 🧠") {
		t.Fatalf("enviados: %v", e.enviados())
	}
}

func TestJuegosFilaVenceAlPasar5Minutos(t *testing.T) {
	e := juegosLista(t)
	e.config(IDJuegos, map[string]any{"max_sesiones": 1.0, "minutos_sesion": 60.0})
	_, _ = juega(t, e, "adivina", pnAna, pnAna, "")
	_, _ = juega(t, e, "adivina", pnLuis, pnLuis, "")
	e.simula(6)
	_, _ = juega(t, e, "rendirse", pnAna, pnAna, "")
	e.simula(1)
	if e.cuantos() != 0 {
		t.Fatalf("la espera ya venció: %v", e.enviados())
	}
}

func TestJuegosTriviaYBancosPropios(t *testing.T) {
	e := juegosLista(t)
	r, err := juega(t, e, "trivia", pnAna, pnAna, "")
	if err != nil || !strings.Contains(r, "A) ") || !strings.Contains(r, "D) ") {
		t.Fatalf("trivia: %q %v", r, err)
	}
	e.g.h.mu.Lock()
	s := e.g.h.juegos.ses[claveSes(pnAna, pnAna)]
	letra := string(rune('a' + s.correcta))
	e.g.h.mu.Unlock()
	dice(e, pnAna, pnAna, letra, nil)
	if !e.hay("¡Correcto!") || !e.hay("+2 puntos") {
		t.Fatalf("enviados: %v", e.enviados())
	}
	// banco propio
	_, err = e.g.AccionModulo(IDJuegos, "preguntas.agregar", Args{"banco": "Historia", "pregunta": "¿En qué año llegó Colón a América?", "correcta": "1492", "incorrectas": "1500\n1453"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.g.AccionModulo(IDJuegos, "preguntas.agregar", Args{"banco": "x", "pregunta": "¿Pregunta sin respuestas?", "correcta": "1", "incorrectas": ""})
	esperaErr(t, err, "incorrectas")
	_, err = e.g.AccionModulo(IDJuegos, "preguntas.agregar", Args{"banco": "x", "pregunta": "¿Igual a la correcta?", "correcta": "Sí", "incorrectas": "si"})
	esperaErr(t, err, "igual")
	r, err = juega(t, e, "trivia", pnLuis, pnLuis, "historia")
	if err != nil || !strings.Contains(r, "Colón") {
		t.Fatalf("banco propio: %q %v", r, err)
	}
	_, err = juega(t, e, "trivia", pnAna, pnAna, "ciencia")
	esperaErr(t, err, "Bancos: general, historia")
	fs, _ := e.g.FilasPanel(IDJuegos, "preguntas")
	if len(fs) != 1 || !strings.Contains(fs[0].Detalle, "historia") {
		t.Fatalf("filas: %v", fs)
	}
	// la respuesta mala termina la partida sin puntos
	dice(e, pnLuis, pnLuis, "zzz", nil) // ni letra ni opción: se ignora (sin citar)
	e.g.h.mu.Lock()
	s = e.g.h.juegos.ses[claveSes(pnLuis, pnLuis)]
	mala := string(rune('a' + (s.correcta+1)%len(s.ops)))
	e.g.h.mu.Unlock()
	dice(e, pnLuis, pnLuis, mala, nil)
	if !e.hay("❌ No. La respuesta era 1492") {
		t.Fatalf("enviados: %v", e.enviados())
	}
	if _, err := e.g.AccionModulo(IDJuegos, "preguntas.borrar", Args{"id": fs[0].ID}); err != nil {
		t.Fatal(err)
	}
}

func TestJuegosTresEnRaya(t *testing.T) {
	e := juegosLista(t)
	r, _ := juega(t, e, "tresenraya", pnAna, pnAna, "")
	if !strings.Contains(r, "1️⃣2️⃣3️⃣") {
		t.Fatalf("tablero: %q", r)
	}
	dice(e, pnAna, pnAna, "5", nil) // centro; el bot toma una esquina (la primera: azar 0 → casilla 1)
	if !e.hay("⭕") {
		t.Fatalf("enviados: %v", e.enviados())
	}
	dice(e, pnAna, pnAna, "1", nil)
	if !e.hay("ya está ocupada") {
		t.Fatalf("enviados: %v", e.enviados())
	}
	// victoria de la persona con 4-5-6: el bot bloquea cuando puede, así que probamos la lógica directa
	var tab [9]int8
	tab[0], tab[1] = 1, 1
	if j := jugadaBotTTT(tab, func(int) int { return 0 }); j != 2 {
		t.Fatalf("el bot debe bloquear en 3, jugó %d", j)
	}
	tab = [9]int8{2, 2, 0, 1, 1, 0, 0, 0, 0}
	if j := jugadaBotTTT(tab, func(int) int { return 0 }); j != 2 {
		t.Fatalf("el bot debe ganar en 3, jugó %d", j)
	}
	tab = [9]int8{1, 1, 1, 0, 2, 2, 0, 0, 0}
	if ganadorTTT(tab) != 1 {
		t.Fatal("ganador")
	}
	// juego completo: la persona gana con puntos si el bot no bloquea (partida armada a mano)
	e.g.h.mu.Lock()
	s := e.g.h.juegos.ses[claveSes(pnAna, pnAna)]
	s.tab = [9]int8{1, 1, 0, 2, 2, 0, 0, 0, 0}
	e.g.h.mu.Unlock()
	dice(e, pnAna, pnAna, "3", nil)
	if !e.hay("¡Ganaste!") || !e.hay("+3 puntos") {
		t.Fatalf("enviados: %v", e.enviados())
	}
}

func TestJuegosRendirseYTopeDeMensajesPorMinutoEnGrupo(t *testing.T) {
	e := juegosLista(t)
	_, err := juega(t, e, "rendirse", pnAna, pnAna, "")
	esperaErr(t, err, "no tienes una partida")
	e.config(IDJuegos, map[string]any{"msgs_min_grupo": 1.0, "intentos_adivina": 30.0})
	_, _ = juega(t, e, "adivina", grupo, pnAna, "")
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: grupo, EsGrupo: true, Persona: pnAna, JID: pnAna, Texto: "50", MensajeID: "a"})
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: grupo, EsGrupo: true, Persona: pnAna, JID: pnAna, Texto: "40", MensajeID: "b"})
	e.simula(0)
	e.corre()
	e.avanza(10 * time.Second)
	e.corre()
	if e.cuantos() != 1 {
		t.Fatalf("el tope por minuto frena el segundo: %v", e.enviados())
	}
	if _, err := juega(t, e, "rendirse", grupo, pnAna, ""); err != nil {
		t.Fatal(err)
	}
}

func TestJuegosNoMolestarYAyuda(t *testing.T) {
	e := juegosLista(t)
	r, err := juega(t, e, "juegos", pnAna, pnAna, "")
	if err != nil || !strings.Contains(r, "!trivia") || !strings.Contains(r, "!marcador") {
		t.Fatalf("ayuda: %q %v", r, err)
	}
	_, _ = juega(t, e, "adivina", pnAna, pnAna, "")
	_ = e.g.AgregarNoMolestar(pnAna, "baja")
	dice(e, pnAna, pnAna, "5", nil)
	if e.cuantos() != 0 {
		t.Fatal("No molestar: nada sale")
	}
}

func TestJuegosOlvidarPersonaBorraSusPuntos(t *testing.T) {
	e := juegosLista(t)
	e.g.sumarPuntos(grupo, pnAna, 4)
	e.g.sumarPuntos(grupo, pnLuis, 2)
	if _, err := e.alm.OlvidarPersona(1, pnAna); err != nil {
		t.Fatal(err)
	}
	m := e.g.marcadorTexto(grupo)
	if strings.Contains(m, "Ana") || !strings.Contains(m, "2") {
		t.Fatalf("marcador: %q", m)
	}
}
