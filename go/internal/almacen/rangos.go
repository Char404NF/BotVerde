package almacen

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"botchat/internal/cifrado"
)

// Rangos y permisos (v0.5.0, bloque A). Aquí solo vive el almacenamiento: las reglas (jerarquía, alcance, límites) y el
// catálogo de casillas están en internal/permisos. Nada de lo que se guarda aquí lleva el texto de un mensaje.

// PersonaDueno es la clave de la persona dueña de la cuenta (yo). Nunca se asigna a otra persona.
const PersonaDueno = QuienYo

// Alcance limita una casilla: "" (donde la función aplique), "privado", "grupos" o "chats" (solo esos chats).
type Alcance struct {
	Modo  string   `json:"modo,omitempty"`
	Chats []string `json:"chats,omitempty"`
}

// Rol es un rango. El Dueño (EsDueno) es fijo: lo puede todo, no se edita ni se borra.
type Rol struct {
	ID        int64              `json:"id"`
	Nombre    string             `json:"nombre"`
	Color     string             `json:"color"`
	Posicion  int                `json:"posicion"` // 0 = Dueño; menor número = más arriba
	Plantilla string             `json:"plantilla,omitempty"`
	EsDueno   bool               `json:"esDueno"`
	Casillas  map[string]Alcance `json:"casillas"`
	MenusModo string             `json:"menusModo"` // "todos", "lista" o "ninguno"
	Menus     []string           `json:"menus"`     // ids de menú ("12") u opción ("12:op3") visibles cuando el modo es "lista"
	LimiteMin int                `json:"limiteMin"` // comandos por minuto (0 = sin límite)
	Personas  int                `json:"personas"`  // cuántas personas lo tienen ahora (solo al listar)
}

// Asignacion es el rango de una persona.
type Asignacion struct {
	Persona     string `json:"persona"`
	RolID       int64  `json:"rolId"`
	CaducaEn    int64  `json:"caducaEn"` // 0 = permanente
	AsignadoPor string `json:"asignadoPor"`
	AsignadoTS  int64  `json:"asignadoTs"`
	Nota        string `json:"nota,omitempty"`
}

// Estados de una solicitud.
const (
	SolPendiente = "pendiente"
	SolAprobada  = "aprobada"
	SolBloqueada = "bloqueada"
	SolIgnorada  = "ignorada"
)

// Solicitud es un desconocido que escribió y espera una decisión.
type Solicitud struct {
	ID         int64  `json:"id"`
	Persona    string `json:"persona"`
	Nombre     string `json:"nombre"`
	Estado     string `json:"estado"`
	PrimeraTS  int64  `json:"primeraTs"`
	UltimaTS   int64  `json:"ultimaTs"`
	Mensajes   int    `json:"mensajes"`
	AvisadaTS  int64  `json:"avisadaTs"`
	DecididaTS int64  `json:"decididaTs"`
}

// Confirmacion es un código corto de un solo uso ligado a una acción y a una persona.
type Confirmacion struct {
	Codigo   string   `json:"codigo"`
	Persona  string   `json:"persona"`
	Comando  string   `json:"comando"`
	Args     []string `json:"args"`
	Chat     string   `json:"chat"`
	CitaID   string   `json:"citaId"`
	CreadoTS int64    `json:"creadoTs"`
	ExpiraTS int64    `json:"expiraTs"`
	Estado   string   `json:"estado"`
}

// PoliticaGrupo dice a quién le contesta el bot en un grupo: "todos", "admins" o "lista".
type PoliticaGrupo struct {
	Chat  string   `json:"chat"`
	Modo  string   `json:"modo"`
	Lista []string `json:"lista"`
}

// Accion es un renglón del registro de acciones (quién, qué, cuándo, resultado; sin textos).
type Accion struct {
	ID        int64  `json:"id"`
	TS        int64  `json:"ts"`
	Quien     string `json:"quien"`
	Accion    string `json:"accion"`
	Objetivo  string `json:"objetivo"`
	Chat      string `json:"chat"`
	Resultado string `json:"resultado"`
	Detalle   string `json:"detalle"`
}

// ---------- ajustes del bot ----------

