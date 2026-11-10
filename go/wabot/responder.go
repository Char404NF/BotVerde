package wabot

import (
	"strings"
	"sync"
	"time"
)

const (
	// Un contacto recibe como máximo una respuesta cada tanto.
	pausaPorContacto = 60 * time.Second
	// Los mensajes más viejos que esto (acumulados mientras el bot estaba apagado) no se responden.
	maxAntiguedad = 2 * time.Minute
	// Servidores de WhatsApp que representan a una persona (número normal o identificador oculto).
	servidorPersona    = "s.whatsapp.net"
	servidorPersonaLID = "lid"
	textoRespuesta     = "hola"
)

// entrada resume solo lo necesario de un mensaje recibido para decidir si se responde.
type entrada struct {
	EsMio      bool
	EsGrupo    bool
	Edicion    bool
	Servidor   string // servidor del chat (JID.Server)
	Antiguedad time.Duration
	Texto      string
	Clave      string // identifica al contacto (JID del chat)
}

type limitador struct {
	mu     sync.Mutex
	ultimo map[string]time.Time
}

func nuevoLimitador() *limitador { return &limitador{ultimo: map[string]time.Time{}} }

// decidir responde si hay que contestar y, si no, el motivo (para el registro).
// Si contesta, anota la hora para respetar la pausa por contacto.
func (l *limitador) decidir(e entrada, ahora time.Time) (bool, string) {
	switch {
	case e.EsMio:
		return false, "mensaje propio"
	case e.EsGrupo:
		return false, "grupo"
	case e.Servidor != servidorPersona && e.Servidor != servidorPersonaLID:
		return false, "no es un chat privado"
	case e.Edicion:
		return false, "mensaje editado"
	case e.Antiguedad > maxAntiguedad:
		return false, "mensaje viejo"
	case strings.TrimSpace(e.Texto) == "":
		return false, "no es texto"
	case e.Clave == "":
		return false, "sin contacto"
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if t, ok := l.ultimo[e.Clave]; ok && ahora.Sub(t) < pausaPorContacto {
		return false, "ya se respondió hace menos de 60 s"
	}
	if len(l.ultimo) > 2000 { // limpieza simple para que el mapa no crezca sin fin
		for k, t := range l.ultimo {
			if ahora.Sub(t) >= pausaPorContacto {
				delete(l.ultimo, k)
			}
		}
	}
	l.ultimo[e.Clave] = ahora
	return true, ""
}

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
