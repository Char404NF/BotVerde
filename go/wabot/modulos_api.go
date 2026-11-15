package wabot

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"botchat/internal/almacen"
	"botchat/internal/envio"
	"botchat/internal/grupos"
	"botchat/internal/modulos"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// ---------- armado dentro del bot ----------

// enviadorModulos es la salida de la cola de iniciativas: el servicio de envío, por la vía automática (sin el tope de envíos
// manuales y sin pausar al bot en el chat).
type enviadorModulos struct{ s *estado }

func (e enviadorModulos) Texto(ctx context.Context, chat, texto, citaID string, menciones []string) error {
	r, err := e.s.envio.EnviarAuto(almacen.ConEtiqueta(ctx, "modulo"), chat, texto, citaID, menciones)
	if err != nil {
		return err
	}
	if r.Estado != "enviado" {
		return errNoEnviado
	}
	return nil
}

func (e enviadorModulos) Medio(ctx context.Context, chat string, a envio.Adjunto, citaID string) error {
	if id, ok := strings.CutPrefix(a.Ruta, modulos.PrefijoAdj); ok {
		// archivo guardado cifrado (difusión, programados): se descifra a una copia temporal que el envío borra al terminar
		ruta, err := e.s.copiaAdjunto(id)
		if err != nil {
			return err
		}
		a.Ruta = ruta
	}
	r, err := e.s.envio.EnviarMedioAuto(almacen.ConEtiqueta(ctx, "modulo"), chat, a, citaID)
	if err != nil {
		return err
	}
	if r.Estado != "enviado" {
		return errNoEnviado
	}
	return nil
}

// Escribiendo muestra (o quita) "escribiendo…" antes de un envío de módulo (difusión, programados): así el mensaje no aparece de golpe.
func (e enviadorModulos) Escribiendo(ctx context.Context, chat string, si bool) {
	_ = e.s.envio.Escribiendo(ctx, chat, si)
}

func (e enviadorModulos) Conectado() bool { return e.s.cli.IsConnected() && e.s.cli.IsLoggedIn() }

// accionesModulos son las acciones de grupo que usan los módulos H1 y H4, hechas con el servicio de grupos del bot.
type accionesModulos struct{ s *estado }

func (a accionesModulos) gru() (*grupos.Servicio, error) {
	if a.s.gru == nil {
		return nil, grupos.ErrNoDisponible
	}
	return a.s.gru, nil
}

func (a accionesModulos) EsAdmin(ctx context.Context, chat string) (bool, error) {
	g, err := a.gru()
	if err != nil {
		return false, err
	}
	return g.EsAdmin(ctx, chat)
}

func (a accionesModulos) Expulsar(ctx context.Context, chat, persona string) error {
	g, err := a.gru()
	if err != nil {
		return err
	}
	if err := g.Expulsar(ctx, chat, persona); err != nil && !errors.Is(err, grupos.ErrNoMiembro) {
		return err // si ya no estaba en el grupo, no hay nada que hacer
	}
	return nil
}

func (a accionesModulos) Enlace(ctx context.Context, chat string) (string, error) {
	g, err := a.gru()
	if err != nil {
		return "", err
	}
	return g.Enlace(ctx, almacen.PersonaDueno, chat, false)
}

func (a accionesModulos) Agregar(ctx context.Context, chat, persona string) (string, error) {
	g, err := a.gru()
	if err != nil {
		return "", err
	}
	return g.AgregarMiembro(ctx, chat, persona)
}

// RechazarLlamada rechaza una llamada entrante con la acción real de whatsmeow (Client.RejectCall). Hacer llamadas no existe.
func (a accionesModulos) RechazarLlamada(ctx context.Context, de, id string) error {
	j, err := types.ParseJID(de)
	if err != nil {
		return errors.New("no entendí de quién es la llamada")
	}
	if a.s.cli == nil || !a.s.cli.IsConnected() {
		return errors.New("sin conexión con WhatsApp")
	}
	return a.s.cli.RejectCall(ctx, j, id)
}

var registroModulosH sync.Once

// registrarModulosH deja registrados en el marco los módulos opcionales H1–H5 (apagados: registrar no enciende nada). Se llama al
// arrancar el bot y al abrir la lista de módulos sin el bot en marcha.
func registrarModulosH() {
	registroModulosH.Do(func() {
		if err := modulos.RegistrarH(); err != nil {
			panic("módulos H: " + err.Error())
		}
	})
}

// armarModulos crea el gestor de módulos del bot que se está iniciando.
func (s *estado) armarModulos() {
	olvidarGestoresLibres()
	registrarModulosH()
	g := modulos.NuevoGestor(alm, s.botID, s.cmd)
	g.Enviador = enviadorModulos{s}
	g.Acc = accionesModulos{s}
	g.Video = motorVideoActual()
	g.Log = s.log
	g.Cambio = s.avisaCambio
	g.Zona = alm.Zona
	g.Aviso = func(titulo, detalle string) {
		if _, err := alm.CrearAviso(s.botID, "modo_seguro", "", titulo, detalle); err == nil {
			s.log("Aviso local: " + titulo + ".")
		}
	}
	s.mods = g
}

