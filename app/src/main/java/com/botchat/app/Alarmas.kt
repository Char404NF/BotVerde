package com.botchat.app

import android.app.AlarmManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.provider.Settings
import com.botchat.app.datos.Repo
import kotlinx.coroutines.runBlocking

/**
 * Alarmas locales de los avisos para mí (recordatorios y resumen diario). Solo programan CUÁNDO despertar a la app; qué toca lo decide el
 * módulo Go contando la base. Sin servicios de Google ni servidor. Con el permiso de alarmas exactas suenan a la hora; sin él se usan
 * alarmas inexactas (pueden retrasarse por el ahorro de batería) y la pantalla Avisos lo explica con un botón para pedir el permiso.
 */
object Alarmas {
    const val ACCION = "com.botchat.app.AVISOS"
    private const val CODIGO = 7100

    private fun gestor(ctx: Context) = ctx.getSystemService(AlarmManager::class.java)

    /** ¿El sistema deja programar alarmas exactas? (Antes de Android 12 siempre.) */
    fun puedeExactas(ctx: Context): Boolean = Build.VERSION.SDK_INT < Build.VERSION_CODES.S || gestor(ctx)?.canScheduleExactAlarms() == true

    /** Pantalla del sistema donde el usuario concede las alarmas exactas (null si no hace falta). */
    fun intentPermisoExactas(ctx: Context): Intent? =
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) Intent(Settings.ACTION_REQUEST_SCHEDULE_EXACT_ALARM, Uri.parse("package:" + ctx.packageName)).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK) else null

    private fun pendiente(ctx: Context): PendingIntent =
        PendingIntent.getBroadcast(ctx, CODIGO, Intent(ctx, AlarmaAvisosReceiver::class.java).setAction(ACCION), PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)

    /** Programa la alarma para `siguienteSeg` (segundos desde 1970); 0 o pasada ya = ahora; menor que 0 la cancela. Devuelve si quedó exacta. */
    fun programar(ctx: Context, siguienteSeg: Long, exactas: Boolean = true): Boolean {
        val am = gestor(ctx) ?: return false
        val pi = pendiente(ctx)
        am.cancel(pi)
        if (siguienteSeg <= 0) return false
        val cuando = maxOf(siguienteSeg * 1000L, System.currentTimeMillis() + 1000L)
        val exacta = exactas && puedeExactas(ctx)
        runCatching {
            if (exacta) am.setExactAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, cuando, pi)
            else am.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, cuando, pi)
        }.onFailure { runCatching { am.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, cuando, pi) } } // el permiso se pudo quitar justo ahora
        return exacta
    }

    /** Lo mismo que [reprogramar], pero fuera del hilo principal (para las pantallas). */
    suspend fun reprogramarAsync(ctx: Context) = kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) { reprogramar(ctx) }

    /** Vuelve a calcular la próxima alarma (después de cambiar un recordatorio o el resumen, al abrir la app y al reiniciar el teléfono). */
    fun reprogramar(ctx: Context) {
        val app = ctx.applicationContext
        runCatching {
            if (!Repo.abrir(app)) return
            val bot = Datos.get(app).primero() ?: return
            runBlocking {
                val sig = Repo.proximoAviso(bot.id)
                val exactas = Repo.ajustesPermisos(bot.id)["avisos_exactos"] != "0"
                programar(app, if (sig > 0) sig else -1, exactas)
            }
        }
    }
}

/**
 * Suena la alarma: el módulo Go crea los avisos que ya tocan, aquí se muestran como notificaciones y se programa la siguiente.
 * Solo muestra avisos locales: no manda ningún mensaje.
 */
