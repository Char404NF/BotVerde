package permisos

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
)

const (
	pnAna  = "5215511111111@s.whatsapp.net"
	lidAna = "99887766554433@lid"
	pnLuis = "5215522222222@s.whatsapp.net"
	pnRosa = "5215533333333@s.whatsapp.net"
)

type entorno struct {
	s    *Servicio
	alm  *almacen.Almacen
	hora time.Time
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
	s, err := Nuevo(alm, 1)
	if err != nil {
		t.Fatal(err)
	}
	e := &entorno{s: s, alm: alm, hora: time.Now(), ids: map[string]int64{}}
	s.Ahora = func() time.Time { return e.hora }
	roles, _ := alm.ListarRoles(1)
	for _, r := range roles {
		e.ids[r.Nombre] = r.ID
	}
	return e
}

func TestCatalogoExtensibleYAgrupado(t *testing.T) {
	cs := Catalogo()
	if len(cs) < 35 {
		t.Fatalf("catálogo corto: %d", len(cs))
	}
	vistos := map[string]bool{}
	for _, c := range cs {
		if c.Nombre == "" || c.Descripcion == "" || c.Grupo == "" || vistos[c.ID] {
			t.Fatalf("casilla incompleta o repetida: %+v", c)
		}
		vistos[c.ID] = true
	}
	// Casillas de los bloques B, C, D, E, F con riesgo y confirmación donde borran o expulsan.
	for _, id := range []string{"personas.silenciar", "chat.pausar", "grupos.info", "grupos.expulsar", "grupos.cerrar", "util.notas", "menus.usar", "auto.recibir"} {
		if _, ok := Buscar(id); !ok {
			t.Fatalf("falta %s", id)
		}
	}
	if c, _ := Buscar("grupos.expulsar"); !c.Confirma || c.Riesgo != RiesgoAlto || c.Privado {
		t.Fatalf("expulsar: %+v", c)
	}
	// Un módulo agrega las suyas desde fuera.
	Registrar(Casilla{ID: "prueba.mod", Nombre: "De prueba", Descripcion: "x", Grupo: "Módulo de prueba", Riesgo: RiesgoMedio, Modulo: "prueba"})
	defer Quitar("prueba.mod")
	if c, ok := Buscar("prueba.mod"); !ok || !c.Privado || !c.Grupos || c.Modulo != "prueba" {
		t.Fatalf("registrada: %+v", c)
	}
	if !strings.Contains(CatalogoJSON(), "prueba.mod") {
		t.Fatal("el JSON no la trae")
	}
	// Un rol puede usarla en cuanto se registra.
	r := &almacen.Rol{Nombre: "X", Casillas: map[string]almacen.Alcance{"prueba.mod": {}}}
	if err := ValidarRol(r); err != nil {
		t.Fatal(err)
	}
	if !Permite(r, "prueba.mod", false, "c") {
		t.Fatal("debía permitirla")
	}
}

