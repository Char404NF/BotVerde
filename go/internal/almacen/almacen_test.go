package almacen

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"botchat/internal/cifrado"
)

func nuevo(t *testing.T) *Almacen {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 1)
	}
	a, err := Abrir(t.TempDir(), k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Cerrar() })
	return a
}

func chatBasico(t *testing.T, a *Almacen, jid, tel, publico string, grupo bool) {
	t.Helper()
	if _, err := a.ActualizarChat(1, jid, grupo, func(c *Chat) {
		c.TelE164, c.Publico = tel, publico
		if grupo {
			c.Guardado = publico
		}
	}); err != nil {
		t.Fatal(err)
	}
}

func msg(chat, id string, ts int64, texto string) *Mensaje {
	return &Mensaje{ID: id, ChatJID: chat, BotID: 1, RemitenteJID: chat, RemitenteNombre: "Ana", Tipo: TipoTexto, Texto: texto, TS: ts}
}

func TestMigracionesYVersion(t *testing.T) {
	a := nuevo(t)
	if a.Version() != len(migraciones) {
		t.Fatalf("versión %d", a.Version())
	}
	// Reabrir no repite migraciones.
	raiz, clave := a.raiz, a.clave
	_ = a.Cerrar()
	b, err := Abrir(raiz, clave)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Cerrar()
	if b.Version() != len(migraciones) {
		t.Fatal("la versión cambió al reabrir")
	}
	// Una base de una versión más nueva se rechaza en vez de dañarse.
	if _, err := b.db.Exec("INSERT INTO schema_info VALUES(999, 0)"); err != nil {
		t.Fatal(err)
	}
	_ = b.Cerrar()
	if _, err := Abrir(raiz, clave); err == nil || !strings.Contains(err.Error(), "más nueva") {
		t.Fatalf("debía rechazar una base más nueva: %v", err)
	}
}

func TestMigracionFallidaNoDejaMedioAplicado(t *testing.T) {
	a := nuevo(t)
	antes := migraciones
	defer func() { migraciones = antes }()
	migraciones = append(append([][]string{}, antes...), []string{`CREATE TABLE prueba_ok(x)`, `ESTO NO ES SQL`})
	if err := a.migrar(context.Background()); err == nil {
		t.Fatal("debía fallar")
	}
	var n int
	_ = a.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name='prueba_ok'").Scan(&n)
	if n != 0 {
		t.Fatal("una migración fallida debe deshacerse completa")
	}
	if a.Version() != len(antes) {
		t.Fatal("la versión no debe avanzar")
	}
}

func TestTextosCifradosEnLaBase(t *testing.T) {
	a := nuevo(t)
	chatBasico(t, a, "5215512345678@s.whatsapp.net", "5215512345678", "Ana", false)
	m := msg("5215512345678@s.whatsapp.net", "A1", 100, "contraseña del wifi: secreto123")
	m.ArchivoNombre = "factura_privada.pdf"
	m.ArchivoInfo = []byte(`{"mk":"clave-de-media"}`)
	if ok, err := a.GuardarMensaje(m); err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
	var texto, nombre, info []byte
	if err := a.db.QueryRow("SELECT texto_cifrado, archivo_nombre_cifrado, archivo_info_cifrado FROM messages").Scan(&texto, &nombre, &info); err != nil {
		t.Fatal(err)
	}
	for _, claro := range []string{"secreto123", "factura_privada", "clave-de-media"} {
		for _, b := range [][]byte{texto, nombre, info} {
			if bytes.Contains(b, []byte(claro)) {
				t.Fatalf("%q aparece en claro en la base", claro)
			}
		}
	}
	// El archivo de la base tampoco debe contener el texto.
	_, _ = a.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	crudo, _ := os.ReadFile(filepath.Join(a.raiz, "messages.db"))
	if bytes.Contains(crudo, []byte("secreto123")) {
		t.Fatal("el texto está en claro en messages.db")
	}
	got, err := a.ObtenerMensaje(1, "5215512345678@s.whatsapp.net", "A1")
	if err != nil || got == nil || got.Texto != m.Texto || got.ArchivoNombre != m.ArchivoNombre || string(got.ArchivoInfo) != string(m.ArchivoInfo) {
		t.Fatalf("no se leyó igual: %+v %v", got, err)
	}
	// Lo que no se cifra sigue legible para listar y ordenar.
	var tipo string
	var ts int64
	_ = a.db.QueryRow("SELECT tipo, ts FROM messages").Scan(&tipo, &ts)
	if tipo != TipoTexto || ts != 100 {
		t.Fatal("tipo y hora deben estar en claro")
	}
}

