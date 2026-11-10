package com.botchat.app.medios

import android.content.Context
import android.os.Handler
import android.os.Looper
import com.botchat.app.seguridad.ArchivoCifrado
import com.botchat.app.seguridad.ClaveDatos
import com.botchat.app.seguridad.FuenteArchivo
import java.io.File
import java.io.FileOutputStream
import java.util.UUID
import java.util.concurrent.ConcurrentHashMap

/**
 * Copias temporales descifradas (PDF y documentos que hay que abrir con otra app). Viven en cache/ (sin respaldo)
 * y se borran al cerrar el visor, al volver a la app, al bloquear, a los 10 minutos y al iniciar la app.
 */
object Temporales {
    private const val CARPETA = "tmp_descifrado"
    private const val VIDA_MS = 10 * 60 * 1000L
    private val manejador = Handler(Looper.getMainLooper())
    private val vivos = ConcurrentHashMap<File, Runnable>()

    fun carpeta(ctx: Context) = File(ctx.cacheDir, CARPETA)

    /** Nombre apto para archivo, sin rutas ni caracteres raros. */
    fun nombreSeguro(nombre: String, respaldo: String = "archivo"): String {
        val limpio = nombre.substringAfterLast('/').substringAfterLast('\\').map { if (it.isLetterOrDigit() || it in "._- ()") it else '_' }.joinToString("").trim().trim('.')
        return limpio.take(80).ifEmpty { respaldo }
    }

    /** Descifra `origen` a un archivo temporal con el nombre dado (en una subcarpeta propia). Se borra solo a los 10 minutos. */
    fun descifrar(ctx: Context, origen: File, nombre: String): File {
        val dir = File(carpeta(ctx), UUID.randomUUID().toString()).apply { mkdirs() }
        val destino = File(dir, nombreSeguro(nombre))
        ArchivoCifrado(FuenteArchivo(origen), ClaveDatos.obtener(ctx)).use { a ->
            FileOutputStream(destino).use { out ->
                val buf = ByteArray(64 * 1024)
                var pos = 0L
                while (pos < a.tam) {
                    val n = a.leer(pos, buf, 0, buf.size)
                    if (n <= 0) break
                    out.write(buf, 0, n); pos += n
                }
            }
        }
        val r = Runnable { borrar(destino) }
        vivos[destino] = r
        manejador.postDelayed(r, VIDA_MS)
        return destino
    }

    fun borrar(f: File) {
        vivos.remove(f)?.let { manejador.removeCallbacks(it) }
        runCatching { f.delete(); f.parentFile?.takeIf { it.listFiles().isNullOrEmpty() }?.delete() }
    }

    /** Borra todas las copias temporales (al bloquear o al volver a la app). */
    fun borrarTodo(ctx: Context) {
        vivos.values.forEach { manejador.removeCallbacks(it) }
        vivos.clear()
        runCatching { carpeta(ctx).deleteRecursively() }
    }

    /** Al iniciar la app: borra lo que haya quedado de la vez anterior. */
    fun barrer(ctx: Context) = borrarTodo(ctx)
}