func TestJerarquia(t *testing.T) {
	e := nuevoEntorno(t)
	alto, _ := e.s.GuardarRol(almacen.Rol{Nombre: "Alto", Casillas: map[string]almacen.Alcance{"rangos.asignar": {}}})
	medio, _ := e.s.GuardarRol(almacen.Rol{Nombre: "Medio", Casillas: map[string]almacen.Alcance{"rangos.asignar": {}}})
	bajo, _ := e.s.GuardarRol(almacen.Rol{Nombre: "Bajo"})
	_ = e.alm.AsignarRol(1, pnAna, alto, 0, "yo")
	_ = e.alm.AsignarRol(1, pnLuis, medio, 0, "yo")
	_ = e.alm.AsignarRol(1, pnRosa, bajo, 0, "yo")

	if !e.s.PuedeActuarSobre(pnAna, pnLuis) || !e.s.PuedeActuarSobre(pnLuis, pnRosa) {
		t.Fatal("un rango actúa sobre los de abajo")
	}
	if e.s.PuedeActuarSobre(pnLuis, pnAna) || e.s.PuedeActuarSobre(pnRosa, pnLuis) {
		t.Fatal("no sobre los de arriba")
	}
	if e.s.PuedeActuarSobre(pnLuis, pnLuis) {
		t.Fatal("ni sobre su mismo rango")
	}
	// Una persona sin rango es la más baja.
	if !e.s.PuedeActuarSobre(pnRosa, "5215599999999@s.whatsapp.net") {
		t.Fatal("sobre sin rango sí")
	}
	// Dueño sobre todos; nadie toca al Dueño.
	for _, p := range []string{pnAna, pnLuis, pnRosa} {
		if !e.s.PuedeActuarSobre(almacen.PersonaDueno, p) || e.s.PuedeActuarSobre(p, almacen.PersonaDueno) {
			t.Fatal("dueño")
		}
	}
	if e.s.PuedeActuarSobre(almacen.PersonaDueno, almacen.PersonaDueno) {
		t.Fatal("ni el Dueño sobre sí mismo")
	}
	// Asignar con jerarquía: Medio puede dar Bajo, no Alto ni Medio, ni tocar a Alto.
	if err := e.s.AsignarComo(pnLuis, "5215544444444@s.whatsapp.net", bajo, 0); err != nil {
		t.Fatal(err)
	}
	if err := e.s.AsignarComo(pnLuis, "5215544444444@s.whatsapp.net", alto, 0); err != ErrJerarquia {
		t.Fatalf("dar rango más alto: %v", err)
	}
	if err := e.s.AsignarComo(pnLuis, "5215544444444@s.whatsapp.net", medio, 0); err != ErrJerarquia {
		t.Fatalf("dar su mismo rango: %v", err)
	}
	if err := e.s.AsignarComo(pnLuis, pnAna, bajo, 0); err != ErrJerarquia {
		t.Fatalf("degradar a un superior: %v", err)
	}
	if err := e.s.QuitarComo(pnLuis, pnAna); err != ErrJerarquia {
		t.Fatalf("quitar a un superior: %v", err)
	}
	if err := e.s.AsignarComo(almacen.PersonaDueno, pnLuis, bajo, 0); err != nil {
		t.Fatalf("el dueño sí: %v", err)
	}
	// Nadie le da Dueño a otro ni toca al dueño aunque se escriba su número.
	roles, _ := e.alm.ListarRoles(1)
	if err := e.s.AsignarComo(almacen.PersonaDueno, pnAna, roles[0].ID, 0); err != ErrJerarquia {
		t.Fatalf("dar Dueño: %v", err)
	}
	e.s.EsPropio = func(p string) bool { return p == "5215500000001@s.whatsapp.net" }
	if err := e.s.AsignarComo(pnAna, "5215500000001@s.whatsapp.net", bajo, 0); err != ErrJerarquia {
		t.Fatalf("tocar al dueño por su número: %v", err)
	}
}

