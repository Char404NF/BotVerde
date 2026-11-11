package comandos

import (
	"errors"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/permisos"
)

const (
	pnAna  = "5215511111111@s.whatsapp.net"
	pnLuis = "5215522222222@s.whatsapp.net"
	pnRosa = "5215533333333@s.whatsapp.net"
	grupo  = "grupo@g.us"
)

type entorno struct {
	d      *Despachador
	alm    *almacen.Almacen
	perm   *permisos.Servicio
	hora   time.Time
	codigo []almacen.Confirmacion
	resp   []string
	ids    map[string]int64
}

func nuevoEntorno(t *testing.T) *entorno {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 5)
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
	e := &entorno{alm: alm, perm: p, hora: time.Now(), ids: map[string]int64{}}
	p.Ahora = func() time.Time { return e.hora }
	e.d = Nuevo(alm, 1, p)
	e.d.Ahora = p.Ahora
	e.d.AlCodigo = func(c almacen.Confirmacion, _ string) { e.codigo = append(e.codigo, c) }
	e.d.Responder = func(chat, cita, texto string) { e.resp = append(e.resp, chat+"|"+cita+"|"+texto) }
	roles, _ := alm.ListarRoles(1)
	for _, r := range roles {
		e.ids[r.Nombre] = r.ID
	}
	return e
}

func (e *entorno) priv(quien, texto string) Resultado {
	return e.d.Ejecutar(Entrada{Persona: quien, Chat: quien, Texto: texto, MensajeID: "M"})
}

func (e *entorno) dueno(texto string) Resultado {
	return e.d.Ejecutar(Entrada{Persona: almacen.PersonaDueno, Chat: "yo@s.whatsapp.net", Texto: texto, Dueno: true, MensajeID: "D"})
}

func (e *entorno) rol(nombre string, cas ...string) int64 {
	m := map[string]almacen.Alcance{}
	for _, c := range cas {
		m[c] = almacen.Alcance{}
	}
	id, err := e.perm.GuardarRol(almacen.Rol{Nombre: nombre, Casillas: m, LimiteMin: 50})
	if err != nil {
		panic(err)
	}
	e.ids[nombre] = id
	return id
}

func TestPrefijoConfigurableYParseo(t *testing.T) {
	e := nuevoEntorno(t)
	for _, s := range []string{"!ayuda", "  !ayuda  ", "!AYUDA", "!ayuda ahora"} {
		if !e.d.Existe(s) {
			t.Fatalf("%q debía ser un comando", s)
		}
	}
	for _, s := range []string{"ayuda", "! ayuda", "!!", "!1", "hola !ayuda", ""} {
		if e.d.EsComando(s) {
			t.Fatalf("%q no es comando", s)
		}
	}
	if e.d.Existe("!inventado") || !e.d.EsComando("!inventado") {
		t.Fatal("conocido vs. con forma de comando")
	}
	_ = e.alm.FijarAjusteBot(1, "prefijo", "#")
	if !e.d.Existe("#ayuda") || e.d.EsComando("!ayuda") {
		t.Fatal("prefijo # configurable")
	}
	// No conocido y no dueño: que siga el motor de menús.
	if r := e.priv(pnAna, "#inventado"); r.Manejado {
		t.Fatal("el desconocido sigue al motor de menús")
	}
	// Pero para el Dueño se responde (nunca cae en el motor de menús).
	if r := e.dueno("#inventado"); !r.Manejado || !strings.Contains(r.Texto, "No conozco") {
		t.Fatalf("%+v", r)
	}
}

