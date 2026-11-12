package fotos

import (
	"bytes"
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"botchat/internal/almacen"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

type clienteFalso struct {
	mu      sync.Mutex
	pedidos []time.Time
	jids    []string
	preview []bool
	resp    func(jid string, p *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error)
}

func (c *clienteFalso) GetProfilePictureInfo(_ context.Context, jid types.JID, p *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
	c.mu.Lock()
	c.pedidos = append(c.pedidos, time.Now())
	c.jids = append(c.jids, jid.String())
	c.preview = append(c.preview, p.Preview)
	f := c.resp
	c.mu.Unlock()
	return f(jid.String(), p)
}

func (c *clienteFalso) pon(f func(string, *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error)) {
	c.mu.Lock()
	c.resp = f
	c.mu.Unlock()
}

func (c *clienteFalso) numero() int { c.mu.Lock(); defer c.mu.Unlock(); return len(c.jids) }

type entorno struct {
	alm       *almacen.Almacen
	g         *Gestor
	cli       *clienteFalso
	conectado bool
	ahora     time.Time
}

func nuevoEntorno(t *testing.T, pausa time.Duration) *entorno {
	t.Helper()
	k := make([]byte, 32)
	alm, err := almacen.Abrir(t.TempDir(), k)
	if err != nil {
		t.Fatal(err)
	}
	e := &entorno{alm: alm, conectado: true, ahora: time.Now()}
	e.cli = &clienteFalso{resp: func(string, *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
		return &types.ProfilePictureInfo{URL: "https://x/foto", ID: "FOTO1"}, nil
	}}
	e.g = Nuevo(Config{BotID: 1, Alm: alm, Pausa: pausa,
		Cliente: func() (Cliente, bool) { return e.cli, e.conectado },
		Bajar:   func(context.Context, string, int64) ([]byte, error) { return []byte("JPEG-DE-PERFIL"), nil },
		Ahora:   func() time.Time { return e.ahora }})
	e.g.Iniciar(context.Background())
	t.Cleanup(func() { e.g.Detener(); _ = alm.Cerrar() })
	return e
}

func (e *entorno) chat(t *testing.T, jid string, conMensajes bool) {
	t.Helper()
	_, _ = e.alm.ActualizarChat(1, jid, false, func(c *almacen.Chat) { c.TelE164 = "+521" })
	if conMensajes {
		_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: "M" + jid, ChatJID: jid, BotID: 1, Tipo: almacen.TipoTexto, Texto: "x", TS: 1})
	}
}

func esperar(t *testing.T, d string, cond func() bool) {
	t.Helper()
	fin := time.Now().Add(5 * time.Second)
	for time.Now().Before(fin) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("tiempo agotado: %s", d)
}

func TestFotoSeGuardaCifrada(t *testing.T) {
	e := nuevoEntorno(t, time.Millisecond)
	j := "5215512345678@s.whatsapp.net"
	e.chat(t, j, true)
	e.g.Pedir(j)
	esperar(t, "foto guardada", func() bool { c, _ := e.alm.ObtenerChat(1, j); return c.FotoRuta != "" })
	c, _ := e.alm.ObtenerChat(1, j)
	if c.FotoID != "FOTO1" || c.FotoTS != e.ahora.Unix() || c.FotoRuta != almacen.RutaFoto(1, j, false) {
		t.Fatalf("%+v", c)
	}
	abs, _ := e.alm.Abs(c.FotoRuta)
	crudo, _ := os.ReadFile(abs)
	if bytes.Contains(crudo, []byte("JPEG-DE-PERFIL")) {
		t.Fatal("la foto quedó en claro")
	}
	if b, err := e.alm.LeerCifrado(c.FotoRuta); err != nil || string(b) != "JPEG-DE-PERFIL" {
		t.Fatalf("%q %v", b, err)
	}
	if !e.cli.preview[0] {
		t.Fatal("para la lista se pide la vista previa")
	}
}

func TestSoloChatsConMensajes(t *testing.T) {
	e := nuevoEntorno(t, time.Millisecond)
	e.chat(t, "111@s.whatsapp.net", false) // número desconocido, sin mensajes
	e.g.Pedir("111@s.whatsapp.net")
	e.g.Forzar("111@s.whatsapp.net")
	e.g.Pedir("noexiste@s.whatsapp.net")
	time.Sleep(80 * time.Millisecond)
	if e.cli.numero() != 0 {
		t.Fatalf("no debe pedir fotos de chats sin mensajes: %v", e.cli.jids)
	}
}

