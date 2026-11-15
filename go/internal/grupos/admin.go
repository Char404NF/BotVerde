package grupos

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
)

// Administrar grupos: baneos, cierre por horario, perfil de un miembro y limpieza de inactivos.

// Banear saca a la persona del grupo (si está) y la anota para que no vuelva: si entra de nuevo, AlEntrar la saca.
func (s *Servicio) Banear(ctx context.Context, actor, chat, persona, motivo string) error {
	if s.Alm.EstaBaneado(s.BotID, chat, persona) {
		return ErrYaBaneado
	}
	err := s.ExpulsarComo(ctx, actor, chat, persona)
	if err != nil && err != ErrNoMiembro {
		return err
	}
	if e := s.Alm.Banear(s.BotID, chat, persona, s.persona(actor), motivo, s.ahora()); e != nil {
		return e
	}
	s.registra(actor, "grupo.banear", persona, chat, "ok", "")
	return nil
}

// Desbanear quita el baneo: la persona puede volver a entrar (no se le agrega sola).
func (s *Servicio) Desbanear(actor, chat, persona string) error {
	ok, err := s.Alm.Desbanear(s.BotID, chat, persona)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNoBaneado
	}
	s.registra(actor, "grupo.desbanear", persona, chat, "ok", "")
	return nil
}

// AlEntrar se llama cuando alguien entra a un grupo: si está baneada, se la saca. Devuelve si lo intentó.
func (s *Servicio) AlEntrar(ctx context.Context, chat, persona string) bool {
	if persona == "" || !s.Alm.EstaBaneado(s.BotID, chat, persona) {
		return false
	}
	if err := s.ExpulsarComo(ctx, almacen.PersonaDueno, chat, persona); err != nil {
		s.log("Grupo: no pude sacar a una persona baneada que volvió a entrar (" + err.Error() + ").")
		return true
	}
	s.registra(almacen.PersonaDueno, "grupo.baneado_volvio", persona, chat, "ok", "")
	return true
}

// AplicarHorarios abre o cierra los grupos que tienen horario. Se llama cada minuto; solo actúa cuando el estado deseado cambió.
func (s *Servicio) AplicarHorarios(ctx context.Context, ahora time.Time) int {
	n := 0
	ahora = ahora.In(s.Alm.Zona()) // las horas del horario son las del teléfono, no las de UTC
	for _, h := range s.Alm.HorariosActivos(s.BotID) {
		quiero := h.EstadoDeseado(ahora)
		if quiero == "" || quiero == h.Ultimo {
			continue
		}
		if quiero == "abierto" && h.Ultimo == "" {
			// recién encendido (o primera vez): el grupo nunca lo cerró el horario, así que no hay nada que abrir ni avisar
			s.Alm.MarcarHorario(s.BotID, h.Grupo, quiero)
			continue
		}
		if !s.conectado() {
			return n
		}
		if err := s.Cerrar(ctx, almacen.PersonaDueno, h.Grupo, quiero == "cerrado"); err != nil {
			s.log("Grupo: el horario no pudo dejar el grupo " + quiero + " (" + err.Error() + ").")
			continue
		}
		s.Alm.MarcarHorario(s.BotID, h.Grupo, quiero)
		n++
		if s.Enviar != nil {
			txt := "🔒 El grupo se cierra por hoy. Hasta mañana."
			if quiero == "abierto" {
				txt = "🔓 El grupo está abierto otra vez. ¡Buen día!"
			}
			_ = s.Enviar(ctx, h.Grupo, txt, "", nil)
		}
	}
	return n
}

