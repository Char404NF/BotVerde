package wabot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/permisos"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// ---------- armado dentro del bot ----------

// armarPermisos crea el servicio de rangos y el despachador de comandos del bot que se está iniciando.
func (s *estado) armarPermisos() {
	s.admins = &cacheAdmins{s: s, m: map[string]adminsGrupo{}}
	p, err := permisos.Nuevo(alm, s.botID)
	if err != nil {
		s.log("No se pudieron preparar los rangos: " + err.Error())
		return
	}
	p.EsPropio = func(persona string) bool {
		j, err := types.ParseJID(persona)
		return err == nil && s.esPropio(j)
	}
	p.AlSolicitud = func(persona, nombre string) {
		det := "Alguien que no conoces escribió. Revisa la bandeja de Solicitudes para aprobarlo, bloquearlo o ignorarlo."
		if nombre != "" {
			det = nombre + " escribió. Revisa la bandeja de Solicitudes para aprobarlo, bloquearlo o ignorarlo."
		}
		if _, err := alm.CrearAviso(s.botID, "desconocido", "", "Nueva solicitud de contacto", det); err == nil {
			s.log("Aviso local: nueva solicitud de contacto.")
		}
	}
	s.perm = p
	// Rango Usuario para quien ya escribió o está en grupos (en segundo plano: solo toca la base).
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() { _ = recover() }()
		p.RepartirUsuarios("")
	}()
	d := comandos.Nuevo(alm, s.botID, p)
	d.AlCodigo = func(c almacen.Confirmacion, desc string) { avisoCodigo(s.botID, c, desc, s.log) }
	d.Responder = func(chat, cita, texto string) {
		if m := s.motor; m != nil {
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				if err := m.ResponderA(chat, cita, texto); err != nil {
					s.log("No se pudo responder tras confirmar: " + err.Error())
				}
			}()
		}
	}
	s.cmd = d
}

// avisoCodigo crea el aviso local con el código de confirmación (la app lo muestra como notificación y como diálogo).
func avisoCodigo(botID int64, c almacen.Confirmacion, desc string, log func(string)) {
	min := int(time.Until(time.Unix(c.ExpiraTS, 0)).Round(time.Minute).Minutes())
	if min < 1 {
		min = 1
	}
	det := fmt.Sprintf("Código: %s\n%s\nVence en %d min.", c.Codigo, desc, min)
	if _, err := alm.CrearAviso(botID, "confirmacion", "", "Confirmación pendiente", det); err == nil && log != nil {
		log("Aviso local: confirmación pendiente.")
	}
}

// iniciarPermisos lanza la limpieza periódica (rangos vencidos, códigos viejos, registro antiguo).
func (s *estado) iniciarPermisos() {
	if s.perm == nil {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.perm.Mantenimiento()
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-t.C:
				s.perm.Mantenimiento()
			}
		}
	}()
}

// aprendeAlias anota qué número y qué LID son la misma persona, con lo que trae el mensaje y con el mapa de WhatsApp.
func (s *estado) aprendeAlias(v *events.Message) {
	aprende := func(x, y types.JID) {
		x, y = x.ToNonAD(), y.ToNonAD()
		if x.IsEmpty() || y.IsEmpty() || s.esPropio(x) || s.esPropio(y) {
			return
		}
		alm.AprenderAlias(s.botID, x.String(), y.String())
	}
	aprende(v.Info.Sender, v.Info.SenderAlt)
	if !v.Info.IsGroup {
		aprende(v.Info.Chat, v.Info.RecipientAlt)
		aprende(v.Info.Chat, v.Info.SenderAlt)
	}
	if s.cli == nil || s.cli.Store == nil || s.cli.Store.LIDs == nil {
		return
	}
	j := v.Info.Sender.ToNonAD()
	if v.Info.IsFromMe && !v.Info.IsGroup {
		j = v.Info.Chat.ToNonAD()
	}
	switch j.Server {
	case types.HiddenUserServer:
		if pn, err := s.cli.Store.LIDs.GetPNForLID(s.ctx, j); err == nil && !pn.IsEmpty() {
			aprende(j, pn)
		}
	case types.DefaultUserServer:
		if lid, err := s.cli.Store.LIDs.GetLIDForPN(s.ctx, j); err == nil && !lid.IsEmpty() {
			aprende(j, lid)
		}
	}
}

// cacheAdmins recuerda unos minutos quiénes administran cada grupo (para la política "solo administradores").
type cacheAdmins struct {
	s  *estado
	mu sync.Mutex
	m  map[string]adminsGrupo
}

