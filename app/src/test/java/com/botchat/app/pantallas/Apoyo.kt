package com.botchat.app.pantallas

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import com.botchat.app.Datos
import com.botchat.app.datos.Ajustes
import com.botchat.app.datos.Motor
import com.botchat.app.datos.Repo
import com.botchat.app.medios.CacheImagenes
import com.botchat.app.seguridad.ClaveDatos
import org.json.JSONArray
import org.json.JSONObject
import wabot.Observador
import java.io.File

/** Motor de ejemplo: devuelve JSON con el mismo formato que el módulo Go y anota lo que la pantalla le pide. */
class MotorFalso : Motor {
    var chatsPersonales = "[]"
    var chatsGrupos = "[]"
    var chat = "null"
    var mensajes = "[]"
    var archivos = "[]"
    var espacioTotal = """{"multimedia":0,"miniaturas":0,"fotos":0,"base":0,"libre":0,"total":0}"""
    var espacioPorTipo = "[]"
    var espacioPorChat = "[]"
    var liberar = """{"bytes":0,"cantidad":0}"""
    var reporte = """{"estadosCorregidos":0,"tamanosCorregidos":0,"archivosDanados":0,"miniaturasLimpiadas":0,"fotosLimpiadas":0,"huerfanos":0,"bytesHuerfanos":0,"chatsRecalculados":0}"""
    val llamadas = mutableListOf<String>()
    val consultas = mutableListOf<String>()

