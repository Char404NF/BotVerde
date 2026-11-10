package com.botchat.app.datos

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** org.json en pruebas locales necesita Robolectric. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class)
class ModelosEstadisticasTest {
    @Test fun resumenCompletaHorasYSemanasQueFaltan() {
        val r = Json.resumenChat("""{"desde":5,"listo":true,"enviados":1,"recibidos":2,"porTipo":[{"tipo":"texto","enviados":1,"recibidos":2,"bytes":0}],"porHora":[1],"porDiaSemana":null,"quien":[]}""")
        assertEquals(24, r.porHora.size); assertEquals(7, r.porSemana.size)
        assertEquals(3L, r.porTipo[0].total)
    }

    @Test fun contactosYUbicacionesLeenElFormatoNuevoYIgnoranElAnterior() {
        val c = Json.contactos("""{"v":1,"contactos":[{"nombre":"Ana","tels":["+52 55","+52 56"]}]}""")!!
        assertEquals("Ana", c[0].nombre); assertEquals(2, c[0].telefonos.size)
        assertNull(Json.contactos("Contacto: Ana"))
        assertNull(Json.ubicacion("Ubicación: Casa"))
        val u = Json.ubicacion("""{"v":1,"lat":19.4,"lon":-99.1,"nombre":"Casa","vivo":true}""")!!
        assertTrue(u.vivo); assertEquals(19.4, u.lat, 0.0001)
    }

    @Test fun globalLeeChatsActivosYDias() {
        val g = Json.resumenGlobal("""{"desde":1,"listo":false,"enviados":2,"recibidos":3,"chats":1,"porTipo":[],"activos":[{"jid":"a","nombre":"Ana","foto":"","fotoTs":0,"esGrupo":true,"pais":"","cantidad":5}],"porDia":[{"dia":"2026-11-01","cantidad":4}]}""")
        assertEquals(false, g.listo); assertEquals("Ana", g.activos[0].nombre); assertEquals(4L, g.porDia[0].cantidad)
    }
}
