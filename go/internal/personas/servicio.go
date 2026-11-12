// Package personas reúne lo que se puede hacer con una persona o con un chat desde el bot (bloque B): ignorar al bot para ella (antes «silenciar»),
// bloquear en el bot, advertir (con regla al llegar al tope), notas, ficha, olvidar y exportar; y del chat: pausar, tomar el control,
// Pendiente / Resuelto, etiquetas y "!reportar". Todo va por persona canónica (número con sus alias LID), respeta la jerarquía de
// rangos y deja una línea en el registro de acciones SIN textos. Lo que avisa al dueño es siempre una notificación local de
// Android (tabla avisos): nunca un mensaje de WhatsApp.
package personas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/grupos"
	"botchat/internal/permisos"
	"botchat/internal/silencio"
)

// Servicio es el conjunto de acciones sobre personas y chats de un bot.
type Servicio struct {
	Alm   *almacen.Almacen
	BotID int64
	Perm  *permisos.Servicio
	// Grupo hace lo que solo puede un administrador de grupo (borrar un mensaje ajeno, expulsar). nil = implementación nula.
	Grupo grupos.AccionesGrupo
	// Sil es el silencio de grupos (v0.6.0): los pasos «silenciar» y «expulsar» de la escalera de advertencias lo usan. nil = sin silencio.
	Sil *silencio.Servicio
	// Avisar le manda un aviso privado a la persona (con los topes sil_*). nil = no se avisa a nadie.
	Avisar func(persona, texto string) (bool, string)
	Ahora  func() time.Time
	Log    func(string)
}

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

// Errores que se le explican a quien pidió la acción.
var (
	ErrSinPersona = errors.New("falta indicar a quién")
	ErrYoMismo    = errors.New("no puedes hacer eso contigo mismo")
)

// persona da la clave canónica.
func (s *Servicio) persona(p string) string { return s.Perm.Persona(p) }

// puede revisa la jerarquía: un rango solo actúa sobre rangos más abajo y nadie toca al Dueño.
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
	if !s.Perm.PuedeActuarSobre(actor, objetivo) {
		return permisos.ErrJerarquia
	}
	return nil
}

// registra deja la acción en el registro (sin textos).
func (s *Servicio) registra(actor, accion, objetivo, chat, resultado, detalle string) {
	s.Alm.RegistrarAccion(s.BotID, s.persona(actor), accion, s.persona(objetivo), chat, resultado, detalle)
}

// Nombre es cómo se ve a esa persona (nombre guardado o número).
func (s *Servicio) Nombre(persona string) string {
	for _, f := range s.Alm.AliasDe(s.BotID, persona) {
		if c, _ := s.Alm.ObtenerChat(s.BotID, f); c != nil && !c.EsGrupo {
			if c.Mostrado != "" {
				return c.Mostrado
			}
			if c.TelMostrado != "" {
				return c.TelMostrado
			}
		}
	}
	p := s.persona(persona)
	if i := strings.IndexByte(p, '@'); i > 0 && strings.HasSuffix(p, "@s.whatsapp.net") {
		return "+" + p[:i]
	}
	return "esa persona"
}

// ---------- silencio y bloqueo ----------

// Ignorar hace que el bot no le conteste a esa persona durante `dur` (0 = siempre). Es el «silenciar» de antes de la 0.6.0; ahora
// «silenciar» es borrar sus mensajes en grupos (paquete silencio). El método se sigue llamando Silenciar por compatibilidad.
func (s *Servicio) Ignorar(actor, objetivo string, dur time.Duration, motivo string) error {
	return s.Silenciar(actor, objetivo, dur, motivo)
}

// Designorar quita el «ignorar». Devuelve si lo había.
func (s *Servicio) Designorar(actor, objetivo string) (bool, error) {
	return s.Desilenciar(actor, objetivo)
}

