// Paquete wabot: conexión con WhatsApp (whatsmeow), almacén de mensajes cifrados y descargas, para usar desde Android con gomobile.
// Solo usa tipos simples (string, bool, int, int64, []byte, error y las interfaces Listener y Observador).
package wabot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"botchat/internal/auto"
	"botchat/internal/avisos"
	"botchat/internal/captura"
	"botchat/internal/comandos"
	"botchat/internal/envio"
	"botchat/internal/estadoapp"
	"botchat/internal/flujo"
	"botchat/internal/fotos"
	"botchat/internal/grupos"
	"botchat/internal/ingesta"
	"botchat/internal/lab"
	"botchat/internal/medios"
	"botchat/internal/modulos"
	"botchat/internal/motor"
	"botchat/internal/permisos"
	"botchat/internal/personas"
	"botchat/internal/silencio"
	"botchat/internal/utilidades"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	_ "modernc.org/sqlite" // SQLite en Go puro (sin cgo)
)

// Listener recibe lo que pasa en el bot. Lo implementa la app Kotlin.
//
// Textos de OnStatus: "detenido", "sin_vincular", "esperando_codigo", "vinculado", "conectando", "conectado", "desconectado",
// "sesion_cerrada" (WhatsApp desvinculó este dispositivo) y "error: <detalle>".
type Listener interface {
	OnStatus(texto string)
	OnPairingCode(codigo string)
	OnLog(linea string)
}

type estado struct {
	l       Listener
	cont    *sqlstore.Container
	cli     *whatsmeow.Client
	qr      chan struct{}
	cerr    bool // true cuando el cierre lo pidió la app (Stop/Logout)
	botID   int64
	ctx     context.Context
	cancel  context.CancelFunc
	trabajo chan func()
	wg      sync.WaitGroup

	ing           *ingesta.Ingesta
	envio         *envio.Servicio
	motor         *motor.Motor
	flujo         *flujo.Motor
	autoDec       *auto.Decisor // envuelve a flujo: bienvenida, ausencia, recibido y cierre
	atajos        *auto.Atajos
	sincronizando atomic.Bool // WhatsApp está entregando mensajes acumulados: no se contestan
	descargas     *medios.Gestor
	fotos         *fotos.Gestor
	grupos        *colaGrupos
	perm          *permisos.Servicio
	cmd           *comandos.Despachador
	pers          *personas.Servicio
	sil           *silencio.Servicio
	gru           *grupos.Servicio
	eapp          *estadoapp.Servicio
	avs           *avisos.Servicio
	util          *utilidades.Servicio
	lab           *lab.Servicio
	captura       *captura.Servicio
	mods          *modulos.Gestor
	admins        *cacheAdmins
	refresco      *time.Timer
	refMu         sync.Mutex
}

var (
	mu sync.Mutex
	st *estado
)

// log envía una línea al registro con los números largos enmascarados. Nunca debe recibir texto de mensajes, nombres ni nombres de archivo.
func (s *estado) log(linea string) { s.l.OnLog(enmascaraTexto(linea)) }

func (s *estado) status(t string) { s.l.OnStatus(t) }

// dsnSesion arma la dirección de la base de la sesión. En Android no existe /tmp: sin temp_store=MEMORY,
// SQLite falla con "disk I/O error (6410)" al necesitar un archivo temporal (se vio al sincronizar el estado de la app).
func dsnSesion(dataDir string) string {
	return "file:" + filepath.Join(dataDir, "sesion.db") +
		"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=temp_store(2)"
}

