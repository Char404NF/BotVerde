// Package silencio es «Silenciar en grupos» (v0.6.0, E5): cuando el dueño silencia a una persona en uno, varios o todos los grupos que
// administra, el bot BORRA lo que escribe ahí (solo siendo administrador), le va contando los intentos del día y le avisa por privado
// cada cierto número; al llegar al tope aplica la sanción elegida (por defecto, avisarme y expulsar solo si yo confirmo con un toque).
//
// No confundir con persona_silencio («ignorar»: que el bot deje de CONTESTARLE; comandos !ignorar y !designorar).
//
// Reglas que no se rompen:
//   - Sin ser administrador NO se borra nada (el servidor decidiría, y no se sabe qué error da): se avisa «No soy admin aquí» al dueño
//     (máximo una vez por día y grupo) y a la persona solo se le dice que está silenciada, sin amenazas que el bot no podría cumplir.
//   - El intento solo cuenta si el mensaje se pudo borrar; así nunca se llega a una sanción por mensajes que siguen ahí.
//   - El enganche (AlMensaje) corre en el trabajador y solo encola: la red va en goroutines propias (un borrado a la vez y los avisos
//     privados aparte, con topes propios sil_* separados de los de iniciativa).
//   - Se ignoran los mensajes de la sincronización offline y los de más de 10 minutos (no se borra historial viejo al reconectar).
package silencio

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/grupos"
	"botchat/internal/permisos"
)

// MaxEdad es la antigüedad máxima de un mensaje para que el silencio lo borre.
const MaxEdad = 10 * time.Minute

// Errores que se le explican a quien pidió la acción.
var (
	ErrSinPersona = errors.New("falta indicar a quién")
	ErrYoMismo    = errors.New("no puedes hacer eso contigo mismo")
	ErrNoSancion  = errors.New("esa confirmación ya no está pendiente")
)

// Servicio es el silencio de grupos de un bot.
type Servicio struct {
	Alm   *almacen.Almacen
	BotID int64
	Perm  *permisos.Servicio
	// Grupo hace lo que solo puede un administrador (saber si lo soy, borrar un mensaje ajeno, expulsar). nil = no se puede nada.
	Grupo grupos.AccionesGrupo
	// Enviar manda un aviso privado como RESPUESTA (por la cola de salida del motor, no como iniciativa). nil = no se avisa a nadie.
	Enviar func(ctx context.Context, chat, texto string) error
	Ahora  func() time.Time
	Log    func(string)

	mu       sync.Mutex
	borrados []time.Time // borrados del último minuto (tope sil_borrados_min)
	avisos   []time.Time // avisos privados de la última hora (tope sil_avisos_hora)

	cola    chan Mensaje
	colaAv  chan aviso
	cancela context.CancelFunc
	wg      sync.WaitGroup
}

type aviso struct{ chat, texto string }

// Nuevo crea el servicio.
func Nuevo(alm *almacen.Almacen, botID int64, perm *permisos.Servicio) *Servicio {
	return &Servicio{Alm: alm, BotID: botID, Perm: perm, Grupo: grupos.Nula{}, Ahora: time.Now, Log: func(string) {}}
}

func (s *Servicio) ahora() time.Time {
	if s.Ahora != nil {
		return s.Ahora()
	}
	return time.Now()
}

func (s *Servicio) log(t string) {
	if s.Log != nil {
		s.Log(t)
	}
}

func (s *Servicio) grupo() grupos.AccionesGrupo {
	if s.Grupo == nil {
		return grupos.Nula{}
	}
	return s.Grupo
}

func (s *Servicio) persona(p string) string {
	if s.Perm != nil {
		return s.Perm.Persona(p)
	}
	return s.Alm.ResolverPersona(s.BotID, p)
}

func (s *Servicio) registra(actor, accion, objetivo, chat, resultado, detalle string) {
	s.Alm.RegistrarAccion(s.BotID, s.persona(actor), accion, s.persona(objetivo), chat, resultado, detalle)
}

func (s *Servicio) entero(clave string) int { return s.Alm.AjusteBotInt(s.BotID, clave) }

// puede revisa la jerarquía de rangos (igual que el resto de acciones sobre personas).
func (s *Servicio) puede(actor, objetivo string) error {
	actor, objetivo = s.persona(actor), s.persona(objetivo)
	if objetivo == "" {
		return ErrSinPersona
	}
	if objetivo == almacen.PersonaDueno {
		return permisos.ErrJerarquia
	}
	if actor == almacen.PersonaDueno {
		return nil
	}
	if actor == objetivo {
		return ErrYoMismo
	}
	if s.Perm != nil && !s.Perm.PuedeActuarSobre(actor, objetivo) {
		return permisos.ErrJerarquia
	}
	return nil
}

