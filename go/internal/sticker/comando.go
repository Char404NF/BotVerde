package sticker

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
)

// Servicio es el comando !sticker: convierte una imagen (la que se cita, o la que trae el comando de pie de foto) en un sticker y la
// manda como respuesta. Solo responde; no escribe por su cuenta.
type Servicio struct {
	Alm   *almacen.Almacen
	BotID int64
	Ahora func() time.Time
	// Asegurar se asegura de que la imagen esté descargada (la pide si hace falta y espera un rato). nil = no intenta descargar.
	Asegurar func(chat, id string) error

	mu    sync.Mutex
	horas []time.Time
}

// Nuevo crea el servicio.
func Nuevo(alm *almacen.Almacen, botID int64) *Servicio {
	return &Servicio{Alm: alm, BotID: botID, Ahora: time.Now}
}

func (s *Servicio) ahora() time.Time {
	if s.Ahora != nil {
		return s.Ahora()
	}
	return time.Now()
}

// toma anota un sticker más en la última hora; false si ya se llegó al tope por hora (ajuste `sticker_tope_hora`, 0 = sin tope).
func (s *Servicio) toma() bool {
	tope := s.Alm.AjusteBotInt(s.BotID, "sticker_tope_hora")
	s.mu.Lock()
	defer s.mu.Unlock()
	ahora := s.ahora()
	n := s.horas[:0]
	for _, t := range s.horas {
		if ahora.Sub(t) < time.Hour {
			n = append(n, t)
		}
	}
	s.horas = n
	if tope > 0 && len(s.horas) >= tope {
		return false
	}
	s.horas = append(s.horas, ahora)
	return true
}

// Hacer devuelve el WebP del sticker de la imagen guardada (chat, id).
func (s *Servicio) Hacer(chat, id string) ([]byte, error) {
	m, err := s.Alm.ObtenerMensaje(s.BotID, chat, id)
	if err != nil || m == nil {
		return nil, errors.New("no encontré esa imagen")
	}
	switch {
	case m.Borrado || m.EliminadoEn != 0:
		return nil, errors.New("esa imagen ya fue eliminada")
	case m.Tipo == almacen.TipoSticker:
		return nil, errors.New("eso ya es un sticker")
	case m.Tipo != almacen.TipoImagen:
		return nil, errors.New("eso no es una imagen: responde a una foto con el comando, o manda la foto con el comando de pie de foto")
	}
	if m.ArchivoEstado != almacen.EstadoDescargado || m.ArchivoRuta == "" {
		if s.Asegurar == nil {
			return nil, errors.New("esa imagen todavía no se descargó, intenta en un momento")
		}
		if err := s.Asegurar(chat, id); err != nil {
			return nil, errors.New("no pude descargar esa imagen, intenta de nuevo en un momento")
		}
		if m, err = s.Alm.ObtenerMensaje(s.BotID, chat, id); err != nil || m == nil || m.ArchivoRuta == "" {
			return nil, errors.New("no pude descargar esa imagen, intenta de nuevo en un momento")
		}
	}
	r, err := s.Alm.AbrirDescifrado(m.ArchivoRuta)
	if err != nil {
		return nil, errors.New("no pude abrir esa imagen")
	}
	defer r.Close()
	return Convertir(io.Reader(r))
}

// RegistrarComandos agrega !sticker al despachador.
func (s *Servicio) RegistrarComandos(d *comandos.Despachador) {
	_ = d.Registrar(comandos.Comando{Nombre: "sticker", Alias: []string{"stiker"}, Resumen: "Convertir una imagen en sticker (responde a la foto, o mándala con el comando de pie)",
		Uso: "sticker", Casilla: "util.sticker",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			id := cx.CitaID
			if id == "" {
				id = cx.MensajeID
			}
			if id == "" {
				return "", errors.New("responde a una foto con el comando, o mándala con el comando de pie de foto")
			}
			if !s.toma() {
				return "", errors.New("ya hice muchos stickers en esta hora; intenta más tarde")
			}
			webp, err := s.Hacer(cx.Chat, id)
			if err != nil {
				return "", err
			}
			ref, _, err := s.guardar(webp)
			if err != nil {
				return "", errors.New("no pude preparar el sticker")
			}
			cx.Archivo = &comandos.Archivo{ID: ref, Tipo: almacen.TipoSticker, Mime: "image/webp", Nombre: "sticker.webp"}
			return "", nil
		}})
}

func (s *Servicio) guardar(webp []byte) (string, int64, error) {
	return s.Alm.GuardarAdjuntoMenu(s.BotID, bytes.NewReader(webp))
}
