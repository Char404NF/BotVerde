@file:OptIn(ExperimentalMaterial3Api::class, androidx.compose.foundation.layout.ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.AssistChip
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
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
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.ChatItem
import com.botchat.app.datos.EstadoCaptura
import com.botchat.app.datos.EstadoLab
import com.botchat.app.datos.PruebaLab
import com.botchat.app.datos.Repo
import com.botchat.app.datos.ResultadoLab
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/** Segundos de espera visible antes de poder aceptar el aviso del Laboratorio. */
const val ESPERA_AVISO_LAB_SEG = 5

/** Nombre corto de un tipo de prueba, en palabras simples. */
internal fun nombreTipoLab(tipo: String) = when (tipo) {
    "encuesta" -> "Encuesta"
    "botones" -> "Botones"
    "lista" -> "Lista"
    "interactivo" -> "Interactivo"
    "interactivo_biz" -> "Interactivo + biz"
    else -> tipo
}

internal fun nombreModoLab(m: String) = when (m) {
    "encuesta" -> "Encuesta"
    "botones" -> "Botones"
    else -> "Texto"
}

/** Si / No / ? para un valor de la matriz (-1 sin saber, 0 no, 1 sí). */
internal fun siNoLab(v: Int) = when (v) { 1 -> "Sí"; 0 -> "No"; else -> "?" }

/** El siguiente valor al tocar una celda que marcas tú: ? → Sí → No → ?. */
internal fun siguienteLab(v: Int) = when (v) { -1 -> 1; 1 -> 0; else -> -1 }

// ---------- Avanzado → Laboratorio ----------

/**
 * Avanzado → Laboratorio (apagado por defecto). Prueba formas nuevas de mostrar el menú: como encuesta o con botones. Siempre hay respaldo
 * en texto numerado. Para encenderlo hay que aceptar un aviso de riesgo.
 */
@Composable
fun PantallaLaboratorio(botId: Long, alVolver: () -> Unit, alAviso: () -> Unit, alMatriz: () -> Unit, alCaptura: () -> Unit = {}) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("lab" in evs || "permisos_ajustes" in evs) version++ } }
    val estado by produceState<EstadoLab?>(null, version) { value = runCatching { Repo.estadoLab(botId) }.getOrNull() ?: value }
    var elegirChat by remember { mutableStateOf(false) }
    val nombreChat by produceState("", estado?.pruebaChat) {
        val j = estado?.pruebaChat.orEmpty()
        value = if (j.isBlank()) "" else runCatching { (Repo.chats(botId, false, "", 400) + Repo.chats(botId, true, "", 400)).firstOrNull { it.jid == j }?.nombre }.getOrNull().orEmpty().ifBlank { j }
    }

    fun fijar(clave: String, valor: String) = scope.launch {
        runCatching { Repo.fijarAjusteLab(botId, clave, valor) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo guardar", true) }
        version++
    }

    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Laboratorio") }) }) { relleno ->
        val e = estado
        if (e == null) { Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }; return@Scaffold }
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Text(
                "Aquí se prueban formas nuevas de mostrar el menú: como encuesta o con botones. Es experimental. Si algo falla o no se ve, el menú sale como siempre, en texto con números.",
                style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Card(Modifier.fillMaxWidth().testTag("tarjeta_lab"), colors = CardDefaults.cardColors(containerColor = if (e.activo) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surfaceContainer)) {
                Row(Modifier.padding(Esp.l), verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text("Laboratorio", style = MaterialTheme.typography.titleMedium)
                        Text(
                            if (e.activo) "Encendido. El menú sale en modo ${nombreModoLab(e.modoReal)}." else "Apagado. Todos los menús salen en texto con números.",
                            Modifier.testTag("lab_estado_texto"), style = MaterialTheme.typography.bodySmall,
                        )
                    }
                    Switch(e.activo, {
                        haptica(Toque.Fuerte)
                        if (!it) fijar("lab_activo", "0") else if (!e.avisoAceptado) alAviso() else fijar("lab_activo", "1")
                    }, Modifier.testTag("lab_activo"))
                }
            }
            Card(Modifier.fillMaxWidth().testTag("tarjeta_captura"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                Row(Modifier.padding(Esp.l), verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text("Captura de mensajes (diagnóstico)", style = MaterialTheme.typography.titleMedium)
                        Text("Muestra con qué forma llegan los mensajes. Solo estructura, en memoria.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    OutlinedButton(alCaptura, Modifier.testTag("lab_abrir_captura")) { Text("Abrir") }
                }
            }
            if (!e.activo) {
                Text(
                    "Al encenderlo verás un aviso de riesgo. WhatsApp no ofrece botones ni listas a las cuentas normales: pueden no verse. No se simula ser una cuenta Business ni se cambia nada de tu cuenta.",
                    Modifier.testTag("lab_apagado_aviso"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                return@Column
            }
            // Modo del menú
            Card(Modifier.fillMaxWidth().testTag("tarjeta_modo"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text("Modo del menú", style = MaterialTheme.typography.titleMedium)
                    val modos = listOf("texto" to "Texto", "encuesta" to "Encuesta", "botones" to "Botones")
                    SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().testTag("lab_modo")) {
                        modos.forEachIndexed { i, (m, t) ->
                            SegmentedButton(e.modo == m, { haptica(Toque.Suave); fijar("lab_modo_menu", m) }, SegmentedButtonDefaults.itemShape(i, modos.size), modifier = Modifier.testTag("lab_modo_$m")) { Text(t) }
                        }
                    }
                    Text(
                        when (e.modo) {
                            "encuesta" -> "El menú sale como una encuesta de WhatsApp: la persona toca una opción y el menú sigue como si hubiera escrito el número. Funciona mejor en privado."
                            "botones" -> "Experimental. El menú sale con botones o lista (los que la librería manda por defecto). Pueden no verse; por eso, debajo va también el texto con números."
                            else -> "Predeterminado. El menú sale como texto con números: la persona escribe 1, 2, 3…"
                        },
                        Modifier.testTag("lab_modo_texto"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    Text("Siempre cae a texto con números si el envío falla (o tras 3 fallos seguidos, vuelve solo a Texto).", style = MaterialTheme.typography.bodySmall)
                }
            }
            // Encuestas
            Card(Modifier.fillMaxWidth().testTag("tarjeta_encuestas"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text("Encuestas", style = MaterialTheme.typography.titleMedium)
                    FilaSwitchLab("También en grupos", "En un grupo, todos ven los votos de todos. Por defecto solo se usan en privado.", e.grupos, "lab_grupos") { fijar("lab_grupos", if (it) "1" else "0") }
                    if (e.grupos) Text("Aviso: en los grupos todos los miembros verán lo que vota cada persona.", Modifier.testTag("lab_grupos_aviso"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
                    CampoNumeroLab("Opciones máximas por encuesta", e.encMax, 2, 32, "lab_enc_max", "Si el menú tiene más, sale en texto. WhatsApp no avisa el límite: se mide en las pruebas.") { fijar("lab_enc_max_opc", it.toString()) }
                    CampoNumeroLab("Caduca a los (minutos)", e.expiraMin, 1, 1440, "lab_expira", "Pasado este tiempo, votar ya no mueve el menú.") { fijar("lab_expira_min", it.toString()) }
                    FilaSwitchLab("Mandar también el texto con números", "Debajo de la encuesta, por si no se ve.", e.respaldoEnc, "lab_respaldo_enc") { fijar("lab_respaldo_enc", if (it) "1" else "0") }
                }
            }
            // Botones y listas
            Card(Modifier.fillMaxWidth().testTag("tarjeta_botones"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text("Botones y listas (experimental)", style = MaterialTheme.typography.titleMedium)
                    Text("Solo se usa lo que la librería manda por defecto: botones, lista y mensaje interactivo con respuesta rápida. Nada de Business ni nodos inventados.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    val tipos = listOf("auto" to "Auto", "botones" to "Botones", "lista" to "Lista", "interactivo" to "Interact.")
                    SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().testTag("lab_botones_tipo")) {
                        tipos.forEachIndexed { i, (m, t) ->
                            SegmentedButton(e.botonesTipo == m, { haptica(Toque.Suave); fijar("lab_botones_tipo", m) }, SegmentedButtonDefaults.itemShape(i, tipos.size), modifier = Modifier.testTag("lab_tipo_$m")) { Text(t, maxLines = 1) }
                        }
                    }
                    Text("Auto: hasta 3 opciones, botones; hasta 10, lista; más, texto.", style = MaterialTheme.typography.bodySmall)
                    FilaSwitchLab("Mandar también el texto con números", "Recomendado: si los botones no se ven, la persona puede contestar con el número.", e.respaldoBot, "lab_respaldo_bot") { fijar("lab_respaldo_bot", if (it) "1" else "0") }
                }
            }
            // Prueba experimental: Interactivo + biz
            Card(Modifier.fillMaxWidth().testTag("tarjeta_biz"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text("Interactivo + biz (experimental)", style = MaterialTheme.typography.titleMedium)
                    Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.errorContainer), modifier = Modifier.fillMaxWidth().testTag("lab_biz_aviso")) {
                        Column(Modifier.padding(Esp.m), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                            Text("RIESGO: puede bloquear tu número.", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.onErrorContainer)
                            Text("Esta prueba añade a mano un nodo interno «biz» (el que propone el PR #1235 de whatsmeow) para ver si los botones aparecen. WhatsApp no lo espera de una cuenta que no sea oficial de empresa y puede limitar o bloquear el número. Solo se manda al chat de prueba, nunca a clientes ni en el menú real. Usa un número secundario.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onErrorContainer)
                        }
                    }
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Checkbox(e.bizAck, { fijar("lab_biz_ack", if (it) "1" else "0") }, Modifier.testTag("lab_biz_ack"))
                        Text("Entiendo que puede bloquear mi número", style = MaterialTheme.typography.bodyMedium, modifier = Modifier.weight(1f))
                    }
                    FilaSwitchLab("Activar la prueba", if (e.bizAck) "Apagada por defecto. Aparece en la matriz de pruebas." else "Marca primero la casilla.", e.bizActivo && e.bizAck, "lab_biz_activo", habilitado = e.bizAck) { fijar("lab_biz_activo", if (it) "1" else "0") }
                }
            }
            // Pruebas
            Card(Modifier.fillMaxWidth().testTag("tarjeta_pruebas"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text("Pruebas", style = MaterialTheme.typography.titleMedium)
                    Text("Las pruebas se mandan SOLO a tu otro número o a un grupo de prueba tuyo, con un tope de envíos. Nunca a clientes.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) {
                            Text("Chat de prueba", style = MaterialTheme.typography.bodyLarge)
                            Text(if (e.pruebaChat.isBlank()) "Sin elegir" else nombreChat, Modifier.testTag("lab_chat_nombre"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        }
                        OutlinedButton({ elegirChat = true }, Modifier.testTag("lab_elegir_chat")) { Text("Elegir") }
                    }
                    CampoTextoLab("Versión de WhatsApp del teléfono que recibe", e.versionWa, "lab_version_wa", "Por ejemplo 2.26.5.1 (Ajustes de WhatsApp → Ayuda → Información de la app). Se anota en cada prueba.") { fijar("lab_version_wa", it) }
                    CampoNumeroLab("Pruebas por hora (tope)", e.pruebaHora, 1, 100, "lab_prueba_hora", "Llevas ${e.pruebasHora} en la última hora.") { fijar("lab_prueba_hora", it.toString()) }
                    CampoNumeroLab("Pruebas por día (tope)", e.pruebaDia, 1, 100, "lab_prueba_dia", "Llevas ${e.pruebasDia} en las últimas 24 horas.") { fijar("lab_prueba_dia", it.toString()) }
                    Button(alMatriz, Modifier.fillMaxWidth().testTag("lab_abrir_matriz")) { Text("Abrir la matriz de pruebas") }
                }
            }
        }
    }
    if (elegirChat) SelectorChatPrueba(botId, alElegir = { j -> elegirChat = false; fijar("lab_prueba_chat", j) }, alCerrar = { elegirChat = false })
}

@Composable
private fun FilaSwitchLab(titulo: String, ayuda: String, valor: Boolean, tag: String, habilitado: Boolean = true, alCambiar: (Boolean) -> Unit) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        Column(Modifier.weight(1f)) {
            Text(titulo, style = MaterialTheme.typography.bodyLarge)
            Text(ayuda, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        Switch(valor, alCambiar, Modifier.testTag(tag), enabled = habilitado)
    }
}

/** Un número editable que se guarda con el botón «Guardar» (solo aparece si cambió). */
@Composable
private fun CampoNumeroLab(titulo: String, valor: Int, min: Int, max: Int, tag: String, ayuda: String, alGuardar: (Int) -> Unit) {
    var texto by remember(valor) { mutableStateOf(valor.toString()) }
    val n = texto.toIntOrNull()
    val valido = n != null && n in min..max
    Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            OutlinedTextField(texto, { texto = it.filter(Char::isDigit).take(4) }, Modifier.weight(1f).testTag(tag), label = { Text(titulo) }, singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number), isError = !valido)
            if (texto != valor.toString() && valido) TextButton({ alGuardar(n) }, Modifier.testTag(tag + "_guardar")) { Text("Guardar") }
        }
        Text(if (valido) ayuda else "Escribe un número de $min a $max.", style = MaterialTheme.typography.bodySmall, color = if (valido) MaterialTheme.colorScheme.onSurfaceVariant else MaterialTheme.colorScheme.error)
    }
}

