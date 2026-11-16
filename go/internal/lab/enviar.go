package lab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"

	"botchat/internal/almacen"
	"botchat/internal/envio"
	"botchat/internal/motor"
)

// Enviado es lo que pasó al intentar mandar un menú como encuesta o botones.
type Enviado struct {
	ID       string // id del mensaje que salió
	Tipo     string // encuesta, botones, lista o interactivo
	Respaldo bool   // hay que mandar también el texto numerado debajo (ajuste)
}

// spec es un menú ya decidido: de qué tipo sale y con qué opciones.
type spec struct {
	Tipo     string
	Titulo   string
	Opciones []almacen.OpcionLab // Num, Texto (tal como sale) e ID (botones y listas, se completa al armar)
	// Especiales: botones de enlace, copiar o llamar por número de opción (solo con botones; no hay respuesta que esperar).
	Especiales map[int]especial
}

// especial es un botón que abre un enlace, copia un texto o marca un número en vez de responder.
type especial struct{ Tipo, Valor string }

func (s *Servicio) tipoBotones(n int) string {
	switch t := s.aj("lab_botones_tipo"); t {
	case "botones":
		if n <= maxBotones {
			return almacen.LabBotones
		}
	case "lista":
		if n <= maxFilas {
			return almacen.LabLista
		}
	case "interactivo":
		if n <= maxBotones {
			return almacen.LabInteractivo
		}
	default: // auto
		if n <= maxBotones {
			return almacen.LabBotones
		}
		if n <= maxFilas {
			return almacen.LabLista
		}
	}
	return ""
}

// decidir elige cómo sale un menú. ok=false significa "como texto" (el menú de siempre).
func (s *Servicio) decidir(chat string, menu *motor.MenuLab) (spec, bool) {
	if menu == nil || len(menu.Opciones) == 0 {
		return spec{}, false
	}
	modo := s.ModoDe(menu.Formato)
	if modo == ModoTexto {
		return spec{}, false
	}
	if esGrupo(chat) && (modo == ModoBotones || s.aj("lab_grupos") != "1") {
		return spec{}, false // botones y listas solo en privado; la encuesta, en grupos, solo si se pidió (todos ven los votos)
	}
	n := len(menu.Opciones)
	titulo := strings.TrimSpace(menu.Titulo)
	if titulo == "" {
		titulo = "Elige una opción"
	}
	sp := spec{Titulo: titulo}
	switch modo {
	case ModoEncuesta:
		max := s.ajInt("lab_enc_max_opc")
		if max < 2 {
			max = 2
		}
		if n < 2 || n > max {
			return spec{}, false
		}
		sp.Tipo = almacen.LabEncuesta
		sp.Titulo = truncar(titulo, maxNombreEncuesta)
		nombres := make([]string, n)
		for i, o := range menu.Opciones {
			nombres[i] = truncar(o.Texto, maxOpcionEncuesta)
		}
		nombres = unicos(nombres)
		for i, o := range menu.Opciones {
			sp.Opciones = append(sp.Opciones, almacen.OpcionLab{Num: o.Num, Texto: nombres[i]})
		}
	case ModoBotones:
		especiales := map[int]especial{}
		for _, o := range menu.Opciones {
			if o.BotonTipo != "" && o.BotonValor != "" {
				especiales[o.Num] = especial{o.BotonTipo, o.BotonValor}
			}
		}
		t := s.tipoBotones(n)
		if len(especiales) > 0 {
			// una lista no puede llevar enlaces, copiar ni llamar: con botones especiales solo caben hasta 3 botones
			t = ""
			if n <= maxBotones {
				t = almacen.LabBotones
			}
		}
		if t == "" || !s.cupoBotones() {
			return spec{}, false
		}
		sp.Tipo = t
		if len(especiales) > 0 {
			sp.Especiales = especiales
		}
		lim := maxBoton
		if t == almacen.LabLista {
			lim = maxFila
		}
		nombres := make([]string, n)
		for i, o := range menu.Opciones {
			nombres[i] = textoVolver(o, lim)
		}
		nombres = unicos(nombres)
		for i, o := range menu.Opciones {
			sp.Opciones = append(sp.Opciones, almacen.OpcionLab{Num: o.Num, Texto: nombres[i]})
		}
	}
	return sp, true
}

