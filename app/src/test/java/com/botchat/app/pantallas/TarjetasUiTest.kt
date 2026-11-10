package com.botchat.app.pantallas

import android.content.ClipboardManager
import android.content.Intent
import android.graphics.Bitmap
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import com.botchat.app.seguridad.FormatoCifrado
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaChat
import org.json.JSONArray
import org.json.JSONObject
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
import java.io.ByteArrayOutputStream
import java.io.File

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class TarjetasUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private val jid = "5215512345678@s.whatsapp.net"
    private val ahora = System.currentTimeMillis() / 1000

    @Before fun antes() { motor = Apoyo.reinicia(); Apoyo.creaBot(); motor.chat = Apoyo.chatJson(jid, "Ana Pérez").toString() }

    private fun contacto(): String = JSONObject().put("v", 1).put(
        "contactos", JSONArray().put(JSONObject().put("nombre", "Juan Pérez").put("tels", JSONArray().put("+52 1 55 1234 5678"))),
    ).toString()

    private fun ubicacion(vivo: Boolean = false): String =
        JSONObject().put("v", 1).put("lat", 19.4326).put("lon", -99.1332).put("nombre", "Zócalo").put("dir", "Centro, CDMX").put("vivo", vivo).toString()

    private fun pantalla() = regla.setContent { BotChatTema { PantallaChat(jid, {}, {}, {}, {}) } }

    @Test fun contactoUbicacionYMensajeEspecialSeVenComoTarjetas() {
        motor.mensajes = Apoyo.arreglo(
            Apoyo.msgJson("o", ahora, texto = "[Mensaje no compatible con esta versión]", tipo = "otro"),
            Apoyo.msgJson("u", ahora - 1, texto = ubicacion(), tipo = "ubicacion"),
            Apoyo.msgJson("c", ahora - 2, texto = contacto(), tipo = "contacto"),
        )
        pantalla()
        regla.esperaTexto("Juan Pérez")
        regla.onNodeWithText("+52 1 55 1234 5678").assertIsDisplayed()
        regla.onNodeWithText("Zócalo").assertIsDisplayed()
        regla.onNodeWithText("19.43260, -99.13320").assertIsDisplayed()
        regla.onNodeWithText("Mensaje especial").assertIsDisplayed()
        assertEquals("Nunca se muestra como no compatible", 0, regla.onAllNodesWithText("no compatible", substring = true).fetchSemanticsNodes().size)
        assertEquals("No se ve el JSON", 0, regla.onAllNodesWithText("\"contactos\"", substring = true).fetchSemanticsNodes().size)
    }

    @Test fun mensajesGuardadosConElFormatoAnteriorSiguenViendose() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("c", ahora, texto = "Contacto: Pepe", tipo = "contacto"), Apoyo.msgJson("u", ahora - 1, texto = "Ubicación: Casa", tipo = "ubicacion"))
        pantalla()
        regla.esperaTexto("Contacto: Pepe"); regla.onNodeWithText("Ubicación: Casa").assertIsDisplayed()
    }

    @Test fun abrirEnElMapaLanzaUnaIntentGeo() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("u", ahora, texto = ubicacion(), tipo = "ubicacion"))
        pantalla()
        regla.esperaTexto("Abrir en el mapa")
        regla.onNodeWithTag("abrir_mapa").performClick()
        val app = shadowOf(Apoyo.ctx() as android.app.Application)
        regla.espera { app.peekNextStartedActivity() != null }
        val i = app.nextStartedActivity
        assertEquals(Intent.ACTION_VIEW, i.action)
        assertEquals("geo", i.data!!.scheme)
        assertTrue(i.data.toString().contains("19.43"))
    }

    @Test fun ubicacionEnTiempoRealAvisaQueEsElUltimoPunto() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("u", ahora, texto = ubicacion(vivo = true), tipo = "ubicacion"))
        pantalla()
        regla.esperaTexto("Ubicación en tiempo real"); regla.onNodeWithText("Se muestra el último punto recibido").assertIsDisplayed()
    }

    @Test fun contactoSeCopiaYSeAgregaALaAgenda() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("c", ahora, texto = contacto(), tipo = "contacto"))
        pantalla()
        regla.esperaTexto("Copiar número")
        regla.onNodeWithTag("copiar_numero").performClick()
        val cm = Apoyo.ctx().getSystemService(ClipboardManager::class.java)
        regla.espera { cm.primaryClip?.getItemAt(0)?.text?.toString() == "+52 1 55 1234 5678" }
        regla.onNodeWithTag("agregar_contacto").performClick()
        val app = shadowOf(Apoyo.ctx() as android.app.Application)
        regla.espera { app.peekNextStartedActivity() != null }
        val i = app.nextStartedActivity
        assertEquals(android.provider.ContactsContract.Intents.Insert.ACTION, i.action)
        assertEquals("Juan Pérez", i.getStringExtra(android.provider.ContactsContract.Intents.Insert.NAME))
        assertEquals("+52 1 55 1234 5678", i.getStringExtra(android.provider.ContactsContract.Intents.Insert.PHONE))
    }

    @Test fun stickerDescargadoSeDibujaYNoMuestraErrores() {
        val png = ByteArrayOutputStream().also { Bitmap.createBitmap(16, 16, Bitmap.Config.ARGB_8888).compress(Bitmap.CompressFormat.PNG, 100, it) }.toByteArray()
        File(Apoyo.ctx().filesDir, "media/1/ab/s1.enc").apply { parentFile!!.mkdirs(); writeBytes(FormatoCifrado.cifrarBytes(Apoyo.clave, png)) }
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("s1", ahora, tipo = "sticker", estado = "descargado", ruta = "media/1/ab/s1.enc", bytes = png.size.toLong()))
        pantalla()
        regla.esperaEtiqueta("sticker")
        regla.onNodeWithTag("sticker").assertIsDisplayed()
        assertNotNull(regla.onNodeWithTag("sticker").fetchSemanticsNode())
    }
}
