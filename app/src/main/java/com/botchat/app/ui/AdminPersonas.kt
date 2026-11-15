@file:OptIn(ExperimentalLayoutApi::class, ExperimentalFoundationApi::class, ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshots.SnapshotStateList
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.MiembroAdmin
import com.botchat.app.datos.Repo
import com.botchat.app.datos.banear
import com.botchat.app.datos.miembrosAdmin
import com.botchat.app.datos.suscribirPresencias
import kotlinx.coroutines.launch

/** Cómo se filtra la lista de personas del grupo. */
enum class FiltroPersonas(val texto: String) {
    TODOS("Todos"), EN_LINEA("En línea"), ADMINS("Admins"), ADVERTIDOS("Advertidos"), SILENCIADOS("Silenciados"), BANEADOS("Baneados"), INACTIVOS("Inactivos"),
}

/** Cómo se ordena la lista de personas del grupo. */
enum class OrdenPersonas(val texto: String) {
    MAS_MENSAJES("Más mensajes"), RECIENTES("Escribieron hace poco"), MENOS_ACTIVOS("Menos activos"), NOMBRE("Nombre"), RANGO("Cargo y rango"),
}

private const val DIAS_INACTIVO = 30

internal fun aplicaFiltro(m: MiembroAdmin, f: FiltroPersonas, ahora: Long): Boolean = when (f) {
    FiltroPersonas.TODOS -> true
    FiltroPersonas.EN_LINEA -> m.enLinea
    FiltroPersonas.ADMINS -> m.esAdmin
    FiltroPersonas.ADVERTIDOS -> m.advertencias > 0 || m.advertido
    FiltroPersonas.SILENCIADOS -> m.silenciado
    FiltroPersonas.BANEADOS -> m.baneado
    FiltroPersonas.INACTIVOS -> m.conteo.ultimoTs < ahora - DIAS_INACTIVO * 86400L
}

internal fun ordena(l: List<MiembroAdmin>, o: OrdenPersonas): List<MiembroAdmin> = when (o) {
    OrdenPersonas.MAS_MENSAJES -> l.sortedWith(compareByDescending<MiembroAdmin> { it.conteo.mensajes }.thenBy { it.titulo.lowercase() })
    OrdenPersonas.RECIENTES -> l.sortedWith(compareByDescending<MiembroAdmin> { it.conteo.ultimoTs }.thenBy { it.titulo.lowercase() })
    OrdenPersonas.MENOS_ACTIVOS -> l.sortedWith(compareBy<MiembroAdmin> { it.conteo.ultimoTs }.thenBy { it.titulo.lowercase() })
    OrdenPersonas.NOMBRE -> l.sortedBy { it.titulo.lowercase() }
    OrdenPersonas.RANGO -> l.sortedWith(compareByDescending<MiembroAdmin> { it.esCreador }.thenByDescending { it.esAdmin }.thenByDescending { it.rango.isNotBlank() }.thenBy { it.titulo.lowercase() })
}

internal fun coincide(m: MiembroAdmin, q: String): Boolean {
    if (q.isBlank()) return true
    val x = q.trim().lowercase()
    return m.titulo.lowercase().contains(x) || m.telefono.contains(x) || m.persona.contains(x) || m.rango.lowercase().contains(x)
}

/**
 * La lista de personas del grupo: foto con puntito de «en línea», nombre, número, cargo y rango, último mensaje y cuánto envió de cada cosa.
 * Tocar abre su perfil; mantener pulsado selecciona varias para sacarlas o banearlas de una vez.
 */
@Composable
fun PestanaPersonasAdmin(botId: Long, jid: String, soyAdmin: Boolean, alAbrirPerfil: (persona: String) -> Unit, alVerBaneados: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    var ultimoAviso by remember { mutableStateOf(0L) }
    LaunchedEffect(jid) {
        runCatching { Repo.suscribirPresencias(botId, jid) }
        Repo.cambios.collect { evs ->
            val ahora = System.currentTimeMillis()
            val presencia = evs.any { it.startsWith("presencia:") }
            if (evs.any { it == "grupo:$jid" || it == "personas" || it == "rangos" || it.startsWith("silenc") || it.startsWith("advert") } || (presencia && ahora - ultimoAviso > 2000)) {
                ultimoAviso = ahora; version++
            }
        }
    }
    val datos by produceState<com.botchat.app.datos.ListaMiembrosAdmin?>(null, jid, version) { value = runCatching { Repo.miembrosAdmin(botId, jid) }.getOrNull() }
    var consulta by remember { mutableStateOf("") }
    var filtro by remember { mutableStateOf(FiltroPersonas.TODOS) }
    var orden by remember { mutableStateOf(OrdenPersonas.MAS_MENSAJES) }
    var menuOrden by remember { mutableStateOf(false) }
    val elegidas: SnapshotStateList<String> = remember { mutableStateListOf() }
    var confirmar by remember { mutableStateOf<String?>(null) }
    var motivo by remember { mutableStateOf<String?>(null) }
    val ahora = System.currentTimeMillis() / 1000
    val todos = datos?.miembros
    val vista = remember(todos, consulta, filtro, orden) { todos?.filter { coincide(it, consulta) && aplicaFiltro(it, filtro, ahora) }?.let { ordena(it, orden) } }

    fun elegir(p: String) { haptica(Toque.Suave); if (p in elegidas) elegidas.remove(p) else elegidas.add(p) }
    fun lote(texto: String, f: suspend (String) -> Unit) {
        val quienes = elegidas.toList(); elegidas.clear()
        scope.launch {
            var bien = 0; var mal: String? = null
            quienes.forEach { p -> runCatching { f(p) }.onSuccess { bien++ }.onFailure { mal = textoError(it) } }
            avisaToast(ctx, if (mal == null) "Listo: $texto a $bien" else "$bien de ${quienes.size}. ${mal}", mal != null)
            version++
        }
    }

    Column(Modifier.fillMaxSize().testTag("pestana_personas_admin")) {
        if (elegidas.isNotEmpty()) {
            Surface(color = MaterialTheme.colorScheme.secondaryContainer, modifier = Modifier.fillMaxWidth().testTag("barra_seleccion")) {
                Row(Modifier.padding(horizontal = Esp.s, vertical = Esp.xs), verticalAlignment = Alignment.CenterVertically) {
                    IconButton({ elegidas.clear() }, Modifier.testTag("seleccion_cancelar")) { Icon(Ico.Cerrar, "Cancelar selección") }
                    Text(plural(elegidas.size.toLong(), "seleccionada", "seleccionadas"), Modifier.weight(1f), style = MaterialTheme.typography.titleMedium)
                    TextButton({ confirmar = "expulsar" }, Modifier.testTag("seleccion_expulsar"), enabled = soyAdmin) { Text("Expulsar") }
                    TextButton({ motivo = "" }, Modifier.testTag("seleccion_banear"), enabled = soyAdmin) { Text("Banear") }
                }
            }
        } else {
            OutlinedTextField(
                consulta, { consulta = it }, Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.xs).testTag("buscar_persona_admin"), singleLine = true,
                placeholder = { Text("Buscar por nombre, número o rango") }, leadingIcon = { Icon(Ico.Buscar, null) },
                trailingIcon = {
                    Box {
                        IconButton({ menuOrden = true }, Modifier.testTag("ordenar_personas")) { Icon(Ico.Mas, "Ordenar") }
                        DropdownMenu(menuOrden, { menuOrden = false }) {
                            OrdenPersonas.entries.forEach { o ->
                                DropdownMenuItem(
                                    text = { Text((if (o == orden) "✓  " else "     ") + o.texto) },
                                    onClick = { menuOrden = false; orden = o }, modifier = Modifier.testTag("orden_${o.name}"),
                                )
                            }
                        }
                    }
                },
            )
            Row(Modifier.horizontalScroll(rememberScrollState()).padding(horizontal = Esp.l), horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                FiltroPersonas.entries.forEach { f ->
                    val n = todos?.count { aplicaFiltro(it, f, ahora) }
                    FilterChip(filtro == f, { haptica(Toque.Suave); filtro = f }, { Text(if (f != FiltroPersonas.TODOS && n != null && n > 0) "${f.texto} $n" else f.texto) }, modifier = Modifier.testTag("filtro_${f.name}"))
                }
            }
        }
        when {
            vista == null -> FilasDeCarga()
            todos.isNullOrEmpty() -> Box(Modifier.fillMaxSize().padding(Esp.l), Alignment.Center) {
                EstadoVacio(Ico.Grupo, "Aún no tengo la lista", "Con el bot encendido la voy guardando sola. Vuelve en un momento.", Modifier.testTag("personas_vacio"))
            }
            else -> {
                val enLinea = todos.count { it.enLinea }
                Text(
                    plural(vista.size.toLong(), "persona", "personas") + if (enLinea > 0) " · $enLinea en línea" else "",
                    Modifier.padding(horizontal = Esp.l, vertical = 2.dp).testTag("contador_personas_admin"), style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                if (vista.isEmpty()) Box(Modifier.fillMaxSize().padding(Esp.l), Alignment.Center) { EstadoVacio(Ico.Buscar, "Nadie coincide", "Prueba con otro filtro o quita la búsqueda.", Modifier.testTag("personas_sin_resultado")) }
                else LazyColumn(Modifier.fillMaxSize().testTag("lista_personas_admin"), contentPadding = PaddingValues(bottom = 24.dp)) {
                    items(vista, key = { it.persona + it.jid }) { m ->
                        FilaPersonaAdmin(m, m.persona in elegidas, elegidas.isNotEmpty(), { if (elegidas.isNotEmpty()) elegir(m.persona) else alAbrirPerfil(m.persona) }, { elegir(m.persona) })
                        HorizontalDivider(Modifier.padding(start = 76.dp), color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                    }
                    item(key = "baneados") { TextButton(alVerBaneados, Modifier.padding(horizontal = Esp.s).testTag("ver_baneados")) { Text("Ver la lista de baneados") } }
                }
            }
        }
    }
    when (confirmar) {
        "expulsar" -> ConfirmarSinDeshacer(
            "¿Expulsar a ${plural(elegidas.size.toLong(), "persona", "personas")}?", "Salen del grupo ahora. Pueden volver si tienen el enlace.", "Expulsar", "confirmar_expulsar_lote",
            { confirmar = null }, { confirmar = null; lote("las saqué") { Repo.expulsarDeGrupo(botId, jid, it) } },
        )
    }
    motivo?.let {
        HojaTexto(
            "Banear a ${plural(elegidas.size.toLong(), "persona", "personas")}", "", "motivo_ban_lote", { motivo = null },
            { t -> motivo = null; lote("las baneé") { Repo.banear(botId, jid, it, t) } },
            ayuda = "Salen del grupo y, si vuelven a entrar, las saco otra vez. El motivo es solo para ti (opcional).", permiteVacio = true, maximo = 200, lineas = 2,
        )
    }
}

/** Una persona de la lista. */
@Composable
fun FilaPersonaAdmin(m: MiembroAdmin, elegida: Boolean, seleccionando: Boolean, alPulsar: () -> Unit, alMantener: () -> Unit) {
    val fondo = if (elegida) MaterialTheme.colorScheme.secondaryContainer.copy(alpha = 0.6f) else androidx.compose.ui.graphics.Color.Transparent
    Row(
        Modifier.fillMaxWidth().background(fondo).combinedClickable(onClick = alPulsar, onLongClick = alMantener).padding(horizontal = Esp.l, vertical = Esp.m).testTag("persona_admin_${m.persona}"),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box {
            AvatarEnLinea(m, 52.dp)
            if (elegida) Box(Modifier.align(Alignment.BottomEnd).size(22.dp).background(MaterialTheme.colorScheme.primary, androidx.compose.foundation.shape.CircleShape), Alignment.Center) {
                Icon(Ico.Listo, "Seleccionada", Modifier.size(14.dp), tint = MaterialTheme.colorScheme.onPrimary)
            }
        }
        Spacer(Modifier.width(Esp.m))
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(m.titulo, Modifier.weight(1f, fill = false), style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                if (m.silenciado) { Spacer(Modifier.width(6.dp)); Icon(Ico.Volumen, "Silenciada", Modifier.size(16.dp).testTag("icono_silenciada_${m.persona}"), tint = ROJO_SILENCIO) }
                if (m.advertencias > 0) { Spacer(Modifier.width(4.dp)); Icon(Ico.Aviso, "Advertida", Modifier.size(16.dp).testTag("icono_advertida_${m.persona}"), tint = AMARILLO_SILENCIO) }
                if (m.baneado) { Spacer(Modifier.width(4.dp)); Text("Baneada", Modifier.testTag("insignia_baneada_${m.persona}"), style = MaterialTheme.typography.labelSmall, color = ROJO_SILENCIO) }
                Spacer(Modifier.weight(1f))
                InsigniaCargo(m, Modifier.testTag("cargo_${m.persona}"))
            }
            Text(
                listOf(m.telefono, textoEnLinea(m)).filter { it.isNotBlank() }.joinToString(" · "),
                style = MaterialTheme.typography.bodySmall, color = if (m.enLinea) VERDE_EN_LINEA else MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis,
                modifier = Modifier.testTag("telefono_${m.persona}"),
            )
            if (m.rango.isNotBlank()) InsigniaRango(m.rango, m.rangoColor, Modifier.testTag("rango_admin_${m.persona}"))
            Text(textoUltimoMensaje(m.conteo), Modifier.testTag("ultimo_${m.persona}"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            ConteoIconos(m.conteo, Modifier.testTag("conteo_${m.persona}"))
        }
    }
}