// Claves de ajustes del bot y sus valores por defecto.
var ajustesBotDefecto = map[string]string{
	"prefijo":            "!",
	"desconocidos":       "menu", // menu | pronto | aprobar | ignorar
	"desconocidos_texto": "Gracias por escribir. Te contestamos pronto.",
	"desconocidos_desde": "0",
	"tope_pendientes":    "50",
	"ignorada_dias":      "7",
	"confirm_seg":        "120",
	"limite_sin_rango":   "5",
	"usuario_auto":       "1", // dar el rango Usuario a todos los que escriben o están en grupos (menos administradores)
	"usuario_rol":        "0", // id del rango Usuario
	"usuario_creado":     "0", // ya se creó una vez (si el dueño lo borra, no se vuelve a crear solo)
	"grupo_modo_defecto": "todos",
	"registro_dias":      "90",
	// Multimedia (v0.6.0, E2): tope total en bytes (0 = sin límite; de fábrica 5 GB), conservar la portada de un video borrado (1/0)
	"tope_total_bytes":  "5368709120",
	"conservar_portada": "1",
	// Personas y avisos (bloque B)
	"adv_tope":         "3",      // advertencias para que actúe la regla (0 = nunca)
	"adv_accion":       "avisar", // avisar | silenciar | borrar | expulsar
	"adv_silencio_min": "1440",   // cuánto dura el silencio de la regla "silenciar"
	"reportar_gracias": "1",      // contestar "Gracias, ya avisé" a !reportar (encendido: quien reporta debe saber que llegó)
	"reportar_texto":   "Gracias por tu reporte, lo analizaremos. Gracias.",
	"alertas_activas":  "1",     // alertas por palabra clave
	"alerta_fragmento": "0",     // incluir un fragmento del mensaje en la alerta (apagado)
	"resumen_activo":   "0",     // resumen diario
	"resumen_hora":     "20:00", // HH:MM de la zona del teléfono
	"resumen_ultimo":   "",      // último día (AAAA-MM-DD) en que se avisó
	"avisos_exactos":   "1",     // pedir alarmas exactas cuando el sistema lo permita
	// Utilidades (bloque F)
	"util_tope":         "50", // máximo de notas y de pendientes por persona (cada lista)
	"sticker_tope_hora": "20", // stickers que el bot hace por hora en total (0 = sin tope)
	// Menús ampliados (bloque C). Cada menú puede traer los suyos; estos son los generales.
	"menu_err_max":       "3", // errores seguidos para que el menú se detenga y reinicie la conversación
	"menu_txt_n1":        "No entendí esa opción. Estas son las opciones:",
	"menu_txt_n2":        "Sigo sin entender. Puedes escribir *persona* para hablar con alguien, o elegir una opción:",
	"menu_txt_n3":        "Parece que no nos estamos entendiendo, así que dejo la conversación aquí. Cuando quieras empezar de nuevo, escribe *menú*.",
	"menu_txt_notexto":   "Por ahora solo entiendo mensajes escritos. Escríbeme el número de la opción, por favor.",
	"menu_txt_ambiguo":   "Encontré varias opciones parecidas. ¿Cuál quieres? Escribe el número:",
	"menu_txt_salir":     "Listo. Cuando necesites algo, escribe *menú*.",
	"menu_txt_cancelar":  "Cancelado. Cuando quieras, escribe *menú*.",
	"menu_saludos":       "hola, buenas, buenos dias, buenas tardes, buenas noches, gracias, muchas gracias, ok, vale, listo, enterado, de acuerdo, perfecto, saludos, que tal, hey, buen dia",
	"menu_expira_min":    "120",       // privado: minutos sin actividad para que caduque en silencio
	"menu_volver":        "reiniciar", // al volver a escribir tras caducar: reiniciar | retomar
	"menu_retoma_horas":  "24",
	"menu_humano_dentro": "Gracias. Una persona te atenderá en cuanto pueda.",
	"menu_humano_fuera":  "Ahora estamos fuera de horario ({horario}). Dejé tu aviso y te responderemos en cuanto abramos.",
	"menu_horario":       "",   // horario de atención general (JSON); vacío = siempre
	"menu_region":        "MX", // país para validar teléfonos de los formularios cuando no traen +
	"noentendio_activo":  "1",
	"noentendio_dias":    "30", // cuánto se guarda lo que no entendió (0 = no se borra solo)
	// Menús en grupos
	"grupo_max_sesiones": "5",  // personas usando un menú a la vez en un grupo
	"grupo_expira_min":   "10", // caducidad en silencio dentro de un grupo
	"grupo_msgs_min":     "8",  // mensajes del bot por minuto en un mismo grupo
	"grupo_cualquiera":   "0",  // cualquiera puede contestar el menú de otra persona
	"grupo_cola_min":     "10", // cuánto espera alguien en la fila antes de descartarse
	"grupo_txt_turno":    "Hay varias personas usando el menú. Estás en el turno {n}; te atiendo en cuanto se libere un lugar.",
	"grupo_txt_vencido":  "Esa conversación terminó. Escribe {prefijo}menu para empezar otra.",
	"grupo_txt_privado":  "Esa opción es solo en privado: escríbeme al privado.",
	// Mensajes automáticos (bloque D)
	"auto_activo":    "1",      // interruptor general de bienvenida, ausencia, recibido y cierre (los atajos tienen el suyo)
	"auto_tope_hora": "30",     // mensajes automáticos por hora en total; pasado el tope solo sale el texto, sin archivos
	"auto_zona":      "",       // zona horaria por defecto de los mensajes automáticos ("" = la del teléfono)
	"atajos_activo":  "1",      // escribir "/atajo" en un chat (desde el teléfono o la app) manda la respuesta rápida
	"atajo_modo":     "editar", // desde el teléfono: editar (cambia mi "/atajo" por el texto) | enviar (manda aparte y deja mi "/atajo")
	// Grupos (bloque E)
	"todos_tope":       "100", // máximo de personas que menciona "!todos" en total
	"todos_lote":       "30",  // menciones por mensaje (si hay más, salen varios mensajes)
	"todos_espera_seg": "300", // espera mínima entre dos "!todos" en el mismo grupo
	"grupo_dias":       "30",  // periodo por defecto del ranking y de los inactivos
	// Laboratorio (bloque G). Todo apagado: el menú sigue en texto numerado mientras no se active y se acepte el riesgo.
	"lab_activo":    "0",     // interruptor del Laboratorio
	"lab_aviso_ts":  "0",     // cuándo se aceptó el aviso de riesgo (0 = nunca)
	"lab_modo_menu": "texto", // texto | encuesta | botones (experimental)
	// Descargas de video: versión de yt-dlp en uso, la última publicada y cuándo se revisó (la app no los muestra como ajustes).
	"video_ytdlp_ver":    "",
	"video_ytdlp_ultima": "",
	"video_ytdlp_rev_ts": "0",
	"video_upd_dia":      "",
	"lab_grupos":         "0",    // permitir encuestas y botones en grupos (todos ven los votos)
	"lab_enc_max_opc":    "12",   // opciones máximas de una encuesta; más que eso el menú sale en texto
	"lab_expira_min":     "120",  // una encuesta o botón sin respuesta deja de valer a los N minutos
	"lab_respaldo_enc":   "0",    // mandar también el texto numerado debajo de la encuesta
	"lab_respaldo_bot":   "1",    // mandar también el texto numerado debajo de los botones
	"lab_botones_tipo":   "auto", // auto | botones | lista | interactivo
	"lab_prueba_chat":    "",     // mi otro número o grupo de prueba (clave del chat)
	"lab_prueba_hora":    "4",    // pruebas por hora
	"lab_prueba_dia":     "12",   // pruebas por día
	"lab_biz_activo":     "0",    // interruptor propio de la prueba experimental "Interactivo + biz" (apagado)
	"lab_biz_ack":        "0",    // casilla "Entiendo que puede bloquear mi número" de esa prueba
	"lab_version_wa":     "",     // versión de WhatsApp del teléfono que recibe las pruebas
	// Auditoría (v0.6.0, etapa E1)
	"audit_defecto":       "1",  // guardar el texto de los mensajes (interruptor global; cada chat puede tener el suyo)
	"audit_ret_dias":      "90", // días que se conserva el texto (30, 90 o 0 = siempre); instalación nueva = 90
	"audit_ret_pendiente": "0",  // 1 = vienes de una versión anterior y todavía no elegiste: no se purga nada
	"audit_aviso_visto":   "0",  // 1 = ya viste el aviso de auditoría
	"tope_confirmado":     "1",  // 0 = actualizaste con más archivos que el tope y aún no lo viste: el tope NO borra nada hasta confirmarlo
	"tope_conf_init":      "0",  // 1 = ya se decidió (una sola vez) si el tope arranca pendiente o confirmado
	"util_notas":          "1",  // notas y pendientes por comando (el asistente «Crea tu bot» puede apagarlos)
	"util_recordatorios":  "1",  // !recordar (idem)
	"asistente_visto":     "0",  // 1 = ya se ofreció el asistente «Crea tu bot» (instalación nueva: se ofrece una vez)
	"sha_relleno_tope":    "0",  // relleno de la huella de archivos viejos: hasta qué fila
	"sha_relleno_cursor":  "0",  // ... y por cuál va
	"sha_relleno_fin":     "0",  // ... 1 = terminó
	// Silenciar en grupos (v0.6.0, etapa E5). Los avisos privados al silenciado tienen topes PROPIOS (no gastan los de iniciativa).
	"sil_activo":             "1",         // interruptor: borrar lo que escribe quien está silenciado
	"sil_avisos_hora":        "12",        // avisos privados por hora (todos los silenciados juntos)
	"sil_avisos_dia":         "60",        // avisos privados por día lógico (todos juntos)
	"sil_avisos_persona_dia": "4",         // avisos privados por persona y día lógico
	"sil_borrados_min":       "20",        // mensajes ajenos que el silencio borra por minuto
	"sil_aviso_cada":         "3",         // cada cuántos mensajes borrados se le avisa a la persona (0 = nunca)
	"sil_tope":               "10",        // mensajes borrados en el día para aplicar la sanción (0 = sin sanción)
	"sil_sancion":            "confirmar", // confirmar (avisarme y expulsar con un toque) | avisar (solo avisarme) | silencio_largo | expulsar
	"sil_expulsion_auto":     "0",         // 1 = con sanción «expulsar» sí expulsa sola (de fábrica NUNCA)
	"sil_largo_min":          "10080",     // cuánto dura el «silencio más largo» de la sanción (una semana)
	"sil_conteo_por_grupo":   "0",         // 1 = el tope cuenta por grupo; 0 = suma todos los grupos
	"sil_reinicio_hora":      "00:00",     // hora a la que empieza el día lógico del contador (zona del teléfono)
	"sil_avisar_inicio":      "1",         // avisarle a la persona cuando la silencio
	"sil_avisar_fin":         "1",         // avisarle a la persona (y a mí) cuando termina su silencio
	"sil_aviso_noadmin":      "1",         // avisarme (máx. 1 por día y grupo) cuando no soy admin donde alguien está silenciado
	// Advertencias con sanciones (v0.6.0, etapa E5)
	"adv_caducidad_dias":  "30",                              // días que vale una advertencia nueva (0 = no caduca); las que ya existían no caducan
	"alerta_wa":           "0",                               // 1 = los avisos importantes también me llegan a mi chat de WhatsApp («Mensajes guardados»)
	"alerta_wa_tipos":     "reporte,grupos,conexion,resumen", // qué avisos se repiten por WhatsApp
	"aviso_desconexion":   "10",                              // minutos sin conexión antes de avisarme (0 = no avisar)
	"resumen_sem_ultimo":  "",
	"resumen_semanal":     "0", // 1 = cada domingo en la noche me llega el resumen de la semana
	"grupo_base_auto":     "0", // 1 = al entrar a un grupo nuevo se le aplica la configuración base
	"adv_tipos_sembrados": "0", // 1 = ya se crearon los tipos de fábrica (Spam, Insultos, Enlaces, Otro)
}

