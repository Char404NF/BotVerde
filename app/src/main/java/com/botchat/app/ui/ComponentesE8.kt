@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.DatePickerDialog
import androidx.compose.material3.DateRangePicker
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.rememberDateRangePickerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.ChatItem
import com.botchat.app.datos.Repo
import kotlinx.coroutines.delay
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId

// Piezas que comparten las pantallas de la etapa E8: avisos de color, elegir un valor con botones, elegir fechas y elegir una persona.

/** Un recuadro de color (verde / amarillo / rojo) con un título y un texto corto. */
@Composable
fun CajaAviso(color: Color, titulo: String, texto: String, etiqueta: String, modifier: Modifier = Modifier, icono: ImageVector = Ico.Aviso) {
    Row(
        modifier.fillMaxWidth().clip(RoundedCornerShape(16.dp)).background(color.copy(alpha = 0.16f)).padding(Esp.m).testTag(etiqueta),
        horizontalArrangement = Arrangement.spacedBy(Esp.m), verticalAlignment = Alignment.Top,
    ) {
        Icon(icono, null, Modifier.size(26.dp), tint = color)
        Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(titulo, style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
            Text(texto, style = MaterialTheme.typography.bodyMedium)
        }
    }
}

/** Una opción de un [SelectorValores]: el valor y lo que dice el botón. */
data class OpcionValor(val valor: Long, val texto: String)

/**
 * Elegir un número con botones grandes (por ejemplo el tope de espacio). Si el valor actual no está entre las opciones se marca «Otro».
 * `alOtro` se llama al tocar «Otro…» para que la pantalla pida el número en una hoja.
 */
@Composable
fun SelectorValores(actual: Long, opciones: List<OpcionValor>, etiqueta: String, alElegir: (Long) -> Unit, alOtro: (() -> Unit)? = null) {
    val haptica = rememberHaptica()
    FlowRow(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
        opciones.forEach { o ->
            FilterChip(actual == o.valor, { haptica(Toque.Suave); alElegir(o.valor) }, { Text(o.texto) }, modifier = Modifier.testTag("${etiqueta}_${o.valor}"))
        }
        if (alOtro != null) FilterChip(opciones.none { it.valor == actual }, { haptica(Toque.Suave); alOtro() }, { Text("Otro…") }, modifier = Modifier.testTag("${etiqueta}_otro"))
    }
}

