package com.botchat.app.datos

import wabot.Observador
import wabot.Wabot

/** Lo que la app le pide al módulo Go. Existe para poder probar las pantallas con datos de ejemplo sin cargar la librería nativa. */
interface Motor {
    fun abrirAlmacen(raiz: String, clave: ByteArray, o: Observador?)
    fun listarChats(bot: Long, grupos: Boolean, consulta: String, limite: Long, desde: Long): String
    fun obtenerChat(bot: Long, chat: String): String
    fun listarMensajes(bot: Long, chat: String, antesTs: Long, antesId: String, limite: Long): String
    fun mensajesPorId(bot: Long, chat: String, idsJson: String): String
    /** Cuántos mensajes de otras personas hay en el chat después de `desdeTs` (segundos): COUNT indexado en Go, sin tope de página. */
    fun contarNoLeidos(bot: Long, chat: String, desdeTs: Long): Long
    fun cargaAnterior(si: Boolean)
    fun crearChatPrueba(bot: Long, cuantos: Long): String
    fun listarArchivos(bot: Long, chat: String, categoria: String, soloDescargados: Boolean, limite: Long, desde: Long): String
    fun espacioTotal(bot: Long): String
    fun espacioPorTipo(bot: Long): String
    fun espacioPorChat(bot: Long): String
    fun estimarBorrado(bot: Long, filtro: String): String
    fun borrarArchivos(bot: Long, filtro: String): String
    fun estimarLista(bot: Long, items: String): String
    fun borrarLista(bot: Long, items: String): String
    fun borrarChat(bot: Long, chat: String): String
    fun reparar(bot: Long): String
    fun descargar(bot: Long, chat: String, id: String)
    fun fotoGrande(bot: Long, chat: String): String
    fun establecerZona(id: String)
    fun estadisticasChat(bot: Long, chat: String): String
    fun calendarioChat(bot: Long, chat: String, mes: String): String
    fun mensajesDia(bot: Long, chat: String, dia: String): String
    fun estadisticasGlobales(bot: Long, dias: Long): String
    fun listarFiltrados(bot: Long, chat: String, filtro: String, antesTs: Long, antesId: String, limite: Long): String
    fun personasChat(bot: Long, chat: String): String
    fun olvidarMensaje(bot: Long, chat: String, id: String)
    fun enviarTexto(bot: Long, chat: String, texto: String, citaId: String, mencionesJson: String): String
    fun reintentarEnvio(bot: Long, chat: String, id: String): String
    fun reaccionar(bot: Long, chat: String, id: String, emoji: String)
    fun editarMensaje(bot: Long, chat: String, id: String, texto: String)
    fun borrarParaTodos(bot: Long, chat: String, id: String)
    fun borrarParaMi(bot: Long, chat: String, idsJson: String): Long
    fun reenviarTexto(bot: Long, origen: String, idsJson: String, destinosJson: String): String
    fun marcarLeido(bot: Long, chat: String)
    fun escribiendo(bot: Long, chat: String, si: Boolean)
    fun enLinea(bot: Long, si: Boolean)
    fun suscribirPresencia(bot: Long, chat: String)
    fun presenciaDe(chat: String): String
    fun participantesGrupo(bot: Long, chat: String): String
    fun infoMensaje(bot: Long, chat: String, id: String): String
    fun guardarBorrador(bot: Long, chat: String, texto: String)
    fun leerBorrador(bot: Long, chat: String): String
    fun silenciarChat(bot: Long, chat: String, si: Boolean)
    /** Fija (o suelta) el chat arriba de la lista; solo en esta app. */
    fun fijarChatLocal(bot: Long, chat: String, si: Boolean)
    /** Borra mensajes y archivos del chat pero deja el chat, sus etiquetas y ajustes. No se puede deshacer. */
    fun vaciarChat(bot: Long, chat: String): String
    fun estadoDelChat(bot: Long, chat: String): String
    fun plazoEdicionSeg(): Long
    fun configurarEnvio(pausaBotMin: Long)
    fun enviarMedio(bot: Long, chat: String, tipo: String, ruta: String, mime: String, nombre: String, pie: String, citaId: String, ancho: Long, alto: Long, durS: Long, notaVoz: Boolean, miniatura: ByteArray?, onda: ByteArray?): String
    fun abrirVerUnaVez(bot: Long, chat: String, id: String): String
    fun cerrarVerUnaVez(bot: Long, chat: String, id: String)
    fun barrerVerUnaVez(bot: Long): Long
    /** (JSON) medidor contra el tope total, uso por tipo con sus topes e interruptores, ahorro por duplicados y videos esperando portada. */
    fun estadoAlmacenamiento(bot: Long): String
    /** Tope total en bytes (0 = sin límite; de fábrica 5 GB). */
    fun topeAlmacenamiento(bot: Long): Long
    fun fijarTope(bot: Long, bytes: Long)
    /** Aplica el tope ya y devuelve (JSON) lo que borró. */
    fun aplicarTopeAhora(bot: Long): String
    fun conservarPortadaVideo(bot: Long): Boolean
    fun fijarConservarPortada(bot: Long, si: Boolean)
    fun configurarVerUnaVez(dias: Long)
    fun configurarMotor(activo: Boolean, ventanaMin: Long)
    fun actividadMotor(bot: Long, limite: Long): String
    fun botEnChat(bot: Long, chat: String, activo: Boolean)
    fun pausarBotEnChat(bot: Long, chat: String, minutos: Long)
    fun listarAutomatizaciones(bot: Long): String
    fun obtenerAutomatizacion(bot: Long, id: Long): String
    fun validarAutomatizacion(def: String): String
    fun guardarAutomatizacion(bot: Long, id: Long, def: String): String
    fun activarAutomatizacion(bot: Long, id: Long, si: Boolean)
    fun duplicarAutomatizacion(bot: Long, id: Long): Long
    fun borrarAutomatizacion(bot: Long, id: Long)
    fun reordenarAutomatizaciones(bot: Long, idsJson: String)
    fun plantillasAutomatizacion(): String
    fun exportarAutomatizacion(bot: Long, id: Long): String
    fun importarAutomatizacion(bot: Long, def: String): String
    fun simularAutomatizacion(bot: Long, id: Long, texto: String, reiniciar: Boolean): String
    fun sesionesEnCurso(bot: Long): String
    fun recorridoSesion(bot: Long, id: Long): String
    fun accionSesion(bot: Long, id: Long, accion: String)
    fun estadisticasAutomatizacion(bot: Long, id: Long, desde: String, hasta: String, prueba: Boolean): String
    fun etiquetaDeChat(bot: Long, chat: String): String
    fun obtenerAviso(bot: Long, id: Long): String
    fun configurarFlujo(expiraMin: Long)

    // Menús ampliados (0.5.0, bloque C)
    fun ajustesDeMenus(bot: Long): String
    fun fijarAjusteDeMenus(bot: Long, clave: String, valor: String)
    fun listarFormularios(bot: Long, menuId: Long, limite: Long): String
    fun obtenerFormulario(bot: Long, id: Long): String
    fun contarFormularios(bot: Long, menuId: Long): Long
    fun borrarFormulario(bot: Long, id: Long)
    fun borrarFormularios(bot: Long, menuId: Long): Long
    fun exportarFormularios(bot: Long, menuId: Long): String
    fun listarNoEntendio(bot: Long, menuId: Long, limite: Long): String
    fun borrarNoEntendio(bot: Long, menuId: Long): Long
    fun subirAdjuntoMenu(bot: Long, ruta: String, tipo: String, mime: String, nombre: String): String
    fun borrarAdjuntoMenu(bot: Long, id: String)
    fun exportarMenus(bot: Long, idsJson: String, clave: String): String
    fun importarMenus(bot: Long, ruta: String, clave: String): String

