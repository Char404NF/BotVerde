package almacen

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMigracion13Existe(t *testing.T) {
	a := nuevo(t)
	if a.Version() < 13 {
		t.Fatalf("falta la migración 13: versión %d", a.Version())
	}
	for _, tabla := range []string{"msg_bot_sesion", "flow_cola", "flow_vencido", "flow_noentendio", "formularios"} {
		var n int
		if err := a.db.QueryRow(`SELECT COUNT(*) FROM ` + tabla).Scan(&n); err != nil {
			t.Fatalf("tabla %s: %v", tabla, err)
		}
	}
	if _, err := a.db.Exec(`SELECT expira_seg, pila, fase, campo, reint, cierre FROM flow_sessions`); err != nil {
		t.Fatal(err)
	}
}

func TestSesionConCaducidadPropiaYExpiracionEnSilencio(t *testing.T) {
	a := nuevo(t)
	ahora := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	id, err := a.NuevaSesionConExpira(1, "g@g.us", pnAna, 1, 1, "m", false, ahora, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := a.SesionVigente(1, "g@g.us", pnAna, false, ahora.Add(9*time.Minute), 2*time.Hour); s == nil || s.ID != id || s.ExpiraSeg != 600 {
		t.Fatalf("a los 9 min sigue, con su caducidad propia aunque el valor general sea de 2 h: %+v", s)
	}
	if n := a.ContarSesionesGrupo(1, "g@g.us", ahora.Add(9*time.Minute), 2*time.Hour); n != 1 {
		t.Fatalf("%d", n)
	}
	if n := a.ContarSesionesGrupo(1, "g@g.us", ahora.Add(11*time.Minute), 2*time.Hour); n != 0 {
		t.Fatalf("vencida no ocupa lugar: %d", n)
	}
	// un barrido la cierra en silencio y cuenta dónde se quedó; conserva la hora de su último mensaje
	if n := a.ExpirarSesiones(1, ahora.Add(11*time.Minute), 2*time.Hour); n != 1 {
		t.Fatalf("%d", n)
	}
	s, _ := a.SesionPorID(1, id)
	if s.Estado != SesExpirada || s.Cierre != "silencio" || s.UltimaTS != ahora.Unix() {
		t.Fatalf("%+v", s)
	}
	est, _ := a.EstadisticasMenu(1, 1, "2026-01-01", "2026-12-31", false)
	c := map[string]int64{}
	for _, f := range est {
		c[f.Paso+"/"+f.Opcion] = f.Cuenta
	}
	if c["/expirado"] != 1 || c["m/abandono"] != 1 {
		t.Fatalf("%v", c)
	}
	if n := a.ExpirarSesiones(1, ahora.Add(time.Hour), 2*time.Hour); n != 0 {
		t.Fatal("no se cuenta dos veces")
	}
	// se puede retomar dentro de un plazo, y solo si no hubo otra conversación después
	if a.UltimaExpirada(1, "g@g.us", pnAna, 1, ahora.Add(-time.Hour)) == nil {
		t.Fatal("debía haber una conversación por retomar")
	}
	if a.UltimaExpirada(1, "g@g.us", pnAna, 1, ahora.Add(time.Minute)) != nil {
		t.Fatal("fuera del plazo")
	}
	if err := a.ReabrirSesion(s, ahora.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if v, _ := a.SesionVigente(1, "g@g.us", pnAna, false, ahora.Add(time.Hour+time.Minute), 2*time.Hour); v == nil || v.ID != id {
		t.Fatalf("%+v", v)
	}
	id2, _ := a.NuevaSesion(1, "g@g.us", pnLuis, 1, 1, "m", false, ahora)
	a.ExpirarSesiones(1, ahora.Add(3*time.Hour), 2*time.Hour)
	_, _ = a.NuevaSesion(1, "g@g.us", pnLuis, 1, 1, "m", false, ahora.Add(4*time.Hour))
	if a.UltimaExpirada(1, "g@g.us", pnLuis, 1, ahora) != nil {
		t.Fatalf("hay una conversación más nueva que la caducada (%d)", id2)
	}
}

func TestMensajesDelBotYConversacionVencida(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	if err := a.GuardarMsgBot(1, "g@g.us", "BOT1", 7, pnAna, 3, ahora); err != nil {
		t.Fatal(err)
	}
	r := a.MsgBotPorID(1, "g@g.us", "BOT1")
	if r == nil || r.SesionID != 7 || r.Persona != pnAna || r.MenuID != 3 {
		t.Fatalf("%+v", r)
	}
	if a.MsgBotPorID(1, "g@g.us", "OTRO") != nil || a.MsgBotPorID(1, "otro@g.us", "BOT1") != nil || a.MsgBotPorID(1, "g@g.us", "") != nil {
		t.Fatal("solo el grupo y el id exactos")
	}
	a.LimpiarMsgBot(1, 7, ahora.AddDate(0, 0, 8))
	if a.MsgBotPorID(1, "g@g.us", "BOT1") != nil {
		t.Fatal("se limpia con los días")
	}
	if !a.MarcarVencido(1, "g@g.us", pnAna, 3, ahora) || a.MarcarVencido(1, "g@g.us", pnAna, 3, ahora) {
		t.Fatal("una sola vez por persona y menú")
	}
	if !a.MarcarVencido(1, "g@g.us", pnLuis, 3, ahora) || !a.MarcarVencido(1, "g@g.us", pnAna, 4, ahora) {
		t.Fatal("otra persona u otro menú sí")
	}
	a.LimpiarVencido(1, "g@g.us", pnAna, 3)
	if !a.MarcarVencido(1, "g@g.us", pnAna, 3, ahora) {
		t.Fatal("tras una conversación nueva vuelve a avisar")
	}
}

func TestFilaDeEsperaDeUnGrupo(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	t1, nueva := a.Encolar(1, "g@g.us", pnAna, pnAna, "M1", 1, ahora)
	t2, _ := a.Encolar(1, "g@g.us", pnLuis, pnLuis, "M2", 1, ahora.Add(time.Second))
	if t1 != 1 || t2 != 2 || !nueva {
		t.Fatal(t1, t2)
	}
	if tr, n := a.Encolar(1, "g@g.us", pnAna, pnAna, "M3", 1, ahora); tr != 1 || n {
		t.Fatalf("repetida: %d %v", tr, n)
	}
	if cs := a.ChatsConCola(1); len(cs) != 1 || cs[0] != "g@g.us" {
		t.Fatal(cs)
	}
	s := a.SiguienteEnCola(1, "g@g.us")
	if s == nil || s.Persona != pnAna || s.MsgID != "M1" || s.MencionWA != pnAna {
		t.Fatalf("%+v", s)
	}
	a.QuitarDeCola(1, s.ID)
	if a.TurnoEnCola(1, "g@g.us", pnLuis) != 1 {
		t.Fatal("pasa al turno 1")
	}
	if n := a.LimpiarCola(1, time.Minute, ahora.Add(time.Hour)); n != 1 || a.SiguienteEnCola(1, "g@g.us") != nil {
		t.Fatal("la espera larga se descarta")
	}
}

func TestNoEntendioFormulariosYAdjuntosVanCifrados(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	_ = a.GuardarNoEntendio(1, 2, "m", "quiero la factura de María", ahora)
	f := Formulario{MenuID: 2, Paso: "f", Titulo: "Cotización", SesionID: 5, Chat: pnAna, Persona: pnAna, Nombre: "Ana Pérez",
		Campos: []CampoValor{{ID: "correo", Etiqueta: "Correo", Valor: "ana.secreta@correo.mx"}}}
	id, err := a.GuardarFormulario(1, f)
	if err != nil || id == 0 {
		t.Fatal(id, err)
	}
	aid, n, err := a.GuardarAdjuntoMenu(1, strings.NewReader("CONTENIDO-SECRETO-DE-LA-FOTO"))
	if err != nil || n != 28 {
		t.Fatal(n, err)
	}
	// nada en claro en la base ni en el disco
	var raw []byte
	_ = a.db.QueryRow(`SELECT texto_cifrado FROM flow_noentendio`).Scan(&raw)
	var raw2, raw3 []byte
	_ = a.db.QueryRow(`SELECT datos_cifrado FROM formularios`).Scan(&raw2)
	_ = a.db.QueryRow(`SELECT nombre_cifrado FROM formularios`).Scan(&raw3)
	for _, b := range [][]byte{raw, raw2, raw3} {
		if len(b) == 0 || bytes.Contains(b, []byte("factura")) || bytes.Contains(b, []byte("secreta")) || bytes.Contains(b, []byte("Pérez")) {
			t.Fatalf("debe ir cifrado: %q", b)
		}
	}
	abs, _ := a.Abs(RutaAdjuntoMenu(1, aid))
	disco, _ := os.ReadFile(abs)
	if len(disco) == 0 || bytes.Contains(disco, []byte("CONTENIDO-SECRETO")) {
		t.Fatal("el adjunto va cifrado en disco")
	}
	d, err := a.AbrirAdjuntoMenu(1, aid)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(d)
	d.Close()
	if string(b) != "CONTENIDO-SECRETO-DE-LA-FOTO" {
		t.Fatalf("%q", b)
	}
	// se leen de vuelta
	l, _ := a.ListarNoEntendio(1, 0, 10)
	if len(l) != 1 || l[0].Texto != "quiero la factura de María" {
		t.Fatalf("%+v", l)
	}
	g, _ := a.ObtenerFormulario(1, id)
	if g == nil || g.Nombre != "Ana Pérez" || g.Campos[0].Valor != "ana.secreta@correo.mx" || g.Titulo != "Cotización" {
		t.Fatalf("%+v", g)
	}
	if a.ContarFormularios(1, 2) != 1 || a.ContarFormularios(1, 9) != 0 || a.ContarFormularios(1, 0) != 1 {
		t.Fatal("conteo")
	}
	if fs, _ := a.ListarFormularios(1, 9, 10); len(fs) != 0 {
		t.Fatal("filtro por menú")
	}
	// exportar datos de la persona incluye sus formularios y olvidarla los borra
	ex, err := a.DatosDePersona(1, pnAna, ahora)
	if err != nil || len(ex.Formularios) != 1 {
		t.Fatalf("%v %+v", err, ex)
	}
	_, _ = a.Encolar(1, "g@g.us", pnAna, pnAna, "M1", 1, ahora)
	_ = a.GuardarMsgBot(1, "g@g.us", "B1", 1, pnAna, 1, ahora)
	a.MarcarVencido(1, "g@g.us", pnAna, 1, ahora)
	if _, err := a.OlvidarPersona(1, pnAna); err != nil {
		t.Fatal(err)
	}
	if a.ContarFormularios(1, 0) != 0 || a.TurnoEnCola(1, "g@g.us", pnAna) != 0 || a.MsgBotPorID(1, "g@g.us", "B1") != nil {
		t.Fatal("olvidar a la persona borra sus formularios, su turno y sus ligas")
	}
	// borrar
	id2, _ := a.GuardarFormulario(1, f)
	if err := a.BorrarFormulario(1, id2); err != nil || a.ContarFormularios(1, 0) != 0 {
		t.Fatal("borrar uno")
	}
	a.GuardarFormulario(1, f)
	a.GuardarFormulario(1, f)
	if n := a.BorrarFormularios(1, 0); n != 2 {
		t.Fatal(n)
	}
	a.BorrarAdjuntoMenu(1, aid)
	if _, err := os.Stat(abs); !os.IsNotExist(err) {
		t.Fatal("el adjunto se borra")
	}
}

func TestEliminarBotLimpiaLasTablasDeMenus(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	a.GuardarNoEntendio(1, 1, "m", "x", ahora)
	a.GuardarFormulario(1, Formulario{MenuID: 1, Chat: pnAna, Persona: pnAna, Campos: []CampoValor{{ID: "a", Valor: "b"}}})
	a.Encolar(1, "g@g.us", pnAna, "", "M", 1, ahora)
	a.GuardarMsgBot(1, "g@g.us", "B", 1, pnAna, 1, ahora)
	a.MarcarVencido(1, "g@g.us", pnAna, 1, ahora)
	aid, _, _ := a.GuardarAdjuntoMenu(1, strings.NewReader("x"))
	abs, _ := a.Abs(RutaAdjuntoMenu(1, aid))
	if err := a.EliminarBot(1); err != nil {
		t.Fatal(err)
	}
	for _, tabla := range []string{"flow_noentendio", "formularios", "flow_cola", "msg_bot_sesion", "flow_vencido"} {
		var n int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM ` + tabla).Scan(&n)
		if n != 0 {
			t.Fatalf("%s sin limpiar", tabla)
		}
	}
	if _, err := os.Stat(filepath.Dir(abs)); !os.IsNotExist(err) {
		t.Fatal("la carpeta de adjuntos de menús se borra")
	}
}

func TestAjustesDeMenusTienenValorPorDefecto(t *testing.T) {
	a := nuevo(t)
	for clave, esperado := range map[string]string{"menu_err_max": "3", "grupo_max_sesiones": "5", "grupo_expira_min": "10", "grupo_cualquiera": "0", "menu_volver": "reiniciar", "noentendio_dias": "30"} {
		if v := a.AjusteBot(1, clave); v != esperado {
			t.Fatalf("%s = %q, esperaba %q", clave, v, esperado)
		}
	}
	if a.AjusteBotInt(1, "grupo_msgs_min") <= 0 {
		t.Fatal("tope por minuto por grupo")
	}
	if err := a.FijarAjusteBot(1, "menu_err_max", "4"); err != nil || a.AjusteBotInt(1, "menu_err_max") != 4 {
		t.Fatal(err)
	}
}

func TestConversacionDeGrupoPasaALaPersonaCanonicaAlAprenderElAlias(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	lid := "9988776655@lid"
	id, _ := a.NuevaSesion(1, "g@g.us", lid, 1, 1, "m", false, ahora) // aún no se sabía que el LID es Ana
	a.Encolar(1, "g@g.us", lid, lid, "M1", 1, ahora)
	a.GuardarMsgBot(1, "g@g.us", "B1", id, lid, 1, ahora)
	a.AprenderAlias(1, pnAna, lid)
	s, _ := a.SesionPorID(1, id)
	if s.Sender != pnAna {
		t.Fatalf("la conversación pasa al número: %+v", s)
	}
	if a.TurnoEnCola(1, "g@g.us", pnAna) != 1 || a.MsgBotPorID(1, "g@g.us", "B1").Persona != pnAna {
		t.Fatal("fila y mensajes del bot también")
	}
	if v, _ := a.SesionVigente(1, "g@g.us", pnAna, false, ahora, time.Hour); v == nil || v.ID != id {
		t.Fatal("se encuentra por el número")
	}
}
