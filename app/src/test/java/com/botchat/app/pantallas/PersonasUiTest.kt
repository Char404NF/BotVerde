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
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaAdvertencias
import com.botchat.app.ui.PantallaInfoChat
import com.botchat.app.ui.PantallaPersonaDetalle
import com.botchat.app.ui.TarjetaPersona
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
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class PersonasUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L
    private val ana = "5215511111111@s.whatsapp.net"

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun tarjeta(alDetalle: () -> Unit = {}, alOlvidada: () -> Unit = {}) =
        regla.setContent { BotChatTema { TarjetaPersona(bot, ana, "Ana Pérez", alDetalle, alOlvidada) } }

    @Test fun laFichaMuestraElEstadoLosConteosYLasFechas() {
        motor.fichaJson = JSONObject(Apoyo.fichaEjemplo()).put("advertencias", 2).put("notas", 1).put("noMolestar", true)
            .put("etiquetas", JSONArray().put(JSONObject().put("id", 1).put("nombre", "Clientes").put("color", "#E5484D"))).toString()
        tarjeta()
        regla.esperaEtiqueta("persona_estado")
        regla.onNodeWithText("El bot le contesta normal").assertIsDisplayed()
        regla.onNodeWithTag("persona_conteos", useUnmergedTree = true).assertIsDisplayed()
        regla.esperaTexto("2 advertencias · 1 nota")
        regla.esperaTexto("Está en la lista No molestar")
        regla.esperaTexto("Clientes")
        regla.esperaTexto("Escribió 12 mensajes", substring = true)
        regla.esperaTexto("Primera vez:", substring = true)
    }

    @Test fun silenciarConDuracionYQuitarElSilencio() {
        tarjeta()
        regla.esperaEtiqueta("silenciar_persona")
        regla.onNodeWithTag("silenciar_persona", useUnmergedTree = true).performClick()
        regla.esperaEtiqueta("silencio_480")
        regla.onNodeWithTag("silencio_480", useUnmergedTree = true).performClick()
        regla.onNodeWithTag("confirmar_silenciar", useUnmergedTree = true).performClick()
        regla.espera { llamo("silenciar:$ana:480:") }
        regla.esperaEtiqueta("quitar_silencio")
        regla.esperaTexto("Silenciada hasta", substring = true)
        regla.onNodeWithTag("quitar_silencio", useUnmergedTree = true).performClick()
        regla.espera { llamo("quitarSilencio:$ana") }
        regla.esperaEtiqueta("silenciar_persona")
        regla.esperaTexto("El bot le contesta normal")
    }

    @Test fun silenciarParaSiempreManda0Minutos() {
        tarjeta()
        regla.esperaEtiqueta("silenciar_persona")
        regla.onNodeWithTag("silenciar_persona", useUnmergedTree = true).performClick()
        regla.esperaEtiqueta("silencio_0")
        regla.onNodeWithTag("silencio_0", useUnmergedTree = true).performClick()
        regla.onNodeWithTag("confirmar_silenciar", useUnmergedTree = true).performClick()
        regla.espera { llamo("silenciar:$ana:0:") }
        regla.esperaTexto("Silenciada para siempre")
    }

    @Test fun bloquearPideConfirmacionYSeDesbloquea() {
        tarjeta()
        regla.esperaEtiqueta("bloquear_persona")
        regla.onNodeWithTag("bloquear_persona", useUnmergedTree = true).performClick()
        regla.esperaTexto("No es un bloqueo de WhatsApp", substring = true)
        assertFalse(llamo("bloquearPersona"))
        regla.onNodeWithTag("confirmar_bloquear", useUnmergedTree = true).performClick()
        regla.espera { llamo("bloquearPersona:$ana") }
        regla.esperaEtiqueta("desbloquear_persona")
        regla.esperaTexto("El bot la ignora por completo")
        regla.onNodeWithTag("desbloquear_persona", useUnmergedTree = true).performClick()
        regla.espera { llamo("desbloquearPersona:$ana") }
        regla.esperaEtiqueta("bloquear_persona")
    }

    @Test fun advertirMuestraElResultadoYSiLlegaAlTopeLoQueHizo() {
        motor.resultadoAdvertir = """{"total":3,"tope":3,"llegoAlTope":true,"accion":"expulsar","hecho":"solo_aviso","motivo":"no soy administrador del grupo"}"""
        tarjeta()
        regla.esperaEtiqueta("advertir_persona")
        regla.onNodeWithTag("advertir_persona", useUnmergedTree = true).performClick()
        regla.esperaEtiqueta("motivo_adv_spam")
        regla.onNodeWithTag("motivo_adv_spam", useUnmergedTree = true).performClick()
        regla.onNodeWithTag("confirmar_advertir", useUnmergedTree = true).performClick()
        regla.espera { llamo("advertir:$ana::Spam") }
        regla.esperaEtiqueta("resultado_advertencia")
        regla.esperaTexto("lleva 3 de 3 advertencias", substring = true)
        regla.esperaTexto("No hice la acción (no soy administrador del grupo)", substring = true)
        regla.onNodeWithTag("cerrar_resultado_advertencia", useUnmergedTree = true).performClick()
        regla.esperaTexto("1 advertencia · 0 notas")
    }

    @Test fun exportarGeneraElArchivoYOfreceCompartirOGuardarCopia() {
        tarjeta()
        regla.esperaEtiqueta("exportar_persona")
        regla.onNodeWithTag("exportar_persona", useUnmergedTree = true).performClick()
        regla.espera { llamo("exportarPersona:$ana") }
        regla.esperaEtiqueta("exportar_listo")
        assertTrue(hay("compartir_exporte") && hay("guardar_copia_exporte"))
        regla.esperaTexto("cifrado con la llave de esta app", substring = true)
        regla.onNodeWithTag("cerrar_exportar", useUnmergedTree = true).performClick()
    }

    @Test fun exportarMuestraElErrorSiFalla() {
        motor.personaFalla = "no hay espacio"
        tarjeta()
        regla.esperaEtiqueta("exportar_persona")
        regla.onNodeWithTag("exportar_persona", useUnmergedTree = true).performClick()
        regla.esperaEtiqueta("error_exportar")
        regla.esperaTexto("no hay espacio")
    }

    @Test fun olvidarPideEntenderYAvisaCuandoTermina() {
        var olvidada = false
        tarjeta(alOlvidada = { olvidada = true })
        regla.esperaEtiqueta("olvidar_persona")
        regla.onNodeWithTag("olvidar_persona", useUnmergedTree = true).performClick()
        regla.esperaEtiqueta("texto_olvidar")
        regla.esperaTexto("se mantiene", substring = true)
        regla.onNodeWithTag("confirmar_olvidar", useUnmergedTree = true).assertIsNotEnabled()
        assertFalse(llamo("olvidarPersona"))
        regla.onNodeWithTag("entiendo_olvidar", useUnmergedTree = true).performClick()
        regla.onNodeWithTag("confirmar_olvidar", useUnmergedTree = true).assertIsEnabled().performClick()
        regla.espera { llamo("olvidarPersona:$ana") }
        regla.espera { olvidada }
    }

    @Test fun elBotonDeAdvertenciasYNotasAbreElDetalle() {
        var abierto = false
        tarjeta(alDetalle = { abierto = true })
        regla.esperaEtiqueta("abrir_advertencias_notas")
        regla.onNodeWithTag("abrir_advertencias_notas", useUnmergedTree = true).performClick()
        assertTrue(abierto)
    }

    @Test fun enInfoDelChatPrivadoSalelaFichaYEnUnGrupoNo() {
        motor.chat = Apoyo.chatJson(ana, "Ana Pérez").toString()
        regla.setContent { BotChatTema { PantallaInfoChat(bot, ana, {}, {}, {}) } }
        regla.esperaEtiqueta("tarjeta_persona")
        assertTrue(hay("tarjeta_persona"))
    }

    @Test fun enUnGrupoNoHayFichaDePersona() {
        motor.chat = Apoyo.chatJson("g@g.us", "Familia", tel = "", pais = "", grupo = true).toString()
        regla.setContent { BotChatTema { PantallaInfoChat(bot, "g@g.us", {}, {}, {}) } }
        regla.esperaEtiqueta("tarjeta_politica_grupo")
        assertFalse(hay("tarjeta_persona"))
    }

    // ---------- Advertencias y notas ----------

    @Test fun sinAdvertenciasMuestraElEstadoVacio() {
        regla.setContent { BotChatTema { PantallaAdvertencias(bot, {}, {}) } }
        regla.esperaEtiqueta("sin_advertencias")
    }

    @Test fun laListaMuestraCuantasLlevaCadaPersonaYAbreSuDetalle() {
        motor.advertidosJson = JSONArray().put(JSONObject().put("persona", ana).put("total", 3).put("ultimaTs", System.currentTimeMillis() / 1000).put("nombre", "Ana Pérez").put("tope", 3))
            .put(JSONObject().put("persona", "5215522222222@s.whatsapp.net").put("total", 1).put("ultimaTs", 1_700_000_000L).put("nombre", "").put("tope", 3)).toString()
        var abierta = ""
        regla.setContent { BotChatTema { PantallaAdvertencias(bot, {}, { abierta = it }) } }
        regla.esperaTexto("Ana Pérez")
        regla.esperaTexto("3 de 3")
        regla.esperaTexto("1 de 3")
        regla.esperaTexto("+5215522222222") // sin nombre guardado: su número
        regla.onNodeWithTag("advertido_$ana", useUnmergedTree = true).performClick()
        assertEquals(ana, abierta)
    }

    @Test fun detalleAdvierteConUnMotivoPropioYMuestraLaActividad() {
        motor.resultadoAdvertir = """{"total":1,"tope":3,"llegoAlTope":false,"accion":"avisar","hecho":"","motivo":""}"""
        regla.setContent { BotChatTema { PantallaPersonaDetalle(bot, ana, {}) } }
        regla.esperaEtiqueta("persona_total")
        regla.esperaTexto("A qué hora escribe")
        regla.onNodeWithTag("motivo_advertencia", useUnmergedTree = true).performScrollTo().performTextInput("insultó en el grupo")
        regla.onNodeWithTag("detalle_advertir", useUnmergedTree = true).performClick()
        regla.espera { llamo("advertir:$ana::insultó en el grupo") }
        regla.esperaEtiqueta("resultado_advertencia")
        regla.esperaTexto("lleva 1 de 3 advertencias", substring = true)
    }

    @Test fun detalleQuitaUnaAdvertenciaYBorraElHistorialConConfirmacion() {
        motor.advertenciasJson = JSONArray().put(JSONObject().put("id", 7).put("persona", ana).put("chat", "").put("motivo", "spam").put("ts", 1_700_000_000L).put("por", "yo"))
            .put(JSONObject().put("id", 8).put("persona", ana).put("chat", "").put("motivo", "").put("ts", 1_700_000_500L).put("por", "yo")).toString()
        regla.setContent { BotChatTema { PantallaPersonaDetalle(bot, ana, {}) } }
        regla.esperaEtiqueta("advertencia_7")
        regla.esperaTexto("spam")
        regla.esperaTexto("Sin motivo")
        regla.onNodeWithTag("quitar_advertencia_7", useUnmergedTree = true).performClick()
        regla.espera { llamo("quitarAdvertencia:$ana:7") }
        regla.onNodeWithTag("borrar_historial", useUnmergedTree = true).performScrollTo().performClick()
        regla.esperaTexto("¿Borrar todas las advertencias?")
        assertFalse(llamo("borrarAdvertencias"))
        regla.onNodeWithTag("confirmar_borrar_historial", useUnmergedTree = true).performClick()
        regla.espera { llamo("borrarAdvertencias:$ana") }
        regla.esperaEtiqueta("sin_advertencias_persona")
    }

    @Test fun detalleEscribeYBorraNotas() {
        regla.setContent { BotChatTema { PantallaPersonaDetalle(bot, ana, {}) } }
        regla.esperaEtiqueta("sin_notas_persona")
        regla.onNodeWithTag("guardar_nota_detalle", useUnmergedTree = true).performScrollTo().assertIsNotEnabled()
        regla.onNodeWithTag("texto_nota_detalle", useUnmergedTree = true).performScrollTo().performTextInput("Pidió factura")
        regla.onNodeWithTag("guardar_nota_detalle", useUnmergedTree = true).assertIsEnabled().performClick()
        regla.espera { llamo("nota:$ana:Pidió factura") }
        regla.esperaTexto("Pidió factura")
        regla.onNodeWithTag("borrar_nota_50", useUnmergedTree = true).performClick()
        regla.espera { llamo("borrarNota:$ana:50") }
        regla.esperaEtiqueta("sin_notas_persona")
    }
}
