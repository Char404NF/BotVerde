package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTouchInput
import androidx.compose.ui.test.longClick
import com.botchat.app.datos.Ajustes
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaChat
import com.botchat.app.ui.PantallaFunciones
import com.botchat.app.ui.PantallaPrivacidad
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class EliminadosUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private val jid = "5215512345678@s.whatsapp.net"
    private val ahora = System.currentTimeMillis() / 1000

    @Before fun antes() { motor = Apoyo.reinicia(); Apoyo.creaBot(); motor.chat = Apoyo.chatJson(jid, "Ana Pérez").toString() }

    private fun pantalla() = regla.setContent { BotChatTema { PantallaChat(jid, {}, {}, {}, {}) } }

    @Test fun elEliminadoConservadoMuestraElOriginalYLaEtiqueta() {
        motor.mensajes = Apoyo.arreglo(
            Apoyo.msgJson("t", ahora, texto = "Esto lo borraron", borrado = true).put("eliminadoEn", ahora + 60).put("eliminadoPor", "Admina"),
            Apoyo.msgJson("v", ahora - 10, texto = "Eliminado sin hora", borrado = true),
        )
        pantalla()
        regla.esperaTexto("Esto lo borraron")
        regla.onNodeWithText("Eliminado sin hora").assertIsDisplayed()
        val etiquetas = regla.onAllNodesWithTag("etiqueta_eliminado", useUnmergedTree = true).fetchSemanticsNodes().size
        assertEquals(2, etiquetas)
        assertEquals(1, regla.onAllNodesWithText("por Admina", substring = true, useUnmergedTree = true).fetchSemanticsNodes().size)
        assertEquals("Ya no se tapa con 'Mensaje borrado'", 0, regla.onAllNodesWithText("Mensaje borrado").fetchSemanticsNodes().size)
    }

    @Test fun sinContenidoDiceQueNoEstaDisponible() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("t", ahora, texto = "", borrado = true))
        pantalla()
        regla.esperaTexto("Eliminado · contenido no disponible")
    }

    @Test fun multimediaSinDescargarSeVeComoTarjetaConTipoYTamano() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("f", ahora, tipo = "imagen", bytes = 2048, borrado = true, estado = "ninguno", puedeBajarse = true).put("eliminadoEn", ahora))
        pantalla()
        regla.esperaEtiqueta("eliminado_sin_descargar")
        regla.onNodeWithText("Foto · 2 KB", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithText("Eliminado antes de descargarse", useUnmergedTree = true).assertIsDisplayed()
        assertEquals("Un eliminado no ofrece Descargar", 0, regla.onAllNodesWithText("Descargar", substring = true).fetchSemanticsNodes().size)
    }

    @Test fun olvidarUnMensajePideConfirmacionYLoBorra() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("t", ahora, texto = "para olvidar"))
        pantalla()
        regla.esperaTexto("para olvidar")
        regla.onNodeWithTag("burbuja_t").performTouchInput { longClick() }
        regla.esperaEtiqueta("accion_mas")
        regla.onNodeWithTag("accion_mas").performClick()
        regla.esperaEtiqueta("olvidar_mensaje")
        regla.onNodeWithTag("olvidar_mensaje").performClick()
        regla.esperaEtiqueta("confirmar_olvidar")
        assertFalse(motor.llamadas.any { it.startsWith("olvidar:") })
        regla.onNodeWithTag("confirmar_olvidar").performClick()
        regla.espera { motor.llamadas.contains("olvidar:$jid:t") }
    }

    @Test fun irAlMensajeDesplazaYDestacaElMensaje() {
        val muchos = (0 until 150).map { Apoyo.msgJson("m$it", ahora - it, texto = "mensaje $it") }
        motor.mensajes = Apoyo.arreglo(*muchos.take(60).toTypedArray())
        regla.setContent { BotChatTema { PantallaChat(jid, {}, {}, {}, {}, irA = "m3") } }
        regla.esperaTexto("mensaje 3")
        regla.onNodeWithText("mensaje 3").assertIsDisplayed()
    }

    @Test fun privacidadConservarEliminadosEstaEncendidoPorDefectoYSeApaga() {
        val ajustes = Ajustes.get(Apoyo.ctx())
        regla.setContent { BotChatTema { PantallaPrivacidad(ajustes, {}) } }
        regla.esperaTexto("Conservar mensajes eliminados por otros")
        regla.onNodeWithText("herramienta local", substring = true).assertIsDisplayed()
        regla.onNodeWithText("bajo tu responsabilidad", substring = true).assertIsDisplayed()
        assertTrue(ajustes.conservarEliminados.value)
        regla.onNodeWithTag("sw_Conservar mensajes eliminados por otros").performClick()
        regla.espera { !ajustes.conservarEliminados.value }
    }

    @Test fun funcionesTraeLosInterruptoresConSusValoresPorDefecto() {
        val a = Ajustes.get(Apoyo.ctx())
        regla.setContent { BotChatTema { PantallaFunciones(a, {}) } }
        regla.esperaTexto("Motor de automatización")
        assertTrue(a.motorAutomatico.value); assertTrue(a.escribirDesdeApp.value); assertTrue(a.conservarEliminados.value)
        assertFalse(a.notificacionesMensajes.value); assertFalse(a.mostrarEnLinea.value)
        regla.onNodeWithTag("sw_Motor de automatización").performClick()
        regla.onNodeWithTag("sw_Mostrar en línea").performClick()
        regla.espera { !a.motorAutomatico.value && a.mostrarEnLinea.value }
    }
}
