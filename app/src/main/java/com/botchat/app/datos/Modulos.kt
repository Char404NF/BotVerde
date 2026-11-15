package com.botchat.app.datos

import org.json.JSONArray
import org.json.JSONObject

/** Tipos de campo de la configuración genérica de un módulo. */
object TipoCampo {
    const val TEXTO = "texto"
    const val NUMERO = "numero"
    const val INTERRUPTOR = "interruptor"
    const val LISTA = "lista"
    const val HORA = "hora"
    const val ADJUNTOS = "adjuntos" // solo en los formularios de los paneles
    const val SECRETO = "secreto" // llave: va al Keystore, nunca a la configuración
}

/** Un botón de un panel: `accion` llega a Go; `confirma`, si no está vacío, se pregunta antes. */
data class BotonPanel(val accion: String, val titulo: String, val confirma: String = "")

/** Una lista con su formulario (destinatarios, programados, conectores…) que la ventana genérica dibuja bajo los ajustes del módulo. */
data class PanelModulo(
    val id: String, val titulo: String, val ayuda: String, val campos: List<CampoModulo>, val alta: String,
    val extras: List<BotonPanel>, val porFila: List<BotonPanel>, val generales: List<BotonPanel>,
)

/** Un renglón de un panel. */
data class FilaPanel(val id: String, val titulo: String, val detalle: String)

/** Un ajuste simple de un módulo (la ventana genérica lo dibuja según su tipo). */
data class CampoModulo(
    val clave: String, val titulo: String, val ayuda: String, val tipo: String, val defecto: Any?,
    val min: Double = 0.0, val max: Double = 0.0, val largo: Int = 0, val varias: Boolean = false,
    val opciones: List<Pair<String, String>> = emptyList(), val obligatorio: Boolean = false,
)

/** Una tarjeta de la lista de módulos. `codigo`: apagado | activo | pausado_seguro | esperando_maestro. */
data class ModuloInfo(
    val id: String, val nombre: String, val descripcion: String, val icono: String, val riesgo: String, val iniciativa: Boolean,
    val activo: Boolean, val pausado: Boolean, val codigo: String, val estado: String, val conEsquema: Boolean,
    val topeHora: Int, val topeDia: Int, val defHora: Int, val defDia: Int, val activadoTs: Long, val esquema: List<CampoModulo> = emptyList(),
    val paneles: List<PanelModulo> = emptyList(),
    /** Si no está vacío, el módulo está BLOQUEADO (no se puede encender) hasta que se cumpla eso; por ejemplo, descargar su motor. */
    val bloqueo: String = "",
) {
    val riesgoAlto get() = riesgo == "alto"
    val pideAviso get() = riesgo == "medio" || riesgo == "alto"
}

data class NoMolestarItem(val persona: String, val desdeTs: Long, val motivo: String, val confirmado: Boolean)
data class RegistroIniciativa(val id: Long, val modulo: String, val que: String, val ts: Long, val resultado: String, val chat: String)
data class ModoSeguro(val activo: Boolean, val ts: Long, val motivo: String, val texto: String, val errores: Int)

object JsonMod {
    private fun <T> lista(json: String, f: (JSONObject) -> T): List<T> {
        val a = JSONArray(json.ifBlank { "[]" })
        return List(a.length()) { f(a.getJSONObject(it)) }
    }

    fun campo(o: JSONObject): CampoModulo {
        val ops = o.optJSONArray("opciones")
        return CampoModulo(
            o.optString("clave"), o.optString("titulo"), o.optString("ayuda"), o.optString("tipo", TipoCampo.TEXTO), if (o.isNull("defecto")) null else o.get("defecto"),
            o.optDouble("min", 0.0), o.optDouble("max", 0.0), o.optInt("largo"), o.optBoolean("varias"),
            List(ops?.length() ?: 0) { i -> ops!!.getJSONObject(i).let { it.optString("valor") to it.optString("titulo") } }, o.optBoolean("obligatorio"),
        )
    }

    private fun boton(o: JSONObject) = BotonPanel(o.optString("accion"), o.optString("titulo"), o.optString("confirma"))

    private fun botones(a: org.json.JSONArray?) = List(a?.length() ?: 0) { boton(a!!.getJSONObject(it)) }

    fun panel(o: JSONObject): PanelModulo {
        val cs = o.optJSONArray("campos")
        return PanelModulo(
            o.optString("id"), o.optString("titulo"), o.optString("ayuda"), List(cs?.length() ?: 0) { campo(cs!!.getJSONObject(it)) }, o.optString("alta"),
            botones(o.optJSONArray("extras")), botones(o.optJSONArray("porFila")), botones(o.optJSONArray("generales")),
        )
    }

    fun filasPanel(json: String) = lista(json) { FilaPanel(it.optString("id"), it.optString("titulo"), it.optString("detalle")) }

    fun modulo(o: JSONObject): ModuloInfo {
        val esq = o.optJSONArray("esquema")
        val pan = o.optJSONArray("paneles")
        return ModuloInfo(
            o.optString("id"), o.optString("nombre"), o.optString("descripcion"), o.optString("icono"), o.optString("riesgo", "bajo"), o.optBoolean("iniciativa"),
            o.optBoolean("activo"), o.optBoolean("pausado"), o.optString("codigo", "apagado"), o.optString("estado"), o.optBoolean("conEsquema"),
            o.optInt("topeHora"), o.optInt("topeDia"), o.optInt("defHora"), o.optInt("defDia"), o.optLong("activadoTs"),
            List(esq?.length() ?: 0) { campo(esq!!.getJSONObject(it)) },
            List(pan?.length() ?: 0) { panel(pan!!.getJSONObject(it)) },
            o.optString("bloqueo"),
        )
    }

    fun modulos(json: String) = lista(json, ::modulo)
    fun moduloOpt(json: String): ModuloInfo? = if (json.isBlank() || json == "null") null else modulo(JSONObject(json))

    /** La configuración de un módulo como mapa (números como Double, interruptores como Boolean). */
    fun config(json: String): Map<String, Any?> {
        val o = JSONObject(json.ifBlank { "{}" })
        val m = LinkedHashMap<String, Any?>()
        o.keys().forEach { k -> m[k] = o.get(k) }
        return m
    }

    fun ajustes(json: String): Map<String, String> {
        val o = JSONObject(json.ifBlank { "{}" })
        val m = LinkedHashMap<String, String>()
        o.keys().forEach { k -> m[k] = o.optString(k) }
        return m
    }

    fun noMolestar(json: String) = lista(json) { NoMolestarItem(it.optString("persona"), it.optLong("desdeTs"), it.optString("motivo"), it.optBoolean("confirmado")) }
    fun registro(json: String) = lista(json) { RegistroIniciativa(it.optLong("id"), it.optString("modulo"), it.optString("que"), it.optLong("ts"), it.optString("resultado"), it.optString("chat")) }
    fun seguro(json: String): ModoSeguro {
        val o = JSONObject(json.ifBlank { "{}" })
        return ModoSeguro(o.optBoolean("activo"), o.optLong("ts"), o.optString("motivo"), o.optString("texto"), o.optInt("errores"))
    }
}
