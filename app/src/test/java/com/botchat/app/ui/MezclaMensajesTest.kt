package com.botchat.app.ui

import com.botchat.app.datos.Msg
import com.botchat.app.datos.Reaccion
import org.junit.Assert.assertEquals
import org.junit.Assert.assertSame
import org.junit.Test

class MezclaMensajesTest {
    private fun m(id: String, ts: Long, texto: String = "t$id", estadoEnvio: String = "", leidos: Int = 0, reacciones: List<Reaccion> = emptyList(), editado: Boolean = false, borrado: Boolean = false) =
        Msg(id, "c", "x", "", false, "texto", texto, ts, borrado, editado, "ninguno", 0, "", "", "", "", 0, 0, 0, false, false, estadoEnvio = estadoEnvio, leidos = leidos, reacciones = reacciones)

    private fun ids(l: List<Msg>) = l.map { it.id }

    /** 10 mensajes (m10 el más nuevo … m01); la primera página es de 4. */
    private val todos = (10 downTo 1).map { m("m%02d".format(it), 1000L + it) }

    @Test fun mensajeNuevoEntraPrimeroSinDuplicarNada() {
        val actuales = todos.take(6)
        val primera = listOf(m("m11", 1011)) + todos.take(3)
        val r = MezclaMensajes.mezclar(actuales, primera, 4)
        assertEquals(listOf("m11", "m10", "m09", "m08", "m07", "m06", "m05"), ids(r)) // lo más viejo ya cargado se queda
    }

    @Test fun sinCambiosDevuelveLaMismaLista() {
        val actuales = todos.take(6)
        assertSame(actuales, MezclaMensajes.mezclar(actuales, todos.take(4), 4))
    }

    @Test fun recibosReaccionesEdicionesYEliminadosDeMensajesViejosSeAplicanPorId() {
        val actuales = todos // los 10 ya cargados
        val cambiados = listOf(
            m("m03", 1003, estadoEnvio = "leido", leidos = 1),
            m("m05", 1005, reacciones = listOf(Reaccion("👍", 1, false))),
            m("m06", 1006, texto = "editado", editado = true),
            m("m07", 1007, borrado = true, texto = ""),
        )
        val r = MezclaMensajes.mezclar(actuales, todos.take(2), 2, cambiados, cambiados.map { it.id }.toSet())
        assertEquals(ids(todos), ids(r)) // mismo orden, sin duplicados
        assertEquals("leido", r.first { it.id == "m03" }.estadoEnvio)
        assertEquals(1, r.first { it.id == "m05" }.reacciones.size)
        assertEquals("editado", r.first { it.id == "m06" }.texto)
        assertEquals(true, r.first { it.id == "m07" }.borrado)
        assertEquals("tm02", r.first { it.id == "m02" }.texto) // lo que no cambió, intacto
    }

    @Test fun cambiosEnLaPrimeraPaginaReemplazanAlMensaje() {
        val primera = listOf(m("m10", 1010, estadoEnvio = "entregado", leidos = 0), todos[1], todos[2], todos[3])
        val r = MezclaMensajes.mezclar(todos, primera, 4)
        assertEquals("entregado", r.first().estadoEnvio)
        assertEquals(10, r.size)
    }

    @Test fun unMensajeViejoBorradoParaMiDesapareceSiNoVuelveEnElLote() {
        val r = MezclaMensajes.mezclar(todos, todos.take(4), 4, porId = emptyList(), pedidos = setOf("m04"))
        assertEquals(9, r.size)
        assertEquals(false, r.any { it.id == "m04" })
    }

    @Test fun unMensajeDeLaPrimeraPaginaBorradoSeQuitaYElQueSigueNoSeToca() {
        // la página completa (4) ahora es m10, m09, m07, m06: m08 ya no existe
        val primera = listOf(todos[0], todos[1], todos[3], todos[4])
        val r = MezclaMensajes.mezclar(todos, primera, 4)
        assertEquals(listOf("m10", "m09", "m07", "m06", "m05", "m04", "m03", "m02", "m01"), ids(r))
    }

    @Test fun paginaIncompletaSignificaQueElChatEntero_CabeEnEllaYLoQueFaltaSeBorro() {
        val primera = todos.take(3) // límite 4, vinieron 3: el chat solo tiene 3
        val r = MezclaMensajes.mezclar(todos, primera, 4)
        assertEquals(listOf("m10", "m09", "m08"), ids(r))
    }

    @Test fun chatVaciadoDejaLaListaVacia() {
        assertEquals(emptyList<Msg>(), MezclaMensajes.mezclar(todos, emptyList(), 4))
    }

    @Test fun mensajeNuevoConLaMismaHoraSeOrdenaPorId() {
        val a = m("a", 2000); val b = m("b", 2000)
        assertEquals(listOf("b", "a"), ids(MezclaMensajes.mezclar(emptyList(), listOf(a, b), 10)))
    }

    @Test fun agregarPaginaVieja_NoDuplicaNiPisaCambiosRecientes() {
        val actuales = todos.take(4).toMutableList().also { it[3] = m("m07", 1007, leidos = 1, estadoEnvio = "leido") } // m07 cambió mientras se pedía la página
        val mas = todos.subList(3, 7) // incluye m07 viejo (sin cambio) y m06..m04
        val r = MezclaMensajes.agregar(actuales, mas)
        assertEquals(listOf("m10", "m09", "m08", "m07", "m06", "m05", "m04"), ids(r))
        assertEquals("leido", r.first { it.id == "m07" }.estadoEnvio)
    }

    @Test fun agregarNadaDevuelveLaMismaLista() {
        val a = todos.take(3)
        assertSame(a, MezclaMensajes.agregar(a, emptyList()))
        assertSame(a, MezclaMensajes.agregar(a, a))
    }

    @Test fun elementosInsertaSeparadoresDeDiaYAgrupa() {
        val dia = 86_400L
        val l = listOf(m("c", 3 * dia + 100), m("b", 3 * dia + 50), m("a", 2 * dia + 10))
        val el = elementos(l, esGrupo = false)
        assertEquals(listOf("Burbuja", "Burbuja", "Dia", "Burbuja", "Dia"), el.map { it::class.simpleName })
    }
}
