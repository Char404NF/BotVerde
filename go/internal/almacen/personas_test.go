package almacen

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMigracion12Existe(t *testing.T) {
	a := nuevo(t)
	if a.Version() < 12 {
		t.Fatalf("falta la migración 12: versión %d", a.Version())
	}
	for _, tabla := range []string{"persona_silencio", "advertencias", "notas_persona", "etiquetas", "chat_etiqueta", "recordatorios", "alertas_palabra"} {
		var n int
		if err := a.db.QueryRow(`SELECT COUNT(*) FROM ` + tabla).Scan(&n); err != nil {
			t.Fatalf("tabla %s: %v", tabla, err)
		}
	}
	if _, err := a.db.Exec(`SELECT estado_trabajo FROM chat_estado`); err != nil {
		t.Fatal(err)
	}
}

func TestSilencioConCaducidadYSiempre(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	if err := a.FijarSilencio(1, pnAna, ahora.Add(time.Hour).Unix(), "mucho ruido", "yo", ahora); err != nil {
		t.Fatal(err)
	}
	if s := a.SilencioDe(1, pnAna, ahora.Add(30*time.Minute)); s == nil || s.Motivo != "mucho ruido" {
		t.Fatalf("debía seguir silenciada: %+v", s)
	}
	if a.SilencioDe(1, pnAna, ahora.Add(2*time.Hour)) != nil {
		t.Fatal("el silencio venció")
	}
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM persona_silencio`).Scan(&n)
	if n != 0 {
		t.Fatal("el vencido se limpia")
	}
	// Para siempre.
	_ = a.FijarSilencio(1, pnLuis, 0, "", "yo", ahora)
	if a.SilencioDe(1, pnLuis, ahora.Add(1000*time.Hour)) == nil {
		t.Fatal("siempre es siempre")
	}
	if ok, _ := a.QuitarSilencio(1, pnLuis); !ok || a.SilencioDe(1, pnLuis, ahora) != nil {
		t.Fatal("quitar el silencio")
	}
	if err := a.FijarSilencio(1, PersonaDueno, 0, "", "yo", ahora); err == nil {
		t.Fatal("al Dueño no se le silencia")
	}
}

func TestSilencioPorAliasNumeroYLID(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	a.AprenderAlias(1, pnAna, lidAna)
	// Se silencia por el LID: vale para el número y al revés.
	if err := a.FijarSilencio(1, lidAna, 0, "", "yo", ahora); err != nil {
		t.Fatal(err)
	}
	if a.SilencioDe(1, pnAna, ahora) == nil || a.SilencioDe(1, lidAna, ahora) == nil {
		t.Fatal("el silencio debe valer para ambas formas")
	}
}

func TestBloqueoUsaElEstadoDeSolicitudes(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	if err := a.BloquearPersona(1, pnAna, ahora); err != nil {
		t.Fatal(err)
	}
	s, _ := a.ObtenerSolicitud(1, pnAna)
	if s == nil || s.Estado != SolBloqueada || !a.EstaBloqueada(1, pnAna) {
		t.Fatalf("bloquear = estado bloqueada de solicitudes: %+v", s)
	}
	if ok, err := a.DesbloquearPersona(1, pnAna); !ok || err != nil {
		t.Fatalf("desbloquear: %v %v", ok, err)
	}
	if a.EstaBloqueada(1, pnAna) {
		t.Fatal("ya no está bloqueada")
	}
	if ok, _ := a.DesbloquearPersona(1, pnAna); ok {
		t.Fatal("no estaba bloqueada")
	}
	// Una aprobada no se toca al "desbloquear".
	_ = a.DecidirSolicitud(1, pnLuis, SolAprobada, ahora)
	if ok, _ := a.DesbloquearPersona(1, pnLuis); ok {
		t.Fatal("una aprobada no es una bloqueada")
	}
}

func TestAdvertenciasMotivoCifradoYQuitar(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	a.AprenderAlias(1, pnAna, lidAna)
	_, n1, err := a.AgregarAdvertencia(1, pnAna, "g@g.us", "mandó spam secreto", "yo", ahora)
	if err != nil || n1 != 1 {
		t.Fatal(n1, err)
	}
	id2, n2, _ := a.AgregarAdvertencia(1, lidAna, "g@g.us", "", "yo", ahora.Add(time.Minute)) // por el LID cuenta igual
	if n2 != 2 {
		t.Fatalf("alias: %d", n2)
	}
	// El motivo no está en claro en la base.
	var motivo []byte
	_ = a.db.QueryRow(`SELECT motivo_cifrado FROM advertencias WHERE motivo_cifrado IS NOT NULL`).Scan(&motivo)
	if len(motivo) == 0 || strings.Contains(string(motivo), "spam") {
		t.Fatal("el motivo debe ir cifrado")
	}
	l, _ := a.ListarAdvertencias(1, pnAna, 10)
	if len(l) != 2 || l[0].ID != id2 || l[1].Motivo != "mandó spam secreto" {
		t.Fatalf("lista: %+v", l)
	}
	quedan, quitada, _ := a.QuitarAdvertencia(1, pnAna, 0)
	if !quitada || quedan != 1 {
		t.Fatalf("quitar la última: %d %v", quedan, quitada)
	}
	if l, _ = a.ListarAdvertencias(1, pnAna, 10); len(l) != 1 || l[0].Motivo == "" {
		t.Fatalf("debía quedar la primera: %+v", l)
	}
	if _, q, _ := a.QuitarAdvertencia(1, pnAna, 9999); q {
		t.Fatal("id inexistente")
	}
	_, _, _ = a.AgregarAdvertencia(1, pnAna, "", "", "yo", ahora)
	if n, _ := a.BorrarAdvertencias(1, lidAna); n != 2 || a.ContarAdvertencias(1, pnAna) != 0 {
		t.Fatalf("borrar historial: %d", n)
	}
	if _, _, err := a.AgregarAdvertencia(1, PersonaDueno, "", "", "yo", ahora); err == nil {
		t.Fatal("no se advierte al Dueño")
	}
}

func TestNotasCifradas(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	if _, err := a.AgregarNota(1, pnAna, "   ", "yo", ahora); err == nil {
		t.Fatal("nota vacía")
	}
	id, err := a.AgregarNota(1, pnAna, "paga siempre tarde", "yo", ahora)
	if err != nil {
		t.Fatal(err)
	}
	var b []byte
	_ = a.db.QueryRow(`SELECT texto_cifrado FROM notas_persona WHERE id=?`, id).Scan(&b)
	if len(b) == 0 || strings.Contains(string(b), "tarde") {
		t.Fatal("la nota debe ir cifrada")
	}
	ns, _ := a.ListarNotas(1, pnAna)
	if len(ns) != 1 || ns[0].Texto != "paga siempre tarde" || a.ContarNotas(1, pnAna) != 1 {
		t.Fatalf("%+v", ns)
	}
	if ok, _ := a.BorrarNota(1, pnLuis, id); ok {
		t.Fatal("no se borra la nota de otra persona")
	}
	if ok, _ := a.BorrarNota(1, pnAna, id); !ok || a.ContarNotas(1, pnAna) != 0 {
		t.Fatal("borrar nota")
	}
}

func TestAprenderAliasUneAdvertenciasNotasYSilencio(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	// Antes de saber que son la misma persona, cada forma tiene lo suyo.
	_, _, _ = a.AgregarAdvertencia(1, lidAna, "", "x", "yo", ahora)
	_, _ = a.AgregarNota(1, lidAna, "nota del LID", "yo", ahora)
	_ = a.FijarSilencio(1, lidAna, 0, "", "yo", ahora)
	_, _, _ = a.AgregarAdvertencia(1, pnAna, "", "y", "yo", ahora)
	_ = a.FijarSilencio(1, pnAna, ahora.Add(time.Hour).Unix(), "", "yo", ahora)
	a.AprenderAlias(1, pnAna, lidAna)
	if a.ContarAdvertencias(1, pnAna) != 2 || a.ContarNotas(1, pnAna) != 1 {
		t.Fatalf("adv=%d notas=%d", a.ContarAdvertencias(1, pnAna), a.ContarNotas(1, pnAna))
	}
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM advertencias WHERE persona=?`, lidAna).Scan(&n)
	if n != 0 {
		t.Fatal("las advertencias pasan a la persona canónica")
	}
	if s := a.SilencioDe(1, lidAna, ahora); s == nil || s.HastaTS != 0 {
		t.Fatalf("de los dos silencios gana el más largo (siempre): %+v", s)
	}
}

