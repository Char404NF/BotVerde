package com.botchat.app.ui

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.unit.DpOffset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.DialogProperties

/**
 * Entrada con resorte (sin rebote): crece desde 88 % mientras se aclara. Con Reducidas solo se aclara y
 * con Ninguna no se anima. El resorte se acomoda solo, así que no tiene "final" brusco como una curva con duración fija.
 */
fun Modifier.apareceDialogo(): Modifier = composed {
    val modo = LocalModoAnim.current
    val avance = remember { Animatable(if (modo == ModoAnim.NINGUNA) 1f else 0f) }
    LaunchedEffect(Unit) {
        if (modo == ModoAnim.NATURAL) avance.animateTo(1f, spring(dampingRatio = 0.9f, stiffness = 420f))
        else if (modo == ModoAnim.REDUCIDAS) avance.animateTo(1f, tween(Transiciones.FUNDIDO_CORTO_MS))
    }
    graphicsLayer {
        val t = avance.value
        alpha = (t * 1.6f).coerceIn(0f, 1f)
        if (modo == ModoAnim.NATURAL) {
            val e = 0.88f + 0.12f * t; scaleX = e; scaleY = e
        }
    }
}

/** El DropdownMenu de Material con la entrada de la app. */
@Composable
fun DropdownMenu(
    expanded: Boolean,
    onDismissRequest: () -> Unit,
    modifier: Modifier = Modifier,
    offset: DpOffset = DpOffset(0.dp, 0.dp),
    content: @Composable ColumnScope.() -> Unit,
) {
    androidx.compose.material3.DropdownMenu(expanded, onDismissRequest, modifier, offset, content = content)
}

/** El AlertDialog de Material con la entrada animada de la app. Mismos parámetros que usa el resto del código. */
@Composable
fun AlertDialog(
    onDismissRequest: () -> Unit,
    confirmButton: @Composable () -> Unit,
    modifier: Modifier = Modifier,
    dismissButton: @Composable (() -> Unit)? = null,
    icon: @Composable (() -> Unit)? = null,
    title: @Composable (() -> Unit)? = null,
    text: @Composable (() -> Unit)? = null,
    properties: DialogProperties = DialogProperties(),
) {
    androidx.compose.material3.AlertDialog(
        onDismissRequest = onDismissRequest, confirmButton = confirmButton, modifier = modifier.apareceDialogo(),
        dismissButton = dismissButton, icon = icon, title = title, text = text, properties = properties,
    )
}
