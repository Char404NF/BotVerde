package grupos

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"botchat/internal/almacen"
)

// Filtros del grupo: enlaces (con lista de dominios permitidos), palabras prohibidas, mensajes repetidos y flood (muchos mensajes en
// poco tiempo). Cada uno se enciende por grupo y tiene su acción y sus excepciones. Se evalúan en el trabajador (nunca en el handler
// de eventos) y la acción de borrar o expulsar solo ocurre si soy administrador; si no, solo se alerta con una notificación local.

// Nombres de los filtros.
const (
	FiltroEnlaces   = "enlaces"
	FiltroPalabras  = "palabras"
	FiltroRepetidos = "repetidos"
	FiltroFlood     = "flood"
)

// Filtros en el orden en que se muestran y evalúan.
var Filtros = []string{FiltroEnlaces, FiltroPalabras, FiltroRepetidos, FiltroFlood}

// Acciones de un filtro.
const (
	AccAvisarme    = "avisarme"     // solo una notificación en mi teléfono
	AccAvisarGrupo = "avisar_grupo" // respondo en el grupo al mensaje (aviso amable)
	AccBorrar      = "borrar"       // borro el mensaje (solo siendo administrador)
	AccAdvertir    = "advertir"     // pongo una advertencia (al tope se aplica la regla de advertencias)
	AccSilenciar   = "silenciar"    // el bot deja de contestarle a esa persona por un rato
	AccExpulsar    = "expulsar"     // la saco del grupo (solo siendo administrador)
)

// Acciones válidas, en el orden de la pantalla.
var Acciones = []string{AccAvisarme, AccAvisarGrupo, AccBorrar, AccAdvertir, AccSilenciar, AccExpulsar}

// EsAccionFiltro dice si es una acción válida.
func EsAccionFiltro(a string) bool {
	for _, x := range Acciones {
		if x == a {
			return true
		}
	}
	return false
}

// EsFiltro dice si es el nombre de un filtro.
func EsFiltro(f string) bool {
	for _, x := range Filtros {
		if x == f {
			return true
		}
	}
	return false
}

// ConfigFiltro son las opciones de un filtro (se guardan cifradas como JSON).
type ConfigFiltro struct {
	ExentoAdmins bool     `json:"exentoAdmins"` // los administradores del grupo no cuentan
	ExentoRangos []int64  `json:"exentoRangos"` // ids de rangos que no cuentan (el Dueño nunca cuenta)
	Permitidos   []string `json:"permitidos"`   // enlaces: dominios permitidos (youtube.com permite también m.youtube.com)
	Palabras     []string `json:"palabras"`     // palabras o frases prohibidas
	Max          int      `json:"max"`          // repetidos: veces seguidas; flood: mensajes
	Segundos     int      `json:"segundos"`     // repetidos y flood: ventana de tiempo
	SilencioMin  int      `json:"silencioMin"`  // acción silenciar: minutos
	Aviso        string   `json:"aviso"`        // acción avisar al grupo: texto ("" = el de siempre)
}

// Filtro es un filtro de un grupo, con sus valores de fábrica si nunca se tocó.
type Filtro struct {
	Chat        string       `json:"chat"`
	Filtro      string       `json:"filtro"`
	Nombre      string       `json:"nombre"`
	Descripcion string       `json:"descripcion"`
	Activo      bool         `json:"activo"`
	Accion      string       `json:"accion"`
	Config      ConfigFiltro `json:"config"`
}

func nombreFiltro(f string) (nombre, desc string) {
	switch f {
	case FiltroEnlaces:
		return "Enlaces", "Detecta enlaces. Puedes dejar pasar algunos dominios."
	case FiltroPalabras:
		return "Palabras prohibidas", "Detecta palabras o frases que no quieres en el grupo."
	case FiltroRepetidos:
		return "Mensajes repetidos", "Detecta a quien manda lo mismo varias veces."
	case FiltroFlood:
		return "Flood", "Detecta a quien manda muchos mensajes en poco tiempo."
	}
	return f, ""
}

