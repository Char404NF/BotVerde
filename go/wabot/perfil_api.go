package wabot

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"botchat/internal/almacen"
)

// API de perfil de persona, actividad, lista de lo que envió, búsqueda en un chat, puntitos y exportar chat (v0.6.0, E4).
// Todo devuelve JSON. Los textos se descifran solo de la página pedida (y, en la búsqueda por palabra, por lotes).

// PerfilPersona devuelve (JSON) la cabecera del perfil: nombre, número, rango, etiquetas, foto, contadores y grupos en común.
func PerfilPersona(botID int64, persona string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.PerfilDe(botID, persona, time.Now()))
}

// ActividadPersona devuelve (JSON) [tarjeta…]: primero "Chat privado", luego un grupo por renglón, con contadores por tipo.
func ActividadPersona(botID int64, persona string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ActividadPersona(botID, persona))
}

// MensajesDePersona devuelve (JSON) una página de lo que la persona envió. filtroJSON:
// {chat, persona, pestana ("", mensajes, fotos, audios, archivos), desde, hasta (hora Unix), palabra, soloMarcados, soloBorrados, soloEditados}.
// Para la siguiente página se pasan sigTs y sigId de la anterior; la primera usa 0 y "". Cada mensaje trae marcaRoja ("borrado" o
// "editado"), borradoPor, versiones y etiquetas. Con palabra, la búsqueda descifra por lotes (total = -1 hasta recorrer todo).
func MensajesDePersona(botID int64, filtroJSON string, antesTS int64, antesID string, limite int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	f, err := almacen.ParseFiltroPersona(filtroJSON)
	if err != nil {
		return "", err
	}
	return js(a.MensajesDePersona(botID, f, antesTS, antesID, limite))
}

// BuscarEnChat busca texto en un chat (menú de tres puntos). Mismo formato de página que MensajesDePersona.
func BuscarEnChat(botID int64, chat, palabra string, desde, hasta, antesTS int64, antesID string, limite int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.BuscarEnChat(botID, chat, palabra, desde, hasta, antesTS, antesID, limite))
}

// PuntitosDeMensajes devuelve (JSON) {idMensaje: [idEtiqueta…]} con las etiquetas de quien envió cada mensaje (idsJSON: arreglo de ids).
func PuntitosDeMensajes(botID int64, chat, idsJSON string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	var ids []string
	if err := jsonEn(idsJSON, &ids); err != nil {
		return "", err
	}
	return js(a.PuntitosDeMensajes(botID, chat, ids), nil)
}

// ExportarChat guarda el chat (formato "json" o "txt"; desde/hasta en hora Unix, 0 = sin límite) en un archivo cifrado BCF1 y devuelve
// (JSON) {ruta, mensajes}. Se borra solo a las 24 horas.
func ExportarChat(botID int64, chat, formato string, desde, hasta int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	rel, n, err := a.EscribirExporteChat(botID, chat, strings.ToLower(strings.TrimSpace(formato)), desde, hasta, time.Now())
	if err != nil {
		return "", err
	}
	accionDueno(botID, "chat.exportar", "", "mensajes="+strconv.Itoa(n))
	b, _ := json.Marshal(map[string]any{"ruta": rel, "mensajes": n})
	return string(b), nil
}

// EstadosDePersonas devuelve (JSON) {persona: {silenciado, ignorada, advertido}} solo de quienes tienen algo, para los puntitos de la
// lista de Personas (personasJSON: arreglo de JIDs de chats privados). Tres consultas en total.
func EstadosDePersonas(botID int64, personasJSON string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	var ps []string
	if err := jsonEn(personasJSON, &ps); err != nil {
		return "", err
	}
	return js(a.EstadosDePersonas(botID, ps, time.Now()), nil)
}
