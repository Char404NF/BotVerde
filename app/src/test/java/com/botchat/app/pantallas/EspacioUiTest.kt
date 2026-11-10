package com.botchat.app.pantallas

import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onLast
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTouchInput
import androidx.compose.ui.test.longClick
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaEspacio
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
class EspacioUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso

    @Before fun antes() {
        motor = Apoyo.reinicia(); Apoyo.creaBot()
        motor.espacioTotal = """{"multimedia":2000000,"miniaturas":10000,"fotos":5000,"base":100000,"libre":3000000000,"total":10000000000}"""
        motor.espacioPorTipo = """[{"categoria":"foto","bytes":1500000,"cantidad":5},{"categoria":"video","bytes":200000,"cantidad":1},{"categoria":"audio","bytes":50000,"cantidad":3},{"categoria":"documento","bytes":250000,"cantidad":2}]"""
        motor.espacioPorChat = """[{"jid":"a@s.whatsapp.net","nombre":"Ana","foto":"","fotoTs":0,"esGrupo":false,"pais":"MX","bytes":1700000,"cantidad":7,"porTipo":[1300000,200000,50000,150000]},
            {"jid":"g@g.us","nombre":"Familia","foto":"","fotoTs":0,"esGrupo":true,"pais":"","bytes":300000,"cantidad":4,"porTipo":[200000,0,0,100000]}]"""
        motor.liberar = """{"bytes":204800,"cantidad":1}"""
    }

    private fun pantalla() = regla.setContent { BotChatTema { PantallaEspacio({}, {}) } }

    @Test fun muestraResumenTiposYChats() {
        pantalla()
        regla.esperaTexto("Ana")
        regla.onNodeWithText("ocupa BotChat").assertIsDisplayed()
        regla.onNodeWithText("Por tipo de archivo").assertIsDisplayed()
        regla.onAllNodesWithText("Fotos").assertCountEquals(2) // una vez en la gráfica y otra en el filtro
        regla.onNodeWithText("Chats que más ocupan").assertIsDisplayed()
        regla.onNodeWithText("Familia").assertIsDisplayed()
        regla.onNodeWithText("Libre en el teléfono", substring = true).assertIsDisplayed()
    }

    @Test fun borrarVideosPideConfirmacionDiciendoCuantoLibera() {
        pantalla()
        regla.esperaTexto("Ana")
        regla.onNodeWithText("Borrar videos").performScrollTo().performClick()
        regla.onNodeWithText("¿Borrar todos los videos?").assertIsDisplayed()
        regla.onNodeWithText("Se liberarán 200 KB.", substring = true).assertIsDisplayed()
        regla.onNodeWithText("Cancelar").performClick()
        assertTrue(motor.llamadas.none { it.startsWith("borrarArchivos") })
        regla.onNodeWithText("Borrar videos").performScrollTo().performClick()
        regla.onAllNodesWithText("Borrar videos").onLast().performClick() // el botón del diálogo
        regla.waitUntil(5_000) { motor.llamadas.contains("borrarArchivos:|video|0") }
    }

    @Test fun borrarMasDe30DiasYTodoLoMultimedia() {
        pantalla()
        regla.esperaTexto("Ana")
        regla.onNodeWithText("Borrar archivos de más de 30 días").performScrollTo().performClick()
        regla.onNodeWithText("¿Borrar archivos de más de 30 días?").assertIsDisplayed()
        regla.onNodeWithText("Borrar archivos").performClick() // botón del diálogo
        regla.waitUntil(5_000) { motor.llamadas.contains("borrarArchivos:||30") }
        regla.waitUntil(5_000) { regla.onAllNodesWithTextSafe("¿Borrar archivos de más de 30 días?") == 0 }
        regla.onNodeWithText("Borrar todo lo multimedia").performScrollTo().performClick()
        regla.onNodeWithText("¿Borrar todo lo multimedia?").assertIsDisplayed()
        regla.onNodeWithText("Se borran fotos, videos, audios y documentos.", substring = true).assertIsDisplayed()
    }

    @Test fun filtrosYSeleccionMultiple() {
        pantalla()
        regla.esperaTexto("Ana")
        regla.onAllNodesWithText("Videos").onLast().performScrollTo().performClick() // filtro: solo Ana tiene videos
        regla.waitUntil(5_000) { regla.onAllNodesWithTextSafe("Familia") == 0 }
        regla.onNodeWithText("Ana").assertIsDisplayed()
        regla.onNodeWithText("Todo").performScrollTo().performClick()
        regla.esperaTexto("Familia")
        // Mantener pulsado selecciona; luego se borran los archivos de lo seleccionado
        regla.onNodeWithText("Ana").performScrollTo().performTouchInput { longClick() }
        regla.esperaTexto("1 seleccionado")
        regla.onNodeWithText("Familia").performClick()
        regla.esperaTexto("2 seleccionados")
        regla.onNodeWithContentDescription("Borrar archivos").performClick()
        regla.onNodeWithText("¿Borrar archivos de 2 chats?").assertIsDisplayed()
        regla.onNodeWithText("Se liberarán 1.9 MB.", substring = true).assertIsDisplayed()
        regla.onNodeWithText("Cancelar").performClick()
        regla.onNodeWithContentDescription("Cancelar selección").performClick()
        regla.esperaTexto("Chats que más ocupan")
    }

    @Test fun revisarYRepararDiceQueEstabaEnOrden() {
        pantalla()
        regla.esperaTexto("Ana")
        regla.onNodeWithText("Revisar y reparar").performScrollTo().performClick()
        regla.esperaTexto("Todo está en orden", substring = true)
        assertEquals(1, motor.llamadas.count { it == "reparar" })
    }

    @Test fun elMenuDeCadaChatOfreceBorrarArchivosOTodoElChat() {
        pantalla()
        regla.esperaTexto("Ana")
        regla.onNodeWithContentDescription("Más opciones de Ana").performClick()
        regla.onNodeWithText("Borrar todo el chat").performClick()
        regla.onNodeWithText("¿Borrar todo el chat con Ana?").assertIsDisplayed()
        regla.onNodeWithText("No se borra nada en WhatsApp.", substring = true).assertIsDisplayed()
    }
}
