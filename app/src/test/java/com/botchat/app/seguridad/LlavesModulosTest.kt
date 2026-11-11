package com.botchat.app.seguridad

import androidx.test.core.app.ApplicationProvider
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
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
@Config(sdk = [34], application = android.app.Application::class)
class LlavesModulosTest {
    private val ctx get() = ApplicationProvider.getApplicationContext<android.content.Context>()

    /** Un cifrador de software con el mismo formato que el del Keystore (el Keystore real no existe en Robolectric). */
    private class DePrueba : LlavesModulos.Cifrador {
        private val k = SecretKeySpec(ByteArray(32) { (it + 3).toByte() }, "AES")
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

    private lateinit var original: LlavesModulos.Cifrador

    @Before fun antes() {
        original = LlavesModulos.cifrador
        LlavesModulos.cifrador = DePrueba()
        File(ctx.filesDir, "llaves_mod").deleteRecursively()
    }

    @After fun despues() { LlavesModulos.cifrador = original; File(ctx.filesDir, "llaves_mod").deleteRecursively() }

    @Test fun guardaCifradaYLaDevuelveSinDejarElTextoEnClaro() {
        LlavesModulos.guardar(ctx, 1, "ia", "llave", "  sk-SECRETA-123 ")
        assertTrue(LlavesModulos.existe(ctx, 1, "ia", "llave"))
        assertEquals("sk-SECRETA-123", LlavesModulos.obtener(ctx, 1, "ia", "llave"))
        val bytes = File(ctx.filesDir, "llaves_mod").listFiles()!!.single { it.name.endsWith(".bin") }.readBytes()
        assertFalse("la llave no puede estar en claro en el disco", String(bytes, Charsets.ISO_8859_1).contains("SECRETA"))
        // no queda en ningún otro archivo de la app (preferencias, base…)
        ctx.filesDir.walkTopDown().filter { it.isFile }.forEach { assertFalse(it.path, String(it.readBytes(), Charsets.ISO_8859_1).contains("SECRETA")) }
    }

    @Test fun vacioLaBorraYCadaBotTieneLasSuyas() {
        LlavesModulos.guardar(ctx, 1, "ia", "llave", "uno")
        LlavesModulos.guardar(ctx, 2, "ia", "llave", "dos")
        assertEquals("dos", LlavesModulos.obtener(ctx, 2, "ia", "llave"))
        LlavesModulos.guardar(ctx, 1, "ia", "llave", "")
        assertNull(LlavesModulos.obtener(ctx, 1, "ia", "llave"))
        assertFalse(LlavesModulos.existe(ctx, 1, "ia", "llave"))
        LlavesModulos.borrarDeBot(ctx, 2)
        assertFalse(LlavesModulos.existe(ctx, 2, "ia", "llave"))
    }

    @Test fun cargarEnMotorEntregaTodasYSaltaLasQueNoSePuedenAbrir() {
        LlavesModulos.guardar(ctx, 1, "ia", "llave", "uno")
        LlavesModulos.guardar(ctx, 1, "traductor", "llave", "dos")
        File(ctx.filesDir, "llaves_mod/1.ia.rota.bin").writeBytes(ByteArray(40) { 7 }) // no descifra: se ignora
        val vistas = mutableMapOf<String, String>()
        LlavesModulos.cargarEnMotor(ctx) { b, m, c, v -> vistas["$b.$m.$c"] = v }
        assertEquals(mapOf("1.ia.llave" to "uno", "1.traductor.llave" to "dos"), vistas)
    }

    @Test fun rechazaNombresRaros() {
        try { LlavesModulos.guardar(ctx, 1, "../ia", "llave", "x"); throw AssertionError("debía fallar") } catch (e: IllegalArgumentException) { /* ok */ }
        try { LlavesModulos.guardar(ctx, 1, "ia", "a.b", "x"); throw AssertionError("debía fallar") } catch (e: IllegalArgumentException) { /* ok */ }
    }
}
