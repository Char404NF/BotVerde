package wabot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/auto"
	"botchat/internal/flujo"
	"botchat/internal/motor"
)

// ---------- armado dentro del bot ----------

// armarAuto crea el decisor de mensajes automáticos (envuelve al de menús) y los atajos. Va después de armarPermisos.
func (s *estado) armarAuto() {
	cfg := auto.DecisorConfig{
		BotID: s.botID, Alm: alm, Base: s.flujo, Zona: alm.Zona, Log: s.log,
		Nombre: func(chat, sender string) string { return nombreDe(s.botID, chat, sender) },
	}
	if s.perm != nil {
		cfg.Persona = s.perm.Persona
		cfg.Conocido = s.perm.EsConocido
	}
	s.autoDec = auto.Nuevo(cfg)
	s.atajos = auto.NuevosAtajos(auto.AtajosConfig{BotID: s.botID, Alm: alm, Dec: s.autoDec, Envio: envioAtajo{s}, Log: s.log})
}

// envioAtajo manda los atajos por el servicio de envío.
type envioAtajo struct{ s *estado }

func (e envioAtajo) Editar(ctx context.Context, chat, id, texto string) error {
	return e.s.envio.Editar(ctx, chat, id, texto)
}

func (e envioAtajo) EnviarTexto(ctx context.Context, chat, texto string) (string, error) {
	r, err := e.s.envio.EnviarAuto(ctx, chat, texto, "", nil)
	if err != nil {
		return "", err
	}
	if r.Estado != "enviado" {
		return "", errNoEnviado
	}
	return r.ID, nil
}

func (e envioAtajo) EnviarMedio(ctx context.Context, chat string, m motor.Medio) error {
	ruta, err := e.s.copiaAdjunto(m.ID)
	if err != nil {
		return err
	}
	r, err := e.s.envio.EnviarMedioAutoCita(ctx, chat, adjuntoEnvio(ruta, &m, ""), "", "", nil)
	if err != nil {
		return err
	}
	if r.Estado != "enviado" {
		return errNoEnviado
	}
	return nil
}

// ---------- vistas ----------

type autoVista struct {
	ID       int64                `json:"id"`
	Tipo     string               `json:"tipo"`
	Nombre   string               `json:"nombre"`
	Texto    string               `json:"texto"`
	Config   auto.Config          `json:"config"`
	Adjuntos []almacen.AdjuntoRef `json:"adjuntos"`
	Activo   bool                 `json:"activo"`
	Orden    int                  `json:"orden"`
	Cuando   string               `json:"cuando"`
	Quien    string               `json:"quien"`
	Repite   string               `json:"repite"`
	Envios7d int                  `json:"envios7d"`
	Estado   string               `json:"estado"` // "activo", "apagado" o un motivo por el que no se manda ("sin_archivo")
}

func vistaDeAuto(a *almacen.Almacen, botID int64, m almacen.AutoMensaje) autoVista {
	c := auto.LeerConfig(m.Tipo, m.Config)
	v := autoVista{ID: m.ID, Tipo: m.Tipo, Nombre: m.Nombre, Texto: m.Texto, Config: c, Adjuntos: m.Adjuntos, Activo: m.Activo, Orden: m.Orden,
		Cuando: auto.ResumenCuando(c), Quien: auto.ResumenQuien(c), Repite: auto.ResumenRepeticion(m.Tipo, c),
		Envios7d: a.ContarAutoRegistro(botID, m.ID, time.Now().AddDate(0, 0, -7)), Estado: "activo"}
	if v.Adjuntos == nil {
		v.Adjuntos = []almacen.AdjuntoRef{}
	}
	if !m.Activo {
		v.Estado = "apagado"
	}
	for _, x := range m.Adjuntos {
		if !a.ExisteAdjunto(botID, x.ID) {
			v.Estado = "sin_archivo"
		}
	}
	if m.Tipo == almacen.AutoAtajo {
		v.Cuando, v.Quien, v.Repite = "", "", ""
	}
	return v
}

type autoLista struct {
	Mensajes    []autoVista `json:"mensajes"`
	Activo      bool        `json:"activo"` // interruptor general
	AtajosOn    bool        `json:"atajosActivo"`
	AtajoModo   string      `json:"atajoModo"`
	TopeHora    int         `json:"topeHora"`
	UsadosHora  int         `json:"usadosHora"`
	Zona        string      `json:"zona"`
	ZonaBot     string      `json:"zonaBot"`
	Conflictos  []string    `json:"conflictos"` // reglas que contestan a todo y se llevan los mensajes automáticos
	Filtrados24 int         `json:"automaticos24h"`
}