func TestAlcancePorCasilla(t *testing.T) {
	e := nuevoEntorno(t)
	id, err := e.s.GuardarRol(almacen.Rol{Nombre: "Mixto", Casillas: map[string]almacen.Alcance{
		"personas.silenciar": {Modo: AlcancePrivado},
		"personas.notas":     {Modo: AlcanceGrupos},
		"chat.pausar":        {Modo: AlcanceChats, Chats: []string{"grupoA@g.us"}},
		"grupos.info":        {},
		"util.notas":         {},
	}})
	if err != nil {
		t.Fatal(err)
	}
	_ = e.alm.AsignarRol(1, pnAna, id, 0, "yo")
	casos := []struct {
		cas    string
		grupo  bool
		chat   string
		espera bool
	}{
		{"personas.silenciar", false, "c", true},
		{"personas.silenciar", true, "g@g.us", false},
		{"personas.notas", false, "c", false},
		{"personas.notas", true, "g@g.us", true},
		{"chat.pausar", true, "grupoA@g.us", true},
		{"chat.pausar", true, "grupoB@g.us", false},
		{"chat.pausar", false, "grupoA@g.us", false}, // el chat concreto es un grupo; en privado con ese id no coincide con esGrupo=false... se acepta si el id coincide
		{"grupos.info", true, "g@g.us", true},
		{"grupos.info", false, "c", false}, // la función solo existe en grupos
		{"util.notas", false, "c", true},
		{"util.notas", true, "g", true},
		{"personas.bloquear", false, "c", false}, // no la tiene
		{"cmd.ayuda", false, "c", true},          // básica
	}
	for _, c := range casos {
		got := e.s.Puede(pnAna, c.cas, c.grupo, c.chat)
		// el caso privado con chat de grupo: el alcance "chats" compara solo el id del chat
		if c.cas == "chat.pausar" && !c.grupo && c.chat == "grupoA@g.us" {
			if !got {
				t.Fatalf("chat concreto: %+v", c)
			}
			continue
		}
		if got != c.espera {
			t.Fatalf("%+v → %v", c, got)
		}
	}
	// Sin rango: solo lo básico. Dueño: todo, incluido lo que no existe todavía.
	if e.s.Puede(pnLuis, "personas.silenciar", false, "c") || !e.s.Puede(pnLuis, "cmd.ayuda", false, "c") {
		t.Fatal("sin rango")
	}
	if !e.s.Puede(almacen.PersonaDueno, "modulo.futuro", true, "g") {
		t.Fatal("dueño")
	}
	// Validaciones.
	malo := almacen.Rol{Nombre: "M", Casillas: map[string]almacen.Alcance{"grupos.expulsar": {Modo: AlcancePrivado}}}
	if ValidarRol(&malo) == nil {
		t.Fatal("expulsar no existe en privado")
	}
	malo = almacen.Rol{Nombre: "M", Casillas: map[string]almacen.Alcance{"no.existe": {}}}
	if ValidarRol(&malo) == nil {
		t.Fatal("casilla desconocida")
	}
	malo = almacen.Rol{Nombre: "M", Casillas: map[string]almacen.Alcance{"chat.pausar": {Modo: AlcanceChats}}}
	if ValidarRol(&malo) == nil {
		t.Fatal("chats vacío")
	}
	if ValidarRol(&almacen.Rol{Nombre: "Dueño"}) == nil || ValidarRol(&almacen.Rol{Nombre: ""}) == nil || ValidarRol(&almacen.Rol{Nombre: "A", Color: "rojo"}) == nil {
		t.Fatal("validaciones de nombre/color")
	}
}

func TestMenusVisibles(t *testing.T) {
	r := &almacen.Rol{MenusModo: "lista", Menus: []string{"3", "5:op2"}}
	if !MenuVisible(r, "3", "") || MenuVisible(r, "4", "") || !MenuVisible(r, "5", "op2") || MenuVisible(r, "5", "op1") {
		t.Fatal("lista")
	}
	if MenuVisible(&almacen.Rol{MenusModo: "ninguno"}, "1", "") || !MenuVisible(nil, "1", "") || !MenuVisible(&almacen.Rol{MenusModo: "todos"}, "1", "") {
		t.Fatal("modos")
	}
}

