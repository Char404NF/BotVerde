@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.heightIn
import androidx.compose.ui.draw.clip
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledIconButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.Repo
import kotlinx.coroutines.launch

private data class BurbujaSim(val esBot: Boolean, val texto: String)

/** Simulador: escribe mensajes falsos como si fueras otra persona y mira qué contestaría el bot. No se manda nada a WhatsApp. */
@Composable
fun PantallaSimulador(botId: Long, menuId: Long, nombre: String, alVolver: () -> Unit) {
    val scope = rememberCoroutineScope()
    val burbujas = remember { mutableStateListOf<BurbujaSim>() }
    var texto by remember { mutableStateOf("") }
    var primera by remember { mutableStateOf(true) }
    var nota by remember { mutableStateOf("") }
    val lista = rememberLazyListState()
    LaunchedEffect(burbujas.size) { if (burbujas.isNotEmpty()) lista.animateScrollToItem(burbujas.lastIndex) }

    fun manda(t: String, reiniciar: Boolean) {
        scope.launch {
            burbujas += BurbujaSim(false, t)
            runCatching { Repo.simular(botId, menuId, t, reiniciar) }.onSuccess { p ->
                if (reiniciar || primera) nota = if (p.coincide) "Este mensaje sí activaría esta automatización." else "Con este mensaje NO se activaría sola (aquí se fuerza el inicio para que pruebes)."
                primera = false
                if (p.salidas.isEmpty()) burbujas += BurbujaSim(true, if (p.omitido.isNotBlank()) "(El bot se quedaría callado: ${p.omitido})" else "(El bot no contestaría)")
                p.salidas.forEach { burbujas += BurbujaSim(true, it) }
                if (p.estado != "activo") burbujas += BurbujaSim(true, "— Fin de la conversación (${when (p.estado) { "humano" -> "pasó a una persona"; "abandonado" -> "se cortó"; else -> "completada" }}) —")
            }.onFailure { burbujas += BurbujaSim(true, "Error: ${it.message}") }
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } },
                title = { Column { Text("Probar: $nombre"); Text("Modo prueba · no se envía nada", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.tertiary) } },
                actions = { TextButton({ burbujas.clear(); primera = true; nota = ""; scope.launch { runCatching { Repo.simular(botId, menuId, "", true) } } }, Modifier.testTag("reiniciar_simulador")) { Text("Reiniciar") } },
            )
        },
        bottomBar = {
            Row(Modifier.fillMaxWidth().padding(Esp.s), verticalAlignment = Alignment.Bottom) {
                OutlinedTextField(texto, { texto = it }, Modifier.weight(1f).testTag("campo_simulador"), placeholder = { Text("Escribe como si fueras un cliente") }, shape = RoundedCornerShape(24.dp), maxLines = 3)
                FilledIconButton({ val t = texto.trim(); if (t.isNotEmpty()) { texto = ""; manda(t, primera) } }, Modifier.padding(start = 6.dp).testTag("enviar_simulador"), enabled = texto.isNotBlank()) { Icon(Ico.Enviar, "Enviar") }
            }
        },
    ) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno)) {
            if (nota.isNotBlank()) Text(nota, Modifier.fillMaxWidth().background(MaterialTheme.colorScheme.tertiaryContainer).padding(Esp.m).testTag("nota_simulador"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onTertiaryContainer)
            if (burbujas.isEmpty()) Box(Modifier.fillMaxSize().padding(Esp.xl), Alignment.Center) {
                Text("Escribe un mensaje abajo para ver cómo responde esta automatización. Las respuestas aparecen aquí; los datos que escribas se borran.", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            } else LazyColumn(Modifier.fillMaxSize().testTag("conversacion_simulador"), state = lista, contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.m), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                items(burbujas.size) { i -> BurbujaDeTexto(burbujas[i].esBot, burbujas[i].texto) }
            }
        }
    }
}

/**
 * Burbuja de conversación al estilo WhatsApp para el simulador, la vista previa y el recorrido de un menú: las del bot a la izquierda,
 * las de la persona a la derecha. `cita` es el mensaje al que responde (el recuadro de arriba, con su barra de color) y `hora` una
 * hora chiquita al pie. Los números de las opciones van dentro del texto del menú ("1. Precios").
 */
@Composable
fun BurbujaDeTexto(esBot: Boolean, texto: String, cita: String? = null, hora: String? = null) {
    Row(Modifier.fillMaxWidth(), horizontalArrangement = if (esBot) Arrangement.Start else Arrangement.End) {
        Surface(
            shape = RoundedCornerShape(16.dp), color = if (esBot) MaterialTheme.colorScheme.surfaceContainerHigh else MaterialTheme.colorScheme.primaryContainer,
            modifier = Modifier.widthIn(max = 300.dp).testTag(if (esBot) "burbuja_bot" else "burbuja_usuario"),
        ) {
            Column(Modifier.padding(horizontal = 12.dp, vertical = 8.dp)) {
                if (!cita.isNullOrBlank()) {
                    Row(Modifier.padding(bottom = 6.dp).clip(RoundedCornerShape(8.dp)).background(MaterialTheme.colorScheme.surface.copy(alpha = 0.55f)).testTag("burbuja_cita")) {
                        Box(Modifier.width(4.dp).heightIn(min = 28.dp).background(MaterialTheme.colorScheme.primary))
                        Text(cita, Modifier.padding(horizontal = 8.dp, vertical = 4.dp), style = MaterialTheme.typography.bodySmall, maxLines = 2, overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }
                Text(FormatoWhatsApp.construir(texto))
                if (!hora.isNullOrBlank()) Text(hora, Modifier.align(Alignment.End), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }
}
