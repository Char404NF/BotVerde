package flujo

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
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

// Textos fijos que ve la persona.
const (
	textoNoEntendi = "No entendí esa opción. Escribe el número de una de las opciones."
	textoMenuPrinc = "Volver al menú principal"
	umbralOpcion   = 24 // un mensaje más largo que esto no se toma como "elegir una opción"
)

var reNumero = regexp.MustCompile(`^\s*(\d{1,2})\b`)

func norm(s string) string { return identidad.Normalizar(s) }

// ---------- entrada del motor ----------

// Decidir implementa motor.Decisor.
func (m *Motor) Decidir(_ context.Context, ultimo motor.Mensaje, lote []motor.Mensaje) (motor.Decision, error) {
	sender := ultimo.Chat
	if ultimo.EsGrupo {
		sender = ultimo.Remitente
	}
	entrada := entrada{Chat: ultimo.Chat, Sender: sender, Texto: ultimo.Texto, EsGrupo: ultimo.EsGrupo, Mencionado: ultimo.Mencionado, TS: ultimo.TS}
	if len(lote) > 1 {
		var b strings.Builder
		for i, x := range lote {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(x.Texto)
		}
		entrada.TextoCompleto = b.String()
		if lote[0].TS.Before(entrada.TS) {
			entrada.PrimerTS = lote[0].TS
		}
	}
	if entrada.PrimerTS.IsZero() {
		entrada.PrimerTS = ultimo.TS
	}
	r, err := m.procesa(entrada, false, 0)
	if err != nil {
		return motor.Decision{}, err
	}
	return r.decision(), nil
}

type entrada struct {
	Chat, Sender  string
	Texto         string
	TextoCompleto string // todo lo que escribió en una ráfaga
	EsGrupo       bool
	Mencionado    bool
	TS            time.Time
	PrimerTS      time.Time
}

type resultado struct {
	Salidas   []string
	Coincidio string
	Humano    bool
	Omitir    string
	Estado    string // estado de la conversación al terminar este turno
	Paso      string
}

func (r resultado) decision() motor.Decision {
	d := motor.Decision{Coincidio: r.Coincidio, PasarAHumano: r.Humano, Omitir: r.Omitir}
	for _, s := range r.Salidas {
		d.Respuestas = append(d.Respuestas, motor.Salida{Texto: s})
	}
	return d
}

// procesa lleva un mensaje por el menú: continúa la conversación abierta de esa persona o busca una automatización que se active.
// forzarMenu > 0 (simulador) empieza ese menú sin revisar disparadores.
func (m *Motor) procesa(e entrada, prueba bool, forzarMenu int64) (resultado, error) {
	ahora := m.c.Ahora()
	bot, alm := m.c.BotID, m.c.Alm
	s, err := alm.SesionVigente(bot, e.Chat, e.Sender, prueba, ahora, m.c.Expira())
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
		} else if cmd := norm(def.Disparadores.Comando); cmd != "" && comandoCoincide(norm(e.Texto), cmd) {
			_ = alm.CerrarSesion(bot, s, almacen.SesAbandonada, 0, ahora) // escribió el comando otra vez: empieza de nuevo
			s = nil
		} else {
			return m.continua(def, s, e, prueba)
		}
	}
	// Sin conversación abierta: ¿alguna automatización se activa con este mensaje?
	menus, err := alm.ListarMenus(bot)
	if err != nil {
		return resultado{}, err
	}
	for _, mn := range menus {
		if forzarMenu > 0 && mn.ID != forzarMenu {
			continue
		}
		if forzarMenu == 0 && !mn.Activo {
			continue
		}
		def, err := Parse(mn.Def)
		if err != nil || len(def.Pasos) == 0 || def.Paso(def.Inicio) == nil {
			continue
		}
		if forzarMenu == 0 && !m.seActiva(def, mn, e, ahora) {
			continue
		}
		id, err := alm.NuevaSesion(bot, e.Chat, e.Sender, mn.ID, mn.Version, def.Inicio, prueba, ahora)
		if err != nil {
			return resultado{}, err
		}
		ns := &almacen.Sesion{ID: id, Chat: e.Chat, Sender: e.Sender, MenuID: mn.ID, MenuVersion: mn.Version, Paso: def.Inicio, InicioTS: ahora.Unix(), UltimaTS: ahora.Unix(), Estado: almacen.SesActiva, Prueba: prueba}
		r, err := m.ejecuta(def, ns, def.Inicio, e, prueba)
		r.Coincidio = def.Nombre
		return r, err
	}
	return resultado{}, nil
}

