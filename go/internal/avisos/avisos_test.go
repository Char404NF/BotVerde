package avisos

import (
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/permisos"
)

const (
	pnAna  = "5215511111111@s.whatsapp.net"
	pnLuis = "5215522222222@s.whatsapp.net"
	grupo  = "grupo@g.us"
)

type entorno struct {
	alm  *almacen.Almacen
	s    *Servicio
	hora time.Time
}

func nuevoEntorno(t *testing.T) *entorno {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 11)
	}
	alm, err := almacen.Abrir(t.TempDir(), k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = alm.Cerrar() })
	_ = alm.FijarZona("America/Mexico_City")
	e := &entorno{alm: alm}
	e.hora = time.Date(2026, 3, 10, 9, 0, 0, 0, alm.Zona())
	e.s = Nuevo(alm, 1)
	e.s.Ahora = func() time.Time { return e.hora }
	_, _ = alm.ActualizarChat(1, pnAna, false, func(c *almacen.Chat) { c.TelE164, c.Publico = "5215511111111", "Ana Pérez" })
	_, _ = alm.ActualizarChat(1, grupo, true, func(c *almacen.Chat) { c.Guardado = "Vecinos" })
	return e
}

func (e *entorno) msg(chat, texto string) Mensaje {
	return Mensaje{Chat: chat, Persona: pnAna, Nombre: "Ana", Texto: texto, EsGrupo: chat == grupo}
}

func TestAlertaPorPalabraNoCopiaElMensaje(t *testing.T) {
	e := nuevoEntorno(t)
	_, _ = e.alm.CrearAlertaPalabra(1, "urgente", nil)
	_, _ = e.alm.CrearAlertaPalabra(1, "mi pedido", nil)
	if id := e.s.AlMensaje(e.msg(pnAna, "Hola, es URGENTE: necesito ayuda con la dirección de mi casa")); id == 0 {
		t.Fatal("debía avisar (sin importar mayúsculas)")
	}
	av, _ := e.alm.AvisosRecientes(1, 5)
	if len(av) != 1 || av[0].Tipo != TipoPalabra || av[0].Chat != pnAna || av[0].Persona != pnAna {
		t.Fatalf("%+v", av)
	}
	if !strings.Contains(av[0].Detalle, "Ana Pérez") || !strings.Contains(av[0].Detalle, "urgente") {
		t.Fatalf("detalle: %q", av[0].Detalle)
	}
	if strings.Contains(av[0].Detalle, "dirección") || strings.Contains(av[0].Detalle, "casa") {
		t.Fatalf("no debe copiar el mensaje: %q", av[0].Detalle)
	}
}

func TestAlertaPalabraCompletaFraseEnChatsYApagada(t *testing.T) {
	e := nuevoEntorno(t)
	_, _ = e.alm.CrearAlertaPalabra(1, "pago", nil)
	_, _ = e.alm.CrearAlertaPalabra(1, "mi pedido", []string{grupo})
	// "pago" no coincide dentro de otra palabra.
	if e.s.AlMensaje(e.msg(pnAna, "pagoda y pagos")) != 0 {
		t.Fatal("solo palabras completas")
	}
	// Frase solo en el grupo elegido.
	e.hora = e.hora.Add(time.Minute)
	if e.s.AlMensaje(e.msg(pnAna, "dónde está mi pedido")) != 0 {
		t.Fatal("esa frase solo vale en el grupo")
	}
	if e.s.AlMensaje(e.msg(grupo, "¿dónde está mi pedido?")) == 0 {
		t.Fatal("en el grupo sí")
	}
	// Sin acentos ni signos: "PAGO." coincide con "pago".
	e.hora = e.hora.Add(time.Minute)
	if e.s.AlMensaje(e.msg(pnAna, "ya hice el PAGO.")) == 0 {
		t.Fatal("ignora puntuación y mayúsculas")
	}
	// Lo mío, lo viejo y las ediciones no avisan.
	e.hora = e.hora.Add(time.Minute)
	m := e.msg(pnAna, "pago")
	m.EsMio = true
	if e.s.AlMensaje(m) != 0 {
		t.Fatal("lo mío no avisa")
	}
	m.EsMio, m.Antiguo = false, true
	if e.s.AlMensaje(m) != 0 {
		t.Fatal("lo de la sincronización no avisa")
	}
	// Interruptor general apagado.
	_ = e.alm.FijarAjusteBot(1, "alertas_activas", "0")
	if e.s.AlMensaje(e.msg(pnAna, "pago")) != 0 {
		t.Fatal("alertas apagadas")
	}
}

