// Package estadoapp cambia, en MI WhatsApp (la cuenta principal), el estado de un chat: silenciarlo con duración, fijarlo y
// archivarlo. Usa el "estado de la app" de WhatsApp (app state) que se comparte entre dispositivos, así que el teléfono principal
// tiene que haber compartido las llaves con este dispositivo vinculado. Si faltan, esas acciones no se pueden y la app OCULTA los
// botones (y lo explica una vez en Avanzado). Nada de esto escribe a otras personas: solo cambia ajustes de MI cuenta.
package estadoapp

import (
	"context"
	"errors"
	"strings"
	"time"

	"botchat/internal/almacen"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/types"
)

// Cliente es lo que se usa de whatsmeow (para poder probar sin red).
type Cliente interface {
	Conectado() bool
	SendAppState(ctx context.Context, patch appstate.PatchInfo) error
	// TieneLlaves dice si ya hay llaves de app state compartidas por el teléfono principal.
	TieneLlaves(ctx context.Context) (bool, error)
	// Sincronizar pide a WhatsApp los cambios pendientes de un tipo de app state (lectura; la librería respeta su propio límite).
	Sincronizar(ctx context.Context, nombre appstate.WAPatchName) error
}

// Real adapta un cliente de whatsmeow.
type Real struct{ *whatsmeow.Client }

func (r Real) Conectado() bool { return r.IsConnected() && r.IsLoggedIn() }

func (r Real) TieneLlaves(ctx context.Context) (bool, error) {
	if r.Store == nil || r.Store.AppStateKeys == nil {
		return false, nil
	}
	id, err := r.Store.AppStateKeys.GetLatestAppStateSyncKeyID(ctx)
	if err != nil {
		return false, err
	}
	return id != nil, nil
}

func (r Real) Sincronizar(ctx context.Context, nombre appstate.WAPatchName) error {
	return r.FetchAppState(ctx, nombre, false, true)
}

// Errores en español (sin punto final).
var (
	ErrSinLlaves   = errors.New("para silenciar, fijar o archivar en tu WhatsApp hace falta que el teléfono principal sincronice: abre WhatsApp en tu teléfono principal y espera un momento")
	ErrSinConexion = errors.New("no hay conexión con WhatsApp: revisa que el bot esté encendido")
	ErrChat        = errors.New("ese chat no es válido")
)

// TextoSinLlaves es la línea que la app muestra en Avanzado cuando faltan las llaves.
const TextoSinLlaves = "Para silenciar/fijar/archivar en tu WhatsApp hace falta que el teléfono principal sincronice (abre WhatsApp en tu teléfono principal y espera un momento)."

// Estado dice si se puede usar el estado de la app.
type Estado struct {
	Disponible bool   `json:"disponible"`
	Conectado  bool   `json:"conectado"`
	Motivo     string `json:"motivo"` // vacío si todo está bien
}

// Servicio aplica los cambios en mi cuenta.
type Servicio struct {
	BotID int64
	Alm   *almacen.Almacen
	Cli   Cliente
	Ahora func() time.Time
	Log   func(string)
}

// Nuevo crea el servicio.
func Nuevo(alm *almacen.Almacen, botID int64, cli Cliente, log func(string)) *Servicio {
	if log == nil {
		log = func(string) {}
	}
	return &Servicio{BotID: botID, Alm: alm, Cli: cli, Ahora: time.Now, Log: log}
}

func (s *Servicio) ahora() time.Time {
	if s.Ahora != nil {
		return s.Ahora()
	}
	return time.Now()
}

// Estado mira si hay conexión y llaves. Es barato: se puede preguntar cada vez que se abre una ficha.
func (s *Servicio) Estado(ctx context.Context) Estado {
	if s.Cli == nil || !s.Cli.Conectado() {
		return Estado{Motivo: "El bot no está conectado. Conéctalo para silenciar, fijar o archivar en tu WhatsApp."}
	}
	ok, err := s.Cli.TieneLlaves(ctx)
	if err != nil || !ok {
		return Estado{Conectado: true, Motivo: TextoSinLlaves}
	}
	return Estado{Disponible: true, Conectado: true}
}

// Disponible dice si se pueden usar silenciar, fijar y archivar ahora mismo.
func (s *Servicio) Disponible(ctx context.Context) bool { return s.Estado(ctx).Disponible }

func (s *Servicio) objetivo(chat string) (types.JID, error) {
	j, err := types.ParseJID(chat)
	if err != nil || j.IsEmpty() || j.Server == types.BroadcastServer {
		return types.EmptyJID, ErrChat
	}
	return j.ToNonAD(), nil
}

