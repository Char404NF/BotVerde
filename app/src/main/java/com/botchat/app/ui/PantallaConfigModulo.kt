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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
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
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.CampoModulo
import com.botchat.app.datos.ModuloInfo
import com.botchat.app.datos.Repo
import com.botchat.app.datos.TipoCampo
import kotlinx.coroutines.launch

/**
 * Registro de ventanas de configuración propias. Un módulo que necesite algo más que campos simples llama a
 * `PantallasModulo.registrar("mi_modulo") { botId, info -> ... }` (por ejemplo desde un `init` de su archivo o al arrancar la app) y esa
 * ventana reemplaza a la genérica. Lo que dibuja va dentro de la columna con scroll de la pantalla del módulo, debajo de la barra con su
 * nombre; los límites propios (si escribe por iniciativa) los pone el marco debajo.
 */
object PantallasModulo {
    private val mapa = java.util.concurrent.ConcurrentHashMap<String, @Composable (botId: Long, info: ModuloInfo) -> Unit>()

    fun registrar(id: String, contenido: @Composable (botId: Long, info: ModuloInfo) -> Unit) { mapa[id] = contenido }
    fun quitar(id: String) { mapa.remove(id) }
    fun de(id: String): (@Composable (botId: Long, info: ModuloInfo) -> Unit)? = mapa[id]
    fun tiene(id: String) = mapa.containsKey(id)
}

/** Ventana de configuración de un módulo: la suya si la registró, o el contenedor genérico que dibuja su esquema de campos. */
@Composable
fun PantallaConfigModulo(botId: Long, id: String, alVolver: () -> Unit, alAbrirHistorial: () -> Unit = {}) {
    var recarga by remember { mutableIntStateOf(0) }
    LaunchedEffect(id) { Repo.cambios.collect { evs -> if ("modulos" in evs || (id == "descargas_video" && "descargas_video" in evs)) recarga++ } }
    val info by produceState<ModuloInfo?>(null, id, recarga) { value = runCatching { Repo.modulo(botId, id) }.getOrNull() ?: value }
    val m = info
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text(m?.nombre ?: "Módulo") }) }) { relleno ->
        if (m == null) { Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }; return@Scaffold }
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text(m.estado.ifBlank { if (m.activo) "Encendido" else "Apagado" }, style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.testTag("config_estado"))
            }
            Text(m.descripcion, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            if (id == "descargas_video") TarjetaMotorVideo(botId, alAbrirHistorial = alAbrirHistorial)
            val propia = PantallasModulo.de(id)
            if (propia != null) Box(Modifier.fillMaxWidth().testTag("config_propia")) { propia(botId, m) }
            else if (m.esquema.isNotEmpty()) ConfiguracionGenerica(botId, m)
            else if (m.paneles.isEmpty()) Text("Este módulo no tiene ajustes.", Modifier.testTag("config_sin_ajustes"), style = MaterialTheme.typography.bodyMedium)
            if (m.paneles.isNotEmpty()) PanelesGenericos(botId, m)
            if (m.iniciativa) LimitesDelModulo(botId, m)
        }
    }
}

// ---------- contenedor genérico ----------

private fun aTexto(v: Any?): String = when (v) {
    null -> ""
    is Double -> if (v == Math.floor(v) && !v.isInfinite()) v.toLong().toString() else v.toString()
    is Int -> v.toString()
    is Long -> v.toString()
    else -> v.toString()
}

