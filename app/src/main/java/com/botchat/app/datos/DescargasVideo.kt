package com.botchat.app.datos

import org.json.JSONArray
import org.json.JSONObject

/** Una fila del historial de descargas de video (el archivo no se guarda: se borra al enviarse; desde aquí se puede pedir otra vez). */
data class DescargaVideo(
    val id: Long, val ts: Long, val persona: String, val nombre: String, val chat: String, val esGrupo: Boolean, val url: String, val titulo: String,
    val subidor: String, val sitio: String, val duracion: Int, val peso: Long, val formato: String, val calidad: String, val partes: Int,
    val entrega: String, val estado: String, val error: String,
)

/** Lo de arriba de la pantalla: versión de yt-dlp, si hay una nueva y qué se está bajando. */
data class EstadoVideo(
    val activo: Boolean = false, val motor: Boolean = false, val version: String = "", val ultima: String = "", val hayNueva: Boolean = false,
    val revisionTs: Long = 0, val actualizando: Boolean = false, val enCurso: Int = 0, val esperando: Int = 0, val error: String = "", val hoy: Int = 0,
    /** El motor (yt-dlp) ya está descargado. La app NO lo trae: hay que bajarlo, y mientras no esté el módulo está bloqueado. */
    val instalado: Boolean = false,
)

object JsonVideo {
    fun estado(json: String): EstadoVideo {
        val o = runCatching { JSONObject(json) }.getOrNull() ?: return EstadoVideo()
        return EstadoVideo(o.optBoolean("activo"), o.optBoolean("motor"), o.optString("version"), o.optString("ultima"), o.optBoolean("hayNueva"), o.optLong("revisionTs"),
            o.optBoolean("actualizando"), o.optInt("enCurso"), o.optInt("esperando"), o.optString("error"), o.optInt("hoy"), o.optBoolean("instalado"))
    }

    fun lista(json: String): List<DescargaVideo> {
        if (json.isBlank() || json == "null") return emptyList()
        val a = JSONArray(json)
        return (0 until a.length()).map { i ->
            val o = a.getJSONObject(i)
            DescargaVideo(o.getLong("id"), o.optLong("ts"), o.optString("persona"), o.optString("nombre"), o.optString("chat"), o.optBoolean("esGrupo"), o.optString("url"),
                o.optString("titulo"), o.optString("subidor"), o.optString("sitio"), o.optInt("duracion"), o.optLong("peso"), o.optString("formato"), o.optString("calidad"),
                o.optInt("partes", 1), o.optString("entrega"), o.optString("estado"), o.optString("error"))
        }
    }
}
