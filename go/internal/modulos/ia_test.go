package modulos

import (
	"encoding/json"
	"strings"
	"testing"

	"botchat/internal/comandos"
)

func iaLista(t *testing.T) (*ent, *descFalso) {
	e, _ := entH(t)
	d := &descFalso{}
	e.g.Descarga = d
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) {
		return cuerpo("application/json", "", `{"choices":[{"message":{"content":"Claro, son las cuatro."}}]}`), nil
	}
	e.conChat(pnAna, false, "Ana")
	e.conChat(grupo, true, "Vecinos")
	FijarLlave(1, IDIA, "llave", "sk-SECRETO-999")
	t.Cleanup(func() { FijarLlave(1, IDIA, "llave", "") })
	e.activa(IDIA)
	e.config(IDIA, map[string]any{"acepto_privacidad": true})
	e.fija("tope_hora", "100")
	e.fija("tope_dia", "100")
	return e, d
}

func cmdIA(t *testing.T, e *ent) comandos.Comando {
	m, _ := Obtener(IDIA)
	return m.(ComandosDeGestor).ComandosG(e.g)[0]
}

func TestIAApagadaPorDefectoYAvisaPrivacidad(t *testing.T) {
	e, _ := entH(t)
	i, _ := e.g.InfoDe(IDIA)
	if i.Activo || !i.Iniciativa || i.Riesgo != RiesgoMedio || !strings.Contains(i.Descripcion, "SALE de tu teléfono") {
		t.Fatalf("info: %+v", i)
	}
	c := e.g.Config(IDIA)
	if c.Bool("memoria") || c.Bool("por_mencion") || c.Bool("chats_activos") || c.Bool("en_menus") || c.Bool("acepto_privacidad") {
		t.Fatalf("todo debe nacer apagado: %v", c)
	}
	_, err := cmdIA(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, Persona: pnAna, Resto: "hola"})
	esperaErr(t, err, "aviso de privacidad")
}

func TestIAComandoUsaLlaveEtiquetaYNoGuardaLlave(t *testing.T) {
	e, d := iaLista(t)
	r, err := cmdIA(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, Persona: pnAna, Resto: "¿qué hora es?"})
	if err != nil || !strings.HasPrefix(r, "🤖 IA: Claro") {
		t.Fatalf("%q %v", r, err)
	}
	p := d.reqs[0]
	if p.Cabeceras["Authorization"] != "Bearer sk-SECRETO-999" || !strings.HasSuffix(p.URL, "/v1/chat/completions") {
		t.Fatalf("petición: %+v", p.URL)
	}
	var cu struct {
		Modelo   string      `json:"model"`
		Mensajes []MensajeIA `json:"messages"`
	}
	_ = json.Unmarshal(p.Cuerpo, &cu)
	if cu.Modelo != "gpt-6-luna" || len(cu.Mensajes) != 2 || cu.Mensajes[1].Texto != "¿qué hora es?" || !strings.Contains(cu.Mensajes[0].Texto, "Nunca digas") {
		t.Fatalf("cuerpo: %+v", cu)
	}
	cj, _ := e.g.ConfigJSON(IDIA)
	f, _ := e.alm.ObtenerModulo(1, IDIA)
	if strings.Contains(cj+f.Config, "SECRETO") {
		t.Fatal("la llave no puede estar en la configuración")
	}
	for _, x := range e.registro() {
		if strings.Contains(x.Que+x.Resultado+x.Chat, "SECRETO") || strings.Contains(x.Que+x.Resultado, "hora") {
			t.Fatalf("el registro no lleva textos: %+v", x)
		}
	}
	b, _ := jsonDe(map[string]any{"llave": "sk-otra"})
	esperaErr(t, e.g.GuardarConfig(IDIA, b), "Keystore")
}

func TestIAValidaciones(t *testing.T) {
	e, _ := iaLista(t)
	b, _ := jsonDe(map[string]any{"etiqueta": "  "})
	esperaErr(t, e.g.GuardarConfig(IDIA, b), "vacío")
	e.config(IDIA, map[string]any{"acepto_privacidad": false})
	b, _ = jsonDe(map[string]any{"acepto_privacidad": false, "por_mencion": true})
	esperaErr(t, e.g.GuardarConfig(IDIA, b), "Entiendo")
}

func TestIALimitesDiaPersonaYLargo(t *testing.T) {
	e, d := iaLista(t)
	e.config(IDIA, map[string]any{"limite_dia": 3.0, "limite_persona_hora": 2.0, "max_entrada": 20.0})
	pg := func(p, q string) error {
		_, err := cmdIA(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, Persona: p, Resto: q})
		return err
	}
	esperaErr(t, pg(pnAna, "esta pregunta es demasiado larga para el tope"), "muy larga")
	if pg(pnAna, "uno") != nil || pg(pnAna, "dos") != nil {
		t.Fatal("deben pasar")
	}
	esperaErr(t, pg(pnAna, "tres"), "muchas preguntas")
	if pg(pnLuis, "cuatro") != nil {
		t.Fatal("otra persona sí")
	}
	esperaErr(t, pg(pnLuis, "cinco"), "límite de consultas de hoy")
	if len(d.reqs) != 3 {
		t.Fatalf("salieron %d peticiones", len(d.reqs))
	}
}

