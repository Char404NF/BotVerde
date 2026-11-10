@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.PrimaryTabRow
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Tab
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.media3.common.util.UnstableApi
import com.botchat.app.datos.CalendarioMes
import com.botchat.app.datos.ChatItem
import com.botchat.app.datos.Msg
import com.botchat.app.datos.Repo
import com.botchat.app.datos.ResumenChat
import com.botchat.app.medios.CacheImagenes
import com.botchat.app.medios.ReproductorAudio
import kotlinx.coroutines.launch
import java.time.Instant
import java.time.LocalDate
import java.time.YearMonth
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import kotlin.math.sqrt

private val PESTANAS_INFO = listOf("Resumen", "Calendario", "Multimedia")

/** Info del chat: resumen de números, calendario de actividad y multimedia por fecha. Todo sale de contadores; nunca se lee el texto de los mensajes. */
@OptIn(UnstableApi::class)
@Composable
fun PantallaInfoChat(botId: Long, jid: String, alVolver: () -> Unit, alAbrirVisor: (String) -> Unit, alAbrirDocumento: (String) -> Unit) {
    val scope = rememberCoroutineScope()
    val pager = rememberPagerState { PESTANAS_INFO.size }
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(jid) { Repo.cambios.collect { evs -> if (evs.any { it == "chat:$jid" || it.startsWith("archivo:$jid:") || it == "estadisticas" }) version++ } }
    val chat by produceState<ChatItem?>(null, jid) { value = runCatching { Repo.chat(botId, jid) }.getOrNull() }
    DisposableEffect(Unit) { onDispose { ReproductorAudio.detener(); CacheImagenes.vaciar() } }
    Scaffold(
        topBar = {
            Column {
                TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Info del chat") })
                CabeceraInfo(chat, jid)
                PrimaryTabRow(pager.currentPage) {
                    PESTANAS_INFO.forEachIndexed { i, titulo -> Tab(pager.currentPage == i, { haptica(Toque.Suave); scope.launch { pager.animateScrollToPage(i) } }, text = { Text(titulo) }) }
                }
            }
        },
    ) { relleno ->
        HorizontalPager(pager, Modifier.fillMaxSize().padding(relleno)) { pagina ->
            when (pagina) {
                0 -> TabResumen(botId, jid, version)
                1 -> TabCalendario(botId, jid, version, alAbrirVisor, alAbrirDocumento)
                else -> TabMultimedia(botId, jid, version, alAbrirVisor, alAbrirDocumento)
            }
        }
    }
}

@Composable
private fun CabeceraInfo(chat: ChatItem?, jid: String) {
    Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s), verticalAlignment = Alignment.CenterVertically) {
        Avatar(chat?.nombre ?: "", chat?.foto ?: "", chat?.fotoTs ?: 0, jid, 56.dp)
        Spacer(Modifier.width(Esp.m))
        Column {
            Text(chat?.nombre ?: "", style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.testTag("info_nombre"))
            if (chat != null) TextoUnaLinea(if (chat.esGrupo) "Grupo" else banderaTelefono(chat), estilo = MaterialTheme.typography.bodyMedium)
        }
    }
}

// ---------- Resumen ----------

