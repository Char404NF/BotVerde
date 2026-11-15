package personas

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/grupos"
	"botchat/internal/permisos"
)

const (
	pnAna   = "5215511111111@s.whatsapp.net"
	pnLuis  = "5215522222222@s.whatsapp.net"
	pnRosa  = "5215533333333@s.whatsapp.net"
	lidRosa = "77665544332211@lid"
	grupo   = "grupo@g.us"
	propio  = "5215500000001@s.whatsapp.net"
)

// grupoFalso apunta qué se pidió y responde según se configure.
type grupoFalso struct {
	admin    bool
	errAdmin error
	borrados []string
	expulsos []string
	errAcc   error
}

func (g *grupoFalso) EsAdmin(context.Context, string) (bool, error) { return g.admin, g.errAdmin }
func (g *grupoFalso) BorrarMensajeAjeno(_ context.Context, chat, id, autor string) error {
	if g.errAcc != nil {
		return g.errAcc
	}
	g.borrados = append(g.borrados, chat+"|"+id+"|"+autor)
	return nil
}
func (g *grupoFalso) Expulsar(_ context.Context, chat, p string) error {
	if g.errAcc != nil {
		return g.errAcc
	}
	g.expulsos = append(g.expulsos, chat+"|"+p)
	return nil
}

type entorno struct {
	alm  *almacen.Almacen
	perm *permisos.Servicio
	d    *comandos.Despachador
	s    *Servicio
	g    *grupoFalso
	hora time.Time
	cod  []almacen.Confirmacion
	ids  map[string]int64
}

func nuevoEntorno(t *testing.T) *entorno {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 3)
	}
	alm, err := almacen.Abrir(t.TempDir(), k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = alm.Cerrar() })
	p, err := permisos.Nuevo(alm, 1)
	if err != nil {
		t.Fatal(err)
	}
	e := &entorno{alm: alm, perm: p, hora: time.Now(), g: &grupoFalso{}, ids: map[string]int64{}}
	p.Ahora = func() time.Time { return e.hora }
	p.EsPropio = func(x string) bool { return x == propio }
	e.d = comandos.Nuevo(alm, 1, p)
	e.d.Ahora = p.Ahora
	e.d.AlCodigo = func(c almacen.Confirmacion, _ string) { e.cod = append(e.cod, c) }
	e.s = Nuevo(alm, 1, p)
	e.s.Ahora = p.Ahora
	e.s.Grupo = e.g
	e.s.RegistrarComandos(e.d)
	roles, _ := alm.ListarRoles(1)
	for _, r := range roles {
		e.ids[r.Nombre] = r.ID
	}
	return e
}

// rol crea un rango con casillas y lo pone al final (más abajo que los anteriores).
func (e *entorno) rol(nombre string, cas ...string) int64 {
	m := map[string]almacen.Alcance{}
	for _, c := range cas {
		m[c] = almacen.Alcance{}
	}
	id, err := e.perm.GuardarRol(almacen.Rol{Nombre: nombre, Casillas: m, LimiteMin: 100})
	if err != nil {
		panic(err)
	}
	e.ids[nombre] = id
	return id
}

func (e *entorno) da(persona, rol string) {
	if err := e.alm.AsignarRol(1, persona, e.ids[rol], 0, "yo"); err != nil {
		panic(err)
	}
}

func (e *entorno) priv(quien, texto string, men ...string) comandos.Resultado {
	return e.d.Ejecutar(comandos.Entrada{Persona: quien, Chat: quien, Texto: texto, MensajeID: "M", Menciones: men})
}

func (e *entorno) enGrupo(quien, texto, citaAutor, citaID string) comandos.Resultado {
	return e.d.Ejecutar(comandos.Entrada{Persona: quien, Chat: grupo, EsGrupo: true, Texto: texto, MensajeID: "M", CitaAutor: citaAutor, CitaID: citaID})
}

func (e *entorno) dueno(texto string) comandos.Resultado {
	return e.d.Ejecutar(comandos.Entrada{Persona: almacen.PersonaDueno, Chat: propio, Texto: texto, Dueno: true, MensajeID: "D"})
}

