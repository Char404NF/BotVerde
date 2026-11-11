package wabot

import (
	"errors"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/sticker"
	"botchat/internal/utilidades"
)

// ---------- armado dentro del bot ----------

// armarUtilidades registra los comandos del bloque F: notas y pendientes (!anotar, !notas, !tarea, !tareas, !hecho…) y !sticker.
func (s *estado) armarUtilidades() {
	if s.cmd == nil {
		return
	}
	s.util = utilidades.Nuevo(alm, s.botID)
	s.util.RegistrarComandos(s.cmd)
	st := sticker.Nuevo(alm, s.botID)
	st.Asegurar = s.asegurarImagen
	st.RegistrarComandos(s.cmd)
}

// asegurarImagen pide la descarga de una imagen que todavía no está en el teléfono y espera (hasta 25 s) a que llegue. Corre en el
// trabajador del motor, nunca en el manejador de eventos.
func (s *estado) asegurarImagen(chat, id string) error {
	listo := func() (bool, error) {
		m, err := alm.ObtenerMensaje(s.botID, chat, id)
		if err != nil || m == nil {
			return false, errors.New("no está")
		}
		switch m.ArchivoEstado {
		case almacen.EstadoDescargado:
			return m.ArchivoRuta != "", nil
		case almacen.EstadoBorrado, almacen.EstadoOmitido:
			return false, errors.New("no disponible")
		}
		return false, nil
	}
	if ok, err := listo(); ok || err != nil {
		if ok {
			return nil
		}
		if m, _ := alm.ObtenerMensaje(s.botID, chat, id); m == nil || m.ArchivoEstado != almacen.EstadoOmitido {
			return err
		}
	}
	if s.descargas == nil {
		return errors.New("sin descargas")
	}
	if err := s.descargas.Manual(chat, id); err != nil {
		return err
	}
	limite := time.NewTimer(25 * time.Second)
	defer limite.Stop()
	tic := time.NewTicker(400 * time.Millisecond)
	defer tic.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-limite.C:
			return errors.New("tardó demasiado")
		case <-tic.C:
			ok, err := listo()
			if ok {
				return nil
			}
			if m, _ := alm.ObtenerMensaje(s.botID, chat, id); m != nil && m.ArchivoEstado == almacen.EstadoFallido {
				return errors.New("falló la descarga")
			} else if err != nil && m == nil {
				return err
			}
		}
	}
}

// ---------- API para la app (las notas y pendientes del dueño) ----------

func utilTipoValido(tipo string) error {
	if tipo != almacen.UtilNota && tipo != almacen.UtilPendiente {
		return errors.New("tipo desconocido: nota o pendiente")
	}
	return nil
}

// ListarUtilidades devuelve (JSON) las notas ("nota") o los pendientes ("pendiente") del dueño, del más viejo al más nuevo:
// [{id, tipo, texto, hecho, ts, hechoTs}].
func ListarUtilidades(botID int64, tipo string) (string, error) {
	if err := utilTipoValido(tipo); err != nil {
		return "", err
	}
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ListarItemsUtil(botID, almacen.PersonaDueno, tipo, true), nil)
}

// AgregarUtilidad guarda una nota o un pendiente del dueño (cifrado). Devuelve su id.
func AgregarUtilidad(botID int64, tipo, texto string) (int64, error) {
	if err := utilTipoValido(tipo); err != nil {
		return 0, err
	}
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	it, err := a.AgregarItemUtil(botID, almacen.PersonaDueno, tipo, texto, time.Now())
	if err != nil {
		return 0, err
	}
	return it.ID, nil
}

// MarcarUtilidad marca un pendiente como hecho (o lo deja por hacer).
func MarcarUtilidad(botID, id int64, hecho bool) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	ok, err := a.MarcarItemUtil(botID, almacen.PersonaDueno, id, hecho, time.Now())
	if err == nil && !ok {
		err = errors.New("ese pendiente ya no existe")
	}
	return err
}

// BorrarUtilidad quita una nota o un pendiente del dueño.
func BorrarUtilidad(botID, id int64) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	_, err = a.BorrarItemUtil(botID, almacen.PersonaDueno, id)
	return err
}

// LimpiarUtilidadesHechas borra los pendientes del dueño que ya están hechos. Devuelve cuántos.
func LimpiarUtilidadesHechas(botID int64) (int, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	return a.BorrarHechosUtil(botID, almacen.PersonaDueno)
}
