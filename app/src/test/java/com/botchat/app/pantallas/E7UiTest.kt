package com.botchat.app.pantallas

import androidx.activity.ComponentActivity
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.performTextReplacement
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.assertIsNotEnabled
import androidx.compose.ui.test.SemanticsNodeInteraction
import androidx.compose.ui.semantics.SemanticsProperties
import com.botchat.app.datos.Ajustes
import com.botchat.app.datos.Repo
import com.botchat.app.ui.BotChatTema
import com.botchat.app.ui.PantallaActividadPersona
import com.botchat.app.ui.PantallaCambiosMiembros
import com.botchat.app.ui.PantallaChat
import com.botchat.app.ui.PantallaInfoGrupo
import com.botchat.app.ui.PantallaPerfilPersona
import com.botchat.app.ui.PantallaPersonas
import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/** Etapa E7 (0.6.0): perfil de persona, actividad por chat, info del grupo, puntitos en el chat y en Personas. */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], application = android.app.Application::class, qualifiers = "w411dp-h1600dp-xxhdpi")
class E7UiTest {
    @get:Rule val regla = createAndroidComposeRule<ComponentActivity>()
    private lateinit var motor: MotorFalso
    private var bot = 0L
    private val ana = "5215512345678@s.whatsapp.net"
    private val beto = "5215587654321@s.whatsapp.net"
    private val g1 = "120363000000000001@g.us"
    private val g2 = "120363000000000002@g.us"
    private val ahora = System.currentTimeMillis() / 1000

    @Before fun antes() {
        motor = Apoyo.reinicia(); bot = Apoyo.creaBot()
        Ajustes.get(Apoyo.ctx()).aceptarAviso()
        motor.chat = Apoyo.chatJson(ana, "Ana Pérez").toString()
        motor.perfilJson = JSONObject().put("persona", ana).put("nombre", "Ana Pérez").put("telefono", "+52 55 1234 5678").put("rango", "Ayudante").put("rangoColor", "#8E4EC6")
            .put("foto", "").put("mensajes", 42).put("archivos", 7).put("archivosBytes", 2_000_000L)
            .put("etiquetas", JSONArray().put(JSONObject().put("id", 77).put("nombre", "VIP").put("color", "#F59E28").put("emoji", "⭐")))
            .put("gruposComun", JSONArray().put(JSONObject().put("jid", g1).put("nombre", "Vecinos del 5").put("foto", "")))
            .put("ficha", JSONObject(Apoyo.fichaEjemplo(ana, "Ana Pérez"))).toString()
        motor.actividadJson = JSONArray()
            .put(tarjeta(ana, "Ana Pérez", true, 10, fotos = 2, videos = 1, archivos = 1, enlaces = 2))
            .put(tarjeta(g1, "Vecinos del 5", false, 30, audios = 4)).toString()
        motor.gruposAdmin = JSONArray()
            .put(JSONObject().put("jid", g1).put("nombre", "Vecinos del 5").put("miembros", 30).put("soyAdmin", true).put("sabido", true))
            .put(JSONObject().put("jid", g2).put("nombre", "Familia").put("miembros", 8).put("soyAdmin", false).put("sabido", true)).toString()
        motor.fichaJson = Apoyo.fichaEjemplo(ana, "Ana Pérez")
    }

    private fun tarjeta(chat: String, nombre: String, privado: Boolean, msgs: Int, fotos: Int = 0, videos: Int = 0, audios: Int = 0, archivos: Int = 0, enlaces: Int = 0) =
        JSONObject().put("chat", chat).put("nombre", nombre).put("foto", "").put("privado", privado).put("miembro", true).put("mensajes", msgs).put("fotos", fotos)
            .put("videos", videos).put("audios", audios).put("archivos", archivos).put("enlaces", enlaces).put("misEnvios", 0).put("ultimoTs", ahora)

