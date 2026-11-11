package auto

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/flujo"
	"botchat/internal/motor"
)

// Integración: el motor real (cola de salida, pausas, cancelación) con el decisor de menús y el de mensajes automáticos, sin red.

const (
	ana = "5215511111111@s.whatsapp.net"
	ben = "5215522222222@s.whatsapp.net"
	cla = "5215533333333@s.whatsapp.net"
)

type saliente struct {
	chat     string
	sal      motor.Salida
	etiqueta string
	id       string
}

// envFalso cumple el mismo contrato que wabot.enviadorMotor: cuenta los archivos que salen, y si uno falla manda solo el texto (o lo
// omite con motor.ErrSaltado).
type envFalso struct {
	mu         sync.Mutex
	l          []saliente
	n          int
	fallaMedio map[string]bool
}

func (f *envFalso) Enviar(context.Context, string, string, string, string) error { return nil }
func (f *envFalso) Escribiendo(context.Context, string, bool)                    {}
func (f *envFalso) EnviarSalida(ctx context.Context, chat string, s motor.Salida, _, _, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s.Medio != nil && f.fallaMedio[s.Medio.ID] {
		s.Auto.Fallo("archivo_ilegible")
		if strings.TrimSpace(s.Texto) == "" {
			return "", motor.ErrSaltado
		}
		s.Medio = nil // solo el texto
	} else if s.Medio != nil && s.Auto != nil {
		s.Auto.AdjOK++
	}
	f.n++
	id := fmt.Sprintf("OUT%d", f.n)
	f.l = append(f.l, saliente{chat, s, almacen.EtiquetaDe(ctx), id})
	return id, nil
}
func (f *envFalso) lista() []saliente {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]saliente(nil), f.l...)
}

// reloj falso: avanza solo cuando alguien duerme o lo adelantan.
type reloj struct {
	mu   sync.Mutex
	t    time.Time
	dorm []time.Duration
}

func (r *reloj) ahora() time.Time { r.mu.Lock(); defer r.mu.Unlock(); return r.t }
func (r *reloj) avanza(d time.Duration) {
	r.mu.Lock()
	r.t = r.t.Add(d)
	r.mu.Unlock()
}
func (r *reloj) dormir(_ context.Context, d time.Duration) bool {
	r.mu.Lock()
	r.t = r.t.Add(d)
	r.dorm = append(r.dorm, d)
	r.mu.Unlock()
	time.Sleep(time.Millisecond)
	return true
}
func (r *reloj) pausas() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]time.Duration(nil), r.dorm...)
}

type ent struct {
	base     int // renglones de actividad "terminados" antes del último mensaje que llegó
	alm      *almacen.Almacen
	fm       *flujo.Motor
	dec      *Decisor
	m        *motor.Motor
	env      *envFalso
	rel      *reloj
	conocido map[string]bool
	seq      int
}

func nuevo(t *testing.T) *ent {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 31)
	}
	alm, err := almacen.Abrir(t.TempDir(), k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = alm.Cerrar() })
	_ = alm.FijarZona("America/Mexico_City")
	zona, _ := time.LoadLocation("America/Mexico_City")
	e := &ent{alm: alm, env: &envFalso{fallaMedio: map[string]bool{}}, conocido: map[string]bool{},
		rel: &reloj{t: time.Date(2026, 11, 10, 16, 0, 0, 0, time.UTC)}} // martes 10:00 en Ciudad de México
	e.fm = flujo.Nuevo(flujo.Config{BotID: 1, Alm: alm, Ahora: e.rel.ahora, Zona: func() *time.Location { return zona },
		Nombre: func(chat, sender string) string { return "Ana" }, Persona: func(p string) string { return alm.ResolverPersona(1, p) }})
	e.dec = Nuevo(DecisorConfig{BotID: 1, Alm: alm, Base: e.fm, Ahora: e.rel.ahora, Zona: func() *time.Location { return zona },
		Nombre:   func(chat, sender string) string { return "Ana" },
		Persona:  func(p string) string { return alm.ResolverPersona(1, p) },
		Conocido: func(p, chat string) bool { return e.conocido[p] }})
	e.m = motor.Nuevo(motor.Config{BotID: 1, Alm: alm, Decisor: e.dec, Enviador: e.env, Ahora: e.rel.ahora, Dormir: e.rel.dormir,
		Debounce: 10 * time.Millisecond, DebounceMax: 40 * time.Millisecond, EsperaMin: time.Millisecond, EsperaMax: 2 * time.Millisecond,
		Aleatorio: func(n int64) int64 { return n / 2 }, AlAuto: e.dec.AlAuto, LimiteGlobal: 1000, LimiteChat: 1000})
	e.m.Iniciar(context.Background())
	t.Cleanup(e.m.Detener)
	return e
}

