@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import androidx.activity.compose.BackHandler
import android.content.Context
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.AdjuntoSubido
import com.botchat.app.datos.BorradorAsistente
import com.botchat.app.datos.EstadoAsistente
import com.botchat.app.datos.GrupoAdmin
import com.botchat.app.datos.OpcionAsistente
import com.botchat.app.datos.Repo
import com.botchat.app.datos.ResultadoAsistente
import com.botchat.app.datos.asistenteAplicar
import com.botchat.app.datos.asistenteBase
import com.botchat.app.datos.asistenteDeshacer
import com.botchat.app.datos.asistenteEstado
import com.botchat.app.datos.asistenteGuardar
import com.botchat.app.datos.asistenteProbar
import com.botchat.app.datos.asistenteVisto
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

// Asistente «Crea tu bot» (0.6.0, E9): cinco pasos, uno por pantalla, con botones grandes (Siguiente / Saltar) y un resumen final.
// El borrador se guarda solo (cifrado, en Go) y se puede dejar a medias y retomar. Aplicar crea todo de golpe y se puede deshacer.

private const val PASOS = 5
private val ALTO_BOTON = 56.dp

/** Pantalla completa del asistente. `alEditarMenu` abre el editor completo del menú creado (submenús y formularios). */
@Composable
fun PantallaAsistenteBot(
    botId: Long, alSalir: () -> Unit, alEditarMenu: (Long) -> Unit = {}, alEditarBienvenida: () -> Unit = {},
) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var estado by remember { mutableStateOf<EstadoAsistente?>(null) }
    var b by remember { mutableStateOf<BorradorAsistente?>(null) }
    var cargado by remember { mutableStateOf(false) }
    var resultado by remember { mutableStateOf<ResultadoAsistente?>(null) }
    var ocupado by remember { mutableStateOf(false) }
    var confirmaDeshacer by remember { mutableStateOf(false) }

    LaunchedEffect(botId) {
        val e = runCatching { Repo.asistenteEstado(botId) }.getOrNull()
        estado = e
        b = e?.borrador ?: BorradorAsistente()
        cargado = true
        runCatching { Repo.asistenteVisto(botId) }
    }
    // Guarda solo, un momento después del último cambio (y al salir).
    LaunchedEffect(b) {
        val x = b ?: return@LaunchedEffect
        if (!cargado) return@LaunchedEffect
        delay(500)
        runCatching { Repo.asistenteGuardar(botId, x) }
    }
    fun guardarYSalir() {
        val x = b
        scope.launch { if (x != null) runCatching { Repo.asistenteGuardar(botId, x) }; alSalir() }
    }
    fun ir(paso: Int) {
        val x = b ?: return
        val nuevo = x.copy(paso = paso.coerceIn(1, PASOS + 1))
        b = nuevo
        scope.launch {
            runCatching { Repo.asistenteGuardar(botId, nuevo) }
            if (nuevo.paso > PASOS) estado = runCatching { Repo.asistenteEstado(botId) }.getOrNull() ?: estado
        }
    }

    val actual = b
    if (!cargado || actual == null) {
        Box(Modifier.fillMaxSize().testTag("asistente_cargando"), Alignment.Center) { CircularProgressIndicator() }
        return
    }
    val paso = actual.paso.coerceIn(1, PASOS + 1)
    val final = paso > PASOS
    val hecho = resultado != null || (estado?.aplicado == true && final)
    BackHandler { if (paso > 1 && !hecho) ir(paso - 1) else guardarYSalir() }

    Scaffold(
        topBar = {
            Column {
                TopAppBar(
                    navigationIcon = { IconButton({ if (paso > 1 && !hecho) ir(paso - 1) else guardarYSalir() }, Modifier.testTag("asistente_atras")) { Icon(Ico.Atras, "Atrás") } },
                    title = {
                        Column {
                            Text("Crea tu bot", maxLines = 1)
                            Text(if (final) "Resumen" else "Paso $paso de $PASOS", style = MaterialTheme.typography.bodySmall, modifier = Modifier.testTag("asistente_paso"))
                        }
                    },
                    actions = { TextButton({ guardarYSalir() }, Modifier.testTag("asistente_salir")) { Text("Seguir luego") } },
                )
                LinearProgressIndicator(progress = { (if (final) PASOS else paso - 1).toFloat() / PASOS }, Modifier.fillMaxWidth())
            }
        },
        bottomBar = {
            if (!final) {
                Surface(tonalElevation = 3.dp) {
                    Row(Modifier.fillMaxWidth().padding(Esp.l), horizontalArrangement = Arrangement.spacedBy(Esp.m)) {
                        OutlinedButton(
                            { b = actual.conSaltado(paso, true); ir(paso + 1) },
                            Modifier.weight(1f).heightIn(min = ALTO_BOTON).testTag("asistente_saltar"),
                        ) { Text("Saltar") }
                        Button(
                            { b = actual.conSaltado(paso, false); ir(paso + 1) },
                            Modifier.weight(1f).heightIn(min = ALTO_BOTON).testTag("asistente_siguiente"),
                        ) { Text(if (paso == PASOS) "Ver resumen" else "Siguiente"); Spacer(Modifier.width(Esp.s)); Icon(Ico.Siguiente, null) }
                    }
                }
            }
        },
    ) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.l)) {
            if (actual.saltado(paso) && !final) Text("Saltaste este paso: no se cambiará nada de esto. Toca Siguiente si cambias de idea.", Modifier.testTag("asistente_saltado"), color = MaterialTheme.colorScheme.onSurfaceVariant, style = MaterialTheme.typography.bodySmall)
            when (paso) {
                1 -> PasoDonde(botId, actual) { b = it }
                2 -> PasoBienvenida(botId, actual) { b = it }
                3 -> PasoMenu(actual) { b = it }
                4 -> PasoReglas(botId, actual) { b = it }
                5 -> PasoUtilidades(actual) { b = it }
                else -> PasoFinal(
                    botId, actual, estado, resultado, ocupado,
                    alCambiar = { b = it },
                    alCrear = {
                        ocupado = true
                        scope.launch {
                            runCatching { Repo.asistenteGuardar(botId, actual); Repo.asistenteAplicar(botId) }
                                .onSuccess { resultado = it; estado = runCatching { Repo.asistenteEstado(botId) }.getOrNull() ?: estado }
                                .onFailure { avisaToast(ctx, it.message ?: "No se pudo crear tu bot", true) }
                            ocupado = false
                        }
                    },
                    alDeshacer = { confirmaDeshacer = true },
                    alEditarMenu = alEditarMenu, alEditarBienvenida = alEditarBienvenida, alListo = { guardarYSalir() },
                )
            }
        }
    }
    if (confirmaDeshacer) AlertDialog(
        onDismissRequest = { confirmaDeshacer = false }, modifier = Modifier.testTag("asistente_confirma_deshacer"),
        title = { Text("¿Quitar lo que creó el asistente?") },
        text = { Text("Se borran la bienvenida y el menú que armó y los interruptores vuelven a como estaban. Tus reglas y todo lo que hiciste por tu cuenta se quedan.") },
        confirmButton = {
            TextButton({
                confirmaDeshacer = false
                scope.launch {
                    runCatching { Repo.asistenteDeshacer(botId) }.onFailure { avisaToast(ctx, it.message ?: "No se pudo deshacer", true) }
                    resultado = null
                    estado = runCatching { Repo.asistenteEstado(botId) }.getOrNull() ?: estado
                }
            }, Modifier.testTag("asistente_deshacer_si")) { Text("Quitar") }
        },
        dismissButton = { TextButton({ confirmaDeshacer = false }) { Text("Cancelar") } },
    )
}

