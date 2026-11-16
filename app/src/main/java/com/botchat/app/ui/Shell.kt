@file:OptIn(ExperimentalSharedTransitionApi::class)

package com.botchat.app.ui

import android.net.Uri
import androidx.compose.animation.AnimatedContentTransitionScope
import androidx.compose.animation.EnterTransition
import androidx.compose.animation.ExitTransition
import androidx.compose.animation.ExperimentalSharedTransitionApi
import androidx.compose.animation.SharedTransitionLayout
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Icon
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.navigation.compose.currentBackStackEntryAsState
import com.botchat.app.BotState
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import com.botchat.app.AppVm
import com.botchat.app.datos.Ajustes
import com.botchat.app.datos.FiltroLista
import com.botchat.app.datos.Repo
import kotlinx.coroutines.launch
import androidx.compose.runtime.rememberCoroutineScope
import com.botchat.app.datos.fijarTopeTotal
import com.botchat.app.datos.confirmarTope
import com.botchat.app.datos.asistenteEstado
import com.botchat.app.datos.asistenteVisto
import com.botchat.app.datos.estadoAlmacen
import com.botchat.app.datos.fijarAvisoAuditoriaVisto
import com.botchat.app.datos.fijarAuditoria
import com.botchat.app.datos.auditoriaGlobal

private fun enc(s: String) = Uri.encode(s)

/** Nombre del chat para las pantallas que lo muestran en la barra (visor, documentos y archivos compartidos). */
@Composable
private fun nombreDeChat(botId: Long, jid: String): String {
    var nombre by remember(jid) { mutableStateOf("") }
    LaunchedEffect(jid) { nombre = runCatching { Repo.chat(botId, jid)?.nombre }.getOrNull() ?: "" }
    return nombre
}

@Composable
private fun nombreDeAuto(botId: Long, id: Long): String {
    var nombre by remember(id) { mutableStateOf("") }
    LaunchedEffect(id) { nombre = runCatching { Repo.automatizacion(botId, id).def.nombre }.getOrDefault("") }
    return nombre
}

@Composable
private fun esGrupoDeChat(botId: Long, jid: String): Boolean {
    var grupo by remember(jid) { mutableStateOf(false) }
    LaunchedEffect(jid) { grupo = runCatching { Repo.chat(botId, jid)?.esGrupo }.getOrNull() ?: false }
    return grupo
}

private val RUTAS_PRINCIPALES = listOf("inicio", "grupos", "bot", "ajustes")

private data class Destino(val ruta: String, val titulo: String, val icono: androidx.compose.ui.graphics.vector.ImageVector)

