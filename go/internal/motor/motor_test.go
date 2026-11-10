package motor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"botchat/internal/almacen"
)

type envio struct{ chat, texto, cita, mencion string }

type enviadorFalso struct {
	mu       sync.Mutex
	enviados []envio
	falla    atomic.Bool
	escr     int
}

func (e *enviadorFalso) Enviar(_ context.Context, chat, texto, cita, mencion string) error {
	if e.falla.Load() {
		return errors.New("falló")
	}
	e.mu.Lock()
	e.enviados = append(e.enviados, envio{chat, texto, cita, mencion})
	e.mu.Unlock()
	return nil
}
func (e *enviadorFalso) Escribiendo(context.Context, string, bool) {
	e.mu.Lock()
	e.escr++
	e.mu.Unlock()
}
func (e *enviadorFalso) lista() []envio {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]envio(nil), e.enviados...)
}

// decisorEco responde "eco <texto del último>" y recuerda cuántos mensajes llegaron en cada lote.
type decisorEco struct {
	mu     sync.Mutex
	lotes  []int
	espera chan struct{}
}

func (d *decisorEco) Decidir(_ context.Context, ultimo Mensaje, lote []Mensaje) (Decision, error) {
	if d.espera != nil {
		<-d.espera
	}
	d.mu.Lock()
	d.lotes = append(d.lotes, len(lote))
	d.mu.Unlock()
	return Decision{Coincidio: "eco", Respuestas: []Salida{{Texto: "eco " + ultimo.Texto}}}, nil
}

// reloj sigue al reloj real y permite adelantarlo (para probar minutos sin esperarlos).
type reloj struct {
	mu       sync.Mutex
	adelanto time.Duration
}

func (r *reloj) ahora() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return time.Now().Add(r.adelanto)
}
func (r *reloj) avanza(d time.Duration) { r.mu.Lock(); r.adelanto += d; r.mu.Unlock() }

type entorno struct {
	m   *Motor
	alm *almacen.Almacen
	env *enviadorFalso
	dec *decisorEco
	rel *reloj
	cfg Config
}

func nuevoEntorno(t *testing.T, ajusta func(*Config)) *entorno {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 9)
	}
	alm, err := almacen.Abrir(t.TempDir(), k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = alm.Cerrar() })
	e := &entorno{alm: alm, env: &enviadorFalso{}, dec: &decisorEco{}, rel: &reloj{}}
	e.cfg = Config{
		BotID: 1, Alm: alm, Decisor: e.dec, Enviador: e.env, Ahora: e.rel.ahora,
		Debounce: 15 * time.Millisecond, DebounceMax: 60 * time.Millisecond, EsperaMin: time.Millisecond, EsperaMax: 2 * time.Millisecond, PorCaracter: time.Nanosecond,
		Aleatorio: func(n int64) int64 { return n / 2 },
	}
	if ajusta != nil {
		ajusta(&e.cfg)
	}
	e.m = Nuevo(e.cfg)
	e.m.Iniciar(context.Background())
	t.Cleanup(e.m.Detener)
	return e
}

func (e *entorno) espera(t *testing.T, cond func() bool) {
	t.Helper()
	limite := time.Now().Add(10 * time.Second)
	for time.Now().Before(limite) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("no se cumplió a tiempo")
}

func (e *entorno) quieto() { time.Sleep(150 * time.Millisecond) }

func priv(chat, id, texto string) Mensaje {
	return Mensaje{Chat: chat, ID: id, Remitente: chat, Texto: texto, EsTexto: true, TS: time.Now()}
}

func grupoM(chat, id, quien, texto string) Mensaje {
	return Mensaje{Chat: chat, ID: id, Remitente: quien, RemitenteWA: quien, Texto: texto, EsGrupo: true, EsTexto: true, Mencionado: true, TS: time.Now()}
}

func (e *entorno) motivos() map[string]int {
	l, _ := e.alm.UltimaActividad(1, 1000)
	r := map[string]int{}
	for _, a := range l {
		if a.Evento == almacen.ActOmitido {
			r[a.Motivo]++
		}
	}
	return r
}

