package com.botchat.app.ui

import com.botchat.app.datos.Msg

/** Orden de la lista del chat: del más nuevo al más viejo (hora y, si empatan, id). */
private val masNuevoPrimero = Comparator<Msg> { a, b -> if (a.ts != b.ts) b.ts.compareTo(a.ts) else b.id.compareTo(a.id) }

/**
 * Mezcla lo que ya está en pantalla con lo que acaba de llegar de la base, sin duplicados y en orden.
 *
 *  - [primera]: la primera página (los más nuevos) recién leída. Reemplaza a los mismos ids y agrega los nuevos.
 *    Si la página vino completa ([limite]), todo lo cargado que sea tan nuevo como su último mensaje y NO esté en ella ya no existe
 *    (se borró); si vino incompleta, el chat entero cabe en ella y lo que falte también se borró.
 *  - [porId]: mensajes ya cargados que se volvieron a pedir por su id (recibos, reacciones, ediciones, eliminados, estado de envío…).
 *  - [pedidos]: los ids que se pidieron; los que no regresaron ya no existen y se quitan.
 *
 * Si nada cambió devuelve la misma lista [actuales] (para no recomponer la pantalla de balde).
 */
object MezclaMensajes {
    fun mezclar(actuales: List<Msg>, primera: List<Msg>, limite: Int, porId: List<Msg> = emptyList(), pedidos: Set<String> = emptySet()): List<Msg> {
        val mapa = LinkedHashMap<String, Msg>(actuales.size + primera.size)
        for (m in actuales) mapa[m.id] = m
        val enPrimera = HashSet<String>(primera.size * 2)
        for (m in primera) enPrimera += m.id
        // Lo que ya no existe: lo pedido por id que no volvió, y lo cargado dentro del rango de la primera página que ya no está en ella.
        val devueltos = porId.mapTo(HashSet()) { it.id }
        for (id in pedidos) if (id !in devueltos && id !in enPrimera) mapa.remove(id)
        val ultima = primera.lastOrNull()
        val completa = primera.size >= limite
        val aQuitar = mapa.values.filter { m -> m.id !in enPrimera && (!completa || ultima == null || masNuevoPrimero.compare(m, ultima) <= 0) }.map { it.id }
        // (con la página vacía y completa=false se quita todo: el chat quedó sin mensajes)
        for (id in aQuitar) mapa.remove(id)
        for (m in porId) mapa[m.id] = m
        for (m in primera) mapa[m.id] = m
        val out = mapa.values.sortedWith(masNuevoPrimero)
        return if (out == actuales) actuales else out
    }

    /** Agrega una página más vieja (cargar más) sin duplicar ni pisar cambios que llegaron mientras se pedía. */
    fun agregar(actuales: List<Msg>, mas: List<Msg>): List<Msg> {
        if (mas.isEmpty()) return actuales
        val ya = actuales.mapTo(HashSet(actuales.size * 2)) { it.id }
        val nuevos = mas.filter { it.id !in ya }
        if (nuevos.isEmpty()) return actuales
        return (actuales + nuevos).sortedWith(masNuevoPrimero)
    }
}
