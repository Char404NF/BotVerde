package grupos

import "strings"

// Enlaces de contenido adulto y sitios peligrosos. Se detectan por la terminación del dominio (.xxx, .porn…), por palabras dentro del
// nombre (pornhub, xvideos…) y por una lista corta de sitios conocidos. La persona puede sumar los suyos en "Bloqueados".

var terminacionesAdultas = []string{".xxx", ".porn", ".sex", ".adult", ".sexy", ".cam"}

var palabrasEnDominioAdulto = []string{
	"porn", "xvideos", "xnxx", "xhamster", "redtube", "youporn", "brazzers", "onlyfans", "hentai", "rule34", "nhentai", "chaturbate",
	"stripchat", "bongacams", "livejasmin", "cam4", "fansly", "spankbang", "eporner", "tnaflix", "motherless", "e-hentai", "pornhub",
	"sexo", "sexy", "nsfw", "nudes", "tetas", "jav",
}

// Acortadores: esconden a dónde lleva el enlace. Solo se bloquean si la persona activa "Bloquear acortadores".
var acortadores = []string{"bit.ly", "tinyurl.com", "cutt.ly", "t.co", "goo.gl", "ow.ly", "is.gd", "rebrand.ly", "shorturl.at", "tiny.cc", "rb.gy", "s.id"}

// DominioAdulto dice si el dominio es de contenido adulto.
func DominioAdulto(d string) bool {
	d = strings.ToLower(d)
	for _, t := range terminacionesAdultas {
		if strings.HasSuffix(d, t) {
			return true
		}
	}
	for _, p := range palabrasEnDominioAdulto {
		if strings.Contains(d, p) {
			return true
		}
	}
	return false
}

// DominioAcortador dice si el dominio es un acortador de enlaces.
func DominioAcortador(d string) bool { return dominioPermitido(strings.ToLower(d), acortadores) }
