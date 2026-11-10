@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import android.widget.Toast
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Slider
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import androidx.media3.common.util.UnstableApi
import com.botchat.app.Paises
import com.botchat.app.datos.Msg
import com.botchat.app.datos.Repo
import com.botchat.app.medios.CacheImagenes
import com.botchat.app.medios.ReproductorAudio

private sealed interface Elem {
    data class Dia(val ts: Long) : Elem
    data class Burbuja(val m: Msg, val mostrarRemitente: Boolean, val ultimoDelGrupo: Boolean) : Elem
}

/** Arma la lista visual (del más nuevo al más viejo, para una lista invertida) con separadores de día y agrupación por remitente. */
private fun elementos(msgs: List<Msg>, esGrupo: Boolean): List<Elem> {
    val out = ArrayList<Elem>(msgs.size + 8)
    for (i in msgs.indices) {
        val m = msgs[i]
        val mas = msgs.getOrNull(i + 1)      // el mensaje anterior en el tiempo
        val menos = msgs.getOrNull(i - 1)    // el siguiente en el tiempo
        val mismoAnterior = mas != null && mismoDia(mas.ts, m.ts) && mas.remitenteId == m.remitenteId && m.ts - mas.ts < 300
        val mismoSiguiente = menos != null && mismoDia(menos.ts, m.ts) && menos.remitenteId == m.remitenteId && menos.ts - m.ts < 300
        out += Elem.Burbuja(m, mostrarRemitente = esGrupo && !m.esMio && !mismoAnterior, ultimoDelGrupo = !mismoSiguiente)
        if (mas == null || !mismoDia(mas.ts, m.ts)) out += Elem.Dia(m.ts)
    }
    return out
}

