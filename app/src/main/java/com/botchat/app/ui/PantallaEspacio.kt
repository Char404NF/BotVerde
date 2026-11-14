@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalFoundationApi::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import android.widget.Toast
import androidx.compose.animation.animateContentSize
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import com.botchat.app.ui.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LargeTopAppBar
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.nestedscroll.nestedScroll
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.botchat.app.datos.EspacioChat
import com.botchat.app.datos.EspacioTipo
import kotlin.math.max

private val NOMBRES = mapOf("foto" to "Fotos", "video" to "Videos", "audio" to "Audios", "documento" to "Documentos")

@Composable
private fun colorDeTipo(cat: String): Color = when (cat) {
    "foto" -> MaterialTheme.colorScheme.primary
    "video" -> MaterialTheme.colorScheme.tertiary
    "audio" -> MaterialTheme.colorScheme.secondary
    else -> MaterialTheme.colorScheme.error.copy(alpha = 0.8f)
}

/** Espacio: lo que ocupa BotChat, por tipo y por chat, con borrado manual (nada se borra solo). */
@Composable
fun PantallaEspacio(alVolver: () -> Unit, alAbrirChat: (String) -> Unit, conBarra: Boolean = true, ajustes: com.botchat.app.datos.Ajustes? = null) {
    val vm: EspacioVm = viewModel()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    val barra = TopAppBarDefaults.exitUntilCollapsedScrollBehavior()
    val barraSel = TopAppBarDefaults.pinnedScrollBehavior()
    val seleccionando = vm.seleccion.isNotEmpty()
    val visibles = vm.visibles()
    LaunchedEffect(vm.mensaje) { vm.mensaje?.let { Toast.makeText(ctx, it, Toast.LENGTH_LONG).show(); vm.mensaje = null } }
    androidx.activity.compose.BackHandler(seleccionando) { vm.limpiarSeleccion() }
    val e8 = remember { EstadoAlmacenE8() }

    Box(Modifier.fillMaxSize()) {
    Scaffold(
        modifier = Modifier.nestedScroll(if (seleccionando) barraSel.nestedScrollConnection else barra.nestedScrollConnection),
        topBar = {
            if (seleccionando) {
                TopAppBar(
                    scrollBehavior = barraSel,
                    navigationIcon = { IconButton({ vm.limpiarSeleccion() }) { Icon(Ico.Cerrar, "Cancelar selección") } },
                    title = { Text("${vm.seleccion.size} seleccionado${if (vm.seleccion.size == 1) "" else "s"}") },
                    actions = {
                        TextButton({ vm.seleccionarTodo() }) { Text("Todos") }
                        IconButton({ haptica(Toque.Fuerte); vm.pedirBorrarSeleccion() }) { Icon(Ico.Borrar, "Borrar archivos") }
                    },
                )
            } else if (conBarra) {
                LargeTopAppBar(
                    scrollBehavior = barra,
                    navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } },
                    title = { Text(if (ajustes != null) "Almacenamiento" else "Espacio") },
                )
            }
        },
    ) { relleno ->
        LazyColumn(Modifier.fillMaxSize().padding(relleno), contentPadding = PaddingValues(bottom = 32.dp)) {
            if (ajustes != null) item { BloqueAlmacenamiento(vm.botId, ajustes, e8) }
            item { ResumenDeTelefono(vm) }
            item { GraficaPorTipo(vm.tipos) }
            item { AccionesRapidas(vm, haptica) }
            item {
                Text("Chats que más ocupan", Modifier.padding(horizontal = Esp.l, vertical = Esp.s), style = MaterialTheme.typography.titleMedium)
                FlowRow(Modifier.padding(horizontal = Esp.l), horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    listOf("" to "Todo", "foto" to "Fotos", "video" to "Videos", "audio" to "Audios", "documento" to "Documentos").forEach { (clave, titulo) ->
                        FilterChip(vm.filtro == clave, { haptica(Toque.Suave); vm.filtro = clave }, label = { Text(titulo) })
                    }
                }
                Spacer(Modifier.height(Esp.s))
            }
            when {
                vm.chats == null -> item { FilasDeCarga(4) }
                visibles.isEmpty() -> item { EstadoVacio(Ico.Espacio, "Sin archivos descargados", "Las fotos, videos, audios y documentos que se descarguen aparecerán aquí.", Modifier.padding(top = Esp.xl)) }
                else -> {
                    val maximo = max(1L, vm.bytesDe(visibles.first()))
                    items(visibles, key = { it.jid }) { c ->
                        FilaDeEspacio(c, vm.bytesDe(c), maximo, c.jid in vm.seleccion, seleccionando, Modifier.animateItem(),
                            alPulsar = { if (seleccionando) vm.alternarSeleccion(c.jid) else alAbrirChat(c.jid) },
                            alMantener = { haptica(Toque.Fuerte); vm.alternarSeleccion(c.jid) },
                            alBorrarArchivos = { vm.pedirBorrarArchivosDe(c) }, alBorrarChat = { vm.pedirBorrarChat(c) })
                    }
                }
            }
        }
    }

    if (ajustes != null) HojasAlmacenamiento(vm.botId, ajustes, e8)
    }
    vm.pendiente?.let { a ->
        AlertDialog(
            onDismissRequest = { vm.pendiente = null },
            title = { Text(a.titulo) },
            text = { Text(a.texto) },
            confirmButton = {
                TextButton({ haptica(Toque.Confirmar); vm.confirmar { Toast.makeText(ctx, "Se liberaron ${formatoBytes(it.bytes)}", Toast.LENGTH_SHORT).show() } }) { Text(a.boton, color = MaterialTheme.colorScheme.error) }
            },
            dismissButton = { TextButton({ vm.pendiente = null }) { Text("Cancelar") } },
        )
    }
    vm.reporte?.let { r ->
        AlertDialog(
            onDismissRequest = { vm.reporte = null },
            title = { Text("Revisión terminada") },
            text = {
                Text(
                    if (r.sinCambios) "Todo está en orden: los archivos coinciden con lo que muestra la app."
                    else buildString {
                        if (r.estadosCorregidos > 0) append("• ${r.estadosCorregidos} archivo(s) faltaban o estaban dañados; ahora aparecen con el botón Descargar.\n")
                        if (r.tamanosCorregidos > 0) append("• ${r.tamanosCorregidos} tamaño(s) corregido(s).\n")
                        if (r.huerfanos > 0) append("• ${r.huerfanos} archivo(s) sobrantes borrados (${formatoBytes(r.bytesHuerfanos)}).\n")
                        if (r.miniaturasLimpiadas + r.fotosLimpiadas > 0) append("• ${r.miniaturasLimpiadas + r.fotosLimpiadas} imagen(es) pequeña(s) sin archivo; se volverán a pedir.\n")
                        if (r.chatsRecalculados > 0) append("• ${r.chatsRecalculados} chat(s) con la hora corregida.")
                    }.trim(),
                )
            },
            confirmButton = { TextButton({ vm.reporte = null }) { Text("Listo") } },
        )
    }
    if (vm.ocupado) Box(Modifier.fillMaxSize().background(Color.Black.copy(alpha = 0.25f)), Alignment.Center) { CircularProgressIndicator() }
}

