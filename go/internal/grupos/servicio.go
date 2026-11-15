package grupos

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"botchat/internal/almacen"
	"botchat/internal/permisos"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

const (
	vidaInfoDefecto = 90 * time.Second // cuánto vale la información de un grupo en memoria (los eventos de grupo la renuevan antes)
	maxNombreGrupo  = 100
	maxDescripcion  = 2048
	maxTextoTodos   = 500
)

// Servicio hace con los grupos lo que se puede sin ser administrador (información, administradores, mencionar a todos, ranking e
// inactivos) y lo que exige serlo (cerrar, bloquear, expulsar, ascender, solicitudes, nombre, descripción, enlace, borrar mensajes
// ajenos y los filtros). Implementa AccionesGrupo para las advertencias. Toda acción de administrador revisa primero que sea
// administrador y traduce los errores de WhatsApp a mensajes claros.
type Servicio struct {
	BotID int64
	Alm   *almacen.Almacen
	Cli   Cliente // nil = sin conexión (la app sigue mostrando ranking, filtros y reglas)
	Perm  *permisos.Servicio
	// Borrar borra para todos un mensaje guardado de otra persona (servicio de envío).
	Borrar func(ctx context.Context, chat, id string) error
	// Enviar manda un texto a un chat citando un mensaje y mencionando a esas personas (respuesta a un comando o aviso de un filtro).
	Enviar func(ctx context.Context, chat, texto, citaID string, menciones []string) error
	// Encuesta manda una encuesta al grupo. nil = no disponible.
	Encuesta func(ctx context.Context, chat, titulo string, opciones []string, multiple bool) error
	// Fijar fija o suelta un mensaje del grupo para todos (segundos: 1, 7 o 30 días). nil = no disponible.
	Fijar func(ctx context.Context, chat, msgID string, segundos int, fijar bool) error
	// Advertir pone una advertencia a una persona (con la regla del tope de la etapa B). nil = el filtro "advertir" solo alerta.
	Advertir func(ctx context.Context, objetivo, chat, msgID, motivo string) error
	Ahora    func() time.Time
	Log      func(string)
	// Pausa espera entre dos mensajes de "mencionar a todos" (las pruebas la dejan vacía).
	Pausa func(ctx context.Context, d time.Duration)
	// VidaInfo: cuánto dura en memoria la información de un grupo (por defecto 90 s).
	VidaInfo time.Duration

	mu      sync.Mutex
	cache   map[string]cacheGrupo
	todos   map[string]time.Time
	filt    map[string]cacheFiltros
	hist    map[string]*historial
	alertas map[string]time.Time
	// protección de entrada: entradas recientes por grupo y cuándo se dio la última alarma de raid
	entradas    map[string][]time.Time
	raidAvisado map[string]time.Time
}

type cacheGrupo struct {
	info *types.GroupInfo
	en   time.Time
}

