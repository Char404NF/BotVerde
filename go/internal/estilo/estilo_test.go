package estilo

import (
	"bytes"
	"strings"
	"testing"
)

func TestTarjetaTieneTituloRayaLineasYPie(t *testing.T) {
	r := Tarjeta("🧮", "Calculadora", []string{"a", "b"}, "ayuda")
	if r != "🧮 *Calculadora*\n"+Raya+"\na\nb\n\n💡 _ayuda_" {
		t.Fatalf("%q", r)
	}
	if strings.Contains(Tarjeta("", "X", nil, ""), "💡") {
		t.Fatal("sin pie no hay línea de ayuda")
	}
}

func TestLas8CarasDelAvatarVienenDentroYSonPNG(t *testing.T) {
	if len(Caras) != 8 {
		t.Fatal(len(Caras))
	}
	for _, c := range Caras {
		b := Cara(c)
		if len(b) < 1000 || !bytes.HasPrefix(b, []byte("\x89PNG")) {
			t.Fatalf("cara %s: %d bytes", c, len(b))
		}
	}
	if Cara("no_existe") != nil {
		t.Fatal("una cara inexistente devuelve nil")
	}
}
