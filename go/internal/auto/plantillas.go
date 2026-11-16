package auto

import (
	"time"

	"botchat/internal/almacen"
)

// Plantilla es un mensaje listo para usar como punto de partida.
type Plantilla struct {
	Tipo        string `json:"tipo"`
	Nombre      string `json:"nombre"`
	Descripcion string `json:"descripcion"`
	Texto       string `json:"texto"`
	Config      Config `json:"config"`
}

// Plantillas devuelve las plantillas listas, por tipo. `hoy` sirve para proponer fechas de vacaciones.
func Plantillas(hoy time.Time) []Plantilla {
	c := func(t string) Config { return ConfigPorDefecto(t) }
	bien := c(almacen.AutoBienvenida)
	bienSil := bien
	bienSil.SilencioDias = 14
	ausH := c(almacen.AutoAusencia)
	vac := c(almacen.AutoAusencia)
	vac.Cuando = Cuando{Vacaciones: []Vacacion{{Nombre: "Vacaciones", Desde: hoy.AddDate(0, 0, 7).Format("2006-01-02"), Hasta: hoy.AddDate(0, 0, 14).Format("2006-01-02")}}}
	vac.Repeticion = Repeticion{Cada: 24, Unidad: "horas"}
	fest := c(almacen.AutoAusencia)
	fest.Cuando = Cuando{Fechas: []FechaAnual{{Nombre: "Navidad", Dia: 25, Mes: 12}, {Nombre: "Año nuevo", Dia: 1, Mes: 1}}}
	fest.Repeticion = Repeticion{Cada: 24, Unidad: "horas"}
	return []Plantilla{
		{almacen.AutoBienvenida, "Bienvenida amable", "Saluda a quien escribe por primera vez.", "¡Hola {nombre}! 👋 Gracias por escribirnos. En un momento te atendemos.\n\nNuestro horario: {horario}.", bien},
		{almacen.AutoBienvenida, "Bienvenida y regreso", "También saluda a quien vuelve tras 14 días sin escribir.", "¡Hola {nombre}! Qué gusto saludarte. ¿En qué te podemos ayudar?", bienSil},
		{almacen.AutoAusencia, "Fuera de horario", "Avisa cuando escriben fuera de tu horario.", "Hola {nombre}, ahora estamos fuera de horario. Atendemos {horario}.\n\nTe respondemos en cuanto abramos. ¡Gracias por tu paciencia!", ausH},
		{almacen.AutoAusencia, "Vacaciones", "Avisa durante un rango de fechas.", "Hola {nombre}, estamos de vacaciones y no podemos atenderte por ahora. Volvemos pronto y te respondemos en cuanto regresemos. ¡Gracias!", vac},
		{almacen.AutoAusencia, "Días festivos", "Avisa en fechas que se repiten cada año.", "Hola {nombre}, hoy ({fecha}) es día festivo y no tenemos atención. Te respondemos el siguiente día hábil. ¡Felices fiestas! 🎉", fest},
		{almacen.AutoRecibido, "Ya vi tu mensaje", "Avisa que recibiste el mensaje, una vez por conversación.", "Recibí tu mensaje, {nombre}. Te respondo en cuanto pueda. 🙌", c(almacen.AutoRecibido)},
		{almacen.AutoCierre, "Gracias por contactarnos", "Se manda al terminar un menú.", "Gracias por contactarnos, {nombre}. Si necesitas algo más, escribe *!menu*.", c(almacen.AutoCierre)},
		{almacen.AutoAtajo, "/gracias", "Una respuesta rápida de agradecimiento.", "¡Gracias por tu mensaje, {nombre}! Quedamos al pendiente.", Config{}},
		{almacen.AutoAtajo, "/horario", "Dice tu horario de atención.", "Atendemos {horario}.", Config{}},
		{almacen.AutoAtajo, "/ubicacion", "Dice dónde estás.", "Estamos en: escribe aquí tu dirección.", Config{}},
	}
}

// NombreSugerido es el nombre de atajo que trae una plantilla de atajo (sin la barra).
func NombreSugerido(p Plantilla) string {
	if p.Tipo == almacen.AutoAtajo && len(p.Nombre) > 1 {
		return p.Nombre[1:]
	}
	return p.Nombre
}
