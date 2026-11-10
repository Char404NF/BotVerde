@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
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
import androidx.compose.material3.LargeTopAppBar
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.nestedscroll.nestedScroll
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import androidx.fragment.app.FragmentActivity
import com.botchat.app.datos.Ajustes
import com.botchat.app.datos.Ajustes.Bloqueo
import com.botchat.app.seguridad.Huella

/** Pasos a pantalla completa que usa Seguridad (crear o pedir un PIN). */
private sealed interface Flujo {
    data object Ninguno : Flujo
    data class CrearPin(val destino: Bloqueo, val titulo: String) : Flujo
    data class VerificarParaCambiar(val destino: Bloqueo) : Flujo
    data object VerificarParaQuitar : Flujo
}

/** Texto que explica el límite del PIN cuando el teléfono no tiene bloqueo de pantalla. */
const val AVISO_PIN_LIGERO = "Tu teléfono no tiene bloqueo de pantalla. Sin él, el PIN de Bot Chat es solo una protección ligera."

/** Ajustes → Seguridad: proteger la app (sin bloqueo, PIN o huella), tiempo de bloqueo y capturas de pantalla. */
@Composable
fun PantallaSeguridad(ajustes: Ajustes, alVolver: () -> Unit) {
    val ctx = LocalContext.current
    val actividad = ctx as? FragmentActivity
    val haptica = rememberHaptica()
    val barra = TopAppBarDefaults.exitUntilCollapsedScrollBehavior()
    val modo by ajustes.bloqueo.collectAsState()
    val tienePin by ajustes.tienePin.collectAsState()
    val seg by ajustes.bloqueoSeg.collectAsState()
    val secure by ajustes.secure.collectAsState()
    val huellaOk = remember { Huella.disponible(ctx) }
    val respaldoTelefono = remember { Huella.conRespaldoDelTelefono(ctx) }
    val telefonoSeguro = remember { Huella.telefonoTieneBloqueo(ctx) }
    var flujo by remember { mutableStateOf<Flujo>(Flujo.Ninguno) }
    var aviso by remember { mutableStateOf<String?>(null) }

    when (val f = flujo) {
        is Flujo.CrearPin -> {
            PantallaConfigurarPin(f.titulo, { pin ->
                ajustes.fijarPin(pin); ajustes.fijarBloqueo(f.destino); EstadoBloqueo.desbloquear(); flujo = Flujo.Ninguno
            }, { flujo = Flujo.Ninguno }, nota = if (telefonoSeguro) null else AVISO_PIN_LIGERO, textoOmitir = "Cancelar")
            return
        }
        is Flujo.VerificarParaCambiar -> {
            PantallaBloqueo(ajustes, "Cambiar el PIN", "Escribe tu PIN actual", alCancelar = { flujo = Flujo.Ninguno }) {
                flujo = Flujo.CrearPin(f.destino, "Elige un PIN nuevo")
            }
            return
        }
        Flujo.VerificarParaQuitar -> {
            PantallaBloqueo(ajustes, "Quitar el bloqueo", "Escribe tu PIN actual para quitarlo", alCancelar = { flujo = Flujo.Ninguno }) {
                ajustes.quitarBloqueo(); EstadoBloqueo.desbloquear(); flujo = Flujo.Ninguno
            }
            return
        }
        Flujo.Ninguno -> {}
    }

    fun elegir(destino: Bloqueo) {
        if (destino == modo) return
        haptica(Toque.Suave); aviso = null
        when (destino) {
            Bloqueo.NINGUNO ->
                if (tienePin) flujo = Flujo.VerificarParaQuitar
                else if (actividad != null) Huella.pedir(actividad, respaldoTelefono = true, alExito = { ajustes.quitarBloqueo(); EstadoBloqueo.desbloquear() })
                else { ajustes.quitarBloqueo(); EstadoBloqueo.desbloquear() }
            Bloqueo.PIN ->
                if (tienePin) { ajustes.fijarBloqueo(Bloqueo.PIN) } else flujo = Flujo.CrearPin(Bloqueo.PIN, "Elige tu PIN")
            Bloqueo.HUELLA -> when {
                !huellaOk -> aviso = "Este teléfono no tiene una huella registrada. Regístrala en los Ajustes del teléfono."
                tienePin -> ajustes.fijarBloqueo(Bloqueo.HUELLA)
                respaldoTelefono && actividad != null -> Huella.pedir(actividad, respaldoTelefono = true, alExito = { ajustes.fijarBloqueo(Bloqueo.HUELLA); EstadoBloqueo.desbloquear() })
                else -> flujo = Flujo.CrearPin(Bloqueo.HUELLA, "Crea un PIN de respaldo")
            }
        }
    }

    Scaffold(
        modifier = Modifier.nestedScroll(barra.nestedScrollConnection),
        topBar = { LargeTopAppBar(scrollBehavior = barra, navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Seguridad") }) },
    ) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(horizontal = Esp.l).padding(bottom = 32.dp), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Text("Proteger la app", style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.primary)
            Text("Por defecto Bot Chat se abre sin pedir nada. Puedes activar un bloqueo cuando quieras y quitarlo después.", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Opcion("Sin bloqueo", "La app se abre directo.", Ico.Abierto, modo == Bloqueo.NINGUNO, true, "op_ninguno") { elegir(Bloqueo.NINGUNO) }
            Opcion("PIN", "Un número de 4 a 6 dígitos.", Ico.Candado, modo == Bloqueo.PIN, true, "op_pin") { elegir(Bloqueo.PIN) }
            Opcion(
                "Huella", if (huellaOk) (if (respaldoTelefono) "Con el bloqueo del teléfono como respaldo." else "Con un PIN de respaldo.") else "Este teléfono no tiene huella registrada.",
                Ico.Huella, modo == Bloqueo.HUELLA, huellaOk, "op_huella",
            ) { elegir(Bloqueo.HUELLA) }
            aviso?.let { Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodyMedium) }

            AnimatedVisibility(modo != Bloqueo.NINGUNO) {
                Column(verticalArrangement = Arrangement.spacedBy(Esp.m)) {
                    Text("Pedir el desbloqueo", style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.primary)
                    val opciones = listOf(0 to "Enseguida", 60 to "1 min", 300 to "5 min")
                    SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth().testTag("tiempo_bloqueo")) {
                        opciones.forEachIndexed { i, (valor, texto) ->
                            SegmentedButton(seg == valor, { haptica(Toque.Suave); ajustes.fijarBloqueoSeg(valor) }, SegmentedButtonDefaults.itemShape(i, opciones.size)) { Text(texto) }
                        }
                    }
                    Text("Es el tiempo que puedes estar fuera de la app antes de que vuelva a pedirte el desbloqueo.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    if (tienePin) {
                        androidx.compose.material3.OutlinedButton({ flujo = Flujo.VerificarParaCambiar(modo) }, Modifier.fillMaxWidth()) { Text("Cambiar el PIN") }
                    }
                    if (!telefonoSeguro) {
                        Surface(shape = MaterialTheme.shapes.medium, color = MaterialTheme.colorScheme.tertiaryContainer) {
                            Row(Modifier.padding(Esp.m), verticalAlignment = Alignment.CenterVertically) {
                                Icon(Ico.Aviso, null, tint = MaterialTheme.colorScheme.onTertiaryContainer)
                                Spacer(Modifier.width(Esp.m))
                                Text(AVISO_PIN_LIGERO, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onTertiaryContainer)
                            }
                        }
                    }
                }
            }

            Text("Capturas de pantalla", style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.primary, modifier = Modifier.padding(top = Esp.s))
            Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                Row(Modifier.fillMaxWidth().padding(Esp.l), verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text("Evitar capturas de pantalla", style = MaterialTheme.typography.bodyLarge)
                        Text("También oculta la vista previa en las apps recientes.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    Switch(secure, { haptica(Toque.Suave); ajustes.fijarSecure(it) }, Modifier.testTag("sw_capturas"))
                }
            }
            Text("Tus mensajes siempre se guardan cifrados. El bloqueo solo decide quién puede abrir la app; quitarlo no toca tus datos.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

@Composable
private fun Opcion(titulo: String, texto: String, icono: androidx.compose.ui.graphics.vector.ImageVector, elegida: Boolean, habilitada: Boolean, etiqueta: String, alElegir: () -> Unit) {
    val colores = MaterialTheme.colorScheme
    Surface(
        onClick = alElegir, enabled = habilitada, modifier = Modifier.fillMaxWidth().testTag(etiqueta), shape = MaterialTheme.shapes.large,
        color = if (elegida) colores.primaryContainer else colores.surfaceContainer,
        border = BorderStroke(if (elegida) 2.dp else 1.dp, if (elegida) colores.primary else colores.outlineVariant),
    ) {
        Row(Modifier.padding(Esp.l), verticalAlignment = Alignment.CenterVertically) {
            Icon(icono, null, Modifier.size(28.dp), tint = if (!habilitada) colores.outline else if (elegida) colores.onPrimaryContainer else colores.onSurfaceVariant)
            Spacer(Modifier.width(Esp.l))
            Column(Modifier.weight(1f)) {
                Text(titulo, style = MaterialTheme.typography.titleMedium, color = if (elegida) colores.onPrimaryContainer else colores.onSurface)
                Text(texto, style = MaterialTheme.typography.bodyMedium, color = if (elegida) colores.onPrimaryContainer else colores.onSurfaceVariant)
            }
            RadioButton(elegida, null, enabled = habilitada)
        }
    }
}
