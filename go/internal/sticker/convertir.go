// Package sticker convierte una imagen (JPG, PNG o GIF) en un sticker de WhatsApp: WebP sin pérdida de 512×512 con fondo transparente,
// la imagen centrada y entera, y a lo más ~100 KB. Todo se hace en el teléfono, sin servicios externos.
package sticker

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif" // formatos que sabe leer
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"sort"
)

// Lado del sticker y límites.
const (
	Lado         = 512
	PesoObjetivo = 100 << 10 // lo que WhatsApp espera de un sticker estático
	maxPixeles   = 40_000_000
	maxEntrada   = 25 << 20
)

// Errores con texto listo para decírselo a la persona.
var (
	ErrFormato = errors.New("no pude leer esa imagen: usa una foto JPG o PNG")
	ErrGrande  = errors.New("esa imagen es demasiado grande para convertirla")
)

// Convertir lee una imagen y devuelve el WebP del sticker.
func Convertir(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, maxEntrada+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxEntrada {
		return nil, ErrGrande
	}
	if len(b) >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP" {
		return nil, errors.New("esa imagen ya está en formato WebP (probablemente ya es un sticker)")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return nil, ErrFormato
	}
	if cfg.Width < 1 || cfg.Height < 1 {
		return nil, ErrFormato
	}
	if cfg.Width*cfg.Height > maxPixeles {
		return nil, ErrGrande
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, ErrFormato
	}
	return DeImagen(img)
}

// DeImagen hace el sticker de una imagen ya leída.
func DeImagen(img image.Image) ([]byte, error) {
	lienzo := encaja(img)
	var mejor []byte
	for _, n := range []int{255, 128, 64, 32, 16} {
		indices, paleta, alfa := cuantiza(lienzo, n)
		datos, err := codificarPaleta(Lado, Lado, indices, paleta, alfa)
		if err != nil {
			return nil, err
		}
		mejor = datos
		if len(datos) <= PesoObjetivo {
			break
		}
	}
	return mejor, nil
}

// encaja pone la imagen entera y centrada en un lienzo transparente de 512×512.
func encaja(img image.Image) *image.NRGBA {
	b := img.Bounds()
	src := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(src, src.Bounds(), img, b.Min, draw.Src)
	esc := math.Min(float64(Lado)/float64(src.Rect.Dx()), float64(Lado)/float64(src.Rect.Dy()))
	w := int(math.Round(float64(src.Rect.Dx()) * esc))
	h := int(math.Round(float64(src.Rect.Dy()) * esc))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if w > Lado {
		w = Lado
	}
	if h > Lado {
		h = Lado
	}
	dst := image.NewNRGBA(image.Rect(0, 0, Lado, Lado))
	ox, oy := (Lado-w)/2, (Lado-h)/2
	escala(src, dst, ox, oy, w, h)
	return dst
}

// escala redimensiona src a w×h dentro de dst (con premultiplicado para no ensuciar los bordes transparentes): promedio de área al
// reducir, interpolación bilineal al agrandar.
func escala(src, dst *image.NRGBA, ox, oy, w, h int) {
	sw, sh := src.Rect.Dx(), src.Rect.Dy()
	px := func(x, y int) (r, g, b, a float64) {
		o := src.PixOffset(src.Rect.Min.X+x, src.Rect.Min.Y+y)
		a = float64(src.Pix[o+3])
		return float64(src.Pix[o]) * a, float64(src.Pix[o+1]) * a, float64(src.Pix[o+2]) * a, a
	}
	pon := func(x, y int, r, g, b, a float64) {
		if a <= 0 {
			return
		}
		o := dst.PixOffset(ox+x, oy+y)
		dst.Pix[o] = clamp8(r / a)
		dst.Pix[o+1] = clamp8(g / a)
		dst.Pix[o+2] = clamp8(b / a)
		dst.Pix[o+3] = clamp8(a)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if sw >= w && sh >= h { // reducir (o igual): promedio del área que cubre
				x0, x1 := x*sw/w, (x+1)*sw/w
				y0, y1 := y*sh/h, (y+1)*sh/h
				if x1 <= x0 {
					x1 = x0 + 1
				}
				if y1 <= y0 {
					y1 = y0 + 1
				}
				var r, g, b, a float64
				for yy := y0; yy < y1; yy++ {
					for xx := x0; xx < x1; xx++ {
						pr, pg, pb, pa := px(xx, yy)
						r, g, b, a = r+pr, g+pg, b+pb, a+pa
					}
				}
				n := float64((x1 - x0) * (y1 - y0))
				pon(x, y, r/n, g/n, b/n, a/n)
				continue
			}
			// agrandar: bilineal
			fx := (float64(x)+0.5)*float64(sw)/float64(w) - 0.5
			fy := (float64(y)+0.5)*float64(sh)/float64(h) - 0.5
			x0, y0 := int(math.Floor(fx)), int(math.Floor(fy))
			tx, ty := fx-float64(x0), fy-float64(y0)
			var r, g, b, a float64
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					xx, yy := min(max(x0+dx, 0), sw-1), min(max(y0+dy, 0), sh-1)
					pr, pg, pb, pa := px(xx, yy)
					wx, wy := 1-tx, 1-ty
					if dx == 1 {
						wx = tx
					}
					if dy == 1 {
						wy = ty
					}
					p := wx * wy
					r, g, b, a = r+pr*p, g+pg*p, b+pb*p, a+pa*p
				}
			}
			pon(x, y, r, g, b, a)
		}
	}
}

