package estilo

import (
	"embed"
	"io/fs"
)

//go:embed caras/*.png
var carasFS embed.FS

// Caras son los nombres de las 8 caras del avatar que se pueden mandar como imagen.
var Caras = []string{CaraFeliz, CaraContento, CaraGuino, CaraPensando, CaraSorprendido, CaraTriste, CaraEnojado, CaraDormido}

// Cara devuelve el PNG de una cara del avatar (nil si no existe).
func Cara(nombre string) []byte {
	b, err := fs.ReadFile(carasFS, "caras/"+nombre+".png")
	if err != nil {
		return nil
	}
	return b
}
