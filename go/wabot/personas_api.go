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
	"botchat/internal/avisos"
	"botchat/internal/comandos"
	"botchat/internal/flujo"
	"botchat/internal/grupos"
	"botchat/internal/permisos"
	"botchat/internal/personas"
)

// ---------- armado dentro del bot ----------

// accionesGrupoWabot: saber si soy administrador es real (información del grupo vía el servicio de envío); borrar un mensaje ajeno
// y expulsar todavía no existen aquí: los crea la etapa E (grupos). Mientras tanto devuelven "no disponible" y el bot solo alerta.
type accionesGrupoWabot struct{ s *estado }

func (a accionesGrupoWabot) EsAdmin(ctx context.Context, chat string) (bool, error) {
	if a.s == nil || a.s.envio == nil {
		return false, grupos.ErrNoDisponible
	}
	return a.s.envio.SoyAdmin(ctx, chat)
}

func (accionesGrupoWabot) BorrarMensajeAjeno(context.Context, string, string, string) error {
	return grupos.ErrNoDisponible
}

func (accionesGrupoWabot) Expulsar(context.Context, string, string) error {
	return grupos.ErrNoDisponible
}

// armarPersonas crea los servicios de personas y avisos del bot y registra sus comandos. Va después de armarPermisos.
func (s *estado) armarPersonas() {
	if s.perm == nil || s.cmd == nil {
		return
	}
	s.pers = personas.Nuevo(alm, s.botID, s.perm)
	s.pers.Grupo = accionesGrupoWabot{s}
	s.pers.Log = s.log
	s.pers.RegistrarComandos(s.cmd)
	s.avs = avisos.Nuevo(alm, s.botID)
	s.avs.Log = s.log
	s.avs.RegistrarComandos(s.cmd)
	s.cmd.Extra = func(cx *comandos.Contexto) []string {
		return s.flujo.AyudaComandos(cx.EsGrupo, cx.Chat, cx.Persona, cx.Dueno)
	}
}

// puedeUsarRango dice si la persona tiene al menos el rango mínimo pedido (el Dueño siempre).
func puedeUsarRango(p *permisos.Servicio, botID int64, sender string, rolMin int64) bool {
	if p == nil {
		return true
	}
	r := p.RolDe(sender)
	if r == nil {
		return false
	}
	if r.EsDueno {
		return true
	}
	min, _ := alm.RolPorID(botID, rolMin)
	if min == nil {
		return true // el rango mínimo se borró: ya no hay requisito
	}
	return r.Posicion <= min.Posicion
}

// ---------- servicios sin el bot en marcha ----------

var (
	persLibres   = map[int64]*personas.Servicio{}
	persLibresMu sync.Mutex
)

// servicioPersonas da el servicio de personas del bot: el del bot en marcha si lo hay, o uno propio para usar la app sin conexión.
func servicioPersonas(botID int64) (*personas.Servicio, error) {
	mu.Lock()
	s := st
	a := alm
	mu.Unlock()
	if a == nil {
		return nil, errors.New("el almacén no está abierto")
	}
	if s != nil && s.botID == botID && s.pers != nil {
		return s.pers, nil
	}
	p, _, err := servicioPerm(botID)
	if err != nil {
		return nil, err
	}
	persLibresMu.Lock()
	defer persLibresMu.Unlock()
	if x, ok := persLibres[botID]; ok && x.Alm == a {
		return x, nil
	}
	x := personas.Nuevo(a, botID, p)
	persLibres[botID] = x
	return x, nil
}

func servicioAvisos(botID int64) (*avisos.Servicio, error) {
	mu.Lock()
	s := st
	mu.Unlock()
	if s != nil && s.botID == botID && s.avs != nil {
		return s.avs, nil
	}
	a, err := almacenAbierto()
	if err != nil {
		return nil, err
	}
	return avisos.Nuevo(a, botID), nil
}

// ---------- API de personas (la usa el dueño desde la app) ----------

func dueno() string { return almacen.PersonaDueno }

// FichaPersona devuelve (JSON) la ficha: nombre, números, rango, silencio/bloqueo, advertencias, notas, etiquetas, primera y última vez y conteos.
func FichaPersona(botID int64, persona string) (string, error) {
	p, err := servicioPersonas(botID)
	if err != nil {
		return "", err
	}
	return js(p.Ficha(dueno(), persona))
}

