package wabot

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/flujo"
	"botchat/internal/motor"
	"botchat/internal/permisos"
)

var flujoExpiraMin atomic.Int32

func init() { flujoExpiraMin.Store(120) }

// ConfigurarFlujo fija cuántos minutos sin actividad caduca una conversación dentro de un menú (por defecto 120).
func ConfigurarFlujo(expiraMin int) {
	if expiraMin < 5 {
		expiraMin = 120
	}
	flujoExpiraMin.Store(int32(expiraMin))
}

func expiraFlujo() time.Duration { return time.Duration(flujoExpiraMin.Load()) * time.Minute }

// decisorPrueba permite a las pruebas cambiar quién decide las respuestas.
var decisorPrueba motor.Decisor

func decisorActual() motor.Decisor { return decisorPrueba }

// nuevoFlujo arma el motor de menús para el bot que se está iniciando.
func (s *estado) nuevoFlujo() *flujo.Motor {
	return flujo.Nuevo(flujo.Config{
		BotID: s.botID, Alm: alm, Zona: alm.Zona, Expira: expiraFlujo,
		Nombre:  func(chat, sender string) string { return nombreDe(s.botID, chat, sender) },
		Prefijo: func() string { return alm.AjusteBot(s.botID, "prefijo") },
		PuedeUsar: func(sender string, rolMin int64) bool {
			return puedeUsarRango(s.perm, s.botID, sender, rolMin)
		},
		MenuVisible: func(sender string, menuID int64) bool {
			if s.perm == nil {
				return true
			}
			return permisos.MenuVisible(s.perm.RolDe(sender), fmt.Sprint(menuID), "")
		},
		OpcionVisible: func(sender string, menuID int64, opcionID string) bool {
			if s.perm == nil {
				return true
			}
			return permisos.OpcionVisible(s.perm.RolDe(sender), fmt.Sprint(menuID), opcionID)
		},
		// Las conversaciones de grupo van por (grupo, persona canónica): el número y el LID de la misma persona son una sola.
		Persona: func(p string) string {
			if s.perm == nil {
				return p
			}
			return s.perm.Persona(p)
		},
		AlAviso: func(chat, tipo, titulo, detalle string) {
			if _, err := alm.CrearAviso(s.botID, tipo, chat, titulo, detalle); err == nil {
				s.log("Aviso local: " + titulo + ".")
			}
		},
		// {api_<nombre>}: texto de un conector HTTP (módulo Descargas y conectores, si está encendido)
		Conector: func(nombre, sender, chat string) string {
			if s.mods == nil {
				return ""
			}
			return s.mods.TextoConector(nombre, sender, chat)
		},
	})
}

func nombreDe(botID int64, chat, sender string) string {
	if alm == nil {
		return ""
	}
	if c, _ := alm.ObtenerChat(botID, chat); c != nil && !c.EsGrupo {
		return c.Mostrado
	}
	return alm.NombreConocido(botID, chat, sender)
}

// ---------- API para la pantalla ----------

type resumenAuto struct {
	ID      int64  `json:"id"`
	Activo  bool   `json:"activo"`
	Sistema bool   `json:"sistema"`
	Version int    `json:"version"`
	Nombre  string `json:"nombre"`
	Resumen string `json:"resumen"`
	Avisos  int    `json:"avisos"`
	Pasos   int    `json:"pasos"`
	Orden   int    `json:"orden"`
}

func resumenDe(d *flujo.Def) string {
	var t []string
	if d.Disparadores.Cualquiera {
		t = append(t, "cualquier mensaje")
	}
	if d.Disparadores.PrimerMensaje {
		t = append(t, "primer mensaje")
	}
	if d.Disparadores.DiasSinEscribir > 0 {
		t = append(t, fmt.Sprintf("tras %d días sin escribir", d.Disparadores.DiasSinEscribir))
	}
	if len(d.Disparadores.Palabras) > 0 {
		t = append(t, "palabras clave")
	}
	if d.Disparadores.Comando != "" {
		t = append(t, "comando "+d.Disparadores.Comando)
	}
	if d.Disparadores.FueraDeHorario {
		t = append(t, "fuera de horario")
	}
	if d.Disparadores.Mencion {
		t = append(t, "mención")
	}
	if len(t) == 0 {
		return "Sin disparador"
	}
	return strings.ToUpper(t[0][:1]) + t[0][1:] + func() string {
		if len(t) > 1 {
			return " y " + fmt.Sprint(len(t)-1) + " más"
		}
		return ""
	}()
}

