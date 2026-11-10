package almacen

import (
	"os"
	"testing"
	"time"
)

func mensajeConFoto(t *testing.T, a *Almacen, chat, id string, ts int64) *Mensaje {
	t.Helper()
	m := msg(chat, id, ts, "con foto")
	m.Tipo, m.ArchivoEstado = TipoImagen, EstadoDescargado
	m.ArchivoRuta, m.MiniaturaRuta = RutaMedio(1, chat, id), RutaMiniatura(1, chat, id)
	if err := a.EscribirCifrado(m.ArchivoRuta, []byte("FOTO")); err != nil {
		t.Fatal(err)
	}
	if err := a.EscribirCifrado(m.MiniaturaRuta, []byte("mini")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GuardarMensaje(m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestEliminadoConservaContenidoYArchivo(t *testing.T) {
	a := nuevo(t)
	c := "1@s.whatsapp.net"
	chatBasico(t, a, c, "521", "Ana", false)
	m := mensajeConFoto(t, a, c, "F1", 1_700_000_000)
	if ok, err := a.MarcarEliminado(1, c, "F1", "", 1_700_000_900); !ok || err != nil {
		t.Fatal(ok, err)
	}
	got, _ := a.ObtenerMensaje(1, c, "F1")
	if !got.Borrado || got.EliminadoEn != 1_700_000_900 || got.EliminadoPor != "" || got.Texto != "con foto" || got.ArchivoRuta != m.ArchivoRuta {
		t.Fatalf("%+v", got)
	}
	if b, err := a.LeerCifrado(m.ArchivoRuta); err != nil || string(b) != "FOTO" {
		t.Fatal("el archivo debe seguir ahí", err)
	}
	// Una segunda eliminación no cambia la hora original.
	_, _ = a.MarcarEliminado(1, c, "F1", "", 1_700_009_999)
	if g, _ := a.ObtenerMensaje(1, c, "F1"); g.EliminadoEn != 1_700_000_900 {
		t.Fatal("la hora original debe conservarse")
	}
	if a.Eliminados(1, c) != 1 {
		t.Fatal("conteo")
	}
}

func TestEliminadoPorAdminSeAnotaYPorElAutorNo(t *testing.T) {
	a := nuevo(t)
	g := "9@g.us"
	chatBasico(t, a, g, "", "Familia", true)
	m1, m2 := msg(g, "A", 100, "uno"), msg(g, "B", 101, "dos")
	m1.RemitenteJID, m2.RemitenteJID, m1.RemitenteNombre = "ana@lid", "ana@lid", "Ana"
	m2.RemitenteNombre = "Ana"
	adm := msg(g, "C", 99, "admin habla")
	adm.RemitenteJID, adm.RemitenteNombre = "admin@lid", "Admina"
	if _, err := a.GuardarMensajes([]*Mensaje{m1, m2, adm}); err != nil {
		t.Fatal(err)
	}
	_, _ = a.MarcarEliminado(1, g, "A", "ana@lid", 200)   // su propio autor
	_, _ = a.MarcarEliminado(1, g, "B", "admin@lid", 201) // un admin
	x, _ := a.ObtenerMensaje(1, g, "A")
	y, _ := a.ObtenerMensaje(1, g, "B")
	if x.EliminadoPor != "" || y.EliminadoPor != "admin@lid" {
		t.Fatalf("por: %q %q", x.EliminadoPor, y.EliminadoPor)
	}
	l, _ := a.ListarMensajes(1, g, 0, "", 10)
	var vb MensajeVista
	for _, v := range l {
		if v.ID == "B" {
			vb = v
		}
	}
	if vb.EliminadoPor != "Admina" || vb.EliminadoEn != 201 {
		t.Fatalf("vista %+v", vb)
	}
}

func TestEliminadoConConservacionApagadaSobrescribeYBorraArchivo(t *testing.T) {
	a := nuevo(t)
	a.FijarConservarEliminados(false)
	c := "1@s.whatsapp.net"
	chatBasico(t, a, c, "521", "Ana", false)
	m := mensajeConFoto(t, a, c, "F1", 1_700_000_000)
	_, _ = a.MarcarEliminado(1, c, "F1", "", 1_700_000_900)
	got, _ := a.ObtenerMensaje(1, c, "F1")
	if !got.Borrado || got.Texto != "" || got.ArchivoRuta != "" || got.MiniaturaRuta != "" || len(got.ArchivoInfo) != 0 {
		t.Fatalf("%+v", got)
	}
	abs, _ := a.Abs(m.ArchivoRuta)
	if _, err := os.Stat(abs); !os.IsNotExist(err) {
		t.Fatal("el archivo debía borrarse")
	}
}

func TestOlvidarMensajeBorraParaSiempre(t *testing.T) {
	a := nuevo(t)
	c := "1@s.whatsapp.net"
	chatBasico(t, a, c, "521", "Ana", false)
	m := mensajeConFoto(t, a, c, "F1", 1_700_000_000)
	if ok, err := a.OlvidarMensaje(1, c, "F1"); !ok || err != nil {
		t.Fatal(ok, err)
	}
	got, _ := a.ObtenerMensaje(1, c, "F1")
	if !got.Borrado || got.Texto != "" || got.ArchivoRuta != "" || got.EliminadoEn == 0 {
		t.Fatalf("%+v", got)
	}
	abs, _ := a.Abs(m.ArchivoRuta)
	if _, err := os.Stat(abs); !os.IsNotExist(err) {
		t.Fatal("el archivo debía borrarse")
	}
	if ok, _ := a.OlvidarMensaje(1, c, "NO"); ok {
		t.Fatal("no existe")
	}
}

func TestEliminarNoDuplicaEstadisticas(t *testing.T) {
	a := nuevo(t)
	_ = a.FijarZona("America/Mexico_City")
	c := "1@s.whatsapp.net"
	chatBasico(t, a, c, "521", "Ana", false)
	_, _ = a.GuardarMensajes([]*Mensaje{msg(c, "A", 1_700_000_000, "x"), msg(c, "B", 1_700_000_001, "y")})
	_, _ = a.MarcarEliminado(1, c, "A", "", 1_700_000_100)
	r, _ := a.ResumenDeChat(1, c)
	if r.Recibidos != 2 || r.Eliminados != 1 {
		t.Fatalf("recibidos %d eliminados %d", r.Recibidos, r.Eliminados)
	}
}

func TestListarFiltrados(t *testing.T) {
	a := nuevo(t)
	l := mx(t, a)
	g := "9@g.us"
	chatBasico(t, a, g, "", "Familia", true)
	dia := func(d, h int) int64 { return time.Date(2026, 11, d, h, 15, 0, 0, l).Unix() }
	mk := func(id string, ts int64, tipo, quien string, mio bool) *Mensaje {
		m := msg(g, id, ts, "t-"+id)
		m.Tipo, m.RemitenteJID, m.RemitenteNombre, m.EsMio = tipo, quien, quien, mio
		return m
	}
	_, err := a.GuardarMensajes([]*Mensaje{
		mk("1", dia(2, 9), TipoTexto, "ana@lid", false),   // lunes 9
		mk("2", dia(2, 10), TipoImagen, "ana@lid", false), // lunes 10
		mk("3", dia(3, 9), TipoImagen, "beto@lid", false), // martes 9
		mk("4", dia(3, 20), TipoTexto, "yo", true),        // martes 20
		mk("5", dia(4, 9), TipoSticker, "beto@lid", false),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = a.MarcarEliminado(1, g, "3", "", dia(5, 1))
	ids := func(f FiltroLista) (int64, string) {
		p, err := a.ListarFiltrados(1, g, f, 0, "", 50)
		if err != nil {
			t.Fatal(err)
		}
		s := ""
		for _, v := range p.Items {
			s += v.ID
		}
		return p.Total, s
	}
	if n, s := ids(FiltroLista{Tipos: []string{TipoImagen}}); n != 2 || s != "32" {
		t.Fatalf("fotos %d %s", n, s)
	}
	if n, s := ids(FiltroLista{Direccion: "enviados"}); n != 1 || s != "4" {
		t.Fatalf("enviados %d %s", n, s)
	}
	if n, s := ids(FiltroLista{Direccion: "recibidos", Persona: "beto@lid"}); n != 2 || s != "53" {
		t.Fatalf("beto %d %s", n, s)
	}
	if n, s := ids(FiltroLista{Eliminados: true}); n != 1 || s != "3" {
		t.Fatalf("eliminados %d %s", n, s)
	}
	h9, lun := 9, 0
	if n, s := ids(FiltroLista{Hora: &h9}); n != 3 || s != "531" {
		t.Fatalf("hora 9: %d %s", n, s)
	}
	if n, s := ids(FiltroLista{Semana: &lun}); n != 2 || s != "21" {
		t.Fatalf("lunes: %d %s", n, s)
	}
	if n, s := ids(FiltroLista{Dia: "2026-11-03"}); n != 2 || s != "43" {
		t.Fatalf("día: %d %s", n, s)
	}
	if n, s := ids(FiltroLista{Hora: &h9, Semana: &lun, Tipos: []string{TipoTexto}}); n != 1 || s != "1" {
		t.Fatalf("combinado: %d %s", n, s)
	}
	// Paginación: de 2 en 2, sin repetir ni saltar.
	p1, _ := a.ListarFiltrados(1, g, FiltroLista{}, 0, "", 2)
	if p1.Total != 5 || len(p1.Items) != 2 || p1.Items[0].ID != "5" {
		t.Fatalf("pág 1 %+v", p1)
	}
	ult := p1.Items[1]
	p2, _ := a.ListarFiltrados(1, g, FiltroLista{}, ult.TS, ult.ID, 2)
	if p2.Total != 0 || len(p2.Items) != 2 || p2.Items[0].ID != "3" || p2.Items[1].ID != "2" {
		t.Fatalf("pág 2 %+v", p2)
	}
	ps, _ := a.PersonasDeChat(1, g)
	if len(ps) != 2 || ps[0].Nombre != "ana@lid" && ps[0].Nombre != "beto@lid" {
		t.Fatalf("personas %+v", ps)
	}
}
