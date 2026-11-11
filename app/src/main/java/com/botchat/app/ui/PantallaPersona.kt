@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import android.content.Context
import android.content.Intent
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.FichaPersona
import com.botchat.app.datos.Repo
import com.botchat.app.medios.Exportar
import com.botchat.app.medios.Temporales
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.File

// ---------- Ficha de la persona (en Info del chat) ----------

internal fun textoDuracionSilencio(min: Int) = when (min) {
    0 -> "Para siempre"
    in 1..59 -> "$min minutos"
    60 -> "1 hora"
    in 61..1439 -> "${min / 60} horas"
    1440 -> "1 día"
    else -> "${min / 1440} días"
}

internal val DURACIONES_SILENCIO = listOf(30, 60, 480, 1440, 10080, 0)

private fun fechaCorta(ts: Long) = etiquetaDia(ts) + " " + horaDe(ts)

/**
 * La ficha de una persona: estado del bot con ella (silenciada, bloqueada), advertencias, notas, primera y última vez, y las acciones
 * (silenciar, bloquear, advertir, nota, exportar y olvidar). Todo es local; "bloquear" es solo del bot, no de WhatsApp.
 */
@Composable
fun TarjetaPersona(botId: Long, jid: String, nombre: String, alAbrirDetalle: () -> Unit = {}, alOlvidada: () -> Unit = {}) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if (evs.any { it == "personas" || it == "persona:$jid" || it == "solicitudes" || it == "rangos" || it == "etiquetas" }) version++ } }
    val ficha by produceState<FichaPersona?>(null, jid, version) { value = runCatching { Repo.ficha(botId, jid) }.getOrNull() }
    var dialogo by remember { mutableStateOf<String?>(null) }
    val f = ficha
    fun hacer(ok: String? = null, accion: suspend () -> Unit) = scope.launch {
        runCatching { accion() }.onSuccess { if (ok != null) avisaToast(ctx, ok, false) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
        version++
    }
    Card(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.xs).testTag("tarjeta_persona"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Text("Ficha de la persona", style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary)
            if (f == null) { CircularProgressIndicator(Modifier.size(24.dp)); return@Column }
            val estado = when {
                f.bloqueada -> "El bot la ignora por completo"
                f.silencio != null && f.silencio.hastaTs == 0L -> "Silenciada para siempre"
                f.silencio != null -> "Silenciada hasta ${fechaCorta(f.silencio.hastaTs)}"
                else -> "El bot le contesta normal"
            }
            Text(estado, Modifier.testTag("persona_estado"), style = MaterialTheme.typography.bodyLarge, fontWeight = FontWeight.Medium,
                color = if (f.bloqueada) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurface)
            if (f.noMolestar) Text("Está en la lista No molestar", Modifier.testTag("persona_nm"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Text(
                plural(f.advertencias.toLong(), "advertencia", "advertencias") + " · " + plural(f.notas.toLong(), "nota", "notas"),
                Modifier.testTag("persona_conteos").padding(top = 2.dp), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            if (f.etiquetas.isNotEmpty()) FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.xs), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                f.etiquetas.forEach { ChipEtiqueta(it) }
            }
            val datos = buildList {
                add("Escribió ${f.recibidos} mensajes" + (if (f.grupos > 0) " (en ${plural(f.grupos.toLong(), "grupo", "grupos")})" else "") + " · le escribiste ${f.enviados}")
                if (f.primeraTs > 0) add("Primera vez: ${etiquetaDia(f.primeraTs)} · Última: ${etiquetaDia(f.ultimaTs)}")
            }
            datos.forEach { Text(it, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.xs), modifier = Modifier.padding(top = Esp.xs)) {
                if (f.silencio != null) OutlinedButton({ haptica(Toque.Suave); hacer("El bot vuelve a contestarle") { Repo.quitarSilencioPersona(botId, jid) } }, Modifier.testTag("quitar_silencio")) { Text("Quitar silencio") }
                else OutlinedButton({ haptica(Toque.Suave); dialogo = "silenciar" }, Modifier.testTag("silenciar_persona")) { Text("Silenciar al bot") }
                if (f.bloqueada) OutlinedButton({ haptica(Toque.Suave); hacer("El bot vuelve a atenderla") { Repo.desbloquearPersona(botId, jid) } }, Modifier.testTag("desbloquear_persona")) { Text("Desbloquear") }
                else OutlinedButton({ haptica(Toque.Fuerte); dialogo = "bloquear" }, Modifier.testTag("bloquear_persona")) { Text("Bloquear en el bot") }
                OutlinedButton({ haptica(Toque.Suave); dialogo = "advertir" }, Modifier.testTag("advertir_persona")) { Text("Advertir") }
                OutlinedButton({ haptica(Toque.Suave); alAbrirDetalle() }, Modifier.testTag("abrir_advertencias_notas")) { Text("Notas y advertencias") }
                OutlinedButton({ haptica(Toque.Suave); dialogo = "exportar" }, Modifier.testTag("exportar_persona")) { Text("Exportar datos") }
                OutlinedButton({ haptica(Toque.Fuerte); dialogo = "olvidar" }, Modifier.testTag("olvidar_persona"), colors = androidx.compose.material3.ButtonDefaults.outlinedButtonColors(contentColor = MaterialTheme.colorScheme.error)) { Text("Olvidar a esta persona") }
            }
        }
    }
    when (dialogo) {
        "silenciar" -> DialogoSilenciar(nombre, { dialogo = null }) { min, motivo -> dialogo = null; hacer("Listo: el bot no le contestará") { Repo.silenciarPersona(botId, jid, min, motivo) } }
        "bloquear" -> AlertDialog(
            onDismissRequest = { dialogo = null }, title = { Text("¿Bloquear a $nombre en el bot?") },
            text = { Text("El bot la ignorará por completo: no le contesta ni le atiende comandos. No es un bloqueo de WhatsApp: ella puede seguir escribiéndote y tú verás sus mensajes. Puedes desbloquearla cuando quieras.") },
            confirmButton = { TextButton({ dialogo = null; hacer("Listo: el bot la ignora") { Repo.bloquearPersona(botId, jid) } }, Modifier.testTag("confirmar_bloquear")) { Text("Bloquear", color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton({ dialogo = null }) { Text("Cancelar") } },
        )
        "advertir" -> DialogoAdvertir(botId, jid, nombre) { dialogo = null; version++ }
        "exportar" -> DialogoExportar(botId, jid, nombre) { dialogo = null }
        "olvidar" -> DialogoOlvidar(botId, jid, nombre, { dialogo = null }, alOlvidada)
    }
}

/** Una etiqueta pequeña con su color. */
@Composable
fun ChipEtiqueta(e: com.botchat.app.datos.EtiquetaItem, modifier: Modifier = Modifier) {
    val color = colorDeRango(e.color)
    Row(
        modifier.clip(RoundedCornerShape(50)).background(color.copy(alpha = 0.18f)).padding(horizontal = 8.dp, vertical = 2.dp).testTag("etiqueta_${e.id}"),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        PuntoRango(e.color, 8)
        Spacer(Modifier.width(4.dp))
        Text(e.nombre, style = MaterialTheme.typography.labelSmall, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

@Composable
private fun DialogoSilenciar(nombre: String, alCerrar: () -> Unit, alElegir: (minutos: Int, motivo: String) -> Unit) {
    var minutos by remember { mutableIntStateOf(60) }
    AlertDialog(
        onDismissRequest = alCerrar, title = { Text("Silenciar al bot para $nombre") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("El bot no le contestará durante este tiempo (tampoco comandos). Tú puedes seguir hablando con ella.", style = MaterialTheme.typography.bodySmall)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    DURACIONES_SILENCIO.forEach { d -> FilterChip(minutos == d, { minutos = d }, { Text(textoDuracionSilencio(d), maxLines = 1) }, modifier = Modifier.testTag("silencio_$d")) }
                }
            }
        },
        confirmButton = { TextButton({ alElegir(minutos, "") }, Modifier.testTag("confirmar_silenciar")) { Text("Silenciar") } },
        dismissButton = { TextButton(alCerrar) { Text("Cancelar") } },
    )
}

internal val MOTIVOS_ADVERTENCIA = listOf("spam" to "Spam", "insultos" to "Insultos", "publicidad" to "Publicidad", "fuera_de_tema" to "Fuera de tema", "sin_motivo" to "Sin motivo")

/** Pone una advertencia con un motivo rápido (para uno propio, abre “Notas y advertencias”). Si llega al tope, la regla elegida actúa y se explica aquí. */
@Composable
fun DialogoAdvertir(botId: Long, persona: String, nombre: String, chat: String = "", alCerrar: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var motivo by remember { mutableStateOf("sin_motivo") }
    var trabajando by remember { mutableStateOf(false) }
    var resultado by remember { mutableStateOf<com.botchat.app.datos.ResultadoAdvertencia?>(null) }
    val r = resultado
    if (r != null) { DialogoResultadoAdvertencia(r, nombre, alCerrar); return }
    AlertDialog(
        onDismissRequest = alCerrar, title = { Text("Advertir a $nombre") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("Se anota la advertencia con la fecha. Si llega al tope que elegiste en Avanzado, el bot hace lo que le indicaste (y siempre te llega un aviso).", style = MaterialTheme.typography.bodySmall)
                Text("Motivo", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    MOTIVOS_ADVERTENCIA.forEach { (k, t) -> FilterChip(motivo == k, { motivo = k }, { Text(t, maxLines = 1) }, modifier = Modifier.testTag("motivo_adv_$k")) }
                }
            }
        },
        confirmButton = {
            TextButton({
                trabajando = true
                val texto = MOTIVOS_ADVERTENCIA.first { it.first == motivo }.let { if (it.first == "sin_motivo") "" else it.second }
                scope.launch { runCatching { Repo.advertir(botId, persona, texto, chat) }.onSuccess { resultado = it }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true); alCerrar() }; trabajando = false }
            }, Modifier.testTag("confirmar_advertir"), enabled = !trabajando) { Text("Advertir") }
        },
        dismissButton = { TextButton(alCerrar) { Text("Cancelar") } },
    )
}

