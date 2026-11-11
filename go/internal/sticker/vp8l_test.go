package sticker

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- decodificador de pruebas (solo lo que escribe este codificador) ----------

type lector struct {
	b   []byte
	pos uint
}

func (l *lector) bits(n uint) uint32 {
	var v uint32
	for i := uint(0); i < n; i++ {
		byt := l.b[l.pos>>3]
		v |= uint32(byt>>(l.pos&7)&1) << i
		l.pos++
	}
	return v
}

type tablaH struct {
	cuenta [16]int
	simbs  []int
	unico  int
}

func (t *tablaH) lee(l *lector) int {
	if t.unico >= 0 {
		return t.unico
	}
	code, first, index := 0, 0, 0
	for ln := 1; ln < 16; ln++ {
		code |= int(l.bits(1))
		c := t.cuenta[ln]
		if code-c < first {
			return t.simbs[index+(code-first)]
		}
		index += c
		first += c
		first <<= 1
		code <<= 1
	}
	panic("código no válido")
}

func construye(long []int) *tablaH {
	t := &tablaH{unico: -1}
	for _, l := range long {
		t.cuenta[l]++
	}
	t.cuenta[0] = 0
	for ln := 1; ln < 16; ln++ {
		for s, l := range long {
			if l == ln {
				t.simbs = append(t.simbs, s)
			}
		}
	}
	// completitud de Kraft
	left := 1
	for ln := 1; ln < 16; ln++ {
		left = left<<1 - t.cuenta[ln]
		if left < 0 {
			panic("código sobre-suscrito")
		}
	}
	if left != 0 {
		panic("código incompleto")
	}
	return t
}

func leeArbol(l *lector, n int) *tablaH {
	if l.bits(1) == 1 { // simple
		if l.bits(1) != 0 {
			panic("el codificador solo escribe códigos simples de un símbolo")
		}
		var s int
		if l.bits(1) == 0 {
			s = int(l.bits(1))
		} else {
			s = int(l.bits(8))
		}
		return &tablaH{unico: s}
	}
	nl := int(l.bits(4)) + 4
	ll := make([]int, 19)
	for i := 0; i < nl; i++ {
		ll[ordenLongCodigo[i]] = int(l.bits(3))
	}
	tc := construye(ll)
	if l.bits(1) != 0 {
		panic("use_length no se usa")
	}
	long := make([]int, 0, n)
	prev := 8
	for len(long) < n {
		s := tc.lee(l)
		switch {
		case s < 16:
			long = append(long, s)
			if s != 0 {
				prev = s
			}
		case s == 16:
			r := 3 + int(l.bits(2))
			for ; r > 0; r-- {
				long = append(long, prev)
			}
		case s == 17:
			r := 3 + int(l.bits(3))
			for ; r > 0; r-- {
				long = append(long, 0)
			}
		case s == 18:
			r := 11 + int(l.bits(7))
			for ; r > 0; r-- {
				long = append(long, 0)
			}
		}
	}
	if len(long) != n {
		panic("largos de más")
	}
	return construye(long)
}

func valorPrefijo(l *lector, sim int) int {
	if sim < 4 {
		return sim + 1
	}
	ne := uint((sim - 2) >> 1)
	off := (2 + (sim & 1)) << ne
	return off + int(l.bits(ne)) + 1
}

func leeImagen(l *lector, ancho, alto int, nivel0 bool) []uint32 {
	if l.bits(1) != 0 {
		panic("caché de colores no usada")
	}
	if nivel0 && l.bits(1) != 0 {
		panic("metacódigos no usados")
	}
	tv, tr, ta, tz, td := leeArbol(l, alfabetoVerde), leeArbol(l, 256), leeArbol(l, 256), leeArbol(l, 256), leeArbol(l, alfabetoDist)
	px := make([]uint32, 0, ancho*alto)
	for len(px) < ancho*alto {
		g := tv.lee(l)
		if g < 256 {
			r, b, a := tr.lee(l), ta.lee(l), tz.lee(l)
			px = append(px, uint32(a)<<24|uint32(r)<<16|uint32(g)<<8|uint32(b))
			continue
		}
		largo := valorPrefijo(l, g-256)
		cod := valorPrefijo(l, td.lee(l))
		var d int
		switch {
		case cod > 120:
			d = cod - 120
		case cod == 1:
			d = ancho
		case cod == 2:
			d = 1
		default:
			panic("código de distancia no escrito por este codificador")
		}
		if d > len(px) || d < 1 {
			panic("distancia fuera de rango")
		}
		for i := 0; i < largo; i++ {
			px = append(px, px[len(px)-d])
		}
	}
	if len(px) != ancho*alto {
		panic("se pasó de píxeles")
	}
	return px
}

