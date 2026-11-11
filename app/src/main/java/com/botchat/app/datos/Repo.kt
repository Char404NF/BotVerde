package com.botchat.app.datos

import android.content.Context
import com.botchat.app.seguridad.ClaveDatos
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.withContext
import org.json.JSONArray
import wabot.Observador
import java.io.File

/** Acceso a los mensajes guardados (módulo Go). Todo corre fuera del hilo principal. */
object Repo {
    /** El motor de datos (el módulo Go). Las pruebas lo cambian por uno de ejemplo. */
    @Volatile var motor: Motor = MotorGo

    private lateinit var raiz: File
    private val cambiosInternos = MutableSharedFlow<List<String>>(extraBufferCapacity = 64)

    /** Eventos en vivo: "chats", "chat:<jid>", "msg:<jid>:<id>" (cambió ese mensaje), "archivo:<jid>:<id>" y "espacio". */
    val cambios: SharedFlow<List<String>> = cambiosInternos.asSharedFlow()

    /** Mensaje de error si no se pudo abrir el almacén (por ejemplo, la llave de datos no está disponible). */
    val error = MutableStateFlow<String?>(null)

    @Volatile var abierto = false
        private set

    private val observador = Observador { json ->
        runCatching {
            val a = JSONArray(json)
            cambiosInternos.tryEmit(List(a.length()) { a.getString(it) })
        }
    }

    /** Abre la base de mensajes. Se puede llamar varias veces. */
    @Synchronized
    fun abrir(ctx: Context): Boolean {
        if (abierto) return true
        return try {
            raiz = ctx.applicationContext.filesDir
            motor.abrirAlmacen(raiz.absolutePath, ClaveDatos.obtener(ctx), observador)
            abierto = true
            actualizarZona()
            com.botchat.app.datos.Ajustes.get(ctx).aplicarAGo()
            error.value = null
            true
        } catch (e: Exception) {
            error.value = e.message ?: "No se pudieron abrir los mensajes guardados"
            false
        }
    }

    /** Avisa al módulo Go la zona horaria del teléfono para agrupar mensajes por día (la primera vez cuenta también los mensajes anteriores). */
    fun actualizarZona() {
        if (!abierto) return
        runCatching { motor.establecerZona(java.util.TimeZone.getDefault().id) }
    }

    fun archivo(rutaRelativa: String): File = File(raiz, rutaRelativa)

    /** Solo para pruebas: indica la carpeta de datos y que el almacén está abierto. */
    fun paraPruebas(carpeta: File, m: Motor) { raiz = carpeta; motor = m; abierto = true; error.value = null }

    /** Solo para pruebas: simula un aviso de cambio que normalmente manda el módulo Go. */
    fun emitirParaPruebas(vararg eventos: String) { cambiosInternos.tryEmit(eventos.toList()) }

    private suspend fun <T> io(f: () -> T): T = withContext(Dispatchers.IO) { f() }

    suspend fun chat(bot: Long, jid: String) = io { Json.chatOpt(motor.obtenerChat(bot, jid)) }

    suspend fun chats(bot: Long, grupos: Boolean, consulta: String, limite: Int = 300, desde: Int = 0) =
        io { Json.chats(motor.listarChats(bot, grupos, consulta, limite.toLong(), desde.toLong())) }

    /** Del más nuevo al más viejo. Para la página siguiente se pasan hora e id del último recibido. */
    suspend fun mensajes(bot: Long, chat: String, antesTs: Long = 0, antesId: String = "", limite: Int = 60) =
        io { Json.mensajes(motor.listarMensajes(bot, chat, antesTs, antesId, limite.toLong())) }

    /** Pide solo estos mensajes (una consulta por lote): sirve para refrescar los ya cargados. Los que ya no existen no vuelven. */
    suspend fun mensajesPorId(bot: Long, chat: String, ids: List<String>) =
        io { Json.mensajes(motor.mensajesPorId(bot, chat, JSONArray(ids).toString())) }

    /** Número real de mensajes sin leer (de otras personas, posteriores a `desdeTs`), contado en la base y no solo en lo cargado. */
    suspend fun contarNoLeidos(bot: Long, chat: String, desdeTs: Long): Int = io { motor.contarNoLeidos(bot, chat, desdeTs).toInt() }

    /** Solo para medir: carga los mensajes como en la 0.4.0 (nombres uno por uno, sin caché). */
    suspend fun cargaAnterior(si: Boolean) = io { motor.cargaAnterior(si) }
    suspend fun crearChatPrueba(bot: Long, cuantos: Int) = io { motor.crearChatPrueba(bot, cuantos.toLong()) }

    suspend fun archivos(bot: Long, chat: String, categoria: String, soloDescargados: Boolean, limite: Int = 300, desde: Int = 0) =
        io { Json.mensajes(motor.listarArchivos(bot, chat, categoria, soloDescargados, limite.toLong(), desde.toLong())) }