type adminsGrupo struct {
	hasta  time.Time
	juntos map[string]bool // todas las formas (número y LID) de cada administrador
}

func (c *cacheAdmins) esAdmin(chat, persona string) bool {
	if c == nil || c.s == nil || c.s.perm == nil {
		return false
	}
	c.mu.Lock()
	e, ok := c.m[chat]
	c.mu.Unlock()
	if !ok || time.Now().After(e.hasta) {
		e = c.cargar(chat)
	}
	if e.juntos[persona] {
		return true
	}
	return e.juntos[c.s.perm.Persona(persona)]
}

// olvida borra lo recordado de un grupo (WhatsApp avisó de un cambio de administradores).
func (c *cacheAdmins) olvida(chat string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	delete(c.m, chat)
	c.mu.Unlock()
}

func (c *cacheAdmins) cargar(chat string) adminsGrupo {
	e := adminsGrupo{hasta: time.Now().Add(30 * time.Second), juntos: map[string]bool{}}
	defer func() {
		c.mu.Lock()
		if len(c.m) > 200 {
			c.m = map[string]adminsGrupo{}
		}
		c.m[chat] = e
		c.mu.Unlock()
	}()
	s := c.s
	if s.cli == nil || !s.cli.IsConnected() || !s.cli.IsLoggedIn() {
		return e
	}
	j, err := types.ParseJID(chat)
	if err != nil {
		return e
	}
	cx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	info, err := s.cli.GetGroupInfo(cx, j)
	if err != nil || info == nil {
		return e
	}
	e.hasta = time.Now().Add(5 * time.Minute)
	for _, p := range info.Participants {
		if !p.IsAdmin && !p.IsSuperAdmin {
			continue
		}
		for _, x := range []types.JID{p.JID, p.PhoneNumber, p.LID} {
			if !x.IsEmpty() {
				k := x.ToNonAD().String()
				e.juntos[k] = true
				e.juntos[s.perm.Persona(k)] = true
			}
		}
	}
	return e
}

// ---------- API para la pantalla ----------

var (
	permLibres   = map[int64]*permisos.Servicio{}
	permLibresMu sync.Mutex
)

// servicioPerm da el servicio de rangos del bot: el del bot en marcha si lo hay, o uno propio para ver y editar sin conexión.
func servicioPerm(botID int64) (*permisos.Servicio, *almacen.Almacen, error) {
	mu.Lock()
	s := st
	a := alm
	mu.Unlock()
	if a == nil {
		return nil, nil, errors.New("el almacén no está abierto")
	}
	if s != nil && s.botID == botID && s.perm != nil {
		return s.perm, a, nil
	}
	permLibresMu.Lock()
	defer permLibresMu.Unlock()
	if p, ok := permLibres[botID]; ok && p.Alm == a {
		return p, a, nil
	}
	p, err := permisos.Nuevo(a, botID)
	if err != nil {
		return nil, nil, err
	}
	permLibres[botID] = p
	return p, a, nil
}

func despachadorDe(botID int64) (*comandos.Despachador, error) {
	mu.Lock()
	s := st
	mu.Unlock()
	if s != nil && s.botID == botID && s.cmd != nil {
		return s.cmd, nil
	}
	p, a, err := servicioPerm(botID)
	if err != nil {
		return nil, err
	}
	d := comandos.Nuevo(a, botID, p)
	d.AlCodigo = func(c almacen.Confirmacion, desc string) { avisoCodigo(botID, c, desc, nil) }
	if ps, err := servicioPersonas(botID); err == nil {
		ps.RegistrarComandos(d)
	}
	if av, err := servicioAvisos(botID); err == nil {
		av.RegistrarComandos(d)
	}
	return d, nil
}

// CatalogoPermisos devuelve (JSON) todas las casillas que se le pueden dar a un rango, agrupadas, con su riesgo y dónde aplican.
func CatalogoPermisos() string { return permisos.CatalogoJSON() }

// ListarRangos devuelve (JSON) los rangos de arriba (Dueño) hacia abajo, con cuántas personas tiene cada uno.
func ListarRangos(botID int64) (string, error) {
	_, a, err := servicioPerm(botID)
	if err != nil {
		return "", err
	}
	return js(a.ListarRoles(botID))
}

// ObtenerRango devuelve (JSON) un rango, o "null".
func ObtenerRango(botID, id int64) (string, error) {
	_, a, err := servicioPerm(botID)
	if err != nil {
		return "", err
	}
	return js(a.RolPorID(botID, id))
}

