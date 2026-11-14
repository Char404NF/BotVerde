@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

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
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.PrimaryTabRow
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Tab
import androidx.compose.material3.Text
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.ChatItem
import com.botchat.app.datos.CambioMiembro
import com.botchat.app.datos.EtiquetaItem
import com.botchat.app.datos.GrupoAdmin
import com.botchat.app.datos.MiembroDetalle
import com.botchat.app.datos.Repo
import com.botchat.app.datos.actualizarMiembros
import com.botchat.app.datos.cambiosMiembros
import com.botchat.app.datos.miembrosDetallado
import kotlinx.coroutines.launch

/**
 * La info de un grupo, en dos pestañas: Miembros (quién es quién, con su rango, etiquetas, iconos y contador; tocar uno abre lo que
 * escribió en este grupo) y Ajustes (todo lo que ya había: política del bot, administrar el grupo, resumen y multimedia).
 */
@Composable
fun PantallaInfoGrupo(
    botId: Long, jid: String, alVolver: () -> Unit,
    alAbrirMiembro: (persona: String, nombre: String) -> Unit = { _, _ -> },
    alVerCambios: () -> Unit = {},
    alAbrirAdministrar: () -> Unit = {},
    alAbrirResumen: () -> Unit = {},
    alAbrirChat: () -> Unit = {},
) {
    var pestana by remember { mutableIntStateOf(0) }
    val haptica = rememberHaptica()
    val chat by produceState<ChatItem?>(null, jid) { value = runCatching { Repo.chat(botId, jid) }.getOrNull() }
    val admin by produceState<GrupoAdmin?>(null, jid) { value = runCatching { Repo.gruposAdmin(botId).firstOrNull { it.jid == jid } }.getOrNull() }
    Scaffold(topBar = {
        Column {
            TopAppBar(navigationIcon = { IconButton(alVolver, Modifier.testTag("volver_info_grupo")) { Icon(Ico.Atras, "Volver") } }, title = { Text("Info del grupo") })
            Column(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s), horizontalAlignment = Alignment.CenterHorizontally) {
                Avatar(chat?.nombre.orEmpty(), chat?.foto.orEmpty(), chat?.fotoTs ?: 0, jid, 96.dp, grupo = true)
                Spacer(Modifier.size(Esp.s))
                Text(chat?.nombre.orEmpty(), Modifier.testTag("grupo_nombre"), style = MaterialTheme.typography.headlineSmall, maxLines = 2, overflow = TextOverflow.Ellipsis)
                val a = admin
                val detalle = buildList {
                    add("Grupo")
                    if (a != null && a.miembros > 0) add(plural(a.miembros.toLong(), "miembro", "miembros"))
                    if (a != null && a.sabido) add(if (a.soyAdmin) "Soy admin" else "No soy admin")
                }.joinToString(" · ")
                Text(detalle, Modifier.testTag("grupo_detalle"), style = MaterialTheme.typography.bodyMedium, color = if (a?.soyAdmin == false && a.sabido) AMARILLO_SILENCIO else MaterialTheme.colorScheme.onSurfaceVariant)
                Spacer(Modifier.size(Esp.m))
                Row(horizontalArrangement = Arrangement.spacedBy(Esp.m)) {
                    AccionRedonda(Ico.Chats, "Chat", Modifier.testTag("grupo_abrir_chat"), alAbrirChat)
                    AccionRedonda(Ico.Ajustes, "Administrar", Modifier.testTag("grupo_administrar_rapido"), alAbrirAdministrar)
                    AccionRedonda(Ico.Estadisticas, "Resumen", Modifier.testTag("grupo_resumen_rapido"), alAbrirResumen)
                }
            }
            PrimaryTabRow(pestana) {
                Tab(pestana == 0, { haptica(Toque.Suave); pestana = 0 }, text = { Text("Miembros") }, modifier = Modifier.testTag("pestana_miembros"))
                Tab(pestana == 1, { haptica(Toque.Suave); pestana = 1 }, text = { Text("Ajustes") }, modifier = Modifier.testTag("pestana_ajustes"))
            }
        }
    }) { relleno ->
        Box(Modifier.fillMaxSize().padding(relleno)) {
            if (pestana == 0) PestanaMiembros(botId, jid, alAbrirMiembro, alVerCambios)
            else PestanaAjustes(botId, jid, chat?.nombre.orEmpty(), alAbrirAdministrar, alAbrirResumen)
        }
    }
}

