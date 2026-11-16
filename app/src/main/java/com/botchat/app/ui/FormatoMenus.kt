package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.EstadoLab
import com.botchat.app.datos.Repo
import kotlinx.coroutines.launch

/**
 * Cómo salen los menús del bot: texto con números (lo normal), encuesta de WhatsApp o botones y listas. Es el formato general; cada menú
 * puede elegir el suyo en su editor y cada opción puede ser un botón especial (enlace, copiar, llamar). Los botones y las listas solo
 * salen en chats privados; en un grupo sale texto. Si algo falla, el menú sale en texto con números.
 */
@Composable
fun TarjetaFormatoMenus(botId: Long) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    var estado by remember { mutableStateOf<EstadoLab?>(null) }
    androidx.compose.runtime.LaunchedEffect(botId, version) { estado = runCatching { Repo.estadoLab(botId) }.getOrNull() ?: estado }
    fun fijar(clave: String, valor: String) = scope.launch {
        runCatching { Repo.fijarAjusteLab(botId, clave, valor) }.onFailure { avisaToast(ctx, textoError(it), true) }
        version++
    }
    val e = estado
    Card(Modifier.fillMaxWidth().testTag("tarjeta_formato_menus"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Text("Cómo se muestran los menús", style = MaterialTheme.typography.titleMedium)
            if (e == null) { FilasDeCarga(2); return@Column }
            val modos = listOf("texto" to "Texto", "encuesta" to "Encuesta", "botones" to "Botones")
            SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().testTag("formato_general")) {
                modos.forEachIndexed { i, (m, t) ->
                    SegmentedButton(e.modo == m, { haptica(Toque.Suave); fijar("lab_modo_menu", m) }, SegmentedButtonDefaults.itemShape(i, modos.size), modifier = Modifier.testTag("formato_general_$m")) { Text(t) }
                }
            }
            Text(
                when (e.modo) {
                    "encuesta" -> "El menú sale como una encuesta de WhatsApp: la persona toca una opción y el menú sigue como si hubiera escrito el número."
                    "botones" -> "Hasta 3 opciones salen como botones; hasta 10, como lista. Solo en chats privados: en un grupo sale texto. Cada opción puede ser un botón de enlace, de copiar o de llamar."
                    else -> "El menú sale como texto con números: la persona escribe 1, 2, 3…"
                },
                Modifier.testTag("formato_general_ayuda"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Text("Cada menú puede elegir el suyo en su editor. Si algo falla (o tras 3 fallos seguidos), el menú sale en texto con números.", style = MaterialTheme.typography.bodySmall)
            if (e.modo == "botones") {
                val tipos = listOf("auto" to "Auto", "botones" to "Botones", "lista" to "Lista")
                Text("Qué usar", style = MaterialTheme.typography.labelLarge)
                SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().testTag("formato_botones_tipo")) {
                    tipos.forEachIndexed { i, (m, t) ->
                        SegmentedButton((if (e.botonesTipo == "interactivo") "botones" else e.botonesTipo) == m, { haptica(Toque.Suave); fijar("lab_botones_tipo", m) }, SegmentedButtonDefaults.itemShape(i, tipos.size), modifier = Modifier.testTag("formato_tipo_$m")) { Text(t, maxLines = 1) }
                    }
                }
                Text("Auto: hasta 3 opciones, botones; hasta 10, lista; más, texto.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                NumeroGuardable("Menús con botones por hora (0 = sin tope)", e.botonesHora, 0, 500, "formato_botones_hora", "Pasado el tope, los menús salen en texto.") { fijar("menu_botones_hora", it.toString()) }
            }
            if (e.modo == "encuesta") {
                Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text("También en grupos")
                        Text("En un grupo, todos ven los votos de todos. Por defecto solo se usan en privado.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    Switch(e.grupos, { fijar("lab_grupos", if (it) "1" else "0") }, Modifier.testTag("formato_enc_grupos"))
                }
                NumeroGuardable("Opciones máximas por encuesta", e.encMax, 2, 32, "formato_enc_max", "Si el menú tiene más, sale en texto.") { fijar("lab_enc_max_opc", it.toString()) }
                NumeroGuardable("Caduca a los (minutos)", e.expiraMin, 1, 1440, "formato_enc_expira", "Pasado este tiempo, votar ya no mueve el menú.") { fijar("lab_expira_min", it.toString()) }
                Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text("Mandar también el texto con números")
                        Text("Debajo de la encuesta, por si no se ve.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    Switch(e.respaldoEnc, { fijar("lab_respaldo_enc", if (it) "1" else "0") }, Modifier.testTag("formato_enc_respaldo"))
                }
            }
            if (e.modo == "botones") Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text("Mandar también el texto con números")
                    Text("Debajo de los botones, por si no se ven.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                Switch(e.respaldoBot, { fijar("lab_respaldo_bot", if (it) "1" else "0") }, Modifier.testTag("formato_botones_respaldo"))
            }
        }
    }
}

/** Un número que se guarda con «Guardar» (solo aparece si cambió y es válido). */
@Composable
private fun NumeroGuardable(titulo: String, valor: Int, min: Int, max: Int, tag: String, ayuda: String, alGuardar: (Int) -> Unit) {
    var texto by remember(valor) { mutableStateOf(valor.toString()) }
    val n = texto.toIntOrNull()
    val valido = n != null && n in min..max
    Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            OutlinedTextField(texto, { texto = it.filter(Char::isDigit).take(4) }, Modifier.weight(1f).testTag(tag), label = { Text(titulo) }, singleLine = true, isError = !valido)
            if (texto != valor.toString() && valido) TextButton({ alGuardar(n!!) }, Modifier.testTag(tag + "_guardar")) { Text("Guardar") }
        }
        Text(if (valido) ayuda else "Va de $min a $max.", style = MaterialTheme.typography.bodySmall, color = if (valido) MaterialTheme.colorScheme.onSurfaceVariant else MaterialTheme.colorScheme.error)
    }
}