/** Qué pasó al advertir: cuántas lleva y, si llegó al tope, lo que se hizo (o por qué no se pudo). */
@Composable
fun DialogoResultadoAdvertencia(r: com.botchat.app.datos.ResultadoAdvertencia, nombre: String, alCerrar: () -> Unit) {
    AlertDialog(
        onDismissRequest = alCerrar, title = { Text("Advertencia puesta") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("$nombre lleva ${r.total}" + (if (r.tope > 0) " de ${r.tope}" else "") + " advertencias.", Modifier.testTag("resultado_advertencia"))
                if (r.llegoAlTope) Text(textoResultadoTope(r), Modifier.testTag("resultado_tope"), color = if (r.hecho == "solo_aviso") MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurface)
            }
        },
        confirmButton = { TextButton(alCerrar, Modifier.testTag("cerrar_resultado_advertencia")) { Text("Listo") } },
    )
}

internal fun textoResultadoTope(r: com.botchat.app.datos.ResultadoAdvertencia): String = when (r.hecho) {
    "silenciado" -> "Llegó al tope: el bot ya no le contesta."
    "borrado" -> "Llegó al tope: se borró su mensaje."
    "expulsado" -> "Llegó al tope: se la expulsó del grupo."
    "solo_aviso" -> "Llegó al tope. No hice la acción (${r.motivo}); solo te avisé para que decidas tú."
    else -> "Llegó al tope: te llegó un aviso."
}

