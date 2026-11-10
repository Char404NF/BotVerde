package wabot

import (
	"fmt"
	"regexp"
	"strings"

	waLog "go.mau.fi/whatsmeow/util/log"
)

var numeroLargo = regexp.MustCompile(`[0-9]{6,}`)

// enmascaraTexto oculta los números largos (teléfonos e identificadores) de una línea de registro: deja solo los últimos 4 dígitos.
func enmascaraTexto(s string) string {
	return numeroLargo.ReplaceAllStringFunc(s, func(n string) string { return "…" + n[len(n)-4:] })
}

// ruido son avisos de whatsmeow que no son problemas: cierres normales del websocket y contactos repetidos.
func ruido(s string) bool {
	return strings.Contains(s, "failed to read frame header: EOF") ||
		strings.Contains(s, "duplicate contacts found") ||
		strings.Contains(s, "Error sending close to websocket")
}

// registro lleva los mensajes de whatsmeow al Listener sin datos personales: números enmascarados y el ruido conocido descartado (debug).
type registro struct {
	salida func(string)
	mod    string
}

func (r *registro) emite(nivel, msg string, a ...any) {
	if r.salida == nil {
		return
	}
	linea := fmt.Sprintf(msg, a...)
	if ruido(linea) {
		return // nivel debug: no se muestra
	}
	r.salida(fmt.Sprintf("[%s %s] %s", r.mod, nivel, linea))
}
func (r *registro) Warnf(m string, a ...any)  { r.emite("aviso", m, a...) }
func (r *registro) Errorf(m string, a ...any) { r.emite("error", m, a...) }
func (r *registro) Infof(m string, a ...any)  { r.emite("info", m, a...) }
func (r *registro) Debugf(string, ...any)     {}
func (r *registro) Sub(mod string) waLog.Logger {
	return &registro{salida: r.salida, mod: mod}
}
