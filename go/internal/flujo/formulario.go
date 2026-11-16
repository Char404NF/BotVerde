package flujo

import (
	"fmt"
	"strings"

	"botchat/internal/almacen"
	"botchat/internal/motor"
)

// Formularios paso a paso: una pregunta a la vez, validación con reintentos, "cancelar" en cualquier momento, resumen final
// "¿Es correcto? 1 Sí 2 Corregir" y guardado cifrado. El estado vive en la conversación: fase (p preguntando, r resumen, e eligiendo
// qué corregir, c corrigiendo), campo y reintentos; las respuestas, en las variables de la conversación (cifradas).

const (
	fasePregunta = "p"
	faseResumen  = "r"
	faseElige    = "e"
	faseCorrige  = "c"
)

func (m *Motor) maxReint(p *Paso) int {
	if p.MaxReintentos > 0 {
		return p.MaxReintentos
	}
	return 3
}

func (m *Motor) region() string { return m.ajuste("menu_region") }

// pantallaForm es lo que se le dice ahora a la persona según la fase del formulario.
func (m *Motor) pantallaForm(def *Def, s *almacen.Sesion, p *Paso) string {
	switch s.Fase {
	case faseResumen:
		return m.resumenForm(def, s, p)
	case faseElige:
		return m.eligeCorregir(s, p)
	}
	idx := s.Campo
	if idx < 0 || idx >= len(p.Campos) {
		idx = 0
	}
	return m.preguntaCampo(def, s, p, idx)
}

func (m *Motor) preguntaCampo(def *Def, s *almacen.Sesion, p *Paso, idx int) string {
	c := p.Campos[idx]
	var b strings.Builder
	if len(p.Campos) > 1 {
		fmt.Fprintf(&b, "(%d de %d) ", idx+1, len(p.Campos))
	}
	b.WriteString(m.sustituye(def, c.Pregunta, entrada{Chat: s.Chat, Sender: s.Sender}, m.c.Alm.Variables(s.ID)))
	if c.Tipo == CampoOpcion {
		n := 0
		for _, o := range c.Opciones {
			if strings.TrimSpace(o) != "" {
				n++
				fmt.Fprintf(&b, "\n%d. %s", n, strings.TrimSpace(o))
			}
		}
	}
	if c.Opcional {
		b.WriteString("\n(Si no quieres contestar, escribe *omitir*.)")
	}
	return b.String()
}

// iniciaForm abre el formulario: la introducción y la primera pregunta.
func (m *Motor) iniciaForm(def *Def, s *almacen.Sesion, p *Paso, e entrada, prueba bool, r resultado) resultado {
	ahora := m.c.Ahora()
	alm := m.c.Alm
	if len(p.Campos) == 0 {
		_ = alm.CerrarSesion(m.c.BotID, s, almacen.SesCompletada, 0, ahora)
		r.Estado = almacen.SesCompletada
		return r
	}
	_ = alm.MoverSesion(m.c.BotID, s, p.ID, true, 0, false, evt(p.ID, "", "mostrado"), ahora)
	_ = alm.FijarFase(s.ID, fasePregunta, 0, 0)
	s.Fase, s.Campo, s.Reint = fasePregunta, 0, 0
	var b strings.Builder
	if intro := strings.TrimSpace(m.sustituye(def, p.Titulo, e, alm.Variables(s.ID))); intro != "" {
		b.WriteString(intro + "\n\n")
	}
	b.WriteString(m.preguntaCampo(def, s, p, 0))
	b.WriteString("\n\n(Para salir escribe *cancelar*.)")
	r.Salidas = append(r.Salidas, motor.Salida{Texto: b.String()})
	r.Paso = p.ID
	return r
}

// descartaForm borra lo contestado de un formulario que no se terminó (cancelar, salir, menú).
func (m *Motor) descartaForm(s *almacen.Sesion, p *Paso) {
	if p == nil || p.Tipo != PasoForm {
		return
	}
	for _, c := range p.Campos {
		m.c.Alm.BorrarVariable(s.ID, c.ID)
	}
	_ = m.c.Alm.FijarFase(s.ID, "", 0, 0)
	s.Fase, s.Campo, s.Reint = "", 0, 0
}

var palabrasOmitir = map[string]bool{"omitir": true, "saltar": true, "salto": true, "paso": true, "no": true, "-": true, "ninguno": true, "ninguna": true}