// ListarAutos devuelve (JSON) los mensajes automáticos de un tipo ("" = todos) con su estado y los ajustes generales.
func ListarAutos(botID int64, tipo string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	ms, err := a.ListarAutos(botID, tipo)
	if err != nil {
		return "", err
	}
	out := autoLista{Mensajes: []autoVista{}, Activo: a.AjusteBot(botID, "auto_activo") != "0", AtajosOn: a.AjusteBot(botID, "atajos_activo") != "0",
		AtajoModo: a.AjusteBot(botID, "atajo_modo"), TopeHora: a.AjusteBotInt(botID, "auto_tope_hora"), Zona: a.AjusteBot(botID, "auto_zona"),
		ZonaBot: a.Zona().String(), Conflictos: []string{}}
	for _, m := range ms {
		out.Mensajes = append(out.Mensajes, vistaDeAuto(a, botID, m))
	}
	out.UsadosHora = a.ContarAutoEnvios(botID, time.Now().Add(-time.Hour))
	out.Filtrados24 = a.ContarMensajesAuto(botID, "", time.Now().Add(-24*time.Hour))
	if menus, err := a.ListarMenus(botID); err == nil {
		for _, mn := range menus {
			if !mn.Activo {
				continue
			}
			if d, err := flujo.Parse(mn.Def); err == nil && d.Propio == nil && d.Disparadores.Cualquiera && d.Alcance.Privados {
				out.Conflictos = append(out.Conflictos, d.Nombre)
			}
		}
	}
	return js(out, nil)
}

// ObtenerAuto devuelve (JSON) un mensaje automático.
func ObtenerAuto(botID, id int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	m, err := a.ObtenerAuto(botID, id)
	if err != nil {
		return "", err
	}
	if m == nil {
		return "", errors.New("Ese mensaje ya no existe.")
	}
	return js(vistaDeAuto(a, botID, *m), nil)
}

type autoEntrada struct {
	ID       int64                `json:"id"`
	Tipo     string               `json:"tipo"`
	Nombre   string               `json:"nombre"`
	Texto    string               `json:"texto"`
	Config   auto.Config          `json:"config"`
	Adjuntos []almacen.AdjuntoRef `json:"adjuntos"`
	Activo   bool                 `json:"activo"`
}

func (e autoEntrada) aMensaje() (almacen.AutoMensaje, auto.Config) {
	return almacen.AutoMensaje{ID: e.ID, Tipo: e.Tipo, Nombre: e.Nombre, Texto: e.Texto, Adjuntos: e.Adjuntos, Activo: e.Activo}, e.Config
}

// GuardarAuto crea (id 0) o cambia un mensaje automático. Recibe (JSON) {id, tipo, nombre, texto, config, adjuntos, activo}. Valida todo
// y devuelve el id. Los archivos que ya no usa se borran del teléfono.
func GuardarAuto(botID int64, entradaJSON string) (int64, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	var e autoEntrada
	if err := json.Unmarshal([]byte(entradaJSON), &e); err != nil {
		return 0, errors.New("El mensaje no es válido.")
	}
	m, cfg := e.aMensaje()
	if m.ID != 0 {
		viejo, err := a.ObtenerAuto(botID, m.ID)
		if err != nil || viejo == nil {
			return 0, errors.New("Ese mensaje ya no existe.")
		}
		m.Tipo = viejo.Tipo // el tipo no cambia
	}
	if err := auto.Validar(&m, &cfg); err != nil {
		return 0, err
	}
	for i, ad := range m.Adjuntos {
		if !a.ExisteAdjunto(botID, ad.ID) {
			return 0, fmt.Errorf("El archivo %d ya no está guardado. Quítalo y vuelve a agregarlo.", i+1)
		}
	}
	if m.Tipo == almacen.AutoAtajo {
		if otros, _ := a.ListarAutos(botID, almacen.AutoAtajo); otros != nil {
			for _, o := range otros {
				if o.ID != m.ID && strings.EqualFold(o.Nombre, m.Nombre) {
					return 0, fmt.Errorf("Ya tienes un atajo llamado /%s.", m.Nombre)
				}
			}
		}
	}
	m.Config = cfg.JSON()
	if m.ID == 0 {
		id, err := a.CrearAuto(botID, &m)
		if err != nil {
			return 0, err
		}
		a.RegistrarAccion(botID, almacen.PersonaDueno, "auto.crear", "", "", "ok", "tipo="+m.Tipo)
		return id, nil
	}
	if err := a.GuardarAuto(botID, &m); err != nil {
		return 0, err
	}
	a.RegistrarAccion(botID, almacen.PersonaDueno, "auto.editar", "", "", "ok", "tipo="+m.Tipo)
	return m.ID, nil
}

