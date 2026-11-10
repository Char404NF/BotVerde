package com.botchat.app

import android.app.Application
import kotlinx.coroutines.flow.MutableStateFlow
import java.io.File
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

class BotApp : Application() {
    override fun onCreate() {
        super.onCreate()
        BotState.iniciar(File(filesDir, "registro.txt"))
    }
}

/** Estado compartido entre el servicio y la pantalla. */
object BotState {
    /** Último estado que avisó el módulo Go (detenido, sin_vincular, esperando_codigo, vinculado, conectando, conectado…). */
    val estado = MutableStateFlow("detenido")
    val codigo = MutableStateFlow<String?>(null)
    val corriendo = MutableStateFlow(false)
    val registro = MutableStateFlow<List<String>>(emptyList())
    /** Sube cada vez que cambian los datos guardados (por ejemplo, al eliminar el bot). */
    val version = MutableStateFlow(0)
    /** Aviso para mostrar una vez (por ejemplo, un problema al cerrar la sesión). */
    val aviso = MutableStateFlow<String?>(null)

    private const val MAX_LINEAS = 300
    private const val MAX_ARCHIVO = 120_000L
    private var archivo: File? = null
    private val hora = SimpleDateFormat("HH:mm:ss", Locale.getDefault())

    @Synchronized
    fun iniciar(f: File) {
        archivo = f
        runCatching { if (f.exists()) registro.value = f.readLines().takeLast(MAX_LINEAS) }
    }

    @Synchronized
    fun log(linea: String) {
        val txt = "${hora.format(Date())}  ${linea.trim()}"
        registro.value = (registro.value + txt).takeLast(MAX_LINEAS)
        runCatching {
            val f = archivo ?: return
            if (f.exists() && f.length() > MAX_ARCHIVO) f.writeText(registro.value.joinToString("\n") + "\n")
            else f.appendText(txt + "\n")
        }
    }

    @Synchronized
    fun borrarRegistro() {
        registro.value = emptyList()
        runCatching { archivo?.delete() }
    }

    fun textoRegistro(): String = registro.value.joinToString("\n")
}