func (m *Motor) continuaForm(def *Def, s *almacen.Sesion, p *Paso, e entrada, prueba bool) (resultado, error) {
	ahora := m.c.Ahora()
	bot, alm := m.c.BotID, m.c.Alm
	texto := strings.TrimSpace(e.Texto)
	fase := s.Fase
	if fase == "" {
		fase = fasePregunta
	}
	toca := func(res, opc string) {
		_ = alm.MoverSesion(bot, s, p.ID, true, 0, false, evt(p.ID, opc, res), ahora)
	}
	sale := func(t string) resultado {
		return resultado{Salidas: []motor.Salida{{Texto: t}}, Estado: almacen.SesActiva, Paso: p.ID}
	}
	cmd := m.comandoSuelto(texto)
	switch cmd {
	case CmdCancelar:
		return m.cancelaForm(def, s, p, e, prueba, "")
	case CmdRepetir:
		toca("comando", "cmd:repetir")
		return sale(m.pantallaForm(def, s, p)), nil
	case CmdAtras:
		switch {
		case fase == fasePregunta && s.Campo > 0:
			toca("comando", "cmd:atras")
			_ = alm.FijarFase(s.ID, fasePregunta, s.Campo-1, 0)
			s.Fase, s.Campo, s.Reint = fasePregunta, s.Campo-1, 0
			return sale(m.preguntaCampo(def, s, p, s.Campo)), nil
		case fase == faseElige || fase == faseCorrige:
			toca("comando", "cmd:atras")
			_ = alm.FijarFase(s.ID, faseResumen, 0, 0)
			s.Fase, s.Campo, s.Reint = faseResumen, 0, 0
			return sale(m.resumenForm(def, s, p)), nil
		case fase == faseResumen:
			toca("comando", "cmd:atras")
			last := len(p.Campos) - 1
			_ = alm.FijarFase(s.ID, fasePregunta, last, 0)
			s.Fase, s.Campo, s.Reint = fasePregunta, last, 0
			return sale(m.preguntaCampo(def, s, p, last)), nil
		}
		return m.aplicaComando(cmd, def, s, p, e, prueba) // en la primera pregunta: vuelve al menú de antes
	case CmdMenu, CmdInicio, CmdSalir, CmdPersona:
		return m.aplicaComando(cmd, def, s, p, e, prueba)
	}
	switch fase {
	case faseResumen:
		return m.respuestaResumen(def, s, p, e, prueba, texto)
	case faseElige:
		return m.respuestaElige(def, s, p, texto)
	}
	// preguntando o corrigiendo un campo
	idx := s.Campo
	if idx < 0 || idx >= len(p.Campos) {
		idx = 0
	}
	c := p.Campos[idx]
	var valor string
	if c.Opcional && palabrasOmitir[norm(texto)] {
		valor = ""
	} else {
		v, msg, ok := ValidarCampo(c, texto, m.region(), ahora.In(m.c.Zona()))
		if !ok {
			reint := s.Reint + 1
			_ = alm.MoverSesion(bot, s, p.ID, true, 0, false, evt(p.ID, "campo:"+c.ID, "invalida"), ahora)
			if reint >= m.maxReint(p) {
				return m.cancelaForm(def, s, p, e, prueba, "No pude entender ese dato después de varios intentos, así que cancelé el formulario.")
			}
			_ = alm.FijarFase(s.ID, fase, idx, reint)
			s.Fase, s.Campo, s.Reint = fase, idx, reint
			return sale(msg + "\n\n" + m.preguntaCampo(def, s, p, idx)), nil
		}
		valor = v
	}
	_ = alm.GuardarVariable(s.ID, c.ID, valor)
	toca("campo", "campo:"+c.ID)
	if fase == faseCorrige {
		_ = alm.FijarFase(s.ID, faseResumen, 0, 0)
		s.Fase, s.Campo, s.Reint = faseResumen, 0, 0
		return sale(m.resumenForm(def, s, p)), nil
	}
	if idx+1 < len(p.Campos) {
		_ = alm.FijarFase(s.ID, fasePregunta, idx+1, 0)
		s.Fase, s.Campo, s.Reint = fasePregunta, idx+1, 0
		return sale(m.preguntaCampo(def, s, p, idx+1)), nil
	}
	if p.SinResumen {
		return m.confirmaForm(def, s, p, e, prueba)
	}
	_ = alm.FijarFase(s.ID, faseResumen, 0, 0)
	s.Fase, s.Campo, s.Reint = faseResumen, 0, 0
	return sale(m.resumenForm(def, s, p)), nil
}

