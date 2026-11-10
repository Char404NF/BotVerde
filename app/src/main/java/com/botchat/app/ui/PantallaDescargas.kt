@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LargeTopAppBar
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.input.nestedscroll.nestedScroll
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.Ajustes
import kotlin.math.roundToInt

/** Ajustes → Descargas: qué se baja solo y por qué conexión. */
@Composable
fun PantallaDescargas(ajustes: Ajustes, alVolver: () -> Unit) {
    val barra = TopAppBarDefaults.exitUntilCollapsedScrollBehavior()
    val d by ajustes.descargas.collectAsState()
    Scaffold(
        modifier = Modifier.nestedScroll(barra.nestedScrollConnection),
        topBar = { LargeTopAppBar(scrollBehavior = barra, navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Descargas") }) },
    ) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(bottom = 32.dp)) {
            Seccion("Descarga automática") {
                Text("Elige qué se descarga solo y por qué conexión. Lo demás queda con el botón Descargar.", Modifier.padding(horizontal = Esp.l), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                Row(Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s)) {
                    Spacer(Modifier.weight(1f)); Text("Wi-Fi", Modifier.width(72.dp), style = MaterialTheme.typography.labelLarge); Text("Datos", Modifier.width(72.dp), style = MaterialTheme.typography.labelLarge)
                }
                FilaDescarga("Fotos", Ico.Foto, d.foto) { r -> ajustes.cambiarDescargas { it.copy(foto = r) } }
                FilaDescarga("Audios", Ico.Audio, d.audio) { r -> ajustes.cambiarDescargas { it.copy(audio = r) } }
                FilaDescarga("Documentos", Ico.Documento, d.documento) { r -> ajustes.cambiarDescargas { it.copy(documento = r) } }
                FilaDescarga("Videos", Ico.Video, d.video) { r -> ajustes.cambiarDescargas { it.copy(video = r) } }
                Deslizador("Tamaño máximo de videos", "${d.videoMaxMb} MB", d.videoMaxMb.toFloat(), 10f..200f, 18) { ajustes.cambiarDescargas { x -> x.copy(videoMaxMb = (it / 10).roundToInt() * 10) } }
                Deslizador("Espacio libre mínimo", if (d.pisoLibreMb >= 1000) "${"%.1f".format(d.pisoLibreMb / 1000f)} GB" else "${d.pisoLibreMb} MB", d.pisoLibreMb.toFloat(), 100f..3000f, 28, "Con menos espacio libre se pausan las descargas automáticas.") { ajustes.cambiarDescargas { x -> x.copy(pisoLibreMb = (it / 100).roundToInt() * 100) } }
            }
        }
    }
}
