@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Paint
import android.media.MediaCodec
import android.media.MediaCodecInfo
import android.media.MediaFormat
import android.media.MediaMuxer
import android.net.Uri
import android.os.Build
import android.view.Choreographer
import android.view.Surface
import android.graphics.SurfaceTexture
import android.widget.Toast
import androidx.compose.foundation.gestures.animateScrollBy
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.media3.common.MediaItem
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.ProgressiveMediaSource
import com.botchat.app.Datos
import com.botchat.app.medios.CacheImagenes
import com.botchat.app.medios.FuenteCifrada
import com.botchat.app.seguridad.ArchivoCifrado
import com.botchat.app.seguridad.ClaveDatos
import com.botchat.app.seguridad.EscritorCifrado
import com.botchat.app.seguridad.FuenteArchivo
import com.botchat.app.datos.ChatItem
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeoutOrNull
import java.io.File
import java.io.FileOutputStream
import kotlin.coroutines.resume
import kotlin.random.Random

private const val OBJ_FOTO_MS = 200
private const val OBJ_VIDEO_MS = 1000

/**
 * "Medir rendimiento": pruebas reales en este teléfono con archivos sintéticos (nada de tus mensajes).
 * Mide abrir una foto de ~5 MB, el primer cuadro de un video de ~50 MB, un salto a la mitad del video, pasar a la
 * foto siguiente y la fluidez al desplazar 200 chats (cuadros caídos), contra los objetivos de la app.
 */
@OptIn(UnstableApi::class)
@Composable
fun PantallaDiagnostico(alVolver: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var lineas by remember { mutableStateOf(listOf<String>()) }
    var corriendo by remember { mutableStateOf(false) }
    var progreso by remember { mutableStateOf(0f) }
    var modoDesplazar by remember { mutableStateOf(false) }
    val estadoLista = rememberLazyListState()
    val filas = remember { List(200) { ChatItem("diag$it@x", "Contacto de prueba ${it + 1}", "+52 55 ${1000 + it} ${2000 + it}", "MX", it % 7 == 0, "", 0, 1_700_000_000L - it * 3600L, "Mensaje de prueba número ${it + 1} con algo de texto para medir", "texto", false, false) } }

    fun anota(s: String) { lineas = lineas + s }

    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Medir rendimiento") }) }) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).padding(Esp.l), verticalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(Esp.m)) {
            Text("Usa archivos de prueba creados aquí mismo (no toca tus mensajes). Tarda cerca de un minuto. Déjalo en pantalla mientras corre.", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Row(horizontalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(Esp.s)) {
                Button(enabled = !corriendo, onClick = {
                    corriendo = true; lineas = emptyList(); progreso = 0f
                    scope.launch {
                        try {
                            anota("Teléfono: ${Build.MANUFACTURER} ${Build.MODEL} · Android ${Build.VERSION.RELEASE} (API ${Build.VERSION.SDK_INT})")
                            anota("Pantalla: ${"%.0f".format(ctx.display.refreshRate)} Hz · Objetivos: foto < $OBJ_FOTO_MS ms, video < $OBJ_VIDEO_MS ms, desplazamiento fluido")
                            corre(ctx, ::anota, { progreso = it }, { modoDesplazar = it }, estadoLista, filas.size)
                        } catch (e: Exception) {
                            anota("Error inesperado: ${e.message}")
                        } finally {
                            modoDesplazar = false; corriendo = false; progreso = 1f
                        }
                    }
                }) { Text(if (corriendo) "Midiendo…" else "Iniciar pruebas") }
                OutlinedButton(enabled = lineas.isNotEmpty(), onClick = {
                    ctx.getSystemService(ClipboardManager::class.java).setPrimaryClip(ClipData.newPlainText("rendimiento", lineas.joinToString("\n")))
                    Toast.makeText(ctx, "Resultados copiados", Toast.LENGTH_SHORT).show()
                }) { Text("Copiar resultados") }
            }
            if (corriendo) LinearProgressIndicator(progress = { progreso }, Modifier.fillMaxWidth())
            Surface(Modifier.fillMaxWidth().height(if (modoDesplazar) 360.dp else 200.dp), shape = RoundedCornerShape(12.dp), color = MaterialTheme.colorScheme.surfaceContainerHigh) {
                if (modoDesplazar) {
                    LazyColumn(state = estadoLista, modifier = Modifier.fillMaxSize()) {
                        items(filas, key = { it.jid }) { c -> FilaChat(c, Modifier, {}) }
                    }
                } else {
                    Text(lineas.joinToString("\n"), Modifier.verticalScroll(rememberScrollState()).padding(10.dp), fontFamily = FontFamily.Monospace, fontSize = 12.sp, lineHeight = 17.sp)
                }
            }
        }
    }
}

