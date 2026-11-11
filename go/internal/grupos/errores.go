package grupos

import (
	"context"
	"errors"

	"go.mau.fi/whatsmeow"
)

// Errores que se le explican a quien pidió la acción. Van sin punto final: el despachador de comandos lo agrega y la app los muestra tal cual.
var (
	ErrNoAdmin      = errors.New("no soy administrador de este grupo: hazme administrador en WhatsApp para poder hacerlo")
	ErrSinConexion  = errors.New("no hay conexión con WhatsApp: revisa que el bot esté encendido")
	ErrNoEsGrupo    = errors.New("eso solo funciona en un grupo")
	ErrNoMiembro    = errors.New("esa persona no está en el grupo")
	ErrSinPersona   = errors.New("dime a quién: menciónalo, responde a su mensaje o escribe su número")
	ErrYoMismo      = errors.New("no puedo hacer eso conmigo mismo")
	ErrCreador      = errors.New("no se puede hacer eso con quien creó el grupo")
	ErrYaAdmin      = errors.New("esa persona ya es administradora")
	ErrNoEsAdmin    = errors.New("esa persona no es administradora")
	ErrTextoVacio   = errors.New("falta el texto")
	ErrTextoLargo   = errors.New("el texto es demasiado largo")
	ErrSinMensaje   = errors.New("no encuentro ese mensaje en este teléfono")
	ErrNoHaySolic   = errors.New("no hay solicitudes pendientes")
	ErrFiltroNoHay  = errors.New("ese filtro no existe: usa enlaces, palabras, repetidos o flood")
	ErrAccionNoHay  = errors.New("esa acción no existe: usa avisarme, avisar_grupo, borrar, advertir, silenciar o expulsar")
	ErrSinMenciones = errors.New("no hay a quién mencionar")
)

// Fallo es un error ya explicado en español; guarda la causa original para los registros (nunca se muestra).
type Fallo struct {
	Msg   string
	Causa error
}

func (f *Fallo) Error() string { return f.Msg }
func (f *Fallo) Unwrap() error { return f.Causa }

func fallo(msg string, causa error) error { return &Fallo{Msg: msg, Causa: causa} }

// Explicar traduce los errores de WhatsApp (los tipados de whatsmeow y los códigos de las consultas) a un mensaje claro en español.
// Los errores de este paquete pasan tal cual. Nunca devuelve el texto en inglés de la librería.
func Explicar(err error) error {
	if err == nil {
		return nil
	}
	var f *Fallo
	if errors.As(err, &f) {
		return err
	}
	for _, propio := range []error{ErrNoAdmin, ErrSinConexion, ErrNoEsGrupo, ErrNoMiembro, ErrSinPersona, ErrYoMismo, ErrCreador, ErrYaAdmin, ErrNoEsAdmin,
		ErrTextoVacio, ErrTextoLargo, ErrSinMensaje, ErrNoHaySolic, ErrFiltroNoHay, ErrAccionNoHay, ErrSinMenciones, ErrNoDisponible} {
		if errors.Is(err, propio) {
			return err
		}
	}
	switch {
	case errors.Is(err, whatsmeow.ErrGroupInviteLinkUnauthorized):
		return fallo("solo los administradores pueden ver o reiniciar el enlace del grupo", err)
	case errors.Is(err, whatsmeow.ErrNotInGroup):
		return fallo("ya no estás en ese grupo", err)
	case errors.Is(err, whatsmeow.ErrGroupNotFound):
		return fallo("ese grupo ya no existe", err)
	case errors.Is(err, whatsmeow.ErrNotConnected), errors.Is(err, whatsmeow.ErrIQDisconnected), errors.Is(err, whatsmeow.ErrNotLoggedIn):
		return fallo(ErrSinConexion.Error(), err)
	case errors.Is(err, whatsmeow.ErrIQTimedOut), errors.Is(err, context.DeadlineExceeded):
		return fallo("WhatsApp tardó demasiado en responder: intenta otra vez en un momento", err)
	case errors.Is(err, whatsmeow.ErrIQForbidden), errors.Is(err, whatsmeow.ErrIQNotAuthorized):
		return fallo("WhatsApp no lo permitió: casi seguro ya no soy administrador de este grupo", err)
	case errors.Is(err, whatsmeow.ErrIQNotFound):
		return fallo("WhatsApp no lo encontró: la persona o el grupo ya no existe", err)
	case errors.Is(err, whatsmeow.ErrIQNotAllowed), errors.Is(err, whatsmeow.ErrIQNotAcceptable):
		return fallo("WhatsApp no aceptó el cambio (revisa que el dato sea válido y que la persona siga en el grupo)", err)
	case errors.Is(err, whatsmeow.ErrIQResourceLimit):
		return fallo("WhatsApp dice que se llegó a un límite del grupo", err)
	case errors.Is(err, whatsmeow.ErrIQRateOverLimit):
		return fallo("WhatsApp pide ir más despacio: espera un momento y vuelve a intentar", err)
	case errors.Is(err, whatsmeow.ErrIQInternalServerError), errors.Is(err, whatsmeow.ErrIQServiceUnavailable), errors.Is(err, whatsmeow.ErrIQPartialServerError):
		return fallo("WhatsApp no está disponible ahora: intenta de nuevo en un rato", err)
	case errors.Is(err, context.Canceled):
		return fallo("se canceló antes de terminar", err)
	}
	return fallo("WhatsApp no pudo hacerlo ahora", err)
}
