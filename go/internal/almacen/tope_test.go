package almacen

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

// viejoEnDisco hace que el archivo de un mensaje parezca bajado hace dos horas (la protección de la última hora mira la hora del archivo).
func viejoEnDisco(t *testing.T, a *Almacen, m *Mensaje) {
	t.Helper()
	for _, r := range []string{m.ArchivoRuta} {
		abs, _ := a.Abs(r)
		d := time.Now().Add(-2 * time.Hour)
		if err := os.Chtimes(abs, d, d); err != nil {
			t.Fatal(err)
		}
	}
}

func estadoArch(t *testing.T, a *Almacen, chat, id string) string {
	t.Helper()
	m, _ := a.ObtenerMensaje(1, chat, id)
	return m.ArchivoEstado
}

func TestAplicarTopeBorraLoMasViejoHastaElNoventaPorCiento(t *testing.T) {
	a := nuevo(t)
	c := "1@s.whatsapp.net"
	chatBasico(t, a, c, "525512345678", "Ana", false)
	base := time.Now().Add(-48 * time.Hour).Unix()
	var ms []*Mensaje
	for i := 0; i < 10; i++ {
		m := conArchivo(t, a, c, string(rune('A'+i)), TipoDocumento, 10000, base+int64(i))
		viejoEnDisco(t, a, m)
		ms = append(ms, m)
	}
	uso := a.UsoMultimedia(1)
	tope := uso - uso/10 // pasa del tope por un 11 %
	r, err := a.AplicarTope(1, tope, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if r.Borrados != 2 || r.Despues > tope/10*9 {
		t.Fatalf("resultado: %+v (objetivo %d)", r, tope/10*9)
	}
	// Se borraron los dos más viejos y quedaron los demás.
	for i, m := range ms {
		est := estadoArch(t, a, c, m.ID)
		if (i < 2) != (est == EstadoBorrado) {
			t.Fatalf("%s: %s", m.ID, est)
		}
	}
	// Anotó motivo y hora.
	var motivo string
	var ts int64
	_ = a.db.QueryRow(`SELECT archivo_borrado_motivo, archivo_borrado_ts FROM messages WHERE id='A'`).Scan(&motivo, &ts)
	if motivo != "tope" || ts == 0 {
		t.Fatalf("motivo %q ts %d", motivo, ts)
	}
	// Aviso local.
	av, _ := a.AvisosRecientes(1, 5)
	if len(av) != 1 || !strings.Contains(av[0].Titulo, "Se borraron 2 archivos viejos") {
		t.Fatalf("avisos: %+v", av)
	}
	// Una segunda corrida ya no hace nada.
	if r2, _ := a.AplicarTope(1, tope, time.Now()); r2.Borrados != 0 {
		t.Fatalf("segunda corrida: %+v", r2)
	}
	// Sin límite (0) no borra nada.
	if r3, _ := a.AplicarTope(1, 0, time.Now()); r3.Borrados != 0 {
		t.Fatal("sin límite")
	}
}

func TestAplicarTopeNuncaBorraLoProtegido(t *testing.T) {
	a := nuevo(t)
	c1, c2 := "1@s.whatsapp.net", "2@s.whatsapp.net"
	chatBasico(t, a, c1, "525512345678", "Ana", false)
	chatBasico(t, a, c2, "525598765432", "Beto", false)
	et, err := a.CrearEtiqueta(1, "Importante", "#2EC4CB")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`INSERT INTO chat_etiqueta(bot_id,chat_jid,etiqueta_id) VALUES(1,?,?)`, c2, et); err != nil {
		t.Fatal(err)
	}
	base := time.Now().Add(-48 * time.Hour).Unix()
	protegido := conArchivo(t, a, c2, "ETIQ", TipoDocumento, 5000, base) // chat etiquetado
	sinPortada := conArchivo(t, a, c1, "VID0", TipoVideo, 5000, base+1)  // video sin portada
	uv := conArchivo(t, a, c1, "UV", TipoVerUnaVez, 5000, base+2)
	reciente := conArchivo(t, a, c1, "REC", TipoDocumento, 5000, time.Now().Add(-10*time.Minute).Unix())
	borrable := conArchivo(t, a, c1, "OK1", TipoDocumento, 5000, base+3)
	conPortada := conArchivo(t, a, c1, "VID1", TipoVideo, 5000, base+4)
	if err := a.GuardarPortada(1, c1, "VID1", []byte("jpeg-de-prueba")); err != nil {
		t.Fatal(err)
	}
	for _, m := range []*Mensaje{protegido, sinPortada, uv, reciente, borrable, conPortada} {
		viejoEnDisco(t, a, m)
	}
	// "REC" llegó hace 10 minutos aunque su archivo se vea viejo.
	r, err := a.AplicarTope(1, 1, time.Now()) // tope ridículo: todo lo borrable se va
	if err != nil {
		t.Fatal(err)
	}
	if r.Borrados != 2 {
		t.Fatalf("debían borrarse OK1 y VID1: %+v", r)
	}
	for id, esperado := range map[string]string{"ETIQ": EstadoDescargado, "VID0": EstadoDescargado, "UV": EstadoDescargado, "REC": EstadoDescargado, "OK1": EstadoBorrado, "VID1": EstadoBorrado} {
		chat := c1
		if id == "ETIQ" {
			chat = c2
		}
		if got := estadoArch(t, a, chat, id); got != esperado {
			t.Errorf("%s: %s, esperado %s", id, got, esperado)
		}
	}
}

