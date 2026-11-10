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
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/** Verdadero si el usuario quitó las animaciones en el sistema (Accesibilidad → Quitar animaciones). */
val LocalReducirMovimiento = compositionLocalOf { false }

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

// Paleta propia (índigo con un acento cálido). Se usa en Android 11 o menor; en Android 12+ se usan los colores del fondo de pantalla.
private val Claro = lightColorScheme(
    primary = Color(0xFF3F4FC8), onPrimary = Color.White, primaryContainer = Color(0xFFDFE0FF), onPrimaryContainer = Color(0xFF000A62),
    secondary = Color(0xFF5B5D72), onSecondary = Color.White, secondaryContainer = Color(0xFFE0E1F9), onSecondaryContainer = Color(0xFF181A2C),
    tertiary = Color(0xFF8E4A63), onTertiary = Color.White, tertiaryContainer = Color(0xFFFFD9E4), onTertiaryContainer = Color(0xFF39071F),
    error = Color(0xFFBA1A1A), onError = Color.White, errorContainer = Color(0xFFFFDAD6), onErrorContainer = Color(0xFF410002),
    background = Color(0xFFFBF8FF), onBackground = Color(0xFF1B1B21), surface = Color(0xFFFBF8FF), onSurface = Color(0xFF1B1B21),
    surfaceVariant = Color(0xFFE3E1EC), onSurfaceVariant = Color(0xFF46464F), outline = Color(0xFF777680), outlineVariant = Color(0xFFC7C5D0),
    surfaceContainerLowest = Color.White, surfaceContainerLow = Color(0xFFF5F2FA), surfaceContainer = Color(0xFFEFEDF4),
    surfaceContainerHigh = Color(0xFFE9E7EF), surfaceContainerHighest = Color(0xFFE4E1E9),
    inverseSurface = Color(0xFF303036), inverseOnSurface = Color(0xFFF2EFF7), inversePrimary = Color(0xFFBBC3FF),
)

private val Oscuro = darkColorScheme(
    primary = Color(0xFFBBC3FF), onPrimary = Color(0xFF0A1B97), primaryContainer = Color(0xFF2634AF), onPrimaryContainer = Color(0xFFDFE0FF),
    secondary = Color(0xFFC4C5DD), onSecondary = Color(0xFF2D2F42), secondaryContainer = Color(0xFF434559), onSecondaryContainer = Color(0xFFE0E1F9),
    tertiary = Color(0xFFFFB0CB), onTertiary = Color(0xFF561D34), tertiaryContainer = Color(0xFF72334B), onTertiaryContainer = Color(0xFFFFD9E4),
    error = Color(0xFFFFB4AB), onError = Color(0xFF690005), errorContainer = Color(0xFF93000A), onErrorContainer = Color(0xFFFFDAD6),
    background = Color(0xFF121318), onBackground = Color(0xFFE4E1E9), surface = Color(0xFF121318), onSurface = Color(0xFFE4E1E9),
    surfaceVariant = Color(0xFF46464F), onSurfaceVariant = Color(0xFFC7C5D0), outline = Color(0xFF91909A), outlineVariant = Color(0xFF46464F),
    surfaceContainerLowest = Color(0xFF0D0E13), surfaceContainerLow = Color(0xFF1B1B21), surfaceContainer = Color(0xFF1F1F25),
    surfaceContainerHigh = Color(0xFF292A2F), surfaceContainerHighest = Color(0xFF34343A),
    inverseSurface = Color(0xFFE4E1E9), inverseOnSurface = Color(0xFF303036), inversePrimary = Color(0xFF3F4FC8),
)

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

@Composable
fun esquemaDeColores(): ColorScheme {
    val oscuro = isSystemInDarkTheme()
    val ctx = LocalContext.current
    return when {
        Build.VERSION.SDK_INT >= 31 -> if (oscuro) dynamicDarkColorScheme(ctx) else dynamicLightColorScheme(ctx)
        oscuro -> Oscuro
        else -> Claro
    }
}

/** Tema de la app: Material 3, claro y oscuro, colores dinámicos en Android 12 o superior. */
@Composable
fun BotChatTema(contenido: @Composable () -> Unit) {
    val ctx = LocalContext.current
    var reducir by remember { mutableStateOf(movimientoReducido(ctx)) }
    androidx.compose.runtime.DisposableEffect(ctx) {
        val obs = object : android.database.ContentObserver(android.os.Handler(android.os.Looper.getMainLooper())) {
            override fun onChange(selfChange: Boolean) { reducir = movimientoReducido(ctx) }
        }
        ctx.contentResolver.registerContentObserver(Settings.Global.getUriFor(Settings.Global.ANIMATOR_DURATION_SCALE), false, obs)
        onDispose { ctx.contentResolver.unregisterContentObserver(obs) }
    }
    CompositionLocalProvider(LocalReducirMovimiento provides reducir) {
        MaterialTheme(colorScheme = esquemaDeColores(), typography = Tipografia, shapes = Formas, content = contenido)
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
