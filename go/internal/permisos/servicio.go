package permisos

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"botchat/internal/almacen"
)

// Servicio junta las reglas con el almacén para un bot: rangos de personas, límite de comandos por minuto, desconocidos y grupos.
type Servicio struct {
	Alm   *almacen.Almacen
	BotID int64
	Ahora func() time.Time
	// EsPropio dice si una persona (número o LID) es la cuenta del dueño: nadie puede tocar al Dueño aunque lo escriba como número.
	EsPropio func(persona string) bool
	// AlSolicitud se llama cuando llega una solicitud nueva (para avisar con una notificación neutra).
	AlSolicitud func(persona, nombre string)

	mu      sync.Mutex
	marcas  map[string][]time.Time
	avisoLm map[string]time.Time
}

// Nuevo crea el servicio y deja listos el Dueño y las plantillas.
func Nuevo(alm *almacen.Almacen, botID int64) (*Servicio, error) {
	s := &Servicio{Alm: alm, BotID: botID, Ahora: time.Now, marcas: map[string][]time.Time{}, avisoLm: map[string]time.Time{}}
	if err := alm.SembrarRoles(botID, Plantillas()); err != nil {
		return nil, err
	}
	return s, nil
}

// Persona da la clave canónica de una persona (número si se conoce).
func (s *Servicio) Persona(p string) string {
	if p == "" {
		return p
	}
	if s.EsPropio != nil && p != almacen.PersonaDueno && s.EsPropio(p) {
		return almacen.PersonaDueno
	}
	return s.Alm.ResolverPersona(s.BotID, p)
}

// RolDe devuelve el rango vigente de una persona (nil si no tiene).
func (s *Servicio) RolDe(persona string) *almacen.Rol {
	r, _, _ := s.Alm.RolDePersona(s.BotID, s.Persona(persona), s.Ahora())
	return r
}

// Puede dice si la persona tiene esa casilla en ese lugar.
func (s *Servicio) Puede(persona, casilla string, esGrupo bool, chat string) bool {
	return Permite(s.RolDe(persona), casilla, esGrupo, chat)
}

// PuedeActuarSobre aplica la jerarquía entre dos personas.
func (s *Servicio) PuedeActuarSobre(actor, objetivo string) bool {
	return PuedeActuar(s.RolDe(actor), s.RolDe(objetivo))
}

// Prefijo es el prefijo de los comandos ("!" por defecto).
func (s *Servicio) Prefijo() string {
	p := strings.TrimSpace(s.Alm.AjusteBot(s.BotID, "prefijo"))
	if p == "" {
		return "!"
	}
	return p
}

// Limitar cuenta un comando de la persona en el último minuto. Devuelve si puede seguir y si es la primera vez que se pasa
// (entonces se le avisa una vez; el resto se ignora sin contestar). El Dueño no tiene límite.
func (s *Servicio) Limitar(persona string) (ok bool, primerExceso bool) {
	persona = s.Persona(persona)
	if persona == almacen.PersonaDueno {
		return true, false
	}
	r := s.RolDe(persona)
	limite := s.Alm.AjusteBotInt(s.BotID, "limite_sin_rango")
	if r != nil {
		limite = r.LimiteMin
	}
	if limite <= 0 {
		return true, false
	}
	ahora := s.Ahora()
	s.mu.Lock()
	defer s.mu.Unlock()
	l := s.marcas[persona]
	i := 0
	for i < len(l) && ahora.Sub(l[i]) >= time.Minute {
		i++
	}
	l = l[i:]
	if len(l) >= limite {
		s.marcas[persona] = l
		if t, ok := s.avisoLm[persona]; !ok || ahora.Sub(t) >= time.Minute {
			s.avisoLm[persona] = ahora
			return false, true
		}
		return false, false
	}
	s.marcas[persona] = append(l, ahora)
	if len(s.marcas) > 5000 {
		for k, v := range s.marcas {
			if len(v) == 0 || ahora.Sub(v[len(v)-1]) > time.Minute {
				delete(s.marcas, k)
				delete(s.avisoLm, k)
			}
		}
	}
	return true, false
}

// ---------- rangos: operaciones con jerarquía ----------

// Errores que se le explican a quien pidió la acción.
var (
	ErrSinPermiso = errors.New("no tienes permiso para eso")
	ErrJerarquia  = errors.New("solo puedes actuar sobre rangos más bajos que el tuyo")
)

// Duración de un rango.
const (
	Dur24h       = 24 * time.Hour
	Dur7d        = 7 * 24 * time.Hour
	DurPermanent = 0
)

