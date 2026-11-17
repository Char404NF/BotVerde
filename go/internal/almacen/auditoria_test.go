package almacen

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"botchat/internal/cifrado"
)

const chatA = "5215512345678@s.whatsapp.net"

func dias(n int) int64 { return time.Now().Add(-time.Duration(n) * 24 * time.Hour).Unix() }

func TestAuditoriaPorDefectoYAjustes(t *testing.T) {
	a := nuevo(t)
	au := a.Auditoria(1)
	if !au.Defecto || au.RetDias != 90 || au.RetPendiente || au.AvisoVisto {
		t.Fatalf("instalación nueva: %+v", au)
	}
	if !a.AuditoriaActiva(1, chatA) {
		t.Fatal("de fábrica se guarda")
	}
	if err := a.FijarAuditoria(1, false, 30); err != nil {
		t.Fatal(err)
	}
	if au := a.Auditoria(1); au.Defecto || au.RetDias != 30 {
		t.Fatalf("%+v", au)
	}
	if a.AuditoriaActiva(1, chatA) {
		t.Fatal("global apagado")
	}
	if err := a.FijarAuditoriaChat(1, chatA, 1, 0); err != nil {
		t.Fatal(err)
	}
	if !a.AuditoriaActiva(1, chatA) {
		t.Fatal("el chat encendido manda sobre el global")
	}
	d := a.AuditoriaChat(1, chatA)
	if d.Audit != 1 || !d.Efectivo || d.RetDias != 0 || d.RetEfectiva != 0 {
		t.Fatalf("%+v", d)
	}
	for _, malo := range []int{7, 365} {
		if a.FijarAuditoria(1, true, malo) == nil || a.FijarAuditoriaChat(1, chatA, -1, malo) == nil {
			t.Fatalf("retención %d no permitida", malo)
		}
	}
	if a.FijarAuditoriaChat(1, chatA, 5, -1) == nil {
		t.Fatal("valor de interruptor no válido")
	}
}

func TestListarAuditoriaChats(t *testing.T) {
	a := nuevo(t)
	chatBasico(t, a, chatA, "5215512345678", "Ana", false)
	chatBasico(t, a, "120363@g.us", "", "Equipo", true)
	_, _ = a.GuardarMensaje(msg(chatA, "1", 100, "x"))
	_, _ = a.GuardarMensaje(msg("120363@g.us", "2", 200, "y"))
	_ = a.FijarAuditoriaChat(1, "120363@g.us", 0, 30)
	l, err := a.ListarAuditoriaChats(1, "", 10, 0)
	if err != nil || len(l) != 2 {
		t.Fatalf("%+v %v", l, err)
	}
	if !l[0].EsGrupo || l[0].Audit != 0 || l[0].Efectivo || l[0].RetEfectiva != 30 || l[1].Audit != -1 || !l[1].Efectivo || l[1].RetEfectiva != 90 {
		t.Fatalf("%+v", l)
	}
	if f, _ := a.ListarAuditoriaChats(1, "equi", 10, 0); len(f) != 1 || !f[0].EsGrupo {
		t.Fatalf("%+v", f)
	}
}

func TestAplicarAuditoriaQuitaTextoPeroConservaLoDemas(t *testing.T) {
	a := nuevo(t)
	_ = a.FijarAuditoriaChat(1, chatA, 0, -1)
	m := &Mensaje{ID: "1", ChatJID: chatA, BotID: 1, RemitenteJID: chatA, Tipo: TipoDocumento, Texto: "pie", ArchivoNombre: "x.pdf", CitaID: "0",
		CitaTexto: "cita", Menciones: []string{"a@s.whatsapp.net"}, ArchivoBytes: 77, ArchivoInfo: []byte(`{"sha":"c2hh"}`), TS: 5}
	a.AplicarAuditoria(m)
	if m.Texto != "" || m.ArchivoNombre != "" || m.CitaTexto != "" || len(m.Menciones) != 0 || !m.Olvidado || m.ArchivoBytes != 77 || len(m.ArchivoInfo) == 0 || m.CitaID != "0" {
		t.Fatalf("%+v", m)
	}
	// ver una vez conserva su aviso fijo y no se marca si no había nada más
	v := &Mensaje{ID: "2", ChatJID: chatA, BotID: 1, Tipo: TipoVerUnaVez, Texto: TextoVerUnaVez, TS: 5}
	a.AplicarAuditoria(v)
	if v.Texto != TextoVerUnaVez || v.Olvidado {
		t.Fatalf("%+v", v)
	}
	// un mensaje sin nada que quitar no queda marcado como olvidado
	vacio := &Mensaje{ID: "3", ChatJID: chatA, BotID: 1, Tipo: TipoImagen, TS: 5}
	a.AplicarAuditoria(vacio)
	if vacio.Olvidado {
		t.Fatal("no había nada que olvidar")
	}
}

