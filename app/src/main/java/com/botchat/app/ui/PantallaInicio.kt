@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.animation.animateContentSize
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LargeTopAppBar
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.PrimaryTabRow
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Tab
import androidx.compose.material3.Text
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.input.nestedscroll.nestedScroll
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.botchat.app.BotState
import com.botchat.app.Paises
import com.botchat.app.datos.ChatItem
import kotlinx.coroutines.launch

/** Pestaña Chats: buscador por nombre y número, y la lista con pestañas Personales | Grupos. La barra superior la pone el Shell. */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun PantallaInicio(
    sinSesion: Boolean,
    alAbrirChat: (String) -> Unit,
    alRevincular: () -> Unit,
) {
    val vm: InicioVm = viewModel()
    val scope = rememberCoroutineScope()
    val pager = rememberPagerState { 2 }
    val personales by vm.personales.collectAsState()
    val grupos by vm.grupos.collectAsState()
    val contadores by vm.contadores.collectAsState()
    val etiquetas by vm.etiquetas.collectAsState()
    val haptica = rememberHaptica()

    Column(Modifier.fillMaxSize()) {
        TextField(
            value = vm.consulta, onValueChange = vm::cambiarConsulta, singleLine = true,
            placeholder = { Text("Buscar por nombre o número") },
            leadingIcon = { Icon(Ico.Buscar, null) },
            trailingIcon = { if (vm.consulta.isNotEmpty()) IconButton({ vm.cambiarConsulta("") }) { Icon(Ico.Cerrar, "Borrar texto") } },
            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
            shape = RoundedCornerShape(28.dp),
            colors = TextFieldDefaults.colors(
                focusedContainerColor = MaterialTheme.colorScheme.surfaceContainerHigh, unfocusedContainerColor = MaterialTheme.colorScheme.surfaceContainerHigh,
                focusedIndicatorColor = Color.Transparent, unfocusedIndicatorColor = Color.Transparent, disabledIndicatorColor = Color.Transparent,
            ),
            modifier = Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s).testTag("buscador"),
        )
        FiltrosDeChats(vm)
        if (sinSesion) BannerSinSesion(alRevincular)
        PrimaryTabRow(selectedTabIndex = pager.currentPage, containerColor = MaterialTheme.colorScheme.surface) {
            listOf("Personales", "Grupos").forEachIndexed { i, titulo ->
                Tab(selected = pager.currentPage == i, onClick = { haptica(Toque.Suave); scope.launch { pager.animateScrollToPage(i) } }, text = { Text(titulo) })
            }
        }
        HorizontalPager(pager, Modifier.fillMaxSize(), key = { it }) { pagina ->
            val lista = if (pagina == 0) personales else grupos
            ListaDeChats(lista, contadores, esGrupos = pagina == 1, buscando = vm.consulta.isNotBlank(), error = vm.error, alAbrirChat = alAbrirChat, etiquetas = etiquetas, filtrando = vm.filtroEstado.isNotEmpty() || vm.filtroEtiqueta != 0L)
        }
    }
}

@Composable
private fun BannerSinSesion(alRevincular: () -> Unit) {
    Surface(color = MaterialTheme.colorScheme.errorContainer, modifier = Modifier.fillMaxWidth().animateContentSize()) {
        Row(Modifier.padding(horizontal = Esp.l, vertical = Esp.m), verticalAlignment = Alignment.CenterVertically) {
            Icon(Ico.Aviso, null, tint = MaterialTheme.colorScheme.onErrorContainer)
            Spacer(Modifier.width(Esp.m))
            Column(Modifier.weight(1f)) {
                Text("WhatsApp cerró la sesión de este bot", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.onErrorContainer)
                Text("Tus mensajes guardados siguen aquí.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onErrorContainer)
            }
            Button(alRevincular) { Text("Vincular de nuevo") }
        }
    }
}

@Composable
private fun ListaDeChats(lista: List<ChatItem>?, contadores: Map<String, Int>, esGrupos: Boolean, buscando: Boolean, error: String?, alAbrirChat: (String) -> Unit, etiquetas: List<com.botchat.app.datos.EtiquetaItem> = emptyList(), filtrando: Boolean = false) {
    when {
        lista == null -> FilasDeCarga()
        lista.isEmpty() -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            if (buscando) {
                EstadoVacio(Ico.Buscar, "Sin resultados", "No hay chats con ese nombre o número.")
            } else if (filtrando) {
                EstadoVacio(Ico.Seleccionar, "Ningún chat con ese filtro", "Marca chats como pendientes o ponles etiquetas desde su menú ⋮.", Modifier.testTag("filtro_vacio"))
            } else {
                EstadoVacio(
                    Ico.Documento,
                    if (esGrupos) "Aún no hay grupos" else "Aún no hay mensajes",
                    error ?: "Cuando alguien escriba al número del bot, la conversación aparecerá aquí.",
                )
            }
        }
        else -> LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(bottom = 24.dp)) {
            item { androidx.compose.runtime.LaunchedEffect(Unit) { kotlinx.coroutines.delay(700); EntradaLista.hecha = true } }
            itemsIndexed(lista, key = { _, c -> c.jid }) { i, c ->
                FilaChat(c, Modifier.animateItem().entradaEscalonada(i), alAbrirChat, contadores[c.jid] ?: 0, etiquetas)
                HorizontalDivider(Modifier.padding(start = 84.dp), color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
            }
        }
    }
}

