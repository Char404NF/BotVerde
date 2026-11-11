package flujo

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/permisos"
)

func menuConReglas() *Def {
	return &Def{V: 1, Nombre: "Reglas", Inicio: "m",
		Pasos: []Paso{
			{ID: "m", Tipo: PasoMenu, Titulo: "Menú", Opciones: []Opcion{
				{ID: "pub", Texto: "Información", Destino: "r1"},
				{ID: "vip", Texto: "Precios de mayoreo", Destino: "r2"},
				{ID: "priv", Texto: "Mis pedidos", Destino: "r3", Ambito: "privado"},
				{ID: "gru", Texto: "Reglas del grupo", Destino: "r4", Ambito: "grupo"},
				{ID: "hor", Texto: "Promoción de la mañana", Destino: "r5", Horario: &Horario{Desde: "10:00", Hasta: "12:00"}},
			}},
			{ID: "r1", Tipo: PasoRespuesta, Texto: "Info general."},
			{ID: "r2", Tipo: PasoRespuesta, Texto: "Mayoreo desde 10 piezas."},
			{ID: "r3", Tipo: PasoRespuesta, Texto: "Tus pedidos."},
			{ID: "r4", Tipo: PasoRespuesta, Texto: "Sé amable."},
			{ID: "r5", Tipo: PasoRespuesta, Texto: "2x1 hasta las 12."},
		},
		Disparadores: Disparadores{Palabras: []string{"reglas"}, Comando: ".menu"},
		Alcance:      Alcance{Privados: true, TodosGrupos: true}}
}

func TestVisibilidadDeOpcionesPorRangoAmbitoYHorario(t *testing.T) {
	// 05:30 en México: la promoción de las 10 a las 12 no se ofrece; en privado no hay opción de grupo
	e := nuevo(t)
	e.crea(t, menuConReglas(), true)
	d := e.dice(t, ana, ana, ".menu")
	if textos(d) != "Menú\n1. Información\n2. Precios de mayoreo\n3. Mis pedidos" {
		t.Fatalf("en privado, de mañana: %q", textos(d))
	}
	// por rango: al rango sin permiso se le oculta "vip" y los números se acomodan; lo oculto no se puede elegir ni escribiendo
	e.m.c.OpcionVisible = func(sender string, menuID int64, opcionID string) bool { return opcionID != "vip" }
	e.dice(t, ana, ana, "salir")
	d = e.dice(t, ana, ana, ".menu")
	if textos(d) != "Menú\n1. Información\n2. Mis pedidos" {
		t.Fatalf("sin la de mayoreo: %q", textos(d))
	}
	if d := e.dice(t, ana, ana, "Precios de mayoreo"); !strings.HasPrefix(textos(d), "No entendí") {
		t.Fatalf("lo oculto no se elige por texto: %q", textos(d))
	}
	if d := e.dice(t, ana, ana, "2"); textos(d) != "Tus pedidos." {
		t.Fatalf("los números siguen lo que ve: %q", textos(d))
	}
	// las estadísticas cuentan por el lugar en la definición, no por el número que vio esa persona
	est, _ := e.alm.EstadisticasMenu(1, 1, "2026-01-01", "2026-12-31", false)
	for _, f := range est {
		if f.Paso == "m" && f.Opcion == "3" && f.Cuenta == 1 {
			goto ok
		}
	}
	t.Fatalf("la opción 3 de la definición: %+v", est)
ok:
	// por horario: a las 10:30 sí aparece
	e.m.c.OpcionVisible = nil
	e.rel.t = time.Date(2026, 11, 10, 16, 30, 0, 0, time.UTC)
	e.dice(t, ana, ana, "salir")
	if d := e.dice(t, ana, ana, ".menu"); !strings.Contains(textos(d), "4. Promoción de la mañana") {
		t.Fatalf("dentro del horario de la opción: %q", textos(d))
	}
	// en un grupo: la de grupo aparece, la privada se marca y avisa "escríbeme al privado" sin escribir al privado
	g := "120363000000000001@g.us"
	ben := "5215533333333@s.whatsapp.net"
	d = e.dice(t, g, ben, "!menu")
	if textos(d) != "Menú\n1. Información\n2. Precios de mayoreo\n3. Mis pedidos (solo en privado)\n4. Reglas del grupo\n5. Promoción de la mañana" {
		t.Fatalf("en el grupo: %q", textos(d))
	}
	d = e.dice(t, g, ben, "3")
	if textos(d) != "Esa opción es solo en privado: escríbeme al privado." {
		t.Fatalf("opción privada: %q", textos(d))
	}
	if s := e.sesion(t, g, ben); s == nil || s.Paso != "m" {
		t.Fatalf("sigue en el menú: %+v", s)
	}
	if d := e.dice(t, g, ben, "4"); textos(d) != "Sé amable." {
		t.Fatalf("%q", textos(d))
	}
}

