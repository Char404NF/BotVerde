package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsOff
import androidx.compose.ui.test.assertIsOn
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.performTextReplacement
import com.botchat.app.datos.AlcanceDef
import com.botchat.app.datos.AutoDef
import com.botchat.app.datos.DispDef
import com.botchat.app.datos.OpcionDef
import com.botchat.app.datos.PasoDef
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.EditorAutomatizacion
import com.botchat.app.ui.PantallaAutomatizaciones
import com.botchat.app.ui.PantallaChat
import com.botchat.app.ui.PantallaEnCurso
import com.botchat.app.ui.PantallaEstadisticasMenu
import com.botchat.app.ui.PantallaRecorrido
import com.botchat.app.ui.PantallaSimulador
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
class AutomatizacionesUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L
    private val ahora = System.currentTimeMillis() / 1000

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()

    private fun ventas() = AutoDef(
        "Ventas", "m",
        listOf(
            PasoDef("m", "menu", "¡Hola {nombre}! ¿Qué buscas?", listOf(OpcionDef("Precios", "p"), OpcionDef("Hablar con alguien", "h"))),
            PasoDef("p", "respuesta", texto = "El plan básico cuesta $100."),
            PasoDef("h", "humano", texto = "Te paso con una persona."),
            PasoDef("s", "fin", texto = "Nadie llega aquí"),
        ),
        DispDef(palabras = listOf("ventas"), comando = ".menu"), AlcanceDef(privados = true),
    )

    private fun detalle(d: AutoDef, activo: Boolean = true, sistema: Boolean = false) =
        JSONObject().put("id", 1).put("activo", activo).put("sistema", sistema).put("version", 1).put("def", JSONObject(d.json())).put("avisos", JSONArray()).toString()

    private fun resumen(id: Int, nombre: String, activo: Boolean = true, avisos: Int = 0, sistema: Boolean = false) =
        JSONObject().put("id", id).put("activo", activo).put("sistema", sistema).put("version", 1).put("nombre", nombre).put("resumen", "Palabras clave").put("avisos", avisos).put("pasos", 3)

    // ---------- lista ----------

    private fun lista(editar: (Long) -> Unit = {}) = regla.setContent { BotChatTema { PantallaAutomatizaciones(bot, {}, editar, {}, {}, {}, {}) } }

    @Test fun lasTarjetasMuestranNombreDisparadorInterruptorYAvisos() {
        motor.autos = Apoyo.arreglo(resumen(1, "Ventas"), resumen(2, "Rota", activo = false, avisos = 2), resumen(3, "Saludo", sistema = true))
        lista()
        regla.esperaTexto("Ventas")
        regla.onNodeWithTag("activa_1").assertIsOn()
        regla.onNodeWithTag("activa_2").assertIsOff()
        regla.onNodeWithText("2 avisos por corregir", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithText("Palabras clave · 3 pasos · De fábrica", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithText("El bot solo responde", substring = true).assertIsDisplayed()
    }

    @Test fun encenderYApagarUnaTarjetaAvisaAlMotor() {
        motor.autos = Apoyo.arreglo(resumen(1, "Ventas", activo = false))
        lista()
        regla.esperaEtiqueta("activa_1")
        regla.onNodeWithTag("activa_1").performClick()
        regla.espera { llamo("activarAuto:1:true") }
    }

    @Test fun duplicarYBorrarConConfirmacion() {
        motor.autos = Apoyo.arreglo(resumen(1, "Ventas"))
        lista()
        regla.esperaEtiqueta("mas_1")
        regla.onNodeWithTag("mas_1").performClick()
        regla.esperaEtiqueta("duplicar_1")
        regla.onNodeWithTag("duplicar_1").performClick()
        regla.espera { llamo("duplicarAuto:1") }
        regla.onNodeWithTag("mas_1").performClick()
        regla.esperaEtiqueta("borrar_1")
        regla.onNodeWithTag("borrar_1").performClick()
        regla.esperaEtiqueta("confirmar_borrar_auto")
        assertFalse(llamo("borrarAuto"))
        regla.onNodeWithTag("confirmar_borrar_auto").performClick()
        regla.espera { llamo("borrarAuto:1") }
    }

    @Test fun nuevaDesdeCeroYDesdePlantillaCreanYAbrenElEditor() {
        motor.autos = "[]"
        motor.plantillas = JSONArray().put(JSONObject().put("nombre", "Atención básica").put("descripcion", "Menú de bienvenida").put("def", JSONObject(ventas().json()))).toString()
        var abierto = 0L
        lista { abierto = it }
        regla.esperaEtiqueta("nueva_automatizacion")
        regla.onNodeWithTag("nueva_automatizacion").performClick()
        regla.esperaEtiqueta("panel_nueva")
        regla.onNodeWithTag("nueva_desde_cero").performClick()
        regla.espera { abierto == 1L && llamo("guardarAuto:0:") }
        abierto = 0
        regla.onNodeWithTag("nueva_automatizacion").performClick()
        regla.esperaEtiqueta("nueva_desde_plantilla")
        regla.onNodeWithTag("nueva_desde_plantilla").performClick()
        regla.esperaEtiqueta("plantilla_Atención básica")
        regla.onNodeWithTag("plantilla_Atención básica").performClick()
        regla.espera { abierto == 1L && motor.llamadas.count { it.startsWith("guardarAuto:0:") } == 2 }
        assertTrue(motor.llamadas.any { it.contains("¡Hola {nombre}! ¿Qué buscas?") })
    }

    // ---------- editor ----------

    private fun editor(volver: () -> Unit = {}, probar: (Long) -> Unit = {}) {
        motor.autoDetalle = detalle(ventas())
        regla.setContent { BotChatTema { EditorAutomatizacion(bot, 1, volver, probar) } }
    }

    @Test fun elEditorMuestraLosPasosConSangriaYLosSueltosAparte() {
        editor()
        regla.esperaEtiqueta("pestana_pasos")
        regla.esperaEtiqueta("paso_m")
        for (id in listOf("m", "p", "h", "s")) regla.onNodeWithTag("paso_$id", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithText("Pasos sueltos (nadie llega a ellos todavía)").assertIsDisplayed()
        regla.onNodeWithText("1. Precios → ", substring = true, useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithText("Menú · Inicio", useUnmergedTree = true).assertIsDisplayed()
    }

    @Test fun lasAdvertenciasDeValidacionSeVen() {
        motor.autoValidacion = """[{"paso":"m","texto":"La opción 2 (Hablar con alguien) no lleva a ningún paso.","grave":true},{"paso":"s","texto":"Ningún paso lleva a este (queda suelto).","grave":false}]"""
        editor()
        regla.esperaEtiqueta("avisos_validacion")
        regla.espera { hay("aviso_validacion") }
        regla.onNodeWithText("La opción 2", substring = true, useUnmergedTree = true).assertIsDisplayed()
        assertEquals(2, regla.onAllNodesWithTag("aviso_validacion", useUnmergedTree = true).fetchSemanticsNodes().size)
    }

    @Test fun editarUnTextoActivaGuardarYManda() {
        editor()
        regla.esperaEtiqueta("abrir_p")
        assertFalse("sin cambios no hay Guardar", hay("guardar_auto"))
        regla.onNodeWithTag("abrir_p").performClick()
        regla.esperaEtiqueta("texto_p")
        regla.onNodeWithTag("texto_p").performTextReplacement("El plan básico ahora cuesta 120.")
        regla.esperaEtiqueta("guardar_auto")
        regla.onNodeWithTag("guardar_auto").performClick()
        regla.espera { motor.llamadas.any { it.startsWith("guardarAuto:1:") && it.contains("El plan básico ahora cuesta 120.") } }
        regla.espera { !hay("guardar_auto") && hay("probar_auto") }
    }

    @Test fun agregarOpcionesYPasosNuevosYBorrarPasos() {
        editor()
        regla.esperaEtiqueta("abrir_m")
        regla.onNodeWithTag("abrir_m").performClick()
        regla.esperaEtiqueta("agregar_opcion_m")
        regla.onNodeWithTag("agregar_opcion_m").performClick()
        regla.esperaEtiqueta("opcion_m_2")
        regla.onNodeWithTag("destino_m_2").performClick()
        regla.esperaEtiqueta("destino_m_2_nuevo_respuesta")
        regla.onNodeWithTag("destino_m_2_nuevo_respuesta").performClick()
        regla.espera { hay("paso_p3") || hay("paso_p5") || motor.llamadas.isEmpty() }
        regla.esperaEtiqueta("guardar_auto")
        regla.onNodeWithTag("guardar_auto").performClick()
        regla.espera { motor.llamadas.any { it.startsWith("guardarAuto:1:") && it.contains("\"opciones\"") } }
        val guardado = JSONObject(motor.llamadas.first { it.startsWith("guardarAuto:1:") }.removePrefix("guardarAuto:1:"))
        val menu = guardado.getJSONArray("pasos").getJSONObject(0)
        assertEquals(3, menu.getJSONArray("opciones").length())
        assertTrue("la opción nueva lleva al paso nuevo", menu.getJSONArray("opciones").getJSONObject(2).getString("destino").isNotEmpty())
        assertEquals(5, guardado.getJSONArray("pasos").length())
    }

    @Test fun borrarUnPasoLimpiaLasOpcionesQueLoUsaban() {
        editor()
        regla.esperaEtiqueta("abrir_p")
        regla.onNodeWithTag("abrir_p").performClick()
        regla.esperaEtiqueta("borrar_paso_p")
        regla.onNodeWithTag("borrar_paso_p").performClick()
        regla.esperaEtiqueta("guardar_auto")
        regla.onNodeWithTag("guardar_auto").performClick()
        regla.espera { llamo("guardarAuto:1:") }
        val guardado = JSONObject(motor.llamadas.first { it.startsWith("guardarAuto:1:") }.removePrefix("guardarAuto:1:"))
        assertEquals(3, guardado.getJSONArray("pasos").length())
        assertEquals("", guardado.getJSONArray("pasos").getJSONObject(0).getJSONArray("opciones").getJSONObject(0).getString("destino"))
    }

    @Test fun cuandoYDondeEditaDisparadoresAlcanceYHorario() {
        editor()
        regla.esperaEtiqueta("pestana_1")
        regla.onNodeWithTag("pestana_1").performClick()
        regla.esperaEtiqueta("pestana_cuando")
        regla.onNodeWithTag("disp_primero").performClick()
        regla.onNodeWithTag("disp_comando").performTextReplacement(".ayuda")
        regla.onNodeWithTag("dia_1").performClick()
        regla.onNodeWithTag("horario_desde").performTextInput("09:00")
        regla.onNodeWithTag("horario_hasta").performTextInput("18:00")
        regla.esperaEtiqueta("guardar_auto")
        regla.onNodeWithTag("guardar_auto").performClick()
        regla.espera { llamo("guardarAuto:1:") }
        val g = JSONObject(motor.llamadas.first { it.startsWith("guardarAuto:1:") }.removePrefix("guardarAuto:1:"))
        assertTrue(g.getJSONObject("disparadores").getBoolean("primerMensaje"))
        assertEquals(".ayuda", g.getJSONObject("disparadores").getString("comando"))
        assertEquals("09:00", g.getJSONObject("horario").getString("desde"))
        assertEquals(1, g.getJSONObject("horario").getJSONArray("dias").getInt(0))
        assertTrue(g.getJSONObject("alcance").getBoolean("privados"))
    }

    @Test fun laVistaPreviaSiguelaConversacionEnBurbujas() {
        editor()
        regla.esperaEtiqueta("pestana_2")
        regla.onNodeWithTag("pestana_2").performClick()
        regla.esperaEtiqueta("pestana_previa")
        regla.esperaTexto("¡Hola María! ¿Qué buscas?", substring = true)
        regla.onNodeWithTag("previa_opcion_0").performClick()
        regla.esperaTexto("El plan básico cuesta $100.")
        regla.esperaTexto("Fin de la conversación.")
        regla.onNodeWithTag("previa_reiniciar").performClick()
        regla.esperaEtiqueta("previa_opcion_1")
        regla.onNodeWithTag("previa_opcion_1").performClick()
        regla.esperaTexto("Te paso con una persona.")
        regla.esperaTexto("La conversación pasa a una persona.")
    }

    @Test fun saliendoConCambiosPideConfirmar() {
        var volvio = false
        editor(volver = { volvio = true })
        regla.esperaEtiqueta("campo_nombre_auto")
        regla.onNodeWithTag("campo_nombre_auto").performTextReplacement("Otro nombre")
        regla.onNodeWithTag("volver_editor").performClick()
        regla.esperaEtiqueta("salir_sin_guardar")
        assertFalse(volvio)
        regla.onNodeWithTag("salir_sin_guardar").performClick()
        regla.espera { volvio }
    }

    // ---------- en curso ----------

    @Test fun enCursoListaQuienEstaDondeYHaceCuanto() {
        motor.enCurso = Apoyo.arreglo(
            JSONObject().put("id", 7).put("chat", "5@s.whatsapp.net").put("sender", "5@s.whatsapp.net").put("menuId", 1).put("paso", "p").put("estado", "activo").put("ultima", ahora - 180).put("inicio", ahora - 400).put("nombre", "Ana").put("menu", "Ventas").put("ruta", "Ventas › Precios"),
            JSONObject().put("id", 8).put("chat", "6@s.whatsapp.net").put("sender", "6@s.whatsapp.net").put("menuId", 1).put("paso", "").put("estado", "humano").put("ultima", ahora - 30).put("inicio", ahora - 90).put("nombre", "Beto").put("menu", "Ventas").put("ruta", "Ventas"),
        )
        var abierto = 0L
        regla.setContent { BotChatTema { PantallaEnCurso(bot, {}, { abierto = it }) } }
        regla.esperaTexto("Ventas › Precios")
        regla.onNodeWithText("hace 3 min").assertIsDisplayed()
        regla.onNodeWithText("Esperando a una persona").assertIsDisplayed()
        regla.onNodeWithTag("sesion_7").performClick()
        regla.espera { abierto == 7L }
    }

    @Test fun elRecorridoTieneLosTresBotonesYNingunoEnviaMensajes() {
        motor.recorrido = JSONObject().put("sesion", JSONObject().put("id", 7).put("chat", "5@s").put("sender", "5@s").put("menuId", 1).put("paso", "p").put("estado", "activo").put("ultima", ahora).put("inicio", ahora - 60))
            .put("menu", "Ventas").put("persona", "Ana").put("pasos", JSONArray()
                .put(JSONObject().put("ts", ahora - 50).put("titulo", "¡Hola! ¿Qué buscas?").put("opcion", "").put("resultado", "mostrado"))
                .put(JSONObject().put("ts", ahora - 40).put("titulo", "¡Hola! ¿Qué buscas?").put("opcion", "1").put("resultado", "valida"))).toString()
        regla.setContent { BotChatTema { PantallaRecorrido(bot, 7, {}) } }
        // El recorrido se ve como una conversación: el menú del bot, y la elección «1» citando ese menú.
        regla.esperaEtiqueta("burbuja_bot")
        regla.onNodeWithTag("burbuja_bot").assertIsDisplayed()
        regla.onNodeWithTag("burbuja_usuario").assertIsDisplayed()
        regla.onNodeWithTag("burbuja_cita").assertIsDisplayed()
        regla.onNodeWithText("1").assertIsDisplayed()
        assertEquals(2, regla.onAllNodesWithTextSafe("¡Hola! ¿Qué buscas?")) // la burbuja del bot y la cita
        regla.onNodeWithTag("reiniciar_menu").performClick()
        regla.espera { llamo("accionSesion:7:reiniciar") }
        regla.onNodeWithTag("pasar_humano").performClick()
        regla.espera { llamo("accionSesion:7:humano") }
        regla.onNodeWithTag("pausar_persona").performClick()
        regla.espera { llamo("accionSesion:7:pausar") }
        assertFalse("estos botones nunca mandan mensajes", llamo("enviar:") || llamo("medio:"))
    }

    // ---------- estadísticas y simulador ----------

    @Test fun lasEstadisticasDelMenuMuestranContadoresBarrasYPruebas() {
        motor.estMenu = """{"iniciados":10,"completados":6,"abandonados":3,"humano":1,"expirados":2,
            "opciones":[{"paso":"m","titulo":"¿Qué buscas?","valor":7,"opcion":"1","texto":"Precios"},{"paso":"m","titulo":"¿Qué buscas?","valor":3,"opcion":"2","texto":"Contacto"}],
            "embudo":[{"paso":"m","titulo":"¿Qué buscas?","valor":10,"opcion":"","texto":""},{"paso":"p","titulo":"Precios","valor":7,"opcion":"","texto":""}],
            "invalidas":[{"paso":"m","titulo":"¿Qué buscas?","valor":4,"opcion":"","texto":""}]}"""
        regla.setContent { BotChatTema { PantallaEstadisticasMenu(bot, 1, "Ventas", {}) } }
        regla.esperaEtiqueta("est_iniciados")
        regla.onNodeWithText("Opciones elegidas").assertIsDisplayed()
        regla.onNodeWithText("Embudo: quién llegó a cada paso").assertIsDisplayed()
        regla.onNodeWithText("Dónde más se equivocan").assertIsDisplayed()
        regla.onNodeWithText("¿Qué buscas? → 1 (Precios)").assertIsDisplayed()
        assertTrue(llamo("estMenu:false"))
        regla.onNodeWithTag("incluir_pruebas").performClick()
        regla.espera { llamo("estMenu:true") }
        regla.onNodeWithTag("rango_7 días").performClick()
        regla.onNodeWithTag("rango_Todo").performClick()
    }

    @Test fun sinDatosLasEstadisticasLoDicen() {
        regla.setContent { BotChatTema { PantallaEstadisticasMenu(bot, 1, "Ventas", {}) } }
        regla.esperaTexto("Sin datos todavía")
    }

    @Test fun elSimuladorMuestraLasRespuestasYAvisaQueNoSeEnviaNada() {
        motor.simulacion = mutableListOf("¡Hola María! ¿Qué buscas?")
        regla.setContent { BotChatTema { PantallaSimulador(bot, 1, "Ventas", {}) } }
        regla.esperaTexto("Modo prueba · no se envía nada")
        regla.onNodeWithTag("campo_simulador").performTextInput("quiero ventas")
        regla.onNodeWithTag("enviar_simulador").performClick()
        regla.esperaTexto("¡Hola María! ¿Qué buscas?")
        regla.onNodeWithText("quiero ventas").assertIsDisplayed()
        regla.esperaEtiqueta("nota_simulador")
        regla.onNodeWithText("Este mensaje sí activaría esta automatización.").assertIsDisplayed()
        assertTrue(llamo("simular:quiero ventas:true"))
        assertFalse(llamo("enviar:"))
        regla.onNodeWithTag("campo_simulador").performTextInput("1")
        regla.onNodeWithTag("enviar_simulador").performClick()
        regla.espera { llamo("simular:1:false") }
    }

    // ---------- etiqueta en el chat ----------

    @Test fun elChatMuestraEnQueMenuEstaLaPersona() {
        val jid = "5@s.whatsapp.net"
        motor.chat = Apoyo.chatJson(jid, "Ana").toString()
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola", remitente = "Ana"))
        motor.etiqueta = """[{"nombre":"Ana","ruta":"Ventas › Precios","ultima":${ahora - 180},"estado":"activo"}]"""
        regla.setContent { BotChatTema { PantallaChat(jid, {}, {}, {}, {}) } }
        regla.esperaEtiqueta("etiqueta_menu")
        regla.onNodeWithText("En menú: Ventas › Precios · hace 3 min", useUnmergedTree = true).assertIsDisplayed()
    }
}
