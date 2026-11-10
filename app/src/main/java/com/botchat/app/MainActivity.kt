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
        // FLAG_SECURE: sin capturas ni vista previa en apps recientes, solo si el usuario lo activa en Ajustes → Seguridad (apagado por defecto).
        lifecycleScope.launch {
            repeatOnLifecycle(Lifecycle.State.CREATED) {
                ajustes.secure.collect { activo ->
                    if (activo) window.setFlags(WindowManager.LayoutParams.FLAG_SECURE, WindowManager.LayoutParams.FLAG_SECURE)
                    else window.clearFlags(WindowManager.LayoutParams.FLAG_SECURE)
                }
            }
        }
        pideChatDe(intent)
        setContent { BotChatTema { Raiz() } }
    }

    override fun onNewIntent(intent: android.content.Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        pideChatDe(intent)
    }

    /** Si la app se abrió desde una notificación de mensaje, abre ese chat. */
    private fun pideChatDe(i: android.content.Intent?) {
        i?.getStringExtra(Notificador.EXTRA_CHAT)?.let { NavegarA.chat.value = it; i.removeExtra(Notificador.EXTRA_CHAT) }
    }

    override fun onStop() {
        super.onStop()
        if (!isChangingConfigurations) EstadoBloqueo.alSalir() // girar la pantalla no cuenta como salir
    }

    override fun onStart() {
        super.onStart()
        com.botchat.app.datos.Repo.actualizarZona() // por si el teléfono cambió de zona horaria
        EstadoBloqueo.alVolver(this, ajustes) // si hay bloqueo activo, se vuelve a pedir tras el tiempo elegido
    }
}
