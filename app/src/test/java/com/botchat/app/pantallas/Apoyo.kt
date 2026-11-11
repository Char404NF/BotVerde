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
    val limites = mutableListOf<Long>()
    override fun listarMensajes(bot: Long, chat: String, antesTs: Long, antesId: String, limite: Long): String {
        llamadas += "mensajes:$chat:$antesTs"
        limites += limite
        val todos = JSONArray(mensajes)
        val out = JSONArray()
        var n = 0
        for (i in 0 until todos.length()) {
            val o = todos.getJSONObject(i)
            val ts = o.getLong("ts")
            if (antesTs > 0 && !(ts < antesTs || (ts == antesTs && o.getString("id") < antesId))) continue
            if (n++ >= limite) break
            out.put(o)
        }
        return out.toString()
    }
    var porId = mutableMapOf<String, String>() // id → JSON del mensaje (lo que devolvería mensajesPorId)
    override fun mensajesPorId(bot: Long, chat: String, idsJson: String): String {
        llamadas += "porId:$chat:$idsJson"
        val ids = JSONArray(idsJson)
        return JSONArray().also { a -> for (i in 0 until ids.length()) porId[ids.getString(i)]?.let { a.put(JSONObject(it)) } }.toString()
    }
    /** Si no es null, es lo que devuelve contarNoLeidos (simula un chat más grande que lo cargado); si es null cuenta en `mensajes`. */
    var noLeidosReales: Long? = null
    override fun contarNoLeidos(bot: Long, chat: String, desdeTs: Long): Long {
        llamadas += "noLeidos:$chat:$desdeTs"
        noLeidosReales?.let { return it }
        val todos = JSONArray(mensajes)
        var n = 0L
        for (i in 0 until todos.length()) { val o = todos.getJSONObject(i); if (!o.optBoolean("esMio") && o.getLong("ts") > desdeTs) n++ }
        return n
    }
    var cargaAnteriorActiva = false
    override fun cargaAnterior(si: Boolean) { cargaAnteriorActiva = si }
    override fun crearChatPrueba(bot: Long, cuantos: Long): String { llamadas += "crearPrueba:$cuantos"; return "120363999999999999@g.us" }
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
    var actividad = "[]"
    var autos = "[]"
    var autoDetalle = ""
    var autoValidacion = "[]"
    var autoGuardado = """{"id":1,"version":2,"avisos":[]}"""
    var plantillas = "[]"
    var enCurso = "[]"
    var recorrido = ""
    var estMenu = """{"iniciados":0,"completados":0,"abandonados":0,"humano":0,"expirados":0,"opciones":[],"embudo":[],"invalidas":[]}"""
    var etiqueta = "[]"
    var aviso = """{"id":1,"ts":0,"tipo":"humano","chat":"","titulo":"Alguien pidió hablar con una persona","detalle":"Ana · +52 55"}"""
    var simulacion = mutableListOf<String>()
    override fun listarAutomatizaciones(bot: Long) = autos
    override fun obtenerAutomatizacion(bot: Long, id: Long) = autoDetalle
    override fun validarAutomatizacion(def: String): String { llamadas += "validar"; return autoValidacion }
    override fun guardarAutomatizacion(bot: Long, id: Long, def: String): String { llamadas += "guardarAuto:$id:$def"; return autoGuardado }
    override fun activarAutomatizacion(bot: Long, id: Long, si: Boolean) { llamadas += "activarAuto:$id:$si"; falla() }
    override fun duplicarAutomatizacion(bot: Long, id: Long): Long { llamadas += "duplicarAuto:$id"; return 99 }
    override fun borrarAutomatizacion(bot: Long, id: Long) { llamadas += "borrarAuto:$id" }
    override fun reordenarAutomatizaciones(bot: Long, idsJson: String) { llamadas += "reordenar:$idsJson" }
    override fun plantillasAutomatizacion() = plantillas
    override fun exportarAutomatizacion(bot: Long, id: Long) = "{}"
    override fun importarAutomatizacion(bot: Long, def: String): String { llamadas += "importarAuto"; return autoGuardado }
    override fun simularAutomatizacion(bot: Long, id: Long, texto: String, reiniciar: Boolean): String { llamadas += "simular:$texto:$reiniciar"; return JSONObject().put("salidas", JSONArray(simulacion.toList())).put("estado", "activo").put("paso", "").put("omitido", "").put("coincide", true).toString() }
    override fun sesionesEnCurso(bot: Long) = enCurso
    override fun recorridoSesion(bot: Long, id: Long) = recorrido
    override fun accionSesion(bot: Long, id: Long, accion: String) { llamadas += "accionSesion:$id:$accion" }
    override fun estadisticasAutomatizacion(bot: Long, id: Long, desde: String, hasta: String, prueba: Boolean): String { llamadas += "estMenu:$prueba"; return estMenu }
    override fun etiquetaDeChat(bot: Long, chat: String) = etiqueta
    override fun obtenerAviso(bot: Long, id: Long) = aviso
    override fun configurarFlujo(expiraMin: Long) { llamadas += "flujo:$expiraMin" }
    // Rangos y permisos (bloque A)
    var catalogo = Apoyo.catalogoEjemplo
    var rangos = Apoyo.rangosEjemplo
    var rangoPersona = """{"rol":null,"asignacion":null,"bloqueada":false}"""
    var solicitudesPor = mutableMapOf<String, String>()
    var politicaGrupo = """{"chat":"","modo":"todos","lista":[]}"""
    var politicas = "[]"
    var ajustesPerm = """{"prefijo":"!","desconocidos":"menu","desconocidos_texto":"Gracias por escribir. Te contestamos pronto.","tope_pendientes":"50","ignorada_dias":"7","confirm_seg":"120","limite_sin_rango":"5","grupo_modo_defecto":"todos","registro_dias":"90"}"""
    var confirmaciones = "[]"
    var acciones = "[]"
    var rangoFalla: String? = null
    var comandoRespuesta = """{"manejado":true,"texto":"Esto puedes usar:\n• !ayuda"}"""
    private fun rFalla() { rangoFalla?.let { throw IllegalStateException(it) } }
    override fun catalogoPermisos() = catalogo
    override fun listarRangos(bot: Long) = rangos
    override fun obtenerRango(bot: Long, id: Long): String {
        val a = JSONArray(rangos)
        for (i in 0 until a.length()) if (a.getJSONObject(i).getLong("id") == id) return a.getJSONObject(i).toString()
        return "null"
    }
    override fun guardarRango(bot: Long, rolJson: String): Long { llamadas += "guardarRango:$rolJson"; rFalla(); return JSONObject(rolJson).optLong("id").let { if (it == 0L) 77 else it } }
    override fun borrarRango(bot: Long, id: Long): Long { llamadas += "borrarRango:$id"; return 0 }
    override fun reordenarRangos(bot: Long, idsJson: String) { llamadas += "reordenarRangos:$idsJson" }
    override fun asignarRango(bot: Long, persona: String, rolId: Long, duracion: String) { llamadas += "asignarRango:$persona:$rolId:$duracion"; rFalla() }
    override fun revocarRango(bot: Long, persona: String) { llamadas += "revocarRango:$persona" }
    override fun rangoDePersona(bot: Long, persona: String) = rangoPersona
    override fun listarAsignaciones(bot: Long) = "[]"
    override fun listarSolicitudes(bot: Long, estado: String) = solicitudesPor[estado] ?: "[]"
    override fun contarSolicitudes(bot: Long) = JSONArray(solicitudesPor["pendiente"] ?: "[]").length().toLong()
    override fun aprobarSolicitud(bot: Long, persona: String) { llamadas += "aprobar:$persona" }
    override fun bloquearSolicitud(bot: Long, persona: String) { llamadas += "bloquear:$persona" }
    override fun ignorarSolicitud(bot: Long, persona: String) { llamadas += "ignorar:$persona" }
    override fun quitarSolicitud(bot: Long, persona: String) { llamadas += "quitarSolicitud:$persona" }
    override fun politicaDeGrupo(bot: Long, chat: String) = politicaGrupo
    override fun fijarPoliticaDeGrupo(bot: Long, politicaJson: String) { llamadas += "politicaGrupo:$politicaJson" }
    override fun listarPoliticasDeGrupo(bot: Long) = politicas
    override fun ajustesDePermisos(bot: Long) = ajustesPerm
    override fun fijarAjusteDePermisos(bot: Long, clave: String, valor: String) {
        llamadas += "ajustePerm:$clave:$valor"; rFalla()
        ajustesPerm = JSONObject(ajustesPerm).put(clave, valor).toString()
    }
    override fun confirmacionesPendientes(bot: Long) = confirmaciones
    override fun confirmarCodigo(bot: Long, codigo: String): String { llamadas += "confirmar:$codigo"; confirmaciones = "[]"; return "Listo." }
    override fun cancelarConfirmacion(bot: Long, codigo: String) { llamadas += "cancelarConfirmacion:$codigo"; confirmaciones = "[]" }
    override fun ejecutarComando(bot: Long, texto: String): String { llamadas += "comando:$texto"; return comandoRespuesta }
    override fun listarAcciones(bot: Long, limite: Long, antesId: Long) = acciones
    // Módulos e iniciativa propia (H0)
    var modulosLista = mutableListOf(
        Apoyo.moduloJson("eco_prueba", "Eco de prueba", "Módulo de ejemplo: repite una frase.", "eco", "medio", iniciativa = true, esquema = Apoyo.esquemaEjemplo()),
        Apoyo.moduloJson("juegos_demo", "Juegos", "Dados y adivina el número.", "juego", "bajo", iniciativa = false),
        Apoyo.moduloJson("difusion_demo", "Difusión", "Mensajes a una lista de personas.", "difusion", "alto", iniciativa = true, esquema = Apoyo.esquemaEjemplo()),
        Apoyo.moduloJson("propio_demo", "Con ventana propia", "Trae su propia ventana.", "reloj", "bajo", iniciativa = false),
    )
    var configModulos = mutableMapOf<String, JSONObject>()
    var ajustesIni = mutableMapOf("maestro" to "0", "tope_hora" to "10", "tope_dia" to "60", "noche_on" to "1", "noche_desde" to "23:00", "noche_hasta" to "07:00", "retraso_min" to "3", "retraso_max" to "12", "errores" to "5", "espera_max" to "30", "baja_texto" to "Listo.")
    var noMolestar = "[]"
    var registroIni = "[]"
    var seguro = """{"activo":false,"ts":0,"motivo":"","texto":"","errores":0}"""
    var moduloFalla: String? = null
    private fun mFalla() { moduloFalla?.let { throw IllegalStateException(it) } }
    private fun moduloPorId(id: String) = modulosLista.first { it.getString("id") == id }
    fun moduloPorIdParaPruebas(id: String) = moduloPorId(id)
    private fun refrescaCodigo(m: JSONObject) {
        val activo = m.getBoolean("activo")
        val seg = JSONObject(seguro).getBoolean("activo")
        val maestro = ajustesIni["maestro"] == "1"
        val ini = m.getBoolean("iniciativa")
        m.put("pausado", activo && ini && seg)
        m.put("codigo", when { !activo -> "apagado"; ini && seg -> "pausado_seguro"; ini && !maestro -> "esperando_maestro"; else -> "activo" })
        m.put("estado", when (m.getString("codigo")) { "apagado" -> "Apagado"; "pausado_seguro" -> "Pausado por modo seguro"; "esperando_maestro" -> "Encendido, pero la Iniciativa propia está apagada"; else -> "Encendido" })
    }
    override fun listarModulos(bot: Long): String { modulosLista.forEach(::refrescaCodigo); return JSONArray(modulosLista.map { JSONObject(it.toString()).apply { remove("esquema") } }).toString() }
    override fun infoModulo(bot: Long, id: String): String { val m = moduloPorId(id); refrescaCodigo(m); return m.toString() }
    override fun activarModulo(bot: Long, id: String) { llamadas += "activarModulo:$id"; mFalla(); moduloPorId(id).put("activo", true) }
    override fun desactivarModulo(bot: Long, id: String) { llamadas += "desactivarModulo:$id"; mFalla(); moduloPorId(id).put("activo", false) }
    override fun configModulo(bot: Long, id: String): String = (configModulos[id] ?: JSONObject().also { c ->
        val esq = moduloPorId(id).optJSONArray("esquema") ?: JSONArray()
        for (i in 0 until esq.length()) esq.getJSONObject(i).let { c.put(it.getString("clave"), it.opt("defecto")) }
    }).toString()
    override fun guardarConfigModulo(bot: Long, id: String, configJson: String) { llamadas += "guardarConfigModulo:$id:$configJson"; mFalla(); configModulos[id] = JSONObject(configJson) }
    override fun fijarTopesModulo(bot: Long, id: String, hora: Long, dia: Long) { llamadas += "topesModulo:$id:$hora:$dia"; mFalla() }
    override fun ajustesIniciativa(bot: Long) = JSONObject(ajustesIni as Map<*, *>).toString()
    override fun fijarAjusteIniciativa(bot: Long, clave: String, valor: String) { llamadas += "ajusteIni:$clave:$valor"; mFalla(); ajustesIni[clave] = valor }
    override fun fijarIniciativaMaestro(bot: Long, activo: Boolean) { llamadas += "maestro:$activo"; mFalla(); ajustesIni["maestro"] = if (activo) "1" else "0" }
    override fun listarNoMolestar(bot: Long) = noMolestar
    override fun agregarNoMolestar(bot: Long, persona: String, motivo: String) {
        llamadas += "agregarNM:$persona:$motivo"; mFalla()
        noMolestar = JSONArray(noMolestar).put(JSONObject().put("persona", persona).put("desdeTs", System.currentTimeMillis() / 1000).put("motivo", motivo).put("confirmado", true)).toString()
    }
    override fun quitarNoMolestar(bot: Long, persona: String) {
        llamadas += "quitarNM:$persona"
        val a = JSONArray(noMolestar); val n = JSONArray()
        for (i in 0 until a.length()) if (a.getJSONObject(i).getString("persona") != persona) n.put(a.get(i))
        noMolestar = n.toString()
    }
    override fun listarIniciativas(bot: Long, modulo: String, limite: Long, antesId: Long) = registroIni
    override fun estadoModoSeguro(bot: Long) = seguro
    override fun reanudarModoSeguro(bot: Long) { llamadas += "reanudarSeguro"; seguro = """{"activo":false,"ts":0,"motivo":"","texto":"","errores":0}""" }
    override fun configurarMotor(activo: Boolean, ventanaMin: Long) { llamadas += "motor:$activo:$ventanaMin" }
    override fun actividadMotor(bot: Long, limite: Long) = actividad
    override fun botEnChat(bot: Long, chat: String, activo: Boolean) { llamadas += "botChat:$activo" }
    override fun pausarBotEnChat(bot: Long, chat: String, minutos: Long) {
        llamadas += "pausarBot:$minutos"
        // como el motor real: el estado del chat refleja la pausa (0 = reactivado)
        estadoChat = JSONObject(estadoChat).put("botPausaHasta", if (minutos > 0) System.currentTimeMillis() / 1000 + minutos * 60 else 0L).put("pausaHumano", false).toString()
    }
    override fun olvidarMensaje(bot: Long, chat: String, id: String) { llamadas += "olvidar:$chat:$id" }
    override fun establecerZona(id: String) { zona = id }
    override fun estadisticasChat(bot: Long, chat: String) = resumenChat
    override fun calendarioChat(bot: Long, chat: String, mes: String): String { llamadas += "calendario:$mes"; return calendario }
    override fun mensajesDia(bot: Long, chat: String, dia: String): String { llamadas += "dia:$dia"; return porDia[dia] ?: "[]" }
    override fun estadisticasGlobales(bot: Long, dias: Long) = globales
}