func TestLlaveEquivocadaNoLeeTextos(t *testing.T) {
	a := nuevo(t)
	chatBasico(t, a, "1@s.whatsapp.net", "5215512345678", "Ana", false)
	_, _ = a.GuardarMensaje(msg("1@s.whatsapp.net", "A1", 1, "hola"))
	otra := make([]byte, 32)
	b := &Almacen{db: a.db, raiz: a.raiz, clave: otra}
	m, _ := b.ObtenerMensaje(1, "1@s.whatsapp.net", "A1")
	if m == nil || m.Texto != "" {
		t.Fatal("con otra llave no se debe poder leer el texto")
	}
}

func TestRepetidosYEdicionYBorrado(t *testing.T) {
	a := nuevo(t)
	c := "1@s.whatsapp.net"
	chatBasico(t, a, c, "525512345678", "Ana", false)
	if ok, _ := a.GuardarMensaje(msg(c, "A1", 10, "uno")); !ok {
		t.Fatal("era nuevo")
	}
	if ok, _ := a.GuardarMensaje(msg(c, "A1", 10, "otro texto")); ok {
		t.Fatal("un repetido se ignora")
	}
	if m, _ := a.ObtenerMensaje(1, c, "A1"); m.Texto != "uno" {
		t.Fatal("el repetido no debe pisar al original")
	}
	if ok, _ := a.EditarTexto(1, c, "A1", "uno editado"); !ok {
		t.Fatal("debía editar")
	}
	m, _ := a.ObtenerMensaje(1, c, "A1")
	if m.Texto != "uno editado" || !m.Editado {
		t.Fatalf("%+v", m)
	}
	if ok, _ := a.EditarTexto(1, c, "NOEXISTE", "x"); ok {
		t.Fatal("no existe")
	}
	if ok, _ := a.MarcarBorrado(1, c, "A1"); !ok {
		t.Fatal("debía marcar")
	}
	m, _ = a.ObtenerMensaje(1, c, "A1")
	if !m.Borrado || m.Texto != "uno editado" {
		t.Fatal("el borrado se conserva en la base")
	}
}

