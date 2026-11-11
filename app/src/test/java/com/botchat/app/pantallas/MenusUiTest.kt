package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.performTextReplacement
import com.botchat.app.datos.AdjuntoDef
import com.botchat.app.datos.AlcanceDef
import com.botchat.app.datos.AutoDef
import com.botchat.app.datos.CampoDef
import com.botchat.app.datos.DispDef
import com.botchat.app.datos.OpcionDef
import com.botchat.app.datos.PasoDef
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.EditorAutomatizacion
import com.botchat.app.ui.EditorRango
import com.botchat.app.ui.PantallaAjustesMenus
import com.botchat.app.ui.PantallaAutomatizaciones
import com.botchat.app.ui.PantallaEstadisticasMenu
import com.botchat.app.ui.PantallaFormulario
import com.botchat.app.ui.PantallaFormularios
import com.botchat.app.ui.PantallaNoEntendio
import com.botchat.app.ui.PantallaSimulador
import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** Menús ampliados (etapa C): editor con formularios, archivos, ubicación, contacto, opciones avanzadas y ajustes; formularios recibidos; no entendió; ajustes generales. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class MenusUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L
    private val ahora = System.currentTimeMillis() / 1000

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()

    private fun detalle(d: AutoDef) =
        JSONObject().put("id", 1).put("activo", true).put("sistema", false).put("version", 1).put("def", JSONObject(d.json())).put("avisos", JSONArray()).toString()

    private fun tienda() = AutoDef(
        "Tienda", "m",
        listOf(
            PasoDef("m", "menu", "¡Hola! ¿Qué buscas?", listOf(OpcionDef("Precios", "p", id = "o1"), OpcionDef("Cotizar", "f", id = "o2"))),
            PasoDef("p", "respuesta", texto = "Desde $50."),
            PasoDef("f", "formulario", titulo = "Cotización", texto = "¡Gracias!", campos = listOf(CampoDef("nombre", "Nombre", "¿Cómo te llamas?", "texto"))),
        ),
        DispDef(comando = ".menu"), AlcanceDef(privados = true),
    )

    private fun editor(d: AutoDef = tienda()) {
        motor.autoDetalle = detalle(d)
        regla.setContent { BotChatTema { EditorAutomatizacion(bot, 1, {}, {}) } }
    }

    private fun guardado(): JSONObject {
        regla.esperaEtiqueta("guardar_auto")
        regla.onNodeWithTag("guardar_auto").performClick()
        regla.espera { llamo("guardarAuto:1:") }
        return JSONObject(motor.llamadas.first { it.startsWith("guardarAuto:1:") }.removePrefix("guardarAuto:1:"))
    }

    // ---------- el modelo conserva lo que la pantalla no conoce ----------

    @Test fun guardarNoPierdeCamposQueLaPantallaNoConoce() {
        val original = JSONObject(tienda().json()).put("propio", JSONObject().put("nombre", "horario")).put("futuro", 7)
        original.getJSONObject("alcance").put("todosGrupos", true)
        motor.autoDetalle = JSONObject().put("id", 1).put("activo", true).put("sistema", false).put("version", 1).put("def", original).put("avisos", JSONArray()).toString()
        regla.setContent { BotChatTema { EditorAutomatizacion(bot, 1, {}, {}) } }
        regla.esperaEtiqueta("campo_nombre_auto")
        regla.onNodeWithTag("campo_nombre_auto").performTextReplacement("Tienda 2")
        val g = guardado()
        assertEquals("horario", g.getJSONObject("propio").getString("nombre"))
        assertEquals(7, g.getInt("futuro"))
        assertTrue(g.getJSONObject("alcance").getBoolean("todosGrupos"))
        assertEquals("o1", g.getJSONArray("pasos").getJSONObject(0).getJSONArray("opciones").getJSONObject(0).getString("id"))
        assertEquals("Tienda 2", g.getString("nombre"))
    }

    // ---------- formularios en el editor ----------

    @Test fun elEditorEditaLasPreguntasDeUnFormulario() {
        editor()
        regla.esperaEtiqueta("abrir_f")
        regla.onNodeWithTag("abrir_f").performClick()
        regla.esperaEtiqueta("campo_pregunta_f_0")
        regla.onNodeWithTag("campo_pregunta_f_0").performTextReplacement("¿Cuál es tu nombre completo?")
        regla.onNodeWithTag("agregar_campo_f").performScrollTo().performClick()
        regla.esperaEtiqueta("campo_tipo_f_1_correo")
        regla.onNodeWithTag("campo_tipo_f_1_correo").performScrollTo().performClick()
        regla.onNodeWithTag("campo_pregunta_f_1").performScrollTo().performTextReplacement("¿Tu correo?")
        regla.onNodeWithTag("campo_opcional_f_1").performScrollTo().performClick()
        regla.onNodeWithTag("campo_error_f_1").performScrollTo().performTextReplacement("Escríbelo como nombre@correo.com")
        regla.onNodeWithTag("agregar_campo_f").performScrollTo().performClick()
        regla.esperaEtiqueta("campo_tipo_f_2_opcion")
        regla.onNodeWithTag("campo_tipo_f_2_opcion").performScrollTo().performClick()
        regla.esperaEtiqueta("campo_opciones_f_2")
        regla.onNodeWithTag("campo_opciones_f_2").performScrollTo().performTextReplacement("Rojo\nVerde")
        regla.onNodeWithTag("campo_tipo_f_2_numero").performScrollTo().performClick()
        regla.esperaEtiqueta("campo_min_f_2")
        regla.onNodeWithTag("campo_min_f_2").performScrollTo().performTextInput("18")
        regla.onNodeWithTag("campo_max_f_2").performScrollTo().performTextInput("99")
        regla.onNodeWithTag("form_resumen_f").performScrollTo().performClick()
        val g = guardado()
        val f = g.getJSONArray("pasos").getJSONObject(2)
        assertEquals("formulario", f.getString("tipo"))
        assertTrue(f.getBoolean("sinResumen"))
        val cs = f.getJSONArray("campos")
        assertEquals(3, cs.length())
        assertEquals("¿Cuál es tu nombre completo?", cs.getJSONObject(0).getString("pregunta"))
        assertEquals("correo", cs.getJSONObject(1).getString("tipo"))
        assertTrue(cs.getJSONObject(1).getBoolean("opcional"))
        assertEquals("Escríbelo como nombre@correo.com", cs.getJSONObject(1).getString("error"))
        assertEquals("numero", cs.getJSONObject(2).getString("tipo"))
        assertEquals(18.0, cs.getJSONObject(2).getDouble("min"), 0.0)
        assertEquals(99.0, cs.getJSONObject(2).getDouble("max"), 0.0)
    }

    @Test fun sePuedeAgregarUnPasoDeFormularioYMoverOQuitarPreguntas() {
        editor()
        regla.esperaEtiqueta("agregar_formulario")
        regla.onNodeWithTag("agregar_formulario").performScrollTo().performClick()
        regla.espera { hay("agregar_campo_p4") || hay("agregar_campo_p3") || hay("agregar_campo_p5") }
        val id = listOf("p4", "p3", "p5").first { hay("agregar_campo_$it") }
        regla.onNodeWithTag("agregar_campo_$id").performScrollTo().performClick()
        regla.esperaEtiqueta("campo_form_${id}_1")
        regla.onNodeWithTag("campo_sube_${id}_1").performScrollTo().performClick()
        regla.onNodeWithTag("campo_quita_${id}_1").performScrollTo().performClick()
        val g = guardado()
        val f = (0 until g.getJSONArray("pasos").length()).map { g.getJSONArray("pasos").getJSONObject(it) }.first { it.getString("id") == id }
        assertEquals("formulario", f.getString("tipo"))
        assertEquals(1, f.getJSONArray("campos").length())
    }

    // ---------- respuesta con archivos, ubicación y contacto ----------

    @Test fun unaRespuestaLlevaUbicacionContactoYArchivos() {
        val d = tienda().copy(pasos = tienda().pasos.map { if (it.id == "p") it.copy(adjuntos = listOf(AdjuntoDef("abc", "imagen", "image/jpeg", "lista.jpg"), AdjuntoDef("def", "documento", "application/pdf", "catalogo.pdf"))) else it })
        editor(d)
        regla.esperaEtiqueta("abrir_p")
        regla.onNodeWithTag("abrir_p").performClick()
        regla.esperaEtiqueta("adjunto_p_0")
        regla.onNodeWithText("  lista.jpg", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("quitar_adjunto_p_0").performScrollTo().performClick()
        regla.onNodeWithTag("sw_ubicacion_p").performScrollTo().performClick()
        regla.esperaEtiqueta("ubicacion_lat_p")
        regla.onNodeWithTag("ubicacion_lat_p").performTextInput("19.4326")
        regla.onNodeWithTag("ubicacion_lon_p").performTextInput("-99.1332")
        regla.onNodeWithTag("ubicacion_nombre_p").performTextInput("Sucursal Centro")
        regla.onNodeWithTag("sw_contacto_p").performScrollTo().performClick()
        regla.onNodeWithTag("contacto_nombre_p").performScrollTo().performTextInput("Ventas")
        regla.onNodeWithTag("contacto_telefono_p").performTextInput("+525512345678")
        val p = guardado().getJSONArray("pasos").getJSONObject(1)
        assertEquals(1, p.getJSONArray("adjuntos").length())
        assertEquals("catalogo.pdf", p.getJSONArray("adjuntos").getJSONObject(0).getString("nombre"))
        assertEquals(19.4326, p.getJSONObject("ubicacion").getDouble("lat"), 0.00001)
        assertEquals(-99.1332, p.getJSONObject("ubicacion").getDouble("lon"), 0.00001)
        assertEquals("Sucursal Centro", p.getJSONObject("ubicacion").getString("nombre"))
        assertEquals("+525512345678", p.getJSONObject("contacto").getString("telefono"))
    }

    // ---------- opciones con ámbito y horario ----------

    @Test fun unaOpcionPuedeSerSoloPrivadaYTenerHorario() {
        editor()
        regla.esperaEtiqueta("abrir_m")
        regla.onNodeWithTag("abrir_m").performClick()
        regla.esperaEtiqueta("avanzada_opcion_m_0")
        regla.onNodeWithTag("avanzada_opcion_m_0").performScrollTo().performClick()
        regla.esperaEtiqueta("ambito_m_0_privado")
        regla.onNodeWithTag("ambito_m_0_privado").performClick()
        regla.onNodeWithTag("opc_dia_m_0_1").performScrollTo().performClick()
        regla.onNodeWithTag("opc_desde_m_0").performScrollTo().performTextInput("09:00")
        regla.onNodeWithTag("opc_hasta_m_0").performTextInput("18:00")
        val o = guardado().getJSONArray("pasos").getJSONObject(0).getJSONArray("opciones")
        assertEquals("privado", o.getJSONObject(0).getString("ambito"))
        assertEquals("09:00", o.getJSONObject(0).getJSONObject("horario").getString("desde"))
        assertEquals(1, o.getJSONObject(0).getJSONObject("horario").getJSONArray("dias").getInt(0))
        assertFalse("la otra opción no cambia", o.getJSONObject(1).has("ambito"))
    }

    // ---------- ajustes del menú ----------

    @Test fun laPestanaAjustesGuardaEscaleraCaducidadVolverYGrupos() {
        editor()
        regla.esperaEtiqueta("pestana_3")
        regla.onNodeWithTag("pestana_3").performClick()
        regla.esperaEtiqueta("pestana_ajustes_menu")
        regla.onNodeWithTag("aj_n1").performTextInput("Uy, esa no es una opción.")
        regla.onNodeWithTag("aj_n3").performScrollTo().performTextInput("Me rindo.")
        regla.onNodeWithTag("aj_notexto").performScrollTo().performTextInput("Solo texto, por favor.")
        regla.onNodeWithTag("aj_expira").performScrollTo().performTextInput("45")
        regla.onNodeWithTag("aj_expira_grupo").performTextInput("5")
        regla.onNodeWithTag("aj_volver_retomar").performScrollTo().performClick()
        regla.onNodeWithTag("aj_humano_fuera").performScrollTo().performTextInput("Abrimos {horario}.")
        regla.onNodeWithTag("aj_max_sesiones").performScrollTo().performTextInput("3")
        regla.onNodeWithTag("aj_cualquiera_si").performScrollTo().performClick()
        val a = guardado().getJSONObject("ajustes")
        assertEquals("Uy, esa no es una opción.", a.getString("txtN1"))
        assertEquals("Me rindo.", a.getString("txtN3"))
        assertEquals("Solo texto, por favor.", a.getString("txtNoTexto"))
        assertEquals(45, a.getInt("expiraMin"))
        assertEquals(5, a.getInt("expiraGrupoMin"))
        assertEquals("retomar", a.getString("volver"))
        assertEquals("Abrimos {horario}.", a.getString("humanoFuera"))
        assertEquals(3, a.getInt("maxSesionesGrupo"))
        assertEquals("si", a.getString("cualquiera"))
    }

    @Test fun sinAjustesElMenuNoGuardaLaSeccion() {
        editor()
        regla.esperaEtiqueta("campo_nombre_auto")
        regla.onNodeWithTag("campo_nombre_auto").performTextReplacement("Otro")
        assertFalse(guardado().has("ajustes"))
    }

    @Test fun laVistaPreviaMuestraFormularioUbicacionYContacto() {
        val d = tienda().copy(pasos = tienda().pasos.map { if (it.id == "p") it.copy(ubicacion = com.botchat.app.datos.UbicacionDef(19.4, -99.1, "Tienda"), contacto = com.botchat.app.datos.ContactoDef("Ventas", "+5255")) else it })
        editor(d)
        regla.esperaEtiqueta("pestana_2")
        regla.onNodeWithTag("pestana_2").performClick()
        regla.esperaEtiqueta("previa_opcion_0")
        regla.onNodeWithTag("previa_opcion_0").performClick()
        regla.esperaTexto("[Ubicación: Tienda]")
        regla.esperaTexto("[Contacto: Ventas · +5255]")
        regla.onNodeWithTag("previa_reiniciar").performClick()
        regla.esperaEtiqueta("previa_opcion_1")
        regla.onNodeWithTag("previa_opcion_1").performClick()
        regla.esperaTexto("Revisa tus datos:", substring = true)
        regla.esperaTexto("¡Gracias!")
    }

    // ---------- formularios recibidos ----------

    private fun forms() = JSONArray()
        .put(JSONObject().put("id", 2).put("menuId", 1).put("menu", "Citas").put("cliente", "Ana Pérez").put("chat", "5215511111111@s.whatsapp.net").put("persona", "5215511111111@s.whatsapp.net").put("ts", ahora - 60)
            .put("campos", JSONArray().put(JSONObject().put("id", "n").put("etiqueta", "Nombre").put("valor", "Ana Pérez")).put(JSONObject().put("id", "c").put("etiqueta", "Correo").put("valor", "ana@correo.mx"))))
        .put(JSONObject().put("id", 1).put("menuId", 1).put("menu", "Citas").put("cliente", "Beto").put("chat", "").put("persona", "").put("ts", ahora - 3600)
            .put("campos", JSONArray().put(JSONObject().put("id", "n").put("etiqueta", "Nombre").put("valor", "Beto"))))
        .toString()

    @Test fun laListaDeFormulariosRecibidosMuestraResumenAbreExportaYBorra() {
        motor.formulariosJson = forms()
        var abierto = 0L
        regla.setContent { BotChatTema { PantallaFormularios(bot, 0, "Formularios recibidos", {}, { abierto = it }) } }
        regla.esperaEtiqueta("formulario_2")
        regla.onNodeWithText("Ana Pérez", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithText("Correo: ana@correo.mx", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithText("Beto", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("formulario_1").performClick()
        regla.espera { abierto == 1L }
        regla.onNodeWithTag("exportar_formularios").performClick()
        regla.espera { llamo("exportarFormularios:0") }
        regla.esperaEtiqueta("exporte_formularios_listo")
        regla.onNodeWithTag("cerrar_exporte_formularios").performClick()
        regla.onNodeWithTag("borrar_formularios").performClick()
        regla.esperaEtiqueta("confirmar_borrar_formularios")
        assertFalse(llamo("borrarFormularios"))
        regla.onNodeWithTag("confirmar_borrar_formularios").performClick()
        regla.espera { llamo("borrarFormularios:0") }
        regla.esperaTexto("Sin formularios todavía")
    }

    @Test fun sinFormulariosLoDiceYElDetalleMuestraTodosLosDatos() {
        regla.setContent { BotChatTema { PantallaFormularios(bot, 0, "Formularios recibidos", {}, {}) } }
        regla.esperaTexto("Sin formularios todavía")
        assertFalse(motor.llamadas.any { it.startsWith("exportarFormularios") })
    }

    @Test fun elDetalleDeUnFormularioMuestraCadaDatoYPermiteAbrirElChatYBorrar() {
        motor.formularioJson = JSONObject(forms().let { JSONArray(it).getJSONObject(0).toString() }).toString()
        var chat = ""
        var volvio = false
        regla.setContent { BotChatTema { PantallaFormulario(bot, 2, { volvio = true }, { chat = it }) } }
        regla.esperaEtiqueta("dato_n")
        assertTrue(regla.onAllNodesWithText("Ana Pérez", useUnmergedTree = true).fetchSemanticsNodes().size >= 1)
        regla.onNodeWithText("ana@correo.mx", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("abrir_chat_formulario").performClick()
        regla.espera { chat == "5215511111111@s.whatsapp.net" }
        regla.onNodeWithTag("borrar_formulario").performClick()
        regla.esperaEtiqueta("confirmar_borrar_formulario")
        regla.onNodeWithTag("confirmar_borrar_formulario").performClick()
        regla.espera { llamo("borrarFormulario:2") && volvio }
    }

    // ---------- lo que no entendió ----------

    @Test fun loQueNoEntendioSeListaYSeBorra() {
        motor.noEntendioJson = JSONArray().put(JSONObject().put("id", 1).put("menuId", 1).put("menu", "Tienda").put("paso", "m").put("ts", ahora - 120).put("texto", "quiero una pizza"))
            .put(JSONObject().put("id", 2).put("menuId", 1).put("menu", "Tienda").put("paso", "m").put("ts", ahora - 60).put("texto", "con piña")).toString()
        regla.setContent { BotChatTema { PantallaNoEntendio(bot, 0, "Lo que no entendió", {}) } }
        regla.esperaEtiqueta("no_entendio_1")
        regla.onNodeWithText("“quiero una pizza”", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithText("“con piña”", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("borrar_no_entendio").performClick()
        regla.esperaEtiqueta("confirmar_borrar_no_entendio")
        regla.onNodeWithTag("confirmar_borrar_no_entendio").performClick()
        regla.espera { llamo("borrarNoEntendio:0") }
        regla.esperaTexto("Nada por aquí")
    }

    // ---------- estadísticas ampliadas ----------

    @Test fun lasEstadisticasMuestranAbandonoFormulariosYNoEntendio() {
        motor.estMenu = """{"iniciados":10,"completados":5,"abandonados":3,"humano":1,"expirados":1,"opciones":[],"embudo":[],"invalidas":[],
            "abandonos":[{"paso":"f","titulo":"Cotización","valor":3,"opcion":"","texto":""}],"formulariosEnviados":4,"formulariosCancelados":2,"noEntendio":6,"formularios":4}"""
        var formularios = 0L
        var noEntendio = 0L
        regla.setContent { BotChatTema { PantallaEstadisticasMenu(bot, 1, "Tienda", {}, { formularios = it }, { noEntendio = it }) } }
        regla.esperaEtiqueta("est_iniciados")
        regla.onNodeWithText("Cómo terminan").assertIsDisplayed()
        regla.esperaEtiqueta("barra_estados")
        regla.onNodeWithText("Dónde abandonan").assertIsDisplayed()
        regla.onNodeWithText("Cotización", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("est_form_enviados").assertIsDisplayed()
        regla.onNodeWithText("6 mensajes que el menú no entendió", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("ver_formularios_menu").performScrollTo().performClick()
        regla.espera { formularios == 1L }
        regla.onNodeWithTag("ver_no_entendio_menu").performScrollTo().performClick()
        regla.espera { noEntendio == 1L }
    }

    // ---------- ajustes generales ----------

    @Test fun losAjustesGeneralesSeEditanYSoloSeGuardaLoCambiado() {
        regla.setContent { BotChatTema { PantallaAjustesMenus(bot, {}) } }
        regla.esperaEtiqueta("am_menu_txt_n1")
        regla.onNodeWithTag("guardar_ajustes_menus").assertIsNotEnabled()
        regla.onNodeWithTag("am_menu_txt_n1").performTextReplacement("Perdón, no te entendí.")
        regla.onNodeWithTag("am_menu_saludos").performScrollTo().performTextReplacement("hola, ey, quiubole")
        regla.onNodeWithTag("am_volver_retomar").performScrollTo().performClick()
        regla.onNodeWithTag("am_dia_1").performScrollTo().performClick()
        regla.onNodeWithTag("am_horario_desde").performTextInput("09:00")
        regla.onNodeWithTag("am_grupo_max_sesiones").performScrollTo().performTextReplacement("3")
        regla.onNodeWithTag("am_grupo_cualquiera").performScrollTo().performClick()
        regla.onNodeWithTag("guardar_ajustes_menus").performClick()
        regla.espera { llamo("ajusteMenus:grupo_cualquiera:1") }
        val l = motor.llamadas.filter { it.startsWith("ajusteMenus:") }
        assertTrue(l.contains("ajusteMenus:menu_txt_n1:Perdón, no te entendí."))
        assertTrue(l.contains("ajusteMenus:menu_saludos:hola, ey, quiubole"))
        assertTrue(l.contains("ajusteMenus:menu_volver:retomar"))
        assertTrue(l.contains("ajusteMenus:grupo_max_sesiones:3"))
        assertTrue(l.any { it.startsWith("ajusteMenus:menu_horario:") && it.contains("\"desde\":\"09:00\"") && it.contains("[1]") })
        assertFalse("lo que no cambió no se manda", l.any { it.startsWith("ajusteMenus:menu_err_max") || it.startsWith("ajusteMenus:menu_txt_n2") })
    }

    @Test fun siUnAjusteNoSirveSeMuestraElErrorDeGo() {
        motor.menusFalla = "Escribe un número de 1 a 100."
        regla.setContent { BotChatTema { PantallaAjustesMenus(bot, {}) } }
        regla.esperaEtiqueta("am_grupo_max_sesiones")
        regla.onNodeWithTag("am_grupo_max_sesiones").performScrollTo().performTextReplacement("999")
        regla.onNodeWithTag("guardar_ajustes_menus").performClick()
        regla.esperaEtiqueta("error_ajustes_menus")
        regla.onNodeWithTag("error_ajustes_menus").performScrollTo().assertIsDisplayed()
        regla.onNodeWithText("Escribe un número de 1 a 100.", useUnmergedTree = true).assertIsDisplayed()
    }

    // ---------- lista de automatizaciones: formularios, no entendió, exportar ----------

    private fun resumen(id: Int, nombre: String) = JSONObject().put("id", id).put("activo", true).put("sistema", false).put("version", 1).put("nombre", nombre).put("resumen", "Comando").put("avisos", 0).put("pasos", 3)

    @Test fun elMenuMasDeAutomatizacionesAbreFormulariosNoEntendioYAjustes() {
        motor.autos = Apoyo.arreglo(resumen(1, "Tienda"))
        motor.formulariosJson = forms()
        var f = 0; var n = 0; var a = 0
        regla.setContent { BotChatTema { PantallaAutomatizaciones(bot, {}, {}, {}, {}, {}, {}, { f++ }, { n++ }, { a++ }) } }
        regla.esperaEtiqueta("mas_automatizaciones")
        regla.onNodeWithTag("mas_automatizaciones").performClick()
        regla.esperaEtiqueta("abrir_formularios")
        regla.onNodeWithText("Formularios recibidos (2)", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("abrir_formularios").performClick()
        regla.onNodeWithTag("mas_automatizaciones").performClick()
        regla.onNodeWithTag("abrir_no_entendio").performClick()
        regla.onNodeWithTag("mas_automatizaciones").performClick()
        regla.onNodeWithTag("abrir_ajustes_menus").performClick()
        regla.espera { f == 1 && n == 1 && a == 1 }
    }

    @Test fun exportarMenusPideElegirYPuedeLlevarContrasena() {
        motor.autos = Apoyo.arreglo(resumen(1, "Tienda"), resumen(2, "Citas"))
        regla.setContent { BotChatTema { PantallaAutomatizaciones(bot, {}, {}, {}, {}, {}, {}) } }
        regla.esperaEtiqueta("mas_automatizaciones")
        regla.onNodeWithTag("mas_automatizaciones").performClick()
        regla.onNodeWithTag("abrir_exportar_menus").performClick()
        regla.esperaEtiqueta("panel_exportar")
        regla.onNodeWithTag("exportar_menus").assertIsNotEnabled()
        regla.onNodeWithTag("exportar_menu_1").performClick()
        regla.onNodeWithTag("exportar_menu_2").performClick()
        regla.onNodeWithTag("exportar_clave").performTextInput("secreto1")
        regla.onNodeWithTag("exportar_menus").performClick()
        regla.espera { llamo("exportarMenus:[1,2]:true") }
        regla.espera { !hay("panel_exportar") }
    }

    @Test fun siElExportarFallaSeAvisaYElPanelSigue() {
        motor.autos = Apoyo.arreglo(resumen(1, "Tienda"))
        motor.menusFalla = "La contraseña debe tener al menos 6 caracteres."
        regla.setContent { BotChatTema { PantallaAutomatizaciones(bot, {}, {}, {}, {}, {}, {}) } }
        regla.esperaEtiqueta("mas_automatizaciones")
        regla.onNodeWithTag("mas_automatizaciones").performClick()
        regla.onNodeWithTag("abrir_exportar_menus").performClick()
        regla.esperaEtiqueta("exportar_menu_1")
        regla.onNodeWithTag("exportar_menu_1").performClick()
        regla.onNodeWithTag("exportar_menus").performClick()
        regla.espera { llamo("exportarMenus:[1]:false") }
        assertTrue(hay("panel_exportar"))
    }

    // ---------- las 8 plantillas se ofrecen al crear ----------

    @Test fun alCrearSeOfrecenTodasLasPlantillas() {
        val nombres = listOf("Atención básica", "Horarios y ubicación", "Cotización", "Refacciones", "Soporte", "Citas", "Preguntas frecuentes", "Menú para grupos")
        motor.plantillas = JSONArray().also { a -> nombres.forEach { n -> a.put(JSONObject().put("nombre", n).put("descripcion", "d").put("def", JSONObject(tienda().json()))) } }.toString()
        regla.setContent { BotChatTema { PantallaAutomatizaciones(bot, {}, {}, {}, {}, {}, {}) } }
        regla.esperaEtiqueta("nueva_automatizacion")
        regla.onNodeWithTag("nueva_automatizacion").performClick()
        regla.onNodeWithTag("nueva_desde_plantilla").performClick()
        regla.esperaEtiqueta("plantilla_Menú para grupos")
        nombres.forEach { assertTrue(it, hay("plantilla_$it")) }
    }

    // ---------- el simulador describe lo que no manda de verdad ----------

    @Test fun elSimuladorMuestraUbicacionYContactoComoEtiquetas() {
        motor.simulacion = mutableListOf("Aquí estamos.", "[Ubicación: Tienda]", "[Contacto: Ventas · +525512345678]")
        regla.setContent { BotChatTema { PantallaSimulador(bot, 1, "Tienda", {}) } }
        regla.esperaEtiqueta("campo_simulador")
        regla.onNodeWithTag("campo_simulador").performTextInput(".menu")
        regla.onNodeWithTag("enviar_simulador").performClick()
        regla.esperaTexto("[Ubicación: Tienda]")
        regla.esperaTexto("[Contacto: Ventas · +525512345678]")
        assertFalse(llamo("enviar:"))
    }

    // ---------- rangos: opciones sueltas ----------

    @Test fun unRangoPuedeVerSoloAlgunasOpcionesDeUnMenu() {
        motor.autos = Apoyo.arreglo(resumen(1, "Tienda"))
        motor.autoDetalle = detalle(tienda())
        regla.setContent { BotChatTema { EditorRango(bot, 0, {}) } }
        regla.esperaEtiqueta("rango_nombre")
        regla.onNodeWithTag("rango_nombre").performTextInput("Mayorista")
        regla.onNodeWithTag("menus_lista").performScrollTo().performClick()
        regla.esperaEtiqueta("opciones_sueltas_1")
        regla.onNodeWithTag("opciones_sueltas_1").performScrollTo().performClick()
        regla.esperaEtiqueta("opcion_1_o2")
        regla.onNodeWithTag("opcion_1_o2").performScrollTo().performClick()
        regla.onNodeWithTag("guardar_rango").performScrollTo().performClick()
        regla.espera { llamo("guardarRango:") }
        val j = JSONObject(motor.llamadas.first { it.startsWith("guardarRango:") }.removePrefix("guardarRango:"))
        assertEquals("lista", j.getString("menusModo"))
        assertEquals("1:o2", j.getJSONArray("menus").getString(0))
        assertEquals(1, j.getJSONArray("menus").length())
    }
}
