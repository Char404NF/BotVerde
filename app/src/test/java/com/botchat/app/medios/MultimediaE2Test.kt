package com.botchat.app.medios

import android.graphics.Bitmap
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import java.io.File

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class)
class MultimediaE2Test {
    private class OrigenFalso(val pend: List<GeneradorPortadas.Pendiente>) : GeneradorPortadas.Origen {
        val guardadas = mutableListOf<Triple<String, String, ByteArray>>()
        override fun pendientes(limite: Int) = pend.take(limite)
        override fun guardar(chat: String, id: String, jpeg: ByteArray, ancho: Int, alto: Int) { guardadas += Triple(chat, id, jpeg) }
    }

    @Test fun generaPortadasJpegYNoReintentaLasFallidas() {
        val origen = OrigenFalso(listOf(
            GeneradorPortadas.Pendiente("c", "ok", "media/1/a/ok.enc"),
            GeneradorPortadas.Pendiente("c", "mal", "media/1/a/mal.enc"),
        ))
        var llamadas = 0
        var ladoPedido = 0
        val gen = GeneradorPortadas(File("."), ByteArray(32), origen, { f, _, lado, _ ->
            llamadas++; ladoPedido = lado
            if (f.name == "ok.enc") Bitmap.createBitmap(320, 180, Bitmap.Config.ARGB_8888) else null
        })
        assertEquals(1, gen.pasada())
        assertEquals(720, ladoPedido)
        assertEquals(1, origen.guardadas.size)
        val j = origen.guardadas[0].third
        assertTrue("debe ser un JPEG", j.size > 4 && j[0] == 0xFF.toByte() && j[1] == 0xD8.toByte())
        // Segunda pasada: el que falló no se vuelve a intentar; el que salió bien sigue en la lista del origen falso y se rehace.
        val antes = llamadas
        gen.pasada()
        assertEquals("solo se reintenta el que no falló", antes + 1, llamadas)
    }

    @Test fun elDestinoDeLaGaleriaDependeDelTipo() {
        assertEquals("Pictures/BotChat", Exportar.destinoDe("image/jpeg").carpeta)
        assertEquals("Movies/BotChat", Exportar.destinoDe("video/mp4").carpeta)
        assertEquals("Music/BotChat", Exportar.destinoDe("audio/ogg").carpeta)
        assertEquals("Documents/BotChat", Exportar.destinoDe("application/pdf").carpeta)
        assertTrue(Exportar.AVISO_SIN_CIFRAR.contains("SIN cifrar"))
    }
}