// TextoPerfil resume a una persona en un grupo para contestar por comando.
func (s *Servicio) TextoPerfil(chat, persona, nombre string) string {
	c := s.Alm.ConteoPersona(s.BotID, chat, persona)
	var b strings.Builder
	fmt.Fprintf(&b, "👤 %s\n", nombre)
	if c.Mensajes == 0 {
		b.WriteString("Sin mensajes guardados en este grupo.")
		return b.String()
	}
	fmt.Fprintf(&b, "💬 %d mensajes · 🖼 %d imágenes · 🎬 %d videos\n🎵 %d audios · 🎤 %d notas de voz · 🧩 %d stickers · 📄 %d documentos\n",
		c.Mensajes, c.Imagenes, c.Videos, c.Audios, c.NotasVoz, c.Stickers, c.Documentos)
	if c.UltimoTS > 0 {
		fmt.Fprintf(&b, "Último mensaje: %s", time.Unix(c.UltimoTS, 0).Format("02/01/2006 15:04"))
	}
	if n := s.Alm.ContarAdvertenciasEn(s.BotID, persona, s.ahora()); n > 0 {
		fmt.Fprintf(&b, "\n⚠️ %d advertencias vigentes", n)
	}
	if s.Alm.EstaBaneado(s.BotID, chat, persona) {
		b.WriteString("\n⛔ Baneada de este grupo")
	}
	return b.String()
}

// Encuestar manda una encuesta al grupo.
func (s *Servicio) Encuestar(ctx context.Context, actor, chat, titulo string, opciones []string, multiple bool) error {
	if _, err := parseaGrupo(chat); err != nil {
		return err
	}
	if s.Encuesta == nil {
		return ErrNoDisponible
	}
	var limpias []string
	for _, o := range opciones {
		if o = strings.TrimSpace(o); o != "" {
			limpias = append(limpias, o)
		}
	}
	if strings.TrimSpace(titulo) == "" || len(limpias) < 2 || len(limpias) > 12 || utf8.RuneCountInString(titulo) > 200 {
		return fallo("una encuesta lleva una pregunta y de 2 a 12 opciones", nil)
	}
	err := s.Encuesta(ctx, chat, strings.TrimSpace(titulo), limpias, multiple)
	res := "ok"
	if err != nil {
		res = "error"
	}
	s.registra(actor, "grupo.encuesta", "", chat, res, fmt.Sprintf("opciones=%d", len(limpias)))
	return Explicar(err)
}

// FijarMensaje fija (o suelta) un mensaje del grupo. dias: 1, 7 o 30.
func (s *Servicio) FijarMensaje(ctx context.Context, actor, chat, msgID string, dias int, fijar bool) error {
	if _, err := parseaGrupo(chat); err != nil {
		return err
	}
	if s.Fijar == nil {
		return ErrNoDisponible
	}
	if msgID == "" {
		return ErrSinMensaje
	}
	err := s.Fijar(ctx, chat, msgID, dias*86400, fijar)
	res, acc := "ok", "grupo.fijar"
	if !fijar {
		acc = "grupo.desfijar"
	}
	if err != nil {
		res = "error"
	}
	s.registra(actor, acc, "", chat, res, "")
	return Explicar(err)
}

// partesEncuesta separa «pregunta | op1 | op2» (con «multiple» al principio para elegir varias).
func partesEncuesta(resto string) (titulo string, ops []string, multiple bool) {
	resto = strings.TrimSpace(resto)
	if p := strings.Fields(resto); len(p) > 0 && strings.EqualFold(p[0], "multiple") {
		multiple = true
		resto = strings.TrimSpace(resto[len(p[0]):])
	}
	partes := strings.Split(resto, "|")
	titulo = strings.TrimSpace(partes[0])
	for _, o := range partes[1:] {
		ops = append(ops, o)
	}
	return
}