func TestDiezChatsCincuentaMensajesSimultaneos(t *testing.T) {
	e := nuevoEntorno(t, func(c *Config) {
		c.MaxCola = 1000
		c.LimiteChat = 100000
		c.LimiteGlobal = 100000
		c.Repetido = time.Nanosecond
	})
	const chats, porChat = 10, 50
	var wg sync.WaitGroup
	for c := 0; c < chats; c++ {
		wg.Add(1)
		go func(c int) {
			defer wg.Done()
			chat := fmt.Sprintf("12036300000000%02d@g.us", c)
			for i := 0; i < porChat; i++ {
				e.m.Entrar(grupoM(chat, fmt.Sprintf("c%d-m%d", c, i), "5215500000001@s.whatsapp.net", fmt.Sprintf("c%d-m%d", c, i)))
			}
		}(c)
	}
	wg.Wait()
	e.espera(t, func() bool { return len(e.env.lista()) >= chats*porChat })
	e.quieto()
	envios := e.env.lista()
	if len(envios) != chats*porChat {
		t.Fatalf("respuestas: %d (debían ser %d: cero duplicados, cero perdidas)", len(envios), chats*porChat)
	}
	visto := map[string]bool{}
	ultimo := map[string]int{}
	for _, x := range envios {
		if visto[x.chat+"|"+x.cita] {
			t.Fatalf("respuesta duplicada: %s %s", x.chat, x.cita)
		}
		visto[x.chat+"|"+x.cita] = true
		var c, i int
		if _, err := fmt.Sscanf(x.cita, "c%d-m%d", &c, &i); err != nil {
			t.Fatal(x.cita)
		}
		if !strings.HasPrefix(x.chat, fmt.Sprintf("12036300000000%02d", c)) {
			t.Fatalf("respuesta cruzada: el mensaje %s se contestó en %s", x.cita, x.chat)
		}
		if !strings.Contains(x.texto, fmt.Sprintf("eco @5215500000001 c%d-m%d", c, i)) && !strings.HasSuffix(x.texto, fmt.Sprintf("c%d-m%d", c, i)) {
			t.Fatalf("el texto no corresponde al mensaje citado: %q / %s", x.texto, x.cita)
		}
		if p, ok := ultimo[x.chat]; ok && i != p+1 {
			t.Fatalf("orden roto en %s: después de %d vino %d", x.chat, p, i)
		}
		ultimo[x.chat] = i
		if x.mencion != "5215500000001@s.whatsapp.net" || !strings.HasPrefix(x.texto, "@5215500000001 ") {
			t.Fatalf("en grupos se menciona a quien lo provocó: %+v", x)
		}
	}
}

func TestRafagaEnPrivadoSeContestaUnaSolaVez(t *testing.T) {
	e := nuevoEntorno(t, func(c *Config) { c.Repetido = time.Nanosecond; c.Debounce = 150 * time.Millisecond; c.DebounceMax = 2 * time.Second })
	for i := 0; i < 5; i++ {
		e.m.Entrar(priv("5215511111111@s.whatsapp.net", fmt.Sprintf("r%d", i), fmt.Sprintf("hola %d", i)))
	}
	e.espera(t, func() bool { return len(e.env.lista()) >= 1 })
	e.quieto()
	envios := e.env.lista()
	if len(envios) != 1 || envios[0].cita != "r4" || envios[0].texto != "eco hola 4" || envios[0].mencion != "" {
		t.Fatalf("%+v", envios)
	}
	if e.dec.lotes[0] != 5 {
		t.Fatalf("el lote debía traer los 5 mensajes: %v", e.dec.lotes)
	}
}

