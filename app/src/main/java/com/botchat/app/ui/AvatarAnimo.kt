package com.botchat.app.ui

import android.graphics.BitmapFactory
import androidx.compose.animation.Crossfade
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext

/** Los ánimos del avatar del bot (imágenes en assets/animos). El primero es el de siempre. */
val ANIMOS = listOf("feliz", "contento", "guino", "pensando", "enojado", "triste", "sorprendido", "dormido")

/** Nombre en español de cada ánimo, para las pantallas. */
fun nombreAnimo(a: String) = when (a) {
    "feliz" -> "Feliz"; "contento" -> "Muy contento"; "guino" -> "Guiño"; "pensando" -> "Pensando"
    "enojado" -> "Enojado"; "triste" -> "Triste"; "sorprendido" -> "Sorprendido"; "dormido" -> "Dormido"; else -> a
}

/** Cómo se ve el bot según su estado: contento si está conectado, pensando si reconecta, dormido si está apagado, triste si perdió la sesión. */
fun animoDeEstado(e: EstadoVisible) = when (e) {
    EstadoVisible.CONECTADO -> "feliz"
    EstadoVisible.RECONECTANDO -> "pensando"
    EstadoVisible.DESVINCULADO -> "triste"
    EstadoVisible.DETENIDO -> "dormido"
}

/** Dibuja el avatar con el ánimo dado; al cambiar de ánimo, la cara se funde suavemente. */
@Composable
fun AvatarAnimo(animo: String, modifier: Modifier = Modifier, descripcion: String? = null) {
    val ctx = LocalContext.current
    val reducir = LocalReducirMovimiento.current
    Crossfade(animo, modifier, animationSpec = Mov.tiempo(reducir, Mov.NORMAL), label = "animo") { a ->
        val imagen = remember(a) {
            runCatching { ctx.assets.open("animos/${if (a in ANIMOS) a else "feliz"}.png").use { BitmapFactory.decodeStream(it) }?.asImageBitmap() }.getOrNull()
        }
        Box(Modifier.fillMaxSize()) {
            if (imagen != null) Image(imagen, descripcion, Modifier.fillMaxSize(), contentScale = ContentScale.Fit)
        }
    }
}