// ---------- día lógico ----------

// HoraReinicio lee sil_reinicio_hora («HH:MM»; 00:00 si no es válida).
func (s *Servicio) HoraReinicio() (h, m int) {
	if _, err := fmt.Sscanf(strings.TrimSpace(s.Alm.AjusteBot(s.BotID, "sil_reinicio_hora")), "%d:%d", &h, &m); err != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0
	}
	return h, m
}

// DiaLogico es el día (AAAA-MM-DD) al que pertenece ese momento para el contador: el día cambia a la hora de reinicio, en la zona
// del teléfono. Con reinicio 04:00, lo que se escribe a las 03:00 todavía cuenta como del día anterior.
func (s *Servicio) DiaLogico(t time.Time) string {
	h, m := s.HoraReinicio()
	return t.In(s.Alm.Zona()).Add(-time.Duration(h)*time.Hour - time.Duration(m)*time.Minute).Format("2006-01-02")
}

// ProximoReinicio da el momento en que el contador de hoy vuelve a cero.
func (s *Servicio) ProximoReinicio(t time.Time) time.Time {
	h, m := s.HoraReinicio()
	loc := s.Alm.Zona()
	l := t.In(loc)
	r := time.Date(l.Year(), l.Month(), l.Day(), h, m, 0, 0, loc)
	if !r.After(l) {
		r = r.AddDate(0, 0, 1)
	}
	return r
}

// ---------- consultas ----------

// Vigente devuelve el silencio que vale para esa persona en ese grupo, o nil.
func (s *Servicio) Vigente(persona, grupo string, ahora time.Time) *almacen.SilencioG {
	xs, err := s.Alm.SilenciosActivos(s.BotID, persona, ahora)
	if err != nil {
		return nil
	}
	for i := range xs {
		if xs[i].Cubre(grupo) {
			return &xs[i]
		}
	}
	return nil
}

// Contador es lo que muestra la barra de color del perfil: cuántos mensajes borrados lleva hoy y cuál es el tope.
type Contador struct {
	Persona         string              `json:"persona"`
	Grupo           string              `json:"grupo"` // "" = total
	Dia             string              `json:"dia"`
	Usado           int                 `json:"usado"` // el número que se compara con el tope (total o del grupo, según sil_conteo_por_grupo)
	Tope            int                 `json:"tope"`  // 0 = sin tope
	PorGrupo        bool                `json:"porGrupo"`
	Avisos          int                 `json:"avisos"` // avisos privados de hoy
	SancionAplicada bool                `json:"sancionAplicada"`
	Total           int                 `json:"total"`
	Grupos          []almacen.ConteoSil `json:"grupos"`
	ProximoReinicio int64               `json:"proximoReinicio"`
	Color           string              `json:"color"` // verde | amarillo | rojo (por si la pantalla no quiere calcularlo)
}

// ContadorDe arma el contador de la persona (grupo "" = el total; si se cuenta por grupo, el grupo dado).
func (s *Servicio) ContadorDe(persona, grupo string, ahora time.Time) Contador {
	persona = s.persona(persona)
	dia := s.DiaLogico(ahora)
	porGrupo := s.entero("sil_conteo_por_grupo") == 1
	c := Contador{Persona: persona, Grupo: grupo, Dia: dia, Tope: s.entero("sil_tope"), PorGrupo: porGrupo,
		ProximoReinicio: s.ProximoReinicio(ahora).Unix(), Grupos: []almacen.ConteoSil{}}
	tot := s.Alm.ConteoDe(s.BotID, persona, "", dia)
	c.Total, c.Avisos = tot.N, tot.Avisos
	for _, x := range s.Alm.ConteosDePersona(s.BotID, persona, dia) {
		if x.Grupo != "" {
			c.Grupos = append(c.Grupos, x)
		}
	}
	c.Usado, c.SancionAplicada = tot.N, tot.SancionTS > 0
	if porGrupo && grupo != "" {
		g := s.Alm.ConteoDe(s.BotID, persona, grupo, dia)
		c.Usado, c.SancionAplicada = g.N, g.SancionTS > 0
	}
	c.Color = "verde"
	if c.Tope > 0 {
		switch {
		case c.Usado*3 >= c.Tope*2:
			c.Color = "rojo"
		case c.Usado*3 >= c.Tope:
			c.Color = "amarillo"
		}
	}
	return c
}