private fun nombreDeMiembro(m: MiembroDetalle) = m.nombre.ifBlank { m.telefono.ifBlank { m.persona.substringBefore('@') } }

@Composable
private fun PestanaMiembros(botId: Long, jid: String, alAbrir: (String, String) -> Unit, alVerCambios: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var version by remember { mutableIntStateOf(0) }
    var salidos by remember { mutableStateOf(false) }
    var consulta by remember { mutableStateOf("") }
    LaunchedEffect(jid) { Repo.cambios.collect { evs -> if (evs.any { it == "etiquetas" || it == "personas" || it == "rangos" || it == "grupo:$jid" || it.startsWith("silenc") || it.startsWith("advert") }) version++ } }
    val miembros by produceState<List<MiembroDetalle>?>(null, jid, version, salidos) { value = runCatching { Repo.miembrosDetallado(botId, jid, salidos) }.getOrDefault(emptyList()) }
    val etiquetas by produceState<List<EtiquetaItem>>(emptyList(), version) { value = runCatching { Repo.etiquetas(botId) }.getOrDefault(emptyList()) }
    val lista = miembros
    Column(Modifier.fillMaxSize().testTag("pestana_miembros_contenido")) {
        androidx.compose.material3.OutlinedTextField(
            consulta, { consulta = it }, Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.xs).testTag("buscar_miembro"), singleLine = true,
            placeholder = { Text("Buscar a alguien") }, leadingIcon = { Icon(Ico.Buscar, null) },
        )
        FlowRow(Modifier.padding(horizontal = Esp.l), horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
            OutlinedButton(alVerCambios, Modifier.testTag("ver_cambios_miembros")) { Icon(Ico.Reloj, null, Modifier.size(18.dp)); Text("  Ver cambios en los miembros") }
            FilterChip(salidos, { salidos = !salidos }, { Text("Mostrar los que salieron") }, modifier = Modifier.testTag("mostrar_salidos"))
        }
        when {
            lista == null -> FilasDeCarga()
            lista.isEmpty() -> Column(Modifier.fillMaxSize().padding(Esp.l), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
                EstadoVacio(Ico.Grupo, "Aún no tengo la lista", "Con el bot encendido la voy guardando sola. También puedes pedirla ahora.", Modifier.testTag("miembros_vacio"))
                OutlinedButton({
                    scope.launch {
                        runCatching { Repo.actualizarMiembros(botId) }.onSuccess { avisaToast(ctx, "Pidiendo la lista…", false) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo", true) }
                        kotlinx.coroutines.delay(1500); version++
                    }
                }, Modifier.testTag("actualizar_miembros")) { Text("Pedir la lista") }
            }
            else -> {
                val ordenada = lista.filter { consulta.isBlank() || nombreDeMiembro(it).contains(consulta, true) || it.telefono.contains(consulta) }
                    .sortedWith(compareBy<MiembroDetalle> { it.hastaTs > 0 }.thenByDescending { it.esAdmin }.thenByDescending { it.mensajes }.thenBy { nombreDeMiembro(it).lowercase() })
                Text(plural(ordenada.size.toLong(), "persona", "personas"), Modifier.padding(horizontal = Esp.l, vertical = 2.dp).testTag("contador_miembros"), style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
                LazyColumn(Modifier.fillMaxSize().testTag("lista_miembros"), contentPadding = PaddingValues(bottom = 24.dp)) {
                    items(ordenada, key = { it.persona + it.jid }) { m ->
                        FilaMiembroE7(m, etiquetas.filter { it.id in m.etiquetas }) { alAbrir(m.persona.ifBlank { m.jid }, nombreDeMiembro(m)) }
                        HorizontalDivider(Modifier.padding(start = 76.dp), color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                    }
                }
            }
        }
    }
}

/** Una persona del grupo: foto, nombre, insignia de rango, etiquetas de color, iconos de silenciada o advertida y cuánto escribe. */
@Composable
fun FilaMiembroE7(m: MiembroDetalle, etiquetas: List<EtiquetaItem>, alPulsar: () -> Unit) {
    val haptica = rememberHaptica()
    Row(
        Modifier.fillMaxWidth().clickableEscala { haptica(Toque.Suave); alPulsar() }.padding(horizontal = Esp.l, vertical = Esp.m).testTag("miembro_${m.persona.ifBlank { m.jid }}"),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Avatar(nombreDeMiembro(m), m.foto, 0, m.jid.ifBlank { m.persona }, 48.dp)
        Spacer(Modifier.width(Esp.m))
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(nombreDeMiembro(m), Modifier.weight(1f, fill = false), style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                if (m.silenciado) { Spacer(Modifier.width(6.dp)); Icon(Ico.Volumen, "Silenciado", Modifier.size(18.dp).testTag("icono_silenciado_${m.persona}"), tint = ROJO_SILENCIO) }
                if (m.advertido) { Spacer(Modifier.width(4.dp)); Icon(Ico.Aviso, "Advertido", Modifier.size(18.dp).testTag("icono_advertido_${m.persona}"), tint = AMARILLO_SILENCIO) }
                Spacer(Modifier.weight(1f))
                if (m.esAdmin) InsigniaAdmin(m.rol == "creador")
            }
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.xs), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                if (m.rango.isNotBlank()) InsigniaRango(m.rango, m.rangoColor, Modifier.testTag("rango_${m.persona}"))
                etiquetas.take(3).forEach { ChipEtiqueta(it) }
                if (etiquetas.size > 3) Text("+${etiquetas.size - 3}", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            Text(
                (if (m.hastaTs > 0) "Ya no está · " else "") + plural(m.mensajes.toLong(), "mensaje", "mensajes") + (if (m.ultimoTs > 0) " · última vez ${etiquetaDia(m.ultimoTs)}" else ""),
                Modifier.testTag("contador_${m.persona}"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Composable
private fun InsigniaAdmin(creador: Boolean) {
    Text(
        if (creador) "Creó el grupo" else "Admin. del grupo",
        Modifier.clip(androidx.compose.foundation.shape.RoundedCornerShape(50)).background(MaterialTheme.colorScheme.primaryContainer).padding(horizontal = 8.dp, vertical = 2.dp).testTag("insignia_admin"),
        style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onPrimaryContainer,
    )
}

@Composable
private fun PestanaAjustes(botId: Long, jid: String, nombre: String, alAdministrar: () -> Unit, alResumen: () -> Unit) {
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(vertical = Esp.s).testTag("pestana_ajustes_contenido"), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
        TituloSeccion("El bot en este grupo")
        TarjetaPoliticaGrupo(botId, jid, nombre)
        TituloSeccion("Administrar")
        Box(Modifier.testTag("tarjeta_administrar_grupo")) { FilaAjuste(Ico.Ajustes, "Administrar el grupo", "Nombre, descripción, enlace, reglas, solicitudes, filtros y actividad", Modifier.testTag("abrir_grupo"), alAdministrar) }
        Box(Modifier.testTag("tarjeta_resumen_grupo")) { FilaAjuste(Ico.Estadisticas, "Resumen, calendario y archivos", "Cuánto se escribe, cuándo, y las fotos y documentos del grupo", Modifier.testTag("abrir_resumen_grupo"), alResumen) }
        TarjetaMiWhatsApp(botId, jid, nombre)
    }
}

// ---------- cambios en los miembros ----------

internal fun textoCambio(c: CambioMiembro): String {
    val quien = c.nombre.ifBlank { c.persona.substringBefore('@') }
    val por = c.porNombre.ifBlank { c.por.substringBefore('@') }
    return when (c.tipo) {
        "entra" -> "$quien entró"
        "enlace" -> "$quien entró con el enlace"
        "agregado" -> if (por == "Yo") "Añadiste a $quien" else if (por.isNotBlank()) "$quien fue agregado por $por" else "$quien fue agregado"
        "sale" -> "$quien salió"
        "expulsado" -> if (por == "Yo") "Quitaste a $quien" else if (por.isNotBlank()) "$quien fue sacado por $por" else "$quien fue sacado del grupo"
        "promovido" -> "$quien ahora es administrador"
        "degradado" -> "$quien ya no es administrador"
        else -> "$quien: ${c.tipo}"
    }
}

private fun colorCambio(tipo: String) = when (tipo) {
    "entra", "enlace", "agregado", "promovido" -> VERDE_SILENCIO
    "expulsado" -> ROJO_SILENCIO
    else -> AMARILLO_SILENCIO
}

/** «Ver cambios en los miembros»: quién entró, quién salió y a quién sacaron, lo más reciente primero. */
@Composable
fun PantallaCambiosMiembros(botId: Long, jid: String, alVolver: () -> Unit) {
    var items by remember { mutableStateOf<List<CambioMiembro>?>(null) }
    var fin by remember { mutableStateOf(false) }
    var cargando by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    fun cargar(primera: Boolean) {
        if (cargando) return
        cargando = true
        val antes = if (primera) 0L else items?.lastOrNull()?.ts ?: 0L
        scope.launch {
            val r = runCatching { Repo.cambiosMiembros(botId, jid, antes, 60) }.getOrDefault(emptyList())
            items = (if (primera) emptyList() else items ?: emptyList()) + r
            fin = r.size < 60; cargando = false
        }
    }
    LaunchedEffect(jid) { cargar(true) }
    MarcoPantalla("Cambios en los miembros", alVolver) {
        val datos = items
        when {
            datos == null -> FilasDeCarga()
            datos.isEmpty() -> EstadoVacio(Ico.Grupo, "Sin cambios todavía", "Cuando alguien entre, salga o lo saquen, aparecerá aquí.", Modifier.align(Alignment.Center).testTag("cambios_vacio"))
            else -> LazyColumn(Modifier.fillMaxSize().testTag("lista_cambios"), contentPadding = PaddingValues(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                items(datos, key = { it.id }) { c ->
                    Row(Modifier.fillMaxWidth().testTag("cambio_${c.id}"), verticalAlignment = Alignment.CenterVertically) {
                        Box(Modifier.size(12.dp).clip(androidx.compose.foundation.shape.CircleShape).background(colorCambio(c.tipo)))
                        Spacer(Modifier.width(Esp.m))
                        Column(Modifier.weight(1f)) {
                            Text(textoCambio(c), style = MaterialTheme.typography.bodyLarge)
                            Text("${etiquetaDia(c.ts)} · ${horaDe(c.ts)}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                }
                if (!fin) item(key = "mas") { OutlinedButton({ cargar(false) }, Modifier.testTag("cambios_mas")) { Text("Ver más") } }
            }
        }
    }
}

/** Botón redondo con su nombre debajo, para las acciones rápidas de la cabecera. */
@Composable
fun AccionRedonda(icono: androidx.compose.ui.graphics.vector.ImageVector, texto: String, modifier: Modifier = Modifier, alPulsar: () -> Unit) {
    val haptica = rememberHaptica()
    Column(modifier.clip(androidx.compose.foundation.shape.RoundedCornerShape(16.dp)).clickable { haptica(Toque.Suave); alPulsar() }.padding(Esp.xs), horizontalAlignment = Alignment.CenterHorizontally) {
        Box(Modifier.size(width = 64.dp, height = 40.dp).clip(androidx.compose.foundation.shape.RoundedCornerShape(20.dp)).background(MaterialTheme.colorScheme.surfaceContainerHigh), Alignment.Center) {
            Icon(icono, null, tint = MaterialTheme.colorScheme.primary)
        }
        Spacer(Modifier.size(4.dp))
        Text(texto, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}
