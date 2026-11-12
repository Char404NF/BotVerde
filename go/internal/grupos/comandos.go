package grupos

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
)

func ctxCmd() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 40*time.Second)
}

// grupoDe dice sobre qué grupo actúa el comando: el grupo donde se escribió; o, para el dueño desde "Mensajes guardados", el que
// nombre el primer argumento (#parte-del-nombre o el id que termina en @g.us). Devuelve el grupo y los argumentos que sobran.
func (s *Servicio) grupoDe(cx *comandos.Contexto) (string, []string, error) {
	if cx.EsGrupo {
		return cx.Chat, cx.Args, nil
	}
	if !cx.Dueno {
		return "", nil, ErrNoEsGrupo
	}
	for i, a := range cx.Args {
		resto := append(append([]string(nil), cx.Args[:i]...), cx.Args[i+1:]...)
		switch {
		case strings.HasSuffix(a, "@g.us"):
			return a, resto, nil
		case strings.HasPrefix(a, "#") && len(a) > 1:
			cs, _ := s.Alm.ListarChats(s.BotID, true, strings.ReplaceAll(a[1:], "_", " "), 6, 0)
			switch len(cs) {
			case 0:
				return "", nil, errors.New("no encontré un grupo con ese nombre")
			case 1:
				return cs[0].JID, resto, nil
			}
			var n []string
			for _, c := range cs {
				n = append(n, c.Nombre)
			}
			return "", nil, errors.New("hay varios grupos con ese nombre (" + strings.Join(n, ", ") + "): escribe más letras")
		}
	}
	return "", nil, errors.New("dime de qué grupo: escribe # y parte de su nombre (por ejemplo #familia)")
}

// personaObjetivo busca a quién se refiere el comando (mención, mensaje citado o número).
func personaObjetivo(cx *comandos.Contexto) (string, []string, error) {
	p, resto, ok := cx.Objetivo()
	if !ok {
		return "", nil, ErrSinPersona
	}
	return p, resto, nil
}

func textoFecha(t string) string {
	if t == "" {
		return "no se sabe"
	}
	return t
}

func diaCorto(d string) string {
	if t, err := time.Parse("2006-01-02", d); err == nil {
		return t.Format("02/01/2006")
	}
	return d
}

func siNo(b bool, si, no string) string {
	if b {
		return si
	}
	return no
}

func nombresDe(ms []Miembro) string {
	var n []string
	for _, m := range ms {
		x := m.Nombre
		if m.EsCreador {
			x += " (creó el grupo)"
		}
		n = append(n, x)
	}
	return strings.Join(n, ", ")
}

// TextoInfo es la información del grupo en palabras.
func TextoInfo(i *InfoGrupo) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*%s*\n", i.Nombre)
	if i.Descripcion != "" {
		fmt.Fprintf(&b, "%s\n", i.Descripcion)
	}
	fmt.Fprintf(&b, "\nMiembros: %d\nCreado: %s", i.Miembros, textoFecha(i.CreadoTexto))
	if i.CreadoPor != "" {
		fmt.Fprintf(&b, " por %s", i.CreadoPor)
	}
	fmt.Fprintf(&b, "\nAdministradores: %d", len(i.Admins))
	fmt.Fprintf(&b, "\nQuién escribe: %s", siNo(i.SoloAdmins, "solo administradores", "todos"))
	fmt.Fprintf(&b, "\nQuién cambia nombre y foto: %s", siNo(i.InfoBloqueada, "solo administradores", "todos"))
	fmt.Fprintf(&b, "\nEntrar: %s", siNo(i.AprobacionIngreso, "pide aprobación", "libre con el enlace"))
	if i.MensajesTemporales > 0 {
		fmt.Fprintf(&b, "\nMensajes temporales: %s", textoSegundos(i.MensajesTemporales))
	}
	if i.Reglas != "" {
		fmt.Fprintf(&b, "\n\nReglas:\n%s", i.Reglas)
	}
	return b.String()
}

func textoSegundos(s int) string {
	switch {
	case s%86400 == 0 && s >= 86400:
		return fmt.Sprintf("%d días", s/86400)
	case s%3600 == 0 && s >= 3600:
		return fmt.Sprintf("%d horas", s/3600)
	}
	return fmt.Sprintf("%d segundos", s)
}

