package com.botchat.app

import android.app.NotificationManager
import android.content.Context
import android.content.Intent
import android.os.Bundle
import androidx.core.app.RemoteInput
import androidx.test.core.app.ApplicationProvider
import com.botchat.app.datos.Ajustes
import com.botchat.app.pantallas.Apoyo
import com.botchat.app.pantallas.MotorFalso
import com.botchat.app.ui.ChatAbierto
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class)
class NotificadorTest {
    private lateinit var motor: MotorFalso
    private val ctx: Context get() = ApplicationProvider.getApplicationContext()
    private val jid = "5215512345678@s.whatsapp.net"
    private val ahora = System.currentTimeMillis() / 1000
    private fun nm() = ctx.getSystemService(NotificationManager::class.java)
    private fun avisos() = shadowOf(nm()).allNotifications

    @Before fun antes() {
        motor = Apoyo.reinicia(); Apoyo.creaBot()
        motor.chat = Apoyo.chatJson(jid, "Ana Pérez").toString()
        shadowOf(ctx as android.app.Application).grantPermissions(android.Manifest.permission.POST_NOTIFICATIONS)
        ChatAbierto.jid = ""
        nm().cancelAll()
    }

    private fun revisa() = runBlocking { Notificador.revisar(ctx, jid) }
    private fun conMensaje(ts: Long = ahora + 300, mio: Boolean = false) {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("n1", ts, texto = "hola, ¿sigues ahí?", remitente = "Ana", mio = mio))
    }

    @Test fun apagadasPorDefectoNoAvisanNada() {
        conMensaje()
        revisa()
        assertEquals(0, avisos().size)
    }

    @Test fun encendidasAvisanDeUnMensajeNuevoConVersionPublicaNeutra() {
        Ajustes.get(ctx).fijarNotificacionesMensajes(true)
        conMensaje()
        revisa()
        assertEquals(1, avisos().size)
        val n = avisos()[0]
        assertEquals("Mensaje nuevo", n.publicVersion.extras.getCharSequence("android.text").toString())
        assertEquals(androidx.core.app.NotificationCompat.VISIBILITY_PRIVATE, n.visibility)
        assertEquals("tiene respuesta rápida", 1, n.actions.size)
        assertNotNull(n.actions[0].remoteInputs)
        revisa()
        assertEquals("el mismo mensaje no se vuelve a avisar", 1, shadowOf(nm()).size())
    }

    @Test fun sinEscribirDesdeLaAppNoHayRespuestaRapida() {
        Ajustes.get(ctx).fijarNotificacionesMensajes(true); Ajustes.get(ctx).fijarEscribirDesdeApp(false)
        conMensaje(); revisa()
        assertEquals(0, avisos()[0].actions?.size ?: 0)
    }

    @Test fun nadaDeAvisarDeLoPropioLoViejoLoSilenciadoNiDelChatAbierto() {
        Ajustes.get(ctx).fijarNotificacionesMensajes(true)
        conMensaje(mio = true); revisa()
        conMensaje(ts = ahora - 5000); revisa()
        motor.estadoChat = """{"silenciado":true,"botPausaHasta":0,"botActivo":true}"""
        conMensaje(); revisa()
        motor.estadoChat = """{"silenciado":false,"botPausaHasta":0,"botActivo":true}"""
        ChatAbierto.jid = jid
        revisa()
        assertEquals(0, avisos().size)
        ChatAbierto.jid = ""
        revisa()
        assertEquals(1, avisos().size)
    }

    @Test fun laRespuestaRapidaSaleComoMensajeManual() {
        val i = Intent(ctx, RespuestaRapida::class.java).putExtra(Notificador.EXTRA_CHAT, jid).putExtra("bot", Datos.get(ctx).primero()!!.id)
        val datos = Bundle().apply { putCharSequence(Notificador.CLAVE_RESPUESTA, "ya voy") }
        RemoteInput.addResultsToIntent(arrayOf(RemoteInput.Builder(Notificador.CLAVE_RESPUESTA).build()), i, datos)
        RespuestaRapida().onReceive(ctx, i)
        val limite = System.currentTimeMillis() + 10_000
        while (System.currentTimeMillis() < limite && motor.llamadas.none { it.startsWith("enviar:$jid:ya voy") }) Thread.sleep(50)
        assertTrue(motor.llamadas.any { it.startsWith("enviar:$jid:ya voy") })
    }
}
