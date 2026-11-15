@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import android.content.ClipData
import android.content.ClipboardManager
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog as AlertaMaterial
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.PrimaryScrollableTabRow
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Switch
import androidx.compose.material3.Tab
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TimePicker
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.rememberTimePickerState
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.BaneadoGrupo
import com.botchat.app.datos.ChatItem
import com.botchat.app.datos.EstadoGrupo
import com.botchat.app.datos.EventoGrupo
import com.botchat.app.datos.HorarioGrupo
import com.botchat.app.datos.InfoGrupo
import com.botchat.app.datos.Repo
import com.botchat.app.datos.ProteccionEntrada
import com.botchat.app.datos.ResumenAdmin
import com.botchat.app.datos.entradaGrupo
import com.botchat.app.datos.guardarEntradaGrupo
import com.botchat.app.datos.baneadosGrupo
import com.botchat.app.datos.desbanear
import com.botchat.app.datos.guardarHorarioGrupo
import com.botchat.app.datos.historialGrupo
import com.botchat.app.datos.horarioGrupo
import com.botchat.app.datos.resumenAdmin
import kotlinx.coroutines.launch

/** Lo que la pantalla de administrar necesita para ir a otras pantallas (la navegación vive en Shell). */
class SalidasAdminGrupo(
    val alVolver: () -> Unit = {},
    val alAbrirPerfil: (persona: String) -> Unit = {},
    val alAbrirBaneados: () -> Unit = {},
    val alAbrirSolicitudes: () -> Unit = {},
    val alAbrirFiltros: () -> Unit = {},
    val alAbrirReglas: (nombre: String) -> Unit = {},
    val alAbrirAjustesAvanzados: () -> Unit = {},
    val alAbrirActividad: () -> Unit = {},
    val alAbrirAdvertencias: () -> Unit = {},
    val alAbrirSilencios: () -> Unit = {},
    val alAbrirBienvenida: () -> Unit = {},
    val alAbrirModulos: () -> Unit = {},
)

private val PESTANAS = listOf("Resumen", "Personas", "Reglas", "Mensajes", "Ajustes", "Historial")

/**
 * Administrar un grupo, todo en un lugar: Resumen (números y acciones rápidas), Personas (quién es quién y qué envió), Reglas, Mensajes,
 * Ajustes (cerrar, horario, baneados) e Historial. Lo que se cambia aquí vale solo para ESTE grupo.
 */
