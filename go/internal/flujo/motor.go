package flujo

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/identidad"
	"botchat/internal/motor"
)

// Config son las piezas del motor de menús.
type Config struct {
	BotID  int64
	Alm    *almacen.Almacen
	Ahora  func() time.Time
	Zona   func() *time.Location
	Expira func() time.Duration // sin actividad por este tiempo, la conversación caduca (2 h)
	// Nombre da el nombre con el que se ve a esa persona.
	Nombre func(chat, sender string) string
	// AlAviso se llama cuando hay una notificación local que mostrar: tipo ("humano" o "solicitud"), título neutro y detalle completo.
	AlAviso func(chat, tipo, titulo, detalle string)
	// MenuVisible dice si el rango de esa persona ve ese menú (nil = todos lo ven). Lo fija cada rango en "menús que ve".
	MenuVisible func(sender string, menuID int64) bool
	// OpcionVisible dice si el rango de esa persona ve esa opción de un menú (el rango guarda "idMenu:idOpcion"). nil = todas.
	OpcionVisible func(sender string, menuID int64, opcionID string) bool
	// Persona lleva el JID de quien escribe (número o LID) a su clave canónica. nil = tal cual. Las conversaciones de grupo van por
	// (grupo, persona canónica).
	Persona func(p string) string
	// Prefijo es el prefijo actual de los comandos ("!" por defecto). Los comandos propios escritos sin símbolo ("horario") usan este
	// prefijo; los que traen símbolo propio (".menu") se comparan tal cual.
	Prefijo func() string
	// PuedeUsar dice si esa persona tiene al menos el rango mínimo (id de rango) que pide un comando propio.
	PuedeUsar func(sender string, rolMin int64) bool
	// Conector (módulo H5) devuelve el texto de un conector HTTP para {api_<nombre>} en menús, reglas y comandos propios. nil o "" =
	// la variable queda vacía (sin romper la conversación).
	Conector func(nombre, sender, chat string) string
	// IA (módulo H7) responde con IA a lo que la persona escribió para {ia_<dato>}: recibe ese dato (la pregunta). nil o "" = vacío.
	IA func(pregunta, sender, chat string) string
}

// Motor ejecuta las automatizaciones. Implementa motor.Decisor.
type Motor struct{ c Config }

// Nuevo crea el motor de menús.
func Nuevo(c Config) *Motor {
	if c.Ahora == nil {
		c.Ahora = time.Now
	}
	if c.Expira == nil {
		c.Expira = func() time.Duration { return 2 * time.Hour }
	}
	if c.Zona == nil {
		c.Zona = func() *time.Location { return time.Local }
	}
	if c.Nombre == nil {
		c.Nombre = func(chat, sender string) string { return "" }
	}
	return &Motor{c: c}
}

// Textos fijos que ve la persona (los demás son ajustes editables: Ajustes → Avanzado → Menús).
const (
	textoMenuPrinc = "Volver al menú principal"
	umbralOpcion   = 24 // un mensaje más largo que esto no se toma como "elegir una opción"
)

var reNumero = regexp.MustCompile(`^\s*(\d{1,2})\b`)

func norm(s string) string { return identidad.Normalizar(s) }

func (m *Motor) ajuste(clave string) string { return m.c.Alm.AjusteBot(m.c.BotID, clave) }
func (m *Motor) ajusteInt(clave string) int { return m.c.Alm.AjusteBotInt(m.c.BotID, clave) }

// persona lleva el JID de quien escribe a su clave canónica (el número, aunque haya escrito con su LID).
func (m *Motor) persona(p string) string {
	if m.c.Persona != nil {
		return m.c.Persona(p)
	}
	return p
}

func esGrupoChat(chat string) bool { return strings.HasSuffix(chat, "@g.us") }

// ---------- entrada del motor ----------

// Decidir implementa motor.Decisor.
func (m *Motor) Decidir(_ context.Context, ultimo motor.Mensaje, lote []motor.Mensaje) (motor.Decision, error) {
	e := m.entradaDe(ultimo, lote)
	r, err := m.procesa(e, false, 0)
	if err != nil {
		return motor.Decision{}, err
	}
	return r.decision(), nil
}

