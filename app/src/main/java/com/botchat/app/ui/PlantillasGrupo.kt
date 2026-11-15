@file:OptIn(ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import com.botchat.app.datos.ChatItem
import com.botchat.app.datos.FiltroGrupo
import com.botchat.app.datos.Repo
import com.botchat.app.datos.guardarHorarioGrupo
import com.botchat.app.datos.horarioGrupo
import kotlinx.coroutines.launch

/** Un filtro dentro de una plantilla: si queda encendido y qué hace el bot cuando salta. */
data class AjusteFiltro(val activo: Boolean, val accion: String)

/** Un conjunto de filtros listo para aplicar a un grupo con un toque. */
data class PlantillaGrupo(val id: String, val nombre: String, val resumen: String, val filtros: Map<String, AjusteFiltro>)

val PLANTILLAS_GRUPO = listOf(
    PlantillaGrupo(
        "familia", "Familia", "Sin filtros: nadie se queda sin escribir. Solo te aviso si algo se pasa.",
        mapOf("enlaces" to AjusteFiltro(false, "avisarme"), "palabras" to AjusteFiltro(false, "avisarme"), "repetidos" to AjusteFiltro(false, "avisarme"), "flood" to AjusteFiltro(false, "avisarme"), "stickers" to AjusteFiltro(false, "avisarme"), "audios" to AjusteFiltro(false, "avisarme"), "reenviados" to AjusteFiltro(false, "avisarme")),
    ),
    PlantillaGrupo(
        "comunidad", "Comunidad", "Cuida el ritmo: avisa con amabilidad a quien repite lo mismo, manda muchos mensajes o llena el grupo de stickers.",
        mapOf("enlaces" to AjusteFiltro(false, "avisarme"), "palabras" to AjusteFiltro(false, "avisarme"), "repetidos" to AjusteFiltro(true, "avisar_grupo"), "flood" to AjusteFiltro(true, "avisar_grupo"), "stickers" to AjusteFiltro(true, "avisar_grupo"), "audios" to AjusteFiltro(false, "avisarme"), "reenviados" to AjusteFiltro(false, "avisarme")),
    ),
    PlantillaGrupo(
        "ventas", "Ventas", "Sin repetidos, ráfagas, stickers en cadena ni reenviados: borra el mensaje y suma una advertencia. Los enlaces se dejan pasar.",
        mapOf("enlaces" to AjusteFiltro(false, "avisarme"), "palabras" to AjusteFiltro(false, "avisarme"), "repetidos" to AjusteFiltro(true, "borrar_advertir"), "flood" to AjusteFiltro(true, "borrar_advertir"), "stickers" to AjusteFiltro(true, "borrar_advertir"), "audios" to AjusteFiltro(false, "avisarme"), "reenviados" to AjusteFiltro(true, "borrar_advertir")),
    ),
    PlantillaGrupo(
        "estricto", "Estricto", "Todo vigilado: enlaces, palabras, repetidos, ráfagas, stickers, audios y reenviados. Borra y advierte.",
        mapOf("enlaces" to AjusteFiltro(true, "borrar_advertir"), "palabras" to AjusteFiltro(true, "borrar_advertir"), "repetidos" to AjusteFiltro(true, "borrar_advertir"), "flood" to AjusteFiltro(true, "borrar_advertir"), "stickers" to AjusteFiltro(true, "borrar_advertir"), "audios" to AjusteFiltro(true, "avisar_grupo"), "reenviados" to AjusteFiltro(true, "borrar_advertir")),
    ),
)

/** Aplica una plantilla a los filtros de un grupo: cambia si están encendidos y su acción; sus listas y límites se conservan. */
fun aplicarPlantilla(actuales: List<FiltroGrupo>, p: PlantillaGrupo): List<FiltroGrupo> =
    actuales.map { f -> p.filtros[f.filtro]?.let { a -> f.copy(activo = a.activo, accion = a.accion) } ?: f }

/** Plantillas con un toque y copiar la configuración de otro grupo (filtros, reglas escritas y horario). */
@Composable
fun SeccionPlantillasGrupo(botId: Long, jid: String, origen: ChatItem?, alPedirOrigen: () -> Unit, alLimpiarOrigen: () -> Unit, alTerminar: () -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val haptica = rememberHaptica()
    var plantilla by remember { mutableStateOf<PlantillaGrupo?>(null) }
    fun trabajo(ok: String, f: suspend () -> Unit) = scope.launch {
        runCatching { f() }.onSuccess { avisaToast(ctx, ok, false) }.onFailure { avisaToast(ctx, textoError(it), true) }
        alTerminar()
    }
    Card(Modifier.fillMaxWidth().padding(horizontal = Esp.l).testTag("tarjeta_plantillas"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Text("Elige cómo quieres que el bot cuide este grupo. Después puedes ajustar cada filtro a tu gusto.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                PLANTILLAS_GRUPO.forEach { p -> OutlinedButton({ haptica(Toque.Suave); plantilla = p }, Modifier.testTag("plantilla_${p.id}")) { Text(p.nombre) } }
            }
            OutlinedButton({ haptica(Toque.Suave); alPedirOrigen() }, Modifier.fillMaxWidth().testTag("copiar_de_otro")) { Text("Copiar la configuración de otro grupo") }
        }
    }
    plantilla?.let { p ->
        AlertDialog(
            onDismissRequest = { plantilla = null }, title = { Text("Plantilla «${p.nombre}»") },
            text = { Column(verticalArrangement = Arrangement.spacedBy(Esp.s)) { Text(p.resumen); Text("Cambia qué filtros están encendidos y qué hacen. Tus listas de palabras y enlaces se conservan.", style = MaterialTheme.typography.bodySmall) } },
            confirmButton = {
                TextButton({
                    plantilla = null
                    trabajo("Listo: aplicada «${p.nombre}»") { aplicarPlantilla(Repo.filtrosGrupo(botId, jid), p).forEach { Repo.guardarFiltroGrupo(botId, it) } }
                }, Modifier.testTag("confirmar_plantilla")) { Text("Aplicar") }
            },
            dismissButton = { TextButton({ plantilla = null }) { Text("Cancelar") } },
        )
    }
    origen?.let { o ->
        AlertDialog(
            onDismissRequest = alLimpiarOrigen, title = { Text("¿Copiar de «${o.nombre}»?") },
            text = { Text("Se copian los filtros, las reglas escritas y el horario de cierre. Lo que tiene este grupo se reemplaza. Las personas, advertencias y baneados no se tocan.") },
            confirmButton = {
                TextButton({
                    alLimpiarOrigen()
                    trabajo("Listo: copié la configuración de ${o.nombre}") {
                        Repo.filtrosGrupo(botId, o.jid).forEach { Repo.guardarFiltroGrupo(botId, it.copy(chat = jid)) }
                        Repo.guardarReglasGrupo(botId, jid, Repo.reglasGrupo(botId, o.jid))
                        Repo.guardarHorarioGrupo(botId, jid, Repo.horarioGrupo(botId, o.jid).copy(ultimo = ""))
                    }
                }, Modifier.testTag("confirmar_copiar")) { Text("Copiar") }
            },
            dismissButton = { TextButton(alLimpiarOrigen) { Text("Cancelar") } },
        )
    }
}