func TestAlertaFragmentoEsOpcionalYUnaRafagaAvisaUnaVez(t *testing.T) {
	e := nuevoEntorno(t)
	_, _ = e.alm.CrearAlertaPalabra(1, "factura", nil)
	if e.s.AlMensaje(e.msg(pnAna, "necesito mi factura de marzo por favor")) == 0 {
		t.Fatal("avisa")
	}
	// Ráfaga: otros dos mensajes seguidos del mismo chat no vuelven a avisar.
	if e.s.AlMensaje(e.msg(pnAna, "la factura")) != 0 || e.s.AlMensaje(e.msg(pnAna, "factura factura")) != 0 {
		t.Fatal("una ráfaga avisa una sola vez")
	}
	e.hora = e.hora.Add(time.Minute)
	_ = e.alm.FijarAjusteBot(1, "alerta_fragmento", "1")
	if id := e.s.AlMensaje(e.msg(pnAna, "necesito mi factura de marzo por favor")); id == 0 {
		t.Fatal("avisa")
	}
	av, _ := e.alm.AvisosRecientes(1, 1)
	if !strings.Contains(av[0].Detalle, "mi factura de marzo") {
		t.Fatalf("con fragmento: %q", av[0].Detalle)
	}
}

func TestResumenDiarioSeCuentaDeLaBaseYSoloUnaVezAlDia(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.alm.FijarAjusteBot(1, "resumen_activo", "1")
	_ = e.alm.FijarAjusteBot(1, "resumen_hora", "20:00")
	ts := e.hora.Unix()
	for i, id := range []string{"a", "b", "c"} {
		_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: id, ChatJID: pnAna, BotID: 1, RemitenteJID: pnAna, Tipo: almacen.TipoTexto, Texto: "x", TS: ts + int64(i)})
	}
	_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: "d", ChatJID: grupo, BotID: 1, RemitenteJID: pnLuis, Tipo: almacen.TipoTexto, Texto: "x", TS: ts + 5})
	_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: "mio", ChatJID: pnAna, BotID: 1, EsMio: true, Tipo: almacen.TipoTexto, Texto: "x", TS: ts + 6})
	_ = e.alm.FijarEstadoTrabajo(1, pnAna, almacen.TrabajoPendiente)
	_ = e.alm.DecidirSolicitud(1, pnLuis, almacen.SolPendiente, e.hora)
	_, _, _ = e.alm.AgregarAdvertencia(1, pnLuis, "", "", "yo", e.hora)
	// A las 9:00 todavía no toca: la próxima alarma es hoy a las 20:00.
	if r := e.s.Disparar(); len(r.Avisos) != 0 || time.Unix(r.Siguiente, 0).In(e.alm.Zona()).Format("15:04") != "20:00" || time.Unix(r.Siguiente, 0).Day() != 10 {
		t.Fatalf("%+v", r)
	}
	e.hora = time.Date(2026, 3, 10, 20, 0, 5, 0, e.alm.Zona())
	r := e.s.Disparar()
	if len(r.Avisos) != 1 {
		t.Fatalf("%+v", r)
	}
	a, _ := e.alm.ObtenerAviso(1, r.Avisos[0])
	for _, frag := range []string{"4 mensajes", "2 chat", "Chats pendientes: 1", "Solicitudes por revisar: 1", "Advertencias de hoy: 1"} {
		if !strings.Contains(a.Detalle, frag) {
			t.Fatalf("falta %q en:\n%s", frag, a.Detalle)
		}
	}
	if a.Tipo != TipoResumen || a.Titulo != "Resumen del día" {
		t.Fatalf("%+v", a)
	}
	// Se puede disparar de más: no repite. La próxima es mañana a las 20:00.
	r = e.s.Disparar()
	if len(r.Avisos) != 0 || time.Unix(r.Siguiente, 0).In(e.alm.Zona()).Format("2006-01-02 15:04") != "2026-03-11 20:00" {
		t.Fatalf("%+v %v", r, time.Unix(r.Siguiente, 0).In(e.alm.Zona()))
	}
	// Si el teléfono estuvo apagado a las 20:00 y se prende a las 21:30, el resumen sale entonces.
	e.hora = time.Date(2026, 3, 11, 21, 30, 0, 0, e.alm.Zona())
	if r = e.s.Disparar(); len(r.Avisos) != 1 {
		t.Fatalf("tarde: %+v", r)
	}
	// Apagado: sin alarma.
	_ = e.alm.FijarAjusteBot(1, "resumen_activo", "0")
	if r = e.s.Disparar(); r.Siguiente != 0 {
		t.Fatalf("apagado: %+v", r)
	}
}

