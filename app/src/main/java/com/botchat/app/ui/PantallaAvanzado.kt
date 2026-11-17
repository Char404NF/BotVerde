@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

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
import com.botchat.app.ui.AlertDialog
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
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.Ajustes
import android.os.Build
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import kotlin.math.roundToInt

/** Avanzado: aspecto, funciones, pausa del bot y eliminar el bot (lo técnico, escondido aquí). */
@Composable
fun PantallaAvanzado(
    ajustes: Ajustes,
    nombreBot: String,
    eliminando: Boolean,
    alVolver: () -> Unit,
    alEliminarBot: () -> Unit,
    alFunciones: () -> Unit = {},
    alPermisosAvanzado: () -> Unit = {},
    alMenus: () -> Unit = {},
    botId: Long = 0,
) {
    val barra = TopAppBarDefaults.exitUntilCollapsedScrollBehavior()
    val haptica = rememberHaptica()
    val modoTema by ajustes.modoTema.collectAsState()
    val dinamico by ajustes.colorDinamico.collectAsState()
    val animModo by ajustes.animaciones.collectAsState()
    val pausa by ajustes.pausaBotMin.collectAsState()
    val estiloLetra by ajustes.estiloLetra.collectAsState()
    val completo by ajustes.modoCompleto.collectAsState()
    var confirmaEliminar by remember { mutableStateOf(false) }

    Scaffold(
        modifier = Modifier.nestedScroll(barra.nestedScrollConnection),
        topBar = { LargeTopAppBar(scrollBehavior = barra, navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Avanzado") }) },
    ) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(bottom = 32.dp)) {
            Seccion("Modo de la app") {
                Column(Modifier.padding(horizontal = Esp.l, vertical = Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    val modosApp = listOf(false to "Simple", true to "Completo")
                    SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().testTag("modo_app")) {
                        modosApp.forEachIndexed { i, (v, t) ->
                            SegmentedButton(completo == v, { haptica(Toque.Suave); ajustes.fijarModoCompleto(v) }, SegmentedButtonDefaults.itemShape(i, modosApp.size), modifier = Modifier.testTag(if (v) "modo_completo" else "modo_simple")) { Text(t) }
                        }
                    }
                    Text(
                        if (completo) "Completo: ves todo, incluidos comandos propios, módulos y límites."
                        else "Simple: se ocultan los comandos propios, los módulos y los límites. No se borra nada; vuelven en modo Completo.",
                        style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.testTag("texto_modo_app"),
                    )
                }
            }
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
                Column(Modifier.padding(horizontal = Esp.l, vertical = Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text("Animaciones de pantallas", style = MaterialTheme.typography.bodyLarge)
                    val modosAnim = listOf(ModoAnim.NATURAL to "Natural", ModoAnim.REDUCIDAS to "Reducidas", ModoAnim.NINGUNA to "Ninguna")
                    SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().testTag("anim_modo")) {
                        modosAnim.forEachIndexed { i, (m, t) ->
                            SegmentedButton(animModo == m, { haptica(Toque.Suave); ajustes.fijarAnimaciones(m) }, SegmentedButtonDefaults.itemShape(i, modosAnim.size), modifier = Modifier.testTag("anim_${m.name.lowercase()}")) { Text(t) }
                        }
                    }
                    Text(
                        when (animModo) {
                            ModoAnim.NATURAL -> "Un solo fundido suave al cambiar de pantalla."
                            ModoAnim.REDUCIDAS -> "Solo fundidos cortos, sin movimiento."
                            ModoAnim.NINGUNA -> "Todo cambia al instante."
                        },
                        style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
            Seccion("Funciones") {
                Fila("Funciones", "Apagar motor, escritura, avisos y más sin reinstalar", Ico.Ajustes) { Button(alFunciones, Modifier.testTag("abrir_funciones")) { Text("Abrir") } }
                if (completo) Fila("Permisos y comandos", "Prefijo, topes, códigos y registro de acciones", Ico.Candado) { OutlinedButton(alPermisosAvanzado, Modifier.testTag("abrir_permisos_avanzado")) { Text("Abrir") } }
                Fila("Menús", "Escalera de errores, saludos, grupos y lo que no entendió", Ico.Chats) { OutlinedButton(alMenus, Modifier.testTag("abrir_ajustes_menus_avanzado")) { Text("Abrir") } }
                Fila("Aa Estilo de letra", "Botón para escribir con letras especiales (𝗡𝗲𝗴𝗿𝗶𝘁𝗮, 𝒮𝒸𝓇𝒾𝓅𝓉…). Puede verse distinto en otros teléfonos, no se puede buscar y los lectores de pantalla lo leen mal", Ico.Editar) {
                    Switch(estiloLetra, { haptica(Toque.Suave); ajustes.fijarEstiloLetra(it) }, Modifier.testTag("sw_estilo_letra"))
                }
            }
            if (botId != 0L) SeccionMiWhatsApp(botId)
            Seccion("Cuando escribo yo") {
                Deslizador(
                    "Pausa del bot cuando escribo yo", if (pausa == 0) "Ninguna" else "$pausa min", pausa.toFloat(), 0f..120f, 11,
                    "Normalmente 0: si contesto yo a un mensaje, el bot solo cancela su respuesta pendiente a ese mensaje y sigue atendiendo al resto. Con un valor mayor, el bot calla en ese chat este tiempo.",
                ) { ajustes.fijarPausaBotMin((it / 10f).toInt() * 10) }
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
