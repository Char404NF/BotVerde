@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.animation.animateContentSize
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
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
import androidx.compose.foundation.lazy.items
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

/** Pantalla principal: lista de chats con pestañas Personales | Grupos y buscador por nombre y por número. */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun PantallaInicio(
    sinSesion: Boolean,
    alAbrirChat: (String) -> Unit,
    alAbrirEspacio: () -> Unit,
    alAbrirAvanzado: () -> Unit,
    alAbrirBot: () -> Unit,
    alRevincular: () -> Unit,
) {
    val vm: InicioVm = viewModel()
    val scope = rememberCoroutineScope()
    val pager = rememberPagerState { 2 }
    val barra = TopAppBarDefaults.exitUntilCollapsedScrollBehavior()
    val barraPequena = TopAppBarDefaults.pinnedScrollBehavior()
    val personales by vm.personales.collectAsState()
    val grupos by vm.grupos.collectAsState()
    val estado by BotState.estado.collectAsState()
    val corriendo by BotState.corriendo.collectAsState()
    val haptica = rememberHaptica()
    var menu by remember { mutableStateOf(false) }

    Scaffold(
        modifier = Modifier.nestedScroll(if (vm.buscando) barraPequena.nestedScrollConnection else barra.nestedScrollConnection),
        topBar = {
            if (vm.buscando) {
                val foco = remember { FocusRequester() }
                TopAppBar(
                    scrollBehavior = barraPequena,
                    navigationIcon = { IconButton({ vm.cerrarBusqueda() }) { Icon(Ico.Atras, "Cerrar búsqueda") } },
                    title = {
                        TextField(
                            value = vm.consulta, onValueChange = vm::cambiarConsulta, singleLine = true,
                            placeholder = { Text("Buscar por nombre o número") },
                            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
                            colors = TextFieldDefaults.colors(focusedContainerColor = androidx.compose.ui.graphics.Color.Transparent, unfocusedContainerColor = androidx.compose.ui.graphics.Color.Transparent,
                                focusedIndicatorColor = androidx.compose.ui.graphics.Color.Transparent, unfocusedIndicatorColor = androidx.compose.ui.graphics.Color.Transparent),
                            modifier = Modifier.fillMaxWidth().focusRequester(foco),
                        )
                        androidx.compose.runtime.LaunchedEffect(Unit) { foco.requestFocus() }
                    },
                    actions = { if (vm.consulta.isNotEmpty()) IconButton({ vm.cambiarConsulta("") }) { Icon(Ico.Cerrar, "Borrar texto") } },
                )
            } else {
                LargeTopAppBar(
                    scrollBehavior = barra,
                    title = { Text("Mensajes") },
                    actions = {
                        EstadoBot(estado, corriendo, sinSesion, alAbrirBot)
                        IconButton({ haptica(Toque.Suave); vm.abrirBusqueda() }) { Icon(Ico.Buscar, "Buscar") }
                        IconButton(alAbrirEspacio) { Icon(Ico.Espacio, "Espacio") }
                        Box {
                            IconButton({ menu = true }) { Icon(Ico.Mas, "Más opciones") }
                            DropdownMenu(menu, { menu = false }) {
                                DropdownMenuItem(text = { Text("Estado del bot") }, onClick = { menu = false; alAbrirBot() })
                                DropdownMenuItem(text = { Text("Avanzado") }, onClick = { menu = false; alAbrirAvanzado() })
                            }
                        }
                    },
                )
            }
        },
    ) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno)) {
            if (sinSesion) BannerSinSesion(alRevincular)
            PrimaryTabRow(selectedTabIndex = pager.currentPage, containerColor = MaterialTheme.colorScheme.surface) {
                listOf("Personales", "Grupos").forEachIndexed { i, titulo ->
                    Tab(selected = pager.currentPage == i, onClick = { haptica(Toque.Suave); scope.launch { pager.animateScrollToPage(i) } }, text = { Text(titulo) })
                }
            }
            HorizontalPager(pager, Modifier.fillMaxSize(), key = { it }) { pagina ->
                val lista = if (pagina == 0) personales else grupos
                ListaDeChats(lista, esGrupos = pagina == 1, buscando = vm.consulta.isNotBlank(), error = vm.error, alAbrirChat = alAbrirChat)
            }
        }
    }
}

@Composable
private fun EstadoBot(estado: String, corriendo: Boolean, sinSesion: Boolean, alPulsar: () -> Unit) {
    val verde = corriendo && estado == "conectado"
    val color = when {
        verde -> Color(0xFF2E9E5B)
        sinSesion || estado.startsWith("error") -> MaterialTheme.colorScheme.error
        else -> MaterialTheme.colorScheme.outline
    }
    val texto = when {
        sinSesion -> "Sin sesión"
        verde -> "En línea"
        corriendo -> "Conectando"
        else -> "Detenido"
    }
    Row(
        Modifier.clip(RoundedCornerShape(50)).clickable(onClick = alPulsar).padding(horizontal = 10.dp, vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.size(9.dp).clip(CircleShape).background(color))
        Spacer(Modifier.width(6.dp))
        Text(texto, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
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
private fun ListaDeChats(lista: List<ChatItem>?, esGrupos: Boolean, buscando: Boolean, error: String?, alAbrirChat: (String) -> Unit) {
    when {
        lista == null -> FilasDeCarga()
        lista.isEmpty() -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            if (buscando) {
                EstadoVacio(Ico.Buscar, "Sin resultados", "No hay chats con ese nombre o número.")
            } else {
                EstadoVacio(
                    Ico.Documento,
                    if (esGrupos) "Aún no hay grupos" else "Aún no hay mensajes",
                    error ?: "Cuando alguien escriba al número del bot, la conversación aparecerá aquí.",
                )
            }
        }
        else -> LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(bottom = 24.dp)) {
            items(lista, key = { it.jid }) { c ->
                FilaChat(c, Modifier.animateItem(), alAbrirChat)
                HorizontalDivider(Modifier.padding(start = 84.dp), color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
            }
        }
    }
}

@Composable
fun FilaChat(c: ChatItem, modifier: Modifier, alAbrir: (String) -> Unit) {
    Row(modifier.fillMaxWidth().clickable { alAbrir(c.jid) }.padding(horizontal = Esp.l, vertical = Esp.m), verticalAlignment = Alignment.CenterVertically) {
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
        }
        Spacer(Modifier.width(Esp.m))
        Text(horaLista(c.ultimoTs), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, fontWeight = FontWeight.Medium)
    }
}

