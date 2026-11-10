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
    fun estadoDelChat(bot: Long, chat: String): String
    fun plazoEdicionSeg(): Long
    fun configurarEnvio(pausaBotMin: Long)
    fun enviarMedio(bot: Long, chat: String, tipo: String, ruta: String, mime: String, nombre: String, pie: String, citaId: String, ancho: Long, alto: Long, durS: Long, notaVoz: Boolean, miniatura: ByteArray?, onda: ByteArray?): String
    fun abrirVerUnaVez(bot: Long, chat: String, id: String): String
    fun cerrarVerUnaVez(bot: Long, chat: String, id: String)
    fun barrerVerUnaVez(bot: Long): Long
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
}

/** El módulo Go de verdad (whatsmeow + almacén cifrado). */
object MotorGo : Motor {
    override fun abrirAlmacen(raiz: String, clave: ByteArray, o: Observador?) = Wabot.abrirAlmacen(raiz, clave, o)
    override fun listarChats(bot: Long, grupos: Boolean, consulta: String, limite: Long, desde: Long) = Wabot.listarChats(bot, grupos, consulta, limite, desde)
    override fun obtenerChat(bot: Long, chat: String) = Wabot.obtenerChat(bot, chat)
    override fun listarMensajes(bot: Long, chat: String, antesTs: Long, antesId: String, limite: Long) = Wabot.listarMensajes(bot, chat, antesTs, antesId, limite)
    override fun mensajesPorId(bot: Long, chat: String, idsJson: String) = Wabot.mensajesPorId(bot, chat, idsJson)
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
    override fun estadoDelChat(bot: Long, chat: String) = Wabot.estadoDelChat(bot, chat)
    override fun plazoEdicionSeg() = Wabot.plazoEdicionSeg().toLong()
    override fun configurarEnvio(pausaBotMin: Long) = Wabot.configurarEnvio(pausaBotMin)
    override fun enviarMedio(bot: Long, chat: String, tipo: String, ruta: String, mime: String, nombre: String, pie: String, citaId: String, ancho: Long, alto: Long, durS: Long, notaVoz: Boolean, miniatura: ByteArray?, onda: ByteArray?) =
        Wabot.enviarMedio(bot, chat, tipo, ruta, mime, nombre, pie, citaId, ancho, alto, durS, notaVoz, miniatura, onda)
    override fun abrirVerUnaVez(bot: Long, chat: String, id: String) = Wabot.abrirVerUnaVez(bot, chat, id)
    override fun cerrarVerUnaVez(bot: Long, chat: String, id: String) = Wabot.cerrarVerUnaVez(bot, chat, id)
    override fun barrerVerUnaVez(bot: Long) = Wabot.barrerVerUnaVez(bot).toLong()
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
}