@Composable
private fun TabResumen(botId: Long, jid: String, version: Int) {
    val r by produceState<ResumenChat?>(null, jid, version) { value = runCatching { Repo.estadisticasChat(botId, jid) }.getOrNull() }
    val datos = r
    if (datos == null) { Box(Modifier.fillMaxSize(), Alignment.Center) { CircularProgressIndicator() }; return }
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
        NotaDesde(datos.desde, datos.listo)
        if (datos.enviados + datos.recibidos == 0L) {
            EstadoVacio(Ico.Estadisticas, "Sin números todavía", "Cuando haya mensajes en este chat, aquí verás cuántos se enviaron y recibieron.", Modifier.padding(top = Esp.xl))
            return@Column
        }
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(Esp.m)) {
            Cifra(datos.enviados, "Enviados", Modifier.weight(1f).testTag("cifra_enviados"))
            Cifra(datos.recibidos, "Recibidos", Modifier.weight(1f).testTag("cifra_recibidos"))
        }
        SeccionEst("Por tipo de mensaje") { datos.porTipo.forEach { FilaTipo(it) } }
        SeccionEst("A qué hora se escribe") {
            GraficoBarras(datos.porHora, etiquetasHoras(), "Mensajes por hora del día. Hora con más mensajes: ${datos.porHora.indices.maxByOrNull { datos.porHora[it] }}:00", Modifier.testTag("grafico_hora"))
        }
        SeccionEst("Qué días de la semana") {
            GraficoBarras(datos.porSemana, ETIQUETAS_SEMANA, "Mensajes por día de la semana. Día con más mensajes: ${NOMBRES_SEMANA[datos.porSemana.indices.maxByOrNull { datos.porSemana[it] } ?: 0]}", Modifier.testTag("grafico_semana"))
        }
        if (datos.quien.isNotEmpty()) {
            SeccionEst("Quién escribe más") {
                val mayor = datos.quien.first().cantidad.coerceAtLeast(1)
                datos.quien.forEach { q ->
                    Column(Modifier.testTag("quien")) {
                        Row { Text(q.nombre, Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis); Text(numero(q.cantidad), color = MaterialTheme.colorScheme.onSurfaceVariant) }
                        BarraRelativa(q.cantidad.toFloat() / mayor, Modifier.padding(top = 4.dp))
                    }
                }
            }
        }
        Text("Los mensajes anteriores a esta versión se cuentan con la zona horaria actual del teléfono.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

// ---------- Calendario ----------

private fun diaLegible(dia: String): String =
    primeraMayuscula(DateTimeFormatter.ofPattern("EEEE d 'de' MMMM", LocaleEs).format(LocalDate.parse(dia)))

@Composable
private fun TabCalendario(botId: Long, jid: String, version: Int, alAbrirVisor: (String) -> Unit, alAbrirDocumento: (String) -> Unit) {
    var mes by rememberSaveable(stateSaver = androidx.compose.runtime.saveable.Saver<YearMonth, String>({ it.toString() }, { YearMonth.parse(it) })) { mutableStateOf(YearMonth.now()) }
    var diaSel by rememberSaveable { mutableStateOf<String?>(null) }
    val cal by produceState<CalendarioMes?>(null, jid, mes, version) { value = runCatching { Repo.calendario(botId, jid, mes.toString()) }.getOrNull() }
    val c = cal
    val primerMes = c?.desde?.takeIf { it > 0 }?.let { YearMonth.from(Instant.ofEpochSecond(it).atZone(ZoneId.systemDefault())) }
    val hoy = YearMonth.now()
    val cuentas = remember(c) { c?.dias?.associate { it.dia to it.cantidad } ?: emptyMap() }
    val maximo = (cuentas.values.maxOrNull() ?: 0L).coerceAtLeast(1)
    val haptica = rememberHaptica()
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            IconButton({ haptica(Toque.Suave); mes = mes.minusMonths(1); diaSel = null }, enabled = primerMes != null && mes > primerMes, modifier = Modifier.testTag("mes_anterior")) { Icon(Ico.Anterior, "Mes anterior") }
            Text(primeraMayuscula(DateTimeFormatter.ofPattern("MMMM yyyy", LocaleEs).format(mes)), Modifier.weight(1f).testTag("titulo_mes"), textAlign = TextAlign.Center, style = MaterialTheme.typography.titleMedium)
            IconButton({ haptica(Toque.Suave); mes = mes.plusMonths(1); diaSel = null }, enabled = mes < hoy, modifier = Modifier.testTag("mes_siguiente")) { Icon(Ico.Siguiente, "Mes siguiente") }
        }
        Row(Modifier.fillMaxWidth()) {
            ETIQUETAS_SEMANA.forEach { Text(it, Modifier.weight(1f), textAlign = TextAlign.Center, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        }
        val desfase = mes.atDay(1).dayOfWeek.value - 1
        val filas = (desfase + mes.lengthOfMonth() + 6) / 7
        val esquema = MaterialTheme.colorScheme
        Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
            repeat(filas) { f ->
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                    repeat(7) { col ->
                        val n = f * 7 + col - desfase + 1
                        if (n < 1 || n > mes.lengthOfMonth()) {
                            Spacer(Modifier.weight(1f).aspectRatio(1f))
                        } else {
                            val fecha = mes.atDay(n)
                            val clave = fecha.toString()
                            val cantidad = cuentas[clave] ?: 0L
                            val intensidad = if (cantidad == 0L) 0f else 0.25f + 0.75f * sqrt(cantidad.toFloat() / maximo)
                            val fondo = if (cantidad == 0L) esquema.surfaceContainer else lerp(esquema.primaryContainer, esquema.primary, intensidad)
                            val tinta = if (cantidad > 0 && intensidad > 0.6f) esquema.onPrimary else if (cantidad > 0) esquema.onPrimaryContainer else esquema.onSurfaceVariant
                            Box(
                                Modifier.weight(1f).aspectRatio(1f).clip(RoundedCornerShape(10.dp)).background(fondo)
                                    .then(if (clave == diaSel) Modifier.border(BorderStroke(2.dp, esquema.onSurface), RoundedCornerShape(10.dp)) else Modifier)
                                    .clickable(enabled = cantidad > 0) { haptica(Toque.Suave); diaSel = clave }.testTag("dia_$clave"),
                                Alignment.Center,
                            ) { Text(n.toString(), color = tinta, style = MaterialTheme.typography.labelLarge) }
                        }
                    }
                }
            }
        }
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.Center, verticalAlignment = Alignment.CenterVertically) {
            Text("Menos", style = MaterialTheme.typography.labelSmall, color = esquema.onSurfaceVariant)
            listOf(0f, 0.25f, 0.5f, 0.75f, 1f).forEach { Box(Modifier.padding(horizontal = 2.dp).size(14.dp).clip(RoundedCornerShape(4.dp)).background(if (it == 0f) esquema.surfaceContainer else lerp(esquema.primaryContainer, esquema.primary, it))) }
            Text("Más", style = MaterialTheme.typography.labelSmall, color = esquema.onSurfaceVariant)
        }
        if (c != null && cuentas.isEmpty()) Text("No hubo mensajes en este mes.", style = MaterialTheme.typography.bodyMedium, color = esquema.onSurfaceVariant, modifier = Modifier.testTag("mes_vacio"))
        diaSel?.let { MensajesDelDia(botId, jid, it, version, alAbrirVisor, alAbrirDocumento) }
    }
}

