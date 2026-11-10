package com.botchat.app.ui

import android.content.Context
import android.net.Uri
import android.view.SurfaceView
import android.widget.Toast
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.Orientation
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.gestures.awaitFirstDown
import androidx.compose.foundation.gestures.calculatePan
import androidx.compose.foundation.gestures.calculateZoom
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.gestures.draggable
import androidx.compose.foundation.gestures.rememberDraggableState
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Slider
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.input.pointer.positionChanged
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.media3.common.MediaItem
import androidx.media3.common.Player
import androidx.media3.common.VideoSize
import androidx.media3.common.util.UnstableApi
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.ProgressiveMediaSource
import com.botchat.app.datos.Msg
import com.botchat.app.datos.Repo
import com.botchat.app.medios.CacheImagenes
import com.botchat.app.medios.Exportar
import com.botchat.app.medios.FuenteCifrada
import com.botchat.app.medios.Temporales
import com.botchat.app.seguridad.ArchivoCifrado
import com.botchat.app.seguridad.ClaveDatos
import com.botchat.app.seguridad.FuenteArchivo
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlin.math.abs

/** Visor de fotos y videos del chat. Las fotos se descifran solo en memoria; los videos se descifran en flujo. */
@OptIn(UnstableApi::class)
@Composable
fun PantallaVisor(botId: Long, jid: String, idInicial: String, nombreChat: String, alCerrar: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    var lista by remember { mutableStateOf<List<Msg>?>(null) }
    LaunchedEffect(jid) {
        val fotos = Repo.archivos(botId, jid, "foto", true, 1000)
        val videos = Repo.archivos(botId, jid, "video", true, 1000)
        lista = (fotos + videos).sortedWith(compareBy<Msg> { it.ts }.thenBy { it.id })
    }
    val items = lista
    if (items == null) { Box(Modifier.fillMaxSize().background(Color.Black), Alignment.Center) { CircularProgressIndicator(color = Color.White) }; return }
    if (items.isEmpty()) { LaunchedEffect(Unit) { alCerrar() }; return }

    val inicial = items.indexOfFirst { it.id == idInicial }.coerceAtLeast(0)
    val pager = rememberPagerState(initialPage = inicial) { items.size }
    var controles by remember { mutableStateOf(true) }
    var zoom by remember { mutableStateOf(false) }
    val arrastre = remember { Animatable(0f) }
    val actual = items.getOrNull(pager.currentPage)

    DisposableEffect(Unit) { onDispose { CacheImagenes.vaciar() } }

    Box(Modifier.fillMaxSize().background(Color.Black.copy(alpha = (1f - abs(arrastre.value) / 900f).coerceIn(0.2f, 1f)))) {
        Box(
            Modifier.fillMaxSize().graphicsLayer { translationY = arrastre.value }.draggable(
                orientation = Orientation.Vertical, enabled = !zoom,
                state = rememberDraggableState { d -> scope.launch { arrastre.snapTo(arrastre.value + d) } },
                onDragStopped = {
                    if (abs(arrastre.value) > 260f) alCerrar() else scope.launch { arrastre.animateTo(0f) }
                },
            ),
        ) {
            HorizontalPager(pager, Modifier.fillMaxSize(), beyondViewportPageCount = 1, key = { items[it].id }, userScrollEnabled = !zoom) { i ->
                val m = items[i]
                if (m.categoria == "video") {
                    PaginaVideo(m, activa = pager.currentPage == i, alTocar = { controles = !controles })
                } else {
                    PaginaFoto(m, alCambiarZoom = { z -> if (pager.currentPage == i) zoom = z }, alTocar = { controles = !controles })
                }
            }
        }
        AnimatedVisibility(controles, Modifier.align(Alignment.TopCenter), enter = fadeIn(), exit = fadeOut()) {
            Row(Modifier.fillMaxWidth().background(Color.Black.copy(alpha = 0.45f)).statusBarsPadding().padding(horizontal = Esp.s, vertical = Esp.xs), verticalAlignment = Alignment.CenterVertically) {
                IconButton(alCerrar) { Icon(Ico.Atras, "Volver", tint = Color.White) }
                Column(Modifier.weight(1f)) {
                    Text(nombreChat, color = Color.White, style = MaterialTheme.typography.titleMedium, maxLines = 1)
                    actual?.let { Text("${etiquetaDia(it.ts)} · ${horaDe(it.ts)}", color = Color.White.copy(alpha = 0.8f), style = MaterialTheme.typography.bodySmall) }
                }
                if (actual != null) {
                    if (Exportar.disponible) IconButton({
                        haptica(Toque.Fuerte)
                        scope.launch { guardarCopia(ctx, actual, nombreChat) }
                    }) { Icon(Ico.Guardar, "Guardar copia en mi carpeta", tint = Color.White) }
                    IconButton({
                        haptica(Toque.Suave)
                        scope.launch { compartirTemporal(ctx, actual, nombreDe(actual)) }
                    }) { Icon(Ico.Compartir, "Compartir", tint = Color.White) }
                }
            }
        }
    }
}

