// Package avisos son los avisos para mí (bloque B): alertas por palabra clave, resumen diario y recordatorios. SIEMPRE llegan como
// notificación de Android (filas de la tabla avisos que la app muestra); nunca se manda un mensaje de WhatsApp. La programación de la
// hora la hace Android con AlarmManager: este paquete solo dice "qué toca ahora" (Disparar) y "cuándo es lo siguiente" (Siguiente),
// contando todo desde la base, sin servidor.
package avisos

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/identidad"
)

// Servicio es el conjunto de avisos de un bot.
type Servicio struct {
	Alm   *almacen.Almacen
	BotID int64
	Ahora func() time.Time
	Log   func(string)

	// Extras son otras fuentes de alarmas (por ejemplo el vencimiento de silencios de grupo): entran en Siguiente y Disparar.
	Extras []Fuente

	mu     sync.Mutex
	ultimo map[string]time.Time // chat -> última alerta por palabra (para no llenar de avisos)
}

// Fuente es algo que también necesita una alarma: dice cuándo toca lo siguiente y, al disparar, devuelve los avisos que creó.
type Fuente interface {
	Siguiente() int64
	Disparar() []int64
}

// Nuevo crea el servicio.
func Nuevo(alm *almacen.Almacen, botID int64) *Servicio {
	return &Servicio{Alm: alm, BotID: botID, Ahora: time.Now, ultimo: map[string]time.Time{}}
}

func (s *Servicio) ahora() time.Time {
	if s.Ahora != nil {
		return s.Ahora()
	}
	return time.Now()
}

func (s *Servicio) log(t string) {
	if s.Log != nil {
		s.Log(t)
	}
}

// Tipos de aviso que crea este paquete.
const (
	TipoPalabra      = "palabra"
	TipoRecordatorio = "recordatorio"
	TipoResumen      = "resumen"
)

// Mensaje es lo que se mira de un mensaje recibido.
type Mensaje struct {
	Chat    string
	Persona string
	Nombre  string // nombre público con el que escribe (si no hay otro)
	Texto   string
	EsGrupo bool
	EsMio   bool
	Antiguo bool // llegó en la sincronización o del historial: no se avisa
	Edicion bool
}

// separaPalabras deja solo letras y números separados por un espacio, sin acentos ni mayúsculas.
func separaPalabras(s string) string {
	s = identidad.Normalizar(s)
	var b strings.Builder
	b.WriteByte(' ')
	esp := true
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127 {
			b.WriteRune(r)
			esp = false
		} else if !esp {
			b.WriteByte(' ')
			esp = true
		}
	}
	if !esp {
		b.WriteByte(' ')
	}
	return b.String()
}

// coincide dice si la palabra o frase aparece completa en el texto.
func coincide(texto, palabra string) bool {
	p := strings.TrimSpace(separaPalabras(palabra))
	if p == "" {
		return false
	}
	return strings.Contains(texto, " "+p+" ")
}

// fragmento da unas palabras alrededor de la coincidencia (sin pasar de ~80 letras).
func fragmento(original, palabra string) string {
	r := []rune(strings.Join(strings.Fields(original), " "))
	n := identidad.Normalizar(string(r))
	i := strings.Index(n, identidad.Normalizar(palabra))
	if i < 0 || len([]rune(n)) != len(r) {
		if len(r) > 80 {
			return string(r[:80]) + "…"
		}
		return string(r)
	}
	pos := len([]rune(n[:i]))
	ini, fin := pos-30, pos+len([]rune(palabra))+30
	if ini < 0 {
		ini = 0
	}
	if fin > len(r) {
		fin = len(r)
	}
	t := string(r[ini:fin])
	if ini > 0 {
		t = "…" + t
	}
	if fin < len(r) {
		t += "…"
	}
	return t
}

