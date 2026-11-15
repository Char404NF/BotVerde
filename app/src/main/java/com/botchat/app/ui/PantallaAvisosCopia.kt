@file:OptIn(ExperimentalLayoutApi::class, ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
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
import androidx.compose.ui.text.input.PasswordVisualTransformation
import com.botchat.app.datos.AjustesAvisos
import com.botchat.app.datos.Repo
import com.botchat.app.datos.Restaurado
import com.botchat.app.datos.ajustesAvisos
import com.botchat.app.datos.exportarConfig
import com.botchat.app.datos.guardarAjustesAvisos
import com.botchat.app.datos.resumenSemanalAhora
import com.botchat.app.datos.restaurarConfig
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

private val TIPOS_AVISO = listOf(
    "reporte" to "Reportes", "grupos" to "Grupos", "conexion" to "Conexión", "resumen" to "Resumen",
    "filtros" to "Filtros", "advertencias" to "Advertencias",
)
private val MINUTOS_DESCONEXION = listOf(0 to "No avisar", 5 to "5 min", 10 to "10 min", 30 to "30 min", 60 to "1 hora")

/** Avisos importantes (también por WhatsApp), aviso de desconexión, resumen semanal y la copia de seguridad cifrada de la configuración. */
@Composable
fun PantallaAvisosCopia(botId: Long, alVolver: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    var ajustes by remember { mutableStateOf<AjustesAvisos?>(null) } // se conserva mientras se vuelve a leer
    LaunchedEffect(version) { ajustes = runCatching { Repo.ajustesAvisos(botId) }.getOrNull() ?: ajustes }
    var resumen by remember { mutableStateOf<String?>(null) }
    fun guardar(nuevo: AjustesAvisos) = scope.launch {
        runCatching { Repo.guardarAjustesAvisos(botId, nuevo) }.onFailure { avisaToast(ctx, textoError(it), true) }
        version++
    }

    // Copia de seguridad: la contraseña se pide en un cuadro y nunca se guarda ni se muestra.
    var pidiendoClaveGuardar by remember { mutableStateOf(false) }
    var copiaLista by remember { mutableStateOf<ByteArray?>(null) }
    var archivoLeido by remember { mutableStateOf<ByteArray?>(null) }
    var restaurado by remember { mutableStateOf<Restaurado?>(null) }
    val crear = rememberLauncherForActivityResult(ActivityResultContracts.CreateDocument("application/octet-stream")) { uri ->
        val datos = copiaLista
        copiaLista = null
        if (uri != null && datos != null) scope.launch {
            runCatching { withContext(Dispatchers.IO) { ctx.contentResolver.openOutputStream(uri)?.use { it.write(datos) } ?: error("No se pudo escribir el archivo") } }
                .onSuccess { avisaToast(ctx, "Copia guardada", false) }.onFailure { avisaToast(ctx, textoError(it), true) }
        }
    }
    val abrir = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        if (uri != null) scope.launch {
            runCatching { withContext(Dispatchers.IO) { ctx.contentResolver.openInputStream(uri)?.use { it.readBytes() } ?: error("No se pudo leer el archivo") } }
                .onSuccess { archivoLeido = it }.onFailure { avisaToast(ctx, textoError(it), true) }
        }
    }

    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver, Modifier.testTag("volver_avisos_copia")) { Icon(Ico.Atras, "Volver") } }, title = { Text("Avisos y copia de seguridad") }) }) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l).testTag("pantalla_avisos_copia"), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            TituloSeccionE8("Avisos")
            val a = ajustes
            if (a == null) FilasDeCarga(3) else Card(Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) {
                            Text("Repetir los avisos importantes en mi WhatsApp", style = MaterialTheme.typography.bodyMedium)
                            Text("Llegan a tu chat «Mensajes guardados», además de la notificación del teléfono. Máximo 12 por hora.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                        Switch(a.wa, { haptica(Toque.Suave); guardar(a.copy(wa = it)) }, Modifier.testTag("sw_alerta_wa"))
                    }
                    if (a.wa) {
                        Text("Cuáles", style = MaterialTheme.typography.labelLarge)
                        FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                            TIPOS_AVISO.forEach { (id, nombre) ->
                                FilterChip(id in a.tipos, { haptica(Toque.Suave); guardar(a.copy(tipos = if (id in a.tipos) a.tipos - id else a.tipos + id)) }, { Text(nombre) }, modifier = Modifier.testTag("alerta_tipo_$id"))
                            }
                        }
                    }
                    Text("Avisarme si el bot pierde la conexión más de…", style = MaterialTheme.typography.labelLarge)
                    FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                        MINUTOS_DESCONEXION.forEach { (m, texto) ->
                            FilterChip(a.desconexion == m, { haptica(Toque.Suave); guardar(a.copy(desconexion = m)) }, { Text(texto) }, modifier = Modifier.testTag("desconexion_$m"))
                        }
                    }
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) {
                            Text("Resumen de la semana", style = MaterialTheme.typography.bodyMedium)
                            Text("Los domingos en la noche: mensajes por grupo, quién entró y salió y lo que hicieron los filtros. Sin textos ni números.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                        Switch(a.resumen, { haptica(Toque.Suave); guardar(a.copy(resumen = it)) }, Modifier.testTag("sw_resumen_semanal"))
                    }
                    OutlinedButton({
                        scope.launch {
                            runCatching { Repo.resumenSemanalAhora(botId) }.onSuccess { t -> if (t.isBlank()) avisaToast(ctx, "Aún no hay nada que contar esta semana", false) else resumen = t }.onFailure { avisaToast(ctx, textoError(it), true) }
                        }
                    }, Modifier.fillMaxWidth().testTag("ver_resumen_ahora")) { Text("Ver el resumen de ahora") }
                }
            }

            TituloSeccionE8("Copia de seguridad")
            Card(Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text(
                        "Guarda tu configuración en un archivo cifrado con una contraseña que tú eliges: rangos de grupos, filtros, reglas, horarios, protección de entrada, baneados, respuestas y ajustes. No lleva mensajes, fotos ni llaves.",
                        style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    Text("Si cambias de teléfono, vincula el bot y restaura el archivo con la misma contraseña. Si la olvidas, no hay forma de abrirlo.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Button({ haptica(Toque.Suave); pidiendoClaveGuardar = true }, Modifier.fillMaxWidth().testTag("copia_guardar")) { Text("Guardar una copia…") }
                    OutlinedButton({ haptica(Toque.Suave); abrir.launch(arrayOf("*/*")) }, Modifier.fillMaxWidth().testTag("copia_restaurar")) { Text("Restaurar una copia…") }
                }
            }
        }
    }

    resumen?.let { t ->
        AlertDialog(onDismissRequest = { resumen = null }, title = { Text("Resumen de la semana") }, text = { Text(t, Modifier.testTag("texto_resumen")) },
            confirmButton = { TextButton({ resumen = null }) { Text("Listo") } })
    }
    if (pidiendoClaveGuardar) DialogoClaveCopia(true, { pidiendoClaveGuardar = false }) { clave ->
        pidiendoClaveGuardar = false
        scope.launch {
            runCatching { Repo.exportarConfig(botId, clave) }
                .onSuccess { copiaLista = it; crear.launch("botchat-configuracion-${SimpleDateFormat("yyyy-MM-dd", Locale.US).format(Date())}.bccfg") }
                .onFailure { avisaToast(ctx, textoError(it), true) }
        }
    }
    archivoLeido?.let { datos ->
        DialogoClaveCopia(false, { archivoLeido = null }) { clave ->
            archivoLeido = null
            scope.launch { runCatching { Repo.restaurarConfig(botId, clave, datos) }.onSuccess { restaurado = it; version++ }.onFailure { avisaToast(ctx, textoError(it), true) } }
        }
    }
    restaurado?.let { r ->
        AlertDialog(
            onDismissRequest = { restaurado = null }, title = { Text("Copia restaurada") },
            text = { Text("Se pusieron ${plural(r.ajustes.toLong(), "ajuste", "ajustes")}, ${plural(r.grupos.toLong(), "grupo", "grupos")} con ${plural(r.filtros.toLong(), "filtro", "filtros")} y ${plural(r.baneados.toLong(), "baneo", "baneos")}" + (if (r.base) ", y la configuración base." else "."), Modifier.testTag("texto_restaurado")) },
            confirmButton = { TextButton({ restaurado = null }, Modifier.testTag("restaurado_listo")) { Text("Listo") } },
        )
    }
}

