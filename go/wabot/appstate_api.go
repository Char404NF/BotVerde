package wabot

import (
	"context"
	"errors"
	"time"

	"botchat/internal/estadoapp"
)

// API del estado de la app de WhatsApp (app state): silenciar con duración, fijar y archivar chats en MI WhatsApp. No escribe a nadie.
// Solo funciona si el teléfono principal ya compartió las llaves con este dispositivo vinculado. Si faltan, la app OCULTA los botones y
// muestra en Avanzado la línea `estadoapp.TextoSinLlaves`; nada de esto produce errores confusos.

func servicioEstadoApp(botID int64) *estadoapp.Servicio {
	mu.Lock()
	s := st
	mu.Unlock()
	if s != nil && s.botID == botID {
		return s.eapp
	}
	return nil
}

func conEstadoApp(botID int64, f func(ctx context.Context, e *estadoapp.Servicio) error) error {
	e := servicioEstadoApp(botID)
	if e == nil {
		return errors.New("el bot no está encendido: enciéndelo para cambiar tu WhatsApp")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	return f(ctx, e)
}

// AppStateDisponible dice si ahora se pueden usar silenciar, fijar y archivar en tu WhatsApp (hay conexión y llaves).
func AppStateDisponible(botID int64) bool {
	e := servicioEstadoApp(botID)
	if e == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return e.Disponible(ctx)
}

// AppStateEstado devuelve (JSON {disponible, conectado, motivo}) el estado; `motivo` explica en español qué falta.
func AppStateEstado(botID int64) (string, error) {
	e := servicioEstadoApp(botID)
	if e == nil {
		return js(estadoapp.Estado{Motivo: "El bot no está encendido. Enciéndelo para silenciar, fijar o archivar en tu WhatsApp."}, nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return js(e.Estado(ctx), nil)
}

// AppStateSilenciar silencia el chat en tu WhatsApp durante `segundos` (0 = siempre; la app ofrece 8 horas, 1 semana y siempre).
func AppStateSilenciar(botID int64, chat string, segundos int64) error {
	if segundos < 0 {
		return errors.New("la duración no es válida")
	}
	return conEstadoApp(botID, func(ctx context.Context, e *estadoapp.Servicio) error {
		return e.Silenciar(ctx, chat, time.Duration(segundos)*time.Second)
	})
}

// AppStateDesilenciar quita el silencio del chat en tu WhatsApp.
func AppStateDesilenciar(botID int64, chat string) error {
	return conEstadoApp(botID, func(ctx context.Context, e *estadoapp.Servicio) error { return e.Desilenciar(ctx, chat) })
}

// AppStateFijar fija o desfija el chat en tu WhatsApp.
func AppStateFijar(botID int64, chat string, fijar bool) error {
	return conEstadoApp(botID, func(ctx context.Context, e *estadoapp.Servicio) error { return e.Fijar(ctx, chat, fijar) })
}

// AppStateArchivar archiva o desarchiva el chat en tu WhatsApp.
func AppStateArchivar(botID int64, chat string, archivar bool) error {
	return conEstadoApp(botID, func(ctx context.Context, e *estadoapp.Servicio) error { return e.Archivar(ctx, chat, archivar) })
}

// AppStateSincronizar le pide a WhatsApp los cambios pendientes de los ajustes de chats (botón "Pedir sincronización" de Avanzado).
// Es una lectura: no fuerza una sincronización completa ni escribe nada.
func AppStateSincronizar(botID int64) error {
	return conEstadoApp(botID, func(ctx context.Context, e *estadoapp.Servicio) error { return e.PedirSincronizacion(ctx) })
}
