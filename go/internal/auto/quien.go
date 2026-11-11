package auto

import "strings"

// Persona es lo que se sabe de quien escribió, para decidir si un mensaje automático le toca.
type Persona struct {
	Clave    string // persona canónica (el mismo número aunque escriba con su LID)
	Chat     string // el chat donde escribió
	EsGrupo  bool
	Conocida func() bool // es un contacto (agenda, rango o aprobada); se calcula solo si hace falta
	Canon    func(string) string
}

func (p Persona) canon(x string) string {
	x = strings.TrimSpace(x)
	if x == "" {
		return ""
	}
	if !strings.Contains(x, "@") {
		x += "@s.whatsapp.net"
	}
	if p.Canon != nil {
		return p.Canon(x)
	}
	return x
}

func (p Persona) enLista(l []string) bool {
	for _, x := range l {
		if x == p.Chat || x == p.Clave || p.canon(x) == p.Clave {
			return true
		}
	}
	return false
}

// Aplica dice si el mensaje le toca a esa persona.
func (q Quien) Aplica(p Persona) bool {
	if p.EsGrupo {
		if !q.Grupos {
			return false
		}
		if len(q.GruposElegidos) > 0 {
			ok := false
			for _, g := range q.GruposElegidos {
				if g == p.Chat {
					ok = true
				}
			}
			if !ok {
				return false
			}
		}
	} else if !q.Privados {
		return false
	}
	if len(q.Excepto) > 0 && p.enLista(q.Excepto) {
		return false
	}
	if len(q.SoloLista) > 0 && !p.enLista(q.SoloLista) {
		return false
	}
	switch q.Personas {
	case "desconocidos":
		return p.Conocida == nil || !p.Conocida()
	case "contactos":
		return p.Conocida != nil && p.Conocida()
	}
	return true
}
