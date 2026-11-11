package com.botchat.app

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import com.botchat.app.datos.Ajustes
import com.botchat.app.seguridad.ClaveDatos
import android.os.Build
import android.os.IBinder
import androidx.core.app.NotificationCompat
import androidx.core.content.ContextCompat
import wabot.Listener
import wabot.Wabot
import java.util.concurrent.Executors

/** Servicio en primer plano: mantiene vivo el módulo Go (whatsmeow) y muestra la notificación fija. */
class BotService : Service() {

    private val hilo = Executors.newSingleThreadExecutor()
    private var bot: Bot? = null

    private val oyente = object : Listener {
        override fun onStatus(texto: String) {
            if (texto == "vinculado") bot?.let { Datos.get(this@BotService).marcarVinculado(it.id) } // primero se guarda
            if (texto == "sesion_cerrada") bot?.let { Datos.get(this@BotService).marcarVinculado(it.id, false) } // WhatsApp desvinculó el bot
            BotState.estado.value = texto
            BotState.log("Estado: $texto")
            actualizaNotificacion()
        }

        override fun onPairingCode(codigo: String) {
            BotState.codigo.value = codigo
            BotState.log("Código de vinculación listo.") // el código no se guarda en el registro
        }

        override fun onLog(linea: String) = BotState.log(linea)
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private var redCallback: ConnectivityManager.NetworkCallback? = null
    private val ambito = kotlinx.coroutines.CoroutineScope(kotlinx.coroutines.SupervisorJob() + kotlinx.coroutines.Dispatchers.Default)

    override fun onCreate() {
        super.onCreate()
        BotState.corriendo.value = true
        creaCanal()
        Notificador.crearCanal(this)
        Notificador.iniciar(this, ambito)
        vigilaRed()
    }

    /** Avisa al módulo Go si el teléfono usa Wi-Fi (o una red sin costo) o datos móviles, para las reglas de descarga. */
    private fun vigilaRed() {
        val cm = getSystemService(ConnectivityManager::class.java) ?: return
        val cb = object : ConnectivityManager.NetworkCallback() {
            override fun onCapabilitiesChanged(network: Network, caps: NetworkCapabilities) {
                val sinCosto = caps.hasCapability(NetworkCapabilities.NET_CAPABILITY_NOT_METERED)
                Wabot.establecerRed(if (sinCosto) Wabot.RedWifi else Wabot.RedDatos)
            }
            override fun onLost(network: Network) { Wabot.establecerRed(Wabot.RedDesconocida) }
        }
        redCallback = cb
        runCatching { cm.registerDefaultNetworkCallback(cb) }
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val prefs = getSharedPreferences("bot", MODE_PRIVATE)
        val accion = intent?.action ?: ACC_INICIAR
        val id = intent?.getLongExtra(EXTRA_ID, 0L)?.takeIf { it != 0L } ?: prefs.getLong("activo", 0L)
        val b = Datos.get(this).porId(id)
        if (b == null) {
            ponPrimerPlano("Bot Chat")
            apaga()
            return START_NOT_STICKY
        }
        bot = b
        ponPrimerPlano(textoNotificacion())
        when (accion) {
            ACC_DETENER -> {
                prefs.edit().remove("activo").apply()
                hilo.execute { Wabot.stop(); BotState.estado.value = "detenido"; apaga() }
                return START_NOT_STICKY
            }
            ACC_ELIMINAR -> {
                prefs.edit().remove("activo").apply()
                hilo.execute { elimina(b) }
                return START_NOT_STICKY
            }
            ACC_VINCULAR -> {
                prefs.edit().putLong("activo", b.id).apply()
                BotState.codigo.value = null
                BotState.estado.value = "pidiendo_codigo"
                hilo.execute { arranca(b); vincula(b) }
            }
            else -> { // ACC_INICIAR y reinicio automático del sistema
                prefs.edit().putLong("activo", b.id).apply()
                hilo.execute { arranca(b) }
            }
        }
        return START_STICKY
    }

    private fun arranca(b: Bot) {
        try {
            Ajustes.get(this).aplicarAGo()
            Wabot.start(filesDir.absolutePath, b.id, ClaveDatos.obtener(this), oyente)
        } catch (e: Exception) {
            if (e.message?.contains("ya está iniciado") == true) return
            BotState.log("No se pudo iniciar: ${e.message}")
            BotState.estado.value = "error: ${e.message}"
            actualizaNotificacion()
        }
    }

    private fun vincula(b: Bot) {
        try {
            Wabot.pair(b.vincular)
        } catch (e: Exception) {
            BotState.log("No se pudo pedir el código: ${e.message}")
            BotState.estado.value = "error: ${e.message}"
            actualizaNotificacion()
        }
    }

    private fun elimina(b: Bot) {
        arranca(b) // hace falta la sesión abierta para poder cerrarla en WhatsApp
        var aviso: String? = null
        try {
            Wabot.logout()
        } catch (e: Exception) {
            aviso = e.message ?: "No se pudo cerrar la sesión en WhatsApp."
        }
        Wabot.stop()
        runCatching { Wabot.eliminarBot(b.id) } // mensajes, archivos y fotos de este bot
        runCatching { com.botchat.app.seguridad.LlavesModulos.borrarDeBot(this, b.id) } // llaves de módulos (IA…)
        runCatching { Datos.get(this).carpetaSesion(b.id).deleteRecursively() }
        Datos.get(this).eliminar(b.id)
        BotState.log("Bot eliminado.")
        BotState.estado.value = "detenido"
        BotState.codigo.value = null
        BotState.aviso.value = aviso
        BotState.version.value += 1
        apaga()
    }

    private fun apaga() {
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    override fun onDestroy() {
        BotState.corriendo.value = false
        ambito.coroutineContext[kotlinx.coroutines.Job]?.cancel()
        redCallback?.let { cb -> runCatching { getSystemService(ConnectivityManager::class.java)?.unregisterNetworkCallback(cb) } }
        hilo.execute { Wabot.stop() }
        hilo.shutdown()
        super.onDestroy()
    }

    // ---------- notificación ----------

    private fun creaCanal() {
        val nm = getSystemService(NotificationManager::class.java)
        nm.createNotificationChannel(NotificationChannel(CANAL, "Bot en marcha", NotificationManager.IMPORTANCE_LOW))
    }

    private fun textoNotificacion(): String = when (val e = BotState.estado.value) {
        "conectado" -> "Conectado y respondiendo"
        "conectando", "pidiendo_codigo" -> "Conectando…"
        "esperando_codigo" -> "Esperando que escribas el código"
        "desconectado" -> "Sin conexión, reintentando…"
        "sesion_cerrada" -> "La sesión se cerró. Abre la app."
        "sin_vincular" -> "Falta vincular"
        else -> if (e.startsWith("error")) "Hay un problema. Abre la app." else "En marcha"
    }

    private fun notificacion(texto: String): Notification {
        val abrir = PendingIntent.getActivity(
            this, 0, Intent(this, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        val detener = PendingIntent.getService(
            this, 1, Intent(this, BotService::class.java).setAction(ACC_DETENER),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        return NotificationCompat.Builder(this, CANAL)
            .setSmallIcon(R.drawable.ic_stat_bot)
            .setContentTitle(bot?.nombre ?: "Bot Chat")
            .setContentText(texto)
            .setContentIntent(abrir)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setCategory(NotificationCompat.CATEGORY_SERVICE)
            .setVisibility(NotificationCompat.VISIBILITY_PRIVATE)
            .addAction(0, "Detener", detener)
            .build()
    }

    private fun ponPrimerPlano(texto: String) {
        val n = notificacion(texto)
        if (Build.VERSION.SDK_INT >= 29) {
            val tipo = if (Build.VERSION.SDK_INT >= 34) ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE else 0
            startForeground(ID_NOTIF, n, tipo)
        } else {
            startForeground(ID_NOTIF, n)
        }
    }

    private fun actualizaNotificacion() {
        getSystemService(NotificationManager::class.java).notify(ID_NOTIF, notificacion(textoNotificacion()))
    }

    companion object {
        const val ACC_INICIAR = "com.botchat.app.INICIAR"
        const val ACC_DETENER = "com.botchat.app.DETENER"
        const val ACC_VINCULAR = "com.botchat.app.VINCULAR"
        const val ACC_ELIMINAR = "com.botchat.app.ELIMINAR"
        const val EXTRA_ID = "bot_id"
        private const val CANAL = "bot"
        private const val ID_NOTIF = 1

        fun manda(ctx: Context, accion: String, botId: Long) {
            val i = Intent(ctx, BotService::class.java).setAction(accion).putExtra(EXTRA_ID, botId)
            ContextCompat.startForegroundService(ctx, i)
        }
    }
}