func TestEditarTextoGuardaVersionesEnLaMismaTransaccion(t *testing.T) {
	a := nuevo(t)
	chatBasico(t, a, chatA, "5215512345678", "Ana", false)
	_, _ = a.GuardarMensaje(msg(chatA, "1", 100, "uno"))
	for _, nuevo := range []string{"dos", "dos", "tres"} {
		if ok, err := a.EditarTexto(1, chatA, "1", nuevo); err != nil || !ok {
			t.Fatalf("%v %v", ok, err)
		}
	}
	v, _ := a.VersionesDeMensaje(1, chatA, "1")
	if len(v) != 2 || v[0].Texto != "uno" || v[1].Texto != "dos" {
		t.Fatalf("la edición repetida no duplica: %+v", v)
	}
	if ok, _ := a.EditarTexto(1, chatA, "no-existe", "x"); ok {
		t.Fatal("no existe")
	}
	// el texto anterior va cifrado con su propio AAD
	var b []byte
	_ = a.db.QueryRow("SELECT texto_cifrado FROM mensaje_ediciones WHERE n=1").Scan(&b)
	if _, err := cifrado.AbrirTexto(a.clave, "texto", b); err == nil {
		t.Fatal("la versión debe usar el AAD texto_ed")
	}
	if t1, err := cifrado.AbrirTexto(a.clave, "texto_ed", b); err != nil || string(t1) != "uno" {
		t.Fatalf("%q %v", t1, err)
	}
	// si falla a la mitad no queda nada: simulamos con un mensaje borrado a mano entre medias no es posible; verificamos que al
	// borrar el mensaje las versiones se van con él (cascada).
	if _, err := a.db.Exec("DELETE FROM messages WHERE id='1'"); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = a.db.QueryRow("SELECT COUNT(*) FROM mensaje_ediciones").Scan(&n)
	if n != 0 {
		t.Fatalf("quedaron %d versiones sueltas", n)
	}
}

