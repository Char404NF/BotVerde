package com.botchat.app.seguridad

import android.app.KeyguardManager
import android.content.Context
import android.os.Build
import androidx.biometric.BiometricManager
import androidx.biometric.BiometricPrompt
import androidx.core.content.ContextCompat
import androidx.fragment.app.FragmentActivity

/** Huella y bloqueo del teléfono (androidx.biometric: no depende de servicios de Google). */
object Huella {
    private const val FUERTE = BiometricManager.Authenticators.BIOMETRIC_STRONG
    private const val TELEFONO = BiometricManager.Authenticators.DEVICE_CREDENTIAL

    /** ¿Hay una huella registrada en el teléfono? */
    fun disponible(ctx: Context): Boolean =
        BiometricManager.from(ctx).canAuthenticate(FUERTE) == BiometricManager.BIOMETRIC_SUCCESS

    /** ¿El teléfono tiene PIN, patrón o contraseña de pantalla? */
    fun telefonoTieneBloqueo(ctx: Context): Boolean =
        runCatching { ctx.getSystemService(KeyguardManager::class.java)?.isDeviceSecure == true }.getOrDefault(false)

    /** Huella con el bloqueo del teléfono como respaldo (Android 11 o superior): no hace falta un PIN propio. */
    fun conRespaldoDelTelefono(ctx: Context): Boolean =
        Build.VERSION.SDK_INT >= 30 && BiometricManager.from(ctx).canAuthenticate(FUERTE or TELEFONO) == BiometricManager.BIOMETRIC_SUCCESS

    private fun prompt(act: FragmentActivity, alExito: () -> Unit, alFallar: () -> Unit) = BiometricPrompt(
        act, ContextCompat.getMainExecutor(act),
        object : BiometricPrompt.AuthenticationCallback() {
            override fun onAuthenticationSucceeded(result: BiometricPrompt.AuthenticationResult) = alExito()
            override fun onAuthenticationError(errorCode: Int, errString: CharSequence) = alFallar()
        },
    )

    /** Pide la huella. Con `respaldoTelefono` el propio aviso ofrece el bloqueo del teléfono; si no, un botón "Usar PIN". */
    fun pedir(act: FragmentActivity, respaldoTelefono: Boolean = false, alExito: () -> Unit, alFallar: () -> Unit = {}) {
        val info = BiometricPrompt.PromptInfo.Builder().setTitle("Abrir Bot Chat").setSubtitle("Usa tu huella")
        if (respaldoTelefono && conRespaldoDelTelefono(act)) info.setAllowedAuthenticators(FUERTE or TELEFONO)
        else info.setAllowedAuthenticators(FUERTE).setNegativeButtonText("Usar PIN")
        prompt(act, alExito, alFallar).authenticate(info.build())
    }

    /** Confirma con el PIN, patrón o contraseña del teléfono (por ejemplo, para "Olvidé mi PIN"). */
    fun confirmarConTelefono(act: FragmentActivity, titulo: String, alExito: () -> Unit, alFallar: () -> Unit = {}) {
        val info = BiometricPrompt.PromptInfo.Builder().setTitle(titulo).setSubtitle("Confirma que eres tú")
            .setAllowedAuthenticators(TELEFONO).build()
        prompt(act, alExito, alFallar).authenticate(info)
    }
}
