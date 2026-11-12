@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.PasoEscalera
import com.botchat.app.datos.ReglaGrupo
import com.botchat.app.datos.Repo
import com.botchat.app.datos.TipoAdvertencia
import com.botchat.app.datos.TipoConEscalera
import com.botchat.app.datos.ajustesSilencio
import com.botchat.app.datos.borrarReglaItem
import com.botchat.app.datos.borrarTipo
import com.botchat.app.datos.fijarAjusteSilencio
import com.botchat.app.datos.guardarReglaItem
import com.botchat.app.datos.guardarTipo
import com.botchat.app.datos.moverReglaItem
import com.botchat.app.datos.reglasGrupoLista
import com.botchat.app.datos.tiposConEscalera
import kotlinx.coroutines.launch

private val ACCIONES = listOf("avisar" to "Avisarle", "silenciar" to "Silenciarlo un rato", "silencio_largo" to "Silencio largo", "expulsar" to "Expulsarlo (te pregunto)")

private fun textoPaso(p: PasoEscalera) = when (p.accion) {
    "avisar" -> "avisarle"
    "silenciar" -> "silenciarlo ${if (p.minutos >= 60 && p.minutos % 60 == 0) "${p.minutos / 60} h" else "${p.minutos} min"}"
    "silencio_largo" -> "silencio largo"
    "expulsar" -> "expulsarlo (con tu permiso)"
    else -> p.accion
}

internal fun textoCaducidad(dias: Int, general: Int) = when {
    dias == -1 -> if (general == 0) "caduca: nunca (como la general)" else "caduca a los $general días (como la general)"
    dias == 0 -> "no caduca"
    else -> "caduca a los $dias días"
}

// ---------------------------------------------------------------- tipos

