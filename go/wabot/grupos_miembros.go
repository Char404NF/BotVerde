package wabot

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"botchat/internal/almacen"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// Miembros de grupo y sus cambios (v0.6.0, E3). El manejador de eventos SOLO copia datos y encola; la resolución de personas, la
// escritura en la base y la consulta de red (GetJoinedGroups) corren en el trabajador o en una goroutine aparte.

const (
	snapshotCada     = 24 * time.Hour
	snapshotPausa    = 150 * time.Millisecond
	snapshotPorTanda = 20
)

var snapshotCorriendo atomic.Bool

// cambioGrupoEv es la copia de lo que se necesita de events.GroupInfo (para no usar el evento fuera del manejador).
type cambioGrupoEv struct {
	grupo    string
	sender   types.JID
	senderPN types.JID
	ts       time.Time
	razon    string
	join     []types.JID
	leave    []types.JID
	promote  []types.JID
	demote   []types.JID
}

// alCambiarMiembros: llamado desde el manejador de eventos. Solo copia y encola.
func (s *estado) alCambiarMiembros(v *events.GroupInfo) {
	if len(v.Join)+len(v.Leave)+len(v.Promote)+len(v.Demote) == 0 {
		return
	}
	c := cambioGrupoEv{grupo: v.JID.ToNonAD().String(), ts: v.Timestamp, razon: v.JoinReason,
		join: append([]types.JID(nil), v.Join...), leave: append([]types.JID(nil), v.Leave...),
		promote: append([]types.JID(nil), v.Promote...), demote: append([]types.JID(nil), v.Demote...)}
	if v.Sender != nil {
		c.sender = v.Sender.ToNonAD()
	}
	if v.SenderPN != nil {
		c.senderPN = v.SenderPN.ToNonAD()
	}
	if c.ts.IsZero() {
		c.ts = time.Now()
	}
	s.encola(func() { s.registrarCambiosGrupo(c) })
}

// registrarCambiosGrupo guarda cada entrada, salida, expulsión y ascenso. "Salió" y "lo sacaron" se distinguen por quién lo hizo
// (WhatsApp manda lo mismo en ambos casos): si lo hizo otra persona, fue expulsión; si fue la misma persona o no viene quién, salió sola.
func (s *estado) registrarCambiosGrupo(c cambioGrupoEv) {
	defer func() {
		if r := recover(); r != nil {
			s.log(fmt.Sprintf("error interno al registrar un cambio de grupo: %v", r))
		}
	}()
	a, err := almacenAbierto()
	if err != nil {
		return
	}
	por := ""
	if !c.sender.IsEmpty() {
		por = s.personaDe(c.sender, c.senderPN)
	}
	propio := func(j types.JID) bool { return s.esPropio(j) }
	nombrePropio := func(j types.JID) (string, string) {
		if propio(j) {
			return almacen.PersonaDueno, ""
		}
		return s.personaDe(j, types.EmptyJID), j.ToNonAD().String()
	}
	for _, j := range c.join {
		p, jid := nombrePropio(j)
		tipo := almacen.CambioEntra
		switch {
		case c.razon == "invite":
			tipo = almacen.CambioEnlace
		case por != "" && por != p:
			tipo = almacen.CambioAgregado
		}
		_ = a.RegistrarCambio(s.botID, c.grupo, tipo, p, jid, porSiNoEs(por, tipo), c.razon, c.ts)
		// Baneada: si vuelve a entrar, se la saca (la acción corre aparte; la red no va en el trabajador).
		if s.gru != nil && !propio(j) && a.EstaBaneado(s.botID, c.grupo, p) {
			grupo, quien := c.grupo, p
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				ctx, cancel := context.WithTimeout(s.ctx, 45*time.Second)
				defer cancel()
				s.gru.AlEntrar(ctx, grupo, quien)
			}()
		}
	}
	for _, j := range c.leave {
		p, jid := nombrePropio(j)
		tipo := almacen.CambioSale
		if por != "" && por != p {
			tipo = almacen.CambioExpulsado
		}
		_ = a.RegistrarCambio(s.botID, c.grupo, tipo, p, jid, porSiNoEs(por, tipo), "", c.ts)
		if propio(j) { // el bot ya no está en el grupo
			a.CerrarGrupo(s.botID, c.grupo, c.ts)
		}
	}
	for _, j := range c.promote {
		p, jid := nombrePropio(j)
		_ = a.RegistrarCambio(s.botID, c.grupo, almacen.CambioPromovido, p, jid, por, "", c.ts)
	}
	for _, j := range c.demote {
		p, jid := nombrePropio(j)
		_ = a.RegistrarCambio(s.botID, c.grupo, almacen.CambioDegradado, p, jid, por, "", c.ts)
	}
}

