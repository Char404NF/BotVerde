package modulos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
	"unicode"

	"botchat/internal/almacen"
	"botchat/internal/envio"
	"botchat/internal/identidad"
)

// Resultados de agregar a una persona a un grupo.
const (
	AgregOK         = "ok"         // quedó dentro
	AgregYaEsta     = "ya_esta"    // ya era miembro
	AgregPrivacidad = "privacidad" // su privacidad no deja que la agreguen (WhatsApp responde sin avisar): hay que invitarla
)

// Acciones son las cosas de grupo que algunos módulos necesitan. En la app las implementa el servicio de grupos; las pruebas usan uno falso.
// Cada una revisa por su cuenta que el bot sea administrador cuando hace falta y devuelve el error en español.
type Acciones interface {
	EsAdmin(ctx context.Context, chat string) (bool, error)
	Expulsar(ctx context.Context, chat, persona string) error
	Enlace(ctx context.Context, chat string) (string, error)
	Agregar(ctx context.Context, chat, persona string) (string, error)
}

// ---------- ayudas comunes de los módulos H ----------

// usuarioDe es la parte del JID antes de la arroba (lo que se escribe después del "@" de una mención).
func usuarioDe(j string) string {
	if i := strings.IndexByte(j, '@'); i >= 0 {
		j = j[:i]
	}
	if i := strings.IndexByte(j, ':'); i >= 0 {
		j = j[:i]
	}
	return j
}

// sustituir reemplaza {clave} por su valor (sin tocar las llaves que no conoce).
func sustituir(texto string, vars map[string]string) string {
	for k, v := range vars {
		texto = strings.ReplaceAll(texto, "{"+k+"}", v)
	}
	return texto
}

// nombreDe da el nombre público o guardado de una persona ("" si no se sabe).
func (g *Gestor) nombreDe(persona string) string {
	if c, _ := g.Alm.ObtenerChat(g.BotID, persona); c != nil {
		if c.Mostrado != "" && c.Mostrado != c.TelMostrado {
			return c.Mostrado
		}
		if c.Publico != "" {
			return c.Publico
		}
	}
	return ""
}

// nombreOPalabra da el nombre o, si no se sabe, una palabra amable.
func (g *Gestor) nombreOPalabra(persona, nombre string) string {
	if n := strings.TrimSpace(nombre); n != "" {
		return n
	}
	if n := g.nombreDe(persona); n != "" {
		return n
	}
	return "amigo"
}

func (g *Gestor) hayChat(chat string) bool {
	c, _ := g.Alm.ObtenerChat(g.BotID, chat)
	return c != nil
}

// Hora actual en la zona del teléfono.
func (g *Gestor) Hora() time.Time { return g.ahora().In(g.zona()) }

// ParseFechaHora lee "dd/mm/aaaa HH:MM" (o "dd/mm HH:MM", que toma el año en curso o el siguiente si ya pasó) en la zona del teléfono.
func (g *Gestor) ParseFechaHora(s string) (time.Time, error) {
	s = strings.Join(strings.Fields(s), " ")
	z := g.zona()
	if t, err := time.ParseInLocation("02/01/2006 15:04", s, z); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2/1/2006 15:04", s, z); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("02/01 15:04", s, z); err == nil {
		ahora := g.Hora()
		t = time.Date(ahora.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, z)
		if t.Before(ahora) {
			t = t.AddDate(1, 0, 0)
		}
		return t, nil
	}
	return time.Time{}, errors.New("la fecha debe verse como 25/12/2026 18:30 (día/mes/año hora:minutos)")
}

// FechaTexto escribe una marca de tiempo como "25/12/2026 18:30" en la zona del teléfono.
func (g *Gestor) FechaTexto(ts int64) string {
	if ts <= 0 {
		return "sin fecha"
	}
	return time.Unix(ts, 0).In(g.zona()).Format("02/01/2006 15:04")
}

// AdjRef es un archivo ya guardado (cifrado) en el teléfono que va en un mensaje.
type AdjRef struct {
	ID     string `json:"id"`
	Tipo   string `json:"tipo"`
	Mime   string `json:"mime"`
	Nombre string `json:"nombre"`
	Bytes  int64  `json:"bytes"`
}