// AjusteBot devuelve un ajuste del bot (su valor por defecto si nunca se fijó).
func (a *Almacen) AjusteBot(botID int64, clave string) string {
	var v string
	if err := a.db.QueryRow(`SELECT valor FROM bot_ajustes WHERE bot_id=? AND clave=?`, botID, clave).Scan(&v); err == nil {
		return v
	}
	return ajustesBotDefecto[clave]
}

// AjusteBotInt es AjusteBot como número (el valor por defecto si no se puede leer).
func (a *Almacen) AjusteBotInt(botID int64, clave string) int {
	n, err := strconv.Atoi(strings.TrimSpace(a.AjusteBot(botID, clave)))
	if err != nil {
		n, _ = strconv.Atoi(ajustesBotDefecto[clave])
	}
	return n
}

// FijarAjusteBot guarda un ajuste del bot. Solo acepta las claves conocidas.
func (a *Almacen) FijarAjusteBot(botID int64, clave, valor string) error {
	if _, ok := ajustesBotDefecto[clave]; !ok {
		return fmt.Errorf("ajuste desconocido")
	}
	_, err := a.db.Exec(`INSERT INTO bot_ajustes(bot_id, clave, valor) VALUES(?,?,?) ON CONFLICT(bot_id,clave) DO UPDATE SET valor=excluded.valor`, botID, clave, valor)
	if err == nil {
		a.avisa("permisos_ajustes")
	}
	return err
}

// AjustesBot devuelve todos los ajustes del bot (con sus valores por defecto).
func (a *Almacen) AjustesBot(botID int64) map[string]string {
	out := map[string]string{}
	for k, v := range ajustesBotDefecto {
		out[k] = v
	}
	filas, err := a.db.Query(`SELECT clave, valor FROM bot_ajustes WHERE bot_id=?`, botID)
	if err != nil {
		return out
	}
	defer filas.Close()
	for filas.Next() {
		var k, v string
		if filas.Scan(&k, &v) == nil {
			if _, ok := ajustesBotDefecto[k]; ok {
				out[k] = v
			}
		}
	}
	return out
}

// ---------- roles ----------

const colsRol = "id,nombre,color,posicion,plantilla,es_dueno,casillas,menus_modo,menus,limite_min"

type escaneable interface{ Scan(dest ...any) error }

func escanearRol(r escaneable) (*Rol, error) {
	var x Rol
	var dueno int
	var cas, men string
	if err := r.Scan(&x.ID, &x.Nombre, &x.Color, &x.Posicion, &x.Plantilla, &dueno, &cas, &x.MenusModo, &men, &x.LimiteMin); err != nil {
		return nil, err
	}
	x.EsDueno = dueno == 1
	x.Casillas = map[string]Alcance{}
	_ = json.Unmarshal([]byte(cas), &x.Casillas)
	if x.Casillas == nil {
		x.Casillas = map[string]Alcance{}
	}
	x.Menus = []string{}
	_ = json.Unmarshal([]byte(men), &x.Menus)
	if x.Menus == nil {
		x.Menus = []string{}
	}
	return &x, nil
}

