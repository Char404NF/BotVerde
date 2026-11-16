// Package avatar decide cuándo una respuesta del bot lleva la imagen de su avatar (con el texto de pie de foto) y cuál imagen es:
// una de las 8 caras de fábrica según el momento, o una imagen propia que sube el dueño y que reemplaza a todas.
package avatar

import (
	"bytes"
	"strings"

	"botchat/internal/almacen"
	"botchat/internal/estilo"
)

// Momentos en que puede salir el avatar.
const (
	Menu       = "menu"       // el menú principal
	Ayuda      = "ayuda"      // !ayuda
	Bienvenida = "bienvenida" // la bienvenida a quien entra o escribe por primera vez
	Aviso      = "aviso"      // advertencias, silencios y otros avisos privados
	Error      = "error"      // algo salió mal
)

// Momentos son todos los momentos, con su nombre en español y la cara de fábrica que les toca.
var Momentos = []struct{ ID, Nombre, Cara string }{
	{Menu, "Menú principal", estilo.CaraFeliz},
	{Ayuda, "Ayuda (!ayuda)", estilo.CaraContento},
	{Bienvenida, "Bienvenida", estilo.CaraFeliz},
	{Aviso, "Avisos y advertencias en privado", estilo.CaraEnojado},
	{Error, "Cuando algo sale mal", estilo.CaraTriste},
}

// EnGrupos son los momentos que también salen en grupos (para no llenarlos de imágenes); los demás solo en chats privados.
var EnGrupos = map[string]bool{Menu: true, Bienvenida: true}

// MaxPie es lo más largo que puede ser el pie de foto para mandar el avatar (letras); si el texto es más largo, sale sin imagen.
const MaxPie = 900

// Ajustes (en bot_ajustes).
const (
	AjActivo = "avatar_activo" // "1" = el avatar sale en los momentos elegidos
	AjEn     = "avatar_en"     // momentos elegidos, separados por coma
	AjPropio = "avatar_propio" // id del adjunto con la imagen propia ("" = las caras de fábrica)
)

// PorDefecto son los momentos encendidos de fábrica.
const PorDefecto = "menu,ayuda,bienvenida,aviso"

// Servicio resuelve el avatar de un bot.
type Servicio struct {
	Alm   *almacen.Almacen
	BotID int64
}

// Nuevo crea el servicio.
func Nuevo(alm *almacen.Almacen, botID int64) *Servicio { return &Servicio{Alm: alm, BotID: botID} }

// Activo dice si el avatar está encendido en general.
func (s *Servicio) Activo() bool { return s.Alm.AjusteBot(s.BotID, AjActivo) != "0" }

// Elegidos devuelve los momentos encendidos.
func (s *Servicio) Elegidos() map[string]bool {
	v := s.Alm.AjusteBot(s.BotID, AjEn)
	if v == "" && s.Alm.AjusteBot(s.BotID, AjEn+"_fijado") != "1" {
		v = PorDefecto
	}
	m := map[string]bool{}
	for _, x := range strings.Split(v, ",") {
		if x = strings.TrimSpace(x); x != "" {
			m[x] = true
		}
	}
	return m
}

// Corresponde dice si en ese momento, en ese chat, con ese texto de pie de foto, debe salir el avatar.
func (s *Servicio) Corresponde(momento, chat, pie string) bool {
	if momento == "" || !s.Activo() || !s.Elegidos()[momento] {
		return false
	}
	if strings.HasSuffix(chat, "@g.us") && !EnGrupos[momento] {
		return false
	}
	return len([]rune(strings.TrimSpace(pie))) <= MaxPie
}

// Imagen devuelve la imagen que toca en ese momento: la propia si hay (y existe todavía) o la cara de fábrica. Ref es el id del adjunto.
func (s *Servicio) Imagen(momento string) (ref, mime, nombre string, ok bool) {
	if id := s.Alm.AjusteBot(s.BotID, AjPropio); id != "" && s.Alm.ExisteAdjunto(s.BotID, id) {
		return id, s.Alm.AjusteBot(s.BotID, AjPropio+"_mime"), "avatar", true
	}
	return s.Cara(CaraDe(momento))
}

// CaraDe es la cara de fábrica de un momento.
func CaraDe(momento string) string {
	for _, m := range Momentos {
		if m.ID == momento {
			return m.Cara
		}
	}
	return estilo.CaraFeliz
}

// Cara guarda (una sola vez) la cara de fábrica como adjunto y devuelve su id.
func (s *Servicio) Cara(cara string) (ref, mime, nombre string, ok bool) {
	clave := "avatar_cara_" + cara
	if id := s.Alm.AjusteBot(s.BotID, clave); id != "" && s.Alm.ExisteAdjunto(s.BotID, id) {
		return id, "image/png", "avatar_" + cara + ".png", true
	}
	png := estilo.Cara(cara)
	if png == nil {
		return "", "", "", false
	}
	id, _, err := s.Alm.GuardarAdjuntoMenu(s.BotID, bytes.NewReader(png))
	if err != nil {
		return "", "", "", false
	}
	_ = s.Alm.FijarAjusteBot(s.BotID, clave, id)
	return id, "image/png", "avatar_" + cara + ".png", true
}

// Configurar enciende o apaga el avatar y fija los momentos (los desconocidos se ignoran).
func (s *Servicio) Configurar(activo bool, momentos []string) error {
	var buenos []string
	for _, m := range momentos {
		for _, k := range Momentos {
			if k.ID == m {
				buenos = append(buenos, m)
			}
		}
	}
	v := "1"
	if !activo {
		v = "0"
	}
	if err := s.Alm.FijarAjusteBot(s.BotID, AjActivo, v); err != nil {
		return err
	}
	_ = s.Alm.FijarAjusteBot(s.BotID, AjEn+"_fijado", "1")
	return s.Alm.FijarAjusteBot(s.BotID, AjEn, strings.Join(buenos, ","))
}

// FijarPropio usa una imagen ya guardada como adjunto como avatar propio ("" = volver a las caras de fábrica). Borra la anterior.
func (s *Servicio) FijarPropio(id, mime string) error {
	if viejo := s.Alm.AjusteBot(s.BotID, AjPropio); viejo != "" && viejo != id {
		s.Alm.BorrarAdjuntoMenu(s.BotID, viejo)
	}
	_ = s.Alm.FijarAjusteBot(s.BotID, AjPropio+"_mime", mime)
	return s.Alm.FijarAjusteBot(s.BotID, AjPropio, id)
}
