package grupos

import "testing"

func TestContienePalabra(t *testing.T) {
	casos := []struct {
		texto, p string
		quiero   bool
	}{
		{"eres un pendejo", "pendejo", true},
		{"eres un p3nd3j0", "pendejo", true},
		{"p u t o", "puto", true},
		{"p.u.t.o", "puto", true},
		{"puuuuto!", "puto", true},
		{"PUT@", "puta", true},
		{"puta!", "puta", true},
		{"p3nd3j0!!", "pendejo", true},
		{"qué pendéjo", "pendejo", true},
		{"computadora nueva", "puta", false},
		{"disputa por la reputación", "puta", false},
		{"la escuela está en la calle 7", "puta", false},
		{"hola a todos", "puta", false},
		{"vete a la verga ya", "vete a la verga", true},
		{"vete  a la  VERGA", "vete a la verga", true},
		{"vete a la casa", "vete a la verga", false},
		{"", "puto", false},
		{"algo", "", false},
		{"el puto amo", "pu", false},
		{"1 2 3 4", "puto", false},
	}
	for _, c := range casos {
		if got := ContienePalabra(c.texto, c.p); got != c.quiero {
			t.Errorf("ContienePalabra(%q, %q) = %v, quiero %v", c.texto, c.p, got, c.quiero)
		}
	}
}

func TestListasDeFabricaSinVacias(t *testing.T) {
	for _, l := range [][]string{PalabrasDeFabrica, PalabrasAdultasDeFabrica} {
		for _, p := range l {
			if colapsa(p, false) == "" {
				t.Errorf("palabra de fábrica vacía: %q", p)
			}
		}
	}
}

func TestEnlacesModoAdultos(t *testing.T) {
	if !DominioAdulto("www.pornhub.com") || !DominioAdulto("algo.xxx") || DominioAdulto("youtube.com") || DominioAdulto("google.com") {
		t.Error("DominioAdulto no distingue bien")
	}
	if !DominioAcortador("bit.ly") || DominioAcortador("bitly.fake.com") {
		t.Error("DominioAcortador no distingue bien")
	}
}
