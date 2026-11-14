package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaChat
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithTag
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1200dp-xxhdpi")
class ChatUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private val jid = "5215512345678@s.whatsapp.net"

    @Before fun antes() { motor = Apoyo.reinicia(); Apoyo.creaBot() }

    private fun pantalla(alVolver: () -> Unit = {}) = regla.setContent { BotChatTema { PantallaChat(jid, alVolver, {}, {}, {}) } }

    @Test fun encabezadoYBurbujasDeTodosLosTipos() {
        val ahora = System.currentTimeMillis() / 1000
        motor.chat = Apoyo.chatJson(jid, "Ana Pérez").toString()
        motor.mensajes = Apoyo.arreglo(
            Apoyo.msgJson("m9", ahora, "hasta luego", mio = true, editado = true),
            Apoyo.msgJson("m8", ahora - 10, "pie de foto", tipo = "imagen", bytes = 1048576, puedeBajarse = true),
            Apoyo.msgJson("m7", ahora - 20, tipo = "imagen", estado = "borrado"),
            Apoyo.msgJson("m6", ahora - 30, "[Ver una vez: ábrelo en tu teléfono]", tipo = "ver_una_vez", estado = "omitido"),
            Apoyo.msgJson("m5", ahora - 40, "esto se borró", borrado = true),
            Apoyo.msgJson("m4", ahora - 50, tipo = "documento", nombre = "factura.pdf", bytes = 2048, puedeBajarse = true),
            Apoyo.msgJson("m3", ahora - 60, tipo = "audio", dur = 12, bytes = 900, puedeBajarse = true, notaVoz = true),
            Apoyo.msgJson("m2", ahora - 86400 * 3, "hola de hace días"),
        )
        pantalla()
        regla.esperaTexto("hasta luego")
        regla.onNodeWithText("Ana Pérez").assertIsDisplayed()
        regla.onNodeWithText("Bot activo").assertIsDisplayed() // estado del bot en la cabecera; el teléfono va en la info del contacto
        assertEquals(0, regla.onAllNodesWithTextSafe("@s.whatsapp.net", substring = true))
        regla.onNodeWithText("(editado)", substring = true).assertIsDisplayed()
        regla.onNodeWithText("pie de foto").assertIsDisplayed()
        regla.onNodeWithText("[foto borrada]").assertIsDisplayed()
        regla.onNodeWithText("Ver una vez").assertIsDisplayed()
        regla.onNodeWithText("Ábrelo en tu teléfono").assertIsDisplayed()
        regla.onNodeWithText("esto se borró").assertIsDisplayed() // el eliminado conservado se sigue viendo, con su etiqueta
        assertEquals(0, regla.onAllNodesWithTextSafe("Mensaje borrado"))
        regla.onNodeWithText("factura.pdf").assertIsDisplayed()
        regla.onNodeWithText("Hoy").assertIsDisplayed() // separador de día
        regla.onNodeWithText("hola de hace días").assertIsDisplayed()
        // "Ver una vez" no ofrece descargar ni guardar; la foto y el audio sí ofrecen descargar.
        regla.onNodeWithText("Descargar · 1 MB").assertIsDisplayed()
        regla.onNodeWithText("Descargar", substring = false).assertIsDisplayed()
        regla.onNodeWithText("Descargar · 1 MB").performClick()
        regla.waitUntil(5_000) { motor.llamadas.contains("descargar:$jid:m8") }
    }

    @Test fun grupoMuestraQuienEscribeUnaVezPorRacha() {
        val ahora = System.currentTimeMillis() / 1000
        motor.chat = Apoyo.chatJson("120@g.us", "Equipo", tel = "", pais = "", grupo = true).toString()
        motor.mensajes = Apoyo.arreglo(
            Apoyo.msgJson("g3", ahora, "tres", remitente = "Luis", remitenteId = "luis"),
            Apoyo.msgJson("g2", ahora - 5, "dos", remitente = "Luis", remitenteId = "luis"),
            Apoyo.msgJson("g1", ahora - 10, "uno", remitente = "Ana", remitenteId = "ana"),
        )
        regla.setContent { BotChatTema { PantallaChat("120@g.us", {}, {}, {}, {}) } }
        regla.esperaTexto("tres")
        regla.onAllNodesWithText("Equipo").onFirst().assertIsDisplayed() // cabecera (y la tarjeta de inicio del grupo)
        regla.onNodeWithText("Bot activo").assertIsDisplayed()
        regla.onNodeWithText("Ana").assertIsDisplayed()
        assertEquals("Luis aparece una sola vez porque sus dos mensajes van seguidos", 1, regla.onAllNodesWithTextSafe("Luis"))
    }

    @Test fun borrarArchivosPideConfirmacionConElTamano() {
        motor.chat = Apoyo.chatJson(jid, "Ana Pérez").toString()
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("m1", System.currentTimeMillis() / 1000, "hola"))
        motor.liberar = """{"bytes":3072,"cantidad":2}"""
        pantalla()
        regla.esperaTexto("hola")
        regla.onNodeWithContentDescription("Más opciones").performClick()
        regla.onNodeWithTag("menu_mas").performClick()
        regla.onNodeWithText("Borrar archivos de este chat").performClick()
        regla.esperaTexto("Se liberarán 3 KB", substring = true)
        regla.onNodeWithText("Los mensajes de texto se conservan.", substring = true).assertIsDisplayed()
        regla.onNodeWithText("Cancelar").performClick()
        assertTrue(motor.llamadas.none { it.startsWith("borrarArchivos") })
        // Ahora confirmando
        regla.onNodeWithContentDescription("Más opciones").performClick()
        regla.onNodeWithTag("menu_mas").performClick()
        regla.onNodeWithText("Borrar archivos de este chat").performClick()
        regla.esperaTexto("Borrar archivos", substring = false)
        regla.onNodeWithText("Borrar archivos").performClick()
        regla.waitUntil(5_000) { motor.llamadas.any { it.startsWith("borrarArchivos") } }
        assertTrue(motor.llamadas.contains("borrarArchivos:$jid||0"))
    }

    @Test fun borrarTodoElChat() {
        motor.chat = Apoyo.chatJson(jid, "Ana Pérez").toString()
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("m1", System.currentTimeMillis() / 1000, "hola"))
        var volvio = false
        pantalla(alVolver = { volvio = true })
        regla.esperaTexto("hola")
        regla.onNodeWithContentDescription("Más opciones").performClick()
        regla.onNodeWithTag("menu_mas").performClick()
        regla.onNodeWithText("Borrar todo el chat").performClick()
        regla.onNodeWithText("No se borra nada en WhatsApp.", substring = true).assertIsDisplayed()
        regla.onNodeWithText("Borrar todo").performClick()
        regla.waitUntil(5_000) { motor.llamadas.contains("borrarChat:$jid") }
        Thread.sleep(300)
        org.robolectric.Shadows.shadowOf(android.os.Looper.getMainLooper()).idle()
        regla.waitForIdle()
        try { regla.waitUntil(5_000) { volvio } } catch (e: Throwable) { throw AssertionError("llamadas=" + motor.llamadas + " toast=" + org.robolectric.shadows.ShadowToast.getTextOfLatestToast(), e) }
        assertTrue(motor.llamadas.contains("borrarChat:$jid"))
    }
}