// crea un mensaje automático activo y devuelve su id.
func (e *ent) crea(t *testing.T, tipo, texto string, ajusta func(*Config), adj ...almacen.AdjuntoRef) int64 {
	t.Helper()
	c := ConfigPorDefecto(tipo)
	if ajusta != nil {
		ajusta(&c)
	}
	m := &almacen.AutoMensaje{Tipo: tipo, Texto: texto, Activo: true, Adjuntos: adj}
	if err := Validar(m, &c); err != nil {
		t.Fatal(err)
	}
	m.Config = c.JSON()
	id, err := e.alm.CrearAuto(1, m)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// llega entrega un mensaje de texto de `de` (privado) y lo deja guardado en el almacén como lo haría la ingesta.
func (e *ent) llega(de, texto string) motor.Mensaje {
	e.base = e.terminados()
	e.seq++
	m := motor.Mensaje{Chat: de, ID: fmt.Sprintf("IN%d", e.seq), Remitente: de, Texto: texto, EsTexto: true, TS: e.rel.ahora()}
	_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: m.ID, ChatJID: de, BotID: 1, RemitenteJID: de, Tipo: "texto", Texto: texto, TS: m.TS.Unix()})
	e.m.Entrar(m)
	return m
}

func (e *ent) espera(t *testing.T, n int) []saliente {
	t.Helper()
	limite := time.Now().Add(10 * time.Second)
	for time.Now().Before(limite) {
		if l := e.env.lista(); len(l) >= n {
			return l
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("esperaba %d envíos y hay %d: %+v", n, len(e.env.lista()), e.env.lista())
	return nil
}

// terminados cuenta los mensajes que el motor ya resolvió (los respondió, no coincidieron con nada o los omitió).
func (e *ent) terminados() int {
	l, _ := e.alm.UltimaActividad(1, 1000)
	n := 0
	for _, a := range l {
		if a.Evento == almacen.ActSinCoincid || a.Evento == almacen.ActOmitido || a.Evento == almacen.ActCoincidio {
			n++
		}
	}
	return n
}

func (e *ent) nada(t *testing.T, n int) {
	t.Helper()
	// primero se espera a que el motor termine con el último mensaje (con el equipo cargado puede tardar), y luego un momento más
	lim := time.Now().Add(10 * time.Second)
	for e.terminados() <= e.base && time.Now().Before(lim) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(150 * time.Millisecond)
	if l := e.env.lista(); len(l) != n {
		t.Fatalf("esperaba %d envíos y hay %d: %+v", n, len(l), l)
	}
}

func textos(l []saliente) string {
	var s []string
	for _, x := range l {
		s = append(s, x.sal.Texto)
	}
	return strings.Join(s, " | ")
}

func TestBienvenidaSoloLaPrimeraVezYEtiquetaAutomatico(t *testing.T) {
	e := nuevo(t)
	e.crea(t, almacen.AutoBienvenida, "¡Hola {nombre}! Bienvenida.", nil)
	e.llega(ana, "hola")
	l := e.espera(t, 1)
	if l[0].sal.Texto != "¡Hola Ana! Bienvenida." || l[0].etiqueta != "bienvenida" || l[0].sal.Etiqueta != "bienvenida" {
		t.Fatalf("%+v", l[0])
	}
	e.rel.avanza(time.Minute)
	e.llega(ana, "otra cosa")
	e.nada(t, 1) // ya había escrito: no se repite
	// otra persona sí
	e.llega(ben, "buenas")
	e.espera(t, 2)
	// se anotó la repetición y el registro sin textos
	autos, _ := e.alm.ListarAutos(1, almacen.AutoBienvenida)
	if e.alm.UltimaRepeticion(1, autos[0].ID, ana) == 0 {
		t.Fatal("repetición por persona")
	}
	reg, _ := e.alm.ListarAutoRegistro(1, autos[0].ID, 10)
	if len(reg) != 2 || reg[0].Resultado != almacen.AutoOK || reg[0].Tipo != "bienvenida" {
		t.Fatalf("registro: %+v", reg)
	}
}

func TestBienvenidaTrasDiasDeSilencio(t *testing.T) {
	for _, dias := range []int{7, 14, 30} {
		t.Run(fmt.Sprint(dias, " días"), func(t *testing.T) {
			e := nuevo(t)
			e.crea(t, almacen.AutoBienvenida, "Hola de nuevo", func(c *Config) { c.SilencioDias = dias })
			e.llega(ana, "hola")
			e.espera(t, 1)
			// un día antes del umbral: no
			e.rel.avanza(time.Duration(dias-1) * 24 * time.Hour)
			e.llega(ana, "sigo aquí")
			e.nada(t, 1)
			// pasa el silencio completo desde su último mensaje
			e.rel.avanza(time.Duration(dias)*24*time.Hour + time.Hour)
			e.llega(ana, "regresé")
			e.espera(t, 2)
		})
	}
}

func TestAusenciaFueraDeHorarioConRepeticion(t *testing.T) {
	e := nuevo(t)
	e.crea(t, almacen.AutoAusencia, "Fuera de horario: atendemos {horario}.", nil)
	// martes 10:00 en CDMX: dentro de horario, no se manda
	e.llega(ana, "hola")
	e.nada(t, 0)
	// martes 20:00 (02:00 UTC del miércoles): fuera de horario
	e.rel.avanza(10 * time.Hour)
	e.llega(ana, "¿hay alguien?")
	l := e.espera(t, 1)
	if !strings.Contains(l[0].sal.Texto, "de lunes a viernes de 09:00 a 13:00 y de 15:00 a 18:00") || l[0].etiqueta != "ausencia" {
		t.Fatalf("%q", l[0].sal.Texto)
	}
	// una hora después, otro mensaje: una vez cada 12 horas
	e.rel.avanza(time.Hour)
	e.llega(ana, "¿?")
	e.nada(t, 1)
	// 13 horas después sigue siendo fuera de horario (miércoles 09:00 CDMX → ya es horario; usamos sábado)
	e.rel.avanza(4*24*time.Hour + time.Hour)
	e.llega(ana, "sábado")
	e.espera(t, 2)
}

func TestAusenciaVacacionesYFechaEspecial(t *testing.T) {
	e := nuevo(t)
	e.crea(t, almacen.AutoAusencia, "Vacaciones", func(c *Config) {
		c.Cuando = Cuando{Vacaciones: []Vacacion{{Desde: "2026-11-10", Hasta: "2026-11-12"}}}
	})
	e.llega(ana, "hola") // 10 de noviembre 10:00 en CDMX: estamos de vacaciones
	e.espera(t, 1)
	e2 := nuevo(t)
	e2.crea(t, almacen.AutoAusencia, "Feliz Navidad", func(c *Config) { c.Cuando = Cuando{Fechas: []FechaAnual{{Dia: 25, Mes: 12}}} })
	e2.llega(ana, "hola")
	e2.nada(t, 0)
	e2.rel.avanza(45 * 24 * time.Hour) // 25 de diciembre
	e2.llega(ben, "hola")
	e2.espera(t, 1)
}

func TestRecibidoUnaVezPorConversacion(t *testing.T) {
	e := nuevo(t)
	e.crea(t, almacen.AutoRecibido, "Recibí tu mensaje", nil)
	e.llega(ana, "uno")
	e.espera(t, 1)
	e.rel.avanza(time.Hour)
	e.llega(ana, "dos")
	e.nada(t, 1)
	e.rel.avanza(7 * time.Hour) // pasó más de la repetición (6 h): es otra conversación
	e.llega(ana, "tres")
	e.espera(t, 2)
}

func TestRecibidoTambienContestaAUnaFoto(t *testing.T) {
	e := nuevo(t)
	e.crea(t, almacen.AutoRecibido, "Recibí tu mensaje", nil)
	e.seq++
	foto := motor.Mensaje{Chat: ana, ID: "FOTO1", Remitente: ana, EsTexto: false, TS: e.rel.ahora()}
	e.m.Entrar(foto)
	l := e.espera(t, 1)
	if l[0].sal.Texto != "Recibí tu mensaje" {
		t.Fatal(textos(l))
	}
}

func TestAQuienDesconocidosContactosYListas(t *testing.T) {
	e := nuevo(t)
	e.conocido[ben] = true
	e.crea(t, almacen.AutoBienvenida, "Para desconocidos", func(c *Config) { c.Quien.Personas = "desconocidos" })
	e.llega(ben, "hola") // contacto: no
	e.nada(t, 0)
	e.llega(ana, "hola") // desconocida: sí
	e.espera(t, 1)

	e2 := nuevo(t)
	e2.conocido[ben] = true
	e2.crea(t, almacen.AutoBienvenida, "Para contactos", func(c *Config) { c.Quien.Personas = "contactos" })
	e2.llega(ana, "hola")
	e2.nada(t, 0)
	e2.llega(ben, "hola")
	e2.espera(t, 1)

	e3 := nuevo(t)
	e3.crea(t, almacen.AutoBienvenida, "Excepto", func(c *Config) { c.Quien.Excepto = []string{ana} })
	e3.llega(ana, "hola")
	e3.nada(t, 0)
	e3.llega(ben, "hola")
	e3.espera(t, 1)

	e4 := nuevo(t)
	e4.crea(t, almacen.AutoBienvenida, "Solo lista", func(c *Config) { c.Quien.SoloLista = []string{cla} })
	e4.llega(ana, "hola")
	e4.nada(t, 0)
	e4.llega(cla, "hola")
	e4.espera(t, 1)
}

func TestGruposSiONo(t *testing.T) {
	grupo := "120363000000000000@g.us"
	en := func(e *ent) {
		e.seq++
		m := motor.Mensaje{Chat: grupo, ID: fmt.Sprint("G", e.seq), Remitente: ana, RemitenteWA: ana, Texto: "hola", EsTexto: true, EsGrupo: true, Mencionado: true, TS: e.rel.ahora()}
		_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: m.ID, ChatJID: grupo, BotID: 1, RemitenteJID: ana, Tipo: "texto", Texto: "hola", TS: m.TS.Unix()})
		e.m.Entrar(m)
	}
	e := nuevo(t)
	e.crea(t, almacen.AutoRecibido, "Recibido", nil) // por defecto solo privados
	en(e)
	e.nada(t, 0)
	e2 := nuevo(t)
	e2.crea(t, almacen.AutoRecibido, "Recibido", func(c *Config) { c.Quien.Grupos = true })
	en(e2)
	l := e2.espera(t, 1)
	if l[0].chat != grupo {
		t.Fatal(l[0])
	}
}

