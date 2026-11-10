package almacen

import (
	"encoding/json"
	"testing"
	"time"
)

func mx(t *testing.T, a *Almacen) *time.Location {
	t.Helper()
	if err := a.FijarZona("America/Mexico_City"); err != nil {
		t.Fatal(err)
	}
	l, _ := time.LoadLocation("America/Mexico_City")
	return l
}

func TestEstadisticasSeActualizanAlGuardar(t *testing.T) {
	a := nuevo(t)
	l := mx(t, a)
	chatBasico(t, a, "5215511111111@s.whatsapp.net", "5215511111111", "Ana", false)
	c := "5215511111111@s.whatsapp.net"
	t1 := time.Date(2026, 11, 3, 9, 30, 0, 0, l).Unix()
	t2 := time.Date(2026, 11, 3, 23, 59, 0, 0, l).Unix() // en UTC ya es 4 de noviembre
	t3 := time.Date(2026, 11, 4, 9, 0, 0, 0, l).Unix()
	m3 := msg(c, "C", t3, "")
	m3.Tipo, m3.ArchivoBytes = TipoImagen, 2048
	m2 := msg(c, "B", t2, "yo")
	m2.EsMio = true
	if _, err := a.GuardarMensajes([]*Mensaje{msg(c, "A", t1, "hola"), m2, m3}); err != nil {
		t.Fatal(err)
	}
	// Un mensaje repetido no cuenta dos veces.
	if _, err := a.GuardarMensajes([]*Mensaje{msg(c, "A", t1, "hola")}); err != nil {
		t.Fatal(err)
	}
	r, err := a.ResumenDeChat(1, c)
	if err != nil {
		t.Fatal(err)
	}
	if r.Enviados != 1 || r.Recibidos != 2 {
		t.Fatalf("enviados %d recibidos %d", r.Enviados, r.Recibidos)
	}
	if r.PorHora[9] != 2 || r.PorHora[23] != 1 {
		t.Fatalf("por hora %v", r.PorHora)
	}
	if r.PorSemana[1] != 2 || r.PorSemana[2] != 1 { // 3 nov 2026 es martes, 4 es miércoles
		t.Fatalf("por semana %v", r.PorSemana)
	}
	var img *TipoCuenta
	for i := range r.PorTipo {
		if r.PorTipo[i].Tipo == TipoImagen {
			img = &r.PorTipo[i]
		}
	}
	if img == nil || img.Recibidos != 1 || img.Bytes != 2048 {
		t.Fatalf("imagen %+v", img)
	}
	cal, _ := a.CalendarioMes(1, c, "2026-11")
	if len(cal.Dias) != 2 || cal.Dias[0].Dia != "2026-11-03" || cal.Dias[0].Cantidad != 2 {
		t.Fatalf("calendario %+v", cal.Dias)
	}
	if r.Desde != t1 {
		t.Fatal("la fecha del primer mensaje")
	}
	dia, _ := a.MensajesDelDia(1, c, "2026-11-03")
	if len(dia) != 2 || dia[0].ID != "A" || dia[1].ID != "B" {
		t.Fatalf("mensajes del día %+v", dia)
	}
}

func TestEstadisticasSinTextoEnClaro(t *testing.T) {
	a := nuevo(t)
	chatBasico(t, a, "1@s.whatsapp.net", "52155", "Ana", false)
	if _, err := a.GuardarMensajes([]*Mensaje{msg("1@s.whatsapp.net", "A", 1_700_000_000, "secreto-muy-privado")}); err != nil {
		t.Fatal(err)
	}
	filas, _ := a.db.Query(`SELECT * FROM daily_stats`)
	defer filas.Close()
	cols, _ := filas.Columns()
	if len(cols) != 8 {
		t.Fatalf("columnas %v", cols)
	}
	for filas.Next() {
		v := make([]any, len(cols))
		p := make([]any, len(cols))
		for i := range v {
			p[i] = &v[i]
		}
		_ = filas.Scan(p...)
		b, _ := json.Marshal(v)
		if string(b) == "" || contiene(string(b), "secreto") {
			t.Fatal("el texto no debe estar en las estadísticas")
		}
	}
}

