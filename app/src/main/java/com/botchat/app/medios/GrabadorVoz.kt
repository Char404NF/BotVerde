package com.botchat.app.medios

import android.content.Context
import android.media.MediaRecorder
import android.os.Build
import java.io.File

/** Una nota de voz grabada. `esNota` es false cuando el teléfono solo puede grabar AAC (Android 8 y 9): sale como audio normal. */
class GrabacionVoz(val archivo: File, val durS: Int, val onda: ByteArray, val esNota: Boolean, val mime: String)

/** Graba notas de voz. Existe como interfaz para poder probar la pantalla sin micrófono. */
interface GrabadorVoz {
    fun iniciar(): Boolean
    /** Devuelve la grabación, o null si fue muy corta o falló. */
    fun terminar(): GrabacionVoz?
    fun cancelar()
}

object Grabadores {
    /** Las pruebas cambian esto por un grabador de mentira. */
    @Volatile var fabrica: (Context) -> GrabadorVoz = { GrabadorMediaRecorder(it) }
}

/** Con Android 10 o más se graba en Opus dentro de OGG, que es lo que WhatsApp reconoce como nota de voz. */
class GrabadorMediaRecorder(private val ctx: Context) : GrabadorVoz {
    private var rec: MediaRecorder? = null
    private var archivo: File? = null
    private var inicio = 0L
    private val muestras = ArrayList<Int>()
    private var sondeo: Thread? = null
    @Volatile private var activo = false
    private val opus = Build.VERSION.SDK_INT >= 29

    @Suppress("DEPRECATION")
    override fun iniciar(): Boolean {
        val f = File(PrepararAdjunto.carpeta(ctx), "voz_${System.nanoTime()}." + if (opus) "ogg" else "m4a")
        return try {
            val r = if (Build.VERSION.SDK_INT >= 31) MediaRecorder(ctx) else MediaRecorder()
            r.setAudioSource(MediaRecorder.AudioSource.MIC)
            if (opus) {
                r.setOutputFormat(MediaRecorder.OutputFormat.OGG)
                r.setAudioEncoder(MediaRecorder.AudioEncoder.OPUS)
                r.setAudioEncodingBitRate(32_000)
                r.setAudioSamplingRate(16_000)
            } else {
                r.setOutputFormat(MediaRecorder.OutputFormat.MPEG_4)
                r.setAudioEncoder(MediaRecorder.AudioEncoder.AAC)
                r.setAudioEncodingBitRate(48_000)
                r.setAudioSamplingRate(22_050)
            }
            r.setAudioChannels(1)
            r.setMaxDuration(10 * 60 * 1000)
            r.setOutputFile(f.absolutePath)
            r.prepare()
            r.start()
            rec = r; archivo = f; inicio = System.currentTimeMillis(); activo = true
            sondeo = Thread {
                while (activo) {
                    runCatching { muestras += rec?.maxAmplitude ?: 0 }
                    try { Thread.sleep(100) } catch (_: InterruptedException) { break }
                }
            }.also { it.start() }
            true
        } catch (_: Exception) {
            cancelar()
            false
        }
    }

    override fun terminar(): GrabacionVoz? {
        val f = archivo ?: return null
        val ms = System.currentTimeMillis() - inicio
        activo = false
        sondeo?.interrupt()
        val ok = runCatching { rec?.stop(); true }.getOrDefault(false)
        runCatching { rec?.release() }
        rec = null; archivo = null
        if (!ok || ms < 1000 || !f.exists() || f.length() == 0L) { f.delete(); return null }
        return GrabacionVoz(f, ((ms + 500) / 1000).toInt(), onda(), opus, if (opus) "audio/ogg; codecs=opus" else "audio/mp4")
    }

    override fun cancelar() {
        activo = false
        sondeo?.interrupt()
        runCatching { rec?.stop() }
        runCatching { rec?.release() }
        rec = null
        archivo?.delete(); archivo = null
    }

    /** 64 valores de 0 a 100 que dibujan la forma de la nota. */
    private fun onda(): ByteArray {
        val m = muestras.toList()
        if (m.isEmpty()) return ByteArray(64) { 10 }
        val maximo = (m.maxOrNull() ?: 1).coerceAtLeast(1)
        return ByteArray(64) { i -> (m[(i * m.size / 64).coerceIn(0, m.size - 1)] * 100L / maximo).toInt().coerceIn(3, 100).toByte() }
    }
}