@Composable
private fun CampoTextoLab(titulo: String, valor: String, tag: String, ayuda: String, alGuardar: (String) -> Unit) {
    var texto by remember(valor) { mutableStateOf(valor) }
    Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            OutlinedTextField(texto, { texto = it.take(30) }, Modifier.weight(1f).testTag(tag), label = { Text(titulo) }, singleLine = true)
            if (texto.trim() != valor) TextButton({ alGuardar(texto.trim()) }, Modifier.testTag(tag + "_guardar")) { Text("Guardar") }
        }
        Text(ayuda, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

@Composable
private fun SelectorChatPrueba(botId: Long, alElegir: (String) -> Unit, alCerrar: () -> Unit) {
    // Sin cuadro de búsqueda: la lista trae los chats más recientes (personas y grupos) y se desplaza.
    val chats by produceState<List<ChatItem>?>(null) {
        value = runCatching { Repo.chats(botId, false, "", 300) + Repo.chats(botId, true, "", 300) }.getOrDefault(emptyList())
    }
    AlertDialog(
        onDismissRequest = alCerrar,
        title = { Text("Chat de prueba") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("Elige tu otro número o un grupo de prueba tuyo. El chat debe existir ya: escríbele algo desde el teléfono si no aparece.", style = MaterialTheme.typography.bodySmall)
                val l = chats
                when {
                    l == null -> Text("Cargando…", style = MaterialTheme.typography.bodyMedium)
                    l.isEmpty() -> Text("No hay chats.", style = MaterialTheme.typography.bodyMedium)
                    else -> Column(Modifier.fillMaxWidth().heightIn(max = 320.dp).verticalScroll(rememberScrollState()).padding(top = Esp.xs).testTag("lab_lista_chats")) {
                        l.forEach { c ->
                            Row(Modifier.fillMaxWidth().clickable { alElegir(c.jid) }.padding(vertical = Esp.s).testTag("lab_chat_" + c.jid), verticalAlignment = Alignment.CenterVertically) {
                                Text(c.nombre.ifBlank { c.telefono.ifBlank { c.jid } } + if (c.esGrupo) "  (grupo)" else "", maxLines = 1, overflow = TextOverflow.Ellipsis)
                            }
                        }
                    }
                }
            }
        },
        confirmButton = { TextButton(alCerrar) { Text("Cancelar") } },
    )
}