// Start abre la sesión guardada y el almacén de mensajes. raiz es la carpeta de datos de la app (filesDir),
// botID el número del bot y clave la llave de datos de 32 bytes (solo vive en memoria). Si la sesión ya está vinculada, se conecta sola.
func Start(raiz string, botID int64, clave []byte, l Listener) error {
	if l == nil {
		return errors.New("falta el listener")
	}
	mu.Lock()
	defer mu.Unlock()
	if st != nil {
		return errors.New("el bot ya está iniciado")
	}
	if err := abrirAlmacenLocked(raiz, clave); err != nil {
		return err
	}
	// v0.6.0 E9: antes de que nada pueda aplicar el tope, se decide si espera el aviso de la primera vez.
	alm.InicializarTope(botID)
	sesion := filepath.Join(raiz, "bots", fmt.Sprint(botID))
	if err := os.MkdirAll(sesion, 0o700); err != nil {
		return fmt.Errorf("no se pudo crear la carpeta: %w", err)
	}
	ctx := context.Background()
	s := &estado{l: l, botID: botID, qr: make(chan struct{}, 1), trabajo: make(chan func(), 8192)}
	lg := &registro{salida: func(x string) { s.log(x) }}
	cont, err := sqlstore.New(ctx, "sqlite", dsnSesion(sesion), lg.Sub("DB"))
	if err != nil {
		return fmt.Errorf("no se pudo abrir la base: %w", err)
	}
	dev, err := cont.GetFirstDevice(ctx)
	if err != nil {
		_ = cont.Close()
		return fmt.Errorf("no se pudo leer la sesión: %w", err)
	}
	cli := whatsmeow.NewClient(dev, lg.Sub("WA"))
	cli.EnableAutoReconnect = true
	s.cont, s.cli = cont, cli
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.armar()
	cli.AddEventHandler(s.evento)
	st = s
	s.wg.Add(3)
	go s.trabajador()
	go func() {
		defer s.wg.Done()
		s.mantenimientoAuditoria(s.ctx)
	}()
	go func() {
		defer s.wg.Done()
		(&vigilante{vinculado: func() bool { return cli.Store.ID != nil }, conectado: cli.IsConnected, log: s.log,
			conectar: func() { _ = cli.Connect() },
			avisoTras: func() time.Duration {
				return time.Duration(alm.AjusteBotInt(s.botID, "aviso_desconexion")) * time.Minute
			},
			alLargo: func(min int) {
				_, _ = alm.CrearAviso(s.botID, "conexion", "", "El bot perdió la conexión", fmt.Sprintf("Lleva %d minutos sin conexión a WhatsApp. Revisa el internet del teléfono; si sigue igual, abre la app.", min))
			},
			alVolver: func(min int) {
				_, _ = alm.CrearAviso(s.botID, "conexion", "", "El bot volvió a conectarse", fmt.Sprintf("Estuvo %d minutos sin conexión y ya está de nuevo en línea.", min))
			}}).correr(s.ctx)
	}()
	if dev.ID == nil {
		s.log("Sesión nueva: falta vincular.")
		s.status("sin_vincular")
		return nil
	}
	s.log("Sesión encontrada. Conectando…")
	s.status("conectando")
	go func() {
		if err := cli.Connect(); err != nil {
			s.log("No se pudo conectar: " + err.Error())
			s.status("error: " + err.Error())
		}
	}()
	return nil
}