func llenaPersona(t *testing.T, a *Almacen) {
	t.Helper()
	ahora := time.Now().Unix()
	chatBasico(t, a, pnAna, "5215511111111", "Ana Pérez", false)
	chatBasico(t, a, pnLuis, "5215522222222", "Luis", false)
	chatBasico(t, a, "g@g.us", "", "Grupo", true)
	conArchivo(t, a, pnAna, "F1", TipoImagen, 1000, ahora-100)
	m := msg(pnAna, "T1", ahora-50, "hola, necesito precios")
	_, _ = a.GuardarMensaje(m)
	mio := msg(pnAna, "T2", ahora-40, "claro, te paso la lista")
	mio.EsMio, mio.RemitenteJID = true, ""
	_, _ = a.GuardarMensaje(mio)
	g := msg("g@g.us", "G1", ahora-30, "mensaje de Ana en el grupo")
	g.RemitenteJID = pnAna
	_, _ = a.GuardarMensaje(g)
	g2 := msg("g@g.us", "G2", ahora-20, "mensaje de Luis en el grupo")
	g2.RemitenteJID = pnLuis
	_, _ = a.GuardarMensaje(g2)
	l := msg(pnLuis, "L1", ahora-10, "hola soy Luis")
	_, _ = a.GuardarMensaje(l)
}