// desconexionForzada: WhatsApp cerró la sesión, la abrieron en otro lugar, hay bloqueo temporal o la versión quedó vieja. Los módulos
// de iniciativa pasan al modo seguro (no se apagan: se reanudan a mano).
func (s *estado) desconexionForzada(tipo string) {
	s.encola(func() { s.mods.ModoSeguro("desconexion:" + tipo) })
}

// alEntrarAlGrupo avisa a los módulos que alguien entró a un grupo.
func (s *estado) alEntrarAlGrupo(v *events.GroupInfo) {
	chat := v.JID.ToNonAD().String()
	joins := v.Join
	s.encola(func() {
		var personas, jids []string
		for _, j := range joins {
			if s.esPropio(j) {
				continue
			}
			personas = append(personas, s.ing.Persona(s.ctx, j))
			jids = append(jids, j.ToNonAD().String())
		}
		if len(personas) > 0 {
			s.mods.Evento(modulos.Evento{Tipo: modulos.EvMiembroEntra, Chat: chat, EsGrupo: true, Personas: personas, JIDs: jids})
		}
	})
}

// alLlamar avisa a los módulos de una llamada entrante.
func (s *estado) alLlamar(v *events.CallOffer) {
	from := v.From
	de := v.CallCreator
	if de.IsEmpty() {
		de = from
	}
	id := v.CallID
	video := false
	if v.Data != nil && v.Data.GetChildByTag("video").Tag != "" {
		video = true
	}
	if s.esPropio(from) {
		return
	}
	s.encola(func() {
		s.mods.Evento(modulos.Evento{Tipo: modulos.EvLlamada, Chat: s.ing.Persona(s.ctx, from), Persona: s.ing.Persona(s.ctx, from),
			LlamadaID: id, LlamadaDe: de.ToNonAD().String(), LlamadaVideo: video})
	})
}

// ---------- API para la pantalla ----------

var (
	gestoresLibres   = map[int64]*modulos.Gestor{}
	gestoresLibresMu sync.Mutex
)

func olvidarGestoresLibres() {
	gestoresLibresMu.Lock()
	gestoresLibres = map[int64]*modulos.Gestor{}
	gestoresLibresMu.Unlock()
}

// gestorDe da el gestor de módulos del bot: el del bot en marcha si lo hay, o uno propio para ver y editar sin conexión
// (sin cola ni envío; al arrancar el bot se restauran los módulos encendidos).
func gestorDe(botID int64) (*modulos.Gestor, error) {
	mu.Lock()
	s := st
	a := alm
	mu.Unlock()
	if a == nil {
		return nil, errors.New("el almacén no está abierto")
	}
	if s != nil && s.botID == botID && s.mods != nil {
		return s.mods, nil
	}
	registrarModulosH()
	gestoresLibresMu.Lock()
	defer gestoresLibresMu.Unlock()
	if g, ok := gestoresLibres[botID]; ok && g.Alm == a {
		return g, nil
	}
	g := modulos.NuevoGestor(a, botID, nil)
	g.Video = motorVideoActual()
	g.Zona = a.Zona
	g.Cambio = func(ev string) {
		if x := agrup; x != nil {
			x.Agregar(ev)
		}
	}
	gestoresLibres[botID] = g
	return g, nil
}

// ListarModulos devuelve (JSON) las tarjetas de los módulos: [{id, nombre, descripcion, icono, riesgo, iniciativa, activo, pausado,
// codigo (apagado|activo|pausado_seguro|esperando_maestro), estado, conEsquema, topeHora, topeDia, defHora, defDia, activadoTs}].
// Eventos: "modulos".
func ListarModulos(botID int64) (string, error) {
	g, err := gestorDe(botID)
	if err != nil {
		return "", err
	}
	return js(g.Lista(), nil)
}

// InfoModulo devuelve (JSON) la tarjeta de un módulo con su esquema de configuración (campos de texto, número, interruptor, lista y
// hora) para dibujar la ventana genérica: esquema = [{clave, titulo, ayuda, tipo, defecto, min, max, largo, varias, opciones, obligatorio}].
func InfoModulo(botID int64, id string) (string, error) {
	g, err := gestorDe(botID)
	if err != nil {
		return "", err
	}
	i, err := g.InfoDe(id)
	if err != nil {
		return "", err
	}
	return js(i, nil)
}