// TextoRanking es el ranking en palabras.
func TextoRanking(r *Ranking) string {
	if len(r.Filas) == 0 {
		return fmt.Sprintf("Nadie escribió en los últimos %d días (o esta app aún no tiene datos).", r.Dias)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Actividad de los últimos %d días (%d mensajes):\n", r.Dias, r.Total)
	for _, f := range r.Filas {
		fmt.Fprintf(&b, "%d. %s: %d (%.0f%%)\n", f.Puesto, f.Nombre, f.Mensajes, f.Porcentaje)
	}
	if r.Aviso != "" {
		b.WriteString(r.Aviso)
	}
	return strings.TrimSpace(b.String())
}

// TextoInactivos lista a los inactivos (hasta 30 nombres).
func TextoInactivos(l []Inactivo, aviso string, dias int) string {
	if len(l) == 0 {
		t := fmt.Sprintf("Todos escribieron en los últimos %d días.", dias)
		if aviso != "" {
			t += " " + aviso
		}
		return t
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d personas sin mensajes en %d días:\n", len(l), dias)
	for i, x := range l {
		if i >= 30 {
			fmt.Fprintf(&b, "… y %d más.\n", len(l)-30)
			break
		}
		ult := "sin mensajes guardados"
		if x.UltimoDia != "" {
			ult = "último: " + diaCorto(x.UltimoDia)
		}
		fmt.Fprintf(&b, "• %s (%s)\n", x.Nombre, ult)
	}
	if aviso != "" {
		b.WriteString(aviso)
	}
	return strings.TrimSpace(b.String())
}

func primerNumero(args []string) (int, []string) {
	for i, a := range args {
		if n, err := strconv.Atoi(strings.TrimSuffix(strings.ToLower(a), "d")); err == nil && n > 0 {
			return n, append(append([]string(nil), args[:i]...), args[i+1:]...)
		}
	}
	return 0, args
}

func nombreFiltroCorto(arg string) (string, bool) {
	switch strings.ToLower(arg) {
	case "enlaces", "enlace", "links", "link":
		return FiltroEnlaces, true
	case "palabras", "palabra":
		return FiltroPalabras, true
	case "repetidos", "repetido":
		return FiltroRepetidos, true
	case "flood", "spam":
		return FiltroFlood, true
	}
	return "", false
}

// TextoFiltros resume los filtros de un grupo.
func TextoFiltros(fs []Filtro) string {
	var b strings.Builder
	b.WriteString("Filtros de este grupo:\n")
	for _, f := range fs {
		est := "apagado"
		if f.Activo {
			est = "encendido, acción: " + textoAccion(f.Accion)
		}
		fmt.Fprintf(&b, "• %s: %s", f.Nombre, est)
		switch f.Filtro {
		case FiltroEnlaces:
			if len(f.Config.Permitidos) > 0 {
				fmt.Fprintf(&b, " (permitidos: %s)", strings.Join(f.Config.Permitidos, ", "))
			}
		case FiltroPalabras:
			fmt.Fprintf(&b, " (%d palabras)", len(f.Config.Palabras))
		case FiltroRepetidos:
			fmt.Fprintf(&b, " (%d veces en %d s)", f.Config.Max, f.Config.Segundos)
		case FiltroFlood:
			fmt.Fprintf(&b, " (%d mensajes en %d s)", f.Config.Max, f.Config.Segundos)
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func textoAccion(a string) string {
	switch a {
	case AccAvisarme:
		return "avisarme"
	case AccAvisarGrupo:
		return "avisar al grupo"
	case AccBorrar:
		return "borrar el mensaje"
	case AccAdvertir:
		return "advertir"
	case AccSilenciar:
		return "ignorar al bot para esa persona (que no le conteste)"
	case AccExpulsar:
		return "expulsar"
	}
	return a
}

// RegistrarComandos agrega a un despachador los comandos de grupos. Los que son del grupo valen donde se escriben; el dueño puede
// usarlos desde "Mensajes guardados" indicando el grupo (#nombre).
func (s *Servicio) RegistrarComandos(d *comandos.Despachador) {
	reg := func(c comandos.Comando) { _ = d.Registrar(c) }
	validaGrupo := func(cx *comandos.Contexto) error { _, _, err := s.grupoDe(cx); return err }
	validaObjetivo := func(cx *comandos.Contexto) error {
		if _, _, err := s.grupoDe(cx); err != nil {
			return err
		}
		_, _, err := personaObjetivo(cx)
		return err
	}

	reg(comandos.Comando{Nombre: "grupo", Alias: []string{"infogrupo"}, Resumen: "Ver la información del grupo", Uso: "grupo", Casilla: "grupos.info", Ambito: comandos.Grupo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, _, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			ctx, cancel := ctxCmd()
			defer cancel()
			i, err := s.Info(ctx, chat)
			if err != nil {
				return "", err
			}
			return TextoInfo(i), nil
		}})
	reg(comandos.Comando{Nombre: "reglas", Resumen: "Ver las reglas del grupo", Uso: "reglas", Casilla: "grupos.info", Ambito: comandos.Grupo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, _, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			if r := s.Alm.ReglasGrupo(s.BotID, chat); r != "" {
				return "Reglas del grupo:\n" + r, nil
			}
			return "Este grupo todavía no tiene reglas guardadas.", nil
		}})
	reg(comandos.Comando{Nombre: "admins", Alias: []string{"administradores"}, Resumen: "Ver quiénes administran el grupo", Uso: "admins", Casilla: "grupos.admins", Ambito: comandos.Grupo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, _, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			ctx, cancel := ctxCmd()
			defer cancel()
			l, err := s.Admins(ctx, chat)
			if err != nil {
				return "", err
			}
			if len(l) == 0 {
				return "No encontré administradores.", nil
			}
			return fmt.Sprintf("Administradores (%d): %s", len(l), nombresDe(l)), nil
		}})
	reg(comandos.Comando{Nombre: "todos", Alias: []string{"everyone"}, Resumen: "Mencionar a todo el grupo", Uso: "todos [mensaje]", Casilla: "grupos.mencionar_todos", Ambito: comandos.Grupo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, resto, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			ctx, cancel := ctxCmd()
			defer cancel()
			r, err := s.MencionarTodos(ctx, cx.Persona, chat, cx.MensajeID, strings.Join(resto, " "))
			if err != nil {
				return "", err
			}
			if !cx.EsGrupo {
				return fmt.Sprintf("Listo: mencioné a %d personas.", r.Mencionadas), nil
			}
			return r.Aviso, nil
		}})
	reg(comandos.Comando{Nombre: "ranking", Resumen: "Quién escribe más en el grupo", Uso: "ranking [7|30|90]", Casilla: "grupos.ranking", Ambito: comandos.Grupo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, resto, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			dias, _ := primerNumero(resto)
			ctx, cancel := ctxCmd()
			defer cancel()
			r, err := s.Ranking(ctx, chat, dias, 10)
			if err != nil {
				return "", err
			}
			return TextoRanking(r), nil
		}})
	reg(comandos.Comando{Nombre: "inactivos", Resumen: "Quién no escribe hace tiempo", Uso: "inactivos [7|30|90]", Casilla: "grupos.inactivos", Ambito: comandos.Grupo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, resto, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			dias, _ := primerNumero(resto)
			dias = s.NormalizaDias(dias)
			ctx, cancel := ctxCmd()
			defer cancel()
			l, aviso, err := s.Inactivos(ctx, chat, dias)
			if err != nil {
				return "", err
			}
			return TextoInactivos(l, aviso, dias), nil
		}})

	simple := func(nombre, resumen, casilla string, f func(ctx context.Context, actor, chat string) error, ok string) {
		reg(comandos.Comando{Nombre: nombre, Resumen: resumen, Uso: nombre, Casilla: casilla, Ambito: comandos.Grupo,
			Ejecutar: func(cx *comandos.Contexto) (string, error) {
				chat, _, err := s.grupoDe(cx)
				if err != nil {
					return "", err
				}
				ctx, cancel := ctxCmd()
				defer cancel()
				if err := f(ctx, cx.Persona, chat); err != nil {
					return "", err
				}
				return ok, nil
			}})
	}
	simple("cerrar", "Cerrar el grupo: solo administradores escriben", "grupos.cerrar",
		func(ctx context.Context, a, c string) error { return s.Cerrar(ctx, a, c, true) }, "Listo: el grupo quedó cerrado, solo los administradores pueden escribir.")
	simple("abrir", "Abrir el grupo: todos pueden escribir", "grupos.cerrar",
		func(ctx context.Context, a, c string) error { return s.Cerrar(ctx, a, c, false) }, "Listo: el grupo quedó abierto, todos pueden escribir.")
	simple("bloquearinfo", "Que solo administradores cambien nombre y foto", "grupos.bloquear_info",
		func(ctx context.Context, a, c string) error { return s.BloquearInfo(ctx, a, c, true) }, "Listo: solo los administradores pueden cambiar la información del grupo.")
	simple("desbloquearinfo", "Que todos puedan cambiar nombre y foto", "grupos.bloquear_info",
		func(ctx context.Context, a, c string) error { return s.BloquearInfo(ctx, a, c, false) }, "Listo: todos pueden cambiar la información del grupo.")

	reg(comandos.Comando{Nombre: "expulsar", Resumen: "Sacar a una persona del grupo", Uso: "expulsar <persona>", Casilla: "grupos.expulsar", Ambito: comandos.Grupo, Confirma: true, Validar: validaObjetivo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, _, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			p, _, err := personaObjetivo(cx)
			if err != nil {
				return "", err
			}
			ctx, cancel := ctxCmd()
			defer cancel()
			if err := s.ExpulsarComo(ctx, cx.Persona, chat, p); err != nil {
				return "", err
			}
			return "Listo: la persona salió del grupo.", nil
		}})
	admin := func(nombre, resumen string, f func(ctx context.Context, a, c, p string) error, ok string) {
		reg(comandos.Comando{Nombre: nombre, Resumen: resumen, Uso: nombre + " <persona>", Casilla: "grupos.ascender", Ambito: comandos.Grupo, Validar: validaObjetivo,
			Ejecutar: func(cx *comandos.Contexto) (string, error) {
				chat, _, err := s.grupoDe(cx)
				if err != nil {
					return "", err
				}
				p, _, err := personaObjetivo(cx)
				if err != nil {
					return "", err
				}
				ctx, cancel := ctxCmd()
				defer cancel()
				if err := f(ctx, cx.Persona, chat, p); err != nil {
					return "", err
				}
				return ok, nil
			}})
	}
	admin("ascender", "Hacer administradora a una persona", s.Ascender, "Listo: ahora es administradora del grupo.")
	admin("bajar", "Quitar el cargo de administradora", s.Bajar, "Listo: ya no es administradora del grupo.")

	reg(comandos.Comando{Nombre: "solicitudes", Resumen: "Ver quién pide entrar al grupo", Uso: "solicitudes", Casilla: "grupos.solicitudes", Ambito: comandos.Grupo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, _, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			ctx, cancel := ctxCmd()
			defer cancel()
			l, err := s.Solicitudes(ctx, cx.Persona, chat)
			if err != nil {
				return "", err
			}
			if len(l) == 0 {
				return "No hay solicitudes pendientes.", nil
			}
			var b strings.Builder
			fmt.Fprintf(&b, "%d solicitudes:\n", len(l))
			for i, x := range l {
				if i >= 30 {
					fmt.Fprintf(&b, "… y %d más.\n", len(l)-30)
					break
				}
				fmt.Fprintf(&b, "• %s\n", x.Nombre)
			}
			fmt.Fprintf(&b, "Usa %saprobar o %srechazar (con el número, para una sola persona).", d.Prefijo(), d.Prefijo())
			return b.String(), nil
		}})
	resolver := func(nombre, resumen string, aprobar bool, ok string) {
		reg(comandos.Comando{Nombre: nombre, Resumen: resumen, Uso: nombre + " [persona]", Casilla: "grupos.solicitudes", Ambito: comandos.Grupo,
			Ejecutar: func(cx *comandos.Contexto) (string, error) {
				chat, _, err := s.grupoDe(cx)
				if err != nil {
					return "", err
				}
				var quienes []string
				if p, _, e := personaObjetivo(cx); e == nil {
					quienes = []string{p}
				}
				ctx, cancel := ctxCmd()
				defer cancel()
				n, err := s.ResolverSolicitudes(ctx, cx.Persona, chat, quienes, aprobar)
				if err != nil {
					return "", err
				}
				return fmt.Sprintf(ok, n), nil
			}})
	}
	resolver("aprobar", "Aceptar solicitudes de ingreso (todas o una persona)", true, "Listo: aprobé %d solicitudes.")
	resolver("rechazar", "Rechazar solicitudes de ingreso (todas o una persona)", false, "Listo: rechacé %d solicitudes.")

	reg(comandos.Comando{Nombre: "nombre", Resumen: "Cambiar el nombre del grupo", Uso: "nombre <texto>", Casilla: "grupos.renombrar", Ambito: comandos.Grupo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, resto, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			ctx, cancel := ctxCmd()
			defer cancel()
			if err := s.Renombrar(ctx, cx.Persona, chat, strings.Join(resto, " ")); err != nil {
				return "", err
			}
			return "Listo: cambié el nombre del grupo.", nil
		}})
	reg(comandos.Comando{Nombre: "descripcion", Alias: []string{"descripción"}, Resumen: "Cambiar la descripción del grupo", Uso: "descripcion <texto>  (sin texto la borra)", Casilla: "grupos.renombrar", Ambito: comandos.Grupo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, resto, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			ctx, cancel := ctxCmd()
			defer cancel()
			t := strings.Join(resto, " ")
			if err := s.Descripcion(ctx, cx.Persona, chat, t); err != nil {
				return "", err
			}
			if t == "" {
				return "Listo: borré la descripción del grupo.", nil
			}
			return "Listo: cambié la descripción del grupo.", nil
		}})
	reg(comandos.Comando{Nombre: "enlace", Alias: []string{"link"}, Resumen: "Ver el enlace de invitación", Uso: "enlace", Casilla: "grupos.enlace", Ambito: comandos.Grupo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, _, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			ctx, cancel := ctxCmd()
			defer cancel()
			l, err := s.Enlace(ctx, cx.Persona, chat, false)
			if err != nil {
				return "", err
			}
			return l, nil
		}})
	reg(comandos.Comando{Nombre: "enlacenuevo", Alias: []string{"reiniciarenlace"}, Resumen: "Cambiar el enlace de invitación (el anterior deja de servir)", Uso: "enlacenuevo", Casilla: "grupos.enlace",
		Ambito: comandos.Grupo, Confirma: true, Validar: validaGrupo,
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, _, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			ctx, cancel := ctxCmd()
			defer cancel()
			l, err := s.Enlace(ctx, cx.Persona, chat, true)
			if err != nil {
				return "", err
			}
			return "Enlace nuevo (el anterior ya no sirve): " + l, nil
		}})
	reg(comandos.Comando{Nombre: "borrar", Alias: []string{"eliminar"}, Resumen: "Borrar el mensaje que citas (de cualquiera)", Uso: "borrar  (respondiendo al mensaje)", Casilla: "grupos.borrar_mensaje",
		Ambito: comandos.Grupo, Confirma: true,
		Validar: func(cx *comandos.Contexto) error {
			if _, _, err := s.grupoDe(cx); err != nil {
				return err
			}
			if cx.CitaID == "" {
				return errors.New("responde al mensaje que quieres borrar y escribe el comando")
			}
			return nil
		},
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, _, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			if cx.CitaID == "" {
				return "", errors.New("falta el mensaje que se iba a borrar")
			}
			ctx, cancel := ctxCmd()
			defer cancel()
			if err := s.BorrarMensaje(ctx, cx.Persona, chat, cx.CitaID); err != nil {
				return "", err
			}
			return "Listo: borré ese mensaje.", nil
		}})
	reg(comandos.Comando{Nombre: "filtros", Resumen: "Ver y cambiar los filtros del grupo", Casilla: "grupos.filtros", Ambito: comandos.Grupo,
		Uso: "filtros | filtros <enlaces|palabras|repetidos|flood> on|off | accion <acción> | agregar/quitar <texto> | permitir <dominio> | limite <n> <segundos>",
		Ejecutar: func(cx *comandos.Contexto) (string, error) {
			chat, resto, err := s.grupoDe(cx)
			if err != nil {
				return "", err
			}
			if len(resto) == 0 {
				return TextoFiltros(s.FiltrosDe(chat)), nil
			}
			nombre, ok := nombreFiltroCorto(resto[0])
			if !ok {
				return "", ErrFiltroNoHay
			}
			var f Filtro
			for _, x := range s.FiltrosDe(chat) {
				if x.Filtro == nombre {
					f = x
				}
			}
			if len(resto) == 1 {
				return TextoFiltros([]Filtro{f}), nil
			}
			sub := strings.ToLower(resto[1])
			valor := strings.TrimSpace(strings.Join(resto[2:], " "))
			msg := "Listo."
			switch sub {
			case "on", "si", "sí", "encender", "activar":
				f.Activo, msg = true, "Listo: el filtro de "+strings.ToLower(f.Nombre)+" quedó encendido."
			case "off", "no", "apagar", "desactivar":
				f.Activo, msg = false, "Listo: el filtro de "+strings.ToLower(f.Nombre)+" quedó apagado."
			case "accion", "acción":
				a := strings.ReplaceAll(strings.ToLower(valor), " ", "_")
				if !EsAccionFiltro(a) {
					return "", ErrAccionNoHay
				}
				f.Accion, msg = a, "Listo: la acción es "+textoAccion(a)+"."
			case "agregar", "añadir":
				if nombre != FiltroPalabras || valor == "" {
					return "", errors.New("solo el filtro de palabras acepta eso: filtros palabras agregar <palabra>")
				}
				f.Config.Palabras = append(f.Config.Palabras, valor)
				msg = "Listo: agregué esa palabra."
			case "permitir":
				if nombre != FiltroEnlaces || valor == "" {
					return "", errors.New("solo el filtro de enlaces acepta eso: filtros enlaces permitir <dominio>")
				}
				f.Config.Permitidos = append(f.Config.Permitidos, valor)
				msg = "Listo: ese dominio queda permitido."
			case "quitar":
				switch nombre {
				case FiltroPalabras:
					f.Config.Palabras = sinElemento(f.Config.Palabras, valor, func(x string) string { return strings.ToLower(x) })
				case FiltroEnlaces:
					f.Config.Permitidos = sinElemento(f.Config.Permitidos, valor, normalizaDominio)
				default:
					return "", errors.New("ese filtro no tiene lista")
				}
				msg = "Listo: lo quité de la lista."
			case "limite", "límite":
				if nombre != FiltroRepetidos && nombre != FiltroFlood {
					return "", errors.New("solo repetidos y flood tienen límite: filtros flood limite <mensajes> <segundos>")
				}
				n, rest := primerNumero(resto[2:])
				seg, _ := primerNumero(rest)
				if n == 0 || seg == 0 {
					return "", errors.New("escribe el límite así: filtros " + nombre + " limite 6 10 (6 mensajes en 10 segundos)")
				}
				f.Config.Max, f.Config.Segundos = n, seg
				msg = fmt.Sprintf("Listo: el límite es %d en %d segundos.", n, seg)
			default:
				return "", errors.New("no entendí: usa on, off, accion, agregar, quitar, permitir o limite")
			}
			if err := s.GuardarFiltro(cx.Persona, f); err != nil {
				return "", err
			}
			if f.Activo && (f.Accion == AccBorrar || f.Accion == AccExpulsar) {
				ctx, cancel := ctxCmd()
				defer cancel()
				if ok, _ := s.EsAdmin(ctx, chat); !ok {
					msg += " Ojo: no soy administrador, así que solo te avisaré con una notificación."
				}
			}
			return msg, nil
		}})
	_ = almacen.PersonaDueno
}

func sinElemento(l []string, valor string, norm func(string) string) []string {
	out := []string{}
	for _, x := range l {
		if norm(x) != norm(valor) {
			out = append(out, x)
		}
	}
	return out
}