// PonerEnCero deja en cero el contador de hoy de la persona (grupo "" o "*" = todos sus contadores; si no, el total y el de ese grupo).
func (s *Servicio) PonerEnCero(actor, persona, grupo string) error {
	if err := s.puede(actor, persona); err != nil {
		s.registra(actor, "silencio.contador_cero", persona, "", "denegado", "")
		return err
	}
	persona = s.persona(persona)
	if grupo == "" {
		grupo = "*"
	}
	if err := s.Alm.PonerConteoCero(s.BotID, persona, grupo, s.DiaLogico(s.ahora())); err != nil {
		s.registra(actor, "silencio.contador_cero", persona, "", "error", "")
		return err
	}
	s.registra(actor, "silencio.contador_cero", persona, "", "ok", "")
	return nil
}

// ---------- crear y quitar ----------

// Opciones de un silencio nuevo.
type Opciones struct {
	Duracion time.Duration // 0 = para siempre
	Alcance  string        // uno | varios | todos
	Grupos   []string      // claves de los grupos (uno o varios)
	Avisar   bool          // avisarle a la persona que la silencié
	Texto    string        // texto propio del aviso ("" = el de fábrica)
	Origen   string        // manual | escalera | tope
}

// Silenciar silencia a la persona en grupos (revisa la jerarquía de rangos).
func (s *Servicio) Silenciar(actor, persona string, o Opciones) (*almacen.SilencioG, error) {
	if err := s.puede(actor, persona); err != nil {
		s.registra(actor, "silencio.silenciar", persona, "", "denegado", "")
		return nil, err
	}
	return s.Crear(actor, persona, o)
}

// Crear silencia sin revisar rangos (lo usan la escalera de advertencias y las sanciones; el Dueño nunca se puede silenciar).
func (s *Servicio) Crear(actor, persona string, o Opciones) (*almacen.SilencioG, error) {
	persona = s.persona(persona)
	if persona == "" {
		return nil, ErrSinPersona
	}
	if persona == almacen.PersonaDueno {
		return nil, permisos.ErrJerarquia
	}
	if o.Alcance == "" {
		if len(o.Grupos) == 1 {
			o.Alcance = almacen.SilUno
		} else if len(o.Grupos) > 1 {
			o.Alcance = almacen.SilVarios
		} else {
			o.Alcance = almacen.SilTodos
		}
	}
	for _, g := range o.Grupos {
		if !strings.HasSuffix(g, "@g.us") {
			return nil, errors.New("eso no es un grupo")
		}
	}
	if o.Origen == "" {
		o.Origen = "manual"
	}
	ahora := s.ahora()
	hasta := int64(0)
	if o.Duracion > 0 {
		hasta = ahora.Add(o.Duracion).Unix()
	}
	id, err := s.Alm.CrearSilencioGrupo(s.BotID, persona, o.Alcance, o.Grupos, hasta, s.persona(actor), o.Origen, o.Texto, o.Avisar, ahora)
	if err != nil {
		s.registra(actor, "silencio.silenciar", persona, "", "error", "")
		return nil, err
	}
	s.registra(actor, "silencio.silenciar", persona, "", "ok", fmt.Sprintf("alcance=%s,seg=%d,origen=%s", o.Alcance, int64(o.Duracion.Seconds()), o.Origen))
	sil, _ := s.Alm.SilencioPorID(s.BotID, id)
	if sil != nil && o.Avisar && s.entero("sil_avisar_inicio") == 1 {
		g := ""
		if len(sil.Grupos) > 0 {
			g = sil.Grupos[0]
		}
		if ok, _ := s.AvisoPrivado(persona, s.armaTexto("inicio", sil, persona, g, 0)); ok {
			s.Alm.MarcarAvisoSilencio(s.BotID, id, ahora.Unix())
		}
	}
	return sil, nil
}

// QuitarSilencio termina los silencios de la persona. Devuelve si tenía alguno. No avisa de «vencimiento» (fue una decisión mía).
func (s *Servicio) QuitarSilencio(actor, persona string) (bool, error) {
	if err := s.puede(actor, persona); err != nil {
		s.registra(actor, "silencio.quitar", persona, "", "denegado", "")
		return false, err
	}
	n, err := s.Alm.QuitarSilenciosGrupo(s.BotID, s.persona(persona), s.ahora(), almacen.FinQuitado)
	if err != nil {
		s.registra(actor, "silencio.quitar", persona, "", "error", "")
		return false, err
	}
	s.registra(actor, "silencio.quitar", persona, "", "ok", fmt.Sprintf("n=%d", n))
	return n > 0, nil
}

