package flujo

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/motor"
)

type evento = struct{ Paso, Opcion, Resultado string }

func evt(paso, opcion, res string) evento { return evento{paso, opcion, res} }

func refDe(s *almacen.Sesion) *motor.RefSesion {
	return &motor.RefSesion{ID: s.ID, Persona: s.Sender, MenuID: s.MenuID}
}

// ---------- opciones visibles ----------

// visibles son las opciones que esa persona puede ver y elegir ahora: por rango (la casilla "menús que ve" del rango), por ámbito
// (privado o grupo) y por horario de la opción. Los números que se le muestran van de 1 en adelante sobre lo que ve.
func (m *Motor) visibles(def *Def, menuID int64, p *Paso, sender string, grupo bool) []visible {
	ahora := m.c.Ahora().In(m.c.Zona())
	var out []visible
	for i, o := range p.Opciones {
		if o.Ambito == "grupo" && !grupo {
			continue
		}
		if o.Horario != nil && !o.Horario.contiene(ahora) {
			continue
		}
		if m.c.OpcionVisible != nil && o.ID != "" && !m.c.OpcionVisible(sender, menuID, o.ID) {
			continue
		}
		out = append(out, visible{Idx: i, Num: len(out) + 1, O: o})
	}
	return out
}

func (m *Motor) menuTexto(def *Def, p *Paso, s *almacen.Sesion, e entrada) string {
	grupo := esGrupoChat(s.Chat)
	vis := m.visibles(def, s.MenuID, p, s.Sender, grupo)
	var b strings.Builder
	b.WriteString(m.sustituye(def, p.Titulo, entrada{Chat: s.Chat, Sender: s.Sender}, m.c.Alm.Variables(s.ID)))
	for _, v := range vis {
		fmt.Fprintf(&b, "\n%d. %s", v.Num, v.O.Texto)
		if grupo && v.O.Ambito == "privado" {
			b.WriteString(" (solo en privado)")
		}
	}
	if len(vis) == 0 {
		b.WriteString("\nPor ahora no hay opciones disponibles.")
	}
	if p.ID != def.Inicio {
		fmt.Fprintf(&b, "\n0. %s", textoMenuPrinc)
	}
	return b.String()
}

// mostrarPaso repite lo que se le preguntó a la persona en el paso donde va ("repetir").
func (m *Motor) mostrarPaso(def *Def, s *almacen.Sesion, p *Paso, e entrada) []motor.Salida {
	switch p.Tipo {
	case PasoMenu:
		return []motor.Salida{{Texto: m.menuTexto(def, p, s, e)}}
	case PasoPedir:
		return []motor.Salida{{Texto: m.sustituye(def, p.Pregunta, entrada{Chat: s.Chat, Sender: s.Sender}, m.c.Alm.Variables(s.ID))}}
	case PasoForm:
		return []motor.Salida{{Texto: m.pantallaForm(def, s, p)}}
	}
	return nil
}

// ---------- continuar una conversación ----------

func (m *Motor) continua(def *Def, s *almacen.Sesion, e entrada, prueba bool) (resultado, error) {
	p := def.Paso(s.Paso)
	if p == nil { // le borraron el paso: vuelve al menú principal
		return m.ejecuta(def, s, def.Inicio, e, prueba)
	}
	r, err := m.continuaPaso(def, s, p, e, prueba)
	if r.Sesion == nil {
		r.Sesion = refDe(s)
	}
	return r, err
}

