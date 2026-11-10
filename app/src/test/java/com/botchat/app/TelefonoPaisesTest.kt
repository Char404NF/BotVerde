package com.botchat.app

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.text.Collator
import java.util.Locale

class TelefonoPaisesTest {
    @Test fun mexicoSeLimpiaSolo() {
        for (e in listOf("5512345678", "525512345678", "5215512345678", "15512345678", "0445512345678", "0455512345678", "+52 (55) 1234-5678")) {
            assertEquals(e, "5512345678", Telefono.limpiaMx(e))
        }
    }

    @Test fun mexicoSeEnviaConoSinUno() {
        assertEquals("525512345678", Telefono.paraVincular("MX", "5512345678", false))
        assertEquals("5215512345678", Telefono.paraVincular("MX", "5512345678", true))
        assertEquals("525512345678", Telefono.paraVincular("MX", "5215512345678", false))
    }

    @Test fun otrosPaises() {
        assertEquals("34612345678", Telefono.paraVincular("ES", "612 34 56 78", false))
        assertEquals("12025550123", Telefono.paraVincular("US", "2025550123", false))
        assertEquals("447911123456", Telefono.paraVincular("GB", "07911 123456", false))
    }

    @Test fun formatoMientrasSeEscribe() {
        assertEquals("55 1234 5678", Telefono.formatea("MX", "5512345678"))
        assertEquals("+52 55 1234 5678", Telefono.internacional("MX", "5512345678"))
        assertEquals("55", Telefono.formatea("MX", "55"))
    }

    @Test fun mensajesDeError() {
        assertEquals("Escribe tu número de teléfono.", Telefono.valida("MX", ""))
        assertEquals("El número es muy corto: faltan 3 dígitos.", Telefono.valida("MX", "5512345"))
        assertNull(Telefono.valida("MX", "5512345678"))
        assertNull(Telefono.valida("MX", "525512345678"))
        assertNotNull(Telefono.valida("ES", "6123"))
        assertNull(Telefono.valida("ES", "612345678"))
        assertNotNull(Telefono.valida("US", "202555"))
    }

    @Test fun listaDePaises() {
        val t = Paises.todos
        println("Países en la lista: ${t.size}")
        assertTrue(t.size > 200)
        assertEquals(t.size, t.map { it.iso }.toSet().size)
        val mx = Paises.porIso("MX")
        assertEquals("México", mx.nombre)
        assertEquals(52, mx.codigo)
        assertEquals("+52", mx.codigoTexto)
        val c = Collator.getInstance(Locale.forLanguageTag("es"))
        for (i in 1 until t.size) assertTrue("${t[i - 1].nombre} / ${t[i].nombre}", c.compare(t[i - 1].nombre, t[i].nombre) <= 0)
        println("Primeros: " + t.take(5).joinToString { it.nombre } + " … Últimos: " + t.takeLast(3).joinToString { it.nombre })
    }

    @Test fun buscador() {
        assertTrue(Paises.buscar("mexico").any { it.iso == "MX" })
        assertTrue(Paises.buscar("MÉXICO").any { it.iso == "MX" })
        assertTrue(Paises.buscar("peru").any { it.iso == "PE" })
        assertTrue(Paises.buscar("+52").any { it.iso == "MX" })
        assertTrue(Paises.buscar("52").any { it.iso == "MX" })
        assertTrue(Paises.buscar("españa").any { it.iso == "ES" })
        assertTrue(Paises.buscar("zzzz").isEmpty())
    }

    @Test fun banderaEmoji() {
        assertEquals("🇲🇽", Paises.emojiBandera("MX"))
        assertEquals("", Paises.emojiBandera("1"))
    }
}
