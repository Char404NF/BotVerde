@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import com.botchat.app.ui.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.Ajustes
import com.botchat.app.datos.EstadoAlmacen
import com.botchat.app.datos.FiltroLimpieza
import com.botchat.app.datos.Liberado
import com.botchat.app.datos.Repo
import com.botchat.app.datos.TipoAlmacen
import com.botchat.app.datos.borrarLimpieza
import com.botchat.app.datos.estadoAlmacen
import com.botchat.app.datos.estimarLimpieza
import com.botchat.app.datos.fijarConservarPortada
import com.botchat.app.datos.fijarTopeTotal
import kotlin.math.roundToInt
import kotlinx.coroutines.launch

private val NOMBRE_TIPO = mapOf("foto" to "Fotos", "video" to "Videos", "audio" to "Audios", "documento" to "Documentos", "sticker" to "Stickers")
private fun iconoTipo(t: String) = when (t) { "foto" -> Ico.Foto; "video" -> Ico.Video; "audio" -> Ico.Audio; "sticker" -> Ico.Sticker; else -> Ico.Documento }

private val TOPES_TOTAL = listOf(1L, 2L, 5L, 10L, 20L).map { OpcionValor(it * GIB, "$it GB") } + OpcionValor(0L, "Sin límite")
private val TOPES_ARCHIVO = listOf(OpcionValor(0, "Sin límite"), OpcionValor(25 * MIB, "25 MB"), OpcionValor(100 * MIB, "100 MB"), OpcionValor(500 * MIB, "500 MB"), OpcionValor(GIB, "1 GB"))
private val TOPES_TIPO = listOf(OpcionValor(0, "Sin límite"), OpcionValor(GIB, "1 GB"), OpcionValor(2 * GIB, "2 GB"), OpcionValor(5 * GIB, "5 GB"))

/** Qué tope se está editando en la hoja: `null` = el total; si no, el tipo y si es por archivo. */
internal data class EditandoTope(val tipo: String?, val porArchivo: Boolean)

/** Estado que comparten el bloque y sus hojas (las hojas van fuera de la lista, en la caja de la pantalla). */
class EstadoAlmacenE8 {
    var version by mutableIntStateOf(0)
    var estado by mutableStateOf<EstadoAlmacen?>(null)
    internal var editando by mutableStateOf<EditandoTope?>(null)
    internal var otro by mutableStateOf<EditandoTope?>(null)
    var eligiendoPersona by mutableStateOf(false)
    var persona by mutableStateOf<com.botchat.app.datos.ChatItem?>(null)
}

/**
 * Todo lo nuevo de Almacenamiento (0.6.0): medidor contra el tope, uso por tipo con interruptores y topes, reglas de Wi-Fi y batería,
 * duplicados ahorrados, portadas y limpieza con filtros. Va dentro de la pantalla de Espacio.
 */
