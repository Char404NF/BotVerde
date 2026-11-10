// Package cifrado implementa el formato de archivo cifrado de BotChat y el sellado de textos.
//
// FORMATO DE ARCHIVO (versión 1). Lo escriben Go y Kotlin con exactamente este formato:
//
//	Cabecera (20 bytes):
//	  0..3   magia "BCF1"
//	  4      versión (1)
//	  5      log2 del tamaño de bloque (16 = 64 KB)
//	  6..7   reservados (0)
//	  8..19  nonce base (12 bytes aleatorios)
//	Bloques, uno tras otro, cada uno = AES-256-GCM(texto del bloque) = cifrado || etiqueta(16 bytes):
//	  el texto de cada bloque mide 65536 bytes, salvo el último, que mide entre 0 y 65536.
//	  Siempre hay al menos un bloque (un archivo vacío tiene un bloque de 0 bytes: solo la etiqueta).
//	  nonce del bloque i = nonce base + i (entero de 96 bits, big-endian, sumado con acarreo).
//	  datos adicionales autenticados (AAD) = cabecera(20) || i (8 bytes big-endian) || último (1 byte: 0 o 1).
//
// El tamaño del texto se deduce del tamaño del archivo; la marca de "último bloque" en el AAD
// hace que cortar el archivo, o cambiar bloques de lugar, se detecte al leer.
package cifrado

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
)

const (
	// TamBloque es el tamaño del texto en claro de cada bloque (64 KB).
	TamBloque = 64 * 1024
	// TamCabecera es el tamaño de la cabecera del archivo.
	TamCabecera = 20
	// TamEtiqueta es el tamaño de la etiqueta de autenticación de AES-GCM.
	TamEtiqueta = 16
	// TamClave es el tamaño de la llave de datos.
	TamClave = 32

	tamBloqueCifrado = TamBloque + TamEtiqueta
	versionArchivo   = 1
	logTamBloque     = 16
)

var magia = [4]byte{'B', 'C', 'F', '1'}

var (
	// ErrClave se devuelve si la llave no mide 32 bytes.
	ErrClave = errors.New("la llave de datos debe medir 32 bytes")
	// ErrFormato se devuelve si el archivo no tiene el formato esperado.
	ErrFormato = errors.New("el archivo cifrado no tiene un formato válido")
	// ErrAutenticacion se devuelve si un bloque fue alterado, cortado o la llave no corresponde.
	ErrAutenticacion = errors.New("el archivo cifrado está dañado o la llave no corresponde")
)

func nuevoAEAD(clave []byte) (cipher.AEAD, error) {
	if len(clave) != TamClave {
		return nil, ErrClave
	}
	b, err := aes.NewCipher(clave)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(b)
}

// nonceBloque suma el contador al nonce base (96 bits, big-endian, con acarreo).
func nonceBloque(base [12]byte, i uint64) [12]byte {
	n := base
	carry := i
	for p := 11; p >= 0 && carry != 0; p-- {
		s := uint64(n[p]) + (carry & 0xff)
		n[p] = byte(s)
		carry = (carry >> 8) + (s >> 8)
	}
	return n
}

func aadBloque(cab []byte, i uint64, ultimo bool) []byte {
	a := make([]byte, 0, TamCabecera+9)
	a = append(a, cab...)
	var idx [8]byte
	binary.BigEndian.PutUint64(idx[:], i)
	a = append(a, idx[:]...)
	if ultimo {
		a = append(a, 1)
	} else {
		a = append(a, 0)
	}
	return a
}

func armaCabecera(nonce [12]byte) [TamCabecera]byte {
	var c [TamCabecera]byte
	copy(c[0:4], magia[:])
	c[4] = versionArchivo
	c[5] = logTamBloque
	copy(c[8:20], nonce[:])
	return c
}

// Escritor cifra en flujo hacia w. Hay que llamar a Close para escribir el último bloque.
type Escritor struct {
	w      io.Writer
	aead   cipher.AEAD
	cab    [TamCabecera]byte
	base   [12]byte
	idx    uint64
	buf    []byte
	n      int
	hdr    bool
	cerrar bool
	err    error
}

// NuevoEscritor crea un escritor con un nonce base aleatorio.
func NuevoEscritor(w io.Writer, clave []byte) (*Escritor, error) {
	var nonce [12]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, err
	}
	return NuevoEscritorConNonce(w, clave, nonce)
}

// NuevoEscritorConNonce fija el nonce base. Solo para pruebas con archivos de referencia: en producción
// un nonce repetido con la misma llave rompe la seguridad de GCM.
func NuevoEscritorConNonce(w io.Writer, clave []byte, nonce [12]byte) (*Escritor, error) {
	a, err := nuevoAEAD(clave)
	if err != nil {
		return nil, err
	}
	return &Escritor{w: w, aead: a, cab: armaCabecera(nonce), base: nonce, buf: make([]byte, TamBloque)}, nil
}

func (e *Escritor) cabecera() error {
	if e.hdr {
		return nil
	}
	e.hdr = true
	_, err := e.w.Write(e.cab[:])
	return err
}

func (e *Escritor) vacia(ultimo bool) error {
	if err := e.cabecera(); err != nil {
		return err
	}
	nonce := nonceBloque(e.base, e.idx)
	salida := e.aead.Seal(nil, nonce[:], e.buf[:e.n], aadBloque(e.cab[:], e.idx, ultimo))
	if _, err := e.w.Write(salida); err != nil {
		return err
	}
	e.idx++
	e.n = 0
	return nil
}

