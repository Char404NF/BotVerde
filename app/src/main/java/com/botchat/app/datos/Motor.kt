package com.botchat.app.datos

import wabot.Observador
import wabot.Wabot

/** Lo que la app le pide al módulo Go. Existe para poder probar las pantallas con datos de ejemplo sin cargar la librería nativa. */
interface Motor {
    fun abrirAlmacen(raiz: String, clave: ByteArray, o: Observador?)
    fun listarChats(bot: Long, grupos: Boolean, consulta: String, limite: Long, desde: Long): String
    fun obtenerChat(bot: Long, chat: String): String
    fun listarMensajes(bot: Long, chat: String, antesTs: Long, antesId: String, limite: Long): String
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
}

/** El módulo Go de verdad (whatsmeow + almacén cifrado). */
object MotorGo : Motor {
    override fun abrirAlmacen(raiz: String, clave: ByteArray, o: Observador?) = Wabot.abrirAlmacen(raiz, clave, o)
    override fun listarChats(bot: Long, grupos: Boolean, consulta: String, limite: Long, desde: Long) = Wabot.listarChats(bot, grupos, consulta, limite, desde)
    override fun obtenerChat(bot: Long, chat: String) = Wabot.obtenerChat(bot, chat)
    override fun listarMensajes(bot: Long, chat: String, antesTs: Long, antesId: String, limite: Long) = Wabot.listarMensajes(bot, chat, antesTs, antesId, limite)
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
}
