package wabot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/grupos"
	"botchat/internal/ingesta"
	"botchat/internal/silencio"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// Silenciar en grupos (v0.6.0, E5). Lo que hace el silencio vive en internal/silencio; aquí se arma dentro del bot, se engancha en el
// trabajador (NUNCA en el manejador de eventos) y se expone a la app.

// accionesSilencio junta lo que el silencio necesita de un administrador: saber si lo soy y expulsar vienen del servicio de grupos;
// borrar el mensaje ajeno va por el envío SIN gastar la ráfaga de mensajes manuales.
type accionesSilencio struct{ s *estado }

func (a accionesSilencio) EsAdmin(ctx context.Context, chat string) (bool, error) {
	if a.s.gru == nil {
		return false, grupos.ErrNoDisponible
	}
	return a.s.gru.EsAdmin(ctx, chat)
}

func (a accionesSilencio) BorrarMensajeAjeno(ctx context.Context, chat, msgID, _ string) error {
	return a.s.envio.BorrarSilencio(ctx, chat, msgID)
}

func (a accionesSilencio) Expulsar(ctx context.Context, chat, persona string) error {
	if a.s.gru == nil {
		return grupos.ErrNoDisponible
	}
	return a.s.gru.Expulsar(ctx, chat, persona)
}

var _ grupos.AccionesGrupo = accionesSilencio{}

// armarSilencio crea el silencio de grupos del bot que se está iniciando y registra sus comandos. Va después de armarGrupos y antes
// de armarPersonas (la escalera de advertencias lo usa).
func (s *estado) armarSilencio() {
	if s.perm == nil || s.cmd == nil {
		return
	}
	v := silencio.Nuevo(alm, s.botID, s.perm)
	v.Log = s.log
	v.Grupo = accionesSilencio{s}
	// Los avisos privados salen por la cola de SALIDA del motor, como respuesta provocada por el mensaje de la persona (no son
	// «iniciativa»): no gastan los topes ini_tope_* y tienen los suyos (sil_*).
	v.Enviar = func(ctx context.Context, chat, texto string) error {
		m := s.motor
		if m == nil {
			return errors.New("el motor no está listo")
		}
		return m.ResponderAvatar(chat, "", texto, "aviso")
	}
	v.RegistrarComandos(s.cmd)
	s.sil = v
}

// iniciarSilencio arranca las tareas de fondo: borrar, avisar y revisar vencimientos cada minuto.
func (s *estado) iniciarSilencio() {
	if s.sil == nil {
		return
	}
	s.sil.Iniciar(s.ctx)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				s.log(fmt.Sprintf("error interno en los vencimientos del silencio: %v", r))
			}
		}()
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-t.C:
				s.sil.Vencer()
			}
		}
	}()
}

// alSilencio es el enganche con cada mensaje que llega: se llama en el trabajador justo después de guardarlo. Solo mira la base
// local y deja el trabajo en la cola propia del silencio; la red (borrar, avisar) va en otras goroutines.
func (s *estado) alSilencio(v *events.Message, sinc bool) {
	if s.sil == nil || sinc {
		return
	}
	c := ingesta.Clasificar(v)
	if c.Omitir || c.Revocar != "" || c.Editar != nil || c.Reaccion != nil {
		return
	}
	if !v.Info.IsGroup && v.Info.Chat.Server != types.GroupServer {
		return
	}
	chat, ok := s.ing.ChatDe(s.ctx, v.Info.MessageSource)
	if !ok {
		return
	}
	s.sil.AlMensaje(silencio.Mensaje{Chat: chat, ID: string(v.Info.ID), Persona: s.ing.Persona(s.ctx, v.Info.Sender), TS: v.Info.Timestamp,
		EsGrupo: true, EsMio: v.Info.IsFromMe, Sinc: sinc})
}

// ---------- servicio sin el bot en marcha ----------

var (
	silLibres   = map[int64]*silencio.Servicio{}
	silLibresMu sync.Mutex
)

