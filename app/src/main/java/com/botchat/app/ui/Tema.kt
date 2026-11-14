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

// Paleta del rediseño: fondos y superficies como una app de mensajería moderna (casi negro azulado, gris verdoso en claro) con el turquesa propio de BotChat en botones y acentos.
private val Oscuro = darkColorScheme(
    primary = Color(0xFF2EC4CB), onPrimary = Color(0xFF00363A), primaryContainer = Color(0xFF0B3A3D), onPrimaryContainer = Color(0xFFBFF4F7),
    secondary = Color(0xFF8696A0), onSecondary = Color(0xFF0B141A), secondaryContainer = Color(0xFF1F2C34), onSecondaryContainer = Color(0xFFE9EDEF),
    tertiary = Color(0xFFF59E28), onTertiary = Color(0xFF4A2A00), tertiaryContainer = Color(0xFF6A3F00), onTertiaryContainer = Color(0xFFFFDCBA),
    error = Color(0xFFFFB4AB), onError = Color(0xFF690005), errorContainer = Color(0xFF93000A), onErrorContainer = Color(0xFFFFDAD6),
    background = Color(0xFF0B141A), onBackground = Color(0xFFE9EDEF), surface = Color(0xFF0B141A), onSurface = Color(0xFFE9EDEF),
    surfaceVariant = Color(0xFF1F2C34), onSurfaceVariant = Color(0xFF8696A0), outline = Color(0xFF667781), outlineVariant = Color(0xFF222E35),
    surfaceContainerLowest = Color(0xFF081015), surfaceContainerLow = Color(0xFF0F1A20), surfaceContainer = Color(0xFF111B21),
    surfaceContainerHigh = Color(0xFF1F2C34), surfaceContainerHighest = Color(0xFF2A3942),
    inverseSurface = Color(0xFFE9EDEF), inverseOnSurface = Color(0xFF111B21), inversePrimary = Color(0xFF00727A),
    scrim = Color(0xFF000000), surfaceTint = Color(0xFF2EC4CB),
)

private val Claro = lightColorScheme(
    primary = Color(0xFF00727A), onPrimary = Color(0xFFFFFFFF), primaryContainer = Color(0xFFD3F3F4), onPrimaryContainer = Color(0xFF002022),
    secondary = Color(0xFF667781), onSecondary = Color(0xFFFFFFFF), secondaryContainer = Color(0xFFE9EDEF), onSecondaryContainer = Color(0xFF111B21),
    tertiary = Color(0xFF8A5100), onTertiary = Color(0xFFFFFFFF), tertiaryContainer = Color(0xFFFFDCBA), onTertiaryContainer = Color(0xFF2C1700),
    error = Color(0xFFBA1A1A), onError = Color(0xFFFFFFFF), errorContainer = Color(0xFFFFDAD6), onErrorContainer = Color(0xFF410002),
    background = Color(0xFFFFFFFF), onBackground = Color(0xFF111B21), surface = Color(0xFFFFFFFF), onSurface = Color(0xFF111B21),
    surfaceVariant = Color(0xFFF0F2F5), onSurfaceVariant = Color(0xFF667781), outline = Color(0xFF8696A0), outlineVariant = Color(0xFFE9EDEF),
    surfaceContainerLowest = Color(0xFFFFFFFF), surfaceContainerLow = Color(0xFFF7F8FA), surfaceContainer = Color(0xFFF0F2F5),
    surfaceContainerHigh = Color(0xFFE9EDEF), surfaceContainerHighest = Color(0xFFDDE2E5),
    inverseSurface = Color(0xFF233138), inverseOnSurface = Color(0xFFF0F2F5), inversePrimary = Color(0xFF2EC4CB),
    scrim = Color(0xFF000000), surfaceTint = Color(0xFF00727A),
)

/** Colores del chat (burbujas, fondo, palomitas, cápsulas de fecha y de sistema): no existen en Material 3. */
@androidx.compose.runtime.Immutable
data class ColoresChat(
    val fondo: Color, val enviada: Color, val recibida: Color, val textoEnviada: Color, val textoRecibida: Color,
    val horaEnviada: Color, val horaRecibida: Color, val palomita: Color, val palomitaLeida: Color, val capsula: Color, val textoCapsula: Color,
    val seleccion: Color,
)

private val ChatOscuro = ColoresChat(
    fondo = Color(0xFF0B141A), enviada = Color(0xFF005C4B), recibida = Color(0xFF1F2C34), textoEnviada = Color(0xFFE9EDEF), textoRecibida = Color(0xFFE9EDEF),
    horaEnviada = Color(0xFF99BEB7), horaRecibida = Color(0xFF8696A0), palomita = Color(0xFF99BEB7), palomitaLeida = Color(0xFF53BDEB),
    capsula = Color(0xFF182229), textoCapsula = Color(0xFF8696A0), seleccion = Color(0x3300A884),
)
private val ChatClaro = ColoresChat(
    fondo = Color(0xFFEFEAE2), enviada = Color(0xFFD9FDD3), recibida = Color(0xFFFFFFFF), textoEnviada = Color(0xFF111B21), textoRecibida = Color(0xFF111B21),
    horaEnviada = Color(0xFF667781), horaRecibida = Color(0xFF667781), palomita = Color(0xFF667781), palomitaLeida = Color(0xFF53BDEB),
    capsula = Color(0xFFFFFFFF), textoCapsula = Color(0xFF54656F), seleccion = Color(0x3300A884),
)

val LocalColoresChat = compositionLocalOf { ChatOscuro }

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
    titleLarge = Base.titleLarge.copy(fontSize = 22.sp, fontWeight = FontWeight.Medium),
    titleMedium = Base.titleMedium.copy(fontSize = 17.sp, fontWeight = FontWeight.Normal),
    bodyLarge = Base.bodyLarge.copy(fontSize = 16.sp, lineHeight = 22.sp),
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
    CompositionLocalProvider(LocalReducirMovimiento provides (modoAnim != ModoAnim.NATURAL), LocalModoAnim provides modoAnim, LocalColoresApp provides extra, LocalColoresChat provides (if (oscuro) ChatOscuro else ChatClaro)) {
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
