@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.PrimaryTabRow
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Slider
import androidx.compose.material3.Switch
import androidx.compose.material3.Tab
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.AutoDef
import com.botchat.app.datos.AvisoVal
import com.botchat.app.datos.ChatItem
import com.botchat.app.datos.OpcionDef
import com.botchat.app.datos.PasoDef
import com.botchat.app.datos.Repo
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import java.time.format.DateTimeFormatter

internal val TIPOS_PASO = listOf("menu" to "Menú", "respuesta" to "Respuesta", "pedir" to "Pedir dato", "humano" to "Pasar a humano", "fin" to "Fin")
private fun nombreTipoPaso(t: String) = TIPOS_PASO.firstOrNull { it.first == t }?.second ?: t

/** Resumen corto de un paso para la lista. */
internal fun resumenPaso(p: PasoDef): String {
    val base = when (p.tipo) { "menu" -> p.titulo; "respuesta", "humano", "fin" -> p.texto; "pedir" -> p.pregunta; else -> "" }
    val t = base.replace('\n', ' ').trim()
    return if (t.length > 46) t.take(46) + "…" else t.ifBlank { "(vacío)" }
}

/** Pasos en el orden en que se recorren desde el inicio (con su nivel de sangría) y, aparte, los que quedaron sueltos. */
internal fun ordenar(d: AutoDef): Pair<List<Pair<PasoDef, Int>>, List<PasoDef>> {
    val visto = HashSet<String>()
    val salida = ArrayList<Pair<PasoDef, Int>>()
    fun visita(id: String, nivel: Int) {
        val p = d.paso(id) ?: return
        if (!visto.add(id)) return
        salida += p to nivel
        when (p.tipo) {
            "menu" -> p.opciones.forEach { if (it.destino.isNotEmpty()) visita(it.destino, nivel + 1) }
            "respuesta", "pedir" -> if (p.siguiente.isNotEmpty()) visita(p.siguiente, nivel)
        }
    }
    visita(d.inicio, 0)
    return salida to d.pasos.filter { it.id !in visto }
}

private fun nuevoId(d: AutoDef): String { var n = d.pasos.size + 1; while (d.paso("p$n") != null) n++; return "p$n" }

private fun pasoNuevo(d: AutoDef, tipo: String): PasoDef {
    val id = nuevoId(d)
    return when (tipo) {
        "menu" -> PasoDef(id, "menu", "Elige una opción:", listOf(OpcionDef("Opción 1", "")))
        "respuesta" -> PasoDef(id, "respuesta", texto = "Escribe aquí la respuesta.")
        "pedir" -> PasoDef(id, "pedir", pregunta = "¿Cuál es tu nombre?", variable = "dato" + (d.pasos.count { it.tipo == "pedir" } + 1))
        "humano" -> PasoDef(id, "humano", texto = "Un momento, una persona te atenderá.")
        else -> PasoDef(id, "fin", texto = "¡Gracias por escribirnos!")
    }
}

