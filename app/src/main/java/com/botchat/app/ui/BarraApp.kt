package com.botchat.app.ui

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp

/** Lo que muestra el punto de estado del bot. */
enum class EstadoVisible { CONECTADO, RECONECTANDO, DESVINCULADO, DETENIDO }

fun estadoVisible(estado: String, corriendo: Boolean, sinSesion: Boolean): EstadoVisible = when {
    sinSesion -> EstadoVisible.DESVINCULADO
    corriendo && estado == "conectado" -> EstadoVisible.CONECTADO
    corriendo -> EstadoVisible.RECONECTANDO
    else -> EstadoVisible.DETENIDO
}

fun etiquetaEstado(e: EstadoVisible): String = when (e) {
    EstadoVisible.CONECTADO -> "Conectado"
    EstadoVisible.RECONECTANDO -> "Reconectando"
    EstadoVisible.DESVINCULADO -> "Desvinculado"
    EstadoVisible.DETENIDO -> "Detenido"
}

/** Punto de color + texto. "Conectado" pulsa lento, "Reconectando" gira y el color cambia con una transición. */
@Composable
fun IndicadorEstado(e: EstadoVisible, modifier: Modifier = Modifier, alPulsar: (() -> Unit)? = null) {
    val reducir = LocalReducirMovimiento.current
    val colores = LocalColoresApp.current
    val objetivo = when (e) {
        EstadoVisible.CONECTADO -> colores.conectado
        EstadoVisible.RECONECTANDO -> colores.reconectando
        EstadoVisible.DESVINCULADO -> MaterialTheme.colorScheme.error
        EstadoVisible.DETENIDO -> MaterialTheme.colorScheme.outline
    }
    val color by animateColorAsState(objetivo, Mov.tiempo(reducir, Mov.LARGA), label = "colorEstado")
    val t = rememberInfiniteTransition(label = "estado")
    val pulso by t.animateFloat(0f, 1f, infiniteRepeatable(tween(1800), RepeatMode.Reverse), label = "pulso")
    val giro by t.animateFloat(0f, 360f, infiniteRepeatable(tween(1100, easing = LinearEasing)), label = "giro")
    Row(
        modifier.testTag("indicador_estado").clip(RoundedCornerShape(50)).let { if (alPulsar != null) it.clickable(onClick = alPulsar) else it }.padding(horizontal = 10.dp, vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.size(14.dp), Alignment.Center) {
            when {
                e == EstadoVisible.RECONECTANDO && !reducir ->
                    Icon(Ico.Actualizar, null, Modifier.size(14.dp).graphicsLayer { rotationZ = giro }, tint = color)
                e == EstadoVisible.CONECTADO && !reducir -> {
                    Box(Modifier.size(14.dp).graphicsLayer { val k = 0.7f + 0.6f * pulso; scaleX = k; scaleY = k; alpha = 0.35f * (1f - pulso) }.clip(CircleShape).background(color))
                    Box(Modifier.size(9.dp).clip(CircleShape).background(color))
                }
                else -> Box(Modifier.size(9.dp).clip(CircleShape).background(color))
            }
        }
        Spacer(Modifier.width(6.dp))
        Text(etiquetaEstado(e), style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

/** Barra superior de las pantallas principales: logo pequeño, nombre y estado del bot. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun BarraApp(estado: EstadoVisible, alPulsarEstado: () -> Unit) {
    TopAppBar(
        title = {
            Row(verticalAlignment = Alignment.CenterVertically) {
                LogoBot(Modifier.size(32.dp))
                Spacer(Modifier.width(10.dp))
                Text("Bot Chat", style = MaterialTheme.typography.titleLarge)
            }
        },
        actions = { IndicadorEstado(estado, Modifier.padding(end = 8.dp), alPulsarEstado) },
        colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.surface),
    )
}