// armar crea la ingesta, las descargas y las fotos de perfil.
func (s *estado) armar() {
	conectado := func() bool { return s.cli.IsConnected() && s.cli.IsLoggedIn() }
	s.descargas = medios.Nuevo(medios.Config{BotID: s.botID, Alm: alm,
		Cliente:  func() (medios.Cliente, bool) { return s.cli, conectado() },
		Politica: politicaActual, Libre: func() int64 { l, _ := alm.Libre(); return l }, Log: s.log,
		Red: redActual, Bateria: bateriaActual, TopeTotal: func() int64 { return alm.TopeTotal(s.botID) }})
	s.fotos = fotos.Nuevo(fotos.Config{BotID: s.botID, Alm: alm, Cliente: func() (fotos.Cliente, bool) { return s.cli, conectado() }, Log: s.log})
	s.grupos = nuevaColaGrupos(s)
	s.ing = ingesta.Nueva(ingesta.Config{BotID: s.botID, Alm: alm, Contactos: s.cli.Store.Contacts, Mapa: s.cli.Store.LIDs,
		Servicios: servicios{s}, EsPropio: s.esPropio, VerUnaVezDias: func() int { return int(verUnaVezDias.Load()) }, Politica: politicaActual, Red: redActual, Libre: func() int64 { l, _ := alm.Libre(); return l }, Log: s.log,
		Bateria: bateriaActual, TopeTotal: func() int64 { return alm.TopeTotal(s.botID) }})
	s.envio = envio.Nuevo(s.botID, alm, envio.Real{Client: s.cli}, s.log)
	s.envio.PausaBot = pausaTrasEscribir
	alm.SembrarSaludo(s.botID, flujo.SaludoJSON())
	s.flujo = s.nuevoFlujo()
	s.armarPermisos()
	s.armarGrupos()
	s.armarSilencio()
	s.armarPersonas()
	s.armarAlertas()
	s.eapp = estadoapp.Nuevo(alm, s.botID, estadoapp.Real{Client: s.cli}, s.log)
	s.armarModulos()
	s.armarAuto()
	s.armarLab()
	s.captura = captura.Nuevo()
	s.motor = motor.Nuevo(motor.Config{
		Perm: s.perm, Cmd: s.cmd, EsAdminGrupo: s.admins.esAdmin, Baja: s.mods.Baja,
		BotID: s.botID, Alm: alm, Decisor: s.decisor(), Enviador: enviadorMotor{s}, Log: s.log,
		Activo: motorActivo.Load, Firma: firmaActual, VentanaAntigua: func() time.Duration { return time.Duration(ventanaAntiguaMin.Load()) * time.Minute },
		// Los envíos fallidos seguidos que frenan al motor también activan el modo seguro de los módulos de iniciativa.
		AlFrenar: func() { s.avisaCambio("motor:freno"); s.mods.ModoSeguro("errores") },
		AlHumano: func(chat, aviso string) { s.avisaCambio("humano:" + chat) },
		// Menús en grupos: tope de mensajes por minuto en un grupo y a qué conversación pertenece cada mensaje del menú que sale.
		LimiteGrupo: func() int { return alm.AjusteBotInt(s.botID, "grupo_msgs_min") },
		// Mensajes automáticos: lo que salió se anota (repetición por persona y registro sin textos); y los "/atajo" que escribo yo.
		AlAuto: s.autoDec.AlAuto,
		Atajos: s.atajos,
		AlEnviado: func(chat, id string, ref *motor.RefSesion) {
			if strings.HasSuffix(chat, "@g.us") {
				_ = alm.GuardarMsgBot(s.botID, chat, id, ref.ID, ref.Persona, ref.MenuID, time.Now())
			}
		},
	})
	s.envio.AlEscribirYo = func(chat string) { s.alEscribirYo(chat, time.Now()) }
	s.motor.Iniciar(s.ctx)
	s.mods.Iniciar(s.ctx)
	s.iniciarPermisos()
	s.descargas.Iniciar(s.ctx)
	s.fotos.Iniciar(s.ctx)
	s.grupos.iniciar(s.ctx)
	s.iniciarSilencio()
}

// trabajador procesa en orden lo que llega (mensajes, historial, nombres) para no frenar el bucle de eventos de whatsmeow.
func (s *estado) trabajador() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case f := <-s.trabajo:
			func() {
				defer func() {
					if r := recover(); r != nil {
						s.log(fmt.Sprintf("error interno: %v", r))
					}
				}()
				f()
			}()
		}
	}
}

func (s *estado) encola(f func()) {
	select {
	case s.trabajo <- f:
	case <-s.ctx.Done():
	}
}

