package com.botchat.app.ui

import android.app.Application
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.botchat.app.Datos
import com.botchat.app.datos.Ajustes
import com.botchat.app.datos.ChatItem
import com.botchat.app.datos.EstadoChat
import com.botchat.app.datos.InfoMsg
import com.botchat.app.datos.Json
import com.botchat.app.datos.ModoCarga
import com.botchat.app.datos.Msg
import com.botchat.app.datos.Participante
import com.botchat.app.datos.PresenciaChat
import com.botchat.app.datos.Repo
import com.botchat.app.medios.AdjuntoListo
import com.botchat.app.medios.GrabadorVoz
import com.botchat.app.medios.Grabadores
import com.botchat.app.medios.PrepararAdjunto
import androidx.compose.ui.graphics.Color
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/** Qué chat está abierto en pantalla ahora (para no avisar con notificación de lo que ya se está viendo). */
/** Lo que dura la animación de entrada de la pantalla (con un margen): hasta entonces no se hace trabajo que no se vea. */
const val ENTRADA_MS = 380L

object ChatAbierto {
    @Volatile var jid: String = ""
}

/** Estado de un chat abierto: encabezado y mensajes (del más nuevo al más viejo), con carga por páginas, cambios en vivo y envío manual. */
class ChatVm(app: Application, val jid: String) : AndroidViewModel(app) {
    val botId: Long = Datos.get(app).primero()?.id ?: 0L

    var chat by mutableStateOf<ChatItem?>(null)
        private set
    var mensajes by mutableStateOf<List<Msg>?>(null) // null = cargando
        private set
    var hayMas by mutableStateOf(true)
        private set
    var cargandoMas by mutableStateOf(false)
        private set
    var aviso by mutableStateOf<String?>(null)

    // ---- escribir ----
    var citando by mutableStateOf<Msg?>(null)
    var editando by mutableStateOf<Msg?>(null)
    var enviando by mutableStateOf(false)
        private set
    var borradorInicial by mutableStateOf<String?>(null) // null = todavía cargando
        private set
    var participantes by mutableStateOf<List<Participante>>(emptyList())
        private set
    var presencia by mutableStateOf(PresenciaChat())
        private set
    var estadoChat by mutableStateOf(EstadoChat())
        private set
    var plazoEdicionSeg by mutableStateOf(1200L)
        private set
    var etiquetasMenu by mutableStateOf<List<com.botchat.app.datos.EtiquetaMenu>>(emptyList())
        private set

    // ---- adjuntos y notas de voz ----
    var adjuntos by mutableStateOf<List<AdjuntoListo>>(emptyList())
        private set
    var preparandoAdjuntos by mutableStateOf(false)
        private set
    var enviandoAdjuntos by mutableStateOf(false)
        private set
    var grabando by mutableStateOf(false)
        private set
    var segundosGrabando by mutableStateOf(0)
        private set
    private var generacionAdjuntos = 0
    private var grabador: GrabadorVoz? = null
    private var cronometro: Job? = null

    /** Hora (segundos) hasta la que ya se había visto este chat antes de abrirlo: lo posterior es "sin leer". */
    val vistoAntes: Long

    private var trabajo: Job? = null
    private var tecleo: Job? = null
    private var borradorJob: Job? = null
    private var leidoJob: Job? = null

    /** Con "Medir con carga anterior" se reproduce la forma de abrir de la 0.4.0 (páginas de 60 y recarga completa) para comparar. */
    private val antiguo = ModoCarga.anterior
    private val pagina = if (antiguo) 60 else 40

    private val ajustes = Ajustes.get(app)

    /** Lo que se dibuja: la lista armada (separadores de día, agrupación, "sin leer"), calculada fuera del hilo principal. */
    internal var elementos by mutableStateOf<List<Elem>>(emptyList())
        private set

    /** Color de los enlaces del tema, para dejar el texto formateado listo antes de dibujar. */
    @Volatile var colorTexto: Color = Color.Unspecified

    /** Se completa cuando termina la animación de entrada: hasta entonces solo se carga lo que se ve. */
    private val entrada = CompletableDeferred<Unit>()

    /** Verdadero cuando ya corrió el trabajo diferido (leído, presencia, participantes, extras). */
    var diferidoListo by mutableStateOf(false)
        private set

    private val candado = Mutex()
    private val idsPendientes = LinkedHashSet<String>() // ids de mensajes que cambiaron (solo en el hilo principal)