func TestLaRafagaLargaSeCortaAlLlegarAlTope(t *testing.T) {
	e := nuevoEntorno(t, func(c *Config) {
		c.Debounce = 60 * time.Millisecond
		c.DebounceMax = 150 * time.Millisecond
		c.Repetido = time.Nanosecond
	})
	for i := 0; i < 20; i++ { // un mensaje cada 25 ms durante medio segundo: nunca hay silencio de 60 ms
		e.m.Entrar(priv("5215511111111@s.whatsapp.net", fmt.Sprintf("l%d", i), fmt.Sprintf("m%d", i)))
		time.Sleep(25 * time.Millisecond)
	}
	e.espera(t, func() bool { return len(e.env.lista()) >= 3 })
	if n := len(e.env.lista()); n > 8 {
		t.Fatalf("demasiadas respuestas para una ráfaga: %d", n)
	}
}

func TestEnGruposNoSeAgrupa(t *testing.T) {
	e := nuevoEntorno(t, func(c *Config) { c.Repetido = time.Nanosecond; c.LimiteChat = 100 })
	for i := 0; i < 4; i++ {
		e.m.Entrar(grupoM("120363000000000001@g.us", fmt.Sprintf("g%d", i), "5215500000001@s.whatsapp.net", fmt.Sprintf("hola %d", i)))
	}
	e.espera(t, func() bool { return len(e.env.lista()) >= 4 })
	for _, n := range e.dec.lotes {
		if n != 1 {
			t.Fatalf("en grupos cada mensaje va solo: %v", e.dec.lotes)
		}
	}
}

func TestNuncaDosRespuestasAlMismoMensajeNiTrasReiniciar(t *testing.T) {
	e := nuevoEntorno(t, nil)
	m := priv("5215511111111@s.whatsapp.net", "UNICO", "hola")
	e.m.Entrar(m)
	e.m.Entrar(m)
	e.espera(t, func() bool { return len(e.env.lista()) >= 1 })
	e.quieto()
	if len(e.env.lista()) != 1 || e.motivos()[almacen.MotDuplicado] != 1 {
		t.Fatalf("%d respuestas, motivos %v", len(e.env.lista()), e.motivos())
	}
	// "Reiniciar la app": otro motor con la misma base no vuelve a contestar ese mensaje.
	e.m.Detener()
	otro := Nuevo(e.cfg)
	otro.Iniciar(context.Background())
	defer otro.Detener()
	otro.Entrar(m)
	e.quieto()
	if len(e.env.lista()) != 1 {
		t.Fatal("tras reiniciar no debe contestar de nuevo")
	}
}

func TestSeOmitenLosViejosLosDeSincronizacionYLosApagados(t *testing.T) {
	activo := atomic.Bool{}
	activo.Store(true)
	e := nuevoEntorno(t, func(c *Config) {
		c.Activo = activo.Load
		c.VentanaAntigua = func() time.Duration { return 10 * time.Minute }
	})
	viejo := priv("5215511111111@s.whatsapp.net", "V1", "hola")
	viejo.TS = e.rel.ahora().Add(-11 * time.Minute)
	e.m.Entrar(viejo)
	sync := priv("5215511111111@s.whatsapp.net", "S1", "hola")
	sync.Antiguo = true
	e.m.Entrar(sync)
	reciente := priv("5215511111111@s.whatsapp.net", "R1", "hola")
	reciente.TS = e.rel.ahora().Add(-9 * time.Minute)
	e.m.Entrar(reciente)
	e.espera(t, func() bool { return len(e.env.lista()) == 1 })
	if e.motivos()[almacen.MotAntiguo] != 2 {
		t.Fatalf("%v", e.motivos())
	}
	activo.Store(false)
	e.m.Entrar(priv("5215522222222@s.whatsapp.net", "A1", "hola"))
	e.quieto()
	if e.motivos()[almacen.MotApagado] != 1 || len(e.env.lista()) != 1 {
		t.Fatalf("%v", e.motivos())
	}
}