@Composable
private fun ResumenDeTelefono(vm: EspacioVm) {
    val t = vm.total
    val reducir = LocalReducirMovimiento.current
    Card(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l)) {
            if (t == null) {
                Box(Modifier.fillMaxWidth().height(18.dp).clip(RoundedCornerShape(9.dp)).shimmer())
            } else {
                Row(verticalAlignment = Alignment.Bottom) {
                    Text(formatoBytes(t.ocupaBotChat), style = MaterialTheme.typography.headlineMedium)
                    Spacer(Modifier.width(Esp.s))
                    Text("ocupa BotChat", Modifier.padding(bottom = 4.dp), color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                Spacer(Modifier.height(Esp.m))
                val total = max(1L, t.total).toFloat()
                val propio = animateFloatAsState((t.ocupaBotChat / total).coerceIn(0.004f, 1f), Mov.tiempo(reducir, Mov.LARGA), label = "propio")
                val otros = animateFloatAsState(((t.total - t.libre - t.ocupaBotChat).coerceAtLeast(0) / total).coerceIn(0f, 1f), Mov.tiempo(reducir, Mov.LARGA), label = "otros")
                val libre = animateFloatAsState((t.libre / total).coerceIn(0f, 1f), Mov.tiempo(reducir, Mov.LARGA), label = "libre")
                Row(Modifier.fillMaxWidth().height(16.dp).clip(RoundedCornerShape(8.dp))) {
                    Box(Modifier.weight(max(propio.value, 0.001f)).fillMaxSize().background(MaterialTheme.colorScheme.primary))
                    Box(Modifier.weight(max(otros.value, 0.001f)).fillMaxSize().background(MaterialTheme.colorScheme.outline.copy(alpha = 0.55f)))
                    Box(Modifier.weight(max(libre.value, 0.001f)).fillMaxSize().background(MaterialTheme.colorScheme.surfaceContainerHighest))
                }
                Spacer(Modifier.height(Esp.s))
                Row(horizontalArrangement = Arrangement.spacedBy(Esp.l)) {
                    Leyenda(MaterialTheme.colorScheme.primary, "BotChat")
                    Leyenda(MaterialTheme.colorScheme.outline.copy(alpha = 0.55f), "Otras apps")
                    Leyenda(MaterialTheme.colorScheme.surfaceContainerHighest, "Libre")
                }
                Spacer(Modifier.height(Esp.s))
                Text("Libre en el teléfono: ${formatoBytes(t.libre)} de ${formatoBytes(t.total)}", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }
}

@Composable
private fun Leyenda(color: Color, texto: String) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.size(10.dp).clip(CircleShape).background(color))
        Spacer(Modifier.width(6.dp))
        Text(texto, style = MaterialTheme.typography.labelMedium)
    }
}

