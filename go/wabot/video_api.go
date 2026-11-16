package wabot

import (
	"context"
	"errors"
	"sync"

	"botchat/internal/envio"
	"botchat/internal/modulos"
	"botchat/internal/motor"
)

// MotorVideo es el motor de descargas que pone la app (Kotlin + Python con yt-dlp). Todo viaja como texto JSON.
type MotorVideo interface {
	Instalado() bool
	Quitar() error
	Version() (string, error)
	Info(url string) (string, error)
	Descargar(url, carpeta, modo string, altura, abr int) (string, error)
	Convertir(ruta, formato string) (string, error)
	Partir(ruta string, n int, tope int64, carpeta, base string) (string, error)
	Zip(rutasJSON, salida string) error
	Instalar(ruta string) (string, error)
	Carpeta() string
	Borrar(ruta string) error
}

var (
	motorVideoMu sync.Mutex
	motorVideo   MotorVideo
)

// RegistrarMotorVideo lo llama la app al arrancar: deja listo el motor de descargas (no arranca Python hasta que alguien pida una).
func RegistrarMotorVideo(m MotorVideo) {
	motorVideoMu.Lock()
	motorVideo = m
	motorVideoMu.Unlock()
}

func motorVideoActual() modulos.MotorVideo {
	motorVideoMu.Lock()
	defer motorVideoMu.Unlock()
	if motorVideo == nil {
		return nil
	}
	return motorVideo
}

// EnviarMenu manda las opciones de un diálogo como botones o encuesta si el Laboratorio lo permite; si no, el texto numerado de siempre.
func (a accionesModulos) EnviarMenu(ctx context.Context, chat, persona, titulo, textoNumerado string, opciones []string, citaID string) error {
	s := a.s
	if s.lab != nil {
		ops := make([]motor.OpcionMenu, len(opciones))
		for i, o := range opciones {
			ops[i] = motor.OpcionMenu{Num: i + 1, Texto: o}
		}
		env, ok, err := s.lab.EnviarMenu(ctx, chat, persona, &motor.MenuLab{Persona: persona, Titulo: titulo, Opciones: ops}, textoNumerado, citaID, "")
		if ok && err == nil {
			if env.Respaldo {
				_, _ = s.envio.EnviarAutoCita(ctx, chat, textoNumerado, "", "", nil)
			}
			return nil
		}
		if err != nil && errors.Is(err, envio.ErrSinConexion) {
			return err
		}
	}
	_, err := s.envio.EnviarAutoCita(ctx, chat, textoNumerado, citaID, "", nil)
	return err
}

// EnviarEncuesta manda una encuesta a un grupo y devuelve su id y su secreto.
func (a accionesModulos) EnviarEncuesta(ctx context.Context, chat, titulo string, opciones []string, multiple bool) (string, []byte, error) {
	r, sec, err := a.s.envio.EnviarEncuestaSecreta(ctx, chat, titulo, opciones, multiple)
	if err != nil {
		return "", nil, err
	}
	if r.Estado != "enviado" {
		return "", nil, errNoEnviado
	}
	return r.ID, sec, nil
}

// EnviarTextoID manda un texto con menciones y devuelve el id del mensaje.
func (a accionesModulos) EnviarTextoID(ctx context.Context, chat, texto string, menciones []string) (string, error) {
	r, err := a.s.envio.EnviarAuto(ctx, chat, texto, "", menciones)
	if err != nil {
		return "", err
	}
	if r.Estado != "enviado" {
		return "", errNoEnviado
	}
	return r.ID, nil
}

// FijarMensaje fija o suelta un mensaje para todos.
func (a accionesModulos) FijarMensaje(ctx context.Context, chat, id string, dias int, fijar bool) error {
	return a.s.envio.FijarMensaje(ctx, chat, id, dias*86400, fijar)
}

// BorrarMensaje borra para todos un mensaje del bot.
func (a accionesModulos) BorrarMensaje(ctx context.Context, chat, id string) error {
	return a.s.envio.BorrarParaTodos(ctx, chat, id)
}

// ---------- API para la pantalla de descargas ----------

// VideoEstado devuelve (JSON) la versión de yt-dlp, si hay una nueva y qué se está descargando.
func VideoEstado(botID int64) (string, error) {
	g, err := gestorDe(botID)
	if err != nil {
		return "", err
	}
	return js(g.VideoEstado(), nil)
}

// VideoLista devuelve (JSON) el historial de descargas, lo más nuevo primero. antes > 0 pide la página siguiente.
func VideoLista(botID int64, limite int, antes int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	l, err := a.ListarDescargasVideo(botID, limite, antes)
	if err != nil {
		return "", err
	}
	if l == nil {
		l = nil
	}
	return js(l, nil)
}

// VideoMiniatura devuelve la miniatura (JPEG) de una descarga, o nada.
func VideoMiniatura(botID, id int64) ([]byte, error) {
	a, err := almacenAbierto()
	if err != nil {
		return nil, err
	}
	return a.MiniaturaDescargaVideo(botID, id), nil
}

// VideoBorrar quita una descarga del historial (id 0 = todas las terminadas).
func VideoBorrar(botID, id int64) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	accionDueno(botID, "video.borrar_historial", "", "")
	return a.BorrarDescargaVideo(botID, id)
}

// VideoActualizar revisa e instala la última versión de yt-dlp. Devuelve una frase para mostrar.
func VideoActualizar(botID int64) (string, error) {
	g, err := gestorDe(botID)
	if err != nil {
		return "", err
	}
	accionDueno(botID, "video.actualizar", "", "")
	return g.VideoActualizar(true)
}

// VideoQuitarMotor borra el motor descargado (yt-dlp) y apaga el módulo; queda bloqueado hasta bajarlo otra vez.
func VideoQuitarMotor(botID int64) error {
	g, err := gestorDe(botID)
	if err != nil {
		return err
	}
	accionDueno(botID, "video.quitar_motor", "", "")
	return g.VideoQuitarMotor()
}

// VideoRepetir vuelve a pedir una descarga del historial con las mismas opciones (el archivo ya no existe: se baja otra vez).
func VideoRepetir(botID, id int64) (string, error) {
	g, err := gestorDe(botID)
	if err != nil {
		return "", err
	}
	accionDueno(botID, "video.repetir", "", "")
	return g.VideoRepetir(id)
}
