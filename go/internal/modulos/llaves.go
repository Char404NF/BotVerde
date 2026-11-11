package modulos

import (
	"fmt"
	"sync"
)

// Llaves de los módulos (H6/H7): la app las guarda cifradas con el Android Keystore y se las pasa a Go SOLO en memoria. Nunca van a la
// configuración, a la base, al registro ni a los logs.
var (
	llavesMu sync.RWMutex
	llavesEn = map[string]string{}
)

func claveLlave(botID int64, modulo, clave string) string {
	return fmt.Sprintf("%d|%s|%s", botID, modulo, clave)
}

// FijarLlave guarda una llave en memoria (valor "" = olvidarla).
func FijarLlave(botID int64, modulo, clave, valor string) {
	llavesMu.Lock()
	defer llavesMu.Unlock()
	if valor == "" {
		delete(llavesEn, claveLlave(botID, modulo, clave))
		return
	}
	llavesEn[claveLlave(botID, modulo, clave)] = valor
}

// TieneLlave dice si hay una llave en memoria (sin devolverla).
func TieneLlave(botID int64, modulo, clave string) bool {
	llavesMu.RLock()
	defer llavesMu.RUnlock()
	_, ok := llavesEn[claveLlave(botID, modulo, clave)]
	return ok
}

// Llave devuelve la llave en memoria ("" si no hay).
func (g *Gestor) Llave(modulo, clave string) string {
	llavesMu.RLock()
	defer llavesMu.RUnlock()
	return llavesEn[claveLlave(g.BotID, modulo, clave)]
}

// CampoLlaveValido dice si el módulo declara ese campo como secreto (para no guardar llaves que nadie va a usar).
func CampoLlaveValido(modulo, clave string) bool {
	m, ok := Obtener(modulo)
	if !ok {
		return false
	}
	for _, c := range m.Esquema() {
		if c.Clave == clave && c.Tipo == CampoSecreto {
			return true
		}
	}
	return false
}