    // Rangos y permisos (0.5.0, bloque A)
    fun catalogoPermisos(): String
    fun listarRangos(bot: Long): String
    fun obtenerRango(bot: Long, id: Long): String
    fun guardarRango(bot: Long, rolJson: String): Long
    fun borrarRango(bot: Long, id: Long): Long
    fun reordenarRangos(bot: Long, idsJson: String)
    fun asignarRango(bot: Long, persona: String, rolId: Long, duracion: String)
    fun revocarRango(bot: Long, persona: String)
    fun rangoDePersona(bot: Long, persona: String): String
    fun listarAsignaciones(bot: Long): String
    fun listarSolicitudes(bot: Long, estado: String): String
    fun contarSolicitudes(bot: Long): Long
    fun aprobarSolicitud(bot: Long, persona: String)
    fun bloquearSolicitud(bot: Long, persona: String)
    fun ignorarSolicitud(bot: Long, persona: String)
    fun quitarSolicitud(bot: Long, persona: String)
    fun politicaDeGrupo(bot: Long, chat: String): String
    fun fijarPoliticaDeGrupo(bot: Long, politicaJson: String)
    fun listarPoliticasDeGrupo(bot: Long): String
    fun ajustesDePermisos(bot: Long): String
    fun fijarAjusteDePermisos(bot: Long, clave: String, valor: String)
    fun confirmacionesPendientes(bot: Long): String
    fun confirmarCodigo(bot: Long, codigo: String): String
    fun cancelarConfirmacion(bot: Long, codigo: String)
    fun ejecutarComando(bot: Long, texto: String): String
    fun listarAcciones(bot: Long, limite: Long, antesId: Long): String

    // Módulos e iniciativa propia (0.5.0, H0). Eventos: "modulos", "iniciativas".
    fun listarModulos(bot: Long): String
    fun infoModulo(bot: Long, id: String): String
    fun activarModulo(bot: Long, id: String)
    fun desactivarModulo(bot: Long, id: String)
    fun configModulo(bot: Long, id: String): String
    fun guardarConfigModulo(bot: Long, id: String, configJson: String)
    fun fijarTopesModulo(bot: Long, id: String, hora: Long, dia: Long)
    fun ajustesIniciativa(bot: Long): String
    fun fijarAjusteIniciativa(bot: Long, clave: String, valor: String)
    fun fijarIniciativaMaestro(bot: Long, activo: Boolean)
    fun listarNoMolestar(bot: Long): String
    fun agregarNoMolestar(bot: Long, persona: String, motivo: String)
    fun quitarNoMolestar(bot: Long, persona: String)
    fun listarIniciativas(bot: Long, modulo: String, limite: Long, antesId: Long): String
    // Paneles de los módulos H (listas y formularios simples) y la alarma de lo agendado (0.5.0, H parte 1).
    fun filasPanelModulo(bot: Long, id: String, panel: String): String
    fun accionModulo(bot: Long, id: String, accion: String, argsJson: String): String
    fun proximoModulos(bot: Long): Long
    fun tickModulos(bot: Long)
    // Llaves de los módulos (IA, traductor): la app las guarda en el Keystore y se las pasa a Go solo en memoria (0.5.0, H parte 2).
    fun fijarLlaveModulo(bot: Long, id: String, clave: String, valor: String)
    fun estadoModoSeguro(bot: Long): String
    fun reanudarModoSeguro(bot: Long)

    // Personas, chat y avisos para mí (0.5.0, bloque B). Eventos: "personas", "persona:<persona>", "etiquetas", "recordatorios", "alertas".
    fun fichaPersona(bot: Long, persona: String): String
    fun estadisticasPersona(bot: Long, persona: String, dias: Long): String
    fun silenciarPersona(bot: Long, persona: String, minutos: Long, motivo: String)
    fun quitarSilencioPersona(bot: Long, persona: String): Boolean
    fun listarSilencios(bot: Long): String
    fun bloquearPersona(bot: Long, persona: String)
    fun desbloquearPersona(bot: Long, persona: String): Boolean
    fun advertirPersona(bot: Long, persona: String, chat: String, motivo: String): String
    fun listarAdvertencias(bot: Long, persona: String): String
    fun listarPersonasAdvertidas(bot: Long): String
    fun quitarAdvertenciaPersona(bot: Long, persona: String, id: Long): Long
    fun borrarAdvertenciasPersona(bot: Long, persona: String): Long
    fun listarNotasPersona(bot: Long, persona: String): String
    fun agregarNotaPersona(bot: Long, persona: String, texto: String)
    fun borrarNotaPersona(bot: Long, persona: String, id: Long)
    fun olvidarPersona(bot: Long, persona: String): String
    fun exportarPersona(bot: Long, persona: String): String
    fun marcarEstadoChat(bot: Long, chat: String, estado: String)
    fun listarChatsFiltrados(bot: Long, grupos: Boolean, consulta: String, estado: String, etiqueta: Long, limite: Long, desde: Long): String
    fun contarPendientes(bot: Long): Long
    fun listarEtiquetas(bot: Long): String
    fun crearEtiqueta(bot: Long, nombre: String, color: String): Long
    fun editarEtiqueta(bot: Long, id: Long, nombre: String, color: String)
    fun borrarEtiqueta(bot: Long, id: Long)
    fun ponerEtiquetaChat(bot: Long, chat: String, id: Long, si: Boolean)
    fun etiquetasDeChat(bot: Long, chat: String): String

    // Etiquetas de personas y mensajes, "Marcados" y miembros de grupo (0.6.0, E3). Eventos: "etiquetas", "personas", "grupo:<jid>".
    fun crearEtiquetaEmoji(bot: Long, nombre: String, color: String, emoji: String): Long
    fun editarEtiquetaEmoji(bot: Long, id: Long, nombre: String, color: String, emoji: String)
    fun ponerEtiquetaPersona(bot: Long, persona: String, id: Long, si: Boolean)
    fun etiquetasDePersona(bot: Long, persona: String): String
    fun etiquetasDePersonas(bot: Long, personasJson: String): String
    fun etiquetasDeChats(bot: Long, chatsJson: String): String
    fun marcarMensaje(bot: Long, chat: String, msgId: String, etiqueta: Long, nota: String)
    fun quitarMarcaMensaje(bot: Long, chat: String, msgId: String, etiqueta: Long)
    fun marcasDeMensaje(bot: Long, chat: String, msgId: String): String
    fun etiquetasDeMensajes(bot: Long, chat: String, idsJson: String): String
    fun listarMarcados(bot: Long, tipo: String, etiqueta: Long, limite: Long, desde: Long): String
    fun contarMarcados(bot: Long, etiqueta: Long): String
    fun miembrosGrupoDetallado(bot: Long, grupo: String, incluirSalidos: Boolean): String
    fun cambiosMiembrosGrupo(bot: Long, grupo: String, antes: Long, limite: Long): String
    fun actualizarMiembrosGrupo(bot: Long)
    fun gruposEnComunPersona(bot: Long, persona: String): String
    fun pedirFotoPersona(bot: Long, persona: String, grupo: String, forzar: Boolean)
    fun fotoPersona(bot: Long, persona: String): String

