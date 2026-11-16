package com.botchat.app.pantallas

import androidx.activity.ComponentActivity
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsEnabled
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import com.botchat.app.BotState
import com.botchat.app.datos.Ajustes
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.EstadoBloqueo
import com.botchat.app.ui.Raiz
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
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h891dp-xxhdpi")
class NavegacionUiTest {
    @get:Rule val regla = createAndroidComposeRule<ComponentActivity>()
    private lateinit var motor: MotorFalso
    private val jid = "5215512345678@s.whatsapp.net"

    @Before fun antes() {
        motor = Apoyo.reinicia()
        motor.chatsPersonales = Apoyo.arreglo(Apoyo.chatJson(jid, "María López", texto = "nos vemos"))
        motor.chat = Apoyo.chatJson(jid, "María López").toString()
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("m1", System.currentTimeMillis() / 1000, "nos vemos"))
        motor.espacioPorChat = "[]"
    }

    private fun contenido() = regla.setContent { BotChatTema { Raiz() } }
    private fun atras() { regla.runOnUiThread { regla.activity.onBackPressedDispatcher.onBackPressed() }; regla.waitForIdle() }

    @Test fun sinBloqueoPorDefectoAbreDirectoSinOfrecerPin() {
        Apoyo.creaBot(); val a = Ajustes.get(Apoyo.ctx()); a.aceptarAviso()
        assertEquals(Ajustes.Bloqueo.NINGUNO, a.bloqueo.value)
        EstadoBloqueo.iniciar(a)
        contenido()
        regla.esperaTexto("María López")
        assertEquals(0, regla.onAllNodesWithTextSafe("Protege tus mensajes con un PIN"))
        assertEquals(0, regla.onAllNodesWithTextSafe("está bloqueado", substring = true))
        regla.onNodeWithText("BotChat").assertIsDisplayed() // título de la barra superior
        regla.onNodeWithText("El bot está detenido. Toca para encenderlo").assertIsDisplayed()
    }

    @Test fun estadoDelBotEnLaBarraSuperior() {
        Apoyo.creaBot(); Ajustes.get(Apoyo.ctx()).aceptarAviso()
        BotState.corriendo.value = true; BotState.estado.value = "conectado"
        contenido(); regla.esperaTexto("María López")
        assertEquals("conectado: no hay cinta de aviso", 0, regla.onAllNodesWithTextSafe("detenido", substring = true))
        regla.runOnUiThread { BotState.estado.value = "conectando" }
        regla.esperaTexto("Reconectando", substring = true)
    }

    @Test fun actualizacionYaVinculadaSoloVeElAvisoUnaVez() {
        Apoyo.creaBot(); val a = Ajustes.get(Apoyo.ctx())
        contenido()
        regla.onNodeWithText("Términos y condiciones").assertIsDisplayed()
        assertEquals(1, regla.onAllNodesWithTextSafe("Tu cuenta de WhatsApp")) // los términos completos, no un resumen
        assertEquals(0, regla.onAllNodesWithTextSafe("Hola, esto es Bot Chat")) // sin la bienvenida completa
        regla.onNodeWithText("Continuar").assertIsNotEnabled()
        regla.onNodeWithTag("casilla_acepto").performClick()
        regla.onNodeWithText("Continuar").assertIsEnabled().performClick()
        regla.esperaTexto("María López") // directo a los chats: no pide vincular de nuevo
        assertTrue(a.avisoAceptado.value)
        assertEquals(0, regla.onAllNodesWithTextSafe("¿Cómo se llamará tu bot?"))
    }

    @Test fun botNuevoVeLaBienvenidaYDebeAceptarAntesDeVincular() {
        contenido() // sin bot
        regla.onNodeWithText("Hola, esto es Bot Chat").assertIsDisplayed()
        regla.onNodeWithText("Siguiente").performClick()
        regla.esperaTexto("Todo se queda en tu teléfono")
        regla.onNodeWithText("Siguiente").performClick()
        regla.esperaTexto("Términos y condiciones")
        regla.onNodeWithText("Empezar").assertIsNotEnabled()
        regla.onNodeWithTag("casilla_acepto").performClick()
        regla.onNodeWithText("Empezar").assertIsEnabled().performClick()
        regla.waitForIdle()
        regla.esperaTexto("¿Cómo se llamará tu bot?")
    }

    @Test fun losTerminosSeAceptanUnaVezYDespuesSoloSeConsultan() {
        Apoyo.creaBot(); val a = Ajustes.get(Apoyo.ctx()); a.aceptarAviso()
        contenido(); regla.esperaTexto("María López")
        assertEquals(0, regla.onAllNodesWithTextSafe("He leído y acepto los términos y condiciones")) // no vuelve a pedirse
        regla.onNodeWithTag("nav_bot").performClick()
        regla.esperaEtiqueta("hub_ajustes")
        regla.onNodeWithTag("hub_ajustes").performScrollTo().performClick()
        regla.esperaEtiqueta("ajuste_terminos")
        regla.onNodeWithTag("ajuste_terminos").performScrollTo().performClick()
        regla.esperaEtiqueta("texto_terminos")
        regla.esperaTexto("Mensajes automáticos")
        regla.onNodeWithTag("cerrar_terminos").performClick()
        regla.waitForIdle()
        assertEquals(0, regla.onAllNodesWithTextSafe("Mensajes automáticos"))
    }

    @Test fun recorridoPorLasPantallas() {
        Apoyo.creaBot(); Ajustes.get(Apoyo.ctx()).aceptarAviso()
        contenido(); regla.esperaTexto("María López")
        // pestañas de abajo (0.6.0): Chats, Personas, Grupos y Bot; Almacenamiento y Ajustes se abren desde Bot
        regla.onNodeWithTag("nav_bot").performClick()
        regla.esperaEtiqueta("hub_almacenamiento")
        regla.onNodeWithTag("hub_almacenamiento").performScrollTo().performClick()
        regla.esperaTexto("Lo que guardo de tus chats") // E8: el medidor contra el tope va primero
        atras()
        regla.esperaEtiqueta("hub_ajustes")
        regla.onNodeWithTag("hub_ajustes").performScrollTo().performClick()
        regla.esperaTexto("Seguridad")
        regla.onNodeWithTag("ajuste_seguridad").performScrollTo().performClick()
        regla.esperaTexto("Proteger la app")
        regla.onNodeWithText("Sin bloqueo").assertIsDisplayed()
        atras()
        regla.onNodeWithTag("ajuste_descargas").performScrollTo().performClick()
        regla.esperaTexto("Descarga automática")
        atras()
        regla.onNodeWithTag("ajuste_avanzado").performScrollTo().performClick()
        regla.esperaTexto("Aspecto")
        regla.onNodeWithText("Medir rendimiento").assertDoesNotExist()
        atras()
        regla.onNodeWithTag("ajuste_bot").performScrollTo().performClick()
        regla.esperaTexto("Iniciar")
        atras()
        atras() // sale de Ajustes y vuelve a Bot
        // chat: la navegación de abajo se esconde y se puede entrar a los archivos compartidos
        regla.onNodeWithTag("nav_inicio").performClick()
        regla.esperaTexto("María López")
        regla.onNodeWithText("María López").performClick()
        regla.esperaTexto("nos vemos", substring = false)
        regla.onNodeWithTag("nav_inicio").assertDoesNotExist()
        regla.onNodeWithContentDescription("Más opciones").performClick()
        regla.esperaEtiqueta("menu_archivos"); regla.onNodeWithTag("menu_archivos").performClick() // "Archivos, enlaces y docs" ahora va en el menú
        regla.esperaTexto("Archivos compartidos")
        regla.onNodeWithText("Fotos").assertIsDisplayed(); regla.onNodeWithText("Videos").assertIsDisplayed()
        atras(); atras()
        regla.onNodeWithTag("nav_inicio").assertIsDisplayed()
    }

    @Test fun conPinPideDesbloquearYAlBloquearLimpiaLoDescifrado() {
        Apoyo.creaBot()
        val a = Ajustes.get(Apoyo.ctx()); a.aceptarAviso(); a.fijarPin("2580"); a.fijarBloqueo(Ajustes.Bloqueo.PIN)
        EstadoBloqueo.iniciar(a)
        contenido()
        regla.onNodeWithText("Bot Chat está bloqueado").assertIsDisplayed()
        assertEquals(0, regla.onAllNodesWithTextSafe("María López")) // nada de los mensajes antes del PIN
        "2580".forEach { regla.onNodeWithText(it.toString()).performClick() }
        regla.onNodeWithText("OK").performClick()
        regla.esperaTexto("María López")
        val temporal = java.io.File(Apoyo.ctx().cacheDir, "tmp_descifrado/x/secreto.pdf").apply { parentFile!!.mkdirs(); writeText("dato") }
        regla.runOnUiThread { EstadoBloqueo.bloquear(Apoyo.ctx()) }
        regla.esperaTexto("Bot Chat está bloqueado")
        assertEquals(0, regla.onAllNodesWithTextSafe("María López"))
        assertFalse("las copias temporales se borran al bloquear", temporal.exists())
    }

    @Test fun elServicioYLosMensajesNoDependenDelBloqueo() {
        // Con el bloqueo apagado todo funciona: los mensajes se leen y el estado del bot sigue llegando.
        Apoyo.creaBot(); val a = Ajustes.get(Apoyo.ctx()); a.aceptarAviso()
        BotState.corriendo.value = true; BotState.estado.value = "conectado"
        contenido(); regla.esperaTexto("María López")
        assertFalse(EstadoBloqueo.bloqueado.value)
        assertEquals(0, regla.onAllNodesWithTextSafe("detenido", substring = true))
        regla.runOnUiThread { com.botchat.app.datos.Repo.emitirParaPruebas("chats") }
        regla.esperaTexto("María López")
    }

    @Test fun sinSesionApareceVincularDeNuevo() {
        Apoyo.creaBot(); Ajustes.get(Apoyo.ctx()).aceptarAviso()
        com.botchat.app.Datos.get(Apoyo.ctx()).marcarVinculado(1, false) // WhatsApp cerró la sesión; el bot se había vinculado antes
        contenido()
        regla.esperaTexto("WhatsApp cerró la sesión de este bot")
        regla.onNodeWithText("WhatsApp cerró la sesión. Toca para ver qué hacer").assertIsDisplayed()
        regla.onNodeWithText("María López").assertIsDisplayed() // los mensajes siguen ahí
        regla.onNodeWithText("Vincular de nuevo").performClick()
        regla.esperaTexto("Vincular de nuevo")
        regla.onNodeWithText("Tus mensajes guardados no se pierden.", substring = true).assertIsDisplayed()
        regla.onNodeWithText("Sí, vincular").assertIsDisplayed()
    }
}