func TestBorrarVideoConservaPortada(t *testing.T) {
	a := nuevo(t)
	c := "1@s.whatsapp.net"
	chatBasico(t, a, c, "525512345678", "Ana", false)
	m := conArchivo(t, a, c, "V", TipoVideo, 4000, 100)
	m.DuracionS = 12
	if _, err := a.db.Exec(`UPDATE messages SET duracion_s=12 WHERE id='V'`); err != nil {
		t.Fatal(err)
	}
	if err := a.GuardarPortada(1, c, "V", []byte("portada-secreta")); err != nil {
		t.Fatal(err)
	}
	if v, _ := a.VideosSinPortada(1, 10); len(v) != 0 {
		t.Fatalf("ya tiene portada: %+v", v)
	}
	if _, n, err := a.BorrarLista(1, []Item{{c, "V"}}); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	got, _ := a.ObtenerMensaje(1, c, "V")
	if got.ArchivoEstado != EstadoBorrado || got.ArchivoRuta != "" || got.DuracionS != 12 || got.ArchivoBytes != 4000 || got.MiniaturaRuta == "" {
		t.Fatalf("%+v", got)
	}
	ruta := a.PortadaRuta(1, c, "V")
	if b, err := a.LeerCifrado(ruta); err != nil || string(b) != "portada-secreta" {
		t.Fatalf("la portada debe seguir: %v", err)
	}
	// El archivo en disco de la portada está cifrado.
	abs, _ := a.Abs(ruta)
	crudo, _ := os.ReadFile(abs)
	if bytes.Contains(crudo, []byte("portada-secreta")) {
		t.Fatal("portada en claro")
	}
	// Reparar no la toma por huérfana.
	if rep, _ := a.Reparar(1); rep.Huerfanos != 0 {
		t.Fatalf("reparar: %+v", rep)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Fatal("reparar borró la portada")
	}
	// Con el interruptor apagado, se borra todo.
	m2 := conArchivo(t, a, c, "V2", TipoVideo, 4000, 101)
	_ = m2
	_ = a.GuardarPortada(1, c, "V2", []byte("otra"))
	_ = a.FijarConservarPortada(1, false)
	_, _, _ = a.BorrarLista(1, []Item{{c, "V2"}})
	if a.PortadaRuta(1, c, "V2") != "" {
		t.Fatal("con el interruptor apagado la portada también se va")
	}
	if _, err := os.Stat(strings.Replace(abs, "V.cover", "V2.cover", 1)); err == nil {
		t.Fatal("el archivo de la portada seguía")
	}
	// Portada de algo que no es video.
	conArchivo(t, a, c, "D", TipoDocumento, 100, 102)
	if err := a.GuardarPortada(1, c, "D", []byte("x")); err == nil {
		t.Fatal("solo los videos tienen portada")
	}
}

func TestFiltroPorPersonaYFecha(t *testing.T) {
	a := nuevo(t)
	g := "120363@g.us"
	chatBasico(t, a, g, "", "Grupo", true)
	chatBasico(t, a, "1@s.whatsapp.net", "525512345678", "Ana", false)
	ahora := time.Now().Unix()
	mk := func(id, quien string, ts int64) {
		m := conArchivo(t, a, g, id, TipoImagen, 100, ts)
		if _, err := a.db.Exec(`UPDATE messages SET remitente_jid=? WHERE id=?`, quien, id); err != nil {
			t.Fatal(err)
		}
		_ = m
	}
	mk("a1", "5215511111111@s.whatsapp.net", ahora-10*86400)
	mk("a2", "5215511111111@s.whatsapp.net", ahora-1*86400)
	mk("b1", "5215522222222@s.whatsapp.net", ahora-10*86400)
	if _, n, _ := a.EstimarBorrado(1, Filtro{Persona: "5215511111111@s.whatsapp.net"}); n != 2 {
		t.Fatalf("persona: %d", n)
	}
	if _, n, _ := a.EstimarBorrado(1, Filtro{Persona: "5215511111111@s.whatsapp.net", Hasta: ahora - 5*86400}); n != 1 {
		t.Fatalf("persona hasta: %d", n)
	}
	if _, n, _ := a.EstimarBorrado(1, Filtro{Desde: ahora - 11*86400, Hasta: ahora - 9*86400}); n != 2 {
		t.Fatalf("fecha: %d", n)
	}
	if _, n, err := a.BorrarArchivos(1, Filtro{Persona: "5215522222222@s.whatsapp.net"}); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if estadoArch(t, a, g, "b1") != EstadoBorrado || estadoArch(t, a, g, "a1") != EstadoDescargado {
		t.Fatal("borró a quien no era")
	}
}

