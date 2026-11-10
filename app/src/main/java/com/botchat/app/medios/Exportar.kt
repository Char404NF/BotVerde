package com.botchat.app.medios

import android.content.ContentValues
import android.content.Context
import android.os.Build
import android.provider.MediaStore
import com.botchat.app.seguridad.ArchivoCifrado
import com.botchat.app.seguridad.ClaveDatos
import com.botchat.app.seguridad.FuenteArchivo
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.File

/**
 * "Guardar copia en mi carpeta": copia SIN cifrar a Documents/BotChat/<chat>/ con MediaStore (Android 10 o superior,
 * sin permisos extra). La copia ya no está protegida por BotChat.
 */
object Exportar {
    val disponible: Boolean get() = Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q

    suspend fun guardarCopia(ctx: Context, cifrado: File, nombreChat: String, nombreArchivo: String, mime: String): Result<String> = withContext(Dispatchers.IO) {
        runCatching {
            check(disponible) { "Esta versión de Android no permite guardar copias aquí" }
            val carpeta = "Documents/BotChat/" + Temporales.nombreSeguro(nombreChat, "chat")
            val valores = ContentValues().apply {
                put(MediaStore.MediaColumns.DISPLAY_NAME, Temporales.nombreSeguro(nombreArchivo))
                put(MediaStore.MediaColumns.MIME_TYPE, mime.ifBlank { "application/octet-stream" })
                put(MediaStore.MediaColumns.RELATIVE_PATH, carpeta)
                put(MediaStore.MediaColumns.IS_PENDING, 1)
            }
            val coleccion = MediaStore.Files.getContentUri(MediaStore.VOLUME_EXTERNAL_PRIMARY)
            val uri = ctx.contentResolver.insert(coleccion, valores) ?: error("No se pudo crear el archivo")
            try {
                ctx.contentResolver.openOutputStream(uri)!!.use { out ->
                    ArchivoCifrado(FuenteArchivo(cifrado), ClaveDatos.obtener(ctx)).use { a ->
                        val buf = ByteArray(64 * 1024)
                        var pos = 0L
                        while (pos < a.tam) {
                            val n = a.leer(pos, buf, 0, buf.size)
                            if (n <= 0) break
                            out.write(buf, 0, n); pos += n
                        }
                    }
                }
                ctx.contentResolver.update(uri, ContentValues().apply { put(MediaStore.MediaColumns.IS_PENDING, 0) }, null, null)
            } catch (e: Exception) {
                ctx.contentResolver.delete(uri, null, null)
                throw e
            }
            carpeta
        }
    }
}