@Composable
fun BloqueAlmacenamiento(botId: Long, ajustes: Ajustes, h: EstadoAlmacenE8) {
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("espacio" in evs) h.version++ } }
    LaunchedEffect(h.version) { h.estado = runCatching { Repo.estadoAlmacen(botId) }.getOrNull() }
    val d by ajustes.descargas.collectAsState()

    fun ponerTopeTotal(bytes: Long) { scope.launch { runCatching { Repo.fijarTopeTotal(botId, bytes) }; h.version++ } }

    Column(Modifier.padding(horizontal = Esp.l, vertical = Esp.s).testTag("bloque_almacenamiento"), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
        val e = h.estado
        // ---- medidor contra el tope ----
        Card(Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
            Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("Lo que guardo de tus chats", style = MaterialTheme.typography.titleMedium)
                if (e == null) Text("Mirando…", color = MaterialTheme.colorScheme.onSurfaceVariant) else {
                    val color = colorDeSemaforo(e.color)
                    Text(
                        if (e.topeTotal > 0) "${formatoBytes(e.uso)} de ${formatoBytes(e.topeTotal)}" else "${formatoBytes(e.uso)} · sin límite",
                        Modifier.testTag("medidor_texto"), style = MaterialTheme.typography.headlineSmall, color = color, fontWeight = FontWeight.Bold,
                    )
                    if (e.topeTotal > 0) LinearProgressIndicator(
                        progress = { e.fraccion }, modifier = Modifier.fillMaxWidth().height(14.dp).clip(RoundedCornerShape(7.dp)).testTag("medidor_barra_${e.color}"),
                        color = color, trackColor = color.copy(alpha = 0.2f), strokeCap = androidx.compose.ui.graphics.StrokeCap.Round, gapSize = 0.dp, drawStopIndicator = {},
                    )
                    Text(
                        if (e.topeTotal > 0) "Al llegar al tope borro lo más viejo hasta quedar en el 90 %. Nunca borro lo marcado, lo de «ver una vez» ni los videos que aún no tienen portada."
                        else "Sin límite: no se borra nada solo.",
                        style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    Text("Tope total", style = MaterialTheme.typography.labelLarge)
                    SelectorValores(e.topeTotal, TOPES_TOTAL, "tope_total", { haptica(Toque.Suave); ponerTopeTotal(it) }, alOtro = { h.otro = EditandoTope(null, false) })
                }
            }
        }
        if (e != null && e.pasadoDelTope) CajaAviso(ROJO_SILENCIO, "Pasaste el tope", "Se va a borrar lo más viejo para volver a entrar. Sube el tope si quieres guardar más.", "aviso_pasado_tope")
        if (e != null && e.sinPortada > 0) CajaAviso(
            AMARILLO_SILENCIO, "${plural(e.sinPortada.toLong(), "video espera", "videos esperan")} su portada",
            "Hasta que tengan portada no se pueden borrar solos y pueden impedir llegar al 90 %.", "aviso_sin_portada",
        )
        if (e != null && e.ahorradoArchivos > 0) Row(Modifier.fillMaxWidth().testTag("ahorro_duplicados"), verticalAlignment = Alignment.CenterVertically) {
            Icon(Ico.Listo, null, Modifier.size(22.dp), tint = VERDE_SILENCIO)
            Spacer(Modifier.width(Esp.s))
            Text("Ahorré ${formatoBytes(e.ahorradoBytes)} al no repetir ${plural(e.ahorradoArchivos.toLong(), "archivo", "archivos")}.", style = MaterialTheme.typography.bodyMedium)
        }

        // ---- por tipo ----
        TituloSeccionE8("Qué guardo y cuánto")
        Text("Apaga un tipo para que no se descargue solo. Puedes poner un tope a cada archivo y a todo el tipo.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Ajustes.TIPOS_DESCARGA.forEach { t ->
            val uso = e?.tipos?.firstOrNull { it.tipo == t } ?: TipoAlmacen(t, 0, 0, false, 0, 0)
            val regla = when (t) { "foto" -> d.foto; "video" -> d.video; "audio" -> d.audio; "documento" -> d.documento; else -> d.sticker }
            val prendido = t !in d.apagados
            val topeArchivo = (d.topeArchivoMb[t] ?: 0) * MIB
            val topeTipo = (d.topeTipoMb[t] ?: 0) * MIB
            Card(Modifier.fillMaxWidth().testTag("tipo_$t"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                Column(Modifier.padding(Esp.m), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Icon(iconoTipo(t), null, Modifier.size(28.dp), tint = MaterialTheme.colorScheme.primary)
                        Spacer(Modifier.width(Esp.m))
                        Column(Modifier.weight(1f)) {
                            Text(NOMBRE_TIPO.getValue(t), style = MaterialTheme.typography.titleSmall)
                            Text("${formatoBytes(uso.bytes)} · ${plural(uso.cantidad.toLong(), "archivo", "archivos")}", Modifier.testTag("uso_$t"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                        Switch(prendido, { haptica(Toque.Suave); ajustes.cambiarDescargas { x -> x.copy(apagados = if (it) x.apagados - t else x.apagados + t) } }, Modifier.testTag("sw_tipo_$t"))
                    }
                    if (!prendido) Text("Apagado: no se descarga solo; queda el botón Descargar.", style = MaterialTheme.typography.bodySmall, color = AMARILLO_SILENCIO)
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text("Solo con Wi-Fi", Modifier.weight(1f), style = MaterialTheme.typography.bodyMedium)
                        Switch(!regla.datos, { haptica(Toque.Suave); cambiarRegla(ajustes, t, regla.copy(datos = !it)) }, Modifier.testTag("sw_wifi_$t"))
                    }
                    Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                        OutlinedButton({ h.editando = EditandoTope(t, true) }, Modifier.weight(1f).testTag("tope_archivo_$t")) { Text("Cada uno: ${textoTope(topeArchivo)}", maxLines = 1) }
                        OutlinedButton({ h.editando = EditandoTope(t, false) }, Modifier.weight(1f).testTag("tope_tipo_$t")) { Text("Todos: ${textoTope(topeTipo)}", maxLines = 1) }
                    }
                }
            }
        }

        // ---- red, batería, espacio libre, portada ----
        TituloSeccionE8("Cuidar datos, batería y espacio")
        Card(Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
            Column(Modifier.padding(vertical = Esp.s)) {
                Interruptor("Archivos de más de 50 MB solo con Wi-Fi", "Así no gastas tus datos móviles con archivos pesados.", d.soloWifiGrandes, true, Ico.Descargar) { ajustes.cambiarDescargas { x -> x.copy(soloWifiGrandes = it) } }
                Interruptor("Esperar si la batería está baja", "Con menos de ${d.bateriaMin} % y sin cargar, las descargas esperan.", d.pausaBateria, true, Ico.Reloj) { ajustes.cambiarDescargas { x -> x.copy(pausaBateria = it) } }
                Deslizador("Espacio libre mínimo", if (d.pisoLibreMb >= 1000) "${"%.1f".format(d.pisoLibreMb / 1000f)} GB" else "${d.pisoLibreMb} MB", d.pisoLibreMb.toFloat(), 100f..3000f, 28, "Con menos espacio libre en el teléfono se pausan las descargas.") {
                    ajustes.cambiarDescargas { x -> x.copy(pisoLibreMb = (it / 100).roundToInt() * 100) }
                }
                if (e != null) Interruptor(
                    "Conservar la miniatura al borrar videos", "El video se borra, pero queda su portada, la duración y el tamaño.", e.conservarPortada, true, Ico.Video,
                ) { si -> scope.launch { runCatching { Repo.fijarConservarPortada(botId, si) }; h.version++ } }
            }
        }

        // ---- galería ----
        CajaAviso(
            MaterialTheme.colorScheme.primary, "Guardar en la galería",
            "En el visor de una foto o video toca el icono de guardar. ${com.botchat.app.medios.Exportar.AVISO_SIN_CIFRAR}", "aviso_galeria", icono = Ico.Guardar,
        )

        // ---- limpiar con filtros ----
        LimpiezaConFiltros(botId, h) { h.version++ }
    }
}

