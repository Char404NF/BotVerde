package com.botchat.app.datos

import org.json.JSONArray
import org.json.JSONObject

// Grupos y estado de la app de WhatsApp (0.5.0, bloque E). Todo se decide en el módulo Go; aquí solo viven los modelos y su lectura.

/** Si el bot sigue en el grupo y si es administrador. `motivo` explica en español qué no se puede (vacío si todo está bien). */
data class EstadoGrupo(val conectado: Boolean, val enGrupo: Boolean, val soyAdmin: Boolean, val motivo: String)

data class MiembroGrupo(
    val jid: String, val persona: String, val nombre: String, val telefono: String,
    val esAdmin: Boolean, val esCreador: Boolean, val yo: Boolean,
)

data class InfoGrupo(
    val chat: String, val nombre: String, val descripcion: String, val creado: Long, val creadoTexto: String, val creadoPor: String,
    val miembros: Int, val admins: List<MiembroGrupo>, val soyAdmin: Boolean, val soloAdmins: Boolean, val infoBloqueada: Boolean,
    val aprobacionIngreso: Boolean, val mensajesTemporales: Int, val reglas: String,
)

data class SolicitudGrupo(val jid: String, val persona: String, val nombre: String, val telefono: String, val ts: Long)

data class FilaRankingGrupo(val puesto: Int, val persona: String, val nombre: String, val telefono: String, val mensajes: Int, val porcentaje: Double, val yo: Boolean)
data class RankingGrupo(val dias: Int, val total: Int, val filas: List<FilaRankingGrupo>, val soloMiembros: Boolean, val desde: String, val aviso: String)

data class InactivoGrupo(val persona: String, val nombre: String, val telefono: String, val ultimoDia: String, val diasSin: Int, val esAdmin: Boolean)
data class InactivosGrupo(val dias: Int, val aviso: String, val lista: List<InactivoGrupo>)

data class ResultadoTodos(val mencionadas: Int, val total: Int, val mensajes: Int, val truncado: Boolean, val aviso: String)

/** Opciones de un filtro. Las listas y números que no aplican a ese filtro van vacíos o en 0. */
data class ConfigFiltroGrupo(
    val exentoAdmins: Boolean = true, val exentoRangos: List<Long> = emptyList(), val permitidos: List<String> = emptyList(),
    val palabras: List<String> = emptyList(), val max: Int = 0, val segundos: Int = 0, val silencioMin: Int = 60, val aviso: String = "",
)

data class FiltroGrupo(
    val chat: String, val filtro: String, val nombre: String, val descripcion: String, val activo: Boolean, val accion: String, val config: ConfigFiltroGrupo,
) {
    fun json(): String = JSONObject().put("chat", chat).put("filtro", filtro).put("activo", activo).put("accion", accion)
        .put("config", JSONObject().put("exentoAdmins", config.exentoAdmins).put("exentoRangos", JSONArray(config.exentoRangos)).put("permitidos", JSONArray(config.permitidos))
            .put("palabras", JSONArray(config.palabras)).put("max", config.max).put("segundos", config.segundos).put("silencioMin", config.silencioMin).put("aviso", config.aviso)).toString()
}

/** Si se puede silenciar, fijar y archivar en MI WhatsApp (hacen falta las llaves que comparte el teléfono principal). */
data class EstadoAppState(val disponible: Boolean, val conectado: Boolean, val motivo: String)

/** Las acciones que puede hacer un filtro, en el orden de la pantalla, con su texto y si necesitan ser administrador. */
data class AccionFiltro(val id: String, val texto: String, val ayuda: String, val requiereAdmin: Boolean)

val ACCIONES_FILTRO = listOf(
    AccionFiltro("avisarme", "Avisarme", "Te llega una notificación en este teléfono.", false),
    AccionFiltro("avisar_grupo", "Avisar al grupo", "Respondo al mensaje con un aviso amable.", false),
    AccionFiltro("borrar", "Borrar el mensaje", "Borra el mensaje para todos.", true),
    AccionFiltro("advertir", "Advertir", "Pone una advertencia a la persona.", false),
    AccionFiltro("silenciar", "Ignorarla (el bot no le contesta)", "El bot deja de contestarle un rato.", false),
    AccionFiltro("expulsar", "Expulsar", "La saca del grupo.", true),
)

fun textoAccionFiltro(id: String) = ACCIONES_FILTRO.firstOrNull { it.id == id }?.texto ?: id