// FijarAutoActivo enciende o apaga un mensaje automático.
func FijarAutoActivo(botID, id int64, activo bool) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	if err := a.FijarAutoActivo(botID, id, activo); err != nil {
		return err
	}
	a.RegistrarAccion(botID, almacen.PersonaDueno, "auto.interruptor", "", "", "ok", fmt.Sprintf("activo=%t", activo))
	return nil
}

// BorrarAuto quita un mensaje automático con sus archivos guardados.
func BorrarAuto(botID, id int64) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	if err := a.BorrarAuto(botID, id); err != nil {
		return err
	}
	a.RegistrarAccion(botID, almacen.PersonaDueno, "auto.borrar", "", "", "ok", "")
	return nil
}

// ReordenarAutos fija el orden (arreglo JSON de ids).
func ReordenarAutos(botID int64, idsJSON string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	var ids []int64
	if err := json.Unmarshal([]byte(idsJSON), &ids); err != nil {
		return errors.New("La lista no es válida.")
	}
	return a.ReordenarAutos(botID, ids)
}

// PlantillasAuto devuelve (JSON) las plantillas listas por tipo.
func PlantillasAuto() (string, error) {
	return js(auto.Plantillas(time.Now()), nil)
}

// ConfigPorDefectoAuto devuelve (JSON) la configuración con la que nace un mensaje de ese tipo.
func ConfigPorDefectoAuto(tipo string) (string, error) {
	return js(auto.ConfigPorDefecto(tipo), nil)
}

// ProbarAuto prueba un mensaje (sin guardarlo ni mandar nada a WhatsApp) con una persona y un momento falsos. Recibe (JSON) el mensaje,
// igual que GuardarAuto, y (JSON) el escenario {nombre, esGrupo, conocida, primera, silencio, cuando, persona}.
func ProbarAuto(botID int64, entradaJSON, escenarioJSON string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	var e autoEntrada
	if err := json.Unmarshal([]byte(entradaJSON), &e); err != nil {
		return "", errors.New("El mensaje no es válido.")
	}
	var esc auto.Escenario
	if strings.TrimSpace(escenarioJSON) != "" {
		_ = json.Unmarshal([]byte(escenarioJSON), &esc)
	}
	m, cfg := e.aMensaje()
	if err := auto.Validar(&m, &cfg); err != nil {
		return "", err
	}
	dec := auto.Nuevo(auto.DecisorConfig{BotID: botID, Alm: a, Zona: a.Zona})
	return js(dec.Probar(m, cfg, esc), nil)
}

type registroVista struct {
	almacen.AutoEnvio
	Texto string `json:"texto"`
}

// RegistroAuto devuelve (JSON) los últimos envíos automáticos (de un mensaje o, con 0, de todos), sin textos.
func RegistroAuto(botID, autoID int64, limite int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	l, err := a.ListarAutoRegistro(botID, autoID, limite)
	if err != nil {
		return "", err
	}
	out := make([]registroVista, 0, len(l))
	for _, x := range l {
		out = append(out, registroVista{AutoEnvio: x, Texto: almacen.DescribeAutoEnvio(x)})
	}
	return js(out, nil)
}

// ---------- adjuntos (un solo servicio para mensajes automáticos, menús y comandos propios) ----------

// SubirAdjunto valida y guarda cifrado un archivo (la copia en claro de `ruta` se borra al terminar). tipo: imagen, video, documento o
// audio (nota de voz: .ogg, Opus, mono). Devuelve (JSON) {id, tipo, mime, nombre, bytes}.
func SubirAdjunto(botID int64, ruta, tipo, mime, nombre string) (string, error) {
	defer func() { _ = os.Remove(ruta) }()
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	r, err := a.GuardarAdjuntoDeArchivo(botID, ruta, tipo, mime, nombre)
	if err != nil {
		return "", err
	}
	return js(r, nil)
}

