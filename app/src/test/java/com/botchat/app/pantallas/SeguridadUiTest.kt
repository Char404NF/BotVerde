package com.botchat.app.pantallas

import android.os.SystemClock
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import com.botchat.app.datos.Ajustes
import com.botchat.app.datos.Ajustes.Bloqueo
import com.botchat.app.seguridad.ClaveDatos
import com.botchat.app.ui.AVISO_PIN_LIGERO
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.EstadoBloqueo
import com.botchat.app.ui.PantallaBloqueo
import com.botchat.app.ui.PantallaSeguridad
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import org.robolectric.shadows.ShadowSystemClock
import java.io.File
import java.security.MessageDigest
import java.time.Duration

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h891dp-xxhdpi")
class SeguridadUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var a: Ajustes

    @Before fun antes() { Apoyo.reinicia(); a = Ajustes.get(Apoyo.ctx()) }

    private fun pantalla() = regla.setContent { BotChatTema { PantallaSeguridad(a, {}) } }
    private fun teclea(pin: String) { pin.forEach { regla.onNodeWithText(it.toString()).performClick() } }
    private fun ok() = regla.onNodeWithText("OK").performClick()

    @Test fun pordefectoNoHayBloqueoNiCapturasNiPin() {
        assertEquals(Bloqueo.NINGUNO, a.bloqueo.value)
        assertFalse(a.tienePin.value); assertFalse(a.secure.value); assertFalse(a.huella.value)
        assertEquals(60, a.bloqueoSeg.value)
        EstadoBloqueo.iniciar(a)
        assertFalse("sin bloqueo la app nunca queda bloqueada", EstadoBloqueo.bloqueado.value)
        pantalla()
        regla.onNodeWithText("Proteger la app").assertIsDisplayed()
        assertEquals(0, regla.onAllNodesWithTextSafe("Pedir el desbloqueo")) // el tiempo solo aparece con bloqueo
    }

    @Test fun activarPinPideConfirmacionYMuestraElTiempo() {
        pantalla()
        regla.onNodeWithTag("op_pin").performClick()
        regla.onNodeWithText("Elige tu PIN").assertIsDisplayed()
        teclea("1357"); ok(); teclea("1357"); ok()
        regla.waitUntil(5_000) { a.bloqueo.value == Bloqueo.PIN }
        assertTrue(a.tienePin.value)
        regla.esperaTexto("Pedir el desbloqueo")
        regla.onNodeWithText("Cambiar el PIN").assertIsDisplayed()
    }

    @Test fun cambiarElPinPideElActualPrimero() {
        a.fijarPin("1357"); a.fijarBloqueo(Bloqueo.PIN)
        pantalla()
        regla.onNodeWithText("Cambiar el PIN").performScrollTo().performClick()
        regla.onNodeWithText("Escribe tu PIN actual").assertIsDisplayed()
        teclea("0000"); ok()
        regla.esperaTexto("PIN incorrecto", substring = true)
        teclea("1357"); ok()
        regla.esperaTexto("Elige un PIN nuevo")
        teclea("2468"); ok(); teclea("2468"); ok()
        regla.waitUntil(5_000) { a.intentos.probar("2468") is com.botchat.app.seguridad.ResultadoPin.Correcto }
        assertEquals(Bloqueo.PIN, a.bloqueo.value)
    }

    @Test fun quitarElBloqueoPideElPinActual() {
        a.fijarPin("1357"); a.fijarBloqueo(Bloqueo.PIN)
        pantalla()
        regla.onNodeWithTag("op_ninguno").performClick()
        regla.onNodeWithText("Escribe tu PIN actual para quitarlo").assertIsDisplayed()
        regla.onNodeWithText("Cancelar").performClick() // cancelar no quita nada
        regla.esperaTexto("Proteger la app")
        assertEquals(Bloqueo.PIN, a.bloqueo.value); assertTrue(a.tienePin.value)
        regla.onNodeWithTag("op_ninguno").performClick()
        teclea("1357"); ok()
        regla.waitUntil(8_000) { a.bloqueo.value == Bloqueo.NINGUNO }
        assertFalse(a.tienePin.value)
    }

    @Test fun tiempoDeBloqueoYCapturasSeGuardan() {
        a.fijarPin("1357"); a.fijarBloqueo(Bloqueo.PIN)
        pantalla()
        regla.onNodeWithText("5 min").performScrollTo().performClick()
        assertEquals(300, a.bloqueoSeg.value)
        regla.onNodeWithText("Enseguida").performClick()
        assertEquals(0, a.bloqueoSeg.value)
        regla.onNodeWithTag("sw_capturas").performScrollTo().performClick()
        assertTrue(a.secure.value)
        Ajustes.reiniciarParaPruebas()
        val otra = Ajustes.get(Apoyo.ctx())
        assertEquals(0, otra.bloqueoSeg.value); assertTrue(otra.secure.value); assertEquals(Bloqueo.PIN, otra.bloqueo.value)
    }

    @Test fun sinBloqueoDeTelefonoAvisaQueElPinEsLigero() {
        a.fijarPin("1357"); a.fijarBloqueo(Bloqueo.PIN) // en la prueba el teléfono no tiene bloqueo de pantalla
        pantalla()
        regla.onNodeWithText(AVISO_PIN_LIGERO).performScrollTo().assertIsDisplayed()
    }

    @Test fun huellaExplicaConQueRespaldoFuncionaOQueHaceFalta() {
        pantalla()
        // Según el teléfono: sin huella registrada, o con un respaldo (PIN o bloqueo del teléfono); nunca queda sin explicación.
        val textos = listOf("Este teléfono no tiene huella registrada.", "Con un PIN de respaldo.", "Con el bloqueo del teléfono como respaldo.")
        assertTrue(textos.any { regla.onAllNodesWithTextSafe(it) > 0 })
    }

    @Test fun olvidePinSinBloqueoDeTelefonoPideDobleConfirmacionYNoTocaLosDatos() {
        a.fijarPin("1357"); a.fijarBloqueo(Bloqueo.PIN)
        val antes = huellaDeDatos()
        var abierto = false
        regla.setContent { BotChatTema { PantallaBloqueo(a) { abierto = true } } }
        regla.onNodeWithText("Olvidé mi PIN").performClick()
        regla.onNodeWithText("Tu teléfono no tiene bloqueo de pantalla").assertIsDisplayed()
        regla.onNodeWithText("Cancelar").performClick() // cancelar: nada cambia
        assertEquals(Bloqueo.PIN, a.bloqueo.value); assertFalse(abierto)
        regla.onNodeWithText("Olvidé mi PIN").performClick()
        regla.onNodeWithText("Seguir").performClick()
        regla.onNodeWithText("¿Quitar el PIN?").assertIsDisplayed()
        regla.onNodeWithText("Sí, quitar el PIN").performClick()
        assertTrue(abierto)
        assertEquals(Bloqueo.NINGUNO, a.bloqueo.value); assertFalse(a.tienePin.value)
        assertEquals("quitar el PIN no toca la llave ni los archivos cifrados", antes, huellaDeDatos())
    }

    @Test fun quitarElBloqueoNoAlteraLosDatosCifrados() {
        // Un archivo cifrado y la llave en memoria: activar, cambiar y quitar el bloqueo no los modifican.
        val f = File(Apoyo.ctx().filesDir, "media/1/ab/F1.enc").apply { parentFile!!.mkdirs() }
        f.writeBytes(com.botchat.app.seguridad.FormatoCifrado.cifrarBytes(Apoyo.clave, "datos de prueba".toByteArray()))
        val antes = huellaDeDatos()
        a.fijarPin("1357"); a.fijarBloqueo(Bloqueo.PIN); a.fijarBloqueoSeg(300); a.fijarPin("2468")
        a.quitarBloqueo()
        assertEquals(antes, huellaDeDatos())
        assertEquals("datos de prueba", String(com.botchat.app.seguridad.FormatoCifrado.descifrarBytes(Apoyo.clave, f.readBytes())))
        assertTrue("la llave de datos sigue siendo la misma", ClaveDatos.obtener(Apoyo.ctx()).contentEquals(Apoyo.clave))
    }

    @Test fun elBloqueoRespetaElTiempoElegido() {
        a.fijarPin("1357"); a.fijarBloqueo(Bloqueo.PIN)
        val ctx = Apoyo.ctx()
        EstadoBloqueo.iniciar(a); assertTrue(EstadoBloqueo.bloqueado.value) // al abrir, con PIN, pide desbloqueo
        EstadoBloqueo.desbloquear()
        a.fijarBloqueoSeg(60)
        EstadoBloqueo.alSalir(); ShadowSystemClock.advanceBy(Duration.ofSeconds(30)); EstadoBloqueo.alVolver(ctx, a)
        assertFalse("30 s fuera con 1 min: no se bloquea", EstadoBloqueo.bloqueado.value)
        EstadoBloqueo.alSalir(); ShadowSystemClock.advanceBy(Duration.ofSeconds(61)); EstadoBloqueo.alVolver(ctx, a)
        assertTrue("61 s fuera con 1 min: se bloquea", EstadoBloqueo.bloqueado.value)
        EstadoBloqueo.desbloquear(); a.fijarBloqueoSeg(300)
        EstadoBloqueo.alSalir(); ShadowSystemClock.advanceBy(Duration.ofSeconds(200)); EstadoBloqueo.alVolver(ctx, a)
        assertFalse("200 s con 5 min: no se bloquea", EstadoBloqueo.bloqueado.value)
        a.fijarBloqueoSeg(0)
        EstadoBloqueo.alSalir(); EstadoBloqueo.alVolver(ctx, a)
        assertTrue("inmediato: se bloquea al volver", EstadoBloqueo.bloqueado.value)
        // Sin bloqueo nunca se bloquea
        EstadoBloqueo.desbloquear(); a.quitarBloqueo()
        EstadoBloqueo.alSalir(); ShadowSystemClock.advanceBy(Duration.ofHours(1)); EstadoBloqueo.alVolver(ctx, a)
        assertFalse(EstadoBloqueo.bloqueado.value)
    }

    @Test fun actualizarDesdeLaVersionAnteriorConservaElBloqueo() {
        // La 0.2.0 guardaba solo el PIN (y "huella"): se interpreta como bloqueo con PIN, y sin PIN como sin bloqueo.
        Ajustes.reiniciarParaPruebas()
        val seg = Apoyo.ctx().getSharedPreferences("seguridad", android.content.Context.MODE_PRIVATE)
        seg.edit().putString("pin", com.botchat.app.seguridad.Pin.crear("1357")).commit()
        assertEquals(Bloqueo.PIN, Ajustes.get(Apoyo.ctx()).bloqueo.value)
        Ajustes.reiniciarParaPruebas()
        Apoyo.ctx().getSharedPreferences("ajustes", android.content.Context.MODE_PRIVATE).edit().putBoolean("huella", true).commit()
        assertEquals(Bloqueo.HUELLA, Ajustes.get(Apoyo.ctx()).bloqueo.value)
        Ajustes.reiniciarParaPruebas()
        seg.edit().clear().commit(); Apoyo.ctx().getSharedPreferences("ajustes", android.content.Context.MODE_PRIVATE).edit().clear().commit()
        assertEquals(Bloqueo.NINGUNO, Ajustes.get(Apoyo.ctx()).bloqueo.value)
    }

    private fun huellaDeDatos(): String {
        val md = MessageDigest.getInstance("SHA-256")
        File(Apoyo.ctx().filesDir, "media").walkTopDown().filter { it.isFile }.sortedBy { it.path }.forEach { md.update(it.path.toByteArray()); md.update(it.readBytes()) }
        md.update(Apoyo.clave)
        return md.digest().joinToString("") { "%02x".format(it) }
    }
}