func TestPrioridadMenuReglaYAutomaticos(t *testing.T) {
	e := nuevo(t)
	e.crea(t, almacen.AutoBienvenida, "Bienvenida", nil)
	e.crea(t, almacen.AutoRecibido, "Recibido", nil)
	// una regla con palabra clave
	regla := &flujo.Def{V: 1, Nombre: "Precios", Inicio: "r", Pasos: []flujo.Paso{{ID: "r", Tipo: flujo.PasoRespuesta, Texto: "Los precios están en la web"}},
		Disparadores: flujo.Disparadores{Palabras: []string{"precios"}}, Alcance: flujo.Alcance{Privados: true}}
	regla.Normalizar()
	if _, err := e.alm.CrearMenu(1, regla.JSON(), true, false); err != nil {
		t.Fatal(err)
	}
	// la regla gana a la bienvenida (primera vez de Ana) y al recibido
	e.llega(ana, "precios")
	l := e.espera(t, 1)
	if l[0].sal.Texto != "Los precios están en la web" || l[0].etiqueta != "regla" {
		t.Fatalf("la regla manda: %+v", l[0])
	}
	e.nada(t, 1)
	// un menú en curso también gana a los automáticos
	menu := &flujo.Def{V: 1, Nombre: "Menú", Inicio: "m", Pasos: []flujo.Paso{
		{ID: "m", Tipo: flujo.PasoMenu, Titulo: "Elige", Opciones: []flujo.Opcion{{Texto: "Horario", Destino: "h"}, {Texto: "Dirección", Destino: "d"}}},
		{ID: "h", Tipo: flujo.PasoRespuesta, Texto: "Abrimos de 9 a 6"}, {ID: "d", Tipo: flujo.PasoRespuesta, Texto: "Calle 1"}},
		Disparadores: flujo.Disparadores{Comando: ".menu"}, Alcance: flujo.Alcance{Privados: true}}
	menu.Normalizar()
	if _, err := e.alm.CrearMenu(1, menu.JSON(), true, false); err != nil {
		t.Fatal(err)
	}
	e.rel.avanza(time.Minute)
	e.llega(ben, ".menu")
	l = e.espera(t, 2)
	if l[1].etiqueta != "menu" || !strings.Contains(l[1].sal.Texto, "Elige") {
		t.Fatalf("menú: %+v", l[1])
	}
	e.rel.avanza(time.Minute)
	e.llega(ben, "1") // sigue dentro del menú: no se mete ningún automático
	l = e.espera(t, 3)
	if l[2].sal.Texto != "Abrimos de 9 a 6" {
		t.Fatalf("%+v", l[2])
	}
	e.nada(t, 3)
	// sin regla ni menú, un mensaje de una tercera persona recibe la bienvenida
	e.llega(cla, "hola")
	l = e.espera(t, 4)
	if l[3].etiqueta != "bienvenida" {
		t.Fatalf("%+v", l[3])
	}
}

