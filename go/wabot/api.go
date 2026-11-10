package wabot

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/ingesta"
)

// Tipos de red para EstablecerRed.
const (
	RedDesconocida = 0
	RedWifi        = 1
	RedDatos       = 2
)

var (
	alm      *almacen.Almacen
	agrup    *agrupador
	redAhora atomic.Int32
	polMu    sync.RWMutex
	pol      = ingesta.PoliticaPorDefecto()
	// ajusteConservarApagado recuerda el ajuste aunque todavía no esté abierto el almacén (por defecto se conserva).
	ajusteConservarApagado atomic.Bool
)

func politicaActual() ingesta.Politica {
	polMu.RLock()
	defer polMu.RUnlock()
	return pol
}

func redActual() ingesta.Red { return ingesta.Red(redAhora.Load()) }

// abrirAlmacenLocked abre messages.db si todavía no está abierto. Hay que tener `mu`.
func abrirAlmacenLocked(raiz string, clave []byte) error {
	if alm != nil {
		if filepath.Clean(alm.Raiz()) == filepath.Clean(raiz) {
			return nil
		}
		return errors.New("el almacén ya está abierto en otra carpeta")
	}
	a, err := almacen.Abrir(raiz, clave)
	if err != nil {
		return err
	}
	a.Cambio = func(ev string) {
		if g := agrup; g != nil {
			g.Agregar(ev)
		}
	}
	alm = a
	a.FijarConservarEliminados(!ajusteConservarApagado.Load())
	return nil
}

// AbrirAlmacen abre la base de mensajes para ver lo guardado (con o sin bot conectado). raiz es la carpeta de datos de la app
// y clave la llave de datos de 32 bytes. El observador recibe los cambios en vivo; puede ser nil.
func AbrirAlmacen(raiz string, clave []byte, o Observador) error {
	mu.Lock()
	defer mu.Unlock()
	if err := abrirAlmacenLocked(raiz, clave); err != nil {
		return err
	}
	if o != nil {
		agrup = nuevoAgrupador(250*time.Millisecond, o.OnCambio)
	}
	return nil
}

// CerrarAlmacen cierra la base (por ejemplo al bloquear la app). Si el bot sigue iniciado, se detiene antes.
func CerrarAlmacen() {
	Stop()
	mu.Lock()
	defer mu.Unlock()
	if alm != nil {
		_ = alm.Cerrar()
		alm = nil
	}
}

func almacenAbierto() (*almacen.Almacen, error) {
	mu.Lock()
	defer mu.Unlock()
	if alm == nil {
		return nil, errors.New("el almacén no está abierto")
	}
	return alm, nil
}

func js(v any, err error) (string, error) {
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(v)
	return string(b), err
}

// ListarChats devuelve (JSON) los chats de personas o de grupos, del más reciente al más viejo.
// consulta busca por nombre y por número. Cada fila trae la clave interna `jid`, que la pantalla nunca muestra.
func ListarChats(botID int64, grupos bool, consulta string, limite, desde int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ListarChats(botID, grupos, consulta, limite, desde))
}

// ObtenerChat devuelve (JSON) un chat con su último mensaje, o "null" si no existe.
func ObtenerChat(botID int64, chat string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ChatPorJID(botID, chat))
}

// ListarMensajes devuelve (JSON) hasta `limite` mensajes de un chat, del más nuevo al más viejo.
// Para la página siguiente se pasan la hora y el id del último mensaje recibido; con antesTS=0 empieza por los últimos.
func ListarMensajes(botID int64, chat string, antesTS int64, antesID string, limite int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ListarMensajes(botID, chat, antesTS, antesID, limite))
}

// MensajesPorId devuelve (JSON) los mensajes pedidos de un chat (idsJSON es un arreglo de ids), con una consulta por lote.
// Los que ya no existen no aparecen. La pantalla lo usa para refrescar mensajes ya cargados (recibos, reacciones, ediciones…).
func MensajesPorId(botID int64, chat, idsJSON string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	var ids []string
	if err := json.Unmarshal([]byte(idsJSON), &ids); err != nil {
		return "", err
	}
	return js(a.MensajesPorID(botID, chat, ids))
}

