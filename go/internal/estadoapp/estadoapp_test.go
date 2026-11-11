package estadoapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
)

const chatPriv = "5215511111111@s.whatsapp.net"

type falso struct {
	conectado bool
	llaves    bool
	errLlaves error
	errEnvio  error
	parches   []appstate.PatchInfo
	sync      []appstate.WAPatchName
	errSync   error
}

func (f *falso) Conectado() bool { return f.conectado }
func (f *falso) SendAppState(_ context.Context, p appstate.PatchInfo) error {
	if f.errEnvio != nil {
		return f.errEnvio
	}
	f.parches = append(f.parches, p)
	return nil
}
func (f *falso) TieneLlaves(context.Context) (bool, error) { return f.llaves, f.errLlaves }
func (f *falso) Sincronizar(_ context.Context, n appstate.WAPatchName) error {
	f.sync = append(f.sync, n)
	return f.errSync
}

func nuevo(t *testing.T, f *falso) *Servicio {
	t.Helper()
	k := make([]byte, 32)
	alm, err := almacen.Abrir(t.TempDir(), k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = alm.Cerrar() })
	return Nuevo(alm, 1, f, nil)
}

func TestConLlavesCadaAccionManda_ElParcheCorrecto(t *testing.T) {
	f := &falso{conectado: true, llaves: true}
	s := nuevo(t, f)
	ctx := context.Background()
	if st := s.Estado(ctx); !st.Disponible || st.Motivo != "" || !s.Disponible(ctx) {
		t.Fatalf("%+v", st)
	}
	antes := time.Now()
	for _, dur := range []time.Duration{8 * time.Hour, 7 * 24 * time.Hour, 0} {
		if err := s.Silenciar(ctx, chatPriv, dur); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Desilenciar(ctx, chatPriv); err != nil {
		t.Fatal(err)
	}
	if err := s.Fijar(ctx, chatPriv, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Fijar(ctx, chatPriv, false); err != nil {
		t.Fatal(err)
	}
	_, _ = s.Alm.ActualizarChat(1, chatPriv, false, func(c *almacen.Chat) { c.UltimoTS = 1760000000 })
	if err := s.Archivar(ctx, chatPriv, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Archivar(ctx, chatPriv, false); err != nil {
		t.Fatal(err)
	}
	if len(f.parches) != 8 {
		t.Fatalf("%d parches", len(f.parches))
	}
	mute := func(i int) (bool, int64) {
		m := f.parches[i].Mutations[0]
		if m.Index[0] != appstate.IndexMute || m.Index[1] != chatPriv || f.parches[i].Type != appstate.WAPatchRegularHigh {
			t.Fatalf("mute %d: %+v", i, f.parches[i])
		}
		a := m.Value.MuteAction
		return a.GetMuted(), a.GetMuteEndTimestamp()
	}
	if m, fin := mute(0); !m || fin < antes.Add(8*time.Hour).UnixMilli() || fin > time.Now().Add(8*time.Hour).UnixMilli() {
		t.Fatalf("8 h: %v %d", m, fin)
	}
	if m, fin := mute(1); !m || fin < antes.Add(7*24*time.Hour).UnixMilli() {
		t.Fatalf("1 semana: %v %d", m, fin)
	}
	if m, fin := mute(2); !m || fin != -1 {
		t.Fatalf("siempre: %v %d", m, fin)
	}
	if m, _ := mute(3); m {
		t.Fatal("desilenciar")
	}
	for i, quiere := range map[int]bool{4: true, 5: false} {
		m := f.parches[i].Mutations[0]
		if m.Index[0] != appstate.IndexPin || m.Index[1] != chatPriv || m.Value.PinAction.GetPinned() != quiere || f.parches[i].Type != appstate.WAPatchRegularLow {
			t.Fatalf("fijar %d: %+v", i, f.parches[i])
		}
	}
	arch := f.parches[6]
	if arch.Mutations[0].Index[0] != appstate.IndexArchive || !arch.Mutations[0].Value.ArchiveChatAction.GetArchived() ||
		arch.Mutations[0].Value.ArchiveChatAction.MessageRange.GetLastMessageTimestamp() != 1760000000 || len(arch.Mutations) != 2 {
		t.Fatalf("archivar (también desfija): %+v", arch)
	}
	if f.parches[7].Mutations[0].Value.ArchiveChatAction.GetArchived() || len(f.parches[7].Mutations) != 1 {
		t.Fatalf("desarchivar: %+v", f.parches[7])
	}
	// Todo quedó en el registro sin textos.
	acs, _ := s.Alm.ListarAcciones(1, 50, 0)
	vistos := map[string]int{}
	for _, a := range acs {
		vistos[a.Accion+"|"+a.Resultado]++
	}
	if vistos["wa.silenciar|ok"] != 3 || vistos["wa.fijar|ok"] != 1 || vistos["wa.archivar|ok"] != 1 {
		t.Fatalf("%v", vistos)
	}
}

func TestSinLlavesNoSeIntentaNadaYSeExplica(t *testing.T) {
	f := &falso{conectado: true, llaves: false}
	s := nuevo(t, f)
	ctx := context.Background()
	st := s.Estado(ctx)
	if st.Disponible || !st.Conectado || !strings.Contains(st.Motivo, "el teléfono principal sincronice") || st.Motivo != TextoSinLlaves {
		t.Fatalf("%+v", st)
	}
	for _, err := range []error{s.Silenciar(ctx, chatPriv, time.Hour), s.Desilenciar(ctx, chatPriv), s.Fijar(ctx, chatPriv, true), s.Archivar(ctx, chatPriv, true)} {
		if !errors.Is(err, ErrSinLlaves) {
			t.Fatalf("%v", err)
		}
	}
	if len(f.parches) != 0 {
		t.Fatal("sin llaves no se manda nada")
	}
	// Un error al leer las llaves cuenta como "no hay".
	f.errLlaves = errors.New("base cerrada")
	if s.Disponible(ctx) {
		t.Fatal("error de lectura")
	}
	// Sin conexión.
	f2 := &falso{conectado: false, llaves: true}
	s2 := nuevo(t, f2)
	if st := s2.Estado(ctx); st.Disponible || st.Conectado || !strings.Contains(st.Motivo, "no está conectado") {
		t.Fatalf("%+v", st)
	}
	if err := s2.Fijar(ctx, chatPriv, true); !errors.Is(err, ErrSinConexion) {
		t.Fatalf("%v", err)
	}
	// Cliente nulo (bot apagado).
	s3 := Nuevo(s.Alm, 1, nil, nil)
	if s3.Disponible(ctx) {
		t.Fatal("nulo")
	}
}

func TestErroresDeLaLibreriaSeTraducen(t *testing.T) {
	casos := []struct {
		err    error
		quiere error
		texto  string
	}{
		{fmt.Errorf("no app state keys found, creating app state keys is not yet supported"), ErrSinLlaves, ""},
		{fmt.Errorf("x: %w", appstate.ErrKeyNotFound), ErrSinLlaves, ""},
		{whatsmeow.ErrNotConnected, ErrSinConexion, ""},
		{fmt.Errorf("%w: hasta luego", whatsmeow.ErrAppStateRateLimited), nil, "más despacio"},
		{fmt.Errorf("%w: algo", whatsmeow.ErrAppStateUpdate), nil, "no aceptó el cambio"},
		{context.DeadlineExceeded, nil, "tardó demasiado"},
		{errors.New("cosa rara"), nil, "no pudo hacerlo"},
	}
	for _, c := range casos {
		f := &falso{conectado: true, llaves: true, errEnvio: c.err}
		s := nuevo(t, f)
		err := s.Fijar(context.Background(), chatPriv, true)
		if c.quiere != nil && !errors.Is(err, c.quiere) {
			t.Fatalf("%v -> %v", c.err, err)
		}
		if c.texto != "" && (err == nil || !strings.Contains(err.Error(), c.texto)) {
			t.Fatalf("%v -> %v", c.err, err)
		}
		if err != nil && strings.Contains(err.Error(), "keys") {
			t.Fatalf("texto en inglés: %v", err)
		}
	}
}

func TestChatInvalidoYSincronizacion(t *testing.T) {
	f := &falso{conectado: true, llaves: true}
	s := nuevo(t, f)
	for _, c := range []string{"", "no es un jid@@", "status@broadcast"} {
		if err := s.Fijar(context.Background(), c, true); !errors.Is(err, ErrChat) {
			t.Fatalf("%q: %v", c, err)
		}
	}
	if err := s.PedirSincronizacion(context.Background()); err != nil || len(f.sync) != 2 || f.sync[0] != appstate.WAPatchRegularHigh || f.sync[1] != appstate.WAPatchRegularLow {
		t.Fatalf("%v %v", err, f.sync)
	}
	f.errSync = whatsmeow.ErrAppStateRateLimited
	if err := s.PedirSincronizacion(context.Background()); err == nil || !strings.Contains(err.Error(), "más despacio") {
		t.Fatalf("%v", err)
	}
	f.conectado = false
	if err := s.PedirSincronizacion(context.Background()); !errors.Is(err, ErrSinConexion) {
		t.Fatalf("%v", err)
	}
}
