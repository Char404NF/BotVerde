package com.botchat.app.datos

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject

// Administrar un grupo: lista de miembros con lo que enviaron, perfil, baneados, horario de cierre, historial y resumen.
// Todo lo calcula el motor Go; aquí solo viven los modelos y su lectura.

/** Lo que una persona envió a un grupo, por tipo. `ultimoTs` y `primerTs` en segundos (0 = nada). */
data class ConteoMiembro(
    val mensajes: Int = 0, val texto: Int = 0, val imagenes: Int = 0, val videos: Int = 0, val audios: Int = 0, val notasVoz: Int = 0,
    val stickers: Int = 0, val documentos: Int = 0, val otros: Int = 0, val borrados: Int = 0, val editados: Int = 0, val primerTs: Long = 0, val ultimoTs: Long = 0,
)

/** Una persona del grupo con sus contadores, si está en línea y si está baneada. */
data class MiembroAdmin(
    val persona: String, val jid: String, val nombre: String, val telefono: String, val rol: String, val rango: String, val rangoColor: String,
    val desdeTs: Long, val silenciado: Boolean, val advertido: Boolean, val etiquetas: List<Long>, val foto: String,
    val conteo: ConteoMiembro, val enLinea: Boolean, val ultimaVez: Long, val baneado: Boolean, val advertencias: Int,
) {
    val esAdmin: Boolean get() = rol == "admin" || rol == "creador"
    val esCreador: Boolean get() = rol == "creador"

    /** El nombre que se muestra: el guardado, o el teléfono si WhatsApp no dio ninguno. */
    val titulo: String get() = nombre.takeIf { it.isNotBlank() && it != "Participante" } ?: telefono.ifBlank { persona.substringBefore('@') }
}

data class ListaMiembrosAdmin(val desde: Long, val miembros: List<MiembroAdmin>)

data class PuntoDia(val dia: String, val n: Int)

/** Una línea del historial del grupo: un cambio de miembros o una acción de moderación. */
data class EventoGrupo(
    val ts: Long, val clase: String, val tipo: String, val texto: String, val actor: String, val actorNombre: String,
    val persona: String, val personaNombre: String, val resultado: String,
) {
    val fallo: Boolean get() = resultado.isNotBlank() && resultado != "ok"
}

data class PerfilMiembroAdmin(
    val miembro: MiembroAdmin, val serie: List<PuntoDia>, val horas: List<Int>, val promedioDia: Double, val mediaGrupo: Double,
    val puesto: Int, val activos: Int, val motivoBan: String, val silenciado: Boolean, val silencioHasta: Long, val historial: List<EventoGrupo>, val desde: Long,
)

data class BaneadoGrupo(val persona: String, val nombre: String, val telefono: String, val ts: Long, val por: String, val porNombre: String, val motivo: String) {
    val titulo: String get() = nombre.ifBlank { telefono.ifBlank { persona.substringBefore('@') } }
}

/** Cierra el grupo a `cierra` y lo abre a `abre` (HH:MM) los días marcados (bit 0 = lunes … bit 6 = domingo). */
data class HorarioGrupo(val activo: Boolean = false, val cierra: String = "22:00", val abre: String = "07:00", val dias: Int = 127, val ultimo: String = "") {
    fun json(): String = JSONObject().put("activo", activo).put("cierra", cierra).put("abre", abre).put("dias", dias).toString()
}

data class ResumenAdmin(
    val miembros: Int, val admins: Int, val mensajesHoy: Int, val mensajes7: Int, val activos7: Int, val inactivos30: Int, val baneados: Int,
    val silenciados: Int, val advertidos: Int, val serie: List<PuntoDia>, val horario: HorarioGrupo, val top: List<MiembroAdmin>,
)

object JsonAdminGrupo {
    private fun <T> lista(a: JSONArray?, f: (JSONObject) -> T): List<T> = List(a?.length() ?: 0) { f(a!!.getJSONObject(it)) }

    fun conteo(o: JSONObject?): ConteoMiembro = if (o == null) ConteoMiembro() else ConteoMiembro(
        o.optInt("mensajes"), o.optInt("texto"), o.optInt("imagenes"), o.optInt("videos"), o.optInt("audios"), o.optInt("notasVoz"),
        o.optInt("stickers"), o.optInt("documentos"), o.optInt("otros"), o.optInt("borrados"), o.optInt("editados"), o.optLong("primerTs"), o.optLong("ultimoTs"),
    )

    fun miembro(o: JSONObject): MiembroAdmin = MiembroAdmin(
        o.optString("persona"), o.optString("jid"), o.optString("nombre"), o.optString("telefono"), o.optString("rol"), o.optString("rango"), o.optString("rangoColor"),
        o.optLong("desdeTs"), o.optBoolean("silenciado"), o.optBoolean("advertido"),
        List(o.optJSONArray("etiquetas")?.length() ?: 0) { o.getJSONArray("etiquetas").getLong(it) }, o.optString("foto"),
        conteo(o.optJSONObject("conteo")), o.optBoolean("enLinea"), o.optLong("ultimaVez"), o.optBoolean("baneado"), o.optInt("advertencias"),
    )