// entradaDe arma lo que el menú necesita saber de un mensaje (o de una ráfaga de mensajes de un privado).
func (m *Motor) entradaDe(ultimo motor.Mensaje, lote []motor.Mensaje) entrada {
	sender := ultimo.Chat
	if ultimo.EsGrupo {
		sender = m.persona(ultimo.Remitente)
	}
	e := entrada{Chat: ultimo.Chat, Sender: sender, EsGrupo: ultimo.EsGrupo, Mencionado: ultimo.Mencionado, CitaEsMia: ultimo.CitaEsMia,
		CitaID: ultimo.CitaID, MsgID: ultimo.ID, WA: ultimo.RemitenteWA, TS: ultimo.TS}
	var textos []motor.Mensaje
	for _, x := range lote {
		if x.EsTexto && strings.TrimSpace(x.Texto) != "" {
			textos = append(textos, x)
		}
	}
	if len(lote) == 0 && ultimo.EsTexto {
		textos = []motor.Mensaje{ultimo}
	}
	if len(textos) == 0 {
		e.NoTexto = true
	} else {
		e.Texto = textos[len(textos)-1].Texto
		if len(textos) > 1 {
			var b strings.Builder
			for i, x := range textos {
				if i > 0 {
					b.WriteString("\n")
				}
				b.WriteString(x.Texto)
			}
			e.TextoCompleto = b.String()
			if textos[0].TS.Before(e.TS) {
				e.PrimerTS = textos[0].TS
			}
		}
	}
	if e.PrimerTS.IsZero() {
		e.PrimerTS = e.TS
	}
	// En grupos, "dirigido al bot" = lo mencionan, citan un mensaje suyo o escriben un comando.
	e.Dirigido = !e.EsGrupo || e.Mencionado || e.CitaEsMia || m.pareceComando(e.Texto)
	return e
}

// pareceComando: el texto empieza con el prefijo configurable o con ".letra" (como ".menu").
func (m *Motor) pareceComando(t string) bool {
	t = strings.TrimSpace(t)
	if t == "" {
		return false
	}
	if p := m.prefijo(); strings.HasPrefix(t, p) && len([]rune(t)) > len([]rune(p)) {
		return true
	}
	if len(t) >= 2 && t[0] == '.' {
		r := []rune(t)[1]
		return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r > 127
	}
	return false
}

type entrada struct {
	Chat, Sender  string
	Quien         string // en un grupo, quien escribió cuando contesta por otra persona ("cualquiera puede contestar")
	Texto         string
	TextoCompleto string // todo lo que escribió en una ráfaga
	EsGrupo       bool
	Mencionado    bool
	CitaEsMia     bool
	Dirigido      bool // en grupos: le hablan al bot (mención, cita o comando); en privado siempre
	NoTexto       bool // imagen, audio, sticker o archivo
	CitaID        string
	MsgID         string
	WA            string // JID de quien escribe, tal como lo mandó WhatsApp (para mencionarlo)
	TS            time.Time
	PrimerTS      time.Time
}

type resultado struct {
	Salidas   []motor.Salida
	Coincidio string
	Humano    bool
	Omitir    string
	Estado    string // estado de la conversación al terminar este turno
	Paso      string
	Sesion    *motor.RefSesion
	Termino   bool   // una conversación de menú acaba de terminar bien (para el mensaje automático de cierre)
	Etiqueta  string // "menu", "regla" o "comando": qué clase de automatización contesta
	Sistema   bool   // contestó la automatización de fábrica (el Saludo)
}

func (r resultado) decision() motor.Decision {
	return motor.Decision{Respuestas: r.Salidas, Coincidio: r.Coincidio, PasarAHumano: r.Humano, Omitir: r.Omitir, Sesion: r.Sesion, Termino: r.Termino, Etiqueta: r.Etiqueta, Sistema: r.Sistema}
}

// etiquetaDe dice qué clase de automatización es: un comando propio, un menú interactivo o una regla de respuesta suelta.
func etiquetaDe(def *Def) string {
	switch {
	case def.Propio != nil:
		return "comando"
	case interactivo(def):
		return "menu"
	}
	return "regla"
}

