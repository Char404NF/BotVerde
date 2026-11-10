package com.botchat.app

import android.Manifest
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.Person
import androidx.core.app.RemoteInput
import androidx.core.content.ContextCompat
import com.botchat.app.datos.Ajustes
import com.botchat.app.datos.Msg
import com.botchat.app.datos.Repo
import com.botchat.app.ui.ChatAbierto
import com.botchat.app.ui.descripcionCorta
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.flow.MutableStateFlow

/** Para abrir un chat desde una notificación: la pantalla principal lo recoge y navega. */
object NavegarA {
    val chat = MutableStateFlow<String?>(null)
}

/**
 * Notificaciones de mensajes nuevos. Apagadas por defecto (Avanzado → Funciones). Sin servicios de Google: son notificaciones locales
 * que se generan cuando el módulo Go guarda un mensaje. En la pantalla bloqueada solo dicen "Mensaje nuevo".
 */
object Notificador {
    const val CANAL = "mensajes"
    const val EXTRA_CHAT = "chat"
    const val CLAVE_RESPUESTA = "respuesta"
    private const val ACC_RESPONDER = "com.botchat.app.RESPONDER"
    private val cerrojo = Mutex()

    fun crearCanal(ctx: Context) {
        val nm = ctx.getSystemService(NotificationManager::class.java) ?: return
        nm.createNotificationChannel(NotificationChannel(CANAL, "Mensajes nuevos", NotificationManager.IMPORTANCE_DEFAULT))
    }

