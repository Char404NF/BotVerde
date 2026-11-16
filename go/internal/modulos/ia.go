package modulos

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"botchat/internal/comandos"
	"botchat/internal/permisos"
)

// ---------- H7 · IA opcional ----------
//
// Un endpoint compatible con OpenAI (URL, modelo y llave). La llave vive SOLO en el Android Keystore (la app se la pasa a Go en
// memoria): nunca en el repo, la configuración, la base ni los logs. PRIVACIDAD: lo que se le pregunta (y, si la memoria está
// encendida, lo anterior de esa conversación) SALE del teléfono hacia el proveedor elegido.
// «!ia <pregunta>» siempre responde (alguien lo pidió). Responder por mención o en chats permitidos pasa por la cola de iniciativas
// (interruptor general, modo seguro, No molestar, topes, retraso al azar). Toda respuesta lleva una etiqueta que dice que es automática.

// IDIA es el id del módulo.
const IDIA = "ia"

type turnoIA struct {
	Rol, Texto string
	TS         time.Time
}

func ia() Modulo {
	return Nuevo(Def{
		ID: IDIA, Nombre: "IA opcional", Icono: "ia", Riesgo: RiesgoMedio, Iniciativa: true,
		Descripcion: "Responde con un modelo de IA (OpenAI, Claude, Gemini, DeepSeek, Groq, Mistral, Grok, Ollama y cualquier servicio compatible con OpenAI) cuando lo pidan con «!ia», lo mencionen en un grupo o escriban en chats que tú permitas. Las respuestas siempre dicen que son automáticas. «!ia» responde siempre; mención y chats permitidos necesitan «Iniciativa propia» encendida.",
		Casillas: []permisos.Casilla{
			{ID: "ia.usar", Nombre: "Usar la IA («!ia»)", Descripcion: "Puede hacerle preguntas a la IA. Lo que escriba sale del teléfono hacia el proveedor de IA.", Grupo: "IA", Riesgo: permisos.RiesgoMedio},
		},
		Esquema: []Campo{
			{Clave: "proveedor", Titulo: "Servicio de IA", Ayuda: "Elige uno de la lista y se llenan solos la dirección y el modelo. «otro» sirve para cualquier servicio compatible con OpenAI.", Tipo: CampoTexto, Largo: 30, Defecto: "openai"},
			{Clave: "url", Titulo: "Dirección del servicio", Ayuda: "Por ejemplo https://api.openai.com/v1 (se agrega /chat/completions).", Tipo: CampoTexto, Largo: 300, Defecto: "https://api.openai.com/v1"},
			{Clave: "modelo", Titulo: "Modelo", Tipo: CampoTexto, Largo: 80, Defecto: "gpt-6-luna", Obligatorio: true},
			{Clave: "llave", Titulo: "Llave del servicio", Ayuda: "Se guarda cifrada en el Keystore de Android. Nunca en la base, el repositorio ni los registros.", Tipo: CampoSecreto},
			{Clave: "personalidad", Titulo: "Personalidad (instrucciones)", Ayuda: "Lo que la IA debe ser y hacer. Cada chat puede tener la suya en la lista de abajo.", Tipo: CampoTexto, Varias: true, Largo: 1200,
				Defecto: "Eres un asistente amable y breve. Respondes en español."},
			{Clave: "etiqueta", Titulo: "Etiqueta de respuesta automática", Ayuda: "Va al inicio de cada respuesta para que se sepa que no es una persona escribiendo. No puede quedar vacía.", Tipo: CampoTexto, Largo: 60, Defecto: "🤖 IA: ", Obligatorio: true},
			{Clave: "por_comando", Titulo: "Responder a «!ia»", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "por_mencion", Titulo: "Responder cuando me mencionan en un grupo", Ayuda: "También si citan un mensaje mío. Pasa por Iniciativa propia.", Tipo: CampoInterruptor, Defecto: false},
			{Clave: "chats_activos", Titulo: "Responder a todo en los chats permitidos", Ayuda: "Solo los chats de la lista de abajo. Pasa por Iniciativa propia.", Tipo: CampoInterruptor, Defecto: false},
			{Clave: "espera_chat_s", Titulo: "Espera mínima entre respuestas de un chat (segundos)", Tipo: CampoNumero, Defecto: 30.0, Min: 5, Max: 3600},
			{Clave: "en_menus", Titulo: "Permitir {ia_dato} en menús", Ayuda: "En un menú, {ia_dato1} responde con IA a lo que la persona escribió en el paso que guardó «dato1».", Tipo: CampoInterruptor, Defecto: false},
			{Clave: "memoria", Titulo: "Recordar la conversación", Ayuda: "Apagado por defecto. Solo en memoria del teléfono (se pierde al cerrar la app) y se manda al proveedor con cada pregunta.", Tipo: CampoInterruptor, Defecto: false},
			{Clave: "memoria_turnos", Titulo: "Cuántos intercambios recordar", Tipo: CampoNumero, Defecto: 6.0, Min: 1, Max: 20},
			{Clave: "limite_dia", Titulo: "Máximo de consultas por día", Ayuda: "Entre todos. 0 = sin límite (no recomendado: puede costar).", Tipo: CampoNumero, Defecto: 50.0, Min: 0, Max: 5000},
			{Clave: "limite_persona_hora", Titulo: "Máximo por persona y hora", Tipo: CampoNumero, Defecto: 10.0, Min: 0, Max: 500},
			{Clave: "max_entrada", Titulo: "Largo máximo de la pregunta (letras)", Tipo: CampoNumero, Defecto: 600.0, Min: 20, Max: 4000},
			{Clave: "max_respuesta", Titulo: "Largo máximo de la respuesta (letras)", Tipo: CampoNumero, Defecto: 600.0, Min: 50, Max: 3000},
			{Clave: "tiempo_s", Titulo: "Tiempo máximo (segundos)", Tipo: CampoNumero, Defecto: 40.0, Min: 5, Max: 180},
			{Clave: "red_local", Titulo: "Servicio en mi red de casa", Ayuda: "Permite direcciones privadas o locales (por ejemplo un Ollama en tu red).", Tipo: CampoInterruptor, Defecto: false},
		},
		TopeHora: 20, TopeDia: 100,
		Validar: func(c map[string]any) error {
			cfg := Config(c)
			if strings.TrimSpace(cfg.Texto("etiqueta")) == "" {
				return errors.New("la etiqueta de respuesta automática no puede quedar vacía: así se sabe que contesta una IA")
			}
			return nil
		},
		ComandosG:    comandosIA,
		AlEvento:     eventoIA,
		AlDesactivar: func(g *Gestor) error { g.h.mu.Lock(); g.h.memoria = nil; g.h.mu.Unlock(); return nil },
		AlEstado: func(g *Gestor) string {
			cfg := g.Config(IDIA)
			switch {
			case !TieneLlave(g.BotID, IDIA, "llave") && !cfg.Bool("red_local") && !sinLlaveIA(cfg):
				return "Encendido, falta la llave del servicio"
			}
			nombre := cfg.Texto("modelo")
			if p, ok := proveedorIA(cfg.Texto("proveedor")); ok {
				nombre = p.Nombre + " · " + nombre
			}
			return fmt.Sprintf("Encendido · %s · %d consultas hoy", nombre, g.consultasHoy())
		},
		Paneles:  panelesIA,
		AlFilas:  filasIA,
		AlAccion: accionIA,
	})
}

