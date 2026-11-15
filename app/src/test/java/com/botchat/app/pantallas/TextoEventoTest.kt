package com.botchat.app.pantallas

import com.botchat.app.datos.EventoGrupo
import com.botchat.app.ui.textoEvento
import org.junit.Assert.assertEquals
import org.junit.Test

class TextoEventoTest {
    private fun accion(tipo: String, texto: String, actorNombre: String = "Yo", personaNombre: String = "", persona: String = "", res: String = "ok") =
        EventoGrupo(1, "accion", tipo, texto, "yo", actorNombre, persona, personaNombre, res)

    @Test fun elDuenoLeeEnSegundaPersona() {
        assertEquals("Tú cerraste el grupo", textoEvento(accion("grupo.cerrar", "cerró el grupo")))
        assertEquals("Tú expulsaste a Carlos", textoEvento(accion("grupo.expulsar", "expulsó a", personaNombre = "Carlos")))
        assertEquals("Tú borraste un mensaje", textoEvento(accion("grupo.borrar_mensaje", "borró un mensaje de")))
        assertEquals("Tú baneaste a +34600000007 (no se pudo)", textoEvento(accion("grupo.banear", "baneó a", persona = "34600000007@s.whatsapp.net", res = "error")))
    }

    @Test fun otrosActoresYLosCambiosDeMiembros() {
        assertEquals("Ana abrió el grupo", textoEvento(accion("grupo.abrir", "abrió el grupo", actorNombre = "Ana")))
        assertEquals("El bot cambió un filtro del grupo", textoEvento(accion("grupo.filtro", "cambió un filtro del grupo", actorNombre = "")))
        val sale = EventoGrupo(1, "cambio", "sale", "salió del grupo", "", "", "p", "Luis", "")
        assertEquals("Luis salió del grupo", textoEvento(sale))
        assertEquals("Luis fue expulsada por Tú", textoEvento(sale.copy(tipo = "expulsado", actorNombre = "Yo", actor = "yo")))
    }
}
