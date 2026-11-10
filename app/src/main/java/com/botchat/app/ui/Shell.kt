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
import androidx.compose.runtime.Composable
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
import com.botchat.app.datos.Repo

private fun enc(s: String) = Uri.encode(s)

/** Nombre del chat para las pantallas que lo muestran en la barra (visor, documentos y archivos compartidos). */
@Composable
private fun nombreDeChat(botId: Long, jid: String): String {
    var nombre by remember(jid) { mutableStateOf("") }
    LaunchedEffect(jid) { nombre = runCatching { Repo.chat(botId, jid)?.nombre }.getOrNull() ?: "" }
    return nombre
}

/** Navegación de la app: lista de chats, chat, visor, archivos, espacio, avanzado y estado del bot. */
@Composable
fun Shell(nav: NavHostController, vm: AppVm, ajustes: Ajustes, conPermiso: (() -> Unit) -> Unit, alCambiarPin: () -> Unit) {
    val reducir = LocalReducirMovimiento.current
    val bot = vm.bot ?: return
    val ctx = LocalContext.current

    fun entra(): AnimatedContentTransitionScope<NavBackStackEntry>.() -> EnterTransition = {
        if (reducir) EnterTransition.None else fadeIn(tween(Mov.NORMAL)) + slideInHorizontally(tween(Mov.LARGA)) { it / 10 }
    }
    fun sale(): AnimatedContentTransitionScope<NavBackStackEntry>.() -> ExitTransition = {
        if (reducir) ExitTransition.None else fadeOut(tween(Mov.CORTA))
    }
    fun vuelveEntra(): AnimatedContentTransitionScope<NavBackStackEntry>.() -> EnterTransition = {
        if (reducir) EnterTransition.None else fadeIn(tween(Mov.NORMAL))
    }
    fun vuelveSale(): AnimatedContentTransitionScope<NavBackStackEntry>.() -> ExitTransition = {
        if (reducir) ExitTransition.None else fadeOut(tween(Mov.CORTA)) + slideOutHorizontally(tween(Mov.LARGA)) { it / 10 }
    }

    SharedTransitionLayout {
        CompositionLocalProvider(LocalSharedScope provides this) {
            NavHost(nav, startDestination = "inicio", enterTransition = entra(), exitTransition = sale(), popEnterTransition = vuelveEntra(), popExitTransition = vuelveSale()) {
                composable("inicio") {
                    CompositionLocalProvider(LocalAnimScope provides this) {
                        PantallaInicio(
                            sinSesion = !bot.vinculado,
                            alAbrirChat = { nav.navigate("chat/${enc(it)}") },
                            alAbrirEspacio = { nav.navigate("espacio") },
                            alAbrirAvanzado = { nav.navigate("avanzado") },
                            alAbrirBot = { nav.navigate("bot") },
                            alRevincular = { vm.revincular() },
                        )
                    }
                }
                composable("chat/{jid}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    CompositionLocalProvider(LocalAnimScope provides this) {
                        PantallaChat(
                            jid, alVolver = { nav.popBackStack() },
                            alAbrirVisor = { id -> nav.navigate("visor/${enc(jid)}/${enc(id)}") },
                            alAbrirDocumento = { id -> nav.navigate("documento/${enc(jid)}/${enc(id)}") },
                            alAbrirArchivos = { nav.navigate("archivos/${enc(jid)}") },
                        )
                    }
                }
                composable("visor/{jid}/{id}", enterTransition = { if (reducir) EnterTransition.None else fadeIn(tween(Mov.NORMAL)) }, exitTransition = { ExitTransition.None },
                    popExitTransition = { if (reducir) ExitTransition.None else fadeOut(tween(Mov.NORMAL)) }) { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    val id = be.arguments?.getString("id") ?: return@composable
                    PantallaVisor(bot.id, jid, id, nombreDeChat(bot.id, jid), alCerrar = { nav.popBackStack() })
                }
                composable("documento/{jid}/{id}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    val id = be.arguments?.getString("id") ?: return@composable
                    PantallaDocumento(bot.id, jid, id, nombreDeChat(bot.id, jid), alVolver = { nav.popBackStack() })
                }
                composable("archivos/{jid}") { be ->
                    val jid = be.arguments?.getString("jid") ?: return@composable
                    PantallaCompartidos(
                        bot.id, jid, nombreDeChat(bot.id, jid), alVolver = { nav.popBackStack() },
                        alAbrirVisor = { id -> nav.navigate("visor/${enc(jid)}/${enc(id)}") },
                        alAbrirDocumento = { id -> nav.navigate("documento/${enc(jid)}/${enc(id)}") },
                    )
                }
                composable("espacio") { PantallaEspacio(alVolver = { nav.popBackStack() }, alAbrirChat = { nav.navigate("chat/${enc(it)}") }) }
                composable("avanzado") {
                    PantallaAvanzado(
                        ajustes, bot.nombre, vm.eliminando, alVolver = { nav.popBackStack() }, alCambiarPin = alCambiarPin,
                        alAbrirEspacio = { nav.navigate("espacio") }, alMedirRendimiento = { nav.navigate("diagnostico") },
                        alEliminarBot = { vm.eliminar(ctx) },
                    )
                }
                composable("bot") { PantallaBot(vm, conPermiso, alVolver = { nav.popBackStack() }, alAbrirAvanzado = { nav.navigate("avanzado") }, alAbrirAyuda = { nav.navigate("ayuda") }) }
                composable("ayuda") { AyudaBateria(alVolver = { nav.popBackStack() }) }
                composable("diagnostico") { PantallaDiagnostico(alVolver = { nav.popBackStack() }) }
            }
        }
    }
}
