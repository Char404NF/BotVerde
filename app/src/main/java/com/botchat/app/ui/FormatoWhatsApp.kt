package com.botchat.app.ui

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.LinkAnnotation
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextLinkStyles
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.TextRange
import com.botchat.app.datos.Mencion

/**
 * Formato de WhatsApp: *negrita*, _cursiva_, ~tachado~ y ```monoespacio```, más enlaces tocables y @menciones con el nombre.
 * Un marcador solo vale si va pegado al texto que encierra y no a mitad de una palabra; no cruza renglones.
 */
object FormatoWhatsApp {
    private val enlace = Regex("""(?i)\b((?:https?://|www\.)[^\s<>]+)""")

    private fun estiloDe(c: Char) = when (c) {
        '*' -> SpanStyle(fontWeight = FontWeight.Bold)
        '_' -> SpanStyle(fontStyle = FontStyle.Italic)
        else -> SpanStyle(textDecoration = TextDecoration.LineThrough)
    }

    private fun puedeAbrir(s: String, i: Int, c: Char): Boolean =
        (i == 0 || !s[i - 1].isLetterOrDigit()) && i + 1 < s.length && !s[i + 1].isWhitespace() && s[i + 1] != c

    private fun cierre(s: String, i: Int, c: Char): Int {
        var j = i + 2
        while (j < s.length) {
            val x = s[j]
            if (x == '\n') return -1
            if (x == c && !s[j - 1].isWhitespace() && (j + 1 == s.length || !s[j + 1].isLetterOrDigit())) return j
            j++
        }
        return -1
    }

    private fun AnnotatedString.Builder.pinta(s: String) {
        var i = 0
        while (i < s.length) {
            if (s.startsWith("```", i)) {
                val fin = s.indexOf("```", i + 3)
                if (fin > i + 3) {
                    withStyle(SpanStyle(fontFamily = FontFamily.Monospace)) { append(s.substring(i + 3, fin).trim('\n')) }
                    i = fin + 3
                    continue
                }
            }
            val c = s[i]
            if ((c == '*' || c == '_' || c == '~') && puedeAbrir(s, i, c)) {
                val j = cierre(s, i, c)
                if (j > 0) {
                    withStyle(estiloDe(c)) { pinta(s.substring(i + 1, j)) }
                    i = j + 1
                    continue
                }
            }
            append(c)
            i++
        }
    }

    /** Texto con formato, menciones (usuario → nombre) y enlaces. */
    fun construir(texto: String, menciones: List<Mencion> = emptyList(), colorEnlace: Color = Color(0xFF1E88E5), colorMencion: Color = colorEnlace): AnnotatedString {
        val base = buildAnnotatedString { pinta(texto) }
        val conMenciones = if (menciones.isEmpty()) base else {
            val nombres = menciones.associate { it.usuario to it.nombre.ifBlank { it.usuario } }
            val re = Regex("@(" + nombres.keys.joinToString("|") { Regex.escape(it) } + ")")
            buildAnnotatedString {
                var pos = 0
                for (m in re.findAll(base.text)) {
                    append(base.subSequence(pos, m.range.first))
                    withStyle(SpanStyle(color = colorMencion, fontWeight = FontWeight.SemiBold)) { append("@" + nombres[m.groupValues[1]]) }
                    pos = m.range.last + 1
                }
                append(base.subSequence(pos, base.length))
            }
        }
        val enlaces = enlace.findAll(conMenciones.text).toList()
        if (enlaces.isEmpty()) return conMenciones
        return buildAnnotatedString {
            append(conMenciones)
            for (m in enlaces) {
                var url = m.value.trimEnd('.', ',', ';', ':', '!', '?', ')', ']', '"', '\'')
                if (url.isEmpty()) continue
                val destino = if (url.startsWith("www.", ignoreCase = true)) "https://$url" else url
                addLink(LinkAnnotation.Url(destino, TextLinkStyles(SpanStyle(color = colorEnlace, textDecoration = TextDecoration.Underline))), m.range.first, m.range.first + url.length)
            }
        }
    }

    /** ¿El mensaje es solo de 1 a 3 emojis? Entonces se muestran grandes. */
    fun esSoloEmojis(texto: String): Boolean {
        val t = texto.trim()
        if (t.isEmpty() || t.length > 40) return false
        var grupos = 0
        var tras = false // el caracter anterior fue un unidor (ZWJ): forma parte del mismo emoji
        var i = 0
        while (i < t.length) {
            val cp = t.codePointAt(i)
            i += Character.charCount(cp)
            when {
                cp == 0x200D -> { tras = true; continue }
                cp == 0xFE0F || cp == 0x20E3 || cp in 0x1F3FB..0x1F3FF || cp in 0xE0020..0xE007F -> continue
                Character.isWhitespace(cp) -> continue
                esEmoji(cp) -> { if (!tras) grupos++; tras = false }
                else -> return false
            }
        }
        return grupos in 1..3
    }

    private fun esEmoji(cp: Int): Boolean =
        cp in 0x1F300..0x1FAFF || cp in 0x2600..0x27BF || cp in 0x1F000..0x1F2FF || cp in 0x2B00..0x2BFF || cp in 0x1F1E6..0x1F1FF ||
            cp == 0x00A9 || cp == 0x00AE || cp in 0x2190..0x21FF || cp == 0x203C || cp == 0x2049 || cp == 0x2122 || cp in 0x2300..0x23FF || cp in 0x25A0..0x25FF || cp == 0x3030 || cp == 0x303D

    // ---------- escribir: botones de formato ----------

    /** Envuelve la selección con un marcador; sin selección, mete el par y deja el cursor en medio. */
    fun aplicar(v: TextFieldValue, marcador: String): TextFieldValue {
        val ini = v.selection.min
        val fin = v.selection.max
        val t = v.text
        return if (ini == fin) {
            TextFieldValue(t.substring(0, ini) + marcador + marcador + t.substring(fin), TextRange(ini + marcador.length))
        } else {
            val sel = t.substring(ini, fin)
            val m = marcador.length
            if (ini >= m && fin + m <= t.length && t.substring(ini - m, ini) == marcador && t.substring(fin, fin + m) == marcador) {
                // la selección ya está envuelta por fuera: se quitan los marcadores de alrededor
                TextFieldValue(t.substring(0, ini - m) + sel + t.substring(fin + m), TextRange(ini - m, fin - m))
            } else if (sel.startsWith(marcador) && sel.endsWith(marcador) && sel.length >= 2 * marcador.length) {
                val sin = sel.substring(marcador.length, sel.length - marcador.length)
                TextFieldValue(t.substring(0, ini) + sin + t.substring(fin), TextRange(ini, ini + sin.length))
            } else {
                TextFieldValue(t.substring(0, ini) + marcador + sel + marcador + t.substring(fin), TextRange(ini + marcador.length, fin + marcador.length))
            }
        }
    }
}
