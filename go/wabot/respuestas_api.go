package wabot

import (
	"encoding/json"
	"errors"

	"botchat/internal/respuestas"
)

// RespuestasLista devuelve (JSON) todo lo que el bot contesta por su cuenta cuando alguien usa un comando: cada respuesta con su
// texto de fábrica, el texto propio (vacío = el de fábrica), las {variables} que admite y la sección donde va; y las frases cambiadas.
func RespuestasLista(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(map[string]any{
		"grupos":     respuestas.Grupos(),
		"filas":      respuestas.Lista(a, botID),
		"reemplazos": append([]respuestas.Reemplazo{}, respuestas.Reemplazos(a, botID)...),
	}, nil)
}

// RespuestaFijar cambia el texto de una respuesta; vacío vuelve al de fábrica. Rechaza {variables} que esa respuesta no tiene.
func RespuestaFijar(botID int64, id, texto string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	if err := respuestas.Validar(id, texto); err != nil {
		return err
	}
	accionDueno(botID, "respuesta.cambiar", id, "")
	return a.FijarRespuesta(botID, id, texto)
}

// RespuestasRestaurar vuelve todas las respuestas (y las frases cambiadas) al texto de fábrica. Devuelve cuántas se tocaron.
func RespuestasRestaurar(botID int64) (int, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	accionDueno(botID, "respuesta.restaurar", "", "")
	return a.RestaurarRespuestas(botID), nil
}

// ReemplazosFijar guarda la lista de frases cambiadas ([{de, a}]).
func ReemplazosFijar(botID int64, listaJSON string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	var l []respuestas.Reemplazo
	if err := json.Unmarshal([]byte(listaJSON), &l); err != nil {
		return errors.New("la lista no es válida")
	}
	accionDueno(botID, "respuesta.reemplazos", "", "")
	return respuestas.FijarReemplazos(a, botID, l)
}
