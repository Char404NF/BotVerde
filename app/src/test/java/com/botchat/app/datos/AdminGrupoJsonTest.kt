package com.botchat.app.datos

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/** Lee el JSON tal como lo escribe el módulo Go (muestras sacadas de su salida real). */
class AdminGrupoJsonTest {
    private val ana = """{"persona":"34600000001@s.whatsapp.net","jid":"34600000001@s.whatsapp.net","nombre":"Ana","telefono":"+34600000001","rol":"admin","rango":"Mod","rangoColor":"#2EC4CB","desdeTs":1794757269,"hastaTs":0,"mensajes":3,"ultimoTs":1794757269,"silenciado":false,"advertido":false,"etiquetas":[2],"foto":"","conteo":{"mensajes":3,"texto":1,"imagenes":1,"videos":0,"audios":0,"notasVoz":1,"stickers":0,"documentos":0,"otros":0,"borrados":1,"editados":2,"primerTs":1794757249,"ultimoTs":1794757269},"enLinea":true,"ultimaVez":5,"baneado":false,"advertencias":2}"""

    @Test fun leeLosMiembrosConSusContadores() {
        val l = JsonAdminGrupo.miembros("""{"desde":99,"miembros":[$ana,{"persona":"p2","jid":"p2","nombre":"Participante","telefono":"+34600000002","rol":"miembro","conteo":{"mensajes":0},"baneado":true}]}""")
        assertEquals(99L, l.desde)
        assertEquals(2, l.miembros.size)
        val a = l.miembros[0]
        assertEquals("Ana", a.titulo)
        assertTrue(a.esAdmin); assertFalse(a.esCreador)
        assertEquals(3, a.conteo.mensajes); assertEquals(1, a.conteo.notasVoz); assertEquals(1, a.conteo.borrados); assertEquals(2, a.conteo.editados)
        assertEquals(1794757269L, a.conteo.ultimoTs)
        assertTrue(a.enLinea); assertEquals(2, a.advertencias); assertEquals(listOf(2L), a.etiquetas)
        val b = l.miembros[1]
        assertEquals("+34600000002", b.titulo) // «Participante» no es un nombre: se muestra el número
        assertTrue(b.baneado); assertEquals(0, b.conteo.mensajes)
    }

    @Test fun leeElPerfilConSerieHorasEHistorial() {
        val serie = (1..30).joinToString(",") { """{"dia":"2026-11-${"%02d".format(it % 28 + 1)}","n":$it}""" }
        val horas = (0 until 24).joinToString(",") { "$it" }
        val p = JsonAdminGrupo.perfil("""{"miembro":$ana,"serie":[$serie],"horas":[$horas],"promedioDia":0.1,"mediaGrupo":3,"puesto":1,"activos":2,"motivoBan":"x","silencioHasta":0,"silenciado":true,"historial":[{"ts":5,"clase":"accion","tipo":"grupo.banear","texto":"baneó a","actor":"yo","actorNombre":"Yo","persona":"p","personaNombre":"","resultado":"error"}],"desde":7}""")
        assertEquals(30, p.serie.size); assertEquals(24, p.horas.size); assertEquals(23, p.horas.last())
        assertEquals(1, p.puesto); assertEquals(2, p.activos); assertEquals(3.0, p.mediaGrupo, 0.0001)
        assertTrue(p.silenciado); assertEquals("x", p.motivoBan)
        assertEquals(1, p.historial.size); assertTrue(p.historial[0].fallo)
    }

    @Test fun leeResumenBaneadosHorarioYEventos() {
        val r = JsonAdminGrupo.resumen("""{"miembros":2,"admins":1,"mensajesHoy":3,"mensajes7":3,"activos7":1,"inactivos30":1,"baneados":1,"silenciados":0,"advertidos":0,"serie":[{"dia":"2026-11-02","n":0}],"horario":{"grupo":"g","activo":true,"cierra":"22:00","abre":"07:00","dias":31,"ultimo":"cerrado"},"top":[$ana]}""")
        assertEquals(2, r.miembros); assertEquals(1, r.serie.size); assertEquals(1, r.top.size)
        assertTrue(r.horario.activo); assertEquals(31, r.horario.dias); assertEquals("cerrado", r.horario.ultimo)
        val b = JsonAdminGrupo.baneados("""[{"grupo":"g","persona":"p","nombre":"","telefono":"+34600000002","ts":1,"por":"yo","porNombre":"Yo","motivo":"spam"}]""")
        assertEquals("+34600000002", b[0].titulo); assertEquals("spam", b[0].motivo)
        assertEquals(0, JsonAdminGrupo.baneados("[]").size)
        assertEquals(0, JsonAdminGrupo.eventos("").size)
        val h = HorarioGrupo(true, "23:30", "06:00", 63)
        assertTrue(h.json().contains("\"cierra\":\"23:30\"") && h.json().contains("\"dias\":63") && h.json().contains("\"activo\":true"))
        assertFalse(JsonAdminGrupo.horario("{}").activo)
    }
}
