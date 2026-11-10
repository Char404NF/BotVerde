package com.botchat.app.ui

import android.app.Application
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.botchat.app.Datos
import com.botchat.app.datos.ChatItem
import com.botchat.app.datos.Json
import com.botchat.app.datos.Msg
import com.botchat.app.datos.Repo
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import wabot.Wabot

/** Estado de un chat abierto: encabezado y mensajes (del más nuevo al más viejo), con carga por páginas y cambios en vivo. */
class ChatVm(app: Application, val jid: String) : AndroidViewModel(app) {
    val botId: Long = Datos.get(app).primero()?.id ?: 0L

    var chat by mutableStateOf<ChatItem?>(null)
        private set
    var mensajes by mutableStateOf<List<Msg>?>(null) // null = cargando
        private set
    var hayMas by mutableStateOf(true)
        private set
    var cargandoMas by mutableStateOf(false)
        private set
    var aviso by mutableStateOf<String?>(null)

    private var trabajo: Job? = null
    private val pagina = 60

    init {
        recargar()
        viewModelScope.launch {
            Repo.cambios.collect { evs ->
                if (evs.any { it == "chat:$jid" || it.startsWith("archivo:$jid:") || it == "chats" }) {
                    trabajo?.cancel()
                    trabajo = launch { delay(80); recargar() }
                }
            }
        }
    }

    private suspend fun cargar(cuantos: Int) {
        val c = Json.chatOpt(runCatching { Wabot.obtenerChat(botId, jid) }.getOrDefault("null"))
        chat = c
        val m = Repo.mensajes(botId, jid, limite = cuantos)
        mensajes = m
        hayMas = m.size >= cuantos
    }

    fun recargar() {
        viewModelScope.launch {
            try {
                cargar(maxOf(pagina, mensajes?.size ?: 0))
            } catch (e: Exception) {
                mensajes = mensajes ?: emptyList()
                aviso = e.message
            }
        }
    }

    fun cargarMas() {
        val actuales = mensajes ?: return
        if (cargandoMas || !hayMas || actuales.isEmpty()) return
        cargandoMas = true
        viewModelScope.launch {
            try {
                val u = actuales.last()
                val mas = Repo.mensajes(botId, jid, u.ts, u.id, pagina)
                mensajes = actuales + mas
                hayMas = mas.size >= pagina
            } catch (e: Exception) {
                aviso = e.message
            } finally {
                cargandoMas = false
            }
        }
    }

    fun descargar(m: Msg) {
        viewModelScope.launch {
            try {
                Repo.descargar(botId, jid, m.id)
            } catch (e: Exception) {
                aviso = e.message ?: "No se pudo descargar"
            }
        }
    }

    fun estimarArchivos(alTerminar: (com.botchat.app.datos.Liberado) -> Unit) {
        viewModelScope.launch { runCatching { Repo.estimarBorrado(botId, chat = jid) }.onSuccess(alTerminar).onFailure { aviso = it.message } }
    }

    fun borrarArchivos(alTerminar: (com.botchat.app.datos.Liberado) -> Unit) {
        viewModelScope.launch { runCatching { Repo.borrarArchivos(botId, chat = jid) }.onSuccess(alTerminar).onFailure { aviso = it.message } }
    }

    fun borrarChat(alTerminar: () -> Unit) {
        viewModelScope.launch { runCatching { Repo.borrarChat(botId, jid) }.onSuccess { alTerminar() }.onFailure { aviso = it.message } }
    }
}
