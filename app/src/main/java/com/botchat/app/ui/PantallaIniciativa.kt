@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.ModoSeguro
import com.botchat.app.datos.NoMolestarItem
import com.botchat.app.datos.RegistroIniciativa
import com.botchat.app.datos.Repo
import kotlinx.coroutines.launch

/** Palabras simples para el resultado de una iniciativa en el registro. */
internal fun textoResultado(r: String): String = when {
    r == "enviado" -> "Se envió"
    r == "fallido" -> "No se pudo enviar"
    r == "diferido" -> "Esperó a que termine la pausa nocturna"
    r == "descartado:maestro" -> "No se envió: Iniciativa propia estaba apagada"
    r == "descartado:modulo" -> "No se envió: el módulo estaba apagado"
    r == "descartado:no_es_iniciativa" -> "No se envió: el módulo no puede escribir solo"
    r == "descartado:seguro" -> "No se envió: modo seguro"
    r == "descartado:no_molestar" -> "No se envió: la persona pidió no recibir mensajes"
    r == "descartado:noche" -> "No se envió: era de noche"
    r == "descartado:tope" -> "No se envió: se llegó al tope de mensajes"
    r == "descartado:sin_conexion" -> "No se envió: no había conexión"
    r == "descartado:cola_llena" -> "No se envió: había demasiadas esperando"
    r == "agregado" -> "Se agregó a No molestar"
    r == "reanudado" -> "Se reanudó"
    r == "errores" -> "Se activó por errores de envío seguidos"
    r.startsWith("desconexion") -> "Se activó porque WhatsApp cortó la conexión"
    else -> r
}

internal fun textoQue(modulo: String, que: String): String = when {
    modulo == "sistema" && que == "modo_seguro" -> "Modo seguro"
    modulo == "sistema" && que == "no_molestar" -> "No molestar"
    else -> "$modulo · $que"
}

// ---------- Ajustes → Iniciativa propia ----------

/**
 * Iniciativa propia: el interruptor general que apaga de golpe todo lo que el bot escribe sin que le hablen, los topes por hora y por día,
 * la pausa nocturna, la lista No molestar, el estado del modo seguro y el registro de iniciativas (sin textos).
 */
