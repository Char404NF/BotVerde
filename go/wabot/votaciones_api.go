package wabot

import (
	"botchat/internal/ingesta"

	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// origenVoto es el remitente original que usa la librería para abrir un voto (misma regla que en whatsmeow).
func origenVoto(evt *events.Message, key *waCommon.MessageKey) (types.JID, bool) {
	if key.GetFromMe() {
		return evt.Info.Sender, true
	}
	if evt.Info.Chat.Server == types.DefaultUserServer || evt.Info.Chat.Server == types.HiddenUserServer {
		j, err := types.ParseJID(key.GetRemoteJID())
		return j, err == nil
	}
	j, err := types.ParseJID(key.GetParticipant())
	return j, err == nil
}

// alVotoModulo cuenta el voto de una encuesta que mandó el módulo Votaciones. Devuelve true si el voto era de una de sus encuestas.
// Corre en el trabajador (el manejador de eventos solo encola).
func (s *estado) alVotoModulo(v *events.Message) bool {
	if s.mods == nil || s.cli == nil || v.Info.IsFromMe || v.Message == nil {
		return false
	}
	m := ingesta.Desenvolver(v.Message)
	pu := m.GetPollUpdateMessage()
	if pu == nil {
		return false
	}
	pollID := pu.GetPollCreationMessageKey().GetID()
	secreto := s.mods.SecretoVotacion(pollID)
	if len(secreto) == 0 {
		return false // no es de una votación del módulo
	}
	ev := *v
	ev.Message = m
	voto, err := s.cli.DecryptPollVote(s.ctx, &ev)
	if err != nil {
		// Si la librería no encuentra el secreto (el voto llega por el chat con LID y la encuesta salió al chat con número), se vuelve a
		// guardar nuestra copia bajo la clave que busca y se reintenta una vez.
		orig, ok := origenVoto(&ev, pu.GetPollCreationMessageKey())
		if !ok || s.cli.Store.MsgSecrets.PutMessageSecret(s.ctx, ev.Info.Chat, orig, types.MessageID(pollID), secreto) != nil {
			s.log("No se pudo abrir un voto de una votación.")
			return true
		}
		if voto, err = s.cli.DecryptPollVote(s.ctx, &ev); err != nil {
			s.log("No se pudo abrir un voto de una votación.")
			return true
		}
	}
	chat, ok := s.ing.ChatDe(s.ctx, v.Info.MessageSource)
	if !ok {
		return true
	}
	s.aprendeAlias(v)
	persona := s.ing.Persona(s.ctx, v.Info.Sender)
	s.mods.VotoEncuesta(chat, pollID, persona, voto.GetSelectedOptions())
	s.avisaCambio("modulos")
	return true
}