func TestFichaYEstadisticasDePersona(t *testing.T) {
	a := nuevo(t)
	llenaPersona(t, a)
	a.AprenderAlias(1, pnAna, lidAna)
	ahora := time.Now()
	_, _, _ = a.AgregarAdvertencia(1, pnAna, "", "", "yo", ahora)
	_, _ = a.AgregarNota(1, pnAna, "n", "yo", ahora)
	_ = a.FijarSilencio(1, pnAna, 0, "", "yo", ahora)
	ide, _ := a.CrearEtiqueta(1, "Cliente", "#FF0000")
	_ = a.PonerEtiqueta(1, pnAna, ide, true)
	f, err := a.FichaDe(1, lidAna, ahora) // pidiendo por el LID
	if err != nil {
		t.Fatal(err)
	}
	if f.Persona != pnAna || f.Nombre != "Ana Pérez" || len(f.Alias) != 2 || f.Advertencias != 1 || f.Notas != 1 || f.Silencio == nil || len(f.Etiquetas) != 1 {
		t.Fatalf("%+v", f)
	}
	// Recibidos: 1 imagen + 1 texto en privado + 1 en grupo = 3; enviados: 1.
	if f.Recibidos != 3 || f.Enviados != 1 || f.Grupos != 1 || f.PrimeraTS == 0 || f.UltimaTS < f.PrimeraTS {
		t.Fatalf("conteos: %+v", f)
	}
	e, _ := a.EstadisticasDePersona(1, pnAna, 30)
	if e.Total != 3 || e.PorTipo[TipoImagen] != 1 || e.PorTipo[TipoTexto] != 2 || len(e.Chats) != 2 || e.Chats[0].Cuenta < 1 {
		t.Fatalf("%+v", e)
	}
	var horas int
	for _, h := range e.PorHora {
		horas += h
	}
	if horas != 3 || len(e.PorDia) < 1 {
		t.Fatalf("horas/dias: %+v", e)
	}
}

