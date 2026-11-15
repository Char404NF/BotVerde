package com.botchat.app.pantallas

import android.app.AlarmManager
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextClearance
import androidx.compose.ui.test.performTextInput
import com.botchat.app.AlarmasModulos
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaConfigModulo
import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class PanelesModuloUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L

    @Before fun antes() {
        motor = Apoyo.reinicia(); bot = Apoyo.creaBot()
        motor.modulosLista.add(Apoyo.moduloJson("difusion_h", "Difusión y anuncios", "Anuncios a una lista.", "difusion", "alto", iniciativa = true, esquema = Apoyo.esquemaEjemplo(),
            paneles = JSONArray().put(Apoyo.panelEjemplo("listas"))))
        motor.modulosLista.add(Apoyo.moduloJson("solo_paneles", "Solo paneles", "Sin ajustes, con una lista.", "grupo", "medio", iniciativa = false, paneles = JSONArray().put(Apoyo.panelEjemplo("listas"))))
        motor.filasPaneles["difusion_h:listas"] = JSONArray().put(JSONObject().put("id", "suscritos").put("titulo", "suscritos").put("detalle", "3 personas")).toString()
    }

    private fun abre(id: String) = regla.setContent { BotChatTema { PantallaConfigModulo(bot, id, alVolver = {}) } }
    private fun llamada(p: String) = motor.llamadas.toList().firstOrNull { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()

    @Test fun elPanelSeDibujaDebajoDeLosAjustesConSusFilas() {
        abre("difusion_h")
        regla.esperaEtiqueta("paneles_modulo")
        assertTrue(hay("config_generica"))
        regla.onNodeWithTag("panel_listas", useUnmergedTree = true).performScrollTo().assertIsDisplayed()
        regla.esperaEtiqueta("fila_listas_suscritos")
        regla.onNodeWithTag("fila_listas_suscritos", useUnmergedTree = true).performScrollTo().assertIsDisplayed()
        assertTrue(hay("boton_listas.vaciar_suscritos"))
    }

    @Test fun unModuloSoloConPanelesNoDiceQueNoTieneAjustes() {
        abre("solo_paneles")
        regla.esperaEtiqueta("paneles_modulo")
        assertFalse(hay("config_sin_ajustes"))
        assertTrue(hay("panel_vacio_listas"))
    }

    @Test fun unModuloSinPanelesNoTrae() {
        abre("eco_prueba")
        regla.esperaEtiqueta("config_generica")
        assertFalse(hay("paneles_modulo"))
    }

    @Test fun elFormularioManda_losValores_yMuestraLaRespuesta() {
        motor.respuestaAccionModulo = "Agregada a «suscritos»."
        abre("difusion_h")
        regla.esperaEtiqueta("alta_listas")
        regla.onNodeWithTag("pcin_listas_lista", useUnmergedTree = true).performScrollTo().performTextClearance()
        regla.onNodeWithTag("pcin_listas_lista", useUnmergedTree = true).performTextInput("vip")
        // más de 5 opciones: lista desplegable
        assertFalse(hay("pcop_listas_persona_p1"))
        regla.onNodeWithTag("pcsel_listas_persona", useUnmergedTree = true).performScrollTo().performClick()
        regla.esperaEtiqueta("pcitem_listas_persona_p3")
        regla.onNodeWithTag("pcitem_listas_persona_p3", useUnmergedTree = true).performClick()
        regla.onNodeWithTag("alta_listas", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamada("accionModulo:difusion_h:listas.agregar") != null }
        val args = JSONObject(llamada("accionModulo:difusion_h:listas.agregar")!!.substringAfter("listas.agregar:"))
        assertEquals("vip", args.getString("lista"))
        assertEquals("p3", args.getString("persona"))
        assertEquals(0, args.getJSONArray("adjuntos").length())
        regla.esperaEtiqueta("panel_ok")
        regla.esperaTexto("Agregada a «suscritos».")
        // vuelve a pedir las filas y el formulario recupera su valor inicial
        regla.espera { motor.llamadas.count { it == "filasPanel:difusion_h:listas" } >= 2 }
    }

    @Test fun losBotonesConConfirmacionPreguntanYSoloEntoncesHacen() {
        abre("difusion_h")
        regla.esperaEtiqueta("fila_listas_suscritos")
        regla.onNodeWithTag("boton_listas.vaciar_suscritos", useUnmergedTree = true).performScrollTo().performClick()
        regla.esperaEtiqueta("panel_confirma_texto")
        assertNull(llamada("accionModulo"))
        regla.onNodeWithTag("panel_confirma_no", useUnmergedTree = true).performClick()
        regla.espera { !hay("panel_confirma_texto") }
        assertNull(llamada("accionModulo"))
        regla.onNodeWithTag("boton_listas.vaciar_suscritos", useUnmergedTree = true).performScrollTo().performClick()
        regla.esperaEtiqueta("panel_confirma_si")
        regla.onNodeWithTag("panel_confirma_si", useUnmergedTree = true).performClick()
        regla.espera { llamada("accionModulo:difusion_h:listas.vaciar") != null }
        assertEquals("""{"id":"suscritos"}""", llamada("accionModulo:difusion_h:listas.vaciar")!!.substringAfter("listas.vaciar:"))
    }

    @Test fun losBotonesExtraUsanElFormularioYLosSimplesNoPreguntan() {
        abre("difusion_h")
        regla.esperaEtiqueta("extra_listas.vista")
        regla.onNodeWithTag("extra_listas.vista", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamada("accionModulo:difusion_h:listas.vista") != null }
        assertFalse(hay("panel_confirma_texto"))
        assertTrue(llamada("accionModulo:difusion_h:listas.vista")!!.contains("\"lista\":\"suscritos\""))
    }

    @Test fun siGoRechazaSeMuestraElMotivoTalCual() {
        abre("difusion_h")
        regla.esperaEtiqueta("alta_listas")
        motor.moduloFalla = "esa persona nunca te ha escrito"
        regla.onNodeWithTag("alta_listas", useUnmergedTree = true).performScrollTo().performClick()
        regla.esperaEtiqueta("panel_error")
        regla.esperaTexto("esa persona nunca te ha escrito")
    }

    // ---------- alarma de lo agendado ----------

    @Test fun laAlarmaDeLosModulosSigueAlProximoAgendadoYSeCancelaSinNada() {
        val ctx = Apoyo.ctx()
        val am = ctx.getSystemService(AlarmManager::class.java)
        motor.proximoModulosTs = System.currentTimeMillis() / 1000 + 3600
        AlarmasModulos.reprogramar(ctx)
        val alarma = shadowOf(am).nextScheduledAlarm // leerla la saca de la fila de Robolectric
        assertNotNull("debía programarse una alarma", alarma)
        assertTrue(Math.abs(alarma!!.triggerAtTime - motor.proximoModulosTs * 1000) < 1500)
        motor.proximoModulosTs = 0
        AlarmasModulos.reprogramar(ctx)
        assertNull(shadowOf(am).nextScheduledAlarm)
    }

    @Test fun alSonarLaAlarmaSeLePideAGoRevisarSuAgendaYSeProgramaLaSiguiente() {
        val ctx = Apoyo.ctx()
        val am = ctx.getSystemService(AlarmManager::class.java)
        motor.proximoModulosTs = System.currentTimeMillis() / 1000 + 7200
        val r = com.botchat.app.AlarmaModulosReceiver()
        r.onReceive(ctx, android.content.Intent(AlarmasModulos.ACCION))
        regla.espera { motor.llamadas.contains("tickModulos") && shadowOf(am).nextScheduledAlarm != null }
    }
}
