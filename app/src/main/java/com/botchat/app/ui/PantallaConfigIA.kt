@file:OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)

package com.botchat.app.ui

import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.AssistChip
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.botchat.app.datos.CampoModulo
import com.botchat.app.datos.ModuloInfo
import com.botchat.app.datos.ProveedorIA
import com.botchat.app.datos.Repo
import com.botchat.app.datos.TipoCampo
import kotlinx.coroutines.launch

/** Ejemplos de «cómo responde»: se tocan y llenan el cuadro de instrucciones, que luego se puede cambiar. */
private val EJEMPLOS_PERSONALIDAD = listOf(
    "Amable" to "Eres un asistente amable y claro. Responde en español, en pocas frases y sin rodeos.",
    "Atención a clientes" to "Atiendes a los clientes de un negocio. Sé cortés y profesional, responde breve y, si no sabes algo, di que una persona lo revisará. No inventes precios, horarios ni promesas.",
    "Breve y directo" to "Responde en una o dos frases, directo al punto, sin saludos ni adornos.",
    "Divertido" to "Responde con buen humor y algún emoji, pero siempre de forma útil y breve.",
    "Tutor de inglés" to "Ayudas a practicar inglés: responde en inglés sencillo y, si la persona cometió errores, corrígelos con una nota corta en español.",
)

private fun colorDe(hex: String, defecto: Color = Color(0xFF607D8B)): Color = runCatching { Color(android.graphics.Color.parseColor(hex)) }.getOrDefault(defecto)

/** El logo del servicio (si viene en la app) o un círculo de color con su inicial. */
@Composable
fun LogoIA(p: ProveedorIA, tam: androidx.compose.ui.unit.Dp = 40.dp) {
    val ctx = LocalContext.current
    val recurso = remember(p.icono) { if (p.icono.isBlank()) 0 else ctx.resources.getIdentifier(p.icono, "drawable", ctx.packageName) }
    if (recurso != 0) {
        // fondo claro fijo: los logos oscuros (OpenAI, Grok…) se ven igual en tema claro y oscuro
        Box(Modifier.size(tam).clip(CircleShape).background(Color.White), Alignment.Center) {
            Image(painterResource(recurso), p.nombre, Modifier.size(tam * 0.64f), contentScale = ContentScale.Fit)
        }
    } else {
        Box(Modifier.size(tam).clip(CircleShape).background(colorDe(p.color)), Alignment.Center) {
            Text(p.inicial, color = Color.White, fontWeight = FontWeight.Bold, fontSize = (tam.value * 0.42f).sp)
        }
    }
}

