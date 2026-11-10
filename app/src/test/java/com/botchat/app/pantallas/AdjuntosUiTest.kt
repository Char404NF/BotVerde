package com.botchat.app.pantallas

import android.graphics.Bitmap
import android.net.Uri
import androidx.activity.result.ActivityResultRegistry
import androidx.activity.result.ActivityResultRegistryOwner
import androidx.activity.result.contract.ActivityResultContract
import androidx.activity.compose.LocalActivityResultRegistryOwner
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.performTouchInput
import androidx.compose.ui.test.down
import androidx.compose.ui.test.moveBy
import androidx.compose.ui.test.up
import androidx.compose.ui.test.center
import androidx.core.app.ActivityOptionsCompat
import com.botchat.app.medios.GrabacionVoz
import com.botchat.app.medios.GrabadorVoz
import com.botchat.app.medios.Grabadores
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaChat
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
import java.io.ByteArrayInputStream
import java.io.ByteArrayOutputStream
import java.io.File

/** Un registro de resultados que contesta de inmediato con los archivos elegidos, como si el usuario los hubiera escogido. */
private class RegistroFalso(var uris: List<Uri>) : ActivityResultRegistry() {
    var lanzados = mutableListOf<String>()
    @Suppress("UNCHECKED_CAST")
    override fun <I, O> onLaunch(requestCode: Int, contract: ActivityResultContract<I, O>, input: I, options: ActivityOptionsCompat?) {
        lanzados += contract.javaClass.simpleName
        dispatchResult(requestCode, (if (contract is androidx.activity.result.contract.ActivityResultContracts.RequestPermission) false else uris) as O)
    }
}