    // Perfil de persona, actividad, lista de lo que envió, búsqueda en un chat, puntitos y exportar chat (0.6.0, E4).
    fun perfilPersona(bot: Long, persona: String): String
    fun actividadPersona(bot: Long, persona: String): String
    fun mensajesDePersona(bot: Long, filtroJson: String, antesTs: Long, antesId: String, limite: Long): String
    fun buscarEnChat(bot: Long, chat: String, palabra: String, desde: Long, hasta: Long, antesTs: Long, antesId: String, limite: Long): String
    fun puntitosDeMensajes(bot: Long, chat: String, idsJson: String): String
    fun exportarChat(bot: Long, chat: String, formato: String, desde: Long, hasta: Long): String
    // Silenciar en grupos y advertencias con sanciones (v0.6.0, E5). Sin pantallas todavía (E7/E8).
    fun silenciarEnGrupos(bot: Long, persona: String, segundos: Long, alcance: String, gruposJson: String, avisar: Boolean, texto: String): String
    fun quitarSilencioGrupos(bot: Long, persona: String): Boolean
    fun listarSilenciosGrupos(bot: Long): String
    fun historialSilencios(bot: Long, persona: String): String
    fun contadorSilencio(bot: Long, persona: String, grupo: String): String
    fun ponerContadorEnCero(bot: Long, persona: String, grupo: String)
    fun ajustesSilencio(bot: Long): String
    fun fijarAjusteSilencio(bot: Long, clave: String, valor: String)
    fun fijarTextoSilencio(bot: Long, id: String, texto: String)
    fun sancionesPendientes(bot: Long): String
    fun confirmarSancionSilencio(bot: Long, id: Long, si: Boolean): String
    fun confirmarSancion(bot: Long, id: Long, si: Boolean): String
    fun gruposQueAdministro(bot: Long): String
    fun listarTiposAdvertencia(bot: Long): String
    fun guardarTipoAdvertencia(bot: Long, tipoJson: String): Long
    fun borrarTipoAdvertencia(bot: Long, id: Long)
    fun listarEscalera(bot: Long, tipoId: Long): String
    fun guardarEscalera(bot: Long, tipoId: Long, pasosJson: String)
    fun advertirConTipo(bot: Long, persona: String, chat: String, msgId: String, motivo: String, tipoId: Long, reglaId: Long): String
    fun listarReglasGrupo(bot: Long, grupo: String): String
    fun guardarReglaItem(bot: Long, grupo: String, id: Long, titulo: String, texto: String, pos: Long): Long
    fun borrarReglaItem(bot: Long, id: Long)
    fun moverReglaItem(bot: Long, id: Long, delta: Long)
    fun listarAlertasPalabra(bot: Long): String
    fun crearAlertaPalabra(bot: Long, palabra: String, chatsJson: String): Long
    fun editarAlertaPalabra(bot: Long, id: Long, chatsJson: String, activa: Boolean)
    fun borrarAlertaPalabra(bot: Long, id: Long)
    fun listarRecordatorios(bot: Long, hechos: Boolean): String
    fun crearRecordatorio(bot: Long, texto: String, proximaTs: Long, repetir: String): Long
    fun editarRecordatorio(bot: Long, id: Long, texto: String, proximaTs: Long, repetir: String)
    fun borrarRecordatorio(bot: Long, id: Long)
    // Notas y pendientes propios del dueño (0.5.0, bloque F). tipo: "nota" o "pendiente". Evento: "util:yo".
    fun listarUtilidades(bot: Long, tipo: String): String
    fun agregarUtilidad(bot: Long, tipo: String, texto: String): Long
    fun marcarUtilidad(bot: Long, id: Long, hecho: Boolean)
    fun borrarUtilidad(bot: Long, id: Long)
    fun limpiarUtilidadesHechas(bot: Long): Long
    // Laboratorio (0.5.0, bloque G): menú como encuesta o botones y matriz de pruebas. Evento: "lab".
    fun estadoLab(bot: Long): String
    fun aceptarAvisoLab(bot: Long)
    fun fijarAjusteLab(bot: Long, clave: String, valor: String)
    fun enviarPruebaLab(bot: Long, tipo: String, variante: Long): String
    fun listarResultadosLab(bot: Long): String
    fun marcarResultadoLab(bot: Long, id: Long, campo: String, valor: Long)
    fun borrarResultadosLab(bot: Long): Long
    // Captura de mensajes (diagnóstico del Laboratorio): solo estructura, solo en memoria.
    fun estadoCaptura(bot: Long): String
    fun activarCaptura(bot: Long, activo: Boolean)
    fun mostrarTextosCaptura(bot: Long, mostrar: Boolean)
    fun borrarCaptura(bot: Long)
    fun usarLimiteMedido(bot: Long): Long
    fun proximoAviso(bot: Long): Long
    fun dispararAvisos(bot: Long): String
    fun vistaPreviaResumen(bot: Long): String
    fun listarComandosPropios(bot: Long): String
    fun guardarComandoPropio(bot: Long, id: Long, datosJson: String): Long
    fun vistaPreviaComandoPropio(bot: Long, datosJson: String): String

    // Mensajes automáticos y adjuntos (0.5.0, bloque D). Evento: "auto", "auto_registro".
    fun listarAutos(bot: Long, tipo: String): String
    fun obtenerAuto(bot: Long, id: Long): String
    fun guardarAuto(bot: Long, entradaJson: String): Long
    fun fijarAutoActivo(bot: Long, id: Long, activo: Boolean)
    fun borrarAuto(bot: Long, id: Long)
    fun reordenarAutos(bot: Long, idsJson: String)
    fun plantillasAuto(): String
    fun configPorDefectoAuto(tipo: String): String
    fun probarAuto(bot: Long, entradaJson: String, escenarioJson: String): String
    fun registroAuto(bot: Long, autoId: Long, limite: Long): String
    fun subirAdjunto(bot: Long, ruta: String, tipo: String, mime: String, nombre: String): String
    fun borrarAdjunto(bot: Long, id: String)
    fun limpiarAdjuntos(bot: Long): Long
    fun listarAtajos(bot: Long, chat: String): String
    fun enviarArchivosAtajo(bot: Long, chat: String, id: Long)
    fun listarMensajesAuto(bot: Long, chat: String, antesTs: Long, limite: Long): String
    fun chatsConAutomaticos(bot: Long, dias: Long): String

    // Grupos y estado de la app de WhatsApp (0.5.0, bloque E)
    fun grupoEstado(bot: Long, chat: String, refrescar: Boolean): String
    fun grupoInfo(bot: Long, chat: String): String
    fun grupoMiembros(bot: Long, chat: String): String
    fun grupoCerrar(bot: Long, chat: String, cerrado: Boolean)
    fun grupoBloquearInfo(bot: Long, chat: String, bloqueada: Boolean)
    fun grupoExpulsar(bot: Long, chat: String, persona: String)
    fun grupoAscender(bot: Long, chat: String, persona: String, admin: Boolean)
    fun grupoSolicitudes(bot: Long, chat: String): String
    fun grupoResolverSolicitudes(bot: Long, chat: String, personasJson: String, aprobar: Boolean): Long
    fun grupoRenombrar(bot: Long, chat: String, nombre: String)
    fun grupoDescripcion(bot: Long, chat: String, texto: String)
    fun grupoEnlace(bot: Long, chat: String, reiniciar: Boolean): String
    fun grupoBorrarMensaje(bot: Long, chat: String, msgId: String)
    fun grupoMencionarTodos(bot: Long, chat: String, texto: String): String
    fun grupoRanking(bot: Long, chat: String, dias: Long, limite: Long): String
    fun grupoInactivos(bot: Long, chat: String, dias: Long): String
    fun grupoReglas(bot: Long, chat: String): String
    fun grupoGuardarReglas(bot: Long, chat: String, texto: String)
    fun grupoFiltros(bot: Long, chat: String): String
    fun grupoGuardarFiltro(bot: Long, chat: String, datosJson: String)
    fun grupoRestaurarFiltro(bot: Long, chat: String, filtro: String)
    fun appStateDisponible(bot: Long): Boolean
    fun appStateEstado(bot: Long): String
    fun appStateSilenciar(bot: Long, chat: String, segundos: Long)
    fun appStateDesilenciar(bot: Long, chat: String)
    fun appStateFijar(bot: Long, chat: String, fijar: Boolean)
    fun appStateArchivar(bot: Long, chat: String, archivar: Boolean)
    fun appStateSincronizar(bot: Long)
}

