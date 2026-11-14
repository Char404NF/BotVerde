@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import com.botchat.app.ui.AlertDialog
import androidx.compose.material3.Checkbox
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
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
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.EtiquetaItem
import com.botchat.app.datos.GrupoAdmin
import com.botchat.app.datos.GrupoComunItem
import com.botchat.app.datos.ReglaGrupo
import com.botchat.app.datos.Repo
import com.botchat.app.datos.ResultadoAdvertenciaTipo
import com.botchat.app.datos.TipoAdvertencia
import com.botchat.app.datos.advertirConTipo
import com.botchat.app.datos.marcarMensaje
import com.botchat.app.datos.marcasDeMensaje
import com.botchat.app.datos.ponerEtiquetaPersona
import com.botchat.app.datos.quitarMarcaMensaje
import com.botchat.app.datos.quitarUltimaAdvertencia
import com.botchat.app.datos.reglasGrupoLista
import com.botchat.app.datos.silenciarEnGrupos
import com.botchat.app.datos.textosSilencio
import com.botchat.app.datos.tiposAdvertencia
import kotlinx.coroutines.launch

// Los diálogos del perfil: Silenciar, Advertir y Marcar. Botones grandes, palabras de todos los días.

private const val AVISO_SILENCIO_DEFECTO = "Te silencié en {grupo} {duracion}. Tus mensajes ahí se borran."

// ---------- Silenciar en grupos ----------

private val DURACIONES = listOf("1h" to "1 hora", "1d" to "1 día", "1s" to "1 semana", "siempre" to "Siempre", "otro" to "Otro tiempo")

internal fun segundosDeDuracion(clave: String, cantidad: Long, unidad: String): Long = when (clave) {
    "1h" -> 3600L
    "1d" -> 86400L
    "1s" -> 7 * 86400L
    "siempre" -> 0L
    else -> cantidad * when (unidad) { "min" -> 60L; "h" -> 3600L; else -> 86400L }
}

/**
 * Silenciar a una persona en grupos: cuánto tiempo, dónde (un grupo, varios o todos los que administro) y el aviso que se le manda.
 * Los grupos donde no soy administrador se ven, pero se dice claro que ahí no podré borrar sus mensajes.
 */