    suspend fun filtrados(bot: Long, chat: String, filtro: FiltroLista, antesTs: Long = 0, antesId: String = "", limite: Int = 60) =
        io { Json.pagina(motor.listarFiltrados(bot, chat, filtro.json(), antesTs, antesId, limite.toLong())) }
    suspend fun personas(bot: Long, chat: String) = io { Json.personas(motor.personasChat(bot, chat)) }
    suspend fun olvidar(bot: Long, chat: String, id: String) = io { motor.olvidarMensaje(bot, chat, id) }

    suspend fun enviarTexto(bot: Long, chat: String, texto: String, citaId: String = "", menciones: List<String> = emptyList()) =
        io { Json.envio(motor.enviarTexto(bot, chat, texto, citaId, JSONArray(menciones).toString())) }
    suspend fun reintentarEnvio(bot: Long, chat: String, id: String) = io { Json.envio(motor.reintentarEnvio(bot, chat, id)) }
    suspend fun reaccionar(bot: Long, chat: String, id: String, emoji: String) = io { motor.reaccionar(bot, chat, id, emoji) }
    suspend fun editar(bot: Long, chat: String, id: String, texto: String) = io { motor.editarMensaje(bot, chat, id, texto) }
    suspend fun borrarParaTodos(bot: Long, chat: String, id: String) = io { motor.borrarParaTodos(bot, chat, id) }
    suspend fun borrarParaMi(bot: Long, chat: String, ids: List<String>) = io { motor.borrarParaMi(bot, chat, JSONArray(ids).toString()) }
    suspend fun reenviar(bot: Long, origen: String, ids: List<String>, destinos: List<String>) =
        io { Json.reenvio(motor.reenviarTexto(bot, origen, JSONArray(ids).toString(), JSONArray(destinos).toString())) }
    suspend fun marcarLeido(bot: Long, chat: String) = io { motor.marcarLeido(bot, chat) }
    suspend fun escribiendo(bot: Long, chat: String, si: Boolean) = io { motor.escribiendo(bot, chat, si) }
    suspend fun enLinea(bot: Long, si: Boolean) = io { motor.enLinea(bot, si) }
    suspend fun suscribirPresencia(bot: Long, chat: String) = io { motor.suscribirPresencia(bot, chat) }
    suspend fun presencia(chat: String) = io { Json.presencia(motor.presenciaDe(chat)) }
    suspend fun participantes(bot: Long, chat: String) = io { Json.participantes(motor.participantesGrupo(bot, chat)) }
    suspend fun infoMensaje(bot: Long, chat: String, id: String) = io { Json.infoMensaje(motor.infoMensaje(bot, chat, id)) }
    suspend fun guardarBorrador(bot: Long, chat: String, texto: String) = io { motor.guardarBorrador(bot, chat, texto) }
    suspend fun borrador(bot: Long, chat: String) = io { motor.leerBorrador(bot, chat) }
    suspend fun silenciar(bot: Long, chat: String, si: Boolean) = io { motor.silenciarChat(bot, chat, si) }
    suspend fun estadoChat(bot: Long, chat: String) = io { Json.estadoChat(motor.estadoDelChat(bot, chat)) }
    suspend fun plazoEdicionSeg() = io { motor.plazoEdicionSeg() }

    suspend fun enviarMedio(
        bot: Long, chat: String, tipo: String, ruta: String, mime: String, nombre: String, pie: String, citaId: String,
        ancho: Int, alto: Int, durS: Int, notaVoz: Boolean, miniatura: ByteArray?, onda: ByteArray?,
    ) = io { Json.envio(motor.enviarMedio(bot, chat, tipo, ruta, mime, nombre, pie, citaId, ancho.toLong(), alto.toLong(), durS.toLong(), notaVoz, miniatura, onda)) }
    suspend fun abrirVerUnaVez(bot: Long, chat: String, id: String) = io { Json.verUnaVez(motor.abrirVerUnaVez(bot, chat, id)) }
    suspend fun cerrarVerUnaVez(bot: Long, chat: String, id: String) = io { motor.cerrarVerUnaVez(bot, chat, id) }
    suspend fun barrerVerUnaVez(bot: Long) = io { motor.barrerVerUnaVez(bot) }

    suspend fun actividadMotor(bot: Long, limite: Int = 200) = io { Json.actividad(motor.actividadMotor(bot, limite.toLong())) }
    suspend fun botEnChat(bot: Long, chat: String, activo: Boolean) = io { motor.botEnChat(bot, chat, activo) }
    suspend fun pausarBotEnChat(bot: Long, chat: String, minutos: Int) = io { motor.pausarBotEnChat(bot, chat, minutos.toLong()) }