func (m *Motor) continuaPaso(def *Def, s *almacen.Sesion, p *Paso, e entrada, prueba bool) (resultado, error) {
	ahora := m.c.Ahora()
	bot, alm := m.c.BotID, m.c.Alm
	if e.NoTexto {
		return m.respondeNoTexto(def, s, e, prueba), nil
	}
	texto := strings.TrimSpace(e.Texto)
	if p.Tipo == PasoForm {
		return m.continuaForm(def, s, p, e, prueba)
	}
	if s.EsperaDato && p.Tipo == PasoPedir {
		if cmd := m.comandoSuelto(texto); cmd != "" {
			return m.aplicaComando(cmd, def, s, p, e, prueba)
		}
		valor := texto
		if e.TextoCompleto != "" {
			valor = strings.TrimSpace(e.TextoCompleto)
		}
		_ = alm.GuardarVariable(s.ID, p.Variable, valor)
		_ = alm.MoverSesion(bot, s, p.ID, false, 0, false, evt(p.ID, "dato", "valida"), ahora)
		if p.Siguiente == "" {
			_ = alm.CerrarSesion(bot, s, almacen.SesCompletada, 0, ahora)
			return resultado{Estado: almacen.SesCompletada}, nil
		}
		return m.ejecuta(def, s, p.Siguiente, e, prueba)
	}
	if p.Tipo != PasoMenu {
		// El paso no espera nada (la conversación quedó en un final): se vuelve a empezar desde el menú principal.
		return m.ejecuta(def, s, def.Inicio, e, prueba)
	}
	vis := m.visibles(def, s.MenuID, p, s.Sender, esGrupoChat(s.Chat))
	// "0" siempre vuelve al menú principal.
	if n, ok := numeroDeOpcion(texto); ok && n == 0 {
		alm.LimpiarIntervencion(s.ID)
		return m.aplicaComando(CmdMenu, def, s, p, e, prueba)
	}
	if v, ok := eligeExacta(vis, texto); ok {
		return m.eligio(def, s, p, v, e, prueba)
	}
	if cmd := m.comandoSuelto(texto); cmd != "" {
		alm.LimpiarIntervencion(s.ID)
		return m.aplicaComando(cmd, def, s, p, e, prueba)
	}
	v, amb, ok := eligeParcial(vis, texto)
	if ok {
		return m.eligio(def, s, p, v, e, prueba)
	}
	if len(amb) > 1 { // varias opciones parecidas: se pide aclarar (no cuenta como error)
		alm.LimpiarIntervencion(s.ID)
		_ = alm.MoverSesion(bot, s, p.ID, false, s.Invalidas, false, evt(p.ID, "", "ambiguo"), ahora)
		var b strings.Builder
		b.WriteString(m.textoDe(def, "menu_txt_ambiguo"))
		for _, a := range amb {
			fmt.Fprintf(&b, "\n%d. %s", a.Num, a.O.Texto)
		}
		return resultado{Salidas: []motor.Salida{{Texto: b.String()}}, Estado: almacen.SesActiva, Paso: p.ID}, nil
	}
	// Si intervine a mano en esta conversación, el texto libre es para mí: el bot ni contesta "opción no válida" ni la cuenta como error.
	if !prueba && alm.SesionIntervenida(s.ID) {
		return resultado{Omitir: almacen.MotIntervino, Estado: almacen.SesActiva, Paso: p.ID}, nil
	}
	// Un saludo, un "gracias" o un emoji suelto no es un error: se le vuelve a mostrar el menú sin contar nada.
	if m.esNeutro(texto) {
		_ = alm.MoverSesion(bot, s, p.ID, false, s.Invalidas, false, evt(p.ID, "", "neutro"), ahora)
		return resultado{Salidas: []motor.Salida{{Texto: m.menuTexto(def, p, s, e)}}, Estado: almacen.SesActiva, Paso: p.ID}, nil
	}
	return m.errorDeMenu(def, s, p, e, prueba), nil
}

// eligio lleva a la persona a donde pidió la opción.
func (m *Motor) eligio(def *Def, s *almacen.Sesion, p *Paso, v visible, e entrada, prueba bool) (resultado, error) {
	ahora := m.c.Ahora()
	bot, alm := m.c.BotID, m.c.Alm
	alm.LimpiarIntervencion(s.ID)
	_ = alm.MoverSesion(bot, s, p.ID, false, 0, false, evt(p.ID, strconv.Itoa(v.Idx+1), "valida"), ahora)
	if esGrupoChat(s.Chat) && v.O.Ambito == "privado" {
		// el bot no escribe al privado por su cuenta: solo le dice que él le escriba
		r := resultado{Estado: almacen.SesActiva, Paso: p.ID}
		r.texto(m.ajuste("grupo_txt_privado"))
		return r, nil
	}
	if v.O.Destino == "" || def.Paso(v.O.Destino) == nil {
		return m.ejecuta(def, s, def.Inicio, e, prueba)
	}
	return m.ejecuta(def, s, v.O.Destino, e, prueba)
}

