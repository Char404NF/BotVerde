package com.botchat.app.pantallas

import com.botchat.app.datos.ConfigFiltroGrupo
import com.botchat.app.datos.FiltroGrupo
import com.botchat.app.ui.PLANTILLAS_GRUPO
import com.botchat.app.ui.aplicarPlantilla
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class PlantillasUnitTest {
    private val base = listOf("enlaces", "palabras", "repetidos", "flood").map {
        FiltroGrupo("g", it, it, "", false, "avisarme", ConfigFiltroGrupo(palabras = if (it == "palabras") listOf("x") else emptyList(), max = 5))
    }

    @Test fun laPlantillaCambiaEncendidoYAccionPeroConservaLasListas() {
        val r = aplicarPlantilla(base, PLANTILLAS_GRUPO.first { it.id == "estricto" })
        assertEquals(4, r.size)
        assertTrue(r.all { it.activo && it.accion == "borrar_advertir" })
        assertEquals(listOf("x"), r.first { it.filtro == "palabras" }.config.palabras)
        assertEquals(5, r.first { it.filtro == "flood" }.config.max)
    }

    @Test fun familiaLoApagaTodoYComunidadSoloVigilaElRitmo() {
        assertTrue(aplicarPlantilla(base.map { it.copy(activo = true) }, PLANTILLAS_GRUPO.first { it.id == "familia" }).none { it.activo })
        val c = aplicarPlantilla(base, PLANTILLAS_GRUPO.first { it.id == "comunidad" })
        assertEquals(setOf("repetidos", "flood"), c.filter { it.activo }.map { it.filtro }.toSet())
        assertFalse(c.first { it.filtro == "enlaces" }.activo)
    }

    @Test fun unFiltroQueLaPlantillaNoConoceNoSeTocaYLasPlantillasTienenNombresUnicos() {
        val raro = base + FiltroGrupo("g", "nuevo", "n", "", true, "borrar", ConfigFiltroGrupo())
        assertTrue(aplicarPlantilla(raro, PLANTILLAS_GRUPO[0]).first { it.filtro == "nuevo" }.activo)
        assertEquals(PLANTILLAS_GRUPO.size, PLANTILLAS_GRUPO.map { it.id }.toSet().size)
    }
}
