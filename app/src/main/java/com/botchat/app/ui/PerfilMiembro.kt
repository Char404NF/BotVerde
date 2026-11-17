@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.PrimaryTabRow
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Tab
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
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.EstadoGrupo
import com.botchat.app.datos.PerfilMiembroAdmin
import com.botchat.app.datos.Repo
import com.botchat.app.datos.nombreFiltroGrupo
import com.botchat.app.datos.reiniciarInfracciones
import com.botchat.app.datos.banear
import com.botchat.app.datos.desbanear
import com.botchat.app.datos.perfilMiembro
import com.botchat.app.datos.quitarSilencioGrupos
import com.botchat.app.datos.silenciarEnGrupos
import kotlinx.coroutines.launch

private val PESTANAS_PERFIL = listOf("Resumen", "Estadísticas", "Auditoría", "Historial")

/**
 * Todo sobre una persona dentro de ESTE grupo: cuánto y qué envió, cuándo, cómo se compara con los demás, lo que borró o editó, su
 * historial de moderación, y los botones para darle rango, advertirla, silenciarla, sacarla o banearla.
 */
@Composable
fun PantallaPerfilMiembro(botId: Long, jid: String, persona: String, alVolver: () -> Unit, alAuditar: (nombre: String) -> Unit = {}) {
    var pestana by rememberSaveable { mutableIntStateOf(0) }
    var version by remember { mutableIntStateOf(0) }
    val haptica = rememberHaptica()
    LaunchedEffect(jid, persona) { Repo.cambios.collect { evs -> if (evs.any { it == "grupo:$jid" || it == "personas" || it.startsWith("silenc") || it.startsWith("advert") || it == "presencia:$persona" }) version++ } }
    val perfil by produceState<PerfilMiembroAdmin?>(null, jid, persona, version) { value = runCatching { Repo.perfilMiembro(botId, jid, persona) }.getOrNull() ?: value }
    val estado by produceState<EstadoGrupo?>(null, jid) { value = runCatching { Repo.estadoGrupo(botId, jid) }.getOrNull() }
    val p = perfil
    var dialogo by remember { mutableStateOf<String?>(null) }
    Scaffold(
        topBar = {
            Column {
                TopAppBar(
                    navigationIcon = { IconButton(alVolver, Modifier.testTag("volver_perfil_miembro")) { Icon(Ico.Atras, "Volver") } },
                    title = { Text("Perfil en el grupo") },
                )
                if (p != null) CabeceraPerfil(p)
                PrimaryTabRow(pestana) {
                    PESTANAS_PERFIL.forEachIndexed { i, t -> Tab(pestana == i, { haptica(Toque.Suave); pestana = i }, text = { Text(t, maxLines = 1) }, modifier = Modifier.testTag("perfil_pestana_$i")) }
                }
            }
        },
        bottomBar = { if (p != null) BarraAccionesMiembro(botId, jid, p, estado?.soyAdmin == true, { dialogo = it }) { version++ } },
    ) { relleno ->
        Box(Modifier.fillMaxSize().padding(relleno)) {
            if (p == null) FilasDeCarga(4) else when (pestana) {
                0 -> PestanaResumenPerfil(botId, jid, p)
                1 -> PestanaEstadisticasPerfil(p)
                2 -> PestanaAuditoriaPerfil(p) { alAuditar(p.miembro.titulo) }
                else -> PestanaHistorialPerfil(p)
            }
            // Los cuadros de confirmar van aquí (no en la barra de abajo): son hojas que ocupan toda la pantalla.
            if (p != null) DialogosMiembro(botId, jid, p, dialogo, { dialogo = null }) { version++ }
        }
    }
}

