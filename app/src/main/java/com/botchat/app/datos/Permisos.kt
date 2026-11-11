package com.botchat.app.datos

import org.json.JSONArray
import org.json.JSONObject

/** Una función que se le puede dar a un rango (el catálogo vive en el módulo Go). */
data class Casilla(
    val id: String, val nombre: String, val descripcion: String, val grupo: String, val riesgo: String,
    val privado: Boolean, val grupos: Boolean, val confirma: Boolean, val base: Boolean,
)

/** Hasta dónde vale una casilla: "" (donde aplique), "privado", "grupos" o "chats" (solo esos chats). */
data class AlcanceCasilla(val modo: String = "", val chats: List<String> = emptyList())

data class Rango(
    val id: Long, val nombre: String, val color: String, val posicion: Int, val plantilla: String, val esDueno: Boolean,
    val casillas: Map<String, AlcanceCasilla>, val menusModo: String, val menus: List<String>, val limiteMin: Int, val personas: Int,
) {
    fun json(): String {
        val c = JSONObject()
        casillas.forEach { (k, a) -> c.put(k, JSONObject().put("modo", a.modo).put("chats", JSONArray(a.chats))) }
        return JSONObject().put("id", id).put("nombre", nombre).put("color", color).put("plantilla", plantilla).put("casillas", c)
            .put("menusModo", menusModo).put("menus", JSONArray(menus)).put("limiteMin", limiteMin).toString()
    }

    companion object {
        fun nuevo() = Rango(0, "", "#2EC4CB", 0, "", false, emptyMap(), "todos", emptyList(), 10, 0)
    }
}

data class AsignacionRango(val persona: String, val rolId: Long, val caducaEn: Long, val asignadoPor: String, val asignadoTs: Long)
data class RangoDePersona(val rango: Rango?, val asignacion: AsignacionRango?, val bloqueada: Boolean)

data class SolicitudItem(val id: Long, val persona: String, val nombre: String, val estado: String, val primeraTs: Long, val ultimaTs: Long, val mensajes: Int)
data class ConfirmacionItem(val codigo: String, val persona: String, val comando: String, val args: List<String>, val chat: String, val expiraTs: Long)
data class PoliticaGrupo(val chat: String, val modo: String, val lista: List<String>)
data class AccionItem(val id: Long, val ts: Long, val quien: String, val accion: String, val objetivo: String, val chat: String, val resultado: String, val detalle: String)
data class ResultadoComando(val manejado: Boolean, val texto: String)

object JsonPerm {
    private fun <T> lista(json: String, f: (JSONObject) -> T): List<T> {
        val a = JSONArray(json.ifBlank { "[]" })
        return List(a.length()) { f(a.getJSONObject(it)) }
    }

    private fun strs(a: JSONArray?) = List(a?.length() ?: 0) { a!!.getString(it) }

    fun casillas(json: String) = lista(json) {
        Casilla(it.optString("id"), it.optString("nombre"), it.optString("descripcion"), it.optString("grupo"), it.optString("riesgo", "bajo"),
            it.optBoolean("privado", true), it.optBoolean("grupos", true), it.optBoolean("confirma"), it.optBoolean("base"))
    }

    fun rango(o: JSONObject): Rango {
        val c = o.optJSONObject("casillas")
        val mapa = LinkedHashMap<String, AlcanceCasilla>()
        c?.keys()?.forEach { k -> val a = c.getJSONObject(k); mapa[k] = AlcanceCasilla(a.optString("modo"), strs(a.optJSONArray("chats"))) }
        return Rango(
            o.optLong("id"), o.optString("nombre"), o.optString("color", "#2EC4CB"), o.optInt("posicion"), o.optString("plantilla"), o.optBoolean("esDueno"),
            mapa, o.optString("menusModo", "todos"), strs(o.optJSONArray("menus")), o.optInt("limiteMin"), o.optInt("personas"),
        )
    }

    fun rangos(json: String) = lista(json) { rango(it) }
    fun rangoOpt(json: String): Rango? = if (json.isBlank() || json.trim() == "null") null else rango(JSONObject(json))

    private fun asignacion(o: JSONObject?) = o?.let { AsignacionRango(it.optString("persona"), it.optLong("rolId"), it.optLong("caducaEn"), it.optString("asignadoPor"), it.optLong("asignadoTs")) }

    fun rangoDePersona(json: String): RangoDePersona {
        val o = JSONObject(json)
        return RangoDePersona(o.optJSONObject("rol")?.let { rango(it) }, asignacion(o.optJSONObject("asignacion")), o.optBoolean("bloqueada"))
    }

    fun asignaciones(json: String) = lista(json) { asignacion(it)!! }
    fun solicitudes(json: String) = lista(json) {
        SolicitudItem(it.optLong("id"), it.optString("persona"), it.optString("nombre"), it.optString("estado"), it.optLong("primeraTs"), it.optLong("ultimaTs"), it.optInt("mensajes"))
    }

    fun confirmaciones(json: String) = lista(json) {
        ConfirmacionItem(it.optString("codigo"), it.optString("persona"), it.optString("comando"), strs(it.optJSONArray("args")), it.optString("chat"), it.optLong("expiraTs"))
    }

    fun politica(o: JSONObject) = PoliticaGrupo(o.optString("chat"), o.optString("modo", "todos"), strs(o.optJSONArray("lista")))
    fun politica(json: String) = politica(JSONObject(json))
    fun politicas(json: String) = lista(json) { politica(it) }
    fun ajustes(json: String): Map<String, String> {
        val o = JSONObject(json.ifBlank { "{}" })
        return o.keys().asSequence().associateWith { o.optString(it) }
    }

    fun acciones(json: String) = lista(json) {
        AccionItem(it.optLong("id"), it.optLong("ts"), it.optString("quien"), it.optString("accion"), it.optString("objetivo"), it.optString("chat"), it.optString("resultado"), it.optString("detalle"))
    }

    fun comando(json: String): ResultadoComando { val o = JSONObject(json); return ResultadoComando(o.optBoolean("manejado"), o.optString("texto")) }
}