// Lista devuelve los silencios vigentes de todos, con el nombre de cada persona.
type Item struct {
	almacen.SilencioG
	Nombre   string   `json:"nombre"`
	Contador Contador `json:"contador"`
}

// Listar devuelve los silencios vigentes (el más nuevo primero) con su contador de hoy.
func (s *Servicio) Listar() ([]Item, error) {
	ahora := s.ahora()
	xs, err := s.Alm.SilenciosActivos(s.BotID, "", ahora)
	if err != nil {
		return nil, err
	}
	out := make([]Item, 0, len(xs))
	for _, x := range xs {
		out = append(out, Item{SilencioG: x, Nombre: s.nombre(x.Persona), Contador: s.ContadorDe(x.Persona, "", ahora)})
	}
	return out, nil
}

func (s *Servicio) nombre(persona string) string {
	if n := s.Alm.NombreDePersona(s.BotID, persona); n != "" && n != "Participante" {
		return n
	}
	if i := strings.Index(persona, "@s.whatsapp.net"); i > 0 {
		return "+" + persona[:i]
	}
	return "esa persona"
}

func (s *Servicio) nombreGrupo(grupo string) string {
	if c, _ := s.Alm.ObtenerChat(s.BotID, grupo); c != nil && c.Mostrado != "" {
		return c.Mostrado
	}
	return "el grupo"
}

// chatPrivado da la clave del chat privado de la persona (el que ya existe entre sus formas; si no, la canónica).
func (s *Servicio) chatPrivado(persona string) string {
	for _, f := range s.Alm.AliasDe(s.BotID, persona) {
		if c, _ := s.Alm.ObtenerChat(s.BotID, f); c != nil && !c.EsGrupo {
			return f
		}
	}
	return persona
}

// ---------- el enganche con los mensajes ----------

// Mensaje es lo que se mira de un mensaje recibido.
type Mensaje struct {
	Chat    string
	ID      string
	Persona string
	TS      time.Time
	EsGrupo bool
	EsMio   bool
	Sinc    bool // llegó en la sincronización offline: nunca se borra
}

// Iniciar arranca las dos tareas de fondo (borrar y avisar). Sin esto, AlMensaje procesa en el momento (así lo usan las pruebas).
func (s *Servicio) Iniciar(ctx context.Context) {
	s.mu.Lock()
	if s.cola != nil {
		s.mu.Unlock()
		return
	}
	ctx, s.cancela = context.WithCancel(ctx)
	s.cola = make(chan Mensaje, 500)
	s.colaAv = make(chan aviso, 100)
	cola, colaAv := s.cola, s.colaAv
	s.mu.Unlock()
	s.wg.Add(2)
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case m := <-cola:
				func() {
					defer func() {
						if r := recover(); r != nil {
							s.log(fmt.Sprintf("Silencio: error interno: %v", r))
						}
					}()
					c, cancel := context.WithTimeout(ctx, 45*time.Second)
					defer cancel()
					s.Procesar(c, m)
				}()
			}
		}
	}()
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case a := <-colaAv:
				func() {
					defer func() {
						if r := recover(); r != nil {
							s.log(fmt.Sprintf("Silencio: error interno: %v", r))
						}
					}()
					c, cancel := context.WithTimeout(ctx, 60*time.Second)
					defer cancel()
					if s.Enviar != nil {
						if err := s.Enviar(c, a.chat, a.texto); err != nil {
							s.log("Silencio: no salió un aviso privado.")
						}
					}
				}()
			}
		}
	}()
}

// Detener para las tareas de fondo y espera a que terminen.
func (s *Servicio) Detener() {
	s.mu.Lock()
	cancela := s.cancela
	s.cancela, s.cola, s.colaAv = nil, nil, nil
	s.mu.Unlock()
	if cancela != nil {
		cancela()
	}
	s.wg.Wait()
}