func (r *resultado) texto(t string) {
	if strings.TrimSpace(t) != "" {
		r.Salidas = append(r.Salidas, motor.Salida{Texto: t})
	}
}

// expiraDefecto es la caducidad en silencio que vale si el menú no trae la suya.
func (m *Motor) expiraDefecto(grupo bool) time.Duration {
	if grupo {
		if n := m.ajusteInt("grupo_expira_min"); n > 0 {
			return time.Duration(n) * time.Minute
		}
		return 10 * time.Minute
	}
	return m.c.Expira()
}

// expira es la caducidad en silencio de una conversación de este menú.
func (m *Motor) expira(def *Def, grupo bool) time.Duration {
	if a := def.Ajustes; a != nil {
		if grupo && a.ExpiraGrupoMin > 0 {
			return time.Duration(a.ExpiraGrupoMin) * time.Minute
		}
		if !grupo && a.ExpiraMin > 0 {
			return time.Duration(a.ExpiraMin) * time.Minute
		}
	}
	return m.expiraDefecto(grupo)
}

// procesa lleva un mensaje por el menú: continúa la conversación abierta de esa persona o busca una automatización que se active.
// forzarMenu > 0 (simulador) empieza ese menú sin revisar disparadores.
func (m *Motor) procesa(e entrada, prueba bool, forzarMenu int64) (resultado, error) {
	ahora := m.c.Ahora()
	bot, alm := m.c.BotID, m.c.Alm
	// Grupos: un mensaje que cita un menú del bot le habla al bot. Si el menú es de otra persona, se ignora (salvo el ajuste).
	var ref *almacen.MsgBot
	if e.EsGrupo && e.CitaID != "" && !prueba {
		if ref = alm.MsgBotPorID(bot, e.Chat, e.CitaID); ref != nil {
			e.Dirigido = true
			if ref.Persona != e.Sender {
				if !m.cualquieraPuede(m.definicionDeMenu(ref.MenuID)) {
					return resultado{Omitir: "menú de otra persona"}, nil
				}
				e.Quien, e.Sender = e.Sender, ref.Persona
			}
		}
	}
	s, err := alm.SesionVigente(bot, e.Chat, e.Sender, prueba, ahora, m.expiraDefecto(e.EsGrupo))
	if err != nil {
		return resultado{}, err
	}
	if s != nil && (s.Estado == almacen.SesHumano || s.Estado == almacen.SesPausada) {
		return resultado{Omitir: almacen.MotHumano}, nil
	}
	if s != nil {
		def := m.definicion(s.MenuID, s.MenuVersion)
		if def == nil { // la automatización ya no existe
			_ = alm.CerrarSesion(bot, s, almacen.SesAbandonada, 0, ahora)
			s = nil
		} else if cmd := m.comandoDe(def.Disparadores.Comando); cmd != "" && !e.NoTexto && comandoCoincide(norm(e.Texto), cmd) {
			_ = alm.CerrarSesion(bot, s, almacen.SesAbandonada, 0, ahora) // escribió el comando otra vez: empieza de nuevo
			s = nil
		} else {
			if e.EsGrupo && !e.Dirigido && !m.validaSinDirigir(def, s, e) {
				return resultado{Omitir: almacen.MotGrupo}, nil
			}
			r, err := m.continua(def, s, e, prueba)
			r.Etiqueta = etiquetaDe(def)
			// Una conversación de menú que termina bien (no un comando de una sola respuesta) abre paso al mensaje de cierre.
			r.Termino = err == nil && !prueba && r.Estado == almacen.SesCompletada && def.Propio == nil
			return r, err
		}
	}
	if e.EsGrupo && !e.Dirigido {
		return resultado{Omitir: almacen.MotGrupo}, nil
	}
	if e.NoTexto {
		return resultado{Omitir: almacen.MotNoTexto}, nil
	}
	// Sin conversación abierta: ¿alguna automatización se activa con este mensaje?
	menus, err := alm.ListarMenus(bot)
	if err != nil {
		return resultado{}, err
	}
	pideMenu := m.pideMenu(e)
	for _, mn := range menus {
		if forzarMenu > 0 && mn.ID != forzarMenu {
			continue
		}
		if forzarMenu == 0 && !mn.Activo {
			continue
		}
		if forzarMenu == 0 && m.c.MenuVisible != nil && !m.c.MenuVisible(e.Sender, mn.ID) {
			continue
		}
		def, err := Parse(mn.Def)
		if err != nil || len(def.Pasos) == 0 || def.Paso(def.Inicio) == nil {
			continue
		}
		if forzarMenu == 0 && def.Propio != nil && def.Propio.RangoMin > 0 && m.c.PuedeUsar != nil && !m.c.PuedeUsar(e.Sender, def.Propio.RangoMin) {
			continue
		}
		if forzarMenu == 0 && !m.seActiva(def, mn, e, ahora) && !(pideMenu && m.sirveParaMenu(def, mn, e, ahora)) {
			continue
		}
		r, err := m.inicia(def, mn, e, prueba, ahora, forzarMenu == 0)
		r.Etiqueta = etiquetaDe(def)
		r.Sistema = mn.Sistema && !prueba && forzarMenu == 0
		return r, err
	}
	// Citaron un menú que ya terminó: se les dice una sola vez (por persona y menú).
	if ref != nil && !prueba && !m.pareceComandoDeMenu(e) {
		if alm.MarcarVencido(bot, e.Chat, e.Sender, ref.MenuID, ahora) {
			t := strings.NewReplacer("{prefijo}", m.prefijo()).Replace(m.ajuste("grupo_txt_vencido"))
			r := resultado{Coincidio: "conversación terminada"}
			r.texto(t)
			return r, nil
		}
		return resultado{Omitir: "conversación terminada (ya se avisó)"}, nil
	}
	return resultado{}, nil
}