    override fun abrirAlmacen(raiz: String, clave: ByteArray, o: Observador?) { llamadas += "abrirAlmacen" }
    override fun listarChats(bot: Long, grupos: Boolean, consulta: String, limite: Long, desde: Long): String {
        consultas += consulta
        val todos = JSONArray(if (grupos) chatsGrupos else chatsPersonales)
        if (consulta.isBlank()) return todos.toString()
        val q = consulta.lowercase()
        return JSONArray().also { a -> for (i in 0 until todos.length()) if (todos.getJSONObject(i).getString("nombre").lowercase().contains(q)) a.put(todos.get(i)) }.toString()
    }
    override fun obtenerChat(bot: Long, chat: String) = this.chat
    override fun listarMensajes(bot: Long, chat: String, antesTs: Long, antesId: String, limite: Long): String {
        llamadas += "mensajes:$chat:$antesTs"
        return if (antesTs > 0) "[]" else mensajes
    }
    override fun listarArchivos(bot: Long, chat: String, categoria: String, soloDescargados: Boolean, limite: Long, desde: Long): String {
        val a = JSONArray(archivos)
        if (categoria.isEmpty()) return a.toString()
        return JSONArray().also { r -> for (i in 0 until a.length()) { val o = a.getJSONObject(i); if (o.optString("tipo").let { t -> (t == "imagen" || t == "sticker") && categoria == "foto" || t == categoria }) r.put(o) } }.toString()
    }
    override fun espacioTotal(bot: Long) = espacioTotal
    override fun espacioPorTipo(bot: Long) = espacioPorTipo
    override fun espacioPorChat(bot: Long) = espacioPorChat
    override fun estimarBorrado(bot: Long, filtro: String): String { llamadas += "estimar:$filtro"; return liberar }
    override fun borrarArchivos(bot: Long, filtro: String): String { llamadas += "borrarArchivos:" + JSONObject(filtro).let { "${it.optString("chat")}|${it.optString("categoria")}|${it.optInt("diasMin")}" }; return liberar }
    override fun estimarLista(bot: Long, items: String) = liberar
    override fun borrarLista(bot: Long, items: String): String { llamadas += "borrarLista:$items"; return liberar }
    override fun borrarChat(bot: Long, chat: String): String { llamadas += "borrarChat:$chat"; return liberar }
    override fun reparar(bot: Long): String { llamadas += "reparar"; return reporte }
    override fun descargar(bot: Long, chat: String, id: String) { llamadas += "descargar:$chat:$id" }
    override fun fotoGrande(bot: Long, chat: String) = throw IllegalStateException("Este chat no tiene foto")
    var resumenChat = """{"desde":0,"listo":true,"enviados":0,"recibidos":0,"porTipo":[],"porHora":[],"porDiaSemana":[],"quien":[]}"""
    var calendario = """{"desde":0,"listo":true,"dias":[]}"""
    var porDia = mutableMapOf<String, String>()
    var globales = """{"desde":0,"listo":true,"enviados":0,"recibidos":0,"chats":0,"porTipo":[],"activos":[],"porDia":[]}"""
    var zona = ""
    var filtrados = """{"total":0,"items":[]}"""
    var personas = "[]"
    override fun listarFiltrados(bot: Long, chat: String, filtro: String, antesTs: Long, antesId: String, limite: Long): String { llamadas += "filtro:$filtro"; return if (antesTs > 0) """{"total":0,"items":[]}""" else filtrados }
    override fun personasChat(bot: Long, chat: String) = personas
    var envioResultado = """{"id":"NUEVO1","estado":"enviado","ts":1}"""
    var envioFalla: String? = null
    var presencia = """{"escribiendo":false,"enLinea":false,"ultimaVez":0}"""
    var participantes = "[]"
    var info = """{"enviado":0,"estado":"enviado","esGrupo":false,"personas":[],"editado":false,"reenviado":false}"""
    var borrador = ""
    var estadoChat = """{"silenciado":false,"botPausaHasta":0,"botActivo":true}"""
    var plazo = 1200L
    var reenvio = """{"enviados":1,"omitidos":0,"fallidos":0}"""
    private fun falla() { envioFalla?.let { throw IllegalStateException(it) } }
    override fun enviarTexto(bot: Long, chat: String, texto: String, citaId: String, mencionesJson: String): String { llamadas += "enviar:$chat:$texto:$citaId:$mencionesJson"; falla(); return envioResultado }
    override fun reintentarEnvio(bot: Long, chat: String, id: String): String { llamadas += "reintentar:$id"; falla(); return envioResultado }
    override fun reaccionar(bot: Long, chat: String, id: String, emoji: String) { llamadas += "reaccion:$id:$emoji"; falla() }
    override fun editarMensaje(bot: Long, chat: String, id: String, texto: String) { llamadas += "editar:$id:$texto"; falla() }
    override fun borrarParaTodos(bot: Long, chat: String, id: String) { llamadas += "borrarTodos:$id"; falla() }
    override fun borrarParaMi(bot: Long, chat: String, idsJson: String): Long { llamadas += "borrarMi:$idsJson"; return 1 }
    override fun reenviarTexto(bot: Long, origen: String, idsJson: String, destinosJson: String): String { llamadas += "reenviar:$idsJson:$destinosJson"; falla(); return reenvio }
    override fun marcarLeido(bot: Long, chat: String) { llamadas += "leido:$chat" }
    override fun escribiendo(bot: Long, chat: String, si: Boolean) { llamadas += "escribiendo:$si" }
    override fun enLinea(bot: Long, si: Boolean) { llamadas += "enLinea:$si" }
    override fun suscribirPresencia(bot: Long, chat: String) { llamadas += "suscribir:$chat" }
    override fun presenciaDe(chat: String) = presencia
    override fun participantesGrupo(bot: Long, chat: String) = participantes
    override fun infoMensaje(bot: Long, chat: String, id: String) = info
    override fun guardarBorrador(bot: Long, chat: String, texto: String) { llamadas += "borrador:$texto"; borrador = texto }
    override fun leerBorrador(bot: Long, chat: String) = borrador
    override fun silenciarChat(bot: Long, chat: String, si: Boolean) { llamadas += "silenciar:$si" }
    override fun estadoDelChat(bot: Long, chat: String) = estadoChat
    override fun plazoEdicionSeg() = plazo
    override fun configurarEnvio(pausaBotMin: Long) { llamadas += "configurarEnvio:$pausaBotMin" }
    var verUnaVezJson = """{"ruta":"media/1/ab/vo.enc","tipo":"imagen","mime":"image/jpeg"}"""
    var verUnaVezFalla: String? = null
    override fun enviarMedio(bot: Long, chat: String, tipo: String, ruta: String, mime: String, nombre: String, pie: String, citaId: String, ancho: Long, alto: Long, durS: Long, notaVoz: Boolean, miniatura: ByteArray?, onda: ByteArray?): String {
        llamadas += "medio:$tipo:$mime:$nombre:$pie:$citaId:$durS:$notaVoz:${miniatura?.size ?: 0}:${onda?.size ?: 0}"
        falla(); return envioResultado
    }
    override fun abrirVerUnaVez(bot: Long, chat: String, id: String): String { llamadas += "abrirUnaVez:$id"; verUnaVezFalla?.let { throw IllegalStateException(it) }; return verUnaVezJson }
    override fun cerrarVerUnaVez(bot: Long, chat: String, id: String) { llamadas += "cerrarUnaVez:$id" }
    override fun barrerVerUnaVez(bot: Long): Long { llamadas += "barrer"; return 0 }
    override fun configurarVerUnaVez(dias: Long) { llamadas += "verDias:$dias" }
    override fun olvidarMensaje(bot: Long, chat: String, id: String) { llamadas += "olvidar:$chat:$id" }
    override fun establecerZona(id: String) { zona = id }
    override fun estadisticasChat(bot: Long, chat: String) = resumenChat
    override fun calendarioChat(bot: Long, chat: String, mes: String): String { llamadas += "calendario:$mes"; return calendario }
    override fun mensajesDia(bot: Long, chat: String, dia: String): String { llamadas += "dia:$dia"; return porDia[dia] ?: "[]" }
    override fun estadisticasGlobales(bot: Long, dias: Long) = globales
}