// ---------- aviso de riesgo del Laboratorio ----------

/** Aviso antes de encender el Laboratorio: espera de 5 segundos visible y una casilla obligatoria. Cancelar no enciende nada. */
@Composable
fun PantallaAvisoLab(botId: Long, alVolver: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var segundos by remember { mutableIntStateOf(ESPERA_AVISO_LAB_SEG) }
    var casilla by remember { mutableStateOf(false) }
    var trabajando by remember { mutableStateOf(false) }
    LaunchedEffect(Unit) { while (segundos > 0) { delay(1000); segundos-- } }
    val puede = !trabajando && segundos == 0 && casilla
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Antes de encender") }) }) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Text("Laboratorio", style = MaterialTheme.typography.titleLarge, modifier = Modifier.testTag("aviso_lab_titulo"))
            Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.errorContainer), modifier = Modifier.fillMaxWidth().testTag("aviso_lab_texto")) {
                Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text("Esto es experimental y puede hacer que WhatsApp limite tu número.", style = MaterialTheme.typography.titleMedium)
                    Text("Las encuestas son parte normal de WhatsApp. Los botones y las listas NO: WhatsApp no los ofrece a las cuentas normales, así que pueden no verse (la persona ve «mensaje no compatible» o nada).", style = MaterialTheme.typography.bodyMedium)
                    Text("No se simula ser una cuenta Business ni se cambia el tipo de dispositivo: solo se manda lo que la librería envía por defecto. Aun así, WhatsApp puede reaccionar mal. Usa un número secundario.", style = MaterialTheme.typography.bodyMedium)
                    Text("Si algo falla, el menú sale en texto con números. Las pruebas van solo a tu otro número o a un grupo tuyo, con tope de envíos. Puedes apagarlo cuando quieras.", style = MaterialTheme.typography.bodyMedium)
                }
            }
            Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.testTag("aviso_lab_fila_casilla")) {
                Checkbox(casilla, { casilla = it }, Modifier.testTag("aviso_lab_casilla"))
                Text("Entiendo que mi número puede ser limitado o bloqueado por WhatsApp", style = MaterialTheme.typography.bodyMedium, modifier = Modifier.weight(1f))
            }
            if (segundos > 0) Text("Espera $segundos ${if (segundos == 1) "segundo" else "segundos"} para poder continuar…", Modifier.testTag("aviso_lab_espera"), style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Button(
                {
                    trabajando = true
                    scope.launch {
                        runCatching { Repo.aceptarAvisoLab(botId); Repo.fijarAjusteLab(botId, "lab_activo", "1") }
                            .onSuccess { alVolver() }
                            .onFailure { trabajando = false; avisaToast(ctx, it.message ?: "No se pudo encender", true) }
                    }
                },
                Modifier.fillMaxWidth().testTag("aviso_lab_entiendo"), enabled = puede,
            ) { Text("Entiendo el riesgo") }
            OutlinedButton(alVolver, Modifier.fillMaxWidth().testTag("aviso_lab_cancelar")) { Text("Cancelar") }
        }
    }
}

