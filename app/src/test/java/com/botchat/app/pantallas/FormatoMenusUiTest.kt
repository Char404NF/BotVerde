package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsOn
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextInput
import com.botchat.app.datos.AlcanceDef
import com.botchat.app.datos.AutoDef
import com.botchat.app.datos.BotonDef
import com.botchat.app.datos.DispDef
import com.botchat.app.datos.OpcionDef
import com.botchat.app.datos.PasoDef
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.EditorAutomatizacion
import com.botchat.app.ui.TarjetaFormatoMenus
import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class FormatoMenusUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L

    @Before fun antes() { motor = Apoyo.reinicia(); bot = Apoyo.creaBot() }
    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun nodo(tag: String) = regla.onNodeWithTag(tag, useUnmergedTree = true)
    private fun toca(tag: String) { regla.esperaEtiqueta(tag); runCatching { nodo(tag).performScrollTo() }; nodo(tag).performClick() }

    private fun tienda(formato: String = "", boton: BotonDef? = null) = AutoDef(
        "Tienda", "m",
        listOf(
            PasoDef("m", "menu", "¡Hola! ¿Qué buscas?", listOf(OpcionDef("Catálogo", "p", id = "o1", boton = boton), OpcionDef("Contacto", "p", id = "o2")), formato = formato),
            PasoDef("p", "respuesta", texto = "Aquí tienes."),
        ),
        DispDef(comando = ".menu"), AlcanceDef(privados = true),
    )

    private fun editor(d: AutoDef) {
        motor.autoDetalle = JSONObject().put("id", 1).put("activo", true).put("sistema", false).put("version", 1).put("def", JSONObject(d.json())).put("avisos", JSONArray()).toString()
        regla.setContent { BotChatTema { EditorAutomatizacion(bot, 1, {}, {}) } }
    }

    private fun guardado(): JSONObject {
        regla.esperaEtiqueta("guardar_auto")
        val antes = motor.llamadas.count { it.startsWith("guardarAuto:1:") }
        nodo("guardar_auto").performClick()
        regla.espera { motor.llamadas.count { it.startsWith("guardarAuto:1:") } > antes }
        return JSONObject(motor.llamadas.last { it.startsWith("guardarAuto:1:") }.removePrefix("guardarAuto:1:"))
    }

    // ---------- el modelo ----------

    @Test fun elFormatoYElBotonEspecialSobrevivenAlJson() {
        val d = tienda("botones", BotonDef("enlace", "https://ejemplo.com/catalogo"))
        val p = AutoDef.desde(d.json()).pasos[0]
        assertEquals("botones", p.formato)
        assertEquals(BotonDef("enlace", "https://ejemplo.com/catalogo"), p.opciones[0].boton)
        assertNull(p.opciones[1].boton)
        // sin ellos el JSON no lleva nada nuevo (no rompe automatizaciones viejas)
        val viejo = JSONObject(tienda().json()).getJSONArray("pasos").getJSONObject(0)
        assertFalse(viejo.has("formato")); assertFalse(viejo.getJSONArray("opciones").getJSONObject(0).has("boton"))
    }

    // ---------- el editor ----------

    @Test fun elMenuEligeSuFormatoYSeGuarda() {
        editor(tienda())
        toca("abrir_m")
        regla.esperaEtiqueta("formato_m_botones")
        assertTrue(hay("formato_ayuda_m"))
        toca("formato_m_botones")
        regla.esperaTexto("Hasta 3 opciones salen como botones", substring = true)
        assertEquals("botones", guardado().getJSONArray("pasos").getJSONObject(0).getString("formato"))
        toca("formato_m_general")
        val g = guardado().getJSONArray("pasos").getJSONObject(0)
        assertFalse(g.has("formato"))
    }

    @Test fun unaOpcionPuedeSerBotonDeEnlaceCopiarOLlamar() {
        editor(tienda())
        toca("abrir_m")
        toca("boton_especial_m_0")
        toca("boton_m_0_enlace")
        regla.esperaEtiqueta("boton_valor_m_0")
        nodo("boton_valor_m_0").performTextInput("https://ejemplo.com/catalogo")
        regla.esperaTexto("abre la página", substring = true)
        var b = guardado().getJSONArray("pasos").getJSONObject(0).getJSONArray("opciones").getJSONObject(0).getJSONObject("boton")
        assertEquals("enlace", b.getString("tipo")); assertEquals("https://ejemplo.com/catalogo", b.getString("valor"))
        toca("boton_m_0_llamar")
        regla.esperaTexto("marca el número", substring = true)
        b = guardado().getJSONArray("pasos").getJSONObject(0).getJSONArray("opciones").getJSONObject(0).getJSONObject("boton")
        assertEquals("llamar", b.getString("tipo"))
        toca("boton_m_0_normal")
        assertFalse(guardado().getJSONArray("pasos").getJSONObject(0).getJSONArray("opciones").getJSONObject(0).has("boton"))
    }

    // ---------- ajustes generales ----------

    @Test fun elFormatoGeneralSeEligeEnAjustesYMuestraLoQueNecesitaCadaUno() {
        regla.setContent { BotChatTema { TarjetaFormatoMenus(bot) } }
        regla.esperaEtiqueta("formato_general_botones")
        assertFalse(hay("formato_botones_hora")); assertFalse(hay("formato_enc_max"))
        toca("formato_general_botones")
        regla.espera { llamo("ajusteLab:lab_modo_menu:botones") }
        regla.esperaEtiqueta("formato_botones_hora")
        regla.esperaTexto("Solo en chats privados", substring = true)
        toca("formato_tipo_lista")
        regla.espera { llamo("ajusteLab:lab_botones_tipo:lista") }
        toca("formato_general_encuesta")
        regla.espera { llamo("ajusteLab:lab_modo_menu:encuesta") }
        regla.esperaEtiqueta("formato_enc_max")
        assertFalse(hay("formato_botones_hora"))
        toca("formato_enc_grupos")
        regla.espera { llamo("ajusteLab:lab_grupos:1") }
        toca("formato_general_texto")
        regla.espera { llamo("ajusteLab:lab_modo_menu:texto") }
        regla.espera { !hay("formato_enc_max") }
    }

    @Test fun noPideAceptarNingunAvisoNiEncenderNadaParaElegirBotones() {
        regla.setContent { BotChatTema { TarjetaFormatoMenus(bot) } }
        toca("formato_general_botones")
        regla.espera { llamo("ajusteLab:lab_modo_menu:botones") }
        assertFalse(llamo("aceptarAvisoLab")); assertFalse(llamo("ajusteLab:lab_activo"))
        assertFalse(hay("aviso_lab_titulo"))
    }
}