/** Crea el archivo cifrado (BCF1) con los datos de la persona y deja compartirlo o guardar una copia legible. */
@Composable
private fun DialogoExportar(botId: Long, persona: String, nombre: String, alCerrar: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var ruta by remember { mutableStateOf<String?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(persona) { runCatching { Repo.exportarPersona(botId, persona) }.onSuccess { ruta = it }.onFailure { error = it.message ?: "No se pudo exportar" } }
    AlertDialog(
        onDismissRequest = alCerrar, title = { Text("Exportar datos de $nombre") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                when {
                    error != null -> Text(error!!, color = MaterialTheme.colorScheme.error, modifier = Modifier.testTag("error_exportar"))
                    ruta == null -> Row(verticalAlignment = Alignment.CenterVertically) { CircularProgressIndicator(Modifier.size(24.dp)); Spacer(Modifier.width(Esp.m)); Text("Preparando el archivo…") }
                    else -> {
                        Text("El archivo lleva sus mensajes, notas, advertencias y estadísticas, cifrado con la llave de esta app. Se borra solo en 24 horas.", style = MaterialTheme.typography.bodySmall, modifier = Modifier.testTag("exportar_listo"))
                        OutlinedButton({ scope.launch { compartirCifrado(ctx, ruta!!, nombre) } }, Modifier.fillMaxWidth().testTag("compartir_exporte")) { Text("Compartir archivo cifrado") }
                        OutlinedButton({
                            scope.launch {
                                val r = Exportar.guardarCopia(ctx, Repo.archivo(ruta!!), "Exportes", Temporales.nombreSeguro("datos_$nombre") + ".json", "application/json")
                                avisaToast(ctx, r.fold({ "Copia guardada en $it. Esa copia ya NO está cifrada." }, { "No se pudo guardar la copia: ${it.message}" }), true)
                            }
                        }, Modifier.fillMaxWidth().testTag("guardar_copia_exporte")) { Text("Guardar copia legible (sin cifrar)") }
                    }
                }
            }
        },
        confirmButton = { TextButton(alCerrar, Modifier.testTag("cerrar_exportar")) { Text("Cerrar") } },
    )
}