func filtroDefecto(chat, f string) Filtro {
	n, d := nombreFiltro(f)
	x := Filtro{Chat: chat, Filtro: f, Nombre: n, Descripcion: d, Accion: AccAvisarme,
		Config: ConfigFiltro{ExentoAdmins: true, ExentoRangos: []int64{}, Permitidos: []string{}, Palabras: []string{}, SilencioMin: 60}}
	switch f {
	case FiltroRepetidos:
		x.Config.Max, x.Config.Segundos = 3, 120
	case FiltroFlood:
		x.Config.Max, x.Config.Segundos = 6, 10
	}
	return x
}

type cacheFiltros struct {
	lista []Filtro
	en    time.Time
}

// FiltrosDe devuelve los cuatro filtros del grupo (con sus valores de fábrica los que nunca se tocaron).
func (s *Servicio) FiltrosDe(chat string) []Filtro {
	guardados, _ := s.Alm.FiltrosDeGrupo(s.BotID, chat)
	por := map[string]almacen.FiltroGrupo{}
	for _, g := range guardados {
		por[g.Filtro] = g
	}
	out := make([]Filtro, 0, len(Filtros))
	for _, nombre := range Filtros {
		f := filtroDefecto(chat, nombre)
		if g, ok := por[nombre]; ok {
			f.Activo = g.Activo
			if EsAccionFiltro(g.Accion) {
				f.Accion = g.Accion
			}
			if g.Config != "" {
				_ = json.Unmarshal([]byte(g.Config), &f.Config) // lo que falta conserva su valor de fábrica
			}
		}
		if f.Config.ExentoRangos == nil {
			f.Config.ExentoRangos = []int64{}
		}
		if f.Config.Permitidos == nil {
			f.Config.Permitidos = []string{}
		}
		if f.Config.Palabras == nil {
			f.Config.Palabras = []string{}
		}
		out = append(out, f)
	}
	return out
}

// filtrosActivos da los filtros encendidos de un grupo (con una caché corta: se mira en cada mensaje del grupo).
func (s *Servicio) filtrosActivos(chat string) []Filtro {
	s.mu.Lock()
	c, ok := s.filt[chat]
	s.mu.Unlock()
	if !ok || s.ahora().Sub(c.en) > 30*time.Second {
		var act []Filtro
		for _, f := range s.FiltrosDe(chat) {
			if f.Activo {
				act = append(act, f)
			}
		}
		c = cacheFiltros{lista: act, en: s.ahora()}
		s.mu.Lock()
		if len(s.filt) > 500 {
			s.filt = map[string]cacheFiltros{}
		}
		s.filt[chat] = c
		s.mu.Unlock()
	}
	return c.lista
}

// normalizaDominio deja un dominio como se compara: minúsculas, sin esquema, sin "www.", sin ruta ni puerto.
func normalizaDominio(d string) string {
	d = strings.ToLower(strings.TrimSpace(d))
	for _, p := range []string{"https://", "http://"} {
		d = strings.TrimPrefix(d, p)
	}
	if i := strings.IndexAny(d, "/?#:"); i >= 0 {
		d = d[:i]
	}
	d = strings.TrimPrefix(d, "www.")
	return strings.Trim(d, ".,;:!?)(\"'")
}

