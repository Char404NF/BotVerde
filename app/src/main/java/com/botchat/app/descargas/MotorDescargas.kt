package com.botchat.app.descargas

import android.content.Context
import com.chaquo.python.PyObject
import com.chaquo.python.Python
import com.chaquo.python.android.AndroidPlatform
import org.json.JSONObject
import java.io.File

/**
 * Python embebido con yt-dlp. No arranca nada hasta que alguien pide una descarga (y sigue vivo solo mientras la app lo está).
 * Todo entra y sale como JSON de texto, para poder pasarlo tal cual al motor Go.
 */
object MotorDescargas {
    /** Lo que llama Python mientras baja (porcentaje y qué parte: "v" imagen, "a" sonido). */
    fun interface Progreso { fun avance(pct: Int, parte: String) }

    @Synchronized
    private fun py(ctx: Context): Python {
        if (!Python.isStarted()) Python.start(AndroidPlatform(ctx.applicationContext))
        return Python.getInstance()
    }

    private fun modulo(ctx: Context): PyObject {
        val m = py(ctx).getModule("descargas_py")
        // si hay un yt-dlp descargado (más nuevo que el de la app) se usa ese
        val actualizado = File(ctx.filesDir, "ytdlp/yt-dlp")
        if (actualizado.isFile) m.callAttr("usar_ytdlp", actualizado.path)
        return m
    }

    /** Pone en uso el yt-dlp de `archivo` (recarga aunque ya hubiera uno). Si no arranca, lanza el error y queda el anterior. Devuelve la versión. */
    fun usarYtdlp(ctx: Context, archivo: File?): String =
        py(ctx).getModule("descargas_py").callAttr("usar_ytdlp", archivo?.path, true).toString()

    /** El intérprete de JavaScript que pide YouTube (QuickJS, viene dentro de la app). */
    fun qjs(ctx: Context): String? = File(ctx.applicationInfo.nativeLibraryDir, "libqjs.so").takeIf { it.canExecute() }?.path

    fun version(ctx: Context): String = modulo(ctx).callAttr("version").toString()

    /** Datos del enlace sin descargar (JSON). */
    fun info(ctx: Context, url: String): String = modulo(ctx).callAttr("info", url, qjs(ctx)).toString()

    /**
     * Baja y deja UN archivo listo (une imagen y sonido si vinieron separados). Devuelve JSON: {ruta, tam, dur, tipo}.
     * modo: "video" o "audio". altura: 360, 720… (video). abr: kbps máximos del audio (0 = el mejor).
     */
    fun descargar(ctx: Context, url: String, carpeta: File, modo: String, altura: Int, abr: Int, progreso: Progreso?): String {
        val r = JSONObject(modulo(ctx).callAttr("bajar", url, carpeta.path, modo, altura, abr, qjs(ctx), progreso).toString())
        val a = r.getJSONArray("archivos")
        val final: File
        if (a.length() == 2) {
            val v = File(a.getJSONObject(0).getString("ruta")); val s = File(a.getJSONObject(1).getString("ruta"))
            final = File(carpeta, "video_${System.currentTimeMillis()}.mp4")
            ArchivosVideo.unir(v, s, final)
            v.delete(); s.delete()
        } else final = File(a.getJSONObject(0).getString("ruta"))
        return JSONObject().put("ruta", final.path).put("tam", final.length()).put("dur", ArchivosVideo.duracionS(final)).put("tipo", if (modo == "audio") "audio" else "video").toString()
    }
}
