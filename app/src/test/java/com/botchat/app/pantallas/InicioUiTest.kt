package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaInicio
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h891dp-xxhdpi")
class InicioUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso

    @Before fun antes() { motor = Apoyo.reinicia(); Apoyo.creaBot() }

    private fun pantalla(sinSesion: Boolean = false, alChat: (String) -> Unit = {}, alRevincular: () -> Unit = {}) =
        regla.setContent { BotChatTema { PantallaInicio(sinSesion, alChat, {}, {}, {}, alRevincular) } }

    @Test fun listaConPestanasYBuscador() {
        motor.chatsPersonales = Apoyo.arreglo(
            Apoyo.chatJson("a@s.whatsapp.net", "María López", tipo = "imagen", texto = "mira"),
            Apoyo.chatJson("b@s.whatsapp.net", "José Peña", tel = "+34 612 34 56 78", pais = "ES", texto = "Hola José"),
        )
        motor.chatsGrupos = Apoyo.arreglo(Apoyo.chatJson("g@g.us", "Familia", tel = "", pais = "", grupo = true, texto = "¿quién viene?"))
        pantalla()
        regla.esperaTexto("María López")
        regla.onNodeWithText("Personales").assertIsDisplayed()
        regla.onNodeWithText("Grupos").assertIsDisplayed()
        regla.onNodeWithText("Foto · mira").assertIsDisplayed()
        regla.onNodeWithText("Hola José").assertIsDisplayed()
        regla.onNodeWithText("José Peña").assertIsDisplayed()
        assertEquals(0, regla.onAllNodesWithTextSafe("Familia"))
        // Pestaña Grupos
        regla.onNodeWithText("Grupos").performClick()
        regla.esperaTexto("Familia")
        regla.onNodeWithText("¿quién viene?").assertIsDisplayed()
        // Buscador
        regla.onNodeWithText("Personales").performClick()
        regla.onNodeWithContentDescription("Buscar").performClick()
        regla.onNode(hasSetTextAction()).performTextInput("jos")
        regla.waitUntil(5_000) { regla.onAllNodesWithTextSafe("María López") == 0 }
        regla.onNodeWithText("José Peña").assertIsDisplayed()
        assertTrue(motor.consultas.contains("jos"))
    }

    @Test fun sinMensajesMuestraAyuda() {
        pantalla()
        regla.esperaTexto("Aún no hay mensajes")
        regla.onNodeWithText("Cuando alguien escriba al número del bot, la conversación aparecerá aquí.").assertIsDisplayed()
    }

    @Test fun tocarUnChatLoAbre() {
        motor.chatsPersonales = Apoyo.arreglo(Apoyo.chatJson("a@s.whatsapp.net", "María López"))
        var abierto = ""
        pantalla(alChat = { abierto = it })
        regla.esperaTexto("María López")
        regla.onNodeWithText("María López").performClick()
        assertEquals("a@s.whatsapp.net", abierto)
    }

    @Test fun avisoCuandoWhatsAppCierraLaSesion() {
        var vinculado = false
        pantalla(sinSesion = true, alRevincular = { vinculado = true })
        regla.onNodeWithText("WhatsApp cerró la sesión de este bot").assertIsDisplayed()
        regla.onNodeWithText("Tus mensajes guardados siguen aquí.").assertIsDisplayed()
        regla.onNodeWithText("Vincular de nuevo").performClick()
        assertTrue(vinculado)
        regla.onNodeWithText("Sin sesión").assertIsDisplayed()
    }
}
