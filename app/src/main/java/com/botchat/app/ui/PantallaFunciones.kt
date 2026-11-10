package com.botchat.app.ui

import androidx.compose.foundation.layout.Column
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
    val motor by ajustes.motorAutomatico.collectAsState()
    val escribir by ajustes.escribirDesdeApp.collectAsState()
    val notifs by ajustes.notificacionesMensajes.collectAsState()
    val conservar by ajustes.conservarEliminados.collectAsState()
    val enLinea by ajustes.mostrarEnLinea.collectAsState()
    Scaffold(
        modifier = Modifier.nestedScroll(barra.nestedScrollConnection),
        topBar = { LargeTopAppBar(scrollBehavior = barra, navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Funciones") }) },
    ) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(bottom = 32.dp)) {
            Seccion("Se pueden apagar sin reinstalar") {
                Interruptor("Motor de automatización", "Las respuestas automáticas del bot (el bot solo responde, nunca escribe primero)", motor, true, Ico.Chats) { haptica(Toque.Suave); ajustes.fijarMotorAutomatico(it) }
                Interruptor("Escribir desde la app", "Poder mandar mensajes a mano desde los chats", escribir, true, Ico.Compartir) { haptica(Toque.Suave); ajustes.fijarEscribirDesdeApp(it) }
                Interruptor("Notificaciones de mensajes", "Avisar cuando llega un mensaje nuevo (apagado por defecto)", notifs, true, Ico.Aviso) { haptica(Toque.Suave); ajustes.fijarNotificacionesMensajes(it) }
                Interruptor("Conservar eliminados", "Guardar lo que otros eliminan para poder verlo", conservar, true, Ico.Candado) { haptica(Toque.Suave); ajustes.fijarConservarEliminados(it) }
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
