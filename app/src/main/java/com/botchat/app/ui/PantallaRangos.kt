@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.animation.animateContentSize
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectDragGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import com.botchat.app.ui.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ExtendedFloatingActionButton
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Slider
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.CustomAccessibilityAction
import androidx.compose.ui.semantics.customActions
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.zIndex
import com.botchat.app.datos.AlcanceCasilla
import com.botchat.app.datos.Casilla
import com.botchat.app.datos.Rango
import com.botchat.app.datos.Repo
import kotlinx.coroutines.launch
import kotlin.math.roundToInt

/** Colores que se pueden elegir para un rango. */
val COLORES_RANGO = listOf("#2EC4CB", "#F59E28", "#E5484D", "#8E4EC6", "#3E63DD", "#30A46C", "#D6409F", "#12A594", "#978365", "#6F6D66")

fun colorDeRango(hex: String): Color = runCatching { Color(android.graphics.Color.parseColor(hex)) }.getOrDefault(Color(0xFF2EC4CB))

@Composable
internal fun PuntoRango(hex: String, tam: Int = 14, modifier: Modifier = Modifier) {
    Box(modifier.size(tam.dp).clip(CircleShape).background(colorDeRango(hex)))
}

/** Rangos: el Dueño arriba (fijo) y los demás se ordenan arrastrando; quien está más arriba manda sobre los de abajo. */
@Composable
fun PantallaRangos(botId: Long, alVolver: () -> Unit, alEditar: (Long) -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("rangos" in evs) version++ } }
    val cargados by produceState<List<Rango>?>(null, version) { value = runCatching { Repo.rangos(botId) }.getOrDefault(emptyList()) }
    val orden = remember { mutableStateListOf<Rango>() }
    LaunchedEffect(cargados) { cargados?.let { orden.clear(); orden.addAll(it.filter { r -> !r.esDueno }) } }
    val dueno = cargados?.firstOrNull { it.esDueno }

    Scaffold(
        topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Rangos") }) },
        floatingActionButton = { ExtendedFloatingActionButton({ haptica(Toque.Suave); alEditar(0) }, Modifier.testTag("nuevo_rango")) { Icon(Ico.Agregar, null); Text("  Nuevo rango") } },
    ) { relleno ->
        if (cargados == null) { Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }; return@Scaffold }
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l, Esp.s, Esp.l, 96.dp), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Text(
                "Quien tiene un rango más arriba puede actuar sobre los de abajo, nunca al revés. Arrastra ≡ para cambiar el orden. El bot solo responde cuando le escriben.",
                style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            if (dueno != null) FilaRango(dueno, null, Modifier.testTag("rango_dueno")) { alEditar(dueno.id) }
            ListaOrdenable(
                orden, clave = { it.id },
                alTerminar = { lista -> scope.launch { runCatching { Repo.reordenarRangos(botId, lista.map { it.id }) }.onFailure { avisaToast(ctx, it.message, true); version++ } } },
            ) { r, manija -> FilaRango(r, manija, Modifier.testTag("rango_${r.id}")) { alEditar(r.id) } }
            if (orden.isEmpty()) EstadoVacio(Ico.Persona, "Sin más rangos", "Toca Nuevo rango para crear uno, o empieza con “Ayudante” o “Cliente VIP”.", Modifier.padding(top = Esp.xl))
        }
    }
}

