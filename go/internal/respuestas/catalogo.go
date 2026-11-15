package respuestas

func init() {
	const (
		gGen   = "Comandos en general"
		gRang  = "Rangos"
		gPers  = "Personas"
		gChat  = "Chats"
		gGrup  = "Grupos"
		gFilt  = "Filtros del grupo"
		gSil   = "Silencios"
		gUtil  = "Notas y pendientes"
		gRepor = "Reportes"
	)
	Registrar(
		// ---- en general
		Def{"cmd.desconocido", gGen, "El dueño escribe un comando que no existe (a las demás personas no se les contesta nada, para no llenar el chat).", "No conozco ese comando. Escribe {prefijo}ayuda para ver los que puedes usar.", []string{"prefijo"}},
		Def{"cmd.sin_permiso", gGen, "Usan un comando que su rango no permite.", "No tienes permiso para usar ese comando.", nil},
		Def{"cmd.solo_privado", gGen, "Usan en un grupo un comando que solo funciona en privado.", "Ese comando solo funciona en un chat privado conmigo.", nil},
		Def{"cmd.solo_grupo", gGen, "Usan en privado un comando que solo funciona en un grupo.", "Ese comando solo funciona dentro de un grupo.", nil},
		Def{"cmd.muy_rapido", gGen, "Alguien manda demasiados comandos en un minuto.", "Vas muy rápido. Espera un momento y vuelve a intentar.", nil},
		Def{"cmd.solo_dueno", gGen, "Alguien usa un comando que solo es del dueño.", "Ese comando solo lo usa el dueño desde “Mensajes guardados” o desde la app.", nil},
		Def{"cmd.confirma_dueno", gGen, "El dueño usa un comando que pide confirmar con código.", "Esto necesita confirmación. Te llegó un código en una notificación y en la app: escribe {prefijo}confirmar y el código.", []string{"prefijo"}},
		Def{"cmd.confirma_otro", gGen, "Otra persona usa un comando que pide confirmar con código.", "Esto necesita confirmación. El dueño recibe un código en su teléfono; cuando te lo dé escribe {prefijo}confirmar y el código.", []string{"prefijo"}},
		Def{"cmd.confirmacion_falla", gGen, "No se pudo preparar el código de confirmación.", "No pude preparar la confirmación. Intenta de nuevo.", nil},
		Def{"cmd.ayuda_titulo", gGen, "Primera línea de la lista de comandos (!ayuda).", "Esto puedes usar{rango}:", []string{"rango"}},
		// ---- rangos
		Def{"cmd.eres_dueno", gRang, "El dueño pregunta su rango (!rango).", "Eres el Dueño: puedes todo.", nil},
		Def{"cmd.sin_rango", gRang, "Alguien sin rango pregunta su rango.", "No tienes un rango. Escribe {prefijo}ayuda para ver lo que puedes usar.", []string{"prefijo"}},
		Def{"cmd.tu_rango", gRang, "Alguien con rango pregunta cuál es.", "Tu rango es {rango}{vence}.", []string{"rango", "vence"}},
		Def{"cmd.rango_dado", gRang, "Se le da un rango a alguien con !asignar.", "Listo: ahora tiene el rango {rango}{duracion}.", []string{"rango", "duracion"}},
		Def{"cmd.rango_quitado", gRang, "Se le quita el rango a alguien.", "Listo: ya no tiene rango.", nil},
		// ---- personas
		Def{"pers.ignorar", gPers, "Se usa !ignorar: el bot deja de contestarle.", "Listo: el bot no le contestará {cuando}.", []string{"cuando"}},
		Def{"pers.designorar", gPers, "Se usa !designorar.", "Listo: el bot vuelve a contestarle.", nil},
		Def{"pers.bloquear", gPers, "Se bloquea a alguien en el bot.", "Listo: el bot la ignora por completo (no es un bloqueo de WhatsApp).", nil},
		Def{"pers.desbloquear", gPers, "Se desbloquea a alguien en el bot.", "Listo: el bot vuelve a atenderla.", nil},
		Def{"pers.adv_quitada", gPers, "Se le quita una advertencia.", "Listo: le quedan {n} advertencia(s).", []string{"n"}},
		Def{"pers.adv_borradas", gPers, "Se borra el historial de advertencias.", "Listo: borré {n} advertencia(s).", []string{"n"}},
		Def{"pers.olvidar", gPers, "Se borra todo lo guardado de una persona.", "Listo: olvidé a esa persona ({n} mensajes borrados).", []string{"n"}},
		// ---- chats
		Def{"chat.pausar", gChat, "Se usa !pausar en un chat.", "Listo: el bot calla aquí {duracion}. Escribe {prefijo}reanudar para volver.", []string{"duracion", "prefijo"}},
		Def{"chat.control", gChat, "Alguien toma el control de un chat.", "Listo: tomaste el control, el bot calla aquí. Escribe {prefijo}reanudar para devolvérselo.", []string{"prefijo"}},
		Def{"chat.reanudar", gChat, "Se usa !reanudar.", "Listo: el bot vuelve a contestar aquí.", nil},
		Def{"chat.pendiente", gChat, "Se marca un chat como pendiente.", "Listo: chat marcado como pendiente.", nil},
		Def{"chat.resuelto", gChat, "Se marca un chat como resuelto.", "Listo: chat marcado como resuelto.", nil},
		Def{"chat.etiqueta_puesta", gChat, "Se pone una etiqueta a un chat.", "Listo: etiqueta {etiqueta} puesta.", []string{"etiqueta"}},
		Def{"chat.etiqueta_quitada", gChat, "Se quita una etiqueta de un chat.", "Listo: quité la etiqueta {etiqueta}.", []string{"etiqueta"}},
		// ---- reportes
		Def{"reportar.ok", gRepor, "Alguien reporta un mensaje con !reportar (si «Contestar gracias» está encendido).", "Gracias por tu reporte, lo analizaremos. Gracias.", nil},
		Def{"reportar.no_propio", gRepor, "Alguien intenta reportar un mensaje suyo.", "No puedes reportar un mensaje tuyo: responde al mensaje de otra persona", nil},
		Def{"reportar.no_bot", gRepor, "Alguien intenta reportar un mensaje del bot.", "No puedes reportar un mensaje del bot: responde al mensaje de una persona", nil},
		// ---- grupos
		Def{"grupo.cerrado", gGrup, "Se cierra el grupo (!cerrar).", "Listo: el grupo quedó cerrado, solo los administradores pueden escribir.", nil},
		Def{"grupo.abierto", gGrup, "Se abre el grupo (!abrir).", "Listo: el grupo quedó abierto, todos pueden escribir.", nil},
		Def{"grupo.info_bloqueada", gGrup, "Se bloquea la información del grupo.", "Listo: solo los administradores pueden cambiar la información del grupo.", nil},
		Def{"grupo.info_libre", gGrup, "Se libera la información del grupo.", "Listo: todos pueden cambiar la información del grupo.", nil},
		Def{"grupo.todos", gGrup, "Se menciona a todos con !todos (cuando se pidió desde un chat privado).", "Listo: mencioné a {n} personas.", []string{"n"}},
		Def{"grupo.expulsada", gGrup, "Se expulsa a alguien (!expulsar).", "Listo: la persona salió del grupo.", nil},
		Def{"grupo.ascendida", gGrup, "Se hace administradora a alguien.", "Listo: ahora es administradora del grupo.", nil},
		Def{"grupo.bajada", gGrup, "Se le quita el cargo de administradora.", "Listo: ya no es administradora del grupo.", nil},
		Def{"grupo.aprobadas", gGrup, "Se aprueban solicitudes de ingreso.", "Listo: aprobé {n} solicitudes.", []string{"n"}},
		Def{"grupo.rechazadas", gGrup, "Se rechazan solicitudes de ingreso.", "Listo: rechacé {n} solicitudes.", []string{"n"}},
		Def{"grupo.nombre", gGrup, "Se cambia el nombre del grupo.", "Listo: cambié el nombre del grupo.", nil},
		Def{"grupo.descripcion", gGrup, "Se cambia la descripción del grupo.", "Listo: cambié la descripción del grupo.", nil},
		Def{"grupo.descripcion_borrada", gGrup, "Se borra la descripción del grupo.", "Listo: borré la descripción del grupo.", nil},
		Def{"grupo.mensaje_borrado", gGrup, "Se borra un mensaje ajeno (!borrar).", "Listo: borré ese mensaje.", nil},
		Def{"grupo.encuesta_enviada", gGrup, "Se manda una encuesta (!encuesta).", "Listo: mandé la encuesta.", nil},
		Def{"grupo.mensaje_fijado", gGrup, "Se fija un mensaje (!fijar).", "Listo: dejé el mensaje fijado.", nil},
		Def{"grupo.mensaje_desfijado", gGrup, "Se suelta un mensaje fijado (!desfijar).", "Listo: solté el mensaje fijado.", nil},
		Def{"grupo.baneada", gGrup, "Se banea a alguien (!ban).", "Listo: la persona salió y no podrá volver a este grupo.", nil},
		Def{"grupo.desbaneada", gGrup, "Se quita un baneo (!unban).", "Listo: ya puede volver a entrar (por enlace o si la agregan).", nil},
		Def{"grupo.sin_baneados", gGrup, "!baneados cuando no hay nadie baneado.", "No hay nadie baneado en este grupo.", nil},
		Def{"grupo.horario_off", gGrup, "Se apaga el horario de cierre.", "Listo: el grupo ya no se cierra solo.", nil},
		Def{"grupo.horario_on", gGrup, "Se guarda el horario de cierre.", "Listo: se cerrará a las {cierra} y se abrirá a las {abre} (hora de este teléfono).", []string{"cierra", "abre"}},
		Def{"grupo.sin_inactivos", gGrup, "!limpiarinactivos cuando no hay a quién sacar.", "No hay nadie inactivo que sacar.", nil},
		Def{"grupo.inactivos_sacados", gGrup, "!limpiarinactivos terminó.", "Listo: saqué a {n} personas que llevaban más de {dias} días sin escribir.", []string{"n", "dias"}},
		// ---- filtros
		Def{"filtro.encendido", gFilt, "Se enciende un filtro por comando.", "Listo: el filtro de {filtro} quedó encendido.", []string{"filtro"}},
		Def{"filtro.apagado", gFilt, "Se apaga un filtro por comando.", "Listo: el filtro de {filtro} quedó apagado.", []string{"filtro"}},
		Def{"filtro.accion", gFilt, "Se cambia lo que hace un filtro.", "Listo: la acción es {accion}.", []string{"accion"}},
		Def{"filtro.palabra_agregada", gFilt, "Se agrega una palabra prohibida.", "Listo: agregué esa palabra.", nil},
		Def{"filtro.dominio_permitido", gFilt, "Se permite un dominio de enlaces.", "Listo: ese dominio queda permitido.", nil},
		Def{"filtro.quitado", gFilt, "Se quita algo de la lista de un filtro.", "Listo: lo quité de la lista.", nil},
		Def{"filtro.limite", gFilt, "Se cambia el límite de repetidos o flood.", "Listo: el límite es {n} en {segundos} segundos.", []string{"n", "segundos"}},
		// ---- silencios
		Def{"sil.silenciada", gSil, "Se silencia a alguien en grupos (!silenciar).", "Listo: {nombre} queda silenciada {donde} {cuando}. Borraré lo que escriba (solo donde soy administrador).", []string{"nombre", "donde", "cuando"}},
		Def{"sil.contador_cero", gSil, "Se pone en cero el contador de un silencio.", "Listo: el contador de hoy de {nombre} quedó en cero.", []string{"nombre"}},
		// ---- notas
		Def{"util.sin_notas", gUtil, "Alguien pide sus notas y no tiene.", "No tienes notas. Guarda una con !anotar y el texto.", nil},
		Def{"util.sin_pendientes", gUtil, "Alguien pide sus pendientes y no tiene.", "No tienes pendientes. Agrega uno con !tarea y el texto.", nil},
		Def{"util.sin_hechos", gUtil, "Se limpian los pendientes hechos y no había.", "No había pendientes hechos.", nil},
		Def{"util.hechos_borrados", gUtil, "Se borran los pendientes hechos.", "Listo: borré {n} pendientes hechos.", []string{"n"}},
	)
}