// Silenciar hace que el bot no le conteste a esa persona durante `dur` (0 = siempre).
func (s *Servicio) Silenciar(actor, objetivo string, dur time.Duration, motivo string) error {
	if err := s.puede(actor, objetivo); err != nil {
		s.registra(actor, "persona.silenciar", objetivo, "", "denegado", "")
		return err
	}
	hasta := int64(0)
	if dur > 0 {
		hasta = s.ahora().Add(dur).Unix()
	}
	if err := s.Alm.FijarSilencio(s.BotID, objetivo, hasta, motivo, s.persona(actor), s.ahora()); err != nil {
		s.registra(actor, "persona.silenciar", objetivo, "", "error", "")
		return err
	}
	s.registra(actor, "persona.silenciar", objetivo, "", "ok", fmt.Sprintf("seg=%d", int64(dur.Seconds())))
	return nil
}

// Desilenciar quita el silencio. Devuelve si lo había.
func (s *Servicio) Desilenciar(actor, objetivo string) (bool, error) {
	if err := s.puede(actor, objetivo); err != nil {
		s.registra(actor, "persona.desilenciar", objetivo, "", "denegado", "")
		return false, err
	}
	ok, err := s.Alm.QuitarSilencio(s.BotID, objetivo)
	if err != nil {
		s.registra(actor, "persona.desilenciar", objetivo, "", "error", "")
		return false, err
	}
	s.registra(actor, "persona.desilenciar", objetivo, "", "ok", "")
	return ok, nil
}

// Bloquear hace que el bot ignore por completo a esa persona. NO es el bloqueo de WhatsApp.
func (s *Servicio) Bloquear(actor, objetivo string) error {
	if err := s.puede(actor, objetivo); err != nil {
		s.registra(actor, "persona.bloquear", objetivo, "", "denegado", "")
		return err
	}
	if err := s.Alm.BloquearPersona(s.BotID, objetivo, s.ahora()); err != nil {
		s.registra(actor, "persona.bloquear", objetivo, "", "error", "")
		return err
	}
	s.registra(actor, "persona.bloquear", objetivo, "", "ok", "")
	return nil
}

// Desbloquear quita el bloqueo del bot. Devuelve si estaba bloqueada.
func (s *Servicio) Desbloquear(actor, objetivo string) (bool, error) {
	if err := s.puede(actor, objetivo); err != nil {
		s.registra(actor, "persona.desbloquear", objetivo, "", "denegado", "")
		return false, err
	}
	ok, err := s.Alm.DesbloquearPersona(s.BotID, objetivo)
	if err != nil {
		s.registra(actor, "persona.desbloquear", objetivo, "", "error", "")
		return false, err
	}
	s.registra(actor, "persona.desbloquear", objetivo, "", "ok", "")
	return ok, nil
}

// ---------- advertencias ----------

// Acciones de la regla al llegar al tope de advertencias.
const (
	AccAvisar    = "avisar"
	AccSilenciar = "silenciar"
	AccBorrar    = "borrar"
	AccExpulsar  = "expulsar"
)

// EsAccionTope dice si el valor es una acción válida.
func EsAccionTope(v string) bool {
	switch v {
	case AccAvisar, AccSilenciar, AccBorrar, AccExpulsar:
		return true
	}
	return false
}

// ResultadoAdvertir dice qué pasó al advertir.
type ResultadoAdvertir struct {
	Total       int    `json:"total"`
	Tope        int    `json:"tope"`
	LlegoAlTope bool   `json:"llegoAlTope"`
	Accion      string `json:"accion"` // la acción de la regla (si llegó al tope)
	Hecho       string `json:"hecho"`  // "avisado", "silenciado", "borrado", "expulsado" o "solo_aviso" (no se pudo hacer)
	Motivo      string `json:"motivo"` // por qué no se pudo ("no soy administrador", "no disponible", "sin mensaje")
	// Escalera por tipo (v0.6.0)
	TipoID    int64  `json:"tipoId"`
	ReglaID   int64  `json:"reglaId"`
	Nivel     int    `json:"nivel"`     // cuántas lleva vigentes de ese tipo
	Paso      string `json:"paso"`      // acción del paso de la escalera que tocó ("" = ninguno)
	Regla     string `json:"regla"`     // «Regla N: título» si la advertencia está ligada a una
	SancionID int64  `json:"sancionId"` // expulsión que espera mi confirmación
}

