package wabot

import "strings"

// formatoCodigo pone el código de 8 caracteres como ABCD-EFGH.
func formatoCodigo(c string) string {
	c = strings.ToUpper(strings.TrimSpace(c))
	if len(c) == 8 {
		return c[:4] + "-" + c[4:]
	}
	return c
}

// soloDigitos deja únicamente los dígitos 0-9.
func soloDigitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// enmascara deja solo los últimos 4 caracteres de un identificador para el registro.
func enmascara(s string) string {
	if i := strings.IndexByte(s, '@'); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexByte(s, ':'); i >= 0 {
		s = s[:i]
	}
	if len(s) <= 4 {
		return "…" + s
	}
	return "…" + s[len(s)-4:]
}