func (s *Servicio) registrarAdmin(reg func(comandos.Comando), validaObjetivo func(*comandos.Contexto) error) {
	reg(comandos.Comando{Nombre: "ban", Alias: []string{"banear"}, Resumen: "Sacar a una persona y no dejarla volver", Uso: "ban <persona> [motivo]", Casilla: "grupos.banear", Ambito: comandos.Grupo, Confirma: true, Validar: validaObjetivo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, _, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			p, resto, err := personaObjetivo(cx)
			if err != nil {
				return "", err
			}
			ctx, cancel := ctxCmd()
			defer cancel()
			if err := s.Banear(ctx, cx.Persona, chat, p, strings.Join(resto, " ")); err != nil {
				return "", err
			}
			return s.r("grupo.baneada"), nil
		}})
	reg(comandos.Comando{Nombre: "encuesta", Resumen: "Mandar una encuesta al grupo", Uso: "encuesta [multiple] ¿Pregunta? | opción 1 | opción 2 | …", Casilla: "grupos.encuesta", Ambito: comandos.Grupo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, _, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			titulo, ops, multi := partesEncuesta(cx.Resto)
			ctx, cancel := ctxCmd()
			defer cancel()
			if err := s.Encuestar(ctx, cx.Persona, chat, titulo, ops, multi); err != nil {
				return "", err
			}
			return s.r("grupo.encuesta_enviada"), nil
		}})
	reg(comandos.Comando{Nombre: "fijar", Resumen: "Fijar el mensaje que citas (1, 7 o 30 días)", Uso: "fijar [1|7|30]  (respondiendo al mensaje)", Casilla: "grupos.encuesta", Ambito: comandos.Grupo,
		Validar: func(cx *comandos.Contexto) error {
			if _, _, err := s.grupoDe(cx); err != nil {
				return err
			}
			if cx.CitaID == "" {
				return errors.New("responde al mensaje que quieres fijar y escribe el comando")
			}
			return nil
		},
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, _, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			dias := 7
			if len(cx.Args) > 0 {
				switch cx.Args[0] {
				case "1", "7", "30":
					dias, _ = strconv.Atoi(cx.Args[0])
				default:
					return "", errors.New("los días pueden ser 1, 7 o 30")
				}
			}
			ctx, cancel := ctxCmd()
			defer cancel()
			if err := s.FijarMensaje(ctx, cx.Persona, chat, cx.CitaID, dias, true); err != nil {
				return "", err
			}
			return s.r("grupo.mensaje_fijado"), nil
		}})
	reg(comandos.Comando{Nombre: "desfijar", Resumen: "Soltar el mensaje fijado que citas", Uso: "desfijar  (respondiendo al mensaje)", Casilla: "grupos.encuesta", Ambito: comandos.Grupo,
		Validar: func(cx *comandos.Contexto) error {
			if _, _, err := s.grupoDe(cx); err != nil {
				return err
			}
			if cx.CitaID == "" {
				return errors.New("responde al mensaje fijado y escribe el comando")
			}
			return nil
		},
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, _, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			ctx, cancel := ctxCmd()
			defer cancel()
			if err := s.FijarMensaje(ctx, cx.Persona, chat, cx.CitaID, 0, false); err != nil {
				return "", err
			}
			return s.r("grupo.mensaje_desfijado"), nil
		}})
	reg(comandos.Comando{Nombre: "unban", Alias: []string{"desbanear"}, Resumen: "Dejar que una persona baneada vuelva", Uso: "unban <persona>", Casilla: "grupos.banear", Ambito: comandos.Grupo, Validar: validaObjetivo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, _, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			p, _, err := personaObjetivo(cx)
			if err != nil {
				return "", err
			}
			if err := s.Desbanear(cx.Persona, chat, p); err != nil {
				return "", err
			}
			return s.r("grupo.desbaneada"), nil
		}})
	reg(comandos.Comando{Nombre: "baneados", Resumen: "Ver a quién está baneado de este grupo", Uso: "baneados", Casilla: "grupos.banear", Ambito: comandos.Grupo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, _, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			l, err := s.Alm.ListarBaneados(s.BotID, chat)
			if err != nil {
				return "", err
			}
			if len(l) == 0 {
				return s.r("grupo.sin_baneados"), nil
			}
			var b strings.Builder
			fmt.Fprintf(&b, "⛔ Baneados (%d):", len(l))
			for i, x := range l {
				if i == 30 {
					fmt.Fprintf(&b, "\n… y %d más.", len(l)-30)
					break
				}
				n := x.Nombre
				if n == "" {
					n = x.Telefono
				}
				fmt.Fprintf(&b, "\n• %s", n)
				if x.Motivo != "" {
					fmt.Fprintf(&b, " — %s", x.Motivo)
				}
			}
			return b.String(), nil
		}})
	reg(comandos.Comando{Nombre: "horariogrupo", Resumen: "Cerrar y abrir el grupo solo a ciertas horas", Uso: "horariogrupo <cierra> <abre> | horariogrupo off", Casilla: "grupos.horario", Ambito: comandos.Grupo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, resto, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			h := s.Alm.HorarioDe(s.BotID, chat)
			switch {
			case len(resto) == 0:
				if !h.Activo {
					return "Este grupo no tiene horario. Ejemplo: !horariogrupo 22:00 07:00", nil
				}
				return fmt.Sprintf("🕙 Se cierra a las %s y se abre a las %s.", h.Cierra, h.Abre), nil
			case len(resto) == 1 && (strings.EqualFold(resto[0], "off") || strings.EqualFold(resto[0], "apagar")):
				h.Activo = false
				if err := s.Alm.GuardarHorario(s.BotID, h); err != nil {
					return "", err
				}
				return s.r("grupo.horario_off"), nil
			case len(resto) == 2:
				h.Activo, h.Cierra, h.Abre = true, resto[0], resto[1]
				if err := s.Alm.GuardarHorario(s.BotID, h); err != nil {
					return "", err
				}
				return s.r("grupo.horario_on", "cierra", h.Cierra, "abre", h.Abre), nil
			}
			return "Escribe: !horariogrupo 22:00 07:00  (o !horariogrupo off).", nil
		}})
	reg(comandos.Comando{Nombre: "limpiarinactivos", Resumen: "Sacar a quienes llevan tiempo sin escribir (no toca a los administradores)", Uso: "limpiarinactivos [30|60|90]", Casilla: "grupos.expulsar", Ambito: comandos.Grupo, Confirma: true,
		Validar: func(cx *comandos.Contexto) error { _, _, err := s.grupoDe(cx); return err },
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, resto, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			dias := 30
			if len(resto) > 0 {
				if n, e := strconv.Atoi(resto[0]); e == nil {
					dias = n
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute) // son varias sacadas con pausa

			defer cancel()
			n, fallos, err := s.LimpiarInactivos(ctx, cx.Persona, chat, dias)
			if err != nil {
				return "", err
			}
			if n == 0 && fallos == 0 {
				return s.r("grupo.sin_inactivos"), nil
			}
			t := s.r("grupo.inactivos_sacados", "n", fmt.Sprint(n), "dias", fmt.Sprint(s.NormalizaDias(dias)))
			if fallos > 0 {
				t += fmt.Sprintf(" A %d no pude sacarlas.", fallos)
			}
			return t, nil
		}})
	reg(comandos.Comando{Nombre: "perfil", Resumen: "Ver cuánto ha enviado una persona al grupo", Uso: "perfil <persona>", Casilla: "grupos.ranking", Ambito: comandos.Grupo, Validar: validaObjetivo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, _, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			p, _, err := personaObjetivo(cx)
			if err != nil {
				return "", err
			}
			return s.TextoPerfil(chat, p, s.Alm.NombreDePersona(s.BotID, p)), nil
		}})
}