    init {
        vistoAntes = ajustes.visto(jid)
        ajustes.marcarVisto(jid)
        if (antiguo) {
            entrada.complete(Unit)
            recargarAntiguo()
            cargarExtras(conBorrador = true)
            diferidoListo = true
        } else {
            abrir()
            viewModelScope.launch { delay(ENTRADA_MS); entrada.complete(Unit); diferidos() }
        }
        viewModelScope.launch {
            Repo.cambios.collect { evs ->
                val pref = "msg:$jid:"
                val prefArchivo = "archivo:$jid:"
                val tocados = evs.filter { it.startsWith(pref) || it.startsWith(prefArchivo) }
                    .map { it.removePrefix(if (it.startsWith(pref)) pref else prefArchivo) }
                if (antiguo) {
                    if (evs.any { it == "chat:$jid" || it.startsWith(prefArchivo) || it == "chats" }) {
                        trabajo?.cancel()
                        trabajo = launch { delay(80); recargarAntiguo() }
                    }
                } else {
                    val nuevos = evs.any { it == "chat:$jid" }
                    if (tocados.isNotEmpty() || nuevos) {
                        idsPendientes.addAll(tocados)
                        trabajo?.cancel()
                        trabajo = launch { delay(80); refrescar() }
                    }
                    if (evs.any { it == "chats" }) launch { refrescarEncabezado() }
                }
                if (evs.any { it == "presencia:$jid" }) actualizarPresencia()
                if (evs.any { it == "sesiones" || it == "chat:$jid" }) actualizarEtiqueta()
            }
        }
    }

    private fun actualizarEtiqueta() {
        viewModelScope.launch { etiquetasMenu = runCatching { Repo.etiquetaChat(botId, jid) }.getOrDefault(emptyList()) }
    }

    private fun cargarExtras(conBorrador: Boolean) {
        actualizarEtiqueta()
        viewModelScope.launch {
            if (conBorrador) borradorInicial = runCatching { Repo.borrador(botId, jid) }.getOrDefault("")
            estadoChat = runCatching { Repo.estadoChat(botId, jid) }.getOrDefault(EstadoChat())
            plazoEdicionSeg = runCatching { Repo.plazoEdicionSeg() }.getOrDefault(1200L).takeIf { it > 0 } ?: 1200L
        }
    }

    // ---------- carga ----------

    /**
     * Abre el chat: lo último que se vio de él (si ya se abrió) aparece al instante, y la primera página pequeña llega de la base.
     * La animación de entrada no espera nada de esto; el contenido aparece encima.
     */
    private fun abrir() {
        viewModelScope.launch {
            CachePaginas.obtener(botId, jid)?.takeIf { it.isNotEmpty() }?.let { enCache ->
                if (iniciales == null) iniciales = enCache.mapTo(HashSet()) { it.id }
                hayMas = true
                actualizar { enCache }
            }
        }
        viewModelScope.launch { chat = runCatching { Repo.chat(botId, jid) }.getOrNull(); recalcularElementos() }
        viewModelScope.launch { borradorInicial = runCatching { Repo.borrador(botId, jid) }.getOrDefault("") }
        viewModelScope.launch {
            try {
                val m = Repo.mensajes(botId, jid, limite = pagina)
                CachePaginas.guardar(botId, jid, m)
                if (iniciales == null) iniciales = m.mapTo(HashSet()) { it.id }
                hayMas = m.size >= pagina
                actualizar { MezclaMensajes.mezclar(it ?: emptyList(), m, pagina) }
            } catch (e: Exception) {
                if (mensajes == null) { mensajes = emptyList(); elementos = emptyList() }
                aviso = e.message
            }
        }
    }

    /** Lo que se puede hacer después de que termina la animación: extras del encabezado, participantes y marcar como leído. */
    private fun diferidos() {
        cargarExtras(conBorrador = false)
        if (chat?.esGrupo == true && participantes.isEmpty()) cargarParticipantes()
        programarLeido()
        diferidoListo = true
    }

