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
    /** "", "pendiente" o "resuelto" (marca de trabajo local). */
    val estadoTrabajo: String = "",
    /** Ids de las etiquetas locales del chat. */
    val etiquetas: List<Long> = emptyList(),
    /** Fijado arriba de la lista (solo en esta app). */
    val fijado: Boolean = false,
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
    val eliminadoEn: Long = 0,
    val eliminadoPor: String = "",
    val estadoEnvio: String = "",
    val entregados: Int = 0,
    val leidos: Int = 0,
    val citaId: String = "",
    val citaAutor: String = "",
    val citaTexto: String = "",
    val menciones: List<Mencion> = emptyList(),
    val reenviado: Boolean = false,
    val reacciones: List<Reaccion> = emptyList(),
    val verEstado: String = "",
    val verTipo: String = "",
    val verExpira: Long = 0,
    /** Si lo mandó una automatización (menu, regla, bienvenida, ausencia, recibido, cierre, atajo, comando, modulo, bot); "" = lo escribí yo. */
    val automatico: String = "",
) {
    val categoria: String get() = categoriaDe(tipo)
    val descargado: Boolean get() = estado == "descargado" && ruta.isNotEmpty()

    /** Eliminado por su autor (o un admin) pero todavía se puede mostrar su contenido. */
    val contenidoDisponible: Boolean get() = !borrado || texto.isNotBlank() || descargado
}

data class Mencion(val usuario: String, val nombre: String)
data class Reaccion(val emoji: String, val cantidad: Int, val mia: Boolean)
data class Participante(val jid: String, val usuario: String, val nombre: String, val telefono: String, val esAdmin: Boolean)
data class AcuseDe(val jid: String, val nombre: String, val entregado: Long, val leido: Long, val reproducido: Long)
data class InfoMsg(val enviado: Long, val estado: String, val esGrupo: Boolean, val personas: List<AcuseDe>, val editado: Boolean, val reenviado: Boolean)
data class VerUnaVezAbierto(val ruta: String, val tipo: String, val mime: String)
data class ResultadoEnvio(val id: String, val estado: String, val ts: Long)
data class PresenciaChat(val escribiendo: Boolean = false, val enLinea: Boolean = false, val ultimaVez: Long = 0)
data class ReenvioResultado(val enviados: Int, val omitidos: Int, val fallidos: Int)
data class EstadoChat(val silenciado: Boolean = false, val botPausaHasta: Long = 0, val botActivo: Boolean = true, val pausaHumano: Boolean = false, val estadoTrabajo: String = "", val fijado: Boolean = false)
data class ActividadItem(val ts: Long, val evento: String, val motivo: String, val chat: String, val detalle: String)

/** Qué mensajes de un chat se quieren ver (mismo formato que el filtro del módulo Go). */
data class FiltroLista(
    val tipos: List<String> = emptyList(),
    val direccion: String = "",
    val persona: String = "",
    val eliminados: Boolean = false,
    val hora: Int? = null,
    val semana: Int? = null,
    val dia: String = "",
) {
    fun json(): String = JSONObject().put("tipos", JSONArray(tipos)).put("direccion", direccion).put("persona", persona).put("eliminados", eliminados)
        .put("dia", dia).apply { hora?.let { put("hora", it) }; semana?.let { put("semana", it) } }.toString()

    companion object {
        fun desde(json: String): FiltroLista = runCatching {
            val o = JSONObject(json)
            val t = o.optJSONArray("tipos")
            FiltroLista(
                List(t?.length() ?: 0) { t!!.getString(it) }, o.optString("direccion"), o.optString("persona"), o.optBoolean("eliminados"),
                if (o.has("hora")) o.getInt("hora") else null, if (o.has("semana")) o.getInt("semana") else null, o.optString("dia"),
            )
        }.getOrDefault(FiltroLista())
    }
}

data class PaginaMensajes(val total: Long, val items: List<Msg>)
data class PersonaChat(val jid: String, val nombre: String, val cantidad: Long)

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

