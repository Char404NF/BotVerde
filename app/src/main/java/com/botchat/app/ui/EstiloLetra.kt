package com.botchat.app.ui

import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.input.TextFieldValue

/** Estilos de letra Unicode del “Aa Estilo de letra” (Avanzado). */
enum class EstiloLetra(val titulo: String) {
    NORMAL("Normal"),
    NEGRITA("Negrita"),
    CURSIVA("Cursiva"),
    SCRIPT("Script"),
    GOTICA("Gótica"),
    DOBLE("Doble línea"),
    MAQUINA("Máquina de escribir"),
    BURBUJAS("Burbujas"),
    VERSALITAS("Versalitas"),
}

/**
 * Convierte letras a su versión Unicode de otro estilo. Reglas:
 * - solo cambia A–Z, a–z y 0–9; los acentos y la ñ se quedan normales (no existen en esos alfabetos);
 * - no toca las variables entre llaves ({nombre}, {hora}…): tienen que seguir funcionando;
 * - convertir lo ya convertido no cambia nada, y “Normal” lo devuelve a letras comunes.
 * Puede verse distinto en otros teléfonos, no se puede buscar y los lectores de pantalla lo leen mal.
 */
object EstiloLetraUnicode {
    const val AVISO = "Las letras especiales pueden verse distinto en otros teléfonos, no se pueden buscar y los lectores de pantalla las leen mal. Úsalas con medida."

    private fun rango(base: Int, n: Int) = IntArray(n) { base + it }

    // Letras mayúsculas (26), minúsculas (26) y números (10) de cada alfabeto, como puntos de código. 0 = igual que el normal.
    private class Alfabeto(val may: IntArray, val min: IntArray, val num: IntArray?)

    private fun conAgujeros(base: Int, n: Int, agujeros: Map<Int, Int>) = IntArray(n) { agujeros[it] ?: (base + it) }

    private val negrita = Alfabeto(rango(0x1D400, 26), rango(0x1D41A, 26), rango(0x1D7CE, 10))
    private val cursiva = Alfabeto(rango(0x1D434, 26), conAgujeros(0x1D44E, 26, mapOf(7 to 0x210E)), null) // la h cursiva vive aparte
    private val script = Alfabeto(
        conAgujeros(0x1D49C, 26, mapOf(1 to 0x212C, 4 to 0x2130, 5 to 0x2131, 7 to 0x210B, 8 to 0x2110, 11 to 0x2112, 12 to 0x2133, 17 to 0x211B)),
        conAgujeros(0x1D4B6, 26, mapOf(4 to 0x212F, 6 to 0x210A, 14 to 0x2134)), null,
    )
    private val gotica = Alfabeto(conAgujeros(0x1D504, 26, mapOf(2 to 0x212D, 7 to 0x210C, 8 to 0x2111, 17 to 0x211C, 25 to 0x2128)), rango(0x1D51E, 26), null)
    private val doble = Alfabeto(conAgujeros(0x1D538, 26, mapOf(2 to 0x2102, 7 to 0x210D, 13 to 0x2115, 15 to 0x2119, 16 to 0x211A, 17 to 0x211D, 25 to 0x2124)), rango(0x1D552, 26), rango(0x1D7D8, 10))
    private val maquina = Alfabeto(rango(0x1D670, 26), rango(0x1D68A, 26), rango(0x1D7F6, 10))
    private val burbujas = Alfabeto(rango(0x24B6, 26), rango(0x24D0, 26), IntArray(10) { if (it == 0) 0x24EA else 0x2460 + it - 1 })
    // Versalitas: las minúsculas pasan a versalitas; las mayúsculas y los números se quedan.
    private val versalitas = Alfabeto(
        IntArray(0),
        intArrayOf(0x1D00, 0x0299, 0x1D04, 0x1D05, 0x1D07, 0xA730, 0x0262, 0x029C, 0x026A, 0x1D0A, 0x1D0B, 0x029F, 0x1D0D, 0x0274, 0x1D0F, 0x1D18, 0x01EB, 0x0280, 0xA731, 0x1D1B, 0x1D1C, 0x1D20, 0x1D21, 'x'.code, 0x028F, 0x1D22),
        null,
    )