@Composable
fun PantallaAdminGrupo(botId: Long, jid: String, salidas: SalidasAdminGrupo, pestanaInicial: Int = 0) {
    var pestana by rememberSaveable { mutableIntStateOf(pestanaInicial) }
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    val chat by produceState<ChatItem?>(null, jid) { value = runCatching { Repo.chat(botId, jid) }.getOrNull() }
    val estado by produceState<EstadoGrupo?>(null, jid, version) { value = runCatching { Repo.estadoGrupo(botId, jid, version > 0) }.getOrElse { EstadoGrupo(false, false, false, textoError(it)) } }
    val info by produceState<InfoGrupo?>(null, jid, version) { value = runCatching { Repo.infoGrupo(botId, jid) }.getOrNull() }
    LaunchedEffect(jid) { Repo.cambios.collect { evs -> if (evs.any { it == "grupo:$jid" }) version++ } }
    val soyAdmin = estado?.soyAdmin == true
    Scaffold(topBar = {
        Column {
            TopAppBar(
                navigationIcon = { IconButton(salidas.alVolver, Modifier.testTag("volver_admin_grupo")) { Icon(Ico.Atras, "Volver") } },
                title = {
                    Column {
                        Text("Administrar", style = MaterialTheme.typography.titleLarge)
                        Text(chat?.nombre.orEmpty(), Modifier.testTag("admin_nombre_grupo"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                },
            )
            PrimaryScrollableTabRow(pestana, edgePadding = 0.dp) {
                PESTANAS.forEachIndexed { i, t -> Tab(pestana == i, { haptica(Toque.Suave); pestana = i }, text = { Text(t) }, modifier = Modifier.testTag("admin_pestana_$i")) }
            }
        }
    }) { relleno ->
        Box(Modifier.fillMaxSize().padding(relleno)) {
            when (pestana) {
                0 -> PestanaResumenAdmin(botId, jid, estado, info, soyAdmin, { version++ }, salidas) { pestana = it }
                1 -> PestanaPersonasAdmin(botId, jid, soyAdmin, salidas.alAbrirPerfil, salidas.alAbrirBaneados)
                2 -> PestanaReglasAdmin(botId, jid, chat?.nombre.orEmpty(), salidas)
                3 -> PestanaMensajesAdmin(botId, jid, estado, salidas)
                4 -> PestanaAjustesAdmin(botId, jid, estado, info, soyAdmin, { version++ }, salidas)
                else -> PestanaHistorialAdmin(botId, jid)
            }
        }
    }
}

// ---------- Resumen ----------

@Composable
private fun TarjetaNumero(icono: ImageVector, valor: Int, nombre: String, modifier: Modifier = Modifier, color: androidx.compose.ui.graphics.Color = MaterialTheme.colorScheme.primary, alPulsar: (() -> Unit)? = null) {
    Card(
        modifier.then(if (alPulsar != null) Modifier.clip(MaterialTheme.shapes.medium).clickable { alPulsar() } else Modifier),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
    ) {
        Column(Modifier.fillMaxWidth().padding(Esp.m), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Icon(icono, null, Modifier.size(20.dp), tint = color)
            Text(numero(valor.toLong()), style = MaterialTheme.typography.headlineSmall)
            Text(nombre, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

@Composable
private fun PestanaResumenAdmin(
    botId: Long, jid: String, estado: EstadoGrupo?, info: InfoGrupo?, soyAdmin: Boolean, refrescar: () -> Unit, salidas: SalidasAdminGrupo, irAPestana: (Int) -> Unit,
) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(jid) { Repo.cambios.collect { evs -> if (evs.any { it == "grupo:$jid" || it == "personas" || it.startsWith("silenc") || it.startsWith("advert") }) version++ } }
    val r by produceState<ResumenAdmin?>(null, jid, version) { value = runCatching { Repo.resumenAdmin(botId, jid) }.getOrNull() }
    var dialogo by remember { mutableStateOf<String?>(null) }
    var textoTodos by remember { mutableStateOf("") }
    fun hacer(ok: String, accion: suspend () -> Unit) = scope.launch {
        runCatching { accion() }.onSuccess { avisaToast(ctx, ok, false) }.onFailure { avisaToast(ctx, textoError(it), true) }
        refrescar(); version++
    }
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(Esp.l).testTag("pestana_resumen_admin"), verticalArrangement = Arrangement.spacedBy(Esp.l)) {
        TarjetaEstadoAdmin(estado) { haptica(Toque.Suave); refrescar() }
        val x = r
        if (x == null) FilasDeCarga(3) else {
            // alertas
            if (x.baneados > 0 || x.inactivos30 > 0) Card(Modifier.fillMaxWidth().testTag("resumen_avisos"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerHigh)) {
                Column(Modifier.padding(Esp.m), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                    if (x.inactivos30 > 0) Text("${plural(x.inactivos30.toLong(), "persona lleva", "personas llevan")} más de 30 días sin escribir.", style = MaterialTheme.typography.bodyMedium)
                    if (x.baneados > 0) Text("${plural(x.baneados.toLong(), "persona baneada", "personas baneadas")}.", style = MaterialTheme.typography.bodyMedium)
                }
            }
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    TarjetaNumero(Ico.Grupo, x.miembros, "miembros", Modifier.weight(1f).testTag("num_miembros")) { irAPestana(1) }
                    TarjetaNumero(Ico.Candado, x.admins, "administradores", Modifier.weight(1f).testTag("num_admins")) { irAPestana(1) }
                }
                Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    TarjetaNumero(Ico.Chats, x.mensajesHoy, "mensajes hoy", Modifier.weight(1f).testTag("num_hoy"))
                    TarjetaNumero(Ico.Estadisticas, x.activos7, "activos en 7 días", Modifier.weight(1f).testTag("num_activos"))
                }
                Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    TarjetaNumero(Ico.Reloj, x.inactivos30, "inactivos (30 días)", Modifier.weight(1f).testTag("num_inactivos"), AMARILLO_SILENCIO) { irAPestana(1) }
                    TarjetaNumero(Ico.Aviso, x.advertidos + x.silenciados, "advertidos o silenciados", Modifier.weight(1f).testTag("num_moderados"), ROJO_SILENCIO) { irAPestana(1) }
                }
            }
            SeccionEst("Mensajes de los últimos 14 días") {
                GraficoBarras(x.serie.map { it.n.toLong() }, x.serie.mapIndexed { i, p -> if (i % 2 == 0) p.dia.takeLast(2).trimStart('0') else "" }, "Mensajes por día en las últimas dos semanas", alto = 130.dp)
                Text("${plural(x.mensajes7.toLong(), "mensaje", "mensajes")} en 7 días", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            if (x.top.isNotEmpty()) SeccionEst("Quién escribe más") {
                x.top.forEachIndexed { i, m ->
                    Row(Modifier.fillMaxWidth().clickable { salidas.alAbrirPerfil(m.persona) }.padding(vertical = Esp.xs).testTag("top_${m.persona}"), verticalAlignment = Alignment.CenterVertically) {
                        Text("${i + 1}", Modifier.width(24.dp), style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.primary)
                        AvatarEnLinea(m, 40.dp)
                        Spacer(Modifier.width(Esp.m))
                        Column(Modifier.weight(1f)) {
                            Text(m.titulo, style = MaterialTheme.typography.bodyLarge, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            Text(plural(m.conteo.mensajes.toLong(), "mensaje", "mensajes"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                }
            }
            // acciones rápidas
            SeccionEst("Acciones rápidas") {
                val cerrado = info?.soloAdmins == true
                FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    AccionRedonda(if (cerrado) Ico.Abierto else Ico.Candado, if (cerrado) "Abrir" else "Cerrar", Modifier.testTag("accion_cerrar_abrir")) {
                        if (!soyAdmin) avisaToast(ctx, "Necesito ser administrador para esto.", true) else dialogo = if (cerrado) "abrir" else "cerrar"
                    }
                    AccionRedonda(Ico.Campana, "Mencionar a todos", Modifier.testTag("accion_todos")) { dialogo = "todos" }
                    AccionRedonda(Ico.Copiar, "Enlace", Modifier.testTag("accion_enlace")) {
                        scope.launch {
                            runCatching { Repo.enlaceGrupo(botId, jid, false) }
                                .onSuccess { l -> ctx.getSystemService(ClipboardManager::class.java).setPrimaryClip(ClipData.newPlainText("enlace", l)); avisaToast(ctx, "Enlace copiado", false) }
                                .onFailure { avisaToast(ctx, textoError(it), true) }
                        }
                    }
                    AccionRedonda(Ico.Agregar, "Solicitudes", Modifier.testTag("accion_solicitudes")) { salidas.alAbrirSolicitudes() }
                }
            }
            if (x.horario.activo) Text(
                "🕙 Se cierra a las ${x.horario.cierra} y se abre a las ${x.horario.abre}.", Modifier.testTag("resumen_horario"),
                style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
    when (dialogo) {
        "cerrar", "abrir" -> {
            val cerrar = dialogo == "cerrar"
            AlertDialog(
                onDismissRequest = { dialogo = null }, title = { Text(if (cerrar) "¿Cerrar el grupo?" else "¿Abrir el grupo?") },
                text = { Text(if (cerrar) "Solo los administradores podrán escribir hasta que lo abras." else "Todos podrán escribir otra vez.") },
                confirmButton = { TextButton({ dialogo = null; hacer(if (cerrar) "Listo: el grupo quedó cerrado" else "Listo: el grupo quedó abierto") { Repo.cerrarGrupo(botId, jid, cerrar) } }, Modifier.testTag("confirmar_cerrar_abrir")) { Text(if (cerrar) "Cerrar" else "Abrir") } },
                dismissButton = { TextButton({ dialogo = null }) { Text("Cancelar") } },
            )
        }
        "todos" -> HojaAlerta(
            onDismissRequest = { dialogo = null }, title = { Text("Mencionar a todos") },
            text = {
                Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                    Text("Le suena el teléfono a cada persona del grupo. Úsalo solo si de verdad es importante.", style = MaterialTheme.typography.bodyMedium)
                    OutlinedTextField(textoTodos, { textoTodos = it }, Modifier.fillMaxWidth().testTag("todos_campo"), label = { Text("Mensaje (opcional)") }, minLines = 2)
                }
            },
            confirmButton = {
                TextButton({
                    dialogo = null
                    scope.launch {
                        runCatching { Repo.mencionarATodos(botId, jid, textoTodos.trim()) }
                            .onSuccess { m -> textoTodos = ""; avisaToast(ctx, if (m.aviso.isNotBlank()) m.aviso else "Listo: mencioné a ${m.mencionadas} personas", m.truncado) }
                            .onFailure { avisaToast(ctx, textoError(it), true) }
                    }
                }, Modifier.testTag("confirmar_todos_admin"), enabled = estado?.conectado == true) { Text("Mencionar") }
            },
            dismissButton = { TextButton({ dialogo = null }) { Text("Cancelar") } },
        )
    }
}

// ---------- Reglas ----------

@Composable
private fun FilaIr(icono: ImageVector, titulo: String, texto: String, etiqueta: String, alAbrir: () -> Unit) {
    FilaAjuste(icono, titulo, texto, Modifier.testTag(etiqueta), alAbrir)
}

@Composable
private fun PestanaReglasAdmin(botId: Long, jid: String, nombre: String, salidas: SalidasAdminGrupo) {
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(vertical = Esp.s).testTag("pestana_reglas_admin"), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
        TituloSeccion("Que el bot cuide el grupo")
        FilaIr(Ico.Candado, "Filtros automáticos", "Enlaces, palabras prohibidas, mensajes repetidos y flood, con lo que hace el bot en cada caso.", "ir_filtros") { salidas.alAbrirFiltros() }
        FilaIr(Ico.Aviso, "Advertencias", "Quién tiene advertencias y qué pasa al llegar a cada tope.", "ir_advertencias") { salidas.alAbrirAdvertencias() }
        FilaIr(Ico.Volumen, "Silencios", "A quién el bot le borra lo que escribe, y por cuánto tiempo.", "ir_silencios") { salidas.alAbrirSilencios() }
        TituloSeccion("Las reglas del grupo")
        FilaIr(Ico.Documento, "Reglas escritas", "Lo que se manda con el comando de reglas. Se arma por puntos y se puede ordenar.", "ir_reglas") { salidas.alAbrirReglas(nombre) }
        FilaIr(Ico.Estadisticas, "Actividad", "Quién escribe más y quién lleva tiempo sin escribir.", "ir_actividad") { salidas.alAbrirActividad() }
    }
}

// ---------- Mensajes ----------

@Composable
private fun PestanaMensajesAdmin(botId: Long, jid: String, estado: EstadoGrupo?, salidas: SalidasAdminGrupo) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var anuncio by remember { mutableStateOf("") }
    var enviando by remember { mutableStateOf(false) }
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(vertical = Esp.s).testTag("pestana_mensajes_admin"), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
        TituloSeccion("Anuncio ahora")
        Column(Modifier.padding(horizontal = Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            OutlinedTextField(anuncio, { anuncio = it }, Modifier.fillMaxWidth().testTag("anuncio_campo"), label = { Text("Escribe el aviso para el grupo") }, minLines = 3)
            Button({
                val t = anuncio.trim(); enviando = true
                scope.launch {
                    runCatching { Repo.enviarTexto(botId, jid, t) }.onSuccess { anuncio = ""; avisaToast(ctx, "Aviso enviado al grupo", false) }.onFailure { avisaToast(ctx, textoError(it), true) }
                    enviando = false
                }
            }, Modifier.testTag("anuncio_enviar"), enabled = anuncio.isNotBlank() && !enviando && estado?.conectado == true) { Text("Enviar al grupo") }
            if (estado?.conectado == false) Text("El bot no está conectado: enciéndelo para enviar.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        TituloSeccion("Encuesta")
        TarjetaEncuesta(botId, jid, estado?.conectado == true)
        TituloSeccion("Automáticos")
        FilaIr(Ico.Chats, "Bienvenida y despedida", "El mensaje que sale cuando alguien entra o se va, con su nombre.", "ir_bienvenida") { salidas.alAbrirBienvenida() }
        FilaIr(Ico.Calendario, "Mensajes programados y difusiones", "Avisos que salen solos a cierta hora (módulos Programados y Difusión).", "ir_programados") { salidas.alAbrirModulos() }
    }
}

// ---------- Ajustes ----------

private val NOMBRES_DIAS = listOf("L", "M", "X", "J", "V", "S", "D")

@Composable
private fun PestanaAjustesAdmin(botId: Long, jid: String, estado: EstadoGrupo?, info: InfoGrupo?, soyAdmin: Boolean, refrescar: () -> Unit, salidas: SalidasAdminGrupo) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    val horario by produceState<HorarioGrupo?>(null, jid, version) { value = runCatching { Repo.horarioGrupo(botId, jid) }.getOrNull() }
    val baneados by produceState<List<BaneadoGrupo>?>(null, jid, version) { value = runCatching { Repo.baneadosGrupo(botId, jid) }.getOrNull() }
    val entrada by produceState<ProteccionEntrada?>(null, jid, version) { value = runCatching { Repo.entradaGrupo(botId, jid) }.getOrNull() }
    var enlace by remember { mutableStateOf<String?>(null) }
    var eligiendoOrigen by remember { mutableStateOf(false) }
    var origenPlantilla by remember { mutableStateOf<ChatItem?>(null) }
    fun hacer(ok: String, f: suspend () -> Unit) = scope.launch {
        runCatching { f() }.onSuccess { avisaToast(ctx, ok, false) }.onFailure { avisaToast(ctx, textoError(it), true) }
        refrescar(); version++
    }
    Box(Modifier.fillMaxSize()) {
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(vertical = Esp.s).testTag("pestana_ajustes_admin"), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
        TituloSeccion("Quién puede qué")
        FilaAjuste(Ico.Candado, "Solo administradores escriben", "Cierra el grupo: nadie más puede mandar mensajes.", Modifier.testTag("fila_cerrado"), derecha = {
            Switch(info?.soloAdmins == true, { v -> haptica(Toque.Fuerte); hacer(if (v) "Listo: el grupo quedó cerrado" else "Listo: el grupo quedó abierto") { Repo.cerrarGrupo(botId, jid, v) } }, Modifier.testTag("sw_admin_cerrado"), enabled = soyAdmin && info != null)
        })
        FilaAjuste(Ico.Info, "Solo administradores cambian la información", "Bloquea nombre, foto y descripción.", Modifier.testTag("fila_info_bloqueada"), derecha = {
            Switch(info?.infoBloqueada == true, { v -> haptica(Toque.Suave); hacer(if (v) "Listo: la información quedó bloqueada" else "Listo: la información quedó libre") { Repo.bloquearInfoGrupo(botId, jid, v) } }, Modifier.testTag("sw_admin_info"), enabled = soyAdmin && info != null)
        })
        if (!soyAdmin && estado != null) AvisoAdmin(true)

        TituloSeccion("Horario de cierre")
        val h = horario
        if (h == null) FilasDeCarga(2) else TarjetaHorario(h, soyAdmin, { nuevo -> hacer(if (nuevo.activo) "Horario guardado" else "Horario apagado") { Repo.guardarHorarioGrupo(botId, jid, nuevo) } })

        TituloSeccion("Protección de entrada")
        entrada?.let { e -> TarjetaEntrada(e, soyAdmin) { nuevo -> hacer("Protección guardada") { Repo.guardarEntradaGrupo(botId, jid, nuevo) } } }

        TituloSeccion("Plantillas")
        SeccionPlantillasGrupo(botId, jid, origenPlantilla, { eligiendoOrigen = true }, { origenPlantilla = null }) { version++ }

        TituloSeccion("Configuración base")
        SeccionBaseGrupos(botId, jid, version) { version++ }

        TituloSeccion("Gente")
        FilaIr(Ico.Agregar, "Solicitudes de ingreso", "Aceptar o rechazar a quien pide entrar.", "ir_solicitudes") { salidas.alAbrirSolicitudes() }
        val n = baneados?.size ?: 0
        FilaIr(Ico.Candado, "Baneados", if (n == 0) "Nadie está baneado. Quien banees no podrá volver a entrar." else plural(n.toLong(), "persona baneada", "personas baneadas"), "ir_baneados") { salidas.alAbrirBaneados() }

        TituloSeccion("Enlace de invitación")
        Row(Modifier.padding(horizontal = Esp.l), horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
            OutlinedButton({ scope.launch { runCatching { Repo.enlaceGrupo(botId, jid, false) }.onSuccess { enlace = it }.onFailure { avisaToast(ctx, textoError(it), true) } } }, Modifier.testTag("admin_ver_enlace"), enabled = soyAdmin) { Text("Ver enlace") }
            enlace?.let { l ->
                OutlinedButton({ ctx.getSystemService(ClipboardManager::class.java).setPrimaryClip(ClipData.newPlainText("enlace", l)); avisaToast(ctx, "Enlace copiado", false) }, Modifier.testTag("admin_copiar_enlace")) { Text("Copiar") }
            }
        }
        enlace?.let { Text(it, Modifier.padding(horizontal = Esp.l).testTag("admin_enlace_texto"), style = MaterialTheme.typography.bodyMedium) }

        TituloSeccion("Más")
        FilaIr(Ico.Ajustes, "Nombre, descripción y más ajustes", "Cambiar el nombre y la descripción, y crear un enlace nuevo.", "ir_ajustes_avanzados") { salidas.alAbrirAjustesAvanzados() }
    }
    // La hoja ocupa toda la pantalla, así que va aquí (fuera de la columna que se desplaza).
    if (eligiendoOrigen) HojaElegirChat(botId, true, "Copiar de…", "elegir_origen", { eligiendoOrigen = false }) { eligiendoOrigen = false; origenPlantilla = it }
    }
}

/** El horario con el que el grupo se cierra y se abre solo: interruptor, dos horas y los días. */
@Composable
private fun TarjetaHorario(h: HorarioGrupo, soyAdmin: Boolean, alGuardar: (HorarioGrupo) -> Unit) {
    var hora by remember { mutableStateOf<String?>(null) } // "cierra" | "abre"
    val haptica = rememberHaptica()
    Card(Modifier.fillMaxWidth().padding(horizontal = Esp.l).testTag("tarjeta_horario"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text("Cerrar y abrir solo", style = MaterialTheme.typography.titleMedium)
                    Text("Así de noche nadie escribe. Se usa la hora de este teléfono.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                Switch(h.activo, { haptica(Toque.Suave); alGuardar(h.copy(activo = it)) }, Modifier.testTag("sw_horario"))
            }
            Row(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                OutlinedButton({ hora = "cierra" }, Modifier.weight(1f).testTag("hora_cierra")) { Text("Cierra ${h.cierra}") }
                OutlinedButton({ hora = "abre" }, Modifier.weight(1f).testTag("hora_abre")) { Text("Abre ${h.abre}") }
            }
            Text("Días en que cierra", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.xs)) {
                NOMBRES_DIAS.forEachIndexed { i, d ->
                    FilterChip((h.dias and (1 shl i)) != 0, {
                        val nuevo = h.dias xor (1 shl i)
                        if (nuevo != 0) { haptica(Toque.Suave); alGuardar(h.copy(dias = nuevo)) }
                    }, { Text(d) }, modifier = Modifier.testTag("dia_$i"))
                }
            }
            if (h.activo && h.ultimo.isNotBlank()) Text("Ahora está ${if (h.ultimo == "cerrado") "cerrado por el horario" else "abierto"}.", Modifier.testTag("horario_estado"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            if (!soyAdmin) Text("Se guarda aquí; para que funcione, el bot debe ser administrador del grupo.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
    hora?.let { cual ->
        val actual = if (cual == "cierra") h.cierra else h.abre
        val estado = rememberTimePickerState(actual.substringBefore(':').toIntOrNull() ?: 22, actual.substringAfter(':').toIntOrNull() ?: 0, true)
        AlertaMaterial(
            onDismissRequest = { hora = null }, title = { Text(if (cual == "cierra") "¿A qué hora se cierra?" else "¿A qué hora se abre?") },
            text = { TimePicker(estado, Modifier.testTag("selector_hora")) },
            confirmButton = {
                TextButton({
                    val t = "%02d:%02d".format(estado.hour, estado.minute); hora = null
                    alGuardar(if (cual == "cierra") h.copy(cierra = t) else h.copy(abre = t))
                }, Modifier.testTag("hora_aceptar")) { Text("Listo") }
            },
            dismissButton = { TextButton({ hora = null }) { Text("Cancelar") } },
        )
    }
}

/** Modo raid (muchas cuentas nuevas de golpe → cierra el grupo y te avisa) y países cuyos números no entran. */
@Composable
private fun TarjetaEntrada(e: ProteccionEntrada, soyAdmin: Boolean, alGuardar: (ProteccionEntrada) -> Unit) {
    val haptica = rememberHaptica()
    var nuevo by remember { mutableStateOf("") }
    Card(Modifier.fillMaxWidth().padding(horizontal = Esp.l).testTag("tarjeta_entrada"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text("Modo raid", style = MaterialTheme.typography.titleMedium)
                    Text("Si entran muchas cuentas en poco tiempo, cierro el grupo, aviso y te llega una notificación.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                Switch(e.raidActivo, { haptica(Toque.Suave); alGuardar(e.copy(raidActivo = it)) }, Modifier.testTag("sw_raid"))
            }
            if (e.raidActivo) {
                PasoNumero("Cuentas nuevas", e.raidMax, 2, 50, "raid_max") { alGuardar(e.copy(raidMax = it)) }
                PasoNumero("En cuántos minutos", e.raidMinutos, 1, 30, "raid_min") { alGuardar(e.copy(raidMinutos = it)) }
            }
            HorizontalDivider()
            Text("Países bloqueados", style = MaterialTheme.typography.titleMedium)
            Text("Quien entre con un número que empiece con estos prefijos sale solo. Escribe el prefijo del país (por ejemplo 212 o +91).", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            if (e.prefijos.isNotEmpty()) FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.xs), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                e.prefijos.forEach { p ->
                    androidx.compose.material3.InputChip(false, { alGuardar(e.copy(prefijos = e.prefijos - p)) }, { Text("+$p") }, modifier = Modifier.testTag("prefijo_$p"),
                        trailingIcon = { Icon(Ico.Cerrar, "Quitar +$p", Modifier.size(16.dp)) })
                }
            }
            Row(verticalAlignment = Alignment.CenterVertically) {
                OutlinedTextField(nuevo, { nuevo = it.filter { c -> c.isDigit() || c == '+' }.take(5) }, Modifier.weight(1f).testTag("prefijo_campo"), singleLine = true, label = { Text("Prefijo") })
                Spacer(Modifier.width(Esp.s))
                Button({
                    val d = nuevo.filter { it.isDigit() }
                    if (d.isNotEmpty() && d !in e.prefijos) { alGuardar(e.copy(prefijos = e.prefijos + d)); nuevo = "" }
                }, Modifier.testTag("prefijo_agregar"), enabled = nuevo.any { it.isDigit() }) { Text("Agregar") }
            }
        }
    }
}

@Composable
private fun PasoNumero(titulo: String, valor: Int, minimo: Int, maximo: Int, etiqueta: String, alCambiar: (Int) -> Unit) {
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        Text(titulo, Modifier.weight(1f), style = MaterialTheme.typography.bodyMedium)
        OutlinedButton({ alCambiar((valor - 1).coerceAtLeast(minimo)) }, Modifier.testTag("${etiqueta}_menos"), enabled = valor > minimo) { Text("−") }
        Text("$valor", Modifier.padding(horizontal = Esp.m).testTag("${etiqueta}_valor"), style = MaterialTheme.typography.titleMedium)
        OutlinedButton({ alCambiar((valor + 1).coerceAtMost(maximo)) }, Modifier.testTag("${etiqueta}_mas"), enabled = valor < maximo) { Text("+") }
    }
}

// ---------- Historial ----------

private fun nombreOYo(nombre: String, persona: String) = when {
    nombre == "Yo" -> "Tú"
    nombre.isNotBlank() -> nombre
    persona.contains('@') && persona.first().isDigit() -> "+" + persona.substringBefore('@')
    else -> ""
}

/** La frase de una línea del historial, lista para leer. */
internal fun textoEvento(e: EventoGrupo): String {
    val persona = nombreOYo(e.personaNombre, e.persona)
    val actor = nombreOYo(e.actorNombre, e.actor)
    return when (e.clase) {
        "cambio" -> when (e.tipo) {
            "expulsado" -> if (actor.isNotBlank()) "$persona fue expulsada por $actor" else "$persona fue expulsada"
            "agregado" -> if (actor.isNotBlank()) "$persona fue agregada por $actor" else "$persona fue agregada"
            else -> "$persona ${e.texto}".trim()
        }
        else -> buildString {
            val tu = actor == "Tú"
            var t = if (persona.isBlank()) e.texto.removeSuffix(" de") else e.texto
            if (tu) t = t.substringBefore(' ').let { v -> (VERBOS_TU[v] ?: v) + t.removePrefix(v) }
            append(actor.ifBlank { "El bot" }).append(' ').append(t)
            if (persona.isNotBlank()) append(' ').append(persona)
            if (e.fallo) append(" (no se pudo)")
        }
    }
}

/** Para que «Tú» no lleve el verbo en tercera persona: «Tú cerraste el grupo». */
private val VERBOS_TU = mapOf(
    "abrió" to "abriste", "cerró" to "cerraste", "borró" to "borraste", "cambió" to "cambiaste", "expulsó" to "expulsaste", "baneó" to "baneaste", "hizo" to "hiciste",
    "quitó" to "quitaste", "aprobó" to "aprobaste", "rechazó" to "rechazaste", "mencionó" to "mencionaste", "bloqueó" to "bloqueaste", "liberó" to "liberaste",
    "agregó" to "agregaste", "pidió" to "pediste", "sacó" to "sacaste",
)

private fun colorEvento(e: EventoGrupo) = when {
    e.fallo -> AMARILLO_SILENCIO
    e.tipo in setOf("expulsado", "grupo.expulsar", "grupo.banear", "grupo.baneado_volvio", "sale") -> ROJO_SILENCIO
    e.tipo in setOf("entra", "enlace", "agregado", "promovido", "grupo.ascender", "grupo.desbanear") -> VERDE_SILENCIO
    else -> AMARILLO_SILENCIO
}

@Composable
internal fun ListaEventos(eventos: List<EventoGrupo>, modifier: Modifier = Modifier, alFinal: (@Composable () -> Unit)? = null) {
    LazyColumn(modifier.testTag("lista_eventos"), contentPadding = PaddingValues(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
        items(eventos, key = { "${it.ts}${it.clase}${it.tipo}${it.persona}${it.actor}" }) { e ->
            Row(Modifier.fillMaxWidth().testTag("evento_${e.ts}"), verticalAlignment = Alignment.CenterVertically) {
                Box(Modifier.size(12.dp).clip(CircleShape).background(colorEvento(e)))
                Spacer(Modifier.width(Esp.m))
                Column(Modifier.weight(1f)) {
                    Text(textoEvento(e), style = MaterialTheme.typography.bodyLarge)
                    Text("${etiquetaDia(e.ts)} · ${horaDe(e.ts)}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
        }
        if (alFinal != null) item(key = "final") { alFinal() }
    }
}

@Composable
private fun PestanaHistorialAdmin(botId: Long, jid: String) {
    var items by remember { mutableStateOf<List<EventoGrupo>?>(null) }
    var fin by remember { mutableStateOf(false) }
    var cargando by remember { mutableStateOf(false) }
    var filtro by remember { mutableStateOf("todo") }
    val scope = rememberCoroutineScope()
    fun cargar(primera: Boolean) {
        if (cargando) return
        cargando = true
        val antes = if (primera) 0L else items?.lastOrNull()?.ts ?: 0L
        scope.launch {
            val r = runCatching { Repo.historialGrupo(botId, jid, antes, 60) }.getOrDefault(emptyList())
            items = (if (primera) emptyList() else items ?: emptyList()) + r
            fin = r.size < 60; cargando = false
        }
    }
    LaunchedEffect(jid) { cargar(true) }
    Column(Modifier.fillMaxSize().testTag("pestana_historial_admin")) {
        Row(Modifier.padding(horizontal = Esp.l, vertical = Esp.s), horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
            listOf("todo" to "Todo", "accion" to "Moderación", "cambio" to "Miembros").forEach { (k, t) ->
                FilterChip(filtro == k, { filtro = k }, { Text(t) }, modifier = Modifier.testTag("filtro_historial_$k"))
            }
        }
        val datos = items
        val vista = datos?.filter { filtro == "todo" || it.clase == filtro }
        when {
            vista == null -> FilasDeCarga()
            vista.isEmpty() -> Box(Modifier.fillMaxSize(), Alignment.Center) {
                EstadoVacio(Ico.Reloj, "Sin movimientos todavía", "Aquí verás quién entró, quién salió y lo que hagas como administrador.", Modifier.testTag("historial_vacio"))
            }
            else -> ListaEventos(vista, Modifier.fillMaxSize(), if (!fin) ({ OutlinedButton({ cargar(false) }, Modifier.testTag("historial_mas")) { Text("Ver más") } }) else null)
        }
    }
}

// ---------- Baneados ----------

/** Quién está baneado de este grupo, con su motivo, y un botón para dejarlos volver. */
@Composable
fun PantallaBaneadosGrupo(botId: Long, jid: String, alVolver: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var version by remember { mutableIntStateOf(0) }
    val lista by produceState<List<BaneadoGrupo>?>(null, jid, version) { value = runCatching { Repo.baneadosGrupo(botId, jid) }.getOrDefault(emptyList()) }
    var quitar by remember { mutableStateOf<BaneadoGrupo?>(null) }
    MarcoPantalla("Baneados", alVolver) {
        val l = lista
        when {
            l == null -> FilasDeCarga(4)
            l.isEmpty() -> EstadoVacio(Ico.Candado, "Nadie está baneado", "Quien banees sale del grupo y, si intenta volver, el bot lo saca otra vez.", Modifier.align(Alignment.Center).testTag("baneados_vacio"))
            else -> LazyColumn(Modifier.fillMaxSize().testTag("lista_baneados"), contentPadding = PaddingValues(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                items(l, key = { it.persona }) { b ->
                    Card(Modifier.fillMaxWidth().testTag("baneado_${b.persona}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                        Row(Modifier.padding(Esp.m), verticalAlignment = Alignment.CenterVertically) {
                            Column(Modifier.weight(1f)) {
                                Text(b.titulo, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                if (b.telefono.isNotBlank() && b.titulo != b.telefono) Text(b.telefono, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                if (b.motivo.isNotBlank()) Text("Motivo: ${b.motivo}", style = MaterialTheme.typography.bodyMedium)
                                Text("${etiquetaDia(b.ts)} · por ${nombreOYo(b.porNombre, b.por).ifBlank { "el bot" }}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                            OutlinedButton({ quitar = b }, Modifier.testTag("desbanear_${b.persona}")) { Text("Quitar baneo") }
                        }
                    }
                }
            }
        }
    }
    quitar?.let { b ->
        AlertDialog(
            onDismissRequest = { quitar = null }, title = { Text("¿Dejar volver a ${b.titulo}?") },
            text = { Text("Podrá entrar de nuevo al grupo (por el enlace o si alguien la agrega). Yo no la agrego sola.") },
            confirmButton = {
                TextButton({
                    quitar = null
                    scope.launch { runCatching { Repo.desbanear(botId, jid, b.persona) }.onSuccess { avisaToast(ctx, "Listo: ya puede volver", false) }.onFailure { avisaToast(ctx, textoError(it), true) }; version++ }
                }, Modifier.testTag("confirmar_desbanear")) { Text("Quitar baneo") }
            },
            dismissButton = { TextButton({ quitar = null }) { Text("Cancelar") } },
        )
    }
}
