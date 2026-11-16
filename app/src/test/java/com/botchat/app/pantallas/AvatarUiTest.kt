package com.botchat.app.pantallas

import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import com.botchat.app.datos.JsonAvatar
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaAvatar
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
class AvatarUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun toca(tag: String) {
        regla.esperaEtiqueta(tag)
        runCatching { regla.onNodeWithTag(tag, useUnmergedTree = true).performScrollTo() }
        regla.onNodeWithTag(tag, useUnmergedTree = true).performClick()
    }

    @Test fun elEstadoSeLeeDelMotor() {
        val e = JsonAvatar.estado(motor.avatarJson)
        assertTrue(e.activo); assertFalse(e.propio)
        assertEquals(listOf("menu", "ayuda", "aviso"), e.momentos.map { it.id })
        assertTrue(e.momentos[0].grupos); assertFalse(e.momentos[1].grupos)
    }

    @Test fun seMuestranLosMomentosYSeCambianConUnToque() {
        regla.setContent { BotChatTema { PantallaAvatar(bot, {}) } }
        regla.esperaEtiqueta("pantalla_avatar")
        regla.esperaTexto("Avisos y advertencias en privado")
        regla.esperaTexto("Solo en chats privados", substring = true)
        toca("avatar_momento_aviso") // el de avisos estaba apagado: se enciende y se guardan los tres
        regla.espera { llamo("avatar:true:menu,ayuda,aviso") }
        toca("avatar_momento_ayuda") // y ahora se apaga la ayuda
        regla.espera { llamo("avatar:true:menu,aviso") }
    }

    @Test fun apagarElAvatarGuardaYBloqueaLosMomentos() {
        regla.setContent { BotChatTema { PantallaAvatar(bot, {}) } }
        toca("avatar_activo")
        regla.espera { llamo("avatar:false:") }
        regla.esperaTexto("Apagado: todo sale solo con texto.")
    }

    @Test fun sinImagenPropiaNoHayBotonDeQuitar() {
        regla.setContent { BotChatTema { PantallaAvatar(bot, {}) } }
        regla.esperaEtiqueta("avatar_elegir")
        assertFalse(hay("avatar_quitar"))
    }
}