private fun nombreDe(m: Msg): String {
    if (m.nombre.isNotBlank()) return m.nombre
    val ext = when {
        m.mime.contains("jpeg") -> "jpg"
        m.mime.contains("png") -> "png"
        m.mime.contains("webp") -> "webp"
        m.mime.contains("mp4") -> "mp4"
        m.mime.contains("ogg") -> "ogg"
        m.mime.contains("pdf") -> "pdf"
        else -> "bin"
    }
    return "${m.categoria.ifEmpty { "archivo" }}_${m.id.takeLast(8)}.$ext"
}

/** Copia sin cifrar a Documents/BotChat/<chat>/. Avisa claramente que la copia ya no está cifrada. */
suspend fun guardarCopia(ctx: Context, m: Msg, nombreChat: String) {
    val r = Exportar.guardarCopia(ctx, Repo.archivo(m.ruta), nombreChat, nombreDe(m), m.mime)
    val texto = r.fold({ "Copia guardada en $it. Esa copia ya NO está cifrada." }, { "No se pudo guardar la copia: ${it.message}" })
    Toast.makeText(ctx, texto, Toast.LENGTH_LONG).show()
}

/** Descifra una copia temporal y abre el selector de apps para compartirla (permiso temporal con FileProvider). */
suspend fun compartirTemporal(ctx: Context, m: Msg, nombre: String, ver: Boolean = false) {
    try {
        val f = withContext(Dispatchers.IO) { Temporales.descifrar(ctx, Repo.archivo(m.ruta), nombre) }
        val uri = androidx.core.content.FileProvider.getUriForFile(ctx, ctx.packageName + ".compartir", f)
        val mime = com.botchat.app.medios.TipoArchivo.mimeEfectivo(nombre, m.mime)
        val intent = if (ver) android.content.Intent(android.content.Intent.ACTION_VIEW).setDataAndType(uri, mime)
        else android.content.Intent(android.content.Intent.ACTION_SEND).setType(mime).putExtra(android.content.Intent.EXTRA_STREAM, uri)
        intent.addFlags(android.content.Intent.FLAG_GRANT_READ_URI_PERMISSION)
        val selector = android.content.Intent.createChooser(intent, if (ver) "Abrir con" else "Compartir").addFlags(android.content.Intent.FLAG_ACTIVITY_NEW_TASK)
        ctx.startActivity(selector)
        EstadoCompartir.pendiente = true
    } catch (e: Exception) {
        Toast.makeText(ctx, "No se pudo preparar el archivo: ${e.message}", Toast.LENGTH_LONG).show()
    }
}

/** Marca que hay una copia temporal compartida: se borra al volver a la app. */
object EstadoCompartir { @Volatile var pendiente = false }

// ---------- foto ----------