func TestPausasYHumano(t *testing.T) {
	e := nuevoEntorno(t, nil)
	chat := "5215511111111@s.whatsapp.net"
	_ = e.alm.PausarBot(1, chat, time.Now().Add(time.Minute).Unix())
	e.m.Entrar(priv(chat, "P1", "hola"))
	_ = e.alm.PausarBotHumano(1, "5215522222222@s.whatsapp.net", time.Now().Add(time.Hour).Unix())
	e.m.Entrar(priv("5215522222222@s.whatsapp.net", "H1", "hola"))
	e.quieto()
	mot := e.motivos()
	if mot[almacen.MotPausado] != 1 || mot[almacen.MotHumano] != 1 || len(e.env.lista()) != 0 {
		t.Fatalf("%v", mot)
	}
	_ = e.alm.PausarBot(1, chat, 0)
	e.m.Entrar(priv(chat, "P2", "ya"))
	e.espera(t, func() bool { return len(e.env.lista()) == 1 })
}

func TestGruposSoloConMencionCitaOComando(t *testing.T) {
	e := nuevoEntorno(t, func(c *Config) { c.Repetido = time.Nanosecond; c.LimiteChat = 100 })
	g := "120363000000000001@g.us"
	mk := func(id, texto string, menc, cita bool) Mensaje {
		m := grupoM(g, id, "5215500000001@s.whatsapp.net", texto)
		m.Mencionado, m.CitaEsMia = menc, cita
		return m
	}
	e.m.Entrar(mk("a", "buenos días a todos", false, false))
	e.m.Entrar(mk("b", "hola bot", true, false))
	e.m.Entrar(mk("c", "sí gracias", false, true))
	e.m.Entrar(mk("d", ".menu", false, false))
	e.m.Entrar(mk("e", "...y luego", false, false))
	e.espera(t, func() bool { return len(e.env.lista()) == 3 })
	e.quieto()
	if e.motivos()[almacen.MotGrupo] != 2 || len(e.env.lista()) != 3 {
		t.Fatalf("%v %d", e.motivos(), len(e.env.lista()))
	}
	// Interruptor por chat: en pausa no contesta ni mencionado.
	_ = e.alm.FijarBotActivo(1, g, false)
	e.m.Entrar(mk("f", "hola otra vez", true, false))
	e.quieto()
	if e.motivos()[almacen.MotChatPausa] != 1 || len(e.env.lista()) != 3 {
		t.Fatalf("%v", e.motivos())
	}
}

func TestElMismoComandoDeLaMismaPersonaSeIgnoraAntesDe5Segundos(t *testing.T) {
	e := nuevoEntorno(t, func(c *Config) { c.LimiteChat = 100 })
	g := "120363000000000001@g.us"
	e.m.Entrar(grupoM(g, "a", "5215500000001@s.whatsapp.net", ".menu"))
	e.m.Entrar(grupoM(g, "b", "5215500000001@s.whatsapp.net", ".MENU "))
	e.m.Entrar(grupoM(g, "c", "5215500000002@s.whatsapp.net", ".menu")) // otra persona: sí
	e.espera(t, func() bool { return len(e.env.lista()) == 2 })
	e.quieto()
	if e.motivos()[almacen.MotRepetido] != 1 {
		t.Fatalf("%v", e.motivos())
	}
	e.rel.avanza(6 * time.Second)
	e.m.Entrar(grupoM(g, "d", "5215500000001@s.whatsapp.net", ".menu"))
	e.espera(t, func() bool { return len(e.env.lista()) == 3 })
}

func TestLimitesPorMinutoGlobalYPorChat(t *testing.T) {
	e := nuevoEntorno(t, func(c *Config) { c.Repetido = time.Nanosecond })
	g := "120363000000000001@g.us"
	for i := 0; i < 9; i++ {
		e.m.Entrar(grupoM(g, fmt.Sprintf("m%d", i), "5215500000001@s.whatsapp.net", fmt.Sprintf("hola %d", i)))
	}
	e.espera(t, func() bool { return len(e.env.lista()) >= 6 })
	e.quieto()
	if len(e.env.lista()) != 6 || e.motivos()[almacen.MotLimite] != 3 {
		t.Fatalf("máximo 6 por minuto en un chat: %d, %v", len(e.env.lista()), e.motivos())
	}
	e.rel.avanza(61 * time.Second)
	e.m.Entrar(grupoM(g, "tarde", "5215500000001@s.whatsapp.net", "ya pasó el minuto"))
	e.espera(t, func() bool { return len(e.env.lista()) == 7 })
}

