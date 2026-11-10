package com.botchat.app.ui

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Button
import androidx.compose.material3.Surface
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.core.content.ContextCompat
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.navigation.compose.rememberNavController
import com.botchat.app.AppVm
import com.botchat.app.Pantalla
import com.botchat.app.datos.Ajustes
import com.botchat.app.datos.Repo

/** Raíz de la app: asistente si no hay bot, bloqueo con PIN, y la navegación principal cuando hay mensajes que ver. */
@Composable
fun Raiz() {
    val vm: AppVm = viewModel()
    val ctx = LocalContext.current
    val ajustes = remember { Ajustes.get(ctx) }
    val nav = rememberNavController()

    // Permiso de notificaciones (Android 13+): se pide antes de iniciar el bot.
    val pendiente = remember { mutableStateOf<(() -> Unit)?>(null) }
    val lanzador = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) {
        pendiente.value?.invoke()
        pendiente.value = null
    }
    val conPermiso: (() -> Unit) -> Unit = { accion ->
        if (Build.VERSION.SDK_INT >= 33 && ContextCompat.checkSelfPermission(ctx, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
            pendiente.value = accion
            lanzador.launch(Manifest.permission.POST_NOTIFICATIONS)
        } else {
            accion()
        }
    }

    BackHandler(enabled = vm.puedeVolver && vm.pantalla != Pantalla.PRINCIPAL) { vm.atras(ctx) }

    val bloqueado by EstadoBloqueo.bloqueado.collectAsState()
    val tienePin by ajustes.tienePin.collectAsState()
    val omitioPin by ajustes.omitioPin.collectAsState()
    val errorDatos by Repo.error.collectAsState()
    var cambiandoPin by remember { mutableStateOf(false) }

    Surface(Modifier.fillMaxSize()) {
        when {
            vm.pantalla != Pantalla.PRINCIPAL -> Asistente(vm, conPermiso)
            errorDatos != null -> PantallaErrorDatos(errorDatos ?: "")
            cambiandoPin -> PantallaConfigurarPin(if (tienePin) "Elige un PIN nuevo" else "Protege tus mensajes con un PIN", { ajustes.fijarPin(it); EstadoBloqueo.desbloquear(); cambiandoPin = false }, { cambiandoPin = false })
            tienePin && bloqueado -> PantallaBloqueo(ajustes) { EstadoBloqueo.desbloquear() }
            !tienePin && !omitioPin -> PantallaConfigurarPin("Protege tus mensajes con un PIN", { ajustes.fijarPin(it); EstadoBloqueo.desbloquear() }, { ajustes.omitirPin() })
            else -> Shell(nav, vm, ajustes, conPermiso, alCambiarPin = { cambiandoPin = true })
        }
    }
}

@Composable
private fun PantallaErrorDatos(mensaje: String) {
    Box(Modifier.fillMaxSize()) {
        EstadoVacio(
            Ico.Aviso, "No se pueden abrir tus mensajes",
            "$mensaje.\n\nLa llave que protege los mensajes ya no está disponible en este teléfono (por ejemplo, tras restablecer la seguridad). Si instalaste la app de nuevo, hay que eliminar el bot y vincularlo otra vez.",
            Modifier.align(androidx.compose.ui.Alignment.Center),
        )
    }
}
