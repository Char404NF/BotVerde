package com.botchat.app.ui

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Checkbox
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp

const val TITULO_AVISO = "Términos y condiciones"

/** Los términos que se aceptan una sola vez al empezar (cada uno: título y texto). Reúnen todo lo que antes avisaba cada módulo. */
val TERMINOS = listOf(
    "Herramienta no oficial" to "Bot Chat no tiene ninguna relación con WhatsApp ni con Meta. Se conecta a tu cuenta como un dispositivo vinculado.",
    "Tu cuenta de WhatsApp" to "WhatsApp puede limitar o bloquear los números que automatizan mensajes, usan funciones no oficiales (como los botones y las listas de los menús), escriben sin que les hayan escrito antes, mandan muchos mensajes seguidos o agregan gente a grupos. Te recomendamos usar un número secundario. Lo que le pase a tu cuenta es tu responsabilidad.",
    "Mensajes automáticos" to "Los módulos que escriben solos (difusiones, mensajes programados, bienvenidas por privado, invitar o agregar a grupos, sorteos, votaciones y tablón) solo funcionan con «Iniciativa propia» encendida. Úsalos con personas que ya te escribieron o que pidieron recibirlos, respeta a quien pida no recibir más y mantén los topes bajos. Puedes apagarlos cuando quieras.",
    "Contenido y derechos de autor" to "Bajar y mandar videos o música puede violar derechos de autor y las reglas de cada sitio. Tú eres responsable de lo que se pida con tu bot y de cumplir las leyes que te apliquen.",
    "Privacidad de tus datos" to "Tus mensajes y archivos se guardan cifrados solo en este teléfono. Lo que mandes a la IA, al traductor o a un conector sale del teléfono hacia el servicio que elijas: no incluyas información que no quieras compartir con él. Las llaves se guardan en el almacén seguro de Android.",
    "Personas de tus grupos" to "Si el bot cuenta mensajes, modera, advierte, banea o expulsa, lo hace por reglas que tú pones. Avisa a tus grupos de las reglas y respeta la privacidad de las personas.",
    "Sin garantías" to "La app se ofrece tal cual, sin garantías. No nos hacemos responsables de bloqueos de cuentas, pérdida de datos, mensajes que no salgan o cualquier otro daño por usarla. La usas bajo tu propia responsabilidad.",
)

/** Bienvenida de 3 pantallas para quien empieza; la última pide aceptar el aviso antes de vincular. */
@Composable
fun Bienvenida(alAceptar: () -> Unit) {
    val reducir = LocalReducirMovimiento.current
    var pagina by rememberSaveable { mutableStateOf(0) }
    var acepta by rememberSaveable { mutableStateOf(false) }
    val haptica = rememberHaptica()
    var adelante by rememberSaveable { mutableStateOf(true) }
    Surface(Modifier.fillMaxSize()) {
        Column(Modifier.fillMaxSize().padding(horizontal = Esp.xl, vertical = Esp.l)) {
            AnimatedContent(
                pagina, Modifier.weight(1f).fillMaxWidth(), label = "bienvenida",
                transitionSpec = {
                    if (reducir) fadeIn(tween(0)) togetherWith fadeOut(tween(0))
                    else fadeIn(tween(220, 80, androidx.compose.animation.core.LinearOutSlowInEasing)) togetherWith fadeOut(tween(100))
                },
            ) { p ->
                Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
                    when (p) {
                        0 -> Paso(null, "Hola, esto es Bot Chat", "Conecta un número secundario y deja que tu bot responda por ti. Todo es muy sencillo: te guiamos paso a paso.")
                        1 -> Paso(Ico.Espacio, "Todo se queda en tu teléfono", "Sin cuentas ni servidores. Tus mensajes y archivos se guardan cifrados aquí, y tú decides qué borrar. Si quieres, puedes proteger la app con PIN o huella.")
                        else -> {
                            Icono(Ico.Aviso)
                            Spacer(Modifier.height(Esp.l))
                            Text(TITULO_AVISO, style = MaterialTheme.typography.headlineSmall, textAlign = TextAlign.Center)
                            Spacer(Modifier.height(Esp.m))
                            Aviso(acepta) { acepta = it }
                        }
                    }
                }
            }
            Puntos(pagina)
            Spacer(Modifier.height(Esp.l))
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(Esp.m)) {
                if (pagina > 0) OutlinedButton({ haptica(Toque.Suave); adelante = false; pagina-- }, Modifier.weight(1f).height(52.dp)) { Text("Atrás") }
                if (pagina < 2) Button({ haptica(Toque.Suave); adelante = true; pagina++ }, Modifier.weight(1f).height(52.dp)) { Text("Siguiente") }
                else Button({ haptica(Toque.Confirmar); alAceptar() }, Modifier.weight(1f).height(52.dp).testTag("btn_empezar"), enabled = acepta) { Text("Empezar") }
            }
        }
    }
}

