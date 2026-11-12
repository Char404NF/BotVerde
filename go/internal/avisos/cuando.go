package avisos

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/identidad"
)

var diasSemana = map[string]time.Weekday{
	"domingo": time.Sunday, "lunes": time.Monday, "martes": time.Tuesday, "miercoles": time.Wednesday,
	"jueves": time.Thursday, "viernes": time.Friday, "sabado": time.Saturday,
}

// hora lee una hora explícita: "9:30", "18:05", "9am", "9:30pm", "21h". Un número suelto no cuenta como hora.
func hora(t string) (h, m int, ok bool) {
	t = strings.ToLower(strings.TrimSpace(t))
	pm, am, explicita := false, false, false
	switch {
	case strings.HasSuffix(t, "pm"):
		pm, explicita, t = true, true, strings.TrimSuffix(t, "pm")
	case strings.HasSuffix(t, "am"):
		am, explicita, t = true, true, strings.TrimSuffix(t, "am")
	case strings.HasSuffix(t, "hrs"):
		explicita, t = true, strings.TrimSuffix(t, "hrs")
	case strings.HasSuffix(t, "h"):
		explicita, t = true, strings.TrimSuffix(t, "h")
	}
	partes := strings.Split(t, ":")
	if len(partes) > 2 {
		return 0, 0, false
	}
	if len(partes) == 2 {
		explicita = true
	}
	if !explicita {
		return 0, 0, false
	}
	h, err := strconv.Atoi(partes[0])
	if err != nil {
		return 0, 0, false
	}
	if len(partes) == 2 {
		if m, err = strconv.Atoi(partes[1]); err != nil {
			return 0, 0, false
		}
	}
	if pm && h < 12 {
		h += 12
	}
	if am && h == 12 {
		h = 0
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

// ParsearCuando entiende cómo se pide un recordatorio por texto y separa el mensaje:
//
//	[diario|semanal] en 30m|2h|3d <texto>
//	[diario|semanal] hoy|mañana [HH:MM] <texto>         (sin hora: 09:00)
//	[diario|semanal] HH:MM <texto>                       (la próxima vez que sea esa hora)
//	[diario|semanal] lunes..domingo [HH:MM] <texto>
//	dd/mm[/aaaa] [HH:MM] <texto>
func ParsearCuando(args []string, ahora time.Time, loc *time.Location) (ts int64, repetir, texto string, err error) {
	faltaCuando := errors.New("no entendí cuándo. Ejemplos: «mañana 9:00 llamar a Ana», «en 2h sacar el pan», «diario 8:00 tomar agua»")
	if len(args) == 0 {
		return 0, "", "", faltaCuando
	}
	a := make([]string, len(args))
	for i, x := range args {
		a[i] = identidad.Normalizar(x)
	}
	i := 0
	switch a[0] {
	case "diario", "diaria", "cadadia", "todoslosdias":
		repetir, i = almacen.RepiteDiaria, 1
	case "semanal", "cadasemana":
		repetir, i = almacen.RepiteSemanal, 1
	}
	if i >= len(a) {
		return 0, "", "", faltaCuando
	}
	t := ahora.In(loc)
	hoy := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	var cuando time.Time
	horaOpc := func() (int, int) {
		if i < len(a) {
			if h, m, ok := hora(a[i]); ok {
				i++
				return h, m
			}
		}
		return 9, 0
	}
	switch {
	case a[i] == "en":
		if i+1 >= len(a) {
			return 0, "", "", faltaCuando
		}
		n, u := a[i+1], ""
		if j := strings.IndexFunc(n, func(r rune) bool { return r < '0' || r > '9' }); j > 0 {
			n, u = a[i+1][:j], a[i+1][j:]
			i += 2
		} else if i+2 < len(a) {
			u = a[i+2]
			i += 3
		} else {
			return 0, "", "", faltaCuando
		}
		v, e := strconv.Atoi(n)
		if e != nil || v <= 0 || v > 100000 {
			return 0, "", "", faltaCuando
		}
		var d time.Duration
		switch {
		case strings.HasPrefix(u, "min") || u == "m":
			d = time.Duration(v) * time.Minute
		case strings.HasPrefix(u, "hora") || u == "h" || u == "hr" || u == "hrs":
			d = time.Duration(v) * time.Hour
		case strings.HasPrefix(u, "dia") || u == "d":
			d = time.Duration(v) * 24 * time.Hour
		case strings.HasPrefix(u, "sem"):
			d = time.Duration(v) * 7 * 24 * time.Hour
		default:
			return 0, "", "", faltaCuando
		}
		cuando = t.Add(d).Truncate(time.Minute)
	case a[i] == "hoy" || a[i] == "manana":
		dias := map[string]int{"hoy": 0, "manana": 1}[a[i]]
		i++
		h, m := horaOpc()
		cuando = hoy.AddDate(0, 0, dias).Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
	case a[i] == "pasado" && i+1 < len(a) && a[i+1] == "manana":
		i += 2
		h, m := horaOpc()
		cuando = hoy.AddDate(0, 0, 2).Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
	default:
		if wd, ok := diasSemana[a[i]]; ok {
			i++
			h, m := horaOpc()
			d := (int(wd) - int(t.Weekday()) + 7) % 7
			cuando = hoy.AddDate(0, 0, d).Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
			if !cuando.After(t) {
				cuando = cuando.AddDate(0, 0, 7)
			}
		} else if partes := strings.Split(a[i], "/"); len(partes) >= 2 && len(partes) <= 3 {
			dd, e1 := strconv.Atoi(partes[0])
			mm, e2 := strconv.Atoi(partes[1])
			aa := t.Year()
			if len(partes) == 3 {
				var e3 error
				if aa, e3 = strconv.Atoi(partes[2]); e3 != nil {
					return 0, "", "", faltaCuando
				}
				if aa < 100 {
					aa += 2000
				}
			}
			if e1 != nil || e2 != nil || dd < 1 || dd > 31 || mm < 1 || mm > 12 {
				return 0, "", "", faltaCuando
			}
			i++
			h, m := horaOpc()
			cuando = time.Date(aa, time.Month(mm), dd, h, m, 0, 0, loc)
			if len(partes) == 2 && !cuando.After(t) {
				cuando = cuando.AddDate(1, 0, 0)
			}
		} else if h, m, ok := hora(a[i]); ok {
			i++
			cuando = hoy.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
			if !cuando.After(t) {
				cuando = cuando.AddDate(0, 0, 1)
			}
		} else {
			return 0, "", "", faltaCuando
		}
	}
	// Fuera de un cambio de hora, sumar días a medianoche + horas puede desfasar: se vuelve a armar con el reloj de pared.
	cuando = cuando.In(loc)
	texto = strings.TrimSpace(strings.Join(args[i:], " "))
	if texto == "" {
		return 0, "", "", errors.New("falta de qué te aviso")
	}
	if !cuando.After(t) {
		return 0, "", "", errors.New("esa fecha y hora ya pasaron")
	}
	if cuando.Sub(t) > 3*366*24*time.Hour {
		return 0, "", "", errors.New("esa fecha está muy lejos (máximo 3 años)")
	}
	return cuando.Unix(), repetir, texto, nil
}

// RegistrarComandos agrega !recordar (solo el Dueño, desde Mensajes guardados o la app).
func (s *Servicio) RegistrarComandos(d *comandos.Despachador) {
	_ = d.Registrar(comandos.Comando{Nombre: "recordar", Alias: []string{"recordatorio"}, Resumen: "Crear un recordatorio para ti", SoloDueno: true,
		Uso: "recordar mañana 9:00 llamar a Ana",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			if s.Alm.AjusteBot(s.BotID, "util_recordatorios") == "0" { // v0.6.0 E9: lo apaga el asistente «Crea tu bot»
				return "", errors.New("los recordatorios están apagados")
			}
			if len(cx.Args) == 1 && identidad.Normalizar(cx.Args[0]) == "lista" {
				rs, err := s.Alm.ListarRecordatorios(s.BotID, false)
				if err != nil {
					return "", err
				}
				if len(rs) == 0 {
					return "No tienes recordatorios pendientes.", nil
				}
				var b strings.Builder
				b.WriteString("Tus recordatorios:")
				for i, r := range rs {
					if i >= 8 {
						break
					}
					fmt.Fprintf(&b, "\n• %s — %s", time.Unix(r.ProximaTS, 0).In(s.Alm.Zona()).Format("02/01 15:04"), r.Texto)
				}
				return b.String(), nil
			}
			ts, rep, texto, err := ParsearCuando(cx.Args, s.ahora(), s.Alm.Zona())
			if err != nil {
				return "", err
			}
			if _, err := s.Alm.CrearRecordatorio(s.BotID, texto, ts, rep, s.ahora()); err != nil {
				return "", err
			}
			s.Alm.RegistrarAccion(s.BotID, almacen.PersonaDueno, "recordatorio.crear", "", "", "ok", "")
			t := "Listo: te aviso el " + time.Unix(ts, 0).In(s.Alm.Zona()).Format("02/01 15:04")
			switch rep {
			case almacen.RepiteDiaria:
				t += " y todos los días"
			case almacen.RepiteSemanal:
				t += " y cada semana"
			}
			return t + ".", nil
		}})
}