func TestOlvidarPersonaBorraTodoMenosNoMolestarYBloqueo(t *testing.T) {
	a := nuevo(t)
	llenaPersona(t, a)
	ahora := time.Now()
	a.AprenderAlias(1, pnAna, lidAna)
	_, _, _ = a.AgregarAdvertencia(1, pnAna, "", "", "yo", ahora)
	_, _ = a.AgregarNota(1, pnAna, "n", "yo", ahora)
	_ = a.FijarSilencio(1, pnAna, 0, "", "yo", ahora)
	_, _, _ = a.AgregarNoMolestar(1, pnAna, "BAJA", ahora)
	_ = a.BloquearPersona(1, pnAna, ahora)
	_, _ = a.CrearAviso(1, "palabra", pnAna, "t", "d")
	a.RegistrarAccion(1, "yo", "persona.nota", pnAna, pnAna, "ok", "")
	if _, err := a.CrearMenu(1, []byte(`{"v":1}`), true, false); err != nil {
		t.Fatal(err)
	}
	sid, _ := a.NuevaSesion(1, pnAna, pnAna, 1, 1, "p1", false, ahora)
	_ = a.GuardarVariable(sid, "dato1", "secreto")
	dir := a.raiz + "/media/1/" + HashChat(1, pnAna)
	if _, err := os.Stat(dir); err != nil {
		t.Fatal("debía haber archivos")
	}
	r, err := a.OlvidarPersona(1, lidAna) // por cualquiera de sus formas
	if err != nil {
		t.Fatal(err)
	}
	if r.Mensajes != 4 || r.Notas != 1 || r.Advertencias != 1 || r.Sesiones != 1 || r.Alias < 2 {
		t.Fatalf("%+v", r)
	}
	if a.ExisteChat(1, pnAna) || a.TieneMensajes(1, pnAna) {
		t.Fatal("su chat se borra")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("sus archivos se borran")
	}
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE remitente_jid=?`, pnAna).Scan(&n)
	if n != 0 {
		t.Fatal("lo que escribió en grupos se borra")
	}
	// El mensaje de Luis en el grupo y su chat siguen.
	if !a.TieneMensajes(1, pnLuis) {
		t.Fatal("el chat de Luis no se toca")
	}
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE chat_jid='g@g.us'`).Scan(&n)
	if n != 1 {
		t.Fatalf("en el grupo queda solo Luis: %d", n)
	}
	for tabla, donde := range map[string]string{"notas_persona": "persona", "advertencias": "persona", "persona_silencio": "persona", "daily_stats": "sender"} {
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM `+tabla+` WHERE `+donde+` IN (?,?)`, pnAna, lidAna).Scan(&n)
		if n != 0 {
			t.Fatalf("%s no se limpió", tabla)
		}
	}
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM persona_alias WHERE bot_id=1`).Scan(&n)
	if n != 0 {
		t.Fatal("los alias se borran")
	}
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM flow_sessions`).Scan(&n)
	if n != 0 {
		t.Fatal("sus sesiones de menú se borran")
	}
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM flow_vars`).Scan(&n)
	if n != 0 {
		t.Fatal("sus datos de menú se borran")
	}
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM avisos`).Scan(&n)
	if n != 0 {
		t.Fatal("sus avisos se borran")
	}
	acs, _ := a.ListarAcciones(1, 10, 0)
	for _, x := range acs {
		if x.Objetivo == pnAna || x.Chat == pnAna {
			t.Fatalf("el registro ya no dice quién era: %+v", x)
		}
	}
	// Se mantiene en No molestar y bloqueada (guardan solo el número).
	if !a.EstaNoMolestar(1, pnAna) || !a.EstaBloqueada(1, pnAna) {
		t.Fatal("No molestar y el bloqueo se conservan")
	}
	if _, err := a.OlvidarPersona(1, PersonaDueno); err == nil {
		t.Fatal("no se olvida al Dueño")
	}
}

func TestExportarPersonaCifradoBCF1(t *testing.T) {
	a := nuevo(t)
	llenaPersona(t, a)
	ahora := time.Now()
	_, _, _ = a.AgregarAdvertencia(1, pnAna, "", "mal", "yo", ahora)
	_, _ = a.AgregarNota(1, pnAna, "nota exportable", "yo", ahora)
	ex, err := a.DatosDePersona(1, pnAna, ahora)
	if err != nil {
		t.Fatal(err)
	}
	if len(ex.Mensajes) != 4 || len(ex.Notas) != 1 || len(ex.Advertencias) != 1 || ex.Ficha.Nombre != "Ana Pérez" {
		t.Fatalf("%d mensajes, %+v", len(ex.Mensajes), ex.Ficha)
	}
	for _, m := range ex.Mensajes {
		if m.Chat == "g@g.us" && m.ID == "G2" {
			t.Fatal("no debe traer lo que escribió otra persona en el grupo")
		}
	}
	b, _ := json.Marshal(ex)
	rel, err := a.EscribirExporte(1, pnAna, b, ahora)
	if err != nil {
		t.Fatal(err)
	}
	abs, _ := a.Abs(rel)
	crudo, _ := os.ReadFile(abs)
	if !bytes.HasPrefix(crudo, []byte("BCF1")) {
		t.Fatalf("debe ser BCF1: %q", crudo[:8])
	}
	if bytes.Contains(crudo, []byte("nota exportable")) || bytes.Contains(crudo, []byte("precios")) {
		t.Fatal("el archivo no puede tener texto en claro")
	}
	plano, err := a.LeerCifrado(rel)
	if err != nil || !strings.Contains(string(plano), "nota exportable") || !strings.Contains(string(plano), "necesito precios") {
		t.Fatalf("al descifrar: %v", err)
	}
	// Un segundo export de la misma persona reemplaza al anterior; los viejos se limpian.
	rel2, _ := a.EscribirExporte(1, pnAna, b, ahora.Add(5*time.Second))
	if rel2 == rel {
		t.Skip("mismo segundo")
	}
	if _, err := os.Stat(abs); !os.IsNotExist(err) {
		t.Fatal("el export anterior debía reemplazarse")
	}
	_ = os.Chtimes(func() string { p, _ := a.Abs(rel2); return p }(), ahora.Add(-48*time.Hour), ahora.Add(-48*time.Hour))
	a.LimpiarExportes(1, 24, ahora)
	if p, _ := a.Abs(rel2); func() bool { _, e := os.Stat(p); return !os.IsNotExist(e) }() {
		t.Fatal("el export de hace 2 días se borra")
	}
}

func TestEstadoTrabajoYEtiquetas(t *testing.T) {
	a := nuevo(t)
	llenaPersona(t, a)
	if err := a.FijarEstadoTrabajo(1, pnAna, "raro"); err == nil {
		t.Fatal("estado inválido")
	}
	_ = a.FijarEstadoTrabajo(1, pnAna, TrabajoPendiente)
	_ = a.FijarEstadoTrabajo(1, pnLuis, TrabajoResuelto)
	if a.EstadoDeChat(1, pnAna).EstadoTrabajo != TrabajoPendiente || a.ContarPendientes(1) != 1 {
		t.Fatal("pendiente")
	}
	l, _ := a.ListarChatsFiltrados(1, false, "", FiltroChats{Estado: TrabajoPendiente}, 50, 0)
	if len(l) != 1 || l[0].JID != pnAna || l[0].EstadoTrabajo != TrabajoPendiente {
		t.Fatalf("filtro pendiente: %+v", l)
	}
	l, _ = a.ListarChatsFiltrados(1, false, "", FiltroChats{Estado: TrabajoResuelto}, 50, 0)
	if len(l) != 1 || l[0].JID != pnLuis {
		t.Fatalf("filtro resuelto: %+v", l)
	}
	if l, _ = a.ListarChatsFiltrados(1, false, "", FiltroChats{}, 50, 0); len(l) != 2 {
		t.Fatalf("sin filtro: %d", len(l))
	}
	_ = a.FijarEstadoTrabajo(1, pnAna, TrabajoNinguno)
	if a.ContarPendientes(1) != 0 {
		t.Fatal("quitar la marca")
	}
	// Etiquetas
	if _, err := a.CrearEtiqueta(1, "  ", ""); err == nil {
		t.Fatal("sin nombre")
	}
	if _, err := a.CrearEtiqueta(1, "x", "rojo"); err == nil {
		t.Fatal("color inválido")
	}
	vip, err := a.CrearEtiqueta(1, "VIP", "#FFAA00")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.CrearEtiqueta(1, "vip", ""); err == nil {
		t.Fatal("nombre repetido sin importar mayúsculas")
	}
	prov, _ := a.CrearEtiqueta(1, "Proveedores", "")
	_ = a.PonerEtiqueta(1, pnAna, vip, true)
	_ = a.PonerEtiqueta(1, pnAna, prov, true)
	_ = a.PonerEtiqueta(1, pnLuis, prov, true)
	if err := a.PonerEtiqueta(1, pnLuis, 999, true); err == nil {
		t.Fatal("etiqueta inexistente")
	}
	l, _ = a.ListarChatsFiltrados(1, false, "", FiltroChats{Etiqueta: vip}, 50, 0)
	if len(l) != 1 || l[0].JID != pnAna || len(l[0].Etiquetas) != 2 {
		t.Fatalf("filtro etiqueta: %+v", l)
	}
	l, _ = a.ListarChatsFiltrados(1, false, "", FiltroChats{Etiqueta: prov, Estado: TrabajoResuelto}, 50, 0)
	if len(l) != 1 || l[0].JID != pnLuis {
		t.Fatalf("etiqueta + estado: %+v", l)
	}
	if err := a.EditarEtiqueta(1, vip, "Cliente VIP", "#00FF00"); err != nil {
		t.Fatal(err)
	}
	if err := a.EditarEtiqueta(1, vip, "proveedores", ""); err == nil {
		t.Fatal("renombrar a un nombre que ya existe")
	}
	ets, _ := a.ListarEtiquetas(1)
	if len(ets) != 2 || ets[0].Nombre != "Cliente VIP" || ets[0].Chats != 1 || ets[1].Chats != 2 {
		t.Fatalf("%+v", ets)
	}
	_ = a.PonerEtiqueta(1, pnAna, prov, false)
	if e, _ := a.EtiquetasDeChat(1, pnAna); len(e) != 1 {
		t.Fatalf("quitar etiqueta: %+v", e)
	}
	// Borrar la etiqueta la quita de los chats; borrar un chat quita sus etiquetas.
	_ = a.BorrarEtiqueta(1, prov)
	if e, _ := a.EtiquetasDeChat(1, pnLuis); len(e) != 0 {
		t.Fatal("la etiqueta borrada sale de los chats")
	}
	_, _ = a.BorrarChat(1, pnAna)
	var n int
	_ = a.db.QueryRow(`SELECT COUNT(*) FROM chat_etiqueta`).Scan(&n)
	if n != 0 {
		t.Fatal("al borrar el chat se limpian sus etiquetas")
	}
}

func TestRecordatoriosCifradosRepeticionYVencidos(t *testing.T) {
	a := nuevo(t)
	ahora := time.Date(2026, 3, 10, 9, 0, 0, 0, a.loc())
	if _, err := a.CrearRecordatorio(1, "", ahora.Unix(), "", ahora); err == nil {
		t.Fatal("sin texto")
	}
	if _, err := a.CrearRecordatorio(1, "x", ahora.Unix(), "mensual", ahora); err == nil {
		t.Fatal("repetición inválida")
	}
	id1, err := a.CrearRecordatorio(1, "llamar al contador", ahora.Add(time.Hour).Unix(), "", ahora)
	if err != nil {
		t.Fatal(err)
	}
	id2, _ := a.CrearRecordatorio(1, "tomar agua", ahora.Add(30*time.Minute).Unix(), RepiteDiaria, ahora)
	id3, _ := a.CrearRecordatorio(1, "junta", ahora.Add(2*time.Hour).Unix(), RepiteSemanal, ahora)
	var b []byte
	_ = a.db.QueryRow(`SELECT texto_cifrado FROM recordatorios WHERE id=?`, id1).Scan(&b)
	if len(b) == 0 || strings.Contains(string(b), "contador") {
		t.Fatal("el texto va cifrado")
	}
	if a.SiguienteRecordatorio(1) != ahora.Add(30*time.Minute).Unix() {
		t.Fatal("el siguiente es el más próximo")
	}
	if v, _ := a.RecordatoriosVencidos(1, ahora); len(v) != 0 {
		t.Fatal("aún nada")
	}
	v, _ := a.RecordatoriosVencidos(1, ahora.Add(90*time.Minute))
	if len(v) != 2 || v[0].ID != id2 || v[1].ID != id1 || v[1].Texto != "llamar al contador" {
		t.Fatalf("vencidos: %+v", v)
	}
	for _, r := range v {
		_ = a.MarcarRecordatorioAvisado(1, r, ahora.Add(90*time.Minute))
	}
	l, _ := a.ListarRecordatorios(1, true)
	var pendientes, hechos int
	for _, r := range l {
		switch r.Estado {
		case RecPendiente:
			pendientes++
		case RecHecho:
			hechos++
		}
	}
	if pendientes != 2 || hechos != 1 {
		t.Fatalf("pendientes %d hechos %d", pendientes, hechos)
	}
	// La diaria pasa a mañana a la misma hora; la semanal sigue en su hora.
	for _, r := range l {
		if r.ID == id2 && time.Unix(r.ProximaTS, 0).In(a.loc()).Format("2006-01-02 15:04") != "2026-03-11 09:30" {
			t.Fatalf("diaria: %v", time.Unix(r.ProximaTS, 0).In(a.loc()))
		}
		if r.ID == id3 && r.ProximaTS != ahora.Add(2*time.Hour).Unix() {
			t.Fatal("la semanal no se movió")
		}
	}
	// La semanal vencida pasa 7 días y se salta los que ya pasaron.
	r3 := Recordatorio{ID: id3, Repetir: RepiteSemanal, ProximaTS: ahora.Add(2 * time.Hour).Unix()}
	_ = a.MarcarRecordatorioAvisado(1, r3, ahora.Add(15*24*time.Hour))
	l, _ = a.ListarRecordatorios(1, false)
	for _, r := range l {
		if r.ID == id3 && time.Unix(r.ProximaTS, 0).In(a.loc()).Format("2006-01-02 15:04") != "2026-03-31 11:00" {
			t.Fatalf("semanal tras 15 días: %v", time.Unix(r.ProximaTS, 0).In(a.loc()))
		}
	}
	if err := a.EditarRecordatorio(1, id1, "nuevo", ahora.Add(5*time.Hour).Unix(), ""); err != nil {
		t.Fatal(err)
	}
	if err := a.BorrarRecordatorio(1, id1); err != nil {
		t.Fatal(err)
	}
	a.LimpiarRecordatoriosHechos(1, 30, ahora.Add(60*24*time.Hour))
}

func TestAlertasPalabraCifradasYDuplicadas(t *testing.T) {
	a := nuevo(t)
	id, err := a.CrearAlertaPalabra(1, "Urgente", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.CrearAlertaPalabra(1, " URGENTE ", nil); err == nil {
		t.Fatal("duplicada sin importar mayúsculas")
	}
	if _, err := a.CrearAlertaPalabra(1, "  ", nil); err == nil {
		t.Fatal("vacía")
	}
	var b []byte
	_ = a.db.QueryRow(`SELECT palabra_cifrada FROM alertas_palabra WHERE id=?`, id).Scan(&b)
	if len(b) == 0 || strings.Contains(strings.ToLower(string(b)), "urgente") {
		t.Fatal("la palabra va cifrada")
	}
	_ = a.EditarAlertaPalabra(1, id, []string{pnAna}, false)
	l, _ := a.ListarAlertasPalabra(1)
	if len(l) != 1 || l[0].Palabra != "Urgente" || l[0].Activa || len(l[0].Chats) != 1 {
		t.Fatalf("%+v", l)
	}
	_ = a.BorrarAlertaPalabra(1, id)
	if l, _ = a.ListarAlertasPalabra(1); len(l) != 0 {
		t.Fatal("borrar")
	}
}

func TestResumenDelDiaCuentaDesdeLaBase(t *testing.T) {
	a := nuevo(t)
	llenaPersona(t, a)
	ahora := time.Now()
	_ = a.FijarEstadoTrabajo(1, pnAna, TrabajoPendiente)
	_ = a.DecidirSolicitud(1, "5215599999999@s.whatsapp.net", SolPendiente, ahora)
	_, _, _ = a.AgregarAdvertencia(1, pnLuis, "", "", "yo", ahora)
	d := a.ResumenDelDia(1, ahora)
	// Recibidos hoy: F1, T1, G1, G2, L1 (5; T2 es mío). Chats: pnAna, g, pnLuis.
	if d.Recibidos != 5 || d.Chats != 3 || d.Pendientes != 1 || d.Solicitudes != 1 || d.Advertencias != 1 {
		t.Fatalf("%+v", d)
	}
}

func TestEliminarBotLimpiaLasTablasNuevas(t *testing.T) {
	a := nuevo(t)
	llenaPersona(t, a)
	ahora := time.Now()
	_, _, _ = a.AgregarAdvertencia(1, pnAna, "", "", "yo", ahora)
	_, _ = a.AgregarNota(1, pnAna, "n", "yo", ahora)
	_ = a.FijarSilencio(1, pnAna, 0, "", "yo", ahora)
	ide, _ := a.CrearEtiqueta(1, "x", "")
	_ = a.PonerEtiqueta(1, pnAna, ide, true)
	_, _ = a.CrearRecordatorio(1, "r", ahora.Add(time.Hour).Unix(), "", ahora)
	_, _ = a.CrearAlertaPalabra(1, "p", nil)
	if err := a.EliminarBot(1); err != nil {
		t.Fatal(err)
	}
	for _, tabla := range []string{"advertencias", "notas_persona", "persona_silencio", "etiquetas", "chat_etiqueta", "recordatorios", "alertas_palabra"} {
		var n int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM ` + tabla).Scan(&n)
		if n != 0 {
			t.Fatalf("%s sigue con datos", tabla)
		}
	}
}

