package com.botchat.app.ui

import android.app.Application
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.botchat.app.Datos
import com.botchat.app.datos.EspacioChat
import com.botchat.app.datos.EspacioTipo
import com.botchat.app.datos.EspacioTotal
import com.botchat.app.datos.Liberado
import com.botchat.app.datos.Repo
import com.botchat.app.datos.ReporteReparacion
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/** Algo que se va a borrar y todavía espera la confirmación del usuario. */
data class AccionBorrar(val titulo: String, val texto: String, val boton: String, val ejecutar: suspend () -> Liberado)

val CATEGORIAS = listOf("foto", "video", "audio", "documento")

class EspacioVm(app: Application) : AndroidViewModel(app) {
    val botId: Long = Datos.get(app).primero()?.id ?: 0L

    var total by mutableStateOf<EspacioTotal?>(null)
        private set
    var tipos by mutableStateOf<List<EspacioTipo>>(emptyList())
        private set
    var chats by mutableStateOf<List<EspacioChat>?>(null)
        private set
    var filtro by mutableStateOf("") // "" = todo
    var seleccion by mutableStateOf<Set<String>>(emptySet())
        private set
    var pendiente by mutableStateOf<AccionBorrar?>(null)
    var estimadoVideos by mutableStateOf(Liberado(0, 0))
        private set
    var estimadoViejos by mutableStateOf(Liberado(0, 0))
        private set
    var estimadoTodo by mutableStateOf(Liberado(0, 0))
        private set
    var reporte by mutableStateOf<ReporteReparacion?>(null)
    var ocupado by mutableStateOf(false)
        private set
    var mensaje by mutableStateOf<String?>(null)

    private var trabajo: Job? = null

    init {
        recargar()
        viewModelScope.launch {
            Repo.cambios.collect { evs ->
                if ("espacio" in evs) { trabajo?.cancel(); trabajo = launch { delay(300); cargar() } }
            }
        }
    }

    fun recargar() { trabajo?.cancel(); trabajo = viewModelScope.launch { cargar() } }

    private suspend fun cargar() {
        try {
            total = Repo.espacioTotal(botId)
            tipos = Repo.espacioPorTipo(botId)
            chats = Repo.espacioPorChat(botId)
            estimadoVideos = Repo.estimarBorrado(botId, categoria = "video")
            estimadoViejos = Repo.estimarBorrado(botId, diasMin = 30)
            estimadoTodo = Repo.estimarBorrado(botId)
            seleccion = seleccion.filter { j -> chats?.any { it.jid == j } == true }.toSet()
        } catch (e: Exception) {
            mensaje = e.message
            if (chats == null) chats = emptyList()
        }
    }

    /** Lo que ocupa un chat con el filtro actual. */
    fun bytesDe(c: EspacioChat): Long = if (filtro.isEmpty()) c.bytes else c.porTipo.getOrElse(CATEGORIAS.indexOf(filtro)) { 0 }

    fun visibles(): List<EspacioChat> = (chats ?: emptyList()).filter { bytesDe(it) > 0 }.sortedByDescending { bytesDe(it) }

    // ---- selección múltiple ----
    fun alternarSeleccion(jid: String) { seleccion = if (jid in seleccion) seleccion - jid else seleccion + jid }
    fun limpiarSeleccion() { seleccion = emptySet() }
    fun seleccionarTodo() { seleccion = visibles().map { it.jid }.toSet() }

    // ---- acciones: siempre piden confirmación diciendo cuánto se libera ----
    fun pedirBorrarVideos() = pedir("¿Borrar todos los videos?", estimadoVideos, "Se conservan los mensajes de texto y las demás fotos y archivos.", "Borrar videos") { Repo.borrarArchivos(botId, categoria = "video") }
    fun pedirBorrarViejos() = pedir("¿Borrar archivos de más de 30 días?", estimadoViejos, "Se conservan los mensajes de texto.", "Borrar archivos") { Repo.borrarArchivos(botId, diasMin = 30) }
    fun pedirBorrarTodo() = pedir("¿Borrar todo lo multimedia?", estimadoTodo, "Se borran fotos, videos, audios y documentos. Los mensajes de texto se conservan.", "Borrar todo lo multimedia") { Repo.borrarArchivos(botId) }

    fun pedirBorrarArchivosDe(c: EspacioChat) = pedir("¿Borrar los archivos de ${c.nombre}?", Liberado(bytesDe(c), c.cantidad), "Los mensajes de texto se conservan.", "Borrar archivos") {
        Repo.borrarArchivos(botId, chat = c.jid, categoria = filtro)
    }

    fun pedirBorrarChat(c: EspacioChat) = pedir("¿Borrar todo el chat con ${c.nombre}?", Liberado(c.bytes, c.cantidad), "Se borran también los mensajes de texto y la foto de este chat. No se borra nada en WhatsApp.", "Borrar todo el chat") {
        Repo.borrarChat(botId, c.jid)
    }

    fun pedirBorrarSeleccion() {
        val elegidos = (chats ?: emptyList()).filter { it.jid in seleccion }
        val bytes = elegidos.sumOf { bytesDe(it) }
        val cant = elegidos.size
        pedir("¿Borrar archivos de $cant chat${if (cant == 1) "" else "s"}?", Liberado(bytes, 0), "Los mensajes de texto se conservan.", "Borrar archivos") {
            var b = 0L; var n = 0
            for (c in elegidos) { val l = Repo.borrarArchivos(botId, chat = c.jid, categoria = filtro); b += l.bytes; n += l.cantidad }
            limpiarSeleccion()
            Liberado(b, n)
        }
    }

    private fun pedir(titulo: String, est: Liberado, aviso: String, boton: String, f: suspend () -> Liberado) {
        pendiente = AccionBorrar(titulo, "Se liberarán ${formatoBytes(est.bytes)}. $aviso", boton, f)
    }

    fun confirmar(alTerminar: (Liberado) -> Unit) {
        val a = pendiente ?: return
        pendiente = null
        ocupado = true
        viewModelScope.launch {
            try {
                val l = a.ejecutar()
                alTerminar(l)
            } catch (e: Exception) {
                mensaje = e.message ?: "No se pudo borrar"
            } finally {
                ocupado = false
                cargar()
            }
        }
    }

    fun reparar() {
        ocupado = true
        viewModelScope.launch {
            try { reporte = Repo.reparar(botId) } catch (e: Exception) { mensaje = e.message } finally { ocupado = false; cargar() }
        }
    }
}
