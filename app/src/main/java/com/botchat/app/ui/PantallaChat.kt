@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import android.widget.Toast
import androidx.activity.compose.BackHandler
import androidx.compose.ui.zIndex
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.layout.imePadding
import androidx.compose.material3.SmallFloatingActionButton
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Slider
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import kotlinx.coroutines.flow.first
import androidx.compose.runtime.snapshotFlow
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import kotlinx.coroutines.launch
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import androidx.media3.common.util.UnstableApi
import com.botchat.app.Paises
import com.botchat.app.datos.Msg
import com.botchat.app.datos.Repo
import com.botchat.app.medios.CacheImagenes
import com.botchat.app.medios.PortadaVideo
import com.botchat.app.medios.TipoArchivo
import com.botchat.app.medios.ReproductorAudio

@OptIn(UnstableApi::class)
@Composable
fun PantallaChat(
    jid: String,
    alVolver: () -> Unit,
    alAbrirVisor: (String) -> Unit,
    alAbrirDocumento: (String) -> Unit,
    alAbrirArchivos: () -> Unit,
    alAbrirInfo: () -> Unit = {},
    irA: String = "",
    alAbrirUnaVez: (String) -> Unit = {},
) {
    val app = LocalContext.current.applicationContext as android.app.Application
    val vm: ChatVm = viewModel(key = "chat-$jid", factory = viewModelFactory { initializer { ChatVm(app, jid) } })
    val ctx = LocalContext.current
    val ajustes = remember { com.botchat.app.datos.Ajustes.get(app) }
    val escribirActivo by ajustes.escribirDesdeApp.collectAsState()
    val enLinea by ajustes.mostrarEnLinea.collectAsState()
    val haptica = rememberHaptica()
    val scope = rememberCoroutineScope()
    val chat = vm.chat
    val msgs = vm.mensajes
    var menu by remember { mutableStateOf(false) }
    var confirmaArchivos by remember { mutableStateOf<com.botchat.app.datos.Liberado?>(null) }
    var confirmaChat by remember { mutableStateOf(false) }
    var verFoto by remember { mutableStateOf(false) }
    var olvidando by remember { mutableStateOf<Msg?>(null) }
    var seleccion by remember { mutableStateOf(setOf<String>()) }
    var borrando by remember { mutableStateOf(false) }
    var reenviando by remember { mutableStateOf(false) }
    var infoDe by remember { mutableStateOf<Msg?>(null) }
    var infoCargada by remember { mutableStateOf<com.botchat.app.datos.InfoMsg?>(null) }
    var texto by remember { mutableStateOf(TextFieldValue("")) }
    val estadoLista = rememberLazyListState()

    // ---- adjuntos y notas de voz ----
    val galeria = rememberLauncherForActivityResult(ActivityResultContracts.OpenMultipleDocuments()) { vm.prepararUris(it) }
    val documentos = rememberLauncherForActivityResult(ActivityResultContracts.OpenMultipleDocuments()) { vm.prepararUris(it) }
    val audios = rememberLauncherForActivityResult(ActivityResultContracts.OpenMultipleDocuments()) { vm.prepararUris(it) }
    var archivoCamara by remember { mutableStateOf<java.io.File?>(null) }
    val camaraFoto = rememberLauncherForActivityResult(ActivityResultContracts.TakePicture()) { ok -> archivoCamara?.let { f -> if (ok) vm.prepararArchivo(f, "image/jpeg") else f.delete() }; archivoCamara = null }
    val camaraVideo = rememberLauncherForActivityResult(ActivityResultContracts.CaptureVideo()) { ok -> archivoCamara?.let { f -> if (ok) vm.prepararArchivo(f, "video/mp4") else f.delete() }; archivoCamara = null }
    val permisoMicrofono = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { ok ->
        if (!ok) Toast.makeText(ctx, "Sin permiso del micrófono no se pueden grabar notas de voz", Toast.LENGTH_LONG).show()
    }
    fun tomaDeCamara(video: Boolean) {
        val f = java.io.File(com.botchat.app.medios.PrepararAdjunto.carpeta(ctx), "cam_${System.nanoTime()}." + if (video) "mp4" else "jpg")
        archivoCamara = f
        val uri = androidx.core.content.FileProvider.getUriForFile(ctx, ctx.packageName + ".compartir", f)
        try { if (video) camaraVideo.launch(uri) else camaraFoto.launch(uri) } catch (_: Exception) { Toast.makeText(ctx, "No hay una app de cámara", Toast.LENGTH_SHORT).show() }
    }

    // Al salir del chat: se vacía la caché de imágenes y se detiene el audio.
    DisposableEffect(jid) {
        onDispose { CacheImagenes.vaciar(); ReproductorAudio.detener() }
    }
    // Con el chat a la vista: presencia y "en línea" (solo si el usuario lo permitió); al salir se apaga.
    val dueno = androidx.lifecycle.compose.LocalLifecycleOwner.current
    DisposableEffect(dueno, enLinea) {
        val obs = androidx.lifecycle.LifecycleEventObserver { _, ev ->
            if (ev == androidx.lifecycle.Lifecycle.Event.ON_RESUME) vm.alMostrar(enLinea)
            if (ev == androidx.lifecycle.Lifecycle.Event.ON_PAUSE) vm.alOcultar(enLinea)
        }
        dueno.lifecycle.addObserver(obs)
        onDispose { dueno.lifecycle.removeObserver(obs); vm.alOcultar(enLinea) }
    }
    LaunchedEffect(vm.aviso) { vm.aviso?.let { Toast.makeText(ctx, it, Toast.LENGTH_LONG).show(); vm.aviso = null } }
    // Borrador del chat: se recupera al abrir y se guarda mientras se escribe.
    var borradorListo by remember(jid) { mutableStateOf(false) }
    LaunchedEffect(vm.borradorInicial) {
        val b = vm.borradorInicial
        if (b != null && !borradorListo) { borradorListo = true; if (texto.text.isEmpty()) texto = TextFieldValue(b, TextRange(b.length)) }
    }
    LaunchedEffect(vm.editando) { vm.editando?.let { texto = TextFieldValue(it.texto, TextRange(it.texto.length)) } }

    val colorEnlace = MaterialTheme.colorScheme.primary
    SideEffect { vm.colorTexto = colorEnlace }
    val items = vm.elementos
    var resaltado by remember { mutableStateOf("") }

    suspend fun irAMensaje(id: String) {
        if (!vm.cargarHasta(id)) { Toast.makeText(ctx, "Ese mensaje ya no está en este teléfono", Toast.LENGTH_SHORT).show(); return }
        kotlinx.coroutines.delay(60)
        val idx = vm.elementos.indexOfFirst { it is Elem.Burbuja && it.m.id == id }
        if (idx >= 0) {
            estadoLista.animateScrollToItem(idx)
            resaltado = id
            kotlinx.coroutines.delay(2200)
            resaltado = ""
        }
    }
    var irHecho by remember(irA) { mutableStateOf(irA.isEmpty()) }
    LaunchedEffect(irA, msgs) {
        if (irHecho || msgs == null) return@LaunchedEffect
        irHecho = true
        irAMensaje(irA)
    }
    // Al abrir con mensajes sin leer, se empieza en el primero.
    var inicioHecho by remember(jid) { mutableStateOf(false) }
    LaunchedEffect(msgs != null) {
        if (msgs == null || inicioHecho) return@LaunchedEffect
        inicioHecho = true
        if (irA.isEmpty()) {
            val idx = items.indexOfFirst { it is Elem.SinLeer }
            if (idx >= 0) estadoLista.scrollToItem(idx)
        }
    }
    val llegandoAlFinal by remember { derivedStateOf { val u = estadoLista.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0; u >= items.size - 6 } }
    LaunchedEffect(llegandoAlFinal, items.size) { if (llegandoAlFinal) vm.cargarMas() }
    val lejosDelFinal by remember { derivedStateOf { estadoLista.firstVisibleItemIndex > 4 } }
    // Avanzado → Medir rendimiento → Abrir chat: esta pantalla reporta cuándo se vio el primer contenido y cuándo estuvo todo.
    if (MedidorChat.mide(jid)) {
        LaunchedEffect(Unit) {
            snapshotFlow { estadoLista.layoutInfo.visibleItemsInfo.size }.first { it > 0 }
            MedidorChat.primerContenido()
        }
        LaunchedEffect(vm.diferidoListo) {
            if (!vm.diferidoListo) return@LaunchedEffect
            snapshotFlow { estadoLista.layoutInfo.visibleItemsInfo.size }.first { it > 0 }
            var calmas = 0
            while (calmas < 3) { kotlinx.coroutines.delay(60); if (MedidorChat.imagenesPendientes() == 0) calmas++ else calmas = 0 }
            MedidorChat.contenidoCompleto()
            kotlinx.coroutines.delay(400)
            alVolver()
        }
    }

    val elegidos = remember(msgs, seleccion) { msgs?.filter { it.id in seleccion } ?: emptyList() }
    val unico = elegidos.singleOrNull()
    fun limpiarSeleccion() { seleccion = emptySet() }
    BackHandler(seleccion.isNotEmpty()) { limpiarSeleccion() }
    fun puedeEditar(m: Msg) = m.esMio && m.tipo == "texto" && !m.borrado && m.estadoEnvio in setOf("enviado", "entregado", "leido") &&
        System.currentTimeMillis() / 1000 - m.ts < vm.plazoEdicionSeg

    Scaffold(
        topBar = {
            if (seleccion.isNotEmpty()) {
                TopAppBar(
                    navigationIcon = { IconButton({ limpiarSeleccion() }, Modifier.testTag("cerrar_seleccion")) { Icon(Ico.Cerrar, "Cancelar selección") } },
                    title = { Text("${seleccion.size}", Modifier.testTag("contador_seleccion")) },
                    actions = {
                        if (unico != null && escribirActivo) IconButton({ vm.citando = unico; limpiarSeleccion() }, Modifier.testTag("accion_responder")) { Icon(Ico.Responder, "Responder") }
                        if (unico != null && escribirActivo && puedeEditar(unico)) IconButton({ vm.editando = unico; limpiarSeleccion() }, Modifier.testTag("accion_editar")) { Icon(Ico.Editar, "Editar") }
                        IconButton({
                            val t = elegidos.sortedBy { it.ts }.filter { it.texto.isNotBlank() && it.tipo != "contacto" && it.tipo != "ubicacion" }.joinToString("\n") { it.texto }
                            ctx.getSystemService(android.content.ClipboardManager::class.java)?.setPrimaryClip(android.content.ClipData.newPlainText("Mensajes", t))
                            Toast.makeText(ctx, "Copiado", Toast.LENGTH_SHORT).show()
                            limpiarSeleccion()
                        }, Modifier.testTag("accion_copiar")) { Icon(Ico.Copiar, "Copiar") }
                        if (escribirActivo) IconButton({ reenviando = true }, Modifier.testTag("accion_reenviar")) { Icon(Ico.Compartir, "Reenviar") }
                        IconButton({ borrando = true }, Modifier.testTag("accion_borrar")) { Icon(Ico.Borrar, "Borrar") }
                        Box {
                            var mas by remember { mutableStateOf(false) }
                            IconButton({ mas = true }, Modifier.testTag("accion_mas")) { Icon(Ico.Mas, "Más") }
                            DropdownMenu(mas, { mas = false }) {
                                if (unico != null && unico.esMio) DropdownMenuItem(text = { Text("Info del mensaje") }, onClick = { mas = false; infoDe = unico; infoCargada = null; scope.launch { infoCargada = vm.info(unico) } }, modifier = Modifier.testTag("accion_info"))
                                if (unico != null) DropdownMenuItem(text = { Text("Olvidar este mensaje") }, onClick = { mas = false; olvidando = unico; limpiarSeleccion() }, modifier = Modifier.testTag("olvidar_mensaje"))
                            }
                        }
                    },
                    colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.secondaryContainer),
                )
            } else TopAppBar(
                navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } },
                title = {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        // La foto abre la foto grande; el nombre y el número abren la Info del chat.
                        Avatar(chat?.nombre ?: "", chat?.foto ?: "", chat?.fotoTs ?: 0, jid, 40.dp, Modifier.clip(CircleShape).clickable { haptica(Toque.Suave); verFoto = true }.compartido("avatar-$jid").testTag("avatar_chat"))
                        Spacer(Modifier.width(Esp.m))
                        Column(Modifier.clip(RoundedCornerShape(12.dp)).clickable { haptica(Toque.Suave); alAbrirInfo() }.padding(vertical = 4.dp, horizontal = 4.dp).testTag("encabezado_info")) {
                            Text(chat?.nombre ?: "", Modifier.compartido("nombre-$jid", limites = true), style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                            val vivo = textoPresencia(vm.presencia)
                            if (vivo.isNotEmpty()) {
                                Text(vivo, Modifier.testTag("presencia"), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.primary, maxLines = 1)
                            } else if (chat != null && !chat.esGrupo) {
                                TextoUnaLinea(banderaTelefono(chat), estilo = MaterialTheme.typography.bodySmall)
                            } else if (chat != null) {
                                TextoUnaLinea("Grupo", estilo = MaterialTheme.typography.bodySmall)
                            }
                        }
                    }
                },
                actions = {
                    IconButton(alAbrirArchivos) { Icon(Ico.Carpeta, "Archivos compartidos") }
                    Box {
                        IconButton({ menu = true }) { Icon(Ico.Mas, "Más opciones") }
                        DropdownMenu(menu, { menu = false }) {
                            if (chat?.esGrupo == true) DropdownMenuItem(
                                text = { Text(if (vm.estadoChat.botActivo) "Bot en este grupo: Activo (tocar para pausar)" else "Bot en este grupo: Pausado (tocar para activar)") },
                                onClick = { menu = false; vm.botActivoEnChat(!vm.estadoChat.botActivo) }, modifier = Modifier.testTag("menu_bot_grupo"),
                            )
                            DropdownMenuItem(text = { Text(if (vm.estadoChat.silenciado) "Activar notificaciones" else "Silenciar notificaciones") }, onClick = { menu = false; vm.silenciar(!vm.estadoChat.silenciado) }, modifier = Modifier.testTag("menu_silenciar"))
                            DropdownMenuItem(text = { Text("Borrar archivos de este chat") }, onClick = {
                                menu = false
                                haptica(Toque.Fuerte)
                                vm.estimarArchivos { confirmaArchivos = it }
                            })
                            DropdownMenuItem(text = { Text("Borrar todo el chat") }, onClick = { menu = false; haptica(Toque.Fuerte); confirmaChat = true })
                        }
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
            )
        },
        bottomBar = {
            Column {
                if (unico != null && escribirActivo) {
                    Box(Modifier.fillMaxWidth().padding(bottom = 6.dp), Alignment.Center) {
                        BarraReacciones(alElegir = { e -> haptica(Toque.Suave); vm.reaccionar(unico, e); limpiarSeleccion() })
                    }
                }
                if (escribirActivo) {
                    Compositor(
                        valor = texto,
                        alCambiar = { v -> val cambio = v.text != texto.text; texto = v; if (cambio && vm.editando == null) { vm.guardarBorrador(v.text); vm.teclea(v.text.isNotBlank()) } },
                        citando = vm.citando, alQuitarCita = { vm.citando = null },
                        editando = vm.editando, alCancelarEdicion = { vm.editando = null; texto = TextFieldValue("") },
                        esGrupo = chat?.esGrupo == true, participantes = vm.participantes, enviando = vm.enviando,
                        alEnviar = { t, men ->
                            vm.enviar(t, men) { texto = TextFieldValue(""); scope.launch { estadoLista.animateScrollToItem(0) } }
                        },
                        adjuntos = vm.adjuntos, preparandoAdjuntos = vm.preparandoAdjuntos, enviandoAdjuntos = vm.enviandoAdjuntos,
                        alAdjuntar = { f ->
                            when (f) {
                                FuenteAdjunto.GALERIA -> galeria.launch(arrayOf("image/*", "video/*"))
                                FuenteAdjunto.DOCUMENTO -> documentos.launch(arrayOf("*/*"))
                                FuenteAdjunto.AUDIO -> audios.launch(arrayOf("audio/*"))
                                FuenteAdjunto.CAMARA_FOTO -> tomaDeCamara(false)
                                FuenteAdjunto.CAMARA_VIDEO -> tomaDeCamara(true)
                            }
                        },
                        alQuitarAdjunto = { vm.quitarAdjunto(it) }, alCancelarAdjuntos = { vm.cancelarAdjuntos() },
                        alEnviarAdjuntos = { pie -> vm.enviarAdjuntos(pie) { texto = TextFieldValue(""); scope.launch { estadoLista.animateScrollToItem(0) } } },
                        grabando = vm.grabando, segundosGrabando = vm.segundosGrabando,
                        alIniciarGrabacion = {
                            if (androidx.core.content.ContextCompat.checkSelfPermission(ctx, android.Manifest.permission.RECORD_AUDIO) != android.content.pm.PackageManager.PERMISSION_GRANTED) {
                                permisoMicrofono.launch(android.Manifest.permission.RECORD_AUDIO); false
                            } else vm.iniciarGrabacion()
                        },
                        alTerminarGrabacion = { cancelado -> vm.terminarGrabacion(cancelado); if (!cancelado) scope.launch { estadoLista.animateScrollToItem(0) } },
                    )
                }
            }
        },
        containerColor = MaterialTheme.colorScheme.surface,
    ) { relleno ->
        Box(Modifier.fillMaxSize().padding(relleno)) {
            val pausaHasta = vm.estadoChat.botPausaHasta
            val ahoraS = System.currentTimeMillis() / 1000
            if (pausaHasta > ahoraS) {
                Surface(Modifier.align(Alignment.TopCenter).fillMaxWidth().zIndex(2f).testTag("aviso_bot_pausado"), color = MaterialTheme.colorScheme.tertiaryContainer) {
                    Row(Modifier.padding(horizontal = Esp.l, vertical = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                        Text(
                            if (vm.estadoChat.pausaHumano) "Esperando a una persona: el bot calla aquí hasta las ${horaDe(pausaHasta)}" else "El bot está en pausa en este chat hasta las ${horaDe(pausaHasta)}",
                            Modifier.weight(1f), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onTertiaryContainer,
                        )
                        TextButton({ haptica(Toque.Suave); vm.reactivarBot() }, Modifier.testTag("reactivar_bot")) { Text("Reactivar") }
                    }
                }
            }
            val etiquetas = vm.etiquetasMenu
            if (etiquetas.isNotEmpty() && pausaHasta <= ahoraS) {
                Surface(Modifier.align(Alignment.TopCenter).fillMaxWidth().zIndex(1f).testTag("etiqueta_menu"), color = MaterialTheme.colorScheme.secondaryContainer) {
                    Column(Modifier.padding(horizontal = Esp.l, vertical = 4.dp)) {
                        etiquetas.take(3).forEach { e ->
                            Text(
                                (if (e.estado == "activo") "En menú: " else "Con una persona: ") + e.ruta.replace(" › ", " › ") + " · " + haceCuanto(e.ultima) + if (chat?.esGrupo == true && e.nombre.isNotBlank()) " (${e.nombre})" else "",
                                style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSecondaryContainer, maxLines = 1, overflow = TextOverflow.Ellipsis,
                            )
                        }
                    }
                }
            }
            when {
                msgs == null -> CargaDeMensajes()
                msgs.isEmpty() -> EstadoVacio(Ico.Documento, "Sin mensajes", "Todavía no hay mensajes en este chat.", Modifier.align(Alignment.Center))
                else -> {
                    LazyColumn(
                        Modifier.fillMaxSize().testTag("lista_chat"), state = estadoLista, reverseLayout = true,
                        contentPadding = PaddingValues(horizontal = Esp.m, vertical = Esp.s), verticalArrangement = Arrangement.spacedBy(2.dp),
                    ) {
                        items(
                            items,
                            key = { e -> when (e) { is Elem.Dia -> "dia-${e.ts}"; is Elem.SinLeer -> "sinleer"; is Elem.Burbuja -> "m-${e.m.id}" } },
                            contentType = { e -> when (e) { is Elem.Dia -> "dia"; is Elem.SinLeer -> "sinleer"; is Elem.Burbuja -> "b-${e.m.categoria}" } },
                        ) { e ->
                            // Sin desvanecido al entrar: las burbujas que ya estaban no se animan (solo lo que se mueve de lugar).
                            val anima = Modifier.animateItem(fadeInSpec = null)
                            when (e) {
                                is Elem.Dia -> SeparadorDia(e.ts, anima)
                                is Elem.SinLeer -> SeparadorSinLeer(e.cuantos, anima)
                                is Elem.Burbuja -> Burbuja(
                                    e, vm, alAbrirVisor, alAbrirDocumento, anima.apareceDesdeAbajo(vm.esNueva(e.m.id)), alAbrirUnaVez,
                                    resaltar = resaltado == e.m.id, seleccionado = e.m.id in seleccion, enSeleccion = seleccion.isNotEmpty(),
                                    alAlternar = { seleccion = if (e.m.id in seleccion) seleccion - e.m.id else seleccion + e.m.id },
                                    alResponder = { if (escribirActivo) vm.citando = e.m },
                                    alIrACita = { id -> scope.launch { irAMensaje(id) } },
                                )
                            }
                        }
                        if (vm.cargandoMas) item { Box(Modifier.fillMaxWidth().padding(Esp.l), contentAlignment = Alignment.Center) { CircularProgressIndicator(Modifier.size(24.dp), strokeWidth = 2.dp) } }
                    }
                    // Separador de día fijo arriba mientras se desplaza.
                    val diaArriba by remember { derivedStateOf { estadoLista.layoutInfo.visibleItemsInfo.lastOrNull()?.index?.let { i -> items.getOrNull(i)?.let { e -> if (e is Elem.Dia) e.ts else (e as? Elem.Burbuja)?.m?.ts } } } }
                    AnimatedVisibility(
                        visible = estadoLista.isScrollInProgress && diaArriba != null, modifier = Modifier.align(Alignment.TopCenter).padding(top = Esp.s),
                        enter = fadeIn() + slideInVertically { -it / 2 }, exit = fadeOut(),
                    ) {
                        Surface(shape = RoundedCornerShape(50), color = MaterialTheme.colorScheme.secondaryContainer.copy(alpha = 0.95f), shadowElevation = 2.dp) {
                            Text(etiquetaDia(diaArriba ?: 0), Modifier.padding(horizontal = 14.dp, vertical = 6.dp), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSecondaryContainer)
                        }
                    }
                    AnimatedVisibility(lejosDelFinal, Modifier.align(Alignment.BottomEnd).padding(Esp.l), enter = fadeIn(), exit = fadeOut()) {
                        SmallFloatingActionButton({ haptica(Toque.Suave); scope.launch { estadoLista.animateScrollToItem(0) } }, Modifier.testTag("bajar_al_final")) { Icon(Ico.Abajo, "Ir al último mensaje") }
                    }
                }
            }
        }
    }

    if (borrando && elegidos.isNotEmpty()) {
        val propios = elegidos.filter { it.esMio || chat?.esGrupo == true }
        DialogoBorrar(
            cuantos = elegidos.size, hayParaTodos = escribirActivo && propios.isNotEmpty(),
            alParaMi = { borrando = false; vm.borrarParaMi(elegidos.map { it.id }); limpiarSeleccion() },
            alParaTodos = { borrando = false; vm.borrarParaTodos(propios); limpiarSeleccion() },
            alCerrar = { borrando = false },
        )
    }
    if (reenviando && elegidos.isNotEmpty()) {
        DialogoReenviar(vm.botId, jid, alReenviar = { destinos ->
            reenviando = false
            vm.reenviar(elegidos.map { it.id }, destinos) { Toast.makeText(ctx, it, Toast.LENGTH_LONG).show() }
            limpiarSeleccion()
        }, alCerrar = { reenviando = false })
    }
    if (infoDe != null) DialogoInfoMensaje(infoCargada, alCerrar = { infoDe = null; limpiarSeleccion() })

    confirmaArchivos?.let { lib ->
        AlertDialog(
            onDismissRequest = { confirmaArchivos = null },
            title = { Text("¿Borrar los archivos de este chat?") },
            text = { Text(if (lib.cantidad == 0) "Este chat no tiene archivos descargados." else "Se liberarán ${formatoBytes(lib.bytes)} (${lib.cantidad} archivos). Los mensajes de texto se conservan.") },
            confirmButton = {
                TextButton(enabled = lib.cantidad > 0, onClick = {
                    confirmaArchivos = null
                    vm.borrarArchivos { haptica(Toque.Confirmar); Toast.makeText(ctx, "Se liberaron ${formatoBytes(it.bytes)}", Toast.LENGTH_SHORT).show() }
                }) { Text("Borrar archivos", color = MaterialTheme.colorScheme.error) }
            },
            dismissButton = { TextButton({ confirmaArchivos = null }) { Text("Cancelar") } },
        )
    }
    olvidando?.let { m ->
        AlertDialog(
            onDismissRequest = { olvidando = null },
            title = { Text("¿Olvidar este mensaje?") },
            text = { Text("Se borran para siempre su contenido y su archivo de este teléfono. No se borra nada en WhatsApp.") },
            confirmButton = { TextButton({ olvidando = null; haptica(Toque.Confirmar); vm.olvidar(m) }, Modifier.testTag("confirmar_olvidar")) { Text("Olvidar", color = MaterialTheme.colorScheme.error) } },
            dismissButton = { TextButton({ olvidando = null }) { Text("Cancelar") } },
        )
    }
    if (confirmaChat) {
        AlertDialog(
            onDismissRequest = { confirmaChat = false },
            title = { Text("¿Borrar todo el chat?") },
            text = { Text("Se borrarán los mensajes, los archivos y la foto de este chat de tu teléfono. No se borra nada en WhatsApp.") },
            confirmButton = {
                TextButton({ confirmaChat = false; haptica(Toque.Confirmar); vm.borrarChat { alVolver() } }) { Text("Borrar todo", color = MaterialTheme.colorScheme.error) }
            },
            dismissButton = { TextButton({ confirmaChat = false }) { Text("Cancelar") } },
        )
    }
    if (verFoto) FotoGrandeDialogo(vm.botId, jid, chat?.nombre ?: "", { verFoto = false })
}