// AlMensaje se llama desde el trabajador, justo después de guardar el mensaje. Solo mira la base local y, si la persona está
// silenciada ahí, deja el trabajo en la cola propia; nada de red aquí. Devuelve si el mensaje quedó en cola (o se procesó).
func (s *Servicio) AlMensaje(m Mensaje) bool {
	if m.EsMio || !m.EsGrupo || m.Sinc || m.ID == "" {
		return false
	}
	ahora := s.ahora()
	if !m.TS.IsZero() && ahora.Sub(m.TS) > MaxEdad {
		return false
	}
	m.Persona = s.persona(m.Persona)
	if m.Persona == "" || s.Vigente(m.Persona, m.Chat, ahora) == nil {
		return false
	}
	s.mu.Lock()
	cola := s.cola
	s.mu.Unlock()
	if cola == nil {
		s.Procesar(context.Background(), m)
		return true
	}
	select {
	case cola <- m:
		return true
	default:
		s.log("Silencio: hay demasiados mensajes en espera; uno no se borró.")
		return false
	}
}

// Resultado dice qué pasó con un mensaje (para las pruebas y el registro).
type Resultado struct {
	Hecho  string `json:"hecho"`  // borrado | no_admin | omitido | tope_borrados | error
	Motivo string `json:"motivo"` // por qué no
	N      int    `json:"n"`      // contador (el que se compara con el tope) después de contar
}

// permiteBorrar aplica el tope de borrados por minuto.
func (s *Servicio) permiteBorrar(ahora time.Time) bool {
	tope := s.entero("sil_borrados_min")
	s.mu.Lock()
	defer s.mu.Unlock()
	i := 0
	for i < len(s.borrados) && ahora.Sub(s.borrados[i]) >= time.Minute {
		i++
	}
	s.borrados = s.borrados[i:]
	if tope > 0 && len(s.borrados) >= tope {
		return false
	}
	s.borrados = append(s.borrados, ahora)
	return true
}

// Procesar atiende un mensaje de una persona (si está silenciada en ese grupo): lo borra, cuenta el intento, avisa y sanciona.
func (s *Servicio) Procesar(ctx context.Context, m Mensaje) Resultado {
	ahora := s.ahora()
	if m.EsMio || !m.EsGrupo || m.Sinc {
		return Resultado{Hecho: "omitido", Motivo: "no aplica"}
	}
	if !m.TS.IsZero() && ahora.Sub(m.TS) > MaxEdad {
		return Resultado{Hecho: "omitido", Motivo: "mensaje viejo"}
	}
	persona := s.persona(m.Persona)
	sil := s.Vigente(persona, m.Chat, ahora)
	if sil == nil {
		return Resultado{Hecho: "omitido", Motivo: "no está silenciada"}
	}
	if s.entero("sil_activo") != 1 {
		return Resultado{Hecho: "omitido", Motivo: "apagado"}
	}
	admin, err := s.grupo().EsAdmin(ctx, m.Chat)
	if err != nil {
		s.log("Silencio: no pude saber si soy administrador.")
		return Resultado{Hecho: "error", Motivo: "no pude saber si soy administrador"}
	}
	dia := s.DiaLogico(ahora)
	if !admin {
		s.sinAdmin(sil, m, persona, dia, ahora)
		return Resultado{Hecho: "no_admin", Motivo: "no soy administrador"}
	}
	if !s.permiteBorrar(ahora) {
		s.registra(almacen.PersonaDueno, "silencio.borrar", persona, m.Chat, "tope", "")
		return Resultado{Hecho: "tope_borrados", Motivo: "tope de borrados por minuto"}
	}
	if err := s.grupo().BorrarMensajeAjeno(ctx, m.Chat, m.ID, persona); err != nil {
		s.registra(almacen.PersonaDueno, "silencio.borrar", persona, m.Chat, "error", "")
		return Resultado{Hecho: "error", Motivo: "WhatsApp no dejó borrarlo"}
	}
	s.registra(almacen.PersonaDueno, "silencio.borrar", persona, m.Chat, "ok", "")
	total, delGrupo, err := s.Alm.SumarIntento(s.BotID, persona, m.Chat, dia)
	if err != nil {
		return Resultado{Hecho: "borrado", Motivo: "no se pudo contar"}
	}
	clave, n := "", total
	if s.entero("sil_conteo_por_grupo") == 1 {
		clave, n = m.Chat, delGrupo
	}
	res := Resultado{Hecho: "borrado", N: n}
	tope, cada := s.entero("sil_tope"), s.entero("sil_aviso_cada")
	fila := s.Alm.ConteoDe(s.BotID, persona, clave, dia)
	switch {
	case tope > 0 && n >= tope:
		if fila.SancionTS == 0 {
			s.Alm.MarcarSancionConteo(s.BotID, persona, clave, dia, ahora.Unix())
			s.sancion(ctx, sil, persona, m.Chat, n, tope)
		}
	case cada > 0 && n%cada == 0:
		s.AvisoPrivado(persona, s.armaTexto("aviso", sil, persona, m.Chat, n))
	}
	return res
}

