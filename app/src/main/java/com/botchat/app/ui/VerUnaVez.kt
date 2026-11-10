@file:OptIn(UnstableApi::class)

package com.botchat.app.ui

import android.app.Activity
import android.content.Context
import android.content.ContextWrapper
import android.view.WindowManager
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.media3.common.util.UnstableApi
import com.botchat.app.datos.Ajustes
import com.botchat.app.datos.Msg
import com.botchat.app.datos.Repo
import com.botchat.app.datos.VerUnaVezAbierto
import com.botchat.app.medios.CacheImagenes
import com.botchat.app.medios.ReproductorAudio
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import java.util.concurrent.atomic.AtomicBoolean

private fun actividadDe(c: Context): Activity? = when (c) { is Activity -> c; is ContextWrapper -> actividadDe(c.baseContext); else -> null }

private sealed interface EstadoUnaVez {
    data object Cargando : EstadoUnaVez
    data class Listo(val a: VerUnaVezAbierto) : EstadoUnaVez
    data class Fallo(val texto: String) : EstadoUnaVez
}

/**
 * Abre un "ver una vez" UNA sola vez, en pantalla segura (sin capturas ni grabación de pantalla), sin guardar, compartir ni reenviar.
 * Al cerrar, salir de la app o apagar la pantalla, el contenido y el archivo se borran para siempre.
 */
@Composable
fun PantallaVerUnaVez(botId: Long, jid: String, id: String, alCerrar: () -> Unit) {
    val ctx = LocalContext.current
    val ajustes = remember { Ajustes.get(ctx) }
    val actividad = remember { actividadDe(ctx) }
    DisposableEffect(Unit) {
        actividad?.window?.addFlags(WindowManager.LayoutParams.FLAG_SECURE)
        onDispose { if (!ajustes.secure.value) actividad?.window?.clearFlags(WindowManager.LayoutParams.FLAG_SECURE) }
    }
    var estado by remember { mutableStateOf<EstadoUnaVez>(EstadoUnaVez.Cargando) }
    LaunchedEffect(Unit) {
        estado = try { EstadoUnaVez.Listo(Repo.abrirVerUnaVez(botId, jid, id)) } catch (e: Exception) { EstadoUnaVez.Fallo(e.message ?: "Ya no está disponible.") }
    }
    val borrado = remember { AtomicBoolean(false) }
    fun borrar() {
        if (estado is EstadoUnaVez.Listo && borrado.compareAndSet(false, true)) CoroutineScope(Dispatchers.IO).launch { runCatching { Repo.cerrarVerUnaVez(botId, jid, id) } }
    }
    val dueno = LocalLifecycleOwner.current
    DisposableEffect(dueno) {
        val obs = LifecycleEventObserver { _, ev -> if (ev == Lifecycle.Event.ON_STOP) { borrar(); alCerrar() } }
        dueno.lifecycle.addObserver(obs)
        onDispose { dueno.lifecycle.removeObserver(obs); borrar(); CacheImagenes.vaciar(); ReproductorAudio.detener() }
    }
    BackHandler { borrar(); alCerrar() }

    Box(Modifier.fillMaxSize().background(Color.Black).testTag("pantalla_ver_una_vez")) {
        when (val e = estado) {
            EstadoUnaVez.Cargando -> CircularProgressIndicator(Modifier.align(Alignment.Center), color = Color.White)
            is EstadoUnaVez.Fallo -> Column(Modifier.align(Alignment.Center).padding(Esp.xl), horizontalAlignment = Alignment.CenterHorizontally) {
                Icon(Ico.Candado, null, Modifier.size(48.dp), tint = Color.White)
                Text(e.texto, Modifier.padding(top = Esp.m).testTag("ver_una_vez_error"), color = Color.White, textAlign = TextAlign.Center)
                FilledTonalButton({ alCerrar() }, Modifier.padding(top = Esp.l)) { Text("Volver") }
            }
            is EstadoUnaVez.Listo -> Contenido(id, jid, e.a)
        }
        Row(Modifier.fillMaxWidth().background(Color.Black.copy(alpha = 0.45f)).statusBarsPadding().padding(horizontal = Esp.s, vertical = Esp.xs), verticalAlignment = Alignment.CenterVertically) {
            IconButton({ borrar(); alCerrar() }, Modifier.testTag("cerrar_ver_una_vez")) { Icon(Ico.Cerrar, "Cerrar", tint = Color.White) }
            Text("Se borra al cerrar", Modifier.weight(1f), color = Color.White.copy(alpha = 0.85f), style = MaterialTheme.typography.bodyMedium)
        }
    }
}

@Composable
private fun Contenido(id: String, jid: String, a: VerUnaVezAbierto) {
    val ctx = LocalContext.current
    when (a.tipo) {
        "video" -> ReproductorVideo(mensajeDe(id, jid, a, "video"), alTocar = {})
        "audio" -> Column(Modifier.fillMaxSize(), verticalArrangement = Arrangement.Center, horizontalAlignment = Alignment.CenterHorizontally) {
            Icon(Ico.Audio, null, Modifier.size(64.dp), tint = Color.White)
            val rep by ReproductorAudio.estado.collectAsState()
            val clave = "unavez|$id"
            FilledTonalButton({ ReproductorAudio.alternar(ctx, clave, Repo.archivo(a.ruta)) }, Modifier.padding(top = Esp.l).testTag("reproducir_ver_una_vez")) {
                Icon(if (rep.clave == clave && rep.reproduciendo) Ico.Pausa else Ico.Reproducir, null); Text(if (rep.clave == clave && rep.reproduciendo) "Pausar" else "Reproducir")
            }
        }
        else -> {
            var imagen by remember { mutableStateOf<androidx.compose.ui.graphics.ImageBitmap?>(null) }
            var fallo by remember { mutableStateOf(false) }
            LaunchedEffect(a.ruta) { val b = CacheImagenes.cargar(ctx, Repo.archivo(a.ruta), 2200); if (b != null) imagen = b.asImageBitmap() else fallo = true }
            val img = imagen
            when {
                img != null -> Image(img, null, Modifier.fillMaxSize().testTag("imagen_ver_una_vez"), contentScale = ContentScale.Fit)
                fallo -> Text("No se pudo abrir la imagen", Modifier.fillMaxSize().padding(Esp.xl), color = Color.White, textAlign = TextAlign.Center)
                else -> Box(Modifier.fillMaxSize(), Alignment.Center) { CircularProgressIndicator(color = Color.White) }
            }
        }
    }
}

private fun mensajeDe(id: String, jid: String, a: VerUnaVezAbierto, tipo: String) = Msg(
    id = id, chat = jid, remitenteId = "", remitente = "", esMio = false, tipo = tipo, texto = "", ts = 0, borrado = false, editado = false, estado = "descargado",
    bytes = 0, mime = a.mime, nombre = "", ruta = a.ruta, miniatura = "", ancho = 0, alto = 0, dur = 0, notaVoz = false, puedeBajarse = false,
)
