package com.botchat.app.pantallas

import androidx.compose.material3.Text
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.assertIsOff
import androidx.compose.ui.test.assertIsOn
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextClearance
import androidx.compose.ui.test.performTextInput
import com.botchat.app.datos.Ajustes
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaAjustes
import com.botchat.app.ui.PantallaConfigModulo
import com.botchat.app.ui.PantallaIniciativa
import com.botchat.app.ui.PantallaModulos
import com.botchat.app.ui.PantallaRegistroIniciativas
import com.botchat.app.ui.PantallasModulo
import org.json.JSONArray
import org.json.JSONObject
import org.junit.After
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
class ModulosUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }
    @After fun despues() { PantallasModulo.quitar("propio_demo") }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()

    private fun lista(alConfigurar: (String) -> Unit = {}, alIniciativa: () -> Unit = {}) =
        regla.setContent { BotChatTema { PantallaModulos(bot, {}, alConfigurar, alIniciativa) } }

    // ---------- tarjetas ----------

    @Test fun cadaModuloEsUnaTarjetaConNombreDescripcionRiesgoInterruptorYConfigurar() {
        lista()
        regla.esperaEtiqueta("modulo_eco_prueba")
        regla.onNodeWithText("Eco de prueba", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithText("Módulo de ejemplo: repite una frase.", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithText("Dados y adivina el número.", useUnmergedTree = true).assertIsDisplayed()
        // Sin etiquetas ni avisos de riesgo: los términos se aceptan una sola vez al empezar.
        assertFalse(hay("riesgo_bajo") || hay("riesgo_medio") || hay("riesgo_alto"))
        // Todo nace apagado, con su botón Configurar.
        for (id in listOf("eco_prueba", "juegos_demo", "difusion_demo", "propio_demo")) {
            regla.onNodeWithTag("modulo_interruptor_$id", useUnmergedTree = true).performScrollTo().assertIsOff()
            regla.onNodeWithTag("modulo_configurar_$id", useUnmergedTree = true).performScrollTo().assertIsDisplayed()
        }
        // Y avisa que por defecto el bot solo responde.
        regla.onNodeWithText("Iniciativa propia: apagada", useUnmergedTree = true).assertIsDisplayed()
    }

    @Test fun sinModulosMuestraUnMensajeAmable() {
        motor.modulosLista.clear()
        lista()
        regla.esperaTexto("Todavía no hay módulos")
    }

    @Test fun elBotonConfigurarAbreLaVentanaDeESeModulo() {
        var abierto = ""
        lista(alConfigurar = { abierto = it })
        regla.esperaEtiqueta("modulo_configurar_eco_prueba")
        regla.onNodeWithTag("modulo_configurar_eco_prueba", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { abierto == "eco_prueba" }
    }

    @Test fun lasTarjetasAbrenIniciativaPropia() {
        var fue = false
        lista(alIniciativa = { fue = true })
        regla.esperaEtiqueta("modulos_iniciativa")
        regla.onNodeWithTag("modulos_iniciativa", useUnmergedTree = true).performClick()
        regla.espera { fue }
    }

    // ---------- encender y apagar ----------

    @Test fun encenderCualquierModuloEsInmediatoSinAvisos() {
        lista()
        for (id in listOf("juegos_demo", "eco_prueba", "difusion_demo")) {
            regla.esperaEtiqueta("modulo_interruptor_$id")
            regla.onNodeWithTag("modulo_interruptor_$id", useUnmergedTree = true).performScrollTo().performClick()
            regla.espera { llamo("activarModulo:$id") }
            // el interruptor se enciende cuando termina de recargarse la lista: se espera en vez de mirar al instante
            regla.espera { runCatching { regla.onNodeWithTag("modulo_interruptor_$id", useUnmergedTree = true).assertIsOn() }.isSuccess }
        }
        assertFalse(hay("aviso_entiendo"))
    }

    @Test fun apagarEsInmediatoSinAviso() {
        motor.moduloPorIdParaPruebas("eco_prueba").put("activo", true)
        lista()
        regla.esperaEtiqueta("modulo_interruptor_eco_prueba")
        regla.onNodeWithTag("modulo_interruptor_eco_prueba", useUnmergedTree = true).performScrollTo().assertIsOn().performClick()
        regla.espera { llamo("desactivarModulo:eco_prueba") }
    }

    @Test fun unModuloEncendidoConElMaestroApagadoLoDice() {
        motor.moduloPorIdParaPruebas("eco_prueba").put("activo", true)
        lista()
        regla.esperaTexto("Encendido, pero la Iniciativa propia está apagada")
    }

    @Test fun unModuloPausadoPorModoSeguroSeVeComoTal() {
        motor.moduloPorIdParaPruebas("eco_prueba").put("activo", true)
        motor.seguro = """{"activo":true,"ts":5,"motivo":"errores","texto":"fallaron varios envíos seguidos","errores":5}"""
        lista()
        regla.esperaTexto("Pausado por modo seguro")
        regla.onNodeWithTag("modulo_interruptor_eco_prueba", useUnmergedTree = true).performScrollTo().assertIsOn()
        regla.onNodeWithText("Modo seguro activo", substring = true, useUnmergedTree = true).assertIsDisplayed()
    }

    // ---------- Iniciativa propia ----------

    @Test fun elInterruptorMaestroNaceApagadoYSeEnciendeYApaga() {
        regla.setContent { BotChatTema { PantallaIniciativa(bot, {}) } }
        regla.esperaEtiqueta("iniciativa_maestro")
        regla.onNodeWithTag("iniciativa_maestro", useUnmergedTree = true).assertIsOff()
        regla.onNodeWithText("Apagada: nada se envía por iniciativa propia.", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("iniciativa_maestro", useUnmergedTree = true).performClick()
        regla.espera { llamo("maestro:true") }
        regla.esperaTexto("Encendida", substring = true)
        regla.onNodeWithTag("iniciativa_maestro", useUnmergedTree = true).assertIsOn().performClick()
        regla.espera { llamo("maestro:false") }
        regla.esperaTexto("Apagada: nada se envía", substring = true)
    }

    @Test fun elModoSeguroActivoSeExplicaYSeReanuda() {
        motor.seguro = """{"activo":true,"ts":5,"motivo":"desconexion:sesion_cerrada","texto":"WhatsApp cortó la conexión (sesion_cerrada)","errores":0}"""
        regla.setContent { BotChatTema { PantallaIniciativa(bot, {}) } }
        regla.esperaTexto("Modo seguro activado")
        regla.onNodeWithTag("seguro_texto", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("reanudar_seguro", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("reanudarSeguro") }
        regla.esperaTexto("Modo seguro: todo en orden")
        assertFalse(hay("reanudar_seguro"))
    }

    @Test fun sinModoSeguroNoHayBotonReanudar() {
        regla.setContent { BotChatTema { PantallaIniciativa(bot, {}) } }
        regla.esperaTexto("Modo seguro: todo en orden")
        assertFalse(hay("reanudar_seguro"))
    }

    @Test fun topesYPausaNocturnaSeGuardanConSuValidacion() {
        regla.setContent { BotChatTema { PantallaIniciativa(bot, {}) } }
        regla.esperaEtiqueta("tope_hora")
        regla.onNodeWithTag("tope_hora", useUnmergedTree = true).performScrollTo().performTextClearance()
        regla.onNodeWithTag("tope_hora", useUnmergedTree = true).performTextInput("4")
        regla.onNodeWithTag("tope_dia", useUnmergedTree = true).performTextClearance()
        regla.onNodeWithTag("tope_dia", useUnmergedTree = true).performTextInput("30")
        regla.onNodeWithTag("guardar_topes", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("ajusteIni:tope_hora:4") && llamo("ajusteIni:tope_dia:30") }
        // Pausa nocturna: se apaga y se encienden sus horas.
        regla.onNodeWithTag("noche_on", useUnmergedTree = true).performScrollTo().assertIsOn()
        regla.onNodeWithTag("noche_desde", useUnmergedTree = true).performScrollTo().performTextClearance()
        regla.onNodeWithTag("noche_desde", useUnmergedTree = true).performTextInput("22:30")
        regla.onNodeWithTag("guardar_noche", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("ajusteIni:noche_desde:22:30") }
        regla.onNodeWithTag("noche_on", useUnmergedTree = true).performClick()
        regla.espera { llamo("ajusteIni:noche_on:0") }
        regla.esperaSinEtiqueta("noche_desde")
    }

    @Test fun unTopeRechazadoPorElMotorNoSeQuedaEnSilencio() {
        motor.moduloFalla = "el valor debe estar entre 1 y 1000"
        regla.setContent { BotChatTema { PantallaIniciativa(bot, {}) } }
        regla.esperaEtiqueta("tope_hora")
        regla.onNodeWithTag("tope_hora", useUnmergedTree = true).performScrollTo().performTextClearance()
        regla.onNodeWithTag("tope_hora", useUnmergedTree = true).performTextInput("0")
        regla.onNodeWithTag("guardar_topes", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("ajusteIni:tope_hora:0") }
    }

    @Test fun noMolestarMuestraLaListaPermiteAgregarYQuitar() {
        motor.noMolestar = Apoyo.arreglo(JSONObject().put("persona", "5215511111111@s.whatsapp.net").put("desdeTs", System.currentTimeMillis() / 1000).put("motivo", "baja").put("confirmado", true))
        regla.setContent { BotChatTema { PantallaIniciativa(bot, {}) } }
        regla.esperaEtiqueta("nm_5215511111111@s.whatsapp.net")
        regla.onNodeWithText("+5215511111111", useUnmergedTree = true).performScrollTo().assertIsDisplayed()
        regla.onNodeWithText("pidió la baja", substring = true, useUnmergedTree = true).assertIsDisplayed()
        // Agregar un número a mano
        regla.onNodeWithTag("nm_agregar", useUnmergedTree = true).performScrollTo().assertIsNotEnabled()
        regla.onNodeWithTag("nm_numero", useUnmergedTree = true).performScrollTo().performTextInput("5215522222222")
        regla.onNodeWithTag("nm_agregar", useUnmergedTree = true).assertIsEnabled().performClick()
        regla.espera { llamo("agregarNM:5215522222222@s.whatsapp.net") }
        regla.esperaEtiqueta("nm_5215522222222@s.whatsapp.net")
        // Quitar
        regla.onNodeWithTag("quitar_nm_5215511111111@s.whatsapp.net", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("quitarNM:5215511111111@s.whatsapp.net") }
        regla.esperaSinEtiqueta("nm_5215511111111@s.whatsapp.net")
    }

    @Test fun elRegistroDeIniciativasNoMuestraTextosNiNumerosCompletos() {
        motor.registroIni = Apoyo.arreglo(
            JSONObject().put("id", 3).put("modulo", "eco_prueba").put("que", "bienvenida").put("ts", System.currentTimeMillis() / 1000).put("resultado", "enviado").put("chat", "…1111"),
            JSONObject().put("id", 2).put("modulo", "eco_prueba").put("que", "bienvenida").put("ts", System.currentTimeMillis() / 1000).put("resultado", "descartado:maestro").put("chat", "…2222"),
            JSONObject().put("id", 1).put("modulo", "sistema").put("que", "modo_seguro").put("ts", System.currentTimeMillis() / 1000).put("resultado", "errores").put("chat", ""),
        )
        regla.setContent { BotChatTema { PantallaRegistroIniciativas(bot, {}) } }
        regla.esperaEtiqueta("registro_3")
        regla.esperaTexto("Se envió", substring = true)
        regla.esperaTexto("No se envió: Iniciativa propia estaba apagada", substring = true)
        regla.esperaTexto("Modo seguro")
        regla.esperaTexto("Sin textos de mensajes", substring = true)
        assertTrue(regla.onAllNodesWithTextSafe("…1111", substring = true) > 0)
        assertEquals(0, regla.onAllNodesWithTextSafe("5215511111111", substring = true))
    }

    @Test fun elRegistroVacioDiceQueNoHayMovimientos() {
        regla.setContent { BotChatTema { PantallaRegistroIniciativas(bot, {}) } }
        regla.esperaTexto("Sin movimientos")
    }

    @Test fun iniciativaPropiaMuestraLosUltimosMovimientos() {
        motor.registroIni = Apoyo.arreglo(JSONObject().put("id", 9).put("modulo", "eco_prueba").put("que", "aviso").put("ts", System.currentTimeMillis() / 1000).put("resultado", "enviado").put("chat", "…1111"))
        var verTodo = false
        regla.setContent { BotChatTema { PantallaIniciativa(bot, {}, alRegistro = { verTodo = true }) } }
        regla.esperaEtiqueta("registro_9")
        regla.onNodeWithTag("ver_registro", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { verTodo }
    }

    // ---------- ventana de configuración ----------

    @Test fun lasVentanasGenericasDibujanLosCincoTiposDeCampo() {
        regla.setContent { BotChatTema { PantallaConfigModulo(bot, "eco_prueba", {}) } }
        regla.esperaEtiqueta("config_generica")
        for (campo in listOf("frase", "veces", "gritar", "estilo", "hora_aviso")) assertTrue("falta el campo $campo", hay("campo_$campo"))
        assertTrue(hay("in_frase") && hay("in_veces") && hay("sw_gritar") && hay("op_estilo_normal") && hay("op_estilo_corto") && hay("in_hora_aviso"))
        regla.onNodeWithTag("sw_gritar", useUnmergedTree = true).assertIsOff()
        regla.onNodeWithText("Entre 1 y 5", substring = true, useUnmergedTree = true).assertIsDisplayed()
        // Como escribe por iniciativa, muestra sus límites propios.
        assertTrue(hay("limites_modulo"))
    }

    @Test fun lasVentanasGenericasGuardanConLosTiposCorrectos() {
        regla.setContent { BotChatTema { PantallaConfigModulo(bot, "eco_prueba", {}) } }
        regla.esperaEtiqueta("in_frase")
        regla.onNodeWithTag("in_frase", useUnmergedTree = true).performTextClearance()
        regla.onNodeWithTag("in_frase", useUnmergedTree = true).performTextInput("hola mundo")
        regla.onNodeWithTag("in_veces", useUnmergedTree = true).performTextClearance()
        regla.onNodeWithTag("in_veces", useUnmergedTree = true).performTextInput("3")
        regla.onNodeWithTag("sw_gritar", useUnmergedTree = true).performClick()
        regla.onNodeWithTag("op_estilo_corto", useUnmergedTree = true).performClick()
        regla.onNodeWithTag("in_hora_aviso", useUnmergedTree = true).performTextClearance()
        regla.onNodeWithTag("in_hora_aviso", useUnmergedTree = true).performTextInput("08:30")
        regla.onNodeWithTag("config_guardar", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("guardarConfigModulo:eco_prueba:") }
        val json = JSONObject(motor.llamadas.first { it.startsWith("guardarConfigModulo:eco_prueba:") }.removePrefix("guardarConfigModulo:eco_prueba:"))
        assertEquals("hola mundo", json.getString("frase"))
        assertEquals(3.0, json.getDouble("veces"), 0.0)
        assertTrue(json.get("veces") is Number)
        assertEquals(true, json.getBoolean("gritar"))
        assertEquals("corto", json.getString("estilo"))
        assertEquals("08:30", json.getString("hora_aviso"))
        regla.esperaEtiqueta("config_guardado")
    }

    @Test fun unNumeroQueNoEsNumeroSeAvisaSinLlamarAlMotor() {
        regla.setContent { BotChatTema { PantallaConfigModulo(bot, "eco_prueba", {}) } }
        regla.esperaEtiqueta("in_veces")
        regla.onNodeWithTag("in_veces", useUnmergedTree = true).performTextClearance()
        regla.onNodeWithTag("in_veces", useUnmergedTree = true).performTextInput("abc")
        regla.onNodeWithTag("config_guardar", useUnmergedTree = true).performScrollTo().performClick()
        regla.esperaTexto("«Veces» debe ser un número")
        assertFalse(llamo("guardarConfigModulo"))
    }

    @Test fun elErrorDeValidacionDeGoSeMuestraTalCual() {
        motor.moduloFalla = "«Veces» debe estar entre 1 y 5"
        regla.setContent { BotChatTema { PantallaConfigModulo(bot, "eco_prueba", {}) } }
        regla.esperaEtiqueta("in_veces")
        regla.onNodeWithTag("in_veces", useUnmergedTree = true).performTextClearance()
        regla.onNodeWithTag("in_veces", useUnmergedTree = true).performTextInput("9")
        regla.onNodeWithTag("config_guardar", useUnmergedTree = true).performScrollTo().performClick()
        regla.esperaTexto("«Veces» debe estar entre 1 y 5")
        assertFalse(hay("config_guardado"))
    }

    @Test fun losLimitesPropiosDelModuloSeGuardan() {
        regla.setContent { BotChatTema { PantallaConfigModulo(bot, "eco_prueba", {}) } }
        regla.esperaEtiqueta("tope_modulo_hora")
        regla.onNodeWithTag("tope_modulo_hora", useUnmergedTree = true).performScrollTo().performTextClearance()
        regla.onNodeWithTag("tope_modulo_hora", useUnmergedTree = true).performTextInput("2")
        regla.onNodeWithTag("tope_modulo_dia", useUnmergedTree = true).performTextClearance()
        regla.onNodeWithTag("tope_modulo_dia", useUnmergedTree = true).performTextInput("9")
        regla.onNodeWithTag("guardar_limites_modulo", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { llamo("topesModulo:eco_prueba:2:9") }
        regla.esperaTexto("Guardado")
    }

    @Test fun unModuloSinAjustesNiPantallaPropiaLoDice() {
        regla.setContent { BotChatTema { PantallaConfigModulo(bot, "juegos_demo", {}) } }
        regla.esperaEtiqueta("config_sin_ajustes")
        assertFalse(hay("limites_modulo")) // no escribe por iniciativa
    }

    @Test fun unModuloPuedeTraerSuPropiaVentanaYReemplazaALaGenerica() {
        PantallasModulo.registrar("propio_demo") { _, info -> Text("Ventana propia de ${info.nombre}") }
        assertTrue(PantallasModulo.tiene("propio_demo"))
        regla.setContent { BotChatTema { PantallaConfigModulo(bot, "propio_demo", {}) } }
        regla.esperaTexto("Ventana propia de Con ventana propia")
        assertTrue(hay("config_propia"))
        assertFalse(hay("config_generica"))
        assertFalse(hay("config_sin_ajustes"))
        PantallasModulo.quitar("propio_demo")
        assertFalse(PantallasModulo.tiene("propio_demo"))
    }

    // ---------- navegación desde Ajustes ----------

    @Test fun ajustesTraeLasTarjetasDeModulosEIniciativaPropia() {
        var modulos = false
        var iniciativa = false
        regla.setContent {
            BotChatTema { PantallaAjustes(Ajustes.get(Apoyo.ctx()), {}, {}, {}, {}, alModulos = { modulos = true }, alIniciativa = { iniciativa = true }) }
        }
        regla.onNodeWithTag("ajuste_modulos").performScrollTo().performClick()
        regla.onNodeWithTag("ajuste_iniciativa").performScrollTo().performClick()
        regla.espera { modulos && iniciativa }
        regla.onNodeWithText("Módulos").assertIsDisplayed()
    }

    private fun androidx.compose.ui.test.junit4.ComposeContentTestRule.esperaSinEtiqueta(tag: String) =
        espera { onAllNodes(androidx.compose.ui.test.hasTestTag(tag), useUnmergedTree = true).fetchSemanticsNodes().isEmpty() }

    // ---------- módulos bloqueados (por ejemplo, sin su motor descargado) ----------

    @Test fun unModuloBloqueadoNoDejaEncenderseYDiceQueFalta() {
        motor.modulosLista[0].put("bloqueo", "Falta descargar el motor.")
        lista()
        regla.esperaEtiqueta("modulo_bloqueo_eco_prueba")
        regla.onNodeWithText("Falta descargar el motor.", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("modulo_interruptor_eco_prueba", useUnmergedTree = true).assertIsNotEnabled()
        assertFalse(llamo("activarModulo"))
    }

    @Test fun unModuloSinBloqueoSePuedeEncender() {
        lista()
        regla.esperaEtiqueta("modulo_eco_prueba")
        assertFalse(hay("modulo_bloqueo_eco_prueba"))
        regla.onNodeWithTag("modulo_interruptor_eco_prueba", useUnmergedTree = true).assertIsEnabled()
    }
}