@Composable
private fun PaginaFoto(m: Msg, alCambiarZoom: (Boolean) -> Unit, alTocar: () -> Unit) {
    val ctx = LocalContext.current
    var imagen by remember(m.id) { mutableStateOf<ImageBitmap?>(null) }
    var fallo by remember(m.id) { mutableStateOf(false) }
    LaunchedEffect(m.id) {
        val b = CacheImagenes.cargar(ctx, Repo.archivo(m.ruta), 2400)
        if (b != null) imagen = b.asImageBitmap() else fallo = true
    }
    Box(Modifier.fillMaxSize(), Alignment.Center) {
        val img = imagen
        when {
            img != null -> ImagenConZoom(img, alCambiarZoom, alTocar)
            fallo -> EstadoVacio(Ico.Foto, "No se pudo abrir", "El archivo está dañado o ya no existe.")
            else -> CircularProgressIndicator(color = Color.White)
        }
    }
}

@Composable
private fun ImagenConZoom(imagen: ImageBitmap, alCambiarZoom: (Boolean) -> Unit, alTocar: () -> Unit) {
    var escala by remember { mutableFloatStateOf(1f) }
    var mov by remember { mutableStateOf(Offset.Zero) }
    var tam by remember { mutableStateOf(IntSize.Zero) }
    val scope = rememberCoroutineScope()
    fun limitar(o: Offset, e: Float): Offset {
        val mx = tam.width * (e - 1f) / 2f
        val my = tam.height * (e - 1f) / 2f
        return Offset(o.x.coerceIn(-mx, mx), o.y.coerceIn(-my, my))
    }
    Box(
        Modifier.fillMaxSize().onSizeChanged { tam = it }
            .pointerInput(Unit) {
                detectTapGestures(
                    onTap = { alTocar() },
                    onDoubleTap = {
                        scope.launch {
                            val destino = if (escala > 1.05f) 1f else 2.5f
                            val a = Animatable(escala)
                            a.animateTo(destino) { escala = value; mov = limitar(mov, value); alCambiarZoom(value > 1.02f) }
                            if (destino == 1f) mov = Offset.Zero
                        }
                    },
                )
            }
            .pointerInput(Unit) {
                // Solo toma el gesto cuando hay zoom o dos dedos: con un dedo y sin zoom deja pasar el deslizamiento al paginador.
                awaitEachGesture {
                    awaitFirstDown(requireUnconsumed = false)
                    do {
                        val evento = awaitPointerEvent()
                        val zoomPaso = evento.calculateZoom()
                        val pan = evento.calculatePan()
                        if (escala > 1.02f || evento.changes.size >= 2) {
                            val nueva = (escala * zoomPaso).coerceIn(1f, 5f)
                            mov = limitar(mov + pan, nueva)
                            escala = nueva
                            alCambiarZoom(nueva > 1.02f)
                            evento.changes.forEach { if (it.positionChanged()) it.consume() }
                        }
                    } while (evento.changes.any { it.pressed })
                    if (escala <= 1.02f) { escala = 1f; mov = Offset.Zero; alCambiarZoom(false) }
                }
            },
    ) {
        Image(
            imagen, null, Modifier.fillMaxSize().graphicsLayer { scaleX = escala; scaleY = escala; translationX = mov.x; translationY = mov.y },
            contentScale = ContentScale.Fit,
        )
    }
}

// ---------- video ----------

@OptIn(UnstableApi::class)
@Composable
private fun PaginaVideo(m: Msg, activa: Boolean, alTocar: () -> Unit) {
    val ctx = LocalContext.current
    // Para un video vecino solo se precargan los primeros bloques (calienta el disco; no se reproduce nada).
    LaunchedEffect(m.id, activa) {
        if (!activa) withContext(Dispatchers.IO) {
            runCatching { ArchivoCifrado(FuenteArchivo(Repo.archivo(m.ruta)), ClaveDatos.obtener(ctx)).use { it.leer(0, ByteArray(128 * 1024), 0, 128 * 1024) } }
        }
    }
    if (!activa) {
        Box(Modifier.fillMaxSize(), Alignment.Center) { Icon(Ico.Video, null, Modifier.size(56.dp), tint = Color.White.copy(alpha = 0.5f)) }
        return
    }
    ReproductorVideo(m, alTocar)
}

