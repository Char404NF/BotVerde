@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import com.botchat.app.ui.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
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
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.NoEntendio
import com.botchat.app.datos.Repo
import kotlinx.coroutines.launch

/** Lo que el menú no entendió: para saber qué opciones o palabras agregar. Se guarda cifrado y se borra solo con los días que elijas. */
@Composable
fun PantallaNoEntendio(botId: Long, menuId: Long, titulo: String, alVolver: () -> Unit) {
    val scope = rememberCoroutineScope()
    var version by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) { Repo.cambios.collect { evs -> if ("noentendio" in evs) version++ } }
    val lista by produceState<List<NoEntendio>?>(null, version) { value = runCatching { Repo.noEntendio(botId, menuId) }.getOrDefault(emptyList()) }
    val hayDatos = !lista.isNullOrEmpty()
    var confirma by remember { mutableStateOf(false) }
    Scaffold(
        topBar = {
            TopAppBar(
                navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } },
                title = { Text(titulo.ifBlank { "Lo que no entendió" }) },
                actions = { IconButton({ confirma = true }, Modifier.testTag("borrar_no_entendio"), enabled = hayDatos) { Icon(Ico.Borrar, "Borrar la lista") } },
            )
        },
    ) { relleno ->
        val datos = lista
        when {
            datos == null -> Box(Modifier.fillMaxSize().padding(relleno), Alignment.Center) { CircularProgressIndicator() }
            datos.isEmpty() -> EstadoVacio(Ico.Info, "Nada por aquí", "Cuando alguien escriba algo que el menú no entienda, aparecerá aquí para que puedas mejorarlo.", Modifier.padding(relleno).padding(top = 60.dp))
            else -> LazyColumn(Modifier.fillMaxSize().padding(relleno).testTag("lista_no_entendio"), contentPadding = androidx.compose.foundation.layout.PaddingValues(Esp.l, Esp.s, Esp.l, 32.dp), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                item { Text("Se guardan cifrados, sin el nombre de quien lo escribió, y se borran solos pasado el tiempo que elijas en Ajustes → Avanzado → Menús.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                items(datos, key = { it.id }) { x ->
                    Card(Modifier.fillMaxWidth().testTag("no_entendio_${x.id}"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
                        Column(Modifier.padding(Esp.m)) {
                            Text("“${x.texto}”", style = MaterialTheme.typography.bodyLarge)
                            Text(x.menu.ifBlank { "Menú" } + " · " + etiquetaDia(x.ts) + " " + horaDe(x.ts), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                }
            }
        }
    }
    if (confirma) AlertDialog(
        onDismissRequest = { confirma = false }, title = { Text("¿Borrar la lista?") }, text = { Text("Se borra para siempre de este teléfono.") },
        confirmButton = { TextButton({ confirma = false; scope.launch { runCatching { Repo.borrarNoEntendio(botId, menuId) }; version++ } }, Modifier.testTag("confirmar_borrar_no_entendio")) { Text("Borrar", color = MaterialTheme.colorScheme.error) } },
        dismissButton = { TextButton({ confirma = false }) { Text("Cancelar") } },
    )
}