@Composable
fun FilaChat(c: ChatItem, modifier: Modifier, alAbrir: (String) -> Unit, contador: Int = 0, etiquetas: List<com.botchat.app.datos.EtiquetaItem> = emptyList()) {
    Row(modifier.fillMaxWidth().clickableEscala { alAbrir(c.jid) }.padding(horizontal = Esp.l, vertical = Esp.m), verticalAlignment = Alignment.CenterVertically) {
        Avatar(c.nombre, c.foto, c.fotoTs, c.jid, 52.dp, Modifier.compartido("avatar-${c.jid}"))
        Spacer(Modifier.width(Esp.l))
        Column(Modifier.weight(1f)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    c.nombre, Modifier.weight(1f, fill = false).compartido("nombre-${c.jid}", limites = true),
                    style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
                )
                if (!c.esGrupo && c.pais.isNotEmpty()) {
                    Spacer(Modifier.width(6.dp))
                    Text(Paises.banderaDe(c.pais), style = MaterialTheme.typography.bodyMedium)
                }
                if (c.estadoTrabajo.isNotEmpty()) {
                    Spacer(Modifier.width(6.dp))
                    MarcaEstadoTrabajo(c.estadoTrabajo, Modifier.testTag("marca_${c.jid}"))
                }
            }
            Spacer(Modifier.size(2.dp))
            Row(verticalAlignment = Alignment.CenterVertically) {
                val icono = iconoDeTipo(c.ultimoTipo)
                if (icono != null && !c.ultimoBorrado) {
                    Icon(icono, null, Modifier.size(16.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
                    Spacer(Modifier.width(4.dp))
                }
                TextoUnaLinea(descripcionCorta(c.ultimoTipo, c.ultimoTexto, c.ultimoBorrado))
            }
            val puestas = etiquetas.filter { it.id in c.etiquetas }
            if (puestas.isNotEmpty()) {
                Spacer(Modifier.size(4.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) { puestas.take(3).forEach { ChipEtiqueta(it) } }
            }
        }
        Spacer(Modifier.width(Esp.m))
        Column(horizontalAlignment = Alignment.End) {
            Text(horaLista(c.ultimoTs), style = MaterialTheme.typography.labelMedium, color = if (contador > 0) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant, fontWeight = FontWeight.Medium)
            if (contador > 0) {
                Spacer(Modifier.size(4.dp))
                Box(Modifier.defaultMinSize(minWidth = 22.dp, minHeight = 22.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primary).padding(horizontal = 6.dp).testTag("contador_${c.jid}"), Alignment.Center) {
                    Text(if (contador >= 30) "30+" else contador.toString(), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onPrimary, fontWeight = FontWeight.Bold)
                }
            }
        }
    }
}


/** Marca visible de Pendiente / Resuelto. */
@Composable
fun MarcaEstadoTrabajo(estado: String, modifier: Modifier = Modifier) {
    val pendiente = estado == "pendiente"
    Box(
        modifier.clip(RoundedCornerShape(50)).background(if (pendiente) MaterialTheme.colorScheme.tertiaryContainer else MaterialTheme.colorScheme.secondaryContainer).padding(horizontal = 8.dp, vertical = 1.dp),
    ) {
        Text(
            if (pendiente) "Pendiente" else "✓ Resuelto", style = MaterialTheme.typography.labelSmall,
            color = if (pendiente) MaterialTheme.colorScheme.onTertiaryContainer else MaterialTheme.colorScheme.onSecondaryContainer, maxLines = 1,
        )
    }
}

/** Filtros de la lista: Todos, Pendientes, Resueltos y una etiqueta. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun FiltrosDeChats(vm: InicioVm) {
    val etiquetas by vm.etiquetas.collectAsState()
    val pendientes by vm.pendientes.collectAsState()
    val haptica = rememberHaptica()
    androidx.compose.foundation.lazy.LazyRow(
        Modifier.fillMaxWidth().testTag("filtros_chats"), contentPadding = PaddingValues(horizontal = Esp.l), horizontalArrangement = Arrangement.spacedBy(Esp.s),
    ) {
        item {
            androidx.compose.material3.FilterChip(vm.filtroEstado.isEmpty() && vm.filtroEtiqueta == 0L, { haptica(Toque.Suave); vm.filtrar("", 0L) }, { Text("Todos") }, modifier = Modifier.testTag("filtro_todos"))
        }
        item {
            androidx.compose.material3.FilterChip(vm.filtroEstado == "pendiente", { haptica(Toque.Suave); vm.filtrar(if (vm.filtroEstado == "pendiente") "" else "pendiente", 0L) }, { Text(if (pendientes > 0) "Pendientes ($pendientes)" else "Pendientes") }, modifier = Modifier.testTag("filtro_pendientes"))
        }
        item {
            androidx.compose.material3.FilterChip(vm.filtroEstado == "resuelto", { haptica(Toque.Suave); vm.filtrar(if (vm.filtroEstado == "resuelto") "" else "resuelto", 0L) }, { Text("Resueltos") }, modifier = Modifier.testTag("filtro_resueltos"))
        }
        items(etiquetas.size) { i ->
            val e = etiquetas[i]
            androidx.compose.material3.FilterChip(
                vm.filtroEtiqueta == e.id, { haptica(Toque.Suave); vm.filtrar("", if (vm.filtroEtiqueta == e.id) 0L else e.id) },
                { Text(e.nombre, maxLines = 1) }, leadingIcon = { PuntoRango(e.color, 10) }, modifier = Modifier.testTag("filtro_etiqueta_${e.id}"),
            )
        }
    }
}