func TestCierreAlTerminarUnMenuYNoEnUnComandoSimple(t *testing.T) {
	e := nuevo(t)
	e.crea(t, almacen.AutoCierre, "Gracias por contactarnos", nil)
	menu := &flujo.Def{V: 1, Nombre: "Menú", Inicio: "m", Pasos: []flujo.Paso{
		{ID: "m", Tipo: flujo.PasoMenu, Titulo: "Elige", Opciones: []flujo.Opcion{{Texto: "Horario", Destino: "h"}}},
		{ID: "h", Tipo: flujo.PasoRespuesta, Texto: "Abrimos de 9 a 6"}},
		Disparadores: flujo.Disparadores{Comando: ".menu"}, Alcance: flujo.Alcance{Privados: true}}
	menu.Normalizar()
	if _, err := e.alm.CrearMenu(1, menu.JSON(), true, false); err != nil {
		t.Fatal(err)
	}
	// una regla de una sola respuesta no cuenta como menú que termina
	regla := &flujo.Def{V: 1, Nombre: "Hola", Inicio: "r", Pasos: []flujo.Paso{{ID: "r", Tipo: flujo.PasoRespuesta, Texto: "Hola!"}},
		Disparadores: flujo.Disparadores{Palabras: []string{"saludo"}}, Alcance: flujo.Alcance{Privados: true}}
	regla.Normalizar()
	_, _ = e.alm.CrearMenu(1, regla.JSON(), true, false)
	e.llega(cla, "saludo")
	e.espera(t, 1)
	e.nada(t, 1)

	e.llega(ana, ".menu")
	e.espera(t, 2)
	e.rel.avanza(time.Minute)
	e.llega(ana, "1")
	l := e.espera(t, 4)
	if l[2].sal.Texto != "Abrimos de 9 a 6" || l[3].sal.Texto != "Gracias por contactarnos" || l[3].etiqueta != "cierre" {
		t.Fatalf("cierre tras el menú: %s", textos(l))
	}
}

