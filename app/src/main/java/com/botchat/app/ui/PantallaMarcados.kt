@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
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
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Tab
import androidx.compose.material3.TabRow
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.ConteoMarcados
import com.botchat.app.datos.EtiquetaItem
import com.botchat.app.datos.MarcadoItem
import com.botchat.app.datos.Repo
import com.botchat.app.datos.conteoMarcados
import com.botchat.app.datos.marcados
import com.botchat.app.datos.marcarMensaje
import com.botchat.app.datos.marcasDeMensaje
import com.botchat.app.datos.quitarMarcaMensaje
import kotlinx.coroutines.launch

private val PESTANAS_MARCADOS = listOf("mensaje" to "Mensajes", "persona" to "Personas", "chat" to "Chats", "grupo" to "Grupos")

/**
 * Elegir etiquetas con botones de colores. Reutilizable: `activas` son las que están prendidas y `alToque` avisa cuál se tocó
 * (la pantalla decide si es una sola o varias). `etiqueta` es el prefijo de los tags de prueba.
 */
@Composable
fun SelectorEtiquetas(todas: List<EtiquetaItem>, activas: Set<Long>, etiqueta: String, modifier: Modifier = Modifier, alToque: (EtiquetaItem) -> Unit) {
    val haptica = rememberHaptica()
    FlowRow(modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
        todas.forEach { e ->
            FilterChip(
                e.id in activas, { haptica(Toque.Suave); alToque(e) }, { Text((e.emoji + " " + e.nombre).trim(), maxLines = 1) },
                modifier = Modifier.testTag("${etiqueta}_${e.id}"), leadingIcon = { PuntoRango(e.color, 10) },
            )
        }
    }
}

/** Bot → Etiquetas y marcados → Ver lo marcado: mensajes, personas, chats y grupos con etiqueta, con filtro por etiqueta. */
@Composable
fun PantallaMarcados(
    botId: Long, alVolver: () -> Unit, alAbrirChat: (String) -> Unit, alIrAlMensaje: (chat: String, id: String) -> Unit,
    alAbrirPersona: (String) -> Unit, alAbrirGrupo: (String) -> Unit, alEtiquetas: () -> Unit = {},
) {
    var pestana by remember { mutableIntStateOf(0) }
    var filtro by remember { mutableLongStateOf(0L) }
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("etiquetas" in evs || "personas" in evs || "chats" in evs) version++ } }
    val etiquetas by produceState<List<EtiquetaItem>>(emptyList(), version) { value = runCatching { Repo.etiquetas(botId) }.getOrDefault(emptyList()) }
    val conteo by produceState(ConteoMarcados(0, 0, 0, 0), filtro, version) { value = runCatching { Repo.conteoMarcados(botId, filtro) }.getOrDefault(ConteoMarcados(0, 0, 0, 0)) }
    val tipo = PESTANAS_MARCADOS[pestana].first
    val lista by produceState<List<MarcadoItem>?>(null, tipo, filtro, version) { value = null; value = runCatching { Repo.marcados(botId, tipo, filtro) }.getOrDefault(emptyList()) }
    fun cuenta(k: String) = when (k) { "mensaje" -> conteo.mensaje; "persona" -> conteo.persona; "chat" -> conteo.chat; else -> conteo.grupo }
    val porId = etiquetas.associateBy { it.id }

    Scaffold(
        topBar = {
            Column {
                TopAppBar(
                    navigationIcon = { IconButton(alVolver, Modifier.testTag("volver_marcados")) { Icon(Ico.Atras, "Volver") } }, title = { Text("Lo marcado") },
                    actions = { TextButton(alEtiquetas, Modifier.testTag("administrar_etiquetas")) { Text("Etiquetas") } },
                )
                TabRow(pestana) {
                    PESTANAS_MARCADOS.forEachIndexed { i, (k, t) ->
                        Tab(pestana == i, { pestana = i }, text = { Text("$t ${cuenta(k)}", maxLines = 1) }, modifier = Modifier.testTag("pestana_marcados_$k"))
                    }
                }
            }
        },
    ) { relleno ->
        LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("lista_marcados"), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            item {
                if (etiquetas.isNotEmpty()) Column(verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                    Text("Filtrar por etiqueta", style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                        FilterChip(filtro == 0L, { filtro = 0L }, { Text("Todas") }, modifier = Modifier.testTag("filtro_marcados_todas"))
                    }
                    SelectorEtiquetas(etiquetas, setOf(filtro), "filtro_marcados") { filtro = if (filtro == it.id) 0L else it.id }
                }
            }
            val datos = lista
            when {
                datos == null -> item { FilasDeCarga(3) }
                datos.isEmpty() -> item {
                    EstadoVacio(
                        Ico.Seleccionar, "Todavía no hay nada aquí",
                        when (tipo) { "mensaje" -> "Mantén pulsado un mensaje en un chat y toca Marcar."; "persona" -> "Abre una persona y toca Marcar."; "chat" -> "En un chat toca ⋮ y pon una etiqueta."; else -> "Abre un grupo y ponle una etiqueta." },
                        Modifier.padding(top = Esp.xl).testTag("sin_marcados"),
                    )
                }
                else -> items(datos, key = { "${it.tipo}|${it.chat}|${it.msgId}|${it.persona}" }) { m ->
                    FilaMarcado(m, porId, Modifier.clickable {
                        when (m.tipo) { "mensaje" -> alIrAlMensaje(m.chat, m.msgId); "persona" -> alAbrirPersona(m.persona.ifBlank { m.chat }); "grupo" -> alAbrirGrupo(m.chat); else -> alAbrirChat(m.chat) }
                    }.testTag("marcado_${m.tipo}_${m.msgId.ifBlank { m.persona.ifBlank { m.chat } }}"))
                }
            }
        }
    }
}

