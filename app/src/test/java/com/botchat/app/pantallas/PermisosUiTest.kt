package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsOn
import androidx.compose.ui.test.assertIsOff
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextClearance
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.performTouchInput
import androidx.compose.ui.test.down
import androidx.compose.ui.test.moveBy
import androidx.compose.ui.test.up
import androidx.compose.ui.geometry.Offset
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.DialogoConfirmacion
import com.botchat.app.ui.EditorRango
import com.botchat.app.ui.PantallaInfoChat
import com.botchat.app.ui.PantallaPermisos
import com.botchat.app.ui.PantallaPermisosAvanzado
import com.botchat.app.ui.PantallaPoliticasGrupo
import com.botchat.app.ui.PantallaRangos
import com.botchat.app.ui.PantallaRegistroAcciones
import com.botchat.app.ui.PantallaSolicitudes
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
class PermisosUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L
    private val ana = "5215511111111@s.whatsapp.net"
    private val luis = "5215522222222@s.whatsapp.net"

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()

    // ---------- Rangos ----------

    @Test fun laListaMuestraElDuenoFijoYLasPlantillas() {
        regla.setContent { BotChatTema { PantallaRangos(bot, {}, {}) } }
        regla.esperaTexto("Dueño")
        regla.esperaTexto("Ayudante")
        regla.esperaTexto("Cliente VIP")
        regla.onNodeWithText("Puede todo · no se puede cambiar", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithText("2 personas · 20 comandos/min · plantilla", useUnmergedTree = true).assertIsDisplayed()
        // El Dueño no se arrastra; los demás sí.
        assertFalse(hay("manija_1"))
        assertTrue(hay("manija_2") && hay("manija_3"))
    }

    @Test fun arrastrarUnRangoCambiaElOrdenYSeGuarda() {
        regla.setContent { BotChatTema { PantallaRangos(bot, {}, {}) } }
        regla.esperaEtiqueta("manija_2")
        regla.onNodeWithTag("manija_2", useUnmergedTree = true).performTouchInput { down(center); moveBy(Offset(0f, 400f)); up() }
        regla.espera { llamo("reordenarRangos:") }
        assertEquals("[3,2]", motor.llamadas.first { it.startsWith("reordenarRangos:") }.removePrefix("reordenarRangos:"))
    }

    @Test fun tocarUnRangoYElBotonNuevoAbrenElEditor() {
        var abierto = -1L
        regla.setContent { BotChatTema { PantallaRangos(bot, {}, { abierto = it }) } }
        regla.esperaEtiqueta("rango_2")
        regla.onNodeWithTag("rango_2", useUnmergedTree = true).performClick()
        regla.espera { abierto == 2L }
        regla.onNodeWithTag("nuevo_rango").performClick()
        regla.espera { abierto == 0L }
    }

    @Test fun editorCreaUnRangoConAlcancePorCasillaLimiteYMenus() {
        motor.autos = Apoyo.arreglo(JSONObject().put("id", 5).put("activo", true).put("sistema", false).put("version", 1).put("nombre", "Ventas").put("resumen", "x").put("avisos", 0).put("pasos", 2))
        motor.chatsPersonales = Apoyo.arreglo(Apoyo.chatJson(ana, "Ana Pérez"))
        var volvio = false
        regla.setContent { BotChatTema { EditorRango(bot, 0, { volvio = true }) } }
        regla.esperaEtiqueta("rango_nombre")
        regla.onNodeWithTag("rango_nombre").performTextInput("Gerente")
        regla.onNodeWithTag("color_#E5484D").performScrollTo().performClick()
        // Casilla con alcance "solo privado"
        regla.onNodeWithTag("sw_casilla_personas.silenciar").performScrollTo().performClick()
        regla.onNodeWithTag("alcance_personas.silenciar_privado").performScrollTo().performClick()
        // Casilla con chats concretos
        regla.onNodeWithTag("sw_casilla_chat.pausar").performScrollTo().performClick()
        regla.onNodeWithTag("alcance_chat.pausar_chats").performScrollTo().performClick()
        regla.esperaEtiqueta("elegir_$ana")
        regla.onNodeWithTag("elegir_$ana").performClick()
        regla.onNodeWithTag("aceptar_chats").performClick()
        // Solo existe en grupos: no ofrece "solo privado"
        regla.onNodeWithTag("sw_casilla_grupos.expulsar").performScrollTo().performClick()
        assertFalse(hay("alcance_grupos.expulsar_privado"))
        assertTrue(regla.onAllNodes(androidx.compose.ui.test.hasText("Pide código"), useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty())
        // Una básica no se marca, se menciona aparte
        assertFalse(hay("sw_casilla_cmd.ayuda"))
        // Menús: solo algunos
        regla.onNodeWithTag("menus_lista").performScrollTo().performClick()
        regla.onNodeWithTag("menu_5").performScrollTo().performClick()
        regla.onNodeWithTag("guardar_rango").performScrollTo().performClick()
        regla.espera { volvio }
        val json = JSONObject(motor.llamadas.first { it.startsWith("guardarRango:") }.removePrefix("guardarRango:"))
        assertEquals(0, json.getInt("id"))
        assertEquals("Gerente", json.getString("nombre"))
        assertEquals("#E5484D", json.getString("color"))
        assertEquals(10, json.getInt("limiteMin"))
        val c = json.getJSONObject("casillas")
        assertEquals("privado", c.getJSONObject("personas.silenciar").getString("modo"))
        assertEquals("chats", c.getJSONObject("chat.pausar").getString("modo"))
        assertEquals(ana, c.getJSONObject("chat.pausar").getJSONArray("chats").getString(0))
        assertTrue(c.has("grupos.expulsar"))
        assertEquals("lista", json.getString("menusModo"))
        assertEquals("5", json.getJSONArray("menus").getString(0))
    }

    @Test fun editorDeUnaPlantillaCargaSusCasillasYPuedeBorrarse() {
        var volvio = false
        regla.setContent { BotChatTema { EditorRango(bot, 2, { volvio = true }) } }
        regla.esperaEtiqueta("rango_nombre")
        regla.onNodeWithTag("sw_casilla_personas.silenciar").performScrollTo().assertIsOn()
        regla.onNodeWithTag("sw_casilla_chat.pausar").performScrollTo().assertIsOff()
        regla.onNodeWithTag("borrar_rango").performClick()
        regla.esperaTexto("¿Borrar “Ayudante”?")
        regla.esperaTexto("Quienes lo tienen (2) se quedarán sin rango.")
    }

    @Test fun borrarUnRangoPideConfirmacion() {
        var volvio = false
        regla.setContent { BotChatTema { EditorRango(bot, 3, { volvio = true }) } }
        regla.esperaEtiqueta("borrar_rango")
        regla.onNodeWithTag("borrar_rango").performClick()
        regla.esperaTexto("¿Borrar “Cliente VIP”?")
        regla.onNodeWithTag("confirmar_borrar_rango").performClick()
        regla.espera { llamo("borrarRango:3") && volvio }
    }

    @Test fun elDuenoSeVeSinEditar() {
        regla.setContent { BotChatTema { EditorRango(bot, 1, {}) } }
        regla.esperaTexto("Es el único rango fijo", substring = true)
        assertFalse(hay("guardar_rango"))
        assertFalse(hay("borrar_rango"))
    }

    @Test fun unErrorAlGuardarSeMuestraYNoSale() {
        motor.rangoFalla = "ya hay un rango con ese nombre"
        var volvio = false
        regla.setContent { BotChatTema { EditorRango(bot, 0, { volvio = true }) } }
        regla.esperaEtiqueta("rango_nombre")
        regla.onNodeWithTag("rango_nombre").performTextInput("Ayudante")
        regla.onNodeWithTag("guardar_rango").performScrollTo().performClick()
        regla.espera { llamo("guardarRango:") }
        regla.waitForIdle()
        assertFalse(volvio)
    }

    // ---------- Solicitudes ----------

    @Test fun solicitudesAprobarBloquearIgnorar() {
        motor.solicitudesPor["pendiente"] = Apoyo.arreglo(Apoyo.solicitudJson(ana, "Ana"), Apoyo.solicitudJson(luis, "", mensajes = 1))
        regla.setContent { BotChatTema { PantallaSolicitudes(bot, {}) } }
        regla.esperaTexto("Ana")
        regla.onNodeWithText("+5215522222222", useUnmergedTree = true).assertExists() // sin nombre: se ve el número, nunca un LID
        regla.onNodeWithTag("aprobar_$ana").performClick()
        regla.espera { llamo("aprobar:$ana") }
        regla.onNodeWithTag("bloquear_$ana").performClick()
        regla.espera { llamo("bloquear:$ana") }
        regla.onNodeWithTag("ignorar_$luis").performClick()
        regla.espera { llamo("ignorar:$luis") }
    }

    @Test fun solicitudesPorEstadoOfreceLasAccionesQueCorresponden() {
        motor.solicitudesPor["bloqueada"] = Apoyo.arreglo(Apoyo.solicitudJson(ana, "Ana", "bloqueada"))
        motor.solicitudesPor["ignorada"] = Apoyo.arreglo(Apoyo.solicitudJson(luis, "Luis", "ignorada"))
        regla.setContent { BotChatTema { PantallaSolicitudes(bot, {}) } }
        regla.esperaEtiqueta("solicitudes_bloqueada")
        regla.onNodeWithTag("solicitudes_bloqueada").performClick()
        regla.esperaEtiqueta("quitar_$ana")
        assertFalse(hay("bloquear_$ana"))
        assertTrue(hay("aprobar_$ana"))
        regla.onNodeWithTag("quitar_$ana").performClick()
        regla.espera { llamo("quitarSolicitud:$ana") }
        regla.onNodeWithTag("solicitudes_ignorada").performClick()
        regla.esperaEtiqueta("aprobar_$luis")
        assertTrue(hay("bloquear_$luis"))
        assertFalse(hay("ignorar_$luis"))
        regla.onNodeWithText("Descartadas sin bloquear", substring = true).assertIsDisplayed()
    }

    @Test fun solicitudesVacias() {
        regla.setContent { BotChatTema { PantallaSolicitudes(bot, {}) } }
        regla.esperaTexto("Nada por aquí")
    }

    // ---------- Ficha de la persona ----------

    private fun ficha(jid: String = ana) = regla.setContent { BotChatTema { PantallaInfoChat(bot, jid, {}, {}, {}) } }

    @Test fun laFichaMuestraElRangoYLoCambiaConUnToqueConCaducidad() {
        motor.chat = Apoyo.chatJson(ana, "Ana Pérez").toString()
        ficha()
        regla.esperaTexto("Sin rango")
        regla.onNodeWithTag("cambiar_rango").performClick()
        regla.esperaEtiqueta("elegir_rango_2")
        regla.onNodeWithTag("duracion_7d").performClick()
        regla.onNodeWithTag("elegir_rango_2").performClick()
        regla.espera { llamo("asignarRango:$ana:2:7d") }
    }

    @Test fun laFichaAsignaPermanenteDeFormaPredeterminadaYRevoca() {
        motor.chat = Apoyo.chatJson(ana, "Ana Pérez").toString()
        val vence = System.currentTimeMillis() / 1000 + 3 * 86400 + 100
        motor.rangoPersona = JSONObject().put("rol", JSONObject(Apoyo.rangoJson(2, "Ayudante", 1, color = "#2EC4CB").toString()))
            .put("asignacion", JSONObject().put("persona", ana).put("rolId", 2).put("caducaEn", vence).put("asignadoPor", "yo").put("asignadoTs", 1)).put("bloqueada", false).toString()
        ficha()
        regla.esperaTexto("Ayudante")
        regla.onNodeWithText("Vence en 3 días", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("cambiar_rango").performClick()
        regla.esperaEtiqueta("elegir_rango_3")
        regla.onNodeWithTag("elegir_rango_3").performClick()
        regla.espera { llamo("asignarRango:$ana:3:perm") }
        regla.onNodeWithTag("cambiar_rango").performClick()
        regla.esperaEtiqueta("revocar_rango")
        regla.onNodeWithTag("revocar_rango").performClick()
        regla.espera { llamo("revocarRango:$ana") }
    }

    @Test fun laFichaDeUnaPersonaBloqueadaLoDice() {
        motor.chat = Apoyo.chatJson(ana, "Ana Pérez").toString()
        motor.rangoPersona = """{"rol":null,"asignacion":null,"bloqueada":true}"""
        ficha()
        regla.esperaTexto("Bloqueada en el bot")
    }

    @Test fun laFichaDeUnGrupoMuestraAQuienLeContestaYLoCambia() {
        val g = "120363000000000001@g.us"
        motor.chat = Apoyo.chatJson(g, "Mi grupo", grupo = true).toString()
        motor.participantes = JSONArray().put(JSONObject().put("jid", ana).put("usuario", "1").put("nombre", "Ana").put("telefono", "").put("esAdmin", true)).toString()
        regla.setContent { BotChatTema { PantallaInfoChat(bot, g, {}, {}, {}) } }
        regla.esperaEtiqueta("tarjeta_politica_grupo")
        regla.esperaTexto("Todos")
        assertFalse(hay("tarjeta_rango"))
        regla.onNodeWithTag("cambiar_politica_grupo").performClick()
        regla.esperaEtiqueta("modo_grupo_lista")
        regla.onNodeWithTag("modo_grupo_lista").performClick()
        regla.esperaEtiqueta("persona_$ana")
        regla.onNodeWithTag("persona_$ana").performClick()
        regla.onNodeWithTag("guardar_politica_grupo").performClick()
        regla.espera { llamo("politicaGrupo:") }
        val j = JSONObject(motor.llamadas.first { it.startsWith("politicaGrupo:") }.removePrefix("politicaGrupo:"))
        assertEquals("lista", j.getString("modo")); assertEquals(g, j.getString("chat")); assertEquals(ana, j.getJSONArray("lista").getString(0))
    }

    // ---------- Código de confirmación ----------

    @Test fun elDialogoMuestraElCodigoYSePuedeAprobar() {
        motor.confirmaciones = JSONArray().put(JSONObject().put("codigo", "K7M2QX").put("persona", ana).put("comando", "expulsar").put("args", JSONArray()).put("chat", "g@g.us").put("expiraTs", System.currentTimeMillis() / 1000 + 100)).toString()
        motor.chat = Apoyo.chatJson(ana, "Ana Pérez").toString()
        regla.setContent { BotChatTema { DialogoConfirmacion(bot) } }
        regla.esperaTexto("Confirmación pendiente")
        regla.onNodeWithTag("codigo_confirmacion").assertIsDisplayed()
        regla.onNodeWithText("K7M2QX").assertIsDisplayed()
        regla.espera { runCatching { regla.onNodeWithText("Ana Pérez pidió expulsar a alguien.").assertExists(); true }.getOrDefault(false) }
        regla.onNodeWithTag("aprobar_confirmacion").performClick()
        regla.espera { llamo("confirmar:K7M2QX") }
        regla.esperaSinTexto("Confirmación pendiente")
    }

    @Test fun elDialogoSePuedeCancelarYNoApareceSiYaVencio() {
        motor.confirmaciones = JSONArray().put(JSONObject().put("codigo", "ABCD23").put("persona", "yo").put("comando", "asignar").put("args", JSONArray()).put("chat", "").put("expiraTs", System.currentTimeMillis() / 1000 + 100)).toString()
        regla.setContent { BotChatTema { DialogoConfirmacion(bot) } }
        regla.esperaTexto("Confirmación pendiente")
        regla.onNodeWithText("Tú pidió dar un rango.", substring = false).assertExists()
        regla.onNodeWithTag("cancelar_confirmacion").performClick()
        regla.espera { llamo("cancelarConfirmacion:ABCD23") }
        regla.esperaSinTexto("Confirmación pendiente")
    }

    @Test fun unCodigoVencidoNoSeMuestra() {
        motor.confirmaciones = JSONArray().put(JSONObject().put("codigo", "OLD234").put("persona", "yo").put("comando", "asignar").put("args", JSONArray()).put("chat", "").put("expiraTs", System.currentTimeMillis() / 1000 - 5)).toString()
        regla.setContent { BotChatTema { DialogoConfirmacion(bot) } }
        regla.waitForIdle()
        Thread.sleep(300)
        regla.waitForIdle()
        assertFalse(hay("codigo_confirmacion"))
    }

    // ---------- Ajustes → Rangos y permisos ----------

    @Test fun elCentroDePermisosApagaYEnciendeElRangoUsuarioAutomatico() {
        regla.setContent { BotChatTema { PantallaPermisos(bot, {}, {}, {}, {}) } }
        regla.esperaEtiqueta("sw_usuario_auto")
        regla.onNodeWithTag("sw_usuario_auto", useUnmergedTree = true).assertIsOn() // viene encendido
        regla.onNodeWithTag("sw_usuario_auto", useUnmergedTree = true).performClick()
        regla.espera { llamo("ajustePerm:usuario_auto:0") }
        regla.espera { runCatching { regla.onNodeWithTag("sw_usuario_auto", useUnmergedTree = true).assertIsOff() }.isSuccess }
        regla.onNodeWithTag("sw_usuario_auto", useUnmergedTree = true).performClick()
        regla.espera { llamo("ajustePerm:usuario_auto:1") }
    }

    @Test fun elCentroDePermisosCambiaLaPoliticaDeDesconocidos() {
        motor.solicitudesPor["pendiente"] = Apoyo.arreglo(Apoyo.solicitudJson(ana, "Ana"))
        regla.setContent { BotChatTema { PantallaPermisos(bot, {}, {}, {}, {}) } }
        regla.esperaEtiqueta("desconocidos_aprobar")
        regla.onNodeWithText("1 persona espera tu decisión", useUnmergedTree = true).assertExists()
        regla.onNodeWithTag("desconocidos_aprobar").performClick()
        regla.espera { llamo("ajustePerm:desconocidos:aprobar") }
        regla.onNodeWithTag("desconocidos_pronto").performClick()
        regla.espera { llamo("ajustePerm:desconocidos:pronto") }
        regla.esperaEtiqueta("texto_pronto")
        regla.onNodeWithTag("texto_pronto").performTextClearance()
        regla.onNodeWithTag("texto_pronto").performTextInput("Ya te atendemos")
        regla.onNodeWithTag("guardar_texto_pronto").performClick()
        regla.espera { llamo("ajustePerm:desconocidos_texto:Ya te atendemos") }
    }

    @Test fun elCentroDePermisosNavegaARangosSolicitudesYGrupos() {
        val destinos = mutableListOf<String>()
        regla.setContent { BotChatTema { PantallaPermisos(bot, {}, { destinos += "rangos" }, { destinos += "solicitudes" }, { destinos += "grupos" }) } }
        regla.esperaEtiqueta("permisos_rangos")
        regla.onNodeWithTag("permisos_rangos").performClick()
        regla.onNodeWithTag("permisos_solicitudes").performClick()
        regla.onNodeWithTag("permisos_grupos").performClick()
        assertEquals(listOf("rangos", "solicitudes", "grupos"), destinos)
    }

    @Test fun politicasDeGruposPorGrupoYGeneral() {
        val g = "120363000000000001@g.us"
        motor.chatsGrupos = Apoyo.arreglo(Apoyo.chatJson(g, "Mi grupo", grupo = true))
        motor.politicas = JSONArray().put(JSONObject().put("chat", g).put("modo", "admins").put("lista", JSONArray())).toString()
        motor.politicaGrupo = JSONObject().put("chat", g).put("modo", "admins").put("lista", JSONArray()).toString()
        regla.setContent { BotChatTema { PantallaPoliticasGrupo(bot, {}) } }
        regla.esperaTexto("Mi grupo")
        regla.onNodeWithText("Solo administradores del grupo", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("general_admins").performClick()
        regla.espera { llamo("ajustePerm:grupo_modo_defecto:admins") }
        regla.onNodeWithTag("politica_$g").performClick()
        regla.esperaEtiqueta("modo_grupo_todos")
        regla.onNodeWithTag("modo_grupo_todos").performClick()
        regla.onNodeWithTag("guardar_politica_grupo").performClick()
        regla.espera { llamo("politicaGrupo:") }
        assertEquals("todos", JSONObject(motor.llamadas.first { it.startsWith("politicaGrupo:") }.removePrefix("politicaGrupo:")).getString("modo"))
    }

    @Test fun avanzadoGuardaElPrefijoYPruebaUnComando() {
        regla.setContent { BotChatTema { PantallaPermisosAvanzado(bot, {}, {}) } }
        regla.esperaEtiqueta("ajuste_prefijo")
        regla.onNodeWithTag("ajuste_prefijo").performTextClearance()
        regla.onNodeWithTag("ajuste_prefijo").performTextInput("#")
        regla.onNodeWithTag("guardar_prefijo").performClick()
        regla.espera { llamo("ajustePerm:prefijo:#") }
        regla.onNodeWithTag("ejecutar_comando").performScrollTo().performClick()
        regla.espera { llamo("comando:!ayuda") }
        regla.esperaTexto("Esto puedes usar", substring = true)
    }

    @Test fun avanzadoMuestraElErrorDeUnValorMalo() {
        motor.rangoFalla = "el prefijo debe ser un símbolo"
        regla.setContent { BotChatTema { PantallaPermisosAvanzado(bot, {}, {}) } }
        regla.esperaEtiqueta("ajuste_prefijo")
        regla.onNodeWithTag("ajuste_prefijo").performTextClearance()
        regla.onNodeWithTag("ajuste_prefijo").performTextInput("a")
        regla.onNodeWithTag("guardar_prefijo").performClick()
        regla.espera { llamo("ajustePerm:prefijo:a") }
    }

    @Test fun elRegistroDeAccionesNoMuestraTextosSoloQuienQueYResultado() {
        motor.acciones = JSONArray().put(JSONObject().put("id", 2).put("ts", System.currentTimeMillis() / 1000).put("quien", ana).put("accion", "cmd.asignar").put("objetivo", luis).put("chat", "").put("resultado", "denegado").put("detalle", "jerarquia"))
            .put(JSONObject().put("id", 1).put("ts", System.currentTimeMillis() / 1000).put("quien", "yo").put("accion", "rango.guardar").put("objetivo", "").put("chat", "").put("resultado", "ok").put("detalle", "rol=2")).toString()
        regla.setContent { BotChatTema { PantallaRegistroAcciones(bot, {}) } }
        regla.esperaTexto("cmd.asignar")
        regla.onNodeWithText("denegado", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithText("+5215511111111 → +5215522222222", substring = true, useUnmergedTree = true).assertExists()
        regla.onNodeWithText("Tú", substring = true, useUnmergedTree = true).assertExists()
    }
}
