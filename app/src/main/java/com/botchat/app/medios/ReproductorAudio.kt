package com.botchat.app.medios

import android.content.Context
import android.net.Uri
import androidx.media3.common.MediaItem
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.ProgressiveMediaSource
import com.botchat.app.seguridad.ClaveDatos
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.io.File

/** Reproduce un audio a la vez, descifrando en flujo desde el archivo cifrado. */
@UnstableApi
object ReproductorAudio {
    data class Estado(val clave: String? = null, val reproduciendo: Boolean = false, val posMs: Long = 0, val durMs: Long = 0, val error: Boolean = false)

    val estado = MutableStateFlow(Estado())
    private var player: ExoPlayer? = null
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private var sondeo: Job? = null

    /** Empieza o pausa el audio `clave`; si es otro audio, cambia a ese. */
    fun alternar(ctx: Context, clave: String, archivo: File) {
        val p = player ?: crear(ctx.applicationContext).also { player = it }
        val actual = estado.value
        if (actual.clave == clave && p.playbackState != Player.STATE_IDLE) {
            if (p.isPlaying) p.pause() else {
                if (p.playbackState == Player.STATE_ENDED) p.seekTo(0)
                p.play()
            }
            return
        }
        p.stop()
        val fuente = ProgressiveMediaSource.Factory(FuenteCifrada.Fabrica(ClaveDatos.obtener(ctx))).createMediaSource(MediaItem.fromUri(Uri.fromFile(archivo)))
        p.setMediaSource(fuente)
        p.prepare()
        estado.value = Estado(clave = clave)
        p.play()
    }

    fun moverA(fraccion: Float) {
        val p = player ?: return
        val d = p.duration.takeIf { it > 0 } ?: return
        p.seekTo((d * fraccion).toLong())
        actualiza()
    }

    private fun crear(ctx: Context): ExoPlayer {
        val p = ExoPlayer.Builder(ctx).build()
        p.addListener(object : Player.Listener {
            override fun onIsPlayingChanged(isPlaying: Boolean) { actualiza(); if (isPlaying) sondear() }
            override fun onPlaybackStateChanged(state: Int) { actualiza() }
            override fun onPlayerError(error: PlaybackException) { estado.value = estado.value.copy(reproduciendo = false, error = true) }
        })
        return p
    }

    private fun sondear() {
        sondeo?.cancel()
        sondeo = scope.launch { while (isActive && player?.isPlaying == true) { actualiza(); delay(200) } }
    }

    private fun actualiza() {
        val p = player ?: return
        estado.value = estado.value.copy(reproduciendo = p.isPlaying, posMs = p.currentPosition, durMs = p.duration.coerceAtLeast(0), error = false)
    }

    /** Para el audio y libera el reproductor (al salir del chat o al bloquear). */
    fun detener() {
        sondeo?.cancel()
        player?.release()
        player = null
        estado.value = Estado()
    }
}