    fun miembros(json: String): ListaMiembrosAdmin {
        val o = JSONObject(json.ifBlank { "{}" })
        return ListaMiembrosAdmin(o.optLong("desde"), lista(o.optJSONArray("miembros")) { miembro(it) })
    }

    private fun puntos(a: JSONArray?) = lista(a) { PuntoDia(it.optString("dia"), it.optInt("n")) }

    fun evento(o: JSONObject) = EventoGrupo(
        o.optLong("ts"), o.optString("clase"), o.optString("tipo"), o.optString("texto"), o.optString("actor"), o.optString("actorNombre"),
        o.optString("persona"), o.optString("personaNombre"), o.optString("resultado"),
    )

    fun eventos(json: String): List<EventoGrupo> = lista(JSONArray(json.ifBlank { "[]" })) { evento(it) }

    fun perfil(json: String): PerfilMiembroAdmin {
        val o = JSONObject(json)
        val h = o.optJSONArray("horas")
        return PerfilMiembroAdmin(
            miembro(o.getJSONObject("miembro")), puntos(o.optJSONArray("serie")), List(h?.length() ?: 0) { h!!.getInt(it) },
            o.optDouble("promedioDia"), o.optDouble("mediaGrupo"), o.optInt("puesto"), o.optInt("activos"), o.optString("motivoBan"),
            o.optBoolean("silenciado"), o.optLong("silencioHasta"), lista(o.optJSONArray("historial")) { evento(it) }, o.optLong("desde"),
        )
    }

    fun baneados(json: String): List<BaneadoGrupo> = lista(JSONArray(json.ifBlank { "[]" })) {
        BaneadoGrupo(it.optString("persona"), it.optString("nombre"), it.optString("telefono"), it.optLong("ts"), it.optString("por"), it.optString("porNombre"), it.optString("motivo"))
    }

    fun horario(o: JSONObject?): HorarioGrupo = if (o == null) HorarioGrupo() else HorarioGrupo(o.optBoolean("activo"), o.optString("cierra", "22:00"), o.optString("abre", "07:00"), o.optInt("dias", 127), o.optString("ultimo"))

    fun horario(json: String): HorarioGrupo = horario(JSONObject(json.ifBlank { "{}" }))

    fun resumen(json: String): ResumenAdmin {
        val o = JSONObject(json)
        return ResumenAdmin(
            o.optInt("miembros"), o.optInt("admins"), o.optInt("mensajesHoy"), o.optInt("mensajes7"), o.optInt("activos7"), o.optInt("inactivos30"),
            o.optInt("baneados"), o.optInt("silenciados"), o.optInt("advertidos"), puntos(o.optJSONArray("serie")), horario(o.optJSONObject("horario")),
            lista(o.optJSONArray("top")) { miembro(it) },
        )
    }
}

// ---------- llamadas ----------

private suspend fun <T> io(f: () -> T): T = withContext(Dispatchers.IO) { f() }

suspend fun Repo.miembrosAdmin(bot: Long, grupo: String) = io { JsonAdminGrupo.miembros(motor.grupoMiembrosAdmin(bot, grupo)) }
suspend fun Repo.suscribirPresencias(bot: Long, grupo: String) = io { motor.grupoSuscribirPresencias(bot, grupo) }
suspend fun Repo.perfilMiembro(bot: Long, grupo: String, persona: String) = io { JsonAdminGrupo.perfil(motor.grupoPerfilMiembro(bot, grupo, persona)) }
suspend fun Repo.historialGrupo(bot: Long, grupo: String, antes: Long = 0, limite: Int = 60) = io { JsonAdminGrupo.eventos(motor.grupoHistorial(bot, grupo, antes, limite.toLong())) }
suspend fun Repo.baneadosGrupo(bot: Long, grupo: String) = io { JsonAdminGrupo.baneados(motor.grupoBaneados(bot, grupo)) }
suspend fun Repo.banear(bot: Long, grupo: String, persona: String, motivo: String) = io { motor.grupoBanear(bot, grupo, persona, motivo) }
suspend fun Repo.desbanear(bot: Long, grupo: String, persona: String) = io { motor.grupoDesbanear(bot, grupo, persona) }
suspend fun Repo.horarioGrupo(bot: Long, grupo: String) = io { JsonAdminGrupo.horario(motor.grupoHorario(bot, grupo)) }
suspend fun Repo.guardarHorarioGrupo(bot: Long, grupo: String, h: HorarioGrupo) = io { motor.grupoGuardarHorario(bot, grupo, h.json()) }
suspend fun Repo.resumenAdmin(bot: Long, grupo: String) = io { JsonAdminGrupo.resumen(motor.grupoResumenAdmin(bot, grupo)) }