// CargaAnterior activa la forma de cargar mensajes de la 0.4.0 (nombres uno por uno, sin caché) solo para medir antes y después.
func CargaAnterior(si bool) { almacen.CargaAntigua(si) }

// CrearChatPrueba crea un grupo de prueba con `cuantos` mensajes para medir la apertura de un chat; devuelve su JID.
// BorrarChat lo quita al terminar.
func CrearChatPrueba(botID int64, cuantos int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return a.ChatSintetico(botID, cuantos)
}

// ListarArchivos devuelve (JSON) los mensajes con multimedia. chat="" es todos los chats; categoria: foto, video, audio, documento o "".
func ListarArchivos(botID int64, chat, categoria string, soloDescargados bool, limite, desde int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ListarArchivos(botID, chat, categoria, soloDescargados, limite, desde))
}

// EspacioTotal devuelve (JSON) lo que ocupa BotChat y el espacio libre del teléfono.
func EspacioTotal(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.EspacioTotal(botID), nil)
}

// EspacioPorTipo devuelve (JSON) lo que ocupa cada categoría: foto, video, audio, documento.
func EspacioPorTipo(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.EspacioPorTipo(botID))
}

// EspacioPorChat devuelve (JSON) los chats con archivos, del que más ocupa al que menos.
func EspacioPorChat(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	l, err := a.EspacioPorChat(botID)
	if l == nil {
		l = []almacen.EspacioChat{}
	}
	return js(l, err)
}

type resultadoBorrado struct {
	Bytes    int64 `json:"bytes"`
	Cantidad int   `json:"cantidad"`
}

// EstimarBorrado dice cuánto se liberaría sin borrar nada. filtroJSON: {"chat":"","categoria":"","diasMin":0}.
func EstimarBorrado(botID int64, filtroJSON string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	var f almacen.Filtro
	if err := json.Unmarshal([]byte(filtroJSON), &f); err != nil {
		return "", err
	}
	b, n, err := a.EstimarBorrado(botID, f)
	return js(resultadoBorrado{b, n}, err)
}

// BorrarArchivos borra los archivos descargados que cumplen el filtro (conserva el texto de los mensajes).
func BorrarArchivos(botID int64, filtroJSON string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	var f almacen.Filtro
	if err := json.Unmarshal([]byte(filtroJSON), &f); err != nil {
		return "", err
	}
	b, n, err := a.BorrarArchivos(botID, f)
	return js(resultadoBorrado{b, n}, err)
}

// EstimarLista dice cuánto se liberaría al borrar archivos concretos. itemsJSON: [{"chat":"…","id":"…"}].
func EstimarLista(botID int64, itemsJSON string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	var it []almacen.Item
	if err := json.Unmarshal([]byte(itemsJSON), &it); err != nil {
		return "", err
	}
	b, n, err := a.EstimarLista(botID, it)
	return js(resultadoBorrado{b, n}, err)
}

// BorrarLista borra archivos concretos (selección múltiple).
func BorrarLista(botID int64, itemsJSON string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	var it []almacen.Item
	if err := json.Unmarshal([]byte(itemsJSON), &it); err != nil {
		return "", err
	}
	b, n, err := a.BorrarLista(botID, it)
	return js(resultadoBorrado{b, n}, err)
}

// BorrarChat borra un chat completo: mensajes, archivos, miniaturas y foto. Devuelve (JSON) lo liberado.
func BorrarChat(botID int64, chat string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	b, err := a.BorrarChat(botID, chat)
	return js(resultadoBorrado{Bytes: b}, err)
}

// EliminarBot borra todo lo guardado de un bot (chats, mensajes, archivos y fotos). La sesión de WhatsApp se cierra con Logout.
func EliminarBot(botID int64) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	return a.EliminarBot(botID)
}

// Reparar revisa la base contra los archivos que existen de verdad y corrige estados y tamaños. Devuelve (JSON) el reporte.
func Reparar(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.Reparar(botID))
}

// Descargar atiende el botón "Descargar" de un archivo. Necesita el bot iniciado y conectado.
func Descargar(botID int64, chat, id string) error {
	mu.Lock()
	s := st
	mu.Unlock()
	if s == nil || s.botID != botID {
		return errors.New("inicia el bot para descargar archivos")
	}
	if !(s.cli.IsConnected() && s.cli.IsLoggedIn()) {
		return errors.New("el bot no está conectado")
	}
	return s.descargas.Manual(chat, id)
}

