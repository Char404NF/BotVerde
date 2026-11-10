package com.botchat.app

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.provider.Settings
import android.widget.Toast
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
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
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

private val Azul = Color(0xFF2F5BEA)
private val Verde = Color(0xFF22A559)
private val Rojo = Color(0xFFD92D20)

@Composable
fun Tema(contenido: @Composable () -> Unit) {
    val oscuro = isSystemInDarkTheme()
    MaterialTheme(
        colorScheme = if (oscuro) {
            darkColorScheme(primary = Color(0xFF8AA4FF), onPrimary = Color(0xFF0B1B52), background = Color(0xFF10131A), surface = Color(0xFF10131A), surfaceVariant = Color(0xFF1C2230))
        } else {
            lightColorScheme(primary = Azul, onPrimary = Color.White, background = Color(0xFFF6F7FB), surface = Color(0xFFF6F7FB), surfaceVariant = Color(0xFFE6EAF5))
        },
        content = contenido,
    )
}

@Composable
fun Pantallas(vm: AppVm, conPermiso: (() -> Unit) -> Unit) {
    when (vm.pantalla) {
        Pantalla.NOMBRE -> PasoNombre(vm)
        Pantalla.NUMERO -> PasoNumero(vm)
        Pantalla.PAIS -> ListaPaises(vm)
        Pantalla.CONFIRMAR -> PasoConfirmar(vm, conPermiso)
        Pantalla.CODIGO -> PasoCodigo(vm, conPermiso)
        Pantalla.PRINCIPAL -> Principal(vm, conPermiso)
        Pantalla.AYUDA -> Ayuda(vm)
    }
}

// ---------- piezas comunes ----------

@Composable
private fun Marco(paso: Int?, titulo: String, atras: (() -> Unit)?, contenido: @Composable ColumnScope.() -> Unit) {
    Column(Modifier.fillMaxSize().padding(horizontal = 20.dp, vertical = 12.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            if (atras != null) {
                TextButton(onClick = atras) { Text("‹ Atrás", fontSize = 18.sp) }
            }
            Spacer(Modifier.weight(1f))
            if (paso != null) Text("Paso $paso de 3", fontSize = 18.sp, fontWeight = FontWeight.Medium)
        }
        if (paso != null) {
            LinearProgressIndicator(progress = { paso / 3f }, modifier = Modifier.fillMaxWidth().height(8.dp).clip(RoundedCornerShape(4.dp)))
        }
        Spacer(Modifier.height(20.dp))
        Text(titulo, fontSize = 28.sp, fontWeight = FontWeight.Bold, lineHeight = 34.sp)
        Spacer(Modifier.height(16.dp))
        contenido()
    }
}

@Composable
private fun BotonGrande(texto: String, modifier: Modifier = Modifier, enabled: Boolean = true, colores: ButtonColorsOpc = ButtonColorsOpc.Normal, onClick: () -> Unit) {
    val c = when (colores) {
        ButtonColorsOpc.Normal -> ButtonDefaults.buttonColors()
        ButtonColorsOpc.Rojo -> ButtonDefaults.buttonColors(containerColor = Rojo, contentColor = Color.White)
    }
    Button(onClick = onClick, enabled = enabled, colors = c, shape = RoundedCornerShape(16.dp), modifier = modifier.fillMaxWidth().heightIn(min = 64.dp)) {
        Text(texto, fontSize = 20.sp, fontWeight = FontWeight.SemiBold)
    }
}

private enum class ButtonColorsOpc { Normal, Rojo }

@Composable
private fun BotonSecundario(texto: String, modifier: Modifier = Modifier, onClick: () -> Unit) {
    OutlinedButton(onClick = onClick, shape = RoundedCornerShape(16.dp), modifier = modifier.fillMaxWidth().heightIn(min = 60.dp)) {
        Text(texto, fontSize = 18.sp)
    }
}

private fun copia(ctx: Context, etiqueta: String, texto: String, aviso: String) {
    val cm = ctx.getSystemService(ClipboardManager::class.java)
    cm.setPrimaryClip(ClipData.newPlainText(etiqueta, texto))
    Toast.makeText(ctx, aviso, Toast.LENGTH_SHORT).show()
}

