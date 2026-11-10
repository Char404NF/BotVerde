package com.botchat.app.seguridad

import java.io.Closeable
import java.io.File
import java.io.IOException
import java.io.OutputStream
import java.io.RandomAccessFile
import java.security.GeneralSecurityException
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec

/**
 * Formato de archivo cifrado de BotChat (versión 1). Es el MISMO que escribe el módulo Go
 * (`go/internal/cifrado/archivo.go`, donde está la descripción completa):
 *
 *  - Cabecera de 20 bytes: "BCF1", versión 1, log2 del bloque (16), 2 reservados, nonce base de 12 bytes.
 *  - Bloques de 64 KB de texto, cada uno AES-256-GCM (cifrado + etiqueta de 16 bytes). El último puede ser más corto
 *    (o vacío: un archivo de 0 bytes lleva un bloque con solo la etiqueta).
 *  - Nonce del bloque i = nonce base + i (96 bits, big-endian, con acarreo).
 *  - AAD = cabecera || i (8 bytes big-endian) || último (0 o 1).
 */
object FormatoCifrado {
    const val TAM_BLOQUE = 64 * 1024
    const val TAM_CABECERA = 20
    const val TAM_ETIQUETA = 16
    const val TAM_CLAVE = 32
    internal const val TAM_BLOQUE_CIFRADO = TAM_BLOQUE + TAM_ETIQUETA
    private val MAGIA = byteArrayOf('B'.code.toByte(), 'C'.code.toByte(), 'F'.code.toByte(), '1'.code.toByte())

    fun cabecera(nonce: ByteArray): ByteArray {
        require(nonce.size == 12)
        val c = ByteArray(TAM_CABECERA)
        MAGIA.copyInto(c, 0)
        c[4] = 1
        c[5] = 16
        nonce.copyInto(c, 8)
        return c
    }

    fun nonceBloque(base: ByteArray, i: Long): ByteArray {
        val n = base.copyOf()
        var acarreo = i
        var p = 11
        while (p >= 0 && acarreo != 0L) {
            val s = (n[p].toLong() and 0xff) + (acarreo and 0xff)
            n[p] = s.toByte()
            acarreo = (acarreo ushr 8) + (s ushr 8)
            p--
        }
        return n
    }

    fun aad(cabecera: ByteArray, i: Long, ultimo: Boolean): ByteArray {
        val a = ByteArray(TAM_CABECERA + 9)
        cabecera.copyInto(a, 0)
        for (k in 0 until 8) a[TAM_CABECERA + k] = (i ushr (56 - 8 * k)).toByte()
        a[TAM_CABECERA + 8] = if (ultimo) 1 else 0
        return a
    }

    fun cifraBloque(clave: ByteArray, cab: ByteArray, nonceBase: ByteArray, i: Long, ultimo: Boolean, plano: ByteArray, largo: Int): ByteArray {
        val c = Cipher.getInstance("AES/GCM/NoPadding")
        c.init(Cipher.ENCRYPT_MODE, SecretKeySpec(clave, "AES"), GCMParameterSpec(128, nonceBloque(nonceBase, i)))
        c.updateAAD(aad(cab, i, ultimo))
        return c.doFinal(plano, 0, largo)
    }

    fun descifraBloque(clave: ByteArray, cab: ByteArray, nonceBase: ByteArray, i: Long, ultimo: Boolean, cifrado: ByteArray): ByteArray {
        val c = Cipher.getInstance("AES/GCM/NoPadding")
        c.init(Cipher.DECRYPT_MODE, SecretKeySpec(clave, "AES"), GCMParameterSpec(128, nonceBloque(nonceBase, i)))
        c.updateAAD(aad(cab, i, ultimo))
        return try {
            c.doFinal(cifrado)
        } catch (e: GeneralSecurityException) {
            throw ArchivoDanadoException("El archivo cifrado está dañado o la llave no corresponde", e)
        }
    }

