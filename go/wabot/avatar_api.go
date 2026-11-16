package wabot

import (
	"context"
	"errors"
	"strings"

	"botchat/internal/almacen"
	"botchat/internal/avatar"
	"botchat/internal/envio"
	"botchat/internal/motor"
)

// conAvatar le suma a una respuesta la imagen del avatar cuando toca (momento, chat, ajustes y largo del texto). Si no toca, o algo falla con
// la imagen, la devuelve igual: el avatar nunca impide que salga la respuesta.
//
//   - Texto, ayuda y avisos: la imagen va con el texto de pie de foto (un solo mensaje).
//   - Menú de texto o de grupo: la imagen va con el menú numerado de pie de foto (un solo mensaje).
//   - Menú con botones o encuesta (solo en privado): primero sale la imagen con el título y debajo los botones, que no admiten imagen.
func (e enviadorMotor) conAvatar(ctx context.Context, chat string, sal motor.Salida, citaID, citaAutorWA string, men []string) (motor.Salida, error) {
	momento := sal.Avatar
	sal.Avatar = ""
	if momento == "" || alm == nil || sal.Medio != nil || sal.Ubicacion != nil || sal.Contacto != nil || sal.Auto != nil {
		return sal, nil
	}
	av := avatar.Nuevo(alm, e.s.botID)
	if !av.Corresponde(momento, chat, "") {
		return sal, nil
	}
	ref, mime, nombre, ok := av.Imagen(momento)
	if !ok {
		return sal, nil
	}
	medio := &motor.Medio{ID: ref, Tipo: almacen.TipoImagen, Mime: mime, Nombre: nombre}
	if len([]rune(strings.TrimSpace(sal.Texto))) > avatar.MaxPie && sal.Menu == nil {
		// texto largo (por ejemplo la lista de comandos): la imagen sale con el título de la tarjeta y debajo el resto
		titulo, resto, ok := partirTarjeta(sal.Texto)
		if !ok {
			return sal, nil
		}
		ruta, err := e.s.copiaAdjunto(ref)
		if err != nil {
			return sal, nil
		}
		if _, err := e.s.envio.EnviarMedioAutoCita(ctx, chat, adjuntoEnvio(ruta, medio, titulo), citaID, citaAutorWA, men); err != nil {
			if errors.Is(err, envio.ErrSinConexion) {
				return sal, err
			}
			return sal, nil
		}
		sal.Texto = resto
		return sal, nil
	}
	if len([]rune(strings.TrimSpace(sal.Texto))) > avatar.MaxPie {
		return sal, nil
	}
	if sal.Menu != nil && e.s.lab != nil && !strings.HasSuffix(chat, "@g.us") && e.s.lab.ModoDe(sal.Menu.Formato) != "texto" {
		// botones o encuesta: la imagen sale antes, con el título; el menú sigue debajo sin repetirlo
		ruta, err := e.s.copiaAdjunto(ref)
		if err != nil {
			return sal, nil
		}
		titulo := strings.TrimSpace(sal.Menu.Titulo)
		if len([]rune(titulo)) > avatar.MaxPie {
			return sal, nil
		}
		if _, err := e.s.envio.EnviarMedioAutoCita(ctx, chat, adjuntoEnvio(ruta, medio, titulo), citaID, citaAutorWA, men); err != nil {
			if errors.Is(err, envio.ErrSinConexion) {
				return sal, err
			}
			return sal, nil
		}
		m := *sal.Menu
		m.Titulo = "Elige una opción 👇"
		sal.Menu = &m
		return sal, nil
	}
	sal.Medio = medio
	sal.Menu = nil // sale como imagen con el texto del menú de pie de foto
	return sal, nil
}

// partirTarjeta separa el título de una tarjeta («🤖 *Título*» y la línea ━━━) del resto. ok=false si el texto no es una tarjeta.
func partirTarjeta(t string) (titulo, resto string, ok bool) {
	l := strings.SplitN(strings.TrimSpace(t), "\n", 3)
	if len(l) < 3 || !strings.HasPrefix(l[1], "━") || len([]rune(l[0])) > 120 {
		return "", "", false
	}
	return l[0], strings.TrimSpace(l[2]), true
}

// ---------- API de la app (Bot → Avatar) ----------

type avatarEstado struct {
	Activo   bool            `json:"activo"`
	Propio   bool            `json:"propio"`
	PropioID string          `json:"propioId,omitempty"`
	Momentos []avatarMomento `json:"momentos"`
	Caras    []string        `json:"caras"`
}

type avatarMomento struct {
	ID      string `json:"id"`
	Nombre  string `json:"nombre"`
	Cara    string `json:"cara"`
	Activo  bool   `json:"activo"`
	Grupos  bool   `json:"grupos"`
	Mostrar bool   `json:"mostrar"`
}

// AvatarEstado devuelve (JSON) cómo está el avatar: encendido, si hay imagen propia y en qué momentos sale.
func AvatarEstado(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	av := avatar.Nuevo(a, botID)
	el := av.Elegidos()
	e := avatarEstado{Activo: av.Activo(), Caras: caras()}
	if id := a.AjusteBot(botID, avatar.AjPropio); id != "" && a.ExisteAdjunto(botID, id) {
		e.Propio, e.PropioID = true, id
	}
	for _, m := range avatar.Momentos {
		e.Momentos = append(e.Momentos, avatarMomento{ID: m.ID, Nombre: m.Nombre, Cara: m.Cara, Activo: el[m.ID], Grupos: avatar.EnGrupos[m.ID], Mostrar: true})
	}
	return js(e, nil)
}

func caras() []string {
	return []string{"feliz", "contento", "guino", "pensando", "sorprendido", "triste", "enojado", "dormido"}
}

// AvatarConfigurar enciende o apaga el avatar y fija los momentos en que sale (ids separados por coma).
func AvatarConfigurar(botID int64, activo bool, momentos string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	var ms []string
	for _, m := range strings.Split(momentos, ",") {
		if m = strings.TrimSpace(m); m != "" {
			ms = append(ms, m)
		}
	}
	if err := avatar.Nuevo(a, botID).Configurar(activo, ms); err != nil {
		return err
	}
	a.RegistrarAccion(botID, almacen.PersonaDueno, "avatar.configurar", "", "", "ok", "")
	return nil
}

// AvatarPropio usa como avatar una imagen ya subida como adjunto (id); con id vacío vuelve a las caras de fábrica.
func AvatarPropio(botID int64, id, mime string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	if id != "" {
		if strings.ContainsAny(id, "/\\.") || !a.ExisteAdjunto(botID, id) {
			return errors.New("Esa imagen ya no está guardada; súbela otra vez.")
		}
		if !strings.HasPrefix(mime, "image/") {
			return errors.New("El avatar tiene que ser una imagen.")
		}
	}
	if err := avatar.Nuevo(a, botID).FijarPropio(id, mime); err != nil {
		return err
	}
	a.RegistrarAccion(botID, almacen.PersonaDueno, "avatar.propio", "", "", "ok", "")
	return nil
}
