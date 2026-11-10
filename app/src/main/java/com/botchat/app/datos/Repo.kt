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
import wabot.Wabot
import java.io.File

/** Acceso a los mensajes guardados (módulo Go). Todo corre fuera del hilo principal. */
object Repo {
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
            Wabot.abrirAlmacen(raiz.absolutePath, ClaveDatos.obtener(ctx), observador)
            abierto = true
            error.value = null
            true
        } catch (e: Exception) {
            error.value = e.message ?: "No se pudieron abrir los mensajes guardados"
            false
        }
    }

    fun archivo(rutaRelativa: String): File = File(raiz, rutaRelativa)

    private suspend fun <T> io(f: () -> T): T = withContext(Dispatchers.IO) { f() }

    suspend fun chats(bot: Long, grupos: Boolean, consulta: String, limite: Int = 300, desde: Int = 0) =
        io { Json.chats(Wabot.listarChats(bot, grupos, consulta, limite.toLong(), desde.toLong())) }

    /** Del más nuevo al más viejo. Para la página siguiente se pasan hora e id del último recibido. */
    suspend fun mensajes(bot: Long, chat: String, antesTs: Long = 0, antesId: String = "", limite: Int = 60) =
        io { Json.mensajes(Wabot.listarMensajes(bot, chat, antesTs, antesId, limite.toLong())) }

    suspend fun archivos(bot: Long, chat: String, categoria: String, soloDescargados: Boolean, limite: Int = 300, desde: Int = 0) =
        io { Json.mensajes(Wabot.listarArchivos(bot, chat, categoria, soloDescargados, limite.toLong(), desde.toLong())) }

    suspend fun espacioTotal(bot: Long) = io { Json.espacioTotal(Wabot.espacioTotal(bot)) }
    suspend fun espacioPorTipo(bot: Long) = io { Json.espacioPorTipo(Wabot.espacioPorTipo(bot)) }
    suspend fun espacioPorChat(bot: Long) = io { Json.espacioPorChat(Wabot.espacioPorChat(bot)) }

    suspend fun estimarBorrado(bot: Long, chat: String = "", categoria: String = "", diasMin: Int = 0) =
        io { Json.liberado(Wabot.estimarBorrado(bot, Json.filtro(chat, categoria, diasMin))) }

    suspend fun borrarArchivos(bot: Long, chat: String = "", categoria: String = "", diasMin: Int = 0) =
        io { Json.liberado(Wabot.borrarArchivos(bot, Json.filtro(chat, categoria, diasMin))) }

    suspend fun estimarLista(bot: Long, items: List<Pair<String, String>>) = io { Json.liberado(Wabot.estimarLista(bot, Json.items(items))) }
    suspend fun borrarLista(bot: Long, items: List<Pair<String, String>>) = io { Json.liberado(Wabot.borrarLista(bot, Json.items(items))) }
    suspend fun borrarChat(bot: Long, chat: String) = io { Json.liberado(Wabot.borrarChat(bot, chat)) }
    suspend fun reparar(bot: Long) = io { Json.reporte(Wabot.reparar(bot)) }
    suspend fun descargar(bot: Long, chat: String, id: String) = io { Wabot.descargar(bot, chat, id) }
    suspend fun fotoGrande(bot: Long, chat: String) = io { Wabot.fotoGrande(bot, chat) }
}
