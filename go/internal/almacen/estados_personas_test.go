package almacen

import (
	"testing"
	"time"
)

func TestEstadosDePersonasLotePorAlias(t *testing.T) {
	a := nuevo(t)
	const pn, lid, otra, sola = "5215500000001@s.whatsapp.net", "99887766@lid", "5215500000002@s.whatsapp.net", "5215500000003@s.whatsapp.net"
	a.AprenderAlias(1, pn, lid)
	ahora := time.Now()
	if m := a.EstadosDePersonas(1, []string{pn, otra}, ahora); len(m) != 0 {
		t.Fatalf("sin nada no sale nadie: %+v", m)
	}
	// el silencio quedó guardado con el LID; la fila de la lista usa el número
	if _, err := a.CrearSilencioGrupo(1, lid, "todos", nil, 0, "yo", "manual", "", false, ahora); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.AgregarAdvertencia(1, otra, "", "spam", "yo", ahora); err != nil {
		t.Fatal(err)
	}
	if err := a.FijarSilencio(1, sola, 0, "", "yo", ahora); err != nil {
		t.Fatal(err)
	}
	m := a.EstadosDePersonas(1, []string{pn, otra, sola, "5215500000009@s.whatsapp.net"}, ahora)
	if !m[pn].Silenciado || m[pn].Advertido || m[pn].Ignorada {
		t.Fatalf("pn: %+v", m[pn])
	}
	if !m[otra].Advertido || m[otra].Silenciado {
		t.Fatalf("otra: %+v", m[otra])
	}
	if !m[sola].Ignorada {
		t.Fatalf("sola: %+v", m[sola])
	}
	if len(m) != 3 {
		t.Fatalf("solo los que tienen algo: %+v", m)
	}
}