/** Las hojas del bloque (elegir un tope, escribir otro, elegir una persona). Van en la caja que ocupa toda la pantalla. */
@Composable
fun HojasAlmacenamiento(botId: Long, ajustes: Ajustes, h: EstadoAlmacenE8) {
    val scope = rememberCoroutineScope()
    val d by ajustes.descargas.collectAsState()
    fun ponerTopeTotal(bytes: Long) { scope.launch { runCatching { Repo.fijarTopeTotal(botId, bytes) }; h.version++ } }
    val editando = h.editando
    val otro = h.otro
    val estado = h.estado
    if (h.eligiendoPersona) HojaElegirChat(botId, false, "Elegir persona", "limpiar_elegir", { h.eligiendoPersona = false }) { c -> h.eligiendoPersona = false; h.persona = c; h.version += 0 }

    editando?.let { ed ->
        val (opciones, actual) = when {
            ed.tipo == null -> TOPES_TOTAL to (estado?.topeTotal ?: 0L)
            ed.porArchivo -> TOPES_ARCHIVO to (d.topeArchivoMb[ed.tipo] ?: 0) * MIB
            else -> TOPES_TIPO to (d.topeTipoMb[ed.tipo] ?: 0) * MIB
        }
        HojaAlerta(
            onDismissRequest = { h.editando = null },
            title = { Text(if (ed.tipo == null) "Tope total" else (if (ed.porArchivo) "Tope por archivo: " else "Tope de todos: ") + NOMBRE_TIPO.getValue(ed.tipo)) },
            text = {
                SelectorValores(actual, opciones, "hoja_tope", { v ->
                    h.editando = null
                    if (ed.tipo == null) ponerTopeTotal(v) else guardarTopeTipo(ajustes, ed, v)
                }, alOtro = { h.otro = ed; h.editando = null })
            },
            confirmButton = { TextButton({ h.editando = null }, Modifier.testTag("hoja_tope_cerrar")) { Text("Cerrar") } },
        )
    }
    otro?.let { ed ->
        HojaNumero(
            "Otro tope", if (ed.tipo == null) "Gigabytes (GB)" else "Megabytes (MB)", 0, "otro_tope", { h.otro = null },
            { n -> h.otro = null; if (ed.tipo == null) ponerTopeTotal(n * GIB) else guardarTopeTipo(ajustes, ed, n * MIB) },
            ayuda = "Escribe 0 para quitar el límite.",
        )
    }
}