private class GrabadorFalso(val archivo: File) : GrabadorVoz {
    var cancelado = false
    var terminado = false
    override fun iniciar() = true
    override fun terminar(): GrabacionVoz? { terminado = true; archivo.writeBytes(byteArrayOf(1, 2, 3)); return GrabacionVoz(archivo, 5, ByteArray(64) { 30 }, true, "audio/ogg; codecs=opus") }
    override fun cancelar() { cancelado = true }
}

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class AdjuntosUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private val jid = "5215512345678@s.whatsapp.net"
    private val ahora = System.currentTimeMillis() / 1000
    private val ctx get() = Apoyo.ctx()

    @Before fun antes() {
        motor = Apoyo.reinicia(); Apoyo.creaBot(); motor.chat = Apoyo.chatJson(jid, "Ana Pérez").toString()
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola", remitente = "Ana"))
    }
    @After fun despues() { Grabadores.fabrica = { com.botchat.app.medios.GrabadorMediaRecorder(it) } }

    private fun pantalla(registro: ActivityResultRegistry) = regla.setContent {
        CompositionLocalProvider(LocalActivityResultRegistryOwner provides object : ActivityResultRegistryOwner { override val activityResultRegistry = registro }) {
            BotChatTema { PantallaChat(jid, {}, {}, {}, {}) }
        }
    }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun llamo(prefijo: String) = motor.llamadas.toList().any { it.startsWith(prefijo) }

    private fun registraUri(nombre: String, bytes: ByteArray): Uri {
        val u = Uri.parse("content://prueba.archivos/$nombre")
        shadowOf(ctx.contentResolver).registerInputStream(u, ByteArrayInputStream(bytes))
        return u
    }
    private fun png(ancho: Int = 400, alto: Int = 300): ByteArray = ByteArrayOutputStream().also { Bitmap.createBitmap(ancho, alto, Bitmap.Config.ARGB_8888).compress(Bitmap.CompressFormat.PNG, 100, it) }.toByteArray()

    @Test fun elegirUnaFotoMuestraLaVistaPreviaYEnviaConPie() {
        val reg = RegistroFalso(listOf(registraUri("foto.png", png())))
        pantalla(reg)
        regla.esperaEtiqueta("boton_adjuntar")
        regla.onNodeWithTag("boton_adjuntar").performClick()
        regla.esperaEtiqueta("adjuntar_GALERIA")
        regla.onNodeWithTag("adjuntar_GALERIA").performClick()
        regla.esperaEtiqueta("panel_adjuntos")
        regla.esperaEtiqueta("adjunto_foto.jpg")
        regla.onNodeWithTag("campo_mensaje").performTextInput("mi foto")
        regla.onNodeWithTag("enviar").performClick()
        regla.espera { llamo("medio:imagen:image/jpeg:foto.jpg:mi foto::0:false:") }
        assertTrue("lleva miniatura", motor.llamadas.first { it.startsWith("medio:imagen") }.split(":")[8].toInt() > 0)
        regla.espera { !hay("panel_adjuntos") }
    }

    @Test fun losDocumentosSeEnvianComoDocumentoConSuNombre() {
        val reg = RegistroFalso(listOf(registraUri("Cuentas 2026.xlsx", byteArrayOf(1, 2, 3, 4))))
        pantalla(reg)
        regla.esperaEtiqueta("boton_adjuntar")
        regla.onNodeWithTag("boton_adjuntar").performClick()
        regla.esperaEtiqueta("adjuntar_DOCUMENTO")
        regla.onNodeWithTag("adjuntar_DOCUMENTO").performClick()
        regla.esperaEtiqueta("panel_adjuntos")
        regla.onNodeWithTag("enviar").performClick()
        regla.espera { llamo("medio:documento:") && motor.llamadas.any { it.contains("Cuentas 2026.xlsx") } }
    }

    @Test fun cancelarLosAdjuntosNoEnviaNada() {
        val reg = RegistroFalso(listOf(registraUri("foto.png", png())))
        pantalla(reg)
        regla.esperaEtiqueta("boton_adjuntar")
        regla.onNodeWithTag("boton_adjuntar").performClick()
        regla.esperaEtiqueta("adjuntar_GALERIA")
        regla.onNodeWithTag("adjuntar_GALERIA").performClick()
        regla.esperaEtiqueta("cancelar_adjuntos")
        regla.onNodeWithTag("cancelar_adjuntos").performClick()
        regla.espera { !hay("panel_adjuntos") }
        assertFalse(llamo("medio:"))
        regla.espera { (com.botchat.app.medios.PrepararAdjunto.carpeta(ctx).listFiles() ?: emptyArray()).isEmpty() } // las copias temporales se borran
    }

    @Test fun elMenuOfreceGaleriaCamaraDocumentoYAudio() {
        pantalla(RegistroFalso(emptyList()))
        regla.esperaEtiqueta("boton_adjuntar")
        regla.onNodeWithTag("boton_adjuntar").performClick()
        for (t in listOf("GALERIA", "CAMARA_FOTO", "CAMARA_VIDEO", "DOCUMENTO", "AUDIO")) regla.esperaEtiqueta("adjuntar_$t")
    }

    @Test fun mantenerPulsadoElMicrofonoGrabaYAlSoltarEnvia() {
        shadowOf(ctx as android.app.Application).grantPermissions(android.Manifest.permission.RECORD_AUDIO)
        val g = GrabadorFalso(File(ctx.cacheDir, "voz.ogg")); Grabadores.fabrica = { g }
        pantalla(RegistroFalso(emptyList()))
        regla.esperaEtiqueta("microfono")
        regla.onNodeWithTag("microfono").performTouchInput { down(center) }
        regla.esperaEtiqueta("grabando")
        regla.onNodeWithTag("microfono").performTouchInput { up() }
        regla.espera { llamo("medio:audio:audio/ogg; codecs=opus:::") }
        assertTrue(motor.llamadas.any { it.startsWith("medio:audio") && it.endsWith(":5:true:0:64") })
        assertTrue(g.terminado)
    }

    @Test fun deslizarALaIzquierdaCancelaLaNotaDeVoz() {
        shadowOf(ctx as android.app.Application).grantPermissions(android.Manifest.permission.RECORD_AUDIO)
        val g = GrabadorFalso(File(ctx.cacheDir, "voz2.ogg")); Grabadores.fabrica = { g }
        pantalla(RegistroFalso(emptyList()))
        regla.esperaEtiqueta("microfono")
        regla.onNodeWithTag("microfono").performTouchInput { down(center); moveBy(Offset(-600f, 0f)); up() }
        regla.espera { g.cancelado }
        assertFalse(llamo("medio:"))
    }

    @Test fun conTextoEscritoElMicrofonoDejaElLugarAlBotonEnviar() {
        pantalla(RegistroFalso(emptyList()))
        regla.esperaEtiqueta("microfono")
        regla.onNodeWithTag("campo_mensaje").performTextInput("hola")
        regla.espera { !hay("microfono") && hay("enviar") }
    }

    @Test fun sinPermisoNoSeGrabaNiSePierdeElToque() {
        val g = GrabadorFalso(File(ctx.cacheDir, "voz3.ogg")); Grabadores.fabrica = { g }
        pantalla(RegistroFalso(emptyList()))
        regla.esperaEtiqueta("microfono")
        regla.onNodeWithTag("microfono").performTouchInput { down(center); up() }
        regla.waitForIdle()
        assertFalse("sin permiso no se graba", hay("grabando"))
        assertFalse(g.terminado)
    }
}
