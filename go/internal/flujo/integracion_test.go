package flujo

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/motor"
)

// Integración: el motor real (cola de salida, mención, cita, registro del mensaje del bot) con el motor de menús, sin red.

type enviado struct {
	chat                         string
	sal                          motor.Salida
	citaID, citaAutor, mencion   string
	id                           string
}

type enviadorRico struct {
	mu   sync.Mutex
	l    []enviado
	n    int
	solo int // veces que se usó el envío de solo texto (no debería pasar)
}

func (f *enviadorRico) Enviar(context.Context, string, string, string, string) error {
	f.mu.Lock()
	f.solo++
	f.mu.Unlock()
	return nil
}
func (f *enviadorRico) Escribiendo(context.Context, string, bool) {}
func (f *enviadorRico) EnviarSalida(_ context.Context, chat string, s motor.Salida, citaID, citaAutor, mencion string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	id := fmt.Sprintf("OUT%d", f.n)
	f.l = append(f.l, enviado{chat, s, citaID, citaAutor, mencion, id})
	return id, nil
}
func (f *enviadorRico) lista() []enviado {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]enviado(nil), f.l...)
}

// relojSeguro sigue al reloj real desde una hora fija (así las esperas del motor avanzan) y permite adelantarlo.
type relojSeguro struct {
	mu       sync.Mutex
	base     time.Time
	inicio   time.Time
	adelanto time.Duration
}

func (r *relojSeguro) ahora() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.base.Add(time.Since(r.inicio) + r.adelanto)
}
func (r *relojSeguro) avanza(d time.Duration) {
	r.mu.Lock()
	r.adelanto += d
	r.mu.Unlock()
}

type integ struct {
	alm *almacen.Almacen
	fm  *Motor
	m   *motor.Motor
	env *enviadorRico
	rel *relojSeguro
	seq int
}

func nuevaIntegracion(t *testing.T, ajusta func(*motor.Config)) *integ {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 77)
	}
	alm, err := almacen.Abrir(t.TempDir(), k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = alm.Cerrar() })
	_ = alm.FijarZona("America/Mexico_City")
	it := &integ{alm: alm, env: &enviadorRico{}, rel: &relojSeguro{base: time.Date(2026, 11, 10, 16, 0, 0, 0, time.UTC), inicio: time.Now()}}
	zona, _ := time.LoadLocation("America/Mexico_City")
	it.fm = Nuevo(Config{BotID: 1, Alm: alm, Ahora: it.rel.ahora, Zona: func() *time.Location { return zona },
		Nombre:  func(chat, sender string) string { return "Ana" },
		Persona: func(p string) string { return alm.ResolverPersona(1, p) }})
	cfg := motor.Config{BotID: 1, Alm: alm, Decisor: it.fm, Enviador: it.env, Ahora: it.rel.ahora,
		Debounce: 10 * time.Millisecond, DebounceMax: 40 * time.Millisecond, EsperaMin: time.Millisecond, EsperaMax: 2 * time.Millisecond,
		Aleatorio: func(n int64) int64 { return n / 2 }, LimiteGrupo: func() int { return alm.AjusteBotInt(1, "grupo_msgs_min") },
		AlEnviado: func(chat, id string, ref *motor.RefSesion) {
			if strings.HasSuffix(chat, "@g.us") {
				_ = alm.GuardarMsgBot(1, chat, id, ref.ID, ref.Persona, ref.MenuID, it.rel.ahora())
			}
		}}
	if ajusta != nil {
		ajusta(&cfg)
	}
	it.m = motor.Nuevo(cfg)
	it.m.Iniciar(context.Background())
	t.Cleanup(it.m.Detener)
	alm.AprenderAlias(1, pnBen, lidBn)
	return it
}