/** Bot → Advertencias → Tipos: Spam, Insultos, Enlaces, Otro… y lo que pasa con cada advertencia. */
@Composable
fun PantallaTiposAdvertencia(botId: Long, alVolver: () -> Unit, alEditar: (Long) -> Unit) {
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("adv_tipos" in evs) version++ } }
    val tipos by produceState<List<TipoConEscalera>?>(null, version) { value = runCatching { Repo.tiposConEscalera(botId) }.getOrDefault(emptyList()) }
    val general by produceState(30, version) { value = runCatching { Repo.ajustesSilencio(botId).entero("adv_caducidad_dias", 30) }.getOrDefault(30) }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver, Modifier.testTag("volver_tipos")) { Icon(Ico.Atras, "Volver") } }, title = { Text("Tipos de advertencia") }) }) { relleno ->
        LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("pantalla_tipos"), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            item {
                Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text("Cada tipo tiene su propia escalera: lo que pasa con la 1.ª, la 2.ª, la 3.ª advertencia…", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Button({ alEditar(0) }, Modifier.fillMaxWidth().testTag("nuevo_tipo")) { Icon(Ico.Agregar, null, Modifier.size(20.dp)); Spacer(Modifier.width(8.dp)); Text("Crear un tipo") }
                }
            }
            val datos = tipos
            when {
                datos == null -> item { FilasDeCarga(3) }
                datos.isEmpty() -> item { EstadoVacio(Ico.Aviso, "No hay tipos", "Crea uno, por ejemplo «Spam».", Modifier.testTag("sin_tipos")) }
                else -> items(datos.filter { it.tipo.activo }, key = { it.tipo.id }) { x ->
                    Card({ alEditar(x.tipo.id) }, Modifier.fillMaxWidth().testTag("tipo_${x.tipo.id}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                        Row(Modifier.padding(Esp.l), verticalAlignment = Alignment.CenterVertically) {
                            Box(Modifier.size(44.dp).clip(CircleShape).background(colorDeRango(x.tipo.color).copy(alpha = 0.25f)), Alignment.Center) { Text(x.tipo.emoji.ifBlank { "⚠️" }, style = MaterialTheme.typography.titleLarge) }
                            Spacer(Modifier.width(Esp.m))
                            Column(Modifier.weight(1f)) {
                                Text(x.tipo.nombre, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                Text(textoCaducidad(x.tipo.caducidadDias, general), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                Text(
                                    if (x.escalera.isEmpty()) "Sin escalera" else x.escalera.joinToString(" · ") { "${it.nivel}.ª ${textoPaso(it)}" },
                                    Modifier.testTag("escalera_resumen_${x.tipo.id}"), style = MaterialTheme.typography.bodySmall, maxLines = 3,
                                )
                            }
                            Icon(Ico.Siguiente, null, Modifier.size(20.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                }
            }
        }
    }
}

private val CADUCIDADES = listOf(OpcionValor(-1, "Como la general"), OpcionValor(0, "Nunca"), OpcionValor(7, "7 días"), OpcionValor(30, "30 días"), OpcionValor(90, "90 días"))

/** Crear (id 0) o cambiar un tipo: nombre, emoji, color, cuándo caduca y su escalera. */
@Composable
fun EditorTipoAdvertencia(botId: Long, id: Long, alVolver: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    val original by produceState<TipoConEscalera?>(null, id) {
        value = if (id == 0L) TipoConEscalera(TipoAdvertencia(0, "", "⚠️", COLORES_RANGO[1], 0, -1, 0, true), listOf(PasoEscalera(1, "avisar", 0, "grupo", "")))
        else runCatching { Repo.tiposConEscalera(botId).firstOrNull { it.tipo.id == id } }.getOrNull()
    }
    val o = original
    if (o == null) { Box(Modifier.fillMaxSize()) { FilasDeCarga(3) }; return }
    var nombre by remember(o) { mutableStateOf(o.tipo.nombre) }
    var emoji by remember(o) { mutableStateOf(o.tipo.emoji) }
    var color by remember(o) { mutableStateOf(o.tipo.color) }
    var caduca by remember(o) { mutableStateOf(o.tipo.caducidadDias) }
    val pasos = remember(o) { mutableStateListOf<PasoEscalera>().also { it.addAll(o.escalera) } }
    var editandoMin by remember { mutableStateOf<Int?>(null) }
    var editandoAviso by remember { mutableStateOf<Int?>(null) }
    var caducaOtro by remember { mutableStateOf(false) }
    var borrando by remember { mutableStateOf(false) }

    fun guardar() {
        scope.launch {
            runCatching { Repo.guardarTipo(botId, o.tipo.copy(nombre = nombre.trim(), emoji = emoji.trim(), color = color, caducidadDias = caduca), pasos.toList()) }
                .onSuccess { alVolver() }.onFailure { avisaToast(ctx, it.message ?: "No se pudo guardar", true) }
        }
    }

    Box(Modifier.fillMaxSize()) {
        Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver, Modifier.testTag("volver_editor_tipo")) { Icon(Ico.Atras, "Volver") } }, title = { Text(if (id == 0L) "Nuevo tipo" else "Cambiar tipo") }) }) { relleno ->
            Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l).testTag("editor_tipo"), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
                OutlinedTextField(nombre, { nombre = it.take(24) }, Modifier.fillMaxWidth().testTag("tipo_nombre"), label = { Text("Nombre") }, singleLine = true)
                OutlinedTextField(emoji, { emoji = it.take(4) }, Modifier.fillMaxWidth().testTag("tipo_emoji"), label = { Text("Emoji") }, singleLine = true)
                Text("Color", style = MaterialTheme.typography.titleSmall)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    COLORES_RANGO.forEach { c ->
                        Box(
                            Modifier.size(36.dp).clip(CircleShape).background(colorDeRango(c)).then(if (c == color) Modifier.border(3.dp, MaterialTheme.colorScheme.onSurface, CircleShape) else Modifier)
                                .clickable { color = c }.testTag("tipo_color_${c.removePrefix("#")}"),
                        )
                    }
                }
                Text("¿Cuándo deja de contar?", style = MaterialTheme.typography.titleSmall)
                SelectorValores(caduca.toLong(), CADUCIDADES, "tipo_caduca", { caduca = it.toInt() }, alOtro = { caducaOtro = true })

                Text("Escalera", style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.primary)
                Text("Lo que pasa con cada advertencia vigente de este tipo. Pasado el último paso se repite el último.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                pasos.forEachIndexed { i, p ->
                    Card(Modifier.fillMaxWidth().testTag("paso_${i + 1}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                        Column(Modifier.padding(Esp.m), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Text("Con la advertencia ${i + 1}", Modifier.weight(1f), style = MaterialTheme.typography.titleSmall)
                                if (pasos.size > 1) IconButton({ haptica(Toque.Suave); pasos.removeAt(i) }, Modifier.testTag("quitar_paso_${i + 1}")) { Icon(Ico.Borrar, "Quitar este paso") }
                            }
                            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                                ACCIONES.forEach { (k, t) ->
                                    FilterChip(p.accion == k, { haptica(Toque.Suave); pasos[i] = p.copy(accion = k, minutos = if (k == "silenciar" && p.minutos == 0) 60 else p.minutos) }, { Text(t) }, modifier = Modifier.testTag("paso_${i + 1}_$k"))
                                }
                            }
                            if (p.accion == "silenciar") OutlinedButton({ editandoMin = i }, Modifier.fillMaxWidth().testTag("paso_${i + 1}_minutos")) { Text("Tiempo: ${p.minutos} minutos") }
                            if (p.accion == "silenciar" || p.accion == "silencio_largo") FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                                FilterChip(p.alcance != "todos", { pasos[i] = p.copy(alcance = "grupo") }, { Text("Solo en este grupo") }, modifier = Modifier.testTag("paso_${i + 1}_grupo"))
                                FilterChip(p.alcance == "todos", { pasos[i] = p.copy(alcance = "todos") }, { Text("En todos mis grupos") }, modifier = Modifier.testTag("paso_${i + 1}_todos"))
                            }
                            if (p.accion == "expulsar") Text("Nunca se expulsa solo: te llega una notificación para que decidas.", style = MaterialTheme.typography.bodySmall, color = AMARILLO_SILENCIO)
                            OutlinedButton({ editandoAviso = i }, Modifier.fillMaxWidth().testTag("paso_${i + 1}_aviso")) { Text(if (p.aviso.isBlank()) "Aviso: el de siempre" else "Aviso: ${p.aviso}", maxLines = 1, overflow = TextOverflow.Ellipsis) }
                        }
                    }
                }
                OutlinedButton({ pasos.add(PasoEscalera(pasos.size + 1, "avisar", 0, "grupo", "")) }, Modifier.fillMaxWidth().testTag("agregar_paso")) { Icon(Ico.Agregar, null, Modifier.size(18.dp)); Spacer(Modifier.width(8.dp)); Text("Agregar un paso") }

                Button({ haptica(Toque.Confirmar); guardar() }, Modifier.fillMaxWidth().testTag("guardar_tipo"), enabled = nombre.isNotBlank() && pasos.isNotEmpty()) { Text("Guardar") }
                if (id != 0L) OutlinedButton({ borrando = true }, Modifier.fillMaxWidth().testTag("borrar_tipo"), colors = androidx.compose.material3.ButtonDefaults.outlinedButtonColors(contentColor = MaterialTheme.colorScheme.error)) { Text("Borrar este tipo") }
            }
        }
        editandoMin?.let { i ->
            HojaNumero("Cuántos minutos", "Minutos", pasos[i].minutos.toLong(), "paso_min", { editandoMin = null }, { n -> pasos[i] = pasos[i].copy(minutos = n.toInt().coerceAtLeast(1)); editandoMin = null })
        }
        editandoAviso?.let { i ->
            HojaTexto("Aviso para la persona", pasos[i].aviso, "paso_aviso", { editandoAviso = null }, { t -> pasos[i] = pasos[i].copy(aviso = t); editandoAviso = null }, ayuda = "Déjalo vacío para usar el aviso de siempre.", permiteVacio = true, botonRestaurar = "Usar el de siempre")
        }
        if (caducaOtro) HojaNumero("¿Cuántos días cuenta?", "Días", 0, "tipo_caduca_otro", { caducaOtro = false }, { n -> caducaOtro = false; caduca = n.toInt() }, ayuda = "0 = no caduca nunca.")
    }
    if (borrando) ConfirmarSinDeshacer("¿Borrar «${o.tipo.nombre}»?", "Sus advertencias se quedan, pero sin tipo.", "Borrar", "confirmar_borrar_tipo", { borrando = false }) {
        borrando = false
        scope.launch { runCatching { Repo.borrarTipo(botId, id) }.onSuccess { alVolver() }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) } }
    }
}