// GuardarRango crea (id 0) o cambia un rango. rolJSON trae nombre, color, casillas (con su alcance), menús y límite por minuto.
// Devuelve el id. El Dueño no se puede editar.
func GuardarRango(botID int64, rolJSON string) (int64, error) {
	p, _, err := servicioPerm(botID)
	if err != nil {
		return 0, err
	}
	var r almacen.Rol
	if err := json.Unmarshal([]byte(rolJSON), &r); err != nil {
		return 0, errors.New("el rango no se entendió")
	}
	id, err := p.GuardarRol(r)
	if err == nil {
		accionDueno(botID, "rango.guardar", "", fmt.Sprintf("rol=%d", id))
	}
	return id, err
}

// BorrarRango quita un rango (sus personas quedan sin rango). Devuelve cuántas personas lo tenían.
func BorrarRango(botID, id int64) (int, error) {
	_, a, err := servicioPerm(botID)
	if err != nil {
		return 0, err
	}
	n, err := a.BorrarRol(botID, id)
	if err == nil {
		accionDueno(botID, "rango.borrar", "", fmt.Sprintf("rol=%d", id))
	}
	return n, err
}

// ReordenarRangos fija el orden de los rangos (idsJSON: arreglo con todos los ids menos el Dueño, de arriba a abajo).
func ReordenarRangos(botID int64, idsJSON string) error {
	_, a, err := servicioPerm(botID)
	if err != nil {
		return err
	}
	var ids []int64
	if err := json.Unmarshal([]byte(idsJSON), &ids); err != nil {
		return err
	}
	err = a.ReordenarRoles(botID, ids)
	if err == nil {
		accionDueno(botID, "rango.reordenar", "", "")
	}
	return err
}

func accionDueno(botID int64, accion, objetivo, detalle string) {
	if a, err := almacenAbierto(); err == nil {
		a.RegistrarAccion(botID, almacen.PersonaDueno, accion, objetivo, "", "ok", detalle)
	}
}

// AsignarRango da un rango a una persona desde la app. duracion: "24h", "7d" o "perm". persona es la clave del chat de esa persona.
func AsignarRango(botID int64, persona string, rolID int64, duracion string) error {
	p, a, err := servicioPerm(botID)
	if err != nil {
		return err
	}
	cad, err := permisos.CaducidadDe(duracion, time.Now())
	if err != nil {
		return err
	}
	persona = p.Persona(persona)
	if persona == almacen.PersonaDueno {
		return errors.New("el Dueño no puede recibir otro rango")
	}
	if err := a.AsignarRol(botID, persona, rolID, cad, almacen.PersonaDueno); err != nil {
		return err
	}
	accionDueno(botID, "rango.asignar", persona, fmt.Sprintf("rol=%d", rolID))
	return nil
}

// RevocarRango quita el rango de una persona desde la app.
func RevocarRango(botID int64, persona string) error {
	p, a, err := servicioPerm(botID)
	if err != nil {
		return err
	}
	persona = p.Persona(persona)
	hubo, err := a.QuitarRol(botID, persona)
	if err == nil && hubo {
		accionDueno(botID, "rango.quitar", persona, "")
	}
	return err
}

type rangoDePersona struct {
	Rol        *almacen.Rol        `json:"rol"`
	Asignacion *almacen.Asignacion `json:"asignacion"`
	Bloqueada  bool                `json:"bloqueada"`
}

// RangoDePersona devuelve (JSON) el rango vigente de una persona: {rol, asignacion, bloqueada}. rol es null si no tiene.
func RangoDePersona(botID int64, persona string) (string, error) {
	p, a, err := servicioPerm(botID)
	if err != nil {
		return "", err
	}
	persona = p.Persona(persona)
	r, as, err := a.RolDePersona(botID, persona, time.Now())
	if err != nil {
		return "", err
	}
	out := rangoDePersona{Rol: r, Asignacion: as}
	if sol, _ := a.ObtenerSolicitud(botID, persona); sol != nil && sol.Estado == almacen.SolBloqueada {
		out.Bloqueada = true
	}
	return js(out, nil)
}

// ListarAsignaciones devuelve (JSON) las personas con rango vigente.
func ListarAsignaciones(botID int64) (string, error) {
	_, a, err := servicioPerm(botID)
	if err != nil {
		return "", err
	}
	return js(a.ListarAsignaciones(botID))
}

