package grupos

import (
	"fmt"
	"strings"
	"time"
)

// TextoResumenSemana arma el resumen de los últimos 7 días de todos los grupos: mensajes, los más activos, quién entró y salió y lo que
// hicieron los filtros. No incluye textos de mensajes ni números de teléfono. Devuelve también si hubo algo que contar.
func (s *Servicio) TextoResumenSemana(ahora time.Time) (string, bool) {
	desde := ahora.Add(-7 * 24 * time.Hour).Unix()
	gs, err := s.Alm.ResumenSemana(s.BotID, desde, 5)
	if err != nil {
		return "", false
	}
	filtros := s.Alm.ContarAcciones(s.BotID, desde, "filtro.")
	baneos := s.Alm.ContarAcciones(s.BotID, desde, "grupo.banear")
	raids := s.Alm.ContarAcciones(s.BotID, desde, "grupo.raid")
	if len(gs) == 0 && filtros == 0 && baneos == 0 && raids == 0 {
		return "", false
	}
	total, entran, salen, expuls := 0, 0, 0, 0
	for _, g := range gs {
		total += g.Mensajes
		entran += g.Entraron
		salen += g.Salieron
		expuls += g.Expulsados
	}
	var b strings.Builder
	b.WriteString("Resumen de la semana\n")
	if len(gs) > 0 {
		fmt.Fprintf(&b, "• Mensajes en tus grupos más activos: %d\n", total)
		b.WriteString("• Más activos: ")
		for i, g := range gs {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s (%d)", g.Nombre, g.Mensajes)
		}
		b.WriteString("\n")
		fmt.Fprintf(&b, "• Entraron %d, salieron %d, expulsaron a %d\n", entran, salen, expuls)
	}
	fmt.Fprintf(&b, "• Filtros que actuaron: %d · Baneos: %d · Alarmas de raid: %d", filtros, baneos, raids)
	return b.String(), true
}

// EnviarResumenSemana crea el aviso con el resumen de la semana (y, si está pedido, también llega por WhatsApp). Devuelve si había algo.
func (s *Servicio) EnviarResumenSemana(ahora time.Time) bool {
	t, ok := s.TextoResumenSemana(ahora)
	if !ok {
		return false
	}
	_, _ = s.Alm.CrearAviso(s.BotID, "resumen", "", "Resumen de la semana", t)
	return true
}

// ResumenSemanalToca dice si hoy toca mandar el resumen (domingo, ya pasadas las 8 de la noche, una sola vez por día) y lo marca.
func (s *Servicio) ResumenSemanalToca(ahora time.Time) bool {
	if s.Alm.AjusteBot(s.BotID, "resumen_semanal") != "1" {
		return false
	}
	l := ahora.In(s.Alm.Zona())
	hoy := l.Format("2006-01-02")
	if l.Weekday() != time.Sunday || l.Hour() < 20 || s.Alm.AjusteBot(s.BotID, "resumen_sem_ultimo") == hoy {
		return false
	}
	_ = s.Alm.FijarAjusteBot(s.BotID, "resumen_sem_ultimo", hoy)
	return true
}