@OptIn(UnstableApi::class)
@Composable
fun ReproductorVideo(m: Msg, alTocar: () -> Unit, alMedir: ((String, Long) -> Unit)? = null) {
    val ctx = LocalContext.current
    var relacion by remember { mutableFloatStateOf(0f) }
    var reproduciendo by remember { mutableStateOf(true) }
    var posMs by remember { mutableStateOf(0L) }
    var durMs by remember { mutableStateOf(0L) }
    var error by remember { mutableStateOf(false) }
    val inicio = remember { System.nanoTime() }
    val player = remember(m.id) {
        ExoPlayer.Builder(ctx.applicationContext).build().apply {
            val fuente = ProgressiveMediaSource.Factory(FuenteCifrada.Fabrica(ClaveDatos.obtener(ctx))).createMediaSource(MediaItem.fromUri(Uri.fromFile(Repo.archivo(m.ruta))))
            setMediaSource(fuente)
            playWhenReady = true
            prepare()
        }
    }
    DisposableEffect(player) {
        val oyente = object : Player.Listener {
            override fun onIsPlayingChanged(isPlaying: Boolean) { reproduciendo = isPlaying }
            override fun onVideoSizeChanged(v: VideoSize) { if (v.height > 0) relacion = v.width * v.pixelWidthHeightRatio / v.height }
            override fun onRenderedFirstFrame() { alMedir?.invoke("primer_cuadro", (System.nanoTime() - inicio) / 1_000_000) }
            override fun onPlayerError(e: androidx.media3.common.PlaybackException) { error = true }
        }
        player.addListener(oyente)
        onDispose { player.removeListener(oyente); player.release() }
    }
    LaunchedEffect(player) {
        while (true) { posMs = player.currentPosition; durMs = player.duration.coerceAtLeast(0); delay(250) }
    }
    Box(Modifier.fillMaxSize().clickable(indication = null, interactionSource = remember { androidx.compose.foundation.interaction.MutableInteractionSource() }) { alTocar() }, Alignment.Center) {
        if (error) {
            EstadoVacio(Ico.Video, "No se pudo reproducir", "El video está dañado o su formato no es compatible.")
        } else {
            AndroidView(
                factory = { SurfaceView(it) },
                update = { player.setVideoSurfaceView(it) },
                modifier = (if (relacion > 0f) Modifier.aspectRatio(relacion) else Modifier.size(1.dp)).fillMaxWidth(),
            )
        }
        Column(Modifier.align(Alignment.BottomCenter).fillMaxWidth().background(Color.Black.copy(alpha = 0.4f)).navigationBarsPadding().padding(horizontal = Esp.l, vertical = Esp.s)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                IconButton({ if (player.isPlaying) player.pause() else { if (player.playbackState == Player.STATE_ENDED) player.seekTo(0); player.play() } }) {
                    Icon(if (reproduciendo) Ico.Pausa else Ico.Reproducir, if (reproduciendo) "Pausar" else "Reproducir", tint = Color.White)
                }
                Text(duracion((posMs / 1000).toInt()), color = Color.White, style = MaterialTheme.typography.labelMedium)
                Slider(
                    value = if (durMs > 0) (posMs.toFloat() / durMs).coerceIn(0f, 1f) else 0f,
                    onValueChange = { f -> if (durMs > 0) { val t0 = System.nanoTime(); player.seekTo((durMs * f).toLong()); posMs = (durMs * f).toLong(); alMedir?.invoke("salto", (System.nanoTime() - t0) / 1_000_000) } },
                    modifier = Modifier.weight(1f).padding(horizontal = Esp.s),
                )
                Text(duracion((durMs / 1000).toInt()), color = Color.White, style = MaterialTheme.typography.labelMedium)
            }
        }
    }
}
