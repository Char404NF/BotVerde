package identidad

import "strings"

// Datos reúne todo lo que se sabe de un chat para decidir cómo se llama en pantalla.
type Datos struct {
	EsGrupo  bool
	Asunto   string // asunto del grupo
	Guardado string // nombre en la agenda del teléfono
	Publico  string // nombre público del perfil (push name)
	Negocio  string // nombre verificado de un negocio
	Telefono string // teléfono ya con formato internacional; vacío si solo se conoce el LID
}

// NombreMostrado aplica el orden: agenda, nombre público, nombre de negocio, número con formato.
// Para grupos, el asunto. Nunca devuelve un JID ni un LID.
func NombreMostrado(d Datos) string {
	if d.EsGrupo {
		if v := limpio(d.Asunto); v != "" {
			return v
		}
		return "Grupo"
	}
	for _, v := range []string{d.Guardado, d.Publico, d.Negocio, d.Telefono} {
		if v = limpio(v); v != "" {
			return v
		}
	}
	return NumeroNoDisponible
}

// TelefonoMostrado es el texto de teléfono que acompaña al nombre; para un LID sin número es "Número no disponible".
func TelefonoMostrado(esGrupo bool, tel Telefono) string {
	if esGrupo {
		return ""
	}
	if tel.Mostrado == "" {
		return NumeroNoDisponible
	}
	return tel.Mostrado
}

// TextoBusqueda arma el texto normalizado por el que se busca: nombre y número (solo dígitos).
func TextoBusqueda(nombre string, tel Telefono) string {
	return strings.TrimSpace(Normalizar(nombre) + " " + Digitos(tel.E164))
}

func limpio(s string) string {
	s = strings.TrimSpace(s)
	// Quita caracteres de control (por ejemplo saltos de línea) que rompen la lista.
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
}