@OptIn(UnstableApi::class)
@Composable
fun PantallaChat(
    jid: String,
    alVolver: () -> Unit,
    alAbrirVisor: (String) -> Unit,
    alAbrirDocumento: (String) -> Unit,
    alAbrirArchivos: () -> Unit,
) {
    val app = LocalContext.current.applicationContext as android.app.Application
    val vm: ChatVm = viewModel(key = "chat-$jid", factory = viewModelFactory { initializer { ChatVm(app, jid) } })
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    val chat = vm.chat
    val msgs = vm.mensajes
    var menu by remember { mutableStateOf(false) }
    var confirmaArchivos by remember { mutableStateOf<com.botchat.app.datos.Liberado?>(null) }
    var confirmaChat by remember { mutableStateOf(false) }
    var verFoto by remember { mutableStateOf(false) }
    val estadoLista = rememberLazyListState()

    // Al salir del chat: se vacía la caché de imágenes y se detiene el audio.
    DisposableEffect(jid) {
        onDispose { CacheImagenes.vaciar(); ReproductorAudio.detener() }
    }
    LaunchedEffect(vm.aviso) { vm.aviso?.let { Toast.makeText(ctx, it, Toast.LENGTH_LONG).show(); vm.aviso = null } }

    val items = remember(msgs, chat?.esGrupo) { if (msgs == null) emptyList() else elementos(msgs, chat?.esGrupo == true) }
    val llegandoAlFinal by remember { derivedStateOf { val u = estadoLista.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0; u >= items.size - 6 } }
    LaunchedEffect(llegandoAlFinal, items.size) { if (llegandoAlFinal) vm.cargarMas() }

    Scaffold(
        topBar = {
            TopAppBar(
                navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } },
                title = {
                    Row(Modifier.clip(RoundedCornerShape(12.dp)).clickable { haptica(Toque.Suave); verFoto = true }.padding(vertical = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                        Avatar(chat?.nombre ?: "", chat?.foto ?: "", chat?.fotoTs ?: 0, jid, 40.dp, Modifier.compartido("avatar-$jid"))
                        Spacer(Modifier.width(Esp.m))
                        Column {
                            Text(chat?.nombre ?: "", Modifier.compartido("nombre-$jid", limites = true), style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            if (chat != null && !chat.esGrupo) {
                                TextoUnaLinea(banderaTelefono(chat), estilo = MaterialTheme.typography.bodySmall)
                            } else if (chat != null) {
                                TextoUnaLinea("Grupo", estilo = MaterialTheme.typography.bodySmall)
                            }
                        }
                    }
                },
                actions = {
                    IconButton(alAbrirArchivos) { Icon(Ico.Carpeta, "Archivos compartidos") }
                    Box {
                        IconButton({ menu = true }) { Icon(Ico.Mas, "Más opciones") }
                        DropdownMenu(menu, { menu = false }) {
                            DropdownMenuItem(text = { Text("Borrar archivos de este chat") }, onClick = {
                                menu = false
                                haptica(Toque.Fuerte)
                                vm.estimarArchivos { confirmaArchivos = it }
                            })
                            DropdownMenuItem(text = { Text("Borrar todo el chat") }, onClick = { menu = false; haptica(Toque.Fuerte); confirmaChat = true })
                        }
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
            )
        },
        containerColor = MaterialTheme.colorScheme.surface,
    ) { relleno ->
        Box(Modifier.fillMaxSize().padding(relleno)) {
            when {
                msgs == null -> CargaDeMensajes()
                msgs.isEmpty() -> EstadoVacio(Ico.Documento, "Sin mensajes", "Todavía no hay mensajes en este chat.", Modifier.align(Alignment.Center))
                else -> {
                    LazyColumn(
                        Modifier.fillMaxSize(), state = estadoLista, reverseLayout = true,
                        contentPadding = PaddingValues(horizontal = Esp.m, vertical = Esp.s), verticalArrangement = Arrangement.spacedBy(2.dp),
                    ) {
                        items(items, key = { e -> when (e) { is Elem.Dia -> "dia-${e.ts}"; is Elem.Burbuja -> "m-${e.m.id}" } }) { e ->
                            when (e) {
                                is Elem.Dia -> SeparadorDia(e.ts, Modifier.animateItem())
                                is Elem.Burbuja -> Burbuja(e, vm, alAbrirVisor, alAbrirDocumento, Modifier.animateItem())
                            }
                        }
                        if (vm.cargandoMas) item { Box(Modifier.fillMaxWidth().padding(Esp.l), contentAlignment = Alignment.Center) { CircularProgressIndicator(Modifier.size(24.dp), strokeWidth = 2.dp) } }
                    }
                    // Separador de día fijo arriba mientras se desplaza.
                    val diaArriba by remember { derivedStateOf { estadoLista.layoutInfo.visibleItemsInfo.lastOrNull()?.index?.let { i -> items.getOrNull(i)?.let { e -> if (e is Elem.Dia) e.ts else (e as? Elem.Burbuja)?.m?.ts } } } }
                    AnimatedVisibility(
                        visible = estadoLista.isScrollInProgress && diaArriba != null, modifier = Modifier.align(Alignment.TopCenter).padding(top = Esp.s),
                        enter = fadeIn() + slideInVertically { -it / 2 }, exit = fadeOut(),
                    ) {
                        Surface(shape = RoundedCornerShape(50), color = MaterialTheme.colorScheme.secondaryContainer.copy(alpha = 0.95f), shadowElevation = 2.dp) {
                            Text(etiquetaDia(diaArriba ?: 0), Modifier.padding(horizontal = 14.dp, vertical = 6.dp), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSecondaryContainer)
                        }
                    }
                }
            }
        }
    }

    confirmaArchivos?.let { lib ->
        AlertDialog(
            onDismissRequest = { confirmaArchivos = null },
            title = { Text("¿Borrar los archivos de este chat?") },
            text = { Text(if (lib.cantidad == 0) "Este chat no tiene archivos descargados." else "Se liberarán ${formatoBytes(lib.bytes)} (${lib.cantidad} archivos). Los mensajes de texto se conservan.") },
            confirmButton = {
                TextButton(enabled = lib.cantidad > 0, onClick = {
                    confirmaArchivos = null
                    vm.borrarArchivos { haptica(Toque.Confirmar); Toast.makeText(ctx, "Se liberaron ${formatoBytes(it.bytes)}", Toast.LENGTH_SHORT).show() }
                }) { Text("Borrar archivos", color = MaterialTheme.colorScheme.error) }
            },
            dismissButton = { TextButton({ confirmaArchivos = null }) { Text("Cancelar") } },
        )
    }
    if (confirmaChat) {
        AlertDialog(
            onDismissRequest = { confirmaChat = false },
            title = { Text("¿Borrar todo el chat?") },
            text = { Text("Se borrarán los mensajes, los archivos y la foto de este chat de tu teléfono. No se borra nada en WhatsApp.") },
            confirmButton = {
                TextButton({ confirmaChat = false; haptica(Toque.Confirmar); vm.borrarChat { alVolver() } }) { Text("Borrar todo", color = MaterialTheme.colorScheme.error) }
            },
            dismissButton = { TextButton({ confirmaChat = false }) { Text("Cancelar") } },
        )
    }
    if (verFoto) FotoGrandeDialogo(vm.botId, jid, chat?.nombre ?: "", { verFoto = false })
}

@Composable
private fun CargaDeMensajes() {
    Column(Modifier.fillMaxSize().padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
        listOf(0.55f to false, 0.4f to true, 0.7f to false, 0.3f to true, 0.5f to false).forEach { (ancho, mio) ->
            Row(Modifier.fillMaxWidth(), horizontalArrangement = if (mio) Arrangement.End else Arrangement.Start) {
                Box(Modifier.fillMaxWidth(ancho).height(46.dp).clip(RoundedCornerShape(18.dp)).shimmer())
            }
        }
    }
}

@Composable
private fun SeparadorDia(ts: Long, modifier: Modifier) {
    Box(modifier.fillMaxWidth().padding(vertical = Esp.s), contentAlignment = Alignment.Center) {
        Surface(shape = RoundedCornerShape(50), color = MaterialTheme.colorScheme.surfaceContainerHigh) {
            Text(etiquetaDia(ts), Modifier.padding(horizontal = 12.dp, vertical = 4.dp), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

// ---------- burbujas ----------

@OptIn(UnstableApi::class)
@Composable
private fun Burbuja(e: Elem.Burbuja, vm: ChatVm, alAbrirVisor: (String) -> Unit, alAbrirDocumento: (String) -> Unit, modifier: Modifier) {
    val m = e.m
    val esquema = MaterialTheme.colorScheme
    val fondo = if (m.esMio) esquema.primaryContainer else esquema.surfaceContainerHigh
    val tinta = if (m.esMio) esquema.onPrimaryContainer else esquema.onSurface
    val g = 18.dp
    val p = 5.dp
    val forma = if (m.esMio) RoundedCornerShape(g, g, if (e.ultimoDelGrupo) p else g, g) else RoundedCornerShape(g, g, g, if (e.ultimoDelGrupo) p else g)
    Row(modifier.fillMaxWidth(), horizontalArrangement = if (m.esMio) Arrangement.End else Arrangement.Start) {
        Surface(color = fondo, contentColor = tinta, shape = forma, modifier = Modifier.widthIn(max = 300.dp)) {
            Column(Modifier.padding(horizontal = 10.dp, vertical = 7.dp)) {
                if (e.mostrarRemitente && m.remitente.isNotEmpty()) {
                    Text(m.remitente, style = MaterialTheme.typography.labelLarge, fontWeight = FontWeight.SemiBold, color = colorEstable(m.remitenteId, androidx.compose.foundation.isSystemInDarkTheme()).let { c -> androidx.compose.ui.graphics.lerp(c, tinta, 0.35f) }, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
                ContenidoMensaje(m, vm, alAbrirVisor, alAbrirDocumento)
                Row(Modifier.align(Alignment.End).padding(top = 2.dp), verticalAlignment = Alignment.CenterVertically) {
                    if (m.editado && !m.borrado) Text("(editado)  ", style = MaterialTheme.typography.labelSmall, color = tinta.copy(alpha = 0.65f))
                    Text(horaDe(m.ts), style = MaterialTheme.typography.labelSmall, color = tinta.copy(alpha = 0.65f))
                }
            }
        }
    }
}

private fun etiquetaBorrado(tipo: String) = when (tipo) {
    "imagen" -> "[foto borrada]"
    "sticker" -> "[sticker borrado]"
    "video" -> "[video borrado]"
    "audio" -> "[audio borrado]"
    "documento" -> "[documento borrado]"
    else -> "[archivo borrado]"
}

@OptIn(UnstableApi::class)
@Composable
private fun ContenidoMensaje(m: Msg, vm: ChatVm, alAbrirVisor: (String) -> Unit, alAbrirDocumento: (String) -> Unit) {
    val cursiva = FontStyle.Italic
    when {
        m.borrado -> Text("Mensaje borrado", fontStyle = cursiva, color = LocalContentColorSuave())
        m.tipo == "ver_una_vez" -> Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(Ico.Candado, null, Modifier.size(20.dp)); Spacer(Modifier.width(8.dp))
            Column { Text("Ver una vez", fontWeight = FontWeight.SemiBold); Text("Ábrelo en tu teléfono", style = MaterialTheme.typography.bodySmall, color = LocalContentColorSuave()) }
        }
        m.categoria.isNotEmpty() && m.estado == "borrado" -> {
            Text(etiquetaBorrado(m.tipo), fontStyle = cursiva, color = LocalContentColorSuave())
            if (m.texto.isNotBlank()) Text(m.texto)
        }
        m.categoria == "foto" || m.categoria == "video" -> {
            MiniaturaMedia(m, vm, alAbrirVisor)
            if (m.texto.isNotBlank()) { Spacer(Modifier.size(4.dp)); Text(m.texto) }
        }
        m.categoria == "audio" -> { BurbujaAudio(m, vm); if (m.texto.isNotBlank()) Text(m.texto) }
        m.categoria == "documento" -> { BurbujaDocumento(m, vm, alAbrirDocumento); if (m.texto.isNotBlank()) { Spacer(Modifier.size(4.dp)); Text(m.texto) } }
        m.tipo == "contacto" || m.tipo == "ubicacion" || m.tipo == "encuesta" -> Text(m.texto)
        m.tipo == "otro" -> Text(m.texto, fontStyle = cursiva, color = LocalContentColorSuave())
        else -> Text(m.texto)
    }
}

@Composable
private fun LocalContentColorSuave() = androidx.compose.material3.LocalContentColor.current.copy(alpha = 0.7f)

@Composable
private fun MiniaturaMedia(m: Msg, vm: ChatVm, alAbrirVisor: (String) -> Unit) {
    val ctx = LocalContext.current
    val densidad = androidx.compose.ui.platform.LocalDensity.current
    val relacion = if (m.ancho > 0 && m.alto > 0) (m.ancho.toFloat() / m.alto).coerceIn(0.6f, 1.8f) else 1.33f
    val ruta = if (m.descargado) m.ruta else m.miniatura
    var imagen by remember(ruta) { mutableStateOf<androidx.compose.ui.graphics.ImageBitmap?>(null) }
    val px = with(densidad) { 300.dp.roundToPx() }
    LaunchedEffect(ruta) {
        imagen = if (ruta.isEmpty()) null else CacheImagenes.cargar(ctx, Repo.archivo(ruta), px)?.asImageBitmap()
    }
    val haptica = rememberHaptica()
    Box(
        Modifier.widthIn(max = 280.dp).aspectRatio(relacion).clip(RoundedCornerShape(12.dp)).background(MaterialTheme.colorScheme.surfaceContainerHighest)
            .clickable(enabled = m.descargado) { alAbrirVisor(m.id) },
        contentAlignment = Alignment.Center,
    ) {
        val img = imagen
        if (img != null) Image(img, null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop) else if (m.estado == "pendiente" || ruta.isNotEmpty()) Box(Modifier.fillMaxSize().shimmer())
        if (m.categoria == "video" && m.descargado) {
            Box(Modifier.size(48.dp).clip(CircleShape).background(Color.Black.copy(alpha = 0.45f)), contentAlignment = Alignment.Center) { Icon(Ico.Reproducir, null, tint = Color.White, modifier = Modifier.size(30.dp)) }
        }
        if (m.categoria == "video" && m.dur > 0) {
            Text(duracion(m.dur), Modifier.align(Alignment.BottomStart).padding(6.dp).clip(RoundedCornerShape(6.dp)).background(Color.Black.copy(alpha = 0.5f)).padding(horizontal = 6.dp, vertical = 2.dp), color = Color.White, style = MaterialTheme.typography.labelSmall)
        }
        if (!m.descargado) EstadoDeDescarga(m, onDescargar = { haptica(Toque.Fuerte); vm.descargar(m) })
    }
}


/** Superpuesto del botón "Descargar", el círculo de progreso o el aviso de error. */
@Composable
private fun EstadoDeDescarga(m: Msg, onDescargar: () -> Unit) {
    Box(Modifier.fillMaxSize().background(Color.Black.copy(alpha = 0.28f)), contentAlignment = Alignment.Center) {
        when {
            m.estado == "pendiente" -> Column(horizontalAlignment = Alignment.CenterHorizontally) {
                CircularProgressIndicator(Modifier.size(34.dp), color = Color.White, strokeWidth = 3.dp)
                Spacer(Modifier.size(6.dp)); Text("Descargando…", color = Color.White, style = MaterialTheme.typography.labelMedium)
            }
            m.puedeBajarse -> FilledTonalButton(onDescargar, contentPadding = PaddingValues(horizontal = 14.dp, vertical = 6.dp)) {
                Icon(Ico.Descargar, null, Modifier.size(18.dp)); Spacer(Modifier.width(6.dp))
                Text(if (m.estado == "fallido") "Reintentar" else if (m.bytes > 0) "Descargar · ${formatoBytes(m.bytes)}" else "Descargar")
            }
            else -> Text("No disponible", color = Color.White, style = MaterialTheme.typography.labelMedium)
        }
    }
}

@OptIn(UnstableApi::class)
@Composable
private fun BurbujaAudio(m: Msg, vm: ChatVm) {
    val ctx = LocalContext.current
    val rep by ReproductorAudio.estado.collectAsState()
    val activo = rep.clave == "${m.chat}|${m.id}"
    val haptica = rememberHaptica()
    Row(Modifier.widthIn(min = 220.dp), verticalAlignment = Alignment.CenterVertically) {
        if (!m.descargado) {
            if (m.estado == "pendiente") CircularProgressIndicator(Modifier.size(32.dp), strokeWidth = 3.dp)
            else if (m.puedeBajarse) FilledTonalButton({ haptica(Toque.Fuerte); vm.descargar(m) }, contentPadding = PaddingValues(horizontal = 12.dp)) { Icon(Ico.Descargar, null, Modifier.size(18.dp)); Spacer(Modifier.width(6.dp)); Text(if (m.estado == "fallido") "Reintentar" else "Descargar") }
            Spacer(Modifier.width(8.dp))
            Icon(Ico.Audio, null, Modifier.size(18.dp)); Spacer(Modifier.width(4.dp))
            Text(if (m.dur > 0) duracion(m.dur) else formatoBytes(m.bytes), style = MaterialTheme.typography.labelMedium)
        } else {
            IconButton({ haptica(Toque.Suave); ReproductorAudio.alternar(ctx, "${m.chat}|${m.id}", Repo.archivo(m.ruta)) }, Modifier.size(40.dp)) {
                Icon(if (activo && rep.reproduciendo) Ico.Pausa else Ico.Reproducir, if (activo && rep.reproduciendo) "Pausar" else "Reproducir")
            }
            val total = if (activo && rep.durMs > 0) rep.durMs else m.dur * 1000L
            val fraccion = if (activo && total > 0) (rep.posMs.toFloat() / total).coerceIn(0f, 1f) else 0f
            Slider(value = fraccion, onValueChange = { if (activo) ReproductorAudio.moverA(it) }, enabled = activo, modifier = Modifier.weight(1f))
            Text(duracion(((if (activo) rep.posMs else 0L) / 1000).toInt().takeIf { activo } ?: m.dur), style = MaterialTheme.typography.labelMedium)
        }
    }
}

@Composable
private fun BurbujaDocumento(m: Msg, vm: ChatVm, alAbrirDocumento: (String) -> Unit) {
    val haptica = rememberHaptica()
    Row(
        Modifier.widthIn(min = 220.dp).clip(RoundedCornerShape(12.dp)).background(MaterialTheme.colorScheme.surface.copy(alpha = 0.55f)).clickable(enabled = m.descargado) { alAbrirDocumento(m.id) }.padding(10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(Ico.Documento, null, Modifier.size(34.dp))
        Spacer(Modifier.width(10.dp))
        Column(Modifier.weight(1f)) {
            Text(m.nombre.ifBlank { "Documento" }, maxLines = 2, overflow = TextOverflow.Ellipsis, fontWeight = FontWeight.Medium)
            Text(listOfNotNull(if (m.bytes > 0) formatoBytes(m.bytes) else null, m.mime.substringAfter('/').substringBefore(';').uppercase().takeIf { it.isNotBlank() }).joinToString(" · "), style = MaterialTheme.typography.bodySmall, color = LocalContentColorSuave())
        }
        if (!m.descargado) {
            Spacer(Modifier.width(8.dp))
            if (m.estado == "pendiente") CircularProgressIndicator(Modifier.size(28.dp), strokeWidth = 3.dp)
            else if (m.puedeBajarse) IconButton({ haptica(Toque.Fuerte); vm.descargar(m) }) { Icon(Ico.Descargar, if (m.estado == "fallido") "Reintentar" else "Descargar") }
        }
    }
}

// ---------- foto grande ----------

@Composable
fun FotoGrandeDialogo(botId: Long, chat: String, nombre: String, alCerrar: () -> Unit) {
    val ctx = LocalContext.current
    var imagen by remember { mutableStateOf<androidx.compose.ui.graphics.ImageBitmap?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(chat) {
        try {
            val rel = Repo.fotoGrande(botId, chat)
            val bmp = CacheImagenes.cargar(ctx, Repo.archivo(rel), 1600, "grande")
            if (bmp != null) imagen = bmp.asImageBitmap() else error = "No se pudo abrir la foto"
        } catch (e: Exception) {
            error = e.message ?: "No se pudo traer la foto"
        }
    }
    androidx.compose.ui.window.Dialog(alCerrar, androidx.compose.ui.window.DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(Modifier.fillMaxSize().clickable(onClick = alCerrar), color = Color.Black.copy(alpha = 0.92f)) {
            Box(contentAlignment = Alignment.Center) {
                val img = imagen
                when {
                    img != null -> Image(img, "Foto de perfil de $nombre", Modifier.fillMaxWidth(), contentScale = ContentScale.Fit)
                    error != null -> EstadoVacio(Ico.Foto, "Sin foto", error ?: "")
                    else -> CircularProgressIndicator(color = Color.White)
                }
                IconButton(alCerrar, Modifier.align(Alignment.TopStart).padding(Esp.s)) { Icon(Ico.Cerrar, "Cerrar", tint = Color.White) }
            }
        }
    }
}
