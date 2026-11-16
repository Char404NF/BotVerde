@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.OutlinedButton
import androidx.compose.ui.draw.clip
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.EstadMenu
import com.botchat.app.datos.FilaEstMenu
import com.botchat.app.datos.Repo
import java.time.LocalDate

private val RANGOS = listOf("Hoy" to 0L, "7 días" to 6L, "30 días" to 29L, "Todo" to -1L)

/** Estadísticas de una automatización: cuántos entraron, cuántos terminaron, qué opciones eligen y dónde se atoran. */
@Composable
fun PantallaEstadisticasMenu(botId: Long, menuId: Long, nombre: String, alVolver: () -> Unit, alFormularios: (Long) -> Unit = {}, alNoEntendio: (Long) -> Unit = {}) {
    var rango by remember { mutableStateOf("30 días") }
    val datos by produceState<EstadMenu?>(null, rango) {
        val dias = RANGOS.first { it.first == rango }.second
        val hoy = LocalDate.now()
        val desde = if (dias < 0) "2000-01-01" else hoy.minusDays(dias).toString()
        value = runCatching { Repo.estadisticasMenu(botId, menuId, desde, hoy.toString(), false) }.getOrNull()
    }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text(nombre.ifBlank { "Estadísticas" }) }) }) { relleno ->
        val d = datos
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                RANGOS.forEach { (t, _) -> FilterChip(rango == t, { rango = t }, label = { Text(t) }, modifier = Modifier.testTag("rango_$t")) }
            }
            if (d == null) { Box(Modifier.fillMaxWidth().padding(Esp.xl), Alignment.Center) { CircularProgressIndicator() }; return@Column }
            if (d.iniciados == 0L) {
                EstadoVacio(Ico.Estadisticas, "Sin datos todavía", "Cuando alguien entre a este menú verás aquí cuántos lo terminan y dónde se atoran.", Modifier.padding(top = Esp.xl))
                return@Column
            }
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                Cifra(d.iniciados, "Iniciados", "Iniciado", Modifier.weight(1f).testTag("est_iniciados"))
                Cifra(d.completados, "Completados", "Completado", Modifier.weight(1f).testTag("est_completados"))
            }
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                Cifra(d.abandonados, "Abandonados", "Abandonado", Modifier.weight(1f).testTag("est_abandonados"))
                Cifra(d.humano, "Pasaron a una persona", "Pasó a una persona", Modifier.weight(1f).testTag("est_humano"))
            }
            SeccionEst("Cómo terminan") { BarraDeEstados(d) }
            if (d.opciones.isNotEmpty()) SeccionEst("Opciones elegidas") { Barras(d.opciones) { "${it.titulo} → ${it.opcion}" + if (it.texto.isNotBlank()) " (${it.texto})" else "" } }
            if (d.embudo.isNotEmpty()) SeccionEst("Embudo: quién llegó a cada paso") { Barras(d.embudo) { it.titulo } }
            if (d.abandonos.isNotEmpty()) SeccionEst("Dónde abandonan") { Barras(d.abandonos) { it.titulo } }
            if (d.invalidas.isNotEmpty()) SeccionEst("Dónde más se equivocan") { Barras(d.invalidas) { it.titulo } }
            if (d.formulariosEnviados + d.formulariosCancelados > 0 || d.formularios > 0) SeccionEst("Formularios") {
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Cifra(d.formulariosEnviados, "Enviados", "Enviado", Modifier.weight(1f).testTag("est_form_enviados"))
                    Cifra(d.formulariosCancelados, "Cancelados", "Cancelado", Modifier.weight(1f).testTag("est_form_cancelados"))
                }
                if (d.formularios > 0) OutlinedButton({ alFormularios(menuId) }, Modifier.fillMaxWidth().testTag("ver_formularios_menu")) { Text("Ver los ${plural(d.formularios, "formulario recibido", "formularios recibidos")}") }
            }
            SeccionEst("Lo que no entendió") {
                Text(if (d.noEntendio == 0L) "Hasta ahora no hay mensajes sin entender." else plural(d.noEntendio, "mensaje que el menú no entendió", "mensajes que el menú no entendió"), Modifier.testTag("est_no_entendio"), style = MaterialTheme.typography.bodyMedium)
                if (d.noEntendio > 0) OutlinedButton({ alNoEntendio(menuId) }, Modifier.fillMaxWidth().testTag("ver_no_entendio_menu")) { Text("Ver la lista") }
            }
        }
    }
}

/** Barra apilada simple: de cada 100 conversaciones, cuántas se completan, pasan a una persona, caducan o se abandonan. */
@Composable
private fun BarraDeEstados(d: EstadMenu) {
    val abandonadas = (d.abandonados - d.expirados).coerceAtLeast(0)
    val partes = listOf(
        Triple("Completadas", d.completados, MaterialTheme.colorScheme.primary), Triple("A una persona", d.humano, MaterialTheme.colorScheme.tertiary),
        Triple("Caducaron", d.expirados, MaterialTheme.colorScheme.outline), Triple("Abandonadas", abandonadas, MaterialTheme.colorScheme.error),
    ).filter { it.second > 0 }
    val total = partes.sumOf { it.second }.coerceAtLeast(1L)
    if (partes.isEmpty()) { Text("Todavía no termina ninguna conversación.", style = MaterialTheme.typography.bodySmall); return }
    Row(Modifier.fillMaxWidth().height(18.dp).clip(RoundedCornerShape(9.dp)).testTag("barra_estados")) {
        partes.forEach { (_, n, c) -> Box(Modifier.weight(n.toFloat() / total).fillMaxHeight().background(c)) }
    }
    partes.forEach { (t, n, c) ->
        Row(verticalAlignment = Alignment.CenterVertically) {
            Box(Modifier.size(10.dp).clip(CircleShape).background(c))
            Text("  $t", Modifier.weight(1f), style = MaterialTheme.typography.bodyMedium)
            Text("${numero(n)} (${(n * 100 / total)}%)", color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

@Composable
private fun Barras(filas: List<FilaEstMenu>, etiqueta: (FilaEstMenu) -> String) {
    val mayor = (filas.maxOfOrNull { it.valor } ?: 1L).coerceAtLeast(1L)
    filas.forEach { f ->
        Column(Modifier.testTag("fila_est")) {
            Row { Text(etiqueta(f), Modifier.weight(1f), style = MaterialTheme.typography.bodyMedium); Text(numero(f.valor), color = MaterialTheme.colorScheme.onSurfaceVariant) }
            BarraRelativa(f.valor.toFloat() / mayor, Modifier.padding(top = 4.dp))
        }
    }
}
