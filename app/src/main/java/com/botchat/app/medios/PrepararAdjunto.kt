package com.botchat.app.medios

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Matrix
import android.media.ExifInterface
import android.media.MediaMetadataRetriever
import android.net.Uri
import android.provider.OpenableColumns
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.ByteArrayOutputStream
import java.io.File
import kotlin.math.max

/** Un archivo listo para mandar: copia temporal en claro (se borra al enviar o cancelar) y datos para la vista previa. */
class AdjuntoListo(
    val tipo: String, // imagen, video, audio o documento
    val archivo: File,
    val mime: String,
    val nombre: String,
    val ancho: Int,
    val alto: Int,
    val durS: Int,
    val miniatura: ByteArray?,
    val previa: Bitmap?,
) {
    val bytes: Long get() = archivo.length()
}

/** Prepara lo que se elige en la galería, la cámara o los archivos: copia, reduce las fotos y saca miniatura, duración y medidas. */
object PrepararAdjunto {
    private const val LADO_FOTO = 1600
    private const val LADO_MINIATURA = 100
    private const val LADO_PREVIA = 360

    fun carpeta(ctx: Context): File = File(ctx.cacheDir, "adjuntos").apply { mkdirs() }

    /** Borra cualquier copia que haya quedado de una vez anterior. */
    fun limpiar(ctx: Context) { runCatching { carpeta(ctx).listFiles()?.forEach { it.deleteRecursively() } } }

    private fun nombreDe(ctx: Context, uri: Uri): String? = runCatching {
        ctx.contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { c -> if (c.moveToFirst()) c.getString(0) else null }
    }.getOrNull()

    private fun seguro(n: String) = n.replace(Regex("[^A-Za-z0-9._ -]"), "_").take(80).ifBlank { "archivo" }

    /** Copia lo elegido a una carpeta temporal y lo prepara según su tipo. */
    suspend fun desdeUri(ctx: Context, uri: Uri): Result<AdjuntoListo> = withContext(Dispatchers.IO) {
        runCatching {
            val nombre = nombreDe(ctx, uri) ?: uri.lastPathSegment ?: "archivo"
            val mime = ctx.contentResolver.getType(uri) ?: TipoArchivo.mimeEfectivo(nombre, "")
            val destino = File(carpeta(ctx), "${System.nanoTime()}_${seguro(nombre)}")
            ctx.contentResolver.openInputStream(uri)?.use { entrada -> destino.outputStream().use { entrada.copyTo(it) } } ?: error("No se pudo leer el archivo")
            prepara(destino, nombre, mime)
        }
    }

    /** Para lo que tomó la cámara: el archivo ya está en la carpeta temporal. */
    suspend fun desdeArchivo(archivo: File, mime: String): Result<AdjuntoListo> = withContext(Dispatchers.IO) { runCatching { prepara(archivo, archivo.name, mime) } }

    private fun prepara(archivo: File, nombre: String, mime: String): AdjuntoListo {
        if (archivo.length() == 0L) { archivo.delete(); error("El archivo está vacío") }
        return when {
            mime.startsWith("image/") -> foto(archivo, nombre) ?: documento(archivo, nombre, mime)
            mime.startsWith("video/") -> video(archivo, nombre, mime)
            mime.startsWith("audio/") -> audio(archivo, nombre, mime)
            else -> documento(archivo, nombre, mime)
        }
    }

    private fun documento(archivo: File, nombre: String, mime: String) =
        AdjuntoListo("documento", archivo, TipoArchivo.mimeEfectivo(nombre, mime), TipoArchivo.nombreMostrable(nombre, mime), 0, 0, 0, null, null)

    private fun jpeg(b: Bitmap, calidad: Int): ByteArray = ByteArrayOutputStream().also { b.compress(Bitmap.CompressFormat.JPEG, calidad, it) }.toByteArray()

