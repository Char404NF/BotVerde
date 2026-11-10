package com.botchat.app.medios

import android.content.Context
import android.graphics.Bitmap
import android.media.MediaDataSource
import android.media.MediaMetadataRetriever
import com.botchat.app.seguridad.ArchivoCifrado
import com.botchat.app.seguridad.ClaveDatos
import com.botchat.app.seguridad.FuenteArchivo
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.sync.Semaphore
import kotlinx.coroutines.sync.withPermit
import kotlinx.coroutines.withContext
import java.io.File
import kotlin.math.max

/**
 * Lectura aleatoria de un archivo cifrado para `MediaMetadataRetriever`: descifra solo los bloques que se piden,
 * así que sacar un cuadro de un video grande no lo descifra completo ni escribe nada en claro en disco.
 */
class OrigenCifrado(private val archivo: ArchivoCifrado) : MediaDataSource() {
    @Synchronized
    override fun readAt(position: Long, buffer: ByteArray, offset: Int, size: Int): Int {
        if (size == 0) return 0
        if (position >= archivo.tam) return -1
        val n = archivo.leer(position, buffer, offset, minOf(size.toLong(), archivo.tam - position).toInt())
        return if (n <= 0) -1 else n
    }

    override fun getSize(): Long = archivo.tam

    @Synchronized
    override fun close() = archivo.close()
}

/**
 * Portada de un video: un cuadro real del propio video (mucho más nítido que la miniatura de WhatsApp, que es diminuta).
 * Vive solo en la caché en memoria de imágenes y se vacía igual que ella.
 */
object PortadaVideo {
    private val limite = Semaphore(2) // sacar cuadros es pesado: de dos en dos

    suspend fun cargar(ctx: Context, archivo: File, ladoMax: Int, durMs: Long = 0): Bitmap? {
        val clave = "portada|${archivo.path}|$ladoMax"
        CacheImagenes.enCache(clave)?.let { return it }
        return withContext(Dispatchers.IO) {
            limite.withPermit {
                CacheImagenes.enCache(clave) ?: sacarCuadro(archivo, ClaveDatos.obtener(ctx), ladoMax, durMs)?.also { CacheImagenes.guardar(clave, it) }
            }
        }
    }

    internal fun sacarCuadro(archivo: File, clave: ByteArray, ladoMax: Int, durMs: Long): Bitmap? {
        if (!archivo.exists()) return null
        val r = MediaMetadataRetriever()
        return try {
            r.setDataSource(OrigenCifrado(ArchivoCifrado(FuenteArchivo(archivo), clave)))
            // El primer cuadro suele ser negro: se toma uno cerca del segundo 1 (o del 10 % si el video es muy corto).
            val real = r.extractMetadata(MediaMetadataRetriever.METADATA_KEY_DURATION)?.toLongOrNull() ?: durMs
            val t = if (real <= 0) 0L else minOf(1_000L, real / 10) * 1000
            val crudo = r.getFrameAtTime(t, MediaMetadataRetriever.OPTION_CLOSEST_SYNC) ?: r.getFrameAtTime(0) ?: return null
            val lado = max(crudo.width, crudo.height)
            if (lado <= ladoMax) crudo else {
                val k = ladoMax.toFloat() / lado
                Bitmap.createScaledBitmap(crudo, max(1, (crudo.width * k).toInt()), max(1, (crudo.height * k).toInt()), true).also { if (it !== crudo) crudo.recycle() }
            }
        } catch (e: Exception) {
            null
        } finally {
            runCatching { r.release() }
        }
    }
}
