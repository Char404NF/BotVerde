package personas

import (
	"botchat/internal/respuestas"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/identidad"
)

// ChatPrivadoDe da la clave del chat privado de una persona (el que ya existe entre sus formas; si no, la canónica).
func (s *Servicio) ChatPrivadoDe(persona string) string {
	for _, f := range s.Alm.AliasDe(s.BotID, persona) {
		if c, _ := s.Alm.ObtenerChat(s.BotID, f); c != nil && !c.EsGrupo {
			return f
		}
	}
	return s.persona(persona)
}

// ParseDuracion entiende "30", "30m", "2h", "1d", "siempre". Devuelve la duración (0 = siempre) y si el texto era una duración.
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

// TextoDuracion dice una duración en palabras cortas.
func TextoDuracion(d time.Duration) string {
	switch {
	case d <= 0:
		return "para siempre"
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	case d < 48*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hora"
		}
		return fmt.Sprintf("%d horas", h)
	}
	return fmt.Sprintf("%d días", int(d.Hours()/24))
}

// objetivo busca a quién se refiere el comando; error si no se pudo.
func objetivo(cx *comandos.Contexto) (string, []string, error) {
	p, resto, ok := cx.Objetivo()
	if !ok {
		return "", nil, errors.New("dime a quién: menciónalo, responde a su mensaje o escribe su número")
	}
	return p, resto, nil
}

// chatDestino es el chat sobre el que actúa un comando de chat: en un grupo, el grupo; en privado, la persona indicada (si la hay)
// o el propio chat. Desde Mensajes guardados hay que indicar a quién.
func (s *Servicio) chatDestino(cx *comandos.Contexto) (chat string, resto []string, err error) {
	if cx.EsGrupo {
		return cx.Chat, cx.Args, nil
	}
	if p, r, ok := cx.Objetivo(); ok {
		if !cx.Dueno {
			if e := s.puede(cx.Persona, p); e != nil {
				return "", nil, e
			}
		}
		return s.ChatPrivadoDe(p), r, nil
	}
	if cx.Dueno {
		return "", nil, errors.New("dime de qué chat: escribe su número o menciónalo")
	}
	return cx.Chat, cx.Args, nil
}