// ListarSolicitudes devuelve (JSON) la bandeja: estado "pendiente" (por defecto), "ignorada", "bloqueada" o "aprobada".
func ListarSolicitudes(botID int64, estado string) (string, error) {
	_, a, err := servicioPerm(botID)
	if err != nil {
		return "", err
	}
	return js(a.ListarSolicitudes(botID, estado, 300))
}

// ContarSolicitudes devuelve cuántas solicitudes pendientes hay.
func ContarSolicitudes(botID int64) (int, error) {
	_, a, err := servicioPerm(botID)
	if err != nil {
		return 0, err
	}
	return a.ContarSolicitudes(botID, almacen.SolPendiente), nil
}

func decidirSolicitud(botID int64, persona, estado string) error {
	p, a, err := servicioPerm(botID)
	if err != nil {
		return err
	}
	persona = p.Persona(persona)
	if err := a.DecidirSolicitud(botID, persona, estado, time.Now()); err != nil {
		return err
	}
	accionDueno(botID, "solicitud."+estado, persona, "")
	return nil
}

// AprobarSolicitud acepta a una persona: el bot le contesta en privado. NO autoriza grupos (cada grupo tiene su propia política).
func AprobarSolicitud(botID int64, persona string) error {
	return decidirSolicitud(botID, persona, almacen.SolAprobada)
}

// BloquearSolicitud hace que el bot ignore por completo a esa persona.
func BloquearSolicitud(botID int64, persona string) error {
	return decidirSolicitud(botID, persona, almacen.SolBloqueada)
}

// IgnorarSolicitud descarta la solicitud sin bloquear: pasados unos días, si vuelve a escribir, vuelve a la bandeja.
func IgnorarSolicitud(botID int64, persona string) error {
	return decidirSolicitud(botID, persona, almacen.SolIgnorada)
}

// QuitarSolicitud saca a una persona de la lista (aprobada, bloqueada o ignorada): vuelve a ser una persona normal.
func QuitarSolicitud(botID int64, persona string) error {
	p, a, err := servicioPerm(botID)
	if err != nil {
		return err
	}
	persona = p.Persona(persona)
	if err := a.BorrarSolicitud(botID, persona); err != nil {
		return err
	}
	accionDueno(botID, "solicitud.quitar", persona, "")
	return nil
}

// PoliticaDeGrupo devuelve (JSON) {chat, modo, lista}: "todos", "admins" o "lista".
func PoliticaDeGrupo(botID int64, chat string) (string, error) {
	_, a, err := servicioPerm(botID)
	if err != nil {
		return "", err
	}
	return js(a.PoliticaDeGrupo(botID, chat), nil)
}

// FijarPoliticaDeGrupo guarda a quién le contesta el bot en un grupo (JSON {chat, modo, lista}).
func FijarPoliticaDeGrupo(botID int64, politicaJSON string) error {
	_, a, err := servicioPerm(botID)
	if err != nil {
		return err
	}
	var p almacen.PoliticaGrupo
	if err := json.Unmarshal([]byte(politicaJSON), &p); err != nil {
		return errors.New("la política no se entendió")
	}
	if p.Chat == "" {
		return errors.New("falta el grupo")
	}
	if err := a.FijarPoliticaGrupo(botID, p); err != nil {
		return err
	}
	accionDueno(botID, "grupo.politica", "", "modo="+p.Modo)
	return nil
}

// ListarPoliticasDeGrupo devuelve (JSON) las políticas de los grupos donde se cambió algo.
func ListarPoliticasDeGrupo(botID int64) (string, error) {
	_, a, err := servicioPerm(botID)
	if err != nil {
		return "", err
	}
	return js(a.ListarPoliticasGrupo(botID))
}

// AjustesDePermisos devuelve (JSON) los ajustes: prefijo, desconocidos, desconocidos_texto, tope_pendientes, ignorada_dias,
// confirm_seg, limite_sin_rango, grupo_modo_defecto, registro_dias.
func AjustesDePermisos(botID int64) (string, error) {
	_, a, err := servicioPerm(botID)
	if err != nil {
		return "", err
	}
	m := a.AjustesBot(botID)
	delete(m, "desconocidos_desde")
	return js(m, nil)
}

