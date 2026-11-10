package com.botchat.app.ui

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.LocalIndication
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathMeasure
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Fill
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.withTransform
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.PathParser
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.delay

/** Ver cuándo ya se hizo la entrada escalonada de la lista (solo ocurre la primera vez que aparece). */
object EntradaLista {
    @Volatile var hecha = false
}

/** Cada fila entra con un pequeño retraso respecto a la anterior (solo la primera vez y sin pasar de 12 filas). */
fun Modifier.entradaEscalonada(indice: Int): Modifier = composed {
    val reducir = LocalReducirMovimiento.current
    val animar = remember { !EntradaLista.hecha && !reducir && indice < 12 }
    val avance = remember { Animatable(if (animar) 0f else 1f) }
    LaunchedEffect(Unit) { if (animar) { delay(indice * 35L); avance.animateTo(1f, tween(Mov.LARGA)) } }
    if (!animar) this else graphicsLayer { alpha = avance.value; translationY = (1f - avance.value) * 24.dp.toPx() }
}

/** Una burbuja nueva se desliza desde abajo y se desvanece hacia adentro. */
fun Modifier.apareceDesdeAbajo(activo: Boolean): Modifier = composed {
    val reducir = LocalReducirMovimiento.current
    val animar = remember { activo && !reducir }
    val avance = remember { Animatable(if (animar) 0f else 1f) }
    LaunchedEffect(Unit) { if (animar) avance.animateTo(1f, tween(Mov.LARGA)) }
    if (!animar) this else graphicsLayer { alpha = avance.value; translationY = (1f - avance.value) * 32.dp.toPx() }
}

/** Aparece con un desvanecido (miniaturas y fotos que cargan: el tamaño no cambia, solo la opacidad). */
fun Modifier.desvanecidoAlCargar(cargado: Boolean): Modifier = composed {
    val reducir = LocalReducirMovimiento.current
    val a by animateFloatAsState(if (cargado) 1f else 0f, Mov.tiempo(reducir, Mov.NORMAL), label = "desvanecido")
    alpha(a)
}

/** Se pulsa con una ondita (la de Material) y una escala leve. */
fun Modifier.clickableEscala(alPulsar: () -> Unit): Modifier = composed {
    val reducir = LocalReducirMovimiento.current
    val fuente = remember { MutableInteractionSource() }
    val pulsado by fuente.collectIsPressedAsState()
    val escala by animateFloatAsState(if (pulsado && !reducir) 0.97f else 1f, Mov.tiempo(reducir, Mov.CORTA), label = "escala")
    graphicsLayer { scaleX = escala; scaleY = escala }.clickable(interactionSource = fuente, indication = LocalIndication.current, onClick = alPulsar)
}

// ---------- logo dibujado ----------

private const val ANILLO_AMBAR = "M482 788 A306 306 0 0 0 648.7 738.6 L765 795 L732.6 657.5 A306 306 0 0 0 773 387.4"
private const val ANILLO_TURQUESA = "M773 387.4 A306 306 0 1 0 482 788"
private const val FLECHA = "M290 440 L452 318 L444 392 C560 393 650 430 660 540 Q662 580 650 615 C640 560 600 490 520 487 L444 487 L452 566 Z"

private val rutaAmbar: Path by lazy { PathParser().parsePathString(ANILLO_AMBAR).toPath() }
private val rutaTurquesa: Path by lazy { PathParser().parsePathString(ANILLO_TURQUESA).toPath() }
private val rutaFlecha: Path by lazy { PathParser().parsePathString(FLECHA).toPath() }

/**
 * El logo dibujado por partes: `anillo` (0 a 1) va trazando el aro (primero el turquesa y al final el tramo ámbar)
 * y `flecha` (0 a 1) hace entrar la flecha desde la izquierda.
 */