@Composable
private fun FilaRango(r: Rango, manija: (@Composable () -> Unit)?, modifier: Modifier = Modifier, alPulsar: () -> Unit) {
    Card(onClick = alPulsar, modifier = modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Row(Modifier.padding(horizontal = Esp.l, vertical = Esp.m), verticalAlignment = Alignment.CenterVertically) {
            PuntoRango(r.color, 18)
            Spacer(Modifier.width(Esp.m))
            Column(Modifier.weight(1f)) {
                Text(r.nombre, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                Text(
                    if (r.esDueno) "Puede todo · no se puede cambiar" else "${plural(r.personas.toLong(), "persona", "personas")} · " + (if (r.limiteMin == 0) "sin límite" else "${r.limiteMin} comandos/min") +
                        (if (r.plantilla.isNotEmpty()) " · plantilla" else ""),
                    style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            if (r.esDueno) Icon(Ico.Candado, "Fijo", Modifier.size(20.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
            else manija?.invoke()
        }
    }
}

/**
 * Lista que se reordena arrastrando la manija "≡". Mientras se arrastra, la fila sigue al dedo y cambia de lugar al pasar la mitad de la
 * siguiente; al soltar se avisa con el orden final. También ofrece "Subir" y "Bajar" para lectores de pantalla.
 */
@Composable
fun <T> ListaOrdenable(lista: MutableList<T>, clave: (T) -> Any, alTerminar: (List<T>) -> Unit, fila: @Composable (T, @Composable () -> Unit) -> Unit) {
    var alto by remember { mutableFloatStateOf(0f) }
    val separacion = with(androidx.compose.ui.platform.LocalDensity.current) { Esp.s.toPx() }
    var arrastrando by remember { mutableStateOf<Any?>(null) }
    var desplaza by remember { mutableFloatStateOf(0f) }
    val listaActual by rememberUpdatedState(lista)
    val termina by rememberUpdatedState(alTerminar)
    Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
        lista.forEach { item ->
            val k = clave(item)
            val moviendo = arrastrando == k
            Box(
                Modifier.fillMaxWidth().onSizeChanged { if (alto == 0f) alto = it.height.toFloat() + separacion }
                    .zIndex(if (moviendo) 1f else 0f)
                    .graphicsLayer { translationY = if (moviendo) desplaza else 0f; alpha = if (moviendo) 0.92f else 1f }
                    .semantics {
                        customActions = listOf(
                            CustomAccessibilityAction("Subir") { val i = lista.indexOfFirst { clave(it) == k }; if (i > 0) { val x = lista.removeAt(i); lista.add(i - 1, x); alTerminar(lista.toList()) }; true },
                            CustomAccessibilityAction("Bajar") { val i = lista.indexOfFirst { clave(it) == k }; if (i in 0 until lista.size - 1) { val x = lista.removeAt(i); lista.add(i + 1, x); alTerminar(lista.toList()) }; true },
                        )
                    },
            ) {
                fila(item) {
                    Box(
                        Modifier.size(40.dp).testTag("manija_$k").pointerInput(k) {
                            detectDragGestures(
                                onDragStart = { arrastrando = k; desplaza = 0f },
                                onDragEnd = { arrastrando = null; desplaza = 0f; termina(listaActual.toList()) },
                                onDragCancel = { arrastrando = null; desplaza = 0f },
                            ) { cambio, delta ->
                                cambio.consume()
                                desplaza += delta.y
                                val l = listaActual
                                val i = l.indexOfFirst { clave(it) == k }
                                if (i < 0 || alto <= 0f) return@detectDragGestures
                                val destino = (i + (desplaza / alto).roundToInt()).coerceIn(0, l.size - 1)
                                if (destino != i) { val x = l.removeAt(i); l.add(destino, x); desplaza -= (destino - i) * alto }
                            }
                        },
                        Alignment.Center,
                    ) { Text("≡", style = MaterialTheme.typography.titleLarge, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                }
            }
        }
    }
}

// ---------- editor de un rango ----------

private val ORDEN_RIESGO = mapOf("bajo" to 0, "medio" to 1, "alto" to 2)

/** Editor de un rango: nombre, color, límite de comandos, menús que ve y las casillas (con alcance por casilla). */
@Composable
fun EditorRango(botId: Long, id: Long, alVolver: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    val inicial by produceState<Rango?>(null, id) { value = if (id == 0L) Rango.nuevo() else runCatching { Repo.rango(botId, id) }.getOrNull() ?: Rango.nuevo() }
    val catalogo by produceState<List<Casilla>?>(null) { value = runCatching { Repo.catalogo() }.getOrDefault(emptyList()) }
    val autos by produceState<List<com.botchat.app.datos.AutoResumen>>(emptyList()) { value = runCatching { Repo.automatizaciones(botId) }.getOrDefault(emptyList()) }
    var nombre by remember { mutableStateOf("") }
    var color by remember { mutableStateOf(COLORES_RANGO[0]) }
    var limite by remember { mutableIntStateOf(10) }
    var menusModo by remember { mutableStateOf("todos") }
    val menus = remember { mutableStateListOf<String>() }
    val casillas = remember { androidx.compose.runtime.mutableStateMapOf<String, AlcanceCasilla>() }
    var cargado by remember { mutableStateOf(false) }
    var borrando by remember { mutableStateOf(false) }
    var eligeChats by remember { mutableStateOf<Casilla?>(null) }
    var guardando by remember { mutableStateOf(false) }
    LaunchedEffect(inicial) {
        inicial?.let { r ->
            if (!cargado) {
                nombre = r.nombre; color = r.color; limite = r.limiteMin; menusModo = r.menusModo
                menus.clear(); menus.addAll(r.menus); casillas.clear(); casillas.putAll(r.casillas); cargado = true
            }
        }
    }
    val r = inicial
    Scaffold(
        topBar = {
            TopAppBar(
                navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } },
                title = { Text(if (id == 0L) "Rango nuevo" else if (r?.esDueno == true) "Dueño" else "Editar rango") },
                actions = {
                    if (id != 0L && r?.esDueno == false) IconButton({ borrando = true }, Modifier.testTag("borrar_rango")) { Icon(Ico.Borrar, "Borrar rango") }
                },
            )
        },
    ) { relleno ->
        if (r == null || catalogo == null) { Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }; return@Scaffold }
        if (r.esDueno) {
            Column(Modifier.fillMaxSize().padding(relleno).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
                Row(verticalAlignment = Alignment.CenterVertically) { PuntoRango(r.color, 20); Spacer(Modifier.width(Esp.m)); Text("Dueño", style = MaterialTheme.typography.headlineSmall) }
                Text("Es el único rango fijo: puede todo y no se puede quitar, editar ni borrar. Nadie puede actuar sobre el Dueño. Sus comandos solo valen desde “Mensajes guardados” y desde la app.", style = MaterialTheme.typography.bodyMedium)
            }
            return@Scaffold
        }
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.l)) {
            OutlinedTextField(nombre, { nombre = it.take(30) }, Modifier.fillMaxWidth().testTag("rango_nombre"), label = { Text("Nombre del rango") }, singleLine = true)
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("Color", style = MaterialTheme.typography.titleSmall)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    COLORES_RANGO.forEach { c ->
                        Box(
                            Modifier.size(40.dp).clip(CircleShape).background(colorDeRango(c)).clickable { haptica(Toque.Suave); color = c }.testTag("color_$c"),
                            Alignment.Center,
                        ) { if (c == color) Icon(Ico.Listo, "Elegido", tint = Color.White) }
                    }
                }
            }
            Column {
                Row { Text("Comandos por minuto", Modifier.weight(1f), style = MaterialTheme.typography.titleSmall); Text(if (limite == 0) "Sin límite" else "$limite", Modifier.testTag("rango_limite_texto")) }
                Slider(limite.toFloat(), { limite = it.roundToInt() }, valueRange = 0f..60f, steps = 59, modifier = Modifier.testTag("rango_limite"))
                Text("Para que nadie sature al bot. Si se pasa, el bot avisa una sola vez y calla hasta el siguiente minuto.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("Menús que ve", style = MaterialTheme.typography.titleSmall)
                val modos = listOf("todos" to "Todos", "lista" to "Solo algunos", "ninguno" to "Ninguno")
                SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().testTag("menus_modo")) {
                    modos.forEachIndexed { i, (m, t) -> SegmentedButton(menusModo == m, { haptica(Toque.Suave); menusModo = m }, SegmentedButtonDefaults.itemShape(i, modos.size), modifier = Modifier.testTag("menus_$m")) { Text(t, maxLines = 1) } }
                }
                if (menusModo == "lista") {
                    if (autos.isEmpty()) Text("Todavía no hay automatizaciones.", style = MaterialTheme.typography.bodySmall)
                    autos.forEach { a ->
                        Row(Modifier.fillMaxWidth().clickable { if (a.id.toString() in menus) menus.remove(a.id.toString()) else menus.add(a.id.toString()) }, verticalAlignment = Alignment.CenterVertically) {
                            Checkbox(a.id.toString() in menus, { if (it) menus.add(a.id.toString()) else menus.remove(a.id.toString()) }, Modifier.testTag("menu_${a.id}"))
                            Text(a.nombre)
                        }
                        if (a.id.toString() !in menus) OpcionesSueltas(botId, a.id, menus)
                    }
                }
            }
            Text("Qué puede hacer", style = MaterialTheme.typography.titleMedium)
            Text("Marca solo lo que quieras darle. En cada casilla puedes limitar dónde vale: solo en privado, solo en grupos o solo en chats que elijas.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            val cat = catalogo!!
            val base = cat.filter { it.base }
            cat.filter { !it.base }.groupBy { it.grupo }.forEach { (grupo, lista) ->
                Card(Modifier.fillMaxWidth().testTag("grupo_$grupo"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                    Column(Modifier.padding(Esp.m), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                        Text(grupo, style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary)
                        lista.sortedBy { ORDEN_RIESGO[it.riesgo] ?: 0 }.forEach { c ->
                            FilaCasilla(c, casillas[c.id], { activa ->
                                haptica(Toque.Suave)
                                if (activa) casillas[c.id] = AlcanceCasilla() else casillas.remove(c.id)
                            }, { a -> if (a.modo == "chats") eligeChats = c else casillas[c.id] = a })
                        }
                    }
                }
            }
            if (base.isNotEmpty()) {
                Text("Esto lo puede hacer cualquiera, con o sin rango: " + base.joinToString(", ") { it.nombre.lowercase() } + ".", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            Button(
                {
                    haptica(Toque.Suave); guardando = true
                    scope.launch {
                        runCatching { Repo.guardarRango(botId, r.copy(nombre = nombre.trim(), color = color, limiteMin = limite, menusModo = menusModo, menus = menus.toList(), casillas = casillas.toMap())) }
                            .onSuccess { alVolver() }.onFailure { avisaToast(ctx, it.message ?: "No se pudo guardar", true) }
                        guardando = false
                    }
                },
                Modifier.fillMaxWidth().height(52.dp).testTag("guardar_rango"), enabled = nombre.isNotBlank() && !guardando,
            ) { Text("Guardar") }
            Spacer(Modifier.height(Esp.xl))
        }
    }
    eligeChats?.let { c ->
        ElegirChats(botId, casillas[c.id]?.chats ?: emptyList(), c.nombre, alCancelar = { eligeChats = null }) { elegidos ->
            casillas[c.id] = AlcanceCasilla("chats", elegidos); eligeChats = null
        }
    }
    if (borrando && r != null) {
        AlertDialog(
            onDismissRequest = { borrando = false },
            title = { Text("¿Borrar “${r.nombre}”?") },
            text = { Text(if (r.personas > 0) "Quienes lo tienen (${r.personas}) se quedarán sin rango." else "Nadie tiene este rango ahora.") },
            confirmButton = {
                TextButton({ borrando = false; scope.launch { runCatching { Repo.borrarRango(botId, r.id) }.onSuccess { alVolver() }.onFailure { avisaToast(ctx, it.message, true) } } }, Modifier.testTag("confirmar_borrar_rango")) {
                    Text("Sí, borrar", color = MaterialTheme.colorScheme.error)
                }
            },
            dismissButton = { TextButton({ borrando = false }) { Text("Cancelar") } },
        )
    }
}

@Composable
private fun FilaCasilla(c: Casilla, alcance: AlcanceCasilla?, alActivar: (Boolean) -> Unit, alAlcance: (AlcanceCasilla) -> Unit) {
    Column(Modifier.fillMaxWidth().testTag("casilla_${c.id}")) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(c.nombre, style = MaterialTheme.typography.bodyLarge, fontWeight = FontWeight.Medium, modifier = Modifier.weight(1f, fill = false))
                    when (c.riesgo) {
                        "alto" -> Etiqueta("Riesgo alto", MaterialTheme.colorScheme.error)
                        "medio" -> Etiqueta("Cuidado", MaterialTheme.colorScheme.tertiary)
                    }
                    if (c.confirma) Etiqueta("Pide código", MaterialTheme.colorScheme.primary)
                }
                Text(c.descripcion, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            Switch(alcance != null, alActivar, Modifier.testTag("sw_casilla_${c.id}"))
        }
        if (alcance != null && (c.privado && c.grupos)) {
            val opciones = listOf("" to "Donde aplique", "privado" to "Solo privado", "grupos" to "Solo grupos", "chats" to if (alcance.modo == "chats") "Chats (${alcance.chats.size})" else "Chats concretos")
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), modifier = Modifier.padding(top = Esp.xs)) {
                opciones.forEach { (m, t) -> FilterChip(alcance.modo == m, { alAlcance(if (m == "chats") AlcanceCasilla("chats", alcance.chats) else AlcanceCasilla(m)) }, { Text(t, maxLines = 1) }, modifier = Modifier.testTag("alcance_${c.id}_${m.ifEmpty { "todo" }}")) }
            }
        } else if (alcance != null) {
            val opciones = listOf("" to if (c.grupos) "En grupos" else "En privado", "chats" to if (alcance.modo == "chats") "Chats (${alcance.chats.size})" else "Chats concretos")
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), modifier = Modifier.padding(top = Esp.xs)) {
                opciones.forEach { (m, t) -> FilterChip(alcance.modo == m, { alAlcance(if (m == "chats") AlcanceCasilla("chats", alcance.chats) else AlcanceCasilla(m)) }, { Text(t, maxLines = 1) }, modifier = Modifier.testTag("alcance_${c.id}_${m.ifEmpty { "todo" }}")) }
            }
        }
    }
}

