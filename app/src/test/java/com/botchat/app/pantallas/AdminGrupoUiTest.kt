package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.performTouchInput
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.printToString
import androidx.compose.ui.test.longClick
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaAdminGrupo
import com.botchat.app.ui.PantallaBaneadosGrupo
import com.botchat.app.ui.PantallaPerfilMiembro
import com.botchat.app.ui.SalidasAdminGrupo
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
class AdminGrupoUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L
    private val g = "g@g.us"
    private val ana = "5215511111111@s.whatsapp.net"
    private val luis = "5215522222222@s.whatsapp.net"
    private val rosa = "5215533333333@s.whatsapp.net"
    private val abiertos = mutableListOf<String>()

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot(); abiertos.clear() }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun nodo(tag: String) = regla.onNodeWithTag(tag, useUnmergedTree = true)
    private fun toca(tag: String) {
        regla.esperaEtiqueta(tag)
        runCatching { nodo(tag).performScrollTo() } // si no está dentro de algo que se desplaza, se toca donde está
        nodo(tag).performClick()
    }
    private fun salidas() = SalidasAdminGrupo(
        alAbrirPerfil = { abiertos += "perfil:$it" }, alAbrirBaneados = { abiertos += "baneados" }, alAbrirSolicitudes = { abiertos += "solicitudes" },
        alAbrirFiltros = { abiertos += "filtros" }, alAbrirReglas = { abiertos += "reglas" }, alAbrirAjustesAvanzados = { abiertos += "avanzados" },
        alAbrirActividad = { abiertos += "actividad" }, alAbrirAdvertencias = { abiertos += "advertencias" }, alAbrirSilencios = { abiertos += "silencios" },
        alAbrirBienvenida = { abiertos += "bienvenida" }, alAbrirModulos = { abiertos += "modulos" },
    )
    private fun admin(pestana: Int = 0) = regla.setContent { BotChatTema { PantallaAdminGrupo(bot, g, salidas(), pestana) } }
    private fun perfil(persona: String = ana) = regla.setContent { BotChatTema { PantallaPerfilMiembro(bot, g, persona, {}, { abiertos += "auditar:$it" }) } }
    private fun noAdmin() { motor.estadoGrupoJson = JSONObject(motor.estadoGrupoJson).put("soyAdmin", false).toString() }

    // ---------- Resumen ----------

    @Test fun elResumenMuestraLosNumerosLaGraficaYQuienEscribeMas() {
        motor.chat = Apoyo.chatJson(g, "Familia", grupo = true).toString()
        admin()
        regla.esperaEtiqueta("num_miembros")
        regla.esperaTexto("Familia")
        regla.esperaEtiqueta("num_hoy")
        regla.esperaTexto("12")
        regla.esperaEtiqueta("resumen_avisos")
        regla.esperaTexto("1 persona lleva más de 30 días sin escribir.", substring = false)
        regla.esperaTexto("1 persona baneada.")
        regla.esperaEtiqueta("top_$ana")
        regla.esperaTexto("120 mensajes")
        regla.esperaEtiqueta("accion_cerrar_abrir")
    }

    @Test fun lasSeisPestanasEstanYCambian() {
        admin()
        regla.esperaEtiqueta("admin_pestana_5")
        listOf("Resumen", "Personas", "Reglas", "Mensajes", "Ajustes", "Historial").forEach { regla.esperaTexto(it) }
        toca("admin_pestana_2"); regla.esperaEtiqueta("pestana_reglas_admin")
        toca("admin_pestana_3"); regla.esperaEtiqueta("pestana_mensajes_admin")
        toca("admin_pestana_4"); regla.esperaEtiqueta("pestana_ajustes_admin")
        toca("admin_pestana_5"); regla.esperaEtiqueta("pestana_historial_admin")
        toca("admin_pestana_1"); regla.esperaEtiqueta("pestana_personas_admin")
    }

    @Test fun cerrarElGrupoDesdeElResumenPideConfirmacionYLlamaALaAPI() {
        admin()
        toca("accion_cerrar_abrir")
        regla.esperaEtiqueta("confirmar_cerrar_abrir")
        assertFalse(llamo("grupoCerrar"))
        nodo("confirmar_cerrar_abrir").performClick()
        regla.espera { llamo("grupoCerrar:$g:true") }
    }

    @Test fun sinSerAdminNoDejaCerrar() {
        noAdmin()
        admin()
        toca("accion_cerrar_abrir")
        regla.waitForIdle()
        assertFalse(hay("confirmar_cerrar_abrir"))
        assertFalse(llamo("grupoCerrar"))
    }

    // ---------- Personas ----------

    @Test fun laListaDePersonasTraeFotoNumeroCargoUltimoMensajeYContadores() {
        admin(1)
        regla.esperaEtiqueta("persona_admin_$ana")
        regla.esperaTexto("+52 55 1111 1111 · En línea")
        regla.esperaEtiqueta("en_linea_$ana")
        assertTrue(hay("cargo_$ana"))
        regla.esperaTexto("Moderadora")
        regla.esperaTexto("Último mensaje hace 2 h")
        regla.esperaEtiqueta("conteo_$ana")
        regla.esperaTexto("120")
        regla.esperaTexto("Aún no ha escrito")
        regla.esperaEtiqueta("insignia_baneada_$rosa")
        assertTrue(llamo("suscribirPresencias:$g"))
        regla.esperaEtiqueta("contador_personas_admin")
        regla.esperaTexto("3 personas · 1 en línea")
    }

    @Test fun losFiltrosYLaBusquedaReducenLaLista() {
        admin(1)
        regla.esperaEtiqueta("persona_admin_$ana")
        toca("filtro_EN_LINEA")
        regla.espera { !hay("persona_admin_$luis") && hay("persona_admin_$ana") }
        toca("filtro_BANEADOS")
        regla.espera { hay("persona_admin_$rosa") && !hay("persona_admin_$ana") }
        toca("filtro_INACTIVOS")
        regla.espera { hay("persona_admin_$luis") && hay("persona_admin_$rosa") && !hay("persona_admin_$ana") }
        toca("filtro_TODOS")
        regla.espera { hay("persona_admin_$ana") && hay("persona_admin_$luis") }
        nodo("buscar_persona_admin").performTextInput("rosa")
        regla.espera { hay("persona_admin_$rosa") && !hay("persona_admin_$ana") }
    }

    @Test fun elOrdenCambiaLaListaYTocarUnaPersonaAbreSuPerfil() {
        admin(1)
        regla.esperaEtiqueta("persona_admin_$ana")
        nodo("persona_admin_$ana").performClick()
        regla.espera { abiertos.contains("perfil:$ana") }
        toca("ordenar_personas")
        regla.esperaEtiqueta("orden_NOMBRE")
        nodo("orden_NOMBRE").performClick()
        regla.waitForIdle()
        assertTrue(hay("persona_admin_$luis"))
    }

    @Test fun seleccionarVariasYExpulsarlasConConfirmacion() {
        admin(1)
        regla.esperaEtiqueta("persona_admin_$luis")
        nodo("persona_admin_$luis").performTouchInput { longClick() }
        regla.esperaEtiqueta("barra_seleccion")
        regla.esperaTexto("1 seleccionada")
        nodo("persona_admin_$rosa").performClick()
        regla.esperaTexto("2 seleccionadas")
        toca("seleccion_expulsar")
        regla.esperaEtiqueta("confirmar_expulsar_lote")
        assertFalse(llamo("grupoExpulsar"))
        nodo("confirmar_expulsar_lote").performClick()
        regla.espera { llamo("grupoExpulsar:$g:$luis") && llamo("grupoExpulsar:$g:$rosa") }
    }

    @Test fun seleccionarYBanearConMotivo() {
        admin(1)
        regla.esperaEtiqueta("persona_admin_$luis")
        nodo("persona_admin_$luis").performTouchInput { longClick() }
        toca("seleccion_banear")
        regla.esperaEtiqueta("motivo_ban_lote_campo")
        nodo("motivo_ban_lote_campo").performTextInput("spam")
        nodo("motivo_ban_lote_aceptar").performClick()
        regla.espera { llamo("grupoBanear:$luis:spam") }
    }

    @Test fun sinSerAdminNoSePuedeExpulsarEnLote() {
        noAdmin()
        admin(1)
        regla.esperaEtiqueta("persona_admin_$luis")
        nodo("persona_admin_$luis").performTouchInput { longClick() }
        regla.esperaEtiqueta("seleccion_expulsar")
        nodo("seleccion_expulsar").assertIsNotEnabled()
        nodo("seleccion_banear").assertIsNotEnabled()
    }

    @Test fun conElGrupoVacioDiceQueAunNoHayLista() {
        motor.miembrosAdminJson = """{"desde":0,"miembros":[]}"""
        admin(1)
        try { regla.esperaEtiqueta("personas_vacio") } catch (e: Throwable) {
            throw AssertionError("Pantalla:\n" + regla.onRoot(useUnmergedTree = true).printToString(40), e)
        }
    }

    // ---------- Perfil ----------

    @Test fun elPerfilMuestraLasInfraccionesDeFiltrosYPermiteReiniciarlas() {
        motor.perfilInfracciones = """[{"filtro":"enlaces","total":3,"ultima":1763000000},{"filtro":"palabras","total":1,"ultima":1763000000}]"""
        perfil()
        regla.esperaEtiqueta("perfil_infracciones")
        regla.esperaTexto("Enlaces: 3 infracciones")
        regla.esperaTexto("Palabras prohibidas: 1 infracción")
        toca("perfil_reiniciar_infracciones")
        regla.espera { llamo("reiniciarInfracciones:$ana:") }
        regla.espera { !hay("perfil_infracciones") }
    }

    @Test fun sinInfraccionesElPerfilNoMuestraLaSeccion() {
        perfil()
        regla.esperaEtiqueta("perfil_nombre")
        assertFalse(hay("perfil_infracciones"))
    }

    @Test fun elPerfilMuestraCabeceraDatosYComparacion() {
        perfil()
        regla.esperaEtiqueta("perfil_nombre")
        regla.esperaTexto("Ana")
        regla.esperaEtiqueta("perfil_telefono")
        regla.esperaEtiqueta("perfil_en_linea")
        regla.esperaTexto("En línea")
        regla.esperaEtiqueta("perfil_cargo")
        regla.esperaEtiqueta("perfil_rango")
        regla.esperaTexto("Último mensaje hace 2 h")
        regla.esperaTexto("Puesto 1 de 2 que han escrito")
        regla.esperaEtiqueta("dato_mensajes")
        regla.esperaEtiqueta("dato_notas")
        toca("perfil_pestana_1")
        regla.esperaEtiqueta("perfil_estadisticas")
        regla.esperaEtiqueta("perfil_promedio")
        regla.esperaTexto("Comparado con el grupo")
        toca("perfil_pestana_2")
        regla.esperaEtiqueta("aud_borrados")
        regla.esperaEtiqueta("ver_todo_lo_que_envio")
        nodo("ver_todo_lo_que_envio").performClick()
        regla.espera { abiertos.contains("auditar:Ana") }
        toca("perfil_pestana_3")
        regla.esperaEtiqueta("perfil_historial")
        regla.esperaTexto("Tú baneaste a Rosa")
    }

    @Test fun advertirSilenciarYHacerAdminLlamanALaAPI() {
        perfil(luis)
        regla.esperaEtiqueta("accion_advertir")
        nodo("accion_advertir").performClick()
        regla.esperaEtiqueta("motivo_advertir_campo")
        nodo("motivo_advertir_campo").performTextInput("insultos")
        nodo("motivo_advertir_aceptar").performClick()
        regla.espera { llamo("advertirPersona") || llamo("advertir") }
        toca("accion_silenciar")
        regla.esperaEtiqueta("silencio_3600")
        nodo("silencio_3600").performClick()
        regla.espera { llamo("silenciarGrupos:$luis:3600:uno") }
        toca("accion_admin")
        regla.esperaEtiqueta("confirmar_admin")
        nodo("confirmar_admin").performClick()
        regla.espera { llamo("grupoAscender:$luis:true") }
    }

    @Test fun expulsarYBanearDesdeElPerfil() {
        perfil(luis)
        toca("accion_expulsar")
        regla.esperaEtiqueta("confirmar_expulsar")
        nodo("confirmar_expulsar").performClick()
        regla.espera { llamo("grupoExpulsar:$g:$luis") }
        toca("accion_banear")
        regla.esperaEtiqueta("motivo_banear_campo")
        nodo("motivo_banear_campo").performTextInput("molesta")
        nodo("motivo_banear_aceptar").performClick()
        regla.espera { llamo("grupoBanear:$luis:molesta") }
    }

    @Test fun unaPersonaBaneadaSeDesbaneaDesdeElPerfil() {
        motor.perfilBaneado = true
        perfil(luis)
        regla.esperaEtiqueta("perfil_baneada")
        regla.esperaEtiqueta("perfil_pestana_0")
        regla.esperaEtiqueta("perfil_motivo_ban")
        toca("accion_banear")
        regla.esperaEtiqueta("confirmar_desbanear_perfil")
        nodo("confirmar_desbanear_perfil").performClick()
        regla.espera { llamo("grupoDesbanear:$luis") }
    }

    @Test fun sinSerAdminLosBotonesDelGrupoSeApagan() {
        noAdmin()
        perfil(luis)
        regla.esperaEtiqueta("accion_expulsar")
        nodo("accion_expulsar").assertIsNotEnabled()
        nodo("accion_admin").assertIsNotEnabled()
        nodo("accion_banear").assertIsNotEnabled()
        nodo("accion_advertir").assertIsEnabled()
    }

    // ---------- Ajustes, horario y baneados ----------

    @Test fun elHorarioSeEnciendeYSeElijenLosDias() {
        admin(4)
        regla.esperaEtiqueta("sw_horario")
        nodo("sw_horario").performScrollTo().performClick()
        regla.espera { llamo("guardarHorario:") && motor.horarioJson.contains("\"activo\":true") }
        regla.esperaEtiqueta("dia_6")
        nodo("dia_6").performScrollTo().performClick() // quita el domingo
        regla.espera { JSONObject(motor.horarioJson).optInt("dias") == 63 }
        assertEquals("22:00", JSONObject(motor.horarioJson).getString("cierra"))
    }

    @Test fun cerrarYBloquearDesdeAjustes() {
        admin(4)
        regla.esperaEtiqueta("sw_admin_cerrado")
        nodo("sw_admin_cerrado").performClick()
        regla.espera { llamo("grupoCerrar:$g:true") }
        nodo("sw_admin_info").performClick()
        regla.espera { llamo("grupoBloquear:$g:true") }
    }

    @Test fun laFilaDeBaneadosCuentaYAbre() {
        admin(4)
        regla.esperaTexto("1 persona baneada")
        toca("ir_baneados")
        regla.espera { abiertos.contains("baneados") }
        toca("ir_solicitudes")
        regla.espera { abiertos.contains("solicitudes") }
        toca("ir_ajustes_avanzados")
        regla.espera { abiertos.contains("avanzados") }
    }

    @Test fun laPantallaDeBaneadosListaYDesbanea() {
        regla.setContent { BotChatTema { PantallaBaneadosGrupo(bot, g, {}) } }
        regla.esperaEtiqueta("baneado_$rosa")
        regla.esperaTexto("Motivo: vendía cosas")
        nodo("desbanear_$rosa").performClick()
        regla.esperaEtiqueta("confirmar_desbanear")
        nodo("confirmar_desbanear").performClick()
        regla.espera { llamo("grupoDesbanear:$rosa") }
        regla.esperaEtiqueta("baneados_vacio")
    }

    // ---------- Reglas, mensajes e historial ----------

    @Test fun lasReglasYLosMensajesLlevanALasPantallasDeSiempre() {
        admin(2)
        toca("ir_filtros"); regla.espera { abiertos.contains("filtros") }
        toca("ir_advertencias"); regla.espera { abiertos.contains("advertencias") }
        toca("ir_silencios"); regla.espera { abiertos.contains("silencios") }
        toca("ir_reglas"); regla.espera { abiertos.contains("reglas") }
        toca("ir_actividad"); regla.espera { abiertos.contains("actividad") }
        toca("admin_pestana_3")
        toca("ir_bienvenida"); regla.espera { abiertos.contains("bienvenida") }
    }

    @Test fun elAnuncioSeManda() {
        admin(3)
        regla.esperaEtiqueta("anuncio_campo")
        nodo("anuncio_enviar").assertIsNotEnabled()
        nodo("anuncio_campo").performTextInput("Mañana hay junta")
        regla.espera { runCatching { nodo("anuncio_enviar").assertIsEnabled() }.isSuccess }
        nodo("anuncio_enviar").performClick()
        regla.espera { llamo("enviar:$g:Mañana hay junta") }
    }

    @Test fun elHistorialMuestraCambiosYAcciones() {
        admin(5)
        regla.esperaTexto("Tú baneaste a Rosa")
        regla.esperaTexto("Luis entró por el enlace")
        toca("filtro_historial_cambio")
        regla.espera { !hay("evento_${motor.ahoraS - 60}") && hay("evento_${motor.ahoraS - 7200}") }
        toca("filtro_historial_accion")
        regla.espera { hay("evento_${motor.ahoraS - 60}") && !hay("evento_${motor.ahoraS - 7200}") }
    }

    @Test fun conElHistorialVacioExplicaQueSeVeraAhi() {
        motor.historialJson = "[]"
        admin(5)
        regla.esperaEtiqueta("historial_vacio")
    }

    // ---------- Plantillas y copiar ----------

    @Test fun unaPlantillaCambiaQueFiltrosEstanEncendidosYSuAccion() {
        admin(4)
        toca("plantilla_estricto")
        regla.esperaEtiqueta("confirmar_plantilla")
        assertFalse(llamo("grupoFiltro"))
        nodo("confirmar_plantilla").performClick()
        regla.espera { motor.llamadas.count { it.startsWith("grupoFiltro:") } == 4 }
        val a = org.json.JSONArray(motor.filtrosGrupoJson)
        for (i in 0 until a.length()) {
            assertTrue(a.getJSONObject(i).getBoolean("activo"))
            assertEquals("borrar_advertir", a.getJSONObject(i).getString("accion"))
        }
        toca("plantilla_familia")
        nodo("confirmar_plantilla").performClick()
        regla.espera { org.json.JSONArray(motor.filtrosGrupoJson).let { b -> (0 until b.length()).none { b.getJSONObject(it).getBoolean("activo") } } }
    }

    @Test fun copiarDeOtroGrupoTraeFiltrosReglasYHorario() {
        motor.chatsGrupos = org.json.JSONArray().put(Apoyo.chatJson("otro@g.us", "Ventas", grupo = true)).toString()
        motor.reglasGrupo = "No spam"
        admin(4)
        toca("copiar_de_otro")
        regla.esperaEtiqueta("elegir_origen_otro@g.us")
        nodo("elegir_origen_otro@g.us").performClick()
        regla.esperaEtiqueta("confirmar_copiar")
        nodo("confirmar_copiar").performClick()
        regla.espera { llamo("guardarHorario:") && llamo("grupoReglas:No spam") && llamo("grupoFiltro:") }
    }

    // ---------- Protección de entrada ----------

    @Test fun elModoRaidYLosPrefijosSeGuardan() {
        admin(4)
        regla.esperaEtiqueta("sw_raid")
        nodo("sw_raid").performScrollTo().performClick()
        regla.espera { llamo("guardarEntrada:") && motor.entradaJson.contains("\"raidActivo\":true") }
        regla.esperaEtiqueta("raid_max_mas")
        nodo("raid_max_mas").performScrollTo().performClick()
        regla.espera { JSONObject(motor.entradaJson).optInt("raidMax") == 9 }
        nodo("prefijo_campo").performScrollTo().performTextInput("+212")
        nodo("prefijo_agregar").performClick()
        regla.espera { JSONObject(motor.entradaJson).getJSONArray("prefijos").toString() == "[\"212\"]" }
        regla.esperaEtiqueta("prefijo_212")
        nodo("prefijo_212").performClick() // tocar el chip lo quita
        regla.espera { JSONObject(motor.entradaJson).getJSONArray("prefijos").length() == 0 }
    }
}
