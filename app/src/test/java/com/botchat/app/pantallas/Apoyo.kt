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
    override fun fijarChatLocal(bot: Long, chat: String, si: Boolean) { llamadas += "fijar:$si"; estadoChat = JSONObject(estadoChat).put("fijado", si).toString() }
    override fun vaciarChat(bot: Long, chat: String): String { llamadas += "vaciarChat:$chat"; mensajes = "[]"; return liberar }
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
    var estadoAlmJson = "{\"topeTotal\":5368709120,\"uso\":0,\"tipos\":[]}"
    override fun estadoAlmacenamiento(bot: Long) = estadoAlmJson
    override fun topeAlmacenamiento(bot: Long) = 5L shl 30
    override fun fijarTope(bot: Long, bytes: Long) { llamadas += "tope:$bytes" }
    override fun aplicarTopeAhora(bot: Long) = "{\"borrados\":0}"
    var topePend = false
    override fun topePendiente(bot: Long) = topePend
    override fun confirmarTope(bot: Long) { llamadas += "tope_confirmado"; topePend = false }
    // Asistente «Crea tu bot»: por defecto ya se ofreció (no estorba a las demás pantallas).
    var asistVisto = true
    var asistAplicado = false
    var asistOtroNumero = ""
    var asistGuardado: String? = null
    var asistProbarError: String? = null
    var asistProblemas = "[]"
    override fun asistenteEstado(bot: Long): String {
        val b = asistGuardado ?: com.botchat.app.datos.JsonE9.borradorPorDefecto()
        return "{\"borrador\":$b,\"problemas\":$asistProblemas,\"visto\":$asistVisto,\"aplicado\":$asistAplicado,\"numeroPrueba\":\"$asistOtroNumero\",\"numeroPruebaDe\":\"asistente\"}"
    }
    // Descargas de video
    var videoFilas = "[]"
    var videoEstadoJson = "{\"activo\":true,\"motor\":true,\"version\":\"2026.10.26\",\"ultima\":\"2026.11.01\",\"hayNueva\":true,\"revisionTs\":1,\"enCurso\":0,\"esperando\":0,\"hoy\":2,\"instalado\":true}"
    override fun videoEstado(bot: Long) = videoEstadoJson
    override fun videoLista(bot: Long, limite: Long, antes: Long) = videoFilas
    override fun videoBorrar(bot: Long, id: Long) { llamadas += "video_borrar:$id" }
    var videoActualizarFalla: String? = null
    override fun videoActualizar(bot: Long): String {
        llamadas += "video_actualizar"
        videoActualizarFalla?.let { throw IllegalStateException(it) }
        if (!JSONObject(videoEstadoJson).optBoolean("instalado")) { videoEstadoJson = JSONObject(videoEstadoJson).put("instalado", true).put("version", "2026.11.01").put("hayNueva", false).toString(); return "Listo: descargué el motor (yt-dlp 2026.11.01). Ya puedes encender el módulo." }
        return "yt-dlp actualizado a la versión 2026.11.01."
    }
    override fun videoQuitarMotor(bot: Long) { llamadas += "video_quitar_motor"; videoEstadoJson = JSONObject(videoEstadoJson).put("instalado", false).put("version", "").put("activo", false).toString() }
    override fun videoRepetir(bot: Long, id: Long): String { llamadas += "video_repetir:$id"; return "Volviendo a descargar…" }
    override fun asistenteGuardar(bot: Long, borradorJson: String) { asistGuardado = borradorJson; llamadas += "asist_guardar" }
    override fun asistenteBase(bot: Long): String {
        llamadas += "asist_base"
        asistGuardado = com.botchat.app.datos.BorradorAsistente(
            grupos = true, bienvenidaTexto = "¡Hola {nombre}! 👋 Soy tu bot. Toca una opción para empezar.", menuTitulo = "¿Qué quieres hacer?",
            menuOpciones = listOf(
                com.botchat.app.datos.OpcionAsistente("GitHub", "https://github.com/x"), com.botchat.app.datos.OpcionAsistente("Menú", "m"),
                com.botchat.app.datos.OpcionAsistente("Ayuda", "a"),
            ),
        ).json()
        return asistenteEstado(bot)
    }
    override fun asistenteVisto(bot: Long) { asistVisto = true; llamadas += "asist_visto" }
    override fun asistenteAplicar(bot: Long): String { llamadas += "asist_aplicar"; asistAplicado = true; return "{\"menu\":7,\"autos\":[3],\"avisos\":[]}" }
    override fun asistenteDeshacer(bot: Long) { llamadas += "asist_deshacer"; asistAplicado = false }
    override fun asistenteDescartar(bot: Long) { llamadas += "asist_descartar"; asistGuardado = null }
    override fun asistenteProbar(bot: Long, numero: String): String {
        asistProbarError?.let { throw IllegalStateException(it) }
        llamadas += "asist_probar:$numero"; return "{\"id\":\"X\"}"
    }
    override fun conservarPortadaVideo(bot: Long) = true
    override fun fijarConservarPortada(bot: Long, si: Boolean) { llamadas += "portada:$si" }
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
    // Menús ampliados (bloque C)
    var ajustesMenus = mutableMapOf(
        "menu_err_max" to "3", "menu_txt_n1" to "No entendí esa opción. Estas son las opciones:", "menu_txt_n2" to "Sigo sin entender. Escribe *persona* para hablar con alguien.",
        "menu_txt_n3" to "Parece que no nos estamos entendiendo.", "menu_txt_notexto" to "Por ahora solo entiendo mensajes escritos.", "menu_txt_ambiguo" to "Encontré varias opciones.",
        "menu_txt_salir" to "Listo.", "menu_txt_cancelar" to "Cancelado.", "menu_saludos" to "hola, gracias, ok", "menu_volver" to "reiniciar", "menu_retoma_horas" to "24",
        "menu_humano_dentro" to "Una persona te atenderá.", "menu_humano_fuera" to "Estamos fuera de horario.", "menu_horario" to "", "menu_region" to "MX",
        "noentendio_activo" to "1", "noentendio_dias" to "30", "grupo_max_sesiones" to "5", "grupo_expira_min" to "10", "grupo_msgs_min" to "8", "grupo_cualquiera" to "0",
        "grupo_cola_min" to "10", "grupo_txt_turno" to "Estás en el turno {n}.", "grupo_txt_vencido" to "Esa conversación terminó.", "grupo_txt_privado" to "Escríbeme al privado.",
    )
    var formulariosJson = "[]"
    var formularioJson = """{"id":1,"menuId":1,"menu":"Citas","cliente":"Ana","chat":"","persona":"","ts":0,"campos":[]}"""
    var noEntendioJson = "[]"
    var adjuntoJson = """{"id":"abc123","tipo":"imagen","mime":"image/jpeg","nombre":"foto.jpg","bytes":14}"""
    var importadoJson = """{"ids":[7],"nombres":["Menú importado"],"avisos":0,"omitidos":0}"""
    var menusFalla: String? = null
    override fun ajustesDeMenus(bot: Long) = JSONObject(ajustesMenus.toMap()).toString()
    override fun fijarAjusteDeMenus(bot: Long, clave: String, valor: String) {
        llamadas += "ajusteMenus:$clave:$valor"
        menusFalla?.let { throw IllegalStateException(it) }
        ajustesMenus[clave] = valor
    }
    override fun listarFormularios(bot: Long, menuId: Long, limite: Long): String { llamadas += "formularios:$menuId"; return formulariosJson }
    override fun obtenerFormulario(bot: Long, id: Long) = formularioJson
    override fun contarFormularios(bot: Long, menuId: Long) = JSONArray(formulariosJson).length().toLong()
    override fun borrarFormulario(bot: Long, id: Long) { llamadas += "borrarFormulario:$id"; formulariosJson = "[]" }
    override fun borrarFormularios(bot: Long, menuId: Long): Long { llamadas += "borrarFormularios:$menuId"; val n = JSONArray(formulariosJson).length().toLong(); formulariosJson = "[]"; return n }
    override fun exportarFormularios(bot: Long, menuId: Long): String { llamadas += "exportarFormularios:$menuId"; menusFalla?.let { throw IllegalStateException(it) }; return "exportes/1/formularios_1.bcf" }
    override fun listarNoEntendio(bot: Long, menuId: Long, limite: Long): String { llamadas += "noEntendio:$menuId"; return noEntendioJson }
    override fun borrarNoEntendio(bot: Long, menuId: Long): Long { llamadas += "borrarNoEntendio:$menuId"; val n = JSONArray(noEntendioJson).length().toLong(); noEntendioJson = "[]"; return n }
    override fun subirAdjuntoMenu(bot: Long, ruta: String, tipo: String, mime: String, nombre: String): String { llamadas += "subirAdjunto:$tipo:$nombre"; menusFalla?.let { throw IllegalStateException(it) }; return adjuntoJson }
    override fun borrarAdjuntoMenu(bot: Long, id: String) { llamadas += "borrarAdjunto:$id" }
    override fun exportarMenus(bot: Long, idsJson: String, clave: String): String { llamadas += "exportarMenus:$idsJson:${clave.isNotEmpty()}"; menusFalla?.let { throw IllegalStateException(it) }; return "exportes/1/menus_1.bcm" }
    override fun importarMenus(bot: Long, ruta: String, clave: String): String { llamadas += "importarMenus:${clave.isNotEmpty()}"; menusFalla?.let { throw IllegalStateException(it) }; return importadoJson }
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
    // Paneles de los módulos H
    var filasPaneles = mutableMapOf<String, String>() // "modulo:panel" -> JSON de filas
    var respuestaAccionModulo = "Hecho."
    override fun filasPanelModulo(bot: Long, id: String, panel: String): String { llamadas += "filasPanel:$id:$panel"; mFalla(); return filasPaneles["$id:$panel"] ?: "[]" }
    override fun accionModulo(bot: Long, id: String, accion: String, argsJson: String): String { llamadas += "accionModulo:$id:$accion:$argsJson"; mFalla(); return respuestaAccionModulo }
    var proximoModulosTs = 0L
    override fun proximoModulos(bot: Long) = proximoModulosTs
    override fun tickModulos(bot: Long) { llamadas += "tickModulos" }
    var llavesEnGo = mutableMapOf<String, String>()
    override fun fijarLlaveModulo(bot: Long, id: String, clave: String, valor: String) { llamadas += "llave:$id:$clave"; llavesEnGo["$id:$clave"] = valor }
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

    // Personas, chat y avisos (bloque B)
    var fichaJson = Apoyo.fichaEjemplo()
    var estadisticasPersonaJson = """{"persona":"p","total":3,"porTipo":{"texto":2,"imagen":1},"porHora":[0,0,0,0,0,0,0,0,0,3,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"porDia":[{"dia":"2026-03-10","cuenta":3}],"chats":[{"chat":"a@s.whatsapp.net","nombre":"Ana","cuenta":3,"grupo":false}],"bytes":0,"primerDia":"2026-03-10","ultimoDia":"2026-03-10"}"""
    var silenciosJson = "[]"
    var advertenciasJson = "[]"
    var advertidosJson = "[]"
    var notasJson = "[]"
    var resultadoAdvertir = """{"total":1,"tope":3,"llegoAlTope":false,"accion":"avisar","hecho":"","motivo":""}"""
    var olvidarJson = """{"mensajes":4,"bytes":0,"notas":1,"advertencias":2,"sesiones":0,"alias":2}"""
    var exportarRuta = "exportes/1/persona_x.bcf"
    var personaFalla: String? = null
    private fun pFalla() { personaFalla?.let { throw IllegalStateException(it) } }
    override fun fichaPersona(bot: Long, persona: String) = fichaJson
    override fun estadisticasPersona(bot: Long, persona: String, dias: Long) = estadisticasPersonaJson
    override fun silenciarPersona(bot: Long, persona: String, minutos: Long, motivo: String) {
        llamadas += "silenciar:$persona:$minutos:$motivo"; pFalla()
        fichaJson = JSONObject(fichaJson).put("silencio", JSONObject().put("persona", persona).put("hastaTs", if (minutos > 0) System.currentTimeMillis() / 1000 + minutos * 60 else 0L).put("motivo", motivo).put("creadoTs", 1).put("por", "yo")).toString()
    }
    override fun quitarSilencioPersona(bot: Long, persona: String): Boolean { llamadas += "quitarSilencio:$persona"; pFalla(); fichaJson = JSONObject(fichaJson).put("silencio", JSONObject.NULL).toString(); return true }
    override fun listarSilencios(bot: Long) = silenciosJson
    override fun bloquearPersona(bot: Long, persona: String) { llamadas += "bloquearPersona:$persona"; pFalla(); fichaJson = JSONObject(fichaJson).put("bloqueada", true).toString() }
    override fun desbloquearPersona(bot: Long, persona: String): Boolean { llamadas += "desbloquearPersona:$persona"; pFalla(); fichaJson = JSONObject(fichaJson).put("bloqueada", false).toString(); return true }
    override fun advertirPersona(bot: Long, persona: String, chat: String, motivo: String): String {
        llamadas += "advertir:$persona:$chat:$motivo"; pFalla()
        fichaJson = JSONObject(fichaJson).put("advertencias", JSONObject(fichaJson).optInt("advertencias") + 1).toString()
        return resultadoAdvertir
    }
    override fun listarAdvertencias(bot: Long, persona: String) = advertenciasJson
    override fun listarPersonasAdvertidas(bot: Long) = advertidosJson
    override fun quitarAdvertenciaPersona(bot: Long, persona: String, id: Long): Long { llamadas += "quitarAdvertencia:$persona:$id"; pFalla(); return 1 }
    override fun borrarAdvertenciasPersona(bot: Long, persona: String): Long { llamadas += "borrarAdvertencias:$persona"; pFalla(); advertenciasJson = "[]"; return 2 }
    override fun listarNotasPersona(bot: Long, persona: String) = notasJson
    override fun agregarNotaPersona(bot: Long, persona: String, texto: String) {
        llamadas += "nota:$persona:$texto"; pFalla()
        notasJson = JSONArray(notasJson).put(JSONObject().put("id", 50).put("persona", persona).put("texto", texto).put("ts", System.currentTimeMillis() / 1000).put("por", "yo")).toString()
    }
    override fun borrarNotaPersona(bot: Long, persona: String, id: Long) { llamadas += "borrarNota:$persona:$id"; pFalla(); notasJson = "[]" }
    override fun olvidarPersona(bot: Long, persona: String): String { llamadas += "olvidarPersona:$persona"; pFalla(); return olvidarJson }
    override fun exportarPersona(bot: Long, persona: String): String { llamadas += "exportarPersona:$persona"; pFalla(); return exportarRuta }
    var chatsFiltrados = mutableMapOf<String, String>() // "estado|etiqueta" → JSON
    override fun marcarEstadoChat(bot: Long, chat: String, estado: String) {
        llamadas += "estadoChat:$chat:$estado"; pFalla()
        estadoChat = JSONObject(estadoChat).put("estadoTrabajo", estado).toString()
    }
    override fun listarChatsFiltrados(bot: Long, grupos: Boolean, consulta: String, estado: String, etiqueta: Long, limite: Long, desde: Long): String {
        llamadas += "filtrar:$grupos:$estado:$etiqueta"
        return chatsFiltrados["$estado|$etiqueta"] ?: "[]"
    }
    override fun contarPendientes(bot: Long) = pendientes
    var pendientes = 0L
    var etiquetasJson = mutableListOf<JSONObject>()
    var etiquetasDeChatMapa = mutableMapOf<String, MutableSet<Long>>()
    private fun etiquetaVista(e: JSONObject) = JSONObject(e.toString()).put("chats", etiquetasDeChatMapa.values.count { e.getLong("id") in it })
    override fun listarEtiquetas(bot: Long) = JSONArray(etiquetasJson.map(::etiquetaVista)).toString()
    override fun crearEtiqueta(bot: Long, nombre: String, color: String): Long {
        llamadas += "crearEtiqueta:$nombre:$color"; pFalla()
        val id = (etiquetasJson.maxOfOrNull { it.getLong("id") } ?: 0L) + 1
        etiquetasJson += JSONObject().put("id", id).put("nombre", nombre).put("color", color.ifBlank { "#2EC4CB" }).put("posicion", id).put("chats", 0)
        return id
    }
    override fun editarEtiqueta(bot: Long, id: Long, nombre: String, color: String) {
        llamadas += "editarEtiqueta:$id:$nombre:$color"; pFalla()
        etiquetasJson.first { it.getLong("id") == id }.put("nombre", nombre).put("color", color)
    }
    // E3 (0.6.0): mínimos para compilar; las pantallas (E7/E8) los usarán con sus propias pruebas.
    var marcadosJson = "[]"
    var miembrosJson = "[]"
    var cambiosJson = "[]"
    override fun crearEtiquetaEmoji(bot: Long, nombre: String, color: String, emoji: String): Long {
        val id = crearEtiqueta(bot, nombre, color)
        etiquetasJson.first { it.getLong("id") == id }.put("emoji", emoji)
        return id
    }
    override fun editarEtiquetaEmoji(bot: Long, id: Long, nombre: String, color: String, emoji: String) {
        editarEtiqueta(bot, id, nombre, color)
        etiquetasJson.first { it.getLong("id") == id }.put("emoji", emoji)
    }
    override fun ponerEtiquetaPersona(bot: Long, persona: String, id: Long, si: Boolean) { llamadas += "etiquetaPersona:$persona:$id:$si"; pFalla() }
    override fun etiquetasDePersona(bot: Long, persona: String) = "[]"
    var etiquetasPersonas = "{}"
    override fun etiquetasDePersonas(bot: Long, personasJson: String) = etiquetasPersonas
    override fun etiquetasDeChats(bot: Long, chatsJson: String) = "{}"
    override fun marcarMensaje(bot: Long, chat: String, msgId: String, etiqueta: Long, nota: String) { llamadas += "marcar:$chat:$msgId:$etiqueta"; pFalla() }
    override fun quitarMarcaMensaje(bot: Long, chat: String, msgId: String, etiqueta: Long) { llamadas += "quitarMarca:$chat:$msgId:$etiqueta"; pFalla() }
    var marcasJson = "[]"
    override fun marcasDeMensaje(bot: Long, chat: String, msgId: String) = marcasJson
    override fun etiquetasDeMensajes(bot: Long, chat: String, idsJson: String) = "{}"
    override fun listarMarcados(bot: Long, tipo: String, etiqueta: Long, limite: Long, desde: Long) = marcadosJson
    override fun contarMarcados(bot: Long, etiqueta: Long) = "{\"mensaje\":0,\"persona\":0,\"chat\":0,\"grupo\":0}"
    override fun miembrosGrupoDetallado(bot: Long, grupo: String, incluirSalidos: Boolean) = miembrosJson
    override fun cambiosMiembrosGrupo(bot: Long, grupo: String, antes: Long, limite: Long) = cambiosJson
    override fun actualizarMiembrosGrupo(bot: Long) { llamadas += "actualizarMiembros" }
    override fun gruposEnComunPersona(bot: Long, persona: String) = "[]"
    override fun pedirFotoPersona(bot: Long, persona: String, grupo: String, forzar: Boolean) { llamadas += "fotoPersona:$persona" }
    override fun fotoPersona(bot: Long, persona: String) = ""
    var perfilJson = "{}"
    override fun perfilPersona(bot: Long, persona: String) = perfilJson
    var actividadJson = "[]"
    override fun actividadPersona(bot: Long, persona: String) = actividadJson
    var paginaPersona = """{"total":0,"items":[],"fin":true,"sigTs":0,"sigId":"","escaneados":0}"""
    val filtrosPersona = mutableListOf<String>()
    val paginasPersona = ArrayDeque<String>()
    override fun mensajesDePersona(bot: Long, filtroJson: String, antesTs: Long, antesId: String, limite: Long): String { filtrosPersona += filtroJson; return paginasPersona.removeFirstOrNull() ?: paginaPersona }
    var busqueda = """{"total":0,"items":[],"fin":true,"sigTs":0,"sigId":"","escaneados":0}"""
    val palabrasBuscadas = mutableListOf<String>()
    override fun buscarEnChat(bot: Long, chat: String, palabra: String, desde: Long, hasta: Long, antesTs: Long, antesId: String, limite: Long): String { palabrasBuscadas += palabra; return busqueda }
    var puntitosJson = "{}"
    var consultasPuntitos = 0
    override fun puntitosDeMensajes(bot: Long, chat: String, idsJson: String): String { consultasPuntitos++; return puntitosJson }
    override fun exportarChat(bot: Long, chat: String, formato: String, desde: Long, hasta: Long): String { llamadas += "exportarChat:$formato"; return """{"ruta":"exportes/chat.txt.bcf","mensajes":3}""" }
    // Silenciar en grupos y advertencias con sanciones (E5): sin pantallas, solo para compilar.
    override fun silenciarEnGrupos(bot: Long, persona: String, segundos: Long, alcance: String, gruposJson: String, avisar: Boolean, texto: String): String { llamadas += "silenciarGrupos:$persona:$segundos:$alcance:$gruposJson:$avisar:$texto"; return "{}" }
    override fun quitarSilencioGrupos(bot: Long, persona: String): Boolean { llamadas += "quitarSilencioGrupos:$persona"; return true }
    var silenciosGruposJson = "[]"
    override fun listarSilenciosGrupos(bot: Long) = silenciosGruposJson
    override fun historialSilencios(bot: Long, persona: String) = "[]"
    var contadorJson = """{"usado":0,"tope":10,"color":"verde"}"""
    override fun contadorSilencio(bot: Long, persona: String, grupo: String) = contadorJson
    override fun ponerContadorEnCero(bot: Long, persona: String, grupo: String) { llamadas += "contadorCero:$persona" }
    var ajustesSilJson = """{"ajustes":{},"textos":{"inicio":{"actual":"Te silencié en {grupo} {duracion}.","defecto":"Te silencié en {grupo} {duracion}."}}}"""
    override fun ajustesSilencio(bot: Long) = ajustesSilJson
    override fun fijarAjusteSilencio(bot: Long, clave: String, valor: String) { llamadas += "ajusteSilencio:$clave:$valor" }
    override fun fijarTextoSilencio(bot: Long, id: String, texto: String) { llamadas += "textoSilencio:$id" }
    var sancionesJson = "[]"
    override fun sancionesPendientes(bot: Long) = sancionesJson
    override fun confirmarSancionSilencio(bot: Long, id: Long, si: Boolean): String { llamadas += "sancion:$id:$si"; return "" }
    override fun confirmarSancion(bot: Long, id: Long, si: Boolean): String { llamadas += "sancion:$id:$si"; return "" }
    var gruposAdmin = "[]"
    override fun gruposQueAdministro(bot: Long) = gruposAdmin
    var tiposAdvJson = "[]"
    override fun listarTiposAdvertencia(bot: Long) = tiposAdvJson
    val tiposGuardados = mutableListOf<String>()
    override fun guardarTipoAdvertencia(bot: Long, tipoJson: String): Long { tiposGuardados += tipoJson; return 1L }
    override fun borrarTipoAdvertencia(bot: Long, id: Long) { llamadas += "borrarTipoAdv:$id" }
    override fun listarEscalera(bot: Long, tipoId: Long) = "[]"
    override fun guardarEscalera(bot: Long, tipoId: Long, pasosJson: String) { llamadas += "escalera:$tipoId" }
    var resultadoAdvTipo = "{}"
    override fun advertirConTipo(bot: Long, persona: String, chat: String, msgId: String, motivo: String, tipoId: Long, reglaId: Long): String { llamadas += "advertirTipo:$persona:$tipoId:$chat:$reglaId"; return resultadoAdvTipo }
    val reglasPorGrupo = mutableMapOf<String, String>()
    override fun listarReglasGrupo(bot: Long, grupo: String) = reglasPorGrupo[grupo] ?: "[]"
    override fun guardarReglaItem(bot: Long, grupo: String, id: Long, titulo: String, texto: String, pos: Long): Long { llamadas += "guardarRegla:$grupo:$id:$titulo:$pos"; return 1L }
    override fun borrarReglaItem(bot: Long, id: Long) { llamadas += "borrarRegla:$id" }
    override fun moverReglaItem(bot: Long, id: Long, delta: Long) { llamadas += "moverRegla:$id:$delta" }
    // E8: auditoría y privacidad.
    var auditoriaJson = """{"defecto":true,"retDias":90,"retPendiente":false,"avisoVisto":true}"""
    override fun auditoriaAjustes(bot: Long) = auditoriaJson
    override fun fijarAuditoria(bot: Long, guardar: Boolean, retDias: Long) {
        llamadas += "fijarAuditoria:$guardar:$retDias"
        val o = JSONObject(auditoriaJson).put("defecto", guardar).put("retPendiente", false)
        if (retDias >= 0) o.put("retDias", retDias)
        auditoriaJson = o.toString()
    }
    override fun fijarAvisoAuditoriaVisto(bot: Long, visto: Boolean) { llamadas += "avisoAuditoria:$visto"; auditoriaJson = JSONObject(auditoriaJson).put("avisoVisto", visto).toString() }
    override fun auditoriaChat(bot: Long, chat: String) = "{}"
    override fun fijarAuditoriaChat(bot: Long, chat: String, audit: Long, retDias: Long) { llamadas += "auditoriaChat:$chat:$audit:$retDias" }
    var auditoriaChatsJson = "[]"
    override fun listarAuditoriaChats(bot: Long, consulta: String, limite: Long, desde: Long) = auditoriaChatsJson
    override fun limpiarPorRetencion(bot: Long) = "{}"
    override fun borrarTodoDePersona(bot: Long, persona: String): String { llamadas += "borrarTodo:$persona"; return "{}" }
    var versionesJson = "[]"
    override fun versionesDeMensaje(bot: Long, chat: String, id: String) = versionesJson
    var estadosPersonas = "{}"
    override fun estadosDePersonas(bot: Long, personasJson: String) = estadosPersonas
    override fun borrarEtiqueta(bot: Long, id: Long) { llamadas += "borrarEtiqueta:$id"; pFalla(); etiquetasJson.removeAll { it.getLong("id") == id }; etiquetasDeChatMapa.values.forEach { it.remove(id) } }
    override fun ponerEtiquetaChat(bot: Long, chat: String, id: Long, si: Boolean) {
        llamadas += "etiquetaChat:$chat:$id:$si"; pFalla()
        val s = etiquetasDeChatMapa.getOrPut(chat) { mutableSetOf() }
        if (si) s += id else s -= id
    }
    override fun etiquetasDeChat(bot: Long, chat: String) = JSONArray(etiquetasJson.filter { it.getLong("id") in (etiquetasDeChatMapa[chat] ?: emptySet<Long>()) }.map(::etiquetaVista)).toString()
    var palabras = mutableListOf<JSONObject>()
    override fun listarAlertasPalabra(bot: Long) = JSONArray(palabras).toString()
    override fun crearAlertaPalabra(bot: Long, palabra: String, chatsJson: String): Long {
        llamadas += "crearPalabra:$palabra:$chatsJson"; pFalla()
        val id = (palabras.maxOfOrNull { it.getLong("id") } ?: 0L) + 1
        palabras += JSONObject().put("id", id).put("palabra", palabra).put("chats", JSONArray(chatsJson)).put("activa", true)
        return id
    }
    override fun editarAlertaPalabra(bot: Long, id: Long, chatsJson: String, activa: Boolean) {
        llamadas += "editarPalabra:$id:$chatsJson:$activa"; pFalla()
        palabras.first { it.getLong("id") == id }.put("chats", JSONArray(chatsJson)).put("activa", activa)
    }
    override fun borrarAlertaPalabra(bot: Long, id: Long) { llamadas += "borrarPalabra:$id"; pFalla(); palabras.removeAll { it.getLong("id") == id } }
    var recordatoriosLista = mutableListOf<JSONObject>()
    override fun listarRecordatorios(bot: Long, hechos: Boolean) = JSONArray(recordatoriosLista.filter { hechos || it.getString("estado") == "pendiente" }).toString()
    override fun crearRecordatorio(bot: Long, texto: String, proximaTs: Long, repetir: String): Long {
        llamadas += "crearRecordatorio:$texto:$proximaTs:$repetir"; pFalla()
        val id = (recordatoriosLista.maxOfOrNull { it.getLong("id") } ?: 0L) + 1
        recordatoriosLista += JSONObject().put("id", id).put("texto", texto).put("proximaTs", proximaTs).put("repetir", repetir).put("estado", "pendiente").put("creadoTs", 1).put("avisadoTs", 0)
        return id
    }
    override fun editarRecordatorio(bot: Long, id: Long, texto: String, proximaTs: Long, repetir: String) {
        llamadas += "editarRecordatorio:$id:$texto:$proximaTs:$repetir"; pFalla()
        recordatoriosLista.first { it.getLong("id") == id }.put("texto", texto).put("proximaTs", proximaTs).put("repetir", repetir).put("estado", "pendiente")
    }
    override fun borrarRecordatorio(bot: Long, id: Long) { llamadas += "borrarRecordatorio:$id"; pFalla(); recordatoriosLista.removeAll { it.getLong("id") == id } }
    // Notas y pendientes propios (bloque F)
    var utilLista = mutableListOf<JSONObject>()
    override fun listarUtilidades(bot: Long, tipo: String) = JSONArray(utilLista.filter { it.getString("tipo") == tipo }).toString()
    override fun agregarUtilidad(bot: Long, tipo: String, texto: String): Long {
        llamadas += "agregarUtil:$tipo:$texto"; pFalla()
        val id = (utilLista.maxOfOrNull { it.getLong("id") } ?: 0L) + 1
        utilLista += JSONObject().put("id", id).put("tipo", tipo).put("texto", texto).put("hecho", false).put("ts", 1)
        return id
    }
    override fun marcarUtilidad(bot: Long, id: Long, hecho: Boolean) { llamadas += "marcarUtil:$id:$hecho"; pFalla(); utilLista.first { it.getLong("id") == id }.put("hecho", hecho) }
    override fun borrarUtilidad(bot: Long, id: Long) { llamadas += "borrarUtil:$id"; pFalla(); utilLista.removeAll { it.getLong("id") == id } }
    override fun limpiarUtilidadesHechas(bot: Long): Long {
        llamadas += "limpiarUtilHechas"; pFalla()
        val n = utilLista.count { it.getString("tipo") == "pendiente" && it.getBoolean("hecho") }
        utilLista.removeAll { it.getString("tipo") == "pendiente" && it.getBoolean("hecho") }
        return n.toLong()
    }
    // Laboratorio (bloque G)
    var labAj = mutableMapOf("lab_activo" to "0", "lab_aviso_ts" to "0", "lab_modo_menu" to "texto", "lab_grupos" to "0", "lab_enc_max_opc" to "12", "lab_expira_min" to "120",
        "lab_respaldo_enc" to "0", "lab_respaldo_bot" to "1", "lab_botones_tipo" to "auto", "lab_prueba_chat" to "", "lab_prueba_hora" to "4", "lab_prueba_dia" to "12", "lab_version_wa" to "", "lab_biz_activo" to "0", "lab_biz_ack" to "0")
    var labResultados = mutableListOf<JSONObject>()
    var labEnMarcha = true
    private fun labLimite(): Int = labResultados.filter { it.getString("tipo") == "encuesta" && it.getInt("visible") == 1 && it.getInt("tocable") != 0 }.maxOfOrNull { it.getString("variante").toInt() } ?: 0
    override fun estadoLab(bot: Long): String {
        llamadas += "estadoLab"; rFalla()
        val on = labAj["lab_activo"] == "1" && labAj["lab_aviso_ts"] != "0"
        val plan = JSONArray().put(JSONObject().put("tipo", "encuesta").put("variante", 4).put("nombre", "Encuesta de 4 opciones"))
            .put(JSONObject().put("tipo", "encuesta").put("variante", 13).put("nombre", "Encuesta de 13 opciones"))
            .put(JSONObject().put("tipo", "botones").put("variante", 3).put("nombre", "Botones (3)"))
            .put(JSONObject().put("tipo", "lista").put("variante", 10).put("nombre", "Lista de 10 filas"))
            .put(JSONObject().put("tipo", "interactivo").put("variante", 3).put("nombre", "Interactivo quick_reply (3)"))
        return JSONObject().put("activo", labAj["lab_activo"] == "1").put("avisoAceptado", labAj["lab_aviso_ts"] != "0").put("avisoTs", 0).put("modo", labAj["lab_modo_menu"])
            .put("modoReal", if (on) labAj["lab_modo_menu"] else "texto").put("grupos", labAj["lab_grupos"] == "1").put("encMax", labAj["lab_enc_max_opc"]!!.toInt())
            .put("expiraMin", labAj["lab_expira_min"]!!.toInt()).put("respaldoEnc", labAj["lab_respaldo_enc"] == "1").put("respaldoBot", labAj["lab_respaldo_bot"] == "1")
            .put("botonesTipo", labAj["lab_botones_tipo"]).put("pruebaChat", labAj["lab_prueba_chat"]).put("pruebaHora", labAj["lab_prueba_hora"]!!.toInt())
            .put("pruebaDia", labAj["lab_prueba_dia"]!!.toInt()).put("versionWa", labAj["lab_version_wa"]).put("versionCliente", "2.3000.1").put("enMarcha", labEnMarcha)
            .put("limiteMedido", labLimite()).put("pruebasHora", labResultados.size).put("pruebasDia", labResultados.size).put("plan", plan).put("bizActivo", labAj["lab_biz_activo"] == "1").put("bizAck", labAj["lab_biz_ack"] == "1").toString()
    }
    override fun aceptarAvisoLab(bot: Long) { llamadas += "aceptarAvisoLab"; rFalla(); labAj["lab_aviso_ts"] = "1700000000" }
    override fun fijarAjusteLab(bot: Long, clave: String, valor: String) {
        llamadas += "ajusteLab:$clave:$valor"; rFalla()
        if (clave == "lab_activo" && valor == "1" && labAj["lab_aviso_ts"] == "0") throw IllegalStateException("Falta aceptar el aviso de riesgo del Laboratorio.")
        labAj[clave] = valor
    }
    override fun enviarPruebaLab(bot: Long, tipo: String, variante: Long): String {
        llamadas += "pruebaLab:$tipo:$variante"; rFalla()
        if (!labEnMarcha) throw IllegalStateException("Enciende el bot para hacer pruebas.")
        val id = (labResultados.maxOfOrNull { it.getLong("id") } ?: 0L) + 1
        val r = JSONObject().put("id", id).put("ts", 1700000000L + id).put("tipo", tipo).put("variante", variante.toString()).put("chat", labAj["lab_prueba_chat"]).put("msgId", "M$id")
            .put("enviado", 1).put("llego", -1).put("visible", -1).put("tocable", -1).put("respondio", -1).put("versionWa", labAj["lab_version_wa"]).put("versionCliente", "2.3000.1").put("error", "")
        labResultados.add(0, r)
        return r.toString()
    }
    override fun listarResultadosLab(bot: Long) = JSONArray(labResultados).toString()
    override fun marcarResultadoLab(bot: Long, id: Long, campo: String, valor: Long) { llamadas += "marcarLab:$id:$campo:$valor"; rFalla(); labResultados.first { it.getLong("id") == id }.put(campo, valor.toInt()) }
    override fun borrarResultadosLab(bot: Long): Long { llamadas += "borrarLab"; rFalla(); val n = labResultados.size; labResultados.clear(); return n.toLong() }
    override fun usarLimiteMedido(bot: Long): Long {
        llamadas += "usarLimiteLab"; rFalla()
        val n = labLimite()
        if (n < 2) throw IllegalStateException("todavía no marcaste ninguna encuesta como «visible»")
        labAj["lab_enc_max_opc"] = n.toString()
        return n.toLong()
    }
    // Captura de mensajes (diagnóstico)
    var capActiva = false
    var capTextos = false
    var capItems = mutableListOf<JSONObject>()
    override fun estadoCaptura(bot: Long): String {
        llamadas += "estadoCaptura"; rFalla()
        return JSONObject().put("activo", capActiva).put("mostrarTextos", capTextos).put("restanSeg", if (capActiva) 540 else 0).put("capturados", capItems.size).put("max", 30).put("items", JSONArray(capItems)).toString()
    }
    override fun activarCaptura(bot: Long, activo: Boolean) { llamadas += "activarCaptura:$activo"; rFalla(); capActiva = activo }
    override fun mostrarTextosCaptura(bot: Long, mostrar: Boolean) { llamadas += "textosCaptura:$mostrar"; rFalla(); capTextos = mostrar }
    override fun borrarCaptura(bot: Long) { llamadas += "borrarCaptura"; rFalla(); capItems.clear() }
    var proximoAvisoTs = 0L
    var disparo = """{"avisos":[],"siguiente":0}"""
    override fun proximoAviso(bot: Long): Long { llamadas += "proximoAviso"; return proximoAvisoTs }
    override fun dispararAvisos(bot: Long): String { llamadas += "dispararAvisos"; return disparo }
    override fun vistaPreviaResumen(bot: Long) = "Hoy llegaron 5 mensajes de 3 chat(s).\nChats pendientes: 1\nSolicitudes por revisar: 0\nAdvertencias de hoy: 0"
    var comandosPropiosJson = mutableListOf<JSONObject>()
    override fun listarComandosPropios(bot: Long) = JSONArray(comandosPropiosJson).toString()
    override fun guardarComandoPropio(bot: Long, id: Long, datosJson: String): Long {
        llamadas += "guardarComando:$id:$datosJson"; pFalla()
        val datos = JSONObject(datosJson)
        val usado = comandosPropiosJson.any { it.getLong("id") != id && it.getJSONObject("datos").getString("nombre").equals(datos.getString("nombre"), true) }
        if (usado) throw IllegalStateException("ya tienes un comando con ese nombre")
        val nuevo = if (id == 0L) (comandosPropiosJson.maxOfOrNull { it.getLong("id") } ?: 0L) + 1 else id
        comandosPropiosJson.removeAll { it.getLong("id") == nuevo }
        comandosPropiosJson += JSONObject().put("id", nuevo).put("activo", true).put("datos", datos)
        return nuevo
    }
    override fun vistaPreviaComandoPropio(bot: Long, datosJson: String): String { llamadas += "vistaComando"; return "Escriben: !" + JSONObject(datosJson).optString("nombre") + "\n\n" + JSONObject(datosJson).optString("texto") }

    // ---- Mensajes automáticos y adjuntos (bloque D) ----
    var mensajesAuto = mutableListOf<JSONObject>()
    var autoActivo = true
    var autoConflictos = listOf<String>()
    var registroAutoJson = "[]"
    var chatsAuto = listOf<String>()
    var mensajesAutoJson = "[]"
    var atajosJson = "[]"
    var subidaFalla: String? = null
    var adjuntosSubidos = mutableListOf<JSONObject>()
    private fun vistaAuto(o: JSONObject) = JSONObject(o.toString()).put("cuando", "siempre").put("quien", "todos · solo privados").put("repite", "una vez cada 6 horas").put("envios7d", 0).put("estado", if (o.optBoolean("activo")) "activo" else "apagado")
    override fun listarAutos(bot: Long, tipo: String): String {
        llamadas += "listarAutos:$tipo"
        return JSONObject().put("mensajes", JSONArray(mensajesAuto.filter { tipo.isEmpty() || it.getString("tipo") == tipo }.map { vistaAuto(it) })).put("activo", autoActivo).put("atajosActivo", true).put("atajoModo", "editar")
            .put("topeHora", 30).put("usadosHora", 0).put("zona", "").put("zonaBot", "America/Mexico_City").put("conflictos", JSONArray(autoConflictos)).put("automaticos24h", 0).toString()
    }
    override fun obtenerAuto(bot: Long, id: Long): String = vistaAuto(mensajesAuto.first { it.getLong("id") == id }).toString()
    override fun guardarAuto(bot: Long, entradaJson: String): Long {
        llamadas += "guardarAuto:$entradaJson"; pFalla()
        val o = JSONObject(entradaJson)
        if (o.optString("texto").isBlank() && o.optJSONArray("adjuntos")?.length().let { it == null || it == 0 }) throw IllegalStateException("Escribe el mensaje (o agrega un archivo).")
        val id = if (o.optLong("id") == 0L) (mensajesAuto.maxOfOrNull { it.getLong("id") } ?: 0L) + 1 else o.getLong("id")
        mensajesAuto.removeAll { it.getLong("id") == id }
        mensajesAuto += o.put("id", id).put("orden", mensajesAuto.size + 1)
        return id
    }
    override fun fijarAutoActivo(bot: Long, id: Long, activo: Boolean) { llamadas += "autoActivo:$id:$activo"; pFalla(); mensajesAuto.first { it.getLong("id") == id }.put("activo", activo) }
    override fun borrarAuto(bot: Long, id: Long) { llamadas += "borrarMensajeAuto:$id"; pFalla(); mensajesAuto.removeAll { it.getLong("id") == id } }
    override fun reordenarAutos(bot: Long, idsJson: String) { llamadas += "reordenarAutos:$idsJson" }
    override fun plantillasAuto(): String = JSONArray()
        .put(JSONObject().put("tipo", "bienvenida").put("nombre", "Bienvenida amable").put("descripcion", "Saluda").put("texto", "¡Hola {nombre}! Gracias por escribirnos.").put("config", JSONObject().put("cuando", JSONObject().put("siempre", true)).put("quien", JSONObject().put("personas", "todos").put("privados", true).put("grupos", false)).put("repeticion", JSONObject().put("cada", 1).put("unidad", "dias"))))
        .put(JSONObject().put("tipo", "ausencia").put("nombre", "Fuera de horario").put("descripcion", "Avisa").put("texto", "Hola {nombre}, estamos fuera de horario: {horario}.").put("config", JSONObject().put("cuando", JSONObject().put("horarioModo", "fuera").put("horario", JSONArray().put(JSONObject().put("dia", 1).put("rangos", JSONArray().put(JSONObject().put("desde", "09:00").put("hasta", "18:00")))))).put("quien", JSONObject().put("personas", "todos").put("privados", true)).put("repeticion", JSONObject().put("cada", 12).put("unidad", "horas"))))
        .put(JSONObject().put("tipo", "recibido").put("nombre", "Ya vi tu mensaje").put("descripcion", "Recibido").put("texto", "Recibí tu mensaje, {nombre}.").put("config", JSONObject().put("cuando", JSONObject().put("siempre", true)).put("quien", JSONObject().put("personas", "todos").put("privados", true)).put("repeticion", JSONObject().put("cada", 6).put("unidad", "horas"))))
        .put(JSONObject().put("tipo", "atajo").put("nombre", "/gracias").put("descripcion", "Gracias").put("texto", "¡Gracias por escribir!").put("config", JSONObject()))
        .toString()
    override fun configPorDefectoAuto(tipo: String): String {
        val base = JSONObject().put("quien", JSONObject().put("personas", "todos").put("privados", true)).put("repeticion", JSONObject().put("cada", 6).put("unidad", "horas"))
        if (tipo != "ausencia") return base.put("cuando", JSONObject().put("siempre", true)).toString()
        // igual que Go: la ausencia nace “fuera de horario” de lunes a viernes con dos rangos
        val dias = JSONArray().also { a -> (1..5).forEach { d -> a.put(JSONObject().put("dia", d).put("rangos", JSONArray().put(JSONObject().put("desde", "09:00").put("hasta", "13:00")).put(JSONObject().put("desde", "15:00").put("hasta", "18:00")))) } }
        return base.put("cuando", JSONObject().put("siempre", false).put("horarioModo", "fuera").put("horario", dias)).toString()
    }
    var ultimaPrueba: Pair<String, String>? = null
    override fun probarAuto(bot: Long, entradaJson: String, escenarioJson: String): String {
        llamadas += "probarAuto"; pFalla(); ultimaPrueba = entradaJson to escenarioJson
        val o = JSONObject(entradaJson); val e = JSONObject(escenarioJson)
        val texto = o.optString("texto").replace("{nombre}", e.optString("nombre"))
        return JSONObject().put("aplica", !e.optBoolean("esGrupo")).put("motivo", if (e.optBoolean("esGrupo")) "No es para esta persona (revisa «A quién»)." else "").put("texto", texto)
            .put("archivos", JSONArray()).put("cuando", "10/11/2026 10:00").toString()
    }
    override fun registroAuto(bot: Long, autoId: Long, limite: Long) = registroAutoJson
    override fun subirAdjunto(bot: Long, ruta: String, tipo: String, mime: String, nombre: String): String {
        llamadas += "subirAdjunto:$tipo:$nombre"; pFalla()
        subidaFalla?.let { throw IllegalStateException(it) }
        val a = JSONObject().put("id", "%024d".format(adjuntosSubidos.size + 1)).put("tipo", tipo).put("mime", mime).put("nombre", nombre).put("bytes", 10)
        adjuntosSubidos += a
        return a.toString()
    }
    override fun borrarAdjunto(bot: Long, id: String) { llamadas += "borrarAdjunto:$id" }
    override fun limpiarAdjuntos(bot: Long): Long { llamadas += "limpiarAdjuntos"; return 0 }
    override fun listarAtajos(bot: Long, chat: String) = atajosJson
    override fun enviarArchivosAtajo(bot: Long, chat: String, id: Long) { llamadas += "archivosAtajo:$chat:$id" }
    override fun listarMensajesAuto(bot: Long, chat: String, antesTs: Long, limite: Long) = mensajesAutoJson
    override fun chatsConAutomaticos(bot: Long, dias: Long) = JSONArray(chatsAuto).toString()

    // Grupos y estado de la app de WhatsApp (bloque E)
    var estadoGrupoJson = """{"conectado":true,"enGrupo":true,"soyAdmin":true,"motivo":""}"""
    var infoGrupoJson = JSONObject().put("chat", "g@g.us").put("nombre", "Familia").put("descripcion", "Para la familia").put("creado", 1_700_000_000L).put("creadoTexto", "14/11/2023")
        .put("creadoPor", "Carla").put("miembros", 3).put("admins", JSONArray().put(JSONObject().put("jid", "5215511111111@s.whatsapp.net").put("nombre", "Ana").put("esAdmin", true)))
        .put("soyAdmin", true).put("soloAdmins", false).put("infoBloqueada", false).put("aprobacionIngreso", false).put("mensajesTemporales", 0).put("reglas", "").toString()
    var miembrosGrupoJson = Apoyo.arreglo(
        JSONObject().put("jid", "5215511111111@s.whatsapp.net").put("persona", "5215511111111@s.whatsapp.net").put("nombre", "Ana").put("telefono", "+52 55 1111 1111").put("esAdmin", true).put("esCreador", false).put("yo", false),
        JSONObject().put("jid", "5215522222222@s.whatsapp.net").put("persona", "5215522222222@s.whatsapp.net").put("nombre", "Luis").put("telefono", "+52 55 2222 2222").put("esAdmin", false).put("esCreador", false).put("yo", false),
        JSONObject().put("jid", "5215544444444@s.whatsapp.net").put("persona", "5215544444444@s.whatsapp.net").put("nombre", "Carla").put("telefono", "+52 55 4444 4444").put("esAdmin", true).put("esCreador", true).put("yo", false),
    )
    var solicitudesGrupoJson = "[]"
    var rankingGrupoJson = """{"dias":30,"total":0,"filas":[],"soloMiembros":true,"desde":"","aviso":""}"""
    var inactivosGrupoJson = """{"dias":30,"aviso":"","inactivos":[]}"""
    var reglasGrupo = ""
    var enlaceGrupoTexto = "https://chat.whatsapp.com/ABCDEF"
    var resultadoTodos = """{"mencionadas":3,"total":3,"mensajes":1,"truncado":false,"aviso":""}"""
    var grupoFalla: String? = null
    var filtrosGrupoJson: String = JSONArray().also { a ->
        listOf("enlaces" to "Enlaces", "palabras" to "Palabras prohibidas", "repetidos" to "Mensajes repetidos", "flood" to "Flood").forEach { (f, n) ->
            a.put(JSONObject().put("chat", "g@g.us").put("filtro", f).put("nombre", n).put("descripcion", "Detecta $n").put("activo", false).put("accion", "avisarme")
                .put("config", JSONObject().put("exentoAdmins", true).put("exentoRangos", JSONArray()).put("permitidos", JSONArray()).put("palabras", JSONArray())
                    .put("max", if (f == "repetidos") 3 else if (f == "flood") 6 else 0).put("segundos", if (f == "repetidos") 120 else if (f == "flood") 10 else 0).put("silencioMin", 60).put("aviso", "")))
        }
    }.toString()
    var appStateJson = """{"disponible":false,"conectado":true,"motivo":"Para silenciar/fijar/archivar en tu WhatsApp hace falta que el teléfono principal sincronice (abre WhatsApp en tu teléfono principal y espera un momento)."}"""
    private fun gFalla() { grupoFalla?.let { throw IllegalStateException(it) } }
    override fun grupoEstado(bot: Long, chat: String, refrescar: Boolean): String { llamadas += "grupoEstado:$chat:$refrescar"; return estadoGrupoJson }
    override fun grupoInfo(bot: Long, chat: String): String { llamadas += "grupoInfo:$chat"; gFalla(); return infoGrupoJson }
    override fun grupoMiembros(bot: Long, chat: String): String { gFalla(); return miembrosGrupoJson }
    override fun grupoCerrar(bot: Long, chat: String, cerrado: Boolean) { llamadas += "grupoCerrar:$chat:$cerrado"; gFalla(); infoGrupoJson = JSONObject(infoGrupoJson).put("soloAdmins", cerrado).toString() }
    override fun grupoBloquearInfo(bot: Long, chat: String, bloqueada: Boolean) { llamadas += "grupoBloquear:$chat:$bloqueada"; gFalla(); infoGrupoJson = JSONObject(infoGrupoJson).put("infoBloqueada", bloqueada).toString() }
    override fun grupoExpulsar(bot: Long, chat: String, persona: String) { llamadas += "grupoExpulsar:$chat:$persona"; gFalla() }
    override fun grupoAscender(bot: Long, chat: String, persona: String, admin: Boolean) { llamadas += "grupoAscender:$persona:$admin"; gFalla() }
    override fun grupoSolicitudes(bot: Long, chat: String): String { gFalla(); return solicitudesGrupoJson }
    override fun grupoResolverSolicitudes(bot: Long, chat: String, personasJson: String, aprobar: Boolean): Long {
        llamadas += "grupoSolicitudes:${if (aprobar) "aprobar" else "rechazar"}:$personasJson"; gFalla()
        val n = if (personasJson.isBlank()) JSONArray(solicitudesGrupoJson).length() else JSONArray(personasJson).length()
        if (personasJson.isBlank()) solicitudesGrupoJson = "[]"
        else {
            val quitar = JSONArray(personasJson).let { a -> List(a.length()) { a.getString(it) } }
            val rest = JSONArray(); val a = JSONArray(solicitudesGrupoJson)
            for (i in 0 until a.length()) if (a.getJSONObject(i).getString("jid") !in quitar) rest.put(a.get(i))
            solicitudesGrupoJson = rest.toString()
        }
        return n.toLong()
    }
    override fun grupoRenombrar(bot: Long, chat: String, nombre: String) { llamadas += "grupoRenombrar:$nombre"; gFalla(); infoGrupoJson = JSONObject(infoGrupoJson).put("nombre", nombre).toString() }
    override fun grupoDescripcion(bot: Long, chat: String, texto: String) { llamadas += "grupoDescripcion:$texto"; gFalla(); infoGrupoJson = JSONObject(infoGrupoJson).put("descripcion", texto).toString() }
    override fun grupoEnlace(bot: Long, chat: String, reiniciar: Boolean): String { llamadas += "grupoEnlace:$reiniciar"; gFalla(); return if (reiniciar) "https://chat.whatsapp.com/NUEVO99" else enlaceGrupoTexto }
    override fun grupoBorrarMensaje(bot: Long, chat: String, msgId: String) { llamadas += "grupoBorrarMensaje:$msgId"; gFalla() }
    override fun grupoMencionarTodos(bot: Long, chat: String, texto: String): String { llamadas += "grupoTodos:$texto"; gFalla(); return resultadoTodos }
    override fun grupoRanking(bot: Long, chat: String, dias: Long, limite: Long): String { llamadas += "grupoRanking:$dias"; gFalla(); return JSONObject(rankingGrupoJson).put("dias", dias).toString() }
    override fun grupoInactivos(bot: Long, chat: String, dias: Long): String { llamadas += "grupoInactivos:$dias"; gFalla(); return JSONObject(inactivosGrupoJson).put("dias", dias).toString() }
    override fun grupoReglas(bot: Long, chat: String) = reglasGrupo
    override fun grupoGuardarReglas(bot: Long, chat: String, texto: String) { llamadas += "grupoReglas:$texto"; reglasGrupo = texto }
    override fun grupoFiltros(bot: Long, chat: String) = filtrosGrupoJson
    override fun grupoGuardarFiltro(bot: Long, chat: String, datosJson: String) {
        llamadas += "grupoFiltro:$datosJson"; gFalla()
        val nuevo = JSONObject(datosJson)
        val a = JSONArray(filtrosGrupoJson)
        for (i in 0 until a.length()) if (a.getJSONObject(i).getString("filtro") == nuevo.getString("filtro")) a.getJSONObject(i).also { o -> o.put("activo", nuevo.getBoolean("activo")); o.put("accion", nuevo.getString("accion")); o.put("config", nuevo.getJSONObject("config")) }
        filtrosGrupoJson = a.toString()
    }
    override fun grupoRestaurarFiltro(bot: Long, chat: String, filtro: String) { llamadas += "grupoRestaurar:$filtro" }

    // Administrar grupo: lista rica de miembros, perfil, baneados, horario, historial y resumen (mismo formato que el módulo Go).
    private fun conteoJson(m: Int, tx: Int, im: Int, vi: Int, au: Int, nv: Int, ultimo: Long) =
        """{"mensajes":$m,"texto":$tx,"imagenes":$im,"videos":$vi,"audios":$au,"notasVoz":$nv,"stickers":0,"documentos":0,"otros":0,"borrados":2,"editados":1,"primerTs":${ultimo - 5000},"ultimoTs":$ultimo}"""
    private fun miembroAdminJson(persona: String, nombre: String, tel: String, rol: String, conteo: String, enLinea: Boolean = false, baneado: Boolean = false, rango: String = "") =
        """{"persona":"$persona","jid":"$persona","nombre":"$nombre","telefono":"$tel","rol":"$rol","rango":"$rango","rangoColor":"#2EC4CB","desdeTs":1700000000,"hastaTs":0,"mensajes":0,"ultimoTs":0,"silenciado":false,"advertido":false,"etiquetas":[],"foto":"","conteo":$conteo,"enLinea":$enLinea,"ultimaVez":0,"baneado":$baneado,"advertencias":0}"""
    var miembrosAdminJson = ""
    var baneadosJson = ""
    var horarioJson = """{"grupo":"g@g.us","activo":false,"cierra":"22:00","abre":"07:00","dias":127,"ultimo":""}"""
    var historialJson = ""
    var perfilAdminJson = ""
    val ahoraS = System.currentTimeMillis() / 1000
    private fun defectosAdmin() {
        if (miembrosAdminJson.isEmpty()) miembrosAdminJson = """{"desde":${ahoraS - 90000},"miembros":[""" +
            miembroAdminJson("5215511111111@s.whatsapp.net", "Ana", "+52 55 1111 1111", "admin", conteoJson(120, 80, 20, 5, 7, 8, ahoraS - 7200), enLinea = true, rango = "Moderadora") + "," +
            miembroAdminJson("5215522222222@s.whatsapp.net", "Luis", "+52 55 2222 2222", "miembro", conteoJson(0, 0, 0, 0, 0, 0, 0)) + "," +
            miembroAdminJson("5215533333333@s.whatsapp.net", "Rosa", "+52 55 3333 3333", "miembro", conteoJson(30, 25, 5, 0, 0, 0, ahoraS - 100 * 86400L), baneado = true) + "]}"
        if (baneadosJson.isEmpty()) baneadosJson = """[{"grupo":"g@g.us","persona":"5215533333333@s.whatsapp.net","nombre":"Rosa","telefono":"+52 55 3333 3333","ts":${ahoraS - 3600},"por":"yo","porNombre":"Yo","motivo":"vendía cosas"}]"""
        if (historialJson.isEmpty()) historialJson = """[{"ts":${ahoraS - 60},"clase":"accion","tipo":"grupo.banear","texto":"baneó a","actor":"yo","actorNombre":"Yo","persona":"5215533333333@s.whatsapp.net","personaNombre":"Rosa","resultado":"ok"},""" +
            """{"ts":${ahoraS - 7200},"clase":"cambio","tipo":"enlace","texto":"entró por el enlace","actor":"","actorNombre":"","persona":"5215522222222@s.whatsapp.net","personaNombre":"Luis","resultado":""}]"""
        if (perfilAdminJson.isEmpty() && JSONObject(miembrosAdminJson).getJSONArray("miembros").length() > 0) perfilAdminJson = """{"miembro":""" + JSONObject(miembrosAdminJson).getJSONArray("miembros").getJSONObject(0).toString() +
            ""","serie":[""" + (0 until 30).joinToString(",") { """{"dia":"2026-10-${"%02d".format(it % 28 + 1)}","n":${it % 4}}""" } + """],"horas":[""" + (0 until 24).joinToString(",") { "${it % 5}" } +
            """],"promedioDia":4.0,"mediaGrupo":75.0,"puesto":1,"activos":2,"motivoBan":"","silencioHasta":0,"silenciado":false,"historial":""" + historialJson + ""","desde":${ahoraS - 90000}}"""
    }
    var respuestasJson = JSONObject().put("grupos", JSONArray(listOf("Comandos en general", "Grupos")))
        .put("filas", JSONArray().put(JSONObject().put("id", "cmd.sin_permiso").put("grupo", "Comandos en general").put("descripcion", "Usan un comando que su rango no permite.").put("defecto", "No tienes permiso para usar ese comando.").put("vars", JSONArray()).put("propio", ""))
            .put(JSONObject().put("id", "grupo.aprobadas").put("grupo", "Grupos").put("descripcion", "Se aprueban solicitudes de ingreso.").put("defecto", "Listo: aprobé {n} solicitudes.").put("vars", JSONArray(listOf("n"))).put("propio", "")))
        .put("reemplazos", JSONArray()).toString()
    override fun respuestasLista(bot: Long) = respuestasJson
    override fun respuestaFijar(bot: Long, id: String, texto: String) {
        llamadas += "respuesta:$id:$texto"; rFalla()
        val o = JSONObject(respuestasJson); val f = o.getJSONArray("filas")
        for (i in 0 until f.length()) if (f.getJSONObject(i).getString("id") == id) f.getJSONObject(i).put("propio", texto)
        respuestasJson = o.toString()
    }
    // ---- configuración base, encuestas, fijar, avisos y copia ----
    var baseJson = """{"existe":false,"desde":"","fecha":0,"filtros":0,"reglas":false,"horario":false,"entrada":false,"auto":false}"""
    var avisosJson = """{"wa":false,"tipos":["reporte","grupos","conexion","resumen"],"desconexion":10,"resumen":false}"""
    var resumenTexto = "Resumen de la semana\n• Mensajes: 90"
    var copiaFalla: String? = null
    override fun grupoBaseInfo(bot: Long) = baseJson
    override fun grupoBaseGuardar(bot: Long, chat: String) { llamadas += "baseGuardar:$chat"; baseJson = JSONObject(baseJson).put("existe", true).put("desde", "Familia").put("fecha", 1763000000).put("filtros", 2).put("reglas", true).toString() }
    override fun grupoBaseQuitar(bot: Long) { llamadas += "baseQuitar"; baseJson = JSONObject(baseJson).put("existe", false).put("auto", false).toString() }
    override fun grupoBaseAuto(bot: Long, si: Boolean) { llamadas += "baseAuto:$si"; baseJson = JSONObject(baseJson).put("auto", si).toString() }
    override fun grupoBaseAplicar(bot: Long, gruposJson: String): Long { llamadas += "baseAplicar:$gruposJson"; return JSONArray(gruposJson).length().toLong() }
    override fun grupoEncuesta(bot: Long, chat: String, titulo: String, opcionesJson: String, multiple: Boolean) { llamadas += "encuesta:$titulo:$opcionesJson:$multiple"; gFalla() }
    override fun grupoFijar(bot: Long, chat: String, msgId: String, dias: Long, fijar: Boolean) { llamadas += "fijar:$msgId:$dias:$fijar"; gFalla() }
    override fun alertasLeer(bot: Long) = avisosJson
    override fun alertasGuardar(bot: Long, ajustesJson: String) { llamadas += "alertas:$ajustesJson"; avisosJson = ajustesJson }
    override fun resumenSemanalAhora(bot: Long): String { llamadas += "resumenSemanal"; return resumenTexto }
    override fun respaldoConfigExportar(bot: Long, contrasena: String): ByteArray { llamadas += "exportar:${contrasena.length}"; copiaFalla?.let { throw IllegalStateException(it) }; return "copia".toByteArray() }
    override fun respaldoConfigRestaurar(bot: Long, contrasena: String, datos: ByteArray): String { llamadas += "restaurar:${contrasena.length}:${datos.size}"; copiaFalla?.let { throw IllegalStateException(it) }; return """{"ajustes":4,"grupos":2,"filtros":5,"baneados":1,"base":true}""" }
    override fun respuestasRestaurar(bot: Long): Long { llamadas += "respuestas_restaurar"; val o = JSONObject(respuestasJson); val f = o.getJSONArray("filas"); for (i in 0 until f.length()) f.getJSONObject(i).put("propio", ""); respuestasJson = o.put("reemplazos", JSONArray()).toString(); return 1 }
    override fun reemplazosFijar(bot: Long, listaJson: String) { llamadas += "reemplazos:$listaJson"; respuestasJson = JSONObject(respuestasJson).put("reemplazos", JSONArray(listaJson)).toString() }
    var perfilBaneado = false
    override fun grupoMiembrosAdmin(bot: Long, grupo: String): String { defectosAdmin(); llamadas += "miembrosAdmin:$grupo"; return miembrosAdminJson }
    override fun grupoSuscribirPresencias(bot: Long, grupo: String) { llamadas += "suscribirPresencias:$grupo" }
    override fun grupoPerfilMiembro(bot: Long, grupo: String, persona: String): String {
        defectosAdmin(); llamadas += "perfilMiembro:$persona"
        val o = JSONObject(perfilAdminJson)
        val ms = JSONObject(miembrosAdminJson).getJSONArray("miembros")
        for (i in 0 until ms.length()) if (ms.getJSONObject(i).getString("persona") == persona) o.put("miembro", ms.getJSONObject(i))
        if (perfilBaneado) { o.getJSONObject("miembro").put("baneado", true); o.put("motivoBan", "vendía cosas") }
        return o.toString()
    }
    override fun grupoHistorial(bot: Long, grupo: String, antes: Long, limite: Long): String { defectosAdmin(); llamadas += "historial:$antes"; return if (antes > 0) "[]" else historialJson }
    override fun grupoBaneados(bot: Long, grupo: String): String { defectosAdmin(); return baneadosJson }
    override fun grupoBanear(bot: Long, grupo: String, persona: String, motivo: String) { llamadas += "grupoBanear:$persona:$motivo"; gFalla() }
    override fun grupoDesbanear(bot: Long, grupo: String, persona: String) { llamadas += "grupoDesbanear:$persona"; gFalla(); baneadosJson = "[]" }
    override fun grupoHorario(bot: Long, grupo: String) = horarioJson
    override fun grupoGuardarHorario(bot: Long, grupo: String, horarioJson: String) { llamadas += "guardarHorario:$horarioJson"; this.horarioJson = JSONObject(horarioJson).put("grupo", grupo).put("ultimo", "").toString() }
    var entradaJson = """{"grupo":"g@g.us","raidActivo":false,"raidMax":8,"raidMinutos":2,"prefijos":[]}"""
    override fun grupoEntrada(bot: Long, grupo: String) = entradaJson
    override fun grupoGuardarEntrada(bot: Long, grupo: String, entradaJson: String) { llamadas += "guardarEntrada:$entradaJson"; this.entradaJson = entradaJson }
    var resumenAdminJson = ""
    override fun grupoResumenAdmin(bot: Long, grupo: String): String {
        defectosAdmin()
        if (resumenAdminJson.isEmpty()) resumenAdminJson = """{"miembros":3,"admins":1,"mensajesHoy":12,"mensajes7":90,"activos7":2,"inactivos30":1,"baneados":1,"silenciados":0,"advertidos":1,"serie":[""" +
            (0 until 14).joinToString(",") { """{"dia":"2026-11-${"%02d".format(it + 2)}","n":${it * 3}}""" } + """],"horario":""" + horarioJson + ""","top":[""" +
            JSONObject(miembrosAdminJson).getJSONArray("miembros").getJSONObject(0).toString() + "]}"
        return JSONObject(resumenAdminJson).put("horario", JSONObject(horarioJson)).toString()
    }
    override fun appStateDisponible(bot: Long) = JSONObject(appStateJson).getBoolean("disponible")
    override fun appStateEstado(bot: Long) = appStateJson
    override fun appStateSilenciar(bot: Long, chat: String, segundos: Long) { llamadas += "appSilenciar:$chat:$segundos"; gFalla() }
    override fun appStateDesilenciar(bot: Long, chat: String) { llamadas += "appDesilenciar:$chat" }
    override fun appStateFijar(bot: Long, chat: String, fijar: Boolean) { llamadas += "appFijar:$chat:$fijar" }
    override fun appStateArchivar(bot: Long, chat: String, archivar: Boolean) { llamadas += "appArchivar:$chat:$archivar" }
    override fun appStateSincronizar(bot: Long) { llamadas += "appSincronizar" }
}