    /**
     * Aplica un cambio a la lista: la mezcla y el armado de renglones corren en un hilo de fondo, de uno en uno, y el resultado
     * (la lista y los renglones juntos) se publica de golpe. Si nada cambió, no se toca nada.
     */
    private suspend fun actualizar(cambio: (List<Msg>?) -> List<Msg>) {
        candado.withLock {
            val antes = mensajes
            val esGrupo = chat?.esGrupo == true
            val color = colorTexto
            val (despues, el) = withContext(Dispatchers.Default) {
                val d = cambio(antes)
                if (d === antes && antes != null) return@withContext d to null
                CacheTexto.precalentar(d, color)
                d to elementos(d, esGrupo, vistoAntes)
            }
            if (el == null) return
            mensajes = despues
            elementos = el
        }
    }

    private suspend fun recalcularElementos() {
        candado.withLock {
            val ms = mensajes ?: return
            val esGrupo = chat?.esGrupo == true
            elementos = withContext(Dispatchers.Default) { elementos(ms, esGrupo, vistoAntes) }
        }
    }

    private suspend fun refrescarEncabezado() {
        val c = runCatching { Repo.chat(botId, jid) }.getOrNull()
        val cambioGrupo = c?.esGrupo != chat?.esGrupo
        chat = c
        if (cambioGrupo) recalcularElementos()
    }

    /**
     * Un cambio en el chat: se vuelve a leer SOLO la primera página (mensajes nuevos) y, por id y en un solo lote, los mensajes ya
     * cargados más viejos que cambiaron (recibos, reacciones, ediciones, eliminados, estado de envío, descargas). Se mezcla sin duplicados y en orden.
     */
    private suspend fun refrescar() {
        val pedidos = idsPendientes.toList()
        idsPendientes.clear()
        try {
            val primera = Repo.mensajes(botId, jid, limite = pagina)
            val enPrimera = primera.mapTo(HashSet()) { it.id }
            val cargados = mensajes?.mapTo(HashSet()) { it.id } ?: emptySet<String>()
            val viejos = pedidos.filter { it !in enPrimera && it in cargados }.distinct()
            val porId = if (viejos.isEmpty()) emptyList() else Repo.mensajesPorId(botId, jid, viejos)
            CachePaginas.guardar(botId, jid, primera)
            actualizar { MezclaMensajes.mezclar(it ?: emptyList(), primera, pagina, porId, viejos.toSet()) }
            if (mensajes == null) mensajes = emptyList()
            ajustes.marcarVisto(jid) // el chat está abierto: lo que llega ahora ya se ve
            runCatching { Repo.estadoChat(botId, jid) }.onSuccess { estadoChat = it }
            runCatching { Repo.chat(botId, jid) }.getOrNull()?.let { chat = it }
            programarLeido()
        } catch (e: kotlinx.coroutines.CancellationException) {
            idsPendientes.addAll(pedidos)
            throw e
        } catch (e: Exception) {
            idsPendientes.addAll(pedidos)
            aviso = e.message
        }
    }

    // ---- ruta antigua (0.4.0), solo para comparar al medir ----

    private fun recargarAntiguo() {
        viewModelScope.launch {
            try {
                val cuantos = maxOf(pagina, mensajes?.size ?: 0)
                val c = runCatching { Repo.chat(botId, jid) }.getOrNull()
                chat = c
                runCatching { Repo.estadoChat(botId, jid) }.onSuccess { estadoChat = it }
                val m = Repo.mensajes(botId, jid, limite = cuantos)
                if (iniciales == null) iniciales = m.mapTo(HashSet()) { it.id }
                mensajes = m
                elementos = elementos(m, c?.esGrupo == true, vistoAntes)
                hayMas = m.size >= cuantos
                ajustes.marcarVisto(jid)
                if (c?.esGrupo == true && participantes.isEmpty()) cargarParticipantes()
                programarLeido()
            } catch (e: Exception) {
                mensajes = mensajes ?: emptyList()
                aviso = e.message
            }
        }
    }

    private var iniciales: Set<String>? = null

    /** Un mensaje es "nuevo" si llegó con el chat ya abierto (para animar su entrada). */
    fun esNueva(id: String): Boolean = iniciales?.let { id !in it } ?: false

    override fun onCleared() {
        adjuntos.forEach { it.archivo.delete() }
        grabador?.cancelar()
        ajustes.marcarVisto(jid)
        if (ChatAbierto.jid == jid) ChatAbierto.jid = ""
        super.onCleared()
    }

    fun recargar() {
        if (antiguo) recargarAntiguo() else { trabajo?.cancel(); trabajo = viewModelScope.launch { refrescar() } }
    }