// ---------------------------------------------------------------- reglas del grupo

/**
 * La lista editable de reglas de un grupo (o las generales si `grupo` es ""). Es una pieza suelta para poder usarla también en el asistente:
 * el formulario va dentro de la misma lista, sin ventanas.
 */
@Composable
fun ListaReglasEditable(botId: Long, grupo: String, modifier: Modifier = Modifier) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    val reglas by produceState<List<ReglaGrupo>?>(null, grupo, version) { value = runCatching { Repo.reglasGrupoLista(botId, grupo) }.getOrDefault(emptyList()) }
    var editando by remember { mutableStateOf<ReglaGrupo?>(null) }
    var creando by remember { mutableStateOf(false) }
    var borrando by remember { mutableStateOf<ReglaGrupo?>(null) }
    fun hacer(f: suspend () -> Unit) = scope.launch { runCatching { f() }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }; version++ }

    Column(modifier.fillMaxWidth().testTag("lista_reglas"), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
        val datos = reglas
        if (datos == null) FilasDeCarga(2)
        else if (datos.isEmpty() && !creando) Text("Todavía no hay reglas. Agrega la primera.", Modifier.testTag("sin_reglas"), color = MaterialTheme.colorScheme.onSurfaceVariant)
        datos?.forEachIndexed { i, r ->
            Card(Modifier.fillMaxWidth().testTag("regla_${r.id}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                Row(Modifier.padding(Esp.m), verticalAlignment = Alignment.CenterVertically) {
                    Text("${i + 1}", Modifier.width(28.dp), style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.primary, fontWeight = FontWeight.Bold)
                    Column(Modifier.weight(1f)) {
                        Text(r.titulo, style = MaterialTheme.typography.titleSmall)
                        if (r.texto.isNotBlank()) Text(r.texto, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    IconButton({ haptica(Toque.Suave); hacer { Repo.moverReglaItem(botId, r.id, -1) } }, Modifier.testTag("regla_subir_${r.id}"), enabled = i > 0) { Text("▲") }
                    IconButton({ haptica(Toque.Suave); hacer { Repo.moverReglaItem(botId, r.id, 1) } }, Modifier.testTag("regla_bajar_${r.id}"), enabled = i < datos.size - 1) { Text("▼") }
                    IconButton({ editando = r; creando = false }, Modifier.testTag("regla_editar_${r.id}")) { Icon(Ico.Editar, "Cambiar", Modifier.size(20.dp)) }
                    IconButton({ borrando = r }, Modifier.testTag("regla_borrar_${r.id}")) { Icon(Ico.Borrar, "Borrar", Modifier.size(20.dp)) }
                }
            }
        }
        if (creando || editando != null) {
            val e = editando
            var titulo by remember(e) { mutableStateOf(e?.titulo.orEmpty()) }
            var texto by remember(e) { mutableStateOf(e?.texto.orEmpty()) }
            Card(Modifier.fillMaxWidth().testTag("formulario_regla"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerHigh)) {
                Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text(if (e == null) "Nueva regla" else "Cambiar regla", style = MaterialTheme.typography.titleSmall)
                    OutlinedTextField(titulo, { titulo = it.take(60) }, Modifier.fillMaxWidth().testTag("regla_titulo"), label = { Text("Título corto") }, singleLine = true)
                    OutlinedTextField(texto, { texto = it.take(400) }, Modifier.fillMaxWidth().testTag("regla_texto"), label = { Text("Explicación (opcional)") }, minLines = 2)
                    Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                        Button({
                            val pos = e?.pos ?: ((datos?.size ?: 0) + 1)
                            val id = e?.id ?: 0L
                            val t = titulo.trim(); val x = texto.trim()
                            creando = false; editando = null
                            hacer { Repo.guardarReglaItem(botId, grupo, id, t, x, pos) }
                        }, Modifier.testTag("guardar_regla"), enabled = titulo.isNotBlank()) { Text("Guardar") }
                        TextButton({ creando = false; editando = null }, Modifier.testTag("cancelar_regla")) { Text("Cancelar") }
                    }
                }
            }
        } else {
            Button({ creando = true }, Modifier.fillMaxWidth().testTag("agregar_regla")) { Icon(Ico.Agregar, null, Modifier.size(20.dp)); Spacer(Modifier.width(8.dp)); Text("Agregar una regla") }
        }
    }
    borrando?.let { r ->
        ConfirmarSinDeshacer("¿Borrar la regla «${r.titulo}»?", "Las advertencias que ya la citaban se quedan.", "Borrar", "confirmar_borrar_regla", { borrando = null }) {
            borrando = null; hacer { Repo.borrarReglaItem(botId, r.id) }
        }
    }
}

