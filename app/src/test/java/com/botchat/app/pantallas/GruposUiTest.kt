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
import androidx.compose.ui.test.performTextReplacement
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaActividadGrupo
import com.botchat.app.ui.PantallaFiltrosGrupo
import com.botchat.app.ui.PantallaGrupo
import com.botchat.app.ui.PantallaInfoChat
import com.botchat.app.ui.PantallaMiembrosGrupo
import com.botchat.app.ui.SeccionMiWhatsApp
import com.botchat.app.ui.TarjetaMiWhatsApp
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
class GruposUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L
    private val g = "g@g.us"
    private val ana = "5215511111111@s.whatsapp.net"
    private val luis = "5215522222222@s.whatsapp.net"
    private val carla = "5215544444444@s.whatsapp.net"

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun nodo(tag: String) = regla.onNodeWithTag(tag, useUnmergedTree = true)
    private fun noAdmin(motivo: String = "No soy administrador de este grupo. Puedes ver la información.") {
        motor.estadoGrupoJson = JSONObject().put("conectado", true).put("enGrupo", true).put("soyAdmin", false).put("motivo", motivo).toString()
        motor.infoGrupoJson = JSONObject(motor.infoGrupoJson).put("soyAdmin", false).toString()
    }
    private fun grupo() = regla.setContent { BotChatTema { PantallaGrupo(bot, g, {}) } }

    // ---------- pantalla Grupo ----------

    @Test fun siSoyAdminSeMuestraLaTarjetaYTodoEstaActivo() {
        grupo()
        regla.esperaEtiqueta("grupo_estado_titulo")
        regla.esperaTexto("Soy administrador")
        regla.esperaEtiqueta("grupo_info")
        regla.esperaTexto("3 miembros · 1 administrador")
        regla.esperaTexto("Administradores: Ana", substring = true)
        nodo("sw_grupo_cerrado").assertIsEnabled()
        nodo("sw_grupo_bloqueado").assertIsEnabled()
        nodo("grupo_ver_enlace").performScrollTo().assertIsEnabled()
        nodo("grupo_enlace_nuevo").assertIsEnabled()
        assertFalse(hay("aviso_admin"))
    }

    @Test fun cerrarYBloquearLlamanALaAPI() {
        grupo()
        regla.esperaEtiqueta("sw_grupo_cerrado")
        nodo("sw_grupo_cerrado").performClick()
        regla.espera { llamo("grupoCerrar:$g:true") }
        nodo("sw_grupo_bloqueado").performClick()
        regla.espera { llamo("grupoBloquear:$g:true") }
    }

    @Test fun renombrarYDescripcionSoloSeActivanSiCambian() {
        grupo()
        regla.esperaEtiqueta("grupo_nombre_guardar")
        nodo("grupo_nombre_guardar").performScrollTo().assertIsNotEnabled() // sin cambios
        nodo("grupo_nombre_campo").performScrollTo().performTextReplacement("Primos")
        nodo("grupo_nombre_guardar").assertIsEnabled().performClick()
        regla.espera { llamo("grupoRenombrar:Primos") }
        nodo("grupo_descripcion_campo").performScrollTo().performTextReplacement("Hola")
        nodo("grupo_descripcion_guardar").assertIsEnabled().performClick()
        regla.espera { llamo("grupoDescripcion:Hola") }
    }

    @Test fun elEnlaceSeVeYReiniciarPideConfirmar() {
        grupo()
        regla.esperaEtiqueta("grupo_ver_enlace")
        nodo("grupo_ver_enlace").performScrollTo().performClick()
        regla.esperaEtiqueta("grupo_enlace_texto")
        regla.esperaTexto("https://chat.whatsapp.com/ABCDEF")
        nodo("grupo_enlace_nuevo").performScrollTo().performClick()
        regla.esperaTexto("El enlace anterior deja de funcionar", substring = true)
        assertFalse(llamo("grupoEnlace:true"))
        nodo("confirmar_enlace_nuevo").performClick()
        regla.espera { llamo("grupoEnlace:true") }
        regla.esperaTexto("https://chat.whatsapp.com/NUEVO99")
    }

    @Test fun mencionarATodosPideConfirmacionYAvisaElTope() {
        motor.resultadoTodos = """{"mencionadas":50,"total":70,"mensajes":3,"truncado":true,"aviso":"Mencioné a 50 de 70 personas porque el tope es 50."}"""
        grupo()
        regla.esperaEtiqueta("grupo_todos")
        nodo("grupo_todos_campo").performScrollTo().performTextInput("Reunión a las 5")
        nodo("grupo_todos").performScrollTo().assertIsEnabled().performClick()
        regla.esperaTexto("¿Mencionar a todos?")
        assertFalse(llamo("grupoTodos"))
        nodo("confirmar_todos").performClick()
        regla.espera { llamo("grupoTodos:Reunión a las 5") }
    }

    @Test fun sinSerAdminTodoLoDeAdminQuedaDesactivadoConUnAvisoClaro() {
        noAdmin()
        grupo()
        regla.esperaEtiqueta("grupo_estado_titulo")
        regla.esperaTexto("No soy administrador")
        regla.esperaTexto("No soy administrador de este grupo. Puedes ver la información.")
        for (t in listOf("sw_grupo_cerrado", "sw_grupo_bloqueado")) nodo(t).assertIsNotEnabled()
        nodo("grupo_nombre_campo").performScrollTo().assertIsNotEnabled()
        nodo("grupo_nombre_guardar").assertIsNotEnabled()
        nodo("grupo_descripcion_guardar").assertIsNotEnabled()
        nodo("grupo_ver_enlace").performScrollTo().assertIsNotEnabled()
        nodo("grupo_enlace_nuevo").assertIsNotEnabled()
        assertTrue(hay("aviso_admin"))
        // Lo que no pide ser admin sigue funcionando: información, reglas, mencionar a todos, filtros y actividad.
        regla.esperaEtiqueta("grupo_info")
        nodo("grupo_reglas_campo").performScrollTo().assertIsEnabled().performTextInput("Sé amable")
        nodo("grupo_reglas_guardar").performClick()
        regla.espera { llamo("grupoReglas:Sé amable") }
        nodo("grupo_todos").performScrollTo().assertIsEnabled()
        nodo("grupo_abrir_filtros").performScrollTo().assertIsEnabled()
        nodo("grupo_abrir_actividad").assertIsEnabled()
        assertFalse(llamo("grupoCerrar") || llamo("grupoRenombrar") || llamo("grupoEnlace"))
    }

    @Test fun sinConexionSeExplicaYNoSeOfreceNada() {
        motor.estadoGrupoJson = """{"conectado":false,"enGrupo":false,"soyAdmin":false,"motivo":"El bot no está conectado, así que no sé si administro este grupo. Conéctalo para usar las acciones del grupo."}"""
        motor.grupoFalla = "no hay conexión con WhatsApp: revisa que el bot esté encendido"
        grupo()
        regla.esperaEtiqueta("grupo_estado_titulo")
        regla.esperaTexto("Sin conexión")
        regla.esperaTexto("El bot no está conectado", substring = true)
        regla.esperaEtiqueta("grupo_info_error")
        regla.esperaTexto("No hay conexión con WhatsApp", substring = true)
        nodo("sw_grupo_cerrado").assertIsNotEnabled()
        nodo("grupo_todos").performScrollTo().assertIsNotEnabled()
    }

    @Test fun unErrorDeWhatsAppSeMuestraEnClaroYNoRompeLaPantalla() {
        grupo()
        regla.esperaEtiqueta("sw_grupo_cerrado")
        motor.grupoFalla = "WhatsApp no lo permitió: casi seguro ya no soy administrador de este grupo"
        nodo("sw_grupo_cerrado").performClick()
        regla.espera { llamo("grupoCerrar") }
        regla.esperaEtiqueta("sw_grupo_cerrado") // sigue ahí
    }

    // ---------- miembros y solicitudes ----------

    @Test fun miembrosSubirBajarYExpulsarConConfirmacion() {
        regla.setContent { BotChatTema { PantallaMiembrosGrupo(bot, g, "miembros", {}) } }
        regla.esperaEtiqueta("miembro_$luis")
        regla.esperaTexto("Creó el grupo")
        assertFalse(hay("ascender_$carla")) // quien creó el grupo no se toca
        nodo("ascender_$luis").assertIsEnabled().performClick()
        regla.espera { llamo("grupoAscender:$luis:true") }
        nodo("ascender_$ana").performClick() // Ana ya es admin: se le quita
        regla.espera { llamo("grupoAscender:$ana:false") }
        nodo("expulsar_$luis").performClick()
        regla.esperaTexto("¿Sacar a Luis del grupo?")
        assertFalse(llamo("grupoExpulsar"))
        nodo("confirmar_expulsar").performClick()
        regla.espera { llamo("grupoExpulsar:$g:$luis") }
    }

    @Test fun miembrosSinSerAdminMuestraLaListaPeroSinBotones() {
        noAdmin()
        regla.setContent { BotChatTema { PantallaMiembrosGrupo(bot, g, "miembros", {}) } }
        regla.esperaEtiqueta("miembro_$luis")
        regla.esperaEtiqueta("aviso_admin")
        nodo("ascender_$luis").assertIsNotEnabled()
        nodo("expulsar_$luis").assertIsNotEnabled()
    }

    @Test fun solicitudesAprobarRechazarYTodas() {
        motor.solicitudesGrupoJson = JSONArray().put(JSONObject().put("jid", luis).put("persona", luis).put("nombre", "Luis").put("telefono", "+52 55 2222 2222").put("ts", 1_700_000_000L))
            .put(JSONObject().put("jid", carla).put("persona", carla).put("nombre", "Carla").put("telefono", "").put("ts", 0)).toString()
        regla.setContent { BotChatTema { PantallaMiembrosGrupo(bot, g, "solicitudes", {}) } }
        regla.esperaEtiqueta("solicitud_$luis")
        nodo("aprobar_$luis").performClick()
        regla.espera { llamo("grupoSolicitudes:aprobar:[\"$luis\"]") }
        nodo("rechazar_todas").performClick()
        regla.espera { llamo("grupoSolicitudes:rechazar:") }
    }

    @Test fun solicitudesSinSerAdminLoExplica() {
        noAdmin()
        regla.setContent { BotChatTema { PantallaMiembrosGrupo(bot, g, "solicitudes", {}) } }
        regla.esperaEtiqueta("solicitudes_sin_admin")
        regla.esperaTexto("Necesito ser administrador del grupo para esto", substring = true)
        assertFalse(hay("aprobar_todas"))
    }

    // ---------- filtros ----------

    private fun filtros() = regla.setContent { BotChatTema { PantallaFiltrosGrupo(bot, g, {}) } }

    @Test fun losCuatroFiltrosSeEnciendenYCambianDeAccion() {
        filtros()
        regla.esperaEtiqueta("sw_filtro_enlaces")
        for (f in listOf("enlaces", "palabras", "repetidos", "flood")) assertTrue(hay("filtro_$f"))
        assertFalse(hay("accion_enlaces_borrar")) // apagado: no muestra opciones
        nodo("sw_filtro_enlaces").performClick()
        regla.espera { llamo("grupoFiltro:") && motor.llamadas.any { it.contains("\"filtro\":\"enlaces\"") && it.contains("\"activo\":true") } }
        regla.esperaEtiqueta("accion_enlaces_borrar")
        nodo("accion_enlaces_borrar").assertIsEnabled().performClick()
        regla.espera { motor.llamadas.any { it.contains("\"filtro\":\"enlaces\"") && it.contains("\"accion\":\"borrar\"") } }
        regla.esperaTexto("Borra el mensaje para todos.", substring = true)
    }

    @Test fun enlacesConDominiosPermitidosYPalabrasEditables() {
        filtros()
        regla.esperaEtiqueta("sw_filtro_enlaces")
        nodo("sw_filtro_enlaces").performClick()
        regla.esperaEtiqueta("permitido_campo")
        nodo("permitido_agregar").assertIsNotEnabled()
        nodo("permitido_campo").performTextInput("youtube.com")
        nodo("permitido_agregar").assertIsEnabled().performClick()
        regla.espera { motor.llamadas.any { it.contains("\"permitidos\":[\"youtube.com\"]") } }
        regla.esperaEtiqueta("permitido_item_youtube.com")
        nodo("permitido_item_youtube.com").performClick() // tocar el chip lo quita
        regla.espera { motor.llamadas.last { it.startsWith("grupoFiltro:") }.contains("\"permitidos\":[]") }
        nodo("sw_filtro_palabras").performClick()
        regla.esperaEtiqueta("palabra_campo")
        nodo("palabra_campo").performTextInput("estafa")
        nodo("palabra_agregar").performClick()
        regla.espera { motor.llamadas.any { it.contains("\"palabras\":[\"estafa\"]") } }
    }

    private fun palabrasMuchas(n: Int) {
        val a = JSONArray(motor.filtrosGrupoJson)
        for (i in 0 until a.length()) a.getJSONObject(i).takeIf { it.getString("filtro") == "palabras" }?.also { o ->
            o.put("activo", true)
            o.getJSONObject("config").put("palabras", JSONArray((0 until n).map { "palabra$it" }))
        }
        motor.filtrosGrupoJson = a.toString()
    }

    @Test fun conMuchasPalabrasLaListaSeVeEnPaginasSeBuscaYNoSaturaLaPantalla() {
        palabrasMuchas(150)
        filtros()
        regla.esperaEtiqueta("palabra_cuenta")
        regla.esperaTexto("150 elementos")
        regla.esperaEtiqueta("palabra_mostrando")
        regla.esperaTexto("Mostrando 30 de 150")
        assertTrue(hay("palabra_item_palabra149")) // los más nuevos primero
        assertFalse(hay("palabra_item_palabra0")) // el resto no se dibuja
        nodo("palabra_ver_mas").performScrollTo().performClick()
        regla.esperaTexto("Mostrando 120 de 150")
        nodo("palabra_buscar").performScrollTo().performTextInput("palabra14")
        regla.espera { hay("palabra_item_palabra149") && !hay("palabra_item_palabra99") && !hay("palabra_mostrando") }
    }

    @Test fun sePuedenPegarVariasPalabrasDeGolpeYQuitarlasTodas() {
        palabrasMuchas(10)
        filtros()
        regla.esperaEtiqueta("palabra_campo")
        nodo("palabra_agregar").performScrollTo().assertIsNotEnabled()
        nodo("palabra_campo").performScrollTo().performTextInput("uno, dos; tres,  palabra3 , UNO")
        regla.esperaTexto("Agregar 3") // sin repetir: palabra3 ya estaba y UNO es igual a uno
        nodo("palabra_agregar").performClick()
        regla.espera { motor.llamadas.last { it.startsWith("grupoFiltro:") }.contains("\"uno\",\"dos\",\"tres\"]") }
        regla.esperaTexto("13 elementos")
        nodo("palabra_quitar_todas").performScrollTo().performClick()
        regla.esperaEtiqueta("palabra_confirmar_quitar")
        nodo("palabra_confirmar_quitar").performClick()
        regla.espera { motor.llamadas.last { it.startsWith("grupoFiltro:") }.contains("\"palabras\":[]") }
        regla.esperaTexto("0 elementos")
    }

    @Test fun cadaFiltroTieneEscaladoPorContadorYListaDeInfractores() {
        filtros()
        regla.esperaEtiqueta("sw_filtro_enlaces")
        nodo("sw_filtro_enlaces").performClick()
        regla.esperaEtiqueta("escala_desde_enlaces_valor")
        assertFalse(hay("escala_accion_enlaces_borrar")) // sin escalado no hay más opciones
        nodo("escala_desde_enlaces_mas").performScrollTo().performClick() // de 0 salta a 2 (la primera infracción es la acción de siempre)
        regla.espera { motor.llamadas.any { it.contains("\"filtro\":\"enlaces\"") && it.contains("\"escalaDesde\":2") } }
        regla.esperaEtiqueta("escala_accion_enlaces_expulsar")
        nodo("escala_accion_enlaces_expulsar").performScrollTo().performClick()
        regla.espera { motor.llamadas.any { it.contains("\"escalaAccion\":\"expulsar\"") } }
        nodo("escala_dias_enlaces_30").performScrollTo().performClick()
        regla.espera { motor.llamadas.any { it.contains("\"escalaDias\":30") } }
        nodo("escala_desde_enlaces_menos").performScrollTo().performClick() // de 2 vuelve a 0
        regla.espera { motor.llamadas.last { it.startsWith("grupoFiltro:") }.contains("\"escalaDesde\":0") }
    }

    @Test fun lasInfraccionesSeVenSeReiniciaYSePuedeEximirAUnaPersona() {
        filtros()
        regla.esperaEtiqueta("sw_filtro_palabras")
        nodo("sw_filtro_palabras").performClick()
        regla.esperaEtiqueta("ver_infracciones_palabras")
        nodo("ver_infracciones_palabras").performScrollTo().performClick()
        regla.esperaEtiqueta("infractor_palabras_5215511111111")
        regla.esperaTexto("3 infracciones")
        regla.esperaTexto("1 infracción")
        assertTrue(llamo("infractores:palabras"))
        nodo("reiniciar_palabras_5215522222222").performScrollTo().performClick() // Luis
        regla.espera { llamo("reiniciarInfracciones:5215522222222@s.whatsapp.net:palabras") }
        regla.espera { !hay("infractor_palabras_5215522222222") }
        nodo("eximir_palabras_5215511111111").performScrollTo().performClick() // Ana no se cuenta más
        regla.espera { motor.llamadas.any { it.contains("\"exentoPersonas\":[\"5215511111111@s.whatsapp.net\"]") } }
        regla.esperaEtiqueta("exento_persona_palabras_5215511111111")
        regla.espera { !hay("infractor_palabras_5215511111111") }
        regla.esperaEtiqueta("infractores_vacio_palabras")
    }

    @Test fun repetidosYFloodTienenLimitesAjustables() {
        filtros()
        regla.esperaEtiqueta("sw_filtro_flood")
        nodo("sw_filtro_flood").performClick()
        regla.esperaEtiqueta("flood_max_valor")
        nodo("flood_max_mas").performScrollTo().performClick()
        regla.espera { motor.llamadas.any { it.contains("\"filtro\":\"flood\"") && it.contains("\"max\":7") } }
        nodo("flood_seg_menos").performClick()
        regla.espera { motor.llamadas.any { it.contains("\"filtro\":\"flood\"") && it.contains("\"segundos\":9") } }
        nodo("sw_filtro_repetidos").performScrollTo().performClick()
        regla.esperaEtiqueta("rep_max_valor")
        nodo("rep_max_menos").performClick()
        regla.espera { motor.llamadas.any { it.contains("\"filtro\":\"repetidos\"") && it.contains("\"max\":2") } }
    }

    @Test fun lasExcepcionesSePuedenElegirPorAdminYPorRango() {
        motor.rangos = Apoyo.rangosEjemplo
        filtros()
        regla.esperaEtiqueta("sw_filtro_enlaces")
        nodo("sw_filtro_enlaces").performClick()
        regla.esperaEtiqueta("exento_admins_enlaces")
        nodo("exento_admins_enlaces").performClick()
        regla.espera { motor.llamadas.any { it.contains("\"filtro\":\"enlaces\"") && it.contains("\"exentoAdmins\":false") } }
        regla.esperaEtiqueta("exento_rango_enlaces_2")
        assertFalse(hay("exento_rango_enlaces_1")) // el Dueño nunca se lista: ya está exento
        nodo("exento_rango_enlaces_2").performClick()
        regla.espera { motor.llamadas.any { it.contains("\"exentoRangos\":[2]") } }
    }

    @Test fun sinSerAdminBorrarYExpulsarQuedanDesactivadosYSeAvisa() {
        noAdmin()
        filtros()
        regla.esperaEtiqueta("filtros_sin_admin")
        regla.esperaTexto("solo te avisan con una notificación", substring = true)
        nodo("sw_filtro_enlaces").performClick()
        regla.esperaEtiqueta("accion_enlaces_borrar")
        nodo("accion_enlaces_borrar").assertIsNotEnabled()
        nodo("accion_enlaces_expulsar").assertIsNotEnabled()
        nodo("accion_enlaces_avisarme").assertIsEnabled()
        nodo("accion_enlaces_advertir").assertIsEnabled()
        nodo("accion_enlaces_silenciar").assertIsEnabled()
        nodo("accion_enlaces_avisar_grupo").assertIsEnabled()
    }

    @Test fun lasAccionesParaElGrupoYElSilencioTienenSusOpciones() {
        filtros()
        regla.esperaEtiqueta("sw_filtro_enlaces")
        nodo("sw_filtro_enlaces").performClick()
        regla.esperaEtiqueta("accion_enlaces_silenciar")
        nodo("accion_enlaces_silenciar").performClick()
        regla.esperaEtiqueta("silencio_valor")
        nodo("accion_enlaces_avisar_grupo").performClick()
        regla.esperaEtiqueta("aviso_enlaces")
        nodo("aviso_enlaces").performTextInput("Nada de enlaces")
        regla.espera { motor.llamadas.any { it.contains("\"aviso\":\"Nada de enlaces\"") } }
        nodo("restaurar_enlaces").performScrollTo().performClick()
        regla.espera { llamo("grupoRestaurar:enlaces") }
    }

    // ---------- actividad ----------

    @Test fun rankingEInactivosConPeriodoElegible() {
        motor.rankingGrupoJson = JSONObject().put("dias", 30).put("total", 10).put("soloMiembros", true).put("desde", "2026-10-01").put("aviso", "Esta app solo cuenta los mensajes desde 2026-10-01.")
            .put("filas", JSONArray().put(JSONObject().put("puesto", 1).put("persona", ana).put("nombre", "Ana").put("telefono", "").put("mensajes", 8).put("porcentaje", 80.0).put("yo", false))
                .put(JSONObject().put("puesto", 2).put("persona", luis).put("nombre", "Luis").put("telefono", "").put("mensajes", 2).put("porcentaje", 20.0).put("yo", false))).toString()
        motor.inactivosGrupoJson = JSONObject().put("dias", 30).put("aviso", "")
            .put("inactivos", JSONArray().put(JSONObject().put("persona", carla).put("nombre", "Carla").put("telefono", "").put("ultimoDia", "").put("diasSin", -1).put("esAdmin", true))).toString()
        regla.setContent { BotChatTema { PantallaActividadGrupo(bot, g, {}) } }
        regla.esperaEtiqueta("ranking_1")
        regla.esperaTexto("1. Ana")
        regla.esperaTexto("8 · 80%")
        regla.esperaEtiqueta("actividad_aviso")
        nodo("dias_7").performClick()
        regla.espera { llamo("grupoRanking:7") }
        nodo("dias_90").performClick()
        regla.espera { llamo("grupoRanking:90") }
        nodo("tab_inactivos").performClick()
        regla.esperaEtiqueta("inactivos_total")
        regla.esperaTexto("Carla (administrador)")
        regla.esperaTexto("Sin mensajes guardados")
        assertTrue(llamo("grupoInactivos:90"))
    }

    @Test fun actividadVaciaOSinConexionSeExplica() {
        regla.setContent { BotChatTema { PantallaActividadGrupo(bot, g, {}) } }
        regla.esperaEtiqueta("ranking_vacio")
        nodo("tab_inactivos").performClick()
        regla.esperaEtiqueta("inactivos_vacio")
        motor.grupoFalla = "no hay conexión con WhatsApp: revisa que el bot esté encendido"
        nodo("dias_7").performClick()
        regla.esperaEtiqueta("actividad_error")
    }

    // ---------- mi WhatsApp (app state) ----------

    private val llaves = """{"disponible":true,"conectado":true,"motivo":""}"""

    @Test fun sinLlavesLosBotonesDeMiWhatsAppNoSeMuestran() {
        regla.setContent { BotChatTema { TarjetaMiWhatsApp(bot, ana, "Ana") } }
        regla.waitForIdle()
        Thread.sleep(300)
        regla.waitForIdle()
        assertFalse(hay("tarjeta_mi_whatsapp"))
        assertFalse(hay("mw_silenciar") || hay("mw_fijar") || hay("mw_archivar"))
    }

    @Test fun conLlavesSilenciarFijarYArchivar() {
        motor.appStateJson = llaves
        regla.setContent { BotChatTema { TarjetaMiWhatsApp(bot, ana, "Ana") } }
        regla.esperaEtiqueta("tarjeta_mi_whatsapp")
        nodo("mw_silenciar").performClick()
        regla.esperaEtiqueta("mw_silencio_8h")
        nodo("mw_confirmar_silencio").performClick()
        regla.espera { llamo("appSilenciar:$ana:28800") }
        nodo("mw_silenciar").performClick()
        nodo("mw_silencio_1s").performClick()
        nodo("mw_confirmar_silencio").performClick()
        regla.espera { llamo("appSilenciar:$ana:604800") }
        nodo("mw_silenciar").performClick()
        nodo("mw_silencio_siempre").performClick()
        nodo("mw_confirmar_silencio").performClick()
        regla.espera { llamo("appSilenciar:$ana:0") }
        nodo("mw_desilenciar").performClick()
        regla.espera { llamo("appDesilenciar:$ana") }
        nodo("mw_fijar").performClick()
        regla.espera { llamo("appFijar:$ana:true") }
        nodo("mw_desfijar").performClick()
        regla.espera { llamo("appFijar:$ana:false") }
        nodo("mw_archivar").performClick()
        regla.espera { llamo("appArchivar:$ana:true") }
        nodo("mw_desarchivar").performClick()
        regla.espera { llamo("appArchivar:$ana:false") }
    }

    @Test fun enInfoDelChatPrivadoLaTarjetaDeMiWhatsAppDependeDeLasLlaves() {
        motor.chat = Apoyo.chatJson(ana, "Ana Pérez").toString()
        regla.setContent { BotChatTema { PantallaInfoChat(bot, ana, {}, {}, {}) } }
        regla.esperaEtiqueta("tarjeta_persona")
        Thread.sleep(300); regla.waitForIdle()
        assertFalse(hay("tarjeta_mi_whatsapp"))
    }

    @Test fun enInfoDelChatPrivadoConLlavesAparece() {
        motor.appStateJson = llaves
        motor.chat = Apoyo.chatJson(ana, "Ana Pérez").toString()
        regla.setContent { BotChatTema { PantallaInfoChat(bot, ana, {}, {}, {}) } }
        regla.esperaEtiqueta("tarjeta_mi_whatsapp")
    }

    @Test fun enInfoDeUnGrupoHayEntradaAAdministrarYTambienMiWhatsApp() {
        motor.appStateJson = llaves
        motor.chat = Apoyo.chatJson(g, "Familia", tel = "", pais = "", grupo = true).toString()
        var abierto = false
        regla.setContent { BotChatTema { PantallaInfoChat(bot, g, {}, {}, {}, alAbrirGrupo = { abierto = true }) } }
        regla.esperaEtiqueta("tarjeta_administrar_grupo")
        regla.esperaEtiqueta("tarjeta_mi_whatsapp")
        nodo("abrir_grupo").performClick()
        assertTrue(abierto)
    }

    @Test fun avanzadoExplicaLoQueFaltaYOfrecePedirSincronizacion() {
        regla.setContent { BotChatTema { androidx.compose.foundation.layout.Column { SeccionMiWhatsApp(bot) } } }
        regla.esperaEtiqueta("appstate_texto")
        regla.esperaTexto("Para silenciar/fijar/archivar en tu WhatsApp hace falta que el teléfono principal sincronice (abre WhatsApp en tu teléfono principal y espera un momento).")
        nodo("appstate_sincronizar").performClick()
        regla.espera { llamo("appSincronizar") }
    }

    @Test fun avanzadoConLlavesLoDiceYNoOfreceSincronizar() {
        motor.appStateJson = llaves
        regla.setContent { BotChatTema { androidx.compose.foundation.layout.Column { SeccionMiWhatsApp(bot) } } }
        regla.esperaTexto("Listo: en la ficha de cada chat verás los botones", substring = true)
        assertFalse(hay("appstate_sincronizar"))
    }
}