// RegistrarComandos agrega a un despachador los comandos de personas y de chat del bloque B.
func (s *Servicio) RegistrarComandos(d *comandos.Despachador) {
	reg := func(c comandos.Comando) { _ = d.Registrar(c) }
	ctx := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), 20*time.Second)
	}

	// Desde la 0.6.0 «silenciar» y «desilenciar» son de los grupos (paquete silencio: borrar lo que escribe alguien). Lo de antes (que el
	// bot deje de contestarle) se llama «ignorar»: no hay alias con los nombres viejos porque chocarían con los nuevos.
	reg(comandos.Comando{Nombre: "ignorar", Resumen: "Que el bot deje de contestarle a alguien", Uso: "ignorar <persona> [30m|2h|1d|siempre]", Casilla: "personas.silenciar",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			p, resto, err := objetivo(cx)
			if err != nil {
				return "", err
			}
			dur := time.Hour
			if len(resto) > 0 {
				x, ok := ParseDuracion(resto[len(resto)-1])
				if !ok {
					return "", errors.New("la duración debe ser como 30m, 2h, 1d o siempre")
				}
				dur = x
			}
			if err := s.Silenciar(cx.Persona, p, dur, ""); err != nil {
				return "", err
			}
			return s.r("pers.ignorar", "cuando", map[bool]string{true: "para siempre", false: "durante " + TextoDuracion(dur)}[dur == 0]), nil
		}})
	reg(comandos.Comando{Nombre: "designorar", Resumen: "Que el bot vuelva a contestarle", Uso: "designorar <persona>", Casilla: "personas.silenciar",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			p, _, err := objetivo(cx)
			if err != nil {
				return "", err
			}
			ok, err := s.Desilenciar(cx.Persona, p)
			if err != nil {
				return "", err
			}
			if !ok {
				return "Esa persona no estaba ignorada. Si querías quitarle el silencio de grupos, usa " + cx.D.Prefijo() + "desilenciar.", nil
			}
			return s.r("pers.designorar"), nil
		}})
	reg(comandos.Comando{Nombre: "bloquear", Resumen: "Que el bot ignore del todo a alguien", Uso: "bloquear <persona>", Casilla: "personas.bloquear",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			p, _, err := objetivo(cx)
			if err != nil {
				return "", err
			}
			if err := s.Bloquear(cx.Persona, p); err != nil {
				return "", err
			}
			return s.r("pers.bloquear"), nil
		}})
	reg(comandos.Comando{Nombre: "desbloquear", Resumen: "Quitar el bloqueo del bot", Uso: "desbloquear <persona>", Casilla: "personas.bloquear",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			p, _, err := objetivo(cx)
			if err != nil {
				return "", err
			}
			ok, err := s.Desbloquear(cx.Persona, p)
			if err != nil {
				return "", err
			}
			if !ok {
				return "Esa persona no estaba bloqueada.", nil
			}
			return s.r("pers.desbloquear"), nil
		}})
	reg(comandos.Comando{Nombre: "advertir", Alias: []string{"warn"}, Resumen: "Poner una advertencia", Uso: "advertir <persona> [tipo] [motivo]", Casilla: "personas.advertir",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			p, resto, err := objetivo(cx)
			if err != nil {
				return "", err
			}
			c, cancel := ctx()
			defer cancel()
			chat := cx.Chat
			if cx.Dueno {
				chat = s.ChatPrivadoDe(p)
			}
			var tipoID int64
			if len(resto) > 0 {
				if tp := s.tipoPorNombre(resto[0]); tp != nil {
					tipoID, resto = tp.ID, resto[1:]
				}
			}
			var r *ResultadoAdvertir
			var err2 error
			if tipoID > 0 {
				r, err2 = s.AdvertirConTipo(c, cx.Persona, p, chat, cx.CitaID, strings.Join(resto, " "), tipoID, 0)
			} else {
				r, err2 = s.Advertir(c, cx.Persona, p, chat, cx.CitaID, strings.Join(resto, " "))
			}
			if err2 != nil {
				return "", err2
			}
			t := fmt.Sprintf("Advertencia puesta (lleva %d", r.Total)
			if r.Tope > 0 {
				t += fmt.Sprintf(" de %d", r.Tope)
			}
			t += ")."
			if r.Nivel > 0 && r.Hecho != "" {
				switch r.Hecho {
				case "avisado":
					t += " Le avisé."
				case "silenciado":
					t += " La silencié en grupos."
				case "pendiente":
					t += " Te pedí confirmar su expulsión (no se expulsa sola)."
				case "solo_aviso":
					t += " No hice el paso de la escalera (" + r.Motivo + ")."
				}
			}
			if r.LlegoAlTope {
				switch r.Hecho {
				case "silenciado":
					t += " Llegó al tope: el bot ya no le contesta."
				case "borrado":
					t += " Llegó al tope: borré su mensaje."
				case "expulsado":
					t += " Llegó al tope: la expulsé."
				default:
					t += " Llegó al tope: ya avisé al dueño."
				}
			}
			return t, nil
		}})
	reg(comandos.Comando{Nombre: "advertencias", Resumen: "Ver las advertencias de alguien", Uso: "advertencias <persona>", Casilla: "personas.advertir",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			p, _, err := objetivo(cx)
			if err != nil {
				return "", err
			}
			if err := s.puede(cx.Persona, p); err != nil {
				return "", err
			}
			todas, err := s.Alm.ListarAdvertenciasEn(s.BotID, p, 200, s.ahora())
			if err != nil {
				return "", err
			}
			var xs []almacen.Advertencia
			for _, x := range todas {
				if x.Vigente && len(xs) < 5 {
					xs = append(xs, x)
				}
			}
			total := s.Alm.ContarAdvertenciasEn(s.BotID, p, s.ahora())
			if total == 0 {
				return s.Nombre(p) + " no tiene advertencias.", nil
			}
			var b strings.Builder
			fmt.Fprintf(&b, "%s tiene %d advertencia(s)", s.Nombre(p), total)
			if tope := s.Alm.AjusteBotInt(s.BotID, "adv_tope"); tope > 0 {
				fmt.Fprintf(&b, " (tope %d)", tope)
			}
			b.WriteString(". Últimas:")
			for _, x := range xs {
				b.WriteString("\n• " + time.Unix(x.TS, 0).In(s.Alm.Zona()).Format("02/01 15:04"))
				if x.Motivo != "" {
					b.WriteString(" — " + x.Motivo)
				}
			}
			return b.String(), nil
		}})
	reg(comandos.Comando{Nombre: "quitaradvertencia", Alias: []string{"quitaradv", "unwarn"}, Resumen: "Quitar la última advertencia", Uso: "quitaradvertencia <persona>", Casilla: "personas.advertir",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			p, _, err := objetivo(cx)
			if err != nil {
				return "", err
			}
			quedan, quitada, err := s.QuitarAdvertencia(cx.Persona, p, 0)
			if err != nil {
				return "", err
			}
			if !quitada {
				return "Esa persona no tiene advertencias.", nil
			}
			return s.r("pers.adv_quitada", "n", fmt.Sprint(quedan)), nil
		}})
	reg(comandos.Comando{Nombre: "borraradvertencias", Resumen: "Borrar todo su historial de advertencias", Uso: "borraradvertencias <persona>", Casilla: "personas.advertir", Confirma: true,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			p, _, err := objetivo(cx)
			if err != nil {
				return "", err
			}
			n, err := s.BorrarHistorialAdvertencias(cx.Persona, p)
			if err != nil {
				return "", err
			}
			return s.r("pers.adv_borradas", "n", fmt.Sprint(n)), nil
		}})
	reg(comandos.Comando{Nombre: "nota", Resumen: "Escribir o ver notas sobre alguien", Uso: "nota <persona> [texto]", Casilla: "personas.notas",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			p, resto, err := objetivo(cx)
			if err != nil {
				return "", err
			}
			if t := strings.TrimSpace(strings.Join(resto, " ")); t != "" {
				if err := s.AgregarNota(cx.Persona, p, t); err != nil {
					return "", err
				}
				return "Nota guardada.", nil
			}
			ns, err := s.Notas(cx.Persona, p)
			if err != nil {
				return "", err
			}
			if len(ns) == 0 {
				return s.Nombre(p) + " no tiene notas.", nil
			}
			var b strings.Builder
			b.WriteString("Notas de " + s.Nombre(p) + ":")
			for i, n := range ns {
				if i >= 5 {
					break
				}
				b.WriteString("\n• " + time.Unix(n.TS, 0).In(s.Alm.Zona()).Format("02/01") + " — " + n.Texto)
			}
			return b.String(), nil
		}})
	reg(comandos.Comando{Nombre: "ficha", Resumen: "Ver la ficha de alguien", Uso: "ficha <persona>", Casilla: "personas.ficha",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			p, _, err := objetivo(cx)
			if err != nil {
				return "", err
			}
			f, err := s.Ficha(cx.Persona, p)
			if err != nil {
				return "", err
			}
			return TextoFicha(f, s.Alm.Zona()), nil
		}})
	reg(comandos.Comando{Nombre: "olvidar", Resumen: "Borrar todo lo que se guardó de alguien", Uso: "olvidar <persona>", Casilla: "personas.olvidar", Confirma: true,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			p, _, err := objetivo(cx)
			if err != nil {
				return "", err
			}
			r, err := s.Olvidar(cx.Persona, p)
			if err != nil {
				return "", err
			}
			return s.r("pers.olvidar", "n", fmt.Sprint(r.Mensajes)), nil
		}})

	// ----- chat -----
	reg(comandos.Comando{Nombre: "pausar", Resumen: "Pausar al bot en este chat", Uso: "pausar [30m|2h|1d]", Casilla: "chat.pausar",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, resto, err := s.chatDestino(cx)
			if err != nil {
				return "", err
			}
			dur := time.Hour
			if len(resto) > 0 {
				x, ok := ParseDuracion(resto[len(resto)-1])
				if !ok || x == 0 {
					return "", errors.New("la duración debe ser como 30m, 2h o 1d")
				}
				dur = x
			}
			if err := s.PausarChat(cx.Persona, chat, dur, false); err != nil {
				return "", err
			}
			return s.r("chat.pausar", "duracion", TextoDuracion(dur), "prefijo", cx.D.Prefijo()), nil
		}})
	reg(comandos.Comando{Nombre: "control", Alias: []string{"tomarcontrol"}, Resumen: "Tomar el control: el bot calla y atiendes tú", Casilla: "chat.tomar_control",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, _, err := s.chatDestino(cx)
			if err != nil {
				return "", err
			}
			if err := s.PausarChat(cx.Persona, chat, DuracionControl, true); err != nil {
				return "", err
			}
			return s.r("chat.control", "prefijo", cx.D.Prefijo()), nil
		}})
	reg(comandos.Comando{Nombre: "reanudar", Resumen: "Que el bot vuelva a contestar en este chat", Uso: "reanudar", Casilla: "chat.pausar", Siempre: true,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, _, err := s.chatDestino(cx)
			if err != nil {
				return "", err
			}
			if err := s.ReanudarChat(cx.Persona, chat); err != nil {
				return "", err
			}
			return s.r("chat.reanudar"), nil
		}})
	estado := func(nombre, est, resumen, ok string) {
		reg(comandos.Comando{Nombre: nombre, Resumen: resumen, Casilla: "chat.estado",
			Ejecutar: func(cx *comandos.Contexto) (string, error) {
				chat, _, err := s.chatDestino(cx)
				if err != nil {
					return "", err
				}
				if err := s.MarcarChat(cx.Persona, chat, est); err != nil {
					return "", err
				}
				return s.r(ok), nil
			}})
	}
	estado("pendiente", almacen.TrabajoPendiente, "Marcar este chat como pendiente", "chat.pendiente")
	estado("resuelto", almacen.TrabajoResuelto, "Marcar este chat como resuelto", "chat.resuelto")
	reg(comandos.Comando{Nombre: "etiquetar", Resumen: "Poner o quitar una etiqueta al chat", Uso: "etiquetar <etiqueta> | etiquetar quitar <etiqueta>", Casilla: "chat.etiquetas",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, resto, err := s.chatDestino(cx)
			if err != nil {
				return "", err
			}
			si := true
			if len(resto) > 0 && identidad.Normalizar(resto[0]) == "quitar" {
				si, resto = false, resto[1:]
			}
			nombre := strings.TrimSpace(strings.Join(resto, " "))
			if nombre == "" {
				ets, _ := s.Alm.ListarEtiquetas(s.BotID)
				if len(ets) == 0 {
					return "", errors.New("escribe la etiqueta, por ejemplo " + cx.D.Prefijo() + "etiquetar cliente")
				}
				var n []string
				for _, e := range ets {
					n = append(n, e.Nombre)
				}
				return "Etiquetas: " + strings.Join(n, ", ") + ".", nil
			}
			e, err := s.Etiquetar(cx.Persona, chat, nombre, si, cx.Dueno)
			if err != nil {
				return "", err
			}
			if !si {
				return s.r("chat.etiqueta_quitada", "etiqueta", e.Nombre), nil
			}
			return s.r("chat.etiqueta_puesta", "etiqueta", e.Nombre), nil
		}})
	reg(comandos.Comando{Nombre: "reportar", Alias: []string{"report"}, Resumen: "Avisar al dueño de un mensaje (responde a él)", Uso: "reportar [motivo]", Casilla: "chat.reportar",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			if cx.CitaID == "" && cx.CitaAutor == "" {
				return "", errors.New("responde al mensaje que quieres reportar y escribe " + cx.D.Prefijo() + "reportar")
			}
			if err := s.Reportar(cx.Persona, cx.CitaAutor, cx.Chat, cx.Resto); err != nil {
				return "", err
			}
			if s.Alm.AjusteBotInt(s.BotID, "reportar_gracias") == 1 {
				// el texto propio de «Respuestas del bot» manda; si no hay, el que ya se tenía guardado (reportar_texto), y si no, el de fábrica
				if s.Alm.Respuesta(s.BotID, "reportar.ok") == "" {
					if t := strings.TrimSpace(s.Alm.AjusteBot(s.BotID, "reportar_texto")); t != "" && t != textoReporteAnterior {
						return t, nil
					}
				}
				return s.r("reportar.ok"), nil
			}
			return "", nil
		}})
}