func TestNombrePropioGanaAlDeLaAgendaYSeQuita(t *testing.T) {
	a := nuevo(t)
	const p = "5215500001111@s.whatsapp.net"
	if _, err := a.ActualizarChat(1, p, false, func(c *Chat) { c.Guardado, c.TelE164 = "Bot VPS", "+5215500001111" }); err != nil {
		t.Fatal(err)
	}
	if err := a.FijarNombrePropio(1, p, "  Usuario  "); err != nil {
		t.Fatal(err)
	}
	if c, _ := a.ObtenerChat(1, p); c.Mostrado != "Usuario" || c.Propio != "Usuario" || a.NombreDePersona(1, p) != "Usuario" {
		t.Fatalf("debía mostrar el nombre propio: %+v", c)
	}
	// un cambio de nombre en la agenda o el público no lo pisa
	if _, err := a.ActualizarChat(1, p, false, func(c *Chat) { c.Guardado, c.Publico = "Otro nombre", "Otro público" }); err != nil {
		t.Fatal(err)
	}
	if c, _ := a.ObtenerChat(1, p); c.Mostrado != "Usuario" || c.Guardado != "Otro nombre" {
		t.Fatalf("el nombre propio se conserva: %+v", c)
	}
	if f, _ := a.FichaDe(1, p, time.Now()); f.NombrePropio != "Usuario" || f.Nombre != "Usuario" {
		t.Fatalf("la ficha lo trae: %+v", f)
	}
	// se encuentra buscando por el nombre nuevo
	if err := a.FijarNombrePropio(1, p, ""); err != nil {
		t.Fatal(err)
	}
	if c, _ := a.ObtenerChat(1, p); c.Mostrado != "Otro nombre" || c.Propio != "" {
		t.Fatalf("al quitarlo vuelve el de la agenda: %+v", c)
	}
	if err := a.FijarNombrePropio(1, p, strings.Repeat("x", 41)); err == nil {
		t.Fatal("un nombre muy largo debe rechazarse")
	}
	if err := a.FijarNombrePropio(1, "5215509998888@s.whatsapp.net", "Nadie"); err == nil {
		t.Fatal("sin chat no hay a quién ponerle nombre")
	}
}
