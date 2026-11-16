package com.botchat.app.datos

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject

private suspend fun <T> avatarIo(f: () -> T): T = withContext(Dispatchers.IO) { f() }

/** Un momento en que el avatar puede salir como imagen (menú, ayuda, bienvenida, avisos…). */
data class MomentoAvatar(val id: String, val nombre: String, val cara: String, val activo: Boolean, val grupos: Boolean, val propio: Boolean = false)

/** Cómo está el avatar del bot: encendido, si usa una imagen propia y en qué momentos sale. */
data class EstadoAvatar(val activo: Boolean = true, val propio: Boolean = false, val momentos: List<MomentoAvatar> = emptyList())

object JsonAvatar {
    fun estado(json: String): EstadoAvatar {
        val o = JSONObject(json.ifBlank { "{}" })
        val m = o.optJSONArray("momentos")
        return EstadoAvatar(
            o.optBoolean("activo", true), o.optBoolean("propio"),
            List(m?.length() ?: 0) { i -> m!!.getJSONObject(i).let { MomentoAvatar(it.optString("id"), it.optString("nombre"), it.optString("cara"), it.optBoolean("activo"), it.optBoolean("grupos"), it.optBoolean("propio")) } },
        )
    }
}

suspend fun Repo.estadoAvatar(bot: Long) = avatarIo { JsonAvatar.estado(motor.avatarEstado(bot)) }
suspend fun Repo.configurarAvatar(bot: Long, activo: Boolean, momentos: List<String>) = avatarIo { motor.avatarConfigurar(bot, activo, momentos.joinToString(",")) }
suspend fun Repo.avatarPropio(bot: Long, id: String, mime: String) = avatarIo { motor.avatarPropio(bot, id, mime) }
suspend fun Repo.avatarPropioDe(bot: Long, momento: String, id: String, mime: String) = avatarIo { motor.avatarPropioDe(bot, momento, id, mime) }