object Apoyo {
    val clave = ByteArray(32) { (it + 1).toByte() }

    fun casillaJson(id: String, nombre: String, grupo: String, riesgo: String = "bajo", privado: Boolean = true, grupos: Boolean = true, confirma: Boolean = false, base: Boolean = false) =
        JSONObject().put("id", id).put("nombre", nombre).put("descripcion", "Descripción de $nombre").put("grupo", grupo).put("riesgo", riesgo)
            .put("privado", privado).put("grupos", grupos).put("confirma", confirma).put("base", base)

    val catalogoEjemplo = arreglo(
        casillaJson("cmd.ayuda", "Ver la ayuda", "Lo básico", base = true),
        casillaJson("personas.silenciar", "Silenciar al bot para una persona", "Personas"),
        casillaJson("personas.olvidar", "Olvidar o exportar datos", "Personas", riesgo = "alto", confirma = true),
        casillaJson("chat.pausar", "Pausar y reanudar el bot", "Chat"),
        casillaJson("grupos.expulsar", "Expulsar", "Grupos (siendo administrador)", riesgo = "alto", privado = false, confirma = true),
    )

    fun rangoJson(id: Long, nombre: String, posicion: Int, dueno: Boolean = false, color: String = "#2EC4CB", limite: Int = 10, personas: Int = 0, plantilla: String = "", casillas: JSONObject = JSONObject(), menusModo: String = "todos") =
        JSONObject().put("id", id).put("nombre", nombre).put("color", color).put("posicion", posicion).put("plantilla", plantilla).put("esDueno", dueno).put("casillas", casillas)
            .put("menusModo", menusModo).put("menus", JSONArray()).put("limiteMin", limite).put("personas", personas)

