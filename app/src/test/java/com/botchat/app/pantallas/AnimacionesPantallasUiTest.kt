package com.botchat.app.pantallas

import android.content.Context
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Text
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.botchat.app.datos.Ajustes
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.LocalModoAnim
import com.botchat.app.ui.LocalReducirMovimiento
import com.botchat.app.ui.ModoAnim
import com.botchat.app.ui.PantallaAvanzado
import com.botchat.app.ui.TipoTransicion
import com.botchat.app.ui.Transiciones
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** Bloque 0 (v0.5.0): "Animaciones de pantallas" Natural / Reducidas / Ninguna, su migración y las transiciones reales de la navegación. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h891dp-xxhdpi")
class AnimacionesPantallasUiTest {
    @get:Rule val regla = createComposeRule()

    @Before fun antes() { Apoyo.reinicia() }

    private fun prefs() = Apoyo.ctx().getSharedPreferences("ajustes", Context.MODE_PRIVATE)

    // ---------- migración del booleano reducir_anim ----------

    @Test fun sinNadaGuardadoEmpiezaEnNatural() {
        Ajustes.reiniciarParaPruebas()
        assertEquals(ModoAnim.NATURAL, Ajustes.get(Apoyo.ctx()).animaciones.value)
    }

    @Test fun reducirAnimEncendidoPasaAReducidasYSeBorraElBooleano() {
        prefs().edit().putBoolean("reducir_anim", true).commit()
        Ajustes.reiniciarParaPruebas()
        assertEquals(ModoAnim.REDUCIDAS, Ajustes.get(Apoyo.ctx()).animaciones.value)
        assertEquals("REDUCIDAS", prefs().getString("anim_modo", null))
        assertFalse("el booleano viejo ya no existe", prefs().contains("reducir_anim"))
    }

    @Test fun reducirAnimApagadoPasaANatural() {
        prefs().edit().putBoolean("reducir_anim", false).commit()
        Ajustes.reiniciarParaPruebas()
        assertEquals(ModoAnim.NATURAL, Ajustes.get(Apoyo.ctx()).animaciones.value)
        assertEquals("NATURAL", prefs().getString("anim_modo", null))
    }

    @Test fun laMigracionNoPisaUnaEleccionNueva() {
        prefs().edit().putString("anim_modo", "NINGUNA").putBoolean("reducir_anim", true).commit()
        Ajustes.reiniciarParaPruebas()
        assertEquals(ModoAnim.NINGUNA, Ajustes.get(Apoyo.ctx()).animaciones.value)
    }

    @Test fun unValorRaroSeTrataComoSiNoHubieraNada() {
        prefs().edit().putString("anim_modo", "LOQUESEA").putBoolean("reducir_anim", true).commit()
        Ajustes.reiniciarParaPruebas()
        assertEquals(ModoAnim.REDUCIDAS, Ajustes.get(Apoyo.ctx()).animaciones.value)
    }

    // ---------- Avanzado → Animaciones de pantallas ----------

    @Test fun avanzadoOfreceLosTresModosYLosGuarda() {
        val ajustes = Ajustes.get(Apoyo.ctx())
        regla.setContent { BotChatTema { PantallaAvanzado(ajustes, "Mi bot", false, {}, {}, {}, {}) } }
        regla.onNodeWithTag("anim_modo").performScrollTo()
        regla.onNodeWithTag("anim_ninguna").performScrollTo().performClick()
        assertEquals(ModoAnim.NINGUNA, ajustes.animaciones.value)
        regla.onNodeWithTag("anim_reducidas").performScrollTo().performClick()
        assertEquals(ModoAnim.REDUCIDAS, ajustes.animaciones.value)
        regla.onNodeWithTag("anim_natural").performScrollTo().performClick()
        assertEquals(ModoAnim.NATURAL, ajustes.animaciones.value)
        regla.onNodeWithTag("anim_ninguna").performScrollTo().performClick()
        Ajustes.reiniciarParaPruebas()
        assertEquals("se conserva al reabrir", ModoAnim.NINGUNA, Ajustes.get(Apoyo.ctx()).animaciones.value)
    }

    @Test fun elTemaPublicaElModoYEsCoherenteConElMovimientoReducido() {
        var modo: ModoAnim? = null; var reducir: Boolean? = null
        regla.setContent { BotChatTema { modo = LocalModoAnim.current; reducir = LocalReducirMovimiento.current; Text("a") } }
        assertEquals(ModoAnim.NATURAL, modo); assertEquals(false, reducir)
        Ajustes.get(Apoyo.ctx()).fijarAnimaciones(ModoAnim.REDUCIDAS); regla.waitForIdle()
        assertEquals(ModoAnim.REDUCIDAS, modo); assertEquals(true, reducir)
        Ajustes.get(Apoyo.ctx()).fijarAnimaciones(ModoAnim.NINGUNA); regla.waitForIdle()
        assertEquals(ModoAnim.NINGUNA, modo); assertEquals(true, reducir)
        Ajustes.get(Apoyo.ctx()).fijarAnimaciones(ModoAnim.NATURAL); regla.waitForIdle()
        assertEquals(ModoAnim.NATURAL, modo); assertEquals(false, reducir)
    }

