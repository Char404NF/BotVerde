package com.botchat.app.ui

import com.botchat.app.datos.Msg

/** Un renglón de la lista del chat: separador de día, línea de "sin leer" o burbuja. */
internal sealed interface Elem {
    data class Dia(val ts: Long) : Elem
    data class SinLeer(val cuantos: Int) : Elem
    /** Aviso del sistema en el chat (alguien entró, salió, lo añadieron…), en una cápsula centrada. */
    data class Sistema(val ts: Long, val texto: String) : Elem
    /** Tarjeta del principio de un grupo (aparece arriba de todo cuando ya se cargaron todos los mensajes). */
    data class InicioGrupo(val texto: String) : Elem
    data class Burbuja(val m: Msg, val mostrarRemitente: Boolean, val ultimoDelGrupo: Boolean, val primeroDelGrupo: Boolean = false) : Elem
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
        out += Elem.Burbuja(m, mostrarRemitente = esGrupo && !m.esMio && !mismoAnterior, ultimoDelGrupo = !mismoSiguiente, primeroDelGrupo = !mismoAnterior)
        if (m.id == primerNoLeido) out += Elem.SinLeer(noLeidos)
        if (mas == null || dias[i + 1] != dias[i]) out += Elem.Dia(m.ts)
    }
    return out
}

/** Texto de un cambio de miembros, como lo escribe una app de mensajería ("Se añadió a Ana"). */
internal fun textoSistema(c: com.botchat.app.datos.CambioMiembro): String {
    val quien = c.nombre.ifBlank { c.persona.substringBefore('@') }
    val por = c.porNombre.ifBlank { c.por.substringBefore('@') }
    return when (c.tipo) {
        "entra" -> "$quien se unió al grupo"
        "enlace" -> "$quien se unió con el enlace"
        "agregado" -> if (por.isNotBlank() && por != quien) "$por añadió a $quien" else "Se añadió a $quien"
        "sale" -> "$quien salió"
        "expulsado" -> if (por.isNotBlank() && por != quien) "$por eliminó a $quien" else "Se eliminó a $quien"
        "promovido" -> "$quien ahora es admin"
        "degradado" -> "$quien ya no es admin"
        else -> "$quien: ${c.tipo}"
    }
}

/**
 * Mete en la lista (del más nuevo al más viejo) los avisos de cambios de miembros según su hora, y la tarjeta de inicio del grupo
 * al final cuando ya no hay mensajes más viejos por cargar.
 */
internal fun mezclarSistema(base: List<Elem>, cambios: List<com.botchat.app.datos.CambioMiembro>, esGrupo: Boolean, hayMas: Boolean, tarjeta: String): List<Elem> {
    if (!esGrupo) return base
    val pend = cambios.sortedByDescending { it.ts }
    val out = ArrayList<Elem>(base.size + pend.size + 1)
    var i = 0
    fun ts(e: Elem): Long? = when (e) { is Elem.Burbuja -> e.m.ts; is Elem.Dia -> e.ts; is Elem.Sistema -> e.ts; else -> null }
    for (e in base) {
        val t = ts(e)
        if (t != null) while (i < pend.size && pend[i].ts >= t) { out += Elem.Sistema(pend[i].ts, textoSistema(pend[i])); i++ }
        out += e
    }
    if (!hayMas) {
        while (i < pend.size) { out += Elem.Sistema(pend[i].ts, textoSistema(pend[i])); i++ }
        out += Elem.InicioGrupo(tarjeta)
    }
    return out
}
