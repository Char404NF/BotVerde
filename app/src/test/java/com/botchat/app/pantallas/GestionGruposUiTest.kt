package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextInput
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaAvisosCopia
import com.botchat.app.ui.claveCopiaValida
import com.botchat.app.ui.PantallaAdminGrupo
import com.botchat.app.ui.SalidasAdminGrupo
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
class GestionGruposUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L
    private val g = "120363000000000001@g.us"

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun nodo(tag: String) = regla.onNodeWithTag(tag, useUnmergedTree = true)
    private fun toca(tag: String) {
        regla.esperaEtiqueta(tag)
        runCatching { nodo(tag).performScrollTo() }
        nodo(tag).performClick()
    }
    private fun escribe(tag: String, t: String) { regla.esperaEtiqueta(tag); runCatching { nodo(tag).performScrollTo() }; nodo(tag).performTextInput(t) }
    private fun salidas() = SalidasAdminGrupo()
    private fun admin(pestana: Int) = regla.setContent { BotChatTema { PantallaAdminGrupo(bot, g, salidas(), pestana) } }
    private fun avisos() = regla.setContent { BotChatTema { PantallaAvisosCopia(bot, {}) } }

    // ---------- configuración base ----------

    @Test fun sinBaseSeOfreceUsarEsteGrupoYSeGuardaTrasConfirmar() {
        admin(4)
        regla.esperaEtiqueta("tarjeta_base")
        regla.esperaTexto("Todavía no hay una configuración base", substring = true)
        assertFalse(hay("base_aplicar_varios"))
        toca("base_guardar")
        toca("base_confirmar")
        regla.espera { llamo("baseGuardar:$g") }
        regla.esperaEtiqueta("base_aplicar_varios")
        regla.esperaTexto("Base de «Familia»", substring = true)
        regla.esperaTexto("2 filtros encendidos", substring = true)
    }

    @Test fun conBaseSePuedePonerEnVariosGruposYActivarLoAutomatico() {
        motor.baseJson = """{"existe":true,"desde":"Club","fecha":1763000000,"filtros":3,"reglas":true,"horario":false,"entrada":false,"auto":false}"""
        motor.chatsGrupos = org.json.JSONArray().put(Apoyo.chatJson(g, "Familia", grupo = true)).put(Apoyo.chatJson("120363000000000002@g.us", "Club", grupo = true)).toString()
        admin(4)
        regla.esperaEtiqueta("base_aplicar_varios")
        toca("sw_base_auto")
        regla.espera { llamo("baseAuto:true") }
        toca("base_aplicar_varios")
        regla.esperaEtiqueta("base_poner_en_elegidos")
        toca("base_todos")
        toca("base_poner_en_elegidos")
        regla.espera { llamo("baseAplicar:[") }
    }

    @Test fun quitarLaBaseNecesitaConfirmar() {
        motor.baseJson = """{"existe":true,"desde":"Club","fecha":1763000000,"filtros":1,"reglas":false,"horario":false,"entrada":false,"auto":true}"""
        admin(4)
        toca("base_quitar")
        assertFalse(llamo("baseQuitar"))
        toca("base_confirmar")
        regla.espera { llamo("baseQuitar") }
    }

    // ---------- encuesta ----------

    @Test fun laEncuestaSeEnviaConPreguntaOpcionesYVarias() {
        admin(3)
        regla.esperaEtiqueta("tarjeta_encuesta")
        assertFalse(nodoHabilitado("encuesta_enviar"))
        escribe("encuesta_pregunta", "¿Qué cenamos?")
        escribe("encuesta_opcion_0", "Tacos")
        escribe("encuesta_opcion_1", "Pizza")
        toca("encuesta_mas")
        escribe("encuesta_opcion_2", "Sushi")
        toca("sw_encuesta_varias")
        toca("encuesta_enviar")
        regla.espera { llamo("encuesta:¿Qué cenamos?:[\"Tacos\",\"Pizza\",\"Sushi\"]:true") }
    }

    @Test fun siLaEncuestaFallaSeAvisaYSeConservaLoEscrito() {
        motor.grupoFalla = "El bot no está conectado."
        admin(3)
        escribe("encuesta_pregunta", "¿Cena?")
        escribe("encuesta_opcion_0", "Sí")
        escribe("encuesta_opcion_1", "No")
        toca("encuesta_enviar")
        regla.espera { llamo("encuesta:") }
        assertTrue(hay("encuesta_pregunta"))
        regla.esperaTexto("¿Cena?")
    }

    private fun nodoHabilitado(tag: String): Boolean {
        regla.esperaEtiqueta(tag)
        return runCatching { nodo(tag).assertIsEnabled(); true }.getOrDefault(false)
    }

    // ---------- avisos y copia ----------

    @Test fun losAvisosPorWhatsAppSeEnciendenYSeEligenCuales() {
        avisos()
        regla.esperaEtiqueta("sw_alerta_wa")
        assertFalse(hay("alerta_tipo_reporte"))
        toca("sw_alerta_wa")
        regla.espera { llamo("alertas:") && motor.avisosJson.contains("\"wa\":true") }
        regla.esperaEtiqueta("alerta_tipo_reporte")
        toca("alerta_tipo_reporte") // lo quita
        regla.espera { !motor.avisosJson.contains("reporte") }
        toca("alerta_tipo_filtros")
        regla.espera { motor.avisosJson.contains("filtros") }
    }

    @Test fun elAvisoDeDesconexionYElResumenSemanalSeGuardan() {
        avisos()
        toca("desconexion_30")
        regla.espera { motor.avisosJson.contains("\"desconexion\":30") }
        toca("sw_resumen_semanal")
        regla.espera { motor.avisosJson.contains("\"resumen\":true") }
        toca("ver_resumen_ahora")
        regla.esperaEtiqueta("texto_resumen")
        regla.esperaTexto("Mensajes: 90", substring = true)
    }

    @Test fun elResumenVacioSoloAvisa() {
        motor.resumenTexto = ""
        avisos()
        toca("ver_resumen_ahora")
        regla.espera { llamo("resumenSemanal") }
        regla.waitForIdle()
        assertFalse(hay("texto_resumen")) // un resumen vacío no abre el cuadro: solo avisa con un mensaje breve
    }

    @Test fun laContrasenaDeLaCopiaPideOchoYQueCoincida() {
        assertFalse(claveCopiaValida("corta", "corta", true))
        assertFalse(claveCopiaValida("Larga1234", "otra", true))
        assertTrue(claveCopiaValida("Larga1234", "Larga1234", true))
        assertTrue(claveCopiaValida("Larga1234", "", false)) // al restaurar no se repite
        assertFalse(claveCopiaValida("corta", "", false))
    }
}