// ---------- paso 1: nombre ----------

@Composable
private fun PasoNombre(vm: AppVm) {
    Marco(1, "¿Cómo se llamará tu bot?", null) {
        OutlinedTextField(
            value = vm.nombre, onValueChange = vm::cambiaNombre,
            textStyle = MaterialTheme.typography.headlineSmall.copy(fontSize = 24.sp),
            singleLine = true, isError = vm.errorNombre != null,
            keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Words),
            placeholder = { Text("Por ejemplo: Mi ayudante", fontSize = 20.sp) },
            supportingText = { Text(vm.errorNombre ?: "${vm.nombre.length}/30", fontSize = 15.sp) },
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(8.dp))
        Text("Este nombre aparece en la pantalla principal y en la notificación.", fontSize = 16.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(24.dp))
        BotonGrande("Siguiente", enabled = vm.nombre.isNotBlank(), onClick = vm::siguienteNombre)
    }
}

// ---------- paso 2: país y número ----------

@Composable
private fun PasoNumero(vm: AppVm) {
    val ctx = LocalContext.current
    val pais = Paises.porIso(vm.iso)
    Marco(2, "¿Con qué número usarás el bot?", { vm.atras(ctx) }) {
        Text("Escribe el número como en WhatsApp.", fontSize = 16.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(12.dp))
        Surface(
            shape = RoundedCornerShape(16.dp), color = MaterialTheme.colorScheme.surfaceVariant,
            modifier = Modifier.fillMaxWidth().heightIn(min = 64.dp).clickable { vm.abrePaises() },
        ) {
            Row(Modifier.padding(horizontal = 16.dp, vertical = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                Text(pais.bandera, fontSize = 28.sp)
                Spacer(Modifier.width(12.dp))
                Text(pais.nombre, fontSize = 20.sp, modifier = Modifier.weight(1f))
                Text(pais.codigoTexto, fontSize = 20.sp, fontWeight = FontWeight.SemiBold)
                Spacer(Modifier.width(8.dp))
                Text("▾", fontSize = 20.sp)
            }
        }
        Spacer(Modifier.height(12.dp))
        val formateado = Telefono.formatea(vm.iso, vm.digitos)
        Row(verticalAlignment = Alignment.Top) {
            Surface(
                shape = RoundedCornerShape(4.dp), color = Color.Transparent, border = BorderStroke(1.dp, MaterialTheme.colorScheme.outline),
                modifier = Modifier.padding(top = 0.dp).height(64.dp),
            ) {
                Box(Modifier.padding(horizontal = 16.dp), contentAlignment = Alignment.Center) {
                    Text(pais.codigoTexto, fontSize = 22.sp, fontWeight = FontWeight.SemiBold)
                }
            }
            Spacer(Modifier.width(8.dp))
            OutlinedTextField(
                value = TextFieldValue(formateado, TextRange(formateado.length)),
                onValueChange = { vm.cambiaDigitos(it.text) },
                textStyle = MaterialTheme.typography.headlineSmall.copy(fontSize = 24.sp),
                singleLine = true, isError = vm.errorNumero != null,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                placeholder = { Text(if (vm.iso == "MX") "55 1234 5678" else "Tu número", fontSize = 22.sp) },
                supportingText = { vm.errorNumero?.let { Text(it, fontSize = 15.sp) } },
                modifier = Modifier.weight(1f),
            )
        }
        Spacer(Modifier.height(24.dp))
        BotonGrande("Siguiente", onClick = vm::siguienteNumero)
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
        if (consulta.isEmpty()) {
            val i = lista.indexOfFirst { it.iso == vm.iso }
            if (i >= 0) estado.scrollToItem(i)
        } else {
            estado.scrollToItem(0)
        }
    }
    Column(Modifier.fillMaxSize().padding(horizontal = 16.dp, vertical = 8.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            TextButton(onClick = { vm.atras(ctx) }) { Text("‹ Atrás", fontSize = 18.sp) }
            Spacer(Modifier.weight(1f))
        }
        Text("Elige tu país", fontSize = 26.sp, fontWeight = FontWeight.Bold)
        Spacer(Modifier.height(8.dp))
        OutlinedTextField(
            value = consulta, onValueChange = { consulta = it }, singleLine = true,
            placeholder = { Text("Buscar por nombre o código", fontSize = 18.sp) },
            textStyle = MaterialTheme.typography.bodyLarge.copy(fontSize = 20.sp),
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(8.dp))
        if (lista.isEmpty()) {
            Text("No hay países con ese nombre.", fontSize = 18.sp, modifier = Modifier.padding(16.dp))
        }
        LazyColumn(state = estado, modifier = Modifier.fillMaxSize()) {
            items(lista, key = { it.iso }) { p ->
                val elegido = p.iso == vm.iso
                Row(
                    Modifier.fillMaxWidth().heightIn(min = 60.dp)
                        .background(if (elegido) MaterialTheme.colorScheme.surfaceVariant else Color.Transparent)
                        .clickable { vm.eligePais(p) }
                        .padding(horizontal = 12.dp, vertical = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(p.bandera, fontSize = 26.sp, modifier = Modifier.width(48.dp))
                    Text(p.nombre, fontSize = 19.sp, modifier = Modifier.weight(1f))
                    Text(p.codigoTexto, fontSize = 19.sp, fontWeight = FontWeight.SemiBold)
                }
                HorizontalDivider()
            }
        }
    }
}

// ---------- paso 3: confirmar y vincular ----------

@Composable
private fun PasoConfirmar(vm: AppVm, conPermiso: (() -> Unit) -> Unit) {
    val ctx = LocalContext.current
    var avanzado by rememberSaveable { mutableStateOf(false) }
    Marco(3, "¿Es correcto?", { vm.atras(ctx) }) {
        Column(Modifier.verticalScroll(rememberScrollState())) {
            Surface(shape = RoundedCornerShape(16.dp), color = MaterialTheme.colorScheme.surfaceVariant, modifier = Modifier.fillMaxWidth()) {
                Column(Modifier.padding(20.dp)) {
                    Text("Tu bot", fontSize = 16.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Text(vm.nombre, fontSize = 26.sp, fontWeight = FontWeight.Bold)
                    Spacer(Modifier.height(12.dp))
                    Text("usará el número", fontSize = 16.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Text(vm.resumenNumero(), fontSize = 26.sp, fontWeight = FontWeight.Bold)
                }
            }
            Spacer(Modifier.height(20.dp))
            BotonGrande("Sí, vincular") { conPermiso { vm.vincular(ctx) } }
            Spacer(Modifier.height(12.dp))
            BotonSecundario("Corregir") { vm.corregir(ctx) }
            if (vm.iso == "MX") {
                Spacer(Modifier.height(16.dp))
                TextButton(onClick = { avanzado = !avanzado }) { Text(if (avanzado) "Opciones avanzadas ▾" else "Opciones avanzadas ▸", fontSize = 16.sp) }
                if (avanzado) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) {
                            Text("Formato antiguo de México (521…)", fontSize = 17.sp)
                            Text("Actívalo solo si WhatsApp dice que el código es incorrecto.", fontSize = 14.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                        Switch(checked = vm.mx1, onCheckedChange = { vm.mx1 = it })
                    }
                }
            }
        }
    }
}

// ---------- código de 8 dígitos ----------

@Composable
private fun PasoCodigo(vm: AppVm, conPermiso: (() -> Unit) -> Unit) {
    val ctx = LocalContext.current
    val codigo by BotState.codigo.collectAsState()
    val estado by BotState.estado.collectAsState()
    val error = estado.startsWith("error")
    Marco(3, "Escribe este código en WhatsApp", { vm.atras(ctx) }) {
        Column(Modifier.verticalScroll(rememberScrollState())) {
            when {
                codigo != null -> {
                    Surface(shape = RoundedCornerShape(20.dp), color = MaterialTheme.colorScheme.surfaceVariant, modifier = Modifier.fillMaxWidth()) {
                        Text(
                            codigo ?: "", fontSize = 46.sp, fontWeight = FontWeight.Bold, fontFamily = FontFamily.Monospace,
                            letterSpacing = 4.sp, textAlign = TextAlign.Center, maxLines = 1, softWrap = false,
                            modifier = Modifier.fillMaxWidth().padding(vertical = 28.dp, horizontal = 8.dp),
                        )
                    }
                    Spacer(Modifier.height(12.dp))
                    BotonSecundario("Copiar código") { copia(ctx, "código", (codigo ?: "").replace("-", ""), "Código copiado") }
                    Spacer(Modifier.height(16.dp))
                    Text("Abre WhatsApp > Dispositivos vinculados > Vincular con el número de teléfono > escribe este código.", fontSize = 19.sp, lineHeight = 26.sp)
                    Spacer(Modifier.height(12.dp))
                    Text("Esta pantalla pasa sola cuando WhatsApp acepte el código.", fontSize = 16.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                error -> {
                    Text("No se pudo pedir el código.", fontSize = 20.sp, fontWeight = FontWeight.SemiBold, color = Rojo)
                    Spacer(Modifier.height(8.dp))
                    Text(mensajeEstado(estado), fontSize = 17.sp)
                }
                else -> {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        CircularProgressIndicator(Modifier.size(36.dp))
                        Spacer(Modifier.width(16.dp))
                        Text("Pidiendo el código a WhatsApp…", fontSize = 19.sp)
                    }
                }
            }
            if (codigo != null || error) {
                Spacer(Modifier.height(20.dp))
                BotonSecundario(if (error) "Intentar de nuevo" else "Pedir otro código") { conPermiso { vm.vincular(ctx) } }
            }
        }
    }
}

// ---------- pantalla principal ----------

internal fun mensajeEstado(e: String): String = when {
    e == "conectado" -> "Conectado y respondiendo"
    e == "conectando" || e == "pidiendo_codigo" -> "Conectando…"
    e == "desconectado" -> "Sin conexión. Reintentando…"
    e == "detenido" -> "Detenido"
    e == "sin_vincular" || e == "esperando_codigo" -> "Falta vincular"
    e == "vinculado" -> "Vinculado"
    e == "sesion_cerrada" -> "WhatsApp cerró la sesión. Elimina el bot y vincula de nuevo."
    e.startsWith("error:") -> "Problema: " + e.removePrefix("error:").trim()
    else -> e
}

@Composable
private fun Principal(vm: AppVm, conPermiso: (() -> Unit) -> Unit) {
    val ctx = LocalContext.current
    val estado by BotState.estado.collectAsState()
    val corriendo by BotState.corriendo.collectAsState()
    val registro by BotState.registro.collectAsState()
    var avanzado by rememberSaveable { mutableStateOf(false) }
    var confirmaEliminar by remember { mutableStateOf(false) }
    val verde = corriendo && estado == "conectado"
    val textoEstado = if (!corriendo && estado != "detenido") "Detenido" else mensajeEstado(estado)

    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(horizontal = 20.dp, vertical = 16.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        Spacer(Modifier.height(12.dp))
        Text(vm.bot?.nombre ?: "", fontSize = 32.sp, fontWeight = FontWeight.Bold, textAlign = TextAlign.Center)
        vm.bot?.let { Text(Telefono.internacional(it.iso, it.numero), fontSize = 16.sp, color = MaterialTheme.colorScheme.onSurfaceVariant) }
        Spacer(Modifier.height(28.dp))
        Box(Modifier.size(150.dp).clip(CircleShape).background(if (verde) Verde else Rojo))
        Spacer(Modifier.height(16.dp))
        Text(textoEstado, fontSize = 20.sp, textAlign = TextAlign.Center, fontWeight = FontWeight.Medium)
        Spacer(Modifier.height(24.dp))
        BotonGrande(if (corriendo) "Detener" else "Iniciar", colores = if (corriendo) ButtonColorsOpc.Rojo else ButtonColorsOpc.Normal) {
            if (corriendo) vm.iniciaODetiene(ctx) else conPermiso { vm.iniciaODetiene(ctx) }
        }
        Spacer(Modifier.height(8.dp))
        TextButton(onClick = vm::abreAyuda) { Text("¿Se apaga solo? Ver ayuda", fontSize = 17.sp) }
        vm.aviso?.let {
            Spacer(Modifier.height(8.dp))
            Surface(shape = RoundedCornerShape(12.dp), color = MaterialTheme.colorScheme.surfaceVariant) {
                Column(Modifier.padding(16.dp)) {
                    Text(it, fontSize = 16.sp)
                    TextButton(onClick = { vm.aviso = null }) { Text("Entendido", fontSize = 16.sp) }
                }
            }
        }
        Spacer(Modifier.height(16.dp))
        HorizontalDivider()
        TextButton(onClick = { avanzado = !avanzado }, modifier = Modifier.fillMaxWidth()) {
            Text(if (avanzado) "Ajustes avanzados ▾" else "Ajustes avanzados ▸", fontSize = 18.sp)
        }
        if (avanzado) {
            Column(Modifier.fillMaxWidth()) {
                Text("Registro", fontSize = 20.sp, fontWeight = FontWeight.SemiBold)
                Spacer(Modifier.height(8.dp))
                val desplazamiento = rememberScrollState()
                LaunchedEffect(registro.size) { desplazamiento.scrollTo(desplazamiento.maxValue) }
                Surface(shape = RoundedCornerShape(12.dp), color = MaterialTheme.colorScheme.surfaceVariant, modifier = Modifier.fillMaxWidth().height(240.dp)) {
                    Text(
                        if (registro.isEmpty()) "Todavía no hay mensajes." else registro.joinToString("\n"),
                        fontFamily = FontFamily.Monospace, fontSize = 12.sp, lineHeight = 16.sp,
                        modifier = Modifier.verticalScroll(desplazamiento).padding(10.dp),
                    )
                }
                Spacer(Modifier.height(8.dp))
                BotonSecundario("Copiar registro") { copia(ctx, "registro", BotState.textoRegistro(), "Registro copiado") }
                Spacer(Modifier.height(24.dp))
                BotonGrande(if (vm.eliminando) "Eliminando…" else "Eliminar bot", enabled = !vm.eliminando, colores = ButtonColorsOpc.Rojo) { confirmaEliminar = true }
                Spacer(Modifier.height(8.dp))
                Text("Se cierra la sesión en WhatsApp y se borran los datos de este bot.", fontSize = 15.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }

    if (confirmaEliminar) {
        AlertDialog(
            onDismissRequest = { confirmaEliminar = false },
            title = { Text("¿Eliminar el bot?", fontSize = 22.sp) },
            text = { Text("Se cerrará la sesión en WhatsApp y se borrarán los datos de este bot. Esto no se puede deshacer.", fontSize = 18.sp) },
            confirmButton = { TextButton(onClick = { confirmaEliminar = false; vm.eliminar(ctx) }) { Text("Sí, eliminar", fontSize = 18.sp, color = Rojo) } },
            dismissButton = { TextButton(onClick = { confirmaEliminar = false }) { Text("Cancelar", fontSize = 18.sp) } },
        )
    }
}

// ---------- ayuda de batería ----------

@Composable
private fun Ayuda(vm: AppVm) {
    val ctx = LocalContext.current
    Marco(null, "Que el bot no se apague", { vm.atras(ctx) }) {
        Column(Modifier.verticalScroll(rememberScrollState())) {
            Text("Android a veces cierra las apps para ahorrar batería. Para que el bot siga respondiendo:", fontSize = 19.sp, lineHeight = 26.sp)
            Spacer(Modifier.height(16.dp))
            listOf(
                "1. Toca el botón de abajo.",
                "2. Busca \"Bot Chat\" y elige \"No optimizar\" o \"Sin restricciones\".",
                "3. En Ajustes > Aplicaciones > Bot Chat > Batería, elige \"Sin restricciones\".",
                "4. En Xiaomi, Huawei, Oppo, Vivo o Samsung, activa también \"Inicio automático\" y bloquea la app en la lista de apps recientes.",
                "5. No quites la notificación fija: es la que mantiene vivo al bot.",
            ).forEach { Text(it, fontSize = 18.sp, lineHeight = 25.sp, modifier = Modifier.padding(bottom = 10.dp)) }
            Spacer(Modifier.height(8.dp))
            BotonGrande("Abrir ajustes de batería") {
                runCatching { ctx.startActivity(Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) }
                    .onFailure { ctx.startActivity(Intent(Settings.ACTION_SETTINGS).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) }
            }
        }
    }
}
