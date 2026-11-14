@file:OptIn(ExperimentalMaterial3Api::class, androidx.compose.foundation.layout.ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Slider
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.foundation.text.KeyboardOptions
import com.botchat.app.datos.AdjuntoDef
import com.botchat.app.datos.AdjuntoSubido
import com.botchat.app.datos.AjustesMenuDef
import com.botchat.app.datos.AutoDef
import com.botchat.app.datos.CampoDef
import com.botchat.app.datos.ContactoDef
import com.botchat.app.datos.HorarioDef
import com.botchat.app.datos.OpcionDef
import com.botchat.app.datos.PasoDef
import com.botchat.app.datos.Repo
import com.botchat.app.datos.UbicacionDef
import com.botchat.app.medios.PrepararAdjunto
import kotlinx.coroutines.launch

/** Piezas del editor de menús de la etapa C: formularios, archivos/ubicación/contacto, opciones avanzadas y ajustes del menú. */

internal val TIPOS_CAMPO = listOf("texto" to "Texto", "numero" to "Número", "correo" to "Correo", "telefono" to "Teléfono", "fecha" to "Fecha", "opcion" to "Opciones")

private val DIAS_OPC = listOf(1 to "L", 2 to "M", 3 to "X", 4 to "J", 5 to "V", 6 to "S", 7 to "D")
private val HORA_OPC = Regex("""^([01]?\d|2[0-3]):[0-5]\d$""")

internal fun campoNuevo(p: PasoDef): CampoDef {
    var n = p.campos.size + 1
    while (p.campos.any { it.id == "campo$n" }) n++
    return CampoDef(id = "campo$n", etiqueta = "Dato $n", pregunta = "Escribe aquí la pregunta.", tipo = "texto")
}

/** Resumen corto de un formulario para la lista de pasos. */
internal fun resumenFormulario(p: PasoDef): String {
    val t = p.titulo.ifBlank { p.campos.firstOrNull()?.pregunta.orEmpty() }.replace('\n', ' ').trim()
    val n = p.campos.size
    return (if (t.isBlank()) "" else t.take(30) + " · ") + if (n == 1) "1 pregunta" else "$n preguntas"
}

@Composable
private fun CampoTexto(valor: String, etiqueta: String, tag: String, lineas: Int = 3, teclado: KeyboardType = KeyboardType.Text, alCambiar: (String) -> Unit) {
    OutlinedTextField(valor, alCambiar, Modifier.fillMaxWidth().testTag(tag), label = { Text(etiqueta) }, maxLines = lineas, keyboardOptions = KeyboardOptions(keyboardType = teclado))
}

// ---------- formulario ----------

@Composable
internal fun EditorFormulario(
    d: AutoDef, p: PasoDef, cambia: (String, (PasoDef) -> PasoDef) -> Unit,
    selector: @Composable (actual: String, tag: String, alElegir: (String) -> Unit, alNuevo: (String) -> Unit) -> Unit, alNuevoPaso: (String, (PasoDef) -> AutoDef) -> Unit,
) {
    CampoTexto(p.titulo, "Introducción (opcional)", "titulo_${p.id}") { t -> cambia(p.id) { it.copy(titulo = t) } }
    Text("Se hace una pregunta a la vez. Cada respuesta se revisa; si no sirve, se vuelve a preguntar. Escribir “cancelar” lo cancela en cualquier momento.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    p.campos.forEachIndexed { i, c ->
        Card(Modifier.fillMaxWidth().testTag("campo_form_${p.id}_$i"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface)) {
            Column(Modifier.padding(Esp.m), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                fun cambiaCampo(f: (CampoDef) -> CampoDef) = cambia(p.id) { q -> q.copy(campos = q.campos.mapIndexed { j, x -> if (j == i) f(x) else x }) }
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text("Pregunta ${i + 1}", Modifier.weight(1f), style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.primary)
                    IconButton({ if (i > 0) cambia(p.id) { q -> q.copy(campos = q.campos.toMutableList().also { l -> val x = l.removeAt(i); l.add(i - 1, x) }) } }, Modifier.testTag("campo_sube_${p.id}_$i"), enabled = i > 0) { Icon(Ico.Anterior, "Subir") }
                    IconButton({ if (i < p.campos.lastIndex) cambia(p.id) { q -> q.copy(campos = q.campos.toMutableList().also { l -> val x = l.removeAt(i); l.add(i + 1, x) }) } }, Modifier.testTag("campo_baja_${p.id}_$i"), enabled = i < p.campos.lastIndex) { Icon(Ico.Siguiente, "Bajar") }
                    IconButton({ cambia(p.id) { q -> q.copy(campos = q.campos.filterIndexed { j, _ -> j != i }) } }, Modifier.testTag("campo_quita_${p.id}_$i")) { Icon(Ico.Borrar, "Quitar pregunta") }
                }
                CampoTexto(c.pregunta, "Lo que se pregunta", "campo_pregunta_${p.id}_$i") { t -> cambiaCampo { it.copy(pregunta = t) } }
                OutlinedTextField(c.etiqueta, { t -> cambiaCampo { it.copy(etiqueta = t.take(30)) } }, Modifier.fillMaxWidth().testTag("campo_etiqueta_${p.id}_$i"), label = { Text("Nombre del dato (para el resumen)") }, singleLine = true)
                Text("Tipo de dato", style = MaterialTheme.typography.labelMedium)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(4.dp), modifier = Modifier.fillMaxWidth()) {
                    TIPOS_CAMPO.forEach { (t, n) -> FilterChip(c.tipo == t, { cambiaCampo { it.copy(tipo = t) } }, label = { Text(n) }, modifier = Modifier.testTag("campo_tipo_${p.id}_${i}_$t")) }
                }
                if (c.tipo == "opcion") OutlinedTextField(
                    c.opciones.joinToString("\n"), { t -> cambiaCampo { it.copy(opciones = t.split('\n').map { x -> x.trim() }) } }, Modifier.fillMaxWidth().testTag("campo_opciones_${p.id}_$i"),
                    label = { Text("Opciones (una por línea)") }, minLines = 2, maxLines = 6,
                )
                if (c.tipo == "numero") Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    OutlinedTextField(c.min?.let { fmtNum(it) } ?: "", { t -> cambiaCampo { it.copy(min = t.replace(',', '.').toDoubleOrNull()) } }, Modifier.weight(1f).testTag("campo_min_${p.id}_$i"), label = { Text("Mínimo") }, singleLine = true, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal))
                    OutlinedTextField(c.max?.let { fmtNum(it) } ?: "", { t -> cambiaCampo { it.copy(max = t.replace(',', '.').toDoubleOrNull()) } }, Modifier.weight(1f).testTag("campo_max_${p.id}_$i"), label = { Text("Máximo") }, singleLine = true, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal))
                }
                if (c.tipo == "telefono") Text("Se guarda con el formato internacional (+52…). Sin código de país se usa el país de Ajustes → Avanzado → Menús.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                if (c.tipo == "fecha") Text("Entiende 25/12/2026, 25 de diciembre, hoy, mañana… y la guarda como dd/mm/aaaa.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                CampoTexto(c.error, "Mensaje si se equivoca (opcional)", "campo_error_${p.id}_$i", lineas = 2) { t -> cambiaCampo { it.copy(error = t) } }
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text("Se puede omitir escribiendo “omitir”", Modifier.weight(1f))
                    Switch(c.opcional, { si -> cambiaCampo { it.copy(opcional = si) } }, Modifier.testTag("campo_opcional_${p.id}_$i"))
                }
            }
        }
    }
    OutlinedButton({ cambia(p.id) { it.copy(campos = it.campos + campoNuevo(it)) } }, Modifier.testTag("agregar_campo_${p.id}")) { Icon(Ico.Agregar, null); Text(" Agregar pregunta") }
    Column {
        val intentos = if (p.maxReintentos == 0) 3 else p.maxReintentos
        Text("Intentos por pregunta antes de cancelar: $intentos")
        Slider(intentos.toFloat(), { v -> cambia(p.id) { it.copy(maxReintentos = v.toInt().coerceIn(1, 10)) } }, valueRange = 1f..10f, steps = 8, modifier = Modifier.testTag("form_intentos_${p.id}"))
    }
    Row(verticalAlignment = Alignment.CenterVertically) {
        Column(Modifier.weight(1f)) { Text("Mostrar resumen para confirmar"); Text("“¿Es correcto? 1 Sí, 2 Corregir”", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        Switch(!p.sinResumen, { si -> cambia(p.id) { it.copy(sinResumen = !si) } }, Modifier.testTag("form_resumen_${p.id}"))
    }
    CampoTexto(p.texto, "Mensaje de agradecimiento (puedes usar {nombre_del_dato})", "texto_${p.id}") { t -> cambia(p.id) { it.copy(texto = t) } }
    Text("Al confirmar se guarda cifrado en Formularios recibidos y te llega un aviso con un título neutro.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    Text("Después:", style = MaterialTheme.typography.labelMedium)
    selector(p.siguiente, "siguiente_${p.id}", { dest -> cambia(p.id) { it.copy(siguiente = dest) } }, { tipo -> alNuevoPaso(tipo) { nuevo -> d.copy(pasos = d.pasos.map { q -> if (q.id == p.id) q.copy(siguiente = nuevo.id) else q } + nuevo) } })
}

private fun fmtNum(v: Double): String = if (v == Math.floor(v) && !v.isInfinite()) v.toLong().toString() else v.toString()

// ---------- archivos, ubicación y contacto de una respuesta ----------

@Composable
internal fun ExtrasRespuesta(botId: Long, p: PasoDef, cambia: (String, (PasoDef) -> PasoDef) -> Unit) {
    // Los archivos de una opción de menú usan el mismo selector (y el mismo servicio de archivos cifrados) que los mensajes automáticos.
    SelectorAdjuntos(
        botId, p.adjuntos.map { AdjuntoSubido(it.id, it.tipo, it.mime, it.nombre, 0) }, p.id,
        alCambiar = { l -> cambia(p.id) { q -> q.copy(adjuntos = l.map { AdjuntoDef(it.id, it.tipo, it.mime, it.nombre) }.take(3)) } },
    )
    Text("Si el texto va junto con una foto, video o documento, sale como pie.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    val u = p.ubicacion
    Row(verticalAlignment = Alignment.CenterVertically) {
        Text("Mandar una ubicación", Modifier.weight(1f))
        Switch(u != null, { si -> cambia(p.id) { it.copy(ubicacion = if (si) UbicacionDef(0.0, 0.0, "") else null) } }, Modifier.testTag("sw_ubicacion_${p.id}"))
    }
    if (u != null) {
        Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
            OutlinedTextField(if (u.lat == 0.0) "" else u.lat.toString(), { t -> cambia(p.id) { q -> q.copy(ubicacion = u.copy(lat = t.replace(',', '.').toDoubleOrNull() ?: 0.0)) } }, Modifier.weight(1f).testTag("ubicacion_lat_${p.id}"), label = { Text("Latitud") }, singleLine = true, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal))
            OutlinedTextField(if (u.lon == 0.0) "" else u.lon.toString(), { t -> cambia(p.id) { q -> q.copy(ubicacion = u.copy(lon = t.replace(',', '.').toDoubleOrNull() ?: 0.0)) } }, Modifier.weight(1f).testTag("ubicacion_lon_${p.id}"), label = { Text("Longitud") }, singleLine = true, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal))
        }
        OutlinedTextField(u.nombre, { t -> cambia(p.id) { q -> q.copy(ubicacion = u.copy(nombre = t)) } }, Modifier.fillMaxWidth().testTag("ubicacion_nombre_${p.id}"), label = { Text("Nombre del lugar") }, singleLine = true)
        OutlinedTextField(u.direccion, { t -> cambia(p.id) { q -> q.copy(ubicacion = u.copy(direccion = t)) } }, Modifier.fillMaxWidth().testTag("ubicacion_direccion_${p.id}"), label = { Text("Dirección (opcional)") }, singleLine = true)
        Text("Tip: en Google Maps mantén pulsado el lugar y copia los dos números.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
    val c = p.contacto
    Row(verticalAlignment = Alignment.CenterVertically) {
        Text("Mandar un contacto", Modifier.weight(1f))
        Switch(c != null, { si -> cambia(p.id) { it.copy(contacto = if (si) ContactoDef("", "") else null) } }, Modifier.testTag("sw_contacto_${p.id}"))
    }
    if (c != null) {
        OutlinedTextField(c.nombre, { t -> cambia(p.id) { q -> q.copy(contacto = c.copy(nombre = t)) } }, Modifier.fillMaxWidth().testTag("contacto_nombre_${p.id}"), label = { Text("Nombre") }, singleLine = true)
        OutlinedTextField(c.telefono, { t -> cambia(p.id) { q -> q.copy(contacto = c.copy(telefono = t)) } }, Modifier.fillMaxWidth().testTag("contacto_telefono_${p.id}"), label = { Text("Teléfono con código de país (+52…)") }, singleLine = true, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Phone))
    }
}

// ---------- opciones avanzadas de una opción de menú ----------

@Composable
internal fun AjustesDeOpcion(p: PasoDef, i: Int, o: OpcionDef, cambia: (String, (PasoDef) -> PasoDef) -> Unit) {
    var abierto by remember { mutableStateOf(o.ambito.isNotEmpty() || o.horario != null) }
    TextButton({ abierto = !abierto }, Modifier.testTag("avanzada_opcion_${p.id}_$i")) { Text(if (abierto) "Ocultar quién la ve y cuándo" else "Quién la ve y cuándo…") }
    fun cambiaO(f: (OpcionDef) -> OpcionDef) = cambia(p.id) { q -> q.copy(opciones = q.opciones.mapIndexed { j, x -> if (j == i) f(x) else x }) }
    androidx.compose.animation.AnimatedVisibility(abierto) {
    Column(Modifier.padding(start = Esp.m), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
        Text("Dónde se ofrece", style = MaterialTheme.typography.labelMedium)
        val ambitos = listOf("" to "Todos lados", "privado" to "Solo privado", "grupo" to "Solo grupos")
        SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().testTag("ambito_opcion_${p.id}_$i")) {
            ambitos.forEachIndexed { k, (v, t) -> SegmentedButton(o.ambito == v, { cambiaO { it.copy(ambito = v) } }, SegmentedButtonDefaults.itemShape(k, ambitos.size), modifier = Modifier.testTag("ambito_${p.id}_${i}_${v.ifEmpty { "todos" }}")) { Text(t, maxLines = 1) } }
        }
        if (o.ambito == "privado") Text("En un grupo la opción se ve marcada “(solo en privado)” y, si la eligen, el bot contesta “escríbeme al privado”. El bot nunca escribe al privado por su cuenta.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        val h = o.horario ?: HorarioDef()
        Text("Solo en este horario (vacío = siempre)", style = MaterialTheme.typography.labelMedium)
        Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
            DIAS_OPC.forEach { (n, l) -> FilterChip(n in h.dias, { cambiaO { q -> val hh = q.horario ?: HorarioDef(); q.copy(horario = hh.copy(dias = if (n in hh.dias) hh.dias - n else (hh.dias + n).sorted())) } }, label = { Text(l) }, modifier = Modifier.testTag("opc_dia_${p.id}_${i}_$n")) }
        }
        Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
            OutlinedTextField(h.desde, { t -> cambiaO { q -> q.copy(horario = (q.horario ?: HorarioDef()).copy(desde = t.take(5))) } }, Modifier.weight(1f).testTag("opc_desde_${p.id}_$i"), label = { Text("Desde (HH:MM)") }, singleLine = true, isError = h.desde.isNotEmpty() && !HORA_OPC.matches(h.desde))
            OutlinedTextField(h.hasta, { t -> cambiaO { q -> q.copy(horario = (q.horario ?: HorarioDef()).copy(hasta = t.take(5))) } }, Modifier.weight(1f).testTag("opc_hasta_${p.id}_$i"), label = { Text("Hasta (HH:MM)") }, singleLine = true, isError = h.hasta.isNotEmpty() && !HORA_OPC.matches(h.hasta))
        }
        Text("Quién la ve según su rango se elige en Ajustes → Rangos y permisos (“Menús que ve”).", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
    }
}

// ---------- pestaña "Ajustes del menú" ----------

@Composable
internal fun PestanaAjustesMenu(d: AutoDef, alCambiar: (AutoDef) -> Unit) {
    val a = d.ajustes
    fun cambia(f: (AjustesMenuDef) -> AjustesMenuDef) = alCambiar(d.copy(ajustes = f(a)))
    Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).padding(Esp.l).testTag("pestana_ajustes_menu"), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
        Text("Lo que dejes vacío usa el valor general (Ajustes → Avanzado → Menús).", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text("Cuando no entiende", style = MaterialTheme.typography.titleMedium)
        Column {
            Text(if (a.errMax == 0) "Errores seguidos antes de parar: general" else "Errores seguidos antes de parar: ${a.errMax}")
            Slider(a.errMax.toFloat(), { v -> cambia { it.copy(errMax = v.toInt()) } }, valueRange = 0f..10f, steps = 9, modifier = Modifier.testTag("aj_errmax"))
            Text("1.º error: reconoce y ofrece las opciones. 2.º: ofrece hablar con una persona. El último: para y reinicia la conversación. Los saludos, “gracias” y los emojis sueltos no cuentan como error.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        CampoTexto(a.txtN1, "Mensaje del 1.er error", "aj_n1") { t -> cambia { it.copy(txtN1 = t) } }
        CampoTexto(a.txtN2, "Mensaje del 2.º error (ofrece a una persona)", "aj_n2") { t -> cambia { it.copy(txtN2 = t) } }
        CampoTexto(a.txtN3, "Mensaje al parar", "aj_n3") { t -> cambia { it.copy(txtN3 = t) } }
        CampoTexto(a.txtNoTexto, "Si mandan una foto, audio, sticker o archivo", "aj_notexto") { t -> cambia { it.copy(txtNoTexto = t) } }

        Text("Caducidad", style = MaterialTheme.typography.titleMedium)
        Text("Si la persona deja de contestar, la conversación caduca en silencio: el bot no manda recordatorios ni mensajes de cierre.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
            OutlinedTextField(if (a.expiraMin == 0) "" else a.expiraMin.toString(), { t -> cambia { it.copy(expiraMin = t.filter(Char::isDigit).take(5).toIntOrNull() ?: 0) } }, Modifier.weight(1f).testTag("aj_expira"), label = { Text("En privado (minutos)") }, singleLine = true, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number))
            OutlinedTextField(if (a.expiraGrupoMin == 0) "" else a.expiraGrupoMin.toString(), { t -> cambia { it.copy(expiraGrupoMin = t.filter(Char::isDigit).take(5).toIntOrNull() ?: 0) } }, Modifier.weight(1f).testTag("aj_expira_grupo"), label = { Text("En grupos (minutos)") }, singleLine = true, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number))
        }
        Text("Al volver a escribir", style = MaterialTheme.typography.labelLarge)
        val volver = listOf("" to "General", "retomar" to "Retomar", "reiniciar" to "Reiniciar")
        SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().testTag("aj_volver")) {
            volver.forEachIndexed { k, (v, t) -> SegmentedButton(a.volver == v, { cambia { it.copy(volver = v) } }, SegmentedButtonDefaults.itemShape(k, volver.size), modifier = Modifier.testTag("aj_volver_${v.ifEmpty { "general" }}")) { Text(t, maxLines = 1) } }
        }
        Text(when (a.volver) { "retomar" -> "Sigue donde iba, con lo que ya había contestado."; "reiniciar" -> "Empieza el menú desde el principio."; else -> "Usa lo que elijas en Avanzado." }, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)

        Text("Hablar con una persona", style = MaterialTheme.typography.titleMedium)
        Text("Marca el chat como Pendiente, te avisa con una notificación, conserva lo que ya contestó y el bot calla en ese chat. Usa el horario de este menú (pestaña Cuándo y dónde) o el general.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        CampoTexto(a.humanoDentro, "Mensaje si estás en horario", "aj_humano_dentro") { t -> cambia { it.copy(humanoDentro = t) } }
        CampoTexto(a.humanoFuera, "Mensaje si estás fuera de horario (puedes usar {horario})", "aj_humano_fuera") { t -> cambia { it.copy(humanoFuera = t) } }

        Text("En grupos", style = MaterialTheme.typography.titleMedium)
        OutlinedTextField(if (a.maxSesionesGrupo == 0) "" else a.maxSesionesGrupo.toString(), { t -> cambia { it.copy(maxSesionesGrupo = t.filter(Char::isDigit).take(3).toIntOrNull() ?: 0) } }, Modifier.fillMaxWidth().testTag("aj_max_sesiones"), label = { Text("Personas usando el menú a la vez (vacío = general)") }, singleLine = true, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number))
        Text("¿Cualquiera puede contestar el menú de otra persona?", style = MaterialTheme.typography.labelLarge)
        val cual = listOf("" to "General", "si" to "Sí", "no" to "No")
        SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().testTag("aj_cualquiera")) {
            cual.forEachIndexed { k, (v, t) -> SegmentedButton(a.cualquiera == v, { cambia { it.copy(cualquiera = v) } }, SegmentedButtonDefaults.itemShape(k, cual.size), modifier = Modifier.testTag("aj_cualquiera_${v.ifEmpty { "general" }}")) { Text(t, maxLines = 1) } }
        }
        Spacer(Modifier.size(80.dp))
    }
}