func TestUnaPeticionCada3Segundos(t *testing.T) {
	e := nuevoEntorno(t, 120*time.Millisecond) // misma lógica, en escala de prueba
	for _, j := range []string{"1@s.whatsapp.net", "2@s.whatsapp.net", "3@s.whatsapp.net"} {
		e.chat(t, j, true)
		e.g.Pedir(j)
	}
	esperar(t, "tres peticiones", func() bool { return e.cli.numero() == 3 })
	e.cli.mu.Lock()
	defer e.cli.mu.Unlock()
	for i := 1; i < 3; i++ {
		if d := e.cli.pedidos[i].Sub(e.cli.pedidos[i-1]); d < 110*time.Millisecond {
			t.Fatalf("separación %v entre peticiones %d y %d", d, i-1, i)
		}
	}
}

func TestPorDefectoLaPausaEsDe3Segundos(t *testing.T) {
	if g := Nuevo(Config{}); g.cfg.Pausa != 3*time.Second || g.cfg.Vigencia != 7*24*time.Hour {
		t.Fatalf("%v %v", g.cfg.Pausa, g.cfg.Vigencia)
	}
}

func TestSinFotoYNoAutorizadoSeAnotanYNoSeRepiten(t *testing.T) {
	e := nuevoEntorno(t, time.Millisecond)
	e.cli.pon(func(jid string, _ *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
		if jid == "1@s.whatsapp.net" {
			return nil, whatsmeow.ErrProfilePictureNotSet
		}
		return nil, whatsmeow.ErrProfilePictureUnauthorized
	})
	e.chat(t, "1@s.whatsapp.net", true)
	e.chat(t, "2@s.whatsapp.net", true)
	e.g.Pedir("1@s.whatsapp.net")
	e.g.Pedir("2@s.whatsapp.net")
	esperar(t, "ambos anotados", func() bool {
		a, _ := e.alm.ObtenerChat(1, "1@s.whatsapp.net")
		b, _ := e.alm.ObtenerChat(1, "2@s.whatsapp.net")
		return a.FotoTS > 0 && b.FotoTS > 0
	})
	c, _ := e.alm.ObtenerChat(1, "1@s.whatsapp.net")
	if c.FotoRuta != "" {
		t.Fatal("sin foto no hay ruta")
	}
	// Dentro de los 7 días no se vuelve a preguntar, ni siquiera al revisar vencidas.
	antes := e.cli.numero()
	e.g.Pedir("1@s.whatsapp.net")
	e.g.RevisarVencidas()
	time.Sleep(60 * time.Millisecond)
	if e.cli.numero() != antes {
		t.Fatal("no debe repetir antes de 7 días")
	}
	// Pasados 7 días sí.
	e.g.mu.Lock()
	e.ahora = e.ahora.Add(8 * 24 * time.Hour)
	e.g.visto = map[string]time.Time{}
	e.g.mu.Unlock()
	e.g.RevisarVencidas()
	esperar(t, "se vuelve a pedir", func() bool { return e.cli.numero() > antes })
}

func TestCambioDeFotoFuerzaLaPeticion(t *testing.T) {
	e := nuevoEntorno(t, time.Millisecond)
	j := "5215512345678@s.whatsapp.net"
	e.chat(t, j, true)
	e.g.Pedir(j)
	esperar(t, "primera", func() bool { return e.cli.numero() == 1 })
	e.cli.pon(func(string, *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
		return &types.ProfilePictureInfo{URL: "https://x/nueva", ID: "FOTO2"}, nil
	})
	e.g.Pedir(j) // reciente: no se pide
	time.Sleep(40 * time.Millisecond)
	if e.cli.numero() != 1 {
		t.Fatal("Pedir no repite una foto reciente")
	}
	e.g.Forzar(j) // el evento de cambio sí
	esperar(t, "segunda", func() bool { c, _ := e.alm.ObtenerChat(1, j); return c.FotoID == "FOTO2" })
}

