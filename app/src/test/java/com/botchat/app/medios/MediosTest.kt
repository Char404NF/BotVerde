package com.botchat.app.medios

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class MediosTest {
    @Test fun muestreoDeImagenes() {
        assertEquals(1, CacheImagenes.muestreo(800, 1080)); assertEquals(1, CacheImagenes.muestreo(1080, 1080))
        assertEquals(2, CacheImagenes.muestreo(2200, 1080)); assertEquals(4, CacheImagenes.muestreo(4500, 1080)); assertEquals(1, CacheImagenes.muestreo(4000, 0))
    }

    @Test fun nombresSeguros() {
        assertEquals("cuenta.pdf", Temporales.nombreSeguro("cuenta.pdf"))
        assertEquals("passwd", Temporales.nombreSeguro("../../etc/passwd"))
        assertFalse(Temporales.nombreSeguro("a/b\\c:d*e?.txt").contains('/'))
        assertEquals("archivo", Temporales.nombreSeguro("...")); assertEquals("archivo", Temporales.nombreSeguro(""))
        assertTrue(Temporales.nombreSeguro("x".repeat(300)).length <= 80)
    }
}
