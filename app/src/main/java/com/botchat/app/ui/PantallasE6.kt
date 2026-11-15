@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import com.botchat.app.ui.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.Paises
import com.botchat.app.datos.ChatItem
import com.botchat.app.datos.EtiquetaItem
import com.botchat.app.datos.Repo
import com.botchat.app.datos.estadosEnBruto
import kotlinx.coroutines.delay

// ---------- marco de una pantalla interna: barra con «Volver» y una sola cosa debajo ----------

@Composable
fun MarcoPantalla(titulo: String, alVolver: () -> Unit, contenido: @Composable androidx.compose.foundation.layout.BoxScope.() -> Unit) {
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver, Modifier.testTag("volver_marco")) { Icon(Ico.Atras, "Volver") } }, title = { Text(titulo) }) }) { relleno ->
        Box(Modifier.fillMaxSize().padding(relleno)) { contenido() }
    }
}

/** Pantalla que todavía no existe: se ve clara en la app, para que nadie piense que falló algo. La completa otra etapa. */
@Composable
fun PantallaProvisoria(titulo: String, texto: String, alVolver: () -> Unit, etiqueta: String = "provisoria") {
    MarcoPantalla(titulo, alVolver) {
        EstadoVacio(Ico.Info, "Esta pantalla llega pronto", texto, Modifier.align(Alignment.Center).testTag(etiqueta))
    }
}

// ---------- Personas y Grupos ----------

@Composable
private fun CajaBuscar(valor: String, alCambiar: (String) -> Unit, pista: String, etiqueta: String) {
    TextField(
        value = valor, onValueChange = alCambiar, singleLine = true, placeholder = { Text(pista, maxLines = 1, overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis) },
        leadingIcon = { Icon(Ico.Buscar, null) },
        trailingIcon = { if (valor.isNotEmpty()) IconButton({ alCambiar("") }) { Icon(Ico.Cerrar, "Borrar texto") } },
        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search), shape = RoundedCornerShape(28.dp),
        colors = TextFieldDefaults.colors(
            focusedContainerColor = MaterialTheme.colorScheme.surfaceContainerHigh, unfocusedContainerColor = MaterialTheme.colorScheme.surfaceContainerHigh,
            focusedIndicatorColor = Color.Transparent, unfocusedIndicatorColor = Color.Transparent, disabledIndicatorColor = Color.Transparent,
        ),
        modifier = Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s).testTag(etiqueta),
    )
}

