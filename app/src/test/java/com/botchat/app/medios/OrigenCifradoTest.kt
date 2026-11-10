package com.botchat.app.medios

import com.botchat.app.seguridad.ArchivoCifrado
import com.botchat.app.seguridad.FormatoCifrado
import com.botchat.app.seguridad.FuenteBytes
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class)
class OrigenCifradoTest {
    private val clave = ByteArray(32) { (it + 7).toByte() }
    // Varios bloques de 64 KB para probar lecturas que cruzan de un bloque a otro.
    private val plano = ByteArray(200_000) { (it * 31 + it / 7).toByte() }

    private fun origen() = OrigenCifrado(ArchivoCifrado(FuenteBytes(FormatoCifrado.cifrarBytes(clave, plano)), clave))

    @Test fun tamanoYLecturasAleatorias() {
        val o = origen()
        assertEquals(plano.size.toLong(), o.size)
        val buf = ByteArray(5_000)
        for (pos in listOf(0L, 65_530L, 131_070L, 150_000L)) {
            val n = o.readAt(pos, buf, 0, buf.size)
            assertEquals(buf.size, n)
            assertArrayEquals("posición $pos", plano.copyOfRange(pos.toInt(), pos.toInt() + n), buf.copyOf(n))
        }
    }

    @Test fun alFinalDevuelveLoQueQuedaYLuegoMenosUno() {
        val o = origen()
        val buf = ByteArray(1_000)
        assertEquals(500, o.readAt(plano.size - 500L, buf, 0, 1_000))
        assertEquals(-1, o.readAt(plano.size.toLong(), buf, 0, 1_000))
        assertEquals(-1, o.readAt(plano.size + 10L, buf, 0, 1_000))
        assertEquals(0, o.readAt(0, buf, 0, 0))
    }

    @Test fun archivoVacioONoVideoNoRompe() {
        val vacio = OrigenCifrado(ArchivoCifrado(FuenteBytes(FormatoCifrado.cifrarBytes(clave, ByteArray(0))), clave))
        assertEquals(0L, vacio.size)
        assertEquals(-1, vacio.readAt(0, ByteArray(10), 0, 10))
        // Sacar un cuadro de algo que no es un video devuelve nulo en vez de fallar.
        val f = java.io.File.createTempFile("noesvideo", ".enc").apply { writeBytes(FormatoCifrado.cifrarBytes(clave, "texto".toByteArray())); deleteOnExit() }
        assertEquals(null, PortadaVideo.sacarCuadro(f, clave, 300, 0))
        assertEquals(null, PortadaVideo.sacarCuadro(java.io.File("/no/existe.enc"), clave, 300, 0))
    }
}
