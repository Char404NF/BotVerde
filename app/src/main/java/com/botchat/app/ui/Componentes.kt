package com.botchat.app.ui

import android.graphics.Bitmap
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.botchat.app.Paises
import com.botchat.app.datos.ChatItem
import com.botchat.app.datos.Msg
import com.botchat.app.datos.Repo
import com.botchat.app.medios.CacheImagenes
import java.text.NumberFormat
import java.time.Instant
import java.time.LocalDate
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.time.format.TextStyle
import java.util.Locale

// ---------- formatos ----------

private val ES = Locale.forLanguageTag("es")

fun formatoBytes(b: Long): String {
    val nf = NumberFormat.getNumberInstance(Locale.getDefault()).apply { maximumFractionDigits = 1; minimumFractionDigits = 0 }
    return when {
        b < 1024 -> "$b B"
        b < 1024 * 1024 -> nf.format(b / 1024.0) + " KB"
        b < 1024L * 1024 * 1024 -> nf.format(b / (1024.0 * 1024)) + " MB"
        else -> nf.format(b / (1024.0 * 1024 * 1024)) + " GB"
    }
}

private fun fecha(ts: Long): LocalDate = Instant.ofEpochSecond(ts).atZone(ZoneId.systemDefault()).toLocalDate()

fun horaDe(ts: Long): String = DateTimeFormatter.ofPattern("HH:mm").format(Instant.ofEpochSecond(ts).atZone(ZoneId.systemDefault()))

/** Hora para la lista de chats: hoy la hora, ayer "Ayer", esta semana el día, antes la fecha. */
fun horaLista(ts: Long, hoy: LocalDate = LocalDate.now()): String {
    if (ts <= 0) return ""
    val d = fecha(ts)
    return when {
        d == hoy -> horaDe(ts)
        d == hoy.minusDays(1) -> "Ayer"
        d.isAfter(hoy.minusDays(7)) -> d.dayOfWeek.getDisplayName(TextStyle.FULL, ES).replaceFirstChar { it.uppercase(ES) }
        else -> DateTimeFormatter.ofPattern("dd/MM/yy").format(d)
    }
}

/** Etiqueta del separador de día en el chat. */
fun etiquetaDia(ts: Long, hoy: LocalDate = LocalDate.now()): String {
    val d = fecha(ts)
    return when {
        d == hoy -> "Hoy"
        d == hoy.minusDays(1) -> "Ayer"
        d.isAfter(hoy.minusDays(7)) -> d.dayOfWeek.getDisplayName(TextStyle.FULL, ES).replaceFirstChar { it.uppercase(ES) }
        d.year == hoy.year -> DateTimeFormatter.ofPattern("d 'de' MMMM", ES).format(d)
        else -> DateTimeFormatter.ofPattern("d 'de' MMMM 'de' yyyy", ES).format(d)
    }
}

fun mismoDia(a: Long, b: Long) = fecha(a) == fecha(b)

/** Número de día (en la zona del teléfono) de una hora; sirve para comparar días sin crear fechas por cada comparación. */
fun diaDe(ts: Long): Long = fecha(ts).toEpochDay()

fun duracion(seg: Int): String = "%d:%02d".format(seg / 60, seg % 60)

// ---------- avatar ----------

fun colorEstable(clave: String, oscuro: Boolean): Color {
    val tono = ((clave.hashCode().toLong() and 0xffffffffL) % 360L).toFloat()
    return Color.hsl(tono, if (oscuro) 0.38f else 0.5f, if (oscuro) 0.34f else 0.42f)
}

fun inicial(nombre: String): String {
    val c = nombre.firstOrNull { it.isLetterOrDigit() } ?: return "?"
    return c.uppercaseChar().toString()
}

/** Foto de perfil redonda; si no hay, un círculo con la inicial y un color estable por persona. */
@Composable
fun Avatar(nombre: String, foto: String, fotoTs: Long, clave: String, tam: Dp, modifier: Modifier = Modifier) {
    val ctx = LocalContext.current
    val px = with(LocalDensity.current) { tam.roundToPx() * 2 }
    var imagen by remember(foto, fotoTs) { mutableStateOf<ImageBitmap?>(null) }
    LaunchedEffect(foto, fotoTs, px) {
        imagen = if (foto.isEmpty()) null else CacheImagenes.cargar(ctx, Repo.archivo(foto), px, fotoTs.toString())?.asImageBitmap()
    }
    val oscuro = isSystemInDarkTheme()
    Box(modifier.size(tam).clip(CircleShape).background(colorEstable(clave, oscuro)), contentAlignment = Alignment.Center) {
        val img = imagen
        if (img != null) {
            Image(img, null, Modifier.size(tam), contentScale = ContentScale.Crop)
        } else {
            Text(inicial(nombre), color = Color.White, fontSize = (tam.value * 0.42f).sp, fontWeight = FontWeight.SemiBold)
        }
    }
}

// ---------- brillo de carga ----------

fun Modifier.shimmer(): Modifier = composed {
    val reducir = LocalReducirMovimiento.current
    val base = MaterialTheme.colorScheme.surfaceContainerHigh
    val luz = MaterialTheme.colorScheme.surfaceContainerHighest
    if (reducir) {
        background(base)
    } else {
        val t = rememberInfiniteTransition(label = "shimmer")
        val x by t.animateFloat(0f, 1f, infiniteRepeatable(tween(1100, easing = LinearEasing), RepeatMode.Restart), label = "x")
        val ancho = 600f
        background(Brush.linearGradient(listOf(base, luz, base), start = androidx.compose.ui.geometry.Offset(x * 1800f - ancho, 0f), end = androidx.compose.ui.geometry.Offset(x * 1800f, ancho / 2)))
    }
}

