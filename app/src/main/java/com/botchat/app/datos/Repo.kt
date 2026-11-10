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

    /** Eventos en vivo: "chats", "chat:<jid>", "archivo:<jid>:<id>" y "espacio". */
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

    suspend fun archivos(bot: Long, chat: String, categoria: String, soloDescargados: Boolean, limite: Int = 300, desde: Int = 0) =
        io { Json.mensajes(motor.listarArchivos(bot, chat, categoria, soloDescargados, limite.toLong(), desde.toLong())) }

    suspend fun filtrados(bot: Long, chat: String, filtro: FiltroLista, antesTs: Long = 0, antesId: String = "", limite: Int = 60) =
        io { Json.pagina(motor.listarFiltrados(bot, chat, filtro.json(), antesTs, antesId, limite.toLong())) }
    suspend fun personas(bot: Long, chat: String) = io { Json.personas(motor.personasChat(bot, chat)) }
    suspend fun olvidar(bot: Long, chat: String, id: String) = io { motor.olvidarMensaje(bot, chat, id) }

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
