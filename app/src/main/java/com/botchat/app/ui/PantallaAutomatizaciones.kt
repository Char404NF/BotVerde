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
import com.botchat.app.ui.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import com.botchat.app.ui.DropdownMenu
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
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/** Automatizaciones: las respuestas y menús del bot. Cada tarjeta se enciende, se duplica, se prueba o se borra. */
@Composable
fun PantallaAutomatizaciones(
    botId: Long, alVolver: () -> Unit, alEditar: (Long) -> Unit, alEnCurso: () -> Unit, alActividad: () -> Unit,
    alEstadisticas: (Long) -> Unit,
    alFormularios: () -> Unit = {}, alNoEntendio: () -> Unit = {}, alAjustesMenus: () -> Unit = {}, alMensajesAuto: () -> Unit = {},
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
    var masMenu by remember { mutableStateOf(false) }
    var exportando by remember { mutableStateOf(false) }
    var archivoImportar by remember { mutableStateOf<java.io.File?>(null) }
    val cuentaForms by produceState(0L, version) { value = runCatching { Repo.contarFormularios(botId) }.getOrDefault(0L) }

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

    val importarBcm = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        if (uri != null) scope.launch {
            runCatching {
                withContext(Dispatchers.IO) {
                    val dir = java.io.File(ctx.cacheDir, "adjuntos").apply { mkdirs() }
                    java.io.File(dir, "importar_${System.nanoTime()}.bcm").also { f -> ctx.contentResolver.openInputStream(uri)?.use { i -> f.outputStream().use { o -> i.copyTo(o) } } ?: error("No se pudo leer el archivo") }
                }
            }.onSuccess { archivoImportar = it }.onFailure { avisaToast(ctx, it.message ?: "No se pudo leer el archivo", true) }
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } },
                title = { Text("Automatizaciones") },
                actions = {
                    IconButton(alMensajesAuto, Modifier.testTag("abrir_mensajes_auto")) { Icon(Ico.Editar, "Mensajes automáticos") }
                    IconButton(alEnCurso, Modifier.testTag("abrir_en_curso")) { Icon(Ico.Chats, "En curso ahora") }
                    IconButton(alActividad, Modifier.testTag("abrir_actividad_auto")) { Icon(Ico.Estadisticas, "Actividad del bot") }
                    Box {
                        IconButton({ masMenu = true }, Modifier.testTag("mas_automatizaciones")) { Icon(Ico.Mas, "Más") }
                        DropdownMenu(masMenu, { masMenu = false }) {
                            DropdownMenuItem(text = { Text(if (cuentaForms > 0) "Formularios recibidos ($cuentaForms)" else "Formularios recibidos") }, onClick = { masMenu = false; alFormularios() }, modifier = Modifier.testTag("abrir_formularios"))
                            DropdownMenuItem(text = { Text("Lo que no entendió") }, onClick = { masMenu = false; alNoEntendio() }, modifier = Modifier.testTag("abrir_no_entendio"))
                            DropdownMenuItem(text = { Text("Ajustes de los menús") }, onClick = { masMenu = false; alAjustesMenus() }, modifier = Modifier.testTag("abrir_ajustes_menus"))
                            DropdownMenuItem(text = { Text("Exportar menús (archivo cifrado)") }, onClick = { masMenu = false; exportando = !exportando }, modifier = Modifier.testTag("abrir_exportar_menus"))
                            DropdownMenuItem(text = { Text("Importar menús (archivo cifrado)") }, onClick = { masMenu = false; importarBcm.launch(arrayOf("*/*")) }, modifier = Modifier.testTag("abrir_importar_menus"))
                        }
                    }
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
            Card(onClick = alMensajesAuto, modifier = Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.xs).testTag("tarjeta_mensajes_auto"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.secondaryContainer)) {
                Row(Modifier.padding(Esp.m), verticalAlignment = Alignment.CenterVertically) {
                    Icon(Ico.Editar, null)
                    Column(Modifier.weight(1f).padding(start = Esp.m)) {
                        Text("Mensajes automáticos", style = MaterialTheme.typography.titleSmall)
                        Text("Bienvenida, ausencia, “ya vi tu mensaje”, cierre y respuestas rápidas (/atajo).", style = MaterialTheme.typography.bodySmall)
                    }
                    Icon(Ico.Siguiente, null)
                }
            }
            if (exportando) PanelExportar(botId, lista.orEmpty().filter { !it.sistema }, alCerrar = { exportando = false })
            archivoImportar?.let { f -> PanelImportar(botId, f, alTerminar = { archivoImportar = null; version++ }) }
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
                            alEstadisticas = { alEstadisticas(a.id) }, alBorrar = { borrando = a },
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