    suspend fun automatizaciones(bot: Long) = io { JsonAuto.resumenes(motor.listarAutomatizaciones(bot)) }
    suspend fun automatizacion(bot: Long, id: Long) = io { JsonAuto.detalle(motor.obtenerAutomatizacion(bot, id)) }
    suspend fun validar(def: AutoDef) = io { JsonAuto.validacion(motor.validarAutomatizacion(def.json())) }
    suspend fun guardarAutomatizacion(bot: Long, id: Long, def: AutoDef) = io { JsonAuto.guardado(motor.guardarAutomatizacion(bot, id, def.json())) }
    suspend fun activarAutomatizacion(bot: Long, id: Long, si: Boolean) = io { motor.activarAutomatizacion(bot, id, si) }
    suspend fun duplicarAutomatizacion(bot: Long, id: Long) = io { motor.duplicarAutomatizacion(bot, id) }
    suspend fun borrarAutomatizacion(bot: Long, id: Long) = io { motor.borrarAutomatizacion(bot, id) }
    suspend fun plantillasAutomatizacion() = io { JsonAuto.plantillas(motor.plantillasAutomatizacion()) }
    suspend fun exportarAutomatizacion(bot: Long, id: Long) = io { motor.exportarAutomatizacion(bot, id) }
    suspend fun importarAutomatizacion(bot: Long, json: String) = io { JsonAuto.guardado(motor.importarAutomatizacion(bot, json)) }
    suspend fun simular(bot: Long, id: Long, texto: String, reiniciar: Boolean) = io { JsonAuto.prueba(motor.simularAutomatizacion(bot, id, texto, reiniciar)) }
    suspend fun sesionesEnCurso(bot: Long) = io { JsonAuto.sesiones(motor.sesionesEnCurso(bot)) }
    suspend fun recorrido(bot: Long, id: Long) = io { JsonAuto.recorrido(motor.recorridoSesion(bot, id)) }
    suspend fun accionSesion(bot: Long, id: Long, accion: String) = io { motor.accionSesion(bot, id, accion) }
    suspend fun estadisticasMenu(bot: Long, id: Long, desde: String, hasta: String, prueba: Boolean) = io { JsonAuto.estadisticas(motor.estadisticasAutomatizacion(bot, id, desde, hasta, prueba)) }
    suspend fun etiquetaChat(bot: Long, chat: String) = io { JsonAuto.etiquetas(motor.etiquetaDeChat(bot, chat)) }
    suspend fun aviso(bot: Long, id: Long) = io { JsonAuto.aviso(motor.obtenerAviso(bot, id)) }

    suspend fun estadisticasChat(bot: Long, chat: String) = io { Json.resumenChat(motor.estadisticasChat(bot, chat)) }
    suspend fun calendario(bot: Long, chat: String, mes: String) = io { Json.calendario(motor.calendarioChat(bot, chat, mes)) }
    suspend fun mensajesDia(bot: Long, chat: String, dia: String) = io { Json.mensajes(motor.mensajesDia(bot, chat, dia)) }
    suspend fun estadisticasGlobales(bot: Long, dias: Int = 30) = io { Json.resumenGlobal(motor.estadisticasGlobales(bot, dias.toLong())) }

    suspend fun espacioTotal(bot: Long) = io { Json.espacioTotal(motor.espacioTotal(bot)) }
    suspend fun espacioPorTipo(bot: Long) = io { Json.espacioPorTipo(motor.espacioPorTipo(bot)) }
    suspend fun espacioPorChat(bot: Long) = io { Json.espacioPorChat(motor.espacioPorChat(bot)) }

    suspend fun estimarBorrado(bot: Long, chat: String = "", categoria: String = "", diasMin: Int = 0) =
        io { Json.liberado(motor.estimarBorrado(bot, Json.filtro(chat, categoria, diasMin))) }

    suspend fun borrarArchivos(bot: Long, chat: String = "", categoria: String = "", diasMin: Int = 0) =
        io { Json.liberado(motor.borrarArchivos(bot, Json.filtro(chat, categoria, diasMin))) }

    suspend fun estimarLista(bot: Long, items: List<Pair<String, String>>) = io { Json.liberado(motor.estimarLista(bot, Json.items(items))) }
    suspend fun borrarLista(bot: Long, items: List<Pair<String, String>>) = io { Json.liberado(motor.borrarLista(bot, Json.items(items))) }
    suspend fun borrarChat(bot: Long, chat: String) = io { Json.liberado(motor.borrarChat(bot, chat)) }
    suspend fun reparar(bot: Long) = io { Json.reporte(motor.reparar(bot)) }
    suspend fun descargar(bot: Long, chat: String, id: String) = io { motor.descargar(bot, chat, id) }
    suspend fun fotoGrande(bot: Long, chat: String) = io { motor.fotoGrande(bot, chat) }
}