func TestRecordatoriosSeDisparanYSeRepiten(t *testing.T) {
	e := nuevoEntorno(t)
	id1, _ := e.alm.CrearRecordatorio(1, "llamar a Ana", e.hora.Add(10*time.Minute).Unix(), "", e.hora)
	_, _ = e.alm.CrearRecordatorio(1, "pastilla", e.hora.Add(20*time.Minute).Unix(), almacen.RepiteDiaria, e.hora)
	if e.s.Siguiente() != e.hora.Add(10*time.Minute).Unix() {
		t.Fatal("la siguiente alarma es el recordatorio más próximo")
	}
	if r := e.s.Disparar(); len(r.Avisos) != 0 {
		t.Fatal("aún no")
	}
	e.hora = e.hora.Add(15 * time.Minute)
	r := e.s.Disparar()
	if len(r.Avisos) != 1 || r.Siguiente != e.hora.Add(5*time.Minute).Unix() {
		t.Fatalf("%+v", r)
	}
	a, _ := e.alm.ObtenerAviso(1, r.Avisos[0])
	if a.Tipo != TipoRecordatorio || a.Detalle != "llamar a Ana" {
		t.Fatalf("%+v", a)
	}
	// Disparar otra vez no repite el mismo.
	if r = e.s.Disparar(); len(r.Avisos) != 0 {
		t.Fatal("no se repite")
	}
	// La diaria suena y queda para mañana.
	e.hora = e.hora.Add(10 * time.Minute)
	r = e.s.Disparar()
	if len(r.Avisos) != 1 || time.Unix(r.Siguiente, 0).In(e.alm.Zona()).Format("2006-01-02 15:04") != "2026-03-11 09:20" {
		t.Fatalf("diaria: %+v %v", r, time.Unix(r.Siguiente, 0).In(e.alm.Zona()))
	}
	l, _ := e.alm.ListarRecordatorios(1, true)
	for _, x := range l {
		if x.ID == id1 && x.Estado != almacen.RecHecho {
			t.Fatal("el de una sola vez queda hecho")
		}
	}
}

func horaM(h, m int) time.Time { return time.Date(2026, 3, 10, h, m, 0, 0, time.UTC) } // martes