/** El módulo Go de verdad (whatsmeow + almacén cifrado). */
object MotorGo : Motor {
    override fun abrirAlmacen(raiz: String, clave: ByteArray, o: Observador?) = Wabot.abrirAlmacen(raiz, clave, o)
    override fun listarChats(bot: Long, grupos: Boolean, consulta: String, limite: Long, desde: Long) = Wabot.listarChats(bot, grupos, consulta, limite, desde)
    override fun obtenerChat(bot: Long, chat: String) = Wabot.obtenerChat(bot, chat)
    override fun listarMensajes(bot: Long, chat: String, antesTs: Long, antesId: String, limite: Long) = Wabot.listarMensajes(bot, chat, antesTs, antesId, limite)
    override fun mensajesPorId(bot: Long, chat: String, idsJson: String) = Wabot.mensajesPorId(bot, chat, idsJson)
    override fun contarNoLeidos(bot: Long, chat: String, desdeTs: Long) = Wabot.contarNoLeidos(bot, chat, desdeTs)
    override fun cargaAnterior(si: Boolean) = Wabot.cargaAnterior(si)
    override fun crearChatPrueba(bot: Long, cuantos: Long) = Wabot.crearChatPrueba(bot, cuantos)
    override fun listarArchivos(bot: Long, chat: String, categoria: String, soloDescargados: Boolean, limite: Long, desde: Long) = Wabot.listarArchivos(bot, chat, categoria, soloDescargados, limite, desde)
    override fun espacioTotal(bot: Long) = Wabot.espacioTotal(bot)
    override fun espacioPorTipo(bot: Long) = Wabot.espacioPorTipo(bot)
    override fun espacioPorChat(bot: Long) = Wabot.espacioPorChat(bot)
    override fun estimarBorrado(bot: Long, filtro: String) = Wabot.estimarBorrado(bot, filtro)
    override fun borrarArchivos(bot: Long, filtro: String) = Wabot.borrarArchivos(bot, filtro)
    override fun estimarLista(bot: Long, items: String) = Wabot.estimarLista(bot, items)
    override fun borrarLista(bot: Long, items: String) = Wabot.borrarLista(bot, items)
    override fun borrarChat(bot: Long, chat: String) = Wabot.borrarChat(bot, chat)
    override fun reparar(bot: Long) = Wabot.reparar(bot)
    override fun descargar(bot: Long, chat: String, id: String) = Wabot.descargar(bot, chat, id)
    override fun fotoGrande(bot: Long, chat: String) = Wabot.fotoGrande(bot, chat)
    override fun establecerZona(id: String) = Wabot.establecerZona(id)
    override fun estadisticasChat(bot: Long, chat: String) = Wabot.estadisticasChat(bot, chat)
    override fun calendarioChat(bot: Long, chat: String, mes: String) = Wabot.calendarioChat(bot, chat, mes)
    override fun mensajesDia(bot: Long, chat: String, dia: String) = Wabot.mensajesDia(bot, chat, dia)
    override fun estadisticasGlobales(bot: Long, dias: Long) = Wabot.estadisticasGlobales(bot, dias)
    override fun listarFiltrados(bot: Long, chat: String, filtro: String, antesTs: Long, antesId: String, limite: Long) = Wabot.listarFiltrados(bot, chat, filtro, antesTs, antesId, limite)
    override fun personasChat(bot: Long, chat: String) = Wabot.personasChat(bot, chat)
    override fun olvidarMensaje(bot: Long, chat: String, id: String) = Wabot.olvidarMensaje(bot, chat, id)
    override fun enviarTexto(bot: Long, chat: String, texto: String, citaId: String, mencionesJson: String) = Wabot.enviarTexto(bot, chat, texto, citaId, mencionesJson)
    override fun reintentarEnvio(bot: Long, chat: String, id: String) = Wabot.reintentarEnvio(bot, chat, id)
    override fun reaccionar(bot: Long, chat: String, id: String, emoji: String) = Wabot.reaccionar(bot, chat, id, emoji)
    override fun editarMensaje(bot: Long, chat: String, id: String, texto: String) = Wabot.editarMensaje(bot, chat, id, texto)
    override fun borrarParaTodos(bot: Long, chat: String, id: String) = Wabot.borrarParaTodos(bot, chat, id)
    override fun borrarParaMi(bot: Long, chat: String, idsJson: String) = Wabot.borrarParaMi(bot, chat, idsJson).toLong()
    override fun reenviarTexto(bot: Long, origen: String, idsJson: String, destinosJson: String) = Wabot.reenviarTexto(bot, origen, idsJson, destinosJson)
    override fun marcarLeido(bot: Long, chat: String) = Wabot.marcarLeido(bot, chat)
    override fun escribiendo(bot: Long, chat: String, si: Boolean) = Wabot.escribiendo(bot, chat, si)
    override fun enLinea(bot: Long, si: Boolean) = Wabot.enLinea(bot, si)
    override fun suscribirPresencia(bot: Long, chat: String) = Wabot.suscribirPresencia(bot, chat)
    override fun presenciaDe(chat: String) = Wabot.presenciaDe(chat)
    override fun participantesGrupo(bot: Long, chat: String) = Wabot.participantesGrupo(bot, chat)
    override fun infoMensaje(bot: Long, chat: String, id: String) = Wabot.infoMensaje(bot, chat, id)
    override fun guardarBorrador(bot: Long, chat: String, texto: String) = Wabot.guardarBorrador(bot, chat, texto)
    override fun leerBorrador(bot: Long, chat: String) = Wabot.leerBorrador(bot, chat)
    override fun silenciarChat(bot: Long, chat: String, si: Boolean) = Wabot.silenciarChat(bot, chat, si)
    override fun fijarChatLocal(bot: Long, chat: String, si: Boolean) = Wabot.fijarChatLocal(bot, chat, si)
    override fun vaciarChat(bot: Long, chat: String) = Wabot.vaciarChat(bot, chat)
    override fun estadoDelChat(bot: Long, chat: String) = Wabot.estadoDelChat(bot, chat)
    override fun plazoEdicionSeg() = Wabot.plazoEdicionSeg().toLong()
    override fun configurarEnvio(pausaBotMin: Long) = Wabot.configurarEnvio(pausaBotMin)
    override fun enviarMedio(bot: Long, chat: String, tipo: String, ruta: String, mime: String, nombre: String, pie: String, citaId: String, ancho: Long, alto: Long, durS: Long, notaVoz: Boolean, miniatura: ByteArray?, onda: ByteArray?) =
        Wabot.enviarMedio(bot, chat, tipo, ruta, mime, nombre, pie, citaId, ancho, alto, durS, notaVoz, miniatura, onda)
    override fun abrirVerUnaVez(bot: Long, chat: String, id: String) = Wabot.abrirVerUnaVez(bot, chat, id)
    override fun cerrarVerUnaVez(bot: Long, chat: String, id: String) = Wabot.cerrarVerUnaVez(bot, chat, id)
    override fun barrerVerUnaVez(bot: Long) = Wabot.barrerVerUnaVez(bot).toLong()
    override fun estadoAlmacenamiento(bot: Long) = Wabot.estadoAlmacenamiento(bot)
    override fun topeAlmacenamiento(bot: Long) = Wabot.topeAlmacenamiento(bot)
    override fun fijarTope(bot: Long, bytes: Long) = Wabot.fijarTope(bot, bytes)
    override fun aplicarTopeAhora(bot: Long) = Wabot.aplicarTopeAhora(bot)
    override fun conservarPortadaVideo(bot: Long) = Wabot.conservarPortadaVideo(bot)
    override fun fijarConservarPortada(bot: Long, si: Boolean) = Wabot.fijarConservarPortada(bot, si)
    override fun configurarVerUnaVez(dias: Long) = Wabot.configurarVerUnaVez(dias)
    override fun configurarMotor(activo: Boolean, ventanaMin: Long) = Wabot.configurarMotor(activo, ventanaMin)
    override fun actividadMotor(bot: Long, limite: Long) = Wabot.actividadMotor(bot, limite)
    override fun botEnChat(bot: Long, chat: String, activo: Boolean) = Wabot.botEnChat(bot, chat, activo)
    override fun pausarBotEnChat(bot: Long, chat: String, minutos: Long) = Wabot.pausarBotEnChat(bot, chat, minutos)
    override fun listarAutomatizaciones(bot: Long) = Wabot.listarAutomatizaciones(bot)
    override fun obtenerAutomatizacion(bot: Long, id: Long) = Wabot.obtenerAutomatizacion(bot, id)
    override fun validarAutomatizacion(def: String) = Wabot.validarAutomatizacion(def)
    override fun guardarAutomatizacion(bot: Long, id: Long, def: String) = Wabot.guardarAutomatizacion(bot, id, def)
    override fun activarAutomatizacion(bot: Long, id: Long, si: Boolean) = Wabot.activarAutomatizacion(bot, id, si)
    override fun duplicarAutomatizacion(bot: Long, id: Long) = Wabot.duplicarAutomatizacion(bot, id)
    override fun borrarAutomatizacion(bot: Long, id: Long) = Wabot.borrarAutomatizacion(bot, id)
    override fun reordenarAutomatizaciones(bot: Long, idsJson: String) = Wabot.reordenarAutomatizaciones(bot, idsJson)
    override fun plantillasAutomatizacion() = Wabot.plantillasAutomatizacion()
    override fun exportarAutomatizacion(bot: Long, id: Long) = Wabot.exportarAutomatizacion(bot, id)
    override fun importarAutomatizacion(bot: Long, def: String) = Wabot.importarAutomatizacion(bot, def)
    override fun simularAutomatizacion(bot: Long, id: Long, texto: String, reiniciar: Boolean) = Wabot.simularAutomatizacion(bot, id, texto, reiniciar)
    override fun sesionesEnCurso(bot: Long) = Wabot.sesionesEnCurso(bot)
    override fun recorridoSesion(bot: Long, id: Long) = Wabot.recorridoSesion(bot, id)
    override fun accionSesion(bot: Long, id: Long, accion: String) = Wabot.accionSesion(bot, id, accion)
    override fun estadisticasAutomatizacion(bot: Long, id: Long, desde: String, hasta: String, prueba: Boolean) = Wabot.estadisticasAutomatizacion(bot, id, desde, hasta, prueba)
    override fun etiquetaDeChat(bot: Long, chat: String) = Wabot.etiquetaDeChat(bot, chat)
    override fun obtenerAviso(bot: Long, id: Long) = Wabot.obtenerAviso(bot, id)
    override fun configurarFlujo(expiraMin: Long) = Wabot.configurarFlujo(expiraMin)
    override fun ajustesDeMenus(bot: Long) = Wabot.ajustesDeMenus(bot)
    override fun fijarAjusteDeMenus(bot: Long, clave: String, valor: String) = Wabot.fijarAjusteDeMenus(bot, clave, valor)
    override fun listarFormularios(bot: Long, menuId: Long, limite: Long) = Wabot.listarFormularios(bot, menuId, limite)
    override fun obtenerFormulario(bot: Long, id: Long) = Wabot.obtenerFormulario(bot, id)
    override fun contarFormularios(bot: Long, menuId: Long) = Wabot.contarFormularios(bot, menuId)
    override fun borrarFormulario(bot: Long, id: Long) = Wabot.borrarFormulario(bot, id)
    override fun borrarFormularios(bot: Long, menuId: Long) = Wabot.borrarFormularios(bot, menuId)
    override fun exportarFormularios(bot: Long, menuId: Long) = Wabot.exportarFormularios(bot, menuId)
    override fun listarNoEntendio(bot: Long, menuId: Long, limite: Long) = Wabot.listarNoEntendio(bot, menuId, limite)
    override fun borrarNoEntendio(bot: Long, menuId: Long) = Wabot.borrarNoEntendio(bot, menuId)
    override fun subirAdjuntoMenu(bot: Long, ruta: String, tipo: String, mime: String, nombre: String) = Wabot.subirAdjuntoMenu(bot, ruta, tipo, mime, nombre)
    override fun borrarAdjuntoMenu(bot: Long, id: String) = Wabot.borrarAdjuntoMenu(bot, id)
    override fun exportarMenus(bot: Long, idsJson: String, clave: String) = Wabot.exportarMenus(bot, idsJson, clave)
    override fun importarMenus(bot: Long, ruta: String, clave: String) = Wabot.importarMenus(bot, ruta, clave)
    override fun catalogoPermisos() = Wabot.catalogoPermisos()
    override fun listarRangos(bot: Long) = Wabot.listarRangos(bot)
    override fun obtenerRango(bot: Long, id: Long) = Wabot.obtenerRango(bot, id)
    override fun guardarRango(bot: Long, rolJson: String) = Wabot.guardarRango(bot, rolJson)
    override fun borrarRango(bot: Long, id: Long) = Wabot.borrarRango(bot, id).toLong()
    override fun reordenarRangos(bot: Long, idsJson: String) = Wabot.reordenarRangos(bot, idsJson)
    override fun asignarRango(bot: Long, persona: String, rolId: Long, duracion: String) = Wabot.asignarRango(bot, persona, rolId, duracion)
    override fun revocarRango(bot: Long, persona: String) = Wabot.revocarRango(bot, persona)
    override fun rangoDePersona(bot: Long, persona: String) = Wabot.rangoDePersona(bot, persona)
    override fun listarAsignaciones(bot: Long) = Wabot.listarAsignaciones(bot)
    override fun listarSolicitudes(bot: Long, estado: String) = Wabot.listarSolicitudes(bot, estado)
    override fun contarSolicitudes(bot: Long) = Wabot.contarSolicitudes(bot).toLong()
    override fun aprobarSolicitud(bot: Long, persona: String) = Wabot.aprobarSolicitud(bot, persona)
    override fun bloquearSolicitud(bot: Long, persona: String) = Wabot.bloquearSolicitud(bot, persona)
    override fun ignorarSolicitud(bot: Long, persona: String) = Wabot.ignorarSolicitud(bot, persona)
    override fun quitarSolicitud(bot: Long, persona: String) = Wabot.quitarSolicitud(bot, persona)
    override fun politicaDeGrupo(bot: Long, chat: String) = Wabot.politicaDeGrupo(bot, chat)
    override fun fijarPoliticaDeGrupo(bot: Long, politicaJson: String) = Wabot.fijarPoliticaDeGrupo(bot, politicaJson)
    override fun listarPoliticasDeGrupo(bot: Long) = Wabot.listarPoliticasDeGrupo(bot)
    override fun ajustesDePermisos(bot: Long) = Wabot.ajustesDePermisos(bot)
    override fun fijarAjusteDePermisos(bot: Long, clave: String, valor: String) = Wabot.fijarAjusteDePermisos(bot, clave, valor)
    override fun confirmacionesPendientes(bot: Long) = Wabot.confirmacionesPendientes(bot)
    override fun confirmarCodigo(bot: Long, codigo: String) = Wabot.confirmarCodigo(bot, codigo)
    override fun cancelarConfirmacion(bot: Long, codigo: String) = Wabot.cancelarConfirmacion(bot, codigo)
    override fun ejecutarComando(bot: Long, texto: String) = Wabot.ejecutarComando(bot, texto)
    override fun listarAcciones(bot: Long, limite: Long, antesId: Long) = Wabot.listarAcciones(bot, limite, antesId)
    override fun listarModulos(bot: Long) = Wabot.listarModulos(bot)
    override fun infoModulo(bot: Long, id: String) = Wabot.infoModulo(bot, id)
    override fun activarModulo(bot: Long, id: String) = Wabot.activarModulo(bot, id)
    override fun desactivarModulo(bot: Long, id: String) = Wabot.desactivarModulo(bot, id)
    override fun configModulo(bot: Long, id: String) = Wabot.configModulo(bot, id)
    override fun guardarConfigModulo(bot: Long, id: String, configJson: String) = Wabot.guardarConfigModulo(bot, id, configJson)
    override fun fijarTopesModulo(bot: Long, id: String, hora: Long, dia: Long) = Wabot.fijarTopesModulo(bot, id, hora, dia)
    override fun ajustesIniciativa(bot: Long) = Wabot.ajustesIniciativa(bot)
    override fun fijarAjusteIniciativa(bot: Long, clave: String, valor: String) = Wabot.fijarAjusteIniciativa(bot, clave, valor)
    override fun fijarIniciativaMaestro(bot: Long, activo: Boolean) = Wabot.fijarIniciativaMaestro(bot, activo)
    override fun listarNoMolestar(bot: Long) = Wabot.listarNoMolestar(bot)
    override fun agregarNoMolestar(bot: Long, persona: String, motivo: String) = Wabot.agregarNoMolestar(bot, persona, motivo)
    override fun quitarNoMolestar(bot: Long, persona: String) = Wabot.quitarNoMolestar(bot, persona)
    override fun listarIniciativas(bot: Long, modulo: String, limite: Long, antesId: Long) = Wabot.listarIniciativas(bot, modulo, limite, antesId)
    override fun filasPanelModulo(bot: Long, id: String, panel: String) = Wabot.filasPanelModulo(bot, id, panel)
    override fun accionModulo(bot: Long, id: String, accion: String, argsJson: String) = Wabot.accionModulo(bot, id, accion, argsJson)
    override fun proximoModulos(bot: Long) = Wabot.proximoModulos(bot)
    override fun tickModulos(bot: Long) = Wabot.tickModulos(bot)
    override fun fijarLlaveModulo(bot: Long, id: String, clave: String, valor: String) = Wabot.fijarLlaveModulo(bot, id, clave, valor)
    override fun estadoModoSeguro(bot: Long) = Wabot.estadoModoSeguro(bot)
    override fun reanudarModoSeguro(bot: Long) = Wabot.reanudarModoSeguro(bot)

