package wabot

import (
	"encoding/json"
	"errors"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/ingesta"
)

// API de multimedia (v0.6.0, etapa E2): tope total, estado del almacenamiento por tipo, portadas de video y batería.

// TopeAlmacenamiento devuelve el tope total en bytes (0 = sin límite; de fábrica 5 GB).
func TopeAlmacenamiento(botID int64) (int64, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	return a.TopeTotal(botID), nil
}

// FijarTope cambia el tope total en bytes (0 = sin límite) y lo aplica en segundo plano si ya se pasó.
func FijarTope(botID int64, bytes int64) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	if err := a.FijarTopeTotal(botID, bytes); err != nil {
		return err
	}
	mu.Lock()
	s := st
	mu.Unlock()
	if s != nil && s.botID == botID && s.descargas != nil {
		s.descargas.PedirTope()
	}
	return nil
}

// AplicarTopeAhora aplica el tope total ya (borra lo más viejo hasta el 90 %) y devuelve (JSON) lo que hizo.
func AplicarTopeAhora(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.AplicarTope(botID, a.TopeTotal(botID), time.Now()))
}

// ConservarPortadaVideo dice si al borrar un video se conserva su portada.
func ConservarPortadaVideo(botID int64) (bool, error) {
	a, err := almacenAbierto()
	if err != nil {
		return false, err
	}
	return a.ConservarPortada(botID), nil
}

// FijarConservarPortada enciende o apaga el conservar la portada de los videos borrados.
func FijarConservarPortada(botID int64, si bool) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.FijarConservarPortada(botID, si)
}

type tipoEstado struct {
	almacen.UsoClave
	Apagado     bool  `json:"apagado"`
	TopeArchivo int64 `json:"topeArchivo"`
	TopeTotal   int64 `json:"topeTotal"`
}

type estadoAlmacenamiento struct {
	TopeTotal        int64        `json:"topeTotal"`
	Uso              int64        `json:"uso"`
	Libre            int64        `json:"libre"`
	Total            int64        `json:"total"`
	AhorradoBytes    int64        `json:"ahorradoBytes"`
	AhorradoArchivos int          `json:"ahorradoArchivos"`
	ConservarPortada bool         `json:"conservarPortada"`
	SinPortada       int          `json:"sinPortada"`
	Tipos            []tipoEstado `json:"tipos"`
}

// EstadoAlmacenamiento devuelve (JSON) el medidor contra el tope, el uso por tipo con sus topes e interruptores, lo ahorrado por
// duplicados y cuántos videos esperan su portada.
func EstadoAlmacenamiento(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	e := estadoAlmacenamiento{TopeTotal: a.TopeTotal(botID), Uso: a.UsoMultimedia(botID), ConservarPortada: a.ConservarPortada(botID), SinPortada: a.ContarSinPortada(botID)}
	e.Libre, e.Total = a.Libre()
	e.AhorradoBytes, e.AhorradoArchivos = a.AhorradoPorDuplicados(botID)
	uso, err := a.UsoPorClave(botID)
	if err != nil {
		return "", err
	}
	p := politicaActual()
	for _, u := range uso {
		t := tipoEstado{UsoClave: u, TopeArchivo: p.TopeArchivo[u.Tipo], TopeTotal: p.TopeTipoTotal[u.Tipo]}
		switch u.Tipo {
		case ingesta.TipoFoto:
			t.Apagado = p.Foto.Apagada
		case ingesta.TipoVideo:
			t.Apagado = p.Video.Apagada
			if p.VideoMaxBytes > 0 && (t.TopeArchivo == 0 || p.VideoMaxBytes < t.TopeArchivo) {
				t.TopeArchivo = p.VideoMaxBytes
			}
		case ingesta.TipoAudio:
			t.Apagado = p.Audio.Apagada
		case ingesta.TipoDocumento:
			t.Apagado = p.Documento.Apagada
		case ingesta.TipoSticker:
			t.Apagado = p.Sticker.Apagada
		}
		e.Tipos = append(e.Tipos, t)
	}
	b, _ := json.Marshal(e)
	return string(b), nil
}

// ArchivosDuplicadosAhorrados devuelve (JSON {bytes, archivos}) lo que se ahorró por no guardar copias del mismo archivo.
func ArchivosDuplicadosAhorrados(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	b, n := a.AhorradoPorDuplicados(botID)
	return js(map[string]any{"bytes": b, "archivos": n}, nil)
}

// VideosSinPortada devuelve (JSON) hasta `limite` videos descargados que todavía no tienen portada: [{chat, id, ruta, bytes}].
func VideosSinPortada(botID int64, limite int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.VideosSinPortada(botID, limite))
}

// GuardarPortada guarda cifrada la portada (JPEG) de un video descargado.
func GuardarPortada(botID int64, chat, id string, jpeg []byte, ancho, alto int) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	if len(jpeg) < 4 || jpeg[0] != 0xFF || jpeg[1] != 0xD8 {
		return errors.New("la portada no es un JPEG")
	}
	return a.GuardarPortada(botID, chat, id, jpeg)
}
