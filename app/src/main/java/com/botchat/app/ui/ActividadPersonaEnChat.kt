@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DatePickerDialog
import androidx.compose.material3.DateRangePicker
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.PrimaryScrollableTabRow
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Tab
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.rememberDateRangePickerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.FiltroActividad
import com.botchat.app.datos.MensajeActividad
import com.botchat.app.datos.Repo
import com.botchat.app.datos.VersionMensaje
import com.botchat.app.datos.mensajesDePersona
import com.botchat.app.datos.versionesDeMensaje
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId

private val PESTANAS_ACTIVIDAD = listOf("mensajes" to "Mensajes", "fotos" to "Fotos y videos", "audios" to "Audios", "archivos" to "Archivos y enlaces")

private fun inicioDeDia(d: LocalDate): Long = d.atStartOfDay(ZoneId.systemDefault()).toEpochSecond()

/**
 * Lo que una persona escribió en un chat (el privado o un grupo): cuatro pestañas, filtros por fecha y palabra, un botón Marcar en cada
 * mensaje, marca roja en los borrados y editados (con lo que decían antes) y paginación.
 */
@Composable
fun PantallaActividadPersona(
    botId: Long, persona: String, chat: String, titulo: String, alVolver: () -> Unit, alIrAlMensaje: (chat: String, id: String) -> Unit = { _, _ -> },
) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    val privado = !chat.endsWith("@g.us")
    var pestana by remember { mutableStateOf("mensajes") }
    var palabra by remember { mutableStateOf("") }
    var palabraAplicada by remember { mutableStateOf("") }
    var desde by remember { mutableStateOf(0L) }
    var hasta by remember { mutableStateOf(0L) }
    var rango by remember { mutableStateOf("todas") }
    var soloMarcados by remember { mutableStateOf(false) }
    var soloBorrados by remember { mutableStateOf(false) }
    var soloEditados by remember { mutableStateOf(false) }
    var version by remember { mutableIntStateOf(0) }
    var items by remember { mutableStateOf<List<MensajeActividad>?>(null) }
    var total by remember { mutableStateOf(-1L) }
    var fin by remember { mutableStateOf(false) }
    var sig by remember { mutableStateOf(0L to "") }
    var cargando by remember { mutableStateOf(false) }
    var generacion by remember { mutableIntStateOf(0) }
    var marcando by remember { mutableStateOf<MensajeActividad?>(null) }
    var borrando by remember { mutableStateOf<MensajeActividad?>(null) }
    var eligiendoFechas by remember { mutableStateOf(false) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if (evs.any { it == "etiquetas" || it == "chat:$chat" }) version++ } }
    LaunchedEffect(palabra) { delay(450); palabraAplicada = palabra.trim().takeIf { it.length >= 2 } ?: "" }

    fun filtro() = FiltroActividad(chat, persona, pestana, desde, hasta, palabraAplicada, soloMarcados, soloBorrados, soloEditados)
    fun cargar(primera: Boolean) {
        if (cargando && !primera) return
        if (primera) generacion++
        val mia = generacion
        cargando = true
        val f = filtro()
        val actuales = if (primera) emptyList() else items ?: emptyList()
        val (ts, id) = if (primera) 0L to "" else sig
        scope.launch {
            val p = runCatching { Repo.mensajesDePersona(botId, f, ts, id) }
            if (mia != generacion) return@launch // llegó tarde: ya se pidió otra cosa
            p.onSuccess { pag ->
                items = actuales + pag.items; total = pag.total; fin = pag.fin; sig = pag.sigTs to pag.sigId
            }.onFailure { if (primera) items = emptyList(); fin = true; avisaToast(ctx, it.message ?: "No se pudo cargar", true) }
            cargando = false
        }
    }
    LaunchedEffect(pestana, palabraAplicada, desde, hasta, soloMarcados, soloBorrados, soloEditados, version, persona, chat) { items = null; fin = false; cargar(true) }

    Box(Modifier.fillMaxSize()) {
    Scaffold(topBar = {
        Column {
            TopAppBar(navigationIcon = { IconButton(alVolver, Modifier.testTag("volver_actividad")) { Icon(Ico.Atras, "Volver") } }, title = {
                Column {
                    Text(titulo, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.testTag("titulo_actividad"))
                    if (total >= 0) Text(plural(total, "resultado", "resultados"), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.testTag("total_actividad"))
                }
            })
            PrimaryScrollableTabRow(PESTANAS_ACTIVIDAD.indexOfFirst { it.first == pestana }, edgePadding = 0.dp) {
                PESTANAS_ACTIVIDAD.forEach { (k, t) -> Tab(pestana == k, { haptica(Toque.Suave); pestana = k }, text = { Text(t, maxLines = 1) }, modifier = Modifier.testTag("pestana_$k")) }
            }
        }
    }) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno)) {
            // ----- filtros -----
            OutlinedTextField(
                palabra, { palabra = it }, Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.xs).testTag("actividad_palabra"), singleLine = true,
                placeholder = { Text("Buscar una palabra") }, leadingIcon = { Icon(Ico.Buscar, null) },
                trailingIcon = { if (palabra.isNotEmpty()) IconButton({ palabra = "" }) { Icon(Ico.Cerrar, "Borrar texto") } },
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
            )
            FlowRow(Modifier.padding(horizontal = Esp.l), horizontalArrangement = Arrangement.spacedBy(Esp.xs), verticalArrangement = Arrangement.spacedBy(0.dp)) {
                fun preset(clave: String, d: Long, h: Long) { rango = clave; desde = d; hasta = h }
                val hoy = LocalDate.now()
                FilterChip(rango == "todas", { preset("todas", 0, 0) }, { Text("Cualquier fecha") }, modifier = Modifier.testTag("fecha_todas"))
                FilterChip(rango == "hoy", { preset("hoy", inicioDeDia(hoy), 0) }, { Text("Hoy") }, modifier = Modifier.testTag("fecha_hoy"))
                FilterChip(rango == "7", { preset("7", inicioDeDia(hoy.minusDays(6)), 0) }, { Text("7 días") }, modifier = Modifier.testTag("fecha_7"))
                FilterChip(rango == "30", { preset("30", inicioDeDia(hoy.minusDays(29)), 0) }, { Text("30 días") }, modifier = Modifier.testTag("fecha_30"))
                FilterChip(rango == "otro", { eligiendoFechas = true }, { Text("Elegir fechas") }, modifier = Modifier.testTag("fecha_otro"))
                FilterChip(soloMarcados, { soloMarcados = !soloMarcados }, { Text("Marcados") }, modifier = Modifier.testTag("solo_marcados"))
                FilterChip(soloBorrados, { soloBorrados = !soloBorrados }, { Text("Borrados") }, modifier = Modifier.testTag("solo_borrados"))
                FilterChip(soloEditados, { soloEditados = !soloEditados }, { Text("Editados") }, modifier = Modifier.testTag("solo_editados"))
            }
            val datos = items
            when {
                datos == null -> Box(Modifier.fillMaxSize(), Alignment.Center) { CircularProgressIndicator() }
                datos.isEmpty() && !fin -> Box(Modifier.fillMaxSize(), Alignment.Center) { CircularProgressIndicator() }
                datos.isEmpty() -> EstadoVacio(Ico.Buscar, "Nada por aquí", "No hay mensajes con estos filtros.", Modifier.padding(top = 48.dp).testTag("actividad_sin_resultados"))
                else -> LazyColumn(Modifier.fillMaxSize().testTag("lista_actividad_mensajes"), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    items(datos, key = { it.msg.id + it.msg.chat }) { x ->
                        FilaActividad(
                            botId, x, privado, alMarcar = { haptica(Toque.Suave); marcando = x }, alIr = { alIrAlMensaje(x.msg.chat.ifBlank { chat }, x.msg.id) },
                            alBorrar = { haptica(Toque.Fuerte); borrando = x },
                        )
                    }
                    if (!fin) item(key = "mas") {
                        LaunchedEffect(datos.size, sig) { cargar(false) }
                        Box(Modifier.fillMaxWidth().padding(Esp.l), Alignment.Center) {
                            if (cargando) CircularProgressIndicator(Modifier.size(24.dp), strokeWidth = 2.dp)
                            else OutlinedButton({ cargar(false) }, Modifier.testTag("buscar_mas")) { Text("Buscar más") }
                        }
                    }
                }
            }
        }
    }

    marcando?.let { x -> DialogoMarcarMensaje(botId, x.msg.chat.ifBlank { chat }, x.msg.id, alCerrar = { marcando = null }, alCambio = { version++ }) }
    borrando?.let { x ->
        ConfirmarSinDeshacer(
            "¿Borrar este mensaje del grupo?", "Se borra para todos en el grupo. Solo puedo hacerlo si soy administrador.", "Borrar del grupo", "confirmar_borrar_ajeno", { borrando = null },
        ) {
            borrando = null
            scope.launch {
                runCatching { Repo.borrarMensajeAjeno(botId, x.msg.chat.ifBlank { chat }, x.msg.id) }
                    .onSuccess { avisaToast(ctx, "Listo: lo borré del grupo", false); version++ }.onFailure { avisaToast(ctx, textoError(it), true) }
            }
        }
    }
    if (eligiendoFechas) {
        val estado = rememberDateRangePickerState()
        DatePickerDialog(
            onDismissRequest = { eligiendoFechas = false },
            confirmButton = {
                TextButton({
                    val a = estado.selectedStartDateMillis
                    val b = estado.selectedEndDateMillis ?: a
                    if (a != null && b != null) {
                        desde = inicioDeDia(Instant.ofEpochMilli(a).atZone(ZoneId.of("UTC")).toLocalDate())
                        hasta = inicioDeDia(Instant.ofEpochMilli(b).atZone(ZoneId.of("UTC")).toLocalDate().plusDays(1))
                        rango = "otro"
                    }
                    eligiendoFechas = false
                }, Modifier.testTag("fechas_aceptar")) { Text("Aceptar") }
            },
            dismissButton = { TextButton({ eligiendoFechas = false }) { Text("Cancelar") } },
        ) { DateRangePicker(estado, Modifier.height(480.dp)) }
    }
    }
}