// LimpiarInactivos saca (de a una, con una pausa) a los miembros que no escriben desde hace `dias` días. No toca a los administradores
// ni a quien no se puede sacar. Como mucho 50 por vez. Devuelve cuántas salieron y a cuántas no se pudo.
func (s *Servicio) LimpiarInactivos(ctx context.Context, actor, chat string, dias int) (sacadas, fallos int, err error) {
	l, _, err := s.Inactivos(ctx, chat, dias)
	if err != nil {
		return 0, 0, err
	}
	for _, x := range l {
		if x.EsAdmin {
			continue
		}
		if sacadas+fallos >= 50 {
			break
		}
		if e := s.ExpulsarComo(ctx, actor, chat, x.Persona); e != nil {
			fallos++
		} else {
			sacadas++
		}
		s.pausa(ctx, 800*time.Millisecond)
	}
	return sacadas, fallos, nil
}

// AlEntrarRevisa aplica las defensas de entrada: baneados, prefijos de país bloqueados y el modo raid. Devuelve qué hizo
// ("baneado", "prefijo", "raid" o "" si no hizo nada). Se llama desde una goroutine (usa la red).
func (s *Servicio) AlEntrarRevisa(ctx context.Context, chat, persona string) string {
	if persona == "" {
		return ""
	}
	if s.AlEntrar(ctx, chat, persona) {
		return "baneado"
	}
	e := s.Alm.EntradaDe(s.BotID, chat)
	if e.PrefijoBloqueado(persona) {
		if err := s.ExpulsarComo(ctx, almacen.PersonaDueno, chat, persona); err != nil {
			s.log("Grupo: no pude sacar a alguien de un país bloqueado (" + err.Error() + ").")
		} else {
			s.registra(almacen.PersonaDueno, "grupo.prefijo_bloqueado", persona, chat, "ok", "")
		}
		return "prefijo"
	}
	if e.RaidActivo && s.contarEntrada(chat, e.RaidMinutos) >= e.RaidMax {
		s.alarmaRaid(ctx, chat, e)
		return "raid"
	}
	return ""
}