// CaducidadDe pasa "24h", "7d" o "perm" a la hora de vencimiento (0 = permanente).
func CaducidadDe(texto string, ahora time.Time) (int64, error) {
	switch strings.ToLower(strings.TrimSpace(texto)) {
	case "", "perm", "permanente", "siempre":
		return 0, nil
	case "24h", "24", "1d", "1dia", "1día", "dia", "día":
		return ahora.Add(Dur24h).Unix(), nil
	case "7d", "7", "7dias", "7días", "semana":
		return ahora.Add(Dur7d).Unix(), nil
	}
	return 0, fmt.Errorf("la duración debe ser 24h, 7d o permanente")
}

// AsignarComo da un rango a una persona en nombre de `actor`, respetando la jerarquía:
// el actor tiene que estar por encima del rango nuevo y de la persona (con su rango actual).
func (s *Servicio) AsignarComo(actor, objetivo string, rolID, caducaEn int64) error {
	actor, objetivo = s.Persona(actor), s.Persona(objetivo)
	if objetivo == almacen.PersonaDueno {
		return ErrJerarquia
	}
	ra, ro := s.RolDe(actor), s.RolDe(objetivo)
	nuevo, err := s.Alm.RolPorID(s.BotID, rolID)
	if err != nil {
		return err
	}
	if nuevo == nil {
		return errors.New("ese rango no existe")
	}
	if !PuedeDar(ra, nuevo) || (ro != nil && !PuedeActuar(ra, ro)) {
		return ErrJerarquia
	}
	if err := s.Alm.AsignarRol(s.BotID, objetivo, rolID, caducaEn, actor); err != nil {
		return err
	}
	s.Alm.RegistrarAccion(s.BotID, actor, "rango.asignar", objetivo, "", "ok", fmt.Sprintf("rol=%d", rolID))
	return nil
}

// QuitarComo revoca el rango de una persona en nombre de `actor`.
func (s *Servicio) QuitarComo(actor, objetivo string) error {
	actor, objetivo = s.Persona(actor), s.Persona(objetivo)
	if objetivo == almacen.PersonaDueno {
		return ErrJerarquia
	}
	ro := s.RolDe(objetivo)
	if ro == nil {
		return errors.New("esa persona no tiene rango")
	}
	if !PuedeActuar(s.RolDe(actor), ro) {
		return ErrJerarquia
	}
	if _, err := s.Alm.QuitarRol(s.BotID, objetivo); err != nil {
		return err
	}
	s.Alm.RegistrarAccion(s.BotID, actor, "rango.quitar", objetivo, "", "ok", fmt.Sprintf("rol=%d", ro.ID))
	return nil
}

// GuardarRol valida y guarda un rango nuevo (id 0) o existente. Devuelve el id.
func (s *Servicio) GuardarRol(r almacen.Rol) (int64, error) {
	if err := ValidarRol(&r); err != nil {
		return 0, err
	}
	if r.ID == 0 {
		return s.Alm.CrearRol(s.BotID, r)
	}
	return r.ID, s.Alm.GuardarRol(s.BotID, r)
}

// ---------- desconocidos y grupos ----------

// Políticas para desconocidos.
const (
	DescMenu    = "menu"    // responder el menú como siempre
	DescPronto  = "pronto"  // solo "te contestamos pronto" (una vez)
	DescAprobar = "aprobar" // esperar mi aprobación en silencio
	DescIgnorar = "ignorar" // ignorar sin guardar
)

// EsPoliticaDesconocidos dice si el valor es una política válida.
func EsPoliticaDesconocidos(v string) bool {
	switch v {
	case DescMenu, DescPronto, DescAprobar, DescIgnorar:
		return true
	}
	return false
}

// FijarPoliticaDesconocidos cambia la política. Al salir de "menu" se anota la hora: quien ya te había escrito antes cuenta como conocido.
func (s *Servicio) FijarPoliticaDesconocidos(v string) error {
	if !EsPoliticaDesconocidos(v) {
		return errors.New("política no válida")
	}
	if v != DescMenu && s.Alm.AjusteBot(s.BotID, "desconocidos") == DescMenu {
		if err := s.Alm.FijarAjusteBot(s.BotID, "desconocidos_desde", fmt.Sprint(s.Ahora().Unix())); err != nil {
			return err
		}
	}
	return s.Alm.FijarAjusteBot(s.BotID, "desconocidos", v)
}

// Entrada es lo que la compuerta necesita saber de un mensaje.
type Entrada struct {
	Persona string // quién escribe (clave guardada)
	Chat    string
	Nombre  string
	EsGrupo bool
	// EsAdmin dice si la persona es administradora del grupo (solo se llama cuando la política del grupo es "admins").
	EsAdmin func() bool
}

