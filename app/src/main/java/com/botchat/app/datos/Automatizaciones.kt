package com.botchat.app.datos

import org.json.JSONArray
import org.json.JSONObject

data class AutoResumen(val id: Long, val activo: Boolean, val sistema: Boolean, val version: Int, val nombre: String, val resumen: String, val avisos: Int, val pasos: Int)

data class OpcionDef(val texto: String = "", val destino: String = "")

data class PasoDef(
    val id: String,
    val tipo: String, // menu, respuesta, pedir, humano, fin
    val titulo: String = "",
    val opciones: List<OpcionDef> = emptyList(),
    val texto: String = "",
    val pregunta: String = "",
    val variable: String = "",
    val siguiente: String = "",
    val notificar: Boolean = false,
    val aviso: String = "",
)

data class DispDef(
    val primerMensaje: Boolean = false,
    val diasSinEscribir: Int = 0,
    val palabras: List<String> = emptyList(),
    val comando: String = "",
    val fueraDeHorario: Boolean = false,
    val mencion: Boolean = false,
    val cualquiera: Boolean = false,
    val enfriamientoSeg: Int = 0,
)

data class AlcanceDef(val privados: Boolean = true, val chats: List<String> = emptyList(), val grupos: List<String> = emptyList())
data class HorarioDef(val dias: List<Int> = emptyList(), val desde: String = "", val hasta: String = "")

/** Una automatización completa, tal como la guarda el módulo Go. */
data class AutoDef(
    val nombre: String,
    val inicio: String,
    val pasos: List<PasoDef>,
    val disp: DispDef = DispDef(),
    val alcance: AlcanceDef = AlcanceDef(),
    val horario: HorarioDef = HorarioDef(),
) {
    fun paso(id: String) = pasos.firstOrNull { it.id == id }

    fun json(): String {
        val ps = JSONArray()
        pasos.forEach { p ->
            ps.put(JSONObject().put("id", p.id).put("tipo", p.tipo).put("titulo", p.titulo).put("texto", p.texto).put("pregunta", p.pregunta).put("variable", p.variable)
                .put("siguiente", p.siguiente).put("notificar", p.notificar).put("aviso", p.aviso)
                .put("opciones", JSONArray().also { a -> p.opciones.forEach { a.put(JSONObject().put("texto", it.texto).put("destino", it.destino)) } }))
        }
        return JSONObject().put("v", 1).put("nombre", nombre).put("inicio", inicio).put("pasos", ps)
            .put("disparadores", JSONObject().put("primerMensaje", disp.primerMensaje).put("diasSinEscribir", disp.diasSinEscribir).put("palabras", JSONArray(disp.palabras))
                .put("comando", disp.comando).put("fueraDeHorario", disp.fueraDeHorario).put("mencion", disp.mencion).put("cualquiera", disp.cualquiera).put("enfriamientoSeg", disp.enfriamientoSeg))
            .put("alcance", JSONObject().put("privados", alcance.privados).put("chats", JSONArray(alcance.chats)).put("grupos", JSONArray(alcance.grupos)))
            .put("horario", JSONObject().put("dias", JSONArray(horario.dias)).put("desde", horario.desde).put("hasta", horario.hasta)).toString()
    }

    companion object {
        private fun lista(a: JSONArray?) = List(a?.length() ?: 0) { a!!.getString(it) }

        fun desde(o: JSONObject): AutoDef {
            val ps = o.optJSONArray("pasos")
            val pasos = List(ps?.length() ?: 0) { i ->
                val p = ps!!.getJSONObject(i)
                val op = p.optJSONArray("opciones")
                PasoDef(
                    p.optString("id"), p.optString("tipo"), p.optString("titulo"),
                    List(op?.length() ?: 0) { j -> op!!.getJSONObject(j).let { OpcionDef(it.optString("texto"), it.optString("destino")) } },
                    p.optString("texto"), p.optString("pregunta"), p.optString("variable"), p.optString("siguiente"), p.optBoolean("notificar"), p.optString("aviso"),
                )
            }
            val d = o.optJSONObject("disparadores")
            val a = o.optJSONObject("alcance")
            val h = o.optJSONObject("horario")
            return AutoDef(
                o.optString("nombre"), o.optString("inicio"), pasos,
                DispDef(
                    d?.optBoolean("primerMensaje") ?: false, d?.optInt("diasSinEscribir") ?: 0, lista(d?.optJSONArray("palabras")), d?.optString("comando") ?: "",
                    d?.optBoolean("fueraDeHorario") ?: false, d?.optBoolean("mencion") ?: false, d?.optBoolean("cualquiera") ?: false, d?.optInt("enfriamientoSeg") ?: 0,
                ),
                AlcanceDef(a?.optBoolean("privados") ?: true, lista(a?.optJSONArray("chats")), lista(a?.optJSONArray("grupos"))),
                HorarioDef(h?.optJSONArray("dias").let { x -> List(x?.length() ?: 0) { x!!.getInt(it) } }, h?.optString("desde") ?: "", h?.optString("hasta") ?: ""),
            )
        }

        fun desde(json: String) = desde(JSONObject(json))

        fun nueva() = AutoDef("Nueva automatización", "p1", listOf(PasoDef("p1", "menu", "¡Hola {nombre}! ¿En qué te ayudo?", listOf(OpcionDef("Opción 1", "p2")), "", "", "", "", false, ""), PasoDef("p2", "respuesta", "", emptyList(), "Escribe aquí la respuesta.")))
    }
}