@Composable
private fun GraficaPorTipo(tipos: List<EspacioTipo>) {
    val reducir = LocalReducirMovimiento.current
    val total = max(1L, tipos.sumOf { it.bytes })
    Card(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l).animateContentSize()) {
            Text("Por tipo de archivo", style = MaterialTheme.typography.titleMedium)
            Spacer(Modifier.height(Esp.m))
            CATEGORIAS.forEach { cat ->
                val e = tipos.firstOrNull { it.categoria == cat }
                val bytes = e?.bytes ?: 0
                val frac = animateFloatAsState(if (bytes == 0L) 0f else (bytes.toFloat() / total).coerceAtLeast(0.02f), Mov.tiempo(reducir, Mov.LARGA), label = cat)
                Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(vertical = 5.dp)) {
                    Icon(when (cat) { "foto" -> Ico.Foto; "video" -> Ico.Video; "audio" -> Ico.Audio; else -> Ico.Documento }, null, Modifier.size(22.dp), tint = colorDeTipo(cat))
                    Spacer(Modifier.width(Esp.m))
                    Column(Modifier.weight(1f)) {
                        Row { Text(NOMBRES.getValue(cat), Modifier.weight(1f), style = MaterialTheme.typography.bodyMedium); Text("${formatoBytes(bytes)} · ${e?.cantidad ?: 0}", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                        Spacer(Modifier.height(4.dp))
                        Box(Modifier.fillMaxWidth().height(8.dp).clip(RoundedCornerShape(4.dp)).background(MaterialTheme.colorScheme.surfaceContainerHighest)) {
                            Box(Modifier.fillMaxWidth(frac.value).fillMaxSize().clip(RoundedCornerShape(4.dp)).background(colorDeTipo(cat)))
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun AccionesRapidas(vm: EspacioVm, haptica: (Toque) -> Unit) {
    Column(Modifier.padding(horizontal = Esp.l, vertical = Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
        Text("Liberar espacio", style = MaterialTheme.typography.titleMedium)
        Text("Nada se borra solo. Siempre te digo cuánto se libera antes de borrar.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        BotonBorrar("Borrar videos", vm.estimadoVideos.bytes, vm.estimadoVideos.bytes > 0) { haptica(Toque.Fuerte); vm.pedirBorrarVideos() }
        BotonBorrar("Borrar archivos de más de 30 días", vm.estimadoViejos.bytes, vm.estimadoViejos.bytes > 0) { haptica(Toque.Fuerte); vm.pedirBorrarViejos() }
        BotonBorrar("Borrar todo lo multimedia", vm.estimadoTodo.bytes, vm.estimadoTodo.bytes > 0) { haptica(Toque.Fuerte); vm.pedirBorrarTodo() }
        OutlinedButton({ haptica(Toque.Suave); vm.reparar() }, Modifier.fillMaxWidth()) { Icon(Ico.Actualizar, null, Modifier.size(18.dp)); Spacer(Modifier.width(8.dp)); Text("Revisar y reparar") }
    }
}

@Composable
private fun BotonBorrar(texto: String, bytes: Long, activo: Boolean, alPulsar: () -> Unit) {
    OutlinedButton(alPulsar, Modifier.fillMaxWidth(), enabled = activo) {
        Icon(Ico.Borrar, null, Modifier.size(18.dp)); Spacer(Modifier.width(8.dp))
        Text(texto, Modifier.weight(1f))
        Text(if (bytes > 0) formatoBytes(bytes) else "—", fontWeight = FontWeight.SemiBold)
    }
}

@Composable
private fun FilaDeEspacio(
    c: EspacioChat, bytes: Long, maximo: Long, seleccionado: Boolean, seleccionando: Boolean, modifier: Modifier,
    alPulsar: () -> Unit, alMantener: () -> Unit, alBorrarArchivos: () -> Unit, alBorrarChat: () -> Unit,
) {
    var menu by remember { mutableStateOf(false) }
    val reducir = LocalReducirMovimiento.current
    val frac = animateFloatAsState((bytes.toFloat() / maximo).coerceIn(0.02f, 1f), Mov.tiempo(reducir, Mov.LARGA), label = "fila")
    val fondo = if (seleccionado) MaterialTheme.colorScheme.secondaryContainer else Color.Transparent
    Row(
        modifier.fillMaxWidth().background(fondo).combinedClickable(onClick = alPulsar, onLongClick = alMantener).padding(horizontal = Esp.l, vertical = Esp.m),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box {
            Avatar(c.nombre, c.foto, c.fotoTs, c.jid, 46.dp)
            if (seleccionado) Box(Modifier.align(Alignment.BottomEnd).size(20.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primary), Alignment.Center) { Icon(Ico.Listo, null, Modifier.size(14.dp), tint = MaterialTheme.colorScheme.onPrimary) }
        }
        Spacer(Modifier.width(Esp.l))
        Column(Modifier.weight(1f)) {
            Row { Text(c.nombre, Modifier.weight(1f), style = MaterialTheme.typography.titleSmall, maxLines = 1, overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis); Text(formatoBytes(bytes), style = MaterialTheme.typography.titleSmall) }
            Spacer(Modifier.height(5.dp))
            LinearProgressIndicator(progress = { frac.value }, Modifier.fillMaxWidth().height(6.dp).clip(RoundedCornerShape(3.dp)), trackColor = MaterialTheme.colorScheme.surfaceContainerHighest)
            Text("${c.cantidad} archivo${if (c.cantidad == 1) "" else "s"}", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.padding(top = 3.dp))
        }
        if (!seleccionando) {
            Box {
                IconButton({ menu = true }) { Icon(Ico.Mas, "Más opciones de ${c.nombre}") }
                DropdownMenu(menu, { menu = false }) {
                    DropdownMenuItem(text = { Text("Borrar archivos") }, onClick = { menu = false; alBorrarArchivos() })
                    DropdownMenuItem(text = { Text("Borrar todo el chat") }, onClick = { menu = false; alBorrarChat() })
                }
            }
        }
    }
}
