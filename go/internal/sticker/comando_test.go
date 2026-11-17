package sticker

import (
	"errors"
	"image/color"
	"strings"
	"testing"
	"time"

	"botchat/internal/almacen"
	"botchat/internal/comandos"
	"botchat/internal/permisos"
)

const (
	pnAna  = "5215511111111@s.whatsapp.net"
	propio = "5215500000001@s.whatsapp.net"
	grupo  = "grupo@g.us"
)

type entorno struct {
	alm  *almacen.Almacen
	perm *permisos.Servicio
	d    *comandos.Despachador
	s    *Servicio
	hora time.Time
	baja int
}

func nuevoEntorno(t *testing.T) *entorno {
	t.Helper()
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 11)
	}
	alm, err := almacen.Abrir(t.TempDir(), k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = alm.Cerrar() })
	_ = alm.FijarAjusteIni(1, "ini_sin_limites", "0") // estas pruebas miden los límites: se apaga «sin límites»
	p, err := permisos.Nuevo(alm, 1)
	if err != nil {
		t.Fatal(err)
	}
	e := &entorno{alm: alm, perm: p, hora: time.Now()}
	p.Ahora = func() time.Time { return e.hora }
	e.d = comandos.Nuevo(alm, 1, p)
	e.d.Ahora = p.Ahora
	e.s = Nuevo(alm, 1)
	e.s.Ahora = p.Ahora
	e.s.RegistrarComandos(e.d)
	return e
}

func (e *entorno) rol(nombre string, cas ...string) int64 {
	m := map[string]almacen.Alcance{}
	for _, c := range cas {
		m[c] = almacen.Alcance{}
	}
	id, err := e.perm.GuardarRol(almacen.Rol{Nombre: nombre, Casillas: m, LimiteMin: 100})
	if err != nil {
		panic(err)
	}
	return id
}

func pngBytes(t *testing.T) []byte {
	return codifica(t, imagenDePrueba(120, 80, func(x, y int) color.NRGBA { return color.NRGBA{uint8(x), uint8(y), 90, 255} }), "png")
}

