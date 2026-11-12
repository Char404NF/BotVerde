package wabot

import (
	"context"
	"fmt"
	"time"
)

// ---------- Auditoría (v0.6.0, etapa E1) ----------
// Interruptor de texto global y por chat/grupo, retención (30/90/siempre), historial de ediciones y "borrar todo de esta persona".
// De fábrica se guarda el texto (encendido); la instalación nueva conserva 90 días y quien actualiza debe elegir antes de que se purgue nada.

// AuditoriaAjustes devuelve (JSON) {defecto, retDias, retPendiente, avisoVisto}.
func AuditoriaAjustes(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.Auditoria(botID), nil)
}

// FijarAuditoria cambia el interruptor global (guardar el texto) y la retención global: 30, 90 o 0 (siempre); con un número negativo
// la retención se deja como está. Elegir una retención termina el estado "pendiente de elegir".
func FijarAuditoria(botID int64, guardar bool, retDias int) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.FijarAuditoria(botID, guardar, retDias)
}

// RetencionPendiente dice si el usuario viene de una versión anterior y todavía no eligió cuánto conservar (no se purga nada mientras).
func RetencionPendiente(botID int64) (bool, error) {
	a, err := almacenAbierto()
	if err != nil {
		return false, err
	}
	return a.Auditoria(botID).RetPendiente, nil
}

// FijarAvisoAuditoriaVisto anota que ya se mostró el aviso de auditoría.
func FijarAvisoAuditoriaVisto(botID int64, visto bool) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	v := "0"
	if visto {
		v = "1"
	}
	return a.FijarAjusteBot(botID, "audit_aviso_visto", v)
}

// AuditoriaChat devuelve (JSON) la auditoría de un chat: {audit (-1 = usa el global), efectivo, retDias (-1 = usa la global), retEfectiva}.
func AuditoriaChat(botID int64, chat string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.AuditoriaChat(botID, chat), nil)
}

// FijarAuditoriaChat cambia el interruptor del chat (-1 = usar el global, 0 = no guardar el texto, 1 = guardarlo) y su retención
// (-1 = usar la global, 0 = siempre, 30 o 90). Solo vale para lo que llegue desde ahora.
func FijarAuditoriaChat(botID int64, chat string, audit, retDias int) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.FijarAuditoriaChat(botID, chat, audit, retDias)
}

// ListarAuditoriaChats devuelve (JSON) los chats y grupos con su auditoría, del más reciente al más viejo. consulta filtra por nombre o número.
func ListarAuditoriaChats(botID int64, consulta string, limite, desde int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ListarAuditoriaChats(botID, consulta, limite, desde))
}

// VersionesDeMensaje devuelve (JSON) los textos anteriores de un mensaje editado, de la más vieja a la más reciente.
func VersionesDeMensaje(botID int64, chat, id string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.VersionesDeMensaje(botID, chat, id))
}

// LimpiarPorRetencion olvida ahora el texto más viejo que la retención (conserva las filas). Devuelve (JSON) {pendiente, mensajes, ediciones}.
// Mientras la retención esté pendiente de elegir no toca nada.
func LimpiarPorRetencion(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.LimpiarPorRetencion(context.Background(), botID, time.Now()))
}

// BorrarTodoDePersona borra todo lo de esa persona (mensajes con sus ediciones, archivos, notas, advertencias…) igual que OlvidarPersona.
// Devuelve (JSON) lo que se borró. La pantalla pide confirmar antes.
func BorrarTodoDePersona(botID int64, persona string) (string, error) {
	return OlvidarPersona(botID, persona)
}

// ---------- tarea periódica ----------

const (
	retEspera   = 45 * time.Second // al arrancar, un rato antes de la primera pasada
	retCadaHora = time.Hour
)

// mantenimientoAuditoria corre al arrancar y cada hora: retención del texto viejo y relleno de huellas (sha256) de archivos anteriores.
// Nunca deja caer el servicio: cualquier fallo se anota (sin datos personales) y se sigue.
func (s *estado) mantenimientoAuditoria(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			s.log(fmt.Sprintf("error interno en el mantenimiento de auditoría: %v", r))
		}
	}()
	espera := retEspera
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(espera):
		}
		espera = retCadaHora
		s.pasadaAuditoria(ctx)
	}
}

func (s *estado) pasadaAuditoria(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			s.log(fmt.Sprintf("error interno en la pasada de auditoría: %v", r))
		}
	}()
	a, err := almacenAbierto()
	if err != nil {
		return
	}
	if res, err := a.LimpiarPorRetencion(ctx, s.botID, time.Now()); err != nil {
		if ctx.Err() == nil {
			s.log("retención: " + err.Error())
		}
	} else if res.Mensajes > 0 {
		s.log(fmt.Sprintf("Retención: se olvidó el texto de %d mensajes viejos.", res.Mensajes))
	}
	for ctx.Err() == nil {
		_, fin, err := a.RellenarSHA(s.botID, 200)
		if err != nil {
			s.log("huellas de archivos: " + err.Error())
			return
		}
		if fin {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}
