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
	todas := []Plantilla{
		{Nombre: atencion.Nombre, Descripcion: "Menú de bienvenida con horarios, ubicación y hablar con una persona.", Def: atencion},
		{Nombre: horarios.Nombre, Descripcion: "Contesta de una vez cuando preguntan por el horario o la dirección.", Def: horarios},
		{Nombre: cotizacion.Nombre, Descripcion: "Pide nombre y necesidad, y te avisa con una notificación.", Def: cotizacion},
		{Nombre: "Refacciones", Descripcion: "Cotizar una pieza con un formulario (marca, modelo, año y pieza), horarios y hablar con una persona.", Def: plantillaRefacciones()},
		{Nombre: "Soporte", Descripcion: "Reportar un problema con formulario, consultar un folio y hablar con una persona.", Def: plantillaSoporte()},
		{Nombre: "Citas", Descripcion: "Agendar una cita: nombre, teléfono, fecha y horario, con resumen para confirmar.", Def: plantillaCitas()},
		{Nombre: "Preguntas frecuentes", Descripcion: "Las dudas más comunes con su respuesta; escribir parte del tema también funciona.", Def: plantillaFAQ()},
		{Nombre: "Menú para grupos", Descripcion: "Para grupos: cada persona abre su menú con una mención, con tope de personas a la vez y caducidad corta.", Def: plantillaGrupo()},
	}
	for _, p := range todas {
		p.Def.Normalizar()
	}
	return todas
}

func plantillaRefacciones() *Def {
	return &Def{V: 1, Nombre: "Refacciones", Inicio: "menu",
		Pasos: []Paso{
			{ID: "menu", Tipo: PasoMenu, Titulo: "¡Hola {nombre}! Bienvenido. ¿En qué te ayudo?", Opciones: []Opcion{
				{Texto: "Cotizar una refacción", Destino: "cotiza"}, {Texto: "Horarios y ubicación", Destino: "horario"}, {Texto: "Hablar con una persona", Destino: "humano"}}},
			{ID: "cotiza", Tipo: PasoForm, Titulo: "Con gusto te cotizamos. Te haré unas preguntas cortas.", Texto: "¡Gracias, {marca}! Recibimos tu solicitud. Te enviaremos la cotización en cuanto la tengamos.", MaxReintentos: 3,
				Campos: []Campo{
					{ID: "marca", Etiqueta: "Marca del vehículo", Pregunta: "¿De qué marca es tu vehículo?", Tipo: CampoTexto},
					{ID: "modelo", Etiqueta: "Modelo", Pregunta: "¿Qué modelo es?", Tipo: CampoTexto},
					{ID: "anio", Etiqueta: "Año", Pregunta: "¿De qué año es?", Tipo: CampoNumero, Min: f64(1950), Max: f64(2035), Error: "Escribe el año con 4 números, por ejemplo 2018."},
					{ID: "pieza", Etiqueta: "Pieza", Pregunta: "¿Qué pieza o refacción buscas?", Tipo: CampoTexto},
					{ID: "telefono", Etiqueta: "Teléfono", Pregunta: "¿A qué teléfono te avisamos? (puede ser este mismo)", Tipo: CampoTelefono, Opcional: true},
				}},
			{ID: "horario", Tipo: PasoRespuesta, Texto: "Atendemos de lunes a viernes de 9:00 a 18:00 y sábados de 9:00 a 14:00.\nDirección: escribe aquí tu dirección.\nAhora son las {hora}."},
			{ID: "humano", Tipo: PasoHumano, Texto: "Un momento, una persona te atenderá en cuanto pueda.", TextoFuera: "Ahora estamos fuera de horario ({horario}). Dejé tu aviso y te contestamos en cuanto abramos."},
		},
		Disparadores: Disparadores{PrimerMensaje: true, DiasSinEscribir: 7, Palabras: []string{"refaccion", "refacciones", "pieza", "cotizar"}, Comando: ".menu"},
		Alcance:      Alcance{Privados: true},
		Horario:      Horario{Dias: []int{1, 2, 3, 4, 5, 6}, Desde: "09:00", Hasta: "18:00"}}
}

