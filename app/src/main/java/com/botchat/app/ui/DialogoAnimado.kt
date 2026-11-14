package com.botchat.app.ui

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.tween
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.window.DialogProperties

/** Un diálogo aparece creciendo desde 92 % con un desvanecido (Natural); con Reducidas solo se aclara y con Ninguna no se anima. */
fun Modifier.apareceDialogo(): Modifier = composed {
    val modo = LocalModoAnim.current
    val avance = remember { Animatable(if (modo == ModoAnim.NINGUNA) 1f else 0f) }
    LaunchedEffect(Unit) {
        if (modo != ModoAnim.NINGUNA) avance.animateTo(1f, tween(if (modo == ModoAnim.NATURAL) 260 else Transiciones.FUNDIDO_CORTO_MS, easing = Transiciones.DESACELERA_ENFATIZADA))
    }
    graphicsLayer {
        val t = avance.value
        alpha = t
        if (modo == ModoAnim.NATURAL) { val e = 0.92f + 0.08f * t; scaleX = e; scaleY = e }
    }
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