/** Copia el archivo cifrado a la carpeta de adjuntos de la caché y abre el selector de apps para compartirlo. */
internal suspend fun compartirCifrado(ctx: Context, ruta: String, nombre: String, extension: String = "bcf") {
    try {
        val f = withContext(Dispatchers.IO) {
            val dir = File(ctx.cacheDir, "adjuntos").apply { mkdirs() }
            File(dir, Temporales.nombreSeguro("datos_$nombre") + "." + extension).also { Repo.archivo(ruta).copyTo(it, overwrite = true) }
        }
        val uri = androidx.core.content.FileProvider.getUriForFile(ctx, ctx.packageName + ".compartir", f)
        val i = Intent(Intent.ACTION_SEND).setType("application/octet-stream").putExtra(Intent.EXTRA_STREAM, uri).addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        ctx.startActivity(Intent.createChooser(i, "Compartir datos exportados").addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
    } catch (e: Exception) {
        avisaToast(ctx, "No se pudo preparar el archivo: ${e.message}", true)
    }
}

/** Olvidar: borra para siempre todo lo de la persona. Pide marcar que se entiende antes de dejar tocar el botón. */
@Composable
private fun DialogoOlvidar(botId: Long, persona: String, nombre: String, alCerrar: () -> Unit, alOlvidada: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var entiendo by remember { mutableStateOf(false) }
    var trabajando by remember { mutableStateOf(false) }
    AlertDialog(
        onDismissRequest = { if (!trabajando) alCerrar() }, title = { Text("¿Olvidar a $nombre?") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text("Se borran para siempre, de este teléfono: su chat completo, los mensajes que escribió en grupos, sus archivos, notas, advertencias, conversaciones de menú y sus números guardados.", Modifier.testTag("texto_olvidar"))
                Text("Si estaba en la lista No molestar o bloqueada en el bot, se mantiene (solo guarda su número) para respetar su decisión. No se borra nada en WhatsApp.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                    Checkbox(entiendo, { entiendo = it }, Modifier.testTag("entiendo_olvidar"))
                    Text("Entiendo que no se puede deshacer", style = MaterialTheme.typography.bodyMedium)
                }
            }
        },
        confirmButton = {
            TextButton({
                trabajando = true
                scope.launch {
                    runCatching { Repo.olvidarPersona(botId, persona) }
                        .onSuccess { avisaToast(ctx, "Listo: la olvidé (${plural(it.mensajes.toLong(), "mensaje borrado", "mensajes borrados")})", false); alCerrar(); alOlvidada() }
                        .onFailure { avisaToast(ctx, it.message ?: "No se pudo", true); trabajando = false }
                }
            }, Modifier.testTag("confirmar_olvidar"), enabled = entiendo && !trabajando) { Text("Olvidar", color = if (entiendo) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurfaceVariant) }
        },
        dismissButton = { TextButton(alCerrar, enabled = !trabajando) { Text("Cancelar") } },
    )
}
