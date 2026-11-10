package flujo

// SaludoJSON es la automatización de fábrica: contesta "hola" (citando el mensaje) a quien escriba en un chat privado,
// como máximo una vez por minuto por persona. Tiene la prioridad más baja: si coincide cualquier otra automatización, no responde.
func SaludoJSON() []byte {
	d := &Def{V: 1, Nombre: "Saludo", Inicio: "s1",
		Pasos:        []Paso{{ID: "s1", Tipo: PasoRespuesta, Texto: "hola"}},
		Disparadores: Disparadores{Cualquiera: true, EnfriamientoSeg: 60},
		Alcance:      Alcance{Privados: true}}
	return d.JSON()
}

// Plantilla es un punto de partida para armar una automatización.
type Plantilla struct {
	Nombre      string `json:"nombre"`
	Descripcion string `json:"descripcion"`
	Def         *Def   `json:"def"`
}

// Plantillas devuelve las plantillas de arranque.
func Plantillas() []Plantilla {
	atencion := &Def{V: 1, Nombre: "Atención básica", Inicio: "menu",
		Pasos: []Paso{
			{ID: "menu", Tipo: PasoMenu, Titulo: "¡Hola {nombre}! Soy el asistente automático. ¿En qué te ayudo?", Opciones: []Opcion{
				{Texto: "Horarios", Destino: "horarios"}, {Texto: "Ubicación", Destino: "ubicacion"}, {Texto: "Hablar con una persona", Destino: "humano"}}},
			{ID: "horarios", Tipo: PasoRespuesta, Texto: "Atendemos de lunes a viernes de 9:00 a 18:00. (Cambia este texto por tu horario.)"},
			{ID: "ubicacion", Tipo: PasoRespuesta, Texto: "Estamos en: escribe aquí tu dirección."},
			{ID: "humano", Tipo: PasoHumano, Texto: "Un momento, una persona te atenderá en cuanto pueda."},
		},
		Disparadores: Disparadores{PrimerMensaje: true, DiasSinEscribir: 7, Palabras: []string{"menu", "ayuda"}, Comando: ".menu"},
		Alcance:      Alcance{Privados: true}}
	horarios := &Def{V: 1, Nombre: "Horarios y ubicación", Inicio: "r",
		Pasos: []Paso{
			{ID: "r", Tipo: PasoRespuesta, Texto: "Horario: lunes a viernes de 9:00 a 18:00.\nUbicación: escribe aquí tu dirección.\nSon las {hora} del {dia}."},
		},
		Disparadores: Disparadores{Palabras: []string{"horario", "horarios", "ubicacion", "direccion", "donde estan"}, Comando: ".horarios"},
		Alcance:      Alcance{Privados: true}}
	cotizacion := &Def{V: 1, Nombre: "Cotización", Inicio: "nombre",
		Pasos: []Paso{
			{ID: "nombre", Tipo: PasoPedir, Pregunta: "¡Con gusto te cotizamos! ¿Cómo te llamas?", Variable: "dato1", Siguiente: "necesidad"},
			{ID: "necesidad", Tipo: PasoPedir, Pregunta: "Mucho gusto, {dato1}. ¿Qué necesitas cotizar?", Variable: "dato2", Siguiente: "gracias"},
			{ID: "gracias", Tipo: PasoRespuesta, Texto: "Gracias, {dato1}. Recibimos tu solicitud y te contactaremos pronto.", Notificar: true, Aviso: "Cotización de {dato1}: {dato2}"},
		},
		Disparadores: Disparadores{Palabras: []string{"cotizacion", "cotizar", "precio", "precios"}, Comando: ".cotizar"},
		Alcance:      Alcance{Privados: true}}
	for _, p := range []*Def{atencion, horarios, cotizacion} {
		p.Normalizar()
	}
	return []Plantilla{
		{Nombre: atencion.Nombre, Descripcion: "Menú de bienvenida con horarios, ubicación y hablar con una persona.", Def: atencion},
		{Nombre: horarios.Nombre, Descripcion: "Contesta de una vez cuando preguntan por el horario o la dirección.", Def: horarios},
		{Nombre: cotizacion.Nombre, Descripcion: "Pide nombre y necesidad, y te avisa con una notificación.", Def: cotizacion},
	}
}