// GuardarFiltro valida y guarda un filtro de un grupo.
func (s *Servicio) GuardarFiltro(actor string, f Filtro) error {
	if _, err := parseaGrupo(f.Chat); err != nil {
		return err
	}
	if !EsFiltro(f.Filtro) {
		return ErrFiltroNoHay
	}
	if !EsAccionFiltro(f.Accion) {
		return ErrAccionNoHay
	}
	c := f.Config
	c.Permitidos = limpiaLista(c.Permitidos, 100, 100, normalizaDominio)
	c.Palabras = limpiaLista(c.Palabras, 300, 80, func(p string) string { return strings.TrimSpace(p) })
	switch f.Filtro {
	case FiltroRepetidos:
		if c.Max < 2 || c.Max > 50 {
			return fallo("las veces seguidas deben estar entre 2 y 50", nil)
		}
	case FiltroFlood:
		if c.Max < 2 || c.Max > 100 {
			return fallo("los mensajes deben estar entre 2 y 100", nil)
		}
	}
	if f.Filtro == FiltroRepetidos || f.Filtro == FiltroFlood {
		if c.Segundos < 1 || c.Segundos > 3600 {
			return fallo("los segundos deben estar entre 1 y 3600", nil)
		}
	}
	if c.SilencioMin < 1 || c.SilencioMin > 525600 {
		return fallo("los minutos de silencio deben estar entre 1 y 525600", nil)
	}
	if utf8.RuneCountInString(c.Aviso) > 300 {
		return fallo("el aviso es demasiado largo (máximo 300 letras)", nil)
	}
	c.Aviso = strings.TrimSpace(c.Aviso)
	if c.ExentoRangos == nil {
		c.ExentoRangos = []int64{}
	}
	b, _ := json.Marshal(c)
	if err := s.Alm.GuardarFiltroGrupo(s.BotID, almacen.FiltroGrupo{Chat: f.Chat, Filtro: f.Filtro, Activo: f.Activo, Accion: f.Accion, Config: string(b)}, s.ahora()); err != nil {
		s.registra(actor, "grupo.filtro", "", f.Chat, "error", f.Filtro)
		return err
	}
	s.mu.Lock()
	delete(s.filt, f.Chat)
	s.mu.Unlock()
	est := "off"
	if f.Activo {
		est = "on"
	}
	s.registra(actor, "grupo.filtro", "", f.Chat, "ok", f.Filtro+":"+est+":"+f.Accion)
	return nil
}

// RestaurarFiltro vuelve un filtro a sus valores de fábrica (apagado).
func (s *Servicio) RestaurarFiltro(actor, chat, filtro string) error {
	if !EsFiltro(filtro) {
		return ErrFiltroNoHay
	}
	if err := s.Alm.BorrarFiltroGrupo(s.BotID, chat, filtro); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.filt, chat)
	s.mu.Unlock()
	s.registra(actor, "grupo.filtro", "", chat, "ok", filtro+":restaurar")
	return nil
}

func limpiaLista(l []string, max, largo int, norm func(string) string) []string {
	out := []string{}
	visto := map[string]bool{}
	for _, x := range l {
		x = norm(x)
		if x == "" || utf8.RuneCountInString(x) > largo || visto[strings.ToLower(x)] {
			continue
		}
		visto[strings.ToLower(x)] = true
		out = append(out, x)
		if len(out) >= max {
			break
		}
	}
	return out
}

// ---------- detección ----------

var reEnlace = regexp.MustCompile(`(?i)(?:https?://|www\.)[^\s<>"']+|\b(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+(?:com|net|org|info|biz|edu|gov|io|co|me|mx|es|ar|cl|pe|ve|us|uk|app|dev|link|ly|gl|be|tv|xyz|top|site|online|store|shop|club|live|page|gg|cc|ws|vip|cloud)\b(?:/[^\s<>"']*)?`)

// DominiosDe saca los dominios de los enlaces que hay en un texto (ya normalizados). No cuenta los correos (nombre@dominio.com).
func DominiosDe(texto string) []string {
	var out []string
	for _, loc := range reEnlace.FindAllStringIndex(texto, -1) {
		if loc[0] > 0 && texto[loc[0]-1] == '@' {
			continue
		}
		if d := normalizaDominio(texto[loc[0]:loc[1]]); d != "" {
			out = append(out, d)
		}
	}
	return out
}

