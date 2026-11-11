package grupos

import (
	"context"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// Cliente es lo que se usa de whatsmeow para los grupos (para poder probar sin red). Las firmas son las de la librería.
type Cliente interface {
	Conectado() bool
	Propio() (pn types.JID, lid types.JID)
	GetGroupInfo(ctx context.Context, jid types.JID) (*types.GroupInfo, error)
	SetGroupAnnounce(ctx context.Context, jid types.JID, announce bool) error
	SetGroupLocked(ctx context.Context, jid types.JID, locked bool) error
	SetGroupName(ctx context.Context, jid types.JID, name string) error
	SetGroupTopic(ctx context.Context, jid types.JID, previousID, newID, topic string) error
	UpdateGroupParticipants(ctx context.Context, jid types.JID, participantChanges []types.JID, action whatsmeow.ParticipantChange) ([]types.GroupParticipant, error)
	GetGroupRequestParticipants(ctx context.Context, jid types.JID) ([]types.GroupParticipantRequest, error)
	UpdateGroupRequestParticipants(ctx context.Context, jid types.JID, participantChanges []types.JID, action whatsmeow.ParticipantRequestChange) ([]types.GroupParticipant, error)
	GetGroupInviteLink(ctx context.Context, jid types.JID, reset bool) (string, error)
}

// Real adapta un cliente de whatsmeow.
type Real struct{ *whatsmeow.Client }

// Conectado dice si hay conexión y sesión.
func (r Real) Conectado() bool { return r.IsConnected() && r.IsLoggedIn() }

// Propio devuelve el teléfono y el LID de la cuenta.
func (r Real) Propio() (types.JID, types.JID) {
	if r.Store == nil {
		return types.EmptyJID, types.EmptyJID
	}
	pn := types.EmptyJID
	if r.Store.ID != nil {
		pn = r.Store.ID.ToNonAD()
	}
	return pn, r.Store.LID.ToNonAD()
}