// Nuevo crea el servicio.
func Nuevo(alm *almacen.Almacen, botID int64, cli Cliente, perm *permisos.Servicio) *Servicio {
	return &Servicio{BotID: botID, Alm: alm, Cli: cli, Perm: perm, Ahora: time.Now, Log: func(string) {},
		cache: map[string]cacheGrupo{}, todos: map[string]time.Time{}, filt: map[string]cacheFiltros{}, hist: map[string]*historial{}, alertas: map[string]time.Time{}}
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

func (s *Servicio) pausa(ctx context.Context, d time.Duration) {
	if s.Pausa != nil {
		s.Pausa(ctx, d)
		return
	}
	select {
	case <-time.After(d):
	case <-ctx.Done():
	}
}

// persona da la clave canónica de una persona.
func (s *Servicio) persona(p string) string {
	if s.Perm != nil {
		return s.Perm.Persona(p)
	}
	return p
}

func (s *Servicio) registra(actor, accion, objetivo, chat, resultado, detalle string) {
	s.Alm.RegistrarAccion(s.BotID, s.persona(actor), accion, s.persona(objetivo), chat, resultado, detalle)
}

func (s *Servicio) conectado() bool { return s.Cli != nil && s.Cli.Conectado() }

// Invalidar olvida la información en memoria de un grupo (se llama cuando WhatsApp avisa de un cambio en el grupo).
func (s *Servicio) Invalidar(chat string) {
	s.mu.Lock()
	delete(s.cache, chat)
	s.mu.Unlock()
}

func parseaGrupo(chat string) (types.JID, error) {
	j, err := types.ParseJID(chat)
	if err != nil || j.IsEmpty() || j.Server != types.GroupServer {
		return types.EmptyJID, ErrNoEsGrupo
	}
	return j.ToNonAD(), nil
}

// info lee la información del grupo (con la caché corta). fresco = ignorar la caché.
func (s *Servicio) info(ctx context.Context, j types.JID, fresco bool) (*types.GroupInfo, error) {
	vida := s.VidaInfo
	if vida <= 0 {
		vida = vidaInfoDefecto
	}
	k := j.String()
	s.mu.Lock()
	c, ok := s.cache[k]
	s.mu.Unlock()
	if ok && !fresco && s.ahora().Sub(c.en) < vida {
		return c.info, nil
	}
	if !s.conectado() {
		return nil, ErrSinConexion
	}
	inf, err := s.Cli.GetGroupInfo(ctx, j)
	if err != nil {
		return nil, Explicar(err)
	}
	s.mu.Lock()
	if len(s.cache) > 300 {
		s.cache = map[string]cacheGrupo{}
	}
	s.cache[k] = cacheGrupo{info: inf, en: s.ahora()}
	s.mu.Unlock()
	return inf, nil
}

// esYo dice si el participante es esta cuenta.
func (s *Servicio) esYo(p types.GroupParticipant) bool {
	if s.Cli == nil {
		return false
	}
	pn, lid := s.Cli.Propio()
	return (!pn.IsEmpty() && (p.JID.User == pn.User || p.PhoneNumber.User == pn.User)) ||
		(!lid.IsEmpty() && (p.JID.User == lid.User || p.LID.User == lid.User))
}

func (s *Servicio) soyAdminEn(inf *types.GroupInfo) bool {
	for _, p := range inf.Participants {
		if s.esYo(p) {
			return p.IsAdmin || p.IsSuperAdmin
		}
	}
	return false
}

// preparar valida que sea un grupo, que haya conexión y (si se pide) que yo sea administrador.
func (s *Servicio) preparar(ctx context.Context, chat string, admin bool) (types.JID, *types.GroupInfo, error) {
	j, err := parseaGrupo(chat)
	if err != nil {
		return j, nil, err
	}
	inf, err := s.info(ctx, j, false)
	if err != nil {
		return j, nil, err
	}
	if admin && !s.soyAdminEn(inf) {
		return j, inf, ErrNoAdmin
	}
	return j, inf, nil
}

// EstadoGrupo es lo que la app necesita para activar o apagar las acciones de un grupo.
type EstadoGrupo struct {
	Conectado bool   `json:"conectado"`
	EnGrupo   bool   `json:"enGrupo"`
	SoyAdmin  bool   `json:"soyAdmin"`
	Motivo    string `json:"motivo"` // explicación clara cuando algo no se puede (vacío si todo está bien)
}

// Estado dice si el bot administra el grupo. refrescar = preguntar a WhatsApp aunque haya información reciente.
func (s *Servicio) Estado(ctx context.Context, chat string, refrescar bool) EstadoGrupo {
	j, err := parseaGrupo(chat)
	if err != nil {
		return EstadoGrupo{Conectado: s.conectado(), Motivo: "Esto solo existe en los grupos."}
	}
	if !s.conectado() {
		return EstadoGrupo{Motivo: "El bot no está conectado, así que no sé si administro este grupo. Conéctalo para usar las acciones del grupo."}
	}
	inf, err := s.info(ctx, j, refrescar)
	if err != nil {
		e := EstadoGrupo{Conectado: true, Motivo: capitaliza(Explicar(err).Error()) + "."}
		if errors.Is(err, whatsmeow.ErrNotInGroup) {
			e.Motivo = "Ya no estás en este grupo."
		}
		return e
	}
	e := EstadoGrupo{Conectado: true, EnGrupo: true, SoyAdmin: s.soyAdminEn(inf)}
	if !e.SoyAdmin {
		e.Motivo = "No soy administrador de este grupo. Puedes ver la información, el ranking, los inactivos y mencionar a todos; " +
			"cerrar el grupo, expulsar, borrar mensajes o cambiar ajustes necesitan que me hagas administrador en WhatsApp. " +
			"Los filtros solo te avisan con una notificación."
	}
	return e
}

func capitaliza(t string) string {
	if t == "" {
		return t
	}
	return strings.ToUpper(t[:1]) + t[1:]
}

// ---------- miembros ----------

// Miembro es una persona del grupo.
type Miembro struct {
	JID       string `json:"jid"`     // el que usa WhatsApp en ese grupo (teléfono o LID)
	Persona   string `json:"persona"` // clave canónica
	Nombre    string `json:"nombre"`
	Telefono  string `json:"telefono"`
	EsAdmin   bool   `json:"esAdmin"`
	EsCreador bool   `json:"esCreador"`
	Yo        bool   `json:"yo"`
}

func (s *Servicio) miembros(chat string, inf *types.GroupInfo) []Miembro {
	out := make([]Miembro, 0, len(inf.Participants))
	for _, p := range inf.Participants {
		m := Miembro{JID: p.JID.ToNonAD().String(), EsAdmin: p.IsAdmin || p.IsSuperAdmin, EsCreador: p.IsSuperAdmin, Yo: s.esYo(p)}
		formas := []string{p.PhoneNumber.ToNonAD().String(), p.JID.ToNonAD().String(), p.LID.ToNonAD().String()}
		if p.PhoneNumber.User != "" {
			m.Telefono = "+" + p.PhoneNumber.User
		} else if p.JID.Server == types.DefaultUserServer {
			m.Telefono = "+" + p.JID.User
		}
		m.Persona = s.persona(p.JID.ToNonAD().String())
		if p.PhoneNumber.User != "" {
			if c := s.persona(p.PhoneNumber.ToNonAD().String()); c != "" && !strings.HasSuffix(c, "@lid") {
				m.Persona = c
			}
		}
		if m.Yo {
			m.Persona = almacen.PersonaDueno
		}
		for _, f := range formas {
			if n := s.Alm.NombreConocido(s.BotID, chat, f); n != "" {
				m.Nombre = n
				break
			}
		}
		if m.Nombre == "" {
			m.Nombre = strings.TrimSpace(p.DisplayName)
		}
		if m.Nombre == "" {
			m.Nombre = m.Telefono
		}
		if m.Nombre == "" {
			m.Nombre = "Participante"
		}
		out = append(out, m)
	}
	return out
}

// buscaMiembro encuentra a una persona (clave canónica, número o JID en cualquiera de sus formas) entre los participantes.
func (s *Servicio) buscaMiembro(inf *types.GroupInfo, persona string) (types.GroupParticipant, bool) {
	objetivo := s.persona(persona)
	if objetivo == "" {
		return types.GroupParticipant{}, false
	}
	for _, p := range inf.Participants {
		if objetivo == almacen.PersonaDueno {
			if s.esYo(p) {
				return p, true
			}
			continue
		}
		for _, f := range []types.JID{p.PhoneNumber, p.JID, p.LID} {
			if f.IsEmpty() {
				continue
			}
			if s.persona(f.ToNonAD().String()) == objetivo || f.ToNonAD().String() == persona {
				return p, true
			}
		}
	}
	return types.GroupParticipant{}, false
}

// ---------- información (sin ser administrador) ----------

// InfoGrupo es la información de un grupo.
type InfoGrupo struct {
	Chat               string    `json:"chat"`
	Nombre             string    `json:"nombre"`
	Descripcion        string    `json:"descripcion"`
	Creado             int64     `json:"creado"`      // segundos; 0 si no se sabe
	CreadoTexto        string    `json:"creadoTexto"` // fecha como dd/mm/aaaa ("" si no se sabe)
	CreadoPor          string    `json:"creadoPor"`
	Miembros           int       `json:"miembros"`
	Admins             []Miembro `json:"admins"`
	SoyAdmin           bool      `json:"soyAdmin"`
	SoloAdmins         bool      `json:"soloAdmins"`         // solo administradores escriben (grupo cerrado)
	InfoBloqueada      bool      `json:"infoBloqueada"`      // solo administradores cambian nombre, foto y descripción
	AprobacionIngreso  bool      `json:"aprobacionIngreso"`  // entrar pide aprobación
	MensajesTemporales int       `json:"mensajesTemporales"` // segundos; 0 = apagados
	Reglas             string    `json:"reglas"`
}

// Info junta lo que WhatsApp sabe del grupo y las reglas que yo guardé.
func (s *Servicio) Info(ctx context.Context, chat string) (*InfoGrupo, error) {
	_, inf, err := s.preparar(ctx, chat, false)
	if err != nil {
		return nil, err
	}
	ms := s.miembros(chat, inf)
	r := &InfoGrupo{Chat: chat, Nombre: inf.Name, Descripcion: inf.Topic, Miembros: len(ms), Admins: []Miembro{}, SoyAdmin: s.soyAdminEn(inf),
		SoloAdmins: inf.IsAnnounce, InfoBloqueada: inf.IsLocked, AprobacionIngreso: inf.IsJoinApprovalRequired,
		Reglas: s.Alm.ReglasGrupo(s.BotID, chat)}
	if inf.ParticipantCount > r.Miembros {
		r.Miembros = inf.ParticipantCount
	}
	if !inf.GroupCreated.IsZero() {
		r.Creado = inf.GroupCreated.Unix()
		r.CreadoTexto = inf.GroupCreated.In(s.Alm.Zona()).Format("02/01/2006")
	}
	if inf.IsEphemeral {
		r.MensajesTemporales = int(inf.DisappearingTimer)
	}
	for _, m := range ms {
		if m.EsAdmin {
			r.Admins = append(r.Admins, m)
		}
		if m.EsCreador && r.CreadoPor == "" {
			r.CreadoPor = m.Nombre
		}
	}
	if r.CreadoPor == "" && !inf.OwnerJID.IsEmpty() {
		for _, m := range ms {
			if m.JID == inf.OwnerJID.ToNonAD().String() {
				r.CreadoPor = m.Nombre
			}
		}
	}
	return r, nil
}

// Admins lista a quienes administran el grupo.
func (s *Servicio) Admins(ctx context.Context, chat string) ([]Miembro, error) {
	_, inf, err := s.preparar(ctx, chat, false)
	if err != nil {
		return nil, err
	}
	out := []Miembro{}
	for _, m := range s.miembros(chat, inf) {
		if m.EsAdmin {
			out = append(out, m)
		}
	}
	return out, nil
}

// Miembros lista a todas las personas del grupo (administradores primero).
func (s *Servicio) Miembros(ctx context.Context, chat string) ([]Miembro, error) {
	_, inf, err := s.preparar(ctx, chat, false)
	if err != nil {
		return nil, err
	}
	ms := s.miembros(chat, inf)
	out := make([]Miembro, 0, len(ms))
	for _, m := range ms {
		if m.EsAdmin {
			out = append(out, m)
		}
	}
	for _, m := range ms {
		if !m.EsAdmin {
			out = append(out, m)
		}
	}
	return out, nil
}

// FijarReglas guarda las reglas del grupo ("" las quita). No necesita ser administrador: es un texto mío.
func (s *Servicio) FijarReglas(actor, chat, texto string) error {
	if _, err := parseaGrupo(chat); err != nil {
		return err
	}
	texto = strings.TrimSpace(texto)
	if utf8.RuneCountInString(texto) > 2000 {
		return ErrTextoLargo
	}
	if err := s.Alm.FijarReglasGrupo(s.BotID, chat, texto, s.ahora()); err != nil {
		s.registra(actor, "grupo.reglas", "", chat, "error", "")
		return err
	}
	s.registra(actor, "grupo.reglas", "", chat, "ok", "")
	return nil
}

// ---------- acciones de administrador ----------

// hacer corre una acción de administrador: revisa que sea administrador, la ejecuta, traduce el error y deja el registro (sin textos).
func (s *Servicio) hacer(ctx context.Context, actor, accion, chat, objetivo string, f func(j types.JID, inf *types.GroupInfo) error) error {
	j, inf, err := s.preparar(ctx, chat, true)
	if err != nil {
		res := "error"
		if err == ErrNoAdmin {
			res = "no_admin"
		}
		s.registra(actor, accion, objetivo, chat, res, "")
		return err
	}
	if err := f(j, inf); err != nil {
		s.Invalidar(chat) // quizá ya no soy administrador: la próxima vez se pregunta de nuevo
		e := Explicar(err)
		s.log("Grupo: la acción " + accion + " falló (" + err.Error() + ").")
		s.registra(actor, accion, objetivo, chat, "error", "")
		return e
	}
	s.Invalidar(chat)
	s.registra(actor, accion, objetivo, chat, "ok", "")
	return nil
}

// Cerrar deja el grupo cerrado (solo administradores escriben) o lo abre (todos escriben).
func (s *Servicio) Cerrar(ctx context.Context, actor, chat string, cerrado bool) error {
	acc := "grupo.abrir"
	if cerrado {
		acc = "grupo.cerrar"
	}
	return s.hacer(ctx, actor, acc, chat, "", func(j types.JID, _ *types.GroupInfo) error {
		return s.Cli.SetGroupAnnounce(ctx, j, cerrado)
	})
}

// BloquearInfo hace que solo los administradores cambien nombre, foto y descripción (o lo deja libre).
func (s *Servicio) BloquearInfo(ctx context.Context, actor, chat string, bloqueada bool) error {
	acc := "grupo.desbloquear_info"
	if bloqueada {
		acc = "grupo.bloquear_info"
	}
	return s.hacer(ctx, actor, acc, chat, "", func(j types.JID, _ *types.GroupInfo) error {
		return s.Cli.SetGroupLocked(ctx, j, bloqueada)
	})
}

// objetivoPermitido revisa que la acción sobre una persona sea posible (no yo, no quien creó el grupo, y la jerarquía de rangos).
func (s *Servicio) objetivoPermitido(actor string, p types.GroupParticipant) error {
	if s.esYo(p) {
		return ErrYoMismo
	}
	if p.IsSuperAdmin {
		return ErrCreador
	}
	if a := s.persona(actor); a != "" && a != almacen.PersonaDueno && s.Perm != nil {
		objetivo := s.persona(p.JID.ToNonAD().String())
		if p.PhoneNumber.User != "" {
			objetivo = s.persona(p.PhoneNumber.ToNonAD().String())
		}
		if a == objetivo {
			return ErrYoMismo
		}
		if !s.Perm.PuedeActuarSobre(a, objetivo) {
			return permisos.ErrJerarquia
		}
	}
	return nil
}

func (s *Servicio) cambiaParticipante(ctx context.Context, actor, accion, chat, persona string, cambio whatsmeow.ParticipantChange, valida func(types.GroupParticipant) error) error {
	return s.hacer(ctx, actor, accion, chat, persona, func(j types.JID, inf *types.GroupInfo) error {
		p, ok := s.buscaMiembro(inf, persona)
		if !ok {
			return ErrNoMiembro
		}
		if err := s.objetivoPermitido(actor, p); err != nil {
			return err
		}
		if valida != nil {
			if err := valida(p); err != nil {
				return err
			}
		}
		res, err := s.Cli.UpdateGroupParticipants(ctx, j, []types.JID{p.JID.ToNonAD()}, cambio)
		if err != nil {
			return err
		}
		for _, r := range res {
			if r.Error != 0 {
				return fallo(fmt.Sprintf("WhatsApp no pudo hacerlo con esa persona (código %d)", r.Error), nil)
			}
		}
		return nil
	})
}

// ExpulsarComo saca a una persona del grupo en nombre de `actor`. Quien llama ya pidió la confirmación con código cuando es un comando.
func (s *Servicio) ExpulsarComo(ctx context.Context, actor, chat, persona string) error {
	return s.cambiaParticipante(ctx, actor, "grupo.expulsar", chat, persona, whatsmeow.ParticipantChangeRemove, nil)
}

// Ascender convierte a una persona en administradora.
func (s *Servicio) Ascender(ctx context.Context, actor, chat, persona string) error {
	return s.cambiaParticipante(ctx, actor, "grupo.ascender", chat, persona, whatsmeow.ParticipantChangePromote, func(p types.GroupParticipant) error {
		if p.IsAdmin || p.IsSuperAdmin {
			return ErrYaAdmin
		}
		return nil
	})
}

// Bajar le quita a una persona el cargo de administradora.
func (s *Servicio) Bajar(ctx context.Context, actor, chat, persona string) error {
	return s.cambiaParticipante(ctx, actor, "grupo.bajar", chat, persona, whatsmeow.ParticipantChangeDemote, func(p types.GroupParticipant) error {
		if !p.IsAdmin && !p.IsSuperAdmin {
			return ErrNoEsAdmin
		}
		return nil
	})
}

// Resultados de AgregarMiembro.
const (
	AgregadoOK         = "ok"         // quedó dentro
	AgregadoYaEsta     = "ya_esta"    // ya era miembro
	AgregadoPrivacidad = "privacidad" // WhatsApp no la deja agregar (su privacidad, o salió hace poco): falla en silencio, hay que invitarla
)

// AgregarMiembro agrega a una persona al grupo con la acción real de WhatsApp (UpdateGroupParticipants "add"). La librería manda el
// token de privacidad de esa persona; sin él, o si su privacidad no lo permite (WhatsApp responde 403 o 408 sin avisarle), el resultado
// es AgregadoPrivacidad y quien llama debe caer a la invitación con enlace. Hay que ser administrador.
func (s *Servicio) AgregarMiembro(ctx context.Context, chat, persona string) (string, error) {
	res := AgregadoOK
	err := s.hacer(ctx, almacen.PersonaDueno, "grupo.agregar", chat, persona, func(j types.JID, inf *types.GroupInfo) error {
		jid, err := types.ParseJID(persona)
		if err != nil || jid.User == "" {
			return ErrSinPersona
		}
		if _, ok := s.buscaMiembro(inf, persona); ok {
			res = AgregadoYaEsta
			return nil
		}
		out, err := s.Cli.UpdateGroupParticipants(ctx, j, []types.JID{jid.ToNonAD()}, whatsmeow.ParticipantChangeAdd)
		if err != nil {
			if strings.Contains(err.Error(), "privacy token") {
				res = AgregadoPrivacidad
				return nil
			}
			return err
		}
		for _, r := range out {
			switch r.Error {
			case 0:
			case 409:
				res = AgregadoYaEsta
			case 403, 408:
				res = AgregadoPrivacidad
			default:
				return fallo(fmt.Sprintf("WhatsApp no pudo agregar a esa persona (código %d)", r.Error), nil)
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return res, nil
}

// Renombrar cambia el nombre del grupo.
func (s *Servicio) Renombrar(ctx context.Context, actor, chat, nombre string) error {
	nombre = strings.TrimSpace(nombre)
	if nombre == "" {
		return ErrTextoVacio
	}
	if utf8.RuneCountInString(nombre) > maxNombreGrupo {
		return fallo(fmt.Sprintf("el nombre es demasiado largo (máximo %d letras)", maxNombreGrupo), nil)
	}
	return s.hacer(ctx, actor, "grupo.renombrar", chat, "", func(j types.JID, _ *types.GroupInfo) error {
		return s.Cli.SetGroupName(ctx, j, nombre)
	})
}

// Descripcion cambia la descripción del grupo ("" la borra). Se pasa previousID vacío a propósito: la librería lee entonces la
// descripción actual de WhatsApp (la que está en memoria puede estar vieja) y manda el identificador correcto.
func (s *Servicio) Descripcion(ctx context.Context, actor, chat, texto string) error {
	texto = strings.TrimSpace(texto)
	if utf8.RuneCountInString(texto) > maxDescripcion {
		return fallo(fmt.Sprintf("la descripción es demasiado larga (máximo %d letras)", maxDescripcion), nil)
	}
	return s.hacer(ctx, actor, "grupo.descripcion", chat, "", func(j types.JID, _ *types.GroupInfo) error {
		return s.Cli.SetGroupTopic(ctx, j, "", "", texto)
	})
}

// Enlace da el enlace de invitación. reiniciar = revocar el anterior y crear uno nuevo (quien llama pide confirmación).
func (s *Servicio) Enlace(ctx context.Context, actor, chat string, reiniciar bool) (string, error) {
	acc := "grupo.enlace"
	if reiniciar {
		acc = "grupo.enlace_reiniciar"
	}
	var link string
	err := s.hacer(ctx, actor, acc, chat, "", func(j types.JID, _ *types.GroupInfo) error {
		l, err := s.Cli.GetGroupInviteLink(ctx, j, reiniciar)
		link = l
		return err
	})
	return link, err
}

// ---------- solicitudes de ingreso ----------

// Solicitud es alguien que pidió entrar al grupo.
type Solicitud struct {
	JID      string `json:"jid"`
	Persona  string `json:"persona"`
	Nombre   string `json:"nombre"`
	Telefono string `json:"telefono"`
	Ts       int64  `json:"ts"`
}

// Solicitudes lista a quienes piden entrar.
func (s *Servicio) Solicitudes(ctx context.Context, actor, chat string) ([]Solicitud, error) {
	var out []Solicitud
	err := s.hacer(ctx, actor, "grupo.solicitudes", chat, "", func(j types.JID, _ *types.GroupInfo) error {
		rs, err := s.Cli.GetGroupRequestParticipants(ctx, j)
		if err != nil {
			return err
		}
		for _, r := range rs {
			x := Solicitud{JID: r.JID.ToNonAD().String(), Persona: s.persona(r.JID.ToNonAD().String())}
			if !r.RequestedAt.IsZero() {
				x.Ts = r.RequestedAt.Unix()
			}
			if r.JID.Server == types.DefaultUserServer {
				x.Telefono = "+" + r.JID.User
			}
			x.Nombre = s.Alm.NombreConocido(s.BotID, chat, x.JID)
			if x.Nombre == "" {
				x.Nombre = x.Telefono
			}
			if x.Nombre == "" {
				x.Nombre = "Persona sin contacto guardado"
			}
			out = append(out, x)
		}
		return nil
	})
	if out == nil {
		out = []Solicitud{}
	}
	return out, err
}

// ResolverSolicitudes aprueba o rechaza solicitudes. personas vacío = todas las pendientes. Devuelve cuántas se resolvieron.
func (s *Servicio) ResolverSolicitudes(ctx context.Context, actor, chat string, personas []string, aprobar bool) (int, error) {
	acc := "grupo.rechazar"
	cambio := whatsmeow.ParticipantChangeReject
	if aprobar {
		acc, cambio = "grupo.aprobar", whatsmeow.ParticipantChangeApprove
	}
	hechas := 0
	err := s.hacer(ctx, actor, acc, chat, "", func(j types.JID, _ *types.GroupInfo) error {
		pend, err := s.Cli.GetGroupRequestParticipants(ctx, j)
		if err != nil {
			return err
		}
		if len(pend) == 0 {
			return ErrNoHaySolic
		}
		var elegidos []types.JID
		if len(personas) == 0 {
			for _, p := range pend {
				elegidos = append(elegidos, p.JID.ToNonAD())
			}
		} else {
			for _, q := range personas {
				cq := s.persona(q)
				for _, p := range pend {
					if p.JID.ToNonAD().String() == q || s.persona(p.JID.ToNonAD().String()) == cq {
						elegidos = append(elegidos, p.JID.ToNonAD())
						break
					}
				}
			}
		}
		if len(elegidos) == 0 {
			return fallo("no encontré a esa persona entre las solicitudes", nil)
		}
		res, err := s.Cli.UpdateGroupRequestParticipants(ctx, j, elegidos, cambio)
		if err != nil {
			return err
		}
		fallidas := 0
		for _, r := range res {
			if r.Error != 0 {
				fallidas++
			} else {
				hechas++
			}
		}
		if len(res) == 0 {
			hechas = len(elegidos)
		}
		if hechas == 0 && fallidas > 0 {
			return fallo("WhatsApp no pudo resolver esas solicitudes", nil)
		}
		return nil
	})
	return hechas, err
}

// ---------- borrar un mensaje ajeno ----------

// BorrarMensaje borra para todos un mensaje de otra persona del grupo (solo siendo administrador). Revisa la jerarquía de rangos
// cuando quien lo pide no es el dueño.
func (s *Servicio) BorrarMensaje(ctx context.Context, actor, chat, msgID string) error {
	return s.hacer(ctx, actor, "grupo.borrar_mensaje", chat, "", func(_ types.JID, _ *types.GroupInfo) error {
		if s.Borrar == nil {
			return ErrNoDisponible
		}
		m, _ := s.Alm.ObtenerMensaje(s.BotID, chat, msgID)
		if m == nil {
			return ErrSinMensaje
		}
		if a := s.persona(actor); !m.EsMio && a != almacen.PersonaDueno && s.Perm != nil {
			if !s.Perm.PuedeActuarSobre(a, s.persona(m.RemitenteJID)) {
				return permisos.ErrJerarquia
			}
		}
		return s.Borrar(ctx, chat, msgID)
	})
}

// ---------- AccionesGrupo (las usan las advertencias al llegar al tope y los filtros) ----------

var _ AccionesGrupo = (*Servicio)(nil)

// EsAdmin dice si el bot administra el grupo.
func (s *Servicio) EsAdmin(ctx context.Context, chat string) (bool, error) {
	if _, err := parseaGrupo(chat); err != nil {
		return false, nil
	}
	_, inf, err := s.preparar(ctx, chat, false)
	if err != nil {
		return false, err
	}
	return s.soyAdminEn(inf), nil
}

// BorrarMensajeAjeno borra un mensaje de otra persona (acción automática: la decide la regla del dueño, sin jerarquía).
func (s *Servicio) BorrarMensajeAjeno(ctx context.Context, chat, msgID, autor string) error {
	return s.BorrarMensaje(ctx, almacen.PersonaDueno, chat, msgID)
}

// Expulsar (AccionesGrupo) saca a una persona como lo pediría el dueño; ver ExpulsarComo para otros actores.
func (s *Servicio) Expulsar(ctx context.Context, chat, persona string) error {
	return s.ExpulsarComo(ctx, almacen.PersonaDueno, chat, persona)
}
