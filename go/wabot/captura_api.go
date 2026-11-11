package wabot

import (
	"errors"
	"fmt"

	"go.mau.fi/whatsmeow/types/events"

	"botchat/internal/captura"
	"botchat/internal/ingesta"
)

// Captura de mensajes (diagnóstico del Laboratorio): solo estructura, solo en memoria, apagada por defecto (se apaga sola a los 10
// minutos o a los 30 mensajes). El manejador de eventos no la toca: se arma en el trabajador.

// capturar anota la estructura de un mensaje entrante si la captura está encendida.
func (s *estado) capturar(v *events.Message) {
	if s.captura == nil || !s.captura.Activa() {
		return
	}
	s.captura.Registrar(v, clasifDe(v))
}

func clasifDe(v *events.Message) string {
	c := ingesta.Clasificar(v)
	switch {
	case c.Omitir:
		return "omitido"
	case c.Revocar != "":
		return "borrado"
	case c.Editar != nil:
		return "edición"
	case c.Reaccion != nil:
		return "reacción"
	}
	t := c.Tipo
	if t == "" {
		t = "?"
	}
	return fmt.Sprintf("Tipo=%s", t)
}

func capturaDe(botID int64) (*captura.Servicio, error) {
	mu.Lock()
	s := st
	mu.Unlock()
	if s == nil || s.botID != botID || s.captura == nil {
		return nil, errors.New("Enciende el bot para capturar mensajes.")
	}
	return s.captura, nil
}

// EstadoCaptura devuelve (JSON) el interruptor y la lista capturada. Sin el bot en marcha, todo vacío y apagado.
func EstadoCaptura(botID int64) (string, error) {
	c, err := capturaDe(botID)
	if err != nil {
		return js(captura.Estado{Max: captura.MaxMensajes, Items: []captura.Item{}}, nil)
	}
	e := c.Estado()
	if e.Items == nil {
		e.Items = []captura.Item{}
	}
	return js(e, nil)
}

// ActivarCaptura enciende o apaga la captura (al encender corren 10 minutos o 30 mensajes).
func ActivarCaptura(botID int64, activo bool) error {
	c, err := capturaDe(botID)
	if err != nil {
		return err
	}
	c.Activar(activo)
	return nil
}

// MostrarTextosCaptura incluye (o no) los textos de hasta 60 caracteres.
func MostrarTextosCaptura(botID int64, mostrar bool) error {
	c, err := capturaDe(botID)
	if err != nil {
		return err
	}
	c.MostrarTextos(mostrar)
	return nil
}

// BorrarCaptura vacía la lista.
func BorrarCaptura(botID int64) error {
	c, err := capturaDe(botID)
	if err != nil {
		return err
	}
	c.Borrar()
	return nil
}
