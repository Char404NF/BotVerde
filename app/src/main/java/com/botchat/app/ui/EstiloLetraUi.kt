@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.material3.AssistChip
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.input.TextFieldValue
import com.botchat.app.ui.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.foundation.layout.padding

/** ¿Se muestra el botón “Aa” en los editores? (Avanzado → Estilo de letra; apagado por defecto). */
val LocalEstiloLetra = compositionLocalOf { false }

/** Los estilos de letra como chips: tocar uno convierte lo que esté seleccionado en el campo. Con el aviso visible siempre. */
@Composable
fun PanelEstiloLetra(valor: TextFieldValue, alCambiar: (TextFieldValue) -> Unit, tag: String = "estilo_letra") {
    val ctx = androidx.compose.ui.platform.LocalContext.current
    Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
        Text("Aa Estilo de letra", style = MaterialTheme.typography.titleSmall)
        Text("Selecciona un texto en el mensaje y elige un estilo. Solo cambia letras A–Z y números; los acentos, la ñ y las {variables} se quedan igual.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(EstiloLetraUnicode.AVISO, Modifier.testTag("${tag}_aviso"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
        FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
            EstiloLetra.values().forEach { e ->
                AssistChip({
                    if (valor.selection.collapsed) avisaToast(ctx, "Primero selecciona el texto que quieres cambiar.", true)
                    else alCambiar(EstiloLetraUnicode.aplicarASeleccion(valor, e))
                }, { Text(if (e == EstiloLetra.NORMAL) e.titulo else EstiloLetraUnicode.aplicar("Aa", e) + " " + e.titulo) }, Modifier.testTag("${tag}_${e.name.lowercase()}"))
            }
        }
    }
}

/** Botón “Aa” compacto con un menú de estilos y el aviso. Para los editores donde no cabe el panel completo. */
@Composable
fun BotonEstiloLetra(valor: TextFieldValue, alCambiar: (TextFieldValue) -> Unit, tag: String) {
    if (!LocalEstiloLetra.current) return
    val ctx = LocalContext.current
    var abierto by remember { mutableStateOf(false) }
    TextButton({ abierto = true }, Modifier.testTag(tag)) { Text("Aa") }
    DropdownMenu(abierto, { abierto = false }) {
        Text(EstiloLetraUnicode.AVISO, Modifier.padding(horizontal = Esp.l, vertical = Esp.s), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
        EstiloLetra.values().forEach { e ->
            DropdownMenuItem({ Text(if (e == EstiloLetra.NORMAL) e.titulo else EstiloLetraUnicode.aplicar("Aa", e) + " " + e.titulo) }, {
                abierto = false
                if (valor.selection.collapsed) avisaToast(ctx, "Primero selecciona el texto que quieres cambiar.", true)
                else alCambiar(EstiloLetraUnicode.aplicarASeleccion(valor, e))
            }, Modifier.testTag("${tag}_${e.name.lowercase()}"))
        }
    }
}

/** Un campo de texto que recuerda su selección y ofrece “Aa” (si está activado en Avanzado). Para los editores de menús y comandos. */
@Composable
fun CampoConEstilo(valor: String, etiqueta: String, tag: String, modifier: Modifier = Modifier, lineas: Int = 4, minLineas: Int = 1, alCambiar: (String) -> Unit) {
    var tf by remember { mutableStateOf(TextFieldValue(valor)) }
    if (tf.text != valor) tf = TextFieldValue(valor, tf.selection.let { if (it.max <= valor.length) it else androidx.compose.ui.text.TextRange(valor.length) })
    Column(modifier.fillMaxWidth()) {
        OutlinedTextField(tf, { tf = it; alCambiar(it.text) }, Modifier.fillMaxWidth().testTag(tag), label = { Text(etiqueta) }, minLines = minLineas, maxLines = lineas)
        BotonEstiloLetra(tf, { nuevo -> tf = nuevo; alCambiar(nuevo.text) }, "aa_$tag")
    }
}
