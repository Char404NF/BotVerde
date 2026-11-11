package flujo

import (
	"strings"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/motor"
)

// Menús en grupos: la conversación es de (grupo, persona canónica); se arranca con mención, prefijo de comando o citando un mensaje
// del bot; un mensaje sin citar solo cuenta si esa persona ya tiene conversación abierta y es una opción o un comando válido; hay un
// tope de personas a la vez con fila de espera.

// validaSinDirigir: el mensaje no menciona ni cita al bot. Solo vale si es una opción o un comando de lo que esa persona tiene abierto.
// Un formulario o una pregunta libre en un grupo exigen mención o cita (si no, cualquier charla se tomaría como respuesta).
func (m *Motor) validaSinDirigir(def *Def, s *almacen.Sesion, e entrada) bool {
	if e.NoTexto {
		return false
	}
	texto := strings.TrimSpace(e.Texto)
	if texto == "" {
		return false
	}
	p := def.Paso(s.Paso)
	if p == nil {
		return false
	}
	if cmd := m.comandoSuelto(texto); cmd != "" {
		return true
	}
	if p.Tipo != PasoMenu {
		return false
	}
	vis := m.visibles(def, s.MenuID, p, s.Sender, true)
	if n, ok := numeroDeOpcion(texto); ok && n == 0 && p.ID != def.Inicio {
		return true
	}
	if _, ok := eligeExacta(vis, texto); ok {
		return true
	}
	_, _, ok := eligeParcial(vis, texto)
	return ok
}

// AdmiteEnGrupo implementa motor.Admisor: ¿le interesa al menú este mensaje de grupo aunque no mencione ni cite al bot?
func (m *Motor) AdmiteEnGrupo(msg motor.Mensaje) bool {
	if !msg.EsGrupo || !msg.EsTexto {
		return false
	}
	sender := m.persona(msg.Remitente)
	s, err := m.c.Alm.SesionVigente(m.c.BotID, msg.Chat, sender, false, m.c.Ahora(), m.expiraDefecto(true))
	if err != nil || s == nil || s.Estado != almacen.SesActiva {
		return false
	}
	def := m.definicion(s.MenuID, s.MenuVersion)
	if def == nil {
		return false
	}
	return m.validaSinDirigir(def, s, entrada{Chat: msg.Chat, Sender: sender, Texto: msg.Texto, EsGrupo: true})
}

// EsperaTexto implementa motor.Admisor: llegó algo que no es texto (foto, audio, sticker, archivo) y hay una conversación de menú
// abierta que espera texto. En un grupo, solo si le hablan al bot (mención o cita).
func (m *Motor) EsperaTexto(msg motor.Mensaje) bool {
	if msg.EsTexto {
		return false
	}
	sender := msg.Chat
	if msg.EsGrupo {
		if !(msg.Mencionado || msg.CitaEsMia || m.citaUnMenu(msg)) {
			return false
		}
		sender = m.persona(msg.Remitente)
		if ref := m.c.Alm.MsgBotPorID(m.c.BotID, msg.Chat, msg.CitaID); ref != nil && ref.Persona != sender {
			if !m.cualquieraPuede(m.definicionDeMenu(ref.MenuID)) {
				return false
			}
			sender = ref.Persona
		}
	}
	s, err := m.c.Alm.SesionVigente(m.c.BotID, msg.Chat, sender, false, m.c.Ahora(), m.expiraDefecto(msg.EsGrupo))
	return err == nil && s != nil && s.Estado == almacen.SesActiva
}

func (m *Motor) citaUnMenu(msg motor.Mensaje) bool {
	return msg.CitaID != "" && m.c.Alm.MsgBotPorID(m.c.BotID, msg.Chat, msg.CitaID) != nil
}

// Diferidas implementa motor.ConDiferidas: cierra en silencio las conversaciones que caducaron y, por cada lugar que se liberó en un
// grupo lleno, abre el menú de la primera persona de la fila. Es una respuesta diferida a un mensaje que esa persona ya había
// escrito (se manda citándolo y mencionándola); el bot no escribe a nadie que no le haya hablado.
func (m *Motor) Diferidas(ahora time.Time) []motor.Diferida {
	bot, alm := m.c.BotID, m.c.Alm
	alm.ExpirarSesiones(bot, ahora, m.expiraDefecto(true))
	vida := time.Duration(m.ajusteInt("grupo_cola_min")) * time.Minute
	if vida <= 0 {
		vida = 10 * time.Minute
	}
	alm.LimpiarCola(bot, vida, ahora)
	var out []motor.Diferida
	for _, chat := range alm.ChatsConCola(bot) {
		for {
			sig := alm.SiguienteEnCola(bot, chat)
			if sig == nil {
				break
			}
			mn, err := alm.ObtenerMenu(bot, sig.MenuID)
			var def *Def
			if err == nil && mn != nil && mn.Activo {
				def, _ = Parse(mn.Def)
			}
			if def == nil || def.Paso(def.Inicio) == nil {
				alm.QuitarDeCola(bot, sig.ID)
				continue
			}
			if alm.ContarSesionesGrupo(bot, chat, ahora, m.expiraDefecto(true)) >= m.maxSesiones(def) {
				break // sigue lleno: se espera al próximo ciclo
			}
			alm.QuitarDeCola(bot, sig.ID)
			e := entrada{Chat: chat, Sender: sig.Persona, EsGrupo: true, Dirigido: true, CitaID: "", MsgID: sig.MsgID, WA: sig.MencionWA, TS: time.Unix(sig.TS, 0), PrimerTS: time.Unix(sig.TS, 0)}
			id, err := alm.NuevaSesionConExpira(bot, chat, sig.Persona, mn.ID, mn.Version, def.Inicio, false, ahora, m.expira(def, true))
			if err != nil {
				continue
			}
			alm.LimpiarVencido(bot, chat, sig.Persona, mn.ID)
			ns := &almacen.Sesion{ID: id, Chat: chat, Sender: sig.Persona, MenuID: mn.ID, MenuVersion: mn.Version, Paso: def.Inicio, InicioTS: ahora.Unix(), UltimaTS: ahora.Unix(),
				Estado: almacen.SesActiva, ExpiraSeg: int64(m.expira(def, true).Seconds())}
			r, err := m.ejecuta(def, ns, def.Inicio, e, false)
			if err != nil || len(r.Salidas) == 0 {
				continue
			}
			r.Coincidio = def.Nombre
			out = append(out, motor.Diferida{Chat: chat, CitaID: sig.MsgID, CitaAutorWA: sig.MencionWA, MencionWA: sig.MencionWA, TS: time.Unix(sig.TS, 0), D: r.decision()})
		}
	}
	return out
}
