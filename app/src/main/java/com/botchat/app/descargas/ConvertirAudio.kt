package com.botchat.app.descargas

import android.content.Context
import android.media.MediaCodec
import android.media.MediaExtractor
import android.media.MediaFormat
import java.io.File
import java.io.RandomAccessFile
import java.nio.ByteBuffer
import java.nio.ByteOrder

/**
 * Pasa un audio (m4a/AAC) a mp3: Android lo decodifica a sonido sin comprimir y LAME (un codificador libre que viaja dentro de la app como
 * programa aparte) lo comprime. Todo en el teléfono; nada sale de él.
 */
object ConvertirAudio {
    @Volatile var contexto: Context? = null

    /** `formato` es "mp3" o "mp3:160" (kbps). Devuelve el archivo nuevo, junto al original. */
    fun convertir(entrada: File, formato: String): File {
        require(formato.startsWith("mp3")) { "Solo sé pasar a mp3" }
        val kbps = formato.substringAfter(':', "").toIntOrNull()?.coerceIn(64, 320) ?: 160
        val ctx = contexto ?: throw IllegalStateException("la app no está lista")
        val lame = File(ctx.applicationInfo.nativeLibraryDir, "liblame.so")
        check(lame.canExecute()) { "no encuentro el codificador de mp3" }
        val wav = File(entrada.parentFile, entrada.nameWithoutExtension + "_tmp.wav")
        val salida = File(entrada.parentFile, entrada.nameWithoutExtension + ".mp3")
        try {
            aWav(entrada, wav)
            val p = ProcessBuilder(lame.path, "--silent", "-b", kbps.toString(), wav.path, salida.path).redirectErrorStream(true).start()
            val texto = p.inputStream.bufferedReader().readText()
            check(p.waitFor() == 0 && salida.length() > 0) { "el codificador de mp3 falló: ${texto.take(120)}" }
        } finally { wav.delete() }
        return salida
    }

    /** Decodifica con MediaCodec y escribe un WAV de 16 bits. */
    private fun aWav(entrada: File, wav: File) {
        val e = MediaExtractor().apply { setDataSource(entrada.path) }
        var codec: MediaCodec? = null
        try {
            var pista = -1
            for (i in 0 until e.trackCount) if (e.getTrackFormat(i).getString(MediaFormat.KEY_MIME)?.startsWith("audio/") == true) { pista = i; break }
            require(pista >= 0) { "no encontré el audio" }
            e.selectTrack(pista)
            val formato = e.getTrackFormat(pista)
            codec = MediaCodec.createDecoderByType(formato.getString(MediaFormat.KEY_MIME)!!)
            codec.configure(formato, null, null, 0)
            codec.start()
            var canales = formato.getInteger(MediaFormat.KEY_CHANNEL_COUNT)
            var hz = formato.getInteger(MediaFormat.KEY_SAMPLE_RATE)
            RandomAccessFile(wav, "rw").use { f ->
                f.setLength(0); f.write(ByteArray(44))
                val info = MediaCodec.BufferInfo()
                var finEntrada = false; var finSalida = false
                var bytes = 0L
                while (!finSalida) {
                    if (!finEntrada) {
                        val i = codec.dequeueInputBuffer(10_000)
                        if (i >= 0) {
                            val b = codec.getInputBuffer(i)!!
                            val n = e.readSampleData(b, 0)
                            if (n < 0) { codec.queueInputBuffer(i, 0, 0, 0, MediaCodec.BUFFER_FLAG_END_OF_STREAM); finEntrada = true }
                            else { codec.queueInputBuffer(i, 0, n, e.sampleTime, 0); e.advance() }
                        }
                    }
                    val o = codec.dequeueOutputBuffer(info, 10_000)
                    when {
                        o == MediaCodec.INFO_OUTPUT_FORMAT_CHANGED -> {
                            val nf = codec.outputFormat
                            canales = nf.getInteger(MediaFormat.KEY_CHANNEL_COUNT); hz = nf.getInteger(MediaFormat.KEY_SAMPLE_RATE)
                        }
                        o >= 0 -> {
                            val b = codec.getOutputBuffer(o)!!
                            if (info.size > 0) {
                                val arr = ByteArray(info.size)
                                b.position(info.offset); b.get(arr)
                                f.write(arr); bytes += arr.size
                            }
                            codec.releaseOutputBuffer(o, false)
                            if (info.flags and MediaCodec.BUFFER_FLAG_END_OF_STREAM != 0) finSalida = true
                        }
                    }
                }
                val h = ByteBuffer.allocate(44).order(ByteOrder.LITTLE_ENDIAN)
                h.put("RIFF".toByteArray()).putInt((36 + bytes).toInt()).put("WAVE".toByteArray()).put("fmt ".toByteArray()).putInt(16)
                    .putShort(1).putShort(canales.toShort()).putInt(hz).putInt(hz * canales * 2).putShort((canales * 2).toShort()).putShort(16)
                    .put("data".toByteArray()).putInt(bytes.toInt())
                f.seek(0); f.write(h.array())
            }
        } finally { runCatching { codec?.stop() }; codec?.release(); e.release() }
    }
}