func TestSilenciarDeLaAppYDelChatConJerarquia(t *testing.T) {
	e := nuevoEntorno(t)
	e.rol("Jefe", "personas.silenciar")
	e.rol("Raso")
	e.da(pnAna, "Jefe")
	e.da(pnLuis, "Raso")
	// El jefe silencia a Luis (rango más abajo) 2 horas y a alguien sin rango.
	if r := e.priv(pnAna, "!ignorar 5215522222222 2h"); !strings.Contains(r.Texto, "2 horas") {
		t.Fatalf("%q", r.Texto)
	}
	if s := e.alm.SilencioDe(1, pnLuis, e.hora); s == nil || s.HastaTS != e.hora.Add(2*time.Hour).Unix() {
		t.Fatalf("%+v", s)
	}
	if r := e.priv(pnAna, "!ignorar 5215533333333 siempre"); !strings.Contains(r.Texto, "para siempre") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.priv(pnAna, "!designorar 5215533333333"); !strings.Contains(r.Texto, "vuelve a contestarle") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.priv(pnAna, "!designorar 5215533333333"); !strings.Contains(r.Texto, "no estaba ignorada") {
		t.Fatalf("%q", r.Texto)
	}
	// Luis (Raso) no tiene la casilla.
	if r := e.priv(pnLuis, "!ignorar 5215511111111"); !strings.Contains(r.Texto, "No tienes permiso") {
		t.Fatalf("%q", r.Texto)
	}
	// El jefe no puede silenciar a otro jefe ni a sí mismo ni al Dueño (por su número).
	e.rol("Jefe2", "personas.silenciar")
	e.da(pnRosa, "Jefe2") // Jefe2 está más abajo que Jefe: el jefe sí puede; al revés no
	if r := e.priv(pnRosa, "!ignorar 5215511111111"); !strings.Contains(r.Texto, "rangos más bajos") {
		t.Fatalf("hacia arriba: %q", r.Texto)
	}
	if r := e.priv(pnAna, "!ignorar 5215511111111"); !strings.Contains(r.Texto, "contigo mismo") {
		t.Fatalf("a sí mismo: %q", r.Texto)
	}
	if r := e.priv(pnAna, "!ignorar "+strings.TrimSuffix(propio, "@s.whatsapp.net")); !strings.Contains(r.Texto, "rangos más bajos") {
		t.Fatalf("al Dueño: %q", r.Texto)
	}
	// Duración mal escrita.
	if r := e.priv(pnAna, "!ignorar 5215522222222 mañana"); !strings.Contains(r.Texto, "30m, 2h, 1d o siempre") {
		t.Fatalf("%q", r.Texto)
	}
	// Todo quedó en el registro de acciones sin textos.
	acs, _ := e.alm.ListarAcciones(1, 50, 0)
	vistos := map[string]bool{}
	for _, a := range acs {
		vistos[a.Accion+"|"+a.Resultado] = true
		if strings.Contains(a.Detalle, "mañana") {
			t.Fatalf("texto en el registro: %+v", a)
		}
	}
	for _, k := range []string{"persona.silenciar|ok", "persona.silenciar|denegado", "persona.desilenciar|ok"} {
		if !vistos[k] {
			t.Fatalf("falta en el registro %s: %+v", k, vistos)
		}
	}
}

func TestSilencioDeLaAppPorMinutosYPersonaPorLID(t *testing.T) {
	e := nuevoEntorno(t)
	e.alm.AprenderAlias(1, pnRosa, lidRosa)
	if err := e.s.Silenciar(almacen.PersonaDueno, lidRosa, 30*time.Minute, "mucho"); err != nil {
		t.Fatal(err)
	}
	if e.alm.SilencioDe(1, pnRosa, e.hora) == nil {
		t.Fatal("silencio por LID vale para el número")
	}
	if err := e.s.Silenciar(almacen.PersonaDueno, propio, 0, ""); !errors.Is(err, permisos.ErrJerarquia) {
		t.Fatalf("al Dueño por su número: %v", err)
	}
	e.hora = e.hora.Add(31 * time.Minute)
	if e.alm.SilencioDe(1, pnRosa, e.hora) != nil {
		t.Fatal("a los 31 min ya no está silenciada")
	}
}

