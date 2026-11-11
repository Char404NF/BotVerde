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
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DatePicker
import androidx.compose.material3.DatePickerDialog
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
import androidx.compose.material3.TimePicker
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.rememberDatePickerState
import androidx.compose.material3.rememberTimePickerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
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
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.Alarmas
import com.botchat.app.datos.AlertaPalabraItem
import com.botchat.app.datos.RecordatorioItem
import com.botchat.app.datos.Repo
import kotlinx.coroutines.launch
import java.time.Instant
import java.time.LocalDate
import java.time.LocalDateTime
import java.time.LocalTime
import java.time.ZoneId
import java.time.ZoneOffset

internal fun textoCuando(ts: Long): String = etiquetaDia(ts) + " " + horaDe(ts)
internal fun textoRepite(r: String) = when (r) { "diaria" -> "Todos los días"; "semanal" -> "Cada semana"; else -> "Una sola vez" }

/**
 * Ajustes → Avisos: las alertas que llegan SOLO como notificación de Android (nunca como mensaje de WhatsApp). Palabras clave, resumen del
 * día y recordatorios; arriba, el estado de las alarmas exactas.
 */
@Composable
fun PantallaAvisos(botId: Long, alVolver: () -> Unit, alRecordatorio: (Long) -> Unit = {}) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if (evs.any { it == "alertas" || it == "recordatorios" || it == "permisos_ajustes" }) version++ } }
    // Al volver de la pantalla del sistema donde se concede el permiso, se vuelve a revisar.
    val dueno = LocalLifecycleOwner.current
    var exactas by remember { mutableStateOf(Alarmas.puedeExactas(ctx)) }
    DisposableEffect(dueno) {
        val obs = androidx.lifecycle.LifecycleEventObserver { _, ev -> if (ev == androidx.lifecycle.Lifecycle.Event.ON_RESUME) { exactas = Alarmas.puedeExactas(ctx); scope.launch { Alarmas.reprogramarAsync(ctx) } } }
        dueno.lifecycle.addObserver(obs)
        onDispose { dueno.lifecycle.removeObserver(obs) }
    }
    val ajustes by produceState<Map<String, String>?>(null, version) { value = runCatching { Repo.ajustesPermisos(botId) }.getOrDefault(emptyMap()) }
    val palabras by produceState<List<AlertaPalabraItem>?>(null, version) { value = runCatching { Repo.palabrasClave(botId) }.getOrDefault(emptyList()) }
    val recordatorios by produceState<List<RecordatorioItem>?>(null, version) { value = runCatching { Repo.recordatorios(botId) }.getOrDefault(emptyList()) }
    var palabraNueva by remember { mutableStateOf("") }
    var eligiendoChats by remember { mutableStateOf<AlertaPalabraItem?>(null) }
    var editandoHora by remember { mutableStateOf(false) }
    var vistaResumen by remember { mutableStateOf<String?>(null) }
    val a = ajustes
    fun fijar(clave: String, valor: String) = scope.launch {
        runCatching { Repo.fijarAjustePermisos(botId, clave, valor) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo guardar", true) }
        version++
        Alarmas.reprogramarAsync(ctx)
    }
    fun hacer(f: suspend () -> Unit) = scope.launch {
        runCatching { f() }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
        version++
        Alarmas.reprogramarAsync(ctx)
    }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Avisos") }) }) { relleno ->
        if (a == null) { Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }; return@Scaffold }
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(bottom = 32.dp)) {
            Text(
                "Los avisos llegan como notificación de Android en este teléfono. Nunca se manda un mensaje de WhatsApp a nadie.",
                Modifier.padding(horizontal = Esp.l, vertical = Esp.s), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Seccion("Alarmas") {
                Column(Modifier.padding(horizontal = Esp.l, vertical = Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    if (exactas) {
                        Text("Las alarmas suenan a la hora exacta.", Modifier.testTag("alarmas_exactas"), style = MaterialTheme.typography.bodyLarge)
                    } else {
                        Text("Puede haber retrasos por el ahorro de batería: el teléfono no deja a la app despertar justo a la hora.", Modifier.testTag("alarmas_retraso"), style = MaterialTheme.typography.bodyLarge, color = MaterialTheme.colorScheme.error)
                        val intent = remember { Alarmas.intentPermisoExactas(ctx) }
                        if (intent != null) Button({ haptica(Toque.Suave); runCatching { ctx.startActivity(intent) }.onFailure { avisaToast(ctx, "Abre Ajustes del teléfono → Apps → Bot Chat → Alarmas y recordatorios", true) } }, Modifier.testTag("pedir_alarmas_exactas")) { Text("Permitir alarmas exactas") }
                    }
                }
                Interruptor("Usar alarmas exactas", "Si el teléfono lo permite. Apagado, los avisos pueden tardar unos minutos.", a["avisos_exactos"] != "0", true, Ico.Reloj) { fijar("avisos_exactos", if (it) "1" else "0") }
            }
            Seccion("Palabras clave") {
                Interruptor("Avisarme por palabra clave", "Una notificación cuando alguien escribe una de estas palabras.", a["alertas_activas"] != "0", true, Ico.Aviso) { fijar("alertas_activas", if (it) "1" else "0") }
                Interruptor("Incluir un fragmento del mensaje", "Apagado: el aviso solo dice la palabra, el chat y quién escribió.", a["alerta_fragmento"] == "1", true, Ico.Documento) { fijar("alerta_fragmento", if (it) "1" else "0") }
                val l = palabras
                when {
                    l == null -> CircularProgressIndicator(Modifier.padding(Esp.l))
                    l.isEmpty() -> Text("Todavía no hay palabras. Agrega, por ejemplo, “urgente” o “cotización”.", Modifier.padding(horizontal = Esp.l).testTag("sin_palabras"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    else -> l.forEach { p ->
                        Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l).testTag("palabra_${p.id}"), verticalAlignment = Alignment.CenterVertically) {
                            Column(Modifier.weight(1f).clickable { eligiendoChats = p }) {
                                Text("“${p.palabra}”", style = MaterialTheme.typography.bodyLarge, fontWeight = FontWeight.Medium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                Text(if (p.chats.isEmpty()) "En todos los chats" else "En " + plural(p.chats.size.toLong(), "chat", "chats"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                            Switch(p.activa, { hacer { Repo.editarPalabraClave(botId, p.id, p.chats, it) } }, Modifier.testTag("palabra_activa_${p.id}"))
                            IconButton({ haptica(Toque.Suave); hacer { Repo.borrarPalabraClave(botId, p.id) } }, Modifier.testTag("borrar_palabra_${p.id}")) { Icon(Ico.Borrar, "Quitar la palabra") }
                        }
                    }
                }
                Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l), verticalAlignment = Alignment.CenterVertically) {
                    OutlinedTextField(palabraNueva, { palabraNueva = it.take(60) }, Modifier.weight(1f).testTag("palabra_nueva"), label = { Text("Palabra o frase") }, singleLine = true)
                    TextButton({ val p = palabraNueva.trim(); palabraNueva = ""; hacer { Repo.crearPalabraClave(botId, p) } }, Modifier.testTag("agregar_palabra"), enabled = palabraNueva.isNotBlank()) { Text("Agregar") }
                }
            }
            Seccion("Resumen del día") {
                Interruptor("Resumen diario", "Mensajes recibidos, chats pendientes, solicitudes y advertencias del día.", a["resumen_activo"] == "1", true, Ico.Estadisticas) { fijar("resumen_activo", if (it) "1" else "0") }
                if (a["resumen_activo"] == "1") Fila("Hora del resumen", "Cada día a esta hora", Ico.Reloj) {
                    OutlinedButton({ haptica(Toque.Suave); editandoHora = true }, Modifier.testTag("hora_resumen")) { Text(a["resumen_hora"] ?: "20:00") }
                }
                OutlinedButton({ haptica(Toque.Suave); scope.launch { vistaResumen = runCatching { Repo.vistaPreviaResumen(botId) }.getOrElse { it.message ?: "No se pudo" } } }, Modifier.padding(horizontal = Esp.l).testTag("ver_resumen")) { Text("Ver cómo queda hoy") }
            }
            Seccion("Recordatorios") {
                val r = recordatorios
                when {
                    r == null -> CircularProgressIndicator(Modifier.padding(Esp.l))
                    r.isEmpty() -> Text("No tienes recordatorios. También puedes crearlos escribiéndote en “Mensajes guardados”: !recordar mañana 9:00 llamar a Ana", Modifier.padding(horizontal = Esp.l).testTag("sin_recordatorios"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    else -> r.forEach { x ->
                        Row(Modifier.fillMaxWidth().clickable { alRecordatorio(x.id) }.padding(horizontal = Esp.l, vertical = Esp.xs).testTag("recordatorio_${x.id}"), verticalAlignment = Alignment.CenterVertically) {
                            Column(Modifier.weight(1f)) {
                                Text(x.texto, style = MaterialTheme.typography.bodyLarge, maxLines = 2, overflow = TextOverflow.Ellipsis)
                                Text(textoCuando(x.proximaTs) + " · " + textoRepite(x.repetir), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                            IconButton({ haptica(Toque.Suave); hacer { Repo.borrarRecordatorio(botId, x.id) } }, Modifier.testTag("borrar_recordatorio_${x.id}")) { Icon(Ico.Borrar, "Borrar el recordatorio") }
                        }
                    }
                }
                Button({ haptica(Toque.Suave); alRecordatorio(0) }, Modifier.padding(horizontal = Esp.l).testTag("nuevo_recordatorio")) { Text("Nuevo recordatorio") }
            }
        }
    }
    eligiendoChats?.let { p -> DialogoChatsDePalabra(botId, p, { eligiendoChats = null }) { chats -> eligiendoChats = null; hacer { Repo.editarPalabraClave(botId, p.id, chats, p.activa) } } }
    if (editandoHora) DialogoHoraResumen(a?.get("resumen_hora") ?: "20:00", { editandoHora = false }) { h -> editandoHora = false; fijar("resumen_hora", h) }
    vistaResumen?.let { t -> AlertDialog(onDismissRequest = { vistaResumen = null }, title = { Text("Resumen del día") }, text = { Text(t, Modifier.testTag("texto_resumen")) }, confirmButton = { TextButton({ vistaResumen = null }) { Text("Cerrar") } }) }
}

/** Elige en qué chats vale una palabra: en todos (por defecto) o solo en los marcados. */
@Composable
private fun DialogoChatsDePalabra(botId: Long, p: AlertaPalabraItem, alCerrar: () -> Unit, alGuardar: (List<String>) -> Unit) {
    val elegidos = remember { mutableStateListOf<String>().apply { addAll(p.chats) } }
    val chats by produceState<List<com.botchat.app.datos.ChatItem>?>(null) {
        value = runCatching { Repo.chats(botId, false, "", 400) + Repo.chats(botId, true, "", 400) }.getOrDefault(emptyList())
    }
    AlertDialog(
        onDismissRequest = alCerrar, title = { Text("Dónde avisar por “${p.palabra}”") },
        text = {
            Column(Modifier.height(320.dp).verticalScroll(rememberScrollState()).testTag("chats_de_palabra")) {
                Row(Modifier.fillMaxWidth().clickable { elegidos.clear() }, verticalAlignment = Alignment.CenterVertically) {
                    Checkbox(elegidos.isEmpty(), { if (it) elegidos.clear() }, Modifier.testTag("palabra_todos")); Text("En todos los chats")
                }
                val l = chats
                if (l == null) CircularProgressIndicator()
                else l.forEach { c ->
                    Row(Modifier.fillMaxWidth().clickable { if (c.jid in elegidos) elegidos.remove(c.jid) else elegidos.add(c.jid) }, verticalAlignment = Alignment.CenterVertically) {
                        Checkbox(c.jid in elegidos, { if (it) elegidos.add(c.jid) else elegidos.remove(c.jid) }, Modifier.testTag("palabra_chat_${c.jid}"))
                        Text(c.nombre, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                }
            }
        },
        confirmButton = { TextButton({ alGuardar(elegidos.toList()) }, Modifier.testTag("guardar_chats_palabra")) { Text("Guardar") } },
        dismissButton = { TextButton(alCerrar) { Text("Cancelar") } },
    )
}

@Composable
private fun DialogoHoraResumen(actual: String, alCerrar: () -> Unit, alElegir: (String) -> Unit) {
    val h = actual.substringBefore(':').toIntOrNull() ?: 20
    val m = actual.substringAfter(':', "0").toIntOrNull() ?: 0
    val estado = rememberTimePickerState(h, m, is24Hour = true)
    AlertDialog(
        onDismissRequest = alCerrar, title = { Text("Hora del resumen") },
        text = { Box(Modifier.fillMaxWidth(), Alignment.Center) { TimePicker(estado, Modifier.testTag("selector_hora_resumen")) } },
        confirmButton = { TextButton({ alElegir("%02d:%02d".format(estado.hour, estado.minute)) }, Modifier.testTag("guardar_hora_resumen")) { Text("Guardar") } },
        dismissButton = { TextButton(alCerrar) { Text("Cancelar") } },
    )
}

/** Crear o cambiar un recordatorio: texto, cuándo (atajos o fecha y hora a mano) y si se repite. */
@Composable
fun EditorRecordatorio(botId: Long, id: Long, alVolver: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val zona = remember { ZoneId.systemDefault() }
    val actual by produceState<RecordatorioItem?>(null, id) { value = if (id == 0L) RecordatorioItem(0, "", 0, "", "pendiente", 0, 0) else runCatching { Repo.recordatorios(botId).firstOrNull { it.id == id } }.getOrNull() }
    val a = actual
    if (a == null) { Box(Modifier.fillMaxSize(), Alignment.Center) { CircularProgressIndicator() }; return }
    var texto by remember(a) { mutableStateOf(a.texto) }
    var cuando by remember(a) { mutableStateOf<LocalDateTime?>(if (a.proximaTs > 0) LocalDateTime.ofInstant(Instant.ofEpochSecond(a.proximaTs), zona) else null) }
    var repetir by remember(a) { mutableStateOf(a.repetir) }
    var eligiendoFecha by remember { mutableStateOf(false) }
    var eligiendoHora by remember { mutableStateOf(false) }
    var fechaElegida by remember { mutableStateOf<LocalDate?>(null) }
    var guardando by remember { mutableStateOf(false) }
    val ahora = LocalDateTime.now(zona)
    val ts = cuando?.atZone(zona)?.toEpochSecond()
    val enElFuturo = ts != null && ts > System.currentTimeMillis() / 1000
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text(if (id == 0L) "Nuevo recordatorio" else "Cambiar recordatorio") }) }) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            OutlinedTextField(texto, { texto = it.take(500) }, Modifier.fillMaxWidth().testTag("texto_recordatorio"), label = { Text("¿De qué te aviso?") }, minLines = 2)
            Text("¿Cuándo?", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                AtajoCuando("En 30 min", "atajo_30m") { cuando = ahora.plusMinutes(30).withSecond(0).withNano(0) }
                AtajoCuando("En 1 hora", "atajo_1h") { cuando = ahora.plusHours(1).withSecond(0).withNano(0) }
                AtajoCuando("Mañana 9:00", "atajo_manana") { cuando = LocalDateTime.of(ahora.toLocalDate().plusDays(1), LocalTime.of(9, 0)) }
                AtajoCuando("Elegir fecha y hora…", "atajo_elegir") { eligiendoFecha = true }
            }
            Text(
                cuando?.let { textoCuando(it.atZone(zona).toEpochSecond()) } ?: "Elige cuándo",
                Modifier.testTag("cuando_recordatorio"), style = MaterialTheme.typography.bodyLarge, fontWeight = FontWeight.Medium,
                color = if (cuando != null && !enElFuturo) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurface,
            )
            if (cuando != null && !enElFuturo) Text("Esa hora ya pasó.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
            Text("¿Se repite?", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                listOf("" to "No", "diaria" to "Todos los días", "semanal" to "Cada semana").forEach { (v, t) -> FilterChip(repetir == v, { repetir = v }, { Text(t, maxLines = 1) }, modifier = Modifier.testTag("repite_${v.ifEmpty { "no" }}")) }
            }
            Button({
                guardando = true
                scope.launch {
                    runCatching { if (id == 0L) Repo.crearRecordatorio(botId, texto.trim(), ts ?: 0L, repetir) else Repo.editarRecordatorio(botId, id, texto.trim(), ts ?: 0L, repetir) }
                        .onSuccess { Alarmas.reprogramarAsync(ctx); alVolver() }
                        .onFailure { avisaToast(ctx, it.message ?: "No se pudo guardar", true); guardando = false }
                }
            }, Modifier.fillMaxWidth().testTag("guardar_recordatorio"), enabled = texto.isNotBlank() && enElFuturo && !guardando) { Text("Guardar") }
        }
    }
    if (eligiendoFecha) {
        val est = rememberDatePickerState(initialSelectedDateMillis = (cuando?.toLocalDate() ?: ahora.toLocalDate()).atStartOfDay().toInstant(ZoneOffset.UTC).toEpochMilli())
        DatePickerDialog(
            onDismissRequest = { eligiendoFecha = false },
            confirmButton = { TextButton({ est.selectedDateMillis?.let { fechaElegida = Instant.ofEpochMilli(it).atZone(ZoneOffset.UTC).toLocalDate(); eligiendoFecha = false; eligiendoHora = true } }, Modifier.testTag("fecha_siguiente")) { Text("Siguiente") } },
            dismissButton = { TextButton({ eligiendoFecha = false }) { Text("Cancelar") } },
        ) { DatePicker(est) }
    }
    if (eligiendoHora) {
        val hEst = rememberTimePickerState((cuando ?: ahora.plusHours(1)).hour, (cuando ?: ahora).minute, is24Hour = true)
        AlertDialog(
            onDismissRequest = { eligiendoHora = false }, title = { Text("¿A qué hora?") },
            text = { Box(Modifier.fillMaxWidth(), Alignment.Center) { TimePicker(hEst) } },
            confirmButton = { TextButton({ fechaElegida?.let { cuando = LocalDateTime.of(it, LocalTime.of(hEst.hour, hEst.minute)) }; eligiendoHora = false }, Modifier.testTag("hora_listo")) { Text("Listo") } },
            dismissButton = { TextButton({ eligiendoHora = false }) { Text("Cancelar") } },
        )
    }
}

@Composable
private fun AtajoCuando(texto: String, tag: String, alPulsar: () -> Unit) {
    androidx.compose.material3.AssistChip(onClick = alPulsar, label = { Text(texto, maxLines = 1) }, modifier = Modifier.testTag(tag))
}
