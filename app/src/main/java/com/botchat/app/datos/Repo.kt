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
import org.json.JSONObject
import wabot.Observador
import java.io.File

/** Acceso a los mensajes guardados (módulo Go). Todo corre fuera del hilo principal. */
object Repo {
    /** El motor de datos (el módulo Go). Las pruebas lo cambian por uno de ejemplo. */
    @Volatile var motor: Motor = MotorGo

    private lateinit var raiz: File
    private val cambiosInternos = MutableSharedFlow<List<String>>(extraBufferCapacity = 64)

    /**
     * Eventos en vivo: "chats", "chat:<jid>", "msg:<jid>:<id>" (cambió ese mensaje), "archivo:<jid>:<id>" y "espacio".
     * Rangos y permisos: "rangos", "rango:<persona>", "solicitudes", "confirmaciones", "politicas", "acciones" y "permisos_ajustes".
     * Módulos: "modulos" (activar, configurar, modo seguro, ajustes) e "iniciativas" (registro, No molestar).
     */
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
            runCatching { com.botchat.app.seguridad.LlavesModulos.cargarEnMotor(ctx) { b, m, c, v -> motor.fijarLlaveModulo(b, m, c, v) } }
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
    /** Ids de etiqueta de cada persona (de un solo golpe, para pintar los puntitos de la lista). */
    suspend fun etiquetasDePersonas(bot: Long, personas: List<String>): Map<String, List<Long>> = io {
        val o = org.json.JSONObject(motor.etiquetasDePersonas(bot, JSONArray(personas).toString()).ifBlank { "{}" })
        o.keys().asSequence().associateWith { k -> o.optJSONArray(k).let { a -> List(a?.length() ?: 0) { i -> a!!.getLong(i) } } }
    }
    suspend fun fijarChat(bot: Long, chat: String, si: Boolean) = io { motor.fijarChatLocal(bot, chat, si) }
    suspend fun vaciarChat(bot: Long, chat: String) = io { Json.liberado(motor.vaciarChat(bot, chat)) }
    suspend fun buscarEnChat(bot: Long, chat: String, palabra: String, antesTs: Long = 0, antesId: String = "", limite: Int = 50) =
        io { Json.busqueda(motor.buscarEnChat(bot, chat, palabra, 0, 0, antesTs, antesId, limite.toLong())) }
    suspend fun exportarChat(bot: Long, chat: String, formato: String) = io { Json.exporte(motor.exportarChat(bot, chat, formato, 0, 0)) }
    suspend fun gruposAdmin(bot: Long) = io { Json.gruposAdmin(motor.gruposQueAdministro(bot)) }
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

    // Menús ampliados (bloque C)
    suspend fun ajustesDeMenus(bot: Long) = io { JsonMenus.ajustes(motor.ajustesDeMenus(bot)) }
    suspend fun fijarAjusteDeMenus(bot: Long, clave: String, valor: String) = io { motor.fijarAjusteDeMenus(bot, clave, valor) }
    suspend fun formularios(bot: Long, menuId: Long = 0, limite: Long = 200) = io { JsonMenus.formularios(motor.listarFormularios(bot, menuId, limite)) }
    suspend fun formulario(bot: Long, id: Long) = io { JsonMenus.formulario(motor.obtenerFormulario(bot, id)) }
    suspend fun contarFormularios(bot: Long, menuId: Long = 0) = io { motor.contarFormularios(bot, menuId) }
    suspend fun borrarFormulario(bot: Long, id: Long) = io { motor.borrarFormulario(bot, id) }
    suspend fun borrarFormularios(bot: Long, menuId: Long = 0) = io { motor.borrarFormularios(bot, menuId) }
    suspend fun exportarFormularios(bot: Long, menuId: Long = 0) = io { motor.exportarFormularios(bot, menuId) }
    suspend fun noEntendio(bot: Long, menuId: Long = 0, limite: Long = 200) = io { JsonMenus.noEntendio(motor.listarNoEntendio(bot, menuId, limite)) }
    suspend fun borrarNoEntendio(bot: Long, menuId: Long = 0) = io { motor.borrarNoEntendio(bot, menuId) }
    suspend fun subirAdjuntoMenu(bot: Long, ruta: String, tipo: String, mime: String, nombre: String) = io { JsonMenus.adjunto(motor.subirAdjuntoMenu(bot, ruta, tipo, mime, nombre)) }
    suspend fun borrarAdjuntoMenu(bot: Long, id: String) = io { motor.borrarAdjuntoMenu(bot, id) }
    suspend fun exportarMenus(bot: Long, ids: List<Long>, clave: String) = io { motor.exportarMenus(bot, org.json.JSONArray(ids).toString(), clave) }
    suspend fun importarMenus(bot: Long, ruta: String, clave: String) = io { JsonMenus.importado(motor.importarMenus(bot, ruta, clave)) }

