@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import com.botchat.app.ui.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import com.botchat.app.ui.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Switch
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.AuditoriaChatItem
import com.botchat.app.datos.AuditoriaGlobal
import com.botchat.app.datos.Repo
import com.botchat.app.datos.auditoriaGlobal
import com.botchat.app.datos.borrarTodoDePersona
import com.botchat.app.datos.chatsAuditoria
import com.botchat.app.datos.fijarAuditoria
import com.botchat.app.datos.fijarAuditoriaChat
import com.botchat.app.datos.fijarAvisoAuditoriaVisto
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/** Lo que guarda la auditoría, dicho en palabras simples. Se usa en el aviso de la primera vez y en la pantalla. */
const val TEXTO_AUDITORIA =
    "Bot Chat guarda el texto de tus mensajes, cifrado, solo en este teléfono. Solo guarda lo que llega desde que lo activas: " +
        "no se recupera el historial anterior. Solo tú puedes verlo (con tu huella o tu código). Nada sale del teléfono."

const val TEXTO_REGLAS_PRIVACIDAD =
    "Qué cambió: ahora puedes apagar el guardado del texto por chat o grupo y elegir cuánto tiempo se conserva. " +
        "Lo que se apaga queda como «contenido no guardado»; las estadísticas no cambian."

private fun textoRetencion(d: Int) = when (d) { 0 -> "Siempre"; 30 -> "30 días"; 90 -> "90 días"; else -> "$d días" }

/** Aviso que se muestra una sola vez al actualizar. Si falta elegir la retención, se pide aquí y NO se borra nada hasta elegir. */
@Composable
fun AvisoAuditoria(retPendiente: Boolean, alElegir: (Int) -> Unit, alEntendido: () -> Unit) {
    AlertDialog(
        onDismissRequest = {}, modifier = Modifier.testTag("aviso_auditoria"),
        title = { Text(if (retPendiente) "¿Cuánto tiempo guardo tus mensajes?" else "Así cuido tu privacidad") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text(TEXTO_AUDITORIA)
                Text(TEXTO_REGLAS_PRIVACIDAD, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                if (retPendiente) {
                    Text("Elige una opción. Hasta que elijas, no se borra nada.", color = AMARILLO_SILENCIO)
                    Button({ alElegir(30) }, Modifier.fillMaxWidth().testTag("aviso_ret_30")) { Text("30 días") }
                    Button({ alElegir(90) }, Modifier.fillMaxWidth().testTag("aviso_ret_90")) { Text("90 días") }
                    Button({ alElegir(0) }, Modifier.fillMaxWidth().testTag("aviso_ret_0")) { Text("Siempre") }
                    Text("Pasado ese tiempo, el texto viejo se olvida (queda «contenido no guardado»); la hora y las estadísticas se quedan.", style = MaterialTheme.typography.bodySmall)
                }
            }
        },
        confirmButton = { if (!retPendiente) TextButton(alEntendido, Modifier.testTag("aviso_auditoria_ok")) { Text("Entendido") } },
    )
}

