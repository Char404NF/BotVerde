package com.botchat.app.datos

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject

// Almacenamiento, auditoría y privacidad, silencios, advertencias con escalera y marcados (0.6.0, etapa E8).
// Todo se calcula en el módulo Go; aquí solo viven los modelos, su lectura y las llamadas.

/** Cuánto ocupa un tipo de archivo y sus reglas: interruptor, tope por archivo y tope para todo el tipo (0 = sin tope). */
data class TipoAlmacen(val tipo: String, val bytes: Long, val cantidad: Int, val apagado: Boolean, val topeArchivo: Long, val topeTotal: Long)

data class EstadoAlmacen(
    val topeTotal: Long, val uso: Long, val libre: Long, val total: Long, val ahorradoBytes: Long, val ahorradoArchivos: Int,
    val conservarPortada: Boolean, val sinPortada: Int, val tipos: List<TipoAlmacen>,
) {
    /** 0 a 1 contra el tope (0 si no hay tope). */
    val fraccion: Float get() = if (topeTotal > 0) (uso.toFloat() / topeTotal).coerceIn(0f, 1f) else 0f
    val pasadoDelTope: Boolean get() = topeTotal > 0 && uso > topeTotal
    /** verde (bien), amarillo (cerca del tope) o rojo (pasado). */
    val color: String get() = when { topeTotal <= 0 -> "verde"; uso > topeTotal -> "rojo"; uso >= topeTotal * 0.8 -> "amarillo"; else -> "verde" }
}

/** Qué quiere limpiar el dueño: tipo, persona y fechas (segundos; 0 = sin límite). */
data class FiltroLimpieza(val categoria: String = "", val persona: String = "", val desde: Long = 0, val hasta: Long = 0, val chat: String = "") {
    fun json(): String = JSONObject().put("chat", chat).put("categoria", categoria).put("diasMin", 0).put("persona", persona).put("desde", desde).put("hasta", hasta).toString()
}

data class AuditoriaGlobal(val defecto: Boolean, val retDias: Int, val retPendiente: Boolean, val avisoVisto: Boolean)

data class AuditoriaChatItem(
    val chat: String, val nombre: String, val esGrupo: Boolean, val ultimoTs: Long, val audit: Int, val efectivo: Boolean, val retDias: Int, val retEfectiva: Int,
)

data class SancionPendiente(val id: Long, val origen: String, val persona: String, val grupo: String, val avisoId: Long, val creadoTs: Long, val nombre: String, val nombreGrupo: String)

data class AjustesSilencio(val ajustes: Map<String, String>, val textos: Map<String, Pair<String, String>>) {
    fun texto(clave: String, defecto: String = "") = ajustes[clave] ?: defecto
    fun entero(clave: String, defecto: Int) = ajustes[clave]?.toIntOrNull() ?: defecto
    fun encendido(clave: String, defecto: Boolean) = ajustes[clave]?.let { it == "1" } ?: defecto
}

data class PasoEscalera(val nivel: Int, val accion: String, val minutos: Int, val alcance: String, val aviso: String)

data class TipoConEscalera(val tipo: TipoAdvertencia, val escalera: List<PasoEscalera>)

/** Algo marcado: un mensaje, una persona, un chat o un grupo. */
data class MarcadoItem(
    val tipo: String, val chat: String, val chatNombre: String, val msgId: String, val msgTipo: String, val texto: String, val olvidado: Boolean, val esMio: Boolean,
    val remitente: String, val persona: String, val nombre: String, val foto: String, val ts: Long, val etiquetas: List<Long>, val nota: String,
)

data class ConteoMarcados(val mensaje: Int, val persona: Int, val chat: Int, val grupo: Int)

object JsonE8 {
    private fun <T> lista(json: String, f: (JSONObject) -> T): List<T> {
        val a = JSONArray(json.ifBlank { "[]" })
        return List(a.length()) { f(a.getJSONObject(it)) }
    }

    private fun longs(a: JSONArray?) = List(a?.length() ?: 0) { a!!.getLong(it) }

    fun estado(json: String): EstadoAlmacen {
        val o = JSONObject(json.ifBlank { "{}" })
        val t = o.optJSONArray("tipos")
        return EstadoAlmacen(
            o.optLong("topeTotal"), o.optLong("uso"), o.optLong("libre"), o.optLong("total"), o.optLong("ahorradoBytes"), o.optInt("ahorradoArchivos"),
            o.optBoolean("conservarPortada", true), o.optInt("sinPortada"),
            List(t?.length() ?: 0) { i -> t!!.getJSONObject(i).let { TipoAlmacen(it.optString("tipo"), it.optLong("bytes"), it.optInt("cantidad"), it.optBoolean("apagado"), it.optLong("topeArchivo"), it.optLong("topeTotal")) } },
        )
    }

