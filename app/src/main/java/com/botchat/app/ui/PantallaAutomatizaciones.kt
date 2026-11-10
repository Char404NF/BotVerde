@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import android.content.Intent
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
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
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ExtendedFloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.AutoDef
import com.botchat.app.datos.AutoResumen
import com.botchat.app.datos.PlantillaAuto
import com.botchat.app.datos.Repo
import kotlinx.coroutines.launch

/** Automatizaciones: las respuestas y menús del bot. Cada tarjeta se enciende, se duplica, se prueba o se borra. */
@Composable
fun PantallaAutomatizaciones(
    botId: Long, alVolver: () -> Unit, alEditar: (Long) -> Unit, alEnCurso: () -> Unit, alActividad: () -> Unit,
    alEstadisticas: (Long) -> Unit, alProbar: (Long) -> Unit,
) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("automatizaciones" in evs) version++ } }
    val lista by produceState<List<AutoResumen>?>(null, version) { value = runCatching { Repo.automatizaciones(botId) }.getOrDefault(emptyList()) }
    var nueva by remember { mutableStateOf(false) }
    var plantillas by remember { mutableStateOf<List<PlantillaAuto>?>(null) }
    var borrando by remember { mutableStateOf<AutoResumen?>(null) }

    val importar = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        if (uri != null) scope.launch {
            try {
                val texto = ctx.contentResolver.openInputStream(uri)?.bufferedReader()?.use { it.readText() } ?: error("No se pudo leer el archivo")
                val (id, _, avisos) = Repo.importarAutomatizacion(botId, texto)
                avisaToast(ctx, if (avisos.any { it.grave }) "Importada, pero apagada: tiene avisos por corregir" else "Importada (apagada hasta que la enciendas)", true)
                alEditar(id)
            } catch (e: Exception) { avisaToast(ctx, e.message ?: "Ese archivo no es una automatización", true) }
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } },
                title = { Text("Automatizaciones") },
                actions = {
                    IconButton(alEnCurso, Modifier.testTag("abrir_en_curso")) { Icon(Ico.Chats, "En curso ahora") }
                    IconButton(alActividad, Modifier.testTag("abrir_actividad_auto")) { Icon(Ico.Estadisticas, "Actividad del bot") }
                },
            )
        },
        floatingActionButton = { ExtendedFloatingActionButton({ haptica(Toque.Suave); nueva = !nueva }, Modifier.testTag("nueva_automatizacion")) { Icon(Ico.Agregar, null); Text("  Nueva") } },
    ) { relleno ->
        val datos = lista
        Column(Modifier.fillMaxSize().padding(relleno)) {
            Text(
                "El bot solo responde cuando alguien le escribe; nunca escribe primero. Se revisan de arriba hacia abajo y responde la primera que coincida (el Saludo va al final).",
                Modifier.padding(horizontal = Esp.l, vertical = Esp.s), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            if (nueva) PanelNueva(
                alCero = { nueva = false; scope.launch { runCatching { Repo.guardarAutomatizacion(botId, 0, AutoDef.nueva()) }.onSuccess { alEditar(it.first) }.onFailure { avisaToast(ctx, it.message, true) } } },
                alPlantillas = { scope.launch { plantillas = runCatching { Repo.plantillasAutomatizacion() }.getOrDefault(emptyList()) } },
                alImportar = { nueva = false; importar.launch(arrayOf("application/json", "text/plain", "*/*")) },
                plantillas = plantillas,
                alElegir = { p -> nueva = false; plantillas = null; scope.launch { runCatching { Repo.guardarAutomatizacion(botId, 0, p.def) }.onSuccess { alEditar(it.first) }.onFailure { avisaToast(ctx, it.message, true) } } },
            )
            when {
                datos == null -> Box(Modifier.fillMaxSize(), Alignment.Center) { CircularProgressIndicator() }
                datos.isEmpty() -> EstadoVacio(Ico.Chats, "Sin automatizaciones", "Toca Nueva para armar un menú o una respuesta automática.", Modifier.padding(top = 60.dp))
                else -> LazyColumn(Modifier.fillMaxSize().testTag("lista_automatizaciones"), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.l, Esp.s, Esp.l, 96.dp), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
                    items(datos, key = { it.id }) { a ->
                        TarjetaAutomatizacion(
                            a, alEditar = { alEditar(a.id) },
                            alActivar = { si -> scope.launch { runCatching { Repo.activarAutomatizacion(botId, a.id, si) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo cambiar", true) }; version++ } },
                            alDuplicar = { scope.launch { runCatching { Repo.duplicarAutomatizacion(botId, a.id) }.onSuccess { avisaToast(ctx, "Copia creada (apagada)", false) }; version++ } },
                            alExportar = {
                                scope.launch {
                                    runCatching { Repo.exportarAutomatizacion(botId, a.id) }.onSuccess { json ->
                                        val i = Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_SUBJECT, a.nombre).putExtra(Intent.EXTRA_TEXT, json)
                                        ctx.startActivity(Intent.createChooser(i, "Compartir automatización").addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
                                    }.onFailure { avisaToast(ctx, it.message, true) }
                                }
                            },
                            alEstadisticas = { alEstadisticas(a.id) }, alProbar = { alProbar(a.id) }, alBorrar = { borrando = a },
                        )
                    }
                }
            }
        }
    }
    borrando?.let { a ->
        AlertDialog(
            onDismissRequest = { borrando = null },
            title = { Text("¿Borrar “${a.nombre}”?") },
            text = { Text("Se cierran las conversaciones que estén en curso con ella. Esto no se puede deshacer.") },
            confirmButton = { TextButton({ borrando = null; scope.launch { runCatching { Repo.borrarAutomatizacion(botId, a.id) }; version++ } }, Modifier.testTag("confirmar_borrar_auto")) { Text("Borrar", color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton({ borrando = null }) { Text("Cancelar") } },
        )
    }
}

@Composable
private fun PanelNueva(alCero: () -> Unit, alPlantillas: () -> Unit, alImportar: () -> Unit, plantillas: List<PlantillaAuto>?, alElegir: (PlantillaAuto) -> Unit) {
    Card(Modifier.fillMaxWidth().padding(horizontal = Esp.l).testTag("panel_nueva"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.secondaryContainer)) {
        Column(Modifier.padding(Esp.m), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                OutlinedButton(alCero, Modifier.testTag("nueva_desde_cero")) { Text("Desde cero") }
                OutlinedButton(alPlantillas, Modifier.testTag("nueva_desde_plantilla")) { Text("Plantilla") }
                OutlinedButton(alImportar, Modifier.testTag("nueva_importar")) { Text("Importar") }
            }
            plantillas?.forEach { p ->
                Column(Modifier.fillMaxWidth().clickable { alElegir(p) }.padding(vertical = 6.dp).testTag("plantilla_${p.nombre}")) {
                    Text(p.nombre, fontWeight = FontWeight.SemiBold)
                    Text(p.descripcion, style = MaterialTheme.typography.bodySmall)
                }
            }
        }
    }
}

@Composable
private fun TarjetaAutomatizacion(
    a: AutoResumen, alEditar: () -> Unit, alActivar: (Boolean) -> Unit, alDuplicar: () -> Unit, alExportar: () -> Unit,
    alEstadisticas: () -> Unit, alProbar: () -> Unit, alBorrar: () -> Unit,
) {
    var menu by remember { mutableStateOf(false) }
    Card(Modifier.fillMaxWidth().clickable { alEditar() }.testTag("auto_${a.id}"), shape = MaterialTheme.shapes.large, colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Row(Modifier.padding(start = Esp.l, top = Esp.m, bottom = Esp.m, end = Esp.s), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text(a.nombre, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                Text("${a.resumen} · ${if (a.pasos == 1) "1 paso" else "${a.pasos} pasos"}" + if (a.sistema) " · De fábrica" else "", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                if (a.avisos > 0) Text(if (a.avisos == 1) "1 aviso por corregir" else "${a.avisos} avisos por corregir", Modifier.testTag("avisos_${a.id}"), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.error)
            }
            Switch(a.activo, alActivar, Modifier.testTag("activa_${a.id}"))
            Box {
                IconButton({ menu = true }, Modifier.testTag("mas_${a.id}")) { Icon(Ico.Mas, "Más") }
                DropdownMenu(menu, { menu = false }) {
                    DropdownMenuItem(text = { Text("Editar") }, onClick = { menu = false; alEditar() })
                    DropdownMenuItem(text = { Text("Probar sin enviar nada") }, onClick = { menu = false; alProbar() }, modifier = Modifier.testTag("probar_${a.id}"))
                    DropdownMenuItem(text = { Text("Estadísticas") }, onClick = { menu = false; alEstadisticas() }, modifier = Modifier.testTag("estadisticas_${a.id}"))
                    DropdownMenuItem(text = { Text("Duplicar") }, onClick = { menu = false; alDuplicar() }, modifier = Modifier.testTag("duplicar_${a.id}"))
                    DropdownMenuItem(text = { Text("Compartir (JSON)") }, onClick = { menu = false; alExportar() })
                    DropdownMenuItem(text = { Text("Borrar", color = MaterialTheme.colorScheme.error) }, onClick = { menu = false; alBorrar() }, modifier = Modifier.testTag("borrar_${a.id}"))
                }
            }
        }
    }
}