private fun cambiarRegla(ajustes: Ajustes, tipo: String, r: Ajustes.Regla) = ajustes.cambiarDescargas {
    when (tipo) { "foto" -> it.copy(foto = r); "video" -> it.copy(video = r); "audio" -> it.copy(audio = r); "documento" -> it.copy(documento = r); else -> it.copy(sticker = r) }
}

private fun guardarTopeTipo(ajustes: Ajustes, ed: EditandoTope, bytes: Long) {
    val tipo = ed.tipo ?: return
    val mb = mbA(bytes)
    ajustes.cambiarDescargas { x ->
        if (ed.porArchivo) x.copy(topeArchivoMb = if (mb > 0) x.topeArchivoMb + (tipo to mb) else x.topeArchivoMb - tipo)
        else x.copy(topeTipoMb = if (mb > 0) x.topeTipoMb + (tipo to mb) else x.topeTipoMb - tipo)
    }
}

/** «Limpiar con vista previa»: se elige tipo, persona y fechas, se estima cuánto se libera y recién entonces se borra (con confirmación). */
@Composable
private fun LimpiezaConFiltros(botId: Long, h: EstadoAlmacenE8, alCambio: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var categoria by remember { mutableStateOf("") }
    val persona = h.persona
    var fechas by remember { mutableStateOf(RangoFechas()) }
    var estimado by remember { mutableStateOf<Liberado?>(null) }
    var confirmando by remember { mutableStateOf(false) }
    fun filtro() = FiltroLimpieza(categoria, persona?.jid.orEmpty(), fechas.desde, fechas.hasta)
    fun cambiar(f: () -> Unit) { f(); estimado = null }

    Card(Modifier.fillMaxWidth().testTag("limpieza_filtros"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Text("Limpiar con vista previa", style = MaterialTheme.typography.titleMedium)
            Text("Elige qué quitar. Primero te digo cuánto se libera; los mensajes de texto se quedan.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Text("Tipo", style = MaterialTheme.typography.labelLarge)
            androidx.compose.foundation.layout.FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                listOf("" to "Todo", "foto" to "Fotos", "video" to "Videos", "audio" to "Audios", "documento" to "Documentos").forEach { (k, t) ->
                    FilterChip(categoria == k, { haptica(Toque.Suave); cambiar { categoria = k } }, { Text(t) }, modifier = Modifier.testTag("limpiar_tipo_${k.ifEmpty { "todo" }}"))
                }
            }
            Text("Persona", style = MaterialTheme.typography.labelLarge)
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalAlignment = Alignment.CenterVertically) {
                OutlinedButton({ h.eligiendoPersona = true }, Modifier.testTag("limpiar_persona")) { Icon(Ico.Persona, null, Modifier.size(18.dp)); Spacer(Modifier.width(6.dp)); Text(persona?.let { it.nombre.ifBlank { it.telefono } } ?: "Cualquiera") }
                if (persona != null) TextButton({ cambiar { h.persona = null } }, Modifier.testTag("limpiar_persona_quitar")) { Text("Quitar") }
            }
            Text("Fecha", style = MaterialTheme.typography.labelLarge)
            SelectorFechas(fechas, "limpiar_fecha") { cambiar { fechas = it } }
            estimado?.let {
                Text(
                    if (it.cantidad == 0) "No hay nada que borrar con esos filtros." else "Se liberarían ${formatoBytes(it.bytes)} (${plural(it.cantidad.toLong(), "archivo", "archivos")}).",
                    Modifier.testTag("limpieza_estimado"), style = MaterialTheme.typography.titleSmall, color = if (it.cantidad == 0) MaterialTheme.colorScheme.onSurfaceVariant else AMARILLO_SILENCIO,
                )
            }
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                OutlinedButton({ haptica(Toque.Suave); scope.launch { estimado = runCatching { Repo.estimarLimpieza(botId, filtro()) }.getOrNull() } }, Modifier.weight(1f).testTag("limpieza_estimar")) { Text("Estimar") }
                Button({ haptica(Toque.Fuerte); confirmando = true }, Modifier.weight(1f).testTag("limpieza_borrar"), enabled = (estimado?.cantidad ?: 0) > 0) { Icon(Ico.Borrar, null, Modifier.size(18.dp)); Spacer(Modifier.width(6.dp)); Text("Borrar") }
            }
        }
    }
    if (confirmando) AlertDialog(
        onDismissRequest = { confirmando = false }, title = { Text("¿Borrar estos archivos?") },
        text = { Text("Se liberarán ${formatoBytes(estimado?.bytes ?: 0)}. Los mensajes de texto se conservan. No se puede deshacer.") },
        confirmButton = {
            TextButton({
                confirmando = false
                scope.launch {
                    runCatching { Repo.borrarLimpieza(botId, filtro()) }.onSuccess { avisaToast(ctx, "Se liberaron ${formatoBytes(it.bytes)}") }.onFailure { avisaToast(ctx, it.message ?: "No se pudo borrar", true) }
                    estimado = null; alCambio()
                }
            }, Modifier.testTag("limpieza_confirmar")) { Text("Borrar", color = MaterialTheme.colorScheme.error) }
        },
        dismissButton = { TextButton({ confirmando = false }, Modifier.testTag("limpieza_cancelar")) { Text("Cancelar") } },
    )
}