@Composable
fun DialogoSilenciarGrupos(botId: Long, persona: String, nombre: String, grupoInicial: String = "", alCerrar: () -> Unit, alListo: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val grupos by produceState<List<GrupoAdmin>?>(null) { value = runCatching { Repo.gruposAdmin(botId) }.getOrDefault(emptyList()) }
    val textoDefecto by produceState(AVISO_SILENCIO_DEFECTO) { value = runCatching { Repo.textosSilencio(botId)["inicio"]?.first?.ifBlank { null } }.getOrNull() ?: AVISO_SILENCIO_DEFECTO }
    val porDefecto = remember { com.botchat.app.datos.Ajustes.get(ctx) }
    var duracion by remember { mutableStateOf(when (porDefecto.silencioTiempoDefecto.value) { 60 -> "1h"; 10080 -> "1s"; 0 -> "siempre"; else -> "1d" }) }
    var cantidad by remember { mutableStateOf("2") }
    var unidad by remember { mutableStateOf("h") }
    var alcance by remember { mutableStateOf(if (grupoInicial.isNotEmpty()) "uno" else porDefecto.silencioAlcanceDefecto.value) }
    val elegidos = remember { mutableStateListOf<String>().also { if (grupoInicial.isNotEmpty()) it.add(grupoInicial) } }
    var avisar by remember { mutableStateOf(true) }
    var texto by remember { mutableStateOf<String?>(null) }
    var trabajando by remember { mutableStateOf(false) }
    val lista = grupos ?: emptyList()
    val soloAdmin = lista.filter { it.soyAdmin }
    val numero = cantidad.toLongOrNull() ?: 0L
    val seg = segundosDeDuracion(duracion, numero, unidad)
    val valido = (duracion != "otro" || numero > 0) && when (alcance) {
        "todos" -> soloAdmin.isNotEmpty()
        else -> elegidos.isNotEmpty()
    }
    HojaAlerta(
        onDismissRequest = { if (!trabajando) alCerrar() }, title = { Text("Silenciar a $nombre") },
        text = {
            // Los campos de texto van FUERA de la parte que se desplaza (un campo dentro de un desplazamiento no deja reposar a Compose).
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("Sus mensajes en los grupos se borran mientras dure el silencio.", style = MaterialTheme.typography.bodyMedium)
                Text("¿Por cuánto tiempo?", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                    DURACIONES.forEach { (k, t) -> FilterChip(duracion == k, { duracion = k }, { Text(t, maxLines = 1) }, modifier = Modifier.testTag("sil_dur_$k")) }
                }
                if (duracion == "otro") Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    OutlinedTextField(cantidad, { cantidad = it.filter(Char::isDigit).take(4) }, Modifier.width(96.dp).testTag("sil_cantidad"), singleLine = true, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number))
                    listOf("min" to "minutos", "h" to "horas", "d" to "días").forEach { (k, t) -> FilterChip(unidad == k, { unidad = k }, { Text(t, maxLines = 1) }, modifier = Modifier.testTag("sil_unidad_$k")) }
                }
                Text("¿Dónde?", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                    listOf("uno" to "Un grupo", "varios" to "Varios grupos", "todos" to "Todos los que administro").forEach { (k, t) ->
                        FilterChip(alcance == k, { alcance = k; if (k == "uno" && elegidos.size > 1) { val p = elegidos.first(); elegidos.clear(); elegidos.add(p) } }, { Text(t, maxLines = 1) }, modifier = Modifier.testTag("sil_alcance_$k"))
                    }
                }
                Column(Modifier.heightIn(max = 200.dp).verticalScroll(rememberScrollState())) {
                    when {
                        grupos == null -> Text("Buscando tus grupos…", style = MaterialTheme.typography.bodySmall)
                        alcance == "todos" -> Text(
                            if (soloAdmin.isEmpty()) "No administro ningún grupo (o aún no lo sé: enciende el bot)." else "Se silencia en ${plural(soloAdmin.size.toLong(), "grupo", "grupos")}: " + soloAdmin.joinToString(", ") { it.nombre.ifBlank { "Grupo" } },
                            Modifier.testTag("sil_todos_texto"), style = MaterialTheme.typography.bodyMedium,
                        )
                        lista.isEmpty() -> Text("Aún no conozco tus grupos. Enciende el bot y vuelve a intentar.", Modifier.testTag("sil_sin_grupos"), style = MaterialTheme.typography.bodyMedium)
                        else -> Column(verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                            lista.forEach { g ->
                                Row(Modifier.fillMaxWidth().testTag("sil_grupo_${g.jid}"), verticalAlignment = Alignment.CenterVertically) {
                                    Checkbox(g.jid in elegidos, { si ->
                                        if (si) { if (alcance == "uno") elegidos.clear(); elegidos.add(g.jid) } else elegidos.remove(g.jid)
                                    }, Modifier.testTag("sil_check_${g.jid}"))
                                    Column(Modifier.weight(1f)) {
                                        Text(g.nombre.ifBlank { "Grupo" }, style = MaterialTheme.typography.bodyLarge, maxLines = 1)
                                        if (!g.soyAdmin) Text(
                                            if (g.sabido) "No soy admin aquí: no podré borrar sus mensajes" else "No sé si soy admin aquí (enciende el bot)",
                                            Modifier.testTag("sil_noadmin_${g.jid}"), style = MaterialTheme.typography.bodySmall, color = ROJO_SILENCIO,
                                        )
                                    }
                                }
                            }
                        }
                    }
                }
                Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text("Avisarle por privado", style = MaterialTheme.typography.bodyLarge)
                        Text("Le llega un mensaje que dice por qué.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    Switch(avisar, { avisar = it }, Modifier.testTag("sil_avisar"))
                }
                if (avisar) OutlinedTextField(
                    texto ?: textoDefecto, { texto = it }, Modifier.fillMaxWidth().testTag("sil_texto"), label = { Text("Lo que le digo") }, minLines = 2, maxLines = 4,
                )
            }
        },
        confirmButton = {
            TextButton({
                trabajando = true
                val grupo = if (alcance == "todos") emptyList() else elegidos.toList()
                val propio = texto?.trim()?.takeIf { it.isNotEmpty() && it != textoDefecto.trim() } ?: ""
                scope.launch {
                    runCatching { Repo.silenciarEnGrupos(botId, persona, seg, alcance, grupo, avisar, propio) }
                        .onSuccess { avisaToast(ctx, "Listo: $nombre quedó silenciada", false); alListo(); alCerrar() }
                        .onFailure { avisaToast(ctx, it.message ?: "No se pudo silenciar", true); trabajando = false }
                }
            }, Modifier.testTag("sil_confirmar"), enabled = valido && !trabajando) { Text("Silenciar") }
        },
        dismissButton = { TextButton({ alCerrar() }, Modifier.testTag("sil_cancelar"), enabled = !trabajando) { Text("Cancelar") } },
    )
}