func idBoton(msgID string, num int) string { return fmt.Sprintf("bc|%s|%d", msgID, num) }

// armar construye el mensaje de WhatsApp de un spec. Usa solo los constructores de la librería y los campos normales del mensaje.
func (s *Servicio) armar(sp *spec, id string) *waE2E.Message {
	switch sp.Tipo {
	case almacen.LabEncuesta:
		nombres := make([]string, len(sp.Opciones))
		for i, o := range sp.Opciones {
			nombres[i] = o.Texto
		}
		return s.Cli.BuildPollCreation(sp.Titulo, nombres, 1)
	case almacen.LabBotones, almacen.LabLista:
		return s.armarNativo(sp, id)
	default: // interactivo con quick_reply
		bs := make([]*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton, len(sp.Opciones))
		for i := range sp.Opciones {
			sp.Opciones[i].ID = idBoton(id, sp.Opciones[i].Num)
			pj, _ := json.Marshal(map[string]string{"display_text": sp.Opciones[i].Texto, "id": sp.Opciones[i].ID})
			bs[i] = &waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{Name: proto.String("quick_reply"), ButtonParamsJSON: proto.String(string(pj))}
		}
		return &waE2E.Message{InteractiveMessage: &waE2E.InteractiveMessage{
			Body: &waE2E.InteractiveMessage_Body{Text: proto.String(sp.Titulo)},
			InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
				NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{Buttons: bs}},
		}}
	}
}

// armarNativo arma botones (hasta 3) o una lista (hasta 10 filas) como mensaje interactivo «native flow», que es lo único que WhatsApp
// muestra a una cuenta normal cuando va con el nodo <biz> (ver NodoBiz). Un botón normal responde con su id; los especiales abren un
// enlace, copian un texto o marcan un número.
func (s *Servicio) armarNativo(sp *spec, id string) *waE2E.Message {
	var bs []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton
	boton := func(nombre string, params map[string]any) {
		pj, _ := json.Marshal(params)
		bs = append(bs, &waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{Name: proto.String(nombre), ButtonParamsJSON: proto.String(string(pj))})
	}
	if sp.Tipo == almacen.LabLista {
		filas := make([]map[string]any, len(sp.Opciones))
		for i := range sp.Opciones {
			sp.Opciones[i].ID = idBoton(id, sp.Opciones[i].Num)
			filas[i] = map[string]any{"id": sp.Opciones[i].ID, "title": sp.Opciones[i].Texto}
		}
		boton("single_select", map[string]any{"title": "Ver opciones", "sections": []any{map[string]any{"title": "Opciones", "rows": filas}}})
	} else {
		for i := range sp.Opciones {
			o := &sp.Opciones[i]
			if e, ok := sp.Especiales[o.Num]; ok {
				switch e.Tipo {
				case "enlace":
					boton("cta_url", map[string]any{"display_text": o.Texto, "url": e.Valor, "merchant_url": e.Valor})
				case "copiar":
					boton("cta_copy", map[string]any{"display_text": o.Texto, "copy_code": e.Valor})
				case "llamar":
					boton("cta_call", map[string]any{"display_text": o.Texto, "phone_number": e.Valor})
				}
				continue
			}
			o.ID = idBoton(id, o.Num)
			boton("quick_reply", map[string]any{"display_text": o.Texto, "id": o.ID})
		}
	}
	return &waE2E.Message{InteractiveMessage: &waE2E.InteractiveMessage{
		Body: &waE2E.InteractiveMessage_Body{Text: proto.String(sp.Titulo)},
		InteractiveMessage: &waE2E.InteractiveMessage_NativeFlowMessage_{
			NativeFlowMessage: &waE2E.InteractiveMessage_NativeFlowMessage{Buttons: bs}},
	}}
}

