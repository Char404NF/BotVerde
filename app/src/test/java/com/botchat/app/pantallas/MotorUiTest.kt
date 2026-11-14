package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import com.botchat.app.datos.Ajustes
import com.botchat.app.datos.ActividadItem
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaActividad
import com.botchat.app.ui.PantallaChat
import com.botchat.app.ui.PantallaFunciones
import com.botchat.app.ui.frase
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
class MotorUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L
    private val ahora = System.currentTimeMillis() / 1000

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()

    @Test fun lasFrasesDelRegistroNoLlevanTextosNiNombres() {
        assertEquals("No respondió: mensaje antiguo o de sincronización", frase(ActividadItem(1, "omitido", "antiguo", "ab12cd", "")))
        assertEquals("Respondió", frase(ActividadItem(1, "respondio", "", "ab12cd", "respondido a XYZ")))
        assertEquals("Enviaste un mensaje a mano (no es el bot)", frase(ActividadItem(1, "manual", "", "ab12cd", "texto")))
        assertEquals("No respondió: en el grupo no lo mencionaron", frase(ActividadItem(1, "omitido", "grupo sin mención", "", "")))
    }

    @Test fun laPantallaDeActividadListaLoQuePasoConCadaMensaje() {
        motor.actividad = """[{"ts":$ahora,"evento":"respondio","motivo":"","chat":"ab12cd","detalle":""},{"ts":${ahora - 5},"evento":"omitido","motivo":"duplicado","chat":"ab12cd","detalle":""},{"ts":${ahora - 9},"evento":"llego","motivo":"","chat":"ab12cd","detalle":"privado"}]"""
        regla.setContent { BotChatTema { PantallaActividad(bot, {}) } }
        regla.esperaTexto("Respondió")
        regla.onNodeWithText("No respondió: ya se había visto ese mensaje").assertIsDisplayed()
        regla.onNodeWithText("Llegó un mensaje privado").assertIsDisplayed()
    }

    @Test fun sinActividadExplicaQueEsperar() {
        regla.setContent { BotChatTema { PantallaActividad(bot, {}) } }
        regla.esperaTexto("Sin actividad")
    }

    @Test fun elDeslizadorDeAntiguedadAvisaAlMotor() {
        val a = Ajustes.get(Apoyo.ctx())
        regla.setContent { BotChatTema { PantallaFunciones(a, {}) } }
        regla.esperaTexto("No responder mensajes de hace más de")
        assertEquals(10, a.antiguedadMin.value)
        a.fijarAntiguedadMin(25)
        a.fijarMotorAutomatico(false)
        assertEquals(25, a.antiguedadMin.value)
        assertFalse(a.motorAutomatico.value)
    }

    @Test fun enUnGrupoElMenuTieneElInterruptorDelBot() {
        val g = "120@g.us"
        motor.chat = Apoyo.chatJson(g, "Familia", grupo = true).toString()
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola", remitente = "Ana"))
        regla.setContent { BotChatTema { PantallaChat(g, {}, {}, {}, {}) } }
        regla.esperaTexto("hola")
        regla.onNodeWithContentDescription("Más opciones").performClick()
        regla.esperaEtiqueta("menu_mas"); regla.onNodeWithTag("menu_mas").performClick()
        regla.esperaEtiqueta("menu_bot_grupo")
        regla.onNodeWithTag("menu_bot_grupo").performClick()
        regla.espera { llamo("botChat:false") }
    }

    @Test fun enUnPrivadoNoHayInterruptorDeGrupo() {
        motor.chat = Apoyo.chatJson("5@s.whatsapp.net", "Ana").toString()
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola", remitente = "Ana"))
        regla.setContent { BotChatTema { PantallaChat("5@s.whatsapp.net", {}, {}, {}, {}) } }
        regla.esperaTexto("hola")
        regla.onNodeWithContentDescription("Más opciones").performClick()
        regla.esperaEtiqueta("menu_silenciar")
        assertFalse(hay("menu_bot_grupo"))
    }

    @Test fun siElBotEstaEnPausaEnElChatSeAvisaYSePuedeReactivar() {
        motor.chat = Apoyo.chatJson("5@s.whatsapp.net", "Ana").toString()
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola", remitente = "Ana"))
        motor.estadoChat = """{"silenciado":false,"botPausaHasta":${ahora + 1800},"botActivo":true,"pausaHumano":false}"""
        regla.setContent { BotChatTema { PantallaChat("5@s.whatsapp.net", {}, {}, {}, {}) } }
        regla.esperaEtiqueta("aviso_bot_pausado")
        regla.esperaTexto("El bot está en pausa en este chat hasta las", substring = true)
        regla.onNodeWithTag("reactivar_bot").performClick()
        regla.espera { llamo("pausarBot:0") }
        regla.espera { !hay("aviso_bot_pausado") }
    }

    @Test fun tomarElControlPausaElBotEnElChatYSePuedeReanudar() {
        motor.chat = Apoyo.chatJson("5@s.whatsapp.net", "Ana").toString()
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola", remitente = "Ana"))
        regla.setContent { BotChatTema { PantallaChat("5@s.whatsapp.net", {}, {}, {}, {}) } }
        regla.esperaTexto("hola")
        assertFalse(hay("aviso_bot_pausado"))
        regla.onNodeWithContentDescription("Más opciones").performClick()
        regla.esperaEtiqueta("menu_pausar_bot")
        regla.onNodeWithTag("menu_pausar_bot").performClick()
        regla.esperaEtiqueta("pausa_control")
        regla.onNodeWithTag("pausa_control").performClick()
        regla.espera { llamo("pausarBot:${com.botchat.app.ui.MINUTOS_CONTROL}") }
        regla.esperaEtiqueta("aviso_control")
        regla.esperaTexto("Tienes el control de este chat: el bot no responde aquí")
        // el menú ahora ofrece reanudar
        regla.onNodeWithContentDescription("Más opciones").performClick()
        regla.esperaEtiqueta("menu_tomar_control")
        regla.onNodeWithText("Reanudar el bot en este chat").assertIsDisplayed()
        regla.onNodeWithTag("menu_tomar_control").performClick()
        regla.espera { llamo("pausarBot:0") }
        regla.espera { !hay("aviso_bot_pausado") }
    }

    @Test fun conEsperaDeUnaPersonaElAvisoLoDice() {
        motor.chat = Apoyo.chatJson("5@s.whatsapp.net", "Ana").toString()
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola", remitente = "Ana"))
        motor.estadoChat = """{"silenciado":false,"botPausaHasta":${ahora + 40000},"botActivo":true,"pausaHumano":true}"""
        regla.setContent { BotChatTema { PantallaChat("5@s.whatsapp.net", {}, {}, {}, {}) } }
        regla.esperaTexto("Esperando a una persona", substring = true)
    }
}