func TestAyudaMuestraSoloLoQuePuede(t *testing.T) {
	e := nuevoEntorno(t)
	e.rol("Jefe", "rangos.asignar")
	_ = e.alm.AsignarRol(1, pnAna, e.ids["Jefe"], 0, "yo")
	e.rol("Raso")
	_ = e.alm.AsignarRol(1, pnLuis, e.ids["Raso"], 0, "yo")

	sin := e.priv(pnRosa, "!ayuda").Texto
	if !strings.Contains(sin, "!ayuda") || !strings.Contains(sin, "!rango") || strings.Contains(sin, "!asignar") || strings.Contains(sin, "!quitarrango") {
		t.Fatalf("sin rango:\n%s", sin)
	}
	raso := e.priv(pnLuis, "!ayuda").Texto
	if strings.Contains(raso, "!asignar") || !strings.Contains(raso, "rango Raso") {
		t.Fatalf("rango raso:\n%s", raso)
	}
	jefe := e.priv(pnAna, "!ayuda").Texto
	if !strings.Contains(jefe, "!asignar") || !strings.Contains(jefe, "!quitarrango") {
		t.Fatalf("jefe:\n%s", jefe)
	}
	du := e.dueno("!ayuda").Texto
	if !strings.Contains(du, "!asignar") {
		t.Fatalf("dueño:\n%s", du)
	}
	// Un comando solo de dueño no sale a nadie más.
	_ = e.d.Registrar(Comando{Nombre: "secreto", SoloDueno: true, Resumen: "x", Ejecutar: func(*Contexto) (string, error) { return "ok", nil }})
	if strings.Contains(e.priv(pnAna, "!ayuda").Texto, "!secreto") || !strings.Contains(e.dueno("!ayuda").Texto, "!secreto") {
		t.Fatal("solo dueño en la ayuda")
	}
	// Las respuestas jamás empiezan con el prefijo (así no hay bucles con los ecos en Mensajes guardados).
	for _, r := range []string{sin, raso, jefe, du} {
		if strings.HasPrefix(r, "!") {
			t.Fatal("la respuesta empieza con el prefijo")
		}
	}
}

func TestAmbitoPrivadoYGrupo(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.d.Registrar(Comando{Nombre: "soloprivado", Ambito: Privado, Ejecutar: func(*Contexto) (string, error) { return "priv", nil }})
	_ = e.d.Registrar(Comando{Nombre: "sologrupo", Ambito: Grupo, Ejecutar: func(*Contexto) (string, error) { return "grp", nil }})
	if r := e.priv(pnAna, "!soloprivado"); r.Texto != "priv" {
		t.Fatal(r.Texto)
	}
	if r := e.d.Ejecutar(Entrada{Persona: pnAna, Chat: grupo, EsGrupo: true, Texto: "!soloprivado"}); !strings.Contains(r.Texto, "privado") {
		t.Fatal(r.Texto)
	}
	if r := e.d.Ejecutar(Entrada{Persona: pnAna, Chat: grupo, EsGrupo: true, Texto: "!sologrupo"}); r.Texto != "grp" {
		t.Fatal(r.Texto)
	}
	if r := e.priv(pnAna, "!sologrupo"); !strings.Contains(r.Texto, "grupo") {
		t.Fatal(r.Texto)
	}
	ay := e.d.Ejecutar(Entrada{Persona: pnAna, Chat: grupo, EsGrupo: true, Texto: "!ayuda"}).Texto
	if strings.Contains(ay, "soloprivado") || !strings.Contains(ay, "sologrupo") {
		t.Fatalf("ayuda en grupo:\n%s", ay)
	}
}

func TestDuenoSoloDesdeMensajesGuardadosOApp(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.d.Registrar(Comando{Nombre: "apagar", SoloDueno: true, Ejecutar: func(*Contexto) (string, error) { return "apagado", nil }})
	// Un tercero (aunque tenga el rango más alto posible que se puede crear) no lo usa.
	e.rol("Casi", "rangos.asignar")
	_ = e.alm.AsignarRol(1, pnAna, e.ids["Casi"], 0, "yo")
	if r := e.priv(pnAna, "!apagar"); !strings.Contains(r.Texto, "solo lo usa el dueño") {
		t.Fatal(r.Texto)
	}
	// Alguien que escribe como "yo" pero NO por el camino del dueño tampoco.
	if r := e.d.Ejecutar(Entrada{Persona: almacen.PersonaDueno, Chat: grupo, EsGrupo: true, Texto: "!apagar"}); !strings.Contains(r.Texto, "solo lo usa el dueño") {
		t.Fatalf("persona 'yo' sin el camino del dueño: %q", r.Texto)
	}
	if r := e.dueno("!apagar"); r.Texto != "apagado" {
		t.Fatal(r.Texto)
	}
	acc, _ := e.alm.ListarAcciones(1, 10, 0)
	var denegadas int
	for _, a := range acc {
		if a.Resultado == "denegado" {
			denegadas++
		}
	}
	if denegadas < 2 {
		t.Fatalf("las negativas se registran: %+v", acc)
	}
}