// ListarAutomatizaciones devuelve (JSON) las automatizaciones en su orden de evaluación.
func ListarAutomatizaciones(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	menus, err := a.ListarMenus(botID)
	if err != nil {
		return "", err
	}
	out := []resumenAuto{}
	for _, m := range menus {
		d, err := flujo.Parse(m.Def)
		if err != nil || d.Propio != nil { // los comandos propios tienen su propia pantalla
			continue
		}
		graves := 0
		for _, v := range d.Validar() {
			if v.Grave {
				graves++
			}
		}
		out = append(out, resumenAuto{ID: m.ID, Activo: m.Activo, Sistema: m.Sistema, Version: m.Version, Nombre: d.Nombre, Resumen: resumenDe(d), Avisos: graves, Pasos: len(d.Pasos), Orden: m.Orden})
	}
	return js(out, nil)
}

type detalleAuto struct {
	ID      int64         `json:"id"`
	Activo  bool          `json:"activo"`
	Sistema bool          `json:"sistema"`
	Version int           `json:"version"`
	Def     *flujo.Def    `json:"def"`
	Avisos  []flujo.Aviso `json:"avisos"`
}

// ObtenerAutomatizacion devuelve (JSON) la definición completa y sus avisos de validación.
func ObtenerAutomatizacion(botID, id int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	m, err := a.ObtenerMenu(botID, id)
	if err != nil || m == nil {
		return "", errors.New("Esa automatización ya no existe.")
	}
	d, err := flujo.Parse(m.Def)
	if err != nil {
		return "", err
	}
	return js(detalleAuto{ID: m.ID, Activo: m.Activo, Sistema: m.Sistema, Version: m.Version, Def: d, Avisos: d.Validar()}, nil)
}

// ValidarAutomatizacion revisa una definición sin guardarla y devuelve (JSON) sus avisos.
func ValidarAutomatizacion(defJSON string) (string, error) {
	d, err := flujo.Parse([]byte(defJSON))
	if err != nil {
		return "", err
	}
	d.Normalizar()
	return js(d.Validar(), nil)
}

// GuardarAutomatizacion guarda una definición: con id 0 crea una nueva (apagada si tiene avisos graves) y con id existente crea una versión
// nueva. Devuelve (JSON) {id, version, avisos}.
func GuardarAutomatizacion(botID, id int64, defJSON string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	d, err := flujo.Parse([]byte(defJSON))
	if err != nil {
		return "", err
	}
	d.Normalizar()
	av := d.Validar()
	version := 1
	if id == 0 {
		id, err = a.CrearMenu(botID, d.JSON(), false, false)
	} else {
		version, err = a.GuardarMenu(botID, id, d.JSON())
	}
	if err != nil {
		return "", err
	}
	return js(map[string]any{"id": id, "version": version, "avisos": av}, nil)
}

// ActivarAutomatizacion enciende o apaga una automatización. No se puede encender una que tenga avisos graves.
func ActivarAutomatizacion(botID, id int64, si bool) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	if si {
		if m, _ := a.ObtenerMenu(botID, id); m != nil {
			if d, err := flujo.Parse(m.Def); err == nil {
				for _, v := range d.Validar() {
					if v.Grave {
						return errors.New("Corrige los avisos en rojo antes de encenderla: " + v.Texto)
					}
				}
			}
		}
	}
	return a.FijarMenuActivo(botID, id, si)
}

