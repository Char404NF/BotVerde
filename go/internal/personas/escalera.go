package personas

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/identidad"
	"botchat/internal/silencio"
)

// tipoPorNombre busca un tipo de advertencia por su nombre (sin importar mayúsculas ni acentos).
func (s *Servicio) tipoPorNombre(n string) *almacen.TipoAdv {
	ts, err := s.Alm.ListarTiposAdv(s.BotID, true)
	if err != nil {
		return nil
	}
	n = identidad.Normalizar(n)
	for i := range ts {
		if identidad.Normalizar(ts[i].Nombre) == n {
			return &ts[i]
		}
	}
	return nil
}

// pasoPara escoge el paso de la escalera que toca con la advertencia número `nivel`: el de ese número; si te pasaste del último,
// se repite el último.
func pasoPara(esc []almacen.PasoEsc, nivel int) *almacen.PasoEsc {
	if len(esc) == 0 || nivel < 1 {
		return nil
	}
	var ult *almacen.PasoEsc
	for i := range esc {
		if esc[i].Nivel == nivel {
			return &esc[i]
		}
		if esc[i].Nivel < nivel {
			ult = &esc[i]
		}
	}
	if nivel > esc[len(esc)-1].Nivel {
		return ult
	}
	return nil
}

// AdvertirConTipo pone una advertencia de un tipo (y, opcional, ligada a una regla del grupo) y aplica el paso de la escalera de ese
// tipo que corresponde a cuántas lleva vigentes. Si el tipo tiene escalera, el tope global antiguo (adv_tope) NO se aplica a esta
// advertencia (así no hay doble sanción); si no la tiene, vale el tope de siempre. La expulsión nunca es sola: espera mi confirmación.
func (s *Servicio) AdvertirConTipo(ctx context.Context, actor, objetivo, chat, msgID, motivo string, tipoID, reglaID int64) (*ResultadoAdvertir, error) {
	if err := s.puede(actor, objetivo); err != nil {
		s.registra(actor, "persona.advertir", objetivo, chat, "denegado", "")
		return nil, err
	}
	var tipo *almacen.TipoAdv
	if tipoID > 0 {
		if tipo = s.Alm.TipoAdvPorID(s.BotID, tipoID); tipo == nil {
			return nil, errors.New("ese tipo de advertencia no existe")
		}
		if reglaID == 0 {
			reglaID = tipo.ReglaID
		}
	}
	if reglaID > 0 && s.Alm.ReglaItemPorID(s.BotID, reglaID) == nil {
		reglaID = 0
	}
	ahora := s.ahora()
	_, total, err := s.Alm.AgregarAdvertenciaTipo(s.BotID, objetivo, chat, motivo, s.persona(actor), tipoID, reglaID, ahora)
	if err != nil {
		s.registra(actor, "persona.advertir", objetivo, chat, "error", "")
		return nil, err
	}
	nivel := s.Alm.ContarAdvertenciasTipo(s.BotID, objetivo, tipoID, ahora)
	s.registra(actor, "persona.advertir", objetivo, chat, "ok", fmt.Sprintf("total=%d,tipo=%d,nivel=%d", total, tipoID, nivel))
	r := &ResultadoAdvertir{Total: total, Tope: s.Alm.AjusteBotInt(s.BotID, "adv_tope"), Accion: s.Alm.AjusteBot(s.BotID, "adv_accion"), TipoID: tipoID, ReglaID: reglaID, Nivel: nivel}
	if tipo != nil && len(tipo.Escalera) > 0 {
		r.Tope = 0 // la escalera manda: el tope global no se aplica
		if p := pasoPara(tipo.Escalera, nivel); p != nil {
			s.aplicarPaso(ctx, objetivo, chat, tipo, p, reglaID, r)
		}
		return r, nil
	}
	if r.Tope > 0 && total >= r.Tope {
		r.LlegoAlTope = true
		s.alTope(ctx, objetivo, chat, msgID, r)
	}
	return r, nil
}