func (m *Motor) definicionDeMenu(menuID int64) *Def {
	mn, err := m.c.Alm.ObtenerMenu(m.c.BotID, menuID)
	if err != nil || mn == nil {
		return nil
	}
	d, err := Parse(mn.Def)
	if err != nil {
		return nil
	}
	return d
}

// cualquieraPuede: en ese menú, cualquier persona del grupo puede contestar el menú de otra.
func (m *Motor) cualquieraPuede(def *Def) bool {
	if def != nil && def.Ajustes != nil {
		switch def.Ajustes.Cualquiera {
		case "si":
			return true
		case "no":
			return false
		}
	}
	return m.ajuste("grupo_cualquiera") == "1"
}

func (m *Motor) maxSesiones(def *Def) int {
	if def.Ajustes != nil && def.Ajustes.MaxSesionesGrupo > 0 {
		return def.Ajustes.MaxSesionesGrupo
	}
	if n := m.ajusteInt("grupo_max_sesiones"); n > 0 {
		return n
	}
	return 5
}

// pideMenu: la persona escribió el comando del menú CON el símbolo («!menu») para abrirlo aunque ninguna regla se active. La palabra
// «menu» suelta no abre nada: así no se activa por error en medio de una conversación normal.
func (m *Motor) pideMenu(e entrada) bool {
	return !e.NoTexto && m.conSimbolo(e.Texto) && m.comandoSuelto(e.Texto) == CmdMenu
}

// conSimbolo dice si el texto empieza con el símbolo de los comandos (el configurado o «!»).
func (m *Motor) conSimbolo(texto string) bool {
	t := strings.TrimSpace(texto)
	return strings.HasPrefix(t, m.prefijo()) || strings.HasPrefix(t, "!")
}

func (m *Motor) pareceComandoDeMenu(e entrada) bool {
	c := m.comandoSuelto(e.Texto)
	return c == CmdMenu || c == CmdInicio
}

// sirveParaMenu: este menú se puede abrir con "menú": es un menú interactivo (no un comando propio ni una respuesta suelta) y
// la persona puede usarlo aquí y ahora.
func (m *Motor) sirveParaMenu(def *Def, mn almacen.Menu, e entrada, ahora time.Time) bool {
	if def.Propio != nil || mn.Sistema {
		return false
	}
	if p := def.Paso(def.Inicio); p == nil || (p.Tipo != PasoMenu && p.Tipo != PasoForm) {
		return false
	}
	if !m.cabeEnAlcance(def, e) {
		return false
	}
	return def.Horario.contiene(ahora.In(m.c.Zona()))
}

