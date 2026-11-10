@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.provider.Settings
import android.widget.Toast
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.animateContentSize
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.scale
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.botchat.app.AppVm
import com.botchat.app.BotState
import com.botchat.app.Paises
import com.botchat.app.Pantalla
import com.botchat.app.Telefono

/** Textos de estado del bot para la gente (el módulo Go avisa con palabras clave). */
fun mensajeEstado(e: String): String = when {
    e == "conectado" -> "Conectado y respondiendo"
    e == "conectando" || e == "pidiendo_codigo" -> "Conectando…"
    e == "desconectado" -> "Sin conexión. Reintentando…"
    e == "detenido" -> "Detenido"
    e == "sin_vincular" || e == "esperando_codigo" -> "Falta vincular"
    e == "vinculado" -> "Vinculado"
    e == "sesion_cerrada" -> "WhatsApp cerró la sesión. Vincula de nuevo."
    e.startsWith("error:") -> "Problema: " + e.removePrefix("error:").trim()
    else -> e
}

private fun copia(ctx: Context, etiqueta: String, texto: String, aviso: String) {
    ctx.getSystemService(ClipboardManager::class.java).setPrimaryClip(ClipData.newPlainText(etiqueta, texto))
    Toast.makeText(ctx, aviso, Toast.LENGTH_SHORT).show()
}

/** Pasos del asistente "Crear bot", con deslizamiento entre pantallas. */
@Composable
fun Asistente(vm: AppVm, conPermiso: (() -> Unit) -> Unit) {
    val reducir = LocalReducirMovimiento.current
    AnimatedContent(
        targetState = vm.pantalla,
        transitionSpec = {
            val adelante = targetState.ordinal >= initialState.ordinal
            if (reducir) fadeIn(tween(0)) togetherWith fadeOut(tween(0))
            else (slideInHorizontally(tween(Mov.LARGA)) { if (adelante) it / 5 else -it / 5 } + fadeIn(tween(Mov.NORMAL))) togetherWith
                (slideOutHorizontally(tween(Mov.LARGA)) { if (adelante) -it / 5 else it / 5 } + fadeOut(tween(Mov.CORTA)))
        },
        label = "asistente",
    ) { p ->
        when (p) {
            Pantalla.NOMBRE -> PasoNombre(vm)
            Pantalla.NUMERO -> PasoNumero(vm)
            Pantalla.PAIS -> ListaPaises(vm)
            Pantalla.CONFIRMAR -> PasoConfirmar(vm, conPermiso)
            Pantalla.CODIGO -> PasoCodigo(vm, conPermiso)
            Pantalla.LISTO -> PasoListo(vm)
            else -> {}
        }
    }
}

@Composable
private fun MarcoAsistente(paso: Int, titulo: String, subtitulo: String?, alVolver: (() -> Unit)?, pie: @Composable () -> Unit, contenido: @Composable () -> Unit) {
    val reducir = LocalReducirMovimiento.current
    val avance by animateFloatAsState(paso / 3f, Mov.tiempo(reducir, Mov.LARGA), label = "avance")
    Scaffold(
        topBar = {
            Column {
                TopAppBar(
                    navigationIcon = { if (alVolver != null) IconButton(alVolver) { Icon(Ico.Atras, "Atrás") } },
                    title = { Text("Paso $paso de 3", style = MaterialTheme.typography.titleMedium) },
                )
                LinearProgressIndicator(progress = { avance }, Modifier.fillMaxWidth().height(4.dp), trackColor = MaterialTheme.colorScheme.surfaceContainerHighest)
            }
        },
        bottomBar = { Surface(color = MaterialTheme.colorScheme.surface) { Column(Modifier.padding(horizontal = Esp.xl, vertical = Esp.m).imePadding()) { pie() } } },
    ) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(horizontal = Esp.xl, vertical = Esp.l)) {
            Text(titulo, style = MaterialTheme.typography.headlineMedium)
            if (subtitulo != null) { Spacer(Modifier.height(Esp.s)); Text(subtitulo, style = MaterialTheme.typography.bodyLarge, color = MaterialTheme.colorScheme.onSurfaceVariant) }
            Spacer(Modifier.height(Esp.xl))
            contenido()
        }
    }
}

