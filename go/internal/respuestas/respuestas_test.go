package respuestas

import (
	"strings"
	"testing"

	"botchat/internal/almacen"
)

func alm(t *testing.T) *almacen.Almacen {
	t.Helper()
	k := make([]byte, 32)
	a, err := almacen.Abrir(t.TempDir(), k)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Cerrar() })
	return a
}

func TestElTextoDeFabricaSeLlenaYElPropioLoReemplaza(t *testing.T) {
	a := alm(t)
	if got := Texto(a, 1, "grupo.aprobadas", "n", "3"); got != "Listo: aprobé 3 solicitudes." {
		t.Fatalf("%q", got)
	}
	if err := a.FijarRespuesta(1, "grupo.aprobadas", "Entraron {n} personas nuevas 🎉"); err != nil {
		t.Fatal(err)
	}
	if got := Texto(a, 1, "grupo.aprobadas", "n", "3"); got != "Entraron 3 personas nuevas 🎉" {
		t.Fatalf("%q", got)
	}
	// otro bot no ve el cambio; vaciar vuelve al de fábrica
	if got := Texto(a, 2, "grupo.aprobadas", "n", "3"); !strings.HasPrefix(got, "Listo:") {
		t.Fatalf("%q", got)
	}
	_ = a.FijarRespuesta(1, "grupo.aprobadas", "   ")
	if got := Texto(a, 1, "grupo.aprobadas", "n", "3"); got != "Listo: aprobé 3 solicitudes." {
		t.Fatalf("%q", got)
	}
	if Texto(a, 1, "no.existe") != "" {
		t.Fatal("una respuesta que no existe no contesta nada")
	}
}

func TestValidarRechazaVariablesInventadasYTextosLargos(t *testing.T) {
	if err := Validar("grupo.aprobadas", "Entraron {n}"); err != nil {
		t.Fatal(err)
	}
	if err := Validar("grupo.aprobadas", "Entró {nombre}"); err == nil || !strings.Contains(err.Error(), "{n}") {
		t.Fatalf("%v", err)
	}
	if err := Validar("grupo.nombre", "Hola {x}"); err == nil || !strings.Contains(err.Error(), "no usa variables") {
		t.Fatalf("%v", err)
	}
	if err := Validar("grupo.nombre", strings.Repeat("a", 601)); err == nil {
		t.Fatal("muy largo")
	}
	if err := Validar("nada", "x"); err == nil {
		t.Fatal("no existe")
	}
	if err := Validar("grupo.nombre", "Texto con { llave suelta"); err != nil {
		t.Fatalf("una llave suelta no es variable: %v", err)
	}
}

func TestElCatalogoEstaCompletoYSusVariablesSeUsanEnElDefecto(t *testing.T) {
	cs := Catalogo()
	if len(cs) < 55 {
		t.Fatalf("catálogo corto: %d", len(cs))
	}
	vistos := map[string]bool{}
	for _, d := range cs {
		if d.Grupo == "" || d.Descripcion == "" || d.Defecto == "" || vistos[d.ID] {
			t.Fatalf("incompleta o repetida: %+v", d)
		}
		vistos[d.ID] = true
		// cada {variable} del texto de fábrica está declarada, y cada variable declarada se usa
		usadas := map[string]bool{}
		for _, v := range variablesDe(d.Defecto) {
			usadas[v] = true
		}
		for _, v := range d.Vars {
			if !usadas[v] {
				t.Errorf("%s declara {%s} pero el texto de fábrica no la usa", d.ID, v)
			}
		}
		if err := Validar(d.ID, d.Defecto); err != nil {
			t.Errorf("%s: su propio texto de fábrica no es válido: %v", d.ID, err)
		}
	}
}

func TestRestaurarTodasLasRespuestas(t *testing.T) {
	a := alm(t)
	_ = a.FijarRespuesta(1, "grupo.nombre", "uno")
	_ = a.FijarRespuesta(1, "grupo.cerrado", "dos")
	if n := a.RestaurarRespuestas(1); n != 2 {
		t.Fatalf("%d", n)
	}
	for _, f := range Lista(a, 1) {
		if f.Propio != "" {
			t.Fatalf("quedó propio: %+v", f)
		}
	}
}

func TestCambiarCualquierFraseFunciona(t *testing.T) {
	a := alm(t)
	if Aplicar(a, 1, "No tienes permiso para usar ese comando.") != "No tienes permiso para usar ese comando." {
		t.Fatal("sin cambios no toca nada")
	}
	if err := FijarReemplazos(a, 1, []Reemplazo{{De: "No tienes permiso", A: "Eso solo lo hacen los admins"}, {De: "", A: "x"}, {De: "No tienes permiso", A: "repetido"}, {De: "hola", A: "buenas"}}); err != nil {
		t.Fatal(err)
	}
	if got := Aplicar(a, 1, "No tienes permiso para usar ese comando. hola"); got != "Eso solo lo hacen los admins para usar ese comando. buenas" {
		t.Fatalf("%q", got)
	}
	if n := len(Reemplazos(a, 1)); n != 2 {
		t.Fatalf("debían quedar 2: %d", n)
	}
	if err := FijarReemplazos(a, 1, nil); err != nil || len(Reemplazos(a, 1)) != 0 {
		t.Fatal("vaciar")
	}
	if err := FijarReemplazos(a, 1, []Reemplazo{{De: strings.Repeat("a", 201), A: "x"}}); err == nil {
		t.Fatal("frase muy larga")
	}
}
