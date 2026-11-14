@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.animation.togetherWith
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.gestures.awaitFirstDown
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import com.botchat.app.ui.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledIconButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.TextField
import androidx.compose.foundation.layout.height
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.Msg
import com.botchat.app.datos.Participante
import com.botchat.app.medios.AdjuntoListo

private val regexMencion = Regex("""(?:^|\s)@([^\s@]*)$""")

/** Quita acentos y mayúsculas para buscar a alguien por su nombre. */
private fun plano(s: String) = com.botchat.app.Paises.normaliza(s)

/** De dónde viene un adjunto. */
enum class FuenteAdjunto { GALERIA, CAMARA_FOTO, CAMARA_VIDEO, DOCUMENTO, AUDIO }

/**
 * Caja de mensaje: texto con formato, cita, edición, @menciones en grupos, adjuntos con vista previa y notas de voz
 * (mantener pulsado el micrófono para grabar, deslizar a la izquierda para cancelar).
 */
@Composable
fun Compositor(
    valor: TextFieldValue,
    alCambiar: (TextFieldValue) -> Unit,
    citando: Msg?,
    alQuitarCita: () -> Unit,
    editando: Msg?,
    alCancelarEdicion: () -> Unit,
    esGrupo: Boolean,
    participantes: List<Participante>,
    enviando: Boolean,
    alEnviar: (texto: String, menciones: List<String>) -> Unit,
    modifier: Modifier = Modifier,
    adjuntos: List<AdjuntoListo> = emptyList(),
    preparandoAdjuntos: Boolean = false,
    enviandoAdjuntos: Boolean = false,
    alAdjuntar: (FuenteAdjunto) -> Unit = {},
    alQuitarAdjunto: (AdjuntoListo) -> Unit = {},
    alCancelarAdjuntos: () -> Unit = {},
    alEnviarAdjuntos: (String) -> Unit = {},
    grabando: Boolean = false,
    segundosGrabando: Int = 0,
    alIniciarGrabacion: () -> Boolean = { false },
    alTerminarGrabacion: (Boolean) -> Unit = {},
) {
    val haptica = rememberHaptica()
    val elegidas = remember { mutableStateMapOf<String, Participante>() } // "@Nombre" → a quién se mencionó
    var formato by remember { mutableStateOf(false) }
    var menuAdjuntar by remember { mutableStateOf(false) }

    val antes = valor.text.substring(0, valor.selection.start.coerceIn(0, valor.text.length))
    val consulta = if (esGrupo) regexMencion.find(antes)?.groupValues?.get(1) else null
    val candidatos = if (consulta == null) emptyList() else participantes.filter { consulta.isEmpty() || plano(it.nombre).contains(plano(consulta)) }.take(6)
    val hayAdjuntos = adjuntos.isNotEmpty()
    val ocupado = enviando || enviandoAdjuntos
    val puedeEnviar = (valor.text.isNotBlank() || hayAdjuntos) && !ocupado
    val mostrarMicrofono = valor.text.isBlank() && !hayAdjuntos && editando == null

    Surface(modifier.fillMaxWidth().imePadding().navigationBarsPadding(), color = androidx.compose.ui.graphics.Color.Transparent) {
        Column {
            if (candidatos.isNotEmpty()) {
                Column(Modifier.fillMaxWidth().background(MaterialTheme.colorScheme.surfaceContainerHigh).testTag("lista_menciones")) {
                    candidatos.forEach { p ->
                        Row(Modifier.fillMaxWidth().clickable {
                            haptica(Toque.Suave)
                            val inicio = antes.length - consulta!!.length - 1
                            val token = "@" + p.nombre
                            val nuevo = valor.text.substring(0, inicio) + token + " " + valor.text.substring(valor.selection.start.coerceIn(0, valor.text.length))
                            elegidas[token] = p
                            alCambiar(TextFieldValue(nuevo, TextRange(inicio + token.length + 1)))
                        }.padding(horizontal = Esp.l, vertical = Esp.s).testTag("mencion_${p.usuario}"), verticalAlignment = Alignment.CenterVertically) {
                            Text(p.nombre, Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis)
                            if (p.telefono.isNotBlank() && p.telefono != p.nombre) Text(p.telefono, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                }
            }
            if (editando != null) {
                Banner("Editando mensaje", editando.texto, "banner_edicion", alCancelarEdicion)
            } else if (citando != null) {
                Banner("Respondiendo a " + (if (citando.esMio) "ti" else citando.remitente.ifBlank { "este mensaje" }), descripcionCorta(citando.tipo, citando.texto, false, citando.notaVoz), "banner_cita", alQuitarCita)
            }
            if (hayAdjuntos || preparandoAdjuntos) PanelAdjuntos(adjuntos, preparandoAdjuntos, enviandoAdjuntos, alQuitarAdjunto, alCancelarAdjuntos)
            if (formato && !grabando) {
                Row(Modifier.fillMaxWidth().padding(horizontal = Esp.m), horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                    listOf("*" to "Negrita", "_" to "Cursiva", "~" to "Tachado", "```" to "Código").forEach { (m, nombre) ->
                        TextButton({ haptica(Toque.Suave); alCambiar(FormatoWhatsApp.aplicar(valor, m)) }, Modifier.testTag("formato_$nombre")) { Text(nombre, fontWeight = FontWeight.SemiBold) }
                    }
                }
            }
            if (grabando) {
                Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s).testTag("grabando"), verticalAlignment = Alignment.CenterVertically) {
                    Box(Modifier.size(10.dp).clip(CircleShape).background(MaterialTheme.colorScheme.error))
                    Spacer(Modifier.width(8.dp))
                    Text(duracion(segundosGrabando), style = MaterialTheme.typography.titleMedium)
                    Spacer(Modifier.width(12.dp))
                    Text("◀ Desliza para cancelar", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
            Row(Modifier.fillMaxWidth().padding(horizontal = 6.dp, vertical = 6.dp), verticalAlignment = Alignment.Bottom) {
                if (!grabando) {
                    Surface(Modifier.weight(1f), shape = RoundedCornerShape(26.dp), color = MaterialTheme.colorScheme.surfaceContainerHigh) {
                        Row(verticalAlignment = Alignment.Bottom) {
                            IconButton({ haptica(Toque.Suave); formato = !formato }, Modifier.testTag("boton_formato")) { Icon(Ico.Editar, "Formato del texto") }
                            TextField(
                                valor, alCambiar, Modifier.weight(1f).testTag("campo_mensaje"), placeholder = { Text(if (hayAdjuntos) "Añade un pie de foto" else "Mensaje") }, maxLines = 5,
                                colors = androidx.compose.material3.TextFieldDefaults.colors(
                                    focusedContainerColor = androidx.compose.ui.graphics.Color.Transparent, unfocusedContainerColor = androidx.compose.ui.graphics.Color.Transparent,
                                    focusedIndicatorColor = androidx.compose.ui.graphics.Color.Transparent, unfocusedIndicatorColor = androidx.compose.ui.graphics.Color.Transparent,
                                    disabledIndicatorColor = androidx.compose.ui.graphics.Color.Transparent,
                                ),
                            )
                            IconButton({ haptica(Toque.Suave); menuAdjuntar = true }, Modifier.testTag("boton_adjuntar")) { Icon(Ico.Adjuntar, "Adjuntar") }
                            if (mostrarMicrofono) IconButton({ haptica(Toque.Suave); alAdjuntar(FuenteAdjunto.CAMARA_FOTO) }, Modifier.testTag("boton_camara")) { Icon(Ico.Camara, "Cámara") }
                        }
                    }
                    Spacer(Modifier.width(6.dp))
                } else {
                    Spacer(Modifier.weight(1f))
                }
                androidx.compose.animation.AnimatedContent(
                    targetState = mostrarMicrofono, label = "micro_enviar",
                    transitionSpec = {
                        (androidx.compose.animation.scaleIn(tween(200, easing = Transiciones.DESACELERA_ENFATIZADA), 0.6f) + androidx.compose.animation.fadeIn(tween(120))) togetherWith
                            (androidx.compose.animation.scaleOut(tween(120), 0.6f) + androidx.compose.animation.fadeOut(tween(90)))
                    },
                ) { micro ->
                if (micro) {
                    val iniciar = rememberUpdatedState(alIniciarGrabacion)
                    val terminar = rememberUpdatedState(alTerminarGrabacion)
                    val umbral = with(LocalDensity.current) { 90.dp.toPx() }
                    Box(
                        Modifier.size(androidx.compose.animation.core.animateDpAsState(if (grabando) 64.dp else 48.dp, tween(Mov.CORTA), label = "tam_micro").value).clip(CircleShape).background(androidx.compose.animation.animateColorAsState(if (grabando) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.primary, tween(Mov.CORTA), label = "color_micro").value).testTag("microfono")
                            .pointerInput(Unit) {
                                awaitEachGesture {
                                    val down = awaitFirstDown()
                                    if (!iniciar.value()) return@awaitEachGesture
                                    var cancelado = false
                                    do {
                                        val ev = awaitPointerEvent()
                                        val c = ev.changes.firstOrNull() ?: break
                                        if (c.position.x - down.position.x < -umbral) cancelado = true
                                    } while (ev.changes.any { it.pressed })
                                    terminar.value(cancelado)
                                }
                            },
                        Alignment.Center,
                    ) { Icon(Ico.Microfono, "Mantén pulsado para grabar una nota de voz", Modifier.size(24.dp), tint = MaterialTheme.colorScheme.onPrimary) }
                } else {
                    FilledIconButton(
                        {
                            haptica(Toque.Suave)
                            var texto = valor.text.trim()
                            val menciones = mutableListOf<String>()
                            elegidas.entries.sortedByDescending { it.key.length }.forEach { (token, p) ->
                                if (texto.contains(token)) { texto = texto.replace(token, "@" + p.usuario); if (p.jid !in menciones) menciones += p.jid }
                            }
                            if (hayAdjuntos) alEnviarAdjuntos(texto) else alEnviar(texto, menciones)
                            elegidas.clear()
                        },
                        Modifier.size(48.dp).testTag("enviar"), enabled = puedeEnviar,
                    ) { Icon(if (editando != null) Ico.Listo else Ico.Enviar, if (editando != null) "Guardar cambio" else "Enviar") }
                }
                }
            }
            if (menuAdjuntar) HojaAdjuntar(alElegir = { f -> menuAdjuntar = false; alAdjuntar(f) }, alCerrar = { menuAdjuntar = false })
        }
    }
}

/** Fotos, videos y archivos listos para enviar, con su vista previa y una barra mientras se preparan o se mandan. */
@Composable
private fun PanelAdjuntos(adjuntos: List<AdjuntoListo>, preparando: Boolean, enviando: Boolean, alQuitar: (AdjuntoListo) -> Unit, alCancelar: () -> Unit) {
    Column(Modifier.fillMaxWidth().background(MaterialTheme.colorScheme.surfaceContainerHigh).padding(vertical = 6.dp).testTag("panel_adjuntos")) {
        if (preparando || enviando) LinearProgressIndicator(Modifier.fillMaxWidth().padding(horizontal = Esp.l).testTag("progreso_adjuntos"))
        Row(verticalAlignment = Alignment.CenterVertically) {
            LazyRow(Modifier.weight(1f), contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = Esp.m), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                items(adjuntos) { a ->
                    Box(Modifier.size(84.dp).clip(RoundedCornerShape(12.dp)).background(MaterialTheme.colorScheme.surfaceContainerHighest).testTag("adjunto_${a.nombre}")) {
                        val p = a.previa
                        if (p != null) Image(p.asImageBitmap(), a.nombre, Modifier.size(84.dp), contentScale = ContentScale.Crop)
                        else Column(Modifier.size(84.dp).padding(6.dp), verticalArrangement = Arrangement.Center, horizontalAlignment = Alignment.CenterHorizontally) {
                            Icon(iconoDeTipo(if (a.tipo == "imagen") "imagen" else a.tipo) ?: Ico.Documento, null, Modifier.size(28.dp))
                            Text(a.nombre, style = MaterialTheme.typography.labelSmall, maxLines = 2, overflow = TextOverflow.Ellipsis)
                        }
                        if (a.tipo == "video") Icon(Ico.Reproducir, null, Modifier.align(Alignment.Center).size(26.dp), tint = androidx.compose.ui.graphics.Color.White)
                        if (a.durS > 0 && a.tipo != "imagen") Text(duracion(a.durS), Modifier.align(Alignment.BottomStart).padding(4.dp).background(androidx.compose.ui.graphics.Color.Black.copy(alpha = 0.5f), RoundedCornerShape(4.dp)).padding(horizontal = 4.dp), style = MaterialTheme.typography.labelSmall, color = androidx.compose.ui.graphics.Color.White)
                        if (!enviando) IconButton({ alQuitar(a) }, Modifier.align(Alignment.TopEnd).size(28.dp).testTag("quitar_${a.nombre}")) { Icon(Ico.Cerrar, "Quitar", Modifier.size(16.dp), tint = androidx.compose.ui.graphics.Color.White) }
                    }
                }
            }
            if (!enviando) TextButton(alCancelar, Modifier.testTag("cancelar_adjuntos")) { Text("Cancelar") }
        }
        val total = adjuntos.sumOf { it.bytes }
        if (adjuntos.isNotEmpty()) Text(
            (if (adjuntos.size == 1) "1 archivo" else "${adjuntos.size} archivos") + " · " + formatoBytes(total) + if (adjuntos.size > 1) " · el pie va en el primero" else "",
            Modifier.padding(horizontal = Esp.l, vertical = 2.dp), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun Banner(titulo: String, texto: String, etiqueta: String, alCerrar: () -> Unit) {
    Row(Modifier.fillMaxWidth().background(MaterialTheme.colorScheme.surfaceContainerHigh).padding(start = Esp.l, top = 4.dp, bottom = 4.dp).testTag(etiqueta), verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.width(4.dp).size(width = 4.dp, height = 34.dp).background(MaterialTheme.colorScheme.primary))
        Column(Modifier.weight(1f).padding(start = 8.dp)) {
            Text(titulo, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.primary)
            Text(texto, style = MaterialTheme.typography.bodySmall, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
        IconButton(alCerrar, Modifier.testTag("cerrar_$etiqueta")) { Icon(Ico.Cerrar, "Cancelar") }
    }
}


private data class OpcionAdjuntar(val fuente: FuenteAdjunto, val nombre: String, val icono: androidx.compose.ui.graphics.vector.ImageVector, val color: androidx.compose.ui.graphics.Color)

/** Hoja de abajo para elegir qué adjuntar: círculos de color con su nombre debajo. */
@OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)
@Composable
private fun HojaAdjuntar(alElegir: (FuenteAdjunto) -> Unit, alCerrar: () -> Unit) {
    val opciones = listOf(
        OpcionAdjuntar(FuenteAdjunto.GALERIA, "Galería", Ico.Foto, androidx.compose.ui.graphics.Color(0xFF3B6FD4)),
        OpcionAdjuntar(FuenteAdjunto.CAMARA_FOTO, "Cámara", Ico.Camara, androidx.compose.ui.graphics.Color(0xFFD6336C)),
        OpcionAdjuntar(FuenteAdjunto.CAMARA_VIDEO, "Video", Ico.Video, androidx.compose.ui.graphics.Color(0xFFE8590C)),
        OpcionAdjuntar(FuenteAdjunto.DOCUMENTO, "Documento", Ico.Documento, androidx.compose.ui.graphics.Color(0xFF7048E8)),
        OpcionAdjuntar(FuenteAdjunto.AUDIO, "Audio", Ico.Audio, androidx.compose.ui.graphics.Color(0xFFF08C00)),
    )
    androidx.compose.material3.ModalBottomSheet(onDismissRequest = alCerrar, containerColor = MaterialTheme.colorScheme.surfaceContainer, modifier = Modifier.testTag("hoja_adjuntar")) {
        Column(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s).navigationBarsPadding(), verticalArrangement = Arrangement.spacedBy(Esp.l)) {
            opciones.chunked(3).forEach { fila ->
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceEvenly) {
                    fila.forEach { o ->
                        Column(
                            Modifier.weight(1f).clip(RoundedCornerShape(12.dp)).clickable { alElegir(o.fuente) }.padding(vertical = 6.dp).testTag("adjuntar_${o.fuente.name}"),
                            horizontalAlignment = Alignment.CenterHorizontally,
                        ) {
                            Box(Modifier.size(56.dp).clip(CircleShape).background(o.color), Alignment.Center) { Icon(o.icono, null, Modifier.size(26.dp), tint = androidx.compose.ui.graphics.Color.White) }
                            Spacer(Modifier.height(6.dp))
                            Text(o.nombre, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                    repeat(3 - fila.size) { Spacer(Modifier.weight(1f)) }
                }
            }
            Spacer(Modifier.height(Esp.m))
        }
    }
}