// ActivarModulo enciende un módulo (la pantalla muestra antes el aviso de riesgo si es medio o alto).
func ActivarModulo(botID int64, id string) error {
	g, err := gestorDe(botID)
	if err != nil {
		return err
	}
	if err := g.Activar(id); err != nil {
		accionDueno(botID, "modulo.activar", id, "error")
		return err
	}
	accionDueno(botID, "modulo.activar", id, "")
	return nil
}

// DesactivarModulo apaga un módulo en el acto (conserva su configuración).
func DesactivarModulo(botID int64, id string) error {
	g, err := gestorDe(botID)
	if err != nil {
		return err
	}
	if err := g.Desactivar(id); err != nil {
		return err
	}
	accionDueno(botID, "modulo.desactivar", id, "")
	return nil
}

// ConfigModulo devuelve (JSON) la configuración del módulo: lo guardado completado con los valores por defecto.
func ConfigModulo(botID int64, id string) (string, error) {
	g, err := gestorDe(botID)
	if err != nil {
		return "", err
	}
	return g.ConfigJSON(id)
}

// GuardarConfigModulo valida y guarda (cifrada) la configuración de un módulo; si algo no vale devuelve el error en español.
func GuardarConfigModulo(botID int64, id, configJSON string) error {
	g, err := gestorDe(botID)
	if err != nil {
		return err
	}
	if err := g.GuardarConfig(id, configJSON); err != nil {
		return err
	}
	accionDueno(botID, "modulo.config", id, "")
	return nil
}

// FijarTopesModulo guarda los topes propios de un módulo por hora y por día (0 = los que trae el módulo).
func FijarTopesModulo(botID int64, id string, hora, dia int64) error {
	g, err := gestorDe(botID)
	if err != nil {
		return err
	}
	if err := g.FijarTopesModulo(id, int(hora), int(dia)); err != nil {
		return err
	}
	accionDueno(botID, "modulo.topes", id, "")
	return nil
}

// AjustesIniciativa devuelve (JSON) los ajustes de iniciativa propia: maestro ("0"|"1"), tope_hora, tope_dia, noche_on, noche_desde,
// noche_hasta, retraso_min, retraso_max (segundos), errores, espera_max (minutos) y baja_texto. Eventos: "modulos", "iniciativas".
func AjustesIniciativa(botID int64) (string, error) {
	g, err := gestorDe(botID)
	if err != nil {
		return "", err
	}
	return g.AjustesJSON(), nil
}

// FijarAjusteIniciativa cambia un ajuste de iniciativa (con validación).
func FijarAjusteIniciativa(botID int64, clave, valor string) error {
	g, err := gestorDe(botID)
	if err != nil {
		return err
	}
	if err := g.FijarAjuste(clave, valor); err != nil {
		return err
	}
	accionDueno(botID, "iniciativa."+clave, "", "")
	return nil
}

// FijarIniciativaMaestro enciende o apaga el interruptor general "Iniciativa propia". Apagado (por defecto) ningún módulo puede
// escribir por su cuenta; al apagarlo se descarta lo que esperaba salir.
func FijarIniciativaMaestro(botID int64, activo bool) error {
	g, err := gestorDe(botID)
	if err != nil {
		return err
	}
	if err := g.FijarMaestro(activo); err != nil {
		return err
	}
	v := "apagado"
	if activo {
		v = "encendido"
	}
	accionDueno(botID, "iniciativa.maestro", "", v)
	return nil
}

// ListarNoMolestar devuelve (JSON) la lista global No molestar: [{persona, desdeTs, motivo, confirmado}].
func ListarNoMolestar(botID int64) (string, error) {
	g, err := gestorDe(botID)
	if err != nil {
		return "", err
	}
	return js(g.Alm.ListarNoMolestar(g.BotID))
}

// AgregarNoMolestar mete a una persona a mano en la lista (no se le escribe nada).
func AgregarNoMolestar(botID int64, persona, motivo string) error {
	g, err := gestorDe(botID)
	if err != nil {
		return err
	}
	if err := g.AgregarNoMolestar(persona, motivo); err != nil {
		return err
	}
	accionDueno(botID, "iniciativa.no_molestar_agregar", enmascara(persona), "")
	return nil
}

// QuitarNoMolestar saca a una persona de la lista.
func QuitarNoMolestar(botID int64, persona string) error {
	g, err := gestorDe(botID)
	if err != nil {
		return err
	}
	if err := g.QuitarNoMolestar(persona); err != nil {
		return err
	}
	accionDueno(botID, "iniciativa.no_molestar_quitar", enmascara(persona), "")
	return nil
}

type registroIniciativa struct {
	ID        int64  `json:"id"`
	Modulo    string `json:"modulo"`
	Que       string `json:"que"`
	TS        int64  `json:"ts"`
	Resultado string `json:"resultado"`
	Chat      string `json:"chat"` // enmascarado: solo los últimos 4 caracteres
}