/** Editor de una automatización: pasos en lista con sangría, cuándo y dónde se activa, y vista previa en burbujas. */
@Composable
fun EditorAutomatizacion(botId: Long, id: Long, alVolver: () -> Unit, alProbar: (Long) -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    var activo by remember { mutableStateOf(false) }
    var sistema by remember { mutableStateOf(false) }
    var version by remember { mutableStateOf(1) }
    var def by remember { mutableStateOf<AutoDef?>(null) }
    var original by remember { mutableStateOf("") }
    var avisos by remember { mutableStateOf<List<AvisoVal>>(emptyList()) }
    var pestana by remember { mutableStateOf(0) }
    var salir by remember { mutableStateOf(false) }
    LaunchedEffect(id) {
        runCatching { Repo.automatizacion(botId, id) }.onSuccess { d -> activo = d.activo; sistema = d.sistema; version = d.version; def = d.def; original = d.def.json(); avisos = d.avisos }
            .onFailure { avisaToast(ctx, it.message ?: "No se pudo abrir", true); alVolver() }
    }
    LaunchedEffect(def) { val d = def ?: return@LaunchedEffect; delay(250); avisos = runCatching { Repo.validar(d) }.getOrDefault(avisos) }
    val d = def
    val hayCambios = d != null && d.json() != original
    BackHandler(hayCambios) { salir = true }

    fun guarda() {
        val actual = def ?: return
        scope.launch {
            runCatching { Repo.guardarAutomatizacion(botId, id, actual) }.onSuccess { (_, v, av) ->
                version = v; original = actual.json(); avisos = av
                if (av.any { it.grave } && activo) { runCatching { Repo.activarAutomatizacion(botId, id, false) }; activo = false; avisaToast(ctx, "Guardada. Se apagó porque tiene avisos por corregir.", true) }
                else avisaToast(ctx, "Guardada (versión $v). Las conversaciones que ya empezaron terminan con la versión anterior.", true)
            }.onFailure { avisaToast(ctx, it.message ?: "No se pudo guardar", true) }
        }
    }

    Scaffold(
        topBar = {
            Column {
                TopAppBar(
                    navigationIcon = { IconButton({ if (hayCambios) salir = true else alVolver() }, Modifier.testTag("volver_editor")) { Icon(Ico.Atras, "Volver") } },
                    title = { Text("Editar automatización") },
                    actions = {
                        TextButton({ haptica(Toque.Suave); if (hayCambios) guarda() else alProbar(id) }, Modifier.testTag(if (hayCambios) "guardar_auto" else "probar_auto")) { Text(if (hayCambios) "Guardar" else "Probar") }
                    },
                )
                PrimaryTabRow(pestana) {
                    listOf("Pasos", "Cuándo y dónde", "Vista previa").forEachIndexed { i, t -> Tab(pestana == i, { pestana = i }, text = { Text(t) }, modifier = Modifier.testTag("pestana_$i")) }
                }
            }
        },
    ) { relleno ->
        if (d == null) { Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }; return@Scaffold }
        Column(Modifier.fillMaxSize().padding(relleno)) {
            when (pestana) {
                0 -> PestanaPasos(d, avisos, sistema) { def = it }
                1 -> PestanaCuando(botId, d) { def = it }
                else -> PestanaVistaPrevia(d)
            }
        }
    }
    if (salir) {
        AlertDialog(
            onDismissRequest = { salir = false },
            title = { Text("¿Salir sin guardar?") },
            text = { Text("Hiciste cambios que todavía no se guardan.") },
            confirmButton = { TextButton({ salir = false; alVolver() }, Modifier.testTag("salir_sin_guardar")) { Text("Salir sin guardar", color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton({ salir = false; guarda() }) { Text("Guardar") } },
        )
    }
}

// ---------- pestaña Pasos ----------

@Composable
private fun PestanaPasos(d: AutoDef, avisos: List<AvisoVal>, sistema: Boolean, alCambiar: (AutoDef) -> Unit) {
    var abierto by remember { mutableStateOf<String?>(null) }
    val (orden, sueltos) = remember(d) { ordenar(d) }
    fun cambiaPaso(id: String, f: (PasoDef) -> PasoDef) = alCambiar(d.copy(pasos = d.pasos.map { if (it.id == id) f(it) else it }))
    fun borra(id: String) {
        val resto = d.pasos.filter { it.id != id }.map { p -> p.copy(siguiente = if (p.siguiente == id) "" else p.siguiente, opciones = p.opciones.map { o -> if (o.destino == id) o.copy(destino = "") else o }) }
        alCambiar(d.copy(pasos = resto, inicio = if (d.inicio == id) resto.firstOrNull()?.id ?: "" else d.inicio))
        if (abierto == id) abierto = null
    }
    fun agrega(tipo: String, alCrear: (PasoDef) -> AutoDef = { nuevo -> d.copy(pasos = d.pasos + nuevo) }) {
        val nuevo = pasoNuevo(d, tipo)
        alCambiar(alCrear(nuevo)); abierto = nuevo.id
    }
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(Esp.l).testTag("pestana_pasos"), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
        OutlinedTextField(d.nombre, { alCambiar(d.copy(nombre = it)) }, Modifier.fillMaxWidth().testTag("campo_nombre_auto"), label = { Text("Nombre") }, singleLine = true, enabled = true)
        if (sistema) Text("Es la respuesta de fábrica: va al final y solo responde si ninguna otra automatización coincide. Puedes cambiarla o apagarla.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        if (avisos.isNotEmpty()) {
            Card(Modifier.fillMaxWidth().testTag("avisos_validacion"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.errorContainer.copy(alpha = 0.5f))) {
                Column(Modifier.padding(Esp.m), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                    avisos.forEach { a ->
                        val donde = d.paso(a.paso)?.let { " (" + nombreTipoPaso(it.tipo) + ": " + resumenPaso(it).take(20) + ")" } ?: ""
                        Text((if (a.grave) "● " else "○ ") + a.texto + donde, Modifier.testTag("aviso_validacion"), style = MaterialTheme.typography.bodySmall, color = if (a.grave) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurface)
                    }
                }
            }
        }
        Text("Puedes usar {nombre}, {hora}, {dia} y los datos que pidas ({dato1}, {dato2}…). Escribir 0 siempre vuelve al menú principal.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        orden.forEach { (p, nivel) -> TarjetaPaso(d, p, nivel, p.id == abierto, { abierto = if (abierto == p.id) null else p.id }, ::cambiaPaso, ::borra, ::agrega, { alCambiar(d.copy(inicio = p.id)) }) }
        if (sueltos.isNotEmpty()) {
            Text("Pasos sueltos (nadie llega a ellos todavía)", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.tertiary)
            sueltos.forEach { p -> TarjetaPaso(d, p, 0, p.id == abierto, { abierto = if (abierto == p.id) null else p.id }, ::cambiaPaso, ::borra, ::agrega, { alCambiar(d.copy(inicio = p.id)) }) }
        }
        Text("Agregar paso", style = MaterialTheme.typography.titleSmall)
        Row(horizontalArrangement = Arrangement.spacedBy(6.dp), modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState())) {
            TIPOS_PASO.forEach { (t, n) -> OutlinedButton({ agrega(t) }, Modifier.testTag("agregar_$t")) { Text(n, maxLines = 1) } }
        }
        Spacer(Modifier.size(80.dp))
    }
}