func dominioPermitido(d string, permitidos []string) bool {
	for _, p := range permitidos {
		if d == p || strings.HasSuffix(d, "."+p) {
			return true
		}
	}
	return false
}

// quitaAcentos pasa a minúsculas, quita acentos y deja solo letras y números separados por un espacio (para comparar palabras).
func quitaAcentos(t string) string {
	var b strings.Builder
	b.WriteByte(' ')
	espacio := true
	for _, r := range strings.ToLower(t) {
		switch r {
		case 'á', 'à', 'ä', 'â':
			r = 'a'
		case 'é', 'è', 'ë', 'ê':
			r = 'e'
		case 'í', 'ì', 'ï', 'î':
			r = 'i'
		case 'ó', 'ò', 'ö', 'ô':
			r = 'o'
		case 'ú', 'ù', 'ü', 'û':
			r = 'u'
		case 'ñ':
			r = 'n'
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			espacio = false
		} else if !espacio {
			b.WriteByte(' ')
			espacio = true
		}
	}
	if !espacio {
		b.WriteByte(' ')
	}
	return b.String()
}

// Entrada es un mensaje de grupo recién guardado que hay que revisar.
type Entrada struct {
	Chat          string
	MensajeID     string
	Persona       string // clave canónica de quien lo escribió
	PersonaWA     string // su JID tal como lo manda WhatsApp (para mencionarla)
	Nombre        string
	Texto         string
	EsMio         bool
	Sincronizando bool
}

// Infraccion es un mensaje que rompe un filtro.
type Infraccion struct {
	Entrada
	Filtro Filtro
	Motivo string // "enlace", "palabra", "repetido", "flood" (sin textos)
}

type marcaTexto struct {
	t     time.Time
	texto string
}

type historial struct {
	textos  []marcaTexto
	tiempos []time.Time
}

func (s *Servicio) historialDe(chat, persona string) *historial {
	k := chat + "\x00" + persona
	h := s.hist[k]
	if h == nil {
		if len(s.hist) > 3000 {
			s.hist = map[string]*historial{}
		}
		h = &historial{}
		s.hist[k] = h
	}
	return h
}

// Evaluar revisa un mensaje con los filtros encendidos del grupo y devuelve el primero que rompe (o nil). Es rápido y no usa la red:
// va en el trabajador. No mira lo que escribo yo ni el historial que llega al sincronizar.
func (s *Servicio) Evaluar(e Entrada) *Infraccion {
	if e.EsMio || e.Sincronizando || e.Persona == "" || !strings.HasSuffix(e.Chat, "@g.us") || strings.TrimSpace(e.Texto) == "" {
		return nil
	}
	fs := s.filtrosActivos(e.Chat)
	if len(fs) == 0 {
		return nil
	}
	var primera *Infraccion
	ahora := s.ahora()
	for _, f := range fs {
		motivo := ""
		switch f.Filtro {
		case FiltroEnlaces:
			for _, d := range DominiosDe(e.Texto) {
				if !dominioPermitido(d, f.Config.Permitidos) {
					motivo = "enlace"
					break
				}
			}
		case FiltroPalabras:
			t := quitaAcentos(e.Texto)
			for _, p := range f.Config.Palabras {
				if q := quitaAcentos(p); strings.TrimSpace(q) != "" && strings.Contains(t, q) {
					motivo = "palabra"
					break
				}
			}
		case FiltroRepetidos:
			motivo = s.cuentaRepetido(e, f, ahora)
		case FiltroFlood:
			motivo = s.cuentaFlood(e, f, ahora)
		}
		if motivo != "" && primera == nil {
			primera = &Infraccion{Entrada: e, Filtro: f, Motivo: motivo}
		}
	}
	return primera
}

