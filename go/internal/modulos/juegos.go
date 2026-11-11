package modulos

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"botchat/internal/comandos"
	"botchat/internal/permisos"
)

// ---------- H9 · Juegos sencillos por chat ----------
//
// Dados, piedra-papel-tijera, ruleta (una sola jugada, responden al comando), y adivina el número, tres en raya y trivia (por
// turnos, con sesión por persona y grupo). Las reglas son las de los menús en grupos: la sesión arranca solo con el comando con
// prefijo; en el grupo cada respuesta cita el mensaje exacto de la persona y la menciona; un mensaje sin citar solo cuenta si esa
// persona tiene partida y es una jugada válida (si no, se ignora); una partida que nadie contesta expira en silencio; hay tope de
// partidas a la vez (con fila de espera) y tope de mensajes por minuto por grupo. Las respuestas de las partidas por turnos pasan por
// la cola de iniciativas. El marcador y las preguntas se guardan en mod_datos (cifrado).

// IDJuegos es el id del módulo.
const IDJuegos = "juegos"

type sesionJuego struct {
	juego, chat, persona string
	ultimo               time.Time
	objetivo, intentos   int
	maxInt, rango        int
	tab                  [9]int8
	preg                 PreguntaTrivia
	ops                  []string
	correcta             int
}

type esperaJuego struct {
	juego, chat, persona, arg string
	hasta                     time.Time
}

type estadoJuegos struct {
	ses    map[string]*sesionJuego
	espera []esperaJuego
}

func claveSes(chat, persona string) string { return chat + "|" + persona }

var nombresJuego = map[string]string{"dado": "Dados", "ppt": "Piedra, papel o tijera", "ruleta": "Ruleta", "adivina": "Adivina el número", "tresenraya": "Tres en raya", "trivia": "Trivia"}

