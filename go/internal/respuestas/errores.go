package respuestas

import (
	"errors"
	"strings"
)

var (
	errNoExiste = errors.New("esa respuesta no existe")
	errLargo    = errors.New("el texto es muy largo (máximo 600 letras)")
)

type errVar struct {
	v    string
	vars []string
}

func (e *errVar) Error() string {
	if len(e.vars) == 0 {
		return "esta respuesta no usa variables: quita {" + e.v + "}"
	}
	return "{" + e.v + "} no existe aquí; puedes usar: {" + strings.Join(e.vars, "}, {") + "}"
}
