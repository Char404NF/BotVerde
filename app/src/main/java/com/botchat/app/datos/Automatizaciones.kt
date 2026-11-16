package com.botchat.app.datos

import org.json.JSONArray
import org.json.JSONObject

data class AutoResumen(val id: Long, val activo: Boolean, val sistema: Boolean, val version: Int, val nombre: String, val resumen: String, val avisos: Int, val pasos: Int)

/** Una opción de un menú. [ambito]: "" (en todos lados), "privado" o "grupo". [horario]: solo se ofrece dentro de ese horario. */
data class OpcionDef(val texto: String = "", val destino: String = "", val id: String = "", val ambito: String = "", val horario: HorarioDef? = null, val boton: BotonDef? = null)

/** Botón especial de una opción cuando el menú sale con botones: [tipo] enlace (abre una página), copiar (copia un texto) o llamar (marca un número). */
data class BotonDef(val tipo: String = "", val valor: String = "")

data class AdjuntoDef(val id: String, val tipo: String, val mime: String = "", val nombre: String = "")
data class UbicacionDef(val lat: Double = 0.0, val lon: Double = 0.0, val nombre: String = "", val direccion: String = "")
data class ContactoDef(val nombre: String = "", val telefono: String = "")

/** Una pregunta de un formulario. [tipo]: texto, numero, correo, telefono, fecha u opcion. */
data class CampoDef(
    val id: String = "", val etiqueta: String = "", val pregunta: String = "", val tipo: String = "texto", val opciones: List<String> = emptyList(),
    val opcional: Boolean = false, val error: String = "", val min: Double? = null, val max: Double? = null,
)

/** Ajustes de UN menú; lo que está vacío o en cero usa el valor general (Ajustes → Avanzado → Menús). */
data class AjustesMenuDef(
    val errMax: Int = 0, val txtN1: String = "", val txtN2: String = "", val txtN3: String = "", val txtNoTexto: String = "",
    val expiraMin: Int = 0, val expiraGrupoMin: Int = 0, val volver: String = "", val humanoDentro: String = "", val humanoFuera: String = "",
    val maxSesionesGrupo: Int = 0, val cualquiera: String = "",
) {
    val vacio get() = this == AjustesMenuDef()
}