/** Pantalla completa con las reglas de un grupo (o las generales si `grupo` es ""). */
@Composable
fun PantallaReglasGrupo(botId: Long, grupo: String, titulo: String, alVolver: () -> Unit) {
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver, Modifier.testTag("volver_reglas")) { Icon(Ico.Atras, "Volver") } }, title = { Text(titulo, maxLines = 1, overflow = TextOverflow.Ellipsis) }) }) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Text("Son las reglas de la casa. Al advertir puedes elegir una y el aviso dirá «Regla 2: …». También salen con el comando !reglas.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            ListaReglasEditable(botId, grupo)
        }
    }
}

/** Elegir de qué grupo son las reglas: las generales o las de uno de tus grupos. */
@Composable
fun PantallaReglasGrupos(botId: Long, alVolver: () -> Unit, alAbrir: (grupo: String, nombre: String) -> Unit) {
    val grupos by produceState<List<com.botchat.app.datos.ChatItem>?>(null) { value = runCatching { Repo.chats(botId, true, "", 300) }.getOrDefault(emptyList()) }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver, Modifier.testTag("volver_reglas_grupos")) { Icon(Ico.Atras, "Volver") } }, title = { Text("Reglas de los grupos") }) }) { relleno ->
        LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("pantalla_reglas_grupos"), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            item { OutlinedButton({ alAbrir("", "Reglas generales") }, Modifier.fillMaxWidth().testTag("reglas_generales")) { Text("Reglas generales (para todos)") } }
            val datos = grupos
            if (datos == null) item { FilasDeCarga(3) }
            else items(datos, key = { it.jid }) { g ->
                OutlinedButton({ alAbrir(g.jid, g.nombre) }, Modifier.fillMaxWidth().testTag("reglas_de_${g.jid}")) { Text(g.nombre.ifBlank { "Grupo" }, Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis); Icon(Ico.Siguiente, null) }
            }
        }
    }
}

