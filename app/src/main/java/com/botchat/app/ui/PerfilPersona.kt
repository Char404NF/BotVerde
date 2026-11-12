@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AssistChip
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.SnackbarResult
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.ContadorSilencio
import com.botchat.app.datos.PerfilPersonaItem
import com.botchat.app.datos.Repo
import com.botchat.app.datos.SilencioGrupoItem
import com.botchat.app.datos.TarjetaActividad
import com.botchat.app.datos.actividadPersona
import com.botchat.app.datos.contadorSilencio
import com.botchat.app.datos.perfilPersona
import com.botchat.app.datos.ponerContadorEnCero
import com.botchat.app.datos.quitarSilencioGrupos
import com.botchat.app.datos.silenciarEnGrupos
import com.botchat.app.datos.silenciosGrupos
import kotlinx.coroutines.launch

/**
 * El perfil de una persona: foto grande, nombre, número, rango, etiquetas y grupos en común; cuatro botones grandes (Silenciar, Advertir,
 * Marcar, Ver mensajes), la barra de color del silencio y una tarjeta de actividad por cada chat donde escribió.
 */
@Composable
fun PantallaPerfilPersona(
    botId: Long, jid: String, alVolver: () -> Unit,
    alAbrirActividad: (persona: String, chat: String, nombre: String) -> Unit = { _, _, _ -> },
    alVerMensajes: (chat: String) -> Unit = {},
    alAbrirGrupo: (String) -> Unit = {},
    alAbrirDetalle: (String) -> Unit = {},
    alAbrirEtiquetas: () -> Unit = {},
    alOlvidada: () -> Unit = {},
) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    val snack = remember { SnackbarHostState() }
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(jid) {
        Repo.cambios.collect { evs -> if (evs.any { it == "personas" || it == "persona:$jid" || it == "etiquetas" || it.startsWith("silenc") || it.startsWith("advert") || it == "rangos" }) version++ }
    }
    val perfil by produceState<PerfilPersonaItem?>(null, jid, version) { value = runCatching { Repo.perfilPersona(botId, jid) }.getOrNull() }
    val persona = perfil?.persona?.ifBlank { null } ?: jid
    val actividad by produceState<List<TarjetaActividad>?>(null, persona, version) { value = runCatching { Repo.actividadPersona(botId, persona) }.getOrDefault(emptyList()) }
    val silencio by produceState<SilencioGrupoItem?>(null, persona, version) {
        value = runCatching { Repo.silenciosGrupos(botId).firstOrNull { it.persona == persona } }.getOrNull()
    }
    val contador by produceState<ContadorSilencio?>(null, persona, version, silencio?.id) {
        value = if (silencio == null) null else silencio?.contador ?: runCatching { Repo.contadorSilencio(botId, persona) }.getOrNull()
    }
    var dialogo by remember { mutableStateOf<String?>(null) }
    var masOpciones by remember { mutableStateOf(false) }
    val p = perfil
    val nombre = p?.nombre?.ifBlank { null } ?: p?.telefono?.ifBlank { null } ?: numeroDePersona(jid)

    Box(Modifier.fillMaxSize()) {
    Scaffold(
        topBar = { TopAppBar(navigationIcon = { IconButton(alVolver, Modifier.testTag("volver_perfil")) { Icon(Ico.Atras, "Volver") } }, title = { Text("Perfil") }) },
        snackbarHost = { SnackbarHost(snack) },
    ) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l).testTag("perfil_persona"), verticalArrangement = Arrangement.spacedBy(Esp.l)) {
            // ----- cabecera -----
            Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                Avatar(nombre, p?.foto.orEmpty(), 0, jid, 120.dp, Modifier.testTag("perfil_foto"))
                Text(nombre, Modifier.testTag("perfil_nombre"), style = MaterialTheme.typography.headlineSmall, textAlign = TextAlign.Center, maxLines = 2, overflow = TextOverflow.Ellipsis)
                val numero = p?.telefono?.ifBlank { null } ?: numeroDePersona(jid)
                if (numero.isNotBlank() && numero != nombre) Text(numero, Modifier.testTag("perfil_numero"), style = MaterialTheme.typography.bodyLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
                if (p != null && p.rango.isNotBlank()) InsigniaRango(p.rango, p.rangoColor, Modifier.testTag("perfil_rango"))
                if (p == null) CircularProgressIndicator(Modifier.size(24.dp))
                if (p != null && p.etiquetas.isNotEmpty()) FlowRow(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(Esp.xs, Alignment.CenterHorizontally), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                    p.etiquetas.forEach { ChipEtiqueta(it) }
                }
                if (p != null) Text(
                    plural(p.mensajes.toLong(), "mensaje", "mensajes") + " · " + plural(p.archivos.toLong(), "archivo", "archivos") + if (p.archivos > 0) " (${formatoBytes(p.archivosBytes)})" else "",
                    Modifier.testTag("perfil_contadores"), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }

            // ----- silencio vigente -----
            val sil = silencio
            if (sil != null) {
                Card(Modifier.fillMaxWidth().testTag("tarjeta_silencio"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                    Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Icon(Ico.Volumen, null, tint = ROJO_SILENCIO)
                            Spacer(Modifier.width(Esp.s))
                            Text("Silenciada " + textoDuracionFin(sil.hastaTs) + textoDondeSilencio(sil), Modifier.testTag("silencio_estado"), style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
                        }
                        contador?.let { BarraContadorSilencio(it) }
                        FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                            OutlinedButton({
                                haptica(Toque.Suave)
                                scope.launch {
                                    runCatching { Repo.ponerContadorEnCero(botId, persona) }.onSuccess { avisaToast(ctx, "Contador en cero", false) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
                                    version++
                                }
                            }, Modifier.testTag("poner_en_cero")) { Text("Poner en cero") }
                            OutlinedButton({
                                haptica(Toque.Suave)
                                scope.launch {
                                    runCatching { Repo.quitarSilencioGrupos(botId, persona) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
                                    version++
                                    val r = snack.showSnackbar("Quité el silencio", actionLabel = "Deshacer", duration = androidx.compose.material3.SnackbarDuration.Long)
                                    if (r == SnackbarResult.ActionPerformed) {
                                        val seg = if (sil.hastaTs <= 0) 0L else maxOf(1L, sil.hastaTs - System.currentTimeMillis() / 1000)
                                        runCatching { Repo.silenciarEnGrupos(botId, persona, seg, sil.alcance.ifBlank { "todos" }, sil.grupos, false, sil.texto) }
                                            .onSuccess { avisaToast(ctx, "Volvió el silencio", false) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
                                        version++
                                    }
                                }
                            }, Modifier.testTag("quitar_silencio_grupos")) { Text("Quitar silencio") }
                        }
                    }
                }
            }

            // ----- cuatro botones grandes -----
            Column(verticalArrangement = Arrangement.spacedBy(Esp.m)) {
                Row(horizontalArrangement = Arrangement.spacedBy(Esp.m)) {
                    BotonGrandeE7(Ico.Volumen, "Silenciar", "boton_silenciar", Modifier.weight(1f)) { haptica(Toque.Suave); dialogo = "silenciar" }
                    BotonGrandeE7(Ico.Aviso, "Advertir", "boton_advertir", Modifier.weight(1f), MaterialTheme.colorScheme.tertiaryContainer, MaterialTheme.colorScheme.onTertiaryContainer) { haptica(Toque.Suave); dialogo = "advertir" }
                }
                Row(horizontalArrangement = Arrangement.spacedBy(Esp.m)) {
                    BotonGrandeE7(Ico.Seleccionar, "Marcar", "boton_marcar", Modifier.weight(1f), MaterialTheme.colorScheme.secondaryContainer, MaterialTheme.colorScheme.onSecondaryContainer) { haptica(Toque.Suave); dialogo = "marcar" }
                    BotonGrandeE7(Ico.Chats, "Ver mensajes", "boton_ver_mensajes", Modifier.weight(1f), MaterialTheme.colorScheme.surfaceContainerHigh, MaterialTheme.colorScheme.onSurface) { haptica(Toque.Suave); alVerMensajes(jid) }
                }
            }

            // ----- grupos en común -----
            if (p != null && p.gruposComun.isNotEmpty()) {
                Text("Grupos en común", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs), modifier = Modifier.testTag("grupos_en_comun")) {
                    p.gruposComun.forEach { g -> AssistChip({ alAbrirGrupo(g.jid) }, { Text(g.nombre.ifBlank { "Grupo" }, maxLines = 1) }, Modifier.testTag("comun_${g.jid}")) }
                }
            }

            // ----- actividad -----
            Text("Actividad", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary)
            val tarjetas = actividad
            when {
                tarjetas == null -> CircularProgressIndicator(Modifier.size(24.dp))
                tarjetas.isEmpty() -> Text("Todavía no hay nada que mostrar.", Modifier.testTag("actividad_vacia"), color = MaterialTheme.colorScheme.onSurfaceVariant)
                else -> Column(verticalArrangement = Arrangement.spacedBy(Esp.s), modifier = Modifier.testTag("lista_actividad")) {
                    tarjetas.forEach { t -> TarjetaDeActividad(t) { haptica(Toque.Suave); alAbrirActividad(persona, t.chat, if (t.privado) "Chat privado" else t.nombre) } }
                }
            }

            // ----- más opciones (lo de siempre) -----
            OutlinedButton({ masOpciones = !masOpciones }, Modifier.fillMaxWidth().testTag("mas_opciones")) { Text(if (masOpciones) "Ocultar más opciones" else "Más opciones (ignorar, bloquear, exportar, olvidar)") }
            if (masOpciones) {
                TarjetaPersona(botId, jid, nombre, alAbrirDetalle = { alAbrirDetalle(persona) }, alOlvidada = alOlvidada)
                TarjetaRango(botId, jid, nombre)
            }
            Spacer(Modifier.size(Esp.xl))
        }
    }

    when (dialogo) {
        "silenciar" -> DialogoSilenciarGrupos(botId, persona, nombre, alCerrar = { dialogo = null }, alListo = { version++ })
        "advertir" -> DialogoAdvertirTipo(botId, persona, nombre, p?.gruposComun ?: emptyList(), alCerrar = { dialogo = null }, alListo = { version++ })
        "marcar" -> DialogoMarcarPersona(botId, persona, nombre, p?.etiquetas ?: emptyList(), alCerrar = { dialogo = null }, alCambio = { version++ }, alAdministrar = { dialogo = null; alAbrirEtiquetas() }, alNota = { dialogo = null; alAbrirDetalle(persona) })
    }
    }
}

