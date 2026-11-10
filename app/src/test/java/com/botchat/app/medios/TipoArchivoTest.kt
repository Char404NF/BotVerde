package com.botchat.app.medios

import com.botchat.app.medios.TipoArchivo.Familia
import org.junit.Assert.assertEquals
import org.junit.Test

class TipoArchivoTest {
    @Test fun extensionDelNombre() {
        assertEquals("pdf", TipoArchivo.extension("Factura Enero.PDF"))
        assertEquals("gz", TipoArchivo.extension("respaldo.tar.gz"))
        assertEquals("", TipoArchivo.extension("sin_extension"))
        assertEquals("", TipoArchivo.extension(".oculto"))
        assertEquals("", TipoArchivo.extension("termina."))
        assertEquals("", TipoArchivo.extension(""))
    }

    @Test fun cualquierMimeTieneNombreYExtensionQueMostrar() {
        assertEquals("informe.docx", TipoArchivo.nombreMostrable("informe.docx", ""))
        assertEquals("Documento.pdf", TipoArchivo.nombreMostrable("", "application/pdf"))
        assertEquals("Documento.xlsx", TipoArchivo.nombreMostrable("  ", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"))
        assertEquals("Documento", TipoArchivo.nombreMostrable("", "application/x-cosa-rara"))
        assertEquals("Documento", TipoArchivo.nombreMostrable("", ""))
    }

    @Test fun mimeParaAbrirConOtraApp() {
        assertEquals("application/pdf", TipoArchivo.mimeEfectivo("a.pdf", "application/pdf; charset=binary"))
        assertEquals("application/zip", TipoArchivo.mimeEfectivo("paquete.zip", ""))
        assertEquals("application/zip", TipoArchivo.mimeEfectivo("paquete.zip", "application/octet-stream"))
        assertEquals("*/*", TipoArchivo.mimeEfectivo("datos.xyz", "application/octet-stream"))
        assertEquals("*/*", TipoArchivo.mimeEfectivo("", ""))
        assertEquals("application/x-cosa-rara", TipoArchivo.mimeEfectivo("a.b", "Application/X-Cosa-Rara"))
    }

    @Test fun etiquetaYFamilia() {
        assertEquals("PDF", TipoArchivo.etiqueta("a.pdf", ""))
        assertEquals("XLSX", TipoArchivo.etiqueta("", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"))
        assertEquals("ARCH", TipoArchivo.etiqueta("sin", "application/octet-stream"))
        assertEquals("ABCD", TipoArchivo.etiqueta("a.abcdefgh", ""))
        assertEquals(Familia.PDF, TipoArchivo.familia("x", "application/pdf"))
        assertEquals(Familia.HOJA, TipoArchivo.familia("cuentas.csv", ""))
        assertEquals(Familia.TEXTO, TipoArchivo.familia("carta.docx", ""))
        assertEquals(Familia.PRESENTACION, TipoArchivo.familia("deck.pptx", ""))
        assertEquals(Familia.COMPRIMIDO, TipoArchivo.familia("a.rar", ""))
        assertEquals(Familia.APP, TipoArchivo.familia("app.apk", ""))
        assertEquals(Familia.OTRO, TipoArchivo.familia("raro.zzz", ""))
    }
}
