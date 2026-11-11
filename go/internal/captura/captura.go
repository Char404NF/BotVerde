// Package captura es el diagnóstico de mensajes entrantes del Laboratorio: guarda, SOLO en memoria, el esquema (nombres de campos) de los
// últimos mensajes recibidos para saber con qué forma llegan. Nunca guarda bytes, claves ni identificadores completos, y nada va a disco.
package captura

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	MaxMensajes = 30
	Duracion    = 10 * time.Minute
	maxTexto    = 60
	maxProf     = 12
	maxLineas   = 400
)

// Item es la captura de un mensaje.
type Item struct {
	N       int    `json:"n"`
	TS      int64  `json:"ts"`
	Chat    string `json:"chat"` // privado, grupo, difusión/estado, canal
	Clasif  string `json:"clasif"`
	Arbol   string `json:"arbol"`
	Detalle string `json:"detalle"` // banderas del evento
}

// Estado es lo que ve la pantalla.
type Estado struct {
	Activo        bool   `json:"activo"`
	MostrarTextos bool   `json:"mostrarTextos"`
	RestanSeg     int    `json:"restanSeg"`
	Capturados    int    `json:"capturados"` // de esta sesión de captura
	Max           int    `json:"max"`
	Items         []Item `json:"items"`
}

// Servicio guarda el interruptor y el buffer. Todo en memoria.
type Servicio struct {
	mu      sync.Mutex
	ahora   func() time.Time
	activo  bool
	textos  bool
	vence   time.Time
	cuenta  int
	items   []Item
	proximo int
}

func Nuevo() *Servicio { return &Servicio{ahora: time.Now} }

// FijarReloj cambia el reloj (pruebas).
func (s *Servicio) FijarReloj(f func() time.Time) { s.mu.Lock(); s.ahora = f; s.mu.Unlock() }

func (s *Servicio) venceLocked() {
	if s.activo && !s.ahora().Before(s.vence) {
		s.activo = false
	}
}

// Activar enciende (o apaga) la captura. Al encender empiezan de nuevo el reloj de 10 minutos y la cuenta de 30; el buffer se conserva.
func (s *Servicio) Activar(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activo = on
	if on {
		s.vence = s.ahora().Add(Duracion)
		s.cuenta = 0
	}
}

// MostrarTextos incluye los textos cortos (opción apagada por defecto).
func (s *Servicio) MostrarTextos(on bool) { s.mu.Lock(); s.textos = on; s.mu.Unlock() }

// Borrar vacía el buffer.
func (s *Servicio) Borrar() { s.mu.Lock(); s.items = nil; s.mu.Unlock() }

// Activa dice si hay que capturar ahora (barato; lo llama el trabajador antes de armar nada).
func (s *Servicio) Activa() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.venceLocked()
	return s.activo
}

// Estado devuelve una copia para la pantalla.
func (s *Servicio) Estado() Estado {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.venceLocked()
	e := Estado{Activo: s.activo, MostrarTextos: s.textos, Capturados: s.cuenta, Max: MaxMensajes, Items: append([]Item{}, s.items...)}
	if s.activo {
		e.RestanSeg = int(s.vence.Sub(s.ahora()).Seconds())
	}
	return e
}

// Registrar arma el volcado estructural de un mensaje entrante y lo guarda. Se llama desde el trabajador, nunca desde el manejador de
// eventos. clasif es la clasificación resultante (la arma quien llama). No hace nada si la captura está apagada.
func (s *Servicio) Registrar(evt *events.Message, clasif string) {
	if evt == nil || evt.Info.IsFromMe {
		return
	}
	s.mu.Lock()
	s.venceLocked()
	if !s.activo {
		s.mu.Unlock()
		return
	}
	textos := s.textos
	s.mu.Unlock()

	arbol := Volcar(evt, textos)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.venceLocked()
	if !s.activo {
		return
	}
	s.proximo++
	s.cuenta++
	s.items = append(s.items, Item{N: s.proximo, TS: s.ahora().Unix(), Chat: TipoChat(evt.Info.Chat), Clasif: clasif, Arbol: arbol, Detalle: banderas(evt)})
	if len(s.items) > MaxMensajes {
		s.items = s.items[len(s.items)-MaxMensajes:]
	}
	if s.cuenta >= MaxMensajes {
		s.activo = false
	}
}

// TipoChat dice qué clase de chat es, sin revelar el identificador.
func TipoChat(j types.JID) string {
	switch j.Server {
	case types.GroupServer:
		return "grupo"
	case types.DefaultUserServer, types.HiddenUserServer, types.LegacyUserServer, types.BotServer:
		return "privado"
	case types.BroadcastServer:
		return "difusión/estado"
	case types.NewsletterServer:
		return "canal"
	}
	return "otro"
}

func banderas(evt *events.Message) string {
	var b []string
	if evt.IsEphemeral {
		b = append(b, "efímero")
	}
	if evt.IsViewOnce {
		b = append(b, "ver una vez")
	}
	if evt.IsEdit {
		b = append(b, "edición")
	}
	if evt.IsDocumentWithCaption {
		b = append(b, "documento con pie")
	}
	if evt.IsBotInvoke {
		b = append(b, "invoca bot")
	}
	if evt.IsLottieSticker {
		b = append(b, "sticker lottie")
	}
	if evt.Info.DeviceSentMeta != nil {
		b = append(b, "enviado desde otro dispositivo")
	}
	if evt.RawMessage != nil && evt.Message != nil && !proto.Equal(evt.RawMessage, evt.Message) {
		b = append(b, "RawMessage distinto de Message")
	}
	if evt.Message == nil {
		b = append(b, "Message vacío")
	}
	if len(b) == 0 {
		return "-"
	}
	return strings.Join(b, ", ")
}