data class ResultadoBusqueda(val items: List<Msg>, val fin: Boolean, val sigTs: Long, val sigId: String)
data class ExporteChat(val ruta: String, val mensajes: Long)
data class GrupoAdmin(val jid: String, val nombre: String, val miembros: Int, val soyAdmin: Boolean, val sabido: Boolean)
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
    val porHora: List<Long>, val porSemana: List<Long>, val quien: List<Quien>, val eliminados: Long = 0,
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
            o.optLong("eliminados"),
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
            it.optBoolean("ultimoBorrado"), it.optBoolean("ultimoMio"), it.optString("estadoTrabajo"), longs(it.optJSONArray("etiquetas")), it.optBoolean("fijado"),
        )
    }

    private fun longs(a: JSONArray?) = List(a?.length() ?: 0) { a!!.getLong(it) }

    fun chatOpt(json: String): ChatItem? {
        if (json.isBlank() || json == "null") return null
        val o = JSONObject(json)
        return ChatItem(
            o.optString("jid"), o.optString("nombre"), o.optString("telefono"), o.optString("pais"), o.optBoolean("esGrupo"),
            o.optString("foto"), o.optLong("fotoTs"), o.optLong("ultimoTs"), o.optString("ultimoTexto"), o.optString("ultimoTipo"),
            o.optBoolean("ultimoBorrado"), o.optBoolean("ultimoMio"), o.optString("estadoTrabajo"), longs(o.optJSONArray("etiquetas")), o.optBoolean("fijado"),
        )
    }

    fun mensajes(json: String) = lista(json) {
        Msg(
            it.optString("id"), it.optString("chat"), it.optString("remitenteId"), it.optString("remitente"), it.optBoolean("esMio"),
            it.optString("tipo"), it.optString("texto"), it.optLong("ts"), it.optBoolean("borrado"), it.optBoolean("editado"),
            it.optString("estado"), it.optLong("bytes"), it.optString("mime"), it.optString("nombre"), it.optString("ruta"),
            it.optString("miniatura"), it.optInt("ancho"), it.optInt("alto"), it.optInt("dur"), it.optBoolean("notaVoz"), it.optBoolean("puedeBajarse"),
            it.optLong("eliminadoEn"), it.optString("eliminadoPor"),
            it.optString("estadoEnvio"), it.optInt("entregados"), it.optInt("leidos"), it.optString("citaId"), it.optString("citaAutor"), it.optString("citaTexto"),
            it.optJSONArray("menciones").let { a -> List(a?.length() ?: 0) { i -> a!!.getJSONObject(i).let { m -> Mencion(m.optString("usuario"), m.optString("nombre")) } } },
            it.optBoolean("reenviado"),
            it.optJSONArray("reacciones").let { a -> List(a?.length() ?: 0) { i -> a!!.getJSONObject(i).let { r -> Reaccion(r.optString("emoji"), r.optInt("cantidad"), r.optBoolean("mia")) } } },
            it.optString("verEstado"), it.optString("verTipo"), it.optLong("verExpira"), it.optString("automatico"),
        )
    }

    fun verUnaVez(json: String): VerUnaVezAbierto { val o = JSONObject(json); return VerUnaVezAbierto(o.optString("ruta"), o.optString("tipo"), o.optString("mime")) }
    fun envio(json: String): ResultadoEnvio { val o = JSONObject(json); return ResultadoEnvio(o.optString("id"), o.optString("estado"), o.optLong("ts")) }
    fun presencia(json: String): PresenciaChat = runCatching { JSONObject(json).let { PresenciaChat(it.optBoolean("escribiendo"), it.optBoolean("enLinea"), it.optLong("ultimaVez")) } }.getOrDefault(PresenciaChat())
    fun reenvio(json: String): ReenvioResultado { val o = JSONObject(json); return ReenvioResultado(o.optInt("enviados"), o.optInt("omitidos"), o.optInt("fallidos")) }
    fun estadoChat(json: String): EstadoChat = runCatching { JSONObject(json).let { EstadoChat(it.optBoolean("silenciado"), it.optLong("botPausaHasta"), it.optBoolean("botActivo", true), it.optBoolean("pausaHumano"), it.optString("estadoTrabajo"), it.optBoolean("fijado")) } }.getOrDefault(EstadoChat())
    fun actividad(json: String) = lista(json) { ActividadItem(it.optLong("ts"), it.optString("evento"), it.optString("motivo"), it.optString("chat"), it.optString("detalle")) }
    fun participantes(json: String) = lista(json) { Participante(it.optString("jid"), it.optString("usuario"), it.optString("nombre"), it.optString("telefono"), it.optBoolean("esAdmin")) }
    fun infoMensaje(json: String): InfoMsg {
        val o = JSONObject(json)
        val a = o.optJSONArray("personas")
        return InfoMsg(
            o.optLong("enviado"), o.optString("estado"), o.optBoolean("esGrupo"),
            List(a?.length() ?: 0) { i -> a!!.getJSONObject(i).let { AcuseDe(it.optString("jid"), it.optString("nombre"), it.optLong("entregado"), it.optLong("leido"), it.optLong("reproducido")) } },
            o.optBoolean("editado"), o.optBoolean("reenviado"),
        )
    }

    fun pagina(json: String): PaginaMensajes {
        val o = JSONObject(json)
        return PaginaMensajes(o.optLong("total"), mensajes(o.optJSONArray("items")?.toString() ?: "[]"))
    }

    /** Resultado de buscar una palabra en un chat: `fin` = ya se recorrió todo; si no, seguir con (sigTs, sigId). */
    fun busqueda(json: String): ResultadoBusqueda {
        val o = JSONObject(json.ifBlank { "{}" })
        return ResultadoBusqueda(mensajes(o.optJSONArray("items")?.toString() ?: "[]"), o.optBoolean("fin", true), o.optLong("sigTs"), o.optString("sigId"))
    }
    fun exporte(json: String): ExporteChat { val o = JSONObject(json.ifBlank { "{}" }); return ExporteChat(o.optString("ruta"), o.optLong("mensajes")) }
    fun gruposAdmin(json: String) = lista(json) { GrupoAdmin(it.optString("jid"), it.optString("nombre"), it.optInt("miembros"), it.optBoolean("soyAdmin"), it.optBoolean("sabido")) }

    fun personas(json: String) = lista(json) { PersonaChat(it.optString("jid"), it.optString("nombre"), it.optLong("cantidad")) }

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
