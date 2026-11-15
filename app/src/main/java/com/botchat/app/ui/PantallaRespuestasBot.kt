@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

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
import androidx.compose.material3.AssistChip
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.ListaRespuestas
import com.botchat.app.datos.ReemplazoBot
import com.botchat.app.datos.Repo
import com.botchat.app.datos.RespuestaBot
import com.botchat.app.datos.fijarReemplazos
import com.botchat.app.datos.fijarRespuesta
import com.botchat.app.datos.respuestas
import com.botchat.app.datos.restaurarRespuestas
import kotlinx.coroutines.launch

/**
 * Respuestas del bot: todo lo que contesta por su cuenta ante un comando («Listo: el grupo quedó cerrado», «No tienes permiso…», el
 * agradecimiento de un reporte…). Cada una trae su texto de fábrica; se cambia tocándola y se puede volver al de fábrica. Abajo, «Cambiar
 * una frase» sirve para cualquier otra respuesta que no esté en la lista.
 */
@Composable
fun PantallaRespuestasBot(botId: Long, alVolver: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("respuestas" in evs) version++ } }
    val datos by produceState<ListaRespuestas?>(null, version, botId) { value = runCatching { Repo.respuestas(botId) }.getOrNull() ?: value }
    var consulta by remember { mutableStateOf("") }
    var editando by remember { mutableStateOf<RespuestaBot?>(null) }
    var frase by remember { mutableStateOf<Int?>(null) } // -1 = nueva
    var restaurando by remember { mutableStateOf(false) }
    val d = datos
    Scaffold(topBar = {
        TopAppBar(navigationIcon = { IconButton(alVolver, Modifier.testTag("volver_respuestas")) { Icon(Ico.Atras, "Volver") } }, title = { Text("Respuestas del bot") }, actions = {
            if (d != null && (d.filas.any { it.cambiada } || d.reemplazos.isNotEmpty()))
                TextButton({ restaurando = true }, Modifier.testTag("restaurar_respuestas")) { Text("Volver a fábrica") }
        })
    }) { relleno ->
        if (d == null) { Box(Modifier.fillMaxSize().padding(relleno)) { FilasDeCarga(5) }; return@Scaffold }
        val q = consulta.trim().lowercase()
        val vistas = d.filas.filter { q.isEmpty() || it.actual.lowercase().contains(q) || it.descripcion.lowercase().contains(q) || it.defecto.lowercase().contains(q) }
        LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("lista_respuestas"), contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = Esp.l, vertical = Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            item {
                Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text(
                        "Esto es lo que el bot contesta cuando alguien usa un comando. Toca una respuesta para escribir la tuya; las palabras entre llaves, como {n}, se llenan solas.",
                        style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    OutlinedTextField(consulta, { consulta = it }, Modifier.fillMaxWidth().testTag("buscar_respuesta"), singleLine = true, placeholder = { Text("Buscar una respuesta") }, leadingIcon = { Icon(Ico.Buscar, null) })
                    val n = d.filas.count { it.cambiada }
                    if (n > 0) Text(plural(n.toLong(), "respuesta cambiada", "respuestas cambiadas"), Modifier.testTag("contador_cambiadas"), style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.primary)
                }
            }
            if (vistas.isEmpty()) item { EstadoVacio(Ico.Buscar, "Nada coincide", "Prueba con otra palabra.", Modifier.testTag("respuestas_sin_resultado")) }
            val ordenGrupos = d.grupos.ifEmpty { vistas.map { it.grupo }.distinct() }
            ordenGrupos.forEach { g ->
                val delGrupo = vistas.filter { it.grupo == g }
                if (delGrupo.isNotEmpty()) {
                    item(key = "g_$g") { TituloSeccion(g) }
                    items(delGrupo, key = { it.id }) { r ->
                        Card(
                            Modifier.fillMaxWidth().clickable { haptica(Toque.Suave); editando = r }.testTag("respuesta_${r.id}"),
                            colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
                        ) {
                            Column(Modifier.padding(Esp.m), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                                Row(verticalAlignment = Alignment.CenterVertically) {
                                    Text(r.descripcion, Modifier.weight(1f), style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                    if (r.cambiada) Text("Cambiada", Modifier.testTag("cambiada_${r.id}"), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.primary)
                                }
                                Text(r.actual, style = MaterialTheme.typography.bodyLarge, maxLines = 3, overflow = TextOverflow.Ellipsis)
                            }
                        }
                    }
                }
            }
            item(key = "frases") {
                Column(Modifier.padding(top = Esp.m), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    TituloSeccion("Cambiar una frase en cualquier respuesta")
                    Text(
                        "Para lo que no está en la lista: escribe la frase tal como la dice el bot y por cuál quieres cambiarla. Se cambia en todas las respuestas donde aparezca.",
                        style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    d.reemplazos.forEachIndexed { i, r ->
                        Card(Modifier.fillMaxWidth().clickable { frase = i }.testTag("frase_$i"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                            Column(Modifier.padding(Esp.m)) {
                                Text("«${r.de}»", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 2, overflow = TextOverflow.Ellipsis)
                                Text("→ «${r.a}»", style = MaterialTheme.typography.bodyLarge, maxLines = 2, overflow = TextOverflow.Ellipsis)
                            }
                        }
                    }
                    OutlinedButton({ frase = -1 }, Modifier.testTag("nueva_frase")) { Icon(Ico.Agregar, null, Modifier.size(18.dp)); Spacer(Modifier.width(Esp.s)); Text("Cambiar otra frase") }
                    Spacer(Modifier.size(Esp.xl))
                }
            }
        }
    }
    editando?.let { r ->
        EditorRespuesta(
            r, alCerrar = { editando = null },
            alGuardar = { t ->
                editando = null
                scope.launch { runCatching { Repo.fijarRespuesta(botId, r.id, t) }.onSuccess { avisaToast(ctx, if (t.isBlank()) "Volvió al texto de fábrica" else "Guardado", false) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo guardar", true) }; version++ }
            },
        )
    }
    frase?.let { i ->
        val actual = d?.reemplazos?.getOrNull(i)
        EditorFrase(actual, alCerrar = { frase = null }, alGuardar = { nueva ->
            frase = null
            val l = d?.reemplazos.orEmpty().toMutableList()
            if (i >= 0 && i < l.size) { if (nueva == null) l.removeAt(i) else l[i] = nueva } else if (nueva != null) l.add(nueva)
            scope.launch { runCatching { Repo.fijarReemplazos(botId, l) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo guardar", true) }; version++ }
        })
    }
    if (restaurando) AlertDialog(
        onDismissRequest = { restaurando = false }, modifier = Modifier.testTag("dialogo_restaurar_respuestas"),
        title = { Text("¿Volver a los textos de fábrica?") }, text = { Text("Todas tus respuestas cambiadas y tus frases cambiadas se borran. No se puede deshacer.") },
        confirmButton = { TextButton({ restaurando = false; scope.launch { runCatching { Repo.restaurarRespuestas(botId) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }; version++ } }, Modifier.testTag("confirmar_restaurar_respuestas")) { Text("Volver a fábrica") } },
        dismissButton = { TextButton({ restaurando = false }) { Text("Cancelar") } },
    )
}

@Composable
private fun EditorRespuesta(r: RespuestaBot, alCerrar: () -> Unit, alGuardar: (String) -> Unit) {
    var texto by remember(r.id) { mutableStateOf(r.actual) }
    HojaAlerta(
        onDismissRequest = alCerrar, title = { Text("Cambiar respuesta") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text(r.descripcion, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                OutlinedTextField(texto, { texto = it.take(600) }, Modifier.fillMaxWidth().testTag("respuesta_campo"), minLines = 3)
                if (r.vars.isNotEmpty()) {
                    Text("Toca para agregar:", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.xs), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                        r.vars.forEach { v -> AssistChip({ texto = (texto + "{$v}").take(600) }, { Text("{$v}") }, modifier = Modifier.testTag("variable_$v")) }
                    }
                }
                Text("De fábrica: ${r.defecto}", Modifier.testTag("respuesta_defecto"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                if (r.cambiada) TextButton({ alGuardar("") }, Modifier.testTag("respuesta_fabrica")) { Text("Volver al de fábrica") }
            }
        },
        confirmButton = { TextButton({ alGuardar(if (texto.trim() == r.defecto.trim()) "" else texto.trim()) }, Modifier.testTag("respuesta_guardar"), enabled = texto.isNotBlank()) { Text("Guardar") } },
        dismissButton = { TextButton(alCerrar, Modifier.testTag("respuesta_cancelar")) { Text("Cancelar") } },
    )
}

@Composable
private fun EditorFrase(actual: ReemplazoBot?, alCerrar: () -> Unit, alGuardar: (ReemplazoBot?) -> Unit) {
    var de by remember { mutableStateOf(actual?.de.orEmpty()) }
    var a by remember { mutableStateOf(actual?.a.orEmpty()) }
    HojaAlerta(
        onDismissRequest = alCerrar, title = { Text(if (actual == null) "Cambiar una frase" else "Frase cambiada") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                OutlinedTextField(de, { de = it.take(200) }, Modifier.fillMaxWidth().testTag("frase_de"), label = { Text("Frase del bot") }, minLines = 1)
                OutlinedTextField(a, { a = it.take(600) }, Modifier.fillMaxWidth().testTag("frase_a"), label = { Text("Cambiarla por") }, minLines = 2)
                if (actual != null) TextButton({ alGuardar(null) }, Modifier.testTag("frase_quitar")) { Text("Quitar este cambio") }
            }
        },
        confirmButton = { TextButton({ alGuardar(ReemplazoBot(de.trim(), a)) }, Modifier.testTag("frase_guardar"), enabled = de.isNotBlank()) { Text("Guardar") } },
        dismissButton = { TextButton(alCerrar, Modifier.testTag("frase_cancelar")) { Text("Cancelar") } },
    )
}