func (s *Servicio) cuentaRepetido(e Entrada, f Filtro, ahora time.Time) string {
	norm := strings.TrimSpace(quitaAcentos(e.Texto))
	if norm == "" {
		return ""
	}
	ventana := time.Duration(f.Config.Segundos) * time.Second
	s.mu.Lock()
	defer s.mu.Unlock()
	h := s.historialDe(e.Chat, e.Persona)
	nuevos := h.textos[:0]
	n := 0
	for _, m := range h.textos {
		if ahora.Sub(m.t) <= ventana {
			nuevos = append(nuevos, m)
			if m.texto == norm {
				n++
			}
		}
	}
	h.textos = append(nuevos, marcaTexto{t: ahora, texto: norm})
	n++
	if n >= f.Config.Max {
		h.textos = nil // una sola vez por tanda
		return "repetido"
	}
	return ""
}

func (s *Servicio) cuentaFlood(e Entrada, f Filtro, ahora time.Time) string {
	ventana := time.Duration(f.Config.Segundos) * time.Second
	s.mu.Lock()
	defer s.mu.Unlock()
	h := s.historialDe(e.Chat, e.Persona)
	nuevos := h.tiempos[:0]
	for _, t := range h.tiempos {
		if ahora.Sub(t) <= ventana {
			nuevos = append(nuevos, t)
		}
	}
	h.tiempos = append(nuevos, ahora)
	if len(h.tiempos) >= f.Config.Max {
		h.tiempos = nil
		return "flood"
	}
	return ""
}

// ---------- aplicar la acción ----------

func (s *Servicio) nombreGrupo(chat string) string {
	if c, _ := s.Alm.ObtenerChat(s.BotID, chat); c != nil && c.Mostrado != "" {
		return c.Mostrado
	}
	return "un grupo"
}

// esAdminDe dice si esa persona administra el grupo (con la información que ya hay; si no se puede saber, no).
func (s *Servicio) esAdminDe(ctx context.Context, chat, persona string) bool {
	_, inf, err := s.preparar(ctx, chat, false)
	if err != nil {
		return false
	}
	p, ok := s.buscaMiembro(inf, persona)
	return ok && (p.IsAdmin || p.IsSuperAdmin)
}

// Exento dice si esa persona queda fuera de este filtro: el Dueño siempre; luego los rangos elegidos y los administradores del grupo.
func (s *Servicio) Exento(ctx context.Context, chat, persona string, c ConfigFiltro) bool {
	p := s.persona(persona)
	if p == "" {
		return false
	}
	if p == almacen.PersonaDueno {
		return true
	}
	if s.Perm != nil {
		if r := s.Perm.RolDe(p); r != nil {
			if r.EsDueno {
				return true
			}
			for _, id := range c.ExentoRangos {
				if r.ID == id {
					return true
				}
			}
		}
	}
	return c.ExentoAdmins && s.esAdminDe(ctx, chat, p)
}

func avisoDefecto(filtro string) string {
	switch filtro {
	case FiltroEnlaces:
		return "Aquí no se permiten enlaces."
	case FiltroPalabras:
		return "Ese mensaje tiene una palabra que no está permitida en este grupo."
	case FiltroRepetidos:
		return "Por favor no repitas el mismo mensaje."
	case FiltroFlood:
		return "Vas muy rápido: escribe con calma, por favor."
	}
	return "Ese mensaje no cumple las reglas del grupo."
}

// alerta crea la notificación local (a lo más una cada 10 minutos por persona y filtro, para no llenar el teléfono).
func (s *Servicio) alerta(inf *Infraccion, detalle string) {
	k := inf.Chat + "\x00" + inf.Persona + "\x00" + inf.Filtro.Filtro
	s.mu.Lock()
	if t, ok := s.alertas[k]; ok && s.ahora().Sub(t) < 10*time.Minute {
		s.mu.Unlock()
		return
	}
	if len(s.alertas) > 2000 {
		s.alertas = map[string]time.Time{}
	}
	s.alertas[k] = s.ahora()
	s.mu.Unlock()
	nombre := inf.Nombre
	if nombre == "" {
		nombre = "Alguien"
	}
	titulo := "Filtro de " + strings.ToLower(inf.Filtro.Nombre)
	cuerpo := fmt.Sprintf("%s rompió el filtro en %s.", nombre, s.nombreGrupo(inf.Chat))
	if detalle != "" {
		cuerpo += " " + detalle
	}
	if _, err := s.Alm.CrearAvisoDe(s.BotID, "filtros", inf.Chat, inf.Persona, titulo, cuerpo); err == nil {
		s.log("Aviso local: un filtro de grupo detectó un mensaje.")
	}
}

