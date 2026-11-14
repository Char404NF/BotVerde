@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Checkbox
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
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
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.ChatItem
import com.botchat.app.datos.InfoMsg
import com.botchat.app.datos.Msg
import com.botchat.app.datos.Reaccion
import com.botchat.app.datos.Repo

internal val AzulLeido = Color(0xFF34B7F1)

/** Palomitas de un mensaje mío: reloj (enviando), una gris (enviado), dos grises (entregado), dos azules (leído) o aviso (fallido). */
@Composable
fun Palomitas(estado: String, tinta: Color, modifier: Modifier = Modifier) {
    val (icono, color, desc) = when (estado) {
        "enviando" -> Triple(Ico.Reloj, tinta.copy(alpha = 0.6f), "Enviando")
        "entregado" -> Triple(Ico.PalomitaDoble, tinta.copy(alpha = 0.6f), "Entregado")
        "leido" -> Triple(Ico.PalomitaDoble, AzulLeido, "Leído")
        "fallido" -> Triple(Ico.Aviso, MaterialTheme.colorScheme.error, "No se envió")
        else -> Triple(Ico.Palomita, tinta.copy(alpha = 0.6f), "Enviado")
    }
    Icon(icono, desc, modifier.size(15.dp).testTag("estado_$estado"), tint = color)
}

/** Recuadro de la cita arriba de un mensaje; al tocarlo se va al original. */
@Composable
fun BloqueCita(m: Msg, tinta: Color, alTocar: () -> Unit) {
    val autor = if (m.citaAutor == "yo") "Tú" else m.citaAutor
    Row(
        Modifier.fillMaxWidth().padding(bottom = 4.dp).clip(RoundedCornerShape(8.dp)).background(tinta.copy(alpha = 0.10f)).clickable { alTocar() }.testTag("cita_${m.id}"),
    ) {
        Box(Modifier.width(4.dp).heightIn(min = 36.dp).background(MaterialTheme.colorScheme.primary))
        Column(Modifier.padding(horizontal = 8.dp, vertical = 4.dp)) {
            if (autor.isNotBlank()) Text(autor, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.primary, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(m.citaTexto.ifBlank { "Mensaje" }, style = MaterialTheme.typography.bodySmall, maxLines = 2, overflow = TextOverflow.Ellipsis, color = tinta.copy(alpha = 0.8f))
        }
    }
}

/** Reacciones de un mensaje, agrupadas por emoji. */
@Composable
fun FilaReacciones(rs: List<Reaccion>, modifier: Modifier = Modifier) {
    if (rs.isEmpty()) return
    Row(modifier.testTag("reacciones"), horizontalArrangement = Arrangement.spacedBy(4.dp)) {
        rs.forEach { r ->
            Surface(shape = RoundedCornerShape(50), color = MaterialTheme.colorScheme.surfaceContainerHighest, border = if (r.mia) androidx.compose.foundation.BorderStroke(1.5.dp, MaterialTheme.colorScheme.primary) else null) {
                Text(if (r.cantidad > 1) "${r.emoji} ${r.cantidad}" else r.emoji, Modifier.padding(horizontal = 8.dp, vertical = 2.dp), style = MaterialTheme.typography.labelMedium)
            }
        }
    }
}

internal val EMOJIS_RAPIDOS = listOf("👍", "❤️", "😂", "😮", "😢", "🙏")

/** Barra de reacciones rápidas; "+" abre una caja para escribir cualquier emoji con el teclado. */
@Composable
fun BarraReacciones(alElegir: (String) -> Unit, modifier: Modifier = Modifier) {
    var otro by remember { mutableStateOf(false) }
    var texto by remember { mutableStateOf("") }
    Surface(modifier.testTag("barra_reacciones"), shape = RoundedCornerShape(if (otro) 24.dp else 50.dp), color = MaterialTheme.colorScheme.surfaceContainerHigh, shadowElevation = 3.dp) {
        Column(Modifier.padding(horizontal = 8.dp, vertical = 4.dp)) {
            Row(horizontalArrangement = Arrangement.spacedBy(2.dp), verticalAlignment = Alignment.CenterVertically) {
                EMOJIS_RAPIDOS.forEach { e -> Text(e, Modifier.clip(CircleShape).clickable { alElegir(e) }.padding(horizontal = 8.dp, vertical = 4.dp).testTag("emoji_$e"), style = MaterialTheme.typography.titleLarge) }
                IconButton({ otro = !otro }, Modifier.size(36.dp).testTag("emoji_mas")) { Icon(if (otro) Ico.Cerrar else Ico.Agregar, "Otro emoji") }
            }
            if (otro) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    OutlinedTextField(texto, { texto = it.take(16) }, Modifier.weight(1f).testTag("campo_emoji"), singleLine = true, placeholder = { Text("Escribe un emoji") })
                    TextButton({ if (texto.isNotBlank()) alElegir(texto.trim()) }, enabled = texto.isNotBlank(), modifier = Modifier.testTag("aceptar_emoji")) { Text("Reaccionar") }
                }
            }
        }
    }
}