// Advertir pone una advertencia. `chat` es donde pasó (grupo o privado) y `msgID` el mensaje al que se refiere (para la acción
// "borrar"; puede ir vacío). Al llegar al tope hace la acción elegida; borrar y expulsar SOLO si soy administrador del grupo y la
// acción existe: si no, solo alerta al dueño con una notificación local.
func (s *Servicio) Advertir(ctx context.Context, actor, objetivo, chat, msgID, motivo string) (*ResultadoAdvertir, error) {
	if err := s.puede(actor, objetivo); err != nil {
		s.registra(actor, "persona.advertir", objetivo, chat, "denegado", "")
		return nil, err
	}
	_, total, err := s.Alm.AgregarAdvertenciaTipo(s.BotID, objetivo, chat, motivo, s.persona(actor), 0, 0, s.ahora())
	if err != nil {
		s.registra(actor, "persona.advertir", objetivo, chat, "error", "")
		return nil, err
	}
	s.registra(actor, "persona.advertir", objetivo, chat, "ok", fmt.Sprintf("total=%d", total))
	r := &ResultadoAdvertir{Total: total, Tope: s.Alm.AjusteBotInt(s.BotID, "adv_tope"), Accion: s.Alm.AjusteBot(s.BotID, "adv_accion")}
	if r.Tope > 0 && total >= r.Tope {
		r.LlegoAlTope = true
		s.alTope(ctx, objetivo, chat, msgID, r)
	}
	return r, nil
}

func (s *Servicio) alTope(ctx context.Context, objetivo, chat, msgID string, r *ResultadoAdvertir) {
	nombre := s.Nombre(objetivo)
	alerta := func(titulo, detalle string) {
		if _, err := s.Alm.CrearAvisoDe(s.BotID, "advertencias", chat, s.persona(objetivo), titulo, detalle); err == nil {
			s.log("Aviso local: una persona llegó al tope de advertencias.")
		}
	}
	solo := func(motivo string) {
		r.Hecho, r.Motivo = "solo_aviso", motivo
	}
	switch r.Accion {
	case AccSilenciar:
		min := s.Alm.AjusteBotInt(s.BotID, "adv_silencio_min")
		if min < 1 {
			min = 1440
		}
		if err := s.Alm.FijarSilencio(s.BotID, objetivo, s.ahora().Add(time.Duration(min)*time.Minute).Unix(), "tope de advertencias", "regla", s.ahora()); err != nil {
			solo("no se pudo silenciar")
		} else {
			r.Hecho = "silenciado"
			s.registra(almacen.PersonaDueno, "persona.tope_silenciar", objetivo, chat, "ok", fmt.Sprintf("min=%d", min))
		}
	case AccBorrar, AccExpulsar:
		esGrupo := strings.HasSuffix(chat, "@g.us")
		switch {
		case !esGrupo:
			solo("solo funciona en grupos")
		case r.Accion == AccBorrar && msgID == "":
			solo("sin mensaje que borrar (advierte respondiendo al mensaje)")
		default:
			ok, err := s.grupo().EsAdmin(ctx, chat)
			switch {
			case err != nil && !errors.Is(err, grupos.ErrNoDisponible):
				solo("no pude saber si soy administrador")
			case !ok:
				solo("no soy administrador del grupo")
			case r.Accion == AccBorrar:
				if err := s.grupo().BorrarMensajeAjeno(ctx, chat, msgID, s.persona(objetivo)); err != nil {
					solo(motivoError(err))
				} else {
					r.Hecho = "borrado"
				}
			default:
				if err := s.grupo().Expulsar(ctx, chat, s.persona(objetivo)); err != nil {
					solo(motivoError(err))
				} else {
					r.Hecho = "expulsado"
				}
			}
		}
		res := "ok"
		if r.Hecho == "solo_aviso" {
			res = "no_hecho"
		}
		s.registra(almacen.PersonaDueno, "persona.tope_"+r.Accion, objetivo, chat, res, "")
	default:
		r.Hecho = "avisado"
	}
	// Siempre le llega una notificación al dueño.
	titulo := "Advertencias: llegó al tope"
	detalle := fmt.Sprintf("%s lleva %d advertencias (tope %d).", nombre, r.Total, r.Tope)
	switch r.Hecho {
	case "silenciado":
		detalle += " El bot la silenció."
	case "borrado":
		detalle += " Se borró su mensaje."
	case "expulsado":
		detalle += " Se la expulsó del grupo."
	case "solo_aviso":
		detalle += " No hice la acción (" + r.Motivo + "): decide tú."
	}
	alerta(titulo, detalle)
}

