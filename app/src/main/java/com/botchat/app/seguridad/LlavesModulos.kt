package com.botchat.app.seguridad

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import java.io.File
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/**
 * Llaves de los módulos (por ejemplo la llave de la IA, v0.5.0 H7). Cada llave se guarda CIFRADA con una llave AES-GCM del Android
 * Keystore (no exportable) en `files/llaves_mod/`. Nunca va a la base, al repositorio, a los registros ni a la configuración del
 * módulo: la app se la pasa a Go solo en memoria (`cargarEnMotor`, al abrir la app, y `guardar`, al escribirla).
 */
object LlavesModulos {
    /** Cómo se cifra: el Keystore en el teléfono; las pruebas ponen uno de software. */
    interface Cifrador {
        fun cifrar(claro: ByteArray): ByteArray
        fun descifrar(cifrado: ByteArray): ByteArray
    }

    object KeystoreCifrador : Cifrador {
        private const val ALIAS = "botchat_llaves_modulos_v1"

        private fun llave(): SecretKey {
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

        override fun cifrar(claro: ByteArray): ByteArray {
            val c = Cipher.getInstance("AES/GCM/NoPadding")
            c.init(Cipher.ENCRYPT_MODE, llave())
            return byteArrayOf(1) + c.iv + c.doFinal(claro)
        }

        override fun descifrar(cifrado: ByteArray): ByteArray {
            require(cifrado.size > 1 + 12 + 16 && cifrado[0].toInt() == 1)
            val c = Cipher.getInstance("AES/GCM/NoPadding")
            c.init(Cipher.DECRYPT_MODE, llave(), GCMParameterSpec(128, cifrado, 1, 12))
            return c.doFinal(cifrado, 13, cifrado.size - 13)
        }
    }

    @Volatile var cifrador: Cifrador = KeystoreCifrador

    private val reId = Regex("^[a-z0-9_]{1,40}$")

    private fun carpeta(ctx: Context) = File(ctx.filesDir, "llaves_mod")

    private fun archivo(ctx: Context, bot: Long, modulo: String, clave: String): File {
        require(reId.matches(modulo) && reId.matches(clave)) { "nombre de módulo o llave no válido" }
        return File(carpeta(ctx), "$bot.$modulo.$clave.bin")
    }

    /** Guarda la llave cifrada. Un valor vacío la borra. */
    @Synchronized
    fun guardar(ctx: Context, bot: Long, modulo: String, clave: String, valor: String) {
        val f = archivo(ctx, bot, modulo, clave)
        if (valor.isBlank()) { f.delete(); return }
        carpeta(ctx).mkdirs()
        val tmp = File(f.path + ".tmp")
        tmp.writeBytes(cifrador.cifrar(valor.trim().toByteArray(Charsets.UTF_8)))
        if (!tmp.renameTo(f)) { f.delete(); tmp.renameTo(f) }
    }

    fun existe(ctx: Context, bot: Long, modulo: String, clave: String): Boolean = archivo(ctx, bot, modulo, clave).exists()

    /** La llave en claro (solo para pasársela a Go); null si no hay o ya no se puede descifrar. */
    @Synchronized
    fun obtener(ctx: Context, bot: Long, modulo: String, clave: String): String? {
        val f = archivo(ctx, bot, modulo, clave)
        if (!f.exists()) return null
        return runCatching { String(cifrador.descifrar(f.readBytes()), Charsets.UTF_8) }.getOrNull()
    }

    fun borrar(ctx: Context, bot: Long, modulo: String, clave: String) { archivo(ctx, bot, modulo, clave).delete() }

    /** Borra las llaves de un bot (cuando se elimina). */
    @Synchronized
    fun borrarDeBot(ctx: Context, bot: Long) {
        carpeta(ctx).listFiles()?.filter { it.name.startsWith("$bot.") }?.forEach { it.delete() }
    }

    /** Le pasa a Go (en memoria) todas las llaves guardadas. Una que ya no se pueda descifrar se ignora. */
    @Synchronized
    fun cargarEnMotor(ctx: Context, poner: (bot: Long, modulo: String, clave: String, valor: String) -> Unit) {
        carpeta(ctx).listFiles()?.filter { it.name.endsWith(".bin") }?.forEach { f ->
            val p = f.name.removeSuffix(".bin").split('.')
            if (p.size != 3) return@forEach
            val bot = p[0].toLongOrNull() ?: return@forEach
            val v = obtener(ctx, bot, p[1], p[2]) ?: return@forEach
            runCatching { poner(bot, p[1], p[2], v) }
        }
    }
}