// guarda una imagen recibida (descargada o no).
func (e *entorno) imagen(t *testing.T, chat, id, tipo string, datos []byte, estado string) {
	t.Helper()
	if _, err := e.alm.ActualizarChat(1, chat, strings.HasSuffix(chat, "@g.us"), func(c *almacen.Chat) {}); err != nil {
		t.Fatal(err)
	}
	m := &almacen.Mensaje{ID: id, ChatJID: chat, BotID: 1, RemitenteJID: chat, Tipo: tipo, TS: time.Now().Unix(), ArchivoEstado: estado, ArchivoMime: "image/jpeg", ArchivoInfo: []byte(`{"x":1}`)}
	if estado == almacen.EstadoDescargado {
		m.ArchivoRuta = almacen.RutaMedio(1, chat, id)
		m.ArchivoBytes = int64(len(datos))
		if err := e.alm.EscribirCifrado(m.ArchivoRuta, datos); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.alm.GuardarMensaje(m); err != nil {
		t.Fatal(err)
	}
}

func (e *entorno) ejecuta(quien, chat, texto, cita string, dueno bool, esGrupo bool) comandos.Resultado {
	persona := quien
	if dueno {
		persona = almacen.PersonaDueno
	}
	return e.d.Ejecutar(comandos.Entrada{Persona: persona, Chat: chat, EsGrupo: esGrupo, Texto: texto, MensajeID: "CMD", CitaID: cita, Dueno: dueno})
}

func webpDelAdjunto(t *testing.T, e *entorno, r comandos.Resultado) []byte {
	t.Helper()
	if r.Archivo == nil || r.Archivo.Tipo != almacen.TipoSticker || r.Archivo.Mime != "image/webp" || !almacen.IDAdjuntoValido(r.Archivo.ID) {
		t.Fatalf("debía venir un sticker: %+v texto=%q", r.Archivo, r.Texto)
	}
	b, err := e.alm.LeerAdjuntoMenu(1, r.Archivo.ID)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestStickerDeLaImagenCitadaDuenoYOtroRango(t *testing.T) {
	e := nuevoEntorno(t)
	e.imagen(t, pnAna, "IMG1", almacen.TipoImagen, pngBytes(t), almacen.EstadoDescargado)
	// el dueño, desde Mensajes guardados citando la imagen guardada
	e.imagen(t, propio, "IMG0", almacen.TipoImagen, pngBytes(t), almacen.EstadoDescargado)
	r := e.ejecuta("", propio, "!sticker", "IMG0", true, false)
	conPanico(t, func() {
		img := decodifica(t, webpDelAdjunto(t, e, r))
		if img.Rect.Dx() != Lado || img.NRGBAAt(256, 256).A != 255 {
			t.Fatal("sticker mal hecho")
		}
	})
	if r.Texto != "" {
		t.Fatalf("no debía llevar texto: %q", r.Texto)
	}
	// otro rango con la casilla, en su chat
	id := e.rol("Cliente", "util.sticker")
	if err := e.alm.AsignarRol(1, pnAna, id, 0, "yo"); err != nil {
		t.Fatal(err)
	}
	r = e.ejecuta(pnAna, pnAna, "!stiker", "IMG1", false, false)
	webpDelAdjunto(t, e, r)
}

func TestStickerDeLaImagenConElComandoDePieDeFoto(t *testing.T) {
	e := nuevoEntorno(t)
	id := e.rol("Cliente", "util.sticker")
	_ = e.alm.AsignarRol(1, pnAna, id, 0, "yo")
	// la propia imagen del comando (sin cita): su id es MensajeID
	e.imagen(t, pnAna, "CMD", almacen.TipoImagen, pngBytes(t), almacen.EstadoDescargado)
	webpDelAdjunto(t, e, e.ejecuta(pnAna, pnAna, "!sticker", "", false, false))
}

func TestStickerPermisosYGrupos(t *testing.T) {
	e := nuevoEntorno(t)
	e.imagen(t, grupo, "G1", almacen.TipoImagen, pngBytes(t), almacen.EstadoDescargado)
	// sin la casilla
	if r := e.ejecuta(pnAna, grupo, "!sticker", "G1", false, true); r.Archivo != nil || !strings.Contains(r.Texto, "No tienes permiso") {
		t.Fatalf("%+v", r)
	}
	// con la casilla funciona también en un grupo (la respuesta sale al grupo, citando)
	id := e.rol("Cliente", "util.sticker")
	_ = e.alm.AsignarRol(1, pnAna, id, 0, "yo")
	webpDelAdjunto(t, e, e.ejecuta(pnAna, grupo, "!sticker", "G1", false, true))
}

func TestStickerMensajesClaros(t *testing.T) {
	e := nuevoEntorno(t)
	e.imagen(t, propio, "TXT", almacen.TipoTexto, nil, "")
	e.imagen(t, propio, "STK", almacen.TipoSticker, []byte("x"), almacen.EstadoDescargado)
	e.imagen(t, propio, "VID", almacen.TipoVideo, []byte("x"), almacen.EstadoDescargado)
	e.imagen(t, propio, "VUV", "ver_una_vez", nil, "")
	e.imagen(t, propio, "ROTA", almacen.TipoImagen, []byte("no soy una imagen"), almacen.EstadoDescargado)
	quiere := func(cita, parte string) {
		t.Helper()
		r := e.ejecuta("", propio, "!sticker", cita, true, false)
		if r.Archivo != nil || !strings.Contains(r.Texto, parte) {
			t.Fatalf("cita %s: esperaba %q en %q (archivo %v)", cita, parte, r.Texto, r.Archivo)
		}
	}
	quiere("TXT", "no es una imagen")
	quiere("STK", "ya es un sticker")
	quiere("VID", "no es una imagen")
	quiere("VUV", "no es una imagen") // las de "ver una vez" nunca se convierten
	quiere("ROTA", "o pude leer esa imagen")
	quiere("NOEXISTE", "No encontré esa imagen")
	// una imagen eliminada por su autor no se convierte
	e.imagen(t, propio, "ELIM", almacen.TipoImagen, pngBytes(t), almacen.EstadoDescargado)
	if _, err := e.alm.MarcarEliminado(1, propio, "ELIM", "", time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	quiere("ELIM", "eliminada")
}

func TestStickerDescargaLaImagenSiHaceFalta(t *testing.T) {
	e := nuevoEntorno(t)
	e.imagen(t, propio, "PEND", almacen.TipoImagen, nil, almacen.EstadoPendiente)
	// sin forma de descargar
	if r := e.ejecuta("", propio, "!sticker", "PEND", true, false); !strings.Contains(r.Texto, "todavía no se descargó") {
		t.Fatalf("%q", r.Texto)
	}
	// con descarga: al pedirla, la imagen queda guardada
	llamadas := 0
	e.s.Asegurar = func(chat, id string) error {
		llamadas++
		rel := almacen.RutaMedio(1, chat, id)
		d := pngBytes(t)
		if err := e.alm.EscribirCifrado(rel, d); err != nil {
			return err
		}
		return e.alm.ActualizarArchivo(1, chat, id, almacen.EstadoDescargado, rel, int64(len(d)))
	}
	webpDelAdjunto(t, e, e.ejecuta("", propio, "!sticker", "PEND", true, false))
	if llamadas != 1 {
		t.Fatalf("llamadas: %d", llamadas)
	}
	// y si la descarga falla, se dice sin detalles técnicos
	e.imagen(t, propio, "PEND2", almacen.TipoImagen, nil, almacen.EstadoPendiente)
	e.s.Asegurar = func(string, string) error { return errors.New("boom interno") }
	r := e.ejecuta("", propio, "!sticker", "PEND2", true, false)
	if !strings.Contains(r.Texto, "o pude descargar") || strings.Contains(r.Texto, "boom") {
		t.Fatalf("%q", r.Texto)
	}
}

func TestStickerTopePorHora(t *testing.T) {
	e := nuevoEntorno(t)
	e.imagen(t, propio, "I", almacen.TipoImagen, pngBytes(t), almacen.EstadoDescargado)
	_ = e.alm.FijarAjusteBot(1, "sticker_tope_hora", "2")
	for i := 0; i < 2; i++ {
		webpDelAdjunto(t, e, e.ejecuta("", propio, "!sticker", "I", true, false))
	}
	if r := e.ejecuta("", propio, "!sticker", "I", true, false); r.Archivo != nil || !strings.Contains(r.Texto, "muchos stickers") {
		t.Fatalf("%+v", r)
	}
	e.hora = e.hora.Add(61 * time.Minute)
	webpDelAdjunto(t, e, e.ejecuta("", propio, "!sticker", "I", true, false))
	// 0 = sin tope
	_ = e.alm.FijarAjusteBot(1, "sticker_tope_hora", "0")
	for i := 0; i < 5; i++ {
		webpDelAdjunto(t, e, e.ejecuta("", propio, "!sticker", "I", true, false))
	}
}

func TestStickerSinCitaNiImagenPideQueCiten(t *testing.T) {
	e := nuevoEntorno(t)
	r := e.d.Ejecutar(comandos.Entrada{Persona: almacen.PersonaDueno, Chat: propio, Texto: "!sticker", Dueno: true})
	if !strings.Contains(r.Texto, "esponde a una foto") {
		t.Fatalf("%q", r.Texto)
	}
}
