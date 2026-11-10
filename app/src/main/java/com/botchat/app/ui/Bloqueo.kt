package com.botchat.app.ui

import android.os.SystemClock
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.fragment.app.FragmentActivity
import androidx.media3.common.util.UnstableApi
import com.botchat.app.datos.Ajustes
import com.botchat.app.medios.CacheImagenes
import com.botchat.app.medios.ReproductorAudio
import com.botchat.app.medios.Temporales
import com.botchat.app.seguridad.Huella
import com.botchat.app.seguridad.ResultadoPin
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

/** Estado del bloqueo: solo existe si el usuario lo activó. Se bloquea al abrir y al volver tras el tiempo elegido. */
@OptIn(UnstableApi::class)
object EstadoBloqueo {
    val bloqueado = MutableStateFlow(false)
    private var salida = -1L

    fun iniciar(a: Ajustes) { bloqueado.value = a.bloqueo.value != Ajustes.Bloqueo.NINGUNO }
    fun alSalir() { salida = SystemClock.elapsedRealtime() }

    fun alVolver(ctx: android.content.Context, a: Ajustes) {
        val fuera = salida
        salida = -1L
        if (fuera < 0L || a.bloqueo.value == Ajustes.Bloqueo.NINGUNO || bloqueado.value) return
        if (SystemClock.elapsedRealtime() - fuera >= a.bloqueoSeg.value * 1000L) bloquear(ctx)
    }

    /** Bloquea y borra todo lo que estaba descifrado en memoria o en copias temporales. */
    fun bloquear(ctx: android.content.Context) {
        bloqueado.value = true
        CacheImagenes.vaciar()
        Temporales.borrarTodo(ctx)
        ReproductorAudio.detener()
    }

    fun desbloquear() { bloqueado.value = false }
}

/** Teclado numérico grande. La tecla de abajo a la derecha confirma (OK) y la de la izquierda es opcional (huella). */
@Composable
fun TecladoPin(alDigito: (Char) -> Unit, alBorrar: () -> Unit, alConfirmar: () -> Unit, confirmarActivo: Boolean, izquierda: (@Composable () -> Unit)? = null) {
    val haptica = rememberHaptica()
    @Composable fun Tecla(contenido: @Composable () -> Unit, habilitada: Boolean = true, alPulsar: () -> Unit) {
        Box(
            Modifier.size(76.dp).clip(CircleShape).background(MaterialTheme.colorScheme.surfaceContainerHigh.copy(alpha = if (habilitada) 1f else 0.4f))
                .clickable(enabled = habilitada) { haptica(Toque.Suave); alPulsar() },
            Alignment.Center,
        ) { contenido() }
    }
    Column(verticalArrangement = Arrangement.spacedBy(Esp.m), horizontalAlignment = Alignment.CenterHorizontally) {
        listOf("123", "456", "789").forEach { fila ->
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.xl)) {
                fila.forEach { d -> Tecla({ Text(d.toString(), fontSize = 28.sp, fontWeight = FontWeight.Medium) }) { alDigito(d) } }
            }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(Esp.xl)) {
            Box(Modifier.size(76.dp), Alignment.Center) { izquierda?.invoke() }
            Tecla({ Text("0", fontSize = 28.sp, fontWeight = FontWeight.Medium) }) { alDigito('0') }
            Tecla({ Text("OK", fontSize = 20.sp, fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.primary) }, confirmarActivo) { alConfirmar() }
        }
        TextButton(alBorrar) { Text("Borrar") }
    }
}

@Composable
fun PuntosPin(cantidad: Int) {
    Row(horizontalArrangement = Arrangement.spacedBy(14.dp), modifier = Modifier.height(24.dp), verticalAlignment = Alignment.CenterVertically) {
        repeat(6) { i ->
            val lleno = i < cantidad
            val escala by animateFloatAsState(if (lleno) 1f else 0.6f, tween(120), label = "punto")
            Box(
                Modifier.size(16.dp).graphicsLayer { scaleX = escala; scaleY = escala }.clip(CircleShape)
                    .background(if (lleno) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.outlineVariant),
            )
        }
    }
}

/**
 * Pantalla de bloqueo: PIN de 4 a 6 dígitos y huella opcional. También sirve para pedir el PIN actual antes de
 * cambiarlo o quitarlo (con `alCancelar`). "Olvidé mi PIN" lo quita tras confirmar con el bloqueo del teléfono;
 * los mensajes no se tocan porque su llave no depende del PIN.
 */