func TestSiContesteYoSeCancela(t *testing.T) {
	e := nuevo(t)
	id := e.crea(t, almacen.AutoRecibido, "Recibido", nil)
	m := e.llega(ana, "hola")
	e.m.Intervino(ana, m.TS) // escribí yo en ese chat antes de que saliera
	e.nada(t, 0)
	if e.alm.UltimaRepeticion(1, id, ana) != 0 {
		t.Fatal("lo cancelado no cuenta como enviado")
	}
	reg, _ := e.alm.ListarAutoRegistro(1, id, 10)
	if len(reg) != 0 {
		t.Fatalf("no hay registro de envío: %+v", reg)
	}
	// y no sale tampoco si contesto mientras espera en la cola de salida: lo prueba el motor (ya contestaste tú)
}

func TestTopePorHoraDegradaASoloTexto(t *testing.T) {
	e := nuevo(t)
	_ = e.alm.FijarAjusteBot(1, "auto_tope_hora", "2")
	foto := almacen.AdjuntoRef{ID: strings.Repeat("a", 24), Tipo: almacen.AdjImagen, Mime: "image/jpeg", Nombre: "foto.jpg"}
	e.crea(t, almacen.AutoRecibido, "Recibido", nil, foto)
	pers := []string{ana, ben, cla, "5215544444444@s.whatsapp.net", "5215555555555@s.whatsapp.net", "5215566666666@s.whatsapp.net", "5215577777777@s.whatsapp.net"}
	for i := 0; i < 2; i++ {
		e.llega(pers[i], "hola")
		e.espera(t, i+1)
	}
	l := e.env.lista()
	if l[0].sal.Medio == nil || l[1].sal.Medio == nil || l[0].sal.Texto != "Recibido" {
		t.Fatalf("dentro del tope salen con archivo: %+v", l)
	}
	// 3.ª a 5.ª: ya se pasó del tope (2): solo texto
	for i := 2; i < 6; i++ {
		e.llega(pers[i], "hola")
		e.espera(t, i+1)
	}
	l = e.env.lista()
	for i := 2; i < 6; i++ {
		if l[i].sal.Medio != nil || l[i].sal.Texto != "Recibido" {
			t.Fatalf("pasado el tope solo texto, sin archivos: #%d %+v", i, l[i])
		}
	}
	// 3 veces el tope (6): ya no se manda nada
	e.llega(pers[6], "hola")
	e.nada(t, 6)
	reg, _ := e.alm.ListarAutoRegistro(1, 0, 20)
	var st, tope int
	for _, r := range reg {
		switch r.Resultado {
		case almacen.AutoSoloTexto:
			st++
		case almacen.AutoTopeDuro:
			tope++
		}
	}
	if st != 4 || tope != 1 {
		t.Fatalf("registro: solo texto %d (esperaba 4), tope %d (esperaba 1)", st, tope)
	}
	// pasada una hora el contador se libera
	e.rel.avanza(61 * time.Minute)
	e.llega(pers[6], "hola otra vez")
	l = e.espera(t, 7)
	if l[6].sal.Medio == nil {
		t.Fatal("tras una hora vuelve a salir con archivo")
	}
}