@Composable
fun PantallaIniciativa(botId: Long, alVolver: () -> Unit, alRegistro: () -> Unit = {}) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("modulos" in evs || "iniciativas" in evs) version++ } }
    val ajustes by produceState<Map<String, String>?>(null, version) { value = runCatching { Repo.ajustesIniciativa(botId) }.getOrDefault(emptyMap()) }
    val seguro by produceState(ModoSeguro(false, 0, "", "", 0), version) { value = runCatching { Repo.modoSeguro(botId) }.getOrDefault(value) }
    val lista by produceState<List<NoMolestarItem>>(emptyList(), version) { value = runCatching { Repo.noMolestar(botId) }.getOrDefault(emptyList()) }
    val reciente by produceState<List<RegistroIniciativa>>(emptyList(), version) { value = runCatching { Repo.iniciativas(botId, "", 5) }.getOrDefault(emptyList()) }

    fun fijar(clave: String, valor: String, despues: () -> Unit = {}) = scope.launch {
        runCatching { Repo.fijarAjusteIniciativa(botId, clave, valor) }
            .onSuccess { despues() }
            .onFailure { avisaToast(ctx, it.message ?: "No se pudo guardar", true) }
        version++
    }

    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Iniciativa propia") }) }) { relleno ->
        val a = ajustes
        if (a == null) { Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }; return@Scaffold }
        val maestro = a["maestro"] == "1"
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Text(
                "Por defecto el bot solo responde. “Iniciativa propia” es el interruptor general de todo lo que escribe sin que le hablen: apagado, ningún módulo puede enviar nada, aunque esté encendido.",
                style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            // Interruptor general
            Card(Modifier.fillMaxWidth().testTag("tarjeta_maestro"), colors = CardDefaults.cardColors(containerColor = if (maestro) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surfaceContainer)) {
                Row(Modifier.padding(Esp.l), verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text("Iniciativa propia", style = MaterialTheme.typography.titleMedium)
                        Text(if (maestro) "Encendida: los módulos pueden escribir solos." else "Apagada: nada se envía por iniciativa propia.", Modifier.testTag("maestro_texto"), style = MaterialTheme.typography.bodySmall)
                    }
                    Switch(maestro, { haptica(Toque.Fuerte); scope.launch { runCatching { Repo.fijarIniciativaMaestro(botId, it) }.onFailure { e -> avisaToast(ctx, e.message ?: "No se pudo cambiar", true) }; version++ } }, Modifier.testTag("iniciativa_maestro"))
                }
            }
            // Modo seguro
            Card(Modifier.fillMaxWidth().testTag("tarjeta_seguro"), colors = CardDefaults.cardColors(containerColor = if (seguro.activo) MaterialTheme.colorScheme.errorContainer else MaterialTheme.colorScheme.surfaceContainer)) {
                Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text(if (seguro.activo) "Modo seguro activado" else "Modo seguro: todo en orden", style = MaterialTheme.typography.titleMedium)
                    Text(
                        if (seguro.activo) "Los módulos que escriben solos están en pausa (siguen encendidos) porque ${seguro.texto}. Revisa tu conexión y reanúdalos cuando quieras."
                        else "Si fallan varios envíos seguidos o WhatsApp corta la conexión, los módulos que escriben solos se pausan solos y te avisamos.",
                        Modifier.testTag("seguro_texto"), style = MaterialTheme.typography.bodySmall,
                    )
                    if (seguro.activo) Button({ haptica(Toque.Confirmar); scope.launch { runCatching { Repo.reanudarModoSeguro(botId) }.onFailure { e -> avisaToast(ctx, e.message ?: "No se pudo reanudar", true) }; version++ } }, Modifier.fillMaxWidth().testTag("reanudar_seguro")) { Text("Reanudar") }
                }
            }
            // Sin límites (de fábrica encendido): sin topes, sin pausa nocturna y sin esperas
            val sinLimites = a["sin_limites"] != "0"
            Card(Modifier.fillMaxWidth().testTag("tarjeta_sin_limites"), colors = CardDefaults.cardColors(containerColor = if (sinLimites) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surfaceContainer)) {
                Row(Modifier.padding(Esp.l), verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text("Sin límites", style = MaterialTheme.typography.titleMedium)
                        Text(
                            if (sinLimites) "El bot manda cuando quiera: sin topes por hora ni por día, sin pausa nocturna y sin esperas. WhatsApp puede bloquear los números que mandan muchísimo seguido."
                            else "Valen los topes, las esperas y la pausa nocturna de abajo.",
                            Modifier.testTag("sin_limites_texto"), style = MaterialTheme.typography.bodySmall,
                        )
                    }
                    Switch(sinLimites, { haptica(Toque.Fuerte); fijar("sin_limites", if (it) "1" else "0") {} }, Modifier.testTag("sin_limites"))
                }
            }
            if (!sinLimites) {
                // Topes
                Topes(a, ::fijar)
                // Pausa nocturna
                PausaNocturna(a, ::fijar)
            }
            // No molestar
            NoMolestar(lista, alAgregar = { n -> scope.launch { runCatching { Repo.agregarNoMolestar(botId, n) }.onFailure { e -> avisaToast(ctx, e.message ?: "No se pudo agregar", true) }; version++ } },
                alQuitar = { p -> scope.launch { runCatching { Repo.quitarNoMolestar(botId, p) }.onFailure { e -> avisaToast(ctx, e.message ?: "No se pudo quitar", true) }; version++ } })
            // Registro
            Card(Modifier.fillMaxWidth().testTag("tarjeta_registro"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                    Text("Registro de iniciativas", style = MaterialTheme.typography.titleMedium)
                    Text("Lo que los módulos enviaron o dejaron de enviar. Nunca guarda textos de mensajes.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    if (reciente.isEmpty()) Text("Todavía no hay movimientos.", style = MaterialTheme.typography.bodyMedium)
                    reciente.forEach { r -> FilaRegistro(r) }
                    OutlinedButton(alRegistro, Modifier.fillMaxWidth().testTag("ver_registro")) { Text("Ver todo el registro") }
                }
            }
        }
    }
}

