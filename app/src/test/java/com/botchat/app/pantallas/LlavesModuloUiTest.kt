package com.botchat.app.pantallas

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onAllNodesWithTag
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performTextInput
import com.botchat.app.seguridad.LlavesModulos
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaAvisoRiesgo
import com.botchat.app.ui.PantallaConfigModulo
import org.json.JSONArray
import org.json.JSONObject
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import java.io.File
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class LlavesModuloUiTest {
    @get:Rule val regla = createComposeRule()
    private lateinit var motor: MotorFalso
    private var bot = 0L
    private lateinit var original: LlavesModulos.Cifrador

    private class DePrueba : LlavesModulos.Cifrador {
        private val k = SecretKeySpec(ByteArray(32) { (it + 5).toByte() }, "AES")
        override fun cifrar(claro: ByteArray): ByteArray {
            val iv = ByteArray(12).also { SecureRandom().nextBytes(it) }
            val c = Cipher.getInstance("AES/GCM/NoPadding"); c.init(Cipher.ENCRYPT_MODE, k, GCMParameterSpec(128, iv))
            return byteArrayOf(1) + iv + c.doFinal(claro)
        }
        override fun descifrar(cifrado: ByteArray): ByteArray {
            val c = Cipher.getInstance("AES/GCM/NoPadding"); c.init(Cipher.DECRYPT_MODE, k, GCMParameterSpec(128, cifrado, 1, 12))
            return c.doFinal(cifrado, 13, cifrado.size - 13)
        }
    }

    @Before fun antes() {
        original = LlavesModulos.cifrador
        LlavesModulos.cifrador = DePrueba()
        motor = Apoyo.reinicia(); bot = Apoyo.creaBot()
        val esquema = JSONArray()
            .put(JSONObject().put("clave", "modelo").put("titulo", "Modelo").put("tipo", "texto").put("defecto", "gpt-4o-mini"))
            .put(JSONObject().put("clave", "llave").put("titulo", "Llave del servicio").put("ayuda", "Se guarda cifrada en el Keystore.").put("tipo", "secreto"))
        motor.modulosLista.add(Apoyo.moduloJson("ia", "IA opcional", "Responde con IA. Aviso de privacidad: el contenido SALE del teléfono.", "ia", "medio", iniciativa = true, esquema = esquema))
    }

    @After fun despues() { LlavesModulos.cifrador = original; File(Apoyo.ctx().filesDir, "llaves_mod").deleteRecursively() }

    private fun hay(tag: String) = regla.onAllNodesWithTag(tag, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()

    @Test fun laLlaveVaAlKeystoreYAGoPeroNuncaALaConfiguracion() {
        regla.setContent { BotChatTema { PantallaConfigModulo(bot, "ia", alVolver = {}) } }
        regla.esperaEtiqueta("config_generica")
        assertTrue(hay("llave_estado_llave"))
        regla.onNodeWithTag("in_llave", useUnmergedTree = true).performScrollTo().performTextInput("sk-SECRETA-999")
        regla.onNodeWithTag("guardar_llave_llave", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { motor.llavesEnGo["ia:llave"] == "sk-SECRETA-999" }
        assertEquals("sk-SECRETA-999", LlavesModulos.obtener(Apoyo.ctx(), bot, "ia", "llave"))
        regla.espera { hay("borrar_llave_llave") }
        // guardar la configuración NO manda la llave
        regla.onNodeWithTag("config_guardar", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { motor.llamadas.any { it.startsWith("guardarConfigModulo:ia:") } }
        val cfg = motor.llamadas.first { it.startsWith("guardarConfigModulo:ia:") }
        assertFalse(cfg, cfg.contains("llave") || cfg.contains("SECRETA"))
        assertTrue(cfg.contains("\"modelo\""))
        // borrar la llave la quita de los dos lados
        regla.onNodeWithTag("borrar_llave_llave", useUnmergedTree = true).performScrollTo().performClick()
        regla.espera { motor.llavesEnGo["ia:llave"] == "" }
        assertFalse(LlavesModulos.existe(Apoyo.ctx(), bot, "ia", "llave"))
    }

    @Test fun elAvisoDeRiesgoDeLaIAAdviertePrivacidad() {
        regla.setContent { BotChatTema { PantallaAvisoRiesgo(bot, "ia", {}) } }
        regla.esperaEtiqueta("aviso_texto")
        regla.onNodeWithTag("aviso_privacidad", useUnmergedTree = true).performScrollTo().assertIsDisplayed()
    }
}
