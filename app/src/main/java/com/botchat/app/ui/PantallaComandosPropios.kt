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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
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
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.ComandoPropio
import com.botchat.app.datos.ComandoPropioItem
import com.botchat.app.datos.OpcionComando
import com.botchat.app.datos.Rango
import com.botchat.app.datos.Repo
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

// ---------- Ajustes → Comandos propios ----------

/**
 * Comandos propios: cosas como “!horario” o “!precios” que cualquiera (o solo cierto rango) puede escribir y el bot contesta con tu texto.
 * Usan el prefijo actual de los comandos. Solo responden: nunca escriben primero.
 */
@Composable
fun PantallaComandosPropios(botId: Long, alVolver: () -> Unit, alEditar: (Long) -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("automatizaciones" in evs || "permisos_ajustes" in evs) version++ } }
    val lista by produceState<List<ComandoPropioItem>?>(null, version) { value = runCatching { Repo.comandosPropios(botId) }.getOrDefault(emptyList()) }
    val prefijo by produceState("!", version) { value = runCatching { Repo.ajustesPermisos(botId)["prefijo"] }.getOrNull()?.ifBlank { null } ?: "!" }
    var borrando by remember { mutableStateOf<ComandoPropioItem?>(null) }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Comandos propios") }) }) { relleno ->
        val datos = lista
        LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("lista_comandos_propios"), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            item {
                Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text("Crea comandos como ${prefijo}horario o ${prefijo}precios: quien los escribe recibe tu texto. Eliges quién puede usarlos y si salen en ${prefijo}ayuda.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Button({ haptica(Toque.Suave); alEditar(0) }, Modifier.testTag("nuevo_comando")) { Text("Crear comando") }
                }
            }
            when {
                datos == null -> item { Box(Modifier.fillMaxWidth().padding(Esp.xl), Alignment.Center) { CircularProgressIndicator() } }
                datos.isEmpty() -> item { EstadoVacio(Ico.Chats, "Sin comandos propios", "Crea el primero, por ejemplo ${prefijo}horario con tus horas de atención.", Modifier.padding(top = Esp.l).testTag("sin_comandos")) }
                else -> items(datos, key = { it.id }) { c ->
                    Card(onClick = { alEditar(c.id) }, modifier = Modifier.fillMaxWidth().testTag("comando_${c.id}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                        Row(Modifier.padding(horizontal = Esp.l, vertical = Esp.s), verticalAlignment = Alignment.CenterVertically) {
                            Column(Modifier.weight(1f)) {
                                Text(prefijo + c.datos.nombre, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                Text(c.datos.resumen.ifBlank { c.datos.texto }, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 2, overflow = TextOverflow.Ellipsis)
                                Text(dondeFunciona(c.datos), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                            Switch(c.activo, { si -> scope.launch { runCatching { Repo.activarAutomatizacion(botId, c.id, si) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }; version++ } }, Modifier.testTag("comando_activo_${c.id}"))
                            IconButton({ haptica(Toque.Fuerte); borrando = c }, Modifier.testTag("borrar_comando_${c.id}")) { Icon(Ico.Borrar, "Borrar el comando") }
                        }
                    }
                }
            }
        }
    }
    borrando?.let { c ->
        AlertDialog(
            onDismissRequest = { borrando = null }, title = { Text("¿Borrar ${prefijo}${c.datos.nombre}?") },
            text = { Text("Dejará de responder. No se puede deshacer.") },
            confirmButton = { TextButton({ borrando = null; scope.launch { runCatching { Repo.borrarAutomatizacion(botId, c.id) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }; version++ } }, Modifier.testTag("confirmar_borrar_comando")) { Text("Borrar", color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton({ borrando = null }) { Text("Cancelar") } },
        )
    }
}

private fun dondeFunciona(c: ComandoPropio) = when {
    c.privados && c.grupos -> "En privados y en grupos"
    c.grupos -> "Solo en grupos"
    else -> "Solo en privados"
}

// ---------- editor ----------

/** Crear o cambiar un comando propio, con vista previa de cómo lo verá quien lo escriba. */
@Composable
fun EditorComandoPropio(botId: Long, id: Long, alVolver: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    val rangos by produceState<List<Rango>?>(null) { value = runCatching { Repo.rangos(botId).filter { !it.esDueno } }.getOrDefault(emptyList()) }
    val prefijo by produceState("!") { value = runCatching { Repo.ajustesPermisos(botId)["prefijo"] }.getOrNull()?.ifBlank { null } ?: "!" }
    val original by produceState<ComandoPropio?>(null, id) {
        value = if (id == 0L) ComandoPropio() else runCatching { Repo.comandosPropios(botId).firstOrNull { it.id == id }?.datos }.getOrNull() ?: ComandoPropio()
    }
    val o = original
    if (o == null) { Box(Modifier.fillMaxSize(), Alignment.Center) { CircularProgressIndicator() }; return }
    var nombre by remember(o) { mutableStateOf(o.nombre) }
    var texto by remember(o) { mutableStateOf(TextFieldValue(o.texto, TextRange(o.texto.length))) }
    var resumen by remember(o) { mutableStateOf(o.resumen) }
    var rangoMin by remember(o) { mutableStateOf(o.rangoMin) }
    var privados by remember(o) { mutableStateOf(o.privados) }
    var grupos by remember(o) { mutableStateOf(o.grupos) }
    var oculto by remember(o) { mutableStateOf(o.ocultoAyuda) }
    val opciones = remember(o) { mutableStateListOf<OpcionComando>().apply { addAll(o.opciones) } }
    var guardando by remember { mutableStateOf(false) }
    fun actual() = ComandoPropio(nombre.trim(), texto.text, resumen.trim(), rangoMin, privados, grupos, oculto, opciones.toList())
    // Vista previa: la arma el módulo Go con el mismo texto y el prefijo actual (con datos de ejemplo en {nombre}, {hora} y {dia}).
    val vista by produceState<String?>(null, nombre, texto.text, opciones.toList(), privados, grupos) {
        delay(250)
        value = runCatching { Repo.vistaPreviaComandoPropio(botId, actual()) }.getOrElse { it.message }
    }
    fun envuelve(marca: String) {
        val t = texto
        val sel = t.selection
        val nuevo = t.text.replaceRange(sel.min, sel.max, marca + t.text.substring(sel.min, sel.max) + marca)
        texto = TextFieldValue(nuevo, TextRange(sel.min + marca.length, sel.max + marca.length))
    }
    fun inserta(x: String) {
        val t = texto
        val sel = t.selection
        texto = TextFieldValue(t.text.replaceRange(sel.min, sel.max, x), TextRange(sel.min + x.length))
    }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text(if (id == 0L) "Nuevo comando" else "Cambiar comando") }) }) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            OutlinedTextField(
                nombre, { nombre = it.filter { c -> c.isLetterOrDigit() }.take(20) }, Modifier.fillMaxWidth().testTag("comando_nombre"),
                label = { Text("Nombre del comando") }, prefix = { Text(prefijo) }, singleLine = true,
                supportingText = { Text("Una sola palabra, sin espacios. Se escribe como $prefijo${nombre.ifBlank { "horario" }}") },
            )
            OutlinedTextField(texto, { texto = it.copy(text = it.text.take(2000)) }, Modifier.fillMaxWidth().testTag("comando_texto"), label = { Text(if (opciones.isEmpty()) "Qué responde" else "Título del menú") }, minLines = 3)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                OutlinedButton({ envuelve("*") }, Modifier.testTag("estilo_negrita")) { Text("Negrita") }
                OutlinedButton({ envuelve("_") }, Modifier.testTag("estilo_cursiva")) { Text("Cursiva") }
                OutlinedButton({ envuelve("~") }, Modifier.testTag("estilo_tachado")) { Text("Tachado") }
                OutlinedButton({ inserta("{nombre}") }, Modifier.testTag("variable_nombre")) { Text("{nombre}") }
                OutlinedButton({ inserta("{hora}") }, Modifier.testTag("variable_hora")) { Text("{hora}") }
                OutlinedButton({ inserta("{dia}") }, Modifier.testTag("variable_dia")) { Text("{dia}") }
            }
            Text("{nombre} pone el nombre de quien escribe; {hora} y {dia}, la hora y el día de ese momento.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            SeccionEst("Opciones (opcional)") {
                Text("Si agregas opciones, el comando muestra un menú numerado y cada opción tiene su respuesta. Los menús solo funcionan en chats privados.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                opciones.forEachIndexed { i, op ->
                    Column(Modifier.fillMaxWidth().testTag("opcion_$i"), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            OutlinedTextField(op.texto, { opciones[i] = op.copy(texto = it.take(40)) }, Modifier.weight(1f).testTag("opcion_texto_$i"), label = { Text("Opción ${i + 1}") }, singleLine = true)
                            IconButton({ opciones.removeAt(i) }, Modifier.testTag("quitar_opcion_$i")) { Icon(Ico.Borrar, "Quitar la opción") }
                        }
                        OutlinedTextField(op.respuesta, { opciones[i] = op.copy(respuesta = it.take(1000)) }, Modifier.fillMaxWidth().testTag("opcion_respuesta_$i"), label = { Text("Qué responde la opción ${i + 1}") }, minLines = 2)
                    }
                }
                if (opciones.size < 9) OutlinedButton({ opciones.add(OpcionComando("", "")); grupos = false }, Modifier.testTag("agregar_opcion")) { Text("Agregar una opción") }
            }
            SeccionEst("Quién y dónde") {
                Text("Quién puede usarlo", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    FilterChip(rangoMin == 0L, { rangoMin = 0 }, { Text("Cualquiera") }, modifier = Modifier.testTag("rango_cualquiera"))
                    rangos?.forEach { r -> FilterChip(rangoMin == r.id, { rangoMin = r.id }, { Text(r.nombre + " o más", maxLines = 1) }, leadingIcon = { PuntoRango(r.color, 10) }, modifier = Modifier.testTag("rango_min_${r.id}")) }
                }
                Row(verticalAlignment = Alignment.CenterVertically) { Switch(privados, { privados = it }, Modifier.testTag("comando_privados")); Text("  En chats privados") }
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Switch(grupos, { grupos = it; if (it) opciones.clear() }, Modifier.testTag("comando_grupos")); Text("  En grupos (donde el bot ya responde)")
                }
                Row(verticalAlignment = Alignment.CenterVertically) { Switch(!oculto, { oculto = !it }, Modifier.testTag("comando_en_ayuda")); Text("  Que salga en ${prefijo}ayuda") }
                if (!oculto) OutlinedTextField(resumen, { resumen = it.take(80) }, Modifier.fillMaxWidth().testTag("comando_resumen"), label = { Text("Descripción corta para la ayuda") }, singleLine = true)
            }
            SeccionEst("Archivos adjuntos") {
                Text("Por ahora el comando responde solo con texto. Los adjuntos (foto, PDF…) llegan con los Mensajes automáticos, que se podrán usar también aquí.", Modifier.testTag("nota_adjuntos"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            SeccionEst("Así lo verá quien lo escriba") {
                Text(vista ?: "…", Modifier.fillMaxWidth().testTag("vista_previa_comando"), style = MaterialTheme.typography.bodyMedium)
            }
            Button({
                guardando = true
                haptica(Toque.Confirmar)
                scope.launch {
                    runCatching { Repo.guardarComandoPropio(botId, id, actual()) }
                        .onSuccess { avisaToast(ctx, "Guardado", false); alVolver() }
                        .onFailure { avisaToast(ctx, it.message ?: "No se pudo guardar", true); guardando = false }
                }
            }, Modifier.fillMaxWidth().testTag("guardar_comando"), enabled = !guardando && nombre.isNotBlank() && texto.text.isNotBlank()) { Text("Guardar") }
        }
    }
}
