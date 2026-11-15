package wabot

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"botchat/internal/almacen"

	"go.mau.fi/whatsmeow/types"
)

// Alertas: los avisos importantes (reportes, raid, conexión, resumen semanal…) además de la notificación del teléfono pueden llegar al
// chat «Mensajes guardados» del dueño por WhatsApp. Está apagado por defecto y tiene un tope por hora para no llenar el chat.

const alertasPorHora = 12

var (
	alertaMu    sync.Mutex
	alertaMarca []time.Time
)

// armarAlertas conecta los avisos locales con el WhatsApp del dueño y arranca el resumen semanal.
func (s *estado) armarAlertas() {
	alm.AlAviso = func(botID int64, tipo, chat, titulo, detalle string) {
		if botID != s.botID || s.ctx.Err() != nil {
			return
		}
		s.alertaWA(tipo, titulo, detalle)
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				s.log("error interno en el resumen semanal")
			}
		}()
		t := time.NewTicker(10 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-t.C:
			}
			if s.gru != nil && s.gru.ResumenSemanalToca(time.Now()) {
				s.gru.EnviarResumenSemana(time.Now())
			}
		}
	}()
}

// tipoAlertable dice si ese tipo de aviso está en la lista de los que se repiten por WhatsApp.
func tipoAlertable(lista, tipo string) bool {
	for _, t := range strings.Split(lista, ",") {
		if strings.TrimSpace(t) == tipo {
			return true
		}
	}
	return false
}

func tomarAlerta(ahora time.Time) bool {
	alertaMu.Lock()
	defer alertaMu.Unlock()
	n := alertaMarca[:0]
	for _, t := range alertaMarca {
		if ahora.Sub(t) < time.Hour {
			n = append(n, t)
		}
	}
	alertaMarca = n
	if len(alertaMarca) >= alertasPorHora {
		return false
	}
	alertaMarca = append(alertaMarca, ahora)
	return true
}

// alertaWA manda el aviso al chat conmigo mismo si el dueño lo pidió.
func (s *estado) alertaWA(tipo, titulo, detalle string) {
	if alm.AjusteBot(s.botID, "alerta_wa") != "1" || !tipoAlertable(alm.AjusteBot(s.botID, "alerta_wa_tipos"), tipo) || !tomarAlerta(time.Now()) {
		return
	}
	if s.cli == nil || s.cli.Store == nil || s.cli.Store.ID == nil || !s.cli.IsConnected() || s.envio == nil {
		return
	}
	yo := s.cli.Store.ID.ToNonAD()
	chat, ok := s.ing.ChatDe(s.ctx, types.MessageSource{Chat: yo, Sender: yo, IsFromMe: true})
	if !ok {
		return
	}
	if c, _ := alm.ObtenerChat(s.botID, chat); c == nil {
		return // aún no hay chat conmigo mismo: no se inventa uno
	}
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()
	texto := "🔔 " + titulo
	if strings.TrimSpace(detalle) != "" {
		texto += "\n" + detalle
	}
	if _, err := s.envio.EnviarAuto(ctx, chat, texto, "", nil); err != nil {
		s.log("No se pudo mandar la alerta por WhatsApp.")
	}
}

// AlertasAjustes son las opciones de aviso (para la pantalla).
type AlertasAjustes struct {
	WA          bool     `json:"wa"`          // repetir los avisos importantes por WhatsApp
	Tipos       []string `json:"tipos"`       // cuáles
	Desconexion int      `json:"desconexion"` // minutos sin conexión antes de avisar (0 = no)
	Resumen     bool     `json:"resumen"`     // resumen de la semana
}

// AlertasLeer devuelve (JSON) las opciones de aviso.
func AlertasLeer(botID int64) (string, error) {
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	tipos := []string{}
	for _, t := range strings.Split(a.AjusteBot(botID, "alerta_wa_tipos"), ",") {
		if t = strings.TrimSpace(t); t != "" {
			tipos = append(tipos, t)
		}
	}
	return js(AlertasAjustes{WA: a.AjusteBot(botID, "alerta_wa") == "1", Tipos: tipos, Desconexion: a.AjusteBotInt(botID, "aviso_desconexion"),
		Resumen: a.AjusteBot(botID, "resumen_semanal") == "1"}, nil)
}

var tiposAlerta = map[string]bool{"reporte": true, "grupos": true, "conexion": true, "resumen": true, "filtros": true, "advertencias": true}

// AlertasGuardar guarda las opciones de aviso (JSON con el mismo formato que AlertasLeer).
func AlertasGuardar(botID int64, ajustesJSON string) error {
	a, err := almacenAbierto()
	if err != nil {
		return err
	}
	var x AlertasAjustes
	if err := json.Unmarshal([]byte(ajustesJSON), &x); err != nil {
		return errors.New("las opciones no son válidas")
	}
	if x.Desconexion < 0 || x.Desconexion > 240 {
		return errors.New("los minutos sin conexión van de 0 a 240")
	}
	var ok []string
	for _, t := range x.Tipos {
		if tiposAlerta[t] {
			ok = append(ok, t)
		}
	}
	b := func(v bool) string {
		if v {
			return "1"
		}
		return "0"
	}
	for k, v := range map[string]string{"alerta_wa": b(x.WA), "alerta_wa_tipos": strings.Join(ok, ","), "aviso_desconexion": strconv.Itoa(x.Desconexion), "resumen_semanal": b(x.Resumen)} {
		if err := a.FijarAjusteBot(botID, k, v); err != nil {
			return err
		}
	}
	a.RegistrarAccion(botID, almacen.PersonaDueno, "ajustes.alertas", "", "", "ok", "")
	return nil
}

// ResumenSemanalAhora crea ya el aviso con el resumen de la semana (para probarlo) y devuelve su texto; "" si no hubo nada que contar.
func ResumenSemanalAhora(botID int64) (string, error) {
	g, err := servicioGrupos(botID)
	if err != nil {
		return "", err
	}
	t, ok := g.TextoResumenSemana(time.Now())
	if !ok {
		return "", nil
	}
	g.EnviarResumenSemana(time.Now())
	return t, nil
}
