@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.PrimaryTabRow
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Tab
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
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
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.media3.common.util.UnstableApi
import com.botchat.app.datos.Msg
import com.botchat.app.datos.Repo
import com.botchat.app.medios.CacheImagenes
import com.botchat.app.medios.ReproductorAudio
import kotlinx.coroutines.launch

private val PESTANAS = listOf("Fotos" to "foto", "Videos" to "video", "Audios" to "audio", "Documentos" to "documento")

/** Archivos compartidos de un chat, con pestañas Fotos / Videos / Audios / Documentos. */
@OptIn(UnstableApi::class)
@Composable
fun PantallaCompartidos(botId: Long, jid: String, nombre: String, alVolver: () -> Unit, alAbrirVisor: (String) -> Unit, alAbrirDocumento: (String) -> Unit) {
    val scope = rememberCoroutineScope()
    val pager = rememberPagerState { PESTANAS.size }
    val haptica = rememberHaptica()
    var version by remember { mutableStateOf(0) }
    LaunchedEffect(jid) { Repo.cambios.collect { evs -> if (evs.any { it.startsWith("archivo:$jid:") || it == "chat:$jid" }) version++ } }
    androidx.compose.runtime.DisposableEffect(Unit) { onDispose { ReproductorAudio.detener(); CacheImagenes.vaciar() } }
    Scaffold(
        topBar = {
            Column {
                TopAppBar(
                    navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } },
                    title = { Column { Text("Archivos compartidos"); TextoUnaLinea(nombre, estilo = MaterialTheme.typography.bodySmall) } },
                )
                PrimaryTabRow(pager.currentPage) {
                    PESTANAS.forEachIndexed { i, (titulo, _) -> Tab(pager.currentPage == i, { haptica(Toque.Suave); scope.launch { pager.animateScrollToPage(i) } }, text = { Text(titulo) }) }
                }
            }
        },
    ) { relleno ->
        HorizontalPager(pager, Modifier.fillMaxSize().padding(relleno)) { pagina ->
            val categoria = PESTANAS[pagina].second
            val lista by androidx.compose.runtime.produceState<List<Msg>?>(null, categoria, version) {
                value = runCatching { Repo.archivos(botId, jid, categoria, false, 1000) }.getOrDefault(emptyList())
            }
            val datos = lista
            when {
                datos == null -> Box(Modifier.fillMaxSize(), Alignment.Center) { CircularProgressIndicator() }
                datos.isEmpty() -> EstadoVacio(
                    when (categoria) { "foto" -> Ico.Foto; "video" -> Ico.Video; "audio" -> Ico.Audio; else -> Ico.Documento },
                    "Nada por aquí", "Cuando este chat tenga ${PESTANAS[pagina].first.lowercase()}, aparecerán en esta pestaña.", Modifier.padding(top = 80.dp),
                )
                categoria == "foto" || categoria == "video" -> Cuadricula(botId, jid, datos, alAbrirVisor)
                categoria == "audio" -> ListaAudios(botId, jid, datos)
                else -> ListaDocumentos(botId, jid, datos, alAbrirDocumento)
            }
        }
    }
}

@Composable
private fun Cuadricula(botId: Long, jid: String, datos: List<Msg>, alAbrirVisor: (String) -> Unit) {
    val ctx = LocalContext.current
    val px = with(LocalDensity.current) { 140.dp.roundToPx() }
    LazyVerticalGrid(GridCells.Adaptive(110.dp), Modifier.fillMaxSize(), contentPadding = PaddingValues(2.dp), horizontalArrangement = Arrangement.spacedBy(2.dp), verticalArrangement = Arrangement.spacedBy(2.dp)) {
        items(datos, key = { it.id }) { m ->
            val ruta = if (m.descargado) m.ruta else m.miniatura
            var img by remember(ruta) { mutableStateOf<androidx.compose.ui.graphics.ImageBitmap?>(null) }
            LaunchedEffect(ruta) { img = if (ruta.isEmpty()) null else CacheImagenes.cargar(ctx, Repo.archivo(ruta), px)?.asImageBitmap() }
            Box(Modifier.aspectRatio(1f).background(MaterialTheme.colorScheme.surfaceContainerHighest).clickable(enabled = m.descargado) { alAbrirVisor(m.id) }, Alignment.Center) {
                val i = img
                if (i != null) Image(i, null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
                if (m.categoria == "video") Icon(Ico.Reproducir, null, Modifier.size(32.dp).background(Color.Black.copy(alpha = 0.4f), CircleShape), tint = Color.White)
                if (!m.descargado) Icon(Ico.Descargar, "No descargado", Modifier.align(Alignment.BottomEnd).padding(6.dp).size(20.dp), tint = Color.White)
            }
        }
    }
}

@OptIn(UnstableApi::class)
@Composable
private fun ListaAudios(botId: Long, jid: String, datos: List<Msg>) {
    val ctx = LocalContext.current
    val rep by ReproductorAudio.estado.collectAsState()
    val scope = rememberCoroutineScope()
    LazyColumn(Modifier.fillMaxSize()) {
        items(datos, key = { it.id }) { m ->
            val clave = "${m.chat}|${m.id}"
            val activo = rep.clave == clave
            Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s), verticalAlignment = Alignment.CenterVertically) {
                IconButton({
                    if (m.descargado) ReproductorAudio.alternar(ctx, clave, Repo.archivo(m.ruta)) else scope.launch { runCatching { Repo.descargar(botId, jid, m.id) } }
                }) { Icon(if (!m.descargado) Ico.Descargar else if (activo && rep.reproduciendo) Ico.Pausa else Ico.Reproducir, null) }
                Column(Modifier.weight(1f)) {
                    Text(if (m.notaVoz) "Nota de voz" else "Audio", style = MaterialTheme.typography.titleSmall)
                    Text("${etiquetaDia(m.ts)} · ${horaDe(m.ts)}${if (m.dur > 0) " · " + duracion(m.dur) else ""}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                Text(if (m.bytes > 0) formatoBytes(m.bytes) else "", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
        }
    }
}

@Composable
private fun ListaDocumentos(botId: Long, jid: String, datos: List<Msg>, alAbrir: (String) -> Unit) {
    val scope = rememberCoroutineScope()
    LazyColumn(Modifier.fillMaxSize()) {
        items(datos, key = { it.id }) { m ->
            Row(
                Modifier.fillMaxWidth().clickable { if (m.descargado) alAbrir(m.id) else scope.launch { runCatching { Repo.descargar(botId, jid, m.id) } } }.padding(horizontal = Esp.l, vertical = Esp.m),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(Modifier.size(44.dp).clip(RoundedCornerShape(12.dp)).background(MaterialTheme.colorScheme.secondaryContainer), Alignment.Center) { Icon(Ico.Documento, null, tint = MaterialTheme.colorScheme.onSecondaryContainer) }
                Spacer(Modifier.width(Esp.m))
                Column(Modifier.weight(1f)) {
                    Text(m.nombre.ifBlank { "Documento" }, maxLines = 1, overflow = TextOverflow.Ellipsis, style = MaterialTheme.typography.titleSmall)
                    Text("${etiquetaDia(m.ts)} · ${if (m.bytes > 0) formatoBytes(m.bytes) else ""}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                if (!m.descargado) Icon(Ico.Descargar, "Descargar", tint = MaterialTheme.colorScheme.primary)
            }
            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
        }
    }
}