    // Rangos y permisos
    suspend fun catalogo() = io { JsonPerm.casillas(motor.catalogoPermisos()) }
    suspend fun rangos(bot: Long) = io { JsonPerm.rangos(motor.listarRangos(bot)) }
    suspend fun rango(bot: Long, id: Long) = io { JsonPerm.rangoOpt(motor.obtenerRango(bot, id)) }
    suspend fun guardarRango(bot: Long, r: Rango) = io { motor.guardarRango(bot, r.json()) }
    suspend fun borrarRango(bot: Long, id: Long) = io { motor.borrarRango(bot, id).toInt() }
    suspend fun reordenarRangos(bot: Long, ids: List<Long>) = io { motor.reordenarRangos(bot, JSONArray(ids).toString()) }
    /** duracion: "24h", "7d" o "perm". */
    suspend fun asignarRango(bot: Long, persona: String, rolId: Long, duracion: String) = io { motor.asignarRango(bot, persona, rolId, duracion) }
    suspend fun revocarRango(bot: Long, persona: String) = io { motor.revocarRango(bot, persona) }
    suspend fun rangoDe(bot: Long, persona: String) = io { JsonPerm.rangoDePersona(motor.rangoDePersona(bot, persona)) }
    suspend fun asignaciones(bot: Long) = io { JsonPerm.asignaciones(motor.listarAsignaciones(bot)) }
    suspend fun solicitudes(bot: Long, estado: String = "pendiente") = io { JsonPerm.solicitudes(motor.listarSolicitudes(bot, estado)) }
    suspend fun contarSolicitudes(bot: Long) = io { motor.contarSolicitudes(bot).toInt() }
    suspend fun aprobarSolicitud(bot: Long, persona: String) = io { motor.aprobarSolicitud(bot, persona) }
    suspend fun bloquearSolicitud(bot: Long, persona: String) = io { motor.bloquearSolicitud(bot, persona) }
    suspend fun ignorarSolicitud(bot: Long, persona: String) = io { motor.ignorarSolicitud(bot, persona) }
    suspend fun quitarSolicitud(bot: Long, persona: String) = io { motor.quitarSolicitud(bot, persona) }
    suspend fun politicaGrupo(bot: Long, chat: String) = io { JsonPerm.politica(motor.politicaDeGrupo(bot, chat)) }
    suspend fun fijarPoliticaGrupo(bot: Long, p: PoliticaGrupo) =
        io { motor.fijarPoliticaDeGrupo(bot, org.json.JSONObject().put("chat", p.chat).put("modo", p.modo).put("lista", JSONArray(p.lista)).toString()) }
    suspend fun politicasGrupo(bot: Long) = io { JsonPerm.politicas(motor.listarPoliticasDeGrupo(bot)) }
    suspend fun ajustesPermisos(bot: Long) = io { JsonPerm.ajustes(motor.ajustesDePermisos(bot)) }
    suspend fun fijarAjustePermisos(bot: Long, clave: String, valor: String) = io { motor.fijarAjusteDePermisos(bot, clave, valor) }
    suspend fun confirmaciones(bot: Long) = io { JsonPerm.confirmaciones(motor.confirmacionesPendientes(bot)) }
    suspend fun confirmarCodigo(bot: Long, codigo: String) = io { motor.confirmarCodigo(bot, codigo) }
    suspend fun cancelarConfirmacion(bot: Long, codigo: String) = io { motor.cancelarConfirmacion(bot, codigo) }
    suspend fun ejecutarComando(bot: Long, texto: String) = io { JsonPerm.comando(motor.ejecutarComando(bot, texto)) }
    suspend fun acciones(bot: Long, limite: Int = 100, antesId: Long = 0) = io { JsonPerm.acciones(motor.listarAcciones(bot, limite.toLong(), antesId)) }