private fun mediana(v: List<Long>): Long = v.sorted()[v.size / 2]

@OptIn(UnstableApi::class)
private suspend fun corre(
    ctx: Context, anota: (String) -> Unit, avance: (Float) -> Unit, desplazando: (Boolean) -> Unit,
    estadoLista: androidx.compose.foundation.lazy.LazyListState, nFilas: Int,
) {
    val clave = ClaveDatos.obtener(ctx)
    val carpeta = File(ctx.cacheDir, "diagnostico").apply { deleteRecursively(); mkdirs() }
    try {
        // 1) Fotos de ~5 MB
        anota("\n— Fotos —")
        val fotos = withContext(Dispatchers.Default) { List(2) { i -> crearFoto(carpeta, clave, i) } }
        avance(0.15f)
        val abrir = fotos.map { (f, _) -> medirMs { withContext(Dispatchers.Default) { CacheImagenes.decodificar(f, clave, ctx.resources.displayMetrics.widthPixels) } } }
        val rep = List(3) { medirMs { withContext(Dispatchers.Default) { CacheImagenes.decodificar(fotos[0].first, clave, ctx.resources.displayMetrics.widthPixels) } } }
        anota("Foto de ${formatoBytes(fotos[0].second)}: abrir (descifrar + decodificar a la pantalla) = ${mediana(rep)} ms  → " + veredicto(mediana(rep), OBJ_FOTO_MS))
        anota("Pasar a la foto siguiente (${formatoBytes(fotos[1].second)}, sin precarga) = ${abrir[1]} ms  → " + veredicto(abrir[1], OBJ_FOTO_MS))
        // Con la precarga que hace el visor (la vecina ya está en memoria), pasar a la siguiente es solo mostrarla:
        withContext(Dispatchers.Default) { CacheImagenes.vaciar() }
        val pre = CacheImagenes.cargar(ctx, fotos[1].first, ctx.resources.displayMetrics.widthPixels, "diag")
        val conPre = medirMs { CacheImagenes.cargar(ctx, fotos[1].first, ctx.resources.displayMetrics.widthPixels, "diag") }
        anota("Pasar a la siguiente CON precarga (ya en memoria) = $conPre ms  → " + veredicto(conPre, OBJ_FOTO_MS) + if (pre == null) "  (no se pudo decodificar)" else "")
        CacheImagenes.vaciar()
        avance(0.3f)

        // 2) Video de ~50 MB
        anota("\n— Video —")
        val video = withContext(Dispatchers.Default) { crearVideo(ctx, carpeta, clave, anota) }
        if (video != null) {
            avance(0.55f)
            val (arch, tam, durMs) = video
            anota("Video de prueba: ${formatoBytes(tam)}, ${durMs / 1000} s")
            val t = medirReproduccion(ctx, arch, clave, durMs)
            anota("Primer cuadro del video = ${t.primerCuadro} ms  → " + veredicto(t.primerCuadro, OBJ_VIDEO_MS))
            anota("Salto a la mitad del video (hasta que sigue reproduciendo) = ${t.salto} ms  → " + veredicto(t.salto, OBJ_VIDEO_MS))
        } else {
            anota("No se pudo crear el video de prueba en este teléfono; no se midió el primer cuadro ni el salto.")
        }
        avance(0.7f)

        // 3) Velocidad de descifrado (independiente del video)
        anota("\n— Descifrado —")
        val grande = withContext(Dispatchers.IO) { archivoAleatorio(File(carpeta, "grande.enc"), clave, 50L * 1024 * 1024) }
        val buf = ByteArray(64 * 1024)
        val seq = medirMs {
            withContext(Dispatchers.IO) { ArchivoCifrado(FuenteArchivo(grande), clave).use { a -> var pos = 0L; while (pos < a.tam) { val n = a.leer(pos, buf, 0, buf.size); if (n <= 0) break; pos += n } } }
        }
        anota("Descifrar 50 MB de corrido = $seq ms (${"%.0f".format(50.0 * 1000 / seq.coerceAtLeast(1))} MB/s)")
        val saltos = List(5) { i -> medirMs { withContext(Dispatchers.IO) { ArchivoCifrado(FuenteArchivo(grande), clave).use { a -> a.leer(25L * 1024 * 1024 + i * 4096, ByteArray(256 * 1024), 0, 256 * 1024) } } } }
        anota("Abrir y leer 256 KB a la mitad del archivo (acceso aleatorio) = ${mediana(saltos)} ms")
        avance(0.8f)

        // 4) Fluidez al desplazar 200 chats
        anota("\n— Lista de 200 chats —")
        desplazando(true)
        delay(500)
        val f = medirFluidez(estadoLista, ctx, nFilas)
        desplazando(false)
        anota("Desplazamiento de ${f.segundos} s: ${f.cuadros} cuadros, ${f.caidos} caídos (${"%.1f".format(100.0 * f.caidos / f.cuadros.coerceAtLeast(1))} %), ${f.muyLentos} muy lentos; peor cuadro = ${f.peorMs} ms")
        anota("  → " + (if (f.caidos * 100 <= f.cuadros * 5 && f.muyLentos * 100 <= f.cuadros) "FLUIDO" else "NO cumple el objetivo (más de 5 % de cuadros caídos)") + "  [cuadro caído = más de ${"%.1f".format(1.5 * 1000 / ctx.display.refreshRate)} ms]")
        anota("\nListo. Esta prueba mide el teléfono y los archivos sintéticos; con tus mensajes reales los números pueden variar.")
    } finally {
        runCatching { carpeta.deleteRecursively() }
        CacheImagenes.vaciar()
    }
}

