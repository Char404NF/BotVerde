package com.botchat.app.pantallas

import androidx.activity.ComponentActivity
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.test.assertIsNotEnabled
import com.botchat.app.datos.Ajustes
import com.botchat.app.datos.BorradorAsistente
import com.botchat.app.datos.EstadoAlmacen
import com.botchat.app.datos.JsonE8
import com.botchat.app.datos.JsonE9
import com.botchat.app.ui.AvisoTopeAlmacen
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaAsistenteBot
import com.botchat.app.ui.Raiz
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

/** Etapa E9 (0.6.0): tope pendiente al actualizar y asistente «Crea tu bot». */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class E9UiTest {
    @get:Rule val regla = createAndroidComposeRule<ComponentActivity>()
    private lateinit var motor: MotorFalso
    private var bot = 0L
    private val g1 = "120363000000000001@g.us"
    private val g2 = "120363000000000002@g.us"
    private val GB = 1024L * 1024 * 1024

    @Before fun antes() {
        motor = Apoyo.reinicia(); bot = Apoyo.creaBot()
        Ajustes.get(Apoyo.ctx()).aceptarAviso()
        motor.gruposAdmin = "[{\"jid\":\"$g1\",\"nombre\":\"Vecinos\",\"miembros\":12,\"soyAdmin\":true,\"sabido\":true},{\"jid\":\"$g2\",\"nombre\":\"Equipo\",\"miembros\":5,\"soyAdmin\":false,\"sabido\":true}]"
    }
    private fun hay(tag: String) = regla.onAllNodes(hasTestTag(tag), useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun toca(tag: String) {
        regla.espera { hay(tag) }
        val n = regla.onNodeWithTag(tag, useUnmergedTree = true)
        runCatching { n.performScrollTo() }
        n.performClick()
    }
    private fun escribe(tag: String, t: String) { regla.espera { hay(tag) }; regla.onNodeWithTag(tag, useUnmergedTree = true).performTextReplacement(t) }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun asistente() = regla.setContent { BotChatTema { PantallaAsistenteBot(bot, alSalir = {}, alEditarMenu = { motor.llamadas += "editar_menu:$it" }, alEditarBienvenida = { motor.llamadas += "editar_bienvenida" }) } }
    private fun guardado() = JsonE9.borrador(JSONObject(motor.asistGuardado ?: error("no se guardó nada")))
    private fun esperaGuardado(f: (BorradorAsistente) -> Boolean) = regla.espera { motor.asistGuardado?.let { runCatching { f(guardado()) }.getOrDefault(false) } == true }

    // ---------- A) tope pendiente ----------

    @Test fun elEstadoTraeSiElTopeEstaPendiente() {
        val e = JsonE8.estado("{\"topeTotal\":1073741824,\"uso\":3221225472,\"topePendiente\":true,\"tipos\":[]}")
        assertTrue(e.topePendiente); assertTrue(e.pasadoDelTope)
        assertFalse(JsonE8.estado("{\"topeTotal\":1,\"uso\":0,\"tipos\":[]}").topePendiente)
    }

    @Test fun elAvisoPendienteDiceQueNoSeBorraNadaHastaElegir() {
        val e = EstadoAlmacen(1 * GB, 3 * GB, 0, 0, 0, 0, true, 0, emptyList(), topePendiente = true)
        regla.setContent { BotChatTema { AvisoTopeAlmacen(e, {}, {}) } }
        regla.espera { hay("aviso_tope_texto") }
        regla.onNodeWithTag("aviso_tope_texto", useUnmergedTree = true).assertExists()
        regla.esperaTexto("Mientras no elijas, no se borra nada", substring = true)
    }

    private fun estadoPendiente(pasado: Boolean) {
        motor.topePend = true
        motor.estadoAlmJson = JSONObject().put("topeTotal", 1 * GB).put("uso", if (pasado) 3 * GB else 0L).put("topePendiente", true).put("tipos", org.json.JSONArray()).toString()
    }

    @Test fun alActualizarConElTopePasadoSaleElAvisoYSoloAlElegirSeConfirma() {
        estadoPendiente(pasado = true)
        regla.setContent { BotChatTema { Raiz() } }
        regla.espera { hay("aviso_tope") }
        assertTrue("Go sigue esperando", motor.topePend)
        assertFalse(llamo("tope_confirmado"))
        toca("aviso_tope_ok")
        regla.espera { llamo("tope_confirmado") }
        assertFalse(motor.topePend)
    }

    @Test fun subirElTopeFijaElNuevoYConfirma() {
        estadoPendiente(pasado = true)
        regla.setContent { BotChatTema { Raiz() } }
        regla.espera { hay("aviso_tope") }
        toca("aviso_tope_sin_limite")
        regla.espera { llamo("tope_confirmado") }
        assertTrue(llamo("tope:0"))
    }

    @Test fun siElTopeYaNoSePasaSeConfirmaSinMostrarNada() {
        estadoPendiente(pasado = false)
        regla.setContent { BotChatTema { Raiz() } }
        regla.espera { llamo("tope_confirmado") }
        assertFalse(hay("aviso_tope"))
    }

    // ---------- C) asistente ----------

    @Test fun arrancaEnElPaso1ConPrivadosYAvanzaConSiguienteHastaElResumen() {
        asistente()
        regla.esperaTexto("¿Dónde responde tu bot?")
        regla.esperaTexto("Paso 1 de 5")
        assertTrue(hay("asist_privados"))
        toca("asistente_siguiente"); regla.esperaTexto("Mensaje de bienvenida"); regla.esperaTexto("Paso 2 de 5")
        toca("asistente_siguiente"); regla.esperaTexto("Tu menú"); regla.esperaTexto("Paso 3 de 5")
        toca("asistente_siguiente"); regla.esperaTexto("Reglas de la casa")
        toca("asistente_siguiente"); regla.esperaTexto("Utilidades")
        toca("asistente_siguiente"); regla.esperaTexto("Así queda tu bot"); regla.esperaTexto("Resumen")
        assertTrue(hay("asist_crear"))
        assertFalse("en el resumen ya no hay Siguiente", hay("asistente_siguiente"))
    }

    @Test fun elBotBaseDejaElSaludoConTresOpcionesYLaFotoDelAvatar() {
        asistente()
        regla.esperaTexto("¿Dónde responde tu bot?")
        toca("asist_usar_base")
        esperaGuardado { b -> b.menuOpciones.map { it.texto } == listOf("GitHub", "Menú", "Ayuda") && b.bienvenidaAdjuntos.any { it.nombre == "avatar_feliz.png" } }
        assertTrue(llamo("asist_base"))
        assertTrue(guardado().grupos)
    }

    @Test fun enLaBienvenidaSeEligeElAnimoDelAvatar() {
        asistente()
        regla.esperaTexto("¿Dónde responde tu bot?")
        toca("asistente_siguiente"); regla.esperaTexto("Mensaje de bienvenida")
        toca("animo_dormido")
        esperaGuardado { b -> b.bienvenidaAdjuntos.any { it.nombre == "avatar_dormido.png" } }
        toca("animo_dormido") // tocar el que está puesto lo quita
        esperaGuardado { b -> b.bienvenidaAdjuntos.none { it.nombre.startsWith("avatar_") } }
    }

    @Test fun saltarMarcaElPasoYNoCambiaNada() {
        asistente()
        regla.esperaTexto("Paso 1 de 5")
        toca("asistente_siguiente"); regla.esperaTexto("Paso 2 de 5")
        toca("asistente_saltar"); regla.esperaTexto("Paso 3 de 5")
        esperaGuardado { it.saltado(2) }
        assertFalse(guardado().saltado(1))
    }

    @Test fun dondeRespondeGruposMuestraLaListaConCasillas() {
        asistente()
        regla.esperaTexto("Chats privados")
        assertFalse(hay("asist_grupo_$g1"))
        toca("asist_grupos")
        regla.espera { hay("asist_grupo_$g1") }
        assertTrue(hay("asist_grupo_$g2")); assertTrue(hay("asist_elige_grupo"))
        toca("asist_grupo_$g1")
        esperaGuardado { it.grupos && it.gruposElegidos == listOf(g1) }
        regla.espera { !hay("asist_elige_grupo") }
        toca("asist_privados")
        esperaGuardado { !it.privados }
    }

    @Test fun bienvenidaTextoCuandoYVistaPrevia() {
        asistente()
        toca("asistente_siguiente")
        regla.esperaTexto("Mensaje de bienvenida")
        assertTrue(hay("asist_bienvenida_ejemplo")); assertTrue(hay("asist_fmt_negrita")); assertTrue(hay("asist_bienvenida_vista"))
        escribe("asist_bienvenida_texto", "Hola {nombre}, soy Bot Verde")
        regla.esperaTexto("Hola Ana, soy Bot Verde", substring = true)
        toca("asist_cuando_diaria")
        esperaGuardado { it.bienvenidaCuando == "diaria" && it.bienvenidaTexto == "Hola {nombre}, soy Bot Verde" }
        toca("asist_bienvenida_on")
        esperaGuardado { !it.bienvenidaActiva }
        assertFalse(hay("asist_bienvenida_texto"))
    }

    @Test fun menuTresTarjetasAgregarMoverYQuitar() {
        asistente()
        toca("asistente_siguiente"); toca("asistente_siguiente")
        regla.esperaTexto("Tu menú")
        regla.espera { hay("asist_opcion_2") }
        assertFalse(hay("asist_opcion_3"))
        toca("asist_opcion_agregar")
        regla.espera { hay("asist_opcion_3") }
        toca("asist_opcion_bajar_0")
        esperaGuardado { it.menuOpciones.size == 4 && it.menuOpciones[1].texto == "Horarios" && it.menuOpciones[0].texto == "Ubicación" }
        toca("asist_opcion_quitar_3")
        esperaGuardado { it.menuOpciones.size == 3 }
        escribe("asist_opcion_texto_0", "Citas")
        esperaGuardado { it.menuOpciones[0].texto == "Citas" }
        assertTrue(hay("asist_menu_avanzado")); assertTrue(hay("asist_menu_vista"))
        regla.esperaTexto("1. Citas", substring = true)
    }

    @Test fun reglasUsaLaListaEditableYElEnvioAlEntrarPideAvisoDeRiesgo() {
        asistente()
        repeat(3) { toca("asistente_siguiente") }
        regla.esperaTexto("Reglas de la casa")
        assertTrue(hay("lista_reglas")); assertTrue(hay("asist_reglas_comando"))
        toca("asist_reglas_entrar")
        regla.espera { hay("asist_reglas_riesgo") }
        assertTrue(hay("asist_reglas_sin_grupo"))
        toca("asist_reglas_aviso")
        esperaGuardado { it.reglasAlEntrar && it.reglasAvisoOk }
        toca("asist_reglas_entrar") // al apagarlo se olvida el aviso aceptado
        esperaGuardado { !it.reglasAlEntrar && !it.reglasAvisoOk }
    }

    @Test fun utilidadesSonInterruptoresSimples() {
        asistente()
        repeat(4) { toca("asistente_siguiente") }
        regla.esperaTexto("Utilidades")
        listOf("silenciar", "advertencias", "etiquetas", "recordatorios", "notas").forEach { assertTrue(it, hay("asist_util_$it")) }
        toca("asist_util_notas"); toca("asist_util_silenciar")
        esperaGuardado { !it.notas && !it.silenciar && it.recordatorios }
    }

    @Test fun crearMiBotAplicaYDespuesOfreceEditarYDeshacer() {
        motor.asistGuardado = BorradorAsistente(paso = 6, bienvenidaTexto = "Hola").json()
        asistente()
        regla.esperaTexto("Así queda tu bot")
        regla.espera { hay("asist_crear") }
        toca("asist_crear")
        regla.espera { llamo("asist_aplicar") }
        regla.esperaTexto("¡Tu bot está listo!")
        assertTrue(hay("asist_editar_menu")); assertTrue(hay("asist_editar_bienvenida"))
        toca("asist_editar_menu"); assertTrue(llamo("editar_menu:7"))
        toca("asist_editar_bienvenida"); assertTrue(llamo("editar_bienvenida"))
        toca("asist_deshacer")
        regla.espera { hay("asistente_confirma_deshacer") }
        toca("asistente_deshacer_si")
        regla.espera { llamo("asist_deshacer") }
        regla.esperaTexto("Así queda tu bot")
    }

    @Test fun siFaltaAlgoElResumenLoDiceYNoDejaCrear() {
        motor.asistGuardado = BorradorAsistente(paso = 6, privados = false).json()
        motor.asistProblemas = "[\"Elige dónde responde: chats privados o al menos un grupo.\"]"
        asistente()
        regla.esperaTexto("Elige dónde responde", substring = true)
        regla.onNodeWithTag("asist_crear", useUnmergedTree = true).assertIsNotEnabledCompat()
    }

    @Test fun probarSinOtroNumeroLoExplicaYConNumeroPideConfirmacion() {
        motor.asistGuardado = BorradorAsistente(paso = 6, bienvenidaTexto = "Hola").json()
        asistente()
        regla.espera { hay("asist_prueba_explica") }
        assertTrue(hay("asist_prueba_explica"))
        escribe("asist_prueba_numero", "5215512345678")
        toca("asist_probar")
        regla.espera { hay("asist_prueba_confirma") }
        assertFalse("no manda nada sin confirmar", llamo("asist_probar"))
        toca("asist_prueba_si")
        regla.espera { llamo("asist_probar:5215512345678") }
        regla.esperaTexto("Listo, mandé la bienvenida", substring = true)
    }

    @Test fun siLaPruebaFallaMuestraElMotivo() {
        motor.asistGuardado = BorradorAsistente(paso = 6, bienvenidaTexto = "Hola", otroNumero = "5215512345678").json()
        motor.asistProbarError = "Ya mandaste 3 pruebas en la última hora."
        asistente()
        toca("asist_probar"); toca("asist_prueba_si")
        regla.esperaTexto("Ya mandaste 3 pruebas", substring = true)
    }

    @Test fun elBorradorSeReanudaDondeSeDejo() {
        motor.asistGuardado = BorradorAsistente(paso = 3, menuTitulo = "Mi pregunta", menuOpciones = listOf(com.botchat.app.datos.OpcionAsistente("Uno", "r1"))).json()
        asistente()
        regla.esperaTexto("Paso 3 de 5")
        regla.esperaTexto("Tu menú")
        assertTrue(hay("asist_opcion_0")); assertFalse(hay("asist_opcion_1"))
        toca("asistente_atras")
        regla.esperaTexto("Paso 2 de 5")
    }

    @Test fun elHubTieneLaEntradaYLaPrimeraVezSeOfreceUnaSolaVez() {
        motor.asistVisto = false
        regla.setContent { BotChatTema { Raiz() } }
        regla.espera { hay("aviso_asistente") }
        toca("aviso_asistente_despues")
        regla.espera { !hay("aviso_asistente") }
        assertTrue(llamo("asist_visto"))
        regla.onNodeWithTag("nav_bot").performClick()
        regla.espera { hay("hub_asistente") }
        toca("hub_asistente")
        regla.esperaTexto("¿Dónde responde tu bot?")
    }

    @Test fun siYaSeOfrecioNoSalePorSiSolo() {
        regla.setContent { BotChatTema { Raiz() } }
        regla.espera { hay("nav_bot") }
        Thread.sleep(300); regla.waitForIdle()
        assertFalse(hay("aviso_asistente"))
    }
}

private fun androidx.compose.ui.test.SemanticsNodeInteraction.assertIsNotEnabledCompat() = this.assertIsNotEnabled()