private fun textoDondeSilencio(s: SilencioGrupoItem) = when (s.alcance) {
    "uno" -> " en un grupo"
    "varios" -> " en ${s.grupos.size} grupos"
    else -> " en todos mis grupos"
}

/** La tarjeta de un chat: cuánto escribió ahí. Tocarla abre el detalle. */
@Composable
private fun TarjetaDeActividad(t: TarjetaActividad, alPulsar: () -> Unit) {
    Card(
        onClick = alPulsar, modifier = Modifier.fillMaxWidth().testTag(if (t.privado) "actividad_privado" else "actividad_${t.chat}"),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
    ) {
        Row(Modifier.padding(Esp.m), verticalAlignment = Alignment.CenterVertically) {
            if (t.privado) androidx.compose.foundation.layout.Box(Modifier.size(44.dp), Alignment.Center) { Icon(Ico.Persona, null, Modifier.size(32.dp), tint = MaterialTheme.colorScheme.primary) }
            else Avatar(t.nombre, t.foto, 0, t.chat, 44.dp)
            Spacer(Modifier.width(Esp.m))
            Column(Modifier.weight(1f)) {
                Text(if (t.privado) "Chat privado" else t.nombre.ifBlank { "Grupo" }, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                val partes = buildList {
                    add(plural(t.mensajes.toLong(), "mensaje", "mensajes"))
                    if (t.multimedia > 0) add(plural(t.multimedia.toLong(), "foto o video", "fotos y videos"))
                    if (t.audios > 0) add(plural(t.audios.toLong(), "audio", "audios"))
                    if (t.archivos + t.enlaces > 0) add(plural((t.archivos + t.enlaces).toLong(), "archivo o enlace", "archivos y enlaces"))
                }
                Text(partes.joinToString(" · "), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                if (!t.privado && !t.miembro) Text("Ya no está en el grupo", style = MaterialTheme.typography.labelSmall, color = AMARILLO_SILENCIO)
                if (t.ultimoTs > 0) Text("Última vez: ${etiquetaDia(t.ultimoTs)}", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            Icon(Ico.Siguiente, null, tint = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}