// Write cifra los datos; un bloque lleno solo se escribe cuando llegan más datos (para poder marcar el último).
func (e *Escritor) Write(p []byte) (int, error) {
	if e.err != nil {
		return 0, e.err
	}
	if e.cerrar {
		return 0, errors.New("escritor cerrado")
	}
	total := len(p)
	for len(p) > 0 {
		if e.n == TamBloque {
			if err := e.vacia(false); err != nil {
				e.err = err
				return total - len(p), err
			}
		}
		c := copy(e.buf[e.n:], p)
		e.n += c
		p = p[c:]
	}
	return total, nil
}

// Close escribe el último bloque (aunque esté vacío).
func (e *Escritor) Close() error {
	if e.cerrar {
		return e.err
	}
	e.cerrar = true
	if e.err != nil {
		return e.err
	}
	e.err = e.vacia(true)
	return e.err
}

// Lector descifra con acceso aleatorio. Es seguro para uso concurrente.
type Lector struct {
	r        io.ReaderAt
	aead     cipher.AEAD
	cab      [TamCabecera]byte
	base     [12]byte
	bloques  int64
	tam      int64
	mu       sync.Mutex
	cacheIdx int64
	cache    []byte
}

// Abrir prepara la lectura de un archivo cifrado de tamaño tamCifrado.
func Abrir(r io.ReaderAt, tamCifrado int64, clave []byte) (*Lector, error) {
	a, err := nuevoAEAD(clave)
	if err != nil {
		return nil, err
	}
	l := &Lector{r: r, aead: a, cacheIdx: -1}
	if tamCifrado < TamCabecera+TamEtiqueta {
		return nil, ErrFormato
	}
	if _, err := r.ReadAt(l.cab[:], 0); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormato, err)
	}
	if [4]byte(l.cab[0:4]) != magia || l.cab[4] != versionArchivo || l.cab[5] != logTamBloque {
		return nil, ErrFormato
	}
	copy(l.base[:], l.cab[8:20])
	cuerpo := tamCifrado - TamCabecera
	l.bloques = (cuerpo + tamBloqueCifrado - 1) / tamBloqueCifrado
	ultimo := cuerpo - (l.bloques-1)*tamBloqueCifrado
	if ultimo < TamEtiqueta {
		return nil, ErrFormato
	}
	l.tam = cuerpo - l.bloques*TamEtiqueta
	return l, nil
}

// Tam es el tamaño del texto en claro.
func (l *Lector) Tam() int64 { return l.tam }

func (l *Lector) bloque(i int64) ([]byte, error) {
	if l.cacheIdx == i {
		return l.cache, nil
	}
	ultimo := i == l.bloques-1
	largo := int64(tamBloqueCifrado)
	if ultimo {
		largo = l.tam - (l.bloques-1)*TamBloque + TamEtiqueta
	}
	ct := make([]byte, largo)
	if _, err := l.r.ReadAt(ct, TamCabecera+i*tamBloqueCifrado); err != nil && !(errors.Is(err, io.EOF) && int64(len(ct)) == largo) {
		if errors.Is(err, io.EOF) {
			return nil, ErrFormato
		}
		return nil, err
	}
	nonce := nonceBloque(l.base, uint64(i))
	pt, err := l.aead.Open(ct[:0], nonce[:], ct, aadBloque(l.cab[:], uint64(i), ultimo))
	if err != nil {
		return nil, ErrAutenticacion
	}
	l.cacheIdx, l.cache = i, pt
	return pt, nil
}

// ReadAt implementa io.ReaderAt sobre el texto en claro.
func (l *Lector) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("posición negativa")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for n < len(p) {
		pos := off + int64(n)
		if pos >= l.tam {
			return n, io.EOF
		}
		b, err := l.bloque(pos / TamBloque)
		if err != nil {
			return n, err
		}
		n += copy(p[n:], b[pos%TamBloque:])
	}
	return n, nil
}

// CifrarBytes cifra un bloque de memoria con el formato de archivo.
func CifrarBytes(clave, plano []byte) ([]byte, error) {
	var sb sliceBuf
	e, err := NuevoEscritor(&sb, clave)
	if err != nil {
		return nil, err
	}
	if _, err := e.Write(plano); err != nil {
		return nil, err
	}
	if err := e.Close(); err != nil {
		return nil, err
	}
	return sb.b, nil
}

// DescifrarBytes descifra un archivo completo en memoria.
func DescifrarBytes(clave, cifrado []byte) ([]byte, error) {
	l, err := Abrir(bytesReaderAt(cifrado), int64(len(cifrado)), clave)
	if err != nil {
		return nil, err
	}
	out := make([]byte, l.Tam())
	if len(out) == 0 {
		// Aun vacío hay que autenticar el único bloque.
		if _, err := l.bloque(0); err != nil {
			return nil, err
		}
		return out, nil
	}
	if _, err := l.ReadAt(out, 0); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return out, nil
}

type sliceBuf struct{ b []byte }

func (s *sliceBuf) Write(p []byte) (int, error) { s.b = append(s.b, p...); return len(p), nil }

type bytesReaderAt []byte

func (b bytesReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(b)) {
		return 0, io.EOF
	}
	n := copy(p, b[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
