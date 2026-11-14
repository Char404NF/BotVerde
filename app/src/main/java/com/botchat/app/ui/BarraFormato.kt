@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.layout.heightIn
import androidx.compose.material3.AssistChip
import com.botchat.app.ui.DropdownMenu
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.unit.dp

/** Las variables que se pueden poner en un mensaje automático. */
val VARIABLES_AUTO = listOf("{nombre}", "{hora}", "{dia}", "{fecha}", "{horario}")

private val EMOJIS = listOf("😀", "😊", "🙂", "😉", "👍", "🙏", "👋", "🤝", "✅", "⭐", "❤️", "🎉", "📞", "📍", "🕘", "📦", "💬", "🚚", "💳", "⏰", "🌟", "🙌", "😴", "🏖️")

/** Mete un texto en el lugar del cursor (o reemplaza la selección). */
fun insertarTexto(v: TextFieldValue, x: String): TextFieldValue {
    val ini = v.selection.min
    val fin = v.selection.max
    return TextFieldValue(v.text.replaceRange(ini, fin, x), TextRange(ini + x.length))
}

/**
 * Barra de formato: negrita, cursiva, tachado, monoespaciado, lista, lista numerada, cita, emoji y las variables. Aplica los marcadores
 * de WhatsApp (los mismos que dibuja FormatoWhatsApp) a lo seleccionado.
 */
@Composable
fun BarraFormato(valor: TextFieldValue, alCambiar: (TextFieldValue) -> Unit, tag: String = "fmt", conVariables: Boolean = true) {
    var emoji by remember { mutableStateOf(false) }
    Column(verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
        FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.xs), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
            BotonFormato("B", "Negrita", "${tag}_negrita", Modifier) { alCambiar(FormatoWhatsApp.aplicar(valor, "*")) }
            BotonFormato("I", "Cursiva", "${tag}_cursiva", Modifier) { alCambiar(FormatoWhatsApp.aplicar(valor, "_")) }
            BotonFormato("S", "Tachado", "${tag}_tachado", Modifier) { alCambiar(FormatoWhatsApp.aplicar(valor, "~")) }
            BotonFormato("</>", "Monoespaciado", "${tag}_mono", Modifier) { alCambiar(FormatoWhatsApp.aplicar(valor, "```")) }
            BotonFormato("• Lista", "Lista", "${tag}_lista", Modifier) { alCambiar(FormatoWhatsApp.aplicarLinea(valor, "- ")) }
            BotonFormato("1. Lista", "Lista numerada", "${tag}_numerada", Modifier) { alCambiar(FormatoWhatsApp.aplicarLinea(valor, "1. ")) }
            BotonFormato("❝ Cita", "Cita", "${tag}_cita", Modifier) { alCambiar(FormatoWhatsApp.aplicarLinea(valor, "> ")) }
            Box {
                BotonFormato("😀", "Emoji", "${tag}_emoji", Modifier) { emoji = true }
                DropdownMenu(emoji, { emoji = false }, Modifier.widthIn(max = 280.dp)) {
                    Column(Modifier.padding(Esp.s)) {
                        FlowRow(horizontalArrangement = Arrangement.spacedBy(2.dp)) {
                            EMOJIS.forEach { e -> TextButton({ emoji = false; alCambiar(insertarTexto(valor, e)) }, Modifier.testTag("${tag}_emoji_$e")) { Text(e, style = MaterialTheme.typography.titleLarge) } }
                        }
                    }
                }
            }
        }
        if (conVariables) {
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.xs), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                VARIABLES_AUTO.forEach { v -> AssistChip({ alCambiar(insertarTexto(valor, v)) }, { Text(v) }, Modifier.testTag("${tag}_var_${v.trim('{', '}')}")) }
            }
        }
    }
}

@Composable
private fun BotonFormato(texto: String, descripcion: String, tag: String, modifier: Modifier, alTocar: () -> Unit) {
    OutlinedButton(alTocar, modifier.testTag(tag), contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 10.dp, vertical = 4.dp)) {
        val estilo = when (descripcion) {
            "Negrita" -> androidx.compose.ui.text.SpanStyle(fontWeight = FontWeight.Bold)
            "Cursiva" -> androidx.compose.ui.text.SpanStyle(fontStyle = FontStyle.Italic)
            "Tachado" -> androidx.compose.ui.text.SpanStyle(textDecoration = TextDecoration.LineThrough)
            "Monoespaciado" -> androidx.compose.ui.text.SpanStyle(fontFamily = FontFamily.Monospace)
            else -> androidx.compose.ui.text.SpanStyle()
        }
        Text(androidx.compose.ui.text.buildAnnotatedString { pushStyle(estilo); append(texto); pop() }, Modifier.testTag("${tag}_t"))
    }
}