func TestVisibilidadPorOpcionConLosRangosReales(t *testing.T) {
	e := nuevo(t)
	id := e.crea(t, menuConReglas(), true)
	pid := fmtID(id)
	roles := map[string]*almacen.Rol{
		"todo":   {MenusModo: "todos"},
		"menu":   {MenusModo: "lista", Menus: []string{pid}},
		"sueltas": {MenusModo: "lista", Menus: []string{pid + ":pub", pid + ":priv"}},
		"ninguno": {MenusModo: "ninguno"},
	}
	visible := func(r *almacen.Rol, op string) bool { return permisos.OpcionVisible(r, pid, op) }
	if !visible(roles["todo"], "vip") || !visible(roles["menu"], "vip") || visible(roles["sueltas"], "vip") || !visible(roles["sueltas"], "pub") || visible(roles["ninguno"], "pub") {
		t.Fatal("reglas de opción")
	}
	// un rango con solo opciones sueltas sí ve el menú (con esas opciones)
	if !permisos.MenuVisible(roles["sueltas"], pid, "") || permisos.MenuVisible(roles["ninguno"], pid, "") {
		t.Fatal("el menú se ve si se ve al menos una opción")
	}
	e.m.c.OpcionVisible = func(sender string, menuID int64, opcionID string) bool {
		return permisos.OpcionVisible(roles["sueltas"], fmtID(menuID), opcionID)
	}
	if d := e.dice(t, ana, ana, ".menu"); textos(d) != "Menú\n1. Información\n2. Mis pedidos" {
		t.Fatalf("%q", textos(d))
	}
}

func fmtID(id int64) string { return strconv.FormatInt(id, 10) }

func TestTodasLasPlantillasSonValidasYEditables(t *testing.T) {
	ps := Plantillas()
	nombres := map[string]bool{}
	for _, p := range ps {
		nombres[p.Nombre] = true
		if p.Descripcion == "" || p.Def == nil {
			t.Fatalf("%q sin descripción", p.Nombre)
		}
		for _, a := range p.Def.Validar() {
			if a.Grave {
				t.Fatalf("plantilla %q: %+v", p.Nombre, a)
			}
		}
		// lo que guarda la app (ids y variables) sobrevive a un ciclo guardar → leer
		b := p.Def.JSON()
		d2, err := Parse(b)
		if err != nil || len(d2.Pasos) != len(p.Def.Pasos) {
			t.Fatalf("%q: %v", p.Nombre, err)
		}
		for _, paso := range d2.Pasos {
			for _, o := range paso.Opciones {
				if o.ID == "" {
					t.Fatalf("%q: opción sin id", p.Nombre)
				}
			}
		}
	}
	for _, esperado := range []string{"Refacciones", "Soporte", "Citas", "Preguntas frecuentes", "Menú para grupos"} {
		if !nombres[esperado] {
			t.Fatalf("falta la plantilla %q", esperado)
		}
	}
	// cada plantilla se puede usar de verdad: se activa y responde
	for _, p := range ps {
		e := nuevo(t)
		e.rel.t = time.Date(2026, 11, 10, 16, 0, 0, 0, time.UTC) // martes 10:00 en México: dentro de horario
		e.crea(t, p.Def, true)
		chat, sender := ana, ana
		if p.Def.Alcance.TodosGrupos {
			chat, sender = "120363000000000001@g.us", "5215533333333@s.whatsapp.net"
		}
		cmd := p.Def.Disparadores.Comando
		if cmd == "" {
			cmd = p.Def.Disparadores.Palabras[0]
		}
		if d := e.dice(t, chat, sender, cmd); len(d.Respuestas) == 0 {
			t.Fatalf("la plantilla %q no responde a %q: %+v", p.Nombre, cmd, d)
		}
	}
}

