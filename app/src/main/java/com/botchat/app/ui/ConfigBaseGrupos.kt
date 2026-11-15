@file:OptIn(ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Checkbox
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.BaseGrupos
import com.botchat.app.datos.ChatItem
import com.botchat.app.datos.Repo
import com.botchat.app.datos.aplicarBaseGrupos
import com.botchat.app.datos.baseGrupos
import com.botchat.app.datos.baseGruposAuto
import com.botchat.app.datos.guardarBaseGrupos
import com.botchat.app.datos.mandarEncuesta
import com.botchat.app.datos.quitarBaseGrupos
import kotlinx.coroutines.launch
import java.text.DateFormat
import java.util.Date

/**
 * La forma en que quieres cuidar todos tus grupos, guardada una vez: se copia de un grupo que ya dejaste a tu gusto y se pone en otros
 * grupos con un toque o sola en cada grupo nuevo. Incluye filtros, reglas, horario de cierre y protección de entrada.
 */
@Composable
fun SeccionBaseGrupos(botId: Long, jid: String, version: Int, alCambiar: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var local by remember { mutableStateOf(0) }
    var base by remember { mutableStateOf<BaseGrupos?>(null) } // se conserva mientras se vuelve a leer: nada de parpadeo
    LaunchedEffect(botId, version, local) { base = runCatching { Repo.baseGrupos(botId) }.getOrNull() ?: base }
    var confirma by remember { mutableStateOf<String?>(null) } // "guardar" | "aqui" | "quitar"
    var eligiendo by remember { mutableStateOf(false) }
    fun trabajo(ok: String, f: suspend () -> Unit) = scope.launch {
        runCatching { f() }.onSuccess { avisaToast(ctx, ok, false) }.onFailure { avisaToast(ctx, textoError(it), true) }
        local++; alCambiar()
    }
    val b = base
    Card(Modifier.fillMaxWidth().padding(horizontal = Esp.l).testTag("tarjeta_base"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            if (b == null) {
                FilasDeCarga(2)
            } else {
                Text(
                    if (!b.existe) "Todavía no hay una configuración base. Deja un grupo a tu gusto y úsalo como base para los demás."
                    else "Base de «${b.desde}», guardada el ${DateFormat.getDateInstance(DateFormat.MEDIUM).format(Date(b.fecha * 1000))}.",
                    Modifier.testTag("base_estado"), style = MaterialTheme.typography.bodyMedium,
                )
                if (b.existe) Text(
                    buildList {
                        add(plural(b.filtros.toLong(), "filtro encendido", "filtros encendidos"))
                        if (b.reglas) add("reglas escritas")
                        if (b.horario) add("horario de cierre")
                        if (b.entrada) add("protección de entrada")
                    }.joinToString(" · "),
                    Modifier.testTag("base_contenido"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                OutlinedButton({ haptica(Toque.Suave); confirma = "guardar" }, Modifier.fillMaxWidth().testTag("base_guardar")) { Text("Usar este grupo como base") }
                if (b.existe) {
                    OutlinedButton({ haptica(Toque.Suave); confirma = "aqui" }, Modifier.fillMaxWidth().testTag("base_aplicar_aqui")) { Text("Ponerle la base a este grupo") }
                    Button({ haptica(Toque.Suave); eligiendo = true }, Modifier.fillMaxWidth().testTag("base_aplicar_varios")) { Text("Ponerla en varios grupos…") }
                    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) {
                            Text("Ponerla sola en cada grupo nuevo", style = MaterialTheme.typography.bodyMedium)
                            Text("Solo si el grupo no tiene nada configurado todavía. Te aviso cuando lo haga.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                        Switch(b.auto, { v -> haptica(Toque.Suave); trabajo(if (v) "Listo: los grupos nuevos recibirán la base" else "Listo: los grupos nuevos quedan sin tocar") { Repo.baseGruposAuto(botId, v) } }, Modifier.testTag("sw_base_auto"))
                    }
                    TextButton({ haptica(Toque.Suave); confirma = "quitar" }, Modifier.testTag("base_quitar")) { Text("Quitar la base") }
                }
            }
        }
    }
    confirma?.let { c ->
        val (titulo, texto, boton) = when (c) {
            "guardar" -> Triple("¿Usar este grupo como base?", "Se guardan sus filtros, reglas, horario y protección de entrada. Reemplaza la base anterior.", "Guardar")
            "aqui" -> Triple("¿Ponerle la base a este grupo?", "Lo que este grupo tiene (filtros, reglas, horario y protección) se reemplaza. Las personas, advertencias y baneados no se tocan.", "Poner")
            else -> Triple("¿Quitar la base?", "Los grupos que ya la recibieron no cambian. Los nuevos dejan de recibirla.", "Quitar")
        }
        AlertDialog(
            onDismissRequest = { confirma = null }, title = { Text(titulo) }, text = { Text(texto) },
            confirmButton = {
                TextButton({
                    confirma = null
                    when (c) {
                        "guardar" -> trabajo("Listo: este grupo es ahora la base") { Repo.guardarBaseGrupos(botId, jid) }
                        "aqui" -> trabajo("Listo: le puse la base a este grupo") { Repo.aplicarBaseGrupos(botId, listOf(jid)) }
                        else -> trabajo("Listo: quité la base") { Repo.quitarBaseGrupos(botId) }
                    }
                }, Modifier.testTag("base_confirmar")) { Text(boton) }
            },
            dismissButton = { TextButton({ confirma = null }) { Text("Cancelar") } },
        )
    }
    if (eligiendo) HojaVariosGrupos(botId, jid, { eligiendo = false }) { elegidos ->
        eligiendo = false
        trabajo("Listo: la base quedó en ${plural(elegidos.size.toLong(), "grupo", "grupos")}") { Repo.aplicarBaseGrupos(botId, elegidos) }
    }
}

/** Hoja para marcar varios grupos. */
@Composable
private fun HojaVariosGrupos(botId: Long, actual: String, alCerrar: () -> Unit, alElegir: (List<String>) -> Unit) {
    val lista by produceState<List<ChatItem>?>(null) { value = runCatching { Repo.chats(botId, true, "", 300) }.getOrDefault(emptyList()) }
    val marcados = remember { mutableStateListOf<String>() }
    AlertDialog(
        onDismissRequest = alCerrar, title = { Text("¿En qué grupos?") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("Lo que tengan ahora se reemplaza por la base.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                val datos = lista
                when {
                    datos == null -> FilasDeCarga(3)
                    datos.isEmpty() -> Text("No tienes grupos todavía.")
                    else -> {
                        TextButton({ if (marcados.size == datos.size) marcados.clear() else { marcados.clear(); marcados.addAll(datos.map { it.jid }) } }, Modifier.testTag("base_todos")) {
                            Text(if (marcados.size == datos.size) "Quitar todos" else "Marcar todos")
                        }
                        LazyColumn(Modifier.fillMaxWidth().height(300.dp)) {
                            items(datos, key = { it.jid }) { c ->
                                Row(Modifier.fillMaxWidth().clickable { if (c.jid in marcados) marcados.remove(c.jid) else marcados.add(c.jid) }.padding(vertical = Esp.xs).testTag("base_grupo_${c.jid}"), verticalAlignment = Alignment.CenterVertically) {
                                    Checkbox(c.jid in marcados, null)
                                    Text(c.nombre.ifBlank { "Grupo" } + if (c.jid == actual) " (este)" else "", Modifier.padding(start = Esp.s), maxLines = 1)
                                }
                            }
                        }
                    }
                }
            }
        },
        confirmButton = { TextButton({ alElegir(marcados.toList()) }, Modifier.testTag("base_poner_en_elegidos"), enabled = marcados.isNotEmpty()) { Text("Poner en ${marcados.size}") } },
        dismissButton = { TextButton(alCerrar) { Text("Cancelar") } },
    )
}

/** Manda una encuesta al grupo: la pregunta, de 2 a 12 opciones y si se puede elegir más de una. */
@Composable
fun TarjetaEncuesta(botId: Long, jid: String, conectado: Boolean) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var pregunta by remember { mutableStateOf("") }
    val opciones = remember { mutableStateListOf("", "") }
    var varias by remember { mutableStateOf(false) }
    var enviando by remember { mutableStateOf(false) }
    val validas = opciones.map { it.trim() }.filter { it.isNotEmpty() }
    Column(Modifier.padding(horizontal = Esp.l).testTag("tarjeta_encuesta"), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
        OutlinedTextField(pregunta, { pregunta = it.take(200) }, Modifier.fillMaxWidth().testTag("encuesta_pregunta"), label = { Text("Pregunta") })
        opciones.forEachIndexed { i, o ->
            OutlinedTextField(o, { opciones[i] = it.take(100) }, Modifier.fillMaxWidth().testTag("encuesta_opcion_$i"), singleLine = true, label = { Text("Opción ${i + 1}") })
        }
        FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
            OutlinedButton({ if (opciones.size < 12) opciones.add("") }, Modifier.testTag("encuesta_mas"), enabled = opciones.size < 12) { Text("Otra opción") }
            if (opciones.size > 2) TextButton({ opciones.removeAt(opciones.lastIndex) }, Modifier.testTag("encuesta_menos")) { Text("Quitar la última") }
        }
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text("Se puede elegir más de una", Modifier.weight(1f), style = MaterialTheme.typography.bodyMedium)
            Switch(varias, { varias = it }, Modifier.testTag("sw_encuesta_varias"))
        }
        Button({
            enviando = true
            scope.launch {
                runCatching { Repo.mandarEncuesta(botId, jid, pregunta.trim(), validas, varias) }
                    .onSuccess { pregunta = ""; opciones.clear(); opciones.addAll(listOf("", "")); avisaToast(ctx, "Encuesta enviada al grupo", false) }
                    .onFailure { avisaToast(ctx, textoError(it), true) }
                enviando = false
            }
        }, Modifier.testTag("encuesta_enviar"), enabled = pregunta.isNotBlank() && validas.size >= 2 && !enviando && conectado) { Text("Enviar encuesta") }
        if (!conectado) Text("El bot no está conectado: enciéndelo para enviar.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}
