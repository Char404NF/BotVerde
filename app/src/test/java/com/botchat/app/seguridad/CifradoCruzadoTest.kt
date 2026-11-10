package com.botchat.app.seguridad

import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test
import java.io.File

/** Prueba cruzada con los archivos de referencia que genera Go (pruebas_cruzadas/). Ver su LEEME.md. */
class CifradoCruzadoTest {
    private val clave = ByteArray(32) { it.toByte() }
    private val nonce = ByteArray(12) { (0xA0 + it).toByte() }
    private val tamanos = listOf(0, 1, 65535, 65536, 65537, 131072, 200000)

    private fun patron(n: Int) = ByteArray(n) { (it * 31 + 7).toByte() }
    private fun referencia(n: Int) = File("../pruebas_cruzadas/n$n.bcf").readBytes()

    @Test fun kotlinDescifraLoQueCifraGo() {
        for (n in tamanos) {
            val plano = FormatoCifrado.descifrarBytes(clave, referencia(n))
            assertArrayEquals("n=$n", patron(n), plano)
        }
    }

    @Test fun kotlinCifraIgualQueGo() {
        // Mismos bytes que Go con la misma llave y nonce: por lo tanto Go también descifra lo de Kotlin.
        for (n in tamanos) {
            val cif = FormatoCifrado.cifrarBytes(clave, patron(n), nonce)
            assertArrayEquals("n=$n", referencia(n), cif)
        }
    }

    @Test fun accesoAleatorio() {
        val ref = referencia(200000)
        val plano = patron(200000)
        ArchivoCifrado(FuenteBytes(ref), clave).use { a ->
            assertEquals(200000L, a.tam)
            for ((off, len) in listOf(0 to 10, 65530 to 20, 65536 to 65536, 131071 to 2, 199990 to 10, 100000 to 90000)) {
                val buf = ByteArray(len)
                val n = a.leer(off.toLong(), buf, 0, len)
                assertEquals(len, n)
                assertArrayEquals(plano.copyOfRange(off, off + len), buf)
            }
            assertEquals(-1, a.leer(200000, ByteArray(4), 0, 4))
            assertEquals(5, a.leer(199995, ByteArray(10), 0, 10))
        }
    }

    @Test fun deteccionDeManipulacion() {
        val ref = referencia(200000)
        val alterado = ref.copyOf().also { it[FormatoCifrado.TAM_CABECERA + 70000] = (it[FormatoCifrado.TAM_CABECERA + 70000].toInt() xor 1).toByte() }
        assertThrows(ArchivoDanadoException::class.java) { FormatoCifrado.descifrarBytes(clave, alterado) }
        // cortado justo en un límite de bloque
        val cortado = ref.copyOf(FormatoCifrado.TAM_CABECERA + 2 * FormatoCifrado.TAM_BLOQUE_CIFRADO)
        assertThrows(ArchivoDanadoException::class.java) { FormatoCifrado.descifrarBytes(clave, cortado) }
        // llave equivocada
        assertThrows(ArchivoDanadoException::class.java) { FormatoCifrado.descifrarBytes(ByteArray(32), ref) }
        // el archivo vacío también se autentica
        val vacio = referencia(0).copyOf().also { it[FormatoCifrado.TAM_CABECERA] = (it[FormatoCifrado.TAM_CABECERA].toInt() xor 1).toByte() }
        assertThrows(ArchivoDanadoException::class.java) { FormatoCifrado.descifrarBytes(clave, vacio) }
    }

    @Test fun escritorEnTrozosIrregulares() {
        val plano = patron(300001)
        val salida = java.io.ByteArrayOutputStream()
        EscritorCifrado(salida, clave, nonce).use { w ->
            var o = 0
            while (o < plano.size) { val c = minOf(7919, plano.size - o); w.write(plano, o, c); o += c }
        }
        assertArrayEquals(FormatoCifrado.cifrarBytes(clave, plano, nonce), salida.toByteArray())
        assertArrayEquals(plano, FormatoCifrado.descifrarBytes(clave, salida.toByteArray()))
    }

    @Test fun nonceConAcarreo() {
        val todoFF = ByteArray(12) { 0xff.toByte() }
        assertArrayEquals(ByteArray(12), FormatoCifrado.nonceBloque(todoFF, 1))
        val b = ByteArray(12).also { it[11] = 0xff.toByte() }
        val esperado = ByteArray(12).also { it[10] = 0x02; it[11] = 0x01 }
        assertArrayEquals(esperado, FormatoCifrado.nonceBloque(b, 0x0102))
    }
}