// EstadisticasPersona devuelve (JSON) los números de actividad de la persona (por tipo, hora, día y chat). dias limita la lista por día.
func EstadisticasPersona(botID int64, persona string, dias int) (string, error) {
	p, err := servicioPersonas(botID)
	if err != nil {
		return "", err
	}
	return js(p.Estadisticas(dueno(), persona, dias))
}

// SilenciarPersona hace que el bot no le conteste a esa persona durante `minutos` (0 = para siempre).
func SilenciarPersona(botID int64, persona string, minutos int, motivo string) error {
	p, err := servicioPersonas(botID)
	if err != nil {
		return err
	}
	if minutos < 0 {
		return errors.New("los minutos no pueden ser negativos")
	}
	return p.Silenciar(dueno(), persona, time.Duration(minutos)*time.Minute, motivo)
}

// QuitarSilencioPersona vuelve a atender a la persona. Devuelve si estaba silenciada.
func QuitarSilencioPersona(botID int64, persona string) (bool, error) {
	p, err := servicioPersonas(botID)
	if err != nil {
		return false, err
	}
	return p.Desilenciar(dueno(), persona)
}

// ListarSilencios devuelve (JSON) a quién se silenció y hasta cuándo.
func ListarSilencios(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ListarSilencios(botID, time.Now()))
}

// BloquearPersona hace que el bot ignore del todo a la persona (no es el bloqueo de WhatsApp).
func BloquearPersona(botID int64, persona string) error {
	p, err := servicioPersonas(botID)
	if err != nil {
		return err
	}
	return p.Bloquear(dueno(), persona)
}

// DesbloquearPersona quita el bloqueo del bot. Devuelve si estaba bloqueada.
func DesbloquearPersona(botID int64, persona string) (bool, error) {
	p, err := servicioPersonas(botID)
	if err != nil {
		return false, err
	}
	return p.Desbloquear(dueno(), persona)
}

// AdvertirPersona pone una advertencia. chat es el chat donde pasó ("" = su chat privado). Devuelve (JSON) {total, tope, llegoAlTope, accion, hecho, motivo}.
func AdvertirPersona(botID int64, persona, chat, motivo string) (string, error) {
	p, err := servicioPersonas(botID)
	if err != nil {
		return "", err
	}
	if chat == "" {
		chat = p.ChatPrivadoDe(persona)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return js(p.Advertir(ctx, dueno(), persona, chat, "", motivo))
}

// ListarAdvertencias devuelve (JSON) las advertencias de la persona, la más reciente primero.
func ListarAdvertencias(botID int64, persona string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ListarAdvertencias(botID, persona, 500))
}

// ListarPersonasAdvertidas devuelve (JSON) quién tiene advertencias: [{persona, total, ultimaTs, nombre}].
func ListarPersonasAdvertidas(botID int64) (string, error) {
	p, err := servicioPersonas(botID)
	if err != nil {
		return "", err
	}
	xs, err := p.Alm.ListarPersonasAdvertidas(botID)
	if err != nil {
		return "", err
	}
	type fila struct {
		almacen.ResumenAdvertencias
		Nombre string `json:"nombre"`
		Tope   int    `json:"tope"`
	}
	tope := p.Alm.AjusteBotInt(botID, "adv_tope")
	out := make([]fila, 0, len(xs))
	for _, x := range xs {
		out = append(out, fila{x, p.Nombre(x.Persona), tope})
	}
	return js(out, nil)
}

// QuitarAdvertenciaPersona quita una advertencia (id 0 = la última). Devuelve cuántas le quedan.
func QuitarAdvertenciaPersona(botID int64, persona string, id int64) (int, error) {
	p, err := servicioPersonas(botID)
	if err != nil {
		return 0, err
	}
	quedan, _, err := p.QuitarAdvertencia(dueno(), persona, id)
	return quedan, err
}

// BorrarAdvertenciasPersona borra todo el historial de advertencias de la persona. Devuelve cuántas había (la pantalla pide confirmar antes).
func BorrarAdvertenciasPersona(botID int64, persona string) (int, error) {
	p, err := servicioPersonas(botID)
	if err != nil {
		return 0, err
	}
	return p.BorrarHistorialAdvertencias(dueno(), persona)
}

// ListarNotasPersona devuelve (JSON) las notas de la persona.
func ListarNotasPersona(botID int64, persona string) (string, error) {
	p, err := servicioPersonas(botID)
	if err != nil {
		return "", err
	}
	return js(p.Notas(dueno(), persona))
}

