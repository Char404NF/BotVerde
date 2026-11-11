@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
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
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.EtiquetaItem
import com.botchat.app.datos.Repo
import kotlinx.coroutines.launch

/** Ajustes → Etiquetas: crea, renombra, cambia el color y borra las etiquetas locales para ordenar tus chats. */
@Composable
fun PantallaEtiquetas(botId: Long, alVolver: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("etiquetas" in evs || "chats" in evs) version++ } }
    val lista by produceState<List<EtiquetaItem>?>(null, version) { value = runCatching { Repo.etiquetas(botId) }.getOrDefault(emptyList()) }
    var editando by remember { mutableStateOf<EtiquetaItem?>(null) }
    var creando by remember { mutableStateOf(false) }
    var borrando by remember { mutableStateOf<EtiquetaItem?>(null) }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Etiquetas") }) }) { relleno ->
        val datos = lista
        LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("lista_etiquetas"), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            item {
                Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text("Las etiquetas son solo tuyas (no se mandan a WhatsApp). Ponlas a un chat desde su menú ⋮ y filtra la lista de chats por etiqueta.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    if (!creando && editando == null) Button({ haptica(Toque.Suave); creando = true }, Modifier.testTag("nueva_etiqueta")) { Text("Crear etiqueta") }
                }
            }
            if (creando || editando != null) item(key = "formulario") {
                val e = editando
                FormularioEtiqueta(e, { creando = false; editando = null }) { n, c ->
                    creando = false; editando = null
                    scope.launch {
                        runCatching { if (e == null) Repo.crearEtiqueta(botId, n, c) else Repo.editarEtiqueta(botId, e.id, n, c) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
                        version++
                    }
                }
            }
            when {
                datos == null -> item { Box(Modifier.fillMaxWidth().padding(Esp.xl), Alignment.Center) { CircularProgressIndicator() } }
                datos.isEmpty() -> item { EstadoVacio(Ico.Seleccionar, "Sin etiquetas", "Crea una, por ejemplo “Clientes” o “Proveedores”.", Modifier.padding(top = Esp.l).testTag("sin_etiquetas")) }
                else -> items(datos, key = { it.id }) { e ->
                    Card(Modifier.fillMaxWidth().testTag("etiqueta_fila_${e.id}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                        Row(Modifier.padding(horizontal = Esp.l, vertical = Esp.s), verticalAlignment = Alignment.CenterVertically) {
                            PuntoRango(e.color, 20)
                            Column(Modifier.padding(start = Esp.m).weight(1f)) {
                                Text(e.nombre, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                Text(if (e.chats == 0) "Sin chats" else plural(e.chats.toLong(), "chat", "chats"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                            IconButton({ haptica(Toque.Suave); editando = e }, Modifier.testTag("editar_etiqueta_${e.id}")) { Icon(Ico.Editar, "Cambiar nombre o color") }
                            IconButton({ haptica(Toque.Fuerte); borrando = e }, Modifier.testTag("borrar_etiqueta_${e.id}")) { Icon(Ico.Borrar, "Borrar la etiqueta") }
                        }
                    }
                }
            }
        }
    }
    borrando?.let { e ->
        AlertDialog(
            onDismissRequest = { borrando = null }, title = { Text("¿Borrar “${e.nombre}”?") },
            text = { Text(if (e.chats > 0) "Se quita de ${plural(e.chats.toLong(), "chat", "chats")}. Los chats y sus mensajes no se tocan." else "No la usa ningún chat.") },
            confirmButton = { TextButton({ borrando = null; scope.launch { runCatching { Repo.borrarEtiqueta(botId, e.id) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }; version++ } }, Modifier.testTag("confirmar_borrar_etiqueta")) { Text("Borrar", color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton({ borrando = null }) { Text("Cancelar") } },
        )
    }
}

/** Nombre y color de una etiqueta (nueva si `actual` es null), en la misma pantalla (sin ventana aparte, para que el teclado no estorbe). */
@Composable
fun FormularioEtiqueta(actual: EtiquetaItem?, alCerrar: () -> Unit, alGuardar: (nombre: String, color: String) -> Unit) {
    var nombre by remember(actual) { mutableStateOf(actual?.nombre.orEmpty()) }
    var color by remember(actual) { mutableStateOf(actual?.color ?: COLORES_RANGO.first()) }
    Card(Modifier.fillMaxWidth().testTag("formulario_etiqueta"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerHigh)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Text(if (actual == null) "Nueva etiqueta" else "Cambiar etiqueta", style = MaterialTheme.typography.titleSmall)
            OutlinedTextField(nombre, { nombre = it.take(24) }, Modifier.fillMaxWidth().testTag("nombre_etiqueta"), label = { Text("Nombre") }, singleLine = true)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                COLORES_RANGO.forEach { c ->
                    Box(
                        Modifier.size(36.dp).clip(CircleShape).background(colorDeRango(c))
                            .then(if (c == color) Modifier.border(3.dp, MaterialTheme.colorScheme.onSurface, CircleShape) else Modifier)
                            .clickable { color = c }.testTag("color_etiqueta_${c.removePrefix("#")}"),
                    )
                }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                Button({ alGuardar(nombre.trim(), color) }, Modifier.testTag("guardar_etiqueta"), enabled = nombre.isNotBlank()) { Text("Guardar") }
                TextButton(alCerrar, Modifier.testTag("cancelar_etiqueta")) { Text("Cancelar") }
            }
        }
    }
}

/** Elige las etiquetas de un chat (marcar y desmarcar). Se puede crear una nueva ahí mismo. */
@Composable
fun DialogoEtiquetasChat(botId: Long, chat: String, alCerrar: () -> Unit, alCrearEtiqueta: () -> Unit = {}) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var version by remember { mutableIntStateOf(0) }
    val todas by produceState<List<EtiquetaItem>?>(null, version) { value = runCatching { Repo.etiquetas(botId) }.getOrDefault(emptyList()) }
    val puestas by produceState<Set<Long>>(emptySet(), version) { value = runCatching { Repo.etiquetasDeChat(botId, chat).map { it.id }.toSet() }.getOrDefault(emptySet()) }
    fun hacer(f: suspend () -> Unit) = scope.launch { runCatching { f() }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }; version++ }
    AlertDialog(
        onDismissRequest = alCerrar, title = { Text("Etiquetas de este chat") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                val t = todas
                when {
                    t == null -> CircularProgressIndicator()
                    t.isEmpty() -> Text("Todavía no hay etiquetas. Crea la primera.", Modifier.testTag("sin_etiquetas_chat"))
                    else -> t.forEach { e ->
                        Row(Modifier.fillMaxWidth().clickable { hacer { Repo.ponerEtiquetaChat(botId, chat, e.id, e.id !in puestas) } }.testTag("elegir_etiqueta_${e.id}"), verticalAlignment = Alignment.CenterVertically) {
                            Checkbox(e.id in puestas, { hacer { Repo.ponerEtiquetaChat(botId, chat, e.id, it) } })
                            PuntoRango(e.color, 14)
                            Spacer(Modifier.width(Esp.s))
                            Text(e.nombre, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        }
                    }
                }
                TextButton({ alCerrar(); alCrearEtiqueta() }, Modifier.testTag("crear_etiqueta_aqui")) { Text("Crear o cambiar etiquetas") }
            }
        },
        confirmButton = { TextButton(alCerrar, Modifier.testTag("cerrar_etiquetas_chat")) { Text("Listo") } },
    )
}