@Composable
private fun FilaMarcado(m: MarcadoItem, porId: Map<Long, EtiquetaItem>, modifier: Modifier) {
    Card(modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Row(Modifier.padding(Esp.m), verticalAlignment = Alignment.CenterVertically) {
            if (m.tipo != "mensaje") { Avatar(m.nombre.ifBlank { m.chatNombre }, m.foto, 0, m.persona.ifBlank { m.chat }, 44.dp); Spacer(Modifier.width(Esp.m)) }
            Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                if (m.tipo == "mensaje") {
                    Text(
                        (m.chatNombre.ifBlank { "Chat" }) + (if (m.remitente.isNotBlank() && !m.esMio) " · ${m.remitente}" else if (m.esMio) " · Tú" else "") + " · " + etiquetaDia(m.ts),
                        style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis,
                    )
                    Text(
                        if (m.texto.isNotBlank()) m.texto else if (m.olvidado) "Contenido no guardado" else descripcionCorta(m.msgTipo, "", false),
                        Modifier.testTag("marcado_texto"), style = MaterialTheme.typography.bodyMedium, maxLines = 3, overflow = TextOverflow.Ellipsis,
                        fontStyle = if (m.texto.isBlank()) androidx.compose.ui.text.font.FontStyle.Italic else null,
                    )
                } else {
                    Text(m.nombre.ifBlank { m.chatNombre }, style = MaterialTheme.typography.titleSmall, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
                if (m.nota.isNotBlank()) Text("Nota: ${m.nota}", style = MaterialTheme.typography.bodySmall, color = AMARILLO_SILENCIO)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    m.etiquetas.mapNotNull { porId[it] }.forEach { e ->
                        Row(verticalAlignment = Alignment.CenterVertically) { PuntoRango(e.color, 10); Spacer(Modifier.width(4.dp)); Text((e.emoji + " " + e.nombre).trim(), style = MaterialTheme.typography.labelSmall) }
                    }
                }
            }
        }
    }
}

/** Marcar un mensaje desde el menú del chat: se toca una etiqueta para ponerla y otra vez para quitarla. */
@Composable
fun DialogoMarcarRapido(botId: Long, chat: String, msgId: String, alCerrar: () -> Unit) {
    val scope = androidx.compose.runtime.rememberCoroutineScope()
    val ctx = androidx.compose.ui.platform.LocalContext.current
    val todas by produceState<List<EtiquetaItem>?>(null) { value = runCatching { Repo.etiquetas(botId) }.getOrDefault(emptyList()) }
    val activas = remember { androidx.compose.runtime.mutableStateListOf<Long>() }
    LaunchedEffect(chat, msgId) { runCatching { Repo.marcasDeMensaje(botId, chat, msgId) }.onSuccess { ms -> activas.clear(); activas.addAll(ms.filter { it.etiquetaId > 0 }.map { it.etiquetaId }) } }
    AlertDialog(
        onDismissRequest = alCerrar, title = { Text("Marcar este mensaje") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("Toca una etiqueta para ponerla; vuélvela a tocar para quitarla.")
                val ts = todas
                when {
                    ts == null -> Text("Cargando…")
                    ts.isEmpty() -> Text("Aún no tienes etiquetas. Créalas en Bot → Etiquetas y marcados.", Modifier.testTag("marcar_rapido_sin_etiquetas"))
                    else -> SelectorEtiquetas(ts, activas.toSet(), "marcar_rapido") { e ->
                        val poner = e.id !in activas
                        scope.launch {
                            runCatching { if (poner) Repo.marcarMensaje(botId, chat, msgId, e.id, "") else Repo.quitarMarcaMensaje(botId, chat, msgId, e.id) }
                                .onSuccess { if (poner) activas.add(e.id) else activas.remove(e.id) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
                        }
                    }
                }
            }
        },
        confirmButton = { TextButton(alCerrar, Modifier.testTag("marcar_rapido_listo")) { Text("Listo") } },
    )
}
