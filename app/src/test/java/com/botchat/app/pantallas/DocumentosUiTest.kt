package com.botchat.app.pantallas

import android.content.Intent
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import com.botchat.app.seguridad.FormatoCifrado
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaChat
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import java.io.File

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class DocumentosUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private val jid = "5215512345678@s.whatsapp.net"

    @Before fun antes() { motor = Apoyo.reinicia(); Apoyo.creaBot(); motor.chat = Apoyo.chatJson(jid, "Ana Pérez").toString() }

    private fun guarda(rel: String, texto: String) {
        File(Apoyo.ctx().filesDir, rel).apply { parentFile!!.mkdirs(); writeBytes(FormatoCifrado.cifrarBytes(Apoyo.clave, texto.toByteArray())) }
    }

    private fun doc(id: String, ts: Long, nombre: String, mime: String, descargado: Boolean = true) =
        Apoyo.msgJson(id, ts, tipo = "documento", nombre = nombre, bytes = 5, estado = if (descargado) "descargado" else "ninguno", ruta = if (descargado) "media/1/ab/$id.enc" else "", puedeBajarse = !descargado)
            .put("mime", mime)

    @Test fun cualquierTipoDeArchivoSeMuestraConTarjetaNombreYAbrirCon() {
        val ahora = System.currentTimeMillis() / 1000
        listOf("d1", "d2", "d3", "d4").forEach { guarda("media/1/ab/$it.enc", "contenido $it") }
        motor.mensajes = Apoyo.arreglo(
            doc("d4", ahora, "rarisimo.xyz", "application/octet-stream"),
            doc("d3", ahora - 1, "", "application/zip"),                         // sin nombre: se deduce del tipo
            doc("d2", ahora - 2, "Cuentas 2026.xlsx", ""),                        // sin tipo: se deduce del nombre
            doc("d1", ahora - 3, "Informe.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"),
            doc("d0", ahora - 4, "Pendiente.pptx", "", descargado = false),
        )
        regla.setContent { BotChatTema { PantallaChat(jid, {}, {}, {}, {}) } }
        regla.esperaTexto("Informe.docx")
        for (n in listOf("rarisimo.xyz", "Documento.zip", "Cuentas 2026.xlsx", "Informe.docx", "Pendiente.pptx")) regla.onNodeWithText(n).assertIsDisplayed()
        regla.onNodeWithText("XLSX").assertIsDisplayed(); regla.onNodeWithText("DOCX").assertIsDisplayed(); regla.onNodeWithText("XYZ").assertIsDisplayed()
        assertEquals(5, regla.onAllNodesWithTag("tarjeta_documento").fetchSemanticsNodes().size)
        assertEquals("Solo los descargados ofrecen Abrir con…", 4, regla.onAllNodesWithText("Abrir con…").fetchSemanticsNodes().size)
        assertEquals("El pendiente ofrece descargar", 0, regla.onAllNodesWithText("Pendiente.pptx").fetchSemanticsNodes().size - 1)
        assertEquals(0, regla.onAllNodesWithText("no compatible", substring = true).fetchSemanticsNodes().size)
    }

    @Test fun abrirConLanzaElSelectorConElTipoCorrecto() {
        val ahora = System.currentTimeMillis() / 1000
        guarda("media/1/ab/d1.enc", "zip de mentira")
        motor.mensajes = Apoyo.arreglo(doc("d1", ahora, "cosas.zip", ""))
        regla.setContent { BotChatTema { PantallaChat(jid, {}, {}, {}, {}) } }
        regla.esperaTexto("cosas.zip")
        regla.onNodeWithText("Abrir con…").performClick()
        val app = shadowOf(Apoyo.ctx() as android.app.Application)
        regla.waitUntil(20_000) { app.peekNextStartedActivity() != null }
        val selector = app.nextStartedActivity
        assertEquals(Intent.ACTION_CHOOSER, selector.action)
        @Suppress("DEPRECATION") val ver = selector.getParcelableExtra<Intent>(Intent.EXTRA_INTENT)!!
        assertEquals(Intent.ACTION_VIEW, ver.action)
        assertEquals("application/zip", ver.type)
        assertNotNull(ver.data)
        assertTrue(ver.flags and Intent.FLAG_GRANT_READ_URI_PERMISSION != 0)
    }
}
