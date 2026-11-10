@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.ActividadItem
import com.botchat.app.datos.Repo

/** Frase corta de un renglón del registro: sin textos de mensajes ni nombres. */
fun frase(a: ActividadItem): String = when (a.evento) {
    "llego" -> if (a.detalle == "grupo") "Llegó un mensaje de un grupo" else "Llegó un mensaje privado"
    "coincidio" -> "Coincidió con: " + a.detalle.ifBlank { "una automatización" }
    "respondio" -> "Respondió"
    "sin_regla" -> "No coincidió con ninguna automatización"
    "manual" -> "Enviaste un mensaje a mano (no es el bot)"
    "freno" -> "El bot se pausó unos minutos por errores de envío"
    "omitido" -> "No respondió: " + when (a.motivo) {
        "duplicado" -> "ya se había visto ese mensaje"
        "antiguo" -> "mensaje antiguo o de sincronización"
        "limite" -> "límite de respuestas por minuto"
        "pausado" -> "bot en pausa en ese chat"
        "humano" -> "esperando a una persona"
        "motor apagado" -> "el motor está apagado"
        "grupo sin mención" -> "en el grupo no lo mencionaron"
        "chat en pausa" -> "bot en pausa en ese grupo"
        "repetido" -> "mensaje repetido"
        "cola llena" -> "había demasiados mensajes esperando"
        "freno" -> "el bot está pausado por errores"
        "no es texto" -> "no era un texto"
        else -> a.motivo.ifBlank { "sin motivo" }
    }
    else -> a.evento
}

/** Avanzado → Actividad del bot: lo que llegó, lo que coincidió y lo que se respondió u omitió. */
@Composable
fun PantallaActividad(botId: Long, alVolver: () -> Unit) {
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("actividad" in evs) version++ } }
    val lista by produceState<List<ActividadItem>?>(null, version) { value = runCatching { Repo.actividadMotor(botId) }.getOrDefault(emptyList()) }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Actividad del bot") }) }) { relleno ->
        val datos = lista
        when {
            datos == null -> Column(Modifier.fillMaxSize().padding(relleno), horizontalAlignment = Alignment.CenterHorizontally) { CircularProgressIndicator(Modifier.padding(Esp.xl)) }
            datos.isEmpty() -> EstadoVacio(Ico.Chats, "Sin actividad", "Cuando lleguen mensajes verás aquí qué hizo el bot con cada uno.", Modifier.padding(relleno).padding(top = 60.dp))
            else -> LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("lista_actividad")) {
                item { Text("Sin textos de mensajes ni nombres. Los chats aparecen con un código corto.", Modifier.padding(Esp.l), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                items(datos) { a ->
                    Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s)) {
                        Text("${etiquetaDia(a.ts)} ${horaDe(a.ts)}", Modifier.width(96.dp), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        Column(Modifier.weight(1f)) {
                            Text(frase(a), style = MaterialTheme.typography.bodyMedium)
                            if (a.chat.isNotBlank()) Text("Chat ${a.chat}", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                    HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                }
            }
        }
    }
}