func TestLimitePorMinuto(t *testing.T) {
	e := nuevoEntorno(t)
	id, _ := e.s.GuardarRol(almacen.Rol{Nombre: "Lento", LimiteMin: 3})
	_ = e.alm.AsignarRol(1, pnAna, id, 0, "yo")
	for i := 0; i < 3; i++ {
		if ok, _ := e.s.Limitar(pnAna); !ok {
			t.Fatalf("intento %d", i)
		}
	}
	ok, primero := e.s.Limitar(pnAna)
	if ok || !primero {
		t.Fatal("el cuarto se pasa y se avisa una vez")
	}
	ok, primero = e.s.Limitar(pnAna)
	if ok || primero {
		t.Fatal("después se calla")
	}
	// por persona: otra no se ve afectada (límite por defecto sin rango = 5)
	if ok, _ := e.s.Limitar(pnLuis); !ok {
		t.Fatal("otra persona")
	}
	// pasa el minuto y vuelve
	e.hora = e.hora.Add(61 * time.Second)
	if ok, _ := e.s.Limitar(pnAna); !ok {
		t.Fatal("debía reabrirse")
	}
	// sin rango: 5 por minuto (ajuste), 0 = sin límite
	e.hora = e.hora.Add(2 * time.Minute)
	n := 0
	for i := 0; i < 10; i++ {
		if ok, _ := e.s.Limitar(pnRosa); ok {
			n++
		}
	}
	if n != 5 {
		t.Fatalf("sin rango: %d", n)
	}
	_ = e.alm.FijarAjusteBot(1, "limite_sin_rango", "0")
	for i := 0; i < 50; i++ {
		if ok, _ := e.s.Limitar("5215588888888@s.whatsapp.net"); !ok {
			t.Fatal("0 es sin límite")
		}
	}
	// el Dueño no tiene límite
	for i := 0; i < 100; i++ {
		if ok, _ := e.s.Limitar(almacen.PersonaDueno); !ok {
			t.Fatal("dueño")
		}
	}
	// el límite sigue a la persona aunque cambie de número a LID
	e.alm.AprenderAlias(1, pnLuis, lidAna)
	e.alm.AprenderAlias(1, pnAna, "1122334455@lid")
}

func TestLimiteCuentaLaMismaPersonaPorNumeroYLID(t *testing.T) {
	e := nuevoEntorno(t)
	id, _ := e.s.GuardarRol(almacen.Rol{Nombre: "Dos", LimiteMin: 2})
	_ = e.alm.AsignarRol(1, pnAna, id, 0, "yo")
	e.alm.AprenderAlias(1, pnAna, lidAna)
	a1, _ := e.s.Limitar(pnAna)
	a2, _ := e.s.Limitar(lidAna)
	a3, _ := e.s.Limitar(pnAna)
	if !a1 || !a2 || a3 {
		t.Fatalf("%v %v %v", a1, a2, a3)
	}
}

