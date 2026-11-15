package com.botchat.app.descargas

import android.media.MediaCodec
import android.media.MediaExtractor
import android.media.MediaFormat
import android.media.MediaMuxer
import java.io.File
import java.nio.ByteBuffer
import java.util.zip.Deflater
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream

/**
 * Lo que se hace con los archivos ya bajados, solo con las herramientas de Android (sin ffmpeg): unir imagen y sonido en un mp4,
 * partir un mp4 o un m4a en pedazos que se pueden reproducir solos, y armar un zip.
 */
object ArchivosVideo {
    private const val BUFER = 4 * 1024 * 1024

    private fun pista(e: MediaExtractor, prefijo: String): Int {
        for (i in 0 until e.trackCount) if (e.getTrackFormat(i).getString(MediaFormat.KEY_MIME)?.startsWith(prefijo) == true) return i
        return -1
    }

    /** Une un archivo de imagen (H.264) y uno de sonido (AAC) en un mp4. */
    fun unir(video: File, audio: File, salida: File) {
        val ev = MediaExtractor().apply { setDataSource(video.path) }
        val ea = MediaExtractor().apply { setDataSource(audio.path) }
        try {
            val pv = pista(ev, "video/"); val pa = pista(ea, "audio/")
            require(pv >= 0 && pa >= 0) { "No encontré la imagen o el sonido para unirlos" }
            ev.selectTrack(pv); ea.selectTrack(pa)
            val m = MediaMuxer(salida.path, MediaMuxer.OutputFormat.MUXER_OUTPUT_MPEG_4)
            try {
                val tv = m.addTrack(ev.getTrackFormat(pv)); val ta = m.addTrack(ea.getTrackFormat(pa))
                m.start()
                val buf = ByteBuffer.allocateDirect(BUFER); val info = MediaCodec.BufferInfo()
                var vivo = true; var avivo = true
                while (vivo || avivo) {
                    val usaVideo = vivo && (!avivo || ev.sampleTime <= ea.sampleTime)
                    val e = if (usaVideo) ev else ea
                    val n = e.readSampleData(buf, 0)
                    if (n < 0) { if (usaVideo) vivo = false else avivo = false; continue }
                    info.set(0, n, e.sampleTime, if (e.sampleFlags and MediaExtractor.SAMPLE_FLAG_SYNC != 0) MediaCodec.BUFFER_FLAG_KEY_FRAME else 0)
                    m.writeSampleData(if (usaVideo) tv else ta, buf, info)
                    e.advance()
                    if (e.sampleTime < 0) { if (usaVideo) vivo = false else avivo = false }
                }
            } finally { runCatching { m.stop() }; m.release() }
        } finally { ev.release(); ea.release() }
    }

    /** Duración en segundos de un mp4 o m4a (0 si no se sabe). */
    fun duracionS(f: File): Int {
        val e = MediaExtractor()
        return try {
            e.setDataSource(f.path)
            var us = 0L
            for (i in 0 until e.trackCount) us = maxOf(us, e.getTrackFormat(i).takeIf { it.containsKey(MediaFormat.KEY_DURATION) }?.getLong(MediaFormat.KEY_DURATION) ?: 0L)
            (us / 1_000_000L).toInt()
        } catch (_: Exception) { 0 } finally { e.release() }
    }

    /**
     * Parte un mp4 (o m4a) en `n` pedazos de duración parecida. Cada pedazo empieza en un cuadro clave, así que se reproduce solo.
     * Devuelve los archivos en orden (parte 1, parte 2…).
     */
    fun partir(entrada: File, n: Int, carpeta: File, base: String): List<File> {
        if (n <= 1) return listOf(entrada)
        val partes = partirSinRevisar(entrada, n, carpeta, base)
        // cada parte tiene que abrirse y traer imagen o sonido con duración: si alguna no sirve, mejor avisar que mandar un archivo roto
        for (p in partes) check(sirve(p)) { "una de las partes no quedó bien" }
        return partes
    }