func (s *Servicio) textoRegla(reglaID int64) string {
	if reglaID <= 0 {
		return ""
	}
	it := s.Alm.ReglaItemPorID(s.BotID, reglaID)
	if it == nil {
		return ""
	}
	t := strings.TrimSpace(it.Titulo)
	if t == "" {
		t = strings.TrimSpace(it.Texto)
	}
	return fmt.Sprintf("Regla %d: %s", s.Alm.NumeroDeRegla(s.BotID, reglaID), t)
}

func (s *Servicio) aplicarPaso(ctx context.Context, objetivo, chat string, tipo *almacen.TipoAdv, p *almacen.PasoEsc, reglaID int64, r *ResultadoAdvertir) {
	nombre := s.Nombre(objetivo)
	persona := s.persona(objetivo)
	r.Paso = p.Accion
	regla := s.textoRegla(reglaID)
	r.Regla = regla
	esGrupo := strings.HasSuffix(chat, "@g.us")
	titulo := "Advertencias: " + tipo.Nombre
	detalle := fmt.Sprintf("%s lleva %d advertencia(s) de «%s».", nombre, r.Nivel, tipo.Nombre)
	if regla != "" {
		detalle += " " + regla + "."
	}
	solo := func(motivo string) { r.Hecho, r.Motivo = "solo_aviso", motivo }
	opciones := func() (silencio.Opciones, bool) {
		o := silencio.Opciones{Avisar: true, Origen: "escalera"}
		if p.Alcance == almacen.EscAlcanceTodos {
			o.Alcance = almacen.SilTodos
			return o, true
		}
		if !esGrupo {
			return o, false
		}
		o.Alcance, o.Grupos = almacen.SilUno, []string{chat}
		return o, true
	}
	switch p.Accion {
	case almacen.EscAvisar:
		txt := strings.TrimSpace(p.Aviso)
		if txt == "" {
			txt = fmt.Sprintf("Recibiste una advertencia por «%s» (llevas %d).", tipo.Nombre, r.Nivel)
		}
		if regla != "" {
			txt += " " + regla + "."
		}
		r.Hecho = "avisado"
		if s.Avisar != nil {
			if ok, why := s.Avisar(persona, txt); !ok {
				r.Motivo = why
			}
		}
		detalle += " Le avisé."
	case almacen.EscSilenciar, almacen.EscSilencioLargo:
		if s.Sil == nil {
			solo("el silencio de grupos no está disponible")
			break
		}
		o, ok := opciones()
		if !ok {
			solo("solo funciona en grupos")
			break
		}
		if p.Accion == almacen.EscSilenciar {
			o.Duracion = time.Duration(p.Minutos) * time.Minute
		} else {
			min := s.Alm.AjusteBotInt(s.BotID, "sil_largo_min")
			if min < 1 {
				min = 10080
			}
			o.Duracion = time.Duration(min) * time.Minute
		}
		o.Texto = p.Aviso
		if _, err := s.Sil.Crear(almacen.PersonaDueno, persona, o); err != nil {
			solo("no se pudo silenciar")
			break
		}
		r.Hecho = "silenciado"
		detalle += " La silencié en grupos."
	case almacen.EscExpulsar:
		if s.Sil == nil {
			solo("el silencio de grupos no está disponible")
			break
		}
		if !esGrupo {
			solo("solo funciona en grupos")
			break
		}
		id, err := s.Sil.PedirExpulsion("advertencia", persona, chat, titulo, detalle)
		if err != nil {
			solo("no se pudo pedir la confirmación")
			break
		}
		r.Hecho, r.SancionID = "pendiente", id
		s.registra(almacen.PersonaDueno, "persona.escalera_expulsar", objetivo, chat, "pendiente", "")
		return // el aviso con el toque de confirmar ya lo creó PedirExpulsion
	}
	if r.Hecho == "solo_aviso" {
		detalle += " No hice el paso (" + r.Motivo + "): decide tú."
	}
	s.registra(almacen.PersonaDueno, "persona.escalera_"+p.Accion, objetivo, chat, map[bool]string{true: "no_hecho", false: "ok"}[r.Hecho == "solo_aviso"], "")
	if _, err := s.Alm.CrearAvisoDe(s.BotID, "advertencias", chat, persona, titulo, detalle); err == nil {
		s.log("Aviso local: advertencia con escalera.")
	}
}
