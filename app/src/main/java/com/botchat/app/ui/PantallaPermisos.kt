@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import com.botchat.app.ui.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LargeTopAppBar
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.botchat.app.datos.PoliticaGrupo
import com.botchat.app.datos.RangoDePersona
import com.botchat.app.datos.Repo
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

// ---------- Ajustes → Rangos y permisos ----------

/** Centro de rangos y permisos: rangos, solicitudes, qué hacer con desconocidos y cómo contestar en los grupos. */
@Composable
fun PantallaPermisos(botId: Long, alVolver: () -> Unit, alRangos: () -> Unit, alSolicitudes: () -> Unit, alGrupos: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if (evs.any { it == "rangos" || it == "solicitudes" || it == "permisos_ajustes" }) version++ } }
    val ajustes by produceState<Map<String, String>?>(null, version) { value = runCatching { Repo.ajustesPermisos(botId) }.getOrDefault(emptyMap()) }
    val rangos by produceState(0, version) { value = runCatching { Repo.rangos(botId).count { !it.esDueno } }.getOrDefault(0) }
    val pendientes by produceState(0, version) { value = runCatching { Repo.contarSolicitudes(botId) }.getOrDefault(0) }
    var textoPronto by remember { mutableStateOf<String?>(null) }
    val a = ajustes
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Rangos y permisos") }) }) { relleno ->
        if (a == null) { Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }; return@Scaffold }
        val politica = a["desconocidos"] ?: "menu"
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Text("Tú decides quién puede usar qué. El bot solo responde: nunca escribe primero.", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            TarjetaIr(Ico.Persona, "Rangos", if (rangos == 0) "Crea rangos como Ayudante o Cliente VIP" else plural(rangos.toLong(), "rango", "rangos") + " además del Dueño", alRangos, "permisos_rangos")
            TarjetaIr(Ico.Aviso, "Solicitudes", if (pendientes == 0) "Nadie espera tu decisión" else plural(pendientes.toLong(), "persona espera", "personas esperan") + " tu decisión", alSolicitudes, "permisos_solicitudes", insignia = pendientes)
            Card(Modifier.fillMaxWidth().testTag("tarjeta_desconocidos"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                    Text("Si escribe alguien que no conoces", style = MaterialTheme.typography.titleMedium)
                    Text("Conoces a quien está en tu agenda, tiene rango, ya aprobaste, o ya te había escrito antes de cambiar esta opción.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    val opciones = listOf(
                        "menu" to ("Responderle el menú" to "Como siempre: le contesta tu menú."),
                        "pronto" to ("Solo avisar que le contestas pronto" to "Le manda un mensaje corto, una sola vez, y espera tu decisión."),
                        "aprobar" to ("Esperar mi aprobación" to "No le contesta nada hasta que lo apruebes en Solicitudes."),
                        "ignorar" to ("Ignorarlo" to "No le contesta ni guarda su solicitud."),
                    )
                    opciones.forEach { (v, tt) ->
                        Row(
                            Modifier.fillMaxWidth().selectable(politica == v, role = Role.RadioButton, onClick = {
                                haptica(Toque.Suave)
                                scope.launch { runCatching { Repo.fijarAjustePermisos(botId, "desconocidos", v) }.onFailure { avisaToast(ctx, it.message, true) }; version++ }
                            }).testTag("desconocidos_$v"),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            RadioButton(politica == v, null)
                            Column(Modifier.padding(start = Esp.m, top = Esp.s, bottom = Esp.s)) { Text(tt.first, style = MaterialTheme.typography.bodyLarge); Text(tt.second, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                        }
                    }
                    if (politica == "pronto") {
                        val t = textoPronto ?: a["desconocidos_texto"].orEmpty()
                        OutlinedTextField(t, { textoPronto = it.take(300) }, Modifier.fillMaxWidth().testTag("texto_pronto"), label = { Text("Mensaje que se les manda") }, minLines = 2)
                        TextButton({
                            scope.launch { runCatching { Repo.fijarAjustePermisos(botId, "desconocidos_texto", t) }.onSuccess { avisaToast(ctx, "Guardado", false) }.onFailure { avisaToast(ctx, it.message, true) }; version++ }
                        }, Modifier.testTag("guardar_texto_pronto")) { Text("Guardar mensaje") }
                    }
                }
            }
            TarjetaIr(Ico.Chats, "Grupos", "A quién le contesta el bot en cada grupo", alGrupos, "permisos_grupos")
        }
    }
}