/** Filas de relleno con brillo mientras llegan los datos. */
@Composable
fun FilasDeCarga(cantidad: Int = 8) {
    Column {
        repeat(cantidad) {
            Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.m), verticalAlignment = Alignment.CenterVertically) {
                Box(Modifier.size(52.dp).clip(CircleShape).shimmer())
                Spacer(Modifier.width(Esp.l))
                Column(Modifier.weight(1f)) {
                    Box(Modifier.fillMaxWidth(0.5f).height(14.dp).clip(RoundedCornerShape(7.dp)).shimmer())
                    Spacer(Modifier.height(10.dp))
                    Box(Modifier.fillMaxWidth(0.85f).height(12.dp).clip(RoundedCornerShape(6.dp)).shimmer())
                }
            }
        }
    }
}

// ---------- háptica ----------

enum class Toque { Suave, Fuerte, Confirmar, Rechazar }

@Composable
fun rememberHaptica(): (Toque) -> Unit {
    val h = LocalHapticFeedback.current
    return remember(h) {
        { t ->
            runCatching {
                h.performHapticFeedback(
                    when (t) {
                        Toque.Suave -> HapticFeedbackType.TextHandleMove
                        Toque.Fuerte -> HapticFeedbackType.LongPress
                        Toque.Confirmar -> HapticFeedbackType.Confirm
                        Toque.Rechazar -> HapticFeedbackType.Reject
                    },
                )
            }
        }
    }
}

// ---------- varios ----------

@Composable
fun EstadoVacio(icono: ImageVector, titulo: String, texto: String, modifier: Modifier = Modifier) {
    Column(modifier.fillMaxWidth().padding(Esp.xl), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
        Box(Modifier.size(84.dp).clip(CircleShape).background(MaterialTheme.colorScheme.secondaryContainer), contentAlignment = Alignment.Center) {
            Icon(icono, null, Modifier.size(40.dp), tint = MaterialTheme.colorScheme.onSecondaryContainer)
        }
        Spacer(Modifier.height(Esp.l))
        Text(titulo, style = MaterialTheme.typography.titleLarge, textAlign = TextAlign.Center)
        Spacer(Modifier.height(Esp.s))
        Text(texto, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, textAlign = TextAlign.Center)
    }
}

/** Icono según el tipo de mensaje, o null si es texto. */
fun iconoDeTipo(tipo: String, notaVoz: Boolean = false): ImageVector? = when (tipo) {
    "imagen" -> Ico.Foto
    "sticker" -> Ico.Sticker
    "video" -> Ico.Video
    "audio" -> Ico.Audio
    "documento" -> Ico.Documento
    "ver_una_vez" -> Ico.Candado
    "contacto" -> Ico.Persona
    "ubicacion" -> Ico.Ubicacion
    else -> null
}

/** Texto corto que describe un mensaje (para la lista de chats y las respuestas). */
fun descripcionCorta(tipo: String, texto: String, borrado: Boolean, notaVoz: Boolean = false): String {
    if (borrado) return "Mensaje borrado"
    val etiqueta = when (tipo) {
        "imagen" -> "Foto"
        "sticker" -> "Sticker"
        "video" -> "Video"
        "audio" -> if (notaVoz) "Nota de voz" else "Audio"
        "documento" -> "Documento"
        "ver_una_vez" -> return "Ver una vez"
        "otro" -> return "Mensaje especial"
        "contacto" -> return com.botchat.app.datos.Json.contactos(texto)?.let { l -> "Contacto: " + l.joinToString(", ") { it.nombre } } ?: texto
        "ubicacion" -> return com.botchat.app.datos.Json.ubicacion(texto)?.let { u -> if (u.vivo) "Ubicación en tiempo real" else if (u.nombre.isNotBlank()) "Ubicación: " + u.nombre else "Ubicación" } ?: texto
        else -> return texto.replace('\n', ' ')
    }
    return if (texto.isBlank()) etiqueta else "$etiqueta · ${texto.replace('\n', ' ')}"
}

@Composable
fun TextoUnaLinea(texto: String, modifier: Modifier = Modifier, color: Color = MaterialTheme.colorScheme.onSurfaceVariant, estilo: androidx.compose.ui.text.TextStyle = MaterialTheme.typography.bodyMedium) {
    Text(texto, modifier, color = color, style = estilo, maxLines = 1, overflow = TextOverflow.Ellipsis)
}

/** Nombre del chat con la bandera del país junto al teléfono. */
fun banderaTelefono(c: ChatItem): String = if (c.esGrupo || c.pais.isEmpty()) c.telefono else "${Paises.banderaDe(c.pais)}  ${c.telefono}"

fun Bitmap.aImagen(): ImageBitmap = asImageBitmap()

/** ¿Este mensaje lleva un archivo que se ve como imagen o video en el visor? */
val Msg.esVisual: Boolean get() = categoria == "foto" || categoria == "video"

/** Logo de la app (vector del ícono, con sus colores). */
@Composable
fun LogoBot(modifier: Modifier = Modifier) {
    Image(androidx.compose.ui.res.painterResource(com.botchat.app.R.drawable.ic_logo), null, modifier)
}

/** Muestra un aviso breve desde cualquier hilo (siempre se pide al hilo principal). */
fun avisaToast(ctx: android.content.Context, texto: String?, largo: Boolean = false) {
    val t = texto ?: return
    android.os.Handler(android.os.Looper.getMainLooper()).post { android.widget.Toast.makeText(ctx, t, if (largo) android.widget.Toast.LENGTH_LONG else android.widget.Toast.LENGTH_SHORT).show() }
}
