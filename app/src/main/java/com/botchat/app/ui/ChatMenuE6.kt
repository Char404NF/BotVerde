package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
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
import com.botchat.app.datos.Repo
import com.botchat.app.medios.Exportar
import com.botchat.app.medios.Temporales
import kotlinx.coroutines.launch

/** Menú ⋮ del chat → «Exportar chat»: se elige texto o datos, se prepara el archivo cifrado y se comparte (o se guarda una copia legible). */
@Composable
fun DialogoExportarChat(botId: Long, jid: String, nombre: String, alCerrar: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var formato by remember { mutableStateOf<String?>(null) }
    var ruta by remember { mutableStateOf<String?>(null) }
    var cuantos by remember { mutableStateOf(0L) }
    var error by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(formato) {
        val f = formato ?: return@LaunchedEffect
        runCatching { Repo.exportarChat(botId, jid, f) }.onSuccess { ruta = it.ruta; cuantos = it.mensajes }.onFailure { error = it.message ?: "No se pudo exportar" }
    }
    AlertDialog(
        onDismissRequest = alCerrar, title = { Text("Exportar este chat") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.m)) {
                when {
                    error != null -> Text(error!!, color = MaterialTheme.colorScheme.error, modifier = Modifier.testTag("error_exportar_chat"))
                    formato == null -> {
                        Text("Se crea un archivo con los mensajes de este chat. Los archivos (fotos, audios…) no van dentro. Va cifrado con la llave de esta app y se borra solo en 24 horas.", style = MaterialTheme.typography.bodyMedium)
                        Button({ formato = "txt" }, Modifier.fillMaxWidth().testTag("exportar_txt")) { Text("Como texto (.txt)") }
                        OutlinedButton({ formato = "json" }, Modifier.fillMaxWidth().testTag("exportar_json")) { Text("Como datos (.json)") }
                    }
                    ruta == null -> Row(verticalAlignment = Alignment.CenterVertically) { CircularProgressIndicator(Modifier.size(24.dp)); Spacer(Modifier.width(Esp.m)); Text("Preparando el archivo…") }
                    else -> {
                        Text("Listo: $cuantos mensajes.", Modifier.testTag("exportar_chat_listo"))
                        Button({ scope.launch { compartirCifrado(ctx, ruta!!, nombre, "bcf") } }, Modifier.fillMaxWidth().testTag("compartir_chat_cifrado")) { Text("Compartir archivo cifrado") }
                        OutlinedButton({
                            scope.launch {
                                val ext = formato ?: "txt"
                                val r = Exportar.guardarCopia(ctx, Repo.archivo(ruta!!), "Exportes", Temporales.nombreSeguro("chat_$nombre") + "." + ext, if (ext == "json") "application/json" else "text/plain")
                                avisaToast(ctx, r.fold({ "Copia guardada en $it. Esa copia ya NO está cifrada." }, { "No se pudo guardar la copia: ${it.message}" }), true)
                            }
                        }, Modifier.fillMaxWidth().testTag("guardar_chat_legible")) { Text("Guardar copia legible (sin cifrar)") }
                    }
                }
            }
        },
        confirmButton = { TextButton(alCerrar, Modifier.testTag("cerrar_exportar_chat")) { Text("Cerrar") } },
    )
}

/** Menú ⋮ del chat → «Vaciar chat». No hay «Deshacer»: por eso se pide confirmar. */
@Composable
fun DialogoVaciarChat(nombre: String, alCerrar: () -> Unit, alConfirmar: () -> Unit) {
    AlertDialog(
        onDismissRequest = alCerrar, title = { Text("¿Vaciar el chat con $nombre?") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("Se borran los mensajes y los archivos de este chat de tu teléfono. El chat se queda, con sus etiquetas y ajustes.", Modifier.testTag("texto_vaciar"))
                Text("No se puede deshacer: no hay «Deshacer». No se borra nada en WhatsApp.", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.error, modifier = Modifier.testTag("aviso_sin_deshacer"))
            }
        },
        confirmButton = { TextButton(alConfirmar, Modifier.testTag("confirmar_vaciar")) { Text("Vaciar chat", color = MaterialTheme.colorScheme.error) } },
        dismissButton = { TextButton(alCerrar, Modifier.testTag("cancelar_vaciar")) { Text("Cancelar") } },
    )
}