// ---------- matriz de pruebas ----------

/**
 * La matriz: manda una prueba a tu chat de prueba y mira, en el teléfono que la recibe, si llegó, si se ve y si se puede tocar. «Llegó» y
 * «Respondió» los anota el bot solo; «Se ve» y «Se toca» los marcas tú tocando la celda (? → Sí → No).
 */
@Composable
fun PantallaMatrizLab(botId: Long, alVolver: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("lab" in evs || "permisos_ajustes" in evs || evs.any { it.startsWith("msg:") }) version++ } }
    val estado by produceState<EstadoLab?>(null, version) { value = runCatching { Repo.estadoLab(botId) }.getOrNull() ?: value }
    val filas by produceState<List<ResultadoLab>?>(null, version) { value = runCatching { Repo.resultadosLab(botId) }.getOrDefault(emptyList()) }
    var enviando by remember { mutableStateOf<String?>(null) }
    var confirmaBorrar by remember { mutableStateOf(false) }

    fun mandar(p: PruebaLab) {
        enviando = p.tipo + p.variante
        scope.launch {
            runCatching { Repo.enviarPruebaLab(botId, p.tipo, p.variante) }
                .onSuccess { if (it.enviado != 1) avisaToast(ctx, "No se pudo enviar: ${it.error.ifBlank { "WhatsApp la rechazó" }}", true) }
                .onFailure { avisaToast(ctx, it.message ?: "No se pudo enviar", true) }
            enviando = null
            version++
        }
    }

    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Matriz de pruebas") }) }) { relleno ->
        val e = estado
        val l = filas
        if (e == null || l == null) { Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }; return@Scaffold }
        val listo = e.activo && e.avisoAceptado && e.pruebaChat.isNotBlank() && e.enMarcha
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Text(
                "Manda una prueba a tu chat de prueba y mira en ese teléfono si llegó, si se ve y si se puede tocar. Toca las celdas «Se ve» y «Se toca» para marcarlas. «Llegó» y «Respondió» los anota el bot solo.",
                style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            if (!listo) {
                Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.errorContainer), modifier = Modifier.fillMaxWidth().testTag("matriz_no_listo")) {
                    Text(
                        when {
                            !e.activo || !e.avisoAceptado -> "Enciende el Laboratorio primero (Avanzado → Laboratorio)."
                            e.pruebaChat.isBlank() -> "Elige el chat de prueba en Avanzado → Laboratorio → Pruebas."
                            else -> "Enciende el bot para mandar pruebas."
                        },
                        Modifier.padding(Esp.l), style = MaterialTheme.typography.bodyMedium,
                    )
                }
            }
            Text("Mandar una prueba", style = MaterialTheme.typography.titleMedium)
            Text("Llevas ${e.pruebasHora} de ${e.pruebaHora} en esta hora y ${e.pruebasDia} de ${e.pruebaDia} hoy.", Modifier.testTag("matriz_topes"), style = MaterialTheme.typography.bodySmall)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                e.plan.forEach { p ->
                    AssistChip({ haptica(Toque.Suave); mandar(p) }, { Text(p.nombre) }, Modifier.testTag("prueba_${p.tipo}_${p.variante}"), enabled = listo && enviando == null)
                }
            }
            if (enviando != null) Row(verticalAlignment = Alignment.CenterVertically) { CircularProgressIndicator(Modifier.padding(end = Esp.s)); Text("Enviando…") }

            Text("Matriz", style = MaterialTheme.typography.titleMedium)
            if (l.isEmpty()) Text("Todavía no hay pruebas.", Modifier.testTag("matriz_vacia"), style = MaterialTheme.typography.bodyMedium)
            else {
                EncabezadoMatriz()
                l.forEach { r ->
                    FilaMatriz(r, alMarcar = { campo, valor ->
                        scope.launch {
                            runCatching { Repo.marcarResultadoLab(botId, r.id, campo, valor) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo marcar", true) }
                            version++
                        }
                    })
                }
            }
            // Límite medido
            Card(Modifier.fillMaxWidth().testTag("tarjeta_limite"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text("Límite de opciones de una encuesta", style = MaterialTheme.typography.titleMedium)
                    Text(
                        if (e.limiteMedido >= 2) "Según tus pruebas, la encuesta más grande que se vio y se pudo tocar tiene ${e.limiteMedido} opciones. Ahora el tope es ${e.encMax}."
                        else "Todavía no hay datos. Manda encuestas de 4, 8, 12, 13… opciones y marca cuáles se ven en el teléfono. WhatsApp no avisa su límite.",
                        Modifier.testTag("limite_texto"), style = MaterialTheme.typography.bodyMedium,
                    )
                    Button({
                        scope.launch {
                            runCatching { Repo.usarLimiteMedido(botId) }
                                .onSuccess { avisaToast(ctx, "Tope de opciones: $it") }
                                .onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
                            version++
                        }
                    }, Modifier.fillMaxWidth().testTag("usar_limite"), enabled = e.limiteMedido >= 2 && e.limiteMedido != e.encMax) { Text("Usar ${if (e.limiteMedido >= 2) e.limiteMedido else "…"} como tope") }
                }
            }
            if (l.isNotEmpty()) OutlinedButton({ confirmaBorrar = true }, Modifier.fillMaxWidth().testTag("borrar_matriz")) { Text("Borrar la matriz") }
        }
    }
    if (confirmaBorrar) AlertDialog(
        onDismissRequest = { confirmaBorrar = false },
        title = { Text("¿Borrar la matriz?") },
        text = { Text("Se borran todas las pruebas anotadas. No afecta a los menús.") },
        confirmButton = { TextButton({ confirmaBorrar = false; scope.launch { runCatching { Repo.borrarResultadosLab(botId) }; version++ } }, Modifier.testTag("borrar_matriz_si")) { Text("Borrar") } },
        dismissButton = { TextButton({ confirmaBorrar = false }) { Text("Cancelar") } },
    )
}