    fun cifrarBytes(clave: ByteArray, plano: ByteArray, nonce: ByteArray? = null): ByteArray {
        val salida = java.io.ByteArrayOutputStream()
        EscritorCifrado(salida, clave, nonce).use { it.write(plano) }
        return salida.toByteArray()
    }

    fun descifrarBytes(clave: ByteArray, cifrado: ByteArray): ByteArray =
        ArchivoCifrado(FuenteBytes(cifrado), clave).use { it.leerTodo() }
}

class ArchivoDanadoException(mensaje: String, causa: Throwable? = null) : IOException(mensaje, causa)

/** Cifra en flujo. Hay que cerrarlo para escribir el último bloque. */
class EscritorCifrado(
    private val salida: OutputStream,
    private val clave: ByteArray,
    nonce: ByteArray? = null,
) : OutputStream() {
    private val base: ByteArray = nonce ?: ByteArray(12).also { SecureRandom().nextBytes(it) }
    private val cab = FormatoCifrado.cabecera(base)
    private val buf = ByteArray(FormatoCifrado.TAM_BLOQUE)
    private var n = 0
    private var idx = 0L
    private var cabEscrita = false
    private var cerrado = false

    init { require(clave.size == FormatoCifrado.TAM_CLAVE) { "La llave de datos debe medir 32 bytes" } }

    private fun escribeCabecera() {
        if (!cabEscrita) { salida.write(cab); cabEscrita = true }
    }

    private fun vacia(ultimo: Boolean) {
        escribeCabecera()
        salida.write(FormatoCifrado.cifraBloque(clave, cab, base, idx, ultimo, buf, n))
        idx++
        n = 0
    }

    override fun write(b: Int) { write(byteArrayOf(b.toByte()), 0, 1) }

    override fun write(b: ByteArray, off: Int, len: Int) {
        check(!cerrado) { "escritor cerrado" }
        var o = off
        var restante = len
        while (restante > 0) {
            if (n == FormatoCifrado.TAM_BLOQUE) vacia(false)
            val c = minOf(restante, FormatoCifrado.TAM_BLOQUE - n)
            System.arraycopy(b, o, buf, n, c)
            n += c; o += c; restante -= c
        }
    }

    override fun close() {
        if (cerrado) return
        cerrado = true
        vacia(true)
        salida.flush()
        salida.close()
    }
}

/** Fuente de bytes con acceso aleatorio (un archivo, o memoria en las pruebas). */
interface FuenteAleatoria : Closeable {
    val tam: Long
    /** Lee hasta `len` bytes desde `pos`; devuelve cuántos leyó, o -1 si `pos` está al final. */
    fun leer(pos: Long, dst: ByteArray, off: Int, len: Int): Int
}

class FuenteArchivo(archivo: File) : FuenteAleatoria {
    private val raf = RandomAccessFile(archivo, "r")
    override val tam: Long = raf.length()
    override fun leer(pos: Long, dst: ByteArray, off: Int, len: Int): Int {
        if (pos >= tam) return -1
        raf.seek(pos)
        return raf.read(dst, off, len)
    }
    override fun close() = raf.close()
}

class FuenteBytes(private val b: ByteArray) : FuenteAleatoria {
    override val tam: Long = b.size.toLong()
    override fun leer(pos: Long, dst: ByteArray, off: Int, len: Int): Int {
        if (pos >= b.size) return -1
        val c = minOf(len.toLong(), b.size - pos).toInt()
        System.arraycopy(b, pos.toInt(), dst, off, c)
        return c
    }
    override fun close() {}
}

/** Lectura con acceso aleatorio de un archivo cifrado. No es seguro entre hilos: usar una instancia por lector. */
class ArchivoCifrado(private val fuente: FuenteAleatoria, private val clave: ByteArray) : Closeable {
    private val cab = ByteArray(FormatoCifrado.TAM_CABECERA)
    private val base = ByteArray(12)
    private val bloques: Long
    /** Tamaño del contenido en claro. */
    val tam: Long
    private val cache = object : LinkedHashMap<Long, ByteArray>(8, 0.75f, true) {
        override fun removeEldestEntry(eldest: MutableMap.MutableEntry<Long, ByteArray>?) = size > 4
    }

