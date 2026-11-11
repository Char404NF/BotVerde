package com.botchat.app.datos

import org.json.JSONArray
import org.json.JSONObject

/** Mensajes automáticos (0.5.0, bloque D): bienvenida, ausencia, recibido, cierre y respuestas rápidas (atajos). */

object TipoAuto {
    const val BIENVENIDA = "bienvenida"
    const val AUSENCIA = "ausencia"
    const val RECIBIDO = "recibido"
    const val CIERRE = "cierre"
    const val ATAJO = "atajo"
    val todos = listOf(BIENVENIDA, AUSENCIA, RECIBIDO, CIERRE, ATAJO)

    fun titulo(t: String) = when (t) {
        BIENVENIDA -> "Bienvenida"; AUSENCIA -> "Ausencia"; RECIBIDO -> "Recibido"; CIERRE -> "Cierre"; ATAJO -> "Respuestas rápidas"; else -> t
    }

    fun descripcion(t: String) = when (t) {
        BIENVENIDA -> "Saluda a quien te escribe por primera vez, o después de varios días sin escribir."
        AUSENCIA -> "Avisa cuando escriben fuera de tu horario, en vacaciones o en fechas especiales."
        RECIBIDO -> "Dice “ya vi tu mensaje”, una vez por conversación."
        CIERRE -> "Se manda cuando termina un menú."
        ATAJO -> "Escribes /atajo en un chat y el bot manda ahí el texto guardado."
        else -> ""
    }

    /** Cómo se llama en la burbuja la etiqueta “automático” de un mensaje. */
    fun etiqueta(a: String) = when (a) {
        "" -> ""
        "menu" -> "Automático · menú"; "regla" -> "Automático · regla"; "comando" -> "Automático · comando"
        BIENVENIDA -> "Automático · bienvenida"; AUSENCIA -> "Automático · ausencia"; RECIBIDO -> "Automático · recibido"; CIERRE -> "Automático · cierre"
        ATAJO -> "Automático · atajo"; "modulo" -> "Automático · módulo"
        else -> "Automático"
    }
}

data class RangoHoras(val desde: String = "09:00", val hasta: String = "18:00")
data class DiaHorarioAuto(val dia: Int, val rangos: List<RangoHoras>)
data class VacacionAuto(val nombre: String = "", val desde: String = "", val hasta: String = "")
data class FechaAnualAuto(val nombre: String = "", val dia: Int = 1, val mes: Int = 1)

data class CuandoAuto(
    val siempre: Boolean = false,
    val horario: List<DiaHorarioAuto> = emptyList(),
    val horarioModo: String = "dentro", // dentro | fuera
    val vacaciones: List<VacacionAuto> = emptyList(),
    val fechas: List<FechaAnualAuto> = emptyList(),
    val zona: String = "",
)

data class QuienAuto(
    val personas: String = "todos", // todos | desconocidos | contactos
    val privados: Boolean = true,
    val grupos: Boolean = false,
    val gruposElegidos: List<String> = emptyList(),
    val excepto: List<String> = emptyList(),
    val soloLista: List<String> = emptyList(),
)

data class RepeticionAuto(val cada: Int = 6, val unidad: String = "horas") // minutos | horas | dias

data class ConfigAuto(
    val cuando: CuandoAuto = CuandoAuto(siempre = true),
    val quien: QuienAuto = QuienAuto(),
    val repeticion: RepeticionAuto = RepeticionAuto(),
    val silencioDias: Int = 0,
)

data class MensajeAuto(
    val id: Long = 0, val tipo: String = TipoAuto.RECIBIDO, val nombre: String = "", val texto: String = "",
    val config: ConfigAuto = ConfigAuto(), val adjuntos: List<AdjuntoSubido> = emptyList(), val activo: Boolean = true, val orden: Int = 0,
    val cuando: String = "", val quien: String = "", val repite: String = "", val envios7d: Int = 0, val estado: String = "activo",
)