    /** ¿El archivo se abre y trae al menos una pista de video o audio con duración? */
    fun sirve(f: File): Boolean {
        val e = MediaExtractor()
        return try {
            e.setDataSource(f.path)
            var ok = false
            for (i in 0 until e.trackCount) {
                val m = e.getTrackFormat(i).getString(MediaFormat.KEY_MIME) ?: continue
                if (m.startsWith("video/") || m.startsWith("audio/")) ok = true
            }
            ok && f.length() > 1024
        } catch (_: Exception) { false } finally { e.release() }
    }

    private fun partirSinRevisar(entrada: File, n: Int, carpeta: File, base: String): List<File> {
        partirFragmentado(entrada, n, carpeta, base)?.let { return it }
        val total = duracionS(entrada) * 1_000_000L
        require(total > 0) { "No pude medir la duración para partirlo" }
        val porParte = total / n
        val salidas = mutableListOf<File>()
        var inicio = 0L
        for (k in 0 until n) {
            val destino = File(carpeta, "%s_parte%02d.%s".format(base, k + 1, entrada.extension.ifEmpty { "mp4" }))
            val fin = if (k == n - 1) Long.MAX_VALUE else (k + 1) * porParte
            val siguiente = copiarTramo(entrada, destino, inicio, fin)
            salidas += destino
            if (siguiente < 0) break
            inicio = siguiente
        }
        return salidas
    }

    /** Copia desde el cuadro clave anterior a `inicioUs` hasta el primer cuadro clave de video a partir de `finUs`. Devuelve dónde empieza el siguiente (-1 si ya acabó). */
    private fun copiarTramo(entrada: File, destino: File, inicioUs: Long, finUs: Long): Long {
        val e = MediaExtractor().apply { setDataSource(entrada.path) }
        try {
            val salida = if (entrada.extension.equals("m4a", true) || entrada.extension.equals("mp3", true)) MediaMuxer.OutputFormat.MUXER_OUTPUT_MPEG_4 else MediaMuxer.OutputFormat.MUXER_OUTPUT_MPEG_4
            val m = MediaMuxer(destino.path, salida)
            try {
                val mapa = HashMap<Int, Int>()
                var pistaVideo = -1
                for (i in 0 until e.trackCount) {
                    val f = e.getTrackFormat(i); val mime = f.getString(MediaFormat.KEY_MIME) ?: continue
                    if (!mime.startsWith("video/") && !mime.startsWith("audio/")) continue
                    e.selectTrack(i); mapa[i] = m.addTrack(f)
                    if (mime.startsWith("video/")) pistaVideo = i
                }
                m.start()
                if (inicioUs > 0) e.seekTo(inicioUs, MediaExtractor.SEEK_TO_CLOSEST_SYNC)
                val buf = ByteBuffer.allocateDirect(BUFER); val info = MediaCodec.BufferInfo()
                var base = -1L; var siguiente = -1L
                while (true) {
                    val n = e.readSampleData(buf, 0)
                    if (n < 0) break
                    val t = e.sampleTime; val pista = e.sampleTrackIndex
                    val clave = e.sampleFlags and MediaExtractor.SAMPLE_FLAG_SYNC != 0
                    val cortaAqui = t >= finUs && clave && (pista == pistaVideo || pistaVideo < 0)
                    if (cortaAqui) { siguiente = t; break }
                    if (base < 0) base = t
                    info.set(0, n, maxOf(0L, t - base), if (clave) MediaCodec.BUFFER_FLAG_KEY_FRAME else 0)
                    m.writeSampleData(mapa[pista] ?: break, buf, info)
                    e.advance()
                }
                return siguiente
            } finally { runCatching { m.stop() }; m.release() }
        } finally { e.release() }
    }

    private class Caja(val tipo: String, val inicio: Long, val largo: Long)

