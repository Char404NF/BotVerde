// Package estilo da a los mensajes del bot una misma cara: un título en negrita con su emoji, líneas cortas con viñetas y un pie con una
// ayuda. Se usa en la ayuda, los menús de grupo y los comandos de todos los días para que todo se vea de una misma marca.
package estilo

import "strings"

// Línea fina que separa la cabecera del cuerpo (se ve bien en WhatsApp con cualquier tema).
const Raya = "━━━━━━━━━━━━"

// Caras del avatar (nombres de los archivos de assets/animos) según lo que pasa.
const (
	CaraFeliz       = "feliz"       // saludo y bienvenida
	CaraContento    = "contento"    // terminó una tarea
	CaraGuino       = "guino"       // chistes y juegos
	CaraPensando    = "pensando"    // descargando, haciendo un sticker o consultando a la IA
	CaraSorprendido = "sorprendido" // ganó un sorteo o salió un resultado
	CaraTriste      = "triste"      // hubo un error
	CaraEnojado     = "enojado"     // el filtro borró algo
	CaraDormido     = "dormido"     // no molestar o fuera de horario
)

// Tarjeta arma el mensaje: «🤖 *Título*», una raya, las líneas y, si hay, un pie en cursiva con 💡.
func Tarjeta(emoji, titulo string, lineas []string, pie string) string {
	var b strings.Builder
	if emoji != "" {
		b.WriteString(emoji + " ")
	}
	b.WriteString("*" + strings.TrimSpace(titulo) + "*\n" + Raya)
	for _, l := range lineas {
		b.WriteString("\n" + l)
	}
	if strings.TrimSpace(pie) != "" {
		b.WriteString("\n\n💡 _" + strings.TrimSpace(pie) + "_")
	}
	return b.String()
}

// Pie es la ayuda corta del final de las respuestas.
func Pie(prefijo string) string {
	return "Escribe " + prefijo + "ayuda para ver todo lo que puedo hacer"
}
