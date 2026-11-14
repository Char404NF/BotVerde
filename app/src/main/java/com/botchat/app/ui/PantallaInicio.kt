@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.animation.animateContentSize
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.combinedClickable
import androidx.activity.compose.BackHandler
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.TextButton
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.RowScope
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

/**
 * Pestaña Chats: una sola lista (personas y grupos juntos, como en las apps de mensajería), buscador, filtros en chips,
 * botón flotante de nuevo chat y selección múltiple con mantener pulsado.
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun PantallaInicio(
    sinSesion: Boolean,
    alAbrirChat: (String) -> Unit,
    alRevincular: () -> Unit,
    estado: EstadoVisible = EstadoVisible.CONECTADO,
    alPulsarEstado: () -> Unit = {},
    alNuevoChat: () -> Unit = {},
    alMarcados: () -> Unit = {},
    alAjustes: () -> Unit = {},
) {
    val vm: InicioVm = viewModel()
    val personales by vm.personales.collectAsState()
    val grupos by vm.grupos.collectAsState()
    val contadores by vm.contadores.collectAsState()
    val etiquetas by vm.etiquetas.collectAsState()
    var soloNoLeidos by rememberSaveable { mutableStateOf(false) }
    var soloGrupos by rememberSaveable { mutableStateOf(false) }
    var seleccion by remember { mutableStateOf(emptySet<String>()) }
    var vaciando by remember { mutableStateOf(false) }

    val lista = remember(personales, grupos, contadores, soloNoLeidos, soloGrupos) {
        val p = personales
        val g = grupos
        if (p == null || g == null) null
        else (p + g).sortedWith(compareByDescending<ChatItem> { it.fijado }.thenByDescending { it.ultimoTs })
            .filter { (!soloGrupos || it.esGrupo) && (!soloNoLeidos || (contadores[it.jid] ?: 0) > 0) }
    }
    BackHandler(seleccion.isNotEmpty()) { seleccion = emptySet() }

    Box(Modifier.fillMaxSize()) {
        Column(Modifier.fillMaxSize()) {
            if (seleccion.isEmpty()) {
                BarraInicio(estado, alPulsarEstado, vm, alMarcados, alAjustes)
            } else {
                val elegidos = lista.orEmpty().filter { it.jid in seleccion }
                BarraSeleccionChats(
                    cuenta = seleccion.size, todosFijados = elegidos.isNotEmpty() && elegidos.all { it.fijado },
                    alCerrar = { seleccion = emptySet() },
                    alFijar = { vm.fijar(seleccion, !(elegidos.isNotEmpty() && elegidos.all { it.fijado })); seleccion = emptySet() },
                    alSilenciar = { vm.silenciar(seleccion, true); seleccion = emptySet() },
                    alQuitarSilencio = { vm.silenciar(seleccion, false); seleccion = emptySet() },
                    alLeido = { vm.marcarLeido(seleccion); seleccion = emptySet() },
                    alPausarBot = { vm.pausarBot(seleccion, 60); seleccion = emptySet() },
                    alReanudarBot = { vm.pausarBot(seleccion, 0); seleccion = emptySet() },
                    alVaciar = { vaciando = true },
                )
            }
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
            FiltrosDeChats(vm, soloNoLeidos, soloGrupos, { soloNoLeidos = it }, { soloGrupos = it }, { soloNoLeidos = false; soloGrupos = false })
            if (sinSesion) BannerSinSesion(alRevincular)
            ListaDeChats(
                lista, contadores, buscando = vm.consulta.isNotBlank(), error = vm.error, etiquetas = etiquetas,
                filtrando = vm.filtroEstado.isNotEmpty() || vm.filtroEtiqueta != 0L || soloNoLeidos || soloGrupos,
                seleccion = seleccion,
                alAbrirChat = { jid -> if (seleccion.isNotEmpty()) seleccion = if (jid in seleccion) seleccion - jid else seleccion + jid else alAbrirChat(jid) },
                alElegir = { jid -> seleccion = if (jid in seleccion) seleccion - jid else seleccion + jid },
            )
        }
        if (seleccion.isEmpty()) {
            FloatingActionButton(
                onClick = alNuevoChat, shape = RoundedCornerShape(16.dp),
                containerColor = MaterialTheme.colorScheme.primary, contentColor = MaterialTheme.colorScheme.onPrimary,
                modifier = Modifier.align(Alignment.BottomEnd).padding(Esp.l).testTag("fab_nuevo_chat"),
            ) { Icon(Ico.NuevoChat, "Nuevo chat") }
        }
    }
    if (vaciando) {
        AlertDialog(
            onDismissRequest = { vaciando = false },
            title = { Text(if (seleccion.size == 1) "¿Vaciar este chat?" else "¿Vaciar ${seleccion.size} chats?") },
            text = { Text("Se borran los mensajes guardados en este teléfono. En WhatsApp no se borra nada y esto no se puede deshacer.") },
            confirmButton = { TextButton({ vm.vaciar(seleccion); seleccion = emptySet(); vaciando = false }, Modifier.testTag("confirmar_vaciar")) { Text("Vaciar") } },
            dismissButton = { TextButton({ vaciando = false }) { Text("Cancelar") } },
        )
    }
}

/** Barra de la lista: título grande y menú de tres puntos. */
@Composable
private fun BarraInicio(estado: EstadoVisible, alPulsarEstado: () -> Unit, vm: InicioVm, alMarcados: () -> Unit, alAjustes: () -> Unit) {
    var menu by remember { mutableStateOf(false) }
    BarraApp(estado, alPulsarEstado, "BotChat") {
        Box {
            IconButton({ menu = true }, Modifier.testTag("menu_inicio")) { Icon(Ico.Mas, "Más opciones") }
            DropdownMenu(menu, { menu = false }) {
                DropdownMenuItem({ Text("Estado del bot") }, { menu = false; alPulsarEstado() })
                DropdownMenuItem({ Text("Mensajes marcados") }, { menu = false; alMarcados() })
                DropdownMenuItem({ Text("Marcar todos como leídos") }, { menu = false; vm.marcarTodosLeidos() }, modifier = Modifier.testTag("marcar_todos_leidos"))
                DropdownMenuItem({ Text("Ajustes") }, { menu = false; alAjustes() })
            }
        }
    }
}

