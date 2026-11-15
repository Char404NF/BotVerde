@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import android.graphics.BitmapFactory
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SuggestionChip
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.DescargaVideo
import com.botchat.app.datos.EstadoVideo
import com.botchat.app.datos.Repo
import kotlinx.coroutines.launch

/** Peso en palabras simples ("45 MB", "1,2 GB"). */
fun pesoVideoTxt(b: Long): String = when {
    b <= 0 -> ""
    b >= 1L shl 30 -> "%.1f GB".format(b / (1024.0 * 1024 * 1024))
    else -> "${maxOf(1L, Math.round(b / (1024.0 * 1024)))} MB"
}

/** Duración "3:20" o "1 h 05 min". */
fun duracionVideoTxt(s: Int): String = if (s >= 3600) "%d h %02d min".format(s / 3600, (s % 3600) / 60) else "%d:%02d".format(s / 60, s % 60)

/** Tarjeta de arriba: versión de yt-dlp, si hay una nueva y un botón para actualizar. También sirve dentro de la ventana del módulo. */
@Composable
fun TarjetaMotorVideo(botId: Long, modifier: Modifier = Modifier, alAbrirHistorial: (() -> Unit)? = null) {
    val scope = rememberCoroutineScope()
    var version by remember { mutableStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("descargas_video" in evs || "modulos" in evs) version++ } }
    val estado by produceState(EstadoVideo(), version, botId) { value = runCatching { Repo.videoEstado(botId) }.getOrDefault(EstadoVideo()) }
    var mensaje by remember { mutableStateOf<String?>(null) }
    var trabajando by remember { mutableStateOf(false) }
    Card(modifier.fillMaxWidth().testTag("tarjeta_motor_video"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Text("Motor de descargas", style = MaterialTheme.typography.titleMedium)
            Text(
                if (estado.version.isBlank()) "Todavía no se ha revisado la versión de yt-dlp." else "yt-dlp ${estado.version}" + if (estado.hayNueva) " · hay una versión nueva (${estado.ultima})" else " · al día",
                Modifier.testTag("version_ytdlp"), style = MaterialTheme.typography.bodyMedium,
                color = if (estado.hayNueva) MaterialTheme.colorScheme.tertiary else MaterialTheme.colorScheme.onSurfaceVariant,
            )
            if (estado.revisionTs > 0) Text("Última revisión: ${haceCuanto(estado.revisionTs)}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            if (estado.error.isNotBlank()) Text(estado.error, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                SuggestionChip({}, { Text("Hoy: ${estado.hoy}") })
                if (estado.enCurso > 0) SuggestionChip({}, { Text("Bajando: ${estado.enCurso}") })
                if (estado.esperando > 0) SuggestionChip({}, { Text("En espera: ${estado.esperando}") })
            }
            mensaje?.let { Text(it, Modifier.testTag("mensaje_motor_video"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.primary) }
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                Button({
                    trabajando = true; mensaje = null
                    scope.launch { mensaje = runCatching { Repo.videoActualizar(botId) }.getOrElse { it.message ?: "No se pudo actualizar" }; trabajando = false; version++ }
                }, enabled = !trabajando && !estado.actualizando, modifier = Modifier.testTag("actualizar_ytdlp")) {
                    if (trabajando || estado.actualizando) CircularProgressIndicator(Modifier.size(16.dp), strokeWidth = 2.dp) else Icon(Ico.Actualizar, null, Modifier.size(18.dp))
                    Spacer(Modifier.width(Esp.s))
                    Text(if (estado.hayNueva) "Actualizar ahora" else "Buscar actualización")
                }
                if (alAbrirHistorial != null) OutlinedButton(alAbrirHistorial, Modifier.testTag("abrir_historial_video")) { Text("Ver descargas") }
            }
        }
    }
}

