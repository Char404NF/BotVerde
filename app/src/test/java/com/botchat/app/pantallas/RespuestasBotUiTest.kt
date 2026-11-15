package com.botchat.app.pantallas

import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextClearance
import androidx.compose.ui.test.performTextInput
import com.botchat.app.datos.JsonRespuestas
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaRespuestasBot
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
class RespuestasBotUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun toca(tag: String) { regla.esperaEtiqueta(tag); regla.onNodeWithTag(tag, useUnmergedTree = true).performClick() }
    private fun pantalla() = regla.setContent { BotChatTema { PantallaRespuestasBot(bot, {}) } }

    @Test fun muestraLasRespuestasPorSeccionConSuTextoDeFabrica() {
        pantalla()
        regla.esperaEtiqueta("respuesta_cmd.sin_permiso")
        regla.esperaTexto("Comandos en general")
        regla.esperaTexto("No tienes permiso para usar ese comando.")
        regla.esperaTexto("Listo: aprobé {n} solicitudes.")
        assertFalse(hay("cambiada_cmd.sin_permiso"))
        assertFalse(hay("restaurar_respuestas")) // nada cambiado: no hay a qué volver
    }

    @Test fun cambiarUnaRespuestaLaGuardaYLaMarcaComoCambiada() {
        pantalla()
        toca("respuesta_cmd.sin_permiso")
        regla.esperaEtiqueta("respuesta_campo")
        regla.esperaEtiqueta("respuesta_defecto")
        regla.onNodeWithTag("respuesta_campo", useUnmergedTree = true).performTextClearance()
        regla.onNodeWithTag("respuesta_campo", useUnmergedTree = true).performTextInput("Eso solo lo hacen los admins")
        toca("respuesta_guardar")
        regla.espera { llamo("respuesta:cmd.sin_permiso:Eso solo lo hacen los admins") }
        regla.esperaEtiqueta("cambiada_cmd.sin_permiso")
        regla.esperaTexto("Eso solo lo hacen los admins")
        assertTrue(hay("restaurar_respuestas"))
        assertEquals(1, regla.onAllNodesWithTag("contador_cambiadas", useUnmergedTree = true).fetchSemanticsNodes().size)
    }

    @Test fun lasVariablesSeAgreganConUnToque() {
        pantalla()
        toca("respuesta_grupo.aprobadas")
        regla.esperaEtiqueta("variable_n")
        regla.onNodeWithTag("respuesta_campo", useUnmergedTree = true).performTextClearance()
        regla.onNodeWithTag("respuesta_campo", useUnmergedTree = true).performTextInput("Entraron ")
        toca("variable_n")
        toca("respuesta_guardar")
        regla.espera { llamo("respuesta:grupo.aprobadas:Entraron {n}") }
    }

    @Test fun dejarElTextoDeFabricaGuardaVacioYVolverAFabricaLoQuita() {
        motor.respuestasJson = motor.respuestasJson.replace("\"propio\":\"\"", "\"propio\":\"Mi texto\"")
        pantalla()
        regla.esperaEtiqueta("cambiada_cmd.sin_permiso")
        toca("respuesta_cmd.sin_permiso")
        toca("respuesta_fabrica")
        regla.espera { llamo("respuesta:cmd.sin_permiso:") && !hay("cambiada_cmd.sin_permiso") || motor.llamadas.contains("respuesta:cmd.sin_permiso:") }
    }

    @Test fun siElMotorRechazaElTextoSeAvisaYNoSeMarcaComoCambiado() {
        motor.rangoFalla = "{x} no existe aquí; puedes usar: {n}"
        pantalla()
        toca("respuesta_grupo.aprobadas")
        regla.esperaEtiqueta("respuesta_campo")
        regla.onNodeWithTag("respuesta_campo", useUnmergedTree = true).performTextInput(" {x}")
        toca("respuesta_guardar")
        regla.espera { llamo("respuesta:grupo.aprobadas:") }
        regla.waitForIdle()
        assertFalse(hay("cambiada_grupo.aprobadas"))
    }

    @Test fun cambiarUnaFraseCualquieraYQuitarla() {
        pantalla()
        toca("nueva_frase")
        regla.esperaEtiqueta("frase_de")
        regla.onNodeWithTag("frase_de", useUnmergedTree = true).performTextInput("Gracias por escribir")
        regla.onNodeWithTag("frase_a", useUnmergedTree = true).performTextInput("¡Gracias por tu mensaje!")
        toca("frase_guardar")
        regla.espera { llamo("reemplazos:") && motor.respuestasJson.contains("Gracias por escribir") }
        regla.esperaEtiqueta("frase_0")
        toca("frase_0")
        toca("frase_quitar")
        regla.espera { motor.respuestasJson.contains("\"reemplazos\":[]") }
    }

    @Test fun volverAFabricaPideConfirmar() {
        motor.respuestasJson = motor.respuestasJson.replaceFirst("\"propio\":\"\"", "\"propio\":\"Mi texto\"")
        pantalla()
        toca("restaurar_respuestas")
        regla.esperaEtiqueta("dialogo_restaurar_respuestas")
        assertFalse(llamo("respuestas_restaurar"))
        toca("confirmar_restaurar_respuestas")
        regla.espera { llamo("respuestas_restaurar") }
        regla.espera { !hay("restaurar_respuestas") }
    }

    @Test fun laBusquedaFiltra() {
        pantalla()
        regla.esperaEtiqueta("buscar_respuesta")
        regla.onNodeWithTag("buscar_respuesta", useUnmergedTree = true).performTextInput("aprobé")
        regla.espera { hay("respuesta_grupo.aprobadas") && !hay("respuesta_cmd.sin_permiso") }
        regla.onNodeWithTag("buscar_respuesta", useUnmergedTree = true).performTextClearance()
        regla.onNodeWithTag("buscar_respuesta", useUnmergedTree = true).performTextInput("zzzz")
        regla.esperaEtiqueta("respuestas_sin_resultado")
    }

    @Test fun leeElJsonDelMotor() {
        val l = JsonRespuestas.lista("""{"grupos":["A"],"filas":[{"id":"x","grupo":"A","descripcion":"d","defecto":"hola {n}","vars":["n"],"propio":"adiós"}],"reemplazos":[{"de":"a","a":"b"}]}""")
        assertEquals("adiós", l.filas[0].actual); assertTrue(l.filas[0].cambiada); assertEquals(listOf("n"), l.filas[0].vars); assertEquals("b", l.reemplazos[0].a)
        assertEquals(0, JsonRespuestas.lista("").filas.size)
    }
}
