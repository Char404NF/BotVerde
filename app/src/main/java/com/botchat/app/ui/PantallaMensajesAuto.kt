@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import com.botchat.app.ui.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
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
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.foundation.text.KeyboardOptions
import com.botchat.app.datos.ListaAuto
import com.botchat.app.datos.MensajeAuto
import com.botchat.app.datos.Msg
import com.botchat.app.datos.RegistroAuto
import com.botchat.app.datos.Repo
import com.botchat.app.datos.TipoAuto
import kotlinx.coroutines.launch

/** Automatizaciones → Mensajes automáticos: la lista por tipo con interruptores y estado, y los ajustes generales. */
@Composable
fun PantallaMensajesAuto(botId: Long, alVolver: () -> Unit, alEditar: (Long, String) -> Unit, alRegistro: () -> Unit = {}, alVerAutomaticos: () -> Unit = {}) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("auto" in evs || "auto_registro" in evs || "automatizaciones" in evs) version++ } }
    val datos by produceState<ListaAuto?>(null, version) { value = runCatching { Repo.listaAuto(botId) }.getOrNull() }
    var borrando by remember { mutableStateOf<MensajeAuto?>(null) }
    var tope by remember { mutableStateOf<String?>(null) }

    fun ajusta(clave: String, valor: String) {
        scope.launch { runCatching { Repo.fijarAjustePermisos(botId, clave, valor) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo cambiar", true) }; version++ }
    }

    Scaffold(topBar = {
        TopAppBar(
            navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Mensajes automáticos") },
            actions = {
                TextButton(alVerAutomaticos, Modifier.testTag("ver_automaticos")) { Text("Enviados") }
                TextButton(alRegistro, Modifier.testTag("abrir_registro_auto")) { Text("Registro") }
            },
        )
    }) { relleno ->
        val d = datos
        if (d == null) { Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }; return@Scaffold }
        LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("lista_mensajes_auto"), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            item {
                Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text("Son respuestas a un mensaje que alguien te escribió: nunca escriben primero. Si contestas tú antes de que salgan, se cancelan.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) { Text("Mensajes automáticos"); Text("Bienvenida, ausencia, recibido y cierre", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                        Switch(d.activo, { haptica(Toque.Suave); ajusta("auto_activo", if (it) "1" else "0") }, Modifier.testTag("auto_general"))
                    }
                    d.conflictos.forEach { c ->
                        Card(Modifier.fillMaxWidth().testTag("conflicto_auto"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.errorContainer)) {
                            Text("La regla “$c” contesta a cualquier mensaje y tiene prioridad: mientras esté encendida, los mensajes automáticos de abajo no salen. Apágala en Automatizaciones si quieres usarlos.", Modifier.padding(Esp.m), style = MaterialTheme.typography.bodySmall)
                        }
                    }
                    Text("Prioridad: menú en curso, luego regla, luego bienvenida y ausencia, y al final “recibido”. El cierre sale cuando termina un menú.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
            TipoAuto.todos.forEach { tipo ->
                val deTipo = d.mensajes.filter { it.tipo == tipo }
                item(key = "t_$tipo") {
                    Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(TipoAuto.titulo(tipo), Modifier.weight(1f), style = MaterialTheme.typography.titleMedium)
                            OutlinedButton({ haptica(Toque.Suave); alEditar(0, tipo) }, Modifier.testTag("nuevo_$tipo")) { Icon(Ico.Agregar, null); Text(" Agregar") }
                        }
                        Text(TipoAuto.descripcion(tipo), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        if (deTipo.isEmpty()) Text("Todavía no hay ninguno.", Modifier.testTag("vacio_$tipo"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }
                items(deTipo, key = { it.id }) { a ->
                    Card(onClick = { alEditar(a.id, a.tipo) }, modifier = Modifier.fillMaxWidth().testTag("mensaje_auto_${a.id}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                        Row(Modifier.padding(horizontal = Esp.l, vertical = Esp.s), verticalAlignment = Alignment.CenterVertically) {
                            Column(Modifier.weight(1f)) {
                                Text(if (tipo == TipoAuto.ATAJO) "/" + a.nombre else a.nombre.ifBlank { TipoAuto.titulo(tipo) }, style = MaterialTheme.typography.titleSmall, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                Text(a.texto.replace('\n', ' '), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 2, overflow = TextOverflow.Ellipsis)
                                if (tipo != TipoAuto.ATAJO) Text("${a.cuando} · ${a.quien}", Modifier.testTag("resumen_auto_${a.id}"), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 2, overflow = TextOverflow.Ellipsis)
                                Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                                    Text(estadoTexto(a), Modifier.testTag("estado_auto_${a.id}"), style = MaterialTheme.typography.labelSmall, color = if (a.estado == "sin_archivo") MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.primary)
                                    if (a.adjuntos.isNotEmpty()) Text("📎 ${a.adjuntos.size}", style = MaterialTheme.typography.labelSmall)
                                    if (a.envios7d > 0) Text("${a.envios7d} en 7 días", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                }
                            }
                            Switch(a.activo, { si -> haptica(Toque.Suave); scope.launch { runCatching { Repo.fijarAutoActivo(botId, a.id, si) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo cambiar", true) }; version++ } }, Modifier.testTag("auto_activo_${a.id}"))
                            IconButton({ haptica(Toque.Fuerte); borrando = a }, Modifier.testTag("borrar_auto_${a.id}")) { Icon(Ico.Borrar, "Borrar") }
                        }
                    }
                }
            }
            item {
                Card(Modifier.fillMaxWidth().testTag("ajustes_auto"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                    Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
                        Text("Límites y ajustes", style = MaterialTheme.typography.titleMedium)
                        val topeTxt = tope ?: d.topeHora.toString()
                        OutlinedTextField(topeTxt, { t -> tope = t.filter(Char::isDigit).take(4); if (!tope.isNullOrEmpty()) ajusta("auto_tope_hora", tope!!) }, Modifier.fillMaxWidth().testTag("auto_tope"),
                            label = { Text("Tope de mensajes automáticos por hora (0 = sin tope)") }, singleLine = true, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                            supportingText = { Text("Esta hora van ${d.usadosHora}. Pasado el tope, los mensajes salen solo con texto, sin archivos; muy por encima del tope ya no sale ninguno. Con «Sin límites» encendido (de fábrica) este tope no se aplica.") })
                        Text("Zona horaria: ${if (d.zona.isBlank()) "la del bot (${d.zonaBot})" else d.zona}. Cada mensaje puede traer la suya.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Column(Modifier.weight(1f)) { Text("Respuestas rápidas (/atajo)"); Text("Si lo apagas, escribir /atajo no hace nada.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                            Switch(d.atajosActivo, { ajusta("atajos_activo", if (it) "1" else "0") }, Modifier.testTag("atajos_general"))
                        }
                        Text("Cuando escribo /atajo desde el teléfono", style = MaterialTheme.typography.labelLarge)
                        FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                            FilterChip(d.atajoModo != "enviar", { ajusta("atajo_modo", "editar") }, { Text("Cambiar mi mensaje por el texto") }, Modifier.testTag("atajo_modo_editar"))
                            FilterChip(d.atajoModo == "enviar", { ajusta("atajo_modo", "enviar") }, { Text("Mandar el texto aparte") }, Modifier.testTag("atajo_modo_enviar"))
                        }
                        Text("Cambiar mi mensaje deja el chat limpio (el cliente ve el texto con la marca “editado”). Si WhatsApp ya no deja editarlo, se manda aparte.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }
            }
        }
    }
    borrando?.let { a ->
        AlertDialog(
            onDismissRequest = { borrando = null }, title = { Text("¿Borrar “${a.nombre.ifBlank { TipoAuto.titulo(a.tipo) }}”?") },
            text = { Text("También se borran sus archivos guardados. No se puede deshacer.") },
            confirmButton = { TextButton({ borrando = null; scope.launch { runCatching { Repo.borrarAuto(botId, a.id) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo borrar", true) }; version++ } }, Modifier.testTag("confirmar_borrar_auto_msg")) { Text("Borrar", color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton({ borrando = null }) { Text("Cancelar") } },
        )
    }
}

private fun estadoTexto(a: MensajeAuto) = when (a.estado) {
    "apagado" -> "Apagado"
    "sin_archivo" -> "Falta un archivo guardado"
    else -> "Activo"
}

/** El registro de envíos automáticos: cuándo salió cada uno y si alguno de sus archivos falló. Sin textos ni nombres. */
@Composable
fun PantallaRegistroAuto(botId: Long, alVolver: () -> Unit) {
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("auto_registro" in evs) version++ } }
    val lista by produceState<List<RegistroAuto>?>(null, version) { value = runCatching { Repo.registroAuto(botId) }.getOrDefault(emptyList()) }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Registro de mensajes automáticos") }) }) { relleno ->
        val datos = lista
        when {
            datos == null -> Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }
            datos.isEmpty() -> EstadoVacio(Ico.Chats, "Sin envíos", "Cuando salga un mensaje automático lo verás aquí.", Modifier.padding(relleno).padding(top = 60.dp).testTag("sin_registro_auto"))
            else -> LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("lista_registro_auto")) {
                item { Text("Sin textos ni nombres: solo qué salió, cuándo y si algún archivo falló.", Modifier.padding(Esp.l), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                items(datos) { r ->
                    Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s)) {
                        Text("${etiquetaDia(r.ts)} ${horaDe(r.ts)}", Modifier.width(96.dp), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        Column(Modifier.weight(1f)) {
                            Text(TipoAuto.titulo(r.tipo) + " · " + r.texto, style = MaterialTheme.typography.bodyMedium)
                            val extra = buildList {
                                if (r.adjOk > 0) add("${r.adjOk} archivo(s) salieron")
                                if (r.adjFallo > 0) add("${r.adjFallo} no salieron")
                                if (r.motivo.isNotBlank()) add(motivoAuto(r.motivo))
                                if (r.chat.isNotBlank()) add("Chat ${r.chat}")
                            }
                            if (extra.isNotEmpty()) Text(extra.joinToString(" · "), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                    HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                }
            }
        }
    }
}

fun motivoAuto(m: String) = when (m) {
    "archivo_ilegible" -> "no se pudo leer el archivo guardado"
    "subida_fallo" -> "falló la subida a WhatsApp"
    "archivo_fallo" -> "falló el envío del archivo"
    "texto_fallo" -> "no se pudo mandar el texto"
    "tope_por_hora" -> "se pasó el tope por hora"
    else -> m
}

/** Los mensajes que mandó una automatización (de un chat, o de todos si `chat` está vacío), para verlos aparte de lo que escribí yo. */
@Composable
fun PantallaMensajesAutomaticos(botId: Long, chat: String, titulo: String, alVolver: () -> Unit, alAbrirChat: (String) -> Unit = {}) {
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if (evs.any { it.startsWith("msg:") || it == "auto_registro" }) version++ } }
    val lista by produceState<List<Msg>?>(null, version) { value = runCatching { Repo.mensajesAuto(botId, chat) }.getOrDefault(emptyList()) }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text(titulo) }) }) { relleno ->
        val datos = lista
        when {
            datos == null -> Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }
            datos.isEmpty() -> EstadoVacio(Ico.Chats, "Nada automático todavía", "Aquí aparecen los mensajes que mandó una bienvenida, ausencia, menú, regla o atajo.", Modifier.padding(relleno).padding(top = 60.dp).testTag("sin_automaticos"))
            else -> LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("lista_automaticos")) {
                items(datos, key = { it.chat + "|" + it.id }) { m ->
                    Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s)) {
                        Text("${etiquetaDia(m.ts)} ${horaDe(m.ts)}", Modifier.width(96.dp), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        Column(Modifier.weight(1f)) {
                            Text(TipoAuto.etiqueta(m.automatico), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.primary, modifier = Modifier.testTag("etiqueta_${m.id}"))
                            Text(m.texto.ifBlank { descripcionCorta(m.tipo, m.texto, m.borrado, m.notaVoz) }, maxLines = 3, overflow = TextOverflow.Ellipsis)
                        }
                        TextButton({ alAbrirChat(m.chat) }) { Text("Abrir chat") }
                    }
                    HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                }
            }
        }
    }
}
