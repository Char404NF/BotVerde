package modulos

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/avatar"
	"botchat/internal/envio"
	"botchat/internal/estilo"
	"botchat/internal/permisos"
)

// ---------- H1 · Bienvenida a miembros nuevos ----------
//
// Cuando alguien entra a un grupo, un mensaje EN EL GRUPO con {nombre} {grupo} {reglas}. Se elige en qué grupos. Verificación opcional
// por texto ("responde 1 para aceptar las reglas" o una suma) y, si soy administrador, expulsión de quien no verifica a tiempo.
// Aparte y apagado: mensaje al privado del nuevo (30–120 s de espera y tope diario bajo).
// El evento solo anota y encola: todo lo que sale pasa por la cola de iniciativas.

// IDBienvenida es el id del módulo.
const IDBienvenida = "bienvenida"

const (
	qBienvenida  = "bienvenida"
	qBienvPriv   = "bienvenida_privada"
	qExpulsar    = "expulsar_sin_verificar"
	listaChatsOk = "bienvenida"
)

var reNumeros = regexp.MustCompile(`\d+`)

func bienvenida() Modulo {
	return Nuevo(Def{
		ID: IDBienvenida, Nombre: "Bienvenida a miembros nuevos", Icono: "saludo", Riesgo: RiesgoMedio, Iniciativa: true,
		Descripcion: "Cuando alguien entra a un grupo, el bot le da la bienvenida en el grupo (con las reglas). Puede pedirle que responda para comprobar que es persona y, si eres administrador, sacar a quien no lo haga a tiempo.",
		Casillas: []permisos.Casilla{
			{ID: "bienvenida.exento", Nombre: "Sin verificación de bienvenida", Descripcion: "Las personas con esta casilla no tienen que responder la verificación ni se les expulsa por no hacerlo.", Grupo: "Bienvenida", Grupos: true, Riesgo: permisos.RiesgoMedio},
		},
		Esquema: []Campo{
			{Clave: "mensaje", Titulo: "Mensaje de bienvenida", Ayuda: "Variables: {nombre} (menciona a quien entró), {grupo}, {reglas} (las reglas tal cual) y {bloque_reglas} (las reglas con su título; no sale nada si el grupo no tiene).", Tipo: CampoTexto, Varias: true, Largo: 900,
				Defecto: mensajeBienvenidaNuevo, Obligatorio: true},
			{Clave: "imagen", Titulo: "Mandar la imagen del avatar", Ayuda: "El mensaje sale como pie de la imagen del bot (la cara, o la tuya en «Avatar del bot»). Se respeta lo que elijas allí para «Bienvenida».", Tipo: CampoInterruptor, Defecto: true},
			{Clave: "reglas", Titulo: "Reglas por defecto", Ayuda: "Se usan en {reglas} cuando el grupo no tiene las suyas (Grupos → Reglas).", Tipo: CampoTexto, Varias: true, Largo: 900, Defecto: ""},
			{Clave: "grupos_modo", Titulo: "En qué grupos", Ayuda: "«Solo los elegidos» y «todos menos» usan la lista de abajo.", Tipo: CampoLista, Defecto: "elegidos",
				Opciones: []Opcion{{"elegidos", "Solo los elegidos"}, {"todos", "Todos los grupos"}, {"todos_menos", "Todos menos los elegidos"}}},
			{Clave: "retraso_min", Titulo: "Espera mínima (segundos)", Tipo: CampoNumero, Defecto: 5.0, Min: 1, Max: 600},
			{Clave: "retraso_max", Titulo: "Espera máxima (segundos)", Tipo: CampoNumero, Defecto: 25.0, Min: 1, Max: 900},
			{Clave: "verificar", Titulo: "Pedir verificación", Ayuda: "La persona debe responder en el grupo.", Tipo: CampoInterruptor, Defecto: false},
			{Clave: "verif_tipo", Titulo: "Tipo de verificación", Tipo: CampoLista, Defecto: "reglas", Opciones: []Opcion{{"reglas", "Responder 1 para aceptar las reglas"}, {"suma", "Una suma sencilla"}}},
			{Clave: "verif_minutos", Titulo: "Minutos para verificar", Tipo: CampoNumero, Defecto: 10.0, Min: 1, Max: 1440},
			{Clave: "expulsar", Titulo: "Expulsar a quien no verifique", Ayuda: "Solo funciona si eres administrador del grupo; si no, te llega un aviso y no se hace nada.", Tipo: CampoInterruptor, Defecto: false},
			{Clave: "privado", Titulo: "Mensaje al privado del nuevo", Ayuda: "Le escribe por privado a quien acaba de entrar al grupo. Apagado por defecto.", Tipo: CampoInterruptor, Defecto: false},
			{Clave: "privado_texto", Titulo: "Texto del mensaje privado", Ayuda: "Variables: {nombre} y {grupo}.", Tipo: CampoTexto, Varias: true, Largo: 600, Defecto: "Hola {nombre}, bienvenido a {grupo}. Si no quieres más mensajes, escribe BAJA."},
			{Clave: "privado_retraso_min", Titulo: "Privado · espera mínima (segundos)", Tipo: CampoNumero, Defecto: 30.0, Min: 30, Max: 600},
			{Clave: "privado_retraso_max", Titulo: "Privado · espera máxima (segundos)", Tipo: CampoNumero, Defecto: 120.0, Min: 30, Max: 1200},
			{Clave: "privado_tope_dia", Titulo: "Privado · máximo por día", Tipo: CampoNumero, Defecto: 5.0, Min: 1, Max: 50},
		},
		TopeHora: 20, TopeDia: 100,
		Validar: func(c map[string]any) error {
			cfg := Config(c)
			if cfg.Num("retraso_max") < cfg.Num("retraso_min") {
				return errors.New("la espera máxima no puede ser menor que la mínima")
			}
			if cfg.Num("privado_retraso_max") < cfg.Num("privado_retraso_min") {
				return errors.New("en el mensaje privado la espera máxima no puede ser menor que la mínima")
			}
			if cfg.Bool("expulsar") && !cfg.Bool("verificar") {
				return errors.New("para expulsar hace falta pedir verificación")
			}
			return nil
		},
		AlEvento: eventoBienvenida,
		AlEstado: func(g *Gestor) string {
			n := g.Alm.ContarRegistroModulo(g.BotID, IDBienvenida, qBienvenida, ResEnviado, g.ahora().Add(-24*time.Hour))
			p := len(g.Alm.PendientesDe(g.BotID, IDBienvenida, ""))
			switch {
			case n > 0 && p > 0:
				return fmt.Sprintf("Encendido · %d bienvenidas en 24 h · %d por verificar", n, p)
			case n > 0:
				return fmt.Sprintf("Encendido · %d bienvenidas en 24 h", n)
			case p > 0:
				return fmt.Sprintf("Encendido · %d por verificar", p)
			}
			return ""
		},
		Paneles: func(g *Gestor) []Panel {
			return []Panel{{
				ID: "grupos", Titulo: "Grupos elegidos", Ayuda: "Con «Solo los elegidos» la bienvenida va solo a estos grupos; con «todos menos», a todos menos estos.",
				Campos:  []Campo{{Clave: "chat", Titulo: "Grupo", Tipo: CampoLista, Opciones: g.opcionesChats(true, 200)}},
				Alta:    "Agregar grupo",
				PorFila: []Boton{{Accion: "grupos.quitar", Titulo: "Quitar"}},
			}}
		},
		AlFilas: func(g *Gestor, panel string) []Fila {
			var out []Fila
			for _, c := range g.Alm.ChatsMod(g.BotID, IDBienvenida) {
				out = append(out, Fila{ID: c, Titulo: g.nombreChat(c)})
			}
			return out
		},
		AlAccion: func(g *Gestor, accion string, a Args) (string, error) {
			switch accion {
			case "grupos.agregar":
				c := a.Texto("chat")
				if !strings.HasSuffix(c, "@g.us") || !g.hayChat(c) {
					return "", errors.New("elige un grupo de la lista")
				}
				if err := g.Alm.AgregarChatMod(g.BotID, IDBienvenida, c); err != nil {
					return "", err
				}
				return "Grupo agregado.", nil
			case "grupos.quitar":
				if err := g.Alm.QuitarChatMod(g.BotID, IDBienvenida, a.Texto("id")); err != nil {
					return "", err
				}
				return "Grupo quitado.", nil
			}
			return "", errors.New("esa acción no existe")
		},
	})
}

