package wabot

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"botchat/internal/almacen"
)

// Advertencias con sanciones (v0.6.0, E5): tipos, escalera por tipo, reglas del grupo como lista y advertir con tipo.

// ListarTiposAdvertencia devuelve (JSON) los tipos con su escalera: [{id, nombre, emoji, color, posicion, caducidadDias, reglaId, activo, escalera:[{nivel, accion, minutos, alcance, aviso}]}].
// La primera vez siembra Spam, Insultos, Enlaces y Otro.
func ListarTiposAdvertencia(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ListarTiposAdv(botID, false))
}

// GuardarTipoAdvertencia crea (id 0) o edita un tipo con su escalera (JSON del mismo formato). Devuelve su id.
func GuardarTipoAdvertencia(botID int64, tipoJSON string) (int64, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	var t almacen.TipoAdv
	if err := json.Unmarshal([]byte(tipoJSON), &t); err != nil {
		return 0, errors.New("el tipo no es válido")
	}
	id, err := a.GuardarTipoAdv(botID, t)
	if err == nil {
		accionDueno(botID, "advertencias.tipo_guardar", "", "")
	}
	return id, err
}

// BorrarTipoAdvertencia borra un tipo; sus advertencias quedan sin tipo.
func BorrarTipoAdvertencia(botID, id int64) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	if err := a.BorrarTipoAdv(botID, id); err != nil {
		return err
	}
	accionDueno(botID, "advertencias.tipo_borrar", "", "")
	return nil
}

// ListarEscalera devuelve (JSON) los pasos de un tipo.
func ListarEscalera(botID, tipoID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.Escalera(tipoID), nil)
}

// GuardarEscalera reemplaza los pasos de un tipo (JSON: [{nivel, accion, minutos, alcance, aviso}]).
func GuardarEscalera(botID, tipoID int64, pasosJSON string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	var ps []almacen.PasoEsc
	if err := json.Unmarshal([]byte(pasosJSON), &ps); err != nil {
		return errors.New("la escalera no es válida")
	}
	if err := a.GuardarEscalera(botID, tipoID, ps); err != nil {
		return err
	}
	accionDueno(botID, "advertencias.escalera_guardar", "", "")
	return nil
}

// AdvertirConTipo pone una advertencia de un tipo (y, opcional, ligada a una regla; 0 = la del tipo) y aplica el paso de la escalera
// que toca. chat "" = su chat privado. Devuelve (JSON) {total, nivel, paso, hecho, motivo, regla, sancionId…}. La expulsión nunca
// es sola: queda esperando ConfirmarSancion.
func AdvertirConTipo(botID int64, persona, chat, msgID, motivo string, tipoID, reglaID int64) (string, error) {
	p, err := servicioPersonas(botID)
	if err != nil {
		return "", err
	}
	if chat == "" {
		chat = p.ChatPrivadoDe(persona)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return js(p.AdvertirConTipo(ctx, dueno(), persona, chat, msgID, motivo, tipoID, reglaID))
}

// ListarReglasGrupo devuelve (JSON) las reglas del grupo como lista ("" = reglas globales): [{id, grupo, pos, titulo, texto}].
func ListarReglasGrupo(botID int64, grupo string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ListarReglaItems(botID, grupo))
}

// GuardarReglaItem crea (id 0) o edita una regla. pos 0 = al final. Al cambiar la lista se regenera el texto de !reglas del grupo.
func GuardarReglaItem(botID int64, grupo string, id int64, titulo, texto string, pos int) (int64, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	return a.GuardarReglaItem(botID, almacen.ReglaItem{ID: id, Grupo: grupo, Titulo: titulo, Texto: texto, Pos: pos}, time.Now())
}

// BorrarReglaItem quita una regla.
func BorrarReglaItem(botID, id int64) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.BorrarReglaItem(botID, id, time.Now())
}

// MoverReglaItem sube (-1) o baja (1) una regla en su lista.
func MoverReglaItem(botID, id int64, delta int) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.MoverReglaItem(botID, id, delta, time.Now())
}