/** Las tarjetas con atajos y la caducidad general que se ven arriba en Advertencias. */
@Composable
fun CabeceraAdvertencias(botId: Long, alTipos: () -> Unit, alReglas: () -> Unit) {
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    val dias by produceState(30, version) { value = runCatching { Repo.ajustesSilencio(botId).entero("adv_caducidad_dias", 30) }.getOrDefault(30) }
    var otro by remember { mutableStateOf(false) }
    Column(Modifier.fillMaxWidth().testTag("cabecera_advertencias"), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
        Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
            BotonGrandeE7(Ico.Aviso, "Tipos y escalera", "adv_tipos", Modifier.weight(1f)) { haptica(Toque.Suave); alTipos() }
            BotonGrandeE7(Ico.Documento, "Reglas de los grupos", "adv_reglas", Modifier.weight(1f), MaterialTheme.colorScheme.secondaryContainer, MaterialTheme.colorScheme.onSecondaryContainer) { haptica(Toque.Suave); alReglas() }
        }
        Text("¿Cuándo deja de contar una advertencia?", style = MaterialTheme.typography.titleSmall)
        SelectorValores(dias.toLong(), listOf(OpcionValor(7, "7 días"), OpcionValor(30, "30 días"), OpcionValor(90, "90 días"), OpcionValor(0, "Nunca")), "caducidad_general", { v ->
            scope.launch { runCatching { Repo.fijarAjusteSilencio(botId, "adv_caducidad_dias", v.toString()) }; version++ }
        }, alOtro = { otro = true })
        Text("Es la de todos los tipos que no tengan la suya. Las que ya existen no cambian.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
    if (otro) HojaNumeroEnLinea("Días", dias.toLong(), { otro = false }) { n -> otro = false; scope.launch { runCatching { Repo.fijarAjusteSilencio(botId, "adv_caducidad_dias", n.toString()) }; version++ } }
}

/** Número en una tarjeta pequeña dentro de la lista (sin hoja): para pantallas que no tienen una caja a pantalla completa. */
@Composable
private fun HojaNumeroEnLinea(unidad: String, inicial: Long, alCerrar: () -> Unit, alAceptar: (Long) -> Unit) {
    var texto by remember { mutableStateOf(inicial.toString()) }
    Card(Modifier.fillMaxWidth().testTag("caducidad_otro"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerHigh)) {
        Row(Modifier.padding(Esp.m), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
            OutlinedTextField(texto, { texto = it.filter(Char::isDigit).take(4) }, Modifier.width(120.dp).testTag("caducidad_otro_campo"), singleLine = true, label = { Text(unidad) })
            Button({ alAceptar(texto.toLongOrNull() ?: 0L) }, Modifier.testTag("caducidad_otro_aceptar")) { Text("Listo") }
            TextButton(alCerrar) { Text("Cancelar") }
        }
    }
}