@Composable
private fun BotonPrincipal(texto: String, enabled: Boolean = true, onClick: () -> Unit) {
    val haptica = rememberHaptica()
    Button({ haptica(Toque.Suave); onClick() }, Modifier.fillMaxWidth().heightIn(min = 60.dp), enabled = enabled, shape = RoundedCornerShape(18.dp)) {
        Text(texto, fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
    }
}

// ---------- paso 1: nombre ----------

@Composable
private fun PasoNombre(vm: AppVm) {
    MarcoAsistente(1, "¿Cómo se llamará tu bot?", "Este nombre aparece en la pantalla principal y en la notificación.", null, pie = {
        BotonPrincipal("Siguiente", enabled = vm.nombre.isNotBlank(), onClick = vm::siguienteNombre)
    }) {
        OutlinedTextField(
            value = vm.nombre, onValueChange = vm::cambiaNombre, singleLine = true, isError = vm.errorNombre != null,
            textStyle = MaterialTheme.typography.headlineSmall, shape = RoundedCornerShape(18.dp),
            keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Words),
            placeholder = { Text("Por ejemplo: Mi ayudante", style = MaterialTheme.typography.headlineSmall) },
            supportingText = { Text(vm.errorNombre ?: "${vm.nombre.length}/30") },
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

// ---------- vinculado: palomita animada ----------

@Composable
private fun PasoListo(vm: AppVm) {
    Column(Modifier.fillMaxSize().padding(Esp.xl), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
        CheckAnimado(Modifier.testTag("check_vinculado"), 120.dp)
        Spacer(Modifier.height(Esp.xl))
        Text("¡Listo!", style = MaterialTheme.typography.headlineMedium)
        Spacer(Modifier.height(Esp.s))
        Text("Tu bot quedó vinculado.", style = MaterialTheme.typography.bodyLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(Esp.xl))
        BotonPrincipal("Continuar", onClick = vm::terminarCelebracion)
    }
}

// ---------- paso 2: país y número ----------

@Composable
private fun PasoNumero(vm: AppVm) {
    val ctx = LocalContext.current
    val pais = Paises.porIso(vm.iso)
    MarcoAsistente(2, "¿Con qué número usarás el bot?", "Escríbelo como en WhatsApp. De preferencia, un número secundario.", { vm.atras(ctx) }, pie = {
        BotonPrincipal("Siguiente", onClick = vm::siguienteNumero)
    }) {
        Surface(Modifier.fillMaxWidth().clickable { vm.abrePaises() }, shape = RoundedCornerShape(18.dp), color = MaterialTheme.colorScheme.surfaceContainerHigh) {
            Row(Modifier.padding(horizontal = Esp.l, vertical = Esp.m).heightIn(min = 44.dp), verticalAlignment = Alignment.CenterVertically) {
                Text(Paises.banderaDe(pais.iso), fontSize = 30.sp)
                Spacer(Modifier.width(Esp.m))
                Text(pais.nombre, Modifier.weight(1f), style = MaterialTheme.typography.titleMedium)
                Text(pais.codigoTexto, style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.primary)
                Spacer(Modifier.width(Esp.s))
                Text("▾", style = MaterialTheme.typography.titleMedium)
            }
        }
        Spacer(Modifier.height(Esp.m))
        val formateado = Telefono.formatea(vm.iso, vm.digitos)
        OutlinedTextField(
            value = TextFieldValue(formateado, TextRange(formateado.length)), onValueChange = { vm.cambiaDigitos(it.text) },
            singleLine = true, isError = vm.errorNumero != null, shape = RoundedCornerShape(18.dp),
            textStyle = MaterialTheme.typography.headlineSmall,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
            prefix = { Text(pais.codigoTexto + "  ", style = MaterialTheme.typography.headlineSmall, color = MaterialTheme.colorScheme.primary) },
            placeholder = { Text(if (vm.iso == "MX") "55 1234 5678" else "Tu número", style = MaterialTheme.typography.headlineSmall) },
            supportingText = { Text(vm.errorNumero ?: " ") },
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

@Composable
private fun ListaPaises(vm: AppVm) {
    val ctx = LocalContext.current
    var consulta by rememberSaveable { mutableStateOf("") }
    val lista = remember(consulta) { Paises.buscar(consulta) }
    val estado = rememberLazyListState()
    // Al abrir, la lista se desplaza hasta el país elegido; al buscar, vuelve arriba.
    LaunchedEffect(consulta) {
        if (consulta.isEmpty()) { val i = lista.indexOfFirst { it.iso == vm.iso }; if (i >= 0) estado.scrollToItem(i) } else estado.scrollToItem(0)
    }
    Scaffold(topBar = {
        Column {
            TopAppBar(navigationIcon = { IconButton({ vm.atras(ctx) }) { Icon(Ico.Atras, "Atrás") } }, title = { Text("Elige tu país") })
            OutlinedTextField(
                consulta, { consulta = it }, singleLine = true, shape = RoundedCornerShape(50),
                placeholder = { Text("Buscar por nombre o código") }, leadingIcon = { Icon(Ico.Buscar, null) },
                trailingIcon = { if (consulta.isNotEmpty()) IconButton({ consulta = "" }) { Icon(Ico.Cerrar, "Borrar") } },
                modifier = Modifier.fillMaxWidth().padding(horizontal = Esp.l, vertical = Esp.s),
            )
        }
    }) { relleno ->
        if (lista.isEmpty()) EstadoVacio(Ico.Buscar, "Sin resultados", "No hay países con ese nombre o código.", Modifier.padding(relleno).padding(top = 60.dp))
        LazyColumn(Modifier.fillMaxSize().padding(relleno), state = estado) {
            items(lista, key = { it.iso }) { p ->
                val elegido = p.iso == vm.iso
                Row(
                    Modifier.fillMaxWidth().animateItem().background(if (elegido) MaterialTheme.colorScheme.secondaryContainer else Color.Transparent).clickable { vm.eligePais(p) }
                        .heightIn(min = 60.dp).padding(horizontal = Esp.l, vertical = Esp.s),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(Paises.banderaDe(p.iso), fontSize = 26.sp, modifier = Modifier.width(48.dp))
                    Text(p.nombre, Modifier.weight(1f), style = MaterialTheme.typography.bodyLarge)
                    Text(p.codigoTexto, style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.primary)
                    if (elegido) { Spacer(Modifier.width(Esp.s)); Icon(Ico.Listo, "Elegido", tint = MaterialTheme.colorScheme.primary) }
                }
                HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.4f))
            }
        }
    }
}

// ---------- paso 3: confirmar y vincular ----------

@Composable
private fun PasoConfirmar(vm: AppVm, conPermiso: (() -> Unit) -> Unit) {
    val ctx = LocalContext.current
    var avanzado by rememberSaveable { mutableStateOf(false) }
    val reenlace = vm.enRevinculacion
    MarcoAsistente(
        3, if (reenlace) "Vincular de nuevo" else "¿Es correcto?",
        if (reenlace) "WhatsApp cerró la sesión de este bot. Tus mensajes guardados no se pierden." else null,
        { vm.atras(ctx) },
        pie = {
            BotonPrincipal("Sí, vincular") { conPermiso { vm.vincular(ctx) } }
            Spacer(Modifier.height(Esp.s))
            OutlinedButton({ if (reenlace) vm.atras(ctx) else vm.corregir(ctx) }, Modifier.fillMaxWidth().heightIn(min = 54.dp), shape = RoundedCornerShape(18.dp)) { Text(if (reenlace) "Ahora no" else "Corregir", fontSize = 17.sp) }
        },
    ) {
        Card(Modifier.fillMaxWidth(), shape = RoundedCornerShape(22.dp), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainerHigh)) {
            Column(Modifier.padding(Esp.xl)) {
                Text("Tu bot", style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
                Text(vm.nombre, style = MaterialTheme.typography.headlineSmall)
                Spacer(Modifier.height(Esp.l))
                Text("usará el número", style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(Paises.banderaDe(vm.iso), fontSize = 24.sp); Spacer(Modifier.width(Esp.s))
                    Text(vm.resumenNumero(), style = MaterialTheme.typography.headlineSmall)
                }
            }
        }
        if (vm.iso == "MX") {
            Spacer(Modifier.height(Esp.l))
            TextButton({ avanzado = !avanzado }) { Text(if (avanzado) "Opciones avanzadas ▾" else "Opciones avanzadas ▸") }
            if (avanzado) {
                Row(Modifier.animateContentSize(), verticalAlignment = Alignment.CenterVertically) {
                    Column(Modifier.weight(1f)) {
                        Text("Formato antiguo de México (521…)", style = MaterialTheme.typography.bodyLarge)
                        Text("Actívalo solo si WhatsApp dice que el código es incorrecto.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                    Switch(vm.mx1, { vm.mx1 = it })
                }
            }
        }
    }
}

// ---------- código de 8 caracteres ----------

@Composable
private fun PasoCodigo(vm: AppVm, conPermiso: (() -> Unit) -> Unit) {
    val ctx = LocalContext.current
    val codigo by BotState.codigo.collectAsState()
    val estado by BotState.estado.collectAsState()
    val error = estado.startsWith("error")
    val haptica = rememberHaptica()
    MarcoAsistente(3, "Escribe este código en WhatsApp", null, { vm.atras(ctx) }, pie = {
        if (codigo != null || error) OutlinedButton({ conPermiso { vm.vincular(ctx) } }, Modifier.fillMaxWidth().heightIn(min = 54.dp), shape = RoundedCornerShape(18.dp)) { Text(if (error) "Intentar de nuevo" else "Pedir otro código", fontSize = 17.sp) }
    }) {
        when {
            codigo != null -> {
                Card(Modifier.fillMaxWidth(), shape = RoundedCornerShape(24.dp), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.primaryContainer)) {
                    Text(
                        codigo ?: "", Modifier.fillMaxWidth().padding(vertical = 30.dp, horizontal = Esp.s), style = EstiloCodigo, fontSize = 44.sp, letterSpacing = 4.sp,
                        textAlign = TextAlign.Center, maxLines = 1, softWrap = false, color = MaterialTheme.colorScheme.onPrimaryContainer,
                    )
                }
                Spacer(Modifier.height(Esp.m))
                OutlinedButton({ haptica(Toque.Confirmar); copia(ctx, "código", (codigo ?: "").replace("-", ""), "Código copiado") }, Modifier.fillMaxWidth().heightIn(min = 54.dp), shape = RoundedCornerShape(18.dp)) {
                    Icon(Ico.Copiar, null, Modifier.size(20.dp)); Spacer(Modifier.width(8.dp)); Text("Copiar código", fontSize = 17.sp)
                }
                Spacer(Modifier.height(Esp.xl))
                listOf("Abre WhatsApp en el teléfono de ese número", "Entra a Dispositivos vinculados", "Toca Vincular con el número de teléfono", "Escribe este código").forEachIndexed { i, t ->
                    Row(Modifier.padding(vertical = 6.dp), verticalAlignment = Alignment.CenterVertically) {
                        Box(Modifier.size(30.dp).clip(CircleShape).background(MaterialTheme.colorScheme.secondaryContainer), Alignment.Center) { Text("${i + 1}", fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.onSecondaryContainer) }
                        Spacer(Modifier.width(Esp.m)); Text(t, style = MaterialTheme.typography.bodyLarge)
                    }
                }
                Spacer(Modifier.height(Esp.m))
                Text("Esta pantalla pasa sola cuando WhatsApp acepte el código.", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            error -> {
                Text("No se pudo pedir el código.", style = MaterialTheme.typography.titleLarge, color = MaterialTheme.colorScheme.error)
                Spacer(Modifier.height(Esp.s)); Text(mensajeEstado(estado), style = MaterialTheme.typography.bodyLarge)
            }
            else -> {
                Box(Modifier.fillMaxWidth().height(110.dp).clip(RoundedCornerShape(24.dp)).shimmer())
                Spacer(Modifier.height(Esp.l))
                Row(verticalAlignment = Alignment.CenterVertically) { CircularProgressIndicator(Modifier.size(28.dp), strokeWidth = 3.dp); Spacer(Modifier.width(Esp.l)); Text("Pidiendo el código a WhatsApp…", style = MaterialTheme.typography.bodyLarge) }
            }
        }
    }
}

// ---------- estado del bot ----------

@Composable
fun PantallaBot(vm: AppVm, conPermiso: (() -> Unit) -> Unit, alVolver: () -> Unit, alAbrirAvanzado: () -> Unit, alAbrirAyuda: () -> Unit) {
    val ctx = LocalContext.current
    val estado by BotState.estado.collectAsState()
    val corriendo by BotState.corriendo.collectAsState()
    val haptica = rememberHaptica()
    val reducir = LocalReducirMovimiento.current
    val bot = vm.bot
    val verde = corriendo && estado == "conectado"
    val trabajando = corriendo && !verde && !estado.startsWith("error") && estado != "sesion_cerrada"
    val color = when {
        verde -> LocalColoresApp.current.conectado
        estado.startsWith("error") || estado == "sesion_cerrada" -> MaterialTheme.colorScheme.error
        trabajando -> MaterialTheme.colorScheme.tertiary
        else -> MaterialTheme.colorScheme.outline
    }
    val pulso = rememberInfiniteTransition(label = "pulso").animateFloat(1f, 1.18f, infiniteRepeatable(tween(900), RepeatMode.Reverse), label = "p")
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Estado del bot") }) }) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.xl), horizontalAlignment = Alignment.CenterHorizontally) {
            Text(bot?.nombre ?: "", style = MaterialTheme.typography.headlineMedium, textAlign = TextAlign.Center)
            bot?.let { Text("${Paises.banderaDe(it.iso)}  ${Telefono.internacional(it.iso, it.numero)}", style = MaterialTheme.typography.bodyLarge, color = MaterialTheme.colorScheme.onSurfaceVariant) }
            Spacer(Modifier.height(Esp.xl))
            Box(Modifier.size(190.dp), Alignment.Center) {
                if (trabajando && !reducir) Box(Modifier.size(150.dp).scale(pulso.value).clip(CircleShape).background(color.copy(alpha = 0.18f)))
                Box(Modifier.size(150.dp).clip(CircleShape).background(color), Alignment.Center) {
                    Text(if (verde) "✓" else if (trabajando) "…" else "•", color = Color.White, fontSize = 64.sp, fontWeight = FontWeight.Bold)
                }
            }
            Spacer(Modifier.height(Esp.l))
            Text(if (!corriendo && estado != "detenido") "Detenido" else mensajeEstado(estado), style = MaterialTheme.typography.titleLarge, textAlign = TextAlign.Center)
            Spacer(Modifier.height(Esp.xl))
            if (bot != null && !bot.vinculado) {
                BotonPrincipal("Vincular de nuevo") { vm.revincular() }
            } else {
                Button(
                    { haptica(Toque.Fuerte); if (corriendo) vm.iniciaODetiene(ctx) else conPermiso { vm.iniciaODetiene(ctx) } },
                    Modifier.fillMaxWidth().heightIn(min = 64.dp), shape = RoundedCornerShape(20.dp),
                    colors = if (corriendo) ButtonDefaults.buttonColors(containerColor = MaterialTheme.colorScheme.errorContainer, contentColor = MaterialTheme.colorScheme.onErrorContainer) else ButtonDefaults.buttonColors(),
                ) { Text(if (corriendo) "Detener" else "Iniciar", fontSize = 20.sp, fontWeight = FontWeight.SemiBold) }
            }
            Spacer(Modifier.height(Esp.m))
            vm.aviso?.let {
                Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.secondaryContainer)) {
                    Column(Modifier.padding(Esp.l)) { Text(it); TextButton({ vm.aviso = null }) { Text("Entendido") } }
                }
                Spacer(Modifier.height(Esp.m))
            }
            OutlinedButton(alAbrirAyuda, Modifier.fillMaxWidth().heightIn(min = 54.dp), shape = RoundedCornerShape(18.dp)) { Text("¿Se apaga solo? Ver ayuda de batería") }
            Spacer(Modifier.height(Esp.s))
            TextButton(alAbrirAvanzado) { Text("Avanzado") }
        }
    }
}

@Composable
fun AyudaBateria(alVolver: () -> Unit) {
    val ctx = LocalContext.current
    Scaffold(topBar = { TopAppBar(navigationIcon = { IconButton(alVolver) { Icon(Ico.Atras, "Volver") } }, title = { Text("Que el bot no se apague") }) }) { relleno ->
        Column(Modifier.fillMaxSize().padding(relleno).verticalScroll(rememberScrollState()).padding(Esp.xl)) {
            Text("Android a veces cierra las apps para ahorrar batería. Para que el bot siga respondiendo:", style = MaterialTheme.typography.bodyLarge)
            Spacer(Modifier.height(Esp.l))
            listOf(
                "Toca el botón de abajo.",
                "Busca “Bot Chat” y elige “No optimizar” o “Sin restricciones”.",
                "En Ajustes > Aplicaciones > Bot Chat > Batería, elige “Sin restricciones”.",
                "En Xiaomi, Huawei, Oppo, Vivo o Samsung, activa también “Inicio automático” y bloquea la app en la lista de apps recientes.",
                "No quites la notificación fija: es la que mantiene vivo al bot.",
            ).forEachIndexed { i, t ->
                Row(Modifier.padding(vertical = 7.dp), verticalAlignment = Alignment.Top) {
                    Box(Modifier.size(30.dp).clip(CircleShape).background(MaterialTheme.colorScheme.secondaryContainer), Alignment.Center) { Text("${i + 1}", fontWeight = FontWeight.Bold, color = MaterialTheme.colorScheme.onSecondaryContainer) }
                    Spacer(Modifier.width(Esp.m)); Text(t, style = MaterialTheme.typography.bodyLarge, modifier = Modifier.padding(top = 3.dp))
                }
            }
            Spacer(Modifier.height(Esp.xl))
            BotonPrincipal("Abrir ajustes de batería") {
                runCatching { ctx.startActivity(Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) }
                    .onFailure { ctx.startActivity(Intent(Settings.ACTION_SETTINGS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) }
            }
        }
    }
}