func TestBloquearDesbloquearComandos(t *testing.T) {
	e := nuevoEntorno(t)
	e.rol("Mod", "personas.bloquear")
	e.da(pnAna, "Mod")
	if r := e.priv(pnAna, "!bloquear 5215533333333"); !strings.Contains(r.Texto, "no es un bloqueo de WhatsApp") {
		t.Fatalf("%q", r.Texto)
	}
	if !e.alm.EstaBloqueada(1, pnRosa) {
		t.Fatal("quedó bloqueada")
	}
	// La compuerta del motor la ignora (mismo estado de solicitudes).
	if sal := e.perm.Compuerta(permisos.Entrada{Persona: pnRosa, Chat: pnRosa}); sal.Pasa || sal.Motivo != "persona bloqueada" {
		t.Fatalf("%+v", sal)
	}
	if r := e.priv(pnAna, "!desbloquear 5215533333333"); !strings.Contains(r.Texto, "vuelve a atenderla") {
		t.Fatalf("%q", r.Texto)
	}
	if sal := e.perm.Compuerta(permisos.Entrada{Persona: pnRosa, Chat: pnRosa}); !sal.Pasa {
		t.Fatalf("%+v", sal)
	}
}

func TestCompuertaIgnoraALaSilenciadaConCaducidad(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.s.Silenciar(almacen.PersonaDueno, pnLuis, 10*time.Minute, "")
	if sal := e.perm.Compuerta(permisos.Entrada{Persona: pnLuis, Chat: pnLuis}); sal.Pasa || sal.Motivo != "persona silenciada" {
		t.Fatalf("%+v", sal)
	}
	// En un grupo también (la persona es el remitente).
	if sal := e.perm.Compuerta(permisos.Entrada{Persona: pnLuis, Chat: grupo, EsGrupo: true}); sal.Pasa {
		t.Fatal("silenciada en grupos")
	}
	if sal := e.perm.Compuerta(permisos.Entrada{Persona: pnAna, Chat: pnAna}); !sal.Pasa {
		t.Fatal("otra persona sigue igual")
	}
	e.hora = e.hora.Add(11 * time.Minute)
	if sal := e.perm.Compuerta(permisos.Entrada{Persona: pnLuis, Chat: pnLuis}); !sal.Pasa {
		t.Fatal("venció el silencio")
	}
}

func TestAdvertirYTopeAvisarSilenciarSinTextosEnElRegistro(t *testing.T) {
	e := nuevoEntorno(t)
	e.rol("Mod", "personas.advertir")
	e.da(pnAna, "Mod")
	_ = e.alm.FijarAjusteBot(1, "adv_tope", "2")
	r := e.enGrupo(pnAna, "!advertir 5215522222222 mandó spam secreto", "", "")
	if !strings.Contains(r.Texto, "lleva 1 de 2") || strings.Contains(r.Texto, "tope:") {
		t.Fatalf("%q", r.Texto)
	}
	// Segunda: llega al tope, acción "avisar" → notificación local al dueño.
	r = e.enGrupo(pnAna, "!advertir 5215522222222 otra vez", "", "")
	if !strings.Contains(r.Texto, "lleva 2 de 2") || !strings.Contains(r.Texto, "ya avisé al dueño") {
		t.Fatalf("%q", r.Texto)
	}
	av, _ := e.alm.AvisosRecientes(1, 1)
	if len(av) != 1 || av[0].Tipo != "advertencias" || !strings.Contains(av[0].Titulo, "tope") {
		t.Fatalf("aviso: %+v", av)
	}
	// Acción "silenciar": la tercera la silencia 60 min.
	_ = e.alm.FijarAjusteBot(1, "adv_accion", "silenciar")
	_ = e.alm.FijarAjusteBot(1, "adv_silencio_min", "60")
	r = e.enGrupo(pnAna, "!advertir 5215522222222", "", "")
	if !strings.Contains(r.Texto, "el bot ya no le contesta") {
		t.Fatalf("%q", r.Texto)
	}
	if s := e.alm.SilencioDe(1, pnLuis, e.hora); s == nil || s.HastaTS != e.hora.Add(time.Hour).Unix() {
		t.Fatalf("%+v", s)
	}
	// ninguna acción lleva el texto del motivo.
	acs, _ := e.alm.ListarAcciones(1, 50, 0)
	for _, a := range acs {
		if strings.Contains(a.Detalle+a.Accion+a.Resultado, "spam") {
			t.Fatalf("texto en el registro: %+v", a)
		}
	}
	// El motivo sí queda (cifrado) en las advertencias.
	l, _ := e.alm.ListarAdvertencias(1, pnLuis, 10)
	if len(l) != 3 {
		t.Fatal(len(l))
	}
	found := false
	for _, x := range l {
		if x.Motivo == "mandó spam secreto" {
			found = true
		}
	}
	if !found {
		t.Fatal("el motivo debe guardarse")
	}
	// Tope 0 = nunca actúa.
	_ = e.alm.FijarAjusteBot(1, "adv_tope", "0")
	_, _, _ = e.s.QuitarAdvertencia(almacen.PersonaDueno, pnLuis, 0)
	if res, _ := e.s.Advertir(context.Background(), almacen.PersonaDueno, pnLuis, grupo, "", ""); res.LlegoAlTope {
		t.Fatal("tope 0 = sin regla")
	}
}