@Composable
private fun TarjetaPaso(
    d: AutoDef, p: PasoDef, nivel: Int, abierto: Boolean, alAbrir: () -> Unit,
    cambia: (String, (PasoDef) -> PasoDef) -> Unit, borra: (String) -> Unit, agrega: (String, (PasoDef) -> AutoDef) -> Unit, alHacerInicial: () -> Unit,
) {
    Card(
        Modifier.fillMaxWidth().padding(start = (nivel * 16).dp).testTag("paso_${p.id}"), shape = MaterialTheme.shapes.medium,
        colors = CardDefaults.cardColors(containerColor = if (abierto) MaterialTheme.colorScheme.secondaryContainer else MaterialTheme.colorScheme.surfaceContainer),
    ) {
        Column(Modifier.padding(Esp.m), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Row(Modifier.fillMaxWidth().clickable { alAbrir() }.testTag("abrir_${p.id}"), verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text(nombreTipoPaso(p.tipo) + if (p.id == d.inicio) " · Inicio" else "", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.primary)
                    Text(resumenPaso(p), maxLines = 2, overflow = TextOverflow.Ellipsis)
                }
                Icon(if (abierto) Ico.Cerrar else Ico.Editar, if (abierto) "Cerrar" else "Editar", Modifier.size(20.dp))
            }
            if (!abierto && p.tipo == "menu") p.opciones.forEachIndexed { i, o ->
                Text("${i + 1}. ${o.texto} → " + (d.paso(o.destino)?.let { resumenPaso(it).take(28) } ?: "sin destino"), style = MaterialTheme.typography.bodySmall,
                    color = if (d.paso(o.destino) == null) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurfaceVariant)
            }
            if (abierto) EditorDePaso(d, p, cambia, borra, agrega, alHacerInicial)
        }
    }
}