// ---------- Advertir con tipo ----------

/**
 * Advertir con un tipo (Spam, Insultos…): botones grandes, y si es en un grupo, una regla opcional. Al terminar dice qué pasó y deja
 * quitar la advertencia que se acaba de poner.
 */
@Composable
fun DialogoAdvertirTipo(botId: Long, persona: String, nombre: String, gruposComun: List<GrupoComunItem>, chatInicial: String = "", alCerrar: () -> Unit, alListo: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val tipos by produceState<List<TipoAdvertencia>?>(null) { value = runCatching { Repo.tiposAdvertencia(botId) }.getOrDefault(emptyList()) }
    var chat by remember { mutableStateOf(chatInicial) }
    val reglas by produceState<List<ReglaGrupo>>(emptyList(), chat) {
        value = if (chat.isEmpty()) emptyList() else runCatching { Repo.reglasGrupoLista(botId, chat) + Repo.reglasGrupoLista(botId, "") }.getOrDefault(emptyList())
    }
    var tipo by remember { mutableStateOf<TipoAdvertencia?>(null) }
    var regla by remember { mutableStateOf(0L) }
    var motivo by remember { mutableStateOf("") }
    var trabajando by remember { mutableStateOf(false) }
    var resultado by remember { mutableStateOf<ResultadoAdvertenciaTipo?>(null) }
    val r = resultado
    if (r != null) {
        AlertDialog(
            onDismissRequest = { alListo(); alCerrar() }, title = { Text("Advertencia puesta") },
            text = {
                Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text("$nombre lleva ${plural(maxOf(r.nivel, 1).toLong(), "advertencia", "advertencias")} de este tipo.", Modifier.testTag("adv_resultado"))
                    Text(textoPasoEscalera(r), Modifier.testTag("adv_paso"), color = if (r.hecho == "solo_aviso") ROJO_SILENCIO else MaterialTheme.colorScheme.onSurface)
                    if (r.regla.isNotBlank()) Text(r.regla, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            },
            confirmButton = { TextButton({ alListo(); alCerrar() }, Modifier.testTag("adv_listo")) { Text("Listo") } },
            dismissButton = {
                TextButton({
                    scope.launch {
                        runCatching { Repo.quitarUltimaAdvertencia(botId, persona) }
                            .onSuccess { avisaToast(ctx, "Quité la advertencia", false) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
                        alListo(); alCerrar()
                    }
                }, Modifier.testTag("adv_deshacer")) { Text("Deshacer") }
            },
        )
        return
    }
    HojaAlerta(
        onDismissRequest = { if (!trabajando) alCerrar() }, title = { Text("Advertir a $nombre") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
              Column(Modifier.heightIn(max = 340.dp).verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("¿Qué hizo?", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary)
                val ts = tipos
                when {
                    ts == null -> Text("Cargando…", style = MaterialTheme.typography.bodySmall)
                    ts.isEmpty() -> Text("Aún no hay tipos de advertencia.", Modifier.testTag("adv_sin_tipos"))
                    else -> ts.chunked(2).forEach { fila ->
                        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                            fila.forEach { t ->
                                val elegido = tipo?.id == t.id
                                androidx.compose.material3.Card(
                                    onClick = { tipo = t }, modifier = Modifier.weight(1f).heightIn(min = 64.dp).testTag("adv_tipo_${t.id}"),
                                    colors = androidx.compose.material3.CardDefaults.cardColors(containerColor = if (elegido) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surfaceContainerHigh),
                                    border = if (elegido) androidx.compose.foundation.BorderStroke(2.dp, MaterialTheme.colorScheme.primary) else null,
                                ) {
                                    Row(Modifier.fillMaxWidth().padding(Esp.m), verticalAlignment = Alignment.CenterVertically) {
                                        Text(t.emoji.ifBlank { "⚠️" }, style = MaterialTheme.typography.titleLarge)
                                        Spacer(Modifier.width(Esp.s))
                                        Text(t.nombre, style = MaterialTheme.typography.titleSmall, maxLines = 2)
                                    }
                                }
                            }
                            if (fila.size == 1) Spacer(Modifier.weight(1f))
                        }
                    }
                }
                if (gruposComun.isNotEmpty()) {
                    Text("¿Dónde pasó?", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary)
                    FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                        FilterChip(chat.isEmpty(), { chat = ""; regla = 0 }, { Text("Chat privado", maxLines = 1) }, modifier = Modifier.testTag("adv_donde_privado"))
                        gruposComun.forEach { g -> FilterChip(chat == g.jid, { chat = g.jid; regla = 0 }, { Text(g.nombre.ifBlank { "Grupo" }, maxLines = 1) }, modifier = Modifier.testTag("adv_donde_${g.jid}")) }
                    }
                }
                if (reglas.isNotEmpty()) {
                    Text("¿Qué regla rompió? (opcional)", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary)
                    FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                        FilterChip(regla == 0L, { regla = 0 }, { Text("Ninguna") }, modifier = Modifier.testTag("adv_regla_ninguna"))
                        reglas.forEachIndexed { i, x -> FilterChip(regla == x.id, { regla = x.id }, { Text("${i + 1}. ${x.titulo}", maxLines = 1) }, modifier = Modifier.testTag("adv_regla_${x.id}")) }
                    }
                }
              }
                OutlinedTextField(motivo, { motivo = it.take(200) }, Modifier.fillMaxWidth().testTag("adv_motivo"), label = { Text("Motivo (opcional)") }, maxLines = 3)
            }
        },
        confirmButton = {
            TextButton({
                val t = tipo ?: return@TextButton
                trabajando = true
                scope.launch {
                    runCatching { Repo.advertirConTipo(botId, persona, chat, "", motivo.trim(), t.id, regla) }
                        .onSuccess { resultado = it; trabajando = false }
                        .onFailure { avisaToast(ctx, it.message ?: "No se pudo advertir", true); trabajando = false }
                }
            }, Modifier.testTag("adv_confirmar"), enabled = tipo != null && !trabajando) { Text("Advertir") }
        },
        dismissButton = { TextButton({ alCerrar() }, Modifier.testTag("adv_cancelar"), enabled = !trabajando) { Text("Cancelar") } },
    )
}

