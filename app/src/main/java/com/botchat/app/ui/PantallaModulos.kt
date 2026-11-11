@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.ModuloInfo
import com.botchat.app.datos.Repo
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

// ---------- colores y textos de riesgo ----------

private val VerdeRiesgo = Color(0xFF2E9E5B)
private val AmarilloRiesgo = Color(0xFFC98A00)

/** Color de la etiqueta de riesgo: verde (bajo), amarillo (medio) o rojo (alto). */
@Composable
internal fun colorRiesgo(riesgo: String): Color = when (riesgo) {
    "alto" -> MaterialTheme.colorScheme.error
    "medio" -> AmarilloRiesgo
    else -> VerdeRiesgo
}

internal fun textoRiesgo(riesgo: String) = when (riesgo) { "alto" -> "Riesgo alto"; "medio" -> "Riesgo medio"; else -> "Riesgo bajo" }

@Composable
internal fun EtiquetaRiesgo(riesgo: String, modifier: Modifier = Modifier) {
    val c = colorRiesgo(riesgo)
    Text(
        textoRiesgo(riesgo), modifier.clip(MaterialTheme.shapes.small).background(c.copy(alpha = 0.16f)).padding(horizontal = 8.dp, vertical = 3.dp).testTag("riesgo_$riesgo"),
        style = MaterialTheme.typography.labelMedium, color = c, maxLines = 1,
    )
}

/** Icono de la tarjeta según el nombre que da el módulo; si no lo conozco, uno genérico. */
fun iconoDeModulo(nombre: String): ImageVector = when (nombre) {
    "saludo", "bienvenida", "persona" -> Ico.Persona
    "difusion", "anuncio", "enviar" -> Ico.Enviar
    "reloj", "programado", "alarma" -> Ico.Reloj
    "invitar", "grupo", "agregar" -> Ico.Agregar
    "descarga", "conector" -> Ico.Descargar
    "traductor", "editar" -> Ico.Editar
    "ia", "info" -> Ico.Info
    "llamada", "microfono" -> Ico.Microfono
    "juego", "emoji" -> Ico.Emoji
    "eco", "responder" -> Ico.Responder
    else -> Ico.Ajustes
}

// ---------- Ajustes → Módulos ----------

/**
 * Módulos opcionales: tarjetas con icono, nombre, descripción simple, etiqueta de riesgo, interruptor y "Configurar". Encender uno de
 * riesgo medio o alto pasa antes por la pantalla de aviso; apagarlo es inmediato. Por defecto el bot solo responde: estos módulos son
 * lo único que puede escribir por su cuenta y todos dependen de "Iniciativa propia".
 */