    val rangosEjemplo = arreglo(
        rangoJson(1, "Dueño", 0, dueno = true, color = "#F59E28", limite = 0),
        rangoJson(2, "Ayudante", 1, personas = 2, plantilla = "ayudante", limite = 20, casillas = JSONObject().put("personas.silenciar", JSONObject().put("modo", "privado"))),
        rangoJson(3, "Cliente VIP", 2, personas = 0, plantilla = "cliente_vip", color = "#8E4EC6"),
    )

    fun moduloJson(id: String, nombre: String, descripcion: String, icono: String, riesgo: String, iniciativa: Boolean, esquema: JSONArray? = null) =
        JSONObject().put("id", id).put("nombre", nombre).put("descripcion", descripcion).put("icono", icono).put("riesgo", riesgo).put("iniciativa", iniciativa)
            .put("activo", false).put("pausado", false).put("codigo", "apagado").put("estado", "Apagado").put("conEsquema", esquema != null)
            .put("topeHora", 5).put("topeDia", 20).put("defHora", 5).put("defDia", 20).put("activadoTs", 0).apply { if (esquema != null) put("esquema", esquema) }

    /** Un campo de cada tipo: texto, número, interruptor, lista y hora. */
    fun esquemaEjemplo() = JSONArray()
        .put(JSONObject().put("clave", "frase").put("titulo", "Frase").put("tipo", "texto").put("defecto", "eco").put("largo", 40).put("obligatorio", true))
        .put(JSONObject().put("clave", "veces").put("titulo", "Veces").put("tipo", "numero").put("defecto", 1.0).put("min", 1.0).put("max", 5.0))
        .put(JSONObject().put("clave", "gritar").put("titulo", "En mayúsculas").put("tipo", "interruptor").put("defecto", false))
        .put(JSONObject().put("clave", "estilo").put("titulo", "Estilo").put("tipo", "lista").put("defecto", "normal")
            .put("opciones", JSONArray().put(JSONObject().put("valor", "normal").put("titulo", "Normal")).put(JSONObject().put("valor", "corto").put("titulo", "Corto"))))
        .put(JSONObject().put("clave", "hora_aviso").put("titulo", "Hora del aviso").put("tipo", "hora").put("defecto", "09:00"))

    fun solicitudJson(persona: String, nombre: String, estado: String = "pendiente", mensajes: Int = 2) =
        JSONObject().put("id", 1).put("persona", persona).put("nombre", nombre).put("estado", estado).put("primeraTs", 1_000).put("ultimaTs", System.currentTimeMillis() / 1000).put("mensajes", mensajes)

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
