package permisos

import (
	"strings"
	"time"

	"botchat/internal/almacen"
)

// RolUsuario es el rango «Usuario» de fábrica: lo reciben solas todas las personas que escriben al bot o están en sus grupos
// (menos quienes administran un grupo). Solo puede pedir descargas; se puede editar o borrar como cualquier rango.
func RolUsuario() almacen.Rol {
	return almacen.Rol{
		Nombre: "Usuario", Color: "#8696A0", Plantilla: "usuario", LimiteMin: 5, MenusModo: "todos",
		Casillas: map[string]almacen.Alcance{"video.pedir": {}},
	}
}

// UsuarioAutoActivo dice si se está dando el rango Usuario automáticamente.
func (s *Servicio) UsuarioAutoActivo() bool { return s.Alm.AjusteBot(s.BotID, "usuario_auto") != "0" }

// RolUsuarioID devuelve el id del rango Usuario, creándolo la primera vez. Si el dueño lo borró no se vuelve a crear solo (devuelve 0).
func (s *Servicio) RolUsuarioID() int64 {
	if id := int64(s.Alm.AjusteBotInt(s.BotID, "usuario_rol")); id > 0 {
		if r, _ := s.Alm.RolPorID(s.BotID, id); r != nil {
			return id
		}
		return 0
	}
	if s.Alm.AjusteBot(s.BotID, "usuario_creado") == "1" {
		return 0
	}
	id, err := s.Alm.CrearRol(s.BotID, RolUsuario())
	if err != nil {
		// ya había un rango con ese nombre: se usa ese
		if roles, e := s.Alm.ListarRoles(s.BotID); e == nil {
			for _, r := range roles {
				if strings.EqualFold(strings.TrimSpace(r.Nombre), "usuario") && !r.EsDueno {
					id, err = r.ID, nil
					break
				}
			}
		}
		if err != nil {
			return 0
		}
	}
	_ = s.Alm.FijarAjusteBot(s.BotID, "usuario_rol", itoa(id))
	_ = s.Alm.FijarAjusteBot(s.BotID, "usuario_creado", "1")
	return id
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// AsegurarUsuario le da el rango Usuario a una persona que acaba de escribir (si no tiene rango, no administra un grupo ni está bloqueada).
// Es barato: quien ya tiene rango solo cuesta una consulta, y a quien se decidió no darle (administra o está bloqueada) se le recuerda unos minutos.
func (s *Servicio) AsegurarUsuario(persona string) {
	if !s.UsuarioAutoActivo() || persona == "" {
		return
	}
	p := s.Persona(persona)
	if p == "" || p == almacen.PersonaDueno || s.Alm.TieneRango(s.BotID, p) {
		return
	}
	ahora := s.Ahora()
	s.mu.Lock()
	if s.usuariosVistos == nil {
		s.usuariosVistos = map[string]time.Time{}
	}
	if t, ok := s.usuariosVistos[p]; ok && ahora.Sub(t) < 10*time.Minute {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	id := s.RolUsuarioID()
	if id <= 0 {
		return
	}
	n, err := s.Alm.AsignarAuto(s.BotID, id, []string{p}, ahora)
	if n == 0 {
		// no se le dio: se anota por qué (en «Actividad del bot») y no se vuelve a intentar enseguida
		motivo := s.Alm.RazonNoUsuario(s.BotID, p)
		if err != nil {
			motivo = "error: " + err.Error()
		} else if motivo == "" {
			motivo = "no se pudo asignar"
		}
		s.Alm.RegistrarActividad(s.BotID, almacen.ActOmitido, "rango Usuario: "+motivo, "", "")
		s.mu.Lock()
		if len(s.usuariosVistos) > 5000 {
			s.usuariosVistos = map[string]time.Time{}
		}
		s.usuariosVistos[p] = ahora // no le toca (administra o está bloqueada)
		s.mu.Unlock()
	}
}

// QuitarUsuarioDeAdmin quita el rango automático a una persona que acaba de volverse administradora de un grupo.
func (s *Servicio) QuitarUsuarioDeAdmin(persona string) {
	s.Alm.QuitarAuto(s.BotID, s.Persona(persona))
	s.mu.Lock()
	delete(s.usuariosVistos, s.Persona(persona))
	s.mu.Unlock()
}

// RepartirUsuarios da el rango Usuario a todos los miembros de los grupos (o solo de uno) y a quienes tienen chat privado, y se lo quita
// a quienes ahora administran un grupo. Devuelve cuántas personas lo recibieron.
func (s *Servicio) RepartirUsuarios(grupo string) int {
	if !s.UsuarioAutoActivo() {
		return 0
	}
	id := s.RolUsuarioID()
	if id <= 0 {
		return 0
	}
	s.Alm.QuitarAutoDeAdmins(s.BotID)
	n, _ := s.Alm.AsignarAuto(s.BotID, id, s.Alm.PersonasParaUsuario(s.BotID, grupo), s.Ahora())
	return n
}

// FijarUsuarioAuto enciende o apaga el rango automático. Al encender reparte el rango; al apagar quita los que se dieron solos.
func (s *Servicio) FijarUsuarioAuto(si bool) (int, error) {
	v := "0"
	if si {
		v = "1"
	}
	if err := s.Alm.FijarAjusteBot(s.BotID, "usuario_auto", v); err != nil {
		return 0, err
	}
	s.mu.Lock()
	s.usuariosVistos = nil
	s.mu.Unlock()
	if si {
		return s.RepartirUsuarios(""), nil
	}
	return s.Alm.QuitarTodosAuto(s.BotID, 0), nil
}

// rolPropio es el rango que la persona tiene de verdad (dado a mano): el que se dio solo no cuenta para saber si es «conocida».
func (s *Servicio) rolPropio(persona string) *almacen.Rol {
	r, as, _ := s.Alm.RolDePersona(s.BotID, s.Persona(persona), s.Ahora())
	if r == nil || (as != nil && as.AsignadoPor == almacen.AsignadoAuto) {
		return nil
	}
	return r
}
