package com.botchat.app.pantallas

import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performScrollToIndex
import com.botchat.app.datos.Repo
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.CachePaginas
import com.botchat.app.ui.CacheTexto
import com.botchat.app.ui.PantallaChat
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

/** La apertura del chat: primera página pequeña y refrescos que mezclan lo nuevo con lo ya cargado, sin releer todo. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1200dp-xxhdpi")
class ChatCargaUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L
    private val jid = "5215512345678@s.whatsapp.net"
    private val ahora = System.currentTimeMillis() / 1000

    /** n mensajes: m(n) el más nuevo … m1 el más viejo. */
    private fun mensajes(n: Int, cambia: (Int) -> JSONObject? = { null }) =
        Apoyo.arreglo(*(n downTo 1).map { cambia(it) ?: Apoyo.msgJson("m$it", ahora - (n - it) * 60L, "texto $it") }.toTypedArray())

    @Before fun antes() {
        motor = Apoyo.reinicia(); bot = Apoyo.creaBot(); CachePaginas.vaciar(); CacheTexto.vaciar()
        motor.chat = Apoyo.chatJson(jid, "Ana Pérez").toString()
    }

    private fun pantalla() = regla.setContent { BotChatTema { PantallaChat(jid, {}, {}, {}, {}) } }
    private fun llamadasA(prefijo: String) = motor.llamadas.toList().filter { it.startsWith(prefijo) }

    @Test fun abreConUnaPrimeraPaginaPequenaYNoMas() {
        motor.mensajes = mensajes(200)
        pantalla()
        regla.esperaTexto("texto 200")
        assertEquals("la primera lectura pide 40", 40L, motor.limites.first())
        assertEquals("una sola lectura de la primera página al abrir", 1, llamadasA("mensajes:").count { it.endsWith(":0") })
        regla.esperaTexto("Ana Pérez")
    }

    @Test fun mensajeNuevoLlegaSinReleerLoYaCargado() {
        motor.mensajes = mensajes(50)
        pantalla()
        regla.esperaTexto("texto 50")
        motor.llamadas.clear(); motor.limites.clear()
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("m51", ahora + 60, "nuevo mensaje"), *(JSON(motor.mensajes)))
        Repo.emitirParaPruebas("chat:$jid", "chats")
        regla.espera { llamadasA("mensajes:").isNotEmpty() }
        // (con la lista parada en otra parte, el mensaje nuevo queda abajo: se baja hasta él)
        regla.espera { regla.desplazaLista(0); regla.onAllNodesWithTextSafe("nuevo mensaje") > 0 }
        assertTrue("solo se vuelve a pedir la primera página: ${motor.limites}", motor.limites.all { it == 40L })
        assertTrue("sin consultas por id", llamadasA("porId:").isEmpty())
        regla.esperaTexto("texto 50")
    }

    @Test fun cambioEnUnMensajeViejoYaCargadoSePideSoloPorId() {
        motor.mensajes = mensajes(60)
        pantalla()
        regla.esperaTexto("texto 60")
        regla.desplazaLista(45) // llega al final: carga la segunda página
        regla.espera { llamadasA("mensajes:$jid:").size > 1 && llamadasA("mensajes:$jid:").any { !it.endsWith(":0") } }
        regla.desplazaLista(55)
        regla.esperaTexto("texto 5")
        motor.llamadas.clear()
        // El mensaje 5 (viejo, fuera de la primera página) fue editado.
        motor.porId["m5"] = Apoyo.msgJson("m5", ahora - 55 * 60L, "texto 5 editado", editado = true).toString()
        Repo.emitirParaPruebas("msg:$jid:m5", "chat:$jid")
        regla.esperaTexto("texto 5 editado", substring = true)
        assertTrue("se pidió solo ese id en un lote: ${llamadasA("porId:")}", llamadasA("porId:").singleOrNull()?.endsWith("""["m5"]""") == true)
        assertEquals("sin duplicados", 1, regla.onAllNodesWithTextSafe("texto 5 editado", substring = true))
        assertEquals(1, regla.onAllNodesWithTextSafe("texto 6", substring = false))
    }

    @Test fun cambiosDeVariosMensajesViejosVanEnUnSoloLote() {
        motor.mensajes = mensajes(60)
        pantalla()
        regla.esperaTexto("texto 60")
        regla.desplazaLista(45)
        regla.espera { llamadasA("mensajes:$jid:").any { !it.endsWith(":0") } }
        motor.llamadas.clear()
        Repo.emitirParaPruebas("msg:$jid:m3", "msg:$jid:m4", "archivo:$jid:m5", "chat:$jid")
        regla.espera { llamadasA("porId:").isNotEmpty() }
        assertEquals(1, llamadasA("porId:").size)
        val ids = llamadasA("porId:").single().substringAfter("$jid:")
        assertTrue(ids, ids.contains("m3") && ids.contains("m4") && ids.contains("m5"))
    }

    @Test fun elMensajeViejoQueYaNoExisteDesaparece() {
        motor.mensajes = mensajes(60)
        pantalla()
        regla.esperaTexto("texto 60")
        regla.desplazaLista(45)
        regla.espera { llamadasA("mensajes:$jid:").any { !it.endsWith(":0") } }
        regla.desplazaLista(55)
        regla.esperaTexto("texto 6", substring = true)
        // «Borrar para mí» el mensaje 6: la base ya no lo devuelve ni en la primera página ni por id.
        motor.porId.clear()
        Repo.emitirParaPruebas("msg:$jid:m6", "chat:$jid")
        regla.espera { regla.onAllNodesWithTextSafe("texto 6", substring = false) == 0 }
        assertTrue(llamadasA("porId:").isNotEmpty())
    }

    @Test fun losCambiosDeLaPrimeraPaginaNoPidenNadaPorId() {
        motor.mensajes = mensajes(10)
        pantalla()
        regla.esperaTexto("texto 10")
        motor.llamadas.clear()
        // un recibo en el mensaje 9, que está en la primera página
        motor.mensajes = mensajes(10) { if (it == 9) Apoyo.msgJson("m9", ahora - 60, "texto 9 leído") else null }
        Repo.emitirParaPruebas("msg:$jid:m9", "chat:$jid")
        regla.esperaTexto("texto 9 leído")
        assertFalse(llamadasA("porId:").isNotEmpty())
    }

    @Test fun alVolverAAbrirElChatSePintaDeLaCacheYSeCorrigeConLaBase() {
        motor.mensajes = mensajes(5)
        pantalla()
        regla.esperaTexto("texto 5")
        assertTrue(CachePaginas.obtener(bot, jid)?.size == 5)
    }

    @Test fun conCargaAnteriorSeReproduceLaRutaDeLa040() {
        kotlinx.coroutines.runBlocking { com.botchat.app.datos.ModoCarga.fijar(true) }
        try {
            assertTrue(motor.cargaAnteriorActiva)
            motor.mensajes = mensajes(100)
            pantalla()
            regla.esperaTexto("texto 100")
            assertEquals("páginas de 60", 60L, motor.limites.first())
            motor.limites.clear()
            Repo.emitirParaPruebas("chat:$jid")
            regla.espera { motor.limites.isNotEmpty() }
            assertTrue("recarga completa de lo ya cargado (${motor.limites})", motor.limites.first() >= 60L)
        } finally {
            kotlinx.coroutines.runBlocking { com.botchat.app.datos.ModoCarga.fijar(false) }
        }
        assertFalse(motor.cargaAnteriorActiva)
    }

    private fun JSON(s: String): Array<JSONObject> = org.json.JSONArray(s).let { a -> Array(a.length()) { a.getJSONObject(it) } }
}
