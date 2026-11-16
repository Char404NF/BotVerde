package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextClearance
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.text.input.TextFieldValue
import com.botchat.app.medios.GrabacionVoz
import com.botchat.app.medios.GrabadorVoz
import com.botchat.app.medios.Grabadores
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.EditorMensajeAuto
import com.botchat.app.ui.PantallaMensajesAuto
import com.botchat.app.ui.PantallaRegistroAuto
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
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import java.io.File

private class GrabadorDeMentira(val archivo: File) : GrabadorVoz {
    override fun iniciar() = true
    override fun terminar(): GrabacionVoz? { archivo.writeBytes(byteArrayOf(1, 2, 3)); return GrabacionVoz(archivo, 5, ByteArray(64) { 30 }, true, "audio/ogg; codecs=opus") }
    override fun cancelar() {}
}

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h4000dp-xxhdpi")
class MensajesAutoUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }
    @After fun despues() { Grabadores.fabrica = { com.botchat.app.medios.GrabadorMediaRecorder(it) } }

    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun nodo(tag: String) = regla.onNodeWithTag(tag, useUnmergedTree = true)
    private fun guardado(): JSONObject = JSONObject(motor.llamadas.last { it.startsWith("guardarAuto:") }.removePrefix("guardarAuto:"))
    private fun cfg(siempre: Boolean = true) = JSONObject().put("cuando", JSONObject().put("siempre", siempre)).put("quien", JSONObject().put("personas", "todos").put("privados", true).put("grupos", false))
        .put("repeticion", JSONObject().put("cada", 6).put("unidad", "horas"))
    private fun existente(id: Long, tipo: String, nombre: String, texto: String, activo: Boolean = true, config: JSONObject = cfg()) {
        motor.mensajesAuto += JSONObject().put("id", id).put("tipo", tipo).put("nombre", nombre).put("texto", texto).put("config", config).put("adjuntos", JSONArray()).put("activo", activo).put("orden", id)
    }

    // ---------- lista ----------

    @Test fun laListaMuestraCadaTipoConInterruptoresYEstado() {
        existente(1, "recibido", "Ya vi", "Recibí tu mensaje")
        existente(2, "ausencia", "Fuera de horario", "Estamos fuera", activo = false)
        existente(3, "atajo", "gracias", "Gracias")
        var editado: Pair<Long, String>? = null
        regla.setContent { BotChatTema { PantallaMensajesAuto(bot, {}, { id, t -> editado = id to t }) } }
        regla.esperaEtiqueta("mensaje_auto_1")
        for (t in listOf("Bienvenida", "Ausencia", "Recibido", "Cierre", "Respuestas rápidas")) regla.esperaTexto(t)
        nodo("estado_auto_1").assertIsDisplayed(); regla.esperaTexto("Activo")
        regla.esperaTexto("Apagado")
        regla.esperaTexto("/gracias")
        regla.esperaTexto("Todavía no hay ninguno.", substring = true) // bienvenida y cierre
        nodo("auto_activo_1").performScrollTo().performClick()
        regla.espera { llamo("autoActivo:1:false") }
        nodo("auto_activo_2").performScrollTo().performClick()
        regla.espera { llamo("autoActivo:2:true") }
        nodo("mensaje_auto_2").performScrollTo().performClick()
        assertEquals(2L to "ausencia", editado)
        nodo("nuevo_bienvenida").performScrollTo().performClick()
        assertEquals(0L to "bienvenida", editado)
    }

    @Test fun borrarUnMensajePideConfirmacion() {
        existente(1, "recibido", "Ya vi", "Recibí tu mensaje")
        regla.setContent { BotChatTema { PantallaMensajesAuto(bot, {}, { _, _ -> }) } }
        regla.esperaEtiqueta("borrar_auto_1")
        nodo("borrar_auto_1").performScrollTo().performClick()
        regla.esperaTexto("También se borran sus archivos", substring = true)
        assertFalse(llamo("borrarMensajeAuto"))
        nodo("confirmar_borrar_auto_msg").performClick()
        regla.espera { llamo("borrarMensajeAuto:1") }
    }

    @Test fun unaReglaQueContestaATodoSeAvisa() {
        motor.autoConflictos = listOf("Saludo")
        regla.setContent { BotChatTema { PantallaMensajesAuto(bot, {}, { _, _ -> }) } }
        regla.esperaEtiqueta("conflicto_auto")
        regla.esperaTexto("La regla “Saludo” contesta a cualquier mensaje", substring = true)
    }

    @Test fun losAjustesGeneralesSeGuardanYSeValidan() {
        regla.setContent { BotChatTema { PantallaMensajesAuto(bot, {}, { _, _ -> }) } }
        regla.esperaEtiqueta("auto_general")
        nodo("auto_general").performClick()
        regla.espera { llamo("ajustePerm:auto_activo:0") }
        nodo("atajos_general").performScrollTo().performClick()
        regla.espera { llamo("ajustePerm:atajos_activo:0") }
        nodo("atajo_modo_enviar").performScrollTo().performClick()
        regla.espera { llamo("ajustePerm:atajo_modo:enviar") }
        nodo("auto_tope").performScrollTo().performTextClearance()
        nodo("auto_tope").performTextInput("45")
        regla.espera { llamo("ajustePerm:auto_tope_hora:45") }
        regla.esperaTexto("Pasado el tope, los mensajes salen solo con texto, sin archivos", substring = true)
    }

    @Test fun elRegistroNoLlevaTextosYDiceQueArchivoFallo() {
        motor.registroAutoJson = JSONArray().put(JSONObject().put("id", 1).put("ts", System.currentTimeMillis() / 1000).put("autoId", 1).put("tipo", "recibido").put("chat", "ab12cd")
            .put("resultado", "parcial").put("adjOk", 1).put("adjFallo", 1).put("motivo", "archivo_ilegible").put("texto", "Enviado; 1 archivo(s) no salieron")).toString()
        regla.setContent { BotChatTema { PantallaRegistroAuto(bot, {}) } }
        regla.esperaEtiqueta("lista_registro_auto")
        regla.esperaTexto("Recibido · Enviado; 1 archivo(s) no salieron")
        regla.esperaTexto("1 no salieron", substring = true)
        regla.esperaTexto("no se pudo leer el archivo guardado", substring = true)
        regla.esperaTexto("Chat ab12cd", substring = true)
    }

    // ---------- editor ----------

    private fun editor(id: Long = 0, tipo: String = "recibido", alVolver: () -> Unit = {}) = regla.setContent { BotChatTema { EditorMensajeAuto(bot, id, tipo, alVolver) } }

    @Test fun laBarraDeFormatoPoneLosMarcadoresDeWhatsAppYLasVariables() {
        editor()
        regla.esperaEtiqueta("auto_texto")
        nodo("auto_texto").performTextInput("hola")
        // sin selección, cada botón mete el par de marcadores y deja el cursor en medio
        nodo("fmt_negrita").performScrollTo().performClick()
        nodo("fmt_cursiva").performScrollTo().performClick()
        nodo("fmt_tachado").performScrollTo().performClick()
        nodo("fmt_mono").performScrollTo().performClick()
        nodo("fmt_var_nombre").performScrollTo().performClick()
        nodo("fmt_var_hora").performScrollTo().performClick()
        nodo("fmt_var_dia").performScrollTo().performClick()
        nodo("fmt_var_fecha").performScrollTo().performClick()
        nodo("fmt_var_horario").performScrollTo().performClick()
        // cada par de marcadores se mete entre los anteriores (el cursor queda en medio), y las variables caen en el centro
        regla.esperaTexto("hola*_~```{nombre}{hora}{dia}{fecha}{horario}```~_*", substring = true)
    }

    @Test fun listaNumeradaCitaYEmojiSeInsertan() {
        editor()
        regla.esperaEtiqueta("auto_texto")
        nodo("auto_texto").performTextInput("uno")
        nodo("fmt_lista").performScrollTo().performClick()
        regla.esperaTexto("- uno", substring = true)
        nodo("fmt_lista").performClick() // otra vez la quita
        regla.espera { regla.onAllNodesWithTextSafe("- uno", substring = true) == 0 }
        nodo("fmt_numerada").performClick()
        regla.esperaTexto("1. uno", substring = true)
        nodo("fmt_numerada").performClick()
        nodo("fmt_cita").performClick()
        regla.esperaTexto("> uno", substring = true)
        nodo("fmt_emoji").performClick()
        regla.esperaEtiqueta("fmt_emoji_👍")
        nodo("fmt_emoji_👍").performClick()
        regla.esperaTexto("👍", substring = true)
    }

    @Test fun laVistaPreviaEsUnaBurbujaConElMismoFormatoQueWhatsApp() {
        editor()
        regla.esperaEtiqueta("auto_texto")
        nodo("auto_texto").performTextInput("Hola {nombre}, *gracias*")
        regla.espera { llamo("probarAuto") }
        // la burbuja usa lo que armó Go (con el nombre de ejemplo) y le quita los asteriscos para dibujar negrita
        regla.esperaTexto("Hola María, gracias", substring = true)
        nodo("vista_previa_auto").assertIsDisplayed()
        nodo("previa_etiqueta").assertIsDisplayed()
        regla.esperaTexto("Automático · recibido")
        assertEquals(0, regla.onAllNodesWithTextSafe("*gracias*"))
    }

    @Test fun laPlantillaLlenaElTextoYLaConfiguracion() {
        editor(tipo = "ausencia")
        regla.esperaEtiqueta("plantilla_0")
        nodo("plantilla_0").performScrollTo().performClick()
        regla.esperaTexto("estamos fuera de horario", substring = true)
        nodo("guardar_auto").performScrollTo().assertIsEnabled()
    }

    @Test fun guardarMandaTodoLoElegidoYVuelve() {
        var volvio = false
        editor(tipo = "ausencia", alVolver = { volvio = true })
        regla.esperaEtiqueta("auto_texto")
        nodo("guardar_auto").performScrollTo().assertIsNotEnabled()
        nodo("auto_nombre").performTextInput("Fuera de horario")
        nodo("auto_texto").performTextInput("Hola {nombre}, estamos fuera de horario")
        // horario por día con dos rangos: lunes a viernes por defecto; se agrega sábado con un solo rango
        nodo("dia_on_6").performScrollTo().performClick()
        nodo("horario_modo_fuera").performScrollTo().performClick()
        nodo("dia_6_agrega_r2").performScrollTo().performClick()
        // vacaciones y una fecha especial anual
        nodo("vac_agrega").performScrollTo().performClick()
        nodo("fecha_agrega").performScrollTo().performClick()
        nodo("fecha_dia_0").performScrollTo().performTextClearance()
        nodo("fecha_dia_0").performTextInput("24")
        // a quién y cada cuánto
        nodo("quien_desconocidos").performScrollTo().performClick()
        nodo("quien_grupos").performScrollTo().performClick()
        nodo("quien_excepto").performScrollTo().performTextInput("52 55 1111 1111")
        nodo("rep_cada").performScrollTo().performTextClearance()
        nodo("rep_cada").performTextInput("8")
        nodo("rep_unidad_horas").performClick()
        nodo("guardar_auto").performScrollTo().assertIsEnabled().performClick()
        regla.espera { volvio }
        val j = guardado()
        assertEquals("ausencia", j.getString("tipo"))
        assertEquals("Fuera de horario", j.getString("nombre"))
        assertEquals("Hola {nombre}, estamos fuera de horario", j.getString("texto"))
        val c = j.getJSONObject("config")
        val cu = c.getJSONObject("cuando")
        assertEquals("fuera", cu.getString("horarioModo"))
        val dias = cu.getJSONArray("horario")
        assertEquals(6, dias.length()) // lunes a viernes y sábado
        assertEquals(2, dias.getJSONObject(5).getJSONArray("rangos").length())
        assertEquals(1, cu.getJSONArray("vacaciones").length())
        assertEquals(24, cu.getJSONArray("fechas").getJSONObject(0).getInt("dia"))
        val q = c.getJSONObject("quien")
        assertEquals("desconocidos", q.getString("personas"))
        assertTrue(q.getBoolean("grupos"))
        assertEquals("525511111111", q.getJSONArray("excepto").getString(0))
        assertEquals(8, c.getJSONObject("repeticion").getInt("cada"))
    }

    @Test fun losErroresDelGuardadoSeMuestranYNoSalenDeLaPantalla() {
        var volvio = false
        motor.personaFalla = "Elige cuándo se manda: fuera de horario, vacaciones o una fecha especial."
        editor(tipo = "ausencia", alVolver = { volvio = true })
        regla.esperaEtiqueta("auto_texto")
        nodo("auto_texto").performTextInput("hola")
        nodo("guardar_auto").performScrollTo().performClick()
        regla.espera { llamo("guardarAuto") }
        regla.waitForIdle()
        assertFalse(volvio)
        nodo("guardar_auto").performScrollTo().assertIsEnabled() // se puede corregir y reintentar
    }

    @Test fun unaAusenciaSinCuandoLoAvisaEnPantalla() {
        editor(tipo = "ausencia")
        regla.esperaEtiqueta("auto_texto")
        nodo("plantilla_0").performScrollTo()
        nodo("cuando_horario").performScrollTo().performClick() // apaga el horario que trae
        regla.esperaEtiqueta("cuando_falta")
        nodo("cuando_siempre").performScrollTo().performClick()
        regla.espera { !hay("cuando_falta") }
    }

    @Test fun laBienvenidaOfreceLosDiasDeSilencio() {
        editor(tipo = "bienvenida")
        regla.esperaEtiqueta("auto_texto")
        nodo("auto_texto").performTextInput("Hola")
        for (d in listOf(0, 7, 14, 30)) nodo("silencio_$d").performScrollTo().assertIsDisplayed()
        nodo("silencio_14").performClick()
        nodo("guardar_auto").performScrollTo().performClick()
        regla.espera { llamo("guardarAuto") }
        assertEquals(14, guardado().getJSONObject("config").getInt("silencioDias"))
    }

    @Test fun elEstiloDeLetraEstaEnAvanzadoConSuAvisoYConvierteLaSeleccion() {
        editor()
        regla.esperaEtiqueta("auto_texto")
        assertFalse(hay("estilo_letra_aviso"))
        nodo("avanzado_auto").performScrollTo().performClick()
        regla.esperaEtiqueta("estilo_letra_aviso")
        regla.esperaTexto("pueden verse distinto en otros teléfonos", substring = true)
        regla.esperaTexto("no se pueden buscar", substring = true)
        regla.esperaTexto("lectores de pantalla", substring = true)
        for (e in listOf("negrita", "cursiva", "script", "gotica", "doble", "maquina", "burbujas", "versalitas", "normal")) nodo("estilo_letra_$e").performScrollTo().assertIsDisplayed()
        // sin texto seleccionado no hace nada (avisa con un mensaje)
        nodo("auto_texto").performTextInput("Hola mundo")
        nodo("estilo_letra_negrita").performScrollTo().performClick()
        regla.waitForIdle()
        assertEquals(0, regla.onAllNodesWithTextSafe("𝐇𝐨𝐥𝐚", substring = true))
    }

    @Test fun grabarUnaNotaDeVozLaSubeYSePuedeQuitar() {
        shadowOf(Apoyo.ctx() as android.app.Application).grantPermissions(android.Manifest.permission.RECORD_AUDIO)
        Grabadores.fabrica = { GrabadorDeMentira(File(Apoyo.ctx().cacheDir, "voz_auto.ogg")) }
        editor()
        regla.esperaEtiqueta("grabar_voz_auto")
        nodo("grabar_voz_auto").performScrollTo().performClick() // empieza
        regla.esperaTexto("Detener y guardar", substring = true)
        nodo("grabar_voz_auto").performClick() // termina y sube
        regla.espera { llamo("subirAdjunto:audio:") }
        regla.esperaEtiqueta("adjunto_auto_0")
        regla.esperaTexto("Nota de voz", substring = true)
        // la burbuja de vista previa la menciona
        nodo("previa_archivo").assertIsDisplayed()
        nodo("guardar_auto").performScrollTo().assertIsEnabled() // un archivo basta aunque no haya texto
        nodo("quitar_adjunto_auto_0").performScrollTo().performClick()
        regla.espera { llamo("borrarAdjunto:") }
        regla.espera { !hay("adjunto_auto_0") }
    }

    @Test fun unaSubidaQueFallaNoAgregaElArchivoYLoDice() {
        shadowOf(Apoyo.ctx() as android.app.Application).grantPermissions(android.Manifest.permission.RECORD_AUDIO)
        Grabadores.fabrica = { GrabadorDeMentira(File(Apoyo.ctx().cacheDir, "voz_auto2.ogg")) }
        motor.subidaFalla = "La nota de voz debe ser mono (un solo canal) y este audio tiene más."
        editor()
        regla.esperaEtiqueta("grabar_voz_auto")
        nodo("grabar_voz_auto").performScrollTo().performClick()
        nodo("grabar_voz_auto").performClick()
        regla.espera { llamo("subirAdjunto") }
        regla.waitForIdle()
        assertFalse(hay("adjunto_auto_0"))
    }

    @Test fun unAtajoTieneNombreConBarraYExplicaComoSeUsa() {
        editor(tipo = "atajo")
        regla.esperaEtiqueta("auto_nombre")
        nodo("auto_nombre").performTextInput("Mi Atajo!")
        regla.esperaTexto("Escribes /miatajo en un chat", substring = true)
        assertTrue(!hay("cuando_siempre") && !hay("quien_todos")) // un atajo no tiene cuándo ni a quién: lo pido yo
        regla.esperaTexto("al escribir “/” en la caja de mensaje", substring = true)
        nodo("auto_texto").performTextInput("Gracias por escribir")
        nodo("guardar_auto").performScrollTo().assertIsEnabled().performClick()
        regla.espera { llamo("guardarAuto") }
        assertEquals("miatajo", guardado().getString("nombre"))
        assertEquals("atajo", guardado().getString("tipo"))
    }

    @Test fun cambiarUnMensajeExistenteCargaSusDatos() {
        existente(7, "recibido", "Ya vi", "Recibí tu mensaje")
        editor(id = 7)
        regla.esperaEtiqueta("auto_texto")
        regla.esperaTexto("Recibí tu mensaje", substring = true)
        regla.esperaTexto("Cambiar: Ya vi")
        nodo("auto_texto").performTextInput(" gracias")
        nodo("guardar_auto").performScrollTo().performClick()
        regla.espera { llamo("guardarAuto") }
        assertEquals(7L, guardado().getLong("id"))
        assertTrue(guardado().getString("texto").endsWith("gracias"))
    }
}