@Composable
private fun Topes(a: Map<String, String>, fijar: (String, String, () -> Unit) -> Any) {
    var hora by remember(a["tope_hora"]) { mutableStateOf(a["tope_hora"].orEmpty()) }
    var dia by remember(a["tope_dia"]) { mutableStateOf(a["tope_dia"].orEmpty()) }
    var min by remember(a["retraso_min"]) { mutableStateOf(a["retraso_min"].orEmpty()) }
    var max by remember(a["retraso_max"]) { mutableStateOf(a["retraso_max"].orEmpty()) }
    Card(Modifier.fillMaxWidth().testTag("tarjeta_topes"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Text("Topes", style = MaterialTheme.typography.titleMedium)
            Text("Entre todos los módulos, cuántos mensajes pueden salir por su cuenta. Cada módulo además tiene su propio límite.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                OutlinedTextField(hora, { hora = it.filter(Char::isDigit).take(4) }, Modifier.weight(1f).testTag("tope_hora"), label = { Text("Por hora") }, singleLine = true, keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = KeyboardType.Number))
                OutlinedTextField(dia, { dia = it.filter(Char::isDigit).take(5) }, Modifier.weight(1f).testTag("tope_dia"), label = { Text("Por día") }, singleLine = true, keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = KeyboardType.Number))
            }
            Text("Espera al azar antes de cada mensaje (segundos)", style = MaterialTheme.typography.labelLarge)
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                OutlinedTextField(min, { min = it.filter(Char::isDigit).take(3) }, Modifier.weight(1f).testTag("retraso_min"), label = { Text("Mínimo") }, singleLine = true, keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = KeyboardType.Number))
                OutlinedTextField(max, { max = it.filter(Char::isDigit).take(4) }, Modifier.weight(1f).testTag("retraso_max"), label = { Text("Máximo") }, singleLine = true, keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = KeyboardType.Number))
            }
            Button({
                if (hora != a["tope_hora"]) fijar("tope_hora", hora) {}
                if (dia != a["tope_dia"]) fijar("tope_dia", dia) {}
                if (max != a["retraso_max"]) fijar("retraso_max", max) {}
                if (min != a["retraso_min"]) fijar("retraso_min", min) {}
            }, Modifier.fillMaxWidth().testTag("guardar_topes")) { Text("Guardar topes") }
        }
    }
}

@Composable
private fun PausaNocturna(a: Map<String, String>, fijar: (String, String, () -> Unit) -> Any) {
    val haptica = rememberHaptica()
    var desde by remember(a["noche_desde"]) { mutableStateOf(a["noche_desde"].orEmpty()) }
    var hasta by remember(a["noche_hasta"]) { mutableStateOf(a["noche_hasta"].orEmpty()) }
    val activa = a["noche_on"] == "1"
    Card(Modifier.fillMaxWidth().testTag("tarjeta_noche"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text("Pausa nocturna", style = MaterialTheme.typography.titleMedium)
                    Text("De noche no sale nada por iniciativa propia; lo que cae en esas horas espera a que termine (o se descarta, según el módulo).", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                Switch(activa, { haptica(Toque.Suave); fijar("noche_on", if (it) "1" else "0") {} }, Modifier.testTag("noche_on"))
            }
            if (activa) {
                Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    OutlinedTextField(desde, { desde = it.take(5) }, Modifier.weight(1f).testTag("noche_desde"), label = { Text("Desde") }, placeholder = { Text("23:00") }, singleLine = true)
                    OutlinedTextField(hasta, { hasta = it.take(5) }, Modifier.weight(1f).testTag("noche_hasta"), label = { Text("Hasta") }, placeholder = { Text("07:00") }, singleLine = true)
                }
                Button({
                    if (desde != a["noche_desde"]) fijar("noche_desde", desde) {}
                    if (hasta != a["noche_hasta"]) fijar("noche_hasta", hasta) {}
                }, Modifier.fillMaxWidth().testTag("guardar_noche")) { Text("Guardar horas") }
            }
        }
    }
}