func (g *Gestor) bienvenidaAplica(cfg Config, chat string) bool {
	switch cfg.Texto("grupos_modo") {
	case "todos":
		return true
	case "todos_menos":
		return !g.Alm.EstaChatMod(g.BotID, IDBienvenida, chat)
	}
	return g.Alm.EstaChatMod(g.BotID, IDBienvenida, chat)
}

func (g *Gestor) esExento(persona, chat string) bool {
	r, _, _ := g.Alm.RolDePersona(g.BotID, g.Alm.ResolverPersona(g.BotID, persona), g.ahora())
	return r != nil && permisos.Permite(r, "bienvenida.exento", true, chat)
}

func limpiaLineasVacias(t string) string {
	var out []string
	vacia := false
	for _, l := range strings.Split(strings.TrimSpace(t), "\n") {
		if strings.TrimSpace(l) == "" {
			if vacia {
				continue
			}
			vacia = true
		} else {
			vacia = false
		}
		out = append(out, strings.TrimRight(l, " "))
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func segundos(f float64) time.Duration { return time.Duration(f * float64(time.Second)) }

func eventoBienvenida(g *Gestor, e Evento) {
	cfg := g.Config(IDBienvenida)
	switch e.Tipo {
	case EvMiembroEntra:
		bienvenidaEntra(g, cfg, e)
	case EvMensaje:
		bienvenidaMensaje(g, cfg, e)
	case EvTick:
		bienvenidaVencidos(g, cfg, e)
	}
}

func bienvenidaEntra(g *Gestor, cfg Config, e Evento) {
	if !strings.HasSuffix(e.Chat, "@g.us") || !g.bienvenidaAplica(cfg, e.Chat) {
		return
	}
	var personas, jids []string
	for i, p := range e.Personas {
		if g.Alm.EstaNoMolestar(g.BotID, p) {
			continue
		}
		personas = append(personas, p)
		j := p
		if i < len(e.JIDs) && e.JIDs[i] != "" {
			j = e.JIDs[i]
		}
		jids = append(jids, j)
	}
	if len(personas) == 0 {
		return
	}
	if len(personas) > 8 { // una oleada grande de entradas: se saluda a los primeros, no se inunda el grupo
		personas, jids = personas[:8], jids[:8]
	}
	menciones := make([]string, 0, len(jids))
	nombres := make([]string, 0, len(jids))
	for _, j := range jids {
		menciones = append(menciones, j)
		nombres = append(nombres, "@"+usuarioDe(j))
	}
	reglas := strings.TrimSpace(g.Alm.ReglasGrupo(g.BotID, e.Chat))
	if reglas == "" {
		reglas = strings.TrimSpace(cfg.Texto("reglas"))
	}
	reglas = reglasEnLineas(reglas)
	grupo := g.nombreChat(e.Chat)
	bloque := ""
	if reglas != "" {
		bloque = "📜 *Reglas del grupo*\n" + reglas
	}
	plantilla := cfg.Texto("mensaje")
	if plantilla == mensajeBienvenidaAntiguo { // quien nunca lo cambió pasa al diseño nuevo
		plantilla = mensajeBienvenidaNuevo
	}
	texto := sustituir(plantilla, map[string]string{"nombre": strings.Join(nombres, ", "), "grupo": grupo, "reglas": reglas, "bloque_reglas": bloque})
	texto = limpiaLineasVacias(texto)
	ver := cfg.Bool("verificar")
	var esperada string
	var exentos = map[string]bool{}
	if ver {
		n := cfg.Int("verif_minutos")
		if cfg.Texto("verif_tipo") == "suma" {
			a, b := int(g.Aleatorio(9))+1, int(g.Aleatorio(9))+1
			esperada = strconv.Itoa(a + b)
			texto += fmt.Sprintf("\n\nPara quedarte en el grupo, responde aquí cuánto es *%d + %d* (solo el número). Tienes %d minutos.", a, b, n)
		} else {
			esperada = "1"
			texto += fmt.Sprintf("\n\nPara quedarte en el grupo, responde *1* si aceptas las reglas. Tienes %d minutos.", n)
		}
		ahora := g.ahora()
		for _, p := range personas {
			if g.esExento(p, e.Chat) {
				exentos[p] = true
				continue
			}
			_ = g.Alm.PonerPendiente(g.BotID, IDBienvenida, almacen.Pendiente{Chat: e.Chat, Persona: g.Alm.ResolverPersona(g.BotID, p), DesdeTS: ahora.Unix(),
				VenceTS: ahora.Add(time.Duration(n) * time.Minute).Unix(), Esperada: esperada})
		}
	}
	if texto != "" {
		ini := Iniciativa{Modulo: IDBienvenida, Que: qBienvenida, Chat: e.Chat, Personas: personas, Texto: texto, Menciones: menciones,
			RetrasoMin: segundos(cfg.Num("retraso_min")), RetrasoMax: segundos(cfg.Num("retraso_max")), Noche: NocheDiferir}
		if cfg.Bool("imagen") {
			ini.Medio = g.imagenBienvenida(e.Chat, texto, menciones)
		}
		_, _ = g.Enviar(ini)
	}
	if cfg.Bool("privado") {
		bienvenidaPrivada(g, cfg, e, personas, grupo)
	}
}

// bienvenidaPrivada manda el mensaje al privado de los nuevos (opción aparte, apagada por defecto): espera larga, tope diario bajo.
func bienvenidaPrivada(g *Gestor, cfg Config, e Evento, personas []string, grupo string) {
	for _, p := range personas {
		hechos := g.Alm.ContarRegistroModulo(g.BotID, IDBienvenida, qBienvPriv, ResEnviado, g.ahora().Add(-24*time.Hour))
		g.h.mu.Lock()
		cola := g.h.privCola
		if hechos+cola >= cfg.Int("privado_tope_dia") {
			g.h.mu.Unlock()
			g.Alm.RegistrarModulo(g.BotID, IDBienvenida, qBienvPriv, ResDescartado+MotTope, p, g.ahora())
			continue
		}
		g.h.privCola++
		g.h.mu.Unlock()
		libera := func(string) { g.h.mu.Lock(); g.h.privCola--; g.h.mu.Unlock() }
		if !g.hayChat(p) {
			// quien nunca escribió no tiene chat en el teléfono; se crea vacío para poder escribirle (esa es justo la parte arriesgada)
			_, _ = g.Alm.ActualizarChat(g.BotID, p, false, func(c *almacen.Chat) {})
		}
		texto := sustituir(cfg.Texto("privado_texto"), map[string]string{"nombre": g.nombreOPalabra(p, e.Nombre), "grupo": grupo})
		res, err := g.Enviar(Iniciativa{Modulo: IDBienvenida, Que: qBienvPriv, Chat: p, Personas: []string{p}, Texto: texto, Noche: NocheDescartar,
			RetrasoMin: segundos(cfg.Num("privado_retraso_min")), RetrasoMax: segundos(cfg.Num("privado_retraso_max")), AlResultado: libera})
		if err != nil || res != EnCola {
			if err != nil {
				libera("")
			}
			// si se descartó al encolar, AlResultado ya liberó el lugar
		}
	}
}

func esRespuestaCorrecta(tipo, esperada, texto string) bool {
	t := llave(texto)
	if t == "" {
		return false
	}
	if esperada != "1" || tipo == "suma" {
		if t == esperada {
			return true
		}
		if n := reNumeros.FindString(t); n == esperada && len(strings.Fields(t)) <= 3 {
			return true
		}
		return false
	}
	switch t {
	case "1", "si", "acepto", "acepto las reglas", "de acuerdo", "ok", "okay", "listo", "claro", "si acepto":
		return true
	}
	return false
}

func bienvenidaMensaje(g *Gestor, cfg Config, e Evento) {
	if !e.EsGrupo || e.EsMio || !cfg.Bool("verificar") {
		return
	}
	persona := g.Alm.ResolverPersona(g.BotID, e.Persona)
	for _, p := range g.Alm.PendientesDe(g.BotID, IDBienvenida, e.Chat) {
		if p.Persona != persona {
			continue
		}
		if esRespuestaCorrecta(cfg.Texto("verif_tipo"), p.Esperada, e.Texto) {
			g.Alm.QuitarPendiente(g.BotID, IDBienvenida, e.Chat, p.Persona)
			g.Alm.RegistrarModulo(g.BotID, IDBienvenida, "verificado", "ok", e.Chat, g.ahora())
		} else {
			g.Alm.SumarIntentoPendiente(g.BotID, IDBienvenida, e.Chat, p.Persona)
		}
		return
	}
}

func bienvenidaVencidos(g *Gestor, cfg Config, e Evento) {
	ahora := g.ahora().Unix()
	for _, p := range g.Alm.PendientesDe(g.BotID, IDBienvenida, "") {
		if p.VenceTS > ahora {
			continue
		}
		g.Alm.QuitarPendiente(g.BotID, IDBienvenida, p.Chat, p.Persona)
		if !cfg.Bool("expulsar") || g.Acc == nil || g.esExento(p.Persona, p.Chat) {
			g.Alm.RegistrarModulo(g.BotID, IDBienvenida, "sin_verificar", "vencida", p.Chat, g.ahora())
			continue
		}
		chat, persona := p.Chat, p.Persona
		_, _ = g.Enviar(Iniciativa{Modulo: IDBienvenida, Que: qExpulsar, Chat: chat, Noche: NocheDiferir, RetrasoMin: time.Second, RetrasoMax: 5 * time.Second,
			Accion: func(ctx context.Context) error {
				ok, err := g.Acc.EsAdmin(ctx, chat)
				if err != nil {
					return err
				}
				if !ok {
					if g.Aviso != nil {
						g.Aviso("No pude expulsar a alguien", "Una persona no verificó a tiempo en «"+g.nombreChat(chat)+"», pero no soy administrador de ese grupo. Hazme administrador o sácala a mano.")
					}
					return ErrOmitida
				}
				if err := g.Acc.Expulsar(ctx, chat, persona); err != nil {
					return err
				}
				if g.Aviso != nil {
					g.Aviso("Persona expulsada", "Se expulsó a alguien de «"+g.nombreChat(chat)+"» por no verificar a tiempo.")
				}
				return nil
			}})
	}
}

// imagenBienvenida arma la imagen del avatar con el mensaje de pie, o nil si el avatar está apagado para la bienvenida o el texto no cabe.
func (g *Gestor) imagenBienvenida(chat, texto string, menciones []string) *envio.Adjunto {
	av := avatar.Nuevo(g.Alm, g.BotID)
	if !av.Corresponde(avatar.Bienvenida, chat, texto) {
		return nil
	}
	ref, mime, nombre, ok := av.Imagen(avatar.Bienvenida)
	if !ok {
		return nil
	}
	return &envio.Adjunto{Tipo: almacen.TipoImagen, Ruta: PrefijoAdj + ref, Mime: mime, Nombre: nombre, Pie: texto, Menciones: menciones}
}

var reNumeroRegla = regexp.MustCompile(`(\S)[ \t]+(\d{1,2}[.)][ \t])`)

// reglasEnLineas pone cada regla numerada en su línea cuando se escribieron seguidas («1. Respeto 2. Sin spam»).
func reglasEnLineas(r string) string {
	if strings.Contains(r, "\n") {
		return r
	}
	return reNumeroRegla.ReplaceAllString(r, "$1\n$2")
}

// El texto de fábrica de antes y el de ahora (tarjeta con título, reglas en lista y una ayuda al final).
const (
	mensajeBienvenidaAntiguo = "¡Hola {nombre}! 👋 Bienvenido a *{grupo}*.\n\n{reglas}"
	mensajeBienvenidaNuevo   = "👋 *¡Bienvenido a {grupo}!*\n" + estilo.Raya + "\nHola {nombre}, qué gusto tenerte aquí.\n\n{bloque_reglas}\n\n💡 _Escribe !menu para ver lo que puedo hacer_"
)