data class ListaAuto(
    val mensajes: List<MensajeAuto>, val activo: Boolean, val atajosActivo: Boolean, val atajoModo: String, val topeHora: Int, val usadosHora: Int,
    val zona: String, val zonaBot: String, val conflictos: List<String>, val automaticos24h: Int,
)

data class PlantillaMensaje(val tipo: String, val nombre: String, val descripcion: String, val texto: String, val config: ConfigAuto)
data class EscenarioAuto(
    val nombre: String = "María", val esGrupo: Boolean = false, val conocida: Boolean = false, val primera: Boolean = true, val silencio: Int = 0, val cuando: String = "",
) {
    fun json(): String = JSONObject().put("nombre", nombre).put("esGrupo", esGrupo).put("conocida", conocida).put("primera", primera).put("silencio", silencio).put("cuando", cuando).toString()
}
data class ArchivoPrueba(val tipo: String, val nombre: String)
data class PruebaAuto(val aplica: Boolean, val motivo: String, val texto: String, val archivos: List<ArchivoPrueba>, val cuando: String)
data class RegistroAuto(val id: Long, val ts: Long, val autoId: Long, val tipo: String, val chat: String, val resultado: String, val adjOk: Int, val adjFallo: Int, val motivo: String, val texto: String)
data class AtajoVista(val id: Long, val nombre: String, val texto: String, val archivos: Int)

object JsonMensajesAuto {
    private fun <T> lista(a: JSONArray?, f: (JSONObject) -> T) = List(a?.length() ?: 0) { f(a!!.getJSONObject(it)) }
    private fun strs(a: JSONArray?) = List(a?.length() ?: 0) { a!!.getString(it) }

    fun config(o: JSONObject?): ConfigAuto {
        if (o == null) return ConfigAuto()
        val c = o.optJSONObject("cuando") ?: JSONObject()
        val q = o.optJSONObject("quien") ?: JSONObject()
        val r = o.optJSONObject("repeticion") ?: JSONObject()
        return ConfigAuto(
            CuandoAuto(
                c.optBoolean("siempre"),
                lista(c.optJSONArray("horario")) { d -> DiaHorarioAuto(d.optInt("dia"), lista(d.optJSONArray("rangos")) { x -> RangoHoras(x.optString("desde"), x.optString("hasta")) }) },
                c.optString("horarioModo", "dentro").ifBlank { "dentro" },
                lista(c.optJSONArray("vacaciones")) { v -> VacacionAuto(v.optString("nombre"), v.optString("desde"), v.optString("hasta")) },
                lista(c.optJSONArray("fechas")) { f -> FechaAnualAuto(f.optString("nombre"), f.optInt("dia"), f.optInt("mes")) },
                c.optString("zona"),
            ),
            QuienAuto(q.optString("personas", "todos").ifBlank { "todos" }, q.optBoolean("privados"), q.optBoolean("grupos"), strs(q.optJSONArray("gruposElegidos")), strs(q.optJSONArray("excepto")), strs(q.optJSONArray("soloLista"))),
            RepeticionAuto(r.optInt("cada"), r.optString("unidad", "horas").ifBlank { "horas" }),
            o.optInt("silencioDias"),
        )
    }

    fun configJson(c: ConfigAuto): JSONObject = JSONObject()
        .put("cuando", JSONObject().put("siempre", c.cuando.siempre).put("horarioModo", c.cuando.horarioModo).put("zona", c.cuando.zona)
            .put("horario", JSONArray().also { a -> c.cuando.horario.forEach { d -> a.put(JSONObject().put("dia", d.dia).put("rangos", JSONArray().also { r -> d.rangos.forEach { x -> r.put(JSONObject().put("desde", x.desde).put("hasta", x.hasta)) } })) } })
            .put("vacaciones", JSONArray().also { a -> c.cuando.vacaciones.forEach { v -> a.put(JSONObject().put("nombre", v.nombre).put("desde", v.desde).put("hasta", v.hasta)) } })
            .put("fechas", JSONArray().also { a -> c.cuando.fechas.forEach { f -> a.put(JSONObject().put("nombre", f.nombre).put("dia", f.dia).put("mes", f.mes)) } }))
        .put("quien", JSONObject().put("personas", c.quien.personas).put("privados", c.quien.privados).put("grupos", c.quien.grupos)
            .put("gruposElegidos", JSONArray(c.quien.gruposElegidos)).put("excepto", JSONArray(c.quien.excepto)).put("soloLista", JSONArray(c.quien.soloLista)))
        .put("repeticion", JSONObject().put("cada", c.repeticion.cada).put("unidad", c.repeticion.unidad))
        .put("silencioDias", c.silencioDias)