    override fun fichaPersona(bot: Long, persona: String) = Wabot.fichaPersona(bot, persona)
    override fun estadisticasPersona(bot: Long, persona: String, dias: Long) = Wabot.estadisticasPersona(bot, persona, dias)
    override fun silenciarPersona(bot: Long, persona: String, minutos: Long, motivo: String) = Wabot.silenciarPersona(bot, persona, minutos, motivo)
    override fun quitarSilencioPersona(bot: Long, persona: String) = Wabot.quitarSilencioPersona(bot, persona)
    override fun listarSilencios(bot: Long) = Wabot.listarSilencios(bot)
    override fun bloquearPersona(bot: Long, persona: String) = Wabot.bloquearPersona(bot, persona)
    override fun desbloquearPersona(bot: Long, persona: String) = Wabot.desbloquearPersona(bot, persona)
    override fun advertirPersona(bot: Long, persona: String, chat: String, motivo: String) = Wabot.advertirPersona(bot, persona, chat, motivo)
    override fun listarAdvertencias(bot: Long, persona: String) = Wabot.listarAdvertencias(bot, persona)
    override fun listarPersonasAdvertidas(bot: Long) = Wabot.listarPersonasAdvertidas(bot)
    override fun quitarAdvertenciaPersona(bot: Long, persona: String, id: Long) = Wabot.quitarAdvertenciaPersona(bot, persona, id).toLong()
    override fun borrarAdvertenciasPersona(bot: Long, persona: String) = Wabot.borrarAdvertenciasPersona(bot, persona).toLong()
    override fun listarNotasPersona(bot: Long, persona: String) = Wabot.listarNotasPersona(bot, persona)
    override fun agregarNotaPersona(bot: Long, persona: String, texto: String) = Wabot.agregarNotaPersona(bot, persona, texto)
    override fun borrarNotaPersona(bot: Long, persona: String, id: Long) = Wabot.borrarNotaPersona(bot, persona, id)
    override fun olvidarPersona(bot: Long, persona: String) = Wabot.olvidarPersona(bot, persona)
    override fun exportarPersona(bot: Long, persona: String) = Wabot.exportarPersona(bot, persona)
    override fun marcarEstadoChat(bot: Long, chat: String, estado: String) = Wabot.marcarEstadoChat(bot, chat, estado)
    override fun listarChatsFiltrados(bot: Long, grupos: Boolean, consulta: String, estado: String, etiqueta: Long, limite: Long, desde: Long) =
        Wabot.listarChatsFiltrados(bot, grupos, consulta, estado, etiqueta, limite, desde)
    override fun contarPendientes(bot: Long) = Wabot.contarPendientes(bot).toLong()
    override fun listarEtiquetas(bot: Long) = Wabot.listarEtiquetas(bot)
    override fun crearEtiqueta(bot: Long, nombre: String, color: String) = Wabot.crearEtiqueta(bot, nombre, color)
    override fun editarEtiqueta(bot: Long, id: Long, nombre: String, color: String) = Wabot.editarEtiqueta(bot, id, nombre, color)
    override fun borrarEtiqueta(bot: Long, id: Long) = Wabot.borrarEtiqueta(bot, id)
    override fun ponerEtiquetaChat(bot: Long, chat: String, id: Long, si: Boolean) = Wabot.ponerEtiquetaChat(bot, chat, id, si)
    override fun etiquetasDeChat(bot: Long, chat: String) = Wabot.etiquetasDeChat(bot, chat)
    override fun crearEtiquetaEmoji(bot: Long, nombre: String, color: String, emoji: String) = Wabot.crearEtiquetaEmoji(bot, nombre, color, emoji)
    override fun editarEtiquetaEmoji(bot: Long, id: Long, nombre: String, color: String, emoji: String) = Wabot.editarEtiquetaEmoji(bot, id, nombre, color, emoji)
    override fun ponerEtiquetaPersona(bot: Long, persona: String, id: Long, si: Boolean) = Wabot.ponerEtiquetaPersona(bot, persona, id, si)
    override fun etiquetasDePersona(bot: Long, persona: String) = Wabot.etiquetasDePersona(bot, persona)
    override fun etiquetasDePersonas(bot: Long, personasJson: String) = Wabot.etiquetasDePersonas(bot, personasJson)
    override fun etiquetasDeChats(bot: Long, chatsJson: String) = Wabot.etiquetasDeChats(bot, chatsJson)
    override fun marcarMensaje(bot: Long, chat: String, msgId: String, etiqueta: Long, nota: String) = Wabot.marcarMensaje(bot, chat, msgId, etiqueta, nota)
    override fun quitarMarcaMensaje(bot: Long, chat: String, msgId: String, etiqueta: Long) = Wabot.quitarMarcaMensaje(bot, chat, msgId, etiqueta)
    override fun marcasDeMensaje(bot: Long, chat: String, msgId: String) = Wabot.marcasDeMensaje(bot, chat, msgId)
    override fun etiquetasDeMensajes(bot: Long, chat: String, idsJson: String) = Wabot.etiquetasDeMensajes(bot, chat, idsJson)
    override fun listarMarcados(bot: Long, tipo: String, etiqueta: Long, limite: Long, desde: Long) = Wabot.listarMarcados(bot, tipo, etiqueta, limite, desde)
    override fun contarMarcados(bot: Long, etiqueta: Long) = Wabot.contarMarcados(bot, etiqueta)
    override fun miembrosGrupoDetallado(bot: Long, grupo: String, incluirSalidos: Boolean) = Wabot.miembrosGrupoDetallado(bot, grupo, incluirSalidos)
    override fun cambiosMiembrosGrupo(bot: Long, grupo: String, antes: Long, limite: Long) = Wabot.cambiosMiembrosGrupo(bot, grupo, antes, limite)
    override fun actualizarMiembrosGrupo(bot: Long) = Wabot.actualizarMiembrosGrupo(bot)
    override fun gruposEnComunPersona(bot: Long, persona: String) = Wabot.gruposEnComunPersona(bot, persona)
    override fun pedirFotoPersona(bot: Long, persona: String, grupo: String, forzar: Boolean) = Wabot.pedirFotoPersona(bot, persona, grupo, forzar)
    override fun fotoPersona(bot: Long, persona: String) = Wabot.fotoPersona(bot, persona)
    override fun perfilPersona(bot: Long, persona: String) = Wabot.perfilPersona(bot, persona)
    override fun actividadPersona(bot: Long, persona: String) = Wabot.actividadPersona(bot, persona)
    override fun mensajesDePersona(bot: Long, filtroJson: String, antesTs: Long, antesId: String, limite: Long) = Wabot.mensajesDePersona(bot, filtroJson, antesTs, antesId, limite)
    override fun buscarEnChat(bot: Long, chat: String, palabra: String, desde: Long, hasta: Long, antesTs: Long, antesId: String, limite: Long) = Wabot.buscarEnChat(bot, chat, palabra, desde, hasta, antesTs, antesId, limite)
    override fun puntitosDeMensajes(bot: Long, chat: String, idsJson: String) = Wabot.puntitosDeMensajes(bot, chat, idsJson)
    override fun exportarChat(bot: Long, chat: String, formato: String, desde: Long, hasta: Long) = Wabot.exportarChat(bot, chat, formato, desde, hasta)
    override fun silenciarEnGrupos(bot: Long, persona: String, segundos: Long, alcance: String, gruposJson: String, avisar: Boolean, texto: String) = Wabot.silenciarEnGrupos(bot, persona, segundos, alcance, gruposJson, avisar, texto)
    override fun quitarSilencioGrupos(bot: Long, persona: String) = Wabot.quitarSilencioGrupos(bot, persona)
    override fun listarSilenciosGrupos(bot: Long) = Wabot.listarSilenciosGrupos(bot)
    override fun historialSilencios(bot: Long, persona: String) = Wabot.historialSilencios(bot, persona)
    override fun contadorSilencio(bot: Long, persona: String, grupo: String) = Wabot.contadorSilencio(bot, persona, grupo)
    override fun ponerContadorEnCero(bot: Long, persona: String, grupo: String) = Wabot.ponerContadorEnCero(bot, persona, grupo)
    override fun ajustesSilencio(bot: Long) = Wabot.ajustesSilencio(bot)
    override fun fijarAjusteSilencio(bot: Long, clave: String, valor: String) = Wabot.fijarAjusteSilencio(bot, clave, valor)
    override fun fijarTextoSilencio(bot: Long, id: String, texto: String) = Wabot.fijarTextoSilencio(bot, id, texto)
    override fun sancionesPendientes(bot: Long) = Wabot.sancionesPendientes(bot)
    override fun confirmarSancionSilencio(bot: Long, id: Long, si: Boolean) = Wabot.confirmarSancionSilencio(bot, id, si)
    override fun confirmarSancion(bot: Long, id: Long, si: Boolean) = Wabot.confirmarSancion(bot, id, si)
    override fun gruposQueAdministro(bot: Long) = Wabot.gruposQueAdministro(bot)
    override fun listarTiposAdvertencia(bot: Long) = Wabot.listarTiposAdvertencia(bot)
    override fun guardarTipoAdvertencia(bot: Long, tipoJson: String) = Wabot.guardarTipoAdvertencia(bot, tipoJson)
    override fun borrarTipoAdvertencia(bot: Long, id: Long) = Wabot.borrarTipoAdvertencia(bot, id)
    override fun listarEscalera(bot: Long, tipoId: Long) = Wabot.listarEscalera(bot, tipoId)
    override fun guardarEscalera(bot: Long, tipoId: Long, pasosJson: String) = Wabot.guardarEscalera(bot, tipoId, pasosJson)
    override fun advertirConTipo(bot: Long, persona: String, chat: String, msgId: String, motivo: String, tipoId: Long, reglaId: Long) = Wabot.advertirConTipo(bot, persona, chat, msgId, motivo, tipoId, reglaId)
    override fun listarReglasGrupo(bot: Long, grupo: String) = Wabot.listarReglasGrupo(bot, grupo)
    override fun guardarReglaItem(bot: Long, grupo: String, id: Long, titulo: String, texto: String, pos: Long) = Wabot.guardarReglaItem(bot, grupo, id, titulo, texto, pos)
    override fun borrarReglaItem(bot: Long, id: Long) = Wabot.borrarReglaItem(bot, id)
    override fun moverReglaItem(bot: Long, id: Long, delta: Long) = Wabot.moverReglaItem(bot, id, delta)
    override fun listarAlertasPalabra(bot: Long) = Wabot.listarAlertasPalabra(bot)
    override fun crearAlertaPalabra(bot: Long, palabra: String, chatsJson: String) = Wabot.crearAlertaPalabra(bot, palabra, chatsJson)
    override fun editarAlertaPalabra(bot: Long, id: Long, chatsJson: String, activa: Boolean) = Wabot.editarAlertaPalabra(bot, id, chatsJson, activa)
    override fun borrarAlertaPalabra(bot: Long, id: Long) = Wabot.borrarAlertaPalabra(bot, id)
    override fun listarRecordatorios(bot: Long, hechos: Boolean) = Wabot.listarRecordatorios(bot, hechos)
    override fun crearRecordatorio(bot: Long, texto: String, proximaTs: Long, repetir: String) = Wabot.crearRecordatorio(bot, texto, proximaTs, repetir)
    override fun editarRecordatorio(bot: Long, id: Long, texto: String, proximaTs: Long, repetir: String) = Wabot.editarRecordatorio(bot, id, texto, proximaTs, repetir)
    override fun borrarRecordatorio(bot: Long, id: Long) = Wabot.borrarRecordatorio(bot, id)
    override fun listarUtilidades(bot: Long, tipo: String) = Wabot.listarUtilidades(bot, tipo)
    override fun agregarUtilidad(bot: Long, tipo: String, texto: String) = Wabot.agregarUtilidad(bot, tipo, texto)
    override fun marcarUtilidad(bot: Long, id: Long, hecho: Boolean) = Wabot.marcarUtilidad(bot, id, hecho)
    override fun borrarUtilidad(bot: Long, id: Long) = Wabot.borrarUtilidad(bot, id)
    override fun limpiarUtilidadesHechas(bot: Long) = Wabot.limpiarUtilidadesHechas(bot)
    override fun estadoLab(bot: Long) = Wabot.estadoLab(bot)
    override fun aceptarAvisoLab(bot: Long) = Wabot.aceptarAvisoLab(bot)
    override fun fijarAjusteLab(bot: Long, clave: String, valor: String) = Wabot.fijarAjusteLab(bot, clave, valor)
    override fun enviarPruebaLab(bot: Long, tipo: String, variante: Long) = Wabot.enviarPruebaLab(bot, tipo, variante)
    override fun listarResultadosLab(bot: Long) = Wabot.listarResultadosLab(bot)
    override fun marcarResultadoLab(bot: Long, id: Long, campo: String, valor: Long) = Wabot.marcarResultadoLab(bot, id, campo, valor)
    override fun borrarResultadosLab(bot: Long) = Wabot.borrarResultadosLab(bot)
    override fun estadoCaptura(bot: Long) = Wabot.estadoCaptura(bot)
    override fun activarCaptura(bot: Long, activo: Boolean) = Wabot.activarCaptura(bot, activo)
    override fun mostrarTextosCaptura(bot: Long, mostrar: Boolean) = Wabot.mostrarTextosCaptura(bot, mostrar)
    override fun borrarCaptura(bot: Long) = Wabot.borrarCaptura(bot)
    override fun usarLimiteMedido(bot: Long) = Wabot.usarLimiteMedido(bot)
    override fun proximoAviso(bot: Long) = Wabot.proximoAviso(bot)
    override fun dispararAvisos(bot: Long) = Wabot.dispararAvisos(bot)
    override fun vistaPreviaResumen(bot: Long) = Wabot.vistaPreviaResumen(bot)
    override fun listarComandosPropios(bot: Long) = Wabot.listarComandosPropios(bot)
    override fun guardarComandoPropio(bot: Long, id: Long, datosJson: String) = Wabot.guardarComandoPropio(bot, id, datosJson)
    override fun vistaPreviaComandoPropio(bot: Long, datosJson: String) = Wabot.vistaPreviaComandoPropio(bot, datosJson)