// ---------- escalera de errores ----------

func (m *Motor) errMax(def *Def) int {
	if def.Ajustes != nil && def.Ajustes.ErrMax > 0 {
		return def.Ajustes.ErrMax
	}
	if n := m.ajusteInt("menu_err_max"); n > 0 {
		return n
	}
	return 3
}

// textoDe da el texto editable: el del menú si lo trae, y si no el general.
func (m *Motor) textoDe(def *Def, clave string) string {
	if a := def.Ajustes; a != nil {
		var v string
		switch clave {
		case "menu_txt_n1":
			v = a.TxtN1
		case "menu_txt_n2":
			v = a.TxtN2
		case "menu_txt_n3":
			v = a.TxtN3
		case "menu_txt_notexto":
			v = a.TxtNoTx
		case "menu_humano_dentro":
			v = a.HumanoDentro
		case "menu_humano_fuera":
			v = a.HumanoFuera
		}
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return m.ajuste(clave)
}

// errorDeMenu aplica la escalera: la 1.ª reconoce y ofrece las opciones, la 2.ª ofrece hablar con una persona y la que llega al
// tope para y cierra la conversación (el siguiente mensaje empieza desde cero según las reglas): nunca queda en un ciclo.
func (m *Motor) errorDeMenu(def *Def, s *almacen.Sesion, p *Paso, e entrada, prueba bool) resultado {
	ahora := m.c.Ahora()
	bot, alm := m.c.BotID, m.c.Alm
	inv := s.Invalidas + 1
	if !prueba && e.Dirigido {
		m.anotaNoEntendio(def, s, p.ID, e.Texto, ahora)
	}
	r := resultado{Estado: almacen.SesActiva, Paso: p.ID}
	_ = alm.MoverSesion(bot, s, p.ID, false, inv, false, evt(p.ID, "", "invalida"), ahora)
	if inv >= m.errMax(def) {
		_ = alm.CerrarSesion(bot, s, almacen.SesAbandonada, 0, ahora)
		alm.FijarCierre(s.ID, "errores")
		r.Estado = almacen.SesAbandonada
		r.texto(m.textoDe(def, "menu_txt_n3"))
		return r
	}
	nivel := "menu_txt_n1"
	if inv >= 2 {
		nivel = "menu_txt_n2"
	}
	r.Salidas = append(r.Salidas, motor.Salida{Texto: strings.TrimSpace(m.textoDe(def, nivel) + "\n" + m.menuTexto(def, p, s, e))})
	return r
}

func (m *Motor) anotaNoEntendio(def *Def, s *almacen.Sesion, paso, texto string, ahora time.Time) {
	if m.ajuste("noentendio_activo") != "1" || strings.TrimSpace(texto) == "" {
		return
	}
	_ = m.c.Alm.GuardarNoEntendio(m.c.BotID, s.MenuID, paso, strings.TrimSpace(texto), ahora)
}

// respondeNoTexto: llegó una imagen, un audio, un sticker o un archivo y el menú espera texto. No cuenta como error.
func (m *Motor) respondeNoTexto(def *Def, s *almacen.Sesion, e entrada, prueba bool) resultado {
	if !prueba && m.c.Alm.SesionIntervenida(s.ID) {
		return resultado{Omitir: almacen.MotIntervino, Estado: almacen.SesActiva, Paso: s.Paso}
	}
	_ = m.c.Alm.MoverSesion(m.c.BotID, s, s.Paso, s.EsperaDato, s.Invalidas, false, evt(s.Paso, "", "no_texto"), m.c.Ahora())
	r := resultado{Estado: almacen.SesActiva, Paso: s.Paso}
	r.texto(m.textoDe(def, "menu_txt_notexto"))
	return r
}

// ---------- comandos dentro de un menú ----------

func (m *Motor) pushPila(s *almacen.Sesion, id string) {
	pila := splitPila(s.Pila)
	if n := len(pila); n > 0 && pila[n-1] == id {
		return
	}
	pila = append(pila, id)
	if len(pila) > 20 {
		pila = pila[len(pila)-20:]
	}
	m.setPila(s, pila)
}

func splitPila(p string) []string {
	if p == "" {
		return nil
	}
	return strings.Split(p, ",")
}

func (m *Motor) setPila(s *almacen.Sesion, pila []string) {
	s.Pila = strings.Join(pila, ",")
	_ = m.c.Alm.FijarPila(s.ID, s.Pila)
}

// aplicaComando hace lo que pide una palabra suelta: menú, inicio, atrás, salir, persona, repetir o cancelar.
func (m *Motor) aplicaComando(cmd string, def *Def, s *almacen.Sesion, p *Paso, e entrada, prueba bool) (resultado, error) {
	ahora := m.c.Ahora()
	bot, alm := m.c.BotID, m.c.Alm
	paso := s.Paso
	sigue := func(opc string) {
		_ = alm.MoverSesion(bot, s, paso, s.EsperaDato, 0, false, evt(paso, "cmd:"+opc, "comando"), ahora)
	}
	switch cmd {
	case CmdMenu:
		m.descartaForm(s, p)
		sigue(cmd)
		m.setPila(s, nil)
		return m.ejecuta(def, s, def.Inicio, e, prueba)
	case CmdInicio:
		m.descartaForm(s, p)
		sigue(cmd)
		alm.BorrarVariables(s.ID)
		m.setPila(s, nil)
		return m.ejecuta(def, s, def.Inicio, e, prueba)
	case CmdAtras:
		m.descartaForm(s, p)
		sigue(cmd)
		pila := splitPila(s.Pila)
		if p.Tipo == PasoMenu && len(pila) > 0 && pila[len(pila)-1] == p.ID {
			pila = pila[:len(pila)-1]
		}
		destino := def.Inicio
		if n := len(pila); n > 0 {
			destino = pila[n-1]
			pila = pila[:n-1]
		}
		m.setPila(s, pila)
		return m.ejecuta(def, s, destino, e, prueba)
	case CmdSalir, CmdCancelar:
		m.descartaForm(s, p)
		sigue(cmd)
		_ = alm.CerrarSesion(bot, s, almacen.SesAbandonada, 0, ahora)
		alm.FijarCierre(s.ID, "salio")
		r := resultado{Estado: almacen.SesAbandonada, Paso: paso}
		if cmd == CmdSalir {
			r.texto(m.ajuste("menu_txt_salir"))
		} else {
			r.texto(m.ajuste("menu_txt_cancelar"))
		}
		return r, nil
	case CmdPersona:
		sigue(cmd)
		return m.pasarAHumano(def, s, e, prueba, nil), nil
	case CmdRepetir:
		sigue(cmd)
		return resultado{Salidas: m.mostrarPaso(def, s, p, e), Estado: almacen.SesActiva, Paso: paso}, nil
	}
	return resultado{Estado: almacen.SesActiva, Paso: paso}, nil
}

// ---------- hablar con una persona ----------

// horarioAtencion es el horario en que alguien atiende: el del menú si lo trae y, si no, el general de Ajustes (vacío = siempre).
func (m *Motor) horarioAtencion(def *Def) (Horario, bool) {
	if len(def.Horario.Dias) > 0 || def.Horario.Desde != "" || def.Horario.Hasta != "" {
		return def.Horario, true
	}
	if j := strings.TrimSpace(m.ajuste("menu_horario")); j != "" {
		var h Horario
		if json.Unmarshal([]byte(j), &h) == nil && (len(h.Dias) > 0 || h.Desde != "" || h.Hasta != "") {
			return h, true
		}
	}
	return Horario{}, false
}

func (m *Motor) dentroDeHorario(def *Def) bool {
	h, tiene := m.horarioAtencion(def)
	return !tiene || h.contiene(m.c.Ahora().In(m.c.Zona()))
}

// pasarAHumano cierra la conversación "con una persona": marca el chat Pendiente, avisa con una notificación local, le dice a la
// persona si es horario de atención o no (y conserva lo que ya había contestado) y el bot calla ahí.
func (m *Motor) pasarAHumano(def *Def, s *almacen.Sesion, e entrada, prueba bool, p *Paso) resultado {
	ahora := m.c.Ahora()
	bot, alm := m.c.BotID, m.c.Alm
	vars := alm.Variables(s.ID)
	var t string
	if m.dentroDeHorario(def) {
		if p != nil {
			t = p.Texto
		}
		if strings.TrimSpace(t) == "" {
			t = m.textoDe(def, "menu_humano_dentro")
		}
	} else {
		if p != nil {
			t = p.TextoFuera
		}
		if strings.TrimSpace(t) == "" {
			t = m.textoDe(def, "menu_humano_fuera")
		}
	}
	r := resultado{Estado: almacen.SesHumano, Humano: true, Paso: s.Paso, Sesion: refDe(s)}
	r.texto(m.sustituye(def, t, e, vars))
	_ = alm.CerrarSesion(bot, s, almacen.SesHumano, ahora.Add(12*time.Hour).Unix(), ahora)
	alm.FijarCierre(s.ID, "persona")
	if !prueba {
		_ = alm.FijarEstadoTrabajo(bot, e.Chat, almacen.TrabajoPendiente)
		if m.c.AlAviso != nil {
			m.c.AlAviso(e.Chat, "humano", "Alguien pidió hablar con una persona", m.detalleHumano(def, e, vars))
		}
	}
	return r
}

func (m *Motor) detalleHumano(def *Def, e entrada, vars map[string]string) string {
	n := m.c.Nombre(e.Chat, e.Sender)
	cab := n
	if c, _ := m.c.Alm.ObtenerChat(m.c.BotID, e.Chat); c != nil && c.TelMostrado != "" {
		cab = strings.TrimSpace(n + " · " + c.TelMostrado)
	}
	if d := m.resumenDatos(def, vars); d != "" {
		return strings.TrimSpace(cab + "\n" + d)
	}
	return cab
}

// resumenDatos lista lo que la persona ya había contestado (preguntas libres y formularios), una línea por dato.
func (m *Motor) resumenDatos(def *Def, vars map[string]string) string {
	var b []string
	for _, q := range def.Pasos {
		switch q.Tipo {
		case PasoPedir:
			if v := strings.TrimSpace(vars[q.Variable]); v != "" {
				b = append(b, corta(q.Pregunta, 30)+": "+v)
			}
		case PasoForm:
			for _, c := range q.Campos {
				if v := strings.TrimSpace(vars[c.ID]); v != "" {
					b = append(b, c.etiqueta()+": "+v)
				}
			}
		}
	}
	return strings.Join(b, "\n")
}

func corta(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// ---------- ejecutar pasos ----------

func (m *Motor) ejecuta(def *Def, s *almacen.Sesion, id string, e entrada, prueba bool) (resultado, error) {
	ahora := m.c.Ahora()
	bot, alm := m.c.BotID, m.c.Alm
	var r resultado
	r.Estado = almacen.SesActiva
	r.Sesion = refDe(s)
	vars := alm.Variables(s.ID)
	for guarda := 0; guarda < 25; guarda++ {
		p := def.Paso(id)
		if p == nil {
			if id != def.Inicio {
				id = def.Inicio
				continue
			}
			_ = alm.CerrarSesion(bot, s, almacen.SesCompletada, 0, ahora)
			r.Estado = almacen.SesCompletada
			return r, nil
		}
		alm.VisitaPaso(bot, s, p.ID, ahora)
		r.Paso = p.ID
		switch p.Tipo {
		case PasoMenu:
			_ = alm.MoverSesion(bot, s, p.ID, false, 0, false, evt(p.ID, "", "mostrado"), ahora)
			m.pushPila(s, p.ID)
			r.Salidas = append(r.Salidas, motor.Salida{Texto: m.menuTexto(def, p, s, e)})
			return r, nil
		case PasoRespuesta:
			r.Salidas = append(r.Salidas, m.salidasDeRespuesta(def, p, e, vars)...)
			if p.Notificar && !prueba && m.c.AlAviso != nil {
				m.c.AlAviso(e.Chat, "solicitud", "Nueva solicitud", m.detalleAviso(def, p, e, vars))
			}
			if p.Siguiente == "" {
				_ = alm.CerrarSesion(bot, s, almacen.SesCompletada, 0, ahora)
				r.Estado = almacen.SesCompletada
				return r, nil
			}
			id = p.Siguiente
		case PasoPedir:
			_ = alm.MoverSesion(bot, s, p.ID, true, 0, false, evt(p.ID, "", "mostrado"), ahora)
			r.Salidas = append(r.Salidas, motor.Salida{Texto: m.sustituye(def, p.Pregunta, e, vars)})
			return r, nil
		case PasoForm:
			return m.iniciaForm(def, s, p, e, prueba, r), nil
		case PasoHumano:
			h := m.pasarAHumano(def, s, e, prueba, p)
			r.Salidas = append(r.Salidas, h.Salidas...)
			r.Estado, r.Humano = almacen.SesHumano, true
			return r, nil
		case PasoFin:
			r.texto(m.sustituye(def, p.Texto, e, vars))
			_ = alm.CerrarSesion(bot, s, almacen.SesCompletada, 0, ahora)
			r.Estado = almacen.SesCompletada
			return r, nil
		default:
			_ = alm.CerrarSesion(bot, s, almacen.SesAbandonada, 0, ahora)
			return r, nil
		}
	}
	// demasiados pasos encadenados sin esperar a nadie: se corta
	_ = alm.CerrarSesion(bot, s, almacen.SesAbandonada, 0, ahora)
	r.Estado = almacen.SesAbandonada
	return r, nil
}

// salidasDeRespuesta arma lo que manda un paso de respuesta: el texto (con variables), hasta 3 archivos, una ubicación y un contacto.
// El texto va de pie de foto en el primer archivo si es foto, video o documento.
func (m *Motor) salidasDeRespuesta(def *Def, p *Paso, e entrada, vars map[string]string) []motor.Salida {
	texto := strings.TrimSpace(m.sustituye(def, p.Texto, e, vars))
	medios := make([]motor.Medio, 0, len(p.Adjuntos))
	for _, a := range p.Adjuntos {
		medios = append(medios, motor.Medio{ID: a.ID, Tipo: a.Tipo, Mime: a.Mime, Nombre: a.Nombre})
	}
	out := motor.ArmarSalidas(texto, medios)
	if u := p.Ubicacion; u != nil {
		out = append(out, motor.Salida{Ubicacion: &motor.Ubicacion{Lat: u.Lat, Lon: u.Lon, Nombre: m.sustituye(def, u.Nombre, e, vars), Direccion: m.sustituye(def, u.Direccion, e, vars)}})
	}
	if c := p.Contacto; c != nil {
		out = append(out, motor.Salida{Contacto: &motor.Contacto{Nombre: strings.TrimSpace(m.sustituye(def, c.Nombre, e, vars)), Telefono: strings.TrimSpace(c.Telefono)}})
	}
	return out
}

// describeSalida es cómo se ve una salida en el simulador (que no manda nada de verdad).
func describeSalida(s motor.Salida) string {
	var partes []string
	if s.Medio != nil {
		n := s.Medio.Nombre
		if n == "" {
			n = s.Medio.Tipo
		}
		partes = append(partes, "["+strings.ToUpper(s.Medio.Tipo[:1])+s.Medio.Tipo[1:]+": "+n+"]")
	}
	if s.Ubicacion != nil {
		n := s.Ubicacion.Nombre
		if n == "" {
			n = fmt.Sprintf("%.5f, %.5f", s.Ubicacion.Lat, s.Ubicacion.Lon)
		}
		partes = append(partes, "[Ubicación: "+n+"]")
	}
	if s.Contacto != nil {
		partes = append(partes, "[Contacto: "+s.Contacto.Nombre+" · "+s.Contacto.Telefono+"]")
	}
	if s.Texto != "" {
		partes = append([]string{s.Texto}, partes...)
	}
	return strings.Join(partes, "\n")
}

var dias = []string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"}

var reVar = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

// sustituye pone {nombre}, {hora}, {dia}, {fecha}, {horario}, {prefijo} y los datos pedidos ({dato1}…) en un texto.
func (m *Motor) sustituye(def *Def, t string, e entrada, vars map[string]string) string {
	return reVar.ReplaceAllStringFunc(t, func(x string) string {
		k := strings.ToLower(x[1 : len(x)-1])
		ahora := m.c.Ahora().In(m.c.Zona())
		switch k {
		case "nombre":
			return m.c.Nombre(e.Chat, e.Sender)
		case "hora":
			return ahora.Format("15:04")
		case "dia", "día":
			return dias[int(ahora.Weekday())]
		case "fecha":
			return ahora.Format("02/01/2006")
		case "horario":
			if def != nil {
				if h, ok := m.horarioAtencion(def); ok {
					return DescribirHorario(h)
				}
			}
			return "todos los días"
		case "prefijo":
			return m.prefijo()
		}
		if v, ok := vars[k]; ok {
			return v
		}
		return ""
	})
}

func (m *Motor) detalleAviso(def *Def, p *Paso, e entrada, vars map[string]string) string {
	if strings.TrimSpace(p.Aviso) != "" {
		return m.sustituye(def, p.Aviso, e, vars)
	}
	var b strings.Builder
	b.WriteString(m.c.Nombre(e.Chat, e.Sender))
	for _, q := range def.Pasos {
		if q.Tipo == PasoPedir {
			fmt.Fprintf(&b, "\n%s", vars[q.Variable])
		}
	}
	return strings.TrimSpace(b.String())
}

var nombresDia = []string{"", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado", "domingo"}

// DescribirHorario dice un horario con palabras: "de lunes a viernes de 09:00 a 18:00".
func DescribirHorario(h Horario) string {
	var partes []string
	ds := map[int]bool{}
	for _, d := range h.Dias {
		if d >= 1 && d <= 7 {
			ds[d] = true
		}
	}
	switch n := len(ds); {
	case n == 0 || n == 7:
		partes = append(partes, "todos los días")
	default:
		// tramos seguidos: "lunes a viernes", "sábado y domingo"
		var tramos [][2]int
		for d := 1; d <= 7; d++ {
			if !ds[d] {
				continue
			}
			if k := len(tramos); k > 0 && tramos[k-1][1] == d-1 {
				tramos[k-1][1] = d
			} else {
				tramos = append(tramos, [2]int{d, d})
			}
		}
		var t []string
		for _, tr := range tramos {
			switch {
			case tr[0] == tr[1]:
				t = append(t, nombresDia[tr[0]])
			case tr[1]-tr[0] == 1:
				t = append(t, nombresDia[tr[0]]+" y "+nombresDia[tr[1]])
			default:
				t = append(t, nombresDia[tr[0]]+" a "+nombresDia[tr[1]])
			}
		}
		switch len(t) {
		case 1:
			partes = append(partes, "de "+t[0])
		default:
			partes = append(partes, strings.Join(t[:len(t)-1], ", ")+" y "+t[len(t)-1])
		}
	}
	if h.Desde != "" || h.Hasta != "" {
		d, a := h.Desde, h.Hasta
		if d == "" {
			d = "00:00"
		}
		if a == "" {
			a = "24:00"
		}
		partes = append(partes, "de "+d+" a "+a)
	}
	return strings.Join(partes, " ")
}
