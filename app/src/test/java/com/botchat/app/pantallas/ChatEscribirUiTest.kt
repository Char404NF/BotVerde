package com.botchat.app.pantallas

import android.content.ClipboardManager
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.longClick
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollToIndex
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.performTouchInput
import com.botchat.app.datos.Ajustes
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaChat
import org.json.JSONArray
import org.json.JSONObject
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
class ChatEscribirUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private val jid = "5215512345678@s.whatsapp.net"
    private val ahora = System.currentTimeMillis() / 1000

    @Before fun antes() { motor = Apoyo.reinicia(); Apoyo.creaBot(); motor.chat = Apoyo.chatJson(jid, "Ana Pérez").toString() }

    private fun pantalla(chat: String = jid) = regla.setContent { BotChatTema { PantallaChat(chat, {}, {}, {}, {}) } }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun escribe(t: String) { regla.esperaEtiqueta("campo_mensaje"); regla.onNodeWithTag("campo_mensaje").performTextInput(t) }
    private fun selecciona(id: String) { regla.esperaEtiqueta("burbuja_$id"); regla.onNodeWithTag("burbuja_$id").performTouchInput { longClick() }; regla.esperaEtiqueta("contador_seleccion") }
    private fun llamo(prefijo: String) = motor.llamadas.toList().any { it.startsWith(prefijo) }

    @Test fun elBorradorSeRecuperaYSeGuardaAlEscribir() {
        motor.borrador = "borrador viejo"
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola"))
        pantalla()
        regla.esperaTexto("borrador viejo")
        escribe(" y más")
        regla.espera { llamo("borrador:borrador viejo y más") }
    }

    @Test fun enviarManda_VaciaLaCajaYSeMarcaComoManual() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola"))
        pantalla()
        escribe("¿Vienes hoy?")
        regla.onNodeWithTag("enviar").performClick()
        regla.espera { llamo("enviar:$jid:¿Vienes hoy?::[]") }
        regla.espera { regla.onAllNodesWithTextSafe("¿Vienes hoy?") == 0 }
    }

    @Test fun citarConLaSeleccionYEnviarMandaLaCita() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "¿a qué hora?", remitente = "Ana"))
        pantalla()
        selecciona("a")
        regla.onNodeWithTag("accion_responder").performClick()
        regla.esperaEtiqueta("banner_cita")
        regla.onNodeWithText("Respondiendo a Ana", useUnmergedTree = true).assertIsDisplayed()
        escribe("a las 5")
        regla.onNodeWithTag("enviar").performClick()
        regla.espera { llamo("enviar:$jid:a las 5:a:[]") }
        regla.espera { !hay("banner_cita") }
    }

    @Test fun lasCitasRecibidasMuestranElOriginal() {
        motor.mensajes = Apoyo.arreglo(
            Apoyo.msgJson("b", ahora, texto = "a las 5", mio = true).put("citaId", "a").put("citaAutor", "Ana").put("citaTexto", "¿a qué hora?"),
            Apoyo.msgJson("a", ahora - 60, texto = "¿a qué hora?", remitente = "Ana"),
        )
        pantalla()
        regla.esperaEtiqueta("cita_b")
        assertEquals("la cita y el original", 2, regla.onAllNodesWithText("¿a qué hora?", useUnmergedTree = true).fetchSemanticsNodes().size)
    }

    @Test fun reaccionarConUnEmojiRapidoYConCualquierEmoji() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "mira", remitente = "Ana"))
        pantalla()
        selecciona("a")
        regla.esperaEtiqueta("barra_reacciones")
        regla.onNodeWithTag("emoji_👍").performClick()
        regla.espera { llamo("reaccion:a:👍") }
        selecciona("a")
        regla.onNodeWithTag("emoji_mas").performClick()
        regla.esperaEtiqueta("campo_emoji")
        regla.onNodeWithTag("campo_emoji").performTextInput("🦄")
        regla.onNodeWithTag("aceptar_emoji").performClick()
        regla.espera { llamo("reaccion:a:🦄") }
    }

    @Test fun lasReaccionesRecibidasSeVenAgrupadas() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "mira", remitente = "Ana").put("reacciones", JSONArray().put(JSONObject().put("emoji", "👍").put("cantidad", 3).put("mia", false))))
        pantalla()
        regla.esperaEtiqueta("reacciones")
        regla.onNodeWithText("👍 3", useUnmergedTree = true).assertIsDisplayed()
    }

    @Test fun borrarParaMiYParaTodos() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "mío", mio = true), Apoyo.msgJson("b", ahora - 5, texto = "de ella", remitente = "Ana"))
        pantalla()
        selecciona("b")
        regla.onNodeWithTag("accion_borrar").performClick()
        regla.esperaEtiqueta("borrar_para_mi")
        assertFalse("en un chat privado no se borra lo ajeno para todos", hay("borrar_para_todos"))
        regla.onNodeWithTag("borrar_para_mi").performClick()
        regla.espera { llamo("borrarMi:[\"b\"]") }
        selecciona("a")
        regla.onNodeWithTag("accion_borrar").performClick()
        regla.esperaEtiqueta("borrar_para_todos")
        regla.onNodeWithTag("borrar_para_todos").performClick()
        regla.espera { llamo("borrarTodos:a") }
    }

    @Test fun siWhatsAppRechazaBorrarParaTodosSeVeElError() {
        motor.envioFalla = "no se puede"
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "mío", mio = true))
        pantalla()
        selecciona("a")
        regla.onNodeWithTag("accion_borrar").performClick()
        regla.esperaEtiqueta("borrar_para_todos")
        regla.onNodeWithTag("borrar_para_todos").performClick()
        regla.espera { llamo("borrarTodos:a") }
    }

    @Test fun editarSoloDentroDelPlazoYGuardaElCambio() {
        motor.plazo = 1200
        motor.mensajes = Apoyo.arreglo(
            Apoyo.msgJson("nuevo", ahora - 60, texto = "reciente", mio = true),
            Apoyo.msgJson("viejo", ahora - 3000, texto = "antiguo", mio = true),
        )
        pantalla()
        selecciona("viejo")
        assertFalse("pasado el plazo se oculta Editar", hay("accion_editar"))
        regla.onNodeWithTag("cerrar_seleccion").performClick()
        regla.espera { !hay("contador_seleccion") }
        selecciona("nuevo")
        regla.esperaEtiqueta("accion_mas"); regla.onNodeWithTag("accion_mas").performClick()
        regla.esperaEtiqueta("accion_editar")
        regla.onNodeWithTag("accion_editar").performClick()
        regla.esperaEtiqueta("banner_edicion")
        regla.esperaTexto("reciente")
        regla.onNodeWithTag("campo_mensaje").performTextInput(" corregido")
        regla.onNodeWithTag("enviar").performClick()
        regla.espera { llamo("editar:nuevo:reciente corregido") }
    }

    @Test fun lasPalomitasSiguenElEstadoDelMensaje() {
        motor.mensajes = Apoyo.arreglo(
            Apoyo.msgJson("l", ahora, texto = "leído", mio = true).put("estadoEnvio", "leido"),
            Apoyo.msgJson("e", ahora - 1, texto = "entregado", mio = true).put("estadoEnvio", "entregado"),
            Apoyo.msgJson("s", ahora - 2, texto = "enviado", mio = true).put("estadoEnvio", "enviado"),
            Apoyo.msgJson("c", ahora - 3, texto = "enviando", mio = true).put("estadoEnvio", "enviando"),
        )
        pantalla()
        regla.esperaEtiqueta("estado_leido")
        for (t in listOf("estado_leido", "estado_entregado", "estado_enviado", "estado_enviando")) assertTrue(t, hay(t))
    }

    @Test fun unEnvioFallidoOfreceReintentar() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("f", ahora, texto = "no salió", mio = true).put("estadoEnvio", "fallido"))
        pantalla()
        regla.esperaEtiqueta("reintentar_f")
        regla.onNodeWithTag("reintentar_f").performClick()
        regla.espera { llamo("reintentar:f") }
    }

    @Test fun copiarPoneElTextoEnElPortapapeles() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "copiame", remitente = "Ana"))
        pantalla()
        selecciona("a")
        regla.onNodeWithTag("accion_copiar").performClick()
        val cm = Apoyo.ctx().getSystemService(ClipboardManager::class.java)
        regla.espera { cm.primaryClip?.getItemAt(0)?.text?.toString() == "copiame" }
    }

    @Test fun reenviarElegirChatsYConfirmar() {
        motor.chatsPersonales = Apoyo.arreglo(Apoyo.chatJson("1@s.whatsapp.net", "Beto"), Apoyo.chatJson("2@s.whatsapp.net", "Carla"))
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "para pasar", remitente = "Ana"))
        pantalla()
        selecciona("a")
        regla.onNodeWithTag("accion_reenviar").performClick()
        regla.esperaEtiqueta("destino_1@s.whatsapp.net")
        regla.onNodeWithTag("destino_1@s.whatsapp.net").performClick()
        regla.onNodeWithTag("destino_2@s.whatsapp.net").performClick()
        regla.onNodeWithTag("confirmar_reenviar").performClick()
        regla.espera { llamo("reenviar:[\"a\"]:[\"1@s.whatsapp.net\",\"2@s.whatsapp.net\"]") }
    }

    @Test fun elReenviadoLlevaSuEtiqueta() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "pasado", remitente = "Ana").put("reenviado", true))
        pantalla()
        regla.esperaEtiqueta("reenviado")
    }

    @Test fun elEncabezadoMuestraEscribiendoEnLineaOUltimaVezSoloSiLlega() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola", remitente = "Ana"))
        pantalla()
        regla.esperaTexto("Bot activo")   // sin datos de presencia: el estado del bot
        assertFalse(hay("presencia"))
    }

    @Test fun conPresenciaSeMuestraEscribiendo() {
        motor.presencia = """{"escribiendo":true,"enLinea":true,"ultimaVez":0}"""
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola", remitente = "Ana"))
        pantalla()
        regla.esperaTexto("escribiendo…")
    }

    @Test fun conUltimaVezSeMuestraLaHora() {
        motor.presencia = """{"escribiendo":false,"enLinea":false,"ultimaVez":${ahora - 120}}"""
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola", remitente = "Ana"))
        pantalla()
        regla.esperaTexto("últ. vez", substring = true)
    }

    @Test fun alAbrirSeMarcaLeidoSoloSiElInterruptorEstaEncendido() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola", remitente = "Ana"))
        pantalla()
        regla.espera { llamo("leido:$jid") }
    }

    @Test fun conElInterruptorApagadoNuncaSeMarcaLeido() {
        Ajustes.get(Apoyo.ctx()).fijarMarcarLeidoAlAbrir(false)
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola", remitente = "Ana"))
        pantalla()
        regla.esperaTexto("hola")
        Thread.sleep(1500); regla.waitForIdle()
        assertFalse(llamo("leido:"))
    }

    @Test fun enLineaSoloConElInterruptorYConUnChatAbierto() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola", remitente = "Ana"))
        pantalla()
        regla.esperaTexto("hola")
        Thread.sleep(500); regla.waitForIdle()
        assertFalse("por defecto nunca se manda 'en línea'", llamo("enLinea:true"))
    }

    @Test fun conElInterruptorEncendidoSeMarcaEnLineaYSeSuscribe() {
        Ajustes.get(Apoyo.ctx()).fijarMostrarEnLinea(true)
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola", remitente = "Ana"))
        pantalla()
        regla.espera { llamo("enLinea:true") && llamo("suscribir:$jid") }
    }

    @Test fun lasMencionesEnGruposUsanElSelectorYMandanElJid() {
        val grupo = "120@g.us"
        motor.chat = Apoyo.chatJson(grupo, "Familia", grupo = true).toString()
        motor.participantes = """[{"jid":"777@lid","usuario":"777","nombre":"Ana Pérez","telefono":"+5215522222222","esAdmin":false},{"jid":"888@lid","usuario":"888","nombre":"Beto","telefono":"","esAdmin":false}]"""
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola", remitente = "Ana"))
        pantalla(grupo)
        regla.esperaTexto("hola")
        regla.espera { true }
        escribe("hola @an")
        regla.esperaEtiqueta("mencion_777")
        assertFalse(hay("mencion_888"))
        regla.onNodeWithTag("mencion_777").performClick()
        regla.esperaTexto("hola @Ana Pérez", substring = true)
        regla.onNodeWithTag("enviar").performClick()
        regla.espera { llamo("enviar:$grupo:hola @777::[\"777@lid\"]") }
    }

    @Test fun textoConFormatoSeMuestraSinMarcadores() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "esto es *importante* y _urgente_", remitente = "Ana"))
        pantalla()
        regla.esperaTexto("esto es importante y urgente")
    }

    @Test fun losBotonesDeFormatoMetenLosMarcadores() {
        pantalla()
        regla.esperaEtiqueta("boton_formato")
        regla.onNodeWithTag("boton_formato").performClick()
        regla.esperaEtiqueta("formato_Negrita")
        escribe("palabra")
        regla.onNodeWithTag("formato_Negrita").performClick()
        regla.esperaTexto("palabra**")
    }

    private fun desdeHace(seg: Long) { Apoyo.ctx().getSharedPreferences("vistos", android.content.Context.MODE_PRIVATE).edit().putLong("_base", ahora - seg).commit() }

    @Test fun separadorDeSinLeerYBotonParaBajar() {
        desdeHace(1000)
        // Más mensajes que lo que cabe en pantalla, todos sin leer (pero menos que una página: nada se carga después y la lista no se mueve).
        val muchos = (0 until 30).map { Apoyo.msgJson("m$it", ahora - 500 + (30 - it), texto = "mensaje $it", remitente = "Ana") }
        motor.mensajes = Apoyo.arreglo(*muchos.toTypedArray())
        pantalla()
        regla.esperaTexto("30 mensajes sin leer")
        assertTrue(llamo("noLeidos:$jid:${ahora - 1000}"))
        // La lista abre ya colocada en el primer mensaje sin leer: la línea está a la vista (sin desplazarse a mano).
        regla.onNodeWithTag("sin_leer", useUnmergedTree = true).assertIsDisplayed()
        regla.onNodeWithTag("lista_chat").assertIsDisplayed()
    }

    @Test fun elConteoRealPasaDeLaPrimeraPaginaDe40() {
        desdeHace(1000)
        val muchos = (0 until 80).map { Apoyo.msgJson("m$it", ahora - 500 + (80 - it), texto = "mensaje $it", remitente = "Ana") }
        motor.mensajes = Apoyo.arreglo(*muchos.toTypedArray())
        pantalla()
        regla.esperaTexto("80 mensajes sin leer")
        assertTrue(llamo("noLeidos:$jid:${ahora - 1000}"))
    }

    @Test fun elContadorDeSinLeerUsaElConteoDeLaBaseAunqueElChatSeaEnorme() {
        desdeHace(1000)
        val muchos = (0 until 30).map { Apoyo.msgJson("m$it", ahora - 500 + (30 - it), texto = "mensaje $it", remitente = "Ana") }
        motor.mensajes = Apoyo.arreglo(*muchos.toTypedArray())
        motor.noLeidosReales = 12_345
        pantalla()
        regla.esperaTexto("12345 mensajes sin leer")
    }

    @Test fun conUnSoloSinLeerDiceUnMensajeSinLeer() {
        desdeHace(1000)
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora - 300, texto = "viejo", mio = true), Apoyo.msgJson("b", ahora - 200, texto = "nuevo", remitente = "Ana"))
        pantalla()
        regla.esperaEtiqueta("sin_leer")
        regla.onNodeWithText("1 mensaje sin leer", useUnmergedTree = true).assertIsDisplayed()
    }

    @Test fun conEscribirDesdeLaAppApagadoNoHayCajaDeMensaje() {
        Ajustes.get(Apoyo.ctx()).fijarEscribirDesdeApp(false)
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola", remitente = "Ana"))
        pantalla()
        regla.esperaTexto("hola")
        assertFalse(hay("campo_mensaje"))
        selecciona("a")
        assertFalse(hay("accion_responder"))
        assertFalse(hay("accion_reenviar"))
    }

    @Test fun silenciarElChatDesdeElMenu() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "hola", remitente = "Ana"))
        pantalla()
        regla.esperaTexto("hola")
        regla.onNodeWithTag("encabezado_info").assertIsDisplayed()
        regla.onNodeWithContentDescription("Más opciones").performClick()
        regla.esperaEtiqueta("menu_silenciar")
        regla.onNodeWithTag("menu_silenciar").performClick()
        regla.espera { llamo("silenciar:true") }
    }

    @Test fun infoDelMensajeMuestraHorasDeEntregaYLectura() {
        motor.info = """{"enviado":$ahora,"estado":"leido","esGrupo":false,"personas":[{"jid":"x","nombre":"Ana","entregado":${ahora + 1},"leido":${ahora + 5},"reproducido":0}],"editado":false,"reenviado":false}"""
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("a", ahora, texto = "mío", mio = true))
        pantalla()
        selecciona("a")
        regla.onNodeWithTag("accion_mas").performClick()
        regla.esperaEtiqueta("accion_info")
        regla.onNodeWithTag("accion_info").performClick()
        regla.esperaEtiqueta("info_mensaje")
        regla.onNodeWithText("Leído:", substring = true, useUnmergedTree = true).assertIsDisplayed()
    }

    @Test fun elBotonDeBajarAparecePasadoElFinalDeLaLista() {
        val muchos = (0 until 80).map { Apoyo.msgJson("m$it", ahora - it, texto = "mensaje $it", remitente = "Ana") }
        motor.mensajes = Apoyo.arreglo(*muchos.toTypedArray())
        pantalla()
        regla.esperaEtiqueta("lista_chat")
        assertFalse(hay("bajar_al_final"))
        regla.desplazaLista(30)
        regla.esperaEtiqueta("bajar_al_final")
        assertEquals(true, hay("bajar_al_final"))
    }
}