    private fun permitido(ctx: Context) = Build.VERSION.SDK_INT < 33 || ContextCompat.checkSelfPermission(ctx, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED

    /** Empieza a vigilar los cambios; hay que cancelar el Job al detener el servicio. */
    fun iniciar(ctx: Context, scope: CoroutineScope): Job = scope.launch(start = kotlinx.coroutines.CoroutineStart.UNDISPATCHED) {
        Repo.cambios.collect { evs ->
            for (e in evs) {
                if (e.startsWith("chat:")) runCatching { revisar(ctx, e.removePrefix("chat:")) }
                if (e == "motor:freno") runCatching { avisoFreno(ctx) }
                if (e.startsWith("aviso:")) e.removePrefix("aviso:").toLongOrNull()?.let { runCatching { avisoLocal(ctx, it) } }
            }
        }
    }

    private fun registro(ctx: Context) = ctx.getSharedPreferences("notificados", Context.MODE_PRIVATE)

    /** Avisa de los mensajes recibidos que todavía no se vieron ni se avisaron. */
    suspend fun revisar(ctx: Context, jid: String) = cerrojo.withLock {
        val ajustes = Ajustes.get(ctx)
        if (!ajustes.notificacionesMensajes.value || !permitido(ctx) || ChatAbierto.jid == jid) return@withLock
        val bot = Datos.get(ctx).primero() ?: return@withLock
        if (Repo.estadoChat(bot.id, jid).silenciado) return@withLock
        val ya = maxOf(registro(ctx).getLong(jid, 0L), ajustes.visto(jid))
        val limite = System.currentTimeMillis() / 1000 - 600 // lo viejo (historial, sincronización) no genera aviso
        val nuevos = Repo.mensajes(bot.id, jid, limite = 6).filter { !it.esMio && !it.borrado && it.ts > ya && it.ts >= limite }
        if (nuevos.isEmpty()) return@withLock
        val chat = Repo.chat(bot.id, jid)
        registro(ctx).edit().putLong(jid, nuevos.maxOf { it.ts }).apply()
        avisar(ctx, bot.id, jid, chat?.nombre ?: "Mensaje nuevo", chat?.esGrupo == true, nuevos.sortedBy { it.ts }, ajustes.escribirDesdeApp.value)
    }

    private fun avisar(ctx: Context, botId: Long, jid: String, nombre: String, grupo: Boolean, msgs: List<Msg>, conRespuesta: Boolean) {
        crearCanal(ctx)
        val yo = Person.Builder().setName("Tú").build()
        val estilo = NotificationCompat.MessagingStyle(yo).setConversationTitle(if (grupo) nombre else null).setGroupConversation(grupo)
        msgs.forEach { m ->
            val quien = Person.Builder().setName(if (grupo && m.remitente.isNotBlank()) m.remitente else nombre).build()
            estilo.addMessage(descripcionCorta(m.tipo, m.texto, false, m.notaVoz), m.ts * 1000, quien)
        }
        val abrir = PendingIntent.getActivity(
            ctx, jid.hashCode(), Intent(ctx, MainActivity::class.java).putExtra(EXTRA_CHAT, jid).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        val publica = NotificationCompat.Builder(ctx, CANAL).setSmallIcon(R.drawable.ic_stat_bot).setContentTitle("Bot Chat").setContentText("Mensaje nuevo").build()
        val b = NotificationCompat.Builder(ctx, CANAL)
            .setSmallIcon(R.drawable.ic_stat_bot).setStyle(estilo).setContentIntent(abrir).setAutoCancel(true)
            .setCategory(NotificationCompat.CATEGORY_MESSAGE).setVisibility(NotificationCompat.VISIBILITY_PRIVATE).setPublicVersion(publica)
            .setOnlyAlertOnce(true).setWhen(msgs.last().ts * 1000)
        if (conRespuesta) {
            val respuesta = PendingIntent.getBroadcast(
                ctx, jid.hashCode(), Intent(ctx, RespuestaRapida::class.java).setAction(ACC_RESPONDER).putExtra(EXTRA_CHAT, jid).putExtra("bot", botId),
                PendingIntent.FLAG_MUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
            )
            b.addAction(NotificationCompat.Action.Builder(0, "Responder", respuesta).addRemoteInput(RemoteInput.Builder(CLAVE_RESPUESTA).setLabel("Escribe tu respuesta").build()).setSemanticAction(NotificationCompat.Action.SEMANTIC_ACTION_REPLY).build())
        }
        ctx.getSystemService(NotificationManager::class.java)?.notify(jid.hashCode(), b.build())
    }

    private const val ID_FRENO = 7001
    private const val CANAL_AVISOS = "avisos"

    /**
     * Aviso local de una automatización ("Alguien pidió hablar con una persona", "Nueva solicitud"). En la pantalla bloqueada solo sale el título
     * neutro (visibilidad privada con versión pública); el detalle (nombre, número, datos) se ve con el teléfono desbloqueado. Un interruptor de
     * Avanzado permite verlo también bloqueado (apagado por defecto).
     */
    private suspend fun avisoLocal(ctx: Context, id: Long) {
        if (!permitido(ctx)) return
        val bot = Datos.get(ctx).primero() ?: return
        val a = Repo.aviso(bot.id, id)
        val nm = ctx.getSystemService(NotificationManager::class.java) ?: return
        nm.createNotificationChannel(NotificationChannel(CANAL_AVISOS, "Avisos del bot", NotificationManager.IMPORTANCE_HIGH))
        val detalleVisibleBloqueado = Ajustes.get(ctx).detalleBloqueado.value
        val abrir = PendingIntent.getActivity(
            ctx, a.chat.hashCode(), Intent(ctx, MainActivity::class.java).apply { if (a.chat.isNotEmpty() && a.tipo == "humano") putExtra(EXTRA_CHAT, a.chat) }.addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        val publica = NotificationCompat.Builder(ctx, CANAL_AVISOS).setSmallIcon(R.drawable.ic_stat_bot).setContentTitle(a.titulo).build()
        val n = NotificationCompat.Builder(ctx, CANAL_AVISOS)
            .setSmallIcon(R.drawable.ic_stat_bot).setContentTitle(a.titulo).setContentText(a.detalle.lineSequence().firstOrNull().orEmpty())
            .setStyle(NotificationCompat.BigTextStyle().bigText(a.detalle)).setContentIntent(abrir).setAutoCancel(true)
            .setVisibility(if (detalleVisibleBloqueado) NotificationCompat.VISIBILITY_PUBLIC else NotificationCompat.VISIBILITY_PRIVATE)
            .apply { if (!detalleVisibleBloqueado) setPublicVersion(publica) }
            .build()
        nm.notify(8000 + (id % 1000).toInt(), n)
    }

    /** El motor se pausó unos minutos porque fallaron varios envíos seguidos. */
    private fun avisoFreno(ctx: Context) {
        if (!permitido(ctx)) return
        crearCanal(ctx)
        val abrir = PendingIntent.getActivity(ctx, ID_FRENO, Intent(ctx, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)
        val n = NotificationCompat.Builder(ctx, CANAL).setSmallIcon(R.drawable.ic_stat_bot).setContentTitle("El bot se pausó unos minutos")
            .setContentText("Fallaron varios envíos seguidos. Se reanuda solo.").setContentIntent(abrir).setAutoCancel(true).build()
        ctx.getSystemService(NotificationManager::class.java)?.notify(ID_FRENO, n)
    }

    fun quitar(ctx: Context, jid: String) { ctx.getSystemService(NotificationManager::class.java)?.cancel(jid.hashCode()) }
}

/** Respuesta rápida escrita en la notificación: sale como un mensaje manual (no del bot). */
class RespuestaRapida : BroadcastReceiver() {
    override fun onReceive(ctx: Context, intent: Intent) {
        val texto = RemoteInput.getResultsFromIntent(intent)?.getCharSequence(Notificador.CLAVE_RESPUESTA)?.toString()?.trim().orEmpty()
        val chat = intent.getStringExtra(Notificador.EXTRA_CHAT) ?: return
        val bot = intent.getLongExtra("bot", 0L)
        if (texto.isEmpty() || bot == 0L || !Ajustes.get(ctx).escribirDesdeApp.value) return
        val pendiente = goAsync()
        Thread {
            try {
                runCatching { Repo.motor.enviarTexto(bot, chat, texto, "", "[]") }
            } finally {
                Notificador.quitar(ctx, chat)
                pendiente.finish()
            }
        }.start()
    }
}
