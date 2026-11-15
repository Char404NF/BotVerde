package com.botchat.app.descargas

import android.content.Context
import org.json.JSONArray
import java.io.File

/**
 * El motor de descargas que ve el módulo Go: Python con el yt-dlp que el dueño descargó (la app no lo trae) y las herramientas de Android para unir, partir y comprimir.
 * Registrarlo no gasta nada: Python solo arranca cuando alguien pide una descarga.
 */
class MotorVideoAndroid(private val ctx: Context) : wabot.MotorVideo {
    private val raiz get() = File(ctx.filesDir, "ytdlp").apply { mkdirs() }

    override fun instalado(): Boolean = MotorDescargas.instalado(ctx)

    override fun quitar() {
        MotorDescargas.soltarYtdlp(ctx)
        File(raiz, "yt-dlp").delete()
        File(raiz, "yt-dlp.previo").delete()
        File(raiz, "yt-dlp.nuevo").delete()
    }

    override fun version(): String = MotorDescargas.version(ctx)

    override fun info(url: String): String = MotorDescargas.info(ctx, url)

    override fun descargar(url: String, carpeta: String, modo: String, altura: Long, abr: Long): String =
        MotorDescargas.descargar(ctx, url, File(carpeta), modo, altura.toInt(), abr.toInt(), null)

    override fun convertir(ruta: String, formato: String): String = ConvertirAudio.convertir(File(ruta), formato).path

    override fun partir(ruta: String, n: Long, tope: Long, carpeta: String, base: String): String {
        val dir = File(carpeta).apply { mkdirs() }
        val partes = ArchivosVideo.partir(File(ruta), n.toInt(), tope, dir, base)
        return JSONArray(partes.map { it.path }).toString()
    }

    override fun zip(rutasJSON: String, salida: String) {
        val a = JSONArray(rutasJSON)
        ArchivosVideo.zip((0 until a.length()).map { File(a.getString(it)) }, File(salida))
    }

    override fun instalar(ruta: String): String {
        val nuevo = File(ruta)
        require(nuevo.isFile) { "no encuentro el archivo descargado" }
        val actual = File(raiz, "yt-dlp"); val previo = File(raiz, "yt-dlp.previo")
        previo.delete()
        val habia = actual.isFile
        if (habia) actual.renameTo(previo)
        if (!nuevo.renameTo(actual)) { nuevo.copyTo(actual, true); nuevo.delete() }
        return try {
            MotorDescargas.usarYtdlp(ctx, actual).also { previo.delete() }
        } catch (e: Throwable) {
            actual.delete()
            if (habia && previo.isFile) { previo.renameTo(actual); runCatching { MotorDescargas.usarYtdlp(ctx, actual) } }
            else MotorDescargas.soltarYtdlp(ctx)
            throw e
        }
    }

    override fun carpeta(): String = raiz.path

    override fun borrar(ruta: String) { File(ruta).deleteRecursively() }
}