/** Barra que sustituye a la de arriba cuando hay chats elegidos: cerrar, cuántos, fijar, silenciar, vaciar y más. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun BarraSeleccionChats(
    cuenta: Int, todosFijados: Boolean, alCerrar: () -> Unit, alFijar: () -> Unit, alSilenciar: () -> Unit, alQuitarSilencio: () -> Unit,
    alLeido: () -> Unit, alPausarBot: () -> Unit, alReanudarBot: () -> Unit, alVaciar: () -> Unit,
) {
    var menu by remember { mutableStateOf(false) }
    TopAppBar(
        navigationIcon = { IconButton(alCerrar, Modifier.testTag("cerrar_seleccion")) { Icon(Ico.Atras, "Salir de la selección") } },
        title = { Text(cuenta.toString(), style = MaterialTheme.typography.titleLarge) },
        actions = {
            IconButton(alFijar, Modifier.testTag("seleccion_fijar")) { Icon(Ico.Fijar, if (todosFijados) "Dejar de fijar" else "Fijar") }
            IconButton(alSilenciar, Modifier.testTag("seleccion_silenciar")) { Icon(Ico.CampanaTachada, "Silenciar notificaciones") }
            IconButton(alVaciar, Modifier.testTag("seleccion_vaciar")) { Icon(Ico.Borrar, "Vaciar chat") }
            Box {
                IconButton({ menu = true }) { Icon(Ico.Mas, "Más opciones") }
                DropdownMenu(menu, { menu = false }) {
                    DropdownMenuItem({ Text("Marcar como leído") }, { menu = false; alLeido() })
                    DropdownMenuItem({ Text("Quitar silencio") }, { menu = false; alQuitarSilencio() })
                    DropdownMenuItem({ Text("Pausar el bot 1 hora") }, { menu = false; alPausarBot() })
                    DropdownMenuItem({ Text("Reanudar el bot") }, { menu = false; alReanudarBot() })
                }
            }
        },
        colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.surfaceContainerHigh),
    )
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
private fun ListaDeChats(
    lista: List<ChatItem>?, contadores: Map<String, Int>, buscando: Boolean, error: String?, etiquetas: List<com.botchat.app.datos.EtiquetaItem> = emptyList(),
    filtrando: Boolean = false, seleccion: Set<String> = emptySet(), alAbrirChat: (String) -> Unit, alElegir: (String) -> Unit,
) {
    when {
        lista == null -> FilasDeCarga()
        lista.isEmpty() -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            if (buscando) {
                EstadoVacio(Ico.Buscar, "Sin resultados", "No hay chats con ese nombre o número.")
            } else if (filtrando) {
                EstadoVacio(Ico.Seleccionar, "Ningún chat con ese filtro", "Quita el filtro o marca chats como pendientes desde su menú ⋮.", Modifier.testTag("filtro_vacio"))
            } else {
                EstadoVacio(Ico.Documento, "Aún no hay mensajes", error ?: "Cuando alguien escriba al número del bot, la conversación aparecerá aquí.")
            }
        }
        else -> LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(bottom = 96.dp)) {
            item { androidx.compose.runtime.LaunchedEffect(Unit) { kotlinx.coroutines.delay(700); EntradaLista.hecha = true } }
            itemsIndexed(lista, key = { _, c -> c.jid }) { i, c ->
                FilaChat(c, Modifier.animateItem().entradaEscalonada(i), alAbrirChat, contadores[c.jid] ?: 0, etiquetas, c.jid in seleccion) { alElegir(c.jid) }
            }
        }
    }
}

/** Una fila de la lista: foto, nombre, hora, última palabra con palomitas, insignia de no leídos y chincheta; fondo verdoso si está elegida. */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun FilaChat(
    c: ChatItem, modifier: Modifier, alAbrir: (String) -> Unit, contador: Int = 0, etiquetas: List<com.botchat.app.datos.EtiquetaItem> = emptyList(),
    seleccionado: Boolean = false, alLargo: (() -> Unit)? = null,
) {
    val colores = LocalColoresChat.current
    Row(
        modifier.fillMaxWidth()
            .background(if (seleccionado) colores.seleccion else Color.Transparent)
            .combinedClickable(onClick = { alAbrir(c.jid) }, onLongClick = alLargo)
            .padding(horizontal = Esp.l, vertical = 10.dp).testTag("fila_${c.jid}"),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box {
            Avatar(c.nombre, c.foto, c.fotoTs, c.jid, 52.dp, Modifier.compartido("avatar-${c.jid}"), grupo = c.esGrupo)
            if (seleccionado) {
                Box(
                    Modifier.align(Alignment.BottomEnd).size(20.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primary).testTag("elegida_${c.jid}"),
                    contentAlignment = Alignment.Center,
                ) { Icon(Ico.Listo, null, Modifier.size(14.dp), tint = MaterialTheme.colorScheme.onPrimary) }
            }
        }
        Spacer(Modifier.width(Esp.l))
        Column(Modifier.weight(1f)) {
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                Row(Modifier.weight(1f), verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        c.nombre, Modifier.weight(1f, fill = false).compartido("nombre-${c.jid}", limites = true),
                        style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
                    )
                    if (c.estadoTrabajo.isNotEmpty()) {
                        Spacer(Modifier.width(6.dp))
                        MarcaEstadoTrabajo(c.estadoTrabajo, Modifier.testTag("marca_${c.jid}"))
                    }
                }
                Spacer(Modifier.width(Esp.s))
                Text(
                    horaLista(c.ultimoTs), style = MaterialTheme.typography.labelMedium,
                    color = if (contador > 0) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
                    fontWeight = if (contador > 0) FontWeight.Medium else FontWeight.Normal,
                )
            }
            Spacer(Modifier.size(2.dp))
            Row(verticalAlignment = Alignment.CenterVertically) {
                Row(Modifier.weight(1f), verticalAlignment = Alignment.CenterVertically) {
                    if (c.ultimoMio && !c.ultimoBorrado) {
                        Icon(Ico.PalomitaDoble, null, Modifier.size(18.dp), tint = colores.palomita)
                        Spacer(Modifier.width(3.dp))
                    }
                    val icono = iconoDeTipo(c.ultimoTipo)
                    if (icono != null && !c.ultimoBorrado) {
                        Icon(icono, null, Modifier.size(16.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
                        Spacer(Modifier.width(4.dp))
                    }
                    TextoUnaLinea(descripcionCorta(c.ultimoTipo, c.ultimoTexto, c.ultimoBorrado))
                }
                if (c.fijado) {
                    Spacer(Modifier.width(Esp.s))
                    Icon(Ico.Fijar, "Fijado", Modifier.size(18.dp).testTag("fijado_${c.jid}"), tint = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                if (contador > 0) {
                    Spacer(Modifier.width(Esp.s))
                    Box(
                        Modifier.defaultMinSize(minWidth = 22.dp, minHeight = 22.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primary)
                            .padding(horizontal = 6.dp).testTag("contador_${c.jid}"),
                        contentAlignment = Alignment.Center,
                    ) {
                        Text(if (contador >= 30) "30+" else contador.toString(), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onPrimary, fontWeight = FontWeight.Bold)
                    }
                }
            }
            val puestas = etiquetas.filter { it.id in c.etiquetas }
            if (puestas.isNotEmpty()) {
                Spacer(Modifier.size(4.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) { puestas.take(3).forEach { ChipEtiqueta(it) } }
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

/** Filtros de la lista en chips: Todos, No leídos, Grupos, Pendientes, Resueltos y las etiquetas. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun FiltrosDeChats(
    vm: InicioVm, soloNoLeidos: Boolean, soloGrupos: Boolean, alNoLeidos: (Boolean) -> Unit, alGrupos: (Boolean) -> Unit, alTodos: () -> Unit,
) {
    val etiquetas by vm.etiquetas.collectAsState()
    val pendientes by vm.pendientes.collectAsState()
    val haptica = rememberHaptica()
    val colores = androidx.compose.material3.FilterChipDefaults.filterChipColors(
        selectedContainerColor = MaterialTheme.colorScheme.primaryContainer, selectedLabelColor = MaterialTheme.colorScheme.onPrimaryContainer,
    )
    val forma = RoundedCornerShape(50)
    val sinFiltro = vm.filtroEstado.isEmpty() && vm.filtroEtiqueta == 0L && !soloNoLeidos && !soloGrupos
    androidx.compose.foundation.lazy.LazyRow(
        Modifier.fillMaxWidth().testTag("filtros_chats"), contentPadding = PaddingValues(horizontal = Esp.l), horizontalArrangement = Arrangement.spacedBy(Esp.s),
    ) {
        item {
            androidx.compose.material3.FilterChip(sinFiltro, { haptica(Toque.Suave); alTodos(); vm.filtrar("", 0L) }, { Text("Todos") }, shape = forma, colors = colores, modifier = Modifier.testTag("filtro_todos"))
        }
        item {
            androidx.compose.material3.FilterChip(soloNoLeidos, { haptica(Toque.Suave); alNoLeidos(!soloNoLeidos) }, { Text("No leídos") }, shape = forma, colors = colores, modifier = Modifier.testTag("filtro_no_leidos"))
        }
        item {
            androidx.compose.material3.FilterChip(soloGrupos, { haptica(Toque.Suave); alGrupos(!soloGrupos) }, { Text("Grupos") }, shape = forma, colors = colores, modifier = Modifier.testTag("filtro_grupos"))
        }
        item {
            androidx.compose.material3.FilterChip(
                vm.filtroEstado == "pendiente", { haptica(Toque.Suave); vm.filtrar(if (vm.filtroEstado == "pendiente") "" else "pendiente", 0L) },
                { Text(if (pendientes > 0) "Pendientes ($pendientes)" else "Pendientes") }, shape = forma, colors = colores, modifier = Modifier.testTag("filtro_pendientes"),
            )
        }
        item {
            androidx.compose.material3.FilterChip(
                vm.filtroEstado == "resuelto", { haptica(Toque.Suave); vm.filtrar(if (vm.filtroEstado == "resuelto") "" else "resuelto", 0L) },
                { Text("Resueltos") }, shape = forma, colors = colores, modifier = Modifier.testTag("filtro_resueltos"),
            )
        }
        items(etiquetas.size) { i ->
            val e = etiquetas[i]
            androidx.compose.material3.FilterChip(
                vm.filtroEtiqueta == e.id, { haptica(Toque.Suave); vm.filtrar("", if (vm.filtroEtiqueta == e.id) 0L else e.id) },
                { Text(e.nombre, maxLines = 1) }, leadingIcon = { PuntoRango(e.color, 10) }, shape = forma, colors = colores, modifier = Modifier.testTag("filtro_etiqueta_${e.id}"),
            )
        }
    }
}