func TestTopeBorrarYExpulsarSoloSiSoyAdmin(t *testing.T) {
	e := nuevoEntorno(t)
	ctx := context.Background()
	_ = e.alm.FijarAjusteBot(1, "adv_tope", "1")
	avisos := func() int {
		av, _ := e.alm.AvisosRecientes(1, 500)
		n := 0
		for _, x := range av {
			if x.Tipo == "advertencias" {
				n++
			}
		}
		return n
	}
	// ---- borrar ----
	_ = e.alm.FijarAjusteBot(1, "adv_accion", "borrar")
	// 1. sin ser administrador: solo alerta.
	e.g.admin = false
	r, _ := e.s.Advertir(ctx, almacen.PersonaDueno, pnLuis, grupo, "MSG1", "")
	if r.Hecho != "solo_aviso" || !strings.Contains(r.Motivo, "no soy administrador") || len(e.g.borrados) != 0 || avisos() != 1 {
		t.Fatalf("sin admin: %+v borrados=%v avisos=%d", r, e.g.borrados, avisos())
	}
	// 2. siendo admin: borra el mensaje citado.
	e.g.admin = true
	r, _ = e.s.Advertir(ctx, almacen.PersonaDueno, pnLuis, grupo, "MSG2", "")
	if r.Hecho != "borrado" || len(e.g.borrados) != 1 || e.g.borrados[0] != grupo+"|MSG2|"+pnLuis || avisos() != 2 {
		t.Fatalf("con admin: %+v %v", r, e.g.borrados)
	}
	// 3. admin pero sin mensaje al que apuntar.
	r, _ = e.s.Advertir(ctx, almacen.PersonaDueno, pnLuis, grupo, "", "")
	if r.Hecho != "solo_aviso" || !strings.Contains(r.Motivo, "sin mensaje") {
		t.Fatalf("sin mensaje: %+v", r)
	}
	// 4. admin pero la API todavía no existe (implementación nula de la etapa B): solo alerta.
	e.g.errAcc = grupos.ErrNoDisponible
	r, _ = e.s.Advertir(ctx, almacen.PersonaDueno, pnLuis, grupo, "MSG3", "")
	if r.Hecho != "solo_aviso" || !strings.Contains(r.Motivo, "todavía no está disponible") {
		t.Fatalf("api no disponible: %+v", r)
	}
	// 5. en un chat privado no se borra ni se expulsa.
	e.g.errAcc = nil
	r, _ = e.s.Advertir(ctx, almacen.PersonaDueno, pnLuis, pnLuis, "MSG4", "")
	if r.Hecho != "solo_aviso" || !strings.Contains(r.Motivo, "solo funciona en grupos") {
		t.Fatalf("privado: %+v", r)
	}
	// ---- expulsar ----
	_ = e.alm.FijarAjusteBot(1, "adv_accion", "expulsar")
	e.g.admin = false
	r, _ = e.s.Advertir(ctx, almacen.PersonaDueno, pnRosa, grupo, "", "")
	if r.Hecho != "solo_aviso" || len(e.g.expulsos) != 0 {
		t.Fatalf("expulsar sin admin: %+v", r)
	}
	e.g.admin = true
	r, _ = e.s.Advertir(ctx, almacen.PersonaDueno, pnRosa, grupo, "", "")
	if r.Hecho != "expulsado" || len(e.g.expulsos) != 1 || e.g.expulsos[0] != grupo+"|"+pnRosa {
		t.Fatalf("expulsar con admin: %+v %v", r, e.g.expulsos)
	}
	// No se pudo saber si soy admin (error de red): solo alerta.
	e.g.errAdmin = errors.New("sin conexión")
	r, _ = e.s.Advertir(ctx, almacen.PersonaDueno, pnAna, grupo, "", "")
	if r.Hecho != "solo_aviso" || len(e.g.expulsos) != 1 {
		t.Fatalf("sin saber: %+v", r)
	}
	// La implementación nula de grupos: EsAdmin dice "no disponible" y nunca actúa.
	e.s.Grupo = grupos.Nula{}
	e.g.errAdmin = nil
	r, _ = e.s.Advertir(ctx, almacen.PersonaDueno, pnLuis, grupo, "MSG9", "")
	if r.Hecho != "solo_aviso" || len(e.g.expulsos) != 1 {
		t.Fatalf("nula: %+v", r)
	}
	// Cada tope quedó en el registro (sin textos).
	acs, _ := e.alm.ListarAcciones(1, 100, 0)
	n := 0
	for _, a := range acs {
		if strings.HasPrefix(a.Accion, "persona.tope_") {
			n++
		}
	}
	if n < 6 {
		t.Fatalf("topes registrados: %d", n)
	}
}