private fun veredicto(ms: Long, objetivo: Int) = if (ms <= objetivo) "CUMPLE (≤ $objetivo ms)" else "NO CUMPLE (objetivo $objetivo ms)"

private suspend inline fun medirMs(f: suspend () -> Unit): Long { val t = System.nanoTime(); f(); return (System.nanoTime() - t) / 1_000_000 }

/** Foto de ~5 MB con ruido (no se comprime mucho), cifrada. Devuelve el archivo y su tamaño sin cifrar. */
private fun crearFoto(dir: File, clave: ByteArray, indice: Int): Pair<File, Long> {
    val w = 3000; val h = 2250
    val bmp = Bitmap.createBitmap(w, h, Bitmap.Config.ARGB_8888)
    val rnd = Random(100 + indice)
    val fila = IntArray(w)
    for (y in 0 until h) {
        for (x in 0 until w) fila[x] = (0xFF shl 24) or ((x * 255 / w) shl 16) or ((y * 255 / h) shl 8) or (rnd.nextInt(256) / 3 + 80)
        bmp.setPixels(fila, 0, w, 0, y, w, 1)
    }
    var calidad = 90
    var jpeg: ByteArray
    do {
        val s = java.io.ByteArrayOutputStream()
        bmp.compress(Bitmap.CompressFormat.JPEG, calidad, s)
        jpeg = s.toByteArray()
        calidad -= 6
    } while (jpeg.size > 5_400_000 && calidad > 30)
    bmp.recycle()
    val f = File(dir, "foto$indice.enc")
    EscritorCifrado(FileOutputStream(f), clave).use { it.write(jpeg) }
    return f to jpeg.size.toLong()
}

