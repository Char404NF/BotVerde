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

    // ---------- los 8 estilos de WhatsApp (etapa D) ----------

    @Test fun losOchoEstilosDeWhatsAppSePintan() {
        // 1 *negrita*, 2 _cursiva_, 3 ~tachado~, 4 ```monoespaciado```, 5 "- " lista, 6 "1. " numerada, 7 "> " cita, 8 `código en línea`
        val a = construye("*n* _c_ ~t~ ```m``` `l`")
        assertEquals("n c t m l", a.text)
        assertEquals(FontWeight.Bold, a.spanStyles.first { a.text.substring(it.start, it.end) == "n" }.item.fontWeight)
        assertEquals(FontStyle.Italic, a.spanStyles.first { a.text.substring(it.start, it.end) == "c" }.item.fontStyle)
        assertEquals(TextDecoration.LineThrough, a.spanStyles.first { a.text.substring(it.start, it.end) == "t" }.item.textDecoration)
        assertTrue(a.spanStyles.any { a.text.substring(it.start, it.end) == "m" && it.item.fontFamily != null })
        assertTrue(a.spanStyles.any { a.text.substring(it.start, it.end) == "l" && it.item.fontFamily != null })
    }

    @Test fun laListaConGuionSeVuelveVineta() {
        assertEquals("• uno\n• dos", construye("- uno\n- dos").text)
        assertEquals("a - b", construye("a - b").text) // un guion en medio no es lista
        assertEquals("-sin espacio", construye("-sin espacio").text)
        assertEquals("- ", construye("- ").text) // un renglón sin nada más no es lista
        // dentro de una lista se sigue aplicando el formato
        val l = construye("- *fuerte* y _suave_")
        assertEquals("• fuerte y suave", l.text)
        assertTrue(l.spanStyles.any { it.item.fontWeight == FontWeight.Bold })
    }

    @Test fun laListaNumeradaMantieneSuNumero() {
        val l = construye("1. uno\n2. dos\n10. diez")
        assertEquals("1. uno\n2. dos\n10. diez", l.text)
        assertEquals(3, l.spanStyles.count { it.item.fontWeight == FontWeight.SemiBold })
        assertEquals("1.sin espacio", construye("1.sin espacio").text)
        assertEquals("versión 2. ya", construye("versión 2. ya").text) // solo al principio del renglón
    }

    @Test fun laCitaSeMarcaYAdmiteFormato() {
        val c = construye("> dijo *esto*\ny sigue")
        assertEquals("▎ dijo esto\ny sigue", c.text)
        assertTrue(c.spanStyles.any { it.item.fontStyle == FontStyle.Italic && it.start == 0 })
        assertTrue(c.spanStyles.any { it.item.fontWeight == FontWeight.Bold && c.text.substring(it.start, it.end) == "esto" })
        assertEquals("a > b", construye("a > b").text)
        assertEquals(">pegado", construye(">pegado").text)
    }

    @Test fun elCodigoEnLineaNoAplicaOtrosFormatosNiCruzaRenglones() {
        val c = construye("usa `*no negrita*` aquí")
        assertEquals("usa *no negrita* aquí", c.text)
        assertTrue(c.spanStyles.none { it.item.fontWeight == FontWeight.Bold })
        assertEquals("`sin cerrar", construye("`sin cerrar").text)
        assertEquals("`uno\ndos`", construye("`uno\ndos`").text)
        assertEquals("``", construye("``").text)
        // el monoespaciado de tres comillas puede ocupar varios renglones
        assertEquals("a\nb", construye("```a\nb```").text)
    }

    @Test fun losBotonesDeRenglonPonenYQuitanLista_numeradaYCita() {
        val v = TextFieldValue("uno\ndos\ntres", TextRange(0, 13))
        val lista = FormatoWhatsApp.aplicarLinea(v, "- ")
        assertEquals("- uno\n- dos\n- tres", lista.text)
        assertEquals("uno\ndos\ntres", FormatoWhatsApp.aplicarLinea(lista, "- ").text)
        val num = FormatoWhatsApp.aplicarLinea(v, "1. ")
        assertEquals("1. uno\n2. dos\n3. tres", num.text)
        assertEquals("uno\ndos\ntres", FormatoWhatsApp.aplicarLinea(num, "1. ").text)
        assertEquals("> uno\ndos\ntres", FormatoWhatsApp.aplicarLinea(TextFieldValue("uno\ndos\ntres", TextRange(1)), "> ").text)
        // solo los renglones tocados por la selección
        assertEquals("uno\n- dos\ntres", FormatoWhatsApp.aplicarLinea(TextFieldValue("uno\ndos\ntres", TextRange(5)), "- ").text)
        assertEquals("- ", FormatoWhatsApp.aplicarLinea(TextFieldValue("", TextRange(0)), "- ").text)
    }
}