    private fun reducido(b: Bitmap, lado: Int): Bitmap {
        val m = max(b.width, b.height)
        if (m <= lado) return b
        val k = lado.toFloat() / m
        return Bitmap.createScaledBitmap(b, (b.width * k).toInt().coerceAtLeast(1), (b.height * k).toInt().coerceAtLeast(1), true)
    }

    private fun foto(archivo: File, nombre: String): AdjuntoListo? {
        val medidas = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeFile(archivo.absolutePath, medidas)
        if (medidas.outWidth <= 0 || medidas.outHeight <= 0) return null
        val op = BitmapFactory.Options().apply { inSampleSize = CacheImagenes.muestreo(max(medidas.outWidth, medidas.outHeight), LADO_FOTO) }
        var bmp = BitmapFactory.decodeFile(archivo.absolutePath, op) ?: return null
        val giro = when (runCatching { ExifInterface(archivo.absolutePath).getAttributeInt(ExifInterface.TAG_ORIENTATION, 1) }.getOrDefault(1)) {
            ExifInterface.ORIENTATION_ROTATE_90 -> 90f
            ExifInterface.ORIENTATION_ROTATE_180 -> 180f
            ExifInterface.ORIENTATION_ROTATE_270 -> 270f
            else -> 0f
        }
        if (giro != 0f) bmp = Bitmap.createBitmap(bmp, 0, 0, bmp.width, bmp.height, Matrix().apply { postRotate(giro) }, true)
        bmp = reducido(bmp, LADO_FOTO)
        val salida = File(archivo.parentFile, archivo.nameWithoutExtension + "_w.jpg")
        salida.writeBytes(jpeg(bmp, 82))
        archivo.delete()
        return AdjuntoListo("imagen", salida, "image/jpeg", nombre.substringBeforeLast('.') + ".jpg", bmp.width, bmp.height, 0, jpeg(reducido(bmp, LADO_MINIATURA), 60), reducido(bmp, LADO_PREVIA))
    }

    private fun video(archivo: File, nombre: String, mime: String): AdjuntoListo {
        val r = MediaMetadataRetriever()
        try {
            r.setDataSource(archivo.absolutePath)
            val dur = (r.extractMetadata(MediaMetadataRetriever.METADATA_KEY_DURATION)?.toLongOrNull() ?: 0L) / 1000
            var w = r.extractMetadata(MediaMetadataRetriever.METADATA_KEY_VIDEO_WIDTH)?.toIntOrNull() ?: 0
            var h = r.extractMetadata(MediaMetadataRetriever.METADATA_KEY_VIDEO_HEIGHT)?.toIntOrNull() ?: 0
            val giro = r.extractMetadata(MediaMetadataRetriever.METADATA_KEY_VIDEO_ROTATION)?.toIntOrNull() ?: 0
            if (giro == 90 || giro == 270) { val t = w; w = h; h = t }
            val cuadro = runCatching { r.getFrameAtTime(1_000_000L.coerceAtMost(dur * 500_000L), MediaMetadataRetriever.OPTION_CLOSEST_SYNC) }.getOrNull()
            return AdjuntoListo("video", archivo, if (mime.startsWith("video/")) mime else "video/mp4", nombre, w, h, dur.toInt(), cuadro?.let { jpeg(reducido(it, LADO_MINIATURA), 60) }, cuadro?.let { reducido(it, LADO_PREVIA) })
        } finally {
            runCatching { r.release() }
        }
    }

    private fun audio(archivo: File, nombre: String, mime: String): AdjuntoListo {
        val r = MediaMetadataRetriever()
        val dur = try {
            r.setDataSource(archivo.absolutePath)
            (r.extractMetadata(MediaMetadataRetriever.METADATA_KEY_DURATION)?.toLongOrNull() ?: 0L) / 1000
        } catch (_: Exception) { 0L } finally { runCatching { r.release() } }
        return AdjuntoListo("audio", archivo, mime, nombre, 0, 0, dur.toInt(), null, null)
    }
}
