package com.botchat.app.datos

import org.json.JSONArray
import org.json.JSONObject

// Personas, chat y avisos para mí (0.5.0, bloque B). Todo se calcula en el módulo Go; aquí solo viven los modelos y su lectura.

/** El bot no le contesta a esa persona (hastaTs 0 = para siempre). */
data class SilencioInfo(val persona: String, val hastaTs: Long, val motivo: String, val creadoTs: Long, val por: String)

/** Etiqueta local para ordenar chats. */
data class EtiquetaItem(val id: Long, val nombre: String, val color: String, val posicion: Int = 0, val chats: Int = 0,
    val emoji: String = "", val personas: Int = 0, val mensajes: Int = 0)

/** Todo lo que se sabe de una persona, en una sola vista. */
data class FichaPersona(
    val persona: String, val alias: List<String>, val nombre: String, val telefono: String,
    val silencio: SilencioInfo?, val bloqueada: Boolean, val noMolestar: Boolean,
    val advertencias: Int, val notas: Int, val etiquetas: List<EtiquetaItem>,
    val primeraTs: Long, val ultimaTs: Long, val recibidos: Int, val enviados: Int, val grupos: Int,
    val rango: String, val rangoColor: String,
    /** El nombre que el dueño le puso en la app ("" = ninguno). */
    val nombrePropio: String = "",
)

data class AdvertenciaItem(val id: Long, val persona: String, val chat: String, val motivo: String, val ts: Long, val por: String, val tipoId: Long = 0, val caducaTs: Long = 0, val vigente: Boolean = true)
data class AdvertidoItem(val persona: String, val total: Int, val ultimaTs: Long, val nombre: String, val tope: Int)
data class NotaItem(val id: Long, val persona: String, val texto: String, val ts: Long, val por: String)

/** Qué pasó al advertir. `hecho`: "avisado", "silenciado", "borrado", "expulsado" o "solo_aviso" (no se pudo hacer: ver `motivo`). */
data class ResultadoAdvertencia(val total: Int, val tope: Int, val llegoAlTope: Boolean, val accion: String, val hecho: String, val motivo: String)

data class ConteoChat(val chat: String, val nombre: String, val cuenta: Int, val grupo: Boolean)
data class EstadisticasPersona(
    val persona: String, val total: Int, val porTipo: Map<String, Int>, val porHora: List<Int>,
    val porDia: List<Pair<String, Int>>, val chats: List<ConteoChat>, val bytes: Long, val primerDia: String, val ultimoDia: String,
)

data class ResultadoOlvidar(val mensajes: Int, val bytes: Long, val notas: Int, val advertencias: Int, val sesiones: Int, val alias: Int)

/** Una nota o un pendiente propio (bloque F). */
data class UtilItem(val id: Long, val tipo: String, val texto: String, val hecho: Boolean, val ts: Long)

data class RecordatorioItem(val id: Long, val texto: String, val proximaTs: Long, val repetir: String, val estado: String, val creadoTs: Long, val avisadoTs: Long)
data class AlertaPalabraItem(val id: Long, val palabra: String, val chats: List<String>, val activa: Boolean)

/** Lo que salió al disparar los avisos: los ids de los avisos nuevos (para mostrarlos) y cuándo es la próxima alarma (0 = ninguna). */
data class DisparoAvisos(val avisos: List<Long>, val siguiente: Long)

data class OpcionComando(val texto: String, val respuesta: String, val adjuntos: List<AdjuntoSubido> = emptyList())

/** Un comando propio (!horario, !precios…). `nombre` va sin prefijo. */
data class ComandoPropio(
    val nombre: String = "", val texto: String = "", val resumen: String = "", val rangoMin: Long = 0,
    val privados: Boolean = true, val grupos: Boolean = false, val ocultoAyuda: Boolean = false, val opciones: List<OpcionComando> = emptyList(),
    /** Archivos (hasta 3) de la respuesta cuando el comando no tiene opciones; con opciones, cada opción lleva los suyos. */
    val adjuntos: List<AdjuntoSubido> = emptyList(),
) {
    fun json(): String = JSONObject().put("nombre", nombre).put("texto", texto).put("resumen", resumen).put("rangoMin", rangoMin)
        .put("privados", privados).put("grupos", grupos).put("ocultoAyuda", ocultoAyuda)
        .put("adjuntos", JsonMensajesAuto.adjuntosJson(adjuntos))
        .put("opciones", JSONArray().also { a -> opciones.forEach { a.put(JSONObject().put("texto", it.texto).put("respuesta", it.respuesta).put("adjuntos", JsonMensajesAuto.adjuntosJson(it.adjuntos))) } }).toString()
}

data class ComandoPropioItem(val id: Long, val activo: Boolean, val datos: ComandoPropio)

object JsonPersonas {
    private fun <T> lista(json: String, f: (JSONObject) -> T): List<T> {
        val a = JSONArray(json.ifBlank { "[]" })
        return List(a.length()) { f(a.getJSONObject(it)) }
    }

    private fun strs(a: JSONArray?) = List(a?.length() ?: 0) { a!!.getString(it) }

    fun silencio(o: JSONObject?) = o?.let { SilencioInfo(it.optString("persona"), it.optLong("hastaTs"), it.optString("motivo"), it.optLong("creadoTs"), it.optString("por")) }
    fun silencios(json: String) = lista(json) { silencio(it)!! }