    fun auditoria(json: String): AuditoriaGlobal {
        val o = JSONObject(json.ifBlank { "{}" })
        return AuditoriaGlobal(o.optBoolean("defecto", true), o.optInt("retDias", 90), o.optBoolean("retPendiente"), o.optBoolean("avisoVisto", true))
    }

    fun chatsAuditoria(json: String) = lista(json) {
        AuditoriaChatItem(it.optString("chat"), it.optString("nombre"), it.optBoolean("esGrupo"), it.optLong("ultimoTs"), it.optInt("audit", -1), it.optBoolean("efectivo", true), it.optInt("retDias", -1), it.optInt("retEfectiva"))
    }

    fun sanciones(json: String) = lista(json) {
        SancionPendiente(it.optLong("id"), it.optString("origen"), it.optString("persona"), it.optString("grupo"), it.optLong("avisoId"), it.optLong("creadoTs"), it.optString("nombre"), it.optString("nombreGrupo"))
    }

    fun ajustesSilencio(json: String): AjustesSilencio {
        val o = JSONObject(json.ifBlank { "{}" })
        val a = o.optJSONObject("ajustes")
        val t = o.optJSONObject("textos")
        return AjustesSilencio(
            a?.keys()?.asSequence()?.associateWith { a.optString(it) } ?: emptyMap(),
            t?.keys()?.asSequence()?.associateWith { k -> t.getJSONObject(k).let { it.optString("actual") to it.optString("defecto") } } ?: emptyMap(),
        )
    }

    fun escalera(json: String) = lista(json) { PasoEscalera(it.optInt("nivel"), it.optString("accion"), it.optInt("minutos"), it.optString("alcance", "grupo"), it.optString("aviso")) }

    fun tiposConEscalera(json: String) = lista(json) {
        val t = TipoAdvertencia(it.optLong("id"), it.optString("nombre"), it.optString("emoji"), it.optString("color", "#F59E28"), it.optInt("posicion"), it.optInt("caducidadDias", -1), it.optLong("reglaId"), it.optBoolean("activo", true))
        val e = it.optJSONArray("escalera")
        TipoConEscalera(t, List(e?.length() ?: 0) { i -> e!!.getJSONObject(i).let { x -> PasoEscalera(x.optInt("nivel"), x.optString("accion"), x.optInt("minutos"), x.optString("alcance", "grupo"), x.optString("aviso")) } })
    }

    fun tipoJson(t: TipoAdvertencia, pasos: List<PasoEscalera>): String {
        val e = JSONArray()
        pasos.forEachIndexed { i, p -> e.put(JSONObject().put("nivel", i + 1).put("accion", p.accion).put("minutos", p.minutos).put("alcance", p.alcance).put("aviso", p.aviso)) }
        return JSONObject().put("id", t.id).put("nombre", t.nombre).put("emoji", t.emoji).put("color", t.color).put("posicion", t.posicion)
            .put("caducidadDias", t.caducidadDias).put("reglaId", t.reglaId).put("activo", t.activo).put("escalera", e).toString()
    }

    fun marcados(json: String) = lista(json) {
        MarcadoItem(
            it.optString("tipo"), it.optString("chat"), it.optString("chatNombre"), it.optString("msgId"), it.optString("msgTipo"), it.optString("texto"), it.optBoolean("olvidado"),
            it.optBoolean("esMio"), it.optString("remitente"), it.optString("persona"), it.optString("nombre"), it.optString("foto"), it.optLong("ts"), longs(it.optJSONArray("etiquetas")), it.optString("nota"),
        )
    }

    fun conteoMarcados(json: String): ConteoMarcados {
        val o = JSONObject(json.ifBlank { "{}" })
        return ConteoMarcados(o.optInt("mensaje"), o.optInt("persona"), o.optInt("chat"), o.optInt("grupo"))
    }
}

private suspend fun <T> io(f: () -> T): T = withContext(Dispatchers.IO) { f() }

// ---- llamadas (extensiones de Repo) ----