/** Bot → Auditoría y privacidad. */
@Composable
fun PantallaAuditoria(botId: Long, alVolver: () -> Unit, alPrivacidad: () -> Unit = {}) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    var consulta by remember { mutableStateOf("") }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("auditoria" in evs || "chats" in evs) version++ } }
    val global by produceState<AuditoriaGlobal?>(null, version) { value = runCatching { Repo.auditoriaGlobal(botId) }.getOrNull() }
    val chats by produceState<List<AuditoriaChatItem>?>(null, consulta, version) {
        if (consulta.isNotBlank()) delay(200)
        value = runCatching { Repo.chatsAuditoria(botId, consulta) }.getOrDefault(emptyList())
    }
    var borrandoPersona by remember { mutableStateOf(false) }
    var confirmar by remember { mutableStateOf<com.botchat.app.datos.ChatItem?>(null) }
    fun hacer(f: suspend () -> Unit) = scope.launch { runCatching { f() }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }; version++ }

    Box(Modifier.fillMaxSize()) {
        Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver, Modifier.testTag("volver_auditoria")) { Icon(Ico.Atras, "Volver") } }, title = { Text("Auditoría y privacidad") }) }) { relleno ->
            LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("pantalla_auditoria"), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
                val g = global
                item { CajaAviso(MaterialTheme.colorScheme.primary, "Qué se guarda", TEXTO_AUDITORIA, "texto_auditoria", icono = Ico.Candado) }
                item { Text(TEXTO_REGLAS_PRIVACIDAD, Modifier.testTag("texto_reglas_privacidad"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                if (g != null) {
                    if (g.retPendiente) item {
                        CajaAviso(AMARILLO_SILENCIO, "Falta elegir cuánto guardar", "Mientras no elijas abajo, no se borra ningún mensaje.", "aviso_ret_pendiente")
                    }
                    item {
                        Card(Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                            Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                                Row(verticalAlignment = Alignment.CenterVertically) {
                                    Column(Modifier.weight(1f)) {
                                        Text("Guardar el texto de los mensajes", style = MaterialTheme.typography.titleSmall)
                                        Text(if (g.defecto) "Encendido en todos los chats (salvo los que apagues abajo)." else "Apagado: lo nuevo se guarda sin texto.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                    }
                                    Switch(g.defecto, { haptica(Toque.Suave); hacer { Repo.fijarAuditoria(botId, it, -1) } }, Modifier.testTag("sw_auditoria_global"))
                                }
                                Text("¿Cuánto tiempo lo guardo?", style = MaterialTheme.typography.titleSmall)
                                androidx.compose.foundation.layout.FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                                    listOf(30, 90, 0).forEach { d ->
                                        FilterChip(!g.retPendiente && g.retDias == d, { haptica(Toque.Suave); hacer { Repo.fijarAuditoria(botId, g.defecto, d); Repo.fijarAvisoAuditoriaVisto(botId, true) } }, { Text(textoRetencion(d)) }, modifier = Modifier.testTag("ret_$d"))
                                    }
                                }
                                Text("Pasado ese tiempo el texto se olvida; la hora y las estadísticas se quedan.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                        }
                    }
                }
                item {
                    Card(Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                            Text("Borrar todo de una persona", style = MaterialTheme.typography.titleSmall)
                            Text("Quita sus mensajes, archivos, notas y advertencias de este teléfono. No toca WhatsApp.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            OutlinedButton({ borrandoPersona = true }, Modifier.fillMaxWidth().testTag("borrar_todo_persona")) { Icon(Ico.Borrar, null, Modifier.size(18.dp)); Spacer(Modifier.width(8.dp)); Text("Elegir persona") }
                        }
                    }
                }
                item { OutlinedButton(alPrivacidad, Modifier.fillMaxWidth().testTag("ir_mensajes_eliminados")) { Text("Mensajes eliminados por otros") } }
                item {
                    Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                        Text("Chats y grupos", style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.primary)
                        Text("Apaga el guardado de un chat y lo nuevo se guardará sin texto. Lo que ya está guardado se queda.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        OutlinedTextField(consulta, { consulta = it }, Modifier.fillMaxWidth().testTag("auditoria_buscar"), singleLine = true, placeholder = { Text("Buscar un chat o grupo") }, leadingIcon = { Icon(Ico.Buscar, null) })
                    }
                }
                val lista = chats
                when {
                    lista == null -> item { FilasDeCarga(3) }
                    lista.isEmpty() -> item { Text("No hay chats.", Modifier.testTag("auditoria_sin_chats"), color = MaterialTheme.colorScheme.onSurfaceVariant) }
                    else -> items(lista, key = { it.chat }) { c -> FilaAuditoriaChat(c, global, { a, r -> hacer { Repo.fijarAuditoriaChat(botId, c.chat, a, r) } }) }
                }
            }
        }
        if (borrandoPersona) HojaElegirChat(botId, false, "¿De quién borro todo?", "borrar_persona", { borrandoPersona = false }) { c -> borrandoPersona = false; confirmar = c }
    }
    confirmar?.let { c ->
        val nombre = c.nombre.ifBlank { c.telefono }
        ConfirmarSinDeshacer(
            "¿Borrar todo de $nombre?", "Se borran sus mensajes, archivos, notas y advertencias de este teléfono.", "Borrar todo", "confirmar_borrar_todo", { confirmar = null },
        ) {
            confirmar = null
            hacer { Repo.borrarTodoDePersona(botId, c.jid); avisaToast(ctx, "Listo: se borró todo de $nombre") }
        }
    }
}

@Composable
private fun FilaAuditoriaChat(c: AuditoriaChatItem, g: AuditoriaGlobal?, alCambiar: (audit: Int, ret: Int) -> Unit) {
    var menu by remember { mutableStateOf(false) }
    val haptica = rememberHaptica()
    Card(Modifier.fillMaxWidth().testTag("auditoria_chat_${c.chat}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Row(Modifier.padding(horizontal = Esp.l, vertical = Esp.s), verticalAlignment = Alignment.CenterVertically) {
            Icon(if (c.esGrupo) Ico.Grupo else Ico.Persona, null, Modifier.size(24.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.width(Esp.m))
            Column(Modifier.weight(1f)) {
                Text(c.nombre.ifBlank { c.chat }, style = MaterialTheme.typography.titleSmall, maxLines = 1, overflow = TextOverflow.Ellipsis)
                Text(
                    (if (c.audit == -1) "Como el general" else if (c.efectivo) "Guardo el texto" else "No guardo el texto") + " · " + (if (c.retDias == -1) "retención general" else textoRetencion(c.retDias)),
                    Modifier.testTag("auditoria_estado_${c.chat}"), style = MaterialTheme.typography.bodySmall, color = if (!c.efectivo) AMARILLO_SILENCIO else MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            Box {
                TextButton({ menu = true }, Modifier.testTag("auditoria_ret_${c.chat}")) { Text("Tiempo") }
                DropdownMenu(menu, { menu = false }) {
                    listOf(-1 to "Como el general", 30 to "30 días", 90 to "90 días", 0 to "Siempre").forEach { (d, t) ->
                        DropdownMenuItem(text = { Text(t) }, onClick = { menu = false; alCambiar(c.audit, d) }, modifier = Modifier.testTag("auditoria_ret_${c.chat}_$d"))
                    }
                }
            }
            Switch(c.efectivo, { haptica(Toque.Suave); alCambiar(if (it) 1 else 0, c.retDias) }, Modifier.testTag("sw_auditoria_${c.chat}"))
        }
    }
}