@Composable
private fun SeparadorSinLeer(cuantos: Int, modifier: Modifier) {
    Box(modifier.fillMaxWidth().padding(vertical = Esp.s), contentAlignment = Alignment.Center) {
        Surface(shape = RoundedCornerShape(50), color = MaterialTheme.colorScheme.primaryContainer) {
            Text(if (cuantos == 1) "1 mensaje sin leer" else "$cuantos mensajes sin leer", Modifier.padding(horizontal = 12.dp, vertical = 4.dp).testTag("sin_leer"), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onPrimaryContainer)
        }
    }
}

@Composable
private fun CargaDeMensajes() {
    Column(Modifier.fillMaxSize().padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
        listOf(0.55f to false, 0.4f to true, 0.7f to false, 0.3f to true, 0.5f to false).forEach { (ancho, mio) ->
            Row(Modifier.fillMaxWidth(), horizontalArrangement = if (mio) Arrangement.End else Arrangement.Start) {
                Box(Modifier.fillMaxWidth(ancho).height(46.dp).clip(RoundedCornerShape(18.dp)).shimmer())
            }
        }
    }
}

@Composable
private fun SeparadorDia(ts: Long, modifier: Modifier) {
    Box(modifier.fillMaxWidth().padding(vertical = Esp.s), contentAlignment = Alignment.Center) {
        Surface(shape = RoundedCornerShape(50), color = MaterialTheme.colorScheme.surfaceContainerHigh) {
            Text(etiquetaDia(ts), Modifier.padding(horizontal = 12.dp, vertical = 4.dp), style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

// ---------- burbujas ----------


@OptIn(UnstableApi::class, androidx.compose.foundation.ExperimentalFoundationApi::class)
@Composable
private fun Burbuja(
    e: Elem.Burbuja, vm: ChatVm, alAbrirVisor: (String) -> Unit, alAbrirDocumento: (String) -> Unit, modifier: Modifier, alAbrirUnaVez: (String) -> Unit,
    resaltar: Boolean = false, seleccionado: Boolean = false, enSeleccion: Boolean = false,
    alAlternar: () -> Unit = {}, alResponder: () -> Unit = {}, alIrACita: (String) -> Unit = {},
) {
    val m = e.m
    val esquema = MaterialTheme.colorScheme
    val fondo = if (resaltar) esquema.tertiaryContainer else if (m.esMio) esquema.primaryContainer else esquema.surfaceContainerHigh
    val tinta = if (resaltar) esquema.onTertiaryContainer else if (m.esMio) esquema.onPrimaryContainer else esquema.onSurface
    val g = 18.dp
    val p = 5.dp
    val forma = if (m.esMio) RoundedCornerShape(g, g, if (e.ultimoDelGrupo) p else g, g) else RoundedCornerShape(g, g, g, if (e.ultimoDelGrupo) p else g)
    val haptica = rememberHaptica()
    // Deslizar a la derecha = responder.
    var desplazado by remember { mutableStateOf(0f) }
    val umbral = with(androidx.compose.ui.platform.LocalDensity.current) { 56.dp.toPx() }
    val nav = androidx.compose.animation.core.animateFloatAsState(desplazado, label = "desliza").value
    Box(modifier.fillMaxWidth().then(if (seleccionado) Modifier.background(esquema.primary.copy(alpha = 0.18f)) else Modifier)) {
        if (nav > 8f) Icon(Ico.Responder, null, Modifier.align(Alignment.CenterStart).padding(start = 8.dp).size(22.dp), tint = esquema.primary.copy(alpha = (nav / umbral).coerceIn(0f, 1f)))
        Row(
            Modifier.fillMaxWidth().graphicsLayer { translationX = nav }
                .pointerInput(enSeleccion) {
                    if (!enSeleccion) detectHorizontalDragGestures(
                        onDragEnd = { if (desplazado >= umbral) { haptica(Toque.Suave); alResponder() }; desplazado = 0f },
                        onDragCancel = { desplazado = 0f },
                    ) { _, dx -> desplazado = (desplazado + dx).coerceIn(0f, umbral * 1.4f) }
                },
            horizontalArrangement = if (m.esMio) Arrangement.End else Arrangement.Start,
        ) {
            Column(horizontalAlignment = if (m.esMio) Alignment.End else Alignment.Start) {
                Surface(
                    color = fondo, contentColor = tinta, shape = forma,
                    modifier = Modifier.widthIn(max = 300.dp).clip(forma).combinedClickable(onClick = { if (enSeleccion) alAlternar() }, onLongClick = { haptica(Toque.Fuerte); alAlternar() }).testTag("burbuja_${m.id}"),
                ) {
                    Column(Modifier.padding(horizontal = 10.dp, vertical = 7.dp)) {
                        if (e.mostrarRemitente && m.remitente.isNotEmpty()) {
                            Text(m.remitente, style = MaterialTheme.typography.labelLarge, fontWeight = FontWeight.SemiBold, color = colorEstable(m.remitenteId, androidx.compose.foundation.isSystemInDarkTheme()).let { c -> androidx.compose.ui.graphics.lerp(c, tinta, 0.35f) }, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        }
                        if (m.reenviado) Text("Reenviado", Modifier.testTag("reenviado"), style = MaterialTheme.typography.labelSmall, fontStyle = FontStyle.Italic, color = tinta.copy(alpha = 0.65f))
                        if (m.citaId.isNotEmpty() && !m.borrado) BloqueCita(m, tinta) { alIrACita(m.citaId) }
                        ContenidoMensaje(m, vm, alAbrirVisor, alAbrirDocumento, alAbrirUnaVez)
                        FilaReacciones(m.reacciones, Modifier.padding(top = 3.dp))
                        Row(Modifier.align(Alignment.End).padding(top = 2.dp), verticalAlignment = Alignment.CenterVertically) {
                            if (m.editado && !m.borrado) Text("(editado)  ", style = MaterialTheme.typography.labelSmall, color = tinta.copy(alpha = 0.65f))
                            Text(horaDe(m.ts), style = MaterialTheme.typography.labelSmall, color = tinta.copy(alpha = 0.65f))
                            if (m.esMio && m.estadoEnvio.isNotEmpty()) { Spacer(Modifier.width(4.dp)); Palomitas(m.estadoEnvio, tinta) }
                        }
                    }
                }
                if (m.esMio && m.estadoEnvio == "fallido") {
                    TextButton({ haptica(Toque.Suave); vm.reintentar(m) }, Modifier.testTag("reintentar_${m.id}")) { Text("No se envió · Reintentar", color = esquema.error) }
                }
            }
        }
    }
}

private fun etiquetaBorrado(tipo: String) = when (tipo) {
    "imagen" -> "[foto borrada]"
    "sticker" -> "[sticker borrado]"
    "video" -> "[video borrado]"
    "audio" -> "[audio borrado]"
    "documento" -> "[documento borrado]"
    else -> "[archivo borrado]"
}

/** Un mensaje eliminado por su autor: se muestra el original con una etiqueta discreta; si no queda nada, se dice así. */
@OptIn(UnstableApi::class)
@Composable
private fun ContenidoMensaje(m: Msg, vm: ChatVm, alAbrirVisor: (String) -> Unit, alAbrirDocumento: (String) -> Unit, alAbrirUnaVez: (String) -> Unit) {
    if (!m.borrado) { ContenidoBase(m, vm, alAbrirVisor, alAbrirDocumento, alAbrirUnaVez); return }
    val cursiva = FontStyle.Italic
    val sinArchivo = m.categoria.isNotEmpty() && !m.descargado
    val vacio = m.texto.isBlank() && (m.categoria.isEmpty() && m.tipo != "ver_una_vez" || m.estado == "borrado")
    when {
        vacio -> {
            Text("Eliminado · contenido no disponible", Modifier.testTag("eliminado_no_disponible"), fontStyle = cursiva, color = LocalContentColorSuave())
            if (m.eliminadoEn > 0 || m.eliminadoPor.isNotEmpty()) EtiquetaEliminado(m, conPalabra = false)
        }
        sinArchivo -> {
            TarjetaSinDescargar(m)
            if (m.texto.isNotBlank()) Text(m.texto)
            EtiquetaEliminado(m)
        }
        else -> { ContenidoBase(m, vm, alAbrirVisor, alAbrirDocumento, alAbrirUnaVez); EtiquetaEliminado(m) }
    }
}

@Composable
private fun EtiquetaEliminado(m: Msg, conPalabra: Boolean = true) {
    val partes = buildList {
        if (conPalabra) add("Eliminado")
        if (m.eliminadoEn > 0) add(horaDe(m.eliminadoEn))
        if (m.eliminadoPor.isNotEmpty()) add("por ${m.eliminadoPor}")
    }
    Text(partes.joinToString(" · "), Modifier.testTag("etiqueta_eliminado"), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.error.copy(alpha = 0.85f))
}

@Composable
private fun TarjetaSinDescargar(m: Msg) {
    Row(Modifier.widthIn(min = 220.dp).testTag("eliminado_sin_descargar"), verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.size(44.dp).clip(RoundedCornerShape(10.dp)).background(MaterialTheme.colorScheme.surface.copy(alpha = 0.55f)), Alignment.Center) {
            Icon(iconoDeTipo(m.tipo) ?: Ico.Documento, null, Modifier.size(24.dp))
        }
        Spacer(Modifier.width(10.dp))
        Column {
            Text(descripcionCorta(m.tipo, "", false, m.notaVoz) + if (m.bytes > 0) " · " + formatoBytes(m.bytes) else "", fontWeight = FontWeight.Medium)
            Text("Eliminado antes de descargarse", style = MaterialTheme.typography.bodySmall, color = LocalContentColorSuave())
        }
    }
}

@OptIn(UnstableApi::class)
@Composable
private fun ContenidoBase(m: Msg, vm: ChatVm, alAbrirVisor: (String) -> Unit, alAbrirDocumento: (String) -> Unit, alAbrirUnaVez: (String) -> Unit) {
    val cursiva = FontStyle.Italic
    when {
        m.tipo == "ver_una_vez" -> TarjetaVerUnaVez(m, alAbrirUnaVez)
        m.categoria.isNotEmpty() && m.estado == "borrado" -> {
            Text(etiquetaBorrado(m.tipo), fontStyle = cursiva, color = LocalContentColorSuave())
            if (m.texto.isNotBlank()) Text(m.texto)
        }
        m.tipo == "sticker" && m.descargado -> StickerAnimado(m)
        m.categoria == "foto" || m.categoria == "video" -> {
            MiniaturaMedia(m, vm, alAbrirVisor)
            if (m.texto.isNotBlank()) { Spacer(Modifier.size(4.dp)); TextoMensaje(m, grande = false) }
        }
        m.categoria == "audio" -> { BurbujaAudio(m, vm); if (m.texto.isNotBlank()) TextoMensaje(m, grande = false) }
        m.categoria == "documento" -> { BurbujaDocumento(m, vm, alAbrirDocumento); if (m.texto.isNotBlank()) { Spacer(Modifier.size(4.dp)); TextoMensaje(m, grande = false) } }
        m.tipo == "contacto" -> TarjetaContacto(m)
        m.tipo == "ubicacion" -> TarjetaUbicacion(m)
        m.tipo == "encuesta" -> Text(m.texto)
        m.tipo == "otro" -> TarjetaEspecial()
        else -> TextoMensaje(m)
    }
}

@Composable
private fun LocalContentColorSuave() = androidx.compose.material3.LocalContentColor.current.copy(alpha = 0.7f)

@Composable
private fun MiniaturaMedia(m: Msg, vm: ChatVm, alAbrirVisor: (String) -> Unit) {
    val ctx = LocalContext.current
    val densidad = androidx.compose.ui.platform.LocalDensity.current
    val relacion = if (m.ancho > 0 && m.alto > 0) (m.ancho.toFloat() / m.alto).coerceIn(0.6f, 1.8f) else 1.33f
    var miniatura by remember(m.miniatura) { mutableStateOf<androidx.compose.ui.graphics.ImageBitmap?>(null) }
    var imagen by remember(m.ruta, m.descargado) { mutableStateOf<androidx.compose.ui.graphics.ImageBitmap?>(null) }
    val px = with(densidad) { 280.dp.roundToPx() }.coerceAtMost(720)
    // Primero la miniatura (llega al instante), después la imagen reducida; todo se decodifica fuera del hilo principal.
    LaunchedEffect(m.miniatura) {
        MedidorChat.empieza()
        try { miniatura = if (m.miniatura.isEmpty()) null else CacheImagenes.cargar(ctx, Repo.archivo(m.miniatura), 320)?.asImageBitmap() } finally { MedidorChat.termina() }
    }
    // La imagen más nítida se prepara solo si la burbuja sigue a la vista un instante (en un desplazamiento rápido se cancela)
    // y a lo que cabe en la burbuja, no a tamaño completo; la foto entera se abre en el visor.
    LaunchedEffect(m.ruta, m.descargado) {
        if (!m.descargado) { imagen = null; return@LaunchedEffect }
        kotlinx.coroutines.delay(if (m.miniatura.isNotEmpty()) 140L else 0L)
        MedidorChat.empieza()
        try {
            imagen = when {
                // Videos: un cuadro real del video (la miniatura de WhatsApp es diminuta y se ve borrosa al ampliarla).
                m.categoria == "video" -> PortadaVideo.cargar(ctx, Repo.archivo(m.ruta), px, m.dur * 1000L)?.asImageBitmap()
                else -> CacheImagenes.cargar(ctx, Repo.archivo(m.ruta), px)?.asImageBitmap()
            }
        } finally { MedidorChat.termina() }
    }
    val ruta = if (m.descargado) m.ruta else m.miniatura
    val haptica = rememberHaptica()
    Box(
        Modifier.widthIn(max = 280.dp).aspectRatio(relacion).clip(RoundedCornerShape(12.dp)).background(MaterialTheme.colorScheme.surfaceContainerHighest)
            .clickable(enabled = m.descargado) { alAbrirVisor(m.id) },
        contentAlignment = Alignment.Center,
    ) {
        val img = imagen ?: miniatura
        if (img != null) androidx.compose.animation.Crossfade(img, Modifier.fillMaxSize(), animationSpec = Mov.tiempo(LocalReducirMovimiento.current, Mov.NORMAL), label = "foto") { Image(it, null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop) }
        else if (m.estado == "pendiente" || ruta.isNotEmpty()) Box(Modifier.fillMaxSize().shimmer())
        if (m.categoria == "video" && m.descargado) {
            Box(Modifier.size(48.dp).clip(CircleShape).background(Color.Black.copy(alpha = 0.45f)), contentAlignment = Alignment.Center) { Icon(Ico.Reproducir, null, tint = Color.White, modifier = Modifier.size(30.dp)) }
        }
        if (m.categoria == "video" && m.dur > 0) {
            Text(duracion(m.dur), Modifier.align(Alignment.BottomStart).padding(6.dp).clip(RoundedCornerShape(6.dp)).background(Color.Black.copy(alpha = 0.5f)).padding(horizontal = 6.dp, vertical = 2.dp), color = Color.White, style = MaterialTheme.typography.labelSmall)
        }
        if (!m.descargado) EstadoDeDescarga(m, onDescargar = { haptica(Toque.Fuerte); vm.descargar(m) })
    }
}


/** Superpuesto del botón "Descargar", el círculo de progreso o el aviso de error. */
@Composable
private fun EstadoDeDescarga(m: Msg, onDescargar: () -> Unit) {
    Box(Modifier.fillMaxSize().background(Color.Black.copy(alpha = 0.28f)), contentAlignment = Alignment.Center) {
        when {
            m.estado == "pendiente" -> Column(horizontalAlignment = Alignment.CenterHorizontally) {
                CircularProgressIndicator(Modifier.size(34.dp), color = Color.White, strokeWidth = 3.dp)
                Spacer(Modifier.size(6.dp)); Text("Descargando…", color = Color.White, style = MaterialTheme.typography.labelMedium)
            }
            m.puedeBajarse -> FilledTonalButton(onDescargar, contentPadding = PaddingValues(horizontal = 14.dp, vertical = 6.dp)) {
                Icon(Ico.Descargar, null, Modifier.size(18.dp)); Spacer(Modifier.width(6.dp))
                Text(if (m.estado == "fallido") "Reintentar" else if (m.bytes > 0) "Descargar · ${formatoBytes(m.bytes)}" else "Descargar")
            }
            else -> Text("No disponible", color = Color.White, style = MaterialTheme.typography.labelMedium)
        }
    }
}

@OptIn(UnstableApi::class)
@Composable
private fun BurbujaAudio(m: Msg, vm: ChatVm) {
    val ctx = LocalContext.current
    val rep by ReproductorAudio.estado.collectAsState()
    val activo = rep.clave == "${m.chat}|${m.id}"
    val haptica = rememberHaptica()
    Row(Modifier.widthIn(min = 220.dp), verticalAlignment = Alignment.CenterVertically) {
        if (!m.descargado) {
            if (m.estado == "pendiente") CircularProgressIndicator(Modifier.size(32.dp), strokeWidth = 3.dp)
            else if (m.puedeBajarse) FilledTonalButton({ haptica(Toque.Fuerte); vm.descargar(m) }, contentPadding = PaddingValues(horizontal = 12.dp)) { Icon(Ico.Descargar, null, Modifier.size(18.dp)); Spacer(Modifier.width(6.dp)); Text(if (m.estado == "fallido") "Reintentar" else "Descargar") }
            Spacer(Modifier.width(8.dp))
            Icon(Ico.Audio, null, Modifier.size(18.dp)); Spacer(Modifier.width(4.dp))
            Text(if (m.dur > 0) duracion(m.dur) else formatoBytes(m.bytes), style = MaterialTheme.typography.labelMedium)
        } else {
            IconButton({ haptica(Toque.Suave); ReproductorAudio.alternar(ctx, "${m.chat}|${m.id}", Repo.archivo(m.ruta)) }, Modifier.size(40.dp)) {
                Icon(if (activo && rep.reproduciendo) Ico.Pausa else Ico.Reproducir, if (activo && rep.reproduciendo) "Pausar" else "Reproducir")
            }
            val total = if (activo && rep.durMs > 0) rep.durMs else m.dur * 1000L
            val fraccion = if (activo && total > 0) (rep.posMs.toFloat() / total).coerceIn(0f, 1f) else 0f
            Slider(value = fraccion, onValueChange = { if (activo) ReproductorAudio.moverA(it) }, enabled = activo, modifier = Modifier.weight(1f))
            Text(duracion(((if (activo) rep.posMs else 0L) / 1000).toInt().takeIf { activo } ?: m.dur), style = MaterialTheme.typography.labelMedium)
        }
    }
}

@Composable
private fun BurbujaDocumento(m: Msg, vm: ChatVm, alAbrirDocumento: (String) -> Unit) {
    val haptica = rememberHaptica()
    val ctx = LocalContext.current
    val scope = androidx.compose.runtime.rememberCoroutineScope()
    val nombre = TipoArchivo.nombreMostrable(m.nombre, m.mime)
    val familia = TipoArchivo.familia(m.nombre, m.mime)
    val esPdf = familia == TipoArchivo.Familia.PDF
    val abrirFuera = { scope.launch { compartirTemporal(ctx, m, nombre, ver = true) }; Unit }
    Column(
        Modifier.widthIn(min = 240.dp, max = 300.dp).clip(RoundedCornerShape(12.dp)).background(MaterialTheme.colorScheme.surface.copy(alpha = 0.55f))
            .clickable(enabled = m.descargado) { haptica(Toque.Suave); if (esPdf) alAbrirDocumento(m.id) else abrirFuera() }.padding(10.dp).testTag("tarjeta_documento"),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Box(Modifier.size(44.dp).clip(RoundedCornerShape(10.dp)).background(colorDeFamilia(familia)), Alignment.Center) {
                Text(TipoArchivo.etiqueta(m.nombre, m.mime), color = Color.White, style = MaterialTheme.typography.labelMedium, fontWeight = FontWeight.Bold, maxLines = 1)
            }
            Spacer(Modifier.width(10.dp))
            Column(Modifier.weight(1f)) {
                Text(nombre, maxLines = 2, overflow = TextOverflow.Ellipsis, fontWeight = FontWeight.Medium)
                Text(if (m.bytes > 0) formatoBytes(m.bytes) else "Tamaño desconocido", style = MaterialTheme.typography.bodySmall, color = LocalContentColorSuave())
            }
            if (!m.descargado) {
                Spacer(Modifier.width(8.dp))
                if (m.estado == "pendiente") CircularProgressIndicator(Modifier.size(28.dp), strokeWidth = 3.dp)
                else if (m.puedeBajarse) IconButton({ haptica(Toque.Fuerte); vm.descargar(m) }) { Icon(Ico.Descargar, if (m.estado == "fallido") "Reintentar" else "Descargar") }
            }
        }
        if (m.descargado) {
            androidx.compose.material3.TextButton({ haptica(Toque.Suave); abrirFuera() }, Modifier.align(Alignment.End).testTag("abrir_con")) { Text("Abrir con…") }
        }
    }
}

@Composable
private fun colorDeFamilia(f: TipoArchivo.Familia): Color = when (f) {
    TipoArchivo.Familia.PDF -> Color(0xFFC62828)
    TipoArchivo.Familia.HOJA -> Color(0xFF2E7D32)
    TipoArchivo.Familia.TEXTO -> Color(0xFF1565C0)
    TipoArchivo.Familia.PRESENTACION -> Color(0xFFE65100)
    TipoArchivo.Familia.COMPRIMIDO -> Color(0xFF6A4C93)
    TipoArchivo.Familia.APP -> Color(0xFF00695C)
    else -> Color(0xFF546E7A)
}

// ---------- foto grande ----------

@Composable
fun FotoGrandeDialogo(botId: Long, chat: String, nombre: String, alCerrar: () -> Unit) {
    val ctx = LocalContext.current
    var imagen by remember { mutableStateOf<androidx.compose.ui.graphics.ImageBitmap?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    LaunchedEffect(chat) {
        try {
            val rel = Repo.fotoGrande(botId, chat)
            val bmp = CacheImagenes.cargar(ctx, Repo.archivo(rel), 1600, "grande")
            if (bmp != null) imagen = bmp.asImageBitmap() else error = "No se pudo abrir la foto"
        } catch (e: Exception) {
            error = e.message ?: "No se pudo traer la foto"
        }
    }
    androidx.compose.ui.window.Dialog(alCerrar, androidx.compose.ui.window.DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(Modifier.fillMaxSize().clickable(onClick = alCerrar), color = Color.Black.copy(alpha = 0.92f)) {
            Box(contentAlignment = Alignment.Center) {
                val img = imagen
                when {
                    img != null -> Image(img, "Foto de perfil de $nombre", Modifier.fillMaxWidth(), contentScale = ContentScale.Fit)
                    error != null -> EstadoVacio(Ico.Foto, "Sin foto", error ?: "")
                    else -> CircularProgressIndicator(color = Color.White)
                }
                IconButton(alCerrar, Modifier.align(Alignment.TopStart).padding(Esp.s)) { Icon(Ico.Cerrar, "Cerrar", tint = Color.White) }
            }
        }
    }
}


/** Texto del mensaje con formato de WhatsApp, enlaces tocables, @menciones por nombre y emojis grandes cuando es solo emojis. */
@Composable
private fun TextoMensaje(m: Msg, grande: Boolean = true) {
    val esquema = MaterialTheme.colorScheme
    val formateado = CacheTexto.formateado(m.texto, m.menciones, esquema.primary) // casi siempre ya está listo (se prepara fuera de la composición)
    val soloEmojis = grande && m.citaId.isEmpty() && CacheTexto.soloEmojis(m.texto)
    Text(formateado, Modifier.testTag("texto_${m.id}"), style = if (soloEmojis) MaterialTheme.typography.displaySmall else MaterialTheme.typography.bodyLarge)
}


/** "Ver una vez": con contenido se abre una sola vez; redactado, avisa que hay que abrirlo en el teléfono. */
@Composable
private fun TarjetaVerUnaVez(m: Msg, alAbrir: (String) -> Unit) {
    val haptica = rememberHaptica()
    val (titulo, detalle, abrible) = when {
        m.borrado -> Triple("Ver una vez", "El remitente lo eliminó", false)
        m.verEstado == "pendiente" && m.descargado -> Triple("Ver una vez", "Toca para abrir. Solo se puede ver una vez.", true)
        m.verEstado == "pendiente" && m.estado == "pendiente" -> Triple("Ver una vez", "Descargando…", false)
        m.verEstado == "pendiente" -> Triple("Ver una vez", "No se pudo descargar; ábrelo en tu teléfono", false)
        m.verEstado == "abierto" -> Triple("Ver una vez", "Abierto", false)
        m.verEstado == "visto" -> Triple("Ver una vez", "Ya visto", false)
        m.verEstado == "expirado" -> Triple("Ver una vez", "Expiró", false)
        else -> Triple("Ver una vez", "Ábrelo en tu teléfono", false)
    }
    Row(Modifier.widthIn(min = 220.dp).clip(RoundedCornerShape(12.dp)).then(if (abrible) Modifier.clickable { haptica(Toque.Suave); alAbrir(m.id) } else Modifier).padding(vertical = 4.dp).testTag("tarjeta_ver_una_vez"), verticalAlignment = Alignment.CenterVertically) {
        Icon(Ico.Candado, null, Modifier.size(24.dp)); Spacer(Modifier.width(10.dp))
        Column { Text(titulo, fontWeight = FontWeight.SemiBold); Text(detalle, style = MaterialTheme.typography.bodySmall, color = LocalContentColorSuave()) }
    }
}