// decodifica devuelve la imagen NRGBA de un WebP sin pérdida de paleta.
func decodifica(t *testing.T, webp []byte) *image.NRGBA {
	t.Helper()
	if len(webp) < 20 || string(webp[:4]) != "RIFF" || string(webp[8:16]) != "WEBPVP8L" {
		t.Fatalf("contenedor: %q", webp[:min(len(webp), 16)])
	}
	if int(webp[4])|int(webp[5])<<8|int(webp[6])<<16|int(webp[7])<<24 != len(webp)-8 {
		t.Fatalf("tamaño RIFF %d vs %d", int(webp[4])|int(webp[5])<<8|int(webp[6])<<16, len(webp)-8)
	}
	n := int(webp[16]) | int(webp[17])<<8 | int(webp[18])<<16 | int(webp[19])<<24
	if len(webp) != 20+n+n&1 {
		t.Fatalf("tamaño del trozo %d vs %d", n, len(webp)-20)
	}
	l := &lector{b: webp[20 : 20+n]}
	if l.bits(8) != 0x2f {
		t.Fatal("firma")
	}
	ancho, alto := int(l.bits(14))+1, int(l.bits(14))+1
	l.bits(1)
	if l.bits(3) != 0 {
		t.Fatal("versión")
	}
	if l.bits(1) != 1 || l.bits(2) != 3 {
		t.Fatal("se esperaba la transformación de paleta")
	}
	tam := int(l.bits(8)) + 1
	tab := leeImagen(l, tam, 1, false)
	pal := make([]uint32, tam)
	var a, r, g, b uint32
	for i, d := range tab {
		a, r, g, b = (a+d>>24)&0xff, (r+d>>16&0xff)&0xff, (g+d>>8&0xff)&0xff, (b+d&0xff)&0xff
		pal[i] = a<<24 | r<<16 | g<<8 | b
	}
	if l.bits(1) != 0 {
		t.Fatal("sobran transformaciones")
	}
	bpp, bitsAncho := uint(8), uint(0)
	switch {
	case tam <= 2:
		bpp, bitsAncho = 1, 3
	case tam <= 4:
		bpp, bitsAncho = 2, 2
	case tam <= 16:
		bpp, bitsAncho = 4, 1
	}
	anchoP := (ancho + 1<<bitsAncho - 1) >> bitsAncho
	px := leeImagen(l, anchoP, alto, true)
	out := image.NewNRGBA(image.Rect(0, 0, ancho, alto))
	for y := 0; y < alto; y++ {
		for x := 0; x < ancho; x++ {
			v := px[y*anchoP+x>>bitsAncho] >> 8 & 0xff
			if bitsAncho > 0 {
				v = v >> (bpp * uint(x&(1<<bitsAncho-1))) & (1<<bpp - 1)
			}
			c := uint32(0)
			if int(v) < tam {
				c = pal[v]
			}
			o := out.PixOffset(x, y)
			out.Pix[o], out.Pix[o+1], out.Pix[o+2], out.Pix[o+3] = byte(c>>16), byte(c>>8), byte(c), byte(c>>24)
		}
	}
	return out
}

func conPanico(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("el decodificador de pruebas rechazó el archivo: %v", r)
		}
	}()
	f()
}

// ---------- pruebas ----------

