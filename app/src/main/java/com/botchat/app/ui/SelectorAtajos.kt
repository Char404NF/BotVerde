@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.style.TextOverflow
import com.botchat.app.datos.AtajoVista
import com.botchat.app.datos.Repo
import kotlinx.coroutines.launch

/** ¿El texto de la caja es el principio de un atajo ("/", "/gra")? Devuelve lo escrito tras la barra, o null si no lo es. */
fun consultaDeAtajo(texto: String): String? = if (texto.startsWith("/") && texto.none { it.isWhitespace() }) texto.substring(1).lowercase() else null

/**
 * Al escribir “/” al principio de la caja de mensaje aparece la lista de respuestas rápidas. Elegir una pone su texto (con las variables
 * ya puestas para este chat) en la caja para revisarlo y mandarlo como siempre: lo manda la persona, no el bot. Si el atajo lleva
 * archivos, un botón los manda aparte (también lo pide la persona).
 */
@Composable
fun SelectorAtajos(botId: Long, chat: String, valor: TextFieldValue, alElegir: (TextFieldValue) -> Unit) {
    val consulta = consultaDeAtajo(valor.text) ?: return
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val atajos by produceState<List<AtajoVista>>(emptyList(), botId, chat) { value = runCatching { Repo.atajos(botId, chat) }.getOrDefault(emptyList()) }
    val ver = atajos.filter { it.nombre.startsWith(consulta) }
    if (atajos.isEmpty()) return
    Surface(Modifier.fillMaxWidth().padding(horizontal = Esp.s).testTag("selector_atajos"), color = MaterialTheme.colorScheme.surfaceContainerHigh, shape = MaterialTheme.shapes.medium, tonalElevation = androidx.compose.ui.unit.Dp(2f)) {
        Column(Modifier.heightIn(max = androidx.compose.ui.unit.Dp(220f)).verticalScroll(rememberScrollState())) {
            Text("Respuestas rápidas", Modifier.padding(horizontal = Esp.m, vertical = Esp.xs), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            if (ver.isEmpty()) Text("Ningún atajo empieza así.", Modifier.padding(horizontal = Esp.m, vertical = Esp.s).testTag("atajos_sin_coincidencia"), style = MaterialTheme.typography.bodySmall)
            ver.forEach { a ->
                Row(Modifier.fillMaxWidth().clickable { alElegir(TextFieldValue(a.texto, TextRange(a.texto.length))) }.padding(horizontal = Esp.m, vertical = Esp.s).testTag("atajo_${a.nombre}"), verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text("/" + a.nombre, style = MaterialTheme.typography.titleSmall)
                        Text(a.texto.replace('\n', ' '), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 2, overflow = TextOverflow.Ellipsis)
                    }
                    if (a.archivos > 0) TextButton({
                        scope.launch { runCatching { Repo.enviarArchivosAtajo(botId, chat, a.id) }.onSuccess { avisaToast(ctx, "Archivos enviados", false) }.onFailure { avisaToast(ctx, it.message ?: "No se pudieron mandar los archivos", true) } }
                    }, Modifier.testTag("atajo_archivos_${a.nombre}")) { Text("📎 ${a.archivos}") }
                }
            }
        }
    }
}