@Composable
private fun Etiqueta(texto: String, color: Color) {
    Text(
        texto, Modifier.padding(start = Esp.s).clip(MaterialTheme.shapes.small).background(color.copy(alpha = 0.16f)).padding(horizontal = 6.dp, vertical = 2.dp),
        style = MaterialTheme.typography.labelSmall, color = color, maxLines = 1,
    )
}

/** Elegir uno o varios chats (personas y grupos) de los que ya existen. */
@Composable
fun ElegirChats(botId: Long, elegidos: List<String>, titulo: String, alCancelar: () -> Unit, alAceptar: (List<String>) -> Unit) {
    val chats by produceState<List<com.botchat.app.datos.ChatItem>?>(null) {
        value = runCatching { Repo.chats(botId, false, "", 400) + Repo.chats(botId, true, "", 400) }.getOrDefault(emptyList())
    }
    val marcados = remember { mutableStateListOf<String>().also { it.addAll(elegidos) } }
    AlertDialog(
        onDismissRequest = alCancelar,
        title = { Text(titulo) },
        text = {
            val l = chats
            if (l == null) CircularProgressIndicator()
            else if (l.isEmpty()) Text("Todavía no hay chats.")
            else Column(Modifier.height(320.dp).verticalScroll(rememberScrollState()).testTag("lista_elegir_chats")) {
                l.forEach { c ->
                    Row(Modifier.fillMaxWidth().clickable { if (c.jid in marcados) marcados.remove(c.jid) else marcados.add(c.jid) }, verticalAlignment = Alignment.CenterVertically) {
                        Checkbox(c.jid in marcados, { if (it) marcados.add(c.jid) else marcados.remove(c.jid) }, Modifier.testTag("elegir_${c.jid}"))
                        Column { Text(c.nombre, maxLines = 1, overflow = TextOverflow.Ellipsis); Text(if (c.esGrupo) "Grupo" else c.telefono, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                    }
                }
            }
        },
        confirmButton = { TextButton({ alAceptar(marcados.toList()) }, enabled = marcados.isNotEmpty(), modifier = Modifier.testTag("aceptar_chats")) { Text("Listo") } },
        dismissButton = { TextButton(alCancelar) { Text("Cancelar") } },
    )
}


/** Opciones sueltas de un menú que el rango puede ver aunque no vea el menú entero (se guardan como "idMenú:idOpción"). */
@Composable
private fun OpcionesSueltas(botId: Long, menuId: Long, menus: androidx.compose.runtime.snapshots.SnapshotStateList<String>) {
    var abierto by remember { mutableStateOf(menus.any { it.startsWith("$menuId:") }) }
    val opciones by produceState<List<Pair<String, String>>?>(null, abierto) {
        value = if (!abierto) null else runCatching {
            Repo.automatizacion(botId, menuId).def.pasos.filter { it.tipo == "menu" }.flatMap { p -> p.opciones.filter { it.id.isNotEmpty() }.map { it.id to it.texto } }
        }.getOrDefault(emptyList())
    }
    Column(Modifier.animateContentSize().padding(start = Esp.xl)) {
        TextButton({ abierto = !abierto }, Modifier.testTag("opciones_sueltas_$menuId")) { Text(if (abierto) "Ocultar opciones" else "Elegir solo algunas opciones…", style = MaterialTheme.typography.labelMedium) }
        if (abierto) {
            val lista = opciones
            when {
                lista == null -> Text("Cargando…", style = MaterialTheme.typography.bodySmall)
                lista.isEmpty() -> Text("Este menú aún no tiene opciones guardadas con identificador: guárdalo una vez en el editor.", style = MaterialTheme.typography.bodySmall)
                else -> lista.forEach { (oid, texto) ->
                    val clave = "$menuId:$oid"
                    Row(Modifier.fillMaxWidth().clickable { if (clave in menus) menus.remove(clave) else menus.add(clave) }, verticalAlignment = Alignment.CenterVertically) {
                        Checkbox(clave in menus, { if (it) menus.add(clave) else menus.remove(clave) }, Modifier.testTag("opcion_${menuId}_$oid"))
                        Text(texto, style = MaterialTheme.typography.bodyMedium)
                    }
                }
            }
        }
    }
}