func juegos() Modulo {
	return Nuevo(Def{
		ID: IDJuegos, Nombre: "Juegos", Icono: "juego", Riesgo: RiesgoBajo, Iniciativa: true,
		Descripcion: "Juegos sencillos por chat: dados, piedra-papel-tijera, ruleta, adivina el número, tres en raya y trivia (con bancos de preguntas que tú editas), con marcador. Las partidas por turnos responden por la cola de Iniciativa propia, así que necesitan ese interruptor encendido.",
		Casillas: []permisos.Casilla{
			{ID: "juegos.jugar", Nombre: "Jugar", Descripcion: "Puede jugar con el bot (dados, trivia, tres en raya…).", Grupo: "Juegos", Riesgo: permisos.RiesgoBajo},
		},
		Esquema: []Campo{
			{Clave: "dado", Titulo: "Dados (!dado)", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "ppt", Titulo: "Piedra, papel o tijera (!ppt)", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "ruleta", Titulo: "Ruleta (!ruleta)", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "adivina", Titulo: "Adivina el número (!adivina)", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "tresenraya", Titulo: "Tres en raya (!tresenraya)", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "trivia", Titulo: "Trivia (!trivia)", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "puntos_ppt", Titulo: "Puntos por ganar piedra-papel-tijera", Tipo: CampoNumero, Defecto: 1.0, Min: 0, Max: 100},
			{Clave: "puntos_ruleta", Titulo: "Puntos por ganar una apuesta de ruleta", Tipo: CampoNumero, Defecto: 1.0, Min: 0, Max: 100},
			{Clave: "puntos_adivina", Titulo: "Puntos por adivinar el número", Tipo: CampoNumero, Defecto: 5.0, Min: 0, Max: 100},
			{Clave: "puntos_tresenraya", Titulo: "Puntos por ganar tres en raya", Tipo: CampoNumero, Defecto: 3.0, Min: 0, Max: 100},
			{Clave: "puntos_trivia", Titulo: "Puntos por acertar la trivia", Tipo: CampoNumero, Defecto: 2.0, Min: 0, Max: 100},
			{Clave: "rango_adivina", Titulo: "Adivina · número más alto", Tipo: CampoNumero, Defecto: 100.0, Min: 10, Max: 10000},
			{Clave: "intentos_adivina", Titulo: "Adivina · intentos", Tipo: CampoNumero, Defecto: 7.0, Min: 1, Max: 30},
			{Clave: "max_sesiones", Titulo: "Partidas por turnos a la vez", Ayuda: "Si hay más, quedan en una fila de espera.", Tipo: CampoNumero, Defecto: 5.0, Min: 1, Max: 50},
			{Clave: "minutos_sesion", Titulo: "Minutos sin jugar para que expire la partida", Ayuda: "Expira en silencio.", Tipo: CampoNumero, Defecto: 10.0, Min: 1, Max: 120},
			{Clave: "msgs_min_grupo", Titulo: "Máximo de mensajes por minuto en un grupo", Tipo: CampoNumero, Defecto: 10.0, Min: 1, Max: 120},
			{Clave: "espera_s", Titulo: "Espera entre jugadas de una persona (segundos)", Tipo: CampoNumero, Defecto: 3.0, Min: 0, Max: 120},
		},
		TopeHora: 60, TopeDia: 400,
		ComandosG:    comandosJuegos,
		AlEvento:     eventoJuegos,
		AlDesactivar: func(g *Gestor) error { g.h.mu.Lock(); g.h.juegos = estadoJuegos{}; g.h.mu.Unlock(); return nil },
		AlEstado: func(g *Gestor) string {
			g.h.mu.Lock()
			n, f := len(g.h.juegos.ses), len(g.h.juegos.espera)
			g.h.mu.Unlock()
			if n+f == 0 {
				return ""
			}
			return fmt.Sprintf("Encendido · %d partidas en curso, %d en fila", n, f)
		},
		Paneles: func(g *Gestor) []Panel {
			return []Panel{{
				ID: "preguntas", Titulo: "Bancos de preguntas de trivia",
				Ayuda: "Además del banco «general» que trae el módulo, puedes crear bancos propios (el nombre del banco va después de !trivia). Una respuesta correcta y de 1 a 3 incorrectas (una por línea).",
				Campos: []Campo{
					{Clave: "banco", Titulo: "Banco", Ayuda: "Por ejemplo general, historia, deportes.", Tipo: CampoTexto, Largo: 24, Defecto: bancoGeneral, Obligatorio: true},
					{Clave: "pregunta", Titulo: "Pregunta", Tipo: CampoTexto, Largo: 300, Varias: true, Obligatorio: true},
					{Clave: "correcta", Titulo: "Respuesta correcta", Tipo: CampoTexto, Largo: 100, Obligatorio: true},
					{Clave: "incorrectas", Titulo: "Respuestas incorrectas (una por línea)", Tipo: CampoTexto, Largo: 320, Varias: true, Obligatorio: true},
				},
				Alta:      "Agregar pregunta",
				PorFila:   []Boton{{Accion: "preguntas.borrar", Titulo: "Borrar", Confirma: "¿Borrar esta pregunta?"}},
				Generales: []Boton{{Accion: "marcador.reiniciar", Titulo: "Reiniciar todos los marcadores", Confirma: "Se borran los puntos de todos. ¿Seguir?"}},
			}}
		},
		AlFilas: func(g *Gestor, panel string) []Fila {
			var out []Fila
			for _, d := range g.Alm.ListarDatosMod(g.BotID, IDJuegos, "pregunta", false, 500) {
				var p PreguntaTrivia
				if json.Unmarshal([]byte(d.Valor), &p) != nil {
					continue
				}
				banco, _, _ := strings.Cut(d.Clave, "|")
				out = append(out, Fila{ID: d.Clave, Titulo: corta(p.P, 80), Detalle: "Banco " + banco + " · respuesta: " + corta(p.C, 40)})
			}
			return out
		},
		AlAccion: accionJuegos,
	})
}

