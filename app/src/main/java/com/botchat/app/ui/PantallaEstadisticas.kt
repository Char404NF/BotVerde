@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.Repo
import com.botchat.app.datos.ResumenGlobal
import java.time.LocalDate

/** Estadísticas de toda la cuenta: totales, chats más activos y actividad de los últimos 30 días. */
@Composable
fun PantallaEstadisticas(botId: Long, alVolver: () -> Unit, alAbrirChat: (String) -> Unit) {
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if (evs.any { it == "chats" || it == "estadisticas" }) version++ } }
    val r by produceState<ResumenGlobal?>(null, version) { value = runCatching { Repo.estadisticasGlobales(botId, 30) }.getOrNull() }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Estadísticas") }) }) { relleno ->
        val datos = r
        if (datos == null) { Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }; return@Scaffold }
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            NotaDesde(datos.desde, datos.listo)
            if (datos.enviados + datos.recibidos == 0L) {
                EstadoVacio(Ico.Estadisticas, "Sin números todavía", "Cuando el bot guarde mensajes, aquí verás cuántos hay y qué chats son los más activos.", Modifier.padding(top = Esp.xl))
            } else {
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(Esp.m)) {
                    Cifra(datos.enviados, "Enviados", "Enviado", Modifier.weight(1f).testTag("cifra_enviados"))
                    Cifra(datos.recibidos, "Recibidos", "Recibido", Modifier.weight(1f).testTag("cifra_recibidos"))
                    Cifra(datos.chats, "Chats", "Chat", Modifier.weight(1f).testTag("cifra_chats"))
                }
                SeccionEst("Actividad de los últimos 30 días") {
                    val dias = datos.porDia
                    val etiquetas = dias.mapIndexed { i, d -> if (i % 7 == 0 || i == dias.lastIndex) LocalDate.parse(d.dia).dayOfMonth.toString() else "" }
                    GraficoBarras(dias.map { it.cantidad }, etiquetas, "Mensajes por día en los últimos 30 días. Total: ${dias.sumOf { it.cantidad }}", Modifier.testTag("grafico_dias"))
                }
                if (datos.activos.isNotEmpty()) {
                    SeccionEst("Chats más activos") {
                        val mayor = datos.activos.first().cantidad.coerceAtLeast(1)
                        datos.activos.forEach { c ->
                            Column(Modifier.fillMaxWidth().clickable { alAbrirChat(c.jid) }.testTag("activo_${c.jid}")) {
                                Row(verticalAlignment = Alignment.CenterVertically) {
                                    Avatar(c.nombre, c.foto, c.fotoTs, c.jid, 36.dp)
                                    Spacer(Modifier.width(Esp.m))
                                    Text(c.nombre, Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis)
                                    Text(numero(c.cantidad), color = MaterialTheme.colorScheme.onSurfaceVariant)
                                }
                                BarraRelativa(c.cantidad.toFloat() / mayor, Modifier.padding(top = 4.dp))
                            }
                        }
                    }
                }
                SeccionEst("Por tipo de mensaje") { datos.porTipo.forEach { FilaTipo(it) } }
            }
            Text(
                "Solo se cuentan datos que no son el contenido: quién, cuándo, tipo y tamaño. Los mensajes anteriores a esta versión se cuentan con la zona horaria actual del teléfono; si viajas o cambias de zona, esos días pueden correrse.",
                style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}
