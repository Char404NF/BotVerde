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
import androidx.compose.ui.test.performTextClearance
import androidx.compose.ui.test.performTextInput
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.EditorComandoPropio
import com.botchat.app.ui.PantallaComandosPropios
import com.botchat.app.ui.PantallaPermisosAvanzado
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
class ComandosPropiosUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun guardado(): JSONObject = JSONObject(motor.llamadas.last { it.startsWith("guardarComando:") }.substringAfter("guardarComando:").substringAfter(":"))
    private fun existente(id: Long, nombre: String, texto: String, activo: Boolean = true, extra: (JSONObject) -> Unit = {}) {
        val datos = JSONObject().put("nombre", nombre).put("texto", texto).put("resumen", "").put("rangoMin", 0).put("privados", true).put("grupos", false).put("ocultoAyuda", false).put("opciones", JSONArray())
        extra(datos)
        motor.comandosPropiosJson += JSONObject().put("id", id).put("activo", activo).put("datos", datos)
    }

    // ---------- lista ----------

    @Test fun sinComandosMuestraElEjemploConElPrefijoActual() {
        motor.ajustesPerm = JSONObject(motor.ajustesPerm).put("prefijo", "#").toString()
        regla.setContent { BotChatTema { PantallaComandosPropios(bot, {}, {}) } }
        regla.esperaEtiqueta("sin_comandos")
        regla.esperaTexto("#horario", substring = true)
    }

    @Test fun laListaMuestraCadaComandoYSuDondeFunciona() {
        existente(1, "horario", "Abrimos a las 9") { it.put("resumen", "Nuestro horario") }
        existente(2, "precios", "Lista", activo = false) { it.put("grupos", true).put("privados", false) }
        var editado = -1L
        regla.setContent { BotChatTema { PantallaComandosPropios(bot, {}, { editado = it }) } }
        regla.esperaTexto("!horario")
        regla.esperaTexto("Nuestro horario")
        regla.esperaTexto("Solo en privados")
        regla.esperaTexto("!precios")
        regla.esperaTexto("Solo en grupos")
        regla.onNodeWithTag("comando_1", useUnmergedTree = true).performClick()
        assertEquals(1L, editado)
        regla.onNodeWithTag("nuevo_comando", useUnmergedTree = true).performClick()
        assertEquals(0L, editado)
    }

    @Test fun encenderApagarYBorrarUnComando() {
        existente(1, "horario", "Abrimos")
        regla.setContent { BotChatTema { PantallaComandosPropios(bot, {}, {}) } }
        regla.esperaEtiqueta("comando_activo_1")
        regla.onNodeWithTag("comando_activo_1", useUnmergedTree = true).performClick()
        regla.espera { llamo("activarAuto:1:false") }
        regla.onNodeWithTag("borrar_comando_1", useUnmergedTree = true).performClick()
        regla.esperaTexto("¿Borrar !horario?")
        assertFalse(llamo("borrarAuto"))
        regla.onNodeWithTag("confirmar_borrar_comando", useUnmergedTree = true).performClick()
        regla.espera { llamo("borrarAuto:1") }
    }

    // ---------- editor ----------

    @Test fun crearUnComandoSimpleConVistaPrevia() {
        var volvio = false
        regla.setContent { BotChatTema { EditorComandoPropio(bot, 0, { volvio = true }) } }
        regla.esperaEtiqueta("comando_nombre")
        regla.onNodeWithTag("guardar_comando", useUnmergedTree = true).performScrollTo().assertIsNotEnabled()
        regla.onNodeWithTag("comando_nombre", useUnmergedTree = true).performTextInput("horario")
        regla.onNodeWithTag("comando_texto", useUnmergedTree = true).performTextInput("Abrimos a las 9, ")
        regla.onNodeWithTag("variable_nombre", useUnmergedTree = true).performScrollTo().performClick()
        regla.onNodeWithTag("comando_resumen", useUnmergedTree = true).performScrollTo().performTextInput("Nuestro horario")
        regla.esperaTexto("Abrimos a las 9, {nombre}", substring = true) // la vista previa (Go) usa el mismo texto
        regla.esperaTexto("Escriben: !horario", substring = true)
        regla.onNodeWithTag("nota_adjuntos", useUnmergedTree = true).performScrollTo().assertIsDisplayed()
        regla.onNodeWithTag("guardar_comando", useUnmergedTree = true).performScrollTo().assertIsEnabled().performClick()
        regla.espera { volvio }
        val j = guardado()
        assertEquals("horario", j.getString("nombre"))
        assertEquals("Abrimos a las 9, {nombre}", j.getString("texto"))
        assertEquals("Nuestro horario", j.getString("resumen"))
        assertTrue(j.getBoolean("privados"))
        assertFalse(j.getBoolean("grupos"))
        assertEquals(0L, j.getLong("rangoMin"))
        assertEquals(0, j.getJSONArray("opciones").length())
    }

    @Test fun elNombreSoloAceptaLetrasYNumeros() {
        regla.setContent { BotChatTema { EditorComandoPropio(bot, 0, {}) } }
        regla.esperaEtiqueta("comando_nombre")
        regla.onNodeWithTag("comando_nombre", useUnmergedTree = true).performTextInput("!Mi horario-2")
        regla.esperaTexto("Se escribe como !Mihorario2", substring = true)
    }

    @Test fun losBotonesDeEstiloEnvuelvenElTexto() {
        regla.setContent { BotChatTema { EditorComandoPropio(bot, 0, {}) } }
        regla.esperaEtiqueta("comando_texto")
        regla.onNodeWithTag("comando_texto", useUnmergedTree = true).performTextInput("hola")
        // Sin texto elegido, la negrita deja las dos marcas y el cursor en medio para escribir ahí.
        regla.onNodeWithTag("estilo_negrita", useUnmergedTree = true).performScrollTo().performClick()
        regla.onNodeWithTag("variable_hora", useUnmergedTree = true).performScrollTo().performClick()
        regla.onNodeWithTag("variable_dia", useUnmergedTree = true).performScrollTo().performClick()
        regla.esperaTexto("hola*{hora}{dia}*", substring = true)
    }

    @Test fun opcionesCreanUnMenuYApagaGrupos() {
        var volvio = false
        regla.setContent { BotChatTema { EditorComandoPropio(bot, 0, { volvio = true }) } }
        regla.esperaEtiqueta("comando_nombre")
        regla.onNodeWithTag("comando_nombre", useUnmergedTree = true).performTextInput("info")
        regla.onNodeWithTag("comando_texto", useUnmergedTree = true).performTextInput("¿Qué quieres saber?")
        regla.onNodeWithTag("comando_grupos", useUnmergedTree = true).performScrollTo().performClick() // grupos encendidos
        regla.onNodeWithTag("agregar_opcion", useUnmergedTree = true).performScrollTo().performClick() // al agregar opciones, grupos se apaga
        regla.onNodeWithTag("opcion_texto_0", useUnmergedTree = true).performScrollTo().performTextInput("Horario")
        regla.onNodeWithTag("opcion_respuesta_0", useUnmergedTree = true).performScrollTo().performTextInput("9 a 6")
        regla.onNodeWithTag("guardar_comando", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { volvio }
        val j = guardado()
        assertFalse(j.getBoolean("grupos"))
        assertEquals("Horario", j.getJSONArray("opciones").getJSONObject(0).getString("texto"))
        assertEquals("9 a 6", j.getJSONArray("opciones").getJSONObject(0).getString("respuesta"))
        assertTrue(j.getBoolean("privados"))
    }

    @Test fun rangoMinimoYVisibilidadEnLaAyuda() {
        motor.rangos = Apoyo.rangosEjemplo
        var volvio = false
        regla.setContent { BotChatTema { EditorComandoPropio(bot, 0, { volvio = true }) } }
        regla.esperaEtiqueta("comando_nombre")
        regla.onNodeWithTag("comando_nombre", useUnmergedTree = true).performTextInput("vip")
        regla.onNodeWithTag("comando_texto", useUnmergedTree = true).performTextInput("Solo VIP")
        regla.esperaEtiqueta("rango_min_2")
        regla.onNodeWithTag("rango_min_2", useUnmergedTree = true).performScrollTo().performClick()
        assertTrue(hay("comando_resumen"))
        regla.onNodeWithTag("comando_en_ayuda", useUnmergedTree = true).performScrollTo().performClick() // ya no sale en la ayuda
        assertFalse(hay("comando_resumen"))
        regla.onNodeWithTag("guardar_comando", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { volvio }
        val j = guardado()
        assertEquals(2L, j.getLong("rangoMin"))
        assertTrue(j.getBoolean("ocultoAyuda"))
    }

    @Test fun cambiarUnComandoExistenteConservaSuId() {
        existente(7, "precios", "Lista vieja")
        var volvio = false
        regla.setContent { BotChatTema { EditorComandoPropio(bot, 7, { volvio = true }) } }
        regla.esperaTexto("Lista vieja")
        regla.onNodeWithTag("comando_texto", useUnmergedTree = true).performTextClearance()
        regla.onNodeWithTag("comando_texto", useUnmergedTree = true).performTextInput("Lista nueva")
        regla.onNodeWithTag("guardar_comando", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { volvio }
        assertTrue(motor.llamadas.any { it.startsWith("guardarComando:7:") })
        assertEquals("Lista nueva", guardado().getString("texto"))
    }

    @Test fun unNombreRepetidoSeExplicaYNoCierraElEditor() {
        existente(1, "horario", "x")
        var volvio = false
        regla.setContent { BotChatTema { EditorComandoPropio(bot, 0, { volvio = true }) } }
        regla.esperaEtiqueta("comando_nombre")
        regla.onNodeWithTag("comando_nombre", useUnmergedTree = true).performTextInput("Horario")
        regla.onNodeWithTag("comando_texto", useUnmergedTree = true).performTextInput("otro")
        regla.onNodeWithTag("guardar_comando", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { motor.llamadas.any { it.startsWith("guardarComando:0:") } }
        Thread.sleep(300)
        assertFalse(volvio)
        regla.onNodeWithTag("guardar_comando", useUnmergedTree = true).performScrollTo().assertIsEnabled() // se puede corregir y reintentar
    }

    // ---------- Avanzado → advertencias ----------

    @Test fun avanzadoCambiaElTopeYLaAccionDeLasAdvertencias() {
        motor.ajustesPerm = JSONObject(motor.ajustesPerm).put("adv_tope", "3").put("adv_accion", "avisar").put("adv_silencio_min", "1440").put("reportar_gracias", "0").toString()
        regla.setContent { BotChatTema { PantallaPermisosAvanzado(bot, {}, {}) } }
        regla.esperaEtiqueta("ajuste_adv_tope")
        regla.onNodeWithTag("ajuste_adv_tope", useUnmergedTree = true).performScrollTo().performTextClearance()
        regla.onNodeWithTag("ajuste_adv_tope", useUnmergedTree = true).performTextInput("5")
        regla.onNodeWithTag("guardar_adv_tope", useUnmergedTree = true).performClick()
        regla.espera { llamo("ajustePerm:adv_tope:5") }
        regla.onNodeWithTag("adv_accion_silenciar", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("ajustePerm:adv_accion:silenciar") }
        regla.esperaEtiqueta("ajuste_adv_silencio_min") // con "silenciar" aparece la duración
        regla.onNodeWithTag("adv_accion_expulsar", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("ajustePerm:adv_accion:expulsar") }
        regla.esperaTexto("solo funcionan en grupos donde el bot es administrador", substring = true)
        // Gracias, ya avisé: apagado por defecto.
        regla.onNodeWithTag("sw_Contestar “Gracias, ya avisé”", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("ajustePerm:reportar_gracias:1") }
        regla.esperaEtiqueta("ajuste_reportar_texto")
    }
}
