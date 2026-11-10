package com.botchat.app.ui

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.AnnotatedString
import com.botchat.app.datos.Mencion
import com.botchat.app.datos.Msg

/** LRU mínima (en memoria, sin Android) para las cachés del chat. */
internal class Lru<K, V>(private val maximo: Int) {
    private val mapa = object : LinkedHashMap<K, V>(64, 0.75f, true) {
        override fun removeEldestEntry(eldest: MutableMap.MutableEntry<K, V>?) = size > maximo
    }
    @Synchronized fun get(k: K): V? = mapa[k]
    @Synchronized fun put(k: K, v: V) { mapa[k] = v }
    @Synchronized fun vaciar() = mapa.clear()
    val tamano: Int get() = synchronized(this) { mapa.size }
}

/** Texto ya formateado (negritas, enlaces, menciones) para no repetir las expresiones regulares cada vez que una burbuja vuelve a la pantalla. */
object CacheTexto {
    private data class Clave(val texto: String, val menciones: List<Mencion>, val color: Color)
    private val formateados = Lru<Clave, AnnotatedString>(600)
    private val emojis = Lru<String, Boolean>(600)

    fun formateado(texto: String, menciones: List<Mencion>, color: Color): AnnotatedString {
        val k = Clave(texto, menciones, color)
        formateados.get(k)?.let { return it }
        return FormatoWhatsApp.construir(texto, menciones, color, color).also { formateados.put(k, it) }
    }

    fun soloEmojis(texto: String): Boolean = emojis.get(texto) ?: FormatoWhatsApp.esSoloEmojis(texto).also { emojis.put(texto, it) }

    /** Deja listos (en un hilo de fondo) los textos de una página, para que dibujarlos sea solo leer la caché. */
    fun precalentar(msgs: List<Msg>, color: Color) {
        if (color == Color.Unspecified) return
        for (m in msgs) if (m.texto.isNotBlank()) { formateado(m.texto, m.menciones, color); soloEmojis(m.texto) }
    }

    fun vaciar() { formateados.vaciar(); emojis.vaciar() }
}

/**
 * La primera página de los últimos chats abiertos, en memoria: al volver a abrir uno se pinta al instante y la lectura de la base
 * solo la corrige. Se vacía al bloquear la app, igual que la caché de fotos.
 */
object CachePaginas {
    private data class Clave(val bot: Long, val jid: String)
    private val paginas = Lru<Clave, List<Msg>>(6)

    fun obtener(bot: Long, jid: String): List<Msg>? = paginas.get(Clave(bot, jid))
    fun guardar(bot: Long, jid: String, primera: List<Msg>) { if (primera.isEmpty()) paginas.put(Clave(bot, jid), emptyList()) else paginas.put(Clave(bot, jid), primera) }
    fun vaciar() = paginas.vaciar()
}