// servicioSilencio da el silencio del bot en marcha, o uno sin conexión (ver, editar ajustes y textos, quitar silencios; lo que
// necesita WhatsApp no se puede).
func servicioSilencio(botID int64) (*silencio.Servicio, error) {
	mu.Lock()
	s := st
	a := alm
	mu.Unlock()
	if a == nil {
		return nil, errors.New("el almacén no está abierto")
	}
	if s != nil && s.botID == botID && s.sil != nil {
		return s.sil, nil
	}
	p, _, err := servicioPerm(botID)
	if err != nil {
		return nil, err
	}
	silLibresMu.Lock()
	defer silLibresMu.Unlock()
	if x, ok := silLibres[botID]; ok && x.Alm == a {
		return x, nil
	}
	x := silencio.Nuevo(a, botID, p)
	silLibres[botID] = x
	return x, nil
}

// ---------- API para la app ----------

// SilenciarEnGrupos silencia a la persona en grupos. segundos = 0 es para siempre. alcance: "uno", "varios" o "todos"; gruposJSON es la
// lista de claves de grupo (["...@g.us"]). texto es el aviso propio de este silencio ("" = el de siempre). Devuelve (JSON) el silencio.
func SilenciarEnGrupos(botID int64, persona string, segundos int64, alcance, gruposJSON string, avisar bool, texto string) (string, error) {
	sv, err := servicioSilencio(botID)
	if err != nil {
		return "", err
	}
	var gs []string
	if strings.TrimSpace(gruposJSON) != "" {
		if err := json.Unmarshal([]byte(gruposJSON), &gs); err != nil {
			return "", errors.New("la lista de grupos no es válida")
		}
	}
	if segundos < 0 {
		return "", errors.New("la duración no es válida")
	}
	return js(sv.Silenciar(dueno(), persona, silencio.Opciones{Duracion: time.Duration(segundos) * time.Second, Alcance: alcance, Grupos: gs, Avisar: avisar, Texto: texto}))
}

// QuitarSilencioGrupos termina el silencio de grupos de la persona. Devuelve si lo tenía.
func QuitarSilencioGrupos(botID int64, persona string) (bool, error) {
	sv, err := servicioSilencio(botID)
	if err != nil {
		return false, err
	}
	return sv.QuitarSilencio(dueno(), persona)
}

// ListarSilenciosGrupos devuelve (JSON) los silencios vigentes con el nombre de la persona y su contador de hoy.
func ListarSilenciosGrupos(botID int64) (string, error) {
	sv, err := servicioSilencio(botID)
	if err != nil {
		return "", err
	}
	return js(sv.Listar())
}

// HistorialSilencios devuelve (JSON) los últimos silencios de la persona, también los terminados.
func HistorialSilencios(botID int64, persona string) (string, error) {
	sv, err := servicioSilencio(botID)
	if err != nil {
		return "", err
	}
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	return js(a.HistorialSilencios(botID, sv.Perm.Persona(persona), 50))
}

// ContadorSilencio devuelve (JSON) el contador de hoy de la persona: {usado, tope, color, total, grupos, proximoReinicio…}. grupo "" = el total.
func ContadorSilencio(botID int64, persona, grupo string) (string, error) {
	sv, err := servicioSilencio(botID)
	if err != nil {
		return "", err
	}
	return js(sv.ContadorDe(persona, grupo, time.Now()), nil)
}

// PonerContadorEnCero deja en cero el contador de hoy de la persona (grupo "" = todos sus contadores).
func PonerContadorEnCero(botID int64, persona, grupo string) error {
	sv, err := servicioSilencio(botID)
	if err != nil {
		return err
	}
	return sv.PonerEnCero(dueno(), persona, grupo)
}

// AjustesSilencio devuelve (JSON) {ajustes: {sil_*, adv_caducidad_dias}, textos: {id: {actual, defecto}}}.
func AjustesSilencio(botID int64) (string, error) {
	sv, err := servicioSilencio(botID)
	if err != nil {
		return "", err
	}
	todos := sv.Alm.AjustesBot(botID)
	aj := map[string]string{}
	for k, v := range todos {
		if strings.HasPrefix(k, "sil_") || k == "adv_caducidad_dias" {
			aj[k] = v
		}
	}
	return js(map[string]any{"ajustes": aj, "textos": sv.Textos()}, nil)
}

