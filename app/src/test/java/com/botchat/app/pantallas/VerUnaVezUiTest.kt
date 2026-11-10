package com.botchat.app.pantallas

import android.graphics.Bitmap
import android.view.WindowManager
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.activity.ComponentActivity
import com.botchat.app.seguridad.FormatoCifrado
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaChat
import com.botchat.app.ui.PantallaVerUnaVez
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import java.io.ByteArrayOutputStream
import java.io.File

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class VerUnaVezUiTest {
    @get:Rule val regla = createAndroidComposeRule<ComponentActivity>()
    private lateinit var motor: MotorFalso
    private var bot = 0L
    private val jid = "5215512345678@s.whatsapp.net"
    private val ahora = System.currentTimeMillis() / 1000

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot(); motor.chat = Apoyo.chatJson(jid, "Ana Pérez").toString() }

    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }

    private fun guardaImagen() {
        val png = ByteArrayOutputStream().also { Bitmap.createBitmap(30, 20, Bitmap.Config.ARGB_8888).compress(Bitmap.CompressFormat.PNG, 100, it) }.toByteArray()
        File(Apoyo.ctx().filesDir, "media/1/ab/vo.enc").apply { parentFile!!.mkdirs(); writeBytes(FormatoCifrado.cifrarBytes(Apoyo.clave, png)) }
    }

    @Test fun seAbreEnPantallaSeguraYAlCerrarSeBorra() {
        guardaImagen()
        var cerrada = false
        regla.setContent { BotChatTema { PantallaVerUnaVez(bot, jid, "VO1", { cerrada = true }) } }
        regla.esperaEtiqueta("imagen_ver_una_vez")
        assertTrue("pantalla segura", regla.activity.window.attributes.flags and WindowManager.LayoutParams.FLAG_SECURE != 0)
        assertTrue(llamo("abrirUnaVez:VO1"))
        assertFalse("no hay botones de guardar ni compartir", hay("guardar") || hay("compartir"))
        regla.onNodeWithTag("cerrar_ver_una_vez").performClick()
        regla.espera { llamo("cerrarUnaVez:VO1") && cerrada }
    }

    @Test fun siYaNoEstaDisponibleLoDiceYNoBorraNada() {
        motor.verUnaVezFalla = "Este mensaje de “ver una vez” ya no está disponible."
        regla.setContent { BotChatTema { PantallaVerUnaVez(bot, jid, "VO2", {}) } }
        regla.esperaEtiqueta("ver_una_vez_error")
        regla.onNodeWithText("ya no está disponible", substring = true, useUnmergedTree = true).assertIsDisplayed()
        regla.waitForIdle()
        assertFalse("si no se pudo abrir no hay nada que cerrar", llamo("cerrarUnaVez:"))
    }

    @Test fun laTarjetaDelChatSigueElEstadoDelVerUnaVez() {
        motor.mensajes = Apoyo.arreglo(
            Apoyo.msgJson("p", ahora, tipo = "ver_una_vez", estado = "descargado", ruta = "media/1/ab/vo.enc", remitente = "Ana").put("verEstado", "pendiente").put("verTipo", "imagen"),
            Apoyo.msgJson("v", ahora - 10, tipo = "ver_una_vez", remitente = "Ana").put("verEstado", "visto"),
            Apoyo.msgJson("x", ahora - 20, tipo = "ver_una_vez", remitente = "Ana").put("verEstado", "expirado"),
            Apoyo.msgJson("r", ahora - 30, texto = "[Ver una vez: ábrelo en tu teléfono]", tipo = "ver_una_vez", remitente = "Ana", estado = "omitido"),
            Apoyo.msgJson("d", ahora - 40, tipo = "ver_una_vez", estado = "pendiente", remitente = "Ana").put("verEstado", "pendiente"),
        )
        var abierto = ""
        regla.setContent { BotChatTema { PantallaChat(jid, {}, {}, {}, {}, alAbrirUnaVez = { abierto = it }) } }
        regla.esperaTexto("Toca para abrir. Solo se puede ver una vez.")
        regla.onNodeWithText("Ya visto").assertIsDisplayed()
        regla.onNodeWithText("Expiró").assertIsDisplayed()
        regla.onNodeWithText("Ábrelo en tu teléfono").assertIsDisplayed()
        regla.onNodeWithText("Descargando…").assertIsDisplayed()
        assertEquals("solo el pendiente descargado se abre", 5, regla.onAllNodesWithTag("tarjeta_ver_una_vez", useUnmergedTree = true).fetchSemanticsNodes().size)
        regla.onNodeWithText("Toca para abrir. Solo se puede ver una vez.").performClick()
        regla.espera { abierto == "p" }
    }
}