func plantillaSoporte() *Def {
	return &Def{V: 1, Nombre: "Soporte", Inicio: "menu",
		Pasos: []Paso{
			{ID: "menu", Tipo: PasoMenu, Titulo: "Hola {nombre}, soy el asistente de soporte. ¿Qué necesitas?", Opciones: []Opcion{
				{Texto: "Reportar un problema", Destino: "reporte"}, {Texto: "Consultar mi solicitud", Destino: "folio"}, {Texto: "Hablar con una persona", Destino: "humano"}}},
			{ID: "reporte", Tipo: PasoForm, Titulo: "Lamento el problema. Voy a pedirte algunos datos para atenderte mejor.", Texto: "Listo, {nombre_cliente}. Registramos tu reporte y te contactaremos pronto.", MaxReintentos: 3,
				Campos: []Campo{
					{ID: "nombre_cliente", Etiqueta: "Nombre", Pregunta: "¿Cómo te llamas?", Tipo: CampoTexto},
					{ID: "correo", Etiqueta: "Correo", Pregunta: "¿Cuál es tu correo?", Tipo: CampoCorreo, Opcional: true},
					{ID: "urgencia", Etiqueta: "Urgencia", Pregunta: "¿Qué tan urgente es?", Tipo: CampoOpcion, Opciones: []string{"Baja", "Media", "Alta"}},
					{ID: "problema", Etiqueta: "Problema", Pregunta: "Cuéntame qué pasó.", Tipo: CampoTexto},
				}},
			{ID: "folio", Tipo: PasoPedir, Pregunta: "Escribe tu número de folio.", Variable: "folio", Siguiente: "folio_ok"},
			{ID: "folio_ok", Tipo: PasoRespuesta, Texto: "Anotamos el folio {folio}. Una persona revisará el estado y te responderá.", Notificar: true, Aviso: "Consulta del folio {folio}"},
			{ID: "humano", Tipo: PasoHumano, Texto: "Un momento, una persona de soporte te atenderá.", TextoFuera: "Ahora estamos fuera de horario ({horario}). Tu aviso quedó pendiente y te respondemos al abrir."},
		},
		Disparadores: Disparadores{Palabras: []string{"soporte", "ayuda", "problema", "falla"}, Comando: ".soporte"},
		Alcance:      Alcance{Privados: true}}
}

func plantillaCitas() *Def {
	return &Def{V: 1, Nombre: "Citas", Inicio: "menu",
		Pasos: []Paso{
			{ID: "menu", Tipo: PasoMenu, Titulo: "¡Hola {nombre}! ¿Qué quieres hacer?", Opciones: []Opcion{
				{Texto: "Agendar una cita", Destino: "cita"}, {Texto: "Cambiar o cancelar mi cita", Destino: "humano"}, {Texto: "Horarios", Destino: "horario"}}},
			{ID: "cita", Tipo: PasoForm, Titulo: "Vamos a apartar tu cita.", Texto: "¡Listo! Apartamos tu cita para el {fecha_cita} en la {franja}. Te confirmaremos por este medio.", MaxReintentos: 3,
				Campos: []Campo{
					{ID: "nombre_cita", Etiqueta: "Nombre", Pregunta: "¿A nombre de quién?", Tipo: CampoTexto},
					{ID: "telefono", Etiqueta: "Teléfono", Pregunta: "¿Con qué teléfono te confirmamos?", Tipo: CampoTelefono},
					{ID: "fecha_cita", Etiqueta: "Fecha", Pregunta: "¿Qué día te acomoda? (por ejemplo 25/12/2026 o 25 de diciembre)", Tipo: CampoFecha},
					{ID: "franja", Etiqueta: "Horario", Pregunta: "¿En qué horario?", Tipo: CampoOpcion, Opciones: []string{"Mañana", "Tarde"}},
					{ID: "motivo", Etiqueta: "Motivo", Pregunta: "¿Para qué es la cita?", Tipo: CampoTexto, Opcional: true},
				}},
			{ID: "horario", Tipo: PasoRespuesta, Texto: "Atendemos {horario}. Hoy es {dia} {fecha} y son las {hora}."},
			{ID: "humano", Tipo: PasoHumano, Texto: "Una persona revisará tu cita y te contestará.", TextoFuera: "Estamos fuera de horario ({horario}). Te respondemos al abrir."},
		},
		Disparadores: Disparadores{Palabras: []string{"cita", "citas", "agendar", "reservar"}, Comando: ".citas"},
		Alcance:      Alcance{Privados: true},
		Horario:      Horario{Dias: []int{1, 2, 3, 4, 5}, Desde: "09:00", Hasta: "18:00"}}
}