func accionJuegos(g *Gestor, accion string, a Args) (string, error) {
	switch accion {
	case "preguntas.agregar":
		banco := strings.ReplaceAll(llave(a.Texto("banco")), " ", "_")
		if banco == "" || len(banco) > 24 {
			return "", errors.New("el nombre del banco debe tener entre 1 y 24 letras")
		}
		p := PreguntaTrivia{P: a.Texto("pregunta"), C: a.Texto("correcta")}
		for _, l := range strings.Split(valorTexto(a, "incorrectas"), "\n") {
			if l = strings.TrimSpace(l); l != "" {
				p.I = append(p.I, l)
			}
		}
		if err := p.valida(); err != nil {
			return "", err
		}
		clave := banco + "|" + strconv.FormatInt(g.ahora().UnixNano(), 36)
		if err := g.Alm.GuardarDatoMod(g.BotID, IDJuegos, "pregunta", clave, 0, p.json(), g.ahora()); err != nil {
			return "", err
		}
		return "Pregunta agregada al banco «" + banco + "».", nil
	case "preguntas.borrar":
		return "Pregunta borrada.", g.Alm.BorrarDatoMod(g.BotID, IDJuegos, "pregunta", a.Texto("id"))
	case "marcador.reiniciar":
		return "Marcadores reiniciados.", g.Alm.BorrarColeccionMod(g.BotID, IDJuegos, "puntos")
	}
	return "", errors.New("esa acción no existe")
}

func (g *Gestor) azar(n int) int {
	if n <= 1 {
		return 0
	}
	return int(g.Aleatorio(int64(n)))
}

// bancosTrivia junta el banco general con los del dueño.
func (g *Gestor) bancosTrivia() map[string][]PreguntaTrivia {
	b := map[string][]PreguntaTrivia{bancoGeneral: append([]PreguntaTrivia(nil), preguntasGeneral...)}
	for _, d := range g.Alm.ListarDatosMod(g.BotID, IDJuegos, "pregunta", false, 2000) {
		var p PreguntaTrivia
		if json.Unmarshal([]byte(d.Valor), &p) != nil || p.valida() != nil {
			continue
		}
		banco, _, _ := strings.Cut(d.Clave, "|")
		b[banco] = append(b[banco], p)
	}
	return b
}

func (g *Gestor) sumarPuntos(chat, persona string, n int) {
	if n > 0 {
		_, _ = g.Alm.SumarDatoMod(g.BotID, IDJuegos, "puntos", claveSes(chat, persona), int64(n), g.ahora())
	}
}

func (g *Gestor) marcadorTexto(chat string) string {
	var filas []string
	for _, d := range g.Alm.ListarDatosMod(g.BotID, IDJuegos, "puntos", true, 5000) {
		c, p, _ := strings.Cut(d.Clave, "|")
		if c != chat || d.Num <= 0 {
			continue
		}
		n := g.nombreDe(p)
		if n == "" {
			n = enmascarar(p)
		}
		filas = append(filas, fmt.Sprintf("%s — %d", n, d.Num))
		if len(filas) == 10 {
			break
		}
	}
	if len(filas) == 0 {
		return "Todavía no hay puntos en este chat. ¡Juega con !trivia, !adivina o !tresenraya!"
	}
	medallas := []string{"🥇", "🥈", "🥉"}
	for i := range filas {
		pre := fmt.Sprintf("%d.", i+1)
		if i < 3 {
			pre = medallas[i]
		}
		filas[i] = pre + " " + filas[i]
	}
	return "🏆 Marcador:\n" + strings.Join(filas, "\n")
}

// ---------- partidas por turnos ----------

