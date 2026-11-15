package grupos

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	for _, h := range s.Alm.HorariosActivos(s.BotID) {
		quiero := h.EstadoDeseado(ahora)
		if quiero == "" || quiero == h.Ultimo {
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
			return "Listo: la persona salió y no podrá volver a este grupo.", nil
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
			return "Listo: ya puede volver a entrar (por enlace o si la agregan).", nil
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
				return "No hay nadie baneado en este grupo.", nil
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
				return "Listo: el grupo ya no se cierra solo.", nil
			case len(resto) == 2:
				h.Activo, h.Cierra, h.Abre = true, resto[0], resto[1]
				if err := s.Alm.GuardarHorario(s.BotID, h); err != nil {
					return "", err
				}
				return fmt.Sprintf("Listo: se cerrará a las %s y se abrirá a las %s (hora de este teléfono).", h.Cierra, h.Abre), nil
			}
			return "Escribe: !horariogrupo 22:00 07:00  (o !horariogrupo off).", nil
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
