package com.botchat.app.datos

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject

// Asistente «Crea tu bot» (0.6.0, etapa E9). El borrador lo guarda y valida Go (cifrado); aquí solo viven el modelo y las llamadas.

data class OpcionAsistente(val texto: String, val respuesta: String)

data class BorradorAsistente(
    val paso: Int = 1,
    val saltados: List<Int> = emptyList(),
    val privados: Boolean = true,
    val grupos: Boolean = false,
    val gruposElegidos: List<String> = emptyList(),
    val bienvenidaActiva: Boolean = true,
    val bienvenidaTexto: String = "",
    val bienvenidaAdjuntos: List<AdjuntoSubido> = emptyList(),
    val bienvenidaCuando: String = "primera", // primera | diaria
    val menuActivo: Boolean = true,
    val menuTitulo: String = "",
    val menuOpciones: List<OpcionAsistente> = emptyList(),
    val reglasAlEntrar: Boolean = false,
    val reglasAvisoOk: Boolean = false,
    val silenciar: Boolean = true,
    val advertencias: Boolean = true,
    val etiquetas: Boolean = true,
    val recordatorios: Boolean = true,
    val notas: Boolean = true,
    val otroNumero: String = "",
) {
    fun json(): String {
        val o = JSONObject()
        o.put("v", 1).put("paso", paso).put("saltados", JSONArray(saltados))
        o.put("privados", privados).put("grupos", grupos).put("gruposElegidos", JSONArray(gruposElegidos))
        o.put("bienvenida", JSONObject().put("activa", bienvenidaActiva).put("texto", bienvenidaTexto).put("cuando", bienvenidaCuando)
            .put("adjuntos", JSONArray(bienvenidaAdjuntos.map { JSONObject().put("id", it.id).put("tipo", it.tipo).put("mime", it.mime).put("nombre", it.nombre).put("bytes", it.bytes) })))
        o.put("menu", JSONObject().put("activo", menuActivo).put("titulo", menuTitulo)
            .put("opciones", JSONArray(menuOpciones.map { JSONObject().put("texto", it.texto).put("respuesta", it.respuesta) })))
        o.put("reglas", JSONObject().put("alEntrar", reglasAlEntrar).put("avisoOk", reglasAvisoOk))
        o.put("util", JSONObject().put("silenciar", silenciar).put("advertencias", advertencias).put("etiquetas", etiquetas).put("recordatorios", recordatorios).put("notas", notas))
        o.put("otroNumero", otroNumero)
        return o.toString()
    }

    fun saltado(p: Int) = p in saltados
    fun conSaltado(p: Int, si: Boolean) = copy(saltados = if (si) (saltados + p).distinct() else saltados - p)
}

data class EstadoAsistente(
    val borrador: BorradorAsistente, val problemas: List<String>, val visto: Boolean, val aplicado: Boolean,
    val numeroPrueba: String, val numeroDe: String,
)

data class ResultadoAsistente(val menuId: Long, val autos: List<Long>, val avisos: List<String>, val moduloEncendido: Boolean)

object JsonE9 {
    private fun strs(a: JSONArray?) = List(a?.length() ?: 0) { a!!.optString(it) }

    fun borrador(o: JSONObject): BorradorAsistente {
        val b = o.optJSONObject("bienvenida") ?: JSONObject()
        val m = o.optJSONObject("menu") ?: JSONObject()
        val r = o.optJSONObject("reglas") ?: JSONObject()
        val u = o.optJSONObject("util") ?: JSONObject()
        val adj = b.optJSONArray("adjuntos")
        val ops = m.optJSONArray("opciones")
        val sal = o.optJSONArray("saltados")
        return BorradorAsistente(
            paso = o.optInt("paso", 1).coerceIn(1, 6),
            saltados = List(sal?.length() ?: 0) { sal!!.optInt(it) },
            privados = o.optBoolean("privados", true), grupos = o.optBoolean("grupos"), gruposElegidos = strs(o.optJSONArray("gruposElegidos")),
            bienvenidaActiva = b.optBoolean("activa", true), bienvenidaTexto = b.optString("texto"), bienvenidaCuando = b.optString("cuando", "primera"),
            bienvenidaAdjuntos = List(adj?.length() ?: 0) { i -> adj!!.getJSONObject(i).let { AdjuntoSubido(it.optString("id"), it.optString("tipo"), it.optString("mime"), it.optString("nombre"), it.optLong("bytes")) } },
            menuActivo = m.optBoolean("activo", true), menuTitulo = m.optString("titulo"),
            menuOpciones = List(ops?.length() ?: 0) { i -> ops!!.getJSONObject(i).let { OpcionAsistente(it.optString("texto"), it.optString("respuesta")) } },
            reglasAlEntrar = r.optBoolean("alEntrar"), reglasAvisoOk = r.optBoolean("avisoOk"),
            silenciar = u.optBoolean("silenciar", true), advertencias = u.optBoolean("advertencias", true), etiquetas = u.optBoolean("etiquetas", true),
            recordatorios = u.optBoolean("recordatorios", true), notas = u.optBoolean("notas", true),
            otroNumero = o.optString("otroNumero"),
        )
    }

    fun estado(json: String): EstadoAsistente {
        val o = JSONObject(json.ifBlank { "{}" })
        return EstadoAsistente(
            borrador(o.optJSONObject("borrador") ?: JSONObject()), strs(o.optJSONArray("problemas")), o.optBoolean("visto"), o.optBoolean("aplicado"),
            o.optString("numeroPrueba"), o.optString("numeroPruebaDe"),
        )
    }

    fun resultado(json: String): ResultadoAsistente {
        val o = JSONObject(json.ifBlank { "{}" })
        val a = o.optJSONArray("autos")
        return ResultadoAsistente(o.optLong("menu"), List(a?.length() ?: 0) { a!!.optLong(it) }, strs(o.optJSONArray("avisos")), o.optBoolean("modulo"))
    }

    /** El borrador de arranque (con los tres ejemplos del menú) para el motor falso de las pruebas. */
    fun borradorPorDefecto(): String = BorradorAsistente(
        bienvenidaTexto = "¡Hola {nombre}! Gracias por escribirnos. Escribe *menu* para ver las opciones.", menuTitulo = "¿En qué te ayudo?",
        menuOpciones = listOf(OpcionAsistente("Horarios", "Atendemos de lunes a viernes de 9:00 a 18:00."), OpcionAsistente("Ubicación", "Estamos en: escribe aquí tu dirección."), OpcionAsistente("Precios", "Pídenos tu cotización.")),
    ).json()
}

private suspend fun <T> io(f: () -> T): T = withContext(Dispatchers.IO) { f() }

suspend fun Repo.asistenteEstado(bot: Long) = io { JsonE9.estado(motor.asistenteEstado(bot)) }
suspend fun Repo.asistenteGuardar(bot: Long, b: BorradorAsistente) = io { motor.asistenteGuardar(bot, b.json()) }
suspend fun Repo.asistenteVisto(bot: Long) = io { motor.asistenteVisto(bot) }
suspend fun Repo.asistenteAplicar(bot: Long) = io { JsonE9.resultado(motor.asistenteAplicar(bot)) }
suspend fun Repo.asistenteDeshacer(bot: Long) = io { motor.asistenteDeshacer(bot) }
suspend fun Repo.asistenteDescartar(bot: Long) = io { motor.asistenteDescartar(bot) }
suspend fun Repo.asistenteProbar(bot: Long, numero: String) = io { motor.asistenteProbar(bot, numero) }