object JsonGrupos {
    private fun <T> lista(json: String, f: (JSONObject) -> T): List<T> {
        val a = JSONArray(json.ifBlank { "[]" })
        return List(a.length()) { f(a.getJSONObject(it)) }
    }
    private fun strs(a: JSONArray?) = List(a?.length() ?: 0) { a!!.getString(it) }
    private fun longs(a: JSONArray?) = List(a?.length() ?: 0) { a!!.getLong(it) }

    fun estado(json: String): EstadoGrupo {
        val o = JSONObject(json)
        return EstadoGrupo(o.optBoolean("conectado"), o.optBoolean("enGrupo"), o.optBoolean("soyAdmin"), o.optString("motivo"))
    }

    fun miembro(o: JSONObject) = MiembroGrupo(o.optString("jid"), o.optString("persona"), o.optString("nombre"), o.optString("telefono"),
        o.optBoolean("esAdmin"), o.optBoolean("esCreador"), o.optBoolean("yo"))
    fun miembros(json: String) = lista(json) { miembro(it) }

    fun info(json: String): InfoGrupo {
        val o = JSONObject(json)
        return InfoGrupo(
            o.optString("chat"), o.optString("nombre"), o.optString("descripcion"), o.optLong("creado"), o.optString("creadoTexto"), o.optString("creadoPor"),
            o.optInt("miembros"), List(o.optJSONArray("admins")?.length() ?: 0) { miembro(o.getJSONArray("admins").getJSONObject(it)) },
            o.optBoolean("soyAdmin"), o.optBoolean("soloAdmins"), o.optBoolean("infoBloqueada"), o.optBoolean("aprobacionIngreso"),
            o.optInt("mensajesTemporales"), o.optString("reglas"),
        )
    }

    fun solicitudes(json: String) = lista(json) { SolicitudGrupo(it.optString("jid"), it.optString("persona"), it.optString("nombre"), it.optString("telefono"), it.optLong("ts")) }

    fun ranking(json: String): RankingGrupo {
        val o = JSONObject(json)
        val filas = List(o.optJSONArray("filas")?.length() ?: 0) {
            val f = o.getJSONArray("filas").getJSONObject(it)
            FilaRankingGrupo(f.optInt("puesto"), f.optString("persona"), f.optString("nombre"), f.optString("telefono"), f.optInt("mensajes"), f.optDouble("porcentaje"), f.optBoolean("yo"))
        }
        return RankingGrupo(o.optInt("dias"), o.optInt("total"), filas, o.optBoolean("soloMiembros"), o.optString("desde"), o.optString("aviso"))
    }

    fun inactivos(json: String): InactivosGrupo {
        val o = JSONObject(json)
        val l = List(o.optJSONArray("inactivos")?.length() ?: 0) {
            val f = o.getJSONArray("inactivos").getJSONObject(it)
            InactivoGrupo(f.optString("persona"), f.optString("nombre"), f.optString("telefono"), f.optString("ultimoDia"), f.optInt("diasSin", -1), f.optBoolean("esAdmin"))
        }
        return InactivosGrupo(o.optInt("dias"), o.optString("aviso"), l)
    }

    fun todos(json: String): ResultadoTodos {
        val o = JSONObject(json)
        return ResultadoTodos(o.optInt("mencionadas"), o.optInt("total"), o.optInt("mensajes"), o.optBoolean("truncado"), o.optString("aviso"))
    }

    fun filtro(o: JSONObject): FiltroGrupo {
        val c = o.optJSONObject("config") ?: JSONObject()
        return FiltroGrupo(
            o.optString("chat"), o.optString("filtro"), o.optString("nombre"), o.optString("descripcion"), o.optBoolean("activo"), o.optString("accion", "avisarme"),
            ConfigFiltroGrupo(c.optBoolean("exentoAdmins", true), longs(c.optJSONArray("exentoRangos")), strs(c.optJSONArray("permitidos")), strs(c.optJSONArray("palabras")),
                c.optInt("max"), c.optInt("segundos"), c.optInt("silencioMin", 60), c.optString("aviso")),
        )
    }
    fun filtros(json: String) = lista(json) { filtro(it) }

    fun appState(json: String): EstadoAppState {
        val o = JSONObject(json)
        return EstadoAppState(o.optBoolean("disponible"), o.optBoolean("conectado"), o.optString("motivo"))
    }
}