/** Para quien ya tenía el bot vinculado (actualización): solo el aviso, una vez. */
@Composable
fun AvisoUnico(alAceptar: () -> Unit) {
    var acepta by rememberSaveable { mutableStateOf(false) }
    val haptica = rememberHaptica()
    Surface(Modifier.fillMaxSize()) {
        Column(Modifier.fillMaxSize().padding(horizontal = Esp.xl, vertical = Esp.l)) {
            Column(Modifier.weight(1f).fillMaxWidth().verticalScroll(rememberScrollState()), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
                Icono(Ico.Aviso)
                Spacer(Modifier.height(Esp.l))
                Text(TITULO_AVISO, style = MaterialTheme.typography.headlineSmall, textAlign = TextAlign.Center)
                Spacer(Modifier.height(Esp.m))
                Aviso(acepta) { acepta = it }
            }
            Spacer(Modifier.height(Esp.l))
            Button({ haptica(Toque.Confirmar); alAceptar() }, Modifier.fillMaxWidth().height(52.dp).testTag("btn_continuar"), enabled = acepta) { Text("Continuar") }
        }
    }
}

@Composable
private fun Aviso(acepta: Boolean, alCambiar: (Boolean) -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(Esp.m)) {
        TextoTerminos()
        Surface(shape = MaterialTheme.shapes.medium, color = MaterialTheme.colorScheme.surfaceContainerHigh, modifier = Modifier.fillMaxWidth().padding(top = Esp.s)) {
            Row(Modifier.clickable(role = Role.Checkbox) { alCambiar(!acepta) }.padding(Esp.m).testTag("casilla_acepto"), verticalAlignment = Alignment.CenterVertically) {
                Checkbox(acepta, null)
                Spacer(Modifier.width(Esp.m))
                Text("He leído y acepto los términos y condiciones", style = MaterialTheme.typography.titleMedium)
            }
        }
    }
}

/** Los términos, un punto por apartado. Se usa al aceptar y al consultarlos después en Ajustes. */
@Composable
fun TextoTerminos() {
    Column(verticalArrangement = Arrangement.spacedBy(Esp.m), modifier = Modifier.testTag("texto_terminos")) {
        TERMINOS.forEach { (titulo, texto) ->
            Column(verticalArrangement = Arrangement.spacedBy(Esp.xs)) {
                Text(titulo, style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary)
                Text(texto, style = MaterialTheme.typography.bodyMedium)
            }
        }
    }
}

@Composable
private fun Paso(icono: ImageVector?, titulo: String, texto: String) {
    if (icono == null) LogoBot(Modifier.size(120.dp)) else Icono(icono)
    Spacer(Modifier.height(Esp.xl))
    Text(titulo, style = MaterialTheme.typography.headlineSmall, textAlign = TextAlign.Center)
    Spacer(Modifier.height(Esp.m))
    Text(texto, style = MaterialTheme.typography.bodyLarge, color = MaterialTheme.colorScheme.onSurfaceVariant, textAlign = TextAlign.Center)
}

@Composable
private fun Icono(icono: ImageVector) {
    Box(Modifier.size(96.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primaryContainer), Alignment.Center) {
        Icon(icono, null, Modifier.size(48.dp), tint = MaterialTheme.colorScheme.onPrimaryContainer)
    }
}

@Composable
private fun Puntos(actual: Int) {
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.Center) {
        repeat(3) { i ->
            val activo = i == actual
            val ancho by androidx.compose.animation.core.animateDpAsState(if (activo) 24.dp else 8.dp, tween(Mov.NORMAL), label = "punto")
            Box(Modifier.padding(horizontal = 4.dp).height(8.dp).width(ancho).clip(CircleShape).background(if (activo) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.outlineVariant))
        }
    }
}
