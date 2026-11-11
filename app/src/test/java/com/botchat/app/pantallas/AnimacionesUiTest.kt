package com.botchat.app.pantallas

import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.size
import androidx.compose.material3.Text
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.Ajustes
import com.botchat.app.ui.ArranqueLogo
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.CheckAnimado
import com.botchat.app.ui.IndicadorEstado
import com.botchat.app.ui.EstadoVisible
import com.botchat.app.ui.LocalReducirMovimiento
import com.botchat.app.ui.LogoBot
import com.botchat.app.ui.LogoDibujado
import com.botchat.app.ui.Splash
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
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
class AnimacionesUiTest {
    @get:Rule val regla = createComposeRule()

    @Before fun antes() { Apoyo.reinicia() }

    @Test fun elSplashDuraMenosDeUnSegundoYNoBloqueaLaApp() {
        ArranqueLogo.mostrado = false
        regla.mainClock.autoAdvance = false
        regla.setContent { BotChatTema { androidx.compose.foundation.layout.Box(Modifier.fillMaxSize()) { Text("contenido"); Splash() } } }
        regla.onNodeWithText("contenido").assertIsDisplayed() // la app ya está debajo desde el primer cuadro
        assertTrue(ArranqueLogo.mostrado)
        regla.mainClock.advanceTimeBy(1_000)
        regla.mainClock.autoAdvance = true
        regla.waitForIdle()
        regla.onNodeWithText("contenido").assertIsDisplayed()
    }

    @Test fun conAnimacionesReducidasElSplashNoSeMuestra() {
        ArranqueLogo.mostrado = false
        Ajustes.get(Apoyo.ctx()).fijarAnimaciones(com.botchat.app.ui.ModoAnim.REDUCIDAS)
        regla.setContent { BotChatTema { Splash() } }
        regla.waitForIdle()
        assertTrue(ArranqueLogo.mostrado)
    }

    @Test fun logoYPalomitaSeDibujanSinErrores() {
        var anillo by mutableStateOf(0f); var flecha by mutableStateOf(0f)
        regla.setContent { BotChatTema { LogoBot(Modifier.size(48.dp)); LogoDibujado(anillo, flecha, Modifier.size(96.dp)); CheckAnimado() } }
        for (p in listOf(0f, 0.3f, 0.75f, 1f)) { anillo = p; flecha = p; regla.waitForIdle() }
    }

    @Test fun elIndicadorDeEstadoSeMuestraEnCadaEstado() {
        var e by mutableStateOf(EstadoVisible.CONECTADO)
        regla.setContent { BotChatTema { IndicadorEstado(e) } }
        for ((estado, texto) in listOf(EstadoVisible.CONECTADO to "Conectado", EstadoVisible.RECONECTANDO to "Reconectando", EstadoVisible.DESVINCULADO to "Desvinculado", EstadoVisible.DETENIDO to "Detenido")) {
            e = estado; regla.waitForIdle(); regla.onNodeWithText(texto).assertIsDisplayed()
        }
    }

    @Test fun reducirAnimacionesSeAplicaATodaLaApp() {
        var reducir: Boolean? = null
        regla.setContent { BotChatTema { reducir = LocalReducirMovimiento.current; Text("a") } }
        assertEquals(false, reducir)
        Ajustes.get(Apoyo.ctx()).fijarAnimaciones(com.botchat.app.ui.ModoAnim.REDUCIDAS)
        regla.waitForIdle()
        assertEquals(true, reducir)
    }
}