func (g *Gestor) inicioDia() time.Time {
	l := g.Hora()
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, l.Location())
}

func (g *Gestor) consultasHoy() int {
	return g.Alm.ContarRegistroModulo(g.BotID, IDIA, "consulta", ResEnviado, g.inicioDia())
}

// opcionesChatsTodos junta grupos y privados para elegir.
func (g *Gestor) opcionesChatsTodos() []Opcion {
	return append(g.opcionesChats(true, 100), g.opcionesChats(false, 100)...)
}

func panelesIA(g *Gestor) []Panel {
	return []Panel{
		{ID: "chats", Titulo: "Chats permitidos", Ayuda: "Con «Responder a todo en los chats permitidos», la IA contesta en estos chats (con espera mínima entre respuestas).",
			Campos:  []Campo{{Clave: "chat", Titulo: "Chat", Tipo: CampoLista, Opciones: g.opcionesChatsTodos()}},
			Alta:    "Permitir chat",
			PorFila: []Boton{{Accion: "chats.quitar", Titulo: "Quitar"}}},
		{ID: "personalidad", Titulo: "Personalidad por chat", Ayuda: "Instrucciones propias para un chat (reemplazan a la general). Se guardan cifradas.",
			Campos: []Campo{{Clave: "chat", Titulo: "Chat", Tipo: CampoLista, Opciones: g.opcionesChatsTodos()},
				{Clave: "texto", Titulo: "Personalidad", Tipo: CampoTexto, Varias: true, Largo: 1200, Obligatorio: true}},
			Alta:    "Guardar personalidad",
			PorFila: []Boton{{Accion: "personalidad.quitar", Titulo: "Quitar", Confirma: "¿Quitar la personalidad de este chat?"}}},
	}
}

