@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.InputChip
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
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.ACCIONES_FILTRO
import com.botchat.app.datos.ConfigFiltroGrupo
import com.botchat.app.datos.EstadoGrupo
import com.botchat.app.datos.FiltroGrupo
import com.botchat.app.datos.Rango
import com.botchat.app.datos.Repo
import kotlinx.coroutines.launch

/**
 * Filtros de un grupo: enlaces (con lista permitida), palabras prohibidas, mensajes repetidos y flood. Cada uno se enciende por separado,
 * tiene su acción y sus excepciones. Cada cambio se guarda al instante. Si el bot no es administrador, borrar y expulsar quedan desactivados:
 * en ese caso un filtro solo avisa con una notificación local.
 */
@Composable
fun PantallaFiltrosGrupo(botId: Long, jid: String, alVolver: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var filtros by remember { mutableStateOf<List<FiltroGrupo>?>(null) }
    var estado by remember { mutableStateOf<EstadoGrupo?>(null) }
    var rangos by remember { mutableStateOf<List<Rango>>(emptyList()) }
    var error by remember { mutableStateOf<String?>(null) }
    suspend fun cargar() {
        runCatching { Repo.filtrosGrupo(botId, jid) }.onSuccess { filtros = it }.onFailure { error = textoError(it) }
    }
    LaunchedEffect(jid) {
        cargar()
        estado = runCatching { Repo.estadoGrupo(botId, jid) }.getOrElse { EstadoGrupo(false, false, false, textoError(it)) }
        rangos = runCatching { Repo.rangos(botId) }.getOrDefault(emptyList())
    }
    fun guardar(f: FiltroGrupo) {
        filtros = filtros?.map { if (it.filtro == f.filtro) f else it }
        scope.launch { runCatching { Repo.guardarFiltroGrupo(botId, f) }.onFailure { avisaToast(ctx, textoError(it), true); cargar() } }
    }
    val admin = estado?.soyAdmin == true
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Filtros del grupo") }) }) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.l)) {
            TarjetaEstadoAdmin(estado) { scope.launch { estado = null; estado = runCatching { Repo.estadoGrupo(botId, jid, true) }.getOrElse { EstadoGrupo(false, false, false, textoError(it)) } } }
            if (estado != null && !admin) Text(
                "Como no soy administrador, los filtros solo te avisan con una notificación en este teléfono. Borrar el mensaje y expulsar quedan desactivados hasta que me hagas administrador.",
                Modifier.testTag("filtros_sin_admin"), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            val lista = filtros
            when {
                error != null -> Text(error!!, Modifier.testTag("filtros_error"), color = MaterialTheme.colorScheme.error)
                lista == null -> CircularProgressIndicator(Modifier.size(24.dp))
                else -> lista.forEach { f -> TarjetaFiltro(f, admin, estado != null, rangos, ::guardar) { scope.launch { runCatching { Repo.restaurarFiltroGrupo(botId, jid, f.filtro) }.onFailure { avisaToast(ctx, textoError(it), true) }; cargar() } } }
            }
        }
    }
}

@Composable
private fun Paso(titulo: String, valor: Int, minimo: Int, maximo: Int, paso: Int, etiqueta: String, alCambiar: (Int) -> Unit) {
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        Text(titulo, Modifier.weight(1f), style = MaterialTheme.typography.bodyMedium)
        OutlinedButton({ alCambiar((valor - paso).coerceAtLeast(minimo)) }, Modifier.testTag("${etiqueta}_menos"), enabled = valor > minimo) { Text("−") }
        Text("$valor", Modifier.padding(horizontal = Esp.m).testTag("${etiqueta}_valor"), style = MaterialTheme.typography.titleMedium)
        OutlinedButton({ alCambiar((valor + paso).coerceAtMost(maximo)) }, Modifier.testTag("${etiqueta}_mas"), enabled = valor < maximo) { Text("+") }
    }
}

@Composable
private fun ListaEditable(titulo: String, ayuda: String, items: List<String>, etiqueta: String, alCambiar: (List<String>) -> Unit) {
    var nuevo by remember { mutableStateOf("") }
    Column(verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
        Text(titulo, style = MaterialTheme.typography.labelLarge)
        Text(ayuda, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        if (items.isNotEmpty()) FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.xs), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
            items.forEach { x -> InputChip(false, { alCambiar(items - x) }, { Text(x) }, modifier = Modifier.testTag("${etiqueta}_item_$x"), trailingIcon = { Icon(Ico.Cerrar, "Quitar $x", Modifier.size(16.dp)) }) }
        }
        Row(verticalAlignment = Alignment.CenterVertically) {
            OutlinedTextField(nuevo, { nuevo = it }, Modifier.weight(1f).testTag("${etiqueta}_campo"), singleLine = true, label = { Text("Agregar") })
            Spacer(Modifier.width(Esp.s))
            Button({ val t = nuevo.trim(); if (t.isNotEmpty()) { alCambiar(items + t); nuevo = "" } }, Modifier.testTag("${etiqueta}_agregar"), enabled = nuevo.isNotBlank()) { Text("Agregar") }
        }
    }
}