func (m *Motor) resumenForm(def *Def, s *almacen.Sesion, p *Paso) string {
	vars := m.c.Alm.Variables(s.ID)
	var b strings.Builder
	b.WriteString("Revisa tus datos:")
	for _, c := range p.Campos {
		v := strings.TrimSpace(vars[c.ID])
		if v == "" {
			v = "(sin contestar)"
		}
		fmt.Fprintf(&b, "\n• %s: %s", c.etiqueta(), v)
	}
	b.WriteString("\n\n¿Es correcto?\n1. Sí\n2. Corregir")
	return b.String()
}

func (m *Motor) eligeCorregir(s *almacen.Sesion, p *Paso) string {
	vars := m.c.Alm.Variables(s.ID)
	var b strings.Builder
	b.WriteString("¿Cuál quieres corregir? Escribe el número:")
	for i, c := range p.Campos {
		v := strings.TrimSpace(vars[c.ID])
		if v == "" {
			v = "(sin contestar)"
		}
		fmt.Fprintf(&b, "\n%d. %s: %s", i+1, c.etiqueta(), v)
	}
	return b.String()
}

func (m *Motor) respuestaResumen(def *Def, s *almacen.Sesion, p *Paso, e entrada, prueba bool, texto string) (resultado, error) {
	ahora := m.c.Ahora()
	bot, alm := m.c.BotID, m.c.Alm
	t := norm(texto)
	sale := func(t string) resultado {
		return resultado{Salidas: []motor.Salida{{Texto: t}}, Estado: almacen.SesActiva, Paso: p.ID}
	}
	switch {
	case t == "1" || t == "si" || t == "correcto" || t == "es correcto" || t == "confirmar" || t == "confirmo" || t == "ok" || t == "listo" || t == "enviar":
		return m.confirmaForm(def, s, p, e, prueba)
	case t == "2" || t == "corregir" || t == "no" || t == "cambiar" || t == "editar":
		_ = alm.MoverSesion(bot, s, p.ID, true, 0, false, evt(p.ID, "corregir", "comando"), ahora)
		if len(p.Campos) == 1 {
			_ = alm.FijarFase(s.ID, faseCorrige, 0, 0)
			s.Fase, s.Campo, s.Reint = faseCorrige, 0, 0
			return sale(m.preguntaCampo(def, s, p, 0)), nil
		}
		_ = alm.FijarFase(s.ID, faseElige, 0, 0)
		s.Fase, s.Campo, s.Reint = faseElige, 0, 0
		return sale(m.eligeCorregir(s, p)), nil
	}
	reint := s.Reint + 1
	_ = alm.MoverSesion(bot, s, p.ID, true, 0, false, evt(p.ID, "resumen", "invalida"), ahora)
	if reint >= m.maxReint(p) {
		return m.cancelaForm(def, s, p, e, prueba, "No me quedó claro si los datos están bien, así que cancelé el formulario.")
	}
	_ = alm.FijarFase(s.ID, faseResumen, 0, reint)
	s.Reint = reint
	return sale("Responde 1 si está bien o 2 si quieres corregir algo.\n\n" + m.resumenForm(def, s, p)), nil
}

func (m *Motor) respuestaElige(def *Def, s *almacen.Sesion, p *Paso, texto string) (resultado, error) {
	alm := m.c.Alm
	vis := make([]visible, len(p.Campos))
	for i, c := range p.Campos {
		vis[i] = visible{Idx: i, Num: i + 1, O: Opcion{Texto: c.etiqueta()}}
	}
	v, _, ok := elige(vis, texto)
	if !ok {
		reint := s.Reint + 1
		_ = alm.MoverSesion(m.c.BotID, s, p.ID, true, 0, false, evt(p.ID, "corregir", "invalida"), m.c.Ahora())
		if reint >= m.maxReint(p) {
			return m.cancelaForm(def, s, p, entrada{Chat: s.Chat, Sender: s.Sender, Dirigido: true}, false, "No me quedó claro qué dato corregir, así que cancelé el formulario.")
		}
		_ = alm.FijarFase(s.ID, faseElige, 0, reint)
		s.Reint = reint
		return resultado{Salidas: []motor.Salida{{Texto: "No entendí cuál. Escribe el número del dato.\n\n" + m.eligeCorregir(s, p)}}, Estado: almacen.SesActiva, Paso: p.ID}, nil
	}
	_ = alm.MoverSesion(m.c.BotID, s, p.ID, true, 0, false, evt(p.ID, "corregir:"+p.Campos[v.Idx].ID, "comando"), m.c.Ahora())
	_ = alm.FijarFase(s.ID, faseCorrige, v.Idx, 0)
	s.Fase, s.Campo, s.Reint = faseCorrige, v.Idx, 0
	return resultado{Salidas: []motor.Salida{{Texto: m.preguntaCampo(def, s, p, v.Idx)}}, Estado: almacen.SesActiva, Paso: p.ID}, nil
}