func plantillaFAQ() *Def {
	return &Def{V: 1, Nombre: "Preguntas frecuentes", Inicio: "menu",
		Pasos: []Paso{
			{ID: "menu", Tipo: PasoMenu, Titulo: "Hola {nombre}. Estas son las dudas más comunes:", Opciones: []Opcion{
				{Texto: "Horarios de atención", Destino: "horarios"}, {Texto: "Formas de pago", Destino: "pagos"}, {Texto: "Envíos y entregas", Destino: "envios"},
				{Texto: "Garantía y devoluciones", Destino: "garantia"}, {Texto: "Hablar con una persona", Destino: "humano"}}},
			{ID: "horarios", Tipo: PasoRespuesta, Texto: "Atendemos {horario}. (Cambia este texto por tu horario.)"},
			{ID: "pagos", Tipo: PasoRespuesta, Texto: "Aceptamos efectivo, transferencia y tarjeta. (Cambia este texto.)"},
			{ID: "envios", Tipo: PasoRespuesta, Texto: "Hacemos envíos en 2 a 5 días hábiles. (Cambia este texto.)"},
			{ID: "garantia", Tipo: PasoRespuesta, Texto: "Todos los productos tienen garantía. Escríbenos si tienes un problema. (Cambia este texto.)"},
			{ID: "humano", Tipo: PasoHumano, Texto: "Un momento, una persona te atenderá.", TextoFuera: "Ahora estamos fuera de horario ({horario}). Te contestamos al abrir."},
		},
		Disparadores: Disparadores{Palabras: []string{"preguntas", "dudas", "faq", "informacion"}, Comando: ".faq"},
		Alcance:      Alcance{Privados: true}}
}

func plantillaGrupo() *Def {
	return &Def{V: 1, Nombre: "Menú para grupos", Inicio: "menu",
		Pasos: []Paso{
			{ID: "menu", Tipo: PasoMenu, Titulo: "Hola {nombre}. ¿Qué necesitas?", Opciones: []Opcion{
				{Texto: "Reglas del grupo", Destino: "reglas"}, {Texto: "Información útil", Destino: "info"},
				{Texto: "Hablar con un administrador", Destino: "humano"}, {Texto: "Ver mis datos", Destino: "privado", Ambito: "privado"}}},
			{ID: "reglas", Tipo: PasoRespuesta, Texto: "Reglas: respeto, sin spam y sin cadenas. (Cambia este texto por las reglas de tu grupo.)"},
			{ID: "info", Tipo: PasoRespuesta, Texto: "Escribe aquí la información que más piden en el grupo."},
			{ID: "privado", Tipo: PasoRespuesta, Texto: "Esta opción se atiende en privado."},
			{ID: "humano", Tipo: PasoHumano, Texto: "Avisé a los administradores; te contestarán en cuanto puedan."},
		},
		Disparadores: Disparadores{Mencion: true, Comando: "menu"},
		Alcance:      Alcance{TodosGrupos: true},
		Ajustes:      &AjustesMenu{MaxSesionesGrupo: 5, ExpiraGrupoMin: 10}}
}

func f64(v float64) *float64 { return &v }