@Composable
private fun NoMolestar(lista: List<NoMolestarItem>, alAgregar: (String) -> Unit, alQuitar: (String) -> Unit) {
    var nuevo by remember { mutableStateOf("") }
    Card(Modifier.fillMaxWidth().testTag("tarjeta_no_molestar"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Text("No molestar", style = MaterialTheme.typography.titleMedium)
            Text("Quien escribe BAJA, STOP o NO MÁS entra aquí solo (y se le avisa una vez). Ningún módulo le escribe a nadie de esta lista.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            if (lista.isEmpty()) Text("Nadie en la lista.", Modifier.testTag("no_molestar_vacio"), style = MaterialTheme.typography.bodyMedium)
            lista.forEach { p ->
                Row(Modifier.fillMaxWidth().testTag("nm_${p.persona}"), verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text(numeroDePersona(p.persona), style = MaterialTheme.typography.bodyLarge, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        Text(etiquetaDia(p.desdeTs) + " · " + if (p.motivo == "baja") "pidió la baja" else p.motivo.ifBlank { "agregado" }, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    TextButton({ alQuitar(p.persona) }, Modifier.testTag("quitar_nm_${p.persona}")) { Text("Quitar") }
                }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalAlignment = Alignment.CenterVertically) {
                OutlinedTextField(nuevo, { nuevo = it.filter { c -> c.isDigit() || c == '+' || c == ' ' }.take(20) }, Modifier.weight(1f).testTag("nm_numero"), label = { Text("Agregar un número") }, placeholder = { Text("5215512345678") }, singleLine = true, keyboardOptions = androidx.compose.foundation.text.KeyboardOptions(keyboardType = KeyboardType.Phone))
                Button({
                    val d = nuevo.filter(Char::isDigit)
                    if (d.length > 6) { alAgregar("$d@s.whatsapp.net"); nuevo = "" }
                }, Modifier.testTag("nm_agregar"), enabled = nuevo.count(Char::isDigit) > 6) { Text("Agregar") }
            }
        }
    }
}

@Composable
private fun FilaRegistro(r: RegistroIniciativa) {
    Column(Modifier.fillMaxWidth().testTag("registro_${r.id}")) {
        Text(textoQue(r.modulo, r.que), style = MaterialTheme.typography.bodyMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
        Text(
            textoResultado(r.resultado) + " · " + etiquetaDia(r.ts) + " " + horaDe(r.ts) + if (r.chat.isNotBlank()) " · " + r.chat else "",
            style = MaterialTheme.typography.bodySmall, color = if (r.resultado == "enviado") MaterialTheme.colorScheme.onSurfaceVariant else MaterialTheme.colorScheme.tertiary,
        )
    }
}

// ---------- registro completo ----------

/** Registro de iniciativas: qué módulo, qué hizo, cuándo y cómo salió. Sin textos de mensajes; el chat aparece con un código corto. */
@Composable
fun PantallaRegistroIniciativas(botId: Long, alVolver: () -> Unit) {
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("iniciativas" in evs) version++ } }
    val lista by produceState<List<RegistroIniciativa>?>(null, version) { value = runCatching { Repo.iniciativas(botId, "", 300) }.getOrDefault(emptyList()) }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Registro de iniciativas") }) }) { relleno ->
        val datos = lista
        when {
            datos == null -> Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }
            datos.isEmpty() -> EstadoVacio(Ico.Reloj, "Sin movimientos", "Cuando un módulo envíe algo por su cuenta, o no pueda hacerlo, quedará anotado aquí.", Modifier.padding(relleno).padding(top = 60.dp))
            else -> LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("lista_registro_iniciativas"), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
                item { Text("Sin textos de mensajes. Los chats aparecen con un código corto.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                items(datos, key = { it.id }) { FilaRegistro(it) }
            }
        }
    }
}
