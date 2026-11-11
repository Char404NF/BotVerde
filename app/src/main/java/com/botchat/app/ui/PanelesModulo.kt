@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateMapOf
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
import androidx.compose.ui.unit.dp
import com.botchat.app.AlarmasModulos
import com.botchat.app.datos.AdjuntoSubido
import com.botchat.app.datos.BotonPanel
import com.botchat.app.datos.CampoModulo
import com.botchat.app.datos.FilaPanel
import com.botchat.app.datos.ModuloInfo
import com.botchat.app.datos.PanelModulo
import com.botchat.app.datos.Repo
import com.botchat.app.datos.TipoCampo
import kotlinx.coroutines.launch

/**
 * Los paneles de un módulo (v0.5.0, H parte 1): listas con su formulario que Go describe y la app dibuja, así los módulos con
 * destinatarios, mensajes programados o conectores NO necesitan una pantalla propia. Cada botón llama a `Repo.accionModulo`; lo que
 * responde Go (o el error) se muestra tal cual, en español.
 */
@Composable
fun PanelesGenericos(botId: Long, m: ModuloInfo) {
    Column(Modifier.fillMaxWidth().testTag("paneles_modulo"), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
        m.paneles.forEach { PanelUno(botId, m, it) }
    }
}

private fun valorInicial(f: CampoModulo): String = when (val d = f.defecto) {
    null -> ""
    is Double -> if (d == Math.floor(d)) d.toLong().toString() else d.toString()
    else -> d.toString()
}

@Composable
private fun PanelUno(botId: Long, m: ModuloInfo, p: PanelModulo) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    var version by remember(p.id) { mutableIntStateOf(0) }
    val filas by produceState<List<FilaPanel>>(emptyList(), m.id, p.id, version) {
        value = runCatching { Repo.filasPanelModulo(botId, m.id, p.id) }.getOrDefault(emptyList())
    }
    val valores = remember(m.id, p.id) { mutableStateMapOf<String, String>().also { mm -> p.campos.forEach { mm[it.clave] = valorInicial(it) } } }
    val archivos = remember(m.id, p.id) { mutableStateListOf<AdjuntoSubido>() }
    var mensaje by remember(m.id, p.id) { mutableStateOf<Pair<Boolean, String>?>(null) } // (esError, texto)
    var confirmar by remember(m.id, p.id) { mutableStateOf<Pair<BotonPanel, Map<String, Any?>>?>(null) }
    var ocupado by remember(m.id, p.id) { mutableStateOf(false) }

    fun args(): Map<String, Any?> {
        val a = LinkedHashMap<String, Any?>()
        p.campos.forEach { f ->
            val t = valores[f.clave].orEmpty().trim()
            when (f.tipo) {
                TipoCampo.NUMERO -> t.replace(',', '.').toDoubleOrNull()?.let { a[f.clave] = it } ?: run { a[f.clave] = t }
                TipoCampo.INTERRUPTOR -> a[f.clave] = t == "true"
                TipoCampo.ADJUNTOS -> a[f.clave] = archivos.map { mapOf("id" to it.id, "tipo" to it.tipo, "mime" to it.mime, "nombre" to it.nombre, "bytes" to it.bytes) }
                else -> a[f.clave] = valores[f.clave].orEmpty()
            }
        }
        return a
    }

    fun ejecuta(accion: String, a: Map<String, Any?>, limpiar: Boolean) {
        ocupado = true
        scope.launch {
            runCatching { Repo.accionModulo(botId, m.id, accion, a) }
                .onSuccess {
                    mensaje = false to it
                    if (limpiar) { p.campos.forEach { f -> if (f.tipo == TipoCampo.TEXTO) valores[f.clave] = valorInicial(f) }; archivos.clear() }
                    version++
                    runCatching { AlarmasModulos.reprogramarAsync(ctx) }
                }
                .onFailure { mensaje = true to (it.message ?: "No se pudo hacer") }
            ocupado = false
        }
    }

    fun pulsa(b: BotonPanel, a: Map<String, Any?>, limpiar: Boolean = false) {
        haptica(Toque.Confirmar)
        if (b.confirma.isNotBlank()) confirmar = b to (a + mapOf("__limpiar" to limpiar)) else ejecuta(b.accion, a, limpiar)
    }

    Card(Modifier.fillMaxWidth().testTag("panel_${p.id}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Text(p.titulo, style = MaterialTheme.typography.titleMedium)
            if (p.ayuda.isNotBlank()) Text(p.ayuda, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)

            // renglones
            if (filas.isEmpty()) Text("Todavía no hay nada aquí.", Modifier.testTag("panel_vacio_${p.id}"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            filas.forEach { fila ->
                Column(Modifier.fillMaxWidth().testTag("fila_${p.id}_${fila.id}"), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    Text(fila.titulo, style = MaterialTheme.typography.titleSmall)
                    if (fila.detalle.isNotBlank()) Text(fila.detalle, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    if (p.porFila.isNotEmpty()) FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                        p.porFila.forEach { b -> TextButton({ pulsa(b, mapOf("id" to fila.id)) }, Modifier.testTag("boton_${b.accion}_${fila.id}"), enabled = !ocupado) { Text(b.titulo) } }
                    }
                }
            }

            // formulario de alta
            if (p.campos.isNotEmpty()) {
                p.campos.forEach { f -> CampoDePanel(botId, m.id, p.id, f, valores, archivos) }
                Button({ pulsa(BotonPanel("${p.id}.agregar", p.alta), args(), limpiar = true) }, Modifier.fillMaxWidth().testTag("alta_${p.id}"), enabled = !ocupado) { Text(p.alta.ifBlank { "Agregar" }) }
                if (p.extras.isNotEmpty()) FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    p.extras.forEach { b -> OutlinedButton({ pulsa(b, args()) }, Modifier.testTag("extra_${b.accion}"), enabled = !ocupado) { Text(b.titulo) } }
                }
            }
            if (p.generales.isNotEmpty()) FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                p.generales.forEach { b -> OutlinedButton({ pulsa(b, emptyMap()) }, Modifier.testTag("general_${b.accion}"), enabled = !ocupado) { Text(b.titulo) } }
            }
            mensaje?.let { (esError, texto) ->
                Text(texto, Modifier.testTag(if (esError) "panel_error" else "panel_ok"), style = MaterialTheme.typography.bodyMedium, color = if (esError) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.primary)
            }
        }
    }

    confirmar?.let { (b, a) ->
        AlertDialog(
            onDismissRequest = { confirmar = null },
            title = { Text(b.titulo) },
            text = { Text(b.confirma, Modifier.testTag("panel_confirma_texto")) },
            confirmButton = { TextButton({ confirmar = null; ejecuta(b.accion, a.filterKeys { it != "__limpiar" }, a["__limpiar"] == true) }, Modifier.testTag("panel_confirma_si")) { Text("Sí, seguir") } },
            dismissButton = { TextButton({ confirmar = null }, Modifier.testTag("panel_confirma_no")) { Text("No") } },
        )
    }
}