func contiene(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestRellenoDeMensajesAnteriores(t *testing.T) {
	a := nuevo(t)
	l := mx(t, a)
	c := "9@s.whatsapp.net"
	chatBasico(t, a, c, "529", "Beto", false)
	// Se simula una base anterior a las estadísticas: mensajes sin contadores y relleno pendiente.
	if _, err := a.GuardarMensajes([]*Mensaje{
		msg(c, "A", time.Date(2026, 1, 5, 10, 0, 0, 0, l).Unix(), "x"),
		msg(c, "B", time.Date(2026, 1, 5, 11, 0, 0, 0, l).Unix(), "y"),
	}); err != nil {
		t.Fatal(err)
	}
	a.db.Exec(`DELETE FROM daily_stats`)
	a.db.Exec(`UPDATE stats_info SET valor=(SELECT MAX(rowid) FROM messages) WHERE campo='rowid_max'`)
	a.db.Exec(`UPDATE stats_info SET valor=0 WHERE campo='relleno'`)
	// Un mensaje que llega durante el relleno se cuenta al guardarlo, no por el relleno.
	if _, err := a.GuardarMensajes([]*Mensaje{msg(c, "C", time.Date(2026, 1, 6, 8, 0, 0, 0, l).Unix(), "z")}); err != nil {
		t.Fatal(err)
	}
	if r, _ := a.ResumenDeChat(1, c); r.Listo || r.Recibidos != 1 {
		t.Fatalf("antes del relleno: listo=%v recibidos=%d", r.Listo, r.Recibidos)
	}
	if err := a.RellenarEstadisticas(); err != nil {
		t.Fatal(err)
	}
	if err := a.RellenarEstadisticas(); err != nil { // repetirlo no suma otra vez
		t.Fatal(err)
	}
	r, _ := a.ResumenDeChat(1, c)
	if !r.Listo || r.Recibidos != 3 {
		t.Fatalf("después del relleno: listo=%v recibidos=%d", r.Listo, r.Recibidos)
	}
	cal, _ := a.CalendarioMes(1, c, "2026-01")
	if len(cal.Dias) != 2 || cal.Dias[0].Cantidad != 2 {
		t.Fatalf("calendario %+v", cal.Dias)
	}
}

func TestEstadisticasDeGrupoYGlobales(t *testing.T) {
	a := nuevo(t)
	mx(t, a)
	g := "123@g.us"
	chatBasico(t, a, g, "", "Familia", true)
	chatBasico(t, a, "7@s.whatsapp.net", "527", "Carla", false)
	ahora := time.Now().Unix()
	mm := func(id, quien, nombre string) *Mensaje {
		m := msg(g, id, ahora, "x")
		m.RemitenteJID, m.RemitenteNombre = quien, nombre
		return m
	}
	if _, err := a.GuardarMensajes([]*Mensaje{mm("1", "a@lid", "Ana"), mm("2", "a@lid", "Ana"), mm("3", "b@lid", "Beto"), msg("7@s.whatsapp.net", "4", ahora, "h")}); err != nil {
		t.Fatal(err)
	}
	r, _ := a.ResumenDeChat(1, g)
	if len(r.Quien) != 2 || r.Quien[0].Nombre != "Ana" || r.Quien[0].Cantidad != 2 {
		t.Fatalf("quién escribe más: %+v", r.Quien)
	}
	rp, _ := a.ResumenDeChat(1, "7@s.whatsapp.net")
	if len(rp.Quien) != 0 {
		t.Fatal("en chats de una persona no hay ranking")
	}
	gl, err := a.ResumenDeTodo(1, 7)
	if err != nil {
		t.Fatal(err)
	}
	if gl.Recibidos != 4 || gl.Chats != 2 || len(gl.Activos) != 2 || gl.Activos[0].JID != g {
		t.Fatalf("globales %+v", gl)
	}
	if len(gl.PorDia) != 7 || gl.PorDia[6].Cantidad != 4 {
		t.Fatalf("por día %+v", gl.PorDia)
	}
}

func TestEstadisticasSigueAlFusionarYBorrar(t *testing.T) {
	a := nuevo(t)
	mx(t, a)
	lid, pn := "99@lid", "52@s.whatsapp.net"
	chatBasico(t, a, lid, "", "X", false)
	chatBasico(t, a, pn, "52", "X", false)
	if _, err := a.GuardarMensajes([]*Mensaje{msg(lid, "1", 1_700_000_000, "a"), msg(pn, "2", 1_700_000_000, "b")}); err != nil {
		t.Fatal(err)
	}
	if err := a.FusionarChat(1, lid, pn); err != nil {
		t.Fatal(err)
	}
	if r, _ := a.ResumenDeChat(1, pn); r.Recibidos != 2 {
		t.Fatalf("fusionado: %d", r.Recibidos)
	}
	if r, _ := a.ResumenDeChat(1, lid); r.Recibidos != 0 {
		t.Fatal("el chat viejo debe quedar sin contadores")
	}
	if _, err := a.BorrarChat(1, pn); err != nil {
		t.Fatal(err)
	}
	if r, _ := a.ResumenDeChat(1, pn); r.Recibidos != 0 {
		t.Fatal("borrar el chat borra sus contadores")
	}
}

func TestVistaPreviaDeContactosYUbicaciones(t *testing.T) {
	if got := VistaPrevia(TipoContacto, `{"v":1,"contactos":[{"nombre":"Juan","tels":["+52 55"]},{"nombre":"Ana","tels":[]}]}`); got != "Contacto: Juan, Ana" {
		t.Fatal(got)
	}
	if got := VistaPrevia(TipoUbicacion, `{"v":1,"lat":19.4,"lon":-99.1,"nombre":"Casa"}`); got != "Ubicación: Casa" {
		t.Fatal(got)
	}
	if got := VistaPrevia(TipoContacto, "Contacto: Viejo"); got != "Contacto: Viejo" {
		t.Fatal("los mensajes guardados antes se muestran igual")
	}
}

func TestZonaInvalidaNoCambiaLaAnterior(t *testing.T) {
	a := nuevo(t)
	mx(t, a)
	if err := a.FijarZona("Marte/Olimpo"); err == nil {
		t.Fatal("debe rechazar una zona inexistente")
	}
	if a.loc().String() != "America/Mexico_City" {
		t.Fatal("la zona anterior se perdió")
	}
}