func TestParsearCuando(t *testing.T) {
	ahora := horaM(9, 0)
	casos := []struct {
		in, texto, rep, cuando string
	}{
		{"en 30m llamar a Ana", "llamar a Ana", "", "2026-03-10 09:30"},
		{"en 2 horas sacar el pan", "sacar el pan", "", "2026-03-10 11:00"},
		{"en 3d pagar luz", "pagar luz", "", "2026-03-13 09:00"},
		{"mañana 8:30 junta", "junta", "", "2026-03-11 08:30"},
		{"manana junta", "junta", "", "2026-03-11 09:00"},
		{"hoy 18:00 comprar pan", "comprar pan", "", "2026-03-10 18:00"},
		{"18:00 cena", "cena", "", "2026-03-10 18:00"},
		{"8:00 cena temprano", "cena temprano", "", "2026-03-11 08:00"}, // ya pasó hoy → mañana
		{"9pm película", "película", "", "2026-03-10 21:00"},
		{"diario 8:00 tomar agua", "tomar agua", "diaria", "2026-03-11 08:00"},
		{"semanal lunes 10:00 reporte", "reporte", "semanal", "2026-03-16 10:00"},
		{"viernes 17:30 salida", "salida", "", "2026-03-13 17:30"},
		{"martes 8:00 hoy ya pasó", "hoy ya pasó", "", "2026-03-17 08:00"},
		{"25/12 10:00 regalo", "regalo", "", "2026-12-25 10:00"},
		{"1/3 9:00 aniversario", "aniversario", "", "2027-03-01 09:00"},
		{"pasado mañana 7:00 vuelo", "vuelo", "", "2026-03-12 07:00"},
	}
	for _, c := range casos {
		ts, rep, texto, err := ParsearCuando(strings.Fields(c.in), ahora, time.UTC)
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if got := time.Unix(ts, 0).UTC().Format("2006-01-02 15:04"); got != c.cuando || rep != c.rep || texto != c.texto {
			t.Fatalf("%q → %s %q %q (esperaba %s %q %q)", c.in, got, rep, texto, c.cuando, c.rep, c.texto)
		}
	}
	for _, mal := range []string{"", "hola mundo", "en 30m", "mañana 9:00", "en x horas algo", "en 5 años algo", "25/13 10:00 algo", "hoy 7:00 algo"} {
		if _, _, _, err := ParsearCuando(strings.Fields(mal), ahora, time.UTC); err == nil {
			t.Fatalf("%q debía fallar", mal)
		}
	}
}

func TestRecordarSoloElDuenoDesdeMensajesGuardados(t *testing.T) {
	e := nuevoEntorno(t)
	p, _ := permisos.Nuevo(e.alm, 1)
	d := comandos.Nuevo(e.alm, 1, p)
	e.s.RegistrarComandos(d)
	// Otra persona (aunque tenga rango) no puede.
	id, _ := p.GuardarRol(almacen.Rol{Nombre: "Jefe", Casillas: map[string]almacen.Alcance{"rangos.asignar": {}}, LimiteMin: 50})
	_ = e.alm.AsignarRol(1, pnLuis, id, 0, "yo")
	r := d.Ejecutar(comandos.Entrada{Persona: pnLuis, Chat: pnLuis, Texto: "!recordar mañana 9:00 algo"})
	if !strings.Contains(r.Texto, "solo lo usa el dueño") {
		t.Fatalf("%q", r.Texto)
	}
	if ayuda := d.Ejecutar(comandos.Entrada{Persona: pnLuis, Chat: pnLuis, Texto: "!ayuda"}).Texto; strings.Contains(ayuda, "!recordar") {
		t.Fatal("!recordar no sale en la ayuda de otros")
	}
	r = d.Ejecutar(comandos.Entrada{Persona: almacen.PersonaDueno, Chat: "yo", Texto: "!recordar mañana 9:00 llamar al banco", Dueno: true})
	if !strings.Contains(r.Texto, "Listo: te aviso el 11/03 09:00") {
		t.Fatalf("%q", r.Texto)
	}
	l, _ := e.alm.ListarRecordatorios(1, false)
	if len(l) != 1 || l[0].Texto != "llamar al banco" {
		t.Fatalf("%+v", l)
	}
	if r = d.Ejecutar(comandos.Entrada{Persona: almacen.PersonaDueno, Chat: "yo", Texto: "!recordar lista", Dueno: true}); !strings.Contains(r.Texto, "llamar al banco") {
		t.Fatalf("%q", r.Texto)
	}
	if r = d.Ejecutar(comandos.Entrada{Persona: almacen.PersonaDueno, Chat: "yo", Texto: "!recordar nada de nada", Dueno: true}); !strings.Contains(r.Texto, "No entendí cuándo") {
		t.Fatalf("%q", r.Texto)
	}
	// Con el texto del recordatorio nunca en el registro de acciones.
	acs, _ := e.alm.ListarAcciones(1, 20, 0)
	for _, a := range acs {
		if strings.Contains(a.Detalle, "banco") {
			t.Fatalf("texto en el registro: %+v", a)
		}
	}
}