// ---------- piezas ----------

@Composable
private fun Titulo(t: String, sub: String) {
    Column(verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
        Text(t, style = MaterialTheme.typography.headlineSmall)
        Text(sub, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

@Composable
private fun TarjetaInterruptor(titulo: String, frase: String, activo: Boolean, etiqueta: String, alCambiar: (Boolean) -> Unit, contenido: @Composable ColumnScope.() -> Unit = {}) {
    Card(Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text(titulo, style = MaterialTheme.typography.titleMedium)
                    Text(frase, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                Switch(activo, alCambiar, Modifier.testTag(etiqueta))
            }
            contenido()
        }
    }
}

// ---------- 1) Dónde responde ----------

@Composable
private fun PasoDonde(botId: Long, b: BorradorAsistente, alCambiar: (BorradorAsistente) -> Unit) {
    TarjetaBotBase(botId, alCambiar)
    Titulo("¿Dónde responde tu bot?", "Elige los lugares donde quieres que conteste. Siempre contesta él, nunca escribe primero.")
    TarjetaInterruptor(
        "Chats privados", "Contesta a quien te escribe en privado: bienvenida, menú y respuestas.", b.privados, "asist_privados",
        { alCambiar(b.copy(privados = it)) },
    )
    TarjetaInterruptor(
        "Grupos", "Contesta en los grupos que elijas, solo cuando lo mencionan, citan su mensaje o escriben un comando.", b.grupos, "asist_grupos",
        { alCambiar(b.copy(grupos = it)) },
    ) {
        if (b.grupos) {
            val grupos by produceState<List<GrupoAdmin>?>(null) { value = runCatching { Repo.gruposAdmin(botId) }.getOrDefault(emptyList()) }
            val datos = grupos
            when {
                datos == null -> FilasDeCarga(2)
                datos.isEmpty() -> Text("Todavía no veo grupos. Abre la pestaña Grupos para que se carguen y vuelve.", Modifier.testTag("asist_sin_grupos"), style = MaterialTheme.typography.bodySmall)
                else -> {
                    Text("Marca los grupos donde responde:", style = MaterialTheme.typography.labelLarge)
                    datos.forEach { g ->
                        val marcado = g.jid in b.gruposElegidos
                        Row(Modifier.fillMaxWidth().heightIn(min = 48.dp), verticalAlignment = Alignment.CenterVertically) {
                            Checkbox(marcado, { si -> alCambiar(b.copy(gruposElegidos = if (si) (b.gruposElegidos + g.jid).distinct() else b.gruposElegidos - g.jid)) }, Modifier.testTag("asist_grupo_${g.jid}"))
                            Column(Modifier.weight(1f)) {
                                Text(g.nombre.ifBlank { "Grupo" }, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                Text("${g.miembros} miembros", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                        }
                    }
                    if (b.gruposElegidos.isEmpty()) Text("Elige al menos uno para que responda en grupos.", Modifier.testTag("asist_elige_grupo"), color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
                }
            }
        }
    }
}

// ---------- 2) Bienvenida ----------

@Composable
private fun PasoBienvenida(botId: Long, b: BorradorAsistente, alCambiar: (BorradorAsistente) -> Unit) {
    Titulo("Mensaje de bienvenida", "Es lo primero que lee quien te escribe. Puedes cambiarlo cuando quieras.")
    TarjetaInterruptor("Mandar una bienvenida", "Se manda sola, una vez, cuando alguien te escribe.", b.bienvenidaActiva, "asist_bienvenida_on", { alCambiar(b.copy(bienvenidaActiva = it)) })
    if (!b.bienvenidaActiva) return
    var campo by remember { mutableStateOf(TextFieldValue(b.bienvenidaTexto)) }
    Text("Ejemplo: «¡Hola {nombre}! Gracias por escribirnos. Escribe *menu* para ver las opciones.» {nombre} se cambia por el nombre de la persona.", Modifier.testTag("asist_bienvenida_ejemplo"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    BarraFormato(campo, { campo = it; alCambiar(b.copy(bienvenidaTexto = it.text.take(900))) }, "asist_fmt")
    OutlinedTextField(
        campo, { campo = it; alCambiar(b.copy(bienvenidaTexto = it.text.take(900))) }, Modifier.fillMaxWidth().testTag("asist_bienvenida_texto"),
        label = { Text("Tu mensaje") }, minLines = 3,
    )
    SelectorAdjuntos(botId, b.bienvenidaAdjuntos, "asist", { alCambiar(b.copy(bienvenidaAdjuntos = it)) })
    SelectorAnimo(botId, b.bienvenidaAdjuntos) { alCambiar(b.copy(bienvenidaAdjuntos = it)) }
    Text("¿Cuándo la mando?", style = MaterialTheme.typography.titleSmall)
    FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
        FilterChip(b.bienvenidaCuando == "primera", { alCambiar(b.copy(bienvenidaCuando = "primera")) }, { Text("La primera vez que escribe") }, Modifier.heightIn(min = 48.dp).testTag("asist_cuando_primera"))
        FilterChip(b.bienvenidaCuando == "diaria", { alCambiar(b.copy(bienvenidaCuando = "diaria")) }, { Text("Una vez al día") }, Modifier.heightIn(min = 48.dp).testTag("asist_cuando_diaria"))
    }
    Text("Así la verá la persona:", style = MaterialTheme.typography.labelLarge)
    Column(Modifier.testTag("asist_bienvenida_vista"), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
        BurbujaDeTexto(false, "Hola")
        b.bienvenidaAdjuntos.forEach { a -> BurbujaDeTexto(true, "[" + (when (a.tipo) { "imagen" -> "Foto"; "video" -> "Video"; "audio" -> "Nota de voz"; else -> "Documento" }) + "]") }
        if (b.bienvenidaTexto.isNotBlank()) BurbujaDeTexto(true, b.bienvenidaTexto.replace("{nombre}", "Ana"))
    }
}

// ---------- 3) Menú ----------

@Composable
private fun PasoMenu(b: BorradorAsistente, alCambiar: (BorradorAsistente) -> Unit) {
    Titulo("Tu menú", "La persona escribe «menu» y elige una opción con su número. Estas son tres de ejemplo: cámbialas.")
    TarjetaInterruptor("Armar un menú", "Se abre cuando escriben «menu», «opciones» o .menu.", b.menuActivo, "asist_menu_on", { alCambiar(b.copy(menuActivo = it)) })
    if (!b.menuActivo) return
    OutlinedTextField(b.menuTitulo, { alCambiar(b.copy(menuTitulo = it.take(200))) }, Modifier.fillMaxWidth().testTag("asist_menu_titulo"), label = { Text("Pregunta del menú") }, singleLine = true)
    b.menuOpciones.forEachIndexed { i, o ->
        Card(Modifier.fillMaxWidth().testTag("asist_opcion_$i"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
            Column(Modifier.padding(Esp.m), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text("${i + 1}", Modifier.width(28.dp), style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.primary)
                    OutlinedTextField(o.texto, { t -> alCambiar(b.copy(menuOpciones = b.menuOpciones.mapIndexed { j, x -> if (j == i) x.copy(texto = t.take(60)) else x })) }, Modifier.weight(1f).testTag("asist_opcion_texto_$i"), label = { Text("Opción") }, singleLine = true)
                }
                OutlinedTextField(o.respuesta, { t -> alCambiar(b.copy(menuOpciones = b.menuOpciones.mapIndexed { j, x -> if (j == i) x.copy(respuesta = t.take(900)) else x })) }, Modifier.fillMaxWidth().testTag("asist_opcion_resp_$i"), label = { Text("Qué contesta") }, minLines = 2)
                Row(horizontalArrangement = Arrangement.spacedBy(Esp.s), verticalAlignment = Alignment.CenterVertically) {
                    OutlinedButton({ alCambiar(b.copy(menuOpciones = mueve(b.menuOpciones, i, -1))) }, Modifier.heightIn(min = 48.dp).testTag("asist_opcion_subir_$i"), enabled = i > 0) { Text("▲ Subir") }
                    OutlinedButton({ alCambiar(b.copy(menuOpciones = mueve(b.menuOpciones, i, 1))) }, Modifier.heightIn(min = 48.dp).testTag("asist_opcion_bajar_$i"), enabled = i < b.menuOpciones.size - 1) { Text("▼ Bajar") }
                    Spacer(Modifier.weight(1f))
                    IconButton({ alCambiar(b.copy(menuOpciones = b.menuOpciones.filterIndexed { j, _ -> j != i })) }, Modifier.size(48.dp).testTag("asist_opcion_quitar_$i")) { Icon(Ico.Borrar, "Quitar opción") }
                }
            }
        }
    }
    if (b.menuOpciones.size < 10) {
        Button({ alCambiar(b.copy(menuOpciones = b.menuOpciones + OpcionAsistente("Nueva opción", ""))) }, Modifier.fillMaxWidth().heightIn(min = ALTO_BOTON).testTag("asist_opcion_agregar")) {
            Icon(Ico.Agregar, null); Spacer(Modifier.width(Esp.s)); Text("Agregar una opción")
        }
    }
    Text("¿Quieres submenús o formularios? Termina el asistente y abre tu menú en el editor completo; ahí los agregas.", Modifier.testTag("asist_menu_avanzado"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    Text("Así lo verá la persona:", style = MaterialTheme.typography.labelLarge)
    Column(Modifier.testTag("asist_menu_vista"), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
        BurbujaDeTexto(false, "menu")
        BurbujaDeTexto(true, (b.menuTitulo.ifBlank { "¿En qué te ayudo?" }) + "\n" + b.menuOpciones.mapIndexed { i, o -> "${i + 1}. ${o.texto}" }.joinToString("\n"))
    }
}

private fun <T> mueve(l: List<T>, i: Int, d: Int): List<T> {
    val j = i + d
    if (j !in l.indices) return l
    val m = l.toMutableList(); val x = m[i]; m[i] = m[j]; m[j] = x
    return m
}

// ---------- 4) Reglas ----------

@Composable
private fun PasoReglas(botId: Long, b: BorradorAsistente, alCambiar: (BorradorAsistente) -> Unit) {
    Titulo("Reglas de la casa", "Escribe las reglas una por una. Se guardan al momento y las usa el comando !reglas y los avisos de advertencia.")
    ListaReglasEditable(botId, "")
    Text("Con !reglas, cualquiera del grupo puede verlas cuando quiera. Eso ya funciona solo.", Modifier.testTag("asist_reglas_comando"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    TarjetaInterruptor(
        "Mandarlas a quien entra a un grupo", "Cuando alguien entra, el bot le da la bienvenida en el grupo con estas reglas.", b.reglasAlEntrar, "asist_reglas_entrar",
        { alCambiar(b.copy(reglasAlEntrar = it)) },
    ) {
        if (b.reglasAlEntrar) {
            if (!(b.grupos && b.gruposElegidos.isNotEmpty())) Text("Primero elige al menos un grupo en el paso 1.", Modifier.testTag("asist_reglas_sin_grupo"), style = MaterialTheme.typography.bodySmall)
        }
    }
}

// ---------- 5) Utilidades ----------

@Composable
private fun PasoUtilidades(b: BorradorAsistente, alCambiar: (BorradorAsistente) -> Unit) {
    Titulo("Utilidades", "Interruptores simples. Empiezan como los tienes hoy; cámbialos si quieres.")
    TarjetaInterruptor("Silenciar en grupos", "Borra lo que escribe alguien silenciado (si eres admin).", b.silenciar, "asist_util_silenciar", { alCambiar(b.copy(silenciar = it)) })
    TarjetaInterruptor("Advertencias", "Anota advertencias a las personas y avisa al llegar al tope.", b.advertencias, "asist_util_advertencias", { alCambiar(b.copy(advertencias = it)) })
    TarjetaInterruptor("Etiquetas", "Si no tienes ninguna, crea tres de ejemplo: Cliente, Importante y Pendiente.", b.etiquetas, "asist_util_etiquetas", { alCambiar(b.copy(etiquetas = it)) })
    TarjetaInterruptor("Recordatorios", "Permite crear recordatorios con el comando !recordar.", b.recordatorios, "asist_util_recordatorios", { alCambiar(b.copy(recordatorios = it)) })
    TarjetaInterruptor("Notas y pendientes", "Permite guardar notas y tareas con comandos como !anotar y !tarea.", b.notas, "asist_util_notas", { alCambiar(b.copy(notas = it)) })
}

// ---------- Final ----------

@Composable
private fun PasoFinal(
    botId: Long, b: BorradorAsistente, estado: EstadoAsistente?, resultado: ResultadoAsistente?, ocupado: Boolean,
    alCambiar: (BorradorAsistente) -> Unit, alCrear: () -> Unit, alDeshacer: () -> Unit, alEditarMenu: (Long) -> Unit, alEditarBienvenida: () -> Unit, alListo: () -> Unit,
) {
    val aplicado = resultado != null || estado?.aplicado == true
    Titulo(if (aplicado) "¡Tu bot está listo!" else "Así queda tu bot", if (aplicado) "Todo lo de abajo ya está funcionando. Puedes probarlo o cambiarlo cuando quieras." else "Revisa y toca «Crear mi bot». Nada cambia hasta entonces.")
    Card(Modifier.fillMaxWidth().testTag("asist_resumen"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            val donde = listOfNotNull(if (b.privados) "chats privados" else null, if (b.grupos && b.gruposElegidos.isNotEmpty()) "${b.gruposElegidos.size} grupo(s)" else null).joinToString(" y ").ifBlank { "ningún lugar" }
            Linea("Responde en", donde)
            Linea("Bienvenida", if (b.saltado(2) || !b.bienvenidaActiva) "sin cambios" else (if (b.bienvenidaCuando == "diaria") "una vez al día" else "la primera vez") + (if (b.bienvenidaAdjuntos.isNotEmpty()) " · ${b.bienvenidaAdjuntos.size} archivo(s)" else ""))
            Linea("Menú", if (b.saltado(3) || !b.menuActivo) "sin cambios" else "${b.menuOpciones.size} opciones")
            Linea("Reglas al entrar a un grupo", if (b.saltado(4) || !b.reglasAlEntrar) "no" else "sí (el bot escribe al entrar alguien)")
            Linea("Utilidades", if (b.saltado(5)) "sin cambios" else listOfNotNull(
                if (b.silenciar) "silenciar" else null, if (b.advertencias) "advertencias" else null, if (b.etiquetas) "etiquetas" else null,
                if (b.recordatorios) "recordatorios" else null, if (b.notas) "notas" else null).joinToString(", ").ifBlank { "todas apagadas" })
        }
    }
    val problemas = if (aplicado) emptyList() else estado?.problemas.orEmpty()
    problemas.forEach { Text("• $it", Modifier.testTag("asist_problema"), color = MaterialTheme.colorScheme.error) }
    resultado?.avisos?.forEach { Text(it, Modifier.testTag("asist_aviso_resultado"), style = MaterialTheme.typography.bodySmall) }
    if (!aplicado) {
        Button(alCrear, Modifier.fillMaxWidth().heightIn(min = ALTO_BOTON).testTag("asist_crear"), enabled = problemas.isEmpty() && !ocupado) { Text(if (ocupado) "Creando…" else "Crear mi bot") }
    }
    Prueba(botId, b, estado, alCambiar)
    if (aplicado) {
        val menu = resultado?.menuId ?: 0L
        if (menu > 0) OutlinedButton({ alEditarMenu(menu) }, Modifier.fillMaxWidth().heightIn(min = ALTO_BOTON).testTag("asist_editar_menu")) { Text("Editar el menú completo (submenús y formularios)") }
        OutlinedButton(alEditarBienvenida, Modifier.fillMaxWidth().heightIn(min = ALTO_BOTON).testTag("asist_editar_bienvenida")) { Text("Editar la bienvenida") }
        OutlinedButton(alDeshacer, Modifier.fillMaxWidth().heightIn(min = ALTO_BOTON).testTag("asist_deshacer")) { Text("Deshacer lo que creó el asistente") }
        Button(alListo, Modifier.fillMaxWidth().heightIn(min = ALTO_BOTON).testTag("asist_listo")) { Text("Listo") }
    }
}

@Composable
private fun Linea(t: String, v: String) {
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
        Text(t, Modifier.weight(1f), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(v, Modifier.weight(1f), style = MaterialTheme.typography.bodyMedium)
    }
}

/** «Probar mi bot»: manda la bienvenida a mi otro número. Si no hay otro número, lo explica. */
@Composable
private fun Prueba(botId: Long, b: BorradorAsistente, estado: EstadoAsistente?, alCambiar: (BorradorAsistente) -> Unit) {
    val scope = rememberCoroutineScope()
    val configurado = estado?.numeroPrueba.orEmpty()
    var numero by remember(configurado, b.otroNumero) { mutableStateOf(b.otroNumero.ifBlank { configurado }) }
    var confirmando by remember { mutableStateOf(false) }
    var mensaje by remember { mutableStateOf<Pair<Boolean, String>?>(null) }
    var enviando by remember { mutableStateOf(false) }
    Card(Modifier.fillMaxWidth().testTag("asist_prueba"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerHigh)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
            Text("Probar mi bot", style = MaterialTheme.typography.titleMedium)
            if (numero.isBlank()) {
                Text("Para probar necesitas un segundo número (otro teléfono o WhatsApp). El bot le manda la bienvenida y desde ese número puedes contestar para ver el menú. Si no tienes otro número, sáltate esta prueba.", Modifier.testTag("asist_prueba_explica"), style = MaterialTheme.typography.bodyMedium)
            } else {
                Text("Se manda la bienvenida a tu otro número. Es un mensaje real: sale de tu WhatsApp.", style = MaterialTheme.typography.bodyMedium)
            }
            OutlinedTextField(numero, { numero = it.filter { c -> c.isDigit() || c == '+' || c == ' ' }.take(20) }, Modifier.fillMaxWidth().testTag("asist_prueba_numero"), label = { Text("Otro número (con código de país)") }, singleLine = true)
            Button({ confirmando = true }, Modifier.fillMaxWidth().heightIn(min = ALTO_BOTON).testTag("asist_probar"), enabled = numero.filter { it.isDigit() }.length >= 8 && !enviando) { Text(if (enviando) "Mandando…" else "Mandar la bienvenida") }
            mensaje?.let { (ok, t) -> Text(t, Modifier.testTag("asist_prueba_msg"), color = if (ok) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error) }
        }
    }
    if (confirmando) AlertDialog(
        onDismissRequest = { confirmando = false }, modifier = Modifier.testTag("asist_prueba_confirma"),
        title = { Text("¿Mandar la prueba?") },
        text = { Text("Se enviará un mensaje real al número $numero. Máximo 3 pruebas por hora.") },
        confirmButton = {
            TextButton({
                confirmando = false; enviando = true; mensaje = null
                val limpio = numero.filter { it.isDigit() }
                alCambiar(b.copy(otroNumero = limpio))
                scope.launch {
                    runCatching { Repo.asistenteGuardar(botId, b.copy(otroNumero = limpio)); Repo.asistenteProbar(botId, limpio) }
                        .onSuccess { mensaje = true to "Listo, mandé la bienvenida. Revisa el otro teléfono." }
                        .onFailure { mensaje = false to (it.message ?: "No se pudo mandar la prueba.") }
                    enviando = false
                }
            }, Modifier.testTag("asist_prueba_si")) { Text("Mandar") }
        },
        dismissButton = { TextButton({ confirmando = false }) { Text("Cancelar") } },
    )
}

// ---------- invitación de la primera vez ----------

/** Se ofrece una sola vez en una instalación nueva. */
@Composable
fun AvisoAsistenteBot(alEmpezar: () -> Unit, alDespues: () -> Unit) {
    AlertDialog(
        onDismissRequest = alDespues, modifier = Modifier.testTag("aviso_asistente"),
        title = { Text("¿Armamos tu bot?") },
        text = { Text("En cinco pasos sencillos eliges dónde responde, su bienvenida, un menú, las reglas y algunas utilidades. Puedes saltar lo que no quieras y retomarlo después desde la pestaña Bot.") },
        confirmButton = { TextButton(alEmpezar, Modifier.testTag("aviso_asistente_empezar")) { Text("Empezar") } },
        dismissButton = { TextButton(alDespues, Modifier.testTag("aviso_asistente_despues")) { Text("Ahora no") } },
    )
}


// ---------- Bot base y ánimos del avatar ----------

private const val PREFIJO_AVATAR = "avatar_"

/** Copia la imagen del ánimo a la caché y la guarda como adjunto del bot (cifrada, como cualquier otro archivo). */
private suspend fun subirAnimo(ctx: Context, botId: Long, animo: String): AdjuntoSubido? = runCatching {
    val f = java.io.File(ctx.cacheDir, "$PREFIJO_AVATAR$animo.png")
    withContext(Dispatchers.IO) { ctx.assets.open("animos/$animo.png").use { i -> f.outputStream().use { o -> i.copyTo(o) } } }
    Repo.subirAdjunto(botId, f.path, "imagen", "image/png", f.name)
}.getOrNull()

/** Un toque: deja el «Bot para todos» listo: saludo con el avatar, menú por categorías, juegos y utilidades. Todo se puede cambiar después. */
@Composable
private fun TarjetaBotBase(botId: Long, alCambiar: (BorradorAsistente) -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    var ocupado by remember { mutableStateOf(false) }
    var listo by remember { mutableStateOf(false) }
    Card(Modifier.fillMaxWidth().testTag("asist_bot_base"), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.primaryContainer)) {
        Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(Esp.m)) {
                AvatarAnimo("contento", Modifier.size(64.dp))
                Column(Modifier.weight(1f)) {
                    Text("Bot para todos, listo para usar", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
                    Text("Stickers, descargas, IA, clima, cuentas, juegos, notas y más, en privado y en grupos. Los botones salen solo en chats privados. Después cambias lo que quieras.", style = MaterialTheme.typography.bodySmall)
                }
            }
            Button(
                {
                    ocupado = true
                    scope.launch {
                        val e = runCatching { Repo.asistenteBase(botId) }.getOrNull()
                        if (e != null) {
                            val foto = if (e.borrador.bienvenidaAdjuntos.isEmpty()) subirAnimo(ctx, botId, "feliz") else null
                            alCambiar(if (foto != null) e.borrador.copy(bienvenidaAdjuntos = listOf(foto)) else e.borrador)
                            listo = true
                        }
                        ocupado = false
                    }
                },
                Modifier.fillMaxWidth().heightIn(min = 48.dp).testTag("asist_usar_base"), enabled = !ocupado && !listo,
            ) { Text(if (listo) "Listo: revisa los pasos y cambia lo que quieras" else "Usar el bot para todos") }
        }
    }
}

/** Elige con qué cara sale el avatar en el saludo (feliz, pensando, guiño…). Tocar la que está puesta la quita. */
@Composable
private fun SelectorAnimo(botId: Long, adjuntos: List<AdjuntoSubido>, alCambiar: (List<AdjuntoSubido>) -> Unit) {
    val scope = rememberCoroutineScope()
    val ctx = LocalContext.current
    val actual = adjuntos.firstOrNull { it.nombre.startsWith(PREFIJO_AVATAR) }?.nombre?.removePrefix(PREFIJO_AVATAR)?.removeSuffix(".png")
    Text("Foto del avatar en el saludo", style = MaterialTheme.typography.titleSmall)
    Text("Elige con qué cara saluda tu bot.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
        ANIMOS.forEach { a ->
            val marcado = a == actual
            Column(
                Modifier.width(72.dp).clip(RoundedCornerShape(12.dp)).clickable {
                    scope.launch {
                        adjuntos.filter { it.nombre.startsWith(PREFIJO_AVATAR) }.forEach { runCatching { Repo.borrarAdjunto(botId, it.id) } }
                        val sin = adjuntos.filterNot { it.nombre.startsWith(PREFIJO_AVATAR) }
                        alCambiar(if (marcado) sin else (subirAnimo(ctx, botId, a)?.let { listOf(it) + sin } ?: sin))
                    }
                }.testTag("animo_$a"),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Box(Modifier.size(64.dp).clip(CircleShape).border(if (marcado) 3.dp else 1.dp, if (marcado) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.outlineVariant, CircleShape)) {
                    AvatarAnimo(a, Modifier.fillMaxSize())
                }
                Text(nombreAnimo(a), style = MaterialTheme.typography.labelSmall, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
    }
}
