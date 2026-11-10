package com.botchat.app.pantallas

import androidx.activity.ComponentActivity
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import com.botchat.app.datos.Ajustes
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.EstadoBloqueo
import com.botchat.app.ui.Raiz
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
class NavegacionUiTest {
    @get:Rule val regla = createAndroidComposeRule<ComponentActivity>()
    private lateinit var motor: MotorFalso
    private val jid = "5215512345678@s.whatsapp.net"

    @Before fun antes() {
        motor = Apoyo.reinicia(); Apoyo.creaBot()
        motor.chatsPersonales = Apoyo.arreglo(Apoyo.chatJson(jid, "María López", texto = "nos vemos"))
        motor.chat = Apoyo.chatJson(jid, "María López").toString()
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("m1", System.currentTimeMillis() / 1000, "nos vemos"))
        motor.espacioPorChat = "[]"
    }

    private fun atras() { regla.runOnUiThread { regla.activity.onBackPressedDispatcher.onBackPressed() }; regla.waitForIdle() }

    @Test fun sinPinOfreceCrearUnoYSePuedeOmitir() {
        regla.setContent { BotChatTema { Raiz() } }
        regla.onNodeWithText("Protege tus mensajes con un PIN").assertIsDisplayed()
        regla.onNodeWithText("Ahora no").performClick()
        regla.esperaTexto("María López")
        regla.onNodeWithText("Mensajes").assertIsDisplayed()
    }

    @Test fun recorridoPorLasPantallas() {
        Ajustes.get(Apoyo.ctx()).omitirPin()
        regla.setContent { BotChatTema { Raiz() } }
        regla.esperaTexto("María López")
        // lista -> chat
        regla.onNodeWithText("María López").performClick()
        regla.esperaTexto("nos vemos", substring = false)
        regla.onNodeWithContentDescription("Archivos compartidos").assertIsDisplayed()
        // chat -> archivos compartidos -> atrás
        regla.onNodeWithContentDescription("Archivos compartidos").performClick()
        regla.esperaTexto("Archivos compartidos")
        regla.onNodeWithText("Fotos").assertIsDisplayed(); regla.onNodeWithText("Videos").assertIsDisplayed()
        regla.onNodeWithText("Audios").assertIsDisplayed(); regla.onNodeWithText("Documentos").assertIsDisplayed()
        atras()
        regla.onNodeWithContentDescription("Archivos compartidos").assertIsDisplayed() // de vuelta en el chat
        atras()
        regla.esperaTexto("Mensajes")
        // lista -> Espacio -> atrás
        regla.onNodeWithContentDescription("Espacio").performClick()
        regla.esperaTexto("Chats que más ocupan")
        atras()
        // lista -> menú -> Avanzado
        regla.onNodeWithContentDescription("Más opciones").performClick()
        regla.onNodeWithText("Avanzado").performClick()
        regla.esperaTexto("Seguridad")
        regla.onNodeWithText("Descarga automática").assertIsDisplayed()
        atras()
        // lista -> menú -> Estado del bot
        regla.onNodeWithContentDescription("Más opciones").performClick()
        regla.onNodeWithText("Estado del bot").performClick()
        regla.esperaTexto("Iniciar")
        regla.onNodeWithText("¿Se apaga solo? Ver ayuda de batería").performClick()
        regla.esperaTexto("Que el bot no se apague")
    }

    @Test fun conPinPideDesbloquearYAlBloquearLimpiaLoDescifrado() {
        val a = Ajustes.get(Apoyo.ctx()); a.fijarPin("2580")
        EstadoBloqueo.iniciar(a)
        regla.setContent { BotChatTema { Raiz() } }
        regla.onNodeWithText("BotChat está bloqueado").assertIsDisplayed()
        assertEquals(0, regla.onAllNodesWithTextSafe("María López")) // nada de los mensajes antes del PIN
        "2580".forEach { regla.onNodeWithText(it.toString()).performClick() }
        regla.onNodeWithText("OK").performClick()
        regla.esperaTexto("María López")
        // Al bloquear se vuelve a la pantalla del PIN y se borran las copias temporales.
        val temporal = java.io.File(Apoyo.ctx().cacheDir, "tmp_descifrado/x/secreto.pdf").apply { parentFile!!.mkdirs(); writeText("dato") }
        regla.runOnUiThread { EstadoBloqueo.bloquear(Apoyo.ctx()) }
        regla.esperaTexto("BotChat está bloqueado")
        assertEquals(0, regla.onAllNodesWithTextSafe("María López"))
        assertTrue("las copias temporales se borran al bloquear", !temporal.exists())
    }

    @Test fun sinSesionApareceVincularDeNuevo() {
        Ajustes.get(Apoyo.ctx()).omitirPin()
        com.botchat.app.Datos.get(Apoyo.ctx()).marcarVinculado(1, false) // WhatsApp cerró la sesión; el bot se había vinculado antes
        regla.setContent { BotChatTema { Raiz() } }
        regla.esperaTexto("WhatsApp cerró la sesión de este bot")
        regla.onNodeWithText("María López").assertIsDisplayed() // los mensajes siguen ahí
        regla.onNodeWithText("Vincular de nuevo").performClick()
        regla.esperaTexto("Vincular de nuevo")
        regla.onNodeWithText("Tus mensajes guardados no se pierden.", substring = true).assertIsDisplayed()
        regla.onNodeWithText("Sí, vincular").assertIsDisplayed()
    }
}