// AlMensaje revisa un mensaje recibido contra la lista de palabras clave y crea el aviso local si coincide. Devuelve el id del aviso
// (0 si no hubo). Por defecto el aviso lleva solo la palabra, el chat y la persona; el fragmento del mensaje es opcional y está
// apagado.
func (s *Servicio) AlMensaje(m Mensaje) int64 {
	if m.EsMio || m.Antiguo || m.Edicion || strings.TrimSpace(m.Texto) == "" || s.Alm.AjusteBotInt(s.BotID, "alertas_activas") != 1 {
		return 0
	}
	lista, err := s.Alm.ListarAlertasPalabra(s.BotID)
	if err != nil || len(lista) == 0 {
		return 0
	}
	texto := separaPalabras(m.Texto)
	var halladas []string
	for _, a := range lista {
		if !a.Activa {
			continue
		}
		if len(a.Chats) > 0 {
			ok := false
			for _, c := range a.Chats {
				if c == m.Chat {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		if coincide(texto, a.Palabra) {
			halladas = append(halladas, a.Palabra)
		}
	}
	if len(halladas) == 0 {
		return 0
	}
	ahora := s.ahora()
	s.mu.Lock()
	if t, ok := s.ultimo[m.Chat]; ok && ahora.Sub(t) < 30*time.Second {
		s.mu.Unlock()
		return 0 // una ráfaga de mensajes avisa una sola vez
	}
	s.ultimo[m.Chat] = ahora
	if len(s.ultimo) > 2000 {
		for k, t := range s.ultimo {
			if ahora.Sub(t) > time.Minute {
				delete(s.ultimo, k)
			}
		}
	}
	s.mu.Unlock()
	nChat, nPersona := "un chat", m.Nombre
	if c, _ := s.Alm.ObtenerChat(s.BotID, m.Chat); c != nil && c.Mostrado != "" {
		nChat = c.Mostrado
	}
	if p := s.Alm.ResolverPersona(s.BotID, m.Persona); p != "" {
		for _, f := range s.Alm.AliasDe(s.BotID, p) {
			if c, _ := s.Alm.ObtenerChat(s.BotID, f); c != nil && !c.EsGrupo && c.Mostrado != "" {
				nPersona = c.Mostrado
				break
			}
		}
	}
	if nPersona == "" {
		nPersona = "Alguien"
	}
	lugar := "en " + nChat
	if !m.EsGrupo {
		lugar = "en su chat"
	}
	det := fmt.Sprintf("%s escribió «%s» %s.", nPersona, strings.Join(halladas, "», «"), lugar)
	if s.Alm.AjusteBotInt(s.BotID, "alerta_fragmento") == 1 {
		det += "\n" + fragmento(m.Texto, halladas[0])
	}
	id, err := s.Alm.CrearAvisoDe(s.BotID, TipoPalabra, m.Chat, s.Alm.ResolverPersona(s.BotID, m.Persona), "Palabra clave en un mensaje", det)
	if err != nil {
		return 0
	}
	s.log("Aviso local: palabra clave en un mensaje.")
	return id
}

// ---------- resumen diario y recordatorios ----------

// Resultado es lo que salió al disparar: los avisos creados (para mostrarlos como notificaciones) y la hora de lo siguiente.
type Resultado struct {
	Avisos    []int64 `json:"avisos"`
	Siguiente int64   `json:"siguiente"` // segundos desde 1970 de la próxima alarma (0 = ninguna)
}

// HoraResumen lee "HH:MM" (20:00 si no es válida).
func HoraResumen(t string) (h, m int) {
	if _, err := fmt.Sscanf(strings.TrimSpace(t), "%d:%d", &h, &m); err != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 20, 0
	}
	return h, m
}

// proximoResumen es la próxima vez que toca el resumen (0 si está apagado).
func (s *Servicio) proximoResumen(ahora time.Time) int64 {
	if s.Alm.AjusteBotInt(s.BotID, "resumen_activo") != 1 {
		return 0
	}
	loc := s.Alm.Zona()
	h, m := HoraResumen(s.Alm.AjusteBot(s.BotID, "resumen_hora"))
	t := ahora.In(loc)
	hoy := time.Date(t.Year(), t.Month(), t.Day(), h, m, 0, 0, loc)
	if s.Alm.AjusteBot(s.BotID, "resumen_ultimo") == t.Format("2006-01-02") {
		return hoy.AddDate(0, 0, 1).Unix()
	}
	return hoy.Unix() // si ya pasó y no se avisó hoy, toca ya
}

// Siguiente da la hora de la próxima alarma: el menor entre el siguiente recordatorio y el resumen del día.
func (s *Servicio) Siguiente() int64 {
	ahora := s.ahora()
	sig := s.Alm.SiguienteRecordatorio(s.BotID)
	if r := s.proximoResumen(ahora); r != 0 && (sig == 0 || r < sig) {
		sig = r
	}
	for _, f := range s.Extras {
		if r := f.Siguiente(); r != 0 && (sig == 0 || r < sig) {
			sig = r
		}
	}
	return sig
}

// Disparar crea los avisos que ya tocan: los recordatorios vencidos y, si es la hora, el resumen del día. Cada uno se marca como
// avisado (los que se repiten pasan a su siguiente vez). Es seguro llamarlo de más: no repite lo que ya sonó.
func (s *Servicio) Disparar() Resultado {
	ahora := s.ahora()
	res := Resultado{Avisos: []int64{}}
	if rs, err := s.Alm.RecordatoriosVencidos(s.BotID, ahora); err == nil {
		for _, r := range rs {
			if err := s.Alm.MarcarRecordatorioAvisado(s.BotID, r, ahora); err != nil {
				continue
			}
			if id, err := s.Alm.CrearAviso(s.BotID, TipoRecordatorio, "", "Recordatorio", r.Texto); err == nil {
				res.Avisos = append(res.Avisos, id)
				s.log("Aviso local: recordatorio.")
			}
		}
	}
	if p := s.proximoResumen(ahora); p != 0 && p <= ahora.Unix() {
		d := s.Alm.ResumenDelDia(s.BotID, ahora)
		_ = s.Alm.FijarAjusteBot(s.BotID, "resumen_ultimo", d.Dia)
		if id, err := s.Alm.CrearAviso(s.BotID, TipoResumen, "", "Resumen del día", TextoResumen(d)); err == nil {
			res.Avisos = append(res.Avisos, id)
			s.log("Aviso local: resumen del día.")
		}
	}
	for _, f := range s.Extras {
		res.Avisos = append(res.Avisos, f.Disparar()...)
	}
	s.Alm.LimpiarRecordatoriosHechos(s.BotID, 30, ahora)
	res.Siguiente = s.Siguiente()
	return res
}

// TextoResumen es el contenido del resumen diario.
func TextoResumen(d almacen.DatosResumen) string {
	var b strings.Builder
	switch d.Recibidos {
	case 0:
		b.WriteString("Hoy no llegaron mensajes.")
	case 1:
		b.WriteString("Hoy llegó 1 mensaje")
		fmt.Fprintf(&b, " de %d chat.", d.Chats)
	default:
		fmt.Fprintf(&b, "Hoy llegaron %d mensajes de %d chat(s).", d.Recibidos, d.Chats)
	}
	fmt.Fprintf(&b, "\nChats pendientes: %d", d.Pendientes)
	fmt.Fprintf(&b, "\nSolicitudes por revisar: %d", d.Solicitudes)
	fmt.Fprintf(&b, "\nAdvertencias de hoy: %d", d.Advertencias)
	return b.String()
}