data class PasoDef(
    val id: String,
    val tipo: String, // menu, respuesta, pedir, humano, fin, formulario
    val titulo: String = "",
    val opciones: List<OpcionDef> = emptyList(),
    val texto: String = "",
    val pregunta: String = "",
    val variable: String = "",
    val siguiente: String = "",
    val notificar: Boolean = false,
    val aviso: String = "",
    val adjuntos: List<AdjuntoDef> = emptyList(),
    val ubicacion: UbicacionDef? = null,
    val contacto: ContactoDef? = null,
    val textoFuera: String = "",
    val campos: List<CampoDef> = emptyList(),
    val maxReintentos: Int = 0,
    val sinResumen: Boolean = false,
    /** Cómo sale ESTE menú: "" (el general de Ajustes), "texto", "encuesta" o "botones" (solo en chats privados). */
    val formato: String = "",
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

data class AlcanceDef(val privados: Boolean = true, val chats: List<String> = emptyList(), val grupos: List<String> = emptyList(), val todosGrupos: Boolean = false)
data class HorarioDef(val dias: List<Int> = emptyList(), val desde: String = "", val hasta: String = "")

/** Una automatización completa, tal como la guarda el módulo Go. Lo que esta pantalla no conoce (por ejemplo `propio`) se conserva en [extra]. */
data class AutoDef(
    val nombre: String,
    val inicio: String,
    val pasos: List<PasoDef>,
    val disp: DispDef = DispDef(),
    val alcance: AlcanceDef = AlcanceDef(),
    val horario: HorarioDef = HorarioDef(),
    val ajustes: AjustesMenuDef = AjustesMenuDef(),
    val extra: String = "{}",
) {
    fun paso(id: String) = pasos.firstOrNull { it.id == id }

    fun json(): String {
        val ps = JSONArray()
        pasos.forEach { p -> ps.put(JsonAuto.pasoJson(p)) }
        val o = JSONObject(extra).put("v", 1).put("nombre", nombre).put("inicio", inicio).put("pasos", ps)
            .put("disparadores", JSONObject().put("primerMensaje", disp.primerMensaje).put("diasSinEscribir", disp.diasSinEscribir).put("palabras", JSONArray(disp.palabras))
                .put("comando", disp.comando).put("fueraDeHorario", disp.fueraDeHorario).put("mencion", disp.mencion).put("cualquiera", disp.cualquiera).put("enfriamientoSeg", disp.enfriamientoSeg))
            .put("alcance", JSONObject().put("privados", alcance.privados).put("chats", JSONArray(alcance.chats)).put("grupos", JSONArray(alcance.grupos)).put("todosGrupos", alcance.todosGrupos))
            .put("horario", JsonAuto.horarioJson(horario))
        if (!ajustes.vacio) o.put("ajustes", JsonAuto.ajustesJson(ajustes)) else o.remove("ajustes")
        return o.toString()
    }

    companion object {
        private fun lista(a: JSONArray?) = List(a?.length() ?: 0) { a!!.getString(it) }
        private val CONOCIDAS = setOf("v", "nombre", "inicio", "pasos", "disparadores", "alcance", "horario", "ajustes")

        fun horario(h: JSONObject?) = HorarioDef(h?.optJSONArray("dias").let { x -> List(x?.length() ?: 0) { x!!.getInt(it) } }, h?.optString("desde") ?: "", h?.optString("hasta") ?: "")

        fun desde(o: JSONObject): AutoDef {
            val ps = o.optJSONArray("pasos")
            val pasos = List(ps?.length() ?: 0) { i -> JsonAuto.paso(ps!!.getJSONObject(i)) }
            val d = o.optJSONObject("disparadores")
            val a = o.optJSONObject("alcance")
            val extra = JSONObject()
            o.keys().forEach { k -> if (k !in CONOCIDAS) extra.put(k, o.get(k)) }
            return AutoDef(
                o.optString("nombre"), o.optString("inicio"), pasos,
                DispDef(
                    d?.optBoolean("primerMensaje") ?: false, d?.optInt("diasSinEscribir") ?: 0, lista(d?.optJSONArray("palabras")), d?.optString("comando") ?: "",
                    d?.optBoolean("fueraDeHorario") ?: false, d?.optBoolean("mencion") ?: false, d?.optBoolean("cualquiera") ?: false, d?.optInt("enfriamientoSeg") ?: 0,
                ),
                AlcanceDef(a?.optBoolean("privados") ?: true, lista(a?.optJSONArray("chats")), lista(a?.optJSONArray("grupos")), a?.optBoolean("todosGrupos") ?: false),
                horario(o.optJSONObject("horario")),
                JsonAuto.ajustes(o.optJSONObject("ajustes")),
                extra.toString(),
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
data class EstadMenu(
    val iniciados: Long, val completados: Long, val abandonados: Long, val humano: Long, val expirados: Long, val opciones: List<FilaEstMenu>, val embudo: List<FilaEstMenu>, val invalidas: List<FilaEstMenu>,
    val abandonos: List<FilaEstMenu> = emptyList(), val formulariosEnviados: Long = 0, val formulariosCancelados: Long = 0, val noEntendio: Long = 0, val formularios: Long = 0,
)
data class EtiquetaMenu(val nombre: String, val ruta: String, val ultima: Long, val estado: String)
data class AvisoLocal(val id: Long, val ts: Long, val tipo: String, val chat: String, val titulo: String, val detalle: String, val persona: String = "")
data class PruebaSim(val salidas: List<String>, val estado: String, val paso: String, val omitido: String, val coincide: Boolean)

object JsonAuto {
    fun horarioJson(h: HorarioDef) = JSONObject().put("dias", JSONArray(h.dias)).put("desde", h.desde).put("hasta", h.hasta)

    fun ajustesJson(a: AjustesMenuDef): JSONObject {
        val o = JSONObject()
        if (a.errMax != 0) o.put("errMax", a.errMax)
        if (a.txtN1.isNotEmpty()) o.put("txtN1", a.txtN1)
        if (a.txtN2.isNotEmpty()) o.put("txtN2", a.txtN2)
        if (a.txtN3.isNotEmpty()) o.put("txtN3", a.txtN3)
        if (a.txtNoTexto.isNotEmpty()) o.put("txtNoTexto", a.txtNoTexto)
        if (a.expiraMin != 0) o.put("expiraMin", a.expiraMin)
        if (a.expiraGrupoMin != 0) o.put("expiraGrupoMin", a.expiraGrupoMin)
        if (a.volver.isNotEmpty()) o.put("volver", a.volver)
        if (a.humanoDentro.isNotEmpty()) o.put("humanoDentro", a.humanoDentro)
        if (a.humanoFuera.isNotEmpty()) o.put("humanoFuera", a.humanoFuera)
        if (a.maxSesionesGrupo != 0) o.put("maxSesionesGrupo", a.maxSesionesGrupo)
        if (a.cualquiera.isNotEmpty()) o.put("cualquiera", a.cualquiera)
        return o
    }

    fun ajustes(o: JSONObject?) = if (o == null) AjustesMenuDef() else AjustesMenuDef(
        o.optInt("errMax"), o.optString("txtN1"), o.optString("txtN2"), o.optString("txtN3"), o.optString("txtNoTexto"), o.optInt("expiraMin"), o.optInt("expiraGrupoMin"),
        o.optString("volver"), o.optString("humanoDentro"), o.optString("humanoFuera"), o.optInt("maxSesionesGrupo"), o.optString("cualquiera"),
    )

    fun pasoJson(p: PasoDef): JSONObject {
        val o = JSONObject().put("id", p.id).put("tipo", p.tipo).put("titulo", p.titulo).put("texto", p.texto).put("pregunta", p.pregunta).put("variable", p.variable)
            .put("siguiente", p.siguiente).put("notificar", p.notificar).put("aviso", p.aviso)
            .put("opciones", JSONArray().also { a ->
                p.opciones.forEach { op ->
                    val j = JSONObject().put("texto", op.texto).put("destino", op.destino)
                    if (op.id.isNotEmpty()) j.put("id", op.id)
                    if (op.ambito.isNotEmpty()) j.put("ambito", op.ambito)
                    if (op.horario != null && (op.horario.dias.isNotEmpty() || op.horario.desde.isNotEmpty() || op.horario.hasta.isNotEmpty())) j.put("horario", horarioJson(op.horario))
                    if (op.boton != null && op.boton.tipo.isNotEmpty()) j.put("boton", JSONObject().put("tipo", op.boton.tipo).put("valor", op.boton.valor))
                    a.put(j)
                }
            })
        if (p.adjuntos.isNotEmpty()) o.put("adjuntos", JSONArray().also { a -> p.adjuntos.forEach { x -> a.put(JSONObject().put("id", x.id).put("tipo", x.tipo).put("mime", x.mime).put("nombre", x.nombre)) } })
        p.ubicacion?.let { o.put("ubicacion", JSONObject().put("lat", it.lat).put("lon", it.lon).put("nombre", it.nombre).put("direccion", it.direccion)) }
        p.contacto?.let { o.put("contacto", JSONObject().put("nombre", it.nombre).put("telefono", it.telefono)) }
        if (p.textoFuera.isNotEmpty()) o.put("textoFuera", p.textoFuera)
        if (p.campos.isNotEmpty()) o.put("campos", JSONArray().also { a ->
            p.campos.forEach { c ->
                val j = JSONObject().put("id", c.id).put("etiqueta", c.etiqueta).put("pregunta", c.pregunta).put("tipo", c.tipo)
                if (c.opciones.isNotEmpty()) j.put("opciones", JSONArray(c.opciones))
                if (c.opcional) j.put("opcional", true)
                if (c.error.isNotEmpty()) j.put("error", c.error)
                c.min?.let { j.put("min", it) }
                c.max?.let { j.put("max", it) }
                a.put(j)
            }
        })
        if (p.maxReintentos != 0) o.put("maxReintentos", p.maxReintentos)
        if (p.sinResumen) o.put("sinResumen", true)
        if (p.formato.isNotEmpty()) o.put("formato", p.formato)
        return o
    }

    fun paso(p: JSONObject): PasoDef {
        val op = p.optJSONArray("opciones")
        val ad = p.optJSONArray("adjuntos")
        val cs = p.optJSONArray("campos")
        return PasoDef(
            p.optString("id"), p.optString("tipo"), p.optString("titulo"),
            List(op?.length() ?: 0) { j ->
                op!!.getJSONObject(j).let { OpcionDef(it.optString("texto"), it.optString("destino"), it.optString("id"), it.optString("ambito"), it.optJSONObject("horario")?.let { h -> AutoDef.horario(h) },
                    it.optJSONObject("boton")?.let { b -> BotonDef(b.optString("tipo"), b.optString("valor")) }) }
            },
            p.optString("texto"), p.optString("pregunta"), p.optString("variable"), p.optString("siguiente"), p.optBoolean("notificar"), p.optString("aviso"),
            List(ad?.length() ?: 0) { j -> ad!!.getJSONObject(j).let { AdjuntoDef(it.optString("id"), it.optString("tipo"), it.optString("mime"), it.optString("nombre")) } },
            p.optJSONObject("ubicacion")?.let { UbicacionDef(it.optDouble("lat"), it.optDouble("lon"), it.optString("nombre"), it.optString("direccion")) },
            p.optJSONObject("contacto")?.let { ContactoDef(it.optString("nombre"), it.optString("telefono")) },
            p.optString("textoFuera"),
            List(cs?.length() ?: 0) { j ->
                cs!!.getJSONObject(j).let { c ->
                    val oo = c.optJSONArray("opciones")
                    CampoDef(
                        c.optString("id"), c.optString("etiqueta"), c.optString("pregunta"), c.optString("tipo", "texto"), List(oo?.length() ?: 0) { k -> oo!!.getString(k) },
                        c.optBoolean("opcional"), c.optString("error"), if (c.has("min")) c.optDouble("min") else null, if (c.has("max")) c.optDouble("max") else null,
                    )
                }
            },
            p.optInt("maxReintentos"), p.optBoolean("sinResumen"), p.optString("formato"),
        )
    }

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
        return EstadMenu(
            o.optLong("iniciados"), o.optLong("completados"), o.optLong("abandonados"), o.optLong("humano"), o.optLong("expirados"), filas(o.optJSONArray("opciones")), filas(o.optJSONArray("embudo")), filas(o.optJSONArray("invalidas")),
            filas(o.optJSONArray("abandonos")), o.optLong("formulariosEnviados"), o.optLong("formulariosCancelados"), o.optLong("noEntendio"), o.optLong("formularios"),
        )
    }

    fun etiquetas(json: String) = lista(json) { EtiquetaMenu(it.optString("nombre"), it.optString("ruta"), it.optLong("ultima"), it.optString("estado")) }
    fun aviso(json: String): AvisoLocal { val o = JSONObject(json); return AvisoLocal(o.optLong("id"), o.optLong("ts"), o.optString("tipo"), o.optString("chat"), o.optString("titulo"), o.optString("detalle"), o.optString("persona")) }
    fun prueba(json: String): PruebaSim {
        val o = JSONObject(json)
        val s = o.optJSONArray("salidas")
        return PruebaSim(List(s?.length() ?: 0) { s!!.getString(it) }, o.optString("estado"), o.optString("paso"), o.optString("omitido"), o.optBoolean("coincide"))
    }
}