// juegoNuevo arma una partida y su primer mensaje (no la guarda).
func (g *Gestor) juegoNuevo(cfg Config, juego, chat, persona, arg string) (*sesionJuego, string, error) {
	s := &sesionJuego{juego: juego, chat: chat, persona: persona, ultimo: g.ahora()}
	switch juego {
	case "adivina":
		s.rango, s.maxInt = cfg.Int("rango_adivina"), cfg.Int("intentos_adivina")
		s.objetivo = 1 + g.azar(s.rango)
		return s, fmt.Sprintf("🔢 Adivina el número: pienso uno entre 1 y %d. Tienes %d intentos. Responde con un número. (!rendirse para salir)", s.rango, s.maxInt), nil
	case "tresenraya":
		return s, "⭕❌ Tres en raya: tú eres ❌ y empiezas. Responde con el número de la casilla (1 a 9). (!rendirse para salir)\n" + tableroTexto(s.tab), nil
	case "trivia":
		bancos := g.bancosTrivia()
		banco := strings.ReplaceAll(llave(arg), " ", "_")
		if banco == "" {
			banco = bancoGeneral
		}
		qs, ok := bancos[banco]
		if !ok {
			var ns []string
			for n := range bancos {
				ns = append(ns, n)
			}
			sort.Strings(ns)
			return nil, "", fmt.Errorf("no existe ese banco. Bancos: %s", strings.Join(ns, ", "))
		}
		s.preg = qs[g.azar(len(qs))]
		s.ops, s.correcta = opcionesTrivia(s.preg, g.azar)
		return s, preguntaTexto(s.preg, s.ops), nil
	}
	return nil, "", errors.New("ese juego no existe")
}

var errNecesitaIniciativa = errors.New("las partidas por turnos responden por la cola de «Iniciativa propia»: pídele a quien administra el bot que la encienda")

// juegoEmpezar abre la partida, o deja a la persona en la fila si ya hay demasiadas.
func (g *Gestor) juegoEmpezar(juego, chat, persona, arg string) (string, error) {
	cfg := g.Config(IDJuegos)
	if !cfg.Bool(juego) {
		return "", errors.New("ese juego está apagado")
	}
	if !g.Maestro() || g.EnModoSeguro() {
		return "", errNecesitaIniciativa
	}
	g.juegoMantener()
	k := claveSes(chat, persona)
	g.h.mu.Lock()
	if g.h.juegos.ses == nil {
		g.h.juegos.ses = map[string]*sesionJuego{}
	}
	if s := g.h.juegos.ses[k]; s != nil {
		g.h.mu.Unlock()
		return "", fmt.Errorf("ya tienes una partida de %s en este chat; escribe !rendirse para terminarla", nombresJuego[s.juego])
	}
	for i, w := range g.h.juegos.espera {
		if claveSes(w.chat, w.persona) == k {
			g.h.mu.Unlock()
			return "", fmt.Errorf("ya estás en la fila (lugar %d); te escribo cuando sea tu turno", i+1)
		}
	}
	if len(g.h.juegos.ses) >= cfg.Int("max_sesiones") {
		g.h.juegos.espera = append(g.h.juegos.espera, esperaJuego{juego, chat, persona, arg, g.ahora().Add(5 * time.Minute)})
		n := len(g.h.juegos.espera)
		g.h.mu.Unlock()
		return fmt.Sprintf("Hay %d partidas en curso. Quedaste en la fila (lugar %d) y te escribo cuando sea tu turno; si pasan 5 minutos se cancela.", cfg.Int("max_sesiones"), n), nil
	}
	g.h.mu.Unlock()
	s, txt, err := g.juegoNuevo(cfg, juego, chat, persona, arg)
	if err != nil {
		return "", err
	}
	g.h.mu.Lock()
	g.h.juegos.ses[k] = s
	g.h.mu.Unlock()
	return txt, nil
}

// juegoMantener expira en silencio las partidas sin actividad y las esperas vencidas, y da su turno a quien sigue en la fila.
func (g *Gestor) juegoMantener() {
	cfg := g.Config(IDJuegos)
	ahora := g.ahora()
	lim := time.Duration(cfg.Int("minutos_sesion")) * time.Minute
	g.h.mu.Lock()
	for k, s := range g.h.juegos.ses {
		if ahora.Sub(s.ultimo) > lim {
			delete(g.h.juegos.ses, k)
		}
	}
	var vivas []esperaJuego
	for _, w := range g.h.juegos.espera {
		if w.hasta.After(ahora) {
			vivas = append(vivas, w)
		}
	}
	g.h.juegos.espera = vivas
	g.h.mu.Unlock()
	for {
		g.h.mu.Lock()
		if g.h.juegos.ses == nil {
			g.h.juegos.ses = map[string]*sesionJuego{}
		}
		if len(g.h.juegos.espera) == 0 || len(g.h.juegos.ses) >= cfg.Int("max_sesiones") {
			g.h.mu.Unlock()
			return
		}
		w := g.h.juegos.espera[0]
		g.h.juegos.espera = g.h.juegos.espera[1:]
		g.h.mu.Unlock()
		s, txt, err := g.juegoNuevo(cfg, w.juego, w.chat, w.persona, w.arg)
		if err != nil {
			continue
		}
		g.h.mu.Lock()
		g.h.juegos.ses[claveSes(w.chat, w.persona)] = s
		g.h.mu.Unlock()
		g.juegoHablar(w.chat, w.persona, w.persona, "", "juego_turno", "Es tu turno. "+txt)
	}
}