// TextoFicha es la ficha en pocas líneas, para un comando.
func TextoFicha(f *FichaCompleta, loc *time.Location) string {
	var b strings.Builder
	nombre := f.Nombre
	if nombre == "" {
		nombre = "Sin nombre"
	}
	b.WriteString(nombre)
	if f.Telefono != "" {
		b.WriteString(" · " + f.Telefono)
	}
	rango := "sin rango"
	if f.Rango != "" {
		rango = f.Rango
	}
	b.WriteString("\nRango: " + rango)
	estado := "normal"
	switch {
	case f.Bloqueada:
		estado = "bloqueada en el bot"
	case f.Silencio != nil && f.Silencio.HastaTS == 0:
		estado = "ignorada por el bot para siempre"
	case f.Silencio != nil:
		estado = "ignorada por el bot hasta " + time.Unix(f.Silencio.HastaTS, 0).In(loc).Format("02/01 15:04")
	}
	if f.NoMolestar {
		estado += " · en No molestar"
	}
	b.WriteString("\nEstado: " + estado)
	fmt.Fprintf(&b, "\nAdvertencias: %d · Notas: %d", f.Advertencias, f.Notas)
	if len(f.Etiquetas) > 0 {
		var n []string
		for _, e := range f.Etiquetas {
			n = append(n, e.Nombre)
		}
		b.WriteString("\nEtiquetas: " + strings.Join(n, ", "))
	}
	fmt.Fprintf(&b, "\nMensajes que escribió: %d · que le escribí: %d", f.Recibidos, f.Enviados)
	if f.PrimeraTS > 0 {
		b.WriteString("\nPrimera vez: " + time.Unix(f.PrimeraTS, 0).In(loc).Format("02/01/2006"))
		b.WriteString(" · Última: " + time.Unix(f.UltimaTS, 0).In(loc).Format("02/01/2006"))
	}
	return b.String()
}

func (s *Servicio) r(id string, vars ...string) string {
	return respuestas.Texto(s.Alm, s.BotID, id, vars...)
}

// textoReporteAnterior era el texto de fábrica de antes: si es el que quedó guardado, se usa el nuevo de fábrica.
const textoReporteAnterior = "Gracias, ya avisé."
