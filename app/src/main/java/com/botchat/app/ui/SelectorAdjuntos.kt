@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.size
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.AdjuntoSubido
import com.botchat.app.datos.Repo
import com.botchat.app.medios.GrabadorVoz
import com.botchat.app.medios.Grabadores
import com.botchat.app.medios.PrepararAdjunto
import kotlinx.coroutines.launch

/**
 * Archivos de un mensaje: hasta 3 (foto, video, documento con nombre o nota de voz). Es el único lugar de la app que sube archivos para
 * mensajes automáticos, opciones de menú y comandos propios: se guardan cifrados en el teléfono (Repo.subirAdjunto) y en cada envío se
 * suben de nuevo a WhatsApp. Las notas de voz se graban aquí mismo o se eligen de un .ogg Opus mono (Go lo valida leyendo la cabecera).
 *
 * Al quitar un archivo se pide borrarlo del teléfono; Go solo lo borra si ningún mensaje guardado lo usa todavía (si ya estaba guardado,
 * se borra al guardar el mensaje sin él).
 */
@Composable
fun SelectorAdjuntos(
    botId: Long, adjuntos: List<AdjuntoSubido>, prefijo: String, alCambiar: (List<AdjuntoSubido>) -> Unit,
    max: Int = 3, permitirVoz: Boolean = true,
) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var ocupado by remember { mutableStateOf(false) }
    var grabador by remember { mutableStateOf<GrabadorVoz?>(null) }

    fun sube(tipo: String, ruta: String, mime: String, nombre: String) {
        ocupado = true
        scope.launch {
            runCatching { Repo.subirAdjunto(botId, ruta, tipo, mime, nombre) }
                .onSuccess { a -> alCambiar((adjuntos + a).take(max)) }
                .onFailure { avisaToast(ctx, it.message ?: "No se pudo guardar el archivo", true) }
            ocupado = false
        }
    }

    val elegir = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        if (uri != null) {
            ocupado = true
            scope.launch {
                PrepararAdjunto.desdeUri(ctx, uri).onSuccess { listo ->
                    runCatching { Repo.subirAdjunto(botId, listo.archivo.path, listo.tipo, listo.mime, listo.nombre) }
                        .onSuccess { a -> alCambiar((adjuntos + a).take(max)) }
                        .onFailure { avisaToast(ctx, it.message ?: "No se pudo guardar el archivo", true) }
                }.onFailure { avisaToast(ctx, it.message ?: "No se pudo leer el archivo", true) }
                ocupado = false
            }
        }
    }
    val permisoMicrofono = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { ok ->
        if (!ok) avisaToast(ctx, "Sin permiso del micrófono no se puede grabar.", true)
    }

    fun alternaGrabacion() {
        val g = grabador
        if (g == null) {
            if (androidx.core.content.ContextCompat.checkSelfPermission(ctx, android.Manifest.permission.RECORD_AUDIO) != android.content.pm.PackageManager.PERMISSION_GRANTED) {
                permisoMicrofono.launch(android.Manifest.permission.RECORD_AUDIO)
                return
            }
            val nuevo = Grabadores.fabrica(ctx)
            if (nuevo.iniciar()) grabador = nuevo else avisaToast(ctx, "No se pudo empezar a grabar.", true)
        } else {
            grabador = null
            val r = g.terminar()
            when {
                r == null -> avisaToast(ctx, "La grabación fue muy corta.", true)
                !r.esNota -> { r.archivo.delete(); avisaToast(ctx, "Este teléfono no puede grabar notas de voz (hace falta Android 10 o más). Elige un archivo .ogg Opus mono.", true) }
                else -> sube("audio", r.archivo.path, r.mime, "nota de voz.ogg")
            }
        }
    }

    Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
        Text("Archivos (hasta $max)", style = MaterialTheme.typography.labelLarge)
        adjuntos.forEachIndexed { i, a ->
            Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.testTag("adjunto_${prefijo}_$i")) {
                Icon(when (a.tipo) { "imagen" -> Ico.Foto; "video" -> Ico.Video; "audio" -> Ico.Audio; else -> Ico.Documento }, null, Modifier.size(20.dp))
                Text("  " + (if (a.tipo == "audio") "Nota de voz" else a.nombre.ifBlank { a.tipo }), Modifier.weight(1f), maxLines = 1)
                IconButton({
                    alCambiar(adjuntos.filterIndexed { j, _ -> j != i })
                    scope.launch { runCatching { Repo.borrarAdjunto(botId, a.id) } }
                }, Modifier.testTag("quitar_adjunto_${prefijo}_$i")) { Icon(Ico.Borrar, "Quitar archivo") }
            }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
            OutlinedButton({ elegir.launch(arrayOf("*/*")) }, Modifier.testTag("agregar_adjunto_$prefijo"), enabled = adjuntos.size < max && !ocupado && grabador == null) {
                Icon(Ico.Adjuntar, null); Text(if (ocupado) " Guardando…" else " Agregar archivo")
            }
            if (permitirVoz) OutlinedButton({ alternaGrabacion() }, Modifier.testTag("grabar_voz_$prefijo"), enabled = adjuntos.size < max && !ocupado) {
                Icon(Ico.Microfono, null); Text(if (grabador != null) " Detener y guardar" else " Grabar nota de voz")
            }
        }
        Text(
            "Foto, video, documento (con su nombre) o nota de voz. Fotos y notas de voz de hasta 8 MB; videos y documentos de hasta 16 MB. La nota de voz se graba aquí " +
                "o se elige un .ogg con códec Opus en mono. Los archivos se guardan cifrados en el teléfono y se suben a WhatsApp en cada envío; salen con 1 a 3 segundos entre uno y otro. " +
                "Si un archivo falla, se manda solo el texto y se anota en el registro.",
            style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}