/**
 * Aviso de la primera vez después de actualizar: si lo guardado ya pasa el tope, se va a borrar lo más viejo. Se puede subir el tope
 * o quitarlo antes de que pase. Sale una sola vez.
 */
@Composable
fun AvisoTopeAlmacen(estado: EstadoAlmacen, alSubir: (Long) -> Unit, alEntendido: () -> Unit) {
    val propuesto = ((estado.uso / GIB) + 2) * GIB
    AlertDialog(
        onDismissRequest = {}, modifier = Modifier.testTag("aviso_tope"),
        title = { Text("Ahora hay un tope de espacio") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("Bot Chat guarda ${formatoBytes(estado.uso)} de fotos, videos y archivos, y el tope es de ${formatoBytes(estado.topeTotal)}.")
                Text(if (estado.topePendiente) "Mientras no elijas, no se borra nada. Si dices «Está bien», se borrarán los archivos más viejos hasta quedar en el 90 %. Los mensajes de texto no se tocan, ni lo marcado."
                else "Si no haces nada, se borrarán los archivos más viejos hasta quedar en el 90 %. Los mensajes de texto no se tocan, ni lo marcado.",
                    Modifier.testTag("aviso_tope_texto"))
                TextButton({ alSubir(propuesto) }, Modifier.testTag("aviso_tope_subir")) { Text("Subir el tope a ${formatoBytes(propuesto)}") }
                TextButton({ alSubir(0) }, Modifier.testTag("aviso_tope_sin_limite")) { Text("Quitar el límite") }
            }
        },
        confirmButton = { TextButton(alEntendido, Modifier.testTag("aviso_tope_ok")) { Text("Está bien, borrar lo viejo") } },
    )
}