suspend fun Repo.estadoAlmacen(bot: Long) = io { JsonE8.estado(motor.estadoAlmacenamiento(bot)) }
suspend fun Repo.fijarTopeTotal(bot: Long, bytes: Long) = io { motor.fijarTope(bot, bytes) }
suspend fun Repo.fijarConservarPortada(bot: Long, si: Boolean) = io { motor.fijarConservarPortada(bot, si) }
suspend fun Repo.estimarLimpieza(bot: Long, f: FiltroLimpieza) = io { Json.liberado(motor.estimarBorrado(bot, f.json())) }
suspend fun Repo.borrarLimpieza(bot: Long, f: FiltroLimpieza) = io { Json.liberado(motor.borrarArchivos(bot, f.json())) }

suspend fun Repo.auditoriaGlobal(bot: Long) = io { JsonE8.auditoria(motor.auditoriaAjustes(bot)) }
suspend fun Repo.fijarAuditoria(bot: Long, guardar: Boolean, retDias: Int) = io { motor.fijarAuditoria(bot, guardar, retDias.toLong()) }
suspend fun Repo.fijarAvisoAuditoriaVisto(bot: Long, visto: Boolean = true) = io { motor.fijarAvisoAuditoriaVisto(bot, visto) }
suspend fun Repo.fijarAuditoriaChat(bot: Long, chat: String, audit: Int, retDias: Int) = io { motor.fijarAuditoriaChat(bot, chat, audit.toLong(), retDias.toLong()) }
suspend fun Repo.chatsAuditoria(bot: Long, consulta: String = "", limite: Int = 400, desde: Int = 0) = io { JsonE8.chatsAuditoria(motor.listarAuditoriaChats(bot, consulta, limite.toLong(), desde.toLong())) }
suspend fun Repo.borrarTodoDePersona(bot: Long, persona: String) = io { motor.borrarTodoDePersona(bot, persona) }

suspend fun Repo.ajustesSilencio(bot: Long) = io { JsonE8.ajustesSilencio(motor.ajustesSilencio(bot)) }
suspend fun Repo.fijarAjusteSilencio(bot: Long, clave: String, valor: String) = io { motor.fijarAjusteSilencio(bot, clave, valor) }
suspend fun Repo.fijarTextoSilencio(bot: Long, id: String, texto: String) = io { motor.fijarTextoSilencio(bot, id, texto) }
suspend fun Repo.sancionesPendientes(bot: Long) = io { JsonE8.sanciones(motor.sancionesPendientes(bot)) }
suspend fun Repo.confirmarSancion(bot: Long, id: Long, si: Boolean) = io { motor.confirmarSancionSilencio(bot, id, si) }

suspend fun Repo.tiposConEscalera(bot: Long) = io { JsonE8.tiposConEscalera(motor.listarTiposAdvertencia(bot)) }
suspend fun Repo.guardarTipo(bot: Long, t: TipoAdvertencia, pasos: List<PasoEscalera>) = io { motor.guardarTipoAdvertencia(bot, JsonE8.tipoJson(t, pasos)) }
suspend fun Repo.borrarTipo(bot: Long, id: Long) = io { motor.borrarTipoAdvertencia(bot, id) }
suspend fun Repo.guardarReglaItem(bot: Long, grupo: String, id: Long, titulo: String, texto: String, pos: Int) = io { motor.guardarReglaItem(bot, grupo, id, titulo, texto, pos.toLong()) }
suspend fun Repo.borrarReglaItem(bot: Long, id: Long) = io { motor.borrarReglaItem(bot, id) }
suspend fun Repo.moverReglaItem(bot: Long, id: Long, delta: Int) = io { motor.moverReglaItem(bot, id, delta.toLong()) }

suspend fun Repo.marcados(bot: Long, tipo: String, etiqueta: Long, limite: Int = 200, desde: Int = 0) = io { JsonE8.marcados(motor.listarMarcados(bot, tipo, etiqueta, limite.toLong(), desde.toLong())) }
suspend fun Repo.conteoMarcados(bot: Long, etiqueta: Long) = io { JsonE8.conteoMarcados(motor.contarMarcados(bot, etiqueta)) }
suspend fun Repo.crearEtiquetaEmoji(bot: Long, nombre: String, color: String, emoji: String) = io { motor.crearEtiquetaEmoji(bot, nombre, color, emoji) }
suspend fun Repo.editarEtiquetaEmoji(bot: Long, id: Long, nombre: String, color: String, emoji: String) = io { motor.editarEtiquetaEmoji(bot, id, nombre, color, emoji) }