/** Exportar menús: se eligen, se les puede poner contraseña (para abrirlos en otro teléfono) y se comparte el archivo cifrado. */
@Composable
private fun PanelExportar(botId: Long, menus: List<AutoResumen>, alCerrar: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val elegidos = remember { androidx.compose.runtime.mutableStateListOf<Long>() }
    var clave by remember { mutableStateOf("") }
    var trabajando by remember { mutableStateOf(false) }
    Card(Modifier.fillMaxWidth().padding(horizontal = Esp.l).testTag("panel_exportar"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.secondaryContainer)) {
        Column(Modifier.padding(Esp.m), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Text("Exportar menús", style = MaterialTheme.typography.titleSmall)
            Text("Se guardan en un archivo cifrado (con sus fotos y archivos). Con contraseña lo puedes abrir en otro teléfono; sin contraseña solo sirve como copia de seguridad en este.", style = MaterialTheme.typography.bodySmall)
            if (menus.isEmpty()) Text("Todavía no hay menús para exportar.", style = MaterialTheme.typography.bodySmall)
            menus.forEach { a ->
                Row(Modifier.fillMaxWidth().clickable { if (a.id in elegidos) elegidos.remove(a.id) else elegidos.add(a.id) }.testTag("exportar_menu_${a.id}"), verticalAlignment = Alignment.CenterVertically) {
                    androidx.compose.material3.Checkbox(a.id in elegidos, null)
                    Text(a.nombre, Modifier.padding(start = 8.dp), maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
            androidx.compose.material3.OutlinedTextField(clave, { clave = it }, Modifier.fillMaxWidth().testTag("exportar_clave"), label = { Text("Contraseña (opcional, mínimo 6)") }, singleLine = true,
                visualTransformation = androidx.compose.ui.text.input.PasswordVisualTransformation())
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                OutlinedButton({
                    trabajando = true
                    scope.launch {
                        runCatching { Repo.exportarMenus(botId, elegidos.toList(), clave) }.onSuccess { ruta -> compartirCifrado(ctx, ruta, "menus", "bcm"); alCerrar() }.onFailure { avisaToast(ctx, it.message ?: "No se pudo exportar", true) }
                        trabajando = false
                    }
                }, Modifier.testTag("exportar_menus"), enabled = elegidos.isNotEmpty() && !trabajando) { Text(if (trabajando) "Preparando…" else "Exportar") }
                TextButton(alCerrar, Modifier.testTag("cerrar_exportar_menus")) { Text("Cancelar") }
            }
        }
    }
}

/** Importar menús: el archivo se lee y se crean los menús APAGADOS (no se ejecuta nada hasta que los enciendas). */
@Composable
private fun PanelImportar(botId: Long, archivo: java.io.File, alTerminar: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var clave by remember { mutableStateOf("") }
    var trabajando by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    Card(Modifier.fillMaxWidth().padding(horizontal = Esp.l).testTag("panel_importar"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.secondaryContainer)) {
        Column(Modifier.padding(Esp.m), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Text("Importar menús", style = MaterialTheme.typography.titleSmall)
            Text("Si el archivo se exportó con contraseña, escríbela. Los menús llegan apagados y sin chats asignados; revísalos antes de encenderlos.", style = MaterialTheme.typography.bodySmall)
            androidx.compose.material3.OutlinedTextField(clave, { clave = it; error = null }, Modifier.fillMaxWidth().testTag("importar_clave"), label = { Text("Contraseña (si tiene)") }, singleLine = true,
                visualTransformation = androidx.compose.ui.text.input.PasswordVisualTransformation(), isError = error != null)
            error?.let { Text(it, color = MaterialTheme.colorScheme.error, modifier = Modifier.testTag("error_importar")) }
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                OutlinedButton({
                    trabajando = true
                    scope.launch {
                        // la copia en claro se borra al intentar; si la contraseña falla se vuelve a copiar
                        val copia = withContext(Dispatchers.IO) { java.io.File(archivo.parentFile, "reintento_${System.nanoTime()}.bcm").also { archivo.copyTo(it, overwrite = true) } }
                        runCatching { Repo.importarMenus(botId, copia.path, clave) }.onSuccess { r ->
                            archivo.delete()
                            avisaToast(ctx, if (r.ids.size == 1) "Se importó “${r.nombres[0]}” (apagado)" else "Se importaron ${r.ids.size} menús (apagados)" + if (r.omitidos > 0) ". ${r.omitidos} no se pudieron leer" else "", true)
                            alTerminar()
                        }.onFailure { error = it.message ?: "No se pudo importar" }
                        trabajando = false
                    }
                }, Modifier.testTag("importar_menus"), enabled = !trabajando) { Text(if (trabajando) "Importando…" else "Importar") }
                TextButton({ archivo.delete(); alTerminar() }, Modifier.testTag("cancelar_importar_menus")) { Text("Cancelar") }
            }
        }
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
    alEstadisticas: () -> Unit, alBorrar: () -> Unit,
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
                    DropdownMenuItem(text = { Text("Estadísticas") }, onClick = { menu = false; alEstadisticas() }, modifier = Modifier.testTag("estadisticas_${a.id}"))
                    DropdownMenuItem(text = { Text("Duplicar") }, onClick = { menu = false; alDuplicar() }, modifier = Modifier.testTag("duplicar_${a.id}"))
                    DropdownMenuItem(text = { Text("Compartir (JSON)") }, onClick = { menu = false; alExportar() })
                    DropdownMenuItem(text = { Text("Borrar", color = MaterialTheme.colorScheme.error) }, onClick = { menu = false; alBorrar() }, modifier = Modifier.testTag("borrar_${a.id}"))
                }
            }
        }
    }
}
