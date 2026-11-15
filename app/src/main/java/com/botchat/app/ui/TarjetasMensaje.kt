package com.botchat.app.ui

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.graphics.drawable.Animatable
import android.net.Uri
import android.provider.ContactsContract
import android.widget.ImageView
import android.widget.Toast
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.Color
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.ui.viewinterop.AndroidView
import com.botchat.app.datos.Json
import com.botchat.app.datos.Msg
import com.botchat.app.datos.Repo
import com.botchat.app.medios.Stickers
import java.util.Locale

@Composable
private fun Icono(icono: androidx.compose.ui.graphics.vector.ImageVector) {
    Box(Modifier.size(44.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primary.copy(alpha = 0.18f)), Alignment.Center) { Icon(icono, null, Modifier.size(24.dp)) }
}

private fun intentaAbrir(ctx: Context, i: Intent, sinApp: String) {
    try { ctx.startActivity(i.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) } catch (_: Exception) { Toast.makeText(ctx, sinApp, Toast.LENGTH_SHORT).show() }
}

/** Contacto compartido: nombre, teléfonos, copiar el número y agregarlo a la agenda del teléfono. */
@Composable
fun TarjetaContacto(m: Msg) {
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    val contactos = remember(m.texto) { Json.contactos(m.texto) }
    if (contactos == null) { Text(m.texto); return }
    Column(Modifier.widthIn(min = 220.dp, max = 300.dp).testTag("tarjeta_contacto"), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
        contactos.forEach { c ->
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icono(Ico.Persona)
                Spacer(Modifier.width(10.dp))
                Column(Modifier.weight(1f)) {
                    Text(c.nombre.ifBlank { "Contacto sin nombre" }, fontWeight = FontWeight.SemiBold)
                    c.telefonos.forEach { Text(it, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                }
            }
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
                val tel = c.telefonos.firstOrNull()
                if (tel != null) TextButton({
                    haptica(Toque.Suave)
                    ctx.getSystemService(ClipboardManager::class.java)?.setPrimaryClip(ClipData.newPlainText("Teléfono", tel))
                    Toast.makeText(ctx, "Número copiado", Toast.LENGTH_SHORT).show()
                }, Modifier.testTag("copiar_numero")) { Text("Copiar número") }
                TextButton({
                    haptica(Toque.Suave)
                    val i = Intent(ContactsContract.Intents.Insert.ACTION).apply {
                        type = ContactsContract.RawContacts.CONTENT_TYPE
                        putExtra(ContactsContract.Intents.Insert.NAME, c.nombre)
                        if (tel != null) putExtra(ContactsContract.Intents.Insert.PHONE, tel)
                    }
                    intentaAbrir(ctx, i, "No hay una app de contactos")
                }, Modifier.testTag("agregar_contacto")) { Text("Agregar") }
            }
        }
    }
}