func TestAdjuntosConPausaDeUnoATresSegundos(t *testing.T) {
	e := nuevo(t)
	mk := func(c string, tipo string) almacen.AdjuntoRef {
		return almacen.AdjuntoRef{ID: strings.Repeat(c, 24), Tipo: tipo, Mime: "x/y", Nombre: "n." + tipo}
	}
	e.crea(t, almacen.AutoRecibido, "Mira esto", nil, mk("a", "imagen"), mk("b", "documento"), mk("c", "video"))
	e.llega(ana, "hola")
	l := e.espera(t, 3)
	if l[0].sal.Medio == nil || l[0].sal.Texto != "Mira esto" || l[0].sal.Continua {
		t.Fatalf("el texto va de pie del primero: %+v", l[0].sal)
	}
	if !l[1].sal.Continua || !l[2].sal.Continua || l[1].sal.Texto != "" {
		t.Fatalf("segundo y tercero siguen al primero: %+v %+v", l[1].sal, l[2].sal)
	}
	// las dos pausas entre archivos están entre 1 y 3 segundos
	n := 0
	for _, d := range e.rel.pausas() {
		if d >= time.Second && d <= 3*time.Second {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("esperaba 2 pausas de 1 a 3 s, hay %d: %v", n, e.rel.pausas())
	}
	reg, _ := e.alm.ListarAutoRegistro(1, 0, 5)
	if len(reg) != 1 || reg[0].AdjOK != 3 || reg[0].Resultado != almacen.AutoOK {
		t.Fatalf("registro: %+v", reg)
	}
}

func TestNotaDeVozLlevaElTextoAntes(t *testing.T) {
	e := nuevo(t)
	voz := almacen.AdjuntoRef{ID: strings.Repeat("d", 24), Tipo: almacen.AdjAudio, Mime: "audio/ogg; codecs=opus"}
	e.crea(t, almacen.AutoRecibido, "Escucha", nil, voz)
	e.llega(ana, "hola")
	l := e.espera(t, 2)
	if l[0].sal.Texto != "Escucha" || l[0].sal.Medio != nil || l[1].sal.Medio == nil || l[1].sal.Medio.Tipo != "audio" {
		t.Fatalf("un audio no lleva pie: texto aparte y luego la nota: %+v", l)
	}
}

func TestSiUnArchivoFallaSaleSoloElTextoYSeAnota(t *testing.T) {
	e := nuevo(t)
	a, b := strings.Repeat("a", 24), strings.Repeat("b", 24)
	e.env.fallaMedio[a], e.env.fallaMedio[b] = true, true
	e.crea(t, almacen.AutoRecibido, "Con foto", nil,
		almacen.AdjuntoRef{ID: a, Tipo: almacen.AdjImagen}, almacen.AdjuntoRef{ID: b, Tipo: almacen.AdjDocumento, Nombre: "x.pdf"})
	e.llega(ana, "hola")
	l := e.espera(t, 1)
	e.nada(t, 1) // el segundo archivo (sin texto) se omite, sin frenar nada
	if l[0].sal.Texto != "Con foto" || l[0].sal.Medio != nil {
		t.Fatalf("sale solo el texto: %+v", l[0].sal)
	}
	reg, _ := e.alm.ListarAutoRegistro(1, 0, 5)
	if len(reg) != 1 || reg[0].Resultado != almacen.AutoParcial || reg[0].AdjFallo != 2 || reg[0].Motivo != "archivo_ilegible" {
		t.Fatalf("registro sin textos: %+v", reg)
	}
	if e.m.Frenado() {
		t.Fatal("un archivo omitido no cuenta como fallo de envío")
	}
}

func TestInterruptoresYOrden(t *testing.T) {
	e := nuevo(t)
	id1 := e.crea(t, almacen.AutoRecibido, "Primero", nil)
	id2 := e.crea(t, almacen.AutoRecibido, "Segundo", nil)
	e.llega(ana, "uno")
	if l := e.espera(t, 1); l[0].sal.Texto != "Primero" {
		t.Fatal(textos(l))
	}
	_ = e.alm.FijarAutoActivo(1, id1, false)
	e.llega(ben, "uno")
	if l := e.espera(t, 2); l[1].sal.Texto != "Segundo" {
		t.Fatalf("apagado el primero responde el segundo: %s", textos(l))
	}
	_ = e.alm.ReordenarAutos(1, []int64{id2, id1})
	_ = e.alm.FijarAutoActivo(1, id1, true)
	e.llega(cla, "uno")
	if l := e.espera(t, 3); l[2].sal.Texto != "Segundo" {
		t.Fatalf("con otro orden gana el segundo: %s", textos(l))
	}
	// el interruptor general apaga los cuatro tipos
	_ = e.alm.FijarAjusteBot(1, "auto_activo", "0")
	e.llega("5215544444444@s.whatsapp.net", "uno")
	e.nada(t, 3)
}

func TestNombreVacioNoDejaHuecos(t *testing.T) {
	e := nuevo(t)
	d := Nuevo(DecisorConfig{BotID: 1, Alm: e.alm, Nombre: func(string, string) string { return "" }})
	if got := d.Render("Hola {nombre}, bienvenido", Config{}, ana, ana, time.Now()); got != "Hola, bienvenido" {
		t.Fatalf("%q", got)
	}
	if got := d.Render("Hola {nombre}!", Config{}, ana, ana, time.Now()); got != "Hola!" {
		t.Fatalf("%q", got)
	}
	d2 := Nuevo(DecisorConfig{BotID: 1, Alm: e.alm, Nombre: func(string, string) string { return "Luz" }, Zona: func() *time.Location { l, _ := time.LoadLocation("America/Mexico_City"); return l }})
	t0 := time.Date(2026, 11, 10, 16, 5, 0, 0, time.UTC)
	if got := d2.Render("{nombre} {hora} {dia} {fecha} {otra}", Config{}, ana, ana, t0); got != "Luz 10:05 martes 10/11/2026 {otra}" {
		t.Fatalf("%q", got)
	}
	// el estilo Unicode no se toca: las llaves se respetan
	if got := d2.Render("𝗛𝗼𝗹𝗮 {nombre}", Config{}, ana, ana, t0); got != "𝗛𝗼𝗹𝗮 Luz" {
		t.Fatalf("%q", got)
	}
}

func TestProbarNoMandaNiGuardaNada(t *testing.T) {
	e := nuevo(t)
	m := almacen.AutoMensaje{Tipo: almacen.AutoAusencia, Texto: "Hola {nombre}, estamos fuera. {horario}", Activo: true}
	c := ConfigPorDefecto(almacen.AutoAusencia)
	if err := Validar(&m, &c); err != nil {
		t.Fatal(err)
	}
	p := e.dec.Probar(m, c, Escenario{Nombre: "Marta", Cuando: "2026-11-14T11:00"}) // sábado
	if !p.Aplica || !strings.HasPrefix(p.Texto, "Hola Marta, estamos fuera. de lunes a viernes") {
		t.Fatalf("%+v", p)
	}
	p = e.dec.Probar(m, c, Escenario{Nombre: "Marta", Cuando: "2026-11-10T11:00"}) // martes en horario
	if p.Aplica || !strings.Contains(p.Motivo, "Cuándo") || !strings.HasPrefix(p.Texto, "Hola Marta") {
		t.Fatalf("dentro de horario no se manda, pero la vista previa del texto sí se arma: %+v", p)
	}
	b := almacen.AutoMensaje{Tipo: almacen.AutoBienvenida, Texto: "Bienvenida", Activo: true}
	bc := ConfigPorDefecto(almacen.AutoBienvenida)
	if p := e.dec.Probar(b, bc, Escenario{Primera: false}); p.Aplica {
		t.Fatal("la bienvenida es para la primera vez")
	}
	if p := e.dec.Probar(b, bc, Escenario{Primera: true}); !p.Aplica {
		t.Fatal("primera vez")
	}
	if len(e.env.lista()) != 0 {
		t.Fatal("probar no manda nada")
	}
	if r, _ := e.alm.ListarAutoRegistro(1, 0, 5); len(r) != 0 {
		t.Fatal("ni registra")
	}
}

// ---------- atajos ----------

type envAtajo struct {
	mu        sync.Mutex
	ediciones []string
	textos    []string
	medios    []string
	falla     bool // la edición falla
	etiquetas []string
}

func (f *envAtajo) Editar(_ context.Context, chat, id, texto string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.falla {
		return fmt.Errorf("plazo vencido")
	}
	f.ediciones = append(f.ediciones, chat+"|"+id+"|"+texto)
	return nil
}
func (f *envAtajo) EnviarTexto(ctx context.Context, chat, texto string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.textos = append(f.textos, chat+"|"+texto)
	f.etiquetas = append(f.etiquetas, almacen.EtiquetaDe(ctx))
	return "X1", nil
}
func (f *envAtajo) EnviarMedio(_ context.Context, chat string, m motor.Medio) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if m.ID == strings.Repeat("e", 24) {
		return fmt.Errorf("no se pudo leer")
	}
	f.medios = append(f.medios, chat+"|"+m.ID)
	return nil
}