// sinAdmin: alguien silenciado escribió donde no soy administrador. No se borra ni se cuenta. Se avisa al dueño (una vez por día y
// grupo) y a la persona (una vez por día y grupo) que está silenciada, sin hablar de expulsiones.
func (s *Servicio) sinAdmin(sil *almacen.SilencioG, m Mensaje, persona, dia string, ahora time.Time) {
	clave := "na:" + m.Chat
	if s.entero("sil_aviso_noadmin") == 1 && s.Alm.ConteoDe(s.BotID, "", clave, dia).Avisos == 0 {
		s.Alm.SumarAvisoConteo(s.BotID, "", clave, dia, ahora.Unix())
		det := fmt.Sprintf("%s está silenciado en %s, pero no soy administrador ahí: no puedo borrar sus mensajes. Hazme administrador o quita el silencio.", s.nombre(persona), s.nombreGrupo(m.Chat))
		if _, err := s.Alm.CrearAvisoDe(s.BotID, "silencio_noadmin", m.Chat, persona, "No soy admin aquí", det); err == nil {
			s.log("Aviso local: no soy administrador donde hay alguien silenciado.")
		}
		s.registra(almacen.PersonaDueno, "silencio.borrar", persona, m.Chat, "no_admin", "")
	}
	if s.Alm.ConteoDe(s.BotID, persona, clave, dia).Avisos == 0 {
		s.Alm.SumarAvisoConteo(s.BotID, persona, clave, dia, ahora.Unix())
		s.AvisoPrivado(persona, s.armaTexto("noadmin", sil, persona, m.Chat, 0))
	}
}

// ---------- avisos privados ----------

// AvisoPrivado le manda un texto a la persona por su chat privado, como respuesta (no iniciativa). Respeta No molestar y el bloqueo,
// y los topes propios: sil_avisos_hora, sil_avisos_dia y sil_avisos_persona_dia. Devuelve si se mandó (o quedó en cola) y, si no,
// por qué.
func (s *Servicio) AvisoPrivado(persona, texto string) (bool, string) {
	persona = s.persona(persona)
	texto = strings.TrimSpace(texto)
	if texto == "" || persona == "" || persona == almacen.PersonaDueno {
		return false, "sin texto"
	}
	if s.Enviar == nil {
		return false, "no disponible"
	}
	if s.Alm.EstaNoMolestar(s.BotID, persona) {
		return false, "no molestar"
	}
	if s.Alm.EstaBloqueada(s.BotID, persona) {
		return false, "bloqueada"
	}
	ahora := s.ahora()
	dia := s.DiaLogico(ahora)
	if t := s.entero("sil_avisos_persona_dia"); t > 0 && s.Alm.ConteoDe(s.BotID, persona, "", dia).Avisos >= t {
		return false, "tope por persona"
	}
	if t := s.entero("sil_avisos_dia"); t > 0 && s.Alm.ConteoDe(s.BotID, "", "", dia).Avisos >= t {
		return false, "tope diario"
	}
	s.mu.Lock()
	i := 0
	for i < len(s.avisos) && ahora.Sub(s.avisos[i]) >= time.Hour {
		i++
	}
	s.avisos = s.avisos[i:]
	if t := s.entero("sil_avisos_hora"); t > 0 && len(s.avisos) >= t {
		s.mu.Unlock()
		return false, "tope por hora"
	}
	s.avisos = append(s.avisos, ahora)
	cola := s.colaAv
	s.mu.Unlock()
	s.Alm.SumarAvisoConteo(s.BotID, persona, "", dia, ahora.Unix())
	s.Alm.SumarAvisoConteo(s.BotID, "", "", dia, ahora.Unix())
	chat := s.chatPrivado(persona)
	s.registra(almacen.PersonaDueno, "silencio.aviso", persona, "", "ok", "")
	if cola == nil {
		if err := s.Enviar(context.Background(), chat, texto); err != nil {
			return false, "no salió"
		}
		return true, ""
	}
	select {
	case cola <- aviso{chat, texto}:
		return true, ""
	default:
		return false, "cola llena"
	}
}

// ---------- sanción ----------

