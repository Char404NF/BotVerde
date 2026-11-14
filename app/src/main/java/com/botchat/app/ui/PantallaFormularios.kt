@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import com.botchat.app.ui.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.FormularioRecibido
import com.botchat.app.datos.Repo
import com.botchat.app.medios.Exportar
import com.botchat.app.medios.Temporales
import kotlinx.coroutines.launch

private fun cuando(ts: Long) = etiquetaDia(ts) + " · " + horaDe(ts)

/** Formularios que llenó la gente dentro de los menús: lista, detalle, exportar y borrar. Todo vive cifrado en el teléfono. */
@Composable
fun PantallaFormularios(botId: Long, menuId: Long, titulo: String, alVolver: () -> Unit, alAbrir: (Long) -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("formularios" in evs) version++ } }
    val lista by produceState<List<FormularioRecibido>?>(null, version) { value = runCatching { Repo.formularios(botId, menuId) }.getOrDefault(emptyList()) }
    val hayDatos = !lista.isNullOrEmpty()
    var confirmaBorrar by remember { mutableStateOf(false) }
    var rutaExporte by remember { mutableStateOf<String?>(null) }
    var exportando by remember { mutableStateOf(false) }
    Scaffold(
        topBar = {
            TopAppBar(
                navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } },
                title = { Text(titulo.ifBlank { "Formularios recibidos" }) },
                actions = {
                    IconButton({
                        exportando = true
                        scope.launch { runCatching { Repo.exportarFormularios(botId, menuId) }.onSuccess { rutaExporte = it }.onFailure { avisaToast(ctx, it.message ?: "No se pudo exportar", true) }; exportando = false }
                    }, Modifier.testTag("exportar_formularios"), enabled = hayDatos && !exportando) { Icon(Ico.Compartir, "Exportar") }
                    IconButton({ confirmaBorrar = true }, Modifier.testTag("borrar_formularios"), enabled = hayDatos) { Icon(Ico.Borrar, "Borrar todos") }
                },
            )
        },
    ) { relleno ->
        val datos = lista
        when {
            datos == null -> Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }
            datos.isEmpty() -> EstadoVacio(Ico.Documento, "Sin formularios todavía", "Cuando alguien termine un formulario de un menú, lo verás aquí y te llegará un aviso.", Modifier.padding(relleno).padding(top = 60.dp))
            else -> LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("lista_formularios"), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.l, Esp.s, Esp.l, 32.dp), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
                item { Text("Los datos se guardan cifrados en este teléfono. Exportar crea un archivo cifrado que puedes compartir o guardar como hoja de cálculo.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                items(datos, key = { it.id }) { f ->
                    Card(Modifier.fillMaxWidth().clickable { alAbrir(f.id) }.testTag("formulario_${f.id}"), shape = MaterialTheme.shapes.large, colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                            Text(f.cliente.ifBlank { "Sin nombre" }, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            Text(f.menu + " · " + cuando(f.ts), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            f.campos.filter { it.valor.isNotBlank() }.take(2).forEach { Text("${it.etiqueta}: ${it.valor}", maxLines = 1, overflow = TextOverflow.Ellipsis, style = MaterialTheme.typography.bodyMedium) }
                        }
                    }
                }
            }
        }
    }
    if (confirmaBorrar) AlertDialog(
        onDismissRequest = { confirmaBorrar = false },
        title = { Text("¿Borrar todos los formularios?") },
        text = { Text("Se borran para siempre de este teléfono. Esto no se puede deshacer.") },
        confirmButton = { TextButton({ confirmaBorrar = false; scope.launch { runCatching { Repo.borrarFormularios(botId, menuId) }; version++ } }, Modifier.testTag("confirmar_borrar_formularios")) { Text("Borrar", color = MaterialTheme.colorScheme.error) } },
        dismissButton = { TextButton({ confirmaBorrar = false }) { Text("Cancelar") } },
    )
    rutaExporte?.let { ruta ->
        AlertDialog(
            onDismissRequest = { rutaExporte = null }, title = { Text("Exportar formularios") },
            text = {
                Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text("El archivo está cifrado con la llave de esta app y se borra solo en 24 horas.", style = MaterialTheme.typography.bodySmall, modifier = Modifier.testTag("exporte_formularios_listo"))
                    OutlinedButton({ scope.launch { compartirCifrado(ctx, ruta, "formularios") } }, Modifier.fillMaxWidth().testTag("compartir_formularios")) { Text("Compartir archivo cifrado") }
                    OutlinedButton({
                        scope.launch {
                            val r = Exportar.guardarCopia(ctx, Repo.archivo(ruta), "Exportes", Temporales.nombreSeguro("formularios") + ".csv", "text/csv")
                            avisaToast(ctx, r.fold({ "Copia guardada en $it. Esa copia ya NO está cifrada." }, { "No se pudo guardar la copia: ${it.message}" }), true)
                        }
                    }, Modifier.fillMaxWidth().testTag("guardar_copia_formularios")) { Text("Guardar hoja de cálculo (sin cifrar)") }
                }
            },
            confirmButton = { TextButton({ rutaExporte = null }, Modifier.testTag("cerrar_exporte_formularios")) { Text("Cerrar") } },
        )
    }
}

/** Un formulario recibido con todas sus respuestas. */
@Composable
fun PantallaFormulario(botId: Long, id: Long, alVolver: () -> Unit, alAbrirChat: (String) -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val f by produceState<FormularioRecibido?>(null, id) { value = runCatching { Repo.formulario(botId, id) }.getOrNull() }
    var confirma by remember { mutableStateOf(false) }
    Scaffold(
        topBar = {
            TopAppBar(
                navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } },
                title = { Text(f?.cliente?.ifBlank { null } ?: "Formulario") },
                actions = { IconButton({ confirma = true }, Modifier.testTag("borrar_formulario"), enabled = f != null) { Icon(Ico.Borrar, "Borrar") } },
            )
        },
    ) { relleno ->
        val d = f
        if (d == null) { Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }; return@Scaffold }
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Text(d.menu + " · " + cuando(d.ts), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.testTag("detalle_formulario_fecha"))
            Card(Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
                    d.campos.forEach { c ->
                        Column(Modifier.testTag("dato_${c.id}")) {
                            Text(c.etiqueta, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.primary)
                            Text(c.valor.ifBlank { "(sin contestar)" }, fontWeight = FontWeight.Medium)
                        }
                    }
                }
            }
            if (d.chat.isNotBlank()) OutlinedButton({ alAbrirChat(d.chat) }, Modifier.fillMaxWidth().testTag("abrir_chat_formulario")) { Icon(Ico.Chats, null); Text("  Abrir el chat") }
        }
    }
    if (confirma) AlertDialog(
        onDismissRequest = { confirma = false }, title = { Text("¿Borrar este formulario?") }, text = { Text("Se borra para siempre de este teléfono.") },
        confirmButton = { TextButton({ confirma = false; scope.launch { runCatching { Repo.borrarFormulario(botId, id) }.onFailure { avisaToast(ctx, it.message, true) }; alVolver() } }, Modifier.testTag("confirmar_borrar_formulario")) { Text("Borrar", color = MaterialTheme.colorScheme.error) } },
        dismissButton = { TextButton({ confirma = false }) { Text("Cancelar") } },
    )
}
