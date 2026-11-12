package com.botchat.app.pantallas

import androidx.activity.ComponentActivity
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTouchInput
import androidx.compose.ui.test.longClick
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.performTextReplacement
import com.botchat.app.datos.Ajustes
import com.botchat.app.datos.EstadoAlmacen
import com.botchat.app.datos.Msg
import com.botchat.app.ui.AvisoAuditoria
import com.botchat.app.ui.AvisoTopeAlmacen
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.EditorTipoAdvertencia
import com.botchat.app.ui.PantallaAdvertencias
import com.botchat.app.ui.PantallaAjustesSilencio
import com.botchat.app.ui.PantallaAuditoria
import com.botchat.app.ui.PantallaChat
import com.botchat.app.ui.PantallaEspacio
import com.botchat.app.ui.PantallaEtiquetas
import com.botchat.app.ui.PantallaMarcados
import com.botchat.app.ui.PantallaReglasGrupo
import com.botchat.app.ui.PantallaSilencios
import com.botchat.app.ui.PantallaTiposAdvertencia
import com.botchat.app.ui.TarjetaVideoEliminado
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

/** Etapa E8 (0.6.0): almacenamiento, auditoría y privacidad, silencios, advertencias con escalera, reglas, etiquetas y marcados. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class E8UiTest {
    @get:Rule val regla = createAndroidComposeRule<ComponentActivity>()
    private lateinit var motor: MotorFalso
    private lateinit var ajustes: Ajustes
    private var bot = 0L
    private val ana = "5215512345678@s.whatsapp.net"
    private val g1 = "120363000000000001@g.us"
    private val ahora = System.currentTimeMillis() / 1000
    private val GB = 1024L * 1024 * 1024
    private val MB = 1024L * 1024

    @Before fun antes() {
        motor = Apoyo.reinicia(); bot = Apoyo.creaBot()
        ajustes = Ajustes.get(Apoyo.ctx())
        ajustes.aceptarAviso()
        motor.chatsPersonales = JSONArray().put(Apoyo.chatJson(ana, "Ana Pérez")).toString()
        motor.chatsGrupos = JSONArray().put(Apoyo.chatJson(g1, "Vecinos del 5", tel = "", pais = "", grupo = true)).toString()
    }

    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodes(hasTestTag(tag), useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun toca(tag: String) {
        regla.espera { hay(tag) }
        val n = regla.onNodeWithTag(tag, useUnmergedTree = true)
        runCatching { n.performScrollTo() }
        n.performClick()
    }
    private fun tocaD(tag: String) { regla.onNodeWithTag(tag, useUnmergedTree = true).assertExists(); regla.onNodeWithTag(tag, useUnmergedTree = true).performClick() }
    private fun escribe(tag: String, texto: String) { regla.esperaEtiqueta(tag); regla.onNodeWithTag(tag, useUnmergedTree = true).performTextReplacement(texto) }

    private fun almacen(tope: Long, uso: Long, sinPortada: Int = 0, ahorro: Int = 0) {
        val tipos = JSONArray()
            .put(JSONObject().put("tipo", "foto").put("bytes", 500 * MB).put("cantidad", 120).put("apagado", false).put("topeArchivo", 0).put("topeTotal", 0))
            .put(JSONObject().put("tipo", "video").put("bytes", uso - 500 * MB).put("cantidad", 9).put("apagado", false).put("topeArchivo", 0).put("topeTotal", 0))
        motor.estadoAlmJson = JSONObject().put("topeTotal", tope).put("uso", uso).put("libre", 3 * GB).put("total", 10 * GB).put("ahorradoBytes", 200 * MB).put("ahorradoArchivos", ahorro)
            .put("conservarPortada", true).put("sinPortada", sinPortada).put("tipos", tipos).toString()
    }

    private fun espacio() = regla.setContent { BotChatTema { PantallaEspacio({}, {}, ajustes = ajustes) } }

    // ---------- almacenamiento ----------

    @Test fun elMedidorSeVeRojoPasadoDelTopeYAvisaDeVideosSinPortadaYDuplicados() {
        almacen(tope = 1 * GB, uso = 2 * GB, sinPortada = 3, ahorro = 4)
        espacio()
        regla.esperaEtiqueta("medidor_barra_rojo")
        assertTrue(hay("aviso_pasado_tope")); assertTrue(hay("aviso_sin_portada")); assertTrue(hay("ahorro_duplicados")); assertTrue(hay("aviso_galeria"))
        regla.esperaTexto("2 GB de 1 GB", substring = true)
        regla.esperaTexto("3 videos esperan su portada", substring = true)
    }

    @Test fun cambiarElTopeTotalConUnToque() {
        almacen(tope = 5 * GB, uso = 1 * GB)
        espacio()
        regla.esperaEtiqueta("medidor_barra_verde")
        toca("tope_total_${10 * GB}")
        regla.espera { llamo("tope:${10 * GB}") }
        toca("tope_total_0")
        regla.espera { llamo("tope:0") }
    }

    @Test fun otroTopeSePideEnUnaHojaEnGigabytes() {
        almacen(tope = 5 * GB, uso = 1 * GB)
        espacio()
        toca("tope_total_otro")
        escribe("otro_tope_campo", "8")
        toca("otro_tope_aceptar")
        regla.espera { llamo("tope:${8 * GB}") }
    }

    @Test fun cadaTipoTieneInterruptorWifiYTopes() {
        almacen(tope = 5 * GB, uso = 1 * GB)
        espacio()
        toca("sw_tipo_foto")
        regla.espera { "foto" in ajustes.descargas.value.apagados }
        assertTrue(JSONObject(Ajustes.jsonDescargas(ajustes.descargas.value)).getJSONObject("foto").getBoolean("apagada"))
        toca("sw_wifi_video") // el video baja solo con Wi-Fi de fábrica: al tocar se permiten también los datos
        regla.espera { ajustes.descargas.value.video.datos }
        toca("tope_archivo_video")
        toca("hoja_tope_${100 * MB}")
        regla.espera { ajustes.descargas.value.topeArchivoMb["video"] == 100 }
        assertEquals(100 * MB, JSONObject(Ajustes.jsonDescargas(ajustes.descargas.value)).getJSONObject("topeArchivo").getLong("video"))
        toca("tope_tipo_audio")
        toca("hoja_tope_${2 * GB}")
        regla.espera { ajustes.descargas.value.topeTipoMb["audio"] == 2048 }
    }

    @Test fun lasReglasDeRedBateriaYPortadaSeGuardan() {
        almacen(tope = 5 * GB, uso = 1 * GB)
        espacio()
        regla.esperaEtiqueta("sw_Archivos de más de 50 MB solo con Wi-Fi")
        toca("sw_Archivos de más de 50 MB solo con Wi-Fi")
        regla.espera { !ajustes.descargas.value.soloWifiGrandes }
        toca("sw_Esperar si la batería está baja")
        regla.espera { !ajustes.descargas.value.pausaBateria }
        val j = JSONObject(Ajustes.jsonDescargas(ajustes.descargas.value))
        assertFalse(j.getBoolean("soloWifiGrandes")); assertFalse(j.getBoolean("pausaBateria"))
        toca("sw_Conservar la miniatura al borrar videos")
        regla.espera { llamo("portada:false") }
    }

    @Test fun limpiarConFiltrosPrimeroEstimaYLuegoBorraConConfirmacion() {
        almacen(tope = 5 * GB, uso = 1 * GB)
        motor.liberar = """{"bytes":52428800,"cantidad":3}"""
        espacio()
        toca("limpiar_tipo_video")
        toca("limpiar_persona")
        toca("limpiar_elegir_$ana")
        toca("limpiar_fecha_m30")
        assertFalse(hay("limpieza_estimado"))
        toca("limpieza_estimar")
        regla.esperaEtiqueta("limpieza_estimado")
        regla.esperaTexto("Se liberarían 50 MB (3 archivos).")
        val f = JSONObject(motor.llamadas.last { it.startsWith("estimar:") }.removePrefix("estimar:"))
        assertEquals("video", f.getString("categoria")); assertEquals(ana, f.getString("persona")); assertTrue(f.getLong("hasta") > 0); assertEquals(0L, f.getLong("desde"))
        toca("limpieza_borrar")
        regla.esperaTexto("No se puede deshacer.", substring = true)
        toca("limpieza_cancelar")
        assertFalse(llamo("borrarArchivos"))
        toca("limpieza_borrar"); toca("limpieza_confirmar")
        regla.espera { llamo("borrarArchivos") }
    }

    @Test fun elAvisoDelTopeOfreceSubirloOQuitarlo() {
        val e = EstadoAlmacen(1 * GB, 3 * GB, 0, 0, 0, 0, true, 0, emptyList())
        var subido = -1L; var entendido = false
        regla.setContent { BotChatTema { AvisoTopeAlmacen(e, { subido = it }, { entendido = true }) } }
        regla.onNodeWithTag("aviso_tope", useUnmergedTree = true).assertExists()
        regla.onNodeWithText("Subir el tope a 5 GB").assertExists()
        tocaD("aviso_tope_subir")
        assertEquals(5 * GB, subido)
        tocaD("aviso_tope_sin_limite")
        assertEquals(0L, subido)
        tocaD("aviso_tope_ok")
        assertTrue(entendido)
    }

    // ---------- auditoría y privacidad ----------

    private fun auditoria() = regla.setContent { BotChatTema { PantallaAuditoria(bot, {}, {}) } }

    @Test fun laAuditoriaDiceQueSeGuardaYPermiteApagarloYElegirLaRetencion() {
        auditoria()
        regla.esperaEtiqueta("texto_auditoria")
        regla.esperaTexto("no se recupera el historial anterior", substring = true)
        assertTrue(hay("texto_reglas_privacidad"))
        toca("ret_30")
        regla.espera { llamo("fijarAuditoria:true:30") }
        toca("sw_auditoria_global")
        regla.espera { llamo("fijarAuditoria:false:-1") }
    }

    @Test fun cadaChatTieneSuInterruptorYSuTiempo() {
        motor.auditoriaChatsJson = JSONArray()
            .put(JSONObject().put("chat", ana).put("nombre", "Ana Pérez").put("esGrupo", false).put("audit", -1).put("efectivo", true).put("retDias", -1).put("retEfectiva", 90))
            .put(JSONObject().put("chat", g1).put("nombre", "Vecinos del 5").put("esGrupo", true).put("audit", 0).put("efectivo", false).put("retDias", 30).put("retEfectiva", 30)).toString()
        auditoria()
        regla.esperaEtiqueta("auditoria_chat_$g1")
        regla.esperaTexto("No guardo el texto", substring = true)
        toca("sw_auditoria_$g1")
        regla.espera { llamo("auditoriaChat:$g1:1:30") }
        toca("sw_auditoria_$ana")
        regla.espera { llamo("auditoriaChat:$ana:0:-1") }
        toca("auditoria_ret_$ana")
        toca("auditoria_ret_${ana}_0")
        regla.espera { llamo("auditoriaChat:$ana:-1:0") }
    }

    @Test fun conLaRetencionPendienteSeAvisaYNoSeDaPorElegida() {
        motor.auditoriaJson = """{"defecto":true,"retDias":90,"retPendiente":true,"avisoVisto":false}"""
        auditoria()
        regla.esperaEtiqueta("aviso_ret_pendiente")
        regla.esperaTexto("no se borra ningún mensaje", substring = true)
    }

    @Test fun elAvisoDeLaPrimeraVezPideElegir30_90OSiempre() {
        var eleg = -1
        regla.setContent { BotChatTema { AvisoAuditoria(true, { eleg = it }, {}) } }
        regla.onNodeWithTag("aviso_auditoria", useUnmergedTree = true).assertExists()
        regla.onNodeWithText("Hasta que elijas, no se borra nada.", substring = true).assertExists()
        regla.onNodeWithTag("aviso_auditoria_ok", useUnmergedTree = true).assertDoesNotExist()
        tocaD("aviso_ret_0")
        assertEquals(0, eleg)
        tocaD("aviso_ret_30"); assertEquals(30, eleg)
    }

    @Test fun elAvisoSimpleSoloDiceLoQueSeGuarda() {
        var ok = false
        regla.setContent { BotChatTema { AvisoAuditoria(false, {}, { ok = true }) } }
        regla.onNodeWithText("Solo guarda lo que llega desde que lo activas", substring = true).assertExists()
        regla.onNodeWithTag("aviso_ret_30", useUnmergedTree = true).assertDoesNotExist()
        tocaD("aviso_auditoria_ok"); assertTrue(ok)
    }

    @Test fun borrarTodoDeUnaPersonaPideConfirmar() {
        auditoria()
        toca("borrar_todo_persona")
        toca("borrar_persona_$ana")
        regla.esperaTexto("¿Borrar todo de Ana Pérez?")
        regla.esperaEtiqueta("aviso_sin_deshacer")
        toca("confirmar_borrar_todo_cancelar")
        assertFalse(llamo("borrarTodo"))
        toca("borrar_todo_persona"); toca("borrar_persona_$ana"); toca("confirmar_borrar_todo")
        regla.espera { llamo("borrarTodo:$ana") }
    }

    @Test fun unMensajeSinTextoDiceContenidoNoGuardadoYSeMarcaDelHistorial() {
        motor.chat = Apoyo.chatJson(ana, "Ana Pérez").toString()
        motor.mensajes = Apoyo.arreglo(
            Apoyo.msgJson("m1", ahora, "", remitente = "Ana", remitenteId = "ana").put("olvidado", true).put("delHistorial", true),
            Apoyo.msgJson("m2", ahora - 5, "hola", remitente = "Ana", remitenteId = "ana"),
        )
        regla.setContent { BotChatTema { PantallaChat(ana, {}, {}, {}, {}) } }
        regla.esperaTexto("hola")
        regla.esperaEtiqueta("contenido_no_guardado")
        assertTrue(hay("del_historial_m1")); assertFalse(hay("del_historial_m2"))
    }

    @Test fun desdeElMenuDeSeleccionDelChatSeMarcaUnMensaje() {
        val id = motor.crearEtiquetaEmoji(bot, "Importante", "#E5484D", "❗")
        motor.chat = Apoyo.chatJson(ana, "Ana Pérez").toString()
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("m1", ahora, "para marcar", remitente = "Ana", remitenteId = "ana"))
        regla.setContent { BotChatTema { PantallaChat(ana, {}, {}, {}, {}) } }
        regla.esperaTexto("para marcar")
        regla.onNodeWithTag("burbuja_m1").performTouchInput { longClick() }
        toca("accion_marcar")
        toca("marcar_rapido_$id")
        regla.espera { llamo("marcar:$ana:m1:$id") }
        toca("marcar_rapido_$id")
        regla.espera { llamo("quitarMarca:$ana:m1:$id") }
    }

    // ---------- silencios ----------

    private fun silencio(color: String, usado: Int, hasta: Long = 0) = JSONArray().put(
        JSONObject().put("id", 1).put("persona", ana).put("nombre", "Ana Pérez").put("alcance", "todos").put("grupos", JSONArray().put(g1)).put("hastaTs", hasta).put("creadoTs", ahora)
            .put("origen", "manual").put("avisar", true).put("texto", "").put("activo", true)
            .put("contador", JSONObject().put("usado", usado).put("tope", 10).put("color", color).put("total", usado).put("avisos", 0).put("grupos", JSONArray())),
    ).toString()

    @Test fun laListaDeSilenciosMuestraAQuienHastaCuandoYSuContador() {
        motor.silenciosGruposJson = silencio("amarillo", 8, ahora + 7200)
        regla.setContent { BotChatTema { PantallaSilencios(bot, {}, {}) } }
        regla.esperaEtiqueta("silencio_1")
        regla.esperaTexto("Ana Pérez")
        regla.esperaTexto("En todos mis grupos", substring = true)
        regla.esperaEtiqueta("barra_silencio_amarillo")
        regla.esperaTexto("Hoy van 8 de 10 mensajes borrados")
    }

    @Test fun quitarUnSilencioSePuedeDeshacer() {
        motor.silenciosGruposJson = silencio("verde", 1, ahora + 3600)
        regla.setContent { BotChatTema { PantallaSilencios(bot, {}, {}) } }
        toca("silencio_quitar_1")
        regla.espera { llamo("quitarSilencioGrupos:$ana") }
        regla.esperaTexto("Deshacer")
        regla.onNodeWithText("Deshacer").performClick()
        regla.espera { llamo("silenciarGrupos:$ana") }
    }

    @Test fun ponerElContadorEnCeroDesdeLaLista() {
        motor.silenciosGruposJson = silencio("rojo", 10)
        regla.setContent { BotChatTema { PantallaSilencios(bot, {}, {}) } }
        regla.esperaEtiqueta("barra_silencio_rojo")
        toca("silencio_cero_1")
        regla.espera { llamo("contadorCero:$ana") }
    }

    @Test fun unaSancionPendienteSeResuelveConExpulsarODejarlo() {
        motor.sancionesJson = JSONArray()
            .put(JSONObject().put("id", 5).put("origen", "silencio").put("persona", ana).put("grupo", g1).put("avisoId", 9).put("creadoTs", ahora).put("nombre", "Ana Pérez").put("nombreGrupo", "Vecinos del 5"))
            .toString()
        regla.setContent { BotChatTema { PantallaSilencios(bot, {}, {}) } }
        regla.esperaEtiqueta("sancion_5")
        regla.esperaTexto("¿Expulsamos a Ana Pérez?")
        toca("sancion_dejarlo_5")
        regla.espera { llamo("sancion:5:false") }
        toca("sancion_expulsar_5")
        regla.esperaEtiqueta("aviso_sin_deshacer")
        assertFalse(llamo("sancion:5:true"))
        toca("confirmar_expulsar")
        regla.espera { llamo("sancion:5:true") }
    }

    @Test fun losAjustesDelSilencioGuardanSancionNumerosHoraYTextos() {
        motor.ajustesSilJson = JSONObject()
            .put("ajustes", JSONObject().put("sil_tope", "10").put("sil_sancion", "confirmar").put("sil_reinicio_hora", "00:00"))
            .put("textos", JSONObject().put("inicio", JSONObject().put("actual", "Te silencié").put("defecto", "Te silencié")).put("noadmin", JSONObject().put("actual", "Estás silenciado").put("defecto", "Estás silenciado"))).toString()
        regla.setContent { BotChatTema { PantallaAjustesSilencio(bot, {}, ajustes) } }
        regla.esperaEtiqueta("pantalla_ajustes_silencio")
        toca("sancion_avisar"); regla.espera { llamo("ajusteSilencio:sil_sancion:avisar") }
        toca("sancion_expulsar"); regla.espera { llamo("ajusteSilencio:sil_sancion:expulsar") }
        toca("hora_04:00"); regla.espera { llamo("ajusteSilencio:sil_reinicio_hora:04:00") }
        toca("num_sil_tope"); escribe("hoja_numero_campo", "12"); toca("hoja_numero_aceptar")
        regla.espera { llamo("ajusteSilencio:sil_tope:12") }
        toca("texto_inicio"); escribe("hoja_texto_campo", "Aviso nuevo"); toca("hoja_texto_aceptar")
        regla.espera { llamo("textoSilencio:inicio") }
        toca("def_tiempo_10080"); regla.espera { ajustes.silencioTiempoDefecto.value == 10080 }
        toca("def_alcance_1"); regla.espera { ajustes.silencioAlcanceDefecto.value == "uno" }
        toca("silencio_avanzado")
        regla.esperaEtiqueta("num_sil_avisos_hora")
    }

    // ---------- advertencias ----------

    private val tiposJson = JSONArray().put(
        JSONObject().put("id", 1).put("nombre", "Spam").put("emoji", "🚫").put("color", "#F59E28").put("posicion", 0).put("caducidadDias", -1).put("reglaId", 0).put("activo", true)
            .put("escalera", JSONArray()
                .put(JSONObject().put("nivel", 1).put("accion", "avisar").put("minutos", 0).put("alcance", "grupo").put("aviso", ""))
                .put(JSONObject().put("nivel", 2).put("accion", "silenciar").put("minutos", 60).put("alcance", "grupo").put("aviso", ""))
                .put(JSONObject().put("nivel", 3).put("accion", "expulsar").put("minutos", 0).put("alcance", "grupo").put("aviso", ""))),
    ).toString()

    @Test fun losTiposMuestranSuEscaleraEnPalabras() {
        motor.tiposAdvJson = tiposJson
        regla.setContent { BotChatTema { PantallaTiposAdvertencia(bot, {}, {}) } }
        regla.esperaEtiqueta("tipo_1")
        regla.esperaTexto("1.ª avisarle · 2.ª silenciarlo 1 h · 3.ª expulsarlo (con tu permiso)")
        assertTrue(hay("nuevo_tipo"))
    }

    @Test fun editarUnTipoCambiaLaEscaleraYLaGuarda() {
        motor.tiposAdvJson = tiposJson
        var volvio = false
        regla.setContent { BotChatTema { EditorTipoAdvertencia(bot, 1, { volvio = true }) } }
        regla.esperaEtiqueta("editor_tipo")
        toca("paso_1_silencio_largo")
        toca("agregar_paso")
        regla.esperaEtiqueta("paso_4")
        toca("tipo_caduca_90")
        toca("guardar_tipo")
        regla.espera { motor.tiposGuardados.isNotEmpty() }
        val t = JSONObject(motor.tiposGuardados.last())
        assertEquals("Spam", t.getString("nombre")); assertEquals(90, t.getInt("caducidadDias"))
        val e = t.getJSONArray("escalera")
        assertEquals(4, e.length()); assertEquals("silencio_largo", e.getJSONObject(0).getString("accion")); assertEquals(4, e.getJSONObject(3).getInt("nivel"))
        regla.espera { volvio }
    }

    @Test fun crearUnTipoNuevoNecesitaNombre() {
        regla.setContent { BotChatTema { EditorTipoAdvertencia(bot, 0, {}) } }
        regla.esperaEtiqueta("editor_tipo")
        regla.onNodeWithTag("guardar_tipo", useUnmergedTree = true).assertIsNotEnabled()
        escribe("tipo_nombre", "Publicidad")
        toca("guardar_tipo")
        regla.espera { motor.tiposGuardados.isNotEmpty() }
        assertEquals(0L, JSONObject(motor.tiposGuardados.last()).getLong("id"))
    }

    @Test fun lasReglasDelGrupoSeAgreganYSeMueven() {
        motor.reglasPorGrupo[g1] = JSONArray()
            .put(JSONObject().put("id", 1).put("grupo", g1).put("pos", 1).put("titulo", "Sin spam").put("texto", "No mandes cadenas"))
            .put(JSONObject().put("id", 2).put("grupo", g1).put("pos", 2).put("titulo", "Respeto").put("texto", "")).toString()
        regla.setContent { BotChatTema { PantallaReglasGrupo(bot, g1, "Vecinos del 5", {}) } }
        regla.esperaEtiqueta("regla_1")
        regla.esperaTexto("No mandes cadenas")
        toca("regla_bajar_1"); regla.espera { llamo("moverRegla:1:1") }
        toca("regla_subir_2"); regla.espera { llamo("moverRegla:2:-1") }
        toca("agregar_regla"); escribe("regla_titulo", "Nada de ventas"); toca("guardar_regla")
        regla.espera { llamo("guardarRegla:$g1:0:Nada de ventas:3") }
        toca("regla_borrar_1"); regla.esperaEtiqueta("aviso_sin_deshacer"); toca("confirmar_borrar_regla")
        regla.espera { llamo("borrarRegla:1") }
    }

    @Test fun laCaducidadGeneralYLosAtajosEstanArribaDeAdvertencias() {
        var tipos = false; var reglas = false
        regla.setContent { BotChatTema { PantallaAdvertencias(bot, {}, {}, { tipos = true }, { reglas = true }) } }
        regla.esperaEtiqueta("cabecera_advertencias")
        toca("caducidad_general_90")
        regla.espera { llamo("ajusteSilencio:adv_caducidad_dias:90") }
        toca("adv_tipos"); assertTrue(tipos)
        toca("adv_reglas"); assertTrue(reglas)
        assertTrue(hay("sin_advertencias"))
    }

    // ---------- etiquetas y marcados ----------

    @Test fun lasEtiquetasLlevanEmojiYNoTienenLimite() {
        regla.setContent { BotChatTema { PantallaEtiquetas(bot, {}, {}) } }
        repeat(3) { n ->
            toca("nueva_etiqueta")
            escribe("nombre_etiqueta", "Etiqueta $n"); escribe("emoji_etiqueta", "🌟")
            toca("guardar_etiqueta")
            regla.espera { motor.etiquetasJson.size == n + 1 }
        }
        assertEquals("🌟", motor.etiquetasJson.first().getString("emoji"))
        regla.esperaEtiqueta("emoji_etiqueta_${motor.etiquetasJson.first().getLong("id")}")
        assertTrue(hay("ver_marcados"))
    }

    @Test fun marcadosTienePestanasFiltroPorEtiquetaYContenidoNoGuardado() {
        val id = motor.crearEtiquetaEmoji(bot, "Importante", "#E5484D", "❗")
        motor.marcadosJson = JSONArray()
            .put(JSONObject().put("tipo", "mensaje").put("chat", ana).put("chatNombre", "Ana Pérez").put("msgId", "m1").put("msgTipo", "texto").put("texto", "").put("olvidado", true)
                .put("esMio", false).put("remitente", "Ana").put("persona", ana).put("nombre", "Ana").put("foto", "").put("ts", ahora).put("etiquetas", JSONArray().put(id)).put("nota", "llamar mañana"))
            .toString()
        var ir = ""
        regla.setContent { BotChatTema { PantallaMarcados(bot, {}, {}, { c, m -> ir = "$c|$m" }, {}, {}) } }
        regla.esperaEtiqueta("marcado_mensaje_m1")
        regla.esperaTexto("Contenido no guardado")
        regla.esperaTexto("Nota: llamar mañana")
        listOf("mensaje", "persona", "chat", "grupo").forEach { assertTrue(it, hay("pestana_marcados_$it")) }
        assertTrue(hay("filtro_marcados_$id"))
        toca("filtro_marcados_$id")
        val guardado = motor.marcadosJson
        motor.marcadosJson = "[]"
        toca("pestana_marcados_grupo")
        regla.esperaEtiqueta("sin_marcados")
        motor.marcadosJson = guardado
        toca("pestana_marcados_mensaje")
        toca("marcado_mensaje_m1")
        assertEquals("$ana|m1", ir)
    }

    // ---------- video eliminado ----------

    @Test fun laTarjetaDeVideoEliminadoMuestraDuracionTamanoFechaYRemitente() {
        val m = Msg(
            "v1", ana, "ana", "Ana", false, "video", "", ahora, false, false, "borrado", 20 * MB, "video/mp4", "", "", "", 640, 480, 75, false, false,
            portada = "portadas/v1.enc", borradoTs = ahora, borradoMotivo = "tope",
        )
        regla.setContent { BotChatTema { TarjetaVideoEliminado(m) } }
        regla.esperaEtiqueta("tarjeta_video_eliminado")
        regla.esperaTexto("Video eliminado")
        regla.esperaTexto("1:15")
        regla.esperaTexto("20 MB", substring = true)
        regla.esperaTexto("Ana", substring = true)
        regla.esperaTexto("Se borró para dejar espacio", substring = true)
    }
}