// SembrarRoles crea el Dueño (una sola vez) y, la primera vez, las plantillas "Ayudante" y "Cliente VIP" (que después se pueden
// editar o borrar sin que vuelvan a aparecer). Se puede llamar las veces que sea.
func (a *Almacen) SembrarRoles(botID int64, plantillas []Rol) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM roles WHERE bot_id=? AND es_dueno=1`, botID).Scan(&n)
	ahora := time.Now().Unix()
	if n == 0 {
		if _, err := a.db.Exec(`INSERT INTO roles(bot_id,nombre,color,posicion,plantilla,es_dueno,casillas,menus_modo,menus,limite_min,creado_ts,actualizado_ts)
			VALUES(?,?,?,?,?,1,'{}','todos','[]',0,?,?)`, botID, "Dueño", "#F59E28", 0, "", ahora, ahora); err != nil {
			return err
		}
	}
	if a.ajusteDirecto(botID, "plantillas_sembradas") == "1" {
		return nil
	}
	for _, p := range plantillas {
		if _, err := a.crearRolLocked(botID, p); err != nil && !errors.Is(err, ErrNombreRepetido) {
			return err
		}
	}
	_, err := a.db.Exec(`INSERT INTO bot_ajustes(bot_id,clave,valor) VALUES(?,?,?) ON CONFLICT(bot_id,clave) DO UPDATE SET valor=excluded.valor`, botID, "plantillas_sembradas", "1")
	a.avisa("rangos")
	return err
}

func (a *Almacen) ajusteDirecto(botID int64, clave string) string {
	var v string
	_ = a.db.QueryRow(`SELECT valor FROM bot_ajustes WHERE bot_id=? AND clave=?`, botID, clave).Scan(&v)
	return v
}

// Errores de los rangos.
var (
	ErrNombreRepetido = errors.New("ya hay un rango con ese nombre")
	ErrRolDueno       = errors.New("el rango Dueño no se puede cambiar")
	ErrNoExiste       = errors.New("no existe")
)

// ListarRoles devuelve los rangos de arriba (Dueño) hacia abajo, con cuántas personas tiene cada uno.
func (a *Almacen) ListarRoles(botID int64) ([]Rol, error) {
	filas, err := a.db.Query(`SELECT `+colsRol+`, (SELECT COUNT(*) FROM persona_rol p WHERE p.rol_id=roles.id AND (p.caduca_en=0 OR p.caduca_en>?)) FROM roles WHERE bot_id=? ORDER BY posicion, id`, time.Now().Unix(), botID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []Rol{}
	for filas.Next() {
		var x Rol
		var dueno int
		var cas, men string
		if err := filas.Scan(&x.ID, &x.Nombre, &x.Color, &x.Posicion, &x.Plantilla, &dueno, &cas, &x.MenusModo, &men, &x.LimiteMin, &x.Personas); err != nil {
			return nil, err
		}
		x.EsDueno = dueno == 1
		x.Casillas = map[string]Alcance{}
		_ = json.Unmarshal([]byte(cas), &x.Casillas)
		x.Menus = []string{}
		_ = json.Unmarshal([]byte(men), &x.Menus)
		if x.Casillas == nil {
			x.Casillas = map[string]Alcance{}
		}
		if x.Menus == nil {
			x.Menus = []string{}
		}
		out = append(out, x)
	}
	return out, filas.Err()
}

// RolPorID devuelve un rango, o nil si no existe.
func (a *Almacen) RolPorID(botID, id int64) (*Rol, error) {
	r, err := escanearRol(a.db.QueryRow(`SELECT `+colsRol+` FROM roles WHERE bot_id=? AND id=?`, botID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return r, err
}

// RolPorNombre busca un rango por nombre (sin importar mayúsculas ni acentos).
func (a *Almacen) RolPorNombre(botID int64, nombre string) (*Rol, error) {
	roles, err := a.ListarRoles(botID)
	if err != nil {
		return nil, err
	}
	n := normNombre(nombre)
	for i := range roles {
		if normNombre(roles[i].Nombre) == n {
			r := roles[i]
			return &r, nil
		}
	}
	return nil, nil
}

func normNombre(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	rep := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n")
	return rep.Replace(s)
}

func jsonRol(r Rol) (cas, men string) {
	if r.Casillas == nil {
		r.Casillas = map[string]Alcance{}
	}
	if r.Menus == nil {
		r.Menus = []string{}
	}
	c, _ := json.Marshal(r.Casillas)
	m, _ := json.Marshal(r.Menus)
	return string(c), string(m)
}

func (a *Almacen) crearRolLocked(botID int64, r Rol) (int64, error) {
	r.Nombre = strings.TrimSpace(r.Nombre)
	if r.Nombre == "" {
		return 0, errors.New("falta el nombre")
	}
	var existe int
	rows, err := a.db.Query(`SELECT nombre FROM roles WHERE bot_id=?`, botID)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var n string
		if rows.Scan(&n) == nil && normNombre(n) == normNombre(r.Nombre) {
			existe++
		}
	}
	rows.Close()
	if existe > 0 {
		return 0, ErrNombreRepetido
	}
	var pos int
	_ = a.db.QueryRow(`SELECT COALESCE(MAX(posicion),0)+1 FROM roles WHERE bot_id=?`, botID).Scan(&pos)
	if r.Color == "" {
		r.Color = "#2EC4CB"
	}
	if r.MenusModo == "" {
		r.MenusModo = "todos"
	}
	cas, men := jsonRol(r)
	ahora := time.Now().Unix()
	res, err := a.db.Exec(`INSERT INTO roles(bot_id,nombre,color,posicion,plantilla,es_dueno,casillas,menus_modo,menus,limite_min,creado_ts,actualizado_ts)
		VALUES(?,?,?,?,?,0,?,?,?,?,?,?)`, botID, r.Nombre, r.Color, pos, r.Plantilla, cas, r.MenusModo, men, r.LimiteMin, ahora, ahora)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CrearRol agrega un rango al final (el más bajo).
func (a *Almacen) CrearRol(botID int64, r Rol) (int64, error) {
	a.mu.Lock()
	id, err := a.crearRolLocked(botID, r)
	a.mu.Unlock()
	if err == nil {
		a.avisa("rangos")
	}
	return id, err
}

// GuardarRol cambia nombre, color, casillas, menús y límite de un rango (no su posición). El Dueño no se puede cambiar.
func (a *Almacen) GuardarRol(botID int64, r Rol) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	act, err := a.RolPorID(botID, r.ID)
	if err != nil {
		return err
	}
	if act == nil {
		return ErrNoExiste
	}
	if act.EsDueno {
		return ErrRolDueno
	}
	r.Nombre = strings.TrimSpace(r.Nombre)
	if r.Nombre == "" {
		return errors.New("falta el nombre")
	}
	otros, _ := a.ListarRoles(botID)
	for _, o := range otros {
		if o.ID != r.ID && normNombre(o.Nombre) == normNombre(r.Nombre) {
			return ErrNombreRepetido
		}
	}
	if r.MenusModo == "" {
		r.MenusModo = "todos"
	}
	cas, men := jsonRol(r)
	_, err = a.db.Exec(`UPDATE roles SET nombre=?, color=?, casillas=?, menus_modo=?, menus=?, limite_min=?, actualizado_ts=? WHERE bot_id=? AND id=?`,
		r.Nombre, r.Color, cas, r.MenusModo, men, r.LimiteMin, time.Now().Unix(), botID, r.ID)
	if err == nil {
		a.avisa("rangos")
	}
	return err
}

// BorrarRol quita un rango; las personas que lo tenían se quedan sin rango. Devuelve cuántas eran.
func (a *Almacen) BorrarRol(botID, id int64) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	act, err := a.RolPorID(botID, id)
	if err != nil {
		return 0, err
	}
	if act == nil {
		return 0, ErrNoExiste
	}
	if act.EsDueno {
		return 0, ErrRolDueno
	}
	tx, err := a.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var n int
	_ = tx.QueryRow(`SELECT COUNT(*) FROM persona_rol WHERE rol_id=?`, id).Scan(&n)
	if _, err := tx.Exec(`DELETE FROM persona_rol WHERE rol_id=?`, id); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(`DELETE FROM roles WHERE bot_id=? AND id=?`, botID, id); err != nil {
		return 0, err
	}
	if err := renumerar(tx, botID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	a.avisa("rangos")
	return n, nil
}

func renumerar(tx *sql.Tx, botID int64) error {
	filas, err := tx.Query(`SELECT id FROM roles WHERE bot_id=? AND es_dueno=0 ORDER BY posicion, id`, botID)
	if err != nil {
		return err
	}
	var ids []int64
	for filas.Next() {
		var id int64
		if filas.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	filas.Close()
	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE roles SET posicion=? WHERE id=?`, i+1, id); err != nil {
			return err
		}
	}
	return nil
}