@Composable
fun LogoDibujado(anillo: Float, flecha: Float, modifier: Modifier = Modifier) {
    Canvas(modifier) {
        val escala = size.minDimension / 720f
        withTransform({ scale(escala, escala, Offset.Zero); translate(-122f, -122f) }) {
            val trazo = Stroke(62f, cap = StrokeCap.Round, join = StrokeJoin.Round)
            fun parcial(ruta: Path, f: Float): Path {
                val m = PathMeasure().apply { setPath(ruta, false) }
                return Path().also { m.getSegment(0f, m.length * f.coerceIn(0f, 1f), it, true) }
            }
            val tTur = (anillo / 0.7f).coerceIn(0f, 1f)
            val tAmb = ((anillo - 0.7f) / 0.3f).coerceIn(0f, 1f)
            if (tTur > 0f) drawPath(parcial(rutaTurquesa, tTur), Turquesa, style = trazo)
            if (tAmb > 0f) drawPath(parcial(rutaAmbar, tAmb), Ambar, style = trazo)
            if (flecha > 0f) {
                withTransform({ translate(-90f * (1f - flecha), 0f) }) {
                    drawPath(rutaFlecha, Turquesa.copy(alpha = flecha), style = Fill)
                    drawPath(rutaFlecha, Turquesa.copy(alpha = flecha), style = Stroke(16f, cap = StrokeCap.Round, join = StrokeJoin.Round))
                }
            }
        }
    }
}

/** Pantalla de arranque: el aro se dibuja y entra la flecha. Dura menos de 1 s y no tapa los toques ni retrasa la app. */
object ArranqueLogo { @Volatile var mostrado = false }

@Composable
fun Splash(modifier: Modifier = Modifier) {
    val reducir = LocalReducirMovimiento.current
    var visible by remember { mutableStateOf(!ArranqueLogo.mostrado && !reducir) }
    val anillo = remember { Animatable(0f) }
    val flecha = remember { Animatable(0f) }
    val opacidad = remember { Animatable(1f) }
    LaunchedEffect(Unit) {
        ArranqueLogo.mostrado = true
        if (!visible) return@LaunchedEffect
        anillo.animateTo(1f, tween(520))
        flecha.animateTo(1f, tween(240))
        delay(60)
        opacidad.animateTo(0f, tween(Mov.CORTA))
        visible = false
    }
    if (visible) {
        Box(modifier.fillMaxSize().graphicsLayer { alpha = opacidad.value }.background(FondoIcono), Alignment.Center) {
            LogoDibujado(anillo.value, flecha.value, Modifier.size(144.dp))
        }
    }
}

/** Círculo con una palomita que se dibuja (al vincular el bot). */
@Composable
fun CheckAnimado(modifier: Modifier = Modifier, tam: Dp = 96.dp, color: Color = Turquesa, fondo: Color = Color.Transparent) {
    val reducir = LocalReducirMovimiento.current
    val avance = remember { Animatable(if (reducir) 1f else 0f) }
    LaunchedEffect(Unit) { if (!reducir) avance.animateTo(1f, tween(650)) }
    Canvas(modifier.size(tam)) {
        val ancho = size.minDimension * 0.07f
        val t = avance.value
        if (fondo != Color.Transparent) drawCircle(fondo)
        val aro = Path().apply { addArc(androidx.compose.ui.geometry.Rect(ancho, ancho, size.width - ancho, size.height - ancho), -90f, 360f * (t / 0.6f).coerceIn(0f, 1f)) }
        drawPath(aro, color, style = Stroke(ancho, cap = StrokeCap.Round))
        val s = size.minDimension
        val marca = Path().apply { moveTo(s * 0.3f, s * 0.52f); lineTo(s * 0.45f, s * 0.67f); lineTo(s * 0.72f, s * 0.36f) }
        val m = PathMeasure().apply { setPath(marca, false) }
        val parte = Path().also { m.getSegment(0f, m.length * ((t - 0.5f) / 0.5f).coerceIn(0f, 1f), it, true) }
        drawPath(parte, color, style = Stroke(ancho * 1.3f, cap = StrokeCap.Round, join = StrokeJoin.Round))
    }
}