    // Módulos e iniciativa propia
    // Descargas de video (módulo opcional)
    suspend fun iaProveedores() = io { JsonIA.proveedores(motor.iaProveedores()) }
    suspend fun iaProbar(bot: Long) = io { motor.iaProbar(bot) }
    suspend fun videoEstado(bot: Long) = io { JsonVideo.estado(motor.videoEstado(bot)) }
    suspend fun videoLista(bot: Long, antes: Long = 0) = io { JsonVideo.lista(motor.videoLista(bot, 40, antes)) }
    suspend fun videoMiniatura(bot: Long, id: Long) = io { motor.videoMiniatura(bot, id) }
    suspend fun videoBorrar(bot: Long, id: Long) = io { motor.videoBorrar(bot, id) }
    suspend fun videoActualizar(bot: Long) = io { motor.videoActualizar(bot) }
    suspend fun videoQuitarMotor(bot: Long) = io { motor.videoQuitarMotor(bot) }
    suspend fun videoRepetir(bot: Long, id: Long) = io { motor.videoRepetir(bot, id) }
    suspend fun modulos(bot: Long) = io { JsonMod.modulos(motor.listarModulos(bot)) }
    suspend fun modulo(bot: Long, id: String) = io { JsonMod.moduloOpt(motor.infoModulo(bot, id)) }
    suspend fun activarModulo(bot: Long, id: String) = io { motor.activarModulo(bot, id) }
    suspend fun desactivarModulo(bot: Long, id: String) = io { motor.desactivarModulo(bot, id) }
    suspend fun configModulo(bot: Long, id: String) = io { JsonMod.config(motor.configModulo(bot, id)) }
    suspend fun guardarConfigModulo(bot: Long, id: String, config: Map<String, Any?>) = io { motor.guardarConfigModulo(bot, id, org.json.JSONObject(config).toString()) }
    /** 0 = usar el tope que trae el módulo. */
    suspend fun fijarTopesModulo(bot: Long, id: String, hora: Int, dia: Int) = io { motor.fijarTopesModulo(bot, id, hora.toLong(), dia.toLong()) }
    suspend fun ajustesIniciativa(bot: Long) = io { JsonMod.ajustes(motor.ajustesIniciativa(bot)) }
    suspend fun fijarAjusteIniciativa(bot: Long, clave: String, valor: String) = io { motor.fijarAjusteIniciativa(bot, clave, valor) }
    suspend fun fijarIniciativaMaestro(bot: Long, activo: Boolean) = io { motor.fijarIniciativaMaestro(bot, activo) }
    suspend fun noMolestar(bot: Long) = io { JsonMod.noMolestar(motor.listarNoMolestar(bot)) }
    suspend fun agregarNoMolestar(bot: Long, persona: String, motivo: String = "a mano") = io { motor.agregarNoMolestar(bot, persona, motivo) }
    suspend fun quitarNoMolestar(bot: Long, persona: String) = io { motor.quitarNoMolestar(bot, persona) }
    suspend fun filasPanelModulo(bot: Long, id: String, panel: String) = io { JsonMod.filasPanel(motor.filasPanelModulo(bot, id, panel)) }
    suspend fun accionModulo(bot: Long, id: String, accion: String, args: Map<String, Any?>) = io { motor.accionModulo(bot, id, accion, org.json.JSONObject(args).toString()) }
    suspend fun proximoModulos(bot: Long) = io { motor.proximoModulos(bot) }
    suspend fun tickModulos(bot: Long) = io { motor.tickModulos(bot) }
    /** Guarda una llave de un módulo: cifrada en el Keystore y en memoria de Go (valor vacío = borrarla). Nunca va a la base. */
    suspend fun guardarLlaveModulo(ctx: Context, bot: Long, id: String, clave: String, valor: String) = io {
        com.botchat.app.seguridad.LlavesModulos.guardar(ctx, bot, id, clave, valor)
        motor.fijarLlaveModulo(bot, id, clave, valor.trim())
    }
    suspend fun iniciativas(bot: Long, modulo: String = "", limite: Int = 100, antesId: Long = 0) = io { JsonMod.registro(motor.listarIniciativas(bot, modulo, limite.toLong(), antesId)) }
    suspend fun modoSeguro(bot: Long) = io { JsonMod.seguro(motor.estadoModoSeguro(bot)) }
    suspend fun reanudarModoSeguro(bot: Long) = io { motor.reanudarModoSeguro(bot) }

