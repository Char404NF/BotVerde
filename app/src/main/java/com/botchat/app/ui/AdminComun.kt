@file:OptIn(ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.graphics.vector.ImageVector
import com.botchat.app.datos.ConteoMiembro
import com.botchat.app.datos.MiembroAdmin

// Piezas compartidas de Administrar grupo: la lista de miembros, el perfil y el resumen las usan.

/** «hace 5 min», «hace 2 h», «hace 3 d» o la fecha si pasó más de un mes. */
internal fun hace(ts: Long, ahora: Long = System.currentTimeMillis() / 1000): String {
    if (ts <= 0) return "nunca"
    val d = (ahora - ts).coerceAtLeast(0)
    return when {
        d < 60 -> "hace un momento"
        d < 3600 -> "hace ${d / 60} min"
        d < 86400 -> "hace ${d / 3600} h"
        d < 30 * 86400L -> "hace ${d / 86400} d"
        else -> fechaLarga(ts)
    }
}

/** Línea de actividad: «Último mensaje hace 2 h» o «Aún no ha escrito». */
internal fun textoUltimoMensaje(c: ConteoMiembro): String =
    if (c.ultimoTs > 0) "Último mensaje ${hace(c.ultimoTs)}" else "Aún no ha escrito"

/** «En línea», «Últ. vez hace 3 h» o vacío si esa persona no deja ver su estado. */
internal fun textoEnLinea(m: MiembroAdmin): String = when {
    m.enLinea -> "En línea"
    m.ultimaVez > 0 -> "Últ. vez ${hace(m.ultimaVez)}"
    else -> ""
}

internal val VERDE_EN_LINEA = VERDE_SILENCIO

/** La foto de la persona con un puntito verde si está en línea. */
@Composable
fun AvatarEnLinea(m: MiembroAdmin, tam: Dp, modifier: Modifier = Modifier) {
    Box(modifier) {
        Avatar(m.titulo, m.foto, 0, m.jid.ifBlank { m.persona }, tam)
        if (m.enLinea) {
            Box(
                Modifier.align(Alignment.BottomEnd).size(tam / 4).clip(CircleShape).background(MaterialTheme.colorScheme.background).testTag("en_linea_${m.persona}"),
                Alignment.Center,
            ) { Box(Modifier.size(tam / 4 - 4.dp).clip(CircleShape).background(VERDE_EN_LINEA)) }
        }
    }
}

@Composable
private fun DatoConteo(icono: ImageVector, n: Int, descripcion: String) {
    Row(Modifier.semantics { contentDescription = "$n $descripcion" }, verticalAlignment = Alignment.CenterVertically) {
        Icon(icono, null, Modifier.size(15.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.width(3.dp))
        Text(numero(n.toLong()), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

/** Cuántos mensajes, fotos, videos, audios y notas de voz envió. Los que están en cero se atenúan para que no estorben. */
@Composable
fun ConteoIconos(c: ConteoMiembro, modifier: Modifier = Modifier) {
    FlowRow(modifier, horizontalArrangement = Arrangement.spacedBy(12.dp), verticalArrangement = Arrangement.spacedBy(2.dp)) {
        DatoConteo(Ico.Chats, c.mensajes, "mensajes")
        DatoConteo(Ico.Foto, c.imagenes, "imágenes")
        DatoConteo(Ico.Video, c.videos, "videos")
        DatoConteo(Ico.Audio, c.audios, "audios")
        DatoConteo(Ico.Microfono, c.notasVoz, "notas de voz")
        if (c.stickers > 0) DatoConteo(Ico.Sticker, c.stickers, "stickers")
        if (c.documentos > 0) DatoConteo(Ico.Documento, c.documentos, "documentos")
    }
}

/** Insignia del cargo dentro del grupo (creador o administrador). */
@Composable
fun InsigniaCargo(m: MiembroAdmin, modifier: Modifier = Modifier) {
    if (!m.esAdmin) return
    Text(
        if (m.esCreador) "Creó el grupo" else "Admin",
        modifier.clip(androidx.compose.foundation.shape.RoundedCornerShape(50)).background(MaterialTheme.colorScheme.primaryContainer).padding(horizontal = 8.dp, vertical = 2.dp),
        style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onPrimaryContainer,
    )
}