// explica traduce los errores de la librería (incluido el de las llaves, que solo viene como texto) a mensajes claros.
func explica(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, appstate.ErrKeyNotFound), strings.Contains(err.Error(), "no app state keys found"):
		return ErrSinLlaves
	case errors.Is(err, whatsmeow.ErrNotConnected), errors.Is(err, whatsmeow.ErrIQDisconnected):
		return ErrSinConexion
	case errors.Is(err, whatsmeow.ErrAppStateRateLimited):
		return errors.New("WhatsApp pide ir más despacio: espera un momento y vuelve a intentar")
	case errors.Is(err, whatsmeow.ErrAppStateUpdate):
		return errors.New("WhatsApp no aceptó el cambio: abre WhatsApp en tu teléfono principal y vuelve a intentar")
	case errors.Is(err, context.DeadlineExceeded):
		return errors.New("WhatsApp tardó demasiado en responder: intenta otra vez en un momento")
	}
	return errors.New("WhatsApp no pudo hacerlo ahora")
}

func (s *Servicio) registra(accion, chat, res string) {
	s.Alm.RegistrarAccion(s.BotID, almacen.PersonaDueno, accion, "", chat, res, "")
}

// enviar revisa conexión y llaves y manda el parche. Nunca intenta mandar sin llaves.
func (s *Servicio) enviar(ctx context.Context, accion, chat string, patch appstate.PatchInfo) error {
	if st := s.Estado(ctx); !st.Disponible {
		s.registra(accion, chat, "no_disponible")
		if !st.Conectado {
			return ErrSinConexion
		}
		return ErrSinLlaves
	}
	if err := s.Cli.SendAppState(ctx, patch); err != nil {
		s.Log("No se pudo cambiar el estado de un chat en mi WhatsApp: " + err.Error())
		s.registra(accion, chat, "error")
		return explica(err)
	}
	s.registra(accion, chat, "ok")
	return nil
}

// Silenciar silencia el chat en mi WhatsApp durante `dur` (0 = siempre).
func (s *Servicio) Silenciar(ctx context.Context, chat string, dur time.Duration) error {
	j, err := s.objetivo(chat)
	if err != nil {
		return err
	}
	return s.enviar(ctx, "wa.silenciar", chat, appstate.BuildMute(j, true, dur))
}

// Desilenciar quita el silencio del chat en mi WhatsApp.
func (s *Servicio) Desilenciar(ctx context.Context, chat string) error {
	j, err := s.objetivo(chat)
	if err != nil {
		return err
	}
	return s.enviar(ctx, "wa.desilenciar", chat, appstate.BuildMute(j, false, 0))
}

// Fijar fija o desfija el chat arriba de la lista de mi WhatsApp.
func (s *Servicio) Fijar(ctx context.Context, chat string, fijar bool) error {
	j, err := s.objetivo(chat)
	if err != nil {
		return err
	}
	acc := "wa.desfijar"
	if fijar {
		acc = "wa.fijar"
	}
	return s.enviar(ctx, acc, chat, appstate.BuildPin(j, fijar))
}

// Archivar archiva o desarchiva el chat en mi WhatsApp (archivar también lo desfija, como hace WhatsApp).
func (s *Servicio) Archivar(ctx context.Context, chat string, archivar bool) error {
	j, err := s.objetivo(chat)
	if err != nil {
		return err
	}
	var ult time.Time
	if c, _ := s.Alm.ObtenerChat(s.BotID, chat); c != nil && c.UltimoTS > 0 {
		ult = time.Unix(c.UltimoTS, 0)
	}
	acc := "wa.desarchivar"
	if archivar {
		acc = "wa.archivar"
	}
	return s.enviar(ctx, acc, chat, appstate.BuildArchive(j, archivar, ult, nil))
}

// PedirSincronizacion le pide a WhatsApp los cambios pendientes de los ajustes de chats (lectura). Si el teléfono principal ya compartió
// las llaves, esto las deja usables; si no, la propia librería vuelve a pedírselas al teléfono. No fuerza una sincronización completa.
func (s *Servicio) PedirSincronizacion(ctx context.Context) error {
	if s.Cli == nil || !s.Cli.Conectado() {
		return ErrSinConexion
	}
	var ultimo error
	for _, n := range []appstate.WAPatchName{appstate.WAPatchRegularHigh, appstate.WAPatchRegularLow} {
		if err := s.Cli.Sincronizar(ctx, n); err != nil {
			ultimo = err
		}
	}
	s.registra("wa.sincronizar", "", "ok")
	if ultimo != nil {
		return explica(ultimo)
	}
	return nil
}
