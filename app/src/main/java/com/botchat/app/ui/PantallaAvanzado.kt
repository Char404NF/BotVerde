@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import android.content.ClipData
import android.content.ClipboardManager
import android.widget.Toast
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LargeTopAppBar
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Slider
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.input.nestedscroll.nestedScroll
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.botchat.app.BotState
import com.botchat.app.datos.Ajustes
import com.botchat.app.seguridad.Huella
import kotlin.math.roundToInt

/** Avanzado: seguridad, descargas automáticas, mantenimiento, registro y eliminar el bot. */
@Composable
fun PantallaAvanzado(
    ajustes: Ajustes,
    nombreBot: String,
    eliminando: Boolean,
    alVolver: () -> Unit,
    alCambiarPin: () -> Unit,
    alAbrirEspacio: () -> Unit,
    alMedirRendimiento: () -> Unit,
    alEliminarBot: () -> Unit,
) {
    val ctx = LocalContext.current
    val barra = TopAppBarDefaults.exitUntilCollapsedScrollBehavior()
    val haptica = rememberHaptica()
    val tienePin by ajustes.tienePin.collectAsState()
    val huella by ajustes.huella.collectAsState()
    val secure by ajustes.secure.collectAsState()
    val d by ajustes.descargas.collectAsState()
    val registro by BotState.registro.collectAsState()
    var confirmaEliminar by remember { mutableStateOf(false) }
    var confirmaQuitarPin by remember { mutableStateOf(false) }
    val huellaDisponible = remember { Huella.disponible(ctx) }

    Scaffold(
        modifier = Modifier.nestedScroll(barra.nestedScrollConnection),
        topBar = { LargeTopAppBar(scrollBehavior = barra, navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Avanzado") }) },
    ) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(bottom = 32.dp)) {
            Seccion("Seguridad") {
                Fila("PIN de bloqueo", if (tienePin) "Se pide al abrir y tras 1 minuto en segundo plano" else "Protege tus mensajes con un PIN de 4 a 6 dígitos", Ico.Candado) {
                    if (tienePin) {
                        Row { TextButton(alCambiarPin) { Text("Cambiar") }; TextButton({ confirmaQuitarPin = true }) { Text("Quitar") } }
                    } else {
                        Button(alCambiarPin) { Text("Crear PIN") }
                    }
                }
                Interruptor("Desbloquear con huella", if (huellaDisponible) "Opcional: el PIN siempre sirve" else "Este teléfono no tiene huella configurada", huella, tienePin && huellaDisponible, Ico.Huella) { haptica(Toque.Suave); ajustes.fijarHuella(it) }
                Interruptor("Bloquear capturas de pantalla", "También oculta la vista previa en apps recientes", secure, true, Ico.Candado) { haptica(Toque.Suave); ajustes.fijarSecure(it) }
            }
            Seccion("Descarga automática") {
                Text("Elige qué se descarga solo y por qué conexión. Lo demás queda con el botón Descargar.", Modifier.padding(horizontal = Esp.l), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s)) {
                    Spacer(Modifier.weight(1f)); Text("Wi-Fi", Modifier.width(72.dp), style = MaterialTheme.typography.labelLarge); Text("Datos", Modifier.width(72.dp), style = MaterialTheme.typography.labelLarge)
                }
                FilaDescarga("Fotos", Ico.Foto, d.foto) { r -> ajustes.cambiarDescargas { it.copy(foto = r) } }
                FilaDescarga("Audios", Ico.Audio, d.audio) { r -> ajustes.cambiarDescargas { it.copy(audio = r) } }
                FilaDescarga("Documentos", Ico.Documento, d.documento) { r -> ajustes.cambiarDescargas { it.copy(documento = r) } }
                FilaDescarga("Videos", Ico.Video, d.video) { r -> ajustes.cambiarDescargas { it.copy(video = r) } }
                Deslizador("Tamaño máximo de videos", "${d.videoMaxMb} MB", d.videoMaxMb.toFloat(), 10f..200f, 18) { ajustes.cambiarDescargas { x -> x.copy(videoMaxMb = (it / 10).roundToInt() * 10) } }
                Deslizador("Espacio libre mínimo", if (d.pisoLibreMb >= 1000) "${"%.1f".format(d.pisoLibreMb / 1000f)} GB" else "${d.pisoLibreMb} MB", d.pisoLibreMb.toFloat(), 100f..3000f, 28, "Con menos espacio libre se pausan las descargas automáticas.") { ajustes.cambiarDescargas { x -> x.copy(pisoLibreMb = (it / 100).roundToInt() * 100) } }
            }
            Seccion("Mantenimiento") {
                Fila("Espacio", "Qué ocupa cada chat y cómo liberarlo", Ico.Espacio) { Button(alAbrirEspacio) { Text("Abrir") } }
                Fila("Medir rendimiento", "Pruebas de velocidad y fluidez en este teléfono", Ico.Actualizar) { OutlinedButton(alMedirRendimiento) { Text("Medir") } }
            }
            Seccion("Registro") {
                Text("Sin textos de mensajes, nombres ni nombres de archivos. Los números van enmascarados.", Modifier.padding(horizontal = Esp.l), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                Surface(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s).height(200.dp), shape = RoundedCornerShape(12.dp), color = MaterialTheme.colorScheme.surfaceContainerHigh) {
                    val desplazamiento = rememberScrollState()
                    LaunchedEffect(registro.size) { desplazamiento.scrollTo(desplazamiento.maxValue) }
                    Text(if (registro.isEmpty()) "Todavía no hay mensajes." else registro.joinToString("\n"), Modifier.verticalScroll(desplazamiento).padding(10.dp), fontFamily = FontFamily.Monospace, fontSize = 12.sp, lineHeight = 16.sp)
                }
                OutlinedButton({
                    val cm = ctx.getSystemService(ClipboardManager::class.java)
                    cm.setPrimaryClip(ClipData.newPlainText("registro", BotState.textoRegistro()))
                    Toast.makeText(ctx, "Registro copiado", Toast.LENGTH_SHORT).show()
                }, Modifier.fillMaxWidth().padding(horizontal = Esp.l)) { Icon(Ico.Copiar, null, Modifier.size(18.dp)); Spacer(Modifier.width(8.dp)); Text("Copiar registro") }
            }
            Seccion("Zona delicada") {
                Button(
                    { haptica(Toque.Fuerte); confirmaEliminar = true }, enabled = !eliminando,
                    colors = ButtonDefaults.buttonColors(containerColor = MaterialTheme.colorScheme.error, contentColor = MaterialTheme.colorScheme.onError),
                    modifier = Modifier.fillMaxWidth().padding(horizontal = Esp.l),
                ) { Text(if (eliminando) "Eliminando…" else "Eliminar bot") }
                Text("Se cierra la sesión en WhatsApp y se borran del teléfono los mensajes, los archivos y los datos de “$nombreBot”.", Modifier.padding(horizontal = Esp.l), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }
    if (confirmaEliminar) {
        AlertDialog(
            onDismissRequest = { confirmaEliminar = false },
            title = { Text("¿Eliminar el bot?") },
            text = { Text("Se cerrará la sesión en WhatsApp y se borrarán de este teléfono todos los mensajes, archivos y fotos guardados. Esto no se puede deshacer.") },
            confirmButton = { TextButton({ confirmaEliminar = false; alEliminarBot() }) { Text("Sí, eliminar", color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton({ confirmaEliminar = false }) { Text("Cancelar") } },
        )
    }
    if (confirmaQuitarPin) {
        AlertDialog(
            onDismissRequest = { confirmaQuitarPin = false },
            title = { Text("¿Quitar el PIN?") },
            text = { Text("Cualquiera que abra la app podrá ver tus mensajes.") },
            confirmButton = { TextButton({ confirmaQuitarPin = false; ajustes.quitarPin(); EstadoBloqueo.desbloquear() }) { Text("Quitar PIN", color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton({ confirmaQuitarPin = false }) { Text("Cancelar") } },
        )
    }
}

@Composable
private fun Seccion(titulo: String, contenido: @Composable () -> Unit) {
    Column(Modifier.padding(top = Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
        Text(titulo, Modifier.padding(horizontal = Esp.l), style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary)
        Card(Modifier.fillMaxWidth().padding(horizontal = Esp.m), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
            Column(Modifier.padding(vertical = Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) { contenido() }
        }
    }
}

@Composable
private fun Fila(titulo: String, texto: String, icono: androidx.compose.ui.graphics.vector.ImageVector, accion: @Composable () -> Unit) {
    Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s), verticalAlignment = Alignment.CenterVertically) {
        Icon(icono, null, Modifier.size(24.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.width(Esp.l))
        Column(Modifier.weight(1f)) { Text(titulo, style = MaterialTheme.typography.bodyLarge); Text(texto, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        accion()
    }
}

@Composable
private fun Interruptor(titulo: String, texto: String, valor: Boolean, activo: Boolean, icono: androidx.compose.ui.graphics.vector.ImageVector, alCambiar: (Boolean) -> Unit) {
    Fila(titulo, texto, icono) { Switch(valor, alCambiar, enabled = activo) }
}

@Composable
private fun FilaDescarga(titulo: String, icono: androidx.compose.ui.graphics.vector.ImageVector, r: Ajustes.Regla, alCambiar: (Ajustes.Regla) -> Unit) {
    Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.xs), verticalAlignment = Alignment.CenterVertically) {
        Icon(icono, null, Modifier.size(22.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.width(Esp.m))
        Text(titulo, Modifier.weight(1f), style = MaterialTheme.typography.bodyLarge)
        Box72 { Switch(r.wifi, { alCambiar(r.copy(wifi = it)) }) }
        Box72 { Switch(r.datos, { alCambiar(r.copy(datos = it)) }) }
    }
}

@Composable
private fun Box72(contenido: @Composable () -> Unit) {
    androidx.compose.foundation.layout.Box(Modifier.width(72.dp), Alignment.CenterStart) { contenido() }
}

@Composable
private fun Deslizador(titulo: String, valorTexto: String, valor: Float, rango: ClosedFloatingPointRange<Float>, pasos: Int, nota: String? = null, alCambiar: (Float) -> Unit) {
    Column(Modifier.padding(horizontal = Esp.l, vertical = Esp.s)) {
        Row { Text(titulo, Modifier.weight(1f), style = MaterialTheme.typography.bodyLarge); Text(valorTexto, style = MaterialTheme.typography.titleSmall) }
        Slider(valor, alCambiar, valueRange = rango, steps = pasos)
        if (nota != null) Text(nota, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}