func filasIA(g *Gestor, panel string) []Fila {
	var out []Fila
	switch panel {
	case "chats":
		for _, c := range g.Alm.ChatsMod(g.BotID, IDIA) {
			out = append(out, Fila{ID: c, Titulo: g.nombreChat(c)})
		}
	case "personalidad":
		for _, d := range g.Alm.ListarDatosMod(g.BotID, IDIA, "personalidad", false, 200) {
			out = append(out, Fila{ID: d.Clave, Titulo: g.nombreChat(d.Clave), Detalle: fmt.Sprintf("%d letras", len([]rune(d.Valor)))})
		}
	}
	return out
}

func accionIA(g *Gestor, accion string, a Args) (string, error) {
	switch accion {
	case "chats.agregar":
		c := a.Texto("chat")
		if c == "" || !g.hayChat(c) {
			return "", errors.New("elige un chat de la lista")
		}
		return "Chat permitido.", g.Alm.AgregarChatMod(g.BotID, IDIA, c)
	case "chats.quitar":
		return "Chat quitado.", g.Alm.QuitarChatMod(g.BotID, IDIA, a.Texto("id"))
	case "personalidad.agregar":
		c, t := a.Texto("chat"), a.Texto("texto")
		if c == "" || !g.hayChat(c) {
			return "", errors.New("elige un chat de la lista")
		}
		if t == "" || len([]rune(t)) > 1200 {
			return "", errors.New("la personalidad debe tener entre 1 y 1200 letras")
		}
		if err := g.Alm.GuardarDatoMod(g.BotID, IDIA, "personalidad", c, 0, t, g.ahora()); err != nil {
			return "", err
		}
		return "Personalidad guardada.", nil
	case "personalidad.quitar":
		return "Personalidad quitada.", g.Alm.BorrarDatoMod(g.BotID, IDIA, "personalidad", a.Texto("id"))
	}
	return "", errors.New("esa acción no existe")
}

func corta(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return strings.TrimSpace(string(r[:n])) + "…"
	}
	return s
}

// ConsultarIA hace una consulta a la IA para esa persona y chat. Revisa el aviso de privacidad, los límites (por día y por persona y
// hora) y la longitud. Devuelve la respuesta ya recortada y con la etiqueta de «automática». En el registro solo queda que hubo una
// consulta (nunca textos).
func (g *Gestor) ConsultarIA(ctx context.Context, chat, persona, pregunta string) (string, error) {
	cfg := g.Config(IDIA)
	if strings.TrimSpace(cfg.Texto("url")) == "" {
		return "", errors.New("todavía no se configuró el servicio de IA")
	}
	pregunta = strings.TrimSpace(pregunta)
	if pregunta == "" {
		return "", errors.New("escribe tu pregunta después del comando")
	}
	if len([]rune(pregunta)) > cfg.Int("max_entrada") {
		return "", fmt.Errorf("la pregunta es muy larga (máximo %d letras)", cfg.Int("max_entrada"))
	}
	if t := cfg.Int("limite_dia"); t > 0 && g.consultasHoy() >= t {
		return "", errors.New("se alcanzó el límite de consultas de hoy; mañana vuelve a funcionar")
	}
	if !g.limite("ia_persona:"+persona, cfg.Int("limite_persona_hora"), time.Hour) {
		return "", errors.New("hiciste muchas preguntas en esta hora; intenta más tarde")
	}
	sis := cfg.Texto("personalidad")
	if d := g.Alm.ObtenerDatoMod(g.BotID, IDIA, "personalidad", chat); d != nil && strings.TrimSpace(d.Valor) != "" {
		sis = d.Valor
	}
	sis += "\nEres un asistente automático (una IA). Nunca digas ni insinúes que eres la persona dueña de esta cuenta o una persona real. Responde de forma breve."
	msgs := []MensajeIA{{"system", sis}}
	clave := chat + "|" + persona
	memoria := cfg.Bool("memoria")
	if memoria {
		g.h.mu.Lock()
		var vivos []turnoIA
		for _, t := range g.h.memoria[clave] {
			if g.ahora().Sub(t.TS) < 30*time.Minute {
				vivos = append(vivos, t)
			}
		}
		g.h.mu.Unlock()
		for _, t := range vivos {
			msgs = append(msgs, MensajeIA{t.Rol, t.Texto})
		}
	}
	msgs = append(msgs, MensajeIA{"user", pregunta})
	g.Alm.RegistrarModulo(g.BotID, IDIA, "consulta", ResEnviado, "", g.ahora())
	d := g.destinoIA(cfg)
	maxR := cfg.Int("max_respuesta")
	r, err := g.chatIA(ctx, d, cfg.Texto("modelo"), maxR/2+50, msgs)
	if err != nil {
		g.Alm.RegistrarModulo(g.BotID, IDIA, "consulta", ResFallido, "", g.ahora())
		return "", err
	}
	r = corta(r, maxR)
	if memoria {
		g.h.mu.Lock()
		if g.h.memoria == nil {
			g.h.memoria = map[string][]turnoIA{}
		}
		h := append(g.h.memoria[clave], turnoIA{"user", pregunta, g.ahora()}, turnoIA{"assistant", r, g.ahora()})
		if n := cfg.Int("memoria_turnos") * 2; len(h) > n {
			h = h[len(h)-n:]
		}
		g.h.memoria[clave] = h
		g.h.mu.Unlock()
	}
	return strings.TrimSpace(cfg.Texto("etiqueta")) + " " + r, nil
}