func comandoCoincide(texto, cmd string) bool {
	return texto == cmd || strings.HasPrefix(texto, cmd+" ")
}

// seActiva comprueba alcance, horario y disparadores de una automatización.
func (m *Motor) seActiva(def *Def, mn almacen.Menu, e entrada, ahora time.Time) bool {
	// dónde
	if e.EsGrupo {
		ok := false
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
	case t.Comando != "" && comandoCoincide(texto, norm(t.Comando)):
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

// ---------- continuar una conversación ----------

func (m *Motor) continua(def *Def, s *almacen.Sesion, e entrada, prueba bool) (resultado, error) {
	ahora := m.c.Ahora()
	bot, alm := m.c.BotID, m.c.Alm
	p := def.Paso(s.Paso)
	if p == nil { // le borraron el paso: vuelve al menú principal
		return m.ejecuta(def, s, def.Inicio, e, prueba)
	}
	ev := func(paso, opcion, res string) struct{ Paso, Opcion, Resultado string } {
		return struct{ Paso, Opcion, Resultado string }{paso, opcion, res}
	}
	texto := strings.TrimSpace(e.Texto)
	if s.EsperaDato && p.Tipo == PasoPedir {
		valor := texto
		if e.TextoCompleto != "" {
			valor = strings.TrimSpace(e.TextoCompleto)
		}
		_ = alm.GuardarVariable(s.ID, p.Variable, valor)
		_ = alm.MoverSesion(bot, s, p.ID, false, 0, false, ev(p.ID, "dato", "valida"), ahora)
		if p.Siguiente == "" {
			_ = alm.CerrarSesion(bot, s, almacen.SesCompletada, 0, ahora)
			return resultado{Estado: almacen.SesCompletada}, nil
		}
		return m.ejecuta(def, s, p.Siguiente, e, prueba)
	}
	if p.Tipo != PasoMenu {
		// El paso no espera nada (la conversación quedó en un final): se vuelve a empezar desde el menú principal.
		return m.ejecuta(def, s, def.Inicio, e, prueba)
	}
	// "0" siempre vuelve al menú principal.
	if n, ok := numeroDeOpcion(texto); ok && n == 0 {
		_ = alm.MoverSesion(bot, s, s.Paso, false, 0, false, ev(p.ID, "0", "valida"), ahora)
		return m.ejecuta(def, s, def.Inicio, e, prueba)
	}
	if o, idx, ok := elegida(p, texto); ok {
		_ = alm.MoverSesion(bot, s, p.ID, false, 0, false, ev(p.ID, strconv.Itoa(idx), "valida"), ahora)
		if o.Destino == "" || def.Paso(o.Destino) == nil {
			return m.ejecuta(def, s, def.Inicio, e, prueba)
		}
		return m.ejecuta(def, s, o.Destino, e, prueba)
	}
	// respuesta inválida
	inv := s.Invalidas + 1
	switch {
	case s.Cortado:
		// ya se reenvió el menú y sigue sin entender: se corta el ciclo y se deja de insistir
		_ = alm.MoverSesion(bot, s, p.ID, false, inv, true, ev(p.ID, "", "invalida"), ahora)
		_ = alm.CerrarSesion(bot, s, almacen.SesAbandonada, 0, ahora)
		return resultado{Omitir: "ciclo cortado", Estado: almacen.SesAbandonada}, nil
	case inv >= 2:
		_ = alm.MoverSesion(bot, s, p.ID, false, 0, true, ev(p.ID, "", "invalida"), ahora)
		return resultado{Salidas: []string{m.menuTexto(def, p, s)}, Estado: almacen.SesActiva, Paso: p.ID}, nil
	default:
		_ = alm.MoverSesion(bot, s, p.ID, false, inv, false, ev(p.ID, "", "invalida"), ahora)
		return resultado{Salidas: []string{textoNoEntendi}, Estado: almacen.SesActiva, Paso: p.ID}, nil
	}
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

// elegida resuelve qué opción eligió la persona: por número o escribiendo el texto de la opción.
func elegida(p *Paso, texto string) (Opcion, int, bool) {
	if n, ok := numeroDeOpcion(texto); ok && n >= 1 && n <= len(p.Opciones) {
		return p.Opciones[n-1], n, true
	}
	t := norm(texto)
	for i, o := range p.Opciones {
		if t != "" && norm(o.Texto) == t {
			return o, i + 1, true
		}
	}
	return Opcion{}, 0, false
}

// ---------- ejecutar pasos ----------

func (m *Motor) ejecuta(def *Def, s *almacen.Sesion, id string, e entrada, prueba bool) (resultado, error) {
	ahora := m.c.Ahora()
	bot, alm := m.c.BotID, m.c.Alm
	var r resultado
	r.Estado = almacen.SesActiva
	vars := alm.Variables(s.ID)
	for guarda := 0; guarda < 25; guarda++ {
		p := def.Paso(id)
		if p == nil {
			if id != def.Inicio {
				id = def.Inicio
				continue
			}
			_ = alm.CerrarSesion(bot, s, almacen.SesCompletada, 0, ahora)
			r.Estado = almacen.SesCompletada
			return r, nil
		}
		alm.VisitaPaso(bot, s, p.ID, ahora)
		r.Paso = p.ID
		switch p.Tipo {
		case PasoMenu:
			_ = alm.MoverSesion(bot, s, p.ID, false, 0, false, struct{ Paso, Opcion, Resultado string }{p.ID, "", "mostrado"}, ahora)
			r.Salidas = append(r.Salidas, m.menuTexto(def, p, s))
			return r, nil
		case PasoRespuesta:
			r.Salidas = append(r.Salidas, m.sustituye(p.Texto, e, vars))
			if p.Notificar && !prueba && m.c.AlAviso != nil {
				m.c.AlAviso(e.Chat, "solicitud", "Nueva solicitud", m.detalleAviso(def, p, e, vars))
			}
			if p.Siguiente == "" {
				_ = alm.CerrarSesion(bot, s, almacen.SesCompletada, 0, ahora)
				r.Estado = almacen.SesCompletada
				return r, nil
			}
			id = p.Siguiente
		case PasoPedir:
			_ = alm.MoverSesion(bot, s, p.ID, true, 0, false, struct{ Paso, Opcion, Resultado string }{p.ID, "", "mostrado"}, ahora)
			r.Salidas = append(r.Salidas, m.sustituye(p.Pregunta, e, vars))
			return r, nil
		case PasoHumano:
			if strings.TrimSpace(p.Texto) != "" {
				r.Salidas = append(r.Salidas, m.sustituye(p.Texto, e, vars))
			}
			_ = alm.CerrarSesion(bot, s, almacen.SesHumano, ahora.Add(12*time.Hour).Unix(), ahora)
			r.Estado, r.Humano = almacen.SesHumano, true
			if !prueba && m.c.AlAviso != nil {
				m.c.AlAviso(e.Chat, "humano", "Alguien pidió hablar con una persona", m.detalleHumano(e))
			}
			return r, nil
		case PasoFin:
			if strings.TrimSpace(p.Texto) != "" {
				r.Salidas = append(r.Salidas, m.sustituye(p.Texto, e, vars))
			}
			_ = alm.CerrarSesion(bot, s, almacen.SesCompletada, 0, ahora)
			r.Estado = almacen.SesCompletada
			return r, nil
		default:
			_ = alm.CerrarSesion(bot, s, almacen.SesAbandonada, 0, ahora)
			return r, nil
		}
	}
	// demasiados pasos encadenados sin esperar a nadie: se corta
	_ = alm.CerrarSesion(bot, s, almacen.SesAbandonada, 0, ahora)
	r.Estado = almacen.SesAbandonada
	return r, nil
}

func (m *Motor) menuTexto(def *Def, p *Paso, s *almacen.Sesion) string {
	var b strings.Builder
	b.WriteString(m.sustituye(p.Titulo, entrada{Chat: s.Chat, Sender: s.Sender}, m.c.Alm.Variables(s.ID)))
	for i, o := range p.Opciones {
		fmt.Fprintf(&b, "\n%d. %s", i+1, o.Texto)
	}
	if p.ID != def.Inicio {
		fmt.Fprintf(&b, "\n0. %s", textoMenuPrinc)
	}
	return b.String()
}

var dias = []string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"}

var reVar = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

// sustituye pone {nombre}, {hora}, {dia} y los datos pedidos ({dato1}…) en un texto.
func (m *Motor) sustituye(t string, e entrada, vars map[string]string) string {
	return reVar.ReplaceAllStringFunc(t, func(x string) string {
		k := strings.ToLower(x[1 : len(x)-1])
		switch k {
		case "nombre":
			return m.c.Nombre(e.Chat, e.Sender)
		case "hora":
			return m.c.Ahora().In(m.c.Zona()).Format("15:04")
		case "dia", "día":
			return dias[int(m.c.Ahora().In(m.c.Zona()).Weekday())]
		}
		if v, ok := vars[k]; ok {
			return v
		}
		return ""
	})
}

func (m *Motor) detalleAviso(def *Def, p *Paso, e entrada, vars map[string]string) string {
	if strings.TrimSpace(p.Aviso) != "" {
		return m.sustituye(p.Aviso, e, vars)
	}
	var b strings.Builder
	b.WriteString(m.c.Nombre(e.Chat, e.Sender))
	for _, q := range def.Pasos {
		if q.Tipo == PasoPedir {
			fmt.Fprintf(&b, "\n%s", vars[q.Variable])
		}
	}
	return strings.TrimSpace(b.String())
}

func (m *Motor) detalleHumano(e entrada) string {
	n := m.c.Nombre(e.Chat, e.Sender)
	if c, _ := m.c.Alm.ObtenerChat(m.c.BotID, e.Chat); c != nil && c.TelMostrado != "" {
		return strings.TrimSpace(n + " · " + c.TelMostrado)
	}
	return n
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
	e := entrada{Chat: chatPrueba, Sender: sender, Texto: texto, TS: m.c.Ahora(), PrimerTS: m.c.Ahora()}
	// Si no hay una conversación de prueba abierta, se ve si el mensaje activaría la automatización de verdad.
	pr := Prueba{}
	if def, err := Parse(mn.Def); err == nil {
		pr.Coincide = m.seActiva(def, *mn, entrada{Chat: "prueba-privado", Sender: "prueba-privado", Texto: texto, TS: e.TS, PrimerTS: e.TS}, e.TS) || reiniciar
	}
	r, err := m.procesa(e, true, menuID)
	if err != nil {
		return Prueba{}, err
	}
	pr.Salidas, pr.Estado, pr.Paso, pr.Omitido = r.Salidas, r.Estado, r.Paso, r.Omitir
	if pr.Estado == "" {
		pr.Estado = almacen.SesActiva
	}
	return pr, nil
}
