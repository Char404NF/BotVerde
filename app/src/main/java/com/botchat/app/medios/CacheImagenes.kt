package com.botchat.app.medios

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.util.LruCache
import com.botchat.app.seguridad.ArchivoCifrado
import com.botchat.app.seguridad.ClaveDatos
import com.botchat.app.seguridad.FuenteArchivo
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File
import kotlin.math.max

/**
 * Caché de imágenes ya descifradas y reducidas: solo en memoria, nunca en disco, de unos 30 MB como máximo.
 * Se vacía al bloquear la app o al salir del chat. Todo el trabajo (leer, descifrar, decodificar) ocurre fuera del hilo principal.
 */
object CacheImagenes {
    private const val MAXIMO = 30 * 1024 * 1024
    private val cache = object : LruCache<String, Bitmap>(MAXIMO) {
        override fun sizeOf(key: String, value: Bitmap) = value.allocationByteCount
    }

    fun vaciar() = cache.evictAll()

    fun enCache(clave: String): Bitmap? = cache.get(clave)

    /**
     * Devuelve la imagen del archivo cifrado `archivo`, reducida para que su lado mayor no pase de `ladoMax` píxeles.
     * `etiqueta` distingue versiones del mismo archivo (por ejemplo una foto de perfil que cambió).
     */
    suspend fun cargar(ctx: Context, archivo: File, ladoMax: Int, etiqueta: String = ""): Bitmap? {
        val clave = "${archivo.path}|$ladoMax|$etiqueta"
        cache.get(clave)?.let { return it }
        return withContext(Dispatchers.Default) {
            cache.get(clave)?.let { return@withContext it }
            val bmp = runCatching { decodificar(archivo, ClaveDatos.obtener(ctx), ladoMax) }.getOrNull()
            if (bmp != null) cache.put(clave, bmp)
            bmp
        }
    }

    internal fun decodificar(archivo: File, clave: ByteArray, ladoMax: Int): Bitmap? {
        if (!archivo.exists()) return null
        val bytes = ArchivoCifrado(FuenteArchivo(archivo), clave).use { it.leerTodo() }
        val medidas = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeByteArray(bytes, 0, bytes.size, medidas)
        if (medidas.outWidth <= 0 || medidas.outHeight <= 0) return null
        val opciones = BitmapFactory.Options().apply { inSampleSize = muestreo(max(medidas.outWidth, medidas.outHeight), ladoMax) }
        return BitmapFactory.decodeByteArray(bytes, 0, bytes.size, opciones)
    }

    /** Potencia de 2 más grande que deja la imagen todavía con al menos `ladoMax` píxeles de lado mayor. */
    fun muestreo(ladoReal: Int, ladoMax: Int): Int {
        var m = 1
        while (ladoMax > 0 && ladoReal / (m * 2) >= ladoMax) m *= 2
        return m
    }
}