func llena(t *testing.T, a *Almacen, chat string, cuantos int, edadDias int) {
	t.Helper()
	for i := 0; i < cuantos; i++ {
		m := msg(chat, fmt.Sprintf("%s-%d-%d", chat[:3], edadDias, i), dias(edadDias)+int64(i), "texto secreto")
		m.CitaID, m.CitaTexto = "c", "cita secreta"
		m.ArchivoNombre = "n.pdf"
		if _, err := a.GuardarMensaje(m); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRetencionOlvidaTextoConservaFilasYEstadisticas(t *testing.T) {
	a := nuevo(t)
	pausaRetencion = 0
	chatBasico(t, a, chatA, "5215512345678", "Ana", false)
	llena(t, a, chatA, 3, 120) // viejos
	llena(t, a, chatA, 2, 10)  // recientes
	_, _ = a.EditarTexto(1, chatA, fmt.Sprintf("%s-120-0", chatA[:3]), "editado")
	var ant int
	_ = a.db.QueryRow("SELECT COALESCE(SUM(count),0) FROM daily_stats").Scan(&ant)
	r, err := a.LimpiarPorRetencion(context.Background(), 1, time.Now())
	if err != nil || r.Pendiente || r.Mensajes != 3 || r.Ediciones != 1 {
		t.Fatalf("%+v %v", r, err)
	}
	var total, olv, sinTexto int
	_ = a.db.QueryRow("SELECT COUNT(*), SUM(contenido_olvidado=1), SUM(texto_cifrado IS NULL) FROM messages").Scan(&total, &olv, &sinTexto)
	if total != 5 || olv != 3 || sinTexto != 3 {
		t.Fatalf("total %d olvidados %d sin texto %d", total, olv, sinTexto)
	}
	var cita, nombre, menc int
	_ = a.db.QueryRow("SELECT SUM(cita_texto_cifrado IS NULL), SUM(archivo_nombre_cifrado IS NULL), SUM(menciones_cifrado IS NULL) FROM messages WHERE contenido_olvidado=1").Scan(&cita, &nombre, &menc)
	if cita != 3 || nombre != 3 || menc != 3 {
		t.Fatal("también se olvida cita, nombre y menciones")
	}
	var desp int
	_ = a.db.QueryRow("SELECT COALESCE(SUM(count),0) FROM daily_stats").Scan(&desp)
	if desp != ant || desp != 5 {
		t.Fatalf("las estadísticas no cambian: %d -> %d", ant, desp)
	}
	var ed int
	_ = a.db.QueryRow("SELECT COUNT(*) FROM mensaje_ediciones").Scan(&ed)
	if ed != 0 {
		t.Fatal("se purgan las ediciones")
	}
	// los recientes siguen con texto y una segunda pasada no hace nada
	if m, _ := a.ObtenerMensaje(1, chatA, fmt.Sprintf("%s-10-0", chatA[:3])); m.Texto == "" || m.Olvidado {
		t.Fatalf("%+v", m)
	}
	if m, _ := a.ObtenerMensaje(1, chatA, fmt.Sprintf("%s-120-1", chatA[:3])); m.Texto != "" || !m.Olvidado || m.TS == 0 {
		t.Fatalf("%+v", m)
	}
	if r2, _ := a.LimpiarPorRetencion(context.Background(), 1, time.Now()); r2.Mensajes != 0 {
		t.Fatalf("%+v", r2)
	}
}

func TestRetencionPorLotesDe500ConMuchosMensajes(t *testing.T) {
	a := nuevo(t)
	pausaRetencion = 0
	chatBasico(t, a, chatA, "5215512345678", "Ana", false)
	lote := make([]*Mensaje, 0, 1200)
	for i := 0; i < 1200; i++ {
		lote = append(lote, msg(chatA, fmt.Sprint("m", i), dias(200)+int64(i), "viejo"))
	}
	if _, err := a.GuardarMensajes(lote); err != nil {
		t.Fatal(err)
	}
	r, err := a.LimpiarPorRetencion(context.Background(), 1, time.Now())
	if err != nil || r.Mensajes != 1200 {
		t.Fatalf("%+v %v", r, err)
	}
	// cancelable
	llena(t, a, chatA, 3, 150)
	ctx, cancela := context.WithCancel(context.Background())
	cancela()
	if _, err := a.LimpiarPorRetencion(ctx, 1, time.Now()); err == nil {
		t.Fatal("debía cancelarse")
	}
}

func TestRetencionPorChatYSiempre(t *testing.T) {
	a := nuevo(t)
	pausaRetencion = 0
	otro := "34612345678@s.whatsapp.net"
	chatBasico(t, a, chatA, "5215512345678", "Ana", false)
	chatBasico(t, a, otro, "34612345678", "Beto", false)
	llena(t, a, chatA, 2, 45) // 90 global: se queda
	llena(t, a, otro, 2, 45)
	_ = a.FijarAuditoriaChat(1, otro, -1, 30) // este chat: 30 días
	r, _ := a.LimpiarPorRetencion(context.Background(), 1, time.Now())
	if r.Mensajes != 2 {
		t.Fatalf("solo el chat de 30 días: %+v", r)
	}
	if m, _ := a.ObtenerMensaje(1, chatA, fmt.Sprintf("%s-45-0", chatA[:3])); m.Texto == "" {
		t.Fatal("el global (90) conserva 45 días")
	}
	// global en 30 pero el chat en "siempre": ese no se toca
	_ = a.FijarRetencion(1, 30)
	_ = a.FijarAuditoriaChat(1, otro, -1, 0)
	llena(t, a, otro, 1, 400)
	r, _ = a.LimpiarPorRetencion(context.Background(), 1, time.Now())
	if r.Mensajes != 2 { // los 2 de chatA con 45 días
		t.Fatalf("%+v", r)
	}
	if m, _ := a.ObtenerMensaje(1, otro, fmt.Sprintf("%s-400-0", otro[:3])); m.Texto == "" {
		t.Fatal("siempre = no se olvida nunca")
	}
	// global siempre: nada
	_ = a.FijarRetencion(1, 0)
	llena(t, a, chatA, 1, 900)
	if r, _ := a.LimpiarPorRetencion(context.Background(), 1, time.Now()); r.Mensajes != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestRetencionPendienteNoPurgaNada(t *testing.T) {
	a := nuevo(t)
	pausaRetencion = 0
	chatBasico(t, a, chatA, "5215512345678", "Ana", false)
	llena(t, a, chatA, 3, 500)
	_ = a.FijarAjusteBot(1, "audit_ret_pendiente", "1")
	_ = a.FijarAuditoriaChat(1, chatA, -1, 30) // ni siquiera con retención propia
	r, err := a.LimpiarPorRetencion(context.Background(), 1, time.Now())
	if err != nil || !r.Pendiente || r.Mensajes != 0 {
		t.Fatalf("%+v %v", r, err)
	}
	var n int
	_ = a.db.QueryRow("SELECT COUNT(*) FROM messages WHERE texto_cifrado IS NULL").Scan(&n)
	if n != 0 {
		t.Fatal("no se debe purgar nada")
	}
	// al elegir, termina el estado pendiente y purga
	if err := a.FijarRetencion(1, 90); err != nil {
		t.Fatal(err)
	}
	if a.Auditoria(1).RetPendiente {
		t.Fatal("elegir quita el estado pendiente")
	}
	if r, _ := a.LimpiarPorRetencion(context.Background(), 1, time.Now()); r.Mensajes != 3 {
		t.Fatalf("%+v", r)
	}
}

func TestActualizarDesdeLa19DejaLaRetencionPendiente(t *testing.T) {
	k := make([]byte, 32)
	raiz := t.TempDir()
	completas := migraciones
	migraciones = completas[:19]
	a, err := Abrir(raiz, k)
	if err != nil {
		migraciones = completas
		t.Fatal(err)
	}
	if a.Version() != 19 {
		migraciones = completas
		t.Fatalf("versión %d", a.Version())
	}
	// el chat con el esquema viejo (sin las columnas que llegaron después, como nombre_propio)
	if _, err := a.db.Exec(`INSERT INTO chats(jid,bot_id,es_grupo,nombre_publico,nombre_mostrado,telefono_e164,creado_ts) VALUES(?,1,0,'Ana','Ana','+5215512345678',1)`, chatA); err != nil {
		migraciones = completas
		t.Fatal(err)
	}
	// un mensaje con el esquema viejo (sin las columnas nuevas)
	if _, err := a.db.Exec(`INSERT INTO messages(id,chat_jid,bot_id,remitente_jid,tipo,ts) VALUES('1',?,1,?, 'texto', 5)`, chatA, chatA); err != nil {
		migraciones = completas
		t.Fatal(err)
	}
	_ = a.Cerrar()
	migraciones = completas
	b, err := Abrir(raiz, k)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Cerrar()
	if b.Version() != len(completas) || len(completas) < 20 {
		t.Fatalf("versión %d", b.Version())
	}
	au := b.Auditoria(1)
	if !au.RetPendiente || !au.Defecto {
		t.Fatalf("quien actualiza queda pendiente y con texto guardado: %+v", au)
	}
	m, _ := b.ObtenerMensaje(1, chatA, "1")
	if m == nil || m.Olvidado || m.DelHistorial || m.SHA256 != nil {
		t.Fatalf("%+v", m)
	}
	if r, _ := b.LimpiarPorRetencion(context.Background(), 1, time.Now()); !r.Pendiente {
		t.Fatal("pendiente")
	}
	// columnas e índices de la migración 20
	for _, col := range []string{"audit_texto", "audit_ret_dias", "fijado"} {
		if _, err := b.db.Exec("SELECT " + col + " FROM chat_estado LIMIT 1"); err != nil {
			t.Fatal(err)
		}
	}
	for _, col := range []string{"sha256", "portada_ruta", "archivo_borrado_ts", "archivo_borrado_motivo", "enlaces", "contenido_olvidado", "del_historial"} {
		if _, err := b.db.Exec("SELECT " + col + " FROM messages LIMIT 1"); err != nil {
			t.Fatal(err)
		}
	}
	for _, ix := range []string{"idx_mensajes_sha", "idx_mensajes_viejos", "idx_mensajes_autor_tipo", "idx_mensajes_autor_global", "idx_mensajes_olvidar"} {
		var n int
		_ = b.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", ix).Scan(&n)
		if n != 1 {
			t.Fatalf("falta el índice %s", ix)
		}
	}
}

func TestInstalacionNuevaNoQuedaPendiente(t *testing.T) {
	a := nuevo(t)
	if a.Auditoria(1).RetPendiente || a.Auditoria(1).RetDias != 90 {
		t.Fatalf("%+v", a.Auditoria(1))
	}
}

func TestRellenarSHAEnSegundoPlano(t *testing.T) {
	a := nuevo(t)
	chatBasico(t, a, chatA, "5215512345678", "Ana", false)
	sha := []byte("huella-uno")
	for i := 0; i < 5; i++ {
		m := msg(chatA, fmt.Sprint("f", i), int64(100+i), "")
		m.Tipo = TipoImagen
		if i < 4 {
			m.ArchivoInfo = []byte(`{"sha":"` + base64.StdEncoding.EncodeToString(sha) + `"}`)
		}
		if _, err := a.GuardarMensaje(m); err != nil {
			t.Fatal(err)
		}
	}
	// simula mensajes de antes de la migración: sin huella
	_, _ = a.db.Exec("UPDATE messages SET sha256=NULL")
	total, vueltas := 0, 0
	for {
		n, fin, err := a.RellenarSHA(1, 2)
		if err != nil {
			t.Fatal(err)
		}
		total += n
		vueltas++
		if fin {
			break
		}
		if vueltas > 20 {
			t.Fatal("no termina")
		}
	}
	if total != 4 {
		t.Fatalf("llenó %d", total)
	}
	m, _ := a.ObtenerMensaje(1, chatA, "f0")
	if string(m.SHA256) != string(sha) {
		t.Fatalf("%q", m.SHA256)
	}
	if m4, _ := a.ObtenerMensaje(1, chatA, "f4"); m4.SHA256 != nil {
		t.Fatal("sin datos de descarga no hay huella")
	}
	// ya terminó: no vuelve a hacer nada
	if n, fin, _ := a.RellenarSHA(1, 2); n != 0 || !fin {
		t.Fatal("debía estar terminado")
	}
}

func TestOlvidarPersonaBorraTambienLasEdiciones(t *testing.T) {
	a := nuevo(t)
	chatBasico(t, a, chatA, "5215512345678", "Ana", false)
	_, _ = a.GuardarMensaje(msg(chatA, "1", 100, "uno"))
	_, _ = a.EditarTexto(1, chatA, "1", "dos")
	if _, err := a.OlvidarPersona(1, chatA); err != nil {
		t.Fatal(err)
	}
	var n, e int
	_ = a.db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&n)
	_ = a.db.QueryRow("SELECT COUNT(*) FROM mensaje_ediciones").Scan(&e)
	if n != 0 || e != 0 {
		t.Fatalf("mensajes %d ediciones %d", n, e)
	}
}