// juegoHablar manda un mensaje de juego por la cola (en grupos cita el mensaje de la persona y la menciona).
func (g *Gestor) juegoHablar(chat, persona, jid, citaID, que, texto string) {
	var menc []string
	if strings.HasSuffix(chat, "@g.us") {
		if jid == "" {
			jid = persona
		}
		menc = []string{jid}
		texto = "@" + usuarioDe(jid) + " " + texto
	}
	_, _ = g.Enviar(Iniciativa{Modulo: IDJuegos, Que: que, Chat: chat, Personas: []string{persona}, Texto: texto, CitaID: citaID, Menciones: menc,
		Noche: NocheDescartar, RetrasoMin: time.Second, RetrasoMax: 3 * time.Second, EsperaMax: 2 * time.Minute})
}

func eventoJuegos(g *Gestor, e Evento) {
	switch e.Tipo {
	case EvTick:
		g.juegoMantener()
	case EvMensaje:
		g.juegoMensaje(e)
	}
}

func (g *Gestor) juegoMensaje(e Evento) {
	if e.EsMio || strings.TrimSpace(e.Texto) == "" || e.Chat == "" {
		return
	}
	if g.Cmd != nil && g.Cmd.EsComando(e.Texto) {
		return
	}
	persona := g.Alm.ResolverPersona(g.BotID, e.Persona)
	k := claveSes(e.Chat, persona)
	cfg := g.Config(IDJuegos)
	g.h.mu.Lock()
	s := g.h.juegos.ses[k]
	if s == nil {
		g.h.mu.Unlock()
		return
	}
	if g.ahora().Sub(s.ultimo) > time.Duration(cfg.Int("minutos_sesion"))*time.Minute {
		delete(g.h.juegos.ses, k)
		g.h.mu.Unlock()
		return
	}
	resp, valido, fin, puntos := g.sesionResponde(s, cfg, e.Texto)
	if !valido && !e.CitaEsMia {
		g.h.mu.Unlock() // sin citar y no es una jugada: se ignora
		return
	}
	if !valido {
		resp = "No entendí esa jugada. " + ayudaJuego(s)
	}
	if valido {
		s.ultimo = g.ahora()
	}
	if fin {
		delete(g.h.juegos.ses, k)
	}
	g.h.mu.Unlock()
	if e.EsGrupo && !g.limite("juegos_grupo:"+e.Chat, cfg.Int("msgs_min_grupo"), time.Minute) {
		g.Alm.RegistrarModulo(g.BotID, IDJuegos, "juego", ResDescartado+MotTope, e.Chat, g.ahora())
		return
	}
	if puntos > 0 {
		g.sumarPuntos(e.Chat, persona, puntos)
		resp += fmt.Sprintf("\n+%d puntos", puntos)
	}
	g.juegoHablar(e.Chat, persona, e.JID, e.MensajeID, "juego", resp)
	if fin {
		g.juegoMantener()
	}
}

func ayudaJuego(s *sesionJuego) string {
	switch s.juego {
	case "adivina":
		return "Escribe un número."
	case "tresenraya":
		return "Escribe el número de una casilla (1 a 9)."
	}
	return "Responde con la letra (A, B…) o el número."
}