    private fun llamo(p: String) = motor.llamadas.toList().any { it.startsWith(p) }
    private fun hay(tag: String) = regla.onAllNodes(androidx.compose.ui.test.hasTestTag(tag), useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    private fun seleccionado(tag: String, si: Boolean) = regla.espera {
        regla.onAllNodes(androidx.compose.ui.test.hasTestTag(tag) and (if (si) androidx.compose.ui.test.isSelected() else androidx.compose.ui.test.isNotSelected()), useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()
    }
    private fun toca(tag: String) { regla.esperaEtiqueta(tag); regla.onNodeWithTag(tag, useUnmergedTree = true).performClick() }
    private fun perfil(vm: (String) -> Unit = {}, ver: (String) -> Unit = {}, abrir: (String, String, String) -> Unit = { _, _, _ -> }) =
        regla.setContent { BotChatTema { PantallaPerfilPersona(bot, ana, {}, alAbrirActividad = abrir, alVerMensajes = ver, alAbrirGrupo = vm) } }

    private fun silencio(color: String, usado: Int, hasta: Long = 0) = JSONArray().put(
        JSONObject().put("id", 1).put("persona", ana).put("nombre", "Ana Pérez").put("alcance", "todos").put("grupos", JSONArray()).put("hastaTs", hasta).put("creadoTs", ahora)
            .put("origen", "manual").put("avisar", true).put("texto", "").put("activo", true)
            .put("contador", JSONObject().put("usado", usado).put("tope", 10).put("color", color).put("total", usado).put("avisos", 0).put("grupos", JSONArray())),
    ).toString()

    // ---------- perfil ----------

    @Test fun elPerfilMuestraFotoNombreNumeroRangoEtiquetasGruposYContadores() {
        perfil()
        regla.esperaTexto("Ana Pérez")
        listOf("perfil_foto", "perfil_numero", "perfil_rango", "etiqueta_77", "grupos_en_comun", "comun_$g1", "perfil_contadores").forEach { assertTrue(it, hay(it)) }
        regla.onNodeWithTag("perfil_contadores", useUnmergedTree = true).assertTextContainsAll("42 mensajes", "7 archivos")
        listOf("boton_silenciar", "boton_advertir", "boton_marcar", "boton_ver_mensajes").forEach { assertTrue(it, hay(it)) }
        assertFalse("sin silencio no hay tarjeta", hay("tarjeta_silencio"))
    }

    @Test fun laActividadTieneTarjetaDelChatPrivadoYUnaPorGrupo() {
        var abierto = Triple("", "", "")
        perfil(abrir = { p, c, n -> abierto = Triple(p, c, n) })
        regla.esperaEtiqueta("actividad_privado")
        assertTrue(hay("actividad_$g1"))
        toca("actividad_$g1")
        regla.espera { abierto.second == g1 }
        assertEquals(ana, abierto.first); assertEquals("Vecinos del 5", abierto.third)
        toca("actividad_privado")
        regla.espera { abierto.second == ana }
        assertEquals("Chat privado", abierto.third)
    }

    @Test fun verMensajesAbreElChatPrivado() {
        var chat = ""
        perfil(ver = { chat = it })
        toca("boton_ver_mensajes")
        regla.espera { chat == ana }
    }

    @Test fun elSilencioMuestraLaBarraVerde() {
        motor.silenciosGruposJson = silencio("verde", 2)
        perfil()
        regla.esperaEtiqueta("barra_silencio_verde")
        regla.onNodeWithTag("texto_contador", useUnmergedTree = true).assertTextContainsAll("2 de 10")
        regla.onNodeWithTag("silencio_estado", useUnmergedTree = true).assertTextContainsAll("para siempre")
    }

    @Test fun elSilencioMuestraLaBarraAmarilla() {
        motor.silenciosGruposJson = silencio("amarillo", 8)
        perfil()
        regla.esperaEtiqueta("barra_silencio_amarillo")
        regla.onNodeWithTag("texto_contador", useUnmergedTree = true).assertTextContainsAll("8 de 10")
    }

    @Test fun elSilencioMuestraLaBarraRoja() {
        motor.silenciosGruposJson = silencio("rojo", 10)
        perfil()
        regla.esperaEtiqueta("barra_silencio_rojo")
        regla.onNodeWithTag("texto_contador", useUnmergedTree = true).assertTextContainsAll("10 de 10")
    }

    @Test fun ponerEnCeroYQuitarSilencioConDeshacer() {
        motor.silenciosGruposJson = silencio("amarillo", 8, hasta = ahora + 7200)
        perfil()
        toca("poner_en_cero")
        regla.espera { llamo("contadorCero:$ana") }
        toca("quitar_silencio_grupos")
        regla.espera { llamo("quitarSilencioGrupos:$ana") }
        regla.esperaTexto("Quité el silencio")
        regla.onNodeWithText("Deshacer").performClick()
        regla.espera { llamo("silenciarGrupos:$ana:") }
        val l = motor.llamadas.first { it.startsWith("silenciarGrupos:$ana:") }.split(":")
        assertTrue("vuelve con el tiempo que quedaba", l[2].toLong() in 7000L..7200L)
        assertEquals("todos", l[3])
    }

    // ---------- Silenciar ----------

    @Test fun silenciarUnaSemanaEnVariosGruposMuestraDondeNoSoyAdmin() {
        perfil()
        toca("boton_silenciar")
        toca("sil_dur_1s")
        toca("sil_alcance_varios")
        regla.esperaEtiqueta("sil_grupo_$g2")
        assertTrue("el grupo sin admin se muestra con su aviso", hay("sil_noadmin_$g2"))
        assertFalse(hay("sil_noadmin_$g1"))
        toca("sil_check_$g1")
        toca("sil_confirmar")
        regla.espera { llamo("silenciarGrupos:$ana:604800:varios") }
        assertTrue(motor.llamadas.any { it.contains("[\"$g1\"]") })
    }

    @Test fun silenciarParaSiempreEnTodosLosQueAdministroUsaSoloLosAdmin() {
        perfil()
        toca("boton_silenciar")
        regla.esperaEtiqueta("sil_todos_texto")
        regla.onNodeWithTag("sil_todos_texto", useUnmergedTree = true).assertTextContainsAll("Vecinos del 5")
        toca("sil_dur_siempre")
        toca("sil_confirmar")
        regla.espera { llamo("silenciarGrupos:$ana:0:todos:[]:true:") }
    }

    @Test fun silenciarConTiempoPersonalizadoYTextoPropio() {
        perfil()
        toca("boton_silenciar")
        toca("sil_dur_otro")
        regla.esperaEtiqueta("sil_cantidad")
        regla.onNodeWithTag("sil_cantidad", useUnmergedTree = true).performTextReplacement("3")
        toca("sil_unidad_h")
        regla.onNodeWithTag("sil_texto", useUnmergedTree = true).performTextInput(" Calma, por favor.")
        toca("sil_confirmar")
        regla.espera { llamo("silenciarGrupos:$ana:") }
        val l = motor.llamadas.first { it.startsWith("silenciarGrupos:") }
        assertTrue(l, l.startsWith("silenciarGrupos:$ana:10800:todos"))
        assertTrue("el texto editado viaja", l.contains("Calma, por favor."))
    }

    @Test fun elBotonSilenciarNoHaceNadaSinGruposNiAdmin() {
        motor.gruposAdmin = "[]"
        perfil()
        toca("boton_silenciar")
        regla.esperaEtiqueta("sil_confirmar")
        regla.espera { hay("sil_todos_texto") }
        regla.onNodeWithTag("sil_confirmar", useUnmergedTree = true).assertIsNotEnabled()
        assertFalse(llamo("silenciarGrupos"))
    }

    // ---------- Advertir ----------

    private fun tipos() {
        motor.tiposAdvJson = JSONArray()
            .put(JSONObject().put("id", 1).put("nombre", "Spam").put("emoji", "📢").put("color", "#F59E28").put("posicion", 0).put("caducidadDias", -1).put("reglaId", 0).put("activo", true))
            .put(JSONObject().put("id", 2).put("nombre", "Insultos").put("emoji", "🤬").put("color", "#D64545").put("posicion", 1).put("caducidadDias", -1).put("reglaId", 0).put("activo", true)).toString()
    }

    @Test fun advertirConTipoMuestraBotonesGrandesYDiceQuePaso() {
        tipos()
        motor.resultadoAdvTipo = """{"total":1,"nivel":1,"paso":"avisar","hecho":"avisado"}"""
        perfil()
        toca("boton_advertir")
        regla.esperaEtiqueta("adv_tipo_2")
        regla.onNodeWithTag("adv_confirmar", useUnmergedTree = true).assertIsNotEnabled()
        toca("adv_tipo_2")
        toca("adv_confirmar")
        regla.espera { llamo("advertirTipo:$ana:2:") }
        regla.esperaEtiqueta("adv_paso")
        regla.onNodeWithTag("adv_paso", useUnmergedTree = true).assertTextContainsAll("aviso")
        toca("adv_deshacer")
        regla.espera { llamo("quitarAdvertencia:$ana:0") }
    }

    @Test fun advertirEnUnGrupoOfreceLasReglasDeEseGrupo() {
        tipos()
        motor.reglasPorGrupo[g1] = JSONArray().put(JSONObject().put("id", 7).put("grupo", g1).put("pos", 1).put("titulo", "Sin enlaces").put("texto", "No se mandan enlaces")).toString()
        perfil()
        toca("boton_advertir")
        regla.esperaEtiqueta("adv_donde_$g1")
        assertFalse("sin grupo no hay reglas", hay("adv_regla_7"))
        toca("adv_donde_$g1")
        regla.esperaEtiqueta("adv_regla_7")
        toca("adv_regla_7")
        toca("adv_tipo_1")
        toca("adv_confirmar")
        regla.espera { llamo("advertirTipo:$ana:1:$g1:7") }
    }

    // ---------- Marcar ----------

    @Test fun marcarUnaPersonaPonePeroTambienQuitaLaEtiqueta() {
        val id = motor.crearEtiquetaEmoji(bot, "Cliente", "#2EC4CB", "🛒")
        perfil()
        toca("boton_marcar")
        toca("marcar_etiqueta_$id")
        regla.espera { llamo("etiquetaPersona:$ana:$id:true") }
        seleccionado("marcar_etiqueta_$id", true)
        toca("marcar_etiqueta_$id")
        regla.espera { llamo("etiquetaPersona:$ana:$id:false") }
    }

    // ---------- Actividad ----------

    private fun pagina(vararg items: JSONObject, fin: Boolean = true, sigTs: Long = 0, sigId: String = "", total: Long = items.size.toLong()) =
        JSONObject().put("total", total).put("items", JSONArray().also { a -> items.forEach { a.put(it) } }).put("fin", fin).put("sigTs", sigTs).put("sigId", sigId).put("escaneados", 0).toString()

    private fun item(id: String, texto: String, roja: String = "", por: String = "", versiones: Int = 0, marcado: Boolean = false, chat: String = g1, mio: Boolean = false, tipo: String = "texto") =
        Apoyo.msgJson(id, ahora - 100, texto, tipo = tipo, remitente = "Ana", mio = mio).put("chat", chat).put("marcaRoja", roja).put("borradoPor", por).put("versiones", versiones)
            .put("marcado", marcado).put("etiquetas", JSONArray()).apply { if (roja == "borrado") put("borrado", true); if (roja == "editado") put("editado", true) }

    private fun actividad(chat: String = g1, titulo: String = "Vecinos del 5", irA: (String, String) -> Unit = { _, _ -> }) =
        regla.setContent { BotChatTema { PantallaActividadPersona(bot, ana, chat, titulo, {}, irA) } }

    @Test fun laActividadTieneCuatroPestanasYPideCadaUna() {
        motor.paginaPersona = pagina(item("m1", "hola vecinos"))
        actividad()
        regla.esperaTexto("hola vecinos")
        listOf("mensajes", "fotos", "audios", "archivos").forEach { assertTrue(it, hay("pestana_$it")) }
        toca("pestana_fotos")
        regla.espera { motor.filtrosPersona.any { it.contains("\"pestana\":\"fotos\"") } }
        toca("pestana_archivos")
        regla.espera { motor.filtrosPersona.any { it.contains("\"pestana\":\"archivos\"") } }
        assertTrue(motor.filtrosPersona.all { it.contains("\"persona\":\"$ana\"") && it.contains("\"chat\":\"$g1\"") })
    }

    @Test fun filtrosPorFechaPalabraYSoloBorrados() {
        motor.paginaPersona = pagina(item("m1", "hola"))
        actividad()
        regla.esperaTexto("hola")
        toca("fecha_7")
        regla.espera { motor.filtrosPersona.any { JSONObject(it).optLong("desde") > 0 } }
        toca("solo_borrados")
        regla.espera { motor.filtrosPersona.any { it.contains("\"soloBorrados\":true") } }
        regla.onNodeWithTag("actividad_palabra", useUnmergedTree = true).performTextInput("camion")
        regla.espera { motor.filtrosPersona.any { it.contains("\"palabra\":\"camion\"") } }
    }

    @Test fun losBorradosYEditadosLlevanMarcaRojaYLasVersionesSeVenAlTocar() {
        motor.paginaPersona = pagina(item("m1", "normal"), item("m2", "ya no está", roja = "borrado", por = "autor"), item("m3", "texto nuevo", roja = "editado", versiones = 2), item("m4", "lo borró un admin", roja = "borrado", por = "Beto"))
        motor.versionesJson = JSONArray().put(JSONObject().put("n", 1).put("ts", ahora - 50).put("texto", "texto viejo uno")).put(JSONObject().put("n", 2).put("ts", ahora - 40).put("texto", "texto viejo dos")).toString()
        actividad()
        regla.esperaTexto("normal")
        assertFalse(hay("marca_roja_m1"))
        listOf("m2", "m3", "m4").forEach { assertTrue(it, hay("marca_roja_$it")) }
        regla.onNodeWithTag("texto_marca_roja_m2", useUnmergedTree = true).assertTextContainsAll("Borrado por quien lo escribió")
        regla.onNodeWithTag("texto_marca_roja_m4", useUnmergedTree = true).assertTextContainsAll("Beto")
        regla.onNodeWithTag("texto_marca_roja_m3", useUnmergedTree = true).assertTextContainsAll("Editado")
        assertFalse(hay("versiones_m3"))
        toca("ver_antes_m3")
        regla.esperaTexto("texto viejo uno", substring = true)
        regla.esperaTexto("texto viejo dos", substring = true)
    }

    @Test fun marcarUnMensajeConEtiquetaYQuitarLaMarca() {
        val id = motor.crearEtiquetaEmoji(bot, "Importante", "#D64545", "❗")
        motor.paginaPersona = pagina(item("m1", "hola vecinos"))
        actividad()
        toca("marcar_m1")
        toca("marcarmsg_etiqueta_$id")
        regla.espera { llamo("marcar:$g1:m1:$id") }
        seleccionado("marcarmsg_etiqueta_$id", true)
        toca("marcarmsg_etiqueta_$id")
        regla.espera { llamo("quitarMarca:$g1:m1:$id") }
        regla.onNodeWithTag("marcarmsg_nota", useUnmergedTree = true).performTextInput("revisar")
        toca("marcarmsg_guardar_nota")
        regla.espera { llamo("marcar:$g1:m1:0") }
    }

    @Test fun laPaginacionPideLaSiguientePaginaConElCursor() {
        motor.paginasPersona += pagina(item("m1", "primero"), fin = false, sigTs = 55, sigId = "m1", total = 2)
        motor.paginaPersona = pagina(item("m2", "segundo"), total = 2)
        actividad()
        regla.esperaTexto("primero")
        regla.esperaTexto("segundo")
        assertTrue(motor.filtrosPersona.size >= 2)
    }

    @Test fun borrarDelGrupoPideConfirmacionSinDeshacer() {
        motor.paginaPersona = pagina(item("m1", "mensaje ajeno"))
        actividad()
        toca("borrar_ajeno_m1")
        regla.esperaEtiqueta("aviso_sin_deshacer")
        assertFalse("no borra hasta confirmar", llamo("grupoBorrarMensaje"))
        toca("confirmar_borrar_ajeno_cancelar")
        regla.esperaSinEtiqueta("aviso_sin_deshacer")
        assertFalse(llamo("grupoBorrarMensaje"))
        toca("borrar_ajeno_m1")
        toca("confirmar_borrar_ajeno")
        regla.espera { llamo("grupoBorrarMensaje:m1") }
    }

    @Test fun enElChatPrivadoNoSeOfreceBorrarDelGrupo() {
        motor.paginaPersona = pagina(item("m1", "hola", chat = ana))
        actividad(ana, "Chat privado")
        regla.esperaTexto("hola")
        assertFalse(hay("borrar_ajeno_m1"))
    }

    @Test fun irAlMensajeAbreElChatEnEseMensaje() {
        var ir = "" to ""
        motor.paginaPersona = pagina(item("m1", "hola"))
        actividad(irA = { c, i -> ir = c to i })
        toca("ir_chat_m1")
        regla.espera { ir == g1 to "m1" }
    }

    // ---------- Info del grupo ----------

    private fun miembros() {
        motor.crearEtiquetaEmoji(bot, "Vecino", "#2EC4CB", "🏠")
        motor.miembrosJson = JSONArray()
            .put(miembro(ana, "Ana Pérez", "admin", 30, silenciado = true, rango = "Ayudante", etiquetas = listOf(1L)))
            .put(miembro(beto, "Beto Ruiz", "miembro", 5, advertido = true))
            .put(miembro("5215500000003@s.whatsapp.net", "Carla", "creador", 99)).toString()
    }

    private fun miembro(p: String, n: String, rol: String, msgs: Int, silenciado: Boolean = false, advertido: Boolean = false, rango: String = "", etiquetas: List<Long> = emptyList()) =
        JSONObject().put("persona", p).put("jid", p).put("nombre", n).put("telefono", "").put("rol", rol).put("rango", rango).put("rangoColor", "#8E4EC6").put("desdeTs", 1).put("hastaTs", 0)
            .put("mensajes", msgs).put("ultimoTs", ahora).put("silenciado", silenciado).put("advertido", advertido).put("etiquetas", JSONArray(etiquetas)).put("foto", "")

    private fun infoGrupo(abrir: (String, String) -> Unit = { _, _ -> }, cambios: () -> Unit = {}) {
        motor.chat = Apoyo.chatJson(g1, "Vecinos del 5", grupo = true).toString()
        regla.setContent { BotChatTema { PantallaInfoGrupo(bot, g1, {}, alAbrirMiembro = abrir, alVerCambios = cambios) } }
    }

    @Test fun infoDelGrupoMuestraMiembrosConInsigniaIconosYContador() {
        miembros()
        infoGrupo()
        regla.esperaEtiqueta("miembro_$ana")
        assertTrue(hay("rango_$ana")); assertTrue(hay("etiqueta_1"))
        assertTrue("silenciada", hay("icono_silenciado_$ana")); assertFalse(hay("icono_silenciado_$beto"))
        assertTrue("advertido", hay("icono_advertido_$beto")); assertFalse(hay("icono_advertido_$ana"))
        regla.onNodeWithTag("contador_$ana", useUnmergedTree = true).assertTextContainsAll("30 mensajes")
        regla.onNodeWithTag("contador_$beto", useUnmergedTree = true).assertTextContainsAll("5 mensajes")
        assertEquals("admin y creador llevan insignia", 2, regla.onAllNodes(androidx.compose.ui.test.hasTestTag("insignia_admin"), useUnmergedTree = true).fetchSemanticsNodes().size)
        regla.onNodeWithTag("grupo_detalle", useUnmergedTree = true).assertTextContainsAll("Soy admin")
    }

    @Test fun tocarUnMiembroAbreSuActividadEnElGrupo() {
        miembros()
        var abierto = "" to ""
        infoGrupo(abrir = { p, n -> abierto = p to n })
        toca("miembro_$beto")
        regla.espera { abierto.first == beto }
        assertEquals("Beto Ruiz", abierto.second)
    }

    @Test fun verCambiosEnLosMiembrosAbreSuPantalla() {
        miembros()
        var cambiosAbiertos = false
        infoGrupo(cambios = { cambiosAbiertos = true })
        toca("ver_cambios_miembros")
        regla.espera { cambiosAbiertos }
    }

    @Test fun laPantallaDeCambiosDiceQuienEntroSalioOFueSacado() {
        motor.cambiosJson = JSONArray()
            .put(JSONObject().put("id", 1).put("grupo", g1).put("ts", ahora).put("tipo", "expulsado").put("persona", beto).put("nombre", "Beto").put("por", ana).put("porNombre", "Ana").put("via", ""))
            .put(JSONObject().put("id", 2).put("grupo", g1).put("ts", ahora - 10).put("tipo", "entra").put("persona", "x").put("nombre", "Dani").put("por", "").put("porNombre", "").put("via", ""))
            .put(JSONObject().put("id", 3).put("grupo", g1).put("ts", ahora - 20).put("tipo", "sale").put("persona", "y").put("nombre", "Eli").put("por", "").put("porNombre", "").put("via", "")).toString()
        regla.setContent { BotChatTema { PantallaCambiosMiembros(bot, g1, {}) } }
        regla.esperaTexto("Beto fue sacado por Ana")
        regla.esperaTexto("Dani entró")
        regla.esperaTexto("Eli salió")
    }

    @Test fun ajustesConservaLoQueYaHabia() {
        miembros()
        infoGrupo()
        toca("pestana_ajustes")
        regla.esperaEtiqueta("tarjeta_administrar_grupo")
        assertTrue(hay("tarjeta_politica_grupo")); assertTrue(hay("tarjeta_resumen_grupo")); assertTrue(hay("abrir_grupo"))
    }

    @Test fun sinListaDeMiembrosOfrecePedirla() {
        motor.miembrosJson = "[]"
        infoGrupo()
        regla.esperaEtiqueta("miembros_vacio")
        toca("actualizar_miembros")
        regla.espera { llamo("actualizarMiembros") }
    }

    // ---------- puntitos ----------

    @Test fun elChatDeGrupoPintaElPuntitoDeColorEnUnaConsultaPorLotes() {
        val id = motor.crearEtiquetaEmoji(bot, "Vecino", "#2EC4CB", "🏠")
        motor.chat = Apoyo.chatJson(g1, "Equipo", tel = "", pais = "", grupo = true).toString()
        motor.mensajes = Apoyo.arreglo(
            Apoyo.msgJson("g3", ahora, "tres", remitente = "Luis", remitenteId = "luis"),
            Apoyo.msgJson("g2", ahora - 5, "dos", remitente = "Ana", remitenteId = "ana"),
            Apoyo.msgJson("g1", ahora - 10, "uno", remitente = "Ana", remitenteId = "ana"),
        )
        motor.puntitosJson = JSONObject().put("g3", JSONArray().put(id)).toString()
        regla.setContent { BotChatTema { PantallaChat(g1, {}, {}, {}, {}) } }
        regla.esperaTexto("tres")
        regla.esperaEtiqueta("puntito_g3")
        assertFalse(hay("puntito_g2")); assertFalse(hay("puntito_g1"))
        assertTrue("una consulta por lote, no una por mensaje: ${motor.consultasPuntitos}", motor.consultasPuntitos in 1..2)
    }

    @Test fun cuandoCambianLasEtiquetasSeVuelveAPedirLaCache() {
        val id = motor.crearEtiquetaEmoji(bot, "Vecino", "#2EC4CB", "🏠")
        motor.chat = Apoyo.chatJson(g1, "Equipo", tel = "", pais = "", grupo = true).toString()
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("g3", ahora, "tres", remitente = "Luis", remitenteId = "luis"))
        regla.setContent { BotChatTema { PantallaChat(g1, {}, {}, {}, {}) } }
        regla.esperaTexto("tres")
        regla.espera { motor.consultasPuntitos >= 1 }
        assertFalse(hay("puntito_g3"))
        motor.puntitosJson = JSONObject().put("g3", JSONArray().put(id)).toString()
        regla.runOnUiThread { Repo.emitirParaPruebas("etiquetas") }
        regla.esperaEtiqueta("puntito_g3")
    }

    @Test fun enUnChatPrivadoNoSeConsultanPuntitos() {
        motor.mensajes = Apoyo.arreglo(Apoyo.msgJson("m1", ahora, "hola", remitente = "Ana"))
        regla.setContent { BotChatTema { PantallaChat(ana, {}, {}, {}, {}) } }
        regla.esperaTexto("hola")
        assertEquals(0, motor.consultasPuntitos)
    }

    // ---------- Personas con iconos ----------

    @Test fun lasFilasDePersonasMuestranSilenciadaYAdvertida() {
        motor.chatsPersonales = Apoyo.arreglo(Apoyo.chatJson(ana, "Ana Pérez"), Apoyo.chatJson(beto, "Beto Ruiz", tel = "+52 55 8765 4321"))
        motor.estadosPersonas = JSONObject().put(ana, JSONObject().put("silenciado", true).put("ignorada", false).put("advertido", false))
            .put(beto, JSONObject().put("silenciado", false).put("ignorada", false).put("advertido", true)).toString()
        regla.setContent { BotChatTema { PantallaPersonas(bot, {}) } }
        regla.esperaEtiqueta("silenciada_$ana")
        regla.esperaEtiqueta("advertida_$beto")
        assertFalse(hay("advertida_$ana")); assertFalse(hay("silenciada_$beto"))
    }

    private fun SemanticsNodeInteraction.assertTextContainsAll(vararg trozos: String) {
        val texto = fetchSemanticsNode().config.let { c -> if (SemanticsProperties.Text in c) c[SemanticsProperties.Text].joinToString(" ") { it.text } else "" }
        trozos.forEach { assertTrue("«$texto» no contiene «$it»", texto.contains(it)) }
    }

    private fun androidx.compose.ui.test.junit4.ComposeContentTestRule.esperaSinEtiqueta(tag: String) =
        espera { onAllNodes(androidx.compose.ui.test.hasTestTag(tag), useUnmergedTree = true).fetchSemanticsNodes().isEmpty() }
}
