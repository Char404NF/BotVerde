@file:OptIn(ExperimentalSharedTransitionApi::class)

package com.botchat.app.ui

import androidx.compose.animation.AnimatedVisibilityScope
import androidx.compose.animation.ExperimentalSharedTransitionApi
import androidx.compose.animation.SharedTransitionScope
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.snap
import androidx.compose.animation.core.spring
import androidx.compose.runtime.Composable
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.ui.Modifier

/** Apagado: la foto y el nombre ya no vuelan entre la lista y el chat (con el deslizamiento de pantalla se veía doble movimiento). */
private const val ELEMENTOS_COMPARTIDOS = false

val LocalSharedScope = compositionLocalOf<SharedTransitionScope?> { null }
val LocalAnimScope = compositionLocalOf<AnimatedVisibilityScope?> { null }

/**
 * Elemento compartido entre la lista de chats y el chat (foto y nombre). Si falta alguno de los ámbitos,
 * o el sistema pidió quitar animaciones, no hace nada.
 */
@Composable
fun Modifier.compartido(clave: String, limites: Boolean = false): Modifier {
    if (!ELEMENTOS_COMPARTIDOS) return this
    val sh = LocalSharedScope.current ?: return this
    val an = LocalAnimScope.current ?: return this
    val reducir = LocalReducirMovimiento.current
    return with(sh) {
        val estado = rememberSharedContentState(key = clave)
        val transformacion: androidx.compose.animation.BoundsTransform = androidx.compose.animation.BoundsTransform { _, _ ->
            if (reducir) snap() else spring(dampingRatio = Spring.DampingRatioNoBouncy, stiffness = Spring.StiffnessMediumLow)
        }
        if (limites) this@compartido.sharedBounds(estado, an, boundsTransform = transformacion)
        else this@compartido.sharedElement(estado, an, boundsTransform = transformacion)
    }
}