internal fun textoPasoEscalera(r: ResultadoAdvertenciaTipo): String = when {
    r.hecho == "solo_aviso" -> "No pude hacer lo que tocaba (${r.motivo.ifBlank { "sin permiso" }}). Solo te avisé para que decidas tú."
    r.sancionId > 0 -> "Toca expulsarla, pero no lo hago sola: te llegó una pregunta para que confirmes."
    r.hecho == "silenciado" || r.paso == "silenciar" || r.paso == "silencio_largo" -> "Quedó silenciada en el grupo."
    r.hecho == "expulsado" -> "La saqué del grupo."
    r.hecho == "borrado" -> "Borré su mensaje."
    r.paso == "avisar" || r.hecho == "avisado" -> "Le mandé un aviso."
    else -> "Quedó anotada."
}

// ---------- Marcar ----------

/** Marcar a una persona con etiquetas: se prende o se apaga con un toque (así también se «deshace»). */
@Composable
fun DialogoMarcarPersona(botId: Long, persona: String, nombre: String, puestas: List<EtiquetaItem>, alCerrar: () -> Unit, alCambio: () -> Unit, alAdministrar: () -> Unit = {}, alNota: () -> Unit = {}) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val todas by produceState<List<EtiquetaItem>?>(null) { value = runCatching { Repo.etiquetas(botId) }.getOrDefault(emptyList()) }
    val activas = remember { mutableStateListOf<Long>().also { l -> l.addAll(puestas.map { it.id }) } }
    AlertDialog(
        onDismissRequest = alCerrar, title = { Text("Marcar a $nombre") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("Toca una etiqueta para ponerla; vuélvela a tocar para quitarla.", style = MaterialTheme.typography.bodyMedium)
                val ts = todas
                when {
                    ts == null -> Text("Cargando…")
                    ts.isEmpty() -> Text("Aún no tienes etiquetas. Crea la primera.", Modifier.testTag("marcar_sin_etiquetas"))
                    else -> FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                        ts.forEach { e ->
                            FilterChip(
                                e.id in activas,
                                {
                                    val poner = e.id !in activas
                                    scope.launch {
                                        runCatching { Repo.ponerEtiquetaPersona(botId, persona, e.id, poner) }
                                            .onSuccess { if (poner) activas.add(e.id) else activas.remove(e.id); alCambio() }
                                            .onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
                                    }
                                },
                                { Text((e.emoji.ifBlank { "" } + " " + e.nombre).trim(), maxLines = 1) }, modifier = Modifier.testTag("marcar_etiqueta_${e.id}"),
                                leadingIcon = { PuntoRango(e.color, 10) },
                            )
                        }
                    }
                }
                OutlinedButton({ alAdministrar() }, Modifier.fillMaxWidth().testTag("marcar_administrar")) { Text("Crear o cambiar etiquetas") }
                OutlinedButton({ alNota() }, Modifier.fillMaxWidth().testTag("marcar_nota_persona")) { Text("Escribir una nota privada") }
            }
        },
        confirmButton = { TextButton(alCerrar, Modifier.testTag("marcar_listo")) { Text("Listo") } },
    )
}