@Composable
fun PantallaModulos(botId: Long, alVolver: () -> Unit, alConfigurar: (String) -> Unit, alAviso: (String) -> Unit, alIniciativa: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("modulos" in evs || "iniciativas" in evs) version++ } }
    val lista by produceState<List<ModuloInfo>?>(null, version) { value = runCatching { Repo.modulos(botId) }.getOrDefault(emptyList()) }
    val ajustes by produceState<Map<String, String>>(emptyMap(), version) { value = runCatching { Repo.ajustesIniciativa(botId) }.getOrDefault(emptyMap()) }
    val seguro by produceState(com.botchat.app.datos.ModoSeguro(false, 0, "", "", 0), version) { value = runCatching { Repo.modoSeguro(botId) }.getOrDefault(value) }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Módulos") }) }) { relleno ->
        val datos = lista
        if (datos == null) { Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }; return@Scaffold }
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Text(
                "Por defecto el bot solo responde. Estos módulos son extras: cada uno está apagado hasta que tú lo enciendas, y los que escriben solos necesitan además que “Iniciativa propia” esté encendida.",
                style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            val maestro = ajustes["maestro"] == "1"
            Card(
                onClick = alIniciativa, modifier = Modifier.fillMaxWidth().testTag("modulos_iniciativa"), shape = MaterialTheme.shapes.large,
                colors = CardDefaults.cardColors(containerColor = if (seguro.activo) MaterialTheme.colorScheme.errorContainer else MaterialTheme.colorScheme.surfaceContainer),
            ) {
                Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                    Text("Iniciativa propia: " + if (maestro) "encendida" else "apagada", style = MaterialTheme.typography.titleMedium)
                    Text(
                        when {
                            seguro.activo -> "Modo seguro activo: los módulos que escriben solos están en pausa. Toca para revisar y reanudar."
                            maestro -> "Los módulos que escriben solos pueden trabajar dentro de sus límites. Toca para ver topes y registro."
                            else -> "Ningún módulo puede escribir por su cuenta. Toca para encenderla o cambiar topes."
                        },
                        style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
            if (datos.isEmpty()) EstadoVacio(Ico.Ajustes, "Todavía no hay módulos", "Cuando se agreguen módulos opcionales aparecerán aquí, apagados.", Modifier.padding(top = Esp.xl))
            datos.forEach { m ->
                TarjetaModulo(
                    m, alInterruptor = { encender ->
                        haptica(Toque.Suave)
                        if (!encender) scope.launch { runCatching { Repo.desactivarModulo(botId, m.id) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo apagar", true) }; version++ }
                        else if (m.pideAviso) alAviso(m.id)
                        else scope.launch { runCatching { Repo.activarModulo(botId, m.id) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo encender", true) }; version++ }
                    },
                    alConfigurar = { alConfigurar(m.id) },
                )
            }
        }
    }
}

@Composable
private fun TarjetaModulo(m: ModuloInfo, alInterruptor: (Boolean) -> Unit, alConfigurar: () -> Unit) {
    Card(Modifier.fillMaxWidth().testTag("modulo_${m.id}"), shape = MaterialTheme.shapes.large, colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Box(Modifier.size(48.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primaryContainer), Alignment.Center) {
                    Icon(iconoDeModulo(m.icono), null, Modifier.size(24.dp), tint = MaterialTheme.colorScheme.onPrimaryContainer)
                }
                Spacer(Modifier.width(Esp.l))
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                    Text(m.nombre, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Medium)
                    EtiquetaRiesgo(m.riesgo)
                }
                Switch(m.activo, alInterruptor, Modifier.testTag("modulo_interruptor_${m.id}"))
            }
            Text(m.descripcion, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            if (m.activo || m.pausado) {
                Text(
                    m.estado, Modifier.testTag("modulo_estado_${m.id}"), style = MaterialTheme.typography.labelLarge,
                    color = if (m.pausado) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.primary,
                )
            }
            OutlinedButton(alConfigurar, Modifier.fillMaxWidth().testTag("modulo_configurar_${m.id}")) { Text("Configurar") }
        }
    }
}

// ---------- aviso de riesgo ----------

/** Segundos de espera obligatoria antes de poder aceptar un riesgo alto. */
const val ESPERA_RIESGO_ALTO_SEG = 5

/**
 * Aviso antes de encender un módulo de riesgo medio o alto. Medio: botón “Entiendo el riesgo”. Alto: además espera de 5 segundos
 * visible y una casilla obligatoria. Cancelar no enciende nada.
 */
@Composable
fun PantallaAvisoRiesgo(botId: Long, id: String, alVolver: () -> Unit, alEncendido: () -> Unit = alVolver) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val modulo by produceState<ModuloInfo?>(null, id) { value = runCatching { Repo.modulo(botId, id) }.getOrNull() }
    val m = modulo
    var segundos by remember { mutableIntStateOf(ESPERA_RIESGO_ALTO_SEG) }
    var casilla by remember { mutableStateOf(false) }
    var trabajando by remember { mutableStateOf(false) }
    val alto = m?.riesgoAlto == true
    LaunchedEffect(alto) {
        if (alto) while (segundos > 0) { delay(1000); segundos-- }
    }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Antes de encender") }) }) { relleno ->
        if (m == null) { Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }; return@Scaffold }
        val puede = !trabajando && (!alto || (segundos == 0 && casilla))
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Box(Modifier.size(56.dp).clip(CircleShape).background(colorRiesgo(m.riesgo).copy(alpha = 0.18f)), Alignment.Center) {
                    Icon(Ico.Aviso, null, Modifier.size(30.dp), tint = colorRiesgo(m.riesgo))
                }
                Spacer(Modifier.width(Esp.l))
                Column {
                    Text(m.nombre, style = MaterialTheme.typography.titleLarge, modifier = Modifier.testTag("aviso_nombre"))
                    EtiquetaRiesgo(m.riesgo)
                }
            }
            Text(m.descripcion, style = MaterialTheme.typography.bodyLarge)
            Card(colors = CardDefaults.cardColors(containerColor = colorRiesgo(m.riesgo).copy(alpha = 0.12f)), modifier = Modifier.fillMaxWidth().testTag("aviso_texto")) {
                Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text(if (alto) "Esto puede hacer que WhatsApp bloquee tu número." else "Úsalo con cuidado.", style = MaterialTheme.typography.titleMedium, color = colorRiesgo(m.riesgo))
                    Text("Este módulo escribe mensajes por su cuenta, sin que nadie le haya escrito antes.", style = MaterialTheme.typography.bodyMedium)
                    Text("WhatsApp no permite el envío automático o masivo, y puede limitar o bloquear el número que lo hace. Conviene usar un número secundario.", style = MaterialTheme.typography.bodyMedium)
                    if (alto) Text("Es el nivel de riesgo más alto: úsalo solo con personas que ya te escribieron o pidieron recibir mensajes, y con pocos envíos.", style = MaterialTheme.typography.bodyMedium)
                    if (m.id == "ia") Text("Privacidad: lo que se le pregunte a la IA SALE de este teléfono hacia el proveedor que elijas. No lo uses con información que no quieras compartir con él.", Modifier.testTag("aviso_privacidad"), style = MaterialTheme.typography.bodyMedium)
                    Text("Lo puedes apagar cuando quieras, y el interruptor general “Iniciativa propia” lo detiene de golpe.", style = MaterialTheme.typography.bodyMedium)
                }
            }
            if (alto) {
                Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.testTag("aviso_fila_casilla")) {
                    Checkbox(casilla, { casilla = it }, Modifier.testTag("aviso_casilla"))
                    Text("Entiendo que mi número puede ser bloqueado por WhatsApp", style = MaterialTheme.typography.bodyMedium, modifier = Modifier.weight(1f))
                }
                if (segundos > 0) Text("Espera $segundos ${if (segundos == 1) "segundo" else "segundos"} para poder continuar…", Modifier.testTag("aviso_espera"), style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            Button(
                {
                    trabajando = true
                    scope.launch {
                        runCatching { Repo.activarModulo(botId, id) }
                            .onSuccess { alEncendido() }
                            .onFailure { trabajando = false; avisaToast(ctx, it.message ?: "No se pudo encender", true) }
                    }
                },
                Modifier.fillMaxWidth().testTag("aviso_entiendo"), enabled = puede,
            ) { Text("Entiendo el riesgo") }
            OutlinedButton(alVolver, Modifier.fillMaxWidth().testTag("aviso_cancelar")) { Text("Cancelar") }
        }
    }
}
