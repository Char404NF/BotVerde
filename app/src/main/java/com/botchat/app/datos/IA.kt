package com.botchat.app.datos

import org.json.JSONArray

/** Un modelo que se sugiere de un servicio de IA. */
data class ModeloIA(val id: String, val nombre: String, val nota: String = "")

/** Un servicio de IA que se elige con un toque (lo arma el motor: dirección, formato y modelos). */
data class ProveedorIA(
    val id: String, val nombre: String, val detalle: String, val url: String, val formato: String,
    val modelos: List<ModeloIA>, val defecto: String, val urlLlave: String, val gratis: Boolean, val sinLlave: Boolean,
    val local: Boolean, val color: String, val inicial: String, val icono: String, val pista: String,
)

object JsonIA {
    fun proveedores(raw: String): List<ProveedorIA> = runCatching {
        val a = JSONArray(raw)
        (0 until a.length()).map { i ->
            val o = a.getJSONObject(i)
            val ms = o.optJSONArray("modelos") ?: JSONArray()
            ProveedorIA(
                o.getString("id"), o.optString("nombre"), o.optString("detalle"), o.optString("url"), o.optString("formato", "openai"),
                (0 until ms.length()).map { j -> ms.getJSONObject(j).let { ModeloIA(it.getString("id"), it.optString("nombre"), it.optString("nota")) } },
                o.optString("defecto"), o.optString("url_llave"), o.optBoolean("gratis"), o.optBoolean("sin_llave"), o.optBoolean("local"),
                o.optString("color", "#607D8B"), o.optString("inicial", "?"), o.optString("icono"), o.optString("pista"),
            )
        }
    }.getOrDefault(emptyList())
}
