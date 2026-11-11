package sticker

// Codificador WebP sin pérdida (VP8L) mínimo, escrito aquí porque la librería estándar de Go no trae uno. Solo hace lo necesario para
// un sticker: imagen de paleta (transformación de índices de color, con empaquetado de píxeles), referencias hacia atrás (LZ77) y códigos
// de Huffman. Se comprueba contra un decodificador propio en las pruebas y contra libwebp (Pillow) a mano.

import (
	"encoding/binary"
	"errors"
)

const (
	alfabetoVerde = 256 + 24
	alfabetoDist  = 40
	maxLongitud   = 4096
)

// ordenLongCodigo es el orden en que se mandan los largos del código de largos.
var ordenLongCodigo = [19]int{17, 18, 0, 1, 2, 3, 4, 5, 16, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

// ficha es un símbolo del flujo de datos: un píxel suelto o una copia (largo, código de distancia).
type ficha struct {
	copia bool
	px    uint32
	largo int
	dcod  int
}

// prefijo parte un valor ≥ 1 en (símbolo, bits extra, valor extra), como pide VP8L para largos y distancias.
func prefijo(v int) (sim, nExtra int, extra int) {
	if v <= 4 {
		return v - 1, 0, 0
	}
	v--
	hb := 0
	for t := v; t > 1; t >>= 1 {
		hb++
	}
	sh := (v >> (hb - 1)) & 1
	nExtra = hb - 1
	return 2*hb + sh, nExtra, v & (1<<nExtra - 1)
}

// arbol es un código de Huffman listo para escribir.
type arbol struct {
	long   []int
	cod    []uint32
	unico  int // si solo hay un símbolo, cuál (los demás casos: -1)
	simple bool
}

func armaArbol(freq []int, max int) arbol {
	usados, ult := 0, 0
	for s, f := range freq {
		if f > 0 {
			usados++
			ult = s
		}
	}
	if usados <= 1 {
		return arbol{unico: ult, simple: true}
	}
	l := longitudesHuffman(freq, max)
	return arbol{long: l, cod: codigosCanonicos(l), unico: -1}
}

func (a arbol) escribe(w *escritorBits, s int) {
	if a.simple {
		return // un solo símbolo posible: no gasta bits
	}
	w.bits(a.cod[s], uint(a.long[s]))
}

// escribeCabecera manda el código al flujo (simple o normal).
func (a arbol) escribeCabecera(w *escritorBits) {
	if a.simple {
		w.bits(1, 1) // código simple
		w.bits(0, 1) // un símbolo
		if a.unico < 2 {
			w.bits(0, 1)
			w.bits(uint32(a.unico), 1)
		} else {
			w.bits(1, 1)
			w.bits(uint32(a.unico), 8)
		}
		return
	}
	w.bits(0, 1)
	type tok struct{ sim, nex, val int }
	var toks []tok
	l := a.long
	for i := 0; i < len(l); {
		if l[i] != 0 {
			toks = append(toks, tok{l[i], 0, 0})
			i++
			continue
		}
		j := i
		for j < len(l) && l[j] == 0 {
			j++
		}
		run := j - i
		i = j
		for run > 0 {
			switch {
			case run >= 11:
				n := run
				if n > 138 {
					n = 138
				}
				toks = append(toks, tok{18, 7, n - 11})
				run -= n
			case run >= 3:
				toks = append(toks, tok{17, 3, run - 3})
				run = 0
			default:
				toks = append(toks, tok{0, 0, 0})
				run--
			}
		}
	}
	fr := make([]int, 19)
	for _, t := range toks {
		fr[t.sim]++
	}
	if distintos(fr) < 2 {
		if fr[0] == 0 {
			fr[0] = 1
		} else {
			fr[1] = 1
		}
	}
	ll := longitudesHuffman(fr, 7)
	cc := codigosCanonicos(ll)
	n := 19
	for n > 4 && ll[ordenLongCodigo[n-1]] == 0 {
		n--
	}
	w.bits(uint32(n-4), 4)
	for i := 0; i < n; i++ {
		w.bits(uint32(ll[ordenLongCodigo[i]]), 3)
	}
	w.bits(0, 1) // se mandan los largos de todos los símbolos
	for _, t := range toks {
		w.bits(cc[t.sim], uint(ll[t.sim]))
		w.bits(uint32(t.val), uint(t.nex))
	}
}

func distintos(f []int) int {
	n := 0
	for _, x := range f {
		if x > 0 {
			n++
		}
	}
	return n
}

// escribeImagen manda una imagen de entropía: 5 códigos y los símbolos. nivel0 añade el bit "sin imagen de metacódigos".
func escribeImagen(w *escritorBits, fichas []ficha, nivel0 bool) {
	fv := make([]int, alfabetoVerde)
	fr, fa, fz := make([]int, 256), make([]int, 256), make([]int, 256)
	fd := make([]int, alfabetoDist)
	for _, f := range fichas {
		if !f.copia {
			fv[(f.px>>8)&0xff]++
			fr[(f.px>>16)&0xff]++
			fa[f.px&0xff]++
			fz[f.px>>24]++
			continue
		}
		s, _, _ := prefijo(f.largo)
		fv[256+s]++
		d, _, _ := prefijo(f.dcod)
		fd[d]++
	}
	w.bits(0, 1) // sin caché de colores
	if nivel0 {
		w.bits(0, 1) // sin metacódigos
	}
	tv, tr, ta, tz, td := armaArbol(fv, 15), armaArbol(fr, 15), armaArbol(fa, 15), armaArbol(fz, 15), armaArbol(fd, 15)
	for _, t := range []arbol{tv, tr, ta, tz, td} {
		t.escribeCabecera(w)
	}
	for _, f := range fichas {
		if !f.copia {
			tv.escribe(w, int((f.px>>8)&0xff))
			tr.escribe(w, int((f.px>>16)&0xff))
			ta.escribe(w, int(f.px&0xff))
			tz.escribe(w, int(f.px>>24))
			continue
		}
		s, ne, ev := prefijo(f.largo)
		tv.escribe(w, 256+s)
		w.bits(uint32(ev), uint(ne))
		d, nd, evd := prefijo(f.dcod)
		td.escribe(w, d)
		w.bits(uint32(evd), uint(nd))
	}
}

// fichasDe recorre los píxeles buscando repeticiones (cadenas de hash con profundidad corta).
func fichasDe(px []uint32, ancho int) []ficha {
	n := len(px)
	out := make([]ficha, 0, n/2+16)
	const bitsHash = 16
	cabeza := make([]int32, 1<<bitsHash)
	for i := range cabeza {
		cabeza[i] = -1
	}
	previo := make([]int32, n)
	hash := func(i int) uint32 {
		return (px[i]*0x9E3779B1 ^ px[i+1]*0x85EBCA6B ^ px[i+2]*0xC2B2AE35) >> (32 - bitsHash)
	}
	inserta := func(i int) {
		if i+2 < n {
			h := hash(i)
			previo[i] = cabeza[h]
			cabeza[h] = int32(i)
		}
	}
	largoDesde := func(i, d int) int {
		l := 0
		for i+l < n && l < maxLongitud && px[i+l] == px[i+l-d] {
			l++
		}
		return l
	}
	i := 0
	for i < n {
		mejorL, mejorD := 0, 0
		if i >= 1 {
			if l := largoDesde(i, 1); l > mejorL {
				mejorL, mejorD = l, 1
			}
		}
		if i >= ancho && ancho > 1 {
			if l := largoDesde(i, ancho); l > mejorL {
				mejorL, mejorD = l, ancho
			}
		}
		if i+2 < n {
			c := cabeza[hash(i)]
			for paso := 0; c >= 0 && paso < 24; paso++ {
				d := i - int(c)
				if d > 1<<20-120 {
					break
				}
				if l := largoDesde(i, d); l > mejorL {
					mejorL, mejorD = l, d
				}
				c = previo[c]
			}
		}
		if mejorL >= 3 {
			cod := mejorD + 120
			switch mejorD {
			case 1:
				cod = 2
			case ancho:
				cod = 1
			}
			out = append(out, ficha{copia: true, largo: mejorL, dcod: cod})
			for k := 0; k < mejorL; k++ {
				inserta(i + k)
			}
			i += mejorL
			continue
		}
		out = append(out, ficha{px: px[i]})
		inserta(i)
		i++
	}
	return out
}

// codificarPaleta arma un WebP sin pérdida de una imagen de paleta. indices: un índice por píxel (ancho×alto); paleta: ARGB.
func codificarPaleta(ancho, alto int, indices []uint8, paleta []uint32, conAlfa bool) ([]byte, error) {
	if ancho < 1 || alto < 1 || ancho > 16384 || alto > 16384 || len(indices) != ancho*alto {
		return nil, errors.New("medidas no válidas")
	}
	if len(paleta) < 1 || len(paleta) > 256 {
		return nil, errors.New("paleta no válida")
	}
	w := &escritorBits{}
	w.bits(0x2f, 8)
	w.bits(uint32(ancho-1), 14)
	w.bits(uint32(alto-1), 14)
	if conAlfa {
		w.bits(1, 1)
	} else {
		w.bits(0, 1)
	}
	w.bits(0, 3) // versión
	// transformación de paleta
	w.bits(1, 1)
	w.bits(3, 2)
	w.bits(uint32(len(paleta)-1), 8)
	delta := make([]ficha, len(paleta))
	var ant uint32
	for i, c := range paleta {
		d := c
		if i > 0 {
			d = (((c>>24 - ant>>24) & 0xff) << 24) | (((c>>16 - ant>>16) & 0xff) << 16) | (((c>>8 - ant>>8) & 0xff) << 8) | ((c - ant) & 0xff)
		}
		delta[i] = ficha{px: d}
		ant = c
	}
	escribeImagen(w, delta, false)
	w.bits(0, 1) // no hay más transformaciones
	// píxeles de índices, empaquetados si la paleta es chica
	bpp, anchoBits := uint(8), uint(0)
	switch {
	case len(paleta) <= 2:
		bpp, anchoBits = 1, 3
	case len(paleta) <= 4:
		bpp, anchoBits = 2, 2
	case len(paleta) <= 16:
		bpp, anchoBits = 4, 1
	}
	anchoP := (ancho + (1 << anchoBits) - 1) >> anchoBits
	px := make([]uint32, anchoP*alto)
	if anchoBits == 0 {
		for i, ix := range indices {
			px[i] = 0xff000000 | uint32(ix)<<8
		}
	} else {
		mask := 1<<anchoBits - 1
		for y := 0; y < alto; y++ {
			for x := 0; x < ancho; x++ {
				j := y*anchoP + x>>anchoBits
				px[j] |= uint32(indices[y*ancho+x]) << (8 + bpp*uint(x&mask))
			}
		}
		for j := range px {
			px[j] |= 0xff000000
		}
	}
	escribeImagen(w, fichasDe(px, anchoP), true)
	datos := w.fin()
	if len(datos)+12 > 1<<30 {
		return nil, errors.New("imagen demasiado grande")
	}
	pad := len(datos) & 1
	out := make([]byte, 0, 20+len(datos)+pad)
	out = append(out, "RIFF"...)
	out = binary.LittleEndian.AppendUint32(out, uint32(4+8+len(datos)+pad))
	out = append(out, "WEBPVP8L"...)
	out = binary.LittleEndian.AppendUint32(out, uint32(len(datos)))
	out = append(out, datos...)
	if pad == 1 {
		out = append(out, 0)
	}
	return out, nil
}
