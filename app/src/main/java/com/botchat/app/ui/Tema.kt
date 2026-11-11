package com.botchat.app.ui

import android.os.Build
import android.provider.Settings
import androidx.compose.animation.core.AnimationSpec
import androidx.compose.animation.core.FiniteAnimationSpec
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.snap
import androidx.compose.animation.core.spring
import androidx.compose.animation.core.tween
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ColorScheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.dynamicDarkColorScheme
import androidx.compose.material3.dynamicLightColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Verdadero si el usuario quitó las animaciones en el sistema (Accesibilidad → Quitar animaciones). */
val LocalReducirMovimiento = compositionLocalOf { false }

/** Animaciones de pantallas (Avanzado): Natural, Reducidas (solo fundidos cortos) o Ninguna (sin transiciones). */
enum class ModoAnim { NATURAL, REDUCIDAS, NINGUNA }

/** Modo efectivo de las transiciones de pantalla: si el sistema quitó las animaciones es Ninguna sin importar el ajuste. */
val LocalModoAnim = compositionLocalOf { ModoAnim.NATURAL }

/** Lee el ajuste del sistema; se vuelve a leer cada vez que se llama (por ejemplo al volver a la app). */
fun movimientoReducido(ctx: android.content.Context): Boolean =
    runCatching { Settings.Global.getFloat(ctx.contentResolver, Settings.Global.ANIMATOR_DURATION_SCALE, 1f) == 0f }.getOrDefault(false)

/** Tiempos y curvas de las animaciones: suaves, de 150 a 300 ms o resortes naturales; sin animación si el sistema lo pide. */
object Mov {
    const val CORTA = 150
    const val NORMAL = 220
    const val LARGA = 300

    fun <T> tiempo(reducir: Boolean, ms: Int = NORMAL): FiniteAnimationSpec<T> = if (reducir) snap() else tween(ms)
    fun <T> resorte(reducir: Boolean): AnimationSpec<T> =
        if (reducir) snap() else spring(dampingRatio = Spring.DampingRatioNoBouncy, stiffness = Spring.StiffnessMediumLow)
}

// Paleta propia, sacada del ícono: turquesa para acciones y "conectado", ámbar solo como acento (avisos, "reconectando").
// En Avanzado se puede activar el color dinámico del sistema (apagado por defecto).
val Turquesa = Color(0xFF2EC4CB)
val Ambar = Color(0xFFF59E28)
val FondoIcono = Color(0xFF151A38)

private val Oscuro = darkColorScheme(
    primary = Color(0xFF2EC4CB), onPrimary = Color(0xFF003739), primaryContainer = Color(0xFF00494D), onPrimaryContainer = Color(0xFFA0F1F5),
    secondary = Color(0xFF9FB3D9), onSecondary = Color(0xFF112447), secondaryContainer = Color(0xFF2A3560), onSecondaryContainer = Color(0xFFD8E0FF),
    tertiary = Color(0xFFF59E28), onTertiary = Color(0xFF4A2A00), tertiaryContainer = Color(0xFF6A3F00), onTertiaryContainer = Color(0xFFFFDCBA),
    error = Color(0xFFFFB4AB), onError = Color(0xFF690005), errorContainer = Color(0xFF93000A), onErrorContainer = Color(0xFFFFDAD6),
    background = Color(0xFF0F1330), onBackground = Color(0xFFE3E5F5), surface = Color(0xFF151A38), onSurface = Color(0xFFE3E5F5),
    surfaceVariant = Color(0xFF1F2547), onSurfaceVariant = Color(0xFFB8BDE0), outline = Color(0xFF8A90B5), outlineVariant = Color(0xFF353C66),
    surfaceContainerLowest = Color(0xFF0B0F28), surfaceContainerLow = Color(0xFF121734), surfaceContainer = Color(0xFF151A38),
    surfaceContainerHigh = Color(0xFF1F2547), surfaceContainerHighest = Color(0xFF2A3157),
    inverseSurface = Color(0xFFE3E5F5), inverseOnSurface = Color(0xFF151A38), inversePrimary = Color(0xFF00727A),
    scrim = Color(0xFF000000), surfaceTint = Color(0xFF2EC4CB),
)

private val Claro = lightColorScheme(
    primary = Color(0xFF00727A), onPrimary = Color(0xFFFFFFFF), primaryContainer = Color(0xFF9EF0F6), onPrimaryContainer = Color(0xFF002022),
    secondary = Color(0xFF4A5A8A), onSecondary = Color(0xFFFFFFFF), secondaryContainer = Color(0xFFDCE2FF), onSecondaryContainer = Color(0xFF0E1B44),
    tertiary = Color(0xFF8A5100), onTertiary = Color(0xFFFFFFFF), tertiaryContainer = Color(0xFFFFDCBA), onTertiaryContainer = Color(0xFF2C1700),
    error = Color(0xFFBA1A1A), onError = Color(0xFFFFFFFF), errorContainer = Color(0xFFFFDAD6), onErrorContainer = Color(0xFF410002),
    background = Color(0xFFF6F8FF), onBackground = Color(0xFF151A38), surface = Color(0xFFFBFCFF), onSurface = Color(0xFF151A38),
    surfaceVariant = Color(0xFFE3E7F7), onSurfaceVariant = Color(0xFF454B6E), outline = Color(0xFF757B9F), outlineVariant = Color(0xFFC5CAE3),
    surfaceContainerLowest = Color(0xFFFFFFFF), surfaceContainerLow = Color(0xFFF1F4FD), surfaceContainer = Color(0xFFEBEFFA),
    surfaceContainerHigh = Color(0xFFE5E9F5), surfaceContainerHighest = Color(0xFFDFE3F0),
    inverseSurface = Color(0xFF2B3050), inverseOnSurface = Color(0xFFEDF0FF), inversePrimary = Color(0xFF2EC4CB),
    scrim = Color(0xFF000000), surfaceTint = Color(0xFF00727A),
)

