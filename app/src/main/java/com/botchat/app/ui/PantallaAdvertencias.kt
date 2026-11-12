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
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.AdvertenciaItem
import com.botchat.app.datos.AdvertidoItem
import com.botchat.app.datos.FichaPersona
import com.botchat.app.datos.NotaItem
import com.botchat.app.datos.Repo
import kotlinx.coroutines.launch

/** Ajustes → Advertencias y notas: a quién se ha advertido y cuántas veces. Tocar una persona abre su detalle. */
@Composable
fun PantallaAdvertencias(botId: Long, alVolver: () -> Unit, alAbrirPersona: (String) -> Unit, alTipos: () -> Unit = {}, alReglas: () -> Unit = {}) {
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if (evs.any { it == "personas" || it == "permisos_ajustes" || it.startsWith("persona:") }) version++ } }
    val lista by produceState<List<AdvertidoItem>?>(null, version) { value = runCatching { Repo.personasAdvertidas(botId) }.getOrDefault(emptyList()) }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Advertencias y notas") }) }) { relleno ->
        val datos = lista
        LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("lista_advertidos"), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            item { CabeceraAdvertencias(botId, alTipos, alReglas) }
            when {
                datos == null -> item { Box(Modifier.fillMaxWidth().padding(Esp.xl), Alignment.Center) { CircularProgressIndicator() } }
                datos.isEmpty() -> item {
                    EstadoVacio(
                        Ico.Aviso, "Nadie tiene advertencias", "Para advertir a alguien abre su perfil en Personas y toca Advertir, o usa el comando !advertir. Las notas de cada persona están en su ficha.",
                        Modifier.padding(top = Esp.xl).testTag("sin_advertencias"),
                    )
                }
                else -> {
                    item { Text("Aquí cuentan solo las advertencias vigentes. Lo que pasa al llegar al tope depende del tipo (Tipos y escalera).", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                    items(datos, key = { it.persona }) { x ->
                        Card(onClick = { alAbrirPersona(x.persona) }, modifier = Modifier.fillMaxWidth().testTag("advertido_${x.persona}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                            Row(Modifier.padding(Esp.l), verticalAlignment = Alignment.CenterVertically) {
                                Column(Modifier.weight(1f)) {
                                    Text(x.nombre.ifBlank { numeroDePersona(x.persona) }, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                    Text("Última: ${etiquetaDia(x.ultimaTs)} ${horaDe(x.ultimaTs)}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                }
                                val alTope = x.tope > 0 && x.total >= x.tope
                                Text(
                                    if (x.tope > 0) "${x.total} de ${x.tope}" else "${x.total}", Modifier.testTag("total_${x.persona}"),
                                    style = MaterialTheme.typography.titleMedium, color = if (alTope) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurface, fontWeight = FontWeight.Bold,
                                )
                                Icon(Ico.Siguiente, null, Modifier.padding(start = Esp.s).size(20.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                        }
                    }
                }
            }
        }
    }
}

/** Advertencias y notas de una persona: quitar una, borrar el historial (con confirmación), escribir y borrar notas. */
@Composable
fun PantallaPersonaDetalle(botId: Long, persona: String, alVolver: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(persona) { Repo.cambios.collect { evs -> if (evs.any { it == "personas" || it == "persona:$persona" }) version++ } }
    val ficha by produceState<FichaPersona?>(null, persona, version) { value = runCatching { Repo.ficha(botId, persona) }.getOrNull() }
    val advertencias by produceState<List<AdvertenciaItem>?>(null, persona, version) { value = runCatching { Repo.advertencias(botId, persona) }.getOrDefault(emptyList()) }
    val notas by produceState<List<NotaItem>?>(null, persona, version) { value = runCatching { Repo.notasPersona(botId, persona) }.getOrDefault(emptyList()) }
    val estadisticas by produceState<com.botchat.app.datos.EstadisticasPersona?>(null, persona, version) { value = runCatching { Repo.estadisticasPersona(botId, persona) }.getOrNull() }
    var dialogo by remember { mutableStateOf<String?>(null) }
    var motivoNuevo by remember { mutableStateOf("") }
    var notaNueva by remember { mutableStateOf("") }
    var resultadoAdv by remember { mutableStateOf<com.botchat.app.datos.ResultadoAdvertencia?>(null) }
    fun hacer(accion: suspend () -> Unit) = scope.launch {
        runCatching { accion() }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
        version++
    }
    val nombre = ficha?.nombre?.ifBlank { null } ?: numeroDePersona(persona)
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text(nombre, maxLines = 1, overflow = TextOverflow.Ellipsis) }) }) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            SeccionEst("Actividad de esta persona") {
                val e = estadisticas
                when {
                    e == null -> CircularProgressIndicator(Modifier.size(24.dp))
                    e.total == 0 -> Text("Todavía no escribió mensajes.", Modifier.testTag("sin_actividad"), color = MaterialTheme.colorScheme.onSurfaceVariant)
                    else -> {
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(Esp.m)) {
                            Cifra(e.total.toLong(), "Mensajes", "Mensaje", Modifier.weight(1f).testTag("persona_total"))
                            Cifra(e.chats.size.toLong(), "Chats", "Chat", Modifier.weight(1f).testTag("persona_chats"))
                        }
                        e.porTipo.entries.sortedByDescending { it.value }.forEach { (t, n) ->
                            Row(Modifier.fillMaxWidth()) { Text(nombreTipo(t), Modifier.weight(1f)); Text(numero(n.toLong()), color = MaterialTheme.colorScheme.onSurfaceVariant) }
                        }
                        Text("A qué hora escribe", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        GraficoBarras(e.porHora.map { it.toLong() }, etiquetasHoras(), "Mensajes por hora del día", Modifier.testTag("persona_grafico_hora"))
                        e.chats.take(5).forEach { c ->
                            Row(Modifier.fillMaxWidth().testTag("persona_chat_${c.chat}")) {
                                Text(c.nombre.ifBlank { if (c.grupo) "Grupo" else "Chat privado" }, Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis)
                                Text(numero(c.cuenta.toLong()), color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                        }
                    }
                }
            }
            SeccionEst("Advertencias") {
                val a = advertencias
                when {
                    a == null -> CircularProgressIndicator(Modifier.size(24.dp))
                    a.isEmpty() -> Text("No tiene advertencias.", Modifier.testTag("sin_advertencias_persona"), color = MaterialTheme.colorScheme.onSurfaceVariant)
                    else -> a.forEach { x ->
                        Row(Modifier.fillMaxWidth().testTag(if (x.vigente) "advertencia_${x.id}" else "advertencia_vencida_${x.id}"), verticalAlignment = Alignment.CenterVertically) {
                            Column(Modifier.weight(1f)) {
                                Text(etiquetaDia(x.ts) + " " + horaDe(x.ts), style = MaterialTheme.typography.bodyMedium, fontWeight = FontWeight.Medium, color = if (x.vigente) MaterialTheme.colorScheme.onSurface else MaterialTheme.colorScheme.onSurfaceVariant)
                                Text(x.motivo.ifBlank { "Sin motivo" }, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                Text(
                                    if (x.vigente) (if (x.caducaTs > 0) "Vigente · deja de contar el ${etiquetaDia(x.caducaTs)}" else "Vigente · no caduca") else "Vencida · ya no cuenta",
                                    Modifier.testTag("estado_adv_${x.id}"), style = MaterialTheme.typography.labelMedium, color = if (x.vigente) AMARILLO_SILENCIO else MaterialTheme.colorScheme.onSurfaceVariant,
                                )
                            }
                            TextButton({ haptica(Toque.Suave); hacer { Repo.quitarAdvertencia(botId, persona, x.id) } }, Modifier.testTag("quitar_advertencia_${x.id}")) { Text("Quitar") }
                        }
                    }
                }
                OutlinedTextField(motivoNuevo, { motivoNuevo = it.take(200) }, Modifier.fillMaxWidth().testTag("motivo_advertencia"), label = { Text("Motivo de una nueva advertencia (opcional)") }, singleLine = true)
                Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    androidx.compose.material3.Button({
                        haptica(Toque.Suave)
                        val m = motivoNuevo.trim(); motivoNuevo = ""
                        scope.launch { runCatching { Repo.advertir(botId, persona, m) }.onSuccess { resultadoAdv = it }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }; version++ }
                    }, Modifier.testTag("detalle_advertir")) { Text("Advertir") }
                    if (!advertencias.isNullOrEmpty()) OutlinedButton({ haptica(Toque.Fuerte); dialogo = "borrar_historial" }, Modifier.testTag("borrar_historial"), colors = androidx.compose.material3.ButtonDefaults.outlinedButtonColors(contentColor = MaterialTheme.colorScheme.error)) { Text("Borrar historial") }
                }
            }
            SeccionEst("Notas (solo tú las ves)") {
                val n = notas
                when {
                    n == null -> CircularProgressIndicator(Modifier.size(24.dp))
                    n.isEmpty() -> Text("No tiene notas.", Modifier.testTag("sin_notas_persona"), color = MaterialTheme.colorScheme.onSurfaceVariant)
                    else -> n.forEach { x ->
                        Row(Modifier.fillMaxWidth().testTag("nota_${x.id}"), verticalAlignment = Alignment.Top) {
                            Column(Modifier.weight(1f)) {
                                Text(etiquetaDia(x.ts), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                Text(x.texto, style = MaterialTheme.typography.bodyMedium)
                            }
                            IconButton({ haptica(Toque.Suave); hacer { Repo.borrarNota(botId, persona, x.id) } }, Modifier.testTag("borrar_nota_${x.id}")) { Icon(Ico.Borrar, "Borrar la nota") }
                        }
                    }
                }
                OutlinedTextField(notaNueva, { notaNueva = it.take(2000) }, Modifier.fillMaxWidth().testTag("texto_nota_detalle"), label = { Text("Escribe una nota") }, minLines = 2)
                androidx.compose.material3.Button({
                    haptica(Toque.Suave)
                    val t = notaNueva.trim(); notaNueva = ""
                    hacer { Repo.agregarNota(botId, persona, t) }
                }, Modifier.testTag("guardar_nota_detalle"), enabled = notaNueva.isNotBlank()) { Text("Guardar nota") }
            }
        }
    }
    when (dialogo) {
        "borrar_historial" -> AlertDialog(
            onDismissRequest = { dialogo = null }, title = { Text("¿Borrar todas las advertencias?") },
            text = { Text("Se borra el historial completo de advertencias de $nombre. No se puede deshacer.") },
            confirmButton = { TextButton({ dialogo = null; haptica(Toque.Fuerte); hacer { Repo.borrarAdvertencias(botId, persona) } }, Modifier.testTag("confirmar_borrar_historial")) { Text("Borrar", color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton({ dialogo = null }) { Text("Cancelar") } },
        )
    }
    resultadoAdv?.let { DialogoResultadoAdvertencia(it, nombre) { resultadoAdv = null; version++ } }
}
