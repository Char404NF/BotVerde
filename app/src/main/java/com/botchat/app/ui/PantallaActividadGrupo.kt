@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.InactivosGrupo
import com.botchat.app.datos.RankingGrupo
import com.botchat.app.datos.Repo

private val PERIODOS = listOf(7, 30, 90)

/** Actividad del grupo desde los contadores guardados (nunca se lee el texto): quién escribe más y quién no escribe hace tiempo. */
@Composable
fun PantallaActividadGrupo(botId: Long, jid: String, alVolver: () -> Unit) {
    var dias by rememberSaveable { mutableIntStateOf(30) }
    var pestana by rememberSaveable { mutableStateOf("ranking") }
    val haptica = rememberHaptica()
    val ranking by produceState<Result<RankingGrupo>?>(null, jid, dias, pestana) { value = if (pestana == "ranking") runCatching { Repo.rankingGrupo(botId, jid, dias, 30) } else null }
    val inactivos by produceState<Result<InactivosGrupo>?>(null, jid, dias, pestana) { value = if (pestana == "inactivos") runCatching { Repo.inactivosGrupo(botId, jid, dias) } else null }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Actividad del grupo") }) }) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                FilterChip(pestana == "ranking", { haptica(Toque.Suave); pestana = "ranking" }, { Text("Quién escribe más") }, modifier = Modifier.testTag("tab_ranking"))
                FilterChip(pestana == "inactivos", { haptica(Toque.Suave); pestana = "inactivos" }, { Text("Inactivos") }, modifier = Modifier.testTag("tab_inactivos"))
            }
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                PERIODOS.forEach { d -> FilterChip(dias == d, { haptica(Toque.Suave); dias = d }, { Text("$d días") }, modifier = Modifier.testTag("dias_$d")) }
            }
            if (pestana == "ranking") {
                val r = ranking
                when {
                    r == null -> CircularProgressIndicator(Modifier.size(24.dp))
                    r.isFailure -> Text(textoError(r.exceptionOrNull()!!), Modifier.testTag("actividad_error"), color = MaterialTheme.colorScheme.onSurfaceVariant)
                    else -> {
                        val rk = r.getOrThrow()
                        if (rk.filas.isEmpty()) EstadoVacio(Ico.Estadisticas, "Sin mensajes", "Nadie escribió en los últimos ${rk.dias} días (o esta app aún no tiene datos).", Modifier.testTag("ranking_vacio"))
                        else {
                            Text("${plural(rk.total.toLong(), "mensaje", "mensajes")} en ${rk.dias} días" + if (rk.soloMiembros) " (de quienes siguen en el grupo)" else "", Modifier.testTag("ranking_total"), style = MaterialTheme.typography.bodyMedium)
                            val mayor = rk.filas.first().mensajes.coerceAtLeast(1)
                            rk.filas.forEach { f ->
                                Column(Modifier.testTag("ranking_${f.puesto}")) {
                                    Row { Text("${f.puesto}. ${f.nombre}", Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis); Text("${f.mensajes} · ${f.porcentaje.toInt()}%", color = MaterialTheme.colorScheme.onSurfaceVariant) }
                                    BarraRelativa(f.mensajes.toFloat() / mayor, Modifier.padding(top = 4.dp))
                                }
                            }
                        }
                        if (rk.aviso.isNotBlank()) Text(rk.aviso, Modifier.testTag("actividad_aviso"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }
            } else {
                val r = inactivos
                when {
                    r == null -> CircularProgressIndicator(Modifier.size(24.dp))
                    r.isFailure -> Text(textoError(r.exceptionOrNull()!!), Modifier.testTag("actividad_error"), color = MaterialTheme.colorScheme.onSurfaceVariant)
                    else -> {
                        val ia = r.getOrThrow()
                        if (ia.lista.isEmpty()) EstadoVacio(Ico.Listo, "Todos escribieron", "Nadie del grupo se quedó sin escribir en los últimos ${ia.dias} días.", Modifier.testTag("inactivos_vacio"))
                        else {
                            Text("${plural(ia.lista.size.toLong(), "persona sin mensajes", "personas sin mensajes")} en ${ia.dias} días", Modifier.testTag("inactivos_total"), style = MaterialTheme.typography.bodyMedium)
                            ia.lista.forEach { x ->
                                Column(Modifier.fillMaxWidth().testTag("inactivo_${x.persona}")) {
                                    Text(x.nombre + if (x.esAdmin) " (administrador)" else "", maxLines = 1, overflow = TextOverflow.Ellipsis)
                                    Text(if (x.ultimoDia.isBlank()) "Sin mensajes guardados" else "Último mensaje: ${x.ultimoDia}" + if (x.diasSin >= 0) " (hace ${plural(x.diasSin.toLong(), "día", "días")})" else "",
                                        style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                }
                            }
                        }
                        if (ia.aviso.isNotBlank()) Text(ia.aviso, Modifier.testTag("actividad_aviso"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }
            }
        }
    }
}
