package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextClearance
import androidx.compose.ui.test.performTextInput
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaChat
import com.botchat.app.ui.PantallaEtiquetas
import com.botchat.app.ui.PantallaInicio
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
class ChatTrabajoUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L
    private val jid = "5@s.whatsapp.net"
    private val ahora = System.currentTimeMillis() / 1000

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun etiqueta(id: Long, nombre: String, color: String = "#2EC4CB") = JSONObject().put("id", id).put("nombre", nombre).put("color", color).put("posicion", id).put("chats", 0)

    private fun chat() {
        motor.chat = Apoyo.chatJson(jid, "Ana").toString()
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola", remitente = "Ana"))
    }

    // ---------- en el chat ----------

    @Test fun marcarPendienteYResueltoDesdeElMenuYSeVeLaMarca() {
        chat()
        regla.setContent { BotChatTema { PantallaChat(jid, {}, {}, {}, {}) } }
        regla.esperaTexto("hola")
        assertFalse(hay("marcas_chat"))
        regla.onNodeWithContentDescription("Más opciones").performClick()
        regla.esperaEtiqueta("menu_pendiente")
        regla.onNodeWithText("Marcar como pendiente").assertIsDisplayed()
        regla.onNodeWithTag("menu_pendiente").performClick()
        regla.espera { llamo("estadoChat:$jid:pendiente") }
        regla.esperaEtiqueta("marca_estado_chat")
        regla.esperaTexto("Pendiente")
        // Ahora el menú ofrece quitar la marca y marcar como resuelto.
        regla.onNodeWithContentDescription("Más opciones").performClick()
        regla.esperaEtiqueta("menu_resuelto")
        regla.onNodeWithText("Quitar la marca de pendiente").assertIsDisplayed()
        regla.onNodeWithTag("menu_resuelto").performClick()
        regla.espera { llamo("estadoChat:$jid:resuelto") }
        regla.esperaTexto("✓ Resuelto")
    }

    @Test fun pausarElBotUnRatoYReanudar() {
        chat()
        regla.setContent { BotChatTema { PantallaChat(jid, {}, {}, {}, {}) } }
        regla.esperaTexto("hola")
        regla.onNodeWithContentDescription("Más opciones").performClick()
        regla.esperaEtiqueta("menu_pausar_bot")
        regla.onNodeWithTag("menu_pausar_bot").performClick()
        regla.esperaEtiqueta("pausa_60")
        regla.onNodeWithTag("pausa_60", useUnmergedTree = true).performClick()
        regla.espera { llamo("pausarBot:60") }
        regla.esperaEtiqueta("aviso_bot_pausado")
        regla.esperaTexto("El bot está en pausa en este chat hasta las", substring = true)
        regla.onNodeWithTag("reactivar_bot").performClick()
        regla.espera { llamo("pausarBot:0") }
        regla.espera { !hay("aviso_bot_pausado") }
    }

    @Test fun ponerEtiquetasDesdeElChatYVerlasEnLaCabecera() {
        chat()
        motor.etiquetasJson += etiqueta(1, "Clientes", "#E5484D")
        motor.etiquetasJson += etiqueta(2, "Proveedores")
        regla.setContent { BotChatTema { PantallaChat(jid, {}, {}, {}, {}) } }
        regla.esperaTexto("hola")
        regla.onNodeWithContentDescription("Más opciones").performClick()
        regla.esperaEtiqueta("menu_etiquetas")
        regla.onNodeWithTag("menu_etiquetas").performClick()
        regla.esperaEtiqueta("elegir_etiqueta_1")
        regla.onNodeWithTag("elegir_etiqueta_1", useUnmergedTree = true).performClick()
        regla.espera { llamo("etiquetaChat:$jid:1:true") }
        regla.onNodeWithTag("cerrar_etiquetas_chat", useUnmergedTree = true).performClick()
        // La etiqueta puesta aparece en la cabecera cuando el módulo avisa que el chat cambió.
        com.botchat.app.datos.Repo.emitirParaPruebas("chat:$jid")
        regla.esperaEtiqueta("marcas_chat")
        regla.esperaTexto("Clientes")
    }

    @Test fun desdeElChatSePuedeIrACrearOCambiarEtiquetas() {
        chat()
        var abierto = false
        regla.setContent { BotChatTema { PantallaChat(jid, {}, {}, {}, {}, alAbrirEtiquetas = { abierto = true }) } }
        regla.esperaTexto("hola")
        regla.onNodeWithContentDescription("Más opciones").performClick()
        regla.esperaEtiqueta("menu_etiquetas")
        regla.onNodeWithTag("menu_etiquetas").performClick()
        regla.esperaEtiqueta("sin_etiquetas_chat")
        regla.onNodeWithTag("crear_etiqueta_aqui", useUnmergedTree = true).performClick()
        regla.espera { abierto }
    }

    // ---------- Ajustes → Etiquetas ----------

    @Test fun crearRenombrarYBorrarEtiquetas() {
        regla.setContent { BotChatTema { PantallaEtiquetas(bot, {}) } }
        regla.esperaEtiqueta("sin_etiquetas")
        regla.onNodeWithTag("nueva_etiqueta").performClick()
        regla.esperaEtiqueta("nombre_etiqueta")
        regla.onNodeWithTag("guardar_etiqueta", useUnmergedTree = true).assertIsNotEnabledSafe()
        regla.onNodeWithTag("cancelar_etiqueta", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("nombre_etiqueta", useUnmergedTree = true).performTextInput("Clientes")
        regla.onNodeWithTag("guardar_etiqueta", useUnmergedTree = true).performClick()
        regla.espera { llamo("crearEtiqueta:Clientes:") }
        regla.esperaEtiqueta("etiqueta_fila_1")
        regla.esperaTexto("Sin chats")
        motor.etiquetasDeChatMapa["x"] = mutableSetOf(1L) // un chat la usa
        // renombrar y cambiar el color
        regla.onNodeWithTag("editar_etiqueta_1", useUnmergedTree = true).performClick()
        regla.esperaEtiqueta("nombre_etiqueta")
        regla.onNodeWithTag("nombre_etiqueta", useUnmergedTree = true).performTextClearance()
        regla.onNodeWithTag("nombre_etiqueta", useUnmergedTree = true).performTextInput("VIP")
        regla.onNodeWithTag("color_etiqueta_30A46C", useUnmergedTree = true).performClick()
        regla.onNodeWithTag("guardar_etiqueta", useUnmergedTree = true).performClick()
        regla.espera { llamo("editarEtiqueta:1:VIP:#30A46C") }
        regla.esperaTexto("VIP")
        // borrar pide confirmación
        regla.onNodeWithTag("borrar_etiqueta_1", useUnmergedTree = true).performClick()
        regla.esperaTexto("¿Borrar “VIP”?")
        regla.esperaTexto("Los chats y sus mensajes no se tocan", substring = true)
        assertFalse(llamo("borrarEtiqueta"))
        regla.onNodeWithTag("confirmar_borrar_etiqueta", useUnmergedTree = true).performClick()
        regla.espera { llamo("borrarEtiqueta:1") }
        regla.esperaEtiqueta("sin_etiquetas")
    }

    @Test fun elErrorDeUnNombreRepetidoSeAvisaSinCerrarNada() {
        motor.personaFalla = "ya existe una etiqueta con ese nombre"
        regla.setContent { BotChatTema { PantallaEtiquetas(bot, {}) } }
        regla.esperaEtiqueta("sin_etiquetas")
        regla.onNodeWithTag("nueva_etiqueta").performClick()
        regla.esperaEtiqueta("nombre_etiqueta")
        regla.onNodeWithTag("nombre_etiqueta", useUnmergedTree = true).performTextInput("Clientes")
        regla.onNodeWithTag("guardar_etiqueta", useUnmergedTree = true).performClick()
        regla.espera { llamo("crearEtiqueta:Clientes") }
        regla.esperaEtiqueta("sin_etiquetas")
    }

    // ---------- lista de chats ----------

    private fun chatLista(jid: String, nombre: String, estado: String = "", etiquetas: List<Long> = emptyList()) =
        Apoyo.chatJson(jid, nombre).put("estadoTrabajo", estado).put("etiquetas", JSONArray(etiquetas))

    @Test fun laListaFiltraPorPendientesResueltosYEtiqueta() {
        motor.etiquetasJson += etiqueta(1, "Clientes", "#E5484D")
        motor.pendientes = 1
        motor.chatsPersonales = Apoyo.arreglo(chatLista("a@s.whatsapp.net", "María", "pendiente", listOf(1)), chatLista("b@s.whatsapp.net", "José"))
        motor.chatsFiltrados["pendiente|0"] = Apoyo.arreglo(chatLista("a@s.whatsapp.net", "María", "pendiente", listOf(1)))
        motor.chatsFiltrados["resuelto|0"] = Apoyo.arreglo(chatLista("c@s.whatsapp.net", "Carla", "resuelto"))
        motor.chatsFiltrados["|1"] = Apoyo.arreglo(chatLista("a@s.whatsapp.net", "María", "pendiente", listOf(1)))
        regla.setContent { BotChatTema { PantallaInicio(false, {}, {}) } }
        regla.esperaTexto("José")
        // Marca visible y etiqueta en la fila
        regla.esperaEtiqueta("marca_a@s.whatsapp.net")
        regla.esperaTexto("Clientes")
        regla.esperaTexto("Pendientes (1)")
        regla.onNodeWithTag("filtro_pendientes", useUnmergedTree = true).performClick()
        regla.espera { regla.onAllNodesWithTextSafe("José") == 0 }
        regla.esperaTexto("María")
        assertTrue(llamo("filtrar:false:pendiente:0"))
        regla.onNodeWithTag("filtro_resueltos", useUnmergedTree = true).performClick()
        regla.esperaTexto("Carla")
        regla.esperaEtiqueta("marca_c@s.whatsapp.net")
        regla.onNodeWithTag("filtro_etiqueta_1", useUnmergedTree = true).performClick()
        regla.espera { llamo("filtrar:false::1") }
        regla.espera { regla.onAllNodesWithTextSafe("Carla") == 0 }
        // Volver a "Todos"
        regla.onNodeWithTag("filtro_todos", useUnmergedTree = true).performClick()
        regla.esperaTexto("José")
    }

    @Test fun unFiltroSinChatsDiceQueNoHay() {
        motor.chatsPersonales = Apoyo.arreglo(chatLista("b@s.whatsapp.net", "José"))
        regla.setContent { BotChatTema { PantallaInicio(false, {}, {}) } }
        regla.esperaTexto("José")
        regla.onNodeWithTag("filtro_resueltos", useUnmergedTree = true).performClick()
        regla.esperaEtiqueta("filtro_vacio")
        regla.esperaTexto("Ningún chat con ese filtro")
    }
}

private fun androidx.compose.ui.test.SemanticsNodeInteraction.assertIsNotEnabledSafe() = this.assertIsNotEnabled()
