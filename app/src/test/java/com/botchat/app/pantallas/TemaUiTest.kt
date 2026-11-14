package com.botchat.app.pantallas

import androidx.compose.material3.ColorScheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.assertIsDisplayed
import com.botchat.app.datos.Ajustes
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.EstadoVisible
import com.botchat.app.ui.LocalColoresApp
import com.botchat.app.ui.ModoTema
import com.botchat.app.ui.estadoVisible
import com.botchat.app.ui.etiquetaEstado
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import kotlin.math.pow

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h891dp-xxhdpi")
class TemaUiTest {
    @get:Rule val regla = createComposeRule()

    @Before fun antes() { Apoyo.reinicia() }

    private var esquemaActual: ColorScheme? = null
    private var coloresApp: com.botchat.app.ui.ColoresApp? = null
    private var iniciado = false

    /** Cambia el modo de tema y devuelve el esquema que quedó aplicado (una sola composición, que se actualiza). */
    private fun esquema(modo: ModoTema): ColorScheme {
        val ajustes = Ajustes.get(Apoyo.ctx())
        if (!iniciado) {
            iniciado = true
            ajustes.fijarModoTema(modo)
            regla.setContent { BotChatTema { esquemaActual = MaterialTheme.colorScheme; coloresApp = LocalColoresApp.current; Text("tema") } }
        } else {
            ajustes.fijarModoTema(modo)
        }
        regla.waitForIdle()
        regla.onNodeWithText("tema").assertIsDisplayed()
        return esquemaActual!!
    }

    private fun luz(c: Color): Double {
        fun f(x: Float) = if (x <= 0.03928f) x / 12.92 else ((x + 0.055) / 1.055).pow(2.4)
        return 0.2126 * f(c.red) + 0.7152 * f(c.green) + 0.0722 * f(c.blue)
    }
    private fun contraste(a: Color, b: Color): Double { val (x, y) = listOf(luz(a), luz(b)).sortedDescending(); return (x + 0.05) / (y + 0.05) }

    @Test fun oscuroTieneLosColoresDelIcono() {
        val e = esquema(ModoTema.OSCURO)
        assertEquals(Color(0xFF0B141A), e.background); assertEquals(Color(0xFF0B141A), e.surface); assertEquals(Color(0xFF1F2C34), e.surfaceVariant)
        assertEquals(Color(0xFF2EC4CB), e.primary); assertEquals(Color(0xFF00363A), e.onPrimary)
        assertEquals(Color(0xFF0B3A3D), e.primaryContainer); assertEquals(Color(0xFFBFF4F7), e.onPrimaryContainer)
        assertEquals(Color(0xFFF59E28), e.tertiary); assertEquals(Color(0xFF4A2A00), e.onTertiary); assertEquals(Color(0xFF6A3F00), e.tertiaryContainer)
        assertEquals(Color(0xFFE9EDEF), e.onSurface); assertEquals(Color(0xFF667781), e.outline)
        assertTrue(e.background.luminance() < 0.1f)
    }

    @Test fun claroTieneLosColoresDelIcono() {
        val e = esquema(ModoTema.CLARO)
        assertEquals(Color(0xFFFFFFFF), e.background); assertEquals(Color(0xFFFFFFFF), e.surface)
        assertEquals(Color(0xFF00727A), e.primary); assertEquals(Color(0xFFFFFFFF), e.onPrimary); assertEquals(Color(0xFFD3F3F4), e.primaryContainer)
        assertEquals(Color(0xFF8A5100), e.tertiary); assertEquals(Color(0xFFFFDCBA), e.tertiaryContainer); assertEquals(Color(0xFF111B21), e.onSurface)
        assertTrue(e.background.luminance() > 0.9f)
    }

    @Test fun elTextoCumpleContrasteAAEnAmbosTemas() {
        for (modo in listOf(ModoTema.CLARO, ModoTema.OSCURO)) {
            val e = esquema(modo)
            val pares = mapOf(
                "texto/fondo" to (e.onBackground to e.background), "texto/superficie" to (e.onSurface to e.surface),
                "texto/variante" to (e.onSurface to e.surfaceVariant), "texto2/superficie" to (e.onSurfaceVariant to e.surface),
                "primario/fondo" to (e.primary to e.background), "sobrePrimario" to (e.onPrimary to e.primary),
                "sobreContenedor" to (e.onPrimaryContainer to e.primaryContainer), "ámbar/superficie" to (e.tertiary to e.surface),
                "sobreÁmbar" to (e.onTertiary to e.tertiary), "sobreContenedorÁmbar" to (e.onTertiaryContainer to e.tertiaryContainer),
                "error/superficie" to (e.error to e.surface), "sobreError" to (e.onError to e.error), "sobreContenedorError" to (e.onErrorContainer to e.errorContainer),
                "secundario/superficie" to (e.secondary to e.surface), "sobreContenedorSec" to (e.onSecondaryContainer to e.secondaryContainer),
            )
            for ((n, p) in pares) assertTrue("$modo $n: ${"%.2f".format(contraste(p.first, p.second))} < 4.5", contraste(p.first, p.second) >= 4.5)
        }
    }

    @Test fun losColoresDeEstadoExistenEnAmbosTemas() {
        for (modo in listOf(ModoTema.CLARO, ModoTema.OSCURO)) {
            val fondo = esquema(modo).surface
            val c = coloresApp!!
            assertTrue("conectado legible en $modo", contraste(c.conectado, fondo) >= 3.0)
            assertTrue("reconectando legible en $modo", contraste(c.reconectando, fondo) >= 3.0)
        }
    }

    @Test fun elColorDinamicoDelSistemaEstaApagadoPorDefecto() {
        assertEquals(false, Ajustes.get(Apoyo.ctx()).colorDinamico.value)
        val e = esquema(ModoTema.OSCURO) // aun en Android 14 se usa la paleta propia, no la del fondo de pantalla
        assertEquals(Color(0xFF2EC4CB), e.primary)
    }

    @Test fun etiquetasDelEstadoDelBot() {
        assertEquals(EstadoVisible.CONECTADO, estadoVisible("conectado", true, false))
        assertEquals(EstadoVisible.RECONECTANDO, estadoVisible("conectando", true, false))
        assertEquals(EstadoVisible.RECONECTANDO, estadoVisible("error_red", true, false))
        assertEquals(EstadoVisible.DESVINCULADO, estadoVisible("conectado", true, true))
        assertEquals(EstadoVisible.DETENIDO, estadoVisible("detenido", false, false))
        assertEquals(listOf("Conectado", "Reconectando", "Desvinculado", "Detenido"), EstadoVisible.values().map(::etiquetaEstado))
    }
}
