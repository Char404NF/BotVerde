package com.botchat.app.pantallas

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextClearance
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.text.input.TextFieldValue
import com.botchat.app.datos.ActividadItem
import com.botchat.app.datos.Ajustes
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaActividad
import com.botchat.app.ui.PantallaAvanzado
import com.botchat.app.ui.PantallaChat
import com.botchat.app.ui.PantallaMensajesAutomaticos
import com.botchat.app.ui.SelectorAtajos
import com.botchat.app.ui.consultaDeAtajo
import com.botchat.app.ui.esAutomatico
import com.botchat.app.ui.etiquetaAutomatica
import com.botchat.app.ui.frase
import com.botchat.app.ui.BotChatTema as Tema
import org.json.JSONArray
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
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h2400dp-xxhdpi")
class AutomaticosChatUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L
    private val jid = "5215512345678@s.whatsapp.net"
    private val ahora = System.currentTimeMillis() / 1000

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()

    @Test fun losMensajesQueMandoUnaAutomatizacionLlevanLaEtiquetaYLosMiosNo() {
        motor.chat = Apoyo.chatJson(jid, "Ana Pérez").toString()
        motor.mensajes = Apoyo.arreglo(
            Apoyo.msgJson("m3", ahora, "escrito a mano", mio = true),
            Apoyo.msgJson("m2", ahora - 5, "Bienvenida automática", mio = true, automatico = "bienvenida"),
            Apoyo.msgJson("m1", ahora - 10, "hola", remitente = "Ana"),
        )
        regla.setContent { BotChatTema { PantallaChat(jid, {}, {}, {}, {}) } }
        regla.esperaTexto("Bienvenida automática")
        regla.esperaEtiqueta("automatico_m2")
        regla.esperaTexto("Automático · bienvenida", substring = true)
        assertFalse(hay("automatico_m3")) // lo que escribí yo no lleva la etiqueta
        assertFalse(hay("automatico_m1"))
    }

    @Test fun elMenuDelChatOfreceVerSoloLosAutomaticos() {
        motor.chat = Apoyo.chatJson(jid, "Ana Pérez").toString()
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("m1", ahora, "hola", remitente = "Ana"))
        var abierto = false
        regla.setContent { BotChatTema { PantallaChat(jid, {}, {}, {}, {}, alVerAutomaticos = { abierto = true }) } }
        regla.esperaTexto("hola")
        regla.onNodeWithContentDescription("Más opciones").performClick()
        regla.esperaEtiqueta("menu_ver_automaticos")
        regla.onNodeWithTag("menu_ver_automaticos", useUnmergedTree = true).performClick()
        assertTrue(abierto)
    }

    @Test fun laPantallaDeAutomaticosListaSoloLosEtiquetados() {
        motor.mensajesAutoJson = Apoyo.arreglo(
            Apoyo.msgJson("a2", ahora, "Fuera de horario: atendemos de lunes a viernes", mio = true, automatico = "ausencia"),
            Apoyo.msgJson("a1", ahora - 60, "Hola Ana", mio = true, automatico = "menu"),
        )
        regla.setContent { BotChatTema { PantallaMensajesAutomaticos(bot, "", "Mensajes automáticos enviados", {}) } }
        regla.esperaTexto("Fuera de horario: atendemos de lunes a viernes")
        regla.esperaTexto("Automático · ausencia")
        regla.esperaTexto("Automático · menú")
        regla.esperaEtiqueta("lista_automaticos")
    }

    @Test fun sinAutomaticosExplicaDeDondeSalen() {
        regla.setContent { BotChatTema { PantallaMensajesAutomaticos(bot, jid, "Mensajes automáticos de este chat", {}) } }
        regla.esperaEtiqueta("sin_automaticos")
    }

    @Test fun elFiltroDeActividadMuestraSoloLoAutomatico() {
        assertEquals("Automático · bienvenida", etiquetaAutomatica("respondido a ABC (bienvenida)"))
        assertEquals("", etiquetaAutomatica("respondido a ABC"))
        assertTrue(esAutomatico(ActividadItem(1, "respondio", "", "ab12cd", "respondido a X (ausencia)")))
        assertFalse(esAutomatico(ActividadItem(1, "respondio", "", "ab12cd", "respondido a X")))
        assertEquals("Respondió (Automático · menú)", frase(ActividadItem(1, "respondio", "", "ab12cd", "respondido a X (menu)")))
        motor.actividad = """[{"ts":$ahora,"evento":"respondio","motivo":"","chat":"ab12cd","detalle":"respondido a X (recibido)"},{"ts":${ahora - 5},"evento":"respondio","motivo":"","chat":"cd34ef","detalle":"respondido a Y"},{"ts":${ahora - 9},"evento":"llego","motivo":"","chat":"ab12cd","detalle":"privado"}]"""
        regla.setContent { BotChatTema { PantallaActividad(bot, {}) } }
        regla.esperaTexto("Respondió (Automático · recibido)")
        regla.esperaTexto("Llegó un mensaje privado")
        regla.onNodeWithTag("filtro_act_auto").performClick()
        regla.espera { regla.onAllNodesWithTextSafe("Llegó un mensaje privado") == 0 }
        regla.esperaTexto("Respondió (Automático · recibido)")
        assertEquals(0, regla.onAllNodesWithTextSafe("Respondió", substring = false)) // el renglón de mano no sale
        regla.onNodeWithTag("filtro_act_todo").performClick()
        regla.esperaTexto("Llegó un mensaje privado")
    }

    // ---------- selector de atajos en la caja de mensaje ----------

    @Test fun laConsultaDeAtajoSoloValeSiTodoElTextoEsLaBarraYUnaPalabra() {
        assertEquals("", consultaDeAtajo("/"))
        assertEquals("gra", consultaDeAtajo("/Gra"))
        assertEquals(null, consultaDeAtajo("hola /gracias"))
        assertEquals(null, consultaDeAtajo("/gracias por todo"))
        assertEquals(null, consultaDeAtajo(""))
    }

    @Test fun alEscribirBarraSaleLaListaYElegirUnoPoneSuTextoEnLaCajaSinMandarNada() {
        motor.atajosJson = JSONArray().put(JSONObject().put("id", 1).put("nombre", "gracias").put("texto", "¡Gracias por escribir, Ana!").put("archivos", 0))
            .put(JSONObject().put("id", 2).put("nombre", "catalogo").put("texto", "Te paso el catálogo").put("archivos", 2)).toString()
        var caja by mutableStateOf(TextFieldValue("/"))
        regla.setContent { BotChatTema { SelectorAtajos(bot, jid, caja) { caja = it } } }
        regla.esperaEtiqueta("selector_atajos")
        regla.esperaEtiqueta("atajo_gracias"); regla.esperaEtiqueta("atajo_catalogo")
        regla.onNodeWithTag("atajo_gracias", useUnmergedTree = true).performClick()
        assertEquals("¡Gracias por escribir, Ana!", caja.text)
        assertFalse("el selector solo pone el texto: lo manda la persona", llamo("enviarTexto") || llamo("enviar:"))
        // ya no es una consulta de atajo: el selector se cierra
        regla.espera { !hay("selector_atajos") }
    }

    @Test fun elSelectorFiltraPorLoEscritoYOfreceMandarLosArchivos() {
        motor.atajosJson = JSONArray().put(JSONObject().put("id", 1).put("nombre", "gracias").put("texto", "Gracias").put("archivos", 0))
            .put(JSONObject().put("id", 2).put("nombre", "catalogo").put("texto", "Te paso el catálogo").put("archivos", 2)).toString()
        var caja by mutableStateOf(TextFieldValue("/cat"))
        regla.setContent { BotChatTema { SelectorAtajos(bot, jid, caja) { caja = it } } }
        regla.esperaEtiqueta("atajo_catalogo")
        assertFalse(hay("atajo_gracias"))
        regla.onNodeWithTag("atajo_archivos_catalogo", useUnmergedTree = true).performClick()
        regla.espera { llamo("archivosAtajo:$jid:2") }
        caja = TextFieldValue("/zzz")
        regla.esperaEtiqueta("atajos_sin_coincidencia")
        caja = TextFieldValue("hola")
        regla.espera { !hay("selector_atajos") }
    }

    @Test fun sinAtajosGuardadosElSelectorNoMuestraNada() {
        motor.atajosJson = "[]"
        var caja by mutableStateOf(TextFieldValue("/"))
        regla.setContent { BotChatTema { SelectorAtajos(bot, jid, caja) { caja = it } } }
        regla.waitForIdle(); Thread.sleep(200); regla.waitForIdle()
        assertFalse(hay("selector_atajos"))
    }

    @Test fun elInterruptorDeEstiloDeLetraEstaEnAvanzadoYApagadoPorDefecto() {
        val a = Ajustes.get(Apoyo.ctx())
        assertFalse(a.estiloLetra.value)
        regla.setContent { BotChatTema { PantallaAvanzado(a, "Mi bot", false, {}, {}, {}, {}) } }
        regla.esperaEtiqueta("sw_estilo_letra")
        regla.onNodeWithTag("sw_estilo_letra", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { a.estiloLetra.value }
        regla.esperaTexto("Puede verse distinto en otros teléfonos", substring = true)
    }
}