@Composable
private fun CabeceraPerfil(p: PerfilMiembroAdmin) {
    val m = p.miembro
    Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s).testTag("cabecera_perfil"), verticalAlignment = Alignment.CenterVertically) {
        AvatarEnLinea(m, 72.dp)
        Spacer(Modifier.width(Esp.l))
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(m.titulo, Modifier.testTag("perfil_nombre"), style = MaterialTheme.typography.titleLarge, maxLines = 2, overflow = TextOverflow.Ellipsis)
            if (m.telefono.isNotBlank() && m.telefono != m.titulo) Text(m.telefono, Modifier.testTag("perfil_telefono"), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            val linea = textoEnLinea(m)
            if (linea.isNotBlank()) Text(linea, Modifier.testTag("perfil_en_linea"), style = MaterialTheme.typography.bodySmall, color = if (m.enLinea) VERDE_EN_LINEA else MaterialTheme.colorScheme.onSurfaceVariant)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.xs), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                InsigniaCargo(m, Modifier.testTag("perfil_cargo"))
                if (m.rango.isNotBlank()) InsigniaRango(m.rango, m.rangoColor, Modifier.testTag("perfil_rango"))
                if (m.baneado) Etiquetita("Baneada", ROJO_SILENCIO, "perfil_baneada")
                if (p.silenciado) Etiquetita("Silenciada", ROJO_SILENCIO, "perfil_silenciada")
                if (m.advertencias > 0) Etiquetita(plural(m.advertencias.toLong(), "advertencia", "advertencias"), AMARILLO_SILENCIO, "perfil_advertencias")
            }
        }
    }
}

@Composable
private fun Etiquetita(texto: String, color: androidx.compose.ui.graphics.Color, etiqueta: String) {
    Text(
        texto, Modifier.background(color.copy(alpha = 0.16f), androidx.compose.foundation.shape.RoundedCornerShape(50)).padding(horizontal = 8.dp, vertical = 2.dp).testTag(etiqueta),
        style = MaterialTheme.typography.labelMedium, color = color,
    )
}