// ReordenarRoles fija el orden de los rangos que no son Dueño (de más alto a más bajo). Debe traer todos.
func (a *Almacen) ReordenarRoles(botID int64, ids []int64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	roles, err := a.ListarRoles(botID)
	if err != nil {
		return err
	}
	quedan := map[int64]bool{}
	for _, r := range roles {
		if !r.EsDueno {
			quedan[r.ID] = true
		}
	}
	if len(ids) != len(quedan) {
		return errors.New("el orden debe incluir todos los rangos")
	}
	for _, id := range ids {
		if !quedan[id] {
			return errors.New("el orden trae un rango que no existe (o repetido)")
		}
		delete(quedan, id)
	}
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE roles SET posicion=? WHERE bot_id=? AND id=?`, i+1, botID, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	a.avisa("rangos")
	return nil
}

// ---------- asignaciones ----------

// RolDePersona devuelve el rango vigente de una persona (con la clave canónica ya resuelta). Si el suyo caducó lo quita.
// La persona dueña ("yo") siempre es Dueño. Devuelve (nil, nil) si no tiene rango.
func (a *Almacen) RolDePersona(botID int64, persona string, ahora time.Time) (*Rol, *Asignacion, error) {
	if persona == PersonaDueno {
		var r *Rol
		r, err := escanearRol(a.db.QueryRow(`SELECT `+colsRol+` FROM roles WHERE bot_id=? AND es_dueno=1`, botID))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil, nil
			}
			return nil, nil, err
		}
		return r, &Asignacion{Persona: persona, RolID: r.ID}, nil
	}
	persona = a.ResolverPersona(botID, persona)
	var as Asignacion
	var nota []byte
	err := a.db.QueryRow(`SELECT persona, rol_id, caduca_en, asignado_por, asignado_ts, nota_cifrada FROM persona_rol WHERE bot_id=? AND persona=?`, botID, persona).
		Scan(&as.Persona, &as.RolID, &as.CaducaEn, &as.AsignadoPor, &as.AsignadoTS, &nota)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if as.CaducaEn > 0 && as.CaducaEn <= ahora.Unix() {
		_, _ = a.db.Exec(`DELETE FROM persona_rol WHERE bot_id=? AND persona=?`, botID, persona)
		a.RegistrarAccion(botID, "sistema", "rango.caducar", persona, "", "ok", "")
		a.avisa("rangos")
		return nil, nil, nil
	}
	if len(nota) > 0 {
		if t, err := cifrado.AbrirTexto(a.clave, "persona_nota", nota); err == nil {
			as.Nota = string(t)
		}
	}
	r, err := a.RolPorID(botID, as.RolID)
	if err != nil || r == nil {
		return nil, nil, err
	}
	return r, &as, nil
}

// AsignarRol da (o cambia) el rango de una persona. caducaEn = 0 es permanente. No se puede dar el rango Dueño.
func (a *Almacen) AsignarRol(botID int64, persona string, rolID, caducaEn int64, por string) error {
	persona = a.ResolverPersona(botID, persona)
	if persona == PersonaDueno {
		return ErrRolDueno
	}
	r, err := a.RolPorID(botID, rolID)
	if err != nil {
		return err
	}
	if r == nil {
		return ErrNoExiste
	}
	if r.EsDueno {
		return ErrRolDueno
	}
	_, err = a.db.Exec(`INSERT INTO persona_rol(bot_id,persona,rol_id,caduca_en,asignado_por,asignado_ts) VALUES(?,?,?,?,?,?)
		ON CONFLICT(bot_id,persona) DO UPDATE SET rol_id=excluded.rol_id, caduca_en=excluded.caduca_en, asignado_por=excluded.asignado_por, asignado_ts=excluded.asignado_ts`,
		botID, persona, rolID, caducaEn, por, time.Now().Unix())
	if err == nil {
		a.avisa("rangos")
		a.avisa("rango:" + persona)
	}
	return err
}

// QuitarRol revoca el rango de una persona. Devuelve si tenía uno.
func (a *Almacen) QuitarRol(botID int64, persona string) (bool, error) {
	persona = a.ResolverPersona(botID, persona)
	if persona == PersonaDueno {
		return false, ErrRolDueno
	}
	r, err := a.db.Exec(`DELETE FROM persona_rol WHERE bot_id=? AND persona=?`, botID, persona)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	if n > 0 {
		a.avisa("rangos")
		a.avisa("rango:" + persona)
	}
	return n > 0, nil
}

// ListarAsignaciones devuelve las personas con rango vigente (más recientes primero).
func (a *Almacen) ListarAsignaciones(botID int64) ([]Asignacion, error) {
	filas, err := a.db.Query(`SELECT persona, rol_id, caduca_en, asignado_por, asignado_ts FROM persona_rol WHERE bot_id=? AND (caduca_en=0 OR caduca_en>?) ORDER BY asignado_ts DESC`, botID, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []Asignacion{}
	for filas.Next() {
		var x Asignacion
		if filas.Scan(&x.Persona, &x.RolID, &x.CaducaEn, &x.AsignadoPor, &x.AsignadoTS) == nil {
			out = append(out, x)
		}
	}
	return out, filas.Err()
}

// FijarNotaPersona guarda (cifrada) una nota sobre una persona que ya tiene rango.
func (a *Almacen) FijarNotaPersona(botID int64, persona, nota string) error {
	persona = a.ResolverPersona(botID, persona)
	var sel []byte
	if nota != "" {
		var err error
		if sel, err = cifrado.SellarTexto(a.clave, "persona_nota", []byte(nota)); err != nil {
			return err
		}
	}
	_, err := a.db.Exec(`UPDATE persona_rol SET nota_cifrada=? WHERE bot_id=? AND persona=?`, sel, botID, persona)
	return err
}

// LimpiarCaducados borra los rangos vencidos. Devuelve cuántos.
func (a *Almacen) LimpiarCaducados(botID int64, ahora time.Time) int {
	r, err := a.db.Exec(`DELETE FROM persona_rol WHERE bot_id=? AND caduca_en>0 AND caduca_en<=?`, botID, ahora.Unix())
	if err != nil {
		return 0
	}
	n, _ := r.RowsAffected()
	if n > 0 {
		a.RegistrarAccion(botID, "sistema", "rango.caducar", "", "", "ok", fmt.Sprintf("%d", n))
		a.avisa("rangos")
	}
	return int(n)
}

// ---------- alias de persona (número y LID) ----------

func esPN(j string) bool  { return strings.HasSuffix(j, "@s.whatsapp.net") }
func esLID(j string) bool { return strings.HasSuffix(j, "@lid") }

// ResolverPersona da la clave canónica de una persona: el número (@s.whatsapp.net) si se conoce, si no el LID tal cual.
func (a *Almacen) ResolverPersona(botID int64, p string) string {
	if p == "" || p == PersonaDueno {
		return p
	}
	var c string
	if err := a.db.QueryRow(`SELECT canonica FROM persona_alias WHERE bot_id=? AND alias=?`, botID, p).Scan(&c); err == nil && c != "" {
		return c
	}
	return p
}

// AliasDe devuelve todas las formas (número y LID) conocidas de una persona, incluida la canónica.
func (a *Almacen) AliasDe(botID int64, persona string) []string {
	c := a.ResolverPersona(botID, persona)
	out := []string{c}
	filas, err := a.db.Query(`SELECT alias FROM persona_alias WHERE bot_id=? AND canonica=? AND alias<>?`, botID, c, c)
	if err != nil {
		return out
	}
	defer filas.Close()
	for filas.Next() {
		var s string
		if filas.Scan(&s) == nil {
			out = append(out, s)
		}
	}
	return out
}

// AprenderAlias anota que dos JID son la misma persona (uno es el número y el otro el LID). La clave canónica es el número.
// Si el LID ya tenía su propio rango o solicitud, se unen a los del número (gana el rango más alto).
func (a *Almacen) AprenderAlias(botID int64, x, y string) {
	pn, lid := x, y
	if esLID(x) && esPN(y) {
		pn, lid = y, x
	}
	if !esPN(pn) || !esLID(lid) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	canon := a.ResolverPersona(botID, pn)
	antiguo := a.ResolverPersona(botID, lid)
	var ya string
	if a.db.QueryRow(`SELECT canonica FROM persona_alias WHERE bot_id=? AND alias=?`, botID, lid).Scan(&ya) == nil && ya == canon && antiguo == canon {
		return // ya se sabía
	}
	tx, err := a.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	ts := time.Now().Unix()
	for _, al := range []string{pn, lid} {
		if _, err := tx.Exec(`INSERT INTO persona_alias(bot_id,alias,canonica,ts) VALUES(?,?,?,?) ON CONFLICT(bot_id,alias) DO UPDATE SET canonica=excluded.canonica`, botID, al, canon, ts); err != nil {
			return
		}
	}
	if antiguo != canon {
		if err := fusionarPersona(tx, botID, antiguo, canon); err != nil {
			return
		}
		if _, err := tx.Exec(`UPDATE persona_alias SET canonica=? WHERE bot_id=? AND canonica=?`, canon, botID, antiguo); err != nil {
			return
		}
	}
	if tx.Commit() == nil {
		a.avisa("rangos")
	}
}

func fusionarPersona(tx *sql.Tx, botID int64, desde, hacia string) error {
	// rango: se queda el más alto (posición menor)
	var posD, posH sql.NullInt64
	_ = tx.QueryRow(`SELECT r.posicion FROM persona_rol p JOIN roles r ON r.id=p.rol_id WHERE p.bot_id=? AND p.persona=?`, botID, desde).Scan(&posD)
	_ = tx.QueryRow(`SELECT r.posicion FROM persona_rol p JOIN roles r ON r.id=p.rol_id WHERE p.bot_id=? AND p.persona=?`, botID, hacia).Scan(&posH)
	switch {
	case posD.Valid && !posH.Valid:
		if _, err := tx.Exec(`UPDATE persona_rol SET persona=? WHERE bot_id=? AND persona=?`, hacia, botID, desde); err != nil {
			return err
		}
	case posD.Valid && posH.Valid && posD.Int64 < posH.Int64:
		if _, err := tx.Exec(`DELETE FROM persona_rol WHERE bot_id=? AND persona=?`, botID, hacia); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE persona_rol SET persona=? WHERE bot_id=? AND persona=?`, hacia, botID, desde); err != nil {
			return err
		}
	case posD.Valid:
		if _, err := tx.Exec(`DELETE FROM persona_rol WHERE bot_id=? AND persona=?`, botID, desde); err != nil {
			return err
		}
	}
	// solicitud: bloqueada > aprobada > resto
	peso := func(e string) int {
		switch e {
		case SolBloqueada:
			return 3
		case SolAprobada:
			return 2
		case SolPendiente:
			return 1
		}
		return 0
	}
	var eD, eH string
	dOK := tx.QueryRow(`SELECT estado FROM solicitudes WHERE bot_id=? AND persona=?`, botID, desde).Scan(&eD) == nil
	hOK := tx.QueryRow(`SELECT estado FROM solicitudes WHERE bot_id=? AND persona=?`, botID, hacia).Scan(&eH) == nil
	switch {
	case dOK && !hOK:
		if _, err := tx.Exec(`UPDATE solicitudes SET persona=? WHERE bot_id=? AND persona=?`, hacia, botID, desde); err != nil {
			return err
		}
	case dOK && peso(eD) > peso(eH):
		if _, err := tx.Exec(`DELETE FROM solicitudes WHERE bot_id=? AND persona=?`, botID, hacia); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE solicitudes SET persona=? WHERE bot_id=? AND persona=?`, hacia, botID, desde); err != nil {
			return err
		}
	case dOK:
		if _, err := tx.Exec(`DELETE FROM solicitudes WHERE bot_id=? AND persona=?`, botID, desde); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE codigos_confirmacion SET persona=? WHERE bot_id=? AND persona=?`, hacia, botID, desde); err != nil {
		return err
	}
	// v0.5.0 C: las conversaciones de menú abiertas en grupos, la fila de espera, los avisos de "conversación terminada" y los
	// formularios pasan a la persona canónica (la conversación es de (grupo, persona canónica)).
	for _, q := range []string{
		`UPDATE flow_sessions SET sender=? WHERE bot_id=? AND sender=? AND estado IN ('activo','humano','pausado') AND NOT EXISTS (SELECT 1 FROM flow_sessions o WHERE o.bot_id=flow_sessions.bot_id AND o.chat=flow_sessions.chat AND o.sender=? AND o.estado IN ('activo','humano','pausado') AND o.prueba=flow_sessions.prueba)`,
	} {
		if _, err := tx.Exec(q, hacia, botID, desde, hacia); err != nil {
			return err
		}
	}
	for _, q := range []string{
		`UPDATE OR IGNORE flow_cola SET persona=? WHERE bot_id=? AND persona=?`,
		`UPDATE OR IGNORE flow_vencido SET persona=? WHERE bot_id=? AND persona=?`,
		`UPDATE msg_bot_sesion SET persona=? WHERE bot_id=? AND persona=?`,
		`UPDATE formularios SET persona=? WHERE bot_id=? AND persona=?`,
	} {
		if _, err := tx.Exec(q, hacia, botID, desde); err != nil {
			return err
		}
	}
	// v0.5.0 B: advertencias y notas pasan a la persona canónica; del silencio se queda el más largo (0 = siempre).
	for _, t := range []string{"advertencias", "notas_persona", "util_items"} {
		if _, err := tx.Exec(`UPDATE `+t+` SET persona=? WHERE bot_id=? AND persona=?`, hacia, botID, desde); err != nil {
			return err
		}
	}
	// v0.6.0 E3: etiquetas de persona y miembros de grupo pasan a la persona canónica.
	for _, q := range []string{
		`INSERT OR IGNORE INTO persona_etiqueta(bot_id,persona,etiqueta_id) SELECT bot_id,?,etiqueta_id FROM persona_etiqueta WHERE bot_id=? AND persona=?`,
		`UPDATE OR IGNORE grupo_miembros SET persona=? WHERE bot_id=? AND persona=?`,
	} {
		if _, err := tx.Exec(q, hacia, botID, desde); err != nil {
			return err
		}
	}
	for _, q := range []string{`DELETE FROM persona_etiqueta WHERE bot_id=? AND persona=?`, `DELETE FROM grupo_miembros WHERE bot_id=? AND persona=?`} {
		if _, err := tx.Exec(q, botID, desde); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE grupo_cambios SET persona=? WHERE bot_id=? AND persona=?`, hacia, botID, desde); err != nil {
		return err
	}
	// v0.6.0 E5: silencios de grupo, contadores y sanciones pendientes pasan a la persona canónica.
	if err := fusionaSilencioDe(tx, botID, desde, hacia); err != nil {
		return err
	}
	var hD, hH sql.NullInt64
	sD := tx.QueryRow(`SELECT hasta_ts FROM persona_silencio WHERE bot_id=? AND persona=?`, botID, desde).Scan(&hD) == nil
	sH := tx.QueryRow(`SELECT hasta_ts FROM persona_silencio WHERE bot_id=? AND persona=?`, botID, hacia).Scan(&hH) == nil
	if sD {
		mas := !sH || hD.Int64 == 0 || (hH.Int64 != 0 && hD.Int64 > hH.Int64)
		if mas {
			if sH {
				if _, err := tx.Exec(`DELETE FROM persona_silencio WHERE bot_id=? AND persona=?`, botID, hacia); err != nil {
					return err
				}
			}
			_, err := tx.Exec(`UPDATE persona_silencio SET persona=? WHERE bot_id=? AND persona=?`, hacia, botID, desde)
			return err
		}
		_, err := tx.Exec(`DELETE FROM persona_silencio WHERE bot_id=? AND persona=?`, botID, desde)
		return err
	}
	return nil
}

// ---------- solicitudes ----------

// ResultadoSolicitud dice qué pasó al registrar a un desconocido.
type ResultadoSolicitud struct {
	Solicitud  *Solicitud
	Nueva      bool // se guardó una solicitud nueva (o se reabrió una ignorada vencida)
	Descartada bool // no se guardó porque se llegó al tope de pendientes
}

func escanearSolicitud(a *Almacen, r escaneable) (*Solicitud, error) {
	var s Solicitud
	var nom []byte
	if err := r.Scan(&s.ID, &s.Persona, &nom, &s.Estado, &s.PrimeraTS, &s.UltimaTS, &s.Mensajes, &s.AvisadaTS, &s.DecididaTS); err != nil {
		return nil, err
	}
	if len(nom) > 0 {
		if t, err := cifrado.AbrirTexto(a.clave, "solicitud_nombre", nom); err == nil {
			s.Nombre = string(t)
		}
	}
	return &s, nil
}

const colsSol = "id,persona,nombre_cifrado,estado,primera_ts,ultima_ts,mensajes,avisada_ts,decidida_ts"

// ObtenerSolicitud devuelve la solicitud de una persona, o nil.
func (a *Almacen) ObtenerSolicitud(botID int64, persona string) (*Solicitud, error) {
	persona = a.ResolverPersona(botID, persona)
	s, err := escanearSolicitud(a, a.db.QueryRow(`SELECT `+colsSol+` FROM solicitudes WHERE bot_id=? AND persona=?`, botID, persona))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return s, err
}

// ContarSolicitudes cuenta las solicitudes en un estado.
func (a *Almacen) ContarSolicitudes(botID int64, estado string) int {
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM solicitudes WHERE bot_id=? AND estado=?`, botID, estado).Scan(&n)
	return n
}