data class AvisoVal(val paso: String, val texto: String, val grave: Boolean)
data class AutoDetalle(val id: Long, val activo: Boolean, val sistema: Boolean, val version: Int, val def: AutoDef, val avisos: List<AvisoVal>)
data class PlantillaAuto(val nombre: String, val descripcion: String, val def: AutoDef)
data class SesionVivo(val id: Long, val chat: String, val sender: String, val menuId: Long, val paso: String, val estado: String, val ultima: Long, val inicio: Long, val nombre: String, val menu: String, val ruta: String)
data class PasoRecorrido(val ts: Long, val titulo: String, val opcion: String, val resultado: String)
data class Recorrido(val sesion: SesionVivo, val persona: String, val pasos: List<PasoRecorrido>)
data class FilaEstMenu(val paso: String, val titulo: String, val valor: Long, val opcion: String, val texto: String)
data class EstadMenu(val iniciados: Long, val completados: Long, val abandonados: Long, val humano: Long, val expirados: Long, val opciones: List<FilaEstMenu>, val embudo: List<FilaEstMenu>, val invalidas: List<FilaEstMenu>)
data class EtiquetaMenu(val nombre: String, val ruta: String, val ultima: Long, val estado: String)
data class AvisoLocal(val id: Long, val ts: Long, val tipo: String, val chat: String, val titulo: String, val detalle: String, val persona: String = "")
data class PruebaSim(val salidas: List<String>, val estado: String, val paso: String, val omitido: String, val coincide: Boolean)

object JsonAuto {
    private fun <T> lista(json: String, f: (JSONObject) -> T): List<T> {
        val a = JSONArray(json.ifBlank { "[]" })
        return List(a.length()) { f(a.getJSONObject(it)) }
    }

    private fun avisos(a: JSONArray?) = List(a?.length() ?: 0) { i -> a!!.getJSONObject(i).let { AvisoVal(it.optString("paso"), it.optString("texto"), it.optBoolean("grave")) } }

    fun resumenes(json: String) = lista(json) { AutoResumen(it.optLong("id"), it.optBoolean("activo"), it.optBoolean("sistema"), it.optInt("version"), it.optString("nombre"), it.optString("resumen"), it.optInt("avisos"), it.optInt("pasos")) }
    fun detalle(json: String): AutoDetalle { val o = JSONObject(json); return AutoDetalle(o.optLong("id"), o.optBoolean("activo"), o.optBoolean("sistema"), o.optInt("version"), AutoDef.desde(o.getJSONObject("def")), avisos(o.optJSONArray("avisos"))) }
    fun validacion(json: String) = avisos(JSONArray(json.ifBlank { "[]" }))
    fun guardado(json: String): Triple<Long, Int, List<AvisoVal>> { val o = JSONObject(json); return Triple(o.optLong("id"), o.optInt("version"), avisos(o.optJSONArray("avisos"))) }
    fun plantillas(json: String) = lista(json) { PlantillaAuto(it.optString("nombre"), it.optString("descripcion"), AutoDef.desde(it.getJSONObject("def"))) }

    private fun sesion(o: JSONObject) = SesionVivo(
        o.optLong("id"), o.optString("chat"), o.optString("sender"), o.optLong("menuId"), o.optString("paso"), o.optString("estado"), o.optLong("ultima"), o.optLong("inicio"),
        o.optString("nombre"), o.optString("menu"), o.optString("ruta"),
    )

    fun sesiones(json: String) = lista(json) { sesion(it) }
    fun recorrido(json: String): Recorrido {
        val o = JSONObject(json)
        val ps = o.optJSONArray("pasos")
        val s = sesion(o.getJSONObject("sesion").put("menu", o.optString("menu")))
        return Recorrido(s, o.optString("persona"), List(ps?.length() ?: 0) { i -> ps!!.getJSONObject(i).let { PasoRecorrido(it.optLong("ts"), it.optString("titulo"), it.optString("opcion"), it.optString("resultado")) } })
    }

    private fun filas(a: JSONArray?) = List(a?.length() ?: 0) { i -> a!!.getJSONObject(i).let { FilaEstMenu(it.optString("paso"), it.optString("titulo"), it.optLong("valor"), it.optString("opcion"), it.optString("texto")) } }
    fun estadisticas(json: String): EstadMenu {
        val o = JSONObject(json)
        return EstadMenu(o.optLong("iniciados"), o.optLong("completados"), o.optLong("abandonados"), o.optLong("humano"), o.optLong("expirados"), filas(o.optJSONArray("opciones")), filas(o.optJSONArray("embudo")), filas(o.optJSONArray("invalidas")))
    }

    fun etiquetas(json: String) = lista(json) { EtiquetaMenu(it.optString("nombre"), it.optString("ruta"), it.optLong("ultima"), it.optString("estado")) }
    fun aviso(json: String): AvisoLocal { val o = JSONObject(json); return AvisoLocal(o.optLong("id"), o.optLong("ts"), o.optString("tipo"), o.optString("chat"), o.optString("titulo"), o.optString("detalle"), o.optString("persona")) }
    fun prueba(json: String): PruebaSim {
        val o = JSONObject(json)
        val s = o.optJSONArray("salidas")
        return PruebaSim(List(s?.length() ?: 0) { s!!.getString(it) }, o.optString("estado"), o.optString("paso"), o.optString("omitido"), o.optBoolean("coincide"))
    }
}