@Composable
private fun TarjetaFiltro(f: FiltroGrupo, admin: Boolean, estadoConocido: Boolean, rangos: List<Rango>, alGuardar: (FiltroGrupo) -> Unit, alRestaurar: () -> Unit) {
    val c = f.config
    fun cfg(n: ConfigFiltroGrupo) = alGuardar(f.copy(config = n))
    val haptica = rememberHaptica()
    Card(Modifier.fillMaxWidth().testTag("filtro_${f.filtro}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text(f.nombre, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
                    Text(f.descripcion, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                Switch(f.activo, { haptica(Toque.Suave); alGuardar(f.copy(activo = it)) }, Modifier.testTag("sw_filtro_${f.filtro}"))
            }
            if (!f.activo) return@Column
            when (f.filtro) {
                "enlaces" -> {
                    Text("Qué enlaces cuentan", style = MaterialTheme.typography.labelLarge)
                    FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                        FilterChip(c.modo == "adultos", { haptica(Toque.Suave); cfg(c.copy(modo = "adultos")) }, { Text("Solo adultos y peligrosos") }, modifier = Modifier.testTag("modo_adultos"))
                        FilterChip(c.modo != "adultos", { haptica(Toque.Suave); cfg(c.copy(modo = "todos")) }, { Text("Todos los enlaces") }, modifier = Modifier.testTag("modo_todos"))
                    }
                    Text(
                        if (c.modo == "adultos") "Detecto páginas de contenido adulto y las que tú agregues abajo. Los demás enlaces pasan."
                        else "Todos los enlaces cuentan, menos los dominios permitidos.",
                        style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    if (c.modo == "adultos") {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Column(Modifier.weight(1f)) {
                                Text("Bloquear acortadores", style = MaterialTheme.typography.bodyMedium)
                                Text("bit.ly, tinyurl… esconden a dónde lleva el enlace.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                            Switch(c.acortadores, { haptica(Toque.Suave); cfg(c.copy(acortadores = it)) }, Modifier.testTag("sw_acortadores"))
                        }
                        ListaEditable("Dominios bloqueados", "Páginas que nunca quieres, además de las de contenido adulto.", c.bloqueados, "bloqueado") { cfg(c.copy(bloqueados = it)) }
                    }
                    ListaEditable("Dominios permitidos", "Un dominio permitido deja pasar también sus subdominios (youtube.com permite m.youtube.com).", c.permitidos, "permitido") { cfg(c.copy(permitidos = it)) }
                }
                "palabras" -> ListaEditable("Palabras o frases prohibidas", "Entiendo mayúsculas, acentos y trucos como p3nd3j0, p.u.t.o o letras repetidas. Cuenta la palabra completa.", c.palabras, "palabra") { cfg(c.copy(palabras = it)) }
                "repetidos" -> {
                    Paso("Veces seguidas", c.max, 2, 50, 1, "rep_max") { cfg(c.copy(max = it)) }
                    Paso("En cuántos segundos", c.segundos, 10, 3600, 10, "rep_seg") { cfg(c.copy(segundos = it)) }
                }
                "flood" -> {
                    Paso("Mensajes", c.max, 2, 100, 1, "flood_max") { cfg(c.copy(max = it)) }
                    Paso("En cuántos segundos", c.segundos, 1, 3600, 1, "flood_seg") { cfg(c.copy(segundos = it)) }
                }
            }
            Text("Qué hago", style = MaterialTheme.typography.labelLarge)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                ACCIONES_FILTRO.forEach { a ->
                    val bloqueada = a.requiereAdmin && !admin && estadoConocido
                    FilterChip(f.accion == a.id, { haptica(Toque.Suave); alGuardar(f.copy(accion = a.id)) }, { Text(a.texto) }, enabled = !bloqueada || f.accion == a.id, modifier = Modifier.testTag("accion_${f.filtro}_${a.id}"))
                }
            }
            val sel = ACCIONES_FILTRO.firstOrNull { it.id == f.accion }
            if (sel != null) Text(sel.ayuda + if (sel.requiereAdmin && !admin && estadoConocido) " $TEXTO_NECESITA_ADMIN Mientras tanto solo te avisaré." else "", Modifier.testTag("accion_ayuda_${f.filtro}"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            if (f.accion == "silenciar") Paso("Minutos de silencio", c.silencioMin, 5, 10080, 5, "silencio") { cfg(c.copy(silencioMin = it)) }
            if (f.accion == "avisar_grupo") OutlinedTextField(c.aviso, { cfg(c.copy(aviso = it)) }, Modifier.fillMaxWidth().testTag("aviso_${f.filtro}"), label = { Text("Texto del aviso (vacío = el de siempre)") }, minLines = 1)

            Text("A quién no se le aplica", style = MaterialTheme.typography.labelLarge)
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text("Los administradores del grupo", Modifier.weight(1f), style = MaterialTheme.typography.bodyMedium)
                Switch(c.exentoAdmins, { haptica(Toque.Suave); cfg(c.copy(exentoAdmins = it)) }, Modifier.testTag("exento_admins_${f.filtro}"))
            }
            val elegibles = rangos.filter { !it.esDueno }
            if (elegibles.isNotEmpty()) {
                Text("Estos rangos tampoco (el dueño nunca):", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    elegibles.forEach { r ->
                        FilterChip(r.id in c.exentoRangos, { haptica(Toque.Suave); cfg(c.copy(exentoRangos = if (r.id in c.exentoRangos) c.exentoRangos - r.id else c.exentoRangos + r.id)) }, { Text(r.nombre) }, modifier = Modifier.testTag("exento_rango_${f.filtro}_${r.id}"))
                    }
                }
            }
            TextButton(alRestaurar, Modifier.testTag("restaurar_${f.filtro}")) { Text("Volver a los valores de fábrica") }
        }
    }
}
