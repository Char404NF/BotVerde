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
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.botchat.app.BotState
import com.botchat.app.datos.Ajustes
import android.os.Build
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import kotlin.math.roundToInt

/** Avanzado: aspecto, mantenimiento, registro y eliminar el bot (lo técnico, escondido aquí). */
@Composable
fun PantallaAvanzado(
    ajustes: Ajustes,
    nombreBot: String,
    eliminando: Boolean,
    alVolver: () -> Unit,
    alAbrirEspacio: () -> Unit,
    alMedirRendimiento: () -> Unit,
    alEliminarBot: () -> Unit,
    alFunciones: () -> Unit = {},
) {
    val ctx = LocalContext.current
    val barra = TopAppBarDefaults.exitUntilCollapsedScrollBehavior()
    val haptica = rememberHaptica()
    val modoTema by ajustes.modoTema.collectAsState()
    val dinamico by ajustes.colorDinamico.collectAsState()
    val reducir by ajustes.reducirAnimaciones.collectAsState()
    val registro by BotState.registro.collectAsState()
    var confirmaEliminar by remember { mutableStateOf(false) }

    Scaffold(
        modifier = Modifier.nestedScroll(barra.nestedScrollConnection),
        topBar = { LargeTopAppBar(scrollBehavior = barra, navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Avanzado") }) },
    ) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(bottom = 32.dp)) {
            Seccion("Aspecto") {
                Column(Modifier.padding(horizontal = Esp.l, vertical = Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text("Tema", style = MaterialTheme.typography.bodyLarge)
                    val modos = listOf(ModoTema.SISTEMA to "Sistema", ModoTema.CLARO to "Claro", ModoTema.OSCURO to "Oscuro")
                    SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().testTag("tema_modo")) {
                        modos.forEachIndexed { i, (m, t) ->
                            SegmentedButton(modoTema == m, { haptica(Toque.Suave); ajustes.fijarModoTema(m) }, SegmentedButtonDefaults.itemShape(i, modos.size)) { Text(t) }
                        }
                    }
                }
                Interruptor("Color del sistema", if (Build.VERSION.SDK_INT >= 31) "Usa los colores de tu fondo de pantalla en vez de los de Bot Chat" else "Disponible desde Android 12", dinamico, Build.VERSION.SDK_INT >= 31, Ico.Espacio) { haptica(Toque.Suave); ajustes.fijarColorDinamico(it) }
                Interruptor("Reducir animaciones", "Todo cambia al instante, sin movimiento", reducir, true, Ico.Actualizar) { haptica(Toque.Suave); ajustes.fijarReducirAnimaciones(it) }
            }
            Seccion("Funciones") {
                Fila("Funciones", "Apagar motor, escritura, avisos y más sin reinstalar", Ico.Ajustes) { Button(alFunciones, Modifier.testTag("abrir_funciones")) { Text("Abrir") } }
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
}

@Composable
internal fun Seccion(titulo: String, contenido: @Composable () -> Unit) {
    Column(Modifier.padding(top = Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
        Text(titulo, Modifier.padding(horizontal = Esp.l), style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary)
        Card(Modifier.fillMaxWidth().padding(horizontal = Esp.m), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
            Column(Modifier.padding(vertical = Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) { contenido() }
        }
    }
}

@Composable
internal fun Fila(titulo: String, texto: String, icono: androidx.compose.ui.graphics.vector.ImageVector, accion: @Composable () -> Unit) {
    Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s), verticalAlignment = Alignment.CenterVertically) {
        Icon(icono, null, Modifier.size(24.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.width(Esp.l))
        Column(Modifier.weight(1f)) { Text(titulo, style = MaterialTheme.typography.bodyLarge); Text(texto, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        accion()
    }
}

@Composable
internal fun Interruptor(titulo: String, texto: String, valor: Boolean, activo: Boolean, icono: androidx.compose.ui.graphics.vector.ImageVector, alCambiar: (Boolean) -> Unit) {
    Fila(titulo, texto, icono) { Switch(valor, alCambiar, Modifier.testTag("sw_" + titulo), enabled = activo) }
}

@Composable
internal fun FilaDescarga(titulo: String, icono: androidx.compose.ui.graphics.vector.ImageVector, r: Ajustes.Regla, alCambiar: (Ajustes.Regla) -> Unit) {
    Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.xs), verticalAlignment = Alignment.CenterVertically) {
        Icon(icono, null, Modifier.size(22.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.width(Esp.m))
        Text(titulo, Modifier.weight(1f), style = MaterialTheme.typography.bodyLarge)
        Box72 { Switch(r.wifi, { alCambiar(r.copy(wifi = it)) }, Modifier.testTag("wifi_$titulo")) }
        Box72 { Switch(r.datos, { alCambiar(r.copy(datos = it)) }, Modifier.testTag("datos_$titulo")) }
    }
}

@Composable
internal fun Box72(contenido: @Composable () -> Unit) {
    androidx.compose.foundation.layout.Box(Modifier.width(72.dp), Alignment.CenterStart) { contenido() }
}

@Composable
internal fun Deslizador(titulo: String, valorTexto: String, valor: Float, rango: ClosedFloatingPointRange<Float>, pasos: Int, nota: String? = null, alCambiar: (Float) -> Unit) {
    Column(Modifier.padding(horizontal = Esp.l, vertical = Esp.s)) {
        Row { Text(titulo, Modifier.weight(1f), style = MaterialTheme.typography.bodyLarge); Text(valorTexto, style = MaterialTheme.typography.titleSmall) }
        Slider(valor, alCambiar, valueRange = rango, steps = pasos)
        if (nota != null) Text(nota, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}
