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

data class TipoCuenta(val tipo: String, val enviados: Long, val recibidos: Long, val bytes: Long) { val total: Long get() = enviados + recibidos }
data class Quien(val nombre: String, val cantidad: Long)
data class ResumenChat(
    val desde: Long, val listo: Boolean, val enviados: Long, val recibidos: Long, val porTipo: List<TipoCuenta>,
    val porHora: List<Long>, val porSemana: List<Long>, val quien: List<Quien>,
)
data class DiaCuenta(val dia: String, val cantidad: Long)
data class CalendarioMes(val desde: Long, val listo: Boolean, val dias: List<DiaCuenta>)
data class ChatActivo(val jid: String, val nombre: String, val foto: String, val fotoTs: Long, val esGrupo: Boolean, val pais: String, val cantidad: Long)
data class ResumenGlobal(
    val desde: Long, val listo: Boolean, val enviados: Long, val recibidos: Long, val chats: Long,
    val porTipo: List<TipoCuenta>, val activos: List<ChatActivo>, val porDia: List<DiaCuenta>,
)

/** Contacto compartido en un mensaje. */
data class ContactoCompartido(val nombre: String, val telefonos: List<String>)

/** Ubicación compartida en un mensaje. */
data class UbicacionCompartida(val lat: Double, val lon: Double, val nombre: String, val direccion: String, val vivo: Boolean)

object Json {
    private fun largos(a: JSONArray?) = List(a?.length() ?: 0) { a!!.optLong(it) }
    private fun tipos(a: JSONArray?) = List(a?.length() ?: 0) { i -> a!!.getJSONObject(i).let { TipoCuenta(it.optString("tipo"), it.optLong("enviados"), it.optLong("recibidos"), it.optLong("bytes")) } }
    private fun dias(a: JSONArray?) = List(a?.length() ?: 0) { i -> a!!.getJSONObject(i).let { DiaCuenta(it.optString("dia"), it.optLong("cantidad")) } }

    fun resumenChat(json: String): ResumenChat {
        val o = JSONObject(json)
        val q = o.optJSONArray("quien")
        return ResumenChat(
            o.optLong("desde"), o.optBoolean("listo"), o.optLong("enviados"), o.optLong("recibidos"), tipos(o.optJSONArray("porTipo")),
            largos(o.optJSONArray("porHora")).let { if (it.size == 24) it else List(24) { 0L } },
            largos(o.optJSONArray("porDiaSemana")).let { if (it.size == 7) it else List(7) { 0L } },
            List(q?.length() ?: 0) { i -> q!!.getJSONObject(i).let { Quien(it.optString("nombre"), it.optLong("cantidad")) } },
        )
    }

    fun calendario(json: String): CalendarioMes {
        val o = JSONObject(json)
        return CalendarioMes(o.optLong("desde"), o.optBoolean("listo"), dias(o.optJSONArray("dias")))
    }

    fun resumenGlobal(json: String): ResumenGlobal {
        val o = JSONObject(json)
        val a = o.optJSONArray("activos")
        return ResumenGlobal(
            o.optLong("desde"), o.optBoolean("listo"), o.optLong("enviados"), o.optLong("recibidos"), o.optLong("chats"),
            tipos(o.optJSONArray("porTipo")),
            List(a?.length() ?: 0) { i -> a!!.getJSONObject(i).let { ChatActivo(it.optString("jid"), it.optString("nombre"), it.optString("foto"), it.optLong("fotoTs"), it.optBoolean("esGrupo"), it.optString("pais"), it.optLong("cantidad")) } },
            dias(o.optJSONArray("porDia")),
        )
    }

    /** Contactos de un mensaje de contacto, o null si es un mensaje guardado con el formato anterior (solo texto). */
    fun contactos(texto: String): List<ContactoCompartido>? = runCatching {
        if (!texto.startsWith("{")) return null
        val a = JSONObject(texto).optJSONArray("contactos") ?: return null
        List(a.length()) { i ->
            val c = a.getJSONObject(i)
            val t = c.optJSONArray("tels")
            ContactoCompartido(c.optString("nombre"), List(t?.length() ?: 0) { t!!.optString(it) })
        }
    }.getOrNull()

    fun ubicacion(texto: String): UbicacionCompartida? = runCatching {
        if (!texto.startsWith("{")) return null
        val o = JSONObject(texto)
        UbicacionCompartida(o.optDouble("lat"), o.optDouble("lon"), o.optString("nombre"), o.optString("dir"), o.optBoolean("vivo"))
    }.getOrNull()

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
