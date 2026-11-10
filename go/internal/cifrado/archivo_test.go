package cifrado

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// Datos de referencia compartidos con la prueba de Kotlin (ver pruebas_cruzadas/LEEME.md).
func claveRef() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i)
	}
	return k
}

func nonceRef() (n [12]byte) {
	for i := range n {
		n[i] = byte(0xA0 + i)
	}
	return
}

func patron(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*31 + 7)
	}
	return b
}

var tamanosRef = []int{0, 1, 65535, 65536, 65537, 131072, 200000}

func cifraRef(t *testing.T, plano []byte) []byte {
	t.Helper()
	var sb sliceBuf
	e, err := NuevoEscritorConNonce(&sb, claveRef(), nonceRef())
	if err != nil {
		t.Fatal(err)
	}
	// Se escribe en trozos irregulares para ejercitar el relleno de bloques.
	for p := plano; len(p) > 0; {
		c := 7919
		if c > len(p) {
			c = len(p)
		}
		if _, err := e.Write(p[:c]); err != nil {
			t.Fatal(err)
		}
		p = p[c:]
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	return sb.b
}

func rutaRef(n int) string {
	return filepath.Join("..", "..", "..", "pruebas_cruzadas", fmt.Sprintf("n%d.bcf", n))
}

func TestArchivosDeReferencia(t *testing.T) {
	actualizar := os.Getenv("ACTUALIZAR_FIXTURES") == "1"
	for _, n := range tamanosRef {
		plano := patron(n)
		cif := cifraRef(t, plano)
		if actualizar {
			if err := os.WriteFile(rutaRef(n), cif, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		guardado, err := os.ReadFile(rutaRef(n))
		if err != nil {
			t.Fatalf("falta el archivo de referencia de %d bytes: %v", n, err)
		}
		if !bytes.Equal(cif, guardado) {
			t.Fatalf("n=%d: lo que cifra Go no coincide con el archivo de referencia", n)
		}
		des, err := DescifrarBytes(claveRef(), guardado)
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		if !bytes.Equal(des, plano) {
			t.Fatalf("n=%d: el descifrado no coincide", n)
		}
	}
}

func TestTamanoCifrado(t *testing.T) {
	// 0 bytes: cabecera + un bloque vacío (solo etiqueta). Un bloque exacto no agrega un bloque vacío.
	casos := map[int]int{0: 20 + 16, 1: 20 + 17, 65536: 20 + 65536 + 16, 65537: 20 + 65536 + 16 + 17}
	for n, esperado := range casos {
		if got := len(cifraRef(t, patron(n))); got != esperado {
			t.Errorf("n=%d: tamaño %d, se esperaba %d", n, got, esperado)
		}
	}
}

func TestAccesoAleatorio(t *testing.T) {
	plano := patron(300000)
	cif := cifraRef(t, plano)
	l, err := Abrir(bytes.NewReader(cif), int64(len(cif)), claveRef())
	if err != nil {
		t.Fatal(err)
	}
	if l.Tam() != int64(len(plano)) {
		t.Fatalf("tamaño %d", l.Tam())
	}
	for _, c := range []struct{ off, n int }{{0, 10}, {65530, 20}, {65536, 65536}, {131071, 2}, {299990, 10}, {100000, 150000}} {
		buf := make([]byte, c.n)
		n, err := l.ReadAt(buf, int64(c.off))
		if err != nil && !(errors.Is(err, io.EOF) && n == c.n) {
			t.Fatalf("off=%d n=%d: %v", c.off, c.n, err)
		}
		if !bytes.Equal(buf[:n], plano[c.off:c.off+n]) {
			t.Fatalf("off=%d n=%d: datos distintos", c.off, c.n)
		}
	}
	// Leer más allá del final devuelve EOF.
	n, err := l.ReadAt(make([]byte, 10), 299995)
	if n != 5 || !errors.Is(err, io.EOF) {
		t.Fatalf("lectura parcial al final: n=%d err=%v", n, err)
	}
	// Lectura secuencial completa.
	todo, err := io.ReadAll(io.NewSectionReader(l, 0, l.Tam()))
	if err != nil || !bytes.Equal(todo, plano) {
		t.Fatalf("lectura secuencial: %v", err)
	}
}

func TestManipulacionSeDetecta(t *testing.T) {
	plano := patron(200000)
	cif := cifraRef(t, plano)
	abre := func(b []byte) error {
		_, err := DescifrarBytes(claveRef(), b)
		return err
	}
	// Un bit cambiado en el cuerpo.
	m := append([]byte(nil), cif...)
	m[TamCabecera+70000] ^= 1
	if !errors.Is(abre(m), ErrAutenticacion) {
		t.Error("un bit alterado debía fallar")
	}
	// Cabecera alterada (el nonce base).
	m = append([]byte(nil), cif...)
	m[10] ^= 1
	if !errors.Is(abre(m), ErrAutenticacion) {
		t.Error("la cabecera alterada debía fallar")
	}
	// Archivo cortado justo en el límite de un bloque: el penúltimo bloque no era el último.
	corte := cif[:TamCabecera+2*(TamBloque+TamEtiqueta)]
	if !errors.Is(abre(corte), ErrAutenticacion) {
		t.Error("cortar en un límite de bloque debía fallar")
	}
	// Bloques intercambiados.
	m = append([]byte(nil), cif...)
	a, b := TamCabecera, TamCabecera+TamBloque+TamEtiqueta
	copy(m[a:a+tamBloqueCifrado], cif[b:b+tamBloqueCifrado])
	copy(m[b:b+tamBloqueCifrado], cif[a:a+tamBloqueCifrado])
	if !errors.Is(abre(m), ErrAutenticacion) {
		t.Error("bloques intercambiados debían fallar")
	}
	// Llave equivocada.
	otra := make([]byte, 32)
	if _, err := DescifrarBytes(otra, cif); !errors.Is(err, ErrAutenticacion) {
		t.Error("una llave equivocada debía fallar")
	}
	// Formato inválido.
	if _, err := DescifrarBytes(claveRef(), []byte("nada")); !errors.Is(err, ErrFormato) {
		t.Error("un archivo pequeño debía dar error de formato")
	}
	m = append([]byte(nil), cif...)
	m[0] = 'X'
	if !errors.Is(abre(m), ErrFormato) {
		t.Error("magia equivocada debía dar error de formato")
	}
	// El vacío también se autentica.
	vacio := cifraRef(t, nil)
	vacio[TamCabecera] ^= 1
	if !errors.Is(abre(vacio), ErrAutenticacion) {
		t.Error("un archivo vacío alterado debía fallar")
	}
}

func TestClaveInvalida(t *testing.T) {
	if _, err := NuevoEscritor(io.Discard, []byte("corta")); !errors.Is(err, ErrClave) {
		t.Fatal("una llave corta debía fallar")
	}
}

func TestNonceConAcarreo(t *testing.T) {
	var base [12]byte
	for i := range base {
		base[i] = 0xff
	}
	got := nonceBloque(base, 1)
	if got != ([12]byte{}) {
		t.Fatalf("el acarreo debía dar todo ceros, dio %x", got)
	}
	base = [12]byte{}
	base[11] = 0xff
	got = nonceBloque(base, 0x0102)
	want := [12]byte{}
	want[10], want[11] = 0x02, 0x01
	if got != want {
		t.Fatalf("got %x want %x", got, want)
	}
}

func TestCifradoAleatorio(t *testing.T) {
	k := make([]byte, 32)
	_, _ = rand.Read(k)
	plano := patron(123456)
	a, _ := CifrarBytes(k, plano)
	b, _ := CifrarBytes(k, plano)
	if bytes.Equal(a, b) {
		t.Fatal("dos cifrados del mismo archivo no deben ser iguales (nonce aleatorio)")
	}
	des, err := DescifrarBytes(k, a)
	if err != nil || !bytes.Equal(des, plano) {
		t.Fatalf("ida y vuelta: %v", err)
	}
}

func TestTextoSellado(t *testing.T) {
	k := claveRef()
	s, err := SellarTexto(k, "texto", []byte("hola mundo ñ"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(s, []byte("hola")) {
		t.Fatal("el texto sellado no debe contener el texto en claro")
	}
	p, err := AbrirTexto(k, "texto", s)
	if err != nil || string(p) != "hola mundo ñ" {
		t.Fatalf("%q %v", p, err)
	}
	if _, err := AbrirTexto(k, "otra_columna", s); err == nil {
		t.Fatal("el AAD de columna distinta debía fallar")
	}
	if v, _ := SellarTexto(k, "texto", nil); v != nil {
		t.Fatal("un texto vacío debe ser NULL")
	}
	if p, err := AbrirTexto(k, "texto", nil); p != nil || err != nil {
		t.Fatal("NULL debe abrirse como vacío")
	}
	s2, _ := SellarTexto(k, "texto", []byte("hola mundo ñ"))
	if bytes.Equal(s, s2) {
		t.Fatal("dos sellados iguales no deben dar el mismo resultado")
	}
}