// cupoBotones dice si todavía caben menús con botones o listas en esta hora (tope «menu_botones_hora»; 0 = sin tope). Si no caben,
// el menú sale en texto. Cuenta al preguntar.
func (s *Servicio) cupoBotones() bool {
	tope := s.ajInt("menu_botones_hora")
	if tope <= 0 {
		return true
	}
	ahora := s.ahora()
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.envBotones[:0]
	for _, t := range s.envBotones {
		if ahora.Sub(t) < time.Hour {
			n = append(n, t)
		}
	}
	s.envBotones = n
	if len(s.envBotones) >= tope {
		return false
	}
	s.envBotones = append(s.envBotones, ahora)
	return true
}

// resumenTexto es lo que se guarda en el chat (la pantalla del teléfono lo muestra como un mensaje de texto).
func resumenTexto(sp *spec, textoNumerado string) string {
	if strings.TrimSpace(textoNumerado) != "" {
		return textoNumerado
	}
	var b strings.Builder
	b.WriteString(sp.Titulo)
	for _, o := range sp.Opciones {
		fmt.Fprintf(&b, "\n%d. %s", o.Num, o.Texto)
	}
	return b.String()
}

// EnviarMenu intenta mandar el menú como encuesta o botones. Devuelve ok=false (sin error) cuando el menú debe salir como texto:
// modo Texto, Laboratorio apagado, grupo sin permiso, o demasiadas/pocas opciones. Si el envío falla devuelve el error y ok=false:
// quien llama manda el texto numerado, que es el respaldo de siempre. Tras fallosParaVolver fallos seguidos el modo vuelve a texto.
// citaID/citaAutorWA citan el mensaje de la persona.
func (s *Servicio) EnviarMenu(ctx context.Context, chat, persona string, menu *motor.MenuLab, textoNumerado, citaID, citaAutorWA string) (Enviado, bool, error) {
	sp, ok := s.decidir(chat, menu)
	if !ok {
		if menu != nil && s.ModoDe(menu.Formato) != ModoTexto {
			s.Log(fmt.Sprintf("Menú en texto aunque el formato es «%s» (%d opciones, grupo: %v).", s.ModoDe(menu.Formato), len(menu.Opciones), esGrupo(chat)))
		}
		return Enviado{}, false, nil
	}
	if persona == "" {
		persona = chat
	}
	exp := s.ahora().Add(time.Duration(maxInt(s.ajInt("lab_expira_min"), 1)) * time.Minute)
	var sesID string
	armar := func(id string, ci *waE2E.ContextInfo) *waE2E.Message {
		m := s.armar(&sp, id)
		ponerContexto(m, ci)
		return m
	}
	antes := func(id string, m *waE2E.Message) error {
		ses := almacen.SesionLab{ID: id, Chat: chat, Persona: persona, Tipo: sp.Tipo, Origen: almacen.LabOrigenMenu,
			TS: s.ahora().Unix(), ExpiraTS: exp.Unix(), Opciones: sp.Opciones}
		if sp.Tipo == almacen.LabEncuesta {
			ses.Secreto = m.GetMessageContextInfo().GetMessageSecret()
		}
		if err := s.Alm.GuardarSesionLab(s.BotID, ses); err != nil {
			return err
		}
		sesID = id
		return nil
	}
	var r envio.Resultado
	var err error
	if en, ok := s.Env.(EmisorNodos); ok && (sp.Tipo == almacen.LabBotones || sp.Tipo == almacen.LabLista) {
		r, err = en.EnviarInteractivoNodos(ctx, chat, resumenTexto(&sp, textoNumerado), citaID, citaAutorWA, armar, antes, []waBinary.Node{NodoBiz(s.ahora(), nil)})
	} else {
		r, err = s.Env.EnviarInteractivo(ctx, chat, resumenTexto(&sp, textoNumerado), citaID, citaAutorWA, armar, antes)
	}
	if err != nil {
		if sesID != "" {
			s.Alm.BorrarSesionLab(s.BotID, sesID)
		}
		if !errors.Is(err, envio.ErrSinConexion) && !errors.Is(err, envio.ErrChatDesconoci) {
			s.falla(err)
		}
		return Enviado{}, false, err
	}
	s.mu.Lock()
	s.fallos = 0
	s.mu.Unlock()
	resp := (sp.Tipo == almacen.LabEncuesta && s.aj("lab_respaldo_enc") == "1") || (sp.Tipo != almacen.LabEncuesta && s.aj("lab_respaldo_bot") == "1")
	return Enviado{ID: r.ID, Tipo: sp.Tipo, Respaldo: resp}, true, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// falla cuenta un envío interactivo fallido; tras varios seguidos el menú vuelve a texto por sí solo.
func (s *Servicio) falla(err error) {
	s.mu.Lock()
	s.fallos++
	n := s.fallos
	s.mu.Unlock()
	s.Log("No se pudo mandar el menú como " + s.Modo() + "; sale en texto (" + err.Error() + ").")
	if n >= fallosParaVolver {
		s.mu.Lock()
		s.fallos = 0
		s.mu.Unlock()
		_ = s.Alm.FijarAjusteBot(s.BotID, "lab_modo_menu", ModoTexto)
		s.Log("Fallaron varios envíos seguidos: el menú volvió a texto numerado.")
		if s.AlVolverATexto != nil {
			s.AlVolverATexto("fallos_seguidos")
		}
	}
}

// BotonCmd es un botón de un módulo: al tocarlo, el bot recibe `Comando` como si esa persona lo hubiera escrito.
type BotonCmd struct{ Texto, Comando string }

// EnviarBotones manda un mensaje con hasta 3 botones que ejecutan comandos (por ejemplo «Reclamar premio» → «!reclamar»). Solo sale con
// botones si el menú general está en «Botones», el chat es privado y hay cupo; si no, devuelve ok=false y quien llama manda su texto de
// siempre. Si el envío falla, devuelve el error (y también conviene mandar el texto).
func (s *Servicio) EnviarBotones(ctx context.Context, chat, persona, texto string, botones []BotonCmd) (bool, error) {
	if s.Modo() != ModoBotones || esGrupo(chat) || len(botones) == 0 || len(botones) > maxBotones || strings.TrimSpace(texto) == "" || !s.cupoBotones() {
		return false, nil
	}
	if persona == "" {
		persona = chat
	}
	sp := spec{Tipo: almacen.LabBotones, Titulo: truncar(texto, 1000)}
	for i, b := range botones {
		sp.Opciones = append(sp.Opciones, almacen.OpcionLab{Num: i + 1, Texto: truncar(b.Texto, maxBoton), Cmd: strings.TrimSpace(b.Comando)})
	}
	exp := s.ahora().Add(24 * time.Hour)
	en, ok := s.Env.(EmisorNodos)
	if !ok {
		return false, nil
	}
	_, err := en.EnviarInteractivoNodos(ctx, chat, texto, "", "",
		func(id string, ci *waE2E.ContextInfo) *waE2E.Message {
			m := s.armar(&sp, id)
			ponerContexto(m, ci)
			return m
		},
		func(id string, m *waE2E.Message) error {
			return s.Alm.GuardarSesionLab(s.BotID, almacen.SesionLab{ID: id, Chat: chat, Persona: persona, Tipo: sp.Tipo, Origen: almacen.LabOrigenMenu,
				TS: s.ahora().Unix(), ExpiraTS: exp.Unix(), Opciones: sp.Opciones})
		}, []waBinary.Node{NodoBiz(s.ahora(), nil)})
	if err != nil {
		return false, err
	}
	return true, nil
}