    // Personas, chat y avisos para mí (bloque B)
    suspend fun ficha(bot: Long, persona: String) = io { JsonPersonas.ficha(motor.fichaPersona(bot, persona)) }
    suspend fun estadisticasPersona(bot: Long, persona: String, dias: Int = 30) = io { JsonPersonas.estadisticas(motor.estadisticasPersona(bot, persona, dias.toLong())) }
    /** minutos 0 = para siempre. */
    suspend fun silenciarPersona(bot: Long, persona: String, minutos: Int, motivo: String = "") = io { motor.silenciarPersona(bot, persona, minutos.toLong(), motivo) }
    suspend fun quitarSilencioPersona(bot: Long, persona: String) = io { motor.quitarSilencioPersona(bot, persona) }
    suspend fun silencios(bot: Long) = io { JsonPersonas.silencios(motor.listarSilencios(bot)) }
    suspend fun bloquearPersona(bot: Long, persona: String) = io { motor.bloquearPersona(bot, persona) }
    suspend fun desbloquearPersona(bot: Long, persona: String) = io { motor.desbloquearPersona(bot, persona) }
    /** chat "" = su chat privado. */
    suspend fun advertir(bot: Long, persona: String, motivo: String, chat: String = "") = io { JsonPersonas.resultadoAdvertencia(motor.advertirPersona(bot, persona, chat, motivo)) }
    suspend fun advertencias(bot: Long, persona: String) = io { JsonPersonas.advertencias(motor.listarAdvertencias(bot, persona)) }
    suspend fun personasAdvertidas(bot: Long) = io { JsonPersonas.advertidos(motor.listarPersonasAdvertidas(bot)) }
    /** id 0 = la última. Devuelve cuántas le quedan. */
    suspend fun quitarAdvertencia(bot: Long, persona: String, id: Long = 0) = io { motor.quitarAdvertenciaPersona(bot, persona, id).toInt() }
    suspend fun borrarAdvertencias(bot: Long, persona: String) = io { motor.borrarAdvertenciasPersona(bot, persona).toInt() }
    suspend fun notasPersona(bot: Long, persona: String) = io { JsonPersonas.notas(motor.listarNotasPersona(bot, persona)) }
    suspend fun agregarNota(bot: Long, persona: String, texto: String) = io { motor.agregarNotaPersona(bot, persona, texto) }
    suspend fun borrarNota(bot: Long, persona: String, id: Long) = io { motor.borrarNotaPersona(bot, persona, id) }
    suspend fun olvidarPersona(bot: Long, persona: String) = io { JsonPersonas.olvidar(motor.olvidarPersona(bot, persona)) }
    /** Guarda los datos de la persona en un archivo cifrado BCF1 y devuelve su ruta relativa (ver [archivo]). */
    suspend fun exportarPersona(bot: Long, persona: String) = io { motor.exportarPersona(bot, persona) }

    suspend fun marcarEstadoChat(bot: Long, chat: String, estado: String) = io { motor.marcarEstadoChat(bot, chat, estado) }
    suspend fun chatsFiltrados(bot: Long, grupos: Boolean, consulta: String, estado: String, etiqueta: Long, limite: Int = 300, desde: Int = 0) =
        io { Json.chats(motor.listarChatsFiltrados(bot, grupos, consulta, estado, etiqueta, limite.toLong(), desde.toLong())) }
    suspend fun contarPendientes(bot: Long) = io { motor.contarPendientes(bot).toInt() }
    suspend fun etiquetas(bot: Long) = io { JsonPersonas.etiquetas(motor.listarEtiquetas(bot)) }
    suspend fun crearEtiqueta(bot: Long, nombre: String, color: String) = io { motor.crearEtiqueta(bot, nombre, color) }
    suspend fun editarEtiqueta(bot: Long, id: Long, nombre: String, color: String) = io { motor.editarEtiqueta(bot, id, nombre, color) }
    suspend fun borrarEtiqueta(bot: Long, id: Long) = io { motor.borrarEtiqueta(bot, id) }
    suspend fun ponerEtiquetaChat(bot: Long, chat: String, id: Long, si: Boolean) = io { motor.ponerEtiquetaChat(bot, chat, id, si) }
    suspend fun etiquetasDeChat(bot: Long, chat: String) = io { JsonPersonas.etiquetas(motor.etiquetasDeChat(bot, chat)) }