func clamp8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v + 0.5)
}

// ---------- paleta ----------

type cubo struct {
	r, g, b, n int // suma de color (por canal) y cuántos píxeles
}

// cuantiza reduce la imagen a una paleta de hasta n colores (más uno transparente si hace falta). La transparencia es de sí o no:
// los píxeles con menos de la mitad de opacidad quedan transparentes.
func cuantiza(img *image.NRGBA, n int) (indices []uint8, paleta []uint32, hayAlfa bool) {
	const bits = 5
	hist := map[int]*cubo{}
	total := len(img.Pix) / 4
	trans := make([]bool, total)
	clave := make([]int32, total)
	for i := 0; i < total; i++ {
		p := img.Pix[i*4 : i*4+4]
		if p[3] < 128 {
			trans[i] = true
			hayAlfa = true
			continue
		}
		k := int(p[0]>>(8-bits))<<(2*bits) | int(p[1]>>(8-bits))<<bits | int(p[2]>>(8-bits))
		clave[i] = int32(k)
		c := hist[k]
		if c == nil {
			c = &cubo{}
			hist[k] = c
		}
		c.r += int(p[0])
		c.g += int(p[1])
		c.b += int(p[2])
		c.n++
	}
	// cortes por la mediana
	type caja struct{ ks []int }
	cs := make([]int, 0, len(hist))
	for k := range hist {
		cs = append(cs, k)
	}
	sort.Ints(cs)
	cajas := []caja{{ks: cs}}
	canal := func(k int) [3]int {
		c := hist[k]
		return [3]int{c.r / c.n, c.g / c.n, c.b / c.n}
	}
	rango := func(ks []int) (int, int) { // canal más ancho y su ancho
		var mn, mx [3]int
		for i := range mn {
			mn[i], mx[i] = 255, 0
		}
		for _, k := range ks {
			v := canal(k)
			for i := 0; i < 3; i++ {
				mn[i], mx[i] = min(mn[i], v[i]), max(mx[i], v[i])
			}
		}
		mejor, w := 0, -1
		for i := 0; i < 3; i++ {
			if mx[i]-mn[i] > w {
				mejor, w = i, mx[i]-mn[i]
			}
		}
		return mejor, w
	}
	cupo := n
	if cupo > 255 {
		cupo = 255
	}
	for len(cajas) < cupo {
		// la caja con más píxeles y más de un color
		bi, bp := -1, 0
		for i, c := range cajas {
			if len(c.ks) < 2 {
				continue
			}
			p := 0
			for _, k := range c.ks {
				p += hist[k].n
			}
			if _, w := rango(c.ks); w > 0 && p > bp {
				bi, bp = i, p
			}
		}
		if bi < 0 {
			break
		}
		c := cajas[bi]
		ch, _ := rango(c.ks)
		ks := append([]int(nil), c.ks...)
		sort.Slice(ks, func(i, j int) bool { return canal(ks[i])[ch] < canal(ks[j])[ch] })
		mitad, acum := 0, 0
		for i, k := range ks {
			acum += hist[k].n
			if acum*2 >= bp {
				mitad = i + 1
				break
			}
		}
		if mitad <= 0 || mitad >= len(ks) {
			mitad = len(ks) / 2
		}
		cajas[bi] = caja{ks: ks[:mitad]}
		cajas = append(cajas, caja{ks: ks[mitad:]})
	}
	var pal [][3]int
	if hayAlfa {
		paleta = append(paleta, 0)
		pal = append(pal, [3]int{-1, -1, -1})
	}
	cajaDe := map[int]int{}
	for _, c := range cajas {
		var r, g, b, nn int
		for _, k := range c.ks {
			r, g, b, nn = r+hist[k].r, g+hist[k].g, b+hist[k].b, nn+hist[k].n
		}
		if nn == 0 {
			continue
		}
		col := [3]int{r / nn, g / nn, b / nn}
		for _, k := range c.ks {
			cajaDe[k] = len(pal)
		}
		pal = append(pal, col)
		paleta = append(paleta, 0xff000000|uint32(col[0])<<16|uint32(col[1])<<8|uint32(col[2]))
	}
	if len(paleta) == 0 { // imagen vacía
		paleta = []uint32{0}
		hayAlfa = true
		pal = [][3]int{{-1, -1, -1}}
	}
	indices = make([]uint8, total)
	for i := 0; i < total; i++ {
		if trans[i] {
			indices[i] = 0
			continue
		}
		indices[i] = uint8(cajaDe[int(clave[i])])
	}
	return indices, paleta, hayAlfa
}

// Describe da una línea corta del resultado (para el registro, sin nada de la imagen).
func Describe(webp []byte) string { return fmt.Sprintf("%d KB", (len(webp)+1023)/1024) }
