package grupos

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"botchat/internal/almacen"
)

// Configuración base: la forma en que quieres cuidar tus grupos, guardada una vez (copiada de un grupo que ya dejaste a tu gusto)
// y aplicable a varios grupos a la vez o a cada grupo nuevo. Se guarda cifrada. Las personas, advertencias y baneados no entran.

// ErrSinBase se devuelve al aplicar la base si todavía no hay una guardada.
var ErrSinBase = errors.New("todavía no guardaste una configuración base: elige un grupo y úsalo como base")

// Base es la configuración base.
type Base struct {
	Desde   string                    `json:"desde"`   // nombre del grupo del que se copió
	Fecha   int64                     `json:"fecha"`   // cuándo se guardó (unix)
	Filtros []almacen.FiltroGrupo     `json:"filtros"` // filtros tal como estaban (sin el grupo)
	Reglas  string                    `json:"reglas"`
	Horario almacen.HorarioGrupo      `json:"horario"`
	Entrada almacen.ProteccionEntrada `json:"entrada"`
}

// InfoBase es lo que se muestra de la base (sin su contenido).
type InfoBase struct {
	Existe  bool   `json:"existe"`
	Desde   string `json:"desde"`
	Fecha   int64  `json:"fecha"`
	Filtros int    `json:"filtros"` // cuántos filtros quedan encendidos
	Reglas  bool   `json:"reglas"`  // si trae reglas escritas
	Horario bool   `json:"horario"` // si trae horario de cierre
	Entrada bool   `json:"entrada"` // si trae protección de entrada
	Auto    bool   `json:"auto"`    // se aplica sola a los grupos nuevos
}

func (s *Servicio) rutaBase() string { return fmt.Sprintf("config/grupo_base_%d.json", s.BotID) }

func (s *Servicio) leerBase() (*Base, error) {
	raw, err := s.Alm.LeerCifrado(s.rutaBase())
	if err != nil {
		return nil, ErrSinBase
	}
	var b Base
	if json.Unmarshal(raw, &b) != nil {
		return nil, ErrSinBase
	}
	return &b, nil
}

// InfoBase dice si hay base y qué trae.
func (s *Servicio) InfoBase() InfoBase {
	out := InfoBase{Auto: s.Alm.AjusteBot(s.BotID, "grupo_base_auto") == "1"}
	b, err := s.leerBase()
	if err != nil {
		return out
	}
	out.Existe, out.Desde, out.Fecha = true, b.Desde, b.Fecha
	for _, f := range b.Filtros {
		if f.Activo {
			out.Filtros++
		}
	}
	out.Reglas = strings.TrimSpace(b.Reglas) != ""
	out.Horario = b.Horario.Activo
	out.Entrada = b.Entrada.RaidActivo || len(b.Entrada.Prefijos) > 0
	return out
}

// FijarBaseAuto enciende o apaga aplicar la base a cada grupo nuevo.
func (s *Servicio) FijarBaseAuto(actor string, si bool) error {
	v := "0"
	if si {
		v = "1"
	}
	if si {
		if _, err := s.leerBase(); err != nil {
			return err
		}
	}
	s.registra(actor, "grupo.base_auto", "", "", "ok", "activo="+v)
	return s.Alm.FijarAjusteBot(s.BotID, "grupo_base_auto", v)
}

// GuardarBase copia la configuración de ese grupo como base.
func (s *Servicio) GuardarBase(actor, chat string) error {
	if _, err := parseaGrupo(chat); err != nil {
		return err
	}
	fs, err := s.Alm.FiltrosDeGrupo(s.BotID, chat)
	if err != nil {
		return err
	}
	b := Base{Desde: s.nombreGrupo(chat), Fecha: s.ahora().Unix(), Filtros: []almacen.FiltroGrupo{}, Reglas: s.Alm.ReglasGrupo(s.BotID, chat),
		Horario: s.Alm.HorarioDe(s.BotID, chat), Entrada: s.Alm.EntradaDe(s.BotID, chat)}
	for _, f := range fs {
		f.Chat = ""
		b.Filtros = append(b.Filtros, f)
	}
	b.Horario.Ultimo = ""
	raw, _ := json.Marshal(b)
	if err := s.Alm.EscribirCifrado(s.rutaBase(), raw); err != nil {
		s.registra(actor, "grupo.base", "", chat, "error", "")
		return err
	}
	s.registra(actor, "grupo.base", "", chat, "ok", "")
	return nil
}

// QuitarBase borra la base guardada (y deja de aplicarla a grupos nuevos).
func (s *Servicio) QuitarBase(actor string) error {
	_ = s.Alm.FijarAjusteBot(s.BotID, "grupo_base_auto", "0")
	if err := s.Alm.BorrarCifrado(s.rutaBase()); err != nil {
		return err
	}
	s.registra(actor, "grupo.base_quitar", "", "", "ok", "")
	return nil
}

// AplicarBase pone la base en cada grupo de la lista y dice a cuántos se la puso. Lo que el grupo tenía se reemplaza (filtros, reglas,
// horario y protección de entrada); sus personas, advertencias y baneados no se tocan.
func (s *Servicio) AplicarBase(actor string, chats []string) (int, error) {
	b, err := s.leerBase()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, chat := range chats {
		if _, err := parseaGrupo(chat); err != nil {
			continue
		}
		if err := s.aplicaBaseA(b, chat); err != nil {
			s.registra(actor, "grupo.base_aplicar", "", chat, "error", "")
			continue
		}
		s.registra(actor, "grupo.base_aplicar", "", chat, "ok", "")
		n++
	}
	if n == 0 && len(chats) > 0 {
		return 0, errors.New("no se pudo aplicar a ningún grupo")
	}
	return n, nil
}

func (s *Servicio) aplicaBaseA(b *Base, chat string) error {
	ahora := s.ahora()
	for _, f := range b.Filtros {
		f.Chat = chat
		if err := s.Alm.GuardarFiltroGrupo(s.BotID, f, ahora); err != nil {
			return err
		}
	}
	s.mu.Lock()
	delete(s.filt, chat) // que el filtro nuevo valga ya, no hasta que caduque la caché
	s.mu.Unlock()
	if err := s.Alm.FijarReglasGrupo(s.BotID, chat, b.Reglas, ahora); err != nil {
		return err
	}
	h := b.Horario
	h.Grupo, h.Ultimo = chat, ""
	if err := s.Alm.GuardarHorario(s.BotID, h); err != nil {
		return err
	}
	e := b.Entrada
	e.Grupo = chat
	return s.Alm.GuardarEntrada(s.BotID, e)
}

// AlGrupoNuevo aplica la base a un grupo al que el bot acaba de entrar, si está pedido y el grupo no tiene nada configurado todavía.
// Devuelve si lo hizo.
func (s *Servicio) AlGrupoNuevo(chat string) bool {
	if s.Alm.AjusteBot(s.BotID, "grupo_base_auto") != "1" {
		return false
	}
	if fs, _ := s.Alm.FiltrosDeGrupo(s.BotID, chat); len(fs) > 0 || s.Alm.ReglasGrupo(s.BotID, chat) != "" {
		return false // ya tiene su propia configuración
	}
	n, err := s.AplicarBase("auto", []string{chat})
	if err == nil && n == 1 {
		_, _ = s.Alm.CrearAvisoDe(s.BotID, "grupos", chat, "", "Configuración base aplicada", "Entré a "+s.nombreGrupo(chat)+" y le puse tu configuración base.")
		return true
	}
	return false
}

var _ = time.Second