// Pair pide a WhatsApp un código de 8 caracteres para vincular el número dado
// (con código de país, solo dígitos, sin "+", por ejemplo 525512345678).
// El código llega por Listener.OnPairingCode; los avisos y errores por OnStatus.
func Pair(telefono string) error {
	mu.Lock()
	s := st
	mu.Unlock()
	if s == nil {
		return errors.New("el bot no está iniciado")
	}
	if s.cli.Store.ID != nil {
		return errors.New("este bot ya está vinculado")
	}
	tel := soloDigitos(telefono)
	if len(tel) <= 6 {
		return errors.New("el número es muy corto")
	}
	if tel[0] == '0' {
		return errors.New("el número debe incluir el código de país")
	}
	go s.vincular(tel)
	return nil
}

func (s *estado) vincular(tel string) {
	select { // vacía avisos viejos de QR
	case <-s.qr:
	default:
	}
	if !s.cli.IsConnected() {
		s.log("Conectando para pedir el código…")
		if err := s.cli.Connect(); err != nil {
			s.log("No se pudo conectar: " + err.Error())
			s.status("error: " + err.Error())
			return
		}
		select { // WhatsApp avisa con un evento QR cuando ya se puede pedir el código
		case <-s.qr:
		case <-time.After(20 * time.Second):
			s.log("WhatsApp tardó en responder; se intenta igual.")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	codigo, err := s.cli.PairPhone(ctx, tel, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		s.log("WhatsApp rechazó la solicitud: " + err.Error())
		s.status("error: " + err.Error())
		return
	}
	s.log("Código de vinculación generado para el número " + enmascara(tel) + ".")
	s.status("esperando_codigo")
	s.l.OnPairingCode(formatoCodigo(codigo))
}

// Stop desconecta y cierra la sesión local (la vinculación con WhatsApp se conserva). Los mensajes guardados siguen disponibles.
func Stop() {
	mu.Lock()
	s := st
	st = nil
	mu.Unlock()
	if s == nil {
		return
	}
	s.cerr = true
	s.cancel()
	s.refMu.Lock()
	if s.refresco != nil {
		s.refresco.Stop()
	}
	s.refMu.Unlock()
	s.cli.Disconnect()
	s.motor.Detener()
	s.mods.Detener()
	if s.sil != nil {
		s.sil.Detener()
	}
	olvidarGestoresLibres()
	s.descargas.Detener()
	s.fotos.Detener()
	s.wg.Wait()
	_ = s.cont.Close()
	s.status("detenido")
}

// IsConnected dice si el bot está conectado y vinculado.
func IsConnected() bool {
	mu.Lock()
	s := st
	mu.Unlock()
	return s != nil && s.cli.IsConnected() && s.cli.IsLoggedIn()
}

// Logout quita el dispositivo vinculado en WhatsApp y borra la sesión local. Deja el bot detenido.
// No borra los mensajes guardados: para eso está EliminarBot.
func Logout() error {
	mu.Lock()
	s := st
	mu.Unlock()
	if s == nil {
		return errors.New("el bot no está iniciado")
	}
	var aviso error
	if s.cli.Store.ID != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		err := s.cli.Logout(ctx)
		cancel()
		if err != nil {
			s.log("No se pudo avisar a WhatsApp: " + err.Error())
			aviso = fmt.Errorf("no se pudo cerrar la sesión en WhatsApp (quita el dispositivo a mano en Dispositivos vinculados): %w", err)
			if derr := s.cli.Store.Delete(context.Background()); derr != nil {
				s.log("Tampoco se pudo borrar la sesión local: " + derr.Error())
			}
		} else {
			s.log("Sesión cerrada en WhatsApp.")
		}
	}
	Stop()
	return aviso
}

func (s *estado) evento(e any) {
	switch v := e.(type) {
	case *events.QR:
		select {
		case s.qr <- struct{}{}:
		default:
		}
	case *events.PairSuccess:
		s.log("Vinculado con el número " + enmascara(v.ID.User) + ".")
		s.status("vinculado")
	case *events.PairError:
		s.log("Falló la vinculación: " + v.Error.Error())
		s.status("error: falló la vinculación")
	case *events.Connected:
		s.log("Conectado.")
		s.status("conectado")
		go s.fotos.RevisarVencidas()
		go s.snapshotSiToca(s.ctx, false)
		s.reconciliarLIDsDespues()
	case *events.Disconnected:
		if !s.cerr {
			s.log("Desconectado; se intentará reconectar.")
			s.status("desconectado")
		}
	case *events.LoggedOut:
		s.log("WhatsApp cerró la sesión de este dispositivo.")
		s.status("sesion_cerrada")
		s.desconexionForzada("sesion_cerrada")
	case *events.StreamReplaced:
		s.log("La sesión se abrió en otro lugar.")
		s.status("error: la sesión se abrió en otro lugar")
		s.desconexionForzada("otra_sesion")
	case *events.TemporaryBan:
		s.log("Bloqueo temporal: " + v.String())
		s.status("error: bloqueo temporal de WhatsApp")
		s.desconexionForzada("bloqueo_temporal")
	case *events.ClientOutdated:
		s.log("WhatsApp pide una versión más nueva de la conexión.")
		s.status("error: conexión desactualizada")
		s.desconexionForzada("version_vieja")
	case *events.ConnectFailure:
		s.log(fmt.Sprintf("No se pudo conectar (código %d).", int(v.Reason)))
		s.status(fmt.Sprintf("error: no se pudo conectar (%d)", int(v.Reason)))
	case *events.Message:
		sinc := s.sincronizando.Load() // se mira al llegar, no cuando le toque al trabajador
		s.encola(func() {
			s.capturar(v) // diagnóstico (apagado por defecto): solo estructura, en memoria
			s.ing.Mensaje(s.ctx, v, false)
			s.alSilencio(v, sinc) // silenciar en grupos: solo encola (nada de red aquí)
			s.pausaPorTelefono(v)
			// Votos y toques de nuestros menús interactivos (Laboratorio): se atienden aquí, en el trabajador.
			if !s.alVotoModulo(v) && !s.alLab(v, sinc) {
				s.alMotor(v, sinc)
			}
		})
	case *events.OfflineSyncPreview:
		s.sincronizando.Store(true)
	case *events.OfflineSyncCompleted:
		s.sincronizando.Store(false)
	case *events.UndecryptableMessage:
		// WhatsApp no entrega a los dispositivos vinculados el contenido de algunos "ver una vez": solo queda el aviso.
		if v.IsUnavailable && v.UnavailableType == events.UnavailableTypeViewOnce {
			info := v.Info
			s.encola(func() { s.ing.VerUnaVezSinContenido(s.ctx, info) })
		}
	case *events.CallOffer:
		s.alLlamar(v)
	case *events.Receipt:
		s.alRecibirAcuse(v)
	case *events.ChatPresence:
		s.alRecibirChatPresencia(v)
	case *events.Presence:
		s.alRecibirPresencia(v)
	case *events.HistorySync:
		s.encola(func() {
			n := s.ing.Historial(s.ctx, v.Data, s.cli.ParseWebMessage)
			if n > 0 {
				s.log(fmt.Sprintf("Historial: %d mensajes guardados.", n))
			}
		})
	case *events.Picture:
		s.encola(func() { s.alCambiarFoto(v) })
	case *events.GroupInfo:
		s.alCambiarGrupo(v.JID.ToNonAD().String())
		s.alCambiarMiembros(v)
		if len(v.Join) > 0 {
			s.alEntrarAlGrupo(v)
		}
		if v.Name != nil {
			nombre, jid := v.Name.Name, v.JID.ToNonAD().String()
			s.encola(func() { s.ing.Asunto(jid, nombre) })
		}
	case *events.JoinedGroup:
		nombre, jid := v.GroupName.Name, v.JID.ToNonAD().String()
		s.alCambiarGrupo(jid)
		s.encola(func() { s.ing.Asunto(jid, nombre) })
		s.alUnirseAGrupo(v)
	case *events.PushName:
		s.encola(func() { s.ing.NombrePublico(s.ctx, v.JID, v.NewPushName) })
	case *events.Contact, *events.AppStateSyncComplete:
		s.refrescarNombres()
	}
}

// decisor devuelve quién decide las respuestas: el motor de menús (o el de las pruebas).
func (s *estado) decisor() motor.Decisor {
	if d := decisorActual(); d != nil {
		return d
	}
	return s.autoDec
}

// esPropio dice si un JID es el de esta misma cuenta (teléfono o LID).
func (s *estado) esPropio(j types.JID) bool {
	if s.cli.Store == nil {
		return false
	}
	if s.cli.Store.ID != nil && j.User == s.cli.Store.ID.User {
		return true
	}
	return !s.cli.Store.LID.IsEmpty() && j.User == s.cli.Store.LID.User
}

// pausaPorTelefono: si yo escribo desde mi teléfono en un chat, lo que el bot tenía pendiente para ese mensaje se cancela, la
// conversación de menú queda "intervenida" y, si hay una pausa configurada (por defecto 0 = ninguna), el bot calla ahí unos
// minutos. Lo que mandó esta app o el motor no cuenta (sus IDs se conocen), para que el bot no se cancele ni se pause solo.
func (s *estado) pausaPorTelefono(v *events.Message) {
	if !v.Info.IsFromMe || time.Since(v.Info.Timestamp) > 2*time.Minute {
		return
	}
	c := ingesta.Clasificar(v)
	if c.Omitir || c.Revocar != "" || c.Editar != nil || c.Reaccion != nil {
		return
	}
	chat, ok := s.ing.ChatDe(s.ctx, v.Info.MessageSource)
	if !ok || s.envio.EsEnvioPropio(chat, string(v.Info.ID)) {
		return
	}
	if v.Info.Chat.Server != types.GroupServer && s.esPropio(v.Info.Chat) {
		return // "Mensajes guardados": ahí escribo comandos para el bot, no hablo con un cliente
	}
	s.alEscribirYo(chat, v.Info.Timestamp)
	if min := pausaTrasEscribirMin.Load(); min > 0 {
		_ = alm.PausarBot(s.botID, chat, time.Now().Add(time.Duration(min)*time.Minute).Unix())
		s.log("El bot calla en un chat porque escribiste desde el teléfono.")
	}
}

// alEscribirYo: escribí yo (a mano) en ese chat a la hora ts.
func (s *estado) alEscribirYo(chat string, ts time.Time) {
	if s.motor != nil {
		s.motor.Intervino(chat, ts)
	}
	alm.MarcarIntervencion(s.botID, chat)
}

// refrescarNombres vuelve a leer la agenda cuando deja de haber cambios por unos segundos.
func (s *estado) refrescarNombres() {
	s.refMu.Lock()
	defer s.refMu.Unlock()
	if s.refresco != nil {
		s.refresco.Stop()
	}
	s.refresco = time.AfterFunc(5*time.Second, func() { s.encola(func() { s.ing.RefrescarNombres(s.ctx) }) })
}

func (s *estado) alCambiarFoto(v *events.Picture) {
	j := v.JID.ToNonAD()
	if j.Server == types.HiddenUserServer {
		if pn, err := s.cli.Store.LIDs.GetPNForLID(s.ctx, j); err == nil && !pn.IsEmpty() {
			j = pn.ToNonAD()
		}
	}
	s.fotos.Forzar(j.String())
}

func textoDe(m *waE2E.Message) string {
	if t := m.GetConversation(); t != "" {
		return t
	}
	return m.GetExtendedTextMessage().GetText()
}

// esVerUnaVez: ver ingesta.EsVerUnaVez.
func esVerUnaVez(v *events.Message) bool { return ingesta.EsVerUnaVez(v) }
