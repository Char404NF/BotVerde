package com.botchat.app.datos

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject

/** Una cosa que el bot contesta por su cuenta ante un comando: su texto de fábrica, el propio (vacío = el de fábrica) y las {variables} que admite. */
data class RespuestaBot(val id: String, val grupo: String, val descripcion: String, val defecto: String, val vars: List<String>, val propio: String) {
    val cambiada: Boolean get() = propio.isNotBlank()
    val actual: String get() = propio.ifBlank { defecto }
}

/** «Cambiar esta frase por esta otra» en cualquier respuesta. */
data class ReemplazoBot(val de: String, val a: String)

data class ListaRespuestas(val grupos: List<String>, val filas: List<RespuestaBot>, val reemplazos: List<ReemplazoBot>)

object JsonRespuestas {
    fun lista(json: String): ListaRespuestas {
        val o = JSONObject(json.ifBlank { "{}" })
        val g = o.optJSONArray("grupos"); val f = o.optJSONArray("filas"); val r = o.optJSONArray("reemplazos")
        return ListaRespuestas(
            List(g?.length() ?: 0) { g!!.getString(it) },
            List(f?.length() ?: 0) {
                val x = f!!.getJSONObject(it); val v = x.optJSONArray("vars")
                RespuestaBot(x.optString("id"), x.optString("grupo"), x.optString("descripcion"), x.optString("defecto"), List(v?.length() ?: 0) { i -> v!!.getString(i) }, x.optString("propio"))
            },
            List(r?.length() ?: 0) { val x = r!!.getJSONObject(it); ReemplazoBot(x.optString("de"), x.optString("a")) },
        )
    }

    fun reemplazosJson(l: List<ReemplazoBot>): String = JSONArray().also { a -> l.forEach { a.put(JSONObject().put("de", it.de).put("a", it.a)) } }.toString()
}

private suspend fun <T> io(f: () -> T): T = withContext(Dispatchers.IO) { f() }

suspend fun Repo.respuestas(bot: Long) = io { JsonRespuestas.lista(motor.respuestasLista(bot)) }
suspend fun Repo.fijarRespuesta(bot: Long, id: String, texto: String) = io { motor.respuestaFijar(bot, id, texto) }
suspend fun Repo.restaurarRespuestas(bot: Long) = io { motor.respuestasRestaurar(bot).toInt() }
suspend fun Repo.fijarReemplazos(bot: Long, l: List<ReemplazoBot>) = io { motor.reemplazosFijar(bot, JsonRespuestas.reemplazosJson(l)) }
