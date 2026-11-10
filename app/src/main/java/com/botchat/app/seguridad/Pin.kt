package com.botchat.app.seguridad

import java.security.MessageDigest
import java.security.SecureRandom
import java.util.Base64
import javax.crypto.SecretKeyFactory
import javax.crypto.spec.PBEKeySpec

/** PIN de 4 a 6 dígitos guardado con sal y PBKDF2-HMAC-SHA256. */
object Pin {
    const val ITERACIONES = 120_000
    private const val LARGO_HASH_BITS = 256

    fun valido(pin: String) = pin.length in 4..6 && pin.all { it in '0'..'9' }

    private fun hash(pin: String, sal: ByteArray, iter: Int): ByteArray =
        SecretKeyFactory.getInstance("PBKDF2WithHmacSHA256").generateSecret(PBEKeySpec(pin.toCharArray(), sal, iter, LARGO_HASH_BITS)).encoded

    /** Formato guardado: `v1$iteraciones$sal$hash` (los dos últimos en base64). */
    fun crear(pin: String, sal: ByteArray = ByteArray(16).also { SecureRandom().nextBytes(it) }, iter: Int = ITERACIONES): String {
        require(valido(pin)) { "El PIN debe tener de 4 a 6 dígitos" }
        val b64 = Base64.getEncoder()
        return "v1$$iter$${b64.encodeToString(sal)}$${b64.encodeToString(hash(pin, sal, iter))}"
    }

    fun verificar(pin: String, guardado: String): Boolean {
        if (!valido(pin)) return false
        val p = guardado.split('$')
        if (p.size != 4 || p[0] != "v1") return false
        return try {
            val d = Base64.getDecoder()
            MessageDigest.isEqual(hash(pin, d.decode(p[2]), p[1].toInt()), d.decode(p[3]))
        } catch (e: Exception) {
            false
        }
    }
}

/** Resultado de probar un PIN. */
sealed class ResultadoPin {
    object Correcto : ResultadoPin()
    data class Incorrecto(val intentosRestantes: Int) : ResultadoPin()
    data class Bloqueado(val segundos: Long) : ResultadoPin()
}

/**
 * Lleva la cuenta de intentos fallidos: tras 5 seguidos hay que esperar (30 s, luego el doble cada vez, hasta 15 minutos).
 * Es independiente de Android para poder probarlo.
 */
class ControlIntentos(
    private val leer: (String) -> String?,
    private val escribir: (String, String?) -> Unit,
    private val ahora: () -> Long = System::currentTimeMillis,
) {
    fun pinGuardado(): String? = leer("pin")
    fun tienePin() = pinGuardado() != null

    fun fijar(pin: String) {
        escribir("pin", Pin.crear(pin)); escribir("fallos", "0"); escribir("bloqueo_hasta", "0"); escribir("bloqueos", "0")
    }

    fun quitar() { for (k in listOf("pin", "fallos", "bloqueo_hasta", "bloqueos")) escribir(k, null) }

    fun segundosDeEspera(): Long = ((leer("bloqueo_hasta")?.toLongOrNull() ?: 0L) - ahora()).coerceAtLeast(0) / 1000

    fun probar(pin: String): ResultadoPin {
        val g = pinGuardado() ?: return ResultadoPin.Correcto
        val espera = segundosDeEspera()
        if (espera > 0) return ResultadoPin.Bloqueado(espera)
        if (Pin.verificar(pin, g)) {
            escribir("fallos", "0"); escribir("bloqueos", "0")
            return ResultadoPin.Correcto
        }
        val fallos = (leer("fallos")?.toIntOrNull() ?: 0) + 1
        if (fallos >= 5) {
            val bloqueos = (leer("bloqueos")?.toIntOrNull() ?: 0) + 1
            val seg = minOf(30L shl (bloqueos - 1).coerceAtMost(10), 15 * 60L)
            escribir("fallos", "0"); escribir("bloqueos", bloqueos.toString()); escribir("bloqueo_hasta", (ahora() + seg * 1000).toString())
            return ResultadoPin.Bloqueado(seg)
        }
        escribir("fallos", fallos.toString())
        return ResultadoPin.Incorrecto(5 - fallos)
    }
}