@Composable
private fun TarjetaIr(icono: androidx.compose.ui.graphics.vector.ImageVector, titulo: String, texto: String, alPulsar: () -> Unit, etiqueta: String, insignia: Int = 0) {
    Card(onClick = alPulsar, modifier = Modifier.fillMaxWidth().testTag(etiqueta), shape = MaterialTheme.shapes.large, colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Row(Modifier.padding(Esp.l), verticalAlignment = Alignment.CenterVertically) {
            Box(Modifier.size(48.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primaryContainer), Alignment.Center) {
                Icon(icono, null, Modifier.size(24.dp), tint = MaterialTheme.colorScheme.onPrimaryContainer)
            }
            Spacer(Modifier.width(Esp.l))
            Column(Modifier.weight(1f)) {
                Text(titulo, style = MaterialTheme.typography.titleMedium)
                Text(texto, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            if (insignia > 0) Text(insignia.toString(), Modifier.clip(CircleShape).background(MaterialTheme.colorScheme.error).padding(horizontal = 8.dp, vertical = 2.dp).testTag("insignia_$etiqueta"), color = MaterialTheme.colorScheme.onError, style = MaterialTheme.typography.labelMedium)
            Icon(Ico.Siguiente, null, Modifier.size(20.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

// ---------- políticas de grupos ----------

internal fun nombrePoliticaGrupo(modo: String) = when (modo) { "admins" -> "Solo administradores del grupo"; "lista" -> "Solo una lista de personas"; else -> "Todos" }

/** A quién le contesta el bot en cada grupo. Aprobar a alguien en privado no lo autoriza aquí: cada grupo se decide aparte. */
@Composable
fun PantallaPoliticasGrupo(botId: Long, alVolver: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if (evs.any { it == "politicas" || it == "permisos_ajustes" || it == "chats" }) version++ } }
    val grupos by produceState<List<com.botchat.app.datos.ChatItem>?>(null, version) { value = runCatching { Repo.chats(botId, true, "", 400) }.getOrDefault(emptyList()) }
    val guardadas by produceState<Map<String, PoliticaGrupo>>(emptyMap(), version) { value = runCatching { Repo.politicasGrupo(botId).associateBy { it.chat } }.getOrDefault(emptyMap()) }
    val general by produceState("todos", version) { value = runCatching { Repo.ajustesPermisos(botId)["grupo_modo_defecto"] }.getOrNull() ?: "todos" }
    var editando by remember { mutableStateOf<com.botchat.app.datos.ChatItem?>(null) }
    val haptica = rememberHaptica()
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Grupos") }) }) { relleno ->
        LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("lista_politicas"), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            item {
                Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text("En los grupos el bot solo responde si lo mencionan, citan uno de sus mensajes o escriben un comando. Aquí eliges a quién le hace caso.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Text("Para los grupos nuevos", style = MaterialTheme.typography.titleSmall)
                    FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                        listOf("todos" to "Todos", "admins" to "Solo administradores").forEach { (m, t) ->
                            FilterChip(general == m, { haptica(Toque.Suave); scope.launch { runCatching { Repo.fijarAjustePermisos(botId, "grupo_modo_defecto", m) }.onFailure { avisaToast(ctx, it.message, true) }; version++ } }, { Text(t, maxLines = 1) }, modifier = Modifier.testTag("general_$m"))
                        }
                    }
                    Text("Cada grupo", style = MaterialTheme.typography.titleSmall, modifier = Modifier.padding(top = Esp.s))
                }
            }
            val datos = grupos
            if (datos == null) item { Box(Modifier.fillMaxWidth().padding(Esp.xl), Alignment.Center) { CircularProgressIndicator() } }
            else if (datos.isEmpty()) item { EstadoVacio(Ico.Chats, "Sin grupos", "Cuando el bot esté en un grupo, aparecerá aquí.", Modifier.padding(top = Esp.xl)) }
            else items(datos, key = { it.jid }) { g ->
                val p = guardadas[g.jid]
                Card(onClick = { editando = g }, modifier = Modifier.fillMaxWidth().testTag("politica_${g.jid}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                    Row(Modifier.padding(Esp.l), verticalAlignment = Alignment.CenterVertically) {
                        Avatar(g.nombre, g.foto, g.fotoTs, g.jid, 44.dp)
                        Column(Modifier.padding(start = Esp.m).weight(1f)) {
                            Text(g.nombre, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            Text(nombrePoliticaGrupo(p?.modo ?: general) + if (p?.modo == "lista") " (${p.lista.size})" else "", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                        Icon(Ico.Siguiente, null, Modifier.size(20.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }
            }
        }
    }
    editando?.let { g -> DialogoPoliticaGrupo(botId, g.jid, g.nombre) { editando = null; version++ } }
}

/** Cambiar a quién le contesta el bot en un grupo: todos, solo administradores o solo una lista. */
@Composable
fun DialogoPoliticaGrupo(botId: Long, chat: String, nombre: String, alCerrar: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val actual by produceState<PoliticaGrupo?>(null, chat) { value = runCatching { Repo.politicaGrupo(botId, chat) }.getOrNull() ?: PoliticaGrupo(chat, "todos", emptyList()) }
    val participantes by produceState<List<com.botchat.app.datos.Participante>?>(null, chat) { value = runCatching { Repo.participantes(botId, chat) }.getOrDefault(emptyList()) }
    var modo by remember { mutableStateOf<String?>(null) }
    val lista = remember { mutableStateListOf<String>() }
    var cargada by remember { mutableStateOf(false) }
    LaunchedEffect(actual) { actual?.let { if (!cargada) { modo = it.modo; lista.addAll(it.lista); cargada = true } } }
    AlertDialog(
        onDismissRequest = alCerrar,
        title = { Text(nombre, maxLines = 1, overflow = TextOverflow.Ellipsis) },
        text = {
            Column(Modifier.verticalScroll(rememberScrollState())) {
                Text("El bot le contesta a:", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                listOf("todos" to "Todos", "admins" to "Solo administradores del grupo", "lista" to "Solo una lista de personas").forEach { (v, t) ->
                    Row(Modifier.fillMaxWidth().selectable(modo == v, role = Role.RadioButton, onClick = { modo = v }).testTag("modo_grupo_$v"), verticalAlignment = Alignment.CenterVertically) {
                        RadioButton(modo == v, null); Text(t, Modifier.padding(start = Esp.m, top = Esp.s, bottom = Esp.s))
                    }
                }
                if (modo == "lista") {
                    val ps = participantes
                    if (ps == null) CircularProgressIndicator()
                    else if (ps.isEmpty()) Text("No se pudo leer la lista de personas del grupo. Conéctate e inténtalo otra vez.", style = MaterialTheme.typography.bodySmall)
                    else Column(Modifier.height(220.dp).verticalScroll(rememberScrollState()).testTag("lista_personas_grupo")) {
                        ps.forEach { p ->
                            Row(Modifier.fillMaxWidth().clickable { if (p.jid in lista) lista.remove(p.jid) else lista.add(p.jid) }, verticalAlignment = Alignment.CenterVertically) {
                                Checkbox(p.jid in lista, { if (it) lista.add(p.jid) else lista.remove(p.jid) }, Modifier.testTag("persona_${p.jid}"))
                                Text(p.nombre.ifBlank { p.telefono }, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            }
                        }
                    }
                }
                Text("Aprobar a alguien en privado no cambia esto: cada grupo se decide aparte.", Modifier.padding(top = Esp.s), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        },
        confirmButton = {
            TextButton({
                val m = modo ?: return@TextButton
                scope.launch { runCatching { Repo.fijarPoliticaGrupo(botId, PoliticaGrupo(chat, m, if (m == "lista") lista.toList() else emptyList())) }.onFailure { avisaToast(ctx, it.message, true) }; alCerrar() }
            }, enabled = modo != null, modifier = Modifier.testTag("guardar_politica_grupo")) { Text("Guardar") }
        },
        dismissButton = { TextButton(alCerrar) { Text("Cancelar") } },
    )
}

/** En la ficha de un grupo: a quién le contesta el bot ahí. */
@Composable
fun TarjetaPoliticaGrupo(botId: Long, chat: String, nombre: String) {
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("politicas" in evs || "permisos_ajustes" in evs) version++ } }
    val p by produceState<PoliticaGrupo?>(null, chat, version) { value = runCatching { Repo.politicaGrupo(botId, chat) }.getOrNull() }
    var abierto by remember { mutableStateOf(false) }
    Card(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.xs).testTag("tarjeta_politica_grupo"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Row(Modifier.padding(horizontal = Esp.l, vertical = Esp.s), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text("El bot le contesta a", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                Text(p?.let { nombrePoliticaGrupo(it.modo) } ?: "…", style = MaterialTheme.typography.bodyLarge, modifier = Modifier.testTag("politica_actual"))
            }
            OutlinedButton({ abierto = true }, Modifier.testTag("cambiar_politica_grupo")) { Text("Cambiar") }
        }
    }
    if (abierto) DialogoPoliticaGrupo(botId, chat, nombre) { abierto = false; version++ }
}

// ---------- rango en la ficha de una persona ----------

internal fun textoVence(caducaEn: Long, ahora: Long = System.currentTimeMillis() / 1000): String {
    if (caducaEn <= 0) return "Para siempre"
    val resta = caducaEn - ahora
    return when {
        resta > 48 * 3600 -> "Vence en ${resta / 86400} días"
        resta > 3600 -> "Vence en ${resta / 3600} horas"
        resta > 0 -> "Vence en menos de una hora"
        else -> "Vencido"
    }
}

/** En la ficha de una persona: su rango, con cambio en un toque (24 horas, 7 días o permanente) y Revocar. */
@Composable
fun TarjetaRango(botId: Long, persona: String, nombre: String) {
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if (evs.any { it == "rangos" || it == "rango:$persona" || it == "solicitudes" }) version++ } }
    val actual by produceState<RangoDePersona?>(null, persona, version) { value = runCatching { Repo.rangoDe(botId, persona) }.getOrNull() }
    var abierto by remember { mutableStateOf(false) }
    val a = actual
    Card(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.xs).testTag("tarjeta_rango"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Row(Modifier.padding(horizontal = Esp.l, vertical = Esp.s), verticalAlignment = Alignment.CenterVertically) {
            if (a?.rango != null) { PuntoRango(a.rango.color, 16); Spacer(Modifier.width(Esp.m)) }
            Column(Modifier.weight(1f)) {
                Text("Rango", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                Text(
                    when { a == null -> "…"; a.bloqueada -> "Bloqueada en el bot"; a.rango == null -> "Sin rango"; else -> a.rango.nombre },
                    style = MaterialTheme.typography.bodyLarge, fontWeight = FontWeight.Medium, modifier = Modifier.testTag("rango_actual"),
                )
                if (a?.rango != null && a.asignacion != null && !a.rango.esDueno) Text(textoVence(a.asignacion.caducaEn), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.testTag("rango_vence"))
            }
            OutlinedButton({ abierto = true }, Modifier.testTag("cambiar_rango"), enabled = a != null) { Text("Cambiar") }
        }
    }
    if (abierto && a != null) DialogoCambiarRango(botId, persona, nombre, a) { abierto = false; version++ }
}

/** Tocar un rango lo asigna con la duración elegida arriba; “Quitar rango” revoca. */
@Composable
fun DialogoCambiarRango(botId: Long, persona: String, nombre: String, actual: RangoDePersona, alCerrar: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    val rangos by produceState<List<com.botchat.app.datos.Rango>?>(null) { value = runCatching { Repo.rangos(botId).filter { !it.esDueno } }.getOrDefault(emptyList()) }
    var duracion by remember { mutableStateOf("perm") }
    fun hacer(f: suspend () -> Unit) = scope.launch { runCatching { f() }.onSuccess { alCerrar() }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) } }
    AlertDialog(
        onDismissRequest = alCerrar,
        title = { Text("Rango de $nombre", maxLines = 1, overflow = TextOverflow.Ellipsis) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("Cuánto dura", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    listOf("24h" to "24 horas", "7d" to "7 días", "perm" to "Permanente").forEach { (v, t) ->
                        FilterChip(duracion == v, { haptica(Toque.Suave); duracion = v }, { Text(t, maxLines = 1) }, modifier = Modifier.testTag("duracion_$v"))
                    }
                }
                Text("Toca un rango para dárselo", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                val l = rangos
                when {
                    l == null -> CircularProgressIndicator()
                    l.isEmpty() -> Text("Todavía no hay rangos. Crea uno en Ajustes → Rangos y permisos.", style = MaterialTheme.typography.bodyMedium)
                    else -> Column(Modifier.height((56 * l.size.coerceAtMost(5)).dp).verticalScroll(rememberScrollState())) {
                        l.forEach { r ->
                            Row(
                                Modifier.fillMaxWidth().clickable { haptica(Toque.Suave); hacer { Repo.asignarRango(botId, persona, r.id, duracion) } }.padding(vertical = Esp.m).testTag("elegir_rango_${r.id}"),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                PuntoRango(r.color, 16); Spacer(Modifier.width(Esp.m))
                                Text(r.nombre, Modifier.weight(1f), style = MaterialTheme.typography.bodyLarge)
                                if (actual.rango?.id == r.id) Icon(Ico.Listo, "Rango actual", Modifier.size(20.dp), tint = MaterialTheme.colorScheme.primary)
                            }
                        }
                    }
                }
            }
        },
        confirmButton = { TextButton(alCerrar) { Text("Cerrar") } },
        dismissButton = {
            if (actual.rango != null && !actual.rango.esDueno) TextButton({ haptica(Toque.Fuerte); hacer { Repo.revocarRango(botId, persona) } }, Modifier.testTag("revocar_rango")) { Text("Quitar rango", color = MaterialTheme.colorScheme.error) }
        },
    )
}

// ---------- confirmación con código ----------

private val COMANDOS_ETIQUETA = mapOf("asignar" to "dar un rango", "quitarrango" to "quitar un rango", "expulsar" to "expulsar a alguien", "borrar" to "borrar algo")

/** Diálogo del dueño: una acción espera el código de confirmación. Se puede aprobar aquí o escribir “!confirmar CÓDIGO”. */
@Composable
fun DialogoConfirmacion(botId: Long) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var version by remember { mutableIntStateOf(0) }
    var ahora by remember { mutableLongStateOf(System.currentTimeMillis() / 1000) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("confirmaciones" in evs) version++ } }
    LaunchedEffect(Unit) { while (true) { delay(15_000); ahora = System.currentTimeMillis() / 1000 } }
    val pendientes by produceState(emptyList<com.botchat.app.datos.ConfirmacionItem>(), version) { value = runCatching { Repo.confirmaciones(botId) }.getOrDefault(emptyList()) }
    val c = pendientes.firstOrNull { it.expiraTs > ahora } ?: return
    val quien by produceState("", c.persona) { value = if (c.persona == "yo") "Tú" else runCatching { Repo.chat(botId, c.persona)?.nombre }.getOrNull().orEmpty().ifBlank { numeroDePersona(c.persona) } }
    val minutos = ((c.expiraTs - ahora + 59) / 60).coerceAtLeast(1)
    AlertDialog(
        onDismissRequest = { },
        title = { Text("Confirmación pendiente") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("$quien pidió " + (COMANDOS_ETIQUETA[c.comando] ?: "la acción “${c.comando}”") + ".", Modifier.testTag("confirm_quien"))
                Text(c.codigo, Modifier.fillMaxWidth().testTag("codigo_confirmacion"), textAlign = TextAlign.Center, fontFamily = FontFamily.Monospace, fontSize = 32.sp, fontWeight = FontWeight.Bold)
                Text("Para confirmar escribe !confirmar ${c.codigo} en el chat, o toca Aprobar. Vence en ${plural(minutos, "minuto", "minutos")}.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        },
        confirmButton = {
            TextButton({
                scope.launch { runCatching { Repo.confirmarCodigo(botId, c.codigo) }.onSuccess { if (it.isNotBlank()) avisaToast(ctx, it, false) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }; version++ }
            }, Modifier.testTag("aprobar_confirmacion")) { Text("Aprobar") }
        },
        dismissButton = {
            TextButton({ scope.launch { runCatching { Repo.cancelarConfirmacion(botId, c.codigo) }; version++ } }, Modifier.testTag("cancelar_confirmacion")) { Text("Cancelar") }
        },
    )
}

// ---------- Avanzado → permisos y comandos ----------

/** Lo técnico de los rangos: prefijo de comandos, topes, tiempos, probar un comando y el registro de acciones. */
@Composable
fun PantallaPermisosAvanzado(botId: Long, alVolver: () -> Unit, alRegistro: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var version by remember { mutableIntStateOf(0) }
    val ajustes by produceState<Map<String, String>?>(null, version) { value = runCatching { Repo.ajustesPermisos(botId) }.getOrDefault(emptyMap()) }
    val a = ajustes
    var comando by remember { mutableStateOf("!ayuda") }
    var respuesta by remember { mutableStateOf("") }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Permisos y comandos") }) }) { relleno ->
        if (a == null) { Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }; return@Scaffold }
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(bottom = 32.dp)) {
            Seccion("Comandos") {
                CampoAjuste("Prefijo de los comandos", "prefijo", a, "Con “!” escribes !ayuda. Puede ser un símbolo de 1 a 3 caracteres.", botId, scope) { version++ }
            }
            Seccion("Desconocidos y topes") {
                CampoAjuste("Máximo de solicitudes pendientes", "tope_pendientes", a, "Al llegar al tope, los nuevos desconocidos se ignoran sin guardarse.", botId, scope, numerico = true) { version++ }
                CampoAjuste("Días que una solicitud ignorada se mantiene ignorada", "ignorada_dias", a, "Pasado este tiempo, si vuelve a escribir regresa a Pendientes.", botId, scope, numerico = true) { version++ }
                CampoAjuste("Comandos por minuto de quien no tiene rango", "limite_sin_rango", a, "0 = sin límite.", botId, scope, numerico = true) { version++ }
            }
            Seccion("Advertencias") {
                CampoAjuste("Advertencias para que actúe la regla", "adv_tope", a, "Al llegar a este número se hace lo de abajo. 0 = nunca. Siempre te llega un aviso.", botId, scope, numerico = true) { version++ }
                Column(Modifier.padding(horizontal = Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                    Text("Qué hace al llegar al tope", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                        listOf("avisar" to "Solo avisarme", "silenciar" to "Ignorar: que el bot deje de contestarle", "borrar" to "Borrar su mensaje", "expulsar" to "Expulsarla del grupo").forEach { (v, t) ->
                            FilterChip((a["adv_accion"] ?: "avisar") == v, {
                                scope.launch { runCatching { Repo.fijarAjustePermisos(botId, "adv_accion", v) }.onFailure { avisaToast(ctx, it.message, true) }; version++ }
                            }, { Text(t, maxLines = 1) }, modifier = Modifier.testTag("adv_accion_$v"))
                        }
                    }
                    Text("Borrar y expulsar solo funcionan en grupos donde el bot es administrador; si no lo es, solo te avisa.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                if ((a["adv_accion"] ?: "avisar") == "silenciar") CampoAjuste("Minutos que dura el silencio", "adv_silencio_min", a, "1440 = un día.", botId, scope, numerico = true) { version++ }
            }
            Seccion("Reportes (!reportar)") {
                Interruptor("Contestar “Gracias, ya avisé”", "Apagado: quien reporta no recibe respuesta en el chat; solo te llega la notificación.", a["reportar_gracias"] == "1", true, Ico.Aviso) {
                    scope.launch { runCatching { Repo.fijarAjustePermisos(botId, "reportar_gracias", if (it) "1" else "0") }.onFailure { e -> avisaToast(ctx, e.message, true) }; version++ }
                }
                if (a["reportar_gracias"] == "1") CampoAjuste("Texto de la respuesta", "reportar_texto", a, "Máximo 200 letras.", botId, scope) { version++ }
            }
            Seccion("Confirmaciones") {
                CampoAjuste("Segundos que dura un código", "confirm_seg", a, "Entre 15 y 900. Cada código sirve una sola vez.", botId, scope, numerico = true) { version++ }
            }
            Seccion("Registro de acciones") {
                CampoAjuste("Días que se guarda el registro", "registro_dias", a, "Solo guarda quién hizo qué y con qué resultado; nunca textos de mensajes.", botId, scope, numerico = true) { version++ }
                OutlinedButton(alRegistro, Modifier.fillMaxWidth().padding(horizontal = Esp.l).testTag("abrir_registro_acciones")) { Text("Ver el registro de acciones") }
            }
            Seccion("Probar un comando (como Dueño)") {
                Column(Modifier.padding(horizontal = Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    OutlinedTextField(comando, { comando = it }, Modifier.fillMaxWidth().testTag("campo_comando"), label = { Text("Comando") }, singleLine = true)
                    Button({
                        scope.launch { runCatching { Repo.ejecutarComando(botId, comando) }.onSuccess { respuesta = if (it.manejado) it.texto.ifBlank { "(sin respuesta)" } else "Eso no es un comando." }.onFailure { respuesta = it.message ?: "No se pudo" } }
                    }, Modifier.testTag("ejecutar_comando")) { Text("Ejecutar") }
                    if (respuesta.isNotEmpty()) Text(respuesta, Modifier.testTag("respuesta_comando"), style = MaterialTheme.typography.bodyMedium)
                }
            }
        }
    }
}

@Composable
private fun CampoAjuste(titulo: String, clave: String, ajustes: Map<String, String>, nota: String, botId: Long, scope: kotlinx.coroutines.CoroutineScope, numerico: Boolean = false, alGuardar: () -> Unit) {
    val ctx = LocalContext.current
    var texto by remember(ajustes[clave]) { mutableStateOf(ajustes[clave].orEmpty()) }
    Column(Modifier.padding(horizontal = Esp.l, vertical = Esp.xs), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            OutlinedTextField(
                texto, { texto = it }, Modifier.weight(1f).testTag("ajuste_$clave"), label = { Text(titulo) }, singleLine = true,
                keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = if (numerico) KeyboardType.Number else KeyboardType.Text),
            )
            TextButton({
                scope.launch { runCatching { Repo.fijarAjustePermisos(botId, clave, texto) }.onSuccess { avisaToast(ctx, "Guardado", false) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo guardar", true) }; alGuardar() }
            }, Modifier.testTag("guardar_$clave"), enabled = texto != ajustes[clave]) { Text("Guardar") }
        }
        Text(nota, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

/** Registro de acciones: quién, qué, cuándo y con qué resultado. No guarda textos de mensajes. */
@Composable
fun PantallaRegistroAcciones(botId: Long, alVolver: () -> Unit) {
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("acciones" in evs) version++ } }
    val lista by produceState<List<com.botchat.app.datos.AccionItem>?>(null, version) { value = runCatching { Repo.acciones(botId, 200) }.getOrDefault(emptyList()) }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Registro de acciones") }) }) { relleno ->
        val datos = lista
        when {
            datos == null -> Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }
            datos.isEmpty() -> EstadoVacio(Ico.Estadisticas, "Sin acciones todavía", "Aquí verás quién usó un comando o cambió un rango.", Modifier.padding(relleno).padding(top = 60.dp))
            else -> LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("lista_acciones"), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                items(datos, key = { it.id }) { x ->
                    Column(Modifier.fillMaxWidth().testTag("accion_${x.id}")) {
                        Row { Text(x.accion, Modifier.weight(1f), style = MaterialTheme.typography.bodyLarge, fontWeight = FontWeight.Medium); Text(x.resultado, color = if (x.resultado == "ok") MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error) }
                        Text(
                            (if (x.quien == "yo") "Tú" else if (x.quien == "sistema") "Sistema" else numeroDePersona(x.quien)) + (if (x.objetivo.isNotEmpty()) " → " + numeroDePersona(x.objetivo) else "") + " · " + etiquetaDia(x.ts) + " " + horaDe(x.ts),
                            style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }
            }
        }
    }
}
