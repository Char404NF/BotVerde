package com.botchat.app.seguridad

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class PinTest {
    @Test fun validaLargoYDigitos() {
        for (p in listOf("1234", "12345", "123456")) assertTrue(p, Pin.valido(p))
        for (p in listOf("", "123", "1234567", "12a4", "12 4", "１２３４")) assertFalse(p, Pin.valido(p))
    }

    @Test fun hashConSalYVerificacion() {
        val a = Pin.crear("4821", iter = 1000)
        val b = Pin.crear("4821", iter = 1000)
        assertNotEquals("la sal es distinta cada vez", a, b)
        assertTrue(a.startsWith("v1\$1000\$"))
        assertTrue(Pin.verificar("4821", a))
        assertFalse(Pin.verificar("4822", a))
        assertFalse(Pin.verificar("482", a))
        assertFalse(a.contains("4821"))
        assertFalse(Pin.verificar("4821", "basura"))
        assertFalse(Pin.verificar("4821", "v1\$1000\$no-base64\$tampoco"))
    }

    @Test fun elFormatoEsDeterministaConLaMismaSal() {
        val sal = ByteArray(16) { it.toByte() }
        assertEquals(Pin.crear("123456", sal, 1000), Pin.crear("123456", sal, 1000))
    }

    @Test fun pinInvalidoNoSeGuarda() {
        try { Pin.crear("12"); throw AssertionError("debía fallar") } catch (e: IllegalArgumentException) { /* ok */ }
    }

    private class Almacen { val m = mutableMapOf<String, String>() }

    private fun control(a: Almacen, reloj: () -> Long) = ControlIntentos({ a.m[it] }, { k, v -> if (v == null) a.m.remove(k) else a.m[k] = v }, reloj)

    @Test fun cincoFallosBloqueanYLaEsperaCrece() {
        var t = 1_000_000L
        val a = Almacen()
        val c = control(a) { t }
        assertFalse(c.tienePin())
        c.fijar("1234")
        assertTrue(c.tienePin())
        assertEquals(ResultadoPin.Incorrecto(4), c.probar("0000"))
        assertEquals(ResultadoPin.Incorrecto(3), c.probar("0000"))
        assertEquals(ResultadoPin.Incorrecto(2), c.probar("0000"))
        assertEquals(ResultadoPin.Incorrecto(1), c.probar("0000"))
        assertEquals(ResultadoPin.Bloqueado(30), c.probar("0000"))
        // Durante el bloqueo ni el PIN correcto entra.
        assertTrue(c.probar("1234") is ResultadoPin.Bloqueado)
        t += 31_000
        assertEquals(0, c.segundosDeEspera())
        // Tras esperar, se vuelve a permitir; otros 5 fallos duplican la espera.
        repeat(4) { c.probar("0000") }
        assertEquals(ResultadoPin.Bloqueado(60), c.probar("0000"))
        t += 61_000
        assertEquals(ResultadoPin.Correcto, c.probar("1234"))
        // Un acierto reinicia la cuenta.
        assertEquals(ResultadoPin.Incorrecto(4), c.probar("0000"))
    }

    @Test fun quitarElPinBorraTodo() {
        val a = Almacen()
        val c = control(a) { 0L }
        c.fijar("1234"); c.probar("1")
        c.quitar()
        assertTrue(a.m.isEmpty())
        assertNull(c.pinGuardado())
        assertEquals(ResultadoPin.Correcto, c.probar("loquesea"))
    }
}
