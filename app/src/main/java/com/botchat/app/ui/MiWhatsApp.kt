@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import com.botchat.app.ui.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import com.botchat.app.datos.EstadoAppState
import com.botchat.app.datos.Repo
import kotlinx.coroutines.launch

/** Duraciones para silenciar en MI WhatsApp: 8 horas, 1 semana o siempre (0). */
internal val DURACIONES_MI_WHATSAPP = listOf(Triple("8h", "8 horas", 8 * 3600L), Triple("1s", "1 semana", 7 * 24 * 3600L), Triple("siempre", "Siempre", 0L))

/**
 * Silenciar, fijar y archivar este chat en MI WhatsApp (la cuenta principal). No escribe a nadie. Solo aparece si el teléfono principal
 * ya compartió las llaves con este dispositivo; si no, la tarjeta no se muestra (la explicación está en Avanzado).
 */
@Composable
fun TarjetaMiWhatsApp(botId: Long, chat: String, nombre: String) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    val estado by produceState<EstadoAppState?>(null, botId) { value = runCatching { Repo.estadoAppState(botId) }.getOrNull() }
    var dialogo by remember { mutableStateOf(false) }
    var dur by remember { mutableLongStateOf(8 * 3600L) }
    if (estado?.disponible != true) return
    fun hacer(ok: String, accion: suspend () -> Unit) = scope.launch {
        runCatching { accion() }.onSuccess { avisaToast(ctx, ok, false) }.onFailure { avisaToast(ctx, textoError(it), true) }
    }
    Card(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.xs).testTag("tarjeta_mi_whatsapp"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Text("En tu WhatsApp", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary)
            Text("Cambia cómo ves este chat en tu WhatsApp principal. No se manda nada a $nombre.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                OutlinedButton({ haptica(Toque.Suave); dialogo = true }, Modifier.testTag("mw_silenciar")) { Text("Silenciar…") }
                OutlinedButton({ haptica(Toque.Suave); hacer("Listo: quité el silencio en tu WhatsApp") { Repo.desilenciarEnMiWhatsApp(botId, chat) } }, Modifier.testTag("mw_desilenciar")) { Text("Quitar silencio") }
                OutlinedButton({ haptica(Toque.Suave); hacer("Listo: lo fijé en tu WhatsApp") { Repo.fijarEnMiWhatsApp(botId, chat, true) } }, Modifier.testTag("mw_fijar")) { Text("Fijar") }
                OutlinedButton({ haptica(Toque.Suave); hacer("Listo: ya no está fijado") { Repo.fijarEnMiWhatsApp(botId, chat, false) } }, Modifier.testTag("mw_desfijar")) { Text("Quitar de fijados") }
                OutlinedButton({ haptica(Toque.Suave); hacer("Listo: lo archivé en tu WhatsApp") { Repo.archivarEnMiWhatsApp(botId, chat, true) } }, Modifier.testTag("mw_archivar")) { Text("Archivar") }
                OutlinedButton({ haptica(Toque.Suave); hacer("Listo: ya no está archivado") { Repo.archivarEnMiWhatsApp(botId, chat, false) } }, Modifier.testTag("mw_desarchivar")) { Text("Sacar de archivados") }
            }
        }
    }
    if (dialogo) AlertDialog(
        onDismissRequest = { dialogo = false }, title = { Text("Silenciar en tu WhatsApp") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("Tu WhatsApp no te avisará de este chat durante este tiempo.", style = MaterialTheme.typography.bodySmall)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    DURACIONES_MI_WHATSAPP.forEach { (id, texto, seg) -> FilterChip(dur == seg, { dur = seg }, { Text(texto) }, modifier = Modifier.testTag("mw_silencio_$id")) }
                }
            }
        },
        confirmButton = { TextButton({ dialogo = false; hacer("Listo: silenciado en tu WhatsApp") { Repo.silenciarEnMiWhatsApp(botId, chat, dur) } }, Modifier.testTag("mw_confirmar_silencio")) { Text("Silenciar") } },
        dismissButton = { TextButton({ dialogo = false }) { Text("Cancelar") } },
    )
}

/** La sección de Avanzado sobre silenciar/fijar/archivar en tu WhatsApp: dice si está disponible y, si no, qué hacer. */
@Composable
fun SeccionMiWhatsApp(botId: Long) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var version by remember { mutableStateOf(0) }
    val estado by produceState<EstadoAppState?>(null, botId, version) { value = runCatching { Repo.estadoAppState(botId) }.getOrNull() }
    var pidiendo by remember { mutableStateOf(false) }
    Seccion("Tu WhatsApp (silenciar, fijar, archivar)") {
        val e = estado
        Text(
            when {
                e == null -> "Comprobando…"
                e.disponible -> "Listo: en la ficha de cada chat verás los botones para silenciar, fijar y archivar en tu WhatsApp."
                else -> e.motivo
            },
            Modifier.padding(horizontal = Esp.l).testTag("appstate_texto"), style = MaterialTheme.typography.bodyMedium,
            color = if (e?.disponible == true) MaterialTheme.colorScheme.onSurface else MaterialTheme.colorScheme.onSurfaceVariant,
        )
        if (e != null && e.conectado && !e.disponible) {
            OutlinedButton({
                pidiendo = true
                scope.launch {
                    runCatching { Repo.pedirSincronizacionAppState(botId) }.onSuccess { avisaToast(ctx, "Listo: se la pedí a WhatsApp. Espera un momento y vuelve a mirar.", false) }.onFailure { avisaToast(ctx, textoError(it), true) }
                    pidiendo = false; version++
                }
            }, Modifier.padding(horizontal = Esp.l).testTag("appstate_sincronizar"), enabled = !pidiendo) { Text("Pedir sincronización") }
        }
    }
}