    fun cargarMas() {
        val actuales = mensajes ?: return
        if (cargandoMas || !hayMas || actuales.isEmpty()) return
        cargandoMas = true
        viewModelScope.launch {
            try {
                val u = actuales.last()
                val mas = Repo.mensajes(botId, jid, u.ts, u.id, pagina)
                actualizar { MezclaMensajes.agregar(it ?: emptyList(), mas) }
                hayMas = mas.size >= pagina
            } catch (e: Exception) {
                aviso = e.message
            } finally {
                cargandoMas = false
            }
        }
    }

    /** Carga páginas hasta que el mensaje `id` esté en la lista. Devuelve false si no existe (o es muy lejano). */
    suspend fun cargarHasta(id: String): Boolean {
        var vueltas = 0
        while (mensajes?.any { it.id == id } != true) {
            val actuales = mensajes ?: return false
            if (!hayMas || actuales.isEmpty() || vueltas++ > 200) return false
            val u = actuales.last()
            val mas = runCatching { Repo.mensajes(botId, jid, u.ts, u.id, pagina) }.getOrNull() ?: return false
            actualizar { MezclaMensajes.agregar(it ?: emptyList(), mas) }
            hayMas = mas.size >= pagina
        }
        return true
    }

    fun olvidar(m: Msg) {
        viewModelScope.launch { runCatching { Repo.olvidar(botId, jid, m.id) }.onFailure { aviso = it.message ?: "No se pudo olvidar el mensaje" } }
    }

    fun descargar(m: Msg) {
        viewModelScope.launch {
            try {
                Repo.descargar(botId, jid, m.id)
            } catch (e: Exception) {
                aviso = e.message ?: "No se pudo descargar"
            }
        }
    }

    fun estimarArchivos(alTerminar: (com.botchat.app.datos.Liberado) -> Unit) {
        viewModelScope.launch { runCatching { Repo.estimarBorrado(botId, chat = jid) }.onSuccess(alTerminar).onFailure { aviso = it.message } }
    }

    fun borrarArchivos(alTerminar: (com.botchat.app.datos.Liberado) -> Unit) {
        viewModelScope.launch { runCatching { Repo.borrarArchivos(botId, chat = jid) }.onSuccess(alTerminar).onFailure { aviso = it.message } }
    }

    fun borrarChat(alTerminar: () -> Unit) {
        viewModelScope.launch { runCatching { Repo.borrarChat(botId, jid) }.onSuccess { alTerminar() }.onFailure { aviso = it.message } }
    }

    // ---------- escribir ----------

    /** Manda un texto escrito a mano (o guarda la edición si se está editando). `menciones` son JIDs de grupo. */
    fun enviar(texto: String, menciones: List<String>, alTerminar: () -> Unit = {}) {
        if (enviando) return
        val t = texto.trim()
        if (t.isEmpty()) return
        enviando = true
        val edicion = editando
        val cita = citando
        viewModelScope.launch {
            try {
                if (edicion != null) Repo.editar(botId, jid, edicion.id, t)
                else Repo.enviarTexto(botId, jid, t, cita?.id ?: "", menciones)
                editando = null; citando = null
                guardarBorrador("")
                alTerminar()
            } catch (e: Exception) {
                aviso = e.message ?: "No se pudo enviar"
                if (edicion == null) { citando = null; alTerminar() } // el mensaje quedó guardado como fallido, con "Reintentar"
            } finally {
                enviando = false
                escribiendoSi(false)
            }
        }
    }

    fun reintentar(m: Msg) {
        viewModelScope.launch { runCatching { Repo.reintentarEnvio(botId, jid, m.id) }.onFailure { aviso = it.message ?: "No se pudo enviar" } }
    }

    fun reaccionar(m: Msg, emoji: String) {
        val quitar = m.reacciones.any { it.mia && it.emoji == emoji }
        viewModelScope.launch { runCatching { Repo.reaccionar(botId, jid, m.id, if (quitar) "" else emoji) }.onFailure { aviso = it.message ?: "No se pudo reaccionar" } }
    }

    fun borrarParaMi(ids: List<String>) {
        viewModelScope.launch { runCatching { Repo.borrarParaMi(botId, jid, ids) }.onFailure { aviso = it.message } }
    }

    fun borrarParaTodos(ms: List<Msg>) {
        viewModelScope.launch {
            for (m in ms) runCatching { Repo.borrarParaTodos(botId, jid, m.id) }.onFailure { aviso = "WhatsApp no permitió borrarlo para todos: ${it.message}" }
        }
    }