    private fun leerCajas(f: File): List<Caja>? {
        val cajas = mutableListOf<Caja>()
        java.io.RandomAccessFile(f, "r").use { r ->
            var pos = 0L
            val tam = r.length()
            while (pos + 8 <= tam) {
                r.seek(pos)
                var largo = r.readInt().toLong() and 0xFFFFFFFFL
                val tipo = ByteArray(4).also { r.readFully(it) }.toString(Charsets.ISO_8859_1)
                if (largo == 1L) largo = r.readLong() else if (largo == 0L) largo = tam - pos
                if (largo < 8 || pos + largo > tam) return null
                cajas += Caja(tipo, pos, largo)
                pos += largo
            }
        }
        return cajas
    }

    /**
     * Archivos "por fragmentos" (los que baja yt-dlp de muchas fuentes): una cabecera (ftyp + moov) y luego pares moof + mdat. Se parten
     * entre pares y a cada pedazo se le pone la cabecera, así cada uno se abre solo. Devuelve null si el archivo no es de ese tipo.
     */
    private fun partirFragmentado(entrada: File, n: Int, carpeta: File, base: String): List<File>? {
        val cajas = leerCajas(entrada) ?: return null
        if (cajas.none { it.tipo == "moof" }) return null
        val cabecera = cajas.filter { it.tipo == "ftyp" || it.tipo == "moov" }
        if (cabecera.none { it.tipo == "moov" }) return null
        val frag = mutableListOf<Pair<Long, Long>>() // inicio y largo de cada grupo moof+mdat (y sidx/styp sueltos van con su moof)
        var ini = -1L; var largo = 0L
        for (c in cajas) {
            if (c.tipo == "ftyp" || c.tipo == "moov") continue
            if (c.tipo == "moof" || c.tipo == "styp" || c.tipo == "sidx") {
                if (ini >= 0 && c.tipo == "moof" && largo > 0 && lastTipoFueMdat) { frag += ini to largo; ini = -1; largo = 0 }
            }
            if (ini < 0) ini = c.inicio
            largo += c.largo
            lastTipoFueMdat = c.tipo == "mdat"
        }
        if (ini >= 0) frag += ini to largo
        if (frag.size < n) return null
        val bytesCab = cabecera.sumOf { it.largo }
        val util = frag.sumOf { it.second }
        val porParte = util / n
        val salidas = mutableListOf<File>()
        var i = 0
        for (k in 0 until n) {
            val destino = File(carpeta, "%s_parte%02d.%s".format(base, k + 1, entrada.extension.ifEmpty { "mp4" }))
            var acumulado = 0L
            java.io.RandomAccessFile(entrada, "r").use { r ->
                destino.outputStream().buffered().use { o ->
                    for (c in cabecera) copiar(r, o, c.inicio, c.largo)
                    while (i < frag.size && (acumulado < porParte || k == n - 1)) { copiar(r, o, frag[i].first, frag[i].second); acumulado += frag[i].second; i++ }
                }
            }
            salidas += destino
            if (i >= frag.size) break
        }
        check(bytesCab > 0)
        return salidas
    }

    private var lastTipoFueMdat = false

    private fun copiar(r: java.io.RandomAccessFile, o: java.io.OutputStream, desde: Long, largo: Long) {
        r.seek(desde)
        val b = ByteArray(256 * 1024)
        var falta = largo
        while (falta > 0) {
            val n = r.read(b, 0, minOf(b.size.toLong(), falta).toInt())
            if (n < 0) break
            o.write(b, 0, n); falta -= n
        }
    }

    /** Mete los archivos en un zip sin comprimir de más (el video ya viene comprimido). */
    fun zip(archivos: List<File>, salida: File) {
        ZipOutputStream(salida.outputStream().buffered()).use { z ->
            z.setLevel(Deflater.NO_COMPRESSION)
            for (f in archivos) { z.putNextEntry(ZipEntry(f.name)); f.inputStream().use { it.copyTo(z) }; z.closeEntry() }
        }
    }
}