private val ANCHO_ETIQUETA = 1.4f

@Composable
private fun EncabezadoMatriz() {
    Row(Modifier.fillMaxWidth().testTag("matriz_encabezado"), verticalAlignment = Alignment.CenterVertically) {
        Text("Prueba", Modifier.weight(ANCHO_ETIQUETA), style = MaterialTheme.typography.labelMedium)
        listOf("Llegó", "Se ve", "Se toca", "Respondió").forEach { Text(it, Modifier.weight(1f), style = MaterialTheme.typography.labelMedium, textAlign = TextAlign.Center, maxLines = 1) }
    }
}

@Composable
private fun FilaMatriz(r: ResultadoLab, alMarcar: (String, Int) -> Unit) {
    Column(Modifier.fillMaxWidth().testTag("fila_lab_${r.id}"), verticalArrangement = Arrangement.spacedBy(2.dp)) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Text("${nombreTipoLab(r.tipo)} ${r.variante}", Modifier.weight(ANCHO_ETIQUETA), style = MaterialTheme.typography.bodyMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Celda(if (r.enviado == 0) "Error" else siNoLab(r.llego), r.enviado == 0, null, "celda_llego_${r.id}")
            Celda(siNoLab(r.visible), false, { alMarcar("visible", siguienteLab(r.visible)) }, "celda_visible_${r.id}")
            Celda(siNoLab(r.tocable), false, { alMarcar("tocable", siguienteLab(r.tocable)) }, "celda_tocable_${r.id}")
            Celda(siNoLab(r.respondio), false, null, "celda_respondio_${r.id}")
        }
        val versiones = listOfNotNull(r.versionWa.takeIf { it.isNotBlank() }?.let { "WhatsApp $it" }, r.versionCliente.takeIf { it.isNotBlank() }?.let { "conexión $it" }).joinToString(" · ")
        if (versiones.isNotEmpty()) Text(versiones, Modifier.testTag("version_lab_${r.id}"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        if (r.error.isNotBlank()) Text(r.error, Modifier.testTag("error_lab_${r.id}"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
    }
}

@Composable
private fun androidx.compose.foundation.layout.RowScope.Celda(texto: String, malo: Boolean, alTocar: (() -> Unit)?, tag: String) {
    val color = when {
        malo || texto == "No" -> MaterialTheme.colorScheme.error
        texto == "Sí" -> MaterialTheme.colorScheme.primary
        else -> MaterialTheme.colorScheme.onSurfaceVariant
    }
    Box(Modifier.weight(1f).padding(2.dp).then(if (alTocar != null) Modifier.clickable { alTocar() } else Modifier).testTag(tag), Alignment.Center) {
        Text(texto, style = if (alTocar != null) MaterialTheme.typography.titleSmall else MaterialTheme.typography.bodyMedium, color = color, textAlign = TextAlign.Center)
    }
}

// ---------- Captura de mensajes (diagnóstico) ----------

private fun horaCaptura(ts: Long) = java.text.SimpleDateFormat("HH:mm:ss", java.util.Locale.getDefault()).format(java.util.Date(ts * 1000))

/**
 * Diagnóstico: guarda, SOLO en la memoria del teléfono, la estructura (nombres de campos y largos) de los últimos mensajes recibidos, para ver
 * con qué forma llega algo raro. Apagada por defecto; se apaga sola a los 10 minutos o a los 30 mensajes. Nada va a disco ni a la base.
 */
@Composable
fun PantallaCapturaLab(botId: Long, alVolver: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    val estado by produceState<EstadoCaptura?>(null, version) { value = runCatching { Repo.estadoCaptura(botId) }.getOrNull() ?: value }
    val activa = estado?.activo == true
    // Mientras está encendida se refresca sola (llegan mensajes y corre el reloj).
    LaunchedEffect(activa) { while (activa) { delay(2000); version++ } }

    fun accion(f: suspend () -> Unit) = scope.launch {
        runCatching { f() }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
        version++
    }

    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Captura de mensajes") }) }) { relleno ->
        val e = estado
        if (e == null) { Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }; return@Scaffold }
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Text(
                "Solo se guarda la estructura del mensaje (qué campos trae y cuánto miden), en la memoria del teléfono: nada va a disco, a la base ni al registro. Sin bytes, claves ni números completos. Se apaga sola a los 10 minutos o a los 30 mensajes.",
                Modifier.testTag("captura_aviso"), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Card(Modifier.fillMaxWidth().testTag("tarjeta_captura_estado"), colors = CardDefaults.cardColors(containerColor = if (e.activo) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surfaceContainer)) {
                Row(Modifier.padding(Esp.l), verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text("Capturar mensajes que lleguen", style = MaterialTheme.typography.titleMedium)
                        Text(
                            if (e.activo) "Encendida. Quedan ${e.restanSeg / 60}:${"%02d".format(e.restanSeg % 60)} · ${e.capturados} de ${e.max} mensajes." else "Apagada.",
                            Modifier.testTag("captura_estado_texto"), style = MaterialTheme.typography.bodySmall,
                        )
                    }
                    Switch(e.activo, { haptica(Toque.Fuerte); accion { Repo.activarCaptura(botId, it) } }, Modifier.testTag("captura_activo"))
                }
            }
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text("Mostrar textos", style = MaterialTheme.typography.bodyLarge)
                    Text("Apagado: los textos solo por su largo. Encendido: los de hasta 60 letras (con los números largos tapados como ***1234). Los ids de botones siempre se ven.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                Switch(e.mostrarTextos, { accion { Repo.mostrarTextosCaptura(botId, it) } }, Modifier.testTag("captura_textos"))
            }
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                Button({
                    runCatching { ctx.getSystemService(android.content.ClipboardManager::class.java).setPrimaryClip(android.content.ClipData.newPlainText("Captura", e.textoCompleto())) }
                    avisaToast(ctx, "Copiado", false)
                }, Modifier.testTag("captura_copiar"), enabled = e.items.isNotEmpty()) { Text("Copiar todo") }
                OutlinedButton({ accion { Repo.borrarCaptura(botId) } }, Modifier.testTag("captura_borrar"), enabled = e.items.isNotEmpty()) { Text("Borrar") }
            }
            if (e.items.isEmpty()) {
                Text(if (e.activo) "Esperando mensajes…" else "Nada capturado.", Modifier.testTag("captura_vacio"), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            e.items.asReversed().forEach { it ->
                Card(Modifier.fillMaxWidth().testTag("captura_item_${it.n}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                    Column(Modifier.padding(Esp.m), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                        Text("#${it.n} · ${horaCaptura(it.ts)} · ${it.chat} · ${it.clasif}", style = MaterialTheme.typography.titleSmall)
                        if (it.detalle != "-") Text(it.detalle, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        Text(it.arbol, style = MaterialTheme.typography.bodySmall, fontFamily = androidx.compose.ui.text.font.FontFamily.Monospace)
                    }
                }
            }
        }
    }
}