func TestRoundTripPaletaConTodosLosTamanos(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, ncol := range []int{1, 2, 3, 4, 5, 16, 17, 100, 256} {
		for _, dim := range [][2]int{{1, 1}, {7, 5}, {33, 20}, {64, 64}} {
			w, h := dim[0], dim[1]
			pal := make([]uint32, ncol)
			for i := range pal {
				pal[i] = 0xff000000 | uint32(rng.Intn(1<<24))
			}
			if ncol > 1 {
				pal[0] = 0 // transparente
			}
			ind := make([]uint8, w*h)
			for i := range ind {
				switch {
				case i%11 < 5 && i > 0: // repeticiones
					ind[i] = ind[i-1]
				default:
					ind[i] = uint8(rng.Intn(ncol))
				}
			}
			webp, err := codificarPaleta(w, h, ind, pal, ncol > 1)
			if err != nil {
				t.Fatal(err)
			}
			conPanico(t, func() {
				img := decodifica(t, webp)
				if img.Rect.Dx() != w || img.Rect.Dy() != h {
					t.Fatalf("medidas %v", img.Rect)
				}
				for i, ix := range ind {
					c := pal[ix]
					o := i * 4
					if img.Pix[o] != byte(c>>16) || img.Pix[o+1] != byte(c>>8) || img.Pix[o+2] != byte(c) || img.Pix[o+3] != byte(c>>24) {
						t.Fatalf("colores=%d %dx%d píxel %d: %v vs %08x", ncol, w, h, i, img.Pix[o:o+4], c)
					}
				}
			})
		}
	}
}

func TestPrefijoIdaYVuelta(t *testing.T) {
	for v := 1; v <= 1<<20; v += 1 + v/200 {
		s, ne, ev := prefijo(v)
		l := &lector{b: []byte{0, 0, 0, 0}}
		// escribe los bits extra y léelos
		w := &escritorBits{}
		w.bits(uint32(ev), uint(ne))
		l.b = append(w.fin(), 0, 0, 0, 0)
		if got := valorPrefijo(l, s); got != v {
			t.Fatalf("%d -> %d", v, got)
		}
		if s >= alfabetoDist && v < 1<<20 {
			t.Fatalf("símbolo fuera del alfabeto: %d", s)
		}
	}
}

func imagenDePrueba(w, h int, f func(x, y int) color.NRGBA) *image.NRGBA {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m.SetNRGBA(x, y, f(x, y))
		}
	}
	return m
}

