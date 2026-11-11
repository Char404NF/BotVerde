package com.botchat.app.pantallas

import android.content.ClipboardManager
import android.content.Context
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.test.core.app.ApplicationProvider
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaCapturaLab
import com.botchat.app.ui.PantallaLaboratorio
import org.json.JSONObject
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
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class CapturaLabUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun item(n: Int, arbol: String) = JSONObject().put("n", n).put("ts", 1700000000L + n).put("chat", "privado").put("clasif", "Tipo=otro").put("detalle", "-").put("arbol", arbol)

    @Test fun nacesApagadaConAvisoDeSoloEstructuraYSinLista() {
        regla.setContent { BotChatTema { PantallaCapturaLab(bot, {}) } }
        regla.esperaEtiqueta("captura_activo")
        regla.onNodeWithTag("captura_aviso", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("captura_estado_texto", useUnmergedTree = true).assertIsDisplayed()
        assertTrue(hay("captura_vacio"))
        regla.onNodeWithTag("captura_copiar", useUnmergedTree = true).assertIsNotEnabled()
        regla.onNodeWithTag("captura_borrar", useUnmergedTree = true).assertIsNotEnabled()
        assertFalse(motor.capActiva)
    }

    @Test fun interruptoresLlamanAlMotor() {
        regla.setContent { BotChatTema { PantallaCapturaLab(bot, {}) } }
        regla.esperaEtiqueta("captura_activo")
        regla.onNodeWithTag("captura_activo", useUnmergedTree = true).performClick()
        regla.waitUntil(5000) { motor.capActiva }
        assertTrue(llamo("activarCaptura:true"))
        regla.onNodeWithTag("captura_textos", useUnmergedTree = true).performClick()
        regla.waitUntil(5000) { motor.capTextos }
        assertTrue(llamo("textosCaptura:true"))
    }

    @Test fun muestraLaListaCopiaTodoYBorra() {
        motor.capItems += item(1, "Message\n  conversation = texto(5)")
        motor.capItems += item(2, "Message\n  templateButtonReplyMessage\n    selectedID = \"bc|X|1\" (6)")
        regla.setContent { BotChatTema { PantallaCapturaLab(bot, {}) } }
        regla.esperaEtiqueta("captura_item_2")
        assertTrue(hay("captura_item_1"))
        regla.onNodeWithTag("captura_copiar", useUnmergedTree = true).performScrollTo().performClick()
        regla.waitForIdle()
        val cm = ApplicationProvider.getApplicationContext<Context>().getSystemService(ClipboardManager::class.java)
        val t = cm.primaryClip!!.getItemAt(0).text.toString()
        assertTrue(t, t.contains("#1 · privado · Tipo=otro") && t.contains("templateButtonReplyMessage") && t.contains("conversation = texto(5)"))
        regla.onNodeWithTag("captura_borrar", useUnmergedTree = true).performScrollTo().performClick()
        regla.waitUntil(5000) { motor.capItems.isEmpty() }
        assertTrue(llamo("borrarCaptura"))
        regla.waitUntil(5000) { hay("captura_vacio") }
    }

    @Test fun encendidaMuestraElReloj() {
        motor.capActiva = true
        regla.setContent { BotChatTema { PantallaCapturaLab(bot, {}) } }
        regla.esperaEtiqueta("captura_estado_texto")
        assertTrue(hay("captura_estado_texto"))
        assertEquals(true, motor.capActiva)
    }

    @Test fun laPantallaDelLaboratorioOfreceLaCapturaAunqueEsteApagado() {
        var abierta = false
        regla.setContent { BotChatTema { PantallaLaboratorio(bot, {}, {}, {}, { abierta = true }) } }
        regla.esperaEtiqueta("lab_abrir_captura")
        regla.onNodeWithTag("lab_abrir_captura", useUnmergedTree = true).performClick()
        assertTrue(abierta)
    }
}