/** Marcar un mensaje con etiquetas y/o una nota privada. Quitar una marca es volver a tocarla. */
@Composable
fun DialogoMarcarMensaje(botId: Long, chat: String, msgId: String, alCerrar: () -> Unit, alCambio: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val todas by produceState<List<EtiquetaItem>?>(null) { value = runCatching { Repo.etiquetas(botId) }.getOrDefault(emptyList()) }
    val activas = remember { mutableStateListOf<Long>() }
    var nota by remember { mutableStateOf("") }
    var cargado by remember { mutableStateOf(false) }
    androidx.compose.runtime.LaunchedEffect(chat, msgId) {
        runCatching { Repo.marcasDeMensaje(botId, chat, msgId) }.onSuccess { ms ->
            activas.clear(); activas.addAll(ms.filter { it.etiquetaId > 0 }.map { it.etiquetaId })
            nota = ms.firstOrNull { it.nota.isNotBlank() }?.nota.orEmpty()
        }
        cargado = true
    }
    HojaAlerta(
        onDismissRequest = alCerrar, title = { Text("Marcar este mensaje") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
              Column(Modifier.heightIn(max = 220.dp).verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("Etiquetas", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary)
                val ts = todas
                if (ts == null) Text("Cargando…")
                else if (ts.isEmpty()) Text("Aún no tienes etiquetas; puedes dejar solo una nota.", Modifier.testTag("marcarmsg_sin_etiquetas"))
                else FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                    ts.forEach { e ->
                        FilterChip(
                            e.id in activas,
                            {
                                val poner = e.id !in activas
                                scope.launch {
                                    runCatching { if (poner) Repo.marcarMensaje(botId, chat, msgId, e.id, nota.trim()) else Repo.quitarMarcaMensaje(botId, chat, msgId, e.id) }
                                        .onSuccess { if (poner) activas.add(e.id) else activas.remove(e.id); alCambio() }
                                        .onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
                                }
                            },
                            { Text((e.emoji.ifBlank { "" } + " " + e.nombre).trim(), maxLines = 1) }, modifier = Modifier.testTag("marcarmsg_etiqueta_${e.id}"),
                            leadingIcon = { PuntoRango(e.color, 10) },
                        )
                    }
                }
              }
                Text("Nota privada", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary)
                OutlinedTextField(nota, { nota = it.take(500) }, Modifier.fillMaxWidth().testTag("marcarmsg_nota"), placeholder = { Text("Solo tú la ves") }, minLines = 2, maxLines = 4, enabled = cargado)
                Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    OutlinedButton({
                        scope.launch {
                            runCatching { Repo.marcarMensaje(botId, chat, msgId, 0, nota.trim()) }
                                .onSuccess { avisaToast(ctx, if (nota.isBlank()) "Quité la nota" else "Nota guardada", false); alCambio() }
                                .onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
                        }
                    }, Modifier.testTag("marcarmsg_guardar_nota")) { Text("Guardar nota") }
                    if (activas.isNotEmpty() || nota.isNotBlank()) OutlinedButton({
                        scope.launch {
                            runCatching { Repo.quitarMarcaMensaje(botId, chat, msgId, -1) }
                                .onSuccess { activas.clear(); nota = ""; avisaToast(ctx, "Quité la marca", false); alCambio() }
                                .onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
                        }
                    }, Modifier.testTag("marcarmsg_quitar_todo")) { Text("Quitar marca") }
                }
            }
        },
        confirmButton = { TextButton(alCerrar, Modifier.testTag("marcarmsg_listo")) { Text("Listo") } },
    )
}