// porSiNoEs: quien hizo el cambio solo se guarda cuando es otra persona (en "entra" y "sale" no hay nadie más).
func porSiNoEs(por, tipo string) string {
	switch tipo {
	case almacen.CambioEntra, almacen.CambioSale:
		return ""
	}
	return por
}

// personaDe da la clave canónica de una persona a partir de su JID (y, si se conoce, su número).
func (s *estado) personaDe(j, pn types.JID) string {
	if s.esPropio(j) {
		return almacen.PersonaDueno
	}
	p := s.ing.Persona(s.ctx, j)
	if !pn.IsEmpty() {
		if c := s.ing.Persona(s.ctx, pn); c != "" && !esLIDStr(c) {
			p = c
		}
	}
	return p
}

func esLIDStr(j string) bool { return len(j) > 4 && j[len(j)-4:] == "@lid" }

// miembrosDe convierte los participantes de WhatsApp en filas de grupo_miembros.
func (s *estado) miembrosDe(gi *types.GroupInfo) []almacen.MiembroGrupo {
	out := make([]almacen.MiembroGrupo, 0, len(gi.Participants))
	for _, p := range gi.Participants {
		rol := almacen.RolGrupoMiembro
		if p.IsSuperAdmin {
			rol = almacen.RolGrupoCreador
		} else if p.IsAdmin {
			rol = almacen.RolGrupoAdmin
		}
		pers := s.personaDe(p.JID.ToNonAD(), p.PhoneNumber.ToNonAD())
		out = append(out, almacen.MiembroGrupo{Persona: pers, JID: p.JID.ToNonAD().String(), Rol: rol})
	}
	return out
}

// guardarMiembros guarda la lista de miembros de un grupo.
func (s *estado) guardarMiembros(gi *types.GroupInfo) {
	a, err := almacenAbierto()
	if err != nil || gi == nil {
		return
	}
	_, _ = a.ActualizarMiembros(s.botID, gi.JID.ToNonAD().String(), s.miembrosDe(gi), time.Now())
	s.aprenderLIDs(gi)
}

// alUnirseAGrupo: el bot entró (o lo agregaron) a un grupo: se guardan sus miembros. Encola; no usa la red.
func (s *estado) alUnirseAGrupo(v *events.JoinedGroup) {
	gi := v.GroupInfo
	s.encola(func() {
		defer func() {
			if r := recover(); r != nil {
				s.log(fmt.Sprintf("error interno al guardar miembros: %v", r))
			}
		}()
		s.guardarMiembros(&gi)
	})
}

// snapshotSiToca hace la foto completa de miembros de todos los grupos si nunca se hizo (primera conexión) o pasaron 24 h.
// Va en una goroutine aparte (usa la red) y solo corre una a la vez.
func (s *estado) snapshotSiToca(ctx context.Context, forzar bool) {
	a, err := almacenAbierto()
	if err != nil || s.cli == nil || !s.cli.IsConnected() || !s.cli.IsLoggedIn() {
		return
	}
	if !forzar && time.Since(time.Unix(a.GrupoSnapshotTS(s.botID), 0)) < snapshotCada {
		return
	}
	if !snapshotCorriendo.CompareAndSwap(false, true) {
		return
	}
	defer snapshotCorriendo.Store(false)
	defer func() {
		if r := recover(); r != nil {
			s.log(fmt.Sprintf("error interno en la lista de miembros: %v", r))
		}
	}()
	cx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	gs, err := s.cli.GetJoinedGroups(cx)
	if err != nil {
		s.log("No se pudo leer la lista de grupos para los miembros.")
		return
	}
	vigentes := make([]string, 0, len(gs))
	for i, gi := range gs {
		vigentes = append(vigentes, gi.JID.ToNonAD().String())
		s.guardarMiembros(gi)
		if (i+1)%snapshotPorTanda == 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(snapshotPausa):
			}
		}
	}
	a.CerrarGruposAusentes(s.botID, vigentes, time.Now())
	a.FijarGrupoSnapshotTS(s.botID, time.Now().Unix())
	s.log(fmt.Sprintf("Miembros de %d grupos al día.", len(gs)))
}