/** Dibuja la configuración de un módulo a partir de su esquema: texto, número, interruptor, lista y hora. */
@Composable
fun ConfiguracionGenerica(botId: Long, m: ModuloInfo) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    val valores = remember(m.id) { mutableStateMapOf<String, String>() } // todo se edita como texto y se convierte al guardar
    val llaves = remember(m.id) { mutableStateMapOf<String, String>() } // lo que se escribe en los campos de llave (no se guarda aquí)
    val llaveGuardada = remember(m.id) { mutableStateMapOf<String, Boolean>() }
    var msgLlave by remember(m.id) { mutableStateOf<String?>(null) }
    var cargado by remember(m.id) { mutableStateOf(false) }
    var error by remember(m.id) { mutableStateOf<String?>(null) }
    var guardado by remember(m.id) { mutableStateOf(false) }
    LaunchedEffect(m.id) {
        val c = runCatching { Repo.configModulo(botId, m.id) }.getOrDefault(emptyMap())
        m.esquema.forEach { f ->
            if (f.tipo == TipoCampo.SECRETO) llaveGuardada[f.clave] = runCatching { com.botchat.app.seguridad.LlavesModulos.existe(ctx, botId, m.id, f.clave) }.getOrDefault(false)
            else valores[f.clave] = aTexto(c[f.clave] ?: f.defecto)
        }
        cargado = true
    }
    if (!cargado) { Box(Modifier.fillMaxWidth().padding(Esp.l), Alignment.Center) { CircularProgressIndicator() }; return }
    Column(Modifier.fillMaxWidth().testTag("config_generica"), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
        m.esquema.forEach { f ->
            Card(Modifier.fillMaxWidth().testTag("campo_${f.clave}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                    val valor = valores[f.clave].orEmpty()
                    when (f.tipo) {
                        TipoCampo.SECRETO -> {
                            val hay = llaveGuardada[f.clave] == true
                            Text(f.titulo, style = MaterialTheme.typography.titleSmall)
                            if (f.ayuda.isNotBlank()) Text(f.ayuda, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            Text(if (hay) "Hay una llave guardada en el Keystore de este teléfono (no se puede volver a ver)." else "Todavía no hay llave guardada.",
                                Modifier.testTag("llave_estado_${f.clave}"), style = MaterialTheme.typography.labelLarge, color = if (hay) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant)
                            OutlinedTextField(
                                llaves[f.clave].orEmpty(), { llaves[f.clave] = it; msgLlave = null }, Modifier.fillMaxWidth().testTag("in_${f.clave}"), singleLine = true,
                                visualTransformation = androidx.compose.ui.text.input.PasswordVisualTransformation(),
                                keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = KeyboardType.Password),
                                placeholder = { Text(if (hay) "Escribe otra para reemplazarla" else "Pega la llave aquí") },
                            )
                            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                                Button({
                                    haptica(Toque.Confirmar)
                                    val v = llaves[f.clave].orEmpty().trim()
                                    if (v.isEmpty()) { msgLlave = "Escribe la llave primero"; return@Button }
                                    scope.launch {
                                        runCatching { Repo.guardarLlaveModulo(ctx, botId, m.id, f.clave, v) }
                                            .onSuccess { llaves[f.clave] = ""; llaveGuardada[f.clave] = true; msgLlave = "Llave guardada en el Keystore" }
                                            .onFailure { msgLlave = it.message ?: "No se pudo guardar la llave" }
                                    }
                                }, Modifier.testTag("guardar_llave_${f.clave}")) { Text("Guardar llave") }
                                if (hay) OutlinedButton({
                                    haptica(Toque.Confirmar)
                                    scope.launch {
                                        runCatching { Repo.guardarLlaveModulo(ctx, botId, m.id, f.clave, "") }
                                            .onSuccess { llaveGuardada[f.clave] = false; msgLlave = "Llave borrada" }
                                            .onFailure { msgLlave = it.message ?: "No se pudo borrar la llave" }
                                    }
                                }, Modifier.testTag("borrar_llave_${f.clave}")) { Text("Borrar llave") }
                            }
                            msgLlave?.let { Text(it, Modifier.testTag("llave_msg"), style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.primary) }
                        }
                        TipoCampo.INTERRUPTOR -> Row(verticalAlignment = Alignment.CenterVertically) {
                            Column(Modifier.weight(1f)) {
                                Text(f.titulo, style = MaterialTheme.typography.titleSmall)
                                if (f.ayuda.isNotBlank()) Text(f.ayuda, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                            Switch(valor == "true", { haptica(Toque.Suave); valores[f.clave] = it.toString(); guardado = false }, Modifier.testTag("sw_${f.clave}"))
                        }
                        TipoCampo.LISTA -> {
                            Text(f.titulo, style = MaterialTheme.typography.titleSmall)
                            if (f.ayuda.isNotBlank()) Text(f.ayuda, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                                f.opciones.forEach { (v, t) -> FilterChip(valor == v, { haptica(Toque.Suave); valores[f.clave] = v; guardado = false }, { Text(t, maxLines = 1) }, modifier = Modifier.testTag("op_${f.clave}_$v")) }
                            }
                        }
                        else -> {
                            Text(f.titulo, style = MaterialTheme.typography.titleSmall)
                            OutlinedTextField(
                                valor, { valores[f.clave] = it; guardado = false }, Modifier.fillMaxWidth().testTag("in_${f.clave}"),
                                singleLine = !f.varias, minLines = if (f.varias) 3 else 1,
                                keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = when (f.tipo) { TipoCampo.NUMERO -> KeyboardType.Number; TipoCampo.HORA -> KeyboardType.Number; else -> KeyboardType.Text }),
                                placeholder = { Text(if (f.tipo == TipoCampo.HORA) "08:30" else "") },
                                supportingText = {
                                    val partes = buildList {
                                        if (f.ayuda.isNotBlank()) add(f.ayuda)
                                        if (f.tipo == TipoCampo.NUMERO) add(if (f.max != 0.0) "Entre ${aTexto(f.min)} y ${aTexto(f.max)}" else "Mínimo ${aTexto(f.min)}")
                                        if (f.tipo == TipoCampo.HORA) add("Formato 24 horas, por ejemplo 08:30")
                                    }
                                    if (partes.isNotEmpty()) Text(partes.joinToString(". "))
                                },
                            )
                        }
                    }
                }
            }
        }
        error?.let { Text(it, Modifier.testTag("config_error"), color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodyMedium) }
        if (guardado) Text("Guardado", Modifier.testTag("config_guardado"), color = MaterialTheme.colorScheme.primary, style = MaterialTheme.typography.labelLarge)
        Button(
            {
                haptica(Toque.Confirmar)
                val cfg = LinkedHashMap<String, Any?>()
                var fallo: String? = null
                m.esquema.forEach { f ->
                    val t = valores[f.clave].orEmpty().trim()
                    when (f.tipo) {
                        TipoCampo.SECRETO -> {} // la llave nunca va a la configuración
                        TipoCampo.NUMERO -> t.replace(',', '.').toDoubleOrNull()?.let { cfg[f.clave] = it } ?: run { fallo = "«${f.titulo}» debe ser un número" }
                        TipoCampo.INTERRUPTOR -> cfg[f.clave] = t == "true"
                        else -> cfg[f.clave] = t
                    }
                }
                error = fallo
                if (fallo == null) scope.launch {
                    runCatching { Repo.guardarConfigModulo(botId, m.id, cfg) }
                        .onSuccess { guardado = true }
                        .onFailure { guardado = false; error = it.message ?: "No se pudo guardar" }
                }
            },
            Modifier.fillMaxWidth().testTag("config_guardar"),
        ) { Text("Guardar") }
    }
}

