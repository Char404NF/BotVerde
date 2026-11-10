package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import com.botchat.app.datos.Ajustes
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaBloqueo
import com.botchat.app.ui.PantallaConfigurarPin
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
class BloqueoUiTest {
    @get:Rule val regla = createComposeRule()

    @Before fun antes() { Apoyo.reinicia() }

    private fun teclea(pin: String) { pin.forEach { regla.onNodeWithText(it.toString()).performClick() } }

    @Test fun pinCorrectoDesbloquea() {
        val a = Ajustes.get(Apoyo.ctx()); a.fijarPin("4821")
        var abierto = false
        regla.setContent { BotChatTema { PantallaBloqueo(a) { abierto = true } } }
        regla.onNodeWithText("Bot Chat está bloqueado").assertIsDisplayed()
        teclea("4821"); regla.onNodeWithText("OK").performClick()
        regla.waitUntil(8_000) { abierto }
    }

    @Test fun pinIncorrectoAvisaCuantosIntentosQuedanYLuegoBloquea() {
        val a = Ajustes.get(Apoyo.ctx()); a.fijarPin("4821")
        var abierto = false
        regla.setContent { BotChatTema { PantallaBloqueo(a) { abierto = true } } }
        teclea("1111"); regla.onNodeWithText("OK").performClick()
        regla.esperaTexto("PIN incorrecto. Te quedan 4 intentos.")
        assertFalse(abierto)
        for (quedan in listOf(3, 2, 1)) {
            teclea("1111"); regla.onNodeWithText("OK").performClick()
            regla.esperaTexto("PIN incorrecto. Te quedan $quedan intento", substring = true)
        }
        teclea("1111"); regla.onNodeWithText("OK").performClick()
        regla.esperaTexto("Demasiados intentos. Espera", substring = true)
        // Durante la espera ni el PIN correcto entra.
        teclea("4821")
        assertFalse(abierto)
    }

    @Test fun crearPinPideConfirmacion() {
        var guardado = ""
        regla.setContent { BotChatTema { PantallaConfigurarPin("Protege tus mensajes con un PIN", { guardado = it }, { }) } }
        regla.onNodeWithText("Elige un PIN de 4 a 6 dígitos").assertIsDisplayed()
        teclea("2468"); regla.onNodeWithText("OK").performClick()
        regla.onNodeWithText("Escríbelo otra vez para confirmar").assertIsDisplayed()
        teclea("2469"); regla.onNodeWithText("OK").performClick()
        regla.onNodeWithText("Los PIN no coinciden. Empieza de nuevo.").assertIsDisplayed()
        assertEquals("", guardado)
        teclea("2468"); regla.onNodeWithText("OK").performClick()
        teclea("2468"); regla.onNodeWithText("OK").performClick()
        assertEquals("2468", guardado)
        regla.onNodeWithText("Ahora no").assertIsDisplayed()
    }

    @Test fun elPinNoSeGuardaEnClaro() {
        val a = Ajustes.get(Apoyo.ctx()); a.fijarPin("135790")
        val prefs = Apoyo.ctx().getSharedPreferences("seguridad", android.content.Context.MODE_PRIVATE)
        val guardado = prefs.all.values.joinToString("|")
        assertFalse(guardado.contains("135790"))
        assertTrue(prefs.getString("pin", "")!!.startsWith("v1$"))
    }
}