func comandosIA(g *Gestor) []comandos.Comando {
	return []comandos.Comando{{
		Nombre: "ia", Resumen: "Preguntarle a la IA", Uso: "ia <pregunta>", Casilla: "ia.usar",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			cfg := g.Config(IDIA)
			if !cfg.Bool("por_comando") {
				return "", errors.New("la IA no está disponible por comando")
			}
			q := strings.TrimSpace(cx.Resto)
			if q == "" && cx.CitaID != "" {
				if m, _ := g.Alm.ObtenerMensaje(g.BotID, cx.Chat, cx.CitaID); m != nil {
					q = m.Texto
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Int("tiempo_s")+10)*time.Second)
			defer cancel()
			return g.ConsultarIA(ctx, cx.Chat, cx.Persona, q)
		},
	}}
}

// TextoIA responde con IA para {ia_<dato>} en un menú: devuelve "" si algo falla (sin romper la conversación).
func (g *Gestor) TextoIA(pregunta, persona, chat string) string {
	if !g.Activo(IDIA) || !g.Config(IDIA).Bool("en_menus") {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	r, err := g.ConsultarIA(ctx, chat, persona, pregunta)
	if err != nil {
		return ""
	}
	return r
}

// eventoIA: solo decide si toca responder y lo hace en segundo plano (la consulta tarda) para no frenar los mensajes; lo que sale pasa
// por la cola de iniciativas.
func eventoIA(g *Gestor, e Evento) {
	if e.Tipo != EvMensaje || e.EsMio || strings.TrimSpace(e.Texto) == "" || e.Chat == "" {
		return
	}
	cfg := g.Config(IDIA)
	if strings.TrimSpace(cfg.Texto("url")) == "" {
		return
	}
	if g.Cmd != nil && g.Cmd.EsComando(e.Texto) {
		return
	}
	aplica := e.EsGrupo && cfg.Bool("por_mencion") && (e.Mencionado || e.CitaEsMia)
	if cfg.Bool("chats_activos") && g.Alm.EstaChatMod(g.BotID, IDIA, e.Chat) {
		aplica = true
	}
	if !aplica {
		return
	}
	persona := g.Alm.ResolverPersona(g.BotID, e.Persona)
	if g.Alm.EstaNoMolestar(g.BotID, persona) {
		return
	}
	if !g.limite("ia_chat:"+e.Chat, 1, time.Duration(cfg.Int("espera_chat_s"))*time.Second) {
		g.Alm.RegistrarModulo(g.BotID, IDIA, "respuesta", ResDescartado+MotTope, e.Chat, g.ahora())
		return
	}
	g.h.wg.Add(1)
	go func() {
		defer g.h.wg.Done()
		base := context.Background()
		g.mu.Lock()
		if g.ctx != nil {
			base = g.ctx
		}
		g.mu.Unlock()
		ctx, cancel := context.WithTimeout(base, time.Duration(cfg.Int("tiempo_s")+10)*time.Second)
		defer cancel()
		r, err := g.ConsultarIA(ctx, e.Chat, persona, e.Texto)
		if err != nil {
			g.Alm.RegistrarModulo(g.BotID, IDIA, "respuesta", ResFallido, e.Chat, g.ahora())
			return
		}
		var menc []string
		if e.EsGrupo && e.JID != "" {
			menc = []string{e.JID}
			r = "@" + usuarioDe(e.JID) + " " + r
		}
		_, _ = g.Enviar(Iniciativa{Modulo: IDIA, Que: "respuesta", Chat: e.Chat, Personas: []string{persona}, Texto: r, CitaID: e.MensajeID, Menciones: menc,
			Noche: NocheDescartar, RetrasoMin: 2 * time.Second, RetrasoMax: 6 * time.Second})
	}()
}

// sinLlaveIA dice si el servicio elegido no pide llave (Ollama).
func sinLlaveIA(cfg Config) bool {
	p, ok := proveedorIA(cfg.Texto("proveedor"))
	return ok && p.SinLlave
}
