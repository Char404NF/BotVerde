package com.botchat.app.pantallas

import androidx.activity.ComponentActivity
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import com.botchat.app.datos.Ajustes
import com.botchat.app.datos.JsonVideo
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaDescargasVideo
import com.botchat.app.ui.duracionVideoTxt
import com.botchat.app.ui.pesoVideoTxt
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** Pantalla de descargas de video: historial con quién, cuándo, peso y formato; volver a descargar; y la versión de yt-dlp. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class DescargasVideoUiTest {
    @get:Rule val regla = createAndroidComposeRule<ComponentActivity>()
    private lateinit var motor: MotorFalso
    private var bot = 0L

    private val filas = """[
      {"id":2,"ts":1,"persona":"p1","nombre":"Ana","chat":"c","esGrupo":true,"url":"https://youtu.be/abc","titulo":"Gatito feliz","subidor":"Canal","sitio":"Youtube","duracion":213,"peso":31457280,"formato":"mp4","calidad":"720p","partes":3,"entrega":"partes","estado":"enviado","error":""},
      {"id":1,"ts":1,"persona":"p2","nombre":"Luis","chat":"d","esGrupo":false,"url":"https://soundcloud.com/x","titulo":"Canción","subidor":"","sitio":"Soundcloud","duracion":120,"peso":4194304,"formato":"mp3","calidad":"160 kbps","partes":1,"entrega":"directo","estado":"fallido","error":"ese video no está disponible"}
    ]"""

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot(); Ajustes.get(Apoyo.ctx()).aceptarAviso(); motor.videoFilas = filas }

    private fun hay(tag: String) = regla.onAllNodes(hasTestTag(tag), useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun toca(tag: String) { regla.espera { hay(tag) }; val n = regla.onNodeWithTag(tag, useUnmergedTree = true); runCatching { n.performScrollTo() }; n.performClick() }

    @Test fun elHistorialMuestraQuienLoPidioElPesoYElFormato() {
        regla.setContent { BotChatTema { PantallaDescargasVideo(bot, alVolver = {}) } }
        regla.espera { hay("descarga_2") && hay("descarga_1") }
        regla.onNodeWithText("Gatito feliz").assertIsDisplayed()
        regla.onNodeWithText("Ana (en un grupo)", substring = true).assertIsDisplayed()
        regla.onNodeWithText("MP4 · 720p").assertIsDisplayed()
        regla.onNodeWithText("30 MB").assertIsDisplayed()
        regla.onNodeWithText("3 partes").assertIsDisplayed()
        regla.onNodeWithText("Falló").assertIsDisplayed()
        regla.onNodeWithText("ese video no está disponible").assertIsDisplayed()
    }

    @Test fun volverADescargarYQuitarDelHistorial() {
        regla.setContent { BotChatTema { PantallaDescargasVideo(bot, alVolver = {}) } }
        toca("repetir_2")
        regla.espera { motor.llamadas.contains("video_repetir:2") }
        toca("borrar_1"); toca("confirmar_borrar_descarga")
        regla.espera { motor.llamadas.contains("video_borrar:1") }
    }

    @Test fun vaciarElHistorialPideConfirmar() {
        regla.setContent { BotChatTema { PantallaDescargasVideo(bot, alVolver = {}) } }
        toca("vaciar_historial_video")
        regla.espera { hay("dialogo_vaciar_descargas") }
        assertTrue(motor.llamadas.none { it.startsWith("video_borrar") })
        toca("confirmar_vaciar_descargas")
        regla.espera { motor.llamadas.contains("video_borrar:0") }
    }

    @Test fun laTarjetaDelMotorAvisaDeUnaVersionNuevaYActualiza() {
        regla.setContent { BotChatTema { PantallaDescargasVideo(bot, alVolver = {}) } }
        regla.espera { hay("version_ytdlp") }
        regla.onNodeWithTag("version_ytdlp", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithText("yt-dlp 2026.10.26 · hay una versión nueva (2026.11.01)").assertIsDisplayed()
        toca("actualizar_ytdlp")
        regla.espera { motor.llamadas.contains("video_actualizar") }
        regla.espera { hay("mensaje_motor_video") }
    }

    @Test fun sinDescargasMuestraUnaPantallaVaciaClara() {
        motor.videoFilas = "[]"
        regla.setContent { BotChatTema { PantallaDescargasVideo(bot, alVolver = {}) } }
        regla.onNodeWithText("Aún no hay descargas").assertIsDisplayed()
    }

    @Test fun lasUnidadesSeEscribenEnPalabrasSimples() {
        assertEquals("", pesoVideoTxt(0))
        assertEquals("1 MB", pesoVideoTxt(500))
        assertEquals("45 MB", pesoVideoTxt(45L * 1024 * 1024))
        assertEquals("1.5 GB", pesoVideoTxt(1536L * 1024 * 1024))
        assertEquals("3:20", duracionVideoTxt(200))
        assertEquals("1 h 05 min", duracionVideoTxt(3900))
        val l = JsonVideo.lista(filas)
        assertEquals(2, l.size); assertEquals("fallido", l[1].estado); assertEquals(3, l[0].partes)
        assertEquals("[]", "[]"); assertTrue(JsonVideo.lista("null").isEmpty())
    }
}