// FijarAjusteSilencio cambia un ajuste sil_* (o adv_caducidad_dias) con validación.
func FijarAjusteSilencio(botID int64, clave, valor string) error {
	if !strings.HasPrefix(clave, "sil_") && clave != "adv_caducidad_dias" {
		return errors.New("ese ajuste no es del silencio")
	}
	return FijarAjusteDePermisos(botID, clave, valor)
}

// FijarTextoSilencio cambia un texto de aviso (aviso, inicio, fin, noadmin, tope). "" vuelve al de fábrica.
func FijarTextoSilencio(botID int64, id, texto string) error {
	sv, err := servicioSilencio(botID)
	if err != nil {
		return err
	}
	if err := sv.FijarTexto(id, texto); err != nil {
		return err
	}
	accionDueno(botID, "ajuste.sil_texto_"+id, "", "")
	return nil
}

// SancionesPendientes devuelve (JSON) las expulsiones que esperan mi confirmación (del silencio y de la escalera de advertencias),
// con el nombre de la persona y del grupo.
func SancionesPendientes(botID int64) (string, error) {
	sv, err := servicioSilencio(botID)
	if err != nil {
		return "", err
	}
	ps, err := sv.SancionesPendientes()
	if err != nil {
		return "", err
	}
	type fila struct {
		almacen.SancionPend
		Nombre string `json:"nombre"`
		NGrupo string `json:"nombreGrupo"`
	}
	out := make([]fila, 0, len(ps))
	for _, p := range ps {
		f := fila{SancionPend: p, Nombre: sv.Alm.NombreDePersona(botID, p.Persona)}
		if c, _ := sv.Alm.ObtenerChat(botID, p.Grupo); c != nil {
			f.NGrupo = c.Mostrado
		}
		out = append(out, f)
	}
	return js(out, nil)
}

// ConfirmarSancionSilencio responde a una sanción pendiente con un toque: si=true la ejecuta (expulsar, solo siendo administrador),
// si=false la descarta. Devuelve un texto corto con lo que pasó.
func ConfirmarSancionSilencio(botID, id int64, si bool) (string, error) {
	sv, err := servicioSilencio(botID)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return sv.ConfirmarSancion(ctx, id, si)
}

// ConfirmarSancion es lo mismo para las expulsiones que pide la escalera de advertencias.
func ConfirmarSancion(botID, id int64, si bool) (string, error) {
	return ConfirmarSancionSilencio(botID, id, si)
}

// GrupoAdmin es un grupo para el selector de alcance.
type grupoAdminVista struct {
	JID      string `json:"jid"`
	Nombre   string `json:"nombre"`
	Miembros int    `json:"miembros"`
	SoyAdmin bool   `json:"soyAdmin"`
	Sabido   bool   `json:"sabido"` // false = no se pudo saber (el bot no está encendido o no hay lista de miembros)
}

// GruposQueAdministro devuelve (JSON) los grupos donde está la cuenta, con la bandera de si es administradora, para escoger dónde
// silenciar. Sale de la lista local de miembros (no usa la red).
func GruposQueAdministro(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	mu.Lock()
	s := st
	mu.Unlock()
	propios := []string{}
	if s != nil && s.botID == botID && s.cli != nil && s.cli.Store != nil {
		if s.cli.Store.ID != nil {
			propios = append(propios, s.cli.Store.ID.User)
		}
		if !s.cli.Store.LID.IsEmpty() {
			propios = append(propios, s.cli.Store.LID.User)
		}
	}
	gs, err := a.GruposConMiembros(botID, propios)
	if err != nil {
		return "", err
	}
	out := make([]grupoAdminVista, 0, len(gs))
	for _, g := range gs {
		v := grupoAdminVista{JID: g.Grupo, Miembros: g.Miembros, SoyAdmin: g.SoyAdmin, Sabido: len(propios) > 0}
		if c, _ := a.ObtenerChat(botID, g.Grupo); c != nil {
			v.Nombre = c.Mostrado
		}
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SoyAdmin != out[j].SoyAdmin {
			return out[i].SoyAdmin
		}
		return strings.ToLower(out[i].Nombre) < strings.ToLower(out[j].Nombre)
	})
	return js(out, nil)
}
