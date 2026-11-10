package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import com.botchat.app.datos.Ajustes
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaAvanzado
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h891dp-xxhdpi")
class AvanzadoUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var ajustes: Ajustes

    @Before fun antes() { Apoyo.reinicia(); ajustes = Ajustes.get(Apoyo.ctx()) }

    private fun pantalla(eliminar: () -> Unit = {}, medir: () -> Unit = {}, pin: () -> Unit = {}) =
        regla.setContent { BotChatTema { PantallaAvanzado(ajustes, "Mi bot", false, {}, pin, {}, medir, eliminar) } }

    @Test fun valoresPorDefectoCumplenLoPedido() {
        assertTrue("FLAG_SECURE activado por defecto", ajustes.secure.value)
        val d = ajustes.descargas.value
        assertTrue(d.foto.wifi && d.foto.datos && d.audio.wifi && d.audio.datos && d.documento.wifi && d.documento.datos)
        assertTrue(d.video.wifi); assertFalse("los videos con datos móviles vienen apagados", d.video.datos)
        assertEquals(50, d.videoMaxMb); assertEquals(500, d.pisoLibreMb)
    }

    @Test fun interruptoresGuardanSusCambios() {
        pantalla()
        regla.onNodeWithTag("sw_Bloquear capturas de pantalla").performScrollTo().performClick()
        assertFalse(ajustes.secure.value)
        regla.onNodeWithTag("datos_Videos").performScrollTo().performClick()
        assertTrue(ajustes.descargas.value.video.datos)
        regla.onNodeWithTag("wifi_Fotos").performScrollTo().performClick()
        assertFalse(ajustes.descargas.value.foto.wifi)
        // Se conservan al volver a abrir la app
        Ajustes.reiniciarParaPruebas()
        val otra = Ajustes.get(Apoyo.ctx())
        assertFalse(otra.secure.value); assertTrue(otra.descargas.value.video.datos); assertFalse(otra.descargas.value.foto.wifi)
    }

    @Test fun reglasDeDescargaViajanAlModuloGo() {
        val json = Ajustes.jsonDescargas(ajustes.descargas.value)
        val o = org.json.JSONObject(json)
        assertEquals(50L * 1024 * 1024, o.getLong("videoMaxBytes"))
        assertEquals(500L * 1024 * 1024, o.getLong("pisoLibre"))
        assertFalse(o.getJSONObject("video").getBoolean("datos")); assertTrue(o.getJSONObject("video").getBoolean("wifi"))
    }

    @Test fun eliminarBotPideConfirmacion() {
        var eliminado = false
        pantalla(eliminar = { eliminado = true })
        regla.onNodeWithText("Eliminar bot").performScrollTo().performClick()
        regla.onNodeWithText("¿Eliminar el bot?").assertIsDisplayed()
        regla.onNodeWithText("Cancelar").performClick()
        assertFalse(eliminado)
        regla.onNodeWithText("Eliminar bot").performScrollTo().performClick()
        regla.onNodeWithText("Sí, eliminar").performClick()
        assertTrue(eliminado)
    }

    @Test fun pinSeCreaYSeQuita() {
        var pedido = false
        pantalla(pin = { pedido = true })
        regla.onNodeWithText("Crear PIN").performScrollTo().performClick()
        assertTrue(pedido)
        ajustes.fijarPin("1357")
        regla.waitForIdle()
        regla.onNodeWithText("Cambiar").performScrollTo().assertIsDisplayed()
        regla.onNodeWithText("Quitar").performScrollTo().performClick()
        regla.onNodeWithText("¿Quitar el PIN?").assertIsDisplayed()
        regla.onNodeWithText("Quitar PIN").performClick()
        regla.waitUntil(5_000) { !ajustes.tienePin.value }
    }

    @Test fun registroYMedicion() {
        var midio = false
        pantalla(medir = { midio = true })
        regla.onNodeWithText("Sin textos de mensajes, nombres ni nombres de archivos. Los números van enmascarados.").performScrollTo().assertIsDisplayed()
        regla.onNodeWithText("Medir").performScrollTo().performClick()
        assertTrue(midio)
    }
}