// cabeEnAlcance: el menú vale en este chat (privado, chats elegidos, grupos elegidos o cualquier grupo).
func (m *Motor) cabeEnAlcance(def *Def, e entrada) bool {
	if e.EsGrupo {
		if def.Alcance.TodosGrupos {
			return true
		}
		for _, g := range def.Alcance.Grupos {
			if g == e.Chat {
				return true
			}
		}
		return false
	}
	if def.Alcance.Privados {
		return true
	}
	for _, c := range def.Alcance.Chats {
		if c == e.Chat {
			return true
		}
	}
	return false
}

func interactivo(def *Def) bool {
	p := def.Paso(def.Inicio)
	return p != nil && (p.Tipo == PasoMenu || p.Tipo == PasoPedir || p.Tipo == PasoForm)
}

// inicia abre una conversación nueva con ese menú (o retoma la que caducó, o pone a la persona en la fila si el grupo está lleno).
func (m *Motor) inicia(def *Def, mn almacen.Menu, e entrada, prueba bool, ahora time.Time, normal bool) (resultado, error) {
	bot, alm := m.c.BotID, m.c.Alm
	// Grupo lleno: fila de espera (solo si es un menú que deja la conversación abierta).
	if e.EsGrupo && !prueba && normal && interactivo(def) {
		if alm.ContarSesionesGrupo(bot, e.Chat, ahora, m.expiraDefecto(true)) >= m.maxSesiones(def) {
			turno, nueva := alm.Encolar(bot, e.Chat, e.Sender, e.WA, e.MsgID, mn.ID, ahora)
			if !nueva {
				return resultado{Omitir: "en la fila de espera"}, nil
			}
			r := resultado{Coincidio: def.Nombre}
			r.texto(strings.NewReplacer("{n}", fmt.Sprint(turno)).Replace(m.ajuste("grupo_txt_turno")))
			return r, nil
		}
	}
	// Retomar donde iba, si el menú lo pide y la conversación caducó hace poco.
	if normal && !prueba && m.volver(def) == "retomar" && !m.esComandoDeInicio(def, e) {
		horas := m.ajusteInt("menu_retoma_horas")
		if horas <= 0 {
			horas = 24
		}
		if old := alm.UltimaExpirada(bot, e.Chat, e.Sender, mn.ID, ahora.Add(-time.Duration(horas)*time.Hour)); old != nil {
			if odef := m.definicion(old.MenuID, old.MenuVersion); odef != nil {
				if p := odef.Paso(old.Paso); p != nil && (p.Tipo == PasoMenu || p.Tipo == PasoPedir || p.Tipo == PasoForm) {
					if err := alm.ReabrirSesion(old, ahora); err == nil {
						r := resultado{Coincidio: odef.Nombre, Estado: almacen.SesActiva, Paso: p.ID}
						r.texto("Seguimos donde ibas.")
						r.Salidas = append(r.Salidas, m.mostrarPaso(odef, old, p, e)...)
						r.Sesion = &motor.RefSesion{ID: old.ID, Persona: e.Sender, MenuID: old.MenuID}
						return r, nil
					}
				}
			}
		}
	}
	id, err := alm.NuevaSesionConExpira(bot, e.Chat, e.Sender, mn.ID, mn.Version, def.Inicio, prueba, ahora, m.expira(def, e.EsGrupo))
	if err != nil {
		return resultado{}, err
	}
	if e.EsGrupo {
		alm.LimpiarVencido(bot, e.Chat, e.Sender, mn.ID)
		alm.QuitarPersonaDeCola(bot, e.Chat, e.Sender)
	}
	ns := &almacen.Sesion{ID: id, Chat: e.Chat, Sender: e.Sender, MenuID: mn.ID, MenuVersion: mn.Version, Paso: def.Inicio, InicioTS: ahora.Unix(), UltimaTS: ahora.Unix(),
		Estado: almacen.SesActiva, Prueba: prueba, ExpiraSeg: int64(m.expira(def, e.EsGrupo).Seconds())}
	r, err := m.ejecuta(def, ns, def.Inicio, e, prueba)
	r.Coincidio = def.Nombre
	return r, err
}