func TestQuitarAdvertenciaYBorrarHistorialConConfirmacion(t *testing.T) {
	e := nuevoEntorno(t)
	e.rol("Mod", "personas.advertir")
	e.da(pnAna, "Mod")
	_ = e.alm.FijarAjusteBot(1, "adv_tope", "9")
	for i := 0; i < 3; i++ {
		e.priv(pnAna, "!advertir 5215522222222 x")
	}
	if r := e.priv(pnAna, "!advertencias 5215522222222"); !strings.Contains(r.Texto, "3 advertencia") || !strings.Contains(r.Texto, "tope 9") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.priv(pnAna, "!quitaradvertencia 5215522222222"); !strings.Contains(r.Texto, "le quedan 2") {
		t.Fatalf("%q", r.Texto)
	}
	// Borrar el historial pide confirmación: no se borra hasta que el dueño apruebe.
	r := e.priv(pnAna, "!borraradvertencias 5215522222222")
	if !strings.Contains(r.Texto, "necesita confirmación") || len(e.cod) != 1 {
		t.Fatalf("%q cod=%d", r.Texto, len(e.cod))
	}
	if e.alm.ContarAdvertencias(1, pnLuis) != 2 {
		t.Fatal("no debe borrar antes de confirmar")
	}
	// El dueño aprueba con el código (desde la app: persona vacía). El objetivo se guardó resuelto.
	txt, err := e.d.Confirmar(e.cod[0].Codigo, "")
	if err != nil || !strings.Contains(txt, "borré 2") {
		t.Fatalf("%q %v", txt, err)
	}
	if e.alm.ContarAdvertencias(1, pnLuis) != 0 {
		t.Fatal("historial borrado")
	}
	// Mismo código otra vez: no sirve.
	if _, err := e.d.Confirmar(e.cod[0].Codigo, ""); err == nil {
		t.Fatal("un solo uso")
	}
}

