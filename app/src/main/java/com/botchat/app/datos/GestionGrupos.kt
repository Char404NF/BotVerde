package com.botchat.app.datos

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject

// Configuración base de los grupos, encuestas, fijar mensajes, avisos y copia de seguridad. Todo lo hace el motor Go.

private suspend fun <T> io(f: () -> T): T = withContext(Dispatchers.IO) { f() }

/** La configuración base: de qué grupo salió y qué trae. */
data class BaseGrupos(
    val existe: Boolean = false, val desde: String = "", val fecha: Long = 0, val filtros: Int = 0, val reglas: Boolean = false,
    val horario: Boolean = false, val entrada: Boolean = false, val auto: Boolean = false,
)

/** Qué avisos importantes también llegan por WhatsApp, cuántos minutos sin conexión antes de avisar y el resumen semanal. */
data class AjustesAvisos(val wa: Boolean = false, val tipos: Set<String> = emptySet(), val desconexion: Int = 10, val resumen: Boolean = false) {
    fun json(): String = JSONObject().put("wa", wa).put("tipos", JSONArray(tipos.toList())).put("desconexion", desconexion).put("resumen", resumen).toString()
}

/** Resultado de restaurar una copia. */
data class Restaurado(val ajustes: Int, val grupos: Int, val filtros: Int, val baneados: Int, val base: Boolean)

object JsonGestionGrupos {
    fun base(json: String): BaseGrupos {
        val o = JSONObject(json.ifBlank { "{}" })
        return BaseGrupos(o.optBoolean("existe"), o.optString("desde"), o.optLong("fecha"), o.optInt("filtros"), o.optBoolean("reglas"), o.optBoolean("horario"), o.optBoolean("entrada"), o.optBoolean("auto"))
    }

    fun avisos(json: String): AjustesAvisos {
        val o = JSONObject(json.ifBlank { "{}" })
        val t = o.optJSONArray("tipos")
        return AjustesAvisos(o.optBoolean("wa"), List(t?.length() ?: 0) { t!!.getString(it) }.toSet(), o.optInt("desconexion", 10), o.optBoolean("resumen"))
    }

    fun restaurado(json: String): Restaurado {
        val o = JSONObject(json.ifBlank { "{}" })
        return Restaurado(o.optInt("ajustes"), o.optInt("grupos"), o.optInt("filtros"), o.optInt("baneados"), o.optBoolean("base"))
    }
}

suspend fun Repo.baseGrupos(bot: Long) = io { JsonGestionGrupos.base(motor.grupoBaseInfo(bot)) }
suspend fun Repo.guardarBaseGrupos(bot: Long, chat: String) = io { motor.grupoBaseGuardar(bot, chat) }
suspend fun Repo.quitarBaseGrupos(bot: Long) = io { motor.grupoBaseQuitar(bot) }
suspend fun Repo.baseGruposAuto(bot: Long, si: Boolean) = io { motor.grupoBaseAuto(bot, si) }
suspend fun Repo.aplicarBaseGrupos(bot: Long, grupos: List<String>): Int = io { motor.grupoBaseAplicar(bot, JSONArray(grupos).toString()).toInt() }
suspend fun Repo.mandarEncuesta(bot: Long, chat: String, titulo: String, opciones: List<String>, multiple: Boolean) =
    io { motor.grupoEncuesta(bot, chat, titulo, JSONArray(opciones).toString(), multiple) }
suspend fun Repo.fijarMensaje(bot: Long, chat: String, msgId: String, dias: Int, fijar: Boolean) = io { motor.grupoFijar(bot, chat, msgId, dias.toLong(), fijar) }
suspend fun Repo.ajustesAvisos(bot: Long) = io { JsonGestionGrupos.avisos(motor.alertasLeer(bot)) }
suspend fun Repo.guardarAjustesAvisos(bot: Long, a: AjustesAvisos) = io { motor.alertasGuardar(bot, a.json()) }
suspend fun Repo.resumenSemanalAhora(bot: Long): String = io { motor.resumenSemanalAhora(bot) }
suspend fun Repo.exportarConfig(bot: Long, contrasena: String): ByteArray = io { motor.respaldoConfigExportar(bot, contrasena) }
suspend fun Repo.restaurarConfig(bot: Long, contrasena: String, datos: ByteArray) = io { JsonGestionGrupos.restaurado(motor.respaldoConfigRestaurar(bot, contrasena, datos)) }