/** Puntitos de color de las etiquetas (como máximo cuatro; si hay más, un «+N»). */
@Composable
fun PuntitosEtiqueta(puestas: List<EtiquetaItem>, modifier: Modifier = Modifier) {
    if (puestas.isEmpty()) return
    Row(modifier, horizontalArrangement = Arrangement.spacedBy(4.dp), verticalAlignment = Alignment.CenterVertically) {
        puestas.take(4).forEach { PuntoRango(it.color, 10, Modifier.testTag("punto_${it.id}")) }
        if (puestas.size > 4) Text("+${puestas.size - 4}", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

private data class CargaLista(val chats: List<ChatItem>?, val etiquetas: List<EtiquetaItem>, val porId: Map<String, List<Long>>, val admin: Map<String, com.botchat.app.datos.GrupoAdmin>, val estados: Map<String, com.botchat.app.datos.EstadoPersonaLista> = emptyMap())

@Composable
private fun cargaLista(botId: Long, grupos: Boolean, consulta: String): CargaLista {
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("chats" in evs || "etiquetas" in evs || "personas" in evs || evs.any { it.startsWith("silenc") || it.startsWith("advert") }) version++ } }
    val estado by produceState(CargaLista(null, emptyList(), emptyMap(), emptyMap()), consulta, version) {
        if (consulta.isNotBlank()) delay(200)
        val chats = runCatching { Repo.chats(botId, grupos, consulta, 500) }.getOrDefault(emptyList())
        val et = runCatching { Repo.etiquetas(botId) }.getOrDefault(emptyList())
        val porId = if (grupos) chats.associate { it.jid to it.etiquetas } else {
            val dePersona = runCatching { Repo.etiquetasDePersonas(botId, chats.map { it.jid }) }.getOrDefault(emptyMap())
            chats.associate { it.jid to (it.etiquetas + (dePersona[it.jid] ?: emptyList())).distinct() }
        }
        val admin = if (grupos) runCatching { Repo.gruposAdmin(botId).associateBy { it.jid } }.getOrDefault(emptyMap()) else emptyMap()
        val estados = if (grupos) emptyMap() else runCatching { com.botchat.app.datos.JsonE7.estados(Repo.estadosEnBruto(botId, chats.map { it.jid })) }.getOrDefault(emptyMap())
        value = CargaLista(chats, et, porId, admin, estados)
    }
    return estado
}

/** Pestaña Personas: todas las personas con las que hay chat, con buscador. Tocar una abre su perfil. */
@Composable
fun PantallaPersonas(botId: Long, alAbrir: (String) -> Unit) {
    var consulta by remember { mutableStateOf("") }
    val c = cargaLista(botId, false, consulta)
    Column(Modifier.fillMaxSize().testTag("pantalla_personas")) {
        CajaBuscar(consulta, { consulta = it }, "Buscar persona por nombre o número", "buscador_personas")
        val lista = c.chats
        if (lista != null) Text(
            if (consulta.isBlank()) plural(lista.size.toLong(), "persona", "personas") else plural(lista.size.toLong(), "resultado", "resultados"),
            Modifier.padding(horizontal = Esp.l, vertical = 2.dp).testTag("contador_personas"), style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        when {
            lista == null -> FilasDeCarga()
            lista.isEmpty() -> Box(Modifier.fillMaxSize()) {
                if (consulta.isNotBlank()) EstadoVacio(Ico.Buscar, "Sin resultados", "No hay personas con ese nombre o número.", Modifier.align(Alignment.Center))
                else EstadoVacio(Ico.Persona, "Aún no hay personas", "Cuando alguien escriba al número del bot, aparecerá aquí.", Modifier.align(Alignment.Center))
            }
            else -> LazyColumn(Modifier.fillMaxSize().testTag("lista_personas"), contentPadding = PaddingValues(bottom = 24.dp)) {
                items(lista, key = { it.jid }) { p ->
                    val puestas = c.etiquetas.filter { it.id in (c.porId[p.jid] ?: emptyList()) }
                    FilaPersona(p, puestas, c.estados[p.jid]) { alAbrir(p.jid) }
                }
            }
        }
    }
}

@Composable
private fun FilaPersona(p: ChatItem, puestas: List<EtiquetaItem>, estado: com.botchat.app.datos.EstadoPersonaLista?, alPulsar: () -> Unit) {
    Row(Modifier.fillMaxWidth().clickableEscala { alPulsar() }.padding(horizontal = Esp.l, vertical = Esp.m).testTag("persona_${p.jid}"), verticalAlignment = Alignment.CenterVertically) {
        Avatar(p.nombre, p.foto, p.fotoTs, p.jid, 52.dp)
        Spacer(Modifier.width(Esp.l))
        Column(Modifier.weight(1f)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(p.nombre, Modifier.weight(1f, fill = false), style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
            if (p.telefono.isNotBlank() && p.telefono != p.nombre) TextoUnaLinea(p.telefono)
        }
        if (estado != null) {
            if (estado.silenciado) Icon(Ico.Volumen, "Silenciada", Modifier.size(18.dp).testTag("silenciada_${p.jid}"), tint = ROJO_SILENCIO)
            if (estado.advertido) Icon(Ico.Aviso, "Advertida", Modifier.padding(start = 2.dp).size(18.dp).testTag("advertida_${p.jid}"), tint = AMARILLO_SILENCIO)
            Spacer(Modifier.width(6.dp))
        }
        PuntitosEtiqueta(puestas, Modifier.testTag("puntitos_${p.jid}"))
    }
}

/** Pestaña Grupos: cada grupo con su número de miembros y si soy admin. Tocar uno abre la info del grupo. */
@Composable
fun PantallaGrupos(botId: Long, alAbrir: (String) -> Unit) {
    var consulta by remember { mutableStateOf("") }
    val c = cargaLista(botId, true, consulta)
    Column(Modifier.fillMaxSize().testTag("pantalla_grupos")) {
        CajaBuscar(consulta, { consulta = it }, "Buscar grupo por nombre", "buscador_grupos")
        val lista = c.chats
        if (lista != null) Text(
            if (consulta.isBlank()) plural(lista.size.toLong(), "grupo", "grupos") else plural(lista.size.toLong(), "resultado", "resultados"),
            Modifier.padding(horizontal = Esp.l, vertical = 2.dp).testTag("contador_grupos"), style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        when {
            lista == null -> FilasDeCarga()
            lista.isEmpty() -> Box(Modifier.fillMaxSize()) {
                if (consulta.isNotBlank()) EstadoVacio(Ico.Buscar, "Sin resultados", "No hay grupos con ese nombre.", Modifier.align(Alignment.Center))
                else EstadoVacio(Ico.Grupo, "Aún no hay grupos", "Cuando el bot esté en un grupo, aparecerá aquí.", Modifier.align(Alignment.Center))
            }
            else -> LazyColumn(Modifier.fillMaxSize().testTag("lista_grupos"), contentPadding = PaddingValues(bottom = 24.dp)) {
                items(lista, key = { it.jid }) { g ->
                    val puestas = c.etiquetas.filter { it.id in (c.porId[g.jid] ?: emptyList()) }
                    val a = c.admin[g.jid]
                    Row(Modifier.fillMaxWidth().clickableEscala { alAbrir(g.jid) }.padding(horizontal = Esp.l, vertical = Esp.m).testTag("grupo_${g.jid}"), verticalAlignment = Alignment.CenterVertically) {
                        Avatar(g.nombre, g.foto, g.fotoTs, g.jid, 52.dp, grupo = true)
                        Spacer(Modifier.width(Esp.l))
                        Column(Modifier.weight(1f)) {
                            Text(g.nombre, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            val detalle = buildList {
                                if (a != null && a.miembros > 0) add(plural(a.miembros.toLong(), "miembro", "miembros"))
                                if (a != null && a.sabido && a.soyAdmin) add("Soy admin")
                            }.joinToString(" · ")
                            if (detalle.isNotEmpty()) TextoUnaLinea(detalle, Modifier.testTag("detalle_${g.jid}"))
                        }
                        PuntitosEtiqueta(puestas, Modifier.testTag("puntitos_${g.jid}"))
                    }
                }
            }
        }
    }
}

// ---------- pestaña Bot ----------

@Composable
private fun BotonGrande(icono: ImageVector, titulo: String, texto: String, etiqueta: String, alPulsar: () -> Unit) {
    FilaAjuste(icono, titulo, texto, Modifier.testTag(etiqueta), alPulsar)
}

/** Pestaña Bot: un botón grande por cada cosa. En modo Completo se suman las herramientas avanzadas. */
@Composable
fun PantallaCentroBot(
    alMiBot: () -> Unit, alAlmacenamiento: () -> Unit, alAuditoria: () -> Unit, alSilencios: () -> Unit, alAdvertencias: () -> Unit,
    alEtiquetas: () -> Unit, alAjustes: () -> Unit,
    alComandos: () -> Unit = {}, alModulos: () -> Unit = {}, alLimites: () -> Unit = {}, alLaboratorio: () -> Unit = {}, alActividad: () -> Unit = {},
    alAsistente: () -> Unit = {}, alRespuestas: () -> Unit = {},
) {
    val completo = LocalModoCompleto.current
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).testTag("pantalla_centro_bot")) {
        TituloSeccion("Tu bot")
        BotonGrande(Ico.Agregar, "Crea tu bot", "Cinco pasos: dónde responde, bienvenida, menú, reglas y utilidades", "hub_asistente", alAsistente)
        BotonGrande(Ico.Bot, "Mi bot", "Encender, apagar y ver si está conectado", "hub_mi_bot", alMiBot)
        BotonGrande(Ico.Editar, "Respuestas del bot", "Cambia lo que contesta: «Listo…», «No tienes permiso…», reportes y más", "hub_respuestas", alRespuestas)
        TituloSeccion("Personas y grupos")
        BotonGrande(Ico.Volumen, "Silencios", "A quién silenciaste en tus grupos", "hub_silencios", alSilencios)
        BotonGrande(Ico.Aviso, "Advertencias", "A quién has advertido y lo que anotaste", "hub_advertencias", alAdvertencias)
        BotonGrande(Ico.Seleccionar, "Etiquetas y marcados", "Ordena personas, grupos y mensajes con colores", "hub_etiquetas", alEtiquetas)
        TituloSeccion("Datos y privacidad")
        BotonGrande(Ico.Espacio, "Almacenamiento", "Cuánto espacio ocupan tus chats y cómo liberarlo", "hub_almacenamiento", alAlmacenamiento)
        BotonGrande(Ico.Candado, "Auditoría y privacidad", "Qué se guarda y qué pasa con lo que borran", "hub_auditoria", alAuditoria)
        BotonGrande(Ico.Ajustes, "Ajustes", "Seguridad, descargas, aspecto y más", "hub_ajustes", alAjustes)
        if (completo) {
            TituloSeccion("Más herramientas", Modifier.testTag("titulo_mas_herramientas"))
            BotonGrande(Ico.Responder, "Comandos propios", "Tus comandos: !horario, !precios…", "hub_comandos", alComandos)
            BotonGrande(Ico.Agregar, "Módulos", "Extras opcionales, apagados hasta que los enciendas", "hub_modulos", alModulos)
            BotonGrande(Ico.Reloj, "Límites e iniciativa", "Lo que el bot escribe solo, topes y No molestar", "hub_limites", alLimites)
            BotonGrande(Ico.Estadisticas, "Actividad del bot", "Qué llegó, qué respondió y por qué no", "hub_actividad", alActividad)
            BotonGrande(Ico.Aviso, "Laboratorio", "Pruebas experimentales, apagadas de fábrica", "hub_laboratorio", alLaboratorio)
        }
    }
}

// ---------- aviso único del modo Simple ----------

/** Se muestra una sola vez a quien actualizó desde la 0.5.0. `yaCompleto` = se le dejó en Completo porque tiene módulos encendidos. */
@Composable
fun AvisoModoSimple(yaCompleto: Boolean, alCompleto: () -> Unit, alSimple: () -> Unit) {
    AlertDialog(
        onDismissRequest = {}, modifier = Modifier.testTag("aviso_modo"),
        title = { Text(if (yaCompleto) "Te dejamos en modo Completo" else "La app ahora es más simple") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Text(
                    if (yaCompleto) "Como ya usas módulos, sigues viendo todo. Si quieres menos botones, puedes pasar al modo Simple."
                    else "Ahora ves lo básico: Chats, Personas, Grupos y Bot. Los comandos propios, los módulos, el Laboratorio y los límites siguen ahí, en el modo Completo.",
                )
                Text("No se borró ni se apagó nada. Puedes cambiar de modo cuando quieras en Ajustes → Avanzado.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, textAlign = TextAlign.Start)
            }
        },
        confirmButton = { TextButton(if (yaCompleto) alSimple else alCompleto, Modifier.testTag(if (yaCompleto) "aviso_modo_simple" else "aviso_modo_completo")) { Text(if (yaCompleto) "Pasar a Simple" else "Ver todo (modo Completo)") } },
        dismissButton = { TextButton(if (yaCompleto) alCompleto else alSimple, Modifier.testTag("aviso_modo_ok")) { Text(if (yaCompleto) "Seguir en Completo" else "Está bien así") } },
    )
}