func motivoError(err error) string {
	if errors.Is(err, grupos.ErrNoDisponible) {
		return "esa acción todavía no está disponible"
	}
	return "WhatsApp no lo permitió"
}

// QuitarAdvertencia quita la última advertencia (id 0) o una concreta. Devuelve cuántas quedan.
func (s *Servicio) QuitarAdvertencia(actor, objetivo string, id int64) (int, bool, error) {
	if err := s.puede(actor, objetivo); err != nil {
		s.registra(actor, "persona.quitar_advertencia", objetivo, "", "denegado", "")
		return 0, false, err
	}
	quedan, quitada, err := s.Alm.QuitarAdvertenciaEn(s.BotID, objetivo, id, s.ahora())
	if err != nil {
		s.registra(actor, "persona.quitar_advertencia", objetivo, "", "error", "")
		return 0, false, err
	}
	s.registra(actor, "persona.quitar_advertencia", objetivo, "", "ok", fmt.Sprintf("quedan=%d", quedan))
	return quedan, quitada, nil
}

// BorrarHistorialAdvertencias borra todas las advertencias de la persona (destructivo: la pantalla y el comando piden confirmar).
func (s *Servicio) BorrarHistorialAdvertencias(actor, objetivo string) (int, error) {
	if err := s.puede(actor, objetivo); err != nil {
		s.registra(actor, "persona.borrar_advertencias", objetivo, "", "denegado", "")
		return 0, err
	}
	n, err := s.Alm.BorrarAdvertencias(s.BotID, objetivo)
	if err != nil {
		s.registra(actor, "persona.borrar_advertencias", objetivo, "", "error", "")
		return 0, err
	}
	s.registra(actor, "persona.borrar_advertencias", objetivo, "", "ok", fmt.Sprintf("n=%d", n))
	return n, nil
}

// ---------- notas ----------

// AgregarNota guarda una nota privada sobre la persona.
func (s *Servicio) AgregarNota(actor, objetivo, texto string) error {
	if err := s.puede(actor, objetivo); err != nil {
		s.registra(actor, "persona.nota", objetivo, "", "denegado", "")
		return err
	}
	if _, err := s.Alm.AgregarNota(s.BotID, objetivo, texto, s.persona(actor), s.ahora()); err != nil {
		s.registra(actor, "persona.nota", objetivo, "", "error", "")
		return err
	}
	s.registra(actor, "persona.nota", objetivo, "", "ok", "")
	return nil
}

// BorrarNota quita una nota.
func (s *Servicio) BorrarNota(actor, objetivo string, id int64) error {
	if err := s.puede(actor, objetivo); err != nil {
		s.registra(actor, "persona.borrar_nota", objetivo, "", "denegado", "")
		return err
	}
	ok, err := s.Alm.BorrarNota(s.BotID, objetivo, id)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("esa nota no existe")
	}
	s.registra(actor, "persona.borrar_nota", objetivo, "", "ok", "")
	return nil
}

// Notas devuelve las notas de la persona.
func (s *Servicio) Notas(actor, objetivo string) ([]almacen.Nota, error) {
	if err := s.puede(actor, objetivo); err != nil {
		return nil, err
	}
	return s.Alm.ListarNotas(s.BotID, objetivo)
}

// ---------- ficha y estadísticas ----------

// FichaCompleta es la ficha con el nombre del rango.
type FichaCompleta struct {
	almacen.Ficha
	Rango      string `json:"rango"`
	RangoColor string `json:"rangoColor"`
}