func TestTopeTotalAjustes(t *testing.T) {
	a := nuevo(t)
	if a.TopeTotal(1) != TopeDefecto {
		t.Fatal("de fábrica 5 GB")
	}
	_ = a.FijarTopeTotal(1, 0)
	if a.TopeTotal(1) != 0 {
		t.Fatal("sin límite")
	}
	_ = a.FijarTopeTotal(1, 123)
	if a.TopeTotal(1) != 123 {
		t.Fatal("editable")
	}
	if !a.ConservarPortada(1) {
		t.Fatal("conservar portada de fábrica")
	}
}

// v0.6.0 E9: al actualizar con más archivos que el tope, el tope NO borra nada hasta que se confirma el aviso.
func prepararPasado(t *testing.T) (*Almacen, int64, []*Mensaje) {
	a := nuevo(t)
	c := "1@s.whatsapp.net"
	chatBasico(t, a, c, "525512345678", "Ana", false)
	base := time.Now().Add(-48 * time.Hour).Unix()
	var ms []*Mensaje
	for i := 0; i < 10; i++ {
		m := conArchivo(t, a, c, string(rune('A'+i)), TipoDocumento, 10000, base+int64(i))
		viejoEnDisco(t, a, m)
		ms = append(ms, m)
	}
	uso := a.UsoMultimedia(1)
	return a, uso - uso/10, ms
}

func TestTopePendienteAlActualizarNoBorraHastaConfirmar(t *testing.T) {
	a, tope, ms := prepararPasado(t)
	_ = a.FijarAjusteBot(1, "audit_ret_pendiente", "1") // marca de la migración 20: viene de una versión anterior
	if err := a.FijarTopeTotal(1, tope); err != nil {
		t.Fatal(err)
	}
	a.InicializarTope(1)
	if !a.TopePendiente(1) {
		t.Fatal("debía quedar pendiente")
	}
	r, err := a.AplicarTope(1, tope, time.Now())
	if err != nil || r.Borrados != 0 {
		t.Fatalf("pendiente no debe borrar: %+v %v", r, err)
	}
	for _, m := range ms {
		if estadoArch(t, a, "1@s.whatsapp.net", m.ID) == EstadoBorrado {
			t.Fatal("se borró un archivo con el tope pendiente")
		}
	}
	// Reiniciar (volver a inicializar) no lo confirma ni lo cambia.
	a.InicializarTope(1)
	if !a.TopePendiente(1) {
		t.Fatal("la segunda inicialización no debe cambiar nada")
	}
	if err := a.ConfirmarTope(1); err != nil {
		t.Fatal(err)
	}
	if a.TopePendiente(1) {
		t.Fatal("debía quedar confirmado")
	}
	if r, _ := a.AplicarTope(1, tope, time.Now()); r.Borrados == 0 {
		t.Fatalf("confirmado debe borrar: %+v", r)
	}
}

func TestTopeInstalacionNuevaYActualizacionSinExcesoEstanConfirmados(t *testing.T) {
	// Instalación nueva (sin marca de actualización) aunque pase del tope: confirmado y borra.
	a, tope, _ := prepararPasado(t)
	_ = a.FijarTopeTotal(1, tope)
	a.InicializarTope(1)
	if a.TopePendiente(1) {
		t.Fatal("instalación nueva debe estar confirmada")
	}
	if r, _ := a.AplicarTope(1, tope, time.Now()); r.Borrados == 0 {
		t.Fatalf("debía borrar: %+v", r)
	}
	// Actualización que NO pasa del tope: confirmado.
	b, _, _ := prepararPasado(t)
	_ = b.FijarAjusteBot(1, "audit_ret_pendiente", "1")
	_ = b.FijarTopeTotal(1, b.UsoMultimedia(1)*2)
	b.InicializarTope(1)
	if b.TopePendiente(1) {
		t.Fatal("sin exceso no hay nada pendiente")
	}
}

func TestSubirOQuitarElTopeNoConfirmaPorSiSolo(t *testing.T) {
	a, tope, _ := prepararPasado(t)
	_ = a.FijarAjusteBot(1, "audit_ret_pendiente", "1")
	_ = a.FijarTopeTotal(1, tope)
	a.InicializarTope(1)
	_ = a.FijarTopeTotal(1, tope/2) // bajarlo desde otra pantalla tampoco borra
	if r, _ := a.AplicarTope(1, tope/2, time.Now()); r.Borrados != 0 {
		t.Fatalf("pendiente: %+v", r)
	}
}