// sesionResponde aplica un mensaje a la partida. Se llama con el candado tomado. Devuelve la respuesta, si fue una jugada válida,
// si la partida terminó y los puntos ganados.
func (g *Gestor) sesionResponde(s *sesionJuego, cfg Config, texto string) (resp string, valido, fin bool, puntos int) {
	switch s.juego {
	case "adivina":
		n, err := strconv.Atoi(strings.Trim(strings.TrimSpace(texto), " .!¡"))
		if err != nil {
			return "", false, false, 0
		}
		if n < 1 || n > s.rango {
			return fmt.Sprintf("Elige un número entre 1 y %d.", s.rango), true, false, 0
		}
		s.intentos++
		switch {
		case n == s.objetivo:
			return fmt.Sprintf("🎉 ¡Correcto! Era %d y lo lograste en %d intentos.", n, s.intentos), true, true, cfg.Int("puntos_adivina")
		case s.intentos >= s.maxInt:
			return fmt.Sprintf("Se acabaron los intentos 😅 Era el %d.", s.objetivo), true, true, 0
		case n < s.objetivo:
			return fmt.Sprintf("⬆️ Más alto (te quedan %d intentos)", s.maxInt-s.intentos), true, false, 0
		}
		return fmt.Sprintf("⬇️ Más bajo (te quedan %d intentos)", s.maxInt-s.intentos), true, false, 0
	case "tresenraya":
		c, err := strconv.Atoi(strings.TrimSpace(texto))
		if err != nil || c < 1 || c > 9 {
			return "", false, false, 0
		}
		if s.tab[c-1] != 0 {
			return "Esa casilla ya está ocupada. Elige otra.\n" + tableroTexto(s.tab), true, false, 0
		}
		s.tab[c-1] = 1
		if ganadorTTT(s.tab) == 1 {
			return "🎉 ¡Ganaste!\n" + tableroTexto(s.tab), true, true, cfg.Int("puntos_tresenraya")
		}
		if llenoTTT(s.tab) {
			return "🤝 Empate.\n" + tableroTexto(s.tab), true, true, 0
		}
		if j := jugadaBotTTT(s.tab, g.azar); j >= 0 {
			s.tab[j] = 2
		}
		if ganadorTTT(s.tab) == 2 {
			return "Gané yo 😎\n" + tableroTexto(s.tab), true, true, 0
		}
		if llenoTTT(s.tab) {
			return "🤝 Empate.\n" + tableroTexto(s.tab), true, true, 0
		}
		return "Tu turno:\n" + tableroTexto(s.tab), true, false, 0
	case "trivia":
		i := respuestaTrivia(texto, s.ops)
		if i < 0 {
			return "", false, false, 0
		}
		if i == s.correcta {
			return "✅ ¡Correcto! Era " + s.preg.C + ".", true, true, cfg.Int("puntos_trivia")
		}
		return "❌ No. La respuesta era " + s.preg.C + ".", true, true, 0
	}
	return "", false, true, 0
}

// ---------- comandos ----------