/** Ubicación compartida: lugar, coordenadas y botón para verla en la app de mapas que tenga el teléfono. */
@Composable
fun TarjetaUbicacion(m: Msg) {
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    val u = remember(m.texto) { Json.ubicacion(m.texto) }
    if (u == null) { Text(m.texto); return }
    val coords = String.format(Locale.US, "%.5f, %.5f", u.lat, u.lon)
    Column(Modifier.widthIn(min = 220.dp, max = 300.dp).testTag("tarjeta_ubicacion")) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icono(Ico.Ubicacion)
            Spacer(Modifier.width(10.dp))
            Column(Modifier.weight(1f)) {
                Text(if (u.vivo) "Ubicación en tiempo real" else u.nombre.ifBlank { "Ubicación" }, fontWeight = FontWeight.SemiBold)
                if (u.direccion.isNotBlank()) Text(u.direccion, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                Text(coords, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                if (u.vivo) Text("Se muestra el último punto recibido", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.End) {
            TextButton({
                haptica(Toque.Suave)
                val etiqueta = Uri.encode(u.nombre.ifBlank { "Ubicación" })
                val geo = String.format(Locale.US, "geo:%f,%f?q=%f,%f(%s)", u.lat, u.lon, u.lat, u.lon, etiqueta)
                intentaAbrir(ctx, Intent(Intent.ACTION_VIEW, Uri.parse(geo)), "No hay una app de mapas")
            }, Modifier.testTag("abrir_mapa")) { Text("Abrir en el mapa") }
        }
    }
}

/** Mensaje que esta versión no sabe dibujar: nunca se muestra como error. */
@Composable
fun TarjetaEspecial() {
    Row(Modifier.widthIn(min = 220.dp, max = 300.dp).testTag("tarjeta_especial"), verticalAlignment = Alignment.CenterVertically) {
        Icono(Ico.Info)
        Spacer(Modifier.width(10.dp))
        Column {
            Text("Mensaje especial", fontWeight = FontWeight.SemiBold)
            Text("No se puede mostrar aquí. Ábrelo en tu teléfono.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

/** Sticker descargado; si es animado se mueve (salvo que se hayan reducido las animaciones). */
@Composable
fun StickerAnimado(m: Msg) {
    val ctx = LocalContext.current
    val reducir = LocalReducirMovimiento.current
    var dibujo by remember(m.ruta) { mutableStateOf<android.graphics.drawable.Drawable?>(null) }
    var fallo by remember(m.ruta) { mutableStateOf(false) }
    LaunchedEffect(m.ruta) {
        dibujo = Stickers.cargar(ctx, Repo.archivo(m.ruta))
        fallo = dibujo == null
    }
    Box(Modifier.size(160.dp).testTag("sticker"), Alignment.Center) {
        val d = dibujo
        when {
            d != null -> AndroidView(
                factory = { c -> ImageView(c).apply { scaleType = ImageView.ScaleType.FIT_CENTER; contentDescription = "Sticker" } },
                update = { v ->
                    v.setImageDrawable(d)
                    if (d is Animatable && !reducir) {
                        if (android.os.Build.VERSION.SDK_INT >= 28 && d is android.graphics.drawable.AnimatedImageDrawable) d.repeatCount = android.graphics.drawable.AnimatedImageDrawable.REPEAT_INFINITE
                        d.start()
                    }
                },
                onRelease = { v -> (v.drawable as? Animatable)?.stop() },
                modifier = Modifier.size(160.dp),
            )
            fallo -> Icon(Ico.Foto, "Sticker", Modifier.size(48.dp))
            else -> Box(Modifier.size(160.dp).clip(RoundedCornerShape(12.dp)).shimmer())
        }
    }
}

/**
 * «Video eliminado»: el video se borró para dejar espacio (o a mano), pero se conserva su portada, la duración, el tamaño, la fecha y
 * quién lo mandó. No se puede reproducir; si WhatsApp todavía lo tiene, se puede volver a pedir.
 */
@Composable
fun TarjetaVideoEliminado(m: Msg) {
    val ctx = LocalContext.current
    var portada by remember(m.portada) { mutableStateOf<androidx.compose.ui.graphics.ImageBitmap?>(null) }
    LaunchedEffect(m.portada) {
        portada = if (m.portada.isEmpty()) null else runCatching { com.botchat.app.medios.CacheImagenes.cargar(ctx, Repo.archivo(m.portada), 360)?.asImageBitmap() }.getOrNull()
    }
    val motivo = when (m.borradoMotivo) { "tope" -> "para dejar espacio"; "limpieza", "manual" -> "al limpiar"; "descarga" -> "al enviarse"; "retencion", "retención" -> "por la retención"; else -> "" }
    Column(Modifier.widthIn(min = 220.dp, max = 280.dp).testTag("tarjeta_video_eliminado"), verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Box(Modifier.fillMaxWidth().aspectRatio(if (m.ancho > 0 && m.alto > 0) (m.ancho.toFloat() / m.alto).coerceIn(0.6f, 1.8f) else 1.33f).clip(RoundedCornerShape(12.dp)).background(MaterialTheme.colorScheme.surfaceContainerHighest), Alignment.Center) {
            portada?.let { androidx.compose.foundation.Image(it, null, Modifier.fillMaxSize(), contentScale = androidx.compose.ui.layout.ContentScale.Crop) }
            Box(Modifier.fillMaxSize().background(Color.Black.copy(alpha = 0.35f)))
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Icon(Ico.Video, null, Modifier.size(34.dp), tint = Color.White)
                Text("Video eliminado", Modifier.testTag("video_eliminado_titulo"), color = Color.White, fontWeight = FontWeight.SemiBold)
            }
            if (m.dur > 0) Text(duracion(m.dur), Modifier.align(Alignment.BottomStart).padding(6.dp).clip(RoundedCornerShape(6.dp)).background(Color.Black.copy(alpha = 0.55f)).padding(horizontal = 6.dp, vertical = 2.dp).testTag("video_eliminado_duracion"), color = Color.White, style = MaterialTheme.typography.labelSmall)
        }
        Text(
            listOfNotNull(m.bytes.takeIf { it > 0 }?.let { formatoBytes(it) }, etiquetaDia(m.ts), m.remitente.takeIf { it.isNotBlank() && !m.esMio }).joinToString(" · "),
            Modifier.testTag("video_eliminado_datos"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        if (motivo.isNotEmpty()) Text("Se borró $motivo" + if (m.borradoTs > 0) " · ${etiquetaDia(m.borradoTs)}" else "", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}