/** Pregunta cómo borrar. */
@Composable
fun DialogoBorrar(cuantos: Int, hayParaTodos: Boolean, alParaMi: () -> Unit, alParaTodos: () -> Unit, alCerrar: () -> Unit) {
    AlertDialog(
        onDismissRequest = alCerrar,
        title = { Text(if (cuantos == 1) "¿Borrar este mensaje?" else "¿Borrar $cuantos mensajes?") },
        text = { Text("“Para mí” solo los quita de este teléfono." + if (hayParaTodos) " “Para todos” le pide a WhatsApp que los borre en el chat de todos; si no lo permite, verás el motivo." else "") },
        confirmButton = {
            Column(horizontalAlignment = Alignment.End) {
                TextButton(alParaMi, Modifier.testTag("borrar_para_mi")) { Text("Borrar para mí") }
                if (hayParaTodos) TextButton(alParaTodos, Modifier.testTag("borrar_para_todos")) { Text("Borrar para todos", color = MaterialTheme.colorScheme.error) }
            }
        },
        dismissButton = { TextButton(alCerrar) { Text("Cancelar") } },
    )
}

/** Elige hasta 5 chats a donde reenviar. */
@Composable
fun DialogoReenviar(botId: Long, jidActual: String, alReenviar: (List<String>) -> Unit, alCerrar: () -> Unit) {
    val chats by produceState<List<ChatItem>>(emptyList()) {
        value = runCatching { Repo.chats(botId, false, "") + Repo.chats(botId, true, "") }.getOrDefault(emptyList()).filter { it.jid != jidActual }.sortedByDescending { it.ultimoTs }
    }
    var elegidos by remember { mutableStateOf(setOf<String>()) }
    AlertDialog(
        onDismissRequest = alCerrar,
        title = { Text("Reenviar a…") },
        text = {
            Column {
                Text("Máximo 5 chats. Llevará la marca “Reenviado”.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                LazyColumn(Modifier.heightIn(max = 360.dp).testTag("lista_reenviar")) {
                    items(chats, key = { it.jid }) { c ->
                        val marcado = c.jid in elegidos
                        Row(
                            Modifier.fillMaxWidth().clickable { elegidos = if (marcado) elegidos - c.jid else if (elegidos.size < 5) elegidos + c.jid else elegidos }.padding(vertical = 4.dp).testTag("destino_${c.jid}"),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Avatar(c.nombre, c.foto, c.fotoTs, c.jid, 36.dp)
                            Spacer(Modifier.width(10.dp))
                            Text(c.nombre, Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis)
                            Checkbox(marcado, null)
                        }
                    }
                }
            }
        },
        confirmButton = { TextButton({ alReenviar(elegidos.toList()) }, enabled = elegidos.isNotEmpty(), modifier = Modifier.testTag("confirmar_reenviar")) { Text("Reenviar (${elegidos.size})") } },
        dismissButton = { TextButton(alCerrar) { Text("Cancelar") } },
    )
}

