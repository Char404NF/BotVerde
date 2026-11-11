package com.botchat.app.ui

import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.input.TextFieldValue
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class EstiloLetraTest {
    private fun a(t: String, e: EstiloLetra) = EstiloLetraUnicode.aplicar(t, e)

    @Test fun cadaEstiloConvierteLasLetrasYLosNumerosEsperados() {
        assertEquals("𝐇𝐨𝐥𝐚 𝟏𝟐𝟑", a("Hola 123", EstiloLetra.NEGRITA))
        assertEquals("𝐻𝑜𝑙𝑎 123", a("Hola 123", EstiloLetra.CURSIVA)) // la cursiva no tiene números
        assertEquals("ℋℴ𝓁𝒶 123", a("Hola 123", EstiloLetra.SCRIPT)) // la o script vive aparte (ℴ)
        assertEquals("ℌ𝔬𝔩𝔞", a("Hola", EstiloLetra.GOTICA))
        assertEquals("ℍ𝕠𝕝𝕒 𝟙𝟚𝟛", a("Hola 123", EstiloLetra.DOBLE))
        assertEquals("𝙷𝚘𝚕𝚊 𝟷𝟸𝟹", a("Hola 123", EstiloLetra.MAQUINA))
        assertEquals("Ⓗⓞⓛⓐ ①②③", a("Hola 123", EstiloLetra.BURBUJAS))
        assertEquals("Hᴏʟᴀ 123", a("Hola 123", EstiloLetra.VERSALITAS)) // las mayúsculas y los números no cambian
    }

    @Test fun losAlfabetosCompletosNoTienenHuecos() {
        val abc = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
        for (e in EstiloLetra.values()) {
            if (e == EstiloLetra.NORMAL) continue
            val r = a(abc, e)
            // ninguna letra se queda como un signo de reemplazo ni se pierde: de vuelta a Normal da lo mismo
            assertEquals("$e", abc, a(r, EstiloLetra.NORMAL))
            assertFalse("$e", r.contains('�'))
        }
        // los casos con “agujero” del bloque matemático de Unicode: usan la letra suelta
        assertEquals("ℎ", a("h", EstiloLetra.CURSIVA))
        assertEquals("ℬ", a("B", EstiloLetra.SCRIPT))
        assertEquals("ℭ", a("C", EstiloLetra.GOTICA))
        assertEquals("ℝ", a("R", EstiloLetra.DOBLE))
    }

    @Test fun losAcentosYLaEnieSeQuedanNormales() {
        assertEquals("𝐜𝐚𝐧𝐜𝐢ó𝐧", a("canción", EstiloLetra.NEGRITA))
        val r = a("mañana está ñandú", EstiloLetra.NEGRITA)
        assertTrue(r.contains("ñ") && r.contains("á") && r.contains("ú"))
        assertEquals("𝐦𝐚ñ𝐚𝐧𝐚 𝐞𝐬𝐭á ñ𝐚𝐧𝐝ú", r)
        assertEquals("¿𝐐𝐮é 𝐭𝐚𝐥?", a("¿Qué tal?", EstiloLetra.NEGRITA))
    }

    @Test fun lasVariablesNoSeTocan() {
        assertEquals("𝐇𝐨𝐥𝐚 {nombre}, 𝐬𝐨𝐧 𝐥𝐚𝐬 {hora}", a("Hola {nombre}, son las {hora}", EstiloLetra.NEGRITA))
        assertEquals("{dia} {fecha} {horario}", a("{dia} {fecha} {horario}", EstiloLetra.GOTICA))
        // una llave que no es una variable (con espacios) sí se convierte
        assertEquals("{𝐚 𝐛}", a("{a b}", EstiloLetra.NEGRITA))
    }

    @Test fun convertirDosVecesNoCambiaNadaYSePuedeCambiarDeEstilo() {
        val una = a("Hola mundo 2026", EstiloLetra.SCRIPT)
        assertEquals(una, a(una, EstiloLetra.SCRIPT))
        assertEquals(a("Hola mundo 2026", EstiloLetra.DOBLE), a(una, EstiloLetra.DOBLE))
        assertEquals("Hola mundo 2026", a(una, EstiloLetra.NORMAL))
        assertTrue(EstiloLetraUnicode.tieneEspeciales(una))
        assertFalse(EstiloLetraUnicode.tieneEspeciales("Hola mundo ñ"))
        assertNotEquals("Hola", a("Hola", EstiloLetra.NEGRITA))
    }

    @Test fun losEmojisYElRestoDelTextoPasanIgual() {
        assertEquals("😀 𝐇𝐨𝐥𝐚 👋🏽\n- 𝐮𝐧𝐨", a("😀 Hola 👋🏽\n- uno", EstiloLetra.NEGRITA))
        assertEquals("", a("", EstiloLetra.NEGRITA))
    }

    @Test fun laSeleccionSeConvierteYLoDemasQueda() {
        val v = TextFieldValue("hola mundo cruel", TextRange(5, 10))
        val r = EstiloLetraUnicode.aplicarASeleccion(v, EstiloLetra.NEGRITA)
        assertEquals("hola 𝐦𝐮𝐧𝐝𝐨 cruel", r.text)
        assertEquals(5, r.selection.min)
        assertEquals(r.text.indexOf(" cruel"), r.selection.max)
        // sin selección no hace nada
        assertEquals(v.copy(selection = TextRange(3)), EstiloLetraUnicode.aplicarASeleccion(v.copy(selection = TextRange(3)), EstiloLetra.NEGRITA))
    }

    @Test fun laSeleccionNoParteUnaVariablePorLaMitad() {
        // la selección cubre "ola {nom": se amplía hasta cerrar la variable, que no se convierte
        val v = TextFieldValue("Hola {nombre} adiós", TextRange(1, 8))
        val r = EstiloLetraUnicode.aplicarASeleccion(v, EstiloLetra.NEGRITA)
        assertEquals("H𝐨𝐥𝐚 {nombre} adiós", r.text)
        // selección que empieza dentro de la variable
        val w = TextFieldValue("Hola {nombre} adiós", TextRange(8, 16))
        val r2 = EstiloLetraUnicode.aplicarASeleccion(w, EstiloLetra.NEGRITA)
        assertEquals("Hola {nombre} 𝐚𝐝iós", r2.text)
        assertTrue(r2.text.contains("{nombre}"))
    }

    @Test fun elAvisoDiceLoQueHayQueDecir() {
        val t = EstiloLetraUnicode.AVISO
        assertTrue(t.contains("otros teléfonos") && t.contains("no se pueden buscar") && t.contains("lectores de pantalla"))
    }
}