// AgregarNotaPersona guarda una nota privada (cifrada) sobre la persona.
func AgregarNotaPersona(botID int64, persona, texto string) error {
	p, err := servicioPersonas(botID)
	if err != nil {
		return err
	}
	return p.AgregarNota(dueno(), persona, texto)
}

// BorrarNotaPersona quita una nota.
func BorrarNotaPersona(botID int64, persona string, id int64) error {
	p, err := servicioPersonas(botID)
	if err != nil {
		return err
	}
	return p.BorrarNota(dueno(), persona, id)
}

// OlvidarPersona borra todo lo de la persona (mensajes, archivos, notas, advertencias, sesiones y alias). Se mantiene su lugar en No molestar
// y el bloqueo del bot. Devuelve (JSON) lo que se borró. La pantalla pide confirmar antes.
func OlvidarPersona(botID int64, persona string) (string, error) {
	p, err := servicioPersonas(botID)
	if err != nil {
		return "", err
	}
	return js(p.Olvidar(dueno(), persona))
}

// ExportarPersona guarda los datos de la persona en un archivo cifrado BCF1 y devuelve su ruta relativa dentro de la carpeta de datos
// (la app lo comparte o lo abre con la llave de la app). Se borra solo a las 24 horas.
func ExportarPersona(botID int64, persona string) (string, error) {
	p, err := servicioPersonas(botID)
	if err != nil {
		return "", err
	}
	return p.Exportar(dueno(), persona)
}

// ---------- API de chat ----------

// MarcarEstadoChat pone el chat como "pendiente", "resuelto" o sin marca ("").
func MarcarEstadoChat(botID int64, chat, estado string) error {
	p, err := servicioPersonas(botID)
	if err != nil {
		return err
	}
	return p.MarcarChat(dueno(), chat, estado)
}

// ListarChatsFiltrados es ListarChats con filtro de estado ("", "pendiente", "resuelto") y de etiqueta (0 = todas).
func ListarChatsFiltrados(botID int64, grupos bool, consulta, estado string, etiqueta int64, limite, desde int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ListarChatsFiltrados(botID, grupos, consulta, almacen.FiltroChats{Estado: estado, Etiqueta: etiqueta}, limite, desde))
}

// ContarPendientes cuenta los chats marcados como pendientes.
func ContarPendientes(botID int64) (int, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	return a.ContarPendientes(botID), nil
}

// ListarEtiquetas devuelve (JSON) las etiquetas con cuántos chats tiene cada una.
func ListarEtiquetas(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ListarEtiquetas(botID))
}

// CrearEtiqueta crea una etiqueta local (color "#RRGGBB" o "" para el de siempre). Devuelve su id.
func CrearEtiqueta(botID int64, nombre, color string) (int64, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	id, err := a.CrearEtiqueta(botID, nombre, color)
	if err == nil {
		accionDueno(botID, "etiqueta.crear", "", "")
	}
	return id, err
}

// EditarEtiqueta cambia el nombre y el color de una etiqueta.
func EditarEtiqueta(botID, id int64, nombre, color string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.EditarEtiqueta(botID, id, nombre, color)
}

// BorrarEtiqueta borra la etiqueta y la quita de todos los chats.
func BorrarEtiqueta(botID, id int64) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	err = a.BorrarEtiqueta(botID, id)
	if err == nil {
		accionDueno(botID, "etiqueta.borrar", "", "")
	}
	return err
}

// PonerEtiquetaChat pone (si) o quita la etiqueta de un chat.
func PonerEtiquetaChat(botID int64, chat string, id int64, si bool) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.PonerEtiqueta(botID, chat, id, si)
}

// EtiquetasDeChat devuelve (JSON) las etiquetas de un chat.
func EtiquetasDeChat(botID int64, chat string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.EtiquetasDeChat(botID, chat))
}

// ---------- API de avisos para mí ----------

// ListarAlertasPalabra devuelve (JSON) las palabras clave: [{id, palabra, chats, activa}].
func ListarAlertasPalabra(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ListarAlertasPalabra(botID))
}

// CrearAlertaPalabra agrega una palabra o frase. chatsJSON: arreglo de chats donde vale ("[]" = todos). Devuelve su id.
func CrearAlertaPalabra(botID int64, palabra, chatsJSON string) (int64, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	var chats []string
	if strings.TrimSpace(chatsJSON) != "" {
		if err := json.Unmarshal([]byte(chatsJSON), &chats); err != nil {
			return 0, errors.New("la lista de chats no se entendió")
		}
	}
	return a.CrearAlertaPalabra(botID, palabra, chats)
}

