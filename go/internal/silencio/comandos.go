package silencio

import (
	"botchat/internal/respuestas"
	"errors"
	"fmt"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/identidad"
)

// ParseDuracion entiende «30», «30m», «2h», «1d», «siempre». Devuelve la duración (0 = siempre) y si el texto era una duración.
func ParseDuracion(t string) (time.Duration, bool) {
	t = strings.ToLower(strings.TrimSpace(t))
	switch t {
	case "siempre", "perm", "permanente", "para-siempre":
		return 0, true
	case "":
		return 0, false
	}
	mult := time.Minute
	switch {
	case strings.HasSuffix(t, "min"):
		t = strings.TrimSuffix(t, "min")
	case strings.HasSuffix(t, "m"):
		t = strings.TrimSuffix(t, "m")
	case strings.HasSuffix(t, "h"):
		t, mult = strings.TrimSuffix(t, "h"), time.Hour
	case strings.HasSuffix(t, "d"):
		t, mult = strings.TrimSuffix(t, "d"), 24*time.Hour
	case strings.HasSuffix(t, "s"):
		return 0, false
	}
	var n int
	if _, err := fmt.Sscanf(t, "%d", &n); err != nil || n <= 0 || fmt.Sprint(n) != t || n > 100000 {
		return 0, false
	}
	return time.Duration(n) * mult, true
}

// RegistrarComandos agrega los comandos de silencio de grupos: !silenciar, !desilenciar, !silenciados y !contador.
func (s *Servicio) RegistrarComandos(d *comandos.Despachador) {
	reg := func(c comandos.Comando) { _ = d.Registrar(c) }
	objetivo := func(cx *comandos.Contexto) (string, []string, error) {
		p, resto, ok := cx.Objetivo()
		if !ok {
			return "", nil, errors.New("dime a quién: menciónalo, responde a su mensaje o escribe su número")
		}
		return p, resto, nil
	}

	reg(comandos.Comando{Nombre: "silenciar", Resumen: "Borrar lo que escribe alguien en el grupo (si soy admin)", Uso: "silenciar <persona> [30m|2h|1d|siempre] [todos]",
		Casilla: "silencio.silenciar",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			p, resto, err := objetivo(cx)
			if err != nil {
				return "", err
			}
			dur, todos := time.Hour, false
			for _, a := range resto {
				if x, ok := ParseDuracion(a); ok {
					dur = x
					continue
				}
				switch identidad.Normalizar(a) {
				case "todos", "todo", "todas":
					todos = true
				case "aqui", "aca":
				default:
					return "", errors.New("la duración debe ser como 30m, 2h, 1d o siempre; y «todos» para todos los grupos")
				}
			}
			o := Opciones{Duracion: dur, Avisar: true, Origen: "manual"}
			donde := "en todos los grupos que administro"
			switch {
			case todos || !cx.EsGrupo:
				o.Alcance = almacen.SilTodos
			default:
				o.Alcance, o.Grupos = almacen.SilUno, []string{cx.Chat}
				donde = "aquí"
			}
			if _, err := s.Silenciar(cx.Persona, p, o); err != nil {
				return "", err
			}
			cuando := "para siempre"
			if dur > 0 {
				cuando = TextoDuracion(s.ahora().Add(dur).Unix(), s.ahora(), s.Alm.Zona())
			}
			return s.r("sil.silenciada", "nombre", s.nombre(p), "donde", donde, "cuando", cuando), nil
		}})
	reg(comandos.Comando{Nombre: "desilenciar", Resumen: "Quitar el silencio de grupos a alguien", Uso: "desilenciar <persona>", Casilla: "silencio.silenciar",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			p, _, err := objetivo(cx)
			if err != nil {
				return "", err
			}
			ok, err := s.QuitarSilencio(cx.Persona, p)
			if err != nil {
				return "", err
			}
			if !ok {
				return "Esa persona no estaba silenciada en grupos. Si querías que el bot vuelva a contestarle, usa " + cx.D.Prefijo() + "designorar.", nil
			}
			return "Listo: " + s.nombre(p) + " ya puede escribir en los grupos.", nil
		}})
	reg(comandos.Comando{Nombre: "silenciados", Alias: []string{"silencios"}, Resumen: "Ver quién está silenciado en grupos", Uso: "silenciados", Casilla: "silencio.ver",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			xs, err := s.Listar()
			if err != nil {
				return "", err
			}
			if len(xs) == 0 {
				return "Nadie está silenciado en grupos.", nil
			}
			var b strings.Builder
			b.WriteString("Silenciados en grupos:")
			loc := s.Alm.Zona()
			for i, x := range xs {
				if i >= 15 {
					fmt.Fprintf(&b, "\n… y %d más", len(xs)-i)
					break
				}
				hasta := "para siempre"
				if x.HastaTS > 0 {
					hasta = "hasta " + time.Unix(x.HastaTS, 0).In(loc).Format("02/01 15:04")
				}
				donde := "todos los grupos"
				if x.Alcance != almacen.SilTodos {
					donde = fmt.Sprintf("%d grupo(s)", len(x.Grupos))
				}
				fmt.Fprintf(&b, "\n• %s — %s, %s (hoy %d)", x.Nombre, donde, hasta, x.Contador.Total)
			}
			return b.String(), nil
		}})
	reg(comandos.Comando{Nombre: "contador", Resumen: "Ver o poner en cero el contador de un silenciado", Uso: "contador <persona> [cero]", Casilla: "silencio.silenciar",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			p, resto, err := objetivo(cx)
			if err != nil {
				return "", err
			}
			if len(resto) > 0 && identidad.Normalizar(resto[0]) == "cero" {
				if err := s.PonerEnCero(cx.Persona, p, ""); err != nil {
					return "", err
				}
				return s.r("sil.contador_cero", "nombre", s.nombre(p)), nil
			}
			c := s.ContadorDe(p, "", s.ahora())
			if c.Tope > 0 {
				return fmt.Sprintf("%s lleva %d de %d mensajes borrados hoy.", s.nombre(p), c.Usado, c.Tope), nil
			}
			return fmt.Sprintf("%s lleva %d mensajes borrados hoy.", s.nombre(p), c.Usado), nil
		}})
}

func (s *Servicio) r(id string, vars ...string) string {
	return respuestas.Texto(s.Alm, s.BotID, id, vars...)
}