func nuevoConAtajos(t *testing.T) (*ent, *envAtajo, *Atajos) {
	e := nuevo(t)
	ea := &envAtajo{}
	at := NuevosAtajos(AtajosConfig{BotID: 1, Alm: e.alm, Dec: e.dec, Envio: ea, Ahora: e.rel.ahora, Dormir: e.rel.dormir, Aleatorio: func(n int64) int64 { return n / 2 }})
	e.m = motor.Nuevo(motor.Config{BotID: 1, Alm: e.alm, Decisor: e.dec, Enviador: e.env, Ahora: e.rel.ahora, Dormir: e.rel.dormir, Atajos: at,
		Debounce: 10 * time.Millisecond, DebounceMax: 40 * time.Millisecond, EsperaMin: time.Millisecond, EsperaMax: 2 * time.Millisecond, AlAuto: e.dec.AlAuto})
	e.m.Iniciar(context.Background())
	t.Cleanup(e.m.Detener)
	return e, ea, at
}

func (e *ent) mio(chat, texto string, ajusta func(*motor.Mensaje)) motor.Mensaje {
	e.seq++
	m := motor.Mensaje{Chat: chat, ID: fmt.Sprintf("MIO%d", e.seq), Texto: texto, EsTexto: true, EsMio: true, TS: e.rel.ahora()}
	if ajusta != nil {
		ajusta(&m)
	}
	e.m.Entrar(m)
	return m
}

func esperaCond(t *testing.T, cond func() bool) {
	t.Helper()
	lim := time.Now().Add(10 * time.Second)
	for time.Now().Before(lim) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("no se cumplió a tiempo")
}

func (f *envAtajo) cuenta() (int, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.ediciones), len(f.textos), len(f.medios)
}

func TestAtajoDesdeElTelefonoEditaMiMensaje(t *testing.T) {
	e, ea, _ := nuevoConAtajos(t)
	m := &almacen.AutoMensaje{Tipo: almacen.AutoAtajo, Nombre: "Gracias", Texto: "¡Gracias {nombre}!", Activo: true}
	cc := Config{}
	if err := Validar(m, &cc); err != nil {
		t.Fatal(err)
	}
	if _, err := e.alm.CrearAuto(1, m); err != nil {
		t.Fatal(err)
	}
	msg := e.mio(ana, "/gracias", nil)
	esperaCond(t, func() bool { a, _, _ := ea.cuenta(); return a == 1 })
	if ea.ediciones[0] != ana+"|"+msg.ID+"|¡Gracias Ana!" {
		t.Fatalf("%v", ea.ediciones)
	}
	if _, tx, _ := ea.cuenta(); tx != 0 {
		t.Fatal("no se manda aparte si la edición funcionó")
	}
	// registro del atajo (cuenta en el registro pero no en el tope por hora)
	reg, _ := e.alm.ListarAutoRegistro(1, 0, 5)
	if len(reg) != 1 || reg[0].Tipo != "atajo" || e.alm.ContarAutoEnvios(1, e.rel.ahora().Add(-time.Hour)) != 0 {
		t.Fatalf("%+v", reg)
	}
}

