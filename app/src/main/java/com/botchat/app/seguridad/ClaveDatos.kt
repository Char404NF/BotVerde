package com.botchat.app.seguridad

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import java.io.File
import java.security.KeyStore
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/**
 * Llave de datos: 32 bytes aleatorios que se generan la primera vez. Se guardan cifrados con una llave AES-GCM del
 * Android Keystore (no exportable y sin pedir huella ni PIN, para que el servicio funcione con la pantalla bloqueada).
 * Kotlin la descifra al iniciar y se la pasa al módulo Go; vive solo en memoria. Nunca va a registros ni a copias.
 */
object ClaveDatos {
    private const val ALIAS = "botchat_envolver_v1"
    private const val ARCHIVO = "clave.bin"
    @Volatile private var enMemoria: ByteArray? = null

    class NoDisponible(mensaje: String, causa: Throwable? = null) : Exception(mensaje, causa)

    private fun llaveKeystore(): SecretKey {
        val ks = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (ks.getKey(ALIAS, null) as? SecretKey)?.let { return it }
        val gen = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
        gen.init(
            KeyGenParameterSpec.Builder(ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                .setKeySize(256)
                .setUserAuthenticationRequired(false)
                .build(),
        )
        return gen.generateKey()
    }

    /** Devuelve la llave de datos (la crea si es la primera vez). Lanza [NoDisponible] si el teléfono ya no puede descifrarla. */
    @Synchronized
    fun obtener(ctx: Context): ByteArray {
        enMemoria?.let { return it.copyOf() }
        val f = File(ctx.filesDir, ARCHIVO)
        val clave = try {
            if (f.exists()) {
                val b = f.readBytes()
                require(b.size > 1 + 12 + 16 && b[0].toInt() == 1)
                val c = Cipher.getInstance("AES/GCM/NoPadding")
                c.init(Cipher.DECRYPT_MODE, llaveKeystore(), GCMParameterSpec(128, b, 1, 12))
                c.doFinal(b, 13, b.size - 13)
            } else {
                val nueva = ByteArray(FormatoCifrado.TAM_CLAVE).also { SecureRandom().nextBytes(it) }
                val c = Cipher.getInstance("AES/GCM/NoPadding")
                c.init(Cipher.ENCRYPT_MODE, llaveKeystore())
                val ct = c.doFinal(nueva)
                f.writeBytes(byteArrayOf(1) + c.iv + ct)
                nueva
            }
        } catch (e: Exception) {
            throw NoDisponible("No se pudo abrir la llave de datos de este teléfono", e)
        }
        enMemoria = clave
        return clave.copyOf()
    }

    /** ¿Ya existe una llave guardada? (para saber si hay datos que dependen de ella) */
    fun existe(ctx: Context): Boolean = File(ctx.filesDir, ARCHIVO).exists()
}