    override fun listarAutos(bot: Long, tipo: String) = Wabot.listarAutos(bot, tipo)
    override fun obtenerAuto(bot: Long, id: Long) = Wabot.obtenerAuto(bot, id)
    override fun guardarAuto(bot: Long, entradaJson: String) = Wabot.guardarAuto(bot, entradaJson)
    override fun fijarAutoActivo(bot: Long, id: Long, activo: Boolean) = Wabot.fijarAutoActivo(bot, id, activo)
    override fun borrarAuto(bot: Long, id: Long) = Wabot.borrarAuto(bot, id)
    override fun reordenarAutos(bot: Long, idsJson: String) = Wabot.reordenarAutos(bot, idsJson)
    override fun plantillasAuto() = Wabot.plantillasAuto()
    override fun configPorDefectoAuto(tipo: String) = Wabot.configPorDefectoAuto(tipo)
    override fun probarAuto(bot: Long, entradaJson: String, escenarioJson: String) = Wabot.probarAuto(bot, entradaJson, escenarioJson)
    override fun registroAuto(bot: Long, autoId: Long, limite: Long) = Wabot.registroAuto(bot, autoId, limite)
    override fun subirAdjunto(bot: Long, ruta: String, tipo: String, mime: String, nombre: String) = Wabot.subirAdjunto(bot, ruta, tipo, mime, nombre)
    override fun borrarAdjunto(bot: Long, id: String) = Wabot.borrarAdjunto(bot, id)
    override fun limpiarAdjuntos(bot: Long) = Wabot.limpiarAdjuntos(bot)
    override fun listarAtajos(bot: Long, chat: String) = Wabot.listarAtajos(bot, chat)
    override fun enviarArchivosAtajo(bot: Long, chat: String, id: Long) = Wabot.enviarArchivosAtajo(bot, chat, id)
    override fun listarMensajesAuto(bot: Long, chat: String, antesTs: Long, limite: Long) = Wabot.listarMensajesAuto(bot, chat, antesTs, limite)
    override fun chatsConAutomaticos(bot: Long, dias: Long) = Wabot.chatsConAutomaticos(bot, dias)