func TestListarChatsYBusqueda(t *testing.T) {
	a := nuevo(t)
	datos := []struct {
		jid, tel, pub string
		grupo         bool
		ts            int64
	}{
		{"a@s.whatsapp.net", "5215512345678", "María López", false, 300},
		{"b@s.whatsapp.net", "525598765432", "José Peña", false, 200},
		{"c@s.whatsapp.net", "34612345678", "Carlos", false, 100},
		{"g1@g.us", "", "Familia", true, 250},
	}
	for _, d := range datos {
		chatBasico(t, a, d.jid, d.tel, d.pub, d.grupo)
		if _, err := a.GuardarMensaje(msg(d.jid, "M"+d.pub, d.ts, "hola desde "+d.pub)); err != nil {
			t.Fatal(err)
		}
	}
	// chat sin mensajes: no aparece
	chatBasico(t, a, "vacio@s.whatsapp.net", "5210000000000", "Nadie", false)

	pers, err := a.ListarChats(1, false, "", 50, 0)
	if err != nil || len(pers) != 3 {
		t.Fatalf("personales: %d %v", len(pers), err)
	}
	if pers[0].Nombre != "María López" || pers[1].Nombre != "José Peña" || pers[2].Nombre != "Carlos" {
		t.Fatalf("orden: %v %v %v", pers[0].Nombre, pers[1].Nombre, pers[2].Nombre)
	}
	if pers[0].Telefono != "+52 55 1234 5678" || pers[0].Pais != "MX" || pers[2].Pais != "ES" {
		t.Fatalf("teléfono/país: %+v", pers[0])
	}
	if pers[0].UltimoTexto != "hola desde María López" || pers[0].UltimoTipo != TipoTexto {
		t.Fatalf("último: %+v", pers[0])
	}
	gr, _ := a.ListarChats(1, true, "", 50, 0)
	if len(gr) != 1 || gr[0].Nombre != "Familia" || gr[0].Telefono != "" {
		t.Fatalf("grupos: %+v", gr)
	}
	// búsqueda por nombre sin acentos, por número y sin resultados
	for consulta, esperado := range map[string]string{"jose": "José Peña", "MARIA": "María López", "9876": "José Peña", "34612": "Carlos"} {
		r, _ := a.ListarChats(1, false, consulta, 50, 0)
		if len(r) != 1 || r[0].Nombre != esperado {
			t.Errorf("%q: %+v", consulta, r)
		}
	}
	if r, _ := a.ListarChats(1, false, "zzz", 50, 0); len(r) != 0 {
		t.Error("debía quedar vacío")
	}
	// los comodines de LIKE no se interpretan
	if r, _ := a.ListarChats(1, false, "%", 50, 0); len(r) != 0 {
		t.Error("el % no debe actuar como comodín")
	}
	// paginación
	p, _ := a.ListarChats(1, false, "", 2, 2)
	if len(p) != 1 || p[0].Nombre != "Carlos" {
		t.Errorf("página: %+v", p)
	}
	// La lista nunca expone JID como nombre.
	for _, c := range pers {
		if strings.Contains(c.Nombre, "@") {
			t.Fatalf("nombre con JID: %q", c.Nombre)
		}
	}
}

func TestListarMensajesPaginado(t *testing.T) {
	a := nuevo(t)
	c := "1@s.whatsapp.net"
	chatBasico(t, a, c, "525512345678", "Ana", false)
	var lote []*Mensaje
	for i := 1; i <= 120; i++ {
		lote = append(lote, msg(c, fmt.Sprintf("M%03d", i), int64(1000+i/2), fmt.Sprintf("mensaje %d", i))) // horas repetidas a propósito
	}
	if _, err := a.GuardarMensajes(lote); err != nil {
		t.Fatal(err)
	}
	vistos := map[string]bool{}
	var antesTS int64
	var antesID string
	paginas := 0
	for {
		pag, err := a.ListarMensajes(1, c, antesTS, antesID, 50)
		if err != nil {
			t.Fatal(err)
		}
		if len(pag) == 0 {
			break
		}
		paginas++
		for _, m := range pag {
			if vistos[m.ID] {
				t.Fatalf("repetido %s", m.ID)
			}
			vistos[m.ID] = true
		}
		u := pag[len(pag)-1]
		antesTS, antesID = u.TS, u.ID
	}
	if len(vistos) != 120 || paginas != 3 {
		t.Fatalf("vistos=%d páginas=%d", len(vistos), paginas)
	}
	primera, _ := a.ListarMensajes(1, c, 0, "", 5)
	if primera[0].ID != "M120" {
		t.Fatalf("debe empezar por el más nuevo: %s", primera[0].ID)
	}
}

func TestVerUnaVezSoloGuardaElAviso(t *testing.T) {
	a := nuevo(t)
	c := "1@s.whatsapp.net"
	chatBasico(t, a, c, "525512345678", "Ana", false)
	m := msg(c, "V1", 5, TextoVerUnaVez)
	m.Tipo, m.ArchivoEstado = TipoVerUnaVez, EstadoOmitido
	_, _ = a.GuardarMensaje(m)
	v, _ := a.ListarMensajes(1, c, 0, "", 10)
	if len(v) != 1 || v[0].PuedeBajarse || v[0].Ruta != "" || v[0].Miniatura != "" {
		t.Fatalf("%+v", v)
	}
	l, _ := a.ListarChats(1, false, "", 10, 0)
	if l[0].UltimoTexto != TextoVerUnaVez {
		t.Fatalf("%q", l[0].UltimoTexto)
	}
	arch, _ := a.ListarArchivos(1, "", "", false, 10, 0)
	if len(arch) != 0 {
		t.Fatal("un ver-una-vez no es un archivo")
	}
}

