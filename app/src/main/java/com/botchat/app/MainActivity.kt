package com.botchat.app

import android.os.Bundle
import android.view.WindowManager
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.fragment.app.FragmentActivity
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import com.botchat.app.datos.Ajustes
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.EstadoBloqueo
import com.botchat.app.ui.Raiz
import kotlinx.coroutines.launch

/** FragmentActivity (en vez de ComponentActivity) porque la huella (androidx.biometric) lo necesita. */
class MainActivity : FragmentActivity() {
    private val ajustes by lazy { Ajustes.get(this) }

    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge()
        super.onCreate(savedInstanceState)
        if (savedInstanceState == null) EstadoBloqueo.iniciar(ajustes)
        // FLAG_SECURE: sin capturas de pantalla ni vista previa en apps recientes (activado por defecto; se cambia en Avanzado).
        lifecycleScope.launch {
            repeatOnLifecycle(Lifecycle.State.CREATED) {
                ajustes.secure.collect { activo ->
                    if (activo) window.setFlags(WindowManager.LayoutParams.FLAG_SECURE, WindowManager.LayoutParams.FLAG_SECURE)
                    else window.clearFlags(WindowManager.LayoutParams.FLAG_SECURE)
                }
            }
        }
        setContent { BotChatTema { Raiz() } }
    }

    override fun onStop() {
        super.onStop()
        EstadoBloqueo.alSalir()
    }

    override fun onStart() {
        super.onStart()
        EstadoBloqueo.alVolver(this, ajustes) // tras 1 minuto en segundo plano se vuelve a pedir el PIN
    }
}
