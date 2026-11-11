package auto

import (
	"strconv"
	"time"

	"botchat/internal/almacen"
)

// Escenario es la persona y el momento falsos con los que se prueba un mensaje en el simulador.
type Escenario struct {
	Nombre   string `json:"nombre"`   // cómo se llama quien escribe
	EsGrupo  bool   `json:"esGrupo"`  // escribe en un grupo
	Conocida bool   `json:"conocida"` // es un contacto
	Primera  bool   `json:"primera"`  // es la primera vez que escribe (para la bienvenida)
	Silencio int    `json:"silencio"` // días que lleva sin escribir (para la bienvenida)
	Cuando   string `json:"cuando"`   // "" = ahora; "2006-01-02T15:04" = ese momento en la zona del mensaje
	Persona  string `json:"persona"`  // JID falso para las listas "excepto" y "solo" (opcional)
}

// ArchivoPrueba es un archivo que se habría mandado.
type ArchivoPrueba struct {
	Tipo   string `json:"tipo"`
	Nombre string `json:"nombre"`
}

// Prueba es lo que habría pasado con ese mensaje: no se manda nada a WhatsApp ni se guarda nada.
type Prueba struct {
	Aplica   bool            `json:"aplica"`
	Motivo   string          `json:"motivo"` // si no aplica, por qué
	Texto    string          `json:"texto"`
	Archivos []ArchivoPrueba `json:"archivos"`
	Cuando   string          `json:"cuando"` // el momento que se usó, para mostrarlo
}

// Probar evalúa un mensaje (sin guardarlo) contra un escenario falso. Usa la misma lógica que el envío real, pero no mira la repetición
// ni el tope por hora ni escribe nada.
func (d *Decisor) Probar(m almacen.AutoMensaje, c Config, esc Escenario) Prueba {
	zona := c.Cuando.ZonaDe(d.zonaDefecto())
	t := d.c.Ahora().In(zona)
	if esc.Cuando != "" {
		if x, err := time.ParseInLocation("2006-01-02T15:04", esc.Cuando, zona); err == nil {
			t = x
		}
	}
	p := Prueba{Cuando: t.Format("02/01/2006 15:04"), Archivos: []ArchivoPrueba{}}
	persona := esc.Persona
	if persona == "" {
		persona = "prueba@s.whatsapp.net"
	}
	chat := persona
	if esc.EsGrupo {
		chat = "prueba@g.us"
	}
	if m.Tipo != almacen.AutoAtajo {
		if !c.Quien.Aplica(Persona{Clave: persona, Chat: chat, EsGrupo: esc.EsGrupo, Canon: d.c.Persona, Conocida: func() bool { return esc.Conocida }}) {
			p.Motivo = "No es para esta persona (revisa «A quién»)."
			return p
		}
		if !c.Cuando.Activo(t, d.zonaDefecto()) {
			p.Motivo = "En ese momento no se manda (revisa «Cuándo»)."
			return p
		}
		if m.Tipo == almacen.AutoBienvenida && !esc.Primera && !(c.SilencioDias > 0 && esc.Silencio >= c.SilencioDias) {
			p.Motivo = "Esta persona ya había escrito: la bienvenida es para la primera vez" + silencioTxt(c.SilencioDias) + "."
			return p
		}
	}
	p.Aplica = true
	nombre := esc.Nombre
	p.Texto = d.renderCon(m.Texto, c, func() string { return nombre }, t)
	for i, a := range m.Adjuntos {
		if i >= almacen.MaxAdjuntos {
			break
		}
		p.Archivos = append(p.Archivos, ArchivoPrueba{Tipo: a.Tipo, Nombre: a.Nombre})
	}
	return p
}

func silencioTxt(dias int) string {
	if dias <= 0 {
		return ""
	}
	return " o cuando pasan " + strconv.Itoa(dias) + " días sin escribir"
}
