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
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.CircularProgressIndicator
import com.botchat.app.ui.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.FiltroLista
import com.botchat.app.datos.Msg
import com.botchat.app.datos.PersonaChat
import com.botchat.app.datos.Repo
import com.botchat.app.medios.CacheImagenes
import kotlinx.coroutines.launch

private val TIPOS_CUADRICULA = setOf("imagen", "sticker", "video")

/** Título de una lista filtrada: "Fotos · 4", "A las 9:00 · 12", "Martes · 7", "Lunes 3 de noviembre · 5", "Eliminados · 2". */
fun tituloDeFiltro(f: FiltroLista, total: Long?): String {
    val base = when {
        f.tipos.size == 1 -> nombreTipo(f.tipos[0])
        f.tipos.isNotEmpty() -> f.tipos.joinToString(", ") { nombreTipo(it) }
        f.dia.isNotEmpty() -> diaLegible(f.dia)
        f.hora != null -> "A las ${f.hora}:00"
        f.semana != null -> nombreDia(f.semana)
        f.eliminados -> "Eliminados"
        else -> "Mensajes"
    }
    val extra = buildList {
        if (f.tipos.isNotEmpty() && f.hora != null) add("a las ${f.hora}:00")
        if (f.tipos.isNotEmpty() && f.semana != null) add(nombreDia(f.semana).lowercase(LocaleEs))
        if (f.tipos.isNotEmpty() && f.dia.isNotEmpty()) add(diaLegible(f.dia).lowercase(LocaleEs))
    }
    val completo = if (extra.isEmpty()) base else base + " " + extra.joinToString(" ")
    return if (total != null) "$completo · ${numero(total)}" else completo
}