/** Navegación de la app: barra superior con el estado del bot, navegación inferior (Chats, Espacio, Ajustes) y las pantallas internas. */
@OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)
@Composable
fun Shell(nav: NavHostController, vm: AppVm, ajustes: Ajustes, conPermiso: (() -> Unit) -> Unit) {
    val modo = LocalModoAnim.current
    val bot = vm.bot ?: return
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    val estiloLetra by ajustes.estiloLetra.collectAsState()
    // Una notificación de mensaje abre el chat correspondiente.
    val pedido by com.botchat.app.NavegarA.chat.collectAsState()
    LaunchedEffect(pedido) {
        pedido?.let { j -> com.botchat.app.NavegarA.chat.value = null; nav.navigate("chat/${enc(j)}") { launchSingleTop = true } }
    }
    // Una notificación de aviso (nueva solicitud) abre su pantalla.
    val rutaPedida by com.botchat.app.NavegarA.ruta.collectAsState()
    LaunchedEffect(rutaPedida) {
        rutaPedida?.let { r -> com.botchat.app.NavegarA.ruta.value = null; nav.navigate(r) { launchSingleTop = true } }
    }
    // Los "ver una vez" que quedaron a medias o caducaron se borran al abrir la app.
    LaunchedEffect(bot.id) { runCatching { Repo.barrerVerUnaVez(bot.id) } }
    // Las alarmas de recordatorios y del resumen diario se vuelven a programar al abrir la app.
    LaunchedEffect(bot.id) { runCatching { com.botchat.app.Alarmas.reprogramarAsync(ctx) }; runCatching { com.botchat.app.AlarmasModulos.reprogramarAsync(ctx) } }
    val entrada by nav.currentBackStackEntryAsState()
    val rutaActual = entrada?.destination?.route
    val principal = rutaActual in RUTAS_PRINCIPALES
    val estado by BotState.estado.collectAsState()
    val corriendo by BotState.corriendo.collectAsState()
    val destinos = remember { listOf(Destino("inicio", "Chats", Ico.Chats), Destino("grupos", "Grupos", Ico.Grupo), Destino("bot", "Bot", Ico.Bot), Destino("ajustes", "Tú", Ico.Persona)) }
    // Modo Simple / Completo: solo cambia lo que se muestra. Quien actualizó y tiene módulos encendidos pasa a Completo (se mira una sola vez).
    val completo by ajustes.modoCompleto.collectAsState()
    val avisoModo by ajustes.avisoModoPendiente.collectAsState()
    var modulosRevisados by remember { mutableStateOf(!ajustes.revisarModulosPendiente()) }
    LaunchedEffect(bot.id) {
        if (!modulosRevisados) {
            val hay = runCatching { Repo.modulos(bot.id).any { it.activo } }.getOrNull()
            if (hay == true) ajustes.fijarModoCompleto(true)
            if (hay != null) ajustes.modulosRevisados()
            modulosRevisados = true
        }
    }

    // 0.6.0 (E8): avisos de la primera vez al actualizar: privacidad/retención y tope de espacio. Van después del aviso del modo Simple.
    val alcance = rememberCoroutineScope()
    var auditoria by remember { mutableStateOf<com.botchat.app.datos.AuditoriaGlobal?>(null) }
    var estadoTope by remember { mutableStateOf<com.botchat.app.datos.EstadoAlmacen?>(null) }
    val topeVisto by ajustes.topeAvisoVisto.collectAsState()
    var ofrecerAsistente by remember { mutableStateOf(false) }
    LaunchedEffect(bot.id) { ofrecerAsistente = runCatching { !Repo.asistenteEstado(bot.id).visto }.getOrDefault(false) }
    LaunchedEffect(bot.id) {
        auditoria = runCatching { Repo.auditoriaGlobal(bot.id) }.getOrNull()
        // E9: si Go dejó el tope «pendiente» (actualización con más archivos que el tope), el aviso sale aunque ya se hubiera marcado
        // como visto en el teléfono, y Go no borra nada hasta que se confirme aquí.
        val e = runCatching { Repo.estadoAlmacen(bot.id) }.getOrNull()
        if (e != null && e.topePendiente) {
            if (!e.pasadoDelTope) { runCatching { Repo.confirmarTope(bot.id) }; ajustes.topeAvisoMarcado() } else estadoTope = e
        } else if (!topeVisto) {
            if (e != null && !e.pasadoDelTope) ajustes.topeAvisoMarcado() else estadoTope = e
        }
    }

    fun tipo(a: String?, b: String?): TipoTransicion = when {
        a in RUTAS_PRINCIPALES && b in RUTAS_PRINCIPALES -> TipoTransicion.PESTANA
        else -> TipoTransicion.EJE
    }
    fun entra(): AnimatedContentTransitionScope<NavBackStackEntry>.() -> EnterTransition = {
        Transiciones.entra(modo, tipo(initialState.destination.route, targetState.destination.route))
    }
    // El visor de fotos aparece con un fundido sobre el chat: el chat no se mueve de debajo.
    fun esVisor(r: String?) = r?.startsWith("visor/") == true
    fun sale(): AnimatedContentTransitionScope<NavBackStackEntry>.() -> ExitTransition = {
        if (esVisor(targetState.destination.route)) ExitTransition.None else Transiciones.sale(modo, tipo(initialState.destination.route, targetState.destination.route))
    }
    fun vuelveEntra(): AnimatedContentTransitionScope<NavBackStackEntry>.() -> EnterTransition = {
        if (esVisor(initialState.destination.route)) EnterTransition.None else Transiciones.vuelveEntra(modo, tipo(initialState.destination.route, targetState.destination.route))
    }
    fun vuelveSale(): AnimatedContentTransitionScope<NavBackStackEntry>.() -> ExitTransition = {
        Transiciones.vuelveSale(modo, tipo(initialState.destination.route, targetState.destination.route))
    }
    fun irA(ruta: String) {
        if (ruta == rutaActual) return
        haptica(Toque.Suave)
        nav.navigate(ruta) { popUpTo("inicio") { saveState = true }; launchSingleTop = true; restoreState = true }
    }

    // Las pestañas dibujan su barra dentro de su propia pantalla: barra y contenido entran y salen juntos, igual que al abrir un chat.
    val conBarra: @Composable (String, @Composable () -> Unit) -> Unit = { titulo, contenido ->
        androidx.compose.foundation.layout.Column(Modifier.fillMaxSize()) {
            BarraApp(estadoVisible(estado, corriendo, !bot.vinculado), { nav.navigate("estado_bot") }, titulo)
            Box(Modifier.weight(1f).fillMaxSize()) { contenido() }
        }
    }

    Scaffold(
        contentWindowInsets = androidx.compose.foundation.layout.WindowInsets(0, 0, 0, 0),
        bottomBar = {
            AnimatedVisibility(principal, enter = Transiciones.fundidoEntra(modo), exit = ExitTransition.None) {
                NavigationBar {
                    destinos.forEach { d ->
                        NavigationBarItem(
                            selected = rutaActual == d.ruta, onClick = { irA(d.ruta) },
                            icon = { Icon(d.icono, null) }, label = { Text(d.titulo) }, modifier = Modifier.testTag("nav_${d.ruta}"),
                        )
                    }
                }
            }
        },
    ) { relleno ->
    SharedTransitionLayout(Modifier.padding(relleno)) {
        CompositionLocalProvider(LocalSharedScope provides this, LocalEstiloLetra provides estiloLetra, LocalModoCompleto provides completo) {
            NavHost(nav, startDestination = "inicio", enterTransition = entra(), exitTransition = sale(), popEnterTransition = vuelveEntra(), popExitTransition = vuelveSale()) {
                composable("inicio") {
                    CompositionLocalProvider(LocalAnimScope provides this) {
                        PantallaInicio(
                            sinSesion = !bot.vinculado,
                            alAbrirChat = { nav.navigate("chat/${enc(it)}") },
                            alRevincular = { vm.revincular() },
                            estado = estadoVisible(estado, corriendo, !bot.vinculado),
                            alPulsarEstado = { nav.navigate("estado_bot") },
                            alNuevoChat = { nav.navigate("contactos") },
                            alMarcados = { nav.navigate("marcados") },
                            alAjustes = { irA("ajustes") },
                            alAbrirInfo = { c -> nav.navigate(if (c.esGrupo) "grupo_info/${enc(c.jid)}" else "persona_perfil/${enc(c.jid)}") },
                        )
                    }
                }
                composable("personas") { PantallaPersonas(bot.id, alAbrir = { nav.navigate("persona_perfil/${enc(it)}") }) }
                // Botón flotante de la lista: elegir con quién hablar (tocar a alguien abre su chat).
                composable("contactos") {
                    MarcoPantalla("Seleccionar contacto", { nav.popBackStack() }) {
                        PantallaPersonas(bot.id, alAbrir = { nav.navigate("chat/${enc(it)}") { popUpTo("contactos") { inclusive = true } } })
                    }
                }
                composable("grupos") { conBarra("Grupos") { PantallaGrupos(bot.id, alAbrir = { nav.navigate("grupo_info/${enc(it)}") }) } }
                composable("bot") {
                    conBarra("Bot") {
                    PantallaCentroBot(
                        alMiBot = { nav.navigate("estado_bot") }, alAlmacenamiento = { nav.navigate("espacio") }, alAuditoria = { nav.navigate("auditoria") },
                        alSilencios = { nav.navigate("silencios") }, alAdvertencias = { nav.navigate("advertencias") }, alEtiquetas = { nav.navigate("etiquetas") },
                        alAjustes = { nav.navigate("ajustes") }, alComandos = { nav.navigate("comandos_propios") }, alModulos = { nav.navigate("modulos") },
                        alLimites = { nav.navigate("iniciativa") }, alActividad = { nav.navigate("actividad") },
                        alAsistente = { nav.navigate("asistente_bot") }, alRespuestas = { nav.navigate("respuestas_bot") }, alAvisos = { nav.navigate("avisos_copia") }, alAvatar = { nav.navigate("avatar_bot") },
                    )
                    }
                }
                // E9: asistente «Crea tu bot» (5 pasos). Reutiliza el editor completo de menús y los mensajes automáticos para seguir editando.
                composable("asistente_bot") {
                    PantallaAsistenteBot(
                        bot.id, alSalir = { nav.popBackStack() },
                        alEditarMenu = { nav.navigate("automatizacion/$it") }, alEditarBienvenida = { nav.navigate("mensajes_auto") },
                    )
                }
                // E7: perfil de la persona (reemplaza a la ficha de siempre) e info del grupo con Miembros y Ajustes.
                composable("persona_perfil/{jid}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    PantallaPerfilPersona(
                        bot.id, jid, alVolver = { nav.popBackStack() },
                        alAbrirActividad = { persona, chat, nombre -> nav.navigate("actividad_persona/${enc(persona)}/${enc(chat)}/${enc(nombre)}") },
                        alVerMensajes = { nav.navigate("chat/${enc(it)}") },
                        alAbrirGrupo = { nav.navigate("grupo_info/${enc(it)}") },
                        alAbrirDetalle = { nav.navigate("persona/${enc(it)}") },
                        alAbrirEtiquetas = { nav.navigate("etiquetas") },
                        alOlvidada = { nav.popBackStack("inicio", false) },
                    )
                }
                composable("actividad_persona/{persona}/{chat}/{titulo}") { be ->
                    val persona = be.arguments?.getString("persona") ?: return@composable
                    val chat = be.arguments?.getString("chat") ?: return@composable
                    PantallaActividadPersona(
                        bot.id, persona, chat, be.arguments?.getString("titulo") ?: "", alVolver = { nav.popBackStack() },
                        alIrAlMensaje = { c, id -> nav.navigate("chat/${enc(c)}?ir=${enc(id)}") },
                    )
                }
                composable("grupo_info/{jid}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    PantallaInfoGrupo(
                        bot.id, jid, alVolver = { nav.popBackStack() },
                        alAbrirMiembro = { persona, nombre -> nav.navigate("actividad_persona/${enc(persona)}/${enc(jid)}/${enc(nombre)}") },
                        alVerCambios = { nav.navigate("grupo_cambios/${enc(jid)}") },
                        alAbrirAdministrar = { nav.navigate("admin_grupo/${enc(jid)}") },
                        alAbrirResumen = { nav.navigate("info/${enc(jid)}") },
                        alAbrirChat = { nav.navigate("chat/${enc(jid)}") },
                    )
                }
                composable("grupo_cambios/{jid}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    PantallaCambiosMiembros(bot.id, jid, alVolver = { nav.popBackStack() })
                }
                // E8: silencios, auditoría y privacidad, tipos de advertencia, reglas de los grupos y lo marcado.
                composable("silencios") { PantallaSilencios(bot.id, alVolver = { nav.popBackStack() }, alAjustes = { nav.navigate("silencios_ajustes") }, alAbrirPersona = { nav.navigate("persona_perfil/${enc(it)}") }) }
                composable("silencios_ajustes") { PantallaAjustesSilencio(bot.id, alVolver = { nav.popBackStack() }, ajustesApp = ajustes) }
                composable("auditoria") { PantallaAuditoria(bot.id, alVolver = { nav.popBackStack() }, alPrivacidad = { nav.navigate("privacidad") }) }
                composable("advertencias_tipos") { PantallaTiposAdvertencia(bot.id, alVolver = { nav.popBackStack() }, alEditar = { nav.navigate("adv_tipo/$it") }) }
                composable("adv_tipo/{id}") { be ->
                    val id = be.arguments?.getString("id")?.toLongOrNull() ?: return@composable
                    EditorTipoAdvertencia(bot.id, id, alVolver = { nav.popBackStack() })
                }
                composable("reglas_grupos") { PantallaReglasGrupos(bot.id, alVolver = { nav.popBackStack() }, alAbrir = { g, n -> nav.navigate("reglas/${enc(g.ifEmpty { "_" })}/${enc(n)}") }) }
                composable("reglas/{grupo}/{nombre}") { be ->
                    val g = be.arguments?.getString("grupo")?.takeIf { it != "_" } ?: ""
                    PantallaReglasGrupo(bot.id, g, be.arguments?.getString("nombre") ?: "Reglas", alVolver = { nav.popBackStack() })
                }
                composable("marcados") {
                    PantallaMarcados(
                        bot.id, alVolver = { nav.popBackStack() }, alAbrirChat = { nav.navigate("chat/${enc(it)}") },
                        alIrAlMensaje = { c, id -> nav.navigate("chat/${enc(c)}?ir=${enc(id)}") }, alAbrirPersona = { nav.navigate("persona_perfil/${enc(it)}") },
                        alAbrirGrupo = { nav.navigate("grupo_info/${enc(it)}") }, alEtiquetas = { nav.navigate("etiquetas") },
                    )
                }
                composable("chat/{jid}?ir={ir}", arguments = listOf(androidx.navigation.navArgument("ir") { type = androidx.navigation.NavType.StringType; defaultValue = "" })) { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    val ir = be.arguments?.getString("ir") ?: ""
                    CompositionLocalProvider(LocalAnimScope provides this) {
                      Box(Modifier.fillMaxSize().compartido("fila-$jid", limites = true)) {
                        PantallaChat(
                            jid, alVolver = { nav.popBackStack() },
                            alAbrirVisor = { id -> nav.navigate("visor/${enc(jid)}/${enc(id)}") },
                            alAbrirDocumento = { id -> nav.navigate("documento/${enc(jid)}/${enc(id)}") },
                            alAbrirArchivos = { nav.navigate("archivos/${enc(jid)}") },
                            alAbrirInfo = { nav.navigate("info/${enc(jid)}") },
                            alVerAutomaticos = { nav.navigate("automaticos_chat/${enc(jid)}") },
                            irA = ir,
                            alAbrirUnaVez = { id -> nav.navigate("unavez/${enc(jid)}/${enc(id)}") },
                            alAbrirEtiquetas = { nav.navigate("etiquetas") },
                        )
                      }
                    }
                }
                composable("visor/{jid}/{id}", enterTransition = { Transiciones.visorEntra(modo) }, exitTransition = { ExitTransition.None },
                    popExitTransition = { Transiciones.visorSale(modo) }) { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    val id = be.arguments?.getString("id") ?: return@composable
                    PantallaVisor(bot.id, jid, id, nombreDeChat(bot.id, jid), alCerrar = { nav.popBackStack() }, alIrAlMensaje = { m -> nav.navigate("chat/${enc(jid)}?ir=${enc(m)}") })
                }
                composable("documento/{jid}/{id}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    val id = be.arguments?.getString("id") ?: return@composable
                    PantallaDocumento(bot.id, jid, id, nombreDeChat(bot.id, jid), alVolver = { nav.popBackStack() })
                }
                composable("unavez/{jid}/{id}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    val id = be.arguments?.getString("id") ?: return@composable
                    PantallaVerUnaVez(bot.id, jid, id, alCerrar = { nav.popBackStack() })
                }
                composable("info/{jid}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    PantallaInfoChat(
                        bot.id, jid, alVolver = { nav.popBackStack() },
                        alAbrirVisor = { id -> nav.navigate("visor/${enc(jid)}/${enc(id)}") },
                        alAbrirDocumento = { id -> nav.navigate("documento/${enc(jid)}/${enc(id)}") },
                        alAbrirLista = { f -> nav.navigate("lista/${enc(jid)}/${enc(f.json())}") },
                        alAbrirPersona = { nav.navigate("persona/${enc(jid)}") },
                        alOlvidada = { nav.popBackStack("inicio", false) },
                        alAbrirGrupo = { nav.navigate("admin_grupo/${enc(jid)}") },
                    )
                }
                composable("avatar_bot") { PantallaAvatar(bot.id, alVolver = { nav.popBackStack() }) }
                composable("avisos_copia") { PantallaAvisosCopia(bot.id, alVolver = { nav.popBackStack() }) }
                composable("respuestas_bot") { PantallaRespuestasBot(bot.id, alVolver = { nav.popBackStack() }) }
                composable("admin_grupo/{jid}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    PantallaAdminGrupo(
                        bot.id, jid,
                        SalidasAdminGrupo(
                            alVolver = { nav.popBackStack() },
                            alAbrirPerfil = { persona -> nav.navigate("admin_perfil/${enc(jid)}/${enc(persona)}") },
                            alAbrirBaneados = { nav.navigate("admin_baneados/${enc(jid)}") },
                            alAbrirSolicitudes = { nav.navigate("grupo_miembros/${enc(jid)}/solicitudes") },
                            alAbrirFiltros = { nav.navigate("grupo_filtros/${enc(jid)}") },
                            alAbrirReglas = { nombre -> nav.navigate("reglas/${enc(jid)}/${enc(nombre.ifBlank { "Grupo" })}") },
                            alAbrirAjustesAvanzados = { nav.navigate("grupo/${enc(jid)}") },
                            alAbrirActividad = { nav.navigate("grupo_actividad/${enc(jid)}") },
                            alAbrirAdvertencias = { nav.navigate("advertencias") },
                            alAbrirSilencios = { nav.navigate("silencios") },
                            alAbrirBienvenida = { nav.navigate("mensajes_auto") },
                            alAbrirModulos = { nav.navigate("modulos") },
                        ),
                    )
                }
                composable("admin_perfil/{jid}/{persona}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    val persona = be.arguments?.getString("persona") ?: return@composable
                    PantallaPerfilMiembro(bot.id, jid, persona, alVolver = { nav.popBackStack() },
                        alAuditar = { nombre -> nav.navigate("actividad_persona/${enc(persona)}/${enc(jid)}/${enc(nombre)}") })
                }
                composable("admin_baneados/{jid}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    PantallaBaneadosGrupo(bot.id, jid, alVolver = { nav.popBackStack() })
                }
                composable("grupo/{jid}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    PantallaGrupo(bot.id, jid, alVolver = { nav.popBackStack() }, alAbrirMiembros = { nav.navigate("grupo_miembros/${enc(jid)}/$it") },
                        alAbrirFiltros = { nav.navigate("grupo_filtros/${enc(jid)}") }, alAbrirActividad = { nav.navigate("grupo_actividad/${enc(jid)}") })
                }
                composable("grupo_miembros/{jid}/{pestana}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    PantallaMiembrosGrupo(bot.id, jid, be.arguments?.getString("pestana") ?: "miembros", alVolver = { nav.popBackStack() })
                }
                composable("grupo_filtros/{jid}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    PantallaFiltrosGrupo(bot.id, jid, alVolver = { nav.popBackStack() })
                }
                composable("grupo_actividad/{jid}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    PantallaActividadGrupo(bot.id, jid, alVolver = { nav.popBackStack() })
                }
                composable("lista/{jid}/{filtro}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    val filtro = FiltroLista.desde(be.arguments?.getString("filtro") ?: "")
                    val esGrupo = esGrupoDeChat(bot.id, jid)
                    PantallaListaFiltrada(
                        bot.id, jid, filtro, esGrupo, alVolver = { nav.popBackStack() },
                        alAbrirVisor = { id -> nav.navigate("visor/${enc(jid)}/${enc(id)}") },
                        alAbrirDocumento = { id -> nav.navigate("documento/${enc(jid)}/${enc(id)}") },
                        alIrAlMensaje = { id -> nav.navigate("chat/${enc(jid)}?ir=${enc(id)}") },
                    )
                }
                composable("estadisticas") { PantallaEstadisticas(bot.id, alVolver = { nav.popBackStack() }, alAbrirChat = { nav.navigate("chat/${enc(it)}") }) }
                composable("archivos/{jid}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    PantallaCompartidos(
                        bot.id, jid, nombreDeChat(bot.id, jid), alVolver = { nav.popBackStack() },
                        alAbrirVisor = { id -> nav.navigate("visor/${enc(jid)}/${enc(id)}") },
                        alAbrirDocumento = { id -> nav.navigate("documento/${enc(jid)}/${enc(id)}") },
                    )
                }
                composable("espacio") { PantallaEspacio(alVolver = { nav.popBackStack() }, alAbrirChat = { nav.navigate("chat/${enc(it)}") }, ajustes = ajustes) }
                composable("ajustes") {
                    var cuenta by remember { mutableStateOf(0) }
                    LaunchedEffect(bot.id) {
                        cuenta = runCatching { Repo.contarSolicitudes(bot.id) }.getOrDefault(0)
                        Repo.cambios.collect { evs -> if ("solicitudes" in evs) cuenta = runCatching { Repo.contarSolicitudes(bot.id) }.getOrDefault(0) }
                    }
                    conBarra("Ajustes") {
                    PantallaAjustes(ajustes, alPermisos = { nav.navigate("permisos") }, pendientesSolicitudes = cuenta, alSeguridad = { nav.navigate("seguridad") }, alDescargas = { nav.navigate("descargas") }, alBot = { nav.navigate("estado_bot") }, alAvanzado = { nav.navigate("avanzado") }, alEstadisticas = { nav.navigate("estadisticas") }, alPrivacidad = { nav.navigate("privacidad") }, alAutomatizaciones = { nav.navigate("automatizaciones") }, alModulos = { nav.navigate("modulos") }, alIniciativa = { nav.navigate("iniciativa") },
                        alAvisos = { nav.navigate("avisos") }, alComandosPropios = { nav.navigate("comandos_propios") }, alEtiquetas = { nav.navigate("etiquetas") }, alAdvertencias = { nav.navigate("advertencias") }, alUtilidades = { nav.navigate("utilidades") })
                    }
                }
                composable("permisos") {
                    PantallaPermisos(bot.id, alVolver = { nav.popBackStack() }, alRangos = { nav.navigate("rangos") }, alSolicitudes = { nav.navigate("solicitudes") }, alGrupos = { nav.navigate("politicas_grupos") })
                }
                composable("rangos") { PantallaRangos(bot.id, alVolver = { nav.popBackStack() }, alEditar = { nav.navigate("rango/$it") }) }
                composable("rango/{id}") { be ->
                    val id = be.arguments?.getString("id")?.toLongOrNull() ?: return@composable
                    EditorRango(bot.id, id, alVolver = { nav.popBackStack() })
                }
                composable("solicitudes") { PantallaSolicitudes(bot.id, alVolver = { nav.popBackStack() }) }
                composable("politicas_grupos") { PantallaPoliticasGrupo(bot.id, alVolver = { nav.popBackStack() }) }
                composable("permisos_avanzado") { PantallaPermisosAvanzado(bot.id, alVolver = { nav.popBackStack() }, alRegistro = { nav.navigate("registro_acciones") }) }
                composable("registro_acciones") { PantallaRegistroAcciones(bot.id, alVolver = { nav.popBackStack() }) }
                composable("utilidades") { PantallaUtilidades(bot.id, alVolver = { nav.popBackStack() }) }
                composable("avisos") { PantallaAvisos(bot.id, alVolver = { nav.popBackStack() }, alRecordatorio = { nav.navigate("recordatorio/$it") }) }
                composable("recordatorio/{id}") { be ->
                    val id = be.arguments?.getString("id")?.toLongOrNull() ?: return@composable
                    EditorRecordatorio(bot.id, id, alVolver = { nav.popBackStack() })
                }
                composable("comandos_propios") { PantallaComandosPropios(bot.id, alVolver = { nav.popBackStack() }, alEditar = { nav.navigate("comando_propio/$it") }) }
                composable("comando_propio/{id}") { be ->
                    val id = be.arguments?.getString("id")?.toLongOrNull() ?: return@composable
                    EditorComandoPropio(bot.id, id, alVolver = { nav.popBackStack() })
                }
                composable("etiquetas") { PantallaEtiquetas(bot.id, alVolver = { nav.popBackStack() }, alMarcados = { nav.navigate("marcados") }) }
                composable("advertencias") { PantallaAdvertencias(bot.id, alVolver = { nav.popBackStack() }, alAbrirPersona = { nav.navigate("persona/${enc(it)}") }, alTipos = { nav.navigate("advertencias_tipos") }, alReglas = { nav.navigate("reglas_grupos") }) }
                composable("persona/{jid}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    PantallaPersonaDetalle(bot.id, jid, alVolver = { nav.popBackStack() })
                }
                composable("modulos") {
                    PantallaModulos(
                        bot.id, alVolver = { nav.popBackStack() }, alConfigurar = { nav.navigate("modulo_config/${enc(it)}") },
                        alIniciativa = { nav.navigate("iniciativa") },
                    )
                }
                composable("modulo_config/{id}") { be ->
                    val id = be.arguments?.getString("id") ?: return@composable
                    PantallaConfigModulo(bot.id, id, alVolver = { nav.popBackStack() }, alAbrirHistorial = { nav.navigate("descargas_video") })
                }
                composable("descargas_video") { PantallaDescargasVideo(bot.id, alVolver = { nav.popBackStack() }) }
                composable("iniciativa") { PantallaIniciativa(bot.id, alVolver = { nav.popBackStack() }, alRegistro = { nav.navigate("registro_iniciativas") }) }
                composable("registro_iniciativas") { PantallaRegistroIniciativas(bot.id, alVolver = { nav.popBackStack() }) }
                composable("privacidad") { PantallaPrivacidad(ajustes, alVolver = { nav.popBackStack() }) }
                composable("automatizaciones") {
                    PantallaAutomatizaciones(
                        bot.id, alVolver = { nav.popBackStack() }, alEditar = { nav.navigate("automatizacion/$it") }, alEnCurso = { nav.navigate("en_curso") },
                        alActividad = { nav.navigate("actividad") }, alEstadisticas = { nav.navigate("estadisticas_menu/$it") },
                        alFormularios = { nav.navigate("formularios/0") }, alNoEntendio = { nav.navigate("no_entendio/0") }, alAjustesMenus = { nav.navigate("ajustes_menus") },
                        alMensajesAuto = { nav.navigate("mensajes_auto") },
                    )
                }
                composable("mensajes_auto") {
                    PantallaMensajesAuto(bot.id, alVolver = { nav.popBackStack() }, alEditar = { id, tipo -> nav.navigate("mensaje_auto/$id/$tipo") }, alRegistro = { nav.navigate("registro_auto") },
                        alVerAutomaticos = { nav.navigate("mensajes_automaticos_enviados") })
                }
                composable("mensaje_auto/{id}/{tipo}") { be ->
                    val id = be.arguments?.getString("id")?.toLongOrNull() ?: return@composable
                    val tipo = be.arguments?.getString("tipo") ?: return@composable
                    EditorMensajeAuto(bot.id, id, tipo, alVolver = { nav.popBackStack() })
                }
                composable("automaticos_chat/{jid}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    PantallaMensajesAutomaticos(bot.id, jid, "Mensajes automáticos de este chat", alVolver = { nav.popBackStack() }, alAbrirChat = { nav.popBackStack() })
                }
                composable("registro_auto") { PantallaRegistroAuto(bot.id, alVolver = { nav.popBackStack() }) }
                composable("mensajes_automaticos_enviados") { PantallaMensajesAutomaticos(bot.id, "", "Mensajes automáticos enviados", alVolver = { nav.popBackStack() }, alAbrirChat = { nav.navigate("chat/${enc(it)}") }) }
                composable("automatizacion/{id}") { be ->
                    val id = be.arguments?.getString("id")?.toLongOrNull() ?: return@composable
                    EditorAutomatizacion(bot.id, id, alVolver = { nav.popBackStack() })
                }
                composable("en_curso") { PantallaEnCurso(bot.id, alVolver = { nav.popBackStack() }, alAbrir = { nav.navigate("recorrido/$it") }) }
                composable("recorrido/{id}") { be ->
                    val id = be.arguments?.getString("id")?.toLongOrNull() ?: return@composable
                    PantallaRecorrido(bot.id, id, alVolver = { nav.popBackStack() })
                }
                composable("estadisticas_menu/{id}") { be ->
                    val id = be.arguments?.getString("id")?.toLongOrNull() ?: return@composable
                    PantallaEstadisticasMenu(bot.id, id, nombreDeAuto(bot.id, id), alVolver = { nav.popBackStack() },
                        alFormularios = { nav.navigate("formularios/$it") }, alNoEntendio = { nav.navigate("no_entendio/$it") })
                }
                composable("formularios/{menu}") { be ->
                    val menu = be.arguments?.getString("menu")?.toLongOrNull() ?: 0L
                    PantallaFormularios(bot.id, menu, if (menu == 0L) "Formularios recibidos" else "Formularios de " + nombreDeAuto(bot.id, menu), alVolver = { nav.popBackStack() }, alAbrir = { nav.navigate("formulario/$it") })
                }
                composable("formulario/{id}") { be ->
                    val id = be.arguments?.getString("id")?.toLongOrNull() ?: return@composable
                    PantallaFormulario(bot.id, id, alVolver = { nav.popBackStack() }, alAbrirChat = { nav.navigate("chat/${enc(it)}") })
                }
                composable("no_entendio/{menu}") { be ->
                    val menu = be.arguments?.getString("menu")?.toLongOrNull() ?: 0L
                    PantallaNoEntendio(bot.id, menu, if (menu == 0L) "Lo que no entendió" else "No entendió: " + nombreDeAuto(bot.id, menu), alVolver = { nav.popBackStack() })
                }
                composable("ajustes_menus") { PantallaAjustesMenus(bot.id, alVolver = { nav.popBackStack() }) }
                composable("actividad") { PantallaActividad(bot.id, alVolver = { nav.popBackStack() }) }
                composable("funciones") { PantallaFunciones(ajustes, alVolver = { nav.popBackStack() }) }
                composable("seguridad") { PantallaSeguridad(ajustes, alVolver = { nav.popBackStack() }) }
                composable("descargas") { PantallaDescargas(ajustes, alVolver = { nav.popBackStack() }) }
                composable("avanzado") {
                    PantallaAvanzado(
                        ajustes, bot.nombre, vm.eliminando, alVolver = { nav.popBackStack() },
                        alAbrirEspacio = { nav.navigate("espacio") },
                        alFunciones = { nav.navigate("funciones") }, alActividad = { nav.navigate("actividad") }, alEliminarBot = { vm.eliminar(ctx) },
                        alPermisosAvanzado = { nav.navigate("permisos_avanzado") }, alMenus = { nav.navigate("ajustes_menus") }, botId = bot.id,
                    )
                }
                composable("estado_bot") { PantallaBot(vm, conPermiso, alVolver = { nav.popBackStack() }, alAbrirAvanzado = { nav.navigate("avanzado") }, alAbrirAyuda = { nav.navigate("ayuda") }) }
                composable("ayuda") { AyudaBateria(alVolver = { nav.popBackStack() }) }
            }
        }
    }
    }
    DialogoConfirmacion(bot.id)
    val esperaModo = avisoModo && modulosRevisados
    auditoria?.let { a ->
        if (!esperaModo && (!a.avisoVisto || a.retPendiente)) AvisoAuditoria(
            a.retPendiente,
            alElegir = { d -> alcance.launch { runCatching { Repo.fijarAuditoria(bot.id, a.defecto, d); Repo.fijarAvisoAuditoriaVisto(bot.id, true) }; auditoria = runCatching { Repo.auditoriaGlobal(bot.id) }.getOrNull() ?: a.copy(avisoVisto = true, retPendiente = false) } },
            alEntendido = { alcance.launch { runCatching { Repo.fijarAvisoAuditoriaVisto(bot.id, true) }; auditoria = a.copy(avisoVisto = true) } },
        )
    }
    estadoTope?.let { t ->
        val auditoriaLista = auditoria?.let { it.avisoVisto && !it.retPendiente } ?: true
        if (!esperaModo && auditoriaLista && (!topeVisto || t.topePendiente)) {
            if (t.pasadoDelTope) AvisoTopeAlmacen(
                t,
                alSubir = { bytes -> alcance.launch { runCatching { Repo.fijarTopeTotal(bot.id, bytes) }; runCatching { Repo.confirmarTope(bot.id) }; ajustes.topeAvisoMarcado(); estadoTope = null } },
                alEntendido = { alcance.launch { runCatching { Repo.confirmarTope(bot.id) }; ajustes.topeAvisoMarcado(); estadoTope = null } },
            )
        }
    }
    // E9: en una instalación nueva se ofrece una vez el asistente «Crea tu bot», después de los demás avisos.
    val auditoriaOk = auditoria?.let { it.avisoVisto && !it.retPendiente } ?: true
    if (ofrecerAsistente && !avisoModo && modulosRevisados && estadoTope == null && auditoriaOk) AvisoAsistenteBot(
        alEmpezar = { ofrecerAsistente = false; alcance.launch { runCatching { Repo.asistenteVisto(bot.id) } }; nav.navigate("asistente_bot") },
        alDespues = { ofrecerAsistente = false; alcance.launch { runCatching { Repo.asistenteVisto(bot.id) } } },
    )
    if (avisoModo && modulosRevisados) AvisoModoSimple(completo, alCompleto = { ajustes.fijarModoCompleto(true); ajustes.avisoModoVisto() }, alSimple = { ajustes.fijarModoCompleto(false); ajustes.avisoModoVisto() })
}
