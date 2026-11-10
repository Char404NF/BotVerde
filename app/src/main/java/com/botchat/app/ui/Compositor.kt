@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledIconButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.Msg
import com.botchat.app.datos.Participante

private val regexMencion = Regex("""(?:^|\s)@([^\s@]*)$""")

/** Quita acentos y mayúsculas para buscar a alguien por su nombre. */
private fun plano(s: String) = com.botchat.app.Paises.normaliza(s)

/** Caja de mensaje: texto con formato, cita, edición, @menciones en grupos y botón de enviar. */
@Composable
fun Compositor(
    valor: TextFieldValue,
    alCambiar: (TextFieldValue) -> Unit,
    citando: Msg?,
    alQuitarCita: () -> Unit,
    editando: Msg?,
    alCancelarEdicion: () -> Unit,
    esGrupo: Boolean,
    participantes: List<Participante>,
    enviando: Boolean,
    alEnviar: (texto: String, menciones: List<String>) -> Unit,
    modifier: Modifier = Modifier,
) {
    val haptica = rememberHaptica()
    val elegidas = remember { mutableStateMapOf<String, Participante>() } // "@Nombre" → a quién se mencionó
    var formato by remember { mutableStateOf(false) }

    val antes = valor.text.substring(0, valor.selection.start.coerceIn(0, valor.text.length))
    val consulta = if (esGrupo) regexMencion.find(antes)?.groupValues?.get(1) else null
    val candidatos = if (consulta == null) emptyList() else participantes.filter { consulta.isEmpty() || plano(it.nombre).contains(plano(consulta)) }.take(6)

    Surface(modifier.fillMaxWidth().imePadding().navigationBarsPadding(), color = MaterialTheme.colorScheme.surfaceContainer) {
        Column {
            if (candidatos.isNotEmpty()) {
                Column(Modifier.fillMaxWidth().background(MaterialTheme.colorScheme.surfaceContainerHigh).testTag("lista_menciones")) {
                    candidatos.forEach { p ->
                        Row(Modifier.fillMaxWidth().clickable {
                            haptica(Toque.Suave)
                            val inicio = antes.length - consulta!!.length - 1
                            val token = "@" + p.nombre
                            val nuevo = valor.text.substring(0, inicio) + token + " " + valor.text.substring(valor.selection.start.coerceIn(0, valor.text.length))
                            elegidas[token] = p
                            alCambiar(TextFieldValue(nuevo, TextRange(inicio + token.length + 1)))
                        }.padding(horizontal = Esp.l, vertical = Esp.s).testTag("mencion_${p.usuario}"), verticalAlignment = Alignment.CenterVertically) {
                            Text(p.nombre, Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis)
                            if (p.telefono.isNotBlank() && p.telefono != p.nombre) Text(p.telefono, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                }
            }
            if (editando != null) {
                Banner("Editando mensaje", editando.texto, "banner_edicion", alCancelarEdicion)
            } else if (citando != null) {
                Banner("Respondiendo a " + (if (citando.esMio) "ti" else citando.remitente.ifBlank { "este mensaje" }), descripcionCorta(citando.tipo, citando.texto, false, citando.notaVoz), "banner_cita", alQuitarCita)
            }
            if (formato) {
                Row(Modifier.fillMaxWidth().padding(horizontal = Esp.m), horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                    listOf("*" to "Negrita", "_" to "Cursiva", "~" to "Tachado", "```" to "Código").forEach { (m, nombre) ->
                        TextButton({ haptica(Toque.Suave); alCambiar(FormatoWhatsApp.aplicar(valor, m)) }, Modifier.testTag("formato_$nombre")) { Text(nombre, fontWeight = FontWeight.SemiBold) }
                    }
                }
            }
            Row(Modifier.fillMaxWidth().padding(horizontal = Esp.s, vertical = Esp.xs), verticalAlignment = Alignment.Bottom) {
                IconButton({ haptica(Toque.Suave); formato = !formato }, Modifier.testTag("boton_formato")) { Icon(Ico.Editar, "Formato del texto") }
                OutlinedTextField(
                    valor, alCambiar, Modifier.weight(1f).testTag("campo_mensaje"), placeholder = { Text("Escribe un mensaje") },
                    shape = RoundedCornerShape(24.dp), maxLines = 5,
                )
                Spacer(Modifier.width(6.dp))
                FilledIconButton(
                    {
                        haptica(Toque.Suave)
                        var texto = valor.text.trim()
                        val menciones = mutableListOf<String>()
                        elegidas.entries.sortedByDescending { it.key.length }.forEach { (token, p) ->
                            if (texto.contains(token)) { texto = texto.replace(token, "@" + p.usuario); if (p.jid !in menciones) menciones += p.jid }
                        }
                        alEnviar(texto, menciones)
                        elegidas.clear()
                    },
                    Modifier.size(48.dp).testTag("enviar"), enabled = valor.text.isNotBlank() && !enviando,
                ) { Icon(if (editando != null) Ico.Listo else Ico.Enviar, if (editando != null) "Guardar cambio" else "Enviar") }
            }
        }
    }
}

@Composable
private fun Banner(titulo: String, texto: String, etiqueta: String, alCerrar: () -> Unit) {
    Row(Modifier.fillMaxWidth().background(MaterialTheme.colorScheme.surfaceContainerHigh).padding(start = Esp.l, top = 4.dp, bottom = 4.dp).testTag(etiqueta), verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.width(4.dp).size(width = 4.dp, height = 34.dp).background(MaterialTheme.colorScheme.primary))
        Column(Modifier.weight(1f).padding(start = 8.dp)) {
            Text(titulo, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.primary)
            Text(texto, style = MaterialTheme.typography.bodySmall, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
        IconButton(alCerrar, Modifier.testTag("cerrar_$etiqueta")) { Icon(Ico.Cerrar, "Cancelar") }
    }
}
