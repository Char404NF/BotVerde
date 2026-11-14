package wabot

import (
	"fmt"
	"time"

	"go.mau.fi/whatsmeow/types"
)

// aprenderLIDs: los participantes de un grupo traen a la vez su LID (identificador oculto) y su número. Con eso se sabe quién es
// quién aunque esa persona le escriba al bot por privado con el LID: así el chat siempre muestra el número y no "Número no disponible".
func (s *estado) aprenderLIDs(gi *types.GroupInfo) {
	if gi == nil {
		return
	}
	for _, p := range gi.Participants {
		lid := p.LID
		if lid.IsEmpty() && p.JID.Server == types.HiddenUserServer {
			lid = p.JID
		}
		pn := p.PhoneNumber
		if pn.IsEmpty() && p.JID.Server == types.DefaultUserServer {
			pn = p.JID
		}
		s.unirLIDconNumero(lid, pn)
	}
}

// unirLIDconNumero guarda el vínculo LID-número (en WhatsApp y en la app) y junta el chat que estuviera guardado por LID con el del número.
func (s *estado) unirLIDconNumero(lid, pn types.JID) {
	if lid.IsEmpty() || pn.IsEmpty() {
		return
	}
	lid, pn = lid.ToNonAD(), pn.ToNonAD()
	if lid.Server != types.HiddenUserServer || pn.Server != types.DefaultUserServer || s.esPropio(lid) || s.esPropio(pn) {
		return
	}
	if s.cli != nil && s.cli.Store != nil && s.cli.Store.LIDs != nil {
		_ = s.cli.Store.LIDs.PutLIDMapping(s.ctx, lid, pn)
	}
	if a, err := almacenAbierto(); err == nil {
		a.AprenderAlias(s.botID, pn.String(), lid.String())
	}
	if s.ing != nil {
		s.ing.UnirChatLID(s.ctx, lid, pn)
	}
}

// reconciliarLIDs repasa los chats que quedaron guardados por LID y, si WhatsApp ya conoce su número, los junta con el del teléfono.
// Corre una vez al conectar (después de leer los grupos), para arreglar también los chats viejos.
func (s *estado) reconciliarLIDs() {
	defer func() {
		if r := recover(); r != nil {
			s.log(fmt.Sprintf("error interno al unir chats por número: %v", r))
		}
	}()
	a, err := almacenAbierto()
	if err != nil || s.cli == nil || s.cli.Store == nil || s.cli.Store.LIDs == nil {
		return
	}
	chats, err := a.TodosLosChats(s.botID)
	if err != nil {
		return
	}
	unidos := 0
	for _, c := range chats {
		j, err := types.ParseJID(c)
		if err != nil || j.Server != types.HiddenUserServer {
			continue
		}
		if pn, err := s.cli.Store.LIDs.GetPNForLID(s.ctx, j); err == nil && !pn.IsEmpty() {
			s.unirLIDconNumero(j, pn)
			unidos++
		}
	}
	if unidos > 0 {
		s.log(fmt.Sprintf("Chats con número oculto unidos a su número: %d.", unidos))
	}
}

// reconciliarLIDsDespues espera a que termine la lectura de grupos y entonces repasa los chats por LID.
func (s *estado) reconciliarLIDsDespues() {
	time.AfterFunc(45*time.Second, func() { s.encola(s.reconciliarLIDs) })
}
