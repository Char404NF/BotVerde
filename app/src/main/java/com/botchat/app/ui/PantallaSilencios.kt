@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.SnackbarResult
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.AjustesSilencio
import com.botchat.app.datos.Repo
import com.botchat.app.datos.SancionPendiente
import com.botchat.app.datos.SilencioGrupoItem
import com.botchat.app.datos.ajustesSilencio
import com.botchat.app.datos.confirmarSancion
import com.botchat.app.datos.fijarAjusteSilencio
import com.botchat.app.datos.fijarTextoSilencio
import com.botchat.app.datos.quitarSilencioGrupos
import com.botchat.app.datos.ponerContadorEnCero
import com.botchat.app.datos.sancionesPendientes
import com.botchat.app.datos.silenciarEnGrupos
import com.botchat.app.datos.silenciosGrupos
import kotlinx.coroutines.launch

/** Bot → Silencios: a quién silenciaste, cuánto va su contador de hoy y lo que espera tu decisión. */
@Composable
fun PantallaSilencios(botId: Long, alVolver: () -> Unit, alAjustes: () -> Unit, alAbrirPersona: (String) -> Unit = {}) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    val snack = remember { SnackbarHostState() }
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if (evs.any { it.startsWith("silenc") || it.startsWith("sancion") || it == "personas" || it.startsWith("aviso") }) version++ } }
    val lista by produceState<List<SilencioGrupoItem>?>(null, version) { value = runCatching { Repo.silenciosGrupos(botId) }.getOrDefault(emptyList()) }
    val sanciones by produceState<List<SancionPendiente>>(emptyList(), version) { value = runCatching { Repo.sancionesPendientes(botId) }.getOrDefault(emptyList()) }
    var expulsando by remember { mutableStateOf<SancionPendiente?>(null) }

    Scaffold(
        topBar = {
            TopAppBar(
                navigationIcon = { IconButton(alVolver, Modifier.testTag("volver_silencios")) { Icon(Ico.Atras, "Volver") } }, title = { Text("Silencios") },
                actions = { IconButton(alAjustes, Modifier.testTag("silencios_ajustes")) { Icon(Ico.Ajustes, "Ajustes de los silencios") } },
            )
        },
        snackbarHost = { SnackbarHost(snack) },
    ) { relleno ->
        LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("pantalla_silencios"), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            items(sanciones, key = { "s${it.id}" }) { s ->
                Card(Modifier.fillMaxWidth().testTag("sancion_${s.id}"), colors = CardDefaults.cardColors(containerColor = ROJO_SILENCIO.copy(alpha = 0.14f))) {
                    Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                        Text("¿Expulsamos a ${s.nombre.ifBlank { "esta persona" }}?", style = MaterialTheme.typography.titleSmall)
                        Text(
                            (if (s.origen == "advertencia") "Llegó al último paso de las advertencias" else "Siguió escribiendo estando silenciada") + (if (s.nombreGrupo.isNotBlank()) " en ${s.nombreGrupo}." else "."),
                            style = MaterialTheme.typography.bodyMedium,
                        )
                        Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                            Button({ haptica(Toque.Fuerte); expulsando = s }, Modifier.weight(1f).testTag("sancion_expulsar_${s.id}")) { Text("Expulsar") }
                            OutlinedButton({
                                haptica(Toque.Suave)
                                scope.launch { runCatching { Repo.confirmarSancion(botId, s.id, false) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }; version++ }
                            }, Modifier.weight(1f).testTag("sancion_dejarlo_${s.id}")) { Text("Dejarlo") }
                        }
                    }
                }
            }
            val datos = lista
            when {
                datos == null -> item { FilasDeCarga(3) }
                datos.isEmpty() -> item { EstadoVacio(Ico.Volumen, "No hay nadie silenciado", "Para silenciar a alguien abre su perfil en Personas y toca Silenciar. Sus mensajes en tus grupos se borran solos.", Modifier.padding(top = Esp.xl).testTag("sin_silencios")) }
                else -> {
                    item { Text("Mientras alguien está silenciado, sus mensajes se borran en tus grupos (solo donde soy admin).", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                    items(datos, key = { it.id }) { x -> TarjetaSilencio(x, alAbrirPersona,
                        alCero = { scope.launch { runCatching { Repo.ponerContadorEnCero(botId, x.persona) }; avisaToast(ctx, "Contador en cero"); version++ } },
                        alQuitar = {
                            haptica(Toque.Suave)
                            scope.launch {
                                runCatching { Repo.quitarSilencioGrupos(botId, x.persona) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
                                version++
                                val r = snack.showSnackbar("Quité el silencio", actionLabel = "Deshacer", duration = androidx.compose.material3.SnackbarDuration.Long)
                                if (r == SnackbarResult.ActionPerformed) {
                                    val seg = if (x.hastaTs <= 0) 0L else maxOf(1L, x.hastaTs - System.currentTimeMillis() / 1000)
                                    runCatching { Repo.silenciarEnGrupos(botId, x.persona, seg, x.alcance.ifBlank { "todos" }, x.grupos, false, x.texto) }
                                        .onSuccess { avisaToast(ctx, "Volvió el silencio") }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
                                    version++
                                }
                            }
                        })
                    }
                }
            }
        }
    }
    expulsando?.let { s ->
        ConfirmarSinDeshacer("¿Expulsar a ${s.nombre.ifBlank { "esta persona" }}?", "Se le saca del grupo en WhatsApp. Solo funciona si soy administrador.", "Expulsar", "confirmar_expulsar", { expulsando = null }) {
            expulsando = null
            scope.launch {
                runCatching { Repo.confirmarSancion(botId, s.id, true) }.onSuccess { avisaToast(ctx, it.ifBlank { "Listo" }) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
                version++
            }
        }
    }
}

@Composable
private fun TarjetaSilencio(x: SilencioGrupoItem, alAbrirPersona: (String) -> Unit, alCero: () -> Unit, alQuitar: () -> Unit) {
    Card(Modifier.fillMaxWidth().testTag("silencio_${x.id}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    TextButton({ alAbrirPersona(x.persona) }, Modifier.testTag("silencio_persona_${x.id}"), contentPadding = androidx.compose.foundation.layout.PaddingValues(0.dp)) {
                        Text(x.nombre.ifBlank { numeroDePersona(x.persona) }, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                    Text(
                        (if (x.alcance == "todos") "En todos mis grupos" else if (x.grupos.size == 1) "En 1 grupo" else "En ${x.grupos.size} grupos") + " · " + textoDuracionFin(x.hastaTs),
                        Modifier.testTag("silencio_hasta_${x.id}"), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
            x.contador?.let { BarraContadorSilencio(it) }
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                OutlinedButton(alCero, Modifier.weight(1f).testTag("silencio_cero_${x.id}")) { Text("Poner en cero") }
                OutlinedButton(alQuitar, Modifier.weight(1f).testTag("silencio_quitar_${x.id}")) { Text("Quitar silencio") }
            }
        }
    }
}

// ---------------------------------------------------------------- ajustes

private val SANCIONES = listOf(
    "confirmar" to "Preguntarme antes de expulsar",
    "avisar" to "Solo avisarme",
    "silencio_largo" to "Silencio más largo",
    "expulsar" to "Expulsar",
)

private val TEXTOS = listOf(
    "inicio" to "Cuando le silencio",
    "aviso" to "Aviso cada vez que insiste",
    "tope" to "Cuando llega al tope",
    "fin" to "Cuando termina el silencio",
    "noadmin" to "Si no soy admin del grupo",
)

private val HORAS_REINICIO = listOf("00:00", "03:00", "04:00", "06:00")

/** Ajustes de los silencios: tiempo y alcance de siempre, textos, cada cuántos mensajes avisa, tope y sanción, reinicio del contador y topes propios. */
@Composable
fun PantallaAjustesSilencio(botId: Long, alVolver: () -> Unit, ajustesApp: com.botchat.app.datos.Ajustes? = null) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    val a by produceState<AjustesSilencio?>(null, version) { value = runCatching { Repo.ajustesSilencio(botId) }.getOrDefault(AjustesSilencio(emptyMap(), emptyMap())) }
    var editandoTexto by remember { mutableStateOf<String?>(null) }
    var editandoNumero by remember { mutableStateOf<Triple<String, String, Int>?>(null) }
    var horaPropia by remember { mutableStateOf(false) }
    var avanzado by remember { mutableStateOf(false) }
    fun poner(clave: String, valor: String) { scope.launch { runCatching { Repo.fijarAjusteSilencio(botId, clave, valor) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }; version++ } }

    Box(Modifier.fillMaxSize()) {
        Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver, Modifier.testTag("volver_ajustes_silencio")) { Icon(Ico.Atras, "Volver") } }, title = { Text("Ajustes de los silencios") }) }) { relleno ->
            val v = a
            if (v == null) { FilasDeCarga(4); return@Scaffold }
            Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(androidx.compose.foundation.rememberScrollState()).padding(Esp.l).testTag("pantalla_ajustes_silencio"), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
                Card(Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                    Column(Modifier.padding(vertical = Esp.s)) {
                        Interruptor("Silenciar en grupos", "Si lo apagas, nadie se silencia y no se borra nada.", v.encendido("sil_activo", true), true, Ico.Volumen) { poner("sil_activo", if (it) "1" else "0") }
                        Interruptor("Avisarle cuando lo silencio", "Le llega un mensaje privado.", v.encendido("sil_avisar_inicio", true), true, Ico.Responder) { poner("sil_avisar_inicio", if (it) "1" else "0") }
                        Interruptor("Avisarle cuando termina", "Y también a mí.", v.encendido("sil_avisar_fin", true), true, Ico.Reloj) { poner("sil_avisar_fin", if (it) "1" else "0") }
                        Interruptor("Avisarme si no soy admin", "Si no puedo borrar, te lo digo (una vez al día por grupo).", v.encendido("sil_aviso_noadmin", true), true, Ico.Aviso) { poner("sil_aviso_noadmin", if (it) "1" else "0") }
                    }
                }

                if (ajustesApp != null) {
                    TituloSeccionE8("Al silenciar, de entrada")
                    val tiempo by ajustesApp.silencioTiempoDefecto.collectAsState()
                    val alcance by ajustesApp.silencioAlcanceDefecto.collectAsState()
                    SelectorValores(tiempo.toLong(), listOf(OpcionValor(60, "1 hora"), OpcionValor(1440, "1 día"), OpcionValor(10080, "1 semana"), OpcionValor(0, "Siempre")), "def_tiempo", { ajustesApp.fijarSilencioTiempoDefecto(it.toInt()) })
                    SelectorValores(if (alcance == "todos") 2 else 1, listOf(OpcionValor(1, "Un grupo"), OpcionValor(2, "Todos los que administro")), "def_alcance", { ajustesApp.fijarSilencioAlcanceDefecto(if (it == 2L) "todos" else "uno") })
                }

                TituloSeccionE8("Qué le digo")
                Text("Puedes cambiar cada texto. Usa {grupo}, {n}, {tope} y {duracion} donde quieras que aparezcan.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                TEXTOS.forEach { (id, titulo) ->
                    val (actual, _) = v.textos[id] ?: ("" to "")
                    OutlinedButton({ editandoTexto = id }, Modifier.fillMaxWidth().testTag("texto_$id")) {
                        Column(Modifier.weight(1f)) { Text(titulo, style = MaterialTheme.typography.labelLarge); Text(actual, maxLines = 1, overflow = TextOverflow.Ellipsis, style = MaterialTheme.typography.bodySmall) }
                        Icon(Ico.Editar, null, Modifier.size(18.dp))
                    }
                }

                TituloSeccionE8("Avisos y tope")
                FilaNumeroE8("Avisarle cada cuántos mensajes borrados", v.entero("sil_aviso_cada", 3), "sil_aviso_cada", "mensajes") { clave, titulo, valor -> editandoNumero = Triple(clave, titulo, valor) }
                FilaNumeroE8("Tope de mensajes borrados por día", v.entero("sil_tope", 10), "sil_tope", "mensajes") { clave, titulo, valor -> editandoNumero = Triple(clave, titulo, valor) }
                Text("Al llegar al tope, ¿qué hago?", style = MaterialTheme.typography.titleSmall)
                androidx.compose.foundation.layout.FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    SANCIONES.forEach { (k, t) -> FilterChip(v.texto("sil_sancion", "confirmar") == k, { haptica(Toque.Suave); poner("sil_sancion", k) }, { Text(t) }, modifier = Modifier.testTag("sancion_$k")) }
                }
                when (v.texto("sil_sancion", "confirmar")) {
                    "silencio_largo" -> FilaNumeroE8("Cuánto se alarga", v.entero("sil_largo_min", 10080) / 60, "sil_largo_min", "horas", 60) { clave, titulo, valor -> editandoNumero = Triple(clave, titulo, valor) }
                    "expulsar" -> Interruptor("Expulsar sin preguntarme", "Si lo apagas (recomendado), te pido un toque antes de expulsar.", v.encendido("sil_expulsion_auto", false), true, Ico.Aviso) { poner("sil_expulsion_auto", if (it) "1" else "0") }
                }

                TituloSeccionE8("El contador de cada día")
                Text("¿A qué hora empieza un día nuevo?", style = MaterialTheme.typography.titleSmall)
                androidx.compose.foundation.layout.FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    val hora = v.texto("sil_reinicio_hora", "00:00")
                    HORAS_REINICIO.forEach { h -> FilterChip(hora == h, { haptica(Toque.Suave); poner("sil_reinicio_hora", h) }, { Text(h) }, modifier = Modifier.testTag("hora_$h")) }
                    FilterChip(hora !in HORAS_REINICIO, { horaPropia = true }, { Text(if (hora in HORAS_REINICIO) "Otra hora…" else hora) }, modifier = Modifier.testTag("hora_otra"))
                }
                Interruptor("Contar cada grupo por separado", "Apagado: se suman los mensajes borrados en todos los grupos.", v.encendido("sil_conteo_por_grupo", false), true, Ico.Grupo) { poner("sil_conteo_por_grupo", if (it) "1" else "0") }

                TextButton({ avanzado = !avanzado }, Modifier.testTag("silencio_avanzado")) { Text(if (avanzado) "Ocultar avanzado" else "Avanzado: topes propios") }
                if (avanzado) {
                    Text("Estos topes cuidan tu cuenta de WhatsApp. Son independientes de los de «Límites e iniciativa».", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    FilaNumeroE8("Avisos privados por hora", v.entero("sil_avisos_hora", 12), "sil_avisos_hora", "avisos") { clave, titulo, valor -> editandoNumero = Triple(clave, titulo, valor) }
                    FilaNumeroE8("Avisos privados por día", v.entero("sil_avisos_dia", 60), "sil_avisos_dia", "avisos") { clave, titulo, valor -> editandoNumero = Triple(clave, titulo, valor) }
                    FilaNumeroE8("Avisos por persona al día", v.entero("sil_avisos_persona_dia", 4), "sil_avisos_persona_dia", "avisos") { clave, titulo, valor -> editandoNumero = Triple(clave, titulo, valor) }
                    FilaNumeroE8("Mensajes borrados por minuto", v.entero("sil_borrados_min", 20), "sil_borrados_min", "mensajes") { clave, titulo, valor -> editandoNumero = Triple(clave, titulo, valor) }
                }
            }
        }
        editandoTexto?.let { id ->
            val (actual, defecto) = a?.textos?.get(id) ?: ("" to "")
            HojaTexto(
                TEXTOS.first { it.first == id }.second, actual, "hoja_texto", { editandoTexto = null },
                { t -> editandoTexto = null; scope.launch { runCatching { Repo.fijarTextoSilencio(botId, id, if (t == defecto) "" else t) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }; version++ } },
                ayuda = "De fábrica: $defecto", permiteVacio = true, botonRestaurar = "Volver al de fábrica",
            )
        }
        editandoNumero?.let { (clave, titulo, valor) ->
            val factor = if (clave == "sil_largo_min") 60 else 1
            HojaNumero(titulo, if (clave == "sil_largo_min") "Horas" else "Número", valor.toLong(), "hoja_numero", { editandoNumero = null }, { n -> editandoNumero = null; poner(clave, (n * factor).toString()) })
        }
        if (horaPropia) HojaNumero("Hora del nuevo día", "Hora (0 a 23)", 0, "hoja_hora", { horaPropia = false }, { n -> horaPropia = false; if (n in 0..23) poner("sil_reinicio_hora", "%02d:00".format(n)) })
    }
}

@Composable
private fun FilaNumeroE8(titulo: String, valor: Int, clave: String, unidad: String, factor: Int = 1, alEditar: (String, String, Int) -> Unit) {
    OutlinedButton({ alEditar(clave, titulo, valor) }, Modifier.fillMaxWidth().testTag("num_$clave")) {
        Text(titulo, Modifier.weight(1f), maxLines = 2)
        Spacer(Modifier.width(Esp.s))
        Text(if (valor == 0) "Sin tope" else "$valor $unidad", fontWeight = FontWeight.SemiBold)
    }
}
