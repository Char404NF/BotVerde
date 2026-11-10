package com.botchat.app.medios

import android.content.Context
import android.graphics.BitmapFactory
import android.graphics.ImageDecoder
import android.graphics.drawable.BitmapDrawable
import android.graphics.drawable.Drawable
import android.os.Build
import com.botchat.app.seguridad.ArchivoCifrado
import com.botchat.app.seguridad.ClaveDatos
import com.botchat.app.seguridad.FuenteArchivo
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File
import java.nio.ByteBuffer

/** Stickers (también los animados, que son WebP animado). Se descifran en memoria y nunca se escriben en claro. */
object Stickers {
    /** Con Android 9 o más se anima; en versiones anteriores se ve el primer cuadro. null si el archivo no se puede abrir. */
    suspend fun cargar(ctx: Context, archivo: File): Drawable? = withContext(Dispatchers.Default) {
        runCatching {
            if (!archivo.exists()) return@runCatching null
            val bytes = ArchivoCifrado(FuenteArchivo(archivo), ClaveDatos.obtener(ctx)).use { it.leerTodo() }
            val animado = if (Build.VERSION.SDK_INT >= 28) runCatching { ImageDecoder.decodeDrawable(ImageDecoder.createSource(ByteBuffer.wrap(bytes))) }.getOrNull() else null
            animado ?: BitmapFactory.decodeByteArray(bytes, 0, bytes.size)?.let { BitmapDrawable(ctx.resources, it) }
        }.getOrNull()
    }
}