    override fun grupoEstado(bot: Long, chat: String, refrescar: Boolean) = Wabot.grupoEstado(bot, chat, refrescar)
    override fun grupoInfo(bot: Long, chat: String) = Wabot.grupoInfo(bot, chat)
    override fun grupoMiembros(bot: Long, chat: String) = Wabot.grupoMiembros(bot, chat)
    override fun grupoCerrar(bot: Long, chat: String, cerrado: Boolean) = Wabot.grupoCerrar(bot, chat, cerrado)
    override fun grupoBloquearInfo(bot: Long, chat: String, bloqueada: Boolean) = Wabot.grupoBloquearInfo(bot, chat, bloqueada)
    override fun grupoExpulsar(bot: Long, chat: String, persona: String) = Wabot.grupoExpulsar(bot, chat, persona)
    override fun grupoAscender(bot: Long, chat: String, persona: String, admin: Boolean) = Wabot.grupoAscender(bot, chat, persona, admin)
    override fun grupoSolicitudes(bot: Long, chat: String) = Wabot.grupoSolicitudes(bot, chat)
    override fun grupoResolverSolicitudes(bot: Long, chat: String, personasJson: String, aprobar: Boolean) = Wabot.grupoResolverSolicitudes(bot, chat, personasJson, aprobar).toLong()
    override fun grupoRenombrar(bot: Long, chat: String, nombre: String) = Wabot.grupoRenombrar(bot, chat, nombre)
    override fun grupoDescripcion(bot: Long, chat: String, texto: String) = Wabot.grupoDescripcion(bot, chat, texto)
    override fun grupoEnlace(bot: Long, chat: String, reiniciar: Boolean) = Wabot.grupoEnlace(bot, chat, reiniciar)
    override fun grupoBorrarMensaje(bot: Long, chat: String, msgId: String) = Wabot.grupoBorrarMensaje(bot, chat, msgId)
    override fun grupoMencionarTodos(bot: Long, chat: String, texto: String) = Wabot.grupoMencionarTodos(bot, chat, texto)
    override fun grupoRanking(bot: Long, chat: String, dias: Long, limite: Long) = Wabot.grupoRanking(bot, chat, dias, limite)
    override fun grupoInactivos(bot: Long, chat: String, dias: Long) = Wabot.grupoInactivos(bot, chat, dias)
    override fun grupoReglas(bot: Long, chat: String) = Wabot.grupoReglas(bot, chat)
    override fun grupoGuardarReglas(bot: Long, chat: String, texto: String) = Wabot.grupoGuardarReglas(bot, chat, texto)
    override fun grupoFiltros(bot: Long, chat: String) = Wabot.grupoFiltros(bot, chat)
    override fun grupoGuardarFiltro(bot: Long, chat: String, datosJson: String) = Wabot.grupoGuardarFiltro(bot, chat, datosJson)
    override fun grupoRestaurarFiltro(bot: Long, chat: String, filtro: String) = Wabot.grupoRestaurarFiltro(bot, chat, filtro)
    override fun appStateDisponible(bot: Long) = Wabot.appStateDisponible(bot)
    override fun appStateEstado(bot: Long) = Wabot.appStateEstado(bot)
    override fun appStateSilenciar(bot: Long, chat: String, segundos: Long) = Wabot.appStateSilenciar(bot, chat, segundos)
    override fun appStateDesilenciar(bot: Long, chat: String) = Wabot.appStateDesilenciar(bot, chat)
    override fun appStateFijar(bot: Long, chat: String, fijar: Boolean) = Wabot.appStateFijar(bot, chat, fijar)
    override fun appStateArchivar(bot: Long, chat: String, archivar: Boolean) = Wabot.appStateArchivar(bot, chat, archivar)
    override fun appStateSincronizar(bot: Long) = Wabot.appStateSincronizar(bot)
}