// Salida es la decisión de la compuerta.
type Salida struct {
	Pasa      bool
	Motivo    string // por qué no pasa (para el registro de actividad; sin textos)
	Responder string // texto que se manda en vez de seguir (política "te contestamos pronto")
}

// Compuerta decide si el bot atiende este mensaje: bloqueados, política de desconocidos (privado) y política del grupo (grupos).
func (s *Servicio) Compuerta(e Entrada) Salida {
	persona := s.Persona(e.Persona)
	sol, _ := s.Alm.ObtenerSolicitud(s.BotID, persona)
	if sol != nil && sol.Estado == almacen.SolBloqueada {
		return Salida{Motivo: "persona bloqueada"}
	}
	// Silenciada (por un rato o para siempre): el bot no le contesta ni le atiende comandos.
	if persona != almacen.PersonaDueno && s.Alm.SilencioDe(s.BotID, persona, s.Ahora()) != nil {
		return Salida{Motivo: "persona silenciada"}
	}
	if e.EsGrupo {
		p := s.Alm.PoliticaDeGrupo(s.BotID, e.Chat)
		switch p.Modo {
		case "admins":
			if e.EsAdmin != nil && e.EsAdmin() {
				return Salida{Pasa: true}
			}
			return Salida{Motivo: "grupo: solo administradores"}
		case "lista":
			for _, x := range p.Lista {
				if s.Persona(x) == persona {
					return Salida{Pasa: true}
				}
			}
			return Salida{Motivo: "grupo: fuera de la lista"}
		}
		return Salida{Pasa: true}
	}
	pol := s.Alm.AjusteBot(s.BotID, "desconocidos")
	if pol == DescMenu || !EsPoliticaDesconocidos(pol) {
		return Salida{Pasa: true}
	}
	if r := s.RolDe(persona); r != nil {
		return Salida{Pasa: true}
	}
	if sol != nil && sol.Estado == almacen.SolAprobada {
		return Salida{Pasa: true}
	}
	if c, _ := s.Alm.ObtenerChat(s.BotID, e.Chat); c != nil && c.Guardado != "" {
		return Salida{Pasa: true} // está en la agenda
	}
	if desde := int64(s.Alm.AjusteBotInt(s.BotID, "desconocidos_desde")); desde > 0 {
		if cr := s.Alm.CreadoChat(s.BotID, e.Chat); cr > 0 && cr < desde {
			return Salida{Pasa: true} // ya te había escrito antes de activar la política
		}
	}
	// Es un desconocido.
	if pol == DescIgnorar {
		return Salida{Motivo: "desconocido ignorado"}
	}
	res, err := s.Alm.RegistrarSolicitud(s.BotID, persona, e.Nombre, s.Alm.AjusteBotInt(s.BotID, "tope_pendientes"), s.Alm.AjusteBotInt(s.BotID, "ignorada_dias"), s.Ahora())
	if err != nil {
		return Salida{Motivo: "desconocido: error al guardar"}
	}
	if res.Nueva && res.Solicitud != nil && s.AlSolicitud != nil {
		s.AlSolicitud(persona, e.Nombre)
	}
	if res.Descartada {
		return Salida{Motivo: "desconocido: tope de pendientes"}
	}
	if res.Solicitud != nil && res.Solicitud.Estado == almacen.SolIgnorada {
		return Salida{Motivo: "desconocido ignorado"}
	}
	if pol == DescPronto && res.Solicitud != nil && res.Solicitud.AvisadaTS == 0 {
		s.Alm.MarcarSolicitudAvisada(s.BotID, persona, s.Ahora())
		t := strings.TrimSpace(s.Alm.AjusteBot(s.BotID, "desconocidos_texto"))
		if t == "" {
			t = "Gracias por escribir. Te contestamos pronto."
		}
		return Salida{Motivo: "desconocido: te contestamos pronto", Responder: t}
	}
	return Salida{Motivo: "desconocido: esperando aprobación"}
}

// Mantenimiento limpia rangos vencidos, códigos viejos y registro antiguo.
func (s *Servicio) Mantenimiento() {
	ahora := s.Ahora()
	s.Alm.LimpiarCaducados(s.BotID, ahora)
	s.Alm.LimpiarConfirmaciones(s.BotID, ahora)
	s.Alm.LimpiarAcciones(s.BotID, s.Alm.AjusteBotInt(s.BotID, "registro_dias"), ahora)
}