func creaAtajo(t *testing.T, e *ent, nombre, texto string, adj ...almacen.AdjuntoRef) {
	t.Helper()
	m := &almacen.AutoMensaje{Tipo: almacen.AutoAtajo, Nombre: nombre, Texto: texto, Activo: true, Adjuntos: adj}
	cc := Config{}
	if err := Validar(m, &cc); err != nil {
		t.Fatal(err)
	}
	if _, err := e.alm.CrearAuto(1, m); err != nil {
		t.Fatal(err)
	}
}

func TestAtajoSoloCoincidenciaExactaYSegura(t *testing.T) {
	e, ea, _ := nuevoConAtajos(t)
	creaAtajo(t, e, "gracias", "Gracias")
	e.mio(ana, "/gracias por todo", nil)                                  // no es exacto
	e.mio(ana, "gracias", nil)                                            // sin barra
	e.mio(ana, "/noexiste", nil)                                          // no existe
	e.mio(ana, "//gracias", nil)                                          // doble barra
	e.mio(ana, "/gracias", func(m *motor.Mensaje) { m.EsEdicion = true }) // una edición
	e.mio(ana, "/gracias", func(m *motor.Mensaje) { m.Antiguo = true })   // del historial
	e.mio(ana, "/gracias", func(m *motor.Mensaje) { m.EsChatPropio = true })
	e.mio(ana, "/gracias", func(m *motor.Mensaje) { m.TS = e.rel.ahora().Add(-time.Hour) }) // vieja
	time.Sleep(250 * time.Millisecond)
	if a, b, c := ea.cuenta(); a+b+c != 0 {
		t.Fatalf("nada de eso es un atajo: %d %d %d", a, b, c)
	}
	// lo que escribe otra persona NUNCA activa un atajo
	e.llega(ben, "/gracias")
	time.Sleep(250 * time.Millisecond)
	if a, b, c := ea.cuenta(); a+b+c != 0 {
		t.Fatal("un cliente no puede activar mis atajos")
	}
	// apagado el interruptor de atajos no hace nada
	_ = e.alm.FijarAjusteBot(1, "atajos_activo", "0")
	e.mio(ana, "/gracias", nil)
	time.Sleep(250 * time.Millisecond)
	if a, b, c := ea.cuenta(); a+b+c != 0 {
		t.Fatal("atajos apagados")
	}
	_ = e.alm.FijarAjusteBot(1, "atajos_activo", "1")
	e.rel.avanza(10 * time.Second)
	e.mio(ana, "/GRACIAS", nil) // mayúsculas valen
	esperaCond(t, func() bool { a, _, _ := ea.cuenta(); return a == 1 })
}

func TestAtajoModoEnviarYRespaldoSiNoSePuedeEditar(t *testing.T) {
	e, ea, _ := nuevoConAtajos(t)
	creaAtajo(t, e, "hola", "Hola, ¿en qué te ayudo?")
	_ = e.alm.FijarAjusteBot(1, "atajo_modo", "enviar")
	e.mio(ana, "/hola", nil)
	esperaCond(t, func() bool { _, b, _ := ea.cuenta(); return b == 1 })
	if a, _, _ := ea.cuenta(); a != 0 {
		t.Fatal("en modo enviar no se edita")
	}
	if ea.textos[0] != ana+"|Hola, ¿en qué te ayudo?" || ea.etiquetas[0] != "atajo" {
		t.Fatalf("%v %v", ea.textos, ea.etiquetas)
	}
	// modo editar, pero WhatsApp ya no deja editar: respaldo, se manda aparte
	_ = e.alm.FijarAjusteBot(1, "atajo_modo", "editar")
	ea.falla = true
	e.rel.avanza(10 * time.Second)
	e.mio(ben, "/hola", nil)
	esperaCond(t, func() bool { _, b, _ := ea.cuenta(); return b == 2 })
}

func TestAtajoConArchivosConPausaYFallo(t *testing.T) {
	e, ea, _ := nuevoConAtajos(t)
	bueno, malo := strings.Repeat("a", 24), strings.Repeat("e", 24)
	creaAtajo(t, e, "catalogo", "Te paso el catálogo",
		almacen.AdjuntoRef{ID: bueno, Tipo: almacen.AdjDocumento, Nombre: "catalogo.pdf"}, almacen.AdjuntoRef{ID: malo, Tipo: almacen.AdjImagen})
	e.mio(ana, "/catalogo", nil)
	esperaCond(t, func() bool { r, _ := e.alm.ListarAutoRegistro(1, 0, 5); return len(r) == 1 })
	_, _, medios := ea.cuenta()
	if medios != 1 {
		t.Fatalf("salió un archivo y el otro falló: %d", medios)
	}
	reg, _ := e.alm.ListarAutoRegistro(1, 0, 5)
	if reg[0].Resultado != almacen.AutoParcial || reg[0].AdjOK != 1 || reg[0].AdjFallo != 1 {
		t.Fatalf("%+v", reg[0])
	}
	n := 0
	for _, d := range e.rel.pausas() {
		if d >= time.Second && d <= 3*time.Second {
			n++
		}
	}
	if n < 2 {
		t.Fatalf("pausa de 1 a 3 s antes de cada archivo: %v", e.rel.pausas())
	}
}
