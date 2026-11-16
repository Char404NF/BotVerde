package com.botchat.app.pantallas

import android.content.Context
import androidx.activity.ComponentActivity
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextInput
import com.botchat.app.datos.Ajustes
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaCentroBot
import com.botchat.app.ui.PantallaChat
import com.botchat.app.ui.PantallaGrupos
import com.botchat.app.ui.PantallaPersonas
import com.botchat.app.ui.PantallaAvanzado
import com.botchat.app.ui.LocalModoCompleto
import com.botchat.app.ui.Raiz
import androidx.compose.runtime.CompositionLocalProvider
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

/** Etapa E6 (0.6.0): 4 pestañas, modo Simple/Completo, Personas, Grupos, hub Bot y menú ⋮ del chat. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class E6UiTest {
    @get:Rule val regla = createAndroidComposeRule<ComponentActivity>()
    private lateinit var motor: MotorFalso
    private var bot = 0L
    private val ana = "5215512345678@s.whatsapp.net"
    private val beto = "5215587654321@s.whatsapp.net"
    private val grupo = "120363000000000001@g.us"

    @Before fun antes() {
        motor = Apoyo.reinicia(); bot = Apoyo.creaBot()
        motor.chatsPersonales = Apoyo.arreglo(Apoyo.chatJson(ana, "Ana Pérez", texto = "hola"), Apoyo.chatJson(beto, "Beto Ruiz", tel = "+52 55 8765 4321", texto = "qué tal"))
        motor.chatsGrupos = Apoyo.arreglo(Apoyo.chatJson(grupo, "Vecinos del 5", grupo = true, texto = "buenas"))
        motor.chat = Apoyo.chatJson(ana, "Ana Pérez").toString()
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("m1", System.currentTimeMillis() / 1000, "nos vemos", remitente = "Ana"))
        motor.espacioPorChat = "[]"
    }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun androidx.compose.ui.test.junit4.ComposeContentTestRule.esperaSinEtiqueta(tag: String) =
        espera { onAllNodes(androidx.compose.ui.test.hasTestTag(tag), useUnmergedTree = true).fetchSemanticsNodes().isEmpty() }
    private fun raiz() = regla.setContent { BotChatTema { Raiz() } }
    private fun prefs(): android.content.SharedPreferences = Apoyo.ctx().getSharedPreferences("ajustes", Context.MODE_PRIVATE)

    // ---------- navegación y modos ----------

    @Test fun laBarraDeAbajoTieneCuatroPestanasYYaNoEspacioNiAjustes() {
        Ajustes.get(Apoyo.ctx()).aceptarAviso(); raiz()
        regla.esperaTexto("Ana Pérez")
        listOf("nav_inicio", "nav_grupos", "nav_bot", "nav_ajustes").forEach { assertTrue(it, hay(it)) }
        assertFalse(hay("nav_espacio")); assertFalse(hay("nav_personas"))
        regla.onNodeWithTag("nav_ajustes").performClick(); regla.esperaTexto("Seguridad")
        regla.onNodeWithTag("nav_grupos").performClick(); regla.esperaEtiqueta("pantalla_grupos")
        regla.onNodeWithTag("nav_bot").performClick(); regla.esperaEtiqueta("pantalla_centro_bot")
    }

    @Test fun instalacionNuevaArrancaEnSimpleSinAvisoYOcultaLoAvanzado() {
        val a = Ajustes.get(Apoyo.ctx()); a.aceptarAviso()
        assertFalse(a.modoCompleto.value); assertFalse(a.avisoModoPendiente.value)
        raiz(); regla.esperaTexto("Ana Pérez")
        assertFalse(hay("aviso_modo"))
        regla.onNodeWithTag("nav_bot").performClick(); regla.esperaEtiqueta("hub_mi_bot")
        listOf("hub_mi_bot", "hub_almacenamiento", "hub_auditoria", "hub_silencios", "hub_advertencias", "hub_etiquetas", "hub_ajustes").forEach { assertTrue(it, hay(it)) }
        listOf("hub_comandos", "hub_modulos", "hub_limites", "hub_actividad").forEach { assertFalse(it, hay(it)) }
        regla.onNodeWithTag("hub_ajustes").performClick(); regla.esperaEtiqueta("ajuste_seguridad")
        assertFalse(hay("ajuste_modulos")); assertFalse(hay("ajuste_comandos_propios")); assertFalse(hay("ajuste_iniciativa"))
    }

    @Test fun enCompletoSeVeTodoYElCambioSeHaceEnAvanzado() {
        val a = Ajustes.get(Apoyo.ctx()); a.aceptarAviso()
        regla.setContent { BotChatTema { PantallaAvanzado(a, "Mi bot", false, {}, {}, {}) } }
        regla.esperaEtiqueta("modo_completo")
        assertFalse(hay("abrir_actividad"))
        regla.onNodeWithTag("modo_completo").performClick()
        regla.espera { a.modoCompleto.value }
        regla.esperaEtiqueta("abrir_actividad")
        regla.onNodeWithTag("modo_simple").performClick()
        regla.espera { !a.modoCompleto.value }
        regla.esperaSinTexto("Laboratorio")
    }

    @Test fun elHubEnCompletoMuestraLasHerramientasAvanzadasYSusEnlaces() {
        val abiertos = mutableListOf<String>()
        regla.setContent {
            BotChatTema {
                CompositionLocalProvider(LocalModoCompleto provides true) {
                    PantallaCentroBot({ abiertos += "mibot" }, {}, {}, {}, {}, {}, {}, alComandos = { abiertos += "comandos" }, alModulos = { abiertos += "modulos" }, alLimites = { abiertos += "limites" }, alActividad = { abiertos += "act" })
                }
            }
        }
        regla.esperaEtiqueta("hub_comandos")
        regla.onNodeWithTag("hub_comandos").performScrollTo().performClick()
        regla.onNodeWithTag("hub_mi_bot").performScrollTo().performClick()
        assertEquals(listOf("comandos", "mibot"), abiertos)
    }

    @Test fun quienActualizaSinModulosQuedaEnSimpleYVeElAvisoUnaSolaVez() {
        prefs().edit().putBoolean("aviso_aceptado", true).putBoolean("terminos_v1", true).commit(); Ajustes.reiniciarParaPruebas()
        val a = Ajustes.get(Apoyo.ctx())
        assertTrue(a.avisoModoPendiente.value); assertFalse(a.modoCompleto.value)
        raiz()
        regla.esperaTexto("La app ahora es más simple")
        regla.onNodeWithTag("aviso_modo_ok").performClick()
        regla.esperaSinTexto("La app ahora es más simple")
        assertFalse(a.modoCompleto.value); assertFalse(a.avisoModoPendiente.value)
        Ajustes.reiniciarParaPruebas() // «se reinicia la app»: el aviso no vuelve
        assertFalse(Ajustes.get(Apoyo.ctx()).avisoModoPendiente.value)
        assertFalse(Ajustes.get(Apoyo.ctx()).modoCompleto.value)
    }

    @Test fun elAvisoTraeUnAtajoAModoCompleto() {
        prefs().edit().putBoolean("aviso_aceptado", true).putBoolean("terminos_v1", true).commit(); Ajustes.reiniciarParaPruebas()
        val a = Ajustes.get(Apoyo.ctx())
        raiz(); regla.esperaTexto("La app ahora es más simple")
        regla.onNodeWithTag("aviso_modo_completo").performClick()
        regla.espera { a.modoCompleto.value }
        regla.esperaSinTexto("La app ahora es más simple")
        regla.onNodeWithTag("nav_bot").performClick(); regla.esperaEtiqueta("hub_modulos")
    }

    @Test fun quienTieneModulosEncendidosArrancaEnCompleto() {
        motor.modulosLista[1].put("activo", true)
        prefs().edit().putBoolean("aviso_aceptado", true).putBoolean("terminos_v1", true).commit(); Ajustes.reiniciarParaPruebas()
        val a = Ajustes.get(Apoyo.ctx())
        raiz()
        regla.esperaTexto("Te dejamos en modo Completo")
        assertTrue(a.modoCompleto.value)
        regla.onNodeWithTag("aviso_modo_ok").performClick()
        regla.esperaSinTexto("Te dejamos en modo Completo")
        assertTrue(a.modoCompleto.value) // «Seguir en Completo»
        regla.onNodeWithTag("nav_bot").performClick(); regla.esperaEtiqueta("hub_modulos")
    }

    @Test fun elHubAbreSusPantallasYSilenciosYaEsLaReal() {
        Ajustes.get(Apoyo.ctx()).aceptarAviso(); raiz(); regla.esperaTexto("Ana Pérez")
        regla.onNodeWithTag("nav_bot").performClick()
        regla.esperaEtiqueta("hub_silencios"); regla.onNodeWithTag("hub_silencios").performClick()
        regla.esperaEtiqueta("pantalla_silencios"); regla.esperaTexto("No hay nadie silenciado") // E8: ya es la pantalla real
        regla.runOnUiThread { regla.activity.onBackPressedDispatcher.onBackPressed() }; regla.waitForIdle()
        regla.esperaEtiqueta("hub_etiquetas"); regla.onNodeWithTag("hub_etiquetas").performClick()
        regla.esperaTexto("Etiquetas")
    }

    // ---------- Personas y Grupos ----------

    @Test fun personasListaConContadorPuntitosBusquedaYAbrePerfil() {
        motor.etiquetasPersonas = JSONObject().put(ana, JSONArray().put(7)).toString()
        motor.etiquetasJson = mutableListOf(JSONObject().put("id", 7).put("nombre", "Clientes").put("color", "#E5484D").put("posicion", 7))
        var abierta = ""
        regla.setContent { BotChatTema { PantallaPersonas(bot) { abierta = it } } }
        regla.esperaTexto("Ana Pérez"); regla.esperaTexto("Beto Ruiz")
        regla.onNodeWithTag("contador_personas").assertIsDisplayed(); regla.esperaTexto("2 personas")
        regla.esperaEtiqueta("punto_7")
        assertEquals(1, regla.onAllNodesWithTag("punto_7", useUnmergedTree = true).fetchSemanticsNodes().size) // solo Ana lo tiene
        regla.onNodeWithTag("buscador_personas").performTextInput("beto")
        regla.esperaSinTexto("Ana Pérez"); regla.esperaTexto("1 resultado")
        regla.onNodeWithText("Beto Ruiz").performClick()
        assertEquals(beto, abierta)
    }

    @Test fun gruposMuestraMiembrosSoyAdminYAbreLaInfo() {
        motor.gruposAdmin = """[{"jid":"$grupo","nombre":"Vecinos del 5","miembros":24,"soyAdmin":true,"sabido":true}]"""
        var abierto = ""
        regla.setContent { BotChatTema { PantallaGrupos(bot) { abierto = it } } }
        regla.esperaTexto("Vecinos del 5"); regla.esperaTexto("1 grupo")
        regla.esperaTexto("24 miembros · Soy admin")
        regla.onNodeWithTag("buscador_grupos").performTextInput("zzz")
        regla.esperaTexto("Sin resultados")
        regla.onNodeWithTag("buscador_grupos").performTextInput("") // sin efecto
    }

    @Test fun tocarUnGrupoOUnaPersonaAbreSusDestinosDesdeLaApp() {
        Ajustes.get(Apoyo.ctx()).aceptarAviso(); raiz(); regla.esperaTexto("Ana Pérez")
        regla.onNodeWithTag("fab_nuevo_chat").performClick() // "Seleccionar contacto": tocar a alguien abre su chat
        regla.esperaTexto("Beto Ruiz"); regla.onNodeWithTag("persona_$beto").performClick()
        regla.runOnUiThread { regla.activity.onBackPressedDispatcher.onBackPressed() }; regla.waitForIdle()
        regla.esperaTexto("Beto Ruiz")
        regla.runOnUiThread { regla.activity.onBackPressedDispatcher.onBackPressed() }; regla.waitForIdle()
        regla.esperaTexto("Ana Pérez")
        regla.onNodeWithTag("nav_grupos").performClick()
        regla.esperaTexto("Vecinos del 5"); regla.onNodeWithTag("grupo_$grupo").performClick()
        regla.waitForIdle()
    }

    @Test fun unChatFijadoMuestraSuChincheta() {
        motor.chatsPersonales = Apoyo.arreglo(Apoyo.chatJson(ana, "Ana Pérez").put("fijado", true), Apoyo.chatJson(beto, "Beto Ruiz"))
        Ajustes.get(Apoyo.ctx()).aceptarAviso(); raiz()
        regla.esperaTexto("Ana Pérez"); regla.esperaEtiqueta("fijado_$ana")
        assertFalse(hay("fijado_$beto"))
    }

    // ---------- menú ⋮ del chat ----------

    private fun chat() = regla.setContent { BotChatTema { PantallaChat(ana, {}, {}, {}, {}) } }
    private fun abreMenu() { regla.esperaTexto("nos vemos"); regla.onNodeWithContentDescription("Más opciones").performClick(); regla.esperaEtiqueta("menu_buscar") }

    @Test fun elMenuTrae_Buscar_Exportar_Vaciar_Silenciar_Y_Fijar() {
        chat(); abreMenu()
        listOf("menu_buscar", "menu_silenciar", "menu_fijar", "menu_mas").forEach { assertTrue(it, hay(it)) }
        regla.onNodeWithTag("menu_mas").performClick(); regla.esperaEtiqueta("menu_exportar_chat")
        listOf("menu_exportar_chat", "menu_vaciar_chat").forEach { assertTrue(it, hay(it)) }
    }

    @Test fun fijarYSilenciarVanAlMotor() {
        chat(); abreMenu()
        regla.onNodeWithTag("menu_fijar").performClick()
        regla.espera { llamo("fijar:true") }
        regla.onNodeWithContentDescription("Más opciones").performClick(); regla.esperaEtiqueta("menu_fijar")
        regla.esperaTexto("Quitar de los fijados")
        regla.onNodeWithTag("menu_silenciar").performClick()
        regla.espera { llamo("silenciar:true") }
    }

    @Test fun buscarEnElChatUsaBuscarEnChatYLlevaAlMensaje() {
        motor.busqueda = JSONObject().put("total", 1).put("fin", true).put("sigTs", 0).put("sigId", "")
            .put("items", JSONArray().put(Apoyo.msgJson("m1", System.currentTimeMillis() / 1000, "nos vemos mañana", remitente = "Ana"))).toString()
        chat(); abreMenu()
        regla.onNodeWithTag("menu_buscar").performClick()
        regla.esperaEtiqueta("campo_buscar_chat")
        regla.onNodeWithTag("campo_buscar_chat").performTextInput("manana")
        regla.esperaEtiqueta("resultado_m1")
        assertTrue(motor.palabrasBuscadas.contains("manana"))
        regla.onNodeWithTag("resultado_m1").performClick()
        regla.esperaSinEtiqueta("resultados_buscar_chat")
    }

    @Test fun buscarSinResultadosLoDiceYSalirVuelveAlChat() {
        chat(); abreMenu()
        regla.onNodeWithTag("menu_buscar").performClick()
        regla.esperaEtiqueta("campo_buscar_chat")
        regla.onNodeWithTag("campo_buscar_chat").performTextInput("zzzz")
        regla.esperaEtiqueta("sin_resultados_chat")
        regla.onNodeWithTag("cerrar_busqueda").performClick()
        regla.esperaSinEtiqueta("resultados_buscar_chat")
        regla.esperaTexto("nos vemos")
    }

    @Test fun vaciarPideConfirmarDiceQueNoHayDeshacerYSoloEntoncesBorra() {
        chat(); abreMenu()
        regla.onNodeWithTag("menu_mas").performClick(); regla.esperaEtiqueta("menu_vaciar_chat")
        regla.onNodeWithTag("menu_vaciar_chat").performClick()
        regla.esperaEtiqueta("aviso_sin_deshacer"); regla.esperaTexto("no hay «Deshacer»", substring = true)
        regla.onNodeWithTag("cancelar_vaciar").performClick()
        regla.waitForIdle(); assertFalse(llamo("vaciarChat"))
        regla.onNodeWithContentDescription("Más opciones").performClick(); regla.esperaEtiqueta("menu_mas")
        regla.onNodeWithTag("menu_mas").performClick(); regla.esperaEtiqueta("menu_vaciar_chat")
        regla.onNodeWithTag("menu_vaciar_chat").performClick()
        regla.esperaEtiqueta("confirmar_vaciar"); regla.onNodeWithTag("confirmar_vaciar").performClick()
        regla.espera { llamo("vaciarChat:$ana") }
        regla.esperaTexto("Sin mensajes")
    }

    @Test fun exportarElChatPideElFormatoYPreparaElArchivo() {
        chat(); abreMenu()
        regla.onNodeWithTag("menu_mas").performClick(); regla.esperaEtiqueta("menu_exportar_chat")
        regla.onNodeWithTag("menu_exportar_chat").performClick()
        regla.esperaEtiqueta("exportar_txt")
        regla.onNodeWithTag("exportar_txt").performClick()
        regla.espera { llamo("exportarChat:txt") }
        regla.esperaEtiqueta("exportar_chat_listo"); regla.esperaTexto("Listo: 3 mensajes.")
        assertTrue(hay("compartir_chat_cifrado")); assertTrue(hay("guardar_chat_legible"))
        regla.onNodeWithTag("cerrar_exportar_chat").performClick()
    }
}