// Volcar arma el árbol de nombres de campos presentes de Message y, si difiere, de RawMessage.
func Volcar(evt *events.Message, textos bool) string {
	var sb strings.Builder
	lineas := 0
	if evt.Message != nil {
		sb.WriteString("Message\n")
		volcarMsg(&sb, evt.Message.ProtoReflect(), 1, textos, false, &lineas)
	}
	if evt.RawMessage != nil && (evt.Message == nil || !proto.Equal(evt.RawMessage, evt.Message)) {
		sb.WriteString("RawMessage\n")
		volcarMsg(&sb, evt.RawMessage.ProtoReflect(), 1, textos, false, &lineas)
	}
	return strings.TrimRight(sb.String(), "\n")
}

// Mensajes de respuesta de botón o lista: sus campos de identificación son nuestros y siempre se muestran.
var respuestas = map[protoreflect.Name]bool{
	"ButtonsResponseMessage": true, "ListResponseMessage": true, "TemplateButtonReplyMessage": true,
	"InteractiveResponseMessage": true, "NativeFlowResponseMessage": true, "SingleSelectReply": true,
}
var camposRespuesta = map[protoreflect.Name]bool{
	"paramsJSON": true, "selectedID": true, "selectedDisplayText": true, "name": true,
	"selectedButtonID": true, "selectedRowID": true, "title": true, "selectedIndex": true,
}

var reNum = regexp.MustCompile(`\d{7,}`)

// Enmascara deja los teléfonos largos como ***1234.
func Enmascara(s string) string {
	return reNum.ReplaceAllStringFunc(s, func(x string) string { return "***" + x[len(x)-4:] })
}

func corto(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return Enmascara(s)
}

func volcarMsg(sb *strings.Builder, m protoreflect.Message, nivel int, textos, esResp bool, lineas *int) {
	var campos []protoreflect.FieldDescriptor
	m.Range(func(fd protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
		campos = append(campos, fd)
		return true
	})
	sort.Slice(campos, func(i, j int) bool { return campos[i].Number() < campos[j].Number() })
	sangria := strings.Repeat("  ", nivel)
	for _, fd := range campos {
		if *lineas >= maxLineas {
			sb.WriteString(sangria + "…\n")
			return
		}
		*lineas++
		v := m.Get(fd)
		nombre := string(fd.Name())
		switch {
		case fd.IsMap():
			fmt.Fprintf(sb, "%s%s (mapa, %d)\n", sangria, nombre, v.Map().Len())
		case fd.IsList():
			l := v.List()
			fmt.Fprintf(sb, "%s%s [%d]", sangria, nombre, l.Len())
			if fd.Message() != nil && l.Len() > 0 && nivel < maxProf {
				sb.WriteString(" (primero)\n")
				volcarMsg(sb, l.Get(0).Message(), nivel+1, textos, respuestas[fd.Message().Name()], lineas)
			} else if fd.Message() == nil && l.Len() > 0 {
				sb.WriteString(" = " + escalar(fd, l.Get(0), textos, esResp) + "\n")
			} else {
				sb.WriteString("\n")
			}
		case fd.Message() != nil:
			if nivel >= maxProf {
				fmt.Fprintf(sb, "%s%s …\n", sangria, nombre)
				continue
			}
			fmt.Fprintf(sb, "%s%s\n", sangria, nombre)
			volcarMsg(sb, v.Message(), nivel+1, textos, esResp || respuestas[fd.Message().Name()], lineas)
		default:
			fmt.Fprintf(sb, "%s%s = %s\n", sangria, nombre, escalar(fd, v, textos, esResp))
		}
	}
	if len(m.GetUnknown()) > 0 {
		fmt.Fprintf(sb, "%s(campos desconocidos: %d bytes)\n", sangria, len(m.GetUnknown()))
	}
}

// escalar describe un valor simple. Los bytes solo por su largo; los textos por su largo (o su contenido corto si se pidió, o si es un
// campo conocido de respuesta de botón); los números no se muestran.
func escalar(fd protoreflect.FieldDescriptor, v protoreflect.Value, textos, esResp bool) string {
	switch fd.Kind() {
	case protoreflect.BytesKind:
		return fmt.Sprintf("bytes(%d)", len(v.Bytes()))
	case protoreflect.StringKind:
		t := v.String()
		n := utf8.RuneCountInString(t)
		if esResp && camposRespuesta[fd.Name()] {
			if n > 600 {
				return fmt.Sprintf("texto(%d)", n)
			}
			return fmt.Sprintf("%q (%d)", Enmascara(t), n)
		}
		if textos && n <= maxTexto {
			return fmt.Sprintf("%q (%d)", corto(t), n)
		}
		return fmt.Sprintf("texto(%d)", n)
	case protoreflect.BoolKind:
		return fmt.Sprintf("%v", v.Bool())
	case protoreflect.EnumKind:
		if ev := fd.Enum().Values().ByNumber(v.Enum()); ev != nil {
			return string(ev.Name())
		}
		return "enum?"
	}
	if esResp && camposRespuesta[fd.Name()] {
		return fmt.Sprintf("%v", v.Interface())
	}
	return "número"
}
