@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Switch
import androidx.compose.material3.TextButton
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.EstadoAvatar
import com.botchat.app.datos.Repo
import com.botchat.app.datos.avatarPropio
import com.botchat.app.datos.avatarPropioDe
import com.botchat.app.datos.configurarAvatar
import com.botchat.app.datos.estadoAvatar
import com.botchat.app.medios.PrepararAdjunto
import kotlinx.coroutines.launch

/**
 * Bot → Avatar del bot. Enciende o apaga la imagen del avatar, elige en qué respuestas sale (con el texto de pie de foto) y deja usar una
 * imagen propia en lugar de las 8 caras de fábrica.
 */
@Composable
fun PantallaAvatar(botId: Long, alVolver: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var e by remember { mutableStateOf<EstadoAvatar?>(null) }
    var ocupado by remember { mutableStateOf(false) }
    suspend fun recarga() { e = runCatching { Repo.estadoAvatar(botId) }.getOrNull() }
    LaunchedEffect(botId) { recarga() }

    fun guarda(activo: Boolean, ids: List<String>) {
        scope.launch {
            runCatching { Repo.configurarAvatar(botId, activo, ids) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo guardar", true) }
            recarga()
        }
    }

    var paraMomento by remember { mutableStateOf("") }
    val elegir = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        if (uri != null) {
            val momento = paraMomento
            ocupado = true
            scope.launch {
                PrepararAdjunto.desdeUri(ctx, uri).onSuccess { listo ->
                    if (listo.tipo != "imagen") avisaToast(ctx, "El avatar tiene que ser una imagen.", true)
                    else runCatching {
                        val a = Repo.subirAdjunto(botId, listo.archivo.path, listo.tipo, listo.mime, listo.nombre)
                        Repo.avatarPropioDe(botId, momento, a.id, listo.mime)
                    }.onFailure { avisaToast(ctx, it.message ?: "No se pudo guardar la imagen", true) }
                }.onFailure { avisaToast(ctx, it.message ?: "No se pudo leer el archivo", true) }
                recarga()
                ocupado = false
            }
        }
    }

    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver, Modifier.testTag("volver_avatar")) { Icon(Ico.Atras, "Volver") } }, title = { Text("Avatar del bot") }) }) { relleno ->
        val s = e
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l).testTag("pantalla_avatar"), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            if (s == null) {
                Text("Cargando…")
            } else {
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Esp.m)) {
                    AvatarAnimo("feliz", Modifier.size(72.dp))
                    Text("El bot manda su imagen en algunas respuestas, y el texto va de pie de foto. Así el menú y los avisos se ven más llamativos sin mensajes de más.", style = MaterialTheme.typography.bodyMedium)
                }
                Card(Modifier.fillMaxWidth()) {
                    Row(Modifier.padding(Esp.l), verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) {
                            Text("Mandar la imagen del avatar", style = MaterialTheme.typography.titleMedium)
                            Text(if (s.activo) "Encendido: sale en los momentos de abajo." else "Apagado: todo sale solo con texto.", style = MaterialTheme.typography.bodySmall)
                        }
                        Switch(s.activo, { guarda(it, s.momentos.filter { m -> m.activo }.map { m -> m.id }) }, Modifier.testTag("avatar_activo"))
                    }
                }
                Text("¿En qué respuestas sale?", style = MaterialTheme.typography.titleSmall)
                s.momentos.forEach { m ->
                    Card(Modifier.fillMaxWidth()) {
                        Row(Modifier.padding(Esp.m), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Esp.m)) {
                            if (!s.propio && !m.propio) AvatarAnimo(m.cara, Modifier.size(48.dp))
                            Column(Modifier.weight(1f)) {
                                Text(m.nombre, style = MaterialTheme.typography.bodyLarge)
                                Text(if (m.grupos) "En privado y en grupos" else "Solo en chats privados", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                                    TextButton({ paraMomento = m.id; elegir.launch(arrayOf("image/*")) }, Modifier.testTag("avatar_imagen_${m.id}"), enabled = !ocupado) { Text(if (m.propio) "Cambiar su imagen" else "Imagen solo para esto") }
                                    if (m.propio) TextButton({ scope.launch { runCatching { Repo.avatarPropioDe(botId, m.id, "", "") }; recarga() } }, Modifier.testTag("avatar_quitar_${m.id}"), enabled = !ocupado) { Text("Quitar") }
                                }
                            }
                            Switch(
                                m.activo,
                                { nuevo -> guarda(s.activo, s.momentos.filter { x -> if (x.id == m.id) nuevo else x.activo }.map { x -> x.id }) },
                                Modifier.testTag("avatar_momento_${m.id}"), enabled = s.activo,
                            )
                        }
                    }
                }
                Text("Tu propia imagen", style = MaterialTheme.typography.titleSmall)
                Text(
                    if (s.propio) "Ahora el bot usa tu imagen en todos los momentos (menos en los que tengan la suya)." else "Ahora usa las 8 caras del avatar, una distinta según el momento (feliz, enojado, pensando…). Puedes poner tu propia imagen, por ejemplo el logo de tu grupo.",
                    style = MaterialTheme.typography.bodySmall,
                )
                Button({ paraMomento = ""; elegir.launch(arrayOf("image/*")) }, Modifier.fillMaxWidth().testTag("avatar_elegir"), enabled = !ocupado) { Text(if (s.propio) "Cambiar mi imagen" else "Elegir mi imagen") }
                if (s.propio) {
                    OutlinedButton(
                        { scope.launch { runCatching { Repo.avatarPropio(botId, "", "") }; recarga() } },
                        Modifier.fillMaxWidth().testTag("avatar_quitar"), enabled = !ocupado,
                    ) { Text("Volver a las caras del avatar") }
                }
            }
        }
    }
}
