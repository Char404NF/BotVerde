package com.botchat.app.datos

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject

// Perfil de persona, info del grupo, silencio en grupos, advertencias con tipo y marcas (0.6.0, etapa E7).
// Todo se calcula en el módulo Go; aquí solo viven los modelos, su lectura y las llamadas.

data class GrupoComunItem(val jid: String, val nombre: String, val foto: String)

data class PerfilPersonaItem(
    val persona: String, val nombre: String, val telefono: String, val rango: String, val rangoColor: String, val foto: String,
    val etiquetas: List<EtiquetaItem>, val mensajes: Int, val archivos: Int, val archivosBytes: Long, val gruposComun: List<GrupoComunItem>,
    val ficha: FichaPersona?,
)

/** Una tarjeta de «Actividad»: el chat privado o un grupo, con lo que la persona escribió ahí. */
data class TarjetaActividad(
    val chat: String, val nombre: String, val foto: String, val privado: Boolean, val miembro: Boolean,
    val mensajes: Int, val fotos: Int, val videos: Int, val audios: Int, val archivos: Int, val enlaces: Int, val misEnvios: Int, val ultimoTs: Long,
) {
    val multimedia: Int get() = fotos + videos
}

/** Un mensaje de la lista de actividad, con su marca roja (borrado o editado) y sus marcas mías. */
data class MensajeActividad(
    val msg: Msg, val marcaRoja: String, val borradoPor: String, val versiones: Int, val marcado: Boolean, val etiquetas: List<Long>,
)

data class PaginaActividad(val total: Long, val items: List<MensajeActividad>, val fin: Boolean, val sigTs: Long, val sigId: String)

/** Lo que se busca en la actividad de una persona. Pestañas: mensajes, fotos, audios, archivos. */
data class FiltroActividad(
    val chat: String = "", val persona: String = "", val pestana: String = "mensajes", val desde: Long = 0, val hasta: Long = 0, val palabra: String = "",
    val soloMarcados: Boolean = false, val soloBorrados: Boolean = false, val soloEditados: Boolean = false,
) {
    fun json(): String = JSONObject().put("chat", chat).put("persona", persona).put("pestana", pestana).put("desde", desde).put("hasta", hasta)
        .put("palabra", palabra).put("soloMarcados", soloMarcados).put("soloBorrados", soloBorrados).put("soloEditados", soloEditados).toString()
}

data class MiembroDetalle(
    val persona: String, val jid: String, val nombre: String, val telefono: String, val rol: String, val rango: String, val rangoColor: String,
    val desdeTs: Long, val hastaTs: Long, val mensajes: Int, val ultimoTs: Long, val silenciado: Boolean, val advertido: Boolean, val etiquetas: List<Long>, val foto: String,
) {
    val esAdmin: Boolean get() = rol == "admin" || rol == "creador"
}

data class CambioMiembro(val id: Long, val grupo: String, val ts: Long, val tipo: String, val persona: String, val nombre: String, val por: String, val porNombre: String, val via: String)

data class ContadorSilencio(
    val usado: Int, val tope: Int, val color: String, val total: Int, val avisos: Int, val proximoReinicio: Long, val grupos: List<Pair<String, Int>>,
)

data class SilencioGrupoItem(
    val id: Long, val persona: String, val nombre: String, val alcance: String, val grupos: List<String>, val hastaTs: Long, val creadoTs: Long,
    val origen: String, val avisar: Boolean, val texto: String, val contador: ContadorSilencio?,
)

data class TipoAdvertencia(val id: Long, val nombre: String, val emoji: String, val color: String, val posicion: Int, val caducidadDias: Int, val reglaId: Long, val activo: Boolean)

data class ReglaGrupo(val id: Long, val grupo: String, val pos: Int, val titulo: String, val texto: String)

/** Qué pasó al advertir con un tipo: `hecho` = avisado | silenciado | borrado | expulsado | solo_aviso; `paso` = la acción de la escalera que tocó. */
data class ResultadoAdvertenciaTipo(
    val total: Int, val nivel: Int, val paso: String, val hecho: String, val motivo: String, val regla: String, val sancionId: Long,
)

