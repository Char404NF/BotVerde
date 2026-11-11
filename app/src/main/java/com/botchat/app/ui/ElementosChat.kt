package com.botchat.app.ui

import com.botchat.app.datos.Msg

/** Un renglón de la lista del chat: separador de día, línea de "sin leer" o burbuja. */
internal sealed interface Elem {
    data class Dia(val ts: Long) : Elem
    data class SinLeer(val cuantos: Int) : Elem
    data class Burbuja(val m: Msg, val mostrarRemitente: Boolean, val ultimoDelGrupo: Boolean) : Elem
}

/**
 * `totalAlAbrir` es el conteo real de no leídos al abrir el chat (puede ser mayor que lo cargado); la línea de "sin leer" lo muestra completo.
 * Arma la lista visual (del más nuevo al más viejo, para una lista invertida) con separadores de día, agrupación por remitente y la línea de "sin leer".
 * Se calcula fuera de la composición (en el ViewModel, en un hilo de fondo), con el día de cada mensaje calculado una sola vez.
 */
internal fun elementos(msgs: List<Msg>, esGrupo: Boolean, vistoAntes: Long = Long.MAX_VALUE, totalAlAbrir: Int = -1, aperturaTs: Long = 0L): List<Elem> {
    val out = ArrayList<Elem>(msgs.size + 8)
    val dias = LongArray(msgs.size) { diaDe(msgs[it].ts) }
    val cargadosSinLeer = msgs.count { !it.esMio && it.ts > vistoAntes }
    // Con el conteo real de la base (lo que había sin leer al abrir) más lo que llegó después; sin él, solo lo cargado.
    val noLeidos = if (totalAlAbrir < 0) cargadosSinLeer else maxOf(cargadosSinLeer, totalAlAbrir + msgs.count { !it.esMio && it.ts > aperturaTs && it.ts > vistoAntes })
    val primerNoLeido = if (cargadosSinLeer > 0) msgs.lastOrNull { !it.esMio && it.ts > vistoAntes }?.id else null
    for (i in msgs.indices) {
        val m = msgs[i]
        val mas = msgs.getOrNull(i + 1)      // el mensaje anterior en el tiempo
        val menos = msgs.getOrNull(i - 1)    // el siguiente en el tiempo
        val mismoAnterior = mas != null && dias[i + 1] == dias[i] && mas.remitenteId == m.remitenteId && m.ts - mas.ts < 300
        val mismoSiguiente = menos != null && dias[i - 1] == dias[i] && menos.remitenteId == m.remitenteId && menos.ts - m.ts < 300
        out += Elem.Burbuja(m, mostrarRemitente = esGrupo && !m.esMio && !mismoAnterior, ultimoDelGrupo = !mismoSiguiente)
        if (m.id == primerNoLeido) out += Elem.SinLeer(noLeidos)
        if (mas == null || dias[i + 1] != dias[i]) out += Elem.Dia(m.ts)
    }
    return out
}