object Apoyo {
    val clave = ByteArray(32) { (it + 1).toByte() }

    fun fichaEjemplo(persona: String = "5215511111111@s.whatsapp.net", nombre: String = "Ana Pérez") = JSONObject()
        .put("persona", persona).put("alias", JSONArray().put(persona)).put("nombre", nombre).put("telefono", "+52 55 1111 1111")
        .put("silencio", JSONObject.NULL).put("bloqueada", false).put("noMolestar", false).put("advertencias", 0).put("notas", 0)
        .put("etiquetas", JSONArray()).put("primeraTs", 1_700_000_000L).put("ultimaTs", 1_700_100_000L).put("recibidos", 12).put("enviados", 5).put("grupos", 1)
        .put("rango", "").put("rangoColor", "").toString()

    fun casillaJson(id: String, nombre: String, grupo: String, riesgo: String = "bajo", privado: Boolean = true, grupos: Boolean = true, confirma: Boolean = false, base: Boolean = false) =
        JSONObject().put("id", id).put("nombre", nombre).put("descripcion", "Descripción de $nombre").put("grupo", grupo).put("riesgo", riesgo)
            .put("privado", privado).put("grupos", grupos).put("confirma", confirma).put("base", base)

    val catalogoEjemplo = arreglo(
        casillaJson("cmd.ayuda", "Ver la ayuda", "Lo básico", base = true),
        casillaJson("personas.silenciar", "Ignorar a una persona", "Personas"),
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

    fun moduloJson(id: String, nombre: String, descripcion: String, icono: String, riesgo: String, iniciativa: Boolean, esquema: JSONArray? = null, paneles: JSONArray? = null) =
        JSONObject().put("id", id).put("nombre", nombre).put("descripcion", descripcion).put("icono", icono).put("riesgo", riesgo).put("iniciativa", iniciativa)
            .put("activo", false).put("pausado", false).put("codigo", "apagado").put("estado", "Apagado").put("conEsquema", esquema != null)
            .put("topeHora", 5).put("topeDia", 20).put("defHora", 5).put("defDia", 20).put("activadoTs", 0)
            .apply { if (esquema != null) put("esquema", esquema); if (paneles != null) put("paneles", paneles) }

    /** Un panel de ejemplo como los que describe Go: una lista con formulario (texto, lista larga y archivos) y botones. */
    fun panelEjemplo(id: String = "listas") = JSONObject().put("id", id).put("titulo", "Listas de destinatarios").put("ayuda", "Solo se escribe a quien ya te escribió.")
        .put("alta", "Agregar a la lista")
        .put("campos", JSONArray()
            .put(JSONObject().put("clave", "lista").put("titulo", "Lista").put("tipo", "texto").put("defecto", "suscritos").put("largo", 30))
            .put(JSONObject().put("clave", "persona").put("titulo", "Persona").put("tipo", "lista").put("opciones", JSONArray().apply {
                for (i in 1..8) put(JSONObject().put("valor", "p$i").put("titulo", "Persona $i"))
            }))
            .put(JSONObject().put("clave", "adjuntos").put("titulo", "Archivos").put("tipo", "adjuntos")))
        .put("extras", JSONArray().put(JSONObject().put("accion", "$id.quitar").put("titulo", "Quitar de la lista").put("confirma", "¿Quitar a esta persona?")).put(JSONObject().put("accion", "$id.vista").put("titulo", "Vista previa")))
        .put("porFila", JSONArray().put(JSONObject().put("accion", "$id.vaciar").put("titulo", "Vaciar").put("confirma", "¿Vaciar esta lista?")))

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
        bytes: Long = 0, ruta: String = "", miniatura: String = "", borrado: Boolean = false, editado: Boolean = false, puedeBajarse: Boolean = false, dur: Int = 0, nombre: String = "", notaVoz: Boolean = false, automatico: String = "",
    ) = JSONObject().put("id", id).put("chat", "chat").put("remitenteId", remitenteId).put("remitente", remitente).put("esMio", mio).put("tipo", tipo).put("texto", texto).put("ts", ts)
        .put("borrado", borrado).put("editado", editado).put("estado", estado).put("bytes", bytes).put("mime", "").put("nombre", nombre).put("ruta", ruta).put("miniatura", miniatura)
        .put("ancho", 0).put("alto", 0).put("dur", dur).put("notaVoz", notaVoz).put("puedeBajarse", puedeBajarse)
        .put("estadoEnvio", if (mio) "enviado" else "").put("menciones", JSONArray()).put("reacciones", JSONArray()).put("automatico", automatico)

    fun arreglo(vararg o: JSONObject) = JSONArray().also { a -> o.forEach { a.put(it) } }.toString()

    fun carpetaTemporal(): File = File(ctx().cacheDir, "pruebas").apply { mkdirs() }
}
