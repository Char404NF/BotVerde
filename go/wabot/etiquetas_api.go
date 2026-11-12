package wabot

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"botchat/internal/almacen"
)

// API de etiquetas (personas y mensajes), "Marcados" y miembros de grupo (v0.6.0, E3). Las de chats y grupos siguen en personas_api.go.

// CrearEtiquetaEmoji crea una etiqueta con nombre, color ("#RRGGBB" o "" para el de siempre) y emoji ("" = sin emoji). Sin límite.
func CrearEtiquetaEmoji(botID int64, nombre, color, emoji string) (int64, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	id, err := a.CrearEtiquetaEmoji(botID, nombre, color, emoji)
	if err == nil {
		accionDueno(botID, "etiqueta.crear", "", "")
	}
	return id, err
}

// EditarEtiquetaEmoji cambia nombre, color y emoji.
func EditarEtiquetaEmoji(botID, id int64, nombre, color, emoji string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.EditarEtiquetaEmoji(botID, id, nombre, color, emoji)
}

// PonerEtiquetaPersona pone (si) o quita la etiqueta de una persona (clave de persona, número o LID).
func PonerEtiquetaPersona(botID int64, persona string, id int64, si bool) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.PonerEtiquetaPersona(botID, persona, id, si)
}

// EtiquetasDePersona devuelve (JSON) las etiquetas de una persona.
func EtiquetasDePersona(botID int64, persona string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.EtiquetasDePersona(botID, persona))
}

// EtiquetasDePersonas devuelve (JSON) {persona: [idEtiqueta…]} para una lista de personas (JSON de textos), de un solo golpe.
func EtiquetasDePersonas(botID int64, personasJSON string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	var ps []string
	if err := jsonEn(personasJSON, &ps); err != nil {
		return "", err
	}
	return js(a.EtiquetasDePersonas(botID, ps), nil)
}

// EtiquetasDeChats devuelve (JSON) {chat: [idEtiqueta…]} para una lista de chats (JSON de textos), de un solo golpe.
func EtiquetasDeChats(botID int64, chatsJSON string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	var cs []string
	if err := jsonEn(chatsJSON, &cs); err != nil {
		return "", err
	}
	return js(a.EtiquetasDeChats(botID, cs), nil)
}

// MarcarMensaje marca un mensaje con una etiqueta (etiquetaID > 0) y/o una nota privada cifrada. etiquetaID 0 = solo nota.
func MarcarMensaje(botID int64, chat, msgID string, etiquetaID int64, nota string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.MarcarMensaje(botID, chat, msgID, etiquetaID, nota, time.Now())
}

// QuitarMarcaMensaje quita la marca de una etiqueta del mensaje (etiquetaID < 0 quita todas las marcas y la nota).
func QuitarMarcaMensaje(botID int64, chat, msgID string, etiquetaID int64) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.QuitarMarca(botID, chat, msgID, etiquetaID)
}

// MarcasDeMensaje devuelve (JSON) las marcas de un mensaje: [{etiquetaId, nota, ts}].
func MarcasDeMensaje(botID int64, chat, msgID string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.MarcasDeMensaje(botID, chat, msgID))
}

// EtiquetasDeMensajes devuelve (JSON) {idMensaje: [idEtiqueta…]} de varios mensajes de un chat (ids en JSON de textos).
func EtiquetasDeMensajes(botID int64, chat, idsJSON string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	var ids []string
	if err := jsonEn(idsJSON, &ids); err != nil {
		return "", err
	}
	return js(a.EtiquetasDeMensajes(botID, chat, ids), nil)
}

// ListarMarcados devuelve (JSON) lo marcado. tipo: "mensaje", "persona", "chat" o "grupo" ("" = mensaje). etiqueta 0 = cualquiera.
func ListarMarcados(botID int64, tipo string, etiqueta int64, limite, desde int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ListarMarcados(botID, almacen.FiltroMarcados{Tipo: tipo, Etiqueta: etiqueta}, limite, desde))
}

// ContarMarcados devuelve (JSON) {mensaje, persona, chat, grupo}: cuántos hay de cada tipo (etiqueta 0 = cualquiera).
func ContarMarcados(botID, etiqueta int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ContarMarcados(botID, etiqueta), nil)
}

// ---------- miembros de grupo ----------

// MiembrosGrupoDetallado devuelve (JSON) los miembros guardados del grupo con rango, etiquetas, contador de mensajes e iconos.
// La lista se llena sola al conectar (y cada 24 h); ActualizarMiembrosGrupo la fuerza.
func MiembrosGrupoDetallado(botID int64, grupo string, incluirSalidos bool) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.MiembrosDe(botID, grupo, incluirSalidos, time.Now()))
}

// CambiosMiembrosGrupo devuelve (JSON) las entradas, salidas, expulsiones y ascensos del grupo, lo más reciente primero.
// antes > 0 pide los anteriores a esa hora (paginar).
func CambiosMiembrosGrupo(botID int64, grupo string, antes int64, limite int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ListarCambios(botID, grupo, antes, limite))
}

// ActualizarMiembrosGrupo vuelve a pedir a WhatsApp la lista de miembros de todos los grupos (en segundo plano).
func ActualizarMiembrosGrupo(botID int64) error {
	mu.Lock()
	s := st
	mu.Unlock()
	if s == nil || s.botID != botID || s.cli == nil || !s.cli.IsConnected() {
		return errors.New("el bot no está conectado")
	}
	go s.snapshotSiToca(s.ctx, true)
	return nil
}

// GruposEnComunPersona devuelve (JSON) [{jid, nombre}] de los grupos donde está la persona.
func GruposEnComunPersona(botID int64, persona string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	gs, err := a.GruposEnComun(botID, persona)
	if err != nil {
		return "", err
	}
	type g struct {
		JID    string `json:"jid"`
		Nombre string `json:"nombre"`
	}
	out := []g{}
	for _, j := range gs {
		x := g{JID: j}
		if c, _ := a.ObtenerChat(botID, j); c != nil {
			x.Nombre = c.Mostrado
		}
		out = append(out, x)
	}
	return js(out, nil)
}

// PedirFotoPersona pone en la cola lenta la foto de una persona (también si solo es miembro de un grupo). grupo (opcional) es un
// grupo que compartimos. forzar = ignorar la vigencia de 7 días. La foto se ve con la ruta `foto` de los miembros.
func PedirFotoPersona(botID int64, persona, grupo string, forzar bool) error {
	mu.Lock()
	s := st
	mu.Unlock()
	if s == nil || s.botID != botID || s.fotos == nil {
		return errors.New("el bot no está en marcha")
	}
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	canon := a.ResolverPersona(botID, persona)
	if canon == "" || canon == almacen.PersonaDueno {
		return errors.New("persona no válida")
	}
	if grupo == "" {
		if gs, _ := a.GruposEnComun(botID, canon); len(gs) > 0 {
			grupo = gs[0]
		}
	}
	if !strings.Contains(canon, "@") {
		return errors.New("persona no válida")
	}
	s.fotos.PedirPersona(canon, canon, grupo, forzar)
	return nil
}

// FotoPersona devuelve la ruta (relativa, cifrada) de la foto de la persona, o "" si no hay.
func FotoPersona(botID int64, persona string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return a.FotoDePersona(botID, persona), nil
}

func jsonEn(texto string, destino any) error {
	if strings.TrimSpace(texto) == "" {
		return nil
	}
	return json.Unmarshal([]byte(texto), destino)
}
