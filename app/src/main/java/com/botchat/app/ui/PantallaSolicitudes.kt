@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

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
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.Repo
import com.botchat.app.datos.SolicitudItem
import kotlinx.coroutines.launch

private val ESTADOS = listOf("pendiente" to "Pendientes", "ignorada" to "Ignoradas", "bloqueada" to "Bloqueadas", "aprobada" to "Aprobadas")

/** Número legible a partir de la clave de una persona ("5215512345678@s.whatsapp.net" → "+5215512345678"). Nunca muestra un LID. */
internal fun numeroDePersona(persona: String): String {
    val u = persona.substringBefore('@')
    return if (persona.endsWith("@s.whatsapp.net") && u.all { it.isDigit() }) "+$u" else "Número no disponible"
}

/**
 * Solicitudes: personas desconocidas que escribieron mientras la política pide tu aprobación. Aprobar deja que el bot les conteste en
 * privado (NO autoriza grupos); Bloquear hace que el bot las ignore del todo; Ignorar las descarta sin bloquear.
 */
@Composable
fun PantallaSolicitudes(botId: Long, alVolver: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    var estado by rememberSaveable { mutableStateOf("pendiente") }
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("solicitudes" in evs) version++ } }
    val lista by produceState<List<SolicitudItem>?>(null, estado, version) { value = runCatching { Repo.solicitudes(botId, estado) }.getOrDefault(emptyList()) }
    val nombres by produceState<Map<String, String>>(emptyMap(), lista) {
        value = lista.orEmpty().associate { s -> s.persona to (s.nombre.ifBlank { runCatching { Repo.chat(botId, s.persona)?.nombre }.getOrNull().orEmpty() }) }
    }
    fun hacer(f: suspend () -> Unit) = scope.launch { runCatching { f() }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }; version++ }

    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Solicitudes") }) }) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno)) {
            Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s), horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                ESTADOS.forEach { (e, t) -> FilterChip(estado == e, { haptica(Toque.Suave); estado = e }, { Text(t, maxLines = 1) }, modifier = Modifier.testTag("solicitudes_$e")) }
            }
            Text(
                when (estado) {
                    "pendiente" -> "Escribieron y todavía no sabes quiénes son. El bot no les ha contestado (salvo el aviso “te contestamos pronto” si lo elegiste)."
                    "ignorada" -> "Descartadas sin bloquear: si vuelven a escribir pasados unos días, regresan a Pendientes."
                    "bloqueada" -> "El bot no les contesta nunca, ni en privado ni en grupos."
                    else -> "El bot les contesta en privado. En los grupos aplica la política de cada grupo."
                },
                Modifier.padding(horizontal = Esp.l), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            val datos = lista
            when {
                datos == null -> Box(Modifier.fillMaxSize(), Alignment.Center) { CircularProgressIndicator() }
                datos.isEmpty() -> EstadoVacio(Ico.Persona, "Nada por aquí", if (estado == "pendiente") "Cuando un desconocido escriba, aparecerá aquí para que decidas." else "Todavía no hay personas en esta lista.", Modifier.padding(top = 60.dp))
                else -> LazyColumn(Modifier.fillMaxSize().testTag("lista_solicitudes"), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
                    items(datos, key = { it.persona }) { s ->
                        Card(Modifier.fillMaxWidth().testTag("solicitud_${s.persona}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                            Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                                Row(verticalAlignment = Alignment.CenterVertically) {
                                    Avatar(nombres[s.persona].orEmpty(), "", 0, s.persona, 44.dp)
                                    Column(Modifier.padding(start = Esp.m).weight(1f)) {
                                        Text(nombres[s.persona].orEmpty().ifBlank { numeroDePersona(s.persona) }, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                        Text(numeroDePersona(s.persona) + " · " + plural(s.mensajes.toLong(), "mensaje", "mensajes") + " · " + etiquetaDia(s.ultimaTs) + " " + horaDe(s.ultimaTs), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                    }
                                }
                                Row(horizontalArrangement = Arrangement.spacedBy(Esp.s), modifier = Modifier.fillMaxWidth()) {
                                    if (estado != "aprobada") Button({ haptica(Toque.Suave); hacer { Repo.aprobarSolicitud(botId, s.persona) } }, Modifier.weight(1f).testTag("aprobar_${s.persona}"), contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 8.dp)) { Text("Aprobar", maxLines = 1) }
                                    if (estado != "bloqueada") OutlinedButton({ haptica(Toque.Fuerte); hacer { Repo.bloquearSolicitud(botId, s.persona) } }, Modifier.weight(1f).testTag("bloquear_${s.persona}"), colors = ButtonDefaults.outlinedButtonColors(contentColor = MaterialTheme.colorScheme.error), contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 8.dp)) { Text("Bloquear", maxLines = 1) }
                                    if (estado == "pendiente") OutlinedButton({ haptica(Toque.Suave); hacer { Repo.ignorarSolicitud(botId, s.persona) } }, Modifier.weight(1f).testTag("ignorar_${s.persona}"), contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 8.dp)) { Text("Ignorar", maxLines = 1) }
                                    if (estado != "pendiente") TextButton({ haptica(Toque.Suave); hacer { Repo.quitarSolicitud(botId, s.persona) } }, Modifier.testTag("quitar_${s.persona}")) { Text("Quitar de la lista", maxLines = 1) }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}