// BorrarAdjunto quita un archivo guardado si nada lo usa (por ejemplo, cuando se quita del editor un archivo que se acababa de subir).
func BorrarAdjunto(botID int64, id string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	if !almacen.IDAdjuntoValido(id) {
		return errors.New("adjunto no válido")
	}
	if a.ReferenciasAdjuntos(botID)[id] {
		return nil // todavía lo usa algo
	}
	a.BorrarAdjuntoMenu(botID, id)
	return nil
}

// LimpiarAdjuntos borra los archivos guardados que ya nada usa (los de menos de una hora se respetan). Devuelve cuántos borró.
func LimpiarAdjuntos(botID int64) (int, error) {
	a, err := almacenAbierto()
	if err != nil {
		return 0, err
	}
	return a.LimpiarAdjuntosHuerfanos(botID, time.Hour, time.Now()), nil
}

// ---------- atajos desde la app ----------

type atajoVista struct {
	ID       int64  `json:"id"`
	Nombre   string `json:"nombre"`
	Texto    string `json:"texto"` // ya con las variables puestas para ese chat
	Archivos int    `json:"archivos"`
}

// ListarAtajos devuelve (JSON) los atajos activos para el selector de la caja de mensaje, con el texto ya puesto para ese chat.
func ListarAtajos(botID int64, chat string) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	out := []atajoVista{}
	if a.AjusteBot(botID, "atajos_activo") == "0" {
		return js(out, nil)
	}
	ms, err := a.ListarAutos(botID, almacen.AutoAtajo)
	if err != nil {
		return "", err
	}
	dec := auto.Nuevo(auto.DecisorConfig{BotID: botID, Alm: a, Zona: a.Zona, Nombre: func(c, s string) string { return nombreDe(botID, c, s) }})
	for _, m := range ms {
		if !m.Activo {
			continue
		}
		persona := chat
		if strings.HasSuffix(chat, "@g.us") {
			persona = ""
		}
		out = append(out, atajoVista{ID: m.ID, Nombre: m.Nombre, Texto: dec.Render(m.Texto, auto.Config{}, chat, persona, time.Now()), Archivos: len(m.Adjuntos)})
	}
	return js(out, nil)
}

// EnviarArchivosAtajo manda los archivos de un atajo al chat (el texto lo manda la persona desde la caja de mensaje). Es envío manual:
// lo pido yo desde la app.
func EnviarArchivosAtajo(botID int64, chat string, id int64) error {
	mu.Lock()
	s := st
	mu.Unlock()
	if s == nil || s.botID != botID {
		return errors.New("el bot no está iniciado")
	}
	m, err := alm.ObtenerAuto(botID, id)
	if err != nil || m == nil || m.Tipo != almacen.AutoAtajo {
		return errors.New("Ese atajo ya no existe.")
	}
	ctx, cancel := context.WithTimeout(s.ctx, 3*time.Minute)
	defer cancel()
	fallo := 0
	for i, ad := range m.Adjuntos {
		if i >= almacen.MaxAdjuntos {
			break
		}
		if i > 0 {
			time.Sleep(time.Second + time.Duration(time.Now().UnixNano()%int64(2*time.Second)))
		}
		ruta, err := s.copiaAdjunto(ad.ID)
		if err != nil {
			fallo++
			continue
		}
		a := adjuntoEnvio(ruta, &motor.Medio{ID: ad.ID, Tipo: ad.Tipo, Mime: ad.Mime, Nombre: ad.Nombre}, "")
		if _, err := s.envio.EnviarMedio(ctx, chat, a, ""); err != nil {
			fallo++
		}
	}
	if fallo > 0 {
		return fmt.Errorf("No se pudieron mandar %d archivo(s).", fallo)
	}
	return nil
}

// ---------- ver los mensajes automáticos ----------

// ListarMensajesAuto devuelve (JSON) los mensajes que mandó una automatización, de un chat ("" = de todos), del más nuevo al más viejo.
func ListarMensajesAuto(botID int64, chat string, antesTS int64, limite int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.ListarMensajesAuto(botID, chat, antesTS, limite))
}

// ChatsConAutomaticos devuelve (JSON) las claves de los chats que tienen mensajes automáticos de los últimos `dias` días.
func ChatsConAutomaticos(botID int64, dias int) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	if dias <= 0 {
		dias = 7
	}
	l := a.ChatsConAutomaticos(botID, time.Now().AddDate(0, 0, -dias))
	if l == nil {
		l = []string{}
	}
	return js(l, nil)
}