// Ficha arma la ficha de la persona.
func (s *Servicio) Ficha(actor, objetivo string) (*FichaCompleta, error) {
	if s.persona(objetivo) == "" {
		return nil, ErrSinPersona
	}
	if s.persona(objetivo) != almacen.PersonaDueno && s.persona(actor) != almacen.PersonaDueno {
		if err := s.puede(actor, objetivo); err != nil && !errors.Is(err, ErrYoMismo) {
			s.registra(actor, "persona.ficha", objetivo, "", "denegado", "")
			return nil, err
		}
	}
	f, err := s.Alm.FichaDe(s.BotID, objetivo, s.ahora())
	if err != nil {
		return nil, err
	}
	out := &FichaCompleta{Ficha: *f}
	if r := s.Perm.RolDe(objetivo); r != nil {
		out.Rango, out.RangoColor = r.Nombre, r.Color
	}
	s.registra(actor, "persona.ficha", objetivo, "", "ok", "")
	return out, nil
}

// Estadisticas devuelve los números de actividad de la persona.
func (s *Servicio) Estadisticas(actor, objetivo string, dias int) (*almacen.EstadisticasPersona, error) {
	if err := s.puede(actor, objetivo); err != nil && !errors.Is(err, ErrYoMismo) {
		return nil, err
	}
	return s.Alm.EstadisticasDePersona(s.BotID, objetivo, dias)
}

// ---------- olvidar y exportar ----------

// Olvidar borra todo lo de la persona (destructivo: quien llama debe haber pedido confirmación).
func (s *Servicio) Olvidar(actor, objetivo string) (*almacen.ResultadoOlvidar, error) {
	if err := s.puede(actor, objetivo); err != nil {
		s.registra(actor, "persona.olvidar", objetivo, "", "denegado", "")
		return nil, err
	}
	canon := s.persona(objetivo)
	r, err := s.Alm.OlvidarPersona(s.BotID, canon)
	if err != nil {
		s.registra(actor, "persona.olvidar", objetivo, "", "error", "")
		return nil, err
	}
	// El registro conserva que se olvidó a alguien, pero ya sin saber a quién.
	s.Alm.RegistrarAccion(s.BotID, s.persona(actor), "persona.olvidar", "", "", "ok", fmt.Sprintf("mensajes=%d", r.Mensajes))
	return r, nil
}

// Exportar guarda los datos de la persona en un archivo cifrado (BCF1) y devuelve su ruta relativa en la carpeta de datos.
func (s *Servicio) Exportar(actor, objetivo string) (string, error) {
	if err := s.puede(actor, objetivo); err != nil {
		s.registra(actor, "persona.exportar", objetivo, "", "denegado", "")
		return "", err
	}
	ex, err := s.Alm.DatosDePersona(s.BotID, objetivo, s.ahora())
	if err != nil {
		s.registra(actor, "persona.exportar", objetivo, "", "error", "")
		return "", err
	}
	b, err := json.Marshal(ex)
	if err != nil {
		return "", err
	}
	rel, err := s.Alm.EscribirExporte(s.BotID, objetivo, b, s.ahora())
	if err != nil {
		s.registra(actor, "persona.exportar", objetivo, "", "error", "")
		return "", err
	}
	s.Alm.LimpiarExportes(s.BotID, 24, s.ahora())
	s.registra(actor, "persona.exportar", objetivo, "", "ok", fmt.Sprintf("mensajes=%d", len(ex.Mensajes)))
	return rel, nil
}

// ---------- chat ----------

// Control: tomar el control es callar al bot en ese chat por un año (se quita con Reanudar).
const DuracionControl = 365 * 24 * time.Hour

// PausarChat hace que el bot calle en el chat durante `dur`.
func (s *Servicio) PausarChat(actor, chat string, dur time.Duration, control bool) error {
	if chat == "" {
		return errors.New("falta el chat")
	}
	if dur <= 0 {
		dur = time.Hour
	}
	if err := s.Alm.PausarBot(s.BotID, chat, s.ahora().Add(dur).Unix()); err != nil {
		s.registra(actor, "chat.pausar", "", chat, "error", "")
		return err
	}
	acc := "chat.pausar"
	if control {
		acc = "chat.tomar_control"
	}
	s.registra(actor, acc, "", chat, "ok", fmt.Sprintf("seg=%d", int64(dur.Seconds())))
	return nil
}