func comandosJuegos(g *Gestor) []comandos.Comando {
	const casilla = "juegos.jugar"
	enfria := func(cx *comandos.Contexto, cfg Config) error {
		if s := cfg.Int("espera_s"); s > 0 && !g.limite("juego_p:"+cx.Persona, 1, time.Duration(s)*time.Second) {
			return errors.New("espera unos segundos entre jugadas")
		}
		return nil
	}
	puntos := func(cx *comandos.Contexto, clave string, ganó bool) string {
		n := g.Config(IDJuegos).Int(clave)
		if !ganó || n <= 0 {
			return ""
		}
		g.sumarPuntos(cx.Chat, g.Alm.ResolverPersona(g.BotID, cx.Persona), n)
		return fmt.Sprintf("\n+%d puntos", n)
	}
	rapido := func(nombre, resumen, uso string, alias []string, f func(cx *comandos.Contexto, cfg Config) (string, error)) comandos.Comando {
		return comandos.Comando{Nombre: nombre, Alias: alias, Resumen: resumen, Uso: uso, Casilla: casilla,
			Ejecutar: func(cx *comandos.Contexto) (string, error) {
				cfg := g.Config(IDJuegos)
				if !cfg.Bool(nombre) {
					return "", errors.New("ese juego está apagado")
				}
				if err := enfria(cx, cfg); err != nil {
					return "", err
				}
				return f(cx, cfg)
			}}
	}
	turnos := func(nombre, resumen, uso string, alias []string) comandos.Comando {
		return comandos.Comando{Nombre: nombre, Alias: alias, Resumen: resumen, Uso: uso, Casilla: casilla,
			Ejecutar: func(cx *comandos.Contexto) (string, error) {
				if err := enfria(cx, g.Config(IDJuegos)); err != nil {
					return "", err
				}
				return g.juegoEmpezar(nombre, cx.Chat, g.Alm.ResolverPersona(g.BotID, cx.Persona), cx.Resto)
			}}
	}
	return []comandos.Comando{
		rapido("dado", "Tirar dados", "dado [caras | 2d6]", nil, func(cx *comandos.Contexto, _ Config) (string, error) { return tiraDados(cx.Resto, g.azar) }),
		rapido("ppt", "Piedra, papel o tijera", "ppt <piedra|papel|tijera>", []string{"piedrapapel"}, func(cx *comandos.Contexto, _ Config) (string, error) {
			t, r, err := jugadaPPT(cx.Resto, g.azar)
			if err != nil {
				return "", err
			}
			return t + puntos(cx, "puntos_ppt", r == 1), nil
		}),
		rapido("ruleta", "Girar la ruleta (puedes apostar)", "ruleta [rojo|negro|par|impar|número]", nil, func(cx *comandos.Contexto, _ Config) (string, error) {
			t, gana, err := ruleta(cx.Resto, g.azar)
			if err != nil {
				return "", err
			}
			return t + puntos(cx, "puntos_ruleta", gana), nil
		}),
		turnos("adivina", "Adivinar un número", "adivina", nil),
		turnos("tresenraya", "Tres en raya contra el bot", "tresenraya", []string{"gato", "ttt"}),
		turnos("trivia", "Pregunta de trivia", "trivia [banco]", nil),
		{Nombre: "rendirse", Resumen: "Terminar tu partida", Casilla: "juegos.jugar",
			Ejecutar: func(cx *comandos.Contexto) (string, error) {
				k := claveSes(cx.Chat, g.Alm.ResolverPersona(g.BotID, cx.Persona))
				g.h.mu.Lock()
				s := g.h.juegos.ses[k]
				delete(g.h.juegos.ses, k)
				n := len(g.h.juegos.espera)
				var resto []esperaJuego
				for _, w := range g.h.juegos.espera {
					if claveSes(w.chat, w.persona) != k {
						resto = append(resto, w)
					}
				}
				g.h.juegos.espera = resto
				g.h.mu.Unlock()
				if s == nil && len(resto) == n {
					return "", errors.New("no tienes una partida en este chat")
				}
				g.juegoMantener()
				return "Listo, terminé tu partida.", nil
			}},
		{Nombre: "marcador", Alias: []string{"puntos"}, Resumen: "Ver el marcador de este chat", Casilla: casilla,
			Ejecutar: func(cx *comandos.Contexto) (string, error) { return g.marcadorTexto(cx.Chat), nil }},
		{Nombre: "juegos", Resumen: "Ver los juegos disponibles", Casilla: casilla,
			Ejecutar: func(cx *comandos.Contexto) (string, error) {
				cfg := g.Config(IDJuegos)
				p := cx.D.Prefijo()
				var l []string
				for _, j := range []string{"dado", "ppt", "ruleta", "adivina", "tresenraya", "trivia"} {
					if cfg.Bool(j) {
						l = append(l, "• "+p+j+" — "+nombresJuego[j])
					}
				}
				l = append(l, "• "+p+"marcador — puntos de este chat", "• "+p+"rendirse — terminar tu partida")
				return "🎮 Juegos:\n" + strings.Join(l, "\n"), nil
			}},
	}
}