@Composable
private fun MensajesDelDia(botId: Long, jid: String, dia: String, version: Int, alAbrirVisor: (String) -> Unit, alAbrirDocumento: (String) -> Unit) {
    val lista by produceState<List<Msg>?>(null, jid, dia, version) { value = runCatching { Repo.mensajesDia(botId, jid, dia) }.getOrDefault(emptyList()) }
    SeccionEst(diaLegible(dia), Modifier.testTag("lista_dia")) {
        val ms = lista
        when {
            ms == null -> CircularProgressIndicator(Modifier.size(24.dp), strokeWidth = 2.dp)
            ms.isEmpty() -> Text("No hay mensajes este día.")
            else -> ms.forEach { m ->
                val abrible = m.descargado && (m.categoria == "foto" || m.categoria == "video" || m.categoria == "documento")
                Row(
                    Modifier.fillMaxWidth().clickable(enabled = abrible) { if (m.categoria == "documento") alAbrirDocumento(m.id) else alAbrirVisor(m.id) }.padding(vertical = 2.dp),
                    verticalAlignment = Alignment.Top,
                ) {
                    Text(horaDe(m.ts), Modifier.width(48.dp), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Column(Modifier.weight(1f)) {
                        Text(if (m.esMio) "Bot" else m.remitente.ifBlank { "Contacto" }, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.primary)
                        Text(descripcionCorta(m.tipo, m.texto, m.borrado, m.notaVoz), maxLines = 3, overflow = TextOverflow.Ellipsis)
                    }
                }
            }
        }
    }
}

// ---------- Multimedia ----------

private val FILTROS = listOf("Imágenes" to "foto", "Videos" to "video", "Audio" to "audio", "Documentos" to "documento")

@OptIn(UnstableApi::class)
@Composable
private fun TabMultimedia(botId: Long, jid: String, version: Int, alAbrirVisor: (String) -> Unit, alAbrirDocumento: (String) -> Unit) {
    var categoria by rememberSaveable { mutableStateOf("foto") }
    val haptica = rememberHaptica()
    val lista by produceState<List<Msg>?>(null, jid, categoria, version) { value = runCatching { Repo.archivos(botId, jid, categoria, false, 1000) }.getOrDefault(emptyList()) }
    Column(Modifier.fillMaxSize()) {
        Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s), horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
            FILTROS.forEach { (titulo, cat) ->
                FilterChip(selected = categoria == cat, onClick = { haptica(Toque.Suave); categoria = cat }, label = { Text(titulo, maxLines = 1) }, modifier = Modifier.testTag("filtro_$cat"))
            }
        }
        val datos = lista
        when {
            datos == null -> Box(Modifier.fillMaxSize(), Alignment.Center) { CircularProgressIndicator() }
            datos.isEmpty() -> EstadoVacio(
                when (categoria) { "foto" -> Ico.Foto; "video" -> Ico.Video; "audio" -> Ico.Audio; else -> Ico.Documento },
                "Nada por aquí", "Cuando este chat tenga ${FILTROS.first { it.second == categoria }.first.lowercase()}, aparecerán aquí ordenados por fecha.", Modifier.padding(top = 60.dp),
            )
            categoria == "foto" || categoria == "video" -> Cuadricula(datos, alAbrirVisor, porFecha = true)
            categoria == "audio" -> ListaAudios(botId, jid, datos)
            else -> ListaDocumentos(botId, jid, datos, alAbrirDocumento)
        }
    }
}
