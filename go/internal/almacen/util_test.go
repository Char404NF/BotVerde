package almacen

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestMigracion16UtilItems(t *testing.T) {
	a := nuevo(t)
	if a.Version() < 16 {
		t.Fatalf("falta la migración 16: %d", a.Version())
	}
	var n int
	if err := a.db.QueryRow(`SELECT COUNT(*) FROM util_items`).Scan(&n); err != nil {
		t.Fatal(err)
	}
}

func TestNotasYPendientesPropiosCifradosYPorAlias(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	a.AprenderAlias(1, pnAna, lidAna)
	n1, err := a.AgregarItemUtil(1, lidAna, UtilNota, "  comprar tornillos secretos  ", ahora)
	if err != nil || n1.Texto != "comprar tornillos secretos" {
		t.Fatalf("%+v %v", n1, err)
	}
	p1, _ := a.AgregarItemUtil(1, pnAna, UtilPendiente, "llamar al banco", ahora)
	_, _ = a.AgregarItemUtil(1, PersonaDueno, UtilNota, "nota del dueño", ahora)
	// por cualquiera de sus formas ven lo mismo; el dueño y los demás no se mezclan
	if l := a.ListarItemsUtil(1, pnAna, UtilNota, true); len(l) != 1 || l[0].Texto != "comprar tornillos secretos" {
		t.Fatalf("notas de Ana: %+v", l)
	}
	if l := a.ListarItemsUtil(1, lidAna, "", true); len(l) != 2 {
		t.Fatalf("ambos tipos por el LID: %+v", l)
	}
	if l := a.ListarItemsUtil(1, PersonaDueno, "", true); len(l) != 1 || l[0].Texto != "nota del dueño" {
		t.Fatalf("dueño: %+v", l)
	}
	// el texto no está en claro en el disco
	var b []byte
	if err := a.db.QueryRow(`SELECT texto_cifrado FROM util_items WHERE id=?`, n1.ID).Scan(&b); err != nil || bytes.Contains(b, []byte("tornillos")) {
		t.Fatalf("el texto debe ir cifrado: %v", err)
	}
	// hecho / deshacer / borrar solo de quien es
	if ok, _ := a.MarcarItemUtil(1, PersonaDueno, p1.ID, true, ahora); ok {
		t.Fatal("el dueño no puede marcar un pendiente de Ana")
	}
	if ok, _ := a.MarcarItemUtil(1, lidAna, p1.ID, true, ahora); !ok {
		t.Fatal("Ana sí")
	}
	if l := a.ListarItemsUtil(1, pnAna, UtilPendiente, false); len(l) != 0 {
		t.Fatalf("hecho no debe salir sin hechos: %+v", l)
	}
	if ok, _ := a.MarcarItemUtil(1, pnAna, n1.ID, true, ahora); ok {
		t.Fatal("una nota no se marca como hecha")
	}
	if n, _ := a.BorrarHechosUtil(1, pnAna); n != 1 {
		t.Fatalf("hechos borrados: %d", n)
	}
	if ok, _ := a.BorrarItemUtil(1, PersonaDueno, n1.ID); ok {
		t.Fatal("el dueño no borra lo de Ana")
	}
	if ok, _ := a.BorrarItemUtil(1, pnAna, n1.ID); !ok {
		t.Fatal("borrar")
	}
}

func TestUtilTopeLargoYVacio(t *testing.T) {
	a := nuevo(t)
	ahora := time.Now()
	if _, err := a.AgregarItemUtil(1, pnAna, UtilNota, "   ", ahora); err == nil {
		t.Fatal("vacío")
	}
	if _, err := a.AgregarItemUtil(1, pnAna, UtilNota, strings.Repeat("a", MaxTextoUtil+1), ahora); err == nil {
		t.Fatal("largo")
	}
	if _, err := a.AgregarItemUtil(1, pnAna, "otro", "x", ahora); err == nil {
		t.Fatal("tipo")
	}
	_ = a.FijarAjusteBot(1, "util_tope", "2")
	_, _ = a.AgregarItemUtil(1, pnAna, UtilPendiente, "uno", ahora)
	_, _ = a.AgregarItemUtil(1, pnAna, UtilPendiente, "dos", ahora)
	if _, err := a.AgregarItemUtil(1, pnAna, UtilPendiente, "tres", ahora); err != ErrTopeUtil {
		t.Fatalf("tope: %v", err)
	}
	// las notas tienen su propia cuenta
	if _, err := a.AgregarItemUtil(1, pnAna, UtilNota, "una", ahora); err != nil {
		t.Fatal(err)
	}
}

func TestOlvidarYExportarIncluyenUtilidades(t *testing.T) {
	a := nuevo(t)
	llenaPersona(t, a)
	ahora := time.Now()
	_, _ = a.AgregarItemUtil(1, pnAna, UtilNota, "mi nota propia", ahora)
	ex, err := a.DatosDePersona(1, pnAna, ahora)
	if err != nil || len(ex.Utilidades) != 1 {
		t.Fatalf("export: %+v %v", ex, err)
	}
	if _, err := a.OlvidarPersona(1, pnAna); err != nil {
		t.Fatal(err)
	}
	if l := a.ListarItemsUtil(1, pnAna, "", true); len(l) != 0 {
		t.Fatalf("olvidar debía borrarlas: %+v", l)
	}
}