// ---------- límites propios de un módulo de iniciativa ----------

@Composable
private fun LimitesDelModulo(botId: Long, m: ModuloInfo) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var hora by remember(m.id, m.topeHora) { mutableStateOf(m.topeHora.toString()) }
    var dia by remember(m.id, m.topeDia) { mutableStateOf(m.topeDia.toString()) }
    var msg by remember(m.id) { mutableStateOf<String?>(null) }
    Card(Modifier.fillMaxWidth().testTag("limites_modulo"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Text("Límites de este módulo", style = MaterialTheme.typography.titleMedium)
            Text("Cuántos mensajes puede mandar por su cuenta. 0 es el límite que trae el módulo (por hora ${m.defHora}, por día ${m.defDia}; 0 = sin límite propio). Además valen los topes generales de Iniciativa propia.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                OutlinedTextField(hora, { hora = it.filter(Char::isDigit).take(4); msg = null }, Modifier.weight(1f).testTag("tope_modulo_hora"), label = { Text("Por hora") }, singleLine = true, keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = KeyboardType.Number))
                OutlinedTextField(dia, { dia = it.filter(Char::isDigit).take(5); msg = null }, Modifier.weight(1f).testTag("tope_modulo_dia"), label = { Text("Por día") }, singleLine = true, keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = KeyboardType.Number))
            }
            msg?.let { Text(it, Modifier.testTag("limites_msg"), style = MaterialTheme.typography.labelLarge, color = if (it == "Guardado") MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error) }
            Button({
                scope.launch {
                    runCatching { Repo.fijarTopesModulo(botId, m.id, hora.toIntOrNull() ?: 0, dia.toIntOrNull() ?: 0) }
                        .onSuccess { msg = "Guardado" }.onFailure { msg = it.message ?: "No se pudo guardar" }
                }
            }, Modifier.fillMaxWidth().testTag("guardar_limites_modulo")) { Text("Guardar límites") }
        }
    }
}
