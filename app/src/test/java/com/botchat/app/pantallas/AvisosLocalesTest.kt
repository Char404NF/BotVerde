package com.botchat.app.pantallas

import android.app.NotificationManager
import androidx.core.app.NotificationCompat
import androidx.test.core.app.ApplicationProvider
import com.botchat.app.Notificador
import com.botchat.app.datos.Ajustes
import com.botchat.app.datos.Repo
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class)
class AvisosLocalesTest {
    private lateinit var motor: MotorFalso
    private val ctx get() = ApplicationProvider.getApplicationContext<android.app.Application>()
    private val ambito = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    private fun nm() = ctx.getSystemService(NotificationManager::class.java)

    @Before fun antes() {
        motor = Apoyo.reinicia(); Apoyo.creaBot()
        shadowOf(ctx).grantPermissions(android.Manifest.permission.POST_NOTIFICATIONS)
        nm().cancelAll()
        Notificador.iniciar(ctx, ambito)
    }
    @After fun despues() { ambito.cancel() }

    private fun espera(c: () -> Boolean) {
        val limite = System.currentTimeMillis() + 15_000
        while (System.currentTimeMillis() < limite && !c()) Thread.sleep(50)
    }

    @Test fun elAvisoDeHablarConUnaPersonaEsNeutroBloqueadoYCompletoDesbloqueado() {
        motor.aviso = """{"id":5,"ts":1,"tipo":"humano","chat":"5@s.whatsapp.net","titulo":"Alguien pidió hablar con una persona","detalle":"Ana Pérez · +52 55 1234 5678"}"""
        Repo.emitirParaPruebas("aviso:5")
        espera { shadowOf(nm()).size() == 1 }
        val n = shadowOf(nm()).allNotifications.single()
        assertEquals(NotificationCompat.VISIBILITY_PRIVATE, n.visibility)
        assertEquals("Alguien pidió hablar con una persona", n.publicVersion.extras.getCharSequence("android.title").toString())
        assertNull("la versión pública no lleva detalle", n.publicVersion.extras.getCharSequence("android.text"))
        assertEquals("Ana Pérez · +52 55 1234 5678", n.extras.getCharSequence("android.text").toString())
    }

    @Test fun conElInterruptorDeAvanzadoElDetalleSeVeTambienBloqueado() {
        Ajustes.get(ctx).fijarDetalleBloqueado(true)
        motor.aviso = """{"id":6,"ts":1,"tipo":"humano","chat":"","titulo":"Alguien pidió hablar con una persona","detalle":"Beto · +52 55 0000 0000"}"""
        Repo.emitirParaPruebas("aviso:6")
        espera { shadowOf(nm()).size() == 1 }
        val n = shadowOf(nm()).allNotifications.single()
        assertEquals(NotificationCompat.VISIBILITY_PUBLIC, n.visibility)
        assertEquals("Beto · +52 55 0000 0000", n.extras.getCharSequence("android.text").toString())
    }

    @Test fun elInterruptorEstaApagadoPorDefecto() {
        assertFalse(Ajustes.get(ctx).detalleBloqueado.value)
    }

    @Test fun sinPermisoNoSeMuestraNada() {
        shadowOf(ctx).denyPermissions(android.Manifest.permission.POST_NOTIFICATIONS)
        motor.aviso = """{"id":7,"ts":1,"tipo":"solicitud","chat":"","titulo":"Nueva solicitud","detalle":"x"}"""
        Repo.emitirParaPruebas("aviso:7")
        Thread.sleep(800)
        assertEquals(0, shadowOf(nm()).size())
    }
}
