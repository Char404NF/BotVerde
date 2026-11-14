package com.botchat.app.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.getValue
import androidx.compose.runtime.setValue
import androidx.compose.ui.unit.sp
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp

// Piezas comunes del estilo de mensajería: aviso del sistema en el chat, tarjeta de inicio de grupo y filas de ajustes con icono.

/** Aviso del sistema en el chat ("Se añadió a Ana"), en una cápsula centrada. */
@Composable
fun CapsulaSistema(texto: String, modifier: Modifier = Modifier) {
    val colores = LocalColoresChat.current
    Box(modifier.fillMaxWidth().padding(vertical = 4.dp), contentAlignment = Alignment.Center) {
        Surface(shape = RoundedCornerShape(10.dp), color = colores.capsula) {
            Text(
                texto, Modifier.padding(horizontal = 12.dp, vertical = 5.dp).testTag("aviso_sistema"),
                style = MaterialTheme.typography.labelMedium, color = colores.textoCapsula, textAlign = TextAlign.Center,
            )
        }
    }
}

/** Tarjeta del principio de un grupo: icono, nombre con cuántos son y un botón para ver su información. */
@Composable
fun TarjetaInicioGrupo(texto: String, alAbrirInfo: () -> Unit, modifier: Modifier = Modifier) {
    val colores = LocalColoresChat.current
    Box(modifier.fillMaxWidth().padding(vertical = 8.dp, horizontal = 12.dp), contentAlignment = Alignment.Center) {
        Surface(shape = RoundedCornerShape(16.dp), color = colores.capsula.copy(alpha = 0.9f)) {
            Column(Modifier.padding(20.dp).testTag("tarjeta_inicio_grupo"), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(8.dp)) {
                Box(Modifier.size(64.dp).clip(CircleShape).background(MaterialTheme.colorScheme.primaryContainer), Alignment.Center) {
                    Icon(Ico.Grupo, null, Modifier.size(34.dp), tint = MaterialTheme.colorScheme.onPrimaryContainer)
                }
                Text("Principio del grupo", style = MaterialTheme.typography.titleMedium, color = colores.textoRecibida)
                Text(texto, style = MaterialTheme.typography.bodyMedium, color = colores.textoCapsula, textAlign = TextAlign.Center)
                OutlinedButton(alAbrirInfo, Modifier.testTag("inicio_grupo_info")) { Text("Ver información del grupo") }
            }
        }
    }
}

/** Fila de ajustes: icono a la izquierda, título y una línea de ayuda; opcionalmente algo a la derecha. */
@Composable
fun FilaAjuste(icono: ImageVector, titulo: String, subtitulo: String = "", modifier: Modifier = Modifier, alPulsar: (() -> Unit)? = null, derecha: @Composable (() -> Unit)? = null) {
    Row(
        modifier.fillMaxWidth().let { if (alPulsar != null) it.clickable(onClick = alPulsar) else it }.padding(horizontal = Esp.l, vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(icono, null, Modifier.size(24.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.width(20.dp))
        Column(Modifier.weight(1f)) {
            Text(titulo, style = MaterialTheme.typography.titleMedium)
            if (subtitulo.isNotBlank()) Text(subtitulo, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        if (derecha != null) { Spacer(Modifier.width(Esp.s)); derecha() }
    }
}

/** Título de una sección dentro de una lista de ajustes (texto pequeño de color). */
@Composable
fun TituloSeccion(texto: String, modifier: Modifier = Modifier) {
    Text(texto, modifier.padding(start = Esp.l, end = Esp.l, top = Esp.l, bottom = Esp.xs), style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.primary)
}


/** Foto cuadrada (la de perfil sin recortar en círculo); si no hay foto, un color con la inicial o el icono de grupo. */
@Composable
fun FotoCuadrada(nombre: String, foto: String, fotoTs: Long, clave: String, grupo: Boolean, modifier: Modifier = Modifier) {
    val ctx = androidx.compose.ui.platform.LocalContext.current
    val px = with(androidx.compose.ui.platform.LocalDensity.current) { 256.dp.roundToPx() * 2 }
    var imagen by androidx.compose.runtime.remember(foto, fotoTs) { androidx.compose.runtime.mutableStateOf<androidx.compose.ui.graphics.ImageBitmap?>(null) }
    androidx.compose.runtime.LaunchedEffect(foto, fotoTs, px) {
        imagen = if (foto.isEmpty()) null else com.botchat.app.medios.CacheImagenes.cargar(ctx, com.botchat.app.datos.Repo.archivo(foto), px, fotoTs.toString())?.asImageBitmap()
    }
    Box(modifier.background(colorEstable(clave, androidx.compose.foundation.isSystemInDarkTheme())), contentAlignment = Alignment.Center) {
        val img = imagen
        if (img != null) androidx.compose.foundation.Image(img, null, Modifier.fillMaxWidth(), contentScale = androidx.compose.ui.layout.ContentScale.Crop)
        else if (grupo) Icon(Ico.Grupo, null, Modifier.size(96.dp), tint = androidx.compose.ui.graphics.Color.White)
        else Text(inicial(nombre), color = androidx.compose.ui.graphics.Color.White, fontSize = 96.sp, fontWeight = androidx.compose.ui.text.font.FontWeight.SemiBold)
    }
}

/**
 * Tarjeta que sale al tocar la foto de alguien en la lista: la foto cuadrada con el nombre arriba y una fila de botones
 * (mensaje e información). Tocar la foto la abre a pantalla completa.
 */
@Composable
fun TarjetaPerfilRapido(
    nombre: String, foto: String, fotoTs: Long, clave: String, esGrupo: Boolean,
    alMensaje: () -> Unit, alInfo: () -> Unit, alFoto: () -> Unit, alCerrar: () -> Unit,
) {
    androidx.compose.ui.window.Dialog(alCerrar) {
        Surface(shape = RoundedCornerShape(2.dp), color = MaterialTheme.colorScheme.surfaceContainer, modifier = Modifier.width(256.dp).testTag("tarjeta_perfil_rapido")) {
            Column {
                Box(Modifier.fillMaxWidth().aspectRatio(1f).clickable(onClick = alFoto).testTag("foto_perfil_rapido")) {
                    FotoCuadrada(nombre, foto, fotoTs, clave, esGrupo, Modifier.fillMaxSize())
                    Box(
                        Modifier.fillMaxWidth().background(androidx.compose.ui.graphics.Color(0xCCBDBDBD)).padding(horizontal = 8.dp, vertical = 5.dp),
                    ) {
                        Text(nombre, style = MaterialTheme.typography.titleLarge.copy(fontSize = 19.sp, fontWeight = androidx.compose.ui.text.font.FontWeight.Normal), color = androidx.compose.ui.graphics.Color.White, maxLines = 1, overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis)
                    }
                }
                Row(Modifier.fillMaxWidth().padding(vertical = 4.dp), horizontalArrangement = Arrangement.SpaceEvenly) {
                    androidx.compose.material3.IconButton(alMensaje, Modifier.testTag("perfil_rapido_mensaje")) { Icon(Ico.Chats, "Mensaje", tint = MaterialTheme.colorScheme.primary) }
                    androidx.compose.material3.IconButton(alInfo, Modifier.testTag("perfil_rapido_info")) { Icon(Ico.Info, if (esGrupo) "Información del grupo" else "Información del contacto", tint = MaterialTheme.colorScheme.primary) }
                }
            }
        }
    }
}
