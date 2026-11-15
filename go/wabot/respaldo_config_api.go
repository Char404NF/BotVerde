package wabot

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"botchat/internal/almacen"
)

// Copia de seguridad de la configuración, cifrada con una contraseña que elige el dueño (la llave del teléfono no sirve en otro
// teléfono). Formato: "BCCFG1" + sal(16) + nonce(12) + AES-256-GCM(JSON). La contraseña nunca se guarda ni se registra.

var cabCopia = []byte("BCCFG1")

const (
	iterCopia   = 210000
	minClaveCop = 8
)

func llaveDeContrasena(pw string, sal []byte) ([]byte, error) {
	return pbkdf2.Key(sha256.New, pw, sal, iterCopia, 32)
}

func sellarCopia(pw string, plano []byte) ([]byte, error) {
	sal := make([]byte, 16)
	nonce := make([]byte, 12)
	if _, err := rand.Read(sal); err != nil {
		return nil, err
	}
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	k, err := llaveDeContrasena(pw, sal)
	if err != nil {
		return nil, err
	}
	b, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(b)
	if err != nil {
		return nil, err
	}
	out := append(append(append([]byte{}, cabCopia...), sal...), nonce...)
	return g.Seal(out, nonce, plano, cabCopia), nil
}

func abrirCopia(pw string, datos []byte) ([]byte, error) {
	if len(datos) < len(cabCopia)+16+12+16 || string(datos[:len(cabCopia)]) != string(cabCopia) {
		return nil, errors.New("ese archivo no es una copia de seguridad de BotChat")
	}
	sal := datos[len(cabCopia) : len(cabCopia)+16]
	nonce := datos[len(cabCopia)+16 : len(cabCopia)+28]
	k, err := llaveDeContrasena(pw, sal)
	if err != nil {
		return nil, err
	}
	b, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(b)
	if err != nil {
		return nil, err
	}
	plano, err := g.Open(nil, nonce, datos[len(cabCopia)+28:], cabCopia)
	if err != nil {
		return nil, errors.New("la contraseña no es la correcta o el archivo está dañado")
	}
	return plano, nil
}

// RespaldoConfigExportar devuelve la copia cifrada de la configuración del bot.
func RespaldoConfigExportar(botID int64, contrasena string) ([]byte, error) {
	if len([]rune(contrasena)) < minClaveCop {
		return nil, errors.New("la contraseña de la copia debe tener al menos 8 caracteres")
	}
	a, err := almacenAbierto()
	if err != nil {
		return nil, err
	}
	g, err := servicioGrupos(botID)
	if err != nil {
		return nil, err
	}
	base, _ := a.LeerCifrado(g.RutaBase())
	r, err := a.ExportarConfig(botID, base, time.Now())
	if err != nil {
		return nil, err
	}
	plano, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	a.RegistrarAccion(botID, almacen.PersonaDueno, "respaldo.exportar", "", "", "ok", "")
	return sellarCopia(contrasena, plano)
}

// RespaldoConfigRestaurar restaura una copia y devuelve (JSON) qué se restauró.
func RespaldoConfigRestaurar(botID int64, contrasena string, datos []byte) (string, error) {
	plano, err := abrirCopia(contrasena, datos)
	if err != nil {
		return "", err
	}
	var r almacen.RespaldoConfig
	if json.Unmarshal(plano, &r) != nil {
		return "", errors.New("la copia está dañada")
	}
	a, err := almacenAbierto()
	if err != nil {
		return "", err
	}
	g, err := servicioGrupos(botID)
	if err != nil {
		return "", err
	}
	res, err := a.RestaurarConfig(botID, &r, func(b []byte) error { return a.EscribirCifrado(g.RutaBase(), b) }, time.Now())
	if err != nil {
		a.RegistrarAccion(botID, almacen.PersonaDueno, "respaldo.restaurar", "", "", "error", "")
		return "", err
	}
	a.RegistrarAccion(botID, almacen.PersonaDueno, "respaldo.restaurar", "", "", "ok", "")
	return js(res, nil)
}
