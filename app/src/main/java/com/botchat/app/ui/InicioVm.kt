package com.botchat.app.ui

import android.app.Application
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.botchat.app.Datos
import com.botchat.app.datos.ChatItem
import com.botchat.app.datos.Repo
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch

/** Estado de la lista de chats (pestañas Personales y Grupos, con buscador). */
class InicioVm(app: Application) : AndroidViewModel(app) {
    val botId: Long = Datos.get(app).primero()?.id ?: 0L

    private val _personales = MutableStateFlow<List<ChatItem>?>(null) // null = todavía cargando
    private val _grupos = MutableStateFlow<List<ChatItem>?>(null)
    val personales: StateFlow<List<ChatItem>?> = _personales
    val grupos: StateFlow<List<ChatItem>?> = _grupos

    /** Mensajes recibidos desde la última vez que se abrió cada chat (hasta 30). Solo se calcula para chats con algo nuevo. */
    private val _contadores = MutableStateFlow<Map<String, Int>>(emptyMap())
    val contadores: StateFlow<Map<String, Int>> = _contadores
    private val ajustes = com.botchat.app.datos.Ajustes.get(app)

    var consulta by mutableStateOf("")
        private set
    var buscando by mutableStateOf(false)
        private set
    var error by mutableStateOf<String?>(null)
        private set

    private var trabajo: Job? = null

    init {
        recargar()
        viewModelScope.launch { Repo.cambios.collect { if ("chats" in it) recargar() } }
    }

    fun cambiarConsulta(q: String) {
        consulta = q
        trabajo?.cancel()
        trabajo = viewModelScope.launch { delay(200); cargar() }
    }

    fun abrirBusqueda() { buscando = true }

    fun cerrarBusqueda() {
        buscando = false
        if (consulta.isNotEmpty()) cambiarConsulta("")
    }

    private suspend fun contar(chats: List<ChatItem>) {
        val nuevo = HashMap<String, Int>()
        for (c in chats) {
            val visto = ajustes.visto(c.jid)
            if (c.ultimoTs <= visto) continue
            val n = runCatching { Repo.mensajes(botId, c.jid, limite = 30).count { it.ts > visto && !it.esMio } }.getOrDefault(0)
            if (n > 0) nuevo[c.jid] = n
        }
        _contadores.value = nuevo
    }

    private fun recargar() {
        trabajo?.cancel()
        trabajo = viewModelScope.launch { cargar() }
    }

    private suspend fun cargar() {
        try {
            val q = consulta
            val p = Repo.chats(botId, false, q)
            val g = Repo.chats(botId, true, q)
            _personales.value = p
            _grupos.value = g
            error = null
            contar(p + g)
        } catch (e: Exception) {
            error = e.message
            if (_personales.value == null) { _personales.value = emptyList(); _grupos.value = emptyList() }
        }
    }
}