// DuplicarAutomatizacion crea una copia (apagada) con el nombre "… (copia)". Devuelve el id nuevo.
func DuplicarAutomatizacion(botID, id int64) (int64, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	m, err := a.ObtenerMenu(botID, id)
	if err != nil || m == nil {
		return 0, errors.New("Esa automatización ya no existe.")
	}
	d, err := flujo.Parse(m.Def)
	if err != nil {
		return 0, err
	}
	d.Nombre += " (copia)"
	return a.CrearMenu(botID, d.JSON(), false, false)
}

// BorrarAutomatizacion la quita; sus conversaciones en curso se cierran.
func BorrarAutomatizacion(botID, id int64) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.BorrarMenu(botID, id)
}

// ReordenarAutomatizaciones fija el orden en que se revisan (idsJSON: arreglo de ids).
func ReordenarAutomatizaciones(botID int64, idsJSON string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	var ids []int64
	if err := json.Unmarshal([]byte(idsJSON), &ids); err != nil {
		return err
	}
	return a.ReordenarMenus(botID, ids)
}

// PlantillasAutomatizacion devuelve (JSON) las plantillas de arranque.
func PlantillasAutomatizacion() (string, error) { return js(flujo.Plantillas(), nil) }

// ExportarAutomatizacion devuelve el JSON de una automatización para compartirlo.
func ExportarAutomatizacion(botID, id int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	m, err := a.ObtenerMenu(botID, id)
	if err != nil || m == nil {
		return "", errors.New("Esa automatización ya no existe.")
	}
	d, err := flujo.Parse(m.Def)
	if err != nil {
		return "", err
	}
	d.Alcance.Chats, d.Alcance.Grupos = nil, nil // los chats son de este teléfono: no se exportan
	b, _ := json.MarshalIndent(d, "", "  ")
	return string(b), nil
}

// ImportarAutomatizacion crea una automatización (apagada) a partir de un JSON exportado. Devuelve (JSON) {id, avisos}.
func ImportarAutomatizacion(botID int64, defJSON string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	d, err := flujo.Parse([]byte(defJSON))
	if err != nil || len(d.Pasos) == 0 {
		return "", errors.New("Ese archivo no es una automatización de Bot Chat.")
	}
	d.Alcance.Chats, d.Alcance.Grupos = nil, nil
	d.Normalizar()
	id, err := a.CrearMenu(botID, d.JSON(), false, false)
	if err != nil {
		return "", err
	}
	return js(map[string]any{"id": id, "avisos": d.Validar()}, nil)
}

// SimularAutomatizacion escribe un mensaje falso a una automatización (con el mismo motor, marcado como prueba y sin enviar nada).
func SimularAutomatizacion(botID, id int64, texto string, reiniciar bool) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	fm := flujo.Nuevo(flujo.Config{BotID: botID, Alm: a, Zona: a.Zona, Expira: expiraFlujo, Nombre: func(chat, sender string) string { return "María (prueba)" }})
	p, err := fm.Probar(id, texto, reiniciar, "")
	if err != nil {
		return "", err
	}
	return js(p, nil)
}

// SesionesEnCurso devuelve (JSON) quién está dentro de un menú ahora.
func SesionesEnCurso(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	l, err := a.SesionesEnCurso(botID, time.Now(), expiraFlujo())
	if err != nil {
		return "", err
	}
	type fila struct {
		almacen.SesionEnCurso
		Menu string `json:"menu"`
		Ruta string `json:"ruta"`
	}
	out := []fila{}
	for _, s := range l {
		f := fila{SesionEnCurso: s}
		if def := defDe(a, s.MenuID, s.MenuVersion); def != nil {
			f.Menu = def.Nombre
			f.Ruta = rutaDe(a, def, s.ID, s.Paso)
		}
		out = append(out, f)
	}
	return js(out, nil)
}

func defDe(a *almacen.Almacen, menuID int64, version int) *flujo.Def {
	if b := a.VersionDeMenu(menuID, version); b != nil {
		if d, err := flujo.Parse(b); err == nil {
			return d
		}
	}
	return nil
}