/** Un mensaje de la lista, con su marca roja si fue borrado o editado y los botones de siempre. */
@Composable
private fun FilaActividad(botId: Long, x: MensajeActividad, privado: Boolean, alMarcar: () -> Unit, alIr: () -> Unit, alBorrar: () -> Unit) {
    val m = x.msg
    val scope = rememberCoroutineScope()
    var versiones by remember(m.id) { mutableStateOf<List<VersionMensaje>?>(null) }
    var verAntes by remember(m.id) { mutableStateOf(false) }
    val roja = x.marcaRoja.isNotBlank()
    Card(Modifier.fillMaxWidth().testTag("actividad_msg_${m.id}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Row(Modifier.height(IntrinsicSize.Min)) {
            if (roja) Box(Modifier.width(5.dp).fillMaxHeight().background(ROJO_SILENCIO).testTag("marca_roja_${m.id}"))
            Column(Modifier.weight(1f).padding(Esp.m), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text("${etiquetaDia(m.ts)} · ${horaDe(m.ts)}", Modifier.weight(1f), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.primary)
                    if (x.marcado) Icon(Ico.Seleccionar, "Marcado", Modifier.size(18.dp).testTag("marcado_${m.id}"), tint = AMARILLO_SILENCIO)
                }
                if (roja) Text(
                    when (x.marcaRoja) {
                        "borrado" -> if (x.borradoPor.isBlank() || x.borradoPor == "autor") "Borrado por quien lo escribió" else "Borrado por ${x.borradoPor}"
                        else -> "Editado"
                    },
                    Modifier.testTag("texto_marca_roja_${m.id}"), style = MaterialTheme.typography.labelLarge, color = ROJO_SILENCIO, fontWeight = androidx.compose.ui.text.font.FontWeight.SemiBold,
                )
                Text(
                    if (m.borrado && !m.contenidoDisponible) "Contenido no disponible" else descripcionCorta(m.tipo, m.texto, false, m.notaVoz),
                    style = MaterialTheme.typography.bodyLarge, maxLines = 6, overflow = TextOverflow.Ellipsis,
                )
                if (x.versiones > 0) {
                    TextButton({
                        verAntes = !verAntes
                        if (verAntes && versiones == null) scope.launch { versiones = runCatching { Repo.versionesDeMensaje(botId, m.chat, m.id) }.getOrDefault(emptyList()) }
                    }, Modifier.testTag("ver_antes_${m.id}")) { Text(if (verAntes) "Ocultar lo que decía antes" else "Ver lo que decía antes (${x.versiones})") }
                    androidx.compose.animation.AnimatedVisibility(verAntes) { Column(Modifier.testTag("versiones_${m.id}"), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                        val vs = versiones
                        if (vs == null) CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                        else vs.forEach { v ->
                            Text("Antes (${etiquetaDia(v.ts)} ${horaDe(v.ts)}): ${v.texto}", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    } }
                }
                FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(0.dp)) {
                    OutlinedButton(alMarcar, Modifier.testTag("marcar_${m.id}")) { Icon(Ico.Seleccionar, null, Modifier.size(18.dp)); Text(if (x.marcado) "  Marcado" else "  Marcar") }
                    OutlinedButton(alIr, Modifier.testTag("ir_chat_${m.id}")) { Icon(Ico.IrA, null, Modifier.size(18.dp)); Text("  Ver en el chat") }
                    if (!privado && !m.esMio && !m.borrado) TextButton(alBorrar, Modifier.testTag("borrar_ajeno_${m.id}")) { Text("Borrar del grupo", color = MaterialTheme.colorScheme.error) }
                }
            }
        }
    }
}
