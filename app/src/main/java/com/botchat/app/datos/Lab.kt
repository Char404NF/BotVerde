package com.botchat.app.datos

import org.json.JSONArray
import org.json.JSONObject

/** Cómo salen los menús (Ajustes de los menús): texto, encuesta o botones. `modo` es lo elegido; `modoReal` lo que vale ahora. */
data class EstadoLab(
    val activo: Boolean, val avisoAceptado: Boolean, val modo: String, val modoReal: String, val grupos: Boolean, val encMax: Int, val expiraMin: Int,
    val respaldoEnc: Boolean, val respaldoBot: Boolean, val botonesTipo: String, val versionWa: String, val versionCliente: String, val enMarcha: Boolean,
    val botonesHora: Int = 40,
)

/** Un mensaje capturado: solo estructura (nombres de campos y largos). */
data class ItemCaptura(val n: Int, val ts: Long, val chat: String, val clasif: String, val detalle: String, val arbol: String)

/** Estado de la captura de diagnóstico. */
data class EstadoCaptura(val activo: Boolean, val mostrarTextos: Boolean, val restanSeg: Int, val capturados: Int, val max: Int, val items: List<ItemCaptura>) {
    /** Todo el texto, listo para pegar. */
    fun textoCompleto(): String = items.joinToString("\n\n") { "#${it.n} · ${it.chat} · ${it.clasif} · ${it.detalle}\n${it.arbol}" }
}

object JsonLab {
    fun captura(json: String): EstadoCaptura {
        val o = JSONObject(json)
        val a = o.optJSONArray("items") ?: JSONArray()
        return EstadoCaptura(
            o.optBoolean("activo"), o.optBoolean("mostrarTextos"), o.optInt("restanSeg"), o.optInt("capturados"), o.optInt("max", 30),
            List(a.length()) { i -> a.getJSONObject(i).let { ItemCaptura(it.optInt("n"), it.optLong("ts"), it.optString("chat"), it.optString("clasif"), it.optString("detalle"), it.optString("arbol")) } },
        )
    }

    fun estado(json: String): EstadoLab {
        val o = JSONObject(json)
        return EstadoLab(
            o.optBoolean("activo"), o.optBoolean("avisoAceptado"), o.optString("modo", "texto"), o.optString("modoReal", "texto"), o.optBoolean("grupos"),
            o.optInt("encMax", 12), o.optInt("expiraMin", 120), o.optBoolean("respaldoEnc"), o.optBoolean("respaldoBot"), o.optString("botonesTipo", "auto"),
            o.optString("versionWa"), o.optString("versionCliente"), o.optBoolean("enMarcha"), o.optInt("botonesHora", 40),
        )
    }
}
