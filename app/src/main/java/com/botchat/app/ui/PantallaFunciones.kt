package com.botchat.app.ui

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import android.widget.Toast
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Column
import androidx.compose.ui.platform.LocalContext
import androidx.core.content.ContextCompat
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LargeTopAppBar
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.nestedscroll.nestedScroll
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.Ajustes

/** Avanzado → Funciones: interruptores para apagar partes de la app sin reinstalar. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PantallaFunciones(ajustes: Ajustes, alVolver: () -> Unit) {
    val barra = TopAppBarDefaults.exitUntilCollapsedScrollBehavior()
    val haptica = rememberHaptica()
    val ctx = LocalContext.current
    val permiso = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { ok ->
        if (ok) ajustes.fijarNotificacionesMensajes(true) else Toast.makeText(ctx, "Sin permiso, no se pueden mostrar avisos", Toast.LENGTH_LONG).show()
    }
    val motor by ajustes.motorAutomatico.collectAsState()
    val escribir by ajustes.escribirDesdeApp.collectAsState()
    val notifs by ajustes.notificacionesMensajes.collectAsState()
    val conservar by ajustes.conservarEliminados.collectAsState()
    val enLinea by ajustes.mostrarEnLinea.collectAsState()
    val leido by ajustes.marcarLeidoAlAbrir.collectAsState()
    val pausa by ajustes.pausaBotMin.collectAsState()
    val verDias by ajustes.verUnaVezDias.collectAsState()
    val antiguedad by ajustes.antiguedadMin.collectAsState()
    Scaffold(
        modifier = Modifier.nestedScroll(barra.nestedScrollConnection),
        topBar = { LargeTopAppBar(scrollBehavior = barra, navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Funciones") }) },
    ) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(bottom = 32.dp)) {
            Seccion("Se pueden apagar sin reinstalar") {
                Interruptor("Motor de automatización", "Las respuestas automáticas del bot (el bot solo responde, nunca escribe primero)", motor, true, Ico.Chats) { haptica(Toque.Suave); ajustes.fijarMotorAutomatico(it) }
                Deslizador("No responder mensajes de hace más de", "$antiguedad min", antiguedad.toFloat(), 1f..60f, 58, "El bot no contesta lo que llegó hace más que esto, ni lo que WhatsApp entrega de golpe al reconectar.") { ajustes.fijarAntiguedadMin(it.toInt().coerceIn(1, 60)) }
                Interruptor("Escribir desde la app", "Poder mandar mensajes a mano desde los chats", escribir, true, Ico.Compartir) { haptica(Toque.Suave); ajustes.fijarEscribirDesdeApp(it) }
                Interruptor("Notificaciones de mensajes", "Avisar cuando llega un mensaje nuevo. En la pantalla bloqueada solo dice “Mensaje nuevo”. Puedes silenciar chats concretos desde el chat.", notifs, true, Ico.Aviso) {
                    haptica(Toque.Suave)
                    if (!it) ajustes.fijarNotificacionesMensajes(false)
                    else if (Build.VERSION.SDK_INT >= 33 && ContextCompat.checkSelfPermission(ctx, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) permiso.launch(Manifest.permission.POST_NOTIFICATIONS)
                    else ajustes.fijarNotificacionesMensajes(true)
                }
                Interruptor("Conservar eliminados", "Guardar lo que otros eliminan para poder verlo", conservar, true, Ico.Candado) { haptica(Toque.Suave); ajustes.fijarConservarEliminados(it) }
                Interruptor("Marcar como leído al abrir", "Al abrir un chat se avisa “leído”. Si en WhatsApp tienes desactivadas las confirmaciones de lectura, nunca se envían.", leido, true, Ico.Listo) { haptica(Toque.Suave); ajustes.fijarMarcarLeidoAlAbrir(it) }
                Deslizador("Pausa del bot cuando escribo yo", if (pausa == 0) "Nunca" else "$pausa min", pausa.toFloat(), 0f..120f, 7, "Si escribes en un chat (desde la app o desde el teléfono), el bot calla ahí este tiempo.") { ajustes.fijarPausaBotMin((it / 10f).toInt() * 10) }
                Deslizador("Días para abrir un “ver una vez”", if (verDias == 1) "1 día" else "$verDias días", verDias.toFloat(), 1f..30f, 28, "Si no lo abres a tiempo, se borra y queda como “Expiró”.") { ajustes.fijarVerUnaVezDias(it.toInt().coerceIn(1, 30)) }
                Interruptor("Mostrar en línea", "Con un chat abierto, tus contactos pueden verte “en línea”. Puede hacer que tu teléfono principal deje de avisarte de mensajes mientras tanto.", enLinea, true, Ico.Info) { haptica(Toque.Suave); ajustes.fijarMostrarEnLinea(it) }
            }
        }
    }
}

/** Ajustes → Privacidad. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PantallaPrivacidad(ajustes: Ajustes, alVolver: () -> Unit) {
    val barra = TopAppBarDefaults.exitUntilCollapsedScrollBehavior()
    val haptica = rememberHaptica()
    val conservar by ajustes.conservarEliminados.collectAsState()
    Scaffold(
        modifier = Modifier.nestedScroll(barra.nestedScrollConnection),
        topBar = { LargeTopAppBar(scrollBehavior = barra, navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Privacidad") }) },
    ) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(bottom = 32.dp)) {
            Seccion("Mensajes eliminados") {
                Interruptor("Conservar mensajes eliminados por otros", "Cuando alguien elimina un mensaje para todos, aquí se sigue viendo con la etiqueta “Eliminado”.", conservar, true, Ico.Candado) { haptica(Toque.Suave); ajustes.fijarConservarEliminados(it) }
                Text(
                    "Es una herramienta local de tu teléfono: nada sale de aquí. Conservar lo que otros eliminan queda bajo tu responsabilidad. " +
                        "Si lo apagas, al llegar una eliminación se borra el contenido y el archivo. Con una pulsación larga en un mensaje puedes olvidarlo para siempre.",
                    Modifier.padding(horizontal = Esp.l), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }
}