// Nombres de las sanciones del tope.
const (
	SancConfirmar     = "confirmar"      // avisarme y expulsar solo si yo confirmo con un toque (de fábrica)
	SancAvisar        = "avisar"         // solo avisarme
	SancSilencioLargo = "silencio_largo" // alargar el silencio
	SancExpulsar      = "expulsar"       // expulsar (solo sola si sil_expulsion_auto = 1; si no, pide confirmar)
)

func (s *Servicio) sancion(ctx context.Context, sil *almacen.SilencioG, persona, grupo string, n, tope int) {
	ahora := s.ahora()
	nombre, ng := s.nombre(persona), s.nombreGrupo(grupo)
	modo := s.Alm.AjusteBot(s.BotID, "sil_sancion")
	titulo := "Silencio: llegó al tope"
	base := fmt.Sprintf("%s llegó a %d mensajes borrados hoy en %s (tope %d).", nombre, n, ng, tope)
	switch modo {
	case SancSilencioLargo:
		min := s.entero("sil_largo_min")
		if min < 1 {
			min = 10080
		}
		if sil.HastaTS != 0 { // si ya es para siempre no hay nada más largo
			nuevo := ahora.Add(time.Duration(min) * time.Minute).Unix()
			if nuevo < sil.HastaTS {
				nuevo = sil.HastaTS
			}
			if err := s.Alm.AlargarSilencio(s.BotID, sil.ID, nuevo, ahora); err != nil {
				s.avisoDueno("silencio_tope", grupo, persona, titulo, base+" No pude alargar su silencio.")
				return
			}
		}
		s.registra(almacen.PersonaDueno, "silencio.sancion", persona, grupo, "ok", "silencio_largo")
		s.avisoDueno("silencio_tope", grupo, persona, titulo, base+" Alargué su silencio.")
		s.AvisoPrivado(persona, s.armaTexto("tope", sil, persona, grupo, n))
	case SancExpulsar:
		if s.entero("sil_expulsion_auto") == 1 {
			if err := s.grupo().Expulsar(ctx, grupo, persona); err != nil {
				s.registra(almacen.PersonaDueno, "silencio.sancion", persona, grupo, "error", "expulsar")
				s.avisoDueno("silencio_tope", grupo, persona, titulo, base+" Intenté expulsarla y no pude: decide tú.")
				return
			}
			s.registra(almacen.PersonaDueno, "silencio.sancion", persona, grupo, "ok", "expulsar")
			s.avisoDueno("silencio_tope", grupo, persona, titulo, base+" La expulsé del grupo.")
			return
		}
		s.pedirConfirmacion("silencio", persona, grupo, titulo, base)
	case SancAvisar:
		s.registra(almacen.PersonaDueno, "silencio.sancion", persona, grupo, "ok", "avisar")
		s.avisoDueno("silencio_tope", grupo, persona, titulo, base+" Decide tú qué hacer.")
	default: // confirmar
		s.pedirConfirmacion("silencio", persona, grupo, titulo, base)
	}
}

// PedirExpulsion deja una expulsión esperando mi confirmación (con una notificación). Lo usa la escalera de advertencias.
func (s *Servicio) PedirExpulsion(origen, persona, grupo, titulo, detalle string) (int64, error) {
	return s.pedirConfirmacion(origen, s.persona(persona), grupo, titulo, detalle)
}

func (s *Servicio) pedirConfirmacion(origen, persona, grupo, titulo, base string) (int64, error) {
	ahora := s.ahora()
	id, nueva, err := s.Alm.CrearSancionPendiente(s.BotID, origen, persona, grupo, "expulsar", 0, ahora)
	if err != nil {
		return 0, err
	}
	if !nueva {
		return id, nil // ya estaba esperando: no se repite el aviso
	}
	avID := s.avisoDueno("sancion", grupo, persona, titulo, base+" ¿La expulsamos? Confirma con un toque; si no haces nada, no pasa nada.")
	s.Alm.PonerAvisoSancion(s.BotID, id, avID)
	s.registra(almacen.PersonaDueno, "silencio.sancion", persona, grupo, "pendiente", "expulsar")
	return id, nil
}

func (s *Servicio) avisoDueno(tipo, chat, persona, titulo, detalle string) int64 {
	id, err := s.Alm.CrearAvisoDe(s.BotID, tipo, chat, persona, titulo, detalle)
	if err != nil {
		return 0
	}
	s.log("Aviso local: " + strings.ToLower(titulo) + ".")
	return id
}