// EditarAlertaPalabra cambia los chats donde vale y si está activa.
func EditarAlertaPalabra(botID, id int64, chatsJSON string, activa bool) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	var chats []string
	if strings.TrimSpace(chatsJSON) != "" {
		if err := json.Unmarshal([]byte(chatsJSON), &chats); err != nil {
			return errors.New("la lista de chats no se entendió")
		}
	}
	return a.EditarAlertaPalabra(botID, id, chats, activa)
}

// BorrarAlertaPalabra quita una palabra de la lista.
func BorrarAlertaPalabra(botID, id int64) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.BorrarAlertaPalabra(botID, id)
}

// ListarRecordatorios devuelve (JSON) los recordatorios pendientes y, con `hechos`, los últimos que ya sonaron.
func ListarRecordatorios(botID int64, hechos bool) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ListarRecordatorios(botID, hechos))
}

// CrearRecordatorio programa un aviso para `proximaTs` (segundos desde 1970). repetir: "", "diaria" o "semanal". Devuelve su id.
func CrearRecordatorio(botID int64, texto string, proximaTs int64, repetir string) (int64, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	if proximaTs <= time.Now().Unix() {
		return 0, errors.New("esa fecha y hora ya pasaron")
	}
	return a.CrearRecordatorio(botID, texto, proximaTs, repetir, time.Now())
}

// EditarRecordatorio cambia texto, hora y repetición (y lo deja pendiente).
func EditarRecordatorio(botID, id int64, texto string, proximaTs int64, repetir string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	if proximaTs <= time.Now().Unix() {
		return errors.New("esa fecha y hora ya pasaron")
	}
	return a.EditarRecordatorio(botID, id, texto, proximaTs, repetir)
}

// BorrarRecordatorio quita un recordatorio.
func BorrarRecordatorio(botID, id int64) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.BorrarRecordatorio(botID, id)
}

// ProximoAviso da la hora (segundos desde 1970) de la próxima alarma que Android debe programar: el siguiente recordatorio o el
// resumen del día. 0 = ninguna.
func ProximoAviso(botID int64) (int64, error) {
	v, err := servicioAvisos(botID)
	if err != nil {
		return 0, err
	}
	return v.Siguiente(), nil
}

// DispararAvisos crea los avisos que ya tocan (recordatorios vencidos y resumen del día). Devuelve (JSON) {avisos: [ids], siguiente}:
// la app muestra cada aviso como notificación y programa la alarma `siguiente`. Se puede llamar de más sin repetir nada.
func DispararAvisos(botID int64) (string, error) {
	v, err := servicioAvisos(botID)
	if err != nil {
		return "", err
	}
	return js(v.Disparar(), nil)
}

// VistaPreviaResumen devuelve el texto que tendría el resumen del día con lo de hoy.
func VistaPreviaResumen(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return avisos.TextoResumen(a.ResumenDelDia(botID, time.Now())), nil
}

// ValidarAjusteAvisos revisa un ajuste de personas y avisos. Devuelve "" si es válido.
func validarAjusteB(clave, valor string) (conocido bool, err error) {
	num := func(min, max int) error {
		var n int
		if _, e := fmt.Sscanf(valor, "%d", &n); e != nil || n < min || n > max || fmt.Sprint(n) != valor {
			return fmt.Errorf("el valor debe estar entre %d y %d", min, max)
		}
		return nil
	}
	switch clave {
	case "adv_tope":
		return true, num(0, 50)
	case "adv_accion":
		if !personas.EsAccionTope(valor) {
			return true, errors.New("la acción debe ser avisar, silenciar, borrar o expulsar")
		}
		return true, nil
	case "adv_silencio_min":
		return true, num(1, 525600)
	case "reportar_gracias", "alertas_activas", "alerta_fragmento", "resumen_activo", "avisos_exactos":
		return true, num(0, 1)
	case "reportar_texto":
		if len([]rune(valor)) > 200 {
			return true, errors.New("el texto es muy largo (máximo 200 letras)")
		}
		return true, nil
	case "auto_activo", "atajos_activo":
		return true, num(0, 1)
	case "auto_tope_hora":
		return true, num(0, 1000)
	case "auto_zona":
		if valor != "" {
			if _, e := time.LoadLocation(valor); e != nil {
				return true, errors.New("esa zona horaria no existe (por ejemplo America/Mexico_City)")
			}
		}
		return true, nil
	case "atajo_modo":
		if valor != "editar" && valor != "enviar" {
			return true, errors.New("el modo debe ser editar o enviar")
		}
		return true, nil
	case "resumen_hora":
		var h, m int
		if _, e := fmt.Sscanf(valor, "%d:%d", &h, &m); e != nil || h < 0 || h > 23 || m < 0 || m > 59 || len(valor) != 5 {
			return true, errors.New("la hora debe ser como 20:00")
		}
		return true, nil
	}
	return false, nil
}