object Apoyo {
    val clave = ByteArray(32) { (it + 1).toByte() }

    fun ctx(): Context = ApplicationProvider.getApplicationContext()

    /** Deja todo limpio: base local, ajustes, llave en memoria y motor de ejemplo. */
    fun reinicia(motor: MotorFalso = MotorFalso()): MotorFalso {
        Datos.reiniciarParaPruebas()
        ctx().deleteDatabase("botchat.db") // cada prueba empieza sin bot, aunque la anterior haya creado uno
        ctx().filesDir.listFiles()?.forEach { it.deleteRecursively() }
        ctx().cacheDir.listFiles()?.forEach { it.deleteRecursively() }
        Datos.get(ctx()).let { d -> d.lista().forEach { d.eliminar(it.id) } } // por si la base de la prueba anterior sigue ahí
        Ajustes.reiniciarParaPruebas()
        ctx().getSharedPreferences("ajustes", Context.MODE_PRIVATE).edit().clear().commit()
        ctx().getSharedPreferences("seguridad", Context.MODE_PRIVATE).edit().clear().commit()
        ctx().getSharedPreferences("vistos", Context.MODE_PRIVATE).edit().clear().commit()
        ctx().getSharedPreferences("notificados", Context.MODE_PRIVATE).edit().clear().commit()
        com.botchat.app.BotState.estado.value = "detenido"; com.botchat.app.BotState.corriendo.value = false
        com.botchat.app.ui.ArranqueLogo.mostrado = true // sin splash salvo en su propia prueba
        com.botchat.app.ui.EntradaLista.hecha = true
        com.botchat.app.ui.EstadoBloqueo.bloqueado.value = false
        ClaveDatos.paraPruebas(clave)
        CacheImagenes.vaciar()
        Repo.paraPruebas(ctx().filesDir, motor)
        return motor
    }

    /** Crea el bot (ya vinculado) y devuelve su id. */
    fun creaBot(vinculado: Boolean = true): Long {
        val d = Datos.get(ctx())
        val id = d.crear("Mi bot", "MX", 52, "5512345678", "525512345678", false)
        if (vinculado) d.marcarVinculado(id)
        return id
    }

    fun chatJson(jid: String, nombre: String, tel: String = "+52 55 1234 5678", pais: String = "MX", grupo: Boolean = false, tipo: String = "texto", texto: String = "hola", ts: Long = System.currentTimeMillis() / 1000, borrado: Boolean = false, mio: Boolean = false) =
        JSONObject().put("jid", jid).put("nombre", nombre).put("telefono", tel).put("pais", pais).put("esGrupo", grupo).put("foto", "").put("fotoTs", 0).put("ultimoTs", ts)
            .put("ultimoTexto", texto).put("ultimoTipo", tipo).put("ultimoBorrado", borrado).put("ultimoMio", mio)

    fun msgJson(
        id: String, ts: Long, texto: String = "", tipo: String = "texto", mio: Boolean = false, remitente: String = "", remitenteId: String = "x", estado: String = "ninguno",
        bytes: Long = 0, ruta: String = "", miniatura: String = "", borrado: Boolean = false, editado: Boolean = false, puedeBajarse: Boolean = false, dur: Int = 0, nombre: String = "", notaVoz: Boolean = false,
    ) = JSONObject().put("id", id).put("chat", "chat").put("remitenteId", remitenteId).put("remitente", remitente).put("esMio", mio).put("tipo", tipo).put("texto", texto).put("ts", ts)
        .put("borrado", borrado).put("editado", editado).put("estado", estado).put("bytes", bytes).put("mime", "").put("nombre", nombre).put("ruta", ruta).put("miniatura", miniatura)
        .put("ancho", 0).put("alto", 0).put("dur", dur).put("notaVoz", notaVoz).put("puedeBajarse", puedeBajarse)
        .put("estadoEnvio", if (mio) "enviado" else "").put("menciones", JSONArray()).put("reacciones", JSONArray())

    fun arreglo(vararg o: JSONObject) = JSONArray().also { a -> o.forEach { a.put(it) } }.toString()

    fun carpetaTemporal(): File = File(ctx().cacheDir, "pruebas").apply { mkdirs() }
}