func TestElLimiteGlobalDe20PorMinuto(t *testing.T) {
	e := nuevoEntorno(t, func(c *Config) { c.Repetido = time.Nanosecond })
	for c := 0; c < 5; c++ {
		for i := 0; i < 6; i++ {
			e.m.Entrar(grupoM(fmt.Sprintf("1203630000000000%02d@g.us", c), fmt.Sprintf("c%d-%d", c, i), "5215500000001@s.whatsapp.net", fmt.Sprintf("hola %d", i)))
		}
	}
	e.espera(t, func() bool { return len(e.env.lista()) >= 20 })
	e.quieto()
	if len(e.env.lista()) != 20 {
		t.Fatalf("el global es 20 por minuto: %d", len(e.env.lista()))
	}
}

func TestCincoFallosSeguidosFrenanElMotorUnosMinutos(t *testing.T) {
	frenos := atomic.Int32{}
	e := nuevoEntorno(t, func(c *Config) {
		c.Repetido = time.Nanosecond
		c.LimiteChat = 100
		c.LimiteGlobal = 100
		c.AlFrenar = func() { frenos.Add(1) }
	})
	e.env.falla.Store(true)
	g := "120363000000000001@g.us"
	for i := 0; i < 5; i++ {
		e.m.Entrar(grupoM(g, fmt.Sprintf("f%d", i), "5215500000001@s.whatsapp.net", fmt.Sprintf("x%d", i)))
	}
	e.espera(t, func() bool { return frenos.Load() == 1 })
	if !e.m.Frenado() {
		t.Fatal("debe estar frenado")
	}
	e.env.falla.Store(false)
	e.m.Entrar(grupoM(g, "durante", "5215500000001@s.whatsapp.net", "durante el freno"))
	e.quieto()
	if len(e.env.lista()) != 0 || e.motivos()[almacen.MotFreno] != 1 {
		t.Fatalf("durante el freno no sale nada: %v", e.motivos())
	}
	e.rel.avanza(6 * time.Minute)
	if e.m.Frenado() {
		t.Fatal("pasados 5 minutos ya no está frenado")
	}
	e.m.Entrar(grupoM(g, "despues", "5215500000001@s.whatsapp.net", "ya"))
	e.espera(t, func() bool { return len(e.env.lista()) == 1 })
}

func TestUnExitoReiniciaLaCuentaDeFallos(t *testing.T) {
	frenos := atomic.Int32{}
	e := nuevoEntorno(t, func(c *Config) {
		c.Repetido = time.Nanosecond
		c.LimiteChat = 100
		c.LimiteGlobal = 100
		c.AlFrenar = func() { frenos.Add(1) }
	})
	g := "120363000000000001@g.us"
	for ronda := 0; ronda < 3; ronda++ {
		e.env.falla.Store(true)
		for i := 0; i < 4; i++ {
			e.m.Entrar(grupoM(g, fmt.Sprintf("f%d-%d", ronda, i), "5215500000001@s.whatsapp.net", fmt.Sprintf("x%d-%d", ronda, i)))
		}
		e.quieto()
		e.env.falla.Store(false)
		e.m.Entrar(grupoM(g, fmt.Sprintf("ok%d", ronda), "5215500000001@s.whatsapp.net", fmt.Sprintf("ok%d", ronda)))
		e.espera(t, func() bool { return len(e.env.lista()) == ronda+1 })
	}
	if frenos.Load() != 0 {
		t.Fatal("cuatro fallos y un éxito no deben frenar")
	}
}

