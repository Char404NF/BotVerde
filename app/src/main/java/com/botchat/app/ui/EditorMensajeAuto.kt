@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.unit.dp
import androidx.compose.foundation.text.KeyboardOptions
import com.botchat.app.datos.AdjuntoSubido
import com.botchat.app.datos.CuandoAuto
import com.botchat.app.datos.DiaHorarioAuto
import com.botchat.app.datos.EscenarioAuto
import com.botchat.app.datos.FechaAnualAuto
import com.botchat.app.datos.MensajeAuto
import com.botchat.app.datos.PlantillaMensaje
import com.botchat.app.datos.PruebaAuto
import com.botchat.app.datos.RangoHoras
import com.botchat.app.datos.Repo
import com.botchat.app.datos.TipoAuto
import com.botchat.app.datos.VacacionAuto
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

// ---------- ayudas puras (probadas en FormatoMensajesAutoTest) ----------

/** "2026-12-20" → "20/12/2026" (lo que se escribe en la pantalla). */
fun isoADmy(iso: String): String = Regex("""^(\d{4})-(\d{2})-(\d{2})$""").matchEntire(iso.trim())?.let { "${it.groupValues[3]}/${it.groupValues[2]}/${it.groupValues[1]}" } ?: iso

/** "20/12/2026" → "2026-12-20", o null si no es una fecha de verdad. */
fun dmyAIso(dmy: String): String? {
    val m = Regex("""^(\d{1,2})/(\d{1,2})/(\d{4})$""").matchEntire(dmy.trim()) ?: return null
    val d = m.groupValues[1].toInt(); val mes = m.groupValues[2].toInt(); val a = m.groupValues[3].toInt()
    return runCatching { java.time.LocalDate.of(a, mes, d).toString() }.getOrNull()
}

private val HORA_RE = Regex("""^([01]?\d|2[0-3]):[0-5]\d$""")
fun horaBien(h: String) = HORA_RE.matches(h.trim())

/** Lo que se escribe en un cuadro de números (uno por línea) → la lista que guarda el mensaje. */
fun listaDePersonas(texto: String): List<String> =
    texto.split('\n', ',', ';').map { it.trim() }.filter { it.isNotEmpty() }.map { if ('@' in it) it else it.filter(Char::isDigit) }.filter { it.isNotEmpty() }.distinct()

private val NOMBRES_DIA = listOf(1 to "Lunes", 2 to "Martes", 3 to "Miércoles", 4 to "Jueves", 5 to "Viernes", 6 to "Sábado", 7 to "Domingo")

private fun nuevoMensaje(tipo: String, cfg: com.botchat.app.datos.ConfigAuto) = MensajeAuto(0, tipo, "", "", cfg, emptyList(), true)

// ---------- el editor ----------

/**
 * Editor de un mensaje automático: texto con barra de formato y vista previa como burbuja de chat, archivos, cuándo, a quién, repetición,
 * probar (sin mandar nada real) y, en Avanzado, el “Aa Estilo de letra”.
 */
@Composable
fun EditorMensajeAuto(botId: Long, id: Long, tipo: String, alVolver: () -> Unit) {
    val inicial by produceState<MensajeAuto?>(null, id, tipo) {
        value = if (id == 0L) nuevoMensaje(tipo, runCatching { Repo.configPorDefectoAuto(tipo) }.getOrDefault(com.botchat.app.datos.ConfigAuto()))
        else runCatching { Repo.obtenerAuto(botId, id) }.getOrNull() ?: nuevoMensaje(tipo, com.botchat.app.datos.ConfigAuto())
    }
    val plantillas by produceState<List<PlantillaMensaje>>(emptyList()) { value = runCatching { Repo.plantillasMensajes() }.getOrDefault(emptyList()) }
    val ini = inicial
    if (ini == null) { Box(Modifier.fillMaxSize(), Alignment.Center) { CircularProgressIndicator() }; return }
    CuerpoEditorAuto(botId, ini, plantillas.filter { it.tipo == ini.tipo }, alVolver)
}