/** Colores que no existen en Material 3 pero la app necesita: "conectado" (turquesa) y "reconectando" (ámbar). */
@androidx.compose.runtime.Immutable
data class ColoresApp(val conectado: Color, val reconectando: Color, val sobreReconectando: Color, val contenedorReconectando: Color)

val LocalColoresApp = compositionLocalOf { ColoresApp(Turquesa, Ambar, Color(0xFF4A2A00), Color(0xFF6A3F00)) }

/** Modo de tema que elige el usuario. */
enum class ModoTema { SISTEMA, CLARO, OSCURO }

private val Formas = Shapes(
    extraSmall = RoundedCornerShape(6.dp), small = RoundedCornerShape(10.dp), medium = RoundedCornerShape(16.dp),
    large = RoundedCornerShape(22.dp), extraLarge = RoundedCornerShape(30.dp),
)

private val Base = Typography()
private val Tipografia = Typography(
    displaySmall = Base.displaySmall.copy(fontWeight = FontWeight.SemiBold),
    headlineMedium = Base.headlineMedium.copy(fontWeight = FontWeight.SemiBold),
    headlineSmall = Base.headlineSmall.copy(fontWeight = FontWeight.SemiBold),
    titleLarge = Base.titleLarge.copy(fontWeight = FontWeight.SemiBold),
    titleMedium = Base.titleMedium.copy(fontWeight = FontWeight.SemiBold),
    bodyLarge = Base.bodyLarge.copy(fontSize = 17.sp, lineHeight = 24.sp),
    bodyMedium = Base.bodyMedium.copy(fontSize = 15.sp, lineHeight = 21.sp),
    labelLarge = Base.labelLarge.copy(fontSize = 15.sp),
)

/** Estilo para números y códigos. */
val EstiloCodigo = TextStyle(fontFamily = FontFamily.Monospace, fontWeight = FontWeight.Bold)

/** Esquema de colores según el modo elegido; el color dinámico del sistema solo si el usuario lo activó (Android 12+). */
@Composable
fun esquemaDeColores(modo: ModoTema = ModoTema.SISTEMA, dinamico: Boolean = false): ColorScheme {
    val oscuro = when (modo) { ModoTema.SISTEMA -> isSystemInDarkTheme(); ModoTema.CLARO -> false; ModoTema.OSCURO -> true }
    val ctx = LocalContext.current
    return when {
        dinamico && Build.VERSION.SDK_INT >= 31 -> if (oscuro) dynamicDarkColorScheme(ctx) else dynamicLightColorScheme(ctx)
        oscuro -> Oscuro
        else -> Claro
    }
}

/** Tema de la app: Material 3 con la paleta del ícono, claro y oscuro (sigue al sistema o se elige a mano). */
@Composable
fun BotChatTema(contenido: @Composable () -> Unit) {
    val ctx = LocalContext.current
    val ajustes = remember { com.botchat.app.datos.Ajustes.get(ctx) }
    val modo by ajustes.modoTema.collectAsState()
    val dinamico by ajustes.colorDinamico.collectAsState()
    val modoAjuste by ajustes.animaciones.collectAsState()
    var reducirSistema by remember { mutableStateOf(movimientoReducido(ctx)) }
    androidx.compose.runtime.DisposableEffect(ctx) {
        val obs = object : android.database.ContentObserver(android.os.Handler(android.os.Looper.getMainLooper())) {
            override fun onChange(selfChange: Boolean) { reducirSistema = movimientoReducido(ctx) }
        }
        ctx.contentResolver.registerContentObserver(Settings.Global.getUriFor(Settings.Global.ANIMATOR_DURATION_SCALE), false, obs)
        onDispose { ctx.contentResolver.unregisterContentObserver(obs) }
    }
    val esquema = esquemaDeColores(modo, dinamico)
    val oscuro = esquema.background.luminance() < 0.5f
    val extra = if (oscuro) ColoresApp(Turquesa, Ambar, Color(0xFF4A2A00), Color(0xFF6A3F00)) else ColoresApp(Color(0xFF00727A), Color(0xFF8A5100), Color.White, Color(0xFFFFDCBA))
    val modoAnim = if (reducirSistema) ModoAnim.NINGUNA else modoAjuste
    CompositionLocalProvider(LocalReducirMovimiento provides (modoAnim != ModoAnim.NATURAL), LocalModoAnim provides modoAnim, LocalColoresApp provides extra) {
        MaterialTheme(colorScheme = esquema, typography = Tipografia, shapes = Formas, content = contenido)
    }
}

/** Atajo para espaciado coherente. */
object Esp {
    val xs = 4.dp
    val s = 8.dp
    val m = 12.dp
    val l = 16.dp
    val xl = 24.dp
}

fun Modifier.si(condicion: Boolean, f: Modifier.() -> Modifier): Modifier = if (condicion) f() else this
