@file:OptIn(ExperimentalMaterial3Api::class)

package com.botchat.app.ui

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
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.botchat.app.datos.Ajustes

/** Pestaña Ajustes: Seguridad, Descargas, Estado del bot y Avanzado. */
@Composable
fun PantallaAjustes(ajustes: Ajustes, alSeguridad: () -> Unit, alDescargas: () -> Unit, alBot: () -> Unit, alAvanzado: () -> Unit, alEstadisticas: () -> Unit = {}, alPrivacidad: () -> Unit = {}, alAutomatizaciones: () -> Unit = {}, alPermisos: () -> Unit = {}, pendientesSolicitudes: Int = 0, alModulos: () -> Unit = {}, alIniciativa: () -> Unit = {},
    alAvisos: () -> Unit = {}, alComandosPropios: () -> Unit = {}, alEtiquetas: () -> Unit = {}, alAdvertencias: () -> Unit = {},
) {
    val modo by ajustes.bloqueo.collectAsState()
    val ctx = LocalContext.current
    val version = remember { runCatching { ctx.packageManager.getPackageInfo(ctx.packageName, 0).versionName }.getOrNull() ?: "" }
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(Esp.l), verticalArrangement = Arrangement.spacedBy(Esp.m)) {
        Tarjeta(Ico.Chats, "Automatizaciones", "Menús y respuestas del bot", alAutomatizaciones, "ajuste_automatizaciones")
        Tarjeta(Ico.Responder, "Comandos propios", "Tus propios comandos: !horario, !precios…", alComandosPropios, "ajuste_comandos_propios")
        Tarjeta(Ico.Persona, "Rangos y permisos", if (pendientesSolicitudes > 0) plural(pendientesSolicitudes.toLong(), "solicitud pendiente", "solicitudes pendientes") else "Quién puede usar qué, solicitudes y grupos", alPermisos, "ajuste_permisos")
        Tarjeta(Ico.Aviso, "Advertencias y notas", "A quién has advertido y lo que anotaste de cada persona", alAdvertencias, "ajuste_advertencias")
        Tarjeta(Ico.Seleccionar, "Etiquetas", "Ordena tus chats con etiquetas de colores", alEtiquetas, "ajuste_etiquetas")
        Tarjeta(Ico.Reloj, "Avisos para mí", "Palabras clave, resumen del día y recordatorios", alAvisos, "ajuste_avisos")
        Tarjeta(Ico.Agregar, "Módulos", "Extras opcionales, apagados hasta que los enciendas", alModulos, "ajuste_modulos")
        Tarjeta(Ico.Aviso, "Iniciativa propia", "El interruptor general de lo que el bot escribe solo, topes y No molestar", alIniciativa, "ajuste_iniciativa")
        Tarjeta(Ico.Candado, "Seguridad", when (modo) { Ajustes.Bloqueo.NINGUNO -> "Sin bloqueo"; Ajustes.Bloqueo.PIN -> "Protegida con PIN"; Ajustes.Bloqueo.HUELLA -> "Protegida con huella" }, alSeguridad, "ajuste_seguridad")
        Tarjeta(Ico.Candado, "Privacidad", "Mensajes eliminados por otros", alPrivacidad, "ajuste_privacidad")
        Tarjeta(Ico.Estadisticas, "Estadísticas", "Mensajes enviados y recibidos, y los chats más activos", alEstadisticas, "ajuste_estadisticas")
        Tarjeta(Ico.Descargar, "Descargas", "Qué se baja solo, con Wi-Fi o con datos", alDescargas, "ajuste_descargas")
        Tarjeta(Ico.Chats, "Estado del bot", "Encender, apagar y mantenerlo conectado", alBot, "ajuste_bot")
        Tarjeta(Ico.Info, "Avanzado", "Aspecto, registro y otras opciones técnicas", alAvanzado, "ajuste_avanzado")
        Spacer(Modifier.size(Esp.l))
        Column(Modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally) {
            LogoBot(Modifier.size(48.dp))
            Spacer(Modifier.size(Esp.s))
            Text("Bot Chat $version", style = MaterialTheme.typography.titleSmall)
            Text("Herramienta no oficial. No tiene relación con WhatsApp ni con Meta. Todo se guarda solo en este teléfono.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, textAlign = TextAlign.Center, modifier = Modifier.padding(top = Esp.xs))
        }
    }
}

@Composable
private fun Tarjeta(icono: ImageVector, titulo: String, texto: String, alPulsar: () -> Unit, etiqueta: String) {
    Card(
        onClick = alPulsar, modifier = Modifier.fillMaxWidth().testTag(etiqueta), shape = MaterialTheme.shapes.large,
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
    ) {
        Row(Modifier.padding(Esp.l), verticalAlignment = Alignment.CenterVertically) {
            Box(Modifier.size(48.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primaryContainer), Alignment.Center) {
                Icon(icono, null, Modifier.size(24.dp), tint = MaterialTheme.colorScheme.onPrimaryContainer)
            }
            Spacer(Modifier.width(Esp.l))
            Column(Modifier.weight(1f)) {
                Text(titulo, style = MaterialTheme.typography.titleMedium)
                Text(texto, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }
}