func tituloPaso(d *flujo.Def, id string) string {
	p := d.Paso(id)
	if p == nil {
		return id
	}
	t := p.Titulo
	if t == "" {
		t = p.Pregunta
	}
	if t == "" {
		t = p.Texto
	}
	t = strings.Join(strings.Fields(t), " ")
	if r := []rune(t); len(r) > 24 {
		t = string(r[:24]) + "…"
	}
	if t == "" {
		t = id
	}
	return t
}

// rutaDe arma "Ventas › Precios": los pasos de menú por los que pasó, hasta el actual.
func rutaDe(a *almacen.Almacen, d *flujo.Def, sesionID int64, actual string) string {
	ev, _ := a.Recorrido(sesionID)
	var partes []string
	vistos := map[string]bool{}
	for _, e := range ev {
		if e.Resultado == "mostrado" && !vistos[e.Paso] {
			if p := d.Paso(e.Paso); p != nil && (p.Tipo == flujo.PasoMenu || p.Tipo == flujo.PasoPedir) {
				partes = append(partes, tituloPaso(d, e.Paso))
				vistos[e.Paso] = true
			}
		}
	}
	if len(partes) == 0 && actual != "" {
		partes = append(partes, tituloPaso(d, actual))
	}
	return d.Nombre + " › " + strings.Join(partes, " › ")
}

// RecorridoSesion devuelve (JSON) lo que hizo una persona dentro del menú: paso por paso, sin lo que escribió.
func RecorridoSesion(botID, sesionID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	s, err := a.SesionPorID(botID, sesionID)
	if err != nil || s == nil {
		return "", errors.New("Esa conversación ya no existe.")
	}
	d := defDe(a, s.MenuID, s.MenuVersion)
	type paso struct {
		TS        int64  `json:"ts"`
		Titulo    string `json:"titulo"`
		Opcion    string `json:"opcion"`
		Resultado string `json:"resultado"`
	}
	ev, _ := a.Recorrido(sesionID)
	pasos := []paso{}
	for _, e := range ev {
		t := e.Paso
		if d != nil {
			t = tituloPaso(d, e.Paso)
		}
		pasos = append(pasos, paso{e.TS, t, e.Opcion, e.Resultado})
	}
	nombre := ""
	if d != nil {
		nombre = d.Nombre
	}
	return js(map[string]any{"sesion": s, "menu": nombre, "pasos": pasos, "persona": a.NombreConocido(botID, s.Chat, s.Sender)}, nil)
}

// AccionSesion cambia el estado de una conversación: "reiniciar" (la próxima vez que escriba empieza de nuevo), "humano" (el bot calla
// con esa persona 12 h), "pausar" (calla 2 h) o "reactivar". NO manda ningún mensaje: el bot solo responde.
func AccionSesion(botID, sesionID int64, accion string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	s, err := a.SesionPorID(botID, sesionID)
	if err != nil || s == nil {
		return errors.New("Esa conversación ya no existe.")
	}
	ahora := time.Now()
	switch accion {
	case "reiniciar", "reactivar":
		return a.CerrarSesion(botID, s, almacen.SesAbandonada, 0, ahora)
	case "humano":
		return a.SilenciarSesion(botID, s.Chat, s.Sender, s.MenuID, s.MenuVersion, almacen.SesHumano, ahora.Add(12*time.Hour), ahora)
	case "pausar":
		return a.SilenciarSesion(botID, s.Chat, s.Sender, s.MenuID, s.MenuVersion, almacen.SesPausada, ahora.Add(12*time.Hour), ahora)
	}
	return errors.New("acción desconocida")
}