    // ---------- transiciones reales (NavHost con las mismas funciones que usa el Shell) ----------

    private lateinit var nav: NavHostController

    private fun arnes(modo: ModoAnim) {
        Ajustes.get(Apoyo.ctx()).fijarAnimaciones(modo)
        regla.mainClock.autoAdvance = false
        regla.setContent {
            BotChatTema {
                nav = rememberNavController()
                val m = LocalModoAnim.current
                NavHost(
                    nav, "a",
                    enterTransition = { Transiciones.entra(m, TipoTransicion.EJE, 90) }, exitTransition = { Transiciones.sale(m, TipoTransicion.EJE, 90) },
                    popEnterTransition = { Transiciones.vuelveEntra(m, TipoTransicion.EJE, 90) }, popExitTransition = { Transiciones.vuelveSale(m, TipoTransicion.EJE, 90) },
                ) {
                    composable("a") { Box(Modifier.fillMaxSize().testTag("pant_a")) { Text("A") } }
                    composable("b") { Box(Modifier.fillMaxSize().testTag("pant_b")) { Text("B") } }
                }
            }
        }
        regla.mainClock.advanceTimeBy(500) // deja asentada la pantalla inicial
    }

    private fun izq(tag: String): Float? = regla.onAllNodesWithTagSafe(tag)?.left
    private fun anchoRaiz() = regla.onRoot().fetchSemanticsNode().boundsInRoot.width

    private fun androidx.compose.ui.test.junit4.ComposeContentTestRule.onAllNodesWithTagSafe(tag: String): androidx.compose.ui.geometry.Rect? =
        onAllNodesWithTag(tag).fetchSemanticsNodes().firstOrNull()?.boundsInRoot

    private fun voyA(ruta: String) { regla.runOnUiThread { nav.navigate(ruta) } }
    private fun atras() { regla.runOnUiThread { nav.popBackStack() } }

    @Test fun naturalLaPantallaNuevaEntraPorUnEjeCortoYConFundidoEnDosTiempos() {
        arnes(ModoAnim.NATURAL)
        assertEquals(280, Transiciones.DESLIZAR_MS)
        voyA("b")
        regla.mainClock.advanceTimeBy(32)
        val l0 = izq("pant_b")
        assertTrue("entra desplazada a la derecha, a lo sumo los 90 px del eje: izq=$l0", l0 != null && l0 > 0f && l0 <= 90.5f)
        assertTrue("la de abajo sigue presente un momento", izq("pant_a") != null)
        regla.mainClock.advanceTimeBy(500)
        assertEquals("termina en su lugar", 0f, izq("pant_b")!!, 0.5f)
        assertTrue("la anterior se retira al terminar", izq("pant_a") == null)
    }

    @Test fun naturalAlVolverLaPantallaSaleHaciaLaDerechaPorElMismoEje() {
        arnes(ModoAnim.NATURAL)
        voyA("b"); regla.mainClock.advanceTimeBy(600)
        atras()
        regla.mainClock.advanceTimeBy(100)
        val l = izq("pant_b")
        assertTrue("al volver, la pantalla que sale se va hacia la derecha: $l", l != null && l > 0f)
        assertTrue("y la de abajo reaparece", izq("pant_a") != null)
        regla.mainClock.advanceTimeBy(600)
        assertTrue(izq("pant_b") == null)
        assertEquals(0f, izq("pant_a")!!, 0.5f)
    }

    @Test fun reducidasSoloFundeSinMoverNada() {
        arnes(ModoAnim.REDUCIDAS)
        voyA("b")
        for (i in 0 until 8) {
            regla.mainClock.advanceTimeBy(16)
            izq("pant_b")?.let { assertEquals("nunca se desplaza", 0f, it, 0.5f) }
        }
        regla.mainClock.advanceTimeBy(300)
        assertEquals(0f, izq("pant_b")!!, 0.5f)
        assertTrue(izq("pant_a") == null)
        assertEquals(100, Transiciones.FUNDIDO_CORTO_MS)
    }

    @Test fun ningunaCambiaAlInstanteSinPantallaIntermedia() {
        arnes(ModoAnim.NINGUNA)
        voyA("b")
        regla.mainClock.advanceTimeBy(64)
        assertEquals(0f, izq("pant_b")!!, 0.5f)
        assertTrue("sin transición no se queda la anterior", izq("pant_a") == null)
        atras(); regla.mainClock.advanceTimeBy(64)
        assertTrue(izq("pant_b") == null); assertEquals(0f, izq("pant_a")!!, 0.5f)
    }

    @Test fun elFundidoDelChatEsDe100msYInstantaneoConNinguna() {
        assertEquals(100, Transiciones.FUNDIDO_CHAT_MS)
        assertTrue(Transiciones.fundidoChat(ModoAnim.NATURAL) is androidx.compose.animation.core.TweenSpec)
        assertTrue(Transiciones.fundidoChat(ModoAnim.REDUCIDAS) is androidx.compose.animation.core.TweenSpec)
        assertTrue(Transiciones.fundidoChat(ModoAnim.NINGUNA) is androidx.compose.animation.core.SnapSpec)
    }
}
