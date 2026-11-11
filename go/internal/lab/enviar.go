package lab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

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
}

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
	modo := s.Modo()
	if modo == ModoTexto {
		return spec{}, false
	}
	if esGrupo(chat) && s.aj("lab_grupos") != "1" {
		return spec{}, false // solo privado por defecto: en un grupo todos ven los votos
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
		t := s.tipoBotones(n)
		if t == "" {
			return spec{}, false
		}
		sp.Tipo = t
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
	case almacen.LabBotones:
		bs := make([]*waE2E.ButtonsMessage_Button, len(sp.Opciones))
		for i := range sp.Opciones {
			sp.Opciones[i].ID = idBoton(id, sp.Opciones[i].Num)
			bs[i] = &waE2E.ButtonsMessage_Button{
				ButtonID:   proto.String(sp.Opciones[i].ID),
				ButtonText: &waE2E.ButtonsMessage_Button_ButtonText{DisplayText: proto.String(sp.Opciones[i].Texto)},
				Type:       waE2E.ButtonsMessage_Button_RESPONSE.Enum(),
			}
		}
		return &waE2E.Message{ButtonsMessage: &waE2E.ButtonsMessage{
			ContentText: proto.String(sp.Titulo), FooterText: proto.String("Toca una opción"),
			Buttons: bs, HeaderType: waE2E.ButtonsMessage_EMPTY.Enum(),
		}}
	case almacen.LabLista:
		rows := make([]*waE2E.ListMessage_Row, len(sp.Opciones))
		for i := range sp.Opciones {
			sp.Opciones[i].ID = idBoton(id, sp.Opciones[i].Num)
			rows[i] = &waE2E.ListMessage_Row{Title: proto.String(sp.Opciones[i].Texto), RowID: proto.String(sp.Opciones[i].ID)}
		}
		return &waE2E.Message{ListMessage: &waE2E.ListMessage{
			Title: proto.String("Menú"), Description: proto.String(sp.Titulo), ButtonText: proto.String("Ver opciones"),
			ListType: waE2E.ListMessage_SINGLE_SELECT.Enum(),
			Sections: []*waE2E.ListMessage_Section{{Title: proto.String("Opciones"), Rows: rows}},
		}}
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
		return Enviado{}, false, nil
	}
	if persona == "" {
		persona = chat
	}
	exp := s.ahora().Add(time.Duration(maxInt(s.ajInt("lab_expira_min"), 1)) * time.Minute)
	var sesID string
	r, err := s.Env.EnviarInteractivo(ctx, chat, resumenTexto(&sp, textoNumerado), citaID, citaAutorWA,
		func(id string, ci *waE2E.ContextInfo) *waE2E.Message {
			m := s.armar(&sp, id)
			ponerContexto(m, ci)
			return m
		},
		func(id string, m *waE2E.Message) error {
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
		})
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