    fun reenviar(ids: List<String>, destinos: List<String>, alTerminar: (String) -> Unit) {
        viewModelScope.launch {
            runCatching { Repo.reenviar(botId, jid, ids, destinos) }
                .onSuccess { r ->
                    alTerminar(buildString {
                        append(if (r.enviados == 1) "Se reenvió 1 mensaje" else "Se reenviaron ${r.enviados} mensajes")
                        if (r.omitidos > 0) append(". Los que no son de texto todavía no se pueden reenviar")
                        if (r.fallidos > 0) append(". Fallaron ${r.fallidos}")
                    })
                }
                .onFailure { aviso = it.message ?: "No se pudo reenviar" }
        }
    }

    suspend fun info(m: Msg): InfoMsg? = runCatching { Repo.infoMensaje(botId, jid, m.id) }.getOrNull()

    /** Interruptor Activo / Pausado del bot en este chat (grupos). */
    fun botActivoEnChat(si: Boolean) {
        viewModelScope.launch { runCatching { Repo.botEnChat(botId, jid, si); estadoChat = estadoChat.copy(botActivo = si) }.onFailure { aviso = it.message } }
    }

    /** Quita la pausa del bot en este chat (la de escribir yo o la de "hablar con una persona"). */
    fun reactivarBot() {
        viewModelScope.launch { runCatching { Repo.pausarBotEnChat(botId, jid, 0); estadoChat = estadoChat.copy(botPausaHasta = 0, pausaHumano = false) }.onFailure { aviso = it.message } }
    }

    fun silenciar(si: Boolean) {
        viewModelScope.launch { runCatching { Repo.silenciar(botId, jid, si); estadoChat = estadoChat.copy(silenciado = si) } }
    }

    // ---------- adjuntos ----------

    /** Prepara (copia, reduce y saca miniatura) lo elegido en galería o archivos. */
    fun prepararUris(uris: List<android.net.Uri>) {
        if (uris.isEmpty()) return
        val gen = generacionAdjuntos
        viewModelScope.launch {
            preparandoAdjuntos = true
            for (u in uris.take(10)) {
                PrepararAdjunto.desdeUri(getApplication(), u)
                    .onSuccess { if (gen == generacionAdjuntos) adjuntos = adjuntos + it else it.archivo.delete() } // si se canceló mientras se preparaba, se descarta
                    .onFailure { aviso = "No se pudo preparar el archivo: ${it.message}" }
            }
            if (gen == generacionAdjuntos) preparandoAdjuntos = false
        }
    }

    /** Prepara lo que acaba de tomar la cámara. */
    fun prepararArchivo(archivo: java.io.File, mime: String) {
        val gen = generacionAdjuntos
        viewModelScope.launch {
            preparandoAdjuntos = true
            PrepararAdjunto.desdeArchivo(archivo, mime)
                .onSuccess { if (gen == generacionAdjuntos) adjuntos = adjuntos + it else it.archivo.delete() }
                .onFailure { aviso = "No se pudo preparar el archivo: ${it.message}" }
            if (gen == generacionAdjuntos) preparandoAdjuntos = false
        }
    }

    fun quitarAdjunto(a: AdjuntoListo) { a.archivo.delete(); adjuntos = adjuntos - a }

    fun cancelarAdjuntos() { generacionAdjuntos++; preparandoAdjuntos = false; adjuntos.forEach { it.archivo.delete() }; adjuntos = emptyList() }

    /** Manda los adjuntos de uno en uno; `pie` va en el primero. */
    fun enviarAdjuntos(pie: String, alTerminar: () -> Unit = {}) {
        if (enviandoAdjuntos || adjuntos.isEmpty()) return
        enviandoAdjuntos = true
        val cita = citando
        viewModelScope.launch {
            try {
                var primero = true
                for (a in adjuntos.toList()) {
                    Repo.enviarMedio(botId, jid, a.tipo, a.archivo.absolutePath, a.mime, a.nombre, if (primero) pie.trim() else "", if (primero) cita?.id ?: "" else "", a.ancho, a.alto, a.durS, false, a.miniatura, null)
                    adjuntos = adjuntos - a
                    primero = false
                }
                citando = null
                alTerminar()
            } catch (e: Exception) {
                aviso = e.message ?: "No se pudo enviar"
                adjuntos = adjuntos.filter { it.archivo.exists() } // el que falló quedó guardado como fallido, con Reintentar
                alTerminar()
            } finally {
                enviandoAdjuntos = false
            }
        }
    }

