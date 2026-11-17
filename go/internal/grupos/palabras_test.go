package grupos

import (
	"fmt"
	"testing"
	"time"
)

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

func TestAccionBorrarAdvertirEsValida(t *testing.T) {
	if !EsAccionFiltro(AccBorrarAdv) || textoAccion(AccBorrarAdv) == AccBorrarAdv {
		t.Error("la acción borrar_advertir no está registrada")
	}
}

func TestContieneAlgunaDaLoMismoQueContienePalabraConCadaUna(t *testing.T) {
	textos := []string{
		"hola a todos", "eres un p3nd3j0", "qué p u t o día", "que puuuuuta madre", "p.u.t.o", "vete a la verga ya", "chinga tu madre!", "bien chingado el examen",
		"nada de nada", "m1erd@", "un texto normal con muchas palabras sin problema ninguno", "", "ctm", "tu madre", "A B C", "hdp!!", "Estúpido total",
	}
	lista := append([]string{"puto", "pendejo", "vete a la verga", "chinga tu madre", "mierda", "ctm", "hdp", "estupido", "  ", "tu madre", "x"}, PalabrasDeFabrica...)
	for _, tx := range textos {
		esperado := false
		for _, p := range lista {
			if ContienePalabra(tx, p) {
				esperado = true
				break
			}
		}
		if got := ContieneAlguna(tx, lista); got != esperado {
			t.Errorf("%q: ContieneAlguna=%v, una por una=%v", tx, got, esperado)
		}
		for _, p := range lista {
			if ContienePalabra(tx, p) != ContieneAlguna(tx, []string{p}) {
				t.Errorf("%q con %q no coincide", tx, p)
			}
		}
	}
	if ContieneAlguna("hola", nil) || ContieneAlguna("hola", []string{}) {
		t.Fatal("sin lista no hay nada que detectar")
	}
}

func TestMilesDePalabrasSiguenSiendoRapidasYSeGuardan(t *testing.T) {
	var lista []string
	for i := 0; i < MaxPalabras; i++ {
		lista = append(lista, fmt.Sprintf("palabra%dxyz", i))
	}
	inicio := time.Now()
	for i := 0; i < 50; i++ {
		if ContieneAlguna("un mensaje normal de buenos días para todos los del grupo, ¿cómo están?", lista) {
			t.Fatal("no debía detectar nada")
		}
	}
	if !ContieneAlguna("mira esta palabra4999xyz aquí", lista) {
		t.Fatal("la última de la lista también cuenta")
	}
	if d := time.Since(inicio); d > 2*time.Second {
		t.Fatalf("50 mensajes con %d palabras tardaron %v", len(lista), d)
	}
	// y el filtro las guarda todas (hasta el tope)
	e := nuevo(t, true)
	e.filtro(FiltroPalabras, true, AccAvisarme, func(c *ConfigFiltro) { c.Palabras = append(append([]string{}, lista...), "sobra1", "sobra2") })
	for _, f := range e.s.FiltrosDe(grupoJID) {
		if f.Filtro == FiltroPalabras && len(f.Config.Palabras) != MaxPalabras {
			t.Fatalf("se guardaron %d, esperaba %d", len(f.Config.Palabras), MaxPalabras)
		}
	}
}