func TestErroresDeRedSeReintentanDespues(t *testing.T) {
	e := nuevoEntorno(t, time.Millisecond)
	j := "1@s.whatsapp.net"
	e.chat(t, j, true)
	fallar := true
	var mu sync.Mutex
	e.cli.pon(func(string, *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
		mu.Lock()
		defer mu.Unlock()
		if fallar {
			return nil, errors.New("sin red")
		}
		return &types.ProfilePictureInfo{URL: "u", ID: "OK"}, nil
	})
	e.g.Pedir(j)
	esperar(t, "primer intento", func() bool { return e.cli.numero() == 1 })
	if c, _ := e.alm.ObtenerChat(1, j); c.FotoTS != 0 {
		t.Fatal("un error de red no marca la foto como revisada")
	}
	mu.Lock()
	fallar = false
	mu.Unlock()
	time.Sleep(20 * time.Millisecond)
	e.g.RevisarVencidas()
	esperar(t, "se reintenta", func() bool { c, _ := e.alm.ObtenerChat(1, j); return c.FotoID == "OK" })
}

func TestSinConexionNoPideNada(t *testing.T) {
	e := nuevoEntorno(t, time.Millisecond)
	e.conectado = false
	e.chat(t, "1@s.whatsapp.net", true)
	e.g.Pedir("1@s.whatsapp.net")
	time.Sleep(50 * time.Millisecond)
	if e.cli.numero() != 0 {
		t.Fatal("sin conexión no se pide")
	}
}

func TestFotoGrandeSoloAlPedirla(t *testing.T) {
	e := nuevoEntorno(t, time.Millisecond)
	j := "1@s.whatsapp.net"
	e.chat(t, j, true)
	e.cli.pon(func(_ string, p *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
		if p.Preview {
			return &types.ProfilePictureInfo{URL: "chica", ID: "F"}, nil
		}
		return &types.ProfilePictureInfo{URL: "grande", ID: "F"}, nil
	})
	e.g.fijaBajar(func(_ context.Context, url string, _ int64) ([]byte, error) { return []byte("IMG-" + url), nil })
	e.g.Pedir(j)
	esperar(t, "chica", func() bool { c, _ := e.alm.ObtenerChat(1, j); return c.FotoRuta != "" })
	if b, _ := e.alm.LeerCifrado(almacen.RutaFoto(1, j, false)); string(b) != "IMG-chica" {
		t.Fatalf("%q", b)
	}
	rel, err := e.g.Grande(context.Background(), j)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := e.alm.LeerCifrado(rel); string(b) != "IMG-grande" {
		t.Fatalf("%q", b)
	}
	n := e.cli.numero()
	if _, err := e.g.Grande(context.Background(), j); err != nil || e.cli.numero() != n {
		t.Fatal("la grande ya guardada no se vuelve a pedir")
	}
	// Casos de error con mensajes claros
	e.cli.pon(func(string, *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
		return nil, whatsmeow.ErrProfilePictureNotSet
	})
	if _, err := e.g.Grande(context.Background(), "9@s.whatsapp.net"); err == nil || err.Error() != "este chat no tiene foto" {
		t.Fatalf("%v", err)
	}
}

func TestFotoDePersonaSinChatUsaGrupoEnComun(t *testing.T) {
	e := nuevoEntorno(t, time.Millisecond)
	vistoCh := make(chan types.JID, 4)
	e.cli.pon(func(_ string, p *whatsmeow.GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
		vistoCh <- p.CommonGID
		return &types.ProfilePictureInfo{ID: "f1", URL: "http://x/f"}, nil
	})
	p := "5215500000001@s.whatsapp.net"
	e.g.PedirPersona(p, p, "120@g.us", false)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && e.alm.FotoDePersona(1, p) == "" {
		time.Sleep(10 * time.Millisecond)
	}
	visto := <-vistoCh
	if e.alm.FotoDePersona(1, p) == "" || visto.String() != "120@g.us" {
		t.Fatalf("foto=%q grupo=%v", e.alm.FotoDePersona(1, p), visto)
	}
	// vigente: no se vuelve a pedir
	antes := e.cli.numero()
	e.g.PedirPersona(p, p, "120@g.us", false)
	time.Sleep(50 * time.Millisecond)
	if e.cli.numero() != antes {
		t.Fatal("no debía pedirse otra vez")
	}
}