// esComandoDeInicio: la persona escribió el comando del menú (".menu") o pidió "inicio": eso siempre empieza de cero.
func (m *Motor) esComandoDeInicio(def *Def, e entrada) bool {
	if e.NoTexto {
		return false
	}
	if cmd := m.comandoDe(def.Disparadores.Comando); cmd != "" && comandoCoincide(norm(e.Texto), cmd) {
		return true
	}
	return m.comandoSuelto(e.Texto) == CmdInicio
}

func (m *Motor) volver(def *Def) string {
	if def.Ajustes != nil && def.Ajustes.Volver != "" {
		return def.Ajustes.Volver
	}
	return m.ajuste("menu_volver")
}

// comandoDe da el comando tal como lo escribe la gente: si empieza con una letra o un número usa el prefijo actual de los comandos
// ("horario" → "!horario"); si trae su propio símbolo (".menu") se deja igual.
func (m *Motor) comandoDe(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return ""
	}
	if r := []rune(c)[0]; r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r > 127 {
		pref := "!"
		if m.c.Prefijo != nil {
			if p := m.c.Prefijo(); p != "" {
				pref = p
			}
		}
		c = pref + c
	}
	return norm(c)
}

func comandoCoincide(texto, cmd string) bool {
	return texto == cmd || strings.HasPrefix(texto, cmd+" ")
}