data class MarcaMensaje(val etiquetaId: Long, val nota: String, val ts: Long)
data class VersionMensaje(val n: Int, val ts: Long, val texto: String)
data class EstadoPersonaLista(val silenciado: Boolean, val ignorada: Boolean, val advertido: Boolean)

object JsonE7 {
    private fun <T> lista(json: String, f: (JSONObject) -> T): List<T> {
        val a = JSONArray(json.ifBlank { "[]" })
        return List(a.length()) { f(a.getJSONObject(it)) }
    }

    private fun longs(a: JSONArray?) = List(a?.length() ?: 0) { a!!.getLong(it) }
    private fun strs(a: JSONArray?) = List(a?.length() ?: 0) { a!!.getString(it) }

    fun perfil(json: String): PerfilPersonaItem {
        val o = JSONObject(json.ifBlank { "{}" })
        val et = o.optJSONArray("etiquetas")
        val gc = o.optJSONArray("gruposComun")
        return PerfilPersonaItem(
            o.optString("persona"), o.optString("nombre"), o.optString("telefono"), o.optString("rango"), o.optString("rangoColor"), o.optString("foto"),
            List(et?.length() ?: 0) { JsonPersonas.etiqueta(et!!.getJSONObject(it)) }, o.optInt("mensajes"), o.optInt("archivos"), o.optLong("archivosBytes"),
            List(gc?.length() ?: 0) { gc!!.getJSONObject(it).let { g -> GrupoComunItem(g.optString("jid"), g.optString("nombre"), g.optString("foto")) } },
            o.optJSONObject("ficha")?.let { runCatching { JsonPersonas.ficha(it.toString()) }.getOrNull() },
        )
    }

    fun actividad(json: String) = lista(json) {
        TarjetaActividad(
            it.optString("chat"), it.optString("nombre"), it.optString("foto"), it.optBoolean("privado"), it.optBoolean("miembro"),
            it.optInt("mensajes"), it.optInt("fotos"), it.optInt("videos"), it.optInt("audios"), it.optInt("archivos"), it.optInt("enlaces"), it.optInt("misEnvios"), it.optLong("ultimoTs"),
        )
    }

    fun paginaActividad(json: String): PaginaActividad {
        val o = JSONObject(json.ifBlank { "{}" })
        val a = o.optJSONArray("items") ?: JSONArray()
        val items = List(a.length()) { i ->
            val x = a.getJSONObject(i)
            MensajeActividad(
                Json.mensajes("[$x]").first(), x.optString("marcaRoja"), x.optString("borradoPor"), x.optInt("versiones"), x.optBoolean("marcado"), longs(x.optJSONArray("etiquetas")),
            )
        }
        return PaginaActividad(o.optLong("total"), items, o.optBoolean("fin", true), o.optLong("sigTs"), o.optString("sigId"))
    }

    fun miembros(json: String) = lista(json) {
        MiembroDetalle(
            it.optString("persona"), it.optString("jid"), it.optString("nombre"), it.optString("telefono"), it.optString("rol"), it.optString("rango"), it.optString("rangoColor"),
            it.optLong("desdeTs"), it.optLong("hastaTs"), it.optInt("mensajes"), it.optLong("ultimoTs"), it.optBoolean("silenciado"), it.optBoolean("advertido"),
            longs(it.optJSONArray("etiquetas")), it.optString("foto"),
        )
    }

    fun cambios(json: String) = lista(json) {
        CambioMiembro(it.optLong("id"), it.optString("grupo"), it.optLong("ts"), it.optString("tipo"), it.optString("persona"), it.optString("nombre"), it.optString("por"), it.optString("porNombre"), it.optString("via"))
    }