@Composable
private fun Campo(valor: String, etiqueta: String, tag: String, lineas: Int = 4, alCambiar: (String) -> Unit) {
    OutlinedTextField(valor, alCambiar, Modifier.fillMaxWidth().testTag(tag), label = { Text(etiqueta) }, maxLines = lineas)
}

@Composable
private fun EditorDePaso(
    d: AutoDef, p: PasoDef, cambia: (String, (PasoDef) -> PasoDef) -> Unit, borra: (String) -> Unit, agrega: (String, (PasoDef) -> AutoDef) -> Unit, alHacerInicial: () -> Unit,
) {
    when (p.tipo) {
        "menu" -> {
            Campo(p.titulo, "Título del menú", "titulo_${p.id}") { t -> cambia(p.id) { it.copy(titulo = t) } }
            p.opciones.forEachIndexed { i, o ->
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    Text("${i + 1}.", fontWeight = FontWeight.SemiBold)
                    OutlinedTextField(o.texto, { t -> cambia(p.id) { q -> q.copy(opciones = q.opciones.mapIndexed { j, x -> if (j == i) x.copy(texto = t) else x }) } }, Modifier.weight(1f).testTag("opcion_${p.id}_$i"), singleLine = true, label = { Text("Opción") })
                    IconButton({ cambia(p.id) { q -> q.copy(opciones = q.opciones.filterIndexed { j, _ -> j != i }) } }, Modifier.testTag("quitar_opcion_${p.id}_$i")) { Icon(Ico.Borrar, "Quitar opción") }
                }
                SelectorDestino(d, o.destino, "destino_${p.id}_$i", conTerminar = false, alElegir = { dest -> cambia(p.id) { q -> q.copy(opciones = q.opciones.mapIndexed { j, x -> if (j == i) x.copy(destino = dest) else x }) } },
                    alNuevo = { tipo -> agrega(tipo) { nuevo -> d.copy(pasos = d.pasos.map { q -> if (q.id == p.id) q.copy(opciones = q.opciones.mapIndexed { j, x -> if (j == i) x.copy(destino = nuevo.id) else x }) else q } + nuevo) } })
            }
            OutlinedButton({ cambia(p.id) { it.copy(opciones = it.opciones + OpcionDef("Opción ${it.opciones.size + 1}", "")) } }, Modifier.testTag("agregar_opcion_${p.id}")) { Icon(Ico.Agregar, null); Text(" Agregar opción") }
        }
        "respuesta" -> {
            Campo(p.texto, "Texto de la respuesta", "texto_${p.id}") { t -> cambia(p.id) { it.copy(texto = t) } }
            Text("Después:", style = MaterialTheme.typography.labelMedium)
            SelectorDestino(d, p.siguiente, "siguiente_${p.id}", conTerminar = true, alElegir = { dest -> cambia(p.id) { it.copy(siguiente = dest) } },
                alNuevo = { tipo -> agrega(tipo) { nuevo -> d.copy(pasos = d.pasos.map { q -> if (q.id == p.id) q.copy(siguiente = nuevo.id) else q } + nuevo) } })
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text("Avisarme con una notificación", Modifier.weight(1f))
                Switch(p.notificar, { si -> cambia(p.id) { it.copy(notificar = si) } }, Modifier.testTag("notificar_${p.id}"))
            }
            if (p.notificar) Campo(p.aviso, "Texto del aviso (opcional)", "aviso_${p.id}") { t -> cambia(p.id) { it.copy(aviso = t) } }
        }
        "pedir" -> {
            Campo(p.pregunta, "Pregunta", "pregunta_${p.id}") { t -> cambia(p.id) { it.copy(pregunta = t) } }
            Text("Lo que conteste se guarda cifrado como {${p.variable.ifBlank { "dato1" }}}.", style = MaterialTheme.typography.bodySmall)
            Text("Después:", style = MaterialTheme.typography.labelMedium)
            SelectorDestino(d, p.siguiente, "siguiente_${p.id}", conTerminar = true, alElegir = { dest -> cambia(p.id) { it.copy(siguiente = dest) } },
                alNuevo = { tipo -> agrega(tipo) { nuevo -> d.copy(pasos = d.pasos.map { q -> if (q.id == p.id) q.copy(siguiente = nuevo.id) else q } + nuevo) } })
        }
        "humano" -> {
            Campo(p.texto, "Lo que se le responde antes de callar", "texto_${p.id}") { t -> cambia(p.id) { it.copy(texto = t) } }
            Text("El bot calla en este chat hasta que lo reactives o pasen 12 horas, y te llega un aviso neutro.", style = MaterialTheme.typography.bodySmall)
        }
        else -> Campo(p.texto, "Mensaje final (opcional)", "texto_${p.id}") { t -> cambia(p.id) { it.copy(texto = t) } }
    }
    Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
        if (p.id != d.inicio) OutlinedButton(alHacerInicial, Modifier.testTag("hacer_inicial_${p.id}")) { Text("Hacer inicial") }
        OutlinedButton({ borra(p.id) }, Modifier.testTag("borrar_paso_${p.id}")) { Text("Borrar paso", color = MaterialTheme.colorScheme.error) }
    }
}