// ListarIniciativas devuelve (JSON) el registro de iniciativas, lo más nuevo primero: [{id, modulo, que, ts, resultado, chat}]. Sin textos
// de mensajes y con el chat enmascarado. modulo "" = todos; antesId > 0 pide la página siguiente. Evento: "iniciativas".
func ListarIniciativas(botID int64, modulo string, limite, antesID int64) (string, error) {
	g, err := gestorDe(botID)
	if err != nil {
		return "", err
	}
	l, err := g.Alm.ListarRegistroModulos(botID, modulo, int(limite), antesID)
	if err != nil {
		return "", err
	}
	out := make([]registroIniciativa, 0, len(l))
	for _, r := range l {
		c := ""
		if r.Chat != "" {
			c = enmascara(r.Chat)
		}
		out = append(out, registroIniciativa{r.ID, r.Modulo, r.Que, r.TS, r.Resultado, c})
	}
	return js(out, nil)
}

// EstadoModoSeguro devuelve (JSON) {activo, ts, motivo, texto, errores}: si los módulos de iniciativa están pausados por el modo seguro.
func EstadoModoSeguro(botID int64) (string, error) {
	g, err := gestorDe(botID)
	if err != nil {
		return "", err
	}
	return g.SeguroJSON(), nil
}

// ReanudarModoSeguro saca del modo seguro: los módulos de iniciativa que estaban encendidos vuelven a trabajar.
func ReanudarModoSeguro(botID int64) error {
	g, err := gestorDe(botID)
	if err != nil {
		return err
	}
	if err := g.Reanudar(); err != nil {
		return err
	}
	accionDueno(botID, "iniciativa.reanudar", "", "")
	return nil
}

// FilasPanelModulo devuelve (JSON) los renglones de un panel de la ventana de un módulo: [{id, titulo, detalle}]. Los paneles (listas y
// formularios simples) vienen en InfoModulo → paneles. Evento: "modulos".
func FilasPanelModulo(botID int64, id, panel string) (string, error) {
	g, err := gestorDe(botID)
	if err != nil {
		return "", err
	}
	f, err := g.FilasPanel(id, panel)
	if err != nil {
		return "", err
	}
	return js(f, nil)
}

// AccionModulo corre un botón de un panel de un módulo (argsJSON: los valores del formulario, o {"id": ...} del renglón). Devuelve el
// mensaje corto en español para mostrar, o el error. Las acciones del dueño quedan en el registro de acciones (sin textos).
func AccionModulo(botID int64, id, accion, argsJSON string) (string, error) {
	g, err := gestorDe(botID)
	if err != nil {
		return "", err
	}
	args := modulos.Args{}
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", errors.New("los datos del formulario no son válidos")
		}
	}
	msg, err := g.AccionModulo(id, accion, args)
	res := ""
	if err != nil {
		res = "error"
	}
	accionDueno(botID, "modulo.accion", id+":"+accion, res)
	return msg, err
}

// ProximoModulos dice cuándo toca lo siguiente que tienen agendado los módulos encendidos (mensajes programados y difusiones), en
// segundos desde 1970; 0 si no hay nada. La app programa con esto una alarma de Android.
func ProximoModulos(botID int64) (int64, error) {
	g, err := gestorDe(botID)
	if err != nil {
		return 0, err
	}
	return g.ProximoAgenda(), nil
}

// TickModulos revisa ya la agenda de los módulos (lo llama la alarma de Android a la hora justa). Si el bot no está en marcha no hace nada:
// al arrancar, cada módulo revisa lo que quedó pendiente (y descarta lo que ya pasó del tiempo de tolerancia).
func TickModulos(botID int64) error {
	mu.Lock()
	s := st
	mu.Unlock()
	if s == nil || s.botID != botID || s.mods == nil {
		return nil
	}
	s.encola(func() { s.mods.TickAgenda() })
	return nil
}

// FijarLlaveModulo le pasa a Go, SOLO en memoria, una llave que la app guarda cifrada en el Android Keystore (por ejemplo la llave de
// la IA). Valor vacío = olvidarla. Nunca se escribe en la base, el registro ni los logs. La app la vuelve a mandar cada vez que arranca.
func FijarLlaveModulo(botID int64, modulo, clave, valor string) error {
	registrarModulosH()
	if !modulos.CampoLlaveValido(modulo, clave) {
		return errors.New("ese módulo no usa esa llave")
	}
	modulos.FijarLlave(botID, modulo, clave, valor)
	accionDueno(botID, "modulo.llave", modulo, map[bool]string{true: "borrada", false: ""}[valor == ""])
	return nil
}

// TieneLlaveModulo dice si hay una llave en memoria para ese módulo (nunca la devuelve).
func TieneLlaveModulo(botID int64, modulo, clave string) bool {
	return modulos.TieneLlave(botID, modulo, clave)
}