    init {
        require(clave.size == FormatoCifrado.TAM_CLAVE) { "La llave de datos debe medir 32 bytes" }
        val total = fuente.tam
        if (total < FormatoCifrado.TAM_CABECERA + FormatoCifrado.TAM_ETIQUETA) throw ArchivoDanadoException("El archivo cifrado es demasiado corto")
        leerExacto(0, cab, 0, cab.size)
        val magiaOk = cab[0] == 'B'.code.toByte() && cab[1] == 'C'.code.toByte() && cab[2] == 'F'.code.toByte() && cab[3] == '1'.code.toByte()
        if (!magiaOk || cab[4].toInt() != 1 || cab[5].toInt() != 16) throw ArchivoDanadoException("El archivo cifrado no tiene un formato válido")
        cab.copyInto(base, 0, 8, 20)
        val cuerpo = total - FormatoCifrado.TAM_CABECERA
        bloques = (cuerpo + FormatoCifrado.TAM_BLOQUE_CIFRADO - 1) / FormatoCifrado.TAM_BLOQUE_CIFRADO
        val ultimo = cuerpo - (bloques - 1) * FormatoCifrado.TAM_BLOQUE_CIFRADO
        if (ultimo < FormatoCifrado.TAM_ETIQUETA) throw ArchivoDanadoException("El archivo cifrado no tiene un formato válido")
        tam = cuerpo - bloques * FormatoCifrado.TAM_ETIQUETA
    }

    private fun leerExacto(pos: Long, dst: ByteArray, off: Int, len: Int) {
        var hecho = 0
        while (hecho < len) {
            val n = fuente.leer(pos + hecho, dst, off + hecho, len - hecho)
            if (n <= 0) throw ArchivoDanadoException("El archivo cifrado está incompleto")
            hecho += n
        }
    }

    private fun bloque(i: Long): ByteArray {
        cache[i]?.let { return it }
        val ultimo = i == bloques - 1
        val largo = if (ultimo) (tam - (bloques - 1) * FormatoCifrado.TAM_BLOQUE + FormatoCifrado.TAM_ETIQUETA).toInt() else FormatoCifrado.TAM_BLOQUE_CIFRADO
        val ct = ByteArray(largo)
        leerExacto(FormatoCifrado.TAM_CABECERA + i * FormatoCifrado.TAM_BLOQUE_CIFRADO, ct, 0, largo)
        val pt = FormatoCifrado.descifraBloque(clave, cab, base, i, ultimo, ct)
        cache[i] = pt
        return pt
    }

    /** Lee hasta `len` bytes del contenido en claro desde `pos`. Devuelve -1 si `pos` está al final o más allá. */
    fun leer(pos: Long, dst: ByteArray, off: Int, len: Int): Int {
        if (tam == 0L) { bloque(0); return -1 } // autentica el bloque vacío
        if (pos >= tam) return -1
        var hecho = 0
        while (hecho < len && pos + hecho < tam) {
            val p = pos + hecho
            val b = bloque(p / FormatoCifrado.TAM_BLOQUE)
            val dentro = (p % FormatoCifrado.TAM_BLOQUE).toInt()
            val c = minOf(len - hecho, b.size - dentro)
            System.arraycopy(b, dentro, dst, off + hecho, c)
            hecho += c
        }
        return hecho
    }

    fun leerTodo(): ByteArray {
        val out = ByteArray(tam.toInt())
        if (tam == 0L) { bloque(0); return out }
        var hecho = 0
        while (hecho < out.size) {
            val n = leer(hecho.toLong(), out, hecho, out.size - hecho)
            if (n <= 0) break
            hecho += n
        }
        return out
    }

    override fun close() { cache.clear(); fuente.close() }
}