func (it *integ) espera(t *testing.T, n int) []enviado {
	t.Helper()
	limite := time.Now().Add(10 * time.Second)
	for time.Now().Before(limite) {
		if l := it.env.lista(); len(l) >= n {
			return l
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("esperaba %d envíos y hay %d: %+v", n, len(it.env.lista()), it.env.lista())
	return nil
}

func (it *integ) quieto() { time.Sleep(200 * time.Millisecond) }

// grupoMsg arma un mensaje de grupo como lo haría wabot.alMotor: quien (JID tal como lo manda WhatsApp) y su clave guardada.
func (it *integ) grupoMsg(quien, texto string) motor.Mensaje {
	it.seq++
	return motor.Mensaje{Chat: grupo, ID: fmt.Sprintf("IN%d", it.seq), Remitente: it.alm.ResolverPersona(1, quien), RemitenteWA: quien, Texto: texto, EsTexto: true, EsGrupo: true, TS: it.rel.ahora()}
}

func creaMenuGrupo(t *testing.T, it *integ) int64 {
	t.Helper()
	d := menuGrupo()
	d.Normalizar()
	id, err := it.alm.CrearMenu(1, d.JSON(), true, false)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestGrupoRespondeCitandoAlAutorExactoYMencionandoConLIDYConNumero(t *testing.T) {
	it := nuevaIntegracion(t, nil)
	creaMenuGrupo(t, it)
	// Ben escribe con su LID (WhatsApp lo manda así); en el almacén es su número
	m1 := it.grupoMsg(lidBn, "tienda")
	m1.Mencionado = true
	it.m.Entrar(m1)
	l := it.espera(t, 1)
	if l[0].chat != grupo || l[0].citaID != m1.ID || l[0].citaAutor != lidBn || l[0].mencion != lidBn || !strings.HasPrefix(l[0].sal.Texto, "@9988776655 Hola Ana") {
		t.Fatalf("con LID: cita al mensaje exacto con su LID tal cual y mención con @usuario: %+v", l[0])
	}
	// el mensaje que salió quedó ligado a la conversación y a su dueña (número canónico)
	ref := it.alm.MsgBotPorID(1, grupo, l[0].id)
	if ref == nil || ref.Persona != pnBen {
		t.Fatalf("msg_bot_sesion: %+v", ref)
	}
	// Clara, con número
	m2 := it.grupoMsg(pnCla, "tienda")
	m2.Mencionado = true
	it.m.Entrar(m2)
	l = it.espera(t, 2)
	if l[1].citaAutor != pnCla || l[1].mencion != pnCla || !strings.HasPrefix(l[1].sal.Texto, "@5215544444444 Hola Ana") || l[1].citaID != m2.ID {
		t.Fatalf("con número: %+v", l[1])
	}
	if it.env.solo != 0 {
		t.Fatal("todo sale por EnviarSalida")
	}
	// Ben sigue SIN mencionar ni citar: escribe una opción válida y se le contesta (cita su mensaje exacto)
	m3 := it.grupoMsg(lidBn, "horario")
	it.m.Entrar(m3)
	l = it.espera(t, 3)
	if !strings.Contains(l[2].sal.Texto, "Abrimos de 9 a 6") || l[2].citaID != m3.ID || l[2].citaAutor != lidBn || l[2].mencion != lidBn {
		t.Fatalf("opción sin citar: %+v", l[2])
	}
	// charla sin mención ni sesión: nada
	m4 := it.grupoMsg(pnDan, "jaja qué buen chiste")
	it.m.Entrar(m4)
	m5 := it.grupoMsg(pnBen, "otra cosa que no es opción")
	it.m.Entrar(m5)
	it.quieto()
	if len(it.env.lista()) != 3 {
		t.Fatalf("la charla se ignora: %+v", it.env.lista()[3:])
	}
	// citando el menú del bot, Ben puede contestar (aunque no mencione)
	m6 := it.grupoMsg(lidBn, "!menu")
	m6.Mencionado = true
	it.m.Entrar(m6)
	l = it.espera(t, 4)
	m7 := it.grupoMsg(lidBn, "mmm no sé")
	m7.CitaID, m7.CitaEsMia = l[3].id, true
	it.m.Entrar(m7)
	l = it.espera(t, 5)
	if !strings.Contains(l[4].sal.Texto, "No entendí") || l[4].citaID != m7.ID {
		t.Fatalf("lo que cita al bot recibe respuesta, incluso un 'no entendí': %+v", l[4])
	}
}

func TestEdicionesYBorradosSeIgnoranYElBotSoloResponde(t *testing.T) {
	it := nuevaIntegracion(t, nil)
	creaMenuGrupo(t, it)
	m := it.grupoMsg(pnBen, "tienda")
	m.Mencionado, m.EsEdicion = true, true
	it.m.Entrar(m)
	it.quieto()
	if len(it.env.lista()) != 0 {
		t.Fatal("una edición no se contesta")
	}
	// sin nadie que escriba, el bot no escribe nada (ni con la fila de espera)
	it.m.ProcesarDiferidas()
	it.quieto()
	if len(it.env.lista()) != 0 {
		t.Fatal("el bot no escribe por su cuenta")
	}
}

func TestFotoOAudioConMenuAbiertoTieneMensajePropioYSinMenuSeOmite(t *testing.T) {
	it := nuevaIntegracion(t, nil)
	creaMenuGrupo(t, it)
	// sin menú abierto: se omite
	f := it.grupoMsg(pnBen, "")
	f.EsTexto, f.Mencionado = false, true
	it.m.Entrar(f)
	it.quieto()
	if len(it.env.lista()) != 0 {
		t.Fatal("sin conversación la foto se omite")
	}
	om := 0
	ls, _ := it.alm.UltimaActividad(1, 100)
	for _, a := range ls {
		if a.Motivo == almacen.MotNoTexto {
			om++
		}
	}
	if om != 1 {
		t.Fatalf("debía anotar que no era texto: %d", om)
	}
	m := it.grupoMsg(pnBen, "tienda")
	m.Mencionado = true
	it.m.Entrar(m)
	it.espera(t, 1)
	// foto sin mención ni cita en el grupo: no es para el bot
	f2 := it.grupoMsg(pnBen, "")
	f2.EsTexto = false
	it.m.Entrar(f2)
	it.quieto()
	if len(it.env.lista()) != 1 {
		t.Fatal("una foto suelta en el grupo se ignora")
	}
	f3 := it.grupoMsg(pnBen, "")
	f3.EsTexto, f3.Mencionado = false, true
	it.m.Entrar(f3)
	l := it.espera(t, 2)
	if !strings.Contains(l[1].sal.Texto, "solo entiendo mensajes escritos") || l[1].citaID != f3.ID || l[1].mencion != pnBen {
		t.Fatalf("%+v", l[1])
	}
	// en un chat privado: sticker/audio con menú abierto (el menú de prueba también sirve en privado)
	it.m.Entrar(motor.Mensaje{Chat: ana, ID: "P2", Remitente: ana, RemitenteWA: ana, Texto: "tienda", EsTexto: true, TS: it.rel.ahora()})
	it.espera(t, 3)
	it.m.Entrar(motor.Mensaje{Chat: ana, ID: "P3", Remitente: ana, RemitenteWA: ana, Texto: "", EsTexto: false, TS: it.rel.ahora()})
	l = it.espera(t, 4)
	if !strings.Contains(l[3].sal.Texto, "solo entiendo mensajes escritos") || l[3].mencion != "" || l[3].citaID != "P3" {
		t.Fatalf("privado: %+v", l[3])
	}
}

func TestTopeDeMensajesPorMinutoEnUnGrupo(t *testing.T) {
	it := nuevaIntegracion(t, nil)
	creaMenuGrupo(t, it)
	if err := it.alm.FijarAjusteBot(1, "grupo_msgs_min", "2"); err != nil {
		t.Fatal(err)
	}
	for i, p := range []string{pnBen, pnCla, pnDan} {
		m := it.grupoMsg(p, "tienda")
		m.Mencionado = true
		it.m.Entrar(m)
		if i < 2 {
			it.espera(t, i+1)
		}
	}
	it.quieto()
	if n := len(it.env.lista()); n != 2 {
		t.Fatalf("máximo 2 por minuto en el grupo: %d", n)
	}
	found := false
	ls, _ := it.alm.UltimaActividad(1, 100)
	for _, a := range ls {
		if a.Motivo == almacen.MotLimite {
			found = true
		}
	}
	if !found {
		t.Fatal("debía anotar el límite")
	}
	// pasado el minuto vuelve a responder
	it.rel.avanza(61 * time.Second)
	m := it.grupoMsg(pnEva, "tienda")
	m.Mencionado = true
	it.m.Entrar(m)
	it.espera(t, 3)
}

func TestFilaDeEsperaSeAtiendeAlLiberarseUnLugarCitandoSuMensaje(t *testing.T) {
	it := nuevaIntegracion(t, func(c *motor.Config) { c.TickDiferidas = time.Hour })
	creaMenuGrupo(t, it)
	it.alm.FijarAjusteBot(1, "grupo_max_sesiones", "1")
	m1 := it.grupoMsg(pnBen, "tienda")
	m1.Mencionado = true
	it.m.Entrar(m1)
	it.espera(t, 1)
	m2 := it.grupoMsg(pnCla, "tienda")
	m2.Mencionado = true
	it.m.Entrar(m2)
	l := it.espera(t, 2)
	if !strings.Contains(l[1].sal.Texto, "turno 1") || l[1].citaID != m2.ID || l[1].mencion != pnCla {
		t.Fatalf("%+v", l[1])
	}
	it.m.ProcesarDiferidas()
	it.quieto()
	if len(it.env.lista()) != 2 {
		t.Fatal("el lugar sigue ocupado")
	}
	// Ben se despide → se libera el lugar → a Clara se le responde el mensaje que escribió antes, mencionándola
	m3 := it.grupoMsg(pnBen, "salir")
	it.m.Entrar(m3)
	it.espera(t, 3)
	it.m.ProcesarDiferidas()
	l = it.espera(t, 4)
	if l[3].citaID != m2.ID || l[3].mencion != pnCla || l[3].citaAutor != pnCla || !strings.HasPrefix(l[3].sal.Texto, "@5215544444444 Hola Ana") {
		t.Fatalf("respuesta diferida: %+v", l[3])
	}
	if ref := it.alm.MsgBotPorID(1, grupo, l[3].id); ref == nil || ref.Persona != pnCla {
		t.Fatalf("también queda ligada: %+v", ref)
	}
	// Clara ya puede seguir su menú sin mencionar
	m4 := it.grupoMsg(pnCla, "horario")
	it.m.Entrar(m4)
	l = it.espera(t, 5)
	if !strings.Contains(l[4].sal.Texto, "Abrimos de 9 a 6") {
		t.Fatalf("%+v", l[4])
	}
}

func TestOpcionPrivadaEnGrupoRespondeEnElGrupoNoAlPrivado(t *testing.T) {
	it := nuevaIntegracion(t, nil)
	d := menuConReglas()
	d.Alcance = Alcance{TodosGrupos: true}
	d.Disparadores = Disparadores{Mencion: true}
	d.Normalizar()
	it.alm.CrearMenu(1, d.JSON(), true, false)
	m := it.grupoMsg(pnBen, "menu")
	m.Mencionado = true
	it.m.Entrar(m)
	it.espera(t, 1)
	m2 := it.grupoMsg(pnBen, "3")
	it.m.Entrar(m2)
	l := it.espera(t, 2)
	if l[1].chat != grupo || l[1].sal.Texto != "@5215533333333 Esa opción es solo en privado: escríbeme al privado." {
		t.Fatalf("%+v", l[1])
	}
	for _, x := range it.env.lista() {
		if !strings.HasSuffix(x.chat, "@g.us") {
			t.Fatalf("el bot no escribe al privado por su cuenta: %+v", x)
		}
	}
}
