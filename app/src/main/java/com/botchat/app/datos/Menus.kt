package com.botchat.app.datos

import org.json.JSONArray
import org.json.JSONObject

/** Una respuesta de un formulario recibido. */
data class CampoRecibido(val id: String, val etiqueta: String, val valor: String)

/** Un formulario que llenó alguien dentro de un menú (los datos viven cifrados en el teléfono). */
data class FormularioRecibido(
    val id: Long, val menuId: Long, val menu: String, val cliente: String, val chat: String, val persona: String, val ts: Long, val campos: List<CampoRecibido>,
)

/** Un mensaje que el menú no entendió. */
data class NoEntendio(val id: Long, val menuId: Long, val menu: String, val paso: String, val ts: Long, val texto: String)

/** Resultado de importar menús: los que se crearon (apagados) y cuántos no se pudieron leer. */
data class ImportMenus(val ids: List<Long>, val nombres: List<String>, val avisos: Int, val omitidos: Int)

/** Un archivo guardado (cifrado) para mandarlo desde un menú. */
data class AdjuntoSubido(val id: String, val tipo: String, val mime: String, val nombre: String, val bytes: Long)

object JsonMenus {
    private fun <T> lista(json: String, f: (JSONObject) -> T): List<T> {
        val a = JSONArray(json.ifBlank { "[]" })
        return List(a.length()) { f(a.getJSONObject(it)) }
    }

    fun ajustes(json: String): Map<String, String> {
        val o = JSONObject(json.ifBlank { "{}" })
        return o.keys().asSequence().associateWith { o.optString(it) }
    }

    fun formulario(o: JSONObject): FormularioRecibido {
        val cs = o.optJSONArray("campos")
        return FormularioRecibido(
            o.optLong("id"), o.optLong("menuId"), o.optString("menu"), o.optString("cliente"), o.optString("chat"), o.optString("persona"), o.optLong("ts"),
            List(cs?.length() ?: 0) { i -> cs!!.getJSONObject(i).let { CampoRecibido(it.optString("id"), it.optString("etiqueta"), it.optString("valor")) } },
        )
    }

    fun formularios(json: String) = lista(json) { formulario(it) }
    fun formulario(json: String) = formulario(JSONObject(json))
    fun noEntendio(json: String) = lista(json) { NoEntendio(it.optLong("id"), it.optLong("menuId"), it.optString("menu"), it.optString("paso"), it.optLong("ts"), it.optString("texto")) }

    fun importado(json: String): ImportMenus {
        val o = JSONObject(json)
        val ids = o.optJSONArray("ids")
        val nombres = o.optJSONArray("nombres")
        return ImportMenus(
            List(ids?.length() ?: 0) { ids!!.getLong(it) }, List(nombres?.length() ?: 0) { nombres!!.getString(it) },
            o.optInt("avisos"), o.optInt("omitidos"),
        )
    }

    fun adjunto(json: String): AdjuntoSubido {
        val o = JSONObject(json)
        return AdjuntoSubido(o.optString("id"), o.optString("tipo"), o.optString("mime"), o.optString("nombre"), o.optLong("bytes"))
    }
}
