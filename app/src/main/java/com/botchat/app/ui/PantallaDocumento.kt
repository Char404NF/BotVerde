package com.botchat.app.ui

import android.graphics.Bitmap
import android.graphics.pdf.PdfRenderer
import android.os.ParcelFileDescriptor
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import com.botchat.app.datos.Msg
import com.botchat.app.datos.Repo
import com.botchat.app.medios.Exportar
import com.botchat.app.medios.Temporales
import com.botchat.app.medios.TipoArchivo
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import java.io.File

/** Documentos: los PDF se ven aquí (copia temporal descifrada en cache, borrada al cerrar); los demás se abren con otra app. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PantallaDocumento(botId: Long, jid: String, id: String, nombreChat: String, alVolver: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var msg by remember { mutableStateOf<Msg?>(null) }
    var cargado by remember { mutableStateOf(false) }
    LaunchedEffect(id) {
        msg = Repo.archivos(botId, jid, "documento", true, 1000).firstOrNull { it.id == id }
        cargado = true
    }
    // Al volver de otra app (abrir o compartir), se borra la copia temporal.
    LifecycleEventEffect(Lifecycle.Event.ON_RESUME) {
        if (EstadoCompartir.pendiente) { EstadoCompartir.pendiente = false; Temporales.borrarTodo(ctx) }
    }
    val m = msg
    val esPdf = m != null && TipoArchivo.familia(m.nombre, m.mime) == TipoArchivo.Familia.PDF
    Scaffold(
        topBar = {
            TopAppBar(
                navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } },
                title = { Text(m?.let { TipoArchivo.nombreMostrable(it.nombre, it.mime) } ?: "Documento", maxLines = 1, overflow = TextOverflow.Ellipsis) },
                actions = {
                    if (m != null && Exportar.disponible) IconButton({ scope.launch { guardarCopia(ctx, m, nombreChat) } }) { Icon(Ico.Guardar, "Guardar en la galería") }
                    if (m != null) IconButton({ scope.launch { compartirTemporal(ctx, m, TipoArchivo.nombreMostrable(m.nombre, m.mime)) } }) { Icon(Ico.Compartir, "Compartir") }
                },
            )
        },
    ) { relleno ->
        Box(Modifier.fillMaxSize().padding(relleno)) {
            when {
                !cargado -> CircularProgressIndicator(Modifier.align(Alignment.Center))
                m == null -> EstadoVacio(Ico.Documento, "No está descargado", "Descarga el documento desde el chat para verlo.", Modifier.align(Alignment.Center))
                esPdf -> VisorPdf(m)
                else -> DocumentoGenerico(m, nombreChat)
            }
        }
    }
}

@Composable
private fun DocumentoGenerico(m: Msg, nombreChat: String) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    Column(Modifier.fillMaxSize().padding(Esp.xl), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
        Box(Modifier.size(96.dp).clip(RoundedCornerShape(24.dp)).background(MaterialTheme.colorScheme.secondaryContainer), Alignment.Center) { Icon(Ico.Documento, null, Modifier.size(52.dp), tint = MaterialTheme.colorScheme.onSecondaryContainer) }
        Spacer(Modifier.height(Esp.l))
        Text(TipoArchivo.nombreMostrable(m.nombre, m.mime), style = MaterialTheme.typography.titleLarge)
        Text(listOfNotNull(TipoArchivo.etiqueta(m.nombre, m.mime), if (m.bytes > 0) formatoBytes(m.bytes) else null).joinToString(" · "), color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(Esp.xl))
        Button({ scope.launch { compartirTemporal(ctx, m, TipoArchivo.nombreMostrable(m.nombre, m.mime), ver = true) } }, Modifier.fillMaxWidth()) { Text("Abrir con…") }
        Spacer(Modifier.height(Esp.s))
        Text("Se abre una copia temporal que se borra al volver aquí, al bloquear la app o a los 10 minutos.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

@Composable
private fun VisorPdf(m: Msg) {
    val ctx = LocalContext.current
    var temporal by remember(m.id) { mutableStateOf<File?>(null) }
    var renderer by remember(m.id) { mutableStateOf<PdfRenderer?>(null) }
    var fallo by remember(m.id) { mutableStateOf(false) }
    val candado = remember(m.id) { Mutex() }
    LaunchedEffect(m.id) {
        try {
            val f = withContext(Dispatchers.IO) { Temporales.descifrar(ctx, Repo.archivo(m.ruta), "visor.pdf") }
            temporal = f
            renderer = withContext(Dispatchers.IO) { PdfRenderer(ParcelFileDescriptor.open(f, ParcelFileDescriptor.MODE_READ_ONLY)) }
        } catch (e: Exception) {
            fallo = true
        }
    }
    DisposableEffect(m.id) {
        onDispose {
            runCatching { renderer?.close() }
            temporal?.let { Temporales.borrar(it) }
        }
    }
    val r = renderer
    when {
        fallo -> EstadoVacio(Ico.Documento, "No se pudo abrir el PDF", "El archivo está dañado o protegido.")
        r == null -> Box(Modifier.fillMaxSize(), Alignment.Center) { CircularProgressIndicator() }
        else -> {
            val ancho = ctx.resources.displayMetrics.widthPixels
            LazyColumn(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.surfaceContainerHighest), verticalArrangement = Arrangement.spacedBy(Esp.s), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.s)) {
                itemsIndexed(List(r.pageCount) { it }) { _, i ->
                    var pagina by remember(i) { mutableStateOf<Bitmap?>(null) }
                    var relacion by remember(i) { mutableStateOf(0.707f) }
                    LaunchedEffect(i) {
                        val bmp = withContext(Dispatchers.Default) {
                            candado.withLock {
                                runCatching {
                                    r.openPage(i).use { p ->
                                        val alto = (ancho * p.height.toFloat() / p.width).toInt()
                                        Bitmap.createBitmap(ancho, alto, Bitmap.Config.ARGB_8888).also { b ->
                                            b.eraseColor(android.graphics.Color.WHITE)
                                            p.render(b, null, null, PdfRenderer.Page.RENDER_MODE_FOR_DISPLAY)
                                        }
                                    }
                                }.getOrNull()
                            }
                        }
                        if (bmp != null) { relacion = bmp.width.toFloat() / bmp.height; pagina = bmp }
                    }
                    Box(Modifier.fillMaxWidth().aspectRatio(relacion).background(Color.White), Alignment.Center) {
                        val b = pagina
                        if (b != null) Image(b.asImageBitmap(), "Página ${i + 1}", Modifier.fillMaxSize(), contentScale = ContentScale.FillWidth) else CircularProgressIndicator(Modifier.size(24.dp), strokeWidth = 2.dp)
                    }
                }
            }
        }
    }
}