// adjuntosDe lee y valida los archivos que mandó la app (hasta 3; solo ids bien formados que existan).
func (g *Gestor) adjuntosDe(a Args, clave string) ([]AdjRef, error) {
	var out []AdjRef
	for _, m := range a.Lista(clave) {
		var r AdjRef
		b, _ := json.Marshal(m)
		if json.Unmarshal(b, &r) != nil || !almacen.IDAdjuntoValido(r.ID) {
			return nil, errors.New("hay un archivo que no es válido")
		}
		if !g.Alm.ExisteAdjunto(g.BotID, r.ID) {
			return nil, errors.New("uno de los archivos ya no está en el teléfono: súbelo otra vez")
		}
		switch r.Tipo {
		case almacen.TipoImagen, almacen.TipoVideo, almacen.TipoAudio, almacen.TipoDocumento:
		default:
			return nil, errors.New("ese tipo de archivo no se puede mandar")
		}
		out = append(out, r)
	}
	if len(out) > 3 {
		return nil, errors.New("hasta 3 archivos por mensaje")
	}
	return out, nil
}

func adjEnvio(r AdjRef, pie string) envio.Adjunto {
	a := envio.Adjunto{Tipo: r.Tipo, Ruta: PrefijoAdj + r.ID, Mime: r.Mime, Nombre: r.Nombre, Pie: pie}
	if r.Tipo == almacen.TipoAudio {
		a.Pie = ""
		a.NotaVoz = strings.Contains(strings.ToLower(r.Mime), "ogg")
	}
	return a
}

// PrefijoAdj marca una ruta que no es un archivo en claro sino el id de un adjunto guardado cifrado; el enviador de la app lo
// descifra a una copia temporal al mandarlo.
const PrefijoAdj = "adj:"

// medioDe arma lo que lleva una iniciativa con archivos: el primero con el texto de pie y los demás detrás.
func medioDe(refs []AdjRef, texto string) (*envio.Adjunto, []envio.Adjunto) {
	if len(refs) == 0 {
		return nil, nil
	}
	p := adjEnvio(refs[0], strings.TrimSpace(texto))
	var extras []envio.Adjunto
	for _, r := range refs[1:] {
		extras = append(extras, adjEnvio(r, ""))
	}
	return &p, extras
}

// opcionesChats arma una lista de opciones con los chats más recientes (privados o grupos), por nombre.
func (g *Gestor) opcionesChats(grupos bool, limite int) []Opcion {
	cs, _ := g.Alm.ListarChats(g.BotID, grupos, "", limite, 0)
	out := make([]Opcion, 0, len(cs))
	for _, c := range cs {
		n := strings.TrimSpace(c.Nombre)
		if n == "" {
			n = c.Telefono
		}
		if n == "" {
			n = c.JID
		}
		out = append(out, Opcion{Valor: c.JID, Titulo: n})
	}
	return out
}

// nombreChat da un nombre legible para un chat guardado.
func (g *Gestor) nombreChat(chat string) string {
	if c, _ := g.Alm.ObtenerChat(g.BotID, chat); c != nil {
		if c.Mostrado != "" {
			return c.Mostrado
		}
	}
	return enmascarar(chat)
}

// enmascarar deja solo el final de un número o id para mostrarlo sin exponerlo entero.
func enmascarar(j string) string {
	u := usuarioDe(j)
	if len(u) <= 4 {
		return u
	}
	return "…" + u[len(u)-4:]
}

// solo letras y números, en minúsculas y sin acentos (para comparar respuestas y claves).
func llave(s string) string {
	s = identidad.Normalizar(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if unicode.IsSpace(r) {
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func ordenar(s []string) []string { sort.Strings(s); return s }

func errf(f string, a ...any) error { return fmt.Errorf(f, a...) }

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

func parseIP(s string) net.IP { return net.ParseIP(s) }

func valorTexto(a Args, k string) string {
	if s, ok := a[k].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}