func TestAsignarConJerarquiaYCaducidad(t *testing.T) {
	e := nuevoEntorno(t)
	e.rol("Alto", "rangos.asignar")
	e.rol("Medio", "rangos.asignar")
	e.rol("Bajo")
	_ = e.alm.AsignarRol(1, pnAna, e.ids["Alto"], 0, "yo")
	_ = e.alm.AsignarRol(1, pnLuis, e.ids["Medio"], 0, "yo")

	// Alto da Bajo por 24 horas a un número.
	r := e.priv(pnAna, "!asignar 5215599999999 Bajo 24h")
	if !strings.Contains(r.Texto, "Listo") || !strings.Contains(r.Texto, "24 horas") {
		t.Fatalf("%q", r.Texto)
	}
	rol, as, _ := e.alm.RolDePersona(1, "5215599999999@s.whatsapp.net", e.hora)
	if rol == nil || rol.Nombre != "Bajo" || as.CaducaEn == 0 || as.AsignadoPor != pnAna {
		t.Fatalf("%+v %+v", rol, as)
	}
	// Caduca solo.
	if rol, _, _ := e.alm.RolDePersona(1, "5215599999999@s.whatsapp.net", e.hora.Add(25*time.Hour)); rol != nil {
		t.Fatal("debía caducar")
	}
	// Medio no puede dar Alto ni tocar a Alto.
	if r := e.priv(pnLuis, "!asignar 5215599999999 Alto"); !strings.Contains(r.Texto, "rangos más bajos") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.priv(pnLuis, "!asignar "+"5215511111111"+" Bajo"); !strings.Contains(r.Texto, "rangos más bajos") {
		t.Fatalf("degradar a un superior: %q", r.Texto)
	}
	if r := e.priv(pnLuis, "!quitarrango 5215511111111"); !strings.Contains(r.Texto, "rangos más bajos") {
		t.Fatalf("quitar a un superior: %q", r.Texto)
	}
	// Rango que no existe, falta de datos, duración mala.
	if r := e.priv(pnAna, "!asignar 5215599999999 Inventado"); !strings.Contains(r.Texto, "No existe") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.priv(pnAna, "!asignar"); !strings.Contains(r.Texto, "a quién") {
		t.Fatalf("%q", r.Texto)
	}
	// Con mención en un grupo y con cita.
	r = e.d.Ejecutar(Entrada{Persona: pnAna, Chat: grupo, EsGrupo: true, Texto: "!asignar @123456789012345 Bajo 7d", Menciones: []string{pnRosa}})
	if !strings.Contains(r.Texto, "7 días") {
		t.Fatalf("%q", r.Texto)
	}
	if rol, _, _ := e.alm.RolDePersona(1, pnRosa, e.hora); rol == nil || rol.Nombre != "Bajo" {
		t.Fatal("la mención no se aplicó")
	}
	r = e.d.Ejecutar(Entrada{Persona: pnAna, Chat: grupo, EsGrupo: true, Texto: "!quitarrango", CitaAutor: pnRosa})
	if !strings.Contains(r.Texto, "Listo") {
		t.Fatalf("%q", r.Texto)
	}
	if rol, _, _ := e.alm.RolDePersona(1, pnRosa, e.hora); rol != nil {
		t.Fatal("la cita no quitó el rango")
	}
	// El dueño desde Mensajes guardados: nadie toca al Dueño, ni siquiera él mismo.
	if r := e.dueno("!asignar 5215511111111 Medio permanente"); !strings.Contains(r.Texto, "Listo") {
		t.Fatalf("%q", r.Texto)
	}
	if rol, _, _ := e.alm.RolDePersona(1, pnAna, e.hora); rol.Nombre != "Medio" {
		t.Fatal("el dueño sí puede degradar")
	}
	// !rango
	if r := e.priv(pnAna, "!rango"); !strings.Contains(r.Texto, "Medio") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.priv(pnRosa, "!rango"); !strings.Contains(r.Texto, "No tienes un rango") {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.dueno("!rango"); !strings.Contains(r.Texto, "Dueño") {
		t.Fatalf("%q", r.Texto)
	}
	// Sin la casilla, no puede asignar.
	if r := e.priv(pnRosa, "!asignar 5215599999999 Bajo"); !strings.Contains(r.Texto, "No tienes permiso") {
		t.Fatalf("%q", r.Texto)
	}
}

func TestConfirmacionDeUnSoloUsoPorCodigo(t *testing.T) {
	e := nuevoEntorno(t)
	hecho := 0
	_ = e.d.Registrar(Comando{Nombre: "expulsar", Casilla: "util.notas", Confirma: true, Resumen: "Expulsar a alguien",
		Ejecutar: func(cx *Contexto) (string, error) { hecho++; return "expulsado " + strings.Join(cx.Args, ","), nil }})
	e.rol("Mod", "util.notas")
	_ = e.alm.AsignarRol(1, pnAna, e.ids["Mod"], 0, "yo")

	r := e.priv(pnAna, "!expulsar 123")
	if !strings.Contains(r.Texto, "confirmación") || hecho != 0 {
		t.Fatalf("debía pedir confirmación: %q", r.Texto)
	}
	if len(e.codigo) != 1 {
		t.Fatal("el callback debía recibir el código")
	}
	cod := e.codigo[0].Codigo
	if strings.Contains(r.Texto, cod) {
		t.Fatal("el código NO va al chat de quien lo pide")
	}
	// Otra persona no puede usarlo.
	if r := e.priv(pnLuis, "!confirmar "+cod); !strings.Contains(r.Texto, "no sirve") || hecho != 0 {
		t.Fatalf("%q", r.Texto)
	}
	// Código malo.
	if r := e.priv(pnAna, "!confirmar ZZZZZZ"); !strings.Contains(r.Texto, "no sirve") {
		t.Fatalf("%q", r.Texto)
	}
	// El bueno ejecuta una vez.
	if r := e.priv(pnAna, "!confirmar "+strings.ToLower(cod)); r.Texto != "expulsado 123" || hecho != 1 {
		t.Fatalf("%q %d", r.Texto, hecho)
	}
	if r := e.priv(pnAna, "!confirmar "+cod); !strings.Contains(r.Texto, "no sirve") || hecho != 1 {
		t.Fatalf("segundo uso: %q", r.Texto)
	}
	// Vence.
	e.priv(pnAna, "!expulsar 9")
	cod2 := e.codigo[len(e.codigo)-1].Codigo
	e.hora = e.hora.Add(3 * time.Minute)
	if r := e.priv(pnAna, "!confirmar "+cod2); !strings.Contains(r.Texto, "no sirve") || hecho != 1 {
		t.Fatalf("vencido: %q", r.Texto)
	}
	// Desde la app: el dueño aprueba la acción de otra persona y la respuesta va al chat de quien la pidió.
	e.priv(pnAna, "!expulsar 77")
	cod3 := e.codigo[len(e.codigo)-1].Codigo
	txt, err := e.d.Confirmar(cod3, "")
	if err != nil || txt != "expulsado 77" || hecho != 2 {
		t.Fatalf("%q %v", txt, err)
	}
	if len(e.resp) != 1 || !strings.HasPrefix(e.resp[0], pnAna+"|M|") {
		t.Fatalf("la respuesta debía ir al chat de quien lo pidió: %v", e.resp)
	}
	if _, err := e.d.Confirmar(cod3, ""); err == nil {
		t.Fatal("el código de la app también es de un solo uso")
	}
	// Los permisos se revisan otra vez al confirmar.
	e.priv(pnAna, "!expulsar 5")
	cod4 := e.codigo[len(e.codigo)-1].Codigo
	_, _ = e.alm.QuitarRol(1, pnAna)
	if _, err := e.d.Confirmar(cod4, ""); err == nil || hecho != 2 {
		t.Fatalf("sin permiso al confirmar: %v hecho=%d", err, hecho)
	}
	// El dueño usa el mismo mecanismo desde Mensajes guardados.
	r = e.dueno("!expulsar 1")
	if !strings.Contains(r.Texto, "notificación") || strings.Contains(r.Texto, e.codigo[len(e.codigo)-1].Codigo) {
		t.Fatalf("dueño: %q", r.Texto)
	}
	if r := e.dueno("!confirmar " + e.codigo[len(e.codigo)-1].Codigo); r.Texto != "expulsado 1" {
		t.Fatalf("%q", r.Texto)
	}
}

func TestLimiteDeComandosPorMinuto(t *testing.T) {
	e := nuevoEntorno(t)
	id, _ := e.perm.GuardarRol(almacen.Rol{Nombre: "Lento", LimiteMin: 2})
	_ = e.alm.AsignarRol(1, pnAna, id, 0, "yo")
	for i := 0; i < 2; i++ {
		if r := e.priv(pnAna, "!rango"); !strings.Contains(r.Texto, "Lento") {
			t.Fatal(r.Texto)
		}
	}
	r := e.priv(pnAna, "!rango")
	if !r.Manejado || !strings.Contains(r.Texto, "muy rápido") {
		t.Fatalf("%+v", r)
	}
	if r := e.priv(pnAna, "!rango"); !r.Manejado || r.Texto != "" {
		t.Fatalf("después se calla: %+v", r)
	}
	e.hora = e.hora.Add(2 * time.Minute)
	if r := e.priv(pnAna, "!rango"); !strings.Contains(r.Texto, "Lento") {
		t.Fatal("tras el minuto vuelve")
	}
	// El Dueño no tiene límite.
	for i := 0; i < 80; i++ {
		if r := e.dueno("!rango"); r.Texto == "" {
			t.Fatal("dueño")
		}
	}
}

func TestRegistroDeAccionesNoGuardaTextos(t *testing.T) {
	e := nuevoEntorno(t)
	e.priv(pnAna, "!asignar 5215500000000 SecretoTextoDeUsuario")
	e.priv(pnAna, "!ayuda")
	acc, _ := e.alm.ListarAcciones(1, 50, 0)
	if len(acc) == 0 {
		t.Fatal("debe haber registro")
	}
	for _, a := range acc {
		campos := a.Quien + a.Accion + a.Objetivo + a.Chat + a.Resultado + a.Detalle
		if strings.Contains(campos, "Secreto") {
			t.Fatalf("el registro guardó texto del usuario: %+v", a)
		}
	}
}

func TestRegistrarRechazaRepetidosYPermiteModulos(t *testing.T) {
	e := nuevoEntorno(t)
	if err := e.d.Registrar(Comando{Nombre: "ayuda", Ejecutar: func(*Contexto) (string, error) { return "", nil }}); err == nil {
		t.Fatal("nombre repetido")
	}
	if err := e.d.Registrar(Comando{Nombre: "nuevo", Alias: []string{"help"}, Ejecutar: func(*Contexto) (string, error) { return "", nil }}); err == nil {
		t.Fatal("alias repetido")
	}
	if err := e.d.Registrar(Comando{Nombre: "mimodulo", Alias: []string{"mm"}, Ejecutar: func(*Contexto) (string, error) { return "hola", nil }}); err != nil {
		t.Fatal(err)
	}
	if r := e.priv(pnAna, "!MM"); r.Texto != "hola" {
		t.Fatalf("alias sin importar mayúsculas: %q", r.Texto)
	}
	e.d.Quitar("mimodulo")
	if e.d.Existe("!mm") || e.d.Existe("!mimodulo") {
		t.Fatal("quitar")
	}
	// Un error del comando se explica sin romper; un pánico no tumba el bot.
	_ = e.d.Registrar(Comando{Nombre: "falla", Ejecutar: func(*Contexto) (string, error) { return "", errors.New("algo salió mal") }})
	_ = e.d.Registrar(Comando{Nombre: "revienta", Ejecutar: func(*Contexto) (string, error) { panic("x") }})
	if r := e.priv(pnAna, "!falla"); r.Texto != "Algo salió mal." {
		t.Fatalf("%q", r.Texto)
	}
	if r := e.priv(pnAna, "!revienta"); !r.Manejado {
		t.Fatal("el pánico se atrapa")
	}
}