/** Pide la contraseña de la copia (dos veces al guardar, para no equivocarse). */
@Composable
private fun DialogoClaveCopia(guardando: Boolean, alCerrar: () -> Unit, alListo: (String) -> Unit) {
    var clave by remember { mutableStateOf("") }
    var repetida by remember { mutableStateOf("") }
    val valida = claveCopiaValida(clave, repetida, guardando)
    AlertDialog(
        onDismissRequest = alCerrar, title = { Text(if (guardando) "Contraseña de la copia" else "Contraseña de la copia guardada") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                if (guardando) Text("Elige una contraseña de al menos 8 caracteres. Anótala: sin ella no se puede abrir.", style = MaterialTheme.typography.bodySmall)
                OutlinedTextField(clave, { clave = it }, Modifier.fillMaxWidth().testTag("copia_clave"), singleLine = true, label = { Text("Contraseña") }, visualTransformation = PasswordVisualTransformation())
                if (guardando) OutlinedTextField(repetida, { repetida = it }, Modifier.fillMaxWidth().testTag("copia_clave2"), singleLine = true, label = { Text("Otra vez") }, visualTransformation = PasswordVisualTransformation(),
                    isError = repetida.isNotEmpty() && repetida != clave)
            }
        },
        confirmButton = { TextButton({ alListo(clave) }, Modifier.testTag("copia_continuar"), enabled = valida) { Text(if (guardando) "Guardar" else "Restaurar") } },
        dismissButton = { TextButton(alCerrar) { Text("Cancelar") } },
    )
}

/** La contraseña de la copia: al menos 8 caracteres y, al guardar, escrita igual dos veces. */
internal fun claveCopiaValida(clave: String, repetida: String, guardando: Boolean) = clave.length >= 8 && (!guardando || clave == repetida)