// AvisoDueno crea un aviso local (notificación) para el dueño.
func (s *Servicio) AvisoDueno(tipo, chat, persona, titulo, detalle string) int64 {
	return s.avisoDueno(tipo, chat, persona, titulo, detalle)
}

// SancionesPendientes devuelve las expulsiones que esperan mi confirmación.
func (s *Servicio) SancionesPendientes() ([]almacen.SancionPend, error) {
	return s.Alm.ListarSancionesPendientes(s.BotID)
}

// ConfirmarSancion responde a una sanción pendiente: si=true la ejecuta (hoy, expulsar; solo siendo administrador), si=false la
// descarta. Devuelve un texto corto con lo que pasó.
func (s *Servicio) ConfirmarSancion(ctx context.Context, id int64, si bool) (string, error) {
	p, err := s.Alm.ObtenerSancion(s.BotID, id)
	if err != nil {
		return "", err
	}
	if p == nil || p.Estado != almacen.SancionPendiente {
		return "", ErrNoSancion
	}
	ahora := s.ahora()
	if !si {
		_, _ = s.Alm.ResolverSancion(s.BotID, id, almacen.SancionRechazada, "no", ahora)
		s.registra(almacen.PersonaDueno, "silencio.confirmar", p.Persona, p.Grupo, "rechazada", p.Accion)
		return "Listo: no hice nada.", nil
	}
	fallo := func(motivo string) (string, error) {
		_, _ = s.Alm.ResolverSancion(s.BotID, id, almacen.SancionFallida, motivo, ahora)
		s.registra(almacen.PersonaDueno, "silencio.confirmar", p.Persona, p.Grupo, "error", p.Accion)
		return "", errors.New(motivo)
	}
	admin, err := s.grupo().EsAdmin(ctx, p.Grupo)
	switch {
	case err != nil && !errors.Is(err, grupos.ErrNoDisponible):
		return fallo("no pude saber si soy administrador")
	case err != nil || !admin:
		return fallo("no soy administrador de ese grupo")
	}
	if err := s.grupo().Expulsar(ctx, p.Grupo, p.Persona); err != nil {
		if errors.Is(err, grupos.ErrNoDisponible) {
			return fallo("esa acción todavía no está disponible")
		}
		return fallo("WhatsApp no lo permitió")
	}
	_, _ = s.Alm.ResolverSancion(s.BotID, id, almacen.SancionHecha, "ok", ahora)
	s.registra(almacen.PersonaDueno, "silencio.confirmar", p.Persona, p.Grupo, "ok", p.Accion)
	return "Listo: la expulsé del grupo.", nil
}

// ---------- vencimientos ----------

// Vencer termina los silencios cuya hora ya llegó: le avisa a la persona y a mí. Es seguro llamarlo de más. Devuelve los avisos
// creados para el dueño. También limpia contadores viejos y pone al día las banderas de los miembros.
func (s *Servicio) Vencer() []int64 {
	ahora := s.ahora()
	out := []int64{}
	xs, err := s.Alm.SilenciosVencidos(s.BotID, ahora)
	if err != nil {
		return out
	}
	for _, x := range xs {
		if ok, err := s.Alm.FinalizarSilencio(s.BotID, x.ID, ahora, almacen.FinVencio); err != nil || !ok {
			continue
		}
		s.registra(almacen.PersonaDueno, "silencio.vencio", x.Persona, "", "ok", "")
		if id := s.avisoDueno("silencio_fin", "", x.Persona, "Terminó un silencio", fmt.Sprintf("El silencio de %s terminó.", s.nombre(x.Persona))); id != 0 {
			out = append(out, id)
		}
		if x.Avisar && s.entero("sil_avisar_fin") == 1 {
			g := ""
			if len(x.Grupos) > 0 {
				g = x.Grupos[0]
			}
			s.AvisoPrivado(x.Persona, s.armaTexto("fin", &x, x.Persona, g, 0))
		}
	}
	s.Alm.LimpiarConteosSilencio(s.BotID, s.DiaLogico(ahora.AddDate(0, 0, -7)))
	s.Alm.RefrescarBanderas(s.BotID, "", ahora)
	return out
}

// Siguiente da la hora en que vence el próximo silencio (0 = ninguno).
func (s *Servicio) Siguiente() int64 { return s.Alm.ProximoVencimientoSilencio(s.BotID) }

// Disparar es Vencer con la forma que usan los avisos para el dueño (ver avisos.Fuente).
func (s *Servicio) Disparar() []int64 { return s.Vencer() }
