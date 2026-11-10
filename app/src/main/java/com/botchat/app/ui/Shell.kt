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
    val reducir = LocalReducirMovimiento.current
    val bot = vm.bot ?: return
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    // Una notificación de mensaje abre el chat correspondiente.
    val pedido by com.botchat.app.NavegarA.chat.collectAsState()
    LaunchedEffect(pedido) {
        pedido?.let { j -> com.botchat.app.NavegarA.chat.value = null; nav.navigate("chat/${enc(j)}") { launchSingleTop = true } }
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
        if (reducir) EnterTransition.None
        else if (entre(initialState.destination.route, targetState.destination.route)) fadeIn(tween(Mov.NORMAL))
        else fadeIn(tween(Mov.NORMAL)) + slideInHorizontally(tween(Mov.LARGA)) { it / 10 }
    }
    fun sale(): AnimatedContentTransitionScope<NavBackStackEntry>.() -> ExitTransition = {
        if (reducir) ExitTransition.None else fadeOut(tween(Mov.CORTA))
    }
    fun vuelveEntra(): AnimatedContentTransitionScope<NavBackStackEntry>.() -> EnterTransition = {
        if (reducir) EnterTransition.None else fadeIn(tween(Mov.NORMAL))
    }
    fun vuelveSale(): AnimatedContentTransitionScope<NavBackStackEntry>.() -> ExitTransition = {
        if (reducir) ExitTransition.None
        else if (entre(initialState.destination.route, targetState.destination.route)) fadeOut(tween(Mov.CORTA))
        else fadeOut(tween(Mov.CORTA)) + slideOutHorizontally(tween(Mov.LARGA)) { it / 10 }
    }
    fun irA(ruta: String) {
        if (ruta == rutaActual) return
        haptica(Toque.Suave)
        nav.navigate(ruta) { popUpTo("inicio") { saveState = true }; launchSingleTop = true; restoreState = true }
    }

    Scaffold(
        contentWindowInsets = androidx.compose.foundation.layout.WindowInsets(0, 0, 0, 0),
        topBar = {
            AnimatedVisibility(principal, enter = if (reducir) EnterTransition.None else fadeIn(tween(Mov.NORMAL)), exit = if (reducir) ExitTransition.None else fadeOut(tween(Mov.CORTA))) {
                BarraApp(estadoVisible(estado, corriendo, !bot.vinculado)) { nav.navigate("bot") }
            }
        },
        bottomBar = {
            AnimatedVisibility(principal, enter = if (reducir) EnterTransition.None else fadeIn(tween(Mov.NORMAL)), exit = if (reducir) ExitTransition.None else fadeOut(tween(Mov.CORTA))) {
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
                composable("visor/{jid}/{id}", enterTransition = { if (reducir) EnterTransition.None else fadeIn(tween(Mov.NORMAL)) }, exitTransition = { ExitTransition.None },
                    popExitTransition = { if (reducir) ExitTransition.None else fadeOut(tween(Mov.NORMAL)) }) { be ->
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
                    PantallaAjustes(ajustes, alSeguridad = { nav.navigate("seguridad") }, alDescargas = { nav.navigate("descargas") }, alBot = { nav.navigate("bot") }, alAvanzado = { nav.navigate("avanzado") }, alEstadisticas = { nav.navigate("estadisticas") }, alPrivacidad = { nav.navigate("privacidad") })
                }
                composable("privacidad") { PantallaPrivacidad(ajustes, alVolver = { nav.popBackStack() }) }
                composable("funciones") { PantallaFunciones(ajustes, alVolver = { nav.popBackStack() }) }
                composable("seguridad") { PantallaSeguridad(ajustes, alVolver = { nav.popBackStack() }) }
                composable("descargas") { PantallaDescargas(ajustes, alVolver = { nav.popBackStack() }) }
                composable("avanzado") {
                    PantallaAvanzado(
                        ajustes, bot.nombre, vm.eliminando, alVolver = { nav.popBackStack() },
                        alAbrirEspacio = { irA("espacio") }, alMedirRendimiento = { nav.navigate("diagnostico") },
                        alFunciones = { nav.navigate("funciones") }, alEliminarBot = { vm.eliminar(ctx) },
                    )
                }
                composable("bot") { PantallaBot(vm, conPermiso, alVolver = { nav.popBackStack() }, alAbrirAvanzado = { nav.navigate("avanzado") }, alAbrirAyuda = { nav.navigate("ayuda") }) }
                composable("ayuda") { AyudaBateria(alVolver = { nav.popBackStack() }) }
                composable("diagnostico") { PantallaDiagnostico(alVolver = { nav.popBackStack() }) }
            }
        }
    }
    }
}
