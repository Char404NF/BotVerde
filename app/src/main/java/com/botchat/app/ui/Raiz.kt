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

/** Raíz de la app: bloqueo (solo si el usuario lo activó), aviso o bienvenida, asistente si no hay bot y la navegación principal. */
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

    val aceptado by ajustes.avisoAceptado.collectAsState()
    BackHandler(enabled = vm.puedeVolver && vm.pantalla != Pantalla.PRINCIPAL && aceptado) { vm.atras(ctx) }

    val bloqueado by EstadoBloqueo.bloqueado.collectAsState()
    val modo by ajustes.bloqueo.collectAsState()
    val errorDatos by Repo.error.collectAsState()

    Surface(Modifier.fillMaxSize()) {
        Box(Modifier.fillMaxSize()) {
            when {
                errorDatos != null -> PantallaErrorDatos(errorDatos ?: "")
                modo != Ajustes.Bloqueo.NINGUNO && bloqueado -> PantallaBloqueo(ajustes) { EstadoBloqueo.desbloquear() }
                // Quien ya tenía el bot vinculado solo ve el aviso una vez; quien empieza ve la bienvenida completa.
                !aceptado -> if (vm.bot?.algunaVez == true) AvisoUnico { ajustes.aceptarAviso() } else Bienvenida { ajustes.aceptarAviso() }
                vm.pantalla != Pantalla.PRINCIPAL -> Asistente(vm, conPermiso)
                else -> Shell(nav, vm, ajustes, conPermiso)
            }
            Splash()
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