// contarEntrada anota una entrada y dice cuántas hubo en los últimos `minutos` minutos en ese grupo.
func (s *Servicio) contarEntrada(chat string, minutos int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entradas == nil {
		s.entradas = map[string][]time.Time{}
	}
	ahora := s.ahora()
	corte := ahora.Add(-time.Duration(minutos) * time.Minute)
	var vivas []time.Time
	for _, t := range s.entradas[chat] {
		if t.After(corte) {
			vivas = append(vivas, t)
		}
	}
	vivas = append(vivas, ahora)
	s.entradas[chat] = vivas
	return len(vivas)
}

// alarmaRaid cierra el grupo, avisa en él y deja una notificación local. Solo salta una vez por ráfaga.
func (s *Servicio) alarmaRaid(ctx context.Context, chat string, e almacen.ProteccionEntrada) {
	s.mu.Lock()
	if t, ok := s.raidAvisado[chat]; ok && s.ahora().Sub(t) < 10*time.Minute {
		s.mu.Unlock()
		return
	}
	if s.raidAvisado == nil {
		s.raidAvisado = map[string]time.Time{}
	}
	s.raidAvisado[chat] = s.ahora()
	s.entradas[chat] = nil
	s.mu.Unlock()
	cerrado := false
	if err := s.Cerrar(ctx, almacen.PersonaDueno, chat, true); err == nil {
		cerrado = true
		s.registra(almacen.PersonaDueno, "grupo.raid", "", chat, "ok", "")
		if s.Enviar != nil {
			_ = s.Enviar(ctx, chat, "🚨 Entraron muchas cuentas en muy poco tiempo. Por seguridad cerré el grupo: solo los administradores pueden escribir hasta que lo abran.", "", nil)
		}
	} else {
		s.registra(almacen.PersonaDueno, "grupo.raid", "", chat, "error", "")
	}
	cuerpo := fmt.Sprintf("Entraron %d cuentas en menos de %d minutos a %s.", e.RaidMax, e.RaidMinutos, s.nombreGrupo(chat))
	if cerrado {
		cuerpo += " Cerré el grupo."
	} else {
		cuerpo += " No pude cerrar el grupo (¿soy administrador?)."
	}
	_, _ = s.Alm.CrearAvisoDe(s.BotID, "filtros", chat, "", "Posible ataque al grupo", cuerpo)
}
