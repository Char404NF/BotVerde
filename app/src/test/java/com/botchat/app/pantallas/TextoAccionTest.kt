package com.botchat.app.pantallas

import com.botchat.app.ui.esCambioConfig
import com.botchat.app.ui.nombreAccion
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class TextoAccionTest {
    @Test fun lasAccionesConocidasTienenNombreClaro() {
        assertEquals("Configuración base puesta en un grupo", nombreAccion("grupo.base_aplicar"))
        assertEquals("Copia de seguridad restaurada", nombreAccion("respaldo.restaurar"))
        assertEquals("Respuesta del bot cambiada", nombreAccion("respuesta.cambiar"))
    }

    @Test fun lasDesconocidasUsanSuFamiliaOSuNombre() {
        assertEquals("Filtro: flood", nombreAccion("filtro.flood"))
        assertEquals("Grupo: algo nuevo", nombreAccion("grupo.algo_nuevo"))
        assertEquals("cosa", nombreAccion("cosa"))
    }

    @Test fun soloLosCambiosDeConfiguracionEntranEnElFiltro() {
        for (a in listOf("grupo.base", "grupo.base_aplicar", "grupo.filtro", "grupo.reglas", "grupo.horario", "grupo.entrada", "rango.asignar", "respuesta.cambiar", "respaldo.restaurar", "modulo.activar", "ajuste.usuario_auto", "ajustes.alertas"))
            assertTrue(a, esCambioConfig(a))
        for (a in listOf("grupo.banear", "grupo.raid", "filtro.flood", "persona.silenciar", "persona.advertir", "grupo.encuesta", "silencio.vencio"))
            assertFalse(a, esCambioConfig(a))
    }
}
