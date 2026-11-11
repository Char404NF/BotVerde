package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextInput
import com.botchat.app.datos.Ajustes
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.ModoAnim
import com.botchat.app.ui.ModoTema
import com.botchat.app.ui.PantallaAvanzado
import com.botchat.app.ui.PantallaDescargas
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

    private fun pantalla(eliminar: () -> Unit = {}, medir: () -> Unit = {}) =
        regla.setContent { BotChatTema { PantallaAvanzado(ajustes, "Mi bot", false, {}, {}, medir, eliminar) } }

    @Test fun valoresPorDefectoCumplenLoPedido() {
        assertFalse("evitar capturas viene apagado", ajustes.secure.value)
        val d = ajustes.descargas.value
        assertTrue(d.foto.wifi && d.foto.datos && d.audio.wifi && d.audio.datos && d.documento.wifi && d.documento.datos)
        assertTrue(d.video.wifi); assertFalse("los videos con datos móviles vienen apagados", d.video.datos)
        assertEquals(50, d.videoMaxMb); assertEquals(500, d.pisoLibreMb)
    }

    @Test fun descargasGuardanSusCambios() {
        regla.setContent { BotChatTema { PantallaDescargas(ajustes, {}) } }
        regla.onNodeWithTag("datos_Videos").performScrollTo().performClick()
        assertTrue(ajustes.descargas.value.video.datos)
        regla.onNodeWithTag("wifi_Fotos").performScrollTo().performClick()
        assertFalse(ajustes.descargas.value.foto.wifi)
        // Se conservan al volver a abrir la app
        Ajustes.reiniciarParaPruebas()
        val otra = Ajustes.get(Apoyo.ctx())
        assertFalse(otra.secure.value); assertTrue(otra.descargas.value.video.datos); assertFalse(otra.descargas.value.foto.wifi)
    }

    @Test fun aspectoGuardaTemaColorYAnimaciones() {
        pantalla()
        assertEquals(ModoTema.SISTEMA, ajustes.modoTema.value); assertFalse(ajustes.colorDinamico.value); assertEquals(ModoAnim.NATURAL, ajustes.animaciones.value)
        regla.onNodeWithText("Oscuro").performScrollTo().performClick()
        assertEquals(ModoTema.OSCURO, ajustes.modoTema.value)
        regla.onNodeWithTag("anim_reducidas").performScrollTo().performClick()
        assertEquals(ModoAnim.REDUCIDAS, ajustes.animaciones.value)
        regla.onNodeWithTag("sw_Color del sistema").performScrollTo().performClick()
        assertTrue(ajustes.colorDinamico.value)
        Ajustes.reiniciarParaPruebas()
        val otra = Ajustes.get(Apoyo.ctx())
        assertEquals(ModoTema.OSCURO, otra.modoTema.value); assertEquals(ModoAnim.REDUCIDAS, otra.animaciones.value)
    }

    @Test fun pausaTrasEscribirYoEs0PorDefectoYVivaEnOpcionesOcultas() {
        assertEquals("v0.5.0: el bot no calla por mí", 0, ajustes.pausaBotMin.value)
        pantalla()
        regla.onNodeWithText("Pausa del bot cuando escribo yo").assertDoesNotExist()
        regla.onNodeWithTag("sw_ocultas").performScrollTo().performClick()
        regla.onNodeWithText("Pausa del bot cuando escribo yo").performScrollTo().assertIsDisplayed()
        ajustes.fijarPausaBotMin(30)
        Ajustes.reiniciarParaPruebas()
        assertEquals("el ajuste pausaBotMin se conserva", 30, Ajustes.get(Apoyo.ctx()).pausaBotMin.value)
    }

    @Test fun lafirmaEstaApagadaPorDefectoYSeConfiguraEnFunciones() {
        assertFalse(ajustes.firmaActiva.value); assertEquals("", ajustes.firmaTexto.value)
        regla.setContent { BotChatTema { com.botchat.app.ui.PantallaFunciones(ajustes, {}) } }
        regla.onNodeWithTag("sw_Firma en las respuestas automáticas").performScrollTo().performClick()
        assertTrue(ajustes.firmaActiva.value)
        regla.onNodeWithTag("firma_texto").performScrollTo().performTextInput("Respuesta automática")
        assertEquals("Respuesta automática", ajustes.firmaTexto.value)
        Ajustes.reiniciarParaPruebas()
        val otra = Ajustes.get(Apoyo.ctx())
        assertTrue(otra.firmaActiva.value); assertEquals("Respuesta automática", otra.firmaTexto.value)
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

    @Test fun registroYMedicion() {
        var midio = false
        pantalla(medir = { midio = true })
        regla.onNodeWithText("Sin textos de mensajes, nombres ni nombres de archivos. Los números van enmascarados.").performScrollTo().assertIsDisplayed()
        regla.onNodeWithText("Medir").performScrollTo().performClick()
        assertTrue(midio)
    }
}
