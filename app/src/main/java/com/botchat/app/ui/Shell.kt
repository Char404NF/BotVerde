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
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import com.botchat.app.AppVm
import com.botchat.app.datos.Ajustes
import com.botchat.app.datos.FiltroLista
import com.botchat.app.datos.Repo

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

private val RUTAS_PRINCIPALES = listOf("inicio", "espacio", "ajustes")

private data class Destino(val ruta: String, val titulo: String, val icono: androidx.compose.ui.graphics.vector.ImageVector)

/** Navegación de la app: barra superior con el estado del bot, navegación inferior (Chats, Espacio, Ajustes) y las pantallas internas. */
@OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)
@Composable
fun Shell(nav: NavHostController, vm: AppVm, ajustes: Ajustes, conPermiso: (() -> Unit) -> Unit) {
    val modo = LocalModoAnim.current
    val bot = vm.bot ?: return
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
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
    val entrada by nav.currentBackStackEntryAsState()
    val rutaActual = entrada?.destination?.route
    val principal = rutaActual in RUTAS_PRINCIPALES
    val estado by BotState.estado.collectAsState()
    val corriendo by BotState.corriendo.collectAsState()
    val destinos = remember { listOf(Destino("inicio", "Chats", Ico.Chats), Destino("espacio", "Espacio", Ico.Espacio), Destino("ajustes", "Ajustes", Ico.Ajustes)) }

    fun entre(a: String?, b: String?) = a in RUTAS_PRINCIPALES && b in RUTAS_PRINCIPALES
    fun entra(): AnimatedContentTransitionScope<NavBackStackEntry>.() -> EnterTransition = {
        Transiciones.entra(modo, entre(initialState.destination.route, targetState.destination.route))
    }
    // El visor de fotos aparece con un fundido sobre el chat: el chat no se mueve de debajo.
    fun esVisor(r: String?) = r?.startsWith("visor/") == true
    fun sale(): AnimatedContentTransitionScope<NavBackStackEntry>.() -> ExitTransition = {
        if (esVisor(targetState.destination.route)) ExitTransition.None else Transiciones.sale(modo, entre(initialState.destination.route, targetState.destination.route))
    }
    fun vuelveEntra(): AnimatedContentTransitionScope<NavBackStackEntry>.() -> EnterTransition = {
        if (esVisor(initialState.destination.route)) EnterTransition.None else Transiciones.vuelveEntra(modo, entre(initialState.destination.route, targetState.destination.route))
    }
    fun vuelveSale(): AnimatedContentTransitionScope<NavBackStackEntry>.() -> ExitTransition = {
        Transiciones.vuelveSale(modo, entre(initialState.destination.route, targetState.destination.route))
    }
    fun irA(ruta: String) {
        if (ruta == rutaActual) return
        haptica(Toque.Suave)
        nav.navigate(ruta) { popUpTo("inicio") { saveState = true }; launchSingleTop = true; restoreState = true }
    }

    Scaffold(
        contentWindowInsets = androidx.compose.foundation.layout.WindowInsets(0, 0, 0, 0),
        topBar = {
            AnimatedVisibility(principal, enter = Transiciones.fundidoEntra(modo), exit = Transiciones.fundidoSale(modo)) {
                BarraApp(estadoVisible(estado, corriendo, !bot.vinculado)) { nav.navigate("bot") }
            }
        },
        bottomBar = {
            AnimatedVisibility(principal, enter = Transiciones.fundidoEntra(modo), exit = Transiciones.fundidoSale(modo)) {
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
        CompositionLocalProvider(LocalSharedScope provides this) {
            NavHost(nav, startDestination = "inicio", enterTransition = entra(), exitTransition = sale(), popEnterTransition = vuelveEntra(), popExitTransition = vuelveSale()) {
                composable("inicio") {
                    CompositionLocalProvider(LocalAnimScope provides this) {
                        PantallaInicio(
                            sinSesion = !bot.vinculado,
                            alAbrirChat = { nav.navigate("chat/${enc(it)}") },
                            alRevincular = { vm.revincular() },
                        )
                    }
                }
                composable("chat/{jid}?ir={ir}", arguments = listOf(androidx.navigation.navArgument("ir") { type = androidx.navigation.NavType.StringType; defaultValue = "" })) { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    val ir = be.arguments?.getString("ir") ?: ""
                    CompositionLocalProvider(LocalAnimScope provides this) {
                        PantallaChat(
                            jid, alVolver = { nav.popBackStack() },
                            alAbrirVisor = { id -> nav.navigate("visor/${enc(jid)}/${enc(id)}") },
                            alAbrirDocumento = { id -> nav.navigate("documento/${enc(jid)}/${enc(id)}") },
                            alAbrirArchivos = { nav.navigate("archivos/${enc(jid)}") },
                            alAbrirInfo = { nav.navigate("info/${enc(jid)}") },
                            irA = ir,
                            alAbrirUnaVez = { id -> nav.navigate("unavez/${enc(jid)}/${enc(id)}") },
                        )
                    }
                }
                composable("visor/{jid}/{id}", enterTransition = { Transiciones.fundidoEntra(modo) }, exitTransition = { ExitTransition.None },
                    popExitTransition = { Transiciones.fundidoSale(modo) }) { be ->
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
                    )
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
                composable("espacio") { PantallaEspacio(alVolver = { irA("inicio") }, alAbrirChat = { nav.navigate("chat/${enc(it)}") }, conBarra = false) }
                composable("ajustes") {
                    var cuenta by remember { mutableStateOf(0) }
                    LaunchedEffect(bot.id) {
                        cuenta = runCatching { Repo.contarSolicitudes(bot.id) }.getOrDefault(0)
                        Repo.cambios.collect { evs -> if ("solicitudes" in evs) cuenta = runCatching { Repo.contarSolicitudes(bot.id) }.getOrDefault(0) }
                    }
                    PantallaAjustes(ajustes, alPermisos = { nav.navigate("permisos") }, pendientesSolicitudes = cuenta, alSeguridad = { nav.navigate("seguridad") }, alDescargas = { nav.navigate("descargas") }, alBot = { nav.navigate("bot") }, alAvanzado = { nav.navigate("avanzado") }, alEstadisticas = { nav.navigate("estadisticas") }, alPrivacidad = { nav.navigate("privacidad") }, alAutomatizaciones = { nav.navigate("automatizaciones") }, alModulos = { nav.navigate("modulos") }, alIniciativa = { nav.navigate("iniciativa") })
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
                composable("modulos") {
                    PantallaModulos(
                        bot.id, alVolver = { nav.popBackStack() }, alConfigurar = { nav.navigate("modulo_config/${enc(it)}") },
                        alAviso = { nav.navigate("modulo_aviso/${enc(it)}") }, alIniciativa = { nav.navigate("iniciativa") },
                    )
                }
                composable("modulo_config/{id}") { be ->
                    val id = be.arguments?.getString("id") ?: return@composable
                    PantallaConfigModulo(bot.id, id, alVolver = { nav.popBackStack() })
                }
                composable("modulo_aviso/{id}") { be ->
                    val id = be.arguments?.getString("id") ?: return@composable
                    PantallaAvisoRiesgo(bot.id, id, alVolver = { nav.popBackStack() })
                }
                composable("iniciativa") { PantallaIniciativa(bot.id, alVolver = { nav.popBackStack() }, alRegistro = { nav.navigate("registro_iniciativas") }) }
                composable("registro_iniciativas") { PantallaRegistroIniciativas(bot.id, alVolver = { nav.popBackStack() }) }
                composable("privacidad") { PantallaPrivacidad(ajustes, alVolver = { nav.popBackStack() }) }
                composable("automatizaciones") {
                    PantallaAutomatizaciones(
                        bot.id, alVolver = { nav.popBackStack() }, alEditar = { nav.navigate("automatizacion/$it") }, alEnCurso = { nav.navigate("en_curso") },
                        alActividad = { nav.navigate("actividad") }, alEstadisticas = { nav.navigate("estadisticas_menu/$it") }, alProbar = { nav.navigate("simulador/$it") },
                    )
                }
                composable("automatizacion/{id}") { be ->
                    val id = be.arguments?.getString("id")?.toLongOrNull() ?: return@composable
                    EditorAutomatizacion(bot.id, id, alVolver = { nav.popBackStack() }, alProbar = { nav.navigate("simulador/$it") })
                }
                composable("en_curso") { PantallaEnCurso(bot.id, alVolver = { nav.popBackStack() }, alAbrir = { nav.navigate("recorrido/$it") }) }
                composable("recorrido/{id}") { be ->
                    val id = be.arguments?.getString("id")?.toLongOrNull() ?: return@composable
                    PantallaRecorrido(bot.id, id, alVolver = { nav.popBackStack() })
                }
                composable("estadisticas_menu/{id}") { be ->
                    val id = be.arguments?.getString("id")?.toLongOrNull() ?: return@composable
                    PantallaEstadisticasMenu(bot.id, id, nombreDeAuto(bot.id, id), alVolver = { nav.popBackStack() })
                }
                composable("simulador/{id}") { be ->
                    val id = be.arguments?.getString("id")?.toLongOrNull() ?: return@composable
                    PantallaSimulador(bot.id, id, nombreDeAuto(bot.id, id), alVolver = { nav.popBackStack() })
                }
                composable("actividad") { PantallaActividad(bot.id, alVolver = { nav.popBackStack() }) }
                composable("funciones") { PantallaFunciones(ajustes, alVolver = { nav.popBackStack() }) }
                composable("seguridad") { PantallaSeguridad(ajustes, alVolver = { nav.popBackStack() }) }
                composable("descargas") { PantallaDescargas(ajustes, alVolver = { nav.popBackStack() }) }
                composable("avanzado") {
                    PantallaAvanzado(
                        ajustes, bot.nombre, vm.eliminando, alVolver = { nav.popBackStack() },
                        alAbrirEspacio = { irA("espacio") }, alMedirRendimiento = { nav.navigate("diagnostico") },
                        alFunciones = { nav.navigate("funciones") }, alActividad = { nav.navigate("actividad") }, alEliminarBot = { vm.eliminar(ctx) },
                        alPermisosAvanzado = { nav.navigate("permisos_avanzado") },
                    )
                }
                composable("bot") { PantallaBot(vm, conPermiso, alVolver = { nav.popBackStack() }, alAbrirAvanzado = { nav.navigate("avanzado") }, alAbrirAyuda = { nav.navigate("ayuda") }) }
                composable("ayuda") { AyudaBateria(alVolver = { nav.popBackStack() }) }
                composable("diagnostico") { PantallaDiagnostico(alVolver = { nav.popBackStack() }, alAbrirChat = { nav.navigate("chat/${enc(it)}") }) }
            }
        }
    }
    }
    DialogoConfirmacion(bot.id)
}