    suspend fun palabrasClave(bot: Long) = io { JsonPersonas.palabras(motor.listarAlertasPalabra(bot)) }
    suspend fun crearPalabraClave(bot: Long, palabra: String, chats: List<String> = emptyList()) = io { motor.crearAlertaPalabra(bot, palabra, JSONArray(chats).toString()) }
    suspend fun editarPalabraClave(bot: Long, id: Long, chats: List<String>, activa: Boolean) = io { motor.editarAlertaPalabra(bot, id, JSONArray(chats).toString(), activa) }
    suspend fun borrarPalabraClave(bot: Long, id: Long) = io { motor.borrarAlertaPalabra(bot, id) }
    suspend fun recordatorios(bot: Long, hechos: Boolean = false) = io { JsonPersonas.recordatorios(motor.listarRecordatorios(bot, hechos)) }
    /** repetir: "", "diaria" o "semanal". proximaTs en segundos. */
    suspend fun crearRecordatorio(bot: Long, texto: String, proximaTs: Long, repetir: String = "") = io { motor.crearRecordatorio(bot, texto, proximaTs, repetir) }
    suspend fun editarRecordatorio(bot: Long, id: Long, texto: String, proximaTs: Long, repetir: String = "") = io { motor.editarRecordatorio(bot, id, texto, proximaTs, repetir) }
    suspend fun borrarRecordatorio(bot: Long, id: Long) = io { motor.borrarRecordatorio(bot, id) }
    // Notas y pendientes propios (bloque F)
    suspend fun notasUtil(bot: Long) = io { JsonPersonas.utilidades(motor.listarUtilidades(bot, "nota")) }
    suspend fun pendientesUtil(bot: Long) = io { JsonPersonas.utilidades(motor.listarUtilidades(bot, "pendiente")) }
    suspend fun agregarNotaUtil(bot: Long, texto: String) = io { motor.agregarUtilidad(bot, "nota", texto) }
    suspend fun agregarPendienteUtil(bot: Long, texto: String) = io { motor.agregarUtilidad(bot, "pendiente", texto) }
    suspend fun marcarUtil(bot: Long, id: Long, hecho: Boolean) = io { motor.marcarUtilidad(bot, id, hecho) }
    suspend fun borrarUtil(bot: Long, id: Long) = io { motor.borrarUtilidad(bot, id) }
    suspend fun limpiarUtilHechas(bot: Long) = io { motor.limpiarUtilidadesHechas(bot) }
    // Laboratorio (bloque G). Evento: "lab".
    suspend fun estadoLab(bot: Long) = io { JsonLab.estado(motor.estadoLab(bot)) }
    suspend fun aceptarAvisoLab(bot: Long) = io { motor.aceptarAvisoLab(bot) }
    suspend fun fijarAjusteLab(bot: Long, clave: String, valor: String) = io { motor.fijarAjusteLab(bot, clave, valor) }
    suspend fun estadoCaptura(bot: Long) = io { JsonLab.captura(motor.estadoCaptura(bot)) }
    suspend fun activarCaptura(bot: Long, activo: Boolean) = io { motor.activarCaptura(bot, activo) }
    suspend fun mostrarTextosCaptura(bot: Long, mostrar: Boolean) = io { motor.mostrarTextosCaptura(bot, mostrar) }
    suspend fun borrarCaptura(bot: Long) = io { motor.borrarCaptura(bot) }
    suspend fun proximoAviso(bot: Long) = io { motor.proximoAviso(bot) }
    suspend fun dispararAvisos(bot: Long) = io { JsonPersonas.disparo(motor.dispararAvisos(bot)) }
    suspend fun vistaPreviaResumen(bot: Long) = io { motor.vistaPreviaResumen(bot) }
    suspend fun comandosPropios(bot: Long) = io { JsonPersonas.comandosPropios(motor.listarComandosPropios(bot)) }
    suspend fun guardarComandoPropio(bot: Long, id: Long, c: ComandoPropio) = io { motor.guardarComandoPropio(bot, id, c.json()) }
    suspend fun vistaPreviaComandoPropio(bot: Long, c: ComandoPropio) = io { motor.vistaPreviaComandoPropio(bot, c.json()) }

