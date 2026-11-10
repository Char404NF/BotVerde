package com.botchat.app.ui

import android.content.Context
import android.view.Choreographer
import com.botchat.app.Datos
import com.botchat.app.datos.ModoCarga
import com.botchat.app.datos.Repo
import com.botchat.app.medios.CacheImagenes
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeoutOrNull
import java.util.concurrent.atomic.AtomicInteger

/**
 * Avanzado → Medir rendimiento → "Abrir chat": crea un chat de prueba con 20,000 mensajes (se borra al terminar), lo abre de verdad
 * varias veces y mide cuántos ms pasan hasta ver el primer contenido y hasta el contenido completo (primera página, trabajo
 * diferido y fotos visibles), más los cuadros que se perdieron durante la animación y la carga. Con "comparar" repite todo con la
 * forma de cargar de la 0.4.0 para ver el antes y el después en el mismo teléfono.
 */
object MedidorChat {
    const val JID_PRUEBA = "120363999999999999@g.us"
    const val MENSAJES = 20_000

    val lineas = MutableStateFlow<List<String>>(emptyList())
    val corriendo = MutableStateFlow(false)

    private val alcance = CoroutineScope(SupervisorJob() + Dispatchers.Main)
    private val pendientes = AtomicInteger(0)
    @Volatile private var t0 = 0L
    @Volatile private var primer = -1L
    @Volatile private var completo = -1L
    @Volatile private var activa = false
    @Volatile private var fin: CompletableDeferred<Unit>? = null

    /** ¿La pantalla del chat debe reportar? Solo con el chat de prueba y una medición en curso. */
    fun mide(jid: String) = activa && jid == JID_PRUEBA

    fun empieza() { pendientes.incrementAndGet() }
    fun termina() { pendientes.decrementAndGet() }
    fun imagenesPendientes() = pendientes.get()

    fun primerContenido() { if (activa && primer < 0) primer = (System.nanoTime() - t0) / 1_000_000 }

    fun contenidoCompleto() {
        if (!activa || completo >= 0) return
        completo = (System.nanoTime() - t0) / 1_000_000
        fin?.complete(Unit)
    }

    private class Corrida(val primerMs: Long, val completoMs: Long, val cuadros: Int, val caidos: Int, val lentos: Int, val peorMs: Long)

    private fun linea(etq: String, c: Corrida?) =
        if (c == null) "  $etq: no terminó (¿saliste de la pantalla?)"
        else "  $etq: primer contenido ${c.primerMs} ms · completo ${c.completoMs} ms · cuadros perdidos ${c.caidos}/${c.cuadros} (${"%.1f".format(100.0 * c.caidos / c.cuadros.coerceAtLeast(1))} %), muy lentos ${c.lentos}, peor ${c.peorMs} ms"

    /** Una apertura: navega, espera a que la pantalla reporte el contenido completo y cuenta los cuadros perdidos. */
    private suspend fun abrirUna(ctx: Context, abrir: (String) -> Unit): Corrida? {
        val periodoNs = (1_000_000_000.0 / hz(ctx)).toLong()
        var ultimo = 0L; var cuadros = 0; var caidos = 0; var lentos = 0; var peor = 0L
        var contando = true
        val cb = object : Choreographer.FrameCallback {
            override fun doFrame(t: Long) {
                if (ultimo != 0L) {
                    val d = t - ultimo
                    cuadros++
                    if (d > periodoNs * 3 / 2) caidos++
                    if (d > periodoNs * 3) lentos++
                    if (d / 1_000_000 > peor) peor = d / 1_000_000
                }
                ultimo = t
                if (contando) Choreographer.getInstance().postFrameCallback(this)
            }
        }
        primer = -1; completo = -1; pendientes.set(0)
        fin = CompletableDeferred()
        activa = true
        Choreographer.getInstance().postFrameCallback(cb)
        t0 = System.nanoTime()
        abrir(JID_PRUEBA)
        val ok = withTimeoutOrNull(60_000) { fin?.await() } != null
        delay(300) // un poco más de cuadros, para que cuente también el final de la animación
        contando = false
        activa = false
        return if (ok) Corrida(primer.coerceAtLeast(0), completo, cuadros, caidos, lentos, peor) else null
    }

    private suspend fun frio() {
        withContext(Dispatchers.Default) { CacheImagenes.vaciar(); CacheTexto.vaciar(); CachePaginas.vaciar() }
    }

    /**
     * Corre la medición completa. [abrir] navega al chat (y la pantalla del chat vuelve sola al terminar).
     * Con [comparar] se mide primero la carga anterior (0.4.0) y luego la nueva.
     */
    fun medir(ctx: Context, comparar: Boolean, abrir: (String) -> Unit) {
        if (!corriendo.compareAndSet(expect = false, update = true)) return
        lineas.value = emptyList()
        fun anota(s: String) { lineas.value = lineas.value + s }
        alcance.launch {
            val bot = Datos.get(ctx).primero()?.id ?: 0L
            try {
                anota("Creando un chat de prueba con $MENSAJES mensajes (fotos, citas, menciones, reacciones y eliminados)…")
                val t = System.nanoTime()
                withContext(Dispatchers.IO) { Repo.crearChatPrueba(bot, MENSAJES) }
                anota("Listo en ${(System.nanoTime() - t) / 1_000_000} ms. Se abrirá el chat solo, varias veces; no toques la pantalla.")
                val modos = if (comparar) listOf(true to "Carga anterior (0.4.0)", false to "Carga nueva") else listOf(false to "Carga nueva")
                for ((anterior, nombre) in modos) {
                    ModoCarga.fijar(anterior)
                    anota("\n— $nombre —")
                    frio()
                    anota(linea("1.ª vez (en frío)", abrirUna(ctx, abrir)))
                    delay(700)
                    anota(linea("2.ª vez (con caché)", abrirUna(ctx, abrir)))
                    delay(700)
                    frio()
                    anota(linea("3.ª vez (en frío otra vez)", abrirUna(ctx, abrir)))
                    delay(700)
                }
                anota("\nObjetivo: primer contenido < 300 ms y cuadros perdidos cerca de 0 %.")
            } catch (e: Exception) {
                anota("Error: ${e.message}")
            } finally {
                activa = false
                ModoCarga.fijar(false)
                withContext(Dispatchers.IO) { runCatching { Repo.borrarChat(bot, JID_PRUEBA) } }
                anota("(El chat de prueba se borró.)")
                corriendo.value = false
            }
        }
    }

    @Suppress("DEPRECATION")
    private fun hz(ctx: Context): Float {
        val d = if (android.os.Build.VERSION.SDK_INT >= 30) ctx.display
        else (ctx.getSystemService(Context.WINDOW_SERVICE) as android.view.WindowManager).defaultDisplay
        return d?.refreshRate?.takeIf { it > 0f } ?: 60f
    }
}