// confirmaForm: la persona dijo que está bien. Se guarda cifrado, se avisa con un título neutro y se sigue al paso que toque.
func (m *Motor) confirmaForm(def *Def, s *almacen.Sesion, p *Paso, e entrada, prueba bool) (resultado, error) {
	ahora := m.c.Ahora()
	bot, alm := m.c.BotID, m.c.Alm
	vars := alm.Variables(s.ID)
	f := almacen.Formulario{MenuID: s.MenuID, Paso: p.ID, Titulo: def.Nombre, SesionID: s.ID, Chat: s.Chat, Persona: s.Sender, TS: ahora.Unix(),
		Nombre: m.c.Nombre(s.Chat, s.Sender), Campos: []almacen.CampoValor{}}
	var det strings.Builder
	det.WriteString(def.Nombre)
	if f.Nombre != "" {
		det.WriteString(" · " + f.Nombre)
	}
	for _, c := range p.Campos {
		v := strings.TrimSpace(vars[c.ID])
		f.Campos = append(f.Campos, almacen.CampoValor{ID: c.ID, Etiqueta: c.etiqueta(), Valor: v})
		if v != "" {
			det.WriteString("\n" + c.etiqueta() + ": " + v)
		}
	}
	if !prueba {
		if _, err := alm.GuardarFormulario(bot, f); err != nil {
			return resultado{}, err
		}
		if m.c.AlAviso != nil {
			m.c.AlAviso(s.Chat, "formulario", "Nuevo formulario recibido", det.String())
		}
	}
	_ = alm.FijarFase(s.ID, "", 0, 0)
	s.Fase, s.Campo, s.Reint = "", 0, 0
	_ = alm.MoverSesion(bot, s, p.ID, false, 0, false, evt(p.ID, "enviado", "valida"), ahora)
	r := resultado{Estado: almacen.SesActiva, Paso: p.ID, Sesion: refDe(s)}
	final := strings.TrimSpace(m.sustituye(def, p.Texto, e, vars))
	if final == "" {
		final = "¡Listo! Recibimos tus datos."
	}
	r.texto(final)
	if p.Siguiente == "" || def.Paso(p.Siguiente) == nil {
		_ = alm.CerrarSesion(bot, s, almacen.SesCompletada, 0, ahora)
		r.Estado = almacen.SesCompletada
		return r, nil
	}
	sig, err := m.ejecuta(def, s, p.Siguiente, e, prueba)
	r.Salidas = append(r.Salidas, sig.Salidas...)
	r.Estado, r.Paso, r.Humano = sig.Estado, sig.Paso, sig.Humano
	return r, err
}

// cancelaForm descarta lo contestado y vuelve al menú principal (o cierra si el formulario era lo único que había).
func (m *Motor) cancelaForm(def *Def, s *almacen.Sesion, p *Paso, e entrada, prueba bool, motivo string) (resultado, error) {
	ahora := m.c.Ahora()
	bot, alm := m.c.BotID, m.c.Alm
	m.descartaForm(s, p)
	_ = alm.MoverSesion(bot, s, p.ID, false, 0, false, evt(p.ID, "cancelado", "valida"), ahora)
	t := "Formulario cancelado."
	if motivo != "" {
		t = motivo
	}
	if ini := def.Paso(def.Inicio); ini != nil && ini.Tipo == PasoMenu {
		m.setPila(s, nil)
		r, err := m.ejecuta(def, s, def.Inicio, e, prueba)
		r.Salidas = append([]motor.Salida{{Texto: t}}, r.Salidas...)
		return r, err
	}
	_ = alm.CerrarSesion(bot, s, almacen.SesAbandonada, 0, ahora)
	alm.FijarCierre(s.ID, "cancelo")
	r := resultado{Estado: almacen.SesAbandonada, Paso: p.ID, Sesion: refDe(s)}
	if motivo == "" {
		r.texto(m.ajuste("menu_txt_cancelar"))
	} else {
		r.texto(motivo + " Cuando quieras, escribe *!menu*.")
	}
	return r, nil
}
