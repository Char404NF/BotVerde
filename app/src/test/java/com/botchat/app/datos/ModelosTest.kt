package com.botchat.app.datos

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/** Los JSON de prueba tienen exactamente el formato que entrega el módulo Go. */
class ModelosTest {
    @Test fun chats() {
        val l = Json.chats("""[{"jid":"5215512345678@s.whatsapp.net","nombre":"Ana","telefono":"+52 55 1234 5678","pais":"MX","esGrupo":false,"foto":"avatars/1/ab.enc","fotoTs":99,"ultimoTs":1700000000,"ultimoTexto":"hola","ultimoTipo":"texto","ultimoBorrado":false,"ultimoMio":true},
            {"jid":"g@g.us","nombre":"Familia","telefono":"","pais":"","esGrupo":true,"foto":"","fotoTs":0,"ultimoTs":5,"ultimoTexto":"","ultimoTipo":"imagen","ultimoBorrado":true,"ultimoMio":false}]""")
        assertEquals(2, l.size)
        assertEquals("Ana", l[0].nombre); assertEquals("MX", l[0].pais); assertTrue(l[0].ultimoMio); assertEquals(99L, l[0].fotoTs)
        assertTrue(l[1].esGrupo); assertTrue(l[1].ultimoBorrado); assertEquals("imagen", l[1].ultimoTipo)
        assertTrue(Json.chats("[]").isEmpty())
        assertTrue(Json.chats("").isEmpty())
    }

    @Test fun chatIndividual() {
        assertNull(Json.chatOpt("null")); assertNull(Json.chatOpt(""))
        val c = Json.chatOpt("""{"jid":"a@s.whatsapp.net","nombre":"Beto","telefono":"+34 612 34 56 78","pais":"ES","esGrupo":false}""")!!
        assertEquals("Beto", c.nombre); assertEquals("ES", c.pais)
    }

    @Test fun mensajes() {
        val m = Json.mensajes("""[{"id":"A1","chat":"c","remitenteId":"yo","remitente":"","esMio":true,"tipo":"imagen","texto":"pie","ts":10,"borrado":false,"editado":true,
            "estado":"descargado","bytes":1234,"mime":"image/jpeg","nombre":"","ruta":"media/1/ab/A1.enc","miniatura":"media/1/ab/A1.thumb.enc","ancho":800,"alto":600,"dur":0,"notaVoz":false,"puedeBajarse":false},
            {"id":"A2","chat":"c","remitenteId":"x","remitente":"Luis","esMio":false,"tipo":"audio","texto":"","ts":11,"borrado":false,"editado":false,"estado":"omitido","bytes":50,"mime":"audio/ogg","nombre":"","ruta":"","miniatura":"","ancho":0,"alto":0,"dur":7,"notaVoz":true,"puedeBajarse":true}]""")
        assertEquals("foto", m[0].categoria); assertTrue(m[0].descargado); assertTrue(m[0].editado)
        assertEquals("audio", m[1].categoria); assertFalse(m[1].descargado); assertTrue(m[1].puedeBajarse); assertTrue(m[1].notaVoz)
        assertEquals("", categoriaDe("texto")); assertEquals("foto", categoriaDe("sticker")); assertEquals("documento", categoriaDe("documento"))
    }

    @Test fun espacio() {
        val t = Json.espacioTotal("""{"multimedia":1000,"miniaturas":50,"fotos":20,"base":30,"libre":5000,"total":10000}""")
        assertEquals(1100L, t.ocupaBotChat); assertEquals(5000L, t.libre)
        val porTipo = Json.espacioPorTipo("""[{"categoria":"foto","bytes":900,"cantidad":3},{"categoria":"video","bytes":100,"cantidad":1}]""")
        assertEquals(2, porTipo.size); assertEquals(900L, porTipo[0].bytes)
        val chats = Json.espacioPorChat("""[{"jid":"a","nombre":"Ana","foto":"","fotoTs":0,"esGrupo":false,"pais":"MX","bytes":1000,"cantidad":4,"porTipo":[600,400,0,0]}]""")
        assertEquals(listOf(600L, 400L, 0L, 0L), chats[0].porTipo)
        assertEquals(12L, Json.liberado("""{"bytes":12,"cantidad":3}""").bytes)
        val r = Json.reporte("""{"estadosCorregidos":0,"tamanosCorregidos":0,"archivosDanados":0,"miniaturasLimpiadas":0,"fotosLimpiadas":0,"huerfanos":0,"bytesHuerfanos":0,"chatsRecalculados":0}""")
        assertTrue(r.sinCambios)
        assertFalse(Json.reporte("""{"huerfanos":2,"bytesHuerfanos":10}""").sinCambios)
    }

    @Test fun jsonDeSalida() {
        // El orden de los campos no importa para el módulo Go: se compara el contenido.
        val f = org.json.JSONObject(Json.filtro("c", "video", 30))
        assertEquals("c", f.getString("chat")); assertEquals("video", f.getString("categoria")); assertEquals(30, f.getInt("diasMin"))
        val a = org.json.JSONArray(Json.items(listOf("a" to "1", "b" to "2")))
        assertEquals(2, a.length()); assertEquals("b", a.getJSONObject(1).getString("chat")); assertEquals("2", a.getJSONObject(1).getString("id"))
    }
}