func TestPlantillaDeCitasFuncionaDeExtremoAExtremo(t *testing.T) {
	var cita *Def
	for _, p := range Plantillas() {
		if p.Nombre == "Citas" {
			cita = p.Def
		}
	}
	e := nuevo(t)
	e.rel.t = time.Date(2026, 11, 10, 16, 0, 0, 0, time.UTC)
	e.crea(t, cita, true)
	e.dice(t, ana, ana, ".citas")
	d := e.dice(t, ana, ana, "agendar")
	if !strings.Contains(textos(d), "(1 de 5) ¿A nombre de quién?") {
		t.Fatalf("%q", textos(d))
	}
	for _, r := range []string{"Ana Pérez", "55 9876 5432", "25 de diciembre", "tarde", "revisión"} {
		d = e.dice(t, ana, ana, r)
	}
	if !strings.Contains(textos(d), "• Fecha: 25/12/2026") || !strings.Contains(textos(d), "• Horario: Tarde") {
		t.Fatalf("%q", textos(d))
	}
	d = e.dice(t, ana, ana, "1")
	if textos(d) != "¡Listo! Apartamos tu cita para el 25/12/2026 en la Tarde. Te confirmaremos por este medio." {
		t.Fatalf("%q", textos(d))
	}
}

func TestExportarEImportarMenusCifrados(t *testing.T) {
	e := nuevo(t)
	d := tienda()
	d.Pasos = append(d.Pasos, Paso{ID: "foto", Tipo: PasoRespuesta, Texto: "Mira", Adjuntos: []AdjuntoMenu{{ID: "x", Tipo: "imagen", Mime: "image/jpeg", Nombre: "lista.jpg"}}})
	d.Pasos[0].Opciones = append(d.Pasos[0].Opciones, Opcion{Texto: "Catálogo", Destino: "foto"})
	d.Alcance.Chats = []string{ana}
	id, _, err := e.alm.GuardarAdjuntoMenu(1, bytes.NewReader([]byte("BYTES-DE-LA-FOTO")))
	if err != nil {
		t.Fatal(err)
	}
	d.Pasos[len(d.Pasos)-1].Adjuntos[0].ID = id
	mid := e.crea(t, d, true)
	otro := e.crea(t, &Def{V: 1, Nombre: "Otro", Pasos: []Paso{{ID: "a", Tipo: PasoRespuesta, Texto: "hola"}}, Disparadores: Disparadores{Cualquiera: true}, Alcance: Alcance{Privados: true}}, false)

	// con contraseña (se puede llevar a otro teléfono)
	arch, err := ExportarMenus(e.alm, 1, []int64{mid, otro}, "secreto1")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(arch, []byte("BCMX\x01\x01")) || bytes.Contains(arch, []byte("Tienda")) || bytes.Contains(arch, []byte("BYTES-DE-LA-FOTO")) {
		t.Fatal("el archivo debe ir cifrado y con su cabecera")
	}
	// otro teléfono (otra llave de datos) con la contraseña
	k2 := make([]byte, 32)
	for i := range k2 {
		k2[i] = byte(200 - i)
	}
	alm2, err := almacen.Abrir(t.TempDir(), k2)
	if err != nil {
		t.Fatal(err)
	}
	defer alm2.Cerrar()
	if _, err := ImportarMenus(alm2, 1, arch, "otra-clave"); err != ErrClaveExport {
		t.Fatalf("contraseña mala: %v", err)
	}
	if _, err := ImportarMenus(alm2, 1, arch, ""); err == nil || !strings.Contains(err.Error(), "contraseña") {
		t.Fatalf("falta la contraseña: %v", err)
	}
	res, err := ImportarMenus(alm2, 1, arch, "secreto1")
	if err != nil || len(res.IDs) != 2 || res.Nombres[0] != "Tienda" || res.Nombres[1] != "Otro" {
		t.Fatalf("%+v %v", res, err)
	}
	ms, _ := alm2.ListarMenus(1)
	if len(ms) != 2 {
		t.Fatalf("%d", len(ms))
	}
	for _, m := range ms {
		if m.Activo {
			t.Fatal("lo importado queda apagado: no se ejecuta nada solo")
		}
	}
	di, _ := Parse(ms[0].Def)
	if len(di.Alcance.Chats) != 0 {
		t.Fatal("los chats de otro teléfono no se importan")
	}
	nuevoID := di.Paso("foto").Adjuntos[0].ID
	if nuevoID == "" || nuevoID == id {
		t.Fatalf("el adjunto se guarda con id nuevo: %q", nuevoID)
	}
	if b, err := alm2.LeerAdjuntoMenu(1, nuevoID); err != nil || string(b) != "BYTES-DE-LA-FOTO" {
		t.Fatalf("el adjunto llegó completo: %q %v", b, err)
	}
	// con la llave del teléfono (copia de seguridad): se abre en el mismo, no en otro
	loc, err := ExportarMenus(e.alm, 1, []int64{mid}, "")
	if err != nil || !bytes.HasPrefix(loc, []byte("BCMX\x01\x00")) {
		t.Fatal(err)
	}
	if r, err := ImportarMenus(e.alm, 1, loc, ""); err != nil || len(r.IDs) != 1 {
		t.Fatalf("%+v %v", r, err)
	}
	if _, err := ImportarMenus(alm2, 1, loc, ""); err == nil || !strings.Contains(err.Error(), "otro teléfono") {
		t.Fatalf("%v", err)
	}
	// validaciones de forma y versión
	for nombre, malo := range map[string][]byte{
		"vacío": nil, "basura": []byte("hola mundo esto no es un archivo"), "magia": append([]byte("XXXX\x01\x00"), arch[6:]...),
		"versión": append([]byte("BCMX\x09"), arch[5:]...), "modo": append([]byte("BCMX\x01\x07"), arch[6:]...), "cortado": arch[:20],
	} {
		if _, err := ImportarMenus(e.alm, 1, malo, "secreto1"); err == nil {
			t.Fatalf("%s: debía rechazarse", nombre)
		}
	}
	// alterar un byte lo detecta
	roto := append([]byte(nil), arch...)
	roto[len(roto)-3] ^= 0xFF
	if _, err := ImportarMenus(alm2, 1, roto, "secreto1"); err == nil {
		t.Fatal("un archivo alterado se rechaza")
	}
	// reglas al exportar
	if _, err := ExportarMenus(e.alm, 1, nil, ""); err == nil {
		t.Fatal("sin menús")
	}
	if _, err := ExportarMenus(e.alm, 1, []int64{mid}, "123"); err == nil {
		t.Fatal("contraseña corta")
	}
	if _, err := ExportarMenus(e.alm, 1, []int64{9999}, ""); err == nil {
		t.Fatal("menú inexistente")
	}
}