@Composable
private fun Dato(icono: androidx.compose.ui.graphics.vector.ImageVector, valor: Int, nombre: String, modifier: Modifier = Modifier) {
    Card(modifier, colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Row(Modifier.fillMaxWidth().padding(horizontal = Esp.m, vertical = Esp.s), verticalAlignment = Alignment.CenterVertically) {
            Icon(icono, null, Modifier.size(20.dp), tint = MaterialTheme.colorScheme.primary)
            Spacer(Modifier.width(Esp.s))
            Column {
                Text(numero(valor.toLong()), style = MaterialTheme.typography.titleMedium)
                Text(nombre, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }
}

@Composable
private fun PestanaResumenPerfil(botId: Long, jid: String, p: PerfilMiembroAdmin) {
    val c = p.miembro.conteo
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(vertical = Esp.s).testTag("perfil_resumen"), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
        Column(Modifier.padding(horizontal = Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Text(textoUltimoMensaje(c), Modifier.testTag("perfil_ultimo"), style = MaterialTheme.typography.bodyLarge)
            if (p.puesto > 0 && p.puesto <= p.activos) Text("Puesto ${p.puesto} de ${p.activos} que han escrito", Modifier.testTag("perfil_puesto"), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            if (p.miembro.desdeTs > 0) Text("En el grupo desde ${fechaLarga(p.miembro.desdeTs)}", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                Dato(Ico.Chats, c.mensajes, "mensajes", Modifier.weight(1f).testTag("dato_mensajes"))
                Dato(Ico.Chats, c.texto, "de texto", Modifier.weight(1f).testTag("dato_texto"))
            }
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                Dato(Ico.Foto, c.imagenes, "imágenes", Modifier.weight(1f).testTag("dato_imagenes"))
                Dato(Ico.Video, c.videos, "videos", Modifier.weight(1f).testTag("dato_videos"))
            }
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                Dato(IcoNota, c.audios, "audios", Modifier.weight(1f).testTag("dato_audios"))
                Dato(Ico.Microfono, c.notasVoz, "notas de voz", Modifier.weight(1f).testTag("dato_notas"))
            }
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                Dato(Ico.Sticker, c.stickers, "stickers", Modifier.weight(1f).testTag("dato_stickers"))
                Dato(Ico.Documento, c.documentos, "documentos", Modifier.weight(1f).testTag("dato_documentos"))
            }
            if (p.silenciado) Text("Silenciada ${textoDuracionFin(p.silencioHasta)}: el bot borra lo que escriba.", Modifier.testTag("perfil_silencio"), style = MaterialTheme.typography.bodyMedium, color = ROJO_SILENCIO)
            if (p.miembro.baneado) Text("Baneada" + (if (p.motivoBan.isNotBlank()) ": ${p.motivoBan}" else ""), Modifier.testTag("perfil_motivo_ban"), style = MaterialTheme.typography.bodyMedium, color = ROJO_SILENCIO)
            if (p.infracciones.isNotEmpty()) InfraccionesPerfil(botId, jid, p)
        }
        TituloSeccion("Rango en el bot")
        TarjetaRango(botId, p.miembro.persona, p.miembro.titulo)
    }
}

@Composable
private fun PestanaEstadisticasPerfil(p: PerfilMiembroAdmin) {
    val total = p.serie.sumOf { it.n }
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(Esp.l).testTag("perfil_estadisticas"), verticalArrangement = Arrangement.spacedBy(Esp.l)) {
        NotaDesde(p.desde, p.desde > 0)
        SeccionEst("Mensajes por día (30 días)") {
            GraficoBarras(p.serie.map { it.n.toLong() }, p.serie.mapIndexed { i, d -> if (i % 5 == 0) d.dia.takeLast(2).trimStart('0') else "" }, "Mensajes por día en los últimos 30 días", alto = 130.dp)
            Text("${plural(total.toLong(), "mensaje", "mensajes")} en 30 días · ${"%.1f".format(java.util.Locale.forLanguageTag("es"), p.promedioDia)} por día", Modifier.testTag("perfil_promedio"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        SeccionEst("A qué hora escribe") {
            GraficoBarras(p.horas.map { it.toLong() }, etiquetasHoras(), "Mensajes por hora del día", alto = 110.dp)
        }
        SeccionEst("Comparado con el grupo") {
            val m = p.miembro.conteo.mensajes
            val media = p.mediaGrupo
            val maximo = maxOf(m.toDouble(), media, 1.0)
            Text("Esta persona: ${numero(m.toLong())}", style = MaterialTheme.typography.bodyMedium)
            BarraRelativa((m / maximo).toFloat())
            Text("Promedio de quienes escriben: ${"%.0f".format(java.util.Locale.forLanguageTag("es"), media)}", style = MaterialTheme.typography.bodyMedium)
            BarraRelativa((media / maximo).toFloat())
        }
        SeccionEst("Lo que envía") {
            val c = p.miembro.conteo
            val filas = listOf(
                Triple(Ico.Chats, "Texto", c.texto), Triple(Ico.Foto, "Imágenes", c.imagenes), Triple(Ico.Video, "Videos", c.videos),
                Triple(IcoNota, "Audios", c.audios), Triple(Ico.Microfono, "Notas de voz", c.notasVoz), Triple(Ico.Sticker, "Stickers", c.stickers), Triple(Ico.Documento, "Documentos", c.documentos),
            )
            val mayor = (filas.maxOfOrNull { it.third } ?: 0).coerceAtLeast(1)
            filas.forEach { (ic, nombre, n) ->
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Icon(ic, null, Modifier.size(18.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
                    Spacer(Modifier.width(Esp.s))
                    Text(nombre, Modifier.width(100.dp), style = MaterialTheme.typography.bodyMedium)
                    Box(Modifier.weight(1f)) { BarraRelativa(n.toFloat() / mayor) }
                    Text(numero(n.toLong()), Modifier.width(48.dp).padding(start = Esp.s), style = MaterialTheme.typography.labelLarge)
                }
            }
        }
    }
}

@Composable
private fun PestanaAuditoriaPerfil(p: PerfilMiembroAdmin, alAuditar: () -> Unit) {
    val c = p.miembro.conteo
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(Esp.l).testTag("perfil_auditoria"), verticalArrangement = Arrangement.spacedBy(Esp.l)) {
        Text("Revisa todo lo que esta persona ha enviado a este grupo, incluso lo que borró o editó, si el bot alcanzó a verlo.", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
            Dato(Ico.Chats, c.mensajes, "mensajes enviados", Modifier.weight(1f).testTag("aud_enviados"))
            Dato(Ico.Borrar, c.borrados, "borrados", Modifier.weight(1f).testTag("aud_borrados"))
        }
        Dato(Ico.Editar, c.editados, "editados", Modifier.fillMaxWidth().testTag("aud_editados"))
        androidx.compose.material3.Button(alAuditar, Modifier.fillMaxWidth().testTag("ver_todo_lo_que_envio")) { Icon(Ico.Buscar, null, Modifier.size(18.dp)); Spacer(Modifier.width(Esp.s)); Text("Ver todo lo que envió") }
        Text("Desde ahí puedes ir al mensaje en el chat o borrarlo del grupo (si el bot es administrador).", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

@Composable
private fun PestanaHistorialPerfil(p: PerfilMiembroAdmin) {
    if (p.historial.isEmpty()) Box(Modifier.fillMaxSize(), Alignment.Center) {
        EstadoVacio(Ico.Reloj, "Sin movimientos", "Aquí queda cuándo entró, y las advertencias, silencios o expulsiones que reciba.", Modifier.testTag("perfil_historial_vacio"))
    } else ListaEventos(p.historial, Modifier.fillMaxSize().testTag("perfil_historial"))
}

// ---------- acciones ----------

@Composable
private fun BarraAccionesMiembro(botId: Long, jid: String, p: PerfilMiembroAdmin, soyAdmin: Boolean, pedir: (String) -> Unit, refrescar: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    val m = p.miembro
    var menuSilencio by remember { mutableStateOf(false) }
    fun hacer(ok: String, f: suspend () -> Unit) = scope.launch {
        runCatching { f() }.onSuccess { avisaToast(ctx, ok, false) }.onFailure { avisaToast(ctx, textoError(it), true) }
        refrescar()
    }
    Surface(tonalElevation = 3.dp, modifier = Modifier.testTag("barra_acciones_miembro")) {
        Row(Modifier.horizontalScroll(rememberScrollState()).navigationBarsPadding().padding(horizontal = Esp.s, vertical = Esp.s), horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalAlignment = Alignment.CenterVertically) {
            OutlinedButton({ haptica(Toque.Suave); pedir("advertir") }, Modifier.testTag("accion_advertir")) { Icon(Ico.Aviso, null, Modifier.size(18.dp)); Spacer(Modifier.width(4.dp)); Text("Advertir") }
            Box {
                OutlinedButton({ haptica(Toque.Suave); if (p.silenciado) pedir("quitar_silencio") else menuSilencio = true }, Modifier.testTag("accion_silenciar")) {
                    Icon(Ico.Volumen, null, Modifier.size(18.dp)); Spacer(Modifier.width(4.dp)); Text(if (p.silenciado) "Quitar silencio" else "Silenciar")
                }
                DropdownMenu(menuSilencio, { menuSilencio = false }) {
                    listOf("15 minutos" to 900L, "1 hora" to 3600L, "1 día" to 86400L, "Siempre" to 0L).forEach { (t, s) ->
                        androidx.compose.material3.DropdownMenuItem(text = { Text(t) }, onClick = {
                            menuSilencio = false
                            hacer("Listo: la silencié ($t)") { Repo.silenciarEnGrupos(botId, m.persona, s, "uno", listOf(jid), true, "") }
                        }, modifier = Modifier.testTag("silencio_$s"))
                    }
                }
            }
            OutlinedButton({ haptica(Toque.Suave); pedir(if (m.esAdmin) "quitar_admin" else "hacer_admin") }, Modifier.testTag("accion_admin"), enabled = soyAdmin && !m.esCreador) {
                Icon(Ico.Candado, null, Modifier.size(18.dp)); Spacer(Modifier.width(4.dp)); Text(if (m.esAdmin) "Quitar admin" else "Hacer admin")
            }
            OutlinedButton({ haptica(Toque.Fuerte); pedir("expulsar") }, Modifier.testTag("accion_expulsar"), enabled = soyAdmin && !m.esCreador) { Text("Expulsar", color = MaterialTheme.colorScheme.error) }
            OutlinedButton({ haptica(Toque.Fuerte); pedir(if (m.baneado) "desbanear" else "banear") }, Modifier.testTag("accion_banear"), enabled = soyAdmin || m.baneado) {
                Text(if (m.baneado) "Quitar baneo" else "Banear", color = if (m.baneado) MaterialTheme.colorScheme.onSurface else MaterialTheme.colorScheme.error)
            }
        }
    }
}

@Composable
private fun DialogosMiembro(botId: Long, jid: String, p: PerfilMiembroAdmin, dialogo: String?, cerrar: () -> Unit, refrescar: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val m = p.miembro
    fun hacer(ok: String, f: suspend () -> Unit) = scope.launch {
        runCatching { f() }.onSuccess { avisaToast(ctx, ok, false) }.onFailure { avisaToast(ctx, textoError(it), true) }
        refrescar()
    }
    val dialogoActual = dialogo
    when (dialogoActual) {
        "advertir" -> HojaTexto(
            "Advertir a ${m.titulo}", "", "motivo_advertir", { cerrar() },
            { t -> cerrar(); hacer("Advertencia puesta") { Repo.advertir(botId, m.persona, t, jid) } },
            ayuda = "Se anota en su ficha. Si llega al tope, el bot hace lo que tengas configurado (por ejemplo silenciar).", permiteVacio = true, maximo = 200, lineas = 2,
        )
        "quitar_silencio" -> AlertDialog(
            onDismissRequest = { cerrar() }, title = { Text("¿Quitar el silencio?") }, text = { Text("El bot dejará de borrar lo que escriba ${m.titulo}.") },
            confirmButton = { TextButton({ cerrar(); hacer("Listo: ya puede escribir") { Repo.quitarSilencioGrupos(botId, m.persona) } }, Modifier.testTag("confirmar_quitar_silencio")) { Text("Quitar silencio") } },
            dismissButton = { TextButton({ cerrar() }) { Text("Cancelar") } },
        )
        "hacer_admin" -> AlertDialog(
            onDismissRequest = { cerrar() }, title = { Text("¿Hacer admin a ${m.titulo}?") }, text = { Text("Podrá cambiar ajustes, agregar y sacar personas en WhatsApp.") },
            confirmButton = { TextButton({ cerrar(); hacer("Listo: ahora es administradora") { Repo.ascenderEnGrupo(botId, jid, m.persona, true) } }, Modifier.testTag("confirmar_admin")) { Text("Hacer admin") } },
            dismissButton = { TextButton({ cerrar() }) { Text("Cancelar") } },
        )
        "quitar_admin" -> AlertDialog(
            onDismissRequest = { cerrar() }, title = { Text("¿Quitar el cargo de admin?") }, text = { Text("${m.titulo} ya no podrá administrar el grupo.") },
            confirmButton = { TextButton({ cerrar(); hacer("Listo: ya no es administradora") { Repo.ascenderEnGrupo(botId, jid, m.persona, false) } }, Modifier.testTag("confirmar_quitar_admin")) { Text("Quitar admin") } },
            dismissButton = { TextButton({ cerrar() }) { Text("Cancelar") } },
        )
        "expulsar" -> ConfirmarSinDeshacer(
            "¿Expulsar a ${m.titulo}?", "Sale del grupo ahora. Podría volver si tiene el enlace; para impedirlo, mejor banéala.", "Expulsar", "confirmar_expulsar",
            { cerrar() }, { cerrar(); hacer("Listo: salió del grupo") { Repo.expulsarDeGrupo(botId, jid, m.persona) } },
        )
        "banear" -> HojaTexto(
            "Banear a ${m.titulo}", "", "motivo_banear", { cerrar() },
            { t -> cerrar(); hacer("Listo: baneada") { Repo.banear(botId, jid, m.persona, t) } },
            ayuda = "Sale del grupo y, si vuelve a entrar, la saco otra vez. El motivo es solo para ti (opcional).", permiteVacio = true, maximo = 200, lineas = 2,
        )
        "desbanear" -> AlertDialog(
            onDismissRequest = { cerrar() }, title = { Text("¿Dejar volver a ${m.titulo}?") }, text = { Text("Podrá entrar otra vez al grupo.") },
            confirmButton = { TextButton({ cerrar(); hacer("Listo: ya puede volver") { Repo.desbanear(botId, jid, m.persona) } }, Modifier.testTag("confirmar_desbanear_perfil")) { Text("Quitar baneo") } },
            dismissButton = { TextButton({ cerrar() }) { Text("Cancelar") } },
        )
    }
}

/** Cuántas infracciones lleva la persona en cada filtro del grupo, con botón para ponerle el contador en cero. */
@Composable
private fun InfraccionesPerfil(botId: Long, jid: String, p: PerfilMiembroAdmin) {
    val scope = rememberCoroutineScope()
    val ctx = androidx.compose.ui.platform.LocalContext.current
    var lista by remember(p.infracciones) { mutableStateOf(p.infracciones) }
    if (lista.isEmpty()) return
    Column(Modifier.fillMaxWidth().testTag("perfil_infracciones"), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
        Text("Infracciones de filtros", style = MaterialTheme.typography.titleSmall)
        lista.forEach { x ->
            Text("${nombreFiltroGrupo(x.filtro)}: ${plural(x.total.toLong(), "infracción", "infracciones")}", Modifier.testTag("perfil_infraccion_${x.filtro}"), style = MaterialTheme.typography.bodyMedium)
        }
        TextButton({
            scope.launch {
                runCatching { Repo.reiniciarInfracciones(botId, jid, p.miembro.persona) }
                    .onSuccess { lista = emptyList() }.onFailure { avisaToast(ctx, textoError(it), true) }
            }
        }, Modifier.testTag("perfil_reiniciar_infracciones")) { Text("Reiniciar el contador") }
    }
}