    fun contador(json: String): ContadorSilencio = contador(JSONObject(json.ifBlank { "{}" }))
    fun contador(o: JSONObject): ContadorSilencio {
        val g = o.optJSONArray("grupos")
        return ContadorSilencio(
            o.optInt("usado"), o.optInt("tope"), o.optString("color", "verde"), o.optInt("total"), o.optInt("avisos"), o.optLong("proximoReinicio"),
            List(g?.length() ?: 0) { g!!.getJSONObject(it).let { x -> x.optString("grupo") to x.optInt("n") } },
        )
    }

    fun silencios(json: String) = lista(json) {
        SilencioGrupoItem(
            it.optLong("id"), it.optString("persona"), it.optString("nombre"), it.optString("alcance"), strs(it.optJSONArray("grupos")), it.optLong("hastaTs"), it.optLong("creadoTs"),
            it.optString("origen"), it.optBoolean("avisar"), it.optString("texto"), it.optJSONObject("contador")?.let { c -> contador(c) },
        )
    }

    fun tipos(json: String) = lista(json) {
        TipoAdvertencia(it.optLong("id"), it.optString("nombre"), it.optString("emoji"), it.optString("color", "#F59E28"), it.optInt("posicion"), it.optInt("caducidadDias", -1), it.optLong("reglaId"), it.optBoolean("activo", true))
    }

    fun reglas(json: String) = lista(json) { ReglaGrupo(it.optLong("id"), it.optString("grupo"), it.optInt("pos"), it.optString("titulo"), it.optString("texto")) }

    fun resultadoTipo(json: String): ResultadoAdvertenciaTipo {
        val o = JSONObject(json.ifBlank { "{}" })
        return ResultadoAdvertenciaTipo(o.optInt("total"), o.optInt("nivel"), o.optString("paso"), o.optString("hecho"), o.optString("motivo"), o.optString("regla"), o.optLong("sancionId"))
    }

    fun marcas(json: String) = lista(json) { MarcaMensaje(it.optLong("etiquetaId"), it.optString("nota"), it.optLong("ts")) }
    fun versiones(json: String) = lista(json) { VersionMensaje(it.optInt("n"), it.optLong("ts"), it.optString("texto")) }

    fun estados(json: String): Map<String, EstadoPersonaLista> {
        val o = JSONObject(json.ifBlank { "{}" })
        return o.keys().asSequence().associateWith { k -> o.getJSONObject(k).let { EstadoPersonaLista(it.optBoolean("silenciado"), it.optBoolean("ignorada"), it.optBoolean("advertido")) } }
    }

    fun puntitos(json: String): Map<String, List<Long>> {
        val o = JSONObject(json.ifBlank { "{}" })
        return o.keys().asSequence().associateWith { k -> longs(o.optJSONArray(k)) }
    }

    /** Textos editables del silencio: id -> (actual, defecto). */
    fun textosSilencio(json: String): Map<String, Pair<String, String>> {
        val t = JSONObject(json.ifBlank { "{}" }).optJSONObject("textos") ?: return emptyMap()
        return t.keys().asSequence().associateWith { k -> t.getJSONObject(k).let { it.optString("actual") to it.optString("defecto") } }
    }
}

private suspend fun <T> io(f: () -> T): T = withContext(Dispatchers.IO) { f() }

// ---- llamadas (extensiones de Repo, para no engordar su archivo) ----

suspend fun Repo.perfilPersona(bot: Long, persona: String) = io { JsonE7.perfil(motor.perfilPersona(bot, persona)) }
suspend fun Repo.actividadPersona(bot: Long, persona: String) = io { JsonE7.actividad(motor.actividadPersona(bot, persona)) }
suspend fun Repo.mensajesDePersona(bot: Long, filtro: FiltroActividad, antesTs: Long = 0, antesId: String = "", limite: Int = 40) =
    io { JsonE7.paginaActividad(motor.mensajesDePersona(bot, filtro.json(), antesTs, antesId, limite.toLong())) }