func TestIAMemoriaApagadaYEncendida(t *testing.T) {
	e, d := iaLista(t)
	n := func() int {
		var cu struct {
			Mensajes []MensajeIA `json:"messages"`
		}
		_ = json.Unmarshal(d.reqs[len(d.reqs)-1].Cuerpo, &cu)
		return len(cu.Mensajes)
	}
	pg := func(q string) { _, _ = cmdIA(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, Persona: pnAna, Resto: q}) }
	pg("uno")
	pg("dos")
	if n() != 2 {
		t.Fatalf("sin memoria deben ir 2 mensajes, fueron %d", n())
	}
	e.config(IDIA, map[string]any{"memoria": true})
	pg("tres")
	pg("cuatro")
	if n() != 4 {
		t.Fatalf("con memoria deben ir 4 mensajes, fueron %d", n())
	}
}

func TestIAPersonalidadPorChat(t *testing.T) {
	e, d := iaLista(t)
	if _, err := e.g.AccionModulo(IDIA, "personalidad.agregar", Args{"chat": pnAna, "texto": "Eres un pirata."}); err != nil {
		t.Fatal(err)
	}
	_, _ = cmdIA(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, Persona: pnAna, Resto: "hola"})
	if !strings.Contains(string(d.reqs[0].Cuerpo), "pirata") {
		t.Fatal("debe usar la personalidad del chat")
	}
	_, _ = cmdIA(t, e).Ejecutar(&comandos.Contexto{Chat: grupo, Persona: pnAna, Resto: "hola"})
	if strings.Contains(string(d.reqs[1].Cuerpo), "pirata") {
		t.Fatal("otro chat usa la general")
	}
	fs, _ := e.g.FilasPanel(IDIA, "personalidad")
	if len(fs) != 1 {
		t.Fatalf("filas: %v", fs)
	}
	// la personalidad se guarda cifrada
	if d := e.alm.ObtenerDatoMod(1, IDIA, "personalidad", pnAna); d == nil || d.Valor != "Eres un pirata." {
		t.Fatal("no se guardó")
	}
}

func TestIAMencionEnGrupoPasaPorLaCola(t *testing.T) {
	e, d := iaLista(t)
	e.config(IDIA, map[string]any{"por_mencion": true, "espera_chat_s": 3600.0})
	// sin mención no hace nada
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: grupo, EsGrupo: true, Persona: pnAna, JID: pnAna, Texto: "hola a todos", MensajeID: "G1"})
	e.g.EsperarH()
	if len(d.reqs) != 0 {
		t.Fatal("sin mención no responde")
	}
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: grupo, EsGrupo: true, Persona: pnAna, JID: pnAna, Texto: "bot, ¿qué hora es?", MensajeID: "G2", Mencionado: true})
	e.g.EsperarH()
	e.simula(1)
	if !e.hay(grupo + "|@5215511111111 🤖 IA: Claro") {
		t.Fatalf("enviados: %v", e.enviados())
	}
	// una segunda mención muy seguida cae en la espera por chat
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: grupo, EsGrupo: true, Persona: pnLuis, JID: pnLuis, Texto: "y a mí", MensajeID: "G3", Mencionado: true})
	e.g.EsperarH()
	if len(d.reqs) != 1 {
		t.Fatalf("la espera mínima por chat debe frenar: %d", len(d.reqs))
	}
}

func TestIAAutomaticaRespetaMaestroNoMolestarYModoSeguro(t *testing.T) {
	e, d := iaLista(t)
	e.config(IDIA, map[string]any{"chats_activos": true})
	_ = e.alm.AgregarChatMod(1, IDIA, pnAna)
	ev := Evento{Tipo: EvMensaje, Chat: pnAna, Persona: pnAna, JID: pnAna, Texto: "hola", MensajeID: "P1"}
	// No molestar
	_ = e.g.AgregarNoMolestar(pnAna, "baja")
	e.g.Evento(ev)
	e.g.EsperarH()
	if len(d.reqs) != 0 {
		t.Fatal("No molestar: ni siquiera consulta")
	}
	_ = e.g.QuitarNoMolestar(pnAna)
	// maestro apagado: consulta pero la cola descarta
	e.fija("maestro", "0")
	e.g.Evento(ev)
	e.g.EsperarH()
	e.simula(1)
	if e.cuantos() != 0 {
		t.Fatalf("maestro apagado: nada sale %v", e.enviados())
	}
	e.fija("maestro", "1")
	e.hora = e.hora.Add(2 * 60 * 1e9)
	e.g.Evento(ev)
	e.g.EsperarH()
	e.simula(1)
	if !e.hay(pnAna + "|🤖 IA:") {
		t.Fatalf("enviados: %v", e.enviados())
	}
	// una conversación que no está en la lista no se atiende; un comando tampoco
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: pnLuis, Persona: pnLuis, Texto: "hola", MensajeID: "L1"})
	e.g.Evento(Evento{Tipo: EvMensaje, Chat: pnAna, Persona: pnAna, Texto: "!ia hola", MensajeID: "P9"})
	e.g.EsperarH()
}