    // Mensajes automáticos y adjuntos (bloque D)
    suspend fun listaAuto(bot: Long, tipo: String = "") = io { JsonMensajesAuto.lista(motor.listarAutos(bot, tipo)) }
    suspend fun obtenerAuto(bot: Long, id: Long) = io { JsonMensajesAuto.mensaje(motor.obtenerAuto(bot, id)) }
    suspend fun guardarAuto(bot: Long, m: MensajeAuto) = io { motor.guardarAuto(bot, JsonMensajesAuto.entradaJson(m)) }
    suspend fun fijarAutoActivo(bot: Long, id: Long, activo: Boolean) = io { motor.fijarAutoActivo(bot, id, activo) }
    suspend fun borrarAuto(bot: Long, id: Long) = io { motor.borrarAuto(bot, id) }
    suspend fun reordenarAutos(bot: Long, ids: List<Long>) = io { motor.reordenarAutos(bot, org.json.JSONArray(ids).toString()) }
    suspend fun plantillasMensajes() = io { JsonMensajesAuto.plantillas(motor.plantillasAuto()) }
    suspend fun configPorDefectoAuto(tipo: String) = io { JsonMensajesAuto.config(org.json.JSONObject(motor.configPorDefectoAuto(tipo))) }
    suspend fun probarAuto(bot: Long, m: MensajeAuto, e: EscenarioAuto) = io { JsonMensajesAuto.prueba(motor.probarAuto(bot, JsonMensajesAuto.entradaJson(m), e.json())) }
    suspend fun registroAuto(bot: Long, autoId: Long = 0, limite: Long = 100) = io { JsonMensajesAuto.registro(motor.registroAuto(bot, autoId, limite)) }
    suspend fun subirAdjunto(bot: Long, ruta: String, tipo: String, mime: String, nombre: String) = io { JsonMenus.adjunto(motor.subirAdjunto(bot, ruta, tipo, mime, nombre)) }
    suspend fun borrarAdjunto(bot: Long, id: String) = io { motor.borrarAdjunto(bot, id) }
    suspend fun limpiarAdjuntos(bot: Long) = io { motor.limpiarAdjuntos(bot) }
    suspend fun atajos(bot: Long, chat: String) = io { JsonMensajesAuto.atajos(motor.listarAtajos(bot, chat)) }
    suspend fun enviarArchivosAtajo(bot: Long, chat: String, id: Long) = io { motor.enviarArchivosAtajo(bot, chat, id) }
    suspend fun mensajesAuto(bot: Long, chat: String, antesTs: Long = 0, limite: Long = 100) = io { Json.mensajes(motor.listarMensajesAuto(bot, chat, antesTs, limite)) }
    suspend fun chatsConAutomaticos(bot: Long, dias: Long = 7) = io { JsonMensajesAuto.jids(motor.chatsConAutomaticos(bot, dias)) }

