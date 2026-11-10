package com.botchat.app.seguridad

import android.content.Context
import androidx.biometric.BiometricManager
import androidx.biometric.BiometricPrompt
import androidx.core.content.ContextCompat
import androidx.fragment.app.FragmentActivity

/** Huella opcional (androidx.biometric: no depende de servicios de Google). */
object Huella {
    fun disponible(ctx: Context): Boolean =
        BiometricManager.from(ctx).canAuthenticate(BiometricManager.Authenticators.BIOMETRIC_STRONG) == BiometricManager.BIOMETRIC_SUCCESS

    fun pedir(act: FragmentActivity, alExito: () -> Unit, alFallar: () -> Unit = {}) {
        val prompt = BiometricPrompt(
            act, ContextCompat.getMainExecutor(act),
            object : BiometricPrompt.AuthenticationCallback() {
                override fun onAuthenticationSucceeded(result: BiometricPrompt.AuthenticationResult) = alExito()
                override fun onAuthenticationError(errorCode: Int, errString: CharSequence) = alFallar()
            },
        )
        val info = BiometricPrompt.PromptInfo.Builder()
            .setTitle("Desbloquear BotChat")
            .setSubtitle("Usa tu huella")
            .setNegativeButtonText("Usar PIN")
            .setAllowedAuthenticators(BiometricManager.Authenticators.BIOMETRIC_STRONG)
            .build()
        prompt.authenticate(info)
    }
}