private fun archivoAleatorio(f: File, clave: ByteArray, bytes: Long): File {
    val rnd = Random(7)
    val trozo = ByteArray(256 * 1024).also { rnd.nextBytes(it) }
    EscritorCifrado(FileOutputStream(f), clave).use { w -> var n = 0L; while (n < bytes) { w.write(trozo); n += trozo.size } }
    return f
}

private data class VideoPrueba(val archivo: File, val bytes: Long, val durMs: Long)

/** Crea un MP4 real (H.264) de ~50 MB con el codificador del teléfono y lo cifra. Devuelve null si el teléfono no puede. */
private fun crearVideo(ctx: Context, dir: File, clave: ByteArray, anota: (String) -> Unit): VideoPrueba? {
    val plano = File(dir, "video_plano.mp4")
    return try {
        val w = 1280; val h = 720; val fps = 30; val segundos = 16; val bitrate = 25_000_000
        val fmt = MediaFormat.createVideoFormat(MediaFormat.MIMETYPE_VIDEO_AVC, w, h).apply {
            setInteger(MediaFormat.KEY_COLOR_FORMAT, MediaCodecInfo.CodecCapabilities.COLOR_FormatSurface)
            setInteger(MediaFormat.KEY_BIT_RATE, bitrate); setInteger(MediaFormat.KEY_FRAME_RATE, fps); setInteger(MediaFormat.KEY_I_FRAME_INTERVAL, 1)
        }
        val enc = MediaCodec.createEncoderByType(MediaFormat.MIMETYPE_VIDEO_AVC)
        enc.configure(fmt, null, null, MediaCodec.CONFIGURE_FLAG_ENCODE)
        val superficie = enc.createInputSurface()
        enc.start()
        val mux = MediaMuxer(plano.absolutePath, MediaMuxer.OutputFormat.MUXER_OUTPUT_MPEG_4)
        var pista = -1; var muxIniciado = false
        val info = MediaCodec.BufferInfo()
        val pincel = Paint()
        val rnd = Random(3)
        fun vacia(fin: Boolean) {
            while (true) {
                val i = enc.dequeueOutputBuffer(info, if (fin) 10_000 else 0)
                when {
                    i == MediaCodec.INFO_TRY_AGAIN_LATER -> if (!fin) return else continue
                    i == MediaCodec.INFO_OUTPUT_FORMAT_CHANGED -> { pista = mux.addTrack(enc.outputFormat); mux.start(); muxIniciado = true }
                    i >= 0 -> {
                        val b = enc.getOutputBuffer(i)!!
                        if (info.flags and MediaCodec.BUFFER_FLAG_CODEC_CONFIG != 0) info.size = 0
                        if (info.size > 0 && muxIniciado) { b.position(info.offset); b.limit(info.offset + info.size); mux.writeSampleData(pista, b, info) }
                        enc.releaseOutputBuffer(i, false)
                        if (info.flags and MediaCodec.BUFFER_FLAG_END_OF_STREAM != 0) return
                    }
                }
            }
        }
        val total = fps * segundos
        for (n in 0 until total) {
            val c: Canvas = superficie.lockCanvas(null)
            c.drawColor(android.graphics.Color.rgb(n * 4 % 256, 40, 90))
            repeat(900) { // mucho detalle por cuadro para que el codificador use el bitrate completo
                pincel.color = android.graphics.Color.rgb(rnd.nextInt(256), rnd.nextInt(256), rnd.nextInt(256))
                val x = rnd.nextInt(w); val y = rnd.nextInt(h)
                c.drawRect(x.toFloat(), y.toFloat(), (x + 8 + rnd.nextInt(60)).toFloat(), (y + 8 + rnd.nextInt(60)).toFloat(), pincel)
            }
            superficie.unlockCanvasAndPost(c)
            vacia(false)
        }
        enc.signalEndOfInputStream()
        vacia(true)
        enc.stop(); enc.release(); mux.stop(); mux.release(); superficie.release()
        val bytes = plano.length()
        val cif = File(dir, "video.enc")
        EscritorCifrado(FileOutputStream(cif), clave).use { w2 -> plano.inputStream().use { it.copyTo(w2, 256 * 1024) } }
        plano.delete()
        VideoPrueba(cif, bytes, segundos * 1000L)
    } catch (e: Exception) {
        anota("(No se pudo generar el video: ${e.javaClass.simpleName}: ${e.message})")
        runCatching { plano.delete() }
        null
    }
}