// RegistrarSolicitud anota que un desconocido escribió. Si ya hay una pendiente solo cuenta el mensaje; una ignorada sigue
// ignorada durante `ignoradaDias` y después vuelve a quedar pendiente; al llegar al tope de pendientes los nuevos no se guardan.
func (a *Almacen) RegistrarSolicitud(botID int64, persona, nombre string, tope, ignoradaDias int, ahora time.Time) (ResultadoSolicitud, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	persona = a.ResolverPersona(botID, persona)
	ts := ahora.Unix()
	actual, err := a.ObtenerSolicitud(botID, persona)
	if err != nil {
		return ResultadoSolicitud{}, err
	}
	pend := a.ContarSolicitudes(botID, SolPendiente)
	selNombre := func() []byte {
		if nombre == "" {
			return nil
		}
		b, _ := cifrado.SellarTexto(a.clave, "solicitud_nombre", []byte(nombre))
		return b
	}
	if actual == nil {
		if tope > 0 && pend >= tope {
			return ResultadoSolicitud{Descartada: true}, nil
		}
		r, err := a.db.Exec(`INSERT INTO solicitudes(bot_id,persona,nombre_cifrado,estado,primera_ts,ultima_ts,mensajes) VALUES(?,?,?,?,?,?,1)`, botID, persona, selNombre(), SolPendiente, ts, ts)
		if err != nil {
			return ResultadoSolicitud{}, err
		}
		id, _ := r.LastInsertId()
		a.avisa("solicitudes")
		return ResultadoSolicitud{Solicitud: &Solicitud{ID: id, Persona: persona, Nombre: nombre, Estado: SolPendiente, PrimeraTS: ts, UltimaTS: ts, Mensajes: 1}, Nueva: true}, nil
	}
	switch actual.Estado {
	case SolPendiente:
		if nombre != "" && actual.Nombre != nombre {
			_, _ = a.db.Exec(`UPDATE solicitudes SET nombre_cifrado=? WHERE id=?`, selNombre(), actual.ID)
		}
		_, _ = a.db.Exec(`UPDATE solicitudes SET ultima_ts=?, mensajes=mensajes+1 WHERE id=?`, ts, actual.ID)
		actual.UltimaTS, actual.Mensajes = ts, actual.Mensajes+1
		a.avisa("solicitudes")
		return ResultadoSolicitud{Solicitud: actual}, nil
	case SolIgnorada:
		if ignoradaDias < 1 {
			ignoradaDias = 7
		}
		if ts < actual.DecididaTS+int64(ignoradaDias)*86400 {
			return ResultadoSolicitud{Solicitud: actual}, nil
		}
		if tope > 0 && pend >= tope {
			return ResultadoSolicitud{Solicitud: actual, Descartada: true}, nil
		}
		_, err := a.db.Exec(`UPDATE solicitudes SET estado=?, ultima_ts=?, mensajes=1, avisada_ts=0 WHERE id=?`, SolPendiente, ts, actual.ID)
		if err != nil {
			return ResultadoSolicitud{}, err
		}
		actual.Estado, actual.UltimaTS, actual.Mensajes, actual.AvisadaTS = SolPendiente, ts, 1, 0
		a.avisa("solicitudes")
		return ResultadoSolicitud{Solicitud: actual, Nueva: true}, nil
	}
	return ResultadoSolicitud{Solicitud: actual}, nil
}