// FijarAjusteDePermisos cambia un ajuste (con validación). Lo técnico (prefijo, tope, registro) está en Avanzado.
func FijarAjusteDePermisos(botID int64, clave, valor string) error {
	p, a, err := servicioPerm(botID)
	if err != nil {
		return err
	}
	valor = strings.TrimSpace(valor)
	num := func(min, max int) error {
		var n int
		if _, err := fmt.Sscanf(valor, "%d", &n); err != nil || n < min || n > max {
			return fmt.Errorf("el valor debe estar entre %d y %d", min, max)
		}
		return nil
	}
	switch clave {
	case "prefijo":
		if len([]rune(valor)) < 1 || len([]rune(valor)) > 3 || strings.ContainsAny(valor, " \n\t") {
			return errors.New("el prefijo debe tener de 1 a 3 símbolos sin espacios")
		}
		for _, r := range valor {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
				return errors.New("el prefijo debe ser un símbolo (por ejemplo ! # / .)")
			}
		}
	case "desconocidos":
		if err := p.FijarPoliticaDesconocidos(valor); err != nil {
			return err
		}
		accionDueno(botID, "ajuste.desconocidos", "", "valor="+valor)
		return nil
	case "usuario_auto":
		if valor != "0" && valor != "1" {
			return errors.New("valor no válido")
		}
		if _, err := p.FijarUsuarioAuto(valor == "1"); err != nil {
			return err
		}
		accionDueno(botID, "ajuste.usuario_auto", "", "valor="+valor)
		return nil
	case "desconocidos_texto":
		if len([]rune(valor)) > 300 {
			return errors.New("el texto es muy largo (máximo 300 letras)")
		}
	case "tope_pendientes":
		if err := num(1, 1000); err != nil {
			return err
		}
	case "ignorada_dias":
		if err := num(1, 365); err != nil {
			return err
		}
	case "confirm_seg":
		if err := num(15, 900); err != nil {
			return err
		}
	case "limite_sin_rango":
		if err := num(0, 600); err != nil {
			return err
		}
	case "grupo_modo_defecto":
		if valor != "todos" && valor != "admins" && valor != "lista" {
			return errors.New("modo no válido")
		}
	case "registro_dias":
		if err := num(1, 3650); err != nil {
			return err
		}
	default:
		conocido, err := validarAjusteB(clave, valor)
		if !conocido {
			return errors.New("ajuste desconocido")
		}
		if err != nil {
			return err
		}
	}
	if err := a.FijarAjusteBot(botID, clave, valor); err != nil {
		return err
	}
	accionDueno(botID, "ajuste."+clave, "", "")
	return nil
}

// ConfirmacionesPendientes devuelve (JSON) los códigos de confirmación vigentes con una descripción: [{codigo, comando, persona, expiraTs}].
func ConfirmacionesPendientes(botID int64) (string, error) {
	_, a, err := servicioPerm(botID)
	if err != nil {
		return "", err
	}
	return js(a.ConfirmacionesPendientes(botID, time.Now()))
}

// ConfirmarCodigo aprueba desde la app una acción pendiente (el dueño la ve en el diálogo). Ejecuta la acción de quien la pidió y,
// si la pidió en un chat, le responde ahí. Devuelve el texto del resultado.
func ConfirmarCodigo(botID int64, codigo string) (string, error) {
	d, err := despachadorDe(botID)
	if err != nil {
		return "", err
	}
	return d.Confirmar(codigo, "")
}

// CancelarConfirmacion descarta un código pendiente.
func CancelarConfirmacion(botID int64, codigo string) error {
	_, a, err := servicioPerm(botID)
	if err != nil {
		return err
	}
	return a.CancelarConfirmacion(botID, codigo)
}

type resultadoComando struct {
	Manejado bool   `json:"manejado"`
	Texto    string `json:"texto"`
}

// EjecutarComando corre un comando como Dueño desde la app (por ejemplo "!ayuda"). Devuelve (JSON) {manejado, texto}.
func EjecutarComando(botID int64, texto string) (string, error) {
	d, err := despachadorDe(botID)
	if err != nil {
		return "", err
	}
	r := d.Ejecutar(comandos.Entrada{Persona: almacen.PersonaDueno, Chat: "", Texto: texto, Dueno: true})
	return js(resultadoComando{Manejado: r.Manejado, Texto: r.Texto}, nil)
}

// ListarAcciones devuelve (JSON) el registro de acciones (quién, qué, cuándo, resultado; sin textos), lo más nuevo primero.
// antesId > 0 pide la página siguiente.
func ListarAcciones(botID int64, limite int, antesID int64) (string, error) {
	_, a, err := servicioPerm(botID)
	if err != nil {
		return "", err
	}
	return js(a.ListarAcciones(botID, limite, antesID))
}