    fun adjuntos(a: JSONArray?) = lista(a) { AdjuntoSubido(it.optString("id"), it.optString("tipo"), it.optString("mime"), it.optString("nombre"), it.optLong("bytes")) }

    fun adjuntosJson(l: List<AdjuntoSubido>) = JSONArray().also { a -> l.forEach { a.put(JSONObject().put("id", it.id).put("tipo", it.tipo).put("mime", it.mime).put("nombre", it.nombre).put("bytes", it.bytes)) } }

    fun mensaje(o: JSONObject) = MensajeAuto(
        o.optLong("id"), o.optString("tipo"), o.optString("nombre"), o.optString("texto"), config(o.optJSONObject("config")), adjuntos(o.optJSONArray("adjuntos")),
        o.optBoolean("activo"), o.optInt("orden"), o.optString("cuando"), o.optString("quien"), o.optString("repite"), o.optInt("envios7d"), o.optString("estado", "activo"),
    )

    fun mensaje(json: String) = mensaje(JSONObject(json))

    fun entradaJson(m: MensajeAuto): String = JSONObject().put("id", m.id).put("tipo", m.tipo).put("nombre", m.nombre).put("texto", m.texto)
        .put("config", configJson(m.config)).put("adjuntos", adjuntosJson(m.adjuntos)).put("activo", m.activo).toString()

    fun lista(json: String): ListaAuto {
        val o = JSONObject(json)
        return ListaAuto(
            lista(o.optJSONArray("mensajes")) { mensaje(it) }, o.optBoolean("activo", true), o.optBoolean("atajosActivo", true), o.optString("atajoModo", "editar"),
            o.optInt("topeHora"), o.optInt("usadosHora"), o.optString("zona"), o.optString("zonaBot"), strs(o.optJSONArray("conflictos")), o.optInt("automaticos24h"),
        )
    }

    fun plantillas(json: String): List<PlantillaMensaje> = lista(JSONArray(json)) { PlantillaMensaje(it.optString("tipo"), it.optString("nombre"), it.optString("descripcion"), it.optString("texto"), config(it.optJSONObject("config"))) }

    fun prueba(json: String): PruebaAuto {
        val o = JSONObject(json)
        return PruebaAuto(o.optBoolean("aplica"), o.optString("motivo"), o.optString("texto"), lista(o.optJSONArray("archivos")) { ArchivoPrueba(it.optString("tipo"), it.optString("nombre")) }, o.optString("cuando"))
    }

    fun registro(json: String): List<RegistroAuto> = lista(JSONArray(json.ifBlank { "[]" })) {
        RegistroAuto(it.optLong("id"), it.optLong("ts"), it.optLong("autoId"), it.optString("tipo"), it.optString("chat"), it.optString("resultado"), it.optInt("adjOk"), it.optInt("adjFallo"), it.optString("motivo"), it.optString("texto"))
    }

    fun atajos(json: String): List<AtajoVista> = lista(JSONArray(json.ifBlank { "[]" })) { AtajoVista(it.optLong("id"), it.optString("nombre"), it.optString("texto"), it.optInt("archivos")) }

    fun jids(json: String): List<String> = strs(JSONArray(json.ifBlank { "[]" }))
}