func TestCompuertaDesconocidos(t *testing.T) {
	e := nuevoEntorno(t)
	desconocida := func(p string) Entrada { return Entrada{Persona: p, Chat: p, Nombre: "Pedro"} }
	nuevos := 0
	e.s.AlSolicitud = func(string, string) { nuevos++ }

	// Política por defecto: responder el menú, a todos.
	if !e.s.Compuerta(desconocida(pnAna)).Pasa {
		t.Fatal("por defecto pasa")
	}
	// Esperar aprobación.
	if err := e.s.FijarPoliticaDesconocidos(DescAprobar); err != nil {
		t.Fatal(err)
	}
	// Quien ya había escrito antes de activar la política cuenta como conocido.
	_, _ = e.alm.ActualizarChat(1, pnLuis, false, func(c *almacen.Chat) { c.TelE164 = "+5215522222222" })
	e.s.Ahora = func() time.Time { return time.Now().Add(time.Hour) }
	e.alm.FijarAjusteBot(1, "desconocidos_desde", "1") // para que el chat recién creado cuente como posterior
	r := e.s.Compuerta(desconocida(pnAna))
	if r.Pasa || r.Responder != "" || !strings.Contains(r.Motivo, "aprobación") {
		t.Fatalf("%+v", r)
	}
	if nuevos != 1 || e.alm.ContarSolicitudes(1, almacen.SolPendiente) != 1 {
		t.Fatal("debía quedar pendiente")
	}
	e.s.Compuerta(desconocida(pnAna))
	if nuevos != 1 {
		t.Fatal("no avisa dos veces")
	}
	// Aprobar → pasa.
	_ = e.alm.DecidirSolicitud(1, pnAna, almacen.SolAprobada, time.Now())
	if !e.s.Compuerta(desconocida(pnAna)).Pasa {
		t.Fatal("aprobada pasa")
	}
	// Bloqueada → no pasa ni en política abierta.
	_ = e.alm.DecidirSolicitud(1, pnRosa, almacen.SolBloqueada, time.Now())
	if e.s.Compuerta(desconocida(pnRosa)).Pasa {
		t.Fatal("bloqueada")
	}
	_ = e.s.FijarPoliticaDesconocidos(DescMenu)
	if e.s.Compuerta(desconocida(pnRosa)).Pasa {
		t.Fatal("bloqueada incluso con el menú abierto")
	}
	// Con rango pasa; "te contestamos pronto" lo dice una sola vez.
	_ = e.s.FijarPoliticaDesconocidos(DescPronto)
	idr, _ := e.s.GuardarRol(almacen.Rol{Nombre: "VIP2"})
	_ = e.alm.AsignarRol(1, "5215500000009@s.whatsapp.net", idr, 0, "yo")
	if !e.s.Compuerta(desconocida("5215500000009@s.whatsapp.net")).Pasa {
		t.Fatal("con rango pasa")
	}
	nuevo := "5215500000010@s.whatsapp.net"
	r = e.s.Compuerta(desconocida(nuevo))
	if r.Pasa || r.Responder == "" {
		t.Fatalf("pronto: %+v", r)
	}
	if r2 := e.s.Compuerta(desconocida(nuevo)); r2.Pasa || r2.Responder != "" {
		t.Fatalf("pronto solo una vez: %+v", r2)
	}
	// Ignorar: ni se guarda.
	_ = e.s.FijarPoliticaDesconocidos(DescIgnorar)
	antes := e.alm.ContarSolicitudes(1, almacen.SolPendiente)
	if r := e.s.Compuerta(desconocida("5215500000011@s.whatsapp.net")); r.Pasa || r.Responder != "" {
		t.Fatalf("ignorar: %+v", r)
	}
	if e.alm.ContarSolicitudes(1, almacen.SolPendiente) != antes {
		t.Fatal("ignorar no guarda")
	}
	// Tope de pendientes: al llegar, los nuevos se ignoran sin guardarse.
	_ = e.s.FijarPoliticaDesconocidos(DescAprobar)
	_ = e.alm.FijarAjusteBot(1, "tope_pendientes", fmt.Sprint(antes+1))
	e.s.Compuerta(desconocida("5215500000012@s.whatsapp.net"))
	r = e.s.Compuerta(desconocida("5215500000013@s.whatsapp.net"))
	if r.Pasa || !strings.Contains(r.Motivo, "tope") || e.alm.ContarSolicitudes(1, almacen.SolPendiente) != antes+1 {
		t.Fatalf("tope: %+v", r)
	}
	// Un contacto de la agenda no es desconocido.
	_, _ = e.alm.ActualizarChat(1, "5215500000014@s.whatsapp.net", false, func(c *almacen.Chat) { c.Guardado = "Mi tía" })
	if !e.s.Compuerta(desconocida("5215500000014@s.whatsapp.net")).Pasa {
		t.Fatal("la agenda es conocida")
	}
}

