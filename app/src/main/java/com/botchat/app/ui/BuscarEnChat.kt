@file:OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import com.botchat.app.datos.Msg
import com.botchat.app.datos.Repo
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/**
 * Buscar una palabra dentro de un chat (menú ⋮ → Buscar). Usa `BuscarEnChat` de Go: recorre los mensajes de ese chat (están cifrados, se
 * descifran al buscar), sin importar mayúsculas ni acentos. Con muchos mensajes puede tardar un poco: se avisa y hay «Buscar más».
 */
class BusquedaChat(private val botId: Long, private val jid: String) {
    var activa by mutableStateOf(false)
    var consulta by mutableStateOf("")
    var items by mutableStateOf<List<Msg>>(emptyList())
        private set
    var cargando by mutableStateOf(false)
        private set
    var fin by mutableStateOf(true)
        private set
    var buscada by mutableStateOf("")
        private set
    var error by mutableStateOf<String?>(null)
        private set
    private var sigTs = 0L
    private var sigId = ""

    fun abrir() { activa = true }
    fun cerrar() { activa = false; consulta = ""; items = emptyList(); buscada = ""; fin = true; error = null }

    suspend fun buscar() {
        val q = consulta.trim()
        sigTs = 0; sigId = ""; items = emptyList(); fin = true; error = null; buscada = ""
        if (q.length < 2) return
        cargando = true
        runCatching { Repo.buscarEnChat(botId, jid, q) }
            .onSuccess { if (q == consulta.trim()) { items = it.items; fin = it.fin; sigTs = it.sigTs; sigId = it.sigId; buscada = q } }
            .onFailure { error = it.message ?: "No se pudo buscar" }
        cargando = false
    }

    suspend fun mas() {
        if (cargando || fin) return
        val q = buscada
        cargando = true
        runCatching { Repo.buscarEnChat(botId, jid, q, sigTs, sigId) }
            .onSuccess { if (q == consulta.trim()) { items = items + it.items; fin = it.fin; sigTs = it.sigTs; sigId = it.sigId } }
            .onFailure { error = it.message ?: "No se pudo buscar" }
        cargando = false
    }
}

/** Barra superior del modo búsqueda: un campo grande para escribir y una X para salir. */
@Composable
fun BarraBuscarEnChat(b: BusquedaChat) {
    val foco = remember { FocusRequester() }
    LaunchedEffect(Unit) { runCatching { foco.requestFocus() } }
    LaunchedEffect(b.consulta) { delay(350); b.buscar() }
    TopAppBar(
        navigationIcon = { IconButton({ b.cerrar() }, Modifier.testTag("cerrar_busqueda")) { Icon(Ico.Atras, "Salir de la búsqueda") } },
        title = {
            TextField(
                value = b.consulta, onValueChange = { b.consulta = it }, singleLine = true,
                placeholder = { Text("Buscar en este chat") },
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
                colors = TextFieldDefaults.colors(
                    focusedContainerColor = Color.Transparent, unfocusedContainerColor = Color.Transparent,
                    focusedIndicatorColor = Color.Transparent, unfocusedIndicatorColor = Color.Transparent,
                ),
                modifier = Modifier.fillMaxWidth().focusRequester(foco).testTag("campo_buscar_chat"),
            )
        },
        actions = { if (b.consulta.isNotEmpty()) IconButton({ b.consulta = "" }, Modifier.testTag("borrar_busqueda")) { Icon(Ico.Cerrar, "Borrar texto") } },
        colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
    )
}

/** Lista de resultados que tapa el chat mientras se busca. Tocar uno lleva a ese mensaje. */
@Composable
fun ResultadosBuscarEnChat(b: BusquedaChat, modifier: Modifier = Modifier, alElegir: (Msg) -> Unit) {
    androidx.compose.material3.Surface(modifier.fillMaxSize().testTag("resultados_buscar_chat"), color = MaterialTheme.colorScheme.surface) {
        val q = b.consulta.trim()
        Column(Modifier.fillMaxSize()) {
            when {
                q.length < 2 -> Box(Modifier.fillMaxSize()) { EstadoVacio(Ico.Buscar, "Busca en este chat", "Escribe una palabra. No importan las mayúsculas ni los acentos.", Modifier.align(Alignment.Center)) }
                b.error != null -> Box(Modifier.fillMaxSize()) { EstadoVacio(Ico.Aviso, "No se pudo buscar", b.error ?: "", Modifier.align(Alignment.Center)) }
                b.cargando && b.items.isEmpty() -> Box(Modifier.fillMaxSize(), Alignment.Center) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(Esp.m)) {
                        CircularProgressIndicator()
                        Text("Buscando…", Modifier.testTag("buscando_texto"))
                    }
                }
                b.buscada == q && b.items.isEmpty() -> Box(Modifier.fillMaxSize()) { EstadoVacio(Ico.Buscar, "Sin resultados", "No hay mensajes con «$q» en este chat.", Modifier.align(Alignment.Center).testTag("sin_resultados_chat")) }
                else -> LazyColumn(Modifier.fillMaxSize()) {
                    items(b.items, key = { it.id }) { m ->
                        Column(Modifier.fillMaxWidth().clickable { alElegir(m) }.padding(horizontal = Esp.l, vertical = Esp.m).testTag("resultado_${m.id}")) {
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Text(if (m.esMio) "Yo" else m.remitente.ifBlank { "Contacto" }, Modifier.weight(1f), style = MaterialTheme.typography.titleSmall, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                Text(fechaCorta(m.ts), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                            Text(m.texto.ifBlank { m.nombre }, style = MaterialTheme.typography.bodyMedium, maxLines = 2, overflow = TextOverflow.Ellipsis)
                        }
                        HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f))
                    }
                    item {
                        if (b.cargando) Box(Modifier.fillMaxWidth().padding(Esp.l), Alignment.Center) { CircularProgressIndicator() }
                        else if (!b.fin) {
                            val scope = androidx.compose.runtime.rememberCoroutineScope()
                            Box(Modifier.fillMaxWidth().padding(Esp.l), Alignment.Center) {
                                Button({ scope.launch { b.mas() } }, Modifier.testTag("buscar_mas")) { Text("Buscar más") }
                            }
                        }
                    }
                }
            }
        }
    }
}

private fun fechaCorta(ts: Long): String =
    java.text.SimpleDateFormat("d MMM yyyy, HH:mm", java.util.Locale.forLanguageTag("es")).format(java.util.Date(ts * 1000))
