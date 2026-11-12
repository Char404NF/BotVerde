package silencio

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"botchat/internal/almacen"
)

// Textos editables de los avisos. Se pueden usar {grupo}, {n} (mensajes borrados hoy), {tope}, {persona} y {duracion}.
const (
	TxtAviso   = "aviso"   // cada N mensajes borrados
	TxtInicio  = "inicio"  // al silenciarla
	TxtFin     = "fin"     // cuando termina el silencio
	TxtNoAdmin = "noadmin" // alguien silenciado escribe donde no soy administrador
	TxtTope    = "tope"    // al llegar al tope con la sanción «silencio más largo»
)

// TextosDeFabrica son los textos de siempre.
var TextosDeFabrica = map[string]string{
	TxtAviso:   "Estás silenciado en {grupo}: tus mensajes se borran. Hoy van {n} de {tope}.",
	TxtInicio:  "Te silencié en {grupo} {duracion}. Tus mensajes ahí se borran.",
	TxtFin:     "Tu silencio terminó. Ya puedes escribir de nuevo.",
	TxtNoAdmin: "Estás silenciado; tus mensajes no se leerán",
	TxtTope:    "Por insistir, tu silencio se alargó.",
}

// EsTexto dice si el id es uno de los textos editables.
func EsTexto(id string) bool { _, ok := TextosDeFabrica[id]; return ok }

// Texto devuelve el texto vigente (el editado, o el de fábrica).
func (s *Servicio) Texto(id string) string {
	if t, ok := s.Alm.TextoSilencio(s.BotID, id); ok && strings.TrimSpace(t) != "" {
		return t
	}
	return TextosDeFabrica[id]
}

// FijarTexto guarda un texto editado ("" vuelve al de fábrica).
func (s *Servicio) FijarTexto(id, texto string) error {
	if !EsTexto(id) {
		return errors.New("ese texto no existe")
	}
	return s.Alm.FijarTextoSilencio(s.BotID, id, texto)
}

// Textos devuelve todos los textos vigentes y los de fábrica: {id: {actual, defecto}}.
func (s *Servicio) Textos() map[string]map[string]string {
	out := map[string]map[string]string{}
	for id, def := range TextosDeFabrica {
		out[id] = map[string]string{"actual": s.Texto(id), "defecto": def}
	}
	return out
}

// TextoDuracion dice cuánto dura un silencio en palabras cortas.
func TextoDuracion(hasta int64, ahora time.Time, loc *time.Location) string {
	if hasta == 0 {
		return "sin fecha de fin"
	}
	d := time.Unix(hasta, 0).Sub(ahora)
	switch {
	case d < time.Hour:
		m := int(d.Minutes())
		if m < 1 {
			m = 1
		}
		return fmt.Sprintf("por %d min", m)
	case d < 48*time.Hour:
		h := int((d + 30*time.Minute).Hours())
		if h == 1 {
			return "por 1 hora"
		}
		return fmt.Sprintf("por %d horas", h)
	}
	return "hasta el " + time.Unix(hasta, 0).In(loc).Format("02/01")
}

// armaTexto escoge el texto (el propio de ese silencio para aviso e inicio; si no, el editado o el de fábrica) y llena los huecos.
func (s *Servicio) armaTexto(id string, sil *almacen.SilencioG, persona, grupo string, n int) string {
	t := ""
	if sil != nil && sil.Texto != "" && (id == TxtAviso || id == TxtInicio) {
		t = sil.Texto
	} else {
		t = s.Texto(id)
	}
	tope := s.entero("sil_tope")
	if tope <= 0 {
		t = strings.ReplaceAll(t, " de {tope}", "")
	}
	ng := "los grupos"
	if grupo != "" {
		ng = s.nombreGrupo(grupo)
	} else if sil != nil && sil.Alcance != almacen.SilTodos && len(sil.Grupos) == 1 {
		ng = s.nombreGrupo(sil.Grupos[0])
	}
	dur := "sin fecha de fin"
	if sil != nil {
		dur = TextoDuracion(sil.HastaTS, s.ahora(), s.Alm.Zona())
	}
	return strings.NewReplacer("{grupo}", ng, "{n}", fmt.Sprint(n), "{tope}", fmt.Sprint(tope), "{persona}", s.nombre(persona), "{duracion}", dur).Replace(t)
}
