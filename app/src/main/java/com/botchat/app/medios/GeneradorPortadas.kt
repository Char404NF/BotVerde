package com.botchat.app.medios

import android.graphics.Bitmap
import java.io.ByteArrayOutputStream
import java.io.File

/**
 * Genera la portada (~720 px, JPEG calidad 85) de los videos ya descargados que todavía no tienen una. Go la guarda cifrada y la
 * conserva aunque el video se borre (por limpieza o por el tope total). Reutiliza [PortadaVideo.sacarCuadro]: nada se escribe en claro.
 * Los videos de los que no se pudo sacar un cuadro se anotan en memoria y no se reintentan hasta que se reinicie el servicio.
 */
class GeneradorPortadas(
    private val raiz: File,
    private val clave: ByteArray,
    private val origen: Origen,
    private val cuadro: (File, ByteArray, Int, Long) -> Bitmap? = { f, k, lado, dur -> PortadaVideo.sacarCuadro(f, k, lado, dur) },
) {
    /** Un video sin portada: chat, id y ruta relativa del archivo cifrado. */
    data class Pendiente(val chat: String, val id: String, val ruta: String)

    /** Lo que se necesita del módulo Go. */
    interface Origen {
        fun pendientes(limite: Int): List<Pendiente>
        fun guardar(chat: String, id: String, jpeg: ByteArray, ancho: Int, alto: Int)
    }

    private val fallidos = HashSet<String>()

    /** Procesa hasta [limite] videos. Devuelve cuántas portadas se guardaron. */
    fun pasada(limite: Int = 3, parar: () -> Boolean = { false }): Int {
        var hechas = 0
        val lista = runCatching { origen.pendientes(limite + fallidos.size) }.getOrDefault(emptyList())
        for (p in lista) {
            if (parar()) break
            val k = p.chat + "\u0000" + p.id
            if (k in fallidos) continue
            if (hechas >= limite) break
            val bmp = runCatching { cuadro(File(raiz, p.ruta), clave, LADO_MAX, 0L) }.getOrNull()
            if (bmp == null) { fallidos += k; continue }
            try {
                val sal = ByteArrayOutputStream()
                bmp.compress(Bitmap.CompressFormat.JPEG, CALIDAD, sal)
                origen.guardar(p.chat, p.id, sal.toByteArray(), bmp.width, bmp.height)
                hechas++
            } catch (e: Exception) {
                fallidos += k
            } finally {
                bmp.recycle()
            }
        }
        return hechas
    }

    companion object {
        const val LADO_MAX = 720
        const val CALIDAD = 85
    }
}