func TestLoQueNoEntendioSeGuardaCifradoConRetencionYBorrado(t *testing.T) {
	e := nuevo(t)
	e.crea(t, tienda(), true)
	e.dice(t, ana, ana, ".menu")
	e.dice(t, ana, ana, "quiero una pizza")
	e.dice(t, ana, ana, "con piña")
	l, _ := e.alm.ListarNoEntendio(1, 1, 10)
	if len(l) != 2 || l[0].Texto != "con piña" || l[1].Texto != "quiero una pizza" || l[0].Paso != "m" {
		t.Fatalf("%+v", l)
	}
	// lo ambiguo, los saludos y los comandos no son "no entendió"
	e.dice(t, ana, ana, ".menu")
	e.dice(t, ana, ana, "precios")
	e.dice(t, ana, ana, "hola")
	e.dice(t, ana, ana, "menú")
	if e.alm.ContarNoEntendio(1, 1) != 2 {
		t.Fatalf("solo lo que de verdad no se entendió: %d", e.alm.ContarNoEntendio(1, 1))
	}
	// se puede apagar
	e.conf(t, "noentendio_activo", "0")
	e.dice(t, ana, ana, "otra cosa rara")
	if e.alm.ContarNoEntendio(1, 1) != 2 {
		t.Fatal("apagado no guarda")
	}
	e.conf(t, "noentendio_activo", "1")
	// retención: lo viejo se borra
	e.rel.t = e.rel.t.AddDate(0, 0, 40)
	e.alm.GuardarNoEntendio(1, 1, "m", "reciente", e.rel.t)
	if n := e.alm.LimpiarNoEntendio(1, 30, e.rel.t); n != 2 {
		t.Fatalf("se borran los de más de 30 días: %d", n)
	}
	if l, _ := e.alm.ListarNoEntendio(1, 0, 10); len(l) != 1 || l[0].Texto != "reciente" {
		t.Fatalf("%+v", l)
	}
	if n := e.alm.LimpiarNoEntendio(1, 0, e.rel.t); n != 0 {
		t.Fatal("0 días = no se borra solo")
	}
	if n := e.alm.BorrarNoEntendio(1, 1); n != 1 || e.alm.ContarNoEntendio(1, 0) != 0 {
		t.Fatal("botón borrar")
	}
	// en un grupo, lo que no es para el bot no se guarda
	g := "120363000000000001@g.us"
	ben := "5215533333333@s.whatsapp.net"
	d := tienda()
	d.Alcance = Alcance{TodosGrupos: true}
	d.Disparadores = Disparadores{Mencion: true}
	e.crea(t, d, true)
	e.dice(t, g, ben, "tienda")
	n0 := e.alm.ContarNoEntendio(1, 0)
	r := e.dice(t, g, ben, "esto no es una opción")
	if len(r.Respuestas) == 0 || e.alm.ContarNoEntendio(1, 0) != n0+1 {
		t.Fatalf("dirigido al bot (menciona): sí se guarda: %+v", r)
	}
}

func TestEstadisticasDeAbandonoPorPasoYPorErrores(t *testing.T) {
	e := nuevo(t)
	e.crea(t, tienda(), true)
	// 3 errores en el menú principal → abandona ahí
	e.dice(t, ana, ana, ".menu")
	e.dice(t, ana, ana, "aaaa")
	e.dice(t, ana, ana, "bbbb")
	e.dice(t, ana, ana, "cccc")
	// sale del submenú
	e.dice(t, ana, ana, ".menu")
	e.dice(t, ana, ana, "contacto")
	e.dice(t, ana, ana, "salir")
	est, _ := e.alm.EstadisticasMenu(1, 1, "2026-01-01", "2026-12-31", false)
	c := map[string]int64{}
	for _, f := range est {
		c[f.Paso+"/"+f.Opcion] = f.Cuenta
	}
	if c["m/abandono"] != 1 || c["c/abandono"] != 1 || c["/abandonado"] != 2 || c["m/invalida"] != 3 {
		t.Fatalf("%v", c)
	}
}
