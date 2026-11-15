package almacen

import (
	"reflect"
	"testing"
	"time"
)

func grupoConGente(t *testing.T, a *Almacen) string {
	chat := "120363111@g.us"
	chatBasico(t, a, chat, "", "Equipo", true)
	ahora := time.Now().Unix() - 1000
	ms := []*Mensaje{}
	for i, q := range []struct{ jid, nombre string }{{"5215511@s.whatsapp.net", "Ana"}, {"5215522@s.whatsapp.net", "Beto"}, {"5215533@s.whatsapp.net", ""}} {
		ms = append(ms, &Mensaje{ID: "P" + string(rune('A'+i)), ChatJID: chat, BotID: 1, RemitenteJID: q.jid, RemitenteNombre: q.nombre, Tipo: TipoTexto, Texto: "hola", TS: ahora + int64(i)})
	}
	c := msg(chat, "C1", ahora+10, "respondo")
	c.RemitenteJID, c.RemitenteNombre = "5215599@s.whatsapp.net", "Zoe"
	c.CitaID, c.CitaAutor, c.CitaTexto = "PA", "5215511@s.whatsapp.net", "hola"
	c.Menciones = []string{"5215522@s.whatsapp.net", "5215533@s.whatsapp.net", "5215577@s.whatsapp.net"}
	e := msg(chat, "E1", ahora+11, "borrado luego")
	e.RemitenteJID, e.RemitenteNombre = "5215599@s.whatsapp.net", "Zoe"
	ms = append(ms, c, e)
	if _, err := a.GuardarMensajes(ms); err != nil {
		t.Fatal(err)
	}
	if _, err := a.MarcarEliminado(1, chat, "E1", "5215522@s.whatsapp.net", ahora+20); err != nil {
		t.Fatal(err)
	}
	return chat
}

