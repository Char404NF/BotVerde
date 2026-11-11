// Package identidad calcula lo que la app muestra de cada persona o grupo: nombre, teléfono con formato y país.
// La app nunca muestra un JID ni un LID.
package identidad

import (
	"strings"
	"unicode"

	"github.com/nyaruka/phonenumbers"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// NumeroNoDisponible es el texto que se muestra cuando solo se conoce el identificador oculto (LID).
const NumeroNoDisponible = "Número no disponible"

// Telefono es un número ya procesado.
type Telefono struct {
	// E164 es el número real tal como lo conoce WhatsApp, con "+" (por ejemplo +5215512345678). Es lo que se guarda.
	E164 string
	// Mostrado es el número con formato internacional para la pantalla (+52 55 1234 5678).
	Mostrado string
	// ISO es el país (MX, US…); vacío si no se pudo determinar.
	ISO string
}

// Digitos deja solo los dígitos 0-9.
func Digitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ParseTelefono procesa los dígitos de un número (con código de país, sin "+").
// Para México, WhatsApp usaba 52 + 1 + 10 dígitos: el "1" móvil se quita solo al MOSTRAR; en E164 se conserva.
func ParseTelefono(dig string) Telefono {
	d := Digitos(dig)
	if d == "" {
		return Telefono{}
	}
	t := Telefono{E164: "+" + d, Mostrado: "+" + d}
	paraFormato := t.E164
	if strings.HasPrefix(d, "521") && len(d) == 13 {
		paraFormato = "+52" + d[3:]
	}
	num, err := phonenumbers.Parse(paraFormato, "")
	if err != nil || !phonenumbers.IsValidNumber(num) {
		return t
	}
	t.Mostrado = phonenumbers.Format(num, phonenumbers.INTERNATIONAL)
	if r := phonenumbers.GetRegionCodeForNumber(num); r != "" && r != "ZZ" {
		t.ISO = r
	}
	return t
}

// Normalizar pasa a minúsculas y quita acentos, para la búsqueda.
func Normalizar(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		out = s
	}
	return strings.ToLower(strings.TrimSpace(out))
}

// ValidarTelefono revisa un teléfono escrito por una persona (con o sin "+", con espacios, guiones o paréntesis) y lo deja en
// formato E.164 (+5215512345678). `region` es el país (MX, US…) que se usa cuando el número no trae código de país. Si la
// librería no reconoce el número pero trae "+" y entre 8 y 15 dígitos, se acepta con un formato razonable.
func ValidarTelefono(texto, region string) (string, bool) {
	t := strings.TrimSpace(texto)
	if t == "" || len(t) > 30 {
		return "", false
	}
	for _, r := range t {
		if !(r >= '0' && r <= '9' || r == '+' || r == ' ' || r == '-' || r == '(' || r == ')' || r == '.') {
			return "", false
		}
	}
	d := Digitos(t)
	if len(d) < 7 || len(d) > 15 {
		return "", false
	}
	if region == "" {
		region = "MX"
	}
	if num, err := phonenumbers.Parse(t, region); err == nil && phonenumbers.IsValidNumber(num) {
		return phonenumbers.Format(num, phonenumbers.E164), true
	}
	if strings.HasPrefix(t, "+") && len(d) >= 8 {
		return "+" + d, true
	}
	return "", false
}