@Composable
fun PantallaBloqueo(
    ajustes: Ajustes,
    titulo: String = "Bot Chat está bloqueado",
    subtitulo: String = "Escribe tu PIN",
    alCancelar: (() -> Unit)? = null,
    alDesbloquear: () -> Unit,
) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    val reducir = LocalReducirMovimiento.current
    var pin by remember { mutableStateOf("") }
    var mensaje by remember { mutableStateOf<String?>(null) }
    var espera by remember { mutableLongStateOf(ajustes.intentos.segundosDeEspera()) }
    var verificando by remember { mutableStateOf(false) }
    var olvido by remember { mutableStateOf(0) }
    val sacudida = remember { Animatable(0f) }
    val huellaOk = remember { Huella.disponible(ctx) }
    val modo by ajustes.bloqueo.collectAsState()
    val tienePin by ajustes.tienePin.collectAsState()
    val usaHuella = modo == Ajustes.Bloqueo.HUELLA && alCancelar == null
    val respaldoTelefono = remember { Huella.conRespaldoDelTelefono(ctx) }
    val telefonoSeguro = remember { Huella.telefonoTieneBloqueo(ctx) }
    val actividad = ctx as? FragmentActivity

    fun pedirHuella() { actividad?.let { Huella.pedir(it, respaldoTelefono = !tienePin, alExito = { haptica(Toque.Confirmar); alDesbloquear() }) } }
    fun quitarYAbrir() { ajustes.quitarBloqueo(); haptica(Toque.Confirmar); alDesbloquear() }

    fun intentar() {
        if (verificando || pin.length < 4) return
        val probado = pin
        verificando = true
        scope.launch {
            val r = withContext(Dispatchers.Default) { ajustes.intentos.probar(probado) }
            verificando = false
            when (r) {
                is ResultadoPin.Correcto -> { haptica(Toque.Confirmar); alDesbloquear() }
                is ResultadoPin.Incorrecto -> {
                    haptica(Toque.Rechazar); pin = ""
                    mensaje = "PIN incorrecto. Te quedan ${r.intentosRestantes} intento${if (r.intentosRestantes == 1) "" else "s"}."
                    if (!reducir) { sacudida.animateTo(18f, tween(50)); sacudida.animateTo(-18f, tween(80)); sacudida.animateTo(10f, tween(60)); sacudida.animateTo(0f, tween(60)) }
                }
                is ResultadoPin.Bloqueado -> { haptica(Toque.Rechazar); pin = ""; espera = r.segundos; mensaje = null }
            }
        }
    }
    LaunchedEffect(espera) { while (espera > 0) { delay(1000); espera = ajustes.intentos.segundosDeEspera() } }
    LaunchedEffect(usaHuella, huellaOk) { if (usaHuella && huellaOk && espera == 0L) pedirHuella() }

    Surface(Modifier.fillMaxSize()) {
        Column(Modifier.fillMaxSize().padding(Esp.xl), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
            LogoBot(Modifier.size(72.dp))
            Spacer(Modifier.height(Esp.l))
            Text(titulo, style = MaterialTheme.typography.titleLarge, textAlign = TextAlign.Center)
            if (!tienePin) {
                // Huella con el bloqueo del teléfono como respaldo: no hay PIN propio.
                Text("Usa tu huella para entrar", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                Spacer(Modifier.height(Esp.xl))
                Box(Modifier.size(96.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primaryContainer).clickable { pedirHuella() }, Alignment.Center) {
                    Icon(Ico.Huella, "Usar huella", Modifier.size(48.dp), tint = MaterialTheme.colorScheme.onPrimaryContainer)
                }
                Spacer(Modifier.height(Esp.l))
                TextButton({ pedirHuella() }) { Text("Desbloquear") }
            } else {
                Text(subtitulo, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                Spacer(Modifier.height(Esp.xl))
                Box(Modifier.graphicsLayer { translationX = sacudida.value }) { PuntosPin(pin.length) }
                Spacer(Modifier.height(Esp.m))
                Text(
                    when { espera > 0 -> "Demasiados intentos. Espera $espera segundo${if (espera == 1L) "" else "s"}."; mensaje != null -> mensaje!!; else -> " " },
                    color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodyMedium, textAlign = TextAlign.Center, modifier = Modifier.height(44.dp),
                )
                Spacer(Modifier.height(Esp.m))
                TecladoPin(
                    alDigito = { d -> if (espera == 0L && pin.length < 6) { pin += d; mensaje = null; if (pin.length == 6) intentar() } },
                    alBorrar = { pin = pin.dropLast(1) }, alConfirmar = ::intentar,
                    confirmarActivo = pin.length >= 4 && espera == 0L && !verificando,
                    izquierda = if (usaHuella && huellaOk) ({
                        Box(Modifier.size(76.dp).clip(CircleShape).clickable { pedirHuella() }, Alignment.Center) { Icon(Ico.Huella, "Usar huella", Modifier.size(32.dp), tint = MaterialTheme.colorScheme.primary) }
                    }) else null,
                )
                Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    if (alCancelar != null) TextButton(alCancelar) { Text("Cancelar") }
                    TextButton({
                        if (telefonoSeguro && actividad != null) Huella.confirmarConTelefono(actividad, "Quitar el PIN", alExito = ::quitarYAbrir)
                        else olvido = 1
                    }) { Text("Olvidé mi PIN") }
                }
            }
        }
    }
    if (olvido == 1) {
        AlertDialog(
            onDismissRequest = { olvido = 0 },
            title = { Text("Tu teléfono no tiene bloqueo de pantalla") },
            text = { Text("Sin bloqueo en el teléfono, el PIN de Bot Chat es solo una protección ligera: cualquiera que tenga el teléfono podría quitarlo desde aquí. Tus mensajes no se borran. ¿Quieres seguir?") },
            confirmButton = { TextButton({ olvido = 2 }) { Text("Seguir") } },
            dismissButton = { TextButton({ olvido = 0 }) { Text("Cancelar") } },
        )
    }
    if (olvido == 2) {
        AlertDialog(
            onDismissRequest = { olvido = 0 },
            title = { Text("¿Quitar el PIN?") },
            text = { Text("Última confirmación: se quitará el bloqueo de Bot Chat y se abrirá la app. Podrás crear otro PIN en Ajustes → Seguridad.") },
            confirmButton = { TextButton({ olvido = 0; quitarYAbrir() }) { Text("Sí, quitar el PIN", color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton({ olvido = 0 }) { Text("Cancelar") } },
        )
    }
}

/** Para crear o cambiar el PIN: se escribe dos veces. */
@Composable
fun PantallaConfigurarPin(titulo: String, alTerminar: (String) -> Unit, alOmitir: (() -> Unit)?, nota: String? = null, textoOmitir: String = "Ahora no") {
    var primero by remember { mutableStateOf<String?>(null) }
    var pin by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }
    val haptica = rememberHaptica()
    Surface(Modifier.fillMaxSize()) {
        Column(Modifier.fillMaxSize().padding(Esp.xl), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
            LogoBot(Modifier.size(72.dp))
            Spacer(Modifier.height(Esp.l))
            Text(titulo, style = MaterialTheme.typography.titleLarge, textAlign = TextAlign.Center)
            Text(if (primero == null) "Elige un PIN de 4 a 6 dígitos" else "Escríbelo otra vez para confirmar", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(Esp.xl))
            PuntosPin(pin.length)
            Spacer(Modifier.height(Esp.m))
            Text(error ?: " ", color = MaterialTheme.colorScheme.error, modifier = Modifier.height(24.dp))
            Spacer(Modifier.height(Esp.m))
            TecladoPin(
                alDigito = { d -> if (pin.length < 6) { pin += d; error = null } },
                alBorrar = { pin = pin.dropLast(1) },
                alConfirmar = {
                    val p = primero
                    if (p == null) { primero = pin; pin = "" }
                    else if (p == pin) { haptica(Toque.Confirmar); alTerminar(pin) }
                    else { haptica(Toque.Rechazar); error = "Los PIN no coinciden. Empieza de nuevo."; primero = null; pin = "" }
                },
                confirmarActivo = pin.length >= 4,
            )
            if (nota != null) { Spacer(Modifier.height(Esp.s)); Text(nota, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.tertiary, textAlign = TextAlign.Center) }
            if (alOmitir != null) { Spacer(Modifier.height(Esp.m)); TextButton(alOmitir) { Text(textoOmitir) } }
        }
    }
}