// La ruta por lote da exactamente los mismos nombres que la antigua (una consulta por persona).
func TestNombresPorLoteIgualQueLaRutaAntigua(t *testing.T) {
	a := nuevo(t)
	chat := grupoConGente(t, a)
	defer CargaAntigua(false)
	CargaAntigua(true)
	antes, err := a.ListarMensajes(1, chat, 0, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	CargaAntigua(false)
	despues, err := a.ListarMensajes(1, chat, 0, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(antes, despues) {
		t.Fatalf("difieren:\nantes   %+v\ndespués %+v", antes, despues)
	}
	for _, v := range despues {
		if v.ID == "C1" && (v.CitaAutor != "Ana" || v.Menciones[0].Nombre != "Beto" || v.Menciones[1].Nombre != "+5215533") {
			t.Fatalf("nombres de la cita/menciones: %q %+v", v.CitaAutor, v.Menciones)
		}
		if v.ID == "E1" && v.EliminadoPor != "Beto" {
			t.Fatalf("eliminado por %q", v.EliminadoPor)
		}
	}
}

func nombreDeCita(t *testing.T, a *Almacen, chat string) string {
	t.Helper()
	vs, err := a.MensajesPorID(1, chat, []string{"C1"})
	if err != nil || len(vs) != 1 {
		t.Fatal(err, len(vs))
	}
	return vs[0].CitaAutor
}

// La caché se invalida al cambiar el contacto (nombre guardado, push name) y al llegar un mensaje con nombre nuevo.
func TestCacheDeNombresSeInvalida(t *testing.T) {
	a := nuevo(t)
	chat := grupoConGente(t, a)
	if n := nombreDeCita(t, a, chat); n != "Ana" {
		t.Fatalf("antes %q", n)
	}
	if n := nombreDeCita(t, a, chat); n != "Ana" { // ya en caché
		t.Fatalf("caché %q", n)
	}
	// 1) Un mensaje nuevo trae otro nombre de la misma persona.
	m := msg(chat, "N1", time.Now().Unix(), "ahora me llamo distinto")
	m.RemitenteJID, m.RemitenteNombre = "5215511@s.whatsapp.net", "Ana María"
	if _, err := a.GuardarMensajes([]*Mensaje{m}); err != nil {
		t.Fatal(err)
	}
	if n := nombreDeCita(t, a, chat); n != "Ana María" {
		t.Fatalf("tras mensaje nuevo %q", n)
	}
	// 2) Se guarda el contacto (nombre de agenda / push name): gana el del contacto.
	if _, err := a.ActualizarChat(1, "5215511@s.whatsapp.net", false, func(c *Chat) { c.Guardado = "Ana de Ventas" }); err != nil {
		t.Fatal(err)
	}
	if n := nombreDeCita(t, a, chat); n != "Ana de Ventas" {
		t.Fatalf("tras contacto %q", n)
	}
	if _, err := a.ActualizarChat(1, "5215511@s.whatsapp.net", false, func(c *Chat) { c.Guardado = ""; c.Publico = "Anita" }); err != nil {
		t.Fatal(err)
	}
	if n := nombreDeCita(t, a, chat); n != "Anita" {
		t.Fatalf("tras push name %q", n)
	}
}

func TestMensajesPorIDYEventos(t *testing.T) {
	a := nuevo(t)
	chat := grupoConGente(t, a)
	var evs []string
	a.Cambio = func(e string) { evs = append(evs, e) }
	vs, err := a.MensajesPorID(1, chat, []string{"PA", "C1", "NOEXISTE", "E1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 3 || vs[0].ID != "E1" || vs[1].ID != "C1" || vs[2].ID != "PA" {
		t.Fatalf("orden/contenido %v", vs)
	}
	// Una reacción, un acuse, una edición y un eliminado avisan del mensaje concreto.
	mio := msg(chat, "MIO", time.Now().Unix(), "mío")
	mio.EsMio, mio.RemitenteJID = true, QuienYo
	_, _ = a.GuardarMensajes([]*Mensaje{mio})
	evs = nil
	_ = a.GuardarReaccion(1, chat, "PA", "5215522@s.whatsapp.net", "👍", 1)
	_, _ = a.GuardarAcuse(1, chat, "MIO", "5215522@s.whatsapp.net", AcuseLeido, 2)
	_, _ = a.EditarTexto(1, chat, "C1", "editado")
	_ = a.EstadoDeEnvio(1, chat, "MIO", "fallido")
	_, _ = a.MarcarEliminado(1, chat, "PB", "5215522@s.whatsapp.net", 3)
	for _, id := range []string{"PA", "MIO", "C1", "PB"} {
		ok := false
		for _, e := range evs {
			if e == "msg:"+chat+":"+id {
				ok = true
			}
		}
		if !ok {
			t.Fatalf("falta el evento del mensaje %s en %v", id, evs)
		}
	}
	// Los cambios se ven al pedir solo esos mensajes.
	vs, _ = a.MensajesPorID(1, chat, []string{"PA", "MIO", "C1", "PB"})
	por := map[string]MensajeVista{}
	for _, v := range vs {
		por[v.ID] = v
	}
	if len(por["PA"].Reacciones) != 1 || por["MIO"].EstadoEnvio != "fallido" || !por["C1"].Editado || !por["PB"].Borrado {
		t.Fatalf("no se reflejaron: %+v", por)
	}
}

// Una mención que viene con el LID de alguien se muestra con el nombre que esa persona tiene por su número (no "Participante").
func TestMencionPorLIDUsaElNombreDelNumero(t *testing.T) {
	a := nuevo(t)
	chat := "120363222@g.us"
	chatBasico(t, a, chat, "", "Equipo", true)
	ahora := time.Now().Unix() - 1000
	hola := msg(chat, "H1", ahora, "hola")
	hola.RemitenteJID, hola.RemitenteNombre = "5215588@s.whatsapp.net", "Lupe"
	resp := msg(chat, "H2", ahora+5, "contesto")
	resp.RemitenteJID, resp.RemitenteNombre = "5215599@s.whatsapp.net", "Zoe"
	resp.Menciones = []string{"777000111@lid"}
	if _, err := a.GuardarMensajes([]*Mensaje{hola, resp}); err != nil {
		t.Fatal(err)
	}
	a.AprenderAlias(1, "5215588@s.whatsapp.net", "777000111@lid")
	vs, err := a.MensajesPorID(1, chat, []string{"H2"})
	if err != nil || len(vs) != 1 || len(vs[0].Menciones) != 1 {
		t.Fatal(err, vs)
	}
	if vs[0].Menciones[0].Nombre != "Lupe" {
		t.Fatalf("la mención debía llamarse Lupe y salió %q", vs[0].Menciones[0].Nombre)
	}
}

// Una mención (o cita) de alguien que solo se conoce por su LID no debe mostrarse como «Número no disponible» si se sabe su nombre o su número.
func TestMencionDeLIDMuestraNombreONumeroNoElAvisoDeNoDisponible(t *testing.T) {
	a := nuevo(t)
	g := "120363000000000001@g.us"
	chatBasico(t, a, g, "", "Grupo", true)
	const lid, pn = "171373624348814@lid", "5215500000002@s.whatsapp.net"
	// el LID tiene un «chat» con el aviso guardado como nombre (así lo dejaba la ingesta)
	chatBasico(t, a, lid, "", "", false) // sin nombre ni número: queda «Número no disponible»
	if c, _ := a.ObtenerChat(1, lid); c == nil || c.Mostrado != "Número no disponible" {
		t.Fatalf("el chat del LID debía quedar con el aviso como nombre: %+v", c)
	}
	m := msg(g, "1", time.Now().Unix(), "hola")
	m.Menciones = []string{lid}
	// 1) se conoce su número por el alias: se muestra el número
	a.AprenderAlias(1, pn, lid)
	v := a.vista(m, nil)
	if got := v.Menciones[0].Nombre; got != "+5215500000002" {
		t.Fatalf("debía ser su número: %q", got)
	}
	// 2) además escribió en el grupo con su nombre: se muestra el nombre
	otro := msg(g, "2", time.Now().Unix(), "yo")
	otro.RemitenteJID, otro.RemitenteNombre = lid, "Bot VPS"
	_, _ = a.GuardarMensaje(otro)
	a.nombres.limpiar()
	if got := a.vista(m, nil).Menciones[0].Nombre; got != "Bot VPS" {
		t.Fatalf("debía ser su nombre: %q", got)
	}
	if got := a.nombreParaMostrar(1, g, lid); got != "Bot VPS" {
		t.Fatalf("nombreParaMostrar: %q", got)
	}
}

// Si el alias no se aprendió pero la lista de miembros del grupo sí sabe quién es ese LID, se usa eso.
func TestMencionDeLIDSeResuelvePorLaListaDeMiembros(t *testing.T) {
	a := nuevo(t)
	g := "120363000000000002@g.us"
	chatBasico(t, a, g, "", "Grupo", true)
	const lid, pn = "171373624348814@lid", "5215500000002@s.whatsapp.net"
	_, _ = a.ActualizarMiembros(1, g, []MiembroGrupo{{Persona: pn, JID: lid, Rol: RolGrupoMiembro}}, time.Now())
	m := msg(g, "1", time.Now().Unix(), "hola")
	m.Menciones = []string{lid}
	if got := a.vista(m, nil).Menciones[0].Nombre; got != "+5215500000002" {
		t.Fatalf("debía ser su número: %q", got)
	}
	// y si escribió con su número y nombre (clave canónica), se muestra el nombre
	o := msg(g, "2", time.Now().Unix(), "yo")
	o.RemitenteJID, o.RemitenteNombre = pn, "Bot VPS"
	_, _ = a.GuardarMensaje(o)
	a.nombres.limpiar()
	if got := a.vista(m, nil).Menciones[0].Nombre; got != "Bot VPS" {
		t.Fatalf("debía ser su nombre: %q", got)
	}
}

// El aviso «Número no disponible» guardado como nombre del remitente (así lo dejaba la ingesta) no cuenta como nombre.
func TestElAvisoGuardadoComoNombreDelRemitenteNoSeUsaEnMenciones(t *testing.T) {
	a := nuevo(t)
	g := "120363000000000003@g.us"
	chatBasico(t, a, g, "", "Grupo", true)
	const lid, pn = "171373624348814@lid", "5215500000002@s.whatsapp.net"
	_, _ = a.ActualizarMiembros(1, g, []MiembroGrupo{{Persona: pn, JID: lid, Rol: RolGrupoMiembro}}, time.Now())
	o := msg(g, "1", time.Now().Unix(), "yo")
	o.RemitenteJID, o.RemitenteNombre = lid, "Número no disponible"
	_, _ = a.GuardarMensaje(o)
	m := msg(g, "2", time.Now().Unix(), "hola")
	m.Menciones = []string{lid}
	if got := a.vista(m, nil).Menciones[0].Nombre; got != "+5215500000002" {
		t.Fatalf("no debía usar el aviso como nombre: %q", got)
	}
	if got := a.nombreParaMostrar(1, g, lid); got != "+5215500000002" {
		t.Fatalf("nombreParaMostrar: %q", got)
	}
}
