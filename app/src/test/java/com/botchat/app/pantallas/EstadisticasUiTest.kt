package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaEstadisticas
import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import java.time.LocalDate

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class EstadisticasUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }

    @Test fun muestraTotalesChatsMasActivosYActividadDiaria() {
        val hoy = LocalDate.now()
        motor.globales = JSONObject().put("desde", 1789992000).put("listo", true).put("enviados", 12).put("recibidos", 30).put("chats", 4)
            .put("porTipo", JSONArray().put(JSONObject().put("tipo", "texto").put("enviados", 12).put("recibidos", 25).put("bytes", 0)))
            .put("activos", JSONArray().put(JSONObject().put("jid", "a@s.whatsapp.net").put("nombre", "Ana Pérez").put("foto", "").put("fotoTs", 0).put("esGrupo", false).put("pais", "MX").put("cantidad", 20)))
            .put("porDia", JSONArray(List(30) { JSONObject().put("dia", hoy.minusDays((29 - it).toLong()).toString()).put("cantidad", it) })).toString()
        var abierto = ""
        regla.setContent { BotChatTema { PantallaEstadisticas(bot, {}, { abierto = it }) } }
        regla.esperaTexto("Estadísticas desde el 21 de septiembre de 2026")
        regla.onNodeWithTag("cifra_enviados").assertIsDisplayed()
        regla.onNodeWithText("30").assertIsDisplayed()
        regla.onNodeWithText("Chats más activos").assertIsDisplayed()
        regla.onNodeWithTag("grafico_dias").assertIsDisplayed()
        regla.onNodeWithText("zona horaria actual", substring = true).assertIsDisplayed()
        regla.onNodeWithTag("activo_a@s.whatsapp.net").performClick()
        assertEquals("a@s.whatsapp.net", abierto)
    }

    @Test fun sinMensajesExplicaQueEsperar() {
        regla.setContent { BotChatTema { PantallaEstadisticas(bot, {}, {}) } }
        regla.esperaTexto("Sin números todavía")
        regla.onNodeWithText("Todavía no hay mensajes guardados").assertIsDisplayed()
    }
}