/** Elige a qué paso va una opción (o "Terminar"), o crea un paso nuevo allí mismo. */
@Composable
private fun SelectorDestino(d: AutoDef, actual: String, tag: String, conTerminar: Boolean, alElegir: (String) -> Unit, alNuevo: (String) -> Unit) {
    var menu by remember { mutableStateOf(false) }
    val p = d.paso(actual)
    val roto = actual.isNotEmpty() && p == null
    Box {
        OutlinedButton({ menu = true }, Modifier.fillMaxWidth().testTag(tag)) {
            Text(
                when { p != null -> "→ " + nombreTipoPaso(p.tipo) + ": " + resumenPaso(p).take(30); roto -> "→ (paso borrado)"; conTerminar -> "→ Terminar la conversación"; else -> "→ Elegir a dónde lleva" },
                Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis, color = if (!conTerminar && p == null) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurface,
            )
        }
        DropdownMenu(menu, { menu = false }) {
            if (conTerminar) DropdownMenuItem(text = { Text("Terminar la conversación") }, onClick = { menu = false; alElegir("") })
            d.pasos.forEach { q -> DropdownMenuItem(text = { Text(nombreTipoPaso(q.tipo) + ": " + resumenPaso(q).take(34)) }, onClick = { menu = false; alElegir(q.id) }, modifier = Modifier.testTag("${tag}_a_${q.id}")) }
            TIPOS_PASO.forEach { (t, n) -> DropdownMenuItem(text = { Text("Nuevo paso: $n", color = MaterialTheme.colorScheme.primary) }, onClick = { menu = false; alNuevo(t) }, modifier = Modifier.testTag("${tag}_nuevo_$t")) }
        }
    }
}

// ---------- pestaña Cuándo y dónde ----------

private val DIAS = listOf(1 to "L", 2 to "M", 3 to "X", 4 to "J", 5 to "V", 6 to "S", 7 to "D")
private val HORA = Regex("""^([01]?\d|2[0-3]):[0-5]\d$""")

