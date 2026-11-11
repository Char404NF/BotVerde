package almacen

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// oggOpus arma la cabecera de un archivo Ogg con un flujo Opus de `canales` canales (lo justo para validarlo).
func oggOpus(canales byte) []byte {
	var b bytes.Buffer
	b.WriteString("OggS")
	b.WriteByte(0)    // versión
	b.WriteByte(0x02) // primera página
	b.Write(make([]byte, 8))
	b.Write([]byte{1, 0, 0, 0}) // serie
	b.Write(make([]byte, 4))    // secuencia
	b.Write(make([]byte, 4))    // crc
	b.WriteByte(1)              // un segmento
	b.WriteByte(19)
	b.WriteString("OpusHead")
	b.WriteByte(1) // versión
	b.WriteByte(canales)
	b.Write([]byte{0x38, 0x01})       // pre-salto
	b.Write([]byte{0x80, 0xBB, 0, 0}) // 48000 Hz
	b.Write([]byte{0, 0})             // ganancia
	b.WriteByte(0)                    // familia de canales
	// segunda página (para la duración): 96000 muestras a 48 kHz = 2 s
	b.WriteString("OggS")
	b.Write([]byte{0, 0x04})
	var g [8]byte
	binary.LittleEndian.PutUint64(g[:], 96000)
	b.Write(g[:])
	b.Write(make([]byte, 16))
	return b.Bytes()
}

func oggVorbis() []byte {
	var b bytes.Buffer
	b.WriteString("OggS")
	b.Write([]byte{0, 2})
	b.Write(make([]byte, 8+4+4+4))
	b.WriteByte(1)
	b.WriteByte(30)
	b.WriteString("\x01vorbis")
	b.Write(make([]byte, 30))
	return b.Bytes()
}

func TestValidarNotaDeVozLeeLaCabeceraOpusHead(t *testing.T) {
	if err := ValidarNotaDeVoz(oggOpus(1)); err != nil {
		t.Fatalf("mono Opus válido: %v", err)
	}
	if err := ValidarNotaDeVoz(oggOpus(2)); err != ErrNoMono {
		t.Fatalf("estéreo: %v", err)
	}
	if err := ValidarNotaDeVoz(oggVorbis()); err != ErrNoOpus {
		t.Fatalf("Vorbis no es Opus: %v", err)
	}
	if err := ValidarNotaDeVoz([]byte("ID3\x03\x00 un mp3")); err != ErrNoOgg {
		t.Fatalf("mp3: %v", err)
	}
	if err := ValidarNotaDeVoz(oggOpus(1)[:30]); err != ErrOggCorrupto {
		t.Fatalf("cortado: %v", err)
	}
	if err := ValidarNotaDeVoz(nil); err != ErrNoOgg {
		t.Fatalf("vacío: %v", err)
	}
	if d := DuracionOpus(oggOpus(1)); d != 2 {
		t.Fatalf("duración %d", d)
	}
}

