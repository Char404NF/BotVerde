package wabot

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestApiRespuestasListaCambiaYRestaura(t *testing.T) {
	reinicia(t)
	if err := AbrirAlmacen(t.TempDir(), clavePrueba(), &obsFalso{}); err != nil {
		t.Fatal(err)
	}
	var r struct {
		Grupos []string
		Filas  []struct {
			ID, Defecto, Propio string
			Vars                []string
		}
		Reemplazos []struct{ De, A string }
	}
	carga := func() {
		j, err := RespuestasLista(1)
		if err != nil {
			t.Fatal(err)
		}
		r.Filas, r.Reemplazos = nil, nil
		if err := json.Unmarshal([]byte(j), &r); err != nil {
			t.Fatal(err, j)
		}
	}
	carga()
	if len(r.Filas) < 55 || len(r.Grupos) < 8 || r.Reemplazos == nil && false {
		t.Fatalf("catálogo corto: %d filas, %d grupos", len(r.Filas), len(r.Grupos))
	}
	if err := RespuestaFijar(1, "grupo.aprobadas", "Entraron {n}"); err != nil {
		t.Fatal(err)
	}
	if err := RespuestaFijar(1, "grupo.aprobadas", "Entró {quien}"); err == nil || !strings.Contains(err.Error(), "{n}") {
		t.Fatalf("variable inventada: %v", err)
	}
	if err := RespuestaFijar(1, "no.existe", "x"); err == nil {
		t.Fatal("no existe")
	}
	if err := ReemplazosFijar(1, `[{"de":"hola","a":"buenas"}]`); err != nil {
		t.Fatal(err)
	}
	if err := ReemplazosFijar(1, `no es json`); err == nil {
		t.Fatal("json inválido")
	}
	carga()
	var propio string
	for _, f := range r.Filas {
		if f.ID == "grupo.aprobadas" {
			propio = f.Propio
		}
	}
	if propio != "Entraron {n}" || len(r.Reemplazos) != 1 {
		t.Fatalf("propio=%q reemplazos=%v", propio, r.Reemplazos)
	}
	if n, err := RespuestasRestaurar(1); err != nil || n != 2 {
		t.Fatalf("%d %v", n, err)
	}
	carga()
	if len(r.Reemplazos) != 0 {
		t.Fatal("debían borrarse")
	}
}
