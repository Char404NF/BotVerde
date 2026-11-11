package com.botchat.app.datos

import org.json.JSONArray
import org.json.JSONObject

/** Ajustes y estado del Laboratorio (Avanzado → Laboratorio). `modo` es lo elegido; `modoReal` lo que vale ahora. */
data class EstadoLab(
    val activo: Boolean, val avisoAceptado: Boolean, val modo: String, val modoReal: String, val grupos: Boolean, val encMax: Int, val expiraMin: Int,
    val respaldoEnc: Boolean, val respaldoBot: Boolean, val botonesTipo: String, val pruebaChat: String, val pruebaHora: Int, val pruebaDia: Int,
    val versionWa: String, val versionCliente: String, val enMarcha: Boolean, val limiteMedido: Int, val pruebasHora: Int, val pruebasDia: Int,
    val plan: List<PruebaLab>,
)

/** Una prueba que se puede mandar: tipo (encuesta, botones, lista, interactivo) y cuántas opciones lleva. */
data class PruebaLab(val tipo: String, val variante: Int, val nombre: String)

/**
 * Un renglón de la matriz. Los números son -1 (sin saber), 0 (no) y 1 (sí). `llego` y `respondio` los anota el bot; `visible` y `tocable`
 * los marcas tú mirando el teléfono que recibió la prueba.
 */
data class ResultadoLab(
    val id: Long, val ts: Long, val tipo: String, val variante: String, val chat: String, val enviado: Int, val llego: Int, val visible: Int, val tocable: Int,
    val respondio: Int, val versionWa: String, val versionCliente: String, val error: String,
)

object JsonLab {
    fun estado(json: String): EstadoLab {
        val o = JSONObject(json)
        val p = o.optJSONArray("plan") ?: JSONArray()
        return EstadoLab(
            o.optBoolean("activo"), o.optBoolean("avisoAceptado"), o.optString("modo", "texto"), o.optString("modoReal", "texto"), o.optBoolean("grupos"),
            o.optInt("encMax", 12), o.optInt("expiraMin", 120), o.optBoolean("respaldoEnc"), o.optBoolean("respaldoBot"), o.optString("botonesTipo", "auto"),
            o.optString("pruebaChat"), o.optInt("pruebaHora", 4), o.optInt("pruebaDia", 12), o.optString("versionWa"), o.optString("versionCliente"),
            o.optBoolean("enMarcha"), o.optInt("limiteMedido"), o.optInt("pruebasHora"), o.optInt("pruebasDia"),
            List(p.length()) { i -> p.getJSONObject(i).let { PruebaLab(it.optString("tipo"), it.optInt("variante"), it.optString("nombre")) } },
        )
    }

    fun resultado(o: JSONObject) = ResultadoLab(
        o.optLong("id"), o.optLong("ts"), o.optString("tipo"), o.optString("variante"), o.optString("chat"), o.optInt("enviado", -1), o.optInt("llego", -1),
        o.optInt("visible", -1), o.optInt("tocable", -1), o.optInt("respondio", -1), o.optString("versionWa"), o.optString("versionCliente"), o.optString("error"),
    )

    fun resultados(json: String): List<ResultadoLab> {
        val a = JSONArray(json.ifBlank { "[]" })
        return List(a.length()) { resultado(a.getJSONObject(it)) }
    }
}