    // Grupos (bloque E). Las acciones que escriben a WhatsApp fallan con un mensaje claro en español (sin conexión, no soy administrador…).
    suspend fun estadoGrupo(bot: Long, chat: String, refrescar: Boolean = false) = io { JsonGrupos.estado(motor.grupoEstado(bot, chat, refrescar)) }
    suspend fun infoGrupo(bot: Long, chat: String) = io { JsonGrupos.info(motor.grupoInfo(bot, chat)) }
    suspend fun miembrosGrupo(bot: Long, chat: String) = io { JsonGrupos.miembros(motor.grupoMiembros(bot, chat)) }
    suspend fun cerrarGrupo(bot: Long, chat: String, cerrado: Boolean) = io { motor.grupoCerrar(bot, chat, cerrado) }
    suspend fun bloquearInfoGrupo(bot: Long, chat: String, bloqueada: Boolean) = io { motor.grupoBloquearInfo(bot, chat, bloqueada) }
    suspend fun expulsarDeGrupo(bot: Long, chat: String, persona: String) = io { motor.grupoExpulsar(bot, chat, persona) }
    suspend fun ascenderEnGrupo(bot: Long, chat: String, persona: String, admin: Boolean) = io { motor.grupoAscender(bot, chat, persona, admin) }
    suspend fun solicitudesGrupo(bot: Long, chat: String) = io { JsonGrupos.solicitudes(motor.grupoSolicitudes(bot, chat)) }
    /** personas vacío = todas las pendientes. Devuelve cuántas se resolvieron. */
    suspend fun resolverSolicitudesGrupo(bot: Long, chat: String, personas: List<String>, aprobar: Boolean) =
        io { motor.grupoResolverSolicitudes(bot, chat, if (personas.isEmpty()) "" else JSONArray(personas).toString(), aprobar).toInt() }
    suspend fun renombrarGrupo(bot: Long, chat: String, nombre: String) = io { motor.grupoRenombrar(bot, chat, nombre) }
    suspend fun descripcionGrupo(bot: Long, chat: String, texto: String) = io { motor.grupoDescripcion(bot, chat, texto) }
    suspend fun enlaceGrupo(bot: Long, chat: String, reiniciar: Boolean = false) = io { motor.grupoEnlace(bot, chat, reiniciar) }
    suspend fun borrarMensajeAjeno(bot: Long, chat: String, msgId: String) = io { motor.grupoBorrarMensaje(bot, chat, msgId) }
    suspend fun mencionarATodos(bot: Long, chat: String, texto: String) = io { JsonGrupos.todos(motor.grupoMencionarTodos(bot, chat, texto)) }
    /** dias: 7, 30 o 90 (0 = el de Ajustes). */
    suspend fun rankingGrupo(bot: Long, chat: String, dias: Int = 30, limite: Int = 20) = io { JsonGrupos.ranking(motor.grupoRanking(bot, chat, dias.toLong(), limite.toLong())) }
    suspend fun inactivosGrupo(bot: Long, chat: String, dias: Int = 30) = io { JsonGrupos.inactivos(motor.grupoInactivos(bot, chat, dias.toLong())) }
    suspend fun reglasGrupo(bot: Long, chat: String) = io { motor.grupoReglas(bot, chat) }
    suspend fun guardarReglasGrupo(bot: Long, chat: String, texto: String) = io { motor.grupoGuardarReglas(bot, chat, texto) }
    suspend fun filtrosGrupo(bot: Long, chat: String) = io { JsonGrupos.filtros(motor.grupoFiltros(bot, chat)) }
    suspend fun guardarFiltroGrupo(bot: Long, f: FiltroGrupo) = io { motor.grupoGuardarFiltro(bot, f.chat, f.json()) }
    suspend fun restaurarFiltroGrupo(bot: Long, chat: String, filtro: String) = io { motor.grupoRestaurarFiltro(bot, chat, filtro) }

    // Mi WhatsApp: silenciar, fijar y archivar (estado de la app). Solo si el teléfono principal compartió las llaves.
    suspend fun estadoAppState(bot: Long) = io { JsonGrupos.appState(motor.appStateEstado(bot)) }
    /** segundos 0 = siempre. */
    suspend fun silenciarEnMiWhatsApp(bot: Long, chat: String, segundos: Long) = io { motor.appStateSilenciar(bot, chat, segundos) }
    suspend fun desilenciarEnMiWhatsApp(bot: Long, chat: String) = io { motor.appStateDesilenciar(bot, chat) }
    suspend fun fijarEnMiWhatsApp(bot: Long, chat: String, fijar: Boolean) = io { motor.appStateFijar(bot, chat, fijar) }
    suspend fun archivarEnMiWhatsApp(bot: Long, chat: String, archivar: Boolean) = io { motor.appStateArchivar(bot, chat, archivar) }
    suspend fun pedirSincronizacionAppState(bot: Long) = io { motor.appStateSincronizar(bot) }

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