func TestColaMaximaDe10PorChatDescartaLosMasViejos(t *testing.T) {
	e := nuevoEntorno(t, func(c *Config) { c.Repetido = time.Nanosecond; c.LimiteChat = 100; c.LimiteGlobal = 100 })
	e.dec.espera = make(chan struct{}) // el primero se queda decidiendo y los demás se acumulan
	g := "120363000000000001@g.us"
	for i := 0; i < 16; i++ {
		e.m.Entrar(grupoM(g, fmt.Sprintf("q%02d", i), "5215500000001@s.whatsapp.net", fmt.Sprintf("m%02d", i)))
		time.Sleep(2 * time.Millisecond)
	}
	e.espera(t, func() bool { return e.motivos()[almacen.MotCola] >= 5 })
	close(e.dec.espera)
	e.espera(t, func() bool { return len(e.env.lista()) >= 11 })
	e.quieto()
	envios := e.env.lista()
	if len(envios) != 11 { // el primero + los 10 más recientes
		t.Fatalf("%d respuestas", len(envios))
	}
	if envios[0].cita != "q00" || envios[1].cita != "q06" || envios[10].cita != "q15" {
		t.Fatalf("se descartan los más viejos: %v", envios)
	}
}

type decisorHumano struct{}

func (decisorHumano) Decidir(_ context.Context, u Mensaje, _ []Mensaje) (Decision, error) {
	return Decision{Coincidio: "pasar", Respuestas: []Salida{{Texto: "Te paso con una persona"}}, PasarAHumano: true, Aviso: "Alguien pidió hablar con una persona"}, nil
}

func TestPasarAHumanoCallaElChat12Horas(t *testing.T) {
	avisos := make(chan string, 2)
	e := nuevoEntorno(t, func(c *Config) {
		c.Decisor = decisorHumano{}
		c.AlHumano = func(chat, aviso string) { avisos <- aviso }
	})
	chat := "5215511111111@s.whatsapp.net"
	e.m.Entrar(priv(chat, "H1", "quiero hablar con alguien"))
	select {
	case a := <-avisos:
		if a != "Alguien pidió hablar con una persona" {
			t.Fatal(a)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("sin aviso")
	}
	e.espera(t, func() bool { return len(e.env.lista()) == 1 })
	est := e.alm.EstadoDeChat(1, chat)
	if !est.PausaHumano || est.BotPausaHasta < time.Now().Add(11*time.Hour).Unix() {
		t.Fatalf("%+v", est)
	}
	e.m.Entrar(priv(chat, "H2", "¿sigues ahí?"))
	e.quieto()
	if e.motivos()[almacen.MotHumano] < 1 || len(e.env.lista()) != 1 {
		t.Fatalf("%v", e.motivos())
	}
}

func TestElRegistroNoLlevaTextosNiNumeros(t *testing.T) {
	e := nuevoEntorno(t, nil)
	e.m.Entrar(priv("5215511111111@s.whatsapp.net", "L1", "mi secreto 123456"))
	e.espera(t, func() bool { return len(e.env.lista()) == 1 })
	e.quieto()
	l, _ := e.alm.UltimaActividad(1, 100)
	if len(l) == 0 {
		t.Fatal("el registro debe tener renglones")
	}
	for _, a := range l {
		for _, campo := range []string{a.Chat, a.Detalle, a.Motivo} {
			if strings.Contains(campo, "secreto") || strings.Contains(campo, "5215511111111") || strings.Contains(campo, "123456") {
				t.Fatalf("el registro filtra datos: %+v", a)
			}
		}
	}
}

func TestNoResponderEdicionesNiNoTexto(t *testing.T) {
	e := nuevoEntorno(t, nil)
	m := priv("5215511111111@s.whatsapp.net", "E1", "editado")
	m.EsEdicion = true
	e.m.Entrar(m)
	n := priv("5215511111111@s.whatsapp.net", "N1", "")
	n.EsTexto = false
	e.m.Entrar(n)
	propio := priv("5215511111111@s.whatsapp.net", "M1", "yo")
	propio.EsMio = true
	e.m.Entrar(propio)
	e.quieto()
	if len(e.env.lista()) != 0 {
		t.Fatal("no se contesta")
	}
}