suspend fun Repo.versionesDeMensaje(bot: Long, chat: String, id: String) = io { JsonE7.versiones(motor.versionesDeMensaje(bot, chat, id)) }
suspend fun Repo.marcasDeMensaje(bot: Long, chat: String, id: String) = io { JsonE7.marcas(motor.marcasDeMensaje(bot, chat, id)) }
suspend fun Repo.marcarMensaje(bot: Long, chat: String, id: String, etiqueta: Long, nota: String) = io { motor.marcarMensaje(bot, chat, id, etiqueta, nota) }
suspend fun Repo.quitarMarcaMensaje(bot: Long, chat: String, id: String, etiqueta: Long) = io { motor.quitarMarcaMensaje(bot, chat, id, etiqueta) }
suspend fun Repo.ponerEtiquetaPersona(bot: Long, persona: String, etiqueta: Long, si: Boolean) = io { motor.ponerEtiquetaPersona(bot, persona, etiqueta, si) }
suspend fun Repo.miembrosDetallado(bot: Long, grupo: String, incluirSalidos: Boolean = false) = io { JsonE7.miembros(motor.miembrosGrupoDetallado(bot, grupo, incluirSalidos)) }
suspend fun Repo.cambiosMiembros(bot: Long, grupo: String, antes: Long = 0, limite: Int = 60) = io { JsonE7.cambios(motor.cambiosMiembrosGrupo(bot, grupo, antes, limite.toLong())) }
suspend fun Repo.actualizarMiembros(bot: Long) = io { motor.actualizarMiembrosGrupo(bot) }
suspend fun Repo.puntitosEnBruto(bot: Long, chat: String, ids: List<String>) = io { motor.puntitosDeMensajes(bot, chat, JSONArray(ids).toString()) }
suspend fun Repo.puntitosDeMensajes(bot: Long, chat: String, ids: List<String>) = io { JsonE7.puntitos(motor.puntitosDeMensajes(bot, chat, JSONArray(ids).toString())) }
suspend fun Repo.estadosEnBruto(bot: Long, personas: List<String>) = io { motor.estadosDePersonas(bot, JSONArray(personas).toString()) }
suspend fun Repo.estadosDePersonas(bot: Long, personas: List<String>) = io { JsonE7.estados(motor.estadosDePersonas(bot, JSONArray(personas).toString())) }

suspend fun Repo.silenciarEnGrupos(bot: Long, persona: String, segundos: Long, alcance: String, grupos: List<String>, avisar: Boolean, texto: String) =
    io { motor.silenciarEnGrupos(bot, persona, segundos, alcance, JSONArray(grupos).toString(), avisar, texto) }
suspend fun Repo.quitarSilencioGrupos(bot: Long, persona: String) = io { motor.quitarSilencioGrupos(bot, persona) }
suspend fun Repo.silenciosGrupos(bot: Long) = io { JsonE7.silencios(motor.listarSilenciosGrupos(bot)) }
suspend fun Repo.contadorSilencio(bot: Long, persona: String, grupo: String = "") = io { JsonE7.contador(motor.contadorSilencio(bot, persona, grupo)) }
suspend fun Repo.ponerContadorEnCero(bot: Long, persona: String, grupo: String = "") = io { motor.ponerContadorEnCero(bot, persona, grupo) }
suspend fun Repo.textosSilencio(bot: Long) = io { JsonE7.textosSilencio(motor.ajustesSilencio(bot)) }

suspend fun Repo.tiposAdvertencia(bot: Long) = io { JsonE7.tipos(motor.listarTiposAdvertencia(bot)).filter { it.activo } }
suspend fun Repo.reglasGrupoLista(bot: Long, grupo: String) = io { JsonE7.reglas(motor.listarReglasGrupo(bot, grupo)) }
suspend fun Repo.advertirConTipo(bot: Long, persona: String, chat: String, msgId: String, motivo: String, tipoId: Long, reglaId: Long) =
    io { JsonE7.resultadoTipo(motor.advertirConTipo(bot, persona, chat, msgId, motivo, tipoId, reglaId)) }
suspend fun Repo.quitarUltimaAdvertencia(bot: Long, persona: String) = io { motor.quitarAdvertenciaPersona(bot, persona, 0) }
