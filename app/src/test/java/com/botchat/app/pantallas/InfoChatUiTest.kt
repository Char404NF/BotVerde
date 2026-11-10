package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaChat
import com.botchat.app.ui.PantallaInfoChat
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
import java.time.YearMonth
import java.time.ZoneId

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class InfoChatUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private val jid = "5215512345678@s.whatsapp.net"
    private var bot = 0L

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot(); motor.chat = Apoyo.chatJson(jid, "Ana Pérez").toString() }

    private fun tipo(t: String, env: Int, rec: Int, bytes: Long = 0) = JSONObject().put("tipo", t).put("enviados", env).put("recibidos", rec).put("bytes", bytes)
    private fun info() = regla.setContent { BotChatTema { PantallaInfoChat(bot, jid, {}, {}, {}) } }

    @Test fun tocarElNombreAbreInfoYTocarLaFotoAbreLaFoto() {
        var info = 0
        regla.setContent { BotChatTema { PantallaChat(jid, {}, {}, {}, {}, alAbrirInfo = { info++ }) } }
        regla.esperaTexto("Ana Pérez")
        regla.onNodeWithTag("avatar_chat").performClick()
        regla.esperaTexto("Sin foto") // el visor de la foto respondió; esta prueba no trae foto de perfil
        assertEquals("La foto no abre la Info", 0, info)
        regla.onNodeWithTag("encabezado_info").performClick()
        regla.espera { info == 1 }
    }

    @Test fun resumenMuestraCifrasTiposGraficosYLaFechaDelPrimerMensaje() {
        motor.resumenChat = JSONObject().put("desde", 1789992000).put("listo", true).put("enviados", 3).put("recibidos", 7)
            .put("porTipo", JSONArray().put(tipo("texto", 2, 5)).put(tipo("imagen", 1, 2, 4096)))
            .put("porHora", JSONArray(List(24) { if (it == 9) 5 else 0 })).put("porDiaSemana", JSONArray(listOf(1, 2, 3, 0, 0, 4, 0)))
            .put("quien", JSONArray()).toString()
        info()
        regla.esperaTexto("Estadísticas desde el 21 de septiembre de 2026")
        regla.onNodeWithTag("cifra_enviados").assertIsDisplayed()
        regla.onNodeWithText("3").assertIsDisplayed(); regla.onNodeWithText("7").assertIsDisplayed()
        regla.onNodeWithText("Mensajes de texto").assertIsDisplayed()
        regla.onNodeWithText("Fotos").assertIsDisplayed()
        regla.onNodeWithText("1 enviados · 2 recibidos · 4 KB").assertIsDisplayed()
        regla.onNodeWithTag("grafico_hora").assertIsDisplayed()
        regla.onNodeWithTag("grafico_semana").assertIsDisplayed()
        assertEquals("Una persona no tiene ranking", 0, regla.onAllNodesWithTag("quien").fetchSemanticsNodes().size)
    }

    @Test fun enGruposMuestraQuienEscribeMas() {
        motor.resumenChat = JSONObject().put("desde", 1789992000).put("listo", true).put("enviados", 0).put("recibidos", 3)
            .put("porTipo", JSONArray().put(tipo("texto", 0, 3))).put("porHora", JSONArray(List(24) { 0 })).put("porDiaSemana", JSONArray(List(7) { 0 }))
            .put("quien", JSONArray().put(JSONObject().put("nombre", "Ana").put("cantidad", 2)).put(JSONObject().put("nombre", "Beto").put("cantidad", 1))).toString()
        info()
        regla.esperaTexto("Quién escribe más")
        assertEquals(2, regla.onAllNodesWithTag("quien").fetchSemanticsNodes().size)
        regla.onNodeWithText("Beto").assertIsDisplayed()
    }

    @Test fun sinMensajesMuestraUnEstadoVacioClaro() {
        info()
        regla.esperaTexto("Sin números todavía")
        regla.onNodeWithText("Todavía no hay mensajes guardados").assertIsDisplayed()
    }

    @Test fun avisaQueSeEstanContandoLosMensajesAnteriores() {
        motor.resumenChat = JSONObject().put("desde", 1789992000).put("listo", false).put("enviados", 0).put("recibidos", 1)
            .put("porTipo", JSONArray()).put("porHora", JSONArray(List(24) { 0 })).put("porDiaSemana", JSONArray(List(7) { 0 })).put("quien", JSONArray()).toString()
        info()
        regla.esperaTexto("contando los mensajes anteriores", substring = true)
    }

    @Test fun calendarioPintaLosDiasConMensajesYAlTocarUnoListaSusMensajes() {
        val mes = YearMonth.now()
        val primero = mes.atDay(1)
        val ts = primero.atTime(10, 30).atZone(ZoneId.systemDefault()).toEpochSecond()
        motor.calendario = JSONObject().put("desde", ts).put("listo", true)
            .put("dias", JSONArray().put(JSONObject().put("dia", primero.toString()).put("cantidad", 2))).toString()
        motor.porDia[primero.toString()] = Apoyo.arreglo(
            Apoyo.msgJson("a", ts, texto = "Buenos días", remitente = "Ana"),
            Apoyo.msgJson("b", ts + 60, texto = "Respuesta del bot", mio = true),
        )
        info()
        regla.esperaTexto("Calendario"); regla.onNodeWithText("Calendario").performClick()
        regla.esperaEtiqueta("dia_$primero")
        regla.onNodeWithTag("mes_anterior").assertIsNotEnabled()
        regla.onNodeWithTag("mes_siguiente").assertIsNotEnabled()
        assertEquals("Sin día elegido no hay lista", 0, regla.onAllNodesWithTag("lista_dia").fetchSemanticsNodes().size)
        regla.onNodeWithTag("dia_$primero").performClick()
        regla.esperaTexto("Buenos días")
        regla.onNodeWithText("Respuesta del bot").assertIsDisplayed()
        regla.onNodeWithText("Bot").assertIsDisplayed()
        assertTrue(motor.llamadas.contains("dia:$primero"))
    }

    @Test fun calendarioPermiteIrAMesesAnterioresSoloDesdeElPrimerMensaje() {
        val hace2 = YearMonth.now().minusMonths(2)
        val ts = hace2.atDay(5).atTime(9, 0).atZone(ZoneId.systemDefault()).toEpochSecond()
        motor.calendario = JSONObject().put("desde", ts).put("listo", true).put("dias", JSONArray()).toString()
        info()
        regla.onNodeWithText("Calendario").performClick()
        regla.esperaEtiqueta("mes_anterior")
        fun retrocede(esperado: YearMonth) {
            regla.espera { runCatching { regla.onNodeWithTag("mes_anterior").assertIsEnabled() }.isSuccess }
            regla.onNodeWithTag("mes_anterior").performClick()
            try {
                regla.espera { motor.llamadas.toList().contains("calendario:$esperado") }
            } catch (e: Throwable) {
                val titulo = regla.onNodeWithTag("titulo_mes").fetchSemanticsNode().config.toString().takeLast(80)
                throw AssertionError("No se pidió $esperado. Llamadas=${motor.llamadas.toList()} titulo=$titulo", e)
            }
        }
        retrocede(YearMonth.now().minusMonths(1))
        retrocede(hace2)
        regla.espera { runCatching { regla.onNodeWithTag("mes_anterior").assertIsNotEnabled() }.isSuccess }
        regla.onNodeWithTag("mes_siguiente").assertIsEnabled()
        regla.onNodeWithTag("mes_vacio").assertIsDisplayed()
    }

    @Test fun multimediaFiltraPorTipoYAgrupaPorFecha() {
        val ts = 1789992000L
        motor.archivos = Apoyo.arreglo(
            Apoyo.msgJson("d1", ts, tipo = "documento", nombre = "Informe.docx", bytes = 10, estado = "descargado", ruta = "media/x.enc"),
            Apoyo.msgJson("f1", ts, tipo = "imagen", bytes = 10, estado = "ninguno", puedeBajarse = true),
        )
        info()
        regla.onNodeWithText("Multimedia").performClick()
        regla.esperaEtiqueta("filtro_documento")
        listOf("Imágenes", "Videos", "Audio", "Documentos").forEach { regla.onNodeWithText(it).assertIsDisplayed() }
        regla.onNodeWithText("Documentos").performClick()
        regla.esperaTexto("Informe.docx")
        regla.onNodeWithText("Videos").performClick()
        regla.esperaTexto("Nada por aquí")
        assertFalse(regla.onAllNodesWithTag("filtro_foto").fetchSemanticsNodes().isEmpty())
    }
}