// EstadisticasAutomatizacion devuelve (JSON) los números de un menú entre dos días (AAAA-MM-DD).
func EstadisticasAutomatizacion(botID, id int64, desde, hasta string, incluirPrueba bool) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	filas, err := a.EstadisticasMenu(botID, id, desde, hasta, incluirPrueba)
	if err != nil {
		return "", err
	}
	m, _ := a.ObtenerMenu(botID, id)
	var d *flujo.Def
	if m != nil {
		d, _ = flujo.Parse(m.Def)
	}
	type paso struct {
		Paso   string `json:"paso"`
		Titulo string `json:"titulo"`
		Valor  int64  `json:"valor"`
		Opcion string `json:"opcion"`
		Texto  string `json:"texto"`
	}
	res := map[string]int64{}
	var embudo, invalidas, opciones, abandonos []paso
	var formEnviados, formCancelados int64
	for _, f := range filas {
		if f.Paso == "" {
			res[f.Opcion] += f.Cuenta
			continue
		}
		t, txt := f.Paso, ""
		if d != nil {
			t = tituloPaso(d, f.Paso)
			if p := d.Paso(f.Paso); p != nil && f.Opcion != "" {
				if n := atoi(f.Opcion); n >= 1 && n <= len(p.Opciones) {
					txt = p.Opciones[n-1].Texto
				}
			}
		}
		switch f.Opcion {
		case "visita":
			embudo = append(embudo, paso{f.Paso, t, f.Cuenta, "", ""})
		case "invalida":
			invalidas = append(invalidas, paso{f.Paso, t, f.Cuenta, "", ""})
		case "abandono":
			abandonos = append(abandonos, paso{f.Paso, t, f.Cuenta, "", ""})
		case "enviado":
			formEnviados += f.Cuenta
		case "cancelado":
			formCancelados += f.Cuenta
		default:
			opciones = append(opciones, paso{f.Paso, t, f.Cuenta, f.Opcion, txt})
		}
	}
	// el embudo sigue el orden de los pasos de la definición
	if d != nil {
		orden := map[string]int{}
		for i, p := range d.Pasos {
			orden[p.ID] = i
		}
		for i := 0; i < len(embudo); i++ {
			for j := i + 1; j < len(embudo); j++ {
				if orden[embudo[j].Paso] < orden[embudo[i].Paso] {
					embudo[i], embudo[j] = embudo[j], embudo[i]
				}
			}
		}
	}
	return js(map[string]any{
		"iniciados": res["iniciado"], "completados": res["completado"], "abandonados": res["abandonado"] + res["expirado"], "humano": res["humano"], "expirados": res["expirado"],
		"opciones": nonNil(opciones), "embudo": nonNil(embudo), "invalidas": nonNil(invalidas),
		"abandonos": nonNil(abandonos), "formulariosEnviados": formEnviados, "formulariosCancelados": formCancelados,
		"noEntendio": a.ContarNoEntendio(botID, id), "formularios": a.ContarFormularios(botID, id),
	}, nil)
}

func nonNil[T any](l []T) []T {
	if l == nil {
		return []T{}
	}
	return l
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// EtiquetaDeChat devuelve (JSON) en qué menú está cada persona de un chat: [{nombre, ruta, ultima}].
func EtiquetaDeChat(botID int64, chat string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	l, err := a.SesionesDeChat(botID, chat, time.Now(), expiraFlujo())
	if err != nil {
		return "", err
	}
	type fila struct {
		Nombre string `json:"nombre"`
		Ruta   string `json:"ruta"`
		Ultima int64  `json:"ultima"`
		Estado string `json:"estado"`
	}
	out := []fila{}
	for _, s := range l {
		f := fila{Nombre: s.Nombre, Ultima: s.UltimaTS, Estado: s.Estado}
		if d := defDe(a, s.MenuID, s.MenuVersion); d != nil {
			f.Ruta = rutaDe(a, d, s.ID, s.Paso)
		}
		out = append(out, f)
	}
	return js(out, nil)
}

// ObtenerAviso devuelve (JSON) una notificación local pendiente con su detalle completo.
func ObtenerAviso(botID, id int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	x, err := a.ObtenerAviso(botID, id)
	if err != nil || x == nil {
		return "", errors.New("aviso no encontrado")
	}
	return js(x, nil)
}