// ReanudarChat quita la pausa y el control; el bot vuelve a contestar ahí.
func (s *Servicio) ReanudarChat(actor, chat string) error {
	if chat == "" {
		return errors.New("falta el chat")
	}
	s.Alm.ReactivarSesiones(s.BotID, chat)
	if err := s.Alm.PausarBot(s.BotID, chat, 0); err != nil {
		s.registra(actor, "chat.reanudar", "", chat, "error", "")
		return err
	}
	if strings.HasSuffix(chat, "@g.us") {
		_ = s.Alm.FijarBotActivo(s.BotID, chat, true)
	}
	s.registra(actor, "chat.reanudar", "", chat, "ok", "")
	return nil
}

// MarcarChat pone el chat como pendiente, resuelto o sin marca ("").
func (s *Servicio) MarcarChat(actor, chat, estado string) error {
	if err := s.Alm.FijarEstadoTrabajo(s.BotID, chat, estado); err != nil {
		s.registra(actor, "chat.estado", "", chat, "error", "")
		return err
	}
	s.registra(actor, "chat.estado", "", chat, "ok", "estado="+estado)
	return nil
}

// Etiquetar pone (si=true) o quita la etiqueta de un chat. Con crear=true crea la etiqueta si no existía.
func (s *Servicio) Etiquetar(actor, chat, nombre string, si, crear bool) (*almacen.Etiqueta, error) {
	e, err := s.Alm.EtiquetaPorNombre(s.BotID, nombre)
	if err != nil {
		return nil, err
	}
	if e == nil {
		if !si {
			return nil, errors.New("esa etiqueta no existe")
		}
		if !crear {
			return nil, errors.New("esa etiqueta no existe: créala en Ajustes o con el dueño")
		}
		id, err := s.Alm.CrearEtiqueta(s.BotID, nombre, "")
		if err != nil {
			return nil, err
		}
		e = &almacen.Etiqueta{ID: id, Nombre: strings.TrimSpace(nombre)}
	}
	if err := s.Alm.PonerEtiqueta(s.BotID, chat, e.ID, si); err != nil {
		s.registra(actor, "chat.etiqueta", "", chat, "error", "")
		return nil, err
	}
	s.registra(actor, "chat.etiqueta", "", chat, "ok", fmt.Sprintf("etiqueta=%d,si=%t", e.ID, si))
	return e, nil
}

// Reportar avisa al dueño con una notificación local. NO guarda ni copia el texto del mensaje: solo el chat, quién reporta, de quién
// es el mensaje y el motivo que escribió quien reporta. El registro de acciones no lleva el motivo.
func (s *Servicio) Reportar(quien, autor, chat, motivo string) error {
	nChat := chat
	if c, _ := s.Alm.ObtenerChat(s.BotID, chat); c != nil && c.Mostrado != "" {
		nChat = c.Mostrado
	} else {
		nChat = "un chat"
	}
	det := fmt.Sprintf("%s reportó un mensaje de %s en %s.", s.Nombre(quien), s.Nombre(autor), nChat)
	if m := strings.TrimSpace(motivo); m != "" {
		if len([]rune(m)) > 200 {
			m = string([]rune(m)[:200]) + "…"
		}
		det += " Motivo: " + m + "."
	}
	if autor == "" {
		det = fmt.Sprintf("%s reportó un mensaje en %s.", s.Nombre(quien), nChat)
		if m := strings.TrimSpace(motivo); m != "" {
			det += " Motivo: " + m + "."
		}
	}
	if _, err := s.Alm.CrearAvisoDe(s.BotID, "reporte", chat, s.persona(autor), "Reporte de un mensaje", det); err != nil {
		s.registra(quien, "chat.reportar", autor, chat, "error", "")
		return err
	}
	s.log("Aviso local: reporte de un mensaje.")
	s.registra(quien, "chat.reportar", autor, chat, "ok", "")
	return nil
}