private data class TiemposVideo(val primerCuadro: Long, val salto: Long)

@OptIn(UnstableApi::class)
private suspend fun medirReproduccion(ctx: Context, archivo: File, clave: ByteArray, durMs: Long): TiemposVideo = withContext(Dispatchers.Main) {
    val textura = SurfaceTexture(0).apply { setDefaultBufferSize(1280, 720) }
    val superficie = Surface(textura)
    val primer = CompletableDeferred<Long>()
    val salto = CompletableDeferred<Long>()
    var t0 = 0L
    var saltando = false
    val player = ExoPlayer.Builder(ctx.applicationContext).build()
    player.addListener(object : Player.Listener {
        override fun onRenderedFirstFrame() { if (!primer.isCompleted) primer.complete((System.nanoTime() - t0) / 1_000_000) }
        override fun onPlaybackStateChanged(state: Int) { if (saltando && state == Player.STATE_READY && !salto.isCompleted) salto.complete((System.nanoTime() - t0) / 1_000_000) }
    })
    player.setVideoSurface(superficie)
    player.setMediaSource(ProgressiveMediaSource.Factory(FuenteCifrada.Fabrica(clave)).createMediaSource(MediaItem.fromUri(Uri.fromFile(archivo))))
    player.playWhenReady = true
    t0 = System.nanoTime()
    player.prepare()
    val p = withTimeoutOrNull(15_000) { primer.await() } ?: -1L
    delay(800)
    saltando = true
    t0 = System.nanoTime()
    player.seekTo(durMs / 2)
    val s = withTimeoutOrNull(15_000) { salto.await() } ?: -1L
    player.release(); superficie.release(); textura.release()
    TiemposVideo(p, s)
}

private data class Fluidez(val segundos: Int, val cuadros: Int, val caidos: Int, val muyLentos: Int, val peorMs: Long)

/** Desplaza la lista arriba y abajo durante 8 s y cuenta los cuadros que tardaron más de lo debido (Choreographer). */
private suspend fun medirFluidez(estado: androidx.compose.foundation.lazy.LazyListState, ctx: Context, filas: Int): Fluidez {
    val periodoNs = (1_000_000_000.0 / ctx.display.refreshRate).toLong()
    var ultimo = 0L; var cuadros = 0; var caidos = 0; var lentos = 0; var peor = 0L
    var activo = true
    val cb = object : Choreographer.FrameCallback {
        override fun doFrame(frameTimeNanos: Long) {
            if (ultimo != 0L) {
                val d = frameTimeNanos - ultimo
                cuadros++
                if (d > periodoNs * 3 / 2) caidos++
                if (d > periodoNs * 3) lentos++
                if (d / 1_000_000 > peor) peor = d / 1_000_000
            }
            ultimo = frameTimeNanos
            if (activo) Choreographer.getInstance().postFrameCallback(this)
        }
    }
    withContext(Dispatchers.Main) { Choreographer.getInstance().postFrameCallback(cb) }
    val inicio = System.nanoTime()
    var abajo = true
    while ((System.nanoTime() - inicio) < 8_000_000_000L) {
        withContext(Dispatchers.Main) { estado.animateScrollBy(if (abajo) 9000f else -9000f, androidx.compose.animation.core.tween(1800, easing = androidx.compose.animation.core.LinearEasing)) }
        abajo = !abajo
    }
    activo = false
    return Fluidez(8, cuadros, caidos, lentos, peor)
}