// ---------- comandos propios ----------

type comandoPropioVista struct {
	ID     int64              `json:"id"`
	Activo bool               `json:"activo"`
	Datos  *flujo.DatosPropio `json:"datos"`
}

// ListarComandosPropios devuelve (JSON) los comandos propios: [{id, activo, datos}].
func ListarComandosPropios(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	menus, err := a.ListarMenus(botID)
	if err != nil {
		return "", err
	}
	out := []comandoPropioVista{}
	for _, m := range menus {
		if d, err := flujo.Parse(m.Def); err == nil && d.Propio != nil {
			out = append(out, comandoPropioVista{ID: m.ID, Activo: m.Activo, Datos: d.Propio})
		}
	}
	return js(out, nil)
}

// GuardarComandoPropio crea (id 0) o cambia un comando propio a partir de sus datos (JSON de DatosPropio). Los comandos nuevos nacen
// encendidos (solo responden). Devuelve el id. Rechaza nombres que ya usa otro comando propio o uno del bot (!ayuda, !rango…).
func GuardarComandoPropio(botID, id int64, datosJSON string) (int64, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	var dp flujo.DatosPropio
	if err := json.Unmarshal([]byte(datosJSON), &dp); err != nil {
		return 0, errors.New("el comando no se entendió")
	}
	def := &flujo.Def{V: 1, Propio: &dp}
	if err := def.ConstruirPropio(); err != nil {
		return 0, err
	}
	def.Normalizar()
	if d, err := despachadorDe(botID); err == nil && d.Existe(d.Prefijo()+dp.Nombre) {
		return 0, errors.New("«" + dp.Nombre + "» ya es un comando del bot: elige otro nombre")
	}
	menus, err := a.ListarMenus(botID)
	if err != nil {
		return 0, err
	}
	for _, m := range menus {
		if m.ID == id {
			continue
		}
		if d, err := flujo.Parse(m.Def); err == nil && d.Propio != nil && d.Propio.Nombre == dp.Nombre {
			return 0, errors.New("ya tienes un comando con ese nombre")
		}
	}
	for _, v := range def.Validar() {
		if v.Grave {
			return 0, errors.New(v.Texto)
		}
	}
	if id == 0 {
		id, err = a.CrearMenu(botID, def.JSON(), true, false)
	} else {
		var m *almacen.Menu
		if m, err = a.ObtenerMenu(botID, id); err == nil && m == nil {
			return 0, errors.New("ese comando ya no existe")
		}
		if err == nil {
			_, err = a.GuardarMenu(botID, id, def.JSON())
		}
	}
	if err != nil {
		return 0, err
	}
	accionDueno(botID, "comando_propio.guardar", "", "")
	return id, nil
}

// VistaPreviaComandoPropio devuelve el texto como lo vería quien escribe el comando (con {nombre}, {hora} y {dia} de ejemplo).
func VistaPreviaComandoPropio(botID int64, datosJSON string) (string, error) {
	var dp flujo.DatosPropio
	if err := json.Unmarshal([]byte(datosJSON), &dp); err != nil {
		return "", errors.New("el comando no se entendió")
	}
	if err := dp.Validar(); err != nil {
		return "", err
	}
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	pref := "!"
	if d, err := despachadorDe(botID); err == nil {
		pref = d.Prefijo()
	}
	var b strings.Builder
	b.WriteString("Escriben: " + pref + dp.Nombre + "\n\n")
	b.WriteString(flujo.TextoDeEjemplo(dp.Texto, "Ana", time.Now(), a.Zona()))
	for i, o := range dp.Opciones {
		fmt.Fprintf(&b, "\n%d. %s", i+1, o.Texto)
	}
	for i, o := range dp.Opciones {
		fmt.Fprintf(&b, "\n\nSi eligen %d:\n%s", i+1, flujo.TextoDeEjemplo(o.Respuesta, "Ana", time.Now(), a.Zona()))
	}
	return b.String(), nil
}