@Composable
private fun CampoDePanel(botId: Long, modId: String, panelId: String, f: CampoModulo, valores: MutableMap<String, String>, archivos: MutableList<AdjuntoSubido>) {
    val valor = valores[f.clave].orEmpty()
    Column(Modifier.fillMaxWidth().testTag("pc_${panelId}_${f.clave}"), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
        when (f.tipo) {
            TipoCampo.INTERRUPTOR -> Row(verticalAlignment = Alignment.CenterVertically) {
                Text(f.titulo, Modifier.weight(1f), style = MaterialTheme.typography.titleSmall)
                Switch(valor == "true", { valores[f.clave] = it.toString() }, Modifier.testTag("pcsw_${panelId}_${f.clave}"))
            }
            TipoCampo.LISTA -> {
                Text(f.titulo, style = MaterialTheme.typography.titleSmall)
                if (f.opciones.size <= 5) FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    f.opciones.forEach { (v, t) -> FilterChip(valor == v, { valores[f.clave] = v }, { Text(t, maxLines = 1) }, modifier = Modifier.testTag("pcop_${panelId}_${f.clave}_$v")) }
                } else {
                    var abierto by remember { mutableStateOf(false) }
                    Box {
                        OutlinedButton({ abierto = true }, Modifier.fillMaxWidth().testTag("pcsel_${panelId}_${f.clave}")) {
                            Text(f.opciones.firstOrNull { it.first == valor }?.second ?: "Elegir…", maxLines = 1)
                        }
                        DropdownMenu(abierto, { abierto = false }) {
                            f.opciones.forEach { (v, t) -> DropdownMenuItem({ Text(t, maxLines = 1) }, { valores[f.clave] = v; abierto = false }, Modifier.testTag("pcitem_${panelId}_${f.clave}_$v")) }
                        }
                    }
                }
                if (f.ayuda.isNotBlank()) Text(f.ayuda, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            TipoCampo.ADJUNTOS -> {
                Text(f.titulo, style = MaterialTheme.typography.titleSmall)
                SelectorAdjuntos(botId, archivos.toList(), "panel_${modId}_$panelId", alCambiar = { n -> archivos.clear(); archivos.addAll(n) })
            }
            else -> {
                Text(f.titulo, style = MaterialTheme.typography.titleSmall)
                OutlinedTextField(
                    valor, { valores[f.clave] = it }, Modifier.fillMaxWidth().testTag("pcin_${panelId}_${f.clave}"),
                    singleLine = !f.varias, minLines = if (f.varias) 3 else 1,
                    keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = if (f.tipo == TipoCampo.NUMERO) KeyboardType.Number else KeyboardType.Text),
                    supportingText = { if (f.ayuda.isNotBlank()) Text(f.ayuda) },
                )
            }
        }
    }
}