@Composable
private fun CuerpoEditorAuto(botId: Long, ini: MensajeAuto, plantillas: List<PlantillaMensaje>, alVolver: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var m by remember(ini) { mutableStateOf(ini) }
    var texto by remember(ini) { mutableStateOf(TextFieldValue(ini.texto, TextRange(ini.texto.length))) }
    var guardando by remember { mutableStateOf(false) }
    var avanzado by remember { mutableStateOf(false) }
    val esAtajo = m.tipo == TipoAuto.ATAJO
    fun actual() = m.copy(texto = texto.text)

    // Vista previa: la arma Go con el mismo código que el envío real (variables, horario), con un nombre de ejemplo.
    val vista by produceState<PruebaAuto?>(null, texto.text, m.config, m.tipo, m.adjuntos, m.nombre) {
        delay(250)
        value = runCatching { Repo.probarAuto(botId, actual(), EscenarioAuto(nombre = "María")) }.getOrNull()
    }

    Scaffold(topBar = {
        TopAppBar(
            navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } },
            title = { Text(if (ini.id == 0L) "Nuevo: " + TipoAuto.titulo(m.tipo).lowercase() else "Cambiar: " + (m.nombre.ifBlank { TipoAuto.titulo(m.tipo) })) },
        )
    }) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Text(TipoAuto.descripcion(m.tipo), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Text("Solo se manda como respuesta a un mensaje: nunca escribe primero.", Modifier.testTag("nota_solo_responde"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.tertiary)

            if (plantillas.isNotEmpty()) SeccionEst("Plantillas listas") {
                FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                    plantillas.forEachIndexed { i, p ->
                        FilterChip(false, {
                            haptica(Toque.Suave)
                            m = m.copy(nombre = if (esAtajo) p.nombre.removePrefix("/") else p.nombre, config = if (esAtajo) m.config else p.config)
                            texto = TextFieldValue(p.texto, TextRange(p.texto.length))
                        }, { Text(p.nombre, maxLines = 1) }, Modifier.testTag("plantilla_$i"))
                    }
                }
            }

            OutlinedTextField(
                m.nombre, { m = m.copy(nombre = if (esAtajo) it.filter { c -> c.isLetterOrDigit() || c == '-' || c == '_' }.lowercase().take(20) else it.take(40)) },
                Modifier.fillMaxWidth().testTag("auto_nombre"), singleLine = true,
                label = { Text(if (esAtajo) "Nombre del atajo" else "Nombre (solo para ti)") }, prefix = if (esAtajo) ({ Text("/") }) else null,
                supportingText = { if (esAtajo) Text("Escribes /${m.nombre.ifBlank { "gracias" }} en un chat y el bot manda ahí este texto.") },
            )

            SeccionEst("Mensaje") {
                OutlinedTextField(texto, { texto = it.copy(text = it.text.take(1500)) }, Modifier.fillMaxWidth().testTag("auto_texto"), label = { Text("Qué dice") }, minLines = 4, maxLines = 12,
                    supportingText = { Text("${texto.text.length}/1500") })
                BarraFormato(texto, { texto = it }, "fmt")
                Text("{nombre} pone el nombre de quien escribió; {hora}, {dia} y {fecha} el momento del mensaje; {horario} tu horario de atención.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }

            SeccionEst("Así se verá") {
                VistaPreviaMensaje(vista?.texto?.takeIf { it.isNotBlank() } ?: texto.text, m.adjuntos, m.tipo)
                if (vista != null && !vista!!.aplica && !esAtajo && vista!!.motivo.isNotBlank()) Text("Con una persona de ejemplo ahora no se mandaría: ${vista!!.motivo}", Modifier.testTag("vista_no_aplica"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }

            SeccionEst("Archivos") {
                SelectorAdjuntos(botId, m.adjuntos, "auto", alCambiar = { m = m.copy(adjuntos = it) })
            }

            if (!esAtajo) {
                SeccionCuando(m.config.cuando, m.tipo) { c -> m = m.copy(config = m.config.copy(cuando = c)) }
                SeccionQuien(m) { m = it }
                SeccionRepeticion(m) { m = it }
            } else {
                SeccionEst("Cómo se usa") {
                    Text(
                        "Desde el teléfono: escribe exactamente /${m.nombre.ifBlank { "atajo" }} en un chat y el bot cambia tu mensaje por este texto (si ya no se puede editar, lo manda aparte) y manda los archivos. " +
                            "Desde esta app: al escribir “/” en la caja de mensaje aparece la lista de atajos y el texto se pone en la caja para que lo revises. Solo cuenta el mensaje exacto; el bot no hace nada más.",
                        style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }

            SeccionEst("Avanzado") {
                TextButton({ avanzado = !avanzado }, Modifier.testTag("avanzado_auto")) { Text(if (avanzado) "Ocultar" else "Mostrar opciones avanzadas") }
                if (avanzado) PanelEstiloLetra(texto, { texto = it }, "estilo_letra")
            }

            Row(verticalAlignment = Alignment.CenterVertically) {
                Text("Activo", Modifier.weight(1f))
                Switch(m.activo, { m = m.copy(activo = it) }, Modifier.testTag("auto_activo"))
            }
            val puede = !guardando && (texto.text.isNotBlank() || m.adjuntos.isNotEmpty()) && (!esAtajo || m.nombre.isNotBlank())
            Button({
                guardando = true
                haptica(Toque.Confirmar)
                scope.launch {
                    runCatching { Repo.guardarAuto(botId, actual()) }
                        .onSuccess { avisaToast(ctx, "Guardado", false); alVolver() }
                        .onFailure { avisaToast(ctx, it.message ?: "No se pudo guardar", true); guardando = false }
                }
            }, Modifier.fillMaxWidth().testTag("guardar_auto"), enabled = puede) { Text("Guardar") }
        }
    }
}

/** El mensaje como burbuja de chat (mío, a la derecha) con el mismo formato de WhatsApp que verá quien lo reciba. */
@Composable
fun VistaPreviaMensaje(texto: String, adjuntos: List<AdjuntoSubido>, tipo: String) {
    Row(Modifier.fillMaxWidth().testTag("vista_previa_auto"), horizontalArrangement = Arrangement.End) {
        Surface(shape = RoundedCornerShape(16.dp), color = MaterialTheme.colorScheme.primaryContainer, modifier = Modifier.widthIn(max = 300.dp)) {
            Column(Modifier.padding(horizontal = 12.dp, vertical = 8.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                adjuntos.forEach { a ->
                    Text((when (a.tipo) { "imagen" -> "🖼 "; "video" -> "🎞 "; "audio" -> "🎤 "; else -> "📄 " }) + (if (a.tipo == "audio") "Nota de voz" else a.nombre.ifBlank { a.tipo }), style = MaterialTheme.typography.labelMedium, modifier = Modifier.testTag("previa_archivo"))
                }
                if (texto.isNotBlank()) Text(FormatoWhatsApp.construir(texto), Modifier.testTag("previa_texto"))
                else if (adjuntos.isEmpty()) Text("(vacío)", color = MaterialTheme.colorScheme.onSurfaceVariant)
                Text(TipoAuto.etiqueta(tipo).ifBlank { "Automático" }, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.testTag("previa_etiqueta"))
            }
        }
    }
}

/** Un número que se puede borrar y volver a escribir sin que el campo "rebote" al valor anterior mientras está vacío. */
@Composable
private fun CampoNumero(valor: Int, etiqueta: String, tag: String, digitos: Int, modifier: Modifier, alCambiar: (Int) -> Unit) {
    var txt by remember(valor) { mutableStateOf(valor.toString()) }
    OutlinedTextField(txt, { t -> txt = t.filter(Char::isDigit).take(digitos); txt.toIntOrNull()?.let(alCambiar) }, modifier.testTag(tag), label = { Text(etiqueta) }, singleLine = true, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number))
}

// ---------- cuándo ----------

@Composable
private fun SeccionCuando(c: CuandoAuto, tipo: String, alCambiar: (CuandoAuto) -> Unit) {
    SeccionEst("Cuándo se manda") {
        Row(verticalAlignment = Alignment.CenterVertically) { Text("Siempre", Modifier.weight(1f)); Switch(c.siempre, { alCambiar(c.copy(siempre = it)) }, Modifier.testTag("cuando_siempre")) }
        if (tipo == TipoAuto.AUSENCIA && !c.siempre && c.horario.isEmpty() && c.vacaciones.isEmpty() && c.fechas.isEmpty())
            Text("Elige al menos una: fuera de horario, vacaciones o una fecha especial.", Modifier.testTag("cuando_falta"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
        // horario por día con hasta dos rangos
        Row(verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) { Text("Según un horario"); Text("Por día, con hasta dos rangos de horas (por ejemplo mañana y tarde).", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
            Switch(c.horario.isNotEmpty(), { si -> alCambiar(c.copy(horario = if (si) (1..5).map { DiaHorarioAuto(it, listOf(RangoHoras("09:00", "13:00"), RangoHoras("15:00", "18:00"))) } else emptyList())) }, Modifier.testTag("cuando_horario"))
        }
        if (c.horario.isNotEmpty()) {
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                FilterChip(c.horarioModo == "dentro", { alCambiar(c.copy(horarioModo = "dentro")) }, { Text("Dentro del horario") }, Modifier.testTag("horario_modo_dentro"))
                FilterChip(c.horarioModo == "fuera", { alCambiar(c.copy(horarioModo = "fuera")) }, { Text("Fuera del horario") }, Modifier.testTag("horario_modo_fuera"))
            }
            NOMBRES_DIA.forEach { (n, nombre) ->
                val dia = c.horario.firstOrNull { it.dia == n }
                fun cambiaDia(f: (DiaHorarioAuto?) -> DiaHorarioAuto?) {
                    val nuevo = f(dia)
                    alCambiar(c.copy(horario = (c.horario.filter { it.dia != n } + listOfNotNull(nuevo)).sortedBy { it.dia }))
                }
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Switch(dia != null, { si -> cambiaDia { if (si) DiaHorarioAuto(n, listOf(RangoHoras("09:00", "18:00"))) else null } }, Modifier.testTag("dia_on_$n"))
                    Text("  $nombre", Modifier.weight(1f))
                }
                if (dia != null) {
                    dia.rangos.forEachIndexed { r, rg ->
                        Row(Modifier.padding(start = Esp.xl), horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalAlignment = Alignment.CenterVertically) {
                            OutlinedTextField(rg.desde, { t -> cambiaDia { d -> d?.copy(rangos = d.rangos.mapIndexed { j, x -> if (j == r) x.copy(desde = t.take(5)) else x }) } }, Modifier.weight(1f).testTag("dia_${n}_r${r + 1}_desde"), label = { Text("Desde") }, singleLine = true, isError = !horaBien(rg.desde))
                            OutlinedTextField(rg.hasta, { t -> cambiaDia { d -> d?.copy(rangos = d.rangos.mapIndexed { j, x -> if (j == r) x.copy(hasta = t.take(5)) else x }) } }, Modifier.weight(1f).testTag("dia_${n}_r${r + 1}_hasta"), label = { Text("Hasta") }, singleLine = true, isError = !horaBien(rg.hasta))
                            IconButton({ cambiaDia { d -> d?.copy(rangos = d.rangos.filterIndexed { j, _ -> j != r }).takeIf { it?.rangos?.isNotEmpty() == true } } }, Modifier.testTag("dia_${n}_quita_r${r + 1}")) { Icon(Ico.Borrar, "Quitar el rango") }
                        }
                    }
                    if (dia.rangos.size < 2) TextButton({ cambiaDia { d -> d?.copy(rangos = d.rangos + RangoHoras("15:00", "18:00")) } }, Modifier.padding(start = Esp.xl).testTag("dia_${n}_agrega_r2")) { Text("+ Segundo rango") }
                }
            }
        }
        // vacaciones
        Text("Vacaciones", style = MaterialTheme.typography.labelLarge)
        c.vacaciones.forEachIndexed { i, v ->
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalAlignment = Alignment.CenterVertically) {
                var desde by remember(v.desde) { mutableStateOf(isoADmy(v.desde)) }
                var hasta by remember(v.hasta) { mutableStateOf(isoADmy(v.hasta)) }
                OutlinedTextField(desde, { t -> desde = t.take(10); dmyAIso(t)?.let { iso -> alCambiar(c.copy(vacaciones = c.vacaciones.mapIndexed { j, x -> if (j == i) x.copy(desde = iso) else x })) } }, Modifier.weight(1f).testTag("vac_desde_$i"), label = { Text("Desde dd/mm/aaaa") }, singleLine = true, isError = dmyAIso(desde) == null)
                OutlinedTextField(hasta, { t -> hasta = t.take(10); dmyAIso(t)?.let { iso -> alCambiar(c.copy(vacaciones = c.vacaciones.mapIndexed { j, x -> if (j == i) x.copy(hasta = iso) else x })) } }, Modifier.weight(1f).testTag("vac_hasta_$i"), label = { Text("Hasta dd/mm/aaaa") }, singleLine = true, isError = dmyAIso(hasta) == null)
                IconButton({ alCambiar(c.copy(vacaciones = c.vacaciones.filterIndexed { j, _ -> j != i })) }, Modifier.testTag("vac_quita_$i")) { Icon(Ico.Borrar, "Quitar") }
            }
        }
        OutlinedButton({
            val hoy = java.time.LocalDate.now()
            alCambiar(c.copy(vacaciones = c.vacaciones + VacacionAuto("", hoy.plusDays(7).toString(), hoy.plusDays(14).toString())))
        }, Modifier.testTag("vac_agrega")) { Text("Agregar vacaciones") }
        // fechas especiales que se repiten cada año
        Text("Fechas especiales (se repiten cada año)", style = MaterialTheme.typography.labelLarge)
        c.fechas.forEachIndexed { i, f ->
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalAlignment = Alignment.CenterVertically) {
                CampoNumero(f.dia, "Día", "fecha_dia_$i", 2, Modifier.weight(1f)) { d -> alCambiar(c.copy(fechas = c.fechas.mapIndexed { j, x -> if (j == i) x.copy(dia = d) else x })) }
                CampoNumero(f.mes, "Mes", "fecha_mes_$i", 2, Modifier.weight(1f)) { mm -> alCambiar(c.copy(fechas = c.fechas.mapIndexed { j, x -> if (j == i) x.copy(mes = mm) else x })) }
                OutlinedTextField(f.nombre, { t -> alCambiar(c.copy(fechas = c.fechas.mapIndexed { j, x -> if (j == i) x.copy(nombre = t.take(30)) else x })) }, Modifier.weight(2f).testTag("fecha_nombre_$i"), label = { Text("Nombre") }, singleLine = true)
                IconButton({ alCambiar(c.copy(fechas = c.fechas.filterIndexed { j, _ -> j != i })) }, Modifier.testTag("fecha_quita_$i")) { Icon(Ico.Borrar, "Quitar") }
            }
        }
        OutlinedButton({ alCambiar(c.copy(fechas = c.fechas + FechaAnualAuto("", 25, 12))) }, Modifier.testTag("fecha_agrega")) { Text("Agregar fecha especial") }
        // zona horaria
        Text("Zona horaria", style = MaterialTheme.typography.labelLarge)
        Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
            FilterChip(c.zona.isEmpty(), { alCambiar(c.copy(zona = "")) }, { Text("La del bot") }, Modifier.testTag("zona_bot"))
            FilterChip(c.zona.isNotEmpty(), { if (c.zona.isEmpty()) alCambiar(c.copy(zona = "America/Mexico_City")) }, { Text("Otra") }, Modifier.testTag("zona_otra"))
        }
        if (c.zona.isNotEmpty()) OutlinedTextField(c.zona, { alCambiar(c.copy(zona = it.trim())) }, Modifier.fillMaxWidth().testTag("zona_texto"), label = { Text("Zona (por ejemplo America/Bogota)") }, singleLine = true)
    }
}

// ---------- a quién ----------

@Composable
private fun SeccionQuien(m: MensajeAuto, alCambiar: (MensajeAuto) -> Unit) {
    val q = m.config.quien
    fun cambia(n: com.botchat.app.datos.QuienAuto) = alCambiar(m.copy(config = m.config.copy(quien = n)))
    SeccionEst("A quién") {
        FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
            listOf("todos" to "Todos", "desconocidos" to "Solo desconocidos", "contactos" to "Solo contactos").forEach { (v, t) ->
                FilterChip(q.personas == v, { cambia(q.copy(personas = v)) }, { Text(t) }, Modifier.testTag("quien_$v"))
            }
        }
        Text("Un contacto es quien está en tu agenda, tiene un rango o ya aprobaste. Lo demás son desconocidos.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Row(verticalAlignment = Alignment.CenterVertically) { Text("En chats privados", Modifier.weight(1f)); Switch(q.privados, { cambia(q.copy(privados = it)) }, Modifier.testTag("quien_privados")) }
        Row(verticalAlignment = Alignment.CenterVertically) { Text("En grupos (solo si me mencionan)", Modifier.weight(1f)); Switch(q.grupos, { cambia(q.copy(grupos = it)) }, Modifier.testTag("quien_grupos")) }
        var excepto by remember(m.id) { mutableStateOf(q.excepto.joinToString("\n") { it.removeSuffix("@s.whatsapp.net") }) }
        var solo by remember(m.id) { mutableStateOf(q.soloLista.joinToString("\n") { it.removeSuffix("@s.whatsapp.net") }) }
        OutlinedTextField(excepto, { excepto = it; cambia(q.copy(excepto = listaDePersonas(it))) }, Modifier.fillMaxWidth().testTag("quien_excepto"), label = { Text("Excepto estas personas (números con código de país, uno por línea)") }, minLines = 2, maxLines = 5)
        OutlinedTextField(solo, { solo = it; cambia(q.copy(soloLista = listaDePersonas(it))) }, Modifier.fillMaxWidth().testTag("quien_solo"), label = { Text("Solo estas personas (déjalo vacío para todas)") }, minLines = 2, maxLines = 5)
    }
}

// ---------- repetición ----------

@Composable
private fun SeccionRepeticion(m: MensajeAuto, alCambiar: (MensajeAuto) -> Unit) {
    val c = m.config
    SeccionEst("Cada cuánto") {
        if (m.tipo == TipoAuto.BIENVENIDA) {
            Text("Además de la primera vez que escribe, ¿volver a dar la bienvenida si pasan muchos días sin que escriba?", style = MaterialTheme.typography.bodySmall)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                listOf(0 to "Solo la primera vez", 7 to "También tras 7 días", 14 to "Tras 14 días", 30 to "Tras 30 días").forEach { (d, t) ->
                    FilterChip(c.silencioDias == d, { alCambiar(m.copy(config = c.copy(silencioDias = d))) }, { Text(t) }, Modifier.testTag("silencio_$d"))
                }
            }
        } else {
            Text("A la misma persona se le manda una vez cada…", style = MaterialTheme.typography.bodySmall)
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalAlignment = Alignment.CenterVertically) {
                CampoNumero(c.repeticion.cada, "Cada", "rep_cada", 5, Modifier.weight(1f)) { n -> alCambiar(m.copy(config = c.copy(repeticion = c.repeticion.copy(cada = n)))) }
                listOf("minutos" to "min", "horas" to "horas", "dias" to "días").forEach { (u, t) ->
                    FilterChip(c.repeticion.unidad == u, { alCambiar(m.copy(config = c.copy(repeticion = c.repeticion.copy(unidad = u)))) }, { Text(t) }, Modifier.testTag("rep_unidad_$u"))
                }
            }
            if (m.tipo == TipoAuto.RECIBIDO) Text("Así el “ya vi tu mensaje” sale una vez por conversación, no con cada mensaje.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}