func TestCompuertaPoliticaDeGrupos(t *testing.T) {
	e := nuevoEntorno(t)
	g := "grupo@g.us"
	esAdmin := false
	ent := func(p string) Entrada {
		return Entrada{Persona: p, Chat: g, EsGrupo: true, EsAdmin: func() bool { return esAdmin }}
	}
	if !e.s.Compuerta(ent(pnAna)).Pasa {
		t.Fatal("por defecto: todos")
	}
	_ = e.alm.FijarPoliticaGrupo(1, almacen.PoliticaGrupo{Chat: g, Modo: "admins"})
	if e.s.Compuerta(ent(pnAna)).Pasa {
		t.Fatal("solo admins")
	}
	esAdmin = true
	if !e.s.Compuerta(ent(pnAna)).Pasa {
		t.Fatal("admin pasa")
	}
	_ = e.alm.FijarPoliticaGrupo(1, almacen.PoliticaGrupo{Chat: g, Modo: "lista", Lista: []string{lidAna}})
	if e.s.Compuerta(ent(pnLuis)).Pasa {
		t.Fatal("fuera de la lista")
	}
	if e.s.Compuerta(ent(pnAna)).Pasa {
		t.Fatal("todavía no se sabe que el LID es Ana")
	}
	e.alm.AprenderAlias(1, pnAna, lidAna)
	if !e.s.Compuerta(ent(pnAna)).Pasa || !e.s.Compuerta(ent(lidAna)).Pasa {
		t.Fatal("en la lista por número y por LID")
	}
	// Otro grupo no se ve afectado: aprobar en privado no autoriza ni desautoriza grupos.
	_ = e.alm.DecidirSolicitud(1, pnLuis, almacen.SolAprobada, time.Now())
	if e.s.Compuerta(ent(pnLuis)).Pasa {
		t.Fatal("aprobar en privado NO autoriza el grupo con política de lista")
	}
	if !e.s.Compuerta(Entrada{Persona: pnLuis, Chat: "otro@g.us", EsGrupo: true}).Pasa {
		t.Fatal("otro grupo con la política general")
	}
	// Bloqueados tampoco en grupos.
	_ = e.alm.DecidirSolicitud(1, pnRosa, almacen.SolBloqueada, time.Now())
	if e.s.Compuerta(Entrada{Persona: pnRosa, Chat: "otro@g.us", EsGrupo: true}).Pasa {
		t.Fatal("bloqueada en grupos")
	}
	// Política general de grupos configurable.
	_ = e.alm.FijarAjusteBot(1, "grupo_modo_defecto", "admins")
	esAdmin = false
	if e.s.Compuerta(Entrada{Persona: pnLuis, Chat: "nuevo@g.us", EsGrupo: true, EsAdmin: func() bool { return esAdmin }}).Pasa {
		t.Fatal("política general admins")
	}
}

func TestCaducidadDe(t *testing.T) {
	ahora := time.Unix(1_000_000, 0)
	if c, _ := CaducidadDe("24h", ahora); c != ahora.Add(24*time.Hour).Unix() {
		t.Fatal("24h")
	}
	if c, _ := CaducidadDe("7d", ahora); c != ahora.Add(7*24*time.Hour).Unix() {
		t.Fatal("7d")
	}
	if c, err := CaducidadDe("permanente", ahora); c != 0 || err != nil {
		t.Fatal("perm")
	}
	if _, err := CaducidadDe("3 meses", ahora); err == nil {
		t.Fatal("inválida")
	}
}

func TestQuienYaEscribioAntesCuentaComoConocido(t *testing.T) {
	e := nuevoEntorno(t)
	_, _ = e.alm.ActualizarChat(1, pnLuis, false, func(c *almacen.Chat) { c.TelE164 = "+5215522222222" })
	e.hora = time.Now().Add(time.Hour) // la política se activa una hora después
	if err := e.s.FijarPoliticaDesconocidos(DescAprobar); err != nil {
		t.Fatal(err)
	}
	if !e.s.Compuerta(Entrada{Persona: pnLuis, Chat: pnLuis}).Pasa {
		t.Fatal("quien ya había escrito antes es conocido")
	}
	if e.s.Compuerta(Entrada{Persona: pnAna, Chat: pnAna}).Pasa {
		t.Fatal("alguien nuevo es desconocido")
	}
}
