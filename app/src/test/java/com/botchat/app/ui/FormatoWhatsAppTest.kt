package com.botchat.app.ui

import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.style.TextDecoration
import com.botchat.app.datos.Mencion
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class FormatoWhatsAppTest {
    private fun construye(t: String, m: List<Mencion> = emptyList()) = FormatoWhatsApp.construir(t, m)

    @Test fun negritaCursivaTachadoYMonoSeAplicanYSeQuitanLosMarcadores() {
        val a = construye("hola *fuerte* y _suave_ y ~viejo~ y ```codigo```")
        assertEquals("hola fuerte y suave y viejo y codigo", a.text)
        assertEquals(FontWeight.Bold, a.spanStyles.first { a.text.substring(it.start, it.end) == "fuerte" }.item.fontWeight)
        assertEquals(FontStyle.Italic, a.spanStyles.first { a.text.substring(it.start, it.end) == "suave" }.item.fontStyle)
        assertEquals(TextDecoration.LineThrough, a.spanStyles.first { a.text.substring(it.start, it.end) == "viejo" }.item.textDecoration)
        assertTrue(a.spanStyles.any { a.text.substring(it.start, it.end) == "codigo" && it.item.fontFamily != null })
    }

    @Test fun losFormatosSePuedenAnidar() {
        val a = construye("*negrita y _cursiva_*")
        assertEquals("negrita y cursiva", a.text)
        assertTrue(a.spanStyles.any { it.item.fontWeight == FontWeight.Bold })
        assertTrue(a.spanStyles.any { it.item.fontStyle == FontStyle.Italic && a.text.substring(it.start, it.end) == "cursiva" })
    }

    @Test fun losMarcadoresSueltosONoPegadosNoSeTocan() {
        assertEquals("2 * 3 * 4", construye("2 * 3 * 4").text)
        assertEquals("snake_case_name", construye("snake_case_name").text)
        assertEquals("*sin cerrar", construye("*sin cerrar").text)
        assertEquals("a*b*c", construye("a*b*c").text)
        assertEquals("** vacío", construye("** vacío").text)
        assertEquals("*uno\ndos*", construye("*uno\ndos*").text)
    }

    @Test fun losEnlacesSeVuelvenTocablesSinLaPuntuacionFinal() {
        val a = construye("mira https://ejemplo.com/a?b=1, o www.sitio.mx.")
        val ls = a.getLinkAnnotations(0, a.length)
        assertEquals(2, ls.size)
        assertEquals("https://ejemplo.com/a?b=1", (ls[0].item as androidx.compose.ui.text.LinkAnnotation.Url).url)
        assertEquals("https://www.sitio.mx", (ls[1].item as androidx.compose.ui.text.LinkAnnotation.Url).url)
        assertEquals("www.sitio.mx", a.text.substring(ls[1].start, ls[1].end))
    }

    @Test fun lasMencionesMuestranElNombreEnVezDelNumero() {
        val a = construye("hola @5215522222222 y @999", listOf(Mencion("5215522222222", "Ana"), Mencion("999", "Beto")))
        assertEquals("hola @Ana y @Beto", a.text)
        assertEquals(2, a.spanStyles.count { it.item.fontWeight == FontWeight.SemiBold })
        assertEquals("@Ana", construye("@5215522222222", listOf(Mencion("5215522222222", "Ana"))).text)
        assertEquals("@123456 queda igual", construye("@123456 queda igual", listOf(Mencion("999", "Beto"))).text)
    }

    @Test fun elNombreConAsteriscosNoRompeElFormato() {
        val a = construye("*hola* @777", listOf(Mencion("777", "Z*ñ")))
        assertEquals("hola @Z*ñ", a.text)
    }

    @Test fun soloEmojisSeDetectaBien() {
        assertTrue(FormatoWhatsApp.esSoloEmojis("😀"))
        assertTrue(FormatoWhatsApp.esSoloEmojis("😀😂 👍"))
        assertTrue(FormatoWhatsApp.esSoloEmojis("👨‍👩‍👧"))
        assertTrue(FormatoWhatsApp.esSoloEmojis("👍🏽"))
        assertFalse(FormatoWhatsApp.esSoloEmojis("😀😂👍🙏"))
        assertFalse(FormatoWhatsApp.esSoloEmojis("hola 😀"))
        assertFalse(FormatoWhatsApp.esSoloEmojis("123"))
        assertFalse(FormatoWhatsApp.esSoloEmojis(""))
    }

    @Test fun losBotonesDeFormatoEnvuelvenYDesenvuelven() {
        val sel = TextFieldValue("hola mundo", TextRange(5, 10))
        val negrita = FormatoWhatsApp.aplicar(sel, "*")
        assertEquals("hola *mundo*", negrita.text)
        assertEquals(TextRange(6, 11), negrita.selection)
        assertEquals("hola mundo", FormatoWhatsApp.aplicar(negrita, "*").text)
        val vacio = FormatoWhatsApp.aplicar(TextFieldValue("ab", TextRange(1)), "_")
        assertEquals("a__b", vacio.text)
        assertEquals(TextRange(2), vacio.selection)
        assertEquals("```x```", FormatoWhatsApp.aplicar(TextFieldValue("x", TextRange(0, 1)), "```").text)
        assertNull(null)
    }
}
