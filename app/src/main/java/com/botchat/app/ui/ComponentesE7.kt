@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.runtime.remember
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.ContadorSilencio

// Piezas pequeñas que comparten el perfil de la persona, la info del grupo y la actividad (E7). Todo grande, claro y con color.

internal val VERDE_SILENCIO = Color(0xFF2E9E4F)
internal val AMARILLO_SILENCIO = Color(0xFFE0A100)
internal val ROJO_SILENCIO = Color(0xFFD64545)

internal fun colorDeSemaforo(color: String): Color = when (color) {
    "rojo" -> ROJO_SILENCIO
    "amarillo" -> AMARILLO_SILENCIO
    else -> VERDE_SILENCIO
}

/** Un botón grande con su icono y una sola palabra. */
@Composable
fun BotonGrandeE7(icono: ImageVector, texto: String, etiqueta: String, modifier: Modifier = Modifier, color: Color = MaterialTheme.colorScheme.primaryContainer, tinta: Color = MaterialTheme.colorScheme.onPrimaryContainer, alPulsar: () -> Unit) {
    Card(
        onClick = alPulsar, modifier = modifier.heightIn(min = 92.dp).testTag(etiqueta), shape = MaterialTheme.shapes.large,
        colors = CardDefaults.cardColors(containerColor = color, contentColor = tinta),
    ) {
        Column(Modifier.fillMaxWidth().padding(Esp.m), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
            Icon(icono, null, Modifier.size(34.dp))
            Spacer(Modifier.height(Esp.xs))
            Text(texto, style = MaterialTheme.typography.titleMedium, textAlign = TextAlign.Center, maxLines = 2)
        }
    }
}

/** La insignia de rango (Dueño, Ayudante…) con el color del rango. */
@Composable
fun InsigniaRango(nombre: String, colorHex: String, modifier: Modifier = Modifier) {
    if (nombre.isBlank()) return
    val c = colorDeRango(colorHex.ifBlank { "#2EC4CB" })
    Row(modifier.clip(RoundedCornerShape(50)).background(c.copy(alpha = 0.2f)).padding(horizontal = 8.dp, vertical = 2.dp), verticalAlignment = Alignment.CenterVertically) {
        PuntoRango(colorHex.ifBlank { "#2EC4CB" }, 8)
        Spacer(Modifier.width(4.dp))
        Text(nombre, style = MaterialTheme.typography.labelMedium, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

/** La barra del contador del silencio: verde (va bien), amarillo (cerca del tope) o rojo (llegó al tope). */
@Composable
fun BarraContadorSilencio(c: ContadorSilencio, modifier: Modifier = Modifier) {
    val color = colorDeSemaforo(c.color)
    val avance = if (c.tope > 0) (c.usado.toFloat() / c.tope).coerceIn(0f, 1f) else 0f
    Column(modifier.fillMaxWidth().testTag("barra_silencio"), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
        androidx.compose.material3.LinearProgressIndicator(
            progress = { if (c.usado > 0) avance.coerceAtLeast(0.04f) else 0f }, modifier = Modifier.fillMaxWidth().height(14.dp).clip(RoundedCornerShape(7.dp)).testTag("barra_silencio_${c.color}"),
            color = color, trackColor = color.copy(alpha = 0.2f), strokeCap = androidx.compose.ui.graphics.StrokeCap.Round, gapSize = 0.dp, drawStopIndicator = {},
        )
        Text(
            if (c.tope > 0) "Hoy van ${c.usado} de ${c.tope} mensajes borrados" else "Hoy van ${c.usado} mensajes borrados (sin tope)",
            Modifier.testTag("texto_contador"), style = MaterialTheme.typography.bodyMedium, color = color, fontWeight = FontWeight.SemiBold,
        )
    }
}

/** Confirmar algo que no se puede deshacer (borrar un mensaje ajeno, expulsar). No hay «Deshacer»: por eso se pregunta antes. */
@Composable
fun ConfirmarSinDeshacer(titulo: String, texto: String, boton: String, etiqueta: String, alCerrar: () -> Unit, alConfirmar: () -> Unit) {
    AlertDialog(
        onDismissRequest = alCerrar, title = { Text(titulo) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text(texto)
                Text("No se puede deshacer.", Modifier.testTag("aviso_sin_deshacer"), color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodyMedium)
            }
        },
        confirmButton = { TextButton(alConfirmar, Modifier.testTag(etiqueta)) { Text(boton, color = MaterialTheme.colorScheme.error) } },
        dismissButton = { TextButton(alCerrar, Modifier.testTag("${etiqueta}_cancelar")) { Text("Cancelar") } },
    )
}

internal fun textoDuracionFin(hastaTs: Long): String {
    if (hastaTs <= 0) return "para siempre"
    val resta = hastaTs - System.currentTimeMillis() / 1000
    return when {
        resta <= 0 -> "ya terminó"
        resta < 3600 -> "unos ${maxOf(1, resta / 60)} min más"
        resta < 86400 -> "unas ${resta / 3600} horas más"
        else -> "${resta / 86400} días más"
    }
}

/**
 * Una hoja que sube desde abajo, DENTRO de la pantalla (no es una ventana aparte). Se usa para los formularios con campos de texto:
 * así el teclado no estorba y todo se puede desplazar. Tiene la misma forma que un AlertDialog; la pantalla que la usa debe estar
 * dentro de un Box que ocupe todo el espacio.
 */
@Composable
fun HojaAlerta(
    onDismissRequest: () -> Unit, title: @Composable () -> Unit, text: @Composable () -> Unit, confirmButton: @Composable () -> Unit, dismissButton: @Composable () -> Unit = {},
) {
    androidx.activity.compose.BackHandler(onBack = onDismissRequest)
    Box(
        Modifier.fillMaxSize().background(Color.Black.copy(alpha = 0.45f))
            .clickable(interactionSource = remember { androidx.compose.foundation.interaction.MutableInteractionSource() }, indication = null, onClick = onDismissRequest)
            .testTag("hoja_fondo"),
    ) {
        androidx.compose.material3.Surface(
            Modifier.align(Alignment.BottomCenter).fillMaxWidth().heightIn(max = 640.dp).imePadding()
                .pointerInput(Unit) { detectTapGestures { } }.testTag("hoja"),
            shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp), tonalElevation = 6.dp,
        ) {
            Column(Modifier.navigationBarsPadding().padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                androidx.compose.runtime.CompositionLocalProvider(androidx.compose.material3.LocalTextStyle provides MaterialTheme.typography.titleLarge) { title() }
                Column(Modifier.weight(1f, fill = false).verticalScroll(rememberScrollState())) { text() }
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(Esp.s, Alignment.End), verticalAlignment = Alignment.CenterVertically) {
                    dismissButton(); confirmButton()
                }
            }
        }
    }
}