/** Configuración propia del módulo de IA: elegir servicio con logo, modelo, llave, probar, cómo responde y dónde. */
@Composable
fun PantallaConfigIA(botId: Long, m: ModuloInfo) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val haptica = rememberHaptica()
    val provs by produceState<List<ProveedorIA>?>(null) { value = Repo.iaProveedores() }
    val v = remember(m.id) { mutableStateMapOf<String, String>() }
    var cargado by remember(m.id) { mutableStateOf(false) }
    var llaveGuardada by remember(m.id) { mutableStateOf(false) }
    var llaveNueva by remember(m.id) { mutableStateOf("") }
    var verLlave by remember(m.id) { mutableStateOf(false) }
    var msgLlave by remember(m.id) { mutableStateOf<String?>(null) }
    var error by remember(m.id) { mutableStateOf<String?>(null) }
    var guardado by remember(m.id) { mutableStateOf(false) }
    var probando by remember(m.id) { mutableStateOf(false) }
    var prueba by remember(m.id) { mutableStateOf<Pair<Boolean, String>?>(null) }
    var avanzado by remember(m.id) { mutableStateOf(false) }

    LaunchedEffect(m.id) {
        val c = runCatching { Repo.configModulo(botId, m.id) }.getOrDefault(emptyMap())
        m.esquema.forEach { f -> if (f.tipo != TipoCampo.SECRETO) v[f.clave] = textoDe(c[f.clave] ?: f.defecto) }
        llaveGuardada = runCatching { com.botchat.app.seguridad.LlavesModulos.existe(ctx, botId, m.id, "llave") }.getOrDefault(false)
        cargado = true
    }
    if (!cargado || provs == null) { Box(Modifier.fillMaxWidth().padding(Esp.l), Alignment.Center) { CircularProgressIndicator() }; return }
    val lista = provs.orEmpty()
    val actual = lista.firstOrNull { it.id == v["proveedor"] } ?: lista.firstOrNull { it.id == "otro" }
    fun campo(c: String): CampoModulo? = m.esquema.firstOrNull { it.clave == c }

    fun guardar(alTerminar: (Boolean) -> Unit = {}) {
        val cfg = LinkedHashMap<String, Any?>()
        var fallo: String? = null
        m.esquema.forEach { f ->
            val t = v[f.clave].orEmpty().trim()
            when (f.tipo) {
                TipoCampo.SECRETO -> {}
                TipoCampo.NUMERO -> t.replace(',', '.').toDoubleOrNull()?.let { cfg[f.clave] = it } ?: run { fallo = "«${f.titulo}» debe ser un número" }
                TipoCampo.INTERRUPTOR -> cfg[f.clave] = t == "true"
                else -> cfg[f.clave] = t
            }
        }
        error = fallo
        if (fallo != null) { alTerminar(false); return }
        scope.launch {
            runCatching { Repo.guardarConfigModulo(botId, m.id, cfg) }
                .onSuccess { guardado = true; error = null; alTerminar(true) }
                .onFailure { guardado = false; error = it.message ?: "No se pudo guardar"; alTerminar(false) }
        }
    }
    fun elegir(p: ProveedorIA) {
        haptica(Toque.Suave)
        v["proveedor"] = p.id
        if (p.url.isNotBlank()) v["url"] = p.url
        if (p.defecto.isNotBlank()) v["modelo"] = p.defecto
        prueba = null; guardado = false
        guardar()
    }

    @Composable
    fun Titulo(t: String, sub: String? = null) {
        Text(t, style = MaterialTheme.typography.titleMedium)
        if (sub != null) Text(sub, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }

    @Composable
    fun Interruptor(clave: String, titulo: String? = null, ayuda: String? = null) {
        val f = campo(clave) ?: return
        Row(Modifier.fillMaxWidth().testTag("campo_$clave"), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text(titulo ?: f.titulo, style = MaterialTheme.typography.titleSmall)
                (ayuda ?: f.ayuda).takeIf { it.isNotBlank() }?.let { Text(it, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant) }
            }
            Switch(v[clave] == "true", { haptica(Toque.Suave); v[clave] = it.toString(); guardado = false; guardar() }, Modifier.testTag("sw_$clave"))
        }
    }

    @Composable
    fun Numero(clave: String, titulo: String? = null) {
        val f = campo(clave) ?: return
        OutlinedTextField(
            v[clave].orEmpty(), { v[clave] = it.filter { c -> c.isDigit() || c == '.' }; guardado = false }, Modifier.fillMaxWidth().testTag("in_$clave"), singleLine = true,
            label = { Text(titulo ?: f.titulo) }, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
            supportingText = { Text((if (f.ayuda.isNotBlank()) f.ayuda + ". " else "") + if (f.max != 0.0) "Entre ${textoDe(f.min)} y ${textoDe(f.max)}" else "Mínimo ${textoDe(f.min)}") },
        )
    }

    @Composable
    fun Tarjeta(contenido: @Composable () -> Unit) {
        Card(Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer)) {
            Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) { contenido() }
        }
    }

    Column(Modifier.fillMaxWidth().testTag("config_ia"), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
        // ---------- 1. servicio ----------
        Tarjeta {
            Titulo("1 · Servicio de IA", "Toca el que quieras usar: la dirección y los modelos se llenan solos.")
            lista.chunked(3).forEach { fila ->
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    fila.forEach { p ->
                        val sel = p.id == actual?.id
                        Column(
                            Modifier.weight(1f).clip(RoundedCornerShape(16.dp))
                                .border(BorderStroke(if (sel) 2.dp else 1.dp, if (sel) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.outlineVariant), RoundedCornerShape(16.dp))
                                .background(if (sel) MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.35f) else Color.Transparent)
                                .clickable { elegir(p) }.padding(vertical = Esp.s, horizontal = Esp.xs).testTag("prov_${p.id}"),
                            horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(Esp.xs),
                        ) {
                            LogoIA(p, 44.dp)
                            Text(p.nombre, style = MaterialTheme.typography.labelMedium, textAlign = TextAlign.Center, maxLines = 1)
                        }
                    }
                    repeat(3 - fila.size) { Box(Modifier.weight(1f)) }
                }
            }
            actual?.let { p ->
                if (p.detalle.isNotBlank()) Text(p.detalle, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                if (p.gratis) Text("Tiene nivel gratis (con límites).", style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.primary)
            }
        }

        // ---------- 2. modelo ----------
        Tarjeta {
            Titulo("2 · Modelo", "El que contesta. Los baratos y rápidos bastan para un bot de chat.")
            actual?.modelos?.takeIf { it.isNotEmpty() }?.let { ms ->
                FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                    ms.forEach { mo ->
                        FilterChip(v["modelo"] == mo.id, { haptica(Toque.Suave); v["modelo"] = mo.id; prueba = null; guardado = false; guardar() },
                            { Text(if (mo.nota.isBlank()) mo.nombre else "${mo.nombre} · ${mo.nota}", maxLines = 1) }, modifier = Modifier.testTag("modelo_${mo.id}"))
                    }
                }
            }
            OutlinedTextField(v["modelo"].orEmpty(), { v["modelo"] = it; prueba = null; guardado = false }, Modifier.fillMaxWidth().testTag("in_modelo"), singleLine = true,
                label = { Text("Nombre del modelo") }, supportingText = { Text("Puedes escribir otro si el servicio saca uno nuevo.") })
            if (actual?.id == "otro" || avanzado || actual?.local == true) {
                OutlinedTextField(v["url"].orEmpty(), { v["url"] = it; prueba = null; guardado = false }, Modifier.fillMaxWidth().testTag("in_url"), singleLine = true,
                    label = { Text("Dirección del servicio") },
                    supportingText = { Text(if (actual?.local == true) "Ejemplo: http://192.168.1.50:11434/v1 (la computadora donde corre Ollama)." else "Se le agrega /chat/completions si hace falta.") })
            } else TextButton({ avanzado = true }) { Text("Cambiar la dirección") }
        }

        // ---------- 3. llave ----------
        if (actual?.sinLlave != true) Tarjeta {
            Titulo("3 · Llave (API key)", "Se guarda cifrada en este teléfono y no se puede volver a ver.")
            Text(if (llaveGuardada) "✅ Ya hay una llave guardada." else "Todavía no hay llave guardada.", Modifier.testTag("llave_estado_llave"),
                style = MaterialTheme.typography.labelLarge, color = if (llaveGuardada) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant)
            OutlinedTextField(
                llaveNueva, { llaveNueva = it; msgLlave = null }, Modifier.fillMaxWidth().testTag("in_llave"), singleLine = true,
                visualTransformation = if (verLlave) VisualTransformation.None else PasswordVisualTransformation(),
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
                placeholder = { Text(actual?.pista?.takeIf { it.isNotBlank() }?.let { "Empieza con $it" } ?: if (llaveGuardada) "Escribe otra para reemplazarla" else "Pega la llave aquí") },
                trailingIcon = { TextButton({ verLlave = !verLlave }) { Text(if (verLlave) "Ocultar" else "Ver") } },
            )
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                Button({
                    haptica(Toque.Confirmar)
                    val x = llaveNueva.trim()
                    if (x.isEmpty()) { msgLlave = "Escribe la llave primero"; return@Button }
                    scope.launch {
                        runCatching { Repo.guardarLlaveModulo(ctx, botId, m.id, "llave", x) }
                            .onSuccess { llaveNueva = ""; llaveGuardada = true; msgLlave = "Llave guardada"; prueba = null }
                            .onFailure { msgLlave = it.message ?: "No se pudo guardar la llave" }
                    }
                }, Modifier.testTag("guardar_llave_llave")) { Text("Guardar llave") }
                if (llaveGuardada) OutlinedButton({
                    haptica(Toque.Confirmar)
                    scope.launch {
                        runCatching { Repo.guardarLlaveModulo(ctx, botId, m.id, "llave", "") }
                            .onSuccess { llaveGuardada = false; msgLlave = "Llave borrada"; prueba = null }
                            .onFailure { msgLlave = it.message ?: "No se pudo borrar la llave" }
                    }
                }, Modifier.testTag("borrar_llave_llave")) { Text("Borrar") }
                actual?.urlLlave?.takeIf { it.isNotBlank() }?.let { u ->
                    OutlinedButton({ runCatching { ctx.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(u)).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)) } }, Modifier.testTag("conseguir_llave")) { Text("Conseguir mi llave") }
                }
            }
            msgLlave?.let { Text(it, Modifier.testTag("llave_msg"), style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.primary) }
        }

        // ---------- 4. probar ----------
        Tarjeta {
            Titulo("Probar la conexión", "Manda una pregunta corta al modelo para ver que todo está bien.")
            Button({
                haptica(Toque.Confirmar)
                probando = true; prueba = null
                guardar { ok ->
                    if (!ok) { probando = false; return@guardar }
                    scope.launch {
                        runCatching { Repo.iaProbar(botId) }
                            .onSuccess { prueba = true to it }
                            .onFailure { prueba = false to (it.message ?: "No se pudo conectar") }
                        probando = false
                    }
                }
            }, Modifier.fillMaxWidth().testTag("ia_probar"), enabled = !probando) {
                if (probando) CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp) else Text("Probar ahora")
            }
            prueba?.let { (ok, t) ->
                Text(if (ok) "✅ Funciona: «$t»" else "❌ $t", Modifier.testTag("ia_prueba"), style = MaterialTheme.typography.bodyMedium,
                    color = if (ok) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error)
            }
        }

        // ---------- 5. cómo responde ----------
        Tarjeta {
            Titulo("4 · Cómo responde", "Aquí le explicas quién es, qué tono usa, qué sabe de tu negocio (horarios, precios…) y qué no debe hacer.")
            FlowRow(horizontalArrangement = Arrangement.spacedBy(Esp.s)) {
                EJEMPLOS_PERSONALIDAD.forEach { (n, t) -> AssistChip({ haptica(Toque.Suave); v["personalidad"] = t; guardado = false }, { Text(n) }, modifier = Modifier.testTag("ejemplo_$n")) }
            }
            OutlinedTextField(v["personalidad"].orEmpty(), { v["personalidad"] = it.take(1200); guardado = false }, Modifier.fillMaxWidth().testTag("in_personalidad"), minLines = 5,
                label = { Text("Instrucciones") }, supportingText = { Text("${v["personalidad"].orEmpty().length}/1200 · Cada chat puede tener las suyas más abajo.") })
            OutlinedTextField(v["etiqueta"].orEmpty(), { v["etiqueta"] = it.take(60); guardado = false }, Modifier.fillMaxWidth().testTag("in_etiqueta"), singleLine = true,
                label = { Text("Etiqueta al inicio de cada respuesta") }, supportingText = { Text("Avisa que escribe una IA y no una persona. No puede quedar vacía.") })
            Numero("max_respuesta")
        }

        // ---------- 6. dónde responde ----------
        Tarjeta {
            Titulo("5 · Dónde responde")
            Interruptor("por_comando")
            Interruptor("por_mencion")
            Interruptor("chats_activos")
            Interruptor("en_menus", "Responder con IA dentro de los menús", "En un menú, el paso puede contestar con IA a lo que la persona escribió.")
            Text("Para «chats permitidos» elige los chats en la lista de más abajo.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }

        // ---------- 7. privacidad ----------
        Card(Modifier.fillMaxWidth(), colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.errorContainer.copy(alpha = 0.35f))) {
            Column(Modifier.padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.s)) {
                Titulo("Privacidad")
                Interruptor("acepto_privacidad")
            }
        }

        // ---------- 8. memoria y límites ----------
        Tarjeta {
            Titulo("Memoria y límites")
            Interruptor("memoria")
            Numero("memoria_turnos")
            Numero("limite_dia")
            Numero("limite_persona_hora")
            Numero("espera_chat_s")
        }
        Tarjeta {
            Titulo("Ajustes técnicos")
            Numero("max_entrada")
            Numero("tiempo_s")
            Interruptor("red_local")
        }

        error?.let { Text(it, Modifier.testTag("config_error"), color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodyMedium) }
        if (guardado) Text("Guardado", Modifier.testTag("config_guardado"), color = MaterialTheme.colorScheme.primary, style = MaterialTheme.typography.labelLarge)
        Button({ haptica(Toque.Confirmar); guardar() }, Modifier.fillMaxWidth().testTag("config_guardar")) { Text("Guardar") }
    }
}

private fun textoDe(v: Any?): String = when (v) {
    null -> ""
    is Double -> if (v == Math.floor(v) && !v.isInfinite()) v.toLong().toString() else v.toString()
    else -> v.toString()
}
