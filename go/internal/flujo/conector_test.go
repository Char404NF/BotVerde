package flujo

import "testing"

// {api_<nombre>} en menús, reglas y comandos propios pide el texto a un conector HTTP; si no hay conector o falla, queda vacío.
func TestVariableDeConector(t *testing.T) {
	m := Nuevo(Config{Nombre: func(chat, sender string) string { return "Ana" },
		Conector: func(nombre, sender, chat string) string {
			if nombre == "dolar" && sender == "s1" && chat == "c1" {
				return "18.20"
			}
			return ""
		}})
	e := entrada{Chat: "c1", Sender: "s1"}
	if got := m.sustituye(nil, "Hola {nombre}: el dólar está en {api_dolar}.", e, nil); got != "Hola Ana: el dólar está en 18.20." {
		t.Fatalf("texto: %q", got)
	}
	if got := m.sustituye(nil, "{API_dolar} / {api_otro}", e, nil); got != "18.20 / " {
		t.Fatalf("mayúsculas y desconocidos: %q", got)
	}
	sin := Nuevo(Config{})
	if got := sin.sustituye(nil, "x{api_dolar}y", e, nil); got != "xy" {
		t.Fatalf("sin conectores queda vacío: %q", got)
	}
	// un dato pedido con ese nombre manda sobre el conector
	if got := m.sustituye(nil, "{api_dolar}", e, map[string]string{"api_dolar": "dato"}); got != "dato" {
		t.Fatalf("los datos del formulario mandan: %q", got)
	}
}

// {ia_<dato>} responde con IA a lo que la persona escribió en ese paso; sin dato o sin módulo queda vacío.
func TestVariableDeIA(t *testing.T) {
	m := Nuevo(Config{IA: func(q, sender, chat string) string { return "IA(" + q + ")" }})
	e := entrada{Chat: "c1", Sender: "s1"}
	if got := m.sustituye(nil, "R: {ia_dato1}", e, map[string]string{"dato1": " ¿hora? "}); got != "R: IA(¿hora?)" {
		t.Fatalf("texto: %q", got)
	}
	if got := m.sustituye(nil, "R: {ia_dato2}", e, map[string]string{"dato1": "x"}); got != "R: " {
		t.Fatalf("sin dato: %q", got)
	}
	sin := Nuevo(Config{})
	if got := sin.sustituye(nil, "R: {ia_dato1}", e, map[string]string{"dato1": "x"}); got != "R: " {
		t.Fatalf("sin módulo: %q", got)
	}
}
