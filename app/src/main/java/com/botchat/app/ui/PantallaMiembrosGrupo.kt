@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import com.botchat.app.ui.AlertDialog
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.EstadoGrupo
import com.botchat.app.datos.MiembroGrupo
import com.botchat.app.datos.Repo
import com.botchat.app.datos.SolicitudGrupo
import kotlinx.coroutines.launch

/** Miembros del grupo (ver quién es administrador y, siendo administrador, subir, bajar o sacar) y solicitudes de ingreso. */
@Composable
fun PantallaMiembrosGrupo(botId: Long, jid: String, pestanaInicial: String = "miembros", alVolver: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var pestana by rememberSaveable { mutableStateOf(pestanaInicial) }
    var version by remember { mutableIntStateOf(0) }
    val estado by produceState<EstadoGrupo?>(null, jid, version) { value = runCatching { Repo.estadoGrupo(botId, jid, version > 0) }.getOrElse { EstadoGrupo(false, false, false, textoError(it)) } }
    val miembros by produceState<Result<List<MiembroGrupo>>?>(null, jid, version) { value = runCatching { Repo.miembrosGrupo(botId, jid) } }
    val solicitudes by produceState<Result<List<SolicitudGrupo>>?>(null, jid, version, pestana, estado?.soyAdmin) {
        value = if (pestana == "solicitudes" && estado?.soyAdmin == true) runCatching { Repo.solicitudesGrupo(botId, jid) } else null
    }
    val admin = estado?.soyAdmin == true
    var expulsando by remember { mutableStateOf<MiembroGrupo?>(null) }
    fun hacer(ok: String, accion: suspend () -> Unit) = scope.launch {
        runCatching { accion() }.onSuccess { avisaToast(ctx, ok, false) }.onFailure { avisaToast(ctx, textoError(it), true) }
        version++
    }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Gente del grupo") }) }) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            TarjetaEstadoAdmin(estado) { haptica(Toque.Suave); version++ }
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                FilterChip(pestana == "miembros", { pestana = "miembros" }, { Text("Miembros") }, modifier = Modifier.testTag("pestana_miembros"))
                FilterChip(pestana == "solicitudes", { pestana = "solicitudes" }, { Text("Solicitudes") }, modifier = Modifier.testTag("pestana_solicitudes"))
            }
            if (pestana == "miembros") {
                val m = miembros
                when {
                    m == null -> CircularProgressIndicator(Modifier.size(24.dp))
                    m.isFailure -> Text(textoError(m.exceptionOrNull()!!), Modifier.testTag("miembros_error"), color = MaterialTheme.colorScheme.onSurfaceVariant)
                    else -> {
                        AvisoAdmin(!admin)
                        m.getOrThrow().forEach { x -> FilaMiembro(x, admin, { haptica(Toque.Suave); hacer(if (x.esAdmin) "Listo: ya no es administradora" else "Listo: ahora es administradora") { Repo.ascenderEnGrupo(botId, jid, x.jid, !x.esAdmin) } }, { haptica(Toque.Fuerte); expulsando = x }) }
                    }
                }
            } else {
                if (!admin) {
                    Text(if (estado == null) "…" else "No puedo ver ni resolver las solicitudes. $TEXTO_NECESITA_ADMIN", Modifier.testTag("solicitudes_sin_admin"), color = MaterialTheme.colorScheme.onSurfaceVariant)
                } else {
                    val s = solicitudes
                    when {
                        s == null -> CircularProgressIndicator(Modifier.size(24.dp))
                        s.isFailure -> Text(textoError(s.exceptionOrNull()!!), Modifier.testTag("solicitudes_error"))
                        s.getOrThrow().isEmpty() -> EstadoVacio(Ico.Persona, "Sin solicitudes", "Cuando alguien pida entrar al grupo aparecerá aquí.", Modifier.testTag("solicitudes_vacio"))
                        else -> {
                            val lista = s.getOrThrow()
                            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                                Button({ haptica(Toque.Suave); hacer("Listo: aprobé a todas") { Repo.resolverSolicitudesGrupo(botId, jid, emptyList(), true) } }, Modifier.testTag("aprobar_todas")) { Text("Aprobar todas") }
                                OutlinedButton({ haptica(Toque.Fuerte); hacer("Listo: rechacé a todas") { Repo.resolverSolicitudesGrupo(botId, jid, emptyList(), false) } }, Modifier.testTag("rechazar_todas")) { Text("Rechazar todas") }
                            }
                            lista.forEach { x ->
                                Card(Modifier.fillMaxWidth().testTag("solicitud_${x.jid}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                                    Row(Modifier.padding(Esp.m), verticalAlignment = Alignment.CenterVertically) {
                                        Column(Modifier.weight(1f)) {
                                            Text(x.nombre, maxLines = 1, overflow = TextOverflow.Ellipsis, style = MaterialTheme.typography.bodyLarge)
                                            if (x.telefono.isNotBlank() && x.telefono != x.nombre) Text(x.telefono, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                            if (x.ts > 0) Text("Pidió entrar ${etiquetaDia(x.ts)}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                        }
                                        TextButton({ haptica(Toque.Suave); hacer("Listo: aprobada") { Repo.resolverSolicitudesGrupo(botId, jid, listOf(x.jid), true) } }, Modifier.testTag("aprobar_${x.jid}")) { Text("Aprobar") }
                                        TextButton({ haptica(Toque.Fuerte); hacer("Listo: rechazada") { Repo.resolverSolicitudesGrupo(botId, jid, listOf(x.jid), false) } }, Modifier.testTag("rechazar_${x.jid}")) { Text("Rechazar", color = MaterialTheme.colorScheme.error) }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
    expulsando?.let { x ->
        AlertDialog(
            onDismissRequest = { expulsando = null }, title = { Text("¿Sacar a ${x.nombre} del grupo?") },
            text = { Text("Sale del grupo ahora mismo. Podrá volver solo si alguien la agrega o entra con el enlace.") },
            confirmButton = { TextButton({ expulsando = null; hacer("Listo: salió del grupo") { Repo.expulsarDeGrupo(botId, jid, x.jid) } }, Modifier.testTag("confirmar_expulsar")) { Text("Sacar del grupo", color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton({ expulsando = null }) { Text("Cancelar") } },
        )
    }
}

@Composable
private fun FilaMiembro(x: MiembroGrupo, admin: Boolean, alAscender: () -> Unit, alExpulsar: () -> Unit) {
    Card(Modifier.fillMaxWidth().testTag("miembro_${x.jid}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.m), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text(x.nombre, maxLines = 1, overflow = TextOverflow.Ellipsis, style = MaterialTheme.typography.bodyLarge, fontWeight = FontWeight.Medium)
                    if (x.telefono.isNotBlank() && x.telefono != x.nombre) Text(x.telefono, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                val etiqueta = when { x.yo -> "Yo (este bot)"; x.esCreador -> "Creó el grupo"; x.esAdmin -> "Administrador"; else -> "" }
                if (etiqueta.isNotEmpty()) Text(etiqueta, Modifier.testTag("rol_${x.jid}"), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.primary)
            }
            if (!x.yo && !x.esCreador) {
                Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    OutlinedButton(alAscender, Modifier.testTag("ascender_${x.jid}"), enabled = admin) { Text(if (x.esAdmin) "Quitar administrador" else "Hacer administrador") }
                    OutlinedButton(alExpulsar, Modifier.testTag("expulsar_${x.jid}"), enabled = admin, colors = ButtonDefaults.outlinedButtonColors(contentColor = MaterialTheme.colorScheme.error)) { Text("Sacar") }
                }
            }
        }
    }
}
