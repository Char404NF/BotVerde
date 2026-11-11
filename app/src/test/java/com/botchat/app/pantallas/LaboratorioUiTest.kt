package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import com.botchat.app.datos.Ajustes
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaAvanzado
import com.botchat.app.ui.PantallaAvisoLab
import com.botchat.app.ui.PantallaLaboratorio
import com.botchat.app.ui.PantallaMatrizLab
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
class LaboratorioUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun principal(alAviso: () -> Unit = {}, alMatriz: () -> Unit = {}) = regla.setContent { BotChatTema { PantallaLaboratorio(bot, {}, alAviso, alMatriz) } }
    private fun encendido() { motor.labAj["lab_activo"] = "1"; motor.labAj["lab_aviso_ts"] = "1700000000" }
    private fun fila(id: Long, tipo: String, variante: Int, enviado: Int = 1, llego: Int = -1, visible: Int = -1, tocable: Int = -1, respondio: Int = -1, error: String = "") =
        JSONObject().put("id", id).put("ts", 1700000000L + id).put("tipo", tipo).put("variante", variante.toString()).put("chat", "x").put("msgId", "M$id").put("enviado", enviado).put("llego", llego)
            .put("visible", visible).put("tocable", tocable).put("respondio", respondio).put("versionWa", "2.26.5.1").put("versionCliente", "2.3000.1").put("error", error)

    // ---------- pantalla principal ----------

    @Test fun nacesApagadoYSinOpcionesSoloElAvisoDeRiesgo() {
        principal()
        regla.esperaEtiqueta("lab_activo")
        assertTrue(hay("lab_apagado_aviso"))
        assertFalse(hay("lab_modo"))
        assertFalse(hay("tarjeta_pruebas"))
        regla.onNodeWithTag("lab_estado_texto", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithText("Apagado. Todos los menús salen en texto con números.", useUnmergedTree = true).assertIsDisplayed()
    }

    @Test fun encenderSinAceptarElAvisoLlevaAlAvisoYNoEnciendeNada() {
        var aviso = 0
        principal(alAviso = { aviso++ })
        regla.esperaEtiqueta("lab_activo")
        regla.onNodeWithTag("lab_activo", useUnmergedTree = true).performClick()
        regla.espera { aviso == 1 }
        assertFalse(llamo("ajusteLab:lab_activo"))
        assertEquals("0", motor.labAj["lab_activo"])
    }

    @Test fun conElAvisoAceptadoSeEnciendeYSeApagaAlInstante() {
        motor.labAj["lab_aviso_ts"] = "1700000000"
        principal()
        regla.esperaEtiqueta("lab_activo")
        regla.onNodeWithTag("lab_activo", useUnmergedTree = true).performClick()
        regla.espera { llamo("ajusteLab:lab_activo:1") }
        regla.esperaEtiqueta("lab_modo")
        regla.onNodeWithTag("lab_activo", useUnmergedTree = true).performClick()
        regla.espera { llamo("ajusteLab:lab_activo:0") }
        regla.espera { !hay("lab_modo") }
    }

    @Test fun elModoDelMenuTrae_Texto_Encuesta_Botones_YPorDefectoEsTexto() {
        encendido()
        principal()
        regla.esperaEtiqueta("lab_modo")
        regla.onNodeWithText("Predeterminado. El menú sale como texto con números: la persona escribe 1, 2, 3…", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("lab_modo_encuesta", useUnmergedTree = true).performClick()
        regla.espera { llamo("ajusteLab:lab_modo_menu:encuesta") }
        regla.espera { hay("lab_modo_texto") && motor.labAj["lab_modo_menu"] == "encuesta" }
        regla.esperaTexto("El menú sale como una encuesta de WhatsApp", substring = true)
        regla.onNodeWithTag("lab_modo_botones", useUnmergedTree = true).performClick()
        regla.espera { llamo("ajusteLab:lab_modo_menu:botones") }
        regla.esperaTexto("Experimental. El menú sale con botones o lista", substring = true)
        regla.onNodeWithText("Experimental. El menú sale con botones o lista", substring = true, useUnmergedTree = true).assertIsDisplayed()
    }

    @Test fun enGruposAvisaQueTodosVenLosVotos() {
        encendido()
        principal()
        regla.esperaEtiqueta("lab_grupos")
        assertFalse(hay("lab_grupos_aviso"))
        regla.onNodeWithTag("lab_grupos", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("ajusteLab:lab_grupos:1") }
        regla.esperaEtiqueta("lab_grupos_aviso")
    }

    @Test fun losNumerosSeValidanYSoloSeGuardanSiCambian() {
        encendido()
        principal()
        regla.esperaEtiqueta("lab_enc_max")
        assertFalse(hay("lab_enc_max_guardar"))
        regla.onNodeWithTag("lab_enc_max", useUnmergedTree = true).performScrollTo().performTextReplacement("40")
        regla.waitForIdle()
        assertFalse("40 pasa del máximo (32)", hay("lab_enc_max_guardar"))
        regla.onNodeWithTag("lab_enc_max", useUnmergedTree = true).performTextReplacement("10")
        regla.esperaEtiqueta("lab_enc_max_guardar")
        regla.onNodeWithTag("lab_enc_max_guardar", useUnmergedTree = true).performClick()
        regla.espera { llamo("ajusteLab:lab_enc_max_opc:10") }
    }

    @Test fun elTipoDeBotonesYLasVersionesSeGuardan() {
        encendido()
        principal()
        regla.esperaEtiqueta("lab_botones_tipo")
        regla.onNodeWithTag("lab_tipo_interactivo", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("ajusteLab:lab_botones_tipo:interactivo") }
        regla.onNodeWithTag("lab_version_wa", useUnmergedTree = true).performScrollTo().performTextInput("2.26.5.1")
        regla.onNodeWithTag("lab_version_wa_guardar", useUnmergedTree = true).performClick()
        regla.espera { llamo("ajusteLab:lab_version_wa:2.26.5.1") }
    }

    @Test fun elChatDePruebaSeEligeDeLaLista() {
        encendido()
        motor.chatsPersonales = """[${Apoyo.chatJson("5215599999999@s.whatsapp.net", "Mi otro número")}]"""
        motor.chatsGrupos = """[${Apoyo.chatJson("120363000@g.us", "Grupo de prueba", grupo = true)}]"""
        principal()
        regla.esperaEtiqueta("lab_elegir_chat")
        regla.onNodeWithTag("lab_elegir_chat", useUnmergedTree = true).performScrollTo().performClick()
        regla.esperaEtiqueta("lab_chat_5215599999999@s.whatsapp.net")
        assertTrue(hay("lab_chat_120363000@g.us"))
        regla.onNodeWithTag("lab_chat_5215599999999@s.whatsapp.net", useUnmergedTree = true).performClick()
        regla.espera { llamo("ajusteLab:lab_prueba_chat:5215599999999@s.whatsapp.net") }
        regla.espera { hay("lab_chat_nombre") && motor.labAj["lab_prueba_chat"].orEmpty().isNotBlank() }
        regla.onNodeWithText("Mi otro número", useUnmergedTree = true).assertIsDisplayed()
    }

    // ---------- aviso de riesgo ----------

    @Test fun elAvisoPideEsperaDe5SegundosYCasillaYSoloEntoncesEnciende() {
        regla.mainClock.autoAdvance = false
        regla.setContent { BotChatTema { PantallaAvisoLab(bot, {}) } }
        var vueltas = 0
        while (!hay("aviso_lab_entiendo") || !hay("aviso_lab_espera")) { regla.mainClock.advanceTimeByFrame(); Thread.sleep(20); if (++vueltas > 400) error("no cargó el aviso") }
        regla.onNodeWithTag("aviso_lab_texto", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("aviso_lab_entiendo").assertIsNotEnabled()
        regla.onNodeWithTag("aviso_lab_casilla", useUnmergedTree = true).performClick()
        regla.mainClock.advanceTimeBy(2_000)
        regla.onNodeWithTag("aviso_lab_entiendo").assertIsNotEnabled()
        regla.mainClock.advanceTimeBy(3_500)
        regla.waitForIdle()
        assertFalse(hay("aviso_lab_espera"))
        regla.onNodeWithTag("aviso_lab_entiendo").assertIsEnabled()
        regla.mainClock.autoAdvance = true
        assertFalse(llamo("aceptarAvisoLab"))
        regla.onNodeWithTag("aviso_lab_entiendo").performClick()
        regla.espera { llamo("ajusteLab:lab_activo:1") }
        assertTrue(llamo("aceptarAvisoLab"))
        assertEquals("1", motor.labAj["lab_activo"])
    }

    @Test fun cancelarElAvisoNoEnciendeNada() {
        var volvio = false
        regla.setContent { BotChatTema { PantallaAvisoLab(bot, { volvio = true }) } }
        regla.esperaEtiqueta("aviso_lab_cancelar")
        regla.onNodeWithTag("aviso_lab_cancelar").performClick()
        regla.espera { volvio }
        assertFalse(llamo("aceptarAvisoLab") || llamo("ajusteLab"))
    }

    // ---------- matriz ----------

    private fun matriz() = regla.setContent { BotChatTema { PantallaMatrizLab(bot, {}) } }

    @Test fun sinLaboratorioNiChatDePruebaLaMatrizLoDiceYNoDejaMandar() {
        matriz()
        regla.esperaEtiqueta("matriz_no_listo")
        regla.onNodeWithTag("prueba_encuesta_4", useUnmergedTree = true).assertIsNotEnabled()
        regla.onNodeWithText("Enciende el Laboratorio primero", substring = true, useUnmergedTree = true).assertIsDisplayed()
        assertTrue(hay("matriz_vacia"))
    }

    @Test fun mandaUnaPruebaYApareceEnLaMatrizConSusVersiones() {
        encendido()
        motor.labAj["lab_prueba_chat"] = "5215599999999@s.whatsapp.net"
        motor.labAj["lab_version_wa"] = "2.26.5.1"
        matriz()
        regla.esperaEtiqueta("prueba_encuesta_13")
        assertFalse(hay("matriz_no_listo"))
        regla.onNodeWithTag("prueba_encuesta_13", useUnmergedTree = true).assertIsEnabled().performClick()
        regla.espera { llamo("pruebaLab:encuesta:13") }
        regla.esperaEtiqueta("fila_lab_1")
        regla.onNodeWithText("Encuesta 13", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("version_lab_1", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithText("WhatsApp 2.26.5.1 · conexión 2.3000.1", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("matriz_topes", useUnmergedTree = true).assertIsDisplayed()
    }

    @Test fun lasCeldasSeVenYTocarlasCambiaSeVeYSeToca() {
        encendido()
        motor.labAj["lab_prueba_chat"] = "x"
        motor.labResultados.add(fila(1, "encuesta", 12, llego = 1, respondio = 1))
        matriz()
        regla.esperaEtiqueta("fila_lab_1")
        assertEquals("Sí", textoDe("celda_llego_1"))
        assertEquals("Sí", textoDe("celda_respondio_1"))
        assertEquals("?", textoDe("celda_visible_1"))
        // ? → Sí → No → ?
        regla.onNodeWithTag("celda_visible_1", useUnmergedTree = true).performClick()
        regla.espera { llamo("marcarLab:1:visible:1") }
        regla.espera { textoDe("celda_visible_1") == "Sí" }
        regla.onNodeWithTag("celda_visible_1", useUnmergedTree = true).performClick()
        regla.espera { llamo("marcarLab:1:visible:0") }
        regla.espera { textoDe("celda_visible_1") == "No" }
        regla.onNodeWithTag("celda_visible_1", useUnmergedTree = true).performClick()
        regla.espera { llamo("marcarLab:1:visible:-1") }
        regla.onNodeWithTag("celda_tocable_1", useUnmergedTree = true).performClick()
        regla.espera { llamo("marcarLab:1:tocable:1") }
        // «Llegó» y «Respondió» las anota el bot: tocarlas no hace nada
        regla.onNodeWithTag("celda_llego_1", useUnmergedTree = true).performClick()
        regla.onNodeWithTag("celda_respondio_1", useUnmergedTree = true).performClick()
        regla.waitForIdle()
        assertFalse(llamo("marcarLab:1:llego") || llamo("marcarLab:1:respondio"))
    }

    private fun textoDe(tag: String): String {
        val n = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().firstOrNull() ?: return ""
        val t = n.children.firstOrNull()?.config?.getOrNull(SemanticsProperties.Text) ?: n.config.getOrNull(SemanticsProperties.Text)
        return t?.joinToString("") { it.text }.orEmpty()
    }

    @Test fun unEnvioFallidoMuestraElErrorEnLaFila() {
        encendido()
        motor.labAj["lab_prueba_chat"] = "x"
        motor.labResultados.add(fila(1, "botones", 3, enviado = 0, error = "no soportado"))
        matriz()
        regla.esperaEtiqueta("fila_lab_1")
        assertEquals("Error", textoDe("celda_llego_1"))
        regla.onNodeWithText("no soportado", useUnmergedTree = true).assertIsDisplayed()
    }

    @Test fun elLimiteMedidoSeMuestraYSePuedeUsarComoTope() {
        encendido()
        motor.labAj["lab_prueba_chat"] = "x"
        motor.labResultados.add(fila(3, "encuesta", 13, visible = 0))
        motor.labResultados.add(fila(2, "encuesta", 12, visible = 1, tocable = 1))
        motor.labResultados.add(fila(1, "encuesta", 8, visible = 1))
        matriz()
        regla.esperaEtiqueta("tarjeta_limite")
        regla.onNodeWithText("tiene 12 opciones. Ahora el tope es 12.", substring = true, useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("usar_limite", useUnmergedTree = true).performScrollTo().assertIsNotEnabled() // ya es 12
    }

    @Test fun usarElLimiteCambiaElTopeDeOpciones() {
        encendido()
        motor.labAj["lab_prueba_chat"] = "x"
        motor.labAj["lab_enc_max_opc"] = "6"
        motor.labResultados.add(fila(2, "encuesta", 12, visible = 1, tocable = 1))
        matriz()
        regla.esperaEtiqueta("usar_limite")
        regla.onNodeWithTag("usar_limite", useUnmergedTree = true).performScrollTo().assertIsEnabled().performClick()
        regla.espera { llamo("usarLimiteLab") }
        assertEquals("12", motor.labAj["lab_enc_max_opc"])
    }

    @Test fun sinPruebasMarcadasNoHayLimiteTodavia() {
        encendido()
        motor.labAj["lab_prueba_chat"] = "x"
        motor.labResultados.add(fila(1, "encuesta", 12))
        matriz()
        regla.esperaEtiqueta("limite_texto")
        regla.onNodeWithText("Todavía no hay datos.", substring = true, useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("usar_limite", useUnmergedTree = true).performScrollTo().assertIsNotEnabled()
    }

    @Test fun borrarLaMatrizPideConfirmacion() {
        encendido()
        motor.labAj["lab_prueba_chat"] = "x"
        motor.labResultados.add(fila(1, "lista", 10))
        matriz()
        regla.esperaEtiqueta("borrar_matriz")
        regla.onNodeWithTag("borrar_matriz", useUnmergedTree = true).performScrollTo().performClick()
        regla.esperaEtiqueta("borrar_matriz_si")
        assertFalse(llamo("borrarLab"))
        regla.onNodeWithTag("borrar_matriz_si", useUnmergedTree = true).performClick()
        regla.espera { llamo("borrarLab") }
        regla.esperaEtiqueta("matriz_vacia")
    }

    @Test fun sinElBotEnMarchaLasPruebasNoSePuedenMandar() {
        encendido()
        motor.labAj["lab_prueba_chat"] = "x"
        motor.labEnMarcha = false
        matriz()
        regla.esperaEtiqueta("matriz_no_listo")
        regla.onNodeWithText("Enciende el bot para mandar pruebas.", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("prueba_botones_3", useUnmergedTree = true).assertIsNotEnabled()
    }

    // ---------- Avanzado ----------

    @Test fun avanzadoTieneLaEntradaAlLaboratorio() {
        var abierto = false
        regla.setContent { BotChatTema { PantallaAvanzado(Ajustes.get(Apoyo.ctx()), "Mi bot", false, {}, {}, {}, {}, alLaboratorio = { abierto = true }) } }
        regla.esperaEtiqueta("abrir_laboratorio")
        regla.onNodeWithTag("abrir_laboratorio", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { abierto }
    }
}