func TestNotasFichaYOlvidarPorComando(t *testing.T) {
	e := nuevoEntorno(t)
	e.rol("Jefe", "personas.notas", "personas.ficha", "personas.olvidar")
	e.da(pnAna, "Jefe")
	_, _ = e.alm.ActualizarChat(1, pnLuis, false, func(c *almacen.Chat) { c.TelE164, c.Publico = "5215522222222", "Luis" })
	_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: "A", ChatJID: pnLuis, BotID: 1, RemitenteJID: pnLuis, Tipo: almacen.TipoTexto, Texto: "hola", TS: e.hora.Unix()})
	if r := e.priv(pnAna, "!nota 5215522222222 siempre pide factura"); r.Texto != "Nota guardada." {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.priv(pnAna, "!nota 5215522222222"); !strings.Contains(r.Texto, "siempre pide factura") {
		t.Fatalf("%q", r.Texto)
	}
	r := e.priv(pnAna, "!ficha 5215522222222")
	if !strings.Contains(r.Texto, "Notas: 1") || !strings.Contains(r.Texto, "Rango: sin rango") || !strings.Contains(r.Texto, "Estado: normal") {
		t.Fatalf("%q", r.Texto)
	}
	// Olvidar pide confirmación y no borra nada hasta aprobarlo.
	r = e.priv(pnAna, "!olvidar 5215522222222")
	if !strings.Contains(r.Texto, "necesita confirmación") || !e.alm.TieneMensajes(1, pnLuis) {
		t.Fatalf("%q", r.Texto)
	}
	if _, err := e.d.Confirmar(e.cod[len(e.cod)-1].Codigo, ""); err != nil {
		t.Fatal(err)
	}
	if e.alm.TieneMensajes(1, pnLuis) || e.alm.ContarNotas(1, pnLuis) != 0 {
		t.Fatal("se olvidó a la persona")
	}
	acs, _ := e.alm.ListarAcciones(1, 50, 0)
	for _, a := range acs {
		if a.Objetivo == pnLuis || strings.Contains(a.Detalle, "factura") {
			t.Fatalf("el registro no debe saber a quién ni el texto: %+v", a)
		}
	}
}

