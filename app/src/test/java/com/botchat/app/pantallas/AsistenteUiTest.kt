package com.botchat.app.pantallas

import android.Manifest
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.onAllNodesWithText
import com.botchat.app.BotState
import com.botchat.app.Datos
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.Raiz
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h891dp-xxhdpi")
class AsistenteUiTest {
    @get:Rule val regla = createComposeRule()

    @Before fun antes() { Apoyo.reinicia(); BotState.codigo.value = null; com.botchat.app.datos.Ajustes.get(Apoyo.ctx()).aceptarAviso() /* la bienvenida tiene su propia prueba */ }

    @Test fun pasoAPasoHastaElCodigo() {
        regla.setContent { BotChatTema { Raiz() } }
        // Paso 1
        regla.onNodeWithText("¿Cómo se llamará tu bot?").assertIsDisplayed()
        regla.onNodeWithText("Paso 1 de 3").assertIsDisplayed()
        regla.onNode(hasSetTextAction()).performTextInput("a")
        regla.onNodeWithText("Siguiente").performClick()
        regla.onNodeWithText("El nombre es muy corto: escribe al menos 2 letras.").assertIsDisplayed()
        regla.onNode(hasSetTextAction()).performTextReplacement("bot de prueba")
        regla.onNodeWithText("Siguiente").performClick()
        // Paso 2: México viene elegido; un número corto da un error claro
        regla.waitForIdle()
        regla.onNodeWithText("Paso 2 de 3").assertIsDisplayed()
        regla.onNodeWithText("México").assertIsDisplayed()
        regla.onNode(hasSetTextAction()).performTextInput("5512345")
        regla.onNodeWithText("Siguiente").performClick()
        regla.onNodeWithText("El número es muy corto: faltan 3 dígitos.").assertIsDisplayed()
        regla.onNode(hasSetTextAction()).performTextInput("678")
        regla.onNodeWithText("Siguiente").performClick()
        // Paso 3
        regla.waitForIdle()
        regla.onNodeWithText("¿Es correcto?").assertIsDisplayed()
        regla.onNodeWithText("bot de prueba").assertIsDisplayed()
        regla.onNodeWithText("+52 55 1234 5678").assertIsDisplayed()
        // Confirmar crea el bot y pide el código (el servicio solo se manda a arrancar)
        shadowOf(Apoyo.ctx() as android.app.Application).grantPermissions(Manifest.permission.POST_NOTIFICATIONS)
        regla.onNodeWithText("Sí, vincular").performClick()
        regla.waitForIdle()
        regla.onNodeWithText("Pidiendo el código a WhatsApp…").assertIsDisplayed()
        val bot = Datos.get(Apoyo.ctx()).primero()
        assertNotNull(bot)
        assertEquals("525512345678", bot!!.vincular)
        assertTrue(!bot.vinculado)
        // Llega el código desde el módulo Go
        BotState.codigo.value = "ABCD-EFGH"
        regla.waitForIdle()
        regla.onNodeWithText("ABCD-EFGH").assertIsDisplayed()
        regla.onNodeWithText("Copiar código").assertIsDisplayed()
        regla.onNodeWithText("Esta pantalla pasa sola cuando WhatsApp acepte el código.").assertIsDisplayed()
    }

    @Test fun listaDePaisesConBuscadorYBandera() {
        regla.setContent { BotChatTema { Raiz() } }
        regla.onNode(hasSetTextAction()).performTextInput("Pruebas")
        regla.onNodeWithText("Siguiente").performClick()
        regla.waitForIdle()
        regla.onNodeWithText("México").performClick() // abre la lista
        regla.waitForIdle()
        regla.onNodeWithText("Elige tu país").assertIsDisplayed()
        // México está preseleccionado y visible aunque la lista es larga (se desplaza hasta él)
        regla.onNodeWithText("+52").assertIsDisplayed()
        regla.onNode(hasSetTextAction()).performTextInput("arge")
        regla.waitForIdle()
        regla.onNodeWithText("Argentina").assertIsDisplayed()
        assertEquals(0, regla.onAllNodesWithText("Alemania").fetchSemanticsNodes().size)
        regla.onNodeWithText("Argentina").performClick()
        regla.waitForIdle()
        regla.onNodeWithText("+54").assertIsDisplayed()
    }
}
