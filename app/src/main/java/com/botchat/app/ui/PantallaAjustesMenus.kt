@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
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
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.HorarioDef
import com.botchat.app.datos.Repo
import kotlinx.coroutines.launch
import org.json.JSONArray
import org.json.JSONObject

private val DIAS_G = listOf(1 to "L", 2 to "M", 3 to "X", 4 to "J", 5 to "V", 6 to "S", 7 to "D")

private fun horarioDe(json: String): HorarioDef = runCatching {
    val o = JSONObject(json)
    val a = o.optJSONArray("dias")
    HorarioDef(List(a?.length() ?: 0) { a!!.getInt(it) }, o.optString("desde"), o.optString("hasta"))
}.getOrDefault(HorarioDef())

private fun horarioJson(h: HorarioDef): String =
    if (h.dias.isEmpty() && h.desde.isEmpty() && h.hasta.isEmpty()) "" else JSONObject().put("dias", JSONArray(h.dias)).put("desde", h.desde).put("hasta", h.hasta).toString()

/** Avanzado → Menús: los valores generales de todos los menús (cada menú puede reemplazarlos en su pestaña Ajustes). */
@Composable
fun PantallaAjustesMenus(botId: Long, alVolver: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    val valores = remember { mutableStateMapOf<String, String>() }
    val originales = remember { mutableStateMapOf<String, String>() }
    var cargado by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    androidx.compose.runtime.LaunchedEffect(botId) {
        runCatching { Repo.ajustesDeMenus(botId) }.onSuccess { valores.putAll(it); originales.putAll(it); cargado = true }.onFailure { avisaToast(ctx, it.message ?: "No se pudo abrir", true); alVolver() }
    }
    val hayCambios = valores.any { (k, v) -> originales[k] != v }
    fun guarda() {
        scope.launch {
            error = null
            var ok = true
            for ((k, v) in valores.toMap()) {
                if (originales[k] == v) continue
                runCatching { Repo.fijarAjusteDeMenus(botId, k, v) }.onSuccess { originales[k] = v }.onFailure { error = it.message ?: "No se pudo guardar"; ok = false }
                if (!ok) break
            }
            if (ok) avisaToast(ctx, "Guardado", false)
        }
    }
    fun v(k: String) = valores[k].orEmpty()
    @Composable
    fun Texto(k: String, etiqueta: String, lineas: Int = 3) {
        OutlinedTextField(v(k), { valores[k] = it }, Modifier.fillMaxWidth().testTag("am_$k"), label = { Text(etiqueta) }, maxLines = lineas)
    }
    @Composable
    fun Numero(k: String, etiqueta: String) {
        OutlinedTextField(v(k), { valores[k] = it.filter(Char::isDigit).take(4) }, Modifier.fillMaxWidth().testTag("am_$k"), label = { Text(etiqueta) }, singleLine = true, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number))
    }
    @Composable
    fun Conmutador(k: String, titulo: String, ayuda: String) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) { Text(titulo); Text(ayuda, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
            Switch(v(k) == "1", { haptica(Toque.Suave); valores[k] = if (it) "1" else "0" }, Modifier.testTag("am_$k"))
        }
    }
    Scaffold(
        topBar = {
            TopAppBar(
                navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } },
                title = { Text("Menús") },
                actions = { TextButton({ guarda() }, Modifier.testTag("guardar_ajustes_menus"), enabled = hayCambios) { Text("Guardar") } },
            )
        },
    ) { relleno ->
        if (!cargado) { Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }; return@Scaffold }
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Text("Valores generales de todos los menús. Cada menú puede cambiarlos en su propia pestaña “Ajustes”. El bot solo responde: nunca escribe primero.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            error?.let { Text(it, color = MaterialTheme.colorScheme.error, modifier = Modifier.testTag("error_ajustes_menus")) }

            Text("Cuando no entiende", style = MaterialTheme.typography.titleMedium)
            val errMax = v("menu_err_max").toIntOrNull() ?: 3
            Column {
                Text("Errores seguidos antes de parar: $errMax")
                Slider(errMax.toFloat(), { valores["menu_err_max"] = it.toInt().coerceIn(1, 10).toString() }, valueRange = 1f..10f, steps = 8, modifier = Modifier.testTag("am_err_max"))
            }
            Texto("menu_txt_n1", "1.er error: reconoce y ofrece las opciones")
            Texto("menu_txt_n2", "2.º error: ofrece hablar con una persona")
            Texto("menu_txt_n3", "Al parar (vacío = no dice nada)")
            Texto("menu_txt_notexto", "Si mandan foto, audio, sticker o archivo")
            Texto("menu_txt_ambiguo", "Cuando varias opciones se parecen")
            Texto("menu_saludos", "Saludos y palabras que NO cuentan como error (separadas por coma)", 5)
            Text("Los emojis y signos sueltos tampoco cuentan.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)

            Text("Comandos dentro de un menú", style = MaterialTheme.typography.titleMedium)
            Texto("menu_txt_salir", "Al escribir “salir”")
            Texto("menu_txt_cancelar", "Al cancelar un formulario")

            Text("Al volver tras un rato", style = MaterialTheme.typography.titleMedium)
            Text("La conversación caduca en silencio (sin mensajes de cierre). El tiempo en privado se elige en Avanzado → Funciones (“Un menú caduca tras”).", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            val volver = listOf("reiniciar" to "Reiniciar", "retomar" to "Retomar")
            SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().testTag("am_volver")) {
                volver.forEachIndexed { i, (k, t) -> SegmentedButton(v("menu_volver") == k, { valores["menu_volver"] = k }, SegmentedButtonDefaults.itemShape(i, volver.size), modifier = Modifier.testTag("am_volver_$k")) { Text(t) } }
            }
            Numero("menu_retoma_horas", "Horas durante las que se puede retomar")

            Text("Hablar con una persona", style = MaterialTheme.typography.titleMedium)
            Texto("menu_humano_dentro", "Mensaje si estás en horario")
            Texto("menu_humano_fuera", "Mensaje si estás fuera de horario ({horario})")
            Text("Horario de atención general (si el menú no trae el suyo; sin elegir nada = siempre)", style = MaterialTheme.typography.labelLarge)
            val h = horarioDe(v("menu_horario"))
            Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                DIAS_G.forEach { (n, l) -> FilterChip(n in h.dias, { valores["menu_horario"] = horarioJson(h.copy(dias = if (n in h.dias) h.dias - n else (h.dias + n).sorted())) }, label = { Text(l) }, modifier = Modifier.testTag("am_dia_$n")) }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                OutlinedTextField(h.desde, { valores["menu_horario"] = horarioJson(h.copy(desde = it.take(5))) }, Modifier.weight(1f).testTag("am_horario_desde"), label = { Text("Desde (HH:MM)") }, singleLine = true)
                OutlinedTextField(h.hasta, { valores["menu_horario"] = horarioJson(h.copy(hasta = it.take(5))) }, Modifier.weight(1f).testTag("am_horario_hasta"), label = { Text("Hasta (HH:MM)") }, singleLine = true)
            }

            Text("Formularios", style = MaterialTheme.typography.titleMedium)
            OutlinedTextField(v("menu_region"), { valores["menu_region"] = it.uppercase().take(2) }, Modifier.fillMaxWidth().testTag("am_menu_region"), label = { Text("País para los teléfonos sin código (2 letras, por ejemplo MX)") }, singleLine = true)

            Text("Lo que no entendió", style = MaterialTheme.typography.titleMedium)
            Conmutador("noentendio_activo", "Guardar lo que no entiende", "Se guarda cifrado y sin el nombre de quien lo escribió.")
            Numero("noentendio_dias", "Se borra solo a los (días; 0 = nunca)")

            Text("Menús en grupos", style = MaterialTheme.typography.titleMedium)
            Text("En un grupo cada persona tiene su propio menú. Empieza con una mención, un comando (por ejemplo !menu) o citando un mensaje del bot.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Numero("grupo_max_sesiones", "Personas usando el menú a la vez en un grupo")
            Numero("grupo_expira_min", "Caducidad en silencio dentro de un grupo (minutos)")
            Numero("grupo_msgs_min", "Mensajes del bot por minuto en un mismo grupo")
            Numero("grupo_cola_min", "Minutos que espera alguien en la fila antes de descartarse")
            Conmutador("grupo_cualquiera", "Cualquiera puede contestar el menú de otra persona", "Apagado: si alguien cita el menú de otra persona, se ignora.")
            Texto("grupo_txt_turno", "A quien espera lugar ({n} = su turno)")
            Texto("grupo_txt_vencido", "Al citar un menú que ya terminó ({prefijo} = el prefijo de comandos)")
            Texto("grupo_txt_privado", "Cuando eligen una opción solo para privado")
            Spacer(Modifier.size(80.dp))
        }
    }
}
