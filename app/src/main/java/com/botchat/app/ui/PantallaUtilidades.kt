@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.style.TextOverflow
import com.botchat.app.datos.Repo
import com.botchat.app.datos.UtilItem
import kotlinx.coroutines.launch

/** El largo máximo de una nota o un pendiente (el mismo que en Go). */
internal const val MAX_TEXTO_UTIL = 500

/**
 * Ajustes → Notas y pendientes: tus notas y tu lista de pendientes (cifradas, solo en este teléfono). Lo mismo se puede hacer desde
 * "Mensajes guardados" con !anotar, !notas, !tarea, !tareas, !hecho… Abajo se explica cómo convertir una foto en sticker.
 */
@Composable
fun PantallaUtilidades(botId: Long, alVolver: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if (evs.any { it == "util:yo" }) version++ } }
    val pendientes by produceState<List<UtilItem>?>(null, version) { value = runCatching { Repo.pendientesUtil(botId) }.getOrDefault(emptyList()) }
    val notas by produceState<List<UtilItem>?>(null, version) { value = runCatching { Repo.notasUtil(botId) }.getOrDefault(emptyList()) }
    var tarea by remember { mutableStateOf("") }
    var nota by remember { mutableStateOf("") }
    fun hacer(f: suspend () -> Unit) = scope.launch {
        runCatching { f() }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
        version++
    }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Notas y pendientes") }) }) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(bottom = Esp.xl), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
            Text(
                "Solo tú las ves. Se guardan cifradas en este teléfono.",
                Modifier.padding(horizontal = Esp.l, vertical = Esp.s), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Seccion("Pendientes") {
                val l = pendientes
                when {
                    l == null -> Box(Modifier.fillMaxWidth().padding(Esp.l), Alignment.Center) { CircularProgressIndicator() }
                    l.isEmpty() -> Text("No tienes pendientes.", Modifier.padding(horizontal = Esp.l).testTag("sin_pendientes"), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    else -> l.forEachIndexed { i, x ->
                        Row(Modifier.fillMaxWidth().padding(horizontal = Esp.m).testTag("pendiente_${x.id}"), verticalAlignment = Alignment.CenterVertically) {
                            Checkbox(x.hecho, { haptica(Toque.Suave); hacer { Repo.marcarUtil(botId, x.id, it) } }, Modifier.testTag("pendiente_hecho_${x.id}"))
                            Text(
                                "${i + 1}. ${x.texto}", Modifier.weight(1f), style = MaterialTheme.typography.bodyLarge, maxLines = 3, overflow = TextOverflow.Ellipsis,
                                textDecoration = if (x.hecho) TextDecoration.LineThrough else null,
                                color = if (x.hecho) MaterialTheme.colorScheme.onSurfaceVariant else MaterialTheme.colorScheme.onSurface,
                            )
                            IconButton({ haptica(Toque.Suave); hacer { Repo.borrarUtil(botId, x.id) } }, Modifier.testTag("borrar_pendiente_${x.id}")) { Icon(Ico.Borrar, "Borrar el pendiente") }
                        }
                    }
                }
                Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l), verticalAlignment = Alignment.CenterVertically) {
                    OutlinedTextField(tarea, { tarea = it.take(MAX_TEXTO_UTIL) }, Modifier.weight(1f).testTag("pendiente_nuevo"), label = { Text("Nuevo pendiente") }, singleLine = true)
                    TextButton({ val t = tarea.trim(); tarea = ""; hacer { Repo.agregarPendienteUtil(botId, t) } }, Modifier.testTag("agregar_pendiente"), enabled = tarea.isNotBlank()) { Text("Agregar") }
                }
                if (l?.any { it.hecho } == true) {
                    TextButton({ haptica(Toque.Suave); hacer { Repo.limpiarUtilHechas(botId) } }, Modifier.padding(horizontal = Esp.m).testTag("limpiar_hechos")) { Text("Quitar los que ya están hechos") }
                }
            }
            Seccion("Notas") {
                val l = notas
                when {
                    l == null -> Box(Modifier.fillMaxWidth().padding(Esp.l), Alignment.Center) { CircularProgressIndicator() }
                    l.isEmpty() -> Text("No tienes notas.", Modifier.padding(horizontal = Esp.l).testTag("sin_notas"), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    else -> l.forEachIndexed { i, x ->
                        Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l).testTag("nota_${x.id}"), verticalAlignment = Alignment.CenterVertically) {
                            Text("${i + 1}. ${x.texto}", Modifier.weight(1f), style = MaterialTheme.typography.bodyLarge)
                            IconButton({ haptica(Toque.Suave); hacer { Repo.borrarUtil(botId, x.id) } }, Modifier.testTag("borrar_nota_${x.id}")) { Icon(Ico.Borrar, "Borrar la nota") }
                        }
                    }
                }
                Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l), verticalAlignment = Alignment.CenterVertically) {
                    OutlinedTextField(nota, { nota = it.take(MAX_TEXTO_UTIL) }, Modifier.weight(1f).testTag("nota_nueva"), label = { Text("Nueva nota") }, singleLine = true)
                    TextButton({ val t = nota.trim(); nota = ""; hacer { Repo.agregarNotaUtil(botId, t) } }, Modifier.testTag("agregar_nota"), enabled = nota.isNotBlank()) { Text("Agregar") }
                }
            }
            Seccion("Por comando") {
                Column(Modifier.padding(horizontal = Esp.l).testTag("ayuda_comandos_util"), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                    Text("Desde “Mensajes guardados” (o en tu chat privado con el bot, si tu rango lo permite):", style = MaterialTheme.typography.bodyMedium)
                    Text("!anotar texto · !notas · !borrarnota 2", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Text("!tarea texto · !tareas · !hecho 1 · !deshacer 1 · !borrartarea 1 · !limpiartareas", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Text("En un grupo no funcionan: la respuesta la vería todo el grupo.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
            Seccion("Convertir una foto en sticker") {
                Column(Modifier.padding(horizontal = Esp.l).testTag("ayuda_sticker"), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                    Text("Responde a una foto con !sticker, o mándala con !sticker de pie de foto. El bot contesta con el sticker (512×512, fondo transparente, hasta unos 100 KB).", style = MaterialTheme.typography.bodyMedium)
                    Text("Quién puede usarlo se elige en Rangos y permisos (casilla “Convertir imagen en sticker”). Las fotos de “ver una vez” y las eliminadas no se convierten.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
        }
    }
}