class AlarmaAvisosReceiver : BroadcastReceiver() {
    override fun onReceive(ctx: Context, intent: Intent) {
        val app = ctx.applicationContext
        val pendiente = goAsync()
        Thread {
            try {
                if (!Repo.abrir(app)) return@Thread
                val bot = Datos.get(app).primero() ?: return@Thread
                runBlocking {
                    val d = Repo.dispararAvisos(bot.id)
                    d.avisos.forEach { runCatching { Notificador.mostrarAviso(app, it) } }
                    val exactas = Repo.ajustesPermisos(bot.id)["avisos_exactos"] != "0"
                    Alarmas.programar(app, if (d.siguiente > 0) d.siguiente else -1, exactas)
                }
            } catch (_: Exception) {
            } finally {
                pendiente?.finish()
            }
        }.start()
    }
}

/** Tras reiniciar el teléfono (o actualizar la app) se vuelven a programar las alarmas locales. No escribe mensajes ni hace nada más. */
class ArranqueReceiver : BroadcastReceiver() {
    override fun onReceive(ctx: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_BOOT_COMPLETED && intent.action != Intent.ACTION_MY_PACKAGE_REPLACED) return
        val app = ctx.applicationContext
        val pendiente = goAsync()
        Thread { try { Alarmas.reprogramar(app); AlarmasModulos.reprogramar(app) } finally { pendiente?.finish() } }.start()
    }
}

/**
 * Alarma de lo que los módulos tienen agendado (mensajes programados y difusiones programadas, H parte 1). Igual que [Alarmas]: solo dice
 * CUÁNDO despertar a la app; qué toca (y si todavía está a tiempo según la tolerancia) lo decide el módulo Go. Si el bot no está en
 * marcha cuando suena, no pasa nada: al arrancar cada módulo revisa lo pendiente y descarta, avisando, lo que ya pasó del tiempo de tolerancia.
 */
object AlarmasModulos {
    const val ACCION = "com.botchat.app.MODULOS"
    private const val CODIGO = 7101

    private fun gestor(ctx: Context) = ctx.getSystemService(AlarmManager::class.java)

    private fun pendiente(ctx: Context): PendingIntent =
        PendingIntent.getBroadcast(ctx, CODIGO, Intent(ctx, AlarmaModulosReceiver::class.java).setAction(ACCION), PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)

    /** Programa la alarma para `siguienteSeg` (segundos desde 1970); 0 o menor la cancela. */
    fun programar(ctx: Context, siguienteSeg: Long) {
        val am = gestor(ctx) ?: return
        val pi = pendiente(ctx)
        am.cancel(pi)
        if (siguienteSeg <= 0) return
        val cuando = maxOf(siguienteSeg * 1000L, System.currentTimeMillis() + 1000L)
        runCatching {
            if (Alarmas.puedeExactas(ctx)) am.setExactAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, cuando, pi)
            else am.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, cuando, pi)
        }.onFailure { runCatching { am.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, cuando, pi) } }
    }

    suspend fun reprogramarAsync(ctx: Context) = kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.IO) { reprogramar(ctx) }

    /** Vuelve a calcular la próxima alarma de los módulos (al abrir la app, tras cambiar un módulo y al reiniciar el teléfono). */
    fun reprogramar(ctx: Context) {
        val app = ctx.applicationContext
        runCatching {
            if (!Repo.abrir(app)) return
            val bot = Datos.get(app).primero() ?: return
            runBlocking { programar(app, Repo.proximoModulos(bot.id)) }
        }
    }
}

/** Suena la alarma de los módulos: se le pide al módulo Go que revise su agenda y se programa la siguiente. No escribe nada por sí sola. */
class AlarmaModulosReceiver : BroadcastReceiver() {
    override fun onReceive(ctx: Context, intent: Intent) {
        val app = ctx.applicationContext
        val pendiente = goAsync()
        Thread {
            try {
                if (!Repo.abrir(app)) return@Thread
                val bot = Datos.get(app).primero() ?: return@Thread
                runBlocking {
                    runCatching { Repo.tickModulos(bot.id) }
                    AlarmasModulos.programar(app, Repo.proximoModulos(bot.id))
                }
            } catch (_: Exception) {
            } finally {
                pendiente?.finish()
            }
        }.start()
    }
}