// DecidirSolicitud aprueba, bloquea o ignora a una persona (también sirve para desbloquear: estado "aprobada").
// Si la persona no tenía solicitud se crea con ese estado.
func (a *Almacen) DecidirSolicitud(botID int64, persona, estado string, ahora time.Time) error {
	switch estado {
	case SolAprobada, SolBloqueada, SolIgnorada, SolPendiente:
	default:
		return errors.New("estado no válido")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	persona = a.ResolverPersona(botID, persona)
	if persona == PersonaDueno {
		return ErrRolDueno
	}
	ts := ahora.Unix()
	_, err := a.db.Exec(`INSERT INTO solicitudes(bot_id,persona,estado,primera_ts,ultima_ts,mensajes,decidida_ts) VALUES(?,?,?,?,?,0,?)
		ON CONFLICT(bot_id,persona) DO UPDATE SET estado=excluded.estado, decidida_ts=excluded.decidida_ts`, botID, persona, estado, ts, ts, ts)
	if err == nil {
		a.avisa("solicitudes")
	}
	return err
}

// MarcarSolicitudAvisada anota que ya se le dijo "te contestamos pronto".
func (a *Almacen) MarcarSolicitudAvisada(botID int64, persona string, ts time.Time) {
	persona = a.ResolverPersona(botID, persona)
	_, _ = a.db.Exec(`UPDATE solicitudes SET avisada_ts=? WHERE bot_id=? AND persona=?`, ts.Unix(), botID, persona)
}

// BorrarSolicitud quita a una persona de la bandeja (vuelve a ser desconocida).
func (a *Almacen) BorrarSolicitud(botID int64, persona string) error {
	persona = a.ResolverPersona(botID, persona)
	_, err := a.db.Exec(`DELETE FROM solicitudes WHERE bot_id=? AND persona=?`, botID, persona)
	if err == nil {
		a.avisa("solicitudes")
	}
	return err
}

// ListarSolicitudes devuelve las solicitudes de un estado ("" = pendientes), las más recientes primero.
func (a *Almacen) ListarSolicitudes(botID int64, estado string, limite int) ([]Solicitud, error) {
	if estado == "" {
		estado = SolPendiente
	}
	if limite <= 0 {
		limite = 200
	}
	filas, err := a.db.Query(`SELECT `+colsSol+` FROM solicitudes WHERE bot_id=? AND estado=? ORDER BY ultima_ts DESC LIMIT ?`, botID, estado, limite)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []Solicitud{}
	for filas.Next() {
		if s, err := escanearSolicitud(a, filas); err == nil {
			out = append(out, *s)
		}
	}
	return out, filas.Err()
}

// CreadoChat da la hora (segundos) en que se guardó el chat por primera vez (0 si no existe).
func (a *Almacen) CreadoChat(botID int64, chat string) int64 {
	var t int64
	_ = a.db.QueryRow(`SELECT creado_ts FROM chats WHERE bot_id=? AND jid=?`, botID, chat).Scan(&t)
	return t
}

// ---------- política de grupos ----------

// PoliticaDeGrupo devuelve la política del grupo; sin renglón, la general (ajuste grupo_modo_defecto, "todos" por defecto).
func (a *Almacen) PoliticaDeGrupo(botID int64, chat string) PoliticaGrupo {
	p := PoliticaGrupo{Chat: chat, Modo: a.AjusteBot(botID, "grupo_modo_defecto"), Lista: []string{}}
	var lista string
	var modo string
	if a.db.QueryRow(`SELECT modo, lista FROM grupos_politica WHERE bot_id=? AND chat_jid=?`, botID, chat).Scan(&modo, &lista) == nil {
		p.Modo = modo
		_ = json.Unmarshal([]byte(lista), &p.Lista)
	}
	if p.Lista == nil {
		p.Lista = []string{}
	}
	switch p.Modo {
	case "todos", "admins", "lista":
	default:
		p.Modo = "todos"
	}
	return p
}

// FijarPoliticaGrupo guarda a quién le contesta el bot en un grupo.
func (a *Almacen) FijarPoliticaGrupo(botID int64, p PoliticaGrupo) error {
	switch p.Modo {
	case "todos", "admins", "lista":
	default:
		return errors.New("modo no válido")
	}
	if p.Lista == nil {
		p.Lista = []string{}
	}
	l := make([]string, 0, len(p.Lista))
	for _, x := range p.Lista {
		l = append(l, a.ResolverPersona(botID, x))
	}
	b, _ := json.Marshal(l)
	_, err := a.db.Exec(`INSERT INTO grupos_politica(bot_id,chat_jid,modo,lista,actualizado_ts) VALUES(?,?,?,?,?)
		ON CONFLICT(bot_id,chat_jid) DO UPDATE SET modo=excluded.modo, lista=excluded.lista, actualizado_ts=excluded.actualizado_ts`, botID, p.Chat, p.Modo, string(b), time.Now().Unix())
	if err == nil {
		a.avisa("politicas")
	}
	return err
}

// ListarPoliticasGrupo devuelve las políticas guardadas (solo de los grupos donde se cambió algo).
func (a *Almacen) ListarPoliticasGrupo(botID int64) ([]PoliticaGrupo, error) {
	filas, err := a.db.Query(`SELECT chat_jid, modo, lista FROM grupos_politica WHERE bot_id=? ORDER BY actualizado_ts DESC`, botID)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []PoliticaGrupo{}
	for filas.Next() {
		var p PoliticaGrupo
		var l string
		if filas.Scan(&p.Chat, &p.Modo, &l) == nil {
			_ = json.Unmarshal([]byte(l), &p.Lista)
			if p.Lista == nil {
				p.Lista = []string{}
			}
			out = append(out, p)
		}
	}
	return out, filas.Err()
}

// ---------- confirmaciones ----------

const alfabetoCodigo = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // sin 0/O/1/I para no confundir

func nuevoCodigo(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	out := make([]byte, n)
	for i := range b {
		out[i] = alfabetoCodigo[int(b[i])%len(alfabetoCodigo)]
	}
	return string(out)
}

// Errores de las confirmaciones.
var (
	ErrCodigoInvalido = errors.New("código no válido o vencido")
)

// CrearConfirmacion genera un código corto de un solo uso para una acción de una persona.
func (a *Almacen) CrearConfirmacion(botID int64, persona, comando string, args []string, chat, citaID string, vida time.Duration, ahora time.Time) (*Confirmacion, error) {
	persona = a.ResolverPersona(botID, persona)
	if args == nil {
		args = []string{}
	}
	ab, _ := json.Marshal(args)
	sel, err := cifrado.SellarTexto(a.clave, "confirmacion", ab)
	if err != nil {
		return nil, err
	}
	// Una persona tiene a lo más un código pendiente por acción: el nuevo reemplaza al viejo.
	_, _ = a.db.Exec(`UPDATE codigos_confirmacion SET estado='cancelado' WHERE bot_id=? AND persona=? AND comando=? AND estado='pendiente'`, botID, persona, comando)
	for intento := 0; intento < 8; intento++ {
		c := &Confirmacion{Codigo: nuevoCodigo(6), Persona: persona, Comando: comando, Args: args, Chat: chat, CitaID: citaID, CreadoTS: ahora.Unix(), ExpiraTS: ahora.Add(vida).Unix(), Estado: "pendiente"}
		_, err = a.db.Exec(`INSERT INTO codigos_confirmacion(bot_id,codigo,persona,comando,args_cifrados,chat,cita_id,creado_ts,expira_ts,estado) VALUES(?,?,?,?,?,?,?,?,?,'pendiente')`,
			botID, c.Codigo, persona, comando, sel, chat, citaID, c.CreadoTS, c.ExpiraTS)
		if err == nil {
			a.avisa("confirmaciones")
			return c, nil
		}
	}
	return nil, err
}

func (a *Almacen) leerConfirmacion(botID int64, codigo string) (*Confirmacion, error) {
	var c Confirmacion
	var sel []byte
	err := a.db.QueryRow(`SELECT codigo,persona,comando,args_cifrados,chat,cita_id,creado_ts,expira_ts,estado FROM codigos_confirmacion WHERE bot_id=? AND codigo=?`, botID, codigo).
		Scan(&c.Codigo, &c.Persona, &c.Comando, &sel, &c.Chat, &c.CitaID, &c.CreadoTS, &c.ExpiraTS, &c.Estado)
	if err != nil {
		return nil, err
	}
	c.Args = []string{}
	if t, err := cifrado.AbrirTexto(a.clave, "confirmacion", sel); err == nil {
		_ = json.Unmarshal(t, &c.Args)
	}
	return &c, nil
}

// ConsumirConfirmacion valida y gasta un código (solo sirve una vez). Si `persona` no es "", el código tiene que ser de esa
// persona (quien escribe "!confirmar"); la app lo gasta sin persona (el dueño aprueba).
func (a *Almacen) ConsumirConfirmacion(botID int64, codigo, persona string, ahora time.Time) (*Confirmacion, error) {
	codigo = strings.ToUpper(strings.TrimSpace(codigo))
	if codigo == "" {
		return nil, ErrCodigoInvalido
	}
	c, err := a.leerConfirmacion(botID, codigo)
	if err != nil {
		return nil, ErrCodigoInvalido
	}
	if persona != "" && a.ResolverPersona(botID, persona) != c.Persona {
		return nil, ErrCodigoInvalido
	}
	if c.ExpiraTS <= ahora.Unix() || c.Estado != "pendiente" {
		return nil, ErrCodigoInvalido
	}
	r, err := a.db.Exec(`UPDATE codigos_confirmacion SET estado='usado' WHERE bot_id=? AND codigo=? AND estado='pendiente' AND expira_ts>?`, botID, codigo, ahora.Unix())
	if err != nil {
		return nil, err
	}
	if n, _ := r.RowsAffected(); n != 1 {
		return nil, ErrCodigoInvalido
	}
	a.avisa("confirmaciones")
	c.Estado = "usado"
	return c, nil
}

// CancelarConfirmacion descarta un código pendiente.
func (a *Almacen) CancelarConfirmacion(botID int64, codigo string) error {
	_, err := a.db.Exec(`UPDATE codigos_confirmacion SET estado='cancelado' WHERE bot_id=? AND codigo=? AND estado='pendiente'`, botID, strings.ToUpper(strings.TrimSpace(codigo)))
	if err == nil {
		a.avisa("confirmaciones")
	}
	return err
}

// ConfirmacionesPendientes devuelve los códigos vigentes (los más nuevos primero).
func (a *Almacen) ConfirmacionesPendientes(botID int64, ahora time.Time) ([]Confirmacion, error) {
	filas, err := a.db.Query(`SELECT codigo FROM codigos_confirmacion WHERE bot_id=? AND estado='pendiente' AND expira_ts>? ORDER BY creado_ts DESC`, botID, ahora.Unix())
	if err != nil {
		return nil, err
	}
	var cods []string
	for filas.Next() {
		var c string
		if filas.Scan(&c) == nil {
			cods = append(cods, c)
		}
	}
	filas.Close()
	out := []Confirmacion{}
	for _, c := range cods {
		if x, err := a.leerConfirmacion(botID, c); err == nil {
			out = append(out, *x)
		}
	}
	return out, nil
}

// LimpiarConfirmaciones borra los códigos vencidos o ya usados con más de una hora.
func (a *Almacen) LimpiarConfirmaciones(botID int64, ahora time.Time) {
	_, _ = a.db.Exec(`DELETE FROM codigos_confirmacion WHERE bot_id=? AND expira_ts<=?`, botID, ahora.Add(-time.Hour).Unix())
}

// ---------- registro de acciones (sin textos) ----------

// RegistrarAccion anota quién hizo qué y con qué resultado. `detalle` es un código corto (por ejemplo "rol=3"), nunca texto de mensajes.
func (a *Almacen) RegistrarAccion(botID int64, quien, accion, objetivo, chat, resultado, detalle string) {
	_, err := a.db.Exec(`INSERT INTO registro_acciones(bot_id,ts,quien,accion,objetivo,chat,resultado,detalle) VALUES(?,?,?,?,?,?,?,?)`,
		botID, time.Now().Unix(), quien, accion, objetivo, chat, resultado, detalle)
	if err == nil {
		a.avisa("acciones")
	}
}

// ListarAcciones devuelve el registro de acciones, lo más nuevo primero. antesID > 0 pide la página siguiente.
func (a *Almacen) ListarAcciones(botID int64, limite int, antesID int64) ([]Accion, error) {
	if limite <= 0 || limite > 500 {
		limite = 100
	}
	q := `SELECT id,ts,quien,accion,objetivo,chat,resultado,detalle FROM registro_acciones WHERE bot_id=?`
	args := []any{botID}
	if antesID > 0 {
		q += ` AND id<?`
		args = append(args, antesID)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limite)
	filas, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	out := []Accion{}
	for filas.Next() {
		var x Accion
		if filas.Scan(&x.ID, &x.TS, &x.Quien, &x.Accion, &x.Objetivo, &x.Chat, &x.Resultado, &x.Detalle) == nil {
			out = append(out, x)
		}
	}
	return out, filas.Err()
}

// LimpiarAcciones borra los renglones con más de `dias` días.
func (a *Almacen) LimpiarAcciones(botID int64, dias int, ahora time.Time) {
	if dias < 1 {
		dias = 90
	}
	_, _ = a.db.Exec(`DELETE FROM registro_acciones WHERE bot_id=? AND ts<?`, botID, ahora.Add(-time.Duration(dias)*24*time.Hour).Unix())
}

// OrdenarPorPosicion ordena roles de arriba hacia abajo (utilidad para las pruebas y la API).
func OrdenarPorPosicion(r []Rol) {
	sort.SliceStable(r, func(i, j int) bool { return r[i].Posicion < r[j].Posicion })
}