// Aplicar hace lo que pide el filtro. Va en una tarea aparte (usa la red). Borrar y expulsar solo si soy administrador; si no lo
// soy, se alerta con una notificación local y no se intenta nada.
func (s *Servicio) Aplicar(ctx context.Context, inf *Infraccion) {
	if inf == nil {
		return
	}
	f := inf.Filtro
	reg := func(res, det string) {
		s.registra(almacen.PersonaDueno, "filtro."+f.Filtro, inf.Persona, inf.Chat, res, "accion="+f.Accion+det)
	}
	if s.Exento(ctx, inf.Chat, inf.Persona, f.Config) {
		return
	}
	soloAlerta := func(motivo string) {
		s.alerta(inf, "No hice la acción ("+motivo+"): decide tú.")
		reg("no_hecho", ",motivo="+motivo)
	}
	switch f.Accion {
	case AccAvisarGrupo:
		if s.Enviar == nil {
			soloAlerta("no puedo escribir ahora")
			return
		}
		texto := f.Config.Aviso
		if texto == "" {
			texto = avisoDefecto(f.Filtro)
		}
		var men []string
		if inf.PersonaWA != "" {
			men = []string{inf.PersonaWA}
			texto = "@" + usuarioDe(inf.PersonaWA) + " " + texto
		}
		if err := s.Enviar(ctx, inf.Chat, texto, inf.MensajeID, men); err != nil {
			soloAlerta("no se pudo avisar al grupo")
			return
		}
		reg("ok", "")
	case AccBorrar, AccExpulsar:
		admin, err := s.EsAdmin(ctx, inf.Chat)
		switch {
		case err != nil:
			soloAlerta("no pude saber si soy administrador")
		case !admin:
			soloAlerta("no soy administrador del grupo")
		case f.Accion == AccBorrar:
			if err := s.BorrarMensajeAjeno(ctx, inf.Chat, inf.MensajeID, inf.Persona); err != nil {
				soloAlerta(Explicar(err).Error())
			} else {
				reg("ok", "")
			}
		default:
			if err := s.Expulsar(ctx, inf.Chat, inf.Persona); err != nil {
				soloAlerta(Explicar(err).Error())
			} else {
				s.alerta(inf, "Se la expulsó del grupo.")
				reg("ok", "")
			}
		}
	case AccAdvertir:
		if s.Advertir == nil {
			soloAlerta("las advertencias no están disponibles")
			return
		}
		if err := s.Advertir(ctx, inf.Persona, inf.Chat, inf.MensajeID, "filtro "+strings.ToLower(f.Nombre)); err != nil {
			soloAlerta("no se pudo advertir")
			return
		}
		reg("ok", "")
	case AccSilenciar:
		min := f.Config.SilencioMin
		if min < 1 {
			min = 60
		}
		hasta := s.ahora().Add(time.Duration(min) * time.Minute).Unix()
		if err := s.Alm.FijarSilencio(s.BotID, inf.Persona, hasta, "filtro "+strings.ToLower(f.Nombre), "regla", s.ahora()); err != nil {
			soloAlerta("no se pudo silenciar")
			return
		}
		s.alerta(inf, fmt.Sprintf("El bot la silenció %d minutos.", min))
		reg("ok", fmt.Sprintf(",min=%d", min))
	default: // avisarme
		s.alerta(inf, "")
		reg("ok", "")
	}
}