func TestPausarReanudarControlYSiempre(t *testing.T) {
	e := nuevoEntorno(t)
	e.rol("Ayuda", "chat.pausar", "chat.tomar_control", "chat.estado", "chat.etiquetas")
	e.da(pnAna, "Ayuda")
	// En un grupo se pausa el grupo.
	if r := e.enGrupo(pnAna, "!pausar 2h", "", ""); !strings.Contains(r.Texto, "2 horas") {
		t.Fatalf("%q", r.Texto)
	}
	if !e.alm.BotPausado(1, grupo, e.hora) {
		t.Fatal("pausado")
	}
	if r := e.enGrupo(pnAna, "!control", "", ""); !strings.Contains(r.Texto, "tomaste el control") {
		t.Fatalf("%q", r.Texto)
	}
	if est := e.alm.EstadoDeChat(1, grupo); est.BotPausaHasta < e.hora.Add(300*24*time.Hour).Unix() {
		t.Fatal("tomar el control es largo")
	}
	if !e.d.SiemprePermitido("!reanudar") || e.d.SiemprePermitido("!pausar") {
		t.Fatal("!reanudar se atiende aun con el chat pausado")
	}
	if r := e.enGrupo(pnAna, "!reanudar", "", ""); !strings.Contains(r.Texto, "vuelve a contestar") {
		t.Fatalf("%q", r.Texto)
	}
	if e.alm.BotPausado(1, grupo, e.hora) || !e.alm.EstadoDeChat(1, grupo).BotActivo {
		t.Fatal("reanudado")
	}
	// En privado sin objetivo: el chat de quien lo escribe; con una persona: el chat de esa persona (con jerarquía).
	e.priv(pnAna, "!pausar 30m")
	if !e.alm.BotPausado(1, pnAna, e.hora) {
		t.Fatal("su propio chat")
	}
	if r := e.priv(pnAna, "!pausar 5215522222222 1h"); !strings.Contains(r.Texto, "1 hora") || !e.alm.BotPausado(1, pnLuis, e.hora) {
		t.Fatalf("%q", r.Texto)
	}
	e.da(pnRosa, "Ayuda")
	if r := e.priv(pnAna, "!pausar 5215533333333"); !strings.Contains(r.Texto, "rangos más bajos") {
		t.Fatalf("mismo rango: %q", r.Texto)
	}
	// Duración mal escrita
	if r := e.enGrupo(pnAna, "!pausar siempre", "", ""); !strings.Contains(r.Texto, "30m, 2h o 1d") {
		t.Fatalf("%q", r.Texto)
	}
	// El Dueño desde Mensajes guardados debe indicar a quién.
	if r := e.dueno("!pausar"); !strings.Contains(r.Texto, "de qué chat") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.dueno("!pausar 5215533333333 15m"); !strings.Contains(r.Texto, "15 min") || !e.alm.BotPausado(1, pnRosa, e.hora) {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.dueno("!reanudar 5215533333333"); !strings.Contains(r.Texto, "vuelve a contestar") || e.alm.BotPausado(1, pnRosa, e.hora) {
		t.Fatalf("%q", r.Texto)
	}
}

func TestPendienteResueltoYEtiquetar(t *testing.T) {
	e := nuevoEntorno(t)
	e.rol("Ayuda", "chat.estado", "chat.etiquetas")
	e.da(pnAna, "Ayuda")
	if r := e.enGrupo(pnAna, "!pendiente", "", ""); !strings.Contains(r.Texto, "pendiente") {
		t.Fatalf("%q", r.Texto)
	}
	if e.alm.EstadoDeChat(1, grupo).EstadoTrabajo != almacen.TrabajoPendiente {
		t.Fatal("pendiente")
	}
	e.enGrupo(pnAna, "!resuelto", "", "")
	if e.alm.EstadoDeChat(1, grupo).EstadoTrabajo != almacen.TrabajoResuelto {
		t.Fatal("resuelto")
	}
	// Una Ayuda no crea etiquetas nuevas (solo el dueño); sí usa las que existen.
	if r := e.enGrupo(pnAna, "!etiquetar Mayoristas", "", ""); !strings.Contains(r.Texto, "no existe") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.dueno("!etiquetar 5215522222222 Mayoristas"); !strings.Contains(r.Texto, "etiqueta Mayoristas puesta") {
		t.Fatalf("dueño crea: %q", r.Texto)
	}
	if r := e.enGrupo(pnAna, "!etiquetar mayoristas", "", ""); !strings.Contains(r.Texto, "puesta") {
		t.Fatalf("%q", r.Texto)
	}
	if et, _ := e.alm.EtiquetasDeChat(1, grupo); len(et) != 1 {
		t.Fatalf("%+v", et)
	}
	if r := e.enGrupo(pnAna, "!etiquetar quitar Mayoristas", "", ""); !strings.Contains(r.Texto, "quité") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.enGrupo(pnAna, "!etiquetar", "", ""); !strings.Contains(r.Texto, "Etiquetas: Mayoristas") {
		t.Fatalf("%q", r.Texto)
	}
}

func TestReportarSoloAvisaYNoGuardaElTextoDelMensaje(t *testing.T) {
	e := nuevoEntorno(t)
	_, _ = e.alm.ActualizarChat(1, grupo, true, func(c *almacen.Chat) { c.Guardado = "Vecinos" })
	_, _ = e.alm.GuardarMensaje(&almacen.Mensaje{ID: "BAD", ChatJID: grupo, BotID: 1, RemitenteJID: pnLuis, Tipo: almacen.TipoTexto, Texto: "texto ofensivo que no debe copiarse", TS: e.hora.Unix()})
	// Cualquiera (sin rango) puede reportar: responde al mensaje.
	r := e.enGrupo(pnRosa, "!reportar me insultó", pnLuis, "BAD")
	if r.Texto != "Gracias, ya avisé." {
		t.Fatalf("por defecto confirma a quien reporta: %q", r.Texto)
	}
	_ = e.alm.FijarAjusteBot(1, "reportar_gracias", "0")
	av, _ := e.alm.AvisosRecientes(1, 5)
	if len(av) != 1 {
		t.Fatalf("avisos: %+v", av)
	}
	a := av[0]
	if a.Tipo != "reporte" || a.Chat != grupo || a.Persona != pnLuis || !strings.Contains(a.Detalle, "Vecinos") || !strings.Contains(a.Detalle, "me insultó") || strings.Contains(a.Detalle, "ofensivo") {
		t.Fatalf("aviso: %+v", a)
	}
	// Sin texto del mensaje en el registro.
	acs, _ := e.alm.ListarAcciones(1, 20, 0)
	for _, x := range acs {
		if strings.Contains(x.Detalle+x.Accion+x.Objetivo+x.Chat, "ofensivo") || strings.Contains(x.Detalle, "insultó") {
			t.Fatalf("texto en el registro: %+v", x)
		}
	}
	// Sin citar nada: pide que responda a un mensaje.
	if r := e.enGrupo(pnRosa, "!reportar", "", ""); !strings.Contains(r.Texto, "esponde al mensaje") {
		t.Fatalf("%q", r.Texto)
	}
	// Apagado: no contesta nada, solo avisa al dueño.
	if r := e.enGrupo(pnRosa, "!reportar", pnLuis, "BAD"); r.Texto != "" {
		t.Fatalf("apagado no responde: %q", r.Texto)
	}
	_ = e.alm.FijarAjusteBot(1, "reportar_gracias", "1")
	if r := e.enGrupo(pnRosa, "!reportar", pnLuis, "BAD"); r.Texto != "Gracias, ya avisé." {
		t.Fatalf("%q", r.Texto)
	}
	_ = e.alm.FijarAjusteBot(1, "reportar_texto", "Listo, el dueño lo verá")
	if r := e.enGrupo(pnRosa, "!reportar", pnLuis, "BAD"); r.Texto != "Listo, el dueño lo verá" {
		t.Fatalf("%q", r.Texto)
	}
}

func TestNoSePuedeReportarUnMensajePropioNiDelBot(t *testing.T) {
	e := nuevoEntorno(t)
	avisos := func() int { a, _ := e.alm.AvisosRecientes(1, 20); return len(a) }
	// un mensaje propio
	r := e.enGrupo(pnRosa, "!reportar", pnRosa, "MIO")
	if !strings.Contains(r.Texto, "mensaje tuyo") || avisos() != 0 {
		t.Fatalf("propio: %q avisos=%d", r.Texto, avisos())
	}
	// un mensaje del bot (la cuenta del dueño: «propio» en las pruebas)
	r = e.enGrupo(pnRosa, "!reportar", propio, "BOT")
	if !strings.Contains(r.Texto, "mensaje del bot") || avisos() != 0 {
		t.Fatalf("bot: %q avisos=%d", r.Texto, avisos())
	}
	// el de otra persona sí
	if r := e.enGrupo(pnRosa, "!reportar", pnLuis, "BAD"); r.Texto == "" || avisos() != 1 {
		t.Fatalf("otra persona: %q avisos=%d", r.Texto, avisos())
	}
}

func TestAyudaListaLosComandosDeBSegunRango(t *testing.T) {
	e := nuevoEntorno(t)
	e.rol("Mod", "personas.silenciar", "personas.advertir")
	e.da(pnAna, "Mod")
	sin := e.priv(pnRosa, "!ayuda").Texto
	if !strings.Contains(sin, "!reportar") || strings.Contains(sin, "!ignorar") || strings.Contains(sin, "!olvidar") || strings.Contains(sin, "!recordar") {
		t.Fatalf("sin rango:\n%s", sin)
	}
	mod := e.priv(pnAna, "!ayuda").Texto
	for _, c := range []string{"!ignorar", "!designorar", "!advertir", "!advertencias", "!reportar"} {
		if !strings.Contains(mod, c) {
			t.Fatalf("falta %s:\n%s", c, mod)
		}
	}
	if strings.Contains(mod, "!bloquear") || strings.Contains(mod, "!nota") || strings.Contains(mod, "!pausar") {
		t.Fatalf("no debía ver:\n%s", mod)
	}
}

func TestParseDuracion(t *testing.T) {
	casos := map[string]time.Duration{"30": 30 * time.Minute, "30m": 30 * time.Minute, "45min": 45 * time.Minute, "2h": 2 * time.Hour, "1d": 24 * time.Hour, "siempre": 0}
	for in, esp := range casos {
		if d, ok := ParseDuracion(in); !ok || d != esp {
			t.Fatalf("%q → %v %v", in, d, ok)
		}
	}
	for _, mal := range []string{"", "x", "-5m", "0m", "1.5h", "10s", "mañana", "999999999h"} {
		if _, ok := ParseDuracion(mal); ok {
			t.Fatalf("%q no es una duración", mal)
		}
	}
}
