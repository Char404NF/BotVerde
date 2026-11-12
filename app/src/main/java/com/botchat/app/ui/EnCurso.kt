@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.Recorrido
import com.botchat.app.datos.Repo
import com.botchat.app.datos.SesionVivo
import kotlinx.coroutines.launch

/** "hace 3 min" / "hace 2 h". */
fun haceCuanto(ts: Long, ahora: Long = System.currentTimeMillis() / 1000): String {
    val s = (ahora - ts).coerceAtLeast(0)
    return when {
        s < 60 -> "hace un momento"
        s < 3600 -> "hace ${s / 60} min"
        s < 86400 -> "hace ${s / 3600} h"
        else -> "hace ${s / 86400} d"
    }
}

private fun textoEstado(e: String) = when (e) {
    "humano" -> "Esperando a una persona"
    "pausado" -> "Bot en pausa con esta persona"
    else -> "En el menú"
}

/** En curso ahora: quién está dentro de qué menú, y hace cuánto. */
@Composable
fun PantallaEnCurso(botId: Long, alVolver: () -> Unit, alAbrir: (Long) -> Unit) {
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("sesiones" in evs || "automatizaciones" in evs) version++ } }
    val lista by produceState<List<SesionVivo>?>(null, version) { value = runCatching { Repo.sesionesEnCurso(botId) }.getOrDefault(emptyList()) }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("En curso ahora") }) }) { relleno ->
        val datos = lista
        when {
            datos == null -> Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }
            datos.isEmpty() -> EstadoVacio(Ico.Chats, "Nadie en un menú", "Cuando alguien entre a un menú, aparecerá aquí con su recorrido.", Modifier.padding(relleno).padding(top = 60.dp))
            else -> LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("lista_en_curso")) {
                items(datos, key = { it.id }) { s ->
                    Column(Modifier.fillMaxWidth().clickable { alAbrir(s.id) }.padding(horizontal = Esp.l, vertical = Esp.m).testTag("sesion_${s.id}")) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(s.nombre.ifBlank { "Contacto" }, Modifier.weight(1f), style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            Text(haceCuanto(s.ultima), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                        Text(s.ruta.ifBlank { s.menu }, style = MaterialTheme.typography.bodyMedium)
                        Text(textoEstado(s.estado), style = MaterialTheme.typography.labelMedium, color = if (s.estado == "activo") MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.tertiary)
                    }
                    HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                }
            }
        }
    }
}

/** Recorrido de una persona dentro del menú, con botones para tomar el control. Ninguno manda mensajes: el bot solo responde. */
@Composable
fun PantallaRecorrido(botId: Long, sesionId: Long, alVolver: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    val r by produceState<Recorrido?>(null, version) { value = runCatching { Repo.recorrido(botId, sesionId) }.getOrNull() }
    fun accion(a: String, aviso: String) = scope.launch {
        runCatching { Repo.accionSesion(botId, sesionId, a) }.onSuccess { avisaToast(ctx, aviso, true); version++ }.onFailure { avisaToast(ctx, it.message, true) }
    }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text(r?.persona?.ifBlank { null } ?: r?.sesion?.nombre?.ifBlank { null } ?: "Recorrido") }) }) { relleno ->
        val d = r
        if (d == null) { Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }; return@Scaffold }
        Column(Modifier.fillMaxSize().padding(relleno).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Text("${d.sesion.menu} · ${textoEstado(d.sesion.estado)} · ${haceCuanto(d.sesion.ultima)}", style = MaterialTheme.typography.bodyMedium)
            // El recorrido se ve como una conversación de WhatsApp: el menú del bot a la izquierda y lo que eligió la persona a la derecha,
            // citando el menú al que respondió.
            LazyColumn(Modifier.weight(1f).testTag("recorrido"), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                items(d.pasos.size) { i ->
                    val p = d.pasos[i]
                    when (p.resultado) {
                        "mostrado" -> BurbujaDeTexto(true, p.titulo, hora = horaDe(p.ts))
                        "valida" ->
                            if (p.opcion == "dato") BurbujaDeTexto(false, "Respondió", cita = p.titulo, hora = horaDe(p.ts))
                            else BurbujaDeTexto(false, p.opcion, cita = p.titulo, hora = horaDe(p.ts))
                        "invalida" -> BurbujaDeTexto(false, "Respuesta no válida", cita = p.titulo, hora = horaDe(p.ts))
                        else -> BurbujaDeTexto(true, p.titulo, hora = horaDe(p.ts))
                    }
                }
            }
            Text("Estos botones no mandan ningún mensaje.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                OutlinedButton({ haptica(Toque.Suave); accion("reiniciar", "Su menú empezará de nuevo cuando escriba") }, Modifier.weight(1f).testTag("reiniciar_menu")) { Text("Reiniciar su menú") }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                OutlinedButton({ haptica(Toque.Suave); accion("humano", "El bot calla con esta persona 12 horas") }, Modifier.weight(1f).testTag("pasar_humano")) { Text("Pasar a humano") }
                Button({ haptica(Toque.Suave); accion("pausar", "El bot calla con esta persona 12 horas") }, Modifier.weight(1f).testTag("pausar_persona")) { Text("Pausar bot con esta persona") }
            }
        }
    }
}