    fun etiqueta(o: JSONObject) = EtiquetaItem(o.optLong("id"), o.optString("nombre"), o.optString("color", "#2EC4CB"), o.optInt("posicion"), o.optInt("chats"),
        o.optString("emoji"), o.optInt("personas"), o.optInt("mensajes"))
    fun etiquetas(json: String) = lista(json) { etiqueta(it) }

    fun ficha(json: String): FichaPersona {
        val o = JSONObject(json)
        val et = o.optJSONArray("etiquetas")
        return FichaPersona(
            o.optString("persona"), strs(o.optJSONArray("alias")), o.optString("nombre"), o.optString("telefono"),
            silencio(o.optJSONObject("silencio")), o.optBoolean("bloqueada"), o.optBoolean("noMolestar"),
            o.optInt("advertencias"), o.optInt("notas"), List(et?.length() ?: 0) { etiqueta(et!!.getJSONObject(it)) },
            o.optLong("primeraTs"), o.optLong("ultimaTs"), o.optInt("recibidos"), o.optInt("enviados"), o.optInt("grupos"),
            o.optString("rango"), o.optString("rangoColor"), o.optString("nombrePropio"),
        )
    }

    fun advertencias(json: String) = lista(json) { AdvertenciaItem(it.optLong("id"), it.optString("persona"), it.optString("chat"), it.optString("motivo"), it.optLong("ts"), it.optString("por"), it.optLong("tipoId"), it.optLong("caducaTs"), it.optBoolean("vigente", true)) }
    fun advertidos(json: String) = lista(json) { AdvertidoItem(it.optString("persona"), it.optInt("total"), it.optLong("ultimaTs"), it.optString("nombre"), it.optInt("tope")) }
    fun notas(json: String) = lista(json) { NotaItem(it.optLong("id"), it.optString("persona"), it.optString("texto"), it.optLong("ts"), it.optString("por")) }

    fun resultadoAdvertencia(json: String): ResultadoAdvertencia {
        val o = JSONObject(json)
        return ResultadoAdvertencia(o.optInt("total"), o.optInt("tope"), o.optBoolean("llegoAlTope"), o.optString("accion"), o.optString("hecho"), o.optString("motivo"))
    }

    fun estadisticas(json: String): EstadisticasPersona {
        val o = JSONObject(json)
        val t = o.optJSONObject("porTipo")
        val h = o.optJSONArray("porHora")
        val d = o.optJSONArray("porDia")
        return EstadisticasPersona(
            o.optString("persona"), o.optInt("total"),
            t?.keys()?.asSequence()?.associateWith { t.optInt(it) } ?: emptyMap(),
            List(24) { h?.optInt(it) ?: 0 },
            List(d?.length() ?: 0) { d!!.getJSONObject(it).let { x -> x.optString("dia") to x.optInt("cuenta") } },
            lista(o.optJSONArray("chats")?.toString() ?: "[]") { ConteoChat(it.optString("chat"), it.optString("nombre"), it.optInt("cuenta"), it.optBoolean("grupo")) },
            o.optLong("bytes"), o.optString("primerDia"), o.optString("ultimoDia"),
        )
    }

    fun olvidar(json: String): ResultadoOlvidar {
        val o = JSONObject(json)
        return ResultadoOlvidar(o.optInt("mensajes"), o.optLong("bytes"), o.optInt("notas"), o.optInt("advertencias"), o.optInt("sesiones"), o.optInt("alias"))
    }

    fun recordatorios(json: String) = lista(json) {
        RecordatorioItem(it.optLong("id"), it.optString("texto"), it.optLong("proximaTs"), it.optString("repetir"), it.optString("estado"), it.optLong("creadoTs"), it.optLong("avisadoTs"))
    }

    fun utilidades(json: String) = lista(json) { UtilItem(it.optLong("id"), it.optString("tipo"), it.optString("texto"), it.optBoolean("hecho"), it.optLong("ts")) }

    fun palabras(json: String) = lista(json) { AlertaPalabraItem(it.optLong("id"), it.optString("palabra"), strs(it.optJSONArray("chats")), it.optBoolean("activa", true)) }

    fun disparo(json: String): DisparoAvisos {
        val o = JSONObject(json)
        val a = o.optJSONArray("avisos")
        return DisparoAvisos(List(a?.length() ?: 0) { a!!.getLong(it) }, o.optLong("siguiente"))
    }

    fun comandoPropio(o: JSONObject): ComandoPropio {
        val ops = o.optJSONArray("opciones")
        return ComandoPropio(
            o.optString("nombre"), o.optString("texto"), o.optString("resumen"), o.optLong("rangoMin"), o.optBoolean("privados"), o.optBoolean("grupos"),
            o.optBoolean("ocultoAyuda"), List(ops?.length() ?: 0) { ops!!.getJSONObject(it).let { x -> OpcionComando(x.optString("texto"), x.optString("respuesta"), JsonMensajesAuto.adjuntos(x.optJSONArray("adjuntos"))) } },
            JsonMensajesAuto.adjuntos(o.optJSONArray("adjuntos")),
        )
    }

    fun comandosPropios(json: String) = lista(json) { ComandoPropioItem(it.optLong("id"), it.optBoolean("activo"), comandoPropio(it.getJSONObject("datos"))) }
}
