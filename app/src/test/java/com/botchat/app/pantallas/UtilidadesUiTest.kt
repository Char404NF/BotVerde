package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextInput
import com.botchat.app.datos.Ajustes
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaAjustes
import com.botchat.app.ui.PantallaUtilidades
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
class UtilidadesUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun item(id: Long, tipo: String, texto: String, hecho: Boolean = false) = JSONObject().put("id", id).put("tipo", tipo).put("texto", texto).put("hecho", hecho).put("ts", 1)
    private fun pantalla() = regla.setContent { BotChatTema { PantallaUtilidades(bot, {}) } }

    @Test fun sinNadaDiceQueNoHayPendientesNiNotas() {
        pantalla()
        regla.esperaEtiqueta("sin_pendientes")
        regla.esperaEtiqueta("sin_notas")
        assertFalse(hay("limpiar_hechos"))
        regla.onNodeWithTag("agregar_pendiente", useUnmergedTree = true).performScrollTo().assertIsNotEnabled()
        regla.onNodeWithTag("agregar_nota", useUnmergedTree = true).performScrollTo().assertIsNotEnabled()
    }

    @Test fun agregaPendientesLosMarcaYLosBorra() {
        pantalla()
        regla.esperaEtiqueta("sin_pendientes")
        regla.onNodeWithTag("pendiente_nuevo", useUnmergedTree = true).performScrollTo().performTextInput("  pagar la luz ")
        regla.onNodeWithTag("agregar_pendiente", useUnmergedTree = true).assertIsEnabled().performClick()
        regla.espera { llamo("agregarUtil:pendiente:pagar la luz") }
        regla.esperaEtiqueta("pendiente_1")
        regla.esperaTexto("1. pagar la luz")
        // marcar como hecho y deshacer
        regla.onNodeWithTag("pendiente_hecho_1", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("marcarUtil:1:true") }
        regla.esperaEtiqueta("limpiar_hechos")
        regla.onNodeWithTag("pendiente_hecho_1", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("marcarUtil:1:false") }
        assertEquals(false, motor.utilLista.first().getBoolean("hecho"))
        // borrar
        regla.onNodeWithTag("borrar_pendiente_1", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("borrarUtil:1") }
        regla.esperaEtiqueta("sin_pendientes")
    }

    @Test fun quitaLosPendientesHechosDeUnaVez() {
        motor.utilLista += item(1, "pendiente", "uno", true)
        motor.utilLista += item(2, "pendiente", "dos")
        motor.utilLista += item(3, "pendiente", "tres", true)
        pantalla()
        regla.esperaEtiqueta("pendiente_3")
        regla.onNodeWithTag("limpiar_hechos", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("limpiarUtilHechas") }
        regla.espera { !hay("pendiente_1") && !hay("pendiente_3") && hay("pendiente_2") }
        assertFalse(hay("limpiar_hechos"))
    }

    @Test fun agregaYBorraNotasNumeradas() {
        motor.utilLista += item(1, "nota", "llamar a mamá")
        pantalla()
        regla.esperaTexto("1. llamar a mamá")
        regla.onNodeWithTag("nota_nueva", useUnmergedTree = true).performScrollTo().performTextInput("comprar café")
        regla.onNodeWithTag("agregar_nota", useUnmergedTree = true).performClick()
        regla.espera { llamo("agregarUtil:nota:comprar café") }
        regla.esperaTexto("2. comprar café")
        regla.onNodeWithTag("borrar_nota_1", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("borrarUtil:1") }
        regla.esperaTexto("1. comprar café")
    }

    @Test fun siFallaMuestraElMensajeYNoSeCae() {
        motor.personaFalla = "Ya hay demasiadas notas"
        pantalla()
        regla.esperaEtiqueta("sin_notas")
        regla.onNodeWithTag("nota_nueva", useUnmergedTree = true).performScrollTo().performTextInput("algo")
        regla.onNodeWithTag("agregar_nota", useUnmergedTree = true).performClick()
        regla.espera { llamo("agregarUtil:nota:algo") }
        regla.esperaEtiqueta("sin_notas")
    }

    @Test fun explicaLosComandosYElSticker() {
        pantalla()
        regla.esperaEtiqueta("ayuda_comandos_util")
        regla.esperaTexto("!anotar texto", substring = true)
        regla.esperaTexto("!tarea texto", substring = true)
        regla.onNodeWithTag("ayuda_sticker", useUnmergedTree = true).performScrollTo()
        regla.esperaTexto("!sticker", substring = true)
        regla.esperaTexto("ver una vez", substring = true)
    }

    @Test fun elAtajoEstaEnAjustes() {
        var abierto = false
        regla.setContent { BotChatTema { PantallaAjustes(Ajustes.get(Apoyo.ctx()), {}, {}, {}, {}, alUtilidades = { abierto = true }) } }
        regla.onNodeWithTag("ajuste_utilidades", useUnmergedTree = true).performScrollTo().performClick()
        assertTrue(abierto)
    }
}