// seActiva comprueba alcance, horario y disparadores de una automatización.
func (m *Motor) seActiva(def *Def, mn almacen.Menu, e entrada, ahora time.Time) bool {
	// dónde
	if e.EsGrupo {
		ok := def.Alcance.TodosGrupos
		for _, g := range def.Alcance.Grupos {
			if g == e.Chat {
				ok = true
			}
		}
		if !ok {
			return false
		}
	} else if !def.Alcance.Privados {
		ok := false
		for _, c := range def.Alcance.Chats {
			if c == e.Chat {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	// cuándo (días y horas)
	enHorario := def.Horario.contiene(ahora.In(m.c.Zona()))
	t := def.Disparadores
	if t.FueraDeHorario {
		if enHorario {
			if !t.otroDisparador() {
				return false
			}
		} else {
			return m.enfriado(mn, e, t, ahora) // fuera de horario: se activa con cualquier mensaje
		}
	} else if !enHorario {
		return false
	}
	texto := norm(e.Texto)
	todo := norm(e.TextoCompleto)
	ok := false
	switch {
	case t.Cualquiera:
		ok = true
	case t.Comando != "" && comandoCoincide(texto, m.comandoDe(t.Comando)):
		ok = true
	case coincidePalabra(texto, t.Palabras) || (todo != "" && coincidePalabra(todo, t.Palabras)):
		ok = true
	case t.Mencion && e.EsGrupo && e.Mencionado:
		ok = true
	case !e.EsGrupo && t.PrimerMensaje && m.c.Alm.UltimoMensajeAnterior(m.c.BotID, e.Chat, "", e.PrimerTS.Unix()) == 0:
		ok = true
	case !e.EsGrupo && t.DiasSinEscribir > 0:
		if ant := m.c.Alm.UltimoMensajeAnterior(m.c.BotID, e.Chat, "", e.PrimerTS.Unix()); ant > 0 && ahora.Sub(time.Unix(ant, 0)) >= time.Duration(t.DiasSinEscribir)*24*time.Hour {
			ok = true
		}
	}
	return ok && m.enfriado(mn, e, t, ahora)
}

func (t Disparadores) otroDisparador() bool {
	return t.Cualquiera || t.Comando != "" || len(t.Palabras) > 0 || t.PrimerMensaje || t.DiasSinEscribir > 0 || t.Mencion
}

// enfriado: true si ya pasó el tiempo de espera entre dos activaciones para el mismo chat.
func (m *Motor) enfriado(mn almacen.Menu, e entrada, t Disparadores, ahora time.Time) bool {
	if t.EnfriamientoSeg <= 0 {
		return true
	}
	var ult int64
	_ = m.c.Alm.UltimaActivacion(m.c.BotID, mn.ID, e.Chat, e.Sender, &ult)
	return ult == 0 || ahora.Sub(time.Unix(ult, 0)) >= time.Duration(t.EnfriamientoSeg)*time.Second
}

func coincidePalabra(texto string, palabras []string) bool {
	for _, p := range palabras {
		p = norm(p)
		if p == "" {
			continue
		}
		if strings.Contains(" "+texto+" ", " "+p+" ") || (strings.Contains(p, " ") && strings.Contains(texto, p)) {
			return true
		}
	}
	return false
}

func (h Horario) contiene(t time.Time) bool {
	if len(h.Dias) > 0 {
		dia := int(t.Weekday())
		if dia == 0 {
			dia = 7
		}
		ok := false
		for _, d := range h.Dias {
			if d == dia {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	if h.Desde == "" && h.Hasta == "" {
		return true
	}
	min := t.Hour()*60 + t.Minute()
	d, h2 := minutos(h.Desde, 0), minutos(h.Hasta, 24*60)
	if d <= h2 {
		return min >= d && min < h2
	}
	return min >= d || min < h2 // horario que cruza la medianoche
}

func minutos(s string, defecto int) int {
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
		return defecto
	}
	return h*60 + m
}

// definicion carga la versión de una automatización con la que arrancó una conversación.
func (m *Motor) definicion(menuID int64, version int) *Def {
	b := m.c.Alm.VersionDeMenu(menuID, version)
	if b == nil {
		return nil
	}
	d, err := Parse(b)
	if err != nil {
		return nil
	}
	return d
}

// ---------- simulador ----------

// Prueba es lo que contesta el simulador a un mensaje falso.
type Prueba struct {
	Salidas  []string `json:"salidas"`
	Estado   string   `json:"estado"` // activo, completado, humano, abandonado
	Paso     string   `json:"paso"`
	Omitido  string   `json:"omitido"` // si el bot se habría quedado callado
	Coincide bool     `json:"coincide"`
}

const chatPrueba = "prueba:simulador@botchat"

// Probar escribe un mensaje falso a una automatización como si lo mandara otra persona. Con el mismo motor, pero marcado como
// prueba en las estadísticas; no manda nada a WhatsApp ni crea notificaciones. `reiniciar` empieza de cero.
func (m *Motor) Probar(menuID int64, texto string, reiniciar bool, nombre string) (Prueba, error) {
	alm, bot := m.c.Alm, m.c.BotID
	mn, err := alm.ObtenerMenu(bot, menuID)
	if err != nil || mn == nil {
		return Prueba{}, fmt.Errorf("Esa automatización ya no existe.")
	}
	sender := fmt.Sprintf("prueba-%d", menuID)
	if reiniciar {
		if s, _ := alm.SesionVigente(bot, chatPrueba, sender, true, m.c.Ahora(), m.c.Expira()); s != nil {
			_ = alm.CerrarSesion(bot, s, almacen.SesAbandonada, 0, m.c.Ahora())
		}
	}
	if reiniciar && strings.TrimSpace(texto) == "" {
		return Prueba{Estado: almacen.SesActiva}, nil // solo borra la conversación de prueba; no empieza otra
	}
	e := entrada{Chat: chatPrueba, Sender: sender, Texto: texto, TS: m.c.Ahora(), PrimerTS: m.c.Ahora(), Dirigido: true}
	// Si no hay una conversación de prueba abierta, se ve si el mensaje activaría la automatización de verdad.
	pr := Prueba{Salidas: []string{}}
	if def, err := Parse(mn.Def); err == nil {
		pr.Coincide = m.seActiva(def, *mn, entrada{Chat: "prueba-privado", Sender: "prueba-privado", Texto: texto, TS: e.TS, PrimerTS: e.TS}, e.TS) || reiniciar
	}
	r, err := m.procesa(e, true, menuID)
	if err != nil {
		return Prueba{}, err
	}
	pr.Salidas = []string{}
	for _, x := range r.Salidas {
		pr.Salidas = append(pr.Salidas, describeSalida(x))
	}
	pr.Estado, pr.Paso, pr.Omitido = r.Estado, r.Paso, r.Omitir
	if pr.Estado == "" {
		pr.Estado = almacen.SesActiva
	}
	return pr, nil
}