/** Historial de descargas: miniatura, título, quién la pidió y cuándo, peso, formato, y un botón para volver a bajarla. */
@Composable
fun PantallaDescargasVideo(botId: Long, alVolver: () -> Unit) {
    val scope = rememberCoroutineScope()
    var version by remember { mutableStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("descargas_video" in evs) version++ } }
    val filas = remember { mutableStateListOf<DescargaVideo>() }
    var cargado by remember { mutableStateOf(false) }
    var hayMas by remember { mutableStateOf(false) }
    LaunchedEffect(version, botId) {
        val l = runCatching { Repo.videoLista(botId) }.getOrDefault(emptyList())
        filas.clear(); filas.addAll(l); hayMas = l.size >= 40; cargado = true
    }
    var borrando by remember { mutableStateOf<DescargaVideo?>(null) }
    var vaciando by remember { mutableStateOf(false) }
    var aviso by remember { mutableStateOf<String?>(null) }
    Scaffold(topBar = {
        TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Descargas") }, actions = {
            if (filas.isNotEmpty()) IconButton({ vaciando = true }, Modifier.testTag("vaciar_historial_video")) { Icon(Ico.Borrar, "Vaciar el historial") }
        })
    }) { relleno ->
        LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("lista_descargas_video"), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            item { TarjetaMotorVideo(botId) }
            aviso?.let { a -> item { Text(a, Modifier.testTag("aviso_descargas_video"), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.primary) } }
            if (!cargado) item { Box(Modifier.fillMaxWidth().padding(Esp.xl), Alignment.Center) { CircularProgressIndicator() } }
            else if (filas.isEmpty()) item {
                EstadoVacio(Ico.Descargar, "Aún no hay descargas", "Cuando alguien pida «!descargar <enlace>» aparecerá aquí con su miniatura, quién la pidió y cuándo.")
            }
            items(filas, key = { it.id }) { d ->
                FilaDescargaVideo(botId, d, alRepetir = {
                    scope.launch { aviso = runCatching { Repo.videoRepetir(botId, d.id) }.getOrElse { it.message ?: "No se pudo volver a pedir" } }
                }, alBorrar = { borrando = d })
            }
            if (hayMas) item {
                TextButton({
                    scope.launch {
                        val mas = runCatching { Repo.videoLista(botId, filas.last().id) }.getOrDefault(emptyList())
                        filas.addAll(mas); hayMas = mas.size >= 40
                    }
                }, Modifier.fillMaxWidth()) { Text("Ver más") }
            }
        }
    }
    borrando?.let { d ->
        AlertDialog(
            onDismissRequest = { borrando = null }, modifier = Modifier.testTag("dialogo_borrar_descarga"),
            title = { Text("¿Quitar del historial?") }, text = { Text("«${d.titulo.ifBlank { "Esta descarga" }}» sale de la lista. El archivo ya no está en el teléfono.") },
            confirmButton = { TextButton({ borrando = null; scope.launch { runCatching { Repo.videoBorrar(botId, d.id) } } }, Modifier.testTag("confirmar_borrar_descarga")) { Text("Quitar") } },
            dismissButton = { TextButton({ borrando = null }) { Text("Cancelar") } },
        )
    }
    if (vaciando) {
        AlertDialog(
            onDismissRequest = { vaciando = false }, modifier = Modifier.testTag("dialogo_vaciar_descargas"),
            title = { Text("¿Vaciar el historial?") }, text = { Text("Se quitan todas las descargas terminadas de la lista. No se puede deshacer.") },
            confirmButton = { TextButton({ vaciando = false; scope.launch { runCatching { Repo.videoBorrar(botId, 0) } } }, Modifier.testTag("confirmar_vaciar_descargas")) { Text("Vaciar") } },
            dismissButton = { TextButton({ vaciando = false }) { Text("Cancelar") } },
        )
    }
}

@Composable
private fun FilaDescargaVideo(botId: Long, d: DescargaVideo, alRepetir: () -> Unit, alBorrar: () -> Unit) {
    val mini by produceState<androidx.compose.ui.graphics.ImageBitmap?>(null, d.id) {
        value = runCatching { Repo.videoMiniatura(botId, d.id)?.let { BitmapFactory.decodeByteArray(it, 0, it.size)?.asImageBitmap() } }.getOrNull()
    }
    Card(Modifier.fillMaxWidth().testTag("descarga_${d.id}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.m), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.m)) {
                Box(Modifier.width(112.dp).size(width = 112.dp, height = 68.dp).clip(RoundedCornerShape(10.dp)).background(MaterialTheme.colorScheme.surfaceContainerHighest), Alignment.Center) {
                    val m = mini
                    if (m != null) Image(m, null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
                    else Icon(if (d.formato == "mp4") Ico.Video else Ico.Audio, null, tint = MaterialTheme.colorScheme.onSurfaceVariant)
                    if (d.duracion > 0) Text(
                        duracionVideoTxt(d.duracion), Modifier.align(Alignment.BottomEnd).padding(4.dp).background(androidx.compose.ui.graphics.Color(0xAA000000), RoundedCornerShape(4.dp)).padding(horizontal = 4.dp),
                        style = MaterialTheme.typography.labelSmall, color = androidx.compose.ui.graphics.Color.White,
                    )
                }
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Text(d.titulo.ifBlank { d.url }, style = MaterialTheme.typography.titleSmall, maxLines = 2, overflow = TextOverflow.Ellipsis)
                    if (d.subidor.isNotBlank()) Text(d.subidor, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Text(
                        (d.nombre.ifBlank { "Alguien" }) + (if (d.esGrupo) " (en un grupo)" else "") + " · " + haceCuanto(d.ts),
                        style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis,
                    )
                }
            }
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.xs), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                SuggestionChip({}, { Text(d.formato.uppercase() + if (d.calidad.isNotBlank()) " · ${d.calidad}" else "") })
                if (d.peso > 0) SuggestionChip({}, { Text(pesoVideoTxt(d.peso)) })
                if (d.partes > 1) SuggestionChip({}, { Text("${d.partes} partes") })
                if (d.entrega == "zip") SuggestionChip({}, { Text("ZIP") })
                if (d.sitio.isNotBlank()) SuggestionChip({}, { Text(d.sitio) })
                SuggestionChip({}, {
                    Text(when (d.estado) { "enviado" -> "Enviado"; "fallido" -> "Falló"; else -> "Bajando…" })
                }, Modifier.testTag("estado_${d.id}"))
            }
            if (d.estado == "fallido" && d.error.isNotBlank()) Text(d.error, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                if (d.estado != "en_curso") OutlinedButton(alRepetir, Modifier.testTag("repetir_${d.id}")) {
                    Icon(Ico.Descargar, null, Modifier.size(18.dp)); Spacer(Modifier.width(Esp.xs)); Text("Volver a descargar")
                }
                if (d.estado != "en_curso") TextButton(alBorrar, Modifier.testTag("borrar_${d.id}")) { Text("Quitar") }
            }
        }
    }
}
