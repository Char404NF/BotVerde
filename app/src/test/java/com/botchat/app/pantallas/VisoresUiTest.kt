package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import com.botchat.app.medios.CacheImagenes
import com.botchat.app.medios.Temporales
import com.botchat.app.seguridad.FormatoCifrado
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaCompartidos
import com.botchat.app.ui.PantallaDiagnostico
import com.botchat.app.ui.PantallaDocumento
import com.botchat.app.ui.PantallaVisor
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onAllNodesWithTag
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import java.io.File
import java.util.Base64

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h891dp-xxhdpi")
class VisoresUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private val jid = "5215512345678@s.whatsapp.net"

    // PNG de 1 píxel
    private val png = Base64.getDecoder().decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")

    @Before fun antes() { motor = Apoyo.reinicia(); Apoyo.creaBot() }

    private fun guarda(rel: String, bytes: ByteArray): File {
        val f = File(Apoyo.ctx().filesDir, rel).apply { parentFile!!.mkdirs() }
        f.writeBytes(FormatoCifrado.cifrarBytes(Apoyo.clave, bytes))
        return f
    }

    @Test fun cacheDeImagenesDescifraYReutiliza() {
        val f = guarda("media/1/ab/F1.enc", png)
        val b1 = kotlinx.coroutines.runBlocking { CacheImagenes.cargar(Apoyo.ctx(), f, 1080) }
        assertNotNull(b1)
        assertTrue(CacheImagenes.enCache("${f.path}|1080|") === b1)
        CacheImagenes.vaciar()
        assertNull(CacheImagenes.enCache("${f.path}|1080|"))
        // Un archivo dañado o inexistente no rompe nada
        assertNull(kotlinx.coroutines.runBlocking { CacheImagenes.cargar(Apoyo.ctx(), File(Apoyo.ctx().filesDir, "no-existe.enc"), 100) })
        val malo = File(Apoyo.ctx().filesDir, "malo.enc").apply { writeBytes(ByteArray(100)) }
        assertNull(kotlinx.coroutines.runBlocking { CacheImagenes.cargar(Apoyo.ctx(), malo, 100) })
    }

    @Test fun copiasTemporalesSeBorran() {
        val f = guarda("media/1/ab/D1.enc", "contenido del documento".toByteArray())
        val tmp = Temporales.descifrar(Apoyo.ctx(), f, "../../mi factura.pdf")
        assertEquals("contenido del documento", tmp.readText())
        assertTrue(tmp.path.startsWith(Temporales.carpeta(Apoyo.ctx()).path)) // nombre sin rutas peligrosas
        assertEquals("mi factura.pdf", tmp.name)
        Temporales.borrar(tmp)
        assertFalse(tmp.exists())
        val otra = Temporales.descifrar(Apoyo.ctx(), f, "a.txt")
        val otra2 = Temporales.descifrar(Apoyo.ctx(), f, "b.txt")
        Temporales.borrarTodo(Apoyo.ctx())
        assertFalse(otra.exists() || otra2.exists())
        // El barrido al iniciar la app deja la carpeta vacía
        val resto = File(Temporales.carpeta(Apoyo.ctx()), "viejo/x.bin").apply { parentFile!!.mkdirs(); writeText("x") }
        Temporales.barrer(Apoyo.ctx())
        assertFalse(resto.exists())
    }

    @Test fun visorDeFotosMuestraBarraYAcciones() {
        guarda("media/1/ab/F1.enc", png)
        motor.archivos = Apoyo.arreglo(Apoyo.msgJson("F1", 1_700_000_000, tipo = "imagen", estado = "descargado", ruta = "media/1/ab/F1.enc", bytes = png.size.toLong()))
        regla.setContent { BotChatTema { PantallaVisor(1, jid, "F1", "María López", {}) } }
        regla.esperaTexto("María López")
        regla.onNodeWithContentDescription("Guardar en la galería").assertIsDisplayed()
        regla.onNodeWithContentDescription("Compartir").assertIsDisplayed()
        regla.onNodeWithContentDescription("Volver").assertIsDisplayed()
        Thread.sleep(500); regla.waitForIdle()
        assertEquals(0, regla.onAllNodesWithTextSafe("No se pudo abrir"))
    }

    @Test fun documentoGenericoSeAbreConOtraApp() {
        guarda("media/1/ab/D1.enc", "texto".toByteArray())
        motor.archivos = Apoyo.arreglo(Apoyo.msgJson("D1", 1_700_000_000, tipo = "documento", estado = "descargado", ruta = "media/1/ab/D1.enc", nombre = "notas.txt", bytes = 5))
        regla.setContent { BotChatTema { PantallaDocumento(1, jid, "D1", "María López", {}) } }
        regla.esperaTexto("Abrir con…")
        regla.onAllNodesWithText("notas.txt", substring = true).onFirst().assertIsDisplayed()
        regla.onNodeWithText("Se abre una copia temporal", substring = true).assertIsDisplayed()
        regla.onNodeWithContentDescription("Guardar en la galería").assertIsDisplayed()
    }

    @Test fun documentoNoDescargadoExplicaQueHacer() {
        motor.archivos = "[]"
        regla.setContent { BotChatTema { PantallaDocumento(1, jid, "NOEXISTE", "María López", {}) } }
        regla.esperaTexto("No está descargado")
    }

    @Test fun archivosCompartidosPorPestana() {
        motor.archivos = Apoyo.arreglo(
            Apoyo.msgJson("F1", 100, tipo = "imagen", estado = "descargado", ruta = "media/1/ab/F1.enc"),
            Apoyo.msgJson("D1", 90, tipo = "documento", estado = "ninguno", nombre = "informe.pdf", bytes = 4096, puedeBajarse = true),
        )
        regla.setContent { BotChatTema { PantallaCompartidos(1, jid, "María López", {}, {}, {}) } }
        regla.esperaTexto("Archivos compartidos")
        regla.onNodeWithText("Documentos").performClick()
        regla.esperaTexto("informe.pdf")
        regla.onNodeWithText("Videos").performClick()
        regla.esperaTexto("Nada por aquí")
    }

    @Test fun medicionDeRendimientoSePuedeAbrir() {
        regla.setContent { BotChatTema { PantallaDiagnostico(alVolver = {}) } }
        regla.onNodeWithText("Medir rendimiento").assertIsDisplayed()
        regla.onNodeWithText("Iniciar pruebas").assertIsDisplayed()
        regla.onNodeWithText("Copiar resultados").assertIsDisplayed()
    }

    @Test fun medirAbrirChatEstaYLaOpcionDeCargaAnteriorEstaOculta() {
        regla.setContent { BotChatTema { PantallaDiagnostico(alVolver = {}) } }
        regla.onNodeWithTag("medir_abrir_chat").assertIsDisplayed()
        assertEquals(0, regla.onAllNodesWithTag("comparar_anterior").fetchSemanticsNodes().size)
        repeat(5) { regla.onNodeWithText("Medir rendimiento").performClick() }
        regla.espera { regla.onAllNodesWithTag("comparar_anterior").fetchSemanticsNodes().isNotEmpty() }
    }
}