func TestFusionarChat(t *testing.T) {
	a := nuevo(t)
	lid, pn := "999@lid", "525512345678@s.whatsapp.net"
	chatBasico(t, a, lid, "", "Luis", false)
	_, _ = a.GuardarMensaje(msg(lid, "L1", 10, "desde lid"))
	_, _ = a.GuardarMensaje(msg(lid, "DUP", 11, "duplicado"))
	chatBasico(t, a, pn, "525512345678", "", false)
	_, _ = a.GuardarMensaje(msg(pn, "P1", 20, "desde teléfono"))
	_, _ = a.GuardarMensaje(msg(pn, "DUP", 11, "duplicado"))
	if err := a.FusionarChat(1, lid, pn); err != nil {
		t.Fatal(err)
	}
	if a.ExisteChat(1, lid) {
		t.Fatal("el chat LID debía desaparecer")
	}
	v, _ := a.ListarMensajes(1, pn, 0, "", 10)
	if len(v) != 3 {
		t.Fatalf("mensajes tras fusionar: %d", len(v))
	}
	c, _ := a.ObtenerChat(1, pn)
	if c.Mostrado != "Luis" || c.TelMostrado != "+52 55 1234 5678" {
		t.Fatalf("%+v", c)
	}
}

func TestSoloLIDMuestraNumeroNoDisponible(t *testing.T) {
	a := nuevo(t)
	chatBasico(t, a, "777@lid", "", "Elena", false)
	c, _ := a.ObtenerChat(1, "777@lid")
	if c.Mostrado != "Elena" || c.TelMostrado != "Número no disponible" {
		t.Fatalf("%+v", c)
	}
	chatBasico(t, a, "888@lid", "", "", false)
	c, _ = a.ObtenerChat(1, "888@lid")
	if c.Mostrado != "Número no disponible" || strings.Contains(c.Mostrado, "@") {
		t.Fatalf("%+v", c)
	}
}

// ---------- archivos, espacio, borrado y reparación ----------

