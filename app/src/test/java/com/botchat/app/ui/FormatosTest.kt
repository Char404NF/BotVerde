package com.botchat.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.time.LocalDate
import java.time.ZoneId

class FormatosTest {
    private val hoy = LocalDate.of(2026, 11, 9)
    private fun ts(d: LocalDate, h: Int = 12, m: Int = 30) = d.atTime(h, m).atZone(ZoneId.systemDefault()).toEpochSecond()

    @Test fun bytes() {
        java.util.Locale.setDefault(java.util.Locale.forLanguageTag("es-MX"))
        assertEquals("0 B", formatoBytes(0)); assertEquals("999 B", formatoBytes(999)); assertEquals("1 KB", formatoBytes(1024))
        assertEquals("1.5 MB", formatoBytes(1_572_864)); assertEquals("2 GB", formatoBytes(2L * 1024 * 1024 * 1024))
    }

    @Test fun horasDeLaLista() {
        assertEquals("12:30", horaLista(ts(hoy), hoy))
        assertEquals("Ayer", horaLista(ts(hoy.minusDays(1)), hoy))
        assertEquals("Jueves", horaLista(ts(hoy.minusDays(4)), hoy)) // 5 de noviembre de 2026
        assertEquals("01/10/26", horaLista(ts(LocalDate.of(2026, 10, 1)), hoy))
        assertEquals("", horaLista(0, hoy))
    }

    @Test fun separadoresDeDia() {
        assertEquals("Hoy", etiquetaDia(ts(hoy), hoy)); assertEquals("Ayer", etiquetaDia(ts(hoy.minusDays(1)), hoy))
        assertEquals("1 de octubre", etiquetaDia(ts(LocalDate.of(2026, 10, 1)), hoy))
        assertEquals("15 de marzo de 2025", etiquetaDia(ts(LocalDate.of(2025, 3, 15)), hoy))
        assertTrue(mismoDia(ts(hoy, 1, 0), ts(hoy, 23, 59))); assertTrue(!mismoDia(ts(hoy), ts(hoy.minusDays(1))))
    }

    @Test fun duracionYVistaPrevia() {
        assertEquals("0:07", duracion(7)); assertEquals("2:05", duracion(125))
        assertEquals("Mensaje borrado", descripcionCorta("texto", "secreto", true))
        assertEquals("Foto", descripcionCorta("imagen", "", false)); assertEquals("Foto · mira", descripcionCorta("imagen", "mira", false))
        assertEquals("Nota de voz", descripcionCorta("audio", "", false, notaVoz = true)); assertEquals("Ver una vez", descripcionCorta("ver_una_vez", "x", false))
        assertEquals("hola mundo", descripcionCorta("texto", "hola\nmundo", false))
    }

    @Test fun avatarColorEstableEInicial() {
        assertEquals(colorEstable("abc@s.whatsapp.net", false), colorEstable("abc@s.whatsapp.net", false))
        assertNotEquals(colorEstable("abc@s.whatsapp.net", false), colorEstable("xyz@s.whatsapp.net", false))
        assertEquals("A", inicial("ana")); assertEquals("5", inicial("+52 55")); assertEquals("?", inicial("")); assertEquals("Ñ", inicial("ñandú"))
    }
}