    private fun alfabeto(e: EstiloLetra): Alfabeto? = when (e) {
        EstiloLetra.NORMAL -> null
        EstiloLetra.NEGRITA -> negrita
        EstiloLetra.CURSIVA -> cursiva
        EstiloLetra.SCRIPT -> script
        EstiloLetra.GOTICA -> gotica
        EstiloLetra.DOBLE -> doble
        EstiloLetra.MAQUINA -> maquina
        EstiloLetra.BURBUJAS -> burbujas
        EstiloLetra.VERSALITAS -> versalitas
    }

    /** De cualquier letra especial a la letra común (para “Normal” y para poder cambiar de estilo sobre lo ya convertido). */
    private val inversa: Map<Int, Char> by lazy {
        val m = HashMap<Int, Char>()
        for (e in EstiloLetra.values()) {
            val a = alfabeto(e) ?: continue
            a.may.forEachIndexed { i, cp -> if (cp != 0) m[cp] = 'A' + i }
            a.min.forEachIndexed { i, cp -> if (cp != 0 && cp != 'x'.code) m[cp] = 'a' + i }
            a.num?.forEachIndexed { i, cp -> if (cp != 0) m[cp] = '0' + i }
        }
        m
    }

    private fun aComun(cp: Int): Int = inversa[cp]?.code ?: cp

    private fun convierteUno(cp: Int, a: Alfabeto?): Int {
        val c = aComun(cp)
        if (a == null) return c
        return when (c) {
            in 'A'.code..'Z'.code -> if (a.may.isEmpty()) c else a.may[c - 'A'.code]
            in 'a'.code..'z'.code -> a.min[c - 'a'.code]
            in '0'.code..'9'.code -> a.num?.get(c - '0'.code) ?: c
            else -> c
        }
    }

    /** Convierte todo el texto, saltándose lo que esté entre llaves. */
    fun aplicar(texto: String, estilo: EstiloLetra): String {
        val a = alfabeto(estilo)
        val sb = StringBuilder()
        var i = 0
        while (i < texto.length) {
            if (texto[i] == '{') {
                val fin = texto.indexOf('}', i)
                if (fin > i && texto.substring(i + 1, fin).all { it.isLetterOrDigit() || it == '_' }) {
                    sb.append(texto, i, fin + 1)
                    i = fin + 1
                    continue
                }
            }
            val cp = texto.codePointAt(i)
            i += Character.charCount(cp)
            sb.appendCodePoint(convierteUno(cp, a))
        }
        return sb.toString()
    }

    /**
     * Convierte la selección de un campo (si no hay selección, no hace nada). Las variables que quedan dentro de la selección no se
     * tocan; la selección queda marcada sobre el texto nuevo.
     */
    fun aplicarASeleccion(v: TextFieldValue, estilo: EstiloLetra): TextFieldValue {
        val ini = v.selection.min
        val fin = v.selection.max
        if (ini == fin) return v
        // las llaves pueden quedar partidas por la selección: se amplía para no cortar una variable por la mitad
        var a = ini
        var b = fin
        val abre = v.text.lastIndexOf('{', a)
        val cierra = v.text.indexOf('}', abre.coerceAtLeast(0))
        if (abre in 0 until a && cierra >= a && v.text.substring(abre + 1, cierra).all { it.isLetterOrDigit() || it == '_' }) a = abre
        val abre2 = v.text.lastIndexOf('{', b - 1)
        val cierra2 = if (abre2 >= 0) v.text.indexOf('}', abre2) else -1
        if (abre2 in 0 until b && cierra2 >= b && v.text.substring(abre2 + 1, cierra2).all { it.isLetterOrDigit() || it == '_' }) b = cierra2 + 1
        val nuevo = aplicar(v.text.substring(a, b), estilo)
        return TextFieldValue(v.text.substring(0, a) + nuevo + v.text.substring(b), TextRange(a, a + nuevo.length))
    }

    /** ¿Hay letras especiales en el texto? (para avisar al guardar). */
    fun tieneEspeciales(texto: String): Boolean {
        var i = 0
        while (i < texto.length) {
            val cp = texto.codePointAt(i)
            i += Character.charCount(cp)
            if (inversa.containsKey(cp)) return true
        }
        return false
    }
}
