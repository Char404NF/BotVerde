package com.botchat.app.ui

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.tween
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.MutableTransitionState
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Surface
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.IntRect
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.LayoutDirection
import androidx.compose.ui.window.Popup
import androidx.compose.ui.window.PopupPositionProvider
import androidx.compose.ui.window.PopupProperties
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.unit.DpOffset
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.DialogProperties

/** Los diálogos entran con el mismo fundido que abrir un chat (sin escala ni rebote). Ninguna: sin animación. */
fun Modifier.apareceDialogo(): Modifier = composed {
    val modo = LocalModoAnim.current
    val avance = remember { Animatable(if (modo == ModoAnim.NINGUNA) 1f else 0f) }
    LaunchedEffect(Unit) {
        if (modo == ModoAnim.NATURAL) avance.animateTo(1f, tween(220, 40, androidx.compose.animation.core.LinearOutSlowInEasing))
        else if (modo == ModoAnim.REDUCIDAS) avance.animateTo(1f, tween(Transiciones.FUNDIDO_CORTO_MS))
    }
    graphicsLayer { alpha = avance.value }
}

/**
 * Menú de tres puntos con la misma animación que abrir un chat: un solo fundido (sin escalar desde la esquina).
 * Se queda dibujado mientras se desvanece al cerrar.
 */
@Composable
fun DropdownMenu(
    expanded: Boolean,
    onDismissRequest: () -> Unit,
    modifier: Modifier = Modifier,
    offset: DpOffset = DpOffset(0.dp, 0.dp),
    content: @Composable ColumnScope.() -> Unit,
) {
    val modo = LocalModoAnim.current
    val estado = remember { MutableTransitionState(false) }
    estado.targetState = expanded
    if (!estado.currentState && !estado.targetState) return
    val dx = with(LocalDensity.current) { offset.x.roundToPx() }
    val dy = with(LocalDensity.current) { offset.y.roundToPx() }
    val posicion = remember(dx, dy) {
        object : PopupPositionProvider {
            override fun calculatePosition(anchorBounds: IntRect, windowSize: IntSize, layoutDirection: LayoutDirection, popupContentSize: IntSize): IntOffset {
                // Pegado al borde del lado donde está el botón (casi siempre la derecha) y bajo él; si no cabe abajo, sube.
                val x = (if (anchorBounds.center.x > windowSize.width / 2) anchorBounds.right - popupContentSize.width - dx else anchorBounds.left + dx)
                    .coerceIn(0, (windowSize.width - popupContentSize.width).coerceAtLeast(0))
                val abajo = anchorBounds.bottom + dy
                val y = if (abajo + popupContentSize.height <= windowSize.height) abajo else (anchorBounds.top - popupContentSize.height - dy).coerceAtLeast(0)
                return IntOffset(x, y)
            }
        }
    }
    Popup(posicion, onDismissRequest, properties = PopupProperties(focusable = true)) {
        AnimatedVisibility(estado, enter = Transiciones.fundidoEntra(modo), exit = Transiciones.fundidoSale(modo)) {
            Surface(
                shape = androidx.compose.material3.MaterialTheme.shapes.extraSmall, color = androidx.compose.material3.MaterialTheme.colorScheme.surfaceContainer,
                tonalElevation = 3.dp, shadowElevation = 3.dp, modifier = Modifier.padding(8.dp),
            ) {
                Column(modifier.width(IntrinsicSize.Max).widthIn(min = 112.dp, max = 280.dp).verticalScroll(rememberScrollState()).padding(vertical = 8.dp), content = content)
            }
        }
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
