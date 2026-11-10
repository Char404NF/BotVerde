package cifrado

import (
	"crypto/rand"
	"errors"
	"io"
)

const versionTexto = 1

// SellarTexto cifra un valor de columna con AES-256-GCM.
// Formato: versión(1) || nonce(12) || cifrado || etiqueta(16). El AAD es el nombre de la columna.
// Un valor vacío se guarda como NULL (nil) para que se pueda distinguir de un texto presente.
func SellarTexto(clave []byte, columna string, plano []byte) ([]byte, error) {
	if len(plano) == 0 {
		return nil, nil
	}
	a, err := nuevoAEAD(clave)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	out := make([]byte, 0, 1+len(nonce)+len(plano)+TamEtiqueta)
	out = append(out, versionTexto)
	out = append(out, nonce...)
	return a.Seal(out, nonce, plano, []byte(columna)), nil
}

// AbrirTexto descifra un valor guardado con SellarTexto. nil devuelve nil.
func AbrirTexto(clave []byte, columna string, sellado []byte) ([]byte, error) {
	if len(sellado) == 0 {
		return nil, nil
	}
	a, err := nuevoAEAD(clave)
	if err != nil {
		return nil, err
	}
	if sellado[0] != versionTexto || len(sellado) < 1+a.NonceSize()+TamEtiqueta {
		return nil, ErrFormato
	}
	nonce := sellado[1 : 1+a.NonceSize()]
	pt, err := a.Open(nil, nonce, sellado[1+a.NonceSize():], []byte(columna))
	if err != nil {
		return nil, errors.Join(ErrAutenticacion)
	}
	return pt, nil
}
