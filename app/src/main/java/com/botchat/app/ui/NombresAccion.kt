package com.botchat.app.ui

/** Nombre claro de una acción del registro («grupo.base_aplicar» → «Configuración base puesta en un grupo»). */
internal fun nombreAccion(a: String): String = NOMBRES_ACCION[a] ?: PREFIJOS_ACCION.firstOrNull { a.startsWith(it.first) }?.let { "${it.second}: ${a.substringAfter('.').replace('_', ' ')}" } ?: a

/** Si la acción cambió cómo funciona el bot (reglas, filtros, horarios, respuestas, ajustes, rangos, módulos, copias…). */
internal fun esCambioConfig(a: String): Boolean =
    a in CAMBIOS_FIJOS || CAMBIOS_PREFIJO.any { a.startsWith(it) }

private val CAMBIOS_PREFIJO = listOf(
    "ajuste", "respuesta.", "rango.", "modulo.", "respaldo.", "advertencias.", "comando_propio.", "etiqueta.", "iniciativa.", "asistente.", "auto.", "menu.",
    "grupo.base", "grupo.filtro", "grupo.reglas", "grupo.horario", "grupo.entrada", "grupo.politica", "ajustes.", "video.", "lab.",
)
private val CAMBIOS_FIJOS = setOf("grupo.politica")

private val PREFIJOS_ACCION = listOf(
    "filtro." to "Filtro", "persona." to "Persona", "rango." to "Rango", "modulo." to "Módulo", "respuesta." to "Respuesta", "ajuste" to "Ajuste",
    "advertencias." to "Advertencias", "silencio." to "Silencio", "grupo." to "Grupo", "video." to "Descargas", "auto." to "Automatización", "menu." to "Menú",
    "chat." to "Chat", "lab." to "Laboratorio", "iniciativa." to "Iniciativa",
)

private val NOMBRES_ACCION = mapOf(
    "grupo.base" to "Un grupo quedó como configuración base",
    "grupo.base_aplicar" to "Configuración base puesta en un grupo",
    "grupo.base_auto" to "Base automática para grupos nuevos",
    "grupo.base_quitar" to "Configuración base quitada",
    "grupo.filtro" to "Filtro de grupo cambiado",
    "grupo.reglas" to "Reglas del grupo cambiadas",
    "grupo.horario" to "Horario de cierre cambiado",
    "grupo.entrada" to "Protección de entrada cambiada",
    "grupo.banear" to "Persona baneada",
    "grupo.desbanear" to "Baneo quitado",
    "grupo.raid" to "Alarma de raid",
    "grupo.baneado_volvio" to "Baneada que intentó volver",
    "grupo.prefijo_bloqueado" to "País bloqueado al entrar",
    "grupo.encuesta" to "Encuesta enviada",
    "grupo.fijar" to "Mensaje fijado",
    "grupo.desfijar" to "Mensaje soltado",
    "grupo.expulsar" to "Persona expulsada",
    "grupo.politica" to "Política de grupos cambiada",
    "ajustes.alertas" to "Avisos y resumen cambiados",
    "respaldo.exportar" to "Copia de seguridad guardada",
    "respaldo.restaurar" to "Copia de seguridad restaurada",
    "rango.asignar" to "Rango asignado",
    "rango.quitar" to "Rango quitado",
    "rango.guardar" to "Rango guardado",
    "rango.borrar" to "Rango borrado",
    "respuesta.cambiar" to "Respuesta del bot cambiada",
    "respuesta.restaurar" to "Respuestas restauradas",
    "respuesta.reemplazos" to "Frases cambiadas",
    "modulo.activar" to "Módulo encendido",
    "modulo.desactivar" to "Módulo apagado",
    "modulo.config" to "Módulo configurado",
    "ajuste.usuario_auto" to "Rango «Usuario» automático cambiado",
    "video.quitar_motor" to "Motor de descargas quitado",
)