func escribe(t *testing.T, dir, nombre string, b []byte) string {
	t.Helper()
	p := filepath.Join(dir, nombre)
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGuardarAdjuntoValidaTipoPesoYNotaDeVoz(t *testing.T) {
	a := nuevo(t)
	dir := t.TempDir()
	// una foto
	r, err := a.GuardarAdjuntoDeArchivo(1, escribe(t, dir, "f.jpg", []byte("jpegdata")), AdjImagen, "image/jpeg", "f.jpg")
	if err != nil || r.ID == "" || !IDAdjuntoValido(r.ID) || r.Bytes != 8 || r.Tipo != AdjImagen {
		t.Fatalf("%+v %v", r, err)
	}
	// queda cifrado en el disco
	abs, _ := a.Abs(RutaAdjuntoMenu(1, r.ID))
	crudo, _ := os.ReadFile(abs)
	if bytes.Contains(crudo, []byte("jpegdata")) {
		t.Fatal("el archivo guardado debe estar cifrado")
	}
	if b, err := a.LeerAdjuntoMenu(1, r.ID); err != nil || string(b) != "jpegdata" {
		t.Fatalf("%q %v", b, err)
	}
	// documento sin nombre toma el del archivo
	d, err := a.GuardarAdjuntoDeArchivo(1, escribe(t, dir, "lista.pdf", []byte("%PDF")), AdjDocumento, "", "")
	if err != nil || d.Nombre != "lista.pdf" || d.Mime == "" {
		t.Fatalf("%+v %v", d, err)
	}
	// tipo no permitido y archivo vacío
	if _, err := a.GuardarAdjuntoDeArchivo(1, escribe(t, dir, "x", []byte("x")), "sticker", "", ""); err == nil {
		t.Fatal("tipo no permitido")
	}
	if _, err := a.GuardarAdjuntoDeArchivo(1, escribe(t, dir, "vacio", nil), AdjImagen, "", ""); err == nil {
		t.Fatal("vacío")
	}
	// demasiado grande
	gran := escribe(t, dir, "gran.jpg", make([]byte, 9<<20))
	if _, err := a.GuardarAdjuntoDeArchivo(1, gran, AdjImagen, "image/jpeg", ""); err == nil || !strings.Contains(err.Error(), "9 MB") && strings.Contains(err.Error(), "8 MB") {
		t.Fatalf("foto de 9 MB: %v", err)
	}
	if _, err := a.GuardarAdjuntoDeArchivo(1, gran, AdjVideo, "video/mp4", "v.mp4"); err != nil {
		t.Fatalf("un video de 9 MB sí cabe (16 MB): %v", err)
	}
	// notas de voz: .ogg Opus mono sí; estéreo, vorbis o mp3 no
	n, err := a.GuardarAdjuntoDeArchivo(1, escribe(t, dir, "n.ogg", oggOpus(1)), AdjAudio, "audio/ogg", "")
	if err != nil || n.Mime != "audio/ogg; codecs=opus" || n.Nombre == "" {
		t.Fatalf("%+v %v", n, err)
	}
	for nombre, b := range map[string][]byte{"e.ogg": oggOpus(2), "v.ogg": oggVorbis(), "m.mp3": []byte("ID3 mp3 mp3 mp3 mp3 mp3")} {
		if _, err := a.GuardarAdjuntoDeArchivo(1, escribe(t, dir, nombre, b), AdjAudio, "", ""); err == nil {
			t.Fatalf("%s no debe aceptarse como nota de voz", nombre)
		}
	}
}

func nuevoAuto(tipo, nombre, texto string, adj ...AdjuntoRef) *AutoMensaje {
	return &AutoMensaje{Tipo: tipo, Nombre: nombre, Texto: texto, Config: `{"cuando":{"siempre":true}}`, Adjuntos: adj, Activo: true}
}

func TestMigracion14Existe(t *testing.T) {
	a := nuevo(t)
	if a.Version() < 14 {
		t.Fatalf("falta la migración 14: %d", a.Version())
	}
	for _, tabla := range []string{"auto_mensajes", "auto_repeticion", "auto_registro"} {
		var n int
		if err := a.db.QueryRow(`SELECT COUNT(*) FROM ` + tabla).Scan(&n); err != nil {
			t.Fatalf("tabla %s: %v", tabla, err)
		}
	}
	if _, err := a.db.Exec(`SELECT automatico FROM messages`); err != nil {
		t.Fatal(err)
	}
}

func TestAutoMensajesSeGuardanCifradosYEnOrden(t *testing.T) {
	a := nuevo(t)
	id1, err := a.CrearAuto(1, nuevoAuto(AutoRecibido, "Recibido", "texto secreto de prueba"))
	if err != nil {
		t.Fatal(err)
	}
	id2, _ := a.CrearAuto(1, nuevoAuto(AutoRecibido, "Otro", "segundo"))
	if _, err := a.CrearAuto(1, nuevoAuto("inventado", "x", "y")); err == nil {
		t.Fatal("tipo desconocido")
	}
	// en la base no hay nada legible
	var t1, c1, a1 []byte
	_ = a.db.QueryRow(`SELECT texto_cifrado, config_cifrado, adjuntos_cifrado FROM auto_mensajes WHERE id=?`, id1).Scan(&t1, &c1, &a1)
	if len(t1) == 0 || bytes.Contains(t1, []byte("secreto")) || bytes.Contains(c1, []byte("siempre")) {
		t.Fatal("texto y configuración van cifrados")
	}
	m, _ := a.ObtenerAuto(1, id1)
	if m.Texto != "texto secreto de prueba" || !m.Activo || m.Orden != 1 || !strings.Contains(m.Config, "siempre") {
		t.Fatalf("%+v", m)
	}
	l, _ := a.ListarAutos(1, AutoRecibido)
	if len(l) != 2 || l[0].ID != id1 {
		t.Fatalf("%+v", l)
	}
	_ = a.ReordenarAutos(1, []int64{id2, id1})
	l, _ = a.ListarAutos(1, AutoRecibido)
	if l[0].ID != id2 {
		t.Fatal("reordenar")
	}
	if err := a.FijarAutoActivo(1, id2, false); err != nil {
		t.Fatal(err)
	}
	if m, _ := a.ObtenerAuto(1, id2); m.Activo {
		t.Fatal("apagar")
	}
	if a.FijarAutoActivo(1, 999, true) == nil {
		t.Fatal("no existe")
	}
	if l, _ := a.ListarAutos(1, AutoCierre); len(l) != 0 {
		t.Fatal("por tipo")
	}
	// otro bot no ve nada
	if l, _ := a.ListarAutos(2, ""); len(l) != 0 {
		t.Fatal("aislado por bot")
	}
}

func TestAtajoPorNombreIgnoraMayusculasYApagados(t *testing.T) {
	a := nuevo(t)
	id, _ := a.CrearAuto(1, nuevoAuto(AutoAtajo, "gracias", "Gracias"))
	if !a.ExisteAtajo(1, "GRACIAS") || a.AutoPorNombre(1, "Gracias") == nil || a.ExisteAtajo(1, "otro") {
		t.Fatal("búsqueda por nombre")
	}
	_ = a.FijarAutoActivo(1, id, false)
	if a.ExisteAtajo(1, "gracias") {
		t.Fatal("un atajo apagado no existe")
	}
	// los recibidos con el mismo nombre no cuentan como atajo
	_, _ = a.CrearAuto(1, nuevoAuto(AutoRecibido, "hola", "x"))
	if a.ExisteAtajo(1, "hola") {
		t.Fatal("solo los atajos")
	}
}

func TestRepeticionPorPersonaYRegistroSinTextos(t *testing.T) {
	a := nuevo(t)
	id, _ := a.CrearAuto(1, nuevoAuto(AutoRecibido, "R", "x"))
	if a.UltimaRepeticion(1, id, "p1") != 0 {
		t.Fatal("nunca")
	}
	t0 := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	a.MarcarRepeticion(1, id, "p1", t0)
	a.MarcarRepeticion(1, id, "p1", t0.Add(time.Hour))
	if a.UltimaRepeticion(1, id, "p1") != t0.Add(time.Hour).Unix() || a.UltimaRepeticion(1, id, "p2") != 0 {
		t.Fatal("por persona")
	}
	a.RegistrarAutoEnvio(1, AutoEnvio{TS: t0.Unix(), AutoID: id, Tipo: AutoRecibido, Chat: "5215511111111@s.whatsapp.net", Resultado: AutoOK})
	a.RegistrarAutoEnvio(1, AutoEnvio{TS: t0.Unix(), AutoID: id, Tipo: AutoAtajo, Chat: "c", Resultado: AutoOK})
	a.RegistrarAutoEnvio(1, AutoEnvio{TS: t0.Unix(), AutoID: id, Tipo: AutoRecibido, Chat: "c", Resultado: AutoTopeDuro})
	if n := a.ContarAutoEnvios(1, t0.Add(-time.Minute)); n != 1 {
		t.Fatalf("el tope cuenta solo envíos automáticos reales (no atajos ni rechazados): %d", n)
	}
	l, _ := a.ListarAutoRegistro(1, id, 10)
	if len(l) != 3 || strings.Contains(l[2].Chat, "5215511111111") || len(l[2].Chat) != 6 {
		t.Fatalf("el chat se guarda como identificador corto: %+v", l)
	}
	// olvidar a la persona borra lo que se recuerda de ella
	a.OlvidarAutoPersona(1, []string{"p1", "5215511111111@s.whatsapp.net"})
	if a.UltimaRepeticion(1, id, "p1") != 0 {
		t.Fatal("olvidar")
	}
	l, _ = a.ListarAutoRegistro(1, id, 10)
	if len(l) != 2 {
		t.Fatalf("se borra su renglón del registro: %d", len(l))
	}
	a.LimpiarAutoRegistro(1, 30, t0.AddDate(0, 0, 40))
	if l, _ := a.ListarAutoRegistro(1, 0, 10); len(l) != 0 {
		t.Fatal("limpieza por antigüedad")
	}
}

func guardaAdj(t *testing.T, a *Almacen, contenido string) AdjuntoRef {
	t.Helper()
	p := escribe(t, t.TempDir(), "a.bin", []byte(contenido))
	r, err := a.GuardarAdjuntoDeArchivo(1, p, AdjDocumento, "", "a.pdf")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestLimpiezaDeArchivosHuerfanos(t *testing.T) {
	a := nuevo(t)
	x, y, z := guardaAdj(t, a, "uno"), guardaAdj(t, a, "dos"), guardaAdj(t, a, "tres")
	id, err := a.CrearAuto(1, nuevoAuto(AutoRecibido, "R", "t", x, y))
	if err != nil {
		t.Fatal(err)
	}
	// quitar un adjunto del editor al guardar borra su archivo (y solo ése)
	m, _ := a.ObtenerAuto(1, id)
	m.Adjuntos = []AdjuntoRef{x}
	if err := a.GuardarAuto(1, m); err != nil {
		t.Fatal(err)
	}
	if !a.ExisteAdjunto(1, x.ID) || a.ExisteAdjunto(1, y.ID) {
		t.Fatal("se borró el que se quitó y se conservó el que sigue")
	}
	// un archivo suelto recién subido (z) no se toca durante la gracia, y luego sí
	if n := a.LimpiarAdjuntosHuerfanos(1, time.Hour, time.Now()); n != 0 {
		t.Fatalf("recién subido: %d", n)
	}
	if n := a.LimpiarAdjuntosHuerfanos(1, time.Hour, time.Now().Add(2*time.Hour)); n != 1 || a.ExisteAdjunto(1, z.ID) || !a.ExisteAdjunto(1, x.ID) {
		t.Fatalf("huérfano borrado, el usado se queda: %d", n)
	}
	// borrar el mensaje borra sus archivos
	if err := a.BorrarAuto(1, id); err != nil {
		t.Fatal(err)
	}
	if a.ExisteAdjunto(1, x.ID) {
		t.Fatal("al borrar el mensaje se borran sus archivos")
	}
	if l, _ := a.ListarAutos(1, ""); len(l) != 0 {
		t.Fatal("borrado")
	}
}

func TestUnArchivoUsadoPorOtroNoSeBorra(t *testing.T) {
	a := nuevo(t)
	x := guardaAdj(t, a, "compartido")
	id1, _ := a.CrearAuto(1, nuevoAuto(AutoRecibido, "R1", "t", x))
	_, _ = a.CrearAuto(1, nuevoAuto(AutoCierre, "R2", "t", x))
	_ = a.BorrarAuto(1, id1)
	if !a.ExisteAdjunto(1, x.ID) {
		t.Fatal("otro mensaje todavía lo usa")
	}
}

func TestLosArchivosDeMenusYComandosPropiosCuentanComoUsados(t *testing.T) {
	a := nuevo(t)
	x := guardaAdj(t, a, "de un menu")
	def := `{"v":1,"nombre":"M","inicio":"r","pasos":[{"id":"r","tipo":"respuesta","texto":"hola","adjuntos":[{"id":"` + x.ID + `","tipo":"documento"}]}],"disparadores":{},"alcance":{"privados":true},"horario":{}}`
	mid, err := a.CrearMenu(1, []byte(def), true, false)
	if err != nil {
		t.Fatal(err)
	}
	if n := a.LimpiarAdjuntosHuerfanos(1, 0, time.Now().Add(time.Hour)); n != 0 || !a.ExisteAdjunto(1, x.ID) {
		t.Fatal("lo que usa un menú no es huérfano")
	}
	// una versión nueva del menú sin el archivo: ya nada lo usa (ninguna conversación abierta)
	def2 := strings.Replace(def, `,"adjuntos":[{"id":"`+x.ID+`","tipo":"documento"}]`, "", 1)
	if _, err := a.GuardarMenu(1, mid, []byte(def2)); err != nil {
		t.Fatal(err)
	}
	if n := a.LimpiarAdjuntosHuerfanos(1, 0, time.Now().Add(time.Hour)); n != 1 || a.ExisteAdjunto(1, x.ID) {
		t.Fatalf("huérfano: %d", n)
	}
}

func TestEtiquetaAutomaticoEnMensajes(t *testing.T) {
	a := nuevo(t)
	chat := "5215511111111@s.whatsapp.net"
	_, _ = a.ActualizarChat(1, chat, false, func(c *Chat) { c.TelE164 = "+5215511111111" })
	t0 := time.Now().Unix()
	_, _ = a.GuardarMensaje(&Mensaje{ID: "A", ChatJID: chat, BotID: 1, RemitenteJID: QuienYo, EsMio: true, Tipo: "texto", Texto: "auto", TS: t0, Automatico: "bienvenida"})
	_, _ = a.GuardarMensaje(&Mensaje{ID: "B", ChatJID: chat, BotID: 1, RemitenteJID: QuienYo, EsMio: true, Tipo: "texto", Texto: "a mano", TS: t0 + 1})
	_, _ = a.GuardarMensaje(&Mensaje{ID: "C", ChatJID: chat, BotID: 1, RemitenteJID: chat, Tipo: "texto", Texto: "cliente", TS: t0 + 2})
	l, err := a.ListarMensajes(1, chat, 0, "", 10)
	if err != nil || len(l) != 3 {
		t.Fatalf("%v %d", err, len(l))
	}
	vistos := map[string]string{}
	for _, m := range l {
		vistos[m.ID] = m.Automatico
	}
	if vistos["A"] != "bienvenida" || vistos["B"] != "" || vistos["C"] != "" {
		t.Fatalf("%v", vistos)
	}
	au, _ := a.ListarMensajesAuto(1, chat, 0, 10)
	if len(au) != 1 || au[0].ID != "A" {
		t.Fatalf("filtro: %+v", au)
	}
	if n := a.ContarMensajesAuto(1, "", time.Unix(t0-10, 0)); n != 1 {
		t.Fatalf("%d", n)
	}
	if cs := a.ChatsConAutomaticos(1, time.Unix(t0-10, 0)); len(cs) != 1 || cs[0] != chat {
		t.Fatalf("%v", cs)
	}
	// MarcarAutomatico solo etiqueta mensajes míos
	a.MarcarAutomatico(1, chat, "B", "atajo")
	a.MarcarAutomatico(1, chat, "C", "atajo")
	if m, _ := a.ObtenerMensaje(1, chat, "B"); m.Automatico != "atajo" {
		t.Fatal("marcar")
	}
	if m, _ := a.ObtenerMensaje(1, chat, "C"); m.Automatico != "" {
		t.Fatal("un mensaje de un cliente nunca es automático")
	}
}

func TestEtiquetaEnElContexto(t *testing.T) {
	ctx := context.Background()
	if EtiquetaDe(ctx) != "" {
		t.Fatal("vacía")
	}
	if EtiquetaDe(ConEtiqueta(ctx, "cierre")) != "cierre" {
		t.Fatal("etiqueta")
	}
}

func TestEliminarBotYOlvidarPersonaIncluyenLoAutomatico(t *testing.T) {
	a := nuevo(t)
	x := guardaAdj(t, a, "adjunto")
	id, _ := a.CrearAuto(1, nuevoAuto(AutoRecibido, "R", "t", x))
	a.MarcarRepeticion(1, id, "p", time.Now())
	a.RegistrarAutoEnvio(1, AutoEnvio{AutoID: id, Tipo: AutoRecibido, Chat: "c", Resultado: AutoOK})
	if err := a.EliminarBot(1); err != nil {
		t.Fatal(err)
	}
	for _, tabla := range []string{"auto_mensajes", "auto_repeticion", "auto_registro"} {
		var n int
		_ = a.db.QueryRow(`SELECT COUNT(*) FROM ` + tabla).Scan(&n)
		if n != 0 {
			t.Fatalf("%s no se borró", tabla)
		}
	}
	if a.ExisteAdjunto(1, x.ID) {
		t.Fatal("los archivos guardados también se borran")
	}
}