/** Hoja para escribir un número (con su unidad). Devuelve el número entero; vacío o cero = `alAceptar(0)`. */
@Composable
fun HojaNumero(titulo: String, unidad: String, inicial: Long, etiqueta: String, alCerrar: () -> Unit, alAceptar: (Long) -> Unit, ayuda: String = "") {
    var texto by remember { mutableStateOf(if (inicial > 0) inicial.toString() else "") }
    HojaAlerta(
        onDismissRequest = alCerrar, title = { Text(titulo) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                OutlinedTextField(
                    texto, { texto = it.filter(Char::isDigit).take(7) }, Modifier.fillMaxWidth().testTag("${etiqueta}_campo"), singleLine = true,
                    label = { Text(unidad) }, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                )
                if (ayuda.isNotBlank()) Text(ayuda, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        },
        confirmButton = { TextButton({ alAceptar(texto.toLongOrNull() ?: 0L) }, Modifier.testTag("${etiqueta}_aceptar")) { Text("Listo") } },
        dismissButton = { TextButton(alCerrar, Modifier.testTag("${etiqueta}_cancelar")) { Text("Cancelar") } },
    )
}

/** Hoja con un campo de texto para escribir o cambiar un texto (por ejemplo un aviso). Vacío = «volver al de fábrica» si `permiteVacio`. */
@Composable
fun HojaTexto(
    titulo: String, inicial: String, etiqueta: String, alCerrar: () -> Unit, alAceptar: (String) -> Unit,
    ayuda: String = "", permiteVacio: Boolean = false, maximo: Int = 300, lineas: Int = 3, botonRestaurar: String? = null,
) {
    var texto by remember { mutableStateOf(inicial) }
    HojaAlerta(
        onDismissRequest = alCerrar, title = { Text(titulo) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                OutlinedTextField(texto, { texto = it.take(maximo) }, Modifier.fillMaxWidth().testTag("${etiqueta}_campo"), minLines = lineas)
                if (ayuda.isNotBlank()) Text(ayuda, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                if (botonRestaurar != null) TextButton({ alAceptar("") }, Modifier.testTag("${etiqueta}_fabrica")) { Text(botonRestaurar) }
            }
        },
        confirmButton = { TextButton({ alAceptar(texto.trim()) }, Modifier.testTag("${etiqueta}_aceptar"), enabled = permiteVacio || texto.isNotBlank()) { Text("Guardar") } },
        dismissButton = { TextButton(alCerrar, Modifier.testTag("${etiqueta}_cancelar")) { Text("Cancelar") } },
    )
}

internal fun inicioDeDiaE8(d: LocalDate): Long = d.atStartOfDay(ZoneId.systemDefault()).toEpochSecond()

/** Rango de fechas elegido: `clave` es el botón que está prendido; `desde` y `hasta` son segundos (0 = sin límite). */
data class RangoFechas(val clave: String = "todas", val desde: Long = 0, val hasta: Long = 0)

/** Botones de fecha («Cualquier fecha», «Más de 30 días»…) y, al tocar «Elegir fechas», un selector de rango. */
@Composable
fun SelectorFechas(rango: RangoFechas, etiqueta: String, alCambiar: (RangoFechas) -> Unit) {
    val haptica = rememberHaptica()
    var eligiendo by remember { mutableStateOf(false) }
    val hoy = LocalDate.now()
    FlowRow(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
        FilterChip(rango.clave == "todas", { haptica(Toque.Suave); alCambiar(RangoFechas()) }, { Text("Cualquier fecha") }, modifier = Modifier.testTag("${etiqueta}_todas"))
        FilterChip(rango.clave == "m30", { haptica(Toque.Suave); alCambiar(RangoFechas("m30", 0, inicioDeDiaE8(hoy.minusDays(30)))) }, { Text("Más de 30 días") }, modifier = Modifier.testTag("${etiqueta}_m30"))
        FilterChip(rango.clave == "m90", { haptica(Toque.Suave); alCambiar(RangoFechas("m90", 0, inicioDeDiaE8(hoy.minusDays(90)))) }, { Text("Más de 90 días") }, modifier = Modifier.testTag("${etiqueta}_m90"))
        FilterChip(rango.clave == "otro", { haptica(Toque.Suave); eligiendo = true }, { Text("Elegir fechas") }, modifier = Modifier.testTag("${etiqueta}_otro"))
    }
    if (eligiendo) {
        val estado = rememberDateRangePickerState()
        DatePickerDialog(
            onDismissRequest = { eligiendo = false },
            confirmButton = {
                TextButton({
                    val a = estado.selectedStartDateMillis
                    val b = estado.selectedEndDateMillis
                    if (a != null && b != null) {
                        alCambiar(RangoFechas("otro", inicioDeDiaE8(Instant.ofEpochMilli(a).atZone(ZoneId.of("UTC")).toLocalDate()), inicioDeDiaE8(Instant.ofEpochMilli(b).atZone(ZoneId.of("UTC")).toLocalDate().plusDays(1))))
                    }
                    eligiendo = false
                }, Modifier.testTag("${etiqueta}_aceptar")) { Text("Aceptar") }
            },
            dismissButton = { TextButton({ eligiendo = false }) { Text("Cancelar") } },
        ) { DateRangePicker(estado, Modifier.height(480.dp)) }
    }
}

/** Hoja para elegir una persona (o un chat) de la lista, con buscador. `grupos` = false: personas; true: grupos. */
@Composable
fun HojaElegirChat(botId: Long, grupos: Boolean, titulo: String, etiqueta: String, alCerrar: () -> Unit, alElegir: (ChatItem) -> Unit) {
    var consulta by remember { mutableStateOf("") }
    val lista by produceState<List<ChatItem>?>(null, consulta) {
        if (consulta.isNotBlank()) delay(200)
        value = runCatching { Repo.chats(botId, grupos, consulta, 200) }.getOrDefault(emptyList())
    }
    HojaAlerta(
        onDismissRequest = alCerrar, title = { Text(titulo) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                OutlinedTextField(consulta, { consulta = it }, Modifier.fillMaxWidth().testTag("${etiqueta}_buscar"), singleLine = true, placeholder = { Text("Buscar") }, leadingIcon = { Icon(Ico.Buscar, null) })
                val datos = lista
                when {
                    datos == null -> FilasDeCarga(3)
                    datos.isEmpty() -> Text("No encontré a nadie.", color = MaterialTheme.colorScheme.onSurfaceVariant)
                    else -> LazyColumn(Modifier.fillMaxWidth().height(320.dp)) {
                        items(datos.take(100), key = { it.jid }) { c ->
                            Row(Modifier.fillMaxWidth().clickable { alElegir(c) }.padding(vertical = Esp.s).testTag("${etiqueta}_${c.jid}"), verticalAlignment = Alignment.CenterVertically) {
                                Avatar(c.nombre, c.foto, c.fotoTs, c.jid, 40.dp)
                                Spacer(Modifier.width(Esp.m))
                                Column {
                                    Text(c.nombre.ifBlank { c.telefono }, maxLines = 1, overflow = TextOverflow.Ellipsis, style = MaterialTheme.typography.titleSmall)
                                    if (!grupos && c.telefono.isNotBlank()) Text(c.telefono, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                }
                            }
                        }
                    }
                }
            }
        },
        confirmButton = {},
        dismissButton = { TextButton(alCerrar, Modifier.testTag("${etiqueta}_cancelar")) { Text("Cancelar") } },
    )
}

/** Un título de sección pequeño para las pantallas de E8. */
@Composable
fun TituloSeccionE8(texto: String, modifier: Modifier = Modifier) {
    Text(texto, modifier.padding(top = Esp.s), style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.primary)
}

internal fun textoTope(bytes: Long, sin: String = "Sin límite"): String = if (bytes <= 0) sin else formatoBytes(bytes)
internal fun mbA(bytes: Long): Int = (bytes / (1024 * 1024)).toInt()
internal const val MIB = 1024L * 1024L
internal const val GIB = 1024L * MIB
