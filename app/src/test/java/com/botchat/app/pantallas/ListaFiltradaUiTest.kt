package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import com.botchat.app.datos.FiltroLista
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaListaFiltrada
import com.botchat.app.ui.tituloDeFiltro
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class ListaFiltradaUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L
    private val jid = "120@g.us"
    private val ts = 1789992000L

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }

    private fun pagina(total: Int, vararg m: JSONObject) = JSONObject().put("total", total).put("items", org.json.JSONArray().also { a -> m.forEach { a.put(it) } }).toString()

    @Test fun lasLongitudesYTitulosSonClaros() {
        assertEquals("Fotos · 4", tituloDeFiltro(FiltroLista(tipos = listOf("imagen")), 4))
        assertEquals("A las 9:00 · 12", tituloDeFiltro(FiltroLista(hora = 9), 12))
        assertEquals("Martes · 7", tituloDeFiltro(FiltroLista(semana = 1), 7))
        assertEquals("Eliminados", tituloDeFiltro(FiltroLista(eliminados = true), null))
        assertEquals("Lunes 2 de noviembre · 5", tituloDeFiltro(FiltroLista(dia = "2026-11-02"), 5))
    }

    @Test fun listaConTituloFechaRemitenteVistaPreviaYIrAlMensaje() {
        motor.filtrados = pagina(
            2, Apoyo.msgJson("a", ts, texto = "Hola a todos", remitente = "Ana"),
            Apoyo.msgJson("b", ts - 60, texto = "Ya no está", remitente = "Beto", borrado = true).put("eliminadoEn", ts),
        )
        var ir = ""
        regla.setContent { BotChatTema { PantallaListaFiltrada(bot, jid, FiltroLista(tipos = listOf("texto")), false, {}, {}, {}, { ir = it }) } }
        regla.esperaTexto("Mensajes de texto · 2")
        regla.onNodeWithText("Hola a todos").assertIsDisplayed()
        regla.onNodeWithText("Ya no está").assertIsDisplayed()
        regla.onNodeWithTag("ir_a").performClick()
        regla.espera { ir == "a" }
        assertTrue(motor.llamadas.contains("filtro:" + FiltroLista(tipos = listOf("texto")).json()))
    }

    @Test fun losChipsCambianElFiltroYRecargan() {
        motor.filtrados = pagina(1, Apoyo.msgJson("a", ts, texto = "uno", remitente = "Ana"))
        regla.setContent { BotChatTema { PantallaListaFiltrada(bot, jid, FiltroLista(), false, {}, {}, {}, {}) } }
        regla.esperaTexto("uno")
        regla.onNodeWithTag("chip_dir_enviados").performClick()
        regla.espera { motor.llamadas.contains("filtro:" + FiltroLista(direccion = "enviados").json()) }
        regla.onNodeWithTag("chip_eliminados").performClick()
        regla.espera { motor.llamadas.contains("filtro:" + FiltroLista(direccion = "enviados", eliminados = true).json()) }
        regla.onNodeWithTag("chip_dir_todos").performClick()
        regla.espera { motor.llamadas.contains("filtro:" + FiltroLista(eliminados = true).json()) }
        assertEquals("Un chat de una persona no ofrece selector de persona", 0, regla.onAllNodesWithTag("chip_persona").fetchSemanticsNodes().size)
    }

    @Test fun enGruposHaySelectorDePersona() {
        motor.personas = """[{"jid":"a@lid","nombre":"Ana","cantidad":3},{"jid":"b@lid","nombre":"Beto","cantidad":1}]"""
        motor.filtrados = pagina(1, Apoyo.msgJson("a", ts, texto = "uno", remitente = "Ana"))
        regla.setContent { BotChatTema { PantallaListaFiltrada(bot, jid, FiltroLista(), true, {}, {}, {}, {}) } }
        regla.esperaEtiqueta("chip_persona")
        regla.onNodeWithTag("chip_persona").performClick()
        regla.esperaTexto("Beto · 1")
        regla.onNodeWithText("Beto · 1").performClick()
        regla.espera { motor.llamadas.contains("filtro:" + FiltroLista(persona = "b@lid").json()) }
        regla.esperaTexto("Beto")
    }

    @Test fun fotosVideosYStickersVanEnCuadriculaPorFecha() {
        motor.filtrados = pagina(
            2, Apoyo.msgJson("f1", ts, tipo = "imagen", estado = "ninguno", puedeBajarse = true), Apoyo.msgJson("f2", ts - 86_400, tipo = "imagen", estado = "ninguno", puedeBajarse = true),
        )
        regla.setContent { BotChatTema { PantallaListaFiltrada(bot, "1@s.whatsapp.net", FiltroLista(tipos = listOf("imagen")), false, {}, {}, {}, {}) } }
        regla.esperaTexto("Fotos · 2")
        assertEquals("Con cuadrícula no hay filas de texto", 0, regla.onAllNodesWithTag("lista_filtrada").fetchSemanticsNodes().size)
    }

    @Test fun sinResultadosExplicaQueNoHay() {
        regla.setContent { BotChatTema { PantallaListaFiltrada(bot, jid, FiltroLista(eliminados = true), false, {}, {}, {}, {}) } }
        regla.esperaTexto("No hay mensajes que cumplan este filtro.")
    }
}
