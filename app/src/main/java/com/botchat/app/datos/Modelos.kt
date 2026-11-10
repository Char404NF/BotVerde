package com.botchat.app.datos

import org.json.JSONArray
import org.json.JSONObject

/** Fila de la lista de chats. `jid` es la clave interna: la pantalla nunca la muestra. */
data class ChatItem(
    val jid: String,
    val nombre: String,
    val telefono: String,
    val pais: String,
    val esGrupo: Boolean,
    val foto: String,
    val fotoTs: Long,
    val ultimoTs: Long,
    val ultimoTexto: String,
    val ultimoTipo: String,
    val ultimoBorrado: Boolean,
    val ultimoMio: Boolean,
)

/** Mensaje listo para mostrar. */
data class Msg(
    val id: String,
    val chat: String,
    val remitenteId: String,
    val remitente: String,
    val esMio: Boolean,
    val tipo: String,
    val texto: String,
    val ts: Long,
    val borrado: Boolean,
    val editado: Boolean,
    val estado: String,
    val bytes: Long,
    val mime: String,
    val nombre: String,
    val ruta: String,
    val miniatura: String,
    val ancho: Int,
    val alto: Int,
    val dur: Int,
    val notaVoz: Boolean,
    val puedeBajarse: Boolean,
) {
    val categoria: String get() = categoriaDe(tipo)
    val descargado: Boolean get() = estado == "descargado" && ruta.isNotEmpty()
}

fun categoriaDe(tipo: String): String = when (tipo) {
    "imagen", "sticker" -> "foto"
    "video" -> "video"
    "audio" -> "audio"
    "documento" -> "documento"
    else -> ""
}

data class EspacioTipo(val categoria: String, val bytes: Long, val cantidad: Int)

data class EspacioChat(
    val jid: String,
    val nombre: String,
    val foto: String,
    val fotoTs: Long,
    val esGrupo: Boolean,
    val pais: String,
    val bytes: Long,
    val cantidad: Int,
    val porTipo: List<Long>, // en el orden foto, video, audio, documento
)

data class EspacioTotal(val multimedia: Long, val miniaturas: Long, val fotos: Long, val base: Long, val libre: Long, val total: Long) {
    val ocupaBotChat: Long get() = multimedia + miniaturas + fotos + base
}

data class Liberado(val bytes: Long, val cantidad: Int)

data class ReporteReparacion(
    val estadosCorregidos: Int,
    val tamanosCorregidos: Int,
    val archivosDanados: Int,
    val miniaturasLimpiadas: Int,
    val fotosLimpiadas: Int,
    val huerfanos: Int,
    val bytesHuerfanos: Long,
    val chatsRecalculados: Int,
) {
    val sinCambios: Boolean get() = estadosCorregidos + tamanosCorregidos + archivosDanados + miniaturasLimpiadas + fotosLimpiadas + huerfanos + chatsRecalculados == 0
}

object Json {
    private inline fun <T> lista(json: String, f: (JSONObject) -> T): List<T> {
        val a = JSONArray(json.ifBlank { "[]" })
        return List(a.length()) { f(a.getJSONObject(it)) }
    }

    fun chats(json: String) = lista(json) {
        ChatItem(
            it.optString("jid"), it.optString("nombre"), it.optString("telefono"), it.optString("pais"), it.optBoolean("esGrupo"),
            it.optString("foto"), it.optLong("fotoTs"), it.optLong("ultimoTs"), it.optString("ultimoTexto"), it.optString("ultimoTipo"),
            it.optBoolean("ultimoBorrado"), it.optBoolean("ultimoMio"),
        )
    }

    fun chatOpt(json: String): ChatItem? {
        if (json.isBlank() || json == "null") return null
        val o = JSONObject(json)
        return ChatItem(
            o.optString("jid"), o.optString("nombre"), o.optString("telefono"), o.optString("pais"), o.optBoolean("esGrupo"),
            o.optString("foto"), o.optLong("fotoTs"), o.optLong("ultimoTs"), o.optString("ultimoTexto"), o.optString("ultimoTipo"),
            o.optBoolean("ultimoBorrado"), o.optBoolean("ultimoMio"),
        )
    }

    fun mensajes(json: String) = lista(json) {
        Msg(
            it.optString("id"), it.optString("chat"), it.optString("remitenteId"), it.optString("remitente"), it.optBoolean("esMio"),
            it.optString("tipo"), it.optString("texto"), it.optLong("ts"), it.optBoolean("borrado"), it.optBoolean("editado"),
            it.optString("estado"), it.optLong("bytes"), it.optString("mime"), it.optString("nombre"), it.optString("ruta"),
            it.optString("miniatura"), it.optInt("ancho"), it.optInt("alto"), it.optInt("dur"), it.optBoolean("notaVoz"), it.optBoolean("puedeBajarse"),
        )
    }

    fun espacioPorTipo(json: String) = lista(json) { EspacioTipo(it.optString("categoria"), it.optLong("bytes"), it.optInt("cantidad")) }

    fun espacioPorChat(json: String) = lista(json) { o ->
        val pt = o.optJSONArray("porTipo")
        EspacioChat(
            o.optString("jid"), o.optString("nombre"), o.optString("foto"), o.optLong("fotoTs"), o.optBoolean("esGrupo"), o.optString("pais"),
            o.optLong("bytes"), o.optInt("cantidad"), List(pt?.length() ?: 0) { pt!!.optLong(it) },
        )
    }

    fun espacioTotal(json: String): EspacioTotal {
        val o = JSONObject(json)
        return EspacioTotal(o.optLong("multimedia"), o.optLong("miniaturas"), o.optLong("fotos"), o.optLong("base"), o.optLong("libre"), o.optLong("total"))
    }

    fun liberado(json: String): Liberado {
        val o = JSONObject(json)
        return Liberado(o.optLong("bytes"), o.optInt("cantidad"))
    }

    fun reporte(json: String): ReporteReparacion {
        val o = JSONObject(json)
        return ReporteReparacion(
            o.optInt("estadosCorregidos"), o.optInt("tamanosCorregidos"), o.optInt("archivosDanados"), o.optInt("miniaturasLimpiadas"),
            o.optInt("fotosLimpiadas"), o.optInt("huerfanos"), o.optLong("bytesHuerfanos"), o.optInt("chatsRecalculados"),
        )
    }

    fun filtro(chat: String = "", categoria: String = "", diasMin: Int = 0): String =
        JSONObject().put("chat", chat).put("categoria", categoria).put("diasMin", diasMin).toString()

    fun items(items: List<Pair<String, String>>): String {
        val a = JSONArray()
        items.forEach { (chat, id) -> a.put(JSONObject().put("chat", chat).put("id", id)) }
        return a.toString()
    }
}