/** Lista de mensajes de un chat que cumplen un filtro: la usan las filas de tipos, las barras de hora y de semana y el calendario. */
@Composable
fun PantallaListaFiltrada(
    botId: Long, jid: String, filtroInicial: FiltroLista, esGrupo: Boolean,
    alVolver: () -> Unit, alAbrirVisor: (String) -> Unit, alAbrirDocumento: (String) -> Unit, alIrAlMensaje: (String) -> Unit,
) {
    var filtro by remember { mutableStateOf(filtroInicial) }
    var items by remember { mutableStateOf<List<Msg>?>(null) }
    var total by remember { mutableStateOf<Long?>(null) }
    var hayMas by remember { mutableStateOf(true) }
    var cargando by remember { mutableStateOf(false) }
    var version by remember { mutableStateOf(0) }
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    val personas by produceState<List<PersonaChat>>(emptyList(), jid, esGrupo) { if (esGrupo) value = runCatching { Repo.personas(botId, jid) }.getOrDefault(emptyList()) }
    DisposableEffect(Unit) { onDispose { CacheImagenes.vaciar() } }

    LaunchedEffect(filtro, version) {
        items = null; total = null; hayMas = true
        val p = runCatching { Repo.filtrados(botId, jid, filtro) }.getOrNull()
        items = p?.items ?: emptyList(); total = p?.total ?: 0; hayMas = (p?.items?.size ?: 0) >= 60
    }
    LaunchedEffect(jid) { Repo.cambios.collect { evs -> if (evs.any { it == "chat:$jid" || it.startsWith("archivo:$jid:") }) version++ } }
    fun cargarMas() {
        val actuales = items ?: return
        if (cargando || !hayMas || actuales.isEmpty()) return
        cargando = true
        scope.launch {
            val u = actuales.last()
            val p = runCatching { Repo.filtrados(botId, jid, filtro, u.ts, u.id) }.getOrNull()
            items = actuales + (p?.items ?: emptyList()); hayMas = (p?.items?.size ?: 0) >= 60; cargando = false
        }
    }

    Scaffold(
        topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text(tituloDeFiltro(filtro, total), Modifier.testTag("titulo_lista"), maxLines = 1, overflow = TextOverflow.Ellipsis) }) },
    ) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno)) {
            Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.xs), horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                listOf("" to "Todos", "enviados" to "Enviados", "recibidos" to "Recibidos").forEach { (valor, texto) ->
                    FilterChip(filtro.direccion == valor, { haptica(Toque.Suave); filtro = filtro.copy(direccion = valor) }, label = { Text(texto, maxLines = 1) }, modifier = Modifier.testTag("chip_dir_${valor.ifEmpty { "todos" }}"))
                }
                FilterChip(filtro.eliminados, { haptica(Toque.Suave); filtro = filtro.copy(eliminados = !filtro.eliminados) }, label = { Text("Eliminados", maxLines = 1) }, modifier = Modifier.testTag("chip_eliminados"))
            }
            if (esGrupo && personas.isNotEmpty()) {
                var menu by remember { mutableStateOf(false) }
                Box(Modifier.padding(horizontal = Esp.l)) {
                    FilterChip(filtro.persona.isNotEmpty(), { menu = true }, label = { Text(personas.firstOrNull { it.jid == filtro.persona }?.nombre ?: "Toda persona", maxLines = 1) }, modifier = Modifier.testTag("chip_persona"))
                    DropdownMenu(menu, { menu = false }) {
                        DropdownMenuItem(text = { Text("Toda persona") }, onClick = { menu = false; filtro = filtro.copy(persona = "") })
                        personas.forEach { p -> DropdownMenuItem(text = { Text("${p.nombre} · ${numero(p.cantidad)}") }, onClick = { menu = false; filtro = filtro.copy(persona = p.jid) }) }
                    }
                }
            }
            val datos = items
            when {
                datos == null -> Box(Modifier.fillMaxSize(), Alignment.Center) { CircularProgressIndicator() }
                datos.isEmpty() -> EstadoVacio(Ico.Estadisticas, "Nada por aquí", "No hay mensajes que cumplan este filtro.", Modifier.padding(top = 60.dp))
                filtro.tipos.isNotEmpty() && filtro.tipos.all { it in TIPOS_CUADRICULA } -> Cuadricula(datos, alAbrirVisor, porFecha = true, alLlegarAlFinal = if (hayMas) ({ cargarMas() }) else null)
                else -> LazyColumn(Modifier.fillMaxSize().testTag("lista_filtrada")) {
                    items(datos, key = { it.id }) { m ->
                        FilaFiltrada(m, alAbrir = {
                            when {
                                m.categoria == "documento" && m.descargado -> alAbrirDocumento(m.id)
                                (m.categoria == "foto" || m.categoria == "video") && m.descargado -> alAbrirVisor(m.id)
                                else -> alIrAlMensaje(m.id)
                            }
                        }, alIr = { alIrAlMensaje(m.id) })
                        HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                    }
                    if (hayMas) item(key = "fin") { LaunchedEffect(datos.size) { cargarMas() }; Box(Modifier.fillMaxWidth().padding(Esp.l), Alignment.Center) { CircularProgressIndicator(Modifier.size(24.dp), strokeWidth = 2.dp) } }
                }
            }
        }
    }
}

@Composable
private fun FilaFiltrada(m: Msg, alAbrir: () -> Unit, alIr: () -> Unit) {
    Row(Modifier.fillMaxWidth().clickable { alAbrir() }.padding(start = Esp.l, top = Esp.s, bottom = Esp.s).testTag("fila_${m.id}"), verticalAlignment = Alignment.CenterVertically) {
        Column(Modifier.weight(1f)) {
            Text("${etiquetaDia(m.ts)} · ${horaDe(m.ts)} · ${if (m.esMio) "Bot" else m.remitente.ifBlank { "Contacto" }}", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.primary)
            Text(
                if (m.borrado && !m.contenidoDisponible) "Eliminado · contenido no disponible" else descripcionCorta(m.tipo, m.texto, false, m.notaVoz),
                maxLines = 2, overflow = TextOverflow.Ellipsis,
            )
            if (m.borrado) Text("Eliminado", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.error)
        }
        if (m.descargado && m.categoria.isNotEmpty() && com.botchat.app.medios.Exportar.disponible) {
            val ctx = androidx.compose.ui.platform.LocalContext.current
            val scope = androidx.compose.runtime.rememberCoroutineScope()
            IconButton({ scope.launch { guardarCopia(ctx, m, "Bot Chat") } }, Modifier.testTag("guardar_galeria_${m.id}")) { Icon(Ico.Guardar, "Guardar en la galería") }
        }
        IconButton(alIr, Modifier.testTag("ir_${m.id}")) { Icon(Ico.IrA, "Ir al mensaje en el chat") }
    }
}
