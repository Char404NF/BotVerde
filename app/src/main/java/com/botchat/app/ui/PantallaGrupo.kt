@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Intent
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
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
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
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.EstadoGrupo
import com.botchat.app.datos.InfoGrupo
import com.botchat.app.datos.Repo
import kotlinx.coroutines.launch

internal fun textoError(e: Throwable) = e.message?.takeIf { it.isNotBlank() }?.let { it.replaceFirstChar { c -> c.uppercase() } } ?: "No se pudo"

/** Lo que el bot no puede hacer si no administra el grupo, dicho una sola vez y claro. */
internal const val TEXTO_NECESITA_ADMIN = "Necesito ser administrador del grupo para esto."

/** Tarjeta "Soy administrador / No soy administrador" con la explicación de qué se puede y qué no. */
@Composable
internal fun TarjetaEstadoAdmin(estado: EstadoGrupo?, alRefrescar: () -> Unit) {
    Card(Modifier.fillMaxWidth().testTag("grupo_estado"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            if (estado == null) { CircularProgressIndicator(Modifier.size(24.dp)); return@Column }
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(if (estado.soyAdmin) Ico.Listo else Ico.Candado, null, Modifier.size(24.dp), tint = if (estado.soyAdmin) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant)
                Spacer(Modifier.width(Esp.m))
                Text(
                    when { !estado.conectado -> "Sin conexión"; !estado.enGrupo -> "Ya no estás en este grupo"; estado.soyAdmin -> "Soy administrador"; else -> "No soy administrador" },
                    Modifier.testTag("grupo_estado_titulo"), style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold,
                )
            }
            if (estado.motivo.isNotBlank()) Text(estado.motivo, Modifier.testTag("grupo_estado_motivo"), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            else if (estado.soyAdmin) Text("Puedo cerrar el grupo, sacar a alguien, borrar mensajes y cambiar los ajustes.", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            OutlinedButton(alRefrescar, Modifier.testTag("grupo_refrescar")) { Text("Volver a comprobar") }
        }
    }
}

/** Una línea gris debajo de lo que está desactivado por no ser administrador. */
@Composable
internal fun AvisoAdmin(visible: Boolean) {
    if (visible) Text(TEXTO_NECESITA_ADMIN, Modifier.padding(horizontal = Esp.l).testTag("aviso_admin"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
}

@Composable
internal fun SeccionGrupo(titulo: String, contenido: @Composable () -> Unit) {
    Card(Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(vertical = Esp.m), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Text(titulo, Modifier.padding(horizontal = Esp.l), style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary)
            contenido()
        }
    }
}

@Composable
private fun FilaInterruptor(titulo: String, texto: String, valor: Boolean, activo: Boolean, etiqueta: String, alCambiar: (Boolean) -> Unit) {
    Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l), verticalAlignment = Alignment.CenterVertically) {
        Column(Modifier.weight(1f)) { Text(titulo, style = MaterialTheme.typography.bodyLarge); Text(texto, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        Switch(valor, alCambiar, Modifier.testTag(etiqueta), enabled = activo)
    }
}

/**
 * Administrar un grupo. Arriba dice si el bot es administrador; todo lo que necesita serlo queda desactivado y lo explica. Lo que no
 * necesita serlo (información, administradores, mencionar a todos, ranking, inactivos, reglas) siempre se puede.
 */
@Composable
fun PantallaGrupo(
    botId: Long, jid: String, alVolver: () -> Unit, alAbrirMiembros: (String) -> Unit = {}, alAbrirFiltros: () -> Unit = {}, alAbrirActividad: () -> Unit = {},
) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var version by remember { mutableIntStateOf(0) }
    var refrescar by remember { mutableStateOf(false) }
    val estado by produceState<EstadoGrupo?>(null, jid, version) { value = runCatching { Repo.estadoGrupo(botId, jid, refrescar) }.getOrElse { EstadoGrupo(false, false, false, textoError(it)) }; refrescar = false }
    val info by produceState<Result<InfoGrupo>?>(null, jid, version) { value = runCatching { Repo.infoGrupo(botId, jid) } }
    val admin = estado?.soyAdmin == true
    fun hacer(ok: String? = null, accion: suspend () -> Unit) = scope.launch {
        runCatching { accion() }.onSuccess { if (ok != null) avisaToast(ctx, ok, false) }.onFailure { avisaToast(ctx, textoError(it), true) }
        refrescar = true; version++
    }
    var nombre by remember(jid) { mutableStateOf<String?>(null) }
    var descripcion by remember(jid) { mutableStateOf<String?>(null) }
    var reglas by remember(jid) { mutableStateOf<String?>(null) }
    var textoTodos by remember(jid) { mutableStateOf("") }
    var enlace by remember(jid) { mutableStateOf<String?>(null) }
    var dialogo by remember { mutableStateOf<String?>(null) }
    val i = info?.getOrNull()
    androidx.compose.runtime.LaunchedEffect(jid) { reglas = runCatching { Repo.reglasGrupo(botId, jid) }.getOrDefault("") }
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text(i?.nombre?.ifBlank { null } ?: "Grupo") }) }) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.l)) {
            TarjetaEstadoAdmin(estado) { haptica(Toque.Suave); refrescar = true; version++ }

            // ----- Información (sin ser administrador) -----
            SeccionGrupo("Información") {
                val inf = info
                when {
                    inf == null -> CircularProgressIndicator(Modifier.padding(horizontal = Esp.l).size(24.dp))
                    inf.isFailure -> Text(textoError(inf.exceptionOrNull()!!), Modifier.padding(horizontal = Esp.l).testTag("grupo_info_error"), color = MaterialTheme.colorScheme.onSurfaceVariant)
                    else -> {
                        val g = inf.getOrThrow()
                        Column(Modifier.padding(horizontal = Esp.l).testTag("grupo_info"), verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                            if (g.descripcion.isNotBlank()) Text(g.descripcion, style = MaterialTheme.typography.bodyMedium)
                            Text("${plural(g.miembros.toLong(), "miembro", "miembros")} · ${plural(g.admins.size.toLong(), "administrador", "administradores")}", Modifier.testTag("grupo_conteos"), style = MaterialTheme.typography.bodyMedium)
                            Text("Creado: ${g.creadoTexto.ifBlank { "no se sabe" }}" + (if (g.creadoPor.isNotBlank()) " por ${g.creadoPor}" else ""), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            Text("Escriben: ${if (g.soloAdmins) "solo administradores" else "todos"} · Cambian nombre y foto: ${if (g.infoBloqueada) "solo administradores" else "todos"}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            Text("Para entrar: ${if (g.aprobacionIngreso) "hay que ser aprobado" else "basta el enlace"}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            if (g.admins.isNotEmpty()) Text("Administradores: " + g.admins.joinToString(", ") { it.nombre }, Modifier.testTag("grupo_admins"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                }
            }

            // ----- Ajustes del grupo (administrador) -----
            SeccionGrupo("Ajustes del grupo") {
                FilaInterruptor("Solo administradores escriben", "Cierra el grupo: nadie más puede mandar mensajes.", i?.soloAdmins == true, admin, "sw_grupo_cerrado") { v ->
                    haptica(Toque.Fuerte); hacer(if (v) "Listo: el grupo quedó cerrado" else "Listo: el grupo quedó abierto") { Repo.cerrarGrupo(botId, jid, v) }
                }
                FilaInterruptor("Solo administradores cambian la información", "Bloquea nombre, foto y descripción.", i?.infoBloqueada == true, admin, "sw_grupo_bloqueado") { v ->
                    haptica(Toque.Suave); hacer(if (v) "Listo: la información quedó bloqueada" else "Listo: la información quedó libre") { Repo.bloquearInfoGrupo(botId, jid, v) }
                }
                val nom = nombre ?: i?.nombre.orEmpty()
                OutlinedTextField(nom, { nombre = it }, Modifier.fillMaxWidth().padding(horizontal = Esp.l).testTag("grupo_nombre_campo"), label = { Text("Nombre del grupo") }, singleLine = true, enabled = admin)
                Button({ haptica(Toque.Suave); hacer("Listo: cambié el nombre") { Repo.renombrarGrupo(botId, jid, nom.trim()) } }, Modifier.padding(horizontal = Esp.l).testTag("grupo_nombre_guardar"),
                    enabled = admin && nom.isNotBlank() && nom.trim() != i?.nombre) { Text("Cambiar nombre") }
                val desc = descripcion ?: i?.descripcion.orEmpty()
                OutlinedTextField(desc, { descripcion = it }, Modifier.fillMaxWidth().padding(horizontal = Esp.l).testTag("grupo_descripcion_campo"), label = { Text("Descripción") }, minLines = 2, enabled = admin)
                Button({ haptica(Toque.Suave); hacer(if (desc.isBlank()) "Listo: borré la descripción" else "Listo: cambié la descripción") { Repo.descripcionGrupo(botId, jid, desc.trim()) } },
                    Modifier.padding(horizontal = Esp.l).testTag("grupo_descripcion_guardar"), enabled = admin && desc.trim() != i?.descripcion?.trim()) { Text("Cambiar descripción") }
                AvisoAdmin(!admin)
            }

            // ----- Reglas (mías, no necesitan administrador) -----
            SeccionGrupo("Reglas del grupo") {
                Text("Se guardan solo en este teléfono (WhatsApp no tiene un lugar para ellas) y las puedes ver con el comando de reglas.", Modifier.padding(horizontal = Esp.l), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                OutlinedTextField(reglas.orEmpty(), { reglas = it }, Modifier.fillMaxWidth().padding(horizontal = Esp.l).testTag("grupo_reglas_campo"), label = { Text("Reglas") }, minLines = 2)
                Button({ haptica(Toque.Suave); hacer("Reglas guardadas") { Repo.guardarReglasGrupo(botId, jid, reglas.orEmpty().trim()) } }, Modifier.padding(horizontal = Esp.l).testTag("grupo_reglas_guardar")) { Text("Guardar reglas") }
            }

            // ----- Gente -----
            SeccionGrupo("Gente") {
                NavFila("Miembros", "Ver quién está y, siendo administrador, subir, bajar o sacar a alguien.", "grupo_abrir_miembros") { alAbrirMiembros("miembros") }
                NavFila("Solicitudes de ingreso", "Aceptar o rechazar a quien pide entrar.", "grupo_abrir_solicitudes", activo = true) { alAbrirMiembros("solicitudes") }
                if (!admin) AvisoAdmin(true)
            }

            // ----- Enlace -----
            SeccionGrupo("Enlace de invitación") {
                Row(Modifier.padding(horizontal = Esp.l), horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    OutlinedButton({ haptica(Toque.Suave); scope.launch { runCatching { Repo.enlaceGrupo(botId, jid, false) }.onSuccess { enlace = it }.onFailure { avisaToast(ctx, textoError(it), true) } } },
                        Modifier.testTag("grupo_ver_enlace"), enabled = admin) { Text("Ver enlace") }
                    OutlinedButton({ haptica(Toque.Fuerte); dialogo = "enlace_nuevo" }, Modifier.testTag("grupo_enlace_nuevo"), enabled = admin) { Text("Crear uno nuevo") }
                }
                enlace?.let { l ->
                    Text(l, Modifier.padding(horizontal = Esp.l).testTag("grupo_enlace_texto"), style = MaterialTheme.typography.bodyMedium)
                    Row(Modifier.padding(horizontal = Esp.l), horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                        OutlinedButton({
                            ctx.getSystemService(ClipboardManager::class.java).setPrimaryClip(ClipData.newPlainText("enlace", l)); avisaToast(ctx, "Enlace copiado", false)
                        }, Modifier.testTag("grupo_copiar_enlace")) { Text("Copiar") }
                        OutlinedButton({
                            ctx.startActivity(Intent.createChooser(Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT, l), "Compartir enlace").addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
                        }, Modifier.testTag("grupo_compartir_enlace")) { Text("Compartir") }
                    }
                }
                AvisoAdmin(!admin)
            }

            // ----- Mencionar a todos (sin administrador) -----
            SeccionGrupo("Mencionar a todos") {
                Text("Manda un mensaje que menciona a cada persona del grupo. Si el grupo es grande salen varios mensajes y hay un tope (se cambia en Permisos y comandos).", Modifier.padding(horizontal = Esp.l), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                OutlinedTextField(textoTodos, { textoTodos = it }, Modifier.fillMaxWidth().padding(horizontal = Esp.l).testTag("grupo_todos_campo"), label = { Text("Mensaje (opcional)") }, minLines = 1)
                Button({ haptica(Toque.Fuerte); dialogo = "todos" }, Modifier.padding(horizontal = Esp.l).testTag("grupo_todos"), enabled = estado?.conectado == true) { Text("Mencionar a todos") }
            }

            // ----- Filtros y actividad -----
            SeccionGrupo("Orden y actividad") {
                NavFila("Filtros del grupo", "Enlaces, palabras prohibidas, repetidos y flood. Si no soy administrador solo te aviso.", "grupo_abrir_filtros") { alAbrirFiltros() }
                NavFila("Actividad", "Quién escribe más y quién no escribe hace tiempo.", "grupo_abrir_actividad") { alAbrirActividad() }
            }
        }
    }
    when (dialogo) {
        "enlace_nuevo" -> AlertDialog(
            onDismissRequest = { dialogo = null }, title = { Text("¿Crear un enlace nuevo?") },
            text = { Text("El enlace anterior deja de funcionar: quien lo tenga ya no podrá entrar con él. Tendrás que compartir el nuevo.") },
            confirmButton = { TextButton({ dialogo = null; scope.launch { runCatching { Repo.enlaceGrupo(botId, jid, true) }.onSuccess { enlace = it; avisaToast(ctx, "Listo: enlace nuevo", false) }.onFailure { avisaToast(ctx, textoError(it), true) } } }, Modifier.testTag("confirmar_enlace_nuevo")) { Text("Crear uno nuevo", color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton({ dialogo = null }) { Text("Cancelar") } },
        )
        "todos" -> AlertDialog(
            onDismissRequest = { dialogo = null }, title = { Text("¿Mencionar a todos?") },
            text = { Text("Se manda ahora un mensaje a este grupo que menciona a todas las personas, y a cada una le suena el teléfono. Úsalo solo si de verdad es importante.") },
            confirmButton = {
                TextButton({
                    dialogo = null
                    scope.launch {
                        runCatching { Repo.mencionarATodos(botId, jid, textoTodos.trim()) }
                            .onSuccess { r -> textoTodos = ""; avisaToast(ctx, if (r.aviso.isNotBlank()) r.aviso else "Listo: mencioné a ${r.mencionadas} personas", r.truncado) }
                            .onFailure { avisaToast(ctx, textoError(it), true) }
                    }
                }, Modifier.testTag("confirmar_todos")) { Text("Mencionar") }
            },
            dismissButton = { TextButton({ dialogo = null }) { Text("Cancelar") } },
        )
    }
}

@Composable
internal fun NavFila(titulo: String, texto: String, etiqueta: String, activo: Boolean = true, alAbrir: () -> Unit) {
    Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l), verticalAlignment = Alignment.CenterVertically) {
        Column(Modifier.weight(1f)) { Text(titulo, style = MaterialTheme.typography.bodyLarge); Text(texto, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        Spacer(Modifier.width(Esp.s))
        OutlinedButton(alAbrir, Modifier.testTag(etiqueta), enabled = activo) { Text("Abrir") }
    }
}