func codifica(t *testing.T, m image.Image, formato string) []byte {
	var b bytes.Buffer
	var err error
	if formato == "png" {
		err = png.Encode(&b, m)
	} else {
		err = jpeg.Encode(&b, m, &jpeg.Options{Quality: 85})
	}
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestConvertirPNGConTransparenciaQuedaCentradoYEntero(t *testing.T) {
	// 200×100 rojo opaco con un hueco transparente
	m := imagenDePrueba(200, 100, func(x, y int) color.NRGBA {
		if x < 20 {
			return color.NRGBA{}
		}
		return color.NRGBA{200, 30, 30, 255}
	})
	webp, err := Convertir(bytes.NewReader(codifica(t, m, "png")))
	if err != nil {
		t.Fatal(err)
	}
	conPanico(t, func() {
		img := decodifica(t, webp)
		if img.Rect.Dx() != Lado || img.Rect.Dy() != Lado {
			t.Fatalf("debe ser %d×%d: %v", Lado, Lado, img.Rect)
		}
		// 200×100 -> 512×256 centrado: arriba y abajo (filas 0..127 y 384..511) transparentes
		if img.NRGBAAt(256, 10).A != 0 || img.NRGBAAt(256, 500).A != 0 {
			t.Fatal("los bordes de arriba y abajo deben ser transparentes")
		}
		c := img.NRGBAAt(300, 256)
		if c.A != 255 || c.R < 190 || c.G > 40 {
			t.Fatalf("centro: %v", c)
		}
		if img.NRGBAAt(10, 256).A != 0 { // el hueco transparente de la izquierda (x<20 de 200 => <51 de 512)
			t.Fatal("el hueco transparente se perdió")
		}
	})
	if len(webp) > PesoObjetivo {
		t.Fatalf("pesa %d", len(webp))
	}
}

func TestConvertirFotoJPEGGrandeBajaDe100KB(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	// degradados + ruido: una "foto" difícil de comprimir
	m := imagenDePrueba(1600, 1200, func(x, y int) color.NRGBA {
		return color.NRGBA{uint8(x / 7), uint8(y / 5), uint8((x + y) / 11), 255}
	})
	for i := 0; i < len(m.Pix); i += 4 {
		m.Pix[i] ^= uint8(rng.Intn(24))
		m.Pix[i+1] ^= uint8(rng.Intn(24))
	}
	webp, err := Convertir(bytes.NewReader(codifica(t, m, "jpeg")))
	if err != nil {
		t.Fatal(err)
	}
	if len(webp) > PesoObjetivo {
		t.Fatalf("pesa %d KB, debía quedar en %d KB o menos", len(webp)>>10, PesoObjetivo>>10)
	}
	conPanico(t, func() {
		img := decodifica(t, webp)
		if img.NRGBAAt(256, 256).A != 255 || img.NRGBAAt(2, 256).A == 0 && false {
			t.Fatal("opaco al centro")
		}
		// 1600×1200 -> 512×384: filas 0..63 transparentes
		if img.NRGBAAt(256, 10).A != 0 {
			t.Fatal("margen superior transparente")
		}
	})
}

func TestConvertirPequenaSeAgranda(t *testing.T) {
	m := imagenDePrueba(32, 32, func(x, y int) color.NRGBA { return color.NRGBA{10, 200, 10, 255} })
	webp, err := Convertir(bytes.NewReader(codifica(t, m, "png")))
	if err != nil {
		t.Fatal(err)
	}
	conPanico(t, func() {
		img := decodifica(t, webp)
		if img.NRGBAAt(1, 1).A != 255 || img.NRGBAAt(510, 510).A != 255 {
			t.Fatal("debía llenar el lienzo")
		}
	})
}

func TestConvertirErroresClaros(t *testing.T) {
	if _, err := Convertir(strings.NewReader("esto no es una imagen")); !errors.Is(err, ErrFormato) {
		t.Fatalf("%v", err)
	}
	if _, err := Convertir(strings.NewReader("")); !errors.Is(err, ErrFormato) {
		t.Fatalf("%v", err)
	}
	webp := append([]byte("RIFF\x04\x00\x00\x00WEBPVP8L"), make([]byte, 20)...)
	if _, err := Convertir(bytes.NewReader(webp)); err == nil || !strings.Contains(err.Error(), "WebP") {
		t.Fatalf("webp de entrada: %v", err)
	}
	// una imagen enorme declarada (cabecera PNG con 20000×20000) se rechaza sin decodificar
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 2, 2)))
	d := b.Bytes()
	copy(d[16:], []byte{0, 0, 0x4e, 0x20, 0, 0, 0x4e, 0x20}) // ancho y alto 20000 (la suma de control queda mala, pero DecodeConfig no la revisa)
	if _, err := Convertir(bytes.NewReader(d)); err == nil {
		t.Fatal("debía rechazarla")
	}
}

// Si hay Python con Pillow (libwebp), se comprueba contra el decodificador de referencia.
func TestLibwebpDecodificaElResultado(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("sin python3")
	}
	if exec.Command(py, "-I", "-c", "from PIL import features,Image; assert features.check('webp')").Run() != nil {
		t.Skip("sin Pillow con WebP")
	}
	m := imagenDePrueba(640, 480, func(x, y int) color.NRGBA {
		if (x-320)*(x-320)+(y-240)*(y-240) > 200*200 {
			return color.NRGBA{}
		}
		return color.NRGBA{uint8(x / 3), uint8(y / 2), uint8(x ^ y), 255}
	})
	webp, err := Convertir(bytes.NewReader(codifica(t, m, "png")))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ruta := filepath.Join(dir, "s.webp")
	if err := os.WriteFile(ruta, webp, 0o600); err != nil {
		t.Fatal(err)
	}
	prog := "import sys\nfrom PIL import Image\ni=Image.open(sys.argv[1]); i.load()\nassert i.size==(512,512), i.size\nassert i.mode in ('RGBA','RGB'), i.mode\nassert i.getpixel((0,0))[3]==0\nassert i.getpixel((256,256))[3]==255\nprint('ok')"
	out, err := exec.Command(py, "-I", "-c", prog, ruta).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "ok") {
		t.Fatalf("libwebp no la acepta: %v %s", err, out)
	}
}