/** Info del mensaje: cuándo se envió, se entregó y se leyó; en grupos, quién. */
@Composable
fun DialogoInfoMensaje(info: InfoMsg?, alCerrar: () -> Unit) {
    AlertDialog(
        onDismissRequest = alCerrar,
        title = { Text("Info del mensaje") },
        text = {
            if (info == null) { Text("Cargando…"); return@AlertDialog }
            Column(verticalArrangement = Arrangement.spacedBy(6.dp), modifier = Modifier.testTag("info_mensaje")) {
                fun hora(ts: Long) = if (ts > 0) "${etiquetaDia(ts)} · ${horaDe(ts)}" else "—"
                Text("Enviado: ${hora(info.enviado)}")
                if (info.editado) Text("Editado", color = MaterialTheme.colorScheme.onSurfaceVariant)
                if (info.reenviado) Text("Reenviado", color = MaterialTheme.colorScheme.onSurfaceVariant)
                HorizontalDivider()
                if (!info.esGrupo) {
                    val p = info.personas.firstOrNull()
                    Text("Entregado: ${hora(p?.entregado ?: 0)}")
                    Text("Leído: ${hora(p?.leido ?: 0)}")
                    if ((p?.reproducido ?: 0) > 0) Text("Reproducido: ${hora(p!!.reproducido)}")
                } else if (info.personas.isEmpty()) {
                    Text("Todavía nadie lo recibió.")
                } else {
                    info.personas.forEach { p ->
                        Column {
                            Text(p.nombre, fontWeight = FontWeight.SemiBold)
                            Text("Entregado: ${hora(p.entregado)}   Leído: ${hora(p.leido)}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                }
            }
        },
        confirmButton = { TextButton(alCerrar) { Text("Cerrar") } },
    )
}

/** Línea del encabezado con lo que se sabe en vivo ("escribiendo…", "en línea" o "últ. vez …"); vacío si no llega nada. */
fun textoPresencia(p: com.botchat.app.datos.PresenciaChat, ahora: Long = System.currentTimeMillis() / 1000): String = when {
    p.escribiendo -> "escribiendo…"
    p.enLinea -> "en línea"
    p.ultimaVez > 0 -> "últ. vez ${etiquetaDia(p.ultimaVez).lowercase()} a las ${horaDe(p.ultimaVez)}"
    else -> ""
}

/**
 * Forma de burbuja de mensaje: rectángulo redondeado con una colita arriba del lado de quien habla (solo en el primero de cada tanda).
 * Deja [cola] de espacio a ese lado aunque no haya colita, para que todas las burbujas de una tanda queden alineadas.
 */
class FormaBurbuja(private val esMio: Boolean, private val conCola: Boolean, private val radio: androidx.compose.ui.unit.Dp = 12.dp, private val cola: androidx.compose.ui.unit.Dp = 8.dp) :
    androidx.compose.ui.graphics.Shape {
    override fun createOutline(size: androidx.compose.ui.geometry.Size, layoutDirection: androidx.compose.ui.unit.LayoutDirection, density: androidx.compose.ui.unit.Density): androidx.compose.ui.graphics.Outline {
        val r = with(density) { radio.toPx() }
        val c = with(density) { cola.toPx() }
        val izq = if (esMio) 0f else c
        val der = if (esMio) size.width - c else size.width
        val esquinaCola = if (conCola) androidx.compose.ui.geometry.CornerRadius(0f, 0f) else androidx.compose.ui.geometry.CornerRadius(r, r)
        val redondo = androidx.compose.ui.geometry.CornerRadius(r, r)
        val cuerpo = androidx.compose.ui.geometry.RoundRect(
            izq, 0f, der, size.height,
            topLeftCornerRadius = if (esMio) redondo else esquinaCola, topRightCornerRadius = if (esMio) esquinaCola else redondo,
            bottomRightCornerRadius = redondo, bottomLeftCornerRadius = redondo,
        )
        val camino = androidx.compose.ui.graphics.Path().apply { addRoundRect(cuerpo) }
        if (conCola) {
            val punta = androidx.compose.ui.graphics.Path().apply {
                if (esMio) { moveTo(der, 0f); lineTo(size.width, 0f); lineTo(der, c * 1.3f); close() }
                else { moveTo(izq, 0f); lineTo(0f, 0f); lineTo(izq, c * 1.3f); close() }
            }
            camino.addPath(punta)
        }
        return androidx.compose.ui.graphics.Outline.Generic(camino)
    }
}