    // ---------- notas de voz ----------

    /** Empieza a grabar. Devuelve false si no se pudo usar el micrófono. */
    fun iniciarGrabacion(): Boolean {
        if (grabando) return false
        val g = Grabadores.fabrica(getApplication())
        if (!g.iniciar()) { aviso = "No se pudo usar el micrófono"; return false }
        grabador = g; grabando = true; segundosGrabando = 0
        cronometro = viewModelScope.launch { while (true) { delay(1000); segundosGrabando++ } }
        return true
    }

    /** Termina la grabación: se manda, o se descarta si `cancelar`. */
    fun terminarGrabacion(cancelar: Boolean) {
        val g = grabador ?: return
        cronometro?.cancel(); grabando = false; grabador = null
        if (cancelar) { g.cancelar(); return }
        val r = g.terminar()
        if (r == null) { aviso = "La nota de voz fue muy corta"; return }
        val cita = citando
        viewModelScope.launch {
            try {
                Repo.enviarMedio(botId, jid, "audio", r.archivo.absolutePath, r.mime, "", "", cita?.id ?: "", 0, 0, r.durS, r.esNota, null, r.onda)
                citando = null
            } catch (e: Exception) { aviso = e.message ?: "No se pudo enviar la nota de voz" }
        }
    }

    // ---------- borrador y "escribiendo…" ----------

    fun guardarBorrador(texto: String) {
        borradorJob?.cancel()
        borradorJob = viewModelScope.launch { delay(400); runCatching { Repo.guardarBorrador(botId, jid, texto) } }
    }

    /** Se llama en cada cambio del texto: manda "escribiendo…" (máximo uno cada 4 s) y lo quita a los 3 s sin teclear. */
    fun teclea(hayTexto: Boolean) {
        if (!hayTexto) { escribiendoSi(false); return }
        tecleo?.cancel()
        tecleo = viewModelScope.launch {
            if (!enviadoEscribiendo || System.currentTimeMillis() - ultimoEscribiendo > 4000) {
                enviadoEscribiendo = true; ultimoEscribiendo = System.currentTimeMillis()
                runCatching { Repo.escribiendo(botId, jid, true) }
            }
            delay(3000)
            escribiendoSi(false)
        }
    }

    private var enviadoEscribiendo = false
    private var ultimoEscribiendo = 0L

    private fun escribiendoSi(si: Boolean) {
        if (si) return
        tecleo?.cancel()
        if (enviadoEscribiendo) { enviadoEscribiendo = false; viewModelScope.launch { runCatching { Repo.escribiendo(botId, jid, false) } } }
    }

    // ---------- presencia, leído y participantes ----------

    private fun actualizarPresencia() {
        viewModelScope.launch { presencia = runCatching { Repo.presencia(jid) }.getOrDefault(PresenciaChat()) }
    }

    /** Con el chat a la vista: se pide la presencia del contacto y, si el usuario lo permitió, se marca "en línea". */
    fun alMostrar(mostrarEnLinea: Boolean) {
        ChatAbierto.jid = jid
        com.botchat.app.Notificador.quitar(getApplication(), jid)
        viewModelScope.launch {
            entrada.await() // presencia y "en línea" esperan a que termine la animación de entrada
            if (mostrarEnLinea) {
                runCatching { Repo.enLinea(botId, true) }
                runCatching { Repo.suscribirPresencia(botId, jid) }
            }
            actualizarPresencia()
        }
        programarLeido()
    }

    fun alOcultar(mostrarEnLinea: Boolean) {
        if (ChatAbierto.jid == jid) ChatAbierto.jid = ""
        escribiendoSi(false)
        if (mostrarEnLinea) viewModelScope.launch { runCatching { Repo.enLinea(botId, false) } }
    }

    private fun programarLeido() {
        if (!entrada.isCompleted || !ajustes.marcarLeidoAlAbrir.value || ChatAbierto.jid != jid) return
        leidoJob?.cancel()
        leidoJob = viewModelScope.launch { delay(600); runCatching { Repo.marcarLeido(botId, jid) } }
    }

    private fun cargarParticipantes() {
        viewModelScope.launch { participantes = runCatching { Repo.participantes(botId, jid) }.getOrDefault(emptyList()) }
    }
}