// FotoGrande trae la foto de perfil completa de un chat (al tocar el encabezado). Devuelve la ruta relativa del archivo cifrado.
func FotoGrande(botID int64, chat string) (string, error) {
	mu.Lock()
	s := st
	mu.Unlock()
	if s == nil || s.botID != botID {
		return "", errors.New("inicia el bot para ver la foto grande")
	}
	return s.fotos.Grande(context.Background(), chat)
}

// ConfigurarDescargas cambia las reglas de descarga automática (JSON de ingesta.Politica). Se pierde al cerrar la app: la app las guarda.
func ConfigurarDescargas(json_ string) error {
	p := politicaActual()
	if err := json.Unmarshal([]byte(json_), &p); err != nil {
		return err
	}
	polMu.Lock()
	pol = p
	polMu.Unlock()
	return nil
}

// DescargasConfig devuelve (JSON) las reglas de descarga actuales.
func DescargasConfig() string {
	b, _ := json.Marshal(politicaActual())
	return string(b)
}

// EstablecerRed avisa el tipo de conexión del teléfono: 0 desconocida, 1 Wi-Fi, 2 datos móviles.
func EstablecerRed(tipo int) { redAhora.Store(int32(tipo)) }

// EstablecerZona fija la zona horaria del teléfono (por ejemplo "America/Mexico_City") para agrupar mensajes por día.
// La primera vez que se llama también cuenta los mensajes que ya estaban guardados, en segundo plano.
func EstablecerZona(id string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	if err := a.FijarZona(id); err != nil {
		return err
	}
	go func() { _ = a.RellenarEstadisticas() }()
	return nil
}

// EstadisticasChat devuelve (JSON) el resumen de un chat: enviados, recibidos, por tipo, por hora, por día de la semana y, en grupos, quién escribe más.
func EstadisticasChat(botID int64, chat string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ResumenDeChat(botID, chat))
}

// CalendarioChat devuelve (JSON) cuántos mensajes hubo cada día de un mes (formato AAAA-MM).
func CalendarioChat(botID int64, chat, mes string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.CalendarioMes(botID, chat, mes))
}

// MensajesDia devuelve (JSON) los mensajes de un día (AAAA-MM-DD) de un chat, del más viejo al más nuevo.
func MensajesDia(botID int64, chat, dia string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.MensajesDelDia(botID, chat, dia))
}

// EstadisticasGlobales devuelve (JSON) los totales de toda la cuenta, los chats más activos y la actividad de los últimos `dias` días.
func EstadisticasGlobales(botID int64, dias int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ResumenDeTodo(botID, dias))
}

// ListarFiltrados devuelve (JSON {total, items}) los mensajes de un chat que cumplen un filtro (ver almacen.FiltroLista), del más nuevo al más viejo.
// En la primera página (antesTS=0) trae también el total; las siguientes usan hora e id del último recibido.
func ListarFiltrados(botID int64, chat, filtroJSON string, antesTS int64, antesID string, limite int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	f, err := almacen.ParseFiltro(filtroJSON)
	if err != nil {
		return "", err
	}
	return js(a.ListarFiltrados(botID, chat, f, antesTS, antesID, limite))
}

// PersonasChat devuelve (JSON) quién escribió en un chat, de quien más a quien menos.
func PersonasChat(botID int64, chat string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.PersonasDeChat(botID, chat))
}

// OlvidarMensaje borra para siempre el contenido y el archivo de un mensaje.
func OlvidarMensaje(botID int64, chat, id string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	_, err = a.OlvidarMensaje(botID, chat, id)
	return err
}

// ConservarEliminados enciende o apaga conservar el contenido de los mensajes que otros eliminan.
// Apagado: al llegar la eliminación se sobrescribe el contenido y se borra el archivo.
func ConservarEliminados(si bool) {
	ajusteConservarApagado.Store(!si)
	if a, err := almacenAbierto(); err == nil {
		a.FijarConservarEliminados(si)
	}
}
