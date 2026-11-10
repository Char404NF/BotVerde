package com.botchat.app.ui

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.drawText
import androidx.compose.ui.text.rememberTextMeasurer
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import java.text.NumberFormat
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.util.Locale

internal val LocaleEs: Locale = Locale.forLanguageTag("es")

private val enteros by lazy { NumberFormat.getIntegerInstance(LocaleEs) }

fun numero(n: Long): String = enteros.format(n)

/** "12 de octubre de 2026" en la zona del teléfono. */
fun fechaLarga(ts: Long): String =
    DateTimeFormatter.ofPattern("d 'de' MMMM 'de' yyyy", LocaleEs).format(Instant.ofEpochSecond(ts).atZone(ZoneId.systemDefault()))

/** Nombre en plural de un tipo de mensaje para las estadísticas. */
fun nombreTipo(tipo: String): String = when (tipo) {
    "texto" -> "Mensajes de texto"
    "imagen" -> "Fotos"
    "sticker" -> "Stickers"
    "video" -> "Videos"
    "audio" -> "Audios y notas de voz"
    "documento" -> "Documentos"
    "contacto" -> "Contactos"
    "ubicacion" -> "Ubicaciones"
    "encuesta" -> "Encuestas"
    "ver_una_vez" -> "Ver una vez"
    else -> "Mensajes especiales"
}

fun iconoDeEstadistica(tipo: String): ImageVector = when (tipo) {
    "imagen", "sticker" -> Ico.Foto
    "video" -> Ico.Video
    "audio" -> Ico.Audio
    "documento" -> Ico.Documento
    "contacto" -> Ico.Persona
    "ubicacion" -> Ico.Ubicacion
    "ver_una_vez" -> Ico.Candado
    "texto" -> Ico.Chats
    else -> Ico.Info
}

/** Tarjeta con título para agrupar una parte de las estadísticas. */
@Composable
fun SeccionEst(titulo: String, modifier: Modifier = Modifier, contenido: @Composable ColumnScope.() -> Unit) {
    Card(modifier.fillMaxWidth(), shape = MaterialTheme.shapes.large, colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Text(titulo, style = MaterialTheme.typography.titleSmall)
            contenido()
        }
    }
}

/** Aviso fijo de desde cuándo hay estadísticas. */
@Composable
fun NotaDesde(desde: Long, listo: Boolean, modifier: Modifier = Modifier) {
    val texto = buildString {
        append(if (desde > 0) "Estadísticas desde el ${fechaLarga(desde)}" else "Todavía no hay mensajes guardados")
        if (!listo) append(" · contando los mensajes anteriores…")
    }
    Text(texto, modifier.testTag("nota_desde"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
}

/** Número grande con su nombre. */
@Composable
fun Cifra(valor: Long, nombre: String, modifier: Modifier = Modifier) {
    Card(modifier, shape = MaterialTheme.shapes.large, colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.secondaryContainer)) {
        Column(Modifier.fillMaxWidth().padding(Esp.l)) {
            Text(numero(valor), style = MaterialTheme.typography.headlineMedium, color = MaterialTheme.colorScheme.onSecondaryContainer)
            Text(nombre, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSecondaryContainer)
        }
    }
}

/** Fila de un tipo de mensaje con sus cantidades. */
@Composable
fun FilaTipo(t: com.botchat.app.datos.TipoCuenta) {
    Row(Modifier.fillMaxWidth().testTag("tipo_${t.tipo}"), verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.size(36.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primaryContainer), Alignment.Center) {
            Icon(iconoDeEstadistica(t.tipo), null, Modifier.size(20.dp), tint = MaterialTheme.colorScheme.onPrimaryContainer)
        }
        Column(Modifier.weight(1f).padding(start = Esp.m)) {
            Text(nombreTipo(t.tipo), style = MaterialTheme.typography.bodyLarge)
            val partes = buildList {
                if (t.enviados > 0) add("${numero(t.enviados)} enviados")
                if (t.recibidos > 0) add("${numero(t.recibidos)} recibidos")
                if (t.bytes > 0) add(formatoBytes(t.bytes))
            }
            Text(partes.joinToString(" · "), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

/** Barra horizontal proporcional (para listas ordenadas como "quién escribe más"). */
@Composable
fun BarraRelativa(fraccion: Float, modifier: Modifier = Modifier) {
    val color = MaterialTheme.colorScheme.primary
    val fondo = MaterialTheme.colorScheme.surfaceContainerHighest
    Canvas(modifier.fillMaxWidth().height(8.dp)) {
        drawRoundRect(fondo, Offset.Zero, size, CornerRadius(size.height / 2))
        drawRoundRect(color, Offset.Zero, Size((size.width * fraccion.coerceIn(0f, 1f)).coerceAtLeast(if (fraccion > 0f) size.height else 0f), size.height), CornerRadius(size.height / 2))
    }
}

/** Gráfico de barras dibujado con Canvas. `etiquetas` puede traer textos vacíos para no saturar el eje. */
@Composable
fun GraficoBarras(valores: List<Long>, etiquetas: List<String>, descripcion: String, modifier: Modifier = Modifier, alto: Dp = 120.dp) {
    val color = MaterialTheme.colorScheme.primary
    val pista = MaterialTheme.colorScheme.surfaceContainerHighest
    val medidor = rememberTextMeasurer()
    val estilo = MaterialTheme.typography.labelSmall.copy(color = MaterialTheme.colorScheme.onSurfaceVariant)
    val maximo = (valores.maxOrNull() ?: 0L).coerceAtLeast(1L)
    Canvas(modifier.fillMaxWidth().height(alto).semantics { contentDescription = descripcion }) {
        val n = valores.size
        if (n == 0) return@Canvas
        val base = 16.dp.toPx()
        val h = size.height - base
        val casilla = size.width / n
        val ancho = casilla * 0.62f
        valores.forEachIndexed { i, v ->
            val x = i * casilla + (casilla - ancho) / 2
            drawRoundRect(pista, Offset(x, 0f), Size(ancho, h), CornerRadius(ancho / 4))
            if (v > 0) {
                val alta = maxOf(h * v / maximo, 3.dp.toPx())
                drawRoundRect(color, Offset(x, h - alta), Size(ancho, alta), CornerRadius(ancho / 4))
            }
            etiquetas.getOrNull(i)?.takeIf { it.isNotEmpty() }?.let { e ->
                val r = medidor.measure(e, estilo)
                drawText(r, topLeft = Offset(i * casilla + (casilla - r.size.width) / 2f, h + 2.dp.toPx()))
            }
        }
    }
}

internal fun primeraMayuscula(s: String) = s.replaceFirstChar { it.uppercase(LocaleEs) }

internal fun etiquetasHoras(): List<String> = List(24) { if (it % 6 == 0) it.toString() else "" }
internal val ETIQUETAS_SEMANA = listOf("L", "M", "X", "J", "V", "S", "D")
internal val NOMBRES_SEMANA = listOf("lunes", "martes", "miércoles", "jueves", "viernes", "sábado", "domingo")