@Composable
private fun Fila(texto: String, tag: String, valor: Boolean, alCambiar: (Boolean) -> Unit, ayuda: String = "") {
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        Column(Modifier.weight(1f)) { Text(texto); if (ayuda.isNotEmpty()) Text(ayuda, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        Switch(valor, alCambiar, Modifier.testTag(tag))
    }
}

@Composable
private fun PestanaCuando(botId: Long, d: AutoDef, alCambiar: (AutoDef) -> Unit) {
    val t = d.disp
    val chats by produceState<List<ChatItem>>(emptyList()) { value = runCatching { Repo.chats(botId, false, "") }.getOrDefault(emptyList()) }
    val grupos by produceState<List<ChatItem>>(emptyList()) { value = runCatching { Repo.chats(botId, true, "") }.getOrDefault(emptyList()) }
    var palabras by remember(d.nombre.hashCode() * 31 + t.palabras.size) { mutableStateOf(t.palabras.joinToString(", ")) }
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(Esp.l).testTag("pestana_cuando"), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
        Text("Cuándo se activa", style = MaterialTheme.typography.titleMedium)
        Text("Basta con que se cumpla una. El bot solo responde: nunca escribe primero.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Fila("Cualquier mensaje", "disp_cualquiera", t.cualquiera, { alCambiar(d.copy(disp = t.copy(cualquiera = it))) })
        Fila("Primer mensaje de esa persona", "disp_primero", t.primerMensaje, { alCambiar(d.copy(disp = t.copy(primerMensaje = it))) })
        Column {
            Text("Después de ${if (t.diasSinEscribir == 0) "—" else "${t.diasSinEscribir} días"} sin escribir")
            Slider(t.diasSinEscribir.toFloat(), { alCambiar(d.copy(disp = t.copy(diasSinEscribir = it.toInt()))) }, valueRange = 0f..30f, steps = 29, modifier = Modifier.testTag("disp_dias"))
        }
        OutlinedTextField(palabras, { x -> palabras = x; alCambiar(d.copy(disp = t.copy(palabras = x.split(',').map { it.trim() }.filter { it.isNotEmpty() }))) }, Modifier.fillMaxWidth().testTag("disp_palabras"), label = { Text("Palabras clave (separadas por coma)") }, singleLine = true)
        OutlinedTextField(t.comando, { alCambiar(d.copy(disp = t.copy(comando = it.trim()))) }, Modifier.fillMaxWidth().testTag("disp_comando"), label = { Text("Comando (por ejemplo .menu)") }, singleLine = true)
        Fila("Fuera de horario", "disp_fuera", t.fueraDeHorario, { alCambiar(d.copy(disp = t.copy(fueraDeHorario = it))) }, "Se activa con cualquier mensaje cuando no es tu horario de atención.")
        Fila("Cuando me mencionen en un grupo", "disp_mencion", t.mencion, { alCambiar(d.copy(disp = t.copy(mencion = it))) })
        Column {
            Text("Esperar entre dos activaciones del mismo chat: ${if (t.enfriamientoSeg == 0) "no esperar" else "${t.enfriamientoSeg} s"}")
            Slider(t.enfriamientoSeg.toFloat(), { alCambiar(d.copy(disp = t.copy(enfriamientoSeg = (it / 30f).toInt() * 30))) }, valueRange = 0f..600f, steps = 19, modifier = Modifier.testTag("disp_enfriamiento"))
        }

        Text("Dónde", style = MaterialTheme.typography.titleMedium)
        Fila("Todos los chats privados", "alcance_privados", d.alcance.privados, { alCambiar(d.copy(alcance = d.alcance.copy(privados = it))) })
        if (!d.alcance.privados) {
            Text("Solo estos chats privados:", style = MaterialTheme.typography.labelMedium)
            chats.take(60).forEach { c -> FilaElegible(c.nombre, c.jid in d.alcance.chats, "chat_${c.jid}") { si -> alCambiar(d.copy(alcance = d.alcance.copy(chats = if (si) d.alcance.chats + c.jid else d.alcance.chats - c.jid))) } }
            if (chats.isEmpty()) Text("Todavía no hay chats.", style = MaterialTheme.typography.bodySmall)
        }
        Text("Grupos donde responde (solo si lo mencionan o usan un comando):", style = MaterialTheme.typography.labelMedium)
        grupos.take(60).forEach { c -> FilaElegible(c.nombre, c.jid in d.alcance.grupos, "grupo_${c.jid}") { si -> alCambiar(d.copy(alcance = d.alcance.copy(grupos = if (si) d.alcance.grupos + c.jid else d.alcance.grupos - c.jid))) } }
        if (grupos.isEmpty()) Text("No hay grupos guardados todavía.", style = MaterialTheme.typography.bodySmall)

        Text("Días y horario", style = MaterialTheme.typography.titleMedium)
        Text("Sin elegir días ni horas, atiende siempre.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
            DIAS.forEach { (n, l) -> FilterChip(n in d.horario.dias, { alCambiar(d.copy(horario = d.horario.copy(dias = if (n in d.horario.dias) d.horario.dias - n else (d.horario.dias + n).sorted()))) }, label = { Text(l) }, modifier = Modifier.testTag("dia_$n")) }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
            val malDesde = d.horario.desde.isNotEmpty() && !HORA.matches(d.horario.desde)
            val malHasta = d.horario.hasta.isNotEmpty() && !HORA.matches(d.horario.hasta)
            OutlinedTextField(d.horario.desde, { alCambiar(d.copy(horario = d.horario.copy(desde = it.take(5)))) }, Modifier.weight(1f).testTag("horario_desde"), label = { Text("Desde (HH:MM)") }, singleLine = true, isError = malDesde)
            OutlinedTextField(d.horario.hasta, { alCambiar(d.copy(horario = d.horario.copy(hasta = it.take(5)))) }, Modifier.weight(1f).testTag("horario_hasta"), label = { Text("Hasta (HH:MM)") }, singleLine = true, isError = malHasta)
        }
        Spacer(Modifier.size(80.dp))
    }
}

@Composable
private fun FilaElegible(nombre: String, marcado: Boolean, tag: String, alCambiar: (Boolean) -> Unit) {
    Row(Modifier.fillMaxWidth().clickable { alCambiar(!marcado) }.testTag(tag), verticalAlignment = Alignment.CenterVertically) {
        Checkbox(marcado, null)
        Text(nombre, Modifier.padding(start = 8.dp), maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

// ---------- pestaña Vista previa ----------

private data class BurbujaPrev(val bot: Boolean, val texto: String)

private fun sustituyeEjemplo(t: String): String {
    val hoy = java.time.LocalDateTime.now()
    val dia = hoy.dayOfWeek.getDisplayName(java.time.format.TextStyle.FULL, java.util.Locale.forLanguageTag("es"))
    return Regex("""\{([a-zA-Z0-9_]+)\}""").replace(t) { m ->
        when (val k = m.groupValues[1].lowercase()) {
            "nombre" -> "María"; "hora" -> hoy.format(DateTimeFormatter.ofPattern("HH:mm")); "dia", "día" -> dia
            else -> if (k.startsWith("dato")) "ejemplo" else ""
        }
    }
}

/** Corre el flujo en el papel: los pasos en cadena y se detiene en un menú o en una pregunta. */
private fun avanzaPrev(d: AutoDef, desde: String, bur: MutableList<BurbujaPrev>): Pair<String?, String> {
    var id = desde
    var guarda = 0
    while (guarda++ < 25) {
        val p = d.paso(id) ?: return null to "fin"
        when (p.tipo) {
            "menu" -> {
                bur += BurbujaPrev(true, sustituyeEjemplo(p.titulo) + p.opciones.mapIndexed { i, o -> "\n${i + 1}. ${o.texto}" }.joinToString("") + if (p.id != d.inicio) "\n0. Volver al menú principal" else "")
                return p.id to "menu"
            }
            "pedir" -> { bur += BurbujaPrev(true, sustituyeEjemplo(p.pregunta)); return p.id to "pedir" }
            "respuesta" -> { bur += BurbujaPrev(true, sustituyeEjemplo(p.texto)); if (p.siguiente.isEmpty()) return null to "fin"; id = p.siguiente }
            "humano" -> { if (p.texto.isNotBlank()) bur += BurbujaPrev(true, sustituyeEjemplo(p.texto)); bur += BurbujaPrev(true, "— Aquí el bot calla y te avisa —"); return null to "humano" }
            else -> { if (p.texto.isNotBlank()) bur += BurbujaPrev(true, sustituyeEjemplo(p.texto)); return null to "fin" }
        }
    }
    return null to "fin"
}

@Composable
private fun PestanaVistaPrevia(d: AutoDef) {
    val burbujas = remember(d) { mutableStateListOf<BurbujaPrev>() }
    var actual by remember(d) { mutableStateOf<String?>(null) }
    var tipo by remember(d) { mutableStateOf("") }
    var iniciado by remember(d) { mutableStateOf(false) }
    LaunchedEffect(d) {
        burbujas.clear()
        if (d.paso(d.inicio) != null) { val (a, t) = avanzaPrev(d, d.inicio, burbujas); actual = a; tipo = t; iniciado = true }
    }
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(Esp.l).testTag("pestana_previa"), verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Text("Así se vería la conversación. Toca una opción para seguirla. Los datos de ejemplo no se guardan.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        if (iniciado && burbujas.isEmpty()) Text("Esta automatización no tiene un paso inicial.")
        burbujas.forEach { BurbujaDeTexto(it.bot, it.texto) }
        val p = actual?.let { d.paso(it) }
        if (p != null && p.tipo == "menu") {
            Row(horizontalArrangement = Arrangement.spacedBy(6.dp), modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState())) {
                p.opciones.forEachIndexed { i, o ->
                    OutlinedButton({
                        burbujas += BurbujaPrev(false, "${i + 1}")
                        if (d.paso(o.destino) == null) { burbujas += BurbujaPrev(true, "(Esta opción no lleva a ningún paso)"); return@OutlinedButton }
                        val (a, t) = avanzaPrev(d, o.destino, burbujas); actual = a; tipo = t
                    }, Modifier.testTag("previa_opcion_$i")) { Text("${i + 1}. ${o.texto}", maxLines = 1) }
                }
                if (p.id != d.inicio) OutlinedButton({ burbujas += BurbujaPrev(false, "0"); val (a, t) = avanzaPrev(d, d.inicio, burbujas); actual = a; tipo = t }, Modifier.testTag("previa_cero")) { Text("0. Menú principal") }
            }
        } else if (p != null && p.tipo == "pedir") {
            OutlinedButton({ burbujas += BurbujaPrev(false, "ejemplo"); val (a, t) = if (p.siguiente.isEmpty()) null to "fin" else avanzaPrev(d, p.siguiente, burbujas); actual = a; tipo = t }, Modifier.testTag("previa_responder")) { Text("Responder con un dato de ejemplo") }
        } else if (iniciado && tipo.isNotEmpty()) {
            Text(if (tipo == "humano") "La conversación pasa a una persona." else "Fin de la conversación.", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.tertiary)
            TextButton({ burbujas.clear(); val (a, t) = avanzaPrev(d, d.inicio, burbujas); actual = a; tipo = t }, Modifier.testTag("previa_reiniciar")) { Text("Volver a empezar") }
        }
        Spacer(Modifier.size(80.dp))
    }
}