func TestIAEnMenusConTextoIA(t *testing.T) {
	e, d := iaLista(t)
	if e.g.TextoIA("hola", pnAna, pnAna) != "" {
		t.Fatal("en_menus apagado: vacío")
	}
	e.config(IDIA, map[string]any{"en_menus": true})
	if r := e.g.TextoIA("hola", pnAna, pnAna); !strings.HasPrefix(r, "🤖 IA:") {
		t.Fatalf("respuesta: %q", r)
	}
	d.resp = func(PeticionHTTP) (*RespuestaHTTP, error) { return nil, ErrDominio }
	if e.g.TextoIA("hola", pnLuis, pnLuis) != "" {
		t.Fatal("si falla, vacío sin romper")
	}
}

func TestIAClaudeUsaSuFormato(t *testing.T) {
	e, d := iaLista(t)
	d.resp = func(p PeticionHTTP) (*RespuestaHTTP, error) {
		return cuerpo("application/json", "", `{"content":[{"type":"text","text":"Hola desde Claude"}]}`), nil
	}
	e.config(IDIA, map[string]any{"acepto_privacidad": true, "proveedor": "anthropic", "url": "https://api.anthropic.com", "modelo": "claude-haiku-5-5"})
	r, err := cmdIA(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, Persona: pnAna, Resto: "hola"})
	if err != nil || !strings.HasSuffix(r, "Hola desde Claude") {
		t.Fatalf("%q %v", r, err)
	}
	p := d.reqs[len(d.reqs)-1]
	if p.URL != "https://api.anthropic.com/v1/messages" || p.Cabeceras["x-api-key"] != "sk-SECRETO-999" || p.Cabeceras["anthropic-version"] == "" || p.Cabeceras["Authorization"] != "" {
		t.Fatalf("petición: %s %v", p.URL, p.Cabeceras)
	}
	var cu struct {
		Modelo    string      `json:"model"`
		MaxTokens int         `json:"max_tokens"`
		Sistema   string      `json:"system"`
		Mensajes  []MensajeIA `json:"messages"`
		Temp      *float64    `json:"temperature"`
	}
	_ = json.Unmarshal(p.Cuerpo, &cu)
	if cu.Modelo != "claude-haiku-5-5" || cu.MaxTokens <= 0 || !strings.Contains(cu.Sistema, "Nunca digas") || len(cu.Mensajes) != 1 || cu.Mensajes[0].Rol != "user" || cu.Temp != nil {
		t.Fatalf("cuerpo: %+v", cu)
	}
}

func TestIAOpenAIUsaMaxCompletionTokensYOtrosMaxTokens(t *testing.T) {
	e, d := iaLista(t)
	e.config(IDIA, map[string]any{"acepto_privacidad": true, "proveedor": "openai"})
	_, _ = cmdIA(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, Persona: pnAna, Resto: "hola"})
	if c := string(d.reqs[len(d.reqs)-1].Cuerpo); !strings.Contains(c, "max_completion_tokens") || strings.Contains(c, "temperature") {
		t.Fatalf("openai: %s", c)
	}
	e.config(IDIA, map[string]any{"acepto_privacidad": true, "proveedor": "groq", "url": "https://api.groq.com/openai/v1", "modelo": "llama-3.1-8b-instant"})
	_, _ = cmdIA(t, e).Ejecutar(&comandos.Contexto{Chat: pnAna, Persona: pnAna, Resto: "hola"})
	if c := string(d.reqs[len(d.reqs)-1].Cuerpo); !strings.Contains(c, `"max_tokens"`) || strings.Contains(c, "max_completion_tokens") {
		t.Fatalf("groq: %s", c)
	}
}

func TestIAProbarYCatalogo(t *testing.T) {
	e, _ := iaLista(t)
	r, err := e.g.ProbarIA()
	if err != nil || r == "" {
		t.Fatalf("probar: %q %v", r, err)
	}
	FijarLlave(1, IDIA, "llave", "")
	_, err = e.g.ProbarIA()
	esperaErr(t, err, "llave")
	var l []ProveedorIA
	if json.Unmarshal([]byte(ProveedoresIA()), &l) != nil || len(l) < 10 {
		t.Fatalf("catálogo: %d", len(l))
	}
	for _, p := range l {
		if p.ID != "otro" && (p.URL == "" || p.Defecto == "" || len(p.Modelos) == 0) {
			t.Fatalf("proveedor incompleto: %+v", p)
		}
	}
}