func conArchivo(t *testing.T, a *Almacen, chat, id string, tipo string, tam int, ts int64) *Mensaje {
	t.Helper()
	m := msg(chat, id, ts, "")
	m.Tipo = tipo
	m.ArchivoMime = "application/octet-stream"
	m.ArchivoInfo = []byte(`{"x":1}`)
	m.ArchivoBytes = int64(tam)
	m.ArchivoRuta = RutaMedio(1, chat, id)
	m.MiniaturaRuta = RutaMiniatura(1, chat, id)
	m.ArchivoEstado = EstadoDescargado
	plano := bytes.Repeat([]byte{7}, tam)
	if err := a.EscribirCifrado(m.ArchivoRuta, plano); err != nil {
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

func escenarioEspacio(t *testing.T) (*Almacen, string, string) {
	a := nuevo(t)
	c1, c2 := "1@s.whatsapp.net", "2@s.whatsapp.net"
	chatBasico(t, a, c1, "525512345678", "Ana", false)
	chatBasico(t, a, c2, "525598765432", "Beto", false)
	ahora := time.Now().Unix()
	viejo := ahora - 40*86400
	conArchivo(t, a, c1, "F1", TipoImagen, 1000, ahora)
	conArchivo(t, a, c1, "V1", TipoVideo, 200000, viejo)
	conArchivo(t, a, c1, "A1", TipoAudio, 5000, viejo)
	conArchivo(t, a, c2, "D1", TipoDocumento, 70000, ahora)
	conArchivo(t, a, c2, "S1", TipoSticker, 300, ahora)
	return a, c1, c2
}

func TestEspacioPorTipoYPorChat(t *testing.T) {
	a, c1, _ := escenarioEspacio(t)
	porTipo, err := a.EspacioPorTipo(1)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	for _, e := range porTipo {
		got[e.Categoria] = e.Bytes
	}
	if got[CatFoto] != 1300 || got[CatVideo] != 200000 || got[CatAudio] != 5000 || got[CatDocumento] != 70000 {
		t.Fatalf("%v", got)
	}
	porChat, _ := a.EspacioPorChat(1)
	if len(porChat) != 2 || porChat[0].JID != c1 || porChat[0].Bytes != 206000 || porChat[1].Bytes != 70300 {
		t.Fatalf("%+v", porChat)
	}
	tot := a.EspacioTotal(1)
	if tot.Multimedia <= 0 || tot.Miniaturas <= 0 || tot.Base <= 0 || tot.Libre <= 0 {
		t.Fatalf("%+v", tot)
	}
}

func TestEstimarYBorrarPorFiltro(t *testing.T) {
	a, c1, c2 := escenarioEspacio(t)
	// videos
	b, n, _ := a.EstimarBorrado(1, Filtro{Categoria: CatVideo})
	if b != 200000 || n != 1 {
		t.Fatalf("estimar videos %d %d", b, n)
	}
	// más de 30 días: video + audio de c1
	b, n, _ = a.EstimarBorrado(1, Filtro{DiasMin: 30})
	if b != 205000 || n != 2 {
		t.Fatalf("estimar antiguos %d %d", b, n)
	}
	// el estimado no borra
	if p, _ := a.ListarArchivos(1, "", "", true, 100, 0); len(p) != 5 {
		t.Fatal("estimar no debe borrar")
	}
	rutaVideo, _ := a.Abs(RutaMedio(1, c1, "V1"))
	bytesLib, n, err := a.BorrarArchivos(1, Filtro{Categoria: CatVideo})
	if err != nil || bytesLib != 200000 || n != 1 {
		t.Fatalf("borrar videos %d %d %v", bytesLib, n, err)
	}
	if _, err := os.Stat(rutaVideo); !os.IsNotExist(err) {
		t.Fatal("el archivo debía desaparecer del disco")
	}
	m, _ := a.ObtenerMensaje(1, c1, "V1")
	if m.ArchivoEstado != EstadoBorrado || m.ArchivoRuta != "" || m.MiniaturaRuta != "" || m.Borrado {
		t.Fatalf("%+v", m)
	}
	// borrar los de un chat conserva los mensajes
	_, n, _ = a.BorrarArchivos(1, Filtro{Chat: c2})
	if n != 2 {
		t.Fatalf("borrar chat: %d", n)
	}
	if v, _ := a.ListarMensajes(1, c2, 0, "", 10); len(v) != 2 {
		t.Fatal("los mensajes se conservan")
	}
	// borrar todo lo multimedia
	_, n, _ = a.BorrarArchivos(1, Filtro{})
	if n != 2 {
		t.Fatalf("resto: %d", n)
	}
	if e := a.EspacioTotal(1); e.Multimedia != 0 || e.Miniaturas != 0 {
		t.Fatalf("debía quedar vacío: %+v", e)
	}
}

func TestBorrarListaYChatCompleto(t *testing.T) {
	a, c1, c2 := escenarioEspacio(t)
	b, n, _ := a.EstimarLista(1, []Item{{c1, "F1"}, {c2, "D1"}, {c2, "NOEXISTE"}})
	if b != 71000 || n != 2 {
		t.Fatalf("estimar lista %d %d", b, n)
	}
	if _, n, _ := a.BorrarLista(1, []Item{{c1, "F1"}, {c2, "D1"}}); n != 2 {
		t.Fatalf("borrar lista %d", n)
	}
	dirC1 := filepath.Join(a.raiz, "media", "1", HashChat(1, c1))
	if _, err := os.Stat(dirC1); err != nil {
		t.Fatal("la carpeta de c1 aún debe existir")
	}
	// foto de perfil de c1
	_ = a.EscribirCifrado(RutaFoto(1, c1, false), []byte("foto"))
	_ = a.FijarFoto(1, c1, RutaFoto(1, c1, false), "ID1", 5)
	if _, err := a.BorrarChat(1, c1); err != nil {
		t.Fatal(err)
	}
	if a.ExisteChat(1, c1) {
		t.Fatal("el chat debía borrarse")
	}
	if v, _ := a.ListarMensajes(1, c1, 0, "", 10); len(v) != 0 {
		t.Fatal("sus mensajes debían borrarse (cascada)")
	}
	if _, err := os.Stat(dirC1); !os.IsNotExist(err) {
		t.Fatal("su carpeta de archivos debía borrarse")
	}
	if foto, _ := a.Abs(RutaFoto(1, c1, false)); true {
		if _, err := os.Stat(foto); !os.IsNotExist(err) {
			t.Fatal("su foto debía borrarse")
		}
	}
	if !a.ExisteChat(1, c2) {
		t.Fatal("el otro chat no se toca")
	}
}

func TestReparar(t *testing.T) {
	a, c1, c2 := escenarioEspacio(t)
	// 1) archivo que desapareció
	faltante, _ := a.Abs(RutaMedio(1, c1, "F1"))
	_ = os.Remove(faltante)
	// 2) tamaño mal anotado
	_, _ = a.db.Exec("UPDATE messages SET archivo_bytes=1 WHERE id='D1'")
	// 3) archivo dañado (cortado)
	dañado, _ := a.Abs(RutaMedio(1, c1, "A1"))
	cif, _ := os.ReadFile(dañado)
	_ = os.WriteFile(dañado, cif[:len(cif)-5], 0o600)
	// 4) archivo huérfano y miniatura que falta
	huerfano := filepath.Join(a.raiz, "media", "1", HashChat(1, c2), "huerfano.enc")
	_ = os.WriteFile(huerfano, []byte("sobrante"), 0o600)
	miniFalta, _ := a.Abs(RutaMiniatura(1, c2, "S1"))
	_ = os.Remove(miniFalta)
	// 5) foto perdida
	_ = a.FijarFoto(1, c2, RutaFoto(1, c2, false), "X", 9)
	// 6) hora del chat desfasada
	_, _ = a.db.Exec("UPDATE chats SET ultimo_ts=1 WHERE jid=?", c2)

	rep, err := a.Reparar(1)
	if err != nil {
		t.Fatal(err)
	}
	if rep.EstadosCorregidos != 2 || rep.TamanosCorregidos != 1 || rep.ArchivosDanados != 1 || rep.Huerfanos != 1 ||
		rep.MiniaturasLimpiadas != 1 || rep.FotosLimpiadas != 1 || rep.ChatsRecalculados != 1 {
		t.Fatalf("%+v", rep)
	}
	if m, _ := a.ObtenerMensaje(1, c1, "F1"); m.ArchivoEstado != EstadoFallido {
		t.Fatalf("F1: %s", m.ArchivoEstado)
	}
	if m, _ := a.ObtenerMensaje(1, c1, "A1"); m.ArchivoEstado != EstadoFallido {
		t.Fatalf("A1: %s", m.ArchivoEstado)
	}
	if m, _ := a.ObtenerMensaje(1, c2, "D1"); m.ArchivoBytes != 70000 {
		t.Fatalf("D1: %d", m.ArchivoBytes)
	}
	if _, err := os.Stat(huerfano); !os.IsNotExist(err) {
		t.Fatal("el huérfano debía borrarse")
	}
	// Una segunda pasada no encuentra nada más.
	rep2, _ := a.Reparar(1)
	if rep2 != (Reporte{}) {
		t.Fatalf("segunda pasada: %+v", rep2)
	}
	// Un fallido puede volver a bajarse.
	m, _ := a.ObtenerMensaje(1, c1, "F1")
	if !a.vista(m).PuedeBajarse {
		t.Fatal("un archivo fallido debe poder descargarse otra vez")
	}
}

func TestRutasNoSalenDeLaRaiz(t *testing.T) {
	a := nuevo(t)
	for _, r := range []string{"../fuera", "media/../../x", "/etc/passwd"} {
		if p, err := a.Abs(r); err == nil && !strings.HasPrefix(p, a.raiz) {
			t.Errorf("%q salió de la raíz: %s", r, p)
		}
	}
	if _, err := a.Abs("../fuera"); err == nil {
		t.Error("debía rechazar ..")
	}
	if got := nombreSeguro("../../etc"); strings.ContainsAny(got, "./") {
		t.Errorf("nombre inseguro: %s", got)
	}
}

func TestTamCifradoCoincideConElDisco(t *testing.T) {
	for _, n := range []int{0, 1, 65535, 65536, 65537, 200000} {
		c, _ := cifrado.CifrarBytes(make([]byte, 32), make([]byte, n))
		if int64(len(c)) != TamCifrado(int64(n)) {
			t.Errorf("n=%d: %d vs %d", n, len(c), TamCifrado(int64(n)))
		}
	}
}

func TestEliminarBot(t *testing.T) {
	a, c1, _ := escenarioEspacio(t)
	if err := a.EliminarBot(1); err != nil {
		t.Fatal(err)
	}
	if a.ExisteChat(1, c1) {
		t.Fatal("debía borrarse todo")
	}
	if _, err := os.Stat(filepath.Join(a.raiz, "media", "1")); !os.IsNotExist(err) {
		t.Fatal("su carpeta de media debía borrarse")
	}
}

var _ = sql.ErrNoRows

func TestEscribirCifradoDesdeNoDejaNadaEnClaro(t *testing.T) {
	a := nuevo(t)
	secreto := bytes.Repeat([]byte("CONTENIDO-SECRETO-"), 20000) // varios bloques
	n, err := a.EscribirCifradoDesde("media/1/abc/X.enc", bytes.NewReader(secreto))
	if err != nil || n != int64(len(secreto)) {
		t.Fatalf("%d %v", n, err)
	}
	abs, _ := a.Abs("media/1/abc/X.enc")
	crudo, _ := os.ReadFile(abs)
	if bytes.Contains(crudo, []byte("CONTENIDO-SECRETO")) {
		t.Fatal("el archivo está en claro")
	}
	if int64(len(crudo)) != TamCifrado(n) {
		t.Fatalf("tamaño %d vs %d", len(crudo), TamCifrado(n))
	}
	if _, err := os.Stat(abs + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("quedó un .tmp")
	}
	back, err := a.LeerCifrado("media/1/abc/X.enc")
	if err != nil || !bytes.Equal(back, secreto) {
		t.Fatalf("ida y vuelta: %v", err)
	}
	// Si la lectura falla a medias no queda archivo.
	_, err = a.EscribirCifradoDesde("media/1/abc/Y.enc", io.MultiReader(bytes.NewReader([]byte("abc")), errReader{}))
	if err == nil {
		t.Fatal("debía fallar")
	}
	if y, _ := a.Abs("media/1/abc/Y.enc"); true {
		if _, err := os.Stat(y); !os.IsNotExist(err) {
			t.Fatal("no debe quedar archivo a medias")
		}
		if _, err := os.Stat(y + ".tmp"); !os.IsNotExist(err) {
			t.Fatal("no debe quedar .tmp")
		}
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("se cortó la red") }

func TestChatPorJID(t *testing.T) {
	a := nuevo(t)
	pers, grupo := "5215512345678@s.whatsapp.net", "120363@g.us"
	chatBasico(t, a, pers, "5215512345678", "Ana", false)
	chatBasico(t, a, grupo, "", "Familia", true)
	_, _ = a.GuardarMensaje(msg(pers, "M1", 10, "hola"))
	_, _ = a.GuardarMensaje(msg(grupo, "G1", 11, "hola grupo"))
	for jid, nombre := range map[string]string{pers: "Ana", grupo: "Familia"} {
		c, err := a.ChatPorJID(1, jid)
		if err != nil || c == nil || c.Nombre != nombre {
			t.Fatalf("%s: %+v %v", jid, c, err)
		}
	}
	if c, err := a.ChatPorJID(1, "no@existe"); err != nil || c != nil {
		t.Fatalf("debía ser nil: %+v %v", c, err)
	}
}
